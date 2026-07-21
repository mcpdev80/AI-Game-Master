package httpapi

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Host                      string
	Port                      string
	Env                       string
	DatabaseURL               string
	RedisURL                  string
	DemoOperatorSecret        string
	CORSAllowedOrigins        []string
	TrustedProxies            []string
	LLMProvider               string
	LLMBaseURL                string
	LLMModel                  string
	LLMAPIKey                 string
	LLMReasoningEffort        string
	LLMStoreResponses         bool
	TTSProvider               string
	TTSBaseURL                string
	TTSModel                  string
	TTSAPIKey                 string
	TTSVoice                  string
	STTProvider               string
	STTBaseURL                string
	STTModel                  string
	STTAPIKey                 string
	STTPrompt                 string
	VisionBaseURL             string
	UploadsDir                string
	MaxJSONBodyBytes          int64
	MaxUploadBytes            int64
	MaxAudioUploadBytes       int64
	MaxZipUploadBytes         int64
	MaxZipEntries             int
	MaxZipExtractBytes        int64
	LLMMaxConcurrent          int
	LLMBreakerThreshold       int
	LLMBreakerCooldownSeconds int
	PublicRateLimitWindowSecs int
	RateLimitDemoSeed         int
	RateLimitGMRespond        int
	RateLimitSTT              int
	RateLimitVision           int
	RateLimitBuilder          int
	OpenAIBudgetSoftLimitUSD  float64
	OpenAIBudgetHardLimitUSD  float64
	OpenAIUsageAlertEmail     string
}

