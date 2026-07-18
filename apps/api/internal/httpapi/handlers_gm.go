package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

var rulesDocumentPagePattern = regexp.MustCompile(`(?i)(?:seite|page|s\.|pg\.)\s*(\d{1,4})`)

func applyStateUpdatesToCharacters(ctx context.Context, store *Store, session Session, updates []StateUpdate) ([]Character, error) {
	changed := make([]Character, 0)
	for _, update := range updates {
		characterID := strings.TrimSpace(update.EntityID)
		if characterID == "" {
			continue
		}
		if strings.EqualFold(characterID, "session") || strings.EqualFold(characterID, "campaign") {
			continue
		}
		character, err := store.GetCharacter(ctx, characterID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			if strings.Contains(strings.ToLower(err.Error()), "invalid input syntax for type uuid") {
				continue
			}
			return nil, err
		}
		if character.CampaignID != nil && session.CampaignID != "" && *character.CampaignID != session.CampaignID {
			continue
		}

		metadata := defaultMetadata(character.Metadata)
		field := normalizeProgressField(update.Field)
		value := strings.TrimSpace(update.Value)
		changedThisCharacter := false

		switch field {
		case "experience_points", "xp", "ep":
			total := parseMetadataInt(metadata["experience_points"])
			if value != "" {
				total = parseMetadataInt(value)
			}
			total += update.Delta
			if total < 0 {
				total = 0
			}
			metadata["experience_points"] = fmt.Sprintf("%d", total)
			changedThisCharacter = true
		case "money", "gold", "current_money":
			total := parseMetadataInt(metadata["current_money_gp"])
			if value != "" {
				total = parseMetadataInt(value)
			}
			total += update.Delta
			if total < 0 {
				total = 0
			}
			metadata["current_money_gp"] = total
			metadata["current_money"] = fmt.Sprintf("%d gp", total)
			changedThisCharacter = true
		case "inventory_add", "item_add", "loot_add":
			items := metadataStringList(metadata["current_inventory"])
			items = mergeUniqueStrings(items, splitProgressItems(value)...)
			metadata["current_inventory"] = items
			changedThisCharacter = true
		case "inventory_remove", "item_remove", "loot_remove":
			items := metadataStringList(metadata["current_inventory"])
			items = removeStringValues(items, splitProgressItems(value)...)
			metadata["current_inventory"] = items
			changedThisCharacter = true
		case "level_up", "level_up_available", "level_up_ready":
			next := "true"
			if value != "" {
				next = value
			}
			metadata["level_up_available"] = next
			changedThisCharacter = true
		case "notes_add", "character_note":
			notes := metadataStringList(metadata["session_notes"])
			notes = mergeUniqueStrings(notes, value)
			metadata["session_notes"] = notes
			changedThisCharacter = true
		}

		if !changedThisCharacter {
			continue
		}
		character.Metadata = metadata
		updated, err := store.UpdateCharacter(ctx, character)
		if err != nil {
			return nil, err
		}
		changed = append(changed, updated)
	}
	return changed, nil
}

func applyStateUpdatesToSession(session *Session, updates []StateUpdate) {
	if session == nil {
		return
	}
	nextInventory := session.State.GroupInventory
	for _, update := range updates {
		entityID := strings.TrimSpace(update.EntityID)
		if !strings.EqualFold(entityID, "session") {
			continue
		}
		field := normalizeProgressField(update.Field)
		value := strings.TrimSpace(update.Value)
		switch field {
		case "group_gold":
			total := nextInventory.Gold
			if value != "" {
				total = parseMetadataInt(value)
			}
			total += update.Delta
			if total < 0 {
				total = 0
			}
			nextInventory.Gold = total
		case "group_inventory_add":
			nextInventory.Items = mergeUniqueStrings(nextInventory.Items, splitProgressItems(value)...)
		case "group_inventory_remove":
			nextInventory.Items = removeStringValues(nextInventory.Items, splitProgressItems(value)...)
		case "group_notes":
			if value != "" {
				nextInventory.Notes = strings.TrimSpace(strings.Join([]string{nextInventory.Notes, value}, "\n"))
			}
		}
	}
	session.State.GroupInventory = nextInventory
}

func parseProficiencyBonus(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(strings.TrimPrefix(value, "+"))
	if err != nil {
		return 0
	}
	return parsed
}

func abilityModifierFromAny(value any) int {
	switch typed := value.(type) {
	case int:
		return (typed - 10) / 2
	case float64:
		return (int(typed) - 10) / 2
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0
		}
		return (parsed - 10) / 2
	default:
		return 0
	}
}

func passivePerceptionForCharacter(character Character) int {
	metadata := defaultMetadata(character.Metadata)
	if existing := parseMetadataInt(metadata["passive_perception"]); existing > 0 {
		return existing
	}
	wisdom := abilityModifierFromAny(character.Abilities["wisdom"])
	total := 10 + wisdom
	for _, skill := range metadataStringList(metadata["skill_proficiencies"]) {
		normalized := strings.ToLower(strings.TrimSpace(skill))
		if normalized == "wahrnehmung" || normalized == "perception" {
			total += parseProficiencyBonus(character.Proficiency)
			break
		}
	}
	return total
}

func activeCharactersForSession(ctx context.Context, store *Store, session Session) ([]map[string]any, error) {
	slots, err := store.ListPlayerSlots(ctx, session.ID)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0)
	for _, slot := range slots {
		if slot.CharacterID == nil {
			continue
		}
		character, err := store.GetCharacter(ctx, *slot.CharacterID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return nil, err
		}
		items = append(items, map[string]any{
			"id":                  character.ID,
			"name":                character.Name,
			"player_name":         character.PlayerName,
			"slot_display":        slot.DisplayName,
			"status":              slot.Status,
			"class_and_level":     character.ClassAndLevel,
			"race":                character.Race,
			"background":          character.Background,
			"alignment":           character.Alignment,
			"armor_class":         character.ArmorClass,
			"speed":               character.Speed,
			"hit_point_max":       character.HitPointMax,
			"proficiency_bonus":   character.Proficiency,
			"abilities":           character.Abilities,
			"current_money":       defaultMetadata(character.Metadata)["current_money"],
			"current_inventory":   defaultMetadata(character.Metadata)["current_inventory"],
			"experience_points":   defaultMetadata(character.Metadata)["experience_points"],
			"level_up_available":  defaultMetadata(character.Metadata)["level_up_available"],
			"features":            character.Features,
			"languages":           character.Languages,
			"skill_proficiencies": metadataStringList(defaultMetadata(character.Metadata)["skill_proficiencies"]),
			"passive_perception":  passivePerceptionForCharacter(character),
			"combat_attacks":      defaultMetadata(character.Metadata)["combat_attacks"],
			"weapon_notes":        defaultMetadata(character.Metadata)["weapon_notes"],
			"starting_equipment":  defaultMetadata(character.Metadata)["starting_equipment"],
		})
	}
	return items, nil
}

