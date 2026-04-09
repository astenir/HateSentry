package observability

import (
	"context"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/process"
	"go.uber.org/zap"
)

var (
	// 系统指标收集器
	systemMetrics *SystemMetricsCollector
	once          sync.Once
)

// SystemMetricsCollector 系统指标收集器
type SystemMetricsCollector struct {
	logger        *zap.Logger
	metrics       *SystemMetrics
	stopChan      chan struct{}
	interval      time.Duration
}

// GetSystemMetricsCollector 获取单例系统指标收集器
func GetSystemMetricsCollector(logger *zap.Logger) *SystemMetricsCollector {
	once.Do(func() {
		systemMetrics = &SystemMetricsCollector{
			logger:   logger,
			metrics:  NewSystemMetrics(),
			stopChan: make(chan struct{}),
			interval: 15 * time.Second,
		}
	})
	return systemMetrics
}

// Start 启动系统指标收集
func (smc *SystemMetricsCollector) Start() {
	ticker := time.NewTicker(smc.interval)
	
	smc.logger.Info("Starting system metrics collector", zap.Duration("interval", smc.interval))

	go func() {
		for {
			select {
			case <-ticker.C:
				smc.collectMetrics()
			case <-smc.stopChan:
				ticker.Stop()
				smc.logger.Info("System metrics collector stopped")
				return
			}
		}
	}()
}

// Stop 停止系统指标收集
func (smc *SystemMetricsCollector) Stop() {
	close(smc.stopChan)
}

// collectMetrics 收集系统指标
func (smc *SystemMetricsCollector) collectMetrics() {
	// 收集内存使用情况
	if memInfo, err := mem.VirtualMemory(); err == nil {
		smc.metrics.SetMemoryUsage(memInfo.Used)
		smc.logger.Debug("Memory metrics collected",
			zap.Uint64("total", memInfo.Total),
			zap.Uint64("used", memInfo.Used),
			zap.Float64("percent", memInfo.UsedPercent),
		)
	}

	// 收集活跃 Goroutine 数量
	smc.metrics.SetActiveGoroutines(runtime.NumGoroutine())

	// 收集活跃连接数
	if httpMetrics := getHTTPServerMetrics(); httpMetrics != nil {
		smc.metrics.SetActiveConnections(httpMetrics.ActiveConnections)
	}

	// 收集 CPU 使用率
	if cpuPercent, err := cpu.Percent(time.Second, false); err == nil && len(cpuPercent) > 0 {
		smc.logger.Debug("CPU usage", zap.Float64("percent", cpuPercent[0]))
	}

	// 收集进程信息
	if proc, err := process.NewProcess(int32(os.Getpid())); err == nil {
		if cpuPercent, err := proc.CPUPercent(); err == nil {
			smc.logger.Debug("Process CPU", zap.Float64("percent", cpuPercent))
		}

		if numFDs, err := proc.NumFDs(); err == nil {
			smc.logger.Debug("Open file descriptors", zap.Uint64("count", uint64(numFDs)))
		}

		if threads, err := proc.NumThreads(); err == nil {
			smc.logger.Debug("Thread count", zap.Int32("count", threads))
		}
	}

	// 收集主机信息
	if hostInfo, err := host.Info(); err == nil {
		smc.logger.Debug("Host info",
			zap.String("hostname", hostInfo.Hostname),
			zap.String("os", hostInfo.OS),
			zap.String("platform", hostInfo.Platform),
			zap.String("platform_family", hostInfo.PlatformFamily),
		)
	}
}

// HTTPServerMetrics HTTP 服务器指标
type HTTPServerMetrics struct {
	ActiveConnections int
}

var httpServerMetrics *HTTPServerMetrics

func getHTTPServerMetrics() *HTTPServerMetrics {
	return httpServerMetrics
}

func SetHTTPServerMetrics(metrics *HTTPServerMetrics) {
	httpServerMetrics = metrics
}

