package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(ctx context.Context, databaseURL string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database config: %w", err)
	}

	cfg.MaxConns = 8
	cfg.MinConns = 1
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}

	store := &Store{pool: pool}
	if err := store.migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return store, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *Store) ListHiddenSystemDocumentIDs(ctx context.Context) (map[string]struct{}, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT document_id
		FROM hidden_system_documents
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make(map[string]struct{})
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		items[id] = struct{}{}
	}
	return items, rows.Err()
}

func (s *Store) HideSystemDocument(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO hidden_system_documents (document_id)
		VALUES ($1)
		ON CONFLICT (document_id) DO NOTHING
	`, id)
	return err
}

func (s *Store) migrate(ctx context.Context) error {
	statements := []string{
		`CREATE EXTENSION IF NOT EXISTS pgcrypto`,
		`CREATE TABLE IF NOT EXISTS campaigns (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS adventures (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			campaign_id UUID NULL REFERENCES campaigns(id) ON DELETE SET NULL,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			language TEXT NOT NULL DEFAULT 'de',
			metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`ALTER TABLE adventures ADD COLUMN IF NOT EXISTS metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
			name TEXT NOT NULL DEFAULT 'New Session',
			adventure_id UUID NULL REFERENCES adventures(id) ON DELETE SET NULL,
			ruleset_work TEXT NOT NULL DEFAULT '',
			ruleset_version TEXT NOT NULL DEFAULT '',
			target_player_count INT NOT NULL DEFAULT 4,
			join_token TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'draft',
			current_scene TEXT NOT NULL DEFAULT '',
			current_location TEXT NOT NULL DEFAULT '',
			language TEXT NOT NULL DEFAULT 'de',
			default_voice_profile_id TEXT NULL,
			state_json JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`ALTER TABLE sessions ADD COLUMN IF NOT EXISTS name TEXT NOT NULL DEFAULT 'New Session'`,
		`ALTER TABLE sessions ADD COLUMN IF NOT EXISTS adventure_id UUID NULL REFERENCES adventures(id) ON DELETE SET NULL`,
		`ALTER TABLE sessions ADD COLUMN IF NOT EXISTS ruleset_work TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sessions ADD COLUMN IF NOT EXISTS ruleset_version TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sessions ADD COLUMN IF NOT EXISTS target_player_count INT NOT NULL DEFAULT 4`,
		`ALTER TABLE sessions ADD COLUMN IF NOT EXISTS join_token TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sessions ADD COLUMN IF NOT EXISTS state_json JSONB NOT NULL DEFAULT '{}'::jsonb`,
		`CREATE TABLE IF NOT EXISTS documents (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			adventure_id UUID NULL REFERENCES adventures(id) ON DELETE SET NULL,
			type TEXT NOT NULL,
			name TEXT NOT NULL,
			source_file_path TEXT NULL,
			metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`ALTER TABLE documents ADD COLUMN IF NOT EXISTS adventure_id UUID NULL REFERENCES adventures(id) ON DELETE SET NULL`,
		`CREATE TABLE IF NOT EXISTS document_chunks (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
			chunk_text TEXT NOT NULL,
			chunk_index INT NOT NULL,
			metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS monster_references (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			name_slug TEXT NOT NULL,
			chunk_index INT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS rule_index_entries (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
			chunk_index INT NOT NULL,
			category TEXT NOT NULL DEFAULT '',
			term TEXT NOT NULL,
			term_slug TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS assets (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			adventure_id UUID NULL REFERENCES adventures(id) ON DELETE SET NULL,
			document_id UUID NULL REFERENCES documents(id) ON DELETE SET NULL,
			type TEXT NOT NULL,
			source_type TEXT NOT NULL,
			name TEXT NOT NULL,
			file_path TEXT NOT NULL,
			mime_type TEXT NOT NULL DEFAULT 'application/octet-stream',
			entity_name TEXT NULL,
			location_name TEXT NULL,
			tags_json JSONB NOT NULL DEFAULT '[]'::jsonb,
			metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS characters (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			campaign_id UUID NULL REFERENCES campaigns(id) ON DELETE SET NULL,
			document_id UUID NULL REFERENCES documents(id) ON DELETE SET NULL,
			name TEXT NOT NULL,
			player_name TEXT NOT NULL DEFAULT '',
			class_and_level TEXT NOT NULL DEFAULT '',
			background TEXT NOT NULL DEFAULT '',
			race TEXT NOT NULL DEFAULT '',
			alignment TEXT NOT NULL DEFAULT '',
			armor_class INT NULL,
			speed TEXT NOT NULL DEFAULT '',
			hit_point_max INT NULL,
			proficiency_bonus TEXT NOT NULL DEFAULT '',
			abilities_json JSONB NOT NULL DEFAULT '{}'::jsonb,
			languages_json JSONB NOT NULL DEFAULT '[]'::jsonb,
			features_json JSONB NOT NULL DEFAULT '[]'::jsonb,
			metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS player_slots (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			session_id UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
			character_id UUID NULL REFERENCES characters(id) ON DELETE SET NULL,
			display_name TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'invited',
			joined_at TIMESTAMPTZ NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS player_access_links (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			player_slot_id UUID NOT NULL REFERENCES player_slots(id) ON DELETE CASCADE,
			token TEXT NOT NULL UNIQUE,
			revoked_at TIMESTAMPTZ NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS player_visible_states (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			player_slot_id UUID NOT NULL UNIQUE REFERENCES player_slots(id) ON DELETE CASCADE,
			visible_character_json JSONB NOT NULL DEFAULT '{}'::jsonb,
			visible_handouts_json JSONB NOT NULL DEFAULT '[]'::jsonb,
			visible_media_json JSONB NOT NULL DEFAULT '[]'::jsonb,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS session_events (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			session_id UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
			type TEXT NOT NULL,
			payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS hidden_system_documents (
			document_id TEXT PRIMARY KEY,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS llm_sessions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			session_type TEXT NOT NULL,
			scope_type TEXT NOT NULL,
			scope_id TEXT NOT NULL,
			request_profile TEXT NOT NULL DEFAULT '',
			ruleset_work TEXT NOT NULL DEFAULT '',
			ruleset_version TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'active',
			message_history_json JSONB NOT NULL DEFAULT '[]'::jsonb,
			working_summary_json JSONB NOT NULL DEFAULT '{}'::jsonb,
			facts_json JSONB NOT NULL DEFAULT '[]'::jsonb,
			open_threads_json JSONB NOT NULL DEFAULT '[]'::jsonb,
			structured_state_json JSONB NOT NULL DEFAULT '{}'::jsonb,
			token_budget INT NOT NULL DEFAULT 0,
			live_turn_window INT NOT NULL DEFAULT 8,
			summary_version INT NOT NULL DEFAULT 1,
			estimated_prompt_tokens INT NOT NULL DEFAULT 0,
			estimated_summary_tokens INT NOT NULL DEFAULT 0,
			last_summarized_at TIMESTAMPTZ NULL,
			archived_at TIMESTAMPTZ NULL,
			last_active_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS system_settings (
			id BOOLEAN PRIMARY KEY DEFAULT TRUE,
			config_json JSONB NOT NULL DEFAULT '{}'::jsonb,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CHECK (id = TRUE)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_campaign_id ON sessions(campaign_id)`,
		`CREATE INDEX IF NOT EXISTS idx_adventures_campaign_id ON adventures(campaign_id)`,
		`CREATE INDEX IF NOT EXISTS idx_documents_type ON documents(type)`,
		`CREATE INDEX IF NOT EXISTS idx_documents_adventure_id ON documents(adventure_id)`,
		`CREATE INDEX IF NOT EXISTS idx_document_chunks_document_id ON document_chunks(document_id)`,
		`CREATE INDEX IF NOT EXISTS idx_monster_references_document_id ON monster_references(document_id)`,
		`CREATE INDEX IF NOT EXISTS idx_monster_references_name_slug ON monster_references(name_slug)`,
		`CREATE INDEX IF NOT EXISTS idx_rule_index_entries_document_id ON rule_index_entries(document_id)`,
		`CREATE INDEX IF NOT EXISTS idx_rule_index_entries_term_slug ON rule_index_entries(term_slug)`,
		`CREATE INDEX IF NOT EXISTS idx_rule_index_entries_category ON rule_index_entries(category)`,
		`CREATE INDEX IF NOT EXISTS idx_assets_adventure_id ON assets(adventure_id)`,
		`CREATE INDEX IF NOT EXISTS idx_characters_campaign_id ON characters(campaign_id)`,
		`CREATE INDEX IF NOT EXISTS idx_characters_document_id ON characters(document_id)`,
		`CREATE INDEX IF NOT EXISTS idx_player_slots_session_id ON player_slots(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_player_access_links_slot_id ON player_access_links(player_slot_id)`,
		`CREATE INDEX IF NOT EXISTS idx_session_events_session_id ON session_events(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_llm_sessions_scope ON llm_sessions(scope_type, scope_id)`,
		`CREATE INDEX IF NOT EXISTS idx_llm_sessions_type ON llm_sessions(session_type)`,
		`ALTER TABLE llm_sessions ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active'`,
		`ALTER TABLE llm_sessions ADD COLUMN IF NOT EXISTS facts_json JSONB NOT NULL DEFAULT '[]'::jsonb`,
		`ALTER TABLE llm_sessions ADD COLUMN IF NOT EXISTS open_threads_json JSONB NOT NULL DEFAULT '[]'::jsonb`,
		`ALTER TABLE llm_sessions ADD COLUMN IF NOT EXISTS live_turn_window INT NOT NULL DEFAULT 8`,
		`ALTER TABLE llm_sessions ADD COLUMN IF NOT EXISTS summary_version INT NOT NULL DEFAULT 1`,
		`ALTER TABLE llm_sessions ADD COLUMN IF NOT EXISTS estimated_prompt_tokens INT NOT NULL DEFAULT 0`,
		`ALTER TABLE llm_sessions ADD COLUMN IF NOT EXISTS estimated_summary_tokens INT NOT NULL DEFAULT 0`,
		`ALTER TABLE llm_sessions ADD COLUMN IF NOT EXISTS last_summarized_at TIMESTAMPTZ NULL`,
		`ALTER TABLE llm_sessions ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ NULL`,
		`CREATE INDEX IF NOT EXISTS idx_llm_sessions_status ON llm_sessions(status)`,
	}

	for _, statement := range statements {
		if _, err := s.pool.Exec(ctx, statement); err != nil {
			return fmt.Errorf("apply migration %q: %w", statement, err)
		}
	}

	if err := s.backfillRuleIndex(ctx); err != nil {
		return fmt.Errorf("backfill rule index: %w", err)
	}

	return nil
}

func (s *Store) GetSystemConfig(ctx context.Context) (SystemConfig, error) {
	var raw []byte
	err := s.pool.QueryRow(ctx, `
		SELECT config_json
		FROM system_settings
		WHERE id = TRUE
	`).Scan(&raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SystemConfig{}, nil
		}
		return SystemConfig{}, err
	}

	var cfg SystemConfig
	if len(raw) == 0 {
		return cfg, nil
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return SystemConfig{}, err
	}
	return cfg, nil
}

func (s *Store) UpdateSystemConfig(ctx context.Context, cfg SystemConfig) (SystemConfig, error) {
	payload, err := json.Marshal(cfg)
	if err != nil {
		return SystemConfig{}, err
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO system_settings (id, config_json, updated_at)
		VALUES (TRUE, $1, NOW())
		ON CONFLICT (id) DO UPDATE
		SET config_json = EXCLUDED.config_json,
		    updated_at = NOW()
	`, payload)
	if err != nil {
		return SystemConfig{}, err
	}
	return cfg, nil
}

func (s *Store) ListCampaigns(ctx context.Context) ([]Campaign, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, name, description, created_at
		FROM campaigns
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Campaign, 0)
	for rows.Next() {
		var item Campaign
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

func (s *Store) CreateCampaign(ctx context.Context, req CreateCampaignRequest) (Campaign, error) {
	var item Campaign
	err := s.pool.QueryRow(ctx, `
		INSERT INTO campaigns (name, description)
		VALUES ($1, $2)
		RETURNING id::text, name, description, created_at
	`, req.Name, req.Description).Scan(&item.ID, &item.Name, &item.Description, &item.CreatedAt)
	return item, err
}

func (s *Store) DeleteCampaign(ctx context.Context, id string) error {
	commandTag, err := s.pool.Exec(ctx, `DELETE FROM campaigns WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) ListAdventures(ctx context.Context) ([]Adventure, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, campaign_id::text, name, description, language, metadata_json, created_at
		FROM adventures
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Adventure, 0)
	for rows.Next() {
		var item Adventure
		var rawMetadata []byte
		if err := rows.Scan(&item.ID, &item.CampaignID, &item.Name, &item.Description, &item.Language, &rawMetadata, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.Metadata = map[string]any{}
		_ = json.Unmarshal(rawMetadata, &item.Metadata)
		items = append(items, item)
	}

	return items, rows.Err()
}

func (s *Store) ListCharacters(ctx context.Context) ([]Character, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, campaign_id::text, document_id::text, name, player_name, class_and_level, background, race, alignment,
		       armor_class, speed, hit_point_max, proficiency_bonus, abilities_json, languages_json, features_json, metadata_json, created_at
		FROM characters
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanCharacters(rows)
}

func (s *Store) CreateCharacter(ctx context.Context, item Character) (Character, error) {
	abilities, err := json.Marshal(defaultIntMap(item.Abilities))
	if err != nil {
		return Character{}, err
	}
	languages, err := json.Marshal(defaultStringSlice(item.Languages))
	if err != nil {
		return Character{}, err
	}
	features, err := json.Marshal(defaultStringSlice(item.Features))
	if err != nil {
		return Character{}, err
	}
	metadata, err := json.Marshal(defaultMetadata(item.Metadata))
	if err != nil {
		return Character{}, err
	}

	var created Character
	var rawAbilities []byte
	var rawLanguages []byte
	var rawFeatures []byte
	var rawMetadata []byte
	err = s.pool.QueryRow(ctx, `
		INSERT INTO characters (
			campaign_id, document_id, name, player_name, class_and_level, background, race, alignment,
			armor_class, speed, hit_point_max, proficiency_bonus, abilities_json, languages_json, features_json, metadata_json
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13::jsonb,$14::jsonb,$15::jsonb,$16::jsonb)
		RETURNING id::text, campaign_id::text, document_id::text, name, player_name, class_and_level, background, race, alignment,
		          armor_class, speed, hit_point_max, proficiency_bonus, abilities_json, languages_json, features_json, metadata_json, created_at
	`,
		item.CampaignID, item.DocumentID, item.Name, item.PlayerName, item.ClassAndLevel, item.Background, item.Race, item.Alignment,
		item.ArmorClass, item.Speed, item.HitPointMax, item.Proficiency, string(abilities), string(languages), string(features), string(metadata),
	).Scan(
		&created.ID, &created.CampaignID, &created.DocumentID, &created.Name, &created.PlayerName, &created.ClassAndLevel, &created.Background,
		&created.Race, &created.Alignment, &created.ArmorClass, &created.Speed, &created.HitPointMax, &created.Proficiency,
		&rawAbilities, &rawLanguages, &rawFeatures, &rawMetadata, &created.CreatedAt,
	)
	if err != nil {
		return Character{}, err
	}
	created.Abilities = map[string]int{}
	created.Languages = []string{}
	created.Features = []string{}
	created.Metadata = map[string]any{}
	_ = json.Unmarshal(rawAbilities, &created.Abilities)
	_ = json.Unmarshal(rawLanguages, &created.Languages)
	_ = json.Unmarshal(rawFeatures, &created.Features)
	_ = json.Unmarshal(rawMetadata, &created.Metadata)
	return created, nil
}

func (s *Store) GetCharacter(ctx context.Context, id string) (Character, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, campaign_id::text, document_id::text, name, player_name, class_and_level, background, race, alignment,
		       armor_class, speed, hit_point_max, proficiency_bonus, abilities_json, languages_json, features_json, metadata_json, created_at
		FROM characters
		WHERE id = $1
	`, id)
	if err != nil {
		return Character{}, err
	}
	defer rows.Close()
	items, err := s.scanCharacters(rows)
	if err != nil {
		return Character{}, err
	}
	if len(items) == 0 {
		return Character{}, pgx.ErrNoRows
	}
	return items[0], nil
}

func (s *Store) UpdateCharacter(ctx context.Context, item Character) (Character, error) {
	abilities, err := json.Marshal(defaultIntMap(item.Abilities))
	if err != nil {
		return Character{}, err
	}
	languages, err := json.Marshal(defaultStringSlice(item.Languages))
	if err != nil {
		return Character{}, err
	}
	features, err := json.Marshal(defaultStringSlice(item.Features))
	if err != nil {
		return Character{}, err
	}
	metadata, err := json.Marshal(defaultMetadata(item.Metadata))
	if err != nil {
		return Character{}, err
	}

	var updated Character
	var rawAbilities []byte
	var rawLanguages []byte
	var rawFeatures []byte
	var rawMetadata []byte
	err = s.pool.QueryRow(ctx, `
		UPDATE characters
		SET campaign_id = $2,
		    name = $3,
		    player_name = $4,
		    class_and_level = $5,
		    background = $6,
		    race = $7,
		    alignment = $8,
		    armor_class = $9,
		    speed = $10,
		    hit_point_max = $11,
		    proficiency_bonus = $12,
		    abilities_json = $13::jsonb,
		    languages_json = $14::jsonb,
		    features_json = $15::jsonb,
		    metadata_json = $16::jsonb
		WHERE id = $1
		RETURNING id::text, campaign_id::text, document_id::text, name, player_name, class_and_level, background, race, alignment,
		          armor_class, speed, hit_point_max, proficiency_bonus, abilities_json, languages_json, features_json, metadata_json, created_at
	`,
		item.ID, item.CampaignID, item.Name, item.PlayerName, item.ClassAndLevel, item.Background, item.Race, item.Alignment,
		item.ArmorClass, item.Speed, item.HitPointMax, item.Proficiency, string(abilities), string(languages), string(features), string(metadata),
	).Scan(
		&updated.ID, &updated.CampaignID, &updated.DocumentID, &updated.Name, &updated.PlayerName, &updated.ClassAndLevel, &updated.Background,
		&updated.Race, &updated.Alignment, &updated.ArmorClass, &updated.Speed, &updated.HitPointMax, &updated.Proficiency,
		&rawAbilities, &rawLanguages, &rawFeatures, &rawMetadata, &updated.CreatedAt,
	)
	if err != nil {
		return Character{}, err
	}
	updated.Abilities = map[string]int{}
	updated.Languages = []string{}
	updated.Features = []string{}
	updated.Metadata = map[string]any{}
	_ = json.Unmarshal(rawAbilities, &updated.Abilities)
	_ = json.Unmarshal(rawLanguages, &updated.Languages)
	_ = json.Unmarshal(rawFeatures, &updated.Features)
	_ = json.Unmarshal(rawMetadata, &updated.Metadata)
	return updated, nil
}

func (s *Store) DeleteCharacter(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM characters WHERE id = $1`, id)
	return err
}

func (s *Store) CreateLLMSession(ctx context.Context, item LLMSession) (LLMSession, error) {
	messageHistory, err := json.Marshal(defaultAnySlice(item.MessageHistory))
	if err != nil {
		return LLMSession{}, err
	}
	workingSummary, err := json.Marshal(defaultMetadata(item.WorkingSummary))
	if err != nil {
		return LLMSession{}, err
	}
	facts, err := json.Marshal(defaultStringSlice(item.Facts))
	if err != nil {
		return LLMSession{}, err
	}
	openThreads, err := json.Marshal(defaultStringSlice(item.OpenThreads))
	if err != nil {
		return LLMSession{}, err
	}
	structuredState, err := json.Marshal(defaultMetadata(item.StructuredState))
	if err != nil {
		return LLMSession{}, err
	}

	var created LLMSession
	var rawHistory, rawSummary, rawFacts, rawThreads, rawState []byte
	err = s.pool.QueryRow(ctx, `
		INSERT INTO llm_sessions (
			session_type, scope_type, scope_id, request_profile, ruleset_work, ruleset_version, status,
			message_history_json, working_summary_json, facts_json, open_threads_json, structured_state_json,
			token_budget, live_turn_window, summary_version, estimated_prompt_tokens, estimated_summary_tokens,
			last_summarized_at, archived_at, last_active_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9::jsonb,$10::jsonb,$11::jsonb,$12::jsonb,$13,$14,$15,$16,$17,$18,$19,$20)
		RETURNING id::text, session_type, scope_type, scope_id, request_profile, ruleset_work, ruleset_version, status,
		          message_history_json, working_summary_json, facts_json, open_threads_json, structured_state_json,
		          token_budget, live_turn_window, summary_version, estimated_prompt_tokens, estimated_summary_tokens,
		          last_summarized_at, archived_at, last_active_at, created_at
	`,
		item.SessionType, item.ScopeType, item.ScopeID, item.RequestProfile, item.RulesetWork, item.RulesetVersion, firstNonEmpty(item.Status, "active"),
		string(messageHistory), string(workingSummary), string(facts), string(openThreads), string(structuredState),
		item.TokenBudget, max(item.LiveTurnWindow, 8), max(item.SummaryVersion, 1), item.EstimatedPromptTokens, item.EstimatedSummaryTokens,
		item.LastSummarizedAt, item.ArchivedAt, item.LastActiveAt,
	).Scan(
		&created.ID, &created.SessionType, &created.ScopeType, &created.ScopeID, &created.RequestProfile, &created.RulesetWork, &created.RulesetVersion, &created.Status,
		&rawHistory, &rawSummary, &rawFacts, &rawThreads, &rawState,
		&created.TokenBudget, &created.LiveTurnWindow, &created.SummaryVersion, &created.EstimatedPromptTokens, &created.EstimatedSummaryTokens,
		&created.LastSummarizedAt, &created.ArchivedAt, &created.LastActiveAt, &created.CreatedAt,
	)
	if err != nil {
		return LLMSession{}, err
	}
	created.MessageHistory = []map[string]any{}
	created.WorkingSummary = map[string]any{}
	created.Facts = []string{}
	created.OpenThreads = []string{}
	created.StructuredState = map[string]any{}
	_ = json.Unmarshal(rawHistory, &created.MessageHistory)
	_ = json.Unmarshal(rawSummary, &created.WorkingSummary)
	_ = json.Unmarshal(rawFacts, &created.Facts)
	_ = json.Unmarshal(rawThreads, &created.OpenThreads)
	_ = json.Unmarshal(rawState, &created.StructuredState)
	return created, nil
}

func (s *Store) UpdateLLMSession(ctx context.Context, item LLMSession) (LLMSession, error) {
	messageHistory, err := json.Marshal(defaultAnySlice(item.MessageHistory))
	if err != nil {
		return LLMSession{}, err
	}
	workingSummary, err := json.Marshal(defaultMetadata(item.WorkingSummary))
	if err != nil {
		return LLMSession{}, err
	}
	facts, err := json.Marshal(defaultStringSlice(item.Facts))
	if err != nil {
		return LLMSession{}, err
	}
	openThreads, err := json.Marshal(defaultStringSlice(item.OpenThreads))
	if err != nil {
		return LLMSession{}, err
	}
	structuredState, err := json.Marshal(defaultMetadata(item.StructuredState))
	if err != nil {
		return LLMSession{}, err
	}

	var updated LLMSession
	var rawHistory, rawSummary, rawFacts, rawThreads, rawState []byte
	err = s.pool.QueryRow(ctx, `
		UPDATE llm_sessions
		SET session_type = $2,
		    scope_type = $3,
		    scope_id = $4,
		    request_profile = $5,
		    ruleset_work = $6,
		    ruleset_version = $7,
		    status = $8,
		    message_history_json = $9::jsonb,
		    working_summary_json = $10::jsonb,
		    facts_json = $11::jsonb,
		    open_threads_json = $12::jsonb,
		    structured_state_json = $13::jsonb,
		    token_budget = $14,
		    live_turn_window = $15,
		    summary_version = $16,
		    estimated_prompt_tokens = $17,
		    estimated_summary_tokens = $18,
		    last_summarized_at = $19,
		    archived_at = $20,
		    last_active_at = $21
		WHERE id = $1
		RETURNING id::text, session_type, scope_type, scope_id, request_profile, ruleset_work, ruleset_version, status,
		          message_history_json, working_summary_json, facts_json, open_threads_json, structured_state_json,
		          token_budget, live_turn_window, summary_version, estimated_prompt_tokens, estimated_summary_tokens,
		          last_summarized_at, archived_at, last_active_at, created_at
	`,
		item.ID, item.SessionType, item.ScopeType, item.ScopeID, item.RequestProfile, item.RulesetWork, item.RulesetVersion, firstNonEmpty(item.Status, "active"),
		string(messageHistory), string(workingSummary), string(facts), string(openThreads), string(structuredState),
		item.TokenBudget, max(item.LiveTurnWindow, 8), max(item.SummaryVersion, 1), item.EstimatedPromptTokens, item.EstimatedSummaryTokens,
		item.LastSummarizedAt, item.ArchivedAt, item.LastActiveAt,
	).Scan(
		&updated.ID, &updated.SessionType, &updated.ScopeType, &updated.ScopeID, &updated.RequestProfile, &updated.RulesetWork, &updated.RulesetVersion, &updated.Status,
		&rawHistory, &rawSummary, &rawFacts, &rawThreads, &rawState,
		&updated.TokenBudget, &updated.LiveTurnWindow, &updated.SummaryVersion, &updated.EstimatedPromptTokens, &updated.EstimatedSummaryTokens,
		&updated.LastSummarizedAt, &updated.ArchivedAt, &updated.LastActiveAt, &updated.CreatedAt,
	)
	if err != nil {
		return LLMSession{}, err
	}
	updated.MessageHistory = []map[string]any{}
	updated.WorkingSummary = map[string]any{}
	updated.Facts = []string{}
	updated.OpenThreads = []string{}
	updated.StructuredState = map[string]any{}
	_ = json.Unmarshal(rawHistory, &updated.MessageHistory)
	_ = json.Unmarshal(rawSummary, &updated.WorkingSummary)
	_ = json.Unmarshal(rawFacts, &updated.Facts)
	_ = json.Unmarshal(rawThreads, &updated.OpenThreads)
	_ = json.Unmarshal(rawState, &updated.StructuredState)
	return updated, nil
}

func (s *Store) GetLLMSession(ctx context.Context, id string) (LLMSession, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id::text, session_type, scope_type, scope_id, request_profile, ruleset_work, ruleset_version,
		       status, message_history_json, working_summary_json, facts_json, open_threads_json, structured_state_json,
		       token_budget, live_turn_window, summary_version, estimated_prompt_tokens, estimated_summary_tokens,
		       last_summarized_at, archived_at, last_active_at, created_at
		FROM llm_sessions
		WHERE id = $1
	`, id)
	return scanLLMSession(row)
}

func (s *Store) GetLatestLLMSessionByScope(ctx context.Context, scopeType string, scopeID string, sessionType string) (LLMSession, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id::text, session_type, scope_type, scope_id, request_profile, ruleset_work, ruleset_version,
		       status, message_history_json, working_summary_json, facts_json, open_threads_json, structured_state_json,
		       token_budget, live_turn_window, summary_version, estimated_prompt_tokens, estimated_summary_tokens,
		       last_summarized_at, archived_at, last_active_at, created_at
		FROM llm_sessions
		WHERE scope_type = $1 AND scope_id = $2 AND session_type = $3
		ORDER BY last_active_at DESC, created_at DESC
		LIMIT 1
	`, scopeType, scopeID, sessionType)
	return scanLLMSession(row)
}

