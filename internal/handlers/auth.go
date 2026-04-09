package handlers

import (
	"hatesentry/internal/auth"
	apperrors "hatesentry/internal/errors"
	"hatesentry/internal/models"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuthHandler handles authentication requests
type AuthHandler struct {
	db         *gorm.DB
	jwtManager *auth.JWTManager
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(db *gorm.DB, jwtManager *auth.JWTManager) *AuthHandler {
	return &AuthHandler{
		db:         db,
		jwtManager: jwtManager,
	}
}

// RegisterRequest represents registration request
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
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
	Token  string `json:"token"`
	User   UserInfo `json:"user"`
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

	// Check if user already exists
	var existingUser models.User
	if err := h.db.Where("username = ? OR email = ?", req.Username, req.Email).First(&existingUser).Error; err == nil {
		apperrors.RespondWithError(c, apperrors.DuplicateRecord("Username or email already exists"))
		return
	} else if err != gorm.ErrRecordNotFound {
		apperrors.RespondWithError(c, apperrors.DatabaseError(err, "Failed to check existing user"))
		return
	}

	// Hash password
	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		apperrors.RespondWithError(c, err.(*apperrors.AppError))
		return
	}

	// Create user
	user := models.User{
		Username: req.Username,
		Email:    req.Email,
		Password: hashedPassword,
		Role:     "user",
		Status:   "active",
		APIKey:   generateAPIKey(),
	}

	if err := h.db.Create(&user).Error; err != nil {
		apperrors.RespondWithError(c, apperrors.DatabaseError(err, "Failed to create user"))
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
