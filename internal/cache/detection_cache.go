package cache

import (
	"context"
	"fmt"
	"hatesentry/internal/models"
	"time"
)

// Cache keys
const (
	CacheKeyDetectionResult = "detection:result:%s"
	CacheKeyUserStats       = "user:stats:%d:%s"
)

// DetectionCache handles caching of detection results
type DetectionCache struct {
	defaultTTL time.Duration
}

// NewDetectionCache creates a new detection cache
func NewDetectionCache(defaultTTL time.Duration) *DetectionCache {
	return &DetectionCache{
		defaultTTL: defaultTTL,
	}
}

// GetDetectionResult retrieves a cached detection result
func (dc *DetectionCache) GetDetectionResult(ctx context.Context, requestID string) (*models.DetectionResult, error) {
	key := fmt.Sprintf(CacheKeyDetectionResult, requestID)
	
	var result models.DetectionResult
	err := GetJSON(ctx, key, &result)
	if err != nil {
		return nil, err
	}
	
	return &result, nil
}

// SetDetectionResult caches a detection result
func (dc *DetectionCache) SetDetectionResult(ctx context.Context, result *models.DetectionResult) error {
	key := fmt.Sprintf(CacheKeyDetectionResult, result.RequestID)
	
	ttl := dc.defaultTTL
	if ttl <= 0 {
		ttl = time.Hour
	}
	
	return SetJSON(ctx, key, result, ttl)
}

// InvalidateDetectionResult removes a cached detection result
func (dc *DetectionCache) InvalidateDetectionResult(ctx context.Context, requestID string) error {
	key := fmt.Sprintf(CacheKeyDetectionResult, requestID)
	return Delete(ctx, key)
}

// GetUserStats retrieves cached user statistics for a date
func (dc *DetectionCache) GetUserStats(ctx context.Context, userID uint, date time.Time) (*models.DetectionStats, error) {
	key := fmt.Sprintf(CacheKeyUserStats, userID, date.Format("2006-01-02"))
	
	var stats models.DetectionStats
	err := GetJSON(ctx, key, &stats)
	if err != nil {
		return nil, err
	}
	
	return &stats, nil
}

// SetUserStats caches user statistics
func (dc *DetectionCache) SetUserStats(ctx context.Context, stats *models.DetectionStats, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	
	dateStr := stats.Date.Format("2006-01-02")
	key := fmt.Sprintf(CacheKeyUserStats, stats.UserID, dateStr)
	
	return SetJSON(ctx, key, stats, ttl)
}

// GenerateCacheKey generates a cache key for a request hash
func GenerateCacheKey(content string, imageURL string) string {
	if imageURL != "" {
		return fmt.Sprintf("detection:hash:%s:%s", content[:min(50, len(content))], imageURL)
	}
	return fmt.Sprintf("detection:hash:%s", content[:min(100, len(content))])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
