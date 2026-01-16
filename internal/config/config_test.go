package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.ContextLength != 4096 {
		t.Errorf("Expected ContextLength 4096, got %d", cfg.ContextLength)
	}
	if cfg.Temperature != 0.7 {
		t.Errorf("Expected Temperature 0.7, got %f", cfg.Temperature)
	}
	if cfg.TopP != 0.9 {
		t.Errorf("Expected TopP 0.9, got %f", cfg.TopP)
	}
	if cfg.TopK != 40 {
		t.Errorf("Expected TopK 40, got %d", cfg.TopK)
	}
	if cfg.DefaultQuant != "Q4_K_M" {
		t.Errorf("Expected DefaultQuant Q4_K_M, got %s", cfg.DefaultQuant)
	}
	if cfg.GPULayers != -1 {
		t.Errorf("Expected GPULayers -1, got %d", cfg.GPULayers)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("Expected Server.Host 127.0.0.1, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Expected Server.Port 8080, got %d", cfg.Server.Port)
	}
	if len(cfg.Server.Preload) != 0 {
		t.Errorf("Expected empty Preload, got %v", cfg.Server.Preload)
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

		if cfg.ContextLength != 4096 {
			t.Errorf("Expected default ContextLength 4096, got %d", cfg.ContextLength)
		}
	})

	t.Run("parses valid config file", func(t *testing.T) {
		configDir := filepath.Join(tmpDir, ".lemme")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create test config dir: %v", err)
		}

		configContent := `context_length: 2048
temperature: 0.8
top_p: 0.95
top_k: 50
default_quant: Q5_K
gpu_layers: 35
llama_path: /custom/path
hf_token: test-token
server:
  host: 0.0.0.0
  port: 9000
  preload:
    - model1
    - model2
`
		configPath := filepath.Join(configDir, "config.yaml")
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("Failed to write test config: %v", err)
		}

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if cfg.ContextLength != 2048 {
			t.Errorf("Expected ContextLength 2048, got %d", cfg.ContextLength)
		}
		if cfg.Temperature != 0.8 {
			t.Errorf("Expected Temperature 0.8, got %f", cfg.Temperature)
		}
		if cfg.TopP != 0.95 {
			t.Errorf("Expected TopP 0.95, got %f", cfg.TopP)
		}
		if cfg.TopK != 50 {
			t.Errorf("Expected TopK 50, got %d", cfg.TopK)
		}
		if cfg.DefaultQuant != "Q5_K" {
			t.Errorf("Expected DefaultQuant Q5_K, got %s", cfg.DefaultQuant)
		}
		if cfg.GPULayers != 35 {
			t.Errorf("Expected GPULayers 35, got %d", cfg.GPULayers)
		}
		if cfg.LLamaPath != "/custom/path" {
			t.Errorf("Expected LLamaPath /custom/path, got %s", cfg.LLamaPath)
		}
		if cfg.HFToken != "test-token" {
			t.Errorf("Expected HFToken test-token, got %s", cfg.HFToken)
		}
		if cfg.Server.Host != "0.0.0.0" {
			t.Errorf("Expected Server.Host 0.0.0.0, got %s", cfg.Server.Host)
		}
		if cfg.Server.Port != 9000 {
			t.Errorf("Expected Server.Port 9000, got %d", cfg.Server.Port)
		}
		if len(cfg.Server.Preload) != 2 {
			t.Errorf("Expected 2 preload models, got %d", len(cfg.Server.Preload))
		}
		if cfg.Server.Preload[0] != "model1" || cfg.Server.Preload[1] != "model2" {
			t.Errorf("Expected preload [model1, model2], got %v", cfg.Server.Preload)
		}
	})

	t.Run("returns error for invalid YAML", func(t *testing.T) {
		configDir := filepath.Join(tmpDir, ".lemme")
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
		ContextLength: 1024,
		Temperature:   0.5,
		TopP:          0.8,
		TopK:          30,
		DefaultQuant:  "Q2_K",
		GPULayers:     20,
		LLamaPath:     "/test/path",
		HFToken:       "test-token",
		Server: Server{
			Host:    "localhost",
			Port:    7000,
			Preload: []string{"model1"},
		},
	}

	err := Save(cfg)
	if err != nil {
		t.Fatalf("Expected no error saving config, got %v", err)
	}

	configPath := filepath.Join(tmpDir, ".lemme", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read saved config: %v", err)
	}

	content := string(data)
	if content == "" {
		t.Error("Expected non-empty config file")
	}

	expectedFields := []string{
		"context_length: 1024",
		"temperature: 0.5",
		"top_p: 0.8",
		"top_k: 30",
		"default_quant: Q2_K",
		"gpu_layers: 20",
		"llama_path: /test/path",
		"hf_token: test-token",
		"host: localhost",
		"port: 7000",
		"- model1",
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

	baseDir := filepath.Join(tmpDir, ".lemme")

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
	expectedConfigPath := filepath.Join(tmpDir, ".lemme", "config.yaml")
	if configPath != expectedConfigPath {
		t.Errorf("Expected ConfigPath %s, got %s", expectedConfigPath, configPath)
	}

	modelsPath := ModelsPath()
	expectedModelsPath := filepath.Join(tmpDir, ".lemme", "models")
	if modelsPath != expectedModelsPath {
		t.Errorf("Expected ModelsPath %s, got %s", expectedModelsPath, modelsPath)
	}

	binPath := BinPath()
	expectedBinPath := filepath.Join(tmpDir, ".lemme", "bin")
	if binPath != expectedBinPath {
		t.Errorf("Expected BinPath %s, got %s", expectedBinPath, binPath)
	}

	blobsPath := BlobsPath()
	expectedBlobsPath := filepath.Join(tmpDir, ".lemme", "blobs")
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
