package proxy

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPatchEmptyToolsArray(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "tools is not none",
			input:    `{% if tools is not none %}Use tools{% endif %}`,
			expected: `{% if (tools is not none and tools | length > 0) %}Use tools{% endif %}`,
		},
		{
			name:     "tools != none",
			input:    `{% if tools != none %}Use tools{% endif %}`,
			expected: `{% if (tools != none and tools | length > 0) %}Use tools{% endif %}`,
		},
		{
			name:     "not tools is none",
			input:    `{% if not tools is none %}Use tools{% endif %}`,
			expected: `{% if (not tools is none and tools | length > 0) %}Use tools{% endif %}`,
		},
		{
			name:     "multiple occurrences",
			input:    `{% if tools is not none %}first{% endif %}{% if tools is not none %}second{% endif %}`,
			expected: `{% if (tools is not none and tools | length > 0) %}first{% endif %}{% if (tools is not none and tools | length > 0) %}second{% endif %}`,
		},
		{
			name:     "no matching patterns",
			input:    `{% if messages %}{{ messages }}{% endif %}`,
			expected: `{% if messages %}{{ messages }}{% endif %}`,
		},
		{
			name:     "already patched - idempotent",
			input:    `{% if (tools is not none and tools | length > 0) %}Use tools{% endif %}`,
			expected: `{% if (tools is not none and tools | length > 0) %}Use tools{% endif %}`,
		},
		{
			name:     "mixed patched and unpatched",
			input:    `{% if (tools is not none and tools | length > 0) %}first{% endif %}{% if tools != none %}second{% endif %}`,
			expected: `{% if (tools is not none and tools | length > 0) %}first{% endif %}{% if (tools != none and tools | length > 0) %}second{% endif %}`,
		},
		{
			name:     "empty template",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := patchEmptyToolsArray.Apply(tt.input)
			if result != tt.expected {
				t.Errorf("Apply() =\n%s\nwant:\n%s", result, tt.expected)
			}
		})
	}
}

func TestPatchEmptyToolsArrayIdempotent(t *testing.T) {
	input := `{% if tools is not none %}Use tools{% endif %}`

	// Apply once
	first := patchEmptyToolsArray.Apply(input)

	// Apply twice
	second := patchEmptyToolsArray.Apply(first)

	// Apply three times
	third := patchEmptyToolsArray.Apply(second)

	if first != second {
		t.Errorf("Patch is not idempotent: first application differs from second")
	}
	if second != third {
		t.Errorf("Patch is not idempotent: second application differs from third")
	}
}

func TestPatchEmptyToolsArrayRealisticTemplate(t *testing.T) {
	// A realistic template snippet similar to what's found in Llama models
	input := `{%- if tools is not none %}
<|start_header_id|>system<|end_header_id|>

You have access to the following tools:
{%- for tool in tools %}
{{ tool | tojson }}
{%- endfor %}
<|eot_id|>
{%- endif %}
{%- for message in messages %}`

	result := patchEmptyToolsArray.Apply(input)

	// Should have replaced the check
	if strings.Contains(result, "tools is not none %}") && !strings.Contains(result, "tools | length > 0") {
		t.Error("Expected patch to replace 'tools is not none' with length check")
	}

	// Should still contain the rest of the template intact
	if !strings.Contains(result, "<|start_header_id|>system<|end_header_id|>") {
		t.Error("Template structure was damaged by patch")
	}
	if !strings.Contains(result, "for tool in tools") {
		t.Error("Template structure was damaged by patch")
	}
}

func TestApplyPatches(t *testing.T) {
	input := `{% if tools is not none %}tools{% endif %}`

	result := applyPatches(input)

	if !strings.Contains(result, "tools | length > 0") {
		t.Error("applyPatches did not apply the empty-tools-array patch")
	}
}

func TestApplyPatchesNoChanges(t *testing.T) {
	input := `{% if messages %}{{ messages }}{% endif %}`

	result := applyPatches(input)

	if result != input {
		t.Errorf("applyPatches modified template that needed no patches:\ninput:  %s\nresult: %s", input, result)
	}
}

