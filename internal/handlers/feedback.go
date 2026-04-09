package handlers

import (
	"hatesentry/internal/auth"
	apperrors "hatesentry/internal/errors"
	"hatesentry/internal/cache"
	"hatesentry/internal/models"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// FeedbackHandler handles user feedback on detection results
type FeedbackHandler struct {
	db         *gorm.DB
	cache      *cache.DetectionCache
	jwtManager *auth.JWTManager
}

// NewFeedbackHandler creates a new feedback handler
func NewFeedbackHandler(
	db *gorm.DB,
	cache *cache.DetectionCache,
	jwtManager *auth.JWTManager,
) *FeedbackHandler {
	return &FeedbackHandler{
		db:         db,
		cache:      cache,
		jwtManager: jwtManager,
	}
}

// SubmitFeedbackRequest represents the feedback submission request
type SubmitFeedbackRequest struct {
	RequestID          string  `json:"request_id" binding:"required"`
	IsCorrect          bool    `json:"is_correct" binding:"required"`
	ActualLabel        string  `json:"actual_label" binding:"required_if=IsCorrect false"`
	Confidence         float64 `json:"confidence" binding:"required,min=1,max=10"`
	Comments          string  `json:"comments"`
	CorrectCategories  []string `json:"correct_categories"`
	CorrectExplanation string  `json:"correct_explanation"`
}

// SubmitFeedback handles user feedback submission
func (h *FeedbackHandler) SubmitFeedback(c *gin.Context) {
	claims, exists := auth.GetClaims(c)
	if !exists {
		apperrors.RespondWithError(c, apperrors.Unauthorized("User not authenticated"))
		return
	}

	var req SubmitFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apperrors.RespondWithError(c, apperrors.ValidationError(err.Error()))
		return
	}

	// Verify the detection request belongs to the user
	var detectionReq models.DetectionRequest
	if err := h.db.Where("request_id = ? AND user_id = ?", req.RequestID, claims.UserID).First(&detectionReq).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			apperrors.RespondWithError(c, apperrors.RecordNotFound("Detection request not found"))
		} else {
			apperrors.RespondWithError(c, apperrors.DatabaseError(err, "Failed to find detection request"))
		}
		return
	}

	// Check if feedback already exists for this request
	var existingFeedback models.DetectionFeedback
	err := h.db.Where("request_id = ? AND user_id = ?", req.RequestID, claims.UserID).First(&existingFeedback).Error
	if err == nil {
		apperrors.RespondWithError(c, apperrors.Conflict("Feedback already submitted for this request"))
		return
	} else if err != gorm.ErrRecordNotFound {
		apperrors.RespondWithError(c, apperrors.DatabaseError(err, "Failed to check existing feedback"))
		return
	}

	// Create feedback record
	feedback := &models.DetectionFeedback{
		RequestID:          req.RequestID,
		UserID:             claims.UserID,
		IsCorrect:          req.IsCorrect,
		ActualLabel:        req.ActualLabel,
		Confidence:         req.Confidence,
		Comments:          req.Comments,
		CorrectCategories:  marshalCategories(req.CorrectCategories),
		CorrectExplanation: req.CorrectExplanation,
		Status:            "pending",
	}

	if err := h.db.Create(feedback).Error; err != nil {
		apperrors.RespondWithError(c, apperrors.DatabaseError(err, "Failed to save feedback"))
		return
	}

	// Invalidate cache for this request to force re-evaluation if needed
	h.cache.InvalidateDetectionResult(c.Request.Context(), req.RequestID)

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Feedback submitted successfully",
		"feedback_id": feedback.ID,
	})
}

// GetFeedback retrieves feedback for a specific request
func (h *FeedbackHandler) GetFeedback(c *gin.Context) {
	claims, exists := auth.GetClaims(c)
	if !exists {
		apperrors.RespondWithError(c, apperrors.Unauthorized("User not authenticated"))
		return
	}

	requestID := c.Param("id")

	var feedback models.DetectionFeedback
	if err := h.db.Where("request_id = ? AND user_id = ?", requestID, claims.UserID).First(&feedback).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			apperrors.RespondWithError(c, apperrors.RecordNotFound("Feedback not found"))
		} else {
			apperrors.RespondWithError(c, apperrors.DatabaseError(err, "Failed to find feedback"))
		}
		return
	}

	c.JSON(http.StatusOK, feedback)
}

