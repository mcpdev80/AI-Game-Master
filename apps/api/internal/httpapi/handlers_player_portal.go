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

func (h *Handler) listPlayerLinks(c *gin.Context) {
	items, err := h.store.ListPlayerLinkSlots(c.Request.Context(), c.Param("id"))
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "list player links", err)
		return
	}
	for i := range items {
		if items[i].Link != nil && items[i].Link.RevokedAt == nil {
			joinURL := fmt.Sprintf("http://localhost:3005/join/%s", items[i].Link.Token)
			items[i].JoinURL = &joinURL
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) createPlayerLink(c *gin.Context) {
	var req CreatePlayerLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid player link payload", err)
		return
	}

	slot, link, err := h.store.CreatePlayerLink(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "create player link", err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"player_slot": slot,
		"link":        link,
		"join_url":    fmt.Sprintf("http://localhost:3005/join/%s", link.Token),
	})
}

func (h *Handler) regeneratePlayerLink(c *gin.Context) {
	link, err := h.store.RegeneratePlayerLink(c.Request.Context(), c.Param("playerSlotId"))
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "regenerate player link", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"link":     link,
		"join_url": fmt.Sprintf("http://localhost:3005/join/%s", link.Token),
	})
}

func (h *Handler) setPlayerLinkState(c *gin.Context) {
	var req struct {
		Revoked bool `json:"revoked"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid player link state payload", err)
		return
	}

	link, err := h.store.SetPlayerLinkRevoked(c.Request.Context(), c.Param("playerSlotId"), req.Revoked)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "player link not found"})
			return
		}
		errorResponse(c, http.StatusInternalServerError, "update player link state", err)
		return
	}

	response := gin.H{"link": link}
	if link.RevokedAt == nil {
		response["join_url"] = fmt.Sprintf("http://localhost:3005/join/%s", link.Token)
	}
	c.JSON(http.StatusOK, response)
}

func (h *Handler) playerPortalJoin(c *gin.Context) {
	token := strings.TrimSpace(c.Param("token"))
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing token"})
		return
	}
	if err := h.store.MarkPlayerLinkJoined(c.Request.Context(), token); err != nil {
		errorResponse(c, http.StatusInternalServerError, "mark player joined", err)
		return
	}
	portal, err := h.loadPlayerPortalSession(c.Request.Context(), token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "player link not found"})
			return
		}
		errorResponse(c, http.StatusInternalServerError, "load player portal", err)
		return
	}
	c.JSON(http.StatusOK, portal)
}

func (h *Handler) playerPortalMe(c *gin.Context) {
	token := strings.TrimSpace(c.Query("token"))
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing token query parameter"})
		return
	}
	portal, err := h.loadPlayerPortalSession(c.Request.Context(), token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "player portal session not found"})
			return
		}
		errorResponse(c, http.StatusInternalServerError, "load player portal", err)
		return
	}
	c.JSON(http.StatusOK, portal)
}

func (h *Handler) joinSession(c *gin.Context) {
	sessionToken := strings.TrimSpace(c.Param("token"))
	if sessionToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing session token"})
		return
	}

	var req JoinSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid join payload", err)
		return
	}

	sessions, err := h.store.ListSessions(c.Request.Context())
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "list sessions", err)
		return
	}

	var session *Session
	for _, item := range sessions {
		if item.JoinToken == sessionToken {
			copy := item
			session = &copy
			break
		}
	}
	if session == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session join link not found"})
		return
	}

	var link PlayerAccessLink
	var slot PlayerSlot
	if req.PlayerSlotID != nil && strings.TrimSpace(*req.PlayerSlotID) != "" {
		slots, err := h.store.ListPlayerSlots(c.Request.Context(), session.ID)
		if err != nil {
			errorResponse(c, http.StatusInternalServerError, "list player slots", err)
			return
		}
		found := false
		for _, item := range slots {
			if item.ID == strings.TrimSpace(*req.PlayerSlotID) {
				slot = item
				found = true
				break
			}
		}
		if !found {
			c.JSON(http.StatusBadRequest, gin.H{"error": "player slot not found in session"})
			return
		}
		link, err = h.store.GetLatestPlayerLink(c.Request.Context(), slot.ID)
		if err != nil {
			errorResponse(c, http.StatusInternalServerError, "load latest player link", err)
			return
		}
		if link.RevokedAt != nil {
			link, err = h.store.RegeneratePlayerLink(c.Request.Context(), slot.ID)
			if err != nil {
				errorResponse(c, http.StatusInternalServerError, "regenerate player link", err)
				return
			}
		}
	} else {
		displayName := strings.TrimSpace(req.DisplayName)
		if displayName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "display_name is required for a new join"})
			return
		}
		slot, err = h.store.FindPlayerSlotBySessionAndDisplayName(c.Request.Context(), session.ID, displayName)
		if err != nil {
			if !errors.Is(err, pgx.ErrNoRows) {
				errorResponse(c, http.StatusInternalServerError, "find session player slot", err)
				return
			}
			slot, link, err = h.store.CreatePlayerLink(c.Request.Context(), session.ID, CreatePlayerLinkRequest{
				DisplayName: displayName,
			})
			if err != nil {
				errorResponse(c, http.StatusInternalServerError, "create session player slot", err)
				return
			}
		} else {
			link, err = h.store.GetLatestPlayerLink(c.Request.Context(), slot.ID)
			if err != nil {
				errorResponse(c, http.StatusInternalServerError, "load latest player link", err)
				return
			}
			if link.RevokedAt != nil {
				link, err = h.store.RegeneratePlayerLink(c.Request.Context(), slot.ID)
				if err != nil {
					errorResponse(c, http.StatusInternalServerError, "regenerate player link", err)
					return
				}
			}
		}
	}
	if err := h.store.MarkPlayerLinkJoined(c.Request.Context(), link.Token); err != nil {
		errorResponse(c, http.StatusInternalServerError, "mark joined player slot", err)
		return
	}

	portal, err := h.loadPlayerPortalSession(c.Request.Context(), link.Token)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "load player portal", err)
		return
	}

	c.JSON(http.StatusCreated, JoinSessionResponse{
		SessionToken: sessionToken,
		PortalToken:  link.Token,
		JoinURL:      fmt.Sprintf("/player-portal/%s", link.Token),
		Portal:       portal,
	})
}

func (h *Handler) getJoinSessionPreview(c *gin.Context) {
	sessionToken := strings.TrimSpace(c.Param("token"))
	if sessionToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing session token"})
		return
	}

	sessions, err := h.store.ListSessions(c.Request.Context())
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "list sessions", err)
		return
	}

	var session *Session
	for _, item := range sessions {
		if item.JoinToken == sessionToken {
			copy := item
			session = &copy
			break
		}
	}
	if session == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session join link not found"})
		return
	}

	slots, err := h.store.ListPlayerSlots(c.Request.Context(), session.ID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "list player slots", err)
		return
	}

	existingPlayers := make([]SessionJoinCandidate, 0)
	for _, slot := range slots {
		if slot.CharacterID == nil {
			continue
		}
		character, err := h.store.GetCharacter(c.Request.Context(), *slot.CharacterID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			errorResponse(c, http.StatusInternalServerError, "load slot character", err)
			return
		}
		existingPlayers = append(existingPlayers, SessionJoinCandidate{
			PlayerSlot: slot,
			Character:  &character,
		})
	}

	c.JSON(http.StatusOK, SessionJoinPreview{
		SessionID:       session.ID,
		SessionName:     session.Name,
		SessionStatus:   session.Status,
		HasProgress:     hasMeaningfulSessionProgress(*session),
		ExistingPlayers: existingPlayers,
	})
}

func (h *Handler) updatePlayerSlotCharacter(c *gin.Context) {
	var req UpdatePlayerSlotCharacterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid player slot character payload", err)
		return
	}
	slot, err := h.store.UpdatePlayerSlotCharacter(c.Request.Context(), c.Param("playerSlotId"), req.CharacterID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "update player slot character", err)
		return
	}
	c.JSON(http.StatusOK, slot)
}

func (h *Handler) updatePlayerSlotStatus(c *gin.Context) {
	var req UpdatePlayerSlotStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid player slot status payload", err)
		return
	}
	slot, err := h.store.UpdatePlayerSlotStatus(c.Request.Context(), c.Param("playerSlotId"), req.Status)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "update player slot status", err)
		return
	}
	c.JSON(http.StatusOK, slot)
}

func (h *Handler) updatePlayerVisibleState(c *gin.Context) {
	var req UpdatePlayerVisibleStateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid visible state payload", err)
		return
	}

	handouts, err := h.store.ListDocumentsByIDs(c.Request.Context(), req.HandoutDocumentIDs)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "load handout documents", err)
		return
	}

	media, err := h.store.ListAssetsByIDs(c.Request.Context(), req.MediaAssetIDs)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "load media assets", err)
		return
	}

	state, err := h.store.UpdatePlayerVisibleState(c.Request.Context(), c.Param("playerSlotId"), handouts, media)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "update player visible state", err)
		return
	}

	c.JSON(http.StatusOK, state)
}

func (h *Handler) loadPlayerPortalSession(ctx context.Context, token string) (PlayerPortalSession, error) {
	portal, err := h.store.GetPlayerPortalSession(ctx, token)
	if err != nil {
		return PlayerPortalSession{}, err
	}

	characters, err := h.store.ListCharacters(ctx)
	if err != nil {
		return PlayerPortalSession{}, err
	}
	slots, err := h.store.ListPlayerSlots(ctx, portal.Session.ID)
	if err != nil {
		return PlayerPortalSession{}, err
	}
	claimed := make(map[string]bool)
	for _, slot := range slots {
		if slot.CharacterID != nil && slot.ID != portal.PlayerSlot.ID {
			claimed[*slot.CharacterID] = true
		}
	}
	available := make([]Character, 0)
	for _, character := range characters {
		if character.CampaignID != nil && *character.CampaignID != portal.Session.CampaignID {
			continue
		}
		if claimed[character.ID] {
			continue
		}
		available = append(available, character)
	}
	portal.AvailableCharacters = available
	return portal, nil
}
