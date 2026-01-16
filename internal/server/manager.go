package server

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/nchapman/gollama/internal/config"
	"github.com/nchapman/gollama/internal/llama"
)

type ServerManager struct {
	binaryPath string
	config     *config.Config
}

func NewManager(cfg *config.Config) *ServerManager {
	return &ServerManager{
		binaryPath: llama.ServerPath(),
		config:     cfg,
	}
}

func (sm *ServerManager) Start(modelPath, modelRef string) error {
	state, err := LoadState()
	if err != nil {
		return fmt.Errorf("failed to load server state: %w", err)
	}

	if IsRunning(state) {
		if state.Model == modelRef {
			return fmt.Errorf("server already running with model %s", modelRef)
		}
		if err := sm.Stop(); err != nil {
			return fmt.Errorf("failed to stop existing server: %w", err)
		}
	}

	args := sm.buildArgs(modelPath)

	cmd := exec.Command(sm.binaryPath, args...)
	cmd.Env = os.Environ()
	cmd.Dir = config.BinPath()

	logFile := filepath.Join(config.BinPath(), "server.log")
	log, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer log.Close()

	cmd.Stdout = log
	cmd.Stderr = log

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	pid := cmd.Process.Pid

	if err := SavePID(pid); err != nil {
		cmd.Process.Kill()
		return fmt.Errorf("failed to save pid: %w", err)
	}

	newState := NewServerState(pid, modelRef, modelPath, sm.config.Server.Host, sm.config.Server.Port)
	if err := SaveState(newState); err != nil {
		cmd.Process.Kill()
		ClearState()
		return fmt.Errorf("failed to save state: %w", err)
	}

	if err := sm.waitForReady(); err != nil {
		sm.Stop()
		return fmt.Errorf("server failed to start: %w", err)
	}

	return nil
}

func (sm *ServerManager) Stop() error {
	state, err := LoadState()
	if err != nil {
		return fmt.Errorf("failed to load server state: %w", err)
	}

	if !IsRunning(state) {
		return fmt.Errorf("server is not running")
	}

	process, err := os.FindProcess(state.PID)
	if err != nil {
		ClearState()
		return nil
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM: %w", err)
	}

	time.Sleep(100 * time.Millisecond)

	process, _ = os.FindProcess(state.PID)
	if process != nil {
		if err := process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
	}

	if err := ClearState(); err != nil {
		return fmt.Errorf("failed to clear state: %w", err)
	}

	return nil
}

func (sm *ServerManager) buildArgs(modelPath string) []string {
	args := []string{
		"--model", modelPath,
		"--host", sm.config.Server.Host,
		"--port", fmt.Sprintf("%d", sm.config.Server.Port),
	}

	if sm.config.ContextLength > 0 {
		args = append(args, "--ctx-size", fmt.Sprintf("%d", sm.config.ContextLength))
	}

	if sm.config.Temperature > 0 {
		args = append(args, "--temp", fmt.Sprintf("%.2f", sm.config.Temperature))
	}

	if sm.config.GPULayers != 0 {
		args = append(args, "--gpu-layers", fmt.Sprintf("%d", sm.config.GPULayers))
	}

	return args
}

func (sm *ServerManager) waitForReady() error {
	logFile := filepath.Join(config.BinPath(), "server.log")

	for i := 0; i < 60; i++ {
		time.Sleep(500 * time.Millisecond)

		file, err := os.Open(logFile)
		if err != nil {
			continue
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "listening on http") {
				return nil
			}
			if strings.Contains(line, "error") || strings.Contains(line, "failed") {
				return fmt.Errorf("server startup failed")
			}
		}
	}

	return fmt.Errorf("server did not start within 30 seconds")
}

func (sm *ServerManager) Status() (*ServerState, error) {
	state, err := LoadState()
	if err != nil {
		return nil, err
	}

	if !IsRunning(state) {
		return nil, nil
	}

	return state, nil
}
