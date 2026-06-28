package moderation

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// Policy converts provider risk signals into service-owned decisions.
type Policy struct {
	Version         string
	ReviewThreshold float64
	BlockThreshold  float64
}

// PolicySet resolves policy versions to concrete threshold policies.
type PolicySet struct {
	defaultPolicy Policy
	policies      map[string]Policy
}

// PolicyConfig describes a configured moderation policy for operator views.
type PolicyConfig struct {
	Version         string  `json:"version"`
	ReviewThreshold float64 `json:"review_threshold"`
	BlockThreshold  float64 `json:"block_threshold"`
	Default         bool    `json:"default"`
}

// DefaultPolicy returns the first-version text moderation policy.
func DefaultPolicy() Policy {
	return Policy{
		Version:         "default-v1",
		ReviewThreshold: 0.4,
		BlockThreshold:  0.75,
	}
}

// NewPolicy validates and returns a configured policy.
func NewPolicy(version string, reviewThreshold, blockThreshold float64) (Policy, error) {
	policy := Policy{
		Version:         strings.TrimSpace(version),
		ReviewThreshold: reviewThreshold,
		BlockThreshold:  blockThreshold,
	}
	if err := policy.Validate(); err != nil {
		return Policy{}, err
	}

	return policy, nil
}

// NewPolicySet validates and indexes the default policy plus optional named policies.
func NewPolicySet(defaultPolicy Policy, policies ...Policy) (PolicySet, error) {
	if err := defaultPolicy.Validate(); err != nil {
		return PolicySet{}, err
	}

	registered := map[string]Policy{
		defaultPolicy.Version: defaultPolicy,
	}
	for _, policy := range policies {
		if err := policy.Validate(); err != nil {
			return PolicySet{}, err
		}
		if _, exists := registered[policy.Version]; exists {
			return PolicySet{}, fmt.Errorf("duplicate policy version %q", policy.Version)
		}
		registered[policy.Version] = policy
	}

	return PolicySet{
		defaultPolicy: defaultPolicy,
		policies:      registered,
	}, nil
}

// PolicyForVersion returns the default policy for an empty version, or a configured policy version.
func (ps PolicySet) PolicyForVersion(version string) (Policy, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return ps.defaultPolicy, nil
	}

	policy, exists := ps.policies[version]
	if !exists {
		return Policy{}, fmt.Errorf("policy_version %q is not configured", version)
	}

	return policy, nil
}

// ValidatePolicyVersion verifies that a client policy assignment can resolve at runtime.
func (ps PolicySet) ValidatePolicyVersion(version string) error {
	_, err := ps.PolicyForVersion(version)
	return err
}

// List returns configured policies in a stable order with the default policy first.
func (ps PolicySet) List() []PolicyConfig {
	output := []PolicyConfig{}
	seen := map[string]bool{}

	if strings.TrimSpace(ps.defaultPolicy.Version) != "" {
		output = append(output, policyConfig(ps.defaultPolicy, true))
		seen[ps.defaultPolicy.Version] = true
	}

	for version, policy := range ps.policies {
		if seen[version] {
			continue
		}
		output = append(output, policyConfig(policy, false))
	}

	sort.Slice(output, func(i, j int) bool {
		if output[i].Default != output[j].Default {
			return output[i].Default
		}
		return output[i].Version < output[j].Version
	})

	return output
}

// Decide returns the allow/review/block action for a parsed provider suggestion.
func (p Policy) Decide(suggestion ProviderSuggestion) (DecisionResult, error) {
	if err := p.Validate(); err != nil {
		return DecisionResult{}, err
	}
	if !validScore(suggestion.RiskScore) {
		return DecisionResult{}, fmt.Errorf("risk_score must be between 0 and 1")
	}

	decision := DecisionAllow
	switch {
	case suggestion.RiskScore >= p.BlockThreshold:
		decision = DecisionBlock
	case suggestion.RiskScore >= p.ReviewThreshold:
		decision = DecisionReview
	}

	return DecisionResult{
		Decision:      decision,
		RiskScore:     suggestion.RiskScore,
		Labels:        append([]string{}, suggestion.Labels...),
		Reason:        suggestion.Reason,
		PolicyVersion: p.Version,
	}, nil
}

// Validate checks policy threshold consistency before decisions are made.
func (p Policy) Validate() error {
	if strings.TrimSpace(p.Version) == "" {
		return fmt.Errorf("policy version is required")
	}
	if !validScore(p.ReviewThreshold) {
		return fmt.Errorf("review threshold must be between 0 and 1")
	}
	if !validScore(p.BlockThreshold) {
		return fmt.Errorf("block threshold must be between 0 and 1")
	}
	if p.ReviewThreshold >= p.BlockThreshold {
		return fmt.Errorf("review threshold must be less than block threshold")
	}

	return nil
}

func policyConfig(policy Policy, isDefault bool) PolicyConfig {
	return PolicyConfig{
		Version:         policy.Version,
		ReviewThreshold: policy.ReviewThreshold,
		BlockThreshold:  policy.BlockThreshold,
		Default:         isDefault,
	}
}

func validScore(score float64) bool {
	return !math.IsNaN(score) && !math.IsInf(score, 0) && score >= 0 && score <= 1
}
