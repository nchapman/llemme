package proxy

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/nchapman/lleme/web"
)

// spaFS wraps an fs.FS to support SPA routing by falling back to index.html
// for paths that don't exist (unless they're API paths).
type spaFS struct {
	fs fs.FS
}

func (s *spaFS) Open(name string) (fs.File, error) {
	// Try to open the requested file
	f, err := s.fs.Open(name)
	if err == nil {
		return f, nil
	}

	// Don't fallback for API paths
	if strings.HasPrefix(name, "v1/") || strings.HasPrefix(name, "api/") {
		return nil, err
	}

	// Don't fallback for paths with file extensions (static assets should 404)
	if strings.Contains(name, ".") {
		return nil, err
	}

	// SPA fallback: serve index.html for route paths
	return s.fs.Open("index.html")
}

// newWebUIHandler creates an http.Handler that serves the embedded web UI.
func newWebUIHandler() http.Handler {
	distFS, err := web.DistFS()
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Web UI not available", http.StatusInternalServerError)
		})
	}

	return http.FileServer(http.FS(&spaFS{fs: distFS}))
}
