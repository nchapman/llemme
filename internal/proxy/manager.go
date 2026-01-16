package proxy

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/nchapman/llemme/internal/config"
	"github.com/nchapman/llemme/internal/llama"
)

// ModelManager manages the lifecycle of llama-server backend instances
type ModelManager struct {
	mu            sync.RWMutex
	backends      map[string]*Backend // model name -> backend
	lruOrder      []string            // for eviction ordering (front = most recent)
	portAllocator *PortAllocator
	resolver      *ModelResolver
	config        *Config
	appConfig     *config.Config
}

// NewModelManager creates a new model manager
func NewModelManager(cfg *Config, appCfg *config.Config) *ModelManager {
	return &ModelManager{
		backends:      make(map[string]*Backend),
		lruOrder:      make([]string, 0),
		portAllocator: NewPortAllocator(cfg.BackendPortMin, cfg.BackendPortMax),
		resolver:      NewModelResolver(),
		config:        cfg,
		appConfig:     appCfg,
	}
}

// GetOrLoadBackend returns a backend for the given model, loading it if necessary
func (m *ModelManager) GetOrLoadBackend(modelQuery string) (*Backend, error) {
	// First, resolve the model name
	result, err := m.resolver.Resolve(modelQuery)
	if err != nil {
		return nil, err
	}

	// Handle resolution errors
	if result.Model == nil {
		if len(result.Matches) > 1 {
			// Ambiguous match
			var names []string
			for _, match := range result.Matches {
				names = append(names, match.FullName)
			}
			return nil, &AmbiguousModelError{
				Query:   modelQuery,
				Matches: names,
			}
		}
		// No match
		var suggestions []string
		for _, s := range result.Suggestions {
			suggestions = append(suggestions, s.FullName)
		}
		return nil, &ModelNotFoundError{
			Query:       modelQuery,
			Suggestions: suggestions,
		}
	}

	modelName := result.Model.FullName
	modelPath := result.Model.ModelPath

	// Check if already loaded or loading
	m.mu.Lock()
	backend, exists := m.backends[modelName]
	if exists {
		status := backend.GetStatus()
		if status == BackendReady {
			// Already ready - update LRU and return
			m.updateLRU(modelName)
			backend.UpdateActivity()
			m.mu.Unlock()
			return backend, nil
		}
		if status == BackendStarting {
			// Currently starting - wait for it
			readyChan := backend.ReadyChan
			m.mu.Unlock()
			<-readyChan
			if backend.GetStatus() == BackendReady {
				backend.UpdateActivity()
				return backend, nil
			}
			return nil, fmt.Errorf("backend failed to start")
		}
	}

	// Need to start a new backend
	// Check if we need to evict
	if m.config.MaxModels > 0 && len(m.backends) >= m.config.MaxModels {
		if err := m.evictLRU(); err != nil {
			m.mu.Unlock()
			return nil, fmt.Errorf("failed to evict model: %w", err)
		}
	}

	// Allocate port
	port, err := m.portAllocator.Allocate()
	if err != nil {
		m.mu.Unlock()
		return nil, fmt.Errorf("failed to allocate port: %w", err)
	}

	// Create backend entry
	backend = &Backend{
		ModelName:    modelName,
		ModelPath:    modelPath,
		Port:         port,
		Status:       BackendStarting,
		StartedAt:    time.Now(),
		LastActivity: time.Now(),
		ReadyChan:    make(chan struct{}),
	}
	m.backends[modelName] = backend
	m.lruOrder = append([]string{modelName}, m.lruOrder...)
	m.mu.Unlock()

	// Start the backend in background
	go m.startBackend(backend)

	// Wait for ready
	select {
	case <-backend.ReadyChan:
		if backend.GetStatus() == BackendReady {
			return backend, nil
		}
		return nil, fmt.Errorf("backend failed to start")
	case <-time.After(m.config.StartupTimeout):
		m.StopBackend(modelName)
		return nil, fmt.Errorf("backend startup timeout after %v", m.config.StartupTimeout)
	}
}

// GetBackend returns a backend if it exists and is ready
func (m *ModelManager) GetBackend(modelName string) *Backend {
	m.mu.RLock()
	defer m.mu.RUnlock()

	backend, exists := m.backends[modelName]
	if !exists {
		return nil
	}

	if backend.GetStatus() != BackendReady {
		return nil
	}

	return backend
}

// ListBackends returns info about all loaded backends
func (m *ModelManager) ListBackends() []BackendInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var infos []BackendInfo
	for _, backend := range m.backends {
		pid := 0
		if backend.Process != nil {
			pid = backend.Process.Pid
		}
		infos = append(infos, BackendInfo{
			ModelName:    backend.ModelName,
			Status:       backend.GetStatus().String(),
			Port:         backend.Port,
			PID:          pid,
			StartedAt:    backend.StartedAt,
			LastActivity: backend.GetLastActivity(),
			IdleMinutes:  backend.IdleDuration().Minutes(),
		})
	}
	return infos
}

// StopBackend stops a specific backend
func (m *ModelManager) StopBackend(modelName string) error {
	m.mu.Lock()
	backend, exists := m.backends[modelName]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("backend not found: %s", modelName)
	}

	backend.SetStatus(BackendStopping)
	m.mu.Unlock()

	// Graceful shutdown
	if backend.Process != nil {
		backend.Process.Signal(syscall.SIGTERM)

		// Wait for graceful exit (up to 5 seconds)
		done := make(chan struct{})
		go func() {
			backend.Process.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Process exited gracefully
		case <-time.After(5 * time.Second):
			// Force kill
			backend.Process.Kill()
			backend.Process.Wait()
		}
	}

	// Cleanup
	m.mu.Lock()
	defer m.mu.Unlock()

	backend.SetStatus(BackendStopped)
	// Only close ReadyChan if not already closed (it's closed when backend becomes ready)
	select {
	case <-backend.ReadyChan:
		// Already closed
	default:
		close(backend.ReadyChan)
	}
	m.portAllocator.Release(backend.Port)
	delete(m.backends, modelName)
	m.removeLRU(modelName)

	return nil
}

