package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type STTClient struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

func NewSTTClient(cfg Config) *STTClient {
	return &STTClient{
		baseURL: strings.TrimRight(cfg.STTBaseURL, "/"),
		model:   cfg.STTModel,
		httpClient: &http.Client{
			Timeout: 240 * time.Second,
		},
	}
}

var (
	sttHypothesisDebugPattern = regexp.MustCompile(`(?is)Hypothesis\([^)]*\)`)
	sttDecoderDebugPattern    = regexp.MustCompile(`(?is),?\s*text='[^']*',\s*dec_out=.*?last_frame=None\)`)
)

func (c *STTClient) Transcribe(ctx context.Context, filename string, contentType string, data []byte) (string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(data); err != nil {
		return "", err
	}
	if strings.TrimSpace(c.model) != "" {
		_ = writer.WriteField("model", c.model)
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/audio/transcriptions", bytes.NewReader(body.Bytes()))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if strings.TrimSpace(contentType) != "" {
		request.Header.Set("X-Original-Content-Type", contentType)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	rawBody, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	if response.StatusCode >= 300 {
		return "", fmt.Errorf("stt request failed with status %d: %s", response.StatusCode, string(rawBody))
	}

	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return "", err
	}
	text := sanitizeSTTTranscript(payload.Text)
	if text == "" {
		return "", fmt.Errorf("empty transcription result")
	}
	return text, nil
}

func sanitizeSTTTranscript(input string) string {
	text := strings.TrimSpace(input)
	if text == "" {
		return ""
	}
	text = sttHypothesisDebugPattern.ReplaceAllString(text, " ")
	text = sttDecoderDebugPattern.ReplaceAllString(text, " ")
	text = strings.Join(strings.Fields(text), " ")
	return strings.TrimSpace(text)
}
