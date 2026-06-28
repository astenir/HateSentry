package moderation

import (
	"math"
	"strings"
	"testing"
)

func TestDefaultPolicyDecide(t *testing.T) {
	policy := DefaultPolicy()

	tests := []struct {
		name      string
		riskScore float64
		want      Decision
	}{
		{
			name:      "below review threshold allows",
			riskScore: 0.39,
			want:      DecisionAllow,
		},
		{
			name:      "at review threshold reviews",
			riskScore: 0.4,
			want:      DecisionReview,
		},
		{
			name:      "below block threshold reviews",
			riskScore: 0.749,
			want:      DecisionReview,
		},
		{
			name:      "at block threshold blocks",
			riskScore: 0.75,
			want:      DecisionBlock,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := policy.Decide(ProviderSuggestion{
				RiskScore: tt.riskScore,
				Labels:    []string{"safe"},
				Reason:    "parsed provider reason",
			})
			if err != nil {
				t.Fatalf("Decide() error = %v", err)
			}
			if result.Decision != tt.want {
				t.Fatalf("Decision = %q, want %q", result.Decision, tt.want)
			}
			if result.PolicyVersion != "default-v1" {
				t.Fatalf("PolicyVersion = %q, want default-v1", result.PolicyVersion)
			}
		})
	}
}

func TestNewPolicyValidatesConfiguredPolicy(t *testing.T) {
	policy, err := NewPolicy(" custom-v1 ", 0.25, 0.8)
	if err != nil {
		t.Fatalf("NewPolicy() error = %v", err)
	}
	if policy.Version != "custom-v1" {
		t.Fatalf("Version = %q, want custom-v1", policy.Version)
	}
	if policy.ReviewThreshold != 0.25 {
		t.Fatalf("ReviewThreshold = %v, want 0.25", policy.ReviewThreshold)
	}
	if policy.BlockThreshold != 0.8 {
		t.Fatalf("BlockThreshold = %v, want 0.8", policy.BlockThreshold)
	}

	if _, err := NewPolicy("invalid", 0.9, 0.8); err == nil {
		t.Fatal("NewPolicy() error = nil, want threshold order error")
	}
	if _, err := NewPolicy(" ", 0.25, 0.8); err == nil {
		t.Fatal("NewPolicy() error = nil, want policy version error")
	}
}

func TestPolicySetResolvesConfiguredVersions(t *testing.T) {
	defaultPolicy := DefaultPolicy()
	strictPolicy, err := NewPolicy("strict-v1", 0.2, 0.5)
	if err != nil {
		t.Fatalf("NewPolicy() error = %v", err)
	}
	policies, err := NewPolicySet(defaultPolicy, strictPolicy)
	if err != nil {
		t.Fatalf("NewPolicySet() error = %v", err)
	}

	resolvedDefault, err := policies.PolicyForVersion("")
	if err != nil {
		t.Fatalf("PolicyForVersion(default) error = %v", err)
	}
	if resolvedDefault.Version != "default-v1" {
		t.Fatalf("default policy = %q, want default-v1", resolvedDefault.Version)
	}

	resolvedStrict, err := policies.PolicyForVersion(" strict-v1 ")
	if err != nil {
		t.Fatalf("PolicyForVersion(strict) error = %v", err)
	}
	if resolvedStrict.BlockThreshold != 0.5 {
		t.Fatalf("strict block threshold = %v, want 0.5", resolvedStrict.BlockThreshold)
	}
}

func TestPolicySetRejectsDuplicateAndUnknownVersions(t *testing.T) {
	defaultPolicy := DefaultPolicy()
	duplicatePolicy, err := NewPolicy("default-v1", 0.2, 0.5)
	if err != nil {
		t.Fatalf("NewPolicy() error = %v", err)
	}
	if _, err := NewPolicySet(defaultPolicy, duplicatePolicy); err == nil {
		t.Fatal("NewPolicySet() error = nil, want duplicate policy error")
	}

	policies, err := NewPolicySet(defaultPolicy)
	if err != nil {
		t.Fatalf("NewPolicySet() error = %v", err)
	}
	_, err = policies.PolicyForVersion("missing-v1")
	if err == nil {
		t.Fatal("PolicyForVersion() error = nil, want unknown policy error")
	}
	if !strings.Contains(err.Error(), `policy_version "missing-v1" is not configured`) {
		t.Fatalf("PolicyForVersion() error = %q, want unknown policy detail", err.Error())
	}
}

