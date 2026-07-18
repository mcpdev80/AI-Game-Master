package httpapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

func (h *Handler) listVoiceProfiles(c *gin.Context) {
	provider := h.ttsClient.Provider()
	model := h.ttsClient.Model()
	c.JSON(http.StatusOK, gin.H{
		"items": []gin.H{
			{
				"id":                "narrator-default",
				"name":              "Narrator Default",
				"language":          "multilingual",
				"style":             "clear",
				"role":              "narrator",
				"provider":          provider,
				"provider_model":    model,
				"provider_voice_id": h.ttsClient.VoiceForProfile("narrator-default"),
				"is_default":        true,
				"description":       "Clear, immersive AI-generated narration voice",
			},
			{
				"id":                "npc-default",
				"name":              "NPC Default",
				"language":          "multilingual",
				"style":             "warm",
				"role":              "npc",
				"provider":          provider,
				"provider_model":    model,
				"provider_voice_id": h.ttsClient.VoiceForProfile("npc-default"),
				"is_default":        false,
				"description":       "Warm AI-generated voice for friendly or neutral NPCs",
			},
			{
				"id":                "orc-deep",
				"name":              "Orc Deep",
				"language":          "multilingual",
				"style":             "rough",
				"role":              "monster",
				"provider":          provider,
				"provider_model":    model,
				"provider_voice_id": h.ttsClient.VoiceForProfile("orc-deep"),
				"is_default":        false,
				"description":       "Tiefe, raue Stimme fuer Orks oder brute Monsterrollen",
			},
			{
				"id":                "elf-bright",
				"name":              "Elf Bright",
				"language":          "multilingual",
				"style":             "light",
				"role":              "npc",
				"provider":          provider,
				"provider_model":    model,
				"provider_voice_id": h.ttsClient.VoiceForProfile("elf-bright"),
				"is_default":        false,
				"description":       "Heller, weicher Klang fuer Elfen oder feinere Figuren",
			},
			{
				"id":                "rules-neutral",
				"name":              "Rules Neutral",
				"language":          "multilingual",
				"style":             "precise",
				"role":              "rules",
				"provider":          provider,
				"provider_model":    model,
				"provider_voice_id": h.ttsClient.VoiceForProfile("rules-neutral"),
				"is_default":        false,
				"description":       "Neutrale Stimme fuer Regelreferenzen und Systemhinweise",
			},
			{
				"id":                "builder-friendly-male",
				"name":              "Builder Friendly Male",
				"language":          "multilingual",
				"style":             "friendly",
				"role":              "narrator",
				"provider":          provider,
				"provider_model":    model,
				"provider_voice_id": h.ttsClient.VoiceForProfile("builder-friendly-male"),
				"is_default":        false,
				"description":       "Friendly, clear AI-generated character-builder voice",
			},
		},
	})
}

func resolveLocalVoiceProviderID(profileID string) string {
	switch strings.TrimSpace(strings.ToLower(profileID)) {
	case "narrator-default":
		return "narrator-default"
	case "npc-default":
		return "narrator-default"
	case "orc-deep":
		return "narrator-default"
	case "elf-bright":
		return "narrator-default"
	case "rules-neutral":
		return "narrator-default"
	case "builder-friendly-male":
		return "narrator-default"
	default:
		return "narrator-default"
	}
}

func voiceProfileInstruction(profileID string, language string) string {
	isGerman := strings.HasPrefix(strings.ToLower(strings.TrimSpace(language)), "de")
	if !isGerman {
		switch strings.TrimSpace(profileID) {
		case "orc-deep":
			return "Speak in English with a deep, rough, intimidating fantasy creature voice. Keep the delivery concise and forceful."
		case "elf-bright":
			return "Speak in English with a bright, soft, elegant fantasy voice."
		case "rules-neutral":
			return "Speak in English with a neutral, precise, calm delivery for a concise rules reference."
		case "builder-friendly-male":
			return "Speak in English with a friendly, calm, clear narrator voice. Sound empathetic and easy to understand."
		case "npc-default":
			return "Speak in English with a warm, clear, characterful NPC voice."
		default:
			return "Speak in English with a clear, immersive, well-paced fantasy narrator voice."
		}
	}
	switch strings.TrimSpace(profileID) {
	case "orc-deep":
		return "Sprich auf Deutsch mit tiefer, rauer, orkischer Stimme. Kurz und druckvoll."
	case "elf-bright":
		return "Sprich auf Deutsch mit heller, weicher, eleganter Elfenstimme."
	case "rules-neutral":
		return "Sprich auf Deutsch neutral, praezise und ruhig wie ein Regelassistent."
	case "builder-friendly-male":
		return "Sprich auf Deutsch freundlich, ruhig und klar als hilfreicher Erzähler. Kling empathisch und gut verständlich."
	case "npc-default":
		return "Sprich auf Deutsch warm, klar und charaktervoll wie ein NPC."
	default:
		return "Sprich auf Deutsch klar, immersiv und gut verstaendlich als Erzaehler."
	}
}

