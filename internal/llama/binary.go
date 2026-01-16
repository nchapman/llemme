package llama

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/nchapman/llemme/internal/config"
)

const (
	llamaRepo    = "ggml-org/llama.cpp"
	apiBase      = "https://api.github.com/repos/" + llamaRepo
	downloadBase = "https://github.com/" + llamaRepo + "/releases/download"
)

type Release struct {
	TagName string  `json:"tag_name"`
	Name    string  `json:"name"`
	Assets  []Asset `json:"assets"`
}

type Asset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadUrl string `json:"browser_download_url"`
}

type VersionInfo struct {
	TagName     string `json:"tag_name"`
	BinaryPath  string `json:"binary_path"`
	InstalledAt string `json:"installed_at"`
}

func getPlatform() string {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	switch osName {
	case "darwin":
		if arch == "arm64" {
			return "macos-arm64"
		}
		return "macos-x64"
	case "linux":
		return "ubuntu-x64"
	default:
		return ""
	}
}

func getBinaryPattern(release *Release) string {
	platform := getPlatform()
	if platform == "" {
		return ""
	}

	return "llama-" + release.TagName + "-bin-" + platform + ".tar.gz"
}

func GetLatestVersion() (*Release, error) {
	url := apiBase + "/releases/latest"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "llemme/0.1.0")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

func FindAssetForPlatform(release *Release) (string, string, error) {
	binaryPattern := getBinaryPattern(release)
	if binaryPattern == "" {
		return "", "", fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	for _, asset := range release.Assets {
		if asset.Name == binaryPattern {
			return asset.BrowserDownloadUrl, asset.Name, nil
		}
	}

	return "", "", fmt.Errorf("could not find binary for platform %s", binaryPattern)
}

func DownloadBinary(downloadURL, destPath string, progress func(int64, int64)) error {
	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "llemme/0.1.0")

	// Use transport timeouts for connection setup, but no overall timeout for large downloads
	transport := &http.Transport{
		ResponseHeaderTimeout: 30 * time.Second,
	}
	client := &http.Client{Transport: transport}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	tmpPath := destPath + ".partial"
	out, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer out.Close()

	size := resp.ContentLength
	written := int64(0)
	buf := make([]byte, 32*1024)

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				return werr
			}
			written += int64(n)
			if progress != nil {
				progress(written, size)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	out.Close()

	if err := os.Rename(tmpPath, destPath); err != nil {
		return err
	}

	return nil
}

func extractTarGz(archivePath, destDir string) error {
	cmd := exec.Command("tar", "-xzf", archivePath, "-C", destDir)
	if err := cmd.Run(); err != nil {
		return err
	}

	entries, err := os.ReadDir(destDir)
	if err != nil {
		return err
	}

	var llamaDir string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "llama-") {
			llamaDir = filepath.Join(destDir, entry.Name())
			break
		}
	}

	if llamaDir == "" {
		return fmt.Errorf("could not find llama directory in archive")
	}

	fileEntries, err := os.ReadDir(llamaDir)
	if err != nil {
		return err
	}

	for _, entry := range fileEntries {
		if entry.IsDir() {
			continue
		}

		srcPath := filepath.Join(llamaDir, entry.Name())

		destName := entry.Name()
		if strings.Contains(entry.Name(), "llama-cli") {
			destName = "llama-cli"
		} else if strings.Contains(entry.Name(), "llama-server") {
			destName = "llama-server"
		} else if !strings.HasSuffix(entry.Name(), ".dylib") {
			continue
		}

		destPath := filepath.Join(destDir, destName)
		symlinkPath := destPath

		if _, err := os.Lstat(symlinkPath); err == nil {
			continue
		}

		if err := os.Symlink(srcPath, symlinkPath); err != nil {
			continue
		}
	}

	return nil
}

func InstallLatest() (*VersionInfo, error) {
	release, err := GetLatestVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get latest release: %w", err)
	}

	downloadURL, binaryName, err := FindAssetForPlatform(release)
	if err != nil {
		return nil, err
	}

	binDir := config.BinPath()
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create bin directory: %w", err)
	}

	archivePath := filepath.Join(binDir, binaryName)

	message := fmt.Sprintf("Downloading llama.cpp %s", release.TagName)
	fmt.Printf("%s\n", message)

	if err := DownloadBinary(downloadURL, archivePath, nil); err != nil {
		return nil, fmt.Errorf("failed to download binary: %w", err)
	}

	fmt.Println("Extracting...")

	if err := extractTarGz(archivePath, binDir); err != nil {
		return nil, fmt.Errorf("failed to extract archive: %w", err)
	}

	cliPath := filepath.Join(binDir, "llama-cli")
	versionInfo := &VersionInfo{
		TagName:     release.TagName,
		BinaryPath:  cliPath,
		InstalledAt: time.Now().Format(time.RFC3339),
	}

	if err := SaveVersionInfo(versionInfo); err != nil {
		return nil, fmt.Errorf("failed to save version info: %w", err)
	}

	return versionInfo, nil
}

func GetInstalledVersion() (*VersionInfo, error) {
	versionPath := filepath.Join(config.BinPath(), "version.json")

	data, err := os.ReadFile(versionPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var version VersionInfo
	if err := json.Unmarshal(data, &version); err != nil {
		return nil, err
	}

	return &version, nil
}

func SaveVersionInfo(version *VersionInfo) error {
	versionPath := filepath.Join(config.BinPath(), "version.json")

	if err := os.MkdirAll(filepath.Dir(versionPath), 0755); err != nil {
		return fmt.Errorf("failed to create version directory: %w", err)
	}

	data, err := json.MarshalIndent(version, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal version: %w", err)
	}

	return os.WriteFile(versionPath, data, 0644)
}

func BinaryPath() string {
	return filepath.Join(config.BinPath(), "llama-cli")
}

func ServerPath() string {
	return filepath.Join(config.BinPath(), "llama-server")
}

func IsInstalled() bool {
	cliPath := filepath.Join(config.BinPath(), "llama-cli")
	if _, err := os.Lstat(cliPath); err != nil {
		return false
	}
	return true
}
