package observability

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics 定义所有 Prometheus 指标
var (
	// HTTP 请求指标
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status_code"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	httpRequestSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_size_bytes",
			Help:    "HTTP request size in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 2, 10),
		},
		[]string{"method", "endpoint"},
	)

	httpResponseSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_response_size_bytes",
			Help:    "HTTP response size in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 2, 10),
		},
		[]string{"method", "endpoint"},
	)

	// AI 检测指标
	detectionRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "detection_requests_total",
			Help: "Total number of detection requests",
		},
		[]string{"type", "provider"},
	)

	detectionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "detection_duration_seconds",
			Help:    "Detection latency in seconds",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
		},
		[]string{"type", "provider"},
	)

	detectionResultsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "detection_results_total",
			Help: "Total number of detection results",
		},
		[]string{"result_type", "hate_type"},
	)

	detectionConfidence = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "detection_confidence",
			Help:    "Detection confidence score",
			Buckets: []float64{0.0, 0.2, 0.4, 0.6, 0.8, 1.0},
		},
		[]string{"result_type"},
	)

	// 数据库指标
	dbQueriesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_queries_total",
			Help: "Total number of database queries",
		},
		[]string{"operation", "table"},
	)

	dbQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Database query latency in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
		},
		[]string{"operation", "table"},
	)

	dbConnectionsActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_connections_active",
			Help: "Number of active database connections",
		},
	)

	dbConnectionsIdle = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_connections_idle",
			Help: "Number of idle database connections",
		},
	)

	// 缓存指标
	cacheHitsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_hits_total",
			Help: "Total number of cache hits",
		},
		[]string{"cache_type"},
	)

	cacheMissesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_misses_total",
			Help: "Total number of cache misses",
		},
		[]string{"cache_type"},
	)

	cacheSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cache_size",
			Help: "Current cache size",
		},
		[]string{"cache_type"},
	)

	// 消息队列指标
	mqMessagesPublishedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mq_messages_published_total",
			Help: "Total number of messages published to message queue",
		},
		[]string{"queue"},
	)

	mqMessagesConsumedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mq_messages_consumed_total",
			Help: "Total number of messages consumed from message queue",
		},
		[]string{"queue"},
	)

	mqMessagesFailedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mq_messages_failed_total",
			Help: "Total number of failed messages in message queue",
		},
		[]string{"queue"},
	)

	mqQueueSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mq_queue_size",
			Help: "Current message queue size",
		},
		[]string{"queue"},
	)

	mqConsumerLag = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mq_consumer_lag",
			Help: "Message queue consumer lag",
		},
		[]string{"queue"},
	)

	// 认证指标
	authAttemptsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auth_attempts_total",
			Help: "Total number of authentication attempts",
		},
		[]string{"method", "result"},
	)

	jwtTokensIssuedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "jwt_tokens_issued_total",
			Help: "Total number of JWT tokens issued",
		},
		[]string{"type"},
	)

	// 限流指标
	rateLimitRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rate_limit_requests_total",
			Help: "Total number of rate limited requests",
		},
		[]string{"endpoint"},
	)

	// 系统指标
	activeConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_connections",
			Help: "Number of active connections",
		},
	)

	activeGoroutines = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_goroutines",
			Help: "Number of active goroutines",
		},
	)

	memoryUsage = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "memory_usage_bytes",
			Help: "Current memory usage in bytes",
		},
	)
)

// HTTPMetrics 封装 HTTP 相关的指标收集
type HTTPMetrics struct{}

func NewHTTPMetrics() *HTTPMetrics {
	return &HTTPMetrics{}
}

func (m *HTTPMetrics) RecordRequest(method, endpoint string, statusCode int) {
	httpRequestsTotal.WithLabelValues(method, endpoint, strconv.Itoa(statusCode)).Inc()
}

func (m *HTTPMetrics) RecordDuration(method, endpoint string, duration time.Duration) {
	httpRequestDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
}

func (m *HTTPMetrics) RecordRequestSize(method, endpoint string, size int64) {
	httpRequestSize.WithLabelValues(method, endpoint).Observe(float64(size))
}

func (m *HTTPMetrics) RecordResponseSize(method, endpoint string, size int64) {
	httpResponseSize.WithLabelValues(method, endpoint).Observe(float64(size))
}

// DetectionMetrics 封装检测相关的指标收集
type DetectionMetrics struct{}

