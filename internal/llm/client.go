// Package llm talks to an OpenAI-compatible chat completions endpoint
// (initially a llama-swap instance).
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Message is a single chat message in OpenAI chat-completions format.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Client is a chat-completions client for an OpenAI-compatible backend.
type Client struct {
	baseURL    string
	model      string
	apiKey     string
	httpClient *http.Client
}

// New builds a Client. The underlying http.Client is created once and
// reused across requests.
func New(baseURL, model, apiKey string, timeout time.Duration) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		model:      model,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: timeout},
	}
}

type chatCompletionRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// ChatCompletion sends messages to the configured model and returns the
// assistant's reply content.
func (c *Client) ChatCompletion(ctx context.Context, messages []Message) (string, error) {
	reqBody := chatCompletionRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   false,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal chat completion request: %w", err)
	}

	url := c.baseURL + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build chat completion request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("chat completion request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read chat completion response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("chat completion request failed: status %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed chatCompletionResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("parse chat completion response: %w (body: %s)", err, string(respBody))
	}

	if parsed.Error != nil {
		return "", fmt.Errorf("llm returned error: %s", parsed.Error.Message)
	}

	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("llm response contained no choices")
	}

	return parsed.Choices[0].Message.Content, nil
}
