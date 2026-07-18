package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type responseJSONSchema struct {
	Name     string
	Schema   map[string]any
	JSONMode bool
}

type OpenAIResponseError struct {
	Kind       string
	StatusCode int
	Message    string
}

func (e *OpenAIResponseError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("openai responses %s (%d): %s", e.Kind, e.StatusCode, e.Message)
	}
	return fmt.Sprintf("openai responses %s: %s", e.Kind, e.Message)
}

func (c *LLMClient) responsesCompletion(ctx context.Context, messages []map[string]string, maxTokens int, format *responseJSONSchema) (string, string, error) {
	if maxTokens < 800 {
		// Reasoning tokens count against this budget. Keep short calls reliable
		// while allowing the model to stop naturally before the limit.
		maxTokens = 800
	}
	payload := map[string]any{
		"model":             c.currentModel(),
		"input":             messages,
		"max_output_tokens": maxTokens,
		"store":             c.storeResponses,
	}
	if effort := strings.TrimSpace(c.reasoningEffort); effort != "" {
		payload["reasoning"] = map[string]any{"effort": effort}
	}
	if identifier := safetyIdentifierFromContext(ctx); identifier != "" {
		payload["safety_identifier"] = identifier
	}
	if format != nil {
		if format.JSONMode {
			payload["text"] = map[string]any{"format": map[string]any{"type": "json_object"}}
		} else {
			payload["text"] = map[string]any{"format": map[string]any{
				"type":   "json_schema",
				"name":   format.Name,
				"strict": true,
				"schema": format.Schema,
			}}
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.currentBaseURL()+"/responses", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	request.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	call := func(callCtx context.Context) (string, string, error) {
		return c.doResponsesCompletion(request.Clone(callCtx))
	}
	if c.gateway == nil {
		return call(ctx)
	}
	return c.gateway.run(ctx, messages, maxTokens, call)
}

func (c *LLMClient) detectDiceWithResponses(ctx context.Context, imageDataURL string, language string) (DetectDiceResponse, error) {
	format := diceVisionSchema()
	payload := map[string]any{
		"model": c.currentModel(),
		"input": []map[string]any{
			{
				"role":    "system",
				"content": []map[string]any{{"type": "input_text", "text": mustReadEmbeddedPrompt("prompts/dice_vision_system_prompt.md")}},
			},
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": fmt.Sprintf("Notes language: %s. Count clearly visible tabletop dice, then read only clearly visible top faces. Return fewer values when uncertain.", language)},
					{"type": "input_image", "image_url": imageDataURL},
				},
			},
		},
		"max_output_tokens": 700,
		"store":             c.storeResponses,
		"reasoning":         map[string]any{"effort": c.reasoningEffort},
		"text": map[string]any{"format": map[string]any{
			"type": "json_schema", "name": format.Name, "strict": true, "schema": format.Schema,
		}},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return DetectDiceResponse{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.currentBaseURL()+"/responses", bytes.NewReader(body))
	if err != nil {
		return DetectDiceResponse{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	content, model, err := c.doResponsesCompletion(request)
	if err != nil {
		return DetectDiceResponse{}, err
	}
	var result DetectDiceResponse
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return DetectDiceResponse{}, &OpenAIResponseError{Kind: "invalid_response", Message: "parse dice response: " + err.Error()}
	}
	result.RawModel = model
	return result, nil
}

func (c *LLMClient) doResponsesCompletion(request *http.Request) (string, string, error) {
	response, err := c.httpClient.Do(request)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", "", &OpenAIResponseError{Kind: "timeout", Message: err.Error()}
		}
		return "", "", err
	}
	defer response.Body.Close()
	rawBody, err := io.ReadAll(response.Body)
	if err != nil {
		return "", "", err
	}
	if response.StatusCode >= 300 {
		kind := "api_error"
		if response.StatusCode == http.StatusTooManyRequests {
			kind = "rate_limit"
		} else if response.StatusCode == http.StatusBadRequest && strings.Contains(strings.ToLower(string(rawBody)), "schema") {
			kind = "invalid_schema"
		}
		return "", "", &OpenAIResponseError{Kind: kind, StatusCode: response.StatusCode, Message: compactAPIError(rawBody)}
	}

	var result struct {
		Model             string `json:"model"`
		Status            string `json:"status"`
		IncompleteDetails *struct {
			Reason string `json:"reason"`
		} `json:"incomplete_details"`
		Output []struct {
			Type    string `json:"type"`
			Content []struct {
				Type    string `json:"type"`
				Text    string `json:"text"`
				Refusal string `json:"refusal"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(rawBody, &result); err != nil {
		return "", "", &OpenAIResponseError{Kind: "invalid_response", Message: err.Error()}
	}
	if result.Status != "completed" {
		reason := result.Status
		if result.IncompleteDetails != nil && result.IncompleteDetails.Reason != "" {
			reason = result.IncompleteDetails.Reason
		}
		return "", result.Model, &OpenAIResponseError{Kind: "incomplete", Message: reason}
	}
	for _, output := range result.Output {
		if output.Type != "message" {
			continue
		}
		for _, content := range output.Content {
			switch content.Type {
			case "refusal":
				return "", result.Model, &OpenAIResponseError{Kind: "refusal", Message: content.Refusal}
			case "output_text":
				if strings.TrimSpace(content.Text) != "" {
					return content.Text, result.Model, nil
				}
			}
		}
	}
	return "", result.Model, &OpenAIResponseError{Kind: "invalid_response", Message: "completed response contained no output_text"}
}

func compactAPIError(raw []byte) string {
	var payload struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &payload) == nil && payload.Error.Message != "" {
		return payload.Error.Message
	}
	return strings.TrimSpace(string(raw))
}

func safetyIdentifierFromContext(ctx context.Context) string {
	meta := llmRequestMetaFromContext(ctx)
	raw := strings.TrimSpace(meta.ScopeType + ":" + meta.ScopeID)
	if raw == ":" {
		return ""
	}
	digest := sha256.Sum256([]byte(raw))
	return "agm_" + hex.EncodeToString(digest[:16])
}

func encounterTurnSchema() *responseJSONSchema {
	nullableString := map[string]any{"type": []string{"string", "null"}}
	nullableInteger := map[string]any{"type": []string{"integer", "null"}}
	rollProperties := map[string]any{
		"type":         map[string]any{"type": "string", "enum": []string{"attack", "damage", "check", "save"}},
		"label":        map[string]any{"type": "string"},
		"dice":         map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"ability":      nullableString,
		"skill":        nullableString,
		"dc":           nullableInteger,
		"hide_dc":      map[string]any{"type": "boolean"},
		"reason":       nullableString,
		"instructions": nullableString,
	}
	followUpProperties := make(map[string]any, len(rollProperties))
	for key, value := range rollProperties {
		followUpProperties[key] = value
	}
	followUp := map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": followUpProperties,
		"required":   []string{"type", "label", "dice", "ability", "skill", "dc", "hide_dc", "reason", "instructions"},
	}
	rollProperties["follow_up_on_success"] = map[string]any{"anyOf": []any{followUp, map[string]any{"type": "null"}}}
	roll := map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": rollProperties,
		"required":   []string{"type", "label", "dice", "ability", "skill", "dc", "hide_dc", "reason", "instructions", "follow_up_on_success"},
	}
	schema := map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{
			"narration":    map[string]any{"type": "string"},
			"language":     map[string]any{"type": "string", "enum": []string{"en", "de"}},
			"rules_used":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"roll_request": map[string]any{"anyOf": []any{roll, map[string]any{"type": "null"}}},
			"state_updates": map[string]any{"type": "array", "items": map[string]any{
				"type": "object", "additionalProperties": false,
				"properties": map[string]any{
					"entity_id": map[string]any{"type": "string"}, "field": map[string]any{"type": "string"},
					"delta": map[string]any{"type": "integer"}, "value": map[string]any{"type": "string"},
				}, "required": []string{"entity_id", "field", "delta", "value"},
			}},
			"scene_events": map[string]any{"type": "array", "items": map[string]any{
				"type": "object", "additionalProperties": false,
				"properties": map[string]any{
					"type": map[string]any{"type": "string", "enum": []string{"sfx", "music", "ambience", "video", "image", "map", "portrait"}},
					"name": map[string]any{"type": "string"},
				},
				"required": []string{"type", "name"},
			}},
			"dm_notes": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		},
		"required": []string{"narration", "language", "rules_used", "roll_request", "state_updates", "scene_events", "dm_notes"},
	}
	return &responseJSONSchema{Name: "encounter_turn_v1", Schema: schema}
}

func diceVisionSchema() *responseJSONSchema {
	box := map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{
			"x": map[string]any{"type": "integer"}, "y": map[string]any{"type": "integer"},
			"w": map[string]any{"type": "integer"}, "h": map[string]any{"type": "integer"},
		},
		"required": []string{"x", "y", "w", "h"},
	}
	die := map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{
			"type": map[string]any{"type": "string"}, "value": map[string]any{"type": "integer"},
		},
		"required": []string{"type", "value"},
	}
	return &responseJSONSchema{Name: "dice_vision_v1", Schema: map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{
			"dice":       map[string]any{"type": "array", "items": die},
			"dice_count": map[string]any{"type": "integer"},
			"boxes":      map[string]any{"type": "array", "items": box},
			"confidence": map[string]any{"type": "number"},
			"notes":      map[string]any{"type": "string"},
		},
		"required": []string{"dice", "dice_count", "boxes", "confidence", "notes"},
	}}
}
