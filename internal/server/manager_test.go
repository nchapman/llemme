package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nchapman/llemme/internal/config"
)

func TestNewManager(t *testing.T) {
	cfg := &config.Config{
		Server: config.Server{
			Host: "127.0.0.1",
			Port: 8080,
		},
	}

	sm := NewManager(cfg)
	if sm == nil {
		t.Fatal("Expected non-nil ServerManager")
	}
	if sm.binaryPath == "" {
		t.Error("Expected binaryPath to be set")
	}
	if sm.config == nil {
		t.Error("Expected config to be set")
	}
	if sm.config.Server.Host != "127.0.0.1" {
		t.Errorf("Expected Host 127.0.0.1, got %s", sm.config.Server.Host)
	}
	if sm.config.Server.Port != 8080 {
		t.Errorf("Expected Port 8080, got %d", sm.config.Server.Port)
	}
}

func TestServerState(t *testing.T) {
	t.Run("save and load state", func(t *testing.T) {
		tmpDir := t.TempDir()
		oldHome := os.Getenv("HOME")
		defer os.Setenv("HOME", oldHome)

		os.Setenv("HOME", tmpDir)

		state := &ServerState{
			PID:       12345,
			Model:     "test-model",
			ModelPath: "/path/to/model.gguf",
			Host:      "127.0.0.1",
			Port:      8080,
			StartedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		}

		err := SaveState(state)
		if err != nil {
			t.Fatalf("Failed to save state: %v", err)
		}

		loaded, err := LoadState()
		if err != nil {
			t.Fatalf("Failed to load state: %v", err)
		}

		if loaded == nil {
			t.Fatal("Expected loaded state to be non-nil")
		}

		if loaded.PID != state.PID {
			t.Errorf("Expected PID %d, got %d", state.PID, loaded.PID)
		}
		if loaded.Model != state.Model {
			t.Errorf("Expected Model %s, got %s", state.Model, loaded.Model)
		}
		if loaded.ModelPath != state.ModelPath {
			t.Errorf("Expected ModelPath %s, got %s", state.ModelPath, loaded.ModelPath)
		}
		if loaded.Host != state.Host {
			t.Errorf("Expected Host %s, got %s", state.Host, loaded.Host)
		}
		if loaded.Port != state.Port {
			t.Errorf("Expected Port %d, got %d", state.Port, loaded.Port)
		}
		if loaded.StartedAt != state.StartedAt {
			t.Errorf("Expected StartedAt %s, got %s", state.StartedAt, loaded.StartedAt)
		}
	})

	t.Run("returns nil when state file does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		oldHome := os.Getenv("HOME")
		defer os.Setenv("HOME", oldHome)

		os.Setenv("HOME", tmpDir)

		state, err := LoadState()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if state != nil {
			t.Error("Expected nil state when file doesn't exist")
		}
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		oldHome := os.Getenv("HOME")
		defer os.Setenv("HOME", oldHome)

		os.Setenv("HOME", tmpDir)

		binDir := filepath.Join(tmpDir, ".llemme", "bin")
		if err := os.MkdirAll(binDir, 0755); err != nil {
			t.Fatalf("Failed to create bin dir: %v", err)
		}

		statePath := StateFilePath()
		if err := os.WriteFile(statePath, []byte("invalid json"), 0644); err != nil {
			t.Fatalf("Failed to write invalid state file: %v", err)
		}

		_, err := LoadState()
		if err == nil {
			t.Error("Expected error for invalid JSON, got nil")
		}
	})
}

