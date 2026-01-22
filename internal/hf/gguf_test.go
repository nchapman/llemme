package hf

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestReadGGUFHeader(t *testing.T) {
	// Test basic header parsing with split.count key
	buf := &bytes.Buffer{}

	// Write magic
	buf.WriteString("GGUF")

	// Write version (3)
	binary.Write(buf, binary.LittleEndian, uint32(3))

	// Write tensor count (0)
	binary.Write(buf, binary.LittleEndian, int64(0))

	// Write KV count (1)
	binary.Write(buf, binary.LittleEndian, int64(1))

	// Write key "split.count"
	key := "split.count"
	binary.Write(buf, binary.LittleEndian, uint64(len(key)))
	buf.WriteString(key)

	// Write value type (UINT16 = 2)
	binary.Write(buf, binary.LittleEndian, int32(2))

	// Write value (3 splits)
	binary.Write(buf, binary.LittleEndian, uint16(3))

	header, err := readGGUFHeader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("readGGUFHeader() error = %v", err)
	}

	if header.Version != 3 {
		t.Errorf("Version = %d, want 3", header.Version)
	}
	if header.SplitCount != 3 {
		t.Errorf("SplitCount = %d, want 3", header.SplitCount)
	}
}

func TestReadGGUFHeaderNoSplit(t *testing.T) {
	// Test header without split.count (normal file)
	buf := &bytes.Buffer{}

	// Write magic
	buf.WriteString("GGUF")

	// Write version (3)
	binary.Write(buf, binary.LittleEndian, uint32(3))

	// Write tensor count (0)
	binary.Write(buf, binary.LittleEndian, int64(0))

	// Write KV count (1)
	binary.Write(buf, binary.LittleEndian, int64(1))

	// Write a different key
	key := "general.name"
	binary.Write(buf, binary.LittleEndian, uint64(len(key)))
	buf.WriteString(key)

	// Write value type (STRING = 8)
	binary.Write(buf, binary.LittleEndian, int32(8))

	// Write string value
	val := "test model"
	binary.Write(buf, binary.LittleEndian, uint64(len(val)))
	buf.WriteString(val)

	header, err := readGGUFHeader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("readGGUFHeader() error = %v", err)
	}

	if header.SplitCount != 0 {
		t.Errorf("SplitCount = %d, want 0 for non-split file", header.SplitCount)
	}
}

func TestReadGGUFHeaderInvalidMagic(t *testing.T) {
	buf := &bytes.Buffer{}
	buf.WriteString("NOTG")

	_, err := readGGUFHeader(bytes.NewReader(buf.Bytes()))
	if err == nil {
		t.Error("readGGUFHeader() should fail with invalid magic")
	}
}

func TestSplitFilePattern(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{
			name:     "split file 1 of 2",
			filename: "model-00001-of-00002.gguf",
			want:     true,
		},
		{
			name:     "split file 2 of 2",
			filename: "model-00002-of-00002.gguf",
			want:     true,
		},
		{
			name:     "split file 1 of 10",
			filename: "model-00001-of-00010.gguf",
			want:     true,
		},
		{
			name:     "split file 10 of 10",
			filename: "model-00010-of-00010.gguf",
			want:     true,
		},
		{
			name:     "complex name",
			filename: "gpt-oss-120b-Q4_K_M-00001-of-00003.gguf",
			want:     true,
		},
		{
			name:     "not a split file",
			filename: "model.gguf",
			want:     false,
		},
		{
			name:     "partial match - wrong format",
			filename: "model-001-of-002.gguf",
			want:     false,
		},
		{
			name:     "partial match - missing gguf",
			filename: "model-00001-of-00002",
			want:     false,
		},
		{
			name:     "regular quantized file",
			filename: "model-Q4_K_M.gguf",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitFilePattern.MatchString(tt.filename)
			if got != tt.want {
				t.Errorf("SplitFilePattern.MatchString(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		prefix     string
		splitNo    int
		splitCount int
		want       string
	}{
		{
			prefix:     "model",
			splitNo:    0,
			splitCount: 2,
			want:       "model-00001-of-00002.gguf",
		},
		{
			prefix:     "model",
			splitNo:    1,
			splitCount: 2,
			want:       "model-00002-of-00002.gguf",
		},
		{
			prefix:     "Q4_K_M/gpt-120b-Q4_K_M",
			splitNo:    0,
			splitCount: 3,
			want:       "Q4_K_M/gpt-120b-Q4_K_M-00001-of-00003.gguf",
		},
		{
			prefix:     "Q4_K_M/gpt-120b-Q4_K_M",
			splitNo:    2,
			splitCount: 3,
			want:       "Q4_K_M/gpt-120b-Q4_K_M-00003-of-00003.gguf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := SplitPath(tt.prefix, tt.splitNo, tt.splitCount)
			if got != tt.want {
				t.Errorf("SplitPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSplitPrefix(t *testing.T) {
	tests := []struct {
		path       string
		splitNo    int
		splitCount int
		want       string
	}{
		{
			path:       "model-00001-of-00002.gguf",
			splitNo:    0,
			splitCount: 2,
			want:       "model",
		},
		{
			path:       "model-00002-of-00002.gguf",
			splitNo:    1,
			splitCount: 2,
			want:       "model",
		},
		{
			path:       "Q4_K_M/gpt-120b-Q4_K_M-00001-of-00003.gguf",
			splitNo:    0,
			splitCount: 3,
			want:       "Q4_K_M/gpt-120b-Q4_K_M",
		},
		{
			path:       "model.gguf", // Not a split file
			splitNo:    0,
			splitCount: 1,
			want:       "",
		},
		{
			path:       "model-00001-of-00003.gguf", // Wrong split count
			splitNo:    0,
			splitCount: 2,
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := SplitPrefix(tt.path, tt.splitNo, tt.splitCount)
			if got != tt.want {
				t.Errorf("SplitPrefix() = %q, want %q", got, tt.want)
			}
		})
	}
}
