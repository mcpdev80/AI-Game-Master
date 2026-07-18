package httpapi

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

func extractRuleIndexEntries(documentName string, chunks []string) []RuleIndexEntry {
	if len(chunks) == 0 {
		return []RuleIndexEntry{}
	}

	now := timeNowUTC()
	documentName = sanitizeRuleIndexText(documentName)
	entries := make([]RuleIndexEntry, 0)
	seen := map[string]struct{}{}

	add := func(documentID string, chunkIndex int, category string, term string) {
		term = sanitizeRuleIndexText(term)
		term = strings.TrimSpace(term)
		if term == "" {
			return
		}
		slug := slugifySearch(term)
		if slug == "" {
			return
		}
		key := fmt.Sprintf("%s|%d|%s|%s", documentID, chunkIndex, category, slug)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		entries = append(entries, RuleIndexEntry{
			DocumentID: documentID,
			ChunkIndex: chunkIndex,
			Category:   category,
			Term:       truncatePromptContextText(term, 160),
			TermSlug:   slug,
			CreatedAt:  now,
		})
	}

	for chunkIndex, chunkText := range chunks {
		clean := sanitizeRuleIndexText(chunkText)
		clean = normalizeWhitespace(clean)
		if clean == "" {
			continue
		}
		category := ruleIndexCategoryForText(documentName, clean)
		summary := ruleIndexSummary(clean)
		add(documentName, chunkIndex, category, summary)

		for _, token := range ruleIndexTokens(clean) {
			add(documentName, chunkIndex, category, token)
		}
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].ChunkIndex == entries[j].ChunkIndex {
			if entries[i].Category == entries[j].Category {
				return entries[i].TermSlug < entries[j].TermSlug
			}
			return entries[i].Category < entries[j].Category
		}
		return entries[i].ChunkIndex < entries[j].ChunkIndex
	})

	return entries
}

func ruleIndexSummary(text string) string {
	text = sanitizeRuleIndexText(text)
	lines := strings.Split(normalizeWhitespace(text), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		return line
	}
	return truncatePromptContextText(text, 180)
}

func ruleIndexTokens(text string) []string {
	text = sanitizeRuleIndexText(text)
	replacer := strings.NewReplacer(
		".", " ",
		",", " ",
		":", " ",
		";", " ",
		"!", " ",
		"?", " ",
		"(", " ",
		")", " ",
		"[", " ",
		"]", " ",
		"{", " ",
		"}", " ",
		"/", " ",
		"\\", " ",
		"\"", " ",
		"'", " ",
		"-", " ",
	)
	parts := strings.Fields(replacer.Replace(strings.ToLower(text)))
	seen := map[string]struct{}{}
	tokens := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if len(part) < 3 {
			continue
		}
		if isRuleSearchStopword(part) {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		tokens = append(tokens, part)
	}
	return tokens
}

func sanitizeRuleIndexText(value string) string {
	return strings.ToValidUTF8(value, "")
}

func ruleIndexCategoryForText(documentName string, text string) string {
	lower := strings.ToLower(documentName + " " + text)
	switch {
	case strings.Contains(lower, "monster") || strings.Contains(lower, "monsterhandbuch"):
		return "monster"
	case strings.Contains(lower, "zauber") || strings.Contains(lower, "spell") || strings.Contains(lower, "cantrip"):
		return "spell"
	case strings.Contains(lower, "feat") || strings.Contains(lower, "talent"):
		return "feat"
	case strings.Contains(lower, "rüstung") || strings.Contains(lower, "armor") || strings.Contains(lower, "weapon") || strings.Contains(lower, "waffe"):
		return "equipment"
	case strings.Contains(lower, "beweg") || strings.Contains(lower, "speed") || strings.Contains(lower, "dunkelsicht") || strings.Contains(lower, "darkvision"):
		return "species"
	case strings.Contains(lower, "klasse") || strings.Contains(lower, "subclass") || strings.Contains(lower, "stufe"):
		return "class"
	default:
		return "chunk"
	}
}

func timeNowUTC() time.Time {
	return time.Now().UTC()
}
