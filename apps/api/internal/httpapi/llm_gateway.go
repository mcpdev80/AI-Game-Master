package httpapi

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	redis "github.com/redis/go-redis/v9"
)

type llmRequestMeta struct {
	Profile        string
	ScopeType      string
	ScopeID        string
	MaxInputTokens int
}

type llmRequestMetaKey struct{}

var (
	errLLMGatewayBusy = errors.New("llm gateway is busy")
	errLLMCircuitOpen = errors.New("llm gateway circuit breaker is open")
)

type llmProfileLimit struct {
	Name            string
	MaxInputTokens  int
	MaxOutputTokens int
	Timeout         time.Duration
	LiveTurnWindow  int
}

type LLMGateway struct {
	maxConcurrent       int
	breakerThreshold    int
	breakerCooldown     time.Duration
	semaphore           chan struct{}
	redis               *redis.Client
	mu                  sync.Mutex
	breakerUntil        time.Time
	consecutiveFailures int
	lastError           string
	inflight            int
	rejected            int64
	timeouts            int64
	profiles            map[string]llmProfileLimit
}

func NewLLMGateway(cfg Config) *LLMGateway {
	var redisClient *redis.Client
	if strings.TrimSpace(cfg.RedisURL) != "" {
		if options, err := redis.ParseURL(cfg.RedisURL); err == nil {
			redisClient = redis.NewClient(options)
		}
	}
	return &LLMGateway{
		maxConcurrent:    max(cfg.LLMMaxConcurrent, 1),
		breakerThreshold: max(cfg.LLMBreakerThreshold, 2),
		breakerCooldown:  time.Duration(max(cfg.LLMBreakerCooldownSeconds, 15)) * time.Second,
		semaphore:        make(chan struct{}, max(cfg.LLMMaxConcurrent, 1)),
		redis:            redisClient,
		profiles: map[string]llmProfileLimit{
			// Encounter prompts combine the game-master contract, strict output
			// instructions, session state, and compact character context. Keep the
			// guard aligned with the 12k campaign-play session budget.
			"scene":             {Name: "scene", MaxInputTokens: 12000, MaxOutputTokens: 700, Timeout: 70 * time.Second, LiveTurnWindow: 8},
			"rules":             {Name: "rules", MaxInputTokens: 6000, MaxOutputTokens: 320, Timeout: 70 * time.Second, LiveTurnWindow: 8},
			"opening":           {Name: "opening", MaxInputTokens: 12000, MaxOutputTokens: 900, Timeout: 90 * time.Second, LiveTurnWindow: 8},
			"reopening":         {Name: "reopening", MaxInputTokens: 12000, MaxOutputTokens: 700, Timeout: 90 * time.Second, LiveTurnWindow: 8},
			"summary":           {Name: "summary", MaxInputTokens: 4000, MaxOutputTokens: 220, Timeout: 35 * time.Second, LiveTurnWindow: 8},
			"character_builder": {Name: "character_builder", MaxInputTokens: 110000, MaxOutputTokens: 1200, Timeout: 90 * time.Second, LiveTurnWindow: 8},
			"builder":           {Name: "builder", MaxInputTokens: 110000, MaxOutputTokens: 1200, Timeout: 90 * time.Second, LiveTurnWindow: 8},
			"level_up":          {Name: "level_up", MaxInputTokens: 110000, MaxOutputTokens: 1200, Timeout: 90 * time.Second, LiveTurnWindow: 8},
		},
	}
}

func withLLMRequestMeta(ctx context.Context, meta llmRequestMeta) context.Context {
	return context.WithValue(ctx, llmRequestMetaKey{}, meta)
}

func llmRequestMetaFromContext(ctx context.Context) llmRequestMeta {
	meta, _ := ctx.Value(llmRequestMetaKey{}).(llmRequestMeta)
	return meta
}

func (g *LLMGateway) limitFor(profile string, requestedOutput int) llmProfileLimit {
	limit, ok := g.profiles[strings.TrimSpace(profile)]
	if !ok {
		limit = llmProfileLimit{Name: firstNonEmpty(strings.TrimSpace(profile), "scene"), MaxInputTokens: 2400, MaxOutputTokens: 384, Timeout: 45 * time.Second, LiveTurnWindow: 8}
	}
	if requestedOutput > 0 && requestedOutput < limit.MaxOutputTokens {
		limit.MaxOutputTokens = requestedOutput
	}
	return limit
}

func estimatePromptTokens(messages []map[string]string) int {
	totalChars := 0
	for _, message := range messages {
		totalChars += len(strings.TrimSpace(message["role"]))
		totalChars += len(strings.TrimSpace(message["content"]))
	}
	return max(1, totalChars/4)
}

