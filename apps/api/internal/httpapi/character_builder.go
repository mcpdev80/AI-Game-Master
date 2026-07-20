package httpapi

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

//go:embed builder_guides/*.yaml embedded_rules/*.md
var embeddedBuilderGuides embed.FS

const (
	builderContextHardLimitTokens   = 110000
	builderSessionRolloverThreshold = 90000
	builderRecentTurnsAfterRollover = 4
)

var builderDocumentPagePattern = regexp.MustCompile(`(?i)(?:seite|page|s\.|pg\.)\s*(\d{1,4})`)

type builderClassRule struct {
	ClassName        string
	Aliases          []string
	SavingThrows     []string
	SkillChoiceCount int
	SkillChoices     []string
	HitDie           string
}

type builderRaceRule struct {
	RaceName                 string
	Aliases                  []string
	Speed                    string
	Darkvision               string
	FixedAbilityBonuses      []string
	FixedLanguages           []string
	FlexibleAbilityChoice    string
	ExtraSkillChoiceCount    int
	ExtraLanguageChoiceCount int
	NotableTraits            []string
}

type builderEquipmentAdvice struct {
	Options        []string
	Recommendation []string
	WeaponNotes    string
	CombatOverview string
}

type builderFeatureAdvice struct {
	Options        []string
	Recommendation []string
}

type builderSpellAdvice struct {
	Options           []string
	Recommendation    []string
	SpellNotes        string
	SpellAttacks      []string
	SpellSaveDC       string
	SpellAttackBonus  string
}

var builderCanonicalSkills = []struct {
	Name    string
	Aliases []string
}{
	{Name: "Akrobatik", Aliases: []string{"akrobatik", "acrobatics"}},
	{Name: "Arkane Kunde", Aliases: []string{"arkane kunde", "arcana"}},
	{Name: "Athletik", Aliases: []string{"athletik", "athletics", "athletic"}},
	{Name: "Auftreten", Aliases: []string{"auftreten", "performance"}},
	{Name: "Einschüchtern", Aliases: []string{"einschuchtern", "einschüchtern", "intimidation"}},
	{Name: "Fingerfertigkeit", Aliases: []string{"fingerfertigkeit", "sleight of hand"}},
	{Name: "Geschichte", Aliases: []string{"geschichte", "history"}},
	{Name: "Heilkunde", Aliases: []string{"heilkunde", "medicine"}},
	{Name: "Heimlichkeit", Aliases: []string{"heimlichkeit", "stealth"}},
	{Name: "Mit Tieren umgehen", Aliases: []string{"mit tieren umgehen", "animal handling"}},
	{Name: "Motiv erkennen", Aliases: []string{"motiv erkennen", "motive erkennen", "motiv erkenne", "insight"}},
	{Name: "Nachforschungen", Aliases: []string{"nachforschungen", "investigation"}},
	{Name: "Naturkunde", Aliases: []string{"naturkunde", "nature"}},
	{Name: "Religion", Aliases: []string{"religion"}},
	{Name: "Täuschung", Aliases: []string{"taeuschung", "täuschung", "deception"}},
	{Name: "Überlebenskunst", Aliases: []string{"uberlebenskunst", "überlebenskunst", "survival"}},
	{Name: "Überzeugen", Aliases: []string{"uberzeugen", "überzeugen", "persuasion"}},
	{Name: "Wahrnehmung", Aliases: []string{"wahrnehmung", "wahrnemung", "warhnemung", "perception"}},
}

var builderCoreClassRules = []builderClassRule{
	{ClassName: "Barbar", Aliases: []string{"barbar", "barbarian"}, SavingThrows: []string{"Stärke", "Konstitution"}, SkillChoiceCount: 2, SkillChoices: []string{"Mit Tieren umgehen", "Athletik", "Einschüchtern", "Naturkunde", "Wahrnehmung", "Überlebenskunst"}, HitDie: "W12"},
	{ClassName: "Barde", Aliases: []string{"barde", "bard"}, SavingThrows: []string{"Geschicklichkeit", "Charisma"}, SkillChoiceCount: 3, SkillChoices: []string{"Beliebige Fertigkeiten"}, HitDie: "W8"},
	{ClassName: "Kleriker", Aliases: []string{"kleriker", "cleric"}, SavingThrows: []string{"Weisheit", "Charisma"}, SkillChoiceCount: 2, SkillChoices: []string{"Geschichte", "Motiv erkennen", "Medizin", "Überzeugen", "Religion"}, HitDie: "W8"},
	{ClassName: "Druide", Aliases: []string{"druide", "druid"}, SavingThrows: []string{"Intelligenz", "Weisheit"}, SkillChoiceCount: 2, SkillChoices: []string{"Arkane Kunde", "Mit Tieren umgehen", "Heilkunde", "Motiv erkennen", "Naturkunde", "Wahrnehmung", "Religion", "Überlebenskunst"}, HitDie: "W8"},
	{ClassName: "Kämpfer", Aliases: []string{"kaempfer", "kämpfer", "fighter"}, SavingThrows: []string{"Stärke", "Konstitution"}, SkillChoiceCount: 2, SkillChoices: []string{"Akrobatik", "Mit Tieren umgehen", "Athletik", "Geschichte", "Motiv erkennen", "Einschüchtern", "Wahrnehmung", "Überlebenskunst"}, HitDie: "W10"},
	{ClassName: "Mönch", Aliases: []string{"moench", "mönch", "monk"}, SavingThrows: []string{"Stärke", "Geschicklichkeit"}, SkillChoiceCount: 2, SkillChoices: []string{"Akrobatik", "Athletik", "Geschichte", "Motiv erkennen", "Religion", "Heimlichkeit"}, HitDie: "W8"},
	{ClassName: "Paladin", Aliases: []string{"paladin"}, SavingThrows: []string{"Weisheit", "Charisma"}, SkillChoiceCount: 2, SkillChoices: []string{"Athletik", "Motiv erkennen", "Einschüchtern", "Medizin", "Überzeugen", "Religion"}, HitDie: "W10"},
	{ClassName: "Waldläufer", Aliases: []string{"waldlaeufer", "waldläufer", "ranger"}, SavingThrows: []string{"Stärke", "Geschicklichkeit"}, SkillChoiceCount: 3, SkillChoices: []string{"Mit Tieren umgehen", "Athletik", "Motiv erkennen", "Nachforschungen", "Naturkunde", "Wahrnehmung", "Heimlichkeit", "Überlebenskunst"}, HitDie: "W10"},
	{ClassName: "Schurke", Aliases: []string{"schurke", "rogue"}, SavingThrows: []string{"Geschicklichkeit", "Intelligenz"}, SkillChoiceCount: 4, SkillChoices: []string{"Akrobatik", "Athletik", "Täuschung", "Motiv erkennen", "Einschüchtern", "Nachforschungen", "Wahrnehmung", "Auftreten", "Überzeugen", "Fingerfertigkeit", "Heimlichkeit"}, HitDie: "W8"},
	{ClassName: "Zauberer", Aliases: []string{"zauberer", "sorcerer"}, SavingThrows: []string{"Konstitution", "Charisma"}, SkillChoiceCount: 2, SkillChoices: []string{"Arkane Kunde", "Täuschung", "Motiv erkennen", "Einschüchtern", "Überzeugen", "Religion"}, HitDie: "W6"},
	{ClassName: "Hexenmeister", Aliases: []string{"hexenmeister", "warlock"}, SavingThrows: []string{"Weisheit", "Charisma"}, SkillChoiceCount: 2, SkillChoices: []string{"Arkane Kunde", "Täuschung", "Geschichte", "Einschüchtern", "Nachforschungen", "Naturkunde", "Religion"}, HitDie: "W8"},
	{ClassName: "Magier", Aliases: []string{"magier", "wizard"}, SavingThrows: []string{"Intelligenz", "Weisheit"}, SkillChoiceCount: 2, SkillChoices: []string{"Arkane Kunde", "Geschichte", "Motiv erkennen", "Nachforschungen", "Medizin", "Religion"}, HitDie: "W6"},
}

var builderCoreRaceRules = []builderRaceRule{
	{RaceName: "Mensch", Aliases: []string{"mensch", "human"}, Speed: "30 Fuß", FixedAbilityBonuses: []string{"+1 auf alle Attribute"}, FixedLanguages: []string{"Gemeinsprache"}, ExtraLanguageChoiceCount: 1},
	{RaceName: "Halbelf", Aliases: []string{"halbelf", "half elf", "halfelf"}, Speed: "30 Fuß", Darkvision: "60 Fuß", FixedAbilityBonuses: []string{"+2 Charisma"}, FixedLanguages: []string{"Gemeinsprache", "Elfisch"}, FlexibleAbilityChoice: "+1 auf zwei verschiedene weitere Attribute nach Wahl", ExtraSkillChoiceCount: 2, ExtraLanguageChoiceCount: 1, NotableTraits: []string{"Fey Ancestry"}},
	{RaceName: "Halbork", Aliases: []string{"halbork", "half orc", "halborc"}, Speed: "30 Fuß", Darkvision: "60 Fuß", FixedAbilityBonuses: []string{"+2 Stärke", "+1 Konstitution"}, FixedLanguages: []string{"Gemeinsprache", "Orkisch"}, NotableTraits: []string{"Relentless Endurance", "Savage Attacks"}},
	{RaceName: "Tiefling", Aliases: []string{"tiefling"}, Speed: "30 Fuß", Darkvision: "60 Fuß", FixedAbilityBonuses: []string{"+1 Intelligenz", "+2 Charisma"}, FixedLanguages: []string{"Gemeinsprache", "Infernal"}, NotableTraits: []string{"Feuerresistenz"}},
	{RaceName: "Hügelzwerg", Aliases: []string{"huegelzwerg", "hügelzwerg", "hill dwarf"}, Speed: "25 Fuß", Darkvision: "60 Fuß", FixedAbilityBonuses: []string{"+2 Konstitution", "+1 Weisheit"}, FixedLanguages: []string{"Gemeinsprache", "Zwergisch"}},
	{RaceName: "Zwerg", Aliases: []string{"zwerg", "dwarf"}, Speed: "25 Fuß", Darkvision: "60 Fuß", FixedAbilityBonuses: []string{"+2 Konstitution"}, FixedLanguages: []string{"Gemeinsprache", "Zwergisch"}},
	{RaceName: "Hochelf", Aliases: []string{"hochelf", "high elf"}, Speed: "30 Fuß", Darkvision: "60 Fuß", FixedAbilityBonuses: []string{"+2 Geschicklichkeit", "+1 Intelligenz"}, FixedLanguages: []string{"Gemeinsprache", "Elfisch"}, ExtraLanguageChoiceCount: 1},
	{RaceName: "Elf", Aliases: []string{"elf"}, Speed: "30 Fuß", Darkvision: "60 Fuß", FixedAbilityBonuses: []string{"+2 Geschicklichkeit"}, FixedLanguages: []string{"Gemeinsprache", "Elfisch"}},
	{RaceName: "Leichtfuß-Halbling", Aliases: []string{"leichtfuss halbling", "leichtfuß halbling", "lightfoot halfling"}, Speed: "25 Fuß", FixedAbilityBonuses: []string{"+2 Geschicklichkeit", "+1 Charisma"}, FixedLanguages: []string{"Gemeinsprache", "Halblingisch"}},
	{RaceName: "Halbling", Aliases: []string{"halbling", "halfling"}, Speed: "25 Fuß", FixedAbilityBonuses: []string{"+2 Geschicklichkeit"}, FixedLanguages: []string{"Gemeinsprache", "Halblingisch"}},
	{RaceName: "Felsgnom", Aliases: []string{"felsgnom", "rock gnome"}, Speed: "25 Fuß", Darkvision: "60 Fuß", FixedAbilityBonuses: []string{"+2 Intelligenz", "+1 Konstitution"}, FixedLanguages: []string{"Gemeinsprache", "Gnomisch"}},
	{RaceName: "Gnom", Aliases: []string{"gnom", "gnome"}, Speed: "25 Fuß", Darkvision: "60 Fuß", FixedAbilityBonuses: []string{"+2 Intelligenz"}, FixedLanguages: []string{"Gemeinsprache", "Gnomisch"}},
	{RaceName: "Drachenblütiger", Aliases: []string{"drachenbluetiger", "drachenblütiger", "dragonborn"}, Speed: "30 Fuß", FixedAbilityBonuses: []string{"+2 Stärke", "+1 Charisma"}, FixedLanguages: []string{"Gemeinsprache", "Drakonisch"}, NotableTraits: []string{"Atemwaffe", "elementare Resistenz"}},
}

func (h *Handler) startCharacterBuilder(c *gin.Context) {
	var req StartCharacterBuilderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, 400, "invalid character builder payload", err)
		return
	}

	documents, err := h.loadSelectedBuilderDocuments(c.Request.Context(), req.SelectedDocumentIDs)
	if err != nil {
		errorResponse(c, 500, "load selected rulebooks", err)
		return
	}
	documents = append(documents, embeddedGuidesForRuleset(req.RulesetWork, req.RulesetVersion)...)
	if raceReference := embeddedRaceReferenceGuide(req.RulesetWork, req.RulesetVersion); raceReference.ID != "" {
		documents = append(documents, raceReference)
	}
	documents = dedupeDocumentsByID(documents)

	selectedDocuments := make([]Document, 0, len(documents))
	selectedNames := make([]string, 0, len(documents))
	for _, document := range documents {
		if document.Type != "rules" {
			continue
		}
		selectedDocuments = append(selectedDocuments, document)
		selectedNames = append(selectedNames, document.Name)
	}
	if len(selectedDocuments) == 0 {
		errorResponse(c, 400, "start character builder", fmt.Errorf("at least one rules document is required"))
		return
	}
	language := strings.ToLower(strings.TrimSpace(req.Language))
	if language != "de" {
		language = "en"
	}

	intro := fmt.Sprintf("You are creating a new character for %s %s. We will primarily use %s. First, describe the kind of character you want to play: a mood, role, class, or rough fantasy concept is enough to begin.", req.RulesetWork, req.RulesetVersion, strings.Join(selectedNames, ", "))
	characterName := "New Character"
	if language == "de" {
		intro = fmt.Sprintf("Du möchtest einen neuen Charakter für %s %s erstellen. Wir orientieren uns primär an %s. Erzähl mir zuerst, was für eine Figur du spielen möchtest: Stimmung, Rolle, Klasse oder eine grobe Fantasy-Idee reichen für den Start.", req.RulesetWork, req.RulesetVersion, strings.Join(selectedNames, ", "))
		characterName = "Neuer Charakter"
	}
	now := time.Now().UTC()
	messages := []CharacterBuilderMessage{
		{
			Role:      "assistant",
			Content:   intro,
			CreatedAt: now,
		},
	}

	character := Character{
		CampaignID: req.CampaignID,
		Name:       characterName,
		PlayerName: strings.TrimSpace(req.PlayerName),
		Abilities:  map[string]int{},
		Languages:  []string{},
		Features:   []string{},
		Metadata: map[string]any{
			"ruleset_work":            strings.TrimSpace(req.RulesetWork),
			"ruleset_version":         strings.TrimSpace(req.RulesetVersion),
			"language":                language,
			"builder_status":          "draft",
			"builder_mode":            "ai_chat",
			"builder_stage":           "concept",
			"selected_document_ids":   req.SelectedDocumentIDs,
			"selected_document_names": selectedNames,
			"builder_messages":        builderMessagesToMetadata(messages),
		},
	}

	created, err := h.store.CreateCharacter(c.Request.Context(), character)
	if err != nil {
		errorResponse(c, 500, "create character draft", err)
		return
	}

	llmSession, err := h.store.CreateLLMSession(c.Request.Context(), LLMSession{
		SessionType:    "character_builder_session",
		ScopeType:      "character",
		ScopeID:        created.ID,
		RequestProfile: "builder",
		RulesetWork:    strings.TrimSpace(req.RulesetWork),
		RulesetVersion: strings.TrimSpace(req.RulesetVersion),
		MessageHistory: builderMessagesToMetadata(messages),
		WorkingSummary: map[string]any{
			"builder_stage": "concept",
		},
		StructuredState: map[string]any{
			"character_id": created.ID,
		},
		TokenBudget:  6000,
		LastActiveAt: now,
	})
	if err != nil {
		errorResponse(c, 500, "create character builder session", err)
		return
	}
	levelUpSession, err := h.store.CreateLLMSession(c.Request.Context(), LLMSession{
		SessionType:    "level_up_session",
		ScopeType:      "character",
		ScopeID:        created.ID,
		RequestProfile: "level_up",
		RulesetWork:    strings.TrimSpace(req.RulesetWork),
		RulesetVersion: strings.TrimSpace(req.RulesetVersion),
		MessageHistory: []map[string]any{},
		WorkingSummary: map[string]any{
			"builder_stage": "not_started",
		},
		StructuredState: map[string]any{
			"character_id": created.ID,
		},
		TokenBudget:  4000,
		LastActiveAt: now,
	})
	if err != nil {
		errorResponse(c, 500, "create character level up session", err)
		return
	}

	if created.Metadata == nil {
		created.Metadata = map[string]any{}
	}
	created.Metadata["llm_session_id"] = llmSession.ID
	created.Metadata["level_up_session_id"] = levelUpSession.ID
	created, err = h.store.UpdateCharacter(c.Request.Context(), created)
	if err != nil {
		errorResponse(c, 500, "link character builder session", err)
		return
	}

	c.JSON(201, StartCharacterBuilderResponse{
		Character: created,
		Messages:  messages,
	})
}

func (h *Handler) characterBuilderMessage(c *gin.Context) {
	var req CharacterBuilderMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, 400, "invalid builder message payload", err)
		return
	}

	character, err := h.store.GetCharacter(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(404, gin.H{"error": "character not found"})
			return
		}
		errorResponse(c, 500, "load character", err)
		return
	}
	if req.Language == "en" || req.Language == "de" {
		if character.Metadata == nil {
			character.Metadata = map[string]any{}
		}
		character.Metadata["language"] = req.Language
	}

	documents, err := h.loadSelectedBuilderDocuments(c.Request.Context(), stringListFromAny(character.Metadata["selected_document_ids"]))
	if err != nil {
		errorResponse(c, 500, "load selected rulebooks", err)
		return
	}
	documents = append(
		documents,
		embeddedGuidesForRuleset(
			fmt.Sprintf("%v", character.Metadata["ruleset_work"]),
			fmt.Sprintf("%v", character.Metadata["ruleset_version"]),
		)...,
	)
	if raceReference := embeddedRaceReferenceGuide(
		fmt.Sprintf("%v", character.Metadata["ruleset_work"]),
		fmt.Sprintf("%v", character.Metadata["ruleset_version"]),
	); raceReference.ID != "" {
		documents = append(documents, raceReference)
	}
	documents = dedupeDocumentsByID(documents)

	messages := builderMessagesFromMetadata(character.Metadata)
	llmSession, err := h.loadOrCreateCharacterBuilderSession(c.Request.Context(), &character, messages)
	if err != nil {
		errorResponse(c, 500, "load character builder session", err)
		return
	}
	if len(llmSession.MessageHistory) > 0 {
		messages = builderMessagesFromHistory(llmSession.MessageHistory)
	}
	userMessage := CharacterBuilderMessage{
		Role:      "user",
		Content:   strings.TrimSpace(req.Message),
		CreatedAt: time.Now().UTC(),
	}
	messages = append(messages, userMessage)

	llmSession, messages, err = h.rolloverCharacterBuilderSessionIfNeeded(c.Request.Context(), &character, llmSession, documents, messages)
	if err != nil {
		errorResponse(c, 500, "rollover character builder session", err)
		return
	}

	completion, err := h.completeCharacterBuilder(c.Request.Context(), &character, &llmSession, documents, messages)
	if err != nil {
		if isLLMGatewayBusy(err) {
			assistantMessage := CharacterBuilderMessage{
				Role:      "assistant",
				Content:   "Der Builder ist gerade ausgelastet. Bitte warte kurz und sende deine letzte Nachricht in ein paar Sekunden erneut.",
				CreatedAt: time.Now().UTC(),
			}
			messages = append(messages, assistantMessage)
			if character.Metadata == nil {
				character.Metadata = map[string]any{}
			}
			character.Metadata["builder_messages"] = builderMessagesToMetadata(messages)
			llmSession.MessageHistory = builderMessagesToMetadata(messages)
			llmSession.LastActiveAt = assistantMessage.CreatedAt
			llmSession.WorkingSummary = mergeBuilderRuntimeSummary(llmSession.WorkingSummary, character, documents, messages)
			if sessionErr := h.saveCharacterBuilderState(c.Request.Context(), &character, &llmSession); sessionErr != nil {
				errorResponse(c, 500, "save character builder session", sessionErr)
				return
			}
			updated := character
			c.JSON(200, CharacterBuilderMessageResponse{
				Character:    updated,
				Messages:     messages,
				Reply:        assistantMessage.Content,
				AppliedPatch: CharacterBuilderPatch{},
			})
			return
		}
		if isCharacterBuilderTimeout(err) {
			assistantMessage := CharacterBuilderMessage{
				Role:      "assistant",
				Content:   "Die KI hat für diesen Schritt zu lange gebraucht. Bitte schicke die letzte Frage oder Anweisung noch einmal etwas kürzer oder in einem kleineren Teilschritt, dann machen wir direkt weiter.",
				CreatedAt: time.Now().UTC(),
			}
			messages = append(messages, assistantMessage)
			if character.Metadata == nil {
				character.Metadata = map[string]any{}
			}
			character.Metadata["builder_messages"] = builderMessagesToMetadata(messages)
			llmSession.MessageHistory = builderMessagesToMetadata(messages)
			llmSession.LastActiveAt = assistantMessage.CreatedAt
			llmSession.WorkingSummary = mergeBuilderRuntimeSummary(llmSession.WorkingSummary, character, documents, messages)
			if sessionErr := h.saveCharacterBuilderState(c.Request.Context(), &character, &llmSession); sessionErr != nil {
				errorResponse(c, 500, "save character builder session", sessionErr)
				return
			}
			updated := character
			c.JSON(200, CharacterBuilderMessageResponse{
				Character:    updated,
				Messages:     messages,
				Reply:        assistantMessage.Content,
				AppliedPatch: CharacterBuilderPatch{},
			})
			return
		}
		errorResponse(c, 502, "character builder response", err)
		return
	}

	currentStage := currentBuilderStage(character, &llmSession)
	sanitizeCharacterBuilderPatchForStage(&completion.Patch, currentStage)
	if isBuilderStoryTransferMessage(req.Message, latestBuilderAssistantReply(messages)) {
		applyBuilderStoryTransferPatch(&completion.Patch, latestBuilderStoryDraft(messages, completion.Reply))
	}
	assistantMessage := CharacterBuilderMessage{
		Role:      "assistant",
		Content:   completion.Reply,
		CreatedAt: time.Now().UTC(),
	}
	messages = append(messages, assistantMessage)

	applyCharacterPatch(&character, completion.Patch)
	if character.Metadata == nil {
		character.Metadata = map[string]any{}
	}
	if normalizedStage := normalizeBuilderStage(safeOptionalString(character.Metadata["builder_stage"])); normalizedStage != "" {
		character.Metadata["builder_stage"] = normalizedStage
	}
	character.Metadata["builder_messages"] = builderMessagesToMetadata(messages)
	llmSession.MessageHistory = builderMessagesToMetadata(messages)
	llmSession.LastActiveAt = assistantMessage.CreatedAt
	llmSession.WorkingSummary = mergeBuilderRuntimeSummary(llmSession.WorkingSummary, character, documents, messages)
	llmSession.StructuredState = mergeBuilderStructuredState(llmSession.StructuredState, character)
	if err := h.saveCharacterBuilderState(c.Request.Context(), &character, &llmSession); err != nil {
		errorResponse(c, 500, "save character builder session", err)
		return
	}
	updated := character

	c.JSON(200, CharacterBuilderMessageResponse{
		Character:    updated,
		Messages:     messages,
		Reply:        completion.Reply,
		AppliedPatch: completion.Patch,
		UIAction:     completion.UIAction,
		UIPayload:    completion.UIPayload,
	})
}

func (h *Handler) loadSelectedBuilderDocuments(ctx context.Context, documentIDs []string) ([]Document, error) {
	embeddedByID := map[string]Document{}
	for _, document := range embeddedBuilderGuideDocuments() {
		embeddedByID[document.ID] = document
	}
	persistedDocumentIDs := make([]string, 0, len(documentIDs))
	documents := make([]Document, 0, len(documentIDs))
	for _, documentID := range documentIDs {
		if embedded, ok := embeddedByID[documentID]; ok {
			documents = append(documents, embedded)
			continue
		}
		persistedDocumentIDs = append(persistedDocumentIDs, documentID)
	}
	if len(persistedDocumentIDs) == 0 {
		return documents, nil
	}
	persistedDocuments, err := h.store.ListDocumentsByIDs(ctx, persistedDocumentIDs)
	if err != nil {
		return nil, err
	}
	return append(documents, persistedDocuments...), nil
}

func (h *Handler) applyCharacterBuilderPatch(c *gin.Context) {
	var req CharacterBuilderApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, 400, "invalid builder patch payload", err)
		return
	}

	character, err := h.store.GetCharacter(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(404, gin.H{"error": "character not found"})
			return
		}
		errorResponse(c, 500, "load character", err)
		return
	}

	applyCharacterPatch(&character, req.Patch)
	if character.Metadata == nil {
		character.Metadata = map[string]any{}
	}
	if normalizedStage := normalizeBuilderStage(safeOptionalString(character.Metadata["builder_stage"])); normalizedStage != "" {
		character.Metadata["builder_stage"] = normalizedStage
	}
	updated, err := h.store.UpdateCharacter(c.Request.Context(), character)
	if err != nil {
		errorResponse(c, 500, "apply character patch", err)
		return
	}

	messages := builderMessagesFromMetadata(updated.Metadata)
	llmSession, sessionErr := h.loadOrCreateCharacterBuilderSession(c.Request.Context(), &updated, messages)
	if sessionErr == nil {
		llmSession.WorkingSummary = mergeBuilderSummary(llmSession.WorkingSummary, updated)
		llmSession.StructuredState = mergeBuilderStructuredState(llmSession.StructuredState, updated)
		llmSession.LastActiveAt = time.Now().UTC()
		if _, sessionErr = h.store.UpdateLLMSession(c.Request.Context(), llmSession); sessionErr != nil {
			errorResponse(c, 500, "apply character builder session patch", sessionErr)
			return
		}
	}

	c.JSON(200, updated)
}

func (h *Handler) finishCharacterBuilder(c *gin.Context) {
	character, err := h.store.GetCharacter(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(404, gin.H{"error": "character not found"})
			return
		}
		errorResponse(c, 500, "load character", err)
		return
	}

	if character.Metadata == nil {
		character.Metadata = map[string]any{}
	}
	character.Metadata["builder_status"] = "ready"
	character.Metadata["builder_stage"] = "complete"

	updated, err := h.store.UpdateCharacter(c.Request.Context(), character)
	if err != nil {
		errorResponse(c, 500, "finish character builder", err)
		return
	}

	c.JSON(200, updated)
}

type characterBuilderCompletion struct {
	Reply     string
	Patch     CharacterBuilderPatch
	UIAction  string
	UIPayload map[string]any
}

