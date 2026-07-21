package httpapi

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

var srdGermanSpellNames = map[string]string{
	"Acid Splash":      "Säurespritzer",
	"Burning Hands":    "Brennende Hände",
	"Color Spray":      "Farbsprühen",
	"Comprehend Languages": "Sprachen verstehen",
	"Bless":            "Segen",
	"Charm Person":     "Person bezaubern",
	"Chill Touch":      "Kältegriff",
	"Detect Thoughts":  "Gedanken entdecken",
	"Disguise Self":    "Selbstverkleidung",
	"Cure Wounds":      "Wunden heilen",
	"Dancing Lights":   "Tanzende Lichter",
	"Detect Magic":     "Magie entdecken",
	"Eldritch Blast":   "Schauriger Strahl",
	"False Life":       "Falsches Leben",
	"Entangle":         "Verstricken",
	"Faerie Fire":      "Feenfeuer",
	"Feather Fall":     "Federfall",
	"Fire Bolt":        "Feuerpfeil",
	"Fog Cloud":        "Nebelwolke",
	"Guidance":         "Führung",
	"Guiding Bolt":     "Lenkender Blitz",
	"Healing Word":     "Heilendes Wort",
	"Hellish Rebuke":   "Höllischer Tadel",
	"Hideous Laughter": "Tashas schallendes Gelächter",
	"Hold Person":      "Person festhalten",
	"Invisibility":     "Unsichtbarkeit",
	"Jump":             "Sprung",
	"Light":            "Licht",
	"Mage Armor":       "Magische Rüstung",
	"Mage Hand":        "Magierhand",
	"Magic Weapon":     "Magische Waffe",
	"Magic Missile":    "Magisches Geschoss",
	"Mirror Image":     "Spiegelbilder",
	"Minor Illusion":   "Kleine Illusion",
	"Ray of Sickness":  "Strahl der Übelkeit",
	"Scorching Ray":    "Sengender Strahl",
	"Shatter":          "Zerbersten",
	"Prestidigitation": "Fingerfertigkeit",
	"Sleep":            "Schlaf",
	"Spider Climb":     "Spinnenklettern",
	"Shocking Grasp":   "Schockgriff",
	"Produce Flame":    "Flamme erzeugen",
	"Ray of Frost":     "Froststrahl",
	"Resistance":       "Widerstand",
	"Sacred Flame":     "Heilige Flamme",
	"Sanctuary":        "Heiligtum",
	"Shield":           "Schild",
	"Shillelagh":       "Shillelagh",
	"Silent Image":     "Stilles Trugbild",
	"Thunderwave":      "Donnerwoge",
	"Thaumaturgy":      "Wundertun",
	"True Strike":      "Wahrer Schlag",
	"Vicious Mockery":  "Hohn",
	"Web":              "Spinnennetz",
}

var srdGermanMonsterNames = map[string]string{
	"Enemy":          "Gegner",
	"Giant Spider":   "Riesenspinne",
	"Hunting Spider": "Jagdspinne",
	"Goblin":         "Goblin",
	"Goblin Raider":  "Goblin-Plünderer",
	"Goblin Scout":   "Goblin-Späher",
	"Wolf":           "Wolf",
	"Dire Wolf":      "Worg",
}

