package httpapi

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type srdLevelClassProgression struct {
	FeaturesByLevel map[int][]string
}

var srdGermanLevelUpFeatureNames = map[string]string{
	"Spellcasting":              "Zauberwirken",
	"Sorcerous Origin":          "Zauberursprung",
	"Font of Magic":             "Quelle der Magie",
	"Metamagic":                 "Metamagie",
	"Sorcerous Origin feature":  "Merkmal des Zauberursprungs",
	"Sorcerous Restoration":     "Magische Wiederherstellung",
	"Fighting Style":            "Kampfstil",
	"Divine Smite":              "Göttliches Niederstrecken",
	"Divine Health":             "Göttliche Gesundheit",
	"Sacred Oath":               "Heiliger Schwur",
	"Channel Divinity":          "Göttliches Kanalisieren",
	"Sacred Oath feature":       "Merkmal des heiligen Schwurs",
	"Lay on Hands":              "Handauflegen",
	"Divine Sense":              "Göttliches Gespür",
	"Ability Score Improvement": "Attributssteigerung",
	"Otherworldly Patron":       "Übernatürlicher Patron",
	"Pact Boon":                 "Paktgabe",
	"Arcane Tradition":          "Arkane Tradition",
}

type srdSpellcastingSnapshot struct {
	MaxSpellLevel  int
	SlotsByLevel   []int
	PactSlots      int
	PactSlotLevel  int
	CantripsKnown  int
	SpellsKnown    int
	PreparedCount  int
	SpellbookTotal int
}

type srdLevelUpPreview struct {
	TargetClass      string
	ClassName        string
	CurrentLevel     int
	TargetLevel      int
	TotalLevel       int
	AverageHPGain    int
	ProficiencyBonus int
	FeaturesGained   []string
	ChoicePrompts    []string
	Spellcasting     srdSpellcastingSnapshot
}

type characterClassLevel struct {
	ClassName string
	Level     int
}

var fullCasterSlotsByLevel = map[int][]int{
	1:  {2},
	2:  {3},
	3:  {4, 2},
	4:  {4, 3},
	5:  {4, 3, 2},
	6:  {4, 3, 3},
	7:  {4, 3, 3, 1},
	8:  {4, 3, 3, 2},
	9:  {4, 3, 3, 3, 1},
	10: {4, 3, 3, 3, 2},
	11: {4, 3, 3, 3, 2, 1},
	12: {4, 3, 3, 3, 2, 1},
	13: {4, 3, 3, 3, 2, 1, 1},
	14: {4, 3, 3, 3, 2, 1, 1},
	15: {4, 3, 3, 3, 2, 1, 1, 1},
	16: {4, 3, 3, 3, 2, 1, 1, 1},
	17: {4, 3, 3, 3, 2, 1, 1, 1, 1},
	18: {4, 3, 3, 3, 3, 1, 1, 1, 1},
	19: {4, 3, 3, 3, 3, 2, 1, 1, 1},
	20: {4, 3, 3, 3, 3, 2, 2, 1, 1},
}

var halfCasterSlotsByLevel = map[int][]int{
	1:  {},
	2:  {2},
	3:  {3},
	4:  {3},
	5:  {4, 2},
	6:  {4, 2},
	7:  {4, 3},
	8:  {4, 3},
	9:  {4, 3, 2},
	10: {4, 3, 2},
	11: {4, 3, 3},
	12: {4, 3, 3},
	13: {4, 3, 3, 1},
	14: {4, 3, 3, 1},
	15: {4, 3, 3, 2},
	16: {4, 3, 3, 2},
	17: {4, 3, 3, 3, 1},
	18: {4, 3, 3, 3, 1},
	19: {4, 3, 3, 3, 2},
	20: {4, 3, 3, 3, 2},
}

var warlockPactSlotsByLevel = map[int]struct {
	Slots int
	Level int
}{
	1:  {Slots: 1, Level: 1},
	2:  {Slots: 2, Level: 1},
	3:  {Slots: 2, Level: 2},
	4:  {Slots: 2, Level: 2},
	5:  {Slots: 2, Level: 3},
	6:  {Slots: 2, Level: 3},
	7:  {Slots: 2, Level: 4},
	8:  {Slots: 2, Level: 4},
	9:  {Slots: 2, Level: 5},
	10: {Slots: 2, Level: 5},
	11: {Slots: 3, Level: 5},
	12: {Slots: 3, Level: 5},
	13: {Slots: 3, Level: 5},
	14: {Slots: 3, Level: 5},
	15: {Slots: 3, Level: 5},
	16: {Slots: 3, Level: 5},
	17: {Slots: 4, Level: 5},
	18: {Slots: 4, Level: 5},
	19: {Slots: 4, Level: 5},
	20: {Slots: 4, Level: 5},
}

var cantripsKnownByClassLevel = map[string]map[int]int{
	"Barde": {
		1: 2, 2: 2, 3: 2, 4: 3, 5: 3, 6: 3, 7: 3, 8: 3, 9: 3, 10: 4,
		11: 4, 12: 4, 13: 4, 14: 4, 15: 4, 16: 4, 17: 4, 18: 4, 19: 4, 20: 4,
	},
	"Kleriker": {
		1: 3, 2: 3, 3: 3, 4: 4, 5: 4, 6: 4, 7: 4, 8: 4, 9: 4, 10: 5,
		11: 5, 12: 5, 13: 5, 14: 5, 15: 5, 16: 5, 17: 5, 18: 5, 19: 5, 20: 5,
	},
	"Druide": {
		1: 2, 2: 2, 3: 2, 4: 3, 5: 3, 6: 3, 7: 3, 8: 3, 9: 3, 10: 4,
		11: 4, 12: 4, 13: 4, 14: 4, 15: 4, 16: 4, 17: 4, 18: 4, 19: 4, 20: 4,
	},
	"Zauberer": {
		1: 4, 2: 4, 3: 4, 4: 5, 5: 5, 6: 5, 7: 5, 8: 5, 9: 5, 10: 6,
		11: 6, 12: 6, 13: 6, 14: 6, 15: 6, 16: 6, 17: 6, 18: 6, 19: 6, 20: 6,
	},
	"Hexenmeister": {
		1: 2, 2: 2, 3: 2, 4: 3, 5: 3, 6: 3, 7: 3, 8: 3, 9: 3, 10: 4,
		11: 4, 12: 4, 13: 4, 14: 4, 15: 4, 16: 4, 17: 4, 18: 4, 19: 4, 20: 4,
	},
	"Magier": {
		1: 3, 2: 3, 3: 3, 4: 4, 5: 4, 6: 4, 7: 4, 8: 4, 9: 4, 10: 5,
		11: 5, 12: 5, 13: 5, 14: 5, 15: 5, 16: 5, 17: 5, 18: 5, 19: 5, 20: 5,
	},
}

