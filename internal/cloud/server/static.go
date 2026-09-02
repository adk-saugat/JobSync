package server

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

// Built by `npm run build` in web/ and copied here during Docker image build.
//
//go:embed all:webdist
var webDist embed.FS

func (s *Server) spaHandler() http.Handler {
	sub, err := fs.Sub(webDist, "webdist")
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "web ui not embedded", http.StatusNotFound)
		})
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(sub, path); err != nil {
			// SPA fallback
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
