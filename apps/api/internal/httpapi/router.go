package httpapi

import (
	"context"
	"errors"
	"fmt"
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
	engine.MaxMultipartMemory = max(cfg.MaxUploadBytes, max(cfg.MaxAudioUploadBytes, cfg.MaxZipUploadBytes))
	if err := engine.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		panic(fmt.Errorf("set trusted proxies: %w", err))
	}
	engine.Use(gin.Logger(), gin.Recovery(), corsMiddleware(cfg.CORSAllowedOrigins), jsonBodyLimitMiddleware(cfg.MaxJSONBodyBytes))

	registerRoutes(engine, newHandler(store, llmClient, llmGateway, ttsClient, sttClient, visionClient, cfg.UploadsDir, cfg), cfg)

	return &Server{
		engine: engine,
		store:  store,
	}
}

func (s *Server) Run(addr string) error {
	return s.engine.Run(addr)
}

func registerRoutes(router *gin.Engine, handler *Handler, cfg Config) {
	operatorOnly := requireOperatorMiddleware(cfg.DemoOperatorSecret)
	limiter := newFixedWindowRateLimiter()
	window := time.Duration(max(1, cfg.PublicRateLimitWindowSecs)) * time.Second

	router.GET("/api/health", handler.health)
	router.GET("/api/system/summary", handler.systemSummary)
	router.GET("/api/system/llm-gateway/status", handler.llmGatewayStatus)
	router.GET("/api/system/config", operatorOnly, handler.getSystemConfig)
	router.PUT("/api/system/config", operatorOnly, handler.updateSystemConfig)
	router.POST("/api/system/llm-test", operatorOnly, handler.testLLMConnection)
	router.POST("/api/system/llm-models", operatorOnly, handler.listLLMModels)
	router.POST("/api/demo/fungal-caverns", rateLimitMiddleware(limiter, "demo-seed", cfg.RateLimitDemoSeed, window), handler.createFungalCavernsDemo)
	router.DELETE("/api/demo/fungal-caverns", operatorOnly, handler.resetFungalCavernsDemo)
	router.GET("/api/campaigns", handler.listCampaigns)
	router.POST("/api/campaigns", operatorOnly, handler.createCampaign)
	router.GET("/api/adventures", handler.listAdventures)
	router.POST("/api/adventures", operatorOnly, handler.createAdventure)
	router.POST("/api/adventures/create-package", operatorOnly, handler.createAdventurePackage)
	router.DELETE("/api/adventures/:id", operatorOnly, handler.deleteAdventure)
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
	router.POST("/api/stt/transcriptions", rateLimitMiddleware(limiter, "stt", cfg.RateLimitSTT, window), handler.transcribeAudio)
	router.GET("/api/sessions/join/:token", handler.getJoinSessionPreview)
	router.POST("/api/sessions/join/:token", handler.joinSession)
	router.GET("/api/sessions/:id/player-links", handler.listPlayerLinks)
	router.POST("/api/sessions/:id/player-links", handler.createPlayerLink)
	router.PUT("/api/player-slots/:playerSlotId/character", handler.updatePlayerSlotCharacter)
	router.PUT("/api/player-slots/:playerSlotId/status", handler.updatePlayerSlotStatus)
	router.PUT("/api/player-slots/:playerSlotId/regenerate-link", operatorOnly, handler.regeneratePlayerLink)
	router.PUT("/api/player-slots/:playerSlotId/link-state", operatorOnly, handler.setPlayerLinkState)
	router.PUT("/api/player-slots/:playerSlotId/visible-state", handler.updatePlayerVisibleState)
	router.PUT("/api/player-portal/character", handler.updatePlayerPortalCharacter)
	router.PUT("/api/player-portal/group-inventory", handler.updatePlayerPortalGroupInventory)
	router.GET("/api/player-portal/private-chat", handler.listPlayerPortalPrivateChat)
	router.POST("/api/player-portal/private-chat", rateLimitMiddleware(limiter, "player-private-chat", cfg.RateLimitGMRespond, window), handler.sendPlayerPortalPrivateChat)
	router.GET("/api/documents", handler.listDocuments)
	router.GET("/api/documents/:id/file", handler.serveDocumentFile)
	router.POST("/api/documents", operatorOnly, handler.createDocument)
	router.POST("/api/documents/upload", operatorOnly, handler.uploadDocument)
	router.POST("/api/documents/:id/reindex-monsters", operatorOnly, handler.reindexDocumentMonsters)
	router.DELETE("/api/documents/:id", operatorOnly, handler.deleteDocument)
	router.POST("/api/adventures/import-zip", operatorOnly, handler.importAdventureZip)
	router.GET("/api/assets", handler.listAssets)
	router.GET("/api/assets/:id/file", handler.serveAssetFile)
	router.POST("/api/assets/upload", operatorOnly, handler.uploadAsset)
	router.DELETE("/api/assets/:id", operatorOnly, handler.deleteAsset)
	router.GET("/api/characters", handler.listCharacters)
	router.POST("/api/characters", handler.createCharacter)
	router.PUT("/api/characters/:id", handler.updateCharacter)
	router.DELETE("/api/characters/:id", handler.deleteCharacter)
	router.POST("/api/characters/builder/start", rateLimitMiddleware(limiter, "builder-start", cfg.RateLimitBuilder, window), handler.startCharacterBuilder)
	router.POST("/api/characters/:id/builder/message", rateLimitMiddleware(limiter, "builder-message", cfg.RateLimitBuilder, window), handler.characterBuilderMessage)
	router.POST("/api/characters/:id/builder/apply", handler.applyCharacterBuilderPatch)
	router.POST("/api/characters/:id/builder/finish", handler.finishCharacterBuilder)
	router.POST("/api/characters/import-sheet", handler.importCharacterSheet)
	router.POST("/api/characters/ability-scores/resolve", handler.resolveAbilityScores)
	router.POST("/api/characters/ability-scores/validate-assignment", handler.validateAbilityAssignment)
	router.POST("/api/vision/dice/detect", rateLimitMiddleware(limiter, "vision-detect", cfg.RateLimitVision, window), handler.detectDice)
	router.POST("/api/vision/dice/stabilize", rateLimitMiddleware(limiter, "vision-stabilize", cfg.RateLimitVision, window), handler.stabilizeDiceFrames)
	router.POST("/api/player-portal/join/:token", handler.playerPortalJoin)
	router.GET("/api/player-portal/me", handler.playerPortalMe)
	router.POST("/api/gm/respond", rateLimitMiddleware(limiter, "gm-respond", cfg.RateLimitGMRespond, window), handler.gmRespond)
	router.GET("/api/voice-profiles", handler.listVoiceProfiles)
}
