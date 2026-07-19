package httpapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRequireOperatorMiddlewareRejectsMissingSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(requireOperatorMiddleware("top-secret"))
	engine.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRequireOperatorMiddlewareAcceptsBearerToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(requireOperatorMiddleware("top-secret"))
	engine.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer top-secret")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestCORSMiddlewareAllowsConfiguredOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(corsMiddleware([]string{"http://localhost:13005"}))
	engine.GET("/ok", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodOptions, "/ok", nil)
	req.Header.Set("Origin", "http://localhost:13005")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:13005" {
		t.Fatalf("expected allowed origin header, got %q", got)
	}
}

func TestCORSMiddlewareRejectsUnknownOriginPreflight(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(corsMiddleware([]string{"http://localhost:13005"}))
	engine.GET("/ok", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodOptions, "/ok", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestRateLimitMiddlewareRejectsAfterLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.SetTrustedProxies([]string{"0.0.0.0/0"})
	limiter := newFixedWindowRateLimiter()
	engine.Use(rateLimitMiddleware(limiter, "gm-respond", 2, time.Minute))
	engine.GET("/limited", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/limited", nil)
		req.RemoteAddr = "203.0.113.10:1234"
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected request %d to pass, got %d", i+1, rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/limited", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}
	if got := rec.Header().Get("X-RateLimit-Limit"); got != "2" {
		t.Fatalf("expected X-RateLimit-Limit=2, got %q", got)
	}
}

func TestJSONBodyLimitMiddlewareRejectsOversizedJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(jsonBodyLimitMiddleware(16))
	engine.POST("/json", func(c *gin.Context) {
		var payload map[string]any
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/json", bytes.NewBufferString(`{"message":"this is too long"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestRedactSensitiveTextMasksTokens(t *testing.T) {
	input := `Authorization: Bearer abc123 token=foo sk-proj-secret join_token=xyz`
	redacted := redactSensitiveText(input)
	for _, forbidden := range []string{"abc123", "foo", "sk-proj-secret", "xyz"} {
		if bytes.Contains([]byte(redacted), []byte(forbidden)) {
			t.Fatalf("expected %q to be redacted from %q", forbidden, redacted)
		}
	}
}