func (s *Store) CountLLMSessionsByStatus(ctx context.Context) (int, int, error) {
	var active int
	var archived int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM llm_sessions WHERE status = 'active'`).Scan(&active); err != nil {
		return 0, 0, err
	}
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM llm_sessions WHERE status = 'archived'`).Scan(&archived); err != nil {
		return 0, 0, err
	}
	return active, archived, nil
}

func (s *Store) ArchiveLLMSessionsByScope(ctx context.Context, scopeType string, scopeID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE llm_sessions
		SET status = 'archived',
		    archived_at = NOW()
		WHERE scope_type = $1 AND scope_id = $2 AND status <> 'archived'
	`, scopeType, scopeID)
	return err
}

func (s *Store) DeleteLLMSessionsByScope(ctx context.Context, scopeType string, scopeID string) (int64, error) {
	commandTag, err := s.pool.Exec(ctx, `
		DELETE FROM llm_sessions
		WHERE scope_type = $1 AND scope_id = $2
	`, scopeType, scopeID)
	if err != nil {
		return 0, err
	}
	return commandTag.RowsAffected(), nil
}

func (s *Store) ArchiveLLMSessionsForInactiveSessions(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE llm_sessions
		SET status = 'archived',
		    archived_at = NOW()
		WHERE scope_type = 'session'
		  AND status <> 'archived'
		  AND EXISTS (
		    SELECT 1
		    FROM sessions
		    WHERE sessions.id::text = llm_sessions.scope_id
		      AND sessions.status <> 'live'
		  )
	`)
	return err
}