var srdShortSpellDescriptions = map[string]struct {
	en string
	de string
}{
	"Bless":            {en: "Adds a d4 to allies' attacks and saves.", de: "Verbündete erhalten +W4 auf Angriffe und Rettungswürfe."},
	"Burning Hands":    {en: "Fire cone that burns nearby creatures.", de: "Feuerkegel trifft nahe Kreaturen."},
	"Charm Person":     {en: "Makes a humanoid regard you as friendly.", de: "Ein Humanoider sieht dich als freundlich an."},
	"Cure Wounds":      {en: "Restores hit points by touch.", de: "Stellt per Berührung Trefferpunkte wieder her."},
	"Dancing Lights":   {en: "Creates moving lights within range.", de: "Erzeugt bewegliche Lichter in Reichweite."},
	"Detect Magic":     {en: "Reveals magic nearby while you concentrate.", de: "Zeigt Magie in der Nähe während Konzentration."},
	"Eldritch Blast":   {en: "Ranged force spell attack.", de: "Fernzauberangriff mit Kraftstrahl."},
	"Entangle":         {en: "Restraining plants hinder creatures in an area.", de: "Ranken behindern Kreaturen in einem Bereich."},
	"Faerie Fire":      {en: "Outlines targets and grants advantage to attackers.", de: "Umrisse verleihen Angreifern Vorteil."},
	"Feather Fall":     {en: "Slows falling creatures before impact.", de: "Bremst fallende Kreaturen vor dem Aufprall."},
	"Fire Bolt":        {en: "Ranged fire spell attack.", de: "Fernzauberangriff mit Feuer."},
	"Fog Cloud":        {en: "Creates a heavily obscuring fog.", de: "Erzeugt dichten verdeckenden Nebel."},
	"Guidance":         {en: "Adds a d4 to one ability check.", de: "Gibt +W4 auf einen Attributswurf."},
	"Guiding Bolt":     {en: "Radiant bolt that leaves the target exposed.", de: "Strahlender Treffer macht das Ziel verwundbar."},
	"Healing Word":     {en: "Heals a creature at short range.", de: "Heilt eine Kreatur auf kurze Distanz."},
	"Hellish Rebuke":   {en: "Fiery retaliation against a creature that hurt you.", de: "Feurige Vergeltung gegen einen Angreifer."},
	"Hideous Laughter": {en: "A creature falls into incapacitating laughter.", de: "Eine Kreatur bricht in lähmendes Gelächter aus."},
	"Light":            {en: "Makes an object shine brightly.", de: "Lässt einen Gegenstand hell leuchten."},
	"Mage Armor":       {en: "Sets a better base Armor Class.", de: "Setzt eine bessere Grund-Rüstungsklasse."},
	"Mage Hand":        {en: "Creates a spectral hand for simple tasks.", de: "Erschafft eine Geisterhand für einfache Aufgaben."},
	"Magic Missile":    {en: "Automatically hits with force darts.", de: "Trifft automatisch mit Kraftgeschossen."},
	"Minor Illusion":   {en: "Creates a small image or sound illusion.", de: "Erschafft eine kleine Bild- oder Klangillusion."},
	"Prestidigitation": {en: "Performs small magical tricks.", de: "Erzeugt kleine magische Kunststücke."},
	"Produce Flame":    {en: "Conjures flame you can carry or hurl.", de: "Erschafft eine Flamme zum Tragen oder Werfen."},
	"Ray of Frost":     {en: "Cold attack that slows the target.", de: "Kälteangriff verlangsamt das Ziel."},
	"Resistance":       {en: "Adds a d4 to one saving throw.", de: "Gibt +W4 auf einen Rettungswurf."},
	"Sacred Flame":     {en: "Radiant flame forcing a Dexterity save.", de: "Heilige Flamme erzwingt einen Geschicklichkeitswurf."},
	"Sanctuary":        {en: "Protects a creature until it acts aggressively.", de: "Schützt eine Kreatur bis zu einem Angriff."},
	"Shield":           {en: "Reaction spell that raises AC briefly.", de: "Reaktionszauber erhöht kurz die RK."},
	"Shillelagh":       {en: "Empowers a club or quarterstaff for melee.", de: "Verstärkt Knüppel oder Kampfstab im Nahkampf."},
	"Silent Image":     {en: "Creates a movable visual illusion.", de: "Erschafft eine bewegliche Bildillusion."},
	"Sleep":            {en: "Puts creatures to sleep starting with the weakest.", de: "Lässt zuerst die schwächsten Kreaturen einschlafen."},
	"Thaumaturgy":      {en: "Creates minor supernatural signs.", de: "Erzeugt kleine übernatürliche Effekte."},
	"Thunderwave":      {en: "Thunder burst pushes creatures away.", de: "Donnerschlag stößt Kreaturen zurück."},
	"True Strike":      {en: "Briefly grants advantage on your next attack.", de: "Verleiht kurz Vorteil auf den nächsten Angriff."},
	"Vicious Mockery":  {en: "Psychic insult hinders the target's next attack.", de: "Psionischer Spott schwächt den nächsten Angriff."},
}

