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
