package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"hatesentry/internal/ai"
	"hatesentry/internal/auth"
	"hatesentry/internal/cache"
	"hatesentry/internal/errors"
	"hatesentry/internal/queue"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BatchDetectionRequest represents a batch detection request
type BatchDetectionRequest struct {
	Items     []DetectionItem `json:"items" binding:"required,max=100"`
	Async      bool          `json:"async"`
	Priority   int           `json:"priority"`
}

// DetectionItem represents a single detection item in batch
type DetectionItem struct {
	Content     string `json:"content" binding:"required_without=ImageURL"`
	ImageURL    string `json:"image_url"`
	BatchItemID string `json:"batch_item_id"` // User-provided ID for tracking
}

// BatchDetectionResponse represents a batch detection response
type BatchDetectionResponse struct {
	BatchID    string                      `json:"batch_id"`
	Status      string                      `json:"status"` // processing, completed, partial, failed
	Total       int                         `json:"total"`
	Processed   int                         `json:"processed"`
	Failed      int                         `json:"failed"`
	Results     map[string]*DetectionResultItem `json:"results"`
	CompletedAt *time.Time                  `json:"completed_at,omitempty"`
	StartedAt   time.Time                   `json:"started_at"`
}

// DetectionResultItem represents a single detection result
type DetectionResultItem struct {
	BatchItemID  string                  `json:"batch_item_id"`
	RequestID    string                  `json:"request_id"`
	IsHateSpeech bool                    `json:"is_hate_speech"`
	Confidence   float64                 `json:"confidence"`
	Categories   []string                `json:"categories"`
	Explanation  string                  `json:"explanation"`
	Error        string                  `json:"error,omitempty"`
	ProcessingTime int64                 `json:"processing_time"`
}

// BatchDetectionHandler handles batch detection requests
type BatchDetectionHandler struct {
	db              *gorm.DB
	detectionService *ai.DetectionService
	publisher       queue.Publisher
	cache           *cache.DetectionCache
	rateLimiter     *cache.RateLimiter
	jwtManager      *auth.JWTManager
}

// NewBatchDetectionHandler creates a new batch detection handler
func NewBatchDetectionHandler(
	db *gorm.DB,
	detectionService *ai.DetectionService,
	publisher queue.Publisher,
	cache *cache.DetectionCache,
	rateLimiter *cache.RateLimiter,
	jwtManager *auth.JWTManager,
) *BatchDetectionHandler {
	return &BatchDetectionHandler{
		db:              db,
		detectionService: detectionService,
		publisher:       publisher,
		cache:           cache,
		rateLimiter:     rateLimiter,
		jwtManager:      jwtManager,
	}
}

// DetectBatch handles batch detection requests
func (h *BatchDetectionHandler) DetectBatch(c *gin.Context) {
	claims, exists := auth.GetClaims(c)
	if !exists {
		errors.Handle(c, errors.Unauthorized("Authentication required"))
		return
	}

	// Rate limiting for batch requests (more restrictive)
	allowed, err := h.rateLimiter.Allow(c.Request.Context(), "batch:"+claims.Username, 10, time.Minute)
	if err != nil {
		errors.Handle(c, errors.Internal("Rate limit check failed").WithDetails(err.Error()))
		return
	}
	if !allowed {
		errors.Handle(c, errors.RateLimitExceeded("Batch rate limit exceeded (max 10 per minute)"))
		return
	}

	var req BatchDetectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errors.Handle(c, errors.ValidationError("Invalid request body").WithDetails(err.Error()))
		return
	}

	batchID := uuid.New().String()
	startTime := time.Now()

	// Create batch tracking record
	batchResp := &BatchDetectionResponse{
		BatchID:    batchID,
		Status:      "processing",
		Total:       len(req.Items),
		Processed:   0,
		Failed:      0,
		Results:     make(map[string]*DetectionResultItem),
		StartedAt:   startTime,
	}

	if req.Async {
		// Async processing
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Batch processing panic: %v", r)
				}
			}()

			if err := h.processBatchAsync(context.Background(), claims.UserID, batchID, &req, batchResp); err != nil {
				log.Printf("Batch processing failed: %v", err)
			}
		}()

		c.JSON(http.StatusAccepted, gin.H{
			"batch_id": batchID,
			"status":   "queued",
			"message":  "Batch detection queued for processing",
		})
		return
	}

	// Sync processing
	h.processBatchSync(c.Request.Context(), claims.UserID, batchID, &req, batchResp)
	
	now := time.Now()
	batchResp.CompletedAt = &now
	
	c.JSON(http.StatusOK, batchResp)
}

