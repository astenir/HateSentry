package logging

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// StructuredLogger 结构化日志接口
type StructuredLogger interface {
	Info(msg string, fields ...zap.Field)
	Debug(msg string, fields ...zap.Field)
	Warn(msg string, fields ...zap.Field)
	Error(msg string, fields ...zap.Field)
	Fatal(msg string, fields ...zap.Field)
	WithFields(fields map[string]interface{}) *Logger
	WithRequestID(requestID string) *Logger
	WithUserID(userID string) *Logger
	WithTraceID(traceID string) *Logger
	WithComponent(component string) *Logger
}

// APILogger API 请求日志记录器
type APILogger struct {
	*Logger
}

func NewAPILogger(baseLogger *Logger) *APILogger {
	return &APILogger{Logger: baseLogger}
}

// LogRequest 记录API请求
func (l *APILogger) LogRequest(method, path, query, clientIP, userAgent string) {
	l.Logger.Info("API Request",
		zap.String("type", "api_request"),
		zap.String("method", method),
		zap.String("path", path),
		zap.String("query", query),
		zap.String("client_ip", clientIP),
		zap.String("user_agent", userAgent),
	)
}

// LogResponse 记录API响应
func (l *APILogger) LogResponse(statusCode int, latency time.Duration, bodySize int) {
	fields := []zap.Field{
		zap.String("type", "api_response"),
		zap.Int("status_code", statusCode),
		zap.Float64("latency_ms", float64(latency.Nanoseconds())/1e6),
		zap.Int("body_size", bodySize),
	}

	if statusCode >= 500 {
		l.Logger.Error("API Response", fields...)
	} else if statusCode >= 400 {
		l.Logger.Warn("API Response", fields...)
	} else {
		l.Logger.Info("API Response", fields...)
	}
}

// LogDetection 记录检测请求
func (l *APILogger) LogDetection(requestID, contentType string, provider string) {
	l.Logger.Info("Detection Request",
		zap.String("type", "detection"),
		zap.String("request_id", requestID),
		zap.String("content_type", contentType),
		zap.String("provider", provider),
	)
}

// LogDetectionResult 记录检测结果
func (l *APILogger) LogDetectionResult(requestID string, isHate bool, confidence float64, duration time.Duration) {
	l.Logger.Info("Detection Result",
		zap.String("type", "detection_result"),
		zap.String("request_id", requestID),
		zap.Bool("is_hate_speech", isHate),
		zap.Float64("confidence", confidence),
		zap.Float64("duration_ms", float64(duration.Nanoseconds())/1e6),
	)
}

// LogDBOperation 记录数据库操作
func (l *APILogger) LogDBOperation(operation, table string, duration time.Duration, err error) {
	fields := []zap.Field{
		zap.String("type", "db_operation"),
		zap.String("operation", operation),
		zap.String("table", table),
		zap.Float64("duration_ms", float64(duration.Nanoseconds())/1e6),
	}

	if err != nil {
		fields = append(fields, zap.Error(err))
		l.Logger.Error("Database Operation", fields...)
	} else {
		l.Logger.Debug("Database Operation", fields...)
	}
}

// LogCacheOperation 记录缓存操作
func (l *APILogger) LogCacheOperation(operation, cacheType string, hit bool, duration time.Duration) {
	fields := []zap.Field{
		zap.String("type", "cache_operation"),
		zap.String("operation", operation),
		zap.String("cache_type", cacheType),
		zap.Bool("hit", hit),
		zap.Float64("duration_ms", float64(duration.Nanoseconds())/1e6),
	}

	l.Logger.Debug("Cache Operation", fields...)
}

// LogQueueOperation 记录队列操作
func (l *APILogger) LogQueueOperation(operation, queue string, count int) {
	fields := []zap.Field{
		zap.String("type", "queue_operation"),
		zap.String("operation", operation),
		zap.String("queue", queue),
		zap.Int("count", count),
	}

	l.Logger.Info("Queue Operation", fields...)
}

// LogError 记录错误
func (l *APILogger) LogError(err error, context map[string]interface{}) {
	fields := []zap.Field{
		zap.String("type", "error"),
		zap.Error(err),
	}

	for k, v := range context {
		fields = append(fields, zap.Any(k, v))
	}

	l.Logger.Error("Error occurred", fields...)
}

