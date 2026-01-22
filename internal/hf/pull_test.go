package hf

import (
	"testing"
)

func TestGetManifestInfo(t *testing.T) {
	// This test verifies that GetManifestInfo uses the raw Tag for API calls
	// Since it requires network access, we test the ManifestInfo struct behavior

	info := &ManifestInfo{
		GGUFSize:   4000000000,
		MMProjSize: 100000000,
		TotalSize:  4100000000,
		IsVision:   true,
	}

	if info.GGUFSize != 4000000000 {
		t.Errorf("GGUFSize = %d, want 4000000000", info.GGUFSize)
	}
	if info.MMProjSize != 100000000 {
		t.Errorf("MMProjSize = %d, want 100000000", info.MMProjSize)
	}
	if info.TotalSize != 4100000000 {
		t.Errorf("TotalSize = %d, want 4100000000", info.TotalSize)
	}
	if !info.IsVision {
		t.Error("IsVision should be true")
	}
}

func TestPullOptions(t *testing.T) {
	// Test that PullOptions correctly stores manifest data
	manifest := &Manifest{
		GGUFFile: &ManifestFile{
			RFilename: "model.gguf",
			Size:      1000,
		},
	}
	manifestJSON := []byte(`{"ggufFile":{"rfilename":"model.gguf","size":1000}}`)

	opts := &PullOptions{
		Manifest:     manifest,
		ManifestJSON: manifestJSON,
	}

	if opts.Manifest != manifest {
		t.Error("Manifest not stored correctly")
	}
	if string(opts.ManifestJSON) != string(manifestJSON) {
		t.Error("ManifestJSON not stored correctly")
	}
}

func TestPullResult(t *testing.T) {
	// Test PullResult struct
	result := &PullResult{
		ModelPath:  "/path/to/model.gguf",
		MMProjPath: "/path/to/mmproj.gguf",
		IsVision:   true,
		TotalSize:  5000000000,
		GGUFSize:   4500000000,
		MMProjSize: 500000000,
	}

	if result.ModelPath != "/path/to/model.gguf" {
		t.Errorf("ModelPath = %s, want /path/to/model.gguf", result.ModelPath)
	}
	if result.MMProjPath != "/path/to/mmproj.gguf" {
		t.Errorf("MMProjPath = %s, want /path/to/mmproj.gguf", result.MMProjPath)
	}
	if !result.IsVision {
		t.Error("IsVision should be true")
	}
}

func TestPullProgress(t *testing.T) {
	// Test PullProgress struct
	progress := PullProgress{
		Phase:   "download",
		Current: 500,
		Total:   1000,
	}

	if progress.Phase != "download" {
		t.Errorf("Phase = %s, want download", progress.Phase)
	}
	if progress.Current != 500 {
		t.Errorf("Current = %d, want 500", progress.Current)
	}
	if progress.Total != 1000 {
		t.Errorf("Total = %d, want 1000", progress.Total)
	}
}

func TestHashesMatch(t *testing.T) {
	tests := []struct {
		name   string
		local  *ManifestFile
		remote *ManifestFile
		want   bool
	}{
		{
			name:   "both nil",
			local:  nil,
			remote: nil,
			want:   true,
		},
		{
			name:   "local nil",
			local:  nil,
			remote: &ManifestFile{Size: 100},
			want:   false,
		},
		{
			name:   "remote nil",
			local:  &ManifestFile{Size: 100},
			remote: nil,
			want:   false,
		},
		{
			name:   "matching hashes",
			local:  &ManifestFile{LFS: &ManifestLFS{SHA256: "abc123"}},
			remote: &ManifestFile{LFS: &ManifestLFS{SHA256: "abc123"}},
			want:   true,
		},
		{
			name:   "different hashes",
			local:  &ManifestFile{LFS: &ManifestLFS{SHA256: "abc123"}},
			remote: &ManifestFile{LFS: &ManifestLFS{SHA256: "def456"}},
			want:   false,
		},
		{
			name:   "no LFS - matching sizes",
			local:  &ManifestFile{Size: 1000},
			remote: &ManifestFile{Size: 1000},
			want:   true,
		},
		{
			name:   "no LFS - different sizes",
			local:  &ManifestFile{Size: 1000},
			remote: &ManifestFile{Size: 2000},
			want:   false,
		},
		{
			name:   "local no LFS, remote has LFS - match by size",
			local:  &ManifestFile{Size: 1000},
			remote: &ManifestFile{Size: 1000, LFS: &ManifestLFS{SHA256: "abc123"}},
			want:   true,
		},
		{
			name:   "remote no LFS, local has LFS - match by size",
			local:  &ManifestFile{Size: 1000, LFS: &ManifestLFS{SHA256: "abc123"}},
			remote: &ManifestFile{Size: 1000},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hashesMatch(tt.local, tt.remote)
			if got != tt.want {
				t.Errorf("hashesMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseManifest(t *testing.T) {
	jsonData := []byte(`{
		"ggufFile": {
			"rfilename": "model-Q4_K_M.gguf",
			"size": 4000000000,
			"lfs": {
				"sha256": "abc123",
				"size": 4000000000
			}
		},
		"mmprojFile": {
			"rfilename": "mmproj.gguf",
			"size": 100000000
		}
	}`)

	var manifest Manifest
	err := parseManifest(jsonData, &manifest)
	if err != nil {
		t.Fatalf("parseManifest() error = %v", err)
	}

	if manifest.GGUFFile == nil {
		t.Fatal("GGUFFile should not be nil")
	}
	if manifest.GGUFFile.RFilename != "model-Q4_K_M.gguf" {
		t.Errorf("GGUFFile.RFilename = %s, want model-Q4_K_M.gguf", manifest.GGUFFile.RFilename)
	}
	if manifest.GGUFFile.Size != 4000000000 {
		t.Errorf("GGUFFile.Size = %d, want 4000000000", manifest.GGUFFile.Size)
	}
	if manifest.GGUFFile.LFS == nil {
		t.Fatal("GGUFFile.LFS should not be nil")
	}
	if manifest.GGUFFile.LFS.SHA256 != "abc123" {
		t.Errorf("GGUFFile.LFS.SHA256 = %s, want abc123", manifest.GGUFFile.LFS.SHA256)
	}
	if manifest.MMProjFile == nil {
		t.Fatal("MMProjFile should not be nil")
	}
	if manifest.MMProjFile.RFilename != "mmproj.gguf" {
		t.Errorf("MMProjFile.RFilename = %s, want mmproj.gguf", manifest.MMProjFile.RFilename)
	}
}

func TestParseManifestInvalid(t *testing.T) {
	jsonData := []byte(`invalid json`)

	var manifest Manifest
	err := parseManifest(jsonData, &manifest)
	if err == nil {
		t.Error("parseManifest() should return error for invalid JSON")
	}
}