func TestTemplatePatchMetadata(t *testing.T) {
	// Verify patch has required metadata
	if patchEmptyToolsArray.ID == "" {
		t.Error("Patch missing ID")
	}
	if patchEmptyToolsArray.Description == "" {
		t.Error("Patch missing Description")
	}
	if patchEmptyToolsArray.Apply == nil {
		t.Error("Patch missing Apply function")
	}
}

func TestAllPatchesHaveMetadata(t *testing.T) {
	for i, patch := range templatePatches {
		if patch.ID == "" {
			t.Errorf("Patch %d missing ID", i)
		}
		if patch.Description == "" {
			t.Errorf("Patch %d (%s) missing Description", i, patch.ID)
		}
		if patch.Apply == nil {
			t.Errorf("Patch %d (%s) missing Apply function", i, patch.ID)
		}
	}
}

func TestAllPatchesAreIdempotent(t *testing.T) {
	// Test input that could trigger any patch
	inputs := []string{
		`{% if tools is not none %}tools{% endif %}`,
		`{% if tools != none %}tools{% endif %}`,
		`{% if not tools is none %}tools{% endif %}`,
		`plain template without tools`,
	}

	for _, patch := range templatePatches {
		for _, input := range inputs {
			t.Run(patch.ID+"/"+input[:min(30, len(input))], func(t *testing.T) {
				first := patch.Apply(input)
				second := patch.Apply(first)
				if first != second {
					t.Errorf("Patch %s is not idempotent for input %q", patch.ID, input)
				}
			})
		}
	}
}

func TestWriteTemplateCache(t *testing.T) {
	tmpDir := t.TempDir()

	// Override config.BinPath for testing
	originalBinPath := os.Getenv("LLEMME_BIN_PATH")
	os.Setenv("LLEMME_BIN_PATH", tmpDir)
	defer os.Setenv("LLEMME_BIN_PATH", originalBinPath)

	// Create a fake model file (needed for mtime-based cache key)
	modelPath := filepath.Join(tmpDir, "model.gguf")
	if err := os.WriteFile(modelPath, []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}

	template := `{% if (tools is not none and tools | length > 0) %}tools{% endif %}`

	cachePath, err := writeTemplateCache(modelPath, template)
	if err != nil {
		t.Fatalf("writeTemplateCache() error = %v", err)
	}

	// Verify file was created
	if cachePath == "" {
		t.Fatal("writeTemplateCache returned empty path")
	}

	// Verify file exists and has correct content
	content, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("Failed to read cache file: %v", err)
	}
	if string(content) != template {
		t.Errorf("Cache file content = %q, want %q", string(content), template)
	}

	// Verify file has .jinja extension
	if filepath.Ext(cachePath) != ".jinja" {
		t.Errorf("Cache file extension = %q, want .jinja", filepath.Ext(cachePath))
	}
}

func TestWriteTemplateCacheConsistentHash(t *testing.T) {
	tmpDir := t.TempDir()

	originalBinPath := os.Getenv("LLEMME_BIN_PATH")
	os.Setenv("LLEMME_BIN_PATH", tmpDir)
	defer os.Setenv("LLEMME_BIN_PATH", originalBinPath)

	// Create a fake model file
	modelPath := filepath.Join(tmpDir, "model.gguf")
	if err := os.WriteFile(modelPath, []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}

	template := `test template`

	// Write twice with same model path (without modifying the file)
	path1, err := writeTemplateCache(modelPath, template)
	if err != nil {
		t.Fatalf("First writeTemplateCache() error = %v", err)
	}

	path2, err := writeTemplateCache(modelPath, template)
	if err != nil {
		t.Fatalf("Second writeTemplateCache() error = %v", err)
	}

	// Should produce same path (deterministic hash based on path + mtime)
	if path1 != path2 {
		t.Errorf("Cache paths differ for same model: %q vs %q", path1, path2)
	}
}