var spellsKnownByClassLevel = map[string]map[int]int{
	"Barde": {
		1: 4, 2: 5, 3: 6, 4: 7, 5: 8, 6: 9, 7: 10, 8: 11, 9: 12, 10: 14,
		11: 15, 12: 15, 13: 16, 14: 18, 15: 19, 16: 19, 17: 20, 18: 22, 19: 22, 20: 22,
	},
	"Zauberer": {
		1: 2, 2: 3, 3: 4, 4: 5, 5: 6, 6: 7, 7: 8, 8: 9, 9: 10, 10: 11,
		11: 12, 12: 12, 13: 13, 14: 13, 15: 14, 16: 14, 17: 15, 18: 15, 19: 15, 20: 15,
	},
	"Hexenmeister": {
		1: 2, 2: 3, 3: 4, 4: 5, 5: 6, 6: 7, 7: 8, 8: 9, 9: 10, 10: 10,
		11: 11, 12: 11, 13: 12, 14: 12, 15: 13, 16: 13, 17: 14, 18: 14, 19: 15, 20: 15,
	},
	"Waldläufer": {
		1: 0, 2: 2, 3: 3, 4: 3, 5: 4, 6: 4, 7: 5, 8: 5, 9: 6, 10: 6,
		11: 7, 12: 7, 13: 8, 14: 8, 15: 9, 16: 9, 17: 10, 18: 10, 19: 11, 20: 11,
	},
}

