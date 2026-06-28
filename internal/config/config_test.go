package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadOverridesComposeEnvironment(t *testing.T) {
	configPath := writeTestConfig(t)

	t.Setenv("DB_HOST", "mysql")
	t.Setenv("DB_PORT", "3307")
	t.Setenv("DB_USERNAME", "compose_user")
	t.Setenv("DB_PASSWORD", "compose_password")
	t.Setenv("DB_DATABASE", "compose_db")
	t.Setenv("REDIS_HOST", "redis")
	t.Setenv("REDIS_PORT", "6380")
	t.Setenv("REDIS_PASSWORD", "redis_secret")
	t.Setenv("RABBITMQ_HOST", "rabbitmq")
	t.Setenv("RABBITMQ_PORT", "5673")
	t.Setenv("RABBITMQ_USERNAME", "queue_user")
	t.Setenv("RABBITMQ_PASSWORD", "queue_password")
	t.Setenv("JWT_SECRET", "jwt_secret")
	t.Setenv("OPENAI_API_KEY", "openai_secret")
	t.Setenv("OLLAMA_BASE_URL", "http://ollama:11434")
	t.Setenv("MODERATION_POLICY_VERSION", "custom-v1")
	t.Setenv("MODERATION_REVIEW_THRESHOLD", "0.25")
	t.Setenv("MODERATION_BLOCK_THRESHOLD", "0.8")
	t.Setenv("MODERATION_CLIENT_RATE_LIMIT", "25")
	t.Setenv("MODERATION_CLIENT_RATE_WINDOW", "2m")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Database.Host != "mysql" {
		t.Fatalf("Database.Host = %q, want mysql", cfg.Database.Host)
	}
	if cfg.Database.Port != 3307 {
		t.Fatalf("Database.Port = %d, want 3307", cfg.Database.Port)
	}
	if cfg.Database.Username != "compose_user" {
		t.Fatalf("Database.Username = %q, want compose_user", cfg.Database.Username)
	}
	if cfg.Database.Password != "compose_password" {
		t.Fatalf("Database.Password = %q, want compose_password", cfg.Database.Password)
	}
	if cfg.Database.Database != "compose_db" {
		t.Fatalf("Database.Database = %q, want compose_db", cfg.Database.Database)
	}
	if cfg.Redis.Host != "redis" {
		t.Fatalf("Redis.Host = %q, want redis", cfg.Redis.Host)
	}
	if cfg.Redis.Port != 6380 {
		t.Fatalf("Redis.Port = %d, want 6380", cfg.Redis.Port)
	}
	if cfg.Redis.Password != "redis_secret" {
		t.Fatalf("Redis.Password = %q, want redis_secret", cfg.Redis.Password)
	}
	if cfg.RabbitMQ.Host != "rabbitmq" {
		t.Fatalf("RabbitMQ.Host = %q, want rabbitmq", cfg.RabbitMQ.Host)
	}
	if cfg.RabbitMQ.Port != 5673 {
		t.Fatalf("RabbitMQ.Port = %d, want 5673", cfg.RabbitMQ.Port)
	}
	if cfg.RabbitMQ.Username != "queue_user" {
		t.Fatalf("RabbitMQ.Username = %q, want queue_user", cfg.RabbitMQ.Username)
	}
	if cfg.RabbitMQ.Password != "queue_password" {
		t.Fatalf("RabbitMQ.Password = %q, want queue_password", cfg.RabbitMQ.Password)
	}
	if cfg.JWT.Secret != "jwt_secret" {
		t.Fatalf("JWT.Secret = %q, want jwt_secret", cfg.JWT.Secret)
	}
	if cfg.AI.OpenAI.APIKey != "openai_secret" {
		t.Fatalf("AI.OpenAI.APIKey = %q, want openai_secret", cfg.AI.OpenAI.APIKey)
	}
	if cfg.AI.Ollama.BaseURL != "http://ollama:11434" {
		t.Fatalf("AI.Ollama.BaseURL = %q, want http://ollama:11434", cfg.AI.Ollama.BaseURL)
	}
	if cfg.Moderation.Policy.Version != "custom-v1" {
		t.Fatalf("Moderation.Policy.Version = %q, want custom-v1", cfg.Moderation.Policy.Version)
	}
	if cfg.Moderation.Policy.ReviewThreshold != 0.25 {
		t.Fatalf("Moderation.Policy.ReviewThreshold = %v, want 0.25", cfg.Moderation.Policy.ReviewThreshold)
	}
	if cfg.Moderation.Policy.BlockThreshold != 0.8 {
		t.Fatalf("Moderation.Policy.BlockThreshold = %v, want 0.8", cfg.Moderation.Policy.BlockThreshold)
	}
	if cfg.Moderation.ClientRateLimit.Limit != 25 {
		t.Fatalf("Moderation.ClientRateLimit.Limit = %d, want 25", cfg.Moderation.ClientRateLimit.Limit)
	}
	if cfg.Moderation.ClientRateLimit.Window != 2*time.Minute {
		t.Fatalf("Moderation.ClientRateLimit.Window = %s, want 2m", cfg.Moderation.ClientRateLimit.Window)
	}
}

