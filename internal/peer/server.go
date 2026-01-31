package peer

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nchapman/lleme/internal/config"
	"github.com/nchapman/lleme/internal/logs"
)

// Server handles peer-to-peer model sharing HTTP endpoints.
// Uses hash-based requests for privacy - peers cannot list available models.
type Server struct {
	httpServer *http.Server
	port       int
	hashIndex  *HashIndex
}

// NewServer creates a new peer sharing server.
func NewServer(port int) *Server {
	s := &Server{
		port:      port,
		hashIndex: NewHashIndex(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/peer/sha256/", s.handleHashDownload)

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", port),
		Handler: mux,
	}

	return s
}

// Start starts the peer server and loads the hash index.
func (s *Server) Start() error {
	// Rebuild index if file doesn't exist, then load it
	if _, err := os.Stat(IndexFilePath()); os.IsNotExist(err) {
		if err := RebuildIndex(); err != nil {
			logs.Warn("Failed to rebuild hash index", "error", err)
		}
	}
	if err := s.hashIndex.Load(); err != nil {
		logs.Warn("Failed to load hash index", "error", err)
	}

	ln, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.httpServer.Addr, err)
	}

	go func() {
		if err := s.httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			logs.Warn("Peer server error", "error", err)
		}
	}()

	logs.Debug("Peer server started", "addr", s.httpServer.Addr, "indexed_files", s.hashIndex.Count())
	return nil
}

// Stop gracefully stops the peer server.
func (s *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}

// Port returns the port the server is listening on.
func (s *Server) Port() int {
	return s.port
}

// ReloadIndex reloads the hash index from disk.
func (s *Server) ReloadIndex() error {
	return s.hashIndex.Load()
}

// handleHashDownload serves a file by its SHA256 hash.
// Endpoint: /api/peer/sha256/{hash}
// Methods: HEAD (check availability + get size), GET (download file)
func (s *Server) handleHashDownload(w http.ResponseWriter, r *http.Request) {
	// Fail fast for unsupported methods
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse hash from URL: /api/peer/sha256/{hash}
	hash := strings.TrimPrefix(r.URL.Path, "/api/peer/sha256/")
	if hash == "" || len(hash) != 64 {
		http.Error(w, "Invalid hash", http.StatusBadRequest)
		return
	}

	// Validate hash is hexadecimal (De Morgan's law applied for linter)
	for _, c := range hash {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			http.Error(w, "Invalid hash format", http.StatusBadRequest)
			return
		}
	}

	// Normalize to lowercase for lookup
	hash = strings.ToLower(hash)

	// Look up file path in index
	filePath := s.hashIndex.Lookup(hash)
	if filePath == "" {
		http.NotFound(w, r)
		return
	}

	// Verify path is under models directory (defense in depth)
	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	absModelsDir, err := filepath.Abs(config.ModelsPath())
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !strings.HasPrefix(absFilePath, absModelsDir+string(filepath.Separator)) {
		http.Error(w, "Invalid file path", http.StatusBadRequest)
		return
	}

	// Get file info
	info, err := os.Stat(filePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Set headers
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	w.Header().Set("X-Model-SHA256", hash)
	w.Header().Set("Content-Type", "application/octet-stream")

	if r.Method == http.MethodHead {
		return
	}

	// Serve the file with range support
	http.ServeFile(w, r, filePath)
}