var srdLevelClassProgressions = map[string]srdLevelClassProgression{
	"Barbar": {FeaturesByLevel: map[int][]string{
		1:  {"Rage", "Unarmored Defense"},
		2:  {"Reckless Attack", "Danger Sense"},
		3:  {"Primal Path"},
		4:  {"Ability Score Improvement"},
		5:  {"Extra Attack", "Fast Movement"},
		6:  {"Primal Path feature"},
		7:  {"Feral Instinct"},
		8:  {"Ability Score Improvement"},
		9:  {"Brutal Critical (1 die)", "Rage Damage +3"},
		10: {"Primal Path feature"},
		11: {"Relentless Rage"},
		12: {"Ability Score Improvement"},
		13: {"Brutal Critical (2 dice)"},
		14: {"Primal Path feature"},
		15: {"Persistent Rage"},
		16: {"Ability Score Improvement"},
		17: {"Brutal Critical (3 dice)", "Rage Damage +4"},
		18: {"Indomitable Might"},
		19: {"Ability Score Improvement"},
		20: {"Primal Champion", "Unlimited Rages"},
	}},
	"Barde": {FeaturesByLevel: map[int][]string{
		1:  {"Spellcasting", "Bardic Inspiration (d6)"},
		2:  {"Jack of All Trades", "Song of Rest (d6)"},
		3:  {"Bard College", "Expertise"},
		4:  {"Ability Score Improvement"},
		5:  {"Font of Inspiration", "Bardic Inspiration (d8)"},
		6:  {"Countercharm", "Bard College feature"},
		8:  {"Ability Score Improvement"},
		9:  {"Song of Rest (d8)"},
		10: {"Bardic Inspiration (d10)", "Expertise", "Magical Secrets"},
		12: {"Ability Score Improvement"},
		13: {"Song of Rest (d10)"},
		14: {"Bard College feature", "Magical Secrets"},
		15: {"Bardic Inspiration (d12)"},
		16: {"Ability Score Improvement"},
		17: {"Song of Rest (d12)"},
		18: {"Magical Secrets"},
		19: {"Ability Score Improvement"},
		20: {"Superior Inspiration"},
	}},
	"Kleriker": {FeaturesByLevel: map[int][]string{
		1:  {"Spellcasting", "Divine Domain", "Domain feature"},
		2:  {"Channel Divinity (1/rest)", "Domain feature"},
		4:  {"Ability Score Improvement"},
		5:  {"Destroy Undead (CR 1/2)"},
		6:  {"Channel Divinity (2/rest)", "Domain feature"},
		8:  {"Ability Score Improvement", "Domain feature", "Destroy Undead (CR 1)"},
		10: {"Divine Intervention"},
		11: {"Destroy Undead (CR 2)"},
		12: {"Ability Score Improvement"},
		14: {"Destroy Undead (CR 3)"},
		16: {"Ability Score Improvement"},
		17: {"Domain feature", "Destroy Undead (CR 4)"},
		18: {"Channel Divinity (3/rest)"},
		19: {"Ability Score Improvement"},
		20: {"Improved Divine Intervention"},
	}},
	"Druide": {FeaturesByLevel: map[int][]string{
		1:  {"Druidic", "Spellcasting"},
		2:  {"Wild Shape", "Druid Circle"},
		4:  {"Wild Shape Improvement (swim)", "Ability Score Improvement"},
		6:  {"Druid Circle feature"},
		8:  {"Wild Shape Improvement (fly)", "Ability Score Improvement"},
		10: {"Druid Circle feature"},
		12: {"Ability Score Improvement"},
		14: {"Druid Circle feature"},
		16: {"Ability Score Improvement"},
		18: {"Timeless Body", "Beast Spells"},
		19: {"Ability Score Improvement"},
		20: {"Archdruid"},
	}},
	"Kämpfer": {FeaturesByLevel: map[int][]string{
		1:  {"Fighting Style", "Second Wind"},
		2:  {"Action Surge"},
		3:  {"Martial Archetype"},
		4:  {"Ability Score Improvement"},
		5:  {"Extra Attack"},
		6:  {"Ability Score Improvement"},
		7:  {"Martial Archetype feature"},
		8:  {"Ability Score Improvement"},
		9:  {"Indomitable"},
		10: {"Martial Archetype feature"},
		11: {"Extra Attack (2)"},
		12: {"Ability Score Improvement"},
		13: {"Indomitable (2 uses)"},
		14: {"Ability Score Improvement"},
		15: {"Martial Archetype feature"},
		16: {"Ability Score Improvement"},
		17: {"Action Surge (2 uses)", "Indomitable (3 uses)"},
		18: {"Martial Archetype feature"},
		19: {"Ability Score Improvement"},
		20: {"Extra Attack (3)"},
	}},
	"Mönch": {FeaturesByLevel: map[int][]string{
		1:  {"Unarmored Defense", "Martial Arts"},
		2:  {"Ki", "Unarmored Movement"},
		3:  {"Monastic Tradition", "Deflect Missiles"},
		4:  {"Slow Fall", "Ability Score Improvement"},
		5:  {"Extra Attack", "Stunning Strike"},
		6:  {"Ki-Empowered Strikes", "Monastic Tradition feature", "Unarmored Movement Improvement"},
		7:  {"Evasion", "Stillness of Mind"},
		8:  {"Ability Score Improvement"},
		9:  {"Unarmored Movement Improvement"},
		10: {"Purity of Body"},
		11: {"Monastic Tradition feature"},
		12: {"Ability Score Improvement"},
		13: {"Tongue of the Sun and Moon"},
		14: {"Diamond Soul"},
		15: {"Timeless Body"},
		16: {"Ability Score Improvement"},
		17: {"Monastic Tradition feature"},
		18: {"Empty Body"},
		19: {"Ability Score Improvement"},
		20: {"Perfect Self"},
	}},
	"Paladin": {FeaturesByLevel: map[int][]string{
		1:  {"Divine Sense", "Lay on Hands"},
		2:  {"Fighting Style", "Spellcasting", "Divine Smite"},
		3:  {"Divine Health", "Sacred Oath", "Channel Divinity", "Sacred Oath feature"},
		4:  {"Ability Score Improvement"},
		5:  {"Extra Attack"},
		6:  {"Aura of Protection"},
		7:  {"Sacred Oath feature"},
		8:  {"Ability Score Improvement"},
		10: {"Aura of Courage"},
		11: {"Improved Divine Smite"},
		12: {"Ability Score Improvement"},
		14: {"Cleansing Touch"},
		15: {"Sacred Oath feature"},
		16: {"Ability Score Improvement"},
		18: {"Aura improvements (30 ft)"},
		19: {"Ability Score Improvement"},
		20: {"Sacred Oath feature"},
	}},
	"Waldläufer": {FeaturesByLevel: map[int][]string{
		1:  {"Favored Enemy", "Natural Explorer"},
		2:  {"Fighting Style", "Spellcasting"},
		3:  {"Ranger Archetype", "Primeval Awareness"},
		4:  {"Ability Score Improvement"},
		5:  {"Extra Attack"},
		6:  {"Favored Enemy Improvement", "Natural Explorer Improvement"},
		7:  {"Ranger Archetype feature"},
		8:  {"Land's Stride", "Ability Score Improvement"},
		10: {"Natural Explorer Improvement", "Hide in Plain Sight"},
		11: {"Ranger Archetype feature"},
		12: {"Ability Score Improvement"},
		14: {"Favored Enemy Improvement", "Vanish"},
		15: {"Ranger Archetype feature"},
		16: {"Ability Score Improvement"},
		18: {"Feral Senses"},
		19: {"Ability Score Improvement"},
		20: {"Foe Slayer"},
	}},
	"Schurke": {FeaturesByLevel: map[int][]string{
		1:  {"Expertise", "Sneak Attack (1d6)", "Thieves' Cant"},
		2:  {"Cunning Action"},
		3:  {"Roguish Archetype", "Sneak Attack (2d6)"},
		4:  {"Ability Score Improvement"},
		5:  {"Uncanny Dodge", "Sneak Attack (3d6)"},
		6:  {"Expertise"},
		7:  {"Evasion", "Sneak Attack (4d6)"},
		8:  {"Ability Score Improvement"},
		9:  {"Roguish Archetype feature", "Sneak Attack (5d6)"},
		10: {"Ability Score Improvement"},
		11: {"Reliable Talent", "Sneak Attack (6d6)"},
		12: {"Ability Score Improvement"},
		13: {"Roguish Archetype feature", "Sneak Attack (7d6)"},
		14: {"Blindsense"},
		15: {"Slippery Mind", "Sneak Attack (8d6)"},
		16: {"Ability Score Improvement"},
		17: {"Roguish Archetype feature", "Sneak Attack (9d6)"},
		18: {"Elusive"},
		19: {"Ability Score Improvement", "Sneak Attack (10d6)"},
		20: {"Stroke of Luck"},
	}},
	"Zauberer": {FeaturesByLevel: map[int][]string{
		1:  {"Spellcasting", "Sorcerous Origin"},
		2:  {"Font of Magic"},
		3:  {"Metamagic"},
		4:  {"Ability Score Improvement"},
		6:  {"Sorcerous Origin feature"},
		8:  {"Ability Score Improvement"},
		10: {"Metamagic"},
		12: {"Ability Score Improvement"},
		14: {"Sorcerous Origin feature"},
		16: {"Ability Score Improvement"},
		17: {"Metamagic"},
		18: {"Sorcerous Origin feature"},
		19: {"Ability Score Improvement"},
		20: {"Sorcerous Restoration"},
	}},
	"Hexenmeister": {FeaturesByLevel: map[int][]string{
		1:  {"Otherworldly Patron", "Pact Magic"},
		2:  {"Eldritch Invocations"},
		3:  {"Pact Boon"},
		4:  {"Ability Score Improvement"},
		6:  {"Otherworldly Patron feature"},
		8:  {"Ability Score Improvement"},
		10: {"Otherworldly Patron feature"},
		11: {"Mystic Arcanum (6th level)"},
		12: {"Ability Score Improvement"},
		13: {"Mystic Arcanum (7th level)"},
		14: {"Otherworldly Patron feature"},
		15: {"Mystic Arcanum (8th level)"},
		16: {"Ability Score Improvement"},
		17: {"Mystic Arcanum (9th level)"},
		19: {"Ability Score Improvement"},
		20: {"Eldritch Master"},
	}},
	"Magier": {FeaturesByLevel: map[int][]string{
		1:  {"Spellcasting", "Arcane Recovery"},
		2:  {"Arcane Tradition"},
		4:  {"Ability Score Improvement"},
		6:  {"Arcane Tradition feature"},
		8:  {"Ability Score Improvement"},
		10: {"Arcane Tradition feature"},
		12: {"Ability Score Improvement"},
		14: {"Arcane Tradition feature"},
		16: {"Ability Score Improvement"},
		18: {"Spell Mastery"},
		19: {"Ability Score Improvement"},
		20: {"Signature Spells"},
	}},
}

func builderAverageHitDieIncrease(hitDie string) int {
	value := strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(hitDie)), "W")
	size, err := strconv.Atoi(value)
	if err != nil || size <= 0 {
		return 0
	}
	return size/2 + 1
}

