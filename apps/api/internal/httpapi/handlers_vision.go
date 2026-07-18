package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func (h *Handler) stabilizeDiceFrames(c *gin.Context) {
	var req StabilizeDiceFramesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid dice stabilization payload", err)
		return
	}

	if len(req.Frames) == 0 {
		errorResponse(c, http.StatusBadRequest, "stabilize dice frames", fmt.Errorf("at least one frame is required"))
		return
	}

	requiredMatches := req.MinConsensus
	if requiredMatches < 2 {
		requiredMatches = 2
	}
	if requiredMatches > 3 {
		requiredMatches = 3
	}

	start := len(req.Frames) - requiredMatches
	if start < 0 {
		start = 0
	}
	recentFrames := req.Frames[start:]

	type aggregate struct {
		matches    int
		confidence float64
		dice       []DiceResult
	}

	signatureOrder := make([]string, 0, len(recentFrames))
	signatures := make(map[string]*aggregate)
	for _, frame := range recentFrames {
		normalized := normalizeDiceResults(frame.Dice)
		signature := diceSignature(normalized)
		if _, exists := signatures[signature]; !exists {
			signatureOrder = append(signatureOrder, signature)
			signatures[signature] = &aggregate{
				dice: normalized,
			}
		}
		signatures[signature].matches++
		signatures[signature].confidence += frame.Confidence
	}

	bestSignature := ""
	bestMatches := 0
	bestConfidence := 0.0
	var bestDice []DiceResult
	for _, signature := range signatureOrder {
		item := signatures[signature]
		averageConfidence := 0.0
		if item.matches > 0 {
			averageConfidence = item.confidence / float64(item.matches)
		}
		if item.matches > bestMatches || (item.matches == bestMatches && averageConfidence > bestConfidence) {
			bestSignature = signature
			bestMatches = item.matches
			bestConfidence = averageConfidence
			bestDice = item.dice
		}
	}

	c.JSON(http.StatusOK, StabilizeDiceFramesResponse{
		Stable:          bestMatches >= requiredMatches,
		RequiredMatches: requiredMatches,
		MatchingFrames:  bestMatches,
		StableDice:      bestDice,
		Confidence:      bestConfidence,
		Signature:       bestSignature,
		RecentFrames:    recentFrames,
	})
}

func (h *Handler) detectDice(c *gin.Context) {
	var req DetectDiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid dice detection payload", err)
		return
	}

	if !strings.HasPrefix(req.ImageDataURL, "data:image/") {
		errorResponse(c, http.StatusBadRequest, "detect dice", fmt.Errorf("image_data_url must be a data:image URL"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
	defer cancel()

	result, err := h.llmClient.DetectDiceFromImage(ctx, req.ImageDataURL, req.Language)
	if err != nil {
		result, err = h.visionClient.DetectDice(ctx, req)
		if err != nil {
			errorResponse(c, http.StatusBadGateway, "detect dice", err)
			return
		}
		if strings.TrimSpace(result.Notes) == "" {
			result.Notes = "Fell back to the local CV detector."
		} else {
			result.Notes = result.Notes + " Fallback path: local CV detector."
		}
	}

	c.JSON(http.StatusOK, result)
}

func normalizeDiceResults(dice []DiceResult) []DiceResult {
	normalized := append([]DiceResult(nil), dice...)
	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].Type == normalized[j].Type {
			return normalized[i].Value < normalized[j].Value
		}
		return normalized[i].Type < normalized[j].Type
	})
	return normalized
}

func diceSignature(dice []DiceResult) string {
	parts := make([]string, 0, len(dice))
	for _, die := range dice {
		parts = append(parts, fmt.Sprintf("%s:%d", strings.ToLower(strings.TrimSpace(die.Type)), die.Value))
	}
	return strings.Join(parts, "|")
}