func LoadConfig() Config {
	provider := envOrDefault("LLM_PROVIDER", "openai")
	llmBaseURL := envOrDefault("LLM_BASE_URL", "http://host.docker.internal:11434/v1")
	llmModel := envOrDefault("LLM_MODEL", "local-model")
	llmAPIKey := os.Getenv("LLM_API_KEY")
	if provider == "openai" {
		llmBaseURL = envOrDefault("OPENAI_BASE_URL", "https://api.openai.com/v1")
		llmModel = envOrDefault("OPENAI_MODEL", "gpt-5.6")
		llmAPIKey = os.Getenv("OPENAI_API_KEY")
	}
	ttsProvider := strings.ToLower(strings.TrimSpace(envOrDefault("TTS_PROVIDER", "openai")))
	ttsBaseURL := envOrDefault("TTS_BASE_URL", "http://host.docker.internal:8091/v1")
	ttsModel := envOrDefault("TTS_MODEL", "piper")
	ttsAPIKey := os.Getenv("TTS_API_KEY")
	ttsVoice := envOrDefault("TTS_VOICE", "narrator-default")
	if ttsProvider == "openai" {
		ttsBaseURL = envOrDefault("OPENAI_BASE_URL", "https://api.openai.com/v1")
		ttsModel = envOrDefault("OPENAI_TTS_MODEL", "gpt-4o-mini-tts")
		ttsAPIKey = os.Getenv("OPENAI_API_KEY")
		ttsVoice = envOrDefault("OPENAI_TTS_VOICE", "cedar")
	}
	sttProvider := strings.ToLower(strings.TrimSpace(envOrDefault("STT_PROVIDER", "openai")))
	sttBaseURL := envOrDefault("STT_BASE_URL", "http://host.docker.internal:8092/v1")
	sttModel := envOrDefault("STT_MODEL", "nvidia/parakeet-tdt-0.6b-v3")
	sttAPIKey := os.Getenv("STT_API_KEY")
	if sttProvider == "openai" {
		sttBaseURL = envOrDefault("OPENAI_BASE_URL", "https://api.openai.com/v1")
		sttModel = envOrDefault("OPENAI_STT_MODEL", "gpt-4o-transcribe")
		sttAPIKey = os.Getenv("OPENAI_API_KEY")
	}
	return Config{
		Host:                      envOrDefault("API_HOST", "0.0.0.0"),
		Port:                      envOrDefault("API_PORT", "8080"),
		Env:                       envOrDefault("APP_ENV", "development"),
		DatabaseURL:               envOrDefault("DATABASE_URL", "postgres://dungeon:dungeon@postgres:5432/dungeon_master?sslmode=disable"),
		RedisURL:                  envOrDefault("REDIS_URL", "redis://redis:6379/0"),
		DemoOperatorSecret:        os.Getenv("DEMO_OPERATOR_SECRET"),
		CORSAllowedOrigins:        envCSVOrDefault("CORS_ALLOWED_ORIGINS", []string{"http://localhost:3005", "http://127.0.0.1:3005", "http://localhost:13005", "http://127.0.0.1:13005"}),
		TrustedProxies:            envCSVOrDefault("TRUSTED_PROXIES", []string{"127.0.0.1", "::1"}),
		LLMProvider:               provider,
		LLMBaseURL:                llmBaseURL,
		LLMModel:                  llmModel,
		LLMAPIKey:                 llmAPIKey,
		LLMReasoningEffort:        envOrDefault("OPENAI_REASONING_EFFORT", "medium"),
		LLMStoreResponses:         envBoolOrDefault("OPENAI_STORE", false),
		TTSProvider:               ttsProvider,
		TTSBaseURL:                ttsBaseURL,
		TTSModel:                  ttsModel,
		TTSAPIKey:                 ttsAPIKey,
		TTSVoice:                  ttsVoice,
		STTProvider:               sttProvider,
		STTBaseURL:                sttBaseURL,
		STTModel:                  sttModel,
		STTAPIKey:                 sttAPIKey,
		STTPrompt:                 envOrDefault("OPENAI_STT_PROMPT", "Tabletop role-playing game session. Preserve character names, fantasy terms, dice notation such as d20, and natural punctuation."),
		VisionBaseURL:             envOrDefault("VISION_BASE_URL", "http://vision:8090"),
		UploadsDir:                envOrDefault("UPLOADS_DIR", "/tmp/data/uploads"),
		MaxJSONBodyBytes:          envInt64OrDefault("MAX_JSON_BODY_BYTES", 1<<20),
		MaxUploadBytes:            envInt64OrDefault("MAX_UPLOAD_BYTES", 25<<20),
		MaxAudioUploadBytes:       envInt64OrDefault("MAX_AUDIO_UPLOAD_BYTES", 12<<20),
		MaxZipUploadBytes:         envInt64OrDefault("MAX_ZIP_UPLOAD_BYTES", 500<<20),
		MaxZipEntries:             envIntOrDefault("MAX_ZIP_ENTRIES", 200),
		MaxZipExtractBytes:        envInt64OrDefault("MAX_ZIP_EXTRACT_BYTES", 1<<30),
		LLMMaxConcurrent:          envIntOrDefault("LLM_MAX_CONCURRENT_REQUESTS", 2),
		LLMBreakerThreshold:       envIntOrDefault("LLM_BREAKER_THRESHOLD", 3),
		LLMBreakerCooldownSeconds: envIntOrDefault("LLM_BREAKER_COOLDOWN_SECONDS", 45),
		PublicRateLimitWindowSecs: envIntOrDefault("PUBLIC_RATE_LIMIT_WINDOW_SECONDS", 60),
		RateLimitDemoSeed:         envIntOrDefault("RATE_LIMIT_DEMO_SEED", 6),
		RateLimitGMRespond:        envIntOrDefault("RATE_LIMIT_GM_RESPOND", 30),
		RateLimitSTT:              envIntOrDefault("RATE_LIMIT_STT", 20),
		RateLimitVision:           envIntOrDefault("RATE_LIMIT_VISION", 30),
		RateLimitBuilder:          envIntOrDefault("RATE_LIMIT_CHARACTER_BUILDER", 20),
		OpenAIBudgetSoftLimitUSD:  envFloat64OrDefault("OPENAI_BUDGET_SOFT_LIMIT_USD", 0),
		OpenAIBudgetHardLimitUSD:  envFloat64OrDefault("OPENAI_BUDGET_HARD_LIMIT_USD", 0),
		OpenAIUsageAlertEmail:     strings.TrimSpace(os.Getenv("OPENAI_USAGE_ALERT_EMAIL")),
	}
}

func envBoolOrDefault(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func (c Config) Address() string {
	return c.Host + ":" + c.Port
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}

func envIntOrDefault(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt64OrDefault(key string, fallback int64) int64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func envCSVOrDefault(key string, fallback []string) []string {
	value := os.Getenv(key)
	if strings.TrimSpace(value) == "" {
		return append([]string(nil), fallback...)
	}
	items := strings.Split(value, ",")
	result := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	if len(result) == 0 {
		return append([]string(nil), fallback...)
	}
	return result
}

func envFloat64OrDefault(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}