func parseCharacterClassLevels(text string) []characterClassLevel {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	normalized := text
	for _, sep := range []string{" / ", "/", " + ", "+", ";", "\n"} {
		normalized = strings.ReplaceAll(normalized, sep, "|")
	}
	segments := strings.Split(normalized, "|")
	items := make([]characterClassLevel, 0, len(segments))
	seen := map[string]bool{}
	for _, rawSegment := range segments {
		segment := strings.TrimSpace(rawSegment)
		if segment == "" {
			continue
		}
		classRule, ok := builderClassRuleForText(segment)
		if !ok {
			continue
		}
		matches := regexp.MustCompile(`\b(\d{1,2})\b`).FindStringSubmatch(segment)
		if len(matches) < 2 {
			continue
		}
		level, err := strconv.Atoi(matches[1])
		if err != nil || level <= 0 {
			continue
		}
		key := strings.ToLower(classRule.ClassName)
		if seen[key] {
			continue
		}
		seen[key] = true
		items = append(items, characterClassLevel{ClassName: classRule.ClassName, Level: level})
	}
	if len(items) > 0 {
		return items
	}
	classRule, ok := builderClassRuleForText(text)
	if !ok {
		return nil
	}
	matches := regexp.MustCompile(`\b(\d{1,2})\b`).FindStringSubmatch(text)
	if len(matches) < 2 {
		return []characterClassLevel{{ClassName: classRule.ClassName, Level: 1}}
	}
	level, err := strconv.Atoi(matches[1])
	if err != nil || level <= 0 {
		return nil
	}
	return []characterClassLevel{{ClassName: classRule.ClassName, Level: level}}
}

func totalCharacterLevel(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	entries := parseCharacterClassLevels(text)
	if len(entries) > 0 {
		total := 0
		for _, entry := range entries {
			total += entry.Level
		}
		if total > 0 {
			return total
		}
	}
	matches := regexp.MustCompile(`\b(\d{1,2})\b`).FindStringSubmatch(text)
	if len(matches) < 2 {
		return 0
	}
	level, err := strconv.Atoi(matches[1])
	if err != nil || level <= 0 {
		return 0
	}
	if level > 20 {
		return 20
	}
	return level
}

func classLevelForName(text string, className string) int {
	for _, entry := range parseCharacterClassLevels(text) {
		if strings.EqualFold(entry.ClassName, className) {
			return entry.Level
		}
	}
	return 0
}

func characterHasMulticlass(text string) bool {
	return len(parseCharacterClassLevels(text)) > 1
}

func singleOrPrimaryClassName(text string) string {
	entries := parseCharacterClassLevels(text)
	if len(entries) == 0 {
		if rule, ok := builderClassRuleForText(text); ok {
			return rule.ClassName
		}
		return ""
	}
	return entries[0].ClassName
}

func incrementClassLevelText(currentText string, className string) string {
	entries := parseCharacterClassLevels(currentText)
	if len(entries) == 0 {
		return fmt.Sprintf("%s 1", className)
	}
	updated := false
	for idx := range entries {
		if strings.EqualFold(entries[idx].ClassName, className) {
			entries[idx].Level++
			updated = true
			break
		}
	}
	if !updated {
		entries = append(entries, characterClassLevel{ClassName: className, Level: 1})
	}
	parts := make([]string, 0, len(entries))
	for _, entry := range entries {
		parts = append(parts, fmt.Sprintf("%s %d", entry.ClassName, entry.Level))
	}
	return strings.Join(parts, " / ")
}

func classLevelsWithOverride(text string, className string, level int) []characterClassLevel {
	entries := parseCharacterClassLevels(text)
	if len(entries) == 0 {
		return []characterClassLevel{{ClassName: className, Level: level}}
	}
	updated := false
	for idx := range entries {
		if strings.EqualFold(entries[idx].ClassName, className) {
			entries[idx].Level = level
			updated = true
			break
		}
	}
	if !updated {
		entries = append(entries, characterClassLevel{ClassName: className, Level: level})
	}
	return entries
}

func multiclassCasterLevelFromEntries(entries []characterClassLevel) int {
	total := 0
	for _, entry := range entries {
		switch entry.ClassName {
		case "Barde", "Kleriker", "Druide", "Zauberer", "Magier":
			total += entry.Level
		case "Paladin", "Waldläufer":
			total += entry.Level / 2
		}
	}
	return total
}

func builderClassAndLevelText(className string, currentText string, level int) string {
	currentPrimary := singleOrPrimaryClassName(currentText)
	if characterHasMulticlass(currentText) || (currentPrimary != "" && !strings.EqualFold(currentPrimary, className)) {
		return incrementClassLevelText(currentText, className)
	}
	if strings.Contains(strings.ToLower(currentText), "stufe") || strings.Contains(currentText, ",") {
		return fmt.Sprintf("%s, Stufe %d", className, level)
	}
	return fmt.Sprintf("%s %d", className, level)
}

func builderLevelUpPreview(character Character, targetClass string, targetLevel int) (srdLevelUpPreview, bool) {
	if strings.TrimSpace(targetClass) == "" {
		targetClass = singleOrPrimaryClassName(character.ClassAndLevel)
	}
	classEntry, ok := srdClassCatalogEntryByName(targetClass)
	if !ok {
		return srdLevelUpPreview{}, false
	}
	classRule := classEntry.ClassRule
	currentLevel := classLevelForName(character.ClassAndLevel, classRule.ClassName)
	if targetLevel <= currentLevel || targetLevel > 20 {
		return srdLevelUpPreview{}, false
	}
	progression, ok := srdLevelClassProgressions[classRule.ClassName]
	if !ok {
		return srdLevelUpPreview{}, false
	}
	conMod := abilityModifierFromAny(character.Abilities["constitution"])
	averageGain := 0
	for level := currentLevel + 1; level <= targetLevel; level++ {
		averageGain += builderAverageHitDieIncrease(classRule.HitDie) + conMod
	}
	features := make([]string, 0)
	for level := currentLevel + 1; level <= targetLevel; level++ {
		features = append(features, progression.FeaturesByLevel[level]...)
	}
	features = uniquePreserveOrder(features)
	features = localizedLevelUpFeatureNames(features, builderCharacterLanguage(&character))
	choices := builderLevelUpChoicePrompts(classRule.ClassName, currentLevel, targetLevel)
	currentTotalLevel := totalCharacterLevel(character.ClassAndLevel)
	targetTotalLevel := currentTotalLevel + (targetLevel - currentLevel)
	currentCasterLevel := multiclassCasterLevelFromEntries(classLevelsWithOverride(character.ClassAndLevel, classRule.ClassName, currentLevel))
	targetCasterLevel := multiclassCasterLevelFromEntries(classLevelsWithOverride(character.ClassAndLevel, classRule.ClassName, targetLevel))
	currentSnapshot := builderSpellcastingSnapshotForClass(classRule.ClassName, currentLevel, currentTotalLevel, currentCasterLevel, character)
	targetSnapshot := builderSpellcastingSnapshotForClass(classRule.ClassName, targetLevel, targetTotalLevel, targetCasterLevel, character)
	choices = append(choices, builderLevelUpSpellChoicePrompts(classRule.ClassName, currentSnapshot, targetSnapshot)...)
	choices = uniquePreserveOrder(choices)
	return srdLevelUpPreview{
		TargetClass:      classRule.ClassName,
		ClassName:        classRule.ClassName,
		CurrentLevel:     currentLevel,
		TargetLevel:      targetLevel,
		TotalLevel:       targetTotalLevel,
		AverageHPGain:    averageGain,
		ProficiencyBonus: proficiencyBonusForLevel(targetTotalLevel),
		FeaturesGained:   features,
		ChoicePrompts:    choices,
		Spellcasting:     targetSnapshot,
	}, true
}