func deriveSessionSpeechText(session Session) string {
	if session.State.VisualMode == "rules_reference" {
		excerpt := strings.TrimSpace(safeOptionalString(session.State.VisualPayload["excerpt"]))
		documentName := strings.TrimSpace(safeOptionalString(session.State.VisualPayload["document_name"]))
		if excerpt != "" && documentName != "" {
			return fmt.Sprintf("Regelreferenz aus %s. %s", documentName, excerpt)
		}
		if excerpt != "" {
			return excerpt
		}
	}

	narration := strings.TrimSpace(safeOptionalString(session.State.VisualPayload["narration"]))
	if narration != "" {
		return narration
	}
	scene := strings.TrimSpace(safeOptionalString(session.State.VisualPayload["scene"]))
	if scene != "" {
		return scene
	}
	title := strings.TrimSpace(safeOptionalString(session.State.VisualPayload["title"]))
	if title != "" {
		switch session.State.VisualMode {
		case "combat":
			return fmt.Sprintf("Kampfansicht aktiv. %s.", title)
		case "scene":
			return fmt.Sprintf("Szenenansicht aktiv. %s.", title)
		case "pause_or_recap":
			return fmt.Sprintf("Sessionstatus. %s.", title)
		default:
			return title
		}
	}
	if session.State.VisualMode == "dice_capture" {
		if rollSummary := strings.TrimSpace(safeOptionalString(session.State.VisualPayload["result"])); rollSummary != "" {
			return fmt.Sprintf("Wurfergebnis erkannt. %s.", rollSummary)
		}
		if session.State.LastDiceRoll != nil {
			parts := make([]string, 0, len(session.State.LastDiceRoll.Dice))
			for _, die := range session.State.LastDiceRoll.Dice {
				if die.Type != "" {
					parts = append(parts, fmt.Sprintf("%s %d", die.Type, die.Value))
				} else {
					parts = append(parts, fmt.Sprintf("%d", die.Value))
				}
			}
			if len(parts) > 0 {
				return fmt.Sprintf("Letzter Wurf: %s.", strings.Join(parts, ", "))
			}
		}
		return "Würfelerkennung aktiv. Bitte prüft das erkannte Ergebnis."
	}
	if strings.TrimSpace(session.State.LastNarration) != "" {
		return strings.TrimSpace(session.State.LastNarration)
	}
	if strings.TrimSpace(session.State.SessionRecap) != "" {
		return strings.TrimSpace(session.State.SessionRecap)
	}
	return ""
}

func (h *Handler) serveSessionTTSAudio(c *gin.Context) {
	ttsCtx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	session, err := h.store.GetSession(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
			return
		}
		errorResponse(c, http.StatusInternalServerError, "load session for tts", err)
		return
	}

	text := strings.TrimSpace(c.Query("text"))
	if text == "" {
		text = deriveSessionSpeechText(session)
	}
	if text == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no speech text available for session"})
		return
	}

	voiceID := strings.TrimSpace(c.Query("voice"))
	if voiceID == "" {
		voiceID = strings.TrimSpace(session.State.ActiveVoiceProfileID)
	}
	if voiceID == "" && session.DefaultVoiceProfileID != nil {
		voiceID = strings.TrimSpace(*session.DefaultVoiceProfileID)
	}
	if voiceID == "" {
		voiceID = "narrator-default"
	}

	audio, contentType, err := h.ttsClient.Synthesize(ttsCtx, text, h.ttsClient.VoiceForProfile(voiceID), voiceProfileInstruction(voiceID, session.Language))
	if err != nil {
		errorResponse(c, http.StatusBadGateway, "synthesize session speech", err)
		return
	}

	c.Header("Cache-Control", "no-store")
	c.Header("Content-Type", contentType)
	c.Header("Content-Length", fmt.Sprintf("%d", len(audio)))
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = c.Writer.Write(audio)
}

