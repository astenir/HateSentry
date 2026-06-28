package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"hatesentry/internal/config"
	"hatesentry/internal/moderation"
)

func TestModerationPolicySetFromConfig(t *testing.T) {
	policies, err := moderationPolicySetFromConfig(config.ModerationConfig{
		Policy: config.ModerationPolicyConfig{
			Version:         "default-v1",
			ReviewThreshold: 0.4,
			BlockThreshold:  0.75,
		},
		Policies: []config.ModerationPolicyConfig{
			{
				Version:         "strict-v1",
				ReviewThreshold: 0.2,
				BlockThreshold:  0.5,
			},
		},
	})
	if err != nil {
		t.Fatalf("moderationPolicySetFromConfig() error = %v", err)
	}

	policy, err := policies.PolicyForVersion("strict-v1")
	if err != nil {
		t.Fatalf("PolicyForVersion() error = %v", err)
	}
	if policy.BlockThreshold != 0.5 {
		t.Fatalf("BlockThreshold = %v, want 0.5", policy.BlockThreshold)
	}
}

func TestModerationPolicySetFromConfigRejectsDuplicateVersion(t *testing.T) {
	_, err := moderationPolicySetFromConfig(config.ModerationConfig{
		Policy: config.ModerationPolicyConfig{
			Version:         "default-v1",
			ReviewThreshold: 0.4,
			BlockThreshold:  0.75,
		},
		Policies: []config.ModerationPolicyConfig{
			{
				Version:         "default-v1",
				ReviewThreshold: 0.2,
				BlockThreshold:  0.5,
			},
		},
	})
	if err == nil {
		t.Fatal("moderationPolicySetFromConfig() error = nil, want duplicate version error")
	}
	if !strings.Contains(err.Error(), `duplicate policy version "default-v1"`) {
		t.Fatalf("moderationPolicySetFromConfig() error = %q, want duplicate version detail", err.Error())
	}
}

func TestStartWebhookRetryWorkerSkipsDisabledConfig(t *testing.T) {
	started := startWebhookRetryWorker(
		context.Background(),
		nil,
		&fakeWebhookRetryService{},
		config.WebhookRetryConfig{
			Enabled:     false,
			Interval:    time.Minute,
			BatchSize:   10,
			MaxAttempts: 3,
		},
	)

	if started {
		t.Fatal("startWebhookRetryWorker() = true, want false")
	}
}

func TestRunWebhookRetryBatchUsesConfig(t *testing.T) {
	service := &fakeWebhookRetryService{
		output: moderation.WebhookRetryOutput{
			Attempted: 1,
			Succeeded: 1,
		},
	}
	cfg := config.WebhookRetryConfig{
		Enabled:     true,
		Interval:    time.Minute,
		BatchSize:   7,
		MaxAttempts: 4,
	}

	runWebhookRetryBatch(context.Background(), nil, service, cfg)

	if service.input.Limit != 7 {
		t.Fatalf("retry input limit = %d, want 7", service.input.Limit)
	}
	if service.input.MaxAttempts != 4 {
		t.Fatalf("retry input max attempts = %d, want 4", service.input.MaxAttempts)
	}
}

type fakeWebhookRetryService struct {
	input  moderation.WebhookRetryInput
	output moderation.WebhookRetryOutput
	err    error
}

func (s *fakeWebhookRetryService) RetryFailedWebhookDeliveries(
	ctx context.Context,
	input moderation.WebhookRetryInput,
) (moderation.WebhookRetryOutput, error) {
	s.input = input
	if s.err != nil {
		return moderation.WebhookRetryOutput{}, s.err
	}
	return s.output, nil
}
