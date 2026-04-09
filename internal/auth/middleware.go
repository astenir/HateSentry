package auth

import (
	"strings"

	apperrors "hatesentry/internal/errors"

	"github.com/gin-gonic/gin"
)

const (
	UserContextKey = "user"
)

// AuthMiddleware creates authentication middleware
func (j *JWTManager) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			apperrors.Handle(c, apperrors.Unauthorized("Authorization header required"))
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			apperrors.Handle(c, apperrors.Unauthorized("Invalid authorization header format"))
			c.Abort()
			return
		}

		claims, err := j.ValidateToken(parts[1])
		if err != nil {
			// Convert error to AppError if needed
			if appErr, ok := err.(*apperrors.AppError); ok {
				apperrors.Handle(c, appErr)
			} else {
				apperrors.Handle(c, apperrors.Internal("Authentication failed").WithDetails(err.Error()))
			}
			c.Abort()
			return
		}

		c.Set(UserContextKey, claims)
		c.Next()
	}
}

// GetClaims retrieves user claims from context
func GetClaims(c *gin.Context) (*Claims, bool) {
	claims, exists := c.Get(UserContextKey)
	if !exists {
		return nil, false
	}

	userClaims, ok := claims.(*Claims)
	return userClaims, ok
}

// RequireRole creates middleware to check user role
func (j *JWTManager) RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, exists := GetClaims(c)
		if !exists {
			apperrors.Handle(c, apperrors.Unauthorized("User not authenticated"))
			c.Abort()
			return
		}

		for _, role := range roles {
			if claims.Role == role {
				c.Next()
				return
			}
		}

		apperrors.Handle(c, apperrors.Forbidden("Insufficient permissions"))
		c.Abort()
	}
}