func TestPIDFile(t *testing.T) {
	t.Run("save and load PID", func(t *testing.T) {
		tmpDir := t.TempDir()
		oldHome := os.Getenv("HOME")
		defer os.Setenv("HOME", oldHome)

		os.Setenv("HOME", tmpDir)

		pid := 12345
		err := SavePID(pid)
		if err != nil {
			t.Fatalf("Failed to save PID: %v", err)
		}

		loaded, err := LoadPID()
		if err != nil {
			t.Fatalf("Failed to load PID: %v", err)
		}

		if loaded != pid {
			t.Errorf("Expected PID %d, got %d", pid, loaded)
		}
	})

	t.Run("returns 0 when PID file does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		oldHome := os.Getenv("HOME")
		defer os.Setenv("HOME", oldHome)

		os.Setenv("HOME", tmpDir)

		pid, err := LoadPID()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if pid != 0 {
			t.Errorf("Expected PID 0 when file doesn't exist, got %d", pid)
		}
	})

	t.Run("returns error for invalid PID", func(t *testing.T) {
		tmpDir := t.TempDir()
		oldHome := os.Getenv("HOME")
		defer os.Setenv("HOME", oldHome)

		os.Setenv("HOME", tmpDir)

		binDir := filepath.Join(tmpDir, ".llemme", "bin")
		if err := os.MkdirAll(binDir, 0755); err != nil {
			t.Fatalf("Failed to create bin dir: %v", err)
		}

		pidPath := PIDFilePath()
		if err := os.WriteFile(pidPath, []byte("not-a-number"), 0644); err != nil {
			t.Fatalf("Failed to write invalid PID file: %v", err)
		}

		_, err := LoadPID()
		if err == nil {
			t.Error("Expected error for invalid PID, got nil")
		}
	})
}

func TestClearState(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)

	os.Setenv("HOME", tmpDir)

	binDir := filepath.Join(tmpDir, ".llemme", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("Failed to create bin dir: %v", err)
	}

	t.Run("clears existing state files", func(t *testing.T) {
		state := &ServerState{
			PID:       12345,
			Model:     "test-model",
			ModelPath: "/path/to/model.gguf",
			Host:      "127.0.0.1",
			Port:      8080,
			StartedAt: time.Now(),
		}

		if err := SaveState(state); err != nil {
			t.Fatalf("Failed to save state: %v", err)
		}

		if err := SavePID(12345); err != nil {
			t.Fatalf("Failed to save PID: %v", err)
		}

		if err := ClearState(); err != nil {
			t.Fatalf("Failed to clear state: %v", err)
		}

		loadedState, err := LoadState()
		if err != nil {
			t.Fatalf("Failed to load state after clearing: %v", err)
		}
		if loadedState != nil {
			t.Error("Expected nil state after clearing")
		}

		loadedPID, err := LoadPID()
		if err != nil {
			t.Fatalf("Failed to load PID after clearing: %v", err)
		}
		if loadedPID != 0 {
			t.Errorf("Expected PID 0 after clearing, got %d", loadedPID)
		}
	})

	t.Run("returns nil when files don't exist", func(t *testing.T) {
		err := ClearState()
		if err != nil {
			t.Errorf("Expected no error when clearing non-existent files, got %v", err)
		}
	})
}

func TestIsRunning(t *testing.T) {
	t.Run("returns false for nil state", func(t *testing.T) {
		if IsRunning(nil) {
			t.Error("Expected IsRunning to return false for nil state")
		}
	})

	t.Run("returns false for zero PID", func(t *testing.T) {
		state := &ServerState{PID: 0}
		if IsRunning(state) {
			t.Error("Expected IsRunning to return false for zero PID")
		}
	})

	t.Run("returns false for negative PID", func(t *testing.T) {
		state := &ServerState{PID: -1}
		if IsRunning(state) {
			t.Error("Expected IsRunning to return false for negative PID")
		}
	})

	t.Run("returns true for current process", func(t *testing.T) {
		state := &ServerState{PID: os.Getpid()}
		if !IsRunning(state) {
			t.Error("Expected IsRunning to return true for current process")
		}
	})

	t.Run("returns false for non-existent PID", func(t *testing.T) {
		// Use a very high PID that's unlikely to exist
		state := &ServerState{PID: 999999999}
		if IsRunning(state) {
			t.Error("Expected IsRunning to return false for non-existent PID")
		}
	})
}

func TestGetServerURL(t *testing.T) {
	t.Run("returns empty string for nil state", func(t *testing.T) {
		url := GetServerURL(nil)
		if url != "" {
			t.Errorf("Expected empty URL for nil state, got %s", url)
		}
	})

	t.Run("returns correct URL", func(t *testing.T) {
		state := &ServerState{
			Host: "127.0.0.1",
			Port: 8080,
		}

		url := GetServerURL(state)
		expected := "http://127.0.0.1:8080"
		if url != expected {
			t.Errorf("Expected URL %s, got %s", expected, url)
		}
	})
}

