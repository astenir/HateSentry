package router

import (
	"context"
	"errors"
	"hatesentry/internal/auth"
	"hatesentry/internal/config"
	"hatesentry/internal/moderation"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestSetupRegistersCoreRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	jwtManager := auth.NewJWTManager(&config.JWTConfig{
		Secret:      "test-secret",
		ExpireHours: 1,
		Issuer:      "hatesentry-test",
	})

	router := NewRouter(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		jwtManager,
		moderation.DefaultPolicy(),
		config.ModerationRateLimitConfig{},
	)

	engine := router.Setup()
	routes := registeredRoutes(engine)

	expectedRoutes := []string{
		"POST /api/v1/auth/register",
		"POST /api/v1/auth/login",
		"GET /api/v1/health",
		"POST /api/v1/auth/refresh",
		"GET /api/v1/auth/profile",
		"POST /api/v1/auth/api-key/regenerate",
		"POST /api/v1/detection/detect",
		"GET /api/v1/detection/result/:id",
		"GET /api/v1/detection/history",
		"POST /api/v1/moderation/check",
		"GET /api/v1/moderation/results/:request_id",
		"GET /api/v1/reviews",
		"GET /api/v1/reviews/stats",
		"GET /api/v1/reviews/:id",
		"POST /api/v1/reviews/:id/approve",
		"POST /api/v1/reviews/:id/reject",
		"POST /api/v1/reviews/:id/mark-mistake",
		"POST /api/v1/admin/clients",
		"GET /api/v1/admin/clients",
		"POST /api/v1/admin/clients/:id/activate",
		"POST /api/v1/admin/clients/:id/deactivate",
		"POST /api/v1/admin/clients/:id/api-key/rotate",
		"GET /api/v1/admin/moderation/results",
		"GET /api/v1/admin/webhook-deliveries",
		"POST /api/v1/admin/webhook-deliveries/:id/retry",
		"GET /metrics",
	}

	for _, route := range expectedRoutes {
		t.Run(route, func(t *testing.T) {
			if !routes[route] {
				t.Fatalf("route %q is not registered", route)
			}
		})
	}
}

func TestSetupProtectsModerationResultRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	jwtManager := auth.NewJWTManager(&config.JWTConfig{
		Secret:      "test-secret",
		ExpireHours: 1,
		Issuer:      "hatesentry-test",
	})

	router := NewRouter(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		jwtManager,
		moderation.DefaultPolicy(),
		config.ModerationRateLimitConfig{},
	)

	engine := router.Setup()
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/moderation/results/request-123",
		nil,
	)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/reviews/3", nil)
	recorder = httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("detail status = %d, want 401", recorder.Code)
	}
}

func TestSetupProtectsModerationCheckRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	jwtManager := auth.NewJWTManager(&config.JWTConfig{
		Secret:      "test-secret",
		ExpireHours: 1,
		Issuer:      "hatesentry-test",
	})

	router := NewRouter(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		jwtManager,
		moderation.DefaultPolicy(),
		config.ModerationRateLimitConfig{},
	)

	engine := router.Setup()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/moderation/check", nil)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}

func TestSetupProtectsReviewRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	jwtManager := auth.NewJWTManager(&config.JWTConfig{
		Secret:      "test-secret",
		ExpireHours: 1,
		Issuer:      "hatesentry-test",
	})

	router := NewRouter(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		jwtManager,
		moderation.DefaultPolicy(),
		config.ModerationRateLimitConfig{},
	)

	engine := router.Setup()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/reviews", nil)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}

func TestSetupProtectsReviewStatsRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	jwtManager := auth.NewJWTManager(&config.JWTConfig{
		Secret:      "test-secret",
		ExpireHours: 1,
		Issuer:      "hatesentry-test",
	})

	router := NewRouter(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		jwtManager,
		moderation.DefaultPolicy(),
		config.ModerationRateLimitConfig{},
	)

	engine := router.Setup()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/reviews/stats", nil)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}

func TestSetupRequiresAdminForReviewRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	jwtManager := auth.NewJWTManager(&config.JWTConfig{
		Secret:      "test-secret",
		ExpireHours: 1,
		Issuer:      "hatesentry-test",
	})
	token, err := jwtManager.GenerateToken(7, "submitter", "user")
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	router := NewRouter(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		jwtManager,
		moderation.DefaultPolicy(),
		config.ModerationRateLimitConfig{},
	)

	engine := router.Setup()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/reviews", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", recorder.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/reviews/stats", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder = httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("stats status = %d, want 403", recorder.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/reviews/3", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder = httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("detail status = %d, want 403", recorder.Code)
	}
}

func TestSetupRequiresAdminForClientRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	jwtManager := auth.NewJWTManager(&config.JWTConfig{
		Secret:      "test-secret",
		ExpireHours: 1,
		Issuer:      "hatesentry-test",
	})
	token, err := jwtManager.GenerateToken(7, "submitter", "user")
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	router := NewRouter(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		jwtManager,
		moderation.DefaultPolicy(),
		config.ModerationRateLimitConfig{},
	)

	engine := router.Setup()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", recorder.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/clients/11/deactivate", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder = httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("client deactivate status = %d, want 403", recorder.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/clients/11/activate", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder = httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("client activate status = %d, want 403", recorder.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/clients/11/api-key/rotate", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder = httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("client api key rotate status = %d, want 403", recorder.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/moderation/results", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder = httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("moderation history status = %d, want 403", recorder.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/webhook-deliveries/5/retry", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder = httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("webhook retry status = %d, want 403", recorder.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/webhook-deliveries", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder = httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("webhook delivery list status = %d, want 403", recorder.Code)
	}
}

func TestCORSAllowsAPIKeyHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(corsMiddleware())
	engine.OPTIONS("/preflight", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodOptions, "/preflight", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "X-API-Key")
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", recorder.Code)
	}
	allowedHeaders := recorder.Header().Get("Access-Control-Allow-Headers")
	if !strings.Contains(allowedHeaders, "X-API-Key") {
		t.Fatalf("Access-Control-Allow-Headers = %q, want X-API-Key", allowedHeaders)
	}
}

func TestClientRateLimitMiddleware(t *testing.T) {
	tests := []struct {
		name       string
		principal  *auth.APIKeyPrincipal
		allowed    bool
		limitError error
		wantStatus int
		wantCalled bool
	}{
		{
			name: "allows api key client within limit",
			principal: &auth.APIKeyPrincipal{
				ClientID: 42,
				UserID:   7,
				Name:     "comments",
			},
			allowed:    true,
			wantStatus: http.StatusOK,
			wantCalled: true,
		},
		{
			name: "rejects api key client over limit",
			principal: &auth.APIKeyPrincipal{
				ClientID: 42,
				UserID:   7,
				Name:     "comments",
			},
			allowed:    false,
			wantStatus: http.StatusTooManyRequests,
			wantCalled: true,
		},
		{
			name:       "skips non api key requests",
			allowed:    false,
			wantStatus: http.StatusOK,
			wantCalled: false,
		},
		{
			name: "fails closed on limiter error",
			principal: &auth.APIKeyPrincipal{
				ClientID: 42,
				UserID:   7,
				Name:     "comments",
			},
			allowed:    true,
			limitError: errors.New("redis unavailable"),
			wantStatus: http.StatusInternalServerError,
			wantCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)

			limiter := &fakeRequestRateLimiter{allowed: tt.allowed, err: tt.limitError}
			engine := gin.New()
			engine.Use(func(c *gin.Context) {
				if tt.principal != nil {
					c.Set(auth.APIKeyContextKey, *tt.principal)
				}
				c.Next()
			})
			engine.Use(clientRateLimitMiddleware(
				limiter,
				config.ModerationRateLimitConfig{
					Limit:  3,
					Window: time.Minute,
				},
			))
			engine.POST("/check", func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodPost, "/check", nil)
			recorder := httptest.NewRecorder()

			engine.ServeHTTP(recorder, req)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", recorder.Code, tt.wantStatus, recorder.Body.String())
			}
			if (limiter.calls > 0) != tt.wantCalled {
				t.Fatalf("limiter called = %v, want %v", limiter.calls > 0, tt.wantCalled)
			}
			if !tt.wantCalled {
				return
			}
			if limiter.key != "moderation:client:42" {
				t.Fatalf("limiter key = %q, want moderation:client:42", limiter.key)
			}
			if limiter.limit != 3 {
				t.Fatalf("limiter limit = %d, want 3", limiter.limit)
			}
			if limiter.window != time.Minute {
				t.Fatalf("limiter window = %s, want 1m", limiter.window)
			}
		})
	}
}

func registeredRoutes(engine *gin.Engine) map[string]bool {
	routes := make(map[string]bool, len(engine.Routes()))

	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = true
	}

	return routes
}

type fakeRequestRateLimiter struct {
	allowed bool
	err     error
	calls   int
	key     string
	limit   int
	window  time.Duration
}

func (f *fakeRequestRateLimiter) Allow(
	ctx context.Context,
	key string,
	limit int,
	window time.Duration,
) (bool, error) {
	f.calls++
	f.key = key
	f.limit = limit
	f.window = window

	if f.err != nil {
		return false, f.err
	}

	return f.allowed, nil
}