func (h *Handler) completeCharacterBuilder(ctx context.Context, character *Character, llmSession *LLMSession, documents []Document, transcript []CharacterBuilderMessage) (characterBuilderCompletion, error) {
	ctx = withLLMRequestMeta(ctx, llmRequestMeta{
		Profile:        "character_builder",
		ScopeType:      "character",
		ScopeID:        character.ID,
		MaxInputTokens: builderContextHardLimitTokens,
	})
	selectedDocs := make([]string, 0, len(documents))
	for _, document := range documents {
		if strings.HasSuffix(strings.TrimSpace(fmt.Sprintf("%v", document.Metadata["kind"])), "_guide") {
			continue
		}
		selectedDocs = append(selectedDocs, document.Name)
	}

	currentRuleset := fmt.Sprintf("%v %v", character.Metadata["ruleset_work"], character.Metadata["ruleset_version"])
	builderStage := currentBuilderStage(*character, llmSession)
	sheetJSON, _ := json.Marshal(compactCharacterDraftForPrompt(*character))
	builderQuery := builderContextQueryForCharacter(*character, builderStage, latestBuilderUserMessage(transcript))
	raceReference := loadEmbeddedBuilderGuide(
		strings.TrimSpace(fmt.Sprintf("%v", character.Metadata["ruleset_work"])),
		strings.TrimSpace(fmt.Sprintf("%v", character.Metadata["ruleset_version"])),
		"race_reference",
	)
	relevantRulesContext := h.retrieveBuilderContext(ctx, documents, builderQuery, 3)
	retrievalEvidence := h.retrieveBuilderEvidence(ctx, documents, builderQuery, 6)
	builderContextJSON, _ := json.Marshal(buildCharacterBuilderPromptContext(character, llmSession, documents, transcript, relevantRulesContext, builderQuery, retrievalEvidence))
	runtimeSummary := mergeBuilderRuntimeSummary(defaultMetadata(llmSession.WorkingSummary), *character, documents, transcript)
	llmSession.WorkingSummary = runtimeSummary
	builderSummaryJSON, _ := json.Marshal(runtimeSummary)
	guideSummary := strings.TrimSpace(fmt.Sprint(runtimeSummary["guide_summary"]))
	levelUpSummary := strings.TrimSpace(fmt.Sprint(runtimeSummary["level_up_summary"]))
	conversationSummary := strings.TrimSpace(fmt.Sprint(runtimeSummary["conversation_summary"]))
	latestUserMessage := latestBuilderUserMessage(transcript)
	previousAssistant := latestBuilderAssistantReply(transcript)
	latestStoryOptions := latestBuilderStoryOptions(transcript)
	language := builderCharacterLanguage(character)

	if language == "de" {
		if storyCompletion, ok := builderDeterministicStorySelectionCompletion(character, latestUserMessage, latestStoryOptions); ok {
			return storyCompletion, nil
		}
		if guidedCompletion, ok := builderDeterministicGuidedChoiceCompletion(character, builderStage, latestUserMessage, previousAssistant); ok {
			return guidedCompletion, nil
		}
	}
	systemPrompt := mustFormatEmbeddedPrompt("prompts/character_builder_system_prompt.md", guideSummary, levelUpSummary)
	if language == "en" {
		systemPrompt += "\n\nMANDATORY OUTPUT LANGUAGE: English. Every user-facing reply, option, label, explanation, error, and follow-up question must be entirely in English. Never mix German into the reply. Preserve exact proper names only when necessary. JSON keys remain unchanged."
	} else {
		systemPrompt += "\n\nVERBINDLICHE AUSGABESPRACHE: Deutsch. Alle sichtbaren Antworten, Optionen, Erklärungen und Rückfragen müssen vollständig auf Deutsch sein. Mische kein Englisch in den reply, außer unveränderte Eigennamen erfordern es. JSON-Schlüssel bleiben unverändert."
	}

	llmMessages := []map[string]string{
		{
			"role":    "system",
			"content": systemPrompt,
		},
		{
			"role": "user",
			"content": fmt.Sprintf(
				"Regelwerk: %s\nAusgewaehlte Buecher: %s\nBuilder-Suchanfrage: %s\nBuilder-Kontext: %s\nAktueller Character-Draft: %s\nBuilder-Zusammenfassung: %s\nBisheriger Gespraechsverlauf komprimiert: %s\nDialogverlauf (letzte Turns):\n%s",
				currentRuleset,
				strings.Join(selectedDocs, ", "),
				builderQuery,
				string(builderContextJSON),
				string(sheetJSON),
				string(builderSummaryJSON),
				firstNonEmpty(conversationSummary, "(noch keine aeltere Gespraechszusammenfassung)"),
				renderCompactBuilderTranscript(compactBuilderTranscript(transcript, 6)),
			),
		},
	}

	content, _, err := h.llmClient.chatCompletion(ctx, llmMessages, true, 1200)
	if err != nil {
		return characterBuilderCompletion{Reply: builderFallbackReply(builderStage, character, language), Patch: CharacterBuilderPatch{}}, nil
	}

	parsed, err := h.parseCharacterBuilderResponse(ctx, content)
	if err != nil {
		return characterBuilderCompletion{Reply: builderFallbackReply(builderStage, character, language), Patch: CharacterBuilderPatch{}}, nil
	}
	if language == "de" {
		if len(retrievalEvidence) == 0 {
			if directReply, ok := builderDirectRaceReferenceReply(latestBuilderUserMessage(transcript), raceReference); ok && strings.TrimSpace(directReply) != "" {
				parsed.Reply = directReply
			}
		} else if adjustedReply, adjusted := h.verifyBuilderEvidence(ctx, latestBuilderUserMessage(transcript), retrievalEvidence, parsed.Reply); adjusted && strings.TrimSpace(adjustedReply) != "" {
			parsed.Reply = adjustedReply
		}
	}
	if language == "de" && builderNeedsStoryRetry(latestBuilderUserMessage(transcript), parsed) {
		retryMessages := append([]map[string]string{}, llmMessages...)
		retryMessages = append(retryMessages, map[string]string{
			"role":    "user",
			"content": "Deine letzte Antwort war unvollständig. Wenn der Nutzer verlangt, dass du die Geschichte oder Persoenlichkeitsinhalte jetzt erstellst, musst du den eigentlichen Text direkt ausschreiben und in updates.metadata.backstory, updates.metadata.concept, updates.metadata.personality_traits, updates.metadata.ideals, updates.metadata.bonds und updates.metadata.flaws setzen, soweit sie erzeugt wurden. Antworte erneut nur als JSON.",
		})
		retriedContent, _, retryErr := h.llmClient.chatCompletion(ctx, retryMessages, true, 1200)
		if retryErr == nil {
			retriedParsed, parseErr := h.parseCharacterBuilderResponse(ctx, retriedContent)
			if parseErr == nil {
				parsed = retriedParsed
			}
		}
	}
	if strings.TrimSpace(parsed.Reply) == "" {
		return characterBuilderCompletion{Reply: builderFallbackReply(builderStage, character, language), Patch: CharacterBuilderPatch{}}, nil
	}

	if language == "de" && (builderStoryDraftRequested(latestUserMessage) || builderStoryTransferRequested(latestUserMessage, previousAssistant)) {
		switch {
		case builderStoryTransferRequested(latestUserMessage, previousAssistant) && !builderReplyLooksLikeStoryDraft(previousAssistant):
			parsed.Reply = builderStoryProposalFallback(character)
			parsed.Updates = CharacterBuilderPatch{}
		case builderShouldOfferStoryProposals(latestUserMessage, previousAssistant) && !builderReplyLooksLikeStoryOptions(parsed.Reply):
			parsed.Reply = builderStoryProposalFallback(character)
			parsed.Updates = CharacterBuilderPatch{}
		case builderShouldWriteFullStory(latestUserMessage, previousAssistant) && !builderReplyLooksLikeStoryDraft(parsed.Reply):
			parsed.Reply = builderStoryDraftFallback(character, previousAssistant)
			parsed.Updates = CharacterBuilderPatch{}
		}
	}
	if builderStoryDraftRequested(latestUserMessage) || builderStoryTransferRequested(latestUserMessage, previousAssistant) {
		stripBuilderStoryPatch(&parsed.Updates)
		if builderStoryTransferRequested(latestUserMessage, previousAssistant) {
			applyBuilderStoryTransferPatch(&parsed.Updates, latestBuilderStoryDraft(transcript, parsed.Reply))
		}
	}

	normalizeCharacterBuilderPatch(&parsed.Updates)
	completion := characterBuilderCompletion{
		Reply: parsed.Reply,
		Patch: parsed.Updates,
	}
	if uiAction, uiPayload := builderDocumentUIAction(latestUserMessage, documents); uiAction != "" {
		completion.UIAction = uiAction
		completion.UIPayload = uiPayload
	}
	return completion, nil
}

func builderDocumentUIAction(question string, documents []Document) (string, map[string]any) {
	normalizedQuestion := normalizeBuilderIntentText(question)
	if normalizedQuestion == "" {
		return "", nil
	}
	if !builderShowDocumentRequested(normalizedQuestion) {
		return "", nil
	}

	document := pickBuilderReferenceDocument(normalizedQuestion, documents)
	if document == nil {
		return "", nil
	}

	payload := map[string]any{
		"document_id":   document.ID,
		"document_name": document.Name,
	}
	if page := builderDocumentPageHint(question, *document); page > 0 {
		payload["document_page"] = page
	}
	return "open_document", payload
}

func builderShowDocumentRequested(message string) bool {
	message = normalizeBuilderIntentText(message)
	if message == "" {
		return false
	}
	showIntent := strings.Contains(message, "zeige mir") ||
		strings.Contains(message, "zeig mir") ||
		strings.Contains(message, "zeige") ||
		strings.Contains(message, "zeig")
	ruleIntent := strings.Contains(message, "aus dem regelwerk") ||
		strings.Contains(message, "im regelwerk") ||
		strings.Contains(message, "regelwerk") ||
		strings.Contains(message, "regeln") ||
		strings.Contains(message, "regel ") ||
		strings.Contains(message, " pdf") ||
		strings.Contains(message, "seite") ||
		strings.Contains(message, "möglichkeiten") ||
		strings.Contains(message, "moeglichkeiten") ||
		strings.Contains(message, "optionen") ||
		strings.Contains(message, "liste") ||
		strings.Contains(message, "buch")
	return showIntent && ruleIntent
}

func builderDocumentPageHint(message string, document Document) int {
	match := builderDocumentPagePattern.FindStringSubmatch(strings.TrimSpace(message))
	if len(match) < 2 {
		return builderDefaultDocumentPage(message, document)
	}
	page, err := strconv.Atoi(match[1])
	if err != nil || page <= 0 {
		return builderDefaultDocumentPage(message, document)
	}
	return page
}

func builderDefaultDocumentPage(message string, document Document) int {
	normalized := normalizeBuilderIntentText(message)
	documentName := normalizeBuilderIntentText(document.Name)
	if normalized == "" {
		return 0
	}

	if builderMentionsRulesBackgroundTopic(normalized) &&
		(strings.Contains(documentName, "spielerhandbuch") || strings.Contains(documentName, "player s handbook") || strings.Contains(documentName, "player handbook")) {
		return 127
	}
	return 0
}

func builderMentionsRulesBackgroundTopic(normalized string) bool {
	return strings.Contains(normalized, "background") ||
		strings.Contains(normalized, "hintergrund") ||
		strings.Contains(normalized, "hintergruend") ||
		strings.Contains(normalized, "hintergr")
}

func pickBuilderReferenceDocument(question string, documents []Document) *Document {
	normalized := normalizeBuilderIntentText(question)
	if normalized == "" {
		return nil
	}

	bestIndex := -1
	bestScore := 0
	for index := range documents {
		document := documents[index]
		if strings.HasSuffix(strings.TrimSpace(fmt.Sprint(document.Metadata["kind"])), "_guide") {
			continue
		}
		sourceType := strings.TrimSpace(fmt.Sprint(document.Metadata["source_type"]))
		if strings.EqualFold(sourceType, "embedded_race_reference") || strings.EqualFold(fmt.Sprint(document.Metadata["system_document"]), "true") {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(document.Type), "rules") {
			continue
		}
		score := 1
		lowerName := strings.ToLower(document.Name)
		nameTokens := normalizeBuilderIntentText(document.Name)
		if nameTokens != "" && strings.Contains(normalized, nameTokens) {
			score += 5
		}
		if strings.Contains(normalized, "spielerhandbuch") && strings.Contains(lowerName, "spielerhandbuch") {
			score += 8
		}
		if builderMentionsRulesBackgroundTopic(normalized) {
			if strings.Contains(lowerName, "spielerhandbuch") || strings.Contains(lowerName, "player") {
				score += 10
			}
		}
		if score > bestScore {
			bestScore = score
			bestIndex = index
		}
	}
	if bestIndex < 0 {
		for index := range documents {
			document := documents[index]
			if strings.EqualFold(strings.TrimSpace(document.Type), "rules") {
				return &documents[index]
			}
		}
		return nil
	}
	return &documents[bestIndex]
}

func (h *Handler) parseCharacterBuilderResponse(ctx context.Context, content string) (struct {
	Reply   string                `json:"reply"`
	Updates CharacterBuilderPatch `json:"updates"`
}, error) {
	type llmCharacterBuilderResponse struct {
		Reply   string                `json:"reply"`
		Updates CharacterBuilderPatch `json:"updates"`
	}

	trimmed := strings.TrimSpace(content)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)

	var parsed llmCharacterBuilderResponse
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		repaired, repairErr := h.llmClient.repairCharacterBuilderResponse(ctx, trimmed)
		if repairErr != nil {
			return struct {
				Reply   string                `json:"reply"`
				Updates CharacterBuilderPatch `json:"updates"`
			}{}, fmt.Errorf("parse character builder response: %w", err)
		}
		repaired = strings.TrimSpace(repaired)
		repaired = strings.TrimPrefix(repaired, "```json")
		repaired = strings.TrimPrefix(repaired, "```")
		repaired = strings.TrimSuffix(repaired, "```")
		repaired = strings.TrimSpace(repaired)
		if err := json.Unmarshal([]byte(repaired), &parsed); err != nil {
			return struct {
				Reply   string                `json:"reply"`
				Updates CharacterBuilderPatch `json:"updates"`
			}{}, fmt.Errorf("parse character builder response: %w", err)
		}
	}

	return struct {
		Reply   string                `json:"reply"`
		Updates CharacterBuilderPatch `json:"updates"`
	}{
		Reply:   parsed.Reply,
		Updates: parsed.Updates,
	}, nil
}

func builderNeedsStoryRetry(latestUserMessage string, parsed struct {
	Reply   string                `json:"reply"`
	Updates CharacterBuilderPatch `json:"updates"`
}) bool {
	message := strings.ToLower(strings.TrimSpace(latestUserMessage))
	if message == "" {
		return false
	}
	if !strings.Contains(message, "geschichte") &&
		!strings.Contains(message, "backstory") &&
		!strings.Contains(message, "hintergrund") &&
		!strings.Contains(message, "merkmale") &&
		!strings.Contains(message, "ideale") &&
		!strings.Contains(message, "bindungen") &&
		!strings.Contains(message, "makel") {
		return false
	}

	metadata := defaultMetadata(parsed.Updates.Metadata)
	hasStoryPatch := strings.TrimSpace(fmt.Sprint(metadata["backstory"])) != "" ||
		strings.TrimSpace(fmt.Sprint(metadata["personality_traits"])) != "" ||
		strings.TrimSpace(fmt.Sprint(metadata["ideals"])) != "" ||
		strings.TrimSpace(fmt.Sprint(metadata["bonds"])) != "" ||
		strings.TrimSpace(fmt.Sprint(metadata["flaws"])) != ""
	if hasStoryPatch {
		return false
	}

	reply := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(parsed.Reply)), " "))
	if reply == "" {
		return true
	}
	return strings.Contains(reply, "ich habe") ||
		strings.Contains(reply, "entworfen") ||
		strings.Contains(reply, "erstellt") ||
		strings.Contains(reply, "eingetragen") ||
		strings.Contains(reply, "vorschläge") ||
		strings.Contains(reply, "vorschlaege")
}

func builderStoryDraftRequested(message string) bool {
	message = normalizeBuilderIntentText(message)
	if message == "" {
		return false
	}
	return strings.Contains(message, "hintergrundgeschichte") ||
		strings.Contains(message, "backstory") ||
		strings.Contains(message, "story") ||
		strings.Contains(message, "hintergrundtext") ||
		strings.Contains(message, "ausformulier") ||
		strings.Contains(message, "ausschm") ||
		(strings.Contains(message, "geschichte") && !strings.Contains(message, "geschichte und")) ||
		strings.Contains(message, "story") ||
		strings.Contains(message, "entwurf") ||
		strings.Contains(message, "charaktergeschichte")
}

func builderStoryTransferRequested(message string, previousAssistant string) bool {
	return isBuilderStoryTransferMessage(message, previousAssistant)
}

func isBuilderStoryTransferMessage(message string, previousAssistant string) bool {
	message = normalizeBuilderIntentText(message)
	if message == "" {
		return false
	}
	storyContext := builderStoryDraftRequested(message) ||
		builderReplyLooksLikeStoryOptions(previousAssistant) ||
		builderReplyLooksLikeStoryDraft(previousAssistant)
	if !storyContext {
		return false
	}
	keywords := []string{
		"uebernimm",
		"uebernehmen",
		"uebernehme",
		"trage",
		"trag",
		"eintragen",
		"eintrag",
		"speichern",
		"finalisieren",
		"draft",
		"konzept",
		"uebertrag",
		"uebernahme",
		"uebergeben",
	}
	for _, keyword := range keywords {
		if strings.Contains(message, keyword) {
			return true
		}
	}
	return false
}

func normalizeBuilderIntentText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"ä", "ae",
		"ö", "oe",
		"ü", "ue",
		"ß", "ss",
		"é", "e",
		"è", "e",
		"ê", "e",
		"à", "a",
		"á", "a",
		"ç", "c",
		"ï", "i",
		"î", "i",
		"ô", "o",
		"ù", "u",
	)
	value = replacer.Replace(value)
	var builder strings.Builder
	builder.Grow(len(value))
	lastSpace := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
			lastSpace = false
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastSpace = false
		default:
			if !lastSpace {
				builder.WriteByte(' ')
				lastSpace = true
			}
		}
	}
	return strings.Join(strings.Fields(builder.String()), " ")
}

func stripBuilderStoryPatch(patch *CharacterBuilderPatch) {
	if patch == nil {
		return
	}
	patch.Background = nil
	if patch.Metadata == nil {
		return
	}
	delete(patch.Metadata, "concept")
	delete(patch.Metadata, "backstory")
	delete(patch.Metadata, "personality_traits")
	delete(patch.Metadata, "ideals")
	delete(patch.Metadata, "bonds")
	delete(patch.Metadata, "flaws")
}

func applyBuilderStoryTransferPatch(patch *CharacterBuilderPatch, storyText string) {
	if patch == nil {
		return
	}
	storyText = strings.TrimSpace(storyText)
	if !builderReplyLooksLikeStoryDraft(storyText) {
		return
	}
	if patch.Metadata == nil {
		patch.Metadata = map[string]any{}
	}
	if value, ok := patch.Metadata["backstory"]; !ok || value == nil || strings.TrimSpace(fmt.Sprint(value)) == "" || strings.TrimSpace(fmt.Sprint(value)) == "<nil>" {
		patch.Metadata["backstory"] = storyText
	}
}

func latestBuilderAssistantReply(messages []CharacterBuilderMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if strings.EqualFold(strings.TrimSpace(messages[i].Role), "assistant") {
			return strings.TrimSpace(messages[i].Content)
		}
	}
	return ""
}

func latestBuilderStoryDraft(messages []CharacterBuilderMessage, fallback string) string {
	for i := len(messages) - 2; i >= 0; i-- {
		message := messages[i]
		if !strings.EqualFold(strings.TrimSpace(message.Role), "assistant") {
			continue
		}
		candidate := strings.TrimSpace(message.Content)
		if candidate == "" {
			continue
		}
		if builderReplyLooksLikeStoryDraft(candidate) {
			return candidate
		}
	}
	fallback = strings.TrimSpace(fallback)
	if builderReplyLooksLikeStoryDraft(fallback) {
		return fallback
	}
	return ""
}

func latestBuilderStoryOptions(messages []CharacterBuilderMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		message := messages[i]
		if !strings.EqualFold(strings.TrimSpace(message.Role), "assistant") {
			continue
		}
		candidate := strings.TrimSpace(message.Content)
		if builderReplyLooksLikeStoryOptions(candidate) {
			return candidate
		}
	}
	return ""
}

func builderShouldOfferStoryProposals(message string, previousAssistant string) bool {
	message = normalizeBuilderIntentText(message)
	if message == "" {
		return false
	}
	if !builderStoryDraftRequested(message) || builderStoryTransferRequested(message, previousAssistant) {
		return false
	}
	if builderReplyLooksLikeStoryOptions(previousAssistant) || builderReplyLooksLikeStoryDraft(previousAssistant) {
		return false
	}
	return strings.Contains(message, "erstell") ||
		strings.Contains(message, "schreib") ||
		strings.Contains(message, "formulier") ||
		strings.Contains(message, "mach") ||
		strings.Contains(message, "entwick")
}

func builderShouldWriteFullStory(message string, previousAssistant string) bool {
	message = normalizeBuilderIntentText(message)
	if message == "" || builderStoryTransferRequested(message, previousAssistant) {
		return false
	}
	if builderReplyLooksLikeStoryDraft(previousAssistant) {
		return false
	}
	if strings.Contains(message, "vorschlag 1") ||
		strings.Contains(message, "vorschlag 2") ||
		strings.Contains(message, "vorschlag 3") ||
		strings.Contains(message, "nummer 1") ||
		strings.Contains(message, "nummer 2") ||
		strings.Contains(message, "nummer 3") {
		return true
	}
	selectionMarkers := []string{
		"nimm den ersten",
		"nimm den zweiten",
		"nimm den dritten",
		"ich nehme",
		"ich waehle",
		"ich wähle",
		"den ersten",
		"den zweiten",
		"den dritten",
		"diese variante",
		"die variante",
	}
	for _, marker := range selectionMarkers {
		if strings.Contains(message, marker) && builderReplyLooksLikeStoryOptions(previousAssistant) {
			return true
		}
	}
	if builderSelectedStoryOptionIndex(message, previousAssistant) > 0 {
		return true
	}
	return false
}

func builderSelectedStoryOptionIndex(message string, previousAssistant string) int {
	message = normalizeBuilderIntentText(message)
	if message == "" || !builderReplyLooksLikeStoryOptions(previousAssistant) {
		return 0
	}
	firstMarkers := []string{
		"erste",
		"ersten",
		"option 1",
		"richtung 1",
		"vorschlag 1",
		"nummer 1",
		"1",
	}
	secondMarkers := []string{
		"zweite",
		"zweiten",
		"option 2",
		"richtung 2",
		"vorschlag 2",
		"nummer 2",
		"2",
	}
	thirdMarkers := []string{
		"dritte",
		"dritten",
		"option 3",
		"richtung 3",
		"vorschlag 3",
		"nummer 3",
		"3",
	}
	if containsAnyBuilderMarker(message, firstMarkers) {
		return 1
	}
	if containsAnyBuilderMarker(message, secondMarkers) {
		return 2
	}
	if containsAnyBuilderMarker(message, thirdMarkers) {
		return 3
	}
	return 0
}

