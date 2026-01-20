package server

import (
	"bufio"
	"bytes"
	"context"
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

type StreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

type ChatCompletionRequest struct {
	Model           string         `json:"model"`
	Messages        []ChatMessage  `json:"messages"`
	Stream          bool           `json:"stream"`
	StreamOptions   *StreamOptions `json:"stream_options,omitempty"`
	Temperature     float64        `json:"temperature,omitempty"`
	TopP            float64        `json:"top_p,omitempty"`
	TopK            int            `json:"top_k,omitempty"`
	MinP            float64        `json:"min_p,omitempty"`
	RepeatPenalty   float64        `json:"repeat_penalty,omitempty"`
	FreqPenalty     float64        `json:"frequency_penalty,omitempty"`
	PresencePenalty float64        `json:"presence_penalty,omitempty"`
	MaxTokens       int            `json:"max_tokens,omitempty"`
	ReasoningFormat string         `json:"reasoning_format,omitempty"`
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
	Usage   *Usage         `json:"usage,omitempty"`
	Timings *Timings       `json:"timings,omitempty"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type Timings struct {
	PredictedN         int     `json:"predicted_n"`
	PredictedMS        float64 `json:"predicted_ms"`
	PredictedPerSecond float64 `json:"predicted_per_second"`
	PromptN            int     `json:"prompt_n"`
	PromptMS           float64 `json:"prompt_ms"`
	PromptPerSecond    float64 `json:"prompt_per_second"`
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
// TimingsCallback is called with timing stats from the final chunk (optional).
type StreamCallback struct {
	ContentCallback   func(string)
	ReasoningCallback func(string)
	TimingsCallback   func(*Timings)
}

func (api *APIClient) StreamChatCompletion(ctx context.Context, req *ChatCompletionRequest, cb StreamCallback) error {
	url := fmt.Sprintf("%s/v1/chat/completions", api.baseURL)

	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
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
		// Check for context cancellation
		if ctx.Err() != nil {
			return ctx.Err()
		}

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

			// Call timings callback if we got timing stats (usually in final chunk)
			if chunk.Timings != nil && cb.TimingsCallback != nil {
				cb.TimingsCallback(chunk.Timings)
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

// StopModel unloads a model from the proxy server.
func (api *APIClient) StopModel(model string) error {
	type StopModelRequest struct {
		Model string `json:"model"`
	}

	url := fmt.Sprintf("%s/api/stop", api.baseURL)

	req := StopModelRequest{Model: model}
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
		return fmt.Errorf("stop model failed: HTTP %d: %s", resp.StatusCode, string(body))
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

// RunOptions contains server options for loading a model.
// Use pointers to distinguish "not set" from "explicitly zero"
// (e.g., GpuLayers=0 means CPU-only, nil means use default).
type RunOptions struct {
	CtxSize   *int           `json:"ctx_size,omitempty"`
	GpuLayers *int           `json:"gpu_layers,omitempty"`
	Threads   *int           `json:"threads,omitempty"`
	Options   map[string]any `json:"options,omitempty"` // Additional llama-server options
}

// IntPtr is a helper to create a pointer to an int value.
func IntPtr(v int) *int {
	return &v
}

// Run loads a model with optional server options.
// This calls /api/run which loads the model with the specified options.
// Explicit fields (CtxSize, etc.) take precedence over Options map.
func (api *APIClient) Run(model string, opts *RunOptions) error {
	type RunRequest struct {
		Model     string         `json:"model"`
		CtxSize   *int           `json:"ctx_size,omitempty"`
		GpuLayers *int           `json:"gpu_layers,omitempty"`
		Threads   *int           `json:"threads,omitempty"`
		Options   map[string]any `json:"options,omitempty"`
	}

	url := fmt.Sprintf("%s/api/run", api.baseURL)

	req := RunRequest{Model: model}
	if opts != nil {
		req.CtxSize = opts.CtxSize
		req.GpuLayers = opts.GpuLayers
		req.Threads = opts.Threads
		req.Options = opts.Options
	}

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
		return fmt.Errorf("run model failed: HTTP %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
