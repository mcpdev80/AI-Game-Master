package httpapi

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

type TTSClient struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

func NewTTSClient(cfg Config) *TTSClient {
	return &TTSClient{
		baseURL: strings.TrimRight(cfg.TTSBaseURL, "/"),
		model:   cfg.TTSModel,
		httpClient: &http.Client{
			Timeout: 240 * time.Second,
		},
	}
}

func (c *TTSClient) Synthesize(ctx context.Context, text string, voiceID string, instructions string) ([]byte, string, error) {
	payload := map[string]any{
		"input":           text,
		"voice":           voiceID,
		"response_format": "wav",
	}
	if strings.TrimSpace(instructions) != "" {
		payload["instruct"] = instructions
	}
	if strings.TrimSpace(c.model) != "" {
		// Some providers ignore unknown fields; keep model for OpenAI-style backends.
		payload["model"] = c.model
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/audio/speech", bytes.NewReader(body))
	if err != nil {
		return nil, "", err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, "", err
	}
	defer response.Body.Close()

	rawBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, "", err
	}
	if response.StatusCode >= 300 {
		return nil, "", fmt.Errorf("tts request failed with status %d: %s", response.StatusCode, string(rawBody))
	}

	contentType := strings.TrimSpace(response.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "audio/mpeg"
	}
	return rawBody, contentType, nil
}
