package httpapi

import "testing"

func TestCurrentBuilderStagePrefersActiveLevelUpFlow(t *testing.T) {
	stage := currentBuilderStage(Character{
		Metadata: map[string]any{
			"builder_stage":        "review",
			"level_up_in_progress": "true",
		},
	}, nil)
	if stage != "level_up" {
		t.Fatalf("expected active level-up stage, got %q", stage)
	}
}

func TestBuilderDeterministicLevelUpCompletionStartsFlow(t *testing.T) {
	completion, ok := builderDeterministicLevelUpCompletion(&Character{
		ClassAndLevel: "Kämpfer, Stufe 1",
		Abilities: map[string]int{
			"constitution": 14,
		},
		Metadata: map[string]any{
			"experience_points": "300",
		},
	}, "review", "level up")
	if !ok {
		t.Fatal("expected level-up flow to start")
	}
	if completion.Patch.Metadata["level_up_in_progress"] != "true" {
		t.Fatalf("expected in-progress flag, got %#v", completion.Patch.Metadata)
	}
	if completion.Patch.Metadata["level_up_step"] != "hp_choice" {
		t.Fatalf("expected hp choice step, got %#v", completion.Patch.Metadata["level_up_step"])
	}
}

func TestBuilderDeterministicLevelUpCompletionAppliesAverageWithoutChoices(t *testing.T) {
	completion, ok := builderDeterministicLevelUpCompletion(&Character{
		ClassAndLevel: "Kämpfer, Stufe 1",
		HitPointMax:   intPtr(12),
		Abilities: map[string]int{
			"strength":     16,
			"dexterity":    12,
			"constitution": 14,
		},
		Metadata: map[string]any{
			"level_up_in_progress":     "true",
			"level_up_step":            "hp_choice",
			"level_up_target_level":    2,
			"level_up_pending_choices": []string{},
			"starting_equipment":       []string{"Kettenhemd", "Schild"},
		},
	}, "level_up", "average hp")
	if !ok {
		t.Fatal("expected average hp application")
	}
	if completion.Patch.ClassAndLevel == nil || *completion.Patch.ClassAndLevel != "Kämpfer, Stufe 2" {
		t.Fatalf("unexpected level patch: %#v", completion.Patch.ClassAndLevel)
	}
	if completion.Patch.HitPointMax == nil || *completion.Patch.HitPointMax != 20 {
		t.Fatalf("expected hp 20 after +8 gain, got %#v", completion.Patch.HitPointMax)
	}
	if completion.Patch.Metadata["level_up_in_progress"] != "false" {
		t.Fatalf("expected flow to finish, got %#v", completion.Patch.Metadata["level_up_in_progress"])
	}
}

func TestBuilderDeterministicLevelUpCompletionCollectsChoiceAnswers(t *testing.T) {
	completion, ok := builderDeterministicLevelUpCompletion(&Character{
		ClassAndLevel: "Paladin, Stufe 2",
		HitPointMax:   intPtr(20),
		Abilities: map[string]int{
			"strength":     16,
			"charisma":     16,
			"constitution": 14,
		},
		Metadata: map[string]any{
			"level_up_in_progress":  "true",
			"level_up_step":         "choice_prompt",
			"level_up_target_level": 3,
			"level_up_hp_mode":      "average",
			"level_up_pending_choices": []string{
				"Choose a Sacred Oath.",
			},
			"level_up_choice_index": 0,
		},
	}, "level_up", "Oath of Devotion")
	if !ok {
		t.Fatal("expected choice prompt handling")
	}
	answers := stringListFromAny(completion.Patch.Metadata["level_up_choice_answers"])
	if len(answers) != 0 {
		t.Fatalf("expected final patch to reset recorded choice answers, got %#v", answers)
	}
	if completion.Patch.Metadata["level_up_in_progress"] != "false" {
		t.Fatalf("expected flow to complete after final choice, got %#v", completion.Patch.Metadata["level_up_in_progress"])
	}
}

func TestBuilderDeterministicLevelUpCompletionStartsWithClassChoiceForMulticlass(t *testing.T) {
	completion, ok := builderDeterministicLevelUpCompletion(&Character{
		ClassAndLevel: "Kämpfer 3 / Magier 2",
		Abilities: map[string]int{
			"constitution": 14,
			"intelligence": 16,
		},
		Metadata: map[string]any{
			"experience_points": "14000",
		},
	}, "review", "level up")
	if !ok {
		t.Fatal("expected multiclass level-up flow to start")
	}
	if completion.Patch.Metadata["level_up_step"] != "class_choice" {
		t.Fatalf("expected class choice step, got %#v", completion.Patch.Metadata["level_up_step"])
	}
}

