package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"hatesentry/internal/auth"
	apperrors "hatesentry/internal/errors"
	"hatesentry/internal/moderation"
)

// ModerationHandler handles text moderation requests.
type ModerationHandler struct {
	service *moderation.Service
}

// NewModerationHandler creates a new moderation handler.
func NewModerationHandler(service *moderation.Service) *ModerationHandler {
	return &ModerationHandler{service: service}
}

// ModerationCheckRequest is the request body for text moderation.
type ModerationCheckRequest struct {
	Content    string `json:"content" binding:"required"`
	Source     string `json:"source"`
	ExternalID string `json:"external_id"`
	ActorID    string `json:"actor_id"`
}

// Check runs a synchronous text moderation check.
func (h *ModerationHandler) Check(c *gin.Context) {
	claims, exists := auth.GetClaims(c)
	if !exists {
		apperrors.RespondWithError(c, apperrors.Unauthorized("User not authenticated"))
		return
	}
	if h.service == nil {
		apperrors.RespondWithError(c, apperrors.ConfigurationError("moderation service is not configured"))
		return
	}

	var req ModerationCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apperrors.RespondWithError(c, apperrors.ValidationError("Invalid request body").WithDetails(err.Error()))
		return
	}

	result, err := h.service.Check(c.Request.Context(), moderation.CheckInput{
		UserID:     claims.UserID,
		Content:    req.Content,
		Source:     req.Source,
		ExternalID: req.ExternalID,
		ActorID:    req.ActorID,
	})
	if err != nil {
		apperrors.Handle(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}
