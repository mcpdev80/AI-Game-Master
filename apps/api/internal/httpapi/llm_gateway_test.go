package httpapi

import (
	"context"
	"strings"
	"testing"
)

func TestEncounterGatewayProfilesFitCampaignSessionBudget(t *testing.T) {
	gateway := NewLLMGateway(Config{LLMMaxConcurrent: 1})
	for _, profile := range []string{"scene", "opening", "reopening"} {
		limit := gateway.limitFor(profile, 0)
		if limit.MaxInputTokens < 12000 {
			t.Fatalf("%s input budget = %d, want at least 12000", profile, limit.MaxInputTokens)
		}
	}
}

func TestGatewayRejectsPromptBeyondProfileBudget(t *testing.T) {
	gateway := NewLLMGateway(Config{LLMMaxConcurrent: 1})
	message := map[string]string{"role": "user", "content": strings.Repeat("x", 12001*4)}
	called := false
	_, _, err := gateway.run(withLLMRequestMeta(t.Context(), llmRequestMeta{Profile: "scene"}), []map[string]string{message}, 100, func(_ context.Context) (string, string, error) {
		called = true
		return "", "", nil
	})
	if err == nil || called {
		t.Fatalf("oversized prompt should fail before provider call: err=%v called=%v", err, called)
	}
}
