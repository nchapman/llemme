package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/nchapman/llemme/internal/config"
)

const Version = "0.2.0"

// Server is the main proxy server that routes requests to backends
type Server struct {
	mu           sync.RWMutex
	httpServer   *http.Server
	manager      *ModelManager
	idleMonitor  *IdleMonitor
	config       *Config
	startedAt    time.Time
	shutdownChan chan struct{}
}

// NewServer creates a new proxy server
func NewServer(cfg *Config, appCfg *config.Config) *Server {
	manager := NewModelManager(cfg, appCfg)

	s := &Server{
		manager:      manager,
		config:       cfg,
		startedAt:    time.Now(),
		shutdownChan: make(chan struct{}),
	}

	// Create idle monitor
	s.idleMonitor = NewIdleMonitor(manager, cfg.IdleTimeout, 60*time.Second)

	// Setup HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", s.handleChatCompletions)
	mux.HandleFunc("/v1/completions", s.handleCompletions)
	mux.HandleFunc("/v1/embeddings", s.handleEmbeddings)
	mux.HandleFunc("/v1/models", s.handleModels)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/stop", s.handleStopModel)
	mux.HandleFunc("/api/stop-all", s.handleStopAll)

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler: mux,
	}

	return s
}

// Start starts the proxy server
func (s *Server) Start() error {
	// Start idle monitor
	s.idleMonitor.Start()

	ln, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.httpServer.Addr, err)
	}

	go func() {
		if err := s.httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Server error: %v\n", err)
		}
	}()

	return nil
}

// Stop gracefully stops the proxy server
func (s *Server) Stop() error {
	close(s.shutdownChan)

	// Stop idle monitor
	s.idleMonitor.Stop()

	// Stop all backends
	s.manager.StopAllBackends()

	// Shutdown HTTP server with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return s.httpServer.Shutdown(ctx)
}

// Manager returns the model manager
func (s *Server) Manager() *ModelManager {
	return s.manager
}

// Addr returns the address the server is listening on
func (s *Server) Addr() string {
	return s.httpServer.Addr
}

// handleChatCompletions proxies chat completion requests to the appropriate backend
func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	s.proxyToBackend(w, r, "/v1/chat/completions")
}

// handleCompletions proxies completion requests to the appropriate backend
func (s *Server) handleCompletions(w http.ResponseWriter, r *http.Request) {
	s.proxyToBackend(w, r, "/v1/completions")
}

// handleEmbeddings proxies embedding requests to the appropriate backend
func (s *Server) handleEmbeddings(w http.ResponseWriter, r *http.Request) {
	s.proxyToBackend(w, r, "/v1/embeddings")
}

// proxyToBackend handles the common logic of extracting model and proxying
func (s *Server) proxyToBackend(w http.ResponseWriter, r *http.Request, path string) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST is allowed")
		return
	}

	// Read and parse body to get model
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Failed to read request body")
		return
	}
	r.Body.Close()

	var req struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Failed to parse request body")
		return
	}

	if req.Model == "" {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Model field is required")
		return
	}

	// Get or load the backend
	backend, err := s.manager.GetOrLoadBackend(req.Model)
	if err != nil {
		s.handleModelError(w, err)
		return
	}

	// Update activity
	backend.UpdateActivity()

	// Proxy the request
	backendURL := fmt.Sprintf("http://%s:%d", s.config.Host, backend.Port)
	target, err := url.Parse(backendURL)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "server_error", "invalid backend URL")
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	// Handle streaming responses properly
	proxy.FlushInterval = -1 // Flush immediately for SSE

	// Restore the body for the proxied request
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	r.URL.Path = path

	proxy.ServeHTTP(w, r)
}

