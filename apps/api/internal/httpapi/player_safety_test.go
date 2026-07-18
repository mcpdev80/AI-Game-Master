package httpapi

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSanitizeSessionForPlayersRemovesPrivateGMState(t *testing.T) {
	session := Session{State: SessionState{
		LastDMNotes:       []string{"secret door behind room four"},
		PlayLLMSessionID:  "play-private",
		RulesLLMSessionID: "rules-private",
		SummarySessionID:  "summary-private",
		VisualPayload:     map[string]any{"hide_dc": true, "roll_dc": 17, "roll_label": "Perception"},
	}}

	sanitized := sanitizeSessionForPlayers(session)
	encoded, err := json.Marshal(sanitized)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, secret := range []string{"last_dm_notes", "secret door", "play-private", "rules-private", "summary-private", "roll_dc"} {
		if strings.Contains(text, secret) {
			t.Fatalf("player-safe session leaked %q: %s", secret, text)
		}
	}
	if !strings.Contains(text, "roll_label") {
		t.Fatalf("public roll context was removed: %s", text)
	}
}
