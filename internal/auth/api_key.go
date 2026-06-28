package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"

	apperrors "hatesentry/internal/errors"

	"github.com/gin-gonic/gin"
)

const (
	APIKeyContextKey = "api_key_principal"
	apiKeyPrefix     = "hs_live_"
	apiKeyBytes      = 32
)

// APIKeyPrincipal is the authenticated external client identity.
type APIKeyPrincipal struct {
	ClientID uint
	UserID   uint
	Name     string
}

// APIKeyAuthenticator verifies external client API keys.
type APIKeyAuthenticator interface {
	AuthenticateAPIKey(ctx context.Context, key string) (APIKeyPrincipal, error)
}

// GenerateAPIKey creates a random API key for external client access.
func GenerateAPIKey() (string, error) {
	secret := make([]byte, apiKeyBytes)
	if _, err := rand.Read(secret); err != nil {
		return "", apperrors.Internal("failed to generate api key").WithDetails(err.Error())
	}

	return apiKeyPrefix + base64.RawURLEncoding.EncodeToString(secret), nil
}

// HashAPIKey returns a stable hash suitable for database lookup.
func HashAPIKey(key string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(key)))
	return hex.EncodeToString(sum[:])
}

// APIKeyPrefix returns a short non-secret prefix for identifying keys operationally.
func APIKeyPrefix(key string) string {
	key = strings.TrimSpace(key)
	if len(key) <= 12 {
		return key
	}

	return key[:12]
}

// APIKeyMiddleware authenticates requests using X-API-Key or Authorization: ApiKey.
func APIKeyMiddleware(authenticator APIKeyAuthenticator) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := extractAPIKey(c)
		if key == "" {
			apperrors.Handle(c, apperrors.Unauthorized("API key required"))
			c.Abort()
			return
		}
		if authenticator == nil {
			apperrors.Handle(c, apperrors.ConfigurationError("api key authentication is not configured"))
			c.Abort()
			return
		}

		principal, err := authenticator.AuthenticateAPIKey(c.Request.Context(), key)
		if err != nil {
			apperrors.Handle(c, err)
			c.Abort()
			return
		}

		c.Set(APIKeyContextKey, principal)
		c.Next()
	}
}

// AuthOrAPIKeyMiddleware authenticates with API key when present, otherwise falls back to JWT.
func (j *JWTManager) AuthOrAPIKeyMiddleware(authenticator APIKeyAuthenticator) gin.HandlerFunc {
	return func(c *gin.Context) {
		if extractAPIKey(c) != "" {
			APIKeyMiddleware(authenticator)(c)
			return
		}

		j.AuthMiddleware()(c)
	}
}

// GetAPIKeyPrincipal retrieves the API-key-authenticated client from context.
func GetAPIKeyPrincipal(c *gin.Context) (APIKeyPrincipal, bool) {
	value, exists := c.Get(APIKeyContextKey)
	if !exists {
		return APIKeyPrincipal{}, false
	}

	principal, ok := value.(APIKeyPrincipal)
	return principal, ok
}

func extractAPIKey(c *gin.Context) string {
	if key := strings.TrimSpace(c.GetHeader("X-API-Key")); key != "" {
		return key
	}

	authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) == 2 && parts[0] == "ApiKey" {
		return strings.TrimSpace(parts[1])
	}

	return ""
}
