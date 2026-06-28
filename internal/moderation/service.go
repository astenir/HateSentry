package moderation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	apperrors "hatesentry/internal/errors"
	"hatesentry/internal/models"
)

const (
	defaultSource     = "api"
	maxContentLength  = 10000
	maxSourceLength   = 50
	maxMetadataLength = 128
)

// Analyzer classifies text and returns a normalized provider suggestion.
type Analyzer interface {
	AnalyzeText(ctx context.Context, content string) (ProviderSuggestion, ProviderInfo, error)
}

// Repository persists moderation audit records.
type Repository interface {
	SaveCheck(ctx context.Context, request *models.ModerationRequest, result *models.ModerationResult) error
}

// Service coordinates provider analysis, policy decisions, and persistence.
type Service struct {
	analyzer   Analyzer
	repository Repository
	policy     Policy
}

// NewService creates a moderation service.
func NewService(analyzer Analyzer, repository Repository, policy Policy) *Service {
	return &Service{
		analyzer:   analyzer,
		repository: repository,
		policy:     policy,
	}
}

// CheckInput is the service input for a text moderation check.
type CheckInput struct {
	UserID     uint
	Content    string
	Source     string
	ExternalID string
	ActorID    string
}

// CheckOutput is the stable public result returned by the moderation API.
type CheckOutput struct {
	RequestID     string   `json:"request_id"`
	Decision      Decision `json:"decision"`
	RiskScore     float64  `json:"risk_score"`
	Labels        []string `json:"labels"`
	Reason        string   `json:"reason"`
	PolicyVersion string   `json:"policy_version"`
}

// Check performs a synchronous text moderation workflow and stores audit records.
func (s *Service) Check(ctx context.Context, input CheckInput) (CheckOutput, error) {
	normalized, err := validateCheckInput(input)
	if err != nil {
		return CheckOutput{}, err
	}
	if s.analyzer == nil {
		return CheckOutput{}, apperrors.ConfigurationError("moderation analyzer is not configured")
	}
	if s.repository == nil {
		return CheckOutput{}, apperrors.ConfigurationError("moderation repository is not configured")
	}

	suggestion, provider, err := s.analyzer.AnalyzeText(ctx, normalized.Content)
	if err != nil {
		return CheckOutput{}, err
	}

	decision, err := s.policy.Decide(suggestion)
	if err != nil {
		return CheckOutput{}, apperrors.ValidationError("invalid moderation policy input").WithDetails(err.Error())
	}

	requestID := uuid.New().String()
	labelsJSON, err := json.Marshal(decision.Labels)
	if err != nil {
		return CheckOutput{}, apperrors.Internal("failed to encode moderation labels").WithDetails(err.Error())
	}

	request := &models.ModerationRequest{
		RequestID:  requestID,
		UserID:     normalized.UserID,
		Content:    normalized.Content,
		Source:     normalized.Source,
		ExternalID: normalized.ExternalID,
		ActorID:    normalized.ActorID,
		Status:     "completed",
	}
	result := &models.ModerationResult{
		RequestID:     requestID,
		UserID:        normalized.UserID,
		Provider:      provider.Provider,
		Model:         provider.Model,
		RawOutput:     suggestion.RawOutput,
		RiskScore:     decision.RiskScore,
		Labels:        string(labelsJSON),
		Decision:      string(decision.Decision),
		Reason:        decision.Reason,
		PolicyVersion: decision.PolicyVersion,
	}

	if err := s.repository.SaveCheck(ctx, request, result); err != nil {
		return CheckOutput{}, err
	}

	return CheckOutput{
		RequestID:     requestID,
		Decision:      decision.Decision,
		RiskScore:     decision.RiskScore,
		Labels:        decision.Labels,
		Reason:        decision.Reason,
		PolicyVersion: decision.PolicyVersion,
	}, nil
}

func validateCheckInput(input CheckInput) (CheckInput, error) {
	input.Content = strings.TrimSpace(input.Content)
	input.Source = strings.TrimSpace(input.Source)
	input.ExternalID = strings.TrimSpace(input.ExternalID)
	input.ActorID = strings.TrimSpace(input.ActorID)

	if input.UserID == 0 {
		return CheckInput{}, apperrors.Unauthorized("User not authenticated")
	}
	if input.Content == "" {
		return CheckInput{}, apperrors.ValidationError("content is required")
	}
	if len(input.Content) > maxContentLength {
		return CheckInput{}, apperrors.ValidationError(
			fmt.Sprintf("content must not exceed %d characters", maxContentLength),
		)
	}
	if input.Source == "" {
		input.Source = defaultSource
	}
	if len(input.Source) > maxSourceLength {
		return CheckInput{}, apperrors.ValidationError(
			fmt.Sprintf("source must not exceed %d characters", maxSourceLength),
		)
	}
	if len(input.ExternalID) > maxMetadataLength {
		return CheckInput{}, apperrors.ValidationError(
			fmt.Sprintf("external_id must not exceed %d characters", maxMetadataLength),
		)
	}
	if len(input.ActorID) > maxMetadataLength {
		return CheckInput{}, apperrors.ValidationError(
			fmt.Sprintf("actor_id must not exceed %d characters", maxMetadataLength),
		)
	}

	return input, nil
}