func activeCharacterContextChunks(activeCharacters []map[string]any) []GMContextChunk {
	items := make([]GMContextChunk, 0, len(activeCharacters))
	for _, character := range activeCharacters {
		name := strings.TrimSpace(fmt.Sprintf("%v", character["name"]))
		if name == "" {
			name = "Charakter"
		}
		summaryParts := []string{
			fmt.Sprintf("Name: %s", name),
			fmt.Sprintf("Klasse/Stufe: %v", character["class_and_level"]),
			fmt.Sprintf("Volk: %v", character["race"]),
			fmt.Sprintf("Hintergrund: %v", character["background"]),
			fmt.Sprintf("Gesinnung: %v", character["alignment"]),
			fmt.Sprintf("RK: %v", character["armor_class"]),
			fmt.Sprintf("Bewegung: %v", character["speed"]),
			fmt.Sprintf("HP max: %v", character["hit_point_max"]),
			fmt.Sprintf("Übungsbonus: %v", character["proficiency_bonus"]),
			fmt.Sprintf("Werte: %v", character["abilities"]),
			fmt.Sprintf("Inventar: %v", character["current_inventory"]),
			fmt.Sprintf("Geld: %v", character["current_money"]),
			fmt.Sprintf("EP: %v", character["experience_points"]),
			fmt.Sprintf("Level-Up bereit: %v", character["level_up_available"]),
			fmt.Sprintf("Merkmale: %v", character["features"]),
			fmt.Sprintf("Sprachen: %v", character["languages"]),
			fmt.Sprintf("Fertigkeiten: %v", character["skill_proficiencies"]),
			fmt.Sprintf("Passive Wahrnehmung: %v", character["passive_perception"]),
			fmt.Sprintf("Kampfangriffe: %v", character["combat_attacks"]),
			fmt.Sprintf("Waffenhinweise: %v", character["weapon_notes"]),
			fmt.Sprintf("Startausrüstung: %v", character["starting_equipment"]),
		}
		items = append(items, GMContextChunk{
			DocumentID:   fmt.Sprintf("character-sheet:%v", character["id"]),
			DocumentName: fmt.Sprintf("Character Sheet: %s", name),
			ChunkText:    strings.Join(summaryParts, "\n"),
		})
	}
	return items
}

func buildScenePromptContext(session Session, req GMRespondRequest, workingSummary map[string]any, contextChunks []GMContextChunk) map[string]any {
	adventureChunks := filterAdventurePromptChunks(contextChunks)
	return map[string]any{
		"scene_context": map[string]any{
			"current_scene":    truncatePromptContextText(session.CurrentScene, 220),
			"current_location": truncatePromptContextText(session.CurrentLocation, 120),
			"scene_summary":    truncatePromptContextText(session.State.SceneSummary, 260),
			"scene_mode":       inferSceneMode(session, req),
		},
		"session_facts":     deriveSessionFacts(session, workingSummary, adventureChunks),
		"known_npcs":        deriveKnownNPCs(session, adventureChunks),
		"adventure_context": deriveAdventureContext(adventureChunks),
	}
}

var damageDicePattern = regexp.MustCompile(`(?i)\b(\d+\s*[wd]\s*\d+(?:\s*[+-]\s*\d+)?)\b`)

func ensureAttackFollowUpRollRequest(playerInput string, activeCharacters []map[string]any, request *RollRequest) *RollRequest {
	if request == nil || !strings.EqualFold(strings.TrimSpace(request.Type), "attack") || request.FollowUpOnSuccess != nil {
		return request
	}
	followUp := deriveDamageRollRequest(playerInput, activeCharacters)
	if followUp == nil {
		return request
	}
	copyRequest := *request
	copyRequest.FollowUpOnSuccess = followUp
	return &copyRequest
}

func deriveDamageRollRequest(playerInput string, activeCharacters []map[string]any) *RollRequest {
	source := strings.ToLower(strings.TrimSpace(playerInput))
	if source == "" {
		return nil
	}
	if dice, label := inferDamageDiceFromCharacters(source, activeCharacters); len(dice) > 0 {
		return &RollRequest{
			Type:         "damage",
			Label:        label,
			Dice:         dice,
			Reason:       "Wenn dein Angriff trifft, brauchen wir direkt den Schaden.",
			Instructions: fmt.Sprintf("Wirf jetzt %s für den Schaden.", strings.Join(dice, ", ")),
		}
	}
	if dice, label := inferDamageDiceFromKeywords(source); len(dice) > 0 {
		return &RollRequest{
			Type:         "damage",
			Label:        label,
			Dice:         dice,
			Reason:       "Wenn dein Angriff trifft, brauchen wir direkt den Schaden.",
			Instructions: fmt.Sprintf("Wirf jetzt %s für den Schaden.", strings.Join(dice, ", ")),
		}
	}
	return nil
}

func inferDamageDiceFromCharacters(playerInput string, activeCharacters []map[string]any) ([]string, string) {
	for _, character := range activeCharacters {
		combatAttackText := strings.TrimSpace(fmt.Sprintf("%v", character["combat_attacks"]))
		if combatAttackText != "" && combatAttackText != "<nil>" {
			matches := 0
			var matchedDice []string
			var matchedLabel string
			for _, line := range strings.Split(combatAttackText, "\n") {
				dice, label := inferDamageDiceFromText(playerInput, line)
				if len(dice) == 0 {
					continue
				}
				matches++
				matchedDice = dice
				matchedLabel = label
			}
			if matches == 1 {
				return matchedDice, matchedLabel
			}
		}
		for _, key := range []string{"weapon_notes", "starting_equipment"} {
			text := strings.TrimSpace(fmt.Sprintf("%v", character[key]))
			if text == "" || text == "<nil>" {
				continue
			}
			if dice, label := inferDamageDiceFromText(playerInput, text); len(dice) > 0 {
				return dice, label
			}
		}
	}
	return nil, ""
}

