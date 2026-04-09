package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"hatesentry/internal/errors"
	"hatesentry/internal/models"
	"sync"
	"time"
)

// CacheLevel represents cache level
type CacheLevel int

const (
	LevelL1 CacheLevel = iota // In-memory cache
	LevelL2                 // Redis cache
	LevelL3                 // Database
)

// CacheEntry represents a cache entry
type CacheEntry struct {
	Data      interface{}
	ExpiresAt time.Time
	AccessedAt time.Time
}

// MultiLevelCache implements a multi-level caching strategy
type MultiLevelCache struct {
	l1Cache *sync.Map // In-memory cache
	l2Cache *DetectionCache
	ttl     time.Duration
	stats   CacheStats
	mu       sync.RWMutex
}

// CacheStats represents cache statistics
type CacheStats struct {
	L1Hits     int64 `json:"l1_hits"`
	L1Misses   int64 `json:"l1_misses"`
	L2Hits     int64 `json:"l2_hits"`
	L2Misses   int64 `json:"l2_misses"`
	Evictions  int64 `json:"evictions"`
}

// NewMultiLevelCache creates a new multi-level cache
func NewMultiLevelCache(l2Cache *DetectionCache, defaultTTL time.Duration) *MultiLevelCache {
	return &MultiLevelCache{
		l1Cache: &sync.Map{},
		l2Cache: l2Cache,
		ttl:     defaultTTL,
	}
}

// Get retrieves a value from cache with multi-level lookup
func (mlc *MultiLevelCache) Get(ctx context.Context, requestID string, result interface{}) (bool, error) {
	hash := mlc.generateHash(requestID)

	// Level 1: In-memory cache
	if entry, ok := mlc.l1Cache.Load(hash); ok {
		cacheEntry := entry.(*CacheEntry)
		if !cacheEntry.ExpiresAt.IsZero() && time.Now().After(cacheEntry.ExpiresAt) {
			// Entry expired, remove it
			mlc.l1Cache.Delete(hash)
			mlc.mu.Lock()
			mlc.stats.Evictions++
			mlc.mu.Unlock()
		} else {
			// Cache hit
			cacheEntry.AccessedAt = time.Now()
			if data, err := json.Marshal(cacheEntry.Data); err == nil {
				json.Unmarshal(data, result)
			}
			mlc.mu.Lock()
			mlc.stats.L1Hits++
			mlc.mu.Unlock()
			return true, nil
		}
	}

	mlc.mu.Lock()
	mlc.stats.L1Misses++
	mlc.mu.Unlock()

	// Level 2: Redis cache
	if mlc.l2Cache != nil {
		cachedResult, err := mlc.l2Cache.GetDetectionResult(ctx, requestID)
		if err == nil {
			// Promote to L1 cache
			mlc.setL1Cache(hash, cachedResult)
			mlc.mu.Lock()
			mlc.stats.L2Hits++
			mlc.mu.Unlock()
			return true, nil
		}
	}

	mlc.mu.Lock()
	mlc.stats.L2Misses++
	mlc.mu.Unlock()

	return false, nil
}

// Set stores a value in all cache levels
func (mlc *MultiLevelCache) Set(ctx context.Context, requestID string, result *models.DetectionResult, ttl ...time.Duration) error {
	hash := mlc.generateHash(requestID)

	// Determine expiration time
	cacheTTL := mlc.ttl
	if len(ttl) > 0 && ttl[0] > 0 {
		cacheTTL = ttl[0]
	}

	// Set in L1 cache with computed TTL
	mlc.setL1CacheWithTTL(hash, result, cacheTTL)

	// Set in L2 cache
	if mlc.l2Cache != nil {
		return mlc.l2Cache.SetDetectionResult(ctx, result)
	}

	return nil
}

// setL1CacheWithTTL stores a value in L1 cache with custom TTL
func (mlc *MultiLevelCache) setL1CacheWithTTL(hash string, data interface{}, ttl time.Duration) {
	entry := &CacheEntry{
		Data:      data,
		ExpiresAt: time.Now().Add(ttl),
		AccessedAt: time.Now(),
	}
	mlc.l1Cache.Store(hash, entry)
}

