package hf

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewDownloader(t *testing.T) {
	client := &Client{}
	downloader := NewDownloader(client)

	if downloader == nil {
		t.Fatal("NewDownloader() returned nil")
	}

	if downloader.client != client {
		t.Error("downloader.client is not the provided client")
	}
}

func TestNewDownloaderWithProgress(t *testing.T) {
	client := &Client{}
	progressCalled := false

	downloader := NewDownloaderWithProgress(client, func(downloaded, total int64, speed float64, eta time.Duration) {
		progressCalled = true
	})

	if downloader == nil {
		t.Fatal("NewDownloaderWithProgress() returned nil")
	}

	if downloader.client != client {
		t.Error("downloader.client is not the provided client")
	}

	if downloader.progress == nil {
		t.Error("downloader.progress callback is nil")
	}

	downloader.progress(100, 1000, 1000000, 10*time.Second)

	if !progressCalled {
		t.Error("progress callback was not called")
	}
}

func TestCalculateProgress(t *testing.T) {
	now := time.Now()
	downloader := &Downloader{
		lastUpdate: now.Add(-2 * time.Second),
		lastBytes:  0,
	}

	progress := downloader.calculateProgress(2048, 4096)

	if progress.Downloaded != 2048 {
		t.Errorf("progress.Downloaded = %v, want 2048", progress.Downloaded)
	}

	if progress.Total != 4096 {
		t.Errorf("progress.Total = %v, want 4096", progress.Total)
	}

	if progress.Speed < 900 || progress.Speed > 1100 {
		t.Errorf("progress.Speed = %v, want approximately 1024", progress.Speed)
	}
}

func TestGetModelPath(t *testing.T) {
	user := "testuser"
	repo := "testrepo"

	path := GetModelPath(user, repo)

	if path == "" {
		t.Error("GetModelPath() returned empty string")
	}

	if filepath.Base(path) != repo {
		t.Errorf("GetModelPath() basename = %v, want %v", filepath.Base(path), repo)
	}
}

func TestGetModelFilePath(t *testing.T) {
	user := "testuser"
	repo := "testrepo"
	quant := "Q4_K_M"

	path := GetModelFilePath(user, repo, quant)

	if path == "" {
		t.Error("GetModelFilePath() returned empty string")
	}

	if !filepath.IsAbs(path) {
		t.Error("GetModelFilePath() should return absolute path")
	}

	if filepath.Base(path) != quant+".gguf" {
		t.Errorf("GetModelFilePath() filename = %v, want %v", filepath.Base(path), quant+".gguf")
	}
}

func TestCalculateSHA256(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "Hello, World!"

	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatal(err)
	}

	hash, err := CalculateSHA256(testFile)
	if err != nil {
		t.Fatalf("CalculateSHA256() error = %v", err)
	}

	if hash == "" {
		t.Error("CalculateSHA256() returned empty hash")
	}

	expectedHash := "dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f"
	if hash != expectedHash {
		t.Errorf("CalculateSHA256() = %v, want %v", hash, expectedHash)
	}
}

func TestVerifySHA256(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "Hello, World!"

	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name         string
		expectedHash string
		want         bool
		wantErr      bool
	}{
		{
			name:         "correct hash",
			expectedHash: "dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f",
			want:         true,
		},
		{
			name:         "incorrect hash",
			expectedHash: "0000000000000000000000000000000000000000000000000000000000000000",
			want:         false,
		},
		{
			name:         "case insensitive",
			expectedHash: "DFFD6021BB2BD5B0AF676290809EC3A53191DD81C7F70A4B28688A362182986F",
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := VerifySHA256(testFile, tt.expectedHash)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifySHA256() error = %v, wantErr %v", err, tt.wantErr)
			}
			if result != tt.want {
				t.Errorf("VerifySHA256() = %v, want %v", result, tt.want)
			}
		})
	}
}
