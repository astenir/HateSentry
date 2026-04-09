package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"hatesentry/internal/config"
	"hatesentry/internal/errors"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	RedisClient *redis.Client
)

// Initialize initializes the Redis connection
func Initialize(cfg *config.RedisConfig) error {
	RedisClient = redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := RedisClient.Ping(ctx).Err(); err != nil {
		return errors.ExternalServiceError(err, "Failed to connect to Redis")
	}

	log.Println("Redis connection established successfully")
	return nil
}

// Close closes the Redis connection
func Close() error {
	if RedisClient == nil {
		return nil
	}
	return RedisClient.Close()
}

// Get retrieves a value from Redis
func Get(ctx context.Context, key string) (string, error) {
	return RedisClient.Get(ctx, key).Result()
}

// Set stores a value in Redis
func Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return RedisClient.Set(ctx, key, value, expiration).Err()
}

// GetJSON retrieves a JSON value from Redis and unmarshals it
func GetJSON(ctx context.Context, key string, dest interface{}) error {
	val, err := RedisClient.Get(ctx, key).Result()
	if err != nil {
		return errors.CacheError(err, "Failed to get JSON value from Redis")
	}
	return json.Unmarshal([]byte(val), dest)
}

// SetJSON stores a value as JSON in Redis
func SetJSON(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return errors.Internal("Failed to marshal JSON").WithDetails(err.Error())
	}
	return RedisClient.Set(ctx, key, data, expiration).Err()
}

// Delete removes a key from Redis
func Delete(ctx context.Context, keys ...string) error {
	return RedisClient.Del(ctx, keys...).Err()
}

// Exists checks if a key exists
func Exists(ctx context.Context, key string) (bool, error) {
	count, err := RedisClient.Exists(ctx, key).Result()
	return count > 0, err
}

// Expire sets an expiration time for a key
func Expire(ctx context.Context, key string, expiration time.Duration) error {
	return RedisClient.Expire(ctx, key, expiration).Err()
}

// Increment increments a counter
func Increment(ctx context.Context, key string) (int64, error) {
	return RedisClient.Incr(ctx, key).Result()
}

// Decrement decrements a counter
func Decrement(ctx context.Context, key string) (int64, error) {
	return RedisClient.Decr(ctx, key).Result()
}

// HealthCheck checks the Redis health
func HealthCheck(ctx context.Context) error {
	if RedisClient == nil {
		return errors.Internal("Redis not initialized")
	}

	if err := RedisClient.Ping(ctx).Err(); err != nil {
		return errors.ExternalServiceError(err, "Redis health check failed")
	}

	return nil
}