// setL1Cache stores a value in L1 cache
func (mlc *MultiLevelCache) setL1Cache(hash string, data interface{}) {
	entry := &CacheEntry{
		Data:      data,
		ExpiresAt: time.Now().Add(mlc.ttl),
		AccessedAt: time.Now(),
	}
	mlc.l1Cache.Store(hash, entry)
}

// Invalidate removes a value from all cache levels
func (mlc *MultiLevelCache) Invalidate(ctx context.Context, requestID string) error {
	hash := mlc.generateHash(requestID)

	// Remove from L1
	mlc.l1Cache.Delete(hash)

	// Remove from L2
	if mlc.l2Cache != nil {
		return mlc.l2Cache.InvalidateDetectionResult(ctx, requestID)
	}

	return nil
}

// GetStats returns cache statistics
func (mlc *MultiLevelCache) GetStats() CacheStats {
	mlc.mu.RLock()
	defer mlc.mu.RUnlock()
	return mlc.stats
}

// ResetStats resets cache statistics
func (mlc *MultiLevelCache) ResetStats() {
	mlc.mu.Lock()
	defer mlc.mu.Unlock()
	mlc.stats = CacheStats{}
}

// generateHash generates a consistent hash for cache key
func (mlc *MultiLevelCache) generateHash(key string) string {
	hasher := sha256.New()
	hasher.Write([]byte(key))
	return hex.EncodeToString(hasher.Sum(nil))[:16] // Use first 16 chars
}

// GetHitRate returns the overall cache hit rate
func (mlc *MultiLevelCache) GetHitRate() float64 {
	mlc.mu.RLock()
	defer mlc.mu.RUnlock()

	totalHits := mlc.stats.L1Hits + mlc.stats.L2Hits
	totalMisses := mlc.stats.L1Misses + mlc.stats.L2Misses
	totalAccesses := totalHits + totalMisses

	if totalAccesses == 0 {
		return 0.0
	}

	return float64(totalHits) / float64(totalAccesses) * 100
}

// L1Evictor evicts expired entries from L1 cache
func (mlc *MultiLevelCache) L1Evictor(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			mlc.l1Cache.Range(func(key, value interface{}) bool {
				entry := value.(*CacheEntry)
				if !entry.ExpiresAt.IsZero() && now.After(entry.ExpiresAt) {
					mlc.l1Cache.Delete(key)
					mlc.mu.Lock()
					mlc.stats.Evictions++
					mlc.mu.Unlock()
				}
				return true
			})
		case <-ctx.Done():
			return
		}
	}
}

// SmartCache implements intelligent caching with content similarity
type SmartCache struct {
	multiLevelCache *MultiLevelCache
	similarityThreshold float64
}

// NewSmartCache creates a new smart cache
func NewSmartCache(mlc *MultiLevelCache, similarityThreshold float64) *SmartCache {
	return &SmartCache{
		multiLevelCache:   mlc,
		similarityThreshold: similarityThreshold,
	}
}

// FindSimilar finds cached results for similar content
func (sc *SmartCache) FindSimilar(ctx context.Context, content string) (*models.DetectionResult, error) {
	// Implement content similarity search
	// This could use vector embeddings or simple text similarity
	// For now, return nil to indicate no similar result found
	return nil, errors.NotImplemented("content similarity search not implemented")
}

// GetWithSimilarity retrieves from cache or finds similar results
func (sc *SmartCache) GetWithSimilarity(ctx context.Context, requestID, content string, result interface{}) (bool, error) {
	// Try exact match first
	found, err := sc.multiLevelCache.Get(ctx, requestID, result)
	if found {
		return true, nil
	}

	// Try to find similar content
	similarResult, err := sc.FindSimilar(ctx, content)
	if err == nil && similarResult != nil {
		// Return similar result with lower confidence
		similarResult.Confidence *= 0.8
		if data, err := json.Marshal(similarResult); err == nil {
			json.Unmarshal(data, result)
		}
		return true, nil
	}

	return false, nil
}
