package cmd

import (
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
