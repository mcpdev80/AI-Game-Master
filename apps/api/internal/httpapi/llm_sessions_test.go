package httpapi

import (
	"strings"
	"testing"
)

func TestCompactLLMHistoryKeepsRecentWindowAndArchivesFacts(t *testing.T) {
	history := []map[string]any{
		{"role": "system", "content": "System rule"},
		{"role": "user", "content": "Old player action"},
		{"role": "assistant", "content": "Old narration"},
		{"role": "user", "content": "Recent player action"},
		{"role": "assistant", "content": "Recent narration"},
	}

	live, facts := compactLLMHistory(history, 2)

	if len(live) != 2 {
		t.Fatalf("expected 2 live entries, got %d", len(live))
	}
	if live[0]["content"] != "Recent player action" || live[1]["content"] != "Recent narration" {
		t.Fatalf("unexpected live window: %#v", live)
	}
	if len(facts) == 0 {
		t.Fatal("expected archived facts")
	}
	if !strings.Contains(facts[0], "system: System rule") {
		t.Fatalf("expected system fact to be preserved, got %v", facts)
	}
}

func TestCompactLLMHistoryIgnoresEmptyEntries(t *testing.T) {
	history := []map[string]any{
		{"role": "", "content": "ignored"},
		{"role": "assistant", "content": ""},
		{"role": "user", "content": "Recent"},
	}

	live, facts := compactLLMHistory(history, 1)

	if len(live) != 1 {
		t.Fatalf("expected 1 live entry, got %d", len(live))
	}
	if len(facts) != 0 {
		t.Fatalf("expected 0 facts from empty archived entries, got %v", facts)
	}
}

func TestRecentHistoryCapsWindowAtEight(t *testing.T) {
	history := make([]map[string]any, 0, 12)
	for i := 0; i < 12; i++ {
		history = append(history, map[string]any{"role": "user", "content": i})
	}

	recent := recentHistory(history, 99)
	if len(recent) != 8 {
		t.Fatalf("expected 8 recent entries, got %d", len(recent))
	}
}

func TestMessageHistoryToStringsDropsInvalidEntries(t *testing.T) {
	history := []map[string]any{
		{"role": "user", "content": "Keep me"},
		{"role": "", "content": "drop"},
		{"role": "assistant", "content": ""},
	}

	items := messageHistoryToStrings(history)
	if len(items) != 1 {
		t.Fatalf("expected 1 valid entry, got %d", len(items))
	}
	if items[0]["role"] != "user" || items[0]["content"] != "Keep me" {
		t.Fatalf("unexpected item: %#v", items[0])
	}
}
