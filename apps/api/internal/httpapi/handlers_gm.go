package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

var rulesDocumentPagePattern = regexp.MustCompile(`(?i)(?:seite|page|s\.|pg\.)\s*(\d{1,4})`)
var uuidLikePattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var encounterSuffixNumberPattern = regexp.MustCompile(`\s+\d+$`)

func isUUIDLike(value string) bool {
	return uuidLikePattern.MatchString(strings.TrimSpace(value))
}

func embeddedDocumentChunk(document Document) (GMContextChunk, bool) {
	content := strings.TrimSpace(fmt.Sprintf("%v", document.Metadata["embedded_content"]))
	if content == "" {
		content = strings.TrimSpace(fmt.Sprintf("%v", document.Metadata["guide_content"]))
	}
	if content == "" {
		return GMContextChunk{}, false
	}
	return GMContextChunk{
		DocumentID:   strings.TrimSpace(document.ID),
		DocumentName: strings.TrimSpace(document.Name),
		ChunkText:    content,
	}, true
}

func (h *Handler) retrieveRelevantContextForDocuments(ctx context.Context, documents []Document, query string, limit int, prioritizeRules bool) ([]GMContextChunk, error) {
	if limit <= 0 {
		limit = 4
	}
	if len(documents) == 0 {
		return []GMContextChunk{}, nil
	}

	dbDocumentIDs := make([]string, 0, len(documents))
	embeddedChunks := make([]GMContextChunk, 0, len(documents))
	for _, document := range documents {
		documentID := strings.TrimSpace(document.ID)
		if isUUIDLike(documentID) {
			dbDocumentIDs = append(dbDocumentIDs, documentID)
			continue
		}
		if chunk, ok := embeddedDocumentChunk(document); ok {
			embeddedChunks = append(embeddedChunks, chunk)
		}
	}

	contextChunks := make([]GMContextChunk, 0, limit)
	if len(dbDocumentIDs) > 0 {
		dbChunks, err := h.store.RetrieveRelevantChunksForDocuments(ctx, dbDocumentIDs, query, limit, prioritizeRules)
		if err != nil {
			return nil, err
		}
		contextChunks = append(contextChunks, dbChunks...)
	}

	if len(contextChunks) >= limit {
		return contextChunks[:limit], nil
	}
	for _, chunk := range embeddedChunks {
		contextChunks = append(contextChunks, chunk)
		if len(contextChunks) >= limit {
			break
		}
	}
	return contextChunks, nil
}

func ensureInteractiveNarration(language string, narration string, rollRequest *RollRequest) string {
	narration = strings.TrimSpace(narration)
	if narration == "" || rollRequest != nil {
		return narration
	}
	if strings.Contains(narration, "?") {
		return narration
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(language)), "de") {
		return strings.TrimSpace(narration + " Was tut ihr jetzt?")
	}
	return strings.TrimSpace(narration + " What do you do now?")
}

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
		reconcileCharacterLevelProgress(&character, field)
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

var dnd5eXPThresholds = map[int]int{
	1:  0,
	2:  300,
	3:  900,
	4:  2700,
	5:  6500,
	6:  14000,
	7:  23000,
	8:  34000,
	9:  48000,
	10: 64000,
	11: 85000,
	12: 100000,
	13: 120000,
	14: 140000,
	15: 165000,
	16: 195000,
	17: 225000,
	18: 265000,
	19: 305000,
	20: 355000,
}

func parseMetadataBool(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		normalized := strings.ToLower(strings.TrimSpace(typed))
		return normalized == "true" || normalized == "1" || normalized == "yes" || normalized == "ja"
	case int:
		return typed != 0
	case float64:
		return typed != 0
	default:
		return false
	}
}

func parseCharacterLevel(classAndLevel string) int {
	classAndLevel = strings.TrimSpace(classAndLevel)
	if classAndLevel == "" {
		return 1
	}
	if entries := parseCharacterClassLevels(classAndLevel); len(entries) > 0 {
		total := 0
		for _, entry := range entries {
			total += entry.Level
		}
		if total > 20 {
			return 20
		}
		if total > 0 {
			return total
		}
	}
	matches := regexp.MustCompile(`\b(\d{1,2})\b`).FindStringSubmatch(classAndLevel)
	if len(matches) < 2 {
		return 1
	}
	level, err := strconv.Atoi(matches[1])
	if err != nil || level < 1 {
		return 1
	}
	if level > 20 {
		return 20
	}
	return level
}

func nextLevelThreshold(level int) (int, bool) {
	if level >= 20 {
		return 0, false
	}
	threshold, ok := dnd5eXPThresholds[level+1]
	return threshold, ok
}

func characterEligibleForLevelUp(character Character) bool {
	level := parseCharacterLevel(character.ClassAndLevel)
	threshold, ok := nextLevelThreshold(level)
	if !ok {
		return false
	}
	metadata := defaultMetadata(character.Metadata)
	if parseMetadataBool(metadata["level_up_available"]) {
		return true
	}
	return parseMetadataInt(metadata["experience_points"]) >= threshold
}

func reconcileCharacterLevelProgress(character *Character, triggerField string) {
	if character == nil {
		return
	}
	metadata := defaultMetadata(character.Metadata)
	level := parseCharacterLevel(character.ClassAndLevel)
	manualReady := parseMetadataBool(metadata["level_up_available"])
	threshold, hasNextLevel := nextLevelThreshold(level)
	if !hasNextLevel {
		metadata["level_up_available"] = "false"
		character.Metadata = metadata
		return
	}
	xpReady := parseMetadataInt(metadata["experience_points"]) >= threshold
	if manualReady || xpReady {
		metadata["level_up_available"] = "true"
	} else if triggerField == "experience_points" || triggerField == "xp" || triggerField == "ep" {
		metadata["level_up_available"] = "false"
	}
	character.Metadata = metadata
}

func loadActiveSessionCharacters(ctx context.Context, store *Store, session Session) ([]Character, error) {
	slots, err := store.ListPlayerSlots(ctx, session.ID)
	if err != nil {
		return nil, err
	}
	items := make([]Character, 0, len(slots))
	seen := map[string]struct{}{}
	for _, slot := range slots {
		if slot.CharacterID == nil || *slot.CharacterID == "" {
			continue
		}
		if _, ok := seen[*slot.CharacterID]; ok {
			continue
		}
		seen[*slot.CharacterID] = struct{}{}
		character, err := store.GetCharacter(ctx, *slot.CharacterID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return nil, err
		}
		items = append(items, character)
	}
	return items, nil
}

func detectsCombatResolution(text string) bool {
	source := strings.ToLower(strings.TrimSpace(text))
	return containsAny(source,
		"kampf ist vorbei", "kampf vorbei", "kampf endet", "combat is over", "combat over", "encounter over",
		"letzter gegner", "last enemy", "gegner besiegt", "enemy defeated", "victory", "sieg",
		"wir haben gewonnen", "we won", "begegnung beendet", "fight is done", "fight is over",
	)
}

func detectsRestTransition(text string) bool {
	source := strings.ToLower(strings.TrimSpace(text))
	return containsAny(source,
		"long rest", "short rest", "rast", "ruhe", "wir rasten", "wir ruhen", "camp", "lager aufschlagen", "downtime",
	)
}

func buildLevelUpQueue(characters []Character, language string) []LevelUpQueueEntry {
	queue := make([]LevelUpQueueEntry, 0)
	for _, character := range characters {
		if !characterEligibleForLevelUp(character) {
			continue
		}
		currentLevel := parseCharacterLevel(character.ClassAndLevel)
		entry := LevelUpQueueEntry{
			CharacterID:   character.ID,
			CharacterName: character.Name,
			CurrentLevel:  currentLevel,
			NextLevel:     min(currentLevel+1, 20),
		}
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(language)), "de") {
			entry.Reason = "Stufenaufstieg nach Rast bereit"
		} else {
			entry.Reason = "Level-up ready after rest"
		}
		queue = append(queue, entry)
	}
	return queue
}

