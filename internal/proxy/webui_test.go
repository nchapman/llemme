package proxy

import (
	"io"
	"io/fs"
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
