package proxy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/nchapman/lleme/internal/config"
	"github.com/nchapman/lleme/internal/fileutil"
)

const (
	proxyPIDFile   = "proxy.pid"
	proxyStateFile = "proxy-state.json"
)

// ProxyState persists proxy metadata for CLI commands to discover
type ProxyState struct {
	PID       int       `json:"pid"`
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	StartedAt time.Time `json:"started_at"`
}

// ProxyPIDPath returns the path to the proxy PID file
func ProxyPIDPath() string {
	return filepath.Join(config.PidsPath(), proxyPIDFile)
}

// ProxyStatePath returns the path to the proxy state file
func ProxyStatePath() string {
	return filepath.Join(config.PidsPath(), proxyStateFile)
}

// SaveProxyState saves the proxy state to disk using atomic writes
func SaveProxyState(state *ProxyState) error {
	if err := os.MkdirAll(filepath.Dir(ProxyStatePath()), 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := fileutil.AtomicWriteFile(ProxyStatePath(), data, 0644); err != nil {
		return fmt.Errorf("failed to write state: %w", err)
	}

	// Also write PID file
	pidStr := fmt.Sprintf("%d", state.PID)
	if err := fileutil.AtomicWriteFile(ProxyPIDPath(), []byte(pidStr), 0644); err != nil {
		return fmt.Errorf("failed to write PID: %w", err)
	}

	return nil
}

// LoadProxyState loads the proxy state from disk
func LoadProxyState() (*ProxyState, error) {
	data, err := os.ReadFile(ProxyStatePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read state: %w", err)
	}

	var state ProxyState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state: %w", err)
	}

	return &state, nil
}

// ClearProxyState removes the proxy state files
func ClearProxyState() error {
	var errors []error

	if err := os.Remove(ProxyStatePath()); err != nil && !os.IsNotExist(err) {
		errors = append(errors, err)
	}
	if err := os.Remove(ProxyPIDPath()); err != nil && !os.IsNotExist(err) {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return errors[0]
	}

	return nil
}

// IsProxyRunning checks if the proxy is running based on saved state
func IsProxyRunning() bool {
	state, err := LoadProxyState()
	if err != nil || state == nil {
		return false
	}

	return isProcessRunning(state.PID)
}

// GetRunningProxyState returns the proxy state if the proxy is running
func GetRunningProxyState() *ProxyState {
	state, err := LoadProxyState()
	if err != nil || state == nil {
		return nil
	}

	if !isProcessRunning(state.PID) {
		// Stale state, clean up
		ClearProxyState()
		return nil
	}

	return state
}

// isProcessRunning checks if a process with the given PID is running
func isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// GetProxyURL returns the URL of the running proxy, or empty if not running
func GetProxyURL() string {
	state := GetRunningProxyState()
	if state == nil {
		return ""
	}
	return fmt.Sprintf("http://%s:%d", state.Host, state.Port)
}