// StopAllBackends stops all running backends
func (m *ModelManager) StopAllBackends() error {
	m.mu.RLock()
	names := make([]string, 0, len(m.backends))
	for name := range m.backends {
		names = append(names, name)
	}
	m.mu.RUnlock()

	var lastErr error
	for _, name := range names {
		if err := m.StopBackend(name); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// LoadedCount returns the number of loaded models
func (m *ModelManager) LoadedCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.backends)
}

// GetIdleBackends returns backends that have been idle longer than the timeout
func (m *ModelManager) GetIdleBackends(timeout time.Duration) []*Backend {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var idle []*Backend
	for _, backend := range m.backends {
		if backend.GetStatus() == BackendReady && backend.IdleDuration() > timeout {
			idle = append(idle, backend)
		}
	}
	return idle
}

// Resolver returns the model resolver
func (m *ModelManager) Resolver() *ModelResolver {
	return m.resolver
}

// startBackend starts the llama-server process for a backend
func (m *ModelManager) startBackend(backend *Backend) {
	defer func() {
		// Ensure ReadyChan is closed even on error
		select {
		case <-backend.ReadyChan:
			// Already closed
		default:
			if backend.GetStatus() != BackendReady {
				close(backend.ReadyChan)
			}
		}
	}()

	serverPath := llama.ServerPath()
	args := m.buildArgs(backend)

	cmd := exec.Command(serverPath, args...)
	cmd.Env = os.Environ()
	cmd.Dir = config.BinPath()

	// Create log file for this backend
	logFile := filepath.Join(config.BinPath(), fmt.Sprintf("backend-%d.log", backend.Port))
	log, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		backend.SetStatus(BackendStopped)
		return
	}
	defer log.Close()

	cmd.Stdout = log
	cmd.Stderr = log

	if err := cmd.Start(); err != nil {
		backend.SetStatus(BackendStopped)
		return
	}

	backend.Process = cmd.Process

	// Wait for server to be ready
	if err := m.waitForReady(backend, logFile); err != nil {
		backend.SetStatus(BackendStopped)
		cmd.Process.Kill()
		return
	}

	backend.SetStatus(BackendReady)
	close(backend.ReadyChan)
}

func (m *ModelManager) buildArgs(backend *Backend) []string {
	args := []string{
		"--model", backend.ModelPath,
		"--host", m.config.Host,
		"--port", fmt.Sprintf("%d", backend.Port),
	}

	if m.appConfig.ContextLength > 0 {
		args = append(args, "--ctx-size", fmt.Sprintf("%d", m.appConfig.ContextLength))
	}

	if m.appConfig.GPULayers != 0 {
		args = append(args, "--gpu-layers", fmt.Sprintf("%d", m.appConfig.GPULayers))
	}

	return args
}

func (m *ModelManager) waitForReady(backend *Backend, logFile string) error {
	healthURL := fmt.Sprintf("http://%s:%d/health", m.config.Host, backend.Port)
	client := &http.Client{Timeout: 2 * time.Second}

	deadline := time.Now().Add(m.config.StartupTimeout)

	for time.Now().Before(deadline) {
		// Try health check
		resp, err := client.Get(healthURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}

		// Check log for errors
		if hasStartupError(logFile) {
			return fmt.Errorf("server startup failed (check %s)", logFile)
		}

		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("server did not become ready within %v", m.config.StartupTimeout)
}

func hasStartupError(logFile string) bool {
	file, err := os.Open(logFile)
	if err != nil {
		return false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.ToLower(scanner.Text())
		if strings.Contains(line, "error") && strings.Contains(line, "failed") {
			return true
		}
		if strings.Contains(line, "could not load model") {
			return true
		}
	}
	return false
}

func (m *ModelManager) updateLRU(modelName string) {
	// Move to front
	m.removeLRU(modelName)
	m.lruOrder = append([]string{modelName}, m.lruOrder...)
}

func (m *ModelManager) removeLRU(modelName string) {
	for i, name := range m.lruOrder {
		if name == modelName {
			m.lruOrder = append(m.lruOrder[:i], m.lruOrder[i+1:]...)
			return
		}
	}
}

func (m *ModelManager) evictLRU() error {
	if len(m.lruOrder) == 0 {
		return fmt.Errorf("no models to evict")
	}

	// Get least recently used (end of list)
	lruModel := m.lruOrder[len(m.lruOrder)-1]
	m.mu.Unlock() // Unlock before stopping (StopBackend needs lock)
	err := m.StopBackend(lruModel)
	m.mu.Lock() // Re-lock
	return err
}

// AmbiguousModelError is returned when a query matches multiple models
type AmbiguousModelError struct {
	Query   string
	Matches []string
}

func (e *AmbiguousModelError) Error() string {
	return fmt.Sprintf("ambiguous model name '%s': matches %v", e.Query, e.Matches)
}

// ModelNotFoundError is returned when no model matches the query
type ModelNotFoundError struct {
	Query       string
	Suggestions []string
}

func (e *ModelNotFoundError) Error() string {
	if len(e.Suggestions) > 0 {
		return fmt.Sprintf("no model matches '%s'. Did you mean: %v", e.Query, e.Suggestions)
	}
	return fmt.Sprintf("no model matches '%s'", e.Query)
}