func inferDamageDiceFromText(playerInput, text string) ([]string, string) {
	lowerText := strings.ToLower(text)
	for weapon, label := range map[string]string{
		"langschwert":  "Schadenswurf mit dem Langschwert",
		"longsword":    "Schadenswurf mit dem Langschwert",
		"kurzschwert":  "Schadenswurf mit dem Kurzschwert",
		"rapier":       "Schadenswurf mit dem Rapier",
		"speer":        "Schadenswurf mit dem Speer",
		"streitkolben": "Schadenswurf mit dem Streitkolben",
	} {
		if strings.Contains(strings.ToLower(playerInput), weapon) || strings.Contains(lowerText, weapon) {
			if match := damageDicePattern.FindStringSubmatch(text); len(match) > 1 {
				return []string{normalizeDiceToken(match[1])}, label
			}
		}
	}
	return nil, ""
}

func inferDamageDiceFromKeywords(source string) ([]string, string) {
	for _, item := range []struct {
		match string
		dice  string
		label string
	}{
		{"langschwert", "1w8", "Schadenswurf mit dem Langschwert"},
		{"longsword", "1w8", "Schadenswurf mit dem Langschwert"},
		{"kurzschwert", "1w6", "Schadenswurf mit dem Kurzschwert"},
		{"rapier", "1w8", "Schadenswurf mit dem Rapier"},
		{"speer", "1w6", "Schadenswurf mit dem Speer"},
		{"streitkolben", "1w6", "Schadenswurf mit dem Streitkolben"},
		{"kriegshammer", "1w8", "Schadenswurf mit dem Kriegshammer"},
	} {
		if strings.Contains(source, item.match) {
			return []string{item.dice}, item.label
		}
	}
	return nil, ""
}

func normalizeDiceToken(input string) string {
	token := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(input), " ", ""), "d", "w"))
	return token
}

func filterAdventurePromptChunks(chunks []GMContextChunk) []GMContextChunk {
	items := make([]GMContextChunk, 0, len(chunks))
	for _, chunk := range chunks {
		if strings.HasPrefix(strings.TrimSpace(chunk.DocumentID), "character-sheet:") {
			continue
		}
		items = append(items, chunk)
	}
	return items
}

func inferSceneMode(session Session, req GMRespondRequest) string {
	source := strings.ToLower(strings.Join([]string{
		req.PlayerInput,
		session.CurrentScene,
		session.CurrentLocation,
		session.State.SceneSummary,
	}, " "))
	if req.DiceRoll != nil || containsAny(source,
		"angriff", "initiative", "kampf", "attack", "danger", "bedroh", "monster", "blut", "explosion", "flieht", "renn", "schrei",
	) {
		return "danger"
	}
	if containsAny(source,
		"rede", "sprich", "fragen", "frage", "überreden", "taeusch", "täusch", "einschüch", "verhand", "talk", "ask", "convince", "bargain",
	) {
		return "social"
	}
	if containsAny(source,
		"rast", "ruhe", "lager", "camp", "shop", "kaufen", "verkaufen", "downtime", "reiseplanung", "long rest", "short rest",
	) {
		return "downtime"
	}
	return "exploration"
}

func deriveSessionFacts(session Session, workingSummary map[string]any, adventureChunks []GMContextChunk) []string {
	facts := make([]string, 0, 5)
	seen := map[string]struct{}{}
	appendFact := func(text string) {
		text = normalizePromptFact(text)
		if text == "" {
			return
		}
		key := strings.ToLower(text)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		facts = append(facts, text)
	}

	appendFact(session.CurrentScene)
	if location := strings.TrimSpace(session.CurrentLocation); location != "" {
		appendFact(fmt.Sprintf("Ort: %s.", location))
	}
	appendFact(session.State.SceneSummary)

	for _, key := range []string{"recent_summary", "last_outcome", "important_state"} {
		appendFact(fmt.Sprintf("%v", defaultMetadata(workingSummary)[key]))
	}

	for _, chunk := range adventureChunks {
		for _, sentence := range firstNSentences(chunk.ChunkText, 2) {
			appendFact(sentence)
			if len(facts) >= 5 {
				return facts
			}
		}
	}
	if len(facts) > 5 {
		return facts[:5]
	}
	return facts
}

func deriveKnownNPCs(session Session, adventureChunks []GMContextChunk) []map[string]any {
	items := make([]map[string]any, 0, 3)
	for _, name := range compactStringList(defaultStringSlice(session.State.ActiveNPCs), 3, 80) {
		matches := collectSentencesContaining(adventureChunks, name, 2)
		visibleTraits := ""
		lastKnownState := ""
		if len(matches) > 0 {
			visibleTraits = truncatePromptContextText(matches[0], 140)
		}
		if len(matches) > 1 {
			lastKnownState = truncatePromptContextText(matches[1], 140)
		} else {
			lastKnownState = truncatePromptContextText(session.State.SceneSummary, 140)
		}
		items = append(items, map[string]any{
			"name":             name,
			"visible_traits":   visibleTraits,
			"last_known_state": lastKnownState,
		})
	}
	return items
}

func deriveAdventureContext(adventureChunks []GMContextChunk) []map[string]any {
	items := make([]map[string]any, 0, 2)
	for _, chunk := range adventureChunks {
		sentences := firstNSentences(chunk.ChunkText, 2)
		if len(sentences) == 0 {
			continue
		}
		items = append(items, map[string]any{
			"source":  truncatePromptContextText(chunk.DocumentName, 80),
			"summary": truncatePromptContextText(strings.Join(sentences, " "), 220),
		})
		if len(items) >= 2 {
			break
		}
	}
	return items
}

func collectSentencesContaining(chunks []GMContextChunk, needle string, limit int) []string {
	matches := make([]string, 0, limit)
	lowerNeedle := strings.ToLower(strings.TrimSpace(needle))
	if lowerNeedle == "" {
		return matches
	}
	for _, chunk := range chunks {
		for _, sentence := range firstNSentences(chunk.ChunkText, 6) {
			if strings.Contains(strings.ToLower(sentence), lowerNeedle) {
				matches = append(matches, sentence)
				if len(matches) >= limit {
					return matches
				}
			}
		}
	}
	return matches
}

func firstNSentences(text string, limit int) []string {
	if limit <= 0 {
		return []string{}
	}
	replacer := strings.NewReplacer("\n", ". ", "!", ". ", "?", ". ")
	parts := strings.Split(replacer.Replace(text), ". ")
	items := make([]string, 0, min(limit, len(parts)))
	for _, part := range parts {
		part = normalizePromptFact(part)
		if part == "" {
			continue
		}
		items = append(items, part)
		if len(items) >= limit {
			break
		}
	}
	return items
}

