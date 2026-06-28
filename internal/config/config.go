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
	ClientRateLimit ModerationRateLimitConfig `mapstructure:"client_rate_limit"`
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
	loader.SetDefault("moderation.policy.version", "default-v1")
	loader.SetDefault("moderation.policy.review_threshold", 0.4)
	loader.SetDefault("moderation.policy.block_threshold", 0.75)
	loader.SetDefault("moderation.client_rate_limit.limit", 60)
	loader.SetDefault("moderation.client_rate_limit.window", time.Minute)

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
	if config.Moderation.ClientRateLimit.Limit < 0 {
		return errors.ConfigurationError("invalid moderation client rate limit").WithDetails("limit must be zero or greater")
	}
	if config.Moderation.ClientRateLimit.Window < 0 {
		return errors.ConfigurationError("invalid moderation client rate limit").WithDetails("window must be zero or greater")
	}
	if config.Moderation.ClientRateLimit.Window > 0 && config.Moderation.ClientRateLimit.Window < time.Second {
		return errors.ConfigurationError("invalid moderation client rate limit").WithDetails("window must be zero or at least 1s")
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
	if err := overrideRequiredString("JWT_SECRET", &config.JWT.Secret); err != nil {
		return err
	}
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
