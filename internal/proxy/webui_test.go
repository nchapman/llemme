package proxy

import (
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestSpaFS(t *testing.T) {
	// Create a mock filesystem with some files
	mockFS := fstest.MapFS{
		"index.html":              {Data: []byte("<html>index</html>")},
		"favicon.ico":             {Data: []byte("icon")},
		"_next/static/chunk.js":   {Data: []byte("js code")},
		"_next/static/styles.css": {Data: []byte("css code")},
	}

	spa := &spaFS{fs: mockFS}

	tests := []struct {
		name        string
		path        string
		wantContent string
		wantErr     bool
	}{
		{
			name:        "serves index.html at root",
			path:        "index.html",
			wantContent: "<html>index</html>",
			wantErr:     false,
		},
		{
			name:        "serves existing static file",
			path:        "_next/static/chunk.js",
			wantContent: "js code",
			wantErr:     false,
		},
		{
			name:        "serves existing css file",
			path:        "_next/static/styles.css",
			wantContent: "css code",
			wantErr:     false,
		},
		{
			name:        "SPA fallback for route path",
			path:        "chat",
			wantContent: "<html>index</html>",
			wantErr:     false,
		},
		{
			name:        "SPA fallback for nested route",
			path:        "settings/profile",
			wantContent: "<html>index</html>",
			wantErr:     false,
		},
		{
			name:    "no fallback for missing JS file",
			path:    "missing.js",
			wantErr: true,
		},
		{
			name:    "no fallback for missing CSS file",
			path:    "missing.css",
			wantErr: true,
		},
		{
			name:    "no fallback for missing image",
			path:    "missing.png",
			wantErr: true,
		},
		{
			name:    "no fallback for v1 API path",
			path:    "v1/models",
			wantErr: true,
		},
		{
			name:    "no fallback for nested v1 API path",
			path:    "v1/chat/completions",
			wantErr: true,
		},
		{
			name:    "no fallback for api path",
			path:    "api/status",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := spa.Open(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Open(%q) expected error, got nil", tt.path)
				}
				return
			}
			if err != nil {
				t.Fatalf("Open(%q) unexpected error: %v", tt.path, err)
			}
			defer f.Close()

			content, err := io.ReadAll(f)
			if err != nil {
				t.Fatalf("ReadAll failed: %v", err)
			}
			if string(content) != tt.wantContent {
				t.Errorf("Open(%q) content = %q, want %q", tt.path, content, tt.wantContent)
			}
		})
	}
}

func TestSpaFSImplementsFSInterface(t *testing.T) {
	mockFS := fstest.MapFS{
		"index.html": {Data: []byte("<html></html>")},
	}

	// Verify spaFS implements fs.FS
	var _ fs.FS = &spaFS{fs: mockFS}
}

func TestAcceptsEncoding(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		encoding string
		want     bool
	}{
		{
			name:     "simple br",
			header:   "br",
			encoding: "br",
			want:     true,
		},
		{
			name:     "simple gzip",
			header:   "gzip",
			encoding: "gzip",
			want:     true,
		},
		{
			name:     "br in list",
			header:   "gzip, deflate, br",
			encoding: "br",
			want:     true,
		},
		{
			name:     "gzip in list",
			header:   "gzip, deflate, br",
			encoding: "gzip",
			want:     true,
		},
		{
			name:     "br with quality",
			header:   "br;q=1.0, gzip;q=0.8",
			encoding: "br",
			want:     true,
		},
		{
			name:     "gzip with quality",
			header:   "br;q=1.0, gzip;q=0.8",
			encoding: "gzip",
			want:     true,
		},
		{
			name:     "not present",
			header:   "gzip, deflate",
			encoding: "br",
			want:     false,
		},
		{
			name:     "empty header",
			header:   "",
			encoding: "br",
			want:     false,
		},
		{
			name:     "partial match should not match",
			header:   "gzip-br",
			encoding: "br",
			want:     false,
		},
		{
			name:     "identity only",
			header:   "identity",
			encoding: "gzip",
			want:     false,
		},
		{
			name:     "with spaces",
			header:   "gzip , br , deflate",
			encoding: "br",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := acceptsEncoding(tt.header, tt.encoding)
			if got != tt.want {
				t.Errorf("acceptsEncoding(%q, %q) = %v, want %v", tt.header, tt.encoding, got, tt.want)
			}
		})
	}
}

