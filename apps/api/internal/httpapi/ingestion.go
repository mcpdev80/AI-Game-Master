package httpapi

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/ledongthuc/pdf"
)

func extractDocumentText(path string) (string, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".pdf":
		return extractPDFText(path)
	default:
		content, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(content), nil
	}
}

func extractPDFText(path string) (string, error) {
	if text, err := extractPDFTextWithTool(path); err == nil && strings.TrimSpace(text) != "" {
		return text, nil
	}

	file, reader, err := pdf.Open(path)
	if err != nil {
		return "", fmt.Errorf("open pdf: %w", err)
	}
	defer file.Close()

	textReader, err := reader.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("read pdf text: %w", err)
	}

	var builder strings.Builder
	if _, err := io.Copy(&builder, textReader); err != nil {
		return "", fmt.Errorf("copy pdf text: %w", err)
	}

	text := builder.String()
	if strings.TrimSpace(text) != "" {
		return text, nil
	}

	return "", fmt.Errorf("empty pdf text extracted from %s", path)
}

func extractPDFTextWithTool(path string) (string, error) {
	if text, err := runPDFTextTool("pdftotext", []string{"-layout", path, "-"}); err == nil && strings.TrimSpace(text) != "" {
		return text, nil
	}
	return runPDFTextTool("mutool", []string{"draw", "-F", "txt", path})
}

func runPDFTextTool(name string, args []string) (string, error) {
	cmd := exec.Command(name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if strings.TrimSpace(stderr.String()) != "" {
			return "", fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(stderr.String()))
		}
		return "", fmt.Errorf("%s: %w", name, err)
	}
	return stdout.String(), nil
}

func chunkDocumentText(text string, chunkSize int) []string {
	clean := normalizeWhitespace(text)
	if clean == "" {
		return []string{}
	}
	if chunkSize <= 0 {
		chunkSize = 1200
	}

	paragraphs := strings.Split(clean, "\n\n")
	chunks := make([]string, 0, len(paragraphs))
	var current strings.Builder

	flush := func() {
		value := strings.TrimSpace(current.String())
		if value != "" {
			chunks = append(chunks, value)
		}
		current.Reset()
	}

	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}

		if len(paragraph) > chunkSize {
			words := strings.Fields(paragraph)
			for _, word := range words {
				if current.Len()+len(word)+1 > chunkSize && current.Len() > 0 {
					flush()
				}
				if current.Len() > 0 {
					current.WriteByte(' ')
				}
				current.WriteString(word)
			}
			continue
		}

		if current.Len()+len(paragraph)+2 > chunkSize && current.Len() > 0 {
			flush()
		}

		if current.Len() > 0 {
			current.WriteString("\n\n")
		}
		current.WriteString(paragraph)
	}

	flush()
	return chunks
}

func extractMonsterReferences(documentName string, chunks []string) []MonsterReference {
	if !strings.Contains(strings.ToLower(documentName), "monster") && !strings.Contains(strings.ToLower(documentName), "monter") {
		return []MonsterReference{}
	}

	headingPattern := regexp.MustCompile(`(?m)(^|[\s>])([A-ZÄÖÜ][A-ZÄÖÜa-zäöüß'’\-]{4,}(?:\s+[A-ZÄÖÜ][A-ZÄÖÜa-zäöüß'’\-]{2,}){0,3})`)
	seen := map[string]struct{}{}
	refs := make([]MonsterReference, 0)

	for index, chunk := range chunks {
		matches := headingPattern.FindAllStringSubmatch(chunk, -1)
		for _, match := range matches {
			name := strings.TrimSpace(match[2])
			if !looksLikeMonsterName(name) {
				continue
			}

			slug := slugifySearch(name)
			if slug == "" {
				continue
			}
			if _, ok := seen[slug]; ok {
				continue
			}
			seen[slug] = struct{}{}

			refs = append(refs, MonsterReference{
				Name:       name,
				NameSlug:   slug,
				ChunkIndex: index,
			})
		}
	}

	return refs
}

