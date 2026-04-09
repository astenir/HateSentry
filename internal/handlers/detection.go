package handlers

import (
	"context"
	"fmt"
	"hatesentry/internal/ai"
	"hatesentry/internal/auth"
	"hatesentry/internal/cache"
	"hatesentry/internal/errors"
	"hatesentry/internal/models"
	"hatesentry/internal/queue"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DetectionHandler handles detection requests
type DetectionHandler struct {
	db              *gorm.DB
	detectionService *ai.DetectionService
	publisher       queue.Publisher
	cache           *cache.DetectionCache
	rateLimiter     *cache.RateLimiter
	jwtManager      *auth.JWTManager
}

// NewDetectionHandler creates a new detection handler
func NewDetectionHandler(
	db *gorm.DB,
	detectionService *ai.DetectionService,
	publisher queue.Publisher,
	cache *cache.DetectionCache,
	rateLimiter *cache.RateLimiter,
	jwtManager *auth.JWTManager,
) *DetectionHandler {
	return &DetectionHandler{
		db:              db,
		detectionService: detectionService,
		publisher:       publisher,
		cache:           cache,
		rateLimiter:     rateLimiter,
		jwtManager:      jwtManager,
	}
}

// DetectRequest represents the request body for detection
type DetectRequest struct {
	Content     string `json:"content" binding:"required_without=ImageURL"`
	ImageURL    string `json:"image_url"`
	Async       bool   `json:"async"`
	Stream      bool   `json:"stream"`
}

// Detect handles detection requests
func (h *DetectionHandler) Detect(c *gin.Context) {
	claims, exists := auth.GetClaims(c)
	if !exists {
		errors.Handle(c, errors.Unauthorized("Authentication required"))
		return
	}

	// Rate limiting
	allowed, err := h.rateLimiter.Allow(c.Request.Context(), "detect:"+claims.Username, 60, time.Minute)
	if err != nil {
		errors.Handle(c, errors.Internal("Rate limit check failed").WithDetails(err.Error()))
		return
	}
	if !allowed {
		errors.Handle(c, errors.RateLimitExceeded("Rate limit exceeded"))
		return
	}

	var req DetectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errors.Handle(c, errors.ValidationError("Invalid request body").WithDetails(err.Error()))
		return
	}

	requestID := uuid.New().String()

	// Determine content type
	contentType := "text"
	if req.Content != "" && req.ImageURL != "" {
		contentType = "mixed"
	} else if req.ImageURL != "" {
		contentType = "image"
	}

	// Create detection request record
	detectionReq := &models.DetectionRequest{
		RequestID:   requestID,
		UserID:      claims.UserID,
		Content:     req.Content,
		ImageURL:    req.ImageURL,
		ContentType: contentType,
		Status:      "pending",
	}

	if err := h.db.Create(detectionReq).Error; err != nil {
		errors.Handle(c, errors.DatabaseError(err, "Failed to create detection request"))
		return
	}

	// Handle streaming requests
	if req.Stream {
		h.handleStreamingDetection(c, detectionReq, req)
		return
	}

	// Handle async requests
	if req.Async {
		if err := h.publisher.PublishDetectionRequest(c.Request.Context(), detectionReq, 5); err != nil {
			errors.Handle(c, errors.Internal("Failed to publish task to queue").WithDetails(err.Error()))
			return
		}

		c.JSON(http.StatusAccepted, gin.H{
			"request_id": requestID,
			"status":     "queued",
			"message":    "Detection task queued for processing",
		})
		return
	}

	// Sync detection
	h.handleSyncDetection(c, detectionReq)
}

// handleSyncDetection handles synchronous detection
func (h *DetectionHandler) handleSyncDetection(c *gin.Context, detectionReq *models.DetectionRequest) {
	// Check cache first
	if cached, err := h.cache.GetDetectionResult(c.Request.Context(), detectionReq.RequestID); err == nil {
		c.JSON(http.StatusOK, cached)
		return
	}

	// Update status to processing
	detectionReq.Status = "processing"
	if err := h.db.Save(detectionReq).Error; err != nil {
		errors.Handle(c, errors.DatabaseError(err, "Failed to update request status"))
		return
	}

	// Perform detection
	aiReq := &ai.DetectionRequest{
		RequestID:   detectionReq.RequestID,
		Content:     detectionReq.Content,
		ImageURL:    detectionReq.ImageURL,
		ContentType: detectionReq.ContentType,
	}

	resp, err := h.detectionService.Detect(c.Request.Context(), aiReq)
	if err != nil {
		detectionReq.Status = "failed"
		if dbErr := h.db.Save(detectionReq).Error; dbErr != nil {
			errors.Handle(c, errors.DatabaseError(dbErr, "Failed to update request status"))
			return
		}
		errors.Handle(c, errors.Internal("Detection failed").WithDetails(err.Error()))
		return
	}

	// Save result
	result := h.detectionService.ConvertToModel(resp)
	if err := h.db.Create(result).Error; err != nil {
		errors.Handle(c, errors.DatabaseError(err, "Failed to save detection result"))
		return
	}

	// Update detection request
	detectionReq.Processed = true
	detectionReq.Status = "completed"
	if err := h.db.Save(detectionReq).Error; err != nil {
		errors.Handle(c, errors.DatabaseError(err, "Failed to update request status"))
		return
	}

	// Cache result
	h.cache.SetDetectionResult(c.Request.Context(), result)

	c.JSON(http.StatusOK, result)
}

