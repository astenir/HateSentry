package moderation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	apperrors "hatesentry/internal/errors"
	"hatesentry/internal/models"
	"hatesentry/internal/webhooks"

	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
)

func TestServiceCheckPersistsDecision(t *testing.T) {
	repository := &fakeRepository{}
	service := NewService(
		fakeAnalyzer{
			suggestion: ProviderSuggestion{
				RiskScore: 0.82,
				Labels:    []string{"harassment", "identity_attack"},
				Reason:    "Contains targeted abuse.",
				RawOutput: `{"risk_score":0.82,"labels":["harassment","identity_attack"],"reason":"Contains targeted abuse."}`,
			},
			provider: ProviderInfo{
				Provider: "test-provider",
				Model:    "test-model",
			},
		},
		repository,
		DefaultPolicy(),
	)

	output, err := service.Check(context.Background(), CheckInput{
		UserID:     7,
		Content:    " user submitted text ",
		Source:     "comment",
		ExternalID: "comment_123",
		ActorID:    "user_456",
	})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if output.RequestID == "" {
		t.Fatal("RequestID is empty")
	}
	if output.Decision != DecisionBlock {
		t.Fatalf("Decision = %q, want %q", output.Decision, DecisionBlock)
	}
	if !equalStrings(output.Labels, []string{"harassment", "identity_attack"}) {
		t.Fatalf("Labels = %#v, want harassment and identity_attack", output.Labels)
	}
	if output.PolicyVersion != "default-v1" {
		t.Fatalf("PolicyVersion = %q, want default-v1", output.PolicyVersion)
	}

	if repository.request == nil {
		t.Fatal("request was not persisted")
	}
	if repository.result == nil {
		t.Fatal("result was not persisted")
	}
	if repository.reviewCase != nil {
		t.Fatal("review case was created for a block decision")
	}
	if repository.request.RequestID != output.RequestID {
		t.Fatalf("persisted request id = %q, want %q", repository.request.RequestID, output.RequestID)
	}
	if repository.request.Content != "user submitted text" {
		t.Fatalf("persisted content = %q, want trimmed content", repository.request.Content)
	}
	if repository.request.Source != "comment" {
		t.Fatalf("persisted source = %q, want comment", repository.request.Source)
	}
	if repository.result.Provider != "test-provider" {
		t.Fatalf("provider = %q, want test-provider", repository.result.Provider)
	}
	if repository.result.Model != "test-model" {
		t.Fatalf("model = %q, want test-model", repository.result.Model)
	}
	if repository.result.RawOutput == "" {
		t.Fatal("raw provider output was not persisted")
	}

	var persistedLabels []string
	if err := json.Unmarshal([]byte(repository.result.Labels), &persistedLabels); err != nil {
		t.Fatalf("decode persisted labels: %v", err)
	}
	if !equalStrings(persistedLabels, output.Labels) {
		t.Fatalf("persisted labels = %#v, want %#v", persistedLabels, output.Labels)
	}
}

func TestServiceCheckRecordsModerationMetrics(t *testing.T) {
	labels := map[string]string{
		"decision":    string(DecisionBlock),
		"provider":    "metric-test-provider",
		"client_type": "operator",
	}
	before := prometheusCounterValue(t, "moderation_checks_total", labels)

	service := NewService(
		fakeAnalyzer{
			suggestion: ProviderSuggestion{
				RiskScore: 0.82,
				Labels:    []string{"harassment"},
				Reason:    "Policy threshold exceeded.",
				RawOutput: `{"risk_score":0.82,"labels":["harassment"],"reason":"Policy threshold exceeded."}`,
			},
			provider: ProviderInfo{
				Provider: "metric-test-provider",
				Model:    "test-model",
			},
		},
		&fakeRepository{},
		DefaultPolicy(),
	)

	if _, err := service.Check(context.Background(), CheckInput{
		UserID:  7,
		Content: "blocked content",
	}); err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	after := prometheusCounterValue(t, "moderation_checks_total", labels)
	if after != before+1 {
		t.Fatalf("moderation_checks_total increment = %v, want %v", after-before, 1.0)
	}
}

func TestServiceCheckPersistsClientID(t *testing.T) {
	repository := &fakeRepository{
		webhookClient: models.ClientApplication{
			ID:     11,
			UserID: 7,
			Name:   "blog-comments",
		},
		webhookClientFound: true,
	}
	service := NewService(
		fakeAnalyzer{
			suggestion: ProviderSuggestion{
				RiskScore: 0.6,
				Labels:    []string{"harassment"},
				Reason:    "Needs operator review.",
				RawOutput: `{"risk_score":0.6,"labels":["harassment"],"reason":"Needs operator review."}`,
			},
			provider: ProviderInfo{Provider: "test-provider", Model: "test-model"},
		},
		repository,
		DefaultPolicy(),
	)

	output, err := service.Check(context.Background(), CheckInput{
		UserID:     7,
		ClientID:   11,
		Content:    "review this text",
		ExternalID: "comment_123",
	})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if output.Decision != DecisionReview {
		t.Fatalf("Decision = %q, want review", output.Decision)
	}
	if output.ReviewStatus != string(ReviewStatusPending) {
		t.Fatalf("ReviewStatus = %q, want pending", output.ReviewStatus)
	}
	if repository.request.ClientID == nil || *repository.request.ClientID != 11 {
		t.Fatalf("request ClientID = %#v, want 11", repository.request.ClientID)
	}
	if repository.request.IdempotencyKey == nil || *repository.request.IdempotencyKey != "11:comment_123" {
		t.Fatalf("request IdempotencyKey = %#v, want 11:comment_123", repository.request.IdempotencyKey)
	}
	if repository.result.ClientID == nil || *repository.result.ClientID != 11 {
		t.Fatalf("result ClientID = %#v, want 11", repository.result.ClientID)
	}
	if repository.reviewCase == nil {
		t.Fatal("review case was not created")
	}
	if repository.reviewCase.ClientID == nil || *repository.reviewCase.ClientID != 11 {
		t.Fatalf("review case ClientID = %#v, want 11", repository.reviewCase.ClientID)
	}
}

func TestServiceCheckReturnsExistingClientExternalIDResult(t *testing.T) {
	analyzerCalls := 0
	service := NewService(
		fakeAnalyzer{calls: &analyzerCalls},
		&fakeRepository{
			clientResultFound: true,
			clientStored: StoredResult{
				Request: models.ModerationRequest{
					RequestID:  "request-123",
					UserID:     7,
					ClientID:   uintPtr(11),
					Content:    "stored content",
					ExternalID: "comment_123",
					Source:     "comment",
					Status:     "completed",
				},
				Result: models.ModerationResult{
					RequestID:     "request-123",
					UserID:        7,
					ClientID:      uintPtr(11),
					RiskScore:     0.6,
					Labels:        `["harassment"]`,
					Decision:      string(DecisionReview),
					Reason:        "Needs operator review.",
					PolicyVersion: "default-v1",
				},
			},
		},
		DefaultPolicy(),
	)

	output, err := service.Check(context.Background(), CheckInput{
		UserID:     7,
		ClientID:   11,
		Content:    "new body should not be analyzed",
		ExternalID: " comment_123 ",
	})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if output.RequestID != "request-123" {
		t.Fatalf("RequestID = %q, want existing request", output.RequestID)
	}
	if output.Decision != DecisionReview {
		t.Fatalf("Decision = %q, want review", output.Decision)
	}
	if analyzerCalls != 0 {
		t.Fatalf("analyzer calls = %d, want 0", analyzerCalls)
	}
}

func TestServiceCheckReturnsExistingClientExternalIDReviewState(t *testing.T) {
	reviewedAt := time.Date(2026, 6, 28, 9, 30, 0, 0, time.UTC)
	analyzerCalls := 0
	service := NewService(
		fakeAnalyzer{calls: &analyzerCalls},
		&fakeRepository{
			clientResultFound: true,
			clientStored: StoredResult{
				Request: models.ModerationRequest{
					RequestID:  "request-123",
					UserID:     7,
					ClientID:   uintPtr(11),
					Content:    "stored content",
					ExternalID: "comment_123",
					Source:     "comment",
					Status:     "completed",
				},
				Result: models.ModerationResult{
					RequestID:     "request-123",
					UserID:        7,
					ClientID:      uintPtr(11),
					RiskScore:     0.6,
					Labels:        `["harassment"]`,
					Decision:      string(DecisionReview),
					Reason:        "Needs operator review.",
					PolicyVersion: "default-v1",
				},
				ReviewCase: &models.ReviewCase{
					RequestID:     "request-123",
					UserID:        7,
					ClientID:      uintPtr(11),
					Status:        string(ReviewStatusApproved),
					FinalDecision: string(DecisionAllow),
					ReviewedAt:    &reviewedAt,
				},
			},
		},
		DefaultPolicy(),
	)

	output, err := service.Check(context.Background(), CheckInput{
		UserID:     7,
		ClientID:   11,
		Content:    "retry after human review",
		ExternalID: "comment_123",
	})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if output.Decision != DecisionReview {
		t.Fatalf("Decision = %q, want original policy review", output.Decision)
	}
	if output.ReviewStatus != string(ReviewStatusApproved) {
		t.Fatalf("ReviewStatus = %q, want approved", output.ReviewStatus)
	}
	if output.FinalDecision != string(DecisionAllow) {
		t.Fatalf("FinalDecision = %q, want allow", output.FinalDecision)
	}
	if output.ReviewedAt == nil || !output.ReviewedAt.Equal(reviewedAt) {
		t.Fatalf("ReviewedAt = %v, want %v", output.ReviewedAt, reviewedAt)
	}
	if analyzerCalls != 0 {
		t.Fatalf("analyzer calls = %d, want 0", analyzerCalls)
	}
}

func TestServiceCheckReturnsExistingResultAfterDuplicateIdempotencySave(t *testing.T) {
	analyzerCalls := 0
	repository := &fakeRepository{
		saveErr: apperrors.Conflict("Moderation request already exists for this client external_id"),
		clientStored: StoredResult{
			Request: models.ModerationRequest{
				RequestID:  "request-123",
				UserID:     7,
				ClientID:   uintPtr(11),
				Content:    "stored content",
				ExternalID: "comment_123",
				Source:     "comment",
				Status:     "completed",
			},
			Result: models.ModerationResult{
				RequestID:     "request-123",
				UserID:        7,
				ClientID:      uintPtr(11),
				RiskScore:     0.8,
				Labels:        `["hate"]`,
				Decision:      string(DecisionBlock),
				Reason:        "Policy threshold exceeded.",
				PolicyVersion: "default-v1",
			},
		},
		clientResultFoundAfterSave: true,
		webhookClient: models.ClientApplication{
			ID:     11,
			UserID: 7,
			Name:   "blog-comments",
		},
		webhookClientFound: true,
	}
	service := NewService(
		fakeAnalyzer{
			calls: &analyzerCalls,
			suggestion: ProviderSuggestion{
				RiskScore: 0.8,
				Labels:    []string{"hate"},
				Reason:    "Policy threshold exceeded.",
			},
		},
		repository,
		DefaultPolicy(),
	)

	output, err := service.Check(context.Background(), CheckInput{
		UserID:     7,
		ClientID:   11,
		Content:    "new body lost duplicate insert race",
		ExternalID: "comment_123",
	})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if output.RequestID != "request-123" {
		t.Fatalf("RequestID = %q, want existing request", output.RequestID)
	}
	if output.Decision != DecisionBlock {
		t.Fatalf("Decision = %q, want block", output.Decision)
	}
	if analyzerCalls != 1 {
		t.Fatalf("analyzer calls = %d, want 1 before duplicate save conflict", analyzerCalls)
	}
	if repository.findClientExternalIDCalls != 2 {
		t.Fatalf("FindResultByClientExternalID calls = %d, want 2", repository.findClientExternalIDCalls)
	}
}