func builderLevelUpSpellChoicePrompts(className string, current srdSpellcastingSnapshot, target srdSpellcastingSnapshot) []string {
	choices := make([]string, 0, 4)
	if target.CantripsKnown > current.CantripsKnown {
		choices = append(choices, fmt.Sprintf("Wähle %d neue Zaubertricks.", target.CantripsKnown-current.CantripsKnown))
	}
	if target.SpellsKnown > current.SpellsKnown {
		choices = append(choices, fmt.Sprintf("Wähle %d neue bekannte Zauber.", target.SpellsKnown-current.SpellsKnown))
	}
	if className == "Magier" && target.SpellbookTotal > current.SpellbookTotal {
		choices = append(choices, fmt.Sprintf("Füge dem Zauberbuch %d neue Zauber hinzu.", target.SpellbookTotal-current.SpellbookTotal))
	}
	if target.PreparedCount > 0 {
		choices = append(choices, fmt.Sprintf("Lege vorbereitete Zauber bis insgesamt %d fest.", target.PreparedCount))
	}
	return choices
}

func localizedLevelUpFeatureNames(items []string, language string) []string {
	if normalizeUILanguage(language) != "de" {
		return uniquePreserveOrder(items)
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if translated, ok := srdGermanLevelUpFeatureNames[trimmed]; ok {
			result = append(result, translated)
		} else {
			result = append(result, trimmed)
		}
	}
	return uniquePreserveOrder(result)
}

func builderExpectedChoiceCount(choice string) (int, string) {
	match := regexp.MustCompile(`\b(\d+)\b`).FindStringSubmatch(choice)
	if len(match) < 2 {
		return 0, ""
	}
	count, err := strconv.Atoi(match[1])
	if err != nil || count <= 0 {
		return 0, ""
	}
	normalized := normalizeBuilderIntentText(choice)
	switch {
	case strings.Contains(normalized, "zaubertrick"), strings.Contains(normalized, "cantrip"):
		return count, "cantrip"
	case strings.Contains(normalized, "bekannte zauber"), strings.Contains(normalized, "known spell"), strings.Contains(normalized, "zauberbuch"), strings.Contains(normalized, "spellbook"), strings.Contains(normalized, "vorbereitete zauber"), strings.Contains(normalized, "prepared spells"):
		return count, "spell"
	default:
		return 0, ""
	}
}

func builderAllowedSpellChoices(preview srdLevelUpPreview, kind string, language string) []string {
	entries := builderSRDSpellsForClassAtLevelLocalized(preview.TargetClass, preview.Spellcasting.MaxSpellLevel, language)
	cantrips, spells := splitSpellNamesForMaxLevel(entries)
	switch kind {
	case "cantrip":
		return cantrips
	case "spell":
		return spells
	default:
		return nil
	}
}

func builderParseSpellLikeSelections(message string, kind string, preview srdLevelUpPreview, language string) []string {
	allowed := builderAllowedSpellChoices(preview, kind, language)
	if len(allowed) == 0 {
		return nil
	}
	allowedSet := map[string]string{}
	for _, name := range allowed {
		canonical := canonicalSpellName(name, language)
		if canonical == "" {
			continue
		}
		local := localizedSpellName(canonical, language)
		allowedSet[normalizeBuilderIntentText(local)] = local
		allowedSet[normalizeBuilderIntentText(canonical)] = local
	}
	candidates := strings.FieldsFunc(message, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n'
	})
	selected := make([]string, 0, len(candidates))
	seen := map[string]bool{}
	for _, candidate := range candidates {
		key := normalizeBuilderIntentText(candidate)
		if key == "" {
			continue
		}
		if localized, ok := allowedSet[key]; ok {
			canonicalKey := normalizeBuilderIntentText(localized)
			if !seen[canonicalKey] {
				seen[canonicalKey] = true
				selected = append(selected, localized)
			}
		}
	}
	return selected
}

func builderParseLevelUpChoiceAnswer(choice string, latestUserMessage string, character Character, preview srdLevelUpPreview) (string, string, bool) {
	answer := strings.TrimSpace(latestUserMessage)
	if answer == "" {
		return "", "", false
	}
	required, kind := builderExpectedChoiceCount(choice)
	if required <= 0 || kind == "" {
		return answer, "", true
	}
	selected := builderParseSpellLikeSelections(answer, kind, preview, builderCharacterLanguage(&character))
	if len(selected) != required {
		label := "Einträge"
		switch kind {
		case "cantrip":
			label = "Zaubertricks"
		case "spell":
			label = "Zauber"
		}
		return "", fmt.Sprintf("Für diesen Wahlpunkt brauche ich genau %d %s. Nenne genau %d gültige %s aus dem aktuellen SRD-Stand.", required, label, required, label), false
	}
	return strings.Join(selected, ", "), "", true
}

func builderLevelUpChoicePrompts(className string, currentLevel int, targetLevel int) []string {
	choices := make([]string, 0, 4)
	for level := currentLevel + 1; level <= targetLevel; level++ {
		switch className {
		case "Barbar":
			if level == 3 {
				choices = append(choices, "Wähle einen Pfad des Berserkers.")
			}
		case "Barde":
			if level == 3 {
				choices = append(choices, "Wähle ein Bardenkollegium.")
			}
		case "Kleriker":
			if level == 1 {
				choices = append(choices, "Wähle eine göttliche Domäne.")
			}
		case "Druide":
			if level == 2 {
				choices = append(choices, "Wähle einen Druidenzirkel.")
			}
		case "Kämpfer":
			if level == 3 {
				choices = append(choices, "Wähle einen kämpferischen Archetyp.")
			}
		case "Mönch":
			if level == 3 {
				choices = append(choices, "Wähle eine klösterliche Tradition.")
			}
		case "Paladin":
			if level == 3 {
				choices = append(choices, "Wähle einen heiligen Schwur.")
			}
		case "Waldläufer":
			if level == 3 {
				choices = append(choices, "Wähle einen Waldläufer-Archetyp.")
			}
		case "Schurke":
			if level == 3 {
				choices = append(choices, "Wähle einen schurkischen Archetyp.")
			}
		case "Zauberer":
			if level == 1 {
				choices = append(choices, "Wähle einen Zauberursprung.")
			}
		case "Hexenmeister":
			if level == 1 {
				choices = append(choices, "Wähle einen übernatürlichen Patron.")
			}
			if level == 3 {
				choices = append(choices, "Wähle eine Paktgabe.")
			}
		case "Magier":
			if level == 2 {
				choices = append(choices, "Wähle eine arkane Tradition.")
			}
		}
		if isAbilityScoreImprovementLevel(className, level) {
			choices = append(choices, fmt.Sprintf("Wähle auf Stufe %d eine Attributssteigerung oder ein Talent.", level))
		}
	}
	return uniquePreserveOrder(choices)
}

