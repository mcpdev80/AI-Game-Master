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
	provider   string
	baseURL    string
	model      string
	apiKey     string
	voice      string
	httpClient *http.Client
}

func NewTTSClient(cfg Config) *TTSClient {
	return &TTSClient{
		provider: strings.ToLower(strings.TrimSpace(cfg.TTSProvider)),
		baseURL:  strings.TrimRight(cfg.TTSBaseURL, "/"),
		model:    cfg.TTSModel,
		apiKey:   cfg.TTSAPIKey,
		voice:    cfg.TTSVoice,
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
		if c.provider == "openai" {
			payload["instructions"] = instructions
		} else {
			payload["instruct"] = instructions
		}
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
	if c.apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

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

func (c *TTSClient) Provider() string { return c.provider }
func (c *TTSClient) BaseURL() string  { return c.baseURL }
func (c *TTSClient) Model() string    { return c.model }

func (c *TTSClient) VoiceForProfile(profileID string) string {
	if c.provider != "openai" {
		return resolveLocalVoiceProviderID(profileID)
	}
	switch strings.ToLower(strings.TrimSpace(profileID)) {
	case "npc-default":
		return "marin"
	case "orc-deep":
		return "onyx"
	case "elf-bright":
		return "shimmer"
	case "rules-neutral":
		return "sage"
	default:
		if strings.TrimSpace(c.voice) != "" {
			return strings.TrimSpace(c.voice)
		}
		return "cedar"
	}
}
