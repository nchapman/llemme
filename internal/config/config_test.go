package config

import (
	"os"
	"path/filepath"
	"strings"
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

	// LlamaCpp defaults - should be empty (let llama-server use its defaults)
	if cfg.LlamaCpp.ServerPath != "" {
		t.Errorf("Expected empty LlamaCpp.ServerPath, got %s", cfg.LlamaCpp.ServerPath)
	}
	if cfg.LlamaCpp.Options != nil {
		t.Errorf("Expected nil LlamaCpp.Options, got %v", cfg.LlamaCpp.Options)
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

		if cfg.Server.Port != 8080 {
			t.Errorf("Expected default Server.Port 8080, got %d", cfg.Server.Port)
		}
	})

	t.Run("parses valid config file with options", func(t *testing.T) {
		configDir := filepath.Join(tmpDir, ".llemme")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create test config dir: %v", err)
		}

		configContent := `huggingface:
  token: test-token
  default_quant: Q5_K
llamacpp:
  server_path: /custom/path
  options:
    ctx-size: 2048
    gpu-layers: 35
    temp: 0.8
server:
  host: 0.0.0.0
  port: 9000
  max_models: 5
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

		// Options
		ctxSize := cfg.LlamaCpp.GetIntOption("ctx-size", 0)
		if ctxSize != 2048 {
			t.Errorf("Expected ctx-size 2048, got %d", ctxSize)
		}

		gpuLayers := cfg.LlamaCpp.GetIntOption("gpu-layers", 0)
		if gpuLayers != 35 {
			t.Errorf("Expected gpu-layers 35, got %d", gpuLayers)
		}

		temp := cfg.LlamaCpp.GetFloatOption("temp", 0)
		if temp != 0.8 {
			t.Errorf("Expected temp 0.8, got %f", temp)
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

func TestSaveDefault(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)

	os.Setenv("HOME", tmpDir)

	err := SaveDefault()
	if err != nil {
		t.Fatalf("Expected no error saving default config, got %v", err)
	}

	configPath := filepath.Join(tmpDir, ".llemme", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read saved config: %v", err)
	}

	content := string(data)

	// Should contain commented options
	expectedStrings := []string{
		"huggingface:",
		"llamacpp:",
		"server:",
		"# threads:",
		"# ctx-size:",
		"# gpu-layers:",
		"# temp:",
	}

	for _, s := range expectedStrings {
		if !strings.Contains(content, s) {
			t.Errorf("Expected config to contain '%s'", s)
		}
	}
}

func TestGetOptionHelpers(t *testing.T) {
	llama := &LlamaCpp{
		Options: map[string]any{
			"ctx-size":   4096,
			"temp":       0.7,
			"gpu-layers": -1,
			"mlock":      true,
		},
	}

	t.Run("GetIntOption", func(t *testing.T) {
		if v := llama.GetIntOption("ctx-size", 0); v != 4096 {
			t.Errorf("Expected 4096, got %d", v)
		}
		if v := llama.GetIntOption("gpu-layers", 0); v != -1 {
			t.Errorf("Expected -1, got %d", v)
		}
		if v := llama.GetIntOption("nonexistent", 999); v != 999 {
			t.Errorf("Expected default 999, got %d", v)
		}
	})

	t.Run("GetFloatOption", func(t *testing.T) {
		if v := llama.GetFloatOption("temp", 0); v != 0.7 {
			t.Errorf("Expected 0.7, got %f", v)
		}
		if v := llama.GetFloatOption("nonexistent", 0.5); v != 0.5 {
			t.Errorf("Expected default 0.5, got %f", v)
		}
	})

	t.Run("GetOption", func(t *testing.T) {
		if v, ok := llama.GetOption("mlock"); !ok || v != true {
			t.Errorf("Expected mlock=true, got %v, %v", v, ok)
		}
		if _, ok := llama.GetOption("nonexistent"); ok {
			t.Error("Expected nonexistent to not be found")
		}
	})

	t.Run("nil options", func(t *testing.T) {
		empty := &LlamaCpp{}
		if v := empty.GetIntOption("ctx-size", 100); v != 100 {
			t.Errorf("Expected default 100, got %d", v)
		}
	})
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
