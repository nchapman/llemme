package proxy

import (
	"os"
	"sync"
	"time"
)

// BackendStatus represents the current state of a backend server
type BackendStatus int

const (
	BackendStarting BackendStatus = iota
	BackendReady
	BackendStopping
	BackendStopped
)

func (s BackendStatus) String() string {
	switch s {
	case BackendStarting:
		return "starting"
	case BackendReady:
		return "ready"
	case BackendStopping:
		return "stopping"
	case BackendStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// Backend represents a running llama-server instance for a specific model
type Backend struct {
	mu           sync.RWMutex
	ModelName    string        // Full model reference: "TheBloke/Llama-2-7B-GGUF:Q4_K_M"
	ModelPath    string        // Absolute path to the .gguf file
	Port         int           // Port this backend is listening on
	Process      *os.Process   // The llama-server process
	LastActivity time.Time     // Last time a request was made to this backend
	StartedAt    time.Time     // When this backend was started
	Status       BackendStatus // Current status
	ReadyChan    chan struct{} // Closed when backend is ready (for request coalescing)
}

// UpdateActivity updates the last activity time for this backend
func (b *Backend) UpdateActivity() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.LastActivity = time.Now()
}

// GetLastActivity returns the last activity time
func (b *Backend) GetLastActivity() time.Time {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.LastActivity
}

// GetStatus returns the current status
func (b *Backend) GetStatus() BackendStatus {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.Status
}

// SetStatus updates the status
func (b *Backend) SetStatus(status BackendStatus) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Status = status
}

// IdleDuration returns how long the backend has been idle
func (b *Backend) IdleDuration() time.Duration {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return time.Since(b.LastActivity)
}

// Config holds proxy configuration
type Config struct {
	Host           string        // Proxy host (default: "127.0.0.1")
	Port           int           // Proxy port (default: 8080)
	MaxModels      int           // Maximum concurrent models (0 = unlimited)
	IdleTimeout    time.Duration // How long before idle models are unloaded
	BackendPortMin int           // Minimum port for backends
	BackendPortMax int           // Maximum port for backends
	StartupTimeout time.Duration // How long to wait for backend startup
}

// DefaultConfig returns the default proxy configuration
func DefaultConfig() *Config {
	return &Config{
		Host:           "127.0.0.1",
		Port:           8080,
		MaxModels:      3,
		IdleTimeout:    10 * time.Minute,
		BackendPortMin: 49152,
		BackendPortMax: 49200,
		StartupTimeout: 120 * time.Second,
	}
}

// ConfigFromAppConfig creates a proxy Config from the app config
func ConfigFromAppConfig(host string, port int, maxModels int, idleTimeoutMins int, backendPortMin int, backendPortMax int, startupTimeoutS int) *Config {
	cfg := DefaultConfig()

	if host != "" {
		cfg.Host = host
	}
	if port > 0 {
		cfg.Port = port
	}
	if maxModels > 0 {
		cfg.MaxModels = maxModels
	}
	if idleTimeoutMins > 0 {
		cfg.IdleTimeout = time.Duration(idleTimeoutMins) * time.Minute
	}
	if backendPortMin > 0 {
		cfg.BackendPortMin = backendPortMin
	}
	if backendPortMax > 0 {
		cfg.BackendPortMax = backendPortMax
	}
	if startupTimeoutS > 0 {
		cfg.StartupTimeout = time.Duration(startupTimeoutS) * time.Second
	}

	return cfg
}

// BackendInfo contains information about a backend for API responses
type BackendInfo struct {
	ModelName    string    `json:"name"`
	Status       string    `json:"status"`
	Port         int       `json:"port"`
	PID          int       `json:"pid"`
	StartedAt    time.Time `json:"started_at"`
	LastActivity time.Time `json:"last_activity"`
	IdleMinutes  float64   `json:"idle_minutes"`
}

// ProxyStatus contains the full proxy status for API responses
type ProxyStatus struct {
	Version       string        `json:"version"`
	UptimeSeconds float64       `json:"uptime_seconds"`
	Host          string        `json:"host"`
	Port          int           `json:"port"`
	MaxModels     int           `json:"max_models"`
	LoadedCount   int           `json:"loaded_count"`
	IdleTimeout   string        `json:"idle_timeout"`
	Models        []BackendInfo `json:"models"`
}

// OpenAIError represents an OpenAI-compatible error response
type OpenAIError struct {
	Error OpenAIErrorDetail `json:"error"`
}

// OpenAIErrorDetail contains the error details
type OpenAIErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
}

// OpenAIModelsResponse represents the /v1/models response
type OpenAIModelsResponse struct {
	Object string            `json:"object"`
	Data   []OpenAIModelInfo `json:"data"`
}

// OpenAIModelInfo represents a single model in the models list
type OpenAIModelInfo struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	OwnedBy string         `json:"owned_by"`
	Lemme *LemmeStatus `json:"lemme,omitempty"`
}

// LemmeStatus contains lemme-specific model status
type LemmeStatus struct {
	Status       string    `json:"status"`
	Port         int       `json:"port"`
	LastActivity time.Time `json:"last_activity"`
	LoadedAt     time.Time `json:"loaded_at"`
}
