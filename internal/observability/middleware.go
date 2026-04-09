package observability

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsMiddleware Gin 中间件，用于收集 HTTP 请求指标
func MetricsMiddleware() gin.HandlerFunc {
	httpMetrics := NewHTTPMetrics()

	return func(c *gin.Context) {
		start := httpTime()

		// 处理请求
		c.Next()

		// 记录指标
		duration := httpTime().Sub(start)

		httpMetrics.RecordRequest(c.Request.Method, c.FullPath(), c.Writer.Status())
		httpMetrics.RecordDuration(c.Request.Method, c.FullPath(), duration)

		if c.Request.ContentLength > 0 {
			httpMetrics.RecordRequestSize(c.Request.Method, c.FullPath(), c.Request.ContentLength)
		}

		if c.Writer.Size() > 0 {
			httpMetrics.RecordResponseSize(c.Request.Method, c.FullPath(), int64(c.Writer.Size()))
		}
	}
}

// httpTime 获取当前时间，用于测试模拟
func httpTime() httpTimeType {
	return httpTimeType(time.Now())
}

type httpTimeType time.Time

func (t httpTimeType) Sub(other httpTimeType) time.Duration {
	return time.Time(t).Sub(time.Time(other))
}

// RegisterMetricsEndpoint 注册 Prometheus 指标端点
func RegisterMetricsEndpoint(router *gin.Engine) {
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
}