// GetFeedbackHistory retrieves user's feedback history
func (h *FeedbackHandler) GetFeedbackHistory(c *gin.Context) {
	claims, exists := auth.GetClaims(c)
	if !exists {
		apperrors.RespondWithError(c, apperrors.Unauthorized("User not authenticated"))
		return
	}

	page := c.DefaultQuery("page", "1")
	limit := c.DefaultQuery("limit", "20")

	var feedbacks []models.DetectionFeedback
	var total int64

	if err := h.db.Model(&models.DetectionFeedback{}).Where("user_id = ?", claims.UserID).Count(&total).Error; err != nil {
		apperrors.RespondWithError(c, apperrors.DatabaseError(err, "Failed to count feedbacks"))
		return
	}

	offset := (parseInt(page) - 1) * parseInt(limit)

	if err := h.db.Where("user_id = ?", claims.UserID).
		Order("created_at DESC").
		Limit(parseInt(limit)).
		Offset(offset).
		Find(&feedbacks).Error; err != nil {
		apperrors.RespondWithError(c, apperrors.DatabaseError(err, "Failed to retrieve feedback history"))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  feedbacks,
		"total": total,
		"page":  parseInt(page),
		"limit": parseInt(limit),
	})
}

// GetFeedbackStats retrieves aggregated feedback statistics
func (h *FeedbackHandler) GetFeedbackStats(c *gin.Context) {
	claims, exists := auth.GetClaims(c)
	if !exists {
		apperrors.Handle(c, apperrors.Unauthorized("Authentication required"))
		return
	}

	// User-level statistics
	var stats struct {
		TotalFeedback      int64   `json:"total_feedback"`
		PositiveFeedback   int64   `json:"positive_feedback"`
		NegativeFeedback   int64   `json:"negative_feedback"`
		AccuracyRate      float64 `json:"accuracy_rate"`
	}

	h.db.Model(&models.DetectionFeedback{}).Where("user_id = ?", claims.UserID).Count(&stats.TotalFeedback)
	h.db.Model(&models.DetectionFeedback{}).Where("user_id = ? AND is_correct = ?", claims.UserID, true).Count(&stats.PositiveFeedback)
	h.db.Model(&models.DetectionFeedback{}).Where("user_id = ? AND is_correct = ?", claims.UserID, false).Count(&stats.NegativeFeedback)

	if stats.TotalFeedback > 0 {
		stats.AccuracyRate = float64(stats.PositiveFeedback) / float64(stats.TotalFeedback) * 100
	}

	c.JSON(http.StatusOK, stats)
}

// Admin: ReviewFeedback allows admins to review feedback
func (h *FeedbackHandler) ReviewFeedback(c *gin.Context) {
	claims, exists := auth.GetClaims(c)
	if !exists {
		apperrors.Handle(c, apperrors.Unauthorized("Authentication required"))
		return
	}

	// Only admins can review feedback
	if claims.Role != "admin" {
		apperrors.Handle(c, apperrors.Forbidden("Insufficient permissions"))
		return
	}

	feedbackID := c.Param("id")

	type ReviewRequest struct {
		Action    string `json:"action" binding:"required"` // approve, reject, incorporate
		Comments  string `json:"comments"`
	}

	var req ReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apperrors.Handle(c, apperrors.ValidationError("Invalid request body").WithDetails(err.Error()))
		return
	}

	updates := map[string]interface{}{
		"reviewed_by": claims.UserID,
		"status":       req.Action,
	}

	if req.Comments != "" {
		updates["review_comments"] = req.Comments
	}

	if err := h.db.Model(&models.DetectionFeedback{}).Where("id = ?", feedbackID).Updates(updates).Error; err != nil {
		apperrors.Handle(c, apperrors.DatabaseError(err, "Failed to update feedback"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Feedback reviewed successfully"})
}

func marshalCategories(categories []string) string {
	// Implement JSON marshaling for categories
	// Simplified implementation
	if len(categories) == 0 {
		return "[]"
	}
	result := "["
	for i, cat := range categories {
		result += `"` + cat + `"`
		if i < len(categories)-1 {
			result += ","
		}
	}
	result += "]"
	return result
}