func looksLikeMonsterName(value string) bool {
	candidate := strings.TrimSpace(value)
	if len(candidate) < 5 || len(candidate) > 60 {
		return false
	}

	lower := strings.ToLower(candidate)
	disallowed := []string{
		"herausforderungsgrad",
		"rüstungsklasse",
		"trefferpunkte",
		"aktionen",
		"legendäre aktionen",
		"legendäre resistenz",
		"geschwindigkeit",
		"stärke",
		"geschicklichkeit",
		"konstitution",
		"intelligenz",
		"weisheit",
		"charisma",
		"kapitel",
		"monsterhandbuch",
	}
	for _, item := range disallowed {
		if lower == item {
			return false
		}
	}

	words := strings.Fields(candidate)
	if len(words) > 4 {
		return false
	}

	upperLike := 0
	for _, r := range candidate {
		if r >= 'A' && r <= 'Z' {
			upperLike++
		}
	}

	return upperLike >= 1
}

func normalizeWhitespace(text string) string {
	lines := strings.Split(text, "\n")
	clean := make([]string, 0, len(lines))
	blankCount := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			blankCount++
			if blankCount <= 1 {
				clean = append(clean, "")
			}
			continue
		}

		blankCount = 0
		clean = append(clean, trimmed)
	}

	return strings.TrimSpace(strings.Join(clean, "\n"))
}

func extractCharacterFromText(text string) Character {
	clean := normalizeWhitespace(firstCharacterSheetPage(text))
	lines := strings.Split(clean, "\n")
	character := Character{
		Abilities: map[string]int{},
		Languages: []string{},
		Features:  []string{},
		Metadata:  map[string]any{},
	}

	lineBefore := func(label string) string {
		for index, line := range lines {
			if strings.EqualFold(strings.TrimSpace(line), label) && index > 0 {
				for cursor := index - 1; cursor >= 0 && cursor >= index-3; cursor-- {
					value := strings.TrimSpace(lines[cursor])
					if value == "" || looksLikeCharacterLabel(value) {
						continue
					}
					return value
				}
			}
		}
		return ""
	}

	character.Name = lineBefore("CHARAKTERNAME")
	character.ClassAndLevel = lineBefore("KLASSE & STUFE")
	character.Background = lineBefore("HINTERGRUND")
	character.PlayerName = lineBefore("NAME DES SPIELERS")
	character.Race = lineBefore("VOLK")
	character.Alignment = lineBefore("GESINNUNG")

	character.Abilities["strength"] = extractAbilityScore(lines, "STÄRKE")
	character.Abilities["dexterity"] = extractAbilityScore(lines, "GESCHICKLICHKEIT")
	character.Abilities["constitution"] = extractAbilityScore(lines, "KONSTITUTION")
	character.Abilities["intelligence"] = extractAbilityScore(lines, "INTELLIGENZ")
	character.Abilities["wisdom"] = extractAbilityScore(lines, "WEISHEIT")
	character.Abilities["charisma"] = extractAbilityScore(lines, "CHARISMA")

	character.Proficiency = firstNonEmpty(extractRegexValue(clean, `\+(\d+)\s+ÜBUNGSBONUS`), extractRegexValue(clean, `ÜBUNGSBONUS\s+\+?(\d+)`))
	if character.Proficiency != "" && !strings.HasPrefix(character.Proficiency, "+") {
		character.Proficiency = "+" + character.Proficiency
	}

	character.Speed = firstNonEmpty(extractRegexValue(clean, `BEWEGUNGSRATE\s+(\d+)`), extractRegexValue(clean, `INITIATIVE\s+[+\-]?\d+\s+(\d+)\s+BEWEGUNGSRATE`))
	if character.Speed != "" && !strings.HasSuffix(character.Speed, " ft") && !strings.HasSuffix(character.Speed, " m") {
		character.Speed += " ft"
	}

	if armorClass := extractNumericNearLabel(lines, "RÜSTUNGSKLASSE"); armorClass != nil {
		character.ArmorClass = armorClass
	}
	if hitPoints := extractNumericNearLabel(lines, "Trefferpunkte Maximum"); hitPoints != nil {
		character.HitPointMax = hitPoints
	}

	character.Languages = extractKnownTerms(clean, []string{"Gemeinsprache", "Elfish", "Infernal", "Celestiisch", "Celestisch", "Drakonisch", "Zwergisch", "Orkisch"})
	character.Features = extractFeatureBlock(clean)

	character.Metadata["parser"] = "character_sheet_v1"
	return character
}

