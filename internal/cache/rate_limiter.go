package cache

import (
	"context"
	"hatesentry/internal/errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter implements rate limiting using Redis
type RateLimiter struct {
	client *redis.Client
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		client: RedisClient,
	}
}

// Allow checks if a request should be allowed
func (rl *RateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	if rl.client == nil {
		return true, nil // If Redis is not available, allow all requests
	}

	// Use Redis Lua script for atomic increment and expire
	luaScript := `
		local current
		current = redis.call("incr", KEYS[1])
		if tonumber(current) == 1 then
			redis.call("expire", KEYS[1], ARGV[1])
		end
		return current
	`

	result, err := rl.client.Eval(ctx, luaScript, []string{key}, int(window.Seconds())).Int()
	if err != nil {
		return false, errors.Internal("Rate limiter error").WithDetails(err.Error())
	}

	return result <= limit, nil
}

// GetRemaining returns remaining requests and reset time
func (rl *RateLimiter) GetRemaining(ctx context.Context, key string) (int, time.Time, error) {
	if rl.client == nil {
		return 0, time.Time{}, nil
	}

	current, err := rl.client.Get(ctx, key).Int()
	if err != nil && err != redis.Nil {
		return 0, time.Time{}, errors.Internal("Failed to get rate limit current value").WithDetails(err.Error())
	}

	ttl, err := rl.client.TTL(ctx, key).Result()
	if err != nil {
		return 0, time.Time{}, errors.Internal("Failed to get rate limit TTL").WithDetails(err.Error())
	}

	return current, time.Now().Add(ttl), nil
}
