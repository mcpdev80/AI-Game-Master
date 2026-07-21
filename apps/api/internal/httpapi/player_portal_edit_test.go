package httpapi

import "testing"

func TestApplyPlayerPortalCharacterUpdate(t *testing.T) {
	character := Character{
		Name: "Thoras",
		Metadata: map[string]any{
			"current_hit_points":   "13",
			"temporary_hit_points": "0",
			"current_money":        "5 gp",
			"session_notes":        []string{"old note"},
			"current_inventory":    []string{"Longsword"},
		},
	}

	money := "12 gp, 4 sp"
	xp := "350"
	inspiration := "yes"
	notes := "watch the crypt door\nask Brother Benjamin"
	updated, err := applyPlayerPortalCharacterUpdate(character, UpdatePlayerPortalCharacterRequest{
		CurrentHitPoints:   testIntPtr(9),
		TemporaryHitPoints: testIntPtr(4),
		CurrentMoney:       &money,
		ExperiencePoints:   &xp,
		Inspiration:        &inspiration,
		SessionNotes:       &notes,
		CurrentInventory:   []string{"Longsword", "Torch", "Torch", " Rope "},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := updated.Metadata["current_hit_points"]; got != "9" {
		t.Fatalf("expected hp 9, got %#v", got)
	}
	if got := updated.Metadata["temporary_hit_points"]; got != "4" {
		t.Fatalf("expected temp hp 4, got %#v", got)
	}
	if got := updated.Metadata["current_money"]; got != "12 gp, 4 sp" {
		t.Fatalf("expected money update, got %#v", got)
	}
	if got := updated.Metadata["experience_points"]; got != "350" {
		t.Fatalf("expected xp update, got %#v", got)
	}
	if got := updated.Metadata["inspiration"]; got != "yes" {
		t.Fatalf("expected inspiration update, got %#v", got)
	}

	notesList, _ := updated.Metadata["session_notes"].([]string)
	if len(notesList) != 2 || notesList[0] != "watch the crypt door" || notesList[1] != "ask Brother Benjamin" {
		t.Fatalf("unexpected session notes: %#v", updated.Metadata["session_notes"])
	}
	inventory, _ := updated.Metadata["current_inventory"].([]string)
	if len(inventory) != 3 || inventory[0] != "Longsword" || inventory[1] != "Torch" || inventory[2] != "Rope" {
		t.Fatalf("unexpected inventory: %#v", updated.Metadata["current_inventory"])
	}
}

func TestApplyPlayerPortalCharacterUpdateRejectsNegativeHitPoints(t *testing.T) {
	_, err := applyPlayerPortalCharacterUpdate(Character{}, UpdatePlayerPortalCharacterRequest{
		CurrentHitPoints: testIntPtr(-1),
	})
	if err == nil {
		t.Fatal("expected error for negative hp")
	}
}

func TestApplyPlayerPortalGroupInventoryUpdate(t *testing.T) {
	notes := "shared clues"
	updated, err := applyPlayerPortalGroupInventoryUpdate(SessionGroupInventory{
		Gold:  3,
		Items: []string{"Spider Fang"},
		Notes: "old",
	}, UpdatePlayerPortalGroupInventoryRequest{
		Gold:  testIntPtr(12),
		Items: []string{"Spider Fang", "Abbey Key", "Abbey Key"},
		Notes: &notes,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Gold != 12 {
		t.Fatalf("expected gold 12, got %d", updated.Gold)
	}
	if len(updated.Items) != 2 || updated.Items[1] != "Abbey Key" {
		t.Fatalf("unexpected items: %#v", updated.Items)
	}
	if updated.Notes != "shared clues" {
		t.Fatalf("unexpected notes: %q", updated.Notes)
	}
}

func TestApplyPlayerPortalGroupInventoryUpdateRejectsNegativeGold(t *testing.T) {
	_, err := applyPlayerPortalGroupInventoryUpdate(SessionGroupInventory{}, UpdatePlayerPortalGroupInventoryRequest{
		Gold: testIntPtr(-5),
	})
	if err == nil {
		t.Fatal("expected error for negative gold")
	}
}

func testIntPtr(v int) *int {
	return &v
}