func TestServiceCheckDispatchesWebhookForAutomaticFinalDecision(t *testing.T) {
	dispatcher := &fakeWebhookDispatcher{}
	repository := &fakeRepository{
		webhookClient: models.ClientApplication{
			ID:            11,
			UserID:        7,
			WebhookURL:    "https://example.com/moderation/webhook",
			WebhookSecret: "whsec_test",
		},
		webhookClientFound: true,
	}
	service := NewService(
		fakeAnalyzer{
			suggestion: ProviderSuggestion{
				RiskScore: 0.8,
				Labels:    []string{"hate"},
				Reason:    "Policy threshold exceeded.",
			},
		},
		repository,
		DefaultPolicy(),
		dispatcher,
	)

	output, err := service.Check(context.Background(), CheckInput{
		UserID:     7,
		ClientID:   11,
		Content:    "block this text",
		ExternalID: "comment_123",
		ActorID:    "user_456",
		Source:     "comment",
	})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if output.Decision != DecisionBlock {
		t.Fatalf("Decision = %q, want block", output.Decision)
	}
	if len(dispatcher.payloads) != 1 {
		t.Fatalf("webhook dispatches = %d, want 1", len(dispatcher.payloads))
	}
	payload := dispatcher.payloads[0]
	if payload.RequestID != output.RequestID {
		t.Fatalf("RequestID = %q, want %q", payload.RequestID, output.RequestID)
	}
	if payload.Decision != string(DecisionBlock) {
		t.Fatalf("Decision = %q, want block", payload.Decision)
	}
	if payload.ReviewStatus != "" {
		t.Fatalf("ReviewStatus = %q, want empty for automatic final decision", payload.ReviewStatus)
	}
	if payload.ExternalID != "comment_123" || payload.ActorID != "user_456" {
		t.Fatalf("payload metadata = %#v", payload)
	}
	if repository.webhookDelivery == nil {
		t.Fatal("webhook delivery was not persisted")
	}
	if repository.webhookDelivery.Status != string(WebhookDeliverySucceeded) {
		t.Fatalf("webhook delivery status = %q, want succeeded", repository.webhookDelivery.Status)
	}
	if repository.webhookDelivery.DeliveryID == "" {
		t.Fatal("webhook delivery id is empty")
	}
	if repository.webhookDelivery.DeliveryID != payload.DeliveryID {
		t.Fatalf("persisted delivery id = %q, want payload delivery id %q", repository.webhookDelivery.DeliveryID, payload.DeliveryID)
	}
}

func TestServiceCheckDoesNotDispatchWebhookForReviewDecision(t *testing.T) {
	dispatcher := &fakeWebhookDispatcher{}
	repository := &fakeRepository{
		webhookClient: models.ClientApplication{
			ID:            11,
			UserID:        7,
			WebhookURL:    "https://example.com/moderation/webhook",
			WebhookSecret: "whsec_test",
		},
		webhookClientFound: true,
	}
	service := NewService(
		fakeAnalyzer{
			suggestion: ProviderSuggestion{
				RiskScore: 0.6,
				Labels:    []string{"harassment"},
				Reason:    "Needs operator review.",
			},
		},
		repository,
		DefaultPolicy(),
		dispatcher,
	)

	output, err := service.Check(context.Background(), CheckInput{
		UserID:   7,
		ClientID: 11,
		Content:  "review this text",
	})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if output.Decision != DecisionReview {
		t.Fatalf("Decision = %q, want review", output.Decision)
	}
	if len(dispatcher.payloads) != 0 {
		t.Fatalf("webhook dispatches = %d, want 0 before human final decision", len(dispatcher.payloads))
	}
}

func TestServiceCheckDoesNotFailWhenWebhookDispatchFails(t *testing.T) {
	dispatcher := &fakeWebhookDispatcher{err: errors.New("send https://example.com/hook?token=secret-token: webhook unavailable")}
	repository := &fakeRepository{
		webhookClient: models.ClientApplication{
			ID:            11,
			UserID:        7,
			WebhookURL:    "https://example.com/moderation/webhook",
			WebhookSecret: "whsec_test",
		},
		webhookClientFound: true,
	}
	service := NewService(
		fakeAnalyzer{
			suggestion: ProviderSuggestion{
				RiskScore: 0.8,
				Labels:    []string{"hate"},
				Reason:    "Policy threshold exceeded.",
			},
		},
		repository,
		DefaultPolicy(),
		dispatcher,
	)

	output, err := service.Check(context.Background(), CheckInput{
		UserID:   7,
		ClientID: 11,
		Content:  "block this text",
	})
	if err != nil {
		t.Fatalf("Check() error = %v, want nil despite webhook failure", err)
	}

	if output.Decision != DecisionBlock {
		t.Fatalf("Decision = %q, want block", output.Decision)
	}
	if repository.webhookDelivery == nil {
		t.Fatal("webhook delivery failure was not persisted")
	}
	if repository.webhookDelivery.Status != string(WebhookDeliveryFailed) {
		t.Fatalf("webhook delivery status = %q, want failed", repository.webhookDelivery.Status)
	}
	if repository.webhookDelivery.ErrorMessage != "webhook delivery failed" {
		t.Fatalf("webhook delivery error = %q, want safe failure category", repository.webhookDelivery.ErrorMessage)
	}
	if strings.Contains(repository.webhookDelivery.ErrorMessage, "secret-token") {
		t.Fatalf("webhook delivery error leaked URL token: %q", repository.webhookDelivery.ErrorMessage)
	}
}

func TestWebhookDeliveryOutputSanitizesLegacySensitiveError(t *testing.T) {
	output := webhookDeliveryOutputFromModel(models.WebhookDelivery{
		ErrorMessage: "send https://example.com/hook?token=secret-token: connection refused",
	})
	if output.ErrorMessage != "webhook delivery failed" {
		t.Fatalf("ErrorMessage = %q, want safe category", output.ErrorMessage)
	}
	if strings.Contains(output.ErrorMessage, "secret-token") {
		t.Fatalf("ErrorMessage leaked legacy URL token: %q", output.ErrorMessage)
	}
}

func TestServiceCheckRecordsInitialWebhookDeliveryMetrics(t *testing.T) {
	labels := map[string]string{
		"status":  string(WebhookDeliveryFailed),
		"trigger": webhookTriggerInitial,
	}
	beforeTotal := prometheusCounterValue(t, "webhook_deliveries_total", labels)
	beforeDuration := prometheusHistogramCount(t, "webhook_delivery_duration_seconds", labels)

	dispatcher := &fakeWebhookDispatcher{err: errors.New("webhook unavailable")}
	repository := &fakeRepository{
		webhookClient: models.ClientApplication{
			ID:            11,
			UserID:        7,
			WebhookURL:    "https://example.com/moderation/webhook",
			WebhookSecret: "whsec_test",
		},
		webhookClientFound: true,
	}
	service := NewService(
		fakeAnalyzer{
			suggestion: ProviderSuggestion{
				RiskScore: 0.8,
				Labels:    []string{"hate"},
				Reason:    "Policy threshold exceeded.",
			},
		},
		repository,
		DefaultPolicy(),
		dispatcher,
	)

	if _, err := service.Check(context.Background(), CheckInput{
		UserID:   7,
		ClientID: 11,
		Content:  "block this text",
	}); err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	afterTotal := prometheusCounterValue(t, "webhook_deliveries_total", labels)
	if afterTotal != beforeTotal+1 {
		t.Fatalf("webhook_deliveries_total increment = %v, want %v", afterTotal-beforeTotal, 1.0)
	}
	afterDuration := prometheusHistogramCount(t, "webhook_delivery_duration_seconds", labels)
	if afterDuration != beforeDuration+1 {
		t.Fatalf("webhook_delivery_duration_seconds count increment = %v, want %v", afterDuration-beforeDuration, 1.0)
	}
}

func TestServiceCheckRecordsWebhookDeliveryAfterRequestCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dispatcher := &fakeWebhookDispatcher{}
	repository := &fakeRepository{
		afterSaveCheck: cancel,
		webhookClient: models.ClientApplication{
			ID:            11,
			UserID:        7,
			WebhookURL:    "https://example.com/moderation/webhook",
			WebhookSecret: "whsec_test",
		},
		webhookClientFound: true,
	}
	service := NewService(
		fakeAnalyzer{
			suggestion: ProviderSuggestion{
				RiskScore: 0.8,
				Labels:    []string{"hate"},
				Reason:    "Policy threshold exceeded.",
			},
		},
		repository,
		DefaultPolicy(),
		dispatcher,
	)

	_, err := service.Check(ctx, CheckInput{
		UserID:   7,
		ClientID: 11,
		Content:  "block this text",
	})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if dispatcher.dispatchContextErr != nil {
		t.Fatalf("dispatch context err = %v, want nil", dispatcher.dispatchContextErr)
	}
	if repository.webhookDeliverySaveContextErr != nil {
		t.Fatalf("save delivery context err = %v, want nil", repository.webhookDeliverySaveContextErr)
	}
	if repository.webhookDelivery == nil {
		t.Fatal("webhook delivery was not persisted")
	}
}

func TestServiceCheckCreatesReviewCaseForReviewDecision(t *testing.T) {
	repository := &fakeRepository{}
	service := NewService(
		fakeAnalyzer{
			suggestion: ProviderSuggestion{
				RiskScore: 0.6,
				Labels:    []string{"harassment"},
				Reason:    "Needs operator review.",
				RawOutput: `{"risk_score":0.6,"labels":["harassment"],"reason":"Needs operator review."}`,
			},
			provider: ProviderInfo{Provider: "test-provider", Model: "test-model"},
		},
		repository,
		DefaultPolicy(),
	)

	output, err := service.Check(context.Background(), CheckInput{
		UserID:  7,
		Content: "review this text",
	})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if output.Decision != DecisionReview {
		t.Fatalf("Decision = %q, want review", output.Decision)
	}
	if repository.reviewCase == nil {
		t.Fatal("review case was not created")
	}
	if repository.reviewCase.RequestID != output.RequestID {
		t.Fatalf("review case request id = %q, want %q", repository.reviewCase.RequestID, output.RequestID)
	}
	if repository.reviewCase.UserID != 7 {
		t.Fatalf("review case user id = %d, want 7", repository.reviewCase.UserID)
	}
	if repository.reviewCase.Status != string(ReviewStatusPending) {
		t.Fatalf("review case status = %q, want pending", repository.reviewCase.Status)
	}
}

