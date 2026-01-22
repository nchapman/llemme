package hf

import (
	"testing"
)

func TestParseQuantization(t *testing.T) {
	tests := []struct {
		name           string
		filename       string
		wantNormalized string
		wantRaw        string
	}{
		{
			name:           "Q4_K_M",
			filename:       "model-Q4_K_M.gguf",
			wantNormalized: "Q4_K_M",
			wantRaw:        "Q4_K_M",
		},
		{
			name:           "Q5_K_S",
			filename:       "model-Q5_K_S.gguf",
			wantNormalized: "Q5_K_S",
			wantRaw:        "Q5_K_S",
		},
		{
			name:           "Q6_K",
			filename:       "model-Q6_K.gguf",
			wantNormalized: "Q6_K",
			wantRaw:        "Q6_K",
		},
		{
			name:           "Q8_0",
			filename:       "model-Q8_0.gguf",
			wantNormalized: "Q8_0",
			wantRaw:        "Q8_0",
		},
		{
			name:           "Q4_0",
			filename:       "model-Q4_0.gguf",
			wantNormalized: "Q4_0",
			wantRaw:        "Q4_0",
		},
		{
			name:           "FP16",
			filename:       "model-FP16.gguf",
			wantNormalized: "FP16",
			wantRaw:        "FP16",
		},
		{
			name:           "FP32",
			filename:       "model-FP32.gguf",
			wantNormalized: "FP32",
			wantRaw:        "FP32",
		},
		{
			name:           "F16 normalization",
			filename:       "model-F16.gguf",
			wantNormalized: "FP16",
			wantRaw:        "F16",
		},
		{
			name:           "F32 normalization",
			filename:       "model-F32.gguf",
			wantNormalized: "FP32",
			wantRaw:        "F32",
		},
		{
			name:           "I8 normalization",
			filename:       "model-I8.gguf",
			wantNormalized: "Q8_0",
			wantRaw:        "I8",
		},
		{
			name:           "I4 normalization",
			filename:       "model-I4.gguf",
			wantNormalized: "Q4_0",
			wantRaw:        "I4",
		},
		{
			name:           "lowercase",
			filename:       "model-q4-k-m.gguf",
			wantNormalized: "Q4_K_M",
			wantRaw:        "Q4_K_M",
		},
		{
			name:           "underscore separator",
			filename:       "model_Q4_K_M.gguf",
			wantNormalized: "Q4_K_M",
			wantRaw:        "Q4_K_M",
		},
		{
			name:           "dot separator",
			filename:       "model.Q4_K_M.gguf",
			wantNormalized: "Q4_K_M",
			wantRaw:        "Q4_K_M",
		},
		{
			name:           "no quantization",
			filename:       "model.gguf",
			wantNormalized: "",
			wantRaw:        "",
		},
		{
			name:           "not a gguf file",
			filename:       "model.bin",
			wantNormalized: "",
			wantRaw:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNorm, gotRaw := ParseQuantization(tt.filename)
			if gotNorm != tt.wantNormalized {
				t.Errorf("ParseQuantization() normalized = %v, want %v", gotNorm, tt.wantNormalized)
			}
			if gotRaw != tt.wantRaw {
				t.Errorf("ParseQuantization() raw = %v, want %v", gotRaw, tt.wantRaw)
			}
		})
	}
}

func TestExtractQuantizations(t *testing.T) {
	files := []FileTree{
		{Path: "model-Q4_K_M.gguf", Size: 4000000000},
		{Path: "model-Q5_K_S.gguf", Size: 5000000000},
		{Path: "model-Q6_K.gguf", Size: 6000000000},
		{Path: "README.md", Size: 1024},
		{Path: "config.json", Size: 2048},
		{Path: "tokenizer.json", Size: 512},
	}

	quants := ExtractQuantizations(files)

	if len(quants) != 3 {
		t.Errorf("ExtractQuantizations() got %d quants, want 3", len(quants))
	}

	wantNames := []string{"Q4_K_M", "Q5_K_S", "Q6_K"}
	for i, want := range wantNames {
		if i >= len(quants) {
			break
		}
		if quants[i].Name != want {
			t.Errorf("ExtractQuantizations()[%d].Name = %v, want %v", i, quants[i].Name, want)
		}
	}
}

func TestExtractQuantizationsEmpty(t *testing.T) {
	files := []FileTree{
		{Path: "README.md", Size: 1024},
		{Path: "config.json", Size: 2048},
	}

	quants := ExtractQuantizations(files)

	if len(quants) != 0 {
		t.Errorf("ExtractQuantizations() got %d quants, want 0", len(quants))
	}
}