func buildRewardSummary(updates []StateUpdate, language string) string {
	xpTotal := 0
	groupGold := 0
	items := make([]string, 0)
	for _, update := range updates {
		field := normalizeProgressField(update.Field)
		switch field {
		case "experience_points", "xp", "ep":
			if update.Delta != 0 {
				xpTotal += update.Delta
			} else if strings.TrimSpace(update.Value) != "" {
				xpTotal += parseMetadataInt(update.Value)
			}
		case "group_gold":
			groupGold += update.Delta
		case "group_inventory_add", "inventory_add", "item_add", "loot_add":
			items = append(items, splitProgressItems(update.Value)...)
		}
	}
	parts := make([]string, 0, 3)
	if xpTotal > 0 {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(language)), "de") {
			parts = append(parts, fmt.Sprintf("%d EP vergeben", xpTotal))
		} else {
			parts = append(parts, fmt.Sprintf("%d XP awarded", xpTotal))
		}
	}
	if groupGold > 0 {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(language)), "de") {
			parts = append(parts, fmt.Sprintf("%d Gruppen-Gold", groupGold))
		} else {
			parts = append(parts, fmt.Sprintf("%d group gold", groupGold))
		}
	}
	if len(items) > 0 {
		label := strings.Join(items, ", ")
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(language)), "de") {
			parts = append(parts, fmt.Sprintf("Beute: %s", label))
		} else {
			parts = append(parts, fmt.Sprintf("Loot: %s", label))
		}
	}
	return strings.Join(parts, " · ")
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
		privateSidebarContext := ""
		if privateMessages, err := store.ListPrivateChatMessages(ctx, session.ID, slot.ID, 8); err == nil {
			privateSidebarContext = summarizePrivateSidebarMessages(privateMessages)
		}
		items = append(items, map[string]any{
			"id":                      character.ID,
			"name":                    character.Name,
			"player_name":             character.PlayerName,
			"slot_display":            slot.DisplayName,
			"status":                  slot.Status,
			"class_and_level":         character.ClassAndLevel,
			"race":                    character.Race,
			"background":              character.Background,
			"alignment":               character.Alignment,
			"armor_class":             character.ArmorClass,
			"speed":                   character.Speed,
			"hit_point_max":           character.HitPointMax,
			"proficiency_bonus":       character.Proficiency,
			"abilities":               character.Abilities,
			"current_money":           defaultMetadata(character.Metadata)["current_money"],
			"current_inventory":       defaultMetadata(character.Metadata)["current_inventory"],
			"experience_points":       defaultMetadata(character.Metadata)["experience_points"],
			"level_up_available":      defaultMetadata(character.Metadata)["level_up_available"],
			"features":                character.Features,
			"languages":               character.Languages,
			"senses":                  defaultMetadata(character.Metadata)["senses"],
			"skill_proficiencies":     metadataStringList(defaultMetadata(character.Metadata)["skill_proficiencies"]),
			"passive_perception":      passivePerceptionForCharacter(character),
			"combat_attacks":          defaultMetadata(character.Metadata)["combat_attacks"],
			"weapon_notes":            defaultMetadata(character.Metadata)["weapon_notes"],
			"starting_equipment":      defaultMetadata(character.Metadata)["starting_equipment"],
			"private_sidebar_context": privateSidebarContext,
			"control_mode":            "player",
			"participant_type":        "player_character",
			"tactics_note":            "",
			"current_hit_points":      defaultMetadata(character.Metadata)["current_hit_points"],
			"temporary_hit_points":    defaultMetadata(character.Metadata)["temporary_hit_points"],
			"death_save_successes":    defaultMetadata(character.Metadata)["death_save_successes"],
			"death_save_failures":     defaultMetadata(character.Metadata)["death_save_failures"],
			"death_save_stable":       defaultMetadata(character.Metadata)["death_save_stable"],
		})
	}
	companions, err := store.ListSessionCompanions(ctx, session.ID)
	if err != nil {
		return nil, err
	}
	for _, companion := range companions {
		character, err := store.GetCharacter(ctx, companion.CharacterID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return nil, err
		}
		name := strings.TrimSpace(companion.DisplayName)
		if name == "" {
			name = character.Name
		}
		currentHP := any(nil)
		if companion.CurrentHitPoints != nil {
			currentHP = *companion.CurrentHitPoints
		} else {
			currentHP = defaultMetadata(character.Metadata)["current_hit_points"]
		}
		items = append(items, map[string]any{
			"id":                      companion.ID,
			"character_id":            character.ID,
			"name":                    name,
			"player_name":             "AI DM",
			"slot_display":            "DM Companion",
			"status":                  companion.Status,
			"class_and_level":         character.ClassAndLevel,
			"race":                    character.Race,
			"background":              character.Background,
			"alignment":               character.Alignment,
			"armor_class":             character.ArmorClass,
			"speed":                   character.Speed,
			"hit_point_max":           character.HitPointMax,
			"proficiency_bonus":       character.Proficiency,
			"abilities":               character.Abilities,
			"current_money":           defaultMetadata(character.Metadata)["current_money"],
			"current_inventory":       defaultMetadata(character.Metadata)["current_inventory"],
			"experience_points":       defaultMetadata(character.Metadata)["experience_points"],
			"level_up_available":      defaultMetadata(character.Metadata)["level_up_available"],
			"features":                character.Features,
			"languages":               character.Languages,
			"senses":                  defaultMetadata(character.Metadata)["senses"],
			"skill_proficiencies":     metadataStringList(defaultMetadata(character.Metadata)["skill_proficiencies"]),
			"passive_perception":      passivePerceptionForCharacter(character),
			"combat_attacks":          defaultMetadata(character.Metadata)["combat_attacks"],
			"weapon_notes":            defaultMetadata(character.Metadata)["weapon_notes"],
			"starting_equipment":      defaultMetadata(character.Metadata)["starting_equipment"],
			"private_sidebar_context": "",
			"control_mode":            companion.ControlMode,
			"participant_type":        "dm_companion",
			"tactics_note":            companion.TacticsNote,
			"current_hit_points":      currentHP,
			"temporary_hit_points":    defaultMetadata(character.Metadata)["temporary_hit_points"],
			"death_save_successes":    defaultMetadata(character.Metadata)["death_save_successes"],
			"death_save_failures":     defaultMetadata(character.Metadata)["death_save_failures"],
			"death_save_stable":       defaultMetadata(character.Metadata)["death_save_stable"],
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
			fmt.Sprintf("Sinne: %v", character["senses"]),
			fmt.Sprintf("Fertigkeiten: %v", character["skill_proficiencies"]),
			fmt.Sprintf("Passive Wahrnehmung: %v", character["passive_perception"]),
			fmt.Sprintf("Kampfangriffe: %v", character["combat_attacks"]),
			fmt.Sprintf("Waffenhinweise: %v", character["weapon_notes"]),
			fmt.Sprintf("Startausrüstung: %v", character["starting_equipment"]),
			fmt.Sprintf("Steuerung: %v", character["control_mode"]),
			fmt.Sprintf("Teilnehmertyp: %v", character["participant_type"]),
			fmt.Sprintf("Taktik: %v", character["tactics_note"]),
			fmt.Sprintf("HP aktuell: %v", character["current_hit_points"]),
			fmt.Sprintf("Private Sidebar: %v", character["private_sidebar_context"]),
		}
		items = append(items, GMContextChunk{
			DocumentID:   fmt.Sprintf("character-sheet:%v", character["id"]),
			DocumentName: fmt.Sprintf("Character Sheet: %s", name),
			ChunkText:    strings.Join(summaryParts, "\n"),
		})
	}
	return items
}

func summarizePrivateSidebarMessages(messages []PrivateChatMessage) string {
	if len(messages) == 0 {
		return ""
	}
	start := 0
	if len(messages) > 6 {
		start = len(messages) - 6
	}
	lines := make([]string, 0, len(messages)-start)
	for _, message := range messages[start:] {
		role := strings.TrimSpace(message.Role)
		if role == "" {
			role = "message"
		}
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: %s", role, content))
	}
	return strings.Join(lines, " | ")
}

type enemyCombatProfile struct {
	Name        string
	AttackBonus int
	DamageDice  string
	DamageType  string
	ArmorClass  int
	HitPointMax int
}

type enemyEncounterDefinition struct {
	Keywords        []string
	PluralKeywords  []string
	BaseName        string
	VariantProfiles []enemyCombatProfile
}

var encounterDefinitions = srdEncounterDefinitions()

func localizedEncounterVariantName(baseName string, variantIndex int, language string) string {
	de := strings.HasPrefix(strings.ToLower(strings.TrimSpace(language)), "de")
	switch baseName {
	case "spider":
		switch variantIndex {
		case 0:
			if de {
				return "Jagdspinne"
			}
			return "Hunting Spider"
		case 1:
			if de {
				return "Junge Riesenspinne"
			}
			return "Young Giant Spider"
		default:
			if de {
				return "Riesenspinne"
			}
			return "Giant Spider"
		}
	case "goblin":
		switch variantIndex {
		case 0:
			if de {
				return "Goblinspäher"
			}
			return "Goblin Scout"
		case 1:
			if de {
				return "Goblinräuber"
			}
			return "Goblin Raider"
		default:
			if de {
				return "Goblinscharmützler"
			}
			return "Goblin Skirmisher"
		}
	case "wolf":
		switch variantIndex {
		case 0:
			if de {
				return "Junger Wolf"
			}
			return "Young Wolf"
		case 1:
			if de {
				return "Wolf"
			}
			return "Wolf"
		default:
			if de {
				return "Schreckenswolf"
			}
			return "Dire Wolf"
		}
	}
	return ""
}

func rollTotalFromEvent(event *DiceRollEvent) int {
	if event == nil {
		return 0
	}
	if event.Total > 0 {
		return event.Total
	}
	total := 0
	for _, die := range event.Dice {
		total += die.Value
	}
	return total
}

func parseLeadingInt(value string) int {
	matches := regexp.MustCompile(`\d+`).FindString(strings.TrimSpace(value))
	if matches == "" {
		return 0
	}
	parsed, err := strconv.Atoi(matches)
	if err != nil {
		return 0
	}
	return parsed
}

func parseIntFromAny(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case *int:
		if typed != nil {
			return *typed
		}
		return 0
	case *int32:
		if typed != nil {
			return int(*typed)
		}
		return 0
	case *int64:
		if typed != nil {
			return int(*typed)
		}
		return 0
	case string:
		return parseMetadataInt(typed)
	default:
		return parseMetadataInt(fmt.Sprintf("%v", value))
	}
}

func parseBoolFromAny(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		normalized := strings.ToLower(strings.TrimSpace(typed))
		return normalized == "true" || normalized == "1" || normalized == "yes" || normalized == "ja" || normalized == "stable" || normalized == "stabil"
	case int:
		return typed != 0
	default:
		return false
	}
}

