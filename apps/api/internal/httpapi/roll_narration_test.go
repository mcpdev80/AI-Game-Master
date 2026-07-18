package httpapi

import (
	"strings"
	"testing"
)

func TestBuildRollRequestNarrationUsesEnglish(t *testing.T) {
	dc := 13
	narration := buildRollRequestNarration("en", &RollRequest{
		Dice:   []string{"1d20"},
		DC:     &dc,
		HideDC: false,
	})
	if narration != "A roll is required. Roll 1d20. The difficulty is 13." {
		t.Fatalf("unexpected narration: %q", narration)
	}
	if strings.Contains(narration, "Würfle") || strings.Contains(narration, "Schwierigkeit") {
		t.Fatalf("English narration contains German text: %q", narration)
	}
}

func TestBuildRollRequestNarrationDoesNotRepeatModelInstructions(t *testing.T) {
	narration := buildRollRequestNarration("en", &RollRequest{
		Label:        "Listen at the door",
		Dice:         []string{"1d20"},
		Instructions: "Roll 1d20 and add Perception.",
	})
	if strings.Count(narration, "Roll 1d20") != 1 {
		t.Fatalf("roll instruction was repeated: %q", narration)
	}
}

func TestBuildRollRequestNarrationKeepsGerman(t *testing.T) {
	narration := buildRollRequestNarration("de-DE", &RollRequest{Dice: []string{"1d20"}})
	if narration != "Eine Probe ist fällig. Würfle 1d20." {
		t.Fatalf("unexpected narration: %q", narration)
	}
}
