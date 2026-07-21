package httpapi

import (
	"regexp"
	"strings"
)

var canonicalClassIDByAlias = map[string]string{
	"barbar":       "barbarian",
	"barbarian":    "barbarian",
	"barde":        "bard",
	"bard":         "bard",
	"kleriker":     "cleric",
	"cleric":       "cleric",
	"druide":       "druid",
	"druid":        "druid",
	"kämpfer":      "fighter",
	"kaempfer":     "fighter",
	"fighter":      "fighter",
	"mönch":        "monk",
	"moench":       "monk",
	"monk":         "monk",
	"paladin":      "paladin",
	"waldläufer":   "ranger",
	"waldlaeufer":  "ranger",
	"ranger":       "ranger",
	"schurke":      "rogue",
	"rogue":        "rogue",
	"zauberer":     "sorcerer",
	"sorcerer":     "sorcerer",
	"hexenmeister": "warlock",
	"warlock":      "warlock",
	"magier":       "wizard",
	"wizard":       "wizard",
}

func canonicalClassID(name string) string {
	normalized := normalizeBuilderIntentText(name)
	if normalized == "" {
		return ""
	}
	if id, ok := canonicalClassIDByAlias[normalized]; ok {
		return id
	}
	return normalized
}

type srdClassCatalogEntry struct {
	ClassRule builderClassRule
	Equipment builderEquipmentAdvice
	Features  builderFeatureAdvice
}

type srdEncounterCatalogEntry struct {
	Definition enemyEncounterDefinition
}

type srdMonsterCatalogEntry struct {
	Name        string
	SizeType    string
	ArmorClass  string
	HitPoints   string
	Speed       string
	Challenge   string
	Senses      string
	Languages   string
	ActionLines []string
	SourcePage  int
	SourcePages []int
	AttackBonus int
	DamageDice  string
	DamageType  string
}