func isAbilityScoreImprovementLevel(className string, level int) bool {
	levels := map[string]map[int]bool{
		"Barbar":       {4: true, 8: true, 12: true, 16: true, 19: true},
		"Barde":        {4: true, 8: true, 12: true, 16: true, 19: true},
		"Kleriker":     {4: true, 8: true, 12: true, 16: true, 19: true},
		"Druide":       {4: true, 8: true, 12: true, 16: true, 19: true},
		"Kämpfer":      {4: true, 6: true, 8: true, 12: true, 14: true, 16: true, 19: true},
		"Mönch":        {4: true, 8: true, 12: true, 16: true, 19: true},
		"Paladin":      {4: true, 8: true, 12: true, 16: true, 19: true},
		"Waldläufer":   {4: true, 8: true, 12: true, 16: true, 19: true},
		"Schurke":      {4: true, 8: true, 10: true, 12: true, 16: true, 19: true},
		"Zauberer":     {4: true, 8: true, 12: true, 16: true, 19: true},
		"Hexenmeister": {4: true, 8: true, 12: true, 16: true, 19: true},
		"Magier":       {4: true, 8: true, 12: true, 16: true, 19: true},
	}
	if perClass, ok := levels[className]; ok {
		return perClass[level]
	}
	return false
}

func proficiencyBonusForLevel(level int) int {
	switch {
	case level <= 0:
		return 0
	case level <= 4:
		return 2
	case level <= 8:
		return 3
	case level <= 12:
		return 4
	case level <= 16:
		return 5
	default:
		return 6
	}
}

func builderSpellcastingSnapshotForClass(className string, level int, totalLevel int, effectiveCasterLevel int, character Character) srdSpellcastingSnapshot {
	snapshot := srdSpellcastingSnapshot{}
	switch className {
	case "Barde", "Kleriker", "Druide", "Zauberer", "Magier":
		if effectiveCasterLevel <= 0 {
			effectiveCasterLevel = level
		}
		snapshot.SlotsByLevel = append([]int{}, fullCasterSlotsByLevel[effectiveCasterLevel]...)
		snapshot.MaxSpellLevel = len(snapshot.SlotsByLevel)
	case "Paladin", "Waldläufer":
		if effectiveCasterLevel <= 0 {
			effectiveCasterLevel = level / 2
		}
		snapshot.SlotsByLevel = append([]int{}, fullCasterSlotsByLevel[effectiveCasterLevel]...)
		snapshot.MaxSpellLevel = len(snapshot.SlotsByLevel)
	case "Hexenmeister":
		if pact, ok := warlockPactSlotsByLevel[level]; ok {
			snapshot.PactSlots = pact.Slots
			snapshot.PactSlotLevel = pact.Level
			snapshot.MaxSpellLevel = pact.Level
		}
	}
	if known, ok := cantripsKnownByClassLevel[className][level]; ok {
		snapshot.CantripsKnown = known
	}
	if known, ok := spellsKnownByClassLevel[className][level]; ok {
		snapshot.SpellsKnown = known
	}
	switch className {
	case "Kleriker", "Druide":
		mod := 0
		if className == "Kleriker" {
			mod = abilityModifierFromAny(character.Abilities["wisdom"])
		} else {
			mod = abilityModifierFromAny(character.Abilities["wisdom"])
		}
		snapshot.PreparedCount = maxInt(1, level+mod)
	case "Paladin":
		if level >= 2 {
			snapshot.PreparedCount = maxInt(1, level/2+abilityModifierFromAny(character.Abilities["charisma"]))
		}
	case "Magier":
		snapshot.SpellbookTotal = 6 + maxInt(0, (level-1)*2)
		snapshot.PreparedCount = maxInt(1, level+abilityModifierFromAny(character.Abilities["intelligence"]))
	}
	return snapshot
}

func multiclassCasterLevel(text string) int {
	return multiclassCasterLevelFromEntries(parseCharacterClassLevels(text))
}

func builderSpellSlotSummary(snapshot srdSpellcastingSnapshot) string {
	parts := make([]string, 0, 3)
	if snapshot.PactSlots > 0 {
		parts = append(parts, fmt.Sprintf("%d pact slot(s) at level %d", snapshot.PactSlots, snapshot.PactSlotLevel))
	}
	if len(snapshot.SlotsByLevel) > 0 {
		slotParts := make([]string, 0, len(snapshot.SlotsByLevel))
		for idx, count := range snapshot.SlotsByLevel {
			if count <= 0 {
				continue
			}
			slotParts = append(slotParts, fmt.Sprintf("L%d:%d", idx+1, count))
		}
		if len(slotParts) > 0 {
			parts = append(parts, "slots "+strings.Join(slotParts, ", "))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "; ")
}

func builderLevelUpSuggestedSpells(character Character, className string, snapshot srdSpellcastingSnapshot) []string {
	if strings.TrimSpace(className) == "" || snapshot.MaxSpellLevel <= 0 {
		return nil
	}
	language := builderCharacterLanguage(&character)
	entries := builderSRDSpellsForClassAtLevelLocalized(className, snapshot.MaxSpellLevel, language)
	if len(entries) == 0 {
		return nil
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Level == entries[j].Level {
			return entries[i].Name < entries[j].Name
		}
		return entries[i].Level < entries[j].Level
	})
	limit := 8
	switch {
	case snapshot.SpellsKnown > 0 && snapshot.SpellsKnown < limit:
		limit = snapshot.SpellsKnown
	case snapshot.PreparedCount > 0 && snapshot.PreparedCount < limit:
		limit = snapshot.PreparedCount
	}
	names := make([]string, 0, limit)
	for _, entry := range entries {
		names = append(names, entry.Name)
		if len(names) >= limit {
			break
		}
	}
	return uniquePreserveOrder(names)
}

func builderExtractSelectedLevelUpSpells(answers []string) (cantrips []string, spells []string) {
	for _, answer := range answers {
		parts := strings.SplitN(answer, "=>", 2)
		if len(parts) != 2 {
			continue
		}
		choice := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if value == "" {
			continue
		}
		_, kind := builderExpectedChoiceCount(choice)
		items := compactNonEmpty(strings.Split(value, ","))
		switch kind {
		case "cantrip":
			cantrips = append(cantrips, items...)
		case "spell":
			spells = append(spells, items...)
		}
	}
	return uniquePreserveOrder(cantrips), uniquePreserveOrder(spells)
}