func combatTurnFromActiveCharacter(character map[string]any, initiative int) CombatTurnEntry {
	participantType := strings.TrimSpace(fmt.Sprintf("%v", character["participant_type"]))
	controlMode := strings.TrimSpace(fmt.Sprintf("%v", character["control_mode"]))
	rawCharacterID, hasCharacterID := character["character_id"]
	characterID := ""
	if hasCharacterID && rawCharacterID != nil {
		characterID = strings.TrimSpace(fmt.Sprintf("%v", rawCharacterID))
	}
	side := "player"
	if participantType == "dm_companion" || controlMode == "dm" {
		side = "ally"
	}
	turn := CombatTurnEntry{
		ID:                 strings.TrimSpace(fmt.Sprintf("%v", character["id"])),
		CharacterID:        characterID,
		Name:               firstNonEmpty(strings.TrimSpace(fmt.Sprintf("%v", character["name"])), "Character"),
		Side:               side,
		ParticipantType:    participantType,
		ControlMode:        controlMode,
		Initiative:         initiative,
		Status:             firstNonEmpty(strings.TrimSpace(fmt.Sprintf("%v", character["status"])), "ready"),
		ArmorClass:         parseIntFromAny(character["armor_class"]),
		HitPointMax:        parseIntFromAny(character["hit_point_max"]),
		CurrentHitPoints:   parseIntFromAny(character["current_hit_points"]),
		TemporaryHitPoints: parseIntFromAny(character["temporary_hit_points"]),
		DeathSaveSuccesses: parseIntFromAny(character["death_save_successes"]),
		DeathSaveFailures:  parseIntFromAny(character["death_save_failures"]),
		Stable:             parseBoolFromAny(character["death_save_stable"]),
	}
	if turn.CharacterID == "" && participantType != "dm_companion" {
		turn.CharacterID = turn.ID
	}
	if turn.CurrentHitPoints <= 0 && turn.HitPointMax > 0 && turn.Side != "enemy" {
		turn.CurrentHitPoints = turn.HitPointMax
	}
	return turn
}

func turnCanAct(turn CombatTurnEntry) bool {
	status := strings.ToLower(strings.TrimSpace(turn.Status))
	if turn.CurrentHitPoints <= 0 {
		return false
	}
	return !containsAny(status, "dead", "slain", "killed", "down", "unconscious", "tot", "bewusstlos")
}

func firstAliveEnemyIndex(state CombatState) int {
	for index, turn := range state.InitiativeOrder {
		if turn.Side == "enemy" && turnCanAct(turn) {
			return index
		}
	}
	return -1
}

func hasLivingEnemies(state CombatState) bool {
	return firstAliveEnemyIndex(state) >= 0
}

func finalizeCombatIfResolved(state *CombatState) bool {
	if state == nil {
		return false
	}
	if hasLivingEnemies(*state) {
		return false
	}
	state.Active = false
	state.ActiveTurnIndex = 0
	return true
}

func partyCombatantIndexes(state CombatState) []int {
	indexes := make([]int, 0)
	for index, turn := range state.InitiativeOrder {
		if (turn.Side == "player" || turn.Side == "ally") && turnCanAct(turn) {
			indexes = append(indexes, index)
		}
	}
	return indexes
}

func firstHumanPlayerTurn(state CombatState) *CombatTurnEntry {
	for index := range state.InitiativeOrder {
		turn := &state.InitiativeOrder[index]
		if turn.Side == "player" && turnCanAct(*turn) {
			return turn
		}
	}
	return nil
}

func pendingRollRequestFromSession(session Session) *RollRequest {
	if session.State.VisualMode != "dice_capture" || session.State.VisualPayload == nil {
		return nil
	}
	payload := defaultMetadata(session.State.VisualPayload)
	if strings.TrimSpace(fmt.Sprintf("%v", payload["type"])) != "roll_request" {
		return nil
	}
	request := &RollRequest{
		Type:         strings.TrimSpace(fmt.Sprintf("%v", payload["roll_type"])),
		Label:        strings.TrimSpace(fmt.Sprintf("%v", payload["roll_label"])),
		Ability:      strings.TrimSpace(fmt.Sprintf("%v", payload["roll_ability"])),
		Skill:        strings.TrimSpace(fmt.Sprintf("%v", payload["roll_skill"])),
		Reason:       strings.TrimSpace(fmt.Sprintf("%v", payload["roll_reason"])),
		Instructions: strings.TrimSpace(fmt.Sprintf("%v", payload["instructions"])),
	}
	if dice, ok := payload["roll_dice"].([]any); ok {
		request.Dice = make([]string, 0, len(dice))
		for _, item := range dice {
			token := strings.TrimSpace(fmt.Sprintf("%v", item))
			if token != "" && token != "<nil>" {
				request.Dice = append(request.Dice, token)
			}
		}
	}
	return request
}

func deterministicDamageTypeForActiveTurn(activeTurn *CombatTurnEntry, activeCharacters []map[string]any) string {
	if activeTurn == nil {
		return "damage"
	}
	for _, character := range activeCharacters {
		characterID := strings.TrimSpace(fmt.Sprintf("%v", character["id"]))
		linkedCharacterID := strings.TrimSpace(fmt.Sprintf("%v", character["character_id"]))
		if characterID != activeTurn.ID && linkedCharacterID != activeTurn.CharacterID {
			continue
		}
		for _, field := range []string{"combat_attacks", "spell_attacks"} {
			text := strings.TrimSpace(fmt.Sprintf("%v", character[field]))
			if text == "" || text == "<nil>" {
				continue
			}
			for _, line := range strings.Split(text, "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(strings.ToLower(line), "description:") {
					continue
				}
				parts := strings.Split(line, "|")
				if len(parts) >= 7 {
					damageType := strings.TrimSpace(parts[6])
					if damageType != "" {
						return damageType
					}
				}
			}
		}
	}
	return "damage"
}

func resolveDeterministicPendingDamageTurn(session Session, req GMRespondRequest, activeCharacters []map[string]any) (GMResponse, CombatState, bool) {
	pending := pendingRollRequestFromSession(session)
	if pending == nil || !strings.EqualFold(strings.TrimSpace(pending.Type), "damage") || req.DiceRoll == nil || !session.State.Combat.Active {
		return GMResponse{}, session.State.Combat, false
	}
	nextCombat := session.State.Combat
	activeTurn := activeCombatTurn(nextCombat)
	if activeTurn == nil || activeTurn.Side != "player" {
		return GMResponse{}, session.State.Combat, false
	}
	targetIndex := firstAliveEnemyIndex(nextCombat)
	if targetIndex < 0 {
		return GMResponse{}, session.State.Combat, false
	}
	target := &nextCombat.InitiativeOrder[targetIndex]
	damage := rollTotalFromEvent(req.DiceRoll)
	damageType := deterministicDamageTypeForActiveTurn(activeTurn, activeCharacters)
	entry := CombatLogEntry{
		Timestamp: time.Now().UTC(),
		ActorID:   activeTurn.ID,
		ActorName: activeTurn.Name,
		Side:      "player",
		Kind:      "player_damage",
		Summary:   fmt.Sprintf("%s deals %d %s damage to %s.", activeTurn.Name, damage, damageType, target.Name),
		Details: map[string]any{
			"dice_roll":    req.DiceRoll,
			"damage_total": damage,
			"damage_type":  damageType,
			"target_name":  target.Name,
		},
	}
	applyDamageToTurn(target, damage)
	nextCombat.Log = append(nextCombat.Log, entry)

	defeatedNow := !hasLivingEnemies(nextCombat)
	if defeatedNow {
		finalizeCombatIfResolved(&nextCombat)
		narration := fmt.Sprintf("%s’s strike lands cleanly for %d %s damage and drops %s where it stands. The immediate clash is over. What does the party do next?", activeTurn.Name, damage, damageType, target.Name)
		return GMResponse{
			SessionID:    session.ID,
			Language:     chooseLanguage(req.Language, session.Language),
			Narration:    narration,
			StateUpdates: []StateUpdate{},
			SceneEvents:  []SceneEvent{},
			DMNotes:      []string{"Deterministic combat damage resolution used."},
			CreatedAt:    time.Now().UTC(),
		}, nextCombat, true
	}

	advanceCombatTurn(&nextCombat)
	autoNarration, autoEntries := resolveAutomatedCombatTurns(chooseLanguage(req.Language, session.Language), &nextCombat, activeCharacters)
	if len(autoEntries) > 0 {
		nextCombat.Log = append(nextCombat.Log, autoEntries...)
	}
	finalizeCombatIfResolved(&nextCombat)
	narration := fmt.Sprintf("%s’s strike lands for %d %s damage on %s.", activeTurn.Name, damage, damageType, target.Name)
	if strings.TrimSpace(autoNarration) != "" {
		narration = strings.TrimSpace(strings.Join([]string{narration, autoNarration}, " "))
	}
	narration = ensureInteractiveNarration(chooseLanguage(req.Language, session.Language), narration, nil)
	return GMResponse{
		SessionID:    session.ID,
		Language:     chooseLanguage(req.Language, session.Language),
		Narration:    narration,
		StateUpdates: []StateUpdate{},
		SceneEvents:  []SceneEvent{},
		DMNotes:      []string{"Deterministic combat damage resolution used."},
		CreatedAt:    time.Now().UTC(),
	}, nextCombat, true
}

