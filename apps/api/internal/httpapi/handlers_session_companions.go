package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (h *Handler) attachSessionCompanions(ctx context.Context, session *Session) error {
	if session == nil || strings.TrimSpace(session.ID) == "" {
		return nil
	}
	companions, err := h.store.ListSessionCompanions(ctx, session.ID)
	if err != nil {
		return err
	}
	for index := range companions {
		character, err := h.store.GetCharacter(ctx, companions[index].CharacterID)
		if err == nil {
			companions[index].Character = &character
			if strings.TrimSpace(companions[index].DisplayName) == "" {
				companions[index].DisplayName = character.Name
			}
			if companions[index].CurrentHitPoints == nil && character.HitPointMax != nil {
				value := *character.HitPointMax
				companions[index].CurrentHitPoints = &value
			}
		}
	}
	session.Companions = companions
	return nil
}

func (h *Handler) listSessionCompanions(c *gin.Context) {
	session, err := h.store.GetSession(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
			return
		}
		errorResponse(c, http.StatusInternalServerError, "load session", err)
		return
	}
	if err := h.attachSessionCompanions(c.Request.Context(), &session); err != nil {
		errorResponse(c, http.StatusInternalServerError, "load session companions", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": session.Companions})
}

func (h *Handler) createSessionCompanion(c *gin.Context) {
	var req CreateSessionCompanionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid companion payload", err)
		return
	}
	session, err := h.store.GetSession(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
			return
		}
		errorResponse(c, http.StatusInternalServerError, "load session", err)
		return
	}
	character, err := h.store.GetCharacter(c.Request.Context(), req.CharacterID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "character not found"})
			return
		}
		errorResponse(c, http.StatusInternalServerError, "load character", err)
		return
	}
	if character.CampaignID != nil && session.CampaignID != "" && *character.CampaignID != session.CampaignID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "character belongs to a different campaign"})
		return
	}
	slots, err := h.store.ListPlayerSlots(c.Request.Context(), session.ID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "load player slots", err)
		return
	}
	for _, slot := range slots {
		if slot.CharacterID != nil && *slot.CharacterID == req.CharacterID {
			c.JSON(http.StatusConflict, gin.H{"error": "character is already assigned to a player slot in this session"})
			return
		}
	}
	companion, err := h.store.CreateSessionCompanion(c.Request.Context(), session.ID, req.CharacterID, req)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			c.JSON(http.StatusConflict, gin.H{"error": "character is already a companion in this session"})
			return
		}
		errorResponse(c, http.StatusInternalServerError, "create session companion", err)
		return
	}
	companion.Character = &character
	if strings.TrimSpace(companion.DisplayName) == "" {
		companion.DisplayName = character.Name
	}
	if companion.CurrentHitPoints == nil && character.HitPointMax != nil {
		value := *character.HitPointMax
		companion.CurrentHitPoints = &value
	}
	c.JSON(http.StatusCreated, companion)
}

func (h *Handler) updateSessionCompanion(c *gin.Context) {
	var req UpdateSessionCompanionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid companion update payload", err)
		return
	}
	item, err := h.store.GetSessionCompanion(c.Request.Context(), c.Param("companionId"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "companion not found"})
			return
		}
		errorResponse(c, http.StatusInternalServerError, "load companion", err)
		return
	}
	if req.DisplayName != nil {
		item.DisplayName = strings.TrimSpace(*req.DisplayName)
	}
	if req.Status != nil && strings.TrimSpace(*req.Status) != "" {
		item.Status = strings.TrimSpace(*req.Status)
	}
	if req.TacticsNote != nil {
		item.TacticsNote = strings.TrimSpace(*req.TacticsNote)
	}
	if req.Visibility != nil && strings.TrimSpace(*req.Visibility) != "" {
		item.Visibility = strings.TrimSpace(*req.Visibility)
	}
	if req.CurrentHitPoints != nil {
		item.CurrentHitPoints = req.CurrentHitPoints
	}
	if req.TemporaryHitPoints != nil {
		item.TemporaryHitPoints = req.TemporaryHitPoints
	}
	if req.Conditions != nil {
		item.Conditions = req.Conditions
	}
	if req.ResourceOverrides != nil {
		item.ResourceOverrides = req.ResourceOverrides
	}
	updated, err := h.store.UpdateSessionCompanion(c.Request.Context(), item)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "update companion", err)
		return
	}
	if character, err := h.store.GetCharacter(c.Request.Context(), updated.CharacterID); err == nil {
		updated.Character = &character
	}
	c.JSON(http.StatusOK, updated)
}

func (h *Handler) deleteSessionCompanion(c *gin.Context) {
	if err := h.store.DeleteSessionCompanion(c.Request.Context(), c.Param("companionId")); err != nil {
		errorResponse(c, http.StatusInternalServerError, "delete companion", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}
