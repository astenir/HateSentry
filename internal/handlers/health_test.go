package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHealthReportsMissingRabbitMQManager(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("APP_VERSION", "0.2.0")

	handler := NewHealthHandler(nil)
	engine := gin.New()
	engine.GET("/api/v1/health", handler.Health)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503, body = %s", recorder.Code, recorder.Body.String())
	}

	var response HealthResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Status != "unhealthy" {
		t.Fatalf("Status = %q, want unhealthy", response.Status)
	}
	if response.Version != "0.2.0" {
		t.Fatalf("Version = %q, want 0.2.0", response.Version)
	}
	rabbitStatus := response.Services["rabbitmq"]
	if !strings.Contains(rabbitStatus, "RabbitMQ manager is not configured") {
		t.Fatalf("rabbitmq status = %q, want missing manager detail", rabbitStatus)
	}
}
