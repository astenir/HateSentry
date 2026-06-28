package config

import (
	"hatesentry/internal/errors"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Redis      RedisConfig      `mapstructure:"redis"`
	RabbitMQ   RabbitMQConfig   `mapstructure:"rabbitmq"`
	Auth       AuthConfig       `mapstructure:"auth"`
	JWT        JWTConfig        `mapstructure:"jwt"`
	AI         AIConfig         `mapstructure:"ai"`
	Detection  DetectionConfig  `mapstructure:"detection"`
	Moderation ModerationConfig `mapstructure:"moderation"`
	Logging    LoggingConfig    `mapstructure:"logging"`
}

type ServerConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	Mode         string        `mapstructure:"mode"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	Username        string        `mapstructure:"username"`
	Password        string        `mapstructure:"password"`
	Database        string        `mapstructure:"database"`
	Charset         string        `mapstructure:"charset"`
	ParseTime       bool          `mapstructure:"parse_time"`
	Loc             string        `mapstructure:"loc"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

type RedisConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Password     string `mapstructure:"password"`
	DB           int    `mapstructure:"db"`
	PoolSize     int    `mapstructure:"pool_size"`
	MinIdleConns int    `mapstructure:"min_idle_conns"`
}

type RabbitMQConfig struct {
	Host       string `mapstructure:"host"`
	Port       int    `mapstructure:"port"`
	Username   string `mapstructure:"username"`
	Password   string `mapstructure:"password"`
	Vhost      string `mapstructure:"vhost"`
	Queue      string `mapstructure:"queue"`
	Exchange   string `mapstructure:"exchange"`
	RoutingKey string `mapstructure:"routing_key"`
}

type AuthConfig struct {
	AdminBootstrapToken string `mapstructure:"admin_bootstrap_token"`
}

type JWTConfig struct {
	Secret      string `mapstructure:"secret"`
	ExpireHours int    `mapstructure:"expire_hours"`
	Issuer      string `mapstructure:"issuer"`
}

type AIConfig struct {
	Provider string       `mapstructure:"provider"`
	OpenAI   OpenAIConfig `mapstructure:"openai"`
	Ollama   OllamaConfig `mapstructure:"ollama"`
}

type OpenAIConfig struct {
	APIKey      string  `mapstructure:"api_key"`
	BaseURL     string  `mapstructure:"base_url"`
	Model       string  `mapstructure:"model"`
	MaxTokens   int     `mapstructure:"max_tokens"`
	Temperature float64 `mapstructure:"temperature"`
}

type OllamaConfig struct {
	BaseURL     string  `mapstructure:"base_url"`
	Model       string  `mapstructure:"model"`
	MaxTokens   int     `mapstructure:"max_tokens"`
	Temperature float64 `mapstructure:"temperature"`
}

type DetectionConfig struct {
	EnableImageAnalysis bool          `mapstructure:"enable_image_analysis"`
	EnableTextAnalysis  bool          `mapstructure:"enable_text_analysis"`
	ConfidenceThreshold float64       `mapstructure:"confidence_threshold"`
	AsyncThreshold      int           `mapstructure:"async_threshold"`
	MaxConcurrentReq    int           `mapstructure:"max_concurrent_requests"`
	ResultCacheTTL      time.Duration `mapstructure:"result_cache_ttl"`
}

type ModerationConfig struct {
	Policy          ModerationPolicyConfig    `mapstructure:"policy"`
	Policies        []ModerationPolicyConfig  `mapstructure:"policies"`
	ClientRateLimit ModerationRateLimitConfig `mapstructure:"client_rate_limit"`
	WebhookRetry    WebhookRetryConfig        `mapstructure:"webhook_retry"`
}

type ModerationPolicyConfig struct {
	Version         string  `mapstructure:"version"`
	ReviewThreshold float64 `mapstructure:"review_threshold"`
	BlockThreshold  float64 `mapstructure:"block_threshold"`
}

type ModerationRateLimitConfig struct {
	Limit  int           `mapstructure:"limit"`
	Window time.Duration `mapstructure:"window"`
}

type WebhookRetryConfig struct {
	Enabled     bool          `mapstructure:"enabled"`
	Interval    time.Duration `mapstructure:"interval"`
	BatchSize   int           `mapstructure:"batch_size"`
	MaxAttempts int           `mapstructure:"max_attempts"`
}

type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
}

