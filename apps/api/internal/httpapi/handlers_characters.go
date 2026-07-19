package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

func (h *Handler) listCharacters(c *gin.Context) {
	items, err := h.store.ListCharacters(c.Request.Context())
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "list characters", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) createCharacter(c *gin.Context) {
	var req CreateCharacterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid character payload", err)
		return
	}

	character := Character{
		CampaignID:    req.CampaignID,
		Name:          req.Name,
		PlayerName:    req.PlayerName,
		ClassAndLevel: req.ClassAndLevel,
		Background:    req.Background,
		Race:          req.Race,
		Alignment:     req.Alignment,
		ArmorClass:    req.ArmorClass,
		Speed:         req.Speed,
		HitPointMax:   req.HitPointMax,
		Proficiency:   req.Proficiency,
		Abilities:     req.Abilities,
		Languages:     req.Languages,
		Features:      req.Features,
		Metadata:      req.Metadata,
	}

	created, err := h.store.CreateCharacter(c.Request.Context(), character)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "create character", err)
		return
	}

	c.JSON(http.StatusCreated, created)
}

func (h *Handler) updateCharacter(c *gin.Context) {
	var req CreateCharacterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid character payload", err)
		return
	}

	existing, err := h.store.GetCharacter(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "character not found"})
			return
		}
		errorResponse(c, http.StatusInternalServerError, "load character", err)
		return
	}

	existing.CampaignID = req.CampaignID
	existing.Name = req.Name
	existing.PlayerName = req.PlayerName
	existing.ClassAndLevel = req.ClassAndLevel
	existing.Background = req.Background
	existing.Race = req.Race
	existing.Alignment = req.Alignment
	existing.ArmorClass = req.ArmorClass
	existing.Speed = req.Speed
	existing.HitPointMax = req.HitPointMax
	existing.Proficiency = req.Proficiency
	existing.Abilities = req.Abilities
	existing.Languages = req.Languages
	existing.Features = req.Features
	existing.Metadata = req.Metadata

	updated, err := h.store.UpdateCharacter(c.Request.Context(), existing)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "update character", err)
		return
	}

	c.JSON(http.StatusOK, updated)
}

func (h *Handler) deleteCharacter(c *gin.Context) {
	character, err := h.store.GetCharacter(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "character not found"})
			return
		}
		errorResponse(c, http.StatusInternalServerError, "load character", err)
		return
	}

	if err := h.store.DeleteCharacter(c.Request.Context(), character.ID); err != nil {
		errorResponse(c, http.StatusInternalServerError, "delete character", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"deleted": true, "id": character.ID})
}

func (h *Handler) resolveAbilityScores(c *gin.Context) {
	var req ResolveAbilityScoresRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid ability score payload", err)
		return
	}

	values, rolledBreakdown, ruleSummary, err := resolveAbilityValues(req)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "resolve ability scores", err)
		return
	}

	className := strings.ToLower(strings.TrimSpace(req.Class))
	assignment := assignAbilityValues(values, className)
	response := ResolveAbilityScoresResponse{
		Method:            req.Method,
		Values:            values,
		Assignment:        assignment,
		RuleSummary:       ruleSummary,
		RecommendedReason: recommendedReasonForClass(className),
		RolledBreakdown:   rolledBreakdown,
		NeedsConfirmation: true,
	}
	c.JSON(http.StatusOK, response)
}

func (h *Handler) validateAbilityAssignment(c *gin.Context) {
	var req ValidateAbilityAssignmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid ability assignment payload", err)
		return
	}

	response := validateAbilityAssignment(req)
	c.JSON(http.StatusOK, response)
}

