package server

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

// uiAssets holds the Next.js static export at frontend/out. The directory is
// embedded lazily — if no build is present (e.g. dev clones), the embed is an
// empty FS and spaHandler returns a friendly placeholder so the API stays
// fully usable on its own.
//
//go:embed all:assets
var uiAssets embed.FS

func (s *Server) spaHandler() http.Handler {
	sub, err := fs.Sub(uiAssets, "assets")
	if err != nil || !hasIndexHTML(sub) {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = w.Write([]byte(devShellHTML))
				return
			}
			http.NotFound(w, r)
		})
	}
	file := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// SPA fallback: anything without a file extension that isn't /api
		// gets index.html so client-side routing works.
		if !strings.HasPrefix(r.URL.Path, "/api/") && !strings.Contains(r.URL.Path[1:], ".") {
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			file.ServeHTTP(w, r2)
			return
		}
		file.ServeHTTP(w, r)
	})
}

func hasIndexHTML(f fs.FS) bool {
	if f == nil {
		return false
	}
	_, err := fs.Stat(f, "index.html")
	return err == nil
}

const devShellHTML = `<!doctype html>
<html><head><meta charset="utf-8"><title>dropboy</title>
<style>body{font-family:ui-sans-serif,system-ui,sans-serif;max-width:640px;margin:6rem auto;padding:0 1.5rem;line-height:1.5;color:#222}code{background:#f4f4f5;padding:.15rem .35rem;border-radius:4px}</style>
</head><body>
<h1>dropboy</h1>
<p>The daemon is running and the API is live at <code>/api/v1</code>. The web UI bundle hasn't been embedded into this binary yet.</p>
<p>For development, run the Next.js dev server:</p>
<pre><code>cd frontend &amp;&amp; npm run dev</code></pre>
<p>Then open <a href="http://localhost:3000">http://localhost:3000</a>. To bake the UI into the binary, run <code>npm run build</code> in <code>frontend/</code> and copy the export into <code>backend/internal/server/assets/</code> before <code>go build</code>.</p>
</body></html>`