func firstCharacterSheetPage(text string) string {
	if strings.Contains(text, "\f") {
		return strings.Split(text, "\f")[0]
	}
	header := "5E-kompatibler Charakterbogen"
	first := strings.Index(text, header)
	if first == -1 {
		return text
	}
	second := strings.Index(text[first+len(header):], header)
	if second == -1 {
		return text
	}
	return text[:first+len(header)+second]
}

func looksLikeCharacterLabel(value string) bool {
	upper := strings.ToUpper(strings.TrimSpace(value))
	labels := map[string]struct{}{
		"CHARAKTERNAME": {}, "KLASSE & STUFE": {}, "HINTERGRUND": {}, "NAME DES SPIELERS": {},
		"VOLK": {}, "GESINNUNG": {}, "STÄRKE": {}, "GESCHICKLICHKEIT": {}, "KONSTITUTION": {},
		"INTELLIGENZ": {}, "WEISHEIT": {}, "CHARISMA": {}, "ÜBUNG UND SPRACHEN": {}, "KLASSENMERKMALE": {},
	}
	_, ok := labels[upper]
	return ok
}

func extractAbilityScore(lines []string, label string) int {
	for index, line := range lines {
		if !strings.EqualFold(strings.TrimSpace(line), label) {
			continue
		}
		for cursor := index + 1; cursor < len(lines) && cursor <= index+4; cursor++ {
			value := strings.TrimSpace(lines[cursor])
			if value == "" {
				continue
			}
			parsed, err := strconv.Atoi(value)
			if err == nil && parsed >= 1 && parsed <= 30 {
				return parsed
			}
		}
	}
	return 0
}

func extractNumericNearLabel(lines []string, label string) *int {
	for index, line := range lines {
		if !strings.EqualFold(strings.TrimSpace(line), label) {
			continue
		}
		for _, cursor := range []int{index - 1, index - 2, index + 1, index + 2} {
			if cursor < 0 || cursor >= len(lines) {
				continue
			}
			value := strings.TrimSpace(lines[cursor])
			parsed, err := strconv.Atoi(value)
			if err == nil {
				return &parsed
			}
		}
	}
	return nil
}

func extractRegexValue(text string, pattern string) string {
	re := regexp.MustCompile(pattern)
	match := re.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func extractRegexInt(text string, pattern string) *int {
	value := extractRegexValue(text, pattern)
	if value == "" {
		return nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return nil
	}
	return &parsed
}

func extractKnownTerms(text string, terms []string) []string {
	found := make([]string, 0)
	seen := map[string]struct{}{}
	for _, term := range terms {
		if strings.Contains(strings.ToLower(text), strings.ToLower(term)) {
			if _, ok := seen[term]; ok {
				continue
			}
			seen[term] = struct{}{}
			found = append(found, term)
		}
	}
	return found
}

func extractFeatureBlock(text string) []string {
	start := strings.Index(strings.ToLower(text), strings.ToLower("KLASSENMERKMALE"))
	if start == -1 {
		return []string{}
	}
	block := text[start:]
	end := strings.Index(block, "\f")
	if end > 0 {
		block = block[:end]
	}
	lines := strings.Split(block, "\n")
	features := make([]string, 0)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.EqualFold(line, "KLASSENMERKMALE") {
			continue
		}
		if len(line) < 4 {
			continue
		}
		features = append(features, line)
		if len(features) >= 8 {
			break
		}
	}
	return features
}
