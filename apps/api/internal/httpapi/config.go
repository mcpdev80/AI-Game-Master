package httpapi

import (
	"os"
	"strconv"
)

type Config struct {
	Host                      string
	Port                      string
	Env                       string
	DatabaseURL               string
	RedisURL                  string
	LLMBaseURL                string
	LLMModel                  string
	LLMAPIKey                 string
	TTSBaseURL                string
	TTSModel                  string
	STTBaseURL                string
	STTModel                  string
	VisionBaseURL             string
	UploadsDir                string
	LLMMaxConcurrent          int
	LLMBreakerThreshold       int
	LLMBreakerCooldownSeconds int
}

func LoadConfig() Config {
	return Config{
		Host:                      envOrDefault("API_HOST", "0.0.0.0"),
		Port:                      envOrDefault("API_PORT", "8080"),
		Env:                       envOrDefault("APP_ENV", "development"),
		DatabaseURL:               envOrDefault("DATABASE_URL", "postgres://dungeon:dungeon@postgres:5432/dungeon_master?sslmode=disable"),
		RedisURL:                  envOrDefault("REDIS_URL", "redis://redis:6379/0"),
		LLMBaseURL:                envOrDefault("LLM_BASE_URL", "http://host.docker.internal:11434/v1"),
		LLMModel:                  envOrDefault("LLM_MODEL", "local-model"),
		LLMAPIKey:                 os.Getenv("LLM_API_KEY"),
		TTSBaseURL:                envOrDefault("TTS_BASE_URL", "http://dungeon-master-speech-tts:8091/v1"),
		TTSModel:                  envOrDefault("TTS_MODEL", "piper"),
		STTBaseURL:                envOrDefault("STT_BASE_URL", "http://dungeon-master-speech-stt:8092/v1"),
		STTModel:                  envOrDefault("STT_MODEL", "nvidia/parakeet-tdt-0.6b-v3"),
		VisionBaseURL:             envOrDefault("VISION_BASE_URL", "http://vision:8090"),
		UploadsDir:                envOrDefault("UPLOADS_DIR", "/tmp/data/uploads"),
		LLMMaxConcurrent:          envIntOrDefault("LLM_MAX_CONCURRENT_REQUESTS", 2),
		LLMBreakerThreshold:       envIntOrDefault("LLM_BREAKER_THRESHOLD", 3),
		LLMBreakerCooldownSeconds: envIntOrDefault("LLM_BREAKER_COOLDOWN_SECONDS", 45),
	}
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
