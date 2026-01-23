package proxy

import (
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/nchapman/lleme/web"
)

// spaFS wraps an fs.FS to support SPA routing by falling back to index.html
// for paths that don't exist (unless they're API paths).
type spaFS struct {
	fs fs.FS
}

func (s *spaFS) Open(name string) (fs.File, error) {
	f, err := s.fs.Open(name)
	if err == nil {
		return f, nil
	}

	if strings.HasPrefix(name, "v1/") || strings.HasPrefix(name, "api/") {
		return nil, err
	}

	if strings.Contains(name, ".") {
		return nil, err
	}

	return s.fs.Open("index.html")
}

// compressedFileServer serves pre-compressed .br/.gz files when available.
type compressedFileServer struct {
	fs fs.FS
}

func (c *compressedFileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}

	acceptEncoding := r.Header.Get("Accept-Encoding")
	servedPath, encoding := c.selectFile(path, acceptEncoding)

	f, err := c.fs.Open(servedPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	stat, err := f.Stat()
	if err != nil {
		f.Close()
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if stat.IsDir() {
		f.Close()
		servedPath = filepath.Join(path, "index.html")
		servedPath, encoding = c.selectFile(servedPath, acceptEncoding)
		f, err = c.fs.Open(servedPath)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		stat, err = f.Stat()
		if err != nil {
			f.Close()
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}
	defer f.Close()

	contentType := mime.TypeByExtension(filepath.Ext(path))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)

	if encoding != "" {
		w.Header().Set("Content-Encoding", encoding)
		w.Header().Set("Vary", "Accept-Encoding")
	}

	c.setCacheHeaders(w, path)

	rs, ok := f.(io.ReadSeeker)
	if ok {
		http.ServeContent(w, r, path, stat.ModTime(), rs)
	} else {
		io.Copy(w, f)
	}
}

func (c *compressedFileServer) selectFile(path, acceptEncoding string) (string, string) {
	if !strings.HasPrefix(path, "assets/") {
		return path, ""
	}

	if acceptsEncoding(acceptEncoding, "br") {
		brPath := path + ".br"
		if _, err := fs.Stat(c.fs, brPath); err == nil {
			return brPath, "br"
		}
	}

	if acceptsEncoding(acceptEncoding, "gzip") {
		gzPath := path + ".gz"
		if _, err := fs.Stat(c.fs, gzPath); err == nil {
			return gzPath, "gzip"
		}
	}

	return path, ""
}

func acceptsEncoding(header, encoding string) bool {
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if idx := strings.Index(part, ";"); idx != -1 {
			part = part[:idx]
		}
		if part == encoding {
			return true
		}
	}
	return false
}

func (c *compressedFileServer) setCacheHeaders(w http.ResponseWriter, path string) {
	if strings.HasPrefix(path, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else if path == "index.html" {
		w.Header().Set("Cache-Control", "no-cache")
	}
}

// newWebUIHandler creates an http.Handler that serves the embedded web UI.
func newWebUIHandler() http.Handler {
	distFS, err := web.DistFS()
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Web UI not available", http.StatusInternalServerError)
		})
	}

	return &compressedFileServer{fs: &spaFS{fs: distFS}}
}