func (g *LLMGateway) run(ctx context.Context, messages []map[string]string, requestedOutput int, call func(context.Context) (string, string, error)) (string, string, error) {
	meta := llmRequestMetaFromContext(ctx)
	limit := g.limitFor(meta.Profile, requestedOutput)
	if meta.MaxInputTokens > 0 && meta.MaxInputTokens > limit.MaxInputTokens {
		limit.MaxInputTokens = meta.MaxInputTokens
	}
	if g.isCircuitOpen() {
		atomic.AddInt64(&g.rejected, 1)
		return "", "", errLLMCircuitOpen
	}
	if estimatePromptTokens(messages) > limit.MaxInputTokens {
		return "", "", errors.New("llm input budget exceeded")
	}
	if !g.tryAcquire() {
		atomic.AddInt64(&g.rejected, 1)
		return "", "", errLLMGatewayBusy
	}
	lockKey := ""
	if meta.ScopeType != "" && meta.ScopeID != "" {
		lockKey = "llm:session-lock:" + meta.ScopeType + ":" + meta.ScopeID
		if err := g.acquireSessionLock(ctx, lockKey, limit.Timeout+15*time.Second); err != nil {
			g.release()
			atomic.AddInt64(&g.rejected, 1)
			return "", "", errLLMGatewayBusy
		}
	}
	defer func() {
		if lockKey != "" {
			g.releaseSessionLock(context.Background(), lockKey)
		}
		g.release()
	}()

	callCtx, cancel := context.WithTimeout(ctx, limit.Timeout)
	defer cancel()
	content, model, err := call(callCtx)
	if err != nil {
		g.recordFailure(err)
		return "", "", err
	}
	g.recordSuccess()
	return content, model, nil
}

func (g *LLMGateway) tryAcquire() bool {
	select {
	case g.semaphore <- struct{}{}:
		g.mu.Lock()
		g.inflight++
		g.mu.Unlock()
		return true
	default:
		return false
	}
}

func (g *LLMGateway) release() {
	select {
	case <-g.semaphore:
	default:
	}
	g.mu.Lock()
	if g.inflight > 0 {
		g.inflight--
	}
	g.mu.Unlock()
}

func (g *LLMGateway) acquireSessionLock(ctx context.Context, key string, ttl time.Duration) error {
	if g.redis == nil {
		return nil
	}
	ok, err := g.redis.SetNX(ctx, key, "1", ttl).Result()
	if err != nil {
		return nil
	}
	if !ok {
		return errLLMGatewayBusy
	}
	return nil
}

func (g *LLMGateway) releaseSessionLock(ctx context.Context, key string) {
	if g.redis == nil || key == "" {
		return
	}
	_, _ = g.redis.Del(ctx, key).Result()
}

func (g *LLMGateway) teardownSession(ctx context.Context, scopeType string, scopeID string) {
	if g.redis == nil || scopeType == "" || scopeID == "" {
		return
	}
	pattern := "llm:session-lock:" + scopeType + ":" + scopeID
	_, _ = g.redis.Del(ctx, pattern).Result()
}

func (g *LLMGateway) isCircuitOpen() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return !g.breakerUntil.IsZero() && time.Now().Before(g.breakerUntil)
}

func (g *LLMGateway) recordFailure(err error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.consecutiveFailures++
	g.lastError = err.Error()
	if strings.Contains(strings.ToLower(err.Error()), "timeout") || strings.Contains(strings.ToLower(err.Error()), "context deadline") {
		atomic.AddInt64(&g.timeouts, 1)
	}
	if g.consecutiveFailures >= g.breakerThreshold {
		g.breakerUntil = time.Now().Add(g.breakerCooldown)
	}
}

func (g *LLMGateway) recordSuccess() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.consecutiveFailures = 0
	g.breakerUntil = time.Time{}
	g.lastError = ""
}

func (g *LLMGateway) Status(activeSessions int, archivedSessions int) LLMGatewayStatus {
	g.mu.Lock()
	defer g.mu.Unlock()
	profiles := make([]LLMGatewayProfileStatus, 0, len(g.profiles))
	for _, profile := range g.profiles {
		profiles = append(profiles, LLMGatewayProfileStatus{
			Name:            profile.Name,
			MaxInputTokens:  profile.MaxInputTokens,
			MaxOutputTokens: profile.MaxOutputTokens,
			TimeoutSeconds:  int(profile.Timeout / time.Second),
			LiveTurnWindow:  profile.LiveTurnWindow,
		})
	}
	status := "ok"
	if !g.breakerUntil.IsZero() && time.Now().Before(g.breakerUntil) {
		status = "degraded"
	}
	return LLMGatewayStatus{
		Status:                  status,
		InFlight:                g.inflight,
		MaxConcurrentRequests:   g.maxConcurrent,
		QueueLength:             0,
		CircuitBreakerOpen:      !g.breakerUntil.IsZero() && time.Now().Before(g.breakerUntil),
		CircuitBreakerUntil:     timePtrOrNil(g.breakerUntil),
		ConsecutiveFailures:     g.consecutiveFailures,
		LastError:               g.lastError,
		RejectedRequests:        atomic.LoadInt64(&g.rejected),
		TimeoutCount:            atomic.LoadInt64(&g.timeouts),
		ActiveGatewaySessions:   activeSessions,
		ArchivedGatewaySessions: archivedSessions,
		Profiles:                profiles,
	}
}

func timePtrOrNil(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copied := value
	return &copied
}
