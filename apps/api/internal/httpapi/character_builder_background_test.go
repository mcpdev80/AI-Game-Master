package httpapi

import (
	"strings"
	"testing"
)

func TestBuilderBackgroundReplyOffersOfficialCustomBackgroundRule(t *testing.T) {
	reply, ok := builderDeterministicBackgroundReply(Character{}, "Welche Hintergründe stehen mir zur Verfügung?")
	if !ok {
		t.Fatal("expected deterministic background guidance")
	}
	for _, expected := range []string{"Akolyth", "eigenen Hintergrund", "zwei Fertigkeiten", "Sprachen oder Werkzeugübungen"} {
		if !strings.Contains(reply, expected) {
			t.Fatalf("reply does not contain %q: %s", expected, reply)
		}
	}
}

func TestBuilderBackgroundReplyOffersSkillListAndKnightSuggestion(t *testing.T) {
	reply, ok := builderDeterministicBackgroundReply(Character{}, "Eigener Hintergrund Ritter, aber welche Fertigkeiten kann ich wählen und was passt am besten?")
	if !ok {
		t.Fatal("expected deterministic background guidance for knight")
	}
	for _, expected := range []string{"Athletik", "Einschüchtern", "Geschichte", "Überzeugen", "Mit Tieren umgehen"} {
		if !strings.Contains(reply, expected) {
			t.Fatalf("reply does not contain %q: %s", expected, reply)
		}
	}
}

func TestBuilderClassStageAdviceOffersKnightSuggestions(t *testing.T) {
	reply, ok := builderDeterministicStageAdviceReply(Character{}, "class_and_level", "Ich möchte etwas Ritterliches, welche Klassen passen am besten?")
	if !ok {
		t.Fatal("expected deterministic class advice")
	}
	for _, expected := range []string{"Kämpfer, Stufe 1", "Paladin, Stufe 1", "Waldläufer, Stufe 1"} {
		if !strings.Contains(reply, expected) {
			t.Fatalf("reply does not contain %q: %s", expected, reply)
		}
	}
}

func TestBuilderClassChoicesReplyOffersSuggestedSkillSets(t *testing.T) {
	character := Character{
		ClassAndLevel: "Kämpfer 1",
		Background:    "Ritter",
	}
	reply, ok := builderDeterministicClassChoicesReply(character)
	if !ok {
		t.Fatal("expected deterministic class choices reply")
	}
	for _, expected := range []string{"Athletik", "Wahrnehmung", "Einschüchtern"} {
		if !strings.Contains(reply, expected) {
			t.Fatalf("reply does not contain %q: %s", expected, reply)
		}
	}
}

func TestBackgroundStageAllowsConciseCustomBackgroundName(t *testing.T) {
	background := "Waldhüter des Grenzlands"
	patch := CharacterBuilderPatch{Background: &background}
	sanitizeCharacterBuilderPatchForStage(&patch, "background_and_alignment")
	if patch.Background == nil || *patch.Background != background {
		t.Fatalf("custom SRD background name was rejected: %#v", patch.Background)
	}

	tooLong := strings.Repeat("x", 97)
	patch = CharacterBuilderPatch{Background: &tooLong}
	sanitizeCharacterBuilderPatchForStage(&patch, "background_and_alignment")
	if patch.Background != nil {
		t.Fatalf("oversized background value was accepted: %q", *patch.Background)
	}
}