func NewDetectionMetrics() *DetectionMetrics {
	return &DetectionMetrics{}
}

func (m *DetectionMetrics) RecordRequest(detectionType, provider string) {
	detectionRequestsTotal.WithLabelValues(detectionType, provider).Inc()
}

func (m *DetectionMetrics) RecordDuration(detectionType, provider string, duration time.Duration) {
	detectionDuration.WithLabelValues(detectionType, provider).Observe(duration.Seconds())
}

func (m *DetectionMetrics) RecordResult(resultType, hateType string) {
	detectionResultsTotal.WithLabelValues(resultType, hateType).Inc()
}

func (m *DetectionMetrics) RecordConfidence(resultType string, confidence float64) {
	detectionConfidence.WithLabelValues(resultType).Observe(confidence)
}

// DatabaseMetrics 封装数据库相关的指标收集
type DatabaseMetrics struct{}

func NewDatabaseMetrics() *DatabaseMetrics {
	return &DatabaseMetrics{}
}

func (m *DatabaseMetrics) RecordQuery(operation, table string) {
	dbQueriesTotal.WithLabelValues(operation, table).Inc()
}

func (m *DatabaseMetrics) RecordDuration(operation, table string, duration time.Duration) {
	dbQueryDuration.WithLabelValues(operation, table).Observe(duration.Seconds())
}

func (m *DatabaseMetrics) SetConnections(active, idle int) {
	dbConnectionsActive.Set(float64(active))
	dbConnectionsIdle.Set(float64(idle))
}

// CacheMetrics 封装缓存相关的指标收集
type CacheMetrics struct{}

func NewCacheMetrics() *CacheMetrics {
	return &CacheMetrics{}
}

func (m *CacheMetrics) RecordHit(cacheType string) {
	cacheHitsTotal.WithLabelValues(cacheType).Inc()
}

func (m *CacheMetrics) RecordMiss(cacheType string) {
	cacheMissesTotal.WithLabelValues(cacheType).Inc()
}

func (m *CacheMetrics) SetSize(cacheType string, size float64) {
	cacheSize.WithLabelValues(cacheType).Set(size)
}

// QueueMetrics 封装消息队列相关的指标收集
type QueueMetrics struct{}

func NewQueueMetrics() *QueueMetrics {
	return &QueueMetrics{}
}

func (m *QueueMetrics) RecordPublished(queue string) {
	mqMessagesPublishedTotal.WithLabelValues(queue).Inc()
}

func (m *QueueMetrics) RecordConsumed(queue string) {
	mqMessagesConsumedTotal.WithLabelValues(queue).Inc()
}

func (m *QueueMetrics) RecordFailed(queue string) {
	mqMessagesFailedTotal.WithLabelValues(queue).Inc()
}

func (m *QueueMetrics) SetQueueSize(queue string, size float64) {
	mqQueueSize.WithLabelValues(queue).Set(size)
}

func (m *QueueMetrics) SetConsumerLag(queue string, lag float64) {
	mqConsumerLag.WithLabelValues(queue).Set(lag)
}

// AuthMetrics 封装认证相关的指标收集
type AuthMetrics struct{}

func NewAuthMetrics() *AuthMetrics {
	return &AuthMetrics{}
}

func (m *AuthMetrics) RecordAttempt(method, result string) {
	authAttemptsTotal.WithLabelValues(method, result).Inc()
}

func (m *AuthMetrics) RecordTokenIssued(tokenType string) {
	jwtTokensIssuedTotal.WithLabelValues(tokenType).Inc()
}

// RateLimitMetrics 封装限流相关的指标收集
type RateLimitMetrics struct{}

func NewRateLimitMetrics() *RateLimitMetrics {
	return &RateLimitMetrics{}
}

func (m *RateLimitMetrics) RecordLimited(endpoint string) {
	rateLimitRequestsTotal.WithLabelValues(endpoint).Inc()
}

// SystemMetrics 封装系统相关的指标收集
type SystemMetrics struct{}

func NewSystemMetrics() *SystemMetrics {
	return &SystemMetrics{}
}

func (m *SystemMetrics) SetActiveConnections(count int) {
	activeConnections.Set(float64(count))
}

func (m *SystemMetrics) SetActiveGoroutines(count int) {
	activeGoroutines.Set(float64(count))
}

func (m *SystemMetrics) SetMemoryUsage(bytes uint64) {
	memoryUsage.Set(float64(bytes))
}