func localizedSpellName(name string, language string) string {
	if normalizeUILanguage(language) != "de" {
		return strings.TrimSpace(name)
	}
	if translated, ok := srdGermanSpellNames[strings.TrimSpace(name)]; ok {
		return translated
	}
	return strings.TrimSpace(name)
}

func canonicalSpellName(name string, language string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	if normalizeUILanguage(language) == "de" {
		for canonical, translated := range srdGermanSpellNames {
			if strings.EqualFold(strings.TrimSpace(translated), trimmed) {
				return canonical
			}
		}
	}
	return trimmed
}

func localizedSpellNames(names []string, language string) []string {
	if len(names) == 0 {
		return nil
	}
	items := make([]string, 0, len(names))
	for _, name := range names {
		items = append(items, localizedSpellName(name, language))
	}
	return items
}

func localizedSpellEntries(entries []builderSpellCatalogEntry, language string) []builderSpellCatalogEntry {
	items := make([]builderSpellCatalogEntry, 0, len(entries))
	for _, entry := range entries {
		copy := entry
		copy.Name = localizedSpellName(entry.Name, language)
		copy.Range = localizedMeasurementText(entry.Range, language)
		items = append(items, copy)
	}
	return items
}

func localizedMonsterName(name string, language string) string {
	if normalizeUILanguage(language) != "de" {
		return strings.TrimSpace(name)
	}
	trimmed := strings.TrimSpace(name)
	if translated, ok := srdGermanMonsterNames[trimmed]; ok {
		return translated
	}
	return trimmed
}

func localizedShortSpellDescription(entry builderSpellCatalogEntry, language string) string {
	canonical := canonicalSpellName(entry.Name, language)
	if text, ok := srdShortSpellDescriptions[canonical]; ok {
		if normalizeUILanguage(language) == "de" {
			return text.de
		}
		return text.en
	}
	return genericSpellDescription(entry, language)
}

func genericSpellDescription(entry builderSpellCatalogEntry, language string) string {
	de := normalizeUILanguage(language) == "de"
	duration := localizedMeasurementText(strings.TrimSpace(entry.Duration), language)
	rangeText := localizedMeasurementText(strings.TrimSpace(entry.Range), language)
	switch entry.AttackCategory {
	case "attack":
		if de {
			return "Zauberangriff gegen ein Ziel in Reichweite."
		}
		return "Spell attack against a target in range."
	case "save":
		if de {
			return "Erzwingt einen Rettungswurf gegen den Zaubereffekt."
		}
		return "Forces a saving throw against the spell's effect."
	case "healing":
		if de {
			return "Stellt Trefferpunkte wieder her."
		}
		return "Restores hit points."
	default:
		if de {
			if rangeText != "" && duration != "" {
				return fmt.Sprintf("Nützlicher Zaubereffekt in %s für %s.", rangeText, duration)
			}
			if rangeText != "" {
				return fmt.Sprintf("Nützlicher Zaubereffekt in %s.", rangeText)
			}
			return "Nützlicher magischer Effekt."
		}
		if rangeText != "" && duration != "" {
			return fmt.Sprintf("Useful magical effect at %s for %s.", rangeText, duration)
		}
		if rangeText != "" {
			return fmt.Sprintf("Useful magical effect at %s.", rangeText)
		}
		return "Useful magical effect."
	}
}

func normalizeUILanguage(language string) string {
	if strings.EqualFold(strings.TrimSpace(language), "en") {
		return "en"
	}
	return "de"
}

