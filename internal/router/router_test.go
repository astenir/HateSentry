package router

import (
	"hatesentry/internal/auth"
	"hatesentry/internal/config"
	"hatesentry/internal/moderation"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
		"POST /api/v1/reviews/:id/approve",
		"POST /api/v1/reviews/:id/reject",
		"POST /api/v1/reviews/:id/mark-mistake",
		"POST /api/v1/admin/clients",
		"GET /api/v1/admin/clients",
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
	)

	engine := router.Setup()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/reviews", nil)
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
	)

	engine := router.Setup()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/reviews", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", recorder.Code)
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
	)

	engine := router.Setup()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clients", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", recorder.Code)
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

func registeredRoutes(engine *gin.Engine) map[string]bool {
	routes := make(map[string]bool, len(engine.Routes()))

	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = true
	}

	return routes
}