func TestServiceCheckDefaultsSource(t *testing.T) {
	repository := &fakeRepository{}
	service := NewService(
		fakeAnalyzer{
			suggestion: ProviderSuggestion{
				RiskScore: 0.1,
				Labels:    []string{"safe"},
				Reason:    "No policy issue.",
				RawOutput: `{"risk_score":0.1,"labels":["safe"],"reason":"No policy issue."}`,
			},
			provider: ProviderInfo{Provider: "test", Model: "model"},
		},
		repository,
		DefaultPolicy(),
	)

	if _, err := service.Check(context.Background(), CheckInput{
		UserID:  1,
		Content: "hello",
	}); err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if repository.request.Source != defaultSource {
		t.Fatalf("Source = %q, want %q", repository.request.Source, defaultSource)
	}
}

func TestServiceCheckUsesConfiguredPolicy(t *testing.T) {
	repository := &fakeRepository{}
	policy, err := NewPolicy("custom-v1", 0.2, 0.5)
	if err != nil {
		t.Fatalf("NewPolicy() error = %v", err)
	}

	service := NewService(
		fakeAnalyzer{
			suggestion: ProviderSuggestion{
				RiskScore: 0.6,
				Labels:    []string{"harassment"},
				Reason:    "Contains abusive language.",
				RawOutput: `{"risk_score":0.6,"labels":["harassment"],"reason":"Contains abusive language."}`,
			},
			provider: ProviderInfo{Provider: "test", Model: "model"},
		},
		repository,
		policy,
	)

	output, err := service.Check(context.Background(), CheckInput{
		UserID:  1,
		Content: "hello",
	})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if output.Decision != DecisionBlock {
		t.Fatalf("Decision = %q, want block", output.Decision)
	}
	if output.PolicyVersion != "custom-v1" {
		t.Fatalf("PolicyVersion = %q, want custom-v1", output.PolicyVersion)
	}
	if repository.result.PolicyVersion != "custom-v1" {
		t.Fatalf("persisted PolicyVersion = %q, want custom-v1", repository.result.PolicyVersion)
	}
}

func TestServiceCheckUsesClientPolicyVersion(t *testing.T) {
	repository := &fakeRepository{
		webhookClient: models.ClientApplication{
			ID:            11,
			UserID:        1,
			Name:          "blog-comments",
			PolicyVersion: "strict-v1",
		},
		webhookClientFound: true,
	}
	strictPolicy, err := NewPolicy("strict-v1", 0.2, 0.5)
	if err != nil {
		t.Fatalf("NewPolicy() error = %v", err)
	}
	policies, err := NewPolicySet(DefaultPolicy(), strictPolicy)
	if err != nil {
		t.Fatalf("NewPolicySet() error = %v", err)
	}
	service := NewServiceWithPolicySet(
		fakeAnalyzer{
			suggestion: ProviderSuggestion{
				RiskScore: 0.6,
				Labels:    []string{"harassment"},
				Reason:    "Contains abusive language.",
				RawOutput: `{"risk_score":0.6,"labels":["harassment"],"reason":"Contains abusive language."}`,
			},
			provider: ProviderInfo{Provider: "test", Model: "model"},
		},
		repository,
		policies,
	)

	output, err := service.Check(context.Background(), CheckInput{
		UserID:   1,
		ClientID: 11,
		Content:  "hello",
	})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if repository.clientID != 11 {
		t.Fatalf("client id lookup = %d, want 11", repository.clientID)
	}
	if output.Decision != DecisionBlock {
		t.Fatalf("Decision = %q, want block", output.Decision)
	}
	if output.PolicyVersion != "strict-v1" {
		t.Fatalf("PolicyVersion = %q, want strict-v1", output.PolicyVersion)
	}
	if repository.result.PolicyVersion != "strict-v1" {
		t.Fatalf("persisted PolicyVersion = %q, want strict-v1", repository.result.PolicyVersion)
	}
}

func TestServiceCheckRejectsUnknownClientPolicyVersion(t *testing.T) {
	analyzerCalls := 0
	repository := &fakeRepository{
		webhookClient: models.ClientApplication{
			ID:            11,
			UserID:        1,
			Name:          "blog-comments",
			PolicyVersion: "missing-v1",
		},
		webhookClientFound: true,
	}
	service := NewService(
		fakeAnalyzer{calls: &analyzerCalls},
		repository,
		DefaultPolicy(),
	)

	_, err := service.Check(context.Background(), CheckInput{
		UserID:   1,
		ClientID: 11,
		Content:  "hello",
	})
	if err == nil {
		t.Fatal("Check() error = nil, want unknown policy error")
	}
	if !strings.Contains(err.Error(), `policy_version "missing-v1" is not configured`) {
		t.Fatalf("Check() error = %q, want unknown policy detail", err.Error())
	}
	if analyzerCalls != 0 {
		t.Fatalf("analyzer calls = %d, want 0", analyzerCalls)
	}
}

func TestServiceCheckRejectsClientOwnedByAnotherUser(t *testing.T) {
	analyzerCalls := 0
	repository := &fakeRepository{
		webhookClient: models.ClientApplication{
			ID:            11,
			UserID:        99,
			Name:          "blog-comments",
			PolicyVersion: "default-v1",
		},
		webhookClientFound: true,
	}
	service := NewService(
		fakeAnalyzer{calls: &analyzerCalls},
		repository,
		DefaultPolicy(),
	)

	_, err := service.Check(context.Background(), CheckInput{
		UserID:   1,
		ClientID: 11,
		Content:  "hello",
	})
	if err == nil {
		t.Fatal("Check() error = nil, want client ownership error")
	}
	if !strings.Contains(err.Error(), "Client not found") {
		t.Fatalf("Check() error = %q, want client not found", err.Error())
	}
	if analyzerCalls != 0 {
		t.Fatalf("analyzer calls = %d, want 0", analyzerCalls)
	}
}

