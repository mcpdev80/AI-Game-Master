package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

func (h *Handler) listSessions(c *gin.Context) {
	items, err := h.store.ListSessions(c.Request.Context())
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "list sessions", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) getSession(c *gin.Context) {
	item, err := h.store.GetSession(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
			return
		}

		errorResponse(c, http.StatusInternalServerError, "load session", err)
		return
	}

	c.JSON(http.StatusOK, item)
}

func (h *Handler) listSessionEvents(c *gin.Context) {
	items, err := h.store.ListSessionEvents(c.Request.Context(), c.Param("id"), 30)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "list session events", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) createSession(c *gin.Context) {
	var req CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid session payload", err)
		return
	}

	if strings.TrimSpace(req.CampaignID) == "" && req.AdventureID != nil && strings.TrimSpace(*req.AdventureID) != "" {
		adventures, err := h.store.ListAdventures(c.Request.Context())
		if err != nil {
			errorResponse(c, http.StatusInternalServerError, "load adventures", err)
			return
		}
		for _, adventure := range adventures {
			if adventure.ID == *req.AdventureID && adventure.CampaignID != nil {
				req.CampaignID = *adventure.CampaignID
				break
			}
		}
	}
	if strings.TrimSpace(req.CampaignID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "campaign_id is required or must be derivable from adventure"})
		return
	}

	item, err := h.store.CreateSession(c.Request.Context(), req)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "create session", err)
		return
	}

	playSession, err := h.ensureScopedLLMSession(
		c.Request.Context(),
		"session",
		item.ID,
		"campaign_play_session",
		"narration",
		"",
		"",
		"",
		nil,
		12000,
		map[string]any{"kind": "campaign_play"},
		map[string]any{"session_id": item.ID, "campaign_id": item.CampaignID},
	)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "create play llm session", err)
		return
	}
	rulesSession, err := h.ensureScopedLLMSession(
		c.Request.Context(),
		"session",
		item.ID,
		"rules_lookup_session",
		"rules",
		"",
		"",
		"",
		nil,
		6000,
		map[string]any{"kind": "rules_lookup"},
		map[string]any{"session_id": item.ID, "campaign_id": item.CampaignID},
	)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "create rules llm session", err)
		return
	}
	summarySession, err := h.ensureScopedLLMSession(
		c.Request.Context(),
		"session",
		item.ID,
		"summary_session",
		"summary",
		"",
		"",
		"",
		nil,
		4000,
		map[string]any{"kind": "session_summary"},
		map[string]any{"session_id": item.ID, "campaign_id": item.CampaignID},
	)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "create summary llm session", err)
		return
	}

	item.State.PlayLLMSessionID = playSession.ID
	item.State.RulesLLMSessionID = rulesSession.ID
	item.State.SummarySessionID = summarySession.ID
	if strings.TrimSpace(item.State.VisualMode) == "" {
		item.State.VisualMode = "pause_or_recap"
	}
	if strings.TrimSpace(item.State.AudioMode) == "" {
		item.State.AudioMode = "silence"
	}
	if strings.TrimSpace(item.State.VoiceMode) == "" {
		item.State.VoiceMode = "narrator"
	}
	if strings.TrimSpace(item.State.ActiveSpeakerName) == "" {
		item.State.ActiveSpeakerName = "AI DM"
	}
	if strings.TrimSpace(item.State.ActiveSpeakerRole) == "" {
		item.State.ActiveSpeakerRole = "narrator"
	}
	if strings.TrimSpace(item.State.TTSStatus) == "" {
		item.State.TTSStatus = "idle"
	}
	if len(item.State.VisualPayload) == 0 {
		item.State.VisualPayload = map[string]any{
			"title": item.Name,
			"scene": "Session angelegt. Warte auf Spieler und Start.",
		}
	}
	if strings.TrimSpace(item.State.SessionRecap) == "" {
		item.State.SessionRecap = "Session angelegt. Warte auf Spieler und Start."
	}
	if err := h.store.UpdateSessionState(c.Request.Context(), item.ID, item.State); err != nil {
		errorResponse(c, http.StatusInternalServerError, "link llm sessions to session", err)
		return
	}

	c.JSON(http.StatusCreated, item)
}

