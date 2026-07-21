package httpapi

import (
	"context"
	"strings"
	"testing"
)

func TestInitialBuilderCharacterNameUsesProvidedName(t *testing.T) {
	got := initialBuilderCharacterName("Thoras", "de")
	if got != "Thoras" {
		t.Fatalf("expected provided name to be kept, got %q", got)
	}
}

func TestInitialBuilderCharacterNameFallsBackByLanguage(t *testing.T) {
	if got := initialBuilderCharacterName("", "de"); got != "Neuer Charakter" {
		t.Fatalf("expected German fallback, got %q", got)
	}
	if got := initialBuilderCharacterName("", "en"); got != "New Character" {
		t.Fatalf("expected English fallback, got %q", got)
	}
}

func TestInferBackgroundSkillsFromLegacyCombinedSkills(t *testing.T) {
	got := inferBackgroundSkills(
		nil,
		[]string{"Motiv erkennen", "Überzeugen"},
		[]string{"Athletik", "Wahrnehmung", "Motiv erkennen", "Überzeugen"},
	)
	if len(got) != 2 || got[0] != "Athletik" || got[1] != "Wahrnehmung" {
		t.Fatalf("expected inferred background skills to be preserved, got %#v", got)
	}
}

func TestBuilderMissingBodyFieldLabelsTreatsNilAsMissing(t *testing.T) {
	got := builderMissingBodyFieldLabels(map[string]any{
		"age":    nil,
		"size":   "Mittel",
		"eyes":   "<nil>",
		"hair":   "",
		"skin":   "Bronze",
		"weight": nil,
	})
	if len(got) != 4 {
		t.Fatalf("expected 4 missing labels, got %#v", got)
	}
}

func TestBuilderDeterministicSensesAndBodyReplyDoesNotSurfaceNilSenses(t *testing.T) {
	reply, patch, ok := builderDeterministicSensesAndBodyReply(Character{
		Race: "Mensch",
		Languages: []string{
			"Gemeinsprache",
		},
		Metadata: map[string]any{
			"size":   "Mittel",
			"age":    nil,
			"eyes":   nil,
			"hair":   nil,
			"skin":   nil,
			"weight": nil,
			"senses": nil,
		},
	}, "ok")
	if !ok {
		t.Fatal("expected deterministic body reply")
	}
	if reply == "" {
		t.Fatal("expected non-empty reply")
	}
	if patch.Metadata["builder_stage"] != "languages_senses_and_body" {
		t.Fatalf("expected builder stage to remain on body step, got %#v", patch.Metadata["builder_stage"])
	}
	if strings.Contains(reply, "<nil>") {
		t.Fatalf("reply must not contain <nil>: %q", reply)
	}
}

func TestInferBuilderStageFromCharacterStaysOnBodyStepWhileFieldsMissing(t *testing.T) {
	stage := inferBuilderStageFromCharacter(Character{
		Race:          "Drachenblütiger",
		ClassAndLevel: "Paladin, Stufe 1",
		Background:    "Ritter",
		Alignment:     "rechtschaffen gut",
		Speed:         "30 Fuß",
		HitPointMax:   intPtr(13),
		Abilities: map[string]int{
			"Stärke": 16,
		},
		Features: []string{"Divine Sense"},
		Metadata: map[string]any{
			"concept":                    "Rachepaladin",
			"creation_method":            "standard array",
			"hit_dice":                   "1W10",
			"skill_proficiencies":        []string{"Athletik", "Wahrnehmung", "Motiv erkennen", "Überzeugen"},
			"saving_throw_proficiencies": []string{"Weisheit", "Charisma"},
			"age":                        "28",
			"size":                       "Mittel",
			"senses":                     "normale Sicht",
		},
	})
	if stage != "languages_senses_and_body" {
		t.Fatalf("expected body step to remain active, got %q", stage)
	}
}

