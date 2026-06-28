package app

import (
	"strings"
	"testing"

	"hatesentry/internal/config"
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
