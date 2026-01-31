package peer

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/nchapman/lleme/internal/hf"
	"github.com/nchapman/lleme/internal/version"
)

const (
	// ClientTimeout is the default timeout for peer HTTP requests
	ClientTimeout = 30 * time.Second

	// DownloadBufferSize is the buffer size for file downloads
	DownloadBufferSize = 32 * 1024
)

// Client handles HTTP communication with a peer using hash-based requests.
type Client struct {
	peer       *Peer
	httpClient *http.Client
}

// NewClient creates a client for communicating with a peer.
func NewClient(peer *Peer) *Client {
	return &Client{
		peer: peer,
		httpClient: &http.Client{
			Timeout: ClientTimeout,
		},
	}
}

// HasHash checks if the peer has a file with the given SHA256 hash.
// Returns the file size if found, or 0 and false if not found.
func (c *Client) HasHash(hash string) (size int64, ok bool) {
	url := fmt.Sprintf("http://%s:%d/api/peer/sha256/%s", c.peer.Host, c.peer.Port, hash)

	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return 0, false
	}
	req.Header.Set("User-Agent", version.UserAgent())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, false
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, false
	}

	return resp.ContentLength, true
}

// DownloadProgressCallback is called during download with progress updates.
type DownloadProgressCallback func(downloaded, total int64)

// DownloadHash downloads a file by its SHA256 hash from the peer.
func (c *Client) DownloadHash(hash, destPath string, progress DownloadProgressCallback) error {
	url := fmt.Sprintf("http://%s:%d/api/peer/sha256/%s", c.peer.Host, c.peer.Port, hash)

	// Use a client with no timeout for large downloads
	downloadClient := &http.Client{
		Transport: &http.Transport{
			ResponseHeaderTimeout: ClientTimeout,
		},
	}

	partialPath := destPath + ".partial"
	fileSize := int64(0)

	// Check for partial download to resume
	if info, err := os.Stat(partialPath); err == nil {
		fileSize = info.Size()
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", version.UserAgent())

	if fileSize > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", fileSize))
	}

	resp, err := downloadClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to contact peer: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("file not found on peer")
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("peer returned HTTP %d", resp.StatusCode)
	}

	flags := os.O_CREATE | os.O_WRONLY
	if resp.StatusCode == http.StatusOK {
		// Server didn't honor Range request, start fresh
		flags |= os.O_TRUNC
		fileSize = 0
	} else {
		flags |= os.O_APPEND
	}

	// Calculate total size after handling status (fileSize may have been reset)
	totalSize := fileSize + resp.ContentLength

	file, err := os.OpenFile(partialPath, flags, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	buf := make([]byte, DownloadBufferSize)
	written := fileSize

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := file.Write(buf[:n]); werr != nil {
				return werr
			}
			written += int64(n)
			if progress != nil {
				progress(written, totalSize)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	// Move partial to final destination
	if err := os.Rename(partialPath, destPath); err != nil {
		return err
	}

	return nil
}

// VerifyDownload checks the SHA256 hash of a downloaded file.
func VerifyDownload(path, expectedHash string) (bool, error) {
	if expectedHash == "" {
		return true, nil
	}

	actualHash, err := hf.CalculateSHA256(path)
	if err != nil {
		return false, err
	}

	return strings.EqualFold(actualHash, expectedHash), nil
}