func (s *Store) ListPlayerSlots(ctx context.Context, sessionID string) ([]PlayerSlot, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, session_id::text, character_id::text, display_name, status, joined_at, created_at
		FROM player_slots
		WHERE session_id = $1
		ORDER BY created_at ASC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]PlayerSlot, 0)
	for rows.Next() {
		var item PlayerSlot
		if err := rows.Scan(&item.ID, &item.SessionID, &item.CharacterID, &item.DisplayName, &item.Status, &item.JoinedAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) FindPlayerSlotBySessionAndDisplayName(ctx context.Context, sessionID string, displayName string) (PlayerSlot, error) {
	var slot PlayerSlot
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, session_id::text, character_id::text, display_name, status, joined_at, created_at
		FROM player_slots
		WHERE session_id = $1 AND lower(trim(display_name)) = lower(trim($2))
		ORDER BY created_at DESC
		LIMIT 1
	`, sessionID, displayName).Scan(
		&slot.ID,
		&slot.SessionID,
		&slot.CharacterID,
		&slot.DisplayName,
		&slot.Status,
		&slot.JoinedAt,
		&slot.CreatedAt,
	)
	return slot, err
}

func (s *Store) ListPlayerLinkSlots(ctx context.Context, sessionID string) ([]PlayerLinkSlot, error) {
	slots, err := s.ListPlayerSlots(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	items := make([]PlayerLinkSlot, 0, len(slots))
	for _, slot := range slots {
		item := PlayerLinkSlot{PlayerSlot: slot}
		link, err := s.GetLatestPlayerLink(ctx, slot.ID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		if err == nil {
			item.Link = &link
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Store) CreatePlayerLink(ctx context.Context, sessionID string, req CreatePlayerLinkRequest) (PlayerSlot, PlayerAccessLink, error) {
	token, err := generatePlayerToken()
	if err != nil {
		return PlayerSlot{}, PlayerAccessLink{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PlayerSlot{}, PlayerAccessLink{}, err
	}
	defer tx.Rollback(ctx)

	var slot PlayerSlot
	if err := tx.QueryRow(ctx, `
		INSERT INTO player_slots (session_id, character_id, display_name)
		VALUES ($1, $2, $3)
		RETURNING id::text, session_id::text, character_id::text, display_name, status, joined_at, created_at
	`, sessionID, req.CharacterID, req.DisplayName).Scan(
		&slot.ID, &slot.SessionID, &slot.CharacterID, &slot.DisplayName, &slot.Status, &slot.JoinedAt, &slot.CreatedAt,
	); err != nil {
		return PlayerSlot{}, PlayerAccessLink{}, err
	}

	var link PlayerAccessLink
	if err := tx.QueryRow(ctx, `
		INSERT INTO player_access_links (player_slot_id, token)
		VALUES ($1, $2)
		RETURNING id::text, player_slot_id::text, token, revoked_at, created_at
	`, slot.ID, token).Scan(&link.ID, &link.PlayerSlotID, &link.Token, &link.RevokedAt, &link.CreatedAt); err != nil {
		return PlayerSlot{}, PlayerAccessLink{}, err
	}

	visibleCharacter := map[string]any{}
	if req.CharacterID != nil {
		if character, err := s.GetCharacter(ctx, *req.CharacterID); err == nil {
			visibleCharacter = map[string]any{
				"id":              character.ID,
				"name":            character.Name,
				"race":            character.Race,
				"class_and_level": character.ClassAndLevel,
				"background":      character.Background,
				"alignment":       character.Alignment,
				"armor_class":     character.ArmorClass,
				"speed":           character.Speed,
				"hit_point_max":   character.HitPointMax,
				"abilities":       character.Abilities,
				"languages":       character.Languages,
				"features":        character.Features,
			}
		}
	}
	visibleCharacterJSON, _ := json.Marshal(visibleCharacter)
	if _, err := tx.Exec(ctx, `
		INSERT INTO player_visible_states (player_slot_id, visible_character_json)
		VALUES ($1, $2::jsonb)
	`, slot.ID, string(visibleCharacterJSON)); err != nil {
		return PlayerSlot{}, PlayerAccessLink{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return PlayerSlot{}, PlayerAccessLink{}, err
	}

	return slot, link, nil
}

func (s *Store) RegeneratePlayerLink(ctx context.Context, playerSlotID string) (PlayerAccessLink, error) {
	token, err := generatePlayerToken()
	if err != nil {
		return PlayerAccessLink{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PlayerAccessLink{}, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		UPDATE player_access_links
		SET revoked_at = COALESCE(revoked_at, NOW())
		WHERE player_slot_id = $1 AND revoked_at IS NULL
	`, playerSlotID); err != nil {
		return PlayerAccessLink{}, err
	}

	var link PlayerAccessLink
	if err := tx.QueryRow(ctx, `
		INSERT INTO player_access_links (player_slot_id, token)
		VALUES ($1, $2)
		RETURNING id::text, player_slot_id::text, token, revoked_at, created_at
	`, playerSlotID, token).Scan(&link.ID, &link.PlayerSlotID, &link.Token, &link.RevokedAt, &link.CreatedAt); err != nil {
		return PlayerAccessLink{}, err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE player_slots
		SET status = CASE WHEN status = 'joined' THEN status ELSE 'invited' END
		WHERE id = $1
	`, playerSlotID); err != nil {
		return PlayerAccessLink{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return PlayerAccessLink{}, err
	}
	return link, nil
}

func (s *Store) SetPlayerLinkRevoked(ctx context.Context, playerSlotID string, revoked bool) (PlayerAccessLink, error) {
	if revoked {
		_, err := s.pool.Exec(ctx, `
			UPDATE player_access_links
			SET revoked_at = COALESCE(revoked_at, NOW())
			WHERE player_slot_id = $1 AND revoked_at IS NULL
		`, playerSlotID)
		if err != nil {
			return PlayerAccessLink{}, err
		}
		_, err = s.pool.Exec(ctx, `
			UPDATE player_slots
			SET status = CASE WHEN status = 'joined' THEN 'joined' ELSE 'locked' END
			WHERE id = $1
		`, playerSlotID)
		if err != nil {
			return PlayerAccessLink{}, err
		}
	} else {
		_, err := s.pool.Exec(ctx, `
			UPDATE player_slots
			SET status = CASE WHEN status = 'joined' THEN 'joined' ELSE 'invited' END
			WHERE id = $1
		`, playerSlotID)
		if err != nil {
			return PlayerAccessLink{}, err
		}
	}

	return s.GetLatestPlayerLink(ctx, playerSlotID)
}

func (s *Store) GetLatestPlayerLink(ctx context.Context, playerSlotID string) (PlayerAccessLink, error) {
	var link PlayerAccessLink
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, player_slot_id::text, token, revoked_at, created_at
		FROM player_access_links
		WHERE player_slot_id = $1
		ORDER BY CASE WHEN revoked_at IS NULL THEN 0 ELSE 1 END, created_at DESC
		LIMIT 1
	`, playerSlotID).Scan(&link.ID, &link.PlayerSlotID, &link.Token, &link.RevokedAt, &link.CreatedAt)
	return link, err
}

