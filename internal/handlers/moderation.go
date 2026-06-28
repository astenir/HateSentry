package handlers

import (
	stderrors "errors"
	"io"
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

// ReviewActionRequest is the optional request body for review finalization.
type ReviewActionRequest struct {
	Notes         string `json:"notes"`
	FinalDecision string `json:"final_decision"`
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

// GetResult retrieves a stored moderation result for the authenticated user.
func (h *ModerationHandler) GetResult(c *gin.Context) {
	claims, exists := auth.GetClaims(c)
	if !exists {
		apperrors.RespondWithError(c, apperrors.Unauthorized("User not authenticated"))
		return
	}
	if h.service == nil {
		apperrors.RespondWithError(c, apperrors.ConfigurationError("moderation service is not configured"))
		return
	}

	result, err := h.service.GetResult(
		c.Request.Context(),
		claims.UserID,
		c.Param("request_id"),
	)
	if err != nil {
		apperrors.Handle(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// ListReviewCases returns review cases for the authenticated user.
func (h *ModerationHandler) ListReviewCases(c *gin.Context) {
	claims, exists := auth.GetClaims(c)
	if !exists {
		apperrors.RespondWithError(c, apperrors.Unauthorized("User not authenticated"))
		return
	}
	if h.service == nil {
		apperrors.RespondWithError(c, apperrors.ConfigurationError("moderation service is not configured"))
		return
	}

	results, err := h.service.ListReviewCases(
		c.Request.Context(),
		claims.UserID,
		c.Query("status"),
	)
	if err != nil {
		apperrors.Handle(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": results})
}

// ApproveReviewCase finalizes a review case as allowed.
func (h *ModerationHandler) ApproveReviewCase(c *gin.Context) {
	h.finalizeReviewCase(c, func(
		claims *auth.Claims,
		req ReviewActionRequest,
	) (moderation.ReviewCaseOutput, error) {
		return h.service.ApproveReviewCase(
			c.Request.Context(),
			c.Param("id"),
			claims.UserID,
			req.Notes,
		)
	})
}

// RejectReviewCase finalizes a review case as blocked.
func (h *ModerationHandler) RejectReviewCase(c *gin.Context) {
	h.finalizeReviewCase(c, func(
		claims *auth.Claims,
		req ReviewActionRequest,
	) (moderation.ReviewCaseOutput, error) {
		return h.service.RejectReviewCase(
			c.Request.Context(),
			c.Param("id"),
			claims.UserID,
			req.Notes,
		)
	})
}

// MarkReviewMistake finalizes a review case and records it as a policy/provider mistake.
func (h *ModerationHandler) MarkReviewMistake(c *gin.Context) {
	h.finalizeReviewCase(c, func(
		claims *auth.Claims,
		req ReviewActionRequest,
	) (moderation.ReviewCaseOutput, error) {
		return h.service.MarkReviewMistake(
			c.Request.Context(),
			c.Param("id"),
			claims.UserID,
			moderation.Decision(req.FinalDecision),
			req.Notes,
		)
	})
}

func (h *ModerationHandler) finalizeReviewCase(
	c *gin.Context,
	action func(*auth.Claims, ReviewActionRequest) (moderation.ReviewCaseOutput, error),
) {
	claims, exists := auth.GetClaims(c)
	if !exists {
		apperrors.RespondWithError(c, apperrors.Unauthorized("User not authenticated"))
		return
	}
	if h.service == nil {
		apperrors.RespondWithError(c, apperrors.ConfigurationError("moderation service is not configured"))
		return
	}

	req, ok := bindOptionalReviewActionRequest(c)
	if !ok {
		return
	}

	result, err := action(claims, req)
	if err != nil {
		apperrors.Handle(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

func bindOptionalReviewActionRequest(c *gin.Context) (ReviewActionRequest, bool) {
	var req ReviewActionRequest
	if c.Request.Body == nil || c.Request.ContentLength == 0 {
		return req, true
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		if stderrors.Is(err, io.EOF) {
			return req, true
		}
		apperrors.RespondWithError(c, apperrors.ValidationError("Invalid request body").WithDetails(err.Error()))
		return ReviewActionRequest{}, false
	}

	return req, true
}