func TestBuilderSRDSpellCatalogContainsWizardCoreSpells(t *testing.T) {
	for _, name := range []string{"Fire Bolt", "Magic Missile", "Shield", "Mage Armor"} {
		entry, ok := builderSRDSpellCatalogEntryByName(name)
		if !ok {
			t.Fatalf("expected %q in SRD spell catalog", name)
		}
		if entry.Name != name {
			t.Fatalf("unexpected catalog entry for %q: %#v", name, entry)
		}
	}
}

func TestBuilderSRDSpellCatalogUsesEnglishCanonicalClassIDs(t *testing.T) {
	entry, ok := builderSRDSpellCatalogEntryByName("Magic Missile")
	if !ok {
		t.Fatal("expected Magic Missile in SRD spell catalog")
	}
	foundWizard := false
	for _, classID := range entry.Classes {
		if classID == "wizard" {
			foundWizard = true
		}
		if classID == "Magier" || classID == "Zauberer" || classID == "Hexenmeister" {
			t.Fatalf("expected canonical English class IDs only, got %#v", entry.Classes)
		}
	}
	if !foundWizard {
		t.Fatalf("expected wizard class id in %#v", entry.Classes)
	}
}

func TestBuilderSpellAdviceUsesStructuredCatalog(t *testing.T) {
	advice, ok := builderSpellAdviceForCharacter(Character{
		ClassAndLevel: "Magier, Stufe 1",
		Abilities: map[string]int{
			"intelligence": 16,
		},
	})
	if !ok {
		t.Fatal("expected spell advice for wizard")
	}
	if len(advice.Options) < 2 {
		t.Fatalf("expected grouped spell options, got %#v", advice.Options)
	}
	if len(advice.Recommendation) < 6 {
		t.Fatalf("expected structured recommendations, got %#v", advice.Recommendation)
	}
	if advice.SpellSaveDC != "13" || advice.SpellAttackBonus != "+5" {
		t.Fatalf("unexpected spell numbers: dc=%q attack=%q", advice.SpellSaveDC, advice.SpellAttackBonus)
	}
}

func TestBuilderSpellAdviceUsesGermanCharacterClassAgainstEnglishCatalog(t *testing.T) {
	advice, ok := builderSpellAdviceForCharacter(Character{
		ClassAndLevel: "Hexenmeister, Stufe 1",
		Abilities: map[string]int{
			"charisma": 16,
		},
	})
	if !ok {
		t.Fatal("expected spell advice for German warlock label")
	}
	if len(advice.Recommendation) == 0 {
		t.Fatal("expected spell recommendations")
	}
	found := false
	for _, spell := range advice.Recommendation {
		if spell == "Schauriger Strahl" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected warlock recommendations from English-backed catalog, got %#v", advice.Recommendation)
	}
}

func TestBuilderClassRuleForCharacterUsesPrimaryMulticlassClass(t *testing.T) {
	rule, ok := builderClassRuleForCharacter(Character{ClassAndLevel: "Paladin 3 / Zauberer 3"})
	if !ok {
		t.Fatal("expected class rule for multiclass character")
	}
	if rule.ClassName != "Paladin" || rule.HitDie != "W10" {
		t.Fatalf("expected primary class paladin with W10, got %+v", rule)
	}
}

func TestBuilderDerivedHitPointsAndDiceForMulticlassUseAllClassLevels(t *testing.T) {
	character := Character{
		ClassAndLevel: "Paladin 3 / Zauberer 3",
		Abilities: map[string]int{
			"constitution": 15,
		},
	}
	if got := builderDerivedHitPointMax(character); got != 46 {
		t.Fatalf("expected multiclass hp 46, got %d", got)
	}
	if got := builderDerivedHitDice(character); got != "3W10 + 3W6" {
		t.Fatalf("expected multiclass hit dice summary, got %q", got)
	}
}

