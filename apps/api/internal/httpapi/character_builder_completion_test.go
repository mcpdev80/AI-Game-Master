package httpapi

import (
	"strings"
	"testing"
)

func TestBuilderDerivedSensesIncludesDarkvisionAndPassivePerception(t *testing.T) {
	character := Character{
		Race:          "High Elf",
		ClassAndLevel: "Wizard 1",
		Abilities: map[string]int{
			"wisdom": 12,
		},
		Metadata: map[string]any{
			"language":            "en",
			"skill_proficiencies": []string{"Perception"},
			"passive_perception":  "",
		},
		Proficiency: "+2",
	}

	got := builderDerivedSenses(character)
	if !strings.Contains(got, "Darkvision") {
		t.Fatalf("expected darkvision in senses, got %q", got)
	}
	if !strings.Contains(got, "Passive Perception") {
		t.Fatalf("expected passive perception in senses, got %q", got)
	}
}

func TestBuilderMissingRequiredFieldsReportsIncompleteDraft(t *testing.T) {
	character := Character{
		Name:          "Incomplete",
		Race:          "Human",
		ClassAndLevel: "Paladin 1",
		Background:    "Acolyte",
		Alignment:     "Lawful Good",
		ArmorClass:    demoIntPtr(18),
		Speed:         "30 ft",
		HitPointMax:   demoIntPtr(12),
		Proficiency:   "+2",
		Abilities: map[string]int{
			"strength": 16, "dexterity": 10, "constitution": 14,
			"intelligence": 10, "wisdom": 12, "charisma": 14,
		},
		Languages: []string{"Common"},
		Features:  []string{"Lay on Hands", "Divine Sense"},
		Metadata: map[string]any{
			"language":                   "en",
			"concept":                    "Frontline holy warrior",
			"creation_method":            "standard_array",
			"skill_proficiencies":        []string{"Athletics", "Insight"},
			"saving_throw_proficiencies": []string{"Wisdom", "Charisma"},
			"hit_dice":                   "1d10",
			"senses":                     "Passive Perception 11",
			"age":                        "24",
			"size":                       "Medium",
			"weight":                     "180 lb",
			"eyes":                       "Gray",
			"skin":                       "Fair",
			"hair":                       "Black",
			"personality_traits":         "Calm",
			"ideals":                     "Duty",
			"bonds":                      "Abbey vows",
			"starting_equipment":         []string{"Chain mail"},
			"current_inventory":          []string{"Chain mail", "Shield"},
			"current_money":              "15 gp",
			"combat_attacks":             "Longsword | +2 | STR | Melee | +5 | 1d8+3 | Slashing",
		},
	}

	missing := builderMissingRequiredFields(character)
	if len(missing) == 0 {
		t.Fatal("expected incomplete draft to report missing fields")
	}
	if !containsStringFold(missing, "Makel") {
		t.Fatalf("expected flaws to be missing, got %#v", missing)
	}
}

func TestBuilderDeterministicSensesAndBodyReplySetsDerivedSensesImmediately(t *testing.T) {
	character := Character{
		Race:          "High Elf",
		ClassAndLevel: "Wizard 1",
		Abilities: map[string]int{
			"wisdom": 12,
		},
		Proficiency: "+2",
		Metadata: map[string]any{
			"language":            "en",
			"skill_proficiencies": []string{"Perception"},
			"age":                 "120",
			"size":                "Medium",
			"weight":              "150 lb",
			"eyes":                "Green",
			"skin":                "Fair",
			"hair":                "Silver",
		},
	}

	reply, patch, ok := builderDeterministicSensesAndBodyReply(character, "continue")
	if !ok {
		t.Fatal("expected deterministic reply")
	}
	got := strings.TrimSpace(safeOptionalString(patch.Metadata["senses"]))
	if !strings.Contains(got, "Darkvision 60 ft") {
		t.Fatalf("expected derived darkvision in patch, got %q", got)
	}
	if !strings.Contains(got, "Passive Perception 13") {
		t.Fatalf("expected derived passive perception in patch, got %q", got)
	}
	if !strings.Contains(reply, "Darkvision 60 ft") {
		t.Fatalf("expected reply to mention derived senses, got %q", reply)
	}
}
