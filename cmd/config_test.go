package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nchapman/llemme/internal/config"
	"gopkg.in/yaml.v3"
)

func TestGetEditor(t *testing.T) {
	// Save original env vars
	origVisual := os.Getenv("VISUAL")
	origEditor := os.Getenv("EDITOR")
	defer func() {
		os.Setenv("VISUAL", origVisual)
		os.Setenv("EDITOR", origEditor)
	}()

	t.Run("prefers VISUAL over EDITOR", func(t *testing.T) {
		os.Setenv("VISUAL", "code")
		os.Setenv("EDITOR", "vim")

		editor := getEditor()
		if editor != "code" {
			t.Errorf("Expected 'code', got '%s'", editor)
		}
	})

	t.Run("uses EDITOR when VISUAL not set", func(t *testing.T) {
		os.Setenv("VISUAL", "")
		os.Setenv("EDITOR", "nano")

		editor := getEditor()
		if editor != "nano" {
			t.Errorf("Expected 'nano', got '%s'", editor)
		}
	})

	t.Run("falls back to common editors when env vars not set", func(t *testing.T) {
		os.Setenv("VISUAL", "")
		os.Setenv("EDITOR", "")

		editor := getEditor()
		// Should find at least one of nano, vim, vi on most systems
		if editor == "" {
			t.Skip("No fallback editor found on this system")
		}
		// Verify it found one of the expected fallbacks
		validEditors := []string{"nano", "vim", "vi"}
		found := false
		for _, valid := range validEditors {
			if strings.HasSuffix(editor, valid) || editor == valid {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected one of %v, got '%s'", validEditors, editor)
		}
	})

	t.Run("returns empty when no editor available", func(t *testing.T) {
		os.Setenv("VISUAL", "")
		os.Setenv("EDITOR", "")
		// Save PATH and set to empty to simulate no editors
		origPath := os.Getenv("PATH")
		os.Setenv("PATH", "")
		defer os.Setenv("PATH", origPath)

		editor := getEditor()
		if editor != "" {
			t.Errorf("Expected empty string when no editors available, got '%s'", editor)
		}
	})
}

func TestPrintConfigOutput(t *testing.T) {
	// Test that config can be loaded and marshaled to YAML
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	// Verify it's valid YAML by unmarshaling back
	var parsed config.Config
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Errorf("Config output is not valid YAML: %v", err)
	}

	// Verify key fields are present in output
	output := string(data)
	expectedFields := []string{
		"context_length:",
		"temperature:",
		"gpu_layers:",
		"server:",
		"proxy:",
	}
	for _, field := range expectedFields {
		if !strings.Contains(output, field) {
			t.Errorf("Expected config output to contain '%s'", field)
		}
	}
}

func TestConfigPath(t *testing.T) {
	path := config.ConfigPath()

	if path == "" {
		t.Error("Expected non-empty config path")
	}

	if !strings.HasSuffix(path, "config.yaml") {
		t.Errorf("Expected path to end with 'config.yaml', got '%s'", path)
	}

	if !strings.Contains(path, ".llemme") {
		t.Errorf("Expected path to contain '.llemme', got '%s'", path)
	}
}

func TestOpenInEditorCreatesDefaultConfig(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)
	os.Setenv("HOME", tmpDir)

	configPath := filepath.Join(tmpDir, ".llemme", "config.yaml")

	// Verify config doesn't exist
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatal("Config file should not exist initially")
	}

	// We can't actually test openInEditor without opening an editor,
	// but we can test the config creation logic separately
	cfg := config.DefaultConfig()
	if err := config.Save(cfg); err != nil {
		t.Fatalf("Failed to save default config: %v", err)
	}

	// Verify config was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file should have been created")
	}

	// Verify it contains valid config
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	var loaded config.Config
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		t.Errorf("Created config is not valid YAML: %v", err)
	}

	if loaded.ContextLength != 4096 {
		t.Errorf("Expected default context_length 4096, got %d", loaded.ContextLength)
	}
}

func TestConfigCommandFlags(t *testing.T) {
	// Test that flags are properly registered
	cmd := configCmd

	pathFlag := cmd.Flags().Lookup("path")
	if pathFlag == nil {
		t.Error("Expected --path flag to be registered")
	}

	showFlag := cmd.Flags().Lookup("show")
	if showFlag == nil {
		t.Error("Expected --show flag to be registered")
	}

	resetFlag := cmd.Flags().Lookup("reset")
	if resetFlag == nil {
		t.Error("Expected --reset flag to be registered")
	}
}

func TestConfigYAMLRoundTrip(t *testing.T) {
	// Test that config survives a round-trip through YAML
	original := config.DefaultConfig()
	original.Temperature = 0.5
	original.ContextLength = 8192
	original.GPULayers = 32

	// Marshal to YAML
	data, err := yaml.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal back
	var restored config.Config
	if err := yaml.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify values preserved
	if restored.Temperature != original.Temperature {
		t.Errorf("Temperature: expected %v, got %v", original.Temperature, restored.Temperature)
	}
	if restored.ContextLength != original.ContextLength {
		t.Errorf("ContextLength: expected %v, got %v", original.ContextLength, restored.ContextLength)
	}
	if restored.GPULayers != original.GPULayers {
		t.Errorf("GPULayers: expected %v, got %v", original.GPULayers, restored.GPULayers)
	}
}

func TestGetEditorWithPath(t *testing.T) {
	// Test that EDITOR with full path works
	origEditor := os.Getenv("EDITOR")
	origVisual := os.Getenv("VISUAL")
	defer func() {
		os.Setenv("EDITOR", origEditor)
		os.Setenv("VISUAL", origVisual)
	}()

	os.Setenv("VISUAL", "")
	os.Setenv("EDITOR", "/usr/bin/vim")

	editor := getEditor()
	if editor != "/usr/bin/vim" {
		t.Errorf("Expected '/usr/bin/vim', got '%s'", editor)
	}
}

func TestResetToDefaults(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)
	os.Setenv("HOME", tmpDir)

	configPath := filepath.Join(tmpDir, ".llemme", "config.yaml")

	// Create a modified config
	cfg := config.DefaultConfig()
	cfg.Temperature = 0.9
	cfg.ContextLength = 16384
	cfg.GPULayers = 99
	if err := config.Save(cfg); err != nil {
		t.Fatalf("Failed to save modified config: %v", err)
	}

	// Verify modified config was saved
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}
	var modified config.Config
	if err := yaml.Unmarshal(data, &modified); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}
	if modified.GPULayers != 99 {
		t.Fatalf("Expected modified gpu_layers 99, got %d", modified.GPULayers)
	}

	// Reset to defaults
	resetToDefaults(configPath)

	// Verify config was reset
	data, err = os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read reset config: %v", err)
	}
	var reset config.Config
	if err := yaml.Unmarshal(data, &reset); err != nil {
		t.Fatalf("Failed to unmarshal reset config: %v", err)
	}

	defaults := config.DefaultConfig()
	if reset.Temperature != defaults.Temperature {
		t.Errorf("Expected default temperature %v, got %v", defaults.Temperature, reset.Temperature)
	}
	if reset.ContextLength != defaults.ContextLength {
		t.Errorf("Expected default context_length %d, got %d", defaults.ContextLength, reset.ContextLength)
	}
	if reset.GPULayers != defaults.GPULayers {
		t.Errorf("Expected default gpu_layers %d, got %d", defaults.GPULayers, reset.GPULayers)
	}
}

// Silence the "imported and not used" error for bytes
var _ = bytes.Buffer{}
