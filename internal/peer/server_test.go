package peer

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNewServer(t *testing.T) {
	s := NewServer(11314)
	if s == nil {
		t.Fatal("NewServer returned nil")
	}
	if s.port != 11314 {
		t.Errorf("expected port 11314, got %d", s.port)
	}
	if s.hashIndex == nil {
		t.Error("hashIndex should be initialized")
	}
}

func TestServerPort(t *testing.T) {
	s := NewServer(12345)
	if s.Port() != 12345 {
		t.Errorf("expected port 12345, got %d", s.Port())
	}
}

func TestHandleHashDownloadInvalidMethod(t *testing.T) {
	s := NewServer(11314)

	req := httptest.NewRequest(http.MethodPost, "/api/peer/sha256/abc123", nil)
	w := httptest.NewRecorder()

	s.handleHashDownload(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestHandleHashDownloadInvalidHashLength(t *testing.T) {
	s := NewServer(11314)

	tests := []struct {
		name string
		hash string
	}{
		{"too short", "abc123"},
		{"too long", "abcdef1234567890abcdef1234567890abcdef1234567890abcdef12345678901"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/peer/sha256/"+tt.hash, nil)
			w := httptest.NewRecorder()

			s.handleHashDownload(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
			}
		})
	}
}

func TestHandleHashDownloadInvalidHashFormat(t *testing.T) {
	s := NewServer(11314)

	// 64 characters but contains invalid hex chars
	invalidHash := "ghijkl1234567890ghijkl1234567890ghijkl1234567890ghijkl1234567890"

	req := httptest.NewRequest(http.MethodGet, "/api/peer/sha256/"+invalidHash, nil)
	w := httptest.NewRecorder()

	s.handleHashDownload(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandleHashDownloadNotFound(t *testing.T) {
	s := NewServer(11314)

	// Valid hash format but not in index
	hash := "0000000000000000000000000000000000000000000000000000000000000000"

	req := httptest.NewRequest(http.MethodGet, "/api/peer/sha256/"+hash, nil)
	w := httptest.NewRecorder()

	s.handleHashDownload(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestHandleHashDownloadHEAD(t *testing.T) {
	// Skip if models directory doesn't exist (can't test path traversal validation)
	modelsDir := os.ExpandEnv("$HOME/.lleme/models")
	if _, err := os.Stat(modelsDir); os.IsNotExist(err) {
		t.Skip("No models directory, skipping integration test")
	}

	s := NewServer(11314)

	// Create a temp file under the models directory
	tmpFile := filepath.Join(modelsDir, "test-peer-server.gguf")
	content := []byte("test model content")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile)

	// Add to index
	hash := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	s.hashIndex.index[hash] = tmpFile

	req := httptest.NewRequest(http.MethodHead, "/api/peer/sha256/"+hash, nil)
	w := httptest.NewRecorder()

	s.handleHashDownload(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Check headers
	if w.Header().Get("Content-Length") != "18" {
		t.Errorf("expected Content-Length 18, got %s", w.Header().Get("Content-Length"))
	}
	if w.Header().Get("X-Model-SHA256") != hash {
		t.Errorf("expected X-Model-SHA256 %s, got %s", hash, w.Header().Get("X-Model-SHA256"))
	}
	if w.Header().Get("Content-Type") != "application/octet-stream" {
		t.Errorf("expected Content-Type application/octet-stream, got %s", w.Header().Get("Content-Type"))
	}

	// HEAD should not return body
	if w.Body.Len() != 0 {
		t.Errorf("HEAD should not return body, got %d bytes", w.Body.Len())
	}
}

func TestHandleHashDownloadGET(t *testing.T) {
	// Skip if models directory doesn't exist
	modelsDir := os.ExpandEnv("$HOME/.lleme/models")
	if _, err := os.Stat(modelsDir); os.IsNotExist(err) {
		t.Skip("No models directory, skipping integration test")
	}

	s := NewServer(11314)

	// Create a temp file under the models directory
	tmpFile := filepath.Join(modelsDir, "test-peer-server-get.gguf")
	content := []byte("test model content for GET")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile)

	// Add to index
	hash := "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321"
	s.hashIndex.index[hash] = tmpFile

	req := httptest.NewRequest(http.MethodGet, "/api/peer/sha256/"+hash, nil)
	w := httptest.NewRecorder()

	s.handleHashDownload(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// GET should return body
	if w.Body.String() != string(content) {
		t.Errorf("expected body %q, got %q", string(content), w.Body.String())
	}
}

func TestHandleHashDownloadCaseNormalization(t *testing.T) {
	// Skip if models directory doesn't exist
	modelsDir := os.ExpandEnv("$HOME/.lleme/models")
	if _, err := os.Stat(modelsDir); os.IsNotExist(err) {
		t.Skip("No models directory, skipping integration test")
	}

	s := NewServer(11314)

	// Create a temp file under the models directory
	tmpFile := filepath.Join(modelsDir, "test-peer-server-case.gguf")
	if err := os.WriteFile(tmpFile, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile)

	// Add to index with lowercase hash
	hash := "aabbcc1234567890aabbcc1234567890aabbcc1234567890aabbcc1234567890"
	s.hashIndex.index[hash] = tmpFile

	// Request with uppercase hash should work (normalized to lowercase)
	upperHash := "AABBCC1234567890AABBCC1234567890AABBCC1234567890AABBCC1234567890"
	req := httptest.NewRequest(http.MethodHead, "/api/peer/sha256/"+upperHash, nil)
	w := httptest.NewRecorder()

	s.handleHashDownload(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d for uppercase hash, got %d", http.StatusOK, w.Code)
	}

	// Mixed case should also work
	mixedHash := "AaBbCc1234567890AaBbCc1234567890AaBbCc1234567890AaBbCc1234567890"
	req = httptest.NewRequest(http.MethodHead, "/api/peer/sha256/"+mixedHash, nil)
	w = httptest.NewRecorder()

	s.handleHashDownload(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d for mixed case hash, got %d", http.StatusOK, w.Code)
	}
}

func TestHandleHashDownloadFileNotExists(t *testing.T) {
	// Skip if models directory doesn't exist
	modelsDir := os.ExpandEnv("$HOME/.lleme/models")
	if _, err := os.Stat(modelsDir); os.IsNotExist(err) {
		t.Skip("No models directory, skipping integration test")
	}

	s := NewServer(11314)

	// Add to index but file doesn't exist (but path is under models dir)
	hash := "1111111111111111111111111111111111111111111111111111111111111111"
	s.hashIndex.index[hash] = filepath.Join(modelsDir, "nonexistent-model.gguf")

	req := httptest.NewRequest(http.MethodHead, "/api/peer/sha256/"+hash, nil)
	w := httptest.NewRecorder()

	s.handleHashDownload(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d for missing file, got %d", http.StatusNotFound, w.Code)
	}
}

func TestHandleHashDownloadPathTraversal(t *testing.T) {
	s := NewServer(11314)

	// Add to index with path outside models directory (defense in depth test)
	hash := "2222222222222222222222222222222222222222222222222222222222222222"
	s.hashIndex.index[hash] = "/etc/passwd"

	req := httptest.NewRequest(http.MethodHead, "/api/peer/sha256/"+hash, nil)
	w := httptest.NewRecorder()

	s.handleHashDownload(w, req)

	// Should be rejected - either 400 (invalid path) or 404 (not found under models)
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 400 or 404 for path traversal attempt, got %d", w.Code)
	}
}