func TestBuilderDeterministicHitPointsReplyCorrectsMulticlassPrimaryHitDie(t *testing.T) {
	reply, patch, ok := builderDeterministicHitPointsReply(Character{
		Race:          "Drachenblütiger",
		ClassAndLevel: "Paladin 3 / Zauberer 3",
		Abilities: map[string]int{
			"constitution": 15,
		},
	}, "Warum 1 w6 der paladin ist die erste klasse damit ist das doch 1 w10 oder nicht?")
	if !ok {
		t.Fatal("expected deterministic hit point correction")
	}
	if !strings.Contains(reply, "Startklasse Paladin") || !strings.Contains(reply, "46 Trefferpunkte") || !strings.Contains(reply, "3W10 + 3W6") {
		t.Fatalf("unexpected reply: %s", reply)
	}
	if patch.HitPointMax == nil || *patch.HitPointMax != 46 {
		t.Fatalf("expected repaired hit points, got %#v", patch.HitPointMax)
	}
	if got := patch.Metadata["hit_dice"]; got != "3W10 + 3W6" {
		t.Fatalf("expected repaired hit dice, got %#v", got)
	}
	if got := patch.Metadata["builder_stage"]; got != "languages_senses_and_body" {
		t.Fatalf("expected next stage body step, got %#v", got)
	}
}

func TestBuilderDeterministicAbilityAssignmentReplyParsesGermanShortKeysAndAppliesRaceBonuses(t *testing.T) {
	reply, patch, ok := builderDeterministicAbilityAssignmentReply(Character{
		Race: "Drachenblütiger",
		Metadata: map[string]any{
			"resolved_values": []any{17, 16, 16, 15, 14, 13},
			"concept":         "Multiclass-Charakter: Paladin 3 / Zauberer 1",
		},
	}, "CH: 17 Ko: 16 WE: 16 In: 14 GE: 15 St: 13")
	if !ok {
		t.Fatal("expected deterministic ability assignment reply")
	}
	if !strings.Contains(reply, "Stärke 15") || !strings.Contains(reply, "Charisma 18") {
		t.Fatalf("reply should mention final race-adjusted values, got: %s", reply)
	}
	if patch.ClassAndLevel == nil || *patch.ClassAndLevel != "Paladin 3 / Zauberer 1" {
		t.Fatalf("expected inferred multiclass text, got %#v", patch.ClassAndLevel)
	}
	if got := patch.Metadata["builder_stage"]; got != "class_proficiencies_and_choices" {
		t.Fatalf("expected next stage class_proficiencies_and_choices, got %#v", got)
	}
	if patch.Abilities["strength"] != 15 || patch.Abilities["charisma"] != 18 || patch.Abilities["constitution"] != 16 || patch.Abilities["wisdom"] != 16 {
		t.Fatalf("unexpected final assignment: %#v", patch.Abilities)
	}
}

func TestBuilderDeterministicHitPointsReplyRefusesToAdvanceWithoutStoredAbilities(t *testing.T) {
	reply, patch, ok := builderDeterministicHitPointsReply(Character{
		Race:          "Drachenblütiger",
		ClassAndLevel: "Paladin 3 / Zauberer 1",
	}, "weiter")
	if !ok {
		t.Fatal("expected deterministic guard reply")
	}
	if !strings.Contains(reply, "Attributswerte") {
		t.Fatalf("expected ability guard in reply, got: %s", reply)
	}
	if got := patch.Metadata["builder_stage"]; got != "ability_scores" {
		t.Fatalf("expected builder stage to return to ability_scores, got %#v", got)
	}
}

