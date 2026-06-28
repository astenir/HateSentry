package moderation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "hatesentry/internal/errors"
	"hatesentry/internal/models"
)

const (
	defaultSource      = "api"
	maxContentLength   = 10000
	maxRequestIDLength = 64
	maxSourceLength    = 50
	maxMetadataLength  = 128
)

// Analyzer classifies text and returns a normalized provider suggestion.
type Analyzer interface {
	AnalyzeText(ctx context.Context, content string) (ProviderSuggestion, ProviderInfo, error)
}

// Repository persists moderation audit records.
type Repository interface {
	SaveCheck(ctx context.Context, request *models.ModerationRequest, result *models.ModerationResult) error
	GetResult(ctx context.Context, userID uint, requestID string) (StoredResult, error)
}

// StoredResult is the repository representation of a persisted moderation check.
type StoredResult struct {
	Request models.ModerationRequest
	Result  models.ModerationResult
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

// ResultOutput is the stable public representation of a stored moderation result.
type ResultOutput struct {
	RequestID     string    `json:"request_id"`
	Content       string    `json:"content"`
	Source        string    `json:"source"`
	ExternalID    string    `json:"external_id,omitempty"`
	ActorID       string    `json:"actor_id,omitempty"`
	Status        string    `json:"status"`
	Provider      string    `json:"provider"`
	Model         string    `json:"model"`
	Decision      Decision  `json:"decision"`
	RiskScore     float64   `json:"risk_score"`
	Labels        []string  `json:"labels"`
	Reason        string    `json:"reason"`
	PolicyVersion string    `json:"policy_version"`
	CreatedAt     time.Time `json:"created_at"`
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

// GetResult retrieves a stored moderation result owned by the authenticated user.
func (s *Service) GetResult(ctx context.Context, userID uint, requestID string) (ResultOutput, error) {
	requestID = strings.TrimSpace(requestID)
	if userID == 0 {
		return ResultOutput{}, apperrors.Unauthorized("User not authenticated")
	}
	if requestID == "" {
		return ResultOutput{}, apperrors.ValidationError("request_id is required")
	}
	if len(requestID) > maxRequestIDLength {
		return ResultOutput{}, apperrors.ValidationError(
			fmt.Sprintf("request_id must not exceed %d characters", maxRequestIDLength),
		)
	}
	if s.repository == nil {
		return ResultOutput{}, apperrors.ConfigurationError("moderation repository is not configured")
	}

	stored, err := s.repository.GetResult(ctx, userID, requestID)
	if err != nil {
		return ResultOutput{}, err
	}

	labels, err := decodeLabels(stored.Result.Labels)
	if err != nil {
		return ResultOutput{}, err
	}

	return ResultOutput{
		RequestID:     stored.Request.RequestID,
		Content:       stored.Request.Content,
		Source:        stored.Request.Source,
		ExternalID:    stored.Request.ExternalID,
		ActorID:       stored.Request.ActorID,
		Status:        stored.Request.Status,
		Provider:      stored.Result.Provider,
		Model:         stored.Result.Model,
		Decision:      Decision(stored.Result.Decision),
		RiskScore:     stored.Result.RiskScore,
		Labels:        labels,
		Reason:        stored.Result.Reason,
		PolicyVersion: stored.Result.PolicyVersion,
		CreatedAt:     stored.Result.CreatedAt,
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

func decodeLabels(labelsJSON string) ([]string, error) {
	labels := []string{}
	if strings.TrimSpace(labelsJSON) == "" {
		return labels, nil
	}
	if err := json.Unmarshal([]byte(labelsJSON), &labels); err != nil {
		return nil, apperrors.Internal("failed to decode moderation labels").WithDetails(err.Error())
	}

	return labels, nil
}
