package httpapi

import (
	"strings"
	"testing"
)

func intPtr(value int) *int {
	return &value
}

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

func TestBuilderBackgroundReplyOffersSkillListAndKnightSuggestion(t *testing.T) {
	reply, ok := builderDeterministicBackgroundReply(Character{}, "Eigener Hintergrund Ritter, aber welche Fertigkeiten kann ich wählen und was passt am besten?")
	if !ok {
		t.Fatal("expected deterministic background guidance for knight")
	}
	for _, expected := range []string{"Athletik", "Einschüchtern", "Geschichte", "Überzeugen", "Mit Tieren umgehen"} {
		if !strings.Contains(reply, expected) {
			t.Fatalf("reply does not contain %q: %s", expected, reply)
		}
	}
}

func TestBuilderClassStageAdviceOffersKnightSuggestions(t *testing.T) {
	reply, ok := builderDeterministicStageAdviceReply(Character{}, "class_and_level", "Ich möchte etwas Ritterliches, welche Klassen passen am besten?")
	if !ok {
		t.Fatal("expected deterministic class advice")
	}
	for _, expected := range []string{"Kämpfer, Stufe 1", "Paladin, Stufe 1", "Waldläufer, Stufe 1"} {
		if !strings.Contains(reply, expected) {
			t.Fatalf("reply does not contain %q: %s", expected, reply)
		}
	}
}