func TestLoadRejectsInvalidIntegerEnvironment(t *testing.T) {
	configPath := writeTestConfig(t)
	t.Setenv("DB_PORT", "not-a-port")

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("Load() error = nil, want invalid environment variable error")
	}
	if !strings.Contains(err.Error(), "invalid environment variable") {
		t.Fatalf("Load() error = %q, want invalid environment variable", err.Error())
	}
	if !strings.Contains(err.Error(), "DB_PORT must be an integer") {
		t.Fatalf("Load() error = %q, want DB_PORT detail", err.Error())
	}
}

func TestLoadRejectsInvalidFloatEnvironment(t *testing.T) {
	configPath := writeTestConfig(t)
	t.Setenv("MODERATION_REVIEW_THRESHOLD", "not-a-number")

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("Load() error = nil, want invalid environment variable error")
	}
	if !strings.Contains(err.Error(), "MODERATION_REVIEW_THRESHOLD must be a number") {
		t.Fatalf("Load() error = %q, want moderation threshold detail", err.Error())
	}
}

func TestLoadRejectsInvalidDurationEnvironment(t *testing.T) {
	configPath := writeTestConfig(t)
	t.Setenv("MODERATION_CLIENT_RATE_WINDOW", "not-a-duration")

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("Load() error = nil, want invalid environment variable error")
	}
	if !strings.Contains(err.Error(), "MODERATION_CLIENT_RATE_WINDOW must be a duration") {
		t.Fatalf("Load() error = %q, want moderation rate window detail", err.Error())
	}
}

func TestLoadRejectsNegativeModerationRateLimit(t *testing.T) {
	configPath := writeTestConfig(t)
	t.Setenv("MODERATION_CLIENT_RATE_LIMIT", "-1")

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("Load() error = nil, want invalid rate limit error")
	}
	if !strings.Contains(err.Error(), "limit must be zero or greater") {
		t.Fatalf("Load() error = %q, want non-negative limit detail", err.Error())
	}
}

func TestLoadRejectsInvalidModerationRateWindow(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		wantDetail string
	}{
		{
			name:       "negative",
			value:      "-1s",
			wantDetail: "window must be zero or greater",
		},
		{
			name:       "sub second positive",
			value:      "500ms",
			wantDetail: "window must be zero or at least 1s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := writeTestConfig(t)
			t.Setenv("MODERATION_CLIENT_RATE_WINDOW", tt.value)

			_, err := Load(configPath)
			if err == nil {
				t.Fatal("Load() error = nil, want invalid rate window error")
			}
			if !strings.Contains(err.Error(), tt.wantDetail) {
				t.Fatalf("Load() error = %q, want %q", err.Error(), tt.wantDetail)
			}
		})
	}
}

func TestLoadRejectsEmptyJWTSecretEnvironment(t *testing.T) {
	configPath := writeTestConfig(t)
	t.Setenv("JWT_SECRET", " ")

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("Load() error = nil, want invalid environment variable error")
	}
	if !strings.Contains(err.Error(), "JWT_SECRET must not be empty") {
		t.Fatalf("Load() error = %q, want JWT_SECRET detail", err.Error())
	}
}

func TestLoadRejectsOutOfRangePortEnvironment(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{
			name:  "zero",
			value: "0",
		},
		{
			name:  "negative",
			value: "-1",
		},
		{
			name:  "above maximum",
			value: "65536",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := writeTestConfig(t)
			t.Setenv("DB_PORT", tt.value)

			_, err := Load(configPath)
			if err == nil {
				t.Fatal("Load() error = nil, want invalid environment variable error")
			}
			if !strings.Contains(err.Error(), "DB_PORT must be between 1 and 65535") {
				t.Fatalf("Load() error = %q, want DB_PORT range detail", err.Error())
			}
		})
	}
}

func writeTestConfig(t *testing.T) string {
	t.Helper()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte(testConfigYAML), 0600); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}

	return configPath
}

const testConfigYAML = `
server:
  host: "127.0.0.1"
  port: 8080
  mode: "test"
  read_timeout: 60s
  write_timeout: 60s
database:
  host: "localhost"
  port: 3306
  username: "root"
  password: "password"
  database: "hatesentry"
  charset: "utf8mb4"
  parse_time: true
  loc: "Local"
  max_idle_conns: 10
  max_open_conns: 100
  conn_max_lifetime: 3600s
redis:
  host: "localhost"
  port: 6379
  password: ""
  db: 0
  pool_size: 100
  min_idle_conns: 10
rabbitmq:
  host: "localhost"
  port: 5672
  username: "guest"
  password: "guest"
  vhost: "/"
  queue: "detection_tasks"
  exchange: "hatesentry"
  routing_key: "detection"
jwt:
  secret: "file_secret"
  expire_hours: 24
  issuer: "hatesentry"
ai:
  provider: "openai"
  openai:
    api_key: "file_openai_key"
    base_url: "https://api.openai.com/v1"
    model: "gpt-4o-mini"
    max_tokens: 1000
    temperature: 0.3
  ollama:
    base_url: "http://localhost:11434"
    model: "llama3"
    max_tokens: 1000
    temperature: 0.3
detection:
  enable_image_analysis: true
  enable_text_analysis: true
  confidence_threshold: 0.7
  async_threshold: 5
  max_concurrent_requests: 100
  result_cache_ttl: 3600s
moderation:
  policy:
    version: "default-v1"
    review_threshold: 0.4
    block_threshold: 0.75
  client_rate_limit:
    limit: 60
    window: 1m
logging:
  level: "info"
  format: "json"
  output: "stdout"
`