func TestBuilderDeterministicCoreFieldRepairReplyRepairsMissingRaceAndClassFromCorrection(t *testing.T) {
	reply, patch, ok := builderDeterministicCoreFieldRepairReply(Character{
		Metadata: map[string]any{
			"builder_stage": "background_and_alignment",
			"concept":       "Mehrklassiger Charakter: Paladin Stufe 3 und Zauberer Stufe 1",
		},
	}, "du hast das volk nicht eingetragen!!!! Drachenblut")
	if !ok {
		t.Fatal("expected core field repair reply")
	}
	if patch.Race == nil || *patch.Race != "Drachenblütiger" {
		t.Fatalf("expected repaired race, got %#v", patch.Race)
	}
	if patch.ClassAndLevel == nil || *patch.ClassAndLevel != "Paladin 3 / Zauberer 1" {
		t.Fatalf("expected repaired class text, got %#v", patch.ClassAndLevel)
	}
	if got := patch.Metadata["builder_stage"]; got != "background_and_alignment" {
		t.Fatalf("expected stage to remain on background_and_alignment, got %#v", got)
	}
	if !strings.Contains(reply, "Volk eingetragen: Drachenblütiger.") || !strings.Contains(reply, "Klassenfolge eingetragen: Paladin 3 / Zauberer 1.") {
		t.Fatalf("unexpected repair reply: %s", reply)
	}
}

func TestBuilderDeterministicRaceChoiceReplySetsRaceAndAdvancesToClassStage(t *testing.T) {
	reply, patch, ok := builderDeterministicRaceChoiceReply(Character{
		Metadata: map[string]any{
			"builder_stage": "race",
			"concept":       "Mehrklassiger Charakter: Paladin Stufe 3 und Zauberer Stufe 1",
		},
	}, "Drachenblütiger")
	if !ok {
		t.Fatal("expected deterministic race choice reply")
	}
	if patch.Race == nil || *patch.Race != "Drachenblütiger" {
		t.Fatalf("expected race patch, got %#v", patch.Race)
	}
	if got := patch.Metadata["builder_stage"]; got != "class_and_level" {
		t.Fatalf("expected next stage class_and_level, got %#v", got)
	}
	if !strings.Contains(reply, "Volk: Drachenblütiger.") || !strings.Contains(reply, "Wähle als Startklasse Paladin oder Zauberer.") {
		t.Fatalf("unexpected race choice reply: %s", reply)
	}
}

func TestBuilderDeterministicRaceChoiceReplyAcceptsDrachenblutAlias(t *testing.T) {
	reply, patch, ok := builderDeterministicRaceChoiceReply(Character{
		Metadata: map[string]any{
			"builder_stage": "race",
			"concept":       "Mehrklassiger Charakter: Paladin Stufe 3 und Zauberer Stufe 1",
		},
	}, "Drachenblut")
	if !ok {
		t.Fatal("expected deterministic race alias reply")
	}
	if patch.Race == nil || *patch.Race != "Drachenblütiger" {
		t.Fatalf("expected canonical dragonborn race, got %#v", patch.Race)
	}
	if got := patch.Metadata["builder_stage"]; got != "class_and_level" {
		t.Fatalf("expected next stage class_and_level, got %#v", got)
	}
	if !strings.Contains(reply, "Drachenblütiger") {
		t.Fatalf("expected canonical race name in reply, got: %s", reply)
	}
}

func TestSanitizeCharacterBuilderPatchForCurrentStagePreservesRaceDuringRaceToClassTransition(t *testing.T) {
	race := "Drachenblütiger"
	patch := CharacterBuilderPatch{
		Race: &race,
		Metadata: map[string]any{
			"builder_stage": "class_and_level",
		},
	}
	sanitizeCharacterBuilderPatchForStage(&patch, "race")
	if patch.Race == nil || *patch.Race != "Drachenblütiger" {
		t.Fatalf("expected race to survive current-stage sanitization, got %#v", patch.Race)
	}
	if got := patch.Metadata["builder_stage"]; got != "class_and_level" {
		t.Fatalf("expected next stage metadata to remain intact, got %#v", got)
	}
}

func TestSanitizeCharacterBuilderPatchForCurrentStagePreservesClassAndLevelDuringClassTransition(t *testing.T) {
	classAndLevel := "Paladin 3 / Zauberer 1"
	patch := CharacterBuilderPatch{
		ClassAndLevel: &classAndLevel,
		Metadata: map[string]any{
			"builder_stage": "background_and_alignment",
		},
	}
	sanitizeCharacterBuilderPatchForStage(&patch, "class_and_level")
	if patch.ClassAndLevel == nil || *patch.ClassAndLevel != classAndLevel {
		t.Fatalf("expected class_and_level to survive current-stage sanitization, got %#v", patch.ClassAndLevel)
	}
}

