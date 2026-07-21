package httpapi

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (s *Store) ListSessionCompanions(ctx context.Context, sessionID string) ([]SessionCompanion, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, session_id::text, character_id::text, display_name, control_mode, status, tactics_note, visibility,
		       current_hit_points, temporary_hit_points, conditions_json, resource_overrides_json, created_at, updated_at
		FROM session_companions
		WHERE session_id = $1
		ORDER BY created_at ASC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSessionCompanions(rows)
}

func (s *Store) GetSessionCompanion(ctx context.Context, id string) (SessionCompanion, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, session_id::text, character_id::text, display_name, control_mode, status, tactics_note, visibility,
		       current_hit_points, temporary_hit_points, conditions_json, resource_overrides_json, created_at, updated_at
		FROM session_companions
		WHERE id = $1
	`, id)
	if err != nil {
		return SessionCompanion{}, err
	}
	defer rows.Close()
	items, err := scanSessionCompanions(rows)
	if err != nil {
		return SessionCompanion{}, err
	}
	if len(items) == 0 {
		return SessionCompanion{}, pgx.ErrNoRows
	}
	return items[0], nil
}

func (s *Store) CreateSessionCompanion(ctx context.Context, sessionID string, characterID string, req CreateSessionCompanionRequest) (SessionCompanion, error) {
	conditions, err := json.Marshal([]string{})
	if err != nil {
		return SessionCompanion{}, err
	}
	resources, err := json.Marshal(map[string]any{})
	if err != nil {
		return SessionCompanion{}, err
	}
	displayName := strings.TrimSpace(req.DisplayName)
	visibility := strings.TrimSpace(req.Visibility)
	if visibility == "" {
		visibility = "player_visible"
	}
	rows, err := s.pool.Query(ctx, `
		INSERT INTO session_companions (
			session_id, character_id, display_name, control_mode, status, tactics_note, visibility,
			current_hit_points, temporary_hit_points, conditions_json, resource_overrides_json
		)
		VALUES ($1, $2, $3, 'dm', 'active', $4, $5, NULL, NULL, $6::jsonb, $7::jsonb)
		RETURNING id::text, session_id::text, character_id::text, display_name, control_mode, status, tactics_note, visibility,
		          current_hit_points, temporary_hit_points, conditions_json, resource_overrides_json, created_at, updated_at
	`, sessionID, characterID, displayName, strings.TrimSpace(req.TacticsNote), visibility, string(conditions), string(resources))
	if err != nil {
		return SessionCompanion{}, err
	}
	defer rows.Close()
	items, err := scanSessionCompanions(rows)
	if err != nil {
		return SessionCompanion{}, err
	}
	if len(items) == 0 {
		return SessionCompanion{}, pgx.ErrNoRows
	}
	return items[0], nil
}

func (s *Store) UpdateSessionCompanion(ctx context.Context, item SessionCompanion) (SessionCompanion, error) {
	conditions, err := json.Marshal(defaultStringSlice(item.Conditions))
	if err != nil {
		return SessionCompanion{}, err
	}
	resources, err := json.Marshal(defaultMetadata(item.ResourceOverrides))
	if err != nil {
		return SessionCompanion{}, err
	}
	rows, err := s.pool.Query(ctx, `
		UPDATE session_companions
		SET display_name = $2,
		    control_mode = $3,
		    status = $4,
		    tactics_note = $5,
		    visibility = $6,
		    current_hit_points = $7,
		    temporary_hit_points = $8,
		    conditions_json = $9::jsonb,
		    resource_overrides_json = $10::jsonb,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id::text, session_id::text, character_id::text, display_name, control_mode, status, tactics_note, visibility,
		          current_hit_points, temporary_hit_points, conditions_json, resource_overrides_json, created_at, updated_at
	`, item.ID, item.DisplayName, item.ControlMode, item.Status, item.TacticsNote, item.Visibility, item.CurrentHitPoints, item.TemporaryHitPoints, string(conditions), string(resources))
	if err != nil {
		return SessionCompanion{}, err
	}
	defer rows.Close()
	items, err := scanSessionCompanions(rows)
	if err != nil {
		return SessionCompanion{}, err
	}
	if len(items) == 0 {
		return SessionCompanion{}, pgx.ErrNoRows
	}
	return items[0], nil
}

func (s *Store) DeleteSessionCompanion(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM session_companions WHERE id = $1`, id)
	return err
}

func scanSessionCompanions(rows pgx.Rows) ([]SessionCompanion, error) {
	items := make([]SessionCompanion, 0)
	for rows.Next() {
		var item SessionCompanion
		var rawConditions []byte
		var rawResources []byte
		if err := rows.Scan(
			&item.ID,
			&item.SessionID,
			&item.CharacterID,
			&item.DisplayName,
			&item.ControlMode,
			&item.Status,
			&item.TacticsNote,
			&item.Visibility,
			&item.CurrentHitPoints,
			&item.TemporaryHitPoints,
			&rawConditions,
			&rawResources,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		item.Conditions = []string{}
		item.ResourceOverrides = map[string]any{}
		_ = json.Unmarshal(rawConditions, &item.Conditions)
		_ = json.Unmarshal(rawResources, &item.ResourceOverrides)
		items = append(items, item)
	}
	return items, rows.Err()
}
