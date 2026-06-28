package router

import (
	"hatesentry/internal/auth"
	"hatesentry/internal/config"
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

func registeredRoutes(engine *gin.Engine) map[string]bool {
	routes := make(map[string]bool, len(engine.Routes()))

	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = true
	}

	return routes
}