func TestSanitizeCharacterBuilderPatchForCurrentStagePreservesBackgroundAndAlignmentDuringBackgroundTransition(t *testing.T) {
	background := "Akolyth"
	alignment := "Rechtschaffen gut"
	patch := CharacterBuilderPatch{
		Background: &background,
		Alignment:  &alignment,
		Metadata: map[string]any{
			"builder_stage": "ability_method",
		},
	}
	sanitizeCharacterBuilderPatchForStage(&patch, "background_and_alignment")
	if patch.Background == nil || *patch.Background != background {
		t.Fatalf("expected background to survive current-stage sanitization, got %#v", patch.Background)
	}
	if patch.Alignment == nil || *patch.Alignment != alignment {
		t.Fatalf("expected alignment to survive current-stage sanitization, got %#v", patch.Alignment)
	}
}

func TestSanitizeCharacterBuilderPatchForCurrentStagePreservesAbilitiesDuringAbilityTransition(t *testing.T) {
	patch := CharacterBuilderPatch{
		Abilities: map[string]int{
			"strength":     15,
			"dexterity":    12,
			"constitution": 14,
			"intelligence": 10,
			"wisdom":       8,
			"charisma":     16,
		},
		Metadata: map[string]any{
			"builder_stage": "class_proficiencies_and_choices",
		},
	}
	sanitizeCharacterBuilderPatchForStage(&patch, "ability_scores")
	if len(patch.Abilities) != 6 {
		t.Fatalf("expected abilities to survive current-stage sanitization, got %#v", patch.Abilities)
	}
}

func TestSanitizeCharacterBuilderPatchForCurrentStagePreservesHitPointFieldsDuringHitPointTransition(t *testing.T) {
	hp := 13
	speed := "30 Fuß"
	prof := "+2"
	patch := CharacterBuilderPatch{
		HitPointMax: &hp,
		Speed:       &speed,
		Proficiency: &prof,
		Metadata: map[string]any{
			"builder_stage": "languages_senses_and_body",
			"hit_dice":      "1W10",
		},
	}
	sanitizeCharacterBuilderPatchForStage(&patch, "hit_points_hit_dice_and_movement")
	if patch.HitPointMax == nil || *patch.HitPointMax != hp {
		t.Fatalf("expected hit points to survive current-stage sanitization, got %#v", patch.HitPointMax)
	}
	if patch.Speed == nil || *patch.Speed != speed {
		t.Fatalf("expected speed to survive current-stage sanitization, got %#v", patch.Speed)
	}
	if patch.Proficiency == nil || *patch.Proficiency != prof {
		t.Fatalf("expected proficiency to survive current-stage sanitization, got %#v", patch.Proficiency)
	}
	if got := patch.Metadata["hit_dice"]; got != "1W10" {
		t.Fatalf("expected hit_dice metadata to survive current-stage sanitization, got %#v", got)
	}
}

