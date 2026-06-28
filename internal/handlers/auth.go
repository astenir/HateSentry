package handlers

import (
	"context"
	"crypto/subtle"
	stderrors "errors"
	"hatesentry/internal/auth"
	"hatesentry/internal/config"
	apperrors "hatesentry/internal/errors"
	"hatesentry/internal/models"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const registrationBootstrapLockName = "auth:registration-bootstrap"

// AuthHandler handles authentication requests
type AuthHandler struct {
	db                  *gorm.DB
	jwtManager          *auth.JWTManager
	adminBootstrapToken string
	registrationMu      sync.Mutex
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(db *gorm.DB, jwtManager *auth.JWTManager) *AuthHandler {
	return NewAuthHandlerWithConfig(db, jwtManager, config.AuthConfig{})
}

// NewAuthHandlerWithConfig creates a new auth handler with auth configuration.
func NewAuthHandlerWithConfig(
	db *gorm.DB,
	jwtManager *auth.JWTManager,
	authConfig config.AuthConfig,
) *AuthHandler {
	return &AuthHandler{
		db:                  db,
		jwtManager:          jwtManager,
		adminBootstrapToken: authConfig.AdminBootstrapToken,
	}
}

// RegisterRequest represents registration request
type RegisterRequest struct {
	Username            string `json:"username" binding:"required,min=3,max=50"`
	Email               string `json:"email" binding:"required,email"`
	Password            string `json:"password" binding:"required,min=8"`
	AdminBootstrapToken string `json:"admin_bootstrap_token,omitempty"`
}

// LoginRequest represents login request
type RegisterResponse struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	APIKey   string `json:"api_key"`
}

// LoginRequest represents login request
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse represents login response
type LoginResponse struct {
	Token string   `json:"token"`
	User  UserInfo `json:"user"`
}

// UserInfo represents user information
type UserInfo struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
}

// Register handles user registration
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apperrors.RespondWithError(c, apperrors.ValidationError(err.Error()))
		return
	}

	// The process mutex avoids duplicate local work. The database lock inside
	// createRegisteredUser serializes the role decision across API processes.
	h.registrationMu.Lock()
	defer h.registrationMu.Unlock()

	user, appErr := h.createRegisteredUser(c.Request.Context(), req)
	if appErr != nil {
		apperrors.RespondWithError(c, appErr)
		return
	}

	// Generate JWT token
	token, err := h.jwtManager.GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		apperrors.RespondWithError(c, err.(*apperrors.AppError))
		return
	}

	c.JSON(http.StatusCreated, LoginResponse{
		Token: token,
		User: UserInfo{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
			Role:     user.Role,
		},
	})
}

// Login handles user login
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apperrors.RespondWithError(c, apperrors.ValidationError(err.Error()))
		return
	}

	// Find user
	var user models.User
	if err := h.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			apperrors.RespondWithError(c, apperrors.InvalidCredentials("Invalid email or password"))
		} else {
			apperrors.RespondWithError(c, apperrors.DatabaseError(err, "Failed to find user"))
		}
		return
	}

	// Check password
	if !auth.CheckPassword(req.Password, user.Password) {
		apperrors.RespondWithError(c, apperrors.InvalidCredentials("Invalid email or password"))
		return
	}

	// Check user status
	if user.Status != "active" {
		apperrors.RespondWithError(c, apperrors.Forbidden("Account is not active"))
		return
	}

	// Generate JWT token
	token, err := h.jwtManager.GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		apperrors.RespondWithError(c, err.(*apperrors.AppError))
		return
	}

	c.JSON(http.StatusOK, LoginResponse{
		Token: token,
		User: UserInfo{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
			Role:     user.Role,
		},
	})
}

// RefreshToken handles token refresh
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		apperrors.RespondWithError(c, apperrors.Unauthorized("Authorization header required"))
		return
	}

	if len(authHeader) < 7 {
		apperrors.RespondWithError(c, apperrors.Unauthorized("Invalid authorization header format"))
		return
	}

	token := authHeader[7:] // Remove "Bearer " prefix

	newToken, err := h.jwtManager.RefreshToken(token)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			apperrors.RespondWithError(c, appErr)
		} else {
			apperrors.RespondWithError(c, apperrors.Unauthorized(err.Error()))
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": newToken})
}

