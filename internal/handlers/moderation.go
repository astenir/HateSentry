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
	principal, exists := moderationPrincipal(c)
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
		UserID:     principal.UserID,
		ClientID:   principal.ClientID,
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

type checkPrincipal struct {
	UserID   uint
	ClientID uint
}

func moderationPrincipal(c *gin.Context) (checkPrincipal, bool) {
	if claims, exists := auth.GetClaims(c); exists {
		return checkPrincipal{UserID: claims.UserID}, true
	}

	if principal, exists := auth.GetAPIKeyPrincipal(c); exists {
		return checkPrincipal{
			UserID:   principal.UserID,
			ClientID: principal.ClientID,
		}, true
	}

	return checkPrincipal{}, false
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

// ListHistory returns recent moderation history for operators.
func (h *ModerationHandler) ListHistory(c *gin.Context) {
	claims, exists := auth.GetClaims(c)
	if !exists {
		apperrors.RespondWithError(c, apperrors.Unauthorized("User not authenticated"))
		return
	}
	if h.service == nil {
		apperrors.RespondWithError(c, apperrors.ConfigurationError("moderation service is not configured"))
		return
	}

	result, err := h.service.ListHistory(
		c.Request.Context(),
		claims.UserID,
		c.Query("decision"),
		c.Query("client_id"),
		c.Query("external_id"),
		c.Query("limit"),
	)
	if err != nil {
		apperrors.Handle(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// ListPolicies returns configured moderation policies for operator client assignment.
func (h *ModerationHandler) ListPolicies(c *gin.Context) {
	claims, exists := auth.GetClaims(c)
	if !exists {
		apperrors.RespondWithError(c, apperrors.Unauthorized("User not authenticated"))
		return
	}
	if h.service == nil {
		apperrors.RespondWithError(c, apperrors.ConfigurationError("moderation service is not configured"))
		return
	}

	result, err := h.service.ListPolicies(claims.UserID)
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

// GetReviewCase returns one review case for operator inspection.
func (h *ModerationHandler) GetReviewCase(c *gin.Context) {
	claims, exists := auth.GetClaims(c)
	if !exists {
		apperrors.RespondWithError(c, apperrors.Unauthorized("User not authenticated"))
		return
	}
	if h.service == nil {
		apperrors.RespondWithError(c, apperrors.ConfigurationError("moderation service is not configured"))
		return
	}

	result, err := h.service.GetReviewCase(
		c.Request.Context(),
		claims.UserID,
		c.Param("id"),
	)
	if err != nil {
		apperrors.Handle(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetReviewStats returns aggregate moderation and review workflow metrics.
func (h *ModerationHandler) GetReviewStats(c *gin.Context) {
	claims, exists := auth.GetClaims(c)
	if !exists {
		apperrors.RespondWithError(c, apperrors.Unauthorized("User not authenticated"))
		return
	}
	if h.service == nil {
		apperrors.RespondWithError(c, apperrors.ConfigurationError("moderation service is not configured"))
		return
	}

	stats, err := h.service.GetStats(c.Request.Context(), claims.UserID)
	if err != nil {
		apperrors.Handle(c, err)
		return
	}

	c.JSON(http.StatusOK, stats)
}

// ListWebhookDeliveries lists recent final-decision webhook delivery records.
func (h *ModerationHandler) ListWebhookDeliveries(c *gin.Context) {
	claims, exists := auth.GetClaims(c)
	if !exists {
		apperrors.RespondWithError(c, apperrors.Unauthorized("User not authenticated"))
		return
	}
	if h.service == nil {
		apperrors.RespondWithError(c, apperrors.ConfigurationError("moderation service is not configured"))
		return
	}

	result, err := h.service.ListWebhookDeliveries(
		c.Request.Context(),
		claims.UserID,
		moderation.WebhookDeliveryListInput{
			Status:    c.Query("status"),
			ClientID:  c.Query("client_id"),
			RequestID: c.Query("request_id"),
			Limit:     c.Query("limit"),
		},
	)
	if err != nil {
		apperrors.Handle(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetWebhookDelivery returns one final-decision webhook delivery record.
func (h *ModerationHandler) GetWebhookDelivery(c *gin.Context) {
	claims, exists := auth.GetClaims(c)
	if !exists {
		apperrors.RespondWithError(c, apperrors.Unauthorized("User not authenticated"))
		return
	}
	if h.service == nil {
		apperrors.RespondWithError(c, apperrors.ConfigurationError("moderation service is not configured"))
		return
	}

	result, err := h.service.GetWebhookDelivery(
		c.Request.Context(),
		claims.UserID,
		c.Param("id"),
	)
	if err != nil {
		apperrors.Handle(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// RetryWebhookDelivery retries a failed final-decision webhook delivery.
func (h *ModerationHandler) RetryWebhookDelivery(c *gin.Context) {
	claims, exists := auth.GetClaims(c)
	if !exists {
		apperrors.RespondWithError(c, apperrors.Unauthorized("User not authenticated"))
		return
	}
	if h.service == nil {
		apperrors.RespondWithError(c, apperrors.ConfigurationError("moderation service is not configured"))
		return
	}

	result, err := h.service.RetryWebhookDelivery(
		c.Request.Context(),
		claims.UserID,
		c.Param("id"),
	)
	if err != nil {
		apperrors.Handle(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
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
