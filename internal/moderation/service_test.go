package moderation

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"hatesentry/internal/models"
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

type fakeAnalyzer struct {
	suggestion ProviderSuggestion
	provider   ProviderInfo
	err        error
}

func (a fakeAnalyzer) AnalyzeText(ctx context.Context, content string) (ProviderSuggestion, ProviderInfo, error) {
	if a.err != nil {
		return ProviderSuggestion{}, ProviderInfo{}, a.err
	}
	return a.suggestion, a.provider, nil
}

type fakeRepository struct {
	request *models.ModerationRequest
	result  *models.ModerationResult
	err     error
}

func (r *fakeRepository) SaveCheck(
	ctx context.Context,
	request *models.ModerationRequest,
	result *models.ModerationResult,
) error {
	if r.err != nil {
		return r.err
	}

	copiedRequest := *request
	copiedResult := *result
	r.request = &copiedRequest
	r.result = &copiedResult
	return nil
}
