// Package daemon owns the long-running background process that watches the
// filesystem, polls S3, runs the reconciler, and serves the local control
// plane (loopback HTTP UI). It wires the engine, watcher, store, S3 client,
// crypto, and HTTP server into one process.
package daemon

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	stdsync "sync"
	"sync/atomic"
	"time"

	"github.com/goodylili/dropboy/internal/config"
	dcrypto "github.com/goodylili/dropboy/internal/crypto"
	"github.com/goodylili/dropboy/internal/keychain"
	"github.com/goodylili/dropboy/internal/s3"
	"github.com/goodylili/dropboy/internal/server"
	"github.com/goodylili/dropboy/internal/store"
	"github.com/goodylili/dropboy/internal/sync"
)

type Daemon struct {
	cfg     config.Config
	dataDir string
	srv     *server.Server

	mu     stdsync.Mutex // guards engine/store/s3c/master during Unlock
	engine *sync.Engine
	store  store.Store
	s3c    s3.Client
	master []byte

	runCtx    context.Context // captured at Run; used by post-boot Unlock
	loopStart stdsync.Once    // ensures reconcile loop only starts once

	paused atomic.Bool
	locked atomic.Bool
}

// Options carry runtime knobs that don't belong in the on-disk config.
type Options struct {
	Passphrase string // required first time; cached in memory while running
	NoS3       bool   // skip S3 wiring (useful when AWS credentials are unavailable)
	NoSync     bool   // serve the UI/API but don't run the reconcile loop
}

func New(cfg config.Config, dataDir string) *Daemon {
	d := &Daemon{cfg: cfg, dataDir: dataDir}
	d.srv = server.New(cfg, dataDir)
	d.srv.SetEngine(d) // server reads stats/conflicts through this interface
	return d
}

func (d *Daemon) Server() *server.Server { return d.srv }

// Paused / SetPaused let the API toggle sync without tearing the engine down.
func (d *Daemon) Paused() bool        { return d.paused.Load() }
func (d *Daemon) SetPaused(v bool)    { d.paused.Store(v) }

// Conflicts / ResolveConflict are exposed to the server package.
func (d *Daemon) Conflicts() []server.Conflict {
	if d.engine == nil {
		return nil
	}
	in := d.engine.Conflicts()
	out := make([]server.Conflict, 0, len(in))
	for _, c := range in {
		out = append(out, server.Conflict{
			ID: c.ID, Path: c.Path, Machine: d.cfg.MachineID,
			Detected: c.Detected.Format(time.RFC3339),
			Local:    c.Local, Remote: c.Remote,
		})
	}
	return out
}

func (d *Daemon) ResolveConflict(id string) bool {
	if d.engine == nil {
		return false
	}
	return d.engine.ResolveConflict(id)
}

// Stats exposes engine activity to the API as a typed snapshot. When the
// engine isn't running (no creds / NoSync), zero values are returned.
func (d *Daemon) Stats() server.EngineStats {
	if d.engine == nil {
		return server.EngineStats{Paused: d.paused.Load()}
	}
	s := d.engine.Stats()
	return server.EngineStats{
		LastRun:    s.LastRun,
		QueueUp:    s.QueueUp,
		QueueDown:  s.QueueDown,
		BytesUp:    s.BytesUp,
		BytesDown:  s.BytesDown,
		Conflicts:  s.Conflicts,
		Paused:     d.paused.Load(),
		EngineLive: true,
	}
}

