package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type APIClient struct {
	baseURL string
	client  *http.Client
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionRequest struct {
	Model           string        `json:"model"`
	Messages        []ChatMessage `json:"messages"`
	Stream          bool          `json:"stream"`
	Temperature     float64       `json:"temperature,omitempty"`
	TopP            float64       `json:"top_p,omitempty"`
	TopK            int           `json:"top_k,omitempty"`
	MaxTokens       int           `json:"max_tokens,omitempty"`
	ReasoningFormat string        `json:"reasoning_format,omitempty"`
}

type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
}

type Choice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type StreamChoice struct {
	Index        int         `json:"index"`
	Delta        StreamDelta `json:"delta"`
	FinishReason string      `json:"finish_reason"`
}

type StreamDelta struct {
	Role             string `json:"role,omitempty"`
	Content          string `json:"content,omitempty"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
}

type StreamChunk struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []StreamChoice `json:"choices"`
}

type HealthResponse struct {
	Status string `json:"status"`
}

func NewAPIClient(host string, port int) *APIClient {
	return &APIClient{
		baseURL: fmt.Sprintf("http://%s:%d", host, port),
		client:  &http.Client{},
	}
}

func NewAPIClientFromURL(baseURL string) *APIClient {
	return &APIClient{
		baseURL: baseURL,
		client:  &http.Client{},
	}
}

func (api *APIClient) Health() error {
	url := fmt.Sprintf("%s/health", api.baseURL)

	resp, err := api.client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed: HTTP %d", resp.StatusCode)
	}

	return nil
}

func (api *APIClient) ChatCompletion(req *ChatCompletionRequest) (*ChatCompletionResponse, error) {
	url := fmt.Sprintf("%s/v1/chat/completions", api.baseURL)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := api.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("chat completion failed: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var response ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return &response, nil
}

// StreamCallback holds callbacks for streaming chat completion responses.
// ContentCallback is called for regular response content.
// ReasoningCallback is called for reasoning/thinking content (optional).
type StreamCallback struct {
	ContentCallback   func(string)
	ReasoningCallback func(string)
}

func (api *APIClient) StreamChatCompletion(req *ChatCompletionRequest, cb StreamCallback) error {
	url := fmt.Sprintf("%s/v1/chat/completions", api.baseURL)

	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := api.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("chat completion failed: HTTP %d: %s", resp.StatusCode, string(body))
	}

	scanner := bufio.NewScanner(resp.Body)
	parseErrors := 0
	var lastParseErr error

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" || line == "data: [DONE]" {
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			jsonData := strings.TrimPrefix(line, "data: ")

			var chunk StreamChunk
			if err := json.Unmarshal([]byte(jsonData), &chunk); err != nil {
				parseErrors++
				lastParseErr = err
				continue
			}

			if len(chunk.Choices) > 0 {
				delta := chunk.Choices[0].Delta
				if delta.ReasoningContent != "" && cb.ReasoningCallback != nil {
					cb.ReasoningCallback(delta.ReasoningContent)
				}
				if delta.Content != "" && cb.ContentCallback != nil {
					cb.ContentCallback(delta.Content)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	if parseErrors > 10 {
		return fmt.Errorf("stream had %d JSON parse errors, last: %w", parseErrors, lastParseErr)
	}

	return nil
}

func (api *APIClient) SetModel(modelPath string) error {
	type LoadModelRequest struct {
		Model string `json:"model"`
	}

	url := fmt.Sprintf("%s/v1/load", api.baseURL)

	req := LoadModelRequest{Model: modelPath}
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := api.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("load model failed: HTTP %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
