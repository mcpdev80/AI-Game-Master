package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

func (h *Handler) listCampaigns(c *gin.Context) {
	items, err := h.store.ListCampaigns(c.Request.Context())
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "list campaigns", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) listAdventures(c *gin.Context) {
	items, err := h.store.ListAdventures(c.Request.Context())
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "list adventures", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) createCampaign(c *gin.Context) {
	var req CreateCampaignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid campaign payload", err)
		return
	}

	item, err := h.store.CreateCampaign(c.Request.Context(), req)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "create campaign", err)
		return
	}

	c.JSON(http.StatusCreated, item)
}

func (h *Handler) createAdventure(c *gin.Context) {
	var req CreateAdventureRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid adventure payload", err)
		return
	}

	item, err := h.store.CreateAdventure(c.Request.Context(), req)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "create adventure", err)
		return
	}

	c.JSON(http.StatusCreated, item)
}

func (h *Handler) deleteAdventure(c *gin.Context) {
	adventureID := c.Param("id")
	documents, err := h.store.ListAdventureDocuments(c.Request.Context(), adventureID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "list adventure documents", err)
		return
	}
	assets, err := h.store.ListAdventureAssets(c.Request.Context(), adventureID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "list adventure assets", err)
		return
	}
	for _, document := range documents {
		if err := h.store.DeleteDocument(c.Request.Context(), document.ID); err != nil {
			errorResponse(c, http.StatusInternalServerError, "delete adventure document", err)
			return
		}
		removeLocalFile(document.SourceFilePath)
	}
	for _, asset := range assets {
		if err := h.store.DeleteAsset(c.Request.Context(), asset.ID); err != nil {
			errorResponse(c, http.StatusInternalServerError, "delete adventure asset", err)
			return
		}
		removeLocalFile(&asset.FilePath)
	}
	if err := h.store.DeleteAdventure(c.Request.Context(), adventureID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "adventure not found"})
			return
		}
		errorResponse(c, http.StatusInternalServerError, "delete adventure", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true, "id": adventureID})
}

func sessionAdventure(ctx context.Context, store *Store, session Session) (*Adventure, error) {
	if session.AdventureID == nil || strings.TrimSpace(*session.AdventureID) == "" {
		return nil, nil
	}
	adventures, err := store.ListAdventures(ctx)
	if err != nil {
		return nil, err
	}
	for _, adventure := range adventures {
		if adventure.ID == *session.AdventureID {
			return &adventure, nil
		}
	}
	return nil, nil
}

func adventureName(adventure *Adventure) string {
	if adventure == nil {
		return ""
	}
	return strings.TrimSpace(adventure.Name)
}

func sessionAdventureDocuments(ctx context.Context, store *Store, session Session) ([]Document, error) {
	if session.AdventureID == nil || strings.TrimSpace(*session.AdventureID) == "" {
		return []Document{}, nil
	}
	return store.ListAdventureDocuments(ctx, *session.AdventureID)
}

func shortRulesDocumentsForRuleset(ctx context.Context, store *Store, work string, version string) []Document {
	hidden, err := store.ListHiddenSystemDocumentIDs(ctx)
	if err != nil {
		return []Document{}
	}
	items := make([]Document, 0)
	for _, document := range embeddedGuidesForRuleset(work, version) {
		if _, ok := hidden[document.ID]; ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(fmt.Sprintf("%v", document.Metadata["kind"])), "short_rules_guide") {
			items = append(items, document)
		}
	}
	return items
}

func appendMissingDocuments(base []Document, extra []Document) []Document {
	if len(extra) == 0 {
		return base
	}
	seen := make(map[string]struct{}, len(base))
	items := append([]Document(nil), base...)
	for _, document := range base {
		seen[document.ID] = struct{}{}
	}
	for _, document := range extra {
		if _, ok := seen[document.ID]; ok {
			continue
		}
		items = append(items, document)
	}
	return items
}

func sessionRulebookDocuments(ctx context.Context, store *Store, session Session, campaignDocuments []Document) ([]Document, error) {
	if len(session.State.SelectedRulebookIDs) > 0 {
		items, err := store.ListDocumentsByIDs(ctx, session.State.SelectedRulebookIDs)
		if err != nil {
			return nil, err
		}
		return appendMissingDocuments(shortRulesDocumentsForRuleset(ctx, store, session.RulesetWork, session.RulesetVersion), items), nil
	}
	items := make([]Document, 0)
	for _, document := range campaignDocuments {
		if document.Type != "rules" {
			continue
		}
		work := strings.TrimSpace(fmt.Sprintf("%v", document.Metadata["ruleset_work"]))
		version := strings.TrimSpace(fmt.Sprintf("%v", document.Metadata["ruleset_version"]))
		if work == session.RulesetWork && version == session.RulesetVersion {
			items = append(items, document)
			continue
		}
		key := fmt.Sprintf("%s:%s", session.RulesetWork, session.RulesetVersion)
		for _, candidate := range stringListFromAny(document.Metadata["ruleset_keys"]) {
			if candidate == key {
				items = append(items, document)
				break
			}
		}
	}
	return appendMissingDocuments(shortRulesDocumentsForRuleset(ctx, store, session.RulesetWork, session.RulesetVersion), items), nil
}