// processBatchSync processes batch detection synchronously
func (h *BatchDetectionHandler) processBatchSync(
	ctx context.Context,
	userID uint,
	batchID string,
	req *BatchDetectionRequest,
	batchResp *BatchDetectionResponse,
) {
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 10) // Limit concurrent goroutines
	resultMutex := sync.Mutex{}

	for _, item := range req.Items {
		wg.Add(1)
		go func(detectionItem DetectionItem) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result := h.processSingleDetection(ctx, userID, detectionItem, req.Priority)
			
			resultMutex.Lock()
			batchResp.Results[detectionItem.BatchItemID] = result
			if result.Error == "" {
				batchResp.Processed++
			} else {
				batchResp.Failed++
			}
			resultMutex.Unlock()
		}(item)
	}

	wg.Wait()

	// Determine overall status
	if batchResp.Failed == 0 {
		batchResp.Status = "completed"
	} else if batchResp.Processed > 0 {
		batchResp.Status = "partial"
	} else {
		batchResp.Status = "failed"
	}
}

// processBatchAsync processes batch detection asynchronously
func (h *BatchDetectionHandler) processBatchAsync(
	ctx context.Context,
	userID uint,
	batchID string,
	req *BatchDetectionRequest,
	batchResp *BatchDetectionResponse,
) error {
	// Process batch (could use worker pool for better performance)
	h.processBatchSync(ctx, userID, batchID, req, batchResp)

	// Cache batch results (could implement batch caching)
	// Store batch metadata in database for tracking
	return nil
}

// processSingleDetection processes a single detection
func (h *BatchDetectionHandler) processSingleDetection(
	ctx context.Context,
	userID uint,
	item DetectionItem,
	priority int,
) *DetectionResultItem {
	// Generate content-based cache key
	contentHash := generateContentHash(item.Content, item.ImageURL)
	requestID := uuid.New().String()

	resultItem := &DetectionResultItem{
		BatchItemID: item.BatchItemID,
		RequestID:   requestID,
	}

	// Check cache first using content hash
	cached, err := h.cache.GetDetectionResult(ctx, contentHash)
	if err == nil {
		resultItem.IsHateSpeech = cached.IsHateSpeech
		resultItem.Confidence = cached.Confidence
		resultItem.Categories = parseCategories(cached.Categories)
		resultItem.Explanation = cached.Explanation
		resultItem.ProcessingTime = cached.ProcessingTime
		return resultItem
	}

	// Determine content type
	contentType := "text"
	if item.Content != "" && item.ImageURL != "" {
		contentType = "mixed"
	} else if item.ImageURL != "" {
		contentType = "image"
	}

	// Create detection request
	aiReq := &ai.DetectionRequest{
		RequestID:   requestID,
		Content:     item.Content,
		ImageURL:    item.ImageURL,
		ContentType: contentType,
	}

	startTime := time.Now()

	// Call detection service
	resp, err := h.detectionService.Detect(ctx, aiReq)
	if err != nil {
		resultItem.Error = err.Error()
		return resultItem
	}

	resultItem.IsHateSpeech = resp.IsHateSpeech
	resultItem.Confidence = resp.Confidence
	resultItem.Categories = resp.Categories
	resultItem.Explanation = resp.Explanation
	resultItem.ProcessingTime = time.Since(startTime).Milliseconds()

	// Save result to database
	modelResult := h.detectionService.ConvertToModel(resp)
	if dbErr := h.db.Create(modelResult).Error; dbErr != nil {
		resultItem.Error = "Failed to save result to database"
		return resultItem
	}

	// Cache result using content hash
	modelResult.RequestID = contentHash // Use content hash as request ID for caching
	h.cache.SetDetectionResult(ctx, modelResult)

	return resultItem
}

// GetBatchResult retrieves batch detection results
func (h *BatchDetectionHandler) GetBatchResult(c *gin.Context) {
	_ = c.Param("id") // TODO: Implement batch result retrieval

	// Retrieve batch results from cache or database
	// For simplicity, this example assumes results are cached

	errors.Handle(c, errors.NotFound("Batch result retrieval not implemented yet").WithDetails("Batch results should be retrieved from a separate batch tracking system"))
}

// GetBatchStatus retrieves batch processing status
func (h *BatchDetectionHandler) GetBatchStatus(c *gin.Context) {
	_ = c.Param("id") // TODO: Implement batch status retrieval

	// Retrieve batch status from tracking system

	errors.Handle(c, errors.NotFound("Batch status tracking not implemented yet").WithDetails("Status tracking should be implemented"))
}

func parseCategories(categoriesJSON string) []string {
	if categoriesJSON == "" {
		return []string{}
	}

	var categories []string
	if err := json.Unmarshal([]byte(categoriesJSON), &categories); err != nil {
		return []string{}
	}
	return categories
}

// generateContentHash generates a hash for content-based caching
func generateContentHash(content, imageURL string) string {
	hasher := sha256.New()
	hasher.Write([]byte(content))
	hasher.Write([]byte(imageURL))
	return hex.EncodeToString(hasher.Sum(nil))
}
