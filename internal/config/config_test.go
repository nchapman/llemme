package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// HuggingFace defaults
	if cfg.HuggingFace.Token != "" {
		t.Errorf("Expected empty HuggingFace.Token, got %s", cfg.HuggingFace.Token)
	}
	if cfg.HuggingFace.DefaultQuant != "Q4_K_M" {
		t.Errorf("Expected HuggingFace.DefaultQuant Q4_K_M, got %s", cfg.HuggingFace.DefaultQuant)
	}

	// LlamaCpp defaults
	if cfg.LlamaCpp.ServerPath != "" {
		t.Errorf("Expected empty LlamaCpp.ServerPath, got %s", cfg.LlamaCpp.ServerPath)
	}
	if cfg.LlamaCpp.ContextLength != 4096 {
		t.Errorf("Expected LlamaCpp.ContextLength 4096, got %d", cfg.LlamaCpp.ContextLength)
	}
	if cfg.LlamaCpp.GPULayers != -1 {
		t.Errorf("Expected LlamaCpp.GPULayers -1, got %d", cfg.LlamaCpp.GPULayers)
	}
	if cfg.LlamaCpp.Temperature != 0.7 {
		t.Errorf("Expected LlamaCpp.Temperature 0.7, got %f", cfg.LlamaCpp.Temperature)
	}
	if cfg.LlamaCpp.TopP != 0.9 {
		t.Errorf("Expected LlamaCpp.TopP 0.9, got %f", cfg.LlamaCpp.TopP)
	}
	if cfg.LlamaCpp.TopK != 40 {
		t.Errorf("Expected LlamaCpp.TopK 40, got %d", cfg.LlamaCpp.TopK)
	}

	// Server defaults
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("Expected Server.Host 127.0.0.1, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Expected Server.Port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Server.MaxModels != 3 {
		t.Errorf("Expected Server.MaxModels 3, got %d", cfg.Server.MaxModels)
	}
	if cfg.Server.IdleTimeoutMins != 10 {
		t.Errorf("Expected Server.IdleTimeoutMins 10, got %d", cfg.Server.IdleTimeoutMins)
	}
	if cfg.Server.StartupTimeoutS != 120 {
		t.Errorf("Expected Server.StartupTimeoutS 120, got %d", cfg.Server.StartupTimeoutS)
	}
	if cfg.Server.BackendPortMin != 49152 {
		t.Errorf("Expected Server.BackendPortMin 49152, got %d", cfg.Server.BackendPortMin)
	}
	if cfg.Server.BackendPortMax != 49200 {
		t.Errorf("Expected Server.BackendPortMax 49200, got %d", cfg.Server.BackendPortMax)
	}
}

func TestLoad(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)

	os.Setenv("HOME", tmpDir)

	t.Run("returns default config when file does not exist", func(t *testing.T) {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if cfg == nil {
			t.Fatal("Expected config to be non-nil")
		}

		if cfg.LlamaCpp.ContextLength != 4096 {
			t.Errorf("Expected default LlamaCpp.ContextLength 4096, got %d", cfg.LlamaCpp.ContextLength)
		}
	})

	t.Run("parses valid config file", func(t *testing.T) {
		configDir := filepath.Join(tmpDir, ".llemme")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create test config dir: %v", err)
		}

		configContent := `huggingface:
  token: test-token
  default_quant: Q5_K
llamacpp:
  server_path: /custom/path
  context_length: 2048
  gpu_layers: 35
  temperature: 0.8
  top_p: 0.95
  top_k: 50
server:
  host: 0.0.0.0
  port: 9000
  max_models: 5
  idle_timeout_mins: 15
  startup_timeout_secs: 60
  backend_port_min: 50000
  backend_port_max: 50100
`
		configPath := filepath.Join(configDir, "config.yaml")
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("Failed to write test config: %v", err)
		}

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// HuggingFace
		if cfg.HuggingFace.Token != "test-token" {
			t.Errorf("Expected HuggingFace.Token test-token, got %s", cfg.HuggingFace.Token)
		}
		if cfg.HuggingFace.DefaultQuant != "Q5_K" {
			t.Errorf("Expected HuggingFace.DefaultQuant Q5_K, got %s", cfg.HuggingFace.DefaultQuant)
		}

		// LlamaCpp
		if cfg.LlamaCpp.ServerPath != "/custom/path" {
			t.Errorf("Expected LlamaCpp.ServerPath /custom/path, got %s", cfg.LlamaCpp.ServerPath)
		}
		if cfg.LlamaCpp.ContextLength != 2048 {
			t.Errorf("Expected LlamaCpp.ContextLength 2048, got %d", cfg.LlamaCpp.ContextLength)
		}
		if cfg.LlamaCpp.GPULayers != 35 {
			t.Errorf("Expected LlamaCpp.GPULayers 35, got %d", cfg.LlamaCpp.GPULayers)
		}
		if cfg.LlamaCpp.Temperature != 0.8 {
			t.Errorf("Expected LlamaCpp.Temperature 0.8, got %f", cfg.LlamaCpp.Temperature)
		}
		if cfg.LlamaCpp.TopP != 0.95 {
			t.Errorf("Expected LlamaCpp.TopP 0.95, got %f", cfg.LlamaCpp.TopP)
		}
		if cfg.LlamaCpp.TopK != 50 {
			t.Errorf("Expected LlamaCpp.TopK 50, got %d", cfg.LlamaCpp.TopK)
		}

		// Server
		if cfg.Server.Host != "0.0.0.0" {
			t.Errorf("Expected Server.Host 0.0.0.0, got %s", cfg.Server.Host)
		}
		if cfg.Server.Port != 9000 {
			t.Errorf("Expected Server.Port 9000, got %d", cfg.Server.Port)
		}
		if cfg.Server.MaxModels != 5 {
			t.Errorf("Expected Server.MaxModels 5, got %d", cfg.Server.MaxModels)
		}
		if cfg.Server.IdleTimeoutMins != 15 {
			t.Errorf("Expected Server.IdleTimeoutMins 15, got %d", cfg.Server.IdleTimeoutMins)
		}
		if cfg.Server.StartupTimeoutS != 60 {
			t.Errorf("Expected Server.StartupTimeoutS 60, got %d", cfg.Server.StartupTimeoutS)
		}
		if cfg.Server.BackendPortMin != 50000 {
			t.Errorf("Expected Server.BackendPortMin 50000, got %d", cfg.Server.BackendPortMin)
		}
		if cfg.Server.BackendPortMax != 50100 {
			t.Errorf("Expected Server.BackendPortMax 50100, got %d", cfg.Server.BackendPortMax)
		}
	})

	t.Run("returns error for invalid YAML", func(t *testing.T) {
		configDir := filepath.Join(tmpDir, ".llemme")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create test config dir: %v", err)
		}

		configPath := filepath.Join(configDir, "config.yaml")
		if err := os.WriteFile(configPath, []byte("invalid: yaml: content:"), 0644); err != nil {
			t.Fatalf("Failed to write test config: %v", err)
		}

		_, err := Load()
		if err == nil {
			t.Error("Expected error for invalid YAML, got nil")
		}
	})
}