func normalizePromptFact(text string) string {
	text = truncatePromptContextText(text, 160)
	text = strings.TrimSpace(strings.Trim(text, "-• "))
	if text == "" {
		return ""
	}
	if !strings.HasSuffix(text, ".") {
		text += "."
	}
	return text
}

func truncatePromptContextText(text string, maxChars int) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if maxChars <= 0 || len([]rune(text)) <= maxChars {
		return text
	}
	runes := []rune(text)
	text = string(runes[:maxChars])
	if cut := strings.LastIndex(text, ". "); cut > maxChars/2 {
		text = text[:cut+1]
	}
	return strings.TrimSpace(text)
}

func containsAny(source string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(source, needle) {
			return true
		}
	}
	return false
}

func (h *Handler) gmRespond(c *gin.Context) {
	var req GMRespondRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid gm payload", err)
		return
	}

	session, err := h.store.GetSession(c.Request.Context(), req.SessionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
			return
		}
		errorResponse(c, http.StatusInternalServerError, "load session", err)
		return
	}

	documents, err := h.store.ListDocumentsForCampaign(c.Request.Context(), session.CampaignID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "load related documents", err)
		return
	}
	adventure, err := sessionAdventure(c.Request.Context(), h.store, session)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "load session adventure", err)
		return
	}
	adventureDocuments, err := sessionAdventureDocuments(c.Request.Context(), h.store, session)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "load adventure documents", err)
		return
	}
	rulebookDocuments, err := sessionRulebookDocuments(c.Request.Context(), h.store, session, documents)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "load session rulebooks", err)
		return
	}
	monsterName := detectMonsterName(req.PlayerInput)
	isRulesQuery := isLikelyRulesQuery(req.PlayerInput)
	contextLimit := 4
	if isRulesQuery {
		contextLimit = 3
	}
	if monsterName != "" {
		contextLimit = 2
	}

	activeSessionType := "campaign_play_session"
	activeProfile := "narration"
	activeSessionID := session.State.PlayLLMSessionID
	if isRulesQuery {
		activeSessionType = "rules_lookup_session"
		activeProfile = "rules"
		activeSessionID = session.State.RulesLLMSessionID
	}
	activeLLMSession, err := h.ensureScopedLLMSession(
		c.Request.Context(),
		"session",
		session.ID,
		activeSessionType,
		activeProfile,
		"",
		"",
		activeSessionID,
		nil,
		12000,
		map[string]any{"query_kind": activeProfile},
		map[string]any{"session_id": session.ID, "campaign_id": session.CampaignID},
	)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "load active llm session", err)
		return
	}
	summaryLLMSession, err := h.ensureScopedLLMSession(
		c.Request.Context(),
		"session",
		session.ID,
		"summary_session",
		"summary",
		"",
		"",
		session.State.SummarySessionID,
		nil,
		4000,
		map[string]any{"query_kind": "summary"},
		map[string]any{"session_id": session.ID, "campaign_id": session.CampaignID},
	)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "load summary llm session", err)
		return
	}
	if activeSessionType == "campaign_play_session" && activeLLMSession.ID != "" {
		session.State.PlayLLMSessionID = activeLLMSession.ID
	}
	if activeSessionType == "rules_lookup_session" && activeLLMSession.ID != "" {
		session.State.RulesLLMSessionID = activeLLMSession.ID
	}
	if summaryLLMSession.ID != "" {
		session.State.SummarySessionID = summaryLLMSession.ID
	}

	activeCharacters, err := activeCharactersForSession(c.Request.Context(), h.store, session)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "load active session characters", err)
		return
	}
	characterContextChunks := activeCharacterContextChunks(activeCharacters)
	contextChunks := make([]GMContextChunk, 0)
	if isRulesQuery {
		shortRulebookIDs := make([]string, 0, len(rulebookDocuments))
		rulebookIDs := make([]string, 0, len(rulebookDocuments))
		for _, document := range rulebookDocuments {
			if strings.EqualFold(strings.TrimSpace(fmt.Sprintf("%v", document.Metadata["kind"])), "short_rules_guide") {
				shortRulebookIDs = append(shortRulebookIDs, document.ID)
				continue
			}
			rulebookIDs = append(rulebookIDs, document.ID)
		}
		if len(shortRulebookIDs) > 0 {
			contextChunks, err = h.store.RetrieveRelevantChunksForDocuments(c.Request.Context(), shortRulebookIDs, req.PlayerInput, contextLimit, true)
			if err != nil {
				errorResponse(c, http.StatusInternalServerError, "retrieve short rules chunks", err)
				return
			}
		}
		if len(contextChunks) == 0 {
			if len(rulebookIDs) == 0 {
				rulebookIDs = append(rulebookIDs, shortRulebookIDs...)
			}
			contextChunks, err = h.store.RetrieveRelevantChunksForDocuments(c.Request.Context(), rulebookIDs, req.PlayerInput, contextLimit, true)
			if err != nil {
				errorResponse(c, http.StatusInternalServerError, "retrieve rulebook chunks", err)
				return
			}
		}
	} else {
		adventureIDs := make([]string, 0, len(adventureDocuments))
		for _, document := range adventureDocuments {
			adventureIDs = append(adventureIDs, document.ID)
		}
		contextChunks, err = h.store.RetrieveRelevantChunksForDocuments(c.Request.Context(), adventureIDs, req.PlayerInput, contextLimit, false)
		if err != nil {
			errorResponse(c, http.StatusInternalServerError, "retrieve adventure chunks", err)
			return
		}
	}
	if monsterName != "" {
		monsterChunks, err := h.store.RetrieveMonsterContext(c.Request.Context(), monsterName, contextLimit)
		if err == nil && len(monsterChunks) > 0 {
			contextChunks = monsterChunks
		}
	}
	if !isRulesQuery && len(characterContextChunks) > 0 {
		contextChunks = append(contextChunks, characterContextChunks...)
	}

	llmTimeout := 50 * time.Second
	if isRulesQuery {
		llmTimeout = 90 * time.Second
	}
	if monsterName != "" {
		llmTimeout = 120 * time.Second
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), llmTimeout)
	defer cancel()

	userMessageTime := time.Now().UTC()
	activeLLMSession.MessageHistory = appendLLMMessage(activeLLMSession.MessageHistory, "user", req.PlayerInput, userMessageTime)
	liveHistory, archivedFacts := compactLLMHistory(activeLLMSession.MessageHistory, activeLLMSession.LiveTurnWindow)
	if len(archivedFacts) > 0 {
		activeLLMSession.Facts = append(defaultStringSlice(activeLLMSession.Facts), archivedFacts...)
		if len(activeLLMSession.Facts) > 8 {
			activeLLMSession.Facts = activeLLMSession.Facts[len(activeLLMSession.Facts)-8:]
		}
		activeLLMSession.LastSummarizedAt = timePtrOrNil(time.Now().UTC())
	}
	activeLLMSession.MessageHistory = liveHistory
	activeLLMSession.EstimatedSummaryTokens = estimatePromptTokens(messageHistoryToStrings(activeLLMSession.MessageHistory))
	scenePromptContext := buildScenePromptContext(session, req, summaryLLMSession.WorkingSummary, contextChunks)
	response, err := h.llmClient.CompleteGMResponse(
		ctx,
		session,
		req,
		append(append([]Document{}, adventureDocuments...), rulebookDocuments...),
		contextChunks,
		monsterName,
		isRulesQuery,
		recentHistory(activeLLMSession.MessageHistory, activeLLMSession.LiveTurnWindow),
		summaryLLMSession.WorkingSummary,
		activeCharacters,
		scenePromptContext,
	)
	if err != nil {
		if !isRulesQuery {
			if retryResponse, retryErr := h.llmClient.CompleteGMSceneNarrationFallback(
				ctx,
				session,
				req,
				append(adventureDocuments, rulebookDocuments...),
				contextChunks,
				monsterName,
				recentHistory(activeLLMSession.MessageHistory, activeLLMSession.LiveTurnWindow),
				summaryLLMSession.WorkingSummary,
				activeCharacters,
				scenePromptContext,
			); retryErr == nil {
				response = retryResponse
				response.DMNotes = append(response.DMNotes, "JSON scene response failed; prose fallback used", err.Error())
			} else {
				response = fallbackGMResponse(session, req, err)
				response.DMNotes = append(response.DMNotes, "Scene prose fallback failed", retryErr.Error())
			}
		} else {
			response = fallbackGMResponse(session, req, err)
		}
	}
	response.ContextChunks = contextChunks
	response.RollRequest = ensureAttackFollowUpRollRequest(req.PlayerInput, activeCharacters, response.RollRequest)
	response.RollRequest = sanitizeInvalidCombatRollRequest(response.RollRequest)
	response.RollRequest = sanitizeRollRequestForDisplay(response.RollRequest)
	if response.RollRequest != nil && req.DiceRoll == nil {
		response.Narration = buildRollRequestNarration(response.Language, response.RollRequest)
		if strings.TrimSpace(response.Narration) == "" {
			response.Narration = "Bevor ich die Szene auflöse, brauche ich erst deinen Wurf."
		}
	}
	activeLLMSession.MessageHistory = appendLLMMessage(activeLLMSession.MessageHistory, "assistant", response.Narration, time.Now().UTC())
	activeLLMSession.LastActiveAt = time.Now().UTC()
	activeLLMSession.EstimatedPromptTokens = estimatePromptTokens(messageHistoryToStrings(activeLLMSession.MessageHistory))
	activeLLMSession.Status = "active"
	var sceneAsset *Asset
	if session.AdventureID != nil {
		resolvedAsset, resolveErr := h.resolveAdventureSceneAsset(c.Request.Context(), *session.AdventureID, response.SceneEvents)
		if resolveErr != nil {
			errorResponse(c, http.StatusInternalServerError, "resolve scene image", resolveErr)
			return
		}
		sceneAsset = resolvedAsset
	}

	nextState := session.State
	nextState.LastNarration = response.Narration
	nextState.LastDMNotes = response.DMNotes
	nextState.SceneSummary = firstNonEmpty(response.Narration, session.State.SceneSummary)
	if response.RollRequest != nil && req.DiceRoll == nil {
		nextState.VisualMode = "dice_capture"
		nextState.VisualPayload = map[string]any{
			"type":                 "roll_request",
			"roll_type":            strings.TrimSpace(response.RollRequest.Type),
			"roll_label":           strings.TrimSpace(response.RollRequest.Label),
			"roll_dice":            response.RollRequest.Dice,
			"roll_ability":         strings.TrimSpace(response.RollRequest.Ability),
			"roll_skill":           strings.TrimSpace(response.RollRequest.Skill),
			"roll_reason":          strings.TrimSpace(response.RollRequest.Reason),
			"instructions":         strings.TrimSpace(response.RollRequest.Instructions),
			"narration":            response.Narration,
			"pending_player_input": strings.TrimSpace(req.PlayerInput),
		}
		if response.RollRequest.DC != nil && *response.RollRequest.DC > 0 {
			nextState.VisualPayload["roll_dc"] = *response.RollRequest.DC
		}
		if response.RollRequest.HideDC {
			nextState.VisualPayload["hide_dc"] = true
		}
		if response.RollRequest.FollowUpOnSuccess != nil {
			nextState.VisualPayload["follow_up_on_success"] = response.RollRequest.FollowUpOnSuccess
		}
		nextState.VoiceMode = "narrator"
		nextState.ActiveSpeakerRole = "narrator"
		nextState.ActiveVoiceProfileID = "narrator-default"
		nextState.TTSStatus = "queued"
	} else if req.DiceRoll != nil {
		nextState.LastDiceRoll = req.DiceRoll
		nextState.LastConfirmedRoll = req.DiceRoll
	}
	if len(response.SceneEvents) > 0 {
		nextState.ActiveMediaCue = response.SceneEvents[0].Name
	}
	if isRulesQuery && len(response.ContextChunks) > 0 {
		pageHint := extractRulesDocumentPageHint(req.PlayerInput)
		nextState.VisualMode = "rules_reference"
		nextState.VisualPayload = map[string]any{
			"document_id":   response.ContextChunks[0].DocumentID,
			"document_name": response.ContextChunks[0].DocumentName,
			"excerpt":       response.ContextChunks[0].ChunkText,
		}
		if pageHint > 0 {
			nextState.VisualPayload["document_page"] = pageHint
		}
		nextState.VoiceMode = "rules"
		nextState.ActiveSpeakerRole = "rules"
		nextState.ActiveVoiceProfileID = "rules-neutral"
		nextState.TTSStatus = "queued"
	} else if response.RollRequest == nil {
		nextState.VisualMode = "scene"
		nextState.VisualPayload = map[string]any{
			"title":     firstNonEmpty(adventureName(adventure), session.Name),
			"scene":     firstNonEmpty(session.CurrentScene, response.Narration),
			"narration": response.Narration,
		}
		if sceneAsset != nil {
			nextState.VisualPayload["image_asset_id"] = sceneAsset.ID
			nextState.VisualPayload["image_cue"] = metadataString(sceneAsset.Metadata, "cue_key")
			if strings.TrimSpace(nextState.ActiveMediaCue) == "" {
				nextState.ActiveMediaCue = metadataString(sceneAsset.Metadata, "cue_key")
			}
		}
		nextState.VoiceMode = "narrator"
		nextState.ActiveSpeakerRole = "narrator"
		nextState.ActiveVoiceProfileID = "narrator-default"
		nextState.TTSStatus = "queued"
	}
	if cueID, payload := guessAmbientCue(nextState.ActiveMediaCue, response.Narration, session.CurrentScene); cueID != "" {
		nextState.AmbientCueID = cueID
		nextState.AudioMode = "ambient_loop"
		nextState.AudioPayload = payload
	}
	nextState.SessionRecap = firstNonEmpty(response.Narration, nextState.SessionRecap)

	if err := h.store.UpdateSessionState(c.Request.Context(), session.ID, nextState); err != nil {
		errorResponse(c, http.StatusInternalServerError, "update session state", err)
		return
	}

	changedCharacters, err := applyStateUpdatesToCharacters(c.Request.Context(), h.store, session, response.StateUpdates)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "apply session progress to characters", err)
		return
	}
	session.State = nextState
	applyStateUpdatesToSession(&session, response.StateUpdates)
	nextState = session.State
	if err := h.store.UpdateSessionState(c.Request.Context(), session.ID, nextState); err != nil {
		errorResponse(c, http.StatusInternalServerError, "update session group inventory", err)
		return
	}

	activeLLMSession.WorkingSummary = map[string]any{
		"last_narration":          response.Narration,
		"scene_summary":           nextState.SceneSummary,
		"last_dm_notes":           nextState.LastDMNotes,
		"query_kind":              activeProfile,
		"character_updates_count": len(changedCharacters),
		"group_inventory":         nextState.GroupInventory,
	}
	activeLLMSession.StructuredState = map[string]any{
		"session_id":       session.ID,
		"campaign_id":      session.CampaignID,
		"current_scene":    session.CurrentScene,
		"current_location": session.CurrentLocation,
		"state":            nextState,
	}
	if _, err := h.store.UpdateLLMSession(c.Request.Context(), activeLLMSession); err != nil {
		errorResponse(c, http.StatusInternalServerError, "save active llm session", err)
		return
	}

	summaryLLMSession.LastActiveAt = time.Now().UTC()
	summaryLLMSession.WorkingSummary = buildSessionWorkingSummary(session, nextState, response)
	summaryLLMSession.Status = "active"
	summaryLLMSession.StructuredState = map[string]any{
		"session_id":       session.ID,
		"campaign_id":      session.CampaignID,
		"current_scene":    session.CurrentScene,
		"current_location": session.CurrentLocation,
		"state":            nextState,
	}
	summaryLLMSession.MessageHistory = appendLLMMessage(
		summaryLLMSession.MessageHistory,
		"system",
		fmt.Sprintf("Player: %s\nNarration: %s", strings.TrimSpace(req.PlayerInput), strings.TrimSpace(response.Narration)),
		time.Now().UTC(),
	)
	summaryLLMSession.Facts = deriveSessionFacts(session, summaryLLMSession.WorkingSummary, contextChunks)
	summaryLLMSession.OpenThreads = defaultStringSlice(session.State.OpenQuests)
	summaryLLMSession.EstimatedPromptTokens = estimatePromptTokens(messageHistoryToStrings(summaryLLMSession.MessageHistory))
	if _, err := h.store.UpdateLLMSession(c.Request.Context(), summaryLLMSession); err != nil {
		errorResponse(c, http.StatusInternalServerError, "save summary llm session", err)
		return
	}

	eventPayload := map[string]any{
		"player_input":            req.PlayerInput,
		"response":                response,
		"character_updates_count": len(changedCharacters),
	}
	if len(changedCharacters) > 0 {
		names := make([]string, 0, len(changedCharacters))
		for _, character := range changedCharacters {
			names = append(names, character.Name)
		}
		eventPayload["updated_characters"] = names
	}
	if _, err := h.store.CreateSessionEvent(c.Request.Context(), session.ID, "gm_response", eventPayload); err != nil {
		errorResponse(c, http.StatusInternalServerError, "persist session event", err)
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *Handler) generateSessionOpening(ctx context.Context, session Session) (GMResponse, error) {
	documents, err := h.store.ListDocumentsForCampaign(ctx, session.CampaignID)
	if err != nil {
		return GMResponse{}, err
	}
	adventure, err := sessionAdventure(ctx, h.store, session)
	if err != nil {
		return GMResponse{}, err
	}
	adventureDocuments, err := sessionAdventureDocuments(ctx, h.store, session)
	if err != nil {
		return GMResponse{}, err
	}
	rulebookDocuments, err := sessionRulebookDocuments(ctx, h.store, session, documents)
	if err != nil {
		return GMResponse{}, err
	}

	activeCharacters, err := activeCharactersForSession(ctx, h.store, session)
	if err != nil {
		return GMResponse{}, err
	}

	adventurePrompt := firstNonEmpty(session.Name, session.CurrentScene)
	if adventure != nil {
		adventurePrompt = strings.TrimSpace(strings.Join([]string{adventure.Name, adventure.Description}, ". "))
	}
	query := strings.TrimSpace(strings.Join([]string{
		"Sessionauftakt",
		adventurePrompt,
		session.CurrentScene,
		session.CurrentLocation,
	}, ". "))
	adventureIDs := make([]string, 0, len(adventureDocuments))
	for _, document := range adventureDocuments {
		adventureIDs = append(adventureIDs, document.ID)
	}
	contextChunks, err := h.store.RetrieveRelevantChunksForDocuments(ctx, adventureIDs, query, 4, false)
	if err != nil {
		return GMResponse{}, err
	}

	openingSession, err := h.ensureScopedLLMSession(
		ctx,
		"session",
		session.ID,
		"opening_session",
		"opening",
		"",
		"",
		"",
		nil,
		8000,
		map[string]any{"query_kind": "opening"},
		map[string]any{"session_id": session.ID, "campaign_id": session.CampaignID},
	)
	if err != nil {
		return GMResponse{}, err
	}

	req := GMRespondRequest{
		SessionID:   session.ID,
		PlayerInput: "__session_start__",
		Language:    chooseLanguage(session.Language, "de"),
	}

	llmCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	scenePromptContext := buildScenePromptContext(session, req, map[string]any{
		"session_phase": "opening",
	}, contextChunks)
	response, err := h.llmClient.CompleteGMResponse(
		llmCtx,
		session,
		req,
		append(adventureDocuments, rulebookDocuments...),
		contextChunks,
		"",
		false,
		recentHistory(openingSession.MessageHistory, 4),
		map[string]any{
			"session_phase": "opening",
		},
		activeCharacters,
		scenePromptContext,
	)
	if err != nil {
		return h.llmClient.CompleteGMSceneNarrationFallback(
			llmCtx,
			session,
			req,
			append(adventureDocuments, rulebookDocuments...),
			contextChunks,
			"",
			recentHistory(openingSession.MessageHistory, 4),
			map[string]any{
				"session_phase": "opening",
			},
			activeCharacters,
			scenePromptContext,
		)
	}
	return response, nil
}