func (h *Handler) updateSession(c *gin.Context) {
	var req UpdateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid session request", err)
		return
	}

	item, err := h.store.UpdateSession(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
			return
		}
		errorResponse(c, http.StatusInternalServerError, "update session", err)
		return
	}

	_, _ = h.store.CreateSessionEvent(c.Request.Context(), item.ID, "session_updated", map[string]any{
		"name":                item.Name,
		"adventure_id":        item.AdventureID,
		"ruleset_work":        item.RulesetWork,
		"ruleset_version":     item.RulesetVersion,
		"target_player_count": item.TargetPlayerCount,
	})

	c.JSON(http.StatusOK, item)
}

func (h *Handler) deleteSession(c *gin.Context) {
	h.llmGateway.teardownSession(c.Request.Context(), "session", c.Param("id"))
	if err := h.store.DeleteSession(c.Request.Context(), c.Param("id")); err != nil {
		errorResponse(c, http.StatusInternalServerError, "delete session", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (h *Handler) startSession(c *gin.Context) {
	sessionID := c.Param("id")
	slots, err := h.store.ListPlayerSlots(c.Request.Context(), sessionID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "list player slots", err)
		return
	}
	activePlayers := 0
	readyPlayers := 0
	for _, slot := range slots {
		if slot.CharacterID == nil {
			continue
		}
		if slot.Status == "joined" || slot.Status == "ready" {
			activePlayers++
		}
		if slot.Status == "ready" {
			readyPlayers++
		}
	}
	if activePlayers == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no joined players are ready to start"})
		return
	}
	if readyPlayers != activePlayers {
		c.JSON(http.StatusBadRequest, gin.H{"error": "all joined players must be ready before starting"})
		return
	}
	h.updateSessionStatus(c, "live")
}

func (h *Handler) pauseSession(c *gin.Context) {
	h.updateSessionStatus(c, "paused")
}

func (h *Handler) stopSession(c *gin.Context) {
	h.updateSessionStatus(c, "finished")
}

func (h *Handler) updateSessionRuntimeState(c *gin.Context) {
	var req UpdateSessionRuntimeStateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid session runtime payload", err)
		return
	}
	session, err := h.store.GetSession(c.Request.Context(), c.Param("id"))
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "load session", err)
		return
	}
	nextState := session.State
	if strings.TrimSpace(req.VisualMode) != "" {
		nextState.VisualMode = req.VisualMode
	}
	if req.VisualPayload != nil {
		nextState.VisualPayload = req.VisualPayload
	}
	if strings.TrimSpace(req.AudioMode) != "" {
		nextState.AudioMode = req.AudioMode
	}
	if req.AudioPayload != nil {
		nextState.AudioPayload = req.AudioPayload
	}
	if strings.TrimSpace(req.VoiceMode) != "" {
		nextState.VoiceMode = req.VoiceMode
	}
	if strings.TrimSpace(req.ActiveVoiceProfileID) != "" {
		nextState.ActiveVoiceProfileID = req.ActiveVoiceProfileID
	}
	if strings.TrimSpace(req.ActiveSpeakerRole) != "" {
		nextState.ActiveSpeakerRole = req.ActiveSpeakerRole
	}
	if strings.TrimSpace(req.ActiveSpeakerName) != "" {
		nextState.ActiveSpeakerName = req.ActiveSpeakerName
	}
	if strings.TrimSpace(req.TTSStatus) != "" {
		nextState.TTSStatus = req.TTSStatus
	}
	if strings.TrimSpace(req.AmbientCueID) != "" {
		nextState.AmbientCueID = req.AmbientCueID
	}
	if strings.TrimSpace(req.SessionRecap) != "" {
		nextState.SessionRecap = req.SessionRecap
	}
	if err := h.store.UpdateSessionState(c.Request.Context(), session.ID, nextState); err != nil {
		errorResponse(c, http.StatusInternalServerError, "update session runtime state", err)
		return
	}
	session.State = nextState
	c.JSON(http.StatusOK, session)
}

