package httpapi

import (
	"strings"
	"testing"
)

func TestBuilderLevelUpPreviewForFighterFiveUsesSRDProgression(t *testing.T) {
	preview, ok := builderLevelUpPreview(Character{
		ClassAndLevel: "Kämpfer, Stufe 4",
		Abilities: map[string]int{
			"constitution": 14,
		},
	}, "Kämpfer", 5)
	if !ok {
		t.Fatal("expected fighter level-up preview")
	}
	if preview.ClassName != "Kämpfer" || preview.CurrentLevel != 4 || preview.TargetLevel != 5 {
		t.Fatalf("unexpected preview levels: %+v", preview)
	}
	if preview.AverageHPGain != 8 {
		t.Fatalf("expected d10 average 6 plus con 2 = 8, got %d", preview.AverageHPGain)
	}
	if preview.ProficiencyBonus != 3 {
		t.Fatalf("expected proficiency bonus +3 at level 5, got %d", preview.ProficiencyBonus)
	}
	if !containsStringFold(preview.FeaturesGained, "Extra Attack") {
		t.Fatalf("expected Extra Attack in features: %#v", preview.FeaturesGained)
	}
}

func TestBuilderLevelUpPreviewForPaladinTwoStartsSpellcasting(t *testing.T) {
	preview, ok := builderLevelUpPreview(Character{
		ClassAndLevel: "Paladin, Stufe 1",
		Abilities: map[string]int{
			"charisma":     16,
			"constitution": 14,
		},
	}, "Paladin", 2)
	if !ok {
		t.Fatal("expected paladin level-up preview")
	}
	if got := builderSpellSlotSummary(preview.Spellcasting); got != "slots L1:2" {
		t.Fatalf("expected paladin level 2 slots, got %q", got)
	}
	if preview.Spellcasting.PreparedCount != 4 {
		t.Fatalf("expected paladin prepared count 4 at level 2 with CHA 16, got %d", preview.Spellcasting.PreparedCount)
	}
	if !containsStringFold(preview.FeaturesGained, "Zauberwirken") || !containsStringFold(preview.FeaturesGained, "Göttliches Niederstrecken") {
		t.Fatalf("expected spellcasting and smite at paladin 2: %#v", preview.FeaturesGained)
	}
}

func TestBuilderLevelUpPreviewForWizardFiveTracksSpellbookAndSlots(t *testing.T) {
	preview, ok := builderLevelUpPreview(Character{
		ClassAndLevel: "Magier, Stufe 4",
		Abilities: map[string]int{
			"intelligence": 16,
			"constitution": 12,
		},
	}, "Magier", 5)
	if !ok {
		t.Fatal("expected wizard level-up preview")
	}
	if preview.Spellcasting.MaxSpellLevel != 3 {
		t.Fatalf("expected max spell level 3 at wizard 5, got %d", preview.Spellcasting.MaxSpellLevel)
	}
	if got := builderSpellSlotSummary(preview.Spellcasting); got != "slots L1:4, L2:3, L3:2" {
		t.Fatalf("unexpected slot summary: %q", got)
	}
	if preview.Spellcasting.SpellbookTotal != 14 {
		t.Fatalf("expected 14 spellbook entries at wizard 5, got %d", preview.Spellcasting.SpellbookTotal)
	}
	if preview.Spellcasting.PreparedCount != 8 {
		t.Fatalf("expected 8 prepared spells at wizard 5 with INT 16, got %d", preview.Spellcasting.PreparedCount)
	}
}

func TestApplyAverageLevelUpPatchAdvancesRogueAndDerivedFields(t *testing.T) {
	patch, err := applyAverageLevelUpPatch(Character{
		ClassAndLevel: "Schurke, Stufe 4",
		HitPointMax:   intPtr(27),
		Abilities: map[string]int{
			"strength":     10,
			"dexterity":    16,
			"constitution": 14,
			"intelligence": 12,
			"wisdom":       10,
			"charisma":     8,
		},
		Features: []string{"Expertise", "Sneak Attack (2d6)"},
		Metadata: map[string]any{
			"starting_equipment": []string{"Rapier", "Kurzbogen", "Lederrüstung"},
			"level_up_available": "true",
		},
	}, "Schurke", 5)
	if err != nil {
		t.Fatalf("expected average level-up patch, got error: %v", err)
	}
	if patch.ClassAndLevel == nil || *patch.ClassAndLevel != "Schurke, Stufe 5" {
		t.Fatalf("unexpected class level patch: %#v", patch.ClassAndLevel)
	}
	if patch.HitPointMax == nil || *patch.HitPointMax != 34 {
		t.Fatalf("expected hp 34 after +7 average gain, got %#v", patch.HitPointMax)
	}
	if patch.Proficiency == nil || *patch.Proficiency != "+3" {
		t.Fatalf("expected proficiency +3, got %#v", patch.Proficiency)
	}
	if patch.Metadata["level_up_available"] != "false" {
		t.Fatalf("expected level-up flag to reset, got %#v", patch.Metadata["level_up_available"])
	}
	if !containsStringFold(patch.Features, "Uncanny Dodge") || !containsStringFold(patch.Features, "Sneak Attack (3d6)") {
		t.Fatalf("expected rogue 5 features in patch: %#v", patch.Features)
	}
	if patch.ArmorClass == nil || *patch.ArmorClass != 14 {
		t.Fatalf("expected leather armor AC 14 with DEX 16, got %#v", patch.ArmorClass)
	}
}