func builderSpellSheetSummary(snapshot srdSpellcastingSnapshot, cantrips []string, spells []string, language string) string {
	lines := []string{}
	if slotSummary := builderSpellSlotSummary(snapshot); slotSummary != "" {
		lines = append(lines, slotSummary)
	}
	if len(cantrips) > 0 {
		lines = append(lines, fmt.Sprintf("Zaubertricks: %s", strings.Join(cantrips, ", ")))
	}
	if len(spells) > 0 {
		lines = append(lines, fmt.Sprintf("Zauber: %s", strings.Join(spells, ", ")))
	}
	_ = language
	return strings.Join(lines, "\n")
}

func builderSpellAttackRowsForEntries(entries []builderSpellCatalogEntry, className string, character Character) string {
	profile, ok := builderSRDSpellClassProfiles[className]
	if !ok {
		return ""
	}
	prof := proficiencyBonusForLevel(totalCharacterLevel(character.ClassAndLevel))
	mod := abilityModifierFromAny(character.Abilities[profile.CastingAbility])
	abilityLabel := strings.ToUpper(firstNonEmpty(profile.CastingAbility, ""))
	language := builderCharacterLanguage(&character)
	rows := []string{}
	for _, entry := range entries {
		if entry.AttackCategory == "" {
			continue
		}
		levelLabel := "Spell"
		if entry.Level == 0 {
			levelLabel = "Cantrip"
		} else {
			levelLabel = fmt.Sprintf("Level %d", entry.Level)
		}
		bonus := "—"
		damageType := "Effect"
		if normalizeUILanguage(language) == "de" {
			levelLabel = "Zauber"
			if entry.Level == 0 {
				levelLabel = "Zaubertrick"
			} else {
				levelLabel = fmt.Sprintf("Grad %d", entry.Level)
			}
			damageType = "Effekt"
		}
		switch entry.AttackCategory {
		case "attack":
			bonus = fmt.Sprintf("+%d", prof+mod)
			if normalizeUILanguage(language) == "de" {
				damageType = "Zauberangriff"
			} else {
				damageType = "Spell Attack"
			}
		case "save":
			bonus = fmt.Sprintf("SG %d", 8+prof+mod)
			if normalizeUILanguage(language) == "de" {
				damageType = "Rettungswurf"
			} else {
				damageType = "Saving Throw"
				bonus = fmt.Sprintf("DC %d", 8+prof+mod)
			}
		}
		rows = append(rows, fmt.Sprintf("%s | %s | %s | %s | %s | — | %s", levelLabel, entry.Name, abilityLabel, firstNonEmpty(entry.Range, "—"), bonus, damageType))
		descriptionPrefix := "Description"
		if normalizeUILanguage(language) == "de" {
			descriptionPrefix = "Beschreibung"
		}
		rows = append(rows, fmt.Sprintf("%s: %s", descriptionPrefix, localizedShortSpellDescription(entry, language)))
	}
	return strings.Join(uniquePreserveOrder(rows), "\n")
}

func applyLevelUpSpellSheetMetadata(patch *CharacterBuilderPatch, character Character, preview srdLevelUpPreview, answers []string) {
	if patch == nil {
		return
	}
	language := builderCharacterLanguage(&character)
	cantrips, spells := builderExtractSelectedLevelUpSpells(answers)
	if len(cantrips) == 0 && len(spells) == 0 {
		return
	}
	selectedNames := append(append([]string{}, cantrips...), spells...)
	entries := builderSRDSpellsForNamesLocalized(selectedNames, language)
	if patch.Metadata == nil {
		patch.Metadata = map[string]any{}
	}
	patch.Metadata["spells"] = builderSpellSheetSummary(preview.Spellcasting, cantrips, spells, language)
	if spellRows := builderSpellAttackRowsForEntries(entries, preview.TargetClass, character); strings.TrimSpace(spellRows) != "" {
		patch.Metadata["spell_attacks"] = spellRows
	}
	if advice, ok := builderSpellAdviceForCharacterAtLevel(character, preview.TargetClass, preview.TargetLevel); ok {
		patch.Metadata["spell_notes"] = advice.SpellNotes
		patch.Metadata["spell_save_dc"] = advice.SpellSaveDC
		patch.Metadata["spell_attack_bonus"] = advice.SpellAttackBonus
	}
}

func applyAverageLevelUpPatch(character Character, targetClass string, targetLevel int) (CharacterBuilderPatch, error) {
	return applyGuidedLevelUpPatch(character, targetClass, targetLevel, "", 0)
}

func applyGuidedLevelUpPatch(character Character, targetClass string, targetLevel int, hpMode string, rolledGain int) (CharacterBuilderPatch, error) {
	preview, ok := builderLevelUpPreview(character, targetClass, targetLevel)
	if !ok {
		return CharacterBuilderPatch{}, fmt.Errorf("no SRD level-up preview available")
	}
	currentHP := 0
	if character.HitPointMax != nil {
		currentHP = *character.HitPointMax
	} else {
		currentHP = builderDerivedHitPointMax(character)
	}
	hpGain := preview.AverageHPGain
	switch strings.ToLower(strings.TrimSpace(hpMode)) {
	case "rolled":
		if rolledGain > 0 {
			hpGain = rolledGain + abilityModifierFromAny(character.Abilities["constitution"])
		}
	case "average":
	default:
		hpMode = "average"
	}
	nextHP := currentHP + hpGain
	nextLevelText := builderClassAndLevelText(preview.TargetClass, character.ClassAndLevel, targetLevel)
	features := uniquePreserveOrder(append(append([]string{}, character.Features...), preview.FeaturesGained...))
	profText := fmt.Sprintf("+%d", preview.ProficiencyBonus)

	patch := CharacterBuilderPatch{
		ClassAndLevel: &nextLevelText,
		HitPointMax:   &nextHP,
		Proficiency:   &profText,
		Features:      features,
		Metadata: map[string]any{
			"level_up_available": "false",
			"builder_stage":      "review",
		},
	}

	if patch.Metadata == nil {
		patch.Metadata = map[string]any{}
	}
	patch.Metadata["level_up_feature_summary"] = preview.FeaturesGained
	patch.Metadata["level_up_target_class"] = preview.TargetClass
	patch.Metadata["level_up_hp_mode"] = firstNonEmpty(strings.TrimSpace(hpMode), "average")
	patch.Metadata["level_up_hp_gain"] = hpGain
	if len(preview.ChoicePrompts) > 0 {
		patch.Metadata["level_up_choices_due"] = preview.ChoicePrompts
	}
	if summary := builderSpellSlotSummary(preview.Spellcasting); summary != "" {
		patch.Metadata["spell_slot_summary"] = summary
		patch.Metadata["level_up_spell_recommendations"] = builderLevelUpSuggestedSpells(character, preview.TargetClass, preview.Spellcasting)
	}
	if preview.Spellcasting.PreparedCount > 0 {
		patch.Metadata["spells_prepared_count"] = preview.Spellcasting.PreparedCount
	}
	if preview.Spellcasting.SpellsKnown > 0 {
		patch.Metadata["spells_known_count"] = preview.Spellcasting.SpellsKnown
	}
	if preview.Spellcasting.SpellbookTotal > 0 {
		patch.Metadata["spellbook_entry_count"] = preview.Spellcasting.SpellbookTotal
	}

	nextCharacter := character
	applyCharacterPatch(&nextCharacter, patch)
	ac := builderDerivedArmorClass(nextCharacter)
	speed := builderDerivedSpeed(nextCharacter)
	patch.ArmorClass = &ac
	patch.Speed = &speed
	if spellAdvice, ok := builderSpellAdviceForCharacterAtLevel(nextCharacter, preview.TargetClass, targetLevel); ok {
		patch.Metadata["spell_notes"] = spellAdvice.SpellNotes
		if strings.TrimSpace(spellAdvice.SpellAttackRows) != "" {
			patch.Metadata["spell_attacks"] = spellAdvice.SpellAttackRows
		}
		patch.Metadata["spell_save_dc"] = spellAdvice.SpellSaveDC
		patch.Metadata["spell_attack_bonus"] = spellAdvice.SpellAttackBonus
		if len(spellAdvice.Recommendation) > 0 {
			patch.Metadata["level_up_spell_recommendations"] = spellAdvice.Recommendation
		}
	}
	return patch, nil
}

