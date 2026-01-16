package hf

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nchapman/gollama/internal/config"
)

type ProgressCallback func(downloaded, total int64, speed float64, eta time.Duration)

type DownloadProgress struct {
	Downloaded int64
	Total      int64
	Speed      float64
	ETA        time.Duration
}

type Downloader struct {
	client     *Client
	progress   ProgressCallback
	startTime  time.Time
	lastUpdate time.Time
	lastBytes  int64
}

func NewDownloader(client *Client) *Downloader {
	return &Downloader{
		client: client,
	}
}

func NewDownloaderWithProgress(client *Client, progress ProgressCallback) *Downloader {
	return &Downloader{
		client:   client,
		progress: progress,
	}
}

func (d *Downloader) DownloadModel(user, repo, branch, filename string, destPath string) (*DownloadProgress, error) {
	url := fmt.Sprintf("%s/%s/%s/resolve/%s/%s", baseURL, user, repo, branch, filename)

	partialPath := destPath + ".partial"
	fileSize := int64(0)

	if info, err := os.Stat(partialPath); err == nil {
		fileSize = info.Size()
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if d.client.token != "" {
		req.Header.Set("Authorization", "Bearer "+d.client.token)
	}

	if fileSize > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", fileSize))
	}

	resp, err := d.client.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	totalSize := fileSize + resp.ContentLength

	flags := os.O_CREATE | os.O_WRONLY
	if resp.StatusCode == http.StatusOK {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_APPEND
	}

	file, err := os.OpenFile(partialPath, flags, 0644)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	d.startTime = time.Now()
	d.lastUpdate = d.startTime
	d.lastBytes = fileSize

	buf := make([]byte, 32*1024)
	written := fileSize

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := file.Write(buf[:n]); werr != nil {
				return nil, werr
			}
			written += int64(n)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if d.progress != nil {
			progress := d.calculateProgress(written, totalSize)
			d.progress(progress.Downloaded, progress.Total, progress.Speed, progress.ETA)
		}
	}

	file.Close()

	if err := os.Rename(partialPath, destPath); err != nil {
		return nil, err
	}

	progress := d.calculateProgress(written, totalSize)
	return progress, nil
}

func (d *Downloader) calculateProgress(downloaded, total int64) *DownloadProgress {
	now := time.Now()

	var speed float64
	var eta time.Duration

	if now.Sub(d.lastUpdate) > time.Second {
		deltaBytes := downloaded - d.lastBytes
		deltaTime := now.Sub(d.lastUpdate).Seconds()
		speed = float64(deltaBytes) / deltaTime

		remaining := total - downloaded
		if speed > 0 {
			eta = time.Duration(float64(remaining)/speed) * time.Second
		}

		d.lastUpdate = now
		d.lastBytes = downloaded
	}

	return &DownloadProgress{
		Downloaded: downloaded,
		Total:      total,
		Speed:      speed,
		ETA:        eta,
	}
}

func CalculateSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func VerifySHA256(filePath, expectedHash string) (bool, error) {
	actualHash, err := CalculateSHA256(filePath)
	if err != nil {
		return false, err
	}

	return strings.EqualFold(actualHash, expectedHash), nil
}

func GetModelPath(user, repo string) string {
	return filepath.Join(config.ModelsPath(), user, repo)
}

func GetModelFilePath(user, repo, quant string) string {
	modelDir := GetModelPath(user, repo)
	return filepath.Join(modelDir, quant+".gguf")
}

func CleanupPartialFiles() error {
	binDir := config.BinPath()
	modelsDir := config.ModelsPath()

	dirs := []string{binDir, modelsDir}

	for _, dir := range dirs {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(path, ".partial") {
				os.Remove(path)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	return nil
}
