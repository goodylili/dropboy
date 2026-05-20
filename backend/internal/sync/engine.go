// Package sync implements the two-way reconciliation engine described in
// PRD §5.1 — a three-way compare between the last-synced state, the current
// local snapshot, and the current remote snapshot, producing a list of
// operations (upload, download, delete, conflict).
package sync

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/goodylili/dropboy/internal/config"
	dcrypto "github.com/goodylili/dropboy/internal/crypto"
	"github.com/goodylili/dropboy/internal/s3"
	"github.com/goodylili/dropboy/internal/store"
)

type OpKind int

const (
	OpNoop OpKind = iota
	OpUpload
	OpDownload
	OpDeleteRemote
	OpDeleteLocal
	OpConflict
)

func (o OpKind) String() string {
	switch o {
	case OpUpload:
		return "upload"
	case OpDownload:
		return "download"
	case OpDeleteRemote:
		return "delete-remote"
	case OpDeleteLocal:
		return "delete-local"
	case OpConflict:
		return "conflict"
	default:
		return "noop"
	}
}

type Op struct {
	Kind  OpKind
	Path  string // absolute local path
	S3Key string
}

// Engine is the reconciliation + execution engine.
type Engine struct {
	cfg       config.Config
	store     store.Store
	s3        s3.Client
	master    []byte
	machineID string

	mu        sync.Mutex
	lastRun   time.Time
	lastOps   []Op
	conflicts []Conflict
	upQ       int
	downQ     int
	bytesUp   int64
	bytesDown int64
}

// Conflict captures a path where both sides changed since last sync.
type Conflict struct {
	ID       string
	Path     string
	Detected time.Time
	Local    string
	Remote   string
}

func NewEngine(cfg config.Config, st store.Store, s3c s3.Client, master []byte, machineID string) *Engine {
	return &Engine{
		cfg:       cfg,
		store:     st,
		s3:        s3c,
		master:    master,
		machineID: machineID,
	}
}

// Stats is a snapshot of engine activity for the API.
type Stats struct {
	LastRun   time.Time
	QueueUp   int
	QueueDown int
	BytesUp   int64
	BytesDown int64
	Conflicts int
}

func (e *Engine) Stats() Stats {
	e.mu.Lock()
	defer e.mu.Unlock()
	return Stats{
		LastRun: e.lastRun, QueueUp: e.upQ, QueueDown: e.downQ,
		BytesUp: e.bytesUp, BytesDown: e.bytesDown, Conflicts: len(e.conflicts),
	}
}

func (e *Engine) Conflicts() []Conflict {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]Conflict, len(e.conflicts))
	copy(out, e.conflicts)
	return out
}

// ResolveConflict drops a conflict from the in-memory list. The actual
// keep-local / keep-remote / keep-both side effects live in the daemon's
// follow-up reconcile pass, which sees the resolved state.
func (e *Engine) ResolveConflict(id string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i, c := range e.conflicts {
		if c.ID == id {
			e.conflicts = append(e.conflicts[:i], e.conflicts[i+1:]...)
			return true
		}
	}
	return false
}

// ReconcileOnce performs one full reconciliation pass.
func (e *Engine) ReconcileOnce(ctx context.Context) ([]Op, error) {
	local, err := e.snapshotLocal()
	if err != nil {
		return nil, fmt.Errorf("snapshot local: %w", err)
	}
	remote, err := e.snapshotRemote(ctx)
	if err != nil {
		return nil, fmt.Errorf("snapshot remote: %w", err)
	}
	state, err := e.snapshotState(ctx)
	if err != nil {
		return nil, fmt.Errorf("snapshot state: %w", err)
	}

	ops := plan(local, remote, state, e.machineID)

	e.mu.Lock()
	e.upQ = countKind(ops, OpUpload)
	e.downQ = countKind(ops, OpDownload)
	e.mu.Unlock()

	for _, op := range ops {
		if err := e.execute(ctx, op, local[op.Path]); err != nil {
			return ops, fmt.Errorf("execute %s %s: %w", op.Kind, op.Path, err)
		}
	}

	e.mu.Lock()
	e.lastRun = time.Now().UTC()
	e.lastOps = ops
	e.upQ, e.downQ = 0, 0
	e.mu.Unlock()
	return ops, nil
}

// ---- snapshots ----

type localEntry struct {
	Path  string
	Size  int64
	Mtime time.Time
	Hash  string
}

