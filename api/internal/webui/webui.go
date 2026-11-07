// Package webui serves the SvelteKit single-page app embedded at build time.
// `make build` (and the Dockerfile) copy web/build into dist/ before
// compiling. Without it the binary still builds and serves a 503 page.
package webui

import (
	"bytes"
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

//go:embed all:dist
var dist embed.FS

// Handler serves the prerendered pages (/, /d, /a) and static assets. Hashed
// assets under _app/immutable are cached for a year; everything else must
// revalidate. Unknown paths get the SPA fallback with a 404 status.
func Handler() http.Handler {
	sub, _ := fs.Sub(dist, "dist")
	return New(sub)
}

func New(files fs.FS) http.Handler {
	fallback, err := fs.ReadFile(files, "200.html")
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "UI not built: run `make build`", http.StatusServiceUnavailable)
		})
	}
	modTime := time.Now()
	page := func(w http.ResponseWriter, r *http.Request, status int, name string, body []byte) {
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if status != http.StatusOK {
			w.WriteHeader(status)
			_, _ = w.Write(body)
			return
		}
		http.ServeContent(w, r, name, modTime, bytes.NewReader(body))
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "" {
			name = "index"
		}
		// Prerendered routes: /d → d.html, / → index.html.
		if !strings.Contains(path.Base(name), ".") {
			if body, err := fs.ReadFile(files, name+".html"); err == nil {
				page(w, r, http.StatusOK, name+".html", body)
				return
			}
		} else if st, err := fs.Stat(files, name); err == nil && !st.IsDir() && !strings.HasSuffix(name, ".html") {
			if strings.HasPrefix(name, "_app/immutable/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache")
			}
			http.ServeFileFS(w, r, files, name)
			return
		}
		// Unknown path: the SPA fallback renders SvelteKit's 404 page.
		page(w, r, http.StatusNotFound, "200.html", fallback)
	})
}
