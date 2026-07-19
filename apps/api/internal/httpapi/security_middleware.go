package httpapi

import (
	"crypto/subtle"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type rateWindow struct {
	start time.Time
	count int
}

type fixedWindowRateLimiter struct {
	mu      sync.Mutex
	windows map[string]rateWindow
}

var sensitiveTextPatterns = []struct {
	pattern *regexp.Regexp
	replace string
}{
	{pattern: regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._-]+`), replace: "Bearer [REDACTED]"},
	{pattern: regexp.MustCompile(`sk-[A-Za-z0-9_-]+`), replace: "[REDACTED_API_KEY]"},
	{pattern: regexp.MustCompile(`(?i)(token=)[A-Za-z0-9._-]+`), replace: "${1}[REDACTED]"},
	{pattern: regexp.MustCompile(`(?i)(authorization\s*[:=]\s*)(?:bearer\s+)?[A-Za-z0-9._-]+`), replace: "${1}[REDACTED]"},
	{pattern: regexp.MustCompile(`(?i)(x-operator-secret\s*[:=]\s*)[A-Za-z0-9._-]+`), replace: "${1}[REDACTED]"},
	{pattern: regexp.MustCompile(`(?i)((?:join|portal|session)?_?token\s*[:=]\s*)[A-Za-z0-9._-]+`), replace: "${1}[REDACTED]"},
}

func newFixedWindowRateLimiter() *fixedWindowRateLimiter {
	return &fixedWindowRateLimiter{windows: make(map[string]rateWindow)}
}

func (l *fixedWindowRateLimiter) allow(key string, limit int, window time.Duration) (bool, int, time.Time) {
	if limit <= 0 {
		return true, 0, time.Now().UTC().Add(window)
	}

	now := time.Now().UTC()
	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.windows[key]
	if !ok || now.Sub(entry.start) >= window {
		entry = rateWindow{start: now, count: 0}
	}
	if entry.count >= limit {
		return false, 0, entry.start.Add(window)
	}

	entry.count++
	l.windows[key] = entry
	remaining := limit - entry.count
	if remaining < 0 {
		remaining = 0
	}
	return true, remaining, entry.start.Add(window)
}

func requireOperatorMiddleware(secret string) gin.HandlerFunc {
	trimmedSecret := strings.TrimSpace(secret)
	if trimmedSecret == "" {
		return func(c *gin.Context) { c.Next() }
	}

	return func(c *gin.Context) {
		provided := strings.TrimSpace(c.GetHeader("X-Operator-Secret"))
		if provided == "" {
			authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
			if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
				provided = strings.TrimSpace(authHeader[7:])
			}
		}

		if subtle.ConstantTimeCompare([]byte(provided), []byte(trimmedSecret)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "operator authorization required"})
			return
		}
		c.Next()
	}
}

func corsMiddleware(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		trimmed := strings.TrimSpace(origin)
		if trimmed != "" {
			allowed[trimmed] = struct{}{}
		}
	}

	return func(c *gin.Context) {
		origin := strings.TrimSpace(c.GetHeader("Origin"))
		c.Writer.Header().Set("Vary", "Origin")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Operator-Secret")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")

		if origin != "" {
			if _, ok := allowed[origin]; ok {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			} else if c.Request.Method == http.MethodOptions {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "origin not allowed"})
				return
			}
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func jsonBodyLimitMiddleware(maxBytes int64) gin.HandlerFunc {
	if maxBytes <= 0 {
		return func(c *gin.Context) { c.Next() }
	}

	return func(c *gin.Context) {
		if c.Request == nil || c.Request.Body == nil {
			c.Next()
			return
		}
		switch c.Request.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch:
		default:
			c.Next()
			return
		}
		contentType := strings.ToLower(strings.TrimSpace(c.ContentType()))
		if !strings.HasPrefix(contentType, "application/json") {
			c.Next()
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}

func rateLimitMiddleware(limiter *fixedWindowRateLimiter, scope string, limit int, window time.Duration) gin.HandlerFunc {
	if limit <= 0 {
		return func(c *gin.Context) { c.Next() }
	}

	return func(c *gin.Context) {
		clientIP := strings.TrimSpace(c.ClientIP())
		if clientIP == "" {
			clientIP = "unknown"
		}
		key := scope + ":" + clientIP
		allowed, remaining, resetAt := limiter.allow(key, limit, window)
		c.Writer.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Writer.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Writer.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetAt.Unix(), 10))
		if !allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}

func redactSensitiveText(input string) string {
	output := strings.TrimSpace(input)
	for _, item := range sensitiveTextPatterns {
		output = item.pattern.ReplaceAllString(output, item.replace)
	}
	if len(output) > 320 {
		output = output[:320] + "... [truncated]"
	}
	return output
}
