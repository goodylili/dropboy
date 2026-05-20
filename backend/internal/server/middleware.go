package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// maxRequestBytes caps any single POST/PUT/PATCH body. 8 MiB is comfortably
// larger than any control-plane payload the API takes (settings, folder
// edits, conflict resolutions) without inviting DoS from a stray client.
const maxRequestBytes = 8 << 20

type ctxKey int

const ctxRequestID ctxKey = iota

// requestID generates a short hex token to tag each request in logs.
func requestID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func RequestIDFrom(ctx context.Context) string {
	if v, ok := ctx.Value(ctxRequestID).(string); ok {
		return v
	}
	return ""
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// requestLog records method, path, status, latency, and request ID.
func (s *Server) requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := requestID()
		ctx := context.WithValue(r.Context(), ctxRequestID, id)
		rec := &statusRecorder{ResponseWriter: w}
		start := time.Now()
		next.ServeHTTP(rec, r.WithContext(ctx))
		// Skip noisy event-stream pulses.
		if strings.HasSuffix(r.URL.Path, "/events") {
			return
		}
		slog.Debug("http",
			slog.String("id", id),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rec.status),
			slog.Int("bytes", rec.bytes),
			slog.Duration("dur", time.Since(start)),
		)
	})
}

// securityHeaders sets defense-in-depth headers on every response. The UI is
// loopback-only and same-origin, so CSP can be tight.
func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		h.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: blob:; "+
				"connect-src 'self'; "+
				"frame-ancestors 'none'; "+
				"base-uri 'self'")
		next.ServeHTTP(w, r)
	})
}

// limitBody rejects oversized request bodies on mutating methods.
func (s *Server) limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isMutating(r.Method) && r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
		}
		next.ServeHTTP(w, r)
	})
}

// rateLimiter is a tiny per-route, per-source token bucket. It is not a
// general-purpose limiter — its job is to slow brute-force attempts on
// /unlock and similar passphrase endpoints. The bucket is bounded by IP
// (always 127.0.0.1 on loopback) plus route, so distinct routes never share
// budget.
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64 // tokens per second
	burst   float64
}

type bucket struct {
	tokens float64
	last   time.Time
}

func newRateLimiter(rate, burst float64) *rateLimiter {
	return &rateLimiter{buckets: map[string]*bucket{}, rate: rate, burst: burst}
}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	b, ok := rl.buckets[key]
	if !ok {
		b = &bucket{tokens: rl.burst, last: now}
		rl.buckets[key] = b
	}
	b.tokens += now.Sub(b.last).Seconds() * rl.rate
	if b.tokens > rl.burst {
		b.tokens = rl.burst
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// rateLimit wraps a handler with a per-source token bucket. `name` namespaces
// the bucket so concurrent endpoints (e.g. /unlock and /sync) don't share
// budget.
func (s *Server) rateLimit(name string, rl *rateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, _ := strings.Cut(r.RemoteAddr, ":")
		if !rl.allow(name + "|" + host) {
			w.Header().Set("Retry-After", "30")
			writeJSONError(w, http.StatusTooManyRequests, "rate limited — slow down")
			return
		}
		next.ServeHTTP(w, r)
	})
}