func (h *Handler) importCharacterSheet(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "missing uploaded character sheet", err)
		return
	}
	if err := ensureAllowedExtension(fileHeader.Filename, allowedCharacterSheetExtensions); err != nil {
		errorResponse(c, uploadErrorStatus(err), "invalid character sheet type", err)
		return
	}
	if err := ensureUploadSize(fileHeader, h.cfg.MaxUploadBytes); err != nil {
		errorResponse(c, uploadErrorStatus(err), "character sheet exceeds allowed size", err)
		return
	}

	name := c.PostForm("name")
	if name == "" {
		name = strings.TrimSuffix(fileHeader.Filename, filepath.Ext(fileHeader.Filename))
	}

	metadata := map[string]any{
		"source_type": "character_sheet",
	}
	campaignID := strings.TrimSpace(c.PostForm("campaign_id"))
	if campaignID != "" {
		metadata["campaign_id"] = campaignID
	}

	document, err := h.persistUploadedDocument(c, fileHeader, CreateDocumentRequest{
		Type:     "character_sheet",
		Name:     name,
		Metadata: metadata,
	})
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "store character sheet", err)
		return
	}

	if document.SourceFilePath == nil {
		errorResponse(c, http.StatusInternalServerError, "character sheet path missing", errors.New("missing source file path"))
		return
	}

	text, err := extractDocumentText(*document.SourceFilePath)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "extract character sheet text", err)
		return
	}

	character := extractCharacterFromText(text)
	if character.Name == "" {
		character.Name = name
	}
	character.DocumentID = &document.ID
	character.CampaignID = stringPtrOrNil(campaignID)
	character.Metadata["source_document_name"] = document.Name

	createdCharacter, err := h.store.CreateCharacter(c.Request.Context(), character)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "create character", err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"document":  document,
		"character": createdCharacter,
	})
}

func parseMetadataInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		var parsed int
		_, _ = fmt.Sscanf(strings.TrimSpace(typed), "%d", &parsed)
		return parsed
	default:
		return 0
	}
}

func normalizeProgressField(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	return normalized
}

func mergeUniqueStrings(current []string, additions ...string) []string {
	seen := make(map[string]bool, len(current))
	items := make([]string, 0, len(current)+len(additions))
	for _, entry := range current {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		items = append(items, trimmed)
	}
	for _, entry := range additions {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		items = append(items, trimmed)
	}
	return items
}

func removeStringValues(current []string, removals ...string) []string {
	blocked := make(map[string]bool, len(removals))
	for _, entry := range removals {
		trimmed := strings.TrimSpace(entry)
		if trimmed != "" {
			blocked[trimmed] = true
		}
	}
	items := make([]string, 0, len(current))
	for _, entry := range current {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" || blocked[trimmed] {
			continue
		}
		items = append(items, trimmed)
	}
	return items
}

func splitProgressItems(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n' || r == ';'
	})
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}

func metadataStringList(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		items := make([]string, 0, len(typed))
		for _, entry := range typed {
			trimmed := strings.TrimSpace(fmt.Sprint(entry))
			if trimmed != "" && trimmed != "<nil>" {
				items = append(items, trimmed)
			}
		}
		return items
	case string:
		return splitProgressItems(typed)
	default:
		return nil
	}
}

func resolveAbilityValues(req ResolveAbilityScoresRequest) ([]int, []int, string, error) {
	switch req.Method {
	case "standard":
		values := []int{15, 14, 13, 12, 10, 8}
		return values, append([]int(nil), values...), "Standard Array: 15, 14, 13, 12, 10, 8", nil
	case "point_buy":
		if len(req.PointBuy) != 6 {
			return nil, nil, "", fmt.Errorf("point_buy requires exactly 6 values")
		}
		values := append([]int(nil), req.PointBuy...)
		sort.Sort(sort.Reverse(sort.IntSlice(values)))
		return values, append([]int(nil), values...), "Point Buy values supplied by client", nil
	case "rolled":
		if len(req.RolledSets) != 6 && len(req.RolledSets) != 7 {
			return nil, nil, "", fmt.Errorf("rolled requires either 6 or 7 roll sets")
		}
		values := make([]int, 0, len(req.RolledSets))
		breakdown := make([]int, 0, len(req.RolledSets))
		for _, set := range req.RolledSets {
			if len(set) != 4 {
				return nil, nil, "", fmt.Errorf("each rolled set must contain 4 dice")
			}
			rolls := append([]int(nil), set...)
			for _, value := range rolls {
				if value < 1 || value > 6 {
					return nil, nil, "", fmt.Errorf("rolled dice must be between 1 and 6")
				}
			}
			sort.Sort(sort.Reverse(sort.IntSlice(rolls)))
			sum := rolls[0] + rolls[1] + rolls[2]
			values = append(values, sum)
			breakdown = append(breakdown, sum)
		}
		sort.Sort(sort.Reverse(sort.IntSlice(values)))
		summary := "Rolled: 4d6, drop the lowest die for each of 6 sets"
		if len(req.RolledSets) == 7 {
			values = append([]int(nil), values[:6]...)
			summary = "Rolled: 7x 4d6, drop the lowest die in each set, then discard the weakest overall set and keep the best 6"
		}
		return values, breakdown, summary, nil
	default:
		return nil, nil, "", fmt.Errorf("unsupported method")
	}
}