// GetProfile returns the authenticated user's profile
func (h *AuthHandler) GetProfile(c *gin.Context) {
	claims, exists := auth.GetClaims(c)
	if !exists {
		apperrors.RespondWithError(c, apperrors.Unauthorized("User not authenticated"))
		return
	}

	var user models.User
	if err := h.db.Where("id = ?", claims.UserID).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			apperrors.RespondWithError(c, apperrors.RecordNotFound("User not found"))
		} else {
			apperrors.RespondWithError(c, apperrors.DatabaseError(err, "Failed to find user"))
		}
		return
	}

	c.JSON(http.StatusOK, UserInfo{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		Role:     user.Role,
	})
}

// RegenerateAPIKey regenerates the user's API key
func (h *AuthHandler) RegenerateAPIKey(c *gin.Context) {
	claims, exists := auth.GetClaims(c)
	if !exists {
		apperrors.RespondWithError(c, apperrors.Unauthorized("User not authenticated"))
		return
	}

	newAPIKey := generateAPIKey()
	if err := h.db.Model(&models.User{}).Where("id = ?", claims.UserID).Update("api_key", newAPIKey).Error; err != nil {
		apperrors.RespondWithError(c, apperrors.DatabaseError(err, "Failed to regenerate API key"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"api_key": newAPIKey})
}

func generateAPIKey() string {
	return "hs_" + uuid.New().String()
}

func (h *AuthHandler) createRegisteredUser(
	ctx context.Context,
	req RegisterRequest,
) (models.User, *apperrors.AppError) {
	var user models.User
	err := h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if appErr := lockRegistrationBootstrap(tx); appErr != nil {
			return appErr
		}
		if appErr := ensureUserDoesNotExist(tx, req); appErr != nil {
			return appErr
		}

		role, appErr := h.nextRegistrationRole(tx, req)
		if appErr != nil {
			return appErr
		}

		hashedPassword, err := auth.HashPassword(req.Password)
		if err != nil {
			var appErr *apperrors.AppError
			if stderrors.As(err, &appErr) {
				return appErr
			}
			return apperrors.Internal("Failed to hash password")
		}

		user = models.User{
			Username: req.Username,
			Email:    req.Email,
			Password: hashedPassword,
			Role:     role,
			Status:   "active",
			APIKey:   generateAPIKey(),
		}
		if err := tx.Create(&user).Error; err != nil {
			return apperrors.DatabaseError(err, "Failed to create user")
		}

		return nil
	})
	if err != nil {
		var appErr *apperrors.AppError
		if stderrors.As(err, &appErr) {
			return models.User{}, appErr
		}
		return models.User{}, apperrors.DatabaseError(err, "Failed to create user")
	}

	return user, nil
}

func lockRegistrationBootstrap(tx *gorm.DB) *apperrors.AppError {
	lock := models.SystemLock{Name: registrationBootstrapLockName}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&lock).Error; err != nil {
		return apperrors.DatabaseError(err, "Failed to create registration bootstrap lock")
	}

	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("name = ?", registrationBootstrapLockName).
		First(&lock).Error; err != nil {
		return apperrors.DatabaseError(err, "Failed to acquire registration bootstrap lock")
	}

	return nil
}

func ensureUserDoesNotExist(db *gorm.DB, req RegisterRequest) *apperrors.AppError {
	var existingUser models.User
	err := db.Where("username = ? OR email = ?", req.Username, req.Email).
		First(&existingUser).Error
	if err == nil {
		return apperrors.DuplicateRecord("Username or email already exists")
	}
	if err != gorm.ErrRecordNotFound {
		return apperrors.DatabaseError(err, "Failed to check existing user")
	}

	return nil
}

func (h *AuthHandler) nextRegistrationRole(db *gorm.DB, req RegisterRequest) (string, *apperrors.AppError) {
	var userCount int64
	if err := db.Model(&models.User{}).Count(&userCount).Error; err != nil {
		return "", apperrors.DatabaseError(err, "Failed to count existing users")
	}

	role := registrationRoleForUserCount(userCount)
	if role != "admin" {
		return role, nil
	}
	if !validAdminBootstrapToken(req.AdminBootstrapToken, h.adminBootstrapToken) {
		return "", apperrors.Forbidden("Valid admin bootstrap token is required to create the initial admin")
	}

	return role, nil
}

func registrationRoleForUserCount(userCount int64) string {
	if userCount == 0 {
		return "admin"
	}

	return "user"
}

func validAdminBootstrapToken(providedToken, configuredToken string) bool {
	if strings.TrimSpace(configuredToken) == "" {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(providedToken), []byte(configuredToken)) == 1
}
