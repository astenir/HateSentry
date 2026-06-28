package cache

import (
	"context"
	"fmt"
	"hatesentry/internal/errors"
	"math"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter implements rate limiting using Redis
type RateLimiter struct {
	client *redis.Client
}

// RateLimitState describes the outcome of an enforced rate limit check.
type RateLimitState struct {
	Allowed    bool
	Enforced   bool
	Limit      int
	Remaining  int
	ResetAt    time.Time
	RetryAfter time.Duration
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		client: RedisClient,
	}
}

// Allow checks if a request should be allowed
func (rl *RateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	state, err := rl.AllowWithState(ctx, key, limit, window)
	if err != nil {
		return false, err
	}

	return state.Allowed, nil
}

// AllowWithState checks if a request should be allowed and returns quota metadata.
func (rl *RateLimiter) AllowWithState(
	ctx context.Context,
	key string,
	limit int,
	window time.Duration,
) (RateLimitState, error) {
	state := RateLimitState{
		Allowed:   true,
		Limit:     limit,
		Remaining: limit,
	}
	if rl.client == nil {
		return state, nil // If Redis is not available, keep legacy allow-all behavior.
	}

	// Use Redis Lua script for atomic increment and expire
	luaScript := `
		local current
		current = redis.call("incr", KEYS[1])
		if tonumber(current) == 1 then
			redis.call("expire", KEYS[1], ARGV[1])
		end
		local pttl
		pttl = redis.call("pttl", KEYS[1])
		return {current, pttl}
	`

	result, err := rl.client.Eval(ctx, luaScript, []string{key}, int(window.Seconds())).Result()
	if err != nil {
		return RateLimitState{}, errors.Internal("Rate limiter error").WithDetails(err.Error())
	}

	values, ok := result.([]interface{})
	if !ok || len(values) != 2 {
		return RateLimitState{}, errors.Internal("Rate limiter error").WithDetails("unexpected redis script result")
	}

	current, err := redisInteger(values[0])
	if err != nil {
		return RateLimitState{}, errors.Internal("Rate limiter error").WithDetails(err.Error())
	}
	ttlMilliseconds, err := redisInteger(values[1])
	if err != nil {
		return RateLimitState{}, errors.Internal("Rate limiter error").WithDetails(err.Error())
	}
	if ttlMilliseconds < 0 {
		ttlMilliseconds = window.Milliseconds()
	}

	remaining := 0
	if current < int64(limit) {
		remaining = limit - int(current)
	}
	now := time.Now()
	resetAt := now.Add(time.Duration(ttlMilliseconds) * time.Millisecond)

	return RateLimitState{
		Allowed:    current <= int64(limit),
		Enforced:   true,
		Limit:      limit,
		Remaining:  remaining,
		ResetAt:    resetAt,
		RetryAfter: resetAt.Sub(now),
	}, nil
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

func redisInteger(value interface{}) (int64, error) {
	switch typed := value.(type) {
	case int:
		return int64(typed), nil
	case int8:
		return int64(typed), nil
	case int16:
		return int64(typed), nil
	case int32:
		return int64(typed), nil
	case int64:
		return typed, nil
	case uint:
		if uint64(typed) > uint64(math.MaxInt64) {
			return 0, fmt.Errorf("redis integer %d overflows int64", typed)
		}
		return int64(typed), nil
	case uint8:
		return int64(typed), nil
	case uint16:
		return int64(typed), nil
	case uint32:
		return int64(typed), nil
	case uint64:
		if typed > uint64(math.MaxInt64) {
			return 0, fmt.Errorf("redis integer %d overflows int64", typed)
		}
		return int64(typed), nil
	case string:
		return strconv.ParseInt(typed, 10, 64)
	case []byte:
		return strconv.ParseInt(string(typed), 10, 64)
	default:
		return 0, fmt.Errorf("unexpected redis integer type %T", value)
	}
}