func looksLikeInitiativePrompt(input string) bool {
	lower := strings.ToLower(strings.TrimSpace(input))
	if lower == "" {
		return false
	}
	for _, marker := range []string{"initiative", "ini", "initiative würfeln", "initiative wuerfeln", "roll initiative"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func shouldAutoStartCombat(session Session, req GMRespondRequest, response GMResponse, activeCharacters []map[string]any) bool {
	if session.State.Combat.Active {
		return false
	}
	enemyNames := detectCombatEnemyNames(session, req.PlayerInput, response, activeCharacters)
	if len(enemyNames) == 0 {
		return false
	}
	if looksLikeInitiativePrompt(req.PlayerInput) {
		return true
	}
	if req.DiceRoll != nil {
		return true
	}
	if response.RollRequest != nil {
		rollType := strings.ToLower(strings.TrimSpace(response.RollRequest.Type))
		if rollType == "attack" || rollType == "damage" || rollType == "save" {
			return true
		}
		if rollType == "check" {
			source := strings.ToLower(strings.Join([]string{
				req.PlayerInput,
				response.Narration,
				response.RollRequest.Label,
				response.RollRequest.Reason,
			}, " "))
			if containsAny(source, "spider", "spinne", "goblin", "monster", "enemy", "feind", "attack", "angriff", "combat", "kampf") {
				return true
			}
		}
	}
	return false
}

func combatPartyLevelBand(activeCharacters []map[string]any) (partySize int, averageLevel int, highestLevel int) {
	ready := combatReadyCharacters(activeCharacters)
	if len(ready) == 0 {
		ready = activeCharacters
	}
	if len(ready) == 0 {
		return 0, 1, 1
	}
	total := 0
	highest := 1
	for _, character := range ready {
		level := parseCharacterLevel(strings.TrimSpace(fmt.Sprintf("%v", character["class_and_level"])))
		total += level
		if level > highest {
			highest = level
		}
	}
	average := total / len(ready)
	if average < 1 {
		average = 1
	}
	return len(ready), average, highest
}

func containsAnySubstring(source string, values []string) bool {
	for _, value := range values {
		if value != "" && strings.Contains(source, value) {
			return true
		}
	}
	return false
}

func selectEncounterVariantIndex(partySize, averageLevel, highestLevel int) int {
	switch {
	case partySize <= 1 && highestLevel <= 1:
		return 0
	case partySize <= 2 && averageLevel <= 1:
		return 0
	case partySize <= 2 && highestLevel <= 2:
		return 1
	default:
		return 2
	}
}

func chooseEncounterCount(explicitPlural bool, partySize, averageLevel, highestLevel int) int {
	if explicitPlural {
		if partySize <= 1 && highestLevel <= 1 {
			return 1
		}
		return 2
	}
	if partySize >= 4 && averageLevel >= 3 {
		return 2
	}
	return 1
}

func balanceEncounterNames(source string, activeCharacters []map[string]any, definition enemyEncounterDefinition, language string) []string {
	partySize, averageLevel, highestLevel := combatPartyLevelBand(activeCharacters)
	explicitPlural := containsAnySubstring(source, definition.PluralKeywords)
	explicitSingle := containsAnySubstring(source, definition.Keywords)
	if !explicitPlural && !explicitSingle {
		return nil
	}
	variantIndex := selectEncounterVariantIndex(partySize, averageLevel, highestLevel)
	if variantIndex >= len(definition.VariantProfiles) {
		variantIndex = len(definition.VariantProfiles) - 1
	}
	count := chooseEncounterCount(explicitPlural, partySize, averageLevel, highestLevel)
	baseName := localizedEncounterVariantName(definition.BaseName, variantIndex, language)
	if baseName == "" {
		baseName = definition.VariantProfiles[variantIndex].Name
	}
	if count <= 1 {
		return []string{baseName}
	}
	items := make([]string, 0, count)
	for i := 1; i <= count; i++ {
		items = append(items, fmt.Sprintf("%s %d", baseName, i))
	}
	return items
}

func detectCombatEnemyNames(session Session, input string, response GMResponse, activeCharacters []map[string]any) []string {
	if len(session.State.Combat.InitiativeOrder) > 0 {
		items := make([]string, 0)
		for _, turn := range session.State.Combat.InitiativeOrder {
			if turn.Side == "enemy" {
				items = append(items, turn.Name)
			}
		}
		if len(items) > 0 {
			return items
		}
	}
	if len(session.State.ActiveNPCs) > 0 {
		return defaultStringSlice(session.State.ActiveNPCs)
	}
	source := strings.ToLower(strings.Join([]string{input, response.Narration}, " "))
	for _, definition := range encounterDefinitions {
		if enemies := balanceEncounterNames(source, activeCharacters, definition, firstNonEmpty(response.Language, session.Language)); len(enemies) > 0 {
			return enemies
		}
	}
	return []string{}
}

func enemyProfileForName(name string, language string) enemyCombatProfile {
	lower := strings.ToLower(strings.TrimSpace(name))
	for _, definition := range encounterDefinitions {
		for _, profile := range definition.VariantProfiles {
			if strings.Contains(lower, strings.ToLower(profile.Name)) {
				profile.Name = localizedMonsterName(profile.Name, language)
				if profile.ArmorClass <= 0 || profile.HitPointMax <= 0 {
					if monster, ok := srdMonsterCatalogEntryByName(strings.TrimSpace(encounterSuffixNumberPattern.ReplaceAllString(profile.Name, ""))); ok {
						profile.ArmorClass = parseLeadingInt(monster.ArmorClass)
						profile.HitPointMax = parseLeadingInt(monster.HitPoints)
					}
				}
				return profile
			}
		}
	}
	canonicalName := strings.TrimSpace(encounterSuffixNumberPattern.ReplaceAllString(name, ""))
	if monster, ok := srdMonsterCatalogEntryByName(canonicalName); ok && monster.AttackBonus > 0 && monster.DamageDice != "" && monster.DamageType != "" {
		return enemyCombatProfile{
			Name:        localizedMonsterName(monster.Name, language),
			AttackBonus: monster.AttackBonus,
			DamageDice:  monster.DamageDice,
			DamageType:  monster.DamageType,
			ArmorClass:  parseLeadingInt(monster.ArmorClass),
			HitPointMax: parseLeadingInt(monster.HitPoints),
		}
	}
	if strings.Contains(lower, "spinne") || strings.Contains(lower, "spider") {
		return enemyCombatProfile{Name: localizedMonsterName("Giant Spider", language), AttackBonus: 5, DamageDice: "1d8+3", DamageType: "piercing", ArmorClass: 14, HitPointMax: 26}
	}
	if strings.Contains(lower, "goblin") {
		return enemyCombatProfile{Name: localizedMonsterName("Goblin Raider", language), AttackBonus: 4, DamageDice: "1d6+2", DamageType: "slashing", ArmorClass: 15, HitPointMax: 7}
	}
	if strings.Contains(lower, "wolf") || strings.Contains(lower, "wölf") || strings.Contains(lower, "woelf") {
		return enemyCombatProfile{Name: localizedMonsterName("Wolf", language), AttackBonus: 4, DamageDice: "1d6+2", DamageType: "piercing", ArmorClass: 13, HitPointMax: 11}
	}
	return enemyCombatProfile{Name: localizedMonsterName("Enemy", language), AttackBonus: 4, DamageDice: "1d6+2", DamageType: "damage", ArmorClass: 12, HitPointMax: 10}
}

var simpleDiceFormulaPattern = regexp.MustCompile(`(?i)^\s*(\d+)\s*[dw]\s*(\d+)(?:\s*([+-])\s*(\d+))?\s*$`)

func rollDiceFormula(formula string) (int, []int) {
	matches := simpleDiceFormulaPattern.FindStringSubmatch(strings.TrimSpace(formula))
	if len(matches) == 0 {
		return 0, nil
	}
	count, _ := strconv.Atoi(matches[1])
	sides, _ := strconv.Atoi(matches[2])
	if count <= 0 || sides <= 0 {
		return 0, nil
	}
	values := make([]int, 0, count)
	total := 0
	for i := 0; i < count; i++ {
		value := rand.Intn(sides) + 1
		values = append(values, value)
		total += value
	}
	if len(matches) >= 5 && matches[4] != "" {
		modifier, _ := strconv.Atoi(matches[4])
		if matches[3] == "-" {
			total -= modifier
		} else {
			total += modifier
		}
	}
	return total, values
}

func combatReadyCharacters(activeCharacters []map[string]any) []map[string]any {
	items := make([]map[string]any, 0, len(activeCharacters))
	for _, character := range activeCharacters {
		status := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", character["status"])))
		if status == "ready" || status == "joined" || status == "active" {
			items = append(items, character)
		}
	}
	if len(items) == 0 && len(activeCharacters) > 0 {
		return append(items, activeCharacters...)
	}
	return items
}

func currentPlayerTurnCharacter(activeCharacters []map[string]any) map[string]any {
	ready := combatReadyCharacters(activeCharacters)
	if len(ready) == 0 {
		return nil
	}
	return ready[0]
}

func activeCombatPlayerCharacter(state CombatState, activeCharacters []map[string]any) map[string]any {
	activeTurn := activeCombatTurn(state)
	if activeTurn == nil || (activeTurn.Side != "player" && activeTurn.Side != "ally") {
		return currentPlayerTurnCharacter(activeCharacters)
	}
	for _, character := range activeCharacters {
		if strings.TrimSpace(fmt.Sprintf("%v", character["id"])) == activeTurn.ID || strings.TrimSpace(fmt.Sprintf("%v", character["character_id"])) == activeTurn.CharacterID {
			return character
		}
	}
	return currentPlayerTurnCharacter(activeCharacters)
}

func initiativeFallbackForCharacter(character map[string]any) int {
	const base = 10
	if abilities, ok := character["abilities"].(map[string]int); ok {
		if dex, exists := abilities["dexterity"]; exists {
			return base + abilityModifierFromAny(dex)
		}
	}
	if abilities, ok := character["abilities"].(map[string]any); ok {
		if dex, exists := abilities["dexterity"]; exists {
			return base + abilityModifierFromAny(dex)
		}
	}
	return base
}

func initializeCombatState(session Session, req GMRespondRequest, response GMResponse, activeCharacters []map[string]any) CombatState {
	players := combatReadyCharacters(activeCharacters)
	enemyNames := detectCombatEnemyNames(session, req.PlayerInput, response, activeCharacters)
	if len(players) == 0 || len(enemyNames) == 0 {
		return session.State.Combat
	}
	rolledPlayerID := strings.TrimSpace(fmt.Sprintf("%v", players[0]["id"]))
	rolledPlayerInit := rollTotalFromEvent(req.DiceRoll)
	order := make([]CombatTurnEntry, 0, len(players)+len(enemyNames))
	for _, player := range players {
		playerID := strings.TrimSpace(fmt.Sprintf("%v", player["id"]))
		playerInit := initiativeFallbackForCharacter(player)
		if playerID == rolledPlayerID && rolledPlayerInit > 0 {
			playerInit = rolledPlayerInit
		}
		turn := combatTurnFromActiveCharacter(player, playerInit)
		turn.ID = playerID
		order = append(order, turn)
	}
	for index, enemyName := range enemyNames {
		profile := enemyProfileForName(enemyName, response.Language)
		enemyInit := rand.Intn(20) + 1
		order = append(order, CombatTurnEntry{
			ID:               fmt.Sprintf("enemy:%d:%s", index+1, strings.ToLower(strings.ReplaceAll(enemyName, " ", "-"))),
			Name:             enemyName,
			Side:             "enemy",
			ParticipantType:  "enemy_npc",
			ControlMode:      "dm",
			Initiative:       enemyInit,
			Status:           "ready",
			ArmorClass:       profile.ArmorClass,
			HitPointMax:      profile.HitPointMax,
			CurrentHitPoints: profile.HitPointMax,
		})
	}
	for i := 0; i < len(order)-1; i++ {
		for j := i + 1; j < len(order); j++ {
			if order[j].Initiative > order[i].Initiative {
				order[i], order[j] = order[j], order[i]
			}
		}
	}
	logEntries := []CombatLogEntry{{
		Timestamp:  time.Now().UTC(),
		ActorID:    rolledPlayerID,
		ActorName:  firstNonEmpty(strings.TrimSpace(fmt.Sprintf("%v", players[0]["name"])), "Spieler"),
		Side:       "system",
		Kind:       "initiative_started",
		Summary:    "Initiative established.",
		Details:    map[string]any{"initiative_order": order},
		PublicText: "Der Kampf beginnt, als alle Beteiligten gleichzeitig reagieren.",
	}}
	return CombatState{
		Active:          true,
		Round:           1,
		ActiveTurnIndex: 0,
		InitiativeOrder: order,
		Log:             logEntries,
	}
}

func advanceCombatTurn(state *CombatState) {
	if state == nil || !state.Active || len(state.InitiativeOrder) == 0 {
		return
	}
	state.ActiveTurnIndex++
	if state.ActiveTurnIndex >= len(state.InitiativeOrder) {
		state.ActiveTurnIndex = 0
		state.Round++
	}
}

func activeCombatTurn(state CombatState) *CombatTurnEntry {
	if !state.Active || len(state.InitiativeOrder) == 0 || state.ActiveTurnIndex < 0 || state.ActiveTurnIndex >= len(state.InitiativeOrder) {
		return nil
	}
	return &state.InitiativeOrder[state.ActiveTurnIndex]
}

type combatActionProfile struct {
	Name        string
	AttackBonus int
	DamageDice  string
	DamageType  string
}

func choosePartyTargetIndex(state CombatState) int {
	candidates := partyCombatantIndexes(state)
	if len(candidates) == 0 {
		return -1
	}
	return candidates[rand.Intn(len(candidates))]
}

func applyDamageToTurn(turn *CombatTurnEntry, damage int) {
	if turn == nil || damage <= 0 {
		return
	}
	if turn.TemporaryHitPoints > 0 {
		absorbed := damage
		if absorbed > turn.TemporaryHitPoints {
			absorbed = turn.TemporaryHitPoints
		}
		turn.TemporaryHitPoints -= absorbed
		damage -= absorbed
	}
	if damage <= 0 {
		return
	}
	turn.CurrentHitPoints -= damage
	if turn.CurrentHitPoints <= 0 {
		turn.CurrentHitPoints = 0
		if turn.Side == "enemy" {
			turn.Status = "dead"
		} else {
			turn.Status = "down"
		}
	}
}

func firstAvailableAttackLine(character map[string]any) string {
	for _, key := range []string{"combat_attacks", "spell_attacks"} {
		text := strings.TrimSpace(fmt.Sprintf("%v", character[key]))
		if text == "" || text == "<nil>" {
			continue
		}
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(strings.ToLower(line), "description:") {
				continue
			}
			if strings.Count(line, "|") >= 5 {
				return line
			}
		}
	}
	return ""
}

func combatActionProfileForCompanion(character map[string]any) combatActionProfile {
	line := firstAvailableAttackLine(character)
	if line == "" {
		return combatActionProfile{Name: "Aid", AttackBonus: 2, DamageDice: "1d4", DamageType: "force"}
	}
	parts := strings.Split(line, "|")
	for index := range parts {
		parts[index] = strings.TrimSpace(parts[index])
	}
	profile := combatActionProfile{Name: parts[0], DamageType: "damage", DamageDice: "1d4"}
	if len(parts) > 4 {
		profile.AttackBonus = parseIntFromAny(strings.TrimPrefix(parts[4], "+"))
	}
	if len(parts) > 5 && strings.TrimSpace(parts[5]) != "" {
		profile.DamageDice = normalizeDiceToken(parts[5])
	}
	if len(parts) > 6 && strings.TrimSpace(parts[6]) != "" {
		profile.DamageType = strings.TrimSpace(parts[6])
	}
	return profile
}

func resolveAutomatedCombatTurns(language string, state *CombatState, activeCharacters []map[string]any) (string, []CombatLogEntry) {
	if state == nil || !state.Active {
		return "", nil
	}
	paragraphs := make([]string, 0)
	entries := make([]CombatLogEntry, 0)
	lookup := map[string]map[string]any{}
	for _, character := range activeCharacters {
		lookup[strings.TrimSpace(fmt.Sprintf("%v", character["id"]))] = character
	}
	for turn := activeCombatTurn(*state); turn != nil && turn.Side != "player"; turn = activeCombatTurn(*state) {
		if !turnCanAct(*turn) {
			advanceCombatTurn(state)
			continue
		}
		if turn.Side == "ally" {
			companion := lookup[turn.ID]
			targetIndex := firstAliveEnemyIndex(*state)
			if companion == nil || targetIndex < 0 {
				advanceCombatTurn(state)
				continue
			}
			target := &state.InitiativeOrder[targetIndex]
			action := combatActionProfileForCompanion(companion)
			baseRoll := rand.Intn(20) + 1
			attackTotal := baseRoll + action.AttackBonus
			hit := attackTotal >= max(target.ArmorClass, 10)
			entry := CombatLogEntry{
				Timestamp: time.Now().UTC(),
				ActorID:   turn.ID,
				ActorName: turn.Name,
				Side:      "ally",
				Kind:      "companion_action",
				Details: map[string]any{
					"roll":         baseRoll,
					"attack_bonus": action.AttackBonus,
					"total":        attackTotal,
					"target_name":  target.Name,
					"target_ac":    target.ArmorClass,
					"action_name":  action.Name,
				},
			}
			if hit {
				damageTotal, damageRolls := rollDiceFormula(action.DamageDice)
				applyDamageToTurn(target, damageTotal)
				entry.Summary = fmt.Sprintf("%s uses %s against %s (%d vs AC %d) for %d %s damage.", turn.Name, action.Name, target.Name, attackTotal, target.ArmorClass, damageTotal, action.DamageType)
				entry.Details["damage_rolls"] = damageRolls
				entry.Details["damage_total"] = damageTotal
				entry.Details["damage_formula"] = action.DamageDice
				if strings.HasPrefix(strings.ToLower(strings.TrimSpace(language)), "de") {
					entry.PublicText = fmt.Sprintf("%s greift im richtigen Moment ein und trifft %s mit %s.", turn.Name, target.Name, action.Name)
					paragraphs = append(paragraphs, fmt.Sprintf("%s bewegt sich im Takt der Gruppe, nutzt die entstandene Lücke und setzt %s mit %s unter Druck.", turn.Name, target.Name, action.Name))
				} else {
					entry.PublicText = fmt.Sprintf("%s steps in at the perfect moment and hits %s with %s.", turn.Name, target.Name, action.Name)
					paragraphs = append(paragraphs, fmt.Sprintf("%s reads the flow of the fight, slips into the opening the party created, and drives %s back with %s.", turn.Name, target.Name, action.Name))
				}
			} else {
				entry.Summary = fmt.Sprintf("%s misses %s with %s (%d vs AC %d).", turn.Name, target.Name, action.Name, attackTotal, target.ArmorClass)
				if strings.HasPrefix(strings.ToLower(strings.TrimSpace(language)), "de") {
					entry.PublicText = fmt.Sprintf("%s setzt %s unter Druck, doch der Schlag geht knapp vorbei.", turn.Name, target.Name)
					paragraphs = append(paragraphs, fmt.Sprintf("%s koordiniert sich sichtbar mit der Gruppe, doch %s entzieht sich im letzten Augenblick dem Angriff.", turn.Name, target.Name))
				} else {
					entry.PublicText = fmt.Sprintf("%s presses %s, but the strike misses by inches.", turn.Name, target.Name)
					paragraphs = append(paragraphs, fmt.Sprintf("%s moves in step with the rest of the party, but %s twists away at the last instant.", turn.Name, target.Name))
				}
			}
			entries = append(entries, entry)
			advanceCombatTurn(state)
			continue
		}
		if turn.Side == "enemy" {
			targetIndex := choosePartyTargetIndex(*state)
			if targetIndex < 0 {
				advanceCombatTurn(state)
				continue
			}
			target := &state.InitiativeOrder[targetIndex]
			profile := enemyProfileForName(turn.Name, language)
			baseRoll := rand.Intn(20) + 1
			attackTotal := baseRoll + profile.AttackBonus
			hit := attackTotal >= max(target.ArmorClass, 10)
			entry := CombatLogEntry{
				Timestamp: time.Now().UTC(),
				ActorID:   turn.ID,
				ActorName: turn.Name,
				Side:      "enemy",
				Kind:      "attack_roll",
				Details: map[string]any{
					"roll":         baseRoll,
					"attack_bonus": profile.AttackBonus,
					"total":        attackTotal,
					"target_ac":    target.ArmorClass,
					"target_name":  target.Name,
				},
			}
			if hit {
				damageTotal, damageRolls := rollDiceFormula(profile.DamageDice)
				applyDamageToTurn(target, damageTotal)
				entry.Summary = fmt.Sprintf("%s hits %s (%d vs AC %d) for %d %s damage.", turn.Name, target.Name, attackTotal, target.ArmorClass, damageTotal, profile.DamageType)
				entry.Details["damage_rolls"] = damageRolls
				entry.Details["damage_total"] = damageTotal
				entry.Details["damage_formula"] = profile.DamageDice
				if strings.HasPrefix(strings.ToLower(strings.TrimSpace(language)), "de") {
					entry.PublicText = fmt.Sprintf("%s bricht durch die Verteidigung von %s und verursacht %d Schaden.", turn.Name, target.Name, damageTotal)
					paragraphs = append(paragraphs, fmt.Sprintf("%s reißt die Initiative an sich, zwingt %s in die Defensive und landet einen harten Treffer.", turn.Name, target.Name))
				} else {
					entry.PublicText = fmt.Sprintf("%s breaks through %s's guard and deals %d damage.", turn.Name, target.Name, damageTotal)
					paragraphs = append(paragraphs, fmt.Sprintf("%s seizes the opening, drives %s back, and lands a punishing blow.", turn.Name, target.Name))
				}
			} else {
				entry.Summary = fmt.Sprintf("%s misses %s (%d vs AC %d).", turn.Name, target.Name, attackTotal, target.ArmorClass)
				if strings.HasPrefix(strings.ToLower(strings.TrimSpace(language)), "de") {
					entry.PublicText = fmt.Sprintf("%s greift %s an, doch der Angriff verfehlt sein Ziel.", turn.Name, target.Name)
					paragraphs = append(paragraphs, fmt.Sprintf("%s fährt aggressiv vor, doch %s weicht im letzten Moment aus und lässt den Angriff ins Leere laufen.", turn.Name, target.Name))
				} else {
					entry.PublicText = fmt.Sprintf("%s attacks %s, but the blow misses.", turn.Name, target.Name)
					paragraphs = append(paragraphs, fmt.Sprintf("%s surges forward, but %s turns aside at the last possible moment and lets the attack glance away.", turn.Name, target.Name))
				}
			}
			entries = append(entries, entry)
			advanceCombatTurn(state)
		}
	}
	if len(paragraphs) == 0 {
		return "", entries
	}
	return strings.Join(paragraphs, " "), entries
}

func buildCombatStateNarration(language string, state CombatState) string {
	if !state.Active || len(state.InitiativeOrder) == 0 {
		return ""
	}
	orderParts := make([]string, 0, len(state.InitiativeOrder))
	for _, turn := range state.InitiativeOrder {
		role := turn.Side
		switch turn.Side {
		case "ally":
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(language)), "de") {
				role = "Verbündeter"
			} else {
				role = "Ally"
			}
		case "player":
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(language)), "de") {
				role = "Spieler"
			} else {
				role = "Player"
			}
		case "enemy":
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(language)), "de") {
				role = "Gegner"
			} else {
				role = "Enemy"
			}
		}
		orderParts = append(orderParts, fmt.Sprintf("%s (%s %d)", turn.Name, role, turn.Initiative))
	}
	if current := activeCombatTurn(state); current != nil {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(language)), "de") {
			return fmt.Sprintf("Initiative steht. Runde %d. Reihenfolge: %s. Zuerst handelt %s.", state.Round, strings.Join(orderParts, ", "), current.Name)
		}
		return fmt.Sprintf("Initiative is set. Round %d. Order: %s. %s acts first.", state.Round, strings.Join(orderParts, ", "), current.Name)
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(language)), "de") {
		return fmt.Sprintf("Initiative steht. Runde %d. Reihenfolge: %s.", state.Round, strings.Join(orderParts, ", "))
	}
	return fmt.Sprintf("Initiative is set. Round %d. Order: %s.", state.Round, strings.Join(orderParts, ", "))
}