func TestBuilderSpellAdviceForCharacterAtLevelUsesSRDSpellProgression(t *testing.T) {
	advice, ok := builderSpellAdviceForCharacterAtLevel(Character{
		ClassAndLevel: "Hexenmeister, Stufe 10",
		Abilities: map[string]int{
			"charisma": 18,
		},
	}, "Hexenmeister", 11)
	if !ok {
		t.Fatal("expected warlock spell advice at higher level")
	}
	if !strings.Contains(advice.SpellNotes, "3 pact slot(s) at level 5") {
		t.Fatalf("expected pact slot summary in notes, got %q", advice.SpellNotes)
	}
	if advice.SpellSaveDC != "16" || advice.SpellAttackBonus != "+8" {
		t.Fatalf("unexpected higher-level spell numbers: dc=%q attack=%q", advice.SpellSaveDC, advice.SpellAttackBonus)
	}
	if len(advice.Recommendation) == 0 {
		t.Fatal("expected spell recommendations")
	}
	if !strings.Contains(advice.SpellAttackRows, "Zaubertrick |") {
		t.Fatalf("expected structured spell attack rows, got %q", advice.SpellAttackRows)
	}
}

func TestApplyGuidedLevelUpPatchStoresStructuredSpellAttackRows(t *testing.T) {
	patch, err := applyGuidedLevelUpPatch(Character{
		ClassAndLevel: "Paladin 3",
		Abilities: map[string]int{
			"constitution": 14,
			"charisma":     16,
			"intelligence": 16,
		},
	}, "Zauberer", 1, "average", 0)
	if err != nil {
		t.Fatalf("expected guided level-up patch, got error: %v", err)
	}
	rows, ok := patch.Metadata["spell_attacks"].(string)
	if !ok {
		t.Fatalf("expected spell_attacks string, got %#v", patch.Metadata["spell_attacks"])
	}
	for _, expected := range []string{"Zaubertrick |", "Grad 1 |"} {
		if !strings.Contains(rows, expected) {
			t.Fatalf("expected structured spell attack rows to contain %q, got %q", expected, rows)
		}
	}
	if !strings.Contains(rows, "Beschreibung:") {
		t.Fatalf("expected spell descriptions in rows, got %q", rows)
	}
}

func TestBuilderLevelUpSummaryUsesSRDPreview(t *testing.T) {
	summary := builderLevelUpSummary(Character{
		ClassAndLevel: "Paladin, Stufe 1",
		Abilities: map[string]int{
			"charisma":     16,
			"constitution": 14,
		},
		Metadata: map[string]any{
			"experience_points": "300",
		},
	})
	if summary == "" {
		t.Fatal("expected non-empty level-up summary")
	}
	if !strings.Contains(summary, "Paladin 1 → 2") && !strings.Contains(summary, "Paladin 1 -> 2") {
		t.Fatalf("unexpected summary: %q", summary)
	}
	if !strings.Contains(summary, "slots L1:2") {
		t.Fatalf("expected slot summary in preview: %q", summary)
	}
}

func TestParseCharacterLevelSumsMulticlassLevels(t *testing.T) {
	if got := parseCharacterLevel("Kämpfer 3 / Magier 2"); got != 5 {
		t.Fatalf("expected multiclass total level 5, got %d", got)
	}
}

func TestBuilderLevelUpPreviewForMulticlassUsesTargetClassAndTotalLevel(t *testing.T) {
	preview, ok := builderLevelUpPreview(Character{
		ClassAndLevel: "Kämpfer 3 / Magier 2",
		Abilities: map[string]int{
			"constitution": 14,
			"intelligence": 16,
		},
	}, "Magier", 3)
	if !ok {
		t.Fatal("expected multiclass preview")
	}
	if preview.TargetClass != "Magier" || preview.CurrentLevel != 2 || preview.TargetLevel != 3 {
		t.Fatalf("unexpected multiclass target preview: %+v", preview)
	}
	if preview.TotalLevel != 6 {
		t.Fatalf("expected total level 6 after multiclass level-up, got %d", preview.TotalLevel)
	}
	if preview.ProficiencyBonus != 3 {
		t.Fatalf("expected proficiency +3 at total level 6, got %d", preview.ProficiencyBonus)
	}
	if got := builderSpellSlotSummary(preview.Spellcasting); got != "slots L1:4, L2:2" {
		t.Fatalf("expected wizard multiclass slots from caster level 3, got %q", got)
	}
}

func TestBuilderLevelUpPreviewAllowsAddingNewMulticlassAtLevelOne(t *testing.T) {
	preview, ok := builderLevelUpPreview(Character{
		ClassAndLevel: "Paladin 3",
		Abilities: map[string]int{
			"constitution": 14,
			"charisma":     16,
		},
	}, "Zauberer", 1)
	if !ok {
		t.Fatal("expected multiclass preview for adding first wizard level")
	}
	if preview.CurrentLevel != 0 || preview.TargetLevel != 1 {
		t.Fatalf("unexpected new multiclass preview levels: %+v", preview)
	}
	if preview.TotalLevel != 4 {
		t.Fatalf("expected total level 4 after adding wizard 1, got %d", preview.TotalLevel)
	}
	if preview.AverageHPGain != 6 {
		t.Fatalf("expected wizard average hit gain 4 plus CON 2, got %d", preview.AverageHPGain)
	}
}

func TestBuilderClassAndLevelTextAppendsNewMulticlassToSingleClassDraft(t *testing.T) {
	got := builderClassAndLevelText("Zauberer", "Paladin 3", 1)
	if got != "Paladin 3 / Zauberer 1" {
		t.Fatalf("expected multiclass append text, got %q", got)
	}
}