func TestBuilderDeterministicLevelUpCompletionAcceptsMulticlassTargetClass(t *testing.T) {
	completion, ok := builderDeterministicLevelUpCompletion(&Character{
		ClassAndLevel: "Kämpfer 3 / Magier 2",
		Abilities: map[string]int{
			"constitution": 14,
			"intelligence": 16,
		},
		Metadata: map[string]any{
			"level_up_in_progress": "true",
			"level_up_step":        "class_choice",
		},
	}, "level_up", "Magier")
	if !ok {
		t.Fatal("expected multiclass class choice to be accepted")
	}
	if completion.Patch.Metadata["level_up_target_class"] != "Magier" {
		t.Fatalf("expected target class Magier, got %#v", completion.Patch.Metadata["level_up_target_class"])
	}
	if completion.Patch.Metadata["level_up_step"] != "hp_choice" {
		t.Fatalf("expected hp choice next, got %#v", completion.Patch.Metadata["level_up_step"])
	}
}

func TestPrepareInitialMulticlassBuilderPatchReducesDraftToStartClassLevelOne(t *testing.T) {
	initial := "Paladin 3 / Zauberer 3"
	patch := CharacterBuilderPatch{
		ClassAndLevel: &initial,
		Metadata: map[string]any{
			"builder_stage": "background_and_alignment",
		},
	}
	prepareInitialMulticlassBuilderPatch(&patch, "class_and_level")
	if patch.ClassAndLevel == nil || *patch.ClassAndLevel != "Paladin 1" {
		t.Fatalf("expected initial draft to start at Paladin 1, got %#v", patch.ClassAndLevel)
	}
	sequence := stringListFromAny(patch.Metadata["builder_planned_level_sequence"])
	expected := []string{"Paladin", "Paladin", "Zauberer", "Zauberer", "Zauberer"}
	if len(sequence) != len(expected) {
		t.Fatalf("expected planned sequence %#v, got %#v", expected, sequence)
	}
	for index, value := range expected {
		if sequence[index] != value {
			t.Fatalf("expected planned sequence %#v, got %#v", expected, sequence)
		}
	}
}

func TestBuilderDeterministicLevelUpCompletionAutoContinuesPlannedMulticlassSequence(t *testing.T) {
	completion, ok := builderDeterministicLevelUpCompletion(&Character{
		ClassAndLevel: "Paladin 3",
		Abilities: map[string]int{
			"constitution": 14,
			"charisma":     16,
		},
		Metadata: map[string]any{
			"builder_stage":                  "review",
			"builder_planned_level_sequence": []string{"Zauberer"},
		},
	}, "review", "weiter")
	if !ok {
		t.Fatal("expected planned multiclass sequence to start automatically from review")
	}
	if completion.Patch.Metadata["level_up_target_class"] != "Zauberer" {
		t.Fatalf("expected planned target class Zauberer, got %#v", completion.Patch.Metadata["level_up_target_class"])
	}
	if completion.Patch.Metadata["level_up_target_level"] != 1 {
		t.Fatalf("expected first multiclass target level 1, got %#v", completion.Patch.Metadata["level_up_target_level"])
	}
	if completion.Patch.Metadata["level_up_step"] != "hp_choice" {
		t.Fatalf("expected hp choice next, got %#v", completion.Patch.Metadata["level_up_step"])
	}
}

func TestSanitizeCharacterBuilderPatchForStageReviewKeepsRepairFields(t *testing.T) {
	speed := "30 Fuß"
	patch := CharacterBuilderPatch{
		HitPointMax: intPtr(52),
		Speed:       &speed,
		Metadata: map[string]any{
			"hit_dice":      "3W10 + 3W6",
			"combat_attacks": "Langschwert | +5 | 1W8+3",
		},
	}
	sanitizeCharacterBuilderPatchForStage(&patch, "review")
	if patch.HitPointMax == nil || *patch.HitPointMax != 52 {
		t.Fatalf("expected review sanitizer to keep hit points, got %#v", patch.HitPointMax)
	}
	if patch.Speed == nil || *patch.Speed != "30 Fuß" {
		t.Fatalf("expected review sanitizer to keep speed, got %#v", patch.Speed)
	}
	if patch.Metadata["hit_dice"] != "3W10 + 3W6" {
		t.Fatalf("expected hit dice metadata to survive review sanitize, got %#v", patch.Metadata["hit_dice"])
	}
	if patch.Metadata["combat_attacks"] == nil {
		t.Fatalf("expected combat attacks metadata to survive review sanitize, got %#v", patch.Metadata)
	}
}