func (h *Handler) generateSessionReopening(ctx context.Context, session Session) (GMResponse, error) {
	documents, err := h.store.ListDocumentsForCampaign(ctx, session.CampaignID)
	if err != nil {
		return GMResponse{}, err
	}
	adventureDocuments, err := sessionAdventureDocuments(ctx, h.store, session)
	if err != nil {
		return GMResponse{}, err
	}
	rulebookDocuments, err := sessionRulebookDocuments(ctx, h.store, session, documents)
	if err != nil {
		return GMResponse{}, err
	}
	activeCharacters, err := activeCharactersForSession(ctx, h.store, session)
	if err != nil {
		return GMResponse{}, err
	}

	query := strings.TrimSpace(strings.Join([]string{
		"Session fortsetzen",
		session.Name,
		session.CurrentScene,
		session.CurrentLocation,
		session.State.SessionRecap,
		session.State.SceneSummary,
	}, ". "))
	adventureIDs := make([]string, 0, len(adventureDocuments))
	for _, document := range adventureDocuments {
		adventureIDs = append(adventureIDs, document.ID)
	}
	contextChunks, err := h.store.RetrieveRelevantChunksForDocuments(ctx, adventureIDs, query, 3, false)
	if err != nil {
		return GMResponse{}, err
	}

	reopeningSession, err := h.ensureScopedLLMSession(
		ctx,
		"session",
		session.ID,
		"reopening_session",
		"reopening",
		"",
		"",
		"",
		nil,
		6000,
		map[string]any{"query_kind": "reopening"},
		map[string]any{"session_id": session.ID, "campaign_id": session.CampaignID},
	)
	if err != nil {
		return GMResponse{}, err
	}

	req := GMRespondRequest{
		SessionID:   session.ID,
		PlayerInput: "__session_resume__",
		Language:    chooseLanguage(session.Language, "de"),
	}

	llmCtx, cancel := context.WithTimeout(ctx, 75*time.Second)
	defer cancel()

	scenePromptContext := buildScenePromptContext(session, req, map[string]any{
		"session_phase": "reopening",
		"session_recap": session.State.SessionRecap,
	}, contextChunks)
	response, err := h.llmClient.CompleteGMResponse(
		llmCtx,
		session,
		req,
		append(adventureDocuments, rulebookDocuments...),
		contextChunks,
		"",
		false,
		recentHistory(reopeningSession.MessageHistory, 4),
		map[string]any{
			"session_phase":  "reopening",
			"session_recap":  session.State.SessionRecap,
			"scene_summary":  session.State.SceneSummary,
			"last_narration": session.State.LastNarration,
		},
		activeCharacters,
		scenePromptContext,
	)
	if err != nil {
		return h.llmClient.CompleteGMSceneNarrationFallback(
			llmCtx,
			session,
			req,
			append(adventureDocuments, rulebookDocuments...),
			contextChunks,
			"",
			recentHistory(reopeningSession.MessageHistory, 4),
			map[string]any{
				"session_phase":  "reopening",
				"session_recap":  session.State.SessionRecap,
				"scene_summary":  session.State.SceneSummary,
				"last_narration": session.State.LastNarration,
			},
			activeCharacters,
			scenePromptContext,
		)
	}
	return response, nil
}

