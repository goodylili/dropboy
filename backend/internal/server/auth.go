package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	tokenFile     = "ui.token"
	authHeader    = "Authorization"
	bearerPrefix  = "Bearer "
	tokenQuery    = "token"
	csrfHeader    = "X-Dropboy-CSRF"
	sessionCookie = "dropboy_session"
)

func loadOrCreateToken(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	p := filepath.Join(dir, tokenFile)
	if data, err := os.ReadFile(p); err == nil {
		t := strings.TrimSpace(string(data))
		if len(t) >= 32 {
			return t, nil
		}
	}
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	tok := hex.EncodeToString(buf[:])
	if err := os.WriteFile(p, []byte(tok+"\n"), 0o600); err != nil {
		return "", err
	}
	return tok, nil
}

// authMiddleware enforces a valid session token on /api/* requests.
// The token can arrive as Authorization: Bearer <tok>, a session cookie, or
// the ?token=<tok> query string (for the initial browser handoff from
// `dropboy ui --open`, after which a Set-Cookie locks the session in).
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected := s.Token()
		if expected == "" {
			writeJSONError(w, http.StatusServiceUnavailable, "server initializing")
			return
		}

		got := ""
		if h := r.Header.Get(authHeader); strings.HasPrefix(h, bearerPrefix) {
			got = strings.TrimPrefix(h, bearerPrefix)
		}
		if got == "" {
			if c, err := r.Cookie(sessionCookie); err == nil {
				got = c.Value
			}
		}
		if got == "" {
			got = r.URL.Query().Get(tokenQuery)
		}

		if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(expected)) != 1 {
			writeJSONError(w, http.StatusUnauthorized, "invalid or missing token")
			return
		}

		// Promote query-string handoff to a cookie so the SPA does not have
		// to keep the token in the URL.
		if r.URL.Query().Get(tokenQuery) != "" {
			http.SetCookie(w, &http.Cookie{
				Name:     sessionCookie,
				Value:    expected,
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteStrictMode,
				MaxAge:   60 * 60 * 24 * 7,
			})
		}

		// CSRF: state-changing methods must echo the token in a custom
		// header. The SPA reads it from the cookie/local storage and sets
		// the header; a cross-site form post cannot set custom headers.
		if isMutating(r.Method) {
			if subtle.ConstantTimeCompare([]byte(r.Header.Get(csrfHeader)), []byte(expected)) != 1 {
				writeJSONError(w, http.StatusForbidden, "missing or invalid CSRF token")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func isMutating(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

// hostGuard rejects requests whose Host header is not a loopback name. This
// defeats DNS-rebinding attacks against the local UI.
func (s *Server) hostGuard(next http.Handler) http.Handler {
	allowed := map[string]struct{}{
		fmt.Sprintf("127.0.0.1:%d", s.Port): {},
		fmt.Sprintf("localhost:%d", s.Port): {},
		fmt.Sprintf("[::1]:%d", s.Port):     {},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if _, ok := allowed[host]; !ok {
			// Allow bare host names too (some proxies strip the port).
			bare := host
			if i := strings.IndexByte(host, ':'); i >= 0 {
				bare = host[:i]
			}
			if bare != "127.0.0.1" && bare != "localhost" && bare != "::1" {
				writeJSONError(w, http.StatusForbidden, "host not allowed")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// cors permits the Next.js dev server on http://localhost:3000 to reach the
// API. In production the SPA is same-origin (served from embed.FS) so this
// only fires during development.
func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "http://localhost:3000" || origin == "http://127.0.0.1:3000" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, "+csrfHeader)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