// LogAuth 记录认证事件
func (l *APILogger) LogAuth(event, userID, method string, success bool) {
	fields := []zap.Field{
		zap.String("type", "auth"),
		zap.String("event", event),
		zap.String("user_id", userID),
		zap.String("method", method),
		zap.Bool("success", success),
	}

	if success {
		l.Logger.Info("Auth Event", fields...)
	} else {
		l.Logger.Warn("Auth Event Failed", fields...)
	}
}

// LogRateLimit 记录限流事件
func (l *APILogger) LogRateLimit(endpoint, clientIP string, limit, remaining int) {
	fields := []zap.Field{
		zap.String("type", "rate_limit"),
		zap.String("endpoint", endpoint),
		zap.String("client_ip", clientIP),
		zap.Int("limit", limit),
		zap.Int("remaining", remaining),
	}

	l.Logger.Warn("Rate Limit Exceeded", fields...)
}

// SystemLogger 系统日志记录器
type SystemLogger struct {
	*Logger
}

func NewSystemLogger(baseLogger *Logger) *SystemLogger {
	return &SystemLogger{Logger: baseLogger}
}

// LogStartup 记录系统启动
func (l *SystemLogger) LogStartup(version, environment string) {
	l.Logger.Info("System Startup",
		zap.String("type", "system"),
		zap.String("event", "startup"),
		zap.String("version", version),
		zap.String("environment", environment),
	)
}

// LogShutdown 记录系统关闭
func (l *SystemLogger) LogShutdown(reason string) {
	l.Logger.Info("System Shutdown",
		zap.String("type", "system"),
		zap.String("event", "shutdown"),
		zap.String("reason", reason),
	)
}

// LogConfig 记录配置变更
func (l *SystemLogger) LogConfig(component string, changes map[string]interface{}) {
	l.Logger.Info("Configuration Changed",
		zap.String("type", "system"),
		zap.String("event", "config_change"),
		zap.String("component", component),
		zap.Any("changes", changes),
	)
}

// LogHealthCheck 记录健康检查
func (l *SystemLogger) LogHealthCheck(component string, status string, duration time.Duration) {
	fields := []zap.Field{
		zap.String("type", "health_check"),
		zap.String("component", component),
		zap.String("status", status),
		zap.Float64("duration_ms", float64(duration.Nanoseconds())/1e6),
	}

	if status == "healthy" {
		l.Logger.Debug("Health Check", fields...)
	} else {
		l.Logger.Warn("Health Check Failed", fields...)
	}
}

// SecurityLogger 安全日志记录器
type SecurityLogger struct {
	*Logger
}

func NewSecurityLogger(baseLogger *Logger) *SecurityLogger {
	return &SecurityLogger{Logger: baseLogger}
}

// LogSecurityEvent 记录安全事件
func (l *SecurityLogger) LogSecurityEvent(eventType, userID, clientIP, details string) {
	l.Logger.Warn("Security Event",
		zap.String("type", "security"),
		zap.String("event_type", eventType),
		zap.String("user_id", userID),
		zap.String("client_ip", clientIP),
		zap.String("details", details),
	)
}

// LogSuspiciousActivity 记录可疑活动
func (l *SecurityLogger) LogSuspiciousActivity(userID, clientIP, pattern string) {
	l.Logger.Error("Suspicious Activity Detected",
		zap.String("type", "security"),
		zap.String("event_type", "suspicious_activity"),
		zap.String("user_id", userID),
		zap.String("client_ip", clientIP),
		zap.String("pattern", pattern),
	)
}

// PerformanceLogger 性能日志记录器
type PerformanceLogger struct {
	*Logger
}

func NewPerformanceLogger(baseLogger *Logger) *PerformanceLogger {
	return &PerformanceLogger{Logger: baseLogger}
}

// LogSlowQuery 记录慢查询
func (l *PerformanceLogger) LogSlowQuery(query string, duration time.Duration, threshold time.Duration) {
	l.Logger.Warn("Slow Query Detected",
		zap.String("type", "performance"),
		zap.String("event_type", "slow_query"),
		zap.Float64("duration_ms", float64(duration.Nanoseconds())/1e6),
		zap.Float64("threshold_ms", float64(threshold.Nanoseconds())/1e6),
		zap.String("query", query),
	)
}