func TestServiceCheckRejectsInvalidInput(t *testing.T) {
	service := NewService(fakeAnalyzer{}, &fakeRepository{}, DefaultPolicy())

	tests := []struct {
		name    string
		input   CheckInput
		wantErr string
	}{
		{
			name: "missing user",
			input: CheckInput{
				Content: "hello",
			},
			wantErr: "User not authenticated",
		},
		{
			name: "missing content",
			input: CheckInput{
				UserID: 1,
			},
			wantErr: "content is required",
		},
		{
			name: "content too long",
			input: CheckInput{
				UserID:  1,
				Content: strings.Repeat("a", maxContentLength+1),
			},
			wantErr: "content must not exceed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.Check(context.Background(), tt.input)
			if err == nil {
				t.Fatal("Check() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Check() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestServiceGetResultReturnsStoredRecord(t *testing.T) {
	createdAt := time.Date(2026, 6, 28, 10, 30, 0, 0, time.UTC)
	reviewedAt := time.Date(2026, 6, 28, 11, 30, 0, 0, time.UTC)
	reviewerID := uint(42)
	service := NewService(fakeAnalyzer{}, &fakeRepository{
		stored: StoredResult{
			Request: models.ModerationRequest{
				RequestID:  "request-123",
				UserID:     7,
				Content:    "stored content",
				Source:     "comment",
				ExternalID: "comment_123",
				ActorID:    "user_456",
				Status:     "completed",
			},
			Result: models.ModerationResult{
				RequestID:     "request-123",
				UserID:        7,
				Provider:      "test-provider",
				Model:         "test-model",
				RawOutput:     `{"risk_score":0.6,"labels":["harassment"],"reason":"Contains abusive language."}`,
				RiskScore:     0.6,
				Labels:        `["harassment"]`,
				Decision:      string(DecisionReview),
				Reason:        "Contains abusive language.",
				PolicyVersion: "default-v1",
				CreatedAt:     createdAt,
			},
			ReviewCase: &models.ReviewCase{
				RequestID:     "request-123",
				UserID:        7,
				Status:        string(ReviewStatusApproved),
				ReviewerID:    &reviewerID,
				FinalDecision: string(DecisionAllow),
				ReviewNotes:   "approved after context check",
				ReviewedAt:    &reviewedAt,
			},
		},
	}, DefaultPolicy())

	output, err := service.GetResult(context.Background(), 7, " request-123 ")
	if err != nil {
		t.Fatalf("GetResult() error = %v", err)
	}

	if output.RequestID != "request-123" {
		t.Fatalf("RequestID = %q, want request-123", output.RequestID)
	}
	if output.Content != "stored content" {
		t.Fatalf("Content = %q, want stored content", output.Content)
	}
	if output.Decision != DecisionReview {
		t.Fatalf("Decision = %q, want review", output.Decision)
	}
	if !equalStrings(output.Labels, []string{"harassment"}) {
		t.Fatalf("Labels = %#v, want harassment", output.Labels)
	}
	if !output.CreatedAt.Equal(createdAt) {
		t.Fatalf("CreatedAt = %v, want %v", output.CreatedAt, createdAt)
	}
	if output.ReviewStatus != string(ReviewStatusApproved) {
		t.Fatalf("ReviewStatus = %q, want approved", output.ReviewStatus)
	}
	if output.FinalDecision != string(DecisionAllow) {
		t.Fatalf("FinalDecision = %q, want allow", output.FinalDecision)
	}
	if output.ReviewedAt == nil || !output.ReviewedAt.Equal(reviewedAt) {
		t.Fatalf("ReviewedAt = %v, want %v", output.ReviewedAt, reviewedAt)
	}
	repository := service.repository.(*fakeRepository)
	if repository.userID != 7 {
		t.Fatalf("repository userID = %d, want 7", repository.userID)
	}
	if repository.requestID != "request-123" {
		t.Fatalf("repository requestID = %q, want request-123", repository.requestID)
	}
}

func TestServiceGetClientResultReturnsClientScopedRecord(t *testing.T) {
	clientID := uint(11)
	service := NewService(fakeAnalyzer{}, &fakeRepository{
		stored: StoredResult{
			Request: models.ModerationRequest{
				RequestID: "request-123",
				UserID:    7,
				ClientID:  &clientID,
				Content:   "stored content",
				Source:    "comment",
				Status:    "completed",
			},
			Result: models.ModerationResult{
				RequestID:     "request-123",
				UserID:        7,
				ClientID:      &clientID,
				Provider:      "test-provider",
				Model:         "test-model",
				RiskScore:     0.6,
				Labels:        `["harassment"]`,
				Decision:      string(DecisionReview),
				Reason:        "Needs review.",
				PolicyVersion: "default-v1",
				CreatedAt:     time.Date(2026, 6, 28, 10, 30, 0, 0, time.UTC),
			},
		},
	}, DefaultPolicy())

	output, err := service.GetClientResult(context.Background(), 7, clientID, " request-123 ")
	if err != nil {
		t.Fatalf("GetClientResult() error = %v", err)
	}

	if output.RequestID != "request-123" {
		t.Fatalf("RequestID = %q, want request-123", output.RequestID)
	}
	repository := service.repository.(*fakeRepository)
	if repository.userID != 7 {
		t.Fatalf("repository userID = %d, want 7", repository.userID)
	}
	if repository.clientID != clientID {
		t.Fatalf("repository clientID = %d, want %d", repository.clientID, clientID)
	}
	if repository.requestID != "request-123" {
		t.Fatalf("repository requestID = %q, want request-123", repository.requestID)
	}
}

func TestServiceGetClientResultRejectsMissingClient(t *testing.T) {
	service := NewService(fakeAnalyzer{}, &fakeRepository{}, DefaultPolicy())

	_, err := service.GetClientResult(context.Background(), 7, 0, "request-123")
	if err == nil {
		t.Fatal("GetClientResult() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "API key client not authenticated") {
		t.Fatalf("GetClientResult() error = %q, want client auth error", err.Error())
	}
}

func TestServiceGetResultRejectsInvalidInput(t *testing.T) {
	service := NewService(fakeAnalyzer{}, &fakeRepository{}, DefaultPolicy())

	tests := []struct {
		name      string
		userID    uint
		requestID string
		wantErr   string
	}{
		{
			name:      "missing user",
			requestID: "request-123",
			wantErr:   "User not authenticated",
		},
		{
			name:    "missing request id",
			userID:  7,
			wantErr: "request_id is required",
		},
		{
			name:      "request id too long",
			userID:    7,
			requestID: strings.Repeat("a", maxRequestIDLength+1),
			wantErr:   "request_id must not exceed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.GetResult(context.Background(), tt.userID, tt.requestID)
			if err == nil {
				t.Fatal("GetResult() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("GetResult() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestServiceListReviewCasesDefaultsToPending(t *testing.T) {
	createdAt := time.Date(2026, 6, 28, 11, 0, 0, 0, time.UTC)
	service := NewService(fakeAnalyzer{}, &fakeRepository{
		reviewCases: []StoredReviewCase{
			{
				Case: models.ReviewCase{
					ID:        3,
					RequestID: "request-123",
					UserID:    7,
					Status:    string(ReviewStatusPending),
					CreatedAt: createdAt,
				},
				Request: models.ModerationRequest{
					RequestID: "request-123",
					UserID:    7,
					Content:   "stored content",
					Source:    "comment",
				},
				Result: models.ModerationResult{
					RequestID:     "request-123",
					UserID:        7,
					RiskScore:     0.6,
					Labels:        `["harassment"]`,
					Decision:      string(DecisionReview),
					Reason:        "Needs operator review.",
					PolicyVersion: "default-v1",
				},
			},
		},
	}, DefaultPolicy())

	output, err := service.ListReviewCases(context.Background(), 7, "", "", "")
	if err != nil {
		t.Fatalf("ListReviewCases() error = %v", err)
	}

	if len(output.Items) != 1 {
		t.Fatalf("len(output) = %d, want 1", len(output.Items))
	}
	if output.Items[0].ID != 3 {
		t.Fatalf("ID = %d, want 3", output.Items[0].ID)
	}
	if output.Items[0].Status != ReviewStatusPending {
		t.Fatalf("Status = %q, want pending", output.Items[0].Status)
	}
	if output.Items[0].PolicyDecision != DecisionReview {
		t.Fatalf("PolicyDecision = %q, want review", output.Items[0].PolicyDecision)
	}
	if !equalStrings(output.Items[0].Labels, []string{"harassment"}) {
		t.Fatalf("Labels = %#v, want harassment", output.Items[0].Labels)
	}

	repository := service.repository.(*fakeRepository)
	if repository.reviewStatus != ReviewStatusPending {
		t.Fatalf("repository status = %q, want pending", repository.reviewStatus)
	}
}

func TestServiceListCompletedReviewCasesReturnsOpaqueCursor(t *testing.T) {
	reviewedAt := time.Date(2026, 7, 12, 8, 0, 0, 0, time.UTC)
	repository := &fakeRepository{
		reviewCases: []StoredReviewCase{
			{
				Case: models.ReviewCase{
					ID:         9,
					RequestID:  "request-completed",
					UserID:     7,
					Status:     string(ReviewStatusApproved),
					ReviewedAt: &reviewedAt,
				},
				Request: models.ModerationRequest{
					RequestID: "request-completed",
					UserID:    7,
					Content:   "completed content",
				},
				Result: models.ModerationResult{
					RequestID:     "request-completed",
					Decision:      string(DecisionReview),
					Labels:        `[]`,
					PolicyVersion: "default-v1",
				},
			},
		},
		reviewNextCursor: &ReviewCaseCursor{
			Version:    reviewCaseCursorVersion,
			Status:     ReviewStatusCompleted,
			ReviewedAt: reviewedAt,
			ID:         9,
		},
	}
	service := NewService(fakeAnalyzer{}, repository, DefaultPolicy())

	output, err := service.ListReviewCases(
		context.Background(),
		7,
		"completed",
		"25",
		"",
	)
	if err != nil {
		t.Fatalf("ListReviewCases() error = %v", err)
	}

	if len(output.Items) != 1 || output.Items[0].ID != 9 {
		t.Fatalf("Items = %#v, want completed case 9", output.Items)
	}
	if output.NextCursor == "" {
		t.Fatal("NextCursor = empty, want opaque cursor")
	}
	decoded, err := decodeReviewCaseCursor(output.NextCursor, ReviewStatusCompleted)
	if err != nil {
		t.Fatalf("decode next cursor: %v", err)
	}
	if decoded.ID != 9 || !decoded.ReviewedAt.Equal(reviewedAt) {
		t.Fatalf("decoded cursor = %#v, want reviewedAt/case 9", decoded)
	}
	if repository.reviewFilter.Status != ReviewStatusCompleted || repository.reviewFilter.Limit != 25 {
		t.Fatalf("repository filter = %#v, want completed limit 25", repository.reviewFilter)
	}
}

func TestServiceListReviewCasesValidatesPagination(t *testing.T) {
	service := NewService(fakeAnalyzer{}, &fakeRepository{}, DefaultPolicy())
	reviewedAt := time.Date(2026, 7, 12, 8, 0, 0, 0, time.UTC)
	approvedCursor, err := encodeReviewCaseCursor(ReviewCaseCursor{
		Version:    reviewCaseCursorVersion,
		Status:     ReviewStatusApproved,
		ReviewedAt: reviewedAt,
		ID:         9,
	})
	if err != nil {
		t.Fatalf("encode approved cursor: %v", err)
	}

	tests := []struct {
		name   string
		status string
		limit  string
		cursor string
		want   string
	}{
		{name: "pending pagination", status: "pending", limit: "10", want: "only supported"},
		{name: "invalid cursor", status: "completed", cursor: "not-base64", want: "cursor is invalid"},
		{name: "cursor status mismatch", status: "mistake", cursor: approvedCursor, want: "cursor is invalid"},
		{name: "large limit", status: "completed", limit: "101", want: "must not exceed 100"},
		{name: "unknown status", status: "done", want: "status must be"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.ListReviewCases(
				context.Background(),
				7,
				tt.status,
				tt.limit,
				tt.cursor,
			)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ListReviewCases() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestServiceGetReviewCaseReturnsStoredCase(t *testing.T) {
	createdAt := time.Date(2026, 6, 28, 11, 0, 0, 0, time.UTC)
	reviewedAt := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	clientID := uint(11)
	reviewerID := uint(42)
	repository := &fakeRepository{
		reviewCaseStored: StoredReviewCase{
			Case: models.ReviewCase{
				ID:            3,
				RequestID:     "request-123",
				UserID:        7,
				ClientID:      &clientID,
				Status:        string(ReviewStatusApproved),
				ReviewerID:    &reviewerID,
				FinalDecision: string(DecisionAllow),
				ReviewNotes:   "looks safe",
				ReviewedAt:    &reviewedAt,
				CreatedAt:     createdAt,
			},
			Request: models.ModerationRequest{
				RequestID:  "request-123",
				UserID:     7,
				ClientID:   &clientID,
				Content:    "stored content",
				Source:     "comment",
				ExternalID: "comment_123",
				ActorID:    "user_456",
			},
			Result: models.ModerationResult{
				RequestID:     "request-123",
				UserID:        7,
				ClientID:      &clientID,
				RiskScore:     0.6,
				Labels:        `["harassment"]`,
				Decision:      string(DecisionReview),
				Reason:        "Needs operator review.",
				PolicyVersion: "default-v1",
			},
		},
	}
	service := NewService(fakeAnalyzer{}, repository, DefaultPolicy())

	output, err := service.GetReviewCase(context.Background(), 9, " 3 ")
	if err != nil {
		t.Fatalf("GetReviewCase() error = %v", err)
	}

	if repository.caseID != 3 {
		t.Fatalf("caseID = %d, want 3", repository.caseID)
	}
	if output.ID != 3 {
		t.Fatalf("ID = %d, want 3", output.ID)
	}
	if output.ClientID == nil || *output.ClientID != 11 {
		t.Fatalf("ClientID = %#v, want 11", output.ClientID)
	}
	if output.Status != ReviewStatusApproved {
		t.Fatalf("Status = %q, want approved", output.Status)
	}
	if output.PolicyDecision != DecisionReview {
		t.Fatalf("PolicyDecision = %q, want review", output.PolicyDecision)
	}
	if output.FinalDecision != DecisionAllow {
		t.Fatalf("FinalDecision = %q, want allow", output.FinalDecision)
	}
	if output.ReviewerID == nil || *output.ReviewerID != 42 {
		t.Fatalf("ReviewerID = %#v, want 42", output.ReviewerID)
	}
	if output.ReviewedAt == nil || !output.ReviewedAt.Equal(reviewedAt) {
		t.Fatalf("ReviewedAt = %v, want %v", output.ReviewedAt, reviewedAt)
	}
	if !equalStrings(output.Labels, []string{"harassment"}) {
		t.Fatalf("Labels = %#v, want harassment", output.Labels)
	}
}

func TestServiceGetReviewCaseRejectsInvalidInput(t *testing.T) {
	service := NewService(fakeAnalyzer{}, &fakeRepository{}, DefaultPolicy())

	tests := []struct {
		name       string
		reviewerID uint
		caseID     string
		wantErr    string
	}{
		{
			name:    "missing reviewer",
			caseID:  "3",
			wantErr: "User not authenticated",
		},
		{
			name:       "missing case id",
			reviewerID: 9,
			wantErr:    "review case id is required",
		},
		{
			name:       "invalid case id",
			reviewerID: 9,
			caseID:     "abc",
			wantErr:    "review case id must be a positive integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.GetReviewCase(context.Background(), tt.reviewerID, tt.caseID)
			if err == nil {
				t.Fatal("GetReviewCase() error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("GetReviewCase() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestServiceGetStatsMapsStoredCounts(t *testing.T) {
	repository := &fakeRepository{
		stats: StoredStats{
			TotalModerated:     12,
			PolicyAllowed:      3,
			PolicyBlocked:      2,
			ReviewFinalAllowed: 4,
			ReviewFinalBlocked: 1,
			PendingReview:      2,
			Reviewed:           5,
			Mistakes:           1,
			WebhookTotal:       9,
			WebhookSucceeded:   6,
			WebhookFailed:      2,
			WebhookRetrying:    1,
		},
	}
	service := NewService(fakeAnalyzer{}, repository, DefaultPolicy())

	output, err := service.GetStats(context.Background(), 7)
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}

	if output.TotalModerated != 12 {
		t.Fatalf("TotalModerated = %d, want 12", output.TotalModerated)
	}
	if output.Allowed != 7 {
		t.Fatalf("Allowed = %d, want 7", output.Allowed)
	}
	if output.Blocked != 3 {
		t.Fatalf("Blocked = %d, want 3", output.Blocked)
	}
	if output.PendingReview != 2 {
		t.Fatalf("PendingReview = %d, want 2", output.PendingReview)
	}
	if output.Reviewed != 5 {
		t.Fatalf("Reviewed = %d, want 5", output.Reviewed)
	}
	if output.Mistakes != 1 {
		t.Fatalf("Mistakes = %d, want 1", output.Mistakes)
	}
	if output.MistakeRate != 0.2 {
		t.Fatalf("MistakeRate = %v, want 0.2", output.MistakeRate)
	}
	if output.WebhookTotal != 9 || output.WebhookSucceeded != 6 || output.WebhookFailed != 2 || output.WebhookRetrying != 1 {
		t.Fatalf("Webhook stats = %#v, want 9/6/2/1", output)
	}
}

func TestServiceGetStatsRejectsMissingUser(t *testing.T) {
	service := NewService(fakeAnalyzer{}, &fakeRepository{}, DefaultPolicy())

	_, err := service.GetStats(context.Background(), 0)
	if err == nil {
		t.Fatal("GetStats() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "User not authenticated") {
		t.Fatalf("GetStats() error = %q, want authentication error", err.Error())
	}
}

func TestServiceListPolicies(t *testing.T) {
	strictPolicy, err := NewPolicy("strict-v1", 0.2, 0.5)
	if err != nil {
		t.Fatalf("NewPolicy() error = %v", err)
	}
	policies, err := NewPolicySet(DefaultPolicy(), strictPolicy)
	if err != nil {
		t.Fatalf("NewPolicySet() error = %v", err)
	}
	service := NewServiceWithPolicySet(nil, nil, policies)

	output, err := service.ListPolicies(9)
	if err != nil {
		t.Fatalf("ListPolicies() error = %v", err)
	}

	if len(output.Items) != 2 {
		t.Fatalf("len(output.Items) = %d, want 2", len(output.Items))
	}
	if output.Items[0].Version != "default-v1" || !output.Items[0].Default {
		t.Fatalf("first policy = %#v, want default-v1 marked default", output.Items[0])
	}
	if output.Items[1].Version != "strict-v1" || output.Items[1].Default {
		t.Fatalf("second policy = %#v, want strict-v1 non-default", output.Items[1])
	}
}

func TestServiceListPoliciesRejectsMissingOperator(t *testing.T) {
	service := NewService(nil, nil, DefaultPolicy())

	_, err := service.ListPolicies(0)
	if err == nil {
		t.Fatal("ListPolicies() error = nil, want unauthorized")
	}
	if !strings.Contains(err.Error(), "User not authenticated") {
		t.Fatalf("ListPolicies() error = %q, want unauthenticated", err.Error())
	}
}

func TestServiceListPoliciesRejectsMissingPolicyRegistry(t *testing.T) {
	service := NewServiceWithPolicySet(nil, nil, PolicySet{})

	_, err := service.ListPolicies(9)
	if err == nil {
		t.Fatal("ListPolicies() error = nil, want configuration error")
	}
	if !strings.Contains(err.Error(), "moderation policies are not configured") {
		t.Fatalf("ListPolicies() error = %q, want missing policies", err.Error())
	}
}

func TestServiceListHistory(t *testing.T) {
	clientID := uint(11)
	createdAt := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	repository := &fakeRepository{
		historyItems: []StoredHistoryItem{
			{
				Request: models.ModerationRequest{
					RequestID:  "request-123",
					UserID:     7,
					ClientID:   &clientID,
					Content:    "stored content",
					Source:     "comment",
					ExternalID: "comment_123",
					ActorID:    "user_456",
					Status:     "completed",
				},
				Result: models.ModerationResult{
					RequestID:     "request-123",
					UserID:        7,
					ClientID:      &clientID,
					Provider:      "test-provider",
					Model:         "test-model",
					RiskScore:     0.6,
					Labels:        `["harassment"]`,
					Decision:      string(DecisionReview),
					Reason:        "Needs operator review.",
					PolicyVersion: "default-v1",
					CreatedAt:     createdAt,
				},
				ReviewCase: &models.ReviewCase{
					RequestID:     "request-123",
					Status:        string(ReviewStatusApproved),
					FinalDecision: string(DecisionAllow),
				},
			},
		},
	}
	service := NewService(fakeAnalyzer{}, repository, DefaultPolicy())

	output, err := service.ListHistory(context.Background(), 9, "review", "11", " comment_123 ", "25")
	if err != nil {
		t.Fatalf("ListHistory() error = %v", err)
	}

	if repository.historyFilter.Decision != DecisionReview {
		t.Fatalf("decision filter = %q, want review", repository.historyFilter.Decision)
	}
	if repository.historyFilter.ClientID == nil || *repository.historyFilter.ClientID != 11 {
		t.Fatalf("client id filter = %#v, want 11", repository.historyFilter.ClientID)
	}
	if repository.historyFilter.ExternalID != "comment_123" {
		t.Fatalf("external id filter = %q, want comment_123", repository.historyFilter.ExternalID)
	}
	if repository.historyFilter.Limit != 25 {
		t.Fatalf("limit = %d, want 25", repository.historyFilter.Limit)
	}
	if len(output.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(output.Items))
	}
	item := output.Items[0]
	if item.RequestID != "request-123" {
		t.Fatalf("RequestID = %q, want request-123", item.RequestID)
	}
	if item.PolicyDecision != DecisionReview {
		t.Fatalf("PolicyDecision = %q, want review", item.PolicyDecision)
	}
	if item.ReviewStatus != ReviewStatusApproved {
		t.Fatalf("ReviewStatus = %q, want approved", item.ReviewStatus)
	}
	if item.FinalDecision != DecisionAllow {
		t.Fatalf("FinalDecision = %q, want allow", item.FinalDecision)
	}
	if len(item.Labels) != 1 || item.Labels[0] != "harassment" {
		t.Fatalf("Labels = %#v, want harassment", item.Labels)
	}
}

func TestServiceListHistoryValidatesFilters(t *testing.T) {
	service := NewService(fakeAnalyzer{}, &fakeRepository{}, DefaultPolicy())

	tests := []struct {
		name       string
		operatorID uint
		decision   string
		clientID   string
		externalID string
		limit      string
		want       string
	}{
		{
			name:       "missing operator",
			operatorID: 0,
			want:       "User not authenticated",
		},
		{
			name:       "invalid decision",
			operatorID: 9,
			decision:   "maybe",
			want:       "decision must be allow, review, or block",
		},
		{
			name:       "invalid client id",
			operatorID: 9,
			clientID:   "abc",
			want:       "client_id must be a positive integer",
		},
		{
			name:       "external id too long",
			operatorID: 9,
			externalID: strings.Repeat("a", maxMetadataLength+1),
			want:       "external_id must not exceed",
		},
		{
			name:       "invalid limit",
			operatorID: 9,
			limit:      "0",
			want:       "limit must be a positive integer",
		},
		{
			name:       "excessive limit",
			operatorID: 9,
			limit:      "101",
			want:       "limit must not exceed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.ListHistory(
				context.Background(),
				tt.operatorID,
				tt.decision,
				tt.clientID,
				tt.externalID,
				tt.limit,
			)
			if err == nil {
				t.Fatal("ListHistory() error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ListHistory() error = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestServiceReviewActionsFinalizePendingCase(t *testing.T) {
	createdAt := time.Date(2026, 6, 28, 11, 0, 0, 0, time.UTC)
	repository := &fakeRepository{
		finalized: StoredReviewCase{
			Case: models.ReviewCase{
				ID:            3,
				RequestID:     "request-123",
				UserID:        7,
				Status:        string(ReviewStatusApproved),
				FinalDecision: string(DecisionAllow),
				ReviewNotes:   "looks safe",
				CreatedAt:     createdAt,
			},
			Request: models.ModerationRequest{
				RequestID: "request-123",
				UserID:    7,
				Content:   "stored content",
				Source:    "comment",
			},
			Result: models.ModerationResult{
				RequestID:     "request-123",
				UserID:        7,
				RiskScore:     0.6,
				Labels:        `["harassment"]`,
				Decision:      string(DecisionReview),
				Reason:        "Needs operator review.",
				PolicyVersion: "default-v1",
			},
		},
	}
	service := NewService(fakeAnalyzer{}, repository, DefaultPolicy())

	output, err := service.ApproveReviewCase(context.Background(), " 3 ", 9, " looks safe ")
	if err != nil {
		t.Fatalf("ApproveReviewCase() error = %v", err)
	}

	if output.Status != ReviewStatusApproved {
		t.Fatalf("Status = %q, want approved", output.Status)
	}
	if output.FinalDecision != DecisionAllow {
		t.Fatalf("FinalDecision = %q, want allow", output.FinalDecision)
	}
	if repository.caseID != 3 {
		t.Fatalf("caseID = %d, want 3", repository.caseID)
	}
	if repository.reviewerID != 9 {
		t.Fatalf("reviewerID = %d, want 9", repository.reviewerID)
	}
	if repository.finalStatus != ReviewStatusApproved {
		t.Fatalf("final status = %q, want approved", repository.finalStatus)
	}
	if repository.finalDecision != DecisionAllow {
		t.Fatalf("final decision = %q, want allow", repository.finalDecision)
	}
	if repository.notes != "looks safe" {
		t.Fatalf("notes = %q, want trimmed notes", repository.notes)
	}
	if repository.reviewedAt.IsZero() {
		t.Fatal("reviewedAt was not set")
	}
}

func TestServiceReviewActionsRecordMetrics(t *testing.T) {
	labels := map[string]string{
		"status":         string(ReviewStatusApproved),
		"final_decision": string(DecisionAllow),
	}
	before := prometheusCounterValue(t, "review_cases_finalized_total", labels)
	createdAt := time.Now().Add(-10 * time.Minute).UTC()
	reviewedAt := time.Now().UTC()
	repository := &fakeRepository{
		finalized: StoredReviewCase{
			Case: models.ReviewCase{
				ID:            3,
				RequestID:     "request-123",
				UserID:        7,
				Status:        string(ReviewStatusApproved),
				FinalDecision: string(DecisionAllow),
				CreatedAt:     createdAt,
				ReviewedAt:    &reviewedAt,
			},
			Request: models.ModerationRequest{
				RequestID: "request-123",
				UserID:    7,
				Content:   "stored content",
				Source:    "comment",
			},
			Result: models.ModerationResult{
				RequestID:     "request-123",
				UserID:        7,
				RiskScore:     0.6,
				Labels:        `["harassment"]`,
				Decision:      string(DecisionReview),
				Reason:        "Needs operator review.",
				PolicyVersion: "default-v1",
			},
		},
	}
	service := NewService(fakeAnalyzer{}, repository, DefaultPolicy())

	if _, err := service.ApproveReviewCase(context.Background(), "3", 9, "looks safe"); err != nil {
		t.Fatalf("ApproveReviewCase() error = %v", err)
	}

	after := prometheusCounterValue(t, "review_cases_finalized_total", labels)
	if after != before+1 {
		t.Fatalf("review_cases_finalized_total increment = %v, want %v", after-before, 1.0)
	}
}

func TestServiceReviewActionsDispatchWebhookWithHumanFinalDecision(t *testing.T) {
	dispatcher := &fakeWebhookDispatcher{}
	repository := &fakeRepository{
		webhookClient: models.ClientApplication{
			ID:            11,
			WebhookURL:    "https://example.com/moderation/webhook",
			WebhookSecret: "whsec_test",
		},
		webhookClientFound: true,
		finalized: StoredReviewCase{
			Case: models.ReviewCase{
				ID:            3,
				RequestID:     "request-123",
				UserID:        7,
				ClientID:      uintPtr(11),
				Status:        string(ReviewStatusApproved),
				FinalDecision: string(DecisionAllow),
			},
			Request: models.ModerationRequest{
				RequestID:  "request-123",
				UserID:     7,
				ClientID:   uintPtr(11),
				Content:    "stored content",
				Source:     "comment",
				ExternalID: "comment_123",
			},
			Result: models.ModerationResult{
				RequestID:     "request-123",
				UserID:        7,
				ClientID:      uintPtr(11),
				RiskScore:     0.6,
				Labels:        `["harassment"]`,
				Decision:      string(DecisionReview),
				Reason:        "Needs operator review.",
				PolicyVersion: "default-v1",
			},
		},
	}
	service := NewService(fakeAnalyzer{}, repository, DefaultPolicy(), dispatcher)

	output, err := service.ApproveReviewCase(context.Background(), "3", 9, "looks safe")
	if err != nil {
		t.Fatalf("ApproveReviewCase() error = %v", err)
	}

	if output.FinalDecision != DecisionAllow {
		t.Fatalf("FinalDecision = %q, want allow", output.FinalDecision)
	}
	if len(dispatcher.payloads) != 1 {
		t.Fatalf("webhook dispatches = %d, want 1", len(dispatcher.payloads))
	}
	payload := dispatcher.payloads[0]
	if payload.Decision != string(DecisionAllow) {
		t.Fatalf("webhook Decision = %q, want human final allow", payload.Decision)
	}
	if payload.ReviewStatus != string(ReviewStatusApproved) {
		t.Fatalf("webhook ReviewStatus = %q, want approved", payload.ReviewStatus)
	}
	if repository.webhookDelivery == nil {
		t.Fatal("webhook delivery was not persisted")
	}
	if repository.webhookDelivery.Status != string(WebhookDeliverySucceeded) {
		t.Fatalf("webhook delivery status = %q, want succeeded", repository.webhookDelivery.Status)
	}
}

func TestServiceRetryWebhookDelivery(t *testing.T) {
	metricLabels := map[string]string{
		"status":  string(WebhookDeliverySucceeded),
		"trigger": webhookTriggerManualRetry,
	}
	beforeDeliveries := prometheusCounterValue(t, "webhook_deliveries_total", metricLabels)

	payload := webhooks.FinalDecisionPayload{
		Event:         "moderation.final_decision",
		RequestID:     "request-123",
		ClientID:      11,
		ExternalID:    "comment_123",
		Decision:      string(DecisionBlock),
		RiskScore:     0.8,
		Labels:        []string{"hate"},
		Reason:        "Policy threshold exceeded.",
		PolicyVersion: "default-v1",
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("encode payload: %v", err)
	}

	dispatcher := &fakeWebhookDispatcher{}
	repository := &fakeRepository{
		webhookClient: models.ClientApplication{
			ID:            11,
			WebhookURL:    "https://example.com/moderation/webhook",
			WebhookSecret: "whsec_test",
		},
		webhookClientFound: true,
		webhookDeliveryStored: models.WebhookDelivery{
			ID:            5,
			DeliveryID:    "delivery-123",
			RequestID:     "request-123",
			ClientID:      11,
			Event:         "moderation.final_decision",
			Status:        string(WebhookDeliveryFailed),
			AttemptCount:  1,
			LastAttemptAt: time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC),
			ErrorMessage:  "webhook returned status 500",
			Payload:       string(payloadJSON),
		},
	}
	service := NewService(fakeAnalyzer{}, repository, DefaultPolicy(), dispatcher)

	output, err := service.RetryWebhookDelivery(context.Background(), 9, "5")
	if err != nil {
		t.Fatalf("RetryWebhookDelivery() error = %v", err)
	}

	if output.Status != WebhookDeliverySucceeded {
		t.Fatalf("Status = %q, want succeeded", output.Status)
	}
	if output.AttemptCount != 2 {
		t.Fatalf("AttemptCount = %d, want 2", output.AttemptCount)
	}
	if repository.webhookDeliveryID != 5 {
		t.Fatalf("delivery id = %d, want 5", repository.webhookDeliveryID)
	}
	if repository.webhookDeliveryStatus != WebhookDeliverySucceeded {
		t.Fatalf("retry status = %q, want succeeded", repository.webhookDeliveryStatus)
	}
	if len(dispatcher.payloads) != 1 {
		t.Fatalf("dispatches = %d, want 1", len(dispatcher.payloads))
	}
	if dispatcher.payloads[0].DeliveryID != "delivery-123" {
		t.Fatalf("retry delivery id = %q, want stored delivery id", dispatcher.payloads[0].DeliveryID)
	}
	afterDeliveries := prometheusCounterValue(t, "webhook_deliveries_total", metricLabels)
	if afterDeliveries != beforeDeliveries+1 {
		t.Fatalf("webhook_deliveries_total increment = %v, want %v", afterDeliveries-beforeDeliveries, 1.0)
	}
}

func TestServiceRetryFailedWebhookDeliveries(t *testing.T) {
	deliveryMetricLabels := map[string]string{
		"status":  string(WebhookDeliverySucceeded),
		"trigger": webhookTriggerAutomaticRetry,
	}
	batchMetricLabels := map[string]string{
		"result": webhookRetryBatchCompleted,
	}
	batchDeliveryMetricLabels := map[string]string{
		"result": webhookRetryBatchSucceeded,
	}
	beforeDeliveries := prometheusCounterValue(t, "webhook_deliveries_total", deliveryMetricLabels)
	beforeBatches := prometheusCounterValue(t, "webhook_retry_batches_total", batchMetricLabels)
	beforeBatchDeliveries := prometheusCounterValue(
		t,
		"webhook_retry_batch_deliveries_total",
		batchDeliveryMetricLabels,
	)

	payload := webhooks.FinalDecisionPayload{
		Event:         "moderation.final_decision",
		RequestID:     "request-123",
		ClientID:      11,
		Decision:      string(DecisionBlock),
		RiskScore:     0.8,
		Labels:        []string{"hate"},
		Reason:        "Policy threshold exceeded.",
		PolicyVersion: "default-v1",
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("encode payload: %v", err)
	}

	dispatcher := &fakeWebhookDispatcher{}
	repository := &fakeRepository{
		retryableWebhookDeliveries: []models.WebhookDelivery{
			{
				ID:            5,
				DeliveryID:    "delivery-123",
				RequestID:     "request-123",
				ClientID:      11,
				Event:         "moderation.final_decision",
				Status:        string(WebhookDeliveryFailed),
				AttemptCount:  1,
				LastAttemptAt: time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC),
			},
		},
		webhookClient: models.ClientApplication{
			ID:            11,
			WebhookURL:    "https://example.com/moderation/webhook",
			WebhookSecret: "whsec_test",
		},
		webhookClientFound: true,
		webhookDeliveryStored: models.WebhookDelivery{
			ID:            5,
			DeliveryID:    "delivery-123",
			RequestID:     "request-123",
			ClientID:      11,
			Event:         "moderation.final_decision",
			Status:        string(WebhookDeliveryFailed),
			AttemptCount:  1,
			LastAttemptAt: time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC),
			ErrorMessage:  "webhook returned status 500",
			Payload:       string(payloadJSON),
		},
	}
	service := NewService(fakeAnalyzer{}, repository, DefaultPolicy(), dispatcher)

	output, err := service.RetryFailedWebhookDeliveries(context.Background(), WebhookRetryInput{
		Limit:       10,
		MaxAttempts: 3,
	})
	if err != nil {
		t.Fatalf("RetryFailedWebhookDeliveries() error = %v", err)
	}

	if output.Attempted != 1 {
		t.Fatalf("Attempted = %d, want 1", output.Attempted)
	}
	if output.Succeeded != 1 {
		t.Fatalf("Succeeded = %d, want 1", output.Succeeded)
	}
	if output.Failed != 0 {
		t.Fatalf("Failed = %d, want 0", output.Failed)
	}
	if repository.webhookRetryLimit != 10 {
		t.Fatalf("retry limit = %d, want 10", repository.webhookRetryLimit)
	}
	if repository.webhookRetryMaxAttempts != 3 {
		t.Fatalf("retry max attempts = %d, want 3", repository.webhookRetryMaxAttempts)
	}
	if repository.webhookDeliveryID != 5 {
		t.Fatalf("delivery id = %d, want 5", repository.webhookDeliveryID)
	}
	if dispatcher.DispatchCount() != 1 {
		t.Fatalf("webhook dispatches = %d, want 1", dispatcher.DispatchCount())
	}
	afterDeliveries := prometheusCounterValue(t, "webhook_deliveries_total", deliveryMetricLabels)
	if afterDeliveries != beforeDeliveries+1 {
		t.Fatalf("webhook_deliveries_total increment = %v, want %v", afterDeliveries-beforeDeliveries, 1.0)
	}
	afterBatches := prometheusCounterValue(t, "webhook_retry_batches_total", batchMetricLabels)
	if afterBatches != beforeBatches+1 {
		t.Fatalf("webhook_retry_batches_total increment = %v, want %v", afterBatches-beforeBatches, 1.0)
	}
	afterBatchDeliveries := prometheusCounterValue(
		t,
		"webhook_retry_batch_deliveries_total",
		batchDeliveryMetricLabels,
	)
	if afterBatchDeliveries != beforeBatchDeliveries+1 {
		t.Fatalf(
			"webhook_retry_batch_deliveries_total increment = %v, want %v",
			afterBatchDeliveries-beforeBatchDeliveries,
			1.0,
		)
	}
}

func TestServiceRetryFailedWebhookDeliveriesSkipsClaimConflicts(t *testing.T) {
	batchMetricLabels := map[string]string{
		"result": webhookRetryBatchCompleted,
	}
	skippedMetricLabels := map[string]string{
		"result": webhookRetryBatchSkipped,
	}
	beforeBatches := prometheusCounterValue(t, "webhook_retry_batches_total", batchMetricLabels)
	beforeSkipped := prometheusCounterValue(
		t,
		"webhook_retry_batch_deliveries_total",
		skippedMetricLabels,
	)

	repository := &fakeRepository{
		retryableWebhookDeliveries: []models.WebhookDelivery{
			{
				ID:        5,
				RequestID: "request-123",
				ClientID:  11,
				Status:    string(WebhookDeliveryFailed),
			},
		},
		claimWebhookDeliveryOnce: true,
		webhookDeliveryClaimed:   true,
	}
	service := NewService(fakeAnalyzer{}, repository, DefaultPolicy(), &fakeWebhookDispatcher{})

	output, err := service.RetryFailedWebhookDeliveries(context.Background(), WebhookRetryInput{
		Limit:       10,
		MaxAttempts: 3,
	})
	if err != nil {
		t.Fatalf("RetryFailedWebhookDeliveries() error = %v", err)
	}

	if output.Skipped != 1 {
		t.Fatalf("Skipped = %d, want 1", output.Skipped)
	}
	if output.Attempted != 0 {
		t.Fatalf("Attempted = %d, want 0", output.Attempted)
	}
	afterBatches := prometheusCounterValue(t, "webhook_retry_batches_total", batchMetricLabels)
	if afterBatches != beforeBatches+1 {
		t.Fatalf("webhook_retry_batches_total increment = %v, want %v", afterBatches-beforeBatches, 1.0)
	}
	afterSkipped := prometheusCounterValue(
		t,
		"webhook_retry_batch_deliveries_total",
		skippedMetricLabels,
	)
	if afterSkipped != beforeSkipped+1 {
		t.Fatalf(
			"webhook_retry_batch_deliveries_total skipped increment = %v, want %v",
			afterSkipped-beforeSkipped,
			1.0,
		)
	}
}

func TestServiceRetryFailedWebhookDeliveriesValidatesInput(t *testing.T) {
	service := NewService(fakeAnalyzer{}, &fakeRepository{}, DefaultPolicy(), &fakeWebhookDispatcher{})

	tests := []struct {
		name    string
		input   WebhookRetryInput
		wantErr string
	}{
		{
			name: "zero limit",
			input: WebhookRetryInput{
				Limit:       0,
				MaxAttempts: 3,
			},
			wantErr: "webhook retry limit must be a positive integer",
		},
		{
			name: "excessive limit",
			input: WebhookRetryInput{
				Limit:       maxWebhookDeliveryListLimit + 1,
				MaxAttempts: 3,
			},
			wantErr: "webhook retry limit must not exceed",
		},
		{
			name: "single max attempt",
			input: WebhookRetryInput{
				Limit:       10,
				MaxAttempts: 1,
			},
			wantErr: "webhook retry max_attempts must be greater than 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.RetryFailedWebhookDeliveries(context.Background(), tt.input)
			if err == nil {
				t.Fatal("RetryFailedWebhookDeliveries() error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("RetryFailedWebhookDeliveries() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestServiceGetWebhookDelivery(t *testing.T) {
	httpStatus := 500
	repository := &fakeRepository{
		webhookDeliveryStored: models.WebhookDelivery{
			ID:            5,
			DeliveryID:    "delivery-123",
			RequestID:     "request-123",
			ClientID:      11,
			Event:         "moderation.final_decision",
			Status:        string(WebhookDeliveryFailed),
			AttemptCount:  2,
			LastAttemptAt: time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC),
			HTTPStatus:    &httpStatus,
			ErrorMessage:  "webhook returned status 500",
		},
	}
	service := NewService(fakeAnalyzer{}, repository, DefaultPolicy())

	output, err := service.GetWebhookDelivery(context.Background(), 9, "5")
	if err != nil {
		t.Fatalf("GetWebhookDelivery() error = %v", err)
	}

	if repository.webhookDeliveryID != 5 {
		t.Fatalf("delivery id = %d, want 5", repository.webhookDeliveryID)
	}
	if output.ID != 5 {
		t.Fatalf("ID = %d, want 5", output.ID)
	}
	if output.Status != WebhookDeliveryFailed {
		t.Fatalf("Status = %q, want failed", output.Status)
	}
	if output.HTTPStatus == nil || *output.HTTPStatus != 500 {
		t.Fatalf("HTTPStatus = %#v, want 500", output.HTTPStatus)
	}
}

func TestServiceGetWebhookDeliveryValidatesID(t *testing.T) {
	service := NewService(fakeAnalyzer{}, &fakeRepository{}, DefaultPolicy())

	_, err := service.GetWebhookDelivery(context.Background(), 9, "abc")
	if err == nil {
		t.Fatal("GetWebhookDelivery() error = nil, want validation error")
	}
	if !strings.Contains(err.Error(), "delivery id must be a positive integer") {
		t.Fatalf("GetWebhookDelivery() error = %q, want delivery id validation", err.Error())
	}
}

func TestServiceListWebhookDeliveries(t *testing.T) {
	repository := &fakeRepository{
		webhookDeliveries: []models.WebhookDelivery{
			{
				ID:            5,
				DeliveryID:    "delivery-123",
				RequestID:     "request-123",
				ClientID:      11,
				Event:         "moderation.final_decision",
				Status:        string(WebhookDeliveryFailed),
				AttemptCount:  2,
				LastAttemptAt: time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC),
			},
		},
	}
	service := NewService(fakeAnalyzer{}, repository, DefaultPolicy())

	output, err := service.ListWebhookDeliveries(context.Background(), 9, WebhookDeliveryListInput{
		Status:    "failed",
		ClientID:  "11",
		RequestID: " request-123 ",
		Limit:     "25",
	})
	if err != nil {
		t.Fatalf("ListWebhookDeliveries() error = %v", err)
	}

	if repository.webhookDeliveryListFilter.Status != WebhookDeliveryFailed {
		t.Fatalf("status filter = %q, want failed", repository.webhookDeliveryListFilter.Status)
	}
	if repository.webhookDeliveryListFilter.ClientID == nil ||
		*repository.webhookDeliveryListFilter.ClientID != 11 {
		t.Fatalf("client id filter = %#v, want 11", repository.webhookDeliveryListFilter.ClientID)
	}
	if repository.webhookDeliveryListFilter.RequestID != "request-123" {
		t.Fatalf("request id filter = %q, want request-123", repository.webhookDeliveryListFilter.RequestID)
	}
	if repository.webhookDeliveryListFilter.Limit != 25 {
		t.Fatalf("limit = %d, want 25", repository.webhookDeliveryListFilter.Limit)
	}
	if len(output.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(output.Items))
	}
	if output.Items[0].ID != 5 {
		t.Fatalf("item id = %d, want 5", output.Items[0].ID)
	}
}

func TestServiceListWebhookDeliveriesValidatesFilters(t *testing.T) {
	service := NewService(fakeAnalyzer{}, &fakeRepository{}, DefaultPolicy())

	tests := []struct {
		name    string
		input   WebhookDeliveryListInput
		wantErr string
	}{
		{
			name: "invalid status",
			input: WebhookDeliveryListInput{
				Status: "pending",
			},
			wantErr: "status must be succeeded, failed, or retrying",
		},
		{
			name: "invalid client id",
			input: WebhookDeliveryListInput{
				ClientID: "abc",
			},
			wantErr: "client_id must be a positive integer",
		},
		{
			name: "request id too long",
			input: WebhookDeliveryListInput{
				RequestID: strings.Repeat("a", maxRequestIDLength+1),
			},
			wantErr: "request_id must not exceed",
		},
		{
			name: "invalid limit",
			input: WebhookDeliveryListInput{
				Limit: "0",
			},
			wantErr: "limit must be a positive integer",
		},
		{
			name: "excessive limit",
			input: WebhookDeliveryListInput{
				Limit: "101",
			},
			wantErr: "limit must not exceed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.ListWebhookDeliveries(context.Background(), 9, tt.input)
			if err == nil {
				t.Fatal("ListWebhookDeliveries() error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ListWebhookDeliveries() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestServiceRetryWebhookDeliveryRecordsOutcomeAfterRequestCancel(t *testing.T) {
	payload := webhooks.FinalDecisionPayload{
		Event:         "moderation.final_decision",
		RequestID:     "request-123",
		ClientID:      11,
		Decision:      string(DecisionBlock),
		RiskScore:     0.8,
		Labels:        []string{"hate"},
		Reason:        "Policy threshold exceeded.",
		PolicyVersion: "default-v1",
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("encode payload: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dispatcher := &fakeWebhookDispatcher{
		afterDispatch: cancel,
	}
	repository := &fakeRepository{
		webhookClient: models.ClientApplication{
			ID:            11,
			WebhookURL:    "https://example.com/moderation/webhook",
			WebhookSecret: "whsec_test",
		},
		webhookClientFound: true,
		webhookDeliveryStored: models.WebhookDelivery{
			ID:            5,
			DeliveryID:    "delivery-123",
			RequestID:     "request-123",
			ClientID:      11,
			Event:         "moderation.final_decision",
			Status:        string(WebhookDeliveryFailed),
			AttemptCount:  1,
			LastAttemptAt: time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC),
			ErrorMessage:  "webhook returned status 500",
			Payload:       string(payloadJSON),
		},
	}
	service := NewService(fakeAnalyzer{}, repository, DefaultPolicy(), dispatcher)

	output, err := service.RetryWebhookDelivery(ctx, 9, "5")
	if err != nil {
		t.Fatalf("RetryWebhookDelivery() error = %v", err)
	}

	if output.Status != WebhookDeliverySucceeded {
		t.Fatalf("Status = %q, want succeeded", output.Status)
	}
	if repository.webhookDeliveryUpdateContextErr != nil {
		t.Fatalf("update context err = %v, want nil", repository.webhookDeliveryUpdateContextErr)
	}
	if dispatcher.DispatchCount() != 1 {
		t.Fatalf("webhook dispatches = %d, want 1", dispatcher.DispatchCount())
	}
}

func TestServiceRetryWebhookDeliveryClaimsFailedDeliveryOnce(t *testing.T) {
	payload := webhooks.FinalDecisionPayload{
		Event:         "moderation.final_decision",
		RequestID:     "request-123",
		ClientID:      11,
		Decision:      string(DecisionBlock),
		RiskScore:     0.8,
		Labels:        []string{"hate"},
		Reason:        "Policy threshold exceeded.",
		PolicyVersion: "default-v1",
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("encode payload: %v", err)
	}

	dispatcher := &fakeWebhookDispatcher{}
	repository := &fakeRepository{
		claimWebhookDeliveryOnce: true,
		webhookClient: models.ClientApplication{
			ID:            11,
			WebhookURL:    "https://example.com/moderation/webhook",
			WebhookSecret: "whsec_test",
		},
		webhookClientFound: true,
		webhookDeliveryStored: models.WebhookDelivery{
			ID:            5,
			DeliveryID:    "delivery-123",
			RequestID:     "request-123",
			ClientID:      11,
			Event:         "moderation.final_decision",
			Status:        string(WebhookDeliveryFailed),
			AttemptCount:  1,
			LastAttemptAt: time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC),
			ErrorMessage:  "webhook returned status 500",
			Payload:       string(payloadJSON),
		},
	}
	service := NewService(fakeAnalyzer{}, repository, DefaultPolicy(), dispatcher)

	const workers = 8
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := service.RetryWebhookDelivery(context.Background(), 9, "5")
			errs <- err
		}()
	}

	close(start)
	wg.Wait()
	close(errs)

	successes := 0
	conflicts := 0
	for err := range errs {
		if err == nil {
			successes++
			continue
		}
		if strings.Contains(err.Error(), "Webhook delivery is not failed") {
			conflicts++
		}
	}

	if successes != 1 {
		t.Fatalf("successful retries = %d, want 1", successes)
	}
	if conflicts != workers-1 {
		t.Fatalf("conflicting retries = %d, want %d", conflicts, workers-1)
	}
	if dispatcher.DispatchCount() != 1 {
		t.Fatalf("webhook dispatches = %d, want 1", dispatcher.DispatchCount())
	}
}

func TestServiceMarkReviewMistakeRequiresFinalDecision(t *testing.T) {
	service := NewService(fakeAnalyzer{}, &fakeRepository{}, DefaultPolicy())

	_, err := service.MarkReviewMistake(context.Background(), "3", 7, DecisionReview, "")
	if err == nil {
		t.Fatal("MarkReviewMistake() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "final_decision must be allow or block") {
		t.Fatalf("MarkReviewMistake() error = %q, want final_decision validation", err.Error())
	}
}

type fakeAnalyzer struct {
	suggestion ProviderSuggestion
	provider   ProviderInfo
	err        error
	calls      *int
}

func (a fakeAnalyzer) AnalyzeText(ctx context.Context, content string) (ProviderSuggestion, ProviderInfo, error) {
	if a.calls != nil {
		*a.calls = *a.calls + 1
	}
	if a.err != nil {
		return ProviderSuggestion{}, ProviderInfo{}, a.err
	}
	return a.suggestion, a.provider, nil
}

type fakeRepository struct {
	mu                              sync.Mutex
	request                         *models.ModerationRequest
	result                          *models.ModerationResult
	reviewCase                      *models.ReviewCase
	webhookDelivery                 *models.WebhookDelivery
	webhookDeliveryStored           models.WebhookDelivery
	webhookDeliveries               []models.WebhookDelivery
	retryableWebhookDeliveries      []models.WebhookDelivery
	claimWebhookDeliveryOnce        bool
	webhookDeliveryClaimed          bool
	stored                          StoredResult
	historyItems                    []StoredHistoryItem
	historyFilter                   HistoryFilter
	clientStored                    StoredResult
	clientResultFound               bool
	clientResultFoundAfterSave      bool
	findClientExternalIDCalls       int
	webhookClient                   models.ClientApplication
	webhookClientFound              bool
	reviewCases                     []StoredReviewCase
	reviewCaseStored                StoredReviewCase
	finalized                       StoredReviewCase
	stats                           StoredStats
	userID                          uint
	clientID                        uint
	externalID                      string
	requestID                       string
	reviewStatus                    ReviewStatus
	reviewFilter                    ReviewCaseFilter
	reviewNextCursor                *ReviewCaseCursor
	caseID                          uint
	webhookDeliveryID               uint
	webhookDeliveryStatus           WebhookDeliveryStatus
	webhookDeliveryHTTPStatus       *int
	webhookDeliveryError            string
	webhookDeliveryAttemptedAt      time.Time
	webhookDeliveryUpdateContextErr error
	webhookDeliverySaveContextErr   error
	webhookDeliveryListFilter       WebhookDeliveryFilter
	webhookRetryLimit               int
	webhookRetryMaxAttempts         int
	webhookRetryStaleBefore         time.Time
	afterSaveCheck                  func()
	reviewerID                      uint
	finalStatus                     ReviewStatus
	finalDecision                   Decision
	notes                           string
	reviewedAt                      time.Time
	err                             error
	saveErr                         error
}

func (r *fakeRepository) SaveCheck(
	ctx context.Context,
	request *models.ModerationRequest,
	result *models.ModerationResult,
	reviewCase *models.ReviewCase,
) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	if r.err != nil {
		return r.err
	}

	copiedRequest := *request
	copiedResult := *result
	r.request = &copiedRequest
	r.result = &copiedResult
	if reviewCase != nil {
		copiedReviewCase := *reviewCase
		r.reviewCase = &copiedReviewCase
	}
	if r.afterSaveCheck != nil {
		r.afterSaveCheck()
	}
	return nil
}

func (r *fakeRepository) GetResult(ctx context.Context, userID uint, requestID string) (StoredResult, error) {
	if r.err != nil {
		return StoredResult{}, r.err
	}
	r.userID = userID
	r.requestID = requestID
	return r.stored, nil
}

func (r *fakeRepository) GetResultForClient(
	ctx context.Context,
	userID uint,
	clientID uint,
	requestID string,
) (StoredResult, error) {
	if r.err != nil {
		return StoredResult{}, r.err
	}
	r.userID = userID
	r.clientID = clientID
	r.requestID = requestID
	return r.stored, nil
}

func (r *fakeRepository) FindResultByClientExternalID(
	ctx context.Context,
	clientID uint,
	externalID string,
) (StoredResult, bool, error) {
	if r.err != nil {
		return StoredResult{}, false, r.err
	}
	r.findClientExternalIDCalls++
	r.clientID = clientID
	r.externalID = externalID
	if r.clientResultFoundAfterSave && r.findClientExternalIDCalls > 1 {
		return r.clientStored, true, nil
	}
	return r.clientStored, r.clientResultFound, nil
}

func (r *fakeRepository) ListHistory(
	ctx context.Context,
	filter HistoryFilter,
) ([]StoredHistoryItem, error) {
	if r.err != nil {
		return nil, r.err
	}
	r.historyFilter = filter
	return r.historyItems, nil
}

func (r *fakeRepository) GetClient(
	ctx context.Context,
	clientID uint,
) (models.ClientApplication, bool, error) {
	if r.err != nil {
		return models.ClientApplication{}, false, r.err
	}
	r.clientID = clientID
	return r.webhookClient, r.webhookClientFound, nil
}

func (r *fakeRepository) SaveWebhookDelivery(
	ctx context.Context,
	delivery *models.WebhookDelivery,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.err != nil {
		return r.err
	}
	copiedDelivery := *delivery
	r.webhookDelivery = &copiedDelivery
	r.webhookDeliverySaveContextErr = ctx.Err()
	return nil
}

func (r *fakeRepository) ListWebhookDeliveries(
	ctx context.Context,
	filter WebhookDeliveryFilter,
) ([]models.WebhookDelivery, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.err != nil {
		return nil, r.err
	}
	r.webhookDeliveryListFilter = filter
	return r.webhookDeliveries, nil
}

func (r *fakeRepository) ListRetryableWebhookDeliveries(
	ctx context.Context,
	limit int,
	maxAttempts int,
	staleRetryingBefore time.Time,
) ([]models.WebhookDelivery, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.err != nil {
		return nil, r.err
	}
	r.webhookRetryLimit = limit
	r.webhookRetryMaxAttempts = maxAttempts
	r.webhookRetryStaleBefore = staleRetryingBefore
	return r.retryableWebhookDeliveries, nil
}

func (r *fakeRepository) ClaimFailedWebhookDelivery(
	ctx context.Context,
	deliveryID uint,
	maxAttempts int,
	attemptedAt time.Time,
) (models.WebhookDelivery, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.err != nil {
		return models.WebhookDelivery{}, r.err
	}
	r.webhookDeliveryID = deliveryID
	if r.claimWebhookDeliveryOnce {
		if r.webhookDeliveryClaimed {
			return models.WebhookDelivery{}, apperrors.Conflict("Webhook delivery is not failed")
		}
		r.webhookDeliveryClaimed = true
	}

	claimed := r.webhookDeliveryStored
	if maxAttempts > 0 && claimed.AttemptCount >= maxAttempts {
		return models.WebhookDelivery{}, apperrors.Conflict("Webhook delivery is not failed")
	}
	claimed.Status = string(WebhookDeliveryRetrying)
	claimed.AttemptCount++
	claimed.LastAttemptAt = attemptedAt
	r.webhookDeliveryStored = claimed
	return claimed, nil
}

func (r *fakeRepository) GetWebhookDelivery(
	ctx context.Context,
	deliveryID uint,
) (models.WebhookDelivery, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.err != nil {
		return models.WebhookDelivery{}, r.err
	}
	r.webhookDeliveryID = deliveryID
	return r.webhookDeliveryStored, nil
}

func (r *fakeRepository) UpdateWebhookDeliveryAttempt(
	ctx context.Context,
	deliveryID uint,
	status WebhookDeliveryStatus,
	httpStatus *int,
	errorMessage string,
	attemptedAt time.Time,
) (models.WebhookDelivery, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.err != nil {
		return models.WebhookDelivery{}, r.err
	}
	r.webhookDeliveryID = deliveryID
	r.webhookDeliveryStatus = status
	r.webhookDeliveryHTTPStatus = httpStatus
	r.webhookDeliveryError = errorMessage
	r.webhookDeliveryAttemptedAt = attemptedAt
	r.webhookDeliveryUpdateContextErr = ctx.Err()

	updated := r.webhookDeliveryStored
	updated.Status = string(status)
	updated.LastAttemptAt = attemptedAt
	updated.HTTPStatus = httpStatus
	updated.ErrorMessage = errorMessage
	return updated, nil
}

func (r *fakeRepository) ListReviewCases(
	ctx context.Context,
	filter ReviewCaseFilter,
) (StoredReviewCasePage, error) {
	if r.err != nil {
		return StoredReviewCasePage{}, r.err
	}
	r.reviewStatus = filter.Status
	r.reviewFilter = filter
	return StoredReviewCasePage{Items: r.reviewCases, NextCursor: r.reviewNextCursor}, nil
}

func (r *fakeRepository) GetReviewCase(ctx context.Context, caseID uint) (StoredReviewCase, error) {
	if r.err != nil {
		return StoredReviewCase{}, r.err
	}
	r.caseID = caseID
	return r.reviewCaseStored, nil
}

func (r *fakeRepository) GetStats(ctx context.Context) (StoredStats, error) {
	if r.err != nil {
		return StoredStats{}, r.err
	}
	return r.stats, nil
}

func (r *fakeRepository) FinalizeReviewCase(
	ctx context.Context,
	caseID uint,
	reviewerID uint,
	status ReviewStatus,
	finalDecision Decision,
	notes string,
	reviewedAt time.Time,
) (StoredReviewCase, error) {
	if r.err != nil {
		return StoredReviewCase{}, r.err
	}
	r.caseID = caseID
	r.reviewerID = reviewerID
	r.finalStatus = status
	r.finalDecision = finalDecision
	r.notes = notes
	r.reviewedAt = reviewedAt
	return r.finalized, nil
}

func prometheusCounterValue(t *testing.T, name string, labels map[string]string) float64 {
	t.Helper()

	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather prometheus metrics: %v", err)
	}

	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.GetMetric() {
			if !prometheusMetricLabelsMatch(metric.GetLabel(), labels) {
				continue
			}
			if metric.GetCounter() == nil {
				t.Fatalf("metric %s with labels %#v is not a counter", name, labels)
			}
			return metric.GetCounter().GetValue()
		}
	}

	return 0
}

func prometheusHistogramCount(t *testing.T, name string, labels map[string]string) float64 {
	t.Helper()

	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather prometheus metrics: %v", err)
	}

	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.GetMetric() {
			if !prometheusMetricLabelsMatch(metric.GetLabel(), labels) {
				continue
			}
			if metric.GetHistogram() == nil {
				t.Fatalf("metric %s with labels %#v is not a histogram", name, labels)
			}
			return float64(metric.GetHistogram().GetSampleCount())
		}
	}

	return 0
}

func prometheusMetricLabelsMatch(metricLabels []*io_prometheus_client.LabelPair, labels map[string]string) bool {
	if len(metricLabels) != len(labels) {
		return false
	}

	for _, label := range metricLabels {
		want, exists := labels[label.GetName()]
		if !exists || want != label.GetValue() {
			return false
		}
	}

	return true
}

func uintPtr(value uint) *uint {
	return &value
}

type fakeWebhookDispatcher struct {
	mu                 sync.Mutex
	clients            []models.ClientApplication
	payloads           []webhooks.FinalDecisionPayload
	err                error
	afterDispatch      func()
	dispatchContextErr error
}

func (d *fakeWebhookDispatcher) DispatchFinalDecision(
	ctx context.Context,
	client models.ClientApplication,
	payload webhooks.FinalDecisionPayload,
) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.err != nil {
		return d.err
	}
	d.dispatchContextErr = ctx.Err()
	d.clients = append(d.clients, client)
	d.payloads = append(d.payloads, payload)
	if d.afterDispatch != nil {
		d.afterDispatch()
	}
	return nil
}

func (d *fakeWebhookDispatcher) DispatchCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()

	return len(d.payloads)
}