func (h *Handler) updateSessionStatus(c *gin.Context, status string) {
	session, err := h.store.UpdateSessionStatus(c.Request.Context(), c.Param("id"), status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
			return
		}
		errorResponse(c, http.StatusInternalServerError, "update session status", err)
		return
	}

	_, _ = h.store.CreateSessionEvent(c.Request.Context(), session.ID, "session_status_changed", map[string]any{
		"status": status,
	})
	if status == "paused" || status == "finished" {
		_ = h.store.ArchiveLLMSessionsByScope(c.Request.Context(), "session", session.ID)
		h.llmGateway.teardownSession(c.Request.Context(), "session", session.ID)
		nextState := session.State
		nextState.VisualMode = "pause_or_recap"
		nextState.AudioMode = "silence"
		nextState.TTSStatus = "idle"
		nextState.VisualPayload = map[string]any{
			"title": session.Name,
			"scene": firstNonEmpty(session.State.SessionRecap, "Session pausiert oder beendet."),
		}
		_ = h.store.UpdateSessionState(c.Request.Context(), session.ID, nextState)
		session.State = nextState
	}
	if status == "live" {
		nextState := session.State
		isReopening := hasMeaningfulSessionProgress(session)
		placeholder := "Die Session beginnt. Der AI DM eröffnet die Szene."
		if isReopening {
			placeholder = firstNonEmpty(
				session.State.SessionRecap,
				session.State.SceneSummary,
				session.State.LastNarration,
				"Ihr nehmt das Abenteuer wieder auf. Der AI DM fasst die Lage kurz zusammen.",
			)
		}
		nextState.VisualMode = "scene"
		nextState.VisualPayload = map[string]any{}
		nextState.AudioMode = "tts_only"
		nextState.AudioPayload = nil
		nextState.VoiceMode = "narrator"
		nextState.ActiveSpeakerRole = "narrator"
		nextState.ActiveSpeakerName = "AI DM"
		nextState.TTSStatus = "queued"
		nextState.LastDiceRoll = nil
		nextState.LastConfirmedRoll = nil
		opening := firstNonEmpty(
			session.CurrentScene,
			session.State.SceneSummary,
			session.State.LastNarration,
			placeholder,
		)
		nextState.LastNarration = opening
		nextState.SceneSummary = firstNonEmpty(nextState.SceneSummary, opening)
		nextState.VisualPayload = map[string]any{
			"title":     session.Name,
			"scene":     opening,
			"narration": opening,
		}
		_ = h.store.UpdateSessionState(c.Request.Context(), session.ID, nextState)
		session.State = nextState
		go h.generateLiveSessionNarration(session, isReopening)
	}

	c.JSON(http.StatusOK, session)
}

func (h *Handler) generateLiveSessionNarration(session Session, reopening bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	currentSession, err := h.store.GetSession(ctx, session.ID)
	if err != nil {
		return
	}
	if currentSession.Status != "live" {
		return
	}

	var response GMResponse
	if reopening {
		response, err = h.generateSessionReopening(ctx, currentSession)
	} else {
		response, err = h.generateSessionOpening(ctx, currentSession)
	}
	if err != nil || strings.TrimSpace(response.Narration) == "" {
		return
	}

	nextState := currentSession.State
	nextState.VisualMode = "scene"
	nextState.LastNarration = response.Narration
	nextState.SceneSummary = firstNonEmpty(response.Narration, nextState.SceneSummary)
	nextState.LastDMNotes = response.DMNotes
	nextState.TTSStatus = "queued"
	nextState.VisualPayload = map[string]any{
		"title":     currentSession.Name,
		"scene":     response.Narration,
		"narration": response.Narration,
	}
	if cueID, payload := guessAmbientCue(nextState.ActiveMediaCue, response.Narration, currentSession.CurrentScene); cueID != "" {
		nextState.AmbientCueID = cueID
		nextState.AudioMode = "ambient_loop"
		nextState.AudioPayload = payload
	} else {
		nextState.AmbientCueID = ""
		nextState.AudioMode = "tts_only"
		nextState.AudioPayload = nil
	}
	nextState.SessionRecap = firstNonEmpty(response.Narration, nextState.SessionRecap)
	_ = h.store.UpdateSessionState(ctx, currentSession.ID, nextState)
}