func hasMeaningfulSessionProgress(session Session) bool {
	placeholders := []string{
		"Session angelegt. Warte auf Spieler und Start.",
		"Die Session beginnt. Der AI DM übernimmt die Szene.",
	}
	values := []string{
		strings.TrimSpace(session.State.SessionRecap),
		strings.TrimSpace(session.State.SceneSummary),
		strings.TrimSpace(session.State.LastNarration),
	}
	for _, value := range values {
		if value == "" {
			continue
		}
		isPlaceholder := false
		for _, placeholder := range placeholders {
			if value == placeholder {
				isPlaceholder = true
				break
			}
		}
		if !isPlaceholder {
			return true
		}
	}
	return false
}

func fallbackGMResponse(session Session, req GMRespondRequest, llmErr error) GMResponse {
	language := chooseLanguage(req.Language, session.Language)
	narration := "Einen Moment lang stockt die Szene, doch die Situation bleibt angespannt."
	if req.DiceRoll != nil && len(req.DiceRoll.Dice) > 0 {
		narration = "Der Wurf fällt, und für einen Herzschlag hält die Szene den Atem an."
	}
	if req.DiceRoll == nil {
		narration += " Deine Aktion steht im Raum, und die Welt reagiert noch nicht sauber darauf. Beschreibe den Schritt noch einmal kurz oder präzisiere dein Ziel."
	}

	return GMResponse{
		SessionID: session.ID,
		Narration: narration,
		Language:  language,
		RulesUsed: []string{"fallback_resolution"},
		StateUpdates: []StateUpdate{
			{EntityID: "session", Field: "scene_summary", Value: narration},
		},
		SceneEvents:  []SceneEvent{},
		DMNotes:      []string{"LLM fallback aktiv", llmErr.Error()},
		PromptSource: "fallback",
		RawModel:     "fallback",
		CreatedAt:    time.Now().UTC(),
	}
}