func TestWriteTemplateCacheDifferentModels(t *testing.T) {
	tmpDir := t.TempDir()

	originalBinPath := os.Getenv("LLEMME_BIN_PATH")
	os.Setenv("LLEMME_BIN_PATH", tmpDir)
	defer os.Setenv("LLEMME_BIN_PATH", originalBinPath)

	// Create two fake model files
	modelPath1 := filepath.Join(tmpDir, "model1.gguf")
	modelPath2 := filepath.Join(tmpDir, "model2.gguf")
	if err := os.WriteFile(modelPath1, []byte("fake1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(modelPath2, []byte("fake2"), 0644); err != nil {
		t.Fatal(err)
	}

	template := `test template`

	path1, err := writeTemplateCache(modelPath1, template)
	if err != nil {
		t.Fatalf("First writeTemplateCache() error = %v", err)
	}

	path2, err := writeTemplateCache(modelPath2, template)
	if err != nil {
		t.Fatalf("Second writeTemplateCache() error = %v", err)
	}

	// Should produce different paths
	if path1 == path2 {
		t.Errorf("Cache paths should differ for different models: both got %q", path1)
	}
}

func TestWriteTemplateCacheInvalidatesOnMtimeChange(t *testing.T) {
	tmpDir := t.TempDir()

	originalBinPath := os.Getenv("LLEMME_BIN_PATH")
	os.Setenv("LLEMME_BIN_PATH", tmpDir)
	defer os.Setenv("LLEMME_BIN_PATH", originalBinPath)

	modelPath := filepath.Join(tmpDir, "model.gguf")
	if err := os.WriteFile(modelPath, []byte("version1"), 0644); err != nil {
		t.Fatal(err)
	}

	template := `test template`

	path1, err := writeTemplateCache(modelPath, template)
	if err != nil {
		t.Fatalf("First writeTemplateCache() error = %v", err)
	}

	// Update the model file (changes mtime)
	if err := os.WriteFile(modelPath, []byte("version2"), 0644); err != nil {
		t.Fatal(err)
	}

	path2, err := writeTemplateCache(modelPath, template)
	if err != nil {
		t.Fatalf("Second writeTemplateCache() error = %v", err)
	}

	// Should produce different paths due to mtime change
	if path1 == path2 {
		t.Errorf("Cache paths should differ after model update: both got %q", path1)
	}
}

// createTestGGUF creates a minimal valid GGUF file with the given key-value pairs.
// Each kv pair is a string key mapped to a string value.
func createTestGGUF(t *testing.T, kvPairs map[string]string) string {
	t.Helper()

	var buf bytes.Buffer

	// Magic: "GGUF"
	buf.WriteString("GGUF")

	// Version: 3
	binary.Write(&buf, binary.LittleEndian, uint32(3))

	// Tensor count: 0
	binary.Write(&buf, binary.LittleEndian, uint64(0))

	// KV count
	binary.Write(&buf, binary.LittleEndian, uint64(len(kvPairs)))

	// Write key-value pairs
	for key, value := range kvPairs {
		// Key: length-prefixed string
		binary.Write(&buf, binary.LittleEndian, uint64(len(key)))
		buf.WriteString(key)

		// Value type: 8 = string
		binary.Write(&buf, binary.LittleEndian, uint32(8))

		// Value: length-prefixed string
		binary.Write(&buf, binary.LittleEndian, uint64(len(value)))
		buf.WriteString(value)
	}

	// Write to temp file
	tmpFile := filepath.Join(t.TempDir(), "test.gguf")
	if err := os.WriteFile(tmpFile, buf.Bytes(), 0644); err != nil {
		t.Fatalf("Failed to write test GGUF: %v", err)
	}

	return tmpFile
}

func TestExtractChatTemplate(t *testing.T) {
	template := `{% if tools is not none %}Use tools{% endif %}`
	ggufPath := createTestGGUF(t, map[string]string{
		"tokenizer.chat_template": template,
		"general.name":            "TestModel",
	})

	result, err := extractChatTemplate(ggufPath)
	if err != nil {
		t.Fatalf("extractChatTemplate() error = %v", err)
	}

	if result != template {
		t.Errorf("extractChatTemplate() = %q, want %q", result, template)
	}
}

func TestExtractChatTemplateNotFound(t *testing.T) {
	// GGUF without chat_template
	ggufPath := createTestGGUF(t, map[string]string{
		"general.name": "TestModel",
	})

	result, err := extractChatTemplate(ggufPath)
	if err != nil {
		t.Fatalf("extractChatTemplate() error = %v", err)
	}

	if result != "" {
		t.Errorf("extractChatTemplate() = %q, want empty string", result)
	}
}

func TestExtractChatTemplateInvalidFile(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "invalid.gguf")
	if err := os.WriteFile(tmpFile, []byte("not a gguf file"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := extractChatTemplate(tmpFile)
	if err == nil {
		t.Error("extractChatTemplate() expected error for invalid file, got nil")
	}
}

func TestExtractChatTemplateUnsupportedVersion(t *testing.T) {
	var buf bytes.Buffer

	// Valid magic
	buf.WriteString("GGUF")
	// Unsupported version (v99)
	binary.Write(&buf, binary.LittleEndian, uint32(99))
	// Rest doesn't matter - should fail on version check

	tmpFile := filepath.Join(t.TempDir(), "unsupported.gguf")
	if err := os.WriteFile(tmpFile, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := extractChatTemplate(tmpFile)
	if err == nil {
		t.Error("extractChatTemplate() expected error for unsupported version, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported GGUF version") {
		t.Errorf("expected 'unsupported GGUF version' error, got: %v", err)
	}
}

func TestExtractChatTemplateNonexistent(t *testing.T) {
	_, err := extractChatTemplate("/nonexistent/path/model.gguf")
	if err == nil {
		t.Error("extractChatTemplate() expected error for nonexistent file, got nil")
	}
}

func TestExtractAndPatchTemplate(t *testing.T) {
	tmpDir := t.TempDir()

	originalBinPath := os.Getenv("LLEMME_BIN_PATH")
	os.Setenv("LLEMME_BIN_PATH", tmpDir)
	defer os.Setenv("LLEMME_BIN_PATH", originalBinPath)

	// Template that needs patching
	template := `{% if tools is not none %}Use tools{% endif %}`
	ggufPath := createTestGGUF(t, map[string]string{
		"tokenizer.chat_template": template,
	})

	cachePath, err := ExtractAndPatchTemplate(ggufPath)
	if err != nil {
		t.Fatalf("ExtractAndPatchTemplate() error = %v", err)
	}

	if cachePath == "" {
		t.Fatal("ExtractAndPatchTemplate() returned empty path, expected patched template")
	}

	// Verify the cached file has the patched template
	content, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("Failed to read cache file: %v", err)
	}

	if !strings.Contains(string(content), "tools | length > 0") {
		t.Errorf("Cached template not patched: %s", string(content))
	}
}

func TestExtractAndPatchTemplateNoChangesNeeded(t *testing.T) {
	tmpDir := t.TempDir()

	originalBinPath := os.Getenv("LLEMME_BIN_PATH")
	os.Setenv("LLEMME_BIN_PATH", tmpDir)
	defer os.Setenv("LLEMME_BIN_PATH", originalBinPath)

	// Template that doesn't need patching
	template := `{% if messages %}{{ messages }}{% endif %}`
	ggufPath := createTestGGUF(t, map[string]string{
		"tokenizer.chat_template": template,
	})

	cachePath, err := ExtractAndPatchTemplate(ggufPath)
	if err != nil {
		t.Fatalf("ExtractAndPatchTemplate() error = %v", err)
	}

	// Should return empty path when no patches needed
	if cachePath != "" {
		t.Errorf("ExtractAndPatchTemplate() = %q, want empty string (no patches needed)", cachePath)
	}
}

func TestExtractAndPatchTemplateNoTemplate(t *testing.T) {
	tmpDir := t.TempDir()

	originalBinPath := os.Getenv("LLEMME_BIN_PATH")
	os.Setenv("LLEMME_BIN_PATH", tmpDir)
	defer os.Setenv("LLEMME_BIN_PATH", originalBinPath)

	// GGUF without chat_template
	ggufPath := createTestGGUF(t, map[string]string{
		"general.name": "TestModel",
	})

	cachePath, err := ExtractAndPatchTemplate(ggufPath)
	if err != nil {
		t.Fatalf("ExtractAndPatchTemplate() error = %v", err)
	}

	if cachePath != "" {
		t.Errorf("ExtractAndPatchTemplate() = %q, want empty string (no template)", cachePath)
	}
}

// createTestGGUFWithTypes creates a GGUF file with various value types to test skipGGUFValue.
// The chat_template is placed after other KV pairs to ensure skip logic is exercised.
func createTestGGUFWithTypes(t *testing.T, chatTemplate string) string {
	t.Helper()

	var buf bytes.Buffer

	// Magic: "GGUF"
	buf.WriteString("GGUF")

	// Version: 3
	binary.Write(&buf, binary.LittleEndian, uint32(3))

	// Tensor count: 0
	binary.Write(&buf, binary.LittleEndian, uint64(0))

	// KV count: 8 (various types + chat_template at the end)
	binary.Write(&buf, binary.LittleEndian, uint64(8))

	// u8 (type 0)
	writeKVKey(&buf, "test.u8")
	binary.Write(&buf, binary.LittleEndian, uint32(0))
	buf.WriteByte(42)

	// i32 (type 5)
	writeKVKey(&buf, "test.i32")
	binary.Write(&buf, binary.LittleEndian, uint32(5))
	binary.Write(&buf, binary.LittleEndian, int32(-123))

	// f32 (type 6)
	writeKVKey(&buf, "test.f32")
	binary.Write(&buf, binary.LittleEndian, uint32(6))
	binary.Write(&buf, binary.LittleEndian, float32(3.14))

	// bool (type 7)
	writeKVKey(&buf, "test.bool")
	binary.Write(&buf, binary.LittleEndian, uint32(7))
	buf.WriteByte(1)

	// u64 (type 10)
	writeKVKey(&buf, "test.u64")
	binary.Write(&buf, binary.LittleEndian, uint32(10))
	binary.Write(&buf, binary.LittleEndian, uint64(999))

	// f64 (type 12)
	writeKVKey(&buf, "test.f64")
	binary.Write(&buf, binary.LittleEndian, uint32(12))
	binary.Write(&buf, binary.LittleEndian, float64(2.718))

	// array of u8 (type 9)
	writeKVKey(&buf, "test.array")
	binary.Write(&buf, binary.LittleEndian, uint32(9)) // array type
	binary.Write(&buf, binary.LittleEndian, uint32(0)) // element type: u8
	binary.Write(&buf, binary.LittleEndian, uint64(3)) // array length
	buf.Write([]byte{1, 2, 3})                         // array data

	// chat_template (string, type 8) - at the end
	writeKVKey(&buf, "tokenizer.chat_template")
	binary.Write(&buf, binary.LittleEndian, uint32(8))
	binary.Write(&buf, binary.LittleEndian, uint64(len(chatTemplate)))
	buf.WriteString(chatTemplate)

	// Write to temp file
	tmpFile := filepath.Join(t.TempDir(), "test_types.gguf")
	if err := os.WriteFile(tmpFile, buf.Bytes(), 0644); err != nil {
		t.Fatalf("Failed to write test GGUF: %v", err)
	}

	return tmpFile
}

func writeKVKey(buf *bytes.Buffer, key string) {
	binary.Write(buf, binary.LittleEndian, uint64(len(key)))
	buf.WriteString(key)
}

func TestExtractChatTemplateSkipsOtherTypes(t *testing.T) {
	template := `{% if tools is not none %}Use tools{% endif %}`
	ggufPath := createTestGGUFWithTypes(t, template)

	result, err := extractChatTemplate(ggufPath)
	if err != nil {
		t.Fatalf("extractChatTemplate() error = %v", err)
	}

	if result != template {
		t.Errorf("extractChatTemplate() = %q, want %q", result, template)
	}
}
