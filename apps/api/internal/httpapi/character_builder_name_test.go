package httpapi

import (
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
