package cache

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRateLimiterAllowWithStateAllowsWhenRedisUnavailable(t *testing.T) {
	limiter := &RateLimiter{}

	state, err := limiter.AllowWithState(context.Background(), "moderation:client:42", 60, time.Minute)
	if err != nil {
		t.Fatalf("AllowWithState() error = %v", err)
	}

	if !state.Allowed {
		t.Fatal("Allowed = false, want true when Redis is unavailable")
	}
	if state.Enforced {
		t.Fatal("Enforced = true, want false when Redis is unavailable")
	}
	if state.Limit != 60 {
		t.Fatalf("Limit = %d, want 60", state.Limit)
	}
	if state.Remaining != 60 {
		t.Fatalf("Remaining = %d, want 60", state.Remaining)
	}

	allowed, err := limiter.Allow(context.Background(), "moderation:client:42", 60, time.Minute)
	if err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	if !allowed {
		t.Fatal("Allow() = false, want true when Redis is unavailable")
	}
}

func TestRedisInteger(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		want    int64
		wantErr string
	}{
		{
			name:  "int64",
			value: int64(42),
			want:  42,
		},
		{
			name:  "string",
			value: "42",
			want:  42,
		},
		{
			name:  "bytes",
			value: []byte("42"),
			want:  42,
		},
		{
			name:    "bad string",
			value:   "not-a-number",
			wantErr: "invalid syntax",
		},
		{
			name:    "unexpected type",
			value:   struct{}{},
			wantErr: "unexpected redis integer type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := redisInteger(tt.value)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("redisInteger() error = nil, want error")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("redisInteger() error = %q, want %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("redisInteger() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("redisInteger() = %d, want %d", got, tt.want)
			}
		})
	}
}