func TestExtractQuantizationsNoSuffix(t *testing.T) {
	files := []FileTree{
		{Path: "gemma-7b.gguf", Size: 34200000000},
		{Path: "README.md", Size: 1024},
		{Path: "config.json", Size: 2048},
	}

	quants := ExtractQuantizations(files)

	if len(quants) != 1 {
		t.Errorf("ExtractQuantizations() got %d quants, want 1", len(quants))
	}

	if quants[0].Name != "default" {
		t.Errorf("ExtractQuantizations()[0].Name = %v, want 'default'", quants[0].Name)
	}

	if quants[0].Tag != "latest" {
		t.Errorf("ExtractQuantizations()[0].Tag = %v, want 'latest'", quants[0].Tag)
	}

	if quants[0].File != "gemma-7b.gguf" {
		t.Errorf("ExtractQuantizations()[0].File = %v, want 'gemma-7b.gguf'", quants[0].File)
	}
}

func TestExtractQuantizationsMixed(t *testing.T) {
	files := []FileTree{
		{Path: "model.gguf", Size: 10000000000},
		{Path: "model-Q4_K_M.gguf", Size: 4000000000},
		{Path: "README.md", Size: 1024},
	}

	quants := ExtractQuantizations(files)

	if len(quants) != 2 {
		t.Errorf("ExtractQuantizations() got %d quants, want 2", len(quants))
	}

	// Check that both default and Q4_K_M are found
	names := make(map[string]bool)
	for _, q := range quants {
		names[q.Name] = true
	}

	if !names["default"] {
		t.Error("ExtractQuantizations() missing 'default' quant")
	}
	if !names["Q4_K_M"] {
		t.Error("ExtractQuantizations() missing 'Q4_K_M' quant")
	}
}

func TestExtractQuantizationsF16Tag(t *testing.T) {
	// Test that F16 files have the correct raw tag for HF API calls
	files := []FileTree{
		{Path: "model-F16.gguf", Size: 65000000000},
		{Path: "README.md", Size: 1024},
	}

	quants := ExtractQuantizations(files)

	if len(quants) != 1 {
		t.Fatalf("ExtractQuantizations() got %d quants, want 1", len(quants))
	}

	// Name should be normalized (FP16)
	if quants[0].Name != "FP16" {
		t.Errorf("ExtractQuantizations()[0].Name = %v, want 'FP16'", quants[0].Name)
	}

	// Tag should be raw (F16) for HuggingFace API calls
	if quants[0].Tag != "F16" {
		t.Errorf("ExtractQuantizations()[0].Tag = %v, want 'F16'", quants[0].Tag)
	}
}

func TestExtractQuantizationsFromDirectories(t *testing.T) {
	// Test extraction from directories (like unsloth/gpt-oss-120b-GGUF)
	files := []FileTree{
		{Path: "Q4_K_M", Type: "directory"},
		{Path: "Q5_K_S", Type: "directory"},
		{Path: "Q8_0", Type: "directory"},
		{Path: "gpt-oss-120b-F16.gguf", Size: 65000000000},
		{Path: "README.md", Size: 1024},
		{Path: "config.json", Size: 2048},
	}

	quants := ExtractQuantizations(files)

	if len(quants) != 4 {
		t.Fatalf("ExtractQuantizations() got %d quants, want 4", len(quants))
	}

	// Check that all expected quants are found
	names := make(map[string]string) // name -> tag
	for _, q := range quants {
		names[q.Name] = q.Tag
	}

	// Directory-based quants
	if tag, ok := names["Q4_K_M"]; !ok {
		t.Error("ExtractQuantizations() missing 'Q4_K_M' quant")
	} else if tag != "Q4_K_M" {
		t.Errorf("Q4_K_M tag = %q, want 'Q4_K_M'", tag)
	}

	if _, ok := names["Q5_K_S"]; !ok {
		t.Error("ExtractQuantizations() missing 'Q5_K_S' quant")
	}

	if _, ok := names["Q8_0"]; !ok {
		t.Error("ExtractQuantizations() missing 'Q8_0' quant")
	}

	// File-based quant (F16 -> FP16)
	if tag, ok := names["FP16"]; !ok {
		t.Error("ExtractQuantizations() missing 'FP16' quant")
	} else if tag != "F16" {
		t.Errorf("FP16 tag = %q, want 'F16'", tag)
	}
}

