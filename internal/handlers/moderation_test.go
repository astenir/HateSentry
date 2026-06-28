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
			Role:     "admin",
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

func TestModerationHandlerCheckAcceptsAPIKeyPrincipal(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repository := &moderationHandlerRepository{}
	handler := NewModerationHandler(moderation.NewService(
		moderationHandlerAnalyzer{},
		repository,
		moderation.DefaultPolicy(),
	))

	engine := gin.New()
	engine.POST("/api/v1/moderation/check", func(c *gin.Context) {
		c.Set(auth.APIKeyContextKey, auth.APIKeyPrincipal{
			ClientID: 11,
			UserID:   42,
			Name:     "blog",
		})
		handler.Check(c)
	})

	body := `{"content":"check this text","source":"comment","external_id":"comment_123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/moderation/check", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if repository.request == nil {
		t.Fatal("moderation request was not persisted")
	}
	if repository.request.UserID != 42 {
		t.Fatalf("UserID = %d, want 42", repository.request.UserID)
	}
	if repository.request.ClientID == nil || *repository.request.ClientID != 11 {
		t.Fatalf("ClientID = %#v, want 11", repository.request.ClientID)
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
			Role:     "admin",
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

func TestModerationHandlerListReviewCases(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repository := &moderationHandlerRepository{
		reviewCases: []moderation.StoredReviewCase{
			{
				Case: models.ReviewCase{
					ID:        3,
					RequestID: "request-123",
					UserID:    42,
					Status:    string(moderation.ReviewStatusPending),
					CreatedAt: time.Date(2026, 6, 28, 11, 0, 0, 0, time.UTC),
				},
				Request: models.ModerationRequest{
					RequestID: "request-123",
					UserID:    42,
					Content:   "stored content",
					Source:    "comment",
				},
				Result: models.ModerationResult{
					RequestID:     "request-123",
					UserID:        42,
					RiskScore:     0.6,
					Labels:        `["harassment"]`,
					Decision:      string(moderation.DecisionReview),
					Reason:        "Needs operator review.",
					PolicyVersion: "default-v1",
				},
			},
		},
	}
	handler := NewModerationHandler(moderation.NewService(
		moderationHandlerAnalyzer{},
		repository,
		moderation.DefaultPolicy(),
	))

	engine := gin.New()
	engine.GET("/api/v1/reviews", func(c *gin.Context) {
		c.Set(auth.UserContextKey, &auth.Claims{
			UserID:   42,
			Username: "reviewer",
			Role:     "admin",
		})
		handler.ListReviewCases(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/reviews?status=pending", nil)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "raw_output") {
		t.Fatalf("response leaked raw output: %s", recorder.Body.String())
	}

	var response struct {
		Items []moderation.ReviewCaseOutput `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(response.Items))
	}
	if response.Items[0].ID != 3 {
		t.Fatalf("ID = %d, want 3", response.Items[0].ID)
	}
	if response.Items[0].Status != moderation.ReviewStatusPending {
		t.Fatalf("Status = %q, want pending", response.Items[0].Status)
	}
	if repository.reviewStatus != moderation.ReviewStatusPending {
		t.Fatalf("repository status = %q, want pending", repository.reviewStatus)
	}
}

func TestModerationHandlerApproveReviewCase(t *testing.T) {
	gin.SetMode(gin.TestMode)

	reviewerID := uint(42)
	repository := &moderationHandlerRepository{
		finalized: moderation.StoredReviewCase{
			Case: models.ReviewCase{
				ID:            3,
				RequestID:     "request-123",
				UserID:        42,
				Status:        string(moderation.ReviewStatusApproved),
				ReviewerID:    &reviewerID,
				FinalDecision: string(moderation.DecisionAllow),
				ReviewNotes:   "looks safe",
				CreatedAt:     time.Date(2026, 6, 28, 11, 0, 0, 0, time.UTC),
			},
			Request: models.ModerationRequest{
				RequestID: "request-123",
				UserID:    42,
				Content:   "stored content",
				Source:    "comment",
			},
			Result: models.ModerationResult{
				RequestID:     "request-123",
				UserID:        42,
				RiskScore:     0.6,
				Labels:        `["harassment"]`,
				Decision:      string(moderation.DecisionReview),
				Reason:        "Needs operator review.",
				PolicyVersion: "default-v1",
			},
		},
	}
	handler := NewModerationHandler(moderation.NewService(
		moderationHandlerAnalyzer{},
		repository,
		moderation.DefaultPolicy(),
	))

	engine := gin.New()
	engine.POST("/api/v1/reviews/:id/approve", func(c *gin.Context) {
		c.Set(auth.UserContextKey, &auth.Claims{
			UserID:   42,
			Username: "reviewer",
			Role:     "admin",
		})
		handler.ApproveReviewCase(c)
	})

	body := `{"notes":" looks safe "}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reviews/3/approve", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var response moderation.ReviewCaseOutput
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Status != moderation.ReviewStatusApproved {
		t.Fatalf("Status = %q, want approved", response.Status)
	}
	if response.FinalDecision != moderation.DecisionAllow {
		t.Fatalf("FinalDecision = %q, want allow", response.FinalDecision)
	}
	if repository.caseID != 3 {
		t.Fatalf("caseID = %d, want 3", repository.caseID)
	}
	if repository.reviewerID != 42 {
		t.Fatalf("reviewerID = %d, want 42", repository.reviewerID)
	}
	if repository.notes != "looks safe" {
		t.Fatalf("notes = %q, want trimmed notes", repository.notes)
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
	request       *models.ModerationRequest
	result        *models.ModerationResult
	reviewCase    *models.ReviewCase
	stored        moderation.StoredResult
	reviewCases   []moderation.StoredReviewCase
	finalized     moderation.StoredReviewCase
	userID        uint
	requestID     string
	reviewStatus  moderation.ReviewStatus
	caseID        uint
	reviewerID    uint
	finalStatus   moderation.ReviewStatus
	finalDecision moderation.Decision
	notes         string
	reviewedAt    time.Time
}

func (r *moderationHandlerRepository) SaveCheck(
	ctx context.Context,
	request *models.ModerationRequest,
	result *models.ModerationResult,
	reviewCase *models.ReviewCase,
) error {
	copiedRequest := *request
	copiedResult := *result
	r.request = &copiedRequest
	r.result = &copiedResult
	if reviewCase != nil {
		copiedReviewCase := *reviewCase
		r.reviewCase = &copiedReviewCase
	}
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

func (r *moderationHandlerRepository) FindResultByClientExternalID(
	ctx context.Context,
	clientID uint,
	externalID string,
) (moderation.StoredResult, bool, error) {
	return moderation.StoredResult{}, false, nil
}

func (r *moderationHandlerRepository) ListReviewCases(
	ctx context.Context,
	status moderation.ReviewStatus,
) ([]moderation.StoredReviewCase, error) {
	r.reviewStatus = status
	return r.reviewCases, nil
}

func (r *moderationHandlerRepository) FinalizeReviewCase(
	ctx context.Context,
	caseID uint,
	reviewerID uint,
	status moderation.ReviewStatus,
	finalDecision moderation.Decision,
	notes string,
	reviewedAt time.Time,
) (moderation.StoredReviewCase, error) {
	r.caseID = caseID
	r.reviewerID = reviewerID
	r.finalStatus = status
	r.finalDecision = finalDecision
	r.notes = notes
	r.reviewedAt = reviewedAt
	return r.finalized, nil
}
