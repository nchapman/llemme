package llama

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGetPlatform(t *testing.T) {
	result := getPlatform()

	if result == "" {
		t.Skipf("Skipping test: unsupported platform %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" && result != "macos-arm64" {
			t.Errorf("Expected platform macos-arm64, got %s", result)
		}
		if runtime.GOARCH == "amd64" && result != "macos-x64" {
			t.Errorf("Expected platform macos-x64, got %s", result)
		}
	case "linux":
		if result != "ubuntu-x64" {
			t.Errorf("Expected platform ubuntu-x64, got %s", result)
		}
	}
}

func TestGetBinaryPattern(t *testing.T) {
	tests := []struct {
		name     string
		tagName  string
		expected string
	}{
		{
			name:     "b7751 release",
			tagName:  "b7751",
			expected: "llama-b7751-bin-" + getPlatform() + ".tar.gz",
		},
		{
			name:     "b7752 release",
			tagName:  "b7752",
			expected: "llama-b7752-bin-" + getPlatform() + ".tar.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			release := &Release{TagName: tt.tagName}
			pattern := getBinaryPattern(release)

			if getPlatform() == "" {
				t.Skip("Skipping test: unsupported platform")
			}

			if pattern != tt.expected {
				t.Errorf("Expected pattern %s, got %s", tt.expected, pattern)
			}
		})
	}
}

func TestBinaryPath(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)

	os.Setenv("HOME", tmpDir)

	expectedPath := filepath.Join(tmpDir, ".gollama", "bin", "llama-cli")
	actualPath := BinaryPath()

	if actualPath != expectedPath {
		t.Errorf("Expected BinaryPath %s, got %s", expectedPath, actualPath)
	}
}

func TestServerPath(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)

	os.Setenv("HOME", tmpDir)

	expectedPath := filepath.Join(tmpDir, ".gollama", "bin", "llama-server")
	actualPath := ServerPath()

	if actualPath != expectedPath {
		t.Errorf("Expected ServerPath %s, got %s", expectedPath, actualPath)
	}
}

func TestIsInstalled(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)

	os.Setenv("HOME", tmpDir)

	binDir := filepath.Join(tmpDir, ".gollama", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("Failed to create bin dir: %v", err)
	}

	t.Run("returns false when binary does not exist", func(t *testing.T) {
		if IsInstalled() {
			t.Error("Expected IsInstalled to return false when binary doesn't exist")
		}
	})

	t.Run("returns true when binary symlink exists", func(t *testing.T) {
		versionDir := filepath.Join(binDir, "llama-b7751")
		if err := os.MkdirAll(versionDir, 0755); err != nil {
			t.Fatalf("Failed to create version dir: %v", err)
		}

		cliBinary := filepath.Join(versionDir, "llama-cli-b7751")
		if err := os.WriteFile(cliBinary, []byte("#!/bin/sh\necho test"), 0755); err != nil {
			t.Fatalf("Failed to create binary: %v", err)
		}

		cliSymlink := filepath.Join(binDir, "llama-cli")
		if err := os.Symlink(cliBinary, cliSymlink); err != nil {
			t.Fatalf("Failed to create symlink: %v", err)
		}

		if !IsInstalled() {
			t.Error("Expected IsInstalled to return true when symlink exists")
		}
	})
}

func TestVersionInfo(t *testing.T) {
	t.Run("save and load version info", func(t *testing.T) {
		tmpDir := t.TempDir()
		oldHome := os.Getenv("HOME")
		defer os.Setenv("HOME", oldHome)

		os.Setenv("HOME", tmpDir)

		version := &VersionInfo{
			TagName:     "b7751",
			BinaryPath:  "/test/path/llama-cli",
			InstalledAt: "2024-01-15T10:00:00Z",
		}

		err := SaveVersionInfo(version)
		if err != nil {
			t.Fatalf("Failed to save version info: %v", err)
		}

		loaded, err := GetInstalledVersion()
		if err != nil {
			t.Fatalf("Failed to load version info: %v", err)
		}

		if loaded == nil {
			t.Fatal("Expected loaded version to be non-nil")
		}

		if loaded.TagName != version.TagName {
			t.Errorf("Expected TagName %s, got %s", version.TagName, loaded.TagName)
		}
		if loaded.BinaryPath != version.BinaryPath {
			t.Errorf("Expected BinaryPath %s, got %s", version.BinaryPath, loaded.BinaryPath)
		}
		if loaded.InstalledAt != version.InstalledAt {
			t.Errorf("Expected InstalledAt %s, got %s", version.InstalledAt, loaded.InstalledAt)
		}
	})

	t.Run("returns nil when version file does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		oldHome := os.Getenv("HOME")
		defer os.Setenv("HOME", oldHome)

		os.Setenv("HOME", tmpDir)

		version, err := GetInstalledVersion()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if version != nil {
			t.Error("Expected nil version when file doesn't exist")
		}
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		oldHome := os.Getenv("HOME")
		defer os.Setenv("HOME", oldHome)

		os.Setenv("HOME", tmpDir)

		binDir := filepath.Join(tmpDir, ".gollama", "bin")
		if err := os.MkdirAll(binDir, 0755); err != nil {
			t.Fatalf("Failed to create bin dir: %v", err)
		}

		versionPath := filepath.Join(binDir, "version.json")
		if err := os.WriteFile(versionPath, []byte("invalid json"), 0644); err != nil {
			t.Fatalf("Failed to write invalid version file: %v", err)
		}

		_, err := GetInstalledVersion()
		if err == nil {
			t.Error("Expected error for invalid JSON, got nil")
		}
	})
}

