package logging

import (
	"context"
	"crypto/rand"
	"math/big"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	// TraceIDKey 上下文中的追踪ID键
	TraceIDKey = "trace_id"
	// RequestIDKey 上下文中的请求ID键
	RequestIDKey = "request_id"
	// UserIDKey 上下文中的用户ID键
	UserIDKey = "user_id"
)

// LoggingMiddleware Gin 日志中间件
func LoggingMiddleware(logger *Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 生成追踪ID和请求ID
		traceID := c.GetHeader("X-Trace-ID")
		if traceID == "" {
			traceID = generateID()
		}

		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateID()
		}

		// 设置到上下文
		c.Set(TraceIDKey, traceID)
		c.Set(RequestIDKey, requestID)

		// 创建请求专属logger
		reqLogger := logger.WithRequestID(requestID).WithTraceID(traceID)
		c.Set("logger", reqLogger)

		// 记录请求开始
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		reqLogger.Info("Request started",
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.String("client_ip", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
		)

		// 处理请求
		c.Next()

		// 记录请求结束
		duration := time.Since(start)
		latency := float64(duration.Nanoseconds()) / 1e6

		// 获取响应数据
		statusCode := c.Writer.Status()
		bodySize := c.Writer.Size()

		fields := []zap.Field{
			zap.Int("status_code", statusCode),
			zap.Float64("latency_ms", latency),
			zap.Int("body_size", bodySize),
			zap.String("client_ip", c.ClientIP()),
		}

		// 获取用户ID（如果有）
		if userID, exists := c.Get(UserIDKey); exists {
			if uid, ok := userID.(string); ok {
				reqLogger = reqLogger.WithUserID(uid)
				fields = append(fields, zap.String("user_id", uid))
			}
		}

		// 记录错误信息
		if len(c.Errors) > 0 {
			fields = append(fields, zap.Any("errors", c.Errors.String()))
		}

		// 根据状态码选择日志级别
		if statusCode >= 500 {
			reqLogger.Error("Request completed with server error", fields...)
		} else if statusCode >= 400 {
			reqLogger.Warn("Request completed with client error", fields...)
		} else {
			reqLogger.Info("Request completed successfully", fields...)
		}
	}
}

// RecoveryMiddleware 恢复中间件，记录 panic
func RecoveryMiddleware(logger *Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				reqLogger, _ := c.Get("logger")
				var log *Logger
				if l, ok := reqLogger.(*Logger); ok {
					log = l
				} else {
					log = logger
				}

				log.Error("Panic recovered",
					zap.Any("error", err),
					zap.String("path", c.Request.URL.Path),
					zap.String("method", c.Request.Method),
					zap.Stack("stack"),
				)

				c.JSON(500, gin.H{
					"error": "Internal Server Error",
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}

// AccessLogWriter 自定义响应写入器，用于记录响应体
type AccessLogWriter struct {
	gin.ResponseWriter
	body []byte
}

func (w *AccessLogWriter) Write(b []byte) (int, error) {
	w.body = append(w.body, b...)
	return w.ResponseWriter.Write(b)
}

func (w *AccessLogWriter) WriteString(s string) (int, error) {
	w.body = append(w.body, s...)
	return w.ResponseWriter.WriteString(s)
}

// BodyCaptureMiddleware 捕获响应体中间件
func BodyCaptureMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		w := &AccessLogWriter{
			ResponseWriter: c.Writer,
			body:          make([]byte, 0),
		}
		c.Writer = w
		c.Next()
	}
}

// FromContext 从上下文获取日志记录器
func FromContext(ctx context.Context) *Logger {
	if logger, ok := ctx.Value("logger").(*Logger); ok {
		return logger
	}
	return nil
}

// FromGinContext 从Gin上下文获取日志记录器
func FromGinContext(c *gin.Context) *Logger {
	if logger, exists := c.Get("logger"); exists {
		if log, ok := logger.(*Logger); ok {
			return log
		}
	}
	return nil
}

// RequestIDMiddleware 请求ID中间件
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateID()
		}
		c.Header("X-Request-ID", requestID)
		c.Set(RequestIDKey, requestID)
		c.Next()
	}
}

// generateID 生成唯一ID
func generateID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

// randomString 生成随机字符串
func randomString(n int) string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, n)
	for i := range result {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(letterBytes))))
		result[i] = letterBytes[num.Int64()]
	}
	return string(result)
}

// LogWriter 实现io.Writer接口，用于标准库日志
type LogWriter struct {
	logger *zap.Logger
	level  zapcore.Level
}

func NewLogWriter(logger *zap.Logger, level zapcore.Level) *LogWriter {
	return &LogWriter{
		logger: logger,
		level:  level,
	}
}

func (w *LogWriter) Write(p []byte) (n int, err error) {
	msg := string(p)
	switch w.level {
	case zapcore.DebugLevel:
		w.logger.Debug(msg)
	case zapcore.InfoLevel:
		w.logger.Info(msg)
	case zapcore.WarnLevel:
		w.logger.Warn(msg)
	case zapcore.ErrorLevel:
		w.logger.Error(msg)
	}
	return len(p), nil
}