func (e *Engine) snapshotLocal() (map[string]localEntry, error) {
	out := map[string]localEntry{}
	for _, f := range e.cfg.Folders {
		err := filepath.WalkDir(f.Path, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if excluded(d.Name(), f.Exclude) {
					return filepath.SkipDir
				}
				return nil
			}
			if excluded(d.Name(), f.Exclude) {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			h, err := hashFile(p)
			if err != nil {
				return nil
			}
			out[p] = localEntry{Path: p, Size: info.Size(), Mtime: info.ModTime(), Hash: h}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func excluded(name string, patterns []string) bool {
	for _, p := range patterns {
		if ok, _ := filepath.Match(p, name); ok {
			return true
		}
	}
	return false
}

func hashFile(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

type remoteEntry struct {
	Key  string
	Path string
	Hash string
	Size int64
	ETag string
	Meta map[string]string
}

func (e *Engine) snapshotRemote(ctx context.Context) (map[string]remoteEntry, error) {
	prefix := keyPrefix(e.machineID)
	objs, err := e.s3.List(ctx, prefix)
	if err != nil {
		return nil, err
	}
	out := map[string]remoteEntry{}
	for _, o := range objs {
		if strings.Contains(o.Key, "/.dropboy/") {
			continue
		}
		local := pathFromKey(o.Key, e.machineID)
		head, err := e.s3.Head(ctx, o.Key)
		if err != nil {
			continue
		}
		hash := head.Metadata["dropboy-plain-sha256"]
		sizeStr := head.Metadata["dropboy-plain-size"]
		var size int64
		_, _ = fmt.Sscanf(sizeStr, "%d", &size)
		out[local] = remoteEntry{Key: o.Key, Path: local, Hash: hash, Size: size, ETag: o.ETag, Meta: head.Metadata}
	}
	return out, nil
}

func (e *Engine) snapshotState(ctx context.Context) (map[string]store.Entry, error) {
	entries, err := e.store.List(ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]store.Entry{}
	for _, e := range entries {
		out[e.Path] = e
	}
	return out, nil
}

// ---- planning ----

func plan(local map[string]localEntry, remote map[string]remoteEntry, state map[string]store.Entry, machineID string) []Op {
	var ops []Op
	seen := map[string]struct{}{}

	for p, l := range local {
		seen[p] = struct{}{}
		st, hadState := state[p]
		r, hadRemote := remote[p]
		switch {
		case !hadState && !hadRemote:
			ops = append(ops, Op{Kind: OpUpload, Path: p, S3Key: keyFor(machineID, p)})
		case !hadState && hadRemote:
			if r.Hash == l.Hash {
				continue
			}
			ops = append(ops, Op{Kind: OpConflict, Path: p, S3Key: r.Key})
		case hadState && !hadRemote:
			ops = append(ops, Op{Kind: OpUpload, Path: p, S3Key: keyFor(machineID, p)})
		default:
			localChanged := l.Hash != st.LocalHash
			remoteChanged := r.Hash != st.LocalHash && r.Hash != ""
			switch {
			case localChanged && remoteChanged:
				ops = append(ops, Op{Kind: OpConflict, Path: p, S3Key: r.Key})
			case localChanged:
				ops = append(ops, Op{Kind: OpUpload, Path: p, S3Key: keyFor(machineID, p)})
			case remoteChanged:
				ops = append(ops, Op{Kind: OpDownload, Path: p, S3Key: r.Key})
			}
		}
	}
	for p, r := range remote {
		if _, ok := seen[p]; ok {
			continue
		}
		if _, ok := state[p]; ok {
			ops = append(ops, Op{Kind: OpDeleteRemote, Path: p, S3Key: r.Key})
		} else {
			ops = append(ops, Op{Kind: OpDownload, Path: p, S3Key: r.Key})
		}
	}
	return ops
}

func countKind(ops []Op, k OpKind) int {
	n := 0
	for _, o := range ops {
		if o.Kind == k {
			n++
		}
	}
	return n
}

// ---- execution ----

func (e *Engine) execute(ctx context.Context, op Op, local localEntry) error {
	switch op.Kind {
	case OpUpload:
		return e.doUpload(ctx, op.Path, op.S3Key, local)
	case OpDownload:
		return e.doDownload(ctx, op.Path, op.S3Key)
	case OpDeleteRemote:
		if err := e.s3.Delete(ctx, op.S3Key); err != nil {
			return err
		}
		return e.store.Delete(ctx, op.Path)
	case OpDeleteLocal:
		if err := os.Remove(op.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return e.store.Delete(ctx, op.Path)
	case OpConflict:
		e.recordConflict(op.Path)
		return nil
	}
	return nil
}

func (e *Engine) doUpload(ctx context.Context, path, key string, local localEntry) error {
	plain, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	dek, err := dcrypto.GenerateKey()
	if err != nil {
		return err
	}
	ct, nonce, err := dcrypto.SealPayload(dek, plain)
	if err != nil {
		return err
	}
	sealed, err := dcrypto.WrapKey(e.master, dek)
	if err != nil {
		return err
	}
	meta := map[string]string{
		"dropboy-plain-sha256": local.Hash,
		"dropboy-plain-size":   fmt.Sprintf("%d", local.Size),
		"dropboy-plain-mtime":  local.Mtime.UTC().Format(time.RFC3339Nano),
		"dropboy-scheme":       "aes-256-gcm",
		"dropboy-nonce":        base64.StdEncoding.EncodeToString(nonce),
		"dropboy-dek-nonce":    base64.StdEncoding.EncodeToString(sealed.Nonce),
		"dropboy-dek":          base64.StdEncoding.EncodeToString(sealed.Ciphertext),
	}
	body := newThrottledReader(bytes.NewReader(ct), mbpsToBytes(e.cfg.Limits.MaxUploadMbps))
	obj, err := e.s3.Put(ctx, key, body, int64(len(ct)), meta)
	if err != nil {
		return err
	}
	e.mu.Lock()
	e.bytesUp += int64(len(ct))
	e.mu.Unlock()
	return e.store.Put(ctx, store.Entry{
		Path:         path,
		LocalMtime:   local.Mtime.UTC(),
		LocalSize:    local.Size,
		LocalHash:    local.Hash,
		S3Key:        key,
		S3ETag:       obj.ETag,
		S3VersionID:  obj.VersionID,
		NonceB64:     meta["dropboy-nonce"],
		DEKNonceB64:  meta["dropboy-dek-nonce"],
		SealedDEKB64: meta["dropboy-dek"],
		LastSyncedAt: time.Now().UTC(),
	})
}

func (e *Engine) doDownload(ctx context.Context, path, key string) error {
	r, obj, err := e.s3.Get(ctx, key)
	if err != nil {
		return err
	}
	defer r.Close()
	ct, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	nonceB64 := obj.Metadata["dropboy-nonce"]
	dekNonceB64 := obj.Metadata["dropboy-dek-nonce"]
	sealedDEKB64 := obj.Metadata["dropboy-dek"]
	if nonceB64 == "" || sealedDEKB64 == "" {
		return errors.New("object missing encryption metadata")
	}
	nonce, _ := base64.StdEncoding.DecodeString(nonceB64)
	dekNonce, _ := base64.StdEncoding.DecodeString(dekNonceB64)
	sealedDEK, _ := base64.StdEncoding.DecodeString(sealedDEKB64)
	dek, err := dcrypto.UnwrapKey(e.master, dcrypto.SealedDEK{Nonce: dekNonce, Ciphertext: sealedDEK})
	if err != nil {
		return err
	}
	plain, err := dcrypto.OpenPayload(dek, nonce, ct)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, plain, 0o644); err != nil {
		return err
	}
	h := sha256.Sum256(plain)
	hash := hex.EncodeToString(h[:])
	info, _ := os.Stat(path)
	var mtime time.Time
	if info != nil {
		mtime = info.ModTime()
	}
	e.mu.Lock()
	e.bytesDown += int64(len(plain))
	e.mu.Unlock()
	return e.store.Put(ctx, store.Entry{
		Path: path, LocalMtime: mtime.UTC(), LocalSize: int64(len(plain)),
		LocalHash: hash, S3Key: key, S3ETag: obj.ETag, S3VersionID: obj.VersionID,
		NonceB64: nonceB64, DEKNonceB64: dekNonceB64, SealedDEKB64: sealedDEKB64,
		LastSyncedAt: time.Now().UTC(),
	})
}

func (e *Engine) recordConflict(path string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, c := range e.conflicts {
		if c.Path == path {
			return
		}
	}
	e.conflicts = append(e.conflicts, Conflict{
		ID:       fmt.Sprintf("c-%d", time.Now().UnixNano()),
		Path:     path,
		Detected: time.Now().UTC(),
		Local:    "local modified",
		Remote:   "remote modified",
	})
}

// ---- key helpers ----

func keyPrefix(machineID string) string {
	return "dropboy/v1/" + machineID + "/"
}

func keyFor(machineID, absPath string) string {
	clean := filepath.ToSlash(absPath)
	clean = strings.TrimPrefix(clean, "/")
	return keyPrefix(machineID) + clean
}

func pathFromKey(key, machineID string) string {
	prefix := keyPrefix(machineID)
	rel := strings.TrimPrefix(key, prefix)
	return "/" + rel
}