var srdClassCatalog = map[string]srdClassCatalogEntry{
	"Barbar": {
		ClassRule: builderClassRule{ClassName: "Barbar", Aliases: []string{"barbar", "barbarian"}, SavingThrows: []string{"Stärke", "Konstitution"}, SkillChoiceCount: 2, SkillChoices: []string{"Mit Tieren umgehen", "Athletik", "Einschüchtern", "Naturkunde", "Wahrnehmung", "Überlebenskunst"}, HitDie: "W12"},
		Equipment: builderEquipmentAdvice{
			Options:        []string{"Waffenwahl: eine Großaxt oder eine beliebige Kriegs-Nahkampfwaffe", "Zweitwaffe: zwei Handäxte oder eine beliebige einfache Waffe", "Paket: Entdeckerausrüstung", "Zusätzlich: vier Wurfspeere"},
			Recommendation: []string{"Großaxt", "zwei Handäxte", "Entdeckerausrüstung", "vier Wurfspeere"},
			WeaponNotes:    "Großaxt für maximalen frühen Nahkampfschaden, Handäxte und Wurfspeere als flexible Ergänzung.",
			CombatOverview: "Klassischer Barbar-Start mit schwerer Zweihandwaffe und soliden Wurfoptionen.",
		},
		Features: builderFeatureAdvice{Options: []string{"Raserei", "Unarmored Defense"}, Recommendation: []string{"Raserei", "Unarmored Defense"}},
	},
	"Barde": {
		ClassRule: builderClassRule{ClassName: "Barde", Aliases: []string{"barde", "bard"}, SavingThrows: []string{"Geschicklichkeit", "Charisma"}, SkillChoiceCount: 3, SkillChoices: []string{"Beliebige Fertigkeiten"}, HitDie: "W8"},
		Equipment: builderEquipmentAdvice{
			Options:        []string{"Waffenwahl: Rapier, Langschwert oder eine beliebige einfache Waffe", "Paket: Diplomatenausrüstung oder Unterhaltungskünstlerausrüstung", "Instrument: Laute oder ein anderes Musikinstrument", "Zusätzlich: Lederrüstung und ein Dolch"},
			Recommendation: []string{"Rapier", "Diplomatenausrüstung", "Laute", "Lederrüstung", "Dolch"},
			WeaponNotes:    "Rapier für solide Finesse-Nahkampfangriffe, Dolch als Reservewaffe.",
			CombatOverview: "Vielseitiger Barden-Start mit brauchbarer Nahkampfwaffe, Fokus auf soziale und mobile Szenen.",
		},
		Features: builderFeatureAdvice{Options: []string{"Zauberwirken", "Bardic Inspiration"}, Recommendation: []string{"Zauberwirken", "Bardic Inspiration"}},
	},
	"Kleriker": {
		ClassRule: builderClassRule{ClassName: "Kleriker", Aliases: []string{"kleriker", "cleric"}, SavingThrows: []string{"Weisheit", "Charisma"}, SkillChoiceCount: 2, SkillChoices: []string{"Geschichte", "Motiv erkennen", "Medizin", "Überzeugen", "Religion"}, HitDie: "W8"},
		Equipment: builderEquipmentAdvice{
			Options:        []string{"Waffenwahl: Streitkolben oder Kriegshammer, falls geübt", "Rüstung: Schuppenpanzer, Lederrüstung oder Kettenhemd, falls geübt", "Fernkampf: leichte Armbrust mit 20 Bolzen oder eine beliebige einfache Waffe", "Paket: Priesterpack oder Entdeckerausrüstung", "Zusätzlich: Schild und heiliges Symbol"},
			Recommendation: []string{"Streitkolben", "Schuppenpanzer", "leichte Armbrust mit 20 Bolzen", "Priesterpack", "Schild", "heiliges Symbol"},
			WeaponNotes:    "Streitkolben und Schild für solide Front, leichte Armbrust für Distanz.",
			CombatOverview: "Defensiver Kleriker-Start mit ordentlicher Rüstung, Schild und sicherer Fernkampfreserve.",
		},
		Features: builderFeatureAdvice{Options: []string{"Zauberwirken", "Göttliche Domäne"}, Recommendation: []string{"Zauberwirken", "Göttliche Domäne: Leben"}},
	},
	"Druide": {
		ClassRule: builderClassRule{ClassName: "Druide", Aliases: []string{"druide", "druid"}, SavingThrows: []string{"Intelligenz", "Weisheit"}, SkillChoiceCount: 2, SkillChoices: []string{"Arkane Kunde", "Mit Tieren umgehen", "Heilkunde", "Motiv erkennen", "Naturkunde", "Wahrnehmung", "Religion", "Überlebenskunst"}, HitDie: "W8"},
		Equipment: builderEquipmentAdvice{
			Options:        []string{"Erste Wahl: Holzschild oder eine beliebige einfache Waffe", "Zweite Wahl: Krummsäbel oder eine beliebige einfache Nahkampfwaffe", "Zusätzlich: Lederrüstung, Entdeckerausrüstung und druidischer Fokus"},
			Recommendation: []string{"Holzschild", "Krummsäbel", "Lederrüstung", "Entdeckerausrüstung", "druidischer Fokus"},
			WeaponNotes:    "Krummsäbel für verlässlichen Nahkampf, Holzschild für frühe Defensive.",
			CombatOverview: "Stabiler Druiden-Start mit Fokus auf Schutz, Zauberwirken und einfache Feldtauglichkeit.",
		},
		Features: builderFeatureAdvice{Options: []string{"Zauberwirken", "Druidic"}, Recommendation: []string{"Zauberwirken", "Druidic"}},
	},
	"Kämpfer": {
		ClassRule: builderClassRule{ClassName: "Kämpfer", Aliases: []string{"kaempfer", "kämpfer", "fighter"}, SavingThrows: []string{"Stärke", "Konstitution"}, SkillChoiceCount: 2, SkillChoices: []string{"Akrobatik", "Mit Tieren umgehen", "Athletik", "Geschichte", "Motiv erkennen", "Einschüchtern", "Wahrnehmung", "Überlebenskunst"}, HitDie: "W10"},
		Equipment: builderEquipmentAdvice{
			Options:        []string{"Rüstungswahl: Kettenhemd oder Lederüstung, Langbogen und 20 Pfeile", "Waffenwahl: eine Kriegswaffe und Schild oder zwei Kriegswaffen", "Zusatzwaffen: leichte Armbrust mit 20 Bolzen oder zwei Handäxte", "Paket: Verlieserkunderausrüstung oder Entdeckerausrüstung"},
			Recommendation: []string{"Kettenhemd", "Langschwert und Schild", "leichte Armbrust mit 20 Bolzen", "Entdeckerausrüstung"},
			WeaponNotes:    "Langschwert und Schild als Hauptausrüstung, leichte Armbrust als Fernkampfoption.",
			CombatOverview: "Defensiver Kämpfer-Start mit Schild, solider Nahkampfwaffe und Fernkampf-Backup.",
		},
		Features: builderFeatureAdvice{Options: []string{"Fighting Style", "Second Wind"}, Recommendation: []string{"Fighting Style: Defense", "Second Wind"}},
	},
	"Mönch": {
		ClassRule: builderClassRule{ClassName: "Mönch", Aliases: []string{"moench", "mönch", "monk"}, SavingThrows: []string{"Stärke", "Geschicklichkeit"}, SkillChoiceCount: 2, SkillChoices: []string{"Akrobatik", "Athletik", "Geschichte", "Motiv erkennen", "Religion", "Heimlichkeit"}, HitDie: "W8"},
		Equipment: builderEquipmentAdvice{
			Options:        []string{"Waffenwahl: Kurzschwert oder eine beliebige einfache Waffe", "Paket: Verlieserkunderausrüstung oder Entdeckerausrüstung", "Zusätzlich: 10 Wurfpfeile"},
			Recommendation: []string{"Kurzschwert", "Entdeckerausrüstung", "10 Wurfpfeile"},
			WeaponNotes:    "Kurzschwert als solide frühe Waffe, Wurfpfeile für Reichweite ohne Ausrüstungslast.",
			CombatOverview: "Leichter, mobiler Mönch-Start mit Fokus auf Beweglichkeit und flexiblem Nah-/Fernkampf.",
		},
		Features: builderFeatureAdvice{Options: []string{"Unarmored Defense", "Martial Arts"}, Recommendation: []string{"Unarmored Defense", "Martial Arts"}},
	},
	"Paladin": {
		ClassRule: builderClassRule{ClassName: "Paladin", Aliases: []string{"paladin"}, SavingThrows: []string{"Weisheit", "Charisma"}, SkillChoiceCount: 2, SkillChoices: []string{"Athletik", "Motiv erkennen", "Einschüchtern", "Medizin", "Überzeugen", "Religion"}, HitDie: "W10"},
		Equipment: builderEquipmentAdvice{
			Options:        []string{"Waffenwahl: eine Kriegswaffe und Schild oder zwei Kriegswaffen", "Zusatzwahl: fünf Wurfspeere oder eine beliebige einfache Nahkampfwaffe", "Paket: Priesterpack oder Entdeckerausrüstung", "Zusätzlich: Kettenhemd und heiliges Symbol"},
			Recommendation: []string{"Langschwert und Schild", "fünf Wurfspeere", "Entdeckerausrüstung", "Kettenhemd", "heiliges Symbol"},
			WeaponNotes:    "Schwert-und-Schild-Start für hohe Rüstungsklasse, Wurfspeere als Distanzoption.",
			CombatOverview: "Klassischer Paladin-Start mit hoher Defensive und guten Voraussetzungen für Frontkampf.",
		},
		Features: builderFeatureAdvice{Options: []string{"Divine Sense", "Lay on Hands"}, Recommendation: []string{"Divine Sense", "Lay on Hands"}},
	},
	"Waldläufer": {
		ClassRule: builderClassRule{ClassName: "Waldläufer", Aliases: []string{"waldlaeufer", "waldläufer", "ranger"}, SavingThrows: []string{"Stärke", "Geschicklichkeit"}, SkillChoiceCount: 3, SkillChoices: []string{"Mit Tieren umgehen", "Athletik", "Motiv erkennen", "Nachforschungen", "Naturkunde", "Wahrnehmung", "Heimlichkeit", "Überlebenskunst"}, HitDie: "W10"},
		Equipment: builderEquipmentAdvice{
			Options:        []string{"Rüstung: Schuppenpanzer oder Lederrüstung", "Nahkampfwaffen: zwei Kurzschwerter oder zwei einfache Nahkampfwaffen", "Paket: Verlieserkunderausrüstung oder Entdeckerausrüstung", "Zusätzlich: Langbogen und Köcher mit 20 Pfeilen"},
			Recommendation: []string{"Lederrüstung", "zwei Kurzschwerter", "Entdeckerausrüstung", "Langbogen und Köcher mit 20 Pfeilen"},
			WeaponNotes:    "Langbogen für starke Fernkampferöffnung, zwei Kurzschwerter für flexible Nahkampfrunden.",
			CombatOverview: "Vielseitiger Waldläufer-Start mit sauberer Mischung aus Fernkampf, Mobilität und Nahkampfflexibilität.",
		},
		Features: builderFeatureAdvice{Options: []string{"Favored Enemy", "Natural Explorer"}, Recommendation: []string{"Favored Enemy: Monstrositäten", "Natural Explorer: Wald"}},
	},
	"Schurke": {
		ClassRule: builderClassRule{ClassName: "Schurke", Aliases: []string{"schurke", "rogue"}, SavingThrows: []string{"Geschicklichkeit", "Intelligenz"}, SkillChoiceCount: 4, SkillChoices: []string{"Akrobatik", "Athletik", "Täuschung", "Motiv erkennen", "Einschüchtern", "Nachforschungen", "Wahrnehmung", "Auftreten", "Überzeugen", "Fingerfertigkeit", "Heimlichkeit"}, HitDie: "W8"},
		Equipment: builderEquipmentAdvice{
			Options:        []string{"Waffenwahl: Rapier oder Kurzschwert", "Fernkampf: Kurzbogen mit Köcher und 20 Pfeilen oder Kurzschwert", "Paket: Einbrecherpack, Verlieserkunderausrüstung oder Entdeckerausrüstung", "Zusätzlich: Lederrüstung, zwei Dolche und Diebeswerkzeug"},
			Recommendation: []string{"Rapier", "Kurzbogen mit Köcher und 20 Pfeilen", "Einbrecherpack", "Lederrüstung", "zwei Dolche", "Diebeswerkzeug"},
			WeaponNotes:    "Rapier für Finesse im Nahkampf, Kurzbogen für verlässliche Hinterhaltsangriffe auf Distanz.",
			CombatOverview: "Klassischer Schurken-Start mit Finesse-Waffe, Stealth-tauglicher Ausrüstung und vollem Utility-Paket.",
		},
		Features: builderFeatureAdvice{Options: []string{"Expertise", "Sneak Attack", "Thieves' Cant"}, Recommendation: []string{"Expertise: Wahrnehmung und Heimlichkeit", "Sneak Attack", "Thieves' Cant"}},
	},
	"Zauberer": {
		ClassRule: builderClassRule{ClassName: "Zauberer", Aliases: []string{"zauberer", "sorcerer"}, SavingThrows: []string{"Konstitution", "Charisma"}, SkillChoiceCount: 2, SkillChoices: []string{"Arkane Kunde", "Täuschung", "Motiv erkennen", "Einschüchtern", "Überzeugen", "Religion"}, HitDie: "W6"},
		Equipment: builderEquipmentAdvice{
			Options:        []string{"Fernkampf oder Waffe: leichte Armbrust mit 20 Bolzen oder eine beliebige einfache Waffe", "Zauberfokus: Komponentenbeutel oder arkane Fokuskomponente", "Paket: Verlieserkunderausrüstung oder Entdeckerausrüstung", "Zusätzlich: zwei Dolche"},
			Recommendation: []string{"leichte Armbrust mit 20 Bolzen", "arkaner Fokus", "Entdeckerausrüstung", "zwei Dolche"},
			WeaponNotes:    "Leichte Armbrust für frühen Fernkampfschaden, Dolche als Notfallreserve.",
			CombatOverview: "Pragmatischer Zauberer-Start mit Zauberfokus, etwas Reichweite und minimaler Selbstverteidigung.",
		},
		Features: builderFeatureAdvice{Options: []string{"Zauberwirken", "Sorcerous Origin"}, Recommendation: []string{"Zauberwirken", "Sorcerous Origin: Draconic Bloodline"}},
	},
	"Hexenmeister": {
		ClassRule: builderClassRule{ClassName: "Hexenmeister", Aliases: []string{"hexenmeister", "warlock"}, SavingThrows: []string{"Weisheit", "Charisma"}, SkillChoiceCount: 2, SkillChoices: []string{"Arkane Kunde", "Täuschung", "Geschichte", "Einschüchtern", "Nachforschungen", "Naturkunde", "Religion"}, HitDie: "W8"},
		Equipment: builderEquipmentAdvice{
			Options:        []string{"Fernkampf oder Waffe: leichte Armbrust mit 20 Bolzen oder eine beliebige einfache Waffe", "Zauberfokus: Komponentenbeutel oder arkane Fokuskomponente", "Paket: Gelehrtenpack oder Verlieserkunderausrüstung", "Zusätzlich: Lederrüstung, eine beliebige einfache Waffe und zwei Dolche"},
			Recommendation: []string{"leichte Armbrust mit 20 Bolzen", "arkaner Fokus", "Gelehrtenpack", "Lederrüstung", "Quarterstaff", "zwei Dolche"},
			WeaponNotes:    "Leichte Armbrust für frühe Distanz, einfacher Stab und Dolche als Backup.",
			CombatOverview: "Stabiler Hexenmeister-Start mit Fokus auf Zauberwirken, brauchbarer Reichweite und etwas Reserveausrüstung.",
		},
		Features: builderFeatureAdvice{Options: []string{"Otherworldly Patron", "Pact Magic"}, Recommendation: []string{"Otherworldly Patron: Fiend", "Pact Magic"}},
	},
	"Magier": {
		ClassRule: builderClassRule{ClassName: "Magier", Aliases: []string{"magier", "wizard"}, SavingThrows: []string{"Intelligenz", "Weisheit"}, SkillChoiceCount: 2, SkillChoices: []string{"Arkane Kunde", "Geschichte", "Motiv erkennen", "Nachforschungen", "Medizin", "Religion"}, HitDie: "W6"},
		Equipment: builderEquipmentAdvice{
			Options:        []string{"Waffenwahl: Quarterstaff oder Dolch", "Zauberfokus: Komponentenbeutel oder arkane Fokuskomponente", "Paket: Gelehrtenpack oder Entdeckerausrüstung", "Zusätzlich: Zauberbuch"},
			Recommendation: []string{"Quarterstaff", "arkaner Fokus", "Gelehrtenpack", "Zauberbuch"},
			WeaponNotes:    "Quarterstaff als einfache Nahkampf-Notlösung, Fokus und Zauberbuch für den Kern des Builds.",
			CombatOverview: "Klassischer Magier-Start mit vollem Zauber-Setup und minimaler, aber ausreichender Reserveausrüstung.",
		},
		Features: builderFeatureAdvice{Options: []string{"Zauberwirken", "Arcane Recovery"}, Recommendation: []string{"Zauberwirken", "Arcane Recovery"}},
	},
}