func buildScenePromptContext(session Session, req GMRespondRequest, workingSummary map[string]any, contextChunks []GMContextChunk, adventureAssets []Asset) map[string]any {
	adventureChunks := filterAdventurePromptChunks(contextChunks)
	return map[string]any{
		"scene_context": map[string]any{
			"current_scene":    truncatePromptContextText(session.CurrentScene, 220),
			"current_location": truncatePromptContextText(session.CurrentLocation, 120),
			"scene_summary":    truncatePromptContextText(session.State.SceneSummary, 260),
			"scene_mode":       inferSceneMode(session, req),
		},
		"session_facts":     deriveSessionFacts(session, workingSummary, adventureChunks),
		"known_npcs":        deriveKnownNPCs(session, req.PlayerInput, adventureChunks, adventureAssets),
		"adventure_context": deriveAdventureContext(req.PlayerInput, adventureChunks, adventureAssets),
	}
}

func adventureAssetsForSession(ctx context.Context, store *Store, session Session) ([]Asset, error) {
	if session.AdventureID == nil || strings.TrimSpace(*session.AdventureID) == "" {
		return []Asset{}, nil
	}
	allAssets, err := store.ListAssets(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]Asset, 0)
	for _, asset := range allAssets {
		if asset.AdventureID != nil && *asset.AdventureID == *session.AdventureID {
			items = append(items, asset)
		}
	}
	return items, nil
}

