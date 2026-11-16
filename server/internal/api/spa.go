package api

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// spaCSP is the policy for the screens (the API keeps its own, which forbids everything): the app's own scripts, styles,
// images and calls, nothing from elsewhere, and never inside a frame.
const spaCSP = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self' data:; connect-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'"

// SPA serves the built web app from dir: the file asked for when there is one, otherwise index.html, so a link to a
// screen (#/p/<id>/compare is the same page, but a refresh on /anything must not be a 404) opens the app. It answers
// GET and HEAD only and never for /api/ or /auth/, which stay JSON.
func SPA(dir string) http.Handler {
	root, err := filepath.Abs(dir)
	if err != nil {
		root = dir
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		p := path.Clean("/" + r.URL.Path)
		if p == "/api" || strings.HasPrefix(p, "/api/") || p == "/auth" || strings.HasPrefix(p, "/auth/") {
			http.NotFound(w, r)
			return
		}
		file := filepath.Join(root, filepath.FromSlash(p))
		// Stay inside the folder whatever the path says.
		if rel, err := filepath.Rel(root, file); err != nil || strings.HasPrefix(rel, "..") {
			http.NotFound(w, r)
			return
		}
		info, err := os.Stat(file)
		if err != nil || info.IsDir() {
			// A missing file with an extension is a missing file; a path without one is a screen of the app.
			if path.Ext(p) != "" && p != "/index.html" {
				http.NotFound(w, r)
				return
			}
			file = filepath.Join(root, "index.html")
			if info, err = os.Stat(file); err != nil {
				http.NotFound(w, r)
				return
			}
		}
		f, err := os.Open(file)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		h := w.Header()
		h.Set("Content-Security-Policy", spaCSP)
		if strings.HasPrefix(p, "/assets/") {
			// Built files carry a hash in their name, so they never change.
			h.Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			h.Set("Cache-Control", "no-cache")
		}
		http.ServeContent(w, r, filepath.Base(file), info.ModTime(), f)
	})
}