func buildRollRequestNarration(language string, request *RollRequest) string {
	if request == nil {
		return ""
	}
	request = sanitizeRollRequestForDisplay(request)
	isGerman := strings.HasPrefix(strings.ToLower(strings.TrimSpace(language)), "de")
	label := strings.TrimSpace(request.Label)
	if label == "" {
		if isGerman {
			label = "Eine Probe ist fällig."
		} else {
			label = "A roll is required."
		}
	}
	parts := []string{label}
	if instructions := strings.TrimSpace(request.Instructions); instructions != "" {
		parts = append(parts, instructions)
	} else if len(request.Dice) > 0 {
		if isGerman {
			parts = append(parts, fmt.Sprintf("Würfle %s.", strings.Join(request.Dice, ", ")))
		} else {
			parts = append(parts, fmt.Sprintf("Roll %s.", strings.Join(request.Dice, ", ")))
		}
	}
	if request.DC != nil && !request.HideDC {
		if isGerman {
			parts = append(parts, fmt.Sprintf("Die Schwierigkeit liegt bei %d.", *request.DC))
		} else {
			parts = append(parts, fmt.Sprintf("The difficulty is %d.", *request.DC))
		}
	}
	if reason := strings.TrimSpace(request.Reason); reason != "" {
		parts = append(parts, reason)
	}
	return strings.Join(parts, " ")
}

