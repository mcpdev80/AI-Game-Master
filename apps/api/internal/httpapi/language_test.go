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

func TestGMKnowledgePromptUsesRequestedEnglish(t *testing.T) {
	prompt := gmKnowledgePrompt(
		Session{Language: "de"},
		GMRespondRequest{Language: "en", PlayerInput: "Who is Brother Benjamin?"},
		[]string{"Die Abtei (adventure)"},
		[]GMContextChunk{{DocumentName: "Die Abtei", ChunkText: "Bruder Benjamin bewacht die Krypta."}},
		"",
		"knowledge",
	)

	if !strings.Contains(prompt, "Requested output language: en") {
		t.Fatalf("expected requested English language in prompt, got %q", prompt)
	}
	if !strings.Contains(prompt, "Answer only in English.") {
		t.Fatalf("expected English-only instruction, got %q", prompt)
	}
	if strings.Contains(prompt, "Antworte als DM in natürlichem Deutsch") {
		t.Fatalf("prompt still contains forced German instruction: %q", prompt)
	}
}

func TestGMKnowledgePromptUsesRequestedGerman(t *testing.T) {
	prompt := gmKnowledgePrompt(
		Session{Language: "en"},
		GMRespondRequest{Language: "de", PlayerInput: "Wer ist Bruder Benjamin?"},
		[]string{"The Abbey (adventure)"},
		[]GMContextChunk{{DocumentName: "The Abbey", ChunkText: "Brother Benjamin guards the crypt."}},
		"",
		"knowledge",
	)

	if !strings.Contains(prompt, "Requested output language: de") {
		t.Fatalf("expected requested German language in prompt, got %q", prompt)
	}
	if !strings.Contains(prompt, "Antwort ausschließlich auf Deutsch.") {
		t.Fatalf("expected German-only instruction, got %q", prompt)
	}
	if strings.Contains(prompt, "Answer as a DM in natural English") {
		t.Fatalf("prompt still contains forced English instruction: %q", prompt)
	}
}

func TestLocalizedSessionRuntimeTexts(t *testing.T) {
	if got := localizedSessionCreatedText("en"); got != "Session created. Waiting for players and start." {
		t.Fatalf("unexpected English created text: %q", got)
	}
	if got := localizedSessionOpeningPlaceholder("en"); got != "The session begins. The AI DM opens the scene." {
		t.Fatalf("unexpected English opening placeholder: %q", got)
	}
	if got := localizedSessionReopeningPlaceholder("en"); got != "You pick the adventure back up. The AI DM gives a short recap of the situation." {
		t.Fatalf("unexpected English reopening placeholder: %q", got)
	}
	if got := localizedSessionStoppedText("de"); got != "Session pausiert oder beendet." {
		t.Fatalf("unexpected German stopped text: %q", got)
	}
}

func TestInitialLiveOpeningTextUsesRealOpeningForFreshSession(t *testing.T) {
	session := Session{
		Language: "en",
		State: SessionState{
			SceneSummary:  localizedSessionCreatedText("en"),
			LastNarration: localizedSessionCreatedText("en"),
		},
	}

	got := initialLiveOpeningText(session, false)
	want := localizedSessionOpeningPlaceholder("en")
	if got != want {
		t.Fatalf("expected fresh session opening %q, got %q", want, got)
	}
}

func TestInitialLiveOpeningTextUsesExistingNarrationForReopening(t *testing.T) {
	session := Session{
		Language: "en",
		State: SessionState{
			SceneSummary:  "The party reached the crypt entrance.",
			LastNarration: "The party reached the crypt entrance.",
		},
	}

	got := initialLiveOpeningText(session, true)
	if got != "The party reached the crypt entrance." {
		t.Fatalf("expected reopening to reuse existing narration, got %q", got)
	}
}
