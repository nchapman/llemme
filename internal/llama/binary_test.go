package llama

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
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
		if runtime.GOARCH == "amd64" {
			// On Linux x64, result depends on GPU detection
			if result != "ubuntu-x64" && result != "ubuntu-vulkan-x64" {
				t.Errorf("Expected platform ubuntu-x64 or ubuntu-vulkan-x64, got %s", result)
			}
		}
		if runtime.GOARCH == "arm64" && result != "" {
			t.Errorf("Expected empty platform for Linux ARM64 (unsupported), got %s", result)
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

	expectedPath := filepath.Join(tmpDir, ".lleme", "bin", "llama-current", "llama-cli")
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

	expectedPath := filepath.Join(tmpDir, ".lleme", "bin", "llama-current", "llama-server")
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

	binDir := filepath.Join(tmpDir, ".lleme", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("Failed to create bin dir: %v", err)
	}

	t.Run("returns false when binary does not exist", func(t *testing.T) {
		if IsInstalled() {
			t.Error("Expected IsInstalled to return false when binary doesn't exist")
		}
	})

	t.Run("returns true when llama-current symlink exists with binary", func(t *testing.T) {
		versionDir := filepath.Join(binDir, "llama-b7751")
		if err := os.MkdirAll(versionDir, 0755); err != nil {
			t.Fatalf("Failed to create version dir: %v", err)
		}

		cliBinary := filepath.Join(versionDir, "llama-cli")
		if err := os.WriteFile(cliBinary, []byte("#!/bin/sh\necho test"), 0755); err != nil {
			t.Fatalf("Failed to create binary: %v", err)
		}

		currentSymlink := filepath.Join(binDir, "llama-current")
		if err := os.Symlink("llama-b7751", currentSymlink); err != nil {
			t.Fatalf("Failed to create llama-current symlink: %v", err)
		}

		if !IsInstalled() {
			t.Error("Expected IsInstalled to return true when llama-current symlink exists")
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

		binDir := filepath.Join(tmpDir, ".lleme", "bin")
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

func TestLlamaCurrentSymlinkStructure(t *testing.T) {
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	versionDir := filepath.Join(binDir, "llama-b7751")

	if err := os.MkdirAll(versionDir, 0755); err != nil {
		t.Fatalf("Failed to create version dir: %v", err)
	}

	t.Run("creates llama-current symlink pointing to version directory", func(t *testing.T) {
		cliBinary := filepath.Join(versionDir, "llama-cli")
		serverBinary := filepath.Join(versionDir, "llama-server")

		if err := os.WriteFile(cliBinary, []byte("#!/bin/sh\necho cli"), 0755); err != nil {
			t.Fatalf("Failed to create CLI binary: %v", err)
		}
		if err := os.WriteFile(serverBinary, []byte("#!/bin/sh\necho server"), 0755); err != nil {
			t.Fatalf("Failed to create server binary: %v", err)
		}

		currentSymlink := filepath.Join(binDir, "llama-current")
		if err := os.Symlink("llama-b7751", currentSymlink); err != nil {
			t.Fatalf("Failed to create llama-current symlink: %v", err)
		}

		if _, err := os.Lstat(currentSymlink); err != nil {
			t.Errorf("Expected llama-current symlink to exist: %v", err)
		}

		// Verify symlink target
		target, err := os.Readlink(currentSymlink)
		if err != nil {
			t.Fatalf("Failed to read symlink: %v", err)
		}
		if target != "llama-b7751" {
			t.Errorf("Expected symlink target llama-b7751, got %s", target)
		}

		// Verify binaries are accessible through llama-current symlink
		if _, err := os.Stat(filepath.Join(binDir, "llama-current", "llama-cli")); err != nil {
			t.Errorf("Expected llama-cli to be accessible through llama-current symlink: %v", err)
		}
		if _, err := os.Stat(filepath.Join(binDir, "llama-current", "llama-server")); err != nil {
			t.Errorf("Expected llama-server to be accessible through llama-current symlink: %v", err)
		}
	})
}

func TestVersionFileJSON(t *testing.T) {
	version := &VersionInfo{
		TagName:     "b7751",
		BinaryPath:  "/path/to/llama-cli",
		InstalledAt: "2024-01-15T10:00:00Z",
	}
	file := VersionFile{Llama: version}

	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal version file: %v", err)
	}

	// Verify JSON structure has "llama" key
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Failed to unmarshal as map: %v", err)
	}
	if _, ok := raw["llama"]; !ok {
		t.Error("Expected 'llama' key in JSON output")
	}

	var decoded VersionFile
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal version file: %v", err)
	}

	if decoded.Llama == nil {
		t.Fatal("Expected Llama to be non-nil")
	}
	if decoded.Llama.TagName != version.TagName {
		t.Errorf("Expected TagName %s, got %s", version.TagName, decoded.Llama.TagName)
	}
	if decoded.Llama.BinaryPath != version.BinaryPath {
		t.Errorf("Expected BinaryPath %s, got %s", version.BinaryPath, decoded.Llama.BinaryPath)
	}
	if decoded.Llama.InstalledAt != version.InstalledAt {
		t.Errorf("Expected InstalledAt %s, got %s", version.InstalledAt, decoded.Llama.InstalledAt)
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

func TestHasVulkanSupport(t *testing.T) {
	// HasVulkanSupport only runs on Linux
	if runtime.GOOS != "linux" {
		t.Run("returns false on non-Linux", func(t *testing.T) {
			if HasVulkanSupport() {
				t.Errorf("HasVulkanSupport() on %s = true, want false", runtime.GOOS)
			}
		})
		return
	}

	// On Linux, just verify it returns a boolean without error
	t.Run("returns boolean on Linux", func(t *testing.T) {
		// Just call it to ensure no panic
		_ = HasVulkanSupport()
	})
}

func TestGetPlatformLinuxVariants(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("Skipping Linux-specific test on non-Linux/amd64 platform")
	}

	result := getPlatform()

	// On Linux x64, should be either ubuntu-x64 (no Vulkan) or ubuntu-vulkan-x64 (Vulkan available)
	validPlatforms := []string{"ubuntu-x64", "ubuntu-vulkan-x64"}
	if !slices.Contains(validPlatforms, result) {
		t.Errorf("getPlatform() on Linux x64 = %q, want one of %v", result, validPlatforms)
	}

	// Verify consistency with Vulkan support detection
	// Platform selection is based on libvulkan.so availability, not GPU detection
	hasVulkan := HasVulkanSupport()
	if hasVulkan && result != "ubuntu-vulkan-x64" {
		t.Errorf("Vulkan support detected but platform is %q, expected ubuntu-vulkan-x64", result)
	}
	if !hasVulkan && result != "ubuntu-x64" {
		t.Errorf("No Vulkan support but platform is %q, expected ubuntu-x64", result)
	}
}
