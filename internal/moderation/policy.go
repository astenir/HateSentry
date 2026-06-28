package moderation

import (
	"fmt"
	"math"
)

// Policy converts provider risk signals into service-owned decisions.
type Policy struct {
	Version         string
	ReviewThreshold float64
	BlockThreshold  float64
}

// DefaultPolicy returns the first-version text moderation policy.
func DefaultPolicy() Policy {
	return Policy{
		Version:         "default-v1",
		ReviewThreshold: 0.4,
		BlockThreshold:  0.75,
	}
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
	if p.Version == "" {
		return fmt.Errorf("policy version is required")
	}
	if !validScore(p.ReviewThreshold) {
		return fmt.Errorf("review threshold must be between 0 and 1")
	}
	if !validScore(p.BlockThreshold) {
		return fmt.Errorf("block threshold must be between 0 and 1")
	}
	if p.ReviewThreshold > p.BlockThreshold {
		return fmt.Errorf("review threshold must not exceed block threshold")
	}

	return nil
}

func validScore(score float64) bool {
	return !math.IsNaN(score) && !math.IsInf(score, 0) && score >= 0 && score <= 1
}
