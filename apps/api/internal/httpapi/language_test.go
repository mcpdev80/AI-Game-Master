package httpapi

import (
	"errors"
	"strings"
	"testing"
)

func TestFallbackGMResponseUsesRequestedLanguage(t *testing.T) {
	session := Session{Language: "de"}

	english := fallbackGMResponse(session, GMRespondRequest{Language: "en"}, errors.New("offline"))
	if english.Language != "en" || !strings.Contains(english.Narration, "Your action") {
		t.Fatalf("expected English fallback, got language=%q narration=%q", english.Language, english.Narration)
	}
	if strings.Contains(english.Narration, "Deine Aktion") {
		t.Fatalf("English fallback contains German text: %q", english.Narration)
	}

	german := fallbackGMResponse(session, GMRespondRequest{Language: "de"}, errors.New("offline"))
	if german.Language != "de" || !strings.Contains(german.Narration, "Deine Aktion") {
		t.Fatalf("expected German fallback, got language=%q narration=%q", german.Language, german.Narration)
	}
}

func TestBuilderFallbackReplyUsesSelectedLanguage(t *testing.T) {
	character := &Character{Metadata: map[string]any{"builder_stage": "concept"}}
	english := builderFallbackReply("concept", character, "en")
	german := builderFallbackReply("concept", character, "de")

	if !strings.Contains(english, "describe") || strings.Contains(english, "Beschreibe") {
		t.Fatalf("unexpected English builder fallback: %q", english)
	}
	if !strings.Contains(german, "Erzähl") {
		t.Fatalf("unexpected German builder fallback: %q", german)
	}
}