var damageDicePattern = regexp.MustCompile(`(?i)\b(\d+\s*[wd]\s*\d+(?:\s*[+-]\s*\d+)?)\b`)
var storySummaryDicePattern = regexp.MustCompile(`(?i)\b\d+\s*[wd]\s*\d+(?:\s*[+-]\s*\d+)?\b`)
var storySummaryNumberPattern = regexp.MustCompile(`\b\d+\b`)
var storySummaryMarkdownPattern = regexp.MustCompile(`[*_` + "`" + `#>]+`)

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

func buildSessionStorySummary(session Session, nextState SessionState, previous string, narration string, language string) string {
	sentences := make([]string, 0, 8)
	seen := map[string]struct{}{}
	appendSentence := func(text string) {
		clean := sanitizeStorySummarySentence(text, language)
		if clean == "" {
			return
		}
		key := strings.ToLower(clean)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		sentences = append(sentences, clean)
	}

	appendSentence(storySummaryContextSentence(session, nextState, language))
	for _, sentence := range firstNSentences(previous, 12) {
		appendSentence(sentence)
	}
	for _, sentence := range firstNSentences(narration, 6) {
		appendSentence(sentence)
	}

	if len(sentences) > 6 {
		sentences = sentences[len(sentences)-6:]
	}
	return strings.TrimSpace(strings.Join(sentences, " "))
}

func storySummaryContextSentence(session Session, nextState SessionState, language string) string {
	parts := make([]string, 0, 3)
	if scene := strings.TrimSpace(firstNonEmpty(session.CurrentScene, nextState.SceneSummary)); scene != "" {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(language)), "de") {
			parts = append(parts, fmt.Sprintf("Die Geschichte befindet sich aktuell bei %s", scene))
		} else {
			parts = append(parts, fmt.Sprintf("The story is currently at %s", scene))
		}
	}
	if location := strings.TrimSpace(session.CurrentLocation); location != "" {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(language)), "de") {
			parts = append(parts, fmt.Sprintf("am Ort %s", location))
		} else {
			parts = append(parts, fmt.Sprintf("in %s", location))
		}
	}
	npcs := compactStringList(defaultStringSlice(nextState.ActiveNPCs), 3, 60)
	if len(npcs) > 0 {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(language)), "de") {
			parts = append(parts, fmt.Sprintf("wobei %s eine wichtige Rolle spielen", joinGermanList(npcs)))
		} else {
			parts = append(parts, fmt.Sprintf("with %s currently in focus", joinLocalizedList(npcs, "en")))
		}
	}
	return strings.Join(parts, ", ")
}

func sanitizeStorySummarySentence(text string, language string) string {
	clean := strings.TrimSpace(text)
	if clean == "" {
		return ""
	}
	clean = storySummaryMarkdownPattern.ReplaceAllString(clean, "")
	clean = storySummaryDicePattern.ReplaceAllString(clean, "")
	clean = storySummaryNumberPattern.ReplaceAllString(clean, "")
	clean = strings.Join(strings.Fields(clean), " ")
	if skipStorySummarySentence(clean, language) {
		return ""
	}
	clean = strings.TrimSpace(strings.Trim(clean, " ,;:"))
	if clean == "" {
		return ""
	}
	last := clean[len(clean)-1]
	if last != '.' && last != '!' {
		clean += "."
	}
	return clean
}

