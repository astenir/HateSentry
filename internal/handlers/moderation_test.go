package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func TestModerationHandlerGetResult(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repository := &moderationHandlerRepository{
		stored: moderation.StoredResult{
			Request: models.ModerationRequest{
				RequestID:  "request-123",
				UserID:     42,
				Content:    "stored content",
				Source:     "comment",
				ExternalID: "comment_123",
				ActorID:    "user_456",
				Status:     "completed",
			},
			Result: models.ModerationResult{
				RequestID:     "request-123",
				UserID:        42,
				Provider:      "test-provider",
				Model:         "test-model",
				RawOutput:     `{"risk_score":0.6,"labels":["harassment"],"reason":"Contains abusive language."}`,
				RiskScore:     0.6,
				Labels:        `["harassment"]`,
				Decision:      string(moderation.DecisionReview),
				Reason:        "Contains abusive language.",
				PolicyVersion: "default-v1",
				CreatedAt:     time.Date(2026, 6, 28, 10, 30, 0, 0, time.UTC),
			},
		},
	}
	handler := NewModerationHandler(moderation.NewService(
		moderationHandlerAnalyzer{},
		repository,
		moderation.DefaultPolicy(),
	))

	engine := gin.New()
	engine.GET("/api/v1/moderation/results/:request_id", func(c *gin.Context) {
		c.Set(auth.UserContextKey, &auth.Claims{
			UserID:   42,
			Username: "reviewer",
			Role:     "user",
		})
		handler.GetResult(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/moderation/results/request-123", nil)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "raw_output") {
		t.Fatalf("response leaked raw output: %s", recorder.Body.String())
	}

	var response moderation.ResultOutput
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.RequestID != "request-123" {
		t.Fatalf("RequestID = %q, want request-123", response.RequestID)
	}
	if response.Decision != moderation.DecisionReview {
		t.Fatalf("Decision = %q, want review", response.Decision)
	}
	if response.Provider != "test-provider" {
		t.Fatalf("Provider = %q, want test-provider", response.Provider)
	}
	if repository.userID != 42 {
		t.Fatalf("repository userID = %d, want 42", repository.userID)
	}
	if repository.requestID != "request-123" {
		t.Fatalf("repository requestID = %q, want request-123", repository.requestID)
	}
}

func TestModerationHandlerGetResultRequiresUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewModerationHandler(moderation.NewService(
		moderationHandlerAnalyzer{},
		&moderationHandlerRepository{},
		moderation.DefaultPolicy(),
	))

	engine := gin.New()
	engine.GET("/api/v1/moderation/results/:request_id", handler.GetResult)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/moderation/results/request-123", nil)
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
	request   *models.ModerationRequest
	result    *models.ModerationResult
	stored    moderation.StoredResult
	userID    uint
	requestID string
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

func (r *moderationHandlerRepository) GetResult(
	ctx context.Context,
	userID uint,
	requestID string,
) (moderation.StoredResult, error) {
	r.userID = userID
	r.requestID = requestID
	return r.stored, nil
}