var srdEncounterCatalog = []srdEncounterCatalogEntry{
	{
		Definition: enemyEncounterDefinition{
			Keywords:       []string{"giant spider", "riesenspinne", "spider", "spinne"},
			PluralKeywords: []string{"two giant spiders", "2 giant spiders", "zwei riesenspinnen", "spiders", "riesenspinnen"},
			BaseName:       "spider",
			VariantProfiles: []enemyCombatProfile{
				{Name: "Hunting Spider", AttackBonus: 3, DamageDice: "1d4+1", DamageType: "piercing"},
				{Name: "Young Giant Spider", AttackBonus: 4, DamageDice: "1d6+2", DamageType: "piercing"},
				{Name: "Giant Spider", AttackBonus: 5, DamageDice: "1d8+3", DamageType: "piercing"},
			},
		},
	},
	{
		Definition: enemyEncounterDefinition{
			Keywords:       []string{"goblin"},
			PluralKeywords: []string{"two goblins", "2 goblins", "zwei goblins", "goblins"},
			BaseName:       "goblin",
			VariantProfiles: []enemyCombatProfile{
				{Name: "Goblin Scout", AttackBonus: 3, DamageDice: "1d4+1", DamageType: "slashing"},
				{Name: "Goblin Raider", AttackBonus: 4, DamageDice: "1d6+2", DamageType: "slashing"},
				{Name: "Goblin Skirmisher", AttackBonus: 4, DamageDice: "1d6+2", DamageType: "slashing"},
			},
		},
	},
	{
		Definition: enemyEncounterDefinition{
			Keywords:       []string{"wolf", "wulf", "wolfe", "wölf", "woelf"},
			PluralKeywords: []string{"two wolves", "2 wolves", "zwei wölfe", "zwei woelfe", "wolves", "wölfe", "woelfe"},
			BaseName:       "wolf",
			VariantProfiles: []enemyCombatProfile{
				{Name: "Young Wolf", AttackBonus: 3, DamageDice: "1d4+1", DamageType: "piercing"},
				{Name: "Wolf", AttackBonus: 4, DamageDice: "1d6+2", DamageType: "piercing"},
				{Name: "Dire Wolf", AttackBonus: 5, DamageDice: "2d6+3", DamageType: "piercing"},
			},
		},
	},
}

