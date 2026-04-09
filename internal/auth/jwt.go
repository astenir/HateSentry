package auth

import (
	"errors"
	"fmt"
	"hatesentry/internal/config"
	apperrors "hatesentry/internal/errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims represents JWT claims
type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// JWTManager manages JWT token operations
type JWTManager struct {
	secret     string
	expireTime time.Duration
	issuer     string
}

// NewJWTManager creates a new JWT manager
func NewJWTManager(cfg *config.JWTConfig) *JWTManager {
	return &JWTManager{
		secret:     cfg.Secret,
		expireTime: time.Duration(cfg.ExpireHours) * time.Hour,
		issuer:     cfg.Issuer,
	}
}

// GenerateToken generates a JWT token for a user
func (j *JWTManager) GenerateToken(userID uint, username, role string) (string, error) {
	claims := Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(j.expireTime)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    j.issuer,
			Subject:   fmt.Sprintf("%d", userID),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(j.secret))
	if err != nil {
		return "", apperrors.Internal("Failed to generate token").WithDetails(err.Error())
	}
	return signedToken, nil
}

// ValidateToken validates a JWT token and returns to claims
func (j *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, apperrors.InvalidToken("Unexpected signing method").WithDetails(fmt.Sprintf("%v", token.Header["alg"]))
		}
		return []byte(j.secret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, apperrors.ExpiredToken("Token has expired")
		}
		return nil, apperrors.InvalidToken("Invalid token").WithDetails(err.Error())
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, apperrors.InvalidToken("Invalid token claims")
	}

	return claims, nil
}

// RefreshToken refreshes a token if it's still valid
func (j *JWTManager) RefreshToken(tokenString string) (string, error) {
	claims, err := j.ValidateToken(tokenString)
	if err != nil {
		return "", err
	}

	// Check if token is within refresh window (e.g., 1 hour before expiration)
	if time.Until(claims.ExpiresAt.Time) > j.expireTime-time.Hour {
		return "", apperrors.BadRequest("Token is still valid, no need to refresh")
	}

	return j.GenerateToken(claims.UserID, claims.Username, claims.Role)
}