func TestNewServerState(t *testing.T) {
	state := NewServerState(12345, "test-model", "/path/to/model.gguf", "127.0.0.1", 8080)

	if state.PID != 12345 {
		t.Errorf("Expected PID 12345, got %d", state.PID)
	}
	if state.Model != "test-model" {
		t.Errorf("Expected Model test-model, got %s", state.Model)
	}
	if state.ModelPath != "/path/to/model.gguf" {
		t.Errorf("Expected ModelPath /path/to/model.gguf, got %s", state.ModelPath)
	}
	if state.Host != "127.0.0.1" {
		t.Errorf("Expected Host 127.0.0.1, got %s", state.Host)
	}
	if state.Port != 8080 {
		t.Errorf("Expected Port 8080, got %d", state.Port)
	}
	if state.StartedAt.IsZero() {
		t.Error("Expected StartedAt to be set")
	}
}

func TestBuildArgs(t *testing.T) {
	tests := []struct {
		name         string
		config       *config.Config
		modelPath    string
		expectedArgs []string
	}{
		{
			name: "minimal config",
			config: &config.Config{
				Server: config.Server{
					Host: "127.0.0.1",
					Port: 8080,
				},
			},
			modelPath:    "/path/to/model.gguf",
			expectedArgs: []string{"--model", "/path/to/model.gguf", "--host", "127.0.0.1", "--port", "8080"},
		},
		{
			name: "with context length",
			config: &config.Config{
				Server:        config.Server{Host: "0.0.0.0", Port: 9000},
				ContextLength: 2048,
			},
			modelPath: "/path/to/model.gguf",
			expectedArgs: []string{
				"--model", "/path/to/model.gguf",
				"--host", "0.0.0.0", "--port", "9000",
				"--ctx-size", "2048",
			},
		},
		{
			name: "with temperature",
			config: &config.Config{
				Server:      config.Server{Host: "127.0.0.1", Port: 8080},
				Temperature: 0.5,
			},
			modelPath: "/path/to/model.gguf",
			expectedArgs: []string{
				"--model", "/path/to/model.gguf",
				"--host", "127.0.0.1", "--port", "8080",
				"--temp", "0.50",
			},
		},
		{
			name: "with GPU layers",
			config: &config.Config{
				Server:    config.Server{Host: "127.0.0.1", Port: 8080},
				GPULayers: 35,
			},
			modelPath: "/path/to/model.gguf",
			expectedArgs: []string{
				"--model", "/path/to/model.gguf",
				"--host", "127.0.0.1", "--port", "8080",
				"--gpu-layers", "35",
			},
		},
		{
			name: "full config",
			config: &config.Config{
				Server:        config.Server{Host: "127.0.0.1", Port: 8080},
				ContextLength: 4096,
				Temperature:   0.7,
				GPULayers:     20,
			},
			modelPath: "/path/to/model.gguf",
			expectedArgs: []string{
				"--model", "/path/to/model.gguf",
				"--host", "127.0.0.1", "--port", "8080",
				"--ctx-size", "4096",
				"--temp", "0.70",
				"--gpu-layers", "20",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := &ServerManager{
				binaryPath: "/path/to/llama-server",
				config:     tt.config,
			}

			args := sm.buildArgs(tt.modelPath)

			if len(args) != len(tt.expectedArgs) {
				t.Errorf("Expected %d args, got %d", len(tt.expectedArgs), len(args))
				t.Logf("Expected: %v", tt.expectedArgs)
				t.Logf("Got:      %v", args)
				return
			}

			for i, expected := range tt.expectedArgs {
				if args[i] != expected {
					t.Errorf("Arg %d: expected %s, got %s", i, expected, args[i])
				}
			}
		})
	}
}

