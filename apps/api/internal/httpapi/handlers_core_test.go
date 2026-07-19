package httpapi

import "testing"

func TestOpenAISafeguardsStatusWarnsWhenBudgetConfigMissing(t *testing.T) {
	status := openAISafeguardsStatus(Config{
		LLMProvider:               "openai",
		PublicRateLimitWindowSecs: 60,
		RateLimitDemoSeed:         6,
		RateLimitGMRespond:        30,
		RateLimitSTT:              20,
		RateLimitVision:           30,
		RateLimitBuilder:          20,
	})

	if configured, _ := status["configured"].(bool); configured {
		t.Fatalf("expected safeguards to be unconfigured")
	}
	warnings, _ := status["warnings"].([]string)
	if len(warnings) < 3 {
		t.Fatalf("expected multiple warnings, got %v", warnings)
	}
}

func TestOpenAISafeguardsStatusAcceptsValidBudgetConfig(t *testing.T) {
	status := openAISafeguardsStatus(Config{
		LLMProvider:               "openai",
		PublicRateLimitWindowSecs: 60,
		RateLimitDemoSeed:         6,
		RateLimitGMRespond:        30,
		RateLimitSTT:              20,
		RateLimitVision:           30,
		RateLimitBuilder:          20,
		OpenAIBudgetSoftLimitUSD:  10,
		OpenAIBudgetHardLimitUSD:  25,
		OpenAIUsageAlertEmail:     "alerts@example.com",
	})

	if configured, _ := status["configured"].(bool); !configured {
		t.Fatalf("expected safeguards to be configured, got %v", status["warnings"])
	}
	warnings, _ := status["warnings"].([]string)
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
}