// HealthCheckHandler 健康检查处理器
type HealthCheckHandler struct {
	db          HealthChecker
	redis       HealthChecker
	rabbitmq    HealthChecker
	logger      *zap.Logger
}

type HealthChecker interface {
	Check(ctx context.Context) error
}

func NewHealthCheckHandler(db, redis, rabbitmq HealthChecker, logger *zap.Logger) *HealthCheckHandler {
	return &HealthCheckHandler{
		db:       db,
		redis:    redis,
		rabbitmq: rabbitmq,
		logger:   logger,
	}
}

func (h *HealthCheckHandler) Check(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	status := map[string]interface{}{
		"status":  "healthy",
		"checks":  make(map[string]interface{}),
		"version": os.Getenv("APP_VERSION"),
	}

	allHealthy := true

	// 检查数据库
	if h.db != nil {
		dbStatus := map[string]interface{}{
			"status": "unknown",
		}
		if err := h.db.Check(ctx); err != nil {
			dbStatus["status"] = "unhealthy"
			dbStatus["error"] = err.Error()
			allHealthy = false
			h.logger.Error("Database health check failed", zap.Error(err))
		} else {
			dbStatus["status"] = "healthy"
		}
		status["checks"].(map[string]interface{})["database"] = dbStatus
	}

	// 检查 Redis
	if h.redis != nil {
		redisStatus := map[string]interface{}{
			"status": "unknown",
		}
		if err := h.redis.Check(ctx); err != nil {
			redisStatus["status"] = "unhealthy"
			redisStatus["error"] = err.Error()
			allHealthy = false
			h.logger.Error("Redis health check failed", zap.Error(err))
		} else {
			redisStatus["status"] = "healthy"
		}
		status["checks"].(map[string]interface{})["redis"] = redisStatus
	}

	// 检查 RabbitMQ
	if h.rabbitmq != nil {
		mqStatus := map[string]interface{}{
			"status": "unknown",
		}
		if err := h.rabbitmq.Check(ctx); err != nil {
			mqStatus["status"] = "unhealthy"
			mqStatus["error"] = err.Error()
			allHealthy = false
			h.logger.Error("RabbitMQ health check failed", zap.Error(err))
		} else {
			mqStatus["status"] = "healthy"
		}
		status["checks"].(map[string]interface{})["rabbitmq"] = mqStatus
	}

	// 系统信息
	if memInfo, err := mem.VirtualMemory(); err == nil {
		systemInfo := map[string]interface{}{
			"memory_percent": memInfo.UsedPercent,
			"memory_total":   memInfo.Total,
			"memory_used":    memInfo.Used,
			"goroutines":     runtime.NumGoroutine(),
		}
		status["checks"].(map[string]interface{})["system"] = systemInfo
	}

	if !allHealthy {
		status["status"] = "unhealthy"
		c.JSON(http.StatusServiceUnavailable, status)
		return
	}

	c.JSON(http.StatusOK, status)
}

// ReadinessHandler 就绪检查处理器
func (h *HealthCheckHandler) Readiness(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	status := map[string]interface{}{
		"status":  "ready",
		"checks":  make(map[string]interface{}),
	}

	allReady := true

	// 检查数据库（核心依赖）
	if h.db != nil {
		if err := h.db.Check(ctx); err != nil {
			status["checks"].(map[string]interface{})["database"] = "not ready"
			allReady = false
		} else {
			status["checks"].(map[string]interface{})["database"] = "ready"
		}
	}

	if !allReady {
		status["status"] = "not ready"
		c.JSON(http.StatusServiceUnavailable, status)
		return
	}

	c.JSON(http.StatusOK, status)
}

// LivenessHandler 存活检查处理器
func (h *HealthCheckHandler) Liveness(c *gin.Context) {
	c.JSON(http.StatusOK, map[string]interface{}{
		"status": "alive",
	})
}
