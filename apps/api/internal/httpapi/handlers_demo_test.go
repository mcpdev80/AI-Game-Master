package httpapi

import "testing"

func TestFungalDemoCharactersAreCompleteLevelOneParty(t *testing.T) {
	characters := fungalDemoCharacters("campaign-1")
	if len(characters) != 4 {
		t.Fatalf("expected 4 demo characters, got %d", len(characters))
	}

	expectedClasses := map[string]string{
		"Seraphine Vale":  "Paladin 1",
		"Rowan Quickstep": "Rogue 1",
		"Brother Alden":   "Cleric 1",
		"Elira Moonfall":  "Wizard 1",
	}

	for _, character := range characters {
		wantClass, ok := expectedClasses[character.Name]
		if !ok {
			t.Fatalf("unexpected demo character %q", character.Name)
		}
		if character.ClassAndLevel != wantClass {
			t.Fatalf("%s class mismatch: got %q want %q", character.Name, character.ClassAndLevel, wantClass)
		}
		if character.CampaignID == nil || *character.CampaignID != "campaign-1" {
			t.Fatalf("%s missing campaign id", character.Name)
		}
		if character.ArmorClass == nil || *character.ArmorClass <= 0 {
			t.Fatalf("%s missing armor class", character.Name)
		}
		if character.HitPointMax == nil || *character.HitPointMax <= 0 {
			t.Fatalf("%s missing hit points", character.Name)
		}
		if len(character.Abilities) != 6 {
			t.Fatalf("%s missing ability spread: %#v", character.Name, character.Abilities)
		}
		if len(character.Languages) == 0 || len(character.Features) == 0 {
			t.Fatalf("%s missing languages or features", character.Name)
		}
		if metadataString(character.Metadata, "demo_id") != fungalCavernsDemoID {
			t.Fatalf("%s missing demo id", character.Name)
		}

		requiredStringKeys := []string{
			"current_hit_points",
			"current_money",
			"concept",
			"backstory",
			"personality_traits",
			"ideals",
			"bonds",
			"flaws",
			"combat_attacks",
			"combat_overview",
			"hit_dice",
			"passive_perception",
		}
		for _, key := range requiredStringKeys {
			if metadataString(character.Metadata, key) == "" {
				t.Fatalf("%s missing metadata key %q", character.Name, key)
			}
		}

		requiredListKeys := []string{
			"skill_proficiencies",
			"saving_throw_proficiencies",
			"starting_equipment",
			"current_inventory",
			"weapon_notes",
		}
		for _, key := range requiredListKeys {
			value, ok := character.Metadata[key].([]string)
			if !ok || len(value) == 0 {
				t.Fatalf("%s missing list metadata key %q: %#v", character.Name, key, character.Metadata[key])
			}
		}
	}
}

func TestBundledDemoCharactersAreStandaloneAndComplete(t *testing.T) {
	characters := bundledDemoCharacters()
	if len(characters) != 4 {
		t.Fatalf("expected 4 bundled demo characters, got %d", len(characters))
	}
	for _, character := range characters {
		if character.CampaignID != nil {
			t.Fatalf("%s should not require a campaign id", character.Name)
		}
		if character.Name == "" || character.ClassAndLevel == "" {
			t.Fatalf("incomplete bundled demo character: %#v", character)
		}
		if metadataString(character.Metadata, "demo_id") != fungalCavernsDemoID {
			t.Fatalf("%s missing demo id", character.Name)
		}
	}
}