// KickSync triggers an immediate reconcile pass.
func (d *Daemon) KickSync() error {
	if d.engine == nil {
		return errors.New("sync engine not running")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	_, err := d.engine.ReconcileOnce(ctx)
	return err
}

// Run boots the engine + watcher + reconcile loop alongside the HTTP server.
//
// Engine startup is best-effort. Passphrase lookup order:
//  1. opts.Passphrase (env / CLI flag) — for foreground/interactive use.
//  2. OS keychain (service=com.dropboy, account=<machine-id|"default">).
//  3. None — the daemon stays "locked": HTTP/UI come up, the engine is nil
//     until /api/v1/unlock supplies the passphrase.
//
// A locked daemon is the steady state under launchd/systemd if the user
// hasn't stored their passphrase in the keychain; the UI prompts on first
// open and (optionally) saves it for next boot.
func (d *Daemon) Run(ctx context.Context, opts Options) error {
	d.runCtx = ctx

	pass := opts.Passphrase
	if pass == "" {
		if v, err := keychain.Get(d.passphraseAccount()); err == nil {
			pass = v
		}
	}

	if err := d.initEngine(ctx, opts, pass); err != nil {
		d.locked.Store(true)
		fmt.Printf("dropboy: engine offline (locked): %v\n", err)
	}

	if d.engine != nil && !opts.NoSync {
		d.startReconcileLoop()
	}

	if err := d.srv.Start(ctx); err != nil {
		return fmt.Errorf("server: %w", err)
	}
	if d.store != nil {
		_ = d.store.Close()
	}
	return nil
}

// Unlock attempts to bring the engine online with the given passphrase. If
// remember is true and the passphrase is correct, it is persisted in the OS
// keychain so subsequent daemon starts don't need the prompt.
func (d *Daemon) Unlock(passphrase string, remember bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.engine != nil {
		return nil // already unlocked
	}
	if err := d.initEngine(d.runCtx, Options{}, passphrase); err != nil {
		return err
	}
	d.locked.Store(false)
	if remember {
		_ = keychain.Set(d.passphraseAccount(), passphrase)
	}
	d.startReconcileLoop()
	return nil
}

// UnlockWithRecovery brings the engine online using the recovery code printed
// at init time. Used by `dropboy recover` when the passphrase has been lost.
// The engine is unlocked for this session; subsequent daemon restarts still
// require the original passphrase until the user sets a new one.
func (d *Daemon) UnlockWithRecovery(code string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.engine != nil {
		return nil
	}
	master, err := d.loadMasterWithRecovery(code)
	if err != nil {
		return err
	}
	if err := d.initEngineWithMaster(d.runCtx, master); err != nil {
		return err
	}
	d.locked.Store(false)
	d.startReconcileLoop()
	return nil
}

// Locked reports whether the engine is awaiting a passphrase.
func (d *Daemon) Locked() bool { return d.locked.Load() }

// ForgetPassphrase removes the keychain entry. The daemon stays unlocked
// until next restart; subsequent restarts will prompt.
func (d *Daemon) ForgetPassphrase() error { return keychain.Delete(d.passphraseAccount()) }

func (d *Daemon) passphraseAccount() string {
	if d.cfg.MachineID != "" {
		return d.cfg.MachineID
	}
	return "default"
}

func (d *Daemon) initEngine(ctx context.Context, opts Options, pass string) error {
	if opts.NoS3 || d.cfg.Bucket == "" {
		return errors.New("S3 not configured (run `dropboy init`)")
	}
	master, err := d.loadOrCreateMaster(pass)
	if err != nil {
		return err
	}
	return d.initEngineWithMaster(ctx, master)
}

func (d *Daemon) initEngineWithMaster(ctx context.Context, master []byte) error {
	if d.cfg.Bucket == "" {
		return errors.New("S3 not configured (run `dropboy init`)")
	}
	st, err := store.Open(filepath.Join(d.dataDir, "state.db"))
	if err != nil {
		return fmt.Errorf("state db: %w", err)
	}

	s3c, err := s3.New(ctx, s3.Options{Bucket: d.cfg.Bucket, Region: d.cfg.Region, Profile: d.cfg.AWSProfile})
	if err != nil {
		_ = st.Close()
		return fmt.Errorf("s3 client: %w", err)
	}

	d.store = st
	d.s3c = s3c
	d.master = master
	d.engine = sync.NewEngine(d.cfg, st, s3c, master, d.cfg.MachineID)
	return nil
}

func (d *Daemon) loadOrCreateMaster(pass string) ([]byte, error) {
	if !dcrypto.HasMasterKey(d.dataDir) {
		return nil, errors.New("encryption not initialized — run `dropboy init`")
	}
	if pass == "" {
		return nil, errors.New("encryption passphrase required")
	}
	return dcrypto.LoadMasterKey(d.dataDir, pass)
}

// loadMasterWithRecovery unwraps the master key using the recovery code
// rather than the passphrase. Same outcome — used by `dropboy recover`.
func (d *Daemon) loadMasterWithRecovery(code string) ([]byte, error) {
	if !dcrypto.HasRecoveryKey(d.dataDir) {
		return nil, errors.New("no recovery key on disk — this install predates recovery codes")
	}
	if code == "" {
		return nil, errors.New("recovery code required")
	}
	return dcrypto.LoadMasterKeyWithRecovery(d.dataDir, code)
}

func (d *Daemon) startReconcileLoop() {
	d.loopStart.Do(func() {
		go d.runReconcileLoop(d.runCtx)
	})
}

func (d *Daemon) runReconcileLoop(ctx context.Context) {
	watcher := sync.NewWatcher(d.cfg)
	ticks := make(chan struct{}, 1)
	go func() { _ = watcher.Run(ctx, ticks) }()

	remote := time.Duration(d.cfg.Poll.RemoteSeconds) * time.Second
	if remote == 0 {
		remote = 60 * time.Second
	}
	full := time.Duration(d.cfg.Poll.FullScanMinutes) * time.Minute
	if full == 0 {
		full = 15 * time.Minute
	}

	remoteT := time.NewTicker(remote)
	fullT := time.NewTicker(full)
	defer remoteT.Stop()
	defer fullT.Stop()

	// Kick once at startup.
	d.reconcileOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks:
			if d.paused.Load() {
				continue
			}
			d.reconcileOnce(ctx)
		case <-remoteT.C:
			if d.paused.Load() {
				continue
			}
			d.reconcileOnce(ctx)
		case <-fullT.C:
			if d.paused.Load() {
				continue
			}
			d.reconcileOnce(ctx)
		}
	}
}

func (d *Daemon) reconcileOnce(ctx context.Context) {
	rctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	if _, err := d.engine.ReconcileOnce(rctx); err != nil {
		fmt.Printf("dropboy: reconcile error: %v\n", err)
	}
}