var localizedFeetPattern = regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*(feet|foot|ft\.?|fuß)`)
var localizedMetersPattern = regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*m(?:eter|eters)?`)
var localizedFootCubePattern = regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*-\s*foot\s+cube`)
var localizedFeetPairPattern = regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*/\s*(\d+(?:[.,]\d+)?)\s*(feet|foot|ft\.?|fuß)`)
var localizedMetersPairPattern = regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*/\s*(\d+(?:[.,]\d+)?)\s*m(?:eter|eters)?`)

func localizedMeasurementText(value string, language string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if normalizeUILanguage(language) == "de" {
		return localizeMeasurementTextToGerman(trimmed)
	}
	return localizeMeasurementTextToEnglish(trimmed)
}

func localizeMeasurementTextToGerman(value string) string {
	replaced := localizedFootCubePattern.ReplaceAllStringFunc(value, func(match string) string {
		parts := localizedFootCubePattern.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		feet, ok := parseLocalizedNumber(parts[1])
		if !ok {
			return match
		}
		return fmt.Sprintf("%s-Würfel", formatMetersForGerman(feetToMeters(feet)))
	})
	replaced = localizedFeetPairPattern.ReplaceAllStringFunc(replaced, func(match string) string {
		parts := localizedFeetPairPattern.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		first, okFirst := parseLocalizedNumber(parts[1])
		second, okSecond := parseLocalizedNumber(parts[2])
		if !okFirst || !okSecond {
			return match
		}
		return fmt.Sprintf("%s/%s m", formatLocalizedGermanNumber(feetToMeters(first)), formatLocalizedGermanNumber(feetToMeters(second)))
	})
	replaced = localizedFeetPattern.ReplaceAllStringFunc(replaced, func(match string) string {
		parts := localizedFeetPattern.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		feet, ok := parseLocalizedNumber(parts[1])
		if !ok {
			return match
		}
		return formatMetersForGerman(feetToMeters(feet))
	})
	return replaced
}

func localizeMeasurementTextToEnglish(value string) string {
	replaced := localizedMetersPairPattern.ReplaceAllStringFunc(value, func(match string) string {
		parts := localizedMetersPairPattern.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		first, okFirst := parseLocalizedNumber(parts[1])
		second, okSecond := parseLocalizedNumber(parts[2])
		if !okFirst || !okSecond {
			return match
		}
		return fmt.Sprintf("%s/%s ft", formatLocalizedEnglishNumber(metersToFeet(first)), formatLocalizedEnglishNumber(metersToFeet(second)))
	})
	replaced = localizedMetersPattern.ReplaceAllStringFunc(replaced, func(match string) string {
		parts := localizedMetersPattern.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		meters, ok := parseLocalizedNumber(parts[1])
		if !ok {
			return match
		}
		return formatFeetForEnglish(metersToFeet(meters))
	})
	return strings.NewReplacer(
		"Fuß", "ft",
		"fuß", "ft",
		"Würfel", "cube",
	).Replace(replaced)
}

func parseLocalizedNumber(value string) (float64, bool) {
	normalized := strings.TrimSpace(strings.ReplaceAll(value, ",", "."))
	if normalized == "" {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(normalized, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func feetToMeters(feet float64) float64 {
	return math.Round((feet*0.3)*10) / 10
}

func metersToFeet(meters float64) float64 {
	return math.Round((meters/0.3)*10) / 10
}

func formatMetersForGerman(meters float64) string {
	return fmt.Sprintf("%s m", formatLocalizedGermanNumber(meters))
}

func formatLocalizedGermanNumber(value float64) string {
	if math.Abs(value-math.Round(value)) < 0.05 {
		return fmt.Sprintf("%.0f", math.Round(value))
	}
	return strings.ReplaceAll(strconv.FormatFloat(value, 'f', 1, 64), ".", ",")
}

func formatFeetForEnglish(feet float64) string {
	return fmt.Sprintf("%s ft", formatLocalizedEnglishNumber(feet))
}

func formatLocalizedEnglishNumber(value float64) string {
	if math.Abs(value-math.Round(value)) < 0.05 {
		return fmt.Sprintf("%.0f", math.Round(value))
	}
	return strconv.FormatFloat(value, 'f', 1, 64)
}