func srdClassRules() []builderClassRule {
	items := make([]builderClassRule, 0, len(srdClassCatalog))
	for _, entry := range srdClassCatalog {
		items = append(items, entry.ClassRule)
	}
	return items
}

func srdClassCatalogEntryByName(name string) (srdClassCatalogEntry, bool) {
	if entry, ok := srdClassCatalog[name]; ok {
		return entry, true
	}
	return srdClassCatalogEntry{}, false
}

func srdEncounterDefinitions() []enemyEncounterDefinition {
	items := make([]enemyEncounterDefinition, 0, len(srdEncounterCatalog))
	for _, entry := range srdEncounterCatalog {
		items = append(items, entry.Definition)
	}
	return items
}

func srdMonsterCatalogEntryByName(name string) (srdMonsterCatalogEntry, bool) {
	needle := strings.TrimSpace(strings.ToLower(name))
	if needle == "" {
		return srdMonsterCatalogEntry{}, false
	}
	for _, entry := range srdMonsterCatalog {
		if strings.EqualFold(strings.TrimSpace(entry.Name), needle) {
			return enrichMonsterCatalogEntry(entry), true
		}
	}
	for _, entry := range srdMonsterCatalog {
		if strings.Contains(strings.ToLower(entry.Name), needle) || strings.Contains(needle, strings.ToLower(entry.Name)) {
			return enrichMonsterCatalogEntry(entry), true
		}
	}
	return srdMonsterCatalogEntry{}, false
}

