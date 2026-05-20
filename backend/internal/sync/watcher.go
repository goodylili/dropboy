package sync

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/goodylili/dropboy/internal/config"
)

// Watcher debounces fsnotify events into a single "something changed" tick
// the daemon can act on by kicking a reconcile pass. Linux's lack of
// recursive watches is handled by adding every subdirectory on startup; new
// directories observed at runtime are watched on the fly.
type Watcher struct {
	cfg      config.Config
	debounce time.Duration
}

func NewWatcher(cfg config.Config) *Watcher {
	return &Watcher{cfg: cfg, debounce: 750 * time.Millisecond}
}

// Run streams change ticks to out until ctx is cancelled.
func (w *Watcher) Run(ctx context.Context, out chan<- struct{}) error {
	notifier, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer notifier.Close()

	for _, f := range w.cfg.Folders {
		_ = filepath.WalkDir(f.Path, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				_ = notifier.Add(p)
			}
			return nil
		})
	}

	tick := time.NewTimer(time.Hour)
	tick.Stop()
	armed := false

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-notifier.Events:
			if !ok {
				return nil
			}
			if ev.Op&fsnotify.Create != 0 {
				if st, err := os.Stat(ev.Name); err == nil && st.IsDir() {
					_ = notifier.Add(ev.Name)
				}
			}
			if !armed {
				armed = true
				tick.Reset(w.debounce)
			}
		case <-notifier.Errors:
			// surface in doctor later
		case <-tick.C:
			armed = false
			select {
			case out <- struct{}{}:
			default:
			}
		}
	}
}
