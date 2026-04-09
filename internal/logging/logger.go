package logging

import (
	"os"

	"gopkg.in/natefinch/lumberjack.v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Config 日志配置
type Config struct {
	Level      string // debug, info, warn, error
	Format     string // json, console
	OutputPath string // 日志文件路径
	MaxSize    int    // 单个日志文件最大大小 (MB)
	MaxBackups int    // 保留的旧日志文件数量
	MaxAge     int    // 保留旧日志文件的最大天数
	Compress   bool   // 是否压缩旧日志文件
}

// DefaultConfig 默认日志配置
func DefaultConfig() *Config {
	return &Config{
		Level:      "info",
		Format:     "json",
		OutputPath: "logs/app.log",
		MaxSize:    100,
		MaxBackups: 10,
		MaxAge:     30,
		Compress:   true,
	}
}

// Logger 封装 zap.Logger
type Logger struct {
	*zap.Logger
	sugar *zap.SugaredLogger
}

// NewLogger 创建新的日志记录器
func NewLogger(config *Config) (*Logger, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// 解析日志级别
	level, err := zapcore.ParseLevel(config.Level)
	if err != nil {
		return nil, err
	}

	// 配置编码器
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    "function",
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	var encoder zapcore.Encoder
	if config.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// 配置输出
	var writer zapcore.WriteSyncer
	if config.OutputPath != "" {
		// 使用 lumberjack 实现日志轮转
		writer = zapcore.AddSync(&lumberjack.Logger{
			Filename:   config.OutputPath,
			MaxSize:    config.MaxSize,
			MaxBackups: config.MaxBackups,
			MaxAge:     config.MaxAge,
			Compress:   config.Compress,
		})
	} else {
		writer = zapcore.AddSync(os.Stdout)
	}

	// 创建核心
	core := zapcore.NewCore(encoder, writer, level)

	// 创建 logger
	zapLogger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	return &Logger{
		Logger: zapLogger,
		sugar:  zapLogger.Sugar(),
	}, nil
}

// NewDevelopmentLogger 创建开发环境日志记录器
func NewDevelopmentLogger() (*Logger, error) {
	config := &Config{
		Level:  "debug",
		Format: "console",
	}
	return NewLogger(config)
}

// NewProductionLogger 创建生产环境日志记录器
func NewProductionLogger() (*Logger, error) {
	config := &Config{
		Level:      "info",
		Format:     "json",
		OutputPath: "logs/app.log",
		MaxSize:    100,
		MaxBackups: 10,
		MaxAge:     30,
		Compress:   true,
	}
	return NewLogger(config)
}

// WithFields 添加字段到日志
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	zapFields := make([]zap.Field, 0, len(fields))
	sugarFields := make([]interface{}, 0, len(fields)*2)
	for k, v := range fields {
		zapFields = append(zapFields, zap.Any(k, v))
		sugarFields = append(sugarFields, k, v)
	}
	return &Logger{
		Logger: l.Logger.With(zapFields...),
		sugar:  l.sugar.With(sugarFields...),
	}
}

// WithRequestID 添加请求ID到日志
func (l *Logger) WithRequestID(requestID string) *Logger {
	return &Logger{
		Logger: l.Logger.With(zap.String("request_id", requestID)),
		sugar:  l.sugar.With(zap.String("request_id", requestID)),
	}
}

// WithUserID 添加用户ID到日志
func (l *Logger) WithUserID(userID string) *Logger {
	return &Logger{
		Logger: l.Logger.With(zap.String("user_id", userID)),
		sugar:  l.sugar.With(zap.String("user_id", userID)),
	}
}

// WithTraceID 添加追踪ID到日志
func (l *Logger) WithTraceID(traceID string) *Logger {
	return &Logger{
		Logger: l.Logger.With(zap.String("trace_id", traceID)),
		sugar:  l.sugar.With(zap.String("trace_id", traceID)),
	}
}

// WithComponent 添加组件名称到日志
func (l *Logger) WithComponent(component string) *Logger {
	return &Logger{
		Logger: l.Logger.With(zap.String("component", component)),
		sugar:  l.sugar.With(zap.String("component", component)),
	}
}

// Info 记录信息日志
func (l *Logger) Info(msg string, fields ...zap.Field) {
	l.Logger.Info(msg, fields...)
}

// Debug 记录调试日志
func (l *Logger) Debug(msg string, fields ...zap.Field) {
	l.Logger.Debug(msg, fields...)
}

// Warn 记录警告日志
func (l *Logger) Warn(msg string, fields ...zap.Field) {
	l.Logger.Warn(msg, fields...)
}

// Error 记录错误日志
func (l *Logger) Error(msg string, fields ...zap.Field) {
	l.Logger.Error(msg, fields...)
}

// Fatal 记录致命错误日志并退出程序
func (l *Logger) Fatal(msg string, fields ...zap.Field) {
	l.Logger.Fatal(msg, fields...)
}

// Sugar 返回 SugaredLogger
func (l *Logger) Sugar() *zap.SugaredLogger {
	return l.sugar
}

// Sync 同步日志缓冲区
func (l *Logger) Sync() error {
	return l.Logger.Sync()
}