var monsterAttackBonusPattern = regexp.MustCompile(`\+([0-9]+)\s+to\s+hit`)
var monsterDamagePattern = regexp.MustCompile(`Hit:\s*[^(]*\(([0-9dD+\- ]+)\)\s*([A-Za-z]+)\s+damage`)

func enrichMonsterCatalogEntry(entry srdMonsterCatalogEntry) srdMonsterCatalogEntry {
	if entry.AttackBonus > 0 && entry.DamageDice != "" && entry.DamageType != "" {
		return entry
	}
	joined := strings.Join(entry.ActionLines, " ")
	if entry.AttackBonus == 0 {
		if matches := monsterAttackBonusPattern.FindStringSubmatch(joined); len(matches) == 2 {
			entry.AttackBonus = parseIntOrZero(matches[1])
		}
	}
	if entry.DamageDice == "" || entry.DamageType == "" {
		if matches := monsterDamagePattern.FindStringSubmatch(joined); len(matches) == 3 {
			entry.DamageDice = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(matches[1]), " ", ""))
			entry.DamageType = strings.ToLower(strings.TrimSpace(matches[2]))
		}
	}
	return entry
}

func parseIntOrZero(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	total := 0
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return 0
		}
		total = total*10 + int(ch-'0')
	}
	return total
}