func TestBuilderClassChoicesReplyOffersSuggestedSkillSets(t *testing.T) {
	character := Character{
		ClassAndLevel: "Kämpfer 1",
		Background:    "Ritter",
	}
	reply, ok := builderDeterministicClassChoicesReply(character)
	if !ok {
		t.Fatal("expected deterministic class choices reply")
	}
	for _, expected := range []string{"Athletik", "Wahrnehmung", "Einschüchtern"} {
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

func TestBackgroundStageKeepsBackgroundSkillMetadata(t *testing.T) {
	patch := CharacterBuilderPatch{
		Metadata: map[string]any{
			"background_skill_proficiencies": []string{"Athletik", "Wahrnehmung"},
			"skill_proficiencies":            []string{"Athletik", "Wahrnehmung"},
			"class_skill_proficiencies":      []string{"Geschichte"},
		},
	}
	sanitizeCharacterBuilderPatchForStage(&patch, "background_and_alignment")
	if patch.Metadata == nil {
		t.Fatal("expected metadata to survive background stage sanitization")
	}
	if _, ok := patch.Metadata["background_skill_proficiencies"]; !ok {
		t.Fatalf("background skill metadata was removed: %#v", patch.Metadata)
	}
	if _, ok := patch.Metadata["skill_proficiencies"]; !ok {
		t.Fatalf("merged skill metadata was removed: %#v", patch.Metadata)
	}
	if _, ok := patch.Metadata["class_skill_proficiencies"]; ok {
		t.Fatalf("class skill metadata should not be allowed in background stage: %#v", patch.Metadata)
	}
}

func TestBuilderDeterministicSkillChoiceReplySeparatesBackgroundAndClassSkills(t *testing.T) {
	character := Character{
		ClassAndLevel: "Kämpfer 1",
		Race:          "Mensch",
		Background:    "Ritter",
		Metadata: map[string]any{
			"background_skill_proficiencies": []string{"Athletik", "Wahrnehmung"},
			"skill_proficiencies":            []string{"Athletik", "Wahrnehmung"},
		},
	}
	reply, patch, ok := builderDeterministicSkillChoiceReply(character, "Geschichte und Motiv erkennen", "")
	if !ok {
		t.Fatal("expected deterministic class skill completion")
	}
	if !strings.Contains(reply, "Athletik") || !strings.Contains(reply, "Geschichte") || !strings.Contains(reply, "Motiv erkennen") {
		t.Fatalf("reply should mention merged skills, got: %s", reply)
	}
	classSkills := stringListFromAny(patch.Metadata["class_skill_proficiencies"])
	if got := strings.Join(classSkills, ", "); got != "Geschichte, Motiv erkennen" {
		t.Fatalf("unexpected class skills: %s", got)
	}
	allSkills := stringListFromAny(patch.Metadata["skill_proficiencies"])
	if got := strings.Join(allSkills, ", "); got != "Athletik, Wahrnehmung, Geschichte, Motiv erkennen" {
		t.Fatalf("unexpected merged skills: %s", got)
	}
	if got := patch.Metadata["builder_stage"]; got != "hit_points_hit_dice_and_movement" {
		t.Fatalf("unexpected next stage: %v", got)
	}
}

func TestBuilderDeterministicSkillChoiceReplyResolvesPreviousSuggestionConfirmation(t *testing.T) {
	character := Character{
		ClassAndLevel: "Kämpfer 1",
		Race:          "Mensch",
		Background:    "Ritter",
		Metadata: map[string]any{
			"background_skill_proficiencies": []string{"Athletik", "Wahrnehmung"},
			"skill_proficiencies":            []string{"Athletik", "Wahrnehmung"},
		},
	}
	previousAssistant := "Ja. Athletik und Wahrnehmung stammen bereits aus deinem Hintergrund „Ritter“ und werden jetzt als Fertigkeitsübungen eingetragen; du kannst sie nicht noch einmal als Kämpfer wählen. Die Rettungswürfe Stärke und Konstitution sind ebenfalls bereits fest. Wähle nun 2 andere Kämpfer-Fertigkeiten aus Akrobatik, Mit Tieren umgehen, Geschichte, Motiv erkennen, Einschüchtern und Überlebenskunst; für den arkanen Klingenkämpfer bietet sich Geschichte und Motiv erkennen an."
	reply, patch, ok := builderDeterministicSkillChoiceReply(character, "ja nimm die beiden", previousAssistant)
	if !ok {
		t.Fatal("expected deterministic class skill completion from prior suggestion")
	}
	if !strings.Contains(reply, "Geschichte") || !strings.Contains(reply, "Motiv erkennen") {
		t.Fatalf("reply should mention resolved class skills, got: %s", reply)
	}
	classSkills := stringListFromAny(patch.Metadata["class_skill_proficiencies"])
	if got := strings.Join(classSkills, ", "); got != "Geschichte, Motiv erkennen" {
		t.Fatalf("unexpected class skills from prior suggestion: %s", got)
	}
}

func TestReconcileBuilderDerivedCharacterFieldsMergesSkillSources(t *testing.T) {
	character := &Character{
		ClassAndLevel: "Kämpfer 1",
		Metadata: map[string]any{
			"background_skill_proficiencies": []string{"Athletik", "Wahrnehmung"},
			"class_skill_proficiencies":      []string{"Geschichte", "Motiv erkennen"},
		},
	}
	reconcileBuilderDerivedCharacterFields(character)
	allSkills := stringListFromAny(character.Metadata["skill_proficiencies"])
	if got := strings.Join(allSkills, ", "); got != "Athletik, Wahrnehmung, Geschichte, Motiv erkennen" {
		t.Fatalf("unexpected merged skills after reconcile: %s", got)
	}
}

func TestBuilderDeterministicBackgroundChoiceReplyStoresBackgroundSkills(t *testing.T) {
	reply, patch, ok := builderDeterministicBackgroundChoiceReply(Character{}, "Ritter\nAthletik und Wahrnehmung\nwelche Werkzeug skills kann ich noch dazu nehmen?")
	if !ok {
		t.Fatal("expected deterministic background choice completion")
	}
	if patch.Background == nil || *patch.Background != "Ritter" {
		t.Fatalf("unexpected background: %#v", patch.Background)
	}
	if !strings.Contains(reply, "Athletik") || !strings.Contains(reply, "Wahrnehmung") {
		t.Fatalf("reply should mention stored background skills, got: %s", reply)
	}
	backgroundSkills := stringListFromAny(patch.Metadata["background_skill_proficiencies"])
	if got := strings.Join(backgroundSkills, ", "); got != "Athletik, Wahrnehmung" {
		t.Fatalf("unexpected background skills: %s", got)
	}
}

func TestBuilderDeterministicSkillRepairReplyMarksMissingClassSkillsAfterStageAdvance(t *testing.T) {
	character := Character{
		ClassAndLevel: "Kämpfer 1",
		Background:    "Ritter",
		Metadata: map[string]any{
			"builder_stage":                  "personality",
			"background_skill_proficiencies": []string{"Athletik", "Wahrnehmung"},
			"skill_proficiencies":            []string{"Athletik", "Wahrnehmung"},
		},
	}
	reply, patch, ok := builderDeterministicSkillRepairReply(character, "personality", "moment Geschichte und Motive erkennen sind noch nicht markiert als geübt bitte markier die auch noch", "")
	if !ok {
		t.Fatal("expected skill repair reply")
	}
	if !strings.Contains(reply, "Geschichte") || !strings.Contains(reply, "Motiv erkennen") {
		t.Fatalf("reply should confirm repaired skills, got: %s", reply)
	}
	classSkills := stringListFromAny(patch.Metadata["class_skill_proficiencies"])
	if got := strings.Join(classSkills, ", "); got != "Geschichte, Motiv erkennen" {
		t.Fatalf("unexpected repaired class skills: %s", got)
	}
	allSkills := stringListFromAny(patch.Metadata["skill_proficiencies"])
	if got := strings.Join(allSkills, ", "); got != "Athletik, Wahrnehmung, Geschichte, Motiv erkennen" {
		t.Fatalf("unexpected repaired merged skills: %s", got)
	}
}

func TestBuilderDeterministicEquipmentAdviceReplyListsFighterOptionsAndRecommendation(t *testing.T) {
	character := Character{ClassAndLevel: "Kämpfer, Stufe 1", Background: "Ritter"}
	reply, ok := builderDeterministicEquipmentAdviceReply(character, "welche Startausrüstungsoptionen habe ich?")
	if !ok {
		t.Fatal("expected equipment advice")
	}
	for _, expected := range []string{"Kettenhemd", "Langschwert", "Schild", "leichte Armbrust", "Entdeckerausrüstung"} {
		if !strings.Contains(reply, expected) {
			t.Fatalf("reply does not contain %q: %s", expected, reply)
		}
	}
}

func TestBuilderDeterministicEquipmentReplyCanApplyRecommendedLoadout(t *testing.T) {
	character := Character{ClassAndLevel: "Kämpfer, Stufe 1", Background: "Ritter"}
	previousAssistant := "Für Kämpfer auf Stufe 1 stehen im SRD 5.1 diese Startausrüstungsoptionen zur Verfügung. Für deinen arkanen Klingenkämpfer empfehle ich als Standardauswahl Kettenhemd, Langschwert und Schild, leichte Armbrust mit 20 Bolzen und Entdeckerausrüstung. Wenn du möchtest, übernehme ich genau diese Auswahl direkt."
	reply, patch, ok := builderDeterministicEquipmentReply(character, "ja mach das bitte", previousAssistant)
	if !ok {
		t.Fatal("expected recommended equipment application")
	}
	if !strings.Contains(reply, "Kettenhemd") {
		t.Fatalf("unexpected reply: %s", reply)
	}
	items := stringListFromAny(patch.Metadata["starting_equipment"])
	if got := strings.Join(items, ", "); got != "Kettenhemd, Langschwert und Schild, leichte Armbrust mit 20 Bolzen, Entdeckerausrüstung" {
		t.Fatalf("unexpected starting equipment: %s", got)
	}
}

func TestBuilderEquipmentAdviceAvailableForAllCoreClasses(t *testing.T) {
	classes := []string{
		"Barbar, Stufe 1",
		"Barde, Stufe 1",
		"Kleriker, Stufe 1",
		"Druide, Stufe 1",
		"Kämpfer, Stufe 1",
		"Mönch, Stufe 1",
		"Paladin, Stufe 1",
		"Waldläufer, Stufe 1",
		"Schurke, Stufe 1",
		"Zauberer, Stufe 1",
		"Hexenmeister, Stufe 1",
		"Magier, Stufe 1",
	}
	for _, classAndLevel := range classes {
		character := Character{ClassAndLevel: classAndLevel}
		advice, ok := builderEquipmentAdviceForCharacter(character)
		if !ok {
			t.Fatalf("expected equipment advice for %s", classAndLevel)
		}
		if len(advice.Options) == 0 || len(advice.Recommendation) == 0 {
			t.Fatalf("expected options and recommendation for %s: %#v", classAndLevel, advice)
		}
	}
}

func TestBuilderEquipmentAdviceReplyListsWizardFallback(t *testing.T) {
	character := Character{ClassAndLevel: "Magier, Stufe 1"}
	reply, ok := builderDeterministicEquipmentAdviceReply(character, "welche Startausrüstungsoptionen habe ich?")
	if !ok {
		t.Fatal("expected wizard equipment advice")
	}
	for _, expected := range []string{"Quarterstaff", "Dolch", "arkaner Fokus", "Zauberbuch"} {
		if !strings.Contains(reply, expected) {
			t.Fatalf("reply does not contain %q: %s", expected, reply)
		}
	}
}

func TestBuilderClassFeatureAdviceReplyListsFighterDefaults(t *testing.T) {
	character := Character{ClassAndLevel: "Kämpfer, Stufe 1"}
	reply, ok := builderDeterministicClassFeatureAdviceReply(character, "welche Klassenmerkmale habe ich?")
	if !ok {
		t.Fatal("expected class feature advice")
	}
	for _, expected := range []string{"Fighting Style", "Second Wind", "Defense"} {
		if !strings.Contains(reply, expected) {
			t.Fatalf("reply does not contain %q: %s", expected, reply)
		}
	}
}

func TestBuilderSpellcastingAdviceReplyListsWizardDefaults(t *testing.T) {
	character := Character{
		ClassAndLevel: "Magier, Stufe 1",
		Abilities: map[string]int{
			"intelligence": 16,
		},
	}
	reply, ok := builderDeterministicSpellcastingAdviceReply(character, "welche Zauber soll ich nehmen?")
	if !ok {
		t.Fatal("expected spellcasting advice")
	}
	for _, expected := range []string{"Feuerpfeil", "Magisches Geschoss", "Schild", "Zauber-SG 13"} {
		if !strings.Contains(reply, expected) {
			t.Fatalf("reply does not contain %q: %s", expected, reply)
		}
	}
}

func TestBuilderDeterministicSpellcastingReplyStoresStructuredSpellAttackRows(t *testing.T) {
	character := Character{
		ClassAndLevel: "Magier, Stufe 1",
		Abilities: map[string]int{
			"intelligence": 16,
		},
	}
	_, patch, ok := builderDeterministicSpellcastingReply(character, "mach das", "")
	if !ok {
		t.Fatal("expected deterministic spellcasting reply")
	}
	rows, ok := patch.Metadata["spell_attacks"].(string)
	if !ok {
		t.Fatalf("expected spell_attacks string, got %#v", patch.Metadata["spell_attacks"])
	}
	for _, expected := range []string{"Zaubertrick | Feuerpfeil", "Grad 1 | Magisches Geschoss"} {
		if !strings.Contains(rows, expected) {
			t.Fatalf("expected spell attack rows to contain %q, got %q", expected, rows)
		}
	}
	for _, forbidden := range []string{"Magierhand", "Fingerfertigkeit", "Schild", "Magische Rüstung"} {
		if strings.Contains(rows, forbidden) {
			t.Fatalf("did not expect non-attack spell %q in spell attack rows: %q", forbidden, rows)
		}
	}
	if !strings.Contains(rows, "Beschreibung:") {
		t.Fatalf("expected german spell descriptions, got %q", rows)
	}
	notes, ok := patch.Metadata["spell_notes"].(string)
	if !ok {
		t.Fatalf("expected spell_notes string, got %#v", patch.Metadata["spell_notes"])
	}
	for _, expected := range []string{"Magierhand:", "Fingerfertigkeit:", "Schild:", "Magische Rüstung:"} {
		if !strings.Contains(notes, expected) {
			t.Fatalf("expected spell notes to contain %q, got %q", expected, notes)
		}
	}
	for _, forbidden := range []string{"Feuerpfeil:", "Magisches Geschoss:", "Schlaf:"} {
		if strings.Contains(notes, forbidden) {
			t.Fatalf("did not expect attack/save spell %q in spell notes: %q", forbidden, notes)
		}
	}
}

func TestBuilderDeterministicGuidedChoiceCompletionKeepsSpellcastingStageConfirmation(t *testing.T) {
	character := Character{
		ClassAndLevel: "Magier, Stufe 1",
		Abilities: map[string]int{
			"intelligence": 16,
		},
		Metadata: map[string]any{
			"builder_stage": "spellcasting_if_available",
		},
	}
	completion, ok := builderDeterministicGuidedChoiceCompletion(
		&character,
		"spellcasting_if_available",
		"mach das",
		"Für Magier auf Stufe 1 stehen im SRD 5.1 diese sinnvollen Zauberoptionen bereit. Wenn du möchtest, übernehme ich genau diese Auswahl direkt.",
	)
	if !ok {
		t.Fatal("expected guided completion to accept spellcasting confirmation")
	}
	rows, ok := completion.Patch.Metadata["spell_attacks"].(string)
	if !ok || !strings.Contains(rows, "Zaubertrick |") {
		t.Fatalf("expected structured spell attack rows in patch, got %#v", completion.Patch.Metadata["spell_attacks"])
	}
	if strings.Contains(rows, "Magierhand") {
		t.Fatalf("did not expect non-attack spell in spell attack rows, got %q", rows)
	}
	if !strings.Contains(rows, "Beschreibung:") {
		t.Fatalf("expected german spell descriptions in patch, got %q", rows)
	}
	if completion.Patch.HitPointMax != nil {
		t.Fatalf("did not expect hit point patch in spellcasting confirmation, got %#v", completion.Patch.HitPointMax)
	}
}

func TestBuilderDeterministicSpellcastingReplyHandlesEnglishListAndConfirmation(t *testing.T) {
	character := Character{
		ClassAndLevel: "Wizard, Level 1",
		Abilities: map[string]int{
			"intelligence": 16,
		},
		Metadata: map[string]any{
			"language": "en",
		},
	}
	reply, ok := builderDeterministicSpellcastingAdviceReply(character, "which spells should I take?")
	if !ok {
		t.Fatal("expected english spell advice")
	}
	if !strings.Contains(reply, "recommended") && !strings.Contains(strings.ToLower(reply), "recommended") {
		t.Fatalf("expected recommendation in english reply, got %q", reply)
	}
	_, patch, ok := builderDeterministicSpellcastingReply(character, "do that", "")
	if !ok {
		t.Fatal("expected english confirmation to be accepted")
	}
	rows, _ := patch.Metadata["spell_attacks"].(string)
	if !strings.Contains(rows, "Description:") {
		t.Fatalf("expected english spell descriptions, got %q", rows)
	}
	if !strings.Contains(rows, "Cantrip |") || !strings.Contains(rows, "Level 1 |") {
		t.Fatalf("expected english spell row labels, got %q", rows)
	}
	if !strings.Contains(rows, "Spell Attack") || !strings.Contains(rows, "Saving Throw") {
		t.Fatalf("expected english spell attack/save labels, got %q", rows)
	}
	reply = builderMaybeAppendStepConfirmation("I’ll apply the recommended spells.", patch)
	if !strings.Contains(reply, "Check briefly whether this looks right.") {
		t.Fatalf("expected english confirmation suffix, got %q", reply)
	}
}

func TestCurrentBuilderStageUsesSpellcastingFallbackForCleric(t *testing.T) {
	character := Character{
		Race:          "Mensch",
		ClassAndLevel: "Kleriker, Stufe 1",
		Features:      []string{"Zauberwirken", "Göttliche Domäne: Leben"},
		Alignment:     "Neutral",
		Background:    "Akolyth",
		Speed:         "30 Fuß",
		HitPointMax:   intPtr(10),
		Abilities: map[string]int{
			"strength":     10,
			"dexterity":    10,
			"constitution": 12,
			"intelligence": 10,
			"wisdom":       16,
			"charisma":     14,
		},
		Metadata: map[string]any{
			"concept":                    "Standhafter Tempelwächter",
			"creation_method":            "standardwerte",
			"personality_traits":         "Pflichtbewusst",
			"ideals":                     "Glaube",
			"bonds":                      "Tempel",
			"flaws":                      "starr",
			"starting_equipment":         []string{"Streitkolben"},
			"skill_proficiencies":        []string{"Religion", "Einsicht"},
			"saving_throw_proficiencies": []string{"Weisheit", "Charisma"},
			"hit_dice":                   "1W8",
			"senses":                     "passive Wahrnehmung 13",
		},
	}
	if stage := currentBuilderStage(character, nil); stage != "spellcasting_if_available" {
		t.Fatalf("expected spellcasting stage, got %s", stage)
	}
}

func TestBuilderDerivedStatsReplyComputesFighterDefaults(t *testing.T) {
	character := Character{
		Race:          "Mensch",
		ClassAndLevel: "Kämpfer, Stufe 1",
		Features:      []string{"Fighting Style: Defense", "Second Wind"},
		Abilities: map[string]int{
			"strength":     16,
			"dexterity":    15,
			"constitution": 16,
		},
		Metadata: map[string]any{
			"starting_equipment": []string{"Kettenhemd", "Langschwert und Schild", "leichte Armbrust mit 20 Bolzen", "Entdeckerausrüstung"},
		},
	}
	reply, patch, ok := builderDeterministicDerivedStatsReply(character, "ja mach das bitte")
	if !ok {
		reply, patch, ok = builderDeterministicDerivedStatsReply(character, "weiter")
	}
	if !ok {
		t.Fatal("expected derived stats reply")
	}
	if !strings.Contains(reply, "Rüstungsklasse 19") || !strings.Contains(reply, "Trefferpunkte 13") || !strings.Contains(reply, "Bewegungsrate 9 m") {
		t.Fatalf("unexpected reply: %s", reply)
	}
	if patch.ArmorClass == nil || *patch.ArmorClass != 19 {
		t.Fatalf("unexpected armor class patch: %#v", patch.ArmorClass)
	}
	if patch.HitPointMax == nil || *patch.HitPointMax != 13 {
		t.Fatalf("unexpected hit point max patch: %#v", patch.HitPointMax)
	}
	if patch.Speed == nil || *patch.Speed != "9 m" {
		t.Fatalf("unexpected speed patch: %#v", patch.Speed)
	}
	if patch.Proficiency == nil || *patch.Proficiency != "+2" {
		t.Fatalf("unexpected proficiency patch: %#v", patch.Proficiency)
	}
	if got := patch.Metadata["builder_stage"]; got != "combat" {
		t.Fatalf("unexpected next stage: %v", got)
	}
}

func TestBuilderCombatAdviceReplyListsFighterAttacks(t *testing.T) {
	character := Character{
		ClassAndLevel: "Kämpfer, Stufe 1",
		Abilities: map[string]int{
			"strength":  16,
			"dexterity": 15,
		},
	}
	reply, ok := builderDeterministicCombatAdviceReply(character, "welche Angriffe habe ich?")
	if !ok {
		t.Fatal("expected combat advice")
	}
	for _, expected := range []string{"Langschwert: Angriff +5", "Leichte Armbrust: Angriff +4"} {
		if !strings.Contains(reply, expected) {
			t.Fatalf("reply does not contain %q: %s", expected, reply)
		}
	}
}

func TestBuilderDeterministicCombatReplyRepairsMissingDerivedStatsAndAdvancesToReview(t *testing.T) {
	character := Character{
		Race:          "Mensch",
		ClassAndLevel: "Kämpfer, Stufe 1",
		Features:      []string{"Fighting Style: Defense", "Second Wind"},
		Abilities: map[string]int{
			"strength":     16,
			"dexterity":    15,
			"constitution": 16,
		},
		Metadata: map[string]any{
			"starting_equipment": []string{"Kettenhemd", "Langschwert und Schild", "leichte Armbrust mit 20 Bolzen", "Entdeckerausrüstung"},
		},
	}
	reply, patch, ok := builderDeterministicCombatReply(character, "ja mach das bitte", "")
	if !ok {
		t.Fatal("expected deterministic combat completion")
	}
	if !strings.Contains(reply, "Abschlussprüfung") {
		t.Fatalf("unexpected reply: %s", reply)
	}
	if patch.HitPointMax == nil || *patch.HitPointMax != 13 {
		t.Fatalf("expected hit point repair, got %#v", patch.HitPointMax)
	}
	if patch.ArmorClass == nil || *patch.ArmorClass != 19 {
		t.Fatalf("expected armor class repair, got %#v", patch.ArmorClass)
	}
	if patch.Speed == nil || *patch.Speed != "9 m" {
		t.Fatalf("expected speed repair, got %#v", patch.Speed)
	}
	if got := patch.Metadata["builder_stage"]; got != "review" {
		t.Fatalf("unexpected next stage: %v", got)
	}
	combatAttacks, _ := patch.Metadata["combat_attacks"].(string)
	if !strings.Contains(combatAttacks, "Langschwert | +2 | STR | 1,5 m | +5 | 1W8+3 | Hieb") {
		t.Fatalf("expected structured combat attacks, got %q", combatAttacks)
	}
	if !strings.Contains(combatAttacks, "Leichte Armbrust | +2 | DEX | 24/96 m | +4 | 1W8+2 | Stich") {
		t.Fatalf("expected structured ranged attack, got %q", combatAttacks)
	}
}

func TestBuilderDeterministicReviewReplyRepairsMissingCombatTable(t *testing.T) {
	character := Character{
		Race:          "Mensch",
		ClassAndLevel: "Kämpfer, Stufe 1",
		ArmorClass:    intPtr(19),
		HitPointMax:   intPtr(13),
		Speed:         "30 Fuß",
		Proficiency:   "+2",
		Abilities: map[string]int{
			"strength":  16,
			"dexterity": 15,
		},
		Metadata: map[string]any{
			"builder_stage":  "review",
			"combat_attacks": "",
		},
	}
	reply, patch, ok := builderDeterministicReviewReply(character, "da fehlt noch das Langschwert und die Armbrust")
	if !ok {
		t.Fatal("expected deterministic review repair")
	}
	if !strings.Contains(reply, "Angriffstabelle") {
		t.Fatalf("unexpected reply: %s", reply)
	}
	combatAttacks, _ := patch.Metadata["combat_attacks"].(string)
	if !strings.Contains(combatAttacks, "Langschwert | +2 | STR | 1,5 m | +5 | 1W8+3 | Hieb") {
		t.Fatalf("expected structured combat attacks, got %q", combatAttacks)
	}
	if got := patch.Metadata["builder_stage"]; got != "review" {
		t.Fatalf("unexpected next stage: %v", got)
	}
}

func TestBuilderStoryTransferRequestedNeedsStoryContext(t *testing.T) {
	if builderStoryTransferRequested("Ja, übernehmen wir sie.", "Auf Stufe 1 sind für Kämpfer im SRD 5.1 diese Klassenmerkmale relevant: Fighting Style und Second Wind.") {
		t.Fatal("feature confirmation must not trigger story transfer")
	}
	if !builderStoryTransferRequested("Ja, übernimm das bitte.", "Ich mache dir zuerst drei Richtungen fuer deinen Charakter.\n\n1. Pflichtruf...\n\n2. Spur...\n\n3. Heilmittel...\n\nWelche Richtung willst du nehmen?") {
		t.Fatal("story option confirmation should trigger story transfer in story context")
	}
}

func TestBuilderDeterministicClassFeatureReplySkipsSpellStepForFighter(t *testing.T) {
	character := Character{ClassAndLevel: "Kämpfer, Stufe 1"}
	reply, _, ok := builderDeterministicClassFeatureReply(character, "ja mach das bitte", "Für diesen Build empfehle ich Fighting Style: Defense und Second Wind.")
	if !ok {
		t.Fatal("expected deterministic class feature completion")
	}
	if strings.Contains(reply, "Zauberoptionen") {
		t.Fatalf("fighter reply must not mention spells: %s", reply)
	}
	if !strings.Contains(reply, "abgeleiteten Werte") {
		t.Fatalf("fighter reply should continue to derived stats: %s", reply)
	}
}