// LogSlowAPI 记录慢API调用
func (l *PerformanceLogger) LogSlowAPI(method, path string, duration time.Duration, threshold time.Duration) {
	l.Logger.Warn("Slow API Call Detected",
		zap.String("type", "performance"),
		zap.String("event_type", "slow_api"),
		zap.String("method", method),
		zap.String("path", path),
		zap.Float64("duration_ms", float64(duration.Nanoseconds())/1e6),
		zap.Float64("threshold_ms", float64(threshold.Nanoseconds())/1e6),
	)
}

// LogHighLatency 记录高延迟
func (l *PerformanceLogger) LogHighLatency(component string, duration time.Duration, threshold time.Duration) {
	l.Logger.Warn("High Latency Detected",
		zap.String("type", "performance"),
		zap.String("event_type", "high_latency"),
		zap.String("component", component),
		zap.Float64("duration_ms", float64(duration.Nanoseconds())/1e6),
		zap.Float64("threshold_ms", float64(threshold.Nanoseconds())/1e6),
	)
}

// RequestContext 请求上下文日志辅助函数
func WithRequestContext(ctx context.Context, logger *Logger) *Logger {
	// 从上下文中提取常见字段
	reqLogger := logger

	if traceID := ctx.Value(TraceIDKey); traceID != nil {
		if tid, ok := traceID.(string); ok {
			reqLogger = reqLogger.WithTraceID(tid)
		}
	}

	if requestID := ctx.Value(RequestIDKey); requestID != nil {
		if rid, ok := requestID.(string); ok {
			reqLogger = reqLogger.WithRequestID(rid)
		}
	}

	if userID := ctx.Value(UserIDKey); userID != nil {
		if uid, ok := userID.(string); ok {
			reqLogger = reqLogger.WithUserID(uid)
		}
	}

	return reqLogger
}

// TimeMeasure 测量执行时间的辅助函数
func TimeMeasure(start time.Time, component, operation string, logger *Logger) {
	duration := time.Since(start)
	logger.Debug("Operation completed",
		zap.String("component", component),
		zap.String("operation", operation),
		zap.Float64("duration_ms", float64(duration.Nanoseconds())/1e6),
	)
}

// MeasureAndLog 测量并记录执行时间
func MeasureAndLog(component, operation string, logger *Logger) func() {
	start := time.Now()
	return func() {
		TimeMeasure(start, component, operation, logger)
	}
}

// SafeLog 安全地记录日志，防止panic
func SafeLog(fn func(), logger *Logger) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Logger panic recovered", zap.Any("panic", r))
		}
	}()
	fn()
}

// ContextLogger 上下文感知的日志记录器
type ContextLogger struct {
	*Logger
	ctx context.Context
}

func NewContextLogger(ctx context.Context, logger *Logger) *ContextLogger {
	return &ContextLogger{
		Logger: WithRequestContext(ctx, logger),
		ctx:    ctx,
	}
}

// InfoWithContext 记录包含上下文的信息日志
func (l *ContextLogger) InfoWithContext(msg string, fields ...zap.Field) {
	l.Logger.Info(msg, fields...)
}

// ErrorWithContext 记录包含上下文的错误日志
func (l *ContextLogger) ErrorWithContext(msg string, err error, fields ...zap.Field) {
	allFields := append(fields, zap.Error(err))
	l.Logger.Error(msg, allFields...)
}

// GetContext 获取上下文
func (l *ContextLogger) GetContext() context.Context {
	return l.ctx
}

// WrapOperation 包装操作以记录执行情况
func WrapOperation(component, operation string, logger *Logger) func(func() error) error {
	return func(fn func() error) error {
		start := time.Now()
		logger.Debug("Operation started",
			zap.String("component", component),
			zap.String("operation", operation),
		)

		err := fn()

		duration := time.Since(start)
		fields := []zap.Field{
			zap.String("component", component),
			zap.String("operation", operation),
			zap.Float64("duration_ms", float64(duration.Nanoseconds())/1e6),
		}

		if err != nil {
			logger.Error("Operation failed", append(fields, zap.Error(err))...)
			return fmt.Errorf("%s failed: %w", operation, err)
		}

		logger.Debug("Operation completed", fields...)
		return nil
	}
}