func TestStatus(t *testing.T) {
	cfg := &config.Config{
		Server: config.Server{
			Host: "127.0.0.1",
			Port: 8080,
		},
	}

	sm := NewManager(cfg)

	t.Run("returns nil when no state file exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		oldHome := os.Getenv("HOME")
		defer os.Setenv("HOME", oldHome)
		os.Setenv("HOME", tmpDir)

		state, err := sm.Status()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if state != nil {
			t.Error("Expected nil state when server is not running")
		}
	})

	t.Run("returns nil when state exists but process not running", func(t *testing.T) {
		tmpDir := t.TempDir()
		oldHome := os.Getenv("HOME")
		defer os.Setenv("HOME", oldHome)
		os.Setenv("HOME", tmpDir)

		// Create state with non-existent PID
		state := &ServerState{
			PID:       999999999,
			Model:     "test-model",
			ModelPath: "/path/to/model.gguf",
			Host:      "127.0.0.1",
			Port:      8080,
			StartedAt: time.Now(),
		}
		if err := SaveState(state); err != nil {
			t.Fatalf("Failed to save state: %v", err)
		}

		result, err := sm.Status()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if result != nil {
			t.Error("Expected nil when process not running")
		}
	})

	t.Run("returns state when process is running", func(t *testing.T) {
		tmpDir := t.TempDir()
		oldHome := os.Getenv("HOME")
		defer os.Setenv("HOME", oldHome)
		os.Setenv("HOME", tmpDir)

		// Create state with current process PID (which is running)
		state := &ServerState{
			PID:       os.Getpid(),
			Model:     "test-model",
			ModelPath: "/path/to/model.gguf",
			Host:      "127.0.0.1",
			Port:      8080,
			StartedAt: time.Now(),
		}
		if err := SaveState(state); err != nil {
			t.Fatalf("Failed to save state: %v", err)
		}

		result, err := sm.Status()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if result == nil {
			t.Fatal("Expected non-nil state when process is running")
		}
		if result.PID != os.Getpid() {
			t.Errorf("Expected PID %d, got %d", os.Getpid(), result.PID)
		}
	})

	t.Run("returns error when state file is invalid", func(t *testing.T) {
		tmpDir := t.TempDir()
		oldHome := os.Getenv("HOME")
		defer os.Setenv("HOME", oldHome)
		os.Setenv("HOME", tmpDir)

		// Create invalid state file
		binDir := filepath.Join(tmpDir, ".llemme", "bin")
		if err := os.MkdirAll(binDir, 0755); err != nil {
			t.Fatalf("Failed to create bin dir: %v", err)
		}
		statePath := StateFilePath()
		if err := os.WriteFile(statePath, []byte("invalid json"), 0644); err != nil {
			t.Fatalf("Failed to write invalid state: %v", err)
		}

		_, err := sm.Status()
		if err == nil {
			t.Error("Expected error for invalid state file")
		}
	})
}

func TestPathHelpers(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)

	os.Setenv("HOME", tmpDir)

	expectedStatePath := filepath.Join(tmpDir, ".llemme", "bin", "server-state.json")
	actualStatePath := StateFilePath()
	if actualStatePath != expectedStatePath {
		t.Errorf("Expected StatePath %s, got %s", expectedStatePath, actualStatePath)
	}

	expectedPIDPath := filepath.Join(tmpDir, ".llemme", "bin", "server.pid")
	actualPIDPath := PIDFilePath()
	if actualPIDPath != expectedPIDPath {
		t.Errorf("Expected PIDPath %s, got %s", expectedPIDPath, actualPIDPath)
	}
}

func TestCheckLogForReady(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Server: config.Server{Host: "127.0.0.1", Port: 8080},
	}
	sm := NewManager(cfg)

	tests := []struct {
		name       string
		logContent string
		wantReady  bool
		wantErr    bool
	}{
		{
			name:       "server ready",
			logContent: "Starting server...\nlistening on http://127.0.0.1:8080\n",
			wantReady:  true,
			wantErr:    false,
		},
		{
			name:       "server startup failed",
			logContent: "Starting server...\nerror loading model\n",
			wantReady:  false,
			wantErr:    true,
		},
		{
			name:       "server failed with failed message",
			logContent: "Starting server...\nfailed to initialize\n",
			wantReady:  false,
			wantErr:    true,
		},
		{
			name:       "server still starting",
			logContent: "Loading model...\nInitializing...\n",
			wantReady:  false,
			wantErr:    false,
		},
		{
			name:       "empty log",
			logContent: "",
			wantReady:  false,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logFile := filepath.Join(tmpDir, tt.name+".log")
			if err := os.WriteFile(logFile, []byte(tt.logContent), 0644); err != nil {
				t.Fatalf("Failed to write log file: %v", err)
			}

			ready, err := sm.checkLogForReady(logFile)

			if tt.wantErr && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
			if ready != tt.wantReady {
				t.Errorf("Expected ready=%v, got %v", tt.wantReady, ready)
			}
		})
	}

	t.Run("non-existent file", func(t *testing.T) {
		ready, err := sm.checkLogForReady("/nonexistent/path/log.txt")
		if err != nil {
			t.Errorf("Expected no error for non-existent file, got %v", err)
		}
		if ready {
			t.Error("Expected ready=false for non-existent file")
		}
	})
}
