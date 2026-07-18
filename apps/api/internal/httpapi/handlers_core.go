package httpapi

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	store        *Store
	llmClient    *LLMClient
	llmGateway   *LLMGateway
	ttsClient    *TTSClient
	sttClient    *STTClient
	visionClient *VisionClient
	uploadsDir   string
}

func newHandler(store *Store, llmClient *LLMClient, llmGateway *LLMGateway, ttsClient *TTSClient, sttClient *STTClient, visionClient *VisionClient, uploadsDir string) *Handler {
	return &Handler{store: store, llmClient: llmClient, llmGateway: llmGateway, ttsClient: ttsClient, sttClient: sttClient, visionClient: visionClient, uploadsDir: uploadsDir}
}

func (h *Handler) health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	status := "ok"
	database := "ok"
	if err := h.store.Ping(ctx); err != nil {
		status = "degraded"
		database = "unreachable"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    status,
		"service":   "dungeon-master-api",
		"database":  database,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *Handler) systemSummary(c *gin.Context) {
	stats, err := h.store.Stats(c.Request.Context())
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "load system summary", err)
		return
	}
	llmConfig := h.llmClient.CurrentConfig()
	activeGatewaySessions, archivedGatewaySessions, _ := h.store.CountLLMSessionsByStatus(c.Request.Context())
	gatewayStatus := h.llmGateway.Status(activeGatewaySessions, archivedGatewaySessions)

	c.JSON(http.StatusOK, gin.H{
		"services": []gin.H{
			{"name": "api", "status": "online"},
			{"name": "postgres", "status": "online"},
			{"name": "web", "status": "external"},
			{"name": "ingestion", "status": "planned"},
			{"name": "vision", "status": "planned"},
			{"name": "media-director", "status": "planned"},
		},
		"counts":         stats,
		"active_session": nil,
		"last_ai_action": nil,
		"llm": gin.H{
			"provider": llmConfig.LLMProvider,
			"base_url": llmConfig.LLMBaseURL,
			"model":    llmConfig.LLMModel,
		},
		"llm_gateway": gatewayStatus,
		"tts": gin.H{
			"provider": h.ttsClient.Provider(),
			"base_url": h.ttsClient.BaseURL(),
			"model":    h.ttsClient.Model(),
		},
		"stt": gin.H{
			"provider": h.sttClient.Provider(),
			"base_url": h.sttClient.BaseURL(),
			"model":    h.sttClient.Model(),
		},
	})
}

func (h *Handler) llmGatewayStatus(c *gin.Context) {
	activeGatewaySessions, archivedGatewaySessions, err := h.store.CountLLMSessionsByStatus(c.Request.Context())
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "load llm gateway status", err)
		return
	}
	c.JSON(http.StatusOK, h.llmGateway.Status(activeGatewaySessions, archivedGatewaySessions))
}

func (h *Handler) getSystemConfig(c *gin.Context) {
	cfg, err := h.store.GetSystemConfig(c.Request.Context())
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "load system config", err)
		return
	}

	current := h.llmClient.CurrentConfig()
	if strings.TrimSpace(cfg.LLMProvider) == "" {
		cfg.LLMProvider = current.LLMProvider
	}
	if strings.TrimSpace(cfg.LLMBaseURL) == "" {
		cfg.LLMBaseURL = current.LLMBaseURL
	}
	if strings.TrimSpace(cfg.LLMModel) == "" {
		cfg.LLMModel = current.LLMModel
	}

	c.JSON(http.StatusOK, cfg)
}

func (h *Handler) updateSystemConfig(c *gin.Context) {
	var req UpdateSystemConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid system config payload", err)
		return
	}

	cfg := SystemConfig{
		LLMProvider: strings.ToLower(strings.TrimSpace(req.LLMProvider)),
		LLMBaseURL:  strings.TrimSpace(req.LLMBaseURL),
		LLMModel:    strings.TrimSpace(req.LLMModel),
	}
	if cfg.LLMProvider == "" {
		cfg.LLMProvider = h.llmClient.CurrentConfig().LLMProvider
	}
	if cfg.LLMProvider != "openai" && cfg.LLMProvider != "local" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "llm_provider must be openai or local"})
		return
	}
	if cfg.LLMBaseURL == "" || cfg.LLMModel == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "llm_base_url and llm_model are required"})
		return
	}

	updated, err := h.store.UpdateSystemConfig(c.Request.Context(), cfg)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "save system config", err)
		return
	}

	h.llmClient.UpdateRuntimeConfig(updated.LLMProvider, updated.LLMBaseURL, updated.LLMModel)
	c.JSON(http.StatusOK, updated)
}

func (h *Handler) testLLMConnection(c *gin.Context) {
	var req UpdateSystemConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid llm test payload", err)
		return
	}
	client := h.llmClient.RuntimeClient(SystemConfig{LLMProvider: req.LLMProvider, LLMBaseURL: req.LLMBaseURL, LLMModel: req.LLMModel})
	content, model, err := client.TestConnection(c.Request.Context())
	if err != nil {
		errorResponse(c, http.StatusBadGateway, "llm test failed", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"content": content, "model": model})
}

func (h *Handler) listLLMModels(c *gin.Context) {
	var req UpdateSystemConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid model list payload", err)
		return
	}
	client := h.llmClient.RuntimeClient(SystemConfig{LLMProvider: req.LLMProvider, LLMBaseURL: req.LLMBaseURL, LLMModel: req.LLMModel})
	models, err := client.ListModels(c.Request.Context())
	if err != nil {
		errorResponse(c, http.StatusBadGateway, "load llm models", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"models": models})
}

func errorResponse(c *gin.Context, status int, message string, err error) {
	log.Printf(
		"httpapi error: status=%d method=%s path=%s message=%q details=%v",
		status,
		c.Request.Method,
		c.Request.URL.Path,
		message,
		err,
	)
	c.JSON(status, gin.H{
		"error":   message,
		"details": err.Error(),
	})
}

func chooseLanguage(preferred string, fallback string) string {
	if preferred != "" {
		return preferred
	}
	if fallback != "" {
		return fallback
	}
	return "de"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func stringPtrOrNil(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}