func containsAnyBuilderMarker(message string, markers []string) bool {
	for _, marker := range markers {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func builderReplyLooksLikeStoryOptions(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	lower := strings.ToLower(text)
	if strings.Contains(lower, "1.") && strings.Contains(lower, "2.") && strings.Contains(lower, "3.") {
		return true
	}
	if strings.Contains(lower, "vorschlag 1") && strings.Contains(lower, "vorschlag 2") {
		return true
	}
	return false
}

func builderReplyLooksLikeStoryDraft(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	if builderReplyLooksLikeStoryOptions(text) {
		return false
	}
	lower := strings.ToLower(text)
	metaMarkers := []string{
		"ich habe",
		"entwurf vorbereitet",
		"als offiziellen entwurf gespeichert",
		"wir sind nun bereit",
		"soll ich diesen entwurf",
		"wie soll",
		"damit wir",
		"welche richtung willst du nehmen",
	}
	for _, marker := range metaMarkers {
		if strings.Contains(lower, marker) {
			return false
		}
	}
	sentenceCount := len(regexp.MustCompile(`[.!?]+`).FindAllString(text, -1))
	if sentenceCount < 4 {
		return false
	}
	wordCount := len(strings.Fields(text))
	return wordCount >= 70
}

func builderStoryProposalFallback(character *Character) string {
	name := strings.TrimSpace(character.Name)
	if name == "" {
		name = "dein Charakter"
	}
	return fmt.Sprintf(
		"Ich mache dir zuerst drei Richtungen fuer %s.\n\n1. Pflichtruf des Zirkels: %s verlaesst den heiligen Hain, weil ein altes Gleichgewicht kippt und nur ein verlorenes Naturzeichen sein Volk retten kann.\n\n2. Spur des Verderbens: Eine fremde, dunkle Macht frisst sich in die Heimat von %s, und die Suche nach ihrem Ursprung fuehrt hinaus in die Zivilisation.\n\n3. Suche nach dem letzten Heilmittel: %s wurde ausgesandt, um ein seltenes Wesen, Kraut oder Artefakt zu finden, bevor der Schutz seines Volkes endgueltig zusammenbricht.\n\nWelche Richtung willst du nehmen?",
		name,
		name,
		name,
		name,
	)
}

func builderDeterministicStorySelectionCompletion(character *Character, latestUserMessage string, storyOptions string) (characterBuilderCompletion, bool) {
	optionIndex := builderSelectedStoryOptionIndex(latestUserMessage, storyOptions)
	if optionIndex == 0 {
		return characterBuilderCompletion{}, false
	}
	message := normalizeBuilderIntentText(latestUserMessage)
	if !strings.Contains(message, "ausschm") &&
		!strings.Contains(message, "ausformulier") &&
		!strings.Contains(message, "schreib") &&
		!strings.Contains(message, "erzaehl") &&
		!strings.Contains(message, "uebernimm") &&
		!strings.Contains(message, "uebernehm") &&
		!strings.Contains(message, "eintrag") &&
		!strings.Contains(message, "speicher") {
		return characterBuilderCompletion{}, false
	}

	reply := builderStoryDraftForOption(character, optionIndex)
	completion := characterBuilderCompletion{
		Reply: reply,
		Patch: CharacterBuilderPatch{},
	}
	if builderStoryTransferRequested(latestUserMessage, storyOptions) {
		completion.Patch.Metadata = map[string]any{
			"backstory": reply,
		}
	}
	return completion, true
}

func builderStoryDraftForOption(character *Character, optionIndex int) string {
	switch optionIndex {
	case 2:
		return builderStoryDraftFallback(character, "Spur des Verderbens")
	case 3:
		name := strings.TrimSpace(character.Name)
		if name == "" {
			name = "Der Charakter"
		}
		return fmt.Sprintf("%s stammt aus einem alten Druidenzirkel, der seit Jahrhunderten über eine abgelegene Landschaft wacht. Lange Zeit lebte sein Volk im Einklang mit den Zyklen der Natur und schützte verborgene Orte vor fremden Eingriffen. Doch in den letzten Jahren schwand die Kraft, die diese Heimat zusammenhielt. Heilige Quellen wurden schwächer, Schutzzeichen verblassten und selbst erfahrene Hüter fanden keine Antwort mehr in den bekannten Riten. Schließlich entdeckten die Ältesten Hinweise auf ein seltenes Heilmittel, das nicht nur Wunden schließt, sondern auch zerbrechende Lebenslinien eines ganzen Landes stärken kann. Dieses Heilmittel sollte in einer fernen Region verborgen sein, bewacht von Gefahren, die kaum jemand aus dem Zirkel je gesehen hat. Weil %s Geduld, Pflichtgefühl und einen wachen Geist besitzt, fiel die Wahl auf ihn. Er wurde ausgesandt, bevor die letzte Schutzschicht seiner Heimat vollends zerbrach. Für %s ist die Reise deshalb keine Suche nach Ruhm, sondern ein Auftrag, von dem das Überleben vieler abhängt. Unterwegs muss er lernen, dass alte Legenden oft nur Bruchstücke einer größeren Wahrheit enthalten. Jede Pflanze, jedes Wesen und jedes uralte Artefakt könnte Teil der Rettung sein, die sein Volk so dringend braucht. Gleichzeitig wächst der Druck mit jedem Tag, an dem er ohne Antwort bleibt. Er trägt die Hoffnung seines Zirkels mit sich, aber auch die Angst, mit leeren Händen heimzukehren. Gerade diese Last zwingt ihn, weiterzugehen, selbst wenn Zweifel, Müdigkeit und Gefahr übermächtig erscheinen. So ist seine Geschichte die eines Suchenden, der für das Wohl seiner Heimat bereit ist, alles Vertraute hinter sich zu lassen.", name, name, name)
	default:
		name := strings.TrimSpace(character.Name)
		if name == "" {
			name = "Der Charakter"
		}
		return fmt.Sprintf("%s wuchs in einem verborgenen Hain auf, den sein Druidenzirkel seit Generationen bewacht. Dort lernte er, dass das Gleichgewicht der Natur nicht selbstverständlich ist, sondern Tag für Tag geschützt werden muss. Vor einiger Zeit bemerkten die Ältesten jedoch beunruhigende Zeichen. Tiere verhielten sich unruhig, heilige Wasserstellen verloren an Klarheit und uralte Schutzmale reagierten nur noch schwach auf die Riten des Zirkels. In den ältesten Überlieferungen fanden sich Hinweise auf ein verlorenes Naturzeichen, ein Relikt aus früheren Zeiten, das verwundete Lebensräume stärken und bedrohte Grenzen erneut festigen kann. Niemand wusste sicher, wo es verborgen liegt, doch mehrere Zeichen deuteten darauf hin, dass es noch existiert. Weil %s sich als besonnen, pflichtbewusst und aufmerksam erwiesen hatte, wurde gerade er mit dieser Suche betraut. Er verließ den heiligen Hain nicht, um Ruhm zu finden, sondern weil sein Volk jemanden brauchte, der die Last dieser Aufgabe tragen konnte. Für ihn ist jeder Schritt durch die Fremde mit dem Gedanken verbunden, dass seine Heimat während seiner Abwesenheit weiter an Halt verlieren könnte. Diese Verantwortung macht ihn ernst und vorsichtig, aber sie gibt ihm auch eine klare Richtung. Er untersucht alte Ruinen, hört auf Legenden und betrachtet selbst unscheinbare Spuren als möglichen Hinweis auf das verlorene Naturzeichen. Gleichzeitig muss er lernen, Menschen und Orte außerhalb des Zirkels zu verstehen, deren Werte sich von den Lehren seiner Heimat unterscheiden. Immer wieder fragt er sich, ob er der Aufgabe wirklich gewachsen ist. Doch gerade dieser Zweifel hält ihn wachsam und verhindert, dass er leichtsinnig wird. So trägt %s nicht nur die Hoffnung seines Zirkels in die Welt hinaus, sondern auch den festen Willen, mit einer Antwort zurückzukehren, bevor das Gleichgewicht seiner Heimat endgültig zerbricht.", name, name, name)
	}
}

func builderDeterministicGuidedChoiceCompletion(character *Character, stage string, latestUserMessage string, previousAssistant string) (characterBuilderCompletion, bool) {
	if reply, patch, ok := builderDeterministicSkillRepairReply(*character, stage, latestUserMessage, previousAssistant); ok {
		return characterBuilderCompletion{Reply: reply, Patch: patch}, true
	}
	if reply, ok := builderDeterministicStageAdviceReply(*character, stage, latestUserMessage); ok {
		return characterBuilderCompletion{Reply: reply, Patch: CharacterBuilderPatch{}}, true
	}
	switch normalizeBuilderStage(stage) {
	case "ability_scores":
		if reply, patch, ok := builderDeterministicHalfElfAbilityReply(character, latestUserMessage); ok {
			return characterBuilderCompletion{Reply: reply, Patch: patch}, true
		}
	case "class_proficiencies_and_choices":
		if reply, patch, ok := builderDeterministicSkillChoiceReply(*character, latestUserMessage, previousAssistant); ok {
			return characterBuilderCompletion{Reply: reply, Patch: patch}, true
		}
		if builderQuestionLooksLikeListRequest(latestUserMessage) || strings.Contains(normalizeBuilderIntentText(latestUserMessage), "fertigkeit") {
			if reply, ok := builderDeterministicClassChoicesReply(*character); ok {
				return characterBuilderCompletion{Reply: reply, Patch: CharacterBuilderPatch{}}, true
			}
		}
	case "background_and_alignment":
		if reply, patch, ok := builderDeterministicBackgroundChoiceReply(*character, latestUserMessage); ok {
			return characterBuilderCompletion{Reply: reply, Patch: patch}, true
		}
		if reply, ok := builderDeterministicBackgroundReply(*character, latestUserMessage); ok {
			return characterBuilderCompletion{Reply: reply, Patch: CharacterBuilderPatch{}}, true
		}
	case "hit_points_hit_dice_and_movement":
		if reply, patch, ok := builderDeterministicPostHitPointsReply(*character, latestUserMessage); ok {
			return characterBuilderCompletion{Reply: reply, Patch: patch}, true
		}
	case "languages_senses_and_body":
		if reply, ok := builderDeterministicLanguageReply(*character, latestUserMessage); ok {
			return characterBuilderCompletion{Reply: reply, Patch: CharacterBuilderPatch{}}, true
		}
		if reply, patch, ok := builderDeterministicSensesAndBodyReply(*character, latestUserMessage); ok {
			return characterBuilderCompletion{Reply: reply, Patch: patch}, true
		}
	case "equipment_and_money":
		if reply, patch, ok := builderDeterministicEquipmentReply(*character, latestUserMessage, previousAssistant); ok {
			return characterBuilderCompletion{Reply: reply, Patch: patch}, true
		}
	case "class_features_not_spells":
		if reply, patch, ok := builderDeterministicClassFeatureReply(*character, latestUserMessage, previousAssistant); ok {
			return characterBuilderCompletion{Reply: reply, Patch: patch}, true
		}
	case "spellcasting_if_available":
		if reply, patch, ok := builderDeterministicSpellcastingReply(*character, latestUserMessage, previousAssistant); ok {
			return characterBuilderCompletion{Reply: reply, Patch: patch}, true
		}
	case "derived_stats":
		if reply, patch, ok := builderDeterministicDerivedStatsReply(*character, latestUserMessage); ok {
			return characterBuilderCompletion{Reply: reply, Patch: patch}, true
		}
	case "combat":
		if reply, patch, ok := builderDeterministicCombatReply(*character, latestUserMessage, previousAssistant); ok {
			return characterBuilderCompletion{Reply: reply, Patch: patch}, true
		}
	case "review":
		if reply, patch, ok := builderDeterministicReviewReply(*character, latestUserMessage); ok {
			return characterBuilderCompletion{Reply: reply, Patch: patch}, true
		}
	}
	return characterBuilderCompletion{}, false
}

func builderDeterministicStageAdviceReply(character Character, stage string, latestUserMessage string) (string, bool) {
	message := normalizeBuilderIntentText(latestUserMessage)
	if !builderQuestionLooksLikeListRequest(message) && !builderQuestionLooksLikeRecommendationRequest(message) {
		return "", false
	}
	switch normalizeBuilderStage(stage) {
	case "race":
		return builderDeterministicRaceAdviceReply(character, message)
	case "class_and_level":
		return builderDeterministicClassStageAdviceReply(character, message)
	case "background_and_alignment":
		return builderDeterministicBackgroundReply(character, latestUserMessage)
	case "ability_method":
		return "Für die Attributsmethode hast du drei offizielle Wege: Standardwerte, Point Buy oder Würfeln. Für einen stabilen, gut vergleichbaren Start empfehle ich Standardwerte; für mehr Feintuning Point Buy; für mehr Zufall Würfeln. Wähle jetzt Standardwerte, Point Buy oder Würfeln.", true
	case "class_proficiencies_and_choices":
		return builderDeterministicClassChoicesReply(character)
	case "languages_senses_and_body":
		return builderDeterministicLanguageReply(character, latestUserMessage)
	case "personality":
		return "Jetzt geht es um Persönlichkeit. Sinnvoll sind jeweils kurze Entscheidungen für Merkmal, Ideal, Bindung und Makel. Für einen disziplinierten Ritter würden zum Beispiel Pflichtbewusstsein, Ehre, Treue zum Lehnsherrn und Stolz gut passen. Nenne jetzt diese vier Punkte oder bitte mich um drei passende Vorschlagssets.", true
	case "equipment_and_money":
		return builderDeterministicEquipmentAdviceReply(character, message)
	case "class_features_not_spells":
		return builderDeterministicClassFeatureAdviceReply(character, message)
	case "spellcasting_if_available":
		return builderDeterministicSpellcastingAdviceReply(character, message)
	case "derived_stats":
		return builderDeterministicDerivedStatsAdviceReply(character, message)
	case "combat":
		return builderDeterministicCombatAdviceReply(character, message)
	default:
		return "", false
	}
}

func builderDeterministicHalfElfAbilityReply(character *Character, latestUserMessage string) (string, CharacterBuilderPatch, bool) {
	if !builderHasPendingRaceChoice(*character) {
		return "", CharacterBuilderPatch{}, false
	}
	message := normalizeBuilderIntentText(latestUserMessage)
	choices := parseAbilityChoiceKeys(message)
	if len(choices) >= 2 {
		reply := fmt.Sprintf("Aktuell ist in der Attributsphase noch die Halbelf-Regel offen. Ich habe die +1-Boni jetzt auf %s und %s festgehalten. Als Nächstes prüfe ich damit die finalen Attributswerte gegen den aktuellen Stand.", abilityKeyLabel(choices[0]), abilityKeyLabel(choices[1]))
		return reply, CharacterBuilderPatch{
			Metadata: map[string]any{
				"race_bonus_choices": choices[:2],
			},
		}, true
	}
	return builderAbilityStageGuidanceReply(*character), CharacterBuilderPatch{}, true
}

func builderAbilityStageGuidanceReply(character Character) string {
	if builderHasPendingRaceChoice(character) {
		return "In der Attributsphase ist noch die Halbelf-Regel offen. Für Halbelf gilt +2 auf Charisma und +1 auf zwei weitere verschiedene Attribute nach Wahl. Nenne jetzt genau die zwei Attribute für die beiden +1-Boni."
	}
	return "Jetzt werden die Attribute abgeschlossen. Nenne die noch offenen Entscheidungen oder die fertige Verteilung."
}

func builderDeterministicClassChoicesReply(character Character) (string, bool) {
	classRule, ok := builderClassRuleForCharacter(character)
	if !ok {
		return "", false
	}
	raceRule, hasRace := builderRaceRuleForCharacter(character)
	parts := []string{
		"Jetzt sind die Klassenentscheidungen dran.",
		fmt.Sprintf("Als %s sind die Rettungswürfe %s fest.", classRule.ClassName, joinGermanList(classRule.SavingThrows)),
	}
	if classRule.SkillChoiceCount > 0 {
		if len(classRule.SkillChoices) == 1 && strings.EqualFold(classRule.SkillChoices[0], "Beliebige Fertigkeiten") {
			parts = append(parts, fmt.Sprintf("%s-Fertigkeiten: Wähle %d beliebige Fertigkeiten.", classRule.ClassName, classRule.SkillChoiceCount))
		} else {
			parts = append(parts, fmt.Sprintf("%s-Fertigkeiten: Wähle %d aus %s.", classRule.ClassName, classRule.SkillChoiceCount, joinGermanList(classRule.SkillChoices)))
		}
	}
	if hasRace {
		if raceRule.ExtraSkillChoiceCount > 0 {
			parts = append(parts, fmt.Sprintf("Als %s wählst du zusätzlich %d beliebige Fertigkeiten.", raceRule.RaceName, raceRule.ExtraSkillChoiceCount))
		}
		if raceRule.ExtraLanguageChoiceCount > 0 {
			parts = append(parts, fmt.Sprintf("Als %s wählst du zusätzlich %d Sprache nach Wahl.", raceRule.RaceName, raceRule.ExtraLanguageChoiceCount))
		}
	}
	if suggestions := builderSuggestedSkillSets(character, classRule); len(suggestions) > 0 {
		parts = append(parts, fmt.Sprintf("Sinnvolle Vorschläge sind %s.", joinGermanList(suggestions)))
	}
	if classRule.SkillChoiceCount > 0 {
		parts = append(parts, fmt.Sprintf("Lege jetzt zuerst die %d %s-Fertigkeiten fest.", classRule.SkillChoiceCount, classRule.ClassName))
	} else {
		parts = append(parts, "Lege jetzt die noch offenen Klassenentscheidungen fest.")
	}
	return strings.Join(parts, " "), true
}

func builderDeterministicSkillChoiceReply(character Character, latestUserMessage string, previousAssistant string) (string, CharacterBuilderPatch, bool) {
	classRule, ok := builderClassRuleForCharacter(character)
	if !ok {
		return "", CharacterBuilderPatch{}, false
	}
	raceRule, hasRace := builderRaceRuleForCharacter(character)
	metadata := defaultMetadata(character.Metadata)
	backgroundSkills := uniqueCanonicalSkills(stringListFromAny(metadata["background_skill_proficiencies"]))
	selectedClassSkills := uniqueCanonicalSkills(stringListFromAny(metadata["class_skill_proficiencies"]))
	legacyCombinedSkills := uniqueCanonicalSkills(stringListFromAny(metadata["skill_proficiencies"]))
	if len(backgroundSkills) == 0 && len(selectedClassSkills) == 0 && len(legacyCombinedSkills) > 0 {
		backgroundSkills = legacyCombinedSkills
	}

	parsedChoices := parseSkillChoicesFromMessage(latestUserMessage)
	if len(parsedChoices) == 0 && builderMessageConfirmsPreviousSuggestion(latestUserMessage) {
		parsedChoices = builderSuggestedSkillsFromAssistantReply(previousAssistant)
	}
	if len(parsedChoices) == 0 {
		return "", CharacterBuilderPatch{}, false
	}
	lockedSkillSet := make(map[string]bool, len(backgroundSkills))
	for _, skill := range backgroundSkills {
		lockedSkillSet[skill] = true
	}
	newClassChoices := make([]string, 0, len(parsedChoices))
	for _, skill := range parsedChoices {
		if lockedSkillSet[skill] {
			continue
		}
		newClassChoices = append(newClassChoices, skill)
	}
	selected := uniqueCanonicalSkills(append(selectedClassSkills, newClassChoices...))

	required := classRule.SkillChoiceCount
	if hasRace {
		required += raceRule.ExtraSkillChoiceCount
	}
	if required <= 0 {
		return "", CharacterBuilderPatch{}, false
	}
	if len(selected) > required {
		selected = selected[:required]
	}
	allSkills := uniqueCanonicalSkills(append(backgroundSkills, selected...))

	patch := CharacterBuilderPatch{
		Metadata: map[string]any{
			"class_skill_proficiencies":  selected,
			"skill_proficiencies":        allSkills,
			"saving_throw_proficiencies": classRule.SavingThrows,
		},
	}

	if len(selected) >= required {
		patch.Metadata["builder_stage"] = "hit_points_hit_dice_and_movement"
		reply := fmt.Sprintf("Die Fertigkeiten sind jetzt festgelegt: %s. Rettungswürfe %s sind damit ebenfalls gesetzt. Als Nächstes leite ich Trefferpunkte, Trefferwürfel und Bewegungsrate aus Klasse, Volk und Attributen ab.", joinGermanList(allSkills), joinGermanList(classRule.SavingThrows))
		return reply, patch, true
	}

	if len(selected) < classRule.SkillChoiceCount {
		remaining := classRule.SkillChoiceCount - len(selected)
		known := allSkills
		if len(known) == 0 {
			known = selected
		}
		reply := fmt.Sprintf("Bisher festgelegt: %s. Für %s fehlen noch %d Klassen-Fertigkeiten aus %s. Nenne jetzt die verbleibenden Fertigkeiten.", joinGermanList(known), classRule.ClassName, remaining, joinGermanList(classRule.SkillChoices))
		return reply, patch, true
	}

	extraRemaining := required - len(selected)
	if hasRace && raceRule.ExtraSkillChoiceCount > 0 && extraRemaining > 0 {
		classSkills := selected
		if len(classSkills) > classRule.SkillChoiceCount {
			classSkills = classSkills[:classRule.SkillChoiceCount]
		}
		reply := fmt.Sprintf("Die %s-Fertigkeiten sind festgelegt: %s. Als %s fehlen noch %d zusätzliche Fertigkeiten nach Wahl. Nenne jetzt diese Fertigkeiten.", classRule.ClassName, joinGermanList(classSkills), raceRule.RaceName, extraRemaining)
		return reply, patch, true
	}

	return "", CharacterBuilderPatch{}, false
}

func builderDeterministicBackgroundReply(character Character, latestUserMessage string) (string, bool) {
	message := normalizeBuilderIntentText(latestUserMessage)
	if !builderMentionsRulesBackgroundTopic(message) {
		return "", false
	}
	if builderQuestionLooksLikeListRequest(message) || builderQuestionLooksLikeRecommendationRequest(message) {
		allSkills := builderCanonicalSkillNames()
		suggestions := builderBackgroundSuggestions(message, character)
		parts := []string{
			"Das SRD 5.1 enthält als einzigen benannten Musterhintergrund Akolyth.",
			"Derselbe offizielle Regelrahmen erlaubt aber ausdrücklich einen eigenen Hintergrund mit eigenem Namen.",
			fmt.Sprintf("Für den eigenen Hintergrund wählst du genau zwei Fertigkeiten aus %s.", joinGermanList(allSkills)),
			"Dazu wählst du insgesamt noch zwei Sprachen oder Werkzeugübungen.",
		}
		if len(suggestions) > 0 {
			parts = append(parts, fmt.Sprintf("Für deinen beschriebenen Hintergrund passen besonders gut %s.", joinGermanList(suggestions)))
		}
		parts = append(parts, "Lege jetzt den Hintergrundnamen fest und nenne danach die zwei Fertigkeiten.")
		return strings.Join(parts, " "), true
	}
	return "", false
}

func builderDeterministicBackgroundChoiceReply(character Character, latestUserMessage string) (string, CharacterBuilderPatch, bool) {
	backgroundName := strings.TrimSpace(character.Background)
	if backgroundName == "" {
		backgroundName = builderBackgroundNameFromMessage(latestUserMessage)
	}
	skills := parseSkillChoicesFromMessage(latestUserMessage)
	if backgroundName == "" || len(skills) < 2 {
		return "", CharacterBuilderPatch{}, false
	}
	selected := uniqueCanonicalSkills(skills[:2])
	patch := CharacterBuilderPatch{
		Background: &backgroundName,
		Metadata: map[string]any{
			"background_skill_proficiencies": selected,
			"skill_proficiencies":            selected,
		},
	}
	reply := fmt.Sprintf("Der Hintergrund %s ist festgelegt. Die Hintergrund-Fertigkeiten sind jetzt als %s eingetragen. Wähle jetzt insgesamt noch zwei Sprachen oder Werkzeugübungen für den Hintergrund; danach legen wir die Gesinnung fest.", backgroundName, joinGermanList(selected))
	return reply, patch, true
}

func builderDeterministicRaceAdviceReply(character Character, message string) (string, bool) {
	options := make([]string, 0, len(builderCoreRaceRules))
	for _, rule := range builderCoreRaceRules {
		options = append(options, rule.RaceName)
	}
	parts := []string{
		fmt.Sprintf("Verfügbare Völker im aktuellen SRD-Profil sind %s.", joinGermanList(options)),
	}
	if suggestions := builderRaceSuggestions(character, message); len(suggestions) > 0 {
		parts = append(parts, fmt.Sprintf("Sinnvolle Vorschläge sind %s.", joinGermanList(suggestions)))
	}
	parts = append(parts, "Wähle jetzt genau ein Volk.")
	return strings.Join(parts, " "), true
}

func builderDeterministicClassStageAdviceReply(character Character, message string) (string, bool) {
	options := make([]string, 0, len(builderCoreClassRules))
	for _, rule := range builderCoreClassRules {
		options = append(options, rule.ClassName)
	}
	parts := []string{
		fmt.Sprintf("Verfügbare Klassen im aktuellen Profil sind %s.", joinGermanList(options)),
		"Für den Einstieg ist normalerweise Stufe 1 sinnvoll.",
	}
	if suggestions := builderClassSuggestions(character, message); len(suggestions) > 0 {
		parts = append(parts, fmt.Sprintf("Sinnvolle Vorschläge sind %s.", joinGermanList(suggestions)))
	}
	parts = append(parts, "Nenne jetzt Klasse und Stufe zusammen, zum Beispiel „Kämpfer, Stufe 1“.")
	return strings.Join(parts, " "), true
}

func builderDeterministicLanguageReply(character Character, latestUserMessage string) (string, bool) {
	message := normalizeBuilderIntentText(latestUserMessage)
	if !strings.Contains(message, "sprache") && !strings.Contains(message, "sprachen") && !strings.Contains(message, "language") {
		return "", false
	}
	raceRule, ok := builderRaceRuleForCharacter(character)
	if !ok {
		return "", false
	}
	parts := []string{"Jetzt sind Sprachen und Sinne dran."}
	if len(raceRule.FixedLanguages) > 0 {
		parts = append(parts, fmt.Sprintf("Als %s sprichst du fest %s.", raceRule.RaceName, joinGermanList(raceRule.FixedLanguages)))
	}
	if raceRule.ExtraLanguageChoiceCount > 0 {
		parts = append(parts, fmt.Sprintf("Dazu wählst du %d weitere Sprache nach Wahl.", raceRule.ExtraLanguageChoiceCount))
		parts = append(parts, "Lege jetzt diese zusätzliche Sprache fest.")
	} else {
		parts = append(parts, "Lege jetzt nur noch die offenen Sinne oder Körperdaten fest.")
	}
	return strings.Join(parts, " "), true
}

func builderDeterministicEquipmentReply(character Character, latestUserMessage string, previousAssistant string) (string, CharacterBuilderPatch, bool) {
	advice, ok := builderEquipmentAdviceForCharacter(character)
	if !ok {
		return "", CharacterBuilderPatch{}, false
	}
	message := normalizeBuilderIntentText(latestUserMessage)
	if builderQuestionLooksLikeListRequest(message) || builderQuestionLooksLikeRecommendationRequest(message) {
		reply, ok := builderDeterministicEquipmentAdviceReply(character, latestUserMessage)
		return reply, CharacterBuilderPatch{}, ok
	}
	if builderMessageConfirmsPreviousSuggestion(latestUserMessage) || strings.Contains(message, "empfehl") || strings.Contains(message, "standard") {
		if !strings.Contains(normalizeBuilderIntentText(previousAssistant), "kettenhemd") &&
			!strings.Contains(normalizeBuilderIntentText(previousAssistant), "chain mail") &&
			!strings.Contains(normalizeBuilderIntentText(previousAssistant), "martial weapon") &&
			!strings.Contains(normalizeBuilderIntentText(previousAssistant), "langschwert") {
			return "", CharacterBuilderPatch{}, false
		}
		patch := CharacterBuilderPatch{
			Metadata: map[string]any{
				"starting_equipment": advice.Recommendation,
				"current_inventory":  advice.Recommendation,
				"weapon_notes":       advice.WeaponNotes,
				"combat_overview":    advice.CombatOverview,
			},
		}
		reply := fmt.Sprintf("Ich nehme den Standardvorschlag: %s. Für deinen eigenen Hintergrund „Ritter“ ist im SRD kein festes Ausrüstungspaket definiert; das ergänzen wir bei Bedarf separat oder kaufen später mit Geld dazu. Als Nächstes halte ich die Klassenmerkmale der 1. Stufe fest.", joinGermanList(advice.Recommendation))
		return reply, patch, true
	}
	return "", CharacterBuilderPatch{}, false
}

func builderDeterministicEquipmentAdviceReply(character Character, latestUserMessage string) (string, bool) {
	advice, ok := builderEquipmentAdviceForCharacter(character)
	if !ok {
		return "", false
	}
	parts := []string{
		fmt.Sprintf("Für %s auf Stufe 1 stehen im SRD 5.1 diese Startausrüstungsoptionen zur Verfügung: %s.", classNameOrFallback(character), joinGermanList(advice.Options)),
	}
	if len(advice.Recommendation) > 0 {
		parts = append(parts, fmt.Sprintf("Für deinen arkanen Klingenkämpfer empfehle ich als Standardauswahl %s.", joinGermanList(advice.Recommendation)))
	}
	parts = append(parts, "Wenn du möchtest, übernehme ich genau diese Auswahl direkt.")
	return strings.Join(parts, " "), true
}

func builderDeterministicClassFeatureReply(character Character, latestUserMessage string, previousAssistant string) (string, CharacterBuilderPatch, bool) {
	advice, ok := builderFeatureAdviceForCharacter(character)
	if !ok {
		return "", CharacterBuilderPatch{}, false
	}
	message := normalizeBuilderIntentText(latestUserMessage)
	if builderQuestionLooksLikeListRequest(message) || builderQuestionLooksLikeRecommendationRequest(message) {
		reply, ok := builderDeterministicClassFeatureAdviceReply(character, latestUserMessage)
		return reply, CharacterBuilderPatch{}, ok
	}
	if builderMessageConfirmsPreviousSuggestion(latestUserMessage) || strings.Contains(message, "übernimm") || strings.Contains(message, "nimm die") {
		patch := CharacterBuilderPatch{
			Features: advice.Recommendation,
			Metadata: map[string]any{
				"class_features": advice.Recommendation,
			},
		}
		reply := fmt.Sprintf("Ich übernehme die empfohlenen Klassenmerkmale: %s.", joinGermanList(advice.Recommendation))
		if builderHasLevelOneSpellcasting(character) {
			reply += " Als Nächstes gehen wir die Zauberoptionen durch."
		} else {
			reply += " Als Nächstes ziehe ich die abgeleiteten Werte wie RK und Übungsbonus gerade."
		}
		return reply, patch, true
	}
	if strings.TrimSpace(previousAssistant) != "" && builderMessageConfirmsPreviousSuggestion(latestUserMessage) {
		patch := CharacterBuilderPatch{Features: advice.Recommendation}
		return fmt.Sprintf("Ich trage jetzt %s als Klassenmerkmale ein.", joinGermanList(advice.Recommendation)), patch, true
	}
	return "", CharacterBuilderPatch{}, false
}

func builderDeterministicClassFeatureAdviceReply(character Character, latestUserMessage string) (string, bool) {
	advice, ok := builderFeatureAdviceForCharacter(character)
	if !ok {
		return "", false
	}
	parts := []string{
		fmt.Sprintf("Auf Stufe 1 sind für %s im SRD 5.1 diese Klassenmerkmale relevant: %s.", classNameOrFallback(character), joinGermanList(advice.Options)),
	}
	if len(advice.Recommendation) > 0 {
		parts = append(parts, fmt.Sprintf("Für diesen Build empfehle ich %s.", joinGermanList(advice.Recommendation)))
	}
	parts = append(parts, "Wenn du möchtest, übernehme ich genau diese Auswahl direkt.")
	return strings.Join(parts, " "), true
}

func builderDeterministicSpellcastingReply(character Character, latestUserMessage string, previousAssistant string) (string, CharacterBuilderPatch, bool) {
	advice, ok := builderSpellAdviceForCharacter(character)
	if !ok {
		return "", CharacterBuilderPatch{}, false
	}
	message := normalizeBuilderIntentText(latestUserMessage)
	if builderQuestionLooksLikeListRequest(message) || builderQuestionLooksLikeRecommendationRequest(message) {
		reply, ok := builderDeterministicSpellcastingAdviceReply(character, latestUserMessage)
		return reply, CharacterBuilderPatch{}, ok
	}
	if builderMessageConfirmsPreviousSuggestion(latestUserMessage) || strings.Contains(message, "übernimm") {
		patch := CharacterBuilderPatch{
			Metadata: map[string]any{
				"spells":            advice.Recommendation,
				"spell_notes":       advice.SpellNotes,
				"spell_attacks":     advice.SpellAttacks,
				"spell_save_dc":     advice.SpellSaveDC,
				"spell_attack_bonus": advice.SpellAttackBonus,
			},
		}
		reply := fmt.Sprintf("Ich übernehme die empfohlenen Zauber: %s. Damit kann ich im nächsten Schritt die abgeleiteten Werte direkt festziehen.", joinGermanList(advice.Recommendation))
		return reply, patch, true
	}
	_ = previousAssistant
	return "", CharacterBuilderPatch{}, false
}

func builderDeterministicSpellcastingAdviceReply(character Character, latestUserMessage string) (string, bool) {
	advice, ok := builderSpellAdviceForCharacter(character)
	if !ok {
		return "", false
	}
	parts := []string{
		fmt.Sprintf("Für %s auf Stufe 1 stehen im SRD 5.1 diese sinnvollen Zauberoptionen bereit: %s.", classNameOrFallback(character), joinGermanList(advice.Options)),
		fmt.Sprintf("Empfohlene Standardauswahl: %s.", joinGermanList(advice.Recommendation)),
		advice.SpellNotes,
	}
	if len(advice.SpellAttacks) > 0 {
		parts = append(parts, fmt.Sprintf("Wichtige Zauberangriffe oder SG-Werte: %s.", joinGermanList(advice.SpellAttacks)))
	}
	parts = append(parts, "Wenn du möchtest, übernehme ich genau diese Auswahl direkt.")
	return strings.Join(parts, " "), true
}

func builderDeterministicDerivedStatsReply(character Character, latestUserMessage string) (string, CharacterBuilderPatch, bool) {
	if !builderLooksLikeContinueIntent(normalizeBuilderIntentText(latestUserMessage)) &&
		!builderQuestionLooksLikeListRequest(latestUserMessage) &&
		!builderQuestionLooksLikeRecommendationRequest(latestUserMessage) {
		return "", CharacterBuilderPatch{}, false
	}
	ac := builderDerivedArmorClass(character)
	prof := deriveCharacterProficiencyBonus(character)
	hitPointMax := builderDerivedHitPointMax(character)
	speed := builderDerivedSpeed(character)
	patch := CharacterBuilderPatch{
		ArmorClass:  &ac,
		HitPointMax: &hitPointMax,
		Speed:       &speed,
	}
	if prof > 0 {
		profStr := fmt.Sprintf("+%d", prof)
		patch.Proficiency = &profStr
	}
	if patch.Metadata == nil {
		patch.Metadata = map[string]any{}
	}
	patch.Metadata["builder_stage"] = "combat"
	if spellAdvice, ok := builderSpellAdviceForCharacter(character); ok {
		patch.Metadata["spell_save_dc"] = spellAdvice.SpellSaveDC
		patch.Metadata["spell_attack_bonus"] = spellAdvice.SpellAttackBonus
	}
	reply := fmt.Sprintf("Die abgeleiteten Werte sind jetzt fest: Rüstungsklasse %d, Trefferpunkte %d, Bewegungsrate %s und Übungsbonus +%d.", ac, hitPointMax, speed, prof)
	if spellAdvice, ok := builderSpellAdviceForCharacter(character); ok && spellAdvice.SpellSaveDC != "" && spellAdvice.SpellAttackBonus != "" {
		reply += fmt.Sprintf(" Für Zauber gilt SG %s und Zauberangriffsbonus %s.", spellAdvice.SpellSaveDC, spellAdvice.SpellAttackBonus)
	}
	reply += " Als Nächstes trage ich die konkreten Angriffe und Kampfdaten ein."
	return reply, patch, true
}

func builderDeterministicDerivedStatsAdviceReply(character Character, latestUserMessage string) (string, bool) {
	ac := builderDerivedArmorClass(character)
	prof := deriveCharacterProficiencyBonus(character)
	parts := []string{
		fmt.Sprintf("Aus Klasse, Attributen und empfohlener Ausrüstung ergeben sich aktuell RK %d und Übungsbonus +%d.", ac, prof),
	}
	if spellAdvice, ok := builderSpellAdviceForCharacter(character); ok {
		parts = append(parts, fmt.Sprintf("Für Zauber ergeben sich aktuell SG %s und Zauberangriffsbonus %s.", spellAdvice.SpellSaveDC, spellAdvice.SpellAttackBonus))
	}
	parts = append(parts, "Wenn du möchtest, übernehme ich diese abgeleiteten Werte jetzt direkt.")
	return strings.Join(parts, " "), true
}

func builderDeterministicCombatReply(character Character, latestUserMessage string, previousAssistant string) (string, CharacterBuilderPatch, bool) {
	message := normalizeBuilderIntentText(latestUserMessage)
	if builderQuestionLooksLikeListRequest(message) || builderQuestionLooksLikeRecommendationRequest(message) {
		reply, ok := builderDeterministicCombatAdviceReply(character, latestUserMessage)
		return reply, CharacterBuilderPatch{}, ok
	}
	if builderMessageConfirmsPreviousSuggestion(latestUserMessage) || strings.Contains(message, "übernimm") {
		attacks := builderCombatAttackRecommendations(character)
		attackTable := builderCombatAttackTable(character)
		patch := CharacterBuilderPatch{
			Metadata: map[string]any{
				"combat_attacks": attackTable,
				"builder_stage":  "review",
			},
		}
		if character.ArmorClass == nil || *character.ArmorClass == 0 {
			ac := builderDerivedArmorClass(character)
			patch.ArmorClass = &ac
		}
		if character.HitPointMax == nil || *character.HitPointMax <= 0 {
			hp := builderDerivedHitPointMax(character)
			patch.HitPointMax = &hp
		}
		if strings.TrimSpace(character.Speed) == "" {
			speed := builderDerivedSpeed(character)
			patch.Speed = &speed
		}
		if strings.TrimSpace(character.Proficiency) == "" {
			prof := fmt.Sprintf("+%d", deriveCharacterProficiencyBonus(character))
			patch.Proficiency = &prof
		}
		reply := fmt.Sprintf("Ich übernehme die Kampfdaten jetzt direkt: %s. Der Draft ist damit bereit für die Abschlussprüfung.", joinGermanList(attacks))
		return reply, patch, true
	}
	_ = previousAssistant
	return "", CharacterBuilderPatch{}, false
}

func builderDeterministicCombatAdviceReply(character Character, latestUserMessage string) (string, bool) {
	attacks := builderCombatAttackRecommendations(character)
	if len(attacks) == 0 {
		return "", false
	}
	return fmt.Sprintf("Mit der aktuellen Standardausrüstung ergeben sich diese Kampfdaten: %s. Wenn du möchtest, übernehme ich genau diese Einträge direkt.", joinGermanList(attacks)), true
}

func builderDeterministicReviewReply(character Character, latestUserMessage string) (string, CharacterBuilderPatch, bool) {
	message := normalizeBuilderIntentText(latestUserMessage)
	metadata := defaultMetadata(character.Metadata)
	missingCombatAttacks := strings.TrimSpace(fmt.Sprint(metadata["combat_attacks"])) == ""
	needsRepair := missingCombatAttacks ||
		character.ArmorClass == nil || *character.ArmorClass <= 0 ||
		character.HitPointMax == nil || *character.HitPointMax <= 0 ||
		strings.TrimSpace(character.Speed) == "" ||
		strings.TrimSpace(character.Proficiency) == ""
	if !needsRepair {
		return "", CharacterBuilderPatch{}, false
	}
	if !strings.Contains(message, "angriff") &&
		!strings.Contains(message, "attack") &&
		!strings.Contains(message, "fehlt") &&
		!strings.Contains(message, "nicht") &&
		!strings.Contains(message, "0") &&
		!builderLooksLikeContinueIntent(message) &&
		!builderMessageConfirmsPreviousSuggestion(latestUserMessage) {
		return "", CharacterBuilderPatch{}, false
	}

	patch := CharacterBuilderPatch{
		Metadata: map[string]any{
			"builder_stage": "review",
		},
	}
	fixes := make([]string, 0, 5)
	if missingCombatAttacks {
		if attackTable := builderCombatAttackTable(character); strings.TrimSpace(attackTable) != "" {
			patch.Metadata["combat_attacks"] = attackTable
			fixes = append(fixes, "die Angriffstabelle")
		}
	}
	if character.ArmorClass == nil || *character.ArmorClass <= 0 {
		ac := builderDerivedArmorClass(character)
		patch.ArmorClass = &ac
		fixes = append(fixes, fmt.Sprintf("RK %d", ac))
	}
	if character.HitPointMax == nil || *character.HitPointMax <= 0 {
		hp := builderDerivedHitPointMax(character)
		patch.HitPointMax = &hp
		fixes = append(fixes, fmt.Sprintf("maximale Trefferpunkte %d", hp))
	}
	if strings.TrimSpace(character.Speed) == "" {
		speed := builderDerivedSpeed(character)
		patch.Speed = &speed
		fixes = append(fixes, fmt.Sprintf("Bewegungsrate %s", speed))
	}
	if strings.TrimSpace(character.Proficiency) == "" {
		prof := fmt.Sprintf("+%d", deriveCharacterProficiencyBonus(character))
		patch.Proficiency = &prof
		fixes = append(fixes, fmt.Sprintf("Übungsbonus %s", prof))
	}
	if len(fixes) == 0 {
		return "", CharacterBuilderPatch{}, false
	}
	reply := fmt.Sprintf("Ich habe %s jetzt direkt nachgetragen. Prüfe den Bogen noch einmal; wenn alles passt, kannst du den Draft danach abschließen.", joinGermanList(fixes))
	return reply, patch, true
}

func builderEquipmentAdviceForCharacter(character Character) (builderEquipmentAdvice, bool) {
	classRule, ok := builderClassRuleForCharacter(character)
	if !ok {
		return builderEquipmentAdvice{}, false
	}
	switch classRule.ClassName {
	case "Barbar":
		return builderEquipmentAdvice{
			Options: []string{
				"Waffenwahl: eine Großaxt oder eine beliebige Kriegs-Nahkampfwaffe",
				"Zweitwaffe: zwei Handäxte oder eine beliebige einfache Waffe",
				"Paket: Entdeckerausrüstung",
				"Zusätzlich: vier Wurfspeere",
			},
			Recommendation: []string{
				"Großaxt",
				"zwei Handäxte",
				"Entdeckerausrüstung",
				"vier Wurfspeere",
			},
			WeaponNotes:    "Großaxt für maximalen frühen Nahkampfschaden, Handäxte und Wurfspeere als flexible Ergänzung.",
			CombatOverview: "Klassischer Barbar-Start mit schwerer Zweihandwaffe und soliden Wurfoptionen.",
		}, true
	case "Barde":
		return builderEquipmentAdvice{
			Options: []string{
				"Waffenwahl: Rapier, Langschwert oder eine beliebige einfache Waffe",
				"Paket: Diplomatenausrüstung oder Unterhaltungskünstlerausrüstung",
				"Instrument: Laute oder ein anderes Musikinstrument",
				"Zusätzlich: Lederrüstung und ein Dolch",
			},
			Recommendation: []string{
				"Rapier",
				"Diplomatenausrüstung",
				"Laute",
				"Lederrüstung",
				"Dolch",
			},
			WeaponNotes:    "Rapier für solide Finesse-Nahkampfangriffe, Dolch als Reservewaffe.",
			CombatOverview: "Vielseitiger Barden-Start mit brauchbarer Nahkampfwaffe, Fokus auf soziale und mobile Szenen.",
		}, true
	case "Kleriker":
		return builderEquipmentAdvice{
			Options: []string{
				"Waffenwahl: Streitkolben oder Kriegshammer, falls geübt",
				"Rüstung: Schuppenpanzer, Lederrüstung oder Kettenhemd, falls geübt",
				"Fernkampf: leichte Armbrust mit 20 Bolzen oder eine beliebige einfache Waffe",
				"Paket: Priesterpack oder Entdeckerausrüstung",
				"Zusätzlich: Schild und heiliges Symbol",
			},
			Recommendation: []string{
				"Streitkolben",
				"Schuppenpanzer",
				"leichte Armbrust mit 20 Bolzen",
				"Priesterpack",
				"Schild",
				"heiliges Symbol",
			},
			WeaponNotes:    "Streitkolben und Schild für solide Front, leichte Armbrust für Distanz.",
			CombatOverview: "Defensiver Kleriker-Start mit ordentlicher Rüstung, Schild und sicherer Fernkampfreserve.",
		}, true
	case "Druide":
		return builderEquipmentAdvice{
			Options: []string{
				"Erste Wahl: Holzschild oder eine beliebige einfache Waffe",
				"Zweite Wahl: Krummsäbel oder eine beliebige einfache Nahkampfwaffe",
				"Zusätzlich: Lederrüstung, Entdeckerausrüstung und druidischer Fokus",
			},
			Recommendation: []string{
				"Holzschild",
				"Krummsäbel",
				"Lederrüstung",
				"Entdeckerausrüstung",
				"druidischer Fokus",
			},
			WeaponNotes:    "Krummsäbel für verlässlichen Nahkampf, Holzschild für frühe Defensive.",
			CombatOverview: "Stabiler Druiden-Start mit Fokus auf Schutz, Zauberwirken und einfache Feldtauglichkeit.",
		}, true
	case "Kämpfer":
		return builderEquipmentAdvice{
			Options: []string{
				"Rüstungswahl: Kettenhemd oder Lederüstung, Langbogen und 20 Pfeile",
				"Waffenwahl: eine Kriegswaffe und Schild oder zwei Kriegswaffen",
				"Zusatzwaffen: leichte Armbrust mit 20 Bolzen oder zwei Handäxte",
				"Paket: Verlieserkunderausrüstung oder Entdeckerausrüstung",
			},
			Recommendation: []string{
				"Kettenhemd",
				"Langschwert und Schild",
				"leichte Armbrust mit 20 Bolzen",
				"Entdeckerausrüstung",
			},
			WeaponNotes:    "Langschwert und Schild als Hauptausrüstung, leichte Armbrust als Fernkampfoption.",
			CombatOverview: "Defensiver Kämpfer-Start mit Schild, solider Nahkampfwaffe und Fernkampf-Backup.",
		}, true
	case "Mönch":
		return builderEquipmentAdvice{
			Options: []string{
				"Waffenwahl: Kurzschwert oder eine beliebige einfache Waffe",
				"Paket: Verlieserkunderausrüstung oder Entdeckerausrüstung",
				"Zusätzlich: 10 Wurfpfeile",
			},
			Recommendation: []string{
				"Kurzschwert",
				"Entdeckerausrüstung",
				"10 Wurfpfeile",
			},
			WeaponNotes:    "Kurzschwert als solide frühe Waffe, Wurfpfeile für Reichweite ohne Ausrüstungslast.",
			CombatOverview: "Leichter, mobiler Mönch-Start mit Fokus auf Beweglichkeit und flexiblem Nah-/Fernkampf.",
		}, true
	case "Paladin":
		return builderEquipmentAdvice{
			Options: []string{
				"Waffenwahl: eine Kriegswaffe und Schild oder zwei Kriegswaffen",
				"Zusatzwahl: fünf Wurfspeere oder eine beliebige einfache Nahkampfwaffe",
				"Paket: Priesterpack oder Entdeckerausrüstung",
				"Zusätzlich: Kettenhemd und heiliges Symbol",
			},
			Recommendation: []string{
				"Langschwert und Schild",
				"fünf Wurfspeere",
				"Entdeckerausrüstung",
				"Kettenhemd",
				"heiliges Symbol",
			},
			WeaponNotes:    "Schwert-und-Schild-Start für hohe Rüstungsklasse, Wurfspeere als Distanzoption.",
			CombatOverview: "Klassischer Paladin-Start mit hoher Defensive und guten Voraussetzungen für Frontkampf.",
		}, true
	case "Waldläufer":
		return builderEquipmentAdvice{
			Options: []string{
				"Rüstung: Schuppenpanzer oder Lederrüstung",
				"Nahkampfwaffen: zwei Kurzschwerter oder zwei einfache Nahkampfwaffen",
				"Paket: Verlieserkunderausrüstung oder Entdeckerausrüstung",
				"Zusätzlich: Langbogen und Köcher mit 20 Pfeilen",
			},
			Recommendation: []string{
				"Lederrüstung",
				"zwei Kurzschwerter",
				"Entdeckerausrüstung",
				"Langbogen und Köcher mit 20 Pfeilen",
			},
			WeaponNotes:    "Langbogen für starke Fernkampferöffnung, zwei Kurzschwerter für flexible Nahkampfrunden.",
			CombatOverview: "Vielseitiger Waldläufer-Start mit sauberer Mischung aus Fernkampf, Mobilität und Nahkampfflexibilität.",
		}, true
	case "Schurke":
		return builderEquipmentAdvice{
			Options: []string{
				"Waffenwahl: Rapier oder Kurzschwert",
				"Fernkampf: Kurzbogen mit Köcher und 20 Pfeilen oder Kurzschwert",
				"Paket: Einbrecherpack, Verlieserkunderausrüstung oder Entdeckerausrüstung",
				"Zusätzlich: Lederrüstung, zwei Dolche und Diebeswerkzeug",
			},
			Recommendation: []string{
				"Rapier",
				"Kurzbogen mit Köcher und 20 Pfeilen",
				"Einbrecherpack",
				"Lederrüstung",
				"zwei Dolche",
				"Diebeswerkzeug",
			},
			WeaponNotes:    "Rapier für Finesse im Nahkampf, Kurzbogen für verlässliche Hinterhaltsangriffe auf Distanz.",
			CombatOverview: "Klassischer Schurken-Start mit Finesse-Waffe, Stealth-tauglicher Ausrüstung und vollem Utility-Paket.",
		}, true
	case "Zauberer":
		return builderEquipmentAdvice{
			Options: []string{
				"Fernkampf oder Waffe: leichte Armbrust mit 20 Bolzen oder eine beliebige einfache Waffe",
				"Zauberfokus: Komponentenbeutel oder arkane Fokuskomponente",
				"Paket: Verlieserkunderausrüstung oder Entdeckerausrüstung",
				"Zusätzlich: zwei Dolche",
			},
			Recommendation: []string{
				"leichte Armbrust mit 20 Bolzen",
				"arkaner Fokus",
				"Entdeckerausrüstung",
				"zwei Dolche",
			},
			WeaponNotes:    "Leichte Armbrust für frühen Fernkampfschaden, Dolche als Notfallreserve.",
			CombatOverview: "Pragmatischer Zauberer-Start mit Zauberfokus, etwas Reichweite und minimaler Selbstverteidigung.",
		}, true
	case "Hexenmeister":
		return builderEquipmentAdvice{
			Options: []string{
				"Fernkampf oder Waffe: leichte Armbrust mit 20 Bolzen oder eine beliebige einfache Waffe",
				"Zauberfokus: Komponentenbeutel oder arkane Fokuskomponente",
				"Paket: Gelehrtenpack oder Verlieserkunderausrüstung",
				"Zusätzlich: Lederrüstung, eine beliebige einfache Waffe und zwei Dolche",
			},
			Recommendation: []string{
				"leichte Armbrust mit 20 Bolzen",
				"arkaner Fokus",
				"Gelehrtenpack",
				"Lederrüstung",
				"Quarterstaff",
				"zwei Dolche",
			},
			WeaponNotes:    "Leichte Armbrust für frühe Distanz, einfacher Stab und Dolche als Backup.",
			CombatOverview: "Stabiler Hexenmeister-Start mit Fokus auf Zauberwirken, brauchbarer Reichweite und etwas Reserveausrüstung.",
		}, true
	case "Magier":
		return builderEquipmentAdvice{
			Options: []string{
				"Waffenwahl: Quarterstaff oder Dolch",
				"Zauberfokus: Komponentenbeutel oder arkane Fokuskomponente",
				"Paket: Gelehrtenpack oder Entdeckerausrüstung",
				"Zusätzlich: Zauberbuch",
			},
			Recommendation: []string{
				"Quarterstaff",
				"arkaner Fokus",
				"Gelehrtenpack",
				"Zauberbuch",
			},
			WeaponNotes:    "Quarterstaff als einfache Nahkampf-Notlösung, Fokus und Zauberbuch für den Kern des Builds.",
			CombatOverview: "Klassischer Magier-Start mit vollem Zauber-Setup und minimaler, aber ausreichender Reserveausrüstung.",
		}, true
	default:
		return builderEquipmentAdvice{}, false
	}
}

func builderFeatureAdviceForCharacter(character Character) (builderFeatureAdvice, bool) {
	classRule, ok := builderClassRuleForCharacter(character)
	if !ok {
		return builderFeatureAdvice{}, false
	}
	switch classRule.ClassName {
	case "Barbar":
		return builderFeatureAdvice{Options: []string{"Raserei", "Unarmored Defense"}, Recommendation: []string{"Raserei", "Unarmored Defense"}}, true
	case "Barde":
		return builderFeatureAdvice{Options: []string{"Zauberwirken", "Bardic Inspiration"}, Recommendation: []string{"Zauberwirken", "Bardic Inspiration"}}, true
	case "Kleriker":
		return builderFeatureAdvice{Options: []string{"Zauberwirken", "Göttliche Domäne"}, Recommendation: []string{"Zauberwirken", "Göttliche Domäne: Leben"}}, true
	case "Druide":
		return builderFeatureAdvice{Options: []string{"Zauberwirken", "Druidic"}, Recommendation: []string{"Zauberwirken", "Druidic"}}, true
	case "Kämpfer":
		return builderFeatureAdvice{Options: []string{"Fighting Style", "Second Wind"}, Recommendation: []string{"Fighting Style: Defense", "Second Wind"}}, true
	case "Mönch":
		return builderFeatureAdvice{Options: []string{"Unarmored Defense", "Martial Arts"}, Recommendation: []string{"Unarmored Defense", "Martial Arts"}}, true
	case "Paladin":
		return builderFeatureAdvice{Options: []string{"Divine Sense", "Lay on Hands"}, Recommendation: []string{"Divine Sense", "Lay on Hands"}}, true
	case "Waldläufer":
		return builderFeatureAdvice{Options: []string{"Favored Enemy", "Natural Explorer"}, Recommendation: []string{"Favored Enemy: Monstrositäten", "Natural Explorer: Wald"}}, true
	case "Schurke":
		return builderFeatureAdvice{Options: []string{"Expertise", "Sneak Attack", "Thieves' Cant"}, Recommendation: []string{"Expertise: Wahrnehmung und Heimlichkeit", "Sneak Attack", "Thieves' Cant"}}, true
	case "Zauberer":
		return builderFeatureAdvice{Options: []string{"Zauberwirken", "Sorcerous Origin"}, Recommendation: []string{"Zauberwirken", "Sorcerous Origin: Draconic Bloodline"}}, true
	case "Hexenmeister":
		return builderFeatureAdvice{Options: []string{"Otherworldly Patron", "Pact Magic"}, Recommendation: []string{"Otherworldly Patron: Fiend", "Pact Magic"}}, true
	case "Magier":
		return builderFeatureAdvice{Options: []string{"Zauberwirken", "Arcane Recovery"}, Recommendation: []string{"Zauberwirken", "Arcane Recovery"}}, true
	default:
		return builderFeatureAdvice{}, false
	}
}

func builderHasLevelOneSpellcasting(character Character) bool {
	classRule, ok := builderClassRuleForCharacter(character)
	if !ok {
		return false
	}
	switch classRule.ClassName {
	case "Barde", "Kleriker", "Druide", "Zauberer", "Hexenmeister", "Magier":
		return true
	default:
		return false
	}
}

func builderSpellAdviceForCharacter(character Character) (builderSpellAdvice, bool) {
	if !builderHasLevelOneSpellcasting(character) {
		return builderSpellAdvice{}, false
	}
	prof := deriveCharacterProficiencyBonus(character)
	if prof <= 0 {
		prof = 2
	}
	classRule, _ := builderClassRuleForCharacter(character)
	switch classRule.ClassName {
	case "Barde":
		mod := abilityModifierFromAny(character.Abilities["charisma"])
		return builderSpellAdvice{
			Options:          []string{"Cantrips wie Vicious Mockery, Mage Hand, Minor Illusion, Dancing Lights", "Zauber 1. Grades wie Healing Word, Faerie Fire, Dissonant Whispers, Tasha's Hideous Laughter"},
			Recommendation:   []string{"Vicious Mockery", "Mage Hand", "Healing Word", "Faerie Fire", "Dissonant Whispers", "Tasha's Hideous Laughter"},
			SpellNotes:       "Als Barde auf Stufe 1 kennst du 2 Zaubertricks und 4 Zauber des 1. Grades, mit 2 Zauberplätzen des 1. Grades.",
			SpellAttacks:     []string{fmt.Sprintf("Zauber-SG %d", 8+prof+mod), fmt.Sprintf("Zauberangriff +%d", prof+mod)},
			SpellSaveDC:      fmt.Sprintf("%d", 8+prof+mod),
			SpellAttackBonus: fmt.Sprintf("+%d", prof+mod),
		}, true
	case "Kleriker":
		mod := abilityModifierFromAny(character.Abilities["wisdom"])
		prepared := maxInt(1, 1+mod)
		return builderSpellAdvice{
			Options:          []string{"Cantrips wie Sacred Flame, Guidance, Thaumaturgy, Light", "Vorbereitbare Zauber wie Bless, Cure Wounds, Healing Word, Guiding Bolt, Sanctuary"},
			Recommendation:   []string{"Sacred Flame", "Guidance", "Thaumaturgy", "Bless", "Cure Wounds", "Healing Word", "Guiding Bolt"},
			SpellNotes:       fmt.Sprintf("Als Kleriker auf Stufe 1 kennst du 3 Zaubertricks und bereitest %d Zauber vor; dazu hast du 2 Zauberplätze des 1. Grades.", prepared),
			SpellAttacks:     []string{fmt.Sprintf("Zauber-SG %d", 8+prof+mod), fmt.Sprintf("Zauberangriff +%d", prof+mod)},
			SpellSaveDC:      fmt.Sprintf("%d", 8+prof+mod),
			SpellAttackBonus: fmt.Sprintf("+%d", prof+mod),
		}, true
	case "Druide":
		mod := abilityModifierFromAny(character.Abilities["wisdom"])
		prepared := maxInt(1, 1+mod)
		return builderSpellAdvice{
			Options:          []string{"Cantrips wie Guidance, Produce Flame, Shillelagh, Resistance", "Vorbereitbare Zauber wie Entangle, Faerie Fire, Cure Wounds, Healing Word, Thunderwave"},
			Recommendation:   []string{"Guidance", "Produce Flame", "Entangle", "Faerie Fire", "Cure Wounds", "Healing Word"},
			SpellNotes:       fmt.Sprintf("Als Druide auf Stufe 1 kennst du 2 Zaubertricks und bereitest %d Zauber vor; dazu hast du 2 Zauberplätze des 1. Grades.", prepared),
			SpellAttacks:     []string{fmt.Sprintf("Zauber-SG %d", 8+prof+mod), fmt.Sprintf("Zauberangriff +%d", prof+mod)},
			SpellSaveDC:      fmt.Sprintf("%d", 8+prof+mod),
			SpellAttackBonus: fmt.Sprintf("+%d", prof+mod),
		}, true
	case "Zauberer":
		mod := abilityModifierFromAny(character.Abilities["charisma"])
		return builderSpellAdvice{
			Options:          []string{"Cantrips wie Fire Bolt, Prestidigitation, Mage Hand, Ray of Frost", "Zauber 1. Grades wie Shield, Magic Missile, Sleep, Mage Armor"},
			Recommendation:   []string{"Fire Bolt", "Prestidigitation", "Mage Hand", "Ray of Frost", "Shield", "Magic Missile"},
			SpellNotes:       "Als Zauberer auf Stufe 1 kennst du 4 Zaubertricks, 2 Zauber des 1. Grades und hast 2 Zauberplätze des 1. Grades.",
			SpellAttacks:     []string{fmt.Sprintf("Zauber-SG %d", 8+prof+mod), fmt.Sprintf("Zauberangriff +%d", prof+mod)},
			SpellSaveDC:      fmt.Sprintf("%d", 8+prof+mod),
			SpellAttackBonus: fmt.Sprintf("+%d", prof+mod),
		}, true
	case "Hexenmeister":
		mod := abilityModifierFromAny(character.Abilities["charisma"])
		return builderSpellAdvice{
			Options:          []string{"Cantrips wie Eldritch Blast, Mage Hand, Minor Illusion", "Zauber 1. Grades wie Hex, Armor of Agathys, Hellish Rebuke"},
			Recommendation:   []string{"Eldritch Blast", "Mage Hand", "Hex", "Armor of Agathys"},
			SpellNotes:       "Als Hexenmeister auf Stufe 1 kennst du 2 Zaubertricks, 2 Zauber des 1. Grades und hast 1 Zauberplatz des 1. Grades.",
			SpellAttacks:     []string{fmt.Sprintf("Zauber-SG %d", 8+prof+mod), fmt.Sprintf("Zauberangriff +%d", prof+mod)},
			SpellSaveDC:      fmt.Sprintf("%d", 8+prof+mod),
			SpellAttackBonus: fmt.Sprintf("+%d", prof+mod),
		}, true
	case "Magier":
		mod := abilityModifierFromAny(character.Abilities["intelligence"])
		prepared := maxInt(1, 1+mod)
		return builderSpellAdvice{
			Options:          []string{"Cantrips wie Fire Bolt, Mage Hand, Prestidigitation, Minor Illusion", "Zauberbuch-Einträge wie Magic Missile, Shield, Sleep, Detect Magic, Mage Armor, Thunderwave"},
			Recommendation:   []string{"Fire Bolt", "Mage Hand", "Prestidigitation", "Magic Missile", "Shield", "Sleep", "Detect Magic", "Mage Armor"},
			SpellNotes:       fmt.Sprintf("Als Magier auf Stufe 1 kennst du 3 Zaubertricks, startest mit 6 Zaubern im Zauberbuch und bereitest %d davon vor; dazu hast du 2 Zauberplätze des 1. Grades.", prepared),
			SpellAttacks:     []string{fmt.Sprintf("Zauber-SG %d", 8+prof+mod), fmt.Sprintf("Zauberangriff +%d", prof+mod)},
			SpellSaveDC:      fmt.Sprintf("%d", 8+prof+mod),
			SpellAttackBonus: fmt.Sprintf("+%d", prof+mod),
		}, true
	default:
		return builderSpellAdvice{}, false
	}
}

func builderDerivedArmorClass(character Character) int {
	items := metadataStringList(defaultMetadata(character.Metadata)["current_inventory"])
	if len(items) == 0 {
		items = metadataStringList(defaultMetadata(character.Metadata)["starting_equipment"])
	}
	joined := strings.ToLower(strings.Join(items, " | "))
	dex := abilityModifierFromAny(character.Abilities["dexterity"])
	con := abilityModifierFromAny(character.Abilities["constitution"])
	wis := abilityModifierFromAny(character.Abilities["wisdom"])
	ac := 10 + dex
	if strings.Contains(joined, "kettenhemd") {
		ac = 16
	} else if strings.Contains(joined, "schuppenpanzer") {
		ac = 14 + minInt(dex, 2)
	} else if strings.Contains(joined, "lederrüstung") || strings.Contains(joined, "lederüstung") || strings.Contains(joined, "leather armor") {
		ac = 11 + dex
	}
	if classRule, ok := builderClassRuleForCharacter(character); ok {
		switch classRule.ClassName {
		case "Barbar":
			if 10+dex+con > ac {
				ac = 10 + dex + con
			}
		case "Mönch":
			if 10+dex+wis > ac {
				ac = 10 + dex + wis
			}
		}
	}
	if strings.Contains(joined, "schild") || strings.Contains(joined, "shield") || strings.Contains(joined, "holzschild") {
		ac += 2
	}
	if containsStringFold(character.Features, "Fighting Style: Defense") {
		ac += 1
	}
	return ac
}

func builderDerivedHitPointMax(character Character) int {
	classRule, ok := builderClassRuleForCharacter(character)
	if !ok || strings.TrimSpace(classRule.HitDie) == "" {
		if character.HitPointMax != nil {
			return *character.HitPointMax
		}
		return 0
	}
	hitDieText := strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(classRule.HitDie)), "W")
	hitDieValue, err := strconv.Atoi(hitDieText)
	if err != nil || hitDieValue <= 0 {
		if character.HitPointMax != nil {
			return *character.HitPointMax
		}
		return 0
	}
	return hitDieValue + abilityModifierFromAny(character.Abilities["constitution"])
}

