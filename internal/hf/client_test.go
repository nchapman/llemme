package hf

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/nchapman/lleme/internal/config"
)

func TestGatedStatusUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "bool false",
			input: `{"gated": false}`,
			want:  false,
		},
		{
			name:  "bool true",
			input: `{"gated": true}`,
			want:  true,
		},
		{
			name:  "string manual",
			input: `{"gated": "manual"}`,
			want:  true,
		},
		{
			name:  "string auto",
			input: `{"gated": "auto"}`,
			want:  true,
		},
		{
			name:  "null value treated as not gated",
			input: `{"gated": null}`,
			want:  false,
		},
		{
			name:  "numeric value treated as gated",
			input: `{"gated": 1}`,
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result struct {
				Gated GatedStatus `json:"gated"`
			}
			if err := json.Unmarshal([]byte(tt.input), &result); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			if bool(result.Gated) != tt.want {
				t.Errorf("GatedStatus = %v, want %v", result.Gated, tt.want)
			}
		})
	}
}

func TestHasToken(t *testing.T) {
	// Save original env and restore after test
	origToken := os.Getenv("HF_TOKEN")
	defer os.Setenv("HF_TOKEN", origToken)

	t.Run("from env var", func(t *testing.T) {
		os.Setenv("HF_TOKEN", "test-token")
		defer os.Unsetenv("HF_TOKEN")

		cfg := &config.Config{}
		if !HasToken(cfg) {
			t.Error("HasToken() = false, want true when HF_TOKEN is set")
		}
	})

	t.Run("from config", func(t *testing.T) {
		os.Unsetenv("HF_TOKEN")

		cfg := &config.Config{
			HuggingFace: config.HuggingFace{
				Token: "config-token",
			},
		}
		if !HasToken(cfg) {
			t.Error("HasToken() = false, want true when config token is set")
		}
	})

	t.Run("from cache file", func(t *testing.T) {
		os.Unsetenv("HF_TOKEN")

		// Create temp home dir with token file
		tmpDir := t.TempDir()
		oldHome := os.Getenv("HOME")
		defer os.Setenv("HOME", oldHome)
		os.Setenv("HOME", tmpDir)

		tokenDir := filepath.Join(tmpDir, ".cache", "huggingface")
		if err := os.MkdirAll(tokenDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tokenDir, "token"), []byte("file-token"), 0600); err != nil {
			t.Fatal(err)
		}

		cfg := &config.Config{}
		if !HasToken(cfg) {
			t.Error("HasToken() = false, want true when cache file token exists")
		}
	})

	t.Run("from cache file with trailing newline", func(t *testing.T) {
		os.Unsetenv("HF_TOKEN")

		// Create temp home dir with token file containing trailing newline
		tmpDir := t.TempDir()
		oldHome := os.Getenv("HOME")
		defer os.Setenv("HOME", oldHome)
		os.Setenv("HOME", tmpDir)

		tokenDir := filepath.Join(tmpDir, ".cache", "huggingface")
		if err := os.MkdirAll(tokenDir, 0755); err != nil {
			t.Fatal(err)
		}
		// Token with trailing newline (common when created by editors or echo)
		if err := os.WriteFile(filepath.Join(tokenDir, "token"), []byte("file-token\n"), 0600); err != nil {
			t.Fatal(err)
		}

		cfg := &config.Config{}
		if !HasToken(cfg) {
			t.Error("HasToken() = false, want true when cache file token exists with newline")
		}
	})

	t.Run("no token", func(t *testing.T) {
		os.Unsetenv("HF_TOKEN")

		// Use temp home dir with no token file
		tmpDir := t.TempDir()
		oldHome := os.Getenv("HOME")
		defer os.Setenv("HOME", oldHome)
		os.Setenv("HOME", tmpDir)

		cfg := &config.Config{}
		if HasToken(cfg) {
			t.Error("HasToken() = true, want false when no token is available")
		}
	})
}
