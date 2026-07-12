package handlers

import (
	"hatesentry/internal/cache"
	"hatesentry/internal/database"
	"hatesentry/internal/queue"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

// HealthHandler handles health check requests
type HealthHandler struct {
	rabbitMQManager *queue.RabbitMQManager
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(rabbitMQManager *queue.RabbitMQManager) *HealthHandler {
	return &HealthHandler{
		rabbitMQManager: rabbitMQManager,
	}
}

// HealthResponse represents health check response
type HealthResponse struct {
	Status    string            `json:"status"`
	Version   string            `json:"version"`
	Timestamp string            `json:"timestamp"`
	Services  map[string]string `json:"services"`
}

// Health performs a health check
func (h *HealthHandler) Health(c *gin.Context) {
	services := make(map[string]string)
	overallStatus := "healthy"

	// Check database
	if err := database.HealthCheck(); err != nil {
		services["database"] = "unhealthy: " + err.Error()
		overallStatus = "unhealthy"
	} else {
		services["database"] = "healthy"
	}

	// Check Redis
	if err := cache.HealthCheck(c.Request.Context()); err != nil {
		services["redis"] = "unhealthy: " + err.Error()
		overallStatus = "unhealthy"
	} else {
		services["redis"] = "healthy"
	}

	// Check RabbitMQ
	if h.rabbitMQManager == nil {
		services["rabbitmq"] = "unhealthy: RabbitMQ manager is not configured"
		overallStatus = "unhealthy"
	} else if err := h.rabbitMQManager.HealthCheck(); err != nil {
		services["rabbitmq"] = "unhealthy: " + err.Error()
		overallStatus = "unhealthy"
	} else {
		services["rabbitmq"] = "healthy"
	}

	// Set appropriate status code
	statusCode := http.StatusOK
	if overallStatus == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, HealthResponse{
		Status:    overallStatus,
		Version:   os.Getenv("APP_VERSION"),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Services:  services,
	})
}