func TestSelectFile(t *testing.T) {
	mockFS := fstest.MapFS{
		"assets/app.js":       {Data: []byte("js")},
		"assets/app.js.br":    {Data: []byte("js-br")},
		"assets/app.js.gz":    {Data: []byte("js-gz")},
		"assets/style.css":    {Data: []byte("css")},
		"assets/style.css.gz": {Data: []byte("css-gz")},
		"assets/image.png":    {Data: []byte("png")},
		"index.html":          {Data: []byte("html")},
		"index.html.gz":       {Data: []byte("html-gz")},
	}

	server := &compressedFileServer{fs: mockFS}

	tests := []struct {
		name           string
		path           string
		acceptEncoding string
		wantPath       string
		wantEncoding   string
	}{
		{
			name:           "prefers brotli when available",
			path:           "assets/app.js",
			acceptEncoding: "gzip, br",
			wantPath:       "assets/app.js.br",
			wantEncoding:   "br",
		},
		{
			name:           "falls back to gzip",
			path:           "assets/style.css",
			acceptEncoding: "gzip, br",
			wantPath:       "assets/style.css.gz",
			wantEncoding:   "gzip",
		},
		{
			name:           "no compression when not accepted",
			path:           "assets/app.js",
			acceptEncoding: "identity",
			wantPath:       "assets/app.js",
			wantEncoding:   "",
		},
		{
			name:           "no compression for non-assets path",
			path:           "index.html",
			acceptEncoding: "gzip, br",
			wantPath:       "index.html",
			wantEncoding:   "",
		},
		{
			name:           "no compression when compressed file missing",
			path:           "assets/image.png",
			acceptEncoding: "gzip, br",
			wantPath:       "assets/image.png",
			wantEncoding:   "",
		},
		{
			name:           "gzip only when br not available",
			path:           "assets/style.css",
			acceptEncoding: "br",
			wantPath:       "assets/style.css",
			wantEncoding:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPath, gotEncoding := server.selectFile(tt.path, tt.acceptEncoding)
			if gotPath != tt.wantPath {
				t.Errorf("selectFile() path = %q, want %q", gotPath, tt.wantPath)
			}
			if gotEncoding != tt.wantEncoding {
				t.Errorf("selectFile() encoding = %q, want %q", gotEncoding, tt.wantEncoding)
			}
		})
	}
}

func TestSetCacheHeaders(t *testing.T) {
	server := &compressedFileServer{}

	tests := []struct {
		name      string
		path      string
		wantCache string
	}{
		{
			name:      "assets get immutable cache",
			path:      "assets/app.js",
			wantCache: "public, max-age=31536000, immutable",
		},
		{
			name:      "nested assets get immutable cache",
			path:      "assets/chunks/vendor.js",
			wantCache: "public, max-age=31536000, immutable",
		},
		{
			name:      "index.html gets no-cache",
			path:      "index.html",
			wantCache: "no-cache",
		},
		{
			name:      "other files get no cache header",
			path:      "favicon.ico",
			wantCache: "",
		},
		{
			name:      "root path gets no cache header",
			path:      "",
			wantCache: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			server.setCacheHeaders(rr, tt.path)
			got := rr.Header().Get("Cache-Control")
			if got != tt.wantCache {
				t.Errorf("setCacheHeaders(%q) Cache-Control = %q, want %q", tt.path, got, tt.wantCache)
			}
		})
	}
}

func TestCompressedFileServerHTTP(t *testing.T) {
	mockFS := fstest.MapFS{
		"index.html":       {Data: []byte("<html>test</html>")},
		"assets/app.js":    {Data: []byte("console.log('app')")},
		"assets/app.js.br": {Data: []byte("br-compressed-js")},
		"assets/app.js.gz": {Data: []byte("gz-compressed-js")},
	}

	server := &compressedFileServer{fs: mockFS}

	tests := []struct {
		name             string
		path             string
		acceptEncoding   string
		wantStatus       int
		wantBody         string
		wantEncoding     string
		wantCacheControl string
	}{
		{
			name:             "serves index.html at root",
			path:             "/",
			acceptEncoding:   "",
			wantStatus:       http.StatusOK,
			wantBody:         "<html>test</html>",
			wantEncoding:     "",
			wantCacheControl: "no-cache",
		},
		{
			name:             "serves brotli compressed asset",
			path:             "/assets/app.js",
			acceptEncoding:   "gzip, br",
			wantStatus:       http.StatusOK,
			wantBody:         "br-compressed-js",
			wantEncoding:     "br",
			wantCacheControl: "public, max-age=31536000, immutable",
		},
		{
			name:             "serves gzip when br not accepted",
			path:             "/assets/app.js",
			acceptEncoding:   "gzip",
			wantStatus:       http.StatusOK,
			wantBody:         "gz-compressed-js",
			wantEncoding:     "gzip",
			wantCacheControl: "public, max-age=31536000, immutable",
		},
		{
			name:             "serves uncompressed when no encoding accepted",
			path:             "/assets/app.js",
			acceptEncoding:   "",
			wantStatus:       http.StatusOK,
			wantBody:         "console.log('app')",
			wantEncoding:     "",
			wantCacheControl: "public, max-age=31536000, immutable",
		},
		{
			name:           "returns 404 for missing file",
			path:           "/missing.js",
			acceptEncoding: "",
			wantStatus:     http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.acceptEncoding != "" {
				req.Header.Set("Accept-Encoding", tt.acceptEncoding)
			}
			rr := httptest.NewRecorder()

			server.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tt.wantStatus)
			}

			if tt.wantStatus != http.StatusOK {
				return
			}

			if body := rr.Body.String(); body != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}

			if got := rr.Header().Get("Content-Encoding"); got != tt.wantEncoding {
				t.Errorf("Content-Encoding = %q, want %q", got, tt.wantEncoding)
			}

			if got := rr.Header().Get("Cache-Control"); got != tt.wantCacheControl {
				t.Errorf("Cache-Control = %q, want %q", got, tt.wantCacheControl)
			}

			if tt.wantEncoding != "" {
				if got := rr.Header().Get("Vary"); got != "Accept-Encoding" {
					t.Errorf("Vary = %q, want %q", got, "Accept-Encoding")
				}
			}
		})
	}
}
