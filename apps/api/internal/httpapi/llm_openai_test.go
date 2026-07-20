package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResponsesCompletionUsesStrictEncounterSchema(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected authorization header: %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
          "model":"gpt-5.6-sol-2026-07-01",
          "status":"completed",
          "output":[{"type":"message","content":[{"type":"output_text","text":"{\"narration\":\"The gate opens.\",\"language\":\"en\",\"rules_used\":[],\"roll_request\":null,\"state_updates\":[],\"scene_events\":[],\"dm_notes\":[]}"}]}]
        }`))
	}))
	defer server.Close()

	client := NewLLMClient(Config{
		LLMProvider:        "openai",
		LLMBaseURL:         server.URL + "/v1",
		LLMModel:           "gpt-5.6",
		LLMAPIKey:          "test-key",
		LLMReasoningEffort: "medium",
		LLMStoreResponses:  false,
	})
	ctx := withLLMRequestMeta(context.Background(), llmRequestMeta{ScopeType: "session", ScopeID: "session-123"})
	content, model, err := client.responsesCompletion(ctx, []map[string]string{
		{"role": "system", "content": "Return a turn."},
		{"role": "user", "content": "Open the gate."},
	}, 900, encounterTurnSchema())
	if err != nil {
		t.Fatal(err)
	}
	if model != "gpt-5.6-sol-2026-07-01" || !strings.Contains(content, "The gate opens") {
		t.Fatalf("unexpected completion: model=%q content=%q", model, content)
	}
	if captured["model"] != "gpt-5.6" || captured["store"] != false {
		t.Fatalf("unexpected core request: %#v", captured)
	}
	if _, exists := captured["messages"]; exists {
		t.Fatal("Responses request must use input, not messages")
	}
	if _, exists := captured["chat_template_kwargs"]; exists {
		t.Fatal("OpenAI request leaked local-provider chat_template_kwargs")
	}
	identifier, _ := captured["safety_identifier"].(string)
	if !strings.HasPrefix(identifier, "agm_") || strings.Contains(identifier, "session-123") {
		t.Fatalf("safety_identifier is not privacy-preserving: %q", identifier)
	}
	textConfig := captured["text"].(map[string]any)
	format := textConfig["format"].(map[string]any)
	if format["type"] != "json_schema" || format["name"] != "encounter_turn_v1" || format["strict"] != true {
		t.Fatalf("strict format missing: %#v", format)
	}
	schema := format["schema"].(map[string]any)
	if schema["additionalProperties"] != false {
		t.Fatalf("top-level schema must reject extra properties: %#v", schema)
	}
}

func TestResponsesCompletionSurfacesRefusal(t *testing.T) {
	client, closeServer := openAITestClient(t, `{
      "model":"gpt-5.6",
      "status":"completed",
      "output":[{"type":"message","content":[{"type":"refusal","refusal":"Cannot help with that."}]}]
    }`)
	defer closeServer()

	_, _, err := client.responsesCompletion(context.Background(), []map[string]string{{"role": "user", "content": "test"}}, 100, nil)
	var responseErr *OpenAIResponseError
	if !errors.As(err, &responseErr) || responseErr.Kind != "refusal" {
		t.Fatalf("expected refusal error, got %v", err)
	}
}

func TestResponsesCompletionSurfacesIncompleteReason(t *testing.T) {
	client, closeServer := openAITestClient(t, `{
      "model":"gpt-5.6",
      "status":"incomplete",
      "incomplete_details":{"reason":"max_output_tokens"},
      "output":[]
    }`)
	defer closeServer()

	_, _, err := client.responsesCompletion(context.Background(), []map[string]string{{"role": "user", "content": "test"}}, 100, nil)
	var responseErr *OpenAIResponseError
	if !errors.As(err, &responseErr) || responseErr.Kind != "incomplete" || responseErr.Message != "max_output_tokens" {
		t.Fatalf("expected incomplete error, got %v", err)
	}
}

func TestResponsesCompletionSurfacesRateLimitError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"rate limit exceeded"}}`))
	}))
	defer server.Close()

	client := NewLLMClient(Config{
		LLMProvider: "openai",
		LLMBaseURL:  server.URL,
		LLMModel:    "gpt-5.6",
	})

	_, _, err := client.responsesCompletion(context.Background(), []map[string]string{{"role": "user", "content": "test"}}, 100, nil)
	var responseErr *OpenAIResponseError
	if !errors.As(err, &responseErr) || responseErr.Kind != "rate_limit" || responseErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected rate_limit error, got %v", err)
	}
}

func TestResponsesCompletionSurfacesInvalidSchemaError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"schema validation failed"}}`))
	}))
	defer server.Close()

	client := NewLLMClient(Config{
		LLMProvider: "openai",
		LLMBaseURL:  server.URL,
		LLMModel:    "gpt-5.6",
	})

	_, _, err := client.responsesCompletion(context.Background(), []map[string]string{{"role": "user", "content": "test"}}, 100, encounterTurnSchema())
	var responseErr *OpenAIResponseError
	if !errors.As(err, &responseErr) || responseErr.Kind != "invalid_schema" || responseErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected invalid_schema error, got %v", err)
	}
}

func TestResponsesCompletionRejectsInvalidJSONBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"gpt-5.6","status":"completed","output":[`))
	}))
	defer server.Close()

	client := NewLLMClient(Config{
		LLMProvider: "openai",
		LLMBaseURL:  server.URL,
		LLMModel:    "gpt-5.6",
	})

	_, _, err := client.responsesCompletion(context.Background(), []map[string]string{{"role": "user", "content": "test"}}, 100, nil)
	var responseErr *OpenAIResponseError
	if !errors.As(err, &responseErr) || responseErr.Kind != "invalid_response" {
		t.Fatalf("expected invalid_response error, got %v", err)
	}
}

func TestResponsesCompletionRejectsCompletedResponseWithoutOutputText(t *testing.T) {
	client, closeServer := openAITestClient(t, `{
      "model":"gpt-5.6",
      "status":"completed",
      "output":[{"type":"message","content":[{"type":"tool_result","text":"ignored"}]}]
    }`)
	defer closeServer()

	_, _, err := client.responsesCompletion(context.Background(), []map[string]string{{"role": "user", "content": "test"}}, 100, nil)
	var responseErr *OpenAIResponseError
	if !errors.As(err, &responseErr) || responseErr.Kind != "invalid_response" {
		t.Fatalf("expected invalid_response error, got %v", err)
	}
}

func openAITestClient(t *testing.T, responseBody string) (*LLMClient, func()) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(responseBody))
	}))
	client := NewLLMClient(Config{
		LLMProvider:        "openai",
		LLMBaseURL:         server.URL,
		LLMModel:           "gpt-5.6",
		LLMReasoningEffort: "medium",
	})
	return client, server.Close
}