func assignAbilityValues(values []int, className string) map[string]int {
	priorities := classAbilityPriority(className)
	assignment := map[string]int{}
	for index, ability := range priorities {
		if index >= len(values) {
			break
		}
		assignment[ability] = values[index]
	}
	return assignment
}

func classAbilityPriority(className string) []string {
	switch className {
	case "fighter", "paladin":
		return []string{"strength", "constitution", "dexterity", "wisdom", "charisma", "intelligence"}
	case "rogue", "ranger", "monk":
		return []string{"dexterity", "wisdom", "constitution", "strength", "charisma", "intelligence"}
	case "wizard":
		return []string{"intelligence", "constitution", "dexterity", "wisdom", "charisma", "strength"}
	case "cleric", "druid":
		return []string{"wisdom", "constitution", "dexterity", "strength", "charisma", "intelligence"}
	case "bard", "sorcerer", "warlock":
		return []string{"charisma", "constitution", "dexterity", "wisdom", "intelligence", "strength"}
	case "barbarian":
		return []string{"strength", "constitution", "dexterity", "wisdom", "charisma", "intelligence"}
	default:
		return []string{"strength", "dexterity", "constitution", "intelligence", "wisdom", "charisma"}
	}
}

func recommendedReasonForClass(className string) string {
	switch className {
	case "fighter", "paladin", "barbarian":
		return "Physical frontline classes prioritize Strength and Constitution."
	case "rogue", "ranger", "monk":
		return "Agile classes prioritize Dexterity, with Wisdom or Constitution next."
	case "wizard":
		return "Intelligence-driven classes need Intelligence first and durability second."
	case "cleric", "druid":
		return "Wisdom casters rely on Wisdom first, then Constitution or Dexterity."
	case "bard", "sorcerer", "warlock":
		return "Charisma casters prioritize Charisma first, then durability and initiative."
	default:
		return "Assignment follows a generic descending recommendation and can be adjusted manually."
	}
}

func validateAbilityAssignment(req ValidateAbilityAssignmentRequest) ValidateAbilityAssignmentResponse {
	expectedAbilities := []string{"strength", "dexterity", "constitution", "intelligence", "wisdom", "charisma"}
	expectedSet := map[string]struct{}{}
	for _, ability := range expectedAbilities {
		expectedSet[ability] = struct{}{}
	}

	missing := make([]string, 0)
	unexpected := make([]string, 0)
	assignedValues := make([]int, 0, len(req.Assignment))
	seenKeys := map[string]bool{}

	for key, value := range req.Assignment {
		normalized := strings.ToLower(strings.TrimSpace(key))
		if _, ok := expectedSet[normalized]; !ok {
			unexpected = append(unexpected, key)
			continue
		}
		seenKeys[normalized] = true
		assignedValues = append(assignedValues, value)
	}
	for _, ability := range expectedAbilities {
		if !seenKeys[ability] {
			missing = append(missing, ability)
		}
	}

	expectedValues := append([]int(nil), req.Values...)
	actualValues := append([]int(nil), assignedValues...)
	sort.Ints(expectedValues)
	sort.Ints(actualValues)

	duplicateValues := make([]int, 0)
	valid := len(missing) == 0 && len(unexpected) == 0 && len(expectedValues) == len(actualValues)
	if valid {
		for i := range expectedValues {
			if expectedValues[i] != actualValues[i] {
				valid = false
				break
			}
		}
	}
	if !valid {
		counts := map[int]int{}
		for _, value := range actualValues {
			counts[value]++
		}
		for value, count := range counts {
			if count > 1 {
				duplicateValues = append(duplicateValues, value)
			}
		}
		sort.Ints(duplicateValues)
	}

	return ValidateAbilityAssignmentResponse{
		Valid:             valid,
		Values:            req.Values,
		Assignment:        req.Assignment,
		MissingAbilities:  missing,
		UnexpectedKeys:    unexpected,
		DuplicateValues:   duplicateValues,
		RecommendedReason: recommendedReasonForClass(strings.ToLower(strings.TrimSpace(req.Class))),
	}
}
