package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type Server struct {
	engine *gin.Engine
	store  *Store
}

func NewServer(cfg Config) *Server {
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	store, err := NewStore(ctx, cfg.DatabaseURL)
	if err != nil {
		panic(fmt.Errorf("init store: %w", err))
	}
	if err := store.ArchiveLLMSessionsForInactiveSessions(ctx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		panic(fmt.Errorf("archive inactive llm sessions: %w", err))
	}
	llmClient := NewLLMClient(cfg)
	llmGateway := NewLLMGateway(cfg)
	llmClient.gateway = llmGateway
	if systemConfig, configErr := store.GetSystemConfig(ctx); configErr == nil {
		// Ignore legacy settings without a provider. This keeps old local Ollama
		// settings from silently overriding the Build Week OpenAI default.
		if systemConfig.LLMProvider != "" {
			llmClient.UpdateRuntimeConfig(systemConfig.LLMProvider, systemConfig.LLMBaseURL, systemConfig.LLMModel)
		}
	}
	ttsClient := NewTTSClient(cfg)
	sttClient := NewSTTClient(cfg)
	visionClient := NewVisionClient(cfg)

	engine := gin.New()
	engine.Use(gin.Logger(), gin.Recovery(), corsMiddleware())

	registerRoutes(engine, newHandler(store, llmClient, llmGateway, ttsClient, sttClient, visionClient, cfg.UploadsDir))

	return &Server{
		engine: engine,
		store:  store,
	}
}

func (s *Server) Run(addr string) error {
	return s.engine.Run(addr)
}

func registerRoutes(router *gin.Engine, handler *Handler) {
	router.GET("/api/health", handler.health)
	router.GET("/api/system/summary", handler.systemSummary)
	router.GET("/api/system/llm-gateway/status", handler.llmGatewayStatus)
	router.GET("/api/system/config", handler.getSystemConfig)
	router.PUT("/api/system/config", handler.updateSystemConfig)
	router.POST("/api/system/llm-test", handler.testLLMConnection)
	router.POST("/api/system/llm-models", handler.listLLMModels)
	router.POST("/api/demo/fungal-caverns", handler.createFungalCavernsDemo)
	router.GET("/api/campaigns", handler.listCampaigns)
	router.POST("/api/campaigns", handler.createCampaign)
	router.GET("/api/adventures", handler.listAdventures)
	router.POST("/api/adventures", handler.createAdventure)
	router.POST("/api/adventures/create-package", handler.createAdventurePackage)
	router.DELETE("/api/adventures/:id", handler.deleteAdventure)
	router.GET("/api/sessions", handler.listSessions)
	router.GET("/api/sessions/:id", handler.getSession)
	router.GET("/api/sessions/:id/events", handler.listSessionEvents)
	router.POST("/api/sessions", handler.createSession)
	router.PUT("/api/sessions/:id", handler.updateSession)
	router.DELETE("/api/sessions/:id", handler.deleteSession)
	router.POST("/api/sessions/:id/start", handler.startSession)
	router.POST("/api/sessions/:id/pause", handler.pauseSession)
	router.POST("/api/sessions/:id/stop", handler.stopSession)
	router.PUT("/api/sessions/:id/runtime-state", handler.updateSessionRuntimeState)
	router.GET("/api/sessions/:id/tts-audio", handler.serveSessionTTSAudio)
	router.GET("/api/tts-audio", handler.serveTTSAudio)
	router.POST("/api/stt/transcriptions", handler.transcribeAudio)
	router.GET("/api/sessions/join/:token", handler.getJoinSessionPreview)
	router.POST("/api/sessions/join/:token", handler.joinSession)
	router.GET("/api/sessions/:id/player-links", handler.listPlayerLinks)
	router.POST("/api/sessions/:id/player-links", handler.createPlayerLink)
	router.PUT("/api/player-slots/:playerSlotId/character", handler.updatePlayerSlotCharacter)
	router.PUT("/api/player-slots/:playerSlotId/status", handler.updatePlayerSlotStatus)
	router.PUT("/api/player-slots/:playerSlotId/regenerate-link", handler.regeneratePlayerLink)
	router.PUT("/api/player-slots/:playerSlotId/link-state", handler.setPlayerLinkState)
	router.PUT("/api/player-slots/:playerSlotId/visible-state", handler.updatePlayerVisibleState)
	router.GET("/api/documents", handler.listDocuments)
	router.GET("/api/documents/:id/file", handler.serveDocumentFile)
	router.POST("/api/documents", handler.createDocument)
	router.POST("/api/documents/upload", handler.uploadDocument)
	router.POST("/api/documents/:id/reindex-monsters", handler.reindexDocumentMonsters)
	router.DELETE("/api/documents/:id", handler.deleteDocument)
	router.POST("/api/adventures/import-zip", handler.importAdventureZip)
	router.GET("/api/assets", handler.listAssets)
	router.GET("/api/assets/:id/file", handler.serveAssetFile)
	router.POST("/api/assets/upload", handler.uploadAsset)
	router.DELETE("/api/assets/:id", handler.deleteAsset)
	router.GET("/api/characters", handler.listCharacters)
	router.POST("/api/characters", handler.createCharacter)
	router.PUT("/api/characters/:id", handler.updateCharacter)
	router.DELETE("/api/characters/:id", handler.deleteCharacter)
	router.POST("/api/characters/builder/start", handler.startCharacterBuilder)
	router.POST("/api/characters/:id/builder/message", handler.characterBuilderMessage)
	router.POST("/api/characters/:id/builder/apply", handler.applyCharacterBuilderPatch)
	router.POST("/api/characters/:id/builder/finish", handler.finishCharacterBuilder)
	router.POST("/api/characters/import-sheet", handler.importCharacterSheet)
	router.POST("/api/characters/ability-scores/resolve", handler.resolveAbilityScores)
	router.POST("/api/characters/ability-scores/validate-assignment", handler.validateAbilityAssignment)
	router.POST("/api/vision/dice/detect", handler.detectDice)
	router.POST("/api/vision/dice/stabilize", handler.stabilizeDiceFrames)
	router.POST("/api/player-portal/join/:token", handler.playerPortalJoin)
	router.GET("/api/player-portal/me", handler.playerPortalMe)
	router.POST("/api/gm/respond", handler.gmRespond)
	router.GET("/api/voice-profiles", handler.listVoiceProfiles)
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
