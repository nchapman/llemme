package hf

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/nchapman/gollama/internal/config"
)

const (
	baseURL    = "https://huggingface.co"
	apiBase    = "https://huggingface.co/api"
	userAgent  = "gollama/0.1.0"
	maxRetries = 3
	retryDelay = 1 * time.Second
)

type Client struct {
	httpClient     *http.Client
	downloadClient *http.Client
	token          string
}

type ModelInfo struct {
	ModelId      string    `json:"modelId"`
	Author       string    `json:"author"`
	CreatedAt    time.Time `json:"createdAt"`
	LastModified time.Time `json:"lastModified"`
	Private      bool      `json:"private"`
	Gated        bool      `json:"gated"`
	Siblings     []Sibling `json:"siblings"`
	Tags         []string  `json:"tags"`
	CardData     CardData  `json:"cardData"`
}

type Sibling struct {
	RFileName string `json:"rfilename"`
	Size      int64  `json:"size"`
}

type CardData struct {
	License   string   `json:"license"`
	BaseModel string   `json:"base_model"`
	Tags      []string `json:"tags"`
}

type FileTree struct {
	Path string `json:"path"`
	Type string `json:"type"`
	Size int64  `json:"size"`
}

type SearchResult struct {
	ModelId      string    `json:"modelId"`
	Author       string    `json:"author"`
	LastModified time.Time `json:"lastModified"`
	Private      bool      `json:"private"`
	Gated        bool      `json:"gated"`
	Downloads    int64     `json:"downloads"`
	LibraryName  string    `json:"library_name"`
	Tags         []string  `json:"tags"`
}

func NewClient(cfg *config.Config) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		downloadClient: &http.Client{
			Transport: &http.Transport{
				ResponseHeaderTimeout: 30 * time.Second,
			},
		},
		token: getToken(cfg),
	}
}

func getToken(cfg *config.Config) string {
	if token := os.Getenv("HF_TOKEN"); token != "" {
		return token
	}

	tokenPath := filepath.Join(config.GetHomeDir(), ".cache", "huggingface", "token")
	if data, err := os.ReadFile(tokenPath); err == nil {
		return string(data)
	}

	return cfg.HFToken
}

func (c *Client) doRequest(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", userAgent)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	var lastErr error
	for i := 0; i < maxRetries; i++ {
		resp, err := c.httpClient.Do(req)
		if err == nil {
			if resp.StatusCode == http.StatusTooManyRequests {
				resp.Body.Close()
				time.Sleep(retryDelay * time.Duration(i+1))
				continue
			}
			return resp, nil
		}
		lastErr = err
		time.Sleep(retryDelay)
	}

	return nil, fmt.Errorf("request failed after %d retries: %w", maxRetries, lastErr)
}

func (c *Client) GetModel(user, repo string) (*ModelInfo, error) {
	url := fmt.Sprintf("%s/models/%s/%s", apiBase, user, repo)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var model ModelInfo
	if err := json.NewDecoder(resp.Body).Decode(&model); err != nil {
		return nil, err
	}

	return &model, nil
}

func (c *Client) ListFiles(user, repo, branch string) ([]FileTree, error) {
	url := fmt.Sprintf("%s/models/%s/%s/tree/%s", apiBase, user, repo, branch)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var files []FileTree
	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		return nil, err
	}

	return files, nil
}

func (c *Client) SearchModels(query string, limit int) ([]SearchResult, error) {
	url := fmt.Sprintf("%s/models?search=%s&limit=%d", apiBase, query, limit)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var results []SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}

	return results, nil
}

func (c *Client) DownloadFile(user, repo, branch, filename string, progress func(int64, int64)) (string, error) {
	url := fmt.Sprintf("%s/%s/%s/resolve/%s/%s", baseURL, user, repo, branch, filename)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	tmpPath := filepath.Join(config.BinPath(), filename+".partial")
	out, err := os.Create(tmpPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	size := resp.ContentLength
	written := int64(0)
	buf := make([]byte, 32*1024)

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			_, werr := out.Write(buf[:n])
			if werr != nil {
				return "", werr
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
			return "", err
		}
	}

	finalPath := filepath.Join(config.BinPath(), filename)
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return "", err
	}

	return finalPath, nil
}
