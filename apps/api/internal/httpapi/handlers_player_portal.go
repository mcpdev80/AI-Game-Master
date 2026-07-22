package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

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

func (h *Handler) updatePlayerPortalCharacter(c *gin.Context) {
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
	if portal.Character == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no character assigned to this player slot"})
		return
	}

	var req UpdatePlayerPortalCharacterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid player portal character payload", err)
		return
	}

	updatedCharacter, err := applyPlayerPortalCharacterUpdate(*portal.Character, req)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "apply player portal character update", err)
		return
	}
	saved, err := h.store.UpdateCharacter(c.Request.Context(), updatedCharacter)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "update player portal character", err)
		return
	}
	c.JSON(http.StatusOK, saved)
}

func (h *Handler) updatePlayerPortalGroupInventory(c *gin.Context) {
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

	var req UpdatePlayerPortalGroupInventoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid player portal group inventory payload", err)
		return
	}

	nextInventory, err := applyPlayerPortalGroupInventoryUpdate(portal.Session.State.GroupInventory, req)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "apply player portal group inventory update", err)
		return
	}

	nextState := portal.Session.State
	nextState.GroupInventory = nextInventory
	if err := h.store.UpdateSessionState(c.Request.Context(), portal.Session.ID, nextState); err != nil {
		errorResponse(c, http.StatusInternalServerError, "update player portal group inventory", err)
		return
	}
	c.JSON(http.StatusOK, nextInventory)
}

func (h *Handler) listPlayerPortalPrivateChat(c *gin.Context) {
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
	items, err := h.store.ListPrivateChatMessages(c.Request.Context(), portal.Session.ID, portal.PlayerSlot.ID, 80)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "list player portal private chat", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) sendPlayerPortalPrivateChat(c *gin.Context) {
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
	var req PlayerPortalPrivateChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid private chat payload", err)
		return
	}
	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "message is required"})
		return
	}

	history, err := h.store.ListPrivateChatMessages(c.Request.Context(), portal.Session.ID, portal.PlayerSlot.ID, 40)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "load private chat history", err)
		return
	}
	contextChunks, err := h.privateSidebarContext(c.Request.Context(), portal, req.Message)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "load private sidebar context", err)
		return
	}

	userMessageEvent, err := h.store.CreateSessionEvent(c.Request.Context(), portal.Session.ID, "private_sidebar_message", privateSidebarEventPayload(portal, "user", req.Message, chooseLanguage(req.Language, portal.Session.Language), nil))
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "persist private user message", err)
		return
	}
	userMessage := privateChatMessageFromEvent(userMessageEvent)

	llmCtx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
	defer cancel()
	response, err := h.llmClient.CompletePrivateSidebarResponse(llmCtx, portal.Session, portal.PlayerSlot, portal.Character, req.Language, req.Message, append(history, userMessage), contextChunks)
	if err != nil {
		errorResponse(c, http.StatusBadGateway, "complete private sidebar response", err)
		return
	}
	replyEvent, err := h.store.CreateSessionEvent(c.Request.Context(), portal.Session.ID, "private_sidebar_message", privateSidebarEventPayload(portal, "assistant", response.Reply, response.Language, response.DMNotes))
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "persist private assistant message", err)
		return
	}
	replyMessage := privateChatMessageFromEvent(replyEvent)
	allMessages, err := h.store.ListPrivateChatMessages(c.Request.Context(), portal.Session.ID, portal.PlayerSlot.ID, 80)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "reload private chat history", err)
		return
	}
	c.JSON(http.StatusOK, PlayerPortalPrivateChatResponse{
		Message:  userMessage,
		Reply:    replyMessage,
		Messages: allMessages,
	})
}