func TestBuilderDeterministicSensesAndBodyReplyParsesGermanFreeformBodyDetailsAndRefreshesSenses(t *testing.T) {
	reply, patch, ok := builderDeterministicSensesAndBodyReply(Character{
		Race: "Drachenblütiger",
		ClassAndLevel: "Paladin 1",
		Abilities: map[string]int{
			"wisdom": 14,
		},
		Metadata: map[string]any{
			"builder_stage":       "languages_senses_and_body",
			"passive_perception":  14,
			"senses":              "Passive Wahrnehmung 5",
			"skill_proficiencies": []string{"Wahrnehmung"},
		},
	}, "alter 80 grösse 205 cm gewicht 150 Kg Augen Blau haut Lila Harre keine ist drachenblüter hat schuppen ;-) passive warhnemnung ist 14!")
	if !ok {
		t.Fatal("expected deterministic senses/body reply")
	}
	if got := patch.Metadata["age"]; got != "80" {
		t.Fatalf("expected age 80, got %#v", got)
	}
	if got := patch.Metadata["size"]; got != "205 cm" {
		t.Fatalf("expected size 205 cm, got %#v", got)
	}
	if got := patch.Metadata["weight"]; got != "150 Kg" {
		t.Fatalf("expected weight 150 Kg, got %#v", got)
	}
	if got := patch.Metadata["eyes"]; got != "Blau" {
		t.Fatalf("expected eyes Blau, got %#v", got)
	}
	if got := patch.Metadata["skin"]; got != "Lila" {
		t.Fatalf("expected skin Lila, got %#v", got)
	}
	if got := patch.Metadata["hair"]; got != "keine ist drachenblüter hat schuppen" {
		t.Fatalf("expected parsed hair text, got %#v", got)
	}
	if got := patch.Metadata["senses"]; got != "Passive Wahrnehmung 14" {
		t.Fatalf("expected refreshed senses, got %#v", got)
	}
	if got := patch.Metadata["builder_stage"]; got != "personality" {
		t.Fatalf("expected next stage personality, got %#v", got)
	}
	if !strings.Contains(reply, "Sinne und Körperdaten sind festgelegt") {
		t.Fatalf("unexpected reply: %s", reply)
	}
}

func TestBuilderQuestionLooksLikeRecommendationRequestRecognizesSchlaegstDuVor(t *testing.T) {
	if !builderQuestionLooksLikeRecommendationRequest("was schlägst du vor") {
		t.Fatal("expected recommendation intent for 'was schlägst du vor'")
	}
	if !builderQuestionLooksLikeRecommendationRequest("ok was schlägst du vor") {
		t.Fatal("expected recommendation intent for 'ok was schlägst du vor'")
	}
}

func TestBuilderDeterministicGuidedChoiceCompletionReturnsEquipmentAdviceForWasSchlaegstDuVor(t *testing.T) {
	character := &Character{
		ClassAndLevel: "Paladin 1",
		Race:          "Drachenblütiger",
		Background:    "Ritter",
		Alignment:     "Neutral",
		Abilities: map[string]int{
			"strength":     16,
			"dexterity":    14,
			"constitution": 16,
			"intelligence": 13,
			"wisdom":       14,
			"charisma":     18,
		},
		Metadata: map[string]any{
			"builder_stage": "equipment_and_money",
		},
	}
	completion, ok := builderDeterministicGuidedChoiceCompletion(character, "equipment_and_money", "was schlägst du vor", "")
	if !ok {
		t.Fatal("expected deterministic guided completion")
	}
	if !strings.Contains(completion.Reply, "Startausrüstungsoptionen") && !strings.Contains(completion.Reply, "empfehle") {
		t.Fatalf("expected equipment advice reply, got: %s", completion.Reply)
	}
}

func TestEquipmentStageRecommendationBypassesGenericFallbackInCompleteCharacterBuilder(t *testing.T) {
	handler := &Handler{}
	character := &Character{
		ID:            "test-char",
		ClassAndLevel: "Paladin 1",
		Race:          "Drachenblütiger",
		Background:    "Ritter",
		Alignment:     "Neutral",
		Metadata: map[string]any{
			"language":        "de",
			"ruleset_work":    "5E",
			"ruleset_version": "2014",
			"builder_stage":   "equipment_and_money",
		},
	}
	session := &LLMSession{
		StructuredState: map[string]any{},
		WorkingSummary:  map[string]any{},
	}
	transcript := []CharacterBuilderMessage{
		{Role: "assistant", Content: "Jetzt tragen wir die Ausrüstung und das Geld sauber ein."},
		{Role: "user", Content: "was schlägst du vor"},
	}
	completion, err := handler.completeCharacterBuilder(context.Background(), character, session, nil, transcript)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(completion.Reply, "Startausrüstungsoptionen") && !strings.Contains(completion.Reply, "empfehle") {
		t.Fatalf("expected direct equipment advice reply, got: %s", completion.Reply)
	}
}