func builderSpellAdviceForCharacterAtLevel(character Character, className string, level int) (builderSpellAdvice, bool) {
	if strings.TrimSpace(className) == "" {
		className = singleOrPrimaryClassName(character.ClassAndLevel)
	}
	if className == "" {
		return builderSpellAdvice{}, false
	}
	snapshot := builderSpellcastingSnapshotForClass(className, level, totalCharacterLevel(character.ClassAndLevel), multiclassCasterLevel(character.ClassAndLevel), character)
	if snapshot.MaxSpellLevel <= 0 && snapshot.PactSlots <= 0 {
		return builderSpellAdvice{}, false
	}
	prof := proficiencyBonusForLevel(totalCharacterLevel(character.ClassAndLevel))
	profile, hasProfile := builderSRDSpellClassProfiles[className]
	language := builderCharacterLanguage(&character)
	mod := 0
	if hasProfile {
		mod = abilityModifierFromAny(character.Abilities[profile.CastingAbility])
	}
	entries := builderSRDSpellsForClassAtLevelLocalized(className, snapshot.MaxSpellLevel, language)
	cantrips, prepared := splitSpellNamesForMaxLevel(entries)
	optionLines := []string{}
	if len(cantrips) > 0 && snapshot.CantripsKnown > 0 {
		optionLines = append(optionLines, builderSpellSummaryLine("Cantrips such as", cantrips))
	}
	if len(prepared) > 0 {
		optionLines = append(optionLines, builderSpellSummaryLine("Spells such as", prepared))
	}
	recommendation := localizedSpellNames(builderLevelUpSuggestedSpells(character, className, snapshot), language)
	notesParts := []string{}
	if slotSummary := builderSpellSlotSummary(snapshot); slotSummary != "" {
		notesParts = append(notesParts, slotSummary)
	}
	if snapshot.CantripsKnown > 0 {
		notesParts = append(notesParts, fmt.Sprintf("cantrips known: %d", snapshot.CantripsKnown))
	}
	if snapshot.SpellsKnown > 0 {
		notesParts = append(notesParts, fmt.Sprintf("spells known: %d", snapshot.SpellsKnown))
	}
	if snapshot.PreparedCount > 0 {
		notesParts = append(notesParts, fmt.Sprintf("spells prepared: %d", snapshot.PreparedCount))
	}
	if snapshot.SpellbookTotal > 0 {
		notesParts = append(notesParts, fmt.Sprintf("spellbook entries: %d", snapshot.SpellbookTotal))
	}
	attackBonus := ""
	saveDC := ""
	if hasProfile {
		attackBonus = fmt.Sprintf("+%d", prof+mod)
		saveDC = fmt.Sprintf("%d", 8+prof+mod)
	}
	return builderSpellAdvice{
		Options:          compactNonEmpty(optionLines),
		Recommendation:   recommendation,
		SpellNotes:       strings.Join(notesParts, "; "),
		SpellAttacks:     builderSpellAttackSummaries(entries, prof, mod),
		SpellAttackRows:  builderSpellAttackRowsForEntries(entries, className, character),
		SpellSaveDC:      saveDC,
		SpellAttackBonus: attackBonus,
	}, true
}

func splitSpellNamesForMaxLevel(entries []builderSpellCatalogEntry) (cantrips []string, spells []string) {
	for _, entry := range entries {
		if entry.Level == 0 {
			cantrips = append(cantrips, entry.Name)
			continue
		}
		spells = append(spells, entry.Name)
	}
	return uniquePreserveOrder(cantrips), uniquePreserveOrder(spells)
}

func builderLevelUpSummary(character Character) string {
	if !characterEligibleForLevelUp(character) {
		return ""
	}
	targetClass := strings.TrimSpace(safeOptionalString(defaultMetadata(character.Metadata)["level_up_target_class"]))
	if targetClass == "" {
		targetClass = singleOrPrimaryClassName(character.ClassAndLevel)
	}
	currentLevel := classLevelForName(character.ClassAndLevel, targetClass)
	targetLevel := currentLevel + 1
	preview, ok := builderLevelUpPreview(character, targetClass, targetLevel)
	if !ok {
		return ""
	}
	parts := []string{
		fmt.Sprintf("%s %d → %d (total %d)", preview.TargetClass, preview.CurrentLevel, preview.TargetLevel, preview.TotalLevel),
		fmt.Sprintf("avg HP gain %+d", preview.AverageHPGain),
		fmt.Sprintf("proficiency +%d", preview.ProficiencyBonus),
	}
	if len(preview.FeaturesGained) > 0 {
		parts = append(parts, "features: "+strings.Join(preview.FeaturesGained, ", "))
	}
	if slots := builderSpellSlotSummary(preview.Spellcasting); slots != "" {
		parts = append(parts, slots)
	}
	if len(preview.ChoicePrompts) > 0 {
		parts = append(parts, "choices: "+strings.Join(preview.ChoicePrompts, " | "))
	}
	return strings.Join(parts, " | ")
}

func compactNonEmpty(items []string) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}