func skipStorySummarySentence(text string, language string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return true
	}
	if strings.Contains(lower, "?") {
		return true
	}
	badFragments := []string{
		"what do you do", "what happens next", "reply with", "roll ", "give me the total", "confirm roll",
		"was tut ihr", "was tust du", "wie reagierst du", "antworte mit", "würfle", "wurf", "bestätige",
		"armor class", "spell save dc", "spell attack", "initiative is set", "order:",
		"rüstungsklasse", "zauber-sg", "zauberangriff", "initiative steht", "reihenfolge:",
		"trefferpunkte", "hit points", "xp", "ep", "dc ", " ac ", " hp ",
	}
	for _, fragment := range badFragments {
		if strings.Contains(lower, fragment) {
			return true
		}
	}
	return false
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
	if session.State.Combat.Active && len(session.State.Combat.InitiativeOrder) > 0 {
		orderParts := make([]string, 0, len(session.State.Combat.InitiativeOrder))
		for _, turn := range session.State.Combat.InitiativeOrder {
			orderParts = append(orderParts, fmt.Sprintf("%s (%s, Ini %d)", turn.Name, turn.Side, turn.Initiative))
		}
		appendFact(fmt.Sprintf("Kampf aktiv. Runde %d. Initiative: %s.", session.State.Combat.Round, strings.Join(orderParts, " -> ")))
		if activeTurn := activeCombatTurn(session.State.Combat); activeTurn != nil {
			appendFact(fmt.Sprintf("Gerade am Zug: %s.", activeTurn.Name))
		}
	}
	if strings.TrimSpace(session.State.LastRewardSummary) != "" {
		appendFact(fmt.Sprintf("Letzte Belohnung: %s.", session.State.LastRewardSummary))
	}
	if session.State.AwaitingLevelUpRest {
		appendFact("Mindestens ein Charakter ist stufenaufstiegsbereit, aber der Aufstieg soll erst bei einer Rast beginnen.")
	}
	if len(session.State.LevelUpQueue) > 0 {
		names := make([]string, 0, len(session.State.LevelUpQueue))
		for _, item := range session.State.LevelUpQueue {
			names = append(names, fmt.Sprintf("%s (%d -> %d)", item.CharacterName, item.CurrentLevel, item.NextLevel))
		}
		appendFact(fmt.Sprintf("Stufenaufstiegs-Reihenfolge vorbereitet: %s.", strings.Join(names, ", ")))
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

func deriveKnownNPCs(session Session, query string, adventureChunks []GMContextChunk, adventureAssets []Asset) []map[string]any {
	items := make([]map[string]any, 0, 3)
	names := compactStringList(defaultStringSlice(session.State.ActiveNPCs), 3, 80)
	names = appendUniqueStrings(names, matchingAdventureEntityNames(query, adventureAssets, 4)...)
	for _, name := range compactStringList(names, 5, 80) {
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

func deriveAdventureContext(query string, adventureChunks []GMContextChunk, adventureAssets []Asset) []map[string]any {
	items := make([]map[string]any, 0, 6)
	for _, name := range matchingAdventureEntityNames(query, adventureAssets, 4) {
		matches := collectSentencesContaining(adventureChunks, name, 2)
		if len(matches) == 0 {
			continue
		}
		items = append(items, map[string]any{
			"source":  "adventure_entity",
			"summary": truncatePromptContextText(fmt.Sprintf("%s: %s", name, strings.Join(matches, " ")), 220),
		})
		if len(items) >= 3 {
			break
		}
	}
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
			continue
		}
	}
	for _, summary := range matchingAdventureAssetSummaries(query, adventureAssets, 3) {
		items = append(items, map[string]any{
			"source":  "adventure_asset",
			"summary": truncatePromptContextText(summary, 220),
		})
		if len(items) >= 6 {
			break
		}
	}
	return items
}

func matchingAdventureEntityNames(query string, adventureAssets []Asset, limit int) []string {
	lowerQuery := expandedAdventureQuery(strings.ToLower(strings.TrimSpace(query)))
	if lowerQuery == "" || limit <= 0 {
		return nil
	}
	names := make([]string, 0, limit)
	seen := map[string]struct{}{}
	for _, asset := range adventureAssets {
		if asset.EntityName == nil {
			continue
		}
		name := strings.TrimSpace(*asset.EntityName)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			continue
		}
		if strings.Contains(lowerQuery, key) || strings.Contains(key, lowerQuery) || queryTokenOverlap(lowerQuery, key) {
			seen[key] = struct{}{}
			names = append(names, name)
			if len(names) >= limit {
				break
			}
		}
	}
	return names
}

func matchingAdventureAssetSummaries(query string, adventureAssets []Asset, limit int) []string {
	lowerQuery := expandedAdventureQuery(strings.ToLower(strings.TrimSpace(query)))
	if lowerQuery == "" || limit <= 0 {
		return nil
	}
	items := make([]string, 0, limit)
	seen := map[string]struct{}{}
	for _, asset := range adventureAssets {
		candidates := []string{asset.Name, asset.Type}
		if asset.EntityName != nil {
			candidates = append(candidates, *asset.EntityName)
		}
		if asset.LocationName != nil {
			candidates = append(candidates, *asset.LocationName)
		}
		matched := false
		for _, candidate := range candidates {
			candidate = strings.TrimSpace(candidate)
			if candidate == "" {
				continue
			}
			lowerCandidate := strings.ToLower(candidate)
			if strings.Contains(lowerQuery, lowerCandidate) || strings.Contains(lowerCandidate, lowerQuery) || queryTokenOverlap(lowerQuery, lowerCandidate) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		summary := describeAdventureAsset(asset)
		key := strings.ToLower(summary)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		items = append(items, summary)
		if len(items) >= limit {
			break
		}
	}
	return items
}

func describeAdventureAsset(asset Asset) string {
	parts := []string{fmt.Sprintf("Asset %s", asset.Name)}
	if asset.Type != "" {
		parts = append(parts, fmt.Sprintf("type %s", asset.Type))
	}
	if asset.EntityName != nil && strings.TrimSpace(*asset.EntityName) != "" {
		parts = append(parts, fmt.Sprintf("entity %s", strings.TrimSpace(*asset.EntityName)))
	}
	if asset.LocationName != nil && strings.TrimSpace(*asset.LocationName) != "" {
		parts = append(parts, fmt.Sprintf("location %s", strings.TrimSpace(*asset.LocationName)))
	}
	if len(asset.Tags) > 0 {
		parts = append(parts, fmt.Sprintf("tags %s", strings.Join(asset.Tags, ", ")))
	}
	return strings.Join(parts, "; ")
}

func queryTokenOverlap(a, b string) bool {
	for _, token := range strings.FieldsFunc(a, func(r rune) bool {
		return r == ' ' || r == '_' || r == '-' || r == ',' || r == '.' || r == ':' || r == ';'
	}) {
		token = strings.TrimSpace(token)
		if len(token) < 4 {
			continue
		}
		if strings.Contains(b, token) {
			return true
		}
	}
	return false
}

func expandedAdventureQuery(input string) string {
	if strings.TrimSpace(input) == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"krypta", "krypta crypt",
		"abtei", "abtei abbey",
		"dach", "dach roof",
		"glockenturm", "glockenturm belltower",
		"erster stock", "erster stock first floor",
		"stock", "stock floor",
		"bruder", "bruder brother",
		"hexe", "hexe hag witch",
		"karte", "karte map battlemap",
		"brief", "brief letter handout",
	)
	return replacer.Replace(input)
}