// handleModels returns the list of loaded models
func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET is allowed")
		return
	}

	backends := s.manager.ListBackends()

	var models []OpenAIModelInfo
	for _, b := range backends {
		models = append(models, OpenAIModelInfo{
			ID:      b.ModelName,
			Object:  "model",
			Created: b.StartedAt.Unix(),
			OwnedBy: "local",
			Llemme: &LlemmeStatus{
				Status:       b.Status,
				Port:         b.Port,
				LastActivity: b.LastActivity,
				LoadedAt:     b.StartedAt,
			},
		})
	}

	// Also include downloaded but not loaded models
	downloaded, _ := s.manager.Resolver().ListDownloadedModels()
	loadedSet := make(map[string]bool)
	for _, b := range backends {
		loadedSet[b.ModelName] = true
	}
	for _, d := range downloaded {
		if !loadedSet[d.FullName] {
			models = append(models, OpenAIModelInfo{
				ID:      d.FullName,
				Object:  "model",
				Created: 0,
				OwnedBy: "local",
			})
		}
	}

	resp := OpenAIModelsResponse{
		Object: "list",
		Data:   models,
	}

	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, resp)
}

// handleHealth returns basic health status
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, map[string]string{"status": "ok"})
}

// handleStatus returns detailed proxy status
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET is allowed")
		return
	}

	backends := s.manager.ListBackends()

	status := ProxyStatus{
		Version:       Version,
		UptimeSeconds: time.Since(s.startedAt).Seconds(),
		Host:          s.config.Host,
		Port:          s.config.Port,
		MaxModels:     s.config.MaxModels,
		LoadedCount:   len(backends),
		IdleTimeout:   s.config.IdleTimeout.String(),
		Models:        backends,
	}

	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, status)
}

// handleModelError converts model errors to appropriate HTTP responses
func (s *Server) handleModelError(w http.ResponseWriter, err error) {
	switch e := err.(type) {
	case *AmbiguousModelError:
		msg := fmt.Sprintf("Ambiguous model name '%s'. Matches: %s",
			e.Query, strings.Join(e.Matches, ", "))
		s.writeError(w, http.StatusBadRequest, "invalid_request", msg)
	case *ModelNotFoundError:
		msg := fmt.Sprintf("No downloaded model matches '%s'", e.Query)
		if len(e.Suggestions) > 0 {
			msg += fmt.Sprintf(". Did you mean: %s", strings.Join(e.Suggestions, ", "))
		}
		s.writeError(w, http.StatusNotFound, "not_found", msg)
	default:
		s.writeError(w, http.StatusInternalServerError, "server_error", err.Error())
	}
}

// writeJSON encodes v as JSON to w. Encoding errors are ignored
// since there's no meaningful recovery (client connection is typically closed).
func writeJSON(w http.ResponseWriter, v any) {
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes an OpenAI-compatible error response
func (s *Server) writeError(w http.ResponseWriter, status int, errType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	writeJSON(w, OpenAIError{
		Error: OpenAIErrorDetail{
			Message: message,
			Type:    errType,
		},
	})
}

// handleStopModel handles requests to unload a specific model
func (s *Server) handleStopModel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST is allowed")
		return
	}

	var req struct {
		Model string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Failed to parse request body")
		return
	}

	if req.Model == "" {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Model field is required")
		return
	}

	// Resolve the model name
	result, err := s.manager.Resolver().Resolve(req.Model)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}

	if result.Model == nil {
		s.writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("Model '%s' is not loaded", req.Model))
		return
	}

	modelName := result.Model.FullName

	// Check if it's actually loaded
	if backend := s.manager.GetBackend(modelName); backend == nil {
		s.writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("Model '%s' is not loaded", req.Model))
		return
	}

	// Stop the backend
	if err := s.manager.StopBackend(modelName); err != nil {
		s.writeError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, map[string]any{
		"success": true,
		"model":   modelName,
	})
}

// handleStopAll handles requests to unload all models
func (s *Server) handleStopAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST is allowed")
		return
	}

	count := s.manager.LoadedCount()
	if err := s.manager.StopAllBackends(); err != nil {
		s.writeError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, map[string]any{
		"success": true,
		"stopped": count,
	})
}