func (h *Handler) serveTTSAudio(c *gin.Context) {
	ttsCtx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	text := strings.TrimSpace(c.Query("text"))
	if text == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "text is required"})
		return
	}
	voiceID := strings.TrimSpace(c.Query("voice"))
	if voiceID == "" {
		voiceID = "narrator-default"
	}
	language := strings.TrimSpace(c.Query("language"))
	if language == "" {
		language = "en"
	}
	audio, contentType, err := h.ttsClient.Synthesize(ttsCtx, text, h.ttsClient.VoiceForProfile(voiceID), voiceProfileInstruction(voiceID, language))
	if err != nil {
		errorResponse(c, http.StatusBadGateway, "synthesize speech", err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Type", contentType)
	c.Header("Content-Length", fmt.Sprintf("%d", len(audio)))
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = c.Writer.Write(audio)
}

func (h *Handler) transcribeAudio(c *gin.Context) {
	sttCtx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()

	file, err := c.FormFile("file")
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "audio file is required", err)
		return
	}

	src, err := file.Open()
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "open uploaded audio", err)
		return
	}
	defer src.Close()

	data, err := io.ReadAll(src)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "read uploaded audio", err)
		return
	}

	text, err := h.sttClient.Transcribe(sttCtx, file.Filename, file.Header.Get("Content-Type"), c.PostForm("language"), data)
	if err != nil {
		errorResponse(c, http.StatusBadGateway, "transcribe speech", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"text":     text,
		"model":    h.sttClient.Model(),
		"provider": h.sttClient.Provider(),
	})
}

func guessAmbientCue(activeCue string, narration string, currentScene string) (string, map[string]any) {
	source := strings.ToLower(strings.Join([]string{activeCue, narration, currentScene}, " "))
	type cue struct {
		id  string
		url string
	}
	candidates := []struct {
		match []string
		cue   cue
	}{
		{match: []string{"tavern", "taverne", "inn", "schenke"}, cue: cue{id: "tavern_music", url: "https://sounds.tabletopaudio.com/177_Tavern_Music.mp3"}},
		{match: []string{"wikinger", "viking", "met hall"}, cue: cue{id: "viking_tavern", url: "https://sounds.tabletopaudio.com/407_Viking_Tavern.mp3"}},
		{match: []string{"rain", "regen", "storm", "gewitter", "village", "dorf"}, cue: cue{id: "rainy_village", url: "https://sounds.tabletopaudio.com/235_Rainy_Village.mp3"}},
		{match: []string{"dungeon", "asylum", "kerker", "verlies", "ruine", "keller"}, cue: cue{id: "dungeon_asylum", url: "https://sounds.tabletopaudio.com/437_Dungeon_Asylum.mp3"}},
		{match: []string{"council", "rat", "palace", "thron", "könig", "koenig", "hof"}, cue: cue{id: "privy_council", url: "https://sounds.tabletopaudio.com/504_Privy_Council.mp3"}},
		{match: []string{"festival", "markt", "feier", "stadtfest"}, cue: cue{id: "village_festival", url: "https://sounds.tabletopaudio.com/500_Village_Festival.mp3"}},
		{match: []string{"city", "stadt", "crowd", "parade"}, cue: cue{id: "dedication_day", url: "https://sounds.tabletopaudio.com/495_Dedication_Day.mp3"}},
		{match: []string{"wasteland", "verwüstung", "verwuestung", "lava", "asche", "ruined lands"}, cue: cue{id: "ravaged_lands", url: "https://sounds.tabletopaudio.com/499_Ravaged_Lands.mp3"}},
		{match: []string{"ice", "snow", "frost", "winter", "gletscher"}, cue: cue{id: "frost_giant_ridge", url: "https://sounds.tabletopaudio.com/503_Frost_Giant_Ridge.mp3"}},
		{match: []string{"dragon", "drache"}, cue: cue{id: "red_dragon_dawn", url: "https://sounds.tabletopaudio.com/491_Red_Dragon_Dawn.mp3"}},
		{match: []string{"battle", "kampf", "war", "schlacht", "ambush", "überfall", "ueberfall"}, cue: cue{id: "war_wagon", url: "https://sounds.tabletopaudio.com/486_War_Wagon.mp3"}},
		{match: []string{"castle", "burg", "siege", "belagerung", "bastion"}, cue: cue{id: "ready_the_castle", url: "https://sounds.tabletopaudio.com/484_Ready_the_Castle.mp3"}},
		{match: []string{"manor", "villa", "anwesen", "mansion", "haunted house"}, cue: cue{id: "manor_dark", url: "https://sounds.tabletopaudio.com/488_Manor_Dark.mp3"}},
		{match: []string{"urban", "roof", "rooftop", "spy", "city night"}, cue: cue{id: "urban_rooftop", url: "https://sounds.tabletopaudio.com/498_Urban_Rooftop.mp3"}},
		{match: []string{"alarm", "alert", "military", "soldiers", "einsatz"}, cue: cue{id: "high_alert", url: "https://sounds.tabletopaudio.com/487_High_Alert.mp3"}},
		{match: []string{"train", "zug", "heist", "western", "station"}, cue: cue{id: "train_job", url: "https://sounds.tabletopaudio.com/391_Train_Job.mp3"}},
	}
	for _, candidate := range candidates {
		for _, term := range candidate.match {
			if strings.Contains(source, term) {
				return candidate.cue.id, map[string]any{
					"provider": "tabletopaudio",
					"type":     "stream",
					"url":      candidate.cue.url,
				}
			}
		}
	}
	return "", nil
}