func TestEnforceBuilderStageConsistencyRewindsWhenStoredStageIsAheadOfActualState(t *testing.T) {
	character := Character{
		Race: "",
		Metadata: map[string]any{
			"builder_stage": "background_and_alignment",
			"concept":       "Paladin level 3 Wizard level 1 multiclass",
		},
	}
	enforceBuilderStageConsistency(&character)
	if got := safeOptionalString(character.Metadata["builder_stage"]); got != "race" {
		t.Fatalf("expected stage rewind to race, got %q", got)
	}
}

func TestEnforceBuilderStageConsistencyWorksIndependentlyOfLanguage(t *testing.T) {
	character := Character{
		Race:          "Dragonborn",
		ClassAndLevel: "Paladin 3 / Wizard 1",
		Metadata: map[string]any{
			"builder_stage": "hit_points_hit_dice_and_movement",
			"concept":       "Paladin level 3 Wizard level 1 multiclass",
		},
	}
	enforceBuilderStageConsistency(&character)
	if got := safeOptionalString(character.Metadata["builder_stage"]); got != "background_and_alignment" {
		t.Fatalf("expected stage rewind to background_and_alignment, got %q", got)
	}
}

func TestVerifyCharacterBuilderCompletionReplacesOvereagerReplyWithFallbackForActualStage(t *testing.T) {
	character := Character{
		Name: "Dragonis",
		Metadata: map[string]any{
			"language":      "en",
			"builder_stage": "race",
			"concept":       "Paladin level 3 Wizard level 1 multiclass",
		},
	}
	completion := characterBuilderCompletion{
		Reply: "Race set: Dragonborn. Next, choose the rules background and alignment.",
		Patch: CharacterBuilderPatch{
			Metadata: map[string]any{
				"builder_stage": "background_and_alignment",
			},
		},
	}
	verified := verifyCharacterBuilderCompletion(character, completion, nil)
	if got := safeOptionalString(verified.Patch.Metadata["builder_stage"]); got != "race" {
		t.Fatalf("expected verified stage race, got %q", got)
	}
	if strings.Contains(strings.ToLower(verified.Reply), "background and alignment") {
		t.Fatalf("reply should have been rewound to race fallback, got: %s", verified.Reply)
	}
	if !strings.Contains(strings.ToLower(verified.Reply), "which ancestry should the character have") {
		t.Fatalf("expected race fallback reply, got: %s", verified.Reply)
	}
}

func TestBuilderClassRuleForTextPrefersEarlierMatchOverLaterParentheticalHint(t *testing.T) {
	rule, ok := builderClassRuleForText("Zauberer 3 (Startklasse: Paladin)")
	if !ok {
		t.Fatal("expected class rule match")
	}
	if rule.ClassName != "Zauberer" {
		t.Fatalf("expected wizard class rule from leading token, got %+v", rule)
	}
}

func TestBuilderDerivedHitDiceParsesStartklasseAnnotatedMulticlassText(t *testing.T) {
	character := Character{
		ClassAndLevel: "Paladin 3 / Zauberer 3 (Startklasse: Paladin)",
		Abilities: map[string]int{
			"constitution": 16,
		},
	}
	if got := builderDerivedHitDice(character); got != "3W10 + 3W6" {
		t.Fatalf("expected annotated multiclass hit dice summary, got %q", got)
	}
	if got := builderDerivedHitPointMax(character); got != 52 {
		t.Fatalf("expected annotated multiclass hp 52, got %d", got)
	}
}

