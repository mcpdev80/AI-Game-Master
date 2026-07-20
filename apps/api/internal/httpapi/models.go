package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

type Campaign struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type Adventure struct {
	ID          string         `json:"id"`
	CampaignID  *string        `json:"campaign_id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Language    string         `json:"language"`
	Metadata    map[string]any `json:"metadata"`
	CreatedAt   time.Time      `json:"created_at"`
}

type Session struct {
	ID                    string       `json:"id"`
	CampaignID            string       `json:"campaign_id"`
	Name                  string       `json:"name"`
	AdventureID           *string      `json:"adventure_id"`
	RulesetWork           string       `json:"ruleset_work"`
	RulesetVersion        string       `json:"ruleset_version"`
	TargetPlayerCount     int          `json:"target_player_count"`
	JoinToken             string       `json:"join_token"`
	Status                string       `json:"status"`
	CurrentScene          string       `json:"current_scene"`
	CurrentLocation       string       `json:"current_location"`
	Language              string       `json:"language"`
	DefaultVoiceProfileID *string      `json:"default_voice_profile_id"`
	State                 SessionState `json:"state"`
	CreatedAt             time.Time    `json:"created_at"`
	UpdatedAt             time.Time    `json:"updated_at"`
}

type SessionState struct {
	SceneSummary         string                `json:"scene_summary"`
	ActiveNPCs           []string              `json:"active_npcs"`
	OpenQuests           []string              `json:"open_quests"`
	LastDiceRoll         *DiceRollEvent        `json:"last_dice_roll"`
	LastConfirmedRoll    *DiceRollEvent        `json:"last_confirmed_roll,omitempty"`
	LastNarration        string                `json:"last_narration"`
	LastDMNotes          []string              `json:"last_dm_notes,omitempty"`
	ActiveMediaCue       string                `json:"active_media_cue"`
	VisualMode           string                `json:"visual_mode,omitempty"`
	VisualPayload        map[string]any        `json:"visual_payload,omitempty"`
	AudioMode            string                `json:"audio_mode,omitempty"`
	AudioPayload         map[string]any        `json:"audio_payload,omitempty"`
	VoiceMode            string                `json:"voice_mode,omitempty"`
	ActiveVoiceProfileID string                `json:"active_voice_profile_id,omitempty"`
	ActiveSpeakerRole    string                `json:"active_speaker_role,omitempty"`
	ActiveSpeakerName    string                `json:"active_speaker_name,omitempty"`
	TTSStatus            string                `json:"tts_status,omitempty"`
	AmbientCueID         string                `json:"ambient_cue_id,omitempty"`
	PlayLLMSessionID     string                `json:"play_llm_session_id,omitempty"`
	RulesLLMSessionID    string                `json:"rules_llm_session_id,omitempty"`
	SummarySessionID     string                `json:"summary_llm_session_id,omitempty"`
	SessionRecap         string                `json:"session_recap,omitempty"`
	SelectedRulebookIDs  []string              `json:"selected_rulebook_ids,omitempty"`
	PromptConfig         SessionPromptConfig   `json:"prompt_config,omitempty"`
	GroupInventory       SessionGroupInventory `json:"group_inventory,omitempty"`
}

type SessionPromptConfig struct {
	GMStyle           string `json:"gm_style,omitempty"`
	IntroStyle        string `json:"intro_style,omitempty"`
	AdventureFocus    string `json:"adventure_focus,omitempty"`
	RulesStrictness   string `json:"rules_strictness,omitempty"`
	PlayerAgencyStyle string `json:"player_agency_style,omitempty"`
	PromptOverride    string `json:"prompt_override,omitempty"`
}

type SessionGroupInventory struct {
	Gold  int      `json:"gold"`
	Items []string `json:"items"`
	Notes string   `json:"notes"`
}

type DiceRollEvent struct {
	Dice       []DiceResult `json:"dice"`
	Total      int          `json:"total,omitempty"`
	Summary    string       `json:"summary,omitempty"`
	Confidence float64      `json:"confidence"`
	Timestamp  time.Time    `json:"timestamp"`
}

type DiceResult struct {
	Type  string `json:"type"`
	Value int    `json:"value"`
}

type DiceBox struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

type Document struct {
	ID             string         `json:"id"`
	AdventureID    *string        `json:"adventure_id"`
	Type           string         `json:"type"`
	Name           string         `json:"name"`
	SourceFilePath *string        `json:"source_file_path"`
	Metadata       map[string]any `json:"metadata"`
	ChunkCount     int            `json:"chunk_count"`
	CreatedAt      time.Time      `json:"created_at"`
}

type DocumentChunk struct {
	ID         string         `json:"id"`
	DocumentID string         `json:"document_id"`
	ChunkText  string         `json:"chunk_text"`
	ChunkIndex int            `json:"chunk_index"`
	Metadata   map[string]any `json:"metadata"`
	CreatedAt  time.Time      `json:"created_at"`
}

type Asset struct {
	ID           string         `json:"id"`
	AdventureID  *string        `json:"adventure_id"`
	DocumentID   *string        `json:"document_id"`
	Type         string         `json:"type"`
	SourceType   string         `json:"source_type"`
	Name         string         `json:"name"`
	FilePath     string         `json:"file_path"`
	MimeType     string         `json:"mime_type"`
	EntityName   *string        `json:"entity_name"`
	LocationName *string        `json:"location_name"`
	Tags         []string       `json:"tags"`
	Metadata     map[string]any `json:"metadata"`
	CreatedAt    time.Time      `json:"created_at"`
}

type Character struct {
	ID            string         `json:"id"`
	CampaignID    *string        `json:"campaign_id"`
	DocumentID    *string        `json:"document_id"`
	Name          string         `json:"name"`
	PlayerName    string         `json:"player_name"`
	ClassAndLevel string         `json:"class_and_level"`
	Background    string         `json:"background"`
	Race          string         `json:"race"`
	Alignment     string         `json:"alignment"`
	ArmorClass    *int           `json:"armor_class"`
	Speed         string         `json:"speed"`
	HitPointMax   *int           `json:"hit_point_max"`
	Proficiency   string         `json:"proficiency_bonus"`
	Abilities     map[string]int `json:"abilities"`
	Languages     []string       `json:"languages"`
	Features      []string       `json:"features"`
	Metadata      map[string]any `json:"metadata"`
	CreatedAt     time.Time      `json:"created_at"`
}

type PlayerSlot struct {
	ID          string     `json:"id"`
	SessionID   string     `json:"session_id"`
	CharacterID *string    `json:"character_id"`
	DisplayName string     `json:"display_name"`
	Status      string     `json:"status"`
	JoinedAt    *time.Time `json:"joined_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

type PlayerAccessLink struct {
	ID           string     `json:"id"`
	PlayerSlotID string     `json:"player_slot_id"`
	Token        string     `json:"token"`
	RevokedAt    *time.Time `json:"revoked_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

type PlayerLinkSlot struct {
	PlayerSlot PlayerSlot        `json:"player_slot"`
	Link       *PlayerAccessLink `json:"link"`
	JoinURL    *string           `json:"join_url"`
}

type PlayerVisibleState struct {
	ID               string           `json:"id"`
	PlayerSlotID     string           `json:"player_slot_id"`
	VisibleCharacter map[string]any   `json:"visible_character"`
	VisibleHandouts  []map[string]any `json:"visible_handouts"`
	VisibleMedia     []map[string]any `json:"visible_media"`
	UpdatedAt        time.Time        `json:"updated_at"`
}

type CreatePlayerLinkRequest struct {
	CharacterID *string `json:"character_id"`
	DisplayName string  `json:"display_name" binding:"required,min=2,max=120"`
}

type UpdatePlayerVisibleStateRequest struct {
	HandoutDocumentIDs []string `json:"handout_document_ids"`
	MediaAssetIDs      []string `json:"media_asset_ids"`
}

type ResolveAbilityScoresRequest struct {
	Method     string  `json:"method" binding:"required,oneof=standard point_buy rolled"`
	Class      string  `json:"class"`
	RolledSets [][]int `json:"rolled_sets"`
	PointBuy   []int   `json:"point_buy"`
}

type DiceDetectionFrame struct {
	FrameID    string       `json:"frame_id"`
	Dice       []DiceResult `json:"dice"`
	Confidence float64      `json:"confidence"`
	Timestamp  time.Time    `json:"timestamp"`
}

type StabilizeDiceFramesRequest struct {
	Frames       []DiceDetectionFrame `json:"frames" binding:"required"`
	MinConsensus int                  `json:"min_consensus"`
}

type DetectDiceRequest struct {
	ImageDataURL string `json:"image_data_url" binding:"required"`
	Language     string `json:"language"`
}

type DetectDiceResponse struct {
	Dice       []DiceResult `json:"dice"`
	DiceCount  int          `json:"dice_count"`
	Boxes      []DiceBox    `json:"boxes"`
	Confidence float64      `json:"confidence"`
	Notes      string       `json:"notes"`
	RawModel   string       `json:"raw_model"`
}

type StabilizeDiceFramesResponse struct {
	Stable          bool                 `json:"stable"`
	RequiredMatches int                  `json:"required_matches"`
	MatchingFrames  int                  `json:"matching_frames"`
	StableDice      []DiceResult         `json:"stable_dice"`
	Confidence      float64              `json:"confidence"`
	Signature       string               `json:"signature"`
	RecentFrames    []DiceDetectionFrame `json:"recent_frames"`
}

type ValidateAbilityAssignmentRequest struct {
	Values     []int          `json:"values" binding:"required"`
	Assignment map[string]int `json:"assignment" binding:"required"`
	Class      string         `json:"class"`
}

type ResolveAbilityScoresResponse struct {
	Method            string         `json:"method"`
	Values            []int          `json:"values"`
	Assignment        map[string]int `json:"assignment"`
	RuleSummary       string         `json:"rule_summary"`
	RecommendedReason string         `json:"recommended_reason"`
	RolledBreakdown   []int          `json:"rolled_breakdown,omitempty"`
	NeedsConfirmation bool           `json:"needs_confirmation"`
}

type ValidateAbilityAssignmentResponse struct {
	Valid             bool           `json:"valid"`
	Values            []int          `json:"values"`
	Assignment        map[string]int `json:"assignment"`
	MissingAbilities  []string       `json:"missing_abilities"`
	UnexpectedKeys    []string       `json:"unexpected_keys"`
	DuplicateValues   []int          `json:"duplicate_values"`
	RecommendedReason string         `json:"recommended_reason"`
}

type PlayerPortalSession struct {
	Token               string             `json:"token"`
	Session             Session            `json:"session"`
	PlayerSlot          PlayerSlot         `json:"player_slot"`
	Character           *Character         `json:"character"`
	VisibleState        PlayerVisibleState `json:"visible_state"`
	AvailableCharacters []Character        `json:"available_characters"`
}

type CreateCampaignRequest struct {
	Name        string `json:"name" binding:"required,min=2,max=120"`
	Description string `json:"description"`
}

type CreateAdventureRequest struct {
	CampaignID  *string        `json:"campaign_id"`
	Name        string         `json:"name" binding:"required,min=2,max=160"`
	Description string         `json:"description"`
	Language    string         `json:"language"`
	Metadata    map[string]any `json:"metadata"`
}

type CreateFungalCavernsDemoRequest struct {
	Language string `json:"language"`
}

type FungalCavernsDemoResponse struct {
	Campaign        Campaign  `json:"campaign"`
	Adventure       Adventure `json:"adventure"`
	Session         Session   `json:"session"`
	MapAsset        Asset     `json:"map_asset"`
	GMURL           string    `json:"gm_url"`
	PlayerScreenURL string    `json:"player_screen_url"`
	Reused          bool      `json:"reused"`
}

type ResetFungalCavernsDemoResponse struct {
	Deleted          bool   `json:"deleted"`
	CampaignID       string `json:"campaign_id,omitempty"`
	AdventureID      string `json:"adventure_id,omitempty"`
	SessionCount     int    `json:"session_count"`
	CharacterCount   int    `json:"character_count"`
	DocumentCount    int    `json:"document_count"`
	AssetCount       int    `json:"asset_count"`
	ArchivedLLMCount int    `json:"archived_llm_count"`
}

type CreateCharacterRequest struct {
	CampaignID    *string        `json:"campaign_id"`
	Name          string         `json:"name" binding:"required,min=2,max=160"`
	PlayerName    string         `json:"player_name"`
	ClassAndLevel string         `json:"class_and_level"`
	Background    string         `json:"background"`
	Race          string         `json:"race"`
	Alignment     string         `json:"alignment"`
	ArmorClass    *int           `json:"armor_class"`
	Speed         string         `json:"speed"`
	HitPointMax   *int           `json:"hit_point_max"`
	Proficiency   string         `json:"proficiency_bonus"`
	Abilities     map[string]int `json:"abilities"`
	Languages     []string       `json:"languages"`
	Features      []string       `json:"features"`
	Metadata      map[string]any `json:"metadata"`
}

type CharacterBuilderMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type CharacterBuilderPatch struct {
	Name          *string        `json:"name,omitempty"`
	PlayerName    *string        `json:"player_name,omitempty"`
	ClassAndLevel *string        `json:"class_and_level,omitempty"`
	Background    *string        `json:"background,omitempty"`
	Race          *string        `json:"race,omitempty"`
	Alignment     *string        `json:"alignment,omitempty"`
	ArmorClass    *int           `json:"armor_class,omitempty"`
	Speed         *string        `json:"speed,omitempty"`
	HitPointMax   *int           `json:"hit_point_max,omitempty"`
	Proficiency   *string        `json:"proficiency_bonus,omitempty"`
	Abilities     map[string]int `json:"abilities,omitempty"`
	Languages     []string       `json:"languages,omitempty"`
	Features      []string       `json:"features,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

func (p *CharacterBuilderPatch) UnmarshalJSON(data []byte) error {
	type rawPatch struct {
		Name          *string         `json:"name,omitempty"`
		PlayerName    *string         `json:"player_name,omitempty"`
		ClassAndLevel *string         `json:"class_and_level,omitempty"`
		Background    *string         `json:"background,omitempty"`
		Race          *string         `json:"race,omitempty"`
		Alignment     *string         `json:"alignment,omitempty"`
		ArmorClass    *int            `json:"armor_class,omitempty"`
		Speed         *string         `json:"speed,omitempty"`
		HitPointMax   *int            `json:"hit_point_max,omitempty"`
		Proficiency   json.RawMessage `json:"proficiency_bonus,omitempty"`
		Abilities     map[string]int  `json:"abilities,omitempty"`
		Languages     []string        `json:"languages,omitempty"`
		Features      []string        `json:"features,omitempty"`
		Metadata      map[string]any  `json:"metadata,omitempty"`
	}

	var raw rawPatch
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	*p = CharacterBuilderPatch{
		Name:          raw.Name,
		PlayerName:    raw.PlayerName,
		ClassAndLevel: raw.ClassAndLevel,
		Background:    raw.Background,
		Race:          raw.Race,
		Alignment:     raw.Alignment,
		ArmorClass:    raw.ArmorClass,
		Speed:         raw.Speed,
		HitPointMax:   raw.HitPointMax,
		Abilities:     raw.Abilities,
		Languages:     raw.Languages,
		Features:      raw.Features,
		Metadata:      raw.Metadata,
	}

	proficiencyRaw := bytes.TrimSpace(raw.Proficiency)
	if len(proficiencyRaw) == 0 || bytes.Equal(proficiencyRaw, []byte("null")) {
		return nil
	}

	var proficiencyString string
	if err := json.Unmarshal(proficiencyRaw, &proficiencyString); err == nil {
		p.Proficiency = &proficiencyString
		return nil
	}

	var proficiencyNumber int
	if err := json.Unmarshal(proficiencyRaw, &proficiencyNumber); err == nil {
		proficiencyString = fmt.Sprintf("%d", proficiencyNumber)
		p.Proficiency = &proficiencyString
		return nil
	}

	return fmt.Errorf("invalid proficiency_bonus value")
}

type StartCharacterBuilderRequest struct {
	CampaignID          *string  `json:"campaign_id"`
	RulesetWork         string   `json:"ruleset_work" binding:"required,min=1,max=120"`
	RulesetVersion      string   `json:"ruleset_version" binding:"required,min=1,max=120"`
	SelectedDocumentIDs []string `json:"selected_document_ids" binding:"required,min=1"`
	Name                string   `json:"name"`
	PlayerName          string   `json:"player_name"`
	Language            string   `json:"language" binding:"omitempty,oneof=en de"`
}

type StartCharacterBuilderResponse struct {
	Character Character                 `json:"character"`
	Messages  []CharacterBuilderMessage `json:"messages"`
}

type CharacterBuilderMessageRequest struct {
	Message  string `json:"message" binding:"required,min=1,max=4000"`
	Language string `json:"language" binding:"omitempty,oneof=en de"`
}

type CharacterBuilderMessageResponse struct {
	Character    Character                 `json:"character"`
	Messages     []CharacterBuilderMessage `json:"messages"`
	Reply        string                    `json:"reply"`
	AppliedPatch CharacterBuilderPatch     `json:"applied_patch"`
	UIAction     string                    `json:"ui_action,omitempty"`
	UIPayload    map[string]any            `json:"ui_payload,omitempty"`
}

type CharacterBuilderApplyRequest struct {
	Patch CharacterBuilderPatch `json:"patch" binding:"required"`
}

type LLMSession struct {
	ID                     string           `json:"id"`
	SessionType            string           `json:"session_type"`
	ScopeType              string           `json:"scope_type"`
	ScopeID                string           `json:"scope_id"`
	RequestProfile         string           `json:"request_profile"`
	RulesetWork            string           `json:"ruleset_work"`
	RulesetVersion         string           `json:"ruleset_version"`
	Status                 string           `json:"status"`
	MessageHistory         []map[string]any `json:"message_history"`
	WorkingSummary         map[string]any   `json:"working_summary"`
	Facts                  []string         `json:"facts"`
	OpenThreads            []string         `json:"open_threads"`
	StructuredState        map[string]any   `json:"structured_state"`
	TokenBudget            int              `json:"token_budget"`
	LiveTurnWindow         int              `json:"live_turn_window"`
	SummaryVersion         int              `json:"summary_version"`
	EstimatedPromptTokens  int              `json:"estimated_prompt_tokens"`
	EstimatedSummaryTokens int              `json:"estimated_summary_tokens"`
	LastSummarizedAt       *time.Time       `json:"last_summarized_at,omitempty"`
	ArchivedAt             *time.Time       `json:"archived_at,omitempty"`
	LastActiveAt           time.Time        `json:"last_active_at"`
	CreatedAt              time.Time        `json:"created_at"`
}

type LLMGatewayStatus struct {
	Status                  string                    `json:"status"`
	InFlight                int                       `json:"in_flight"`
	MaxConcurrentRequests   int                       `json:"max_concurrent_requests"`
	QueueLength             int                       `json:"queue_length"`
	CircuitBreakerOpen      bool                      `json:"circuit_breaker_open"`
	CircuitBreakerUntil     *time.Time                `json:"circuit_breaker_until,omitempty"`
	ConsecutiveFailures     int                       `json:"consecutive_failures"`
	LastError               string                    `json:"last_error,omitempty"`
	RejectedRequests        int64                     `json:"rejected_requests"`
	TimeoutCount            int64                     `json:"timeout_count"`
	ActiveGatewaySessions   int                       `json:"active_gateway_sessions"`
	ArchivedGatewaySessions int                       `json:"archived_gateway_sessions"`
	Profiles                []LLMGatewayProfileStatus `json:"profiles"`
}

type LLMGatewayProfileStatus struct {
	Name            string `json:"name"`
	MaxInputTokens  int    `json:"max_input_tokens"`
	MaxOutputTokens int    `json:"max_output_tokens"`
	TimeoutSeconds  int    `json:"timeout_seconds"`
	LiveTurnWindow  int    `json:"live_turn_window"`
}

type CreateSessionRequest struct {
	CampaignID            string  `json:"campaign_id"`
	Name                  string  `json:"name" binding:"required,min=2,max=160"`
	AdventureID           *string `json:"adventure_id"`
	RulesetWork           string  `json:"ruleset_work" binding:"required,min=1,max=120"`
	RulesetVersion        string  `json:"ruleset_version" binding:"required,min=1,max=120"`
	TargetPlayerCount     int     `json:"target_player_count" binding:"required,min=1,max=12"`
	Status                string  `json:"status"`
	CurrentScene          string  `json:"current_scene"`
	CurrentLocation       string  `json:"current_location"`
	Language              string  `json:"language"`
	DefaultVoiceProfileID *string `json:"default_voice_profile_id"`
}

type UpdateSessionRequest struct {
	CampaignID            string                `json:"campaign_id"`
	Name                  string                `json:"name" binding:"required,min=2,max=160"`
	AdventureID           *string               `json:"adventure_id"`
	RulesetWork           string                `json:"ruleset_work" binding:"required,min=1,max=120"`
	RulesetVersion        string                `json:"ruleset_version" binding:"required,min=1,max=120"`
	TargetPlayerCount     int                   `json:"target_player_count" binding:"required,min=1,max=12"`
	CurrentScene          string                `json:"current_scene"`
	CurrentLocation       string                `json:"current_location"`
	Language              string                `json:"language"`
	DefaultVoiceProfileID *string               `json:"default_voice_profile_id"`
	SelectedRulebookIDs   []string              `json:"selected_rulebook_ids"`
	PromptConfig          SessionPromptConfig   `json:"prompt_config"`
	GroupInventory        SessionGroupInventory `json:"group_inventory"`
}

type JoinSessionRequest struct {
	DisplayName  string  `json:"display_name"`
	PlayerSlotID *string `json:"player_slot_id,omitempty"`
}

type JoinSessionResponse struct {
	SessionToken string              `json:"session_token"`
	PortalToken  string              `json:"portal_token"`
	JoinURL      string              `json:"join_url"`
	Portal       PlayerPortalSession `json:"portal"`
}

type SessionJoinCandidate struct {
	PlayerSlot PlayerSlot `json:"player_slot"`
	Character  *Character `json:"character"`
}

type SessionJoinPreview struct {
	SessionID       string                 `json:"session_id"`
	SessionName     string                 `json:"session_name"`
	SessionStatus   string                 `json:"session_status"`
	HasProgress     bool                   `json:"has_progress"`
	ExistingPlayers []SessionJoinCandidate `json:"existing_players"`
}

type SystemConfig struct {
	LLMProvider string `json:"llm_provider"`
	LLMBaseURL  string `json:"llm_base_url"`
	LLMModel    string `json:"llm_model"`
}

type UpdateSystemConfigRequest struct {
	LLMProvider string `json:"llm_provider"`
	LLMBaseURL  string `json:"llm_base_url"`
	LLMModel    string `json:"llm_model"`
}

type UpdatePlayerSlotCharacterRequest struct {
	CharacterID string `json:"character_id" binding:"required,uuid"`
}

type UpdatePlayerSlotStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=invited joined ready locked"`
}

type UpdateSessionRuntimeStateRequest struct {
	VisualMode           string         `json:"visual_mode"`
	VisualPayload        map[string]any `json:"visual_payload"`
	AudioMode            string         `json:"audio_mode"`
	AudioPayload         map[string]any `json:"audio_payload"`
	VoiceMode            string         `json:"voice_mode"`
	ActiveVoiceProfileID string         `json:"active_voice_profile_id"`
	ActiveSpeakerRole    string         `json:"active_speaker_role"`
	ActiveSpeakerName    string         `json:"active_speaker_name"`
	TTSStatus            string         `json:"tts_status"`
	AmbientCueID         string         `json:"ambient_cue_id"`
	SessionRecap         string         `json:"session_recap"`
}

type GMRespondRequest struct {
	SessionID   string         `json:"session_id" binding:"required,uuid"`
	PlayerInput string         `json:"player_input" binding:"required,min=2"`
	Language    string         `json:"language"`
	DiceRoll    *DiceRollEvent `json:"dice_roll"`
}

type GMResponse struct {
	SessionID     string           `json:"session_id"`
	Narration     string           `json:"narration"`
	Language      string           `json:"language"`
	RulesUsed     []string         `json:"rules_used"`
	RollRequest   *RollRequest     `json:"roll_request,omitempty"`
	StateUpdates  []StateUpdate    `json:"state_updates"`
	SceneEvents   []SceneEvent     `json:"scene_events"`
	DMNotes       []string         `json:"dm_notes"`
	ContextChunks []GMContextChunk `json:"context_chunks"`
	PromptSource  string           `json:"prompt_source"`
	RawModel      string           `json:"raw_model"`
	CreatedAt     time.Time        `json:"created_at"`
}

type GMContextChunk struct {
	DocumentID   string `json:"document_id"`
	DocumentName string `json:"document_name"`
	ChunkText    string `json:"chunk_text"`
}

type MonsterReference struct {
	ID           string    `json:"id"`
	DocumentID   string    `json:"document_id"`
	DocumentName string    `json:"document_name"`
	Name         string    `json:"name"`
	NameSlug     string    `json:"name_slug"`
	ChunkIndex   int       `json:"chunk_index"`
	CreatedAt    time.Time `json:"created_at"`
}

type RuleIndexEntry struct {
	ID         string    `json:"id"`
	DocumentID string    `json:"document_id"`
	ChunkIndex int       `json:"chunk_index"`
	Category   string    `json:"category"`
	Term       string    `json:"term"`
	TermSlug   string    `json:"term_slug"`
	CreatedAt  time.Time `json:"created_at"`
}

type StateUpdate struct {
	EntityID string `json:"entity_id"`
	Field    string `json:"field"`
	Delta    int    `json:"delta,omitempty"`
	Value    string `json:"value,omitempty"`
}

type RollRequest struct {
	Type              string       `json:"type"`
	Label             string       `json:"label"`
	Dice              []string     `json:"dice"`
	Ability           string       `json:"ability,omitempty"`
	Skill             string       `json:"skill,omitempty"`
	DC                *int         `json:"dc,omitempty"`
	HideDC            bool         `json:"hide_dc,omitempty"`
	Reason            string       `json:"reason,omitempty"`
	Instructions      string       `json:"instructions,omitempty"`
	FollowUpOnSuccess *RollRequest `json:"follow_up_on_success,omitempty"`
}

type SceneEvent struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type CreateDocumentRequest struct {
	AdventureID    *string        `json:"adventure_id"`
	Type           string         `json:"type" binding:"required,oneof=rules adventure character_sheet asset"`
	Name           string         `json:"name" binding:"required,min=2,max=255"`
	SourceFilePath *string        `json:"source_file_path"`
	Metadata       map[string]any `json:"metadata"`
}

type SessionEvent struct {
	ID        string         `json:"id"`
	SessionID string         `json:"session_id"`
	Type      string         `json:"type"`
	Payload   map[string]any `json:"payload"`
	CreatedAt time.Time      `json:"created_at"`
}

type ZipImportReport struct {
	Adventure Adventure        `json:"adventure"`
	Documents []Document       `json:"documents"`
	Assets    []Asset          `json:"assets"`
	Summary   ZipImportSummary `json:"summary"`
}

type ZipImportSummary struct {
	ImportedDocuments  int `json:"imported_documents"`
	ImportedAssets     int `json:"imported_assets"`
	ImportedBattlemaps int `json:"imported_battlemaps"`
	ImportedPortraits  int `json:"imported_portraits"`
	ImportedTokens     int `json:"imported_tokens"`
	ImportedHandouts   int `json:"imported_handouts"`
}