func (s *Store) GetPlayerPortalSession(ctx context.Context, token string) (PlayerPortalSession, error) {
	var portal PlayerPortalSession
	var rawState []byte
	var rawVisibleCharacter []byte
	var rawVisibleHandouts []byte
	var rawVisibleMedia []byte
	var characterID *string

	err := s.pool.QueryRow(ctx, `
		SELECT
			pal.token,
			s.id::text, s.campaign_id::text, s.name, s.adventure_id::text, s.ruleset_work, s.ruleset_version, s.target_player_count, s.join_token, s.status, s.current_scene, s.current_location, s.language, s.default_voice_profile_id, s.state_json, s.created_at, s.updated_at,
			ps.id::text, ps.session_id::text, ps.character_id::text, ps.display_name, ps.status, ps.joined_at, ps.created_at,
			pvs.id::text, pvs.player_slot_id::text, pvs.visible_character_json, pvs.visible_handouts_json, pvs.visible_media_json, pvs.updated_at
		FROM player_access_links pal
		JOIN player_slots ps ON ps.id = pal.player_slot_id
		JOIN sessions s ON s.id = ps.session_id
		JOIN player_visible_states pvs ON pvs.player_slot_id = ps.id
		WHERE pal.token = $1 AND pal.revoked_at IS NULL
	`, token).Scan(
		&portal.Token,
		&portal.Session.ID, &portal.Session.CampaignID, &portal.Session.Name, &portal.Session.AdventureID, &portal.Session.RulesetWork, &portal.Session.RulesetVersion, &portal.Session.TargetPlayerCount, &portal.Session.JoinToken, &portal.Session.Status, &portal.Session.CurrentScene, &portal.Session.CurrentLocation, &portal.Session.Language, &portal.Session.DefaultVoiceProfileID, &rawState, &portal.Session.CreatedAt, &portal.Session.UpdatedAt,
		&portal.PlayerSlot.ID, &portal.PlayerSlot.SessionID, &characterID, &portal.PlayerSlot.DisplayName, &portal.PlayerSlot.Status, &portal.PlayerSlot.JoinedAt, &portal.PlayerSlot.CreatedAt,
		&portal.VisibleState.ID, &portal.VisibleState.PlayerSlotID, &rawVisibleCharacter, &rawVisibleHandouts, &rawVisibleMedia, &portal.VisibleState.UpdatedAt,
	)
	if err != nil {
		return PlayerPortalSession{}, err
	}
	portal.PlayerSlot.CharacterID = characterID
	portal.Session.State = defaultSessionState()
	portal.VisibleState.VisibleCharacter = map[string]any{}
	portal.VisibleState.VisibleHandouts = []map[string]any{}
	portal.VisibleState.VisibleMedia = []map[string]any{}
	if len(rawState) > 0 {
		_ = json.Unmarshal(rawState, &portal.Session.State)
	}
	_ = json.Unmarshal(rawVisibleCharacter, &portal.VisibleState.VisibleCharacter)
	_ = json.Unmarshal(rawVisibleHandouts, &portal.VisibleState.VisibleHandouts)
	_ = json.Unmarshal(rawVisibleMedia, &portal.VisibleState.VisibleMedia)

	if characterID != nil {
		if character, err := s.GetCharacter(ctx, *characterID); err == nil {
			portal.Character = &character
		}
	}

	return portal, nil
}

func (s *Store) UpdatePlayerSlotCharacter(ctx context.Context, playerSlotID string, characterID string) (PlayerSlot, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PlayerSlot{}, err
	}
	defer tx.Rollback(ctx)

	var slot PlayerSlot
	err = tx.QueryRow(ctx, `
		UPDATE player_slots
		SET character_id = $2
		WHERE id = $1
		RETURNING id::text, session_id::text, character_id::text, display_name, status, joined_at, created_at
	`, playerSlotID, characterID).Scan(
		&slot.ID,
		&slot.SessionID,
		&slot.CharacterID,
		&slot.DisplayName,
		&slot.Status,
		&slot.JoinedAt,
		&slot.CreatedAt,
	)
	if err != nil {
		return PlayerSlot{}, err
	}

	visibleCharacter := map[string]any{}
	if character, err := s.GetCharacter(ctx, characterID); err == nil {
		visibleCharacter = map[string]any{
			"id":              character.ID,
			"name":            character.Name,
			"race":            character.Race,
			"class_and_level": character.ClassAndLevel,
			"background":      character.Background,
			"alignment":       character.Alignment,
			"armor_class":     character.ArmorClass,
			"speed":           character.Speed,
			"hit_point_max":   character.HitPointMax,
			"abilities":       character.Abilities,
			"languages":       character.Languages,
			"features":        character.Features,
		}
	}
	visibleCharacterJSON, _ := json.Marshal(visibleCharacter)
	if _, err := tx.Exec(ctx, `
		UPDATE player_visible_states
		SET visible_character_json = $2::jsonb, updated_at = NOW()
		WHERE player_slot_id = $1
	`, playerSlotID, string(visibleCharacterJSON)); err != nil {
		return PlayerSlot{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return PlayerSlot{}, err
	}
	return slot, nil
}

func (s *Store) UpdatePlayerSlotStatus(ctx context.Context, playerSlotID string, status string) (PlayerSlot, error) {
	var slot PlayerSlot
	err := s.pool.QueryRow(ctx, `
		UPDATE player_slots
		SET status = $2,
		    joined_at = CASE WHEN $2 IN ('joined', 'ready') THEN COALESCE(joined_at, NOW()) ELSE joined_at END
		WHERE id = $1
		RETURNING id::text, session_id::text, character_id::text, display_name, status, joined_at, created_at
	`, playerSlotID, status).Scan(
		&slot.ID,
		&slot.SessionID,
		&slot.CharacterID,
		&slot.DisplayName,
		&slot.Status,
		&slot.JoinedAt,
		&slot.CreatedAt,
	)
	return slot, err
}

