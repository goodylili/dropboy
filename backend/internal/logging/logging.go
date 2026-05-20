// Package logging configures the application logger.
//
// In foreground use (CLI commands, `dropboy start --foreground` with a TTY)
// logs go to stderr. When the daemon runs under launchd, stdout/stderr are
// redirected by the plist; we additionally tee to a size-capped file at
// ~/Library/Logs/dropboy/dropboy.log so `dropboy logs` works without going
// through Console.app.
package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

const (
	maxLogBytes  = 10 << 20 // 10 MiB before rotation
	logKeepCount = 3        // dropboy.log + .1 + .2
)

func Setup(verbose bool) *slog.Logger {
	return setup(verbose, defaultLogFile())
}

func setup(verbose bool, logPath string) *slog.Logger {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}

	// Under launchd, stderr is already redirected to dropboy.log by the
	// plist, so adding a file tee would double-write every line. Only tee
	// when stderr looks like a terminal (interactive `dropboy` commands or
	// `start --foreground` from a shell).
	var w io.Writer = os.Stderr
	if logPath != "" && isStderrTerminal() {
		if rw, err := openRotating(logPath); err == nil {
			w = io.MultiWriter(os.Stderr, rw)
		}
	}

	h := slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})
	logger := slog.New(h)
	slog.SetDefault(logger)
	return logger
}

func isStderrTerminal() bool {
	fi, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func defaultLogFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "Logs", "dropboy", "dropboy.log")
}

// rotatingWriter is a size-based rotator. When the current file exceeds
// maxLogBytes it renames .log → .log.1, .log.1 → .log.2, and drops .log.2.
// Keeping the implementation in-tree avoids a lumberjack dependency.
type rotatingWriter struct {
	mu   sync.Mutex
	path string
	f    *os.File
	n    int64
}

func openRotating(path string) (*rotatingWriter, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	st, _ := f.Stat()
	var n int64
	if st != nil {
		n = st.Size()
	}
	return &rotatingWriter{path: path, f: f, n: n}, nil
}

func (r *rotatingWriter) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.n+int64(len(p)) > maxLogBytes {
		_ = r.rotate()
	}
	n, err := r.f.Write(p)
	r.n += int64(n)
	return n, err
}

func (r *rotatingWriter) rotate() error {
	_ = r.f.Close()
	for i := logKeepCount - 1; i >= 1; i-- {
		src := r.path
		if i > 1 {
			src = r.path + "." + strconv.Itoa(i-1)
		}
		dst := r.path + "." + strconv.Itoa(i)
		_ = os.Rename(src, dst)
	}
	f, err := os.OpenFile(r.path, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		r.f, _ = os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		r.n = 0
		return err
	}
	r.f = f
	r.n = 0
	return nil
}