func (h *Handler) loadPlayerPortalSession(ctx context.Context, token string) (PlayerPortalSession, error) {
	portal, err := h.store.GetPlayerPortalSession(ctx, token)
	if err != nil {
		return PlayerPortalSession{}, err
	}
	if err := h.attachSessionCompanions(ctx, &portal.Session); err != nil {
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

	companionCharacterIDs := make(map[string]bool, len(portal.Session.Companions))
	for _, companion := range portal.Session.Companions {
		if strings.TrimSpace(companion.CharacterID) != "" {
			companionCharacterIDs[companion.CharacterID] = true
		}
	}

	available := make([]Character, 0)
	for _, character := range characters {
		if claimed[character.ID] {
			continue
		}
		if companionCharacterIDs[character.ID] {
			continue
		}
		if isSessionClone, ok := character.Metadata["is_session_clone"].(bool); ok && isSessionClone {
			continue
		}
		if rawClonedForSessionID, ok := character.Metadata["cloned_for_session_id"]; ok {
			clonedForSessionID := strings.TrimSpace(fmt.Sprintf("%v", rawClonedForSessionID))
			if clonedForSessionID != "" && clonedForSessionID != "<nil>" {
				continue
			}
		}
		available = append(available, character)
	}
	portal.AvailableCharacters = available
	portal.Session = sanitizeSessionForPlayers(portal.Session)
	return portal, nil
}

func sanitizeSessionForPlayers(session Session) Session {
	session.State.LastDMNotes = nil
	session.State.PlayLLMSessionID = ""
	session.State.RulesLLMSessionID = ""
	session.State.SummarySessionID = ""
	if len(session.State.VisualPayload) > 0 {
		payload := make(map[string]any, len(session.State.VisualPayload))
		for key, value := range session.State.VisualPayload {
			payload[key] = value
		}
		if hidden, _ := payload["hide_dc"].(bool); hidden {
			delete(payload, "roll_dc")
		}
		session.State.VisualPayload = payload
	}
	return session
}

func privateSidebarEventPayload(portal PlayerPortalSession, role string, content string, language string, dmNotes []string) map[string]any {
	payload := map[string]any{
		"scope":          "private_player",
		"visibility":     "private",
		"player_slot_id": portal.PlayerSlot.ID,
		"display_name":   portal.PlayerSlot.DisplayName,
		"role":           role,
		"content":        strings.TrimSpace(content),
		"language":       chooseLanguage(language, portal.Session.Language),
	}
	if portal.PlayerSlot.CharacterID != nil {
		payload["character_id"] = *portal.PlayerSlot.CharacterID
	}
	if portal.Character != nil {
		payload["character_name"] = portal.Character.Name
	}
	if len(dmNotes) > 0 {
		payload["dm_notes"] = dmNotes
	}
	return payload
}

func privateChatMessageFromEvent(event SessionEvent) PrivateChatMessage {
	message := PrivateChatMessage{
		ID:        event.ID,
		SessionID: event.SessionID,
		Role:      strings.TrimSpace(fmt.Sprintf("%v", event.Payload["role"])),
		Content:   strings.TrimSpace(fmt.Sprintf("%v", event.Payload["content"])),
		Language:  strings.TrimSpace(fmt.Sprintf("%v", event.Payload["language"])),
		CreatedAt: event.CreatedAt,
	}
	message.PlayerSlotID = strings.TrimSpace(fmt.Sprintf("%v", event.Payload["player_slot_id"]))
	if characterID := strings.TrimSpace(fmt.Sprintf("%v", event.Payload["character_id"])); characterID != "" && characterID != "<nil>" {
		message.CharacterID = &characterID
	}
	return message
}

func (h *Handler) privateSidebarContext(ctx context.Context, portal PlayerPortalSession, query string) ([]GMContextChunk, error) {
	adventureDocuments, err := sessionAdventureDocuments(ctx, h.store, portal.Session)
	if err != nil {
		return nil, err
	}
	if len(adventureDocuments) == 0 {
		return []GMContextChunk{}, nil
	}
	return h.retrieveRelevantContextForDocuments(ctx, adventureDocuments, query, 3, false)
}

func applyPlayerPortalCharacterUpdate(character Character, req UpdatePlayerPortalCharacterRequest) (Character, error) {
	metadata := defaultMetadata(character.Metadata)
	if req.CurrentHitPoints != nil {
		if *req.CurrentHitPoints < 0 {
			return Character{}, fmt.Errorf("current_hit_points must be 0 or greater")
		}
		metadata["current_hit_points"] = strconv.Itoa(*req.CurrentHitPoints)
	}
	if req.TemporaryHitPoints != nil {
		if *req.TemporaryHitPoints < 0 {
			return Character{}, fmt.Errorf("temporary_hit_points must be 0 or greater")
		}
		metadata["temporary_hit_points"] = strconv.Itoa(*req.TemporaryHitPoints)
	}
	if req.CurrentMoney != nil {
		metadata["current_money"] = strings.TrimSpace(*req.CurrentMoney)
	}
	if req.ExperiencePoints != nil {
		metadata["experience_points"] = strings.TrimSpace(*req.ExperiencePoints)
	}
	if req.Inspiration != nil {
		metadata["inspiration"] = strings.TrimSpace(*req.Inspiration)
	}
	if req.SessionNotes != nil {
		metadata["session_notes"] = normalizePlayerPortalNotes(*req.SessionNotes)
	}
	if req.CurrentInventory != nil {
		metadata["current_inventory"] = normalizePlayerPortalList(req.CurrentInventory)
	}
	character.Metadata = metadata
	return character, nil
}

func applyPlayerPortalGroupInventoryUpdate(current SessionGroupInventory, req UpdatePlayerPortalGroupInventoryRequest) (SessionGroupInventory, error) {
	next := current
	if req.Gold != nil {
		if *req.Gold < 0 {
			return SessionGroupInventory{}, fmt.Errorf("gold must be 0 or greater")
		}
		next.Gold = *req.Gold
	}
	if req.Items != nil {
		next.Items = normalizePlayerPortalList(req.Items)
	}
	if req.Notes != nil {
		next.Notes = strings.TrimSpace(*req.Notes)
	}
	return next, nil
}

func normalizePlayerPortalList(items []string) []string {
	normalized := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func normalizePlayerPortalNotes(value string) []string {
	lines := strings.FieldsFunc(value, func(r rune) bool {
		return r == '\n' || r == ';'
	})
	return normalizePlayerPortalList(lines)
}
