package httpapi

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func safeOptionalString(value any) string {
	if value == nil {
		return ""
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" || text == "null" {
		return ""
	}
	return text
}

func messageHistoryToStrings(history []map[string]any) []map[string]string {
	items := make([]map[string]string, 0, len(history))
	for _, entry := range history {
		role := strings.TrimSpace(fmt.Sprint(entry["role"]))
		content := strings.TrimSpace(fmt.Sprint(entry["content"]))
		if role == "" || content == "" {
			continue
		}
		items = append(items, map[string]string{
			"role":    role,
			"content": content,
		})
	}
	return items
}

func appendLLMMessage(history []map[string]any, role string, content string, createdAt time.Time) []map[string]any {
	next := make([]map[string]any, 0, len(history)+1)
	next = append(next, defaultAnySlice(history)...)
	next = append(next, map[string]any{
		"role":       strings.TrimSpace(role),
		"content":    strings.TrimSpace(content),
		"created_at": createdAt.UTC().Format(time.RFC3339),
	})
	return next
}

func recentHistory(history []map[string]any, limit int) []map[string]any {
	if limit > 8 {
		limit = 8
	}
	if limit <= 0 || len(history) <= limit {
		return defaultAnySlice(history)
	}
	return defaultAnySlice(history[len(history)-limit:])
}

func compactLLMHistory(history []map[string]any, liveWindow int) ([]map[string]any, []string) {
	if liveWindow <= 0 {
		liveWindow = 8
	}
	if liveWindow > 8 {
		liveWindow = 8
	}
	if len(history) <= liveWindow {
		return defaultAnySlice(history), []string{}
	}
	older := history[:len(history)-liveWindow]
	facts := make([]string, 0, 4)
	for _, entry := range older {
		role := strings.TrimSpace(fmt.Sprint(entry["role"]))
		content := truncatePromptContextText(strings.TrimSpace(fmt.Sprint(entry["content"])), 180)
		if role == "" || content == "" {
			continue
		}
		facts = append(facts, fmt.Sprintf("%s: %s", role, content))
		if len(facts) >= 4 {
			break
		}
	}
	return defaultAnySlice(history[len(history)-liveWindow:]), facts
}

func (h *Handler) ensureScopedLLMSession(ctx context.Context, scopeType string, scopeID string, sessionType string, requestProfile string, rulesetWork string, rulesetVersion string, existingID string, seedHistory []map[string]any, tokenBudget int, initialSummary map[string]any, initialState map[string]any) (LLMSession, error) {
	existingID = safeOptionalString(existingID)
	if existingID != "" {
		item, err := h.store.GetLLMSession(ctx, existingID)
		if err == nil {
			if item.Status == "archived" {
				item.Status = "active"
				item.ArchivedAt = nil
				item.LastActiveAt = time.Now().UTC()
				return h.store.UpdateLLMSession(ctx, item)
			}
			return item, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return LLMSession{}, err
		}
	}

	item, err := h.store.GetLatestLLMSessionByScope(ctx, scopeType, scopeID, sessionType)
	if err == nil {
		if item.Status == "archived" {
			item.Status = "active"
			item.ArchivedAt = nil
			item.LastActiveAt = time.Now().UTC()
			return h.store.UpdateLLMSession(ctx, item)
		}
		return item, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return LLMSession{}, err
	}

	now := time.Now().UTC()
	return h.store.CreateLLMSession(ctx, LLMSession{
		SessionType:     sessionType,
		ScopeType:       scopeType,
		ScopeID:         scopeID,
		RequestProfile:  requestProfile,
		RulesetWork:     strings.TrimSpace(rulesetWork),
		RulesetVersion:  strings.TrimSpace(rulesetVersion),
		Status:          "active",
		MessageHistory:  defaultAnySlice(seedHistory),
		WorkingSummary:  defaultMetadata(initialSummary),
		Facts:           []string{},
		OpenThreads:     []string{},
		StructuredState: defaultMetadata(initialState),
		TokenBudget:     tokenBudget,
		LiveTurnWindow:  8,
		SummaryVersion:  1,
		LastActiveAt:    now,
	})
}
