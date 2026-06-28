package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"hatesentry/internal/auth"
	"hatesentry/internal/models"
	"hatesentry/internal/moderation"
)

func TestModerationHandlerCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repository := &moderationHandlerRepository{}
	service := moderation.NewService(
		moderationHandlerAnalyzer{},
		repository,
		moderation.DefaultPolicy(),
	)
	handler := NewModerationHandler(service)

	engine := gin.New()
	engine.POST("/api/v1/moderation/check", func(c *gin.Context) {
		c.Set(auth.UserContextKey, &auth.Claims{
			UserID:   42,
			Username: "reviewer",
			Role:     "user",
		})
		handler.Check(c)
	})

	body := `{"content":"check this text","source":"comment","external_id":"comment_123","actor_id":"user_456"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/moderation/check", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "raw_output") {
		t.Fatalf("response leaked raw output: %s", recorder.Body.String())
	}

	var response moderation.CheckOutput
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Decision != moderation.DecisionReview {
		t.Fatalf("Decision = %q, want review", response.Decision)
	}
	if response.RiskScore != 0.6 {
		t.Fatalf("RiskScore = %v, want 0.6", response.RiskScore)
	}
	if response.RequestID == "" {
		t.Fatal("RequestID is empty")
	}
	if repository.request == nil || repository.result == nil {
		t.Fatal("moderation records were not persisted")
	}
	if repository.request.UserID != 42 {
		t.Fatalf("UserID = %d, want 42", repository.request.UserID)
	}
}

func TestModerationHandlerCheckRequiresUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewModerationHandler(moderation.NewService(
		moderationHandlerAnalyzer{},
		&moderationHandlerRepository{},
		moderation.DefaultPolicy(),
	))

	engine := gin.New()
	engine.POST("/api/v1/moderation/check", handler.Check)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/moderation/check",
		strings.NewReader(`{"content":"check this text"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}

type moderationHandlerAnalyzer struct{}

func (a moderationHandlerAnalyzer) AnalyzeText(
	ctx context.Context,
	content string,
) (moderation.ProviderSuggestion, moderation.ProviderInfo, error) {
	return moderation.ProviderSuggestion{
			RiskScore: 0.6,
			Labels:    []string{"harassment"},
			Reason:    "Contains abusive language.",
			RawOutput: `{"risk_score":0.6,"labels":["harassment"],"reason":"Contains abusive language."}`,
		},
		moderation.ProviderInfo{
			Provider: "test-provider",
			Model:    "test-model",
		},
		nil
}

type moderationHandlerRepository struct {
	request *models.ModerationRequest
	result  *models.ModerationResult
}

func (r *moderationHandlerRepository) SaveCheck(
	ctx context.Context,
	request *models.ModerationRequest,
	result *models.ModerationResult,
) error {
	copiedRequest := *request
	copiedResult := *result
	r.request = &copiedRequest
	r.result = &copiedResult
	return nil
}