func (s *Store) MarkPlayerLinkJoined(ctx context.Context, token string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE player_slots
		SET status = CASE WHEN status = 'ready' THEN 'ready' ELSE 'joined' END,
		    joined_at = COALESCE(joined_at, NOW())
		WHERE id = (
			SELECT player_slot_id FROM player_access_links
			WHERE token = $1 AND revoked_at IS NULL
		)
	`, token)
	return err
}

func (s *Store) ListDocumentsByIDs(ctx context.Context, ids []string) ([]Document, error) {
	if len(ids) == 0 {
		return []Document{}, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT d.id::text, d.adventure_id::text, d.type, d.name, d.source_file_path, d.metadata_json, d.created_at, COUNT(dc.id) AS chunk_count
		FROM documents d
		LEFT JOIN document_chunks dc ON dc.document_id = d.id
		WHERE d.id = ANY($1::uuid[])
		GROUP BY d.id, d.type, d.name, d.source_file_path, d.metadata_json, d.created_at
		ORDER BY d.created_at DESC
	`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Document, 0)
	for rows.Next() {
		item, err := scanDocument(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListAssetsByIDs(ctx context.Context, ids []string) ([]Asset, error) {
	if len(ids) == 0 {
		return []Asset{}, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, adventure_id::text, document_id::text, type, source_type, name, file_path, mime_type, entity_name, location_name, tags_json, metadata_json, created_at
		FROM assets
		WHERE id = ANY($1::uuid[])
		ORDER BY created_at DESC
	`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Asset, 0)
	for rows.Next() {
		item, err := scanAsset(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) UpdatePlayerVisibleState(ctx context.Context, playerSlotID string, handouts []Document, media []Asset) (PlayerVisibleState, error) {
	visibleHandouts := make([]map[string]any, 0, len(handouts))
	for _, document := range handouts {
		visibleHandouts = append(visibleHandouts, map[string]any{
			"id":               document.ID,
			"name":             document.Name,
			"type":             document.Type,
			"source_file_path": document.SourceFilePath,
			"metadata":         document.Metadata,
		})
	}

	visibleMedia := make([]map[string]any, 0, len(media))
	for _, asset := range media {
		visibleMedia = append(visibleMedia, map[string]any{
			"id":            asset.ID,
			"name":          asset.Name,
			"type":          asset.Type,
			"file_path":     asset.FilePath,
			"mime_type":     asset.MimeType,
			"entity_name":   asset.EntityName,
			"location_name": asset.LocationName,
			"tags":          asset.Tags,
			"metadata":      asset.Metadata,
		})
	}

	handoutsJSON, err := json.Marshal(visibleHandouts)
	if err != nil {
		return PlayerVisibleState{}, err
	}
	mediaJSON, err := json.Marshal(visibleMedia)
	if err != nil {
		return PlayerVisibleState{}, err
	}

	var state PlayerVisibleState
	var rawCharacter []byte
	var rawHandouts []byte
	var rawMedia []byte
	err = s.pool.QueryRow(ctx, `
		UPDATE player_visible_states
		SET visible_handouts_json = $2::jsonb,
		    visible_media_json = $3::jsonb,
		    updated_at = NOW()
		WHERE player_slot_id = $1
		RETURNING id::text, player_slot_id::text, visible_character_json, visible_handouts_json, visible_media_json, updated_at
	`, playerSlotID, string(handoutsJSON), string(mediaJSON)).Scan(
		&state.ID, &state.PlayerSlotID, &rawCharacter, &rawHandouts, &rawMedia, &state.UpdatedAt,
	)
	if err != nil {
		return PlayerVisibleState{}, err
	}
	state.VisibleCharacter = map[string]any{}
	state.VisibleHandouts = []map[string]any{}
	state.VisibleMedia = []map[string]any{}
	_ = json.Unmarshal(rawCharacter, &state.VisibleCharacter)
	_ = json.Unmarshal(rawHandouts, &state.VisibleHandouts)
	_ = json.Unmarshal(rawMedia, &state.VisibleMedia)
	return state, nil
}

func (s *Store) CreateAdventure(ctx context.Context, req CreateAdventureRequest) (Adventure, error) {
	language := req.Language
	if language == "" {
		language = "de"
	}
	metadata, err := json.Marshal(defaultMetadata(req.Metadata))
	if err != nil {
		return Adventure{}, err
	}

	var item Adventure
	err = s.pool.QueryRow(ctx, `
		INSERT INTO adventures (campaign_id, name, description, language, metadata_json)
		VALUES ($1, $2, $3, $4, $5::jsonb)
		RETURNING id::text, campaign_id::text, name, description, language, metadata_json, created_at
	`, req.CampaignID, req.Name, req.Description, language, string(metadata)).Scan(
		&item.ID,
		&item.CampaignID,
		&item.Name,
		&item.Description,
		&item.Language,
		&metadata,
		&item.CreatedAt,
	)
	item.Metadata = map[string]any{}
	_ = json.Unmarshal(metadata, &item.Metadata)
	return item, err
}

func (s *Store) ListSessions(ctx context.Context) ([]Session, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, campaign_id::text, name, adventure_id::text, ruleset_work, ruleset_version, target_player_count, join_token, status, current_scene, current_location, language, default_voice_profile_id, state_json, created_at, updated_at
		FROM sessions
		ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Session, 0)
	for rows.Next() {
		var item Session
		var rawState []byte
		if err := rows.Scan(
			&item.ID,
			&item.CampaignID,
			&item.Name,
			&item.AdventureID,
			&item.RulesetWork,
			&item.RulesetVersion,
			&item.TargetPlayerCount,
			&item.JoinToken,
			&item.Status,
			&item.CurrentScene,
			&item.CurrentLocation,
			&item.Language,
			&item.DefaultVoiceProfileID,
			&rawState,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		item.State = defaultSessionState()
		if len(rawState) > 0 {
			if err := json.Unmarshal(rawState, &item.State); err != nil {
				return nil, err
			}
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

func (s *Store) GetSession(ctx context.Context, id string) (Session, error) {
	var item Session
	var rawState []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, campaign_id::text, name, adventure_id::text, ruleset_work, ruleset_version, target_player_count, join_token, status, current_scene, current_location, language, default_voice_profile_id, state_json, created_at, updated_at
		FROM sessions
		WHERE id = $1
	`, id).Scan(
		&item.ID,
		&item.CampaignID,
		&item.Name,
		&item.AdventureID,
		&item.RulesetWork,
		&item.RulesetVersion,
		&item.TargetPlayerCount,
		&item.JoinToken,
		&item.Status,
		&item.CurrentScene,
		&item.CurrentLocation,
		&item.Language,
		&item.DefaultVoiceProfileID,
		&rawState,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return item, err
	}
	item.State = defaultSessionState()
	if len(rawState) > 0 {
		if err := json.Unmarshal(rawState, &item.State); err != nil {
			return Session{}, err
		}
	}
	return item, err
}

func (s *Store) CreateSession(ctx context.Context, req CreateSessionRequest) (Session, error) {
	status := req.Status
	if status == "" {
		status = "draft"
	}

	language := req.Language
	if language == "" {
		language = "de"
	}

	statePayload, err := json.Marshal(defaultSessionState())
	if err != nil {
		return Session{}, err
	}

	joinToken, err := generateSessionToken()
	if err != nil {
		return Session{}, err
	}

	var item Session
	var rawState []byte
	err = s.pool.QueryRow(ctx, `
		INSERT INTO sessions (
			campaign_id,
			name,
			adventure_id,
			ruleset_work,
			ruleset_version,
			target_player_count,
			join_token,
			status,
			current_scene,
			current_location,
			language,
			default_voice_profile_id,
			state_json
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::jsonb)
		RETURNING id::text, campaign_id::text, name, adventure_id::text, ruleset_work, ruleset_version, target_player_count, join_token, status, current_scene, current_location, language, default_voice_profile_id, state_json, created_at, updated_at
	`, req.CampaignID, req.Name, req.AdventureID, req.RulesetWork, req.RulesetVersion, req.TargetPlayerCount, joinToken, status, req.CurrentScene, req.CurrentLocation, language, req.DefaultVoiceProfileID, string(statePayload)).Scan(
		&item.ID,
		&item.CampaignID,
		&item.Name,
		&item.AdventureID,
		&item.RulesetWork,
		&item.RulesetVersion,
		&item.TargetPlayerCount,
		&item.JoinToken,
		&item.Status,
		&item.CurrentScene,
		&item.CurrentLocation,
		&item.Language,
		&item.DefaultVoiceProfileID,
		&rawState,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return item, err
	}
	item.State = defaultSessionState()
	if len(rawState) > 0 {
		if err := json.Unmarshal(rawState, &item.State); err != nil {
			return Session{}, err
		}
	}
	return item, err
}

func (s *Store) UpdateSessionStatus(ctx context.Context, sessionID string, status string) (Session, error) {
	var item Session
	var rawState []byte
	err := s.pool.QueryRow(ctx, `
		UPDATE sessions
		SET status = $2, updated_at = NOW()
		WHERE id = $1
		RETURNING id::text, campaign_id::text, name, adventure_id::text, ruleset_work, ruleset_version, target_player_count, join_token, status, current_scene, current_location, language, default_voice_profile_id, state_json, created_at, updated_at
	`, sessionID, status).Scan(
		&item.ID,
		&item.CampaignID,
		&item.Name,
		&item.AdventureID,
		&item.RulesetWork,
		&item.RulesetVersion,
		&item.TargetPlayerCount,
		&item.JoinToken,
		&item.Status,
		&item.CurrentScene,
		&item.CurrentLocation,
		&item.Language,
		&item.DefaultVoiceProfileID,
		&rawState,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return Session{}, err
	}
	item.State = defaultSessionState()
	if len(rawState) > 0 {
		if err := json.Unmarshal(rawState, &item.State); err != nil {
			return Session{}, err
		}
	}
	return item, nil
}

func (s *Store) UpdateSession(ctx context.Context, sessionID string, req UpdateSessionRequest) (Session, error) {
	language := req.Language
	if strings.TrimSpace(language) == "" {
		language = "de"
	}

	current, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return Session{}, err
	}
	nextState := current.State
	if req.SelectedRulebookIDs != nil {
		nextState.SelectedRulebookIDs = req.SelectedRulebookIDs
	}
	nextState.PromptConfig = req.PromptConfig
	nextState.GroupInventory = SessionGroupInventory{
		Gold:  req.GroupInventory.Gold,
		Items: defaultStringSlice(req.GroupInventory.Items),
		Notes: req.GroupInventory.Notes,
	}
	statePayload, err := json.Marshal(nextState)
	if err != nil {
		return Session{}, err
	}

	var item Session
	var rawState []byte
	err = s.pool.QueryRow(ctx, `
		UPDATE sessions
		SET
			campaign_id = $2,
			name = $3,
			adventure_id = $4,
			ruleset_work = $5,
			ruleset_version = $6,
			target_player_count = $7,
			current_scene = $8,
			current_location = $9,
			language = $10,
			default_voice_profile_id = $11,
			state_json = $12::jsonb,
			updated_at = NOW()
		WHERE id = $1
		RETURNING id::text, campaign_id::text, name, adventure_id::text, ruleset_work, ruleset_version, target_player_count, join_token, status, current_scene, current_location, language, default_voice_profile_id, state_json, created_at, updated_at
	`, sessionID, req.CampaignID, req.Name, req.AdventureID, req.RulesetWork, req.RulesetVersion, req.TargetPlayerCount, req.CurrentScene, req.CurrentLocation, language, req.DefaultVoiceProfileID, string(statePayload)).Scan(
		&item.ID,
		&item.CampaignID,
		&item.Name,
		&item.AdventureID,
		&item.RulesetWork,
		&item.RulesetVersion,
		&item.TargetPlayerCount,
		&item.JoinToken,
		&item.Status,
		&item.CurrentScene,
		&item.CurrentLocation,
		&item.Language,
		&item.DefaultVoiceProfileID,
		&rawState,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return Session{}, err
	}
	item.State = defaultSessionState()
	if len(rawState) > 0 {
		if err := json.Unmarshal(rawState, &item.State); err != nil {
			return Session{}, err
		}
	}
	return item, nil
}

func (s *Store) UpdateSessionState(ctx context.Context, sessionID string, state SessionState) error {
	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}

	_, err = s.pool.Exec(ctx, `
		UPDATE sessions
		SET state_json = $2::jsonb, updated_at = NOW()
		WHERE id = $1
	`, sessionID, string(payload))
	return err
}

func (s *Store) DeleteSession(ctx context.Context, sessionID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, sessionID)
	return err
}