func builderDerivedSpeed(character Character) string {
	if strings.TrimSpace(character.Speed) != "" {
		return strings.TrimSpace(character.Speed)
	}
	if raceRule, ok := builderRaceRuleForCharacter(character); ok && strings.TrimSpace(raceRule.Speed) != "" {
		return strings.TrimSpace(raceRule.Speed)
	}
	return ""
}

func builderCombatAttackRecommendations(character Character) []string {
	classRule, ok := builderClassRuleForCharacter(character)
	if !ok {
		return nil
	}
	prof := deriveCharacterProficiencyBonus(character)
	if prof <= 0 {
		prof = 2
	}
	strMod := abilityModifierFromAny(character.Abilities["strength"])
	dexMod := abilityModifierFromAny(character.Abilities["dexterity"])
	switch classRule.ClassName {
	case "Barbar":
		return []string{
			fmt.Sprintf("Großaxt: Angriff +%d, Schaden 1W12+%d Hiebschaden", prof+strMod, strMod),
			fmt.Sprintf("Wurfspeer: Angriff +%d, Schaden 1W6+%d Stichschaden", prof+strMod, strMod),
		}
	case "Barde":
		finesse := maxInt(dexMod, abilityModifierFromAny(character.Abilities["strength"]))
		return []string{fmt.Sprintf("Rapier: Angriff +%d, Schaden 1W8+%d Stichschaden", prof+finesse, finesse)}
	case "Kleriker":
		return []string{
			fmt.Sprintf("Streitkolben: Angriff +%d, Schaden 1W6+%d Wuchtschaden", prof+strMod, strMod),
			fmt.Sprintf("Leichte Armbrust: Angriff +%d, Schaden 1W8+%d Stichschaden", prof+dexMod, dexMod),
		}
	case "Druide":
		finesse := maxInt(dexMod, abilityModifierFromAny(character.Abilities["strength"]))
		return []string{
			fmt.Sprintf("Krummsäbel: Angriff +%d, Schaden 1W6+%d Hiebschaden", prof+finesse, finesse),
			"Produce Flame oder ein anderer Angriffszauber nach Zauberwahl",
		}
	case "Kämpfer":
		return []string{
			fmt.Sprintf("Langschwert: Angriff +%d, Schaden 1W8+%d Hiebschaden", prof+strMod, strMod),
			fmt.Sprintf("Leichte Armbrust: Angriff +%d, Schaden 1W8+%d Stichschaden", prof+dexMod, dexMod),
		}
	case "Mönch":
		finesse := maxInt(strMod, dexMod)
		return []string{
			fmt.Sprintf("Kurzschwert: Angriff +%d, Schaden 1W6+%d Stichschaden", prof+finesse, finesse),
			fmt.Sprintf("Wurfpfeile: Angriff +%d, Schaden 1W4+%d Stichschaden", prof+dexMod, dexMod),
		}
	case "Paladin":
		return []string{
			fmt.Sprintf("Langschwert: Angriff +%d, Schaden 1W8+%d Hiebschaden", prof+strMod, strMod),
			fmt.Sprintf("Wurfspeer: Angriff +%d, Schaden 1W6+%d Stichschaden", prof+strMod, strMod),
		}
	case "Waldläufer":
		finesse := maxInt(strMod, dexMod)
		return []string{
			fmt.Sprintf("Langbogen: Angriff +%d, Schaden 1W8+%d Stichschaden", prof+dexMod, dexMod),
			fmt.Sprintf("Kurzschwert: Angriff +%d, Schaden 1W6+%d Stichschaden", prof+finesse, finesse),
		}
	case "Schurke":
		finesse := maxInt(strMod, dexMod)
		return []string{
			fmt.Sprintf("Rapier: Angriff +%d, Schaden 1W8+%d Stichschaden", prof+finesse, finesse),
			fmt.Sprintf("Kurzbogen: Angriff +%d, Schaden 1W6+%d Stichschaden", prof+dexMod, dexMod),
		}
	case "Zauberer":
		return []string{
			fmt.Sprintf("Leichte Armbrust: Angriff +%d, Schaden 1W8+%d Stichschaden", prof+dexMod, dexMod),
			"Fire Bolt oder ein anderer Angriffszauber nach Zauberwahl",
		}
	case "Hexenmeister":
		chaMod := abilityModifierFromAny(character.Abilities["charisma"])
		return []string{
			fmt.Sprintf("Leichte Armbrust: Angriff +%d, Schaden 1W8+%d Stichschaden", prof+dexMod, dexMod),
			fmt.Sprintf("Eldritch Blast: Zauberangriff +%d, Schaden 1W10 Kraftschaden", prof+chaMod),
		}
	case "Magier":
		intMod := abilityModifierFromAny(character.Abilities["intelligence"])
		return []string{
			fmt.Sprintf("Quarterstaff: Angriff +%d, Schaden 1W6+%d Wuchtschaden", prof+strMod, strMod),
			fmt.Sprintf("Fire Bolt: Zauberangriff +%d, Schaden 1W10 Feuerschaden", prof+intMod),
		}
	default:
		return nil
	}
}

