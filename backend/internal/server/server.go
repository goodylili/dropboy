// Package server hosts the loopback HTTP API + event stream that the local
// Next.js UI talks to (PRD §5.8). The same JSON surface is intended to back
// the CLI over a Unix-domain socket; that wiring lives in the daemon package.
//
// Live event streaming uses Server-Sent Events on /api/v1/events rather than
// the WebSocket called out in PRD §5.8. SSE is one-way (server → browser),
// which fits every listed use case (status, queue, throughput, log lines) and
// avoids a third-party dependency in the Go binary.
package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/goodylili/dropboy/internal/config"
)

type Server struct {
	Port    int
	Cfg     config.Config
	DataDir string // ~/.dropboy

	mu       sync.RWMutex
	token    string
	events   *eventHub
	mux      *http.ServeMux
	engine   Engine
	unlockRL *rateLimiter
	mutateRL *rateLimiter
}

// Engine is the surface the HTTP server consumes from the daemon. The daemon
// implements this interface; defining it here keeps server/ free of any
// daemon import (otherwise we'd have a cycle).
type Engine interface {
	Stats() EngineStats
	Conflicts() []Conflict
	ResolveConflict(id string) bool
	KickSync() error
	Paused() bool
	SetPaused(bool)
	Locked() bool
	Unlock(passphrase string, remember bool) error
	ForgetPassphrase() error
}

// EngineStats mirrors sync.Stats but lives in server/ to avoid the cycle.
type EngineStats struct {
	LastRun    time.Time
	QueueUp    int
	QueueDown  int
	BytesUp    int64
	BytesDown  int64
	Conflicts  int
	Paused     bool
	EngineLive bool
}

// SetEngine wires the daemon's engine adapter into the server.
func (s *Server) SetEngine(e Engine) {
	s.mu.Lock()
	s.engine = e
	s.mu.Unlock()
}

func New(cfg config.Config, dataDir string) *Server {
	port := cfg.UI.Port
	if port == 0 {
		port = 7777
	}
	return &Server{
		Port:     port,
		Cfg:      cfg,
		DataDir:  dataDir,
		events:   newEventHub(),
		unlockRL: newRateLimiter(0.2, 5),
		mutateRL: newRateLimiter(20, 40),
	}
}

// Token returns the active UI session token. It is rotated each time the
// server starts and written to <DataDir>/ui.token (mode 0600).
func (s *Server) Token() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.token
}

// Start binds 127.0.0.1:port and serves until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	tok, err := loadOrCreateToken(s.DataDir)
	if err != nil {
		return fmt.Errorf("session token: %w", err)
	}
	s.mu.Lock()
	s.token = tok
	s.mu.Unlock()

	mux := http.NewServeMux()
	s.registerRoutes(mux)
	s.mux = mux

	handler := s.recoverer(
		s.requestLog(
			s.securityHeaders(
				s.hostGuard(
					s.cors(
						s.limitBody(mux))))))

	addr := fmt.Sprintf("127.0.0.1:%d", s.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go s.events.run(ctx)
	go s.tickStatus(ctx)

	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(listener) }()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
		return nil
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