func TestPolicySetListReturnsDefaultFirstThenSortedPolicies(t *testing.T) {
	defaultPolicy := DefaultPolicy()
	strictPolicy, err := NewPolicy("strict-v1", 0.2, 0.5)
	if err != nil {
		t.Fatalf("NewPolicy() strict error = %v", err)
	}
	lenientPolicy, err := NewPolicy("lenient-v1", 0.6, 0.9)
	if err != nil {
		t.Fatalf("NewPolicy() lenient error = %v", err)
	}
	policies, err := NewPolicySet(defaultPolicy, strictPolicy, lenientPolicy)
	if err != nil {
		t.Fatalf("NewPolicySet() error = %v", err)
	}

	output := policies.List()

	if len(output) != 3 {
		t.Fatalf("len(output) = %d, want 3", len(output))
	}
	if output[0].Version != "default-v1" || !output[0].Default {
		t.Fatalf("first policy = %#v, want default-v1 marked default", output[0])
	}
	if output[1].Version != "lenient-v1" || output[1].Default {
		t.Fatalf("second policy = %#v, want lenient-v1 non-default", output[1])
	}
	if output[2].Version != "strict-v1" || output[2].Default {
		t.Fatalf("third policy = %#v, want strict-v1 non-default", output[2])
	}
	if output[2].ReviewThreshold != 0.2 || output[2].BlockThreshold != 0.5 {
		t.Fatalf("strict thresholds = %#v, want 0.2/0.5", output[2])
	}
}

func TestPolicyValidateRejectsInvalidThresholds(t *testing.T) {
	tests := []struct {
		name    string
		policy  Policy
		wantErr string
	}{
		{
			name: "missing version",
			policy: Policy{
				ReviewThreshold: 0.4,
				BlockThreshold:  0.75,
			},
			wantErr: "policy version is required",
		},
		{
			name: "review below zero",
			policy: Policy{
				Version:         "test",
				ReviewThreshold: -0.01,
				BlockThreshold:  0.75,
			},
			wantErr: "review threshold must be between 0 and 1",
		},
		{
			name: "review is nan",
			policy: Policy{
				Version:         "test",
				ReviewThreshold: math.NaN(),
				BlockThreshold:  0.75,
			},
			wantErr: "review threshold must be between 0 and 1",
		},
		{
			name: "review is infinity",
			policy: Policy{
				Version:         "test",
				ReviewThreshold: math.Inf(1),
				BlockThreshold:  0.75,
			},
			wantErr: "review threshold must be between 0 and 1",
		},
		{
			name: "block above one",
			policy: Policy{
				Version:         "test",
				ReviewThreshold: 0.4,
				BlockThreshold:  1.01,
			},
			wantErr: "block threshold must be between 0 and 1",
		},
		{
			name: "block is nan",
			policy: Policy{
				Version:         "test",
				ReviewThreshold: 0.4,
				BlockThreshold:  math.NaN(),
			},
			wantErr: "block threshold must be between 0 and 1",
		},
		{
			name: "review exceeds block",
			policy: Policy{
				Version:         "test",
				ReviewThreshold: 0.8,
				BlockThreshold:  0.75,
			},
			wantErr: "review threshold must be less than block threshold",
		},
		{
			name: "review equals block",
			policy: Policy{
				Version:         "test",
				ReviewThreshold: 0.75,
				BlockThreshold:  0.75,
			},
			wantErr: "review threshold must be less than block threshold",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.policy.Validate()
			if err == nil {
				t.Fatal("Validate() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestPolicyDecideRejectsNonFiniteRiskScore(t *testing.T) {
	tests := []struct {
		name      string
		riskScore float64
	}{
		{
			name:      "nan",
			riskScore: math.NaN(),
		},
		{
			name:      "positive infinity",
			riskScore: math.Inf(1),
		},
		{
			name:      "negative infinity",
			riskScore: math.Inf(-1),
		},
	}

	policy := DefaultPolicy()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := policy.Decide(ProviderSuggestion{
				RiskScore: tt.riskScore,
				Labels:    []string{"safe"},
				Reason:    "parsed provider reason",
			})
			if err == nil {
				t.Fatal("Decide() error = nil, want invalid risk_score error")
			}
			if !strings.Contains(err.Error(), "risk_score must be between 0 and 1") {
				t.Fatalf("Decide() error = %q, want risk_score detail", err.Error())
			}
		})
	}
}