func builderCombatAttackTable(character Character) string {
	classRule, ok := builderClassRuleForCharacter(character)
	if !ok {
		return ""
	}
	prof := deriveCharacterProficiencyBonus(character)
	if prof <= 0 {
		prof = 2
	}
	strMod := abilityModifierFromAny(character.Abilities["strength"])
	dexMod := abilityModifierFromAny(character.Abilities["dexterity"])
	switch classRule.ClassName {
	case "Barbar":
		return strings.Join([]string{
			fmt.Sprintf("Großaxt | +%d | STR | 5 ft | +%d | 1W12+%d | Hieb", prof, prof+strMod, strMod),
			"Beschreibung: Standard-Nahkampfangriff mit der Hauptwaffe.",
			fmt.Sprintf("Wurfspeer | +%d | STR | 20/60 ft | +%d | 1W6+%d | Stich", prof, prof+strMod, strMod),
			"Beschreibung: Nah- oder Wurfwaffe für Distanz bis 60 Fuß.",
		}, "\n")
	case "Barde":
		finesse := maxInt(dexMod, strMod)
		return strings.Join([]string{
			fmt.Sprintf("Rapier | +%d | %s | 5 ft | +%d | 1W8+%d | Stich", prof, abilityShortLabelForModChoice(dexMod, strMod), prof+finesse, finesse),
			"Beschreibung: Finesse-Nahkampfangriff mit der empfohlenen Standardwaffe.",
		}, "\n")
	case "Kleriker":
		return strings.Join([]string{
			fmt.Sprintf("Streitkolben | +%d | STR | 5 ft | +%d | 1W6+%d | Wucht", prof, prof+strMod, strMod),
			"Beschreibung: Robuste Nahkampfoption des Klerikers.",
			fmt.Sprintf("Leichte Armbrust | +%d | DEX | 80/320 ft | +%d | 1W8+%d | Stich", prof, prof+dexMod, dexMod),
			"Beschreibung: Solide Distanzwaffe für den SRD-Standardstart.",
		}, "\n")
	case "Druide":
		finesse := maxInt(dexMod, strMod)
		return strings.Join([]string{
			fmt.Sprintf("Krummsäbel | +%d | %s | 5 ft | +%d | 1W6+%d | Hieb", prof, abilityShortLabelForModChoice(dexMod, strMod), prof+finesse, finesse),
			"Beschreibung: Empfohlene Nahkampfwaffe für den SRD-Druidenstart.",
		}, "\n")
	case "Kämpfer":
		return strings.Join([]string{
			fmt.Sprintf("Langschwert | +%d | STR | 5 ft | +%d | 1W8+%d | Hieb", prof, prof+strMod, strMod),
			"Beschreibung: Einhändig mit Schild geführt; Standardschaden ohne Zweihandhaltung.",
			fmt.Sprintf("Leichte Armbrust | +%d | DEX | 80/320 ft | +%d | 1W8+%d | Stich", prof, prof+dexMod, dexMod),
			"Beschreibung: Standard-Fernkampfwaffe mit 20 Bolzen.",
		}, "\n")
	case "Mönch":
		finesse := maxInt(strMod, dexMod)
		return strings.Join([]string{
			fmt.Sprintf("Kurzschwert | +%d | %s | 5 ft | +%d | 1W6+%d | Stich", prof, abilityShortLabelForModChoice(dexMod, strMod), prof+finesse, finesse),
			"Beschreibung: Finesse-Nahkampfangriff mit der Hauptwaffe.",
			fmt.Sprintf("Wurfpfeile | +%d | DEX | 20/60 ft | +%d | 1W4+%d | Stich", prof, prof+dexMod, dexMod),
			"Beschreibung: Leichte Distanzoption für den Start.",
		}, "\n")
	case "Paladin":
		return strings.Join([]string{
			fmt.Sprintf("Langschwert | +%d | STR | 5 ft | +%d | 1W8+%d | Hieb", prof, prof+strMod, strMod),
			"Beschreibung: Standardschlag mit Schild in der Nebenhand.",
			fmt.Sprintf("Wurfspeer | +%d | STR | 20/60 ft | +%d | 1W6+%d | Stich", prof, prof+strMod, strMod),
			"Beschreibung: Nah- oder Distanzoption für kurze Reichweiten.",
		}, "\n")
	case "Waldläufer":
		finesse := maxInt(strMod, dexMod)
		return strings.Join([]string{
			fmt.Sprintf("Langbogen | +%d | DEX | 150/600 ft | +%d | 1W8+%d | Stich", prof, prof+dexMod, dexMod),
			"Beschreibung: Primäre Distanzwaffe des Standard-Builds.",
			fmt.Sprintf("Kurzschwert | +%d | %s | 5 ft | +%d | 1W6+%d | Stich", prof, abilityShortLabelForModChoice(dexMod, strMod), prof+finesse, finesse),
			"Beschreibung: Fallback für Nahkampfreichweite.",
		}, "\n")
	case "Schurke":
		finesse := maxInt(strMod, dexMod)
		return strings.Join([]string{
			fmt.Sprintf("Rapier | +%d | %s | 5 ft | +%d | 1W8+%d | Stich", prof, abilityShortLabelForModChoice(dexMod, strMod), prof+finesse, finesse),
			"Beschreibung: Finesse-Waffe für präzise Nahkampfangriffe.",
			fmt.Sprintf("Kurzbogen | +%d | DEX | 80/320 ft | +%d | 1W6+%d | Stich", prof, prof+dexMod, dexMod),
			"Beschreibung: Standard-Fernkampfwaffe des Schurkenstarts.",
		}, "\n")
	case "Zauberer":
		return strings.Join([]string{
			fmt.Sprintf("Leichte Armbrust | +%d | DEX | 80/320 ft | +%d | 1W8+%d | Stich", prof, prof+dexMod, dexMod),
			"Beschreibung: Nichtmagische Distanzoption für frühe Kämpfe.",
		}, "\n")
	case "Hexenmeister":
		chaMod := abilityModifierFromAny(character.Abilities["charisma"])
		return strings.Join([]string{
			fmt.Sprintf("Leichte Armbrust | +%d | DEX | 80/320 ft | +%d | 1W8+%d | Stich", prof, prof+dexMod, dexMod),
			"Beschreibung: Standard-Fernkampfwaffe ohne Zauberverbrauch.",
			fmt.Sprintf("Eldritch Blast | +%d | CHA | 120 ft | +%d | 1W10 | Kraft", prof, prof+chaMod),
			"Beschreibung: Haupt-Zauberangriff, falls als Zaubertrick gewählt.",
		}, "\n")
	case "Magier":
		intMod := abilityModifierFromAny(character.Abilities["intelligence"])
		return strings.Join([]string{
			fmt.Sprintf("Quarterstaff | +%d | STR | 5 ft | +%d | 1W6+%d | Wucht", prof, prof+strMod, strMod),
			"Beschreibung: Einfache Nahkampfreserve für den Notfall.",
			fmt.Sprintf("Fire Bolt | +%d | INT | 120 ft | +%d | 1W10 | Feuer", prof, prof+intMod),
			"Beschreibung: Standard-Zauberangriff, falls als Zaubertrick gewählt.",
		}, "\n")
	default:
		return ""
	}
}

func abilityShortLabelForModChoice(primary int, secondary int) string {
	if primary >= secondary {
		return "DEX"
	}
	return "STR"
}

func containsStringFold(items []string, needle string) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), needle) {
			return true
		}
	}
	return false
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func classNameOrFallback(character Character) string {
	if classRule, ok := builderClassRuleForCharacter(character); ok {
		return classRule.ClassName
	}
	if value := strings.TrimSpace(character.ClassAndLevel); value != "" {
		return value
	}
	return "die Klasse"
}

func builderDeterministicSkillRepairReply(character Character, stage string, latestUserMessage string, previousAssistant string) (string, CharacterBuilderPatch, bool) {
	message := normalizeBuilderIntentText(latestUserMessage)
	if !strings.Contains(message, "markier") &&
		!strings.Contains(message, "markiert") &&
		!strings.Contains(message, "geubt") &&
		!strings.Contains(message, "geübt") &&
		!strings.Contains(message, "noch nicht") {
		return "", CharacterBuilderPatch{}, false
	}
	classRule, ok := builderClassRuleForCharacter(character)
	if !ok || classRule.SkillChoiceCount <= 0 {
		return "", CharacterBuilderPatch{}, false
	}
	backgroundSkills, classSkills, _ := builderSkillSources(character)
	selected := parseSkillChoicesFromMessage(latestUserMessage)
	if len(selected) == 0 && builderMessageConfirmsPreviousSuggestion(latestUserMessage) {
		selected = builderSuggestedSkillsFromAssistantReply(previousAssistant)
	}
	if len(selected) == 0 {
		return "", CharacterBuilderPatch{}, false
	}
	backgroundSet := map[string]bool{}
	for _, skill := range backgroundSkills {
		backgroundSet[skill] = true
	}
	classAllowed := map[string]bool{}
	for _, skill := range classRule.SkillChoices {
		classAllowed[skill] = true
	}
	newClassSkills := append([]string{}, classSkills...)
	for _, skill := range selected {
		if backgroundSet[skill] || !classAllowed[skill] {
			continue
		}
		newClassSkills = append(newClassSkills, skill)
	}
	newClassSkills = uniqueCanonicalSkills(newClassSkills)
	if len(newClassSkills) == len(classSkills) {
		return "", CharacterBuilderPatch{}, false
	}
	if len(newClassSkills) > classRule.SkillChoiceCount {
		newClassSkills = newClassSkills[:classRule.SkillChoiceCount]
	}
	allSkills := uniqueCanonicalSkills(append(backgroundSkills, newClassSkills...))
	patch := CharacterBuilderPatch{
		Metadata: map[string]any{
			"class_skill_proficiencies": selectedOrNil(newClassSkills),
			"skill_proficiencies":       allSkills,
		},
	}
	if normalizeBuilderStage(stage) == "class_proficiencies_and_choices" && len(newClassSkills) >= classRule.SkillChoiceCount {
		patch.Metadata["builder_stage"] = "hit_points_hit_dice_and_movement"
		reply := fmt.Sprintf("Ich habe %s jetzt zusätzlich als Klassen-Fertigkeiten markiert. Insgesamt sind damit %s als geübt eingetragen. Als Nächstes leite ich Trefferpunkte, Trefferwürfel und Bewegungsrate ab.", joinGermanList(newClassSkills), joinGermanList(allSkills))
		return reply, patch, true
	}
	reply := fmt.Sprintf("Ich habe %s jetzt zusätzlich als Klassen-Fertigkeiten markiert. Insgesamt sind damit %s als geübt eingetragen. Der aktuelle Schritt bleibt %s.", joinGermanList(newClassSkills), joinGermanList(allSkills), builderStageTitle(stage))
	return reply, patch, true
}

func builderDeterministicPostHitPointsReply(character Character, latestUserMessage string) (string, CharacterBuilderPatch, bool) {
	if !builderLooksLikeContinueIntent(normalizeBuilderIntentText(latestUserMessage)) {
		return "", CharacterBuilderPatch{}, false
	}
	reply, patch, ok := builderDeterministicSensesAndBodyReply(character, latestUserMessage)
	if !ok {
		return "", CharacterBuilderPatch{}, false
	}
	if patch.Metadata == nil {
		patch.Metadata = map[string]any{}
	}
	patch.Metadata["builder_stage"] = "languages_senses_and_body"
	return reply, patch, true
}

func builderDeterministicSensesAndBodyReply(character Character, latestUserMessage string) (string, CharacterBuilderPatch, bool) {
	raceRule, hasRace := builderRaceRuleForCharacter(character)
	metadata := defaultMetadata(character.Metadata)
	message := normalizeBuilderIntentText(latestUserMessage)

	patch := CharacterBuilderPatch{
		Metadata: map[string]any{
			"builder_stage": "languages_senses_and_body",
		},
	}

	sensesValue := strings.TrimSpace(fmt.Sprint(metadata["senses"]))
	if hasRace && sensesValue == "" && strings.TrimSpace(raceRule.Darkvision) != "" {
		sensesValue = fmt.Sprintf("Dunkelsicht %s", raceRule.Darkvision)
		patch.Metadata["senses"] = sensesValue
	}

	currentLanguages := stringListFromAny(character.Languages)
	if len(currentLanguages) == 0 && hasRace && len(raceRule.FixedLanguages) > 0 {
		currentLanguages = append(currentLanguages, raceRule.FixedLanguages...)
	}

	if builderLooksLikeLanguageSelection(message) && hasRace && raceRule.ExtraLanguageChoiceCount > 0 {
		selected := strings.TrimSpace(latestUserMessage)
		if selected != "" {
			currentLanguages = append([]string{}, raceRule.FixedLanguages...)
			currentLanguages = append(currentLanguages, selected)
			currentLanguages = uniquePreserveOrder(currentLanguages)
			patch.Languages = currentLanguages
		}
	}

	missingBodyFields := builderMissingBodyFieldLabels(metadata)
	parts := []string{}
	if len(currentLanguages) > 0 {
		parts = append(parts, fmt.Sprintf("Die Sprachen sind festgelegt: %s.", joinGermanList(currentLanguages)))
	}
	if sensesValue != "" {
		parts = append(parts, fmt.Sprintf("Die Sinne sind festgelegt: %s.", sensesValue))
	}
	if len(missingBodyFields) > 0 {
		parts = append(parts, fmt.Sprintf("Lege jetzt %s fest.", joinGermanList(missingBodyFields)))
	} else {
		patch.Metadata["builder_stage"] = "personality"
		parts = append(parts, "Sinne und Körperdaten sind festgelegt. Lege jetzt Persönlichkeitsmerkmale, Ideale, Bindungen und Makel fest.")
	}
	return strings.Join(parts, " "), patch, true
}

func builderLooksLikeContinueIntent(message string) bool {
	if message == "" {
		return false
	}
	markers := []string{
		"weiter",
		"naechst",
		"nächst",
		"fortfahren",
		"weiter machen",
		"ok",
		"okay",
	}
	for _, marker := range markers {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func builderLooksLikeLanguageSelection(message string) bool {
	if message == "" {
		return false
	}
	if builderQuestionLooksLikeListRequest(message) {
		return false
	}
	blockers := []string{
		"welch", "zeige", "regel", "pdf", "seite", "sprache", "sprachen", "sinn", "sinne",
		"koerper", "körper", "merkmal", "merkmale", "weiter", "naechst", "nächst",
	}
	for _, blocker := range blockers {
		if strings.Contains(message, blocker) {
			return false
		}
	}
	return true
}

func builderMissingBodyFieldLabels(metadata map[string]any) []string {
	type bodyField struct {
		key   string
		label string
	}
	fields := []bodyField{
		{key: "size", label: "Größe"},
		{key: "age", label: "Alter"},
		{key: "eyes", label: "Augenfarbe"},
		{key: "hair", label: "Haarfarbe"},
		{key: "skin", label: "Hautfarbe"},
		{key: "weight", label: "Gewicht"},
	}
	missing := make([]string, 0, len(fields))
	for _, field := range fields {
		if strings.TrimSpace(fmt.Sprint(metadata[field.key])) == "" {
			missing = append(missing, field.label)
		}
	}
	return missing
}

func builderClassRuleForCharacter(character Character) (builderClassRule, bool) {
	return builderClassRuleForText(character.ClassAndLevel)
}

func builderClassRuleForText(value string) (builderClassRule, bool) {
	normalized := normalizeBuilderIntentText(value)
	if normalized == "" {
		return builderClassRule{}, false
	}
	for _, rule := range builderCoreClassRules {
		for _, alias := range rule.Aliases {
			if strings.Contains(normalized, normalizeBuilderIntentText(alias)) {
				return rule, true
			}
		}
	}
	return builderClassRule{}, false
}

func builderRaceRuleForCharacter(character Character) (builderRaceRule, bool) {
	return builderRaceRuleForText(character.Race)
}

func builderRaceRuleForText(value string) (builderRaceRule, bool) {
	normalized := normalizeBuilderIntentText(value)
	if normalized == "" {
		return builderRaceRule{}, false
	}
	best := builderRaceRule{}
	bestScore := 0
	for _, rule := range builderCoreRaceRules {
		for _, alias := range rule.Aliases {
			normalizedAlias := normalizeBuilderIntentText(alias)
			if normalizedAlias == "" {
				continue
			}
			if strings.Contains(normalized, normalizedAlias) {
				score := len(strings.Fields(normalizedAlias))
				if score > bestScore {
					best = rule
					bestScore = score
				}
			}
		}
	}
	if bestScore == 0 {
		return builderRaceRule{}, false
	}
	return best, true
}

func joinGermanList(items []string) string {
	clean := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			clean = append(clean, item)
		}
	}
	switch len(clean) {
	case 0:
		return ""
	case 1:
		return clean[0]
	case 2:
		return clean[0] + " und " + clean[1]
	default:
		return strings.Join(clean[:len(clean)-1], ", ") + " und " + clean[len(clean)-1]
	}
}

func parseSkillChoicesFromMessage(message string) []string {
	normalized := normalizeBuilderIntentText(message)
	if normalized == "" {
		return []string{}
	}
	found := make([]string, 0, 4)
	for _, skill := range builderCanonicalSkills {
		for _, alias := range skill.Aliases {
			if strings.Contains(normalized, normalizeBuilderIntentText(alias)) {
				found = append(found, skill.Name)
				break
			}
		}
	}
	return uniqueCanonicalSkills(found)
}

func builderMessageConfirmsPreviousSuggestion(message string) bool {
	normalized := normalizeBuilderIntentText(message)
	if normalized == "" {
		return false
	}
	if len(parseSkillChoicesFromMessage(message)) > 0 {
		return false
	}
	confirmationPhrases := []string{
		"ja nimm",
		"nimm die",
		"die beiden",
		"ja die",
		"mach das",
		"nehme die",
		"nehme ich",
		"passt so",
	}
	for _, phrase := range confirmationPhrases {
		if strings.Contains(normalized, phrase) {
			return true
		}
	}
	return false
}