func TestSave(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)

	os.Setenv("HOME", tmpDir)

	cfg := &Config{
		HuggingFace: HuggingFace{
			Token:        "test-token",
			DefaultQuant: "Q2_K",
		},
		LlamaCpp: LlamaCpp{
			ServerPath:    "/test/path",
			ContextLength: 1024,
			GPULayers:     20,
			Temperature:   0.5,
			TopP:          0.8,
			TopK:          30,
		},
		Server: Server{
			Host:            "localhost",
			Port:            7000,
			MaxModels:       2,
			IdleTimeoutMins: 5,
			StartupTimeoutS: 30,
			BackendPortMin:  40000,
			BackendPortMax:  40100,
		},
	}

	err := Save(cfg)
	if err != nil {
		t.Fatalf("Expected no error saving config, got %v", err)
	}

	configPath := filepath.Join(tmpDir, ".llemme", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read saved config: %v", err)
	}

	content := string(data)
	if content == "" {
		t.Error("Expected non-empty config file")
	}

	expectedFields := []string{
		"token: test-token",
		"default_quant: Q2_K",
		"server_path: /test/path",
		"context_length: 1024",
		"gpu_layers: 20",
		"temperature: 0.5",
		"top_p: 0.8",
		"top_k: 30",
		"host: localhost",
		"port: 7000",
		"max_models: 2",
		"idle_timeout_mins: 5",
		"startup_timeout_secs: 30",
		"backend_port_min: 40000",
		"backend_port_max: 40100",
	}

	for _, field := range expectedFields {
		if !contains(content, field) {
			t.Errorf("Expected config to contain '%s', got:\n%s", field, content)
		}
	}
}

func TestEnsureDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)

	os.Setenv("HOME", tmpDir)

	err := EnsureDirectories()
	if err != nil {
		t.Fatalf("Expected no error creating directories, got %v", err)
	}

	baseDir := filepath.Join(tmpDir, ".llemme")

	expectedDirs := []string{
		baseDir,
		filepath.Join(baseDir, "models"),
		filepath.Join(baseDir, "bin"),
		filepath.Join(baseDir, "blobs"),
	}

	for _, dir := range expectedDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Expected directory %s to exist", dir)
		}
	}
}

func TestPathHelpers(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)

	os.Setenv("HOME", tmpDir)

	configPath := ConfigPath()
	expectedConfigPath := filepath.Join(tmpDir, ".llemme", "config.yaml")
	if configPath != expectedConfigPath {
		t.Errorf("Expected ConfigPath %s, got %s", expectedConfigPath, configPath)
	}

	modelsPath := ModelsPath()
	expectedModelsPath := filepath.Join(tmpDir, ".llemme", "models")
	if modelsPath != expectedModelsPath {
		t.Errorf("Expected ModelsPath %s, got %s", expectedModelsPath, modelsPath)
	}

	binPath := BinPath()
	expectedBinPath := filepath.Join(tmpDir, ".llemme", "bin")
	if binPath != expectedBinPath {
		t.Errorf("Expected BinPath %s, got %s", expectedBinPath, binPath)
	}

	blobsPath := BlobsPath()
	expectedBlobsPath := filepath.Join(tmpDir, ".llemme", "blobs")
	if blobsPath != expectedBlobsPath {
		t.Errorf("Expected BlobsPath %s, got %s", expectedBlobsPath, blobsPath)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && indexOfSubstring(s, substr) >= 0))
}

func indexOfSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