func TestBuilderDeterministicHitPointsReplyWorksEvenAfterStageAlreadyAdvanced(t *testing.T) {
	reply, patch, ok := builderDeterministicHitPointsReply(Character{
		Race:          "Drachenblütiger",
		ClassAndLevel: "Paladin 3 / Zauberer 3 (Startklasse: Paladin)",
		Speed:         "30 Fuß",
		Abilities: map[string]int{
			"constitution": 16,
		},
		Metadata: map[string]any{
			"builder_stage": "languages_senses_and_body",
			"hit_dice":      "3W10",
		},
	}, "Moment Trefferwürfel 3W10 gibt es nicht 1 w10 ist richtig für stufe 1")
	if !ok {
		t.Fatal("expected late correction to be intercepted")
	}
	if !strings.Contains(reply, "Für den gesamten Charakterbogen ist der Trefferwürfel-Pool") {
		t.Fatalf("unexpected repair reply: %s", reply)
	}
	if patch.HitPointMax == nil || *patch.HitPointMax != 52 {
		t.Fatalf("expected repaired hp 52, got %#v", patch.HitPointMax)
	}
	if got := patch.Metadata["hit_dice"]; got != "3W10 + 3W6" {
		t.Fatalf("expected repaired hit dice pool, got %#v", got)
	}
}

func TestBuilderSRDSpellCatalogGeneratedFromOfficialListHasExpectedSize(t *testing.T) {
	if got := len(builderSRDSpellCatalog); got != 319 {
		t.Fatalf("expected full SRD 5.1 spell catalog size 319, got %d", got)
	}
}

func TestBuilderSpellAdviceForWarlockStaysWithinSRD51(t *testing.T) {
	advice, ok := builderSpellAdviceForCharacter(Character{
		ClassAndLevel: "Hexenmeister, Stufe 1",
		Abilities: map[string]int{
			"charisma": 16,
		},
	})
	if !ok {
		t.Fatal("expected spell advice for warlock")
	}
	for _, forbidden := range []string{"Hex", "Armor of Agathys"} {
		for _, option := range advice.Options {
			if strings.Contains(option, forbidden) {
				t.Fatalf("unexpected non-SRD 5.1 warlock option %q in %#v", forbidden, advice.Options)
			}
		}
		for _, spell := range advice.Recommendation {
			if spell == forbidden {
				t.Fatalf("unexpected non-SRD 5.1 warlock recommendation %q", spell)
			}
		}
	}
	for _, expected := range []string{"Schauriger Strahl", "Höllischer Tadel", "Person bezaubern"} {
		found := false
		for _, spell := range advice.Recommendation {
			if spell == expected {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected %q in warlock recommendations: %#v", expected, advice.Recommendation)
		}
	}
}

func TestEmbeddedBuilderGuideDocumentsIncludeSRD51SourceSnapshot(t *testing.T) {
	docs := embeddedGuidesForRuleset("5E", "2014")
	for _, doc := range docs {
		if doc.ID == "embedded-srd51-source-dnd-5e" {
			if doc.Metadata["source_type"] != "embedded_srd51_source" {
				t.Fatalf("unexpected source type: %#v", doc.Metadata["source_type"])
			}
			if doc.ChunkCount == 0 {
				t.Fatal("expected chunked SRD source snapshot")
			}
			return
		}
	}
	t.Fatal("expected embedded SRD 5.1 source snapshot guide")
}

func TestBuilderCoreClassRulesComeFromCentralSRDCatalog(t *testing.T) {
	if len(builderCoreClassRules) != len(srdClassCatalog) {
		t.Fatalf("expected %d class rules from central catalog, got %d", len(srdClassCatalog), len(builderCoreClassRules))
	}
	entry, ok := srdClassCatalogEntryByName("Kämpfer")
	if !ok {
		t.Fatal("expected fighter entry in central catalog")
	}
	advice, ok := builderEquipmentAdviceForCharacter(Character{ClassAndLevel: "Kämpfer, Stufe 1"})
	if !ok {
		t.Fatal("expected fighter equipment advice")
	}
	if len(advice.Recommendation) != len(entry.Equipment.Recommendation) {
		t.Fatalf("expected equipment advice from central catalog, got %#v", advice.Recommendation)
	}
}
