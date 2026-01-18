package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/nchapman/llemme/internal/config"
)

type ServerState struct {
	PID       int       `json:"pid"`
	Model     string    `json:"model"`
	ModelPath string    `json:"model_path"`
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	StartedAt time.Time `json:"started_at"`
}

const (
	stateFile = "server-state.json"
	pidFile   = "server.pid"
)

func StateFilePath() string {
	return filepath.Join(config.BinPath(), stateFile)
}

func PIDFilePath() string {
	return filepath.Join(config.BinPath(), pidFile)
}

func LoadState() (*ServerState, error) {
	data, err := os.ReadFile(StateFilePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read state: %w", err)
	}

	var state ServerState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state: %w", err)
	}

	return &state, nil
}

func SaveState(state *ServerState) error {
	if err := os.MkdirAll(filepath.Dir(StateFilePath()), 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	return atomicWriteFile(StateFilePath(), data, 0644)
}

func SavePID(pid int) error {
	if err := os.MkdirAll(filepath.Dir(PIDFilePath()), 0755); err != nil {
		return fmt.Errorf("failed to create pid directory: %w", err)
	}

	pidStr := fmt.Sprintf("%d", pid)
	return atomicWriteFile(PIDFilePath(), []byte(pidStr), 0644)
}

// atomicWriteFile writes data to a temp file then renames it to path.
// This ensures the file is never partially written.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func LoadPID() (int, error) {
	data, err := os.ReadFile(PIDFilePath())
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	var pid int
	_, err = fmt.Sscanf(string(data), "%d", &pid)
	if err != nil {
		return 0, fmt.Errorf("failed to parse pid: %w", err)
	}

	return pid, nil
}

func ClearState() error {
	var errors []error

	if err := os.Remove(StateFilePath()); err != nil && !os.IsNotExist(err) {
		errors = append(errors, err)
	}
	if err := os.Remove(PIDFilePath()); err != nil && !os.IsNotExist(err) {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return errors[0]
	}

	return nil
}

func IsRunning(state *ServerState) bool {
	if state == nil {
		return false
	}

	process, err := os.FindProcess(state.PID)
	if err != nil {
		return false
	}

	err = process.Signal(syscall.Signal(0))
	return err == nil
}

func GetServerURL(state *ServerState) string {
	if state == nil {
		return ""
	}
	return fmt.Sprintf("http://%s:%d", state.Host, state.Port)
}

func NewServerState(pid int, model, modelPath, host string, port int) *ServerState {
	return &ServerState{
		PID:       pid,
		Model:     model,
		ModelPath: modelPath,
		Host:      host,
		Port:      port,
		StartedAt: time.Now(),
	}
}