// handleStreamingDetection handles streaming detection
func (h *DetectionHandler) handleStreamingDetection(c *gin.Context, detectionReq *models.DetectionRequest, req DetectRequest) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")

	// Update status
	detectionReq.Status = "processing"
	h.db.Save(detectionReq)

	aiReq := &ai.DetectionRequest{
		RequestID:   detectionReq.RequestID,
		Content:     req.Content,
		ImageURL:    req.ImageURL,
		ContentType: detectionReq.ContentType,
	}

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		errors.Handle(c, errors.Internal("Streaming not supported"))
		return
	}

	done := make(chan bool)

	go func() {
		resp, err := h.detectionService.DetectWithStreaming(c.Request.Context(), aiReq, func(event *ai.StreamDetectionEvent) {
			c.SSEvent(event.Type, event.Data)
			flusher.Flush()
		})

		if err != nil {
			detectionReq.Status = "failed"
			h.db.Save(detectionReq)
			c.SSEvent("error", map[string]string{"error": err.Error()})
			flusher.Flush()
			done <- true
			return
		}

		// Save result
		result := h.detectionService.ConvertToModel(resp)
		if err := h.db.Create(result).Error; err != nil {
			errors.Handle(c, errors.DatabaseError(err, "Failed to save detection result"))
			return
		}

		// Cache result
		h.cache.SetDetectionResult(c.Request.Context(), result)

		detectionReq.Processed = true
		detectionReq.Status = "completed"
		if err := h.db.Save(detectionReq).Error; err != nil {
			errors.Handle(c, errors.DatabaseError(err, "Failed to update request status"))
			return
		}

		done <- true
	}()

	<-done
}

// GetResult retrieves a detection result
func (h *DetectionHandler) GetResult(c *gin.Context) {
	requestID := c.Param("id")

	// Check cache first
	if cached, err := h.cache.GetDetectionResult(c.Request.Context(), requestID); err == nil {
		c.JSON(http.StatusOK, cached)
		return
	}

	var result models.DetectionResult
	if err := h.db.Where("request_id = ?", requestID).First(&result).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			errors.Handle(c, errors.RecordNotFound("Detection result not found"))
		} else {
			errors.Handle(c, errors.DatabaseError(err, "Failed to retrieve detection result"))
		}
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetHistory retrieves detection history for the authenticated user
func (h *DetectionHandler) GetHistory(c *gin.Context) {
	claims, exists := auth.GetClaims(c)
	if !exists {
		errors.Handle(c, errors.Unauthorized("Authentication required"))
		return
	}

	page := c.DefaultQuery("page", "1")
	limit := c.DefaultQuery("limit", "10")

	var requests []models.DetectionRequest
	var total int64

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if err := h.db.WithContext(ctx).Model(&models.DetectionRequest{}).Where("user_id = ?", claims.UserID).Count(&total).Error; err != nil {
		errors.Handle(c, errors.DatabaseError(err, "Failed to count detection requests"))
		return
	}

	offset := (parseInt(page) - 1) * parseInt(limit)

	if err := h.db.WithContext(ctx).Where("user_id = ?", claims.UserID).
		Order("created_at DESC").
		Limit(parseInt(limit)).
		Offset(offset).
		Preload("DetectionResult").
		Find(&requests).Error; err != nil {
		errors.Handle(c, errors.DatabaseError(err, "Failed to retrieve detection history"))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  requests,
		"total": total,
		"page":  parseInt(page),
		"limit": parseInt(limit),
	})
}

func parseInt(s string) int {
	var i int
	if _, err := fmt.Sscanf(s, "%d", &i); err != nil {
		return 0
	}
	return i
}