func TestFindAssetForPlatform(t *testing.T) {
	if getPlatform() == "" {
		t.Skip("Skipping test: unsupported platform")
	}

	t.Run("finds matching asset", func(t *testing.T) {
		platform := getPlatform()
		expectedPattern := "llama-b7751-bin-" + platform + ".tar.gz"

		release := &Release{
			TagName: "b7751",
			Assets: []Asset{
				{Name: "llama-b7751-bin-" + platform + ".tar.gz", BrowserDownloadUrl: "http://example.com/" + expectedPattern},
				{Name: "llama-b7751-bin-linux-x64.tar.gz", BrowserDownloadUrl: "http://example.com/linux"},
			},
		}

		url, name, err := FindAssetForPlatform(release)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if name != expectedPattern {
			t.Errorf("Expected asset name %s, got %s", expectedPattern, name)
		}

		if !contains(url, expectedPattern) {
			t.Errorf("Expected URL to contain %s, got %s", expectedPattern, url)
		}
	})

	t.Run("returns error for unsupported platform", func(t *testing.T) {
		release := &Release{
			TagName: "b7751",
			Assets: []Asset{
				{Name: "llama-b7751-bin-windows-x64.zip"},
			},
		}

		_, _, err := FindAssetForPlatform(release)
		if err == nil {
			t.Error("Expected error for unsupported platform, got nil")
		}
	})

	t.Run("returns error when asset not found", func(t *testing.T) {
		release := &Release{
			TagName: "b7751",
			Assets: []Asset{
				{Name: "source.tar.gz"},
			},
		}

		_, _, err := FindAssetForPlatform(release)
		if err == nil {
			t.Error("Expected error when asset not found, got nil")
		}
	})
}

func TestExtractTarGz(t *testing.T) {
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	versionDir := filepath.Join(binDir, "llama-b7751")

	if err := os.MkdirAll(versionDir, 0755); err != nil {
		t.Fatalf("Failed to create version dir: %v", err)
	}

	t.Run("creates symlinks for binaries", func(t *testing.T) {
		cliBinary := filepath.Join(versionDir, "llama-cli-b7751")
		serverBinary := filepath.Join(versionDir, "llama-server-b7751")

		if err := os.WriteFile(cliBinary, []byte("#!/bin/sh\necho cli"), 0755); err != nil {
			t.Fatalf("Failed to create CLI binary: %v", err)
		}
		if err := os.WriteFile(serverBinary, []byte("#!/bin/sh\necho server"), 0755); err != nil {
			t.Fatalf("Failed to create server binary: %v", err)
		}

		cliSymlink := filepath.Join(binDir, "llama-cli")
		serverSymlink := filepath.Join(binDir, "llama-server")

		if err := os.Symlink(cliBinary, cliSymlink); err != nil {
			t.Fatalf("Failed to create CLI symlink: %v", err)
		}
		if err := os.Symlink(serverBinary, serverSymlink); err != nil {
			t.Fatalf("Failed to create server symlink: %v", err)
		}

		if _, err := os.Lstat(cliSymlink); err != nil {
			t.Errorf("Expected CLI symlink to exist: %v", err)
		}
		if _, err := os.Lstat(serverSymlink); err != nil {
			t.Errorf("Expected server symlink to exist: %v", err)
		}
	})
}

func TestVersionInfoJSON(t *testing.T) {
	version := &VersionInfo{
		TagName:     "b7751",
		BinaryPath:  "/path/to/llama-cli",
		InstalledAt: "2024-01-15T10:00:00Z",
	}

	data, err := json.MarshalIndent(version, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal version: %v", err)
	}

	var decoded VersionInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal version: %v", err)
	}

	if decoded.TagName != version.TagName {
		t.Errorf("Expected TagName %s, got %s", version.TagName, decoded.TagName)
	}
	if decoded.BinaryPath != version.BinaryPath {
		t.Errorf("Expected BinaryPath %s, got %s", version.BinaryPath, decoded.BinaryPath)
	}
	if decoded.InstalledAt != version.InstalledAt {
		t.Errorf("Expected InstalledAt %s, got %s", version.InstalledAt, decoded.InstalledAt)
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
