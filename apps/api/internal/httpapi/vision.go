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

type VisionClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewVisionClient(cfg Config) *VisionClient {
	return &VisionClient{
		baseURL: strings.TrimRight(cfg.VisionBaseURL, "/"),
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
}

func (c *VisionClient) DetectDice(ctx context.Context, req DetectDiceRequest) (DetectDiceResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return DetectDiceResponse{}, err
	}

	endpoint := c.baseURL + "/detect/dice"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return DetectDiceResponse{}, err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return DetectDiceResponse{}, err
	}
	defer response.Body.Close()

	rawBody, err := io.ReadAll(response.Body)
	if err != nil {
		return DetectDiceResponse{}, err
	}
	if response.StatusCode >= 300 {
		return DetectDiceResponse{}, fmt.Errorf("vision request failed with status %d: %s", response.StatusCode, string(rawBody))
	}

	var result DetectDiceResponse
	if err := json.Unmarshal(rawBody, &result); err != nil {
		return DetectDiceResponse{}, err
	}
	result.Dice = normalizeDiceResults(result.Dice)
	if result.Dice == nil {
		result.Dice = []DiceResult{}
	}
	if result.Boxes == nil {
		result.Boxes = []DiceBox{}
	}
	if result.DiceCount == 0 && len(result.Boxes) > 0 {
		result.DiceCount = len(result.Boxes)
	}
	return result, nil
}