func builderSuggestedSkillsFromAssistantReply(reply string) []string {
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return []string{}
	}
	recommendationPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)bietet sich\s+(.+?)\s+an`),
		regexp.MustCompile(`(?i)empfehle(?:\s+ich)?\s+(.+?)(?:\.|$)`),
		regexp.MustCompile(`(?i)vorschlag(?:\s+ist|\s+wäre|:)?\s+(.+?)(?:\.|$)`),
	}
	for _, pattern := range recommendationPatterns {
		match := pattern.FindStringSubmatch(reply)
		if len(match) < 2 {
			continue
		}
		skills := parseSkillChoicesFromMessage(match[1])
		if len(skills) == 2 {
			return skills
		}
	}
	sentences := regexp.MustCompile(`[.!?]`).Split(reply, -1)
	for i := len(sentences) - 1; i >= 0; i-- {
		candidate := strings.TrimSpace(sentences[i])
		if candidate == "" {
			continue
		}
		skills := parseSkillChoicesFromMessage(candidate)
		if len(skills) == 2 {
			return skills
		}
	}
	for _, match := range regexp.MustCompile(`„([^“]+)“`).FindAllStringSubmatch(reply, -1) {
		if len(match) < 2 {
			continue
		}
		skills := parseSkillChoicesFromMessage(match[1])
		if len(skills) == 2 {
			return skills
		}
	}
	return []string{}
}

func uniqueCanonicalSkills(items []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		result = append(result, item)
	}
	return result
}

func uniquePreserveOrder(items []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, item)
	}
	return result
}

func selectedOrNil(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	return items
}

func builderSkillSources(character Character) ([]string, []string, []string) {
	metadata := defaultMetadata(character.Metadata)
	backgroundSkills := uniqueCanonicalSkills(stringListFromAny(metadata["background_skill_proficiencies"]))
	classSkills := uniqueCanonicalSkills(stringListFromAny(metadata["class_skill_proficiencies"]))
	allSkills := uniqueCanonicalSkills(stringListFromAny(metadata["skill_proficiencies"]))
	if len(backgroundSkills) == 0 && len(classSkills) == 0 && len(allSkills) > 0 {
		backgroundSkills = allSkills
	}
	if len(allSkills) == 0 {
		allSkills = uniqueCanonicalSkills(append(backgroundSkills, classSkills...))
	}
	return backgroundSkills, classSkills, allSkills
}

func builderBackgroundNameFromMessage(message string) string {
	for _, line := range strings.Split(message, "\n") {
		candidate := strings.TrimSpace(line)
		if candidate == "" {
			continue
		}
		if len(parseSkillChoicesFromMessage(candidate)) > 0 {
			continue
		}
		normalized := normalizeBuilderIntentText(candidate)
		if strings.Contains(normalized, "hintergrund") ||
			strings.Contains(normalized, "welche") ||
			strings.Contains(normalized, "sprache") ||
			strings.Contains(normalized, "werkzeug") ||
			strings.Contains(normalized, "fertigkeit") {
			continue
		}
		if len([]rune(candidate)) > 96 {
			continue
		}
		return normalizeBackground(candidate)
	}
	return ""
}

func parseAbilityChoiceKeys(message string) []string {
	normalized := normalizeBuilderIntentText(message)
	if normalized == "" {
		return []string{}
	}
	aliases := []struct {
		key      string
		patterns []string
	}{
		{key: "strength", patterns: []string{"staerke", "strength"}},
		{key: "dexterity", patterns: []string{"geschicklichkeit", "dexterity", "dex"}},
		{key: "constitution", patterns: []string{"konstitution", "constitution", "con"}},
		{key: "intelligence", patterns: []string{"intelligenz", "intelligence", "int"}},
		{key: "wisdom", patterns: []string{"weisheit", "wisdom", "wis"}},
		{key: "charisma", patterns: []string{"charisma", "cha"}},
	}
	seen := map[string]bool{}
	keys := make([]string, 0, 3)
	for _, alias := range aliases {
		for _, pattern := range alias.patterns {
			if strings.Contains(normalized, pattern) {
				if !seen[alias.key] {
					seen[alias.key] = true
					keys = append(keys, alias.key)
				}
				break
			}
		}
	}
	return keys
}

func abilityKeyLabel(key string) string {
	switch key {
	case "strength":
		return "Stärke"
	case "dexterity":
		return "Geschicklichkeit"
	case "constitution":
		return "Konstitution"
	case "intelligence":
		return "Intelligenz"
	case "wisdom":
		return "Weisheit"
	case "charisma":
		return "Charisma"
	default:
		return key
	}
}

func builderStoryDraftFallback(character *Character, previousAssistant string) string {
	name := strings.TrimSpace(character.Name)
	if name == "" {
		name = "Der Charakter"
	}
	if strings.Contains(strings.ToLower(previousAssistant), "spur des verderbens") {
		return fmt.Sprintf("%s stammt aus einem abgeschiedenen Zirkel, der seit Generationen einen alten Wald schuetzt. In den letzten Monden begann sich dort jedoch ein fauliger Schatten auszubreiten. Tiere flohen, Quellen kippten um und selbst die heiligen Steine des Zirkels verloren ihren Glanz. Die Aeltesten erkannten, dass hinter dem Verderben mehr als eine natuerliche Krankheit steckte. Sie glaubten, dass eine fremde Macht von ausserhalb die Wurzeln des Waldes vergiftete. Weil %s die Zeichen der Natur am klarsten deuten konnte, fiel die Wahl auf ihn. Er verliess den Zirkel nicht aus Abenteuerlust, sondern aus Verantwortung. Jeder Schritt fort von seiner Heimat fuehlte sich wie ein Bruch mit seinem alten Leben an. Trotzdem wusste er, dass Untaetigkeit sein Volk dem Untergang preisgeben wuerde. Auf seiner Reise sucht er nach dem Ursprung des Verderbens und nach einem Weg, es endgueltig zu brechen. Dabei ringt er mit der Angst, zu spaet zurueckzukehren. Zugleich lernt er, dass die Welt ausserhalb des Waldes voller Kraefte ist, die mit dem Schicksal seines Volkes verwoben sind. Jede Spur, die er findet, koennte Rettung oder noch groesseren Untergang bedeuten. Darum betrachtet er jede Begegnung, jedes alte Relikt und jedes Geruecht als moeglichen Teil einer groesseren Wahrheit. Seine Reise ist nicht nur eine Suche nach Antworten, sondern ein Kampf darum, ob seine Heimat ueberlebt.", name, name)
	}
	return fmt.Sprintf("%s stammt aus einem verborgenen Druidenzirkel, der tief in einer alten und schwer zugaenglichen Wildnis lebt. Dort lernte er frueh, dass das Gleichgewicht der Natur nicht von allein bestehen bleibt, sondern behuetet werden muss. Eines Tages zeigten die Zeichen des Waldes, dass sein Volk auf eine ernste Pruefung zusteuerte. Krankheiten breiteten sich aus, die Ernten wurden duenn und selbst die Tiere verhielten sich ungewoehnlich unruhig. Die Aeltesten des Zirkels deuteten diese Entwicklung als Vorboten einer groesseren Gefahr. In alten Ueberlieferungen fanden sie Hinweise auf eine ferne Quelle der Heilung und Erneuerung. Weil %s zugleich pflichtbewusst, wachsam und eng mit den alten Riten vertraut war, wurde er ausgesandt. Er verliess seine Heimat mit dem Auftrag, diese Hoffnung zu finden und zurueckzubringen. Fuer ihn ist diese Reise keine einfache Mission, sondern eine Last, die ueber das Schicksal seines ganzen Volkes entscheidet. Unterwegs muss er lernen, Menschen, Staedte und fremde Maechte zu verstehen, die ihm lange fern waren. Gleichzeitig klammert er sich an die Werte seines Zirkels, auch wenn sie in der weiten Welt nicht immer geteilt werden. Jede Spur von altem Wissen, jeder heilige Ort und jede Legende ueber verlorene Naturmaechte hat fuer ihn Gewicht. Er reist mit dem Bewusstsein, dass Versagen nicht nur ihn selbst treffen wuerde, sondern alle, die zuhause auf seine Rueckkehr hoffen. Darum begegnet er der Welt mit Respekt, aber auch mit innerer Unruhe. Tief in sich traegt er die Frage, ob er tatsaechlich retten kann, was bereits zu zerbrechen beginnt.", name, name)
}

func isLLMGatewayBusy(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, errLLMGatewayBusy) || errors.Is(err, errLLMCircuitOpen)
}

func (h *Handler) saveCharacterBuilderState(ctx context.Context, character *Character, session *LLMSession) error {
	if session != nil {
		if _, err := h.store.UpdateLLMSession(ctx, *session); err != nil {
			if ctx.Err() != nil {
				saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if _, retryErr := h.store.UpdateLLMSession(saveCtx, *session); retryErr != nil {
					return retryErr
				}
				// continue to character save
			} else {
				return err
			}
		}
	}
	if character != nil {
		if _, err := h.store.UpdateCharacter(ctx, *character); err != nil {
			if ctx.Err() != nil {
				saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if _, retryErr := h.store.UpdateCharacter(saveCtx, *character); retryErr != nil {
					return retryErr
				}
				return nil
			}
			return err
		}
	}
	return nil
}

func normalizeCharacterBuilderPatch(patch *CharacterBuilderPatch) {
	if patch == nil {
		return
	}
	if patch.Alignment != nil {
		aligned := normalizeAlignment(*patch.Alignment)
		if aligned != "" {
			*patch.Alignment = aligned
		}
	}
	if patch.Background != nil {
		background := normalizeBackground(*patch.Background)
		if background != "" {
			*patch.Background = background
		}
	}
	if patch.Metadata != nil {
		if value, ok := patch.Metadata["builder_stage"]; ok {
			stage := normalizeBuilderStage(fmt.Sprint(value))
			if stage != "" {
				patch.Metadata["builder_stage"] = stage
			}
		}
		if value, ok := patch.Metadata["creation_method"]; ok {
			method := strings.ToLower(strings.TrimSpace(fmt.Sprint(value)))
			switch method {
			case "rolled", "standard", "point_buy":
				patch.Metadata["creation_method"] = method
			}
		}
	}
}

func normalizeAlignment(value string) string {
	raw := strings.ToLower(strings.TrimSpace(value))
	if raw == "" {
		return ""
	}
	parts := strings.Fields(raw)
	raw = strings.Join(parts, " ")
	switch raw {
	case "lawful good":
		return "Rechtschaffen Gut"
	case "neutral good":
		return "Neutral Gut"
	case "chaotic good":
		return "Chaotisch Gut"
	case "lawful neutral":
		return "Rechtschaffen Neutral"
	case "true neutral":
		return "Neutral"
	case "neutral":
		return "Neutral"
	case "chaotic neutral":
		return "Chaotisch Neutral"
	case "lawful evil":
		return "Rechtschaffen Böse"
	case "neutral evil":
		return "Neutral Böse"
	case "chaotic evil":
		return "Chaotisch Böse"
	default:
		return strings.TrimSpace(value)
	}
}

func normalizeBackground(value string) string {
	raw := strings.ToLower(strings.TrimSpace(value))
	if raw == "" {
		return ""
	}
	parts := strings.Fields(raw)
	raw = strings.Join(parts, " ")
	switch raw {
	case "acolyte":
		return "Akolyth"
	default:
		return strings.TrimSpace(value)
	}
}

func isAllowedBackgroundName(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && len(value) <= 96 && !strings.ContainsAny(value, "\r\n\t")
}

func latestBuilderUserMessage(messages []CharacterBuilderMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if strings.EqualFold(strings.TrimSpace(messages[i].Role), "user") {
			return strings.TrimSpace(messages[i].Content)
		}
	}
	return ""
}

func (h *Handler) rolloverCharacterBuilderSessionIfNeeded(ctx context.Context, character *Character, llmSession LLMSession, documents []Document, transcript []CharacterBuilderMessage) (LLMSession, []CharacterBuilderMessage, error) {
	if h.estimateBuilderPromptTokens(character, &llmSession, documents, transcript) <= builderSessionRolloverThreshold {
		return llmSession, transcript, nil
	}

	recentTranscript := transcript
	if len(recentTranscript) > builderRecentTurnsAfterRollover {
		recentTranscript = recentTranscript[len(recentTranscript)-builderRecentTurnsAfterRollover:]
	}
	now := time.Now().UTC()
	llmSession.Status = "archived"
	llmSession.ArchivedAt = &now
	llmSession.LastActiveAt = now
	llmSession.WorkingSummary = mergeBuilderRuntimeSummary(llmSession.WorkingSummary, *character, documents, transcript)
	if _, err := h.store.UpdateLLMSession(ctx, llmSession); err != nil {
		return LLMSession{}, nil, err
	}

	newSession, err := h.store.CreateLLMSession(ctx, LLMSession{
		SessionType:      "character_builder_session",
		ScopeType:        "character",
		ScopeID:          character.ID,
		RequestProfile:   "builder",
		RulesetWork:      safeOptionalString(character.Metadata["ruleset_work"]),
		RulesetVersion:   safeOptionalString(character.Metadata["ruleset_version"]),
		Status:           "active",
		MessageHistory:   builderMessagesToMetadata(recentTranscript),
		WorkingSummary:   mergeBuilderRuntimeSummary(llmSession.WorkingSummary, *character, documents, transcript),
		StructuredState:  mergeBuilderStructuredState(llmSession.StructuredState, *character),
		TokenBudget:      128000,
		LiveTurnWindow:   8,
		SummaryVersion:   max(llmSession.SummaryVersion+1, 1),
		LastActiveAt:     now,
		LastSummarizedAt: &now,
	})
	if err != nil {
		return LLMSession{}, nil, err
	}
	if character.Metadata == nil {
		character.Metadata = map[string]any{}
	}
	character.Metadata["llm_session_id"] = newSession.ID
	if _, err := h.store.UpdateCharacter(ctx, *character); err != nil {
		return LLMSession{}, nil, err
	}
	return newSession, recentTranscript, nil
}

type builderRetrievalChunk struct {
	DocumentID   string
	DocumentName string
	ChunkIndex   int
	ChunkText    string
	Query        string
	Weight       int
}

func (h *Handler) retrieveBuilderContext(ctx context.Context, documents []Document, query string, limit int) string {
	evidence := h.retrieveBuilderEvidence(ctx, documents, query, limit)
	if len(evidence) == 0 {
		return "(kein passender Regelkontext in den aktuell ausgewaehlten Buechern gefunden)"
	}

	lines := make([]string, 0, len(evidence))
	for _, item := range evidence {
		lines = append(lines, fmt.Sprintf("- %s [%d]: %s", item.DocumentName, item.ChunkIndex+1, compactBuilderChunk(item.ChunkText)))
	}
	return strings.Join(lines, "\n")
}

func (h *Handler) retrieveBuilderEvidence(ctx context.Context, documents []Document, query string, limit int) []builderRetrievalChunk {
	if limit <= 0 {
		limit = 6
	}
	query = strings.TrimSpace(query)
	if query == "" || len(documents) == 0 {
		return []builderRetrievalChunk{}
	}

	documentIDs := builderRelevantDocumentIDs(documents)
	if len(documentIDs) == 0 {
		return []builderRetrievalChunk{}
	}

	variants := builderSearchVariants(query)
	if len(variants) == 0 {
		variants = []string{query}
	}

	type seed struct {
		chunk GMContextChunk
		query string
		rank  int
	}

	seedMap := map[string]seed{}
	seeds := make([]seed, 0, len(variants)*4)
	for rank, variant := range variants {
		chunks, err := h.store.RetrieveRelevantChunksForDocuments(ctx, documentIDs, variant, 4, true)
		if err != nil {
			continue
		}
		for _, chunk := range chunks {
			key := builderRetrievalSeedKey(chunk.DocumentID, chunk.ChunkText)
			if _, ok := seedMap[key]; ok {
				continue
			}
			item := seed{
				chunk: chunk,
				query: variant,
				rank:  rank,
			}
			seedMap[key] = item
			seeds = append(seeds, item)
		}
	}

	if len(seeds) == 0 {
		return []builderRetrievalChunk{}
	}

	docChunksByID := map[string][]DocumentChunk{}
	resultsByKey := map[string]builderRetrievalChunk{}
	for _, seed := range seeds {
		chunks, ok := docChunksByID[seed.chunk.DocumentID]
		if !ok {
			var err error
			chunks, err = h.store.ListDocumentChunks(ctx, seed.chunk.DocumentID)
			if err != nil || len(chunks) == 0 {
				docChunksByID[seed.chunk.DocumentID] = []DocumentChunk{}
				continue
			}
			docChunksByID[seed.chunk.DocumentID] = chunks
		}

		index := matchBuilderChunkIndex(seed.chunk.ChunkText, chunks)
		if index < 0 {
			index = bestBuilderChunkIndex(seed.chunk.ChunkText, chunks)
		}
		if index < 0 {
			continue
		}

		for offset := -1; offset <= 1; offset++ {
			pos := index + offset
			if pos < 0 || pos >= len(chunks) {
				continue
			}
			weight := 100 - (seed.rank * 15) - (absInt(offset) * 20)
			if offset == 0 {
				weight += 10
			}
			key := fmt.Sprintf("%s:%d", seed.chunk.DocumentID, pos)
			current := builderRetrievalChunk{
				DocumentID:   seed.chunk.DocumentID,
				DocumentName: seed.chunk.DocumentName,
				ChunkIndex:   pos,
				ChunkText:    strings.TrimSpace(chunks[pos].ChunkText),
				Query:        seed.query,
				Weight:       weight,
			}
			if existing, ok := resultsByKey[key]; !ok || current.Weight > existing.Weight {
				resultsByKey[key] = current
			}
		}
	}

	results := make([]builderRetrievalChunk, 0, len(resultsByKey))
	for _, item := range resultsByKey {
		results = append(results, item)
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Weight == results[j].Weight {
			if results[i].DocumentName == results[j].DocumentName {
				return results[i].ChunkIndex < results[j].ChunkIndex
			}
			return results[i].DocumentName < results[j].DocumentName
		}
		return results[i].Weight > results[j].Weight
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

func builderRelevantDocumentIDs(documents []Document) []string {
	ids := make([]string, 0, len(documents))
	for _, document := range documents {
		if strings.EqualFold(strings.TrimSpace(document.Type), "rules") || strings.EqualFold(strings.TrimSpace(fmt.Sprint(document.Metadata["system_document"])), "true") {
			ids = append(ids, document.ID)
		}
	}
	if len(ids) == 0 {
		for _, document := range documents {
			ids = append(ids, document.ID)
		}
	}
	return ids
}

func builderRetrievalSeedKey(documentID string, chunkText string) string {
	return documentID + "|" + normalizeBuilderSearchText(chunkText)
}

func normalizeBuilderSearchText(value string) string {
	return slugifySearch(strings.TrimSpace(value))
}

func matchBuilderChunkIndex(seedText string, chunks []DocumentChunk) int {
	seedNorm := normalizeBuilderSearchText(seedText)
	if seedNorm == "" {
		return -1
	}
	for index, chunk := range chunks {
		if normalizeBuilderSearchText(chunk.ChunkText) == seedNorm {
			return index
		}
	}
	for index, chunk := range chunks {
		chunkNorm := normalizeBuilderSearchText(chunk.ChunkText)
		if chunkNorm == "" {
			continue
		}
		if strings.Contains(chunkNorm, seedNorm) || strings.Contains(seedNorm, chunkNorm) {
			return index
		}
	}
	return -1
}

func bestBuilderChunkIndex(seedText string, chunks []DocumentChunk) int {
	seedTerms := uniqueSearchTerms(seedText)
	if len(seedTerms) == 0 {
		return -1
	}
	bestIndex := -1
	bestScore := 0
	for index, chunk := range chunks {
		score := scoreChunkTerms(chunk.ChunkText, seedTerms)
		if score > bestScore {
			bestScore = score
			bestIndex = index
		}
	}
	if bestScore <= 0 {
		return -1
	}
	return bestIndex
}

func builderSearchVariants(query string) []string {
	query = strings.TrimSpace(query)
	if query == "" {
		return []string{}
	}

	terms := uniqueSearchTerms(query)
	hints := builderSearchHints(query)
	seen := map[string]struct{}{}
	variants := make([]string, 0, 4)
	add := func(value string) {
		value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
		if value == "" {
			return
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		variants = append(variants, value)
	}

	add(query)
	if len(terms) > 0 {
		add(strings.Join(append(terms, hints...), " "))
	}
	if len(hints) > 0 {
		add(strings.Join(hints, " "))
	}
	return variants
}

func builderSearchHints(query string) []string {
	lower := strings.ToLower(query)
	hints := make([]string, 0, 8)
	add := func(value string) {
		if value == "" {
			return
		}
		for _, existing := range hints {
			if existing == value {
				return
			}
		}
		hints = append(hints, value)
	}

	if strings.Contains(lower, "rass") || strings.Contains(lower, "volk") || strings.Contains(lower, "species") {
		add("rasse volk race species")
	}
	if strings.Contains(lower, "weis") || strings.Contains(lower, "wisdom") || strings.Contains(lower, "konstit") {
		add("weisheit wisdom konstitution constitution ability score increase bonus")
	}
	if strings.Contains(lower, "waehl") || strings.Contains(lower, "wähl") || strings.Contains(lower, "auswaehl") || strings.Contains(lower, "auswähl") ||
		strings.Contains(lower, "attribute") || strings.Contains(lower, "boni") || strings.Contains(lower, "erhöhen") || strings.Contains(lower, "erhoehen") {
		add("halbelf half elf nach wahl ability score increase")
	}
	if strings.Contains(lower, "dunkel") || strings.Contains(lower, "darkvision") || strings.Contains(lower, "sicht") {
		add("dunkelsicht darkvision sicht reichweite")
	}
	if strings.Contains(lower, "beweg") || strings.Contains(lower, "speed") || strings.Contains(lower, "fuß") || strings.Contains(lower, "feet") {
		add("bewegung speed fuß feet")
	}
	if strings.Contains(lower, "liste") || strings.Contains(lower, "welche") || strings.Contains(lower, "aufzähl") {
		add("liste optionen auflisten")
	}
	if strings.Contains(lower, "klasse") || strings.Contains(lower, "subclass") || strings.Contains(lower, "stufe") {
		add("klasse stufe subclass level")
	}
	if strings.Contains(lower, "hintergrund") || strings.Contains(lower, "background") {
		add("hintergrund background feat fertigkeiten tools")
	}
	if strings.Contains(lower, "fertig") || strings.Contains(lower, "skill") {
		add("skill proficiency fertigkeiten klassenfertigkeiten")
	}
	if strings.Contains(lower, "sprache") || strings.Contains(lower, "language") {
		add("languages sprachen race class background choice")
	}
	if strings.Contains(lower, "rettungs") || strings.Contains(lower, "saving throw") {
		add("saving throw proficiency rettungswurf proficiency")
	}
	if strings.Contains(lower, "trefferpunkt") || strings.Contains(lower, "hp") || strings.Contains(lower, "hit point") {
		add("level 1 hit points class hit die constitution modifier")
	}
	return hints
}

func uniqueSearchTerms(input string) []string {
	parts := strings.FieldsFunc(strings.ToLower(input), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r >= 'ä' && r <= 'ü' || r == 'ß')
	})
	seen := map[string]struct{}{}
	terms := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if len(part) < 3 {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		terms = append(terms, part)
	}
	return terms
}

func scoreChunkTerms(chunk string, terms []string) int {
	text := strings.ToLower(chunk)
	score := 0
	for _, term := range terms {
		if strings.Contains(text, term) {
			score++
		}
	}
	return score
}

func compactBuilderChunk(text string) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if len(text) <= 220 {
		return text
	}
	return strings.TrimSpace(text[:220]) + " ..."
}

func normalizeBuilderStage(stage string) string {
	switch strings.ToLower(strings.TrimSpace(stage)) {
	case "", "not_started":
		return ""
	case "ability_scores_review":
		return "ability_scores"
	case "complete":
		return "review"
	default:
		return strings.TrimSpace(stage)
	}
}

func builderStageTitle(stage string) string {
	switch normalizeBuilderStage(stage) {
	case "concept":
		return "Konzept"
	case "race":
		return "Volk"
	case "class_and_level":
		return "Klasse und Stufe"
	case "background_and_alignment":
		return "Hintergrund und Gesinnung"
	case "ability_method":
		return "Attributsmethode"
	case "ability_scores":
		return "Attribute"
	case "class_proficiencies_and_choices":
		return "Klassenentscheidungen"
	case "hit_points_hit_dice_and_movement":
		return "Trefferpunkte und Bewegung"
	case "languages_senses_and_body":
		return "Sprache, Sinne und Körper"
	case "personality":
		return "Persönlichkeit"
	case "equipment_and_money":
		return "Ausrüstung und Geld"
	case "class_features_not_spells":
		return "Klassenmerkmale"
	case "spellcasting_if_available":
		return "Zauber und Magie"
	case "derived_stats":
		return "Abgeleitete Werte"
	case "combat":
		return "Kampf"
	case "review":
		return "Review"
	default:
		return "Builder"
	}
}

func builderStageOrder() []string {
	return []string{
		"concept",
		"race",
		"class_and_level",
		"background_and_alignment",
		"ability_method",
		"ability_scores",
		"class_proficiencies_and_choices",
		"hit_points_hit_dice_and_movement",
		"languages_senses_and_body",
		"personality",
		"equipment_and_money",
		"class_features_not_spells",
		"spellcasting_if_available",
		"derived_stats",
		"combat",
		"review",
	}
}

func nextBuilderStage(stage string) string {
	current := normalizeBuilderStage(stage)
	if current == "" {
		return "concept"
	}
	order := builderStageOrder()
	for index, item := range order {
		if item == current && index+1 < len(order) {
			return order[index+1]
		}
	}
	return "review"
}

func inferBuilderStageFromCharacter(character Character) string {
	metadata := defaultMetadata(character.Metadata)
	metadataHasText := func(key string) bool {
		value, ok := metadata[key]
		if !ok || value == nil {
			return false
		}
		text := strings.TrimSpace(fmt.Sprint(value))
		return text != "" && text != "<nil>" && text != "[]" && text != "{}"
	}
	metadataHasList := func(key string) bool {
		return len(stringListFromAny(metadata[key])) > 0
	}
	if !metadataHasText("concept") {
		return "concept"
	}
	if strings.TrimSpace(character.Race) == "" {
		return "race"
	}
	if strings.TrimSpace(character.ClassAndLevel) == "" {
		return "class_and_level"
	}
	if strings.TrimSpace(character.Background) == "" || strings.TrimSpace(character.Alignment) == "" {
		return "background_and_alignment"
	}
	if !metadataHasText("creation_method") {
		return "ability_method"
	}
	if len(character.Abilities) == 0 {
		return "ability_scores"
	}
	if builderHasPendingRaceChoice(character) {
		return "ability_scores"
	}
	if !metadataHasList("skill_proficiencies") || !metadataHasList("saving_throw_proficiencies") {
		return "class_proficiencies_and_choices"
	}
	if character.HitPointMax == nil || strings.TrimSpace(character.Speed) == "" || !metadataHasText("hit_dice") {
		return "hit_points_hit_dice_and_movement"
	}
	if !metadataHasText("age") &&
		!metadataHasText("size") &&
		!metadataHasText("weight") &&
		!metadataHasText("eyes") &&
		!metadataHasText("skin") &&
		!metadataHasText("hair") &&
		!metadataHasText("senses") {
		return "languages_senses_and_body"
	}
	if !metadataHasText("personality_traits") &&
		!metadataHasText("ideals") &&
		!metadataHasText("bonds") &&
		!metadataHasText("flaws") {
		return "personality"
	}
	if !metadataHasList("starting_equipment") &&
		!metadataHasText("starting_money") &&
		!metadataHasList("current_inventory") {
		return "equipment_and_money"
	}
	if len(character.Features) == 0 {
		return "class_features_not_spells"
	}
	if !metadataHasList("spells") && builderHasLevelOneSpellcasting(character) {
		return "spellcasting_if_available"
	}
	if strings.TrimSpace(character.Proficiency) == "" || character.ArmorClass == nil {
		return "derived_stats"
	}
	if !metadataHasList("combat_attacks") {
		return "combat"
	}
	return "review"
}

func currentBuilderStage(character Character, session *LLMSession) string {
	if stage := normalizeBuilderStage(safeOptionalString(character.Metadata["builder_stage"])); stage != "" {
		return stage
	}
	if session != nil {
		if stage := normalizeBuilderStage(safeOptionalString(session.StructuredState["builder_stage"])); stage != "" {
			return stage
		}
	}
	return inferBuilderStageFromCharacter(character)
}

func builderHasPendingRaceChoice(character Character) bool {
	race := normalizeBuilderIntentText(character.Race)
	metadata := defaultMetadata(character.Metadata)
	if strings.Contains(race, "halbelf") || strings.Contains(race, "half elf") || strings.Contains(race, "halfelf") {
		return len(stringListFromAny(metadata["race_bonus_choices"])) < 2
	}
	return false
}

func builderStageHiddenChecks(stage string) []string {
	switch normalizeBuilderStage(stage) {
	case "concept":
		return []string{"Konzept bleibt offen und frei formuliert", "keine Regelpruefung noetig"}
	case "race":
		return []string{"Rassenboni intern mitdenken", "Speziesmerkmale und Bewegung still validieren", "keine Bonus-Diskussion im Chat"}
	case "class_and_level":
		return []string{"Klassenstufe pruefen", "Subclass erst beim erlaubten Level freigeben", "keine verfruehten Features zeigen"}
	case "background_and_alignment":
		return []string{"Hintergrundboni intern anwenden", "Gesinnung normalisieren", "kein Regeltext im Chat"}
	case "ability_method":
		return []string{"Attributsmethode festhalten", "keine spaeteren Felder vorziehen"}
	case "ability_scores":
		return []string{"Rassen- und Hintergrundboni still einrechnen", "Attributszuordnung gegen Regeln validieren"}
	case "class_proficiencies_and_choices":
		return []string{"Skills, Saves und Klassenwahl gegen Klasse und Stufe pruefen", "keine unpassenden Features eintragen"}
	case "hit_points_hit_dice_and_movement":
		return []string{"HP, Hit Dice und Speed intern plausibilisieren"}
	case "languages_senses_and_body":
		return []string{"Sprachen und Sinne gegen Volk und Klasse plausibilisieren"}
	case "personality":
		return []string{"nur Persoenlichkeitsfelder aktualisieren"}
	case "equipment_and_money":
		return []string{"Ausruestung und Geld intern zusammenfuehren"}
	case "class_features_not_spells":
		return []string{"Klassenmerkmale nicht als Zauber behandeln", "verfruehte Features still ignorieren"}
	case "spellcasting_if_available":
		return []string{"Zauber nur bei echter Berechtigung zulassen"}
	case "derived_stats":
		return []string{"Proficiency, AC, passive Werte und Attack-Boni aus dem aktuellen Stand ableiten"}
	case "combat":
		return []string{"Angriffszeilen und Schadenswerte gegen Ausruestung und Attribute validieren"}
	case "review":
		return []string{"finalen Draft intern auf Vollstaendigkeit pruefen"}
	default:
		return []string{"Builder intern konsistent halten"}
	}
}

func builderStageAllowedMetadataKeys(stage string) map[string]bool {
	keys := map[string]bool{
		"builder_stage":   true,
		"builder_status":  true,
		"creation_method": true,
		"concept":         true,
	}
	switch normalizeBuilderStage(stage) {
	case "concept":
		keys["concept"] = true
	case "race":
		keys["race"] = true
	case "class_and_level":
		keys["class_and_level"] = true
	case "background_and_alignment":
		keys["background"] = true
		keys["alignment"] = true
		keys["background_skill_proficiencies"] = true
		keys["skill_proficiencies"] = true
	case "ability_method":
		keys["creation_method"] = true
	case "ability_scores":
		keys["resolved_values"] = true
		keys["suggested_assignment"] = true
		keys["creation_method"] = true
		keys["race_bonus_choices"] = true
	case "class_proficiencies_and_choices":
		keys["class_skill_proficiencies"] = true
		keys["skill_proficiencies"] = true
		keys["saving_throw_proficiencies"] = true
		keys["tools_and_proficiencies"] = true
		keys["weapon_notes"] = true
		keys["combat_overview"] = true
	case "hit_points_hit_dice_and_movement":
		keys["hit_dice"] = true
	case "languages_senses_and_body":
		keys["age"] = true
		keys["size"] = true
		keys["weight"] = true
		keys["eyes"] = true
		keys["skin"] = true
		keys["hair"] = true
		keys["senses"] = true
	case "personality":
		keys["personality_traits"] = true
		keys["ideals"] = true
		keys["bonds"] = true
		keys["flaws"] = true
	case "equipment_and_money":
		keys["starting_equipment"] = true
		keys["tools_and_proficiencies"] = true
		keys["weapon_notes"] = true
		keys["combat_overview"] = true
		keys["starting_money"] = true
		keys["current_money"] = true
		keys["current_inventory"] = true
		keys["level_up_available"] = true
	case "class_features_not_spells":
		keys["class_features"] = true
	case "spellcasting_if_available":
		keys["spells"] = true
		keys["spell_notes"] = true
		keys["spell_attacks"] = true
	case "derived_stats":
		keys["spell_save_dc"] = true
		keys["spell_attack_bonus"] = true
	case "combat":
		keys["combat_attacks"] = true
	case "review":
		keys["builder_stage"] = true
		keys["builder_status"] = true
	}
	return keys
}

func sanitizeCharacterBuilderPatchForStage(patch *CharacterBuilderPatch, stage string) {
	if patch == nil {
		return
	}
	allowedMetadata := builderStageAllowedMetadataKeys(stage)
	allowedTopLevel := map[string]bool{
		"name":          true,
		"player_name":   true,
		"builder_stage": true,
	}
	switch normalizeBuilderStage(stage) {
	case "race":
		allowedTopLevel["race"] = true
	case "class_and_level":
		allowedTopLevel["class_and_level"] = true
	case "background_and_alignment":
		allowedTopLevel["background"] = true
		allowedTopLevel["alignment"] = true
	case "ability_scores":
		allowedTopLevel["abilities"] = true
	case "class_proficiencies_and_choices":
		allowedTopLevel["languages"] = true
		allowedTopLevel["features"] = true
	case "hit_points_hit_dice_and_movement":
		allowedTopLevel["hit_point_max"] = true
		allowedTopLevel["speed"] = true
		allowedTopLevel["proficiency_bonus"] = true
	case "languages_senses_and_body":
		allowedTopLevel["name"] = true
		allowedTopLevel["languages"] = true
	case "personality":
		allowedTopLevel["name"] = true
	case "equipment_and_money":
		allowedTopLevel["name"] = true
	case "class_features_not_spells":
		allowedTopLevel["features"] = true
	case "derived_stats":
		allowedTopLevel["proficiency_bonus"] = true
		allowedTopLevel["armor_class"] = true
		allowedTopLevel["hit_point_max"] = true
		allowedTopLevel["speed"] = true
	case "review":
		allowedTopLevel["name"] = true
		allowedTopLevel["player_name"] = true
	}
	if !allowedTopLevel["name"] {
		patch.Name = nil
	}
	if !allowedTopLevel["player_name"] {
		patch.PlayerName = nil
	}
	if !allowedTopLevel["class_and_level"] {
		patch.ClassAndLevel = nil
	}
	if !allowedTopLevel["background"] {
		patch.Background = nil
	}
	if !allowedTopLevel["race"] {
		patch.Race = nil
	}
	if !allowedTopLevel["alignment"] {
		patch.Alignment = nil
	}
	if !allowedTopLevel["armor_class"] {
		patch.ArmorClass = nil
	}
	if !allowedTopLevel["speed"] {
		patch.Speed = nil
	}
	if !allowedTopLevel["hit_point_max"] {
		patch.HitPointMax = nil
	}
	if !allowedTopLevel["proficiency_bonus"] {
		patch.Proficiency = nil
	}
	if !allowedTopLevel["abilities"] {
		patch.Abilities = nil
	}
	if !allowedTopLevel["languages"] {
		patch.Languages = nil
	}
	if !allowedTopLevel["features"] {
		patch.Features = nil
	}
	if patch.Background != nil {
		background := normalizeBackground(*patch.Background)
		if !isAllowedBackgroundName(background) {
			patch.Background = nil
		} else {
			*patch.Background = background
		}
	}
	if len(patch.Metadata) > 0 {
		filtered := map[string]any{}
		for key, value := range patch.Metadata {
			normalizedKey := strings.TrimSpace(key)
			if normalizedKey == "" {
				continue
			}
			if allowedMetadata[normalizedKey] {
				filtered[normalizedKey] = value
			}
		}
		patch.Metadata = filtered
	}
	if len(patch.Metadata) == 0 {
		patch.Metadata = nil
	}
}

func builderCharacterLanguage(character *Character) string {
	if character != nil {
		language := strings.ToLower(strings.TrimSpace(fmt.Sprint(character.Metadata["language"])))
		if language == "en" || language == "de" {
			return language
		}
	}
	return "de"
}

func builderFallbackReply(stage string, character *Character, language string) string {
	if language != "de" {
		switch normalizeBuilderStage(stage) {
		case "concept":
			return "Briefly describe the character you want to play. A role, feeling, or image is enough for the next step."
		case "race":
			return "Which ancestry should the character have? I will then apply the appropriate starting traits."
		case "class_and_level":
			return "Which class and starting level should the character have?"
		case "background_and_alignment":
			return "Choose the rules background and alignment. Under SRD 5.1 you may use Acolyte or create a custom background with two skills and a total of two languages or tool proficiencies."
		case "ability_method":
			return "Choose the ability-score method: standard array, point buy, or rolling."
		case "ability_scores":
			return "Complete the ability scores and any remaining ancestry bonus choices."
		case "class_proficiencies_and_choices":
			return "Next, choose the available class proficiencies and starting options."
		case "hit_points_hit_dice_and_movement":
			return "I will derive hit points, Hit Dice, and speed from the class, ancestry, and abilities."
		case "languages_senses_and_body":
			return "Now choose any remaining languages, senses, and physical details."
		case "personality":
			return "Now define personality traits, ideals, bonds, and flaws."
		case "equipment_and_money":
			return "Now record starting equipment and money."
		case "class_features_not_spells":
			return "Now record the class features available at this level."
		case "spellcasting_if_available":
			return "If this character can cast spells at this level, choose the available magic now."
		case "derived_stats":
			return "I will now verify and calculate the derived statistics."
		case "combat":
			return "Finally, record attacks and combat details."
		case "review":
			return "The character is almost complete. Review the sheet for any missing choices."
		default:
			return "Tell me the next clear choice for this character."
		}
	}
	switch normalizeBuilderStage(stage) {
	case "concept":
		return "Erzähl mir kurz, was für eine Figur du spielen möchtest: eine Rolle, ein Gefühl oder ein Bild reichen für den nächsten Schritt."
	case "race":
		return "Welches Volk soll die Figur haben? Dann bauen wir die passenden Grundwerte direkt sauber ein."
	case "class_and_level":
		return "Welche Klasse und welche Stufe soll die Figur haben? Dann machen wir mit den Klassengrundlagen weiter."
	case "background_and_alignment":
		return "Jetzt ist der regeltechnische Hintergrund dran. Im SRD 5.1 kannst du Akolyth wählen oder nach der offiziellen Anpassungsregel einen eigenen Hintergrund mit zwei Fertigkeiten und insgesamt zwei Sprachen oder Werkzeugübungen erstellen. Lege zuerst den Hintergrund fest, danach die Gesinnung; die narrative Hintergrundgeschichte kommt getrennt davon."
	case "ability_method":
		return "Lege jetzt die Attributsmethode fest: Standardwerte, Point Buy oder Würfeln."
	case "ability_scores":
		if character != nil && builderHasPendingRaceChoice(*character) {
			return builderAbilityStageGuidanceReply(*character)
		}
		return "Jetzt werden die Attribute abgeschlossen. Nenne die Verteilung oder die noch offenen Bonusentscheidungen, dann trage ich sie direkt ein."
	case "class_proficiencies_and_choices":
		if character != nil {
			if reply, ok := builderDeterministicClassChoicesReply(*character); ok {
				return reply
			}
		}
		return "Jetzt folgen die Klassenentscheidungen. Ich nenne nur die Optionen, die für diesen Stand nach Regeln wirklich offen sind."
	case "hit_points_hit_dice_and_movement":
		if character != nil {
			if raceRule, ok := builderRaceRuleForCharacter(*character); ok {
				parts := []string{}
				if character.HitPointMax != nil {
					parts = append(parts, fmt.Sprintf("Die Trefferpunkte sind festgelegt: %d.", *character.HitPointMax))
				}
				if raceRule.Speed != "" {
					parts = append(parts, fmt.Sprintf("Die Bewegungsrate beträgt %s.", raceRule.Speed))
				}
				if len(raceRule.FixedLanguages) > 0 {
					parts = append(parts, fmt.Sprintf("Als %s sprichst du fest %s.", raceRule.RaceName, joinGermanList(raceRule.FixedLanguages)))
				}
				if raceRule.ExtraLanguageChoiceCount > 0 {
					parts = append(parts, fmt.Sprintf("Lege jetzt %d zusätzliche Sprache nach Wahl fest.", raceRule.ExtraLanguageChoiceCount))
				} else {
					parts = append(parts, "Lege jetzt Sinne und Körperdaten der Figur fest.")
				}
				return strings.Join(parts, " ")
			}
		}
		return "Trefferpunkte, Trefferwürfel und Bewegung leite ich jetzt aus Klasse, Volk und Attributen ab. Lege danach direkt Sprache, Sinne und die Körperdaten der Figur fest."
	case "languages_senses_and_body":
		return "Jetzt fehlen nur noch Sprache, Sinne und die Körperdaten der Figur."
	case "personality":
		return "Lass uns der Figur jetzt Persönlichkeit geben: Merkmale, Ideale, Bindungen und Makel."
	case "equipment_and_money":
		return "Jetzt tragen wir die Ausrüstung und das Geld sauber ein."
	case "class_features_not_spells":
		return "Dann sammeln wir die Klassenmerkmale ein, die auf diesem Stand wirklich schon gelten."
	case "spellcasting_if_available":
		return "Falls diese Figur auf diesem Stand wirklich zaubern kann, gehen wir jetzt die Magie durch."
	case "derived_stats":
		return "Ich prüfe jetzt die abgeleiteten Werte und ziehe alles intern gerade, was aus Klasse, Volk und Ausrüstung folgt."
	case "combat":
		return "Zum Schluss halte ich die Angriffe und Kampfdaten sauber fest."
	case "review":
		return "Die Figur wirkt fast fertig. Lass uns kurz prüfen, ob noch ein letzter Punkt fehlt, bevor wir abschließen."
	default:
		if character != nil && strings.TrimSpace(character.Name) != "" {
			return fmt.Sprintf("Lass uns mit %s weitermachen: Sag mir einfach den nächsten klaren Schritt.", character.Name)
		}
		return "Lass uns mit dem nächsten klaren Schritt weitermachen."
	}
}

func mustJSON(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

func compactCharacterDraftForPrompt(character Character) map[string]any {
	metadata := defaultMetadata(character.Metadata)
	delete(metadata, "builder_messages")
	delete(metadata, "llm_session_id")
	delete(metadata, "level_up_session_id")
	delete(metadata, "builder_runtime_context")

	return map[string]any{
		"identity": map[string]any{
			"name":        character.Name,
			"player_name": character.PlayerName,
			"alignment":   character.Alignment,
		},
		"progression": map[string]any{
			"class_and_level": character.ClassAndLevel,
			"background":      character.Background,
			"race":            character.Race,
		},
		"core_stats": map[string]any{
			"armor_class":        character.ArmorClass,
			"speed":              character.Speed,
			"hit_point_max":      character.HitPointMax,
			"proficiency":        character.Proficiency,
			"passive_perception": metadata["passive_perception"],
		},
		"abilities": character.Abilities,
		"languages": character.Languages,
		"features":  character.Features,
		"metadata":  compactCharacterMetadataForPrompt(metadata),
	}
}

func compactCharacterMetadataForPrompt(metadata map[string]any) map[string]any {
	source := defaultMetadata(metadata)
	if len(source) == 0 {
		return map[string]any{}
	}

	allowed := []string{
		"builder_stage",
		"builder_status",
		"creation_method",
		"race_bonus_choices",
		"concept",
		"personality_traits",
		"ideals",
		"bonds",
		"flaws",
		"backstory",
		"age",
		"size",
		"weight",
		"eyes",
		"skin",
		"hair",
		"senses",
		"background_skill_proficiencies",
		"class_skill_proficiencies",
		"skill_proficiencies",
		"saving_throw_proficiencies",
		"starting_equipment",
		"current_inventory",
		"current_money",
		"tools_and_proficiencies",
		"weapon_notes",
		"combat_overview",
		"combat_attacks",
		"experience_points",
		"level_up_available",
		"hit_dice",
		"spell_save_dc",
		"spell_attack_bonus",
		"spell_attacks",
		"spell_notes",
		"passive_perception",
	}

	result := make(map[string]any)
	for _, key := range allowed {
		value, ok := source[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case string:
			if text := strings.TrimSpace(typed); text != "" {
				result[key] = text
			}
		case []string:
			if len(typed) > 0 {
				result[key] = typed
			}
		case map[string]any:
			if len(typed) > 0 {
				result[key] = typed
			}
		case []any:
			if len(typed) > 0 {
				result[key] = typed
			}
		default:
			result[key] = value
		}
	}
	return result
}

func builderRulesContext(character Character, stage string, latestUserMessage string, retrievalEvidence []builderRetrievalChunk) map[string]any {
	stage = normalizeBuilderStage(stage)
	classRule, hasClass := builderClassRuleForCharacter(character)
	raceRule, hasRace := builderRaceRuleForCharacter(character)

	fixedRules := make([]string, 0, 6)
	playerChoices := make([]string, 0, 6)
	derived := make([]string, 0, 6)
	sources := make([]map[string]any, 0, 3)

	addFixed := func(value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			fixedRules = append(fixedRules, value)
		}
	}
	addChoice := func(value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			playerChoices = append(playerChoices, value)
		}
	}
	addDerived := func(value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			derived = append(derived, value)
		}
	}

	if hasClass {
		addFixed(fmt.Sprintf("%s: Rettungswürfe %s.", classRule.ClassName, joinGermanList(classRule.SavingThrows)))
		if classRule.HitDie != "" {
			addFixed(fmt.Sprintf("%s: Trefferwürfel %s.", classRule.ClassName, classRule.HitDie))
		}
	}
	if hasRace {
		if raceRule.Speed != "" {
			addFixed(fmt.Sprintf("%s: Bewegungsrate %s.", raceRule.RaceName, raceRule.Speed))
		}
		if raceRule.Darkvision != "" {
			addFixed(fmt.Sprintf("%s: Dunkelsicht %s.", raceRule.RaceName, raceRule.Darkvision))
		}
		if len(raceRule.FixedAbilityBonuses) > 0 {
			addFixed(fmt.Sprintf("%s: Volksboni %s.", raceRule.RaceName, joinGermanList(raceRule.FixedAbilityBonuses)))
		}
		if len(raceRule.FixedLanguages) > 0 {
			addFixed(fmt.Sprintf("%s: Feste Sprachen %s.", raceRule.RaceName, joinGermanList(raceRule.FixedLanguages)))
		}
	}

	switch stage {
	case "ability_scores":
		if hasRace && raceRule.FlexibleAbilityChoice != "" {
			addChoice(fmt.Sprintf("%s: %s", raceRule.RaceName, raceRule.FlexibleAbilityChoice))
		}
		if builderHasPendingRaceChoice(character) {
			addChoice("Lege jetzt die noch offenen flexiblen Attributsboni fest, bevor die Attributsphase abgeschlossen werden kann.")
		}
	case "class_proficiencies_and_choices":
		if hasClass && classRule.SkillChoiceCount > 0 {
			if len(classRule.SkillChoices) == 1 && strings.EqualFold(classRule.SkillChoices[0], "Beliebige Fertigkeiten") {
				addChoice(fmt.Sprintf("%s: Wähle %d beliebige Fertigkeiten.", classRule.ClassName, classRule.SkillChoiceCount))
			} else {
				addChoice(fmt.Sprintf("%s: Wähle %d Fertigkeiten aus %s.", classRule.ClassName, classRule.SkillChoiceCount, joinGermanList(classRule.SkillChoices)))
			}
		}
		if hasRace && raceRule.ExtraSkillChoiceCount > 0 {
			addChoice(fmt.Sprintf("%s: Wähle zusätzlich %d beliebige Fertigkeiten.", raceRule.RaceName, raceRule.ExtraSkillChoiceCount))
		}
		if hasRace && raceRule.ExtraLanguageChoiceCount > 0 {
			addChoice(fmt.Sprintf("%s: Wähle zusätzlich %d Sprache nach Wahl.", raceRule.RaceName, raceRule.ExtraLanguageChoiceCount))
		}
	case "hit_points_hit_dice_and_movement":
		if hasClass && classRule.HitDie != "" {
			addDerived(fmt.Sprintf("Trefferpunkte auf Stufe 1 leiten sich aus %s und dem Konstitutionsmodifikator ab.", classRule.HitDie))
		}
		if hasRace && raceRule.Speed != "" {
			addDerived(fmt.Sprintf("Bewegungsrate ist aus dem Volk bereits fest: %s.", raceRule.Speed))
		}
	case "background_and_alignment":
		addChoice("Wähle Akolyth oder erstelle nach der offiziellen SRD-5.1-Anpassungsregel einen eigenen Hintergrund mit zwei Fertigkeiten und insgesamt zwei Sprachen oder Werkzeugübungen. Die Hintergrundgeschichte bleibt davon getrennt.")
	case "languages_senses_and_body":
		if hasRace && raceRule.ExtraLanguageChoiceCount > 0 {
			addChoice(fmt.Sprintf("%s: Wähle %d zusätzliche Sprache nach Wahl.", raceRule.RaceName, raceRule.ExtraLanguageChoiceCount))
		}
	}

	for _, chunk := range retrievalEvidence {
		sourceName := strings.TrimSpace(chunk.DocumentName)
		if sourceName == "" {
			continue
		}
		sources = append(sources, map[string]any{
			"document": sourceName,
			"score":    chunk.Weight,
			"excerpt":  compactBuilderChunk(chunk.ChunkText),
		})
		if len(sources) >= 3 {
			break
		}
	}

	return map[string]any{
		"fixed_rules":             fixedRules,
		"player_choices_required": playerChoices,
		"derived_now":             derived,
		"query_variants":          builderSearchVariants(builderContextQueryForCharacter(character, stage, latestUserMessage)),
		"sources":                 sources,
	}
}

func buildCharacterBuilderPromptContext(character *Character, session *LLMSession, documents []Document, transcript []CharacterBuilderMessage, relevantRulesContext string, builderQuery string, retrievalEvidence []builderRetrievalChunk) map[string]any {
	stage := currentBuilderStage(*character, session)
	raceReference := loadEmbeddedBuilderGuide(
		strings.TrimSpace(fmt.Sprintf("%v", character.Metadata["ruleset_work"])),
		strings.TrimSpace(fmt.Sprintf("%v", character.Metadata["ruleset_version"])),
		"race_reference",
	)
	rulesContext := builderRulesContext(*character, stage, latestBuilderUserMessage(transcript), retrievalEvidence)
	return map[string]any{
		"current_step":        stage,
		"current_step_title":  builderStageTitle(stage),
		"next_step":           nextBuilderStage(stage),
		"step_lock":           true,
		"step_lock_rule":      "Bleibe strikt beim aktuellen Schritt und springe nicht zu spaeteren Feldern, auch wenn der Nutzer danach fragt.",
		"allowed_writes":      builderStageAllowedMetadataKeys(stage),
		"hidden_validation":   builderStageHiddenChecks(stage),
		"fixed_flow":          builderStageOrder(),
		"builder_query":       strings.TrimSpace(builderQuery),
		"relevant_rules":      strings.TrimSpace(relevantRulesContext),
		"retrieval_policy":    "PDF-Treffer sind die Grundlage. Wenn Treffer vorhanden sind, antworte direkt daraus und sage nicht, du habest keinen Zugriff.",
		"retrieval_evidence":  compactBuilderRetrievalEvidenceForPrompt(retrievalEvidence),
		"race_reference":      truncatePromptContextText(raceReference, 2200),
		"rules_context":       rulesContext,
		"open_decisions":      builderOpenDecisions(*character, stage, rulesContext),
		"derived_now":         builderDerivedNow(*character, stage, rulesContext),
		"reply_contract":      builderReplyContract(*character, stage),
		"confirmed_character": compactCharacterDraftForPrompt(*character),
		"selected_documents":  compactSelectedDocumentsForPrompt(documents),
		"recent_turns":        compactBuilderTranscript(transcript, 6),
	}
}

func builderOpenDecisions(character Character, stage string, rulesContext map[string]any) []string {
	stage = normalizeBuilderStage(stage)
	decisions := make([]string, 0, 4)
	if structured := stringListFromAny(rulesContext["player_choices_required"]); len(structured) > 0 {
		return structured
	}
	switch stage {
	case "ability_scores":
		if builderHasPendingRaceChoice(character) {
			decisions = append(decisions, "Halbelf: Lege zwei verschiedene Attribute für die beiden +1-Boni fest. +2 Charisma ist fest.")
		}
	case "class_proficiencies_and_choices":
		if reply, ok := builderDeterministicClassChoicesReply(character); ok {
			decisions = append(decisions, reply)
		}
	case "background_and_alignment":
		decisions = append(decisions, "Lege zuerst den offiziellen Regelwerk-Hintergrund fest. Die Hintergrundgeschichte ist davon getrennt.")
	}
	return decisions
}

func builderDerivedNow(character Character, stage string, rulesContext map[string]any) []string {
	if structured := stringListFromAny(rulesContext["derived_now"]); len(structured) > 0 {
		return structured
	}
	switch normalizeBuilderStage(stage) {
	case "hit_points_hit_dice_and_movement":
		return []string{"Trefferpunkte, Trefferwürfel und Bewegungsrate werden aus Klasse, Volk und Attributen abgeleitet."}
	case "derived_stats":
		return []string{"Übungsbonus, RK und weitere abgeleitete Werte werden intern aus dem aktuellen Stand berechnet."}
	default:
		return []string{}
	}
}

func builderReplyContract(character Character, stage string) string {
	switch normalizeBuilderStage(stage) {
	case "ability_scores":
		if builderHasPendingRaceChoice(character) {
			return "Nenne den aktuellen Stand der Attributsphase, erkläre die noch offene Halbelf-Bonuswahl knapp und fordere dann genau zwei Attribute an."
		}
		return "Wenn nach Optionen oder Empfehlungen gefragt wird, nenne die erlaubten Attributs- oder Bonusentscheidungen und gib einen knappen, passenden Vorschlag, bevor du die konkrete Wahl anforderst."
	case "class_proficiencies_and_choices":
		return "Nenne zuerst feste Klassen- und Volksvorgaben, dann die konkreten Auswahloptionen und mindestens einen passenden Vorschlag, bevor du die Auswahl einforderst."
	case "background_and_alignment":
		return "Trenne regeltechnischen Hintergrund und narrative Geschichte sauber. Nenne Akolyth als einzigen benannten SRD-5.1-Musterhintergrund, liste bei Nachfrage die verfügbaren Fertigkeiten für eigene Hintergründe auf und gib mindestens einen thematisch passenden Vorschlag, bevor du die Auswahl einforderst."
	case "hit_points_hit_dice_and_movement":
		return "Nenne Trefferpunkte, Trefferwürfel und Bewegungsrate knapp als festgelegten Stand und fordere dann direkt die nächste Pflichtentscheidung an, zum Beispiel Sprachen, Sinne oder Körperdaten."
	case "race":
		return "Wenn nach Optionen oder Empfehlungen gefragt wird, nenne die verfügbaren Völker im aktuellen Profil und gib einen knappen, passenden Vorschlag, bevor du die Wahl anforderst."
	case "class_and_level":
		return "Wenn nach Optionen oder Empfehlungen gefragt wird, nenne die verfügbaren Klassen im aktuellen Profil, schlage eine passende Klasse knapp vor und fordere dann Klasse plus Stufe an."
	case "ability_method":
		return "Wenn nach Optionen oder Empfehlungen gefragt wird, nenne Standardwerte, Point Buy und Würfeln, gib eine kurze Empfehlung mit Begründung und fordere dann die konkrete Methode an."
	case "languages_senses_and_body":
		return "Wenn nach Optionen oder Empfehlungen gefragt wird, nenne feste Sprachen und offene Sprachwahlen klar und gib bei offenen Sprachen einen knappen Vorschlag, bevor du die konkrete Wahl anforderst."
	}
	return "Kurz, sachlich, führend: aktueller Schritt, offene Regeloptionen, klare Auswahlaufforderung."
}

func builderContextQuery(stage string, latestUserMessage string) string {
	return builderContextQueryForCharacter(Character{}, stage, latestUserMessage)
}

func builderContextQueryForCharacter(character Character, stage string, latestUserMessage string) string {
	base := strings.TrimSpace(latestUserMessage)
	stage = normalizeBuilderStage(stage)
	selected := make([]string, 0, 3)
	if rule, ok := builderClassRuleForCharacter(character); ok {
		selected = append(selected, rule.ClassName)
	}
	if rule, ok := builderRaceRuleForCharacter(character); ok {
		selected = append(selected, rule.RaceName)
	}
	if background := strings.TrimSpace(character.Background); background != "" {
		selected = append(selected, background)
	}
	selectedText := strings.Join(selected, " ")
	switch stage {
	case "race":
		return strings.TrimSpace(strings.Join([]string{
			base,
			selectedText,
			"Rasse Spezies Volk ability score increase wisdom constitution dexterity bonus",
			"race species ability score increase wisdom constitution bonus",
		}, " "))
	case "background_and_alignment":
		return strings.TrimSpace(strings.Join([]string{
			base,
			selectedText,
			"background alignment ability score increase feat skill tool",
		}, " "))
	case "class_and_level":
		return strings.TrimSpace(strings.Join([]string{
			base,
			selectedText,
			"class level subclass feature saving throw proficiency",
		}, " "))
	case "class_proficiencies_and_choices":
		return strings.TrimSpace(strings.Join([]string{
			base,
			selectedText,
			"class skill proficiency saving throw language tool starting proficiencies",
			"class skill choices race extra skills saving throws languages proficiencies",
		}, " "))
	case "ability_scores":
		return strings.TrimSpace(strings.Join([]string{
			base,
			selectedText,
			"ability scores race background bonus adjustment standard array point buy rolled",
			"half elf ability score increase charisma plus two different abilities of your choice",
			"half-elf asi charisma plus two other ability scores",
		}, " "))
	case "hit_points_hit_dice_and_movement":
		return strings.TrimSpace(strings.Join([]string{
			base,
			selectedText,
			"level 1 hit points hit die movement speed race class",
			"class level 1 hp hit die race speed movement",
		}, " "))
	case "equipment_and_money":
		return strings.TrimSpace(strings.Join([]string{
			base,
			selectedText,
			"starting equipment class background druid equipment package money gear",
		}, " "))
	default:
		return strings.TrimSpace(strings.Join([]string{base, selectedText}, " "))
	}
}

func compactSelectedDocumentsForPrompt(documents []Document) []map[string]any {
	items := make([]map[string]any, 0, len(documents))
	for _, document := range documents {
		if strings.HasSuffix(strings.TrimSpace(fmt.Sprintf("%v", document.Metadata["kind"])), "_guide") {
			continue
		}
		items = append(items, map[string]any{
			"id":   document.ID,
			"name": document.Name,
			"type": document.Type,
		})
	}
	return items
}

func compactBuilderTranscript(messages []CharacterBuilderMessage, keepRecent int) []map[string]any {
	if keepRecent < 0 {
		keepRecent = 0
	}
	trimmed := messages
	if len(trimmed) > keepRecent {
		trimmed = trimmed[len(trimmed)-keepRecent:]
	}
	result := make([]map[string]any, 0, len(trimmed))
	for _, message := range trimmed {
		result = append(result, map[string]any{
			"role":       message.Role,
			"content":    strings.TrimSpace(message.Content),
			"created_at": message.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	return result
}

func compactBuilderRetrievalEvidenceForPrompt(evidence []builderRetrievalChunk) []map[string]any {
	items := make([]map[string]any, 0, len(evidence))
	for _, chunk := range evidence {
		items = append(items, map[string]any{
			"source":      chunk.DocumentName,
			"chunk_index": chunk.ChunkIndex + 1,
			"query":       chunk.Query,
			"weight":      chunk.Weight,
			"excerpt":     compactBuilderChunk(chunk.ChunkText),
		})
	}
	return items
}

func builderQuestionLooksLikeListRequest(message string) bool {
	lower := strings.ToLower(strings.TrimSpace(message))
	if lower == "" {
		return false
	}
	keywords := []string{
		"liste",
		"welche",
		"welcher",
		"welches",
		"welchen",
		"auflisten",
		"aufzähl",
		"verfueg",
		"stehen mir zu",
		"optionen",
		"weish",
		"dunkel",
		"beweg",
		"speed",
		"rass",
		"volk",
		"fertig",
		"skill",
		"sprache",
		"rettungs",
	}
	for _, keyword := range keywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

func builderQuestionLooksLikeRecommendationRequest(message string) bool {
	lower := strings.ToLower(strings.TrimSpace(message))
	if lower == "" {
		return false
	}
	keywords := []string{
		"empfehl",
		"vorschlag",
		"sinn",
		"pass",
		"besten",
		"geeignet",
		"rat",
		"recommend",
		"suggest",
		"fit",
		"good choice",
	}
	for _, keyword := range keywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

func builderCanonicalSkillNames() []string {
	items := make([]string, 0, len(builderCanonicalSkills))
	for _, skill := range builderCanonicalSkills {
		items = append(items, skill.Name)
	}
	return items
}

func builderBackgroundSuggestions(message string, character Character) []string {
	lower := strings.ToLower(message)
	suggestions := []string{}
	if strings.Contains(lower, "ritter") || strings.Contains(lower, "knight") {
		suggestions = append(suggestions,
			"„Athletik und Einschüchtern“ für einen klassischen Ritter",
			"„Geschichte und Überzeugen“ für einen höfischen Ritter",
			"„Mit Tieren umgehen und Überlebenskunst“ für einen reitenden Grenzritter",
		)
	}
	if classRule, ok := builderClassRuleForCharacter(character); ok && strings.EqualFold(classRule.ClassName, "Kämpfer") {
		suggestions = append(suggestions, "„Athletik und Wahrnehmung“ für einen wachsamen Frontkämpfer")
	}
	return uniquePreserveOrder(suggestions)
}

func builderRaceSuggestions(character Character, message string) []string {
	lower := strings.ToLower(message)
	suggestions := []string{}
	if strings.Contains(lower, "ark") || strings.Contains(lower, "mag") || strings.Contains(lower, "zauber") || strings.Contains(lower, "intelligenz") {
		suggestions = append(suggestions, "Hochelf für geschickte arkane Figuren", "Felsgnom für intelligenzbasierte Tüftler")
	}
	if strings.Contains(lower, "ritter") || strings.Contains(lower, "nahkampf") || strings.Contains(lower, "front") {
		suggestions = append(suggestions, "Mensch für einen flexiblen Ritterstart", "Hochelf für einen wendigen Kampfstil", "Halbork für rohe Nahkampfstärke")
	}
	if len(suggestions) == 0 {
		suggestions = append(suggestions, "Mensch für den flexibelsten Start", "Hochelf für Geschicklichkeit plus Intelligenz", "Felsgnom für einen Intelligenzfokus")
	}
	return uniquePreserveOrder(suggestions)
}

func builderClassSuggestions(character Character, message string) []string {
	lower := strings.ToLower(message)
	suggestions := []string{}
	if strings.Contains(lower, "ritter") || strings.Contains(lower, "knight") || strings.Contains(lower, "nahkampf") {
		suggestions = append(suggestions, "Kämpfer, Stufe 1 für einen direkten Ritterstart", "Paladin, Stufe 1 für einen ehrengebundenen Ritter", "Waldläufer, Stufe 1 für einen Grenzritter oder Späher")
	}
	if strings.Contains(lower, "ark") || strings.Contains(lower, "zauber") || strings.Contains(lower, "mag") {
		suggestions = append(suggestions, "Magier, Stufe 1 für klassische Arkana", "Hexenmeister, Stufe 1 für dunklere Magie", "Zauberer, Stufe 1 für angeborene Magie")
	}
	if raceRule, ok := builderRaceRuleForCharacter(character); ok {
		switch raceRule.RaceName {
		case "Hochelf":
			suggestions = append(suggestions, "Magier, Stufe 1 für einen sauberen Intelligenzpfad", "Kämpfer, Stufe 1 für einen wendigen Schwertkämpfer")
		case "Felsgnom":
			suggestions = append(suggestions, "Magier, Stufe 1 für einen starken Intelligenzfokus")
		case "Mensch":
			suggestions = append(suggestions, "Kämpfer, Stufe 1 für einen stabilen Allround-Start")
		}
	}
	if len(suggestions) == 0 {
		suggestions = append(suggestions, "Kämpfer, Stufe 1 für einen robusten Start", "Schurke, Stufe 1 für Geschicklichkeit und Utility", "Magier, Stufe 1 für reinen Arkana-Fokus")
	}
	return uniquePreserveOrder(suggestions)
}

func builderSuggestedSkillSets(character Character, classRule builderClassRule) []string {
	suggestions := []string{}
	background := strings.ToLower(strings.TrimSpace(character.Background))
	switch classRule.ClassName {
	case "Kämpfer":
		suggestions = append(suggestions, "„Athletik und Wahrnehmung“")
		if strings.Contains(background, "ritter") {
			suggestions = append(suggestions, "„Athletik und Einschüchtern“", "„Athletik und Geschichte“")
		}
	case "Paladin":
		suggestions = append(suggestions, "„Athletik und Überzeugen“", "„Athletik und Einschüchtern“")
	case "Waldläufer":
		suggestions = append(suggestions, "„Wahrnehmung und Überlebenskunst“", "„Heimlichkeit und Wahrnehmung“")
	case "Schurke":
		suggestions = append(suggestions, "„Heimlichkeit und Wahrnehmung“", "„Fingerfertigkeit und Täuschung“")
	case "Magier":
		suggestions = append(suggestions, "„Arkane Kunde und Nachforschungen“", "„Arkane Kunde und Geschichte“")
	}
	return uniquePreserveOrder(suggestions)
}

func builderDirectRaceReferenceReply(question string, raceReference string) (string, bool) {
	questionLower := strings.ToLower(strings.TrimSpace(question))
	if questionLower == "" || strings.TrimSpace(raceReference) == "" {
		return "", false
	}
	if !strings.Contains(questionLower, "rass") && !strings.Contains(questionLower, "volk") && !strings.Contains(questionLower, "species") {
		return "", false
	}

	wantWisdom := strings.Contains(questionLower, "weis") || strings.Contains(questionLower, "wisdom")
	wantDarkvision := strings.Contains(questionLower, "dunkel") || strings.Contains(questionLower, "darkvision") || strings.Contains(questionLower, "sicht")
	wantSpeed := strings.Contains(questionLower, "beweg") || strings.Contains(questionLower, "speed") || strings.Contains(questionLower, "fuß") || strings.Contains(questionLower, "feet")
	wantFlexibleBonuses := strings.Contains(questionLower, "wähl") ||
		strings.Contains(questionLower, "waehl") ||
		strings.Contains(questionLower, "auswähl") ||
		strings.Contains(questionLower, "auswaehl") ||
		strings.Contains(questionLower, "choose") ||
		strings.Contains(questionLower, "auswählen") ||
		strings.Contains(questionLower, "attribute erhöhen") ||
		strings.Contains(questionLower, "attribut") ||
		strings.Contains(questionLower, "boni wählen")
	if !wantWisdom && !wantDarkvision && !wantSpeed && !wantFlexibleBonuses {
		return "", false
	}

	lines := strings.Split(raceReference, "\n")
	darkvision := make([]string, 0, 8)
	speed := make([]string, 0, 8)
	flexible := make([]string, 0, 8)

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		clean := strings.TrimSpace(strings.TrimPrefix(line, "- "))
		lower := strings.ToLower(clean)
		switch {
		case strings.Contains(lower, "dunkelsicht") || strings.Contains(lower, "darkvision"):
			darkvision = append(darkvision, line)
		case strings.Contains(lower, "bewegung"):
			speed = append(speed, line)
		case strings.Contains(lower, "nach wahl") || strings.Contains(lower, "frei wählen") || strings.Contains(lower, "freie wahl"):
			flexible = append(flexible, line)
		}
	}

	sections := make([]string, 0, 3)
	if wantWisdom {
		sections = append(sections, strings.Join([]string{
			"Rassen mit festem Weisheitsbonus:",
			"- Hügelzwerg: +1 Weisheit",
		}, "\n"))
	}
	if wantFlexibleBonuses {
		flexibleSection := []string{
			"Rassen mit frei waelbaren oder flexiblen Attributsboni:",
			"- Halbelf: +2 Charisma, +1 auf zwei weitere Attribute nach Wahl.",
		}
		if len(flexible) > 0 {
			flexibleSection = append(flexibleSection, flexible...)
		}
		sections = append(sections, strings.Join(flexibleSection, "\n"))
	}
	if wantDarkvision && len(darkvision) > 0 {
		sections = append(sections, "Rassen mit Dunkelsicht:\n"+strings.Join(darkvision, "\n"))
	}
	if wantSpeed && len(speed) > 0 {
		sections = append(sections, "Rassen mit Bewegungsrate:\n"+strings.Join(speed, "\n"))
	}

	if len(sections) == 0 {
		return "", false
	}

	return strings.Join(sections, "\n\n"), true
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func (h *Handler) verifyBuilderEvidence(ctx context.Context, question string, evidence []builderRetrievalChunk, answer string) (string, bool) {
	if len(evidence) == 0 || !builderQuestionLooksLikeListRequest(question) {
		return answer, false
	}

	evidencePayload, _ := json.Marshal(compactBuilderRetrievalEvidenceForPrompt(evidence))
	messages := []map[string]string{
		{
			"role":    "system",
			"content": "Du pruefst Builder-Antworten streng gegen PDF-Belege. Nutze nur die Belege, keine Aussenkenntnis. Wenn die Antwort durch die Belege gedeckt ist, gib supported=true und dieselbe oder eine kuerzere, saubere Antwort zurueck. Wenn die Antwort nicht gedeckt ist, gib supported=false und formuliere eine kurze, nur aus den Belegen abgeleitete Korrektur. Gib nur JSON zurueck mit den Feldern supported, reply und notes.",
		},
		{
			"role":    "user",
			"content": fmt.Sprintf("Frage:\n%s\n\nAktuelle Antwort:\n%s\n\nPDF-Belege:\n%s", strings.TrimSpace(question), strings.TrimSpace(answer), string(evidencePayload)),
		},
	}

	content, _, err := h.llmClient.chatCompletion(ctx, messages, true, 320)
	if err != nil {
		return answer, false
	}

	var verdict struct {
		Supported bool     `json:"supported"`
		Reply     string   `json:"reply"`
		Notes     []string `json:"notes"`
	}
	trimmed := strings.TrimSpace(content)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)
	if err := json.Unmarshal([]byte(trimmed), &verdict); err != nil {
		return answer, false
	}
	if strings.TrimSpace(verdict.Reply) == "" {
		return answer, verdict.Supported
	}
	return verdict.Reply, verdict.Supported
}

func (h *Handler) estimateBuilderPromptTokens(character *Character, llmSession *LLMSession, documents []Document, transcript []CharacterBuilderMessage) int {
	currentRuleset := fmt.Sprintf("%v %v", character.Metadata["ruleset_work"], character.Metadata["ruleset_version"])
	selectedDocs := make([]string, 0, len(documents))
	for _, document := range documents {
		if strings.HasSuffix(strings.TrimSpace(fmt.Sprintf("%v", document.Metadata["kind"])), "_guide") {
			continue
		}
		selectedDocs = append(selectedDocs, document.Name)
	}
	runtimeSummary := mergeBuilderRuntimeSummary(defaultMetadata(llmSession.WorkingSummary), *character, documents, transcript)
	guideSummary := strings.TrimSpace(fmt.Sprint(runtimeSummary["guide_summary"]))
	levelUpSummary := strings.TrimSpace(fmt.Sprint(runtimeSummary["level_up_summary"]))
	conversationSummary := strings.TrimSpace(fmt.Sprint(runtimeSummary["conversation_summary"]))
	sheetJSON, _ := json.Marshal(compactCharacterDraftForPrompt(*character))
	builderQuery := builderContextQueryForCharacter(*character, currentBuilderStage(*character, llmSession), latestBuilderUserMessage(transcript))
	relevantRulesContext := h.retrieveBuilderContext(context.Background(), documents, builderQuery, 3)
	builderContextJSON, _ := json.Marshal(buildCharacterBuilderPromptContext(character, llmSession, documents, transcript, relevantRulesContext, builderQuery, h.retrieveBuilderEvidence(context.Background(), documents, builderQuery, 6)))
	messages := []map[string]string{
		{
			"role":    "system",
			"content": mustFormatEmbeddedPrompt("prompts/character_builder_system_prompt.md", guideSummary, levelUpSummary),
		},
		{
			"role": "user",
			"content": fmt.Sprintf(
				"Regelwerk: %s\nAusgewaehlte Buecher: %s\nBuilder-Suchanfrage: %s\nBuilder-Kontext: %s\nAktueller Character-Draft: %s\nBuilder-Zusammenfassung: %s\nBisheriger Gespraechsverlauf komprimiert: %s\nDialogverlauf (letzte Turns):\n%s\nWichtige Antwortregel: Wenn der Builder-Kontext konkrete PDF-Treffer oder Listen enthaelt, antworte direkt daraus und nenne die passenden Optionen als Liste statt nachzufragen.",
				currentRuleset,
				strings.Join(selectedDocs, ", "),
				builderQuery,
				string(builderContextJSON),
				string(sheetJSON),
				mustJSON(runtimeSummary),
				firstNonEmpty(conversationSummary, "(noch keine aeltere Gespraechszusammenfassung)"),
				renderCompactBuilderTranscript(compactBuilderTranscript(transcript, 6)),
			),
		},
	}
	return estimatePromptTokens(messages)
}

func applyCharacterPatch(character *Character, patch CharacterBuilderPatch) {
	if patch.Name != nil && strings.TrimSpace(*patch.Name) != "" {
		character.Name = strings.TrimSpace(*patch.Name)
	}
	if patch.PlayerName != nil {
		character.PlayerName = strings.TrimSpace(*patch.PlayerName)
	}
	if patch.ClassAndLevel != nil {
		character.ClassAndLevel = strings.TrimSpace(*patch.ClassAndLevel)
	}
	if patch.Background != nil {
		character.Background = strings.TrimSpace(*patch.Background)
	}
	if patch.Race != nil {
		character.Race = strings.TrimSpace(*patch.Race)
	}
	if patch.Alignment != nil {
		character.Alignment = strings.TrimSpace(*patch.Alignment)
	}
	if patch.ArmorClass != nil {
		character.ArmorClass = patch.ArmorClass
	}
	if patch.Speed != nil {
		character.Speed = strings.TrimSpace(*patch.Speed)
	}
	if patch.HitPointMax != nil {
		character.HitPointMax = patch.HitPointMax
	}
	if patch.Proficiency != nil {
		character.Proficiency = strings.TrimSpace(*patch.Proficiency)
	}
	if len(patch.Abilities) > 0 {
		character.Abilities = patch.Abilities
	}
	if patch.Languages != nil {
		character.Languages = defaultStringSlice(patch.Languages)
	}
	if patch.Features != nil {
		character.Features = defaultStringSlice(patch.Features)
	}
	if len(patch.Metadata) > 0 {
		if character.Metadata == nil {
			character.Metadata = map[string]any{}
		}
		for key, value := range patch.Metadata {
			character.Metadata[key] = value
		}
	}
	reconcileBuilderDerivedCharacterFields(character)
}

func reconcileBuilderDerivedCharacterFields(character *Character) {
	if character == nil {
		return
	}
	if character.Metadata == nil {
		character.Metadata = map[string]any{}
	}
	metadata := defaultMetadata(character.Metadata)
	backgroundSkills := uniqueCanonicalSkills(stringListFromAny(metadata["background_skill_proficiencies"]))
	classSkills := uniqueCanonicalSkills(stringListFromAny(metadata["class_skill_proficiencies"]))
	legacySkills := uniqueCanonicalSkills(stringListFromAny(metadata["skill_proficiencies"]))
	if len(backgroundSkills) == 0 && len(classSkills) == 0 && len(legacySkills) > 0 {
		backgroundSkills = legacySkills
	}
	mergedSkills := uniqueCanonicalSkills(append(backgroundSkills, classSkills...))
	if len(mergedSkills) == 0 {
		mergedSkills = legacySkills
	}
	if len(mergedSkills) > 0 {
		character.Metadata["skill_proficiencies"] = mergedSkills
	}
	if classRule, ok := builderClassRuleForCharacter(*character); ok {
		if len(stringListFromAny(metadata["saving_throw_proficiencies"])) == 0 && len(classRule.SavingThrows) > 0 {
			character.Metadata["saving_throw_proficiencies"] = classRule.SavingThrows
		}
	}
	if bonus := deriveCharacterProficiencyBonus(*character); bonus > 0 {
		character.Proficiency = fmt.Sprintf("+%d", bonus)
	}
	if passive := derivedPassivePerception(*character); passive > 0 {
		character.Metadata["passive_perception"] = passive
	}
}

func deriveCharacterProficiencyBonus(character Character) int {
	level := deriveCharacterLevel(character.ClassAndLevel)
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

func deriveCharacterLevel(classAndLevel string) int {
	match := regexp.MustCompile(`(?i)level\s*(\d{1,2})`).FindStringSubmatch(classAndLevel)
	if len(match) >= 2 {
		if value, err := strconv.Atoi(match[1]); err == nil {
			return value
		}
	}
	match = regexp.MustCompile(`\b(\d{1,2})\b`).FindStringSubmatch(classAndLevel)
	if len(match) >= 2 {
		if value, err := strconv.Atoi(match[1]); err == nil {
			return value
		}
	}
	return 0
}

func derivedPassivePerception(character Character) int {
	wisdom := abilityModifierFromAny(character.Abilities["wisdom"])
	total := 10 + wisdom
	for _, skill := range metadataStringList(defaultMetadata(character.Metadata)["skill_proficiencies"]) {
		normalized := strings.ToLower(strings.TrimSpace(skill))
		if normalized == "wahrnehmung" || normalized == "perception" {
			total += deriveCharacterProficiencyBonus(character)
			break
		}
	}
	return total
}

func builderMessagesFromMetadata(metadata map[string]any) []CharacterBuilderMessage {
	raw := metadata["builder_messages"]
	items, ok := raw.([]any)
	if !ok {
		return []CharacterBuilderMessage{}
	}

	result := make([]CharacterBuilderMessage, 0, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role := strings.TrimSpace(fmt.Sprint(entry["role"]))
		content := strings.TrimSpace(fmt.Sprint(entry["content"]))
		if role == "" || content == "" {
			continue
		}
		createdAt := time.Now().UTC()
		if rawTime, ok := entry["created_at"].(string); ok {
			if parsed, err := time.Parse(time.RFC3339, rawTime); err == nil {
				createdAt = parsed
			}
		}
		result = append(result, CharacterBuilderMessage{
			Role:      role,
			Content:   content,
			CreatedAt: createdAt,
		})
	}
	return result
}

func builderMessagesToMetadata(messages []CharacterBuilderMessage) []map[string]any {
	result := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		result = append(result, map[string]any{
			"role":       message.Role,
			"content":    message.Content,
			"created_at": message.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	return result
}

func builderMessagesFromHistory(history []map[string]any) []CharacterBuilderMessage {
	result := make([]CharacterBuilderMessage, 0, len(history))
	for _, entry := range history {
		role := strings.TrimSpace(fmt.Sprint(entry["role"]))
		content := strings.TrimSpace(fmt.Sprint(entry["content"]))
		if role == "" || content == "" {
			continue
		}
		createdAt := time.Now().UTC()
		if rawTime, ok := entry["created_at"].(string); ok {
			if parsed, err := time.Parse(time.RFC3339, rawTime); err == nil {
				createdAt = parsed
			}
		}
		result = append(result, CharacterBuilderMessage{
			Role:      role,
			Content:   content,
			CreatedAt: createdAt,
		})
	}
	return result
}

func renderBuilderTranscript(messages []CharacterBuilderMessage) string {
	if len(messages) == 0 {
		return "(noch kein Verlauf)"
	}
	lines := make([]string, 0, len(messages))
	for _, message := range messages {
		lines = append(lines, fmt.Sprintf("%s: %s", strings.ToUpper(message.Role), strings.TrimSpace(message.Content)))
	}
	return strings.Join(lines, "\n")
}

func renderCompactBuilderTranscript(messages []map[string]any) string {
	if len(messages) == 0 {
		return "(noch kein Verlauf)"
	}
	lines := make([]string, 0, len(messages))
	for _, message := range messages {
		role := strings.ToUpper(strings.TrimSpace(fmt.Sprint(message["role"])))
		content := strings.TrimSpace(fmt.Sprint(message["content"]))
		if role == "" {
			role = "UNKNOWN"
		}
		lines = append(lines, fmt.Sprintf("%s: %s", role, content))
	}
	return strings.Join(lines, "\n")
}

func stringListFromAny(value any) []string {
	items, ok := value.([]any)
	if !ok {
		if typed, ok := value.([]string); ok {
			return typed
		}
		return []string{}
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		text := strings.TrimSpace(fmt.Sprint(item))
		if text != "" {
			result = append(result, text)
		}
	}
	return result
}

func isCharacterBuilderTimeout(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "timeout") ||
		strings.Contains(message, "deadline exceeded") ||
		strings.Contains(message, "client.timeout exceeded") ||
		strings.Contains(message, "context deadline exceeded") ||
		strings.Contains(message, "context canceled")
}

func (h *Handler) loadOrCreateCharacterBuilderSession(ctx context.Context, character *Character, messages []CharacterBuilderMessage) (LLMSession, error) {
	if character.Metadata == nil {
		character.Metadata = map[string]any{}
	}
	if sessionID := safeOptionalString(character.Metadata["llm_session_id"]); sessionID != "" {
		session, err := h.store.GetLLMSession(ctx, sessionID)
		if err == nil {
			return session, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return LLMSession{}, err
		}
	}

	session, err := h.store.GetLatestLLMSessionByScope(ctx, "character", character.ID, "character_builder_session")
	if err == nil {
		character.Metadata["llm_session_id"] = session.ID
		if _, updateErr := h.store.UpdateCharacter(ctx, *character); updateErr != nil {
			return LLMSession{}, updateErr
		}
		return session, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return LLMSession{}, err
	}

	now := time.Now().UTC()
	session, err = h.store.CreateLLMSession(ctx, LLMSession{
		SessionType:     "character_builder_session",
		ScopeType:       "character",
		ScopeID:         character.ID,
		RequestProfile:  "builder",
		RulesetWork:     safeOptionalString(character.Metadata["ruleset_work"]),
		RulesetVersion:  safeOptionalString(character.Metadata["ruleset_version"]),
		MessageHistory:  builderMessagesToMetadata(messages),
		WorkingSummary:  mergeBuilderSummary(nil, *character),
		StructuredState: mergeBuilderStructuredState(nil, *character),
		TokenBudget:     6000,
		LastActiveAt:    now,
	})
	if err != nil {
		return LLMSession{}, err
	}
	character.Metadata["llm_session_id"] = session.ID
	if _, err := h.store.UpdateCharacter(ctx, *character); err != nil {
		return LLMSession{}, err
	}
	return session, nil
}

func mergeBuilderSummary(existing map[string]any, character Character) map[string]any {
	summary := map[string]any{}
	for key, value := range defaultMetadata(existing) {
		summary[key] = value
	}
	if character.Metadata != nil {
		if stage := strings.TrimSpace(fmt.Sprint(character.Metadata["builder_stage"])); stage != "" {
			summary["builder_stage"] = stage
		}
		if status := strings.TrimSpace(fmt.Sprint(character.Metadata["builder_status"])); status != "" {
			summary["builder_status"] = status
		}
	}
	if name := strings.TrimSpace(character.Name); name != "" {
		summary["name"] = name
	}
	return summary
}

func mergeBuilderRuntimeSummary(existing map[string]any, character Character, documents []Document, messages []CharacterBuilderMessage) map[string]any {
	summary := mergeBuilderSummary(existing, character)
	if summary["guide_summary"] == nil || strings.TrimSpace(fmt.Sprint(summary["guide_summary"])) == "" {
		guideContent := findGuideContent(documents, "character_builder_guide")
		if guideContent == "" {
			guideContent = loadEmbeddedBuilderGuide(
				strings.TrimSpace(fmt.Sprintf("%v", character.Metadata["ruleset_work"])),
				strings.TrimSpace(fmt.Sprintf("%v", character.Metadata["ruleset_version"])),
				"character_builder",
			)
		}
		summary["guide_summary"] = summarizeBuilderGuide(guideContent)
	}
	if summary["level_up_summary"] == nil || strings.TrimSpace(fmt.Sprint(summary["level_up_summary"])) == "" {
		levelUpContent := findGuideContent(documents, "level_up_guide")
		if levelUpContent == "" {
			levelUpContent = loadEmbeddedBuilderGuide(
				strings.TrimSpace(fmt.Sprintf("%v", character.Metadata["ruleset_work"])),
				strings.TrimSpace(fmt.Sprintf("%v", character.Metadata["ruleset_version"])),
				"level_up",
			)
		}
		summary["level_up_summary"] = summarizeBuilderGuide(levelUpContent)
	}
	summary["conversation_summary"] = summarizeBuilderConversation(messages, 4)
	return summary
}

func mergeBuilderStructuredState(existing map[string]any, character Character) map[string]any {
	state := map[string]any{}
	for key, value := range defaultMetadata(existing) {
		state[key] = value
	}
	state["character_id"] = character.ID
	state["name"] = character.Name
	state["class_and_level"] = character.ClassAndLevel
	state["race"] = character.Race
	state["background"] = character.Background
	state["alignment"] = character.Alignment
	state["abilities"] = defaultIntMap(character.Abilities)
	if character.Metadata != nil {
		state["builder_stage"] = character.Metadata["builder_stage"]
		state["builder_status"] = character.Metadata["builder_status"]
		state["background_skill_proficiencies"] = character.Metadata["background_skill_proficiencies"]
		state["class_skill_proficiencies"] = character.Metadata["class_skill_proficiencies"]
		state["skill_proficiencies"] = character.Metadata["skill_proficiencies"]
	}
	return state
}

func summarizeBuilderGuide(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return "(kein Builder-Leitfaden vorhanden)"
	}
	lines := strings.Split(content, "\n")
	steps := make([]string, 0, 18)
	guardrails := make([]string, 0, 6)
	constraints := make([]string, 0, 6)
	currentID := ""
	currentTitle := ""
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		switch {
		case strings.HasPrefix(line, "- id:"):
			if currentID != "" || currentTitle != "" {
				steps = append(steps, firstNonEmpty(currentTitle, currentID))
			}
			currentID = strings.TrimSpace(strings.TrimPrefix(line, "- id:"))
			currentTitle = ""
		case strings.HasPrefix(line, "title:"):
			currentTitle = strings.TrimSpace(strings.TrimPrefix(line, "title:"))
		case strings.Contains(line, ": true"):
			constraints = append(constraints, strings.TrimSuffix(line, ": true"))
		case strings.HasPrefix(line, "rule:"):
			guardrails = append(guardrails, strings.TrimSpace(strings.TrimPrefix(line, "rule:")))
		}
	}
	if currentID != "" || currentTitle != "" {
		steps = append(steps, firstNonEmpty(currentTitle, currentID))
	}
	if len(steps) > 12 {
		steps = steps[:12]
	}
	if len(guardrails) > 4 {
		guardrails = guardrails[:4]
	}
	if len(constraints) > 4 {
		constraints = constraints[:4]
	}
	parts := []string{}
	if len(steps) > 0 {
		parts = append(parts, "Ablauf: "+strings.Join(steps, " -> "))
	}
	if len(constraints) > 0 {
		parts = append(parts, "Leitplanken: "+strings.Join(constraints, ", "))
	}
	if len(guardrails) > 0 {
		parts = append(parts, "Wichtige Regeln: "+strings.Join(guardrails, " | "))
	}
	if len(parts) == 0 {
		return content
	}
	return strings.Join(parts, "\n")
}

func summarizeBuilderConversation(messages []CharacterBuilderMessage, keepRecent int) string {
	if keepRecent < 0 {
		keepRecent = 0
	}
	if len(messages) <= keepRecent {
		return ""
	}
	older := messages[:len(messages)-keepRecent]
	lines := make([]string, 0, len(older))
	for _, message := range older {
		role := "Nutzer"
		if strings.EqualFold(strings.TrimSpace(message.Role), "assistant") {
			role = "Builder"
		}
		text := strings.Join(strings.Fields(strings.TrimSpace(message.Content)), " ")
		if text == "" {
			continue
		}
		if len(text) > 140 {
			text = strings.TrimSpace(text[:140]) + " ..."
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", role, text))
	}
	if len(lines) > 10 {
		lines = lines[len(lines)-10:]
	}
	return strings.Join(lines, "\n")
}

func embeddedGuidesForRuleset(work string, version string) []Document {
	all := embeddedBuilderGuideDocuments()
	filtered := make([]Document, 0, len(all))
	for _, item := range all {
		if strings.EqualFold(strings.TrimSpace(fmt.Sprintf("%v", item.Metadata["ruleset_work"])), strings.TrimSpace(work)) &&
			strings.EqualFold(strings.TrimSpace(fmt.Sprintf("%v", item.Metadata["ruleset_version"])), strings.TrimSpace(version)) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func embeddedRaceReferenceGuide(work string, version string) Document {
	content := loadEmbeddedBuilderGuide(work, version, "race_reference")
	if strings.TrimSpace(content) == "" {
		return Document{}
	}
	now := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)
	metadata := map[string]any{
		"ruleset_work":          "5E",
		"ruleset_version":       "2014",
		"ruleset_keys":          []string{"5E:2014"},
		"kind":                  "race_reference",
		"source_type":           "embedded_race_reference",
		"embedded_guide_id":     "embedded-race-reference-dnd-5e",
		"system_document":       true,
		"guide_content":         content,
		"embedded_content":      content,
		"embedded_content_type": "text/markdown; charset=utf-8",
	}
	return Document{
		ID:             "embedded-race-reference-dnd-5e",
		Type:           "rules",
		Name:           "race_reference",
		SourceFilePath: nil,
		Metadata:       metadata,
		ChunkCount:     len(chunkDocumentText(content, 1200)),
		CreatedAt:      now,
	}
}

func dedupeDocumentsByID(documents []Document) []Document {
	seen := make(map[string]struct{}, len(documents))
	result := make([]Document, 0, len(documents))
	for _, document := range documents {
		id := strings.TrimSpace(document.ID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, document)
	}
	return result
}

func findGuideContent(documents []Document, kind string) string {
	for _, document := range documents {
		if !strings.EqualFold(strings.TrimSpace(fmt.Sprintf("%v", document.Metadata["kind"])), kind) {
			continue
		}
		if content := strings.TrimSpace(fmt.Sprintf("%v", document.Metadata["guide_content"])); content != "" {
			return content
		}
	}
	return ""
}

func embeddedBuilderGuidePath(work string, version string, kind string) string {
	normalizedWork := strings.ToLower(strings.TrimSpace(work))
	normalizedVersion := strings.ToLower(strings.TrimSpace(version))

	switch {
	case normalizedWork == "5e" && normalizedVersion == "2014" && kind == "character_builder":
		return "builder_guides/dnd-5e.character-builder.yaml"
	case normalizedWork == "5e" && normalizedVersion == "2014" && kind == "level_up":
		return "builder_guides/dnd-5e.level-up.yaml"
	case normalizedWork == "5e" && normalizedVersion == "2014" && kind == "short_rules":
		return "embedded_rules/dnd-5e.short_rules.md"
	case normalizedWork == "5e" && normalizedVersion == "2014" && kind == "race_reference":
		return "embedded_rules/dnd-5e.race_reference.md"
	default:
		return ""
	}
}

func loadEmbeddedBuilderGuide(work string, version string, kind string) string {
	path := embeddedBuilderGuidePath(work, version, kind)
	if path == "" {
		return ""
	}
	content, err := embeddedBuilderGuides.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(content))
}

func embeddedBuilderGuideDocuments() []Document {
	now := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)
	items := []struct {
		id      string
		name    string
		work    string
		version string
		kind    string
		path    string
	}{
		{
			id:      "embedded-guide-dnd-5e-character-builder",
			name:    "5E-compatible Character Builder Guide (SRD 5.1 Example)",
			work:    "5E",
			version: "2014",
			kind:    "character_builder_guide",
			path:    "builder_guides/dnd-5e.character-builder.yaml",
		},
		{
			id:      "embedded-guide-dnd-5e-level-up",
			name:    "5E-compatible Level-Up Guide (SRD 5.1 Example)",
			work:    "5E",
			version: "2014",
			kind:    "level_up_guide",
			path:    "builder_guides/dnd-5e.level-up.yaml",
		},
		{
			id:      "embedded-short-rules-dnd-5e",
			name:    "short_rules",
			work:    "5E",
			version: "2014",
			kind:    "short_rules_guide",
			path:    "embedded_rules/dnd-5e.short_rules.md",
		},
		{
			id:      "embedded-race-reference-dnd-5e",
			name:    "race_reference",
			work:    "5E",
			version: "2014",
			kind:    "race_reference",
			path:    "embedded_rules/dnd-5e.race_reference.md",
		},
	}

	documents := make([]Document, 0, len(items))
	for _, item := range items {
		content, err := embeddedBuilderGuides.ReadFile(item.path)
		if err != nil {
			continue
		}
		metadata := map[string]any{
			"ruleset_work":      item.work,
			"ruleset_version":   item.version,
			"ruleset_keys":      []string{fmt.Sprintf("%s:%s", item.work, item.version)},
			"kind":              item.kind,
			"source_type":       "embedded_example",
			"embedded_guide_id": item.id,
			"system_document":   true,
			"guide_content":     strings.TrimSpace(string(content)),
			"embedded_content":  strings.TrimSpace(string(content)),
			"embedded_content_type": func() string {
				if strings.HasSuffix(item.path, ".md") {
					return "text/markdown; charset=utf-8"
				}
				return "application/yaml; charset=utf-8"
			}(),
		}
		if item.kind == "short_rules_guide" {
			metadata["source_type"] = "embedded_short_rules"
		}
		if item.kind == "race_reference" {
			metadata["source_type"] = "embedded_race_reference"
		}
		chunkCount := 0
		if trimmed := strings.TrimSpace(string(content)); trimmed != "" {
			chunkCount = len(chunkDocumentText(trimmed, 1200))
		}
		documents = append(documents, Document{
			ID:             item.id,
			Type:           "rules",
			Name:           item.name,
			SourceFilePath: nil,
			Metadata:       metadata,
			ChunkCount:     chunkCount,
			CreatedAt:      now,
		})
	}
	return documents
}

func mergeEmbeddedGuideDocuments(items []Document, hidden map[string]struct{}) []Document {
	existing := make(map[string]struct{}, len(items))
	for _, item := range items {
		if embeddedID := fmt.Sprintf("%v", item.Metadata["embedded_guide_id"]); strings.TrimSpace(embeddedID) != "" {
			existing[embeddedID] = struct{}{}
		}
	}
	merged := append([]Document(nil), items...)
	for _, doc := range embeddedBuilderGuideDocuments() {
		if _, ok := hidden[doc.ID]; ok {
			continue
		}
		if _, ok := existing[doc.ID]; ok {
			continue
		}
		merged = append(merged, doc)
	}
	return merged
}

func shouldIncludeLevelUpGuide(transcript []CharacterBuilderMessage) bool {
	for i := len(transcript) - 1; i >= 0 && i >= len(transcript)-4; i-- {
		message := strings.ToLower(strings.TrimSpace(transcript[i].Content))
		if strings.Contains(message, "level up") ||
			strings.Contains(message, "levelup") ||
			strings.Contains(message, "aufstieg") ||
			strings.Contains(message, "stufe erhöhen") ||
			strings.Contains(message, "stufe erhoehen") ||
			strings.Contains(message, "nächste stufe") ||
			strings.Contains(message, "naechste stufe") {
			return true
		}
	}
	return false
}