func sanitizeRollRequestForDisplay(request *RollRequest) *RollRequest {
	if request == nil {
		return nil
	}
	sanitized := *request
	if sanitized.DC != nil && *sanitized.DC <= 0 {
		sanitized.DC = nil
	}
	if sanitized.HideDC {
		sanitized.Label = sanitizeHiddenThresholdText(sanitized.Label)
		sanitized.Reason = sanitizeHiddenThresholdText(sanitized.Reason)
		sanitized.Instructions = sanitizeHiddenThresholdText(sanitized.Instructions)
	}
	sanitized = simplifyPassivePerceptionRollRequest(sanitized)
	if sanitized.FollowUpOnSuccess != nil {
		sanitizedFollowUp := *sanitized.FollowUpOnSuccess
		sanitized.FollowUpOnSuccess = sanitizeRollRequestForDisplay(&sanitizedFollowUp)
	}
	return &sanitized
}

func simplifyPassivePerceptionRollRequest(request RollRequest) RollRequest {
	combined := strings.ToLower(strings.Join([]string{
		request.Label,
		request.Reason,
		request.Instructions,
		request.Skill,
	}, " "))
	isPerception := strings.Contains(combined, "wahrnehm") || strings.Contains(combined, "perception")
	if !isPerception {
		return request
	}
	if !strings.Contains(combined, "passiv") && !strings.Contains(combined, "passive") {
		return request
	}
	request.Label = "Wahrnehmung"
	request.Reason = strings.TrimSpace(firstNonEmpty(
		extractShortPerceptionReason(request.Reason),
		extractShortPerceptionReason(request.Instructions),
		"Prüfe, ob du an der Stelle etwas Verborgenes oder Ungewöhnliches bemerkst.",
	))
	request.Instructions = "Mach bitte einen Wahrnehmungswurf und addiere deinen Wahrnehmungs-Modifikator."
	return request
}

func extractShortPerceptionReason(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	lower := strings.ToLower(text)
	startMarkers := []string{
		"um zu erkennen, ob",
		"um zu erkennen, was",
		"prüfe, ob",
		"prüfe, was",
	}
	for _, marker := range startMarkers {
		if idx := strings.Index(lower, marker); idx >= 0 {
			candidate := strings.TrimSpace(text[idx:])
			candidate = strings.TrimSpace(strings.TrimRight(candidate, "."))
			if candidate != "" {
				return candidate + "."
			}
		}
	}
	return ""
}

func sanitizeInvalidCombatRollRequest(request *RollRequest) *RollRequest {
	if request == nil {
		return nil
	}
	combined := strings.ToLower(strings.Join([]string{
		request.Type,
		request.Label,
		request.Reason,
		request.Instructions,
	}, " "))
	if isGenericDefenseReactionPrompt(combined) {
		return nil
	}
	if request.DC != nil && *request.DC <= 0 {
		requestCopy := *request
		requestCopy.DC = nil
		return &requestCopy
	}
	return request
}

func isGenericDefenseReactionPrompt(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	defenseMarkers := []string{
		"reaktion auf den angriff",
		"verteidigen",
		"verteidigung",
		"gegenangriff",
		"ac erhöhen",
		"ausweichen oder gegenangriff",
		"wähle deine aktion",
		"wie reagierst du",
	}
	enemyAttackMarkers := []string{
		"greift dich",
		"greift sofort an",
		"angriff des gegners",
		"biss abzuwehren",
		"den biss abzuwehren",
	}
	hasDefenseMarker := false
	for _, marker := range defenseMarkers {
		if strings.Contains(text, marker) {
			hasDefenseMarker = true
			break
		}
	}
	if !hasDefenseMarker {
		return false
	}
	for _, marker := range enemyAttackMarkers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func sanitizeHiddenThresholdText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(?:dc|sg)\s*\d+\b`),
		regexp.MustCompile(`(?i)\b\d+\s*(?:oder|and)\s*h(?:ö|oe)her\b`),
		regexp.MustCompile(`(?i)\b(?:at least|mindestens)\s*\d+\b`),
		regexp.MustCompile(`(?i)\bbeat\s+\d+\b`),
		regexp.MustCompile(`(?i)\btriffst du bei\s+\d+\b`),
	}
	for _, pattern := range patterns {
		text = pattern.ReplaceAllString(text, "")
	}
	text = strings.Join(strings.Fields(text), " ")
	return strings.TrimSpace(strings.Trim(text, " ,.;:"))
}

func buildSessionWorkingSummary(session Session, nextState SessionState, response GMResponse) map[string]any {
	return map[string]any{
		"session_id":            session.ID,
		"campaign_id":           session.CampaignID,
		"current_scene":         session.CurrentScene,
		"current_location":      session.CurrentLocation,
		"scene_summary":         nextState.SceneSummary,
		"session_recap":         nextState.SessionRecap,
		"last_narration":        nextState.LastNarration,
		"active_npcs":           defaultStringSlice(nextState.ActiveNPCs),
		"open_quests":           defaultStringSlice(nextState.OpenQuests),
		"last_dm_notes":         defaultStringSlice(nextState.LastDMNotes),
		"last_rules_used":       defaultStringSlice(response.RulesUsed),
		"last_media_cue":        nextState.ActiveMediaCue,
		"group_inventory":       nextState.GroupInventory,
		"selected_rulebook_ids": nextState.SelectedRulebookIDs,
		"prompt_config":         nextState.PromptConfig,
		"last_response_at":      response.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func extractRulesDocumentPageHint(text string) int {
	match := rulesDocumentPagePattern.FindStringSubmatch(strings.TrimSpace(text))
	if len(match) < 2 {
		return 0
	}
	page, err := strconv.Atoi(match[1])
	if err != nil || page <= 0 {
		return 0
	}
	return page
}
