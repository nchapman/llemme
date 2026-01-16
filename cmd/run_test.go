package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseModelRef(t *testing.T) {
	tests := []struct {
		input     string
		wantUser  string
		wantRepo  string
		wantQuant string
		wantErr   bool
	}{
		{"user/repo", "user", "repo", "", false},
		{"user/repo:Q4_K", "user", "repo", "Q4_K", false},
		{"user/repo:Q6_K.gguf", "user", "repo", "Q6_K.gguf", false},
		{"user", "", "", "", true},
		{"user/repo:too:many:colons", "", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			user, repo, quant, err := parseModelRef(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseModelRef() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if user != tt.wantUser {
					t.Errorf("parseModelRef() user = %v, want %v", user, tt.wantUser)
				}
				if repo != tt.wantRepo {
					t.Errorf("parseModelRef() repo = %v, want %v", repo, tt.wantRepo)
				}
				if quant != tt.wantQuant {
					t.Errorf("parseModelRef() quant = %v, want %v", quant, tt.wantQuant)
				}
			}
		})
	}
}

func TestFindModelInDir(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("no gguf files", func(t *testing.T) {
		result := findModelInDir(tmpDir)
		if result != "" {
			t.Errorf("findModelInDir() = %v, want empty string", result)
		}
	})

	t.Run("one gguf file", func(t *testing.T) {
		ggufPath := filepath.Join(tmpDir, "model.Q4_K.gguf")
		if err := os.WriteFile(ggufPath, []byte("dummy"), 0644); err != nil {
			t.Fatal(err)
		}

		result := findModelInDir(tmpDir)
		if result != ggufPath {
			t.Errorf("findModelInDir() = %v, want %v", result, ggufPath)
		}
	})

	t.Run("multiple gguf files", func(t *testing.T) {
		for _, name := range []string{"model.Q4_K.gguf", "model.Q6_K.gguf"} {
			if err := os.WriteFile(filepath.Join(tmpDir, name), []byte("dummy"), 0644); err != nil {
				t.Fatal(err)
			}
		}

		result := findModelInDir(tmpDir)
		if result == "" {
			t.Error("findModelInDir() returned empty string, want a gguf file")
		}
	})
}

func TestExtractQuantFromPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/path/to/model.Q4_K.gguf", "model.Q4_K.gguf"},
		{"/path/to/model.Q6_K.gguf", "model.Q6_K.gguf"},
		{"model.Q8_0.gguf", "model.Q8_0.gguf"},
		{"/path/to/", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractQuantFromPath(tt.input)
			if result != tt.want {
				t.Errorf("extractQuantFromPath() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
		{1024 * 1024 * 1024 * 1024, "1.0 TB"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			result := formatBytes(tt.input)
			if result != tt.want {
				t.Errorf("formatBytes() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestIsPipedInput(t *testing.T) {
	t.Run("not piped", func(t *testing.T) {
		result := isPipedInput()
		if result {
			t.Error("isPipedInput() = true, want false")
		}
	})
}

func TestParseCommandArgs(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantPrompt string
		wantEmpty  bool
	}{
		{
			name:       "with prompt",
			args:       []string{"model:quant", "Hello", "world"},
			wantPrompt: "Hello world",
			wantEmpty:  false,
		},
		{
			name:       "single word prompt",
			args:       []string{"model:quant", "Hello"},
			wantPrompt: "Hello",
			wantEmpty:  false,
		},
		{
			name:       "no prompt",
			args:       []string{"model:quant"},
			wantPrompt: "",
			wantEmpty:  true,
		},
		{
			name:       "empty string prompt",
			args:       []string{"model:quant", ""},
			wantPrompt: "",
			wantEmpty:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			promptArg := ""
			if len(tt.args) > 1 {
				promptArg = strings.Join(tt.args[1:], " ")
			}

			isEmpty := promptArg == ""

			if isEmpty != tt.wantEmpty {
				t.Errorf("promptArg empty = %v, want %v", isEmpty, tt.wantEmpty)
			}

			if !isEmpty && promptArg != tt.wantPrompt {
				t.Errorf("promptArg = %v, want %v", promptArg, tt.wantPrompt)
			}
		})
	}
}
