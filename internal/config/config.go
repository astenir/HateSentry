package config

import (
	"hatesentry/internal/errors"
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

type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
}

func Load(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	// Set defaults
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.mode", "debug")

	// Bind environment variables
	viper.AutomaticEnv()
	viper.SetEnvPrefix("HATESENTRY")

	if err := viper.ReadInConfig(); err != nil {
		return nil, errors.ConfigurationError("failed to read config file").WithDetails(err.Error())
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, errors.ConfigurationError("failed to unmarshal config").WithDetails(err.Error())
	}

	// Override with environment variables if set
	if dbHost := viper.GetString("DB_HOST"); dbHost != "" {
		config.Database.Host = dbHost
	}
	if dbPass := viper.GetString("DB_PASSWORD"); dbPass != "" {
		config.Database.Password = dbPass
	}
	if jwtSecret := viper.GetString("JWT_SECRET"); jwtSecret != "" {
		config.JWT.Secret = jwtSecret
	}
	if openaiKey := viper.GetString("OPENAI_API_KEY"); openaiKey != "" {
		config.AI.OpenAI.APIKey = openaiKey
	}

	return &config, nil
}