func Load(configPath string) (*Config, error) {
	loader := viper.New()
	loader.SetConfigFile(configPath)
	loader.SetConfigType("yaml")

	// Set defaults
	loader.SetDefault("server.host", "0.0.0.0")
	loader.SetDefault("server.port", 8080)
	loader.SetDefault("server.mode", "debug")
	loader.SetDefault("ai.provider", "openai")
	loader.SetDefault("moderation.policy.version", "default-v1")
	loader.SetDefault("moderation.policy.review_threshold", 0.4)
	loader.SetDefault("moderation.policy.block_threshold", 0.75)
	loader.SetDefault("moderation.client_rate_limit.limit", 60)
	loader.SetDefault("moderation.client_rate_limit.window", time.Minute)
	loader.SetDefault("moderation.webhook_retry.enabled", true)
	loader.SetDefault("moderation.webhook_retry.interval", time.Minute)
	loader.SetDefault("moderation.webhook_retry.batch_size", 10)
	loader.SetDefault("moderation.webhook_retry.max_attempts", 3)

	if err := loader.ReadInConfig(); err != nil {
		return nil, errors.ConfigurationError("failed to read config file").WithDetails(err.Error())
	}

	var config Config
	if err := loader.Unmarshal(&config); err != nil {
		return nil, errors.ConfigurationError("failed to unmarshal config").WithDetails(err.Error())
	}

	if err := applyEnvironmentOverrides(&config); err != nil {
		return nil, err
	}
	if err := validateConfig(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

func validateConfig(config *Config) error {
	switch strings.TrimSpace(config.AI.Provider) {
	case "openai", "ollama":
	default:
		return errors.ConfigurationError("invalid AI provider").WithDetails("AI_PROVIDER must be openai or ollama")
	}
	if config.Moderation.ClientRateLimit.Limit < 0 {
		return errors.ConfigurationError("invalid moderation client rate limit").WithDetails("limit must be zero or greater")
	}
	if config.Moderation.ClientRateLimit.Window < 0 {
		return errors.ConfigurationError("invalid moderation client rate limit").WithDetails("window must be zero or greater")
	}
	if config.Moderation.ClientRateLimit.Window > 0 && config.Moderation.ClientRateLimit.Window < time.Second {
		return errors.ConfigurationError("invalid moderation client rate limit").WithDetails("window must be zero or at least 1s")
	}
	if config.Moderation.WebhookRetry.Interval < 0 {
		return errors.ConfigurationError("invalid moderation webhook retry").WithDetails("interval must be zero or greater")
	}
	if config.Moderation.WebhookRetry.Interval > 0 && config.Moderation.WebhookRetry.Interval < time.Second {
		return errors.ConfigurationError("invalid moderation webhook retry").WithDetails("interval must be zero or at least 1s")
	}
	if config.Moderation.WebhookRetry.BatchSize < 0 {
		return errors.ConfigurationError("invalid moderation webhook retry").WithDetails("batch_size must be zero or greater")
	}
	if config.Moderation.WebhookRetry.MaxAttempts < 0 {
		return errors.ConfigurationError("invalid moderation webhook retry").WithDetails("max_attempts must be zero or greater")
	}
	if config.Moderation.WebhookRetry.Enabled {
		if config.Moderation.WebhookRetry.Interval == 0 {
			return errors.ConfigurationError("invalid moderation webhook retry").WithDetails("interval is required when retry is enabled")
		}
		if config.Moderation.WebhookRetry.BatchSize == 0 {
			return errors.ConfigurationError("invalid moderation webhook retry").WithDetails("batch_size is required when retry is enabled")
		}
		if config.Moderation.WebhookRetry.MaxAttempts <= 1 {
			return errors.ConfigurationError("invalid moderation webhook retry").WithDetails("max_attempts must be greater than 1 when retry is enabled")
		}
	}

	return nil
}

func applyEnvironmentOverrides(config *Config) error {
	overrideString("DB_HOST", &config.Database.Host)
	overrideString("DB_USERNAME", &config.Database.Username)
	overrideString("DB_PASSWORD", &config.Database.Password)
	overrideString("DB_DATABASE", &config.Database.Database)
	overrideString("REDIS_HOST", &config.Redis.Host)
	overrideString("REDIS_PASSWORD", &config.Redis.Password)
	overrideString("RABBITMQ_HOST", &config.RabbitMQ.Host)
	overrideString("RABBITMQ_USERNAME", &config.RabbitMQ.Username)
	overrideString("RABBITMQ_PASSWORD", &config.RabbitMQ.Password)
	overrideString("RABBITMQ_VHOST", &config.RabbitMQ.Vhost)
	overrideString("RABBITMQ_QUEUE", &config.RabbitMQ.Queue)
	overrideString("RABBITMQ_EXCHANGE", &config.RabbitMQ.Exchange)
	overrideString("RABBITMQ_ROUTING_KEY", &config.RabbitMQ.RoutingKey)
	overrideString("ADMIN_BOOTSTRAP_TOKEN", &config.Auth.AdminBootstrapToken)
	if err := overrideRequiredString("JWT_SECRET", &config.JWT.Secret); err != nil {
		return err
	}
	overrideString("AI_PROVIDER", &config.AI.Provider)
	overrideString("OPENAI_API_KEY", &config.AI.OpenAI.APIKey)
	overrideString("OPENAI_BASE_URL", &config.AI.OpenAI.BaseURL)
	overrideString("OPENAI_MODEL", &config.AI.OpenAI.Model)
	overrideString("OLLAMA_BASE_URL", &config.AI.Ollama.BaseURL)
	overrideString("OLLAMA_MODEL", &config.AI.Ollama.Model)
	overrideString("MODERATION_POLICY_VERSION", &config.Moderation.Policy.Version)
	overrideString("LOG_LEVEL", &config.Logging.Level)
	overrideString("LOG_FORMAT", &config.Logging.Format)
	overrideString("LOG_OUTPUT", &config.Logging.Output)

	if err := overridePort("DB_PORT", &config.Database.Port); err != nil {
		return err
	}
	if err := overridePort("REDIS_PORT", &config.Redis.Port); err != nil {
		return err
	}
	if err := overrideInt("REDIS_DB", &config.Redis.DB); err != nil {
		return err
	}
	if err := overridePort("RABBITMQ_PORT", &config.RabbitMQ.Port); err != nil {
		return err
	}
	if err := overrideFloat("MODERATION_REVIEW_THRESHOLD", &config.Moderation.Policy.ReviewThreshold); err != nil {
		return err
	}
	if err := overrideFloat("MODERATION_BLOCK_THRESHOLD", &config.Moderation.Policy.BlockThreshold); err != nil {
		return err
	}
	if err := overrideInt("MODERATION_CLIENT_RATE_LIMIT", &config.Moderation.ClientRateLimit.Limit); err != nil {
		return err
	}
	if err := overrideDuration("MODERATION_CLIENT_RATE_WINDOW", &config.Moderation.ClientRateLimit.Window); err != nil {
		return err
	}
	if err := overrideBool("MODERATION_WEBHOOK_RETRY_ENABLED", &config.Moderation.WebhookRetry.Enabled); err != nil {
		return err
	}
	if err := overrideDuration("MODERATION_WEBHOOK_RETRY_INTERVAL", &config.Moderation.WebhookRetry.Interval); err != nil {
		return err
	}
	if err := overrideInt("MODERATION_WEBHOOK_RETRY_BATCH_SIZE", &config.Moderation.WebhookRetry.BatchSize); err != nil {
		return err
	}
	if err := overrideInt("MODERATION_WEBHOOK_RETRY_MAX_ATTEMPTS", &config.Moderation.WebhookRetry.MaxAttempts); err != nil {
		return err
	}

	return nil
}

func overrideString(name string, target *string) {
	if value, ok := os.LookupEnv(name); ok {
		*target = value
	}
}

func overrideRequiredString(name string, target *string) error {
	value, ok := os.LookupEnv(name)
	if !ok {
		return nil
	}
	if strings.TrimSpace(value) == "" {
		return errors.ConfigurationError("invalid environment variable").WithDetails(name + " must not be empty")
	}

	*target = value
	return nil
}

func overrideInt(name string, target *int) error {
	value, ok := os.LookupEnv(name)
	if !ok {
		return nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return errors.ConfigurationError("invalid environment variable").WithDetails(name + " must be an integer")
	}

	*target = parsed
	return nil
}

func overrideBool(name string, target *bool) error {
	value, ok := os.LookupEnv(name)
	if !ok {
		return nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return errors.ConfigurationError("invalid environment variable").WithDetails(name + " must be a boolean")
	}

	*target = parsed
	return nil
}

func overrideFloat(name string, target *float64) error {
	value, ok := os.LookupEnv(name)
	if !ok {
		return nil
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return errors.ConfigurationError("invalid environment variable").WithDetails(name + " must be a number")
	}

	*target = parsed
	return nil
}

func overrideDuration(name string, target *time.Duration) error {
	value, ok := os.LookupEnv(name)
	if !ok {
		return nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return errors.ConfigurationError("invalid environment variable").WithDetails(name + " must be a duration")
	}

	*target = parsed
	return nil
}

func overridePort(name string, target *int) error {
	value, ok := os.LookupEnv(name)
	if !ok {
		return nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return errors.ConfigurationError("invalid environment variable").WithDetails(name + " must be an integer")
	}
	if parsed < 1 || parsed > 65535 {
		return errors.ConfigurationError("invalid environment variable").WithDetails(name + " must be between 1 and 65535")
	}

	*target = parsed
	return nil
}