func (s *Store) ListDocumentsForCampaign(ctx context.Context, campaignID string) ([]Document, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT d.id::text, d.adventure_id::text, d.type, d.name, d.source_file_path, d.metadata_json, d.created_at, COUNT(dc.id) AS chunk_count
		FROM documents d
		LEFT JOIN document_chunks dc ON dc.document_id = d.id
		WHERE d.metadata_json->>'campaign_id' = $1 OR d.metadata_json->>'campaign_id' IS NULL
		GROUP BY d.id, d.type, d.name, d.source_file_path, d.metadata_json, d.created_at
		ORDER BY d.created_at DESC
	`, campaignID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Document, 0)
	for rows.Next() {
		item, err := scanDocument(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

func (s *Store) ReplaceDocumentChunks(ctx context.Context, documentID string, chunks []string, metadata map[string]any) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM document_chunks WHERE document_id = $1`, documentID); err != nil {
		return err
	}

	encodedMetadata, err := json.Marshal(defaultMetadata(metadata))
	if err != nil {
		return err
	}

	for index, chunk := range chunks {
		if _, err := tx.Exec(ctx, `
			INSERT INTO document_chunks (document_id, chunk_text, chunk_index, metadata_json)
			VALUES ($1, $2, $3, $4::jsonb)
		`, documentID, chunk, index, string(encodedMetadata)); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (s *Store) ReplaceMonsterReferences(ctx context.Context, documentID string, refs []MonsterReference) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM monster_references WHERE document_id = $1`, documentID); err != nil {
		return err
	}

	for _, ref := range refs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO monster_references (document_id, name, name_slug, chunk_index)
			VALUES ($1, $2, $3, $4)
		`, documentID, ref.Name, ref.NameSlug, ref.ChunkIndex); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (s *Store) ReplaceRuleIndexEntries(ctx context.Context, documentID string, entries []RuleIndexEntry) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM rule_index_entries WHERE document_id = $1`, documentID); err != nil {
		return err
	}

	if len(entries) == 0 {
		return tx.Commit(ctx)
	}

	for _, entry := range entries {
		if _, err := tx.Exec(ctx, `
			INSERT INTO rule_index_entries (document_id, chunk_index, category, term, term_slug)
			VALUES ($1, $2, $3, $4, $5)
		`, documentID, entry.ChunkIndex, entry.Category, entry.Term, entry.TermSlug); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (s *Store) ListDocumentChunks(ctx context.Context, documentID string) ([]DocumentChunk, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, document_id::text, chunk_text, chunk_index, metadata_json, created_at
		FROM document_chunks
		WHERE document_id = $1
		ORDER BY chunk_index ASC
	`, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]DocumentChunk, 0)
	for rows.Next() {
		var item DocumentChunk
		var rawMetadata []byte
		if err := rows.Scan(&item.ID, &item.DocumentID, &item.ChunkText, &item.ChunkIndex, &rawMetadata, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.Metadata = map[string]any{}
		if len(rawMetadata) > 0 {
			if err := json.Unmarshal(rawMetadata, &item.Metadata); err != nil {
				return nil, err
			}
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

func (s *Store) backfillRuleIndex(ctx context.Context) error {
	log.Printf("backfill rule index: start")
	rows, err := s.pool.Query(ctx, `
		SELECT d.id::text, d.name, COALESCE(d.source_file_path, '')
		FROM documents d
		LEFT JOIN rule_index_entries rie ON rie.document_id = d.id
		WHERE d.type = 'rules'
		  AND COALESCE(d.source_file_path, '') <> ''
		GROUP BY d.id, d.name, d.source_file_path
		HAVING COUNT(rie.id) = 0
		ORDER BY d.created_at ASC
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var documentID string
		var documentName string
		var sourceFilePath string
		if err := rows.Scan(&documentID, &documentName, &sourceFilePath); err != nil {
			log.Printf("backfill rule index: scan failed: %v", err)
			continue
		}
		log.Printf("backfill rule index: document=%s name=%s", documentID, documentName)
		chunkTexts := []string{}
		if strings.TrimSpace(sourceFilePath) != "" {
			if text, err := extractDocumentText(sourceFilePath); err == nil {
				log.Printf("backfill rule index: extracted %d bytes from %s", len(text), sourceFilePath)
				chunkTexts = chunkDocumentText(text, 1200)
				if len(chunkTexts) > 0 {
					if err := s.ReplaceDocumentChunks(ctx, documentID, chunkTexts, map[string]any{"source_type": "rule_index_backfill"}); err != nil {
						log.Printf("backfill rule index: replace chunks failed for %s: %v", documentID, err)
						continue
					}
					log.Printf("backfill rule index: replaced %d chunks for %s", len(chunkTexts), documentID)
				}
			} else {
				log.Printf("backfill rule index: extract failed for %s: %v", sourceFilePath, err)
			}
		}
		if len(chunkTexts) == 0 {
			chunks, err := s.ListDocumentChunks(ctx, documentID)
			if err != nil {
				log.Printf("backfill rule index: list chunks failed for %s: %v", documentID, err)
				continue
			}
			chunkTexts = make([]string, 0, len(chunks))
			for _, chunk := range chunks {
				chunkTexts = append(chunkTexts, chunk.ChunkText)
			}
		}
		entries := extractRuleIndexEntries(documentName, chunkTexts)
		log.Printf("backfill rule index: generated %d entries for %s", len(entries), documentID)
		if err := s.ReplaceRuleIndexEntries(ctx, documentID, entries); err != nil {
			log.Printf("backfill rule index: replace entries failed for %s: %v", documentID, err)
			continue
		}
	}
	log.Printf("backfill rule index: done")
	return rows.Err()
}

func (s *Store) RetrieveMonsterContext(ctx context.Context, monsterName string, limit int) ([]GMContextChunk, error) {
	if limit <= 0 {
		limit = 4
	}

	slug := slugifySearch(monsterName)
	if slug == "" {
		return []GMContextChunk{}, nil
	}

	rows, err := s.pool.Query(ctx, `
		WITH matched_monsters AS (
			SELECT
				mr.document_id,
				mr.name_slug,
				mr.chunk_index,
				d.name AS document_name,
				CASE
					WHEN mr.name_slug = $1 THEN 0
					WHEN mr.name_slug LIKE '%' || $1 || '%' THEN 1
					WHEN $1 LIKE '%' || mr.name_slug || '%' THEN 2
					ELSE 3
				END AS match_rank
			FROM monster_references mr
			JOIN documents d ON d.id = mr.document_id
			WHERE mr.name_slug = $1
			   OR mr.name_slug LIKE '%' || $1 || '%'
			   OR $1 LIKE '%' || mr.name_slug || '%'
			ORDER BY
				CASE
					WHEN mr.chunk_index < 50 THEN 1
					ELSE 0
				END ASC,
				CASE
					WHEN mr.name_slug = $1 AND mr.chunk_index >= 50 THEN 0
					WHEN mr.name_slug LIKE '%' || $1 || '%' AND mr.chunk_index >= 50 THEN 1
					WHEN $1 LIKE '%' || mr.name_slug || '%' AND mr.chunk_index >= 50 THEN 2
					ELSE 3
				END ASC,
				match_rank ASC,
				ABS(LENGTH(mr.name_slug) - LENGTH($1)) ASC,
				mr.chunk_index ASC
		),
		best_match AS (
			SELECT document_id, chunk_index, document_name, match_rank, name_slug
			FROM matched_monsters
			ORDER BY
				CASE
					WHEN chunk_index < 50 THEN 1
					ELSE 0
				END ASC,
				match_rank ASC,
				ABS(LENGTH(name_slug) - LENGTH($1)) ASC,
				chunk_index ASC
			LIMIT 1
		)
		SELECT
			bm.document_id::text,
			bm.document_name,
			dc.chunk_text
		FROM best_match bm
		JOIN document_chunks dc
			ON dc.document_id = bm.document_id
		   AND dc.chunk_index BETWEEN GREATEST(bm.chunk_index, 0) AND bm.chunk_index + 2
		ORDER BY
			dc.chunk_index ASC
		LIMIT $2
	`, slug, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]GMContextChunk, 0)
	for rows.Next() {
		var item GMContextChunk
		if err := rows.Scan(&item.DocumentID, &item.DocumentName, &item.ChunkText); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

func (s *Store) RetrieveRelevantChunks(ctx context.Context, campaignID string, query string, limit int) ([]GMContextChunk, error) {
	if limit <= 0 {
		limit = 4
	}

	trimmedQuery := strings.TrimSpace(query)
	patterns := buildSearchPatterns(trimmedQuery)
	prioritizeRules := isLikelyRulesQuery(trimmedQuery)

	if indexed, err := s.retrieveRelevantChunksViaIndex(ctx, campaignID, patterns, limit, prioritizeRules); err == nil && len(indexed) > 0 {
		return indexed, nil
	}

	rows, err := s.pool.Query(ctx, `
		WITH ranked_chunks AS (
			SELECT
				d.id::text AS document_id,
				d.name AS document_name,
				d.type AS document_type,
				dc.chunk_text,
				dc.chunk_index,
				(
					CASE
						WHEN $3 <> '' AND dc.chunk_text ILIKE '%' || $3 || '%' THEN 100
						ELSE 0
					END
				)
				+
				(
					SELECT COUNT(*)::INT * 6
					FROM unnest($2::text[]) AS pattern
					WHERE dc.chunk_text ILIKE pattern
				)
				+
				(
					CASE
						WHEN $4 AND d.type = 'rules' THEN 25
						WHEN $4 AND d.type <> 'rules' THEN -45
						ELSE 0
					END
				)
				+
				(
					CASE
						WHEN d.type = 'rules' AND dc.chunk_index < 8 THEN -90
						WHEN d.type = 'rules' AND dc.chunk_index < 20 THEN -40
						ELSE 0
					END
				)
				+
				(
					CASE
						WHEN d.type = 'rules' AND (
							dc.chunk_text ILIKE '%impressum%'
							OR dc.chunk_text ILIKE '%credits%'
							OR dc.chunk_text ILIKE '%on the cover%'
							OR dc.chunk_text ILIKE '%isbn%'
							OR dc.chunk_text ILIKE '%printed in the usa%'
							OR dc.chunk_text ILIKE '%wizards of the coast%'
							OR dc.chunk_text ILIKE '%lead designers%'
							OR dc.chunk_text ILIKE '%deutsche ausgabe%'
						) THEN -80
						ELSE 0
					END
				)
				+
				(
					CASE
						WHEN d.type = 'rules' AND (
							LENGTH(dc.chunk_text) > 0
							AND (
								LENGTH(regexp_replace(dc.chunk_text, '[[:alpha:][:space:][:digit:][:punct:]]', '', 'g')) > 24
								OR regexp_count(dc.chunk_text, '[[:lower:]][[:upper:]]') > 18
							)
						) THEN -55
						ELSE 0
					END
				) AS score
			FROM document_chunks dc
			JOIN documents d ON d.id = dc.document_id
			WHERE (
				(
					$4
					AND d.type = 'rules'
				)
				OR (
					NOT $4
					AND (
						d.metadata_json->>'campaign_id' = $1
						OR d.metadata_json->>'campaign_id' IS NULL
						OR d.type = 'rules'
					)
				)
			)
			AND (
				($3 <> '' AND dc.chunk_text ILIKE '%' || $3 || '%')
				OR EXISTS (
					SELECT 1
					FROM unnest($2::text[]) AS pattern
					WHERE dc.chunk_text ILIKE pattern
				)
			)
		)
		SELECT document_id, document_name, chunk_text
		FROM ranked_chunks
		ORDER BY score DESC, chunk_index ASC
		LIMIT $5
	`, campaignID, patterns, trimmedQuery, prioritizeRules, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]GMContextChunk, 0)
	for rows.Next() {
		var item GMContextChunk
		if err := rows.Scan(&item.DocumentID, &item.DocumentName, &item.ChunkText); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(items) > 0 || trimmedQuery == "" {
		return items, nil
	}

	return s.retrieveFallbackChunks(ctx, campaignID, limit, prioritizeRules)
}

func (s *Store) retrieveRelevantChunksViaIndex(ctx context.Context, campaignID string, patterns []string, limit int, prioritizeRules bool) ([]GMContextChunk, error) {
	if limit <= 0 {
		limit = 4
	}
	if campaignID == "" || len(patterns) == 0 {
		return []GMContextChunk{}, nil
	}

	rows, err := s.pool.Query(ctx, `
		WITH ranked_hits AS (
			SELECT
				d.id::text AS document_id,
				d.name AS document_name,
				dc.chunk_text,
				dc.chunk_index,
				COUNT(DISTINCT rie.term_slug)
				+
				CASE
					WHEN $4 AND d.type = 'rules' THEN 25
					WHEN $4 AND d.type <> 'rules' THEN -45
					ELSE 0
				END AS score
			FROM rule_index_entries rie
			JOIN document_chunks dc
				ON dc.document_id = rie.document_id
			   AND dc.chunk_index = rie.chunk_index
			JOIN documents d ON d.id = dc.document_id
			WHERE (
				($4 AND d.type = 'rules')
				OR (
					NOT $4
					AND (
						d.metadata_json->>'campaign_id' = $1
						OR d.metadata_json->>'campaign_id' IS NULL
						OR d.type = 'rules'
					)
				)
			)
			  AND EXISTS (
				SELECT 1
				FROM unnest($2::text[]) AS pattern
				WHERE rie.term_slug LIKE pattern
				   OR rie.term LIKE pattern
			  )
			GROUP BY d.id, d.name, dc.chunk_text, dc.chunk_index
		)
		SELECT document_id, document_name, chunk_text
		FROM ranked_hits
		ORDER BY score DESC, chunk_index ASC
		LIMIT $3
	`, campaignID, patterns, limit, prioritizeRules)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]GMContextChunk, 0)
	for rows.Next() {
		var item GMContextChunk
		if err := rows.Scan(&item.DocumentID, &item.DocumentName, &item.ChunkText); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) RetrieveRelevantChunksForDocuments(ctx context.Context, documentIDs []string, query string, limit int, prioritizeRules bool) ([]GMContextChunk, error) {
	if limit <= 0 {
		limit = 4
	}
	if len(documentIDs) == 0 {
		return []GMContextChunk{}, nil
	}

	trimmedQuery := strings.TrimSpace(query)
	patterns := buildSearchPatterns(trimmedQuery)

	if indexed, err := s.retrieveRelevantChunksForDocumentsViaIndex(ctx, documentIDs, patterns, limit, prioritizeRules); err == nil && len(indexed) > 0 {
		return indexed, nil
	}

	rows, err := s.pool.Query(ctx, `
		WITH ranked_chunks AS (
			SELECT
				d.id::text AS document_id,
				d.name AS document_name,
				d.type AS document_type,
				dc.chunk_text,
				dc.chunk_index,
				(
					CASE
						WHEN $3 <> '' AND dc.chunk_text ILIKE '%' || $3 || '%' THEN 100
						ELSE 0
					END
				)
				+
				(
					SELECT COUNT(*)::INT * 6
					FROM unnest($2::text[]) AS pattern
					WHERE dc.chunk_text ILIKE pattern
				)
				+
				(
					CASE
						WHEN $4 AND d.type = 'rules' THEN 25
						WHEN $4 AND d.type <> 'rules' THEN -45
						ELSE 0
					END
				) AS score
			FROM document_chunks dc
			JOIN documents d ON d.id = dc.document_id
			WHERE d.id = ANY($1::uuid[])
			AND (
				($3 <> '' AND dc.chunk_text ILIKE '%' || $3 || '%')
				OR EXISTS (
					SELECT 1
					FROM unnest($2::text[]) AS pattern
					WHERE dc.chunk_text ILIKE pattern
				)
			)
		)
		SELECT document_id, document_name, chunk_text
		FROM ranked_chunks
		ORDER BY score DESC, chunk_index ASC
		LIMIT $5
	`, documentIDs, patterns, trimmedQuery, prioritizeRules, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]GMContextChunk, 0)
	for rows.Next() {
		var item GMContextChunk
		if err := rows.Scan(&item.DocumentID, &item.DocumentName, &item.ChunkText); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(items) > 0 || trimmedQuery == "" {
		return items, nil
	}
	return s.retrieveFallbackChunksForDocuments(ctx, documentIDs, limit, prioritizeRules)
}

func (s *Store) retrieveRelevantChunksForDocumentsViaIndex(ctx context.Context, documentIDs []string, patterns []string, limit int, prioritizeRules bool) ([]GMContextChunk, error) {
	if limit <= 0 {
		limit = 4
	}
	if len(documentIDs) == 0 || len(patterns) == 0 {
		return []GMContextChunk{}, nil
	}

	rows, err := s.pool.Query(ctx, `
		WITH ranked_hits AS (
			SELECT
				d.id::text AS document_id,
				d.name AS document_name,
				dc.chunk_text,
				dc.chunk_index,
				COUNT(DISTINCT rie.term_slug)
				+
				CASE
					WHEN $4 AND d.type = 'rules' THEN 25
					WHEN $4 AND d.type <> 'rules' THEN -45
					ELSE 0
				END AS score
			FROM rule_index_entries rie
			JOIN document_chunks dc
				ON dc.document_id = rie.document_id
			   AND dc.chunk_index = rie.chunk_index
			JOIN documents d ON d.id = dc.document_id
			WHERE d.id = ANY($1::uuid[])
			  AND EXISTS (
				SELECT 1
				FROM unnest($2::text[]) AS pattern
				WHERE rie.term_slug LIKE pattern
				   OR rie.term LIKE pattern
			  )
			GROUP BY d.id, d.name, dc.chunk_text, dc.chunk_index
		)
		SELECT document_id, document_name, chunk_text
		FROM ranked_hits
		ORDER BY score DESC, chunk_index ASC
		LIMIT $3
		`, documentIDs, patterns, limit, prioritizeRules)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]GMContextChunk, 0)
	for rows.Next() {
		var item GMContextChunk
		if err := rows.Scan(&item.DocumentID, &item.DocumentName, &item.ChunkText); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) retrieveFallbackChunksForDocuments(ctx context.Context, documentIDs []string, limit int, prioritizeRules bool) ([]GMContextChunk, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT d.id::text, d.name, dc.chunk_text
		FROM document_chunks dc
		JOIN documents d ON d.id = dc.document_id
		WHERE d.id = ANY($1::uuid[])
		ORDER BY
			CASE
				WHEN $3 AND d.type = 'rules' THEN 0
				ELSE 1
			END,
			d.created_at DESC,
			dc.chunk_index ASC
		LIMIT $2
	`, documentIDs, limit, prioritizeRules)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]GMContextChunk, 0)
	for rows.Next() {
		var item GMContextChunk
		if err := rows.Scan(&item.DocumentID, &item.DocumentName, &item.ChunkText); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) retrieveFallbackChunks(ctx context.Context, campaignID string, limit int, prioritizeRules bool) ([]GMContextChunk, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT d.id::text, d.name, dc.chunk_text
		FROM document_chunks dc
		JOIN documents d ON d.id = dc.document_id
		WHERE
			($3 AND d.type = 'rules')
			OR (
				NOT $3
				AND (
					d.metadata_json->>'campaign_id' = $1
					OR d.metadata_json->>'campaign_id' IS NULL
					OR d.type = 'rules'
				)
			)
		ORDER BY
			CASE
				WHEN $3 AND d.type = 'rules' THEN 0
				ELSE 1
			END,
			d.created_at DESC,
			dc.chunk_index ASC
		LIMIT $2
	`, campaignID, limit, prioritizeRules)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]GMContextChunk, 0)
	for rows.Next() {
		var item GMContextChunk
		if err := rows.Scan(&item.DocumentID, &item.DocumentName, &item.ChunkText); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

func buildSearchPatterns(query string) []string {
	if strings.TrimSpace(query) == "" {
		return []string{}
	}

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
	)

	parts := strings.Fields(replacer.Replace(strings.ToLower(query)))
	patterns := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
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
		patterns = append(patterns, "%"+part+"%")
	}

	for _, extra := range expandedSearchTokens(parts) {
		if len(extra) < 3 {
			continue
		}
		if isRuleSearchStopword(extra) {
			continue
		}
		if _, ok := seen[extra]; ok {
			continue
		}
		seen[extra] = struct{}{}
		patterns = append(patterns, "%"+extra+"%")
	}
	return patterns
}

func expandedSearchTokens(tokens []string) []string {
	expanded := make([]string, 0, len(tokens)*2)
	add := func(value string) {
		value = strings.TrimSpace(strings.ToLower(value))
		if value == "" {
			return
		}
		for _, existing := range expanded {
			if existing == value {
				return
			}
		}
		expanded = append(expanded, value)
	}

	for _, token := range tokens {
		add(token)
		switch {
		case strings.Contains(token, "rass") || strings.Contains(token, "volk") || strings.Contains(token, "species"):
			add("rasse")
			add("volk")
			add("species")
		case strings.Contains(token, "weis"):
			add("weisheit")
			add("wisdom")
		case strings.Contains(token, "dunkel") || strings.Contains(token, "sicht"):
			add("dunkelsicht")
			add("darkvision")
		case strings.Contains(token, "beweg") || strings.Contains(token, "speed") || strings.Contains(token, "fuß") || strings.Contains(token, "feet"):
			add("bewegung")
			add("speed")
		case strings.Contains(token, "zauber") || strings.Contains(token, "spell"):
			add("zauber")
			add("spell")
		case strings.Contains(token, "feat") || strings.Contains(token, "talent"):
			add("feat")
			add("talent")
		case strings.Contains(token, "monster"):
			add("monster")
		}
	}

	return expanded
}

func isRuleSearchStopword(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "a", "an", "and", "as", "at", "be", "by", "for", "from", "has", "have", "in", "is", "it", "its", "of", "on", "or", "the", "to", "with",
		"alle", "auch", "aus", "bitte", "dann", "der", "die", "das", "dem", "den", "des", "ein", "eine", "einer", "eines", "einem", "einen", "gib", "gibt", "hier", "ich", "im", "ist", "ja", "mal", "mir", "mit", "nur", "oder", "sag", "sind", "soll", "sollen", "und", "uns", "vom", "von", "was", "wer", "wie", "wir":
		return true
	default:
		return false
	}
}

func detectMonsterName(query string) string {
	lower := strings.ToLower(strings.TrimSpace(query))
	if lower == "" {
		return ""
	}

	knownMonsters := []string{
		"tarraske",
		"tarrasque",
		"drachen",
		"dragon",
	}

	for _, candidate := range knownMonsters {
		if strings.Contains(lower, candidate) {
			return candidate
		}
	}

	return ""
}

func slugifySearch(value string) string {
	replacer := strings.NewReplacer(
		"ä", "ae",
		"ö", "oe",
		"ü", "ue",
		"ß", "ss",
		"'", "",
		"\"", "",
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
		"-", " ",
		"_", " ",
	)
	normalized := strings.ToLower(replacer.Replace(value))
	return strings.Join(strings.Fields(normalized), " ")
}

func isLikelyRulesQuery(query string) bool {
	lower := strings.ToLower(strings.TrimSpace(query))
	if lower == "" {
		return false
	}
	lower = strings.TrimLeft(lower, " \t\r\n.,!?;:-_\"'`“”„‚‘()[]{}")
	if lower == "" {
		return false
	}

	sceneIntentIndicators := []string{
		"ich ",
		"wir ",
		"mein charakter",
		"mein held",
		"ich versuche",
		"ich greife",
		"ich rede",
		"ich spreche",
		"ich gehe",
		"ich untersuche",
		"ich will",
		"ich möchte",
		"ich moechte",
		"ich würfle",
		"ich wuerfle",
		"wir versuchen",
		"wir schauen",
		"wir sehen",
		"wir öffnen",
		"wir oeffnen",
		"wir untersuchen",
		"wir schleichen",
		"wir reden",
		"wir sprechen",
		"wir greifen",
		"wir gehen",
		"wir wollen",
		"wir möchten",
		"wir moechten",
	}
	for _, prefix := range sceneIntentIndicators {
		if strings.HasPrefix(lower, prefix) || strings.Contains(lower, " "+prefix) {
			return false
		}
	}

	explicitIndicators := []string{
		"regel",
		"regeln",
		"regelwerk",
		"rules",
		"rule",
		"wie funktioniert",
		"wie geht",
		"how does",
		"how do",
		"ist es erlaubt",
		"welcher sg",
		"welche regel",
		"welches talent",
		"welcher zauber",
		"zeige mir",
		"zeig mir",
		"aus dem regelwerk",
		"nach den regeln",
	}
	for _, indicator := range explicitIndicators {
		if strings.Contains(lower, indicator) {
			return true
		}
	}

	rulesQuestionIndicators := []string{
		"welcher sg",
		"welche probe",
		"welchen wurf",
		"was muss ich würfeln",
		"was muss ich wuerfeln",
		"welche regel gilt",
		"wie ist die regel",
		"wie lauten die regeln",
		"nach den regeln",
		"aus dem regelwerk",
	}
	for _, indicator := range rulesQuestionIndicators {
		if strings.Contains(lower, indicator) {
			return true
		}
	}

	return false
}

func (s *Store) CreateSessionEvent(ctx context.Context, sessionID, eventType string, payload map[string]any) (SessionEvent, error) {
	encoded, err := json.Marshal(defaultMetadata(payload))
	if err != nil {
		return SessionEvent{}, err
	}

	var item SessionEvent
	var rawPayload []byte
	err = s.pool.QueryRow(ctx, `
		INSERT INTO session_events (session_id, type, payload_json)
		VALUES ($1, $2, $3::jsonb)
		RETURNING id::text, session_id::text, type, payload_json, created_at
	`, sessionID, eventType, string(encoded)).Scan(&item.ID, &item.SessionID, &item.Type, &rawPayload, &item.CreatedAt)
	if err != nil {
		return SessionEvent{}, err
	}

	item.Payload = map[string]any{}
	if len(rawPayload) > 0 {
		if err := json.Unmarshal(rawPayload, &item.Payload); err != nil {
			return SessionEvent{}, err
		}
	}

	return item, nil
}

func (s *Store) ListSessionEvents(ctx context.Context, sessionID string, limit int) ([]SessionEvent, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id::text, session_id::text, type, payload_json, created_at
		FROM session_events
		WHERE session_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]SessionEvent, 0)
	for rows.Next() {
		var item SessionEvent
		var rawPayload []byte
		if err := rows.Scan(&item.ID, &item.SessionID, &item.Type, &rawPayload, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.Payload = map[string]any{}
		if len(rawPayload) > 0 {
			if err := json.Unmarshal(rawPayload, &item.Payload); err != nil {
				return nil, err
			}
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

func (s *Store) ListDocuments(ctx context.Context) ([]Document, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT d.id::text, d.adventure_id::text, d.type, d.name, d.source_file_path, d.metadata_json, d.created_at, COUNT(dc.id) AS chunk_count
		FROM documents d
		LEFT JOIN document_chunks dc ON dc.document_id = d.id
		GROUP BY d.id, d.type, d.name, d.source_file_path, d.metadata_json, d.created_at
		ORDER BY d.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Document, 0)
	for rows.Next() {
		item, err := scanDocument(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

func (s *Store) CreateDocument(ctx context.Context, req CreateDocumentRequest) (Document, error) {
	payload, err := json.Marshal(defaultMetadata(req.Metadata))
	if err != nil {
		return Document{}, err
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO documents (adventure_id, type, name, source_file_path, metadata_json)
		VALUES ($1, $2, $3, $4, $5::jsonb)
		RETURNING id::text, adventure_id::text, type, name, source_file_path, metadata_json, created_at, 0
	`, req.AdventureID, req.Type, req.Name, req.SourceFilePath, string(payload))

	return scanDocument(row)
}

func (s *Store) GetDocument(ctx context.Context, id string) (Document, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT d.id::text, d.adventure_id::text, d.type, d.name, d.source_file_path, d.metadata_json, d.created_at, COUNT(dc.id) AS chunk_count
		FROM documents d
		LEFT JOIN document_chunks dc ON dc.document_id = d.id
		WHERE d.id = $1
		GROUP BY d.id, d.type, d.name, d.source_file_path, d.metadata_json, d.created_at
	`, id)
	return scanDocument(row)
}

func (s *Store) DeleteDocument(ctx context.Context, id string) error {
	commandTag, err := s.pool.Exec(ctx, `DELETE FROM documents WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) ListAssets(ctx context.Context) ([]Asset, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, adventure_id::text, document_id::text, type, source_type, name, file_path, mime_type, entity_name, location_name, tags_json, metadata_json, created_at
		FROM assets
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Asset, 0)
	for rows.Next() {
		item, err := scanAsset(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

func (s *Store) CreateAsset(ctx context.Context, asset Asset) (Asset, error) {
	encodedTags, err := json.Marshal(asset.Tags)
	if err != nil {
		return Asset{}, err
	}
	encodedMetadata, err := json.Marshal(defaultMetadata(asset.Metadata))
	if err != nil {
		return Asset{}, err
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO assets (
			adventure_id, document_id, type, source_type, name, file_path, mime_type, entity_name, location_name, tags_json, metadata_json
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb, $11::jsonb)
		RETURNING id::text, adventure_id::text, document_id::text, type, source_type, name, file_path, mime_type, entity_name, location_name, tags_json, metadata_json, created_at
	`, asset.AdventureID, asset.DocumentID, asset.Type, asset.SourceType, asset.Name, asset.FilePath, asset.MimeType, asset.EntityName, asset.LocationName, string(encodedTags), string(encodedMetadata))
	return scanAsset(row)
}

func (s *Store) GetAsset(ctx context.Context, id string) (Asset, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id::text, adventure_id::text, document_id::text, type, source_type, name, file_path, mime_type, entity_name, location_name, tags_json, metadata_json, created_at
		FROM assets
		WHERE id = $1
	`, id)
	return scanAsset(row)
}

func (s *Store) DeleteAsset(ctx context.Context, id string) error {
	commandTag, err := s.pool.Exec(ctx, `DELETE FROM assets WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) ListAdventureDocuments(ctx context.Context, adventureID string) ([]Document, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT d.id::text, d.adventure_id::text, d.type, d.name, d.source_file_path, d.metadata_json, d.created_at, COUNT(dc.id) AS chunk_count
		FROM documents d
		LEFT JOIN document_chunks dc ON dc.document_id = d.id
		WHERE d.adventure_id = $1
		GROUP BY d.id, d.type, d.name, d.source_file_path, d.metadata_json, d.created_at
		ORDER BY d.created_at DESC
	`, adventureID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Document, 0)
	for rows.Next() {
		item, err := scanDocument(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListAdventureAssets(ctx context.Context, adventureID string) ([]Asset, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, adventure_id::text, document_id::text, type, source_type, name, file_path, mime_type, entity_name, location_name, tags_json, metadata_json, created_at
		FROM assets
		WHERE adventure_id = $1
		ORDER BY created_at DESC
	`, adventureID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Asset, 0)
	for rows.Next() {
		item, err := scanAsset(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) DeleteAdventure(ctx context.Context, id string) error {
	commandTag, err := s.pool.Exec(ctx, `DELETE FROM adventures WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) Stats(ctx context.Context) (map[string]int64, error) {
	type pair struct {
		key   string
		query string
	}

	queries := []pair{
		{key: "campaigns", query: `SELECT COUNT(*) FROM campaigns`},
		{key: "sessions", query: `SELECT COUNT(*) FROM sessions`},
		{key: "documents", query: `SELECT COUNT(*) FROM documents`},
		{key: "document_chunks", query: `SELECT COUNT(*) FROM document_chunks`},
		{key: "adventures", query: `SELECT COUNT(*) FROM adventures`},
		{key: "assets", query: `SELECT COUNT(*) FROM assets`},
		{key: "characters", query: `SELECT COUNT(*) FROM characters`},
	}

	stats := make(map[string]int64, len(queries))
	for _, item := range queries {
		var count int64
		if err := s.pool.QueryRow(ctx, item.query).Scan(&count); err != nil {
			return nil, err
		}
		stats[item.key] = count
	}

	return stats, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanDocument(row scanner) (Document, error) {
	var item Document
	var rawMetadata []byte
	err := row.Scan(&item.ID, &item.AdventureID, &item.Type, &item.Name, &item.SourceFilePath, &rawMetadata, &item.CreatedAt, &item.ChunkCount)
	if err != nil {
		return Document{}, err
	}

	item.Metadata = defaultMetadata(nil)
	if len(rawMetadata) > 0 {
		if err := json.Unmarshal(rawMetadata, &item.Metadata); err != nil {
			return Document{}, err
		}
	}

	return item, nil
}

func scanAsset(row scanner) (Asset, error) {
	var item Asset
	var rawTags []byte
	var rawMetadata []byte
	err := row.Scan(
		&item.ID,
		&item.AdventureID,
		&item.DocumentID,
		&item.Type,
		&item.SourceType,
		&item.Name,
		&item.FilePath,
		&item.MimeType,
		&item.EntityName,
		&item.LocationName,
		&rawTags,
		&rawMetadata,
		&item.CreatedAt,
	)
	if err != nil {
		return Asset{}, err
	}

	item.Tags = []string{}
	item.Metadata = map[string]any{}
	if len(rawTags) > 0 {
		if err := json.Unmarshal(rawTags, &item.Tags); err != nil {
			return Asset{}, err
		}
	}
	if len(rawMetadata) > 0 {
		if err := json.Unmarshal(rawMetadata, &item.Metadata); err != nil {
			return Asset{}, err
		}
	}

	return item, nil
}

func defaultMetadata(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	return input
}

func defaultAnySlice(input []map[string]any) []map[string]any {
	if input == nil {
		return []map[string]any{}
	}
	return input
}

func (s *Store) scanCharacters(rows pgx.Rows) ([]Character, error) {
	items := make([]Character, 0)
	for rows.Next() {
		var item Character
		var rawAbilities []byte
		var rawLanguages []byte
		var rawFeatures []byte
		var rawMetadata []byte
		if err := rows.Scan(
			&item.ID, &item.CampaignID, &item.DocumentID, &item.Name, &item.PlayerName, &item.ClassAndLevel, &item.Background,
			&item.Race, &item.Alignment, &item.ArmorClass, &item.Speed, &item.HitPointMax, &item.Proficiency,
			&rawAbilities, &rawLanguages, &rawFeatures, &rawMetadata, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		item.Abilities = map[string]int{}
		item.Languages = []string{}
		item.Features = []string{}
		item.Metadata = map[string]any{}
		_ = json.Unmarshal(rawAbilities, &item.Abilities)
		_ = json.Unmarshal(rawLanguages, &item.Languages)
		_ = json.Unmarshal(rawFeatures, &item.Features)
		_ = json.Unmarshal(rawMetadata, &item.Metadata)
		items = append(items, item)
	}
	return items, rows.Err()
}

func defaultStringSlice(input []string) []string {
	if input == nil {
		return []string{}
	}
	return input
}

func scanLLMSession(row scanner) (LLMSession, error) {
	var item LLMSession
	var rawHistory []byte
	var rawSummary []byte
	var rawFacts []byte
	var rawThreads []byte
	var rawState []byte
	if err := row.Scan(
		&item.ID,
		&item.SessionType,
		&item.ScopeType,
		&item.ScopeID,
		&item.RequestProfile,
		&item.RulesetWork,
		&item.RulesetVersion,
		&item.Status,
		&rawHistory,
		&rawSummary,
		&rawFacts,
		&rawThreads,
		&rawState,
		&item.TokenBudget,
		&item.LiveTurnWindow,
		&item.SummaryVersion,
		&item.EstimatedPromptTokens,
		&item.EstimatedSummaryTokens,
		&item.LastSummarizedAt,
		&item.ArchivedAt,
		&item.LastActiveAt,
		&item.CreatedAt,
	); err != nil {
		return LLMSession{}, err
	}
	item.MessageHistory = []map[string]any{}
	item.WorkingSummary = map[string]any{}
	item.Facts = []string{}
	item.OpenThreads = []string{}
	item.StructuredState = map[string]any{}
	_ = json.Unmarshal(rawHistory, &item.MessageHistory)
	_ = json.Unmarshal(rawSummary, &item.WorkingSummary)
	_ = json.Unmarshal(rawFacts, &item.Facts)
	_ = json.Unmarshal(rawThreads, &item.OpenThreads)
	_ = json.Unmarshal(rawState, &item.StructuredState)
	return item, nil
}

func defaultIntMap(input map[string]int) map[string]int {
	if input == nil {
		return map[string]int{}
	}
	return input
}

func generatePlayerToken() (string, error) {
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func generateSessionToken() (string, error) {
	bytes := make([]byte, 10)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func defaultSessionState() SessionState {
	return SessionState{
		SceneSummary:        "",
		ActiveNPCs:          []string{},
		OpenQuests:          []string{},
		LastDMNotes:         []string{},
		ActiveMediaCue:      "",
		VisualMode:          "pause_or_recap",
		VisualPayload:       map[string]any{},
		AudioMode:           "silence",
		AudioPayload:        map[string]any{},
		VoiceMode:           "narrator",
		TTSStatus:           "idle",
		SelectedRulebookIDs: []string{},
		PromptConfig: SessionPromptConfig{
			GMStyle:           "immersive",
			IntroStyle:        "cinematic",
			AdventureFocus:    "strict_adventure_first",
			RulesStrictness:   "table_balanced",
			PlayerAgencyStyle: "proactive_questions",
			PromptOverride:    "",
		},
		GroupInventory: SessionGroupInventory{
			Gold:  0,
			Items: []string{},
			Notes: "",
		},
		Combat: CombatState{
			Active:          false,
			Round:           0,
			ActiveTurnIndex: 0,
			InitiativeOrder: []CombatTurnEntry{},
			Log:             []CombatLogEntry{},
		},
		AwaitingLevelUpRest: false,
		LevelUpQueue:        []LevelUpQueueEntry{},
		LastRewardSummary:   "",
	}
}
