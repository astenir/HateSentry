package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apperrors "hatesentry/internal/errors"

	"github.com/gin-gonic/gin"
)

func TestGenerateAPIKeyAndHash(t *testing.T) {
	key, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey() error = %v", err)
	}
	if !strings.HasPrefix(key, apiKeyPrefix) {
		t.Fatalf("key = %q, want prefix %q", key, apiKeyPrefix)
	}

	hash := HashAPIKey(key)
	if hash == "" {
		t.Fatal("HashAPIKey() returned empty hash")
	}
	if strings.Contains(hash, key) {
		t.Fatal("HashAPIKey() contains raw key")
	}
	if HashAPIKey(" "+key+" ") != hash {
		t.Fatal("HashAPIKey() should trim surrounding whitespace")
	}
}

func TestAPIKeyMiddlewareSetsPrincipal(t *testing.T) {
	gin.SetMode(gin.TestMode)

	authenticator := &fakeAPIKeyAuthenticator{
		principal: APIKeyPrincipal{
			ClientID: 11,
			UserID:   42,
			Name:     "blog",
		},
	}
	engine := gin.New()
	engine.Use(APIKeyMiddleware(authenticator))
	engine.GET("/protected", func(c *gin.Context) {
		principal, exists := GetAPIKeyPrincipal(c)
		if !exists {
			t.Fatal("api key principal was not set")
		}
		c.JSON(http.StatusOK, principal)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("X-API-Key", "test-key")
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if authenticator.key != "test-key" {
		t.Fatalf("authenticator key = %q, want test-key", authenticator.key)
	}
}

func TestAPIKeyMiddlewareRequiresKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(APIKeyMiddleware(&fakeAPIKeyAuthenticator{}))
	engine.GET("/protected", func(c *gin.Context) {
		t.Fatal("handler should not be called")
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}

type fakeAPIKeyAuthenticator struct {
	principal APIKeyPrincipal
	key       string
	err       error
}

func (a *fakeAPIKeyAuthenticator) AuthenticateAPIKey(ctx context.Context, key string) (APIKeyPrincipal, error) {
	a.key = key
	if a.err != nil {
		return APIKeyPrincipal{}, a.err
	}
	if a.principal.ClientID == 0 {
		return APIKeyPrincipal{}, apperrors.Unauthorized("Invalid API key")
	}

	return a.principal, nil
}
