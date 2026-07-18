package httpapi

import "testing"

func TestSceneAssetScorePrefersExactCue(t *testing.T) {
	exact := Asset{Type: "map", Name: "Player map", Tags: []string{"fungal_caverns_map"}, Metadata: map[string]any{"cue_key": "fungal_caverns_map"}}
	other := Asset{Type: "map", Name: "Other cave"}
	if sceneAssetScore(exact, []string{"fungal_caverns_map"}) <= sceneAssetScore(other, []string{"fungal_caverns_map"}) {
		t.Fatal("exact cue should outrank generic map fallback")
	}
}

func TestNormalizeAssetCue(t *testing.T) {
	if got := normalizeAssetCue(" Fungal Caverns: Map "); got != "fungal_caverns_map" {
		t.Fatalf("unexpected normalized cue: %q", got)
	}
}