func appendUniqueStrings(existing []string, extra ...string) []string {
	seen := make(map[string]struct{}, len(existing))
	for _, item := range existing {
		seen[strings.ToLower(strings.TrimSpace(item))] = struct{}{}
	}
	for _, item := range extra {
		key := strings.ToLower(strings.TrimSpace(item))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		existing = append(existing, item)
	}
	return existing
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
	adventureAssets := make([]Asset, 0)
	if adventure != nil {
		adventureAssets, err = adventureAssetsForSession(c.Request.Context(), h.store, session)
		if err != nil {
			errorResponse(c, http.StatusInternalServerError, "load adventure assets", err)
			return
		}
	}
	characterContextChunks := activeCharacterContextChunks(activeCharacters)
	contextChunks := make([]GMContextChunk, 0)
	if isRulesQuery {
		shortRulebookDocuments := make([]Document, 0, len(rulebookDocuments))
		regularRulebookDocuments := make([]Document, 0, len(rulebookDocuments))
		for _, document := range rulebookDocuments {
			if strings.EqualFold(strings.TrimSpace(fmt.Sprintf("%v", document.Metadata["kind"])), "short_rules_guide") {
				shortRulebookDocuments = append(shortRulebookDocuments, document)
				continue
			}
			regularRulebookDocuments = append(regularRulebookDocuments, document)
		}
		if len(shortRulebookDocuments) > 0 {
			contextChunks, err = h.retrieveRelevantContextForDocuments(c.Request.Context(), shortRulebookDocuments, req.PlayerInput, contextLimit, true)
			if err != nil {
				errorResponse(c, http.StatusInternalServerError, "retrieve short rules chunks", err)
				return
			}
		}
		if len(contextChunks) == 0 {
			if len(regularRulebookDocuments) == 0 {
				regularRulebookDocuments = append(regularRulebookDocuments, shortRulebookDocuments...)
			}
			contextChunks, err = h.retrieveRelevantContextForDocuments(c.Request.Context(), regularRulebookDocuments, req.PlayerInput, contextLimit, true)
			if err != nil {
				errorResponse(c, http.StatusInternalServerError, "retrieve rulebook chunks", err)
				return
			}
		}
	} else {
		contextChunks, err = h.retrieveRelevantContextForDocuments(c.Request.Context(), adventureDocuments, req.PlayerInput, contextLimit, false)
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
	scenePromptContext := buildScenePromptContext(session, req, summaryLLMSession.WorkingSummary, contextChunks, adventureAssets)
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

	// --- server-side validation of model output (P0.6) ---
	// Build a set of known character IDs from the active characters list.
	knownIDs := make(map[string]struct{})
	for _, ch := range activeCharacters {
		if id, ok := ch["id"].(string); ok {
			knownIDs[id] = struct{}{}
		}
	}

	// Validate state_updates against the allowlist.
	validatedUpdates, stateErrors := validateStateUpdates(response.StateUpdates, knownIDs)
	if len(stateErrors) > 0 {
		for _, se := range stateErrors {
			log.Printf("httpapi state_update rejected: %s", redactSensitiveText(se.Error()))
		}
		response.DMNotes = append(response.DMNotes, fmt.Sprintf("state_updates rejected: %d invalid update(s) filtered", len(stateErrors)))
	}
	response.StateUpdates = validatedUpdates

	// Validate roll_request.
	if response.RollRequest != nil {
		cleanedRoll, rollErrors := ValidateRollRequest(response.RollRequest)
		if len(rollErrors) > 0 {
			for _, re := range rollErrors {
				log.Printf("httpapi roll_request rejected: %s", redactSensitiveText(re))
			}
			response.DMNotes = append(response.DMNotes, fmt.Sprintf("roll_request invalid: %d error(s) filtered", len(rollErrors)))
			response.RollRequest = nil
		} else {
			response.RollRequest = cleanedRoll
		}
	}

	response.RollRequest = ensureAttackFollowUpRollRequest(req.PlayerInput, activeCharacters, response.RollRequest)
	response.RollRequest = sanitizeInvalidCombatRollRequest(response.RollRequest)
	response.RollRequest = sanitizeRollRequestForDisplay(response.RollRequest)
	if response.RollRequest != nil && req.DiceRoll == nil {
		response.Narration = buildRollRequestNarration(response.Language, response.RollRequest)
		if strings.TrimSpace(response.Narration) == "" {
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(response.Language)), "de") {
				response.Narration = "Bevor ich die Szene auflöse, brauche ich erst deinen Wurf."
			} else {
				response.Narration = "Before I resolve the scene, I need your roll."
			}
		}
	}
	response.Narration = ensureInteractiveNarration(response.Language, response.Narration, response.RollRequest)

	nextCombat := session.State.Combat
	combatJustStarted := false
	deterministicCombatResolved := false
	if deterministicResponse, deterministicCombat, ok := resolveDeterministicPendingDamageTurn(session, req, activeCharacters); ok {
		response = deterministicResponse
		nextCombat = deterministicCombat
		deterministicCombatResolved = true
	}
	if shouldAutoStartCombat(session, req, response, activeCharacters) && !nextCombat.Active {
		nextCombat = initializeCombatState(session, req, response, activeCharacters)
		if nextCombat.Active {
			combatJustStarted = true
			session.State.ActiveNPCs = detectCombatEnemyNames(session, req.PlayerInput, response, activeCharacters)
			response.DMNotes = append(response.DMNotes, fmt.Sprintf("Combat started. Round %d.", nextCombat.Round))
		}
	}
	if nextCombat.Active {
		if deterministicCombatResolved {
			goto afterGenericCombatFlow
		}
		if req.DiceRoll != nil {
			if activeTurn := activeCombatTurn(nextCombat); activeTurn != nil && activeTurn.Side == "player" {
				playerName := firstNonEmpty(strings.TrimSpace(activeTurn.Name), "Spieler")
				nextCombat.Log = append(nextCombat.Log, CombatLogEntry{
					Timestamp:  time.Now().UTC(),
					ActorID:    activeTurn.ID,
					ActorName:  playerName,
					Side:       "player",
					Kind:       "player_roll",
					Summary:    fmt.Sprintf("%s würfelt %d.", playerName, rollTotalFromEvent(req.DiceRoll)),
					Details:    map[string]any{"dice_roll": req.DiceRoll, "player_input": req.PlayerInput},
					PublicText: fmt.Sprintf("%s handelt entschlossen, während der Ausgang des Manövers einen Atemzug lang in der Luft hängt.", playerName),
				})
			}
		}
		if response.RollRequest == nil {
			preResolutionNarration := ""
			skipAutomatedResolution := false
			if combatJustStarted {
				preResolutionNarration = buildCombatStateNarration(response.Language, nextCombat)
				if activeTurn := activeCombatTurn(nextCombat); activeTurn != nil && activeTurn.Side == "player" {
					response.Narration = ensureInteractiveNarration(response.Language, preResolutionNarration, nil)
					skipAutomatedResolution = true
				}
			}
			if !skipAutomatedResolution {
				if activeTurn := activeCombatTurn(nextCombat); activeTurn != nil && activeTurn.Side == "player" {
					advanceCombatTurn(&nextCombat)
				}
				autoNarration, autoLogEntries := resolveAutomatedCombatTurns(response.Language, &nextCombat, activeCharacters)
				if len(autoLogEntries) > 0 {
					nextCombat.Log = append(nextCombat.Log, autoLogEntries...)
				}
				if combatJustStarted {
					response.Narration = strings.TrimSpace(strings.Join([]string{preResolutionNarration, autoNarration}, " "))
					response.Narration = ensureInteractiveNarration(response.Language, response.Narration, nil)
				} else if strings.TrimSpace(autoNarration) != "" {
					response.Narration = strings.TrimSpace(strings.Join([]string{strings.TrimSpace(response.Narration), autoNarration}, " "))
					response.Narration = ensureInteractiveNarration(response.Language, response.Narration, nil)
				}
			}
		}
	}
afterGenericCombatFlow:
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
	nextState.Combat = nextCombat
	if len(session.State.ActiveNPCs) > 0 {
		nextState.ActiveNPCs = session.State.ActiveNPCs
	}
	nextState.LastNarration = response.Narration
	nextState.LastDMNotes = response.DMNotes
	nextState.SceneSummary = firstNonEmpty(response.Narration, session.State.SceneSummary)
	if req.DiceRoll != nil && response.RollRequest == nil {
		nextState.VisualMode = "scene"
		nextState.VisualPayload = map[string]any{
			"narration": response.Narration,
		}
	}
	if response.RollRequest != nil {
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
	}
	if req.DiceRoll != nil {
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
	nextState.SessionRecap = buildSessionStorySummary(session, nextState, nextState.SessionRecap, response.Narration, response.Language)
	if nextState.Combat.Active && response.RollRequest == nil && (detectsCombatResolution(req.PlayerInput) || detectsRestTransition(req.PlayerInput)) {
		nextState.Combat.Log = append(nextState.Combat.Log, CombatLogEntry{
			Timestamp: time.Now().UTC(),
			ActorID:   "system",
			ActorName: "System",
			Side:      "system",
			Kind:      "combat_ended",
			Summary:   "Combat resolved.",
			PublicText: func() string {
				if strings.HasPrefix(strings.ToLower(strings.TrimSpace(response.Language)), "de") {
					return "Der unmittelbare Kampf ist vorerst beendet."
				}
				return "The immediate combat has ended for now."
			}(),
		})
		nextState.Combat.Active = false
		nextState.Combat.ActiveTurnIndex = 0
	}

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
	if summary := buildRewardSummary(response.StateUpdates, response.Language); strings.TrimSpace(summary) != "" {
		nextState.LastRewardSummary = summary
		response.DMNotes = append(response.DMNotes, summary)
	}
	activeSessionCharacters, err := loadActiveSessionCharacters(c.Request.Context(), h.store, session)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "load active session characters", err)
		return
	}
	eligibleLevelUps := buildLevelUpQueue(activeSessionCharacters, response.Language)
	if len(eligibleLevelUps) == 0 {
		nextState.AwaitingLevelUpRest = false
		nextState.LevelUpQueue = []LevelUpQueueEntry{}
	} else if detectsRestTransition(req.PlayerInput) {
		nextState.AwaitingLevelUpRest = false
		nextState.LevelUpQueue = eligibleLevelUps
		names := make([]string, 0, len(eligibleLevelUps))
		for _, item := range eligibleLevelUps {
			names = append(names, item.CharacterName)
		}
		firstName := eligibleLevelUps[0].CharacterName
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(response.Language)), "de") {
			response.Narration = strings.TrimSpace(response.Narration + " Während dieser Rast sind folgende Stufenaufstiege bereit: " + strings.Join(names, ", ") + ". Soll ich mit " + firstName + " beginnen?")
			response.DMNotes = append(response.DMNotes, "Level-up queue prepared after rest.")
		} else {
			response.Narration = strings.TrimSpace(response.Narration + " During this rest, the following level-ups are ready: " + strings.Join(names, ", ") + ". Shall I begin with " + firstName + "?")
			response.DMNotes = append(response.DMNotes, "Level-up queue prepared after rest.")
		}
		response.Narration = ensureInteractiveNarration(response.Language, response.Narration, nil)
	} else {
		nextState.AwaitingLevelUpRest = true
		if len(nextState.LevelUpQueue) > 0 {
			nextState.LevelUpQueue = eligibleLevelUps
		}
	}
	nextState.LastNarration = response.Narration
	nextState.LastDMNotes = response.DMNotes
	nextState.SceneSummary = firstNonEmpty(response.Narration, nextState.SceneSummary)
	nextState.SessionRecap = buildSessionStorySummary(session, nextState, nextState.SessionRecap, response.Narration, response.Language)
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
		"last_reward_summary":     nextState.LastRewardSummary,
		"awaiting_level_up_rest":  nextState.AwaitingLevelUpRest,
		"level_up_queue":          nextState.LevelUpQueue,
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
	adventureAssets, err := adventureAssetsForSession(ctx, h.store, session)
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
	}, contextChunks, adventureAssets)
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
	adventureAssets, err := adventureAssetsForSession(ctx, h.store, session)
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
	}, contextChunks, adventureAssets)
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
	narration := "For a moment the scene falters, but the situation remains tense."
	dmNote := "LLM fallback active"
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(language)), "de") {
		narration = "Einen Moment lang stockt die Szene, doch die Situation bleibt angespannt."
		dmNote = "LLM-Fallback aktiv"
		if req.DiceRoll != nil && len(req.DiceRoll.Dice) > 0 {
			narration = "Der Wurf fällt, und für einen Herzschlag hält die Szene den Atem an."
		}
		if req.DiceRoll == nil {
			narration += " Deine Aktion steht im Raum, und die Welt reagiert noch nicht sauber darauf. Beschreibe den Schritt noch einmal kurz oder präzisiere dein Ziel."
		}
	} else {
		if req.DiceRoll != nil && len(req.DiceRoll.Dice) > 0 {
			narration = "The dice fall, and for a heartbeat the scene holds its breath."
		}
		if req.DiceRoll == nil {
			narration += " Your action hangs in the air, but the world has not responded clearly. Briefly describe the action again or clarify your goal."
		}
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
		DMNotes:      []string{dmNote, llmErr.Error()},
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
		"story_summary":         nextState.SessionRecap,
		"session_recap":         nextState.SessionRecap,
		"recent_summary":        nextState.SessionRecap,
		"last_outcome":          nextState.LastNarration,
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