func TestGetBestQuantization(t *testing.T) {
	tests := []struct {
		name   string
		quants []Quantization
		want   string
	}{
		{
			name: "Q4_K_M preferred",
			quants: []Quantization{
				{Name: "Q5_K_S"},
				{Name: "Q4_K_M"},
				{Name: "Q6_K"},
			},
			want: "Q4_K_M",
		},
		{
			name: "Q5_K_M preferred",
			quants: []Quantization{
				{Name: "Q6_K"},
				{Name: "Q5_K_M"},
				{Name: "Q8_0"},
			},
			want: "Q5_K_M",
		},
		{
			name: "no preferred quant",
			quants: []Quantization{
				{Name: "I8"},
				{Name: "I4"},
			},
			want: "I8",
		},
		{
			name:   "empty",
			quants: []Quantization{},
			want:   "",
		},
		{
			name: "default only",
			quants: []Quantization{
				{Name: "default"},
			},
			want: "default",
		},
		{
			name: "default with preferred quants",
			quants: []Quantization{
				{Name: "default"},
				{Name: "Q4_K_M"},
			},
			want: "Q4_K_M",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetBestQuantization(tt.quants)
			if got != tt.want {
				t.Errorf("GetBestQuantization() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSortQuantizations(t *testing.T) {
	quants := []Quantization{
		{Name: "Q8_0", Size: 8000000000},
		{Name: "Q4_K_M", Size: 4000000000},
		{Name: "Q6_K", Size: 6000000000},
		{Name: "Q5_K_S", Size: 5000000000},
	}

	sorted := SortQuantizations(quants)

	wantOrder := []string{"Q4_K_M", "Q5_K_S", "Q6_K", "Q8_0"}
	for i, want := range wantOrder {
		if sorted[i].Name != want {
			t.Errorf("SortQuantizations()[%d].Name = %v, want %v", i, sorted[i].Name, want)
		}
	}
}

func TestFindQuantization(t *testing.T) {
	quants := []Quantization{
		{Name: "Q4_K_M", File: "model.Q4_K_M.gguf", Size: 4000000000},
		{Name: "Q5_K_S", File: "model.Q5_K_S.gguf", Size: 5000000000},
		{Name: "Q6_K", File: "model.Q6_K.gguf", Size: 6000000000},
	}

	tests := []struct {
		name    string
		quant   string
		want    Quantization
		wantErr bool
	}{
		{
			name:  "found",
			quant: "Q4_K_M",
			want:  Quantization{Name: "Q4_K_M", File: "model.Q4_K_M.gguf", Size: 4000000000},
		},
		{
			name:  "case insensitive",
			quant: "q4_k_m",
			want:  Quantization{Name: "Q4_K_M", File: "model.Q4_K_M.gguf", Size: 4000000000},
		},
		{
			name:    "not found",
			quant:   "Q9_0",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := FindQuantization(quants, tt.quant)
			if found != !tt.wantErr {
				t.Errorf("FindQuantization() found = %v, want %v", found, !tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("FindQuantization() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFindQuantizationWithNormalizedName(t *testing.T) {
	// Simulate quantizations extracted from files with raw tags
	// This tests the real-world scenario where a user requests "FP16"
	// but the file was named "model-F16.gguf"
	quants := []Quantization{
		{Name: "FP16", Tag: "F16", File: "model-F16.gguf", Size: 65000000000},
		{Name: "Q4_K_M", Tag: "Q4_K_M", File: "model-Q4_K_M.gguf", Size: 4000000000},
	}

	// Should find FP16 by normalized name
	got, found := FindQuantization(quants, "FP16")
	if !found {
		t.Fatal("FindQuantization() should find FP16 by Name")
	}
	if got.Name != "FP16" {
		t.Errorf("FindQuantization().Name = %v, want FP16", got.Name)
	}
	if got.Tag != "F16" {
		t.Errorf("FindQuantization().Tag = %v, want F16", got.Tag)
	}

	// Should also find by raw tag (case insensitive)
	got, found = FindQuantization(quants, "f16")
	if !found {
		t.Fatal("FindQuantization() should find FP16 by Tag 'f16'")
	}
	if got.Name != "FP16" {
		t.Errorf("FindQuantization().Name = %v, want FP16", got.Name)
	}
	if got.Tag != "F16" {
		t.Errorf("FindQuantization().Tag = %v, want F16", got.Tag)
	}
}

func TestIsGGUFFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{
			name:     "gguf file",
			filename: "model.gguf",
			want:     true,
		},
		{
			name:     "gguf with quant",
			filename: "model.Q4_K_M.gguf",
			want:     true,
		},
		{
			name:     "not gguf",
			filename: "model.bin",
			want:     false,
		},
		{
			name:     "json file",
			filename: "config.json",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsGGUFFile(tt.filename)
			if got != tt.want {
				t.Errorf("IsGGUFFile() = %v, want %v", got, tt.want)
			}
		})
	}
}
