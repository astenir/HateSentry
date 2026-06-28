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
	apperrors "hatesentry/internal/errors"
	"hatesentry/internal/models"
	"hatesentry/internal/moderation"
	"hatesentry/internal/webhooks"
)

func TestModerationHandlerCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repository := &moderationHandlerRepository{
		webhookClient: models.ClientApplication{
			ID:     11,
			UserID: 42,
			Name:   "blog",
		},
		webhookClientFound: true,
	}
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

	repository := &moderationHandlerRepository{
		webhookClient: models.ClientApplication{
			ID:     11,
			UserID: 42,
			Name:   "blog",
		},
		webhookClientFound: true,
	}
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

func TestModerationHandlerListPolicies(t *testing.T) {
	gin.SetMode(gin.TestMode)

	strictPolicy, err := moderation.NewPolicy("strict-v1", 0.2, 0.5)
	if err != nil {
		t.Fatalf("NewPolicy() error = %v", err)
	}
	policies, err := moderation.NewPolicySet(moderation.DefaultPolicy(), strictPolicy)
	if err != nil {
		t.Fatalf("NewPolicySet() error = %v", err)
	}
	handler := NewModerationHandler(moderation.NewServiceWithPolicySet(
		nil,
		nil,
		policies,
	))

	engine := gin.New()
	engine.GET("/api/v1/admin/moderation/policies", func(c *gin.Context) {
		c.Set(auth.UserContextKey, &auth.Claims{
			UserID:   42,
			Username: "reviewer",
			Role:     "admin",
		})
		handler.ListPolicies(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/moderation/policies", nil)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var response moderation.ListPoliciesOutput
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(response.Items))
	}
	if response.Items[0].Version != "default-v1" || !response.Items[0].Default {
		t.Fatalf("first policy = %#v, want default-v1 default", response.Items[0])
	}
	if response.Items[1].Version != "strict-v1" || response.Items[1].Default {
		t.Fatalf("second policy = %#v, want strict-v1 non-default", response.Items[1])
	}
}

func TestModerationHandlerListPoliciesRequiresUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewModerationHandler(moderation.NewService(
		nil,
		nil,
		moderation.DefaultPolicy(),
	))

	engine := gin.New()
	engine.GET("/api/v1/admin/moderation/policies", handler.ListPolicies)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/moderation/policies", nil)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}

func TestModerationHandlerListHistory(t *testing.T) {
	gin.SetMode(gin.TestMode)

	clientID := uint(11)
	repository := &moderationHandlerRepository{
		historyItems: []moderation.StoredHistoryItem{
			{
				Request: models.ModerationRequest{
					RequestID:  "request-123",
					UserID:     42,
					ClientID:   &clientID,
					Content:    "stored content",
					Source:     "comment",
					ExternalID: "comment_123",
					ActorID:    "user_456",
					Status:     "completed",
				},
				Result: models.ModerationResult{
					RequestID:     "request-123",
					UserID:        42,
					ClientID:      &clientID,
					Provider:      "test-provider",
					Model:         "test-model",
					RawOutput:     `{"risk_score":0.6,"labels":["harassment"],"reason":"Contains abusive language."}`,
					RiskScore:     0.6,
					Labels:        `["harassment"]`,
					Decision:      string(moderation.DecisionReview),
					Reason:        "Needs operator review.",
					PolicyVersion: "default-v1",
					CreatedAt:     time.Date(2026, 6, 28, 10, 30, 0, 0, time.UTC),
				},
				ReviewCase: &models.ReviewCase{
					RequestID:     "request-123",
					Status:        string(moderation.ReviewStatusApproved),
					FinalDecision: string(moderation.DecisionAllow),
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
	engine.GET("/api/v1/admin/moderation/results", func(c *gin.Context) {
		c.Set(auth.UserContextKey, &auth.Claims{
			UserID:   42,
			Username: "reviewer",
			Role:     "admin",
		})
		handler.ListHistory(c)
	})

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/admin/moderation/results?decision=review&client_id=11&external_id=comment_123&limit=10",
		nil,
	)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "raw_output") {
		t.Fatalf("response leaked raw output: %s", recorder.Body.String())
	}

	var response moderation.ListHistoryOutput
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(response.Items))
	}
	if response.Items[0].PolicyDecision != moderation.DecisionReview {
		t.Fatalf("PolicyDecision = %q, want review", response.Items[0].PolicyDecision)
	}
	if response.Items[0].FinalDecision != moderation.DecisionAllow {
		t.Fatalf("FinalDecision = %q, want allow", response.Items[0].FinalDecision)
	}
	if repository.historyFilter.Decision != moderation.DecisionReview {
		t.Fatalf("decision filter = %q, want review", repository.historyFilter.Decision)
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

func TestModerationHandlerGetReviewCase(t *testing.T) {
	gin.SetMode(gin.TestMode)

	reviewerID := uint(42)
	repository := &moderationHandlerRepository{
		reviewCaseStored: moderation.StoredReviewCase{
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
	engine.GET("/api/v1/reviews/:id", func(c *gin.Context) {
		c.Set(auth.UserContextKey, &auth.Claims{
			UserID:   42,
			Username: "reviewer",
			Role:     "admin",
		})
		handler.GetReviewCase(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/reviews/3", nil)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "raw_output") {
		t.Fatalf("response leaked raw output: %s", recorder.Body.String())
	}

	var response moderation.ReviewCaseOutput
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ID != 3 {
		t.Fatalf("ID = %d, want 3", response.ID)
	}
	if response.Status != moderation.ReviewStatusApproved {
		t.Fatalf("Status = %q, want approved", response.Status)
	}
	if response.FinalDecision != moderation.DecisionAllow {
		t.Fatalf("FinalDecision = %q, want allow", response.FinalDecision)
	}
	if repository.caseID != 3 {
		t.Fatalf("repository caseID = %d, want 3", repository.caseID)
	}
}

func TestModerationHandlerGetReviewStats(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repository := &moderationHandlerRepository{
		stats: moderation.StoredStats{
			TotalModerated:     10,
			PolicyAllowed:      2,
			PolicyBlocked:      3,
			ReviewFinalAllowed: 1,
			ReviewFinalBlocked: 2,
			PendingReview:      1,
			Reviewed:           3,
			Mistakes:           1,
		},
	}
	handler := NewModerationHandler(moderation.NewService(
		moderationHandlerAnalyzer{},
		repository,
		moderation.DefaultPolicy(),
	))

	engine := gin.New()
	engine.GET("/api/v1/reviews/stats", func(c *gin.Context) {
		c.Set(auth.UserContextKey, &auth.Claims{
			UserID:   42,
			Username: "reviewer",
			Role:     "admin",
		})
		handler.GetReviewStats(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/reviews/stats", nil)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var response moderation.StatsOutput
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.TotalModerated != 10 {
		t.Fatalf("TotalModerated = %d, want 10", response.TotalModerated)
	}
	if response.Allowed != 3 {
		t.Fatalf("Allowed = %d, want 3", response.Allowed)
	}
	if response.Blocked != 5 {
		t.Fatalf("Blocked = %d, want 5", response.Blocked)
	}
	if response.PendingReview != 1 {
		t.Fatalf("PendingReview = %d, want 1", response.PendingReview)
	}
	if response.MistakeRate != float64(1)/float64(3) {
		t.Fatalf("MistakeRate = %v, want %v", response.MistakeRate, float64(1)/float64(3))
	}
}

func TestModerationHandlerListWebhookDeliveries(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repository := &moderationHandlerRepository{
		webhookDeliveries: []models.WebhookDelivery{
			{
				ID:            5,
				DeliveryID:    "delivery-123",
				RequestID:     "request-123",
				ClientID:      11,
				Event:         "moderation.final_decision",
				Status:        string(moderation.WebhookDeliveryFailed),
				AttemptCount:  2,
				LastAttemptAt: time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC),
			},
		},
	}
	handler := NewModerationHandler(moderation.NewService(
		moderationHandlerAnalyzer{},
		repository,
		moderation.DefaultPolicy(),
	))

	engine := gin.New()
	engine.GET("/api/v1/admin/webhook-deliveries", func(c *gin.Context) {
		c.Set(auth.UserContextKey, &auth.Claims{
			UserID:   42,
			Username: "reviewer",
			Role:     "admin",
		})
		handler.ListWebhookDeliveries(c)
	})

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/admin/webhook-deliveries?status=failed&client_id=11&request_id=request-123&limit=10",
		nil,
	)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var response moderation.ListWebhookDeliveriesOutput
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(response.Items))
	}
	if response.Items[0].ID != 5 {
		t.Fatalf("item id = %d, want 5", response.Items[0].ID)
	}
	if repository.webhookDeliveryFilter.Status != moderation.WebhookDeliveryFailed {
		t.Fatalf("status filter = %q, want failed", repository.webhookDeliveryFilter.Status)
	}
	if repository.webhookDeliveryFilter.ClientID == nil || *repository.webhookDeliveryFilter.ClientID != 11 {
		t.Fatalf("client id filter = %#v, want 11", repository.webhookDeliveryFilter.ClientID)
	}
	if repository.webhookDeliveryFilter.RequestID != "request-123" {
		t.Fatalf("request id filter = %q, want request-123", repository.webhookDeliveryFilter.RequestID)
	}
	if repository.webhookDeliveryFilter.Limit != 10 {
		t.Fatalf("limit filter = %d, want 10", repository.webhookDeliveryFilter.Limit)
	}
}

func TestModerationHandlerGetWebhookDelivery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	httpStatus := 500
	repository := &moderationHandlerRepository{
		webhookDelivery: models.WebhookDelivery{
			ID:            5,
			DeliveryID:    "delivery-123",
			RequestID:     "request-123",
			ClientID:      11,
			Event:         "moderation.final_decision",
			Status:        string(moderation.WebhookDeliveryFailed),
			AttemptCount:  2,
			LastAttemptAt: time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC),
			HTTPStatus:    &httpStatus,
			ErrorMessage:  "webhook returned status 500",
		},
	}
	handler := NewModerationHandler(moderation.NewService(
		moderationHandlerAnalyzer{},
		repository,
		moderation.DefaultPolicy(),
	))

	engine := gin.New()
	engine.GET("/api/v1/admin/webhook-deliveries/:id", func(c *gin.Context) {
		c.Set(auth.UserContextKey, &auth.Claims{
			UserID:   42,
			Username: "reviewer",
			Role:     "admin",
		})
		handler.GetWebhookDelivery(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/webhook-deliveries/5", nil)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var response moderation.WebhookDeliveryOutput
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ID != 5 {
		t.Fatalf("ID = %d, want 5", response.ID)
	}
	if response.Status != moderation.WebhookDeliveryFailed {
		t.Fatalf("Status = %q, want failed", response.Status)
	}
	if response.HTTPStatus == nil || *response.HTTPStatus != 500 {
		t.Fatalf("HTTPStatus = %#v, want 500", response.HTTPStatus)
	}
	if repository.webhookDeliveryID != 5 {
		t.Fatalf("webhook delivery id = %d, want 5", repository.webhookDeliveryID)
	}
}

func TestModerationHandlerGetWebhookDeliveryErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		path       string
		repository *moderationHandlerRepository
		wantStatus int
	}{
		{
			name:       "invalid id",
			path:       "/api/v1/admin/webhook-deliveries/abc",
			repository: &moderationHandlerRepository{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "missing record",
			path: "/api/v1/admin/webhook-deliveries/999",
			repository: &moderationHandlerRepository{
				err: apperrors.RecordNotFound("Webhook delivery not found"),
			},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewModerationHandler(moderation.NewService(
				moderationHandlerAnalyzer{},
				tt.repository,
				moderation.DefaultPolicy(),
			))

			engine := gin.New()
			engine.GET("/api/v1/admin/webhook-deliveries/:id", func(c *gin.Context) {
				c.Set(auth.UserContextKey, &auth.Claims{
					UserID:   42,
					Username: "reviewer",
					Role:     "admin",
				})
				handler.GetWebhookDelivery(c)
			})

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			recorder := httptest.NewRecorder()

			engine.ServeHTTP(recorder, req)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, body = %s, want %d", recorder.Code, recorder.Body.String(), tt.wantStatus)
			}
		})
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

func TestModerationHandlerRetryWebhookDelivery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repository := &moderationHandlerRepository{
		webhookClient: models.ClientApplication{
			ID:            11,
			WebhookURL:    "https://example.com/moderation/webhook",
			WebhookSecret: "whsec_test",
		},
		webhookClientFound: true,
		webhookDelivery: models.WebhookDelivery{
			ID:            5,
			DeliveryID:    "delivery-123",
			RequestID:     "request-123",
			ClientID:      11,
			Event:         "moderation.final_decision",
			Status:        string(moderation.WebhookDeliveryFailed),
			AttemptCount:  1,
			LastAttemptAt: time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC),
			Payload:       `{"event":"moderation.final_decision","request_id":"request-123","client_id":11,"source":"comment","decision":"block","risk_score":0.8,"labels":["hate"],"reason":"Policy threshold exceeded.","policy_version":"default-v1","created_at":"2026-06-28T12:00:00Z"}`,
		},
	}
	handler := NewModerationHandler(moderation.NewService(
		moderationHandlerAnalyzer{},
		repository,
		moderation.DefaultPolicy(),
		&moderationHandlerWebhookDispatcher{},
	))

	engine := gin.New()
	engine.POST("/api/v1/admin/webhook-deliveries/:id/retry", func(c *gin.Context) {
		c.Set(auth.UserContextKey, &auth.Claims{
			UserID:   42,
			Username: "reviewer",
			Role:     "admin",
		})
		handler.RetryWebhookDelivery(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/webhook-deliveries/5/retry", nil)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var response moderation.WebhookDeliveryOutput
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Status != moderation.WebhookDeliverySucceeded {
		t.Fatalf("Status = %q, want succeeded", response.Status)
	}
	if response.AttemptCount != 2 {
		t.Fatalf("AttemptCount = %d, want 2", response.AttemptCount)
	}
	if repository.webhookDeliveryID != 5 {
		t.Fatalf("webhook delivery id = %d, want 5", repository.webhookDeliveryID)
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
	request               *models.ModerationRequest
	result                *models.ModerationResult
	reviewCase            *models.ReviewCase
	webhookClient         models.ClientApplication
	webhookDelivery       models.WebhookDelivery
	webhookDeliveries     []models.WebhookDelivery
	stored                moderation.StoredResult
	historyItems          []moderation.StoredHistoryItem
	historyFilter         moderation.HistoryFilter
	reviewCases           []moderation.StoredReviewCase
	reviewCaseStored      moderation.StoredReviewCase
	finalized             moderation.StoredReviewCase
	stats                 moderation.StoredStats
	userID                uint
	requestID             string
	reviewStatus          moderation.ReviewStatus
	caseID                uint
	webhookClientFound    bool
	webhookDeliveryID     uint
	webhookDeliveryFilter moderation.WebhookDeliveryFilter
	webhookDeliveryStatus moderation.WebhookDeliveryStatus
	reviewerID            uint
	finalStatus           moderation.ReviewStatus
	finalDecision         moderation.Decision
	notes                 string
	reviewedAt            time.Time
	err                   error
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

func (r *moderationHandlerRepository) ListHistory(
	ctx context.Context,
	filter moderation.HistoryFilter,
) ([]moderation.StoredHistoryItem, error) {
	r.historyFilter = filter
	return r.historyItems, nil
}

func (r *moderationHandlerRepository) GetClient(
	ctx context.Context,
	clientID uint,
) (models.ClientApplication, bool, error) {
	return r.webhookClient, r.webhookClientFound, nil
}

func (r *moderationHandlerRepository) SaveWebhookDelivery(
	ctx context.Context,
	delivery *models.WebhookDelivery,
) error {
	copiedDelivery := *delivery
	r.webhookDelivery = copiedDelivery
	return nil
}

func (r *moderationHandlerRepository) GetWebhookDelivery(
	ctx context.Context,
	deliveryID uint,
) (models.WebhookDelivery, error) {
	if r.err != nil {
		return models.WebhookDelivery{}, r.err
	}
	r.webhookDeliveryID = deliveryID
	return r.webhookDelivery, nil
}

func (r *moderationHandlerRepository) ListWebhookDeliveries(
	ctx context.Context,
	filter moderation.WebhookDeliveryFilter,
) ([]models.WebhookDelivery, error) {
	r.webhookDeliveryFilter = filter
	return r.webhookDeliveries, nil
}

func (r *moderationHandlerRepository) ClaimFailedWebhookDelivery(
	ctx context.Context,
	deliveryID uint,
	attemptedAt time.Time,
) (models.WebhookDelivery, error) {
	r.webhookDeliveryID = deliveryID
	claimed := r.webhookDelivery
	claimed.Status = string(moderation.WebhookDeliveryRetrying)
	claimed.LastAttemptAt = attemptedAt
	return claimed, nil
}

func (r *moderationHandlerRepository) UpdateWebhookDeliveryAttempt(
	ctx context.Context,
	deliveryID uint,
	status moderation.WebhookDeliveryStatus,
	httpStatus *int,
	errorMessage string,
	attemptedAt time.Time,
) (models.WebhookDelivery, error) {
	r.webhookDeliveryID = deliveryID
	r.webhookDeliveryStatus = status

	updated := r.webhookDelivery
	updated.Status = string(status)
	updated.AttemptCount++
	updated.LastAttemptAt = attemptedAt
	updated.HTTPStatus = httpStatus
	updated.ErrorMessage = errorMessage
	return updated, nil
}

func (r *moderationHandlerRepository) ListReviewCases(
	ctx context.Context,
	status moderation.ReviewStatus,
) ([]moderation.StoredReviewCase, error) {
	r.reviewStatus = status
	return r.reviewCases, nil
}

func (r *moderationHandlerRepository) GetReviewCase(
	ctx context.Context,
	caseID uint,
) (moderation.StoredReviewCase, error) {
	r.caseID = caseID
	return r.reviewCaseStored, nil
}

func (r *moderationHandlerRepository) GetStats(ctx context.Context) (moderation.StoredStats, error) {
	return r.stats, nil
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

type moderationHandlerWebhookDispatcher struct{}

func (d *moderationHandlerWebhookDispatcher) DispatchFinalDecision(
	ctx context.Context,
	client models.ClientApplication,
	payload webhooks.FinalDecisionPayload,
) error {
	return nil
}
