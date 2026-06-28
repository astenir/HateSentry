package moderation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	apperrors "hatesentry/internal/errors"
	"hatesentry/internal/models"
	"hatesentry/internal/webhooks"
)

const (
	defaultSource                   = "api"
	maxContentLength                = 10000
	maxRequestIDLength              = 64
	maxSourceLength                 = 50
	maxMetadataLength               = 128
	maxReviewNotesLen               = 2000
	maxWebhookDeliveryListLimit     = 100
	defaultWebhookDeliveryListLimit = 50
	webhookDeliveryAttemptTimeout   = 6 * time.Second
	webhookDeliveryRecordTimeout    = 5 * time.Second
	webhookRetryStatusUpdateTimeout = 5 * time.Second
	webhookRetryLease               = 2 * time.Minute
)

// Analyzer classifies text and returns a normalized provider suggestion.
type Analyzer interface {
	AnalyzeText(ctx context.Context, content string) (ProviderSuggestion, ProviderInfo, error)
}

// Repository persists moderation audit records.
type Repository interface {
	SaveCheck(
		ctx context.Context,
		request *models.ModerationRequest,
		result *models.ModerationResult,
		reviewCase *models.ReviewCase,
	) error
	GetResult(ctx context.Context, userID uint, requestID string) (StoredResult, error)
	FindResultByClientExternalID(ctx context.Context, clientID uint, externalID string) (StoredResult, bool, error)
	GetClient(ctx context.Context, clientID uint) (models.ClientApplication, bool, error)
	SaveWebhookDelivery(ctx context.Context, delivery *models.WebhookDelivery) error
	GetWebhookDelivery(ctx context.Context, deliveryID uint) (models.WebhookDelivery, error)
	ListWebhookDeliveries(
		ctx context.Context,
		status WebhookDeliveryStatus,
		limit int,
	) ([]models.WebhookDelivery, error)
	ClaimFailedWebhookDelivery(
		ctx context.Context,
		deliveryID uint,
		attemptedAt time.Time,
	) (models.WebhookDelivery, error)
	UpdateWebhookDeliveryAttempt(
		ctx context.Context,
		deliveryID uint,
		status WebhookDeliveryStatus,
		httpStatus *int,
		errorMessage string,
		attemptedAt time.Time,
	) (models.WebhookDelivery, error)
	GetStats(ctx context.Context) (StoredStats, error)
	ListReviewCases(ctx context.Context, status ReviewStatus) ([]StoredReviewCase, error)
	FinalizeReviewCase(
		ctx context.Context,
		caseID uint,
		reviewerID uint,
		status ReviewStatus,
		finalDecision Decision,
		notes string,
		reviewedAt time.Time,
	) (StoredReviewCase, error)
}

// StoredResult is the repository representation of a persisted moderation check.
type StoredResult struct {
	Request models.ModerationRequest
	Result  models.ModerationResult
}

// StoredReviewCase is the repository representation of a persisted review case.
type StoredReviewCase struct {
	Case    models.ReviewCase
	Request models.ModerationRequest
	Result  models.ModerationResult
}

// StoredStats is the repository representation for moderation operations metrics.
type StoredStats struct {
	TotalModerated     int64
	PolicyAllowed      int64
	PolicyBlocked      int64
	ReviewFinalAllowed int64
	ReviewFinalBlocked int64
	PendingReview      int64
	Reviewed           int64
	Mistakes           int64
}

// Service coordinates provider analysis, policy decisions, and persistence.
type Service struct {
	analyzer          Analyzer
	repository        Repository
	policy            Policy
	webhookDispatcher webhooks.Dispatcher
}

// NewService creates a moderation service.
func NewService(
	analyzer Analyzer,
	repository Repository,
	policy Policy,
	webhookDispatchers ...webhooks.Dispatcher,
) *Service {
	var webhookDispatcher webhooks.Dispatcher
	if len(webhookDispatchers) > 0 {
		webhookDispatcher = webhookDispatchers[0]
	}

	return &Service{
		analyzer:          analyzer,
		repository:        repository,
		policy:            policy,
		webhookDispatcher: webhookDispatcher,
	}
}

// CheckInput is the service input for a text moderation check.
type CheckInput struct {
	UserID     uint
	ClientID   uint
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
	ClientID      *uint     `json:"client_id,omitempty"`
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

// ReviewCaseOutput is the public representation of a review queue item.
type ReviewCaseOutput struct {
	ID             uint         `json:"id"`
	RequestID      string       `json:"request_id"`
	UserID         uint         `json:"user_id"`
	ClientID       *uint        `json:"client_id,omitempty"`
	Content        string       `json:"content"`
	Source         string       `json:"source"`
	ExternalID     string       `json:"external_id,omitempty"`
	ActorID        string       `json:"actor_id,omitempty"`
	Status         ReviewStatus `json:"status"`
	PolicyDecision Decision     `json:"policy_decision"`
	FinalDecision  Decision     `json:"final_decision,omitempty"`
	RiskScore      float64      `json:"risk_score"`
	Labels         []string     `json:"labels"`
	Reason         string       `json:"reason"`
	PolicyVersion  string       `json:"policy_version"`
	ReviewerID     *uint        `json:"reviewer_id,omitempty"`
	ReviewNotes    string       `json:"review_notes,omitempty"`
	ReviewedAt     *time.Time   `json:"reviewed_at,omitempty"`
	CreatedAt      time.Time    `json:"created_at"`
}

// StatsOutput is the public representation of moderation and review operations metrics.
type StatsOutput struct {
	TotalModerated int64   `json:"total_moderated"`
	Allowed        int64   `json:"allowed"`
	Blocked        int64   `json:"blocked"`
	PendingReview  int64   `json:"pending_review"`
	Reviewed       int64   `json:"reviewed"`
	Mistakes       int64   `json:"mistakes"`
	MistakeRate    float64 `json:"mistake_rate"`
}

// WebhookDeliveryOutput is the operator view of a persisted webhook delivery.
type WebhookDeliveryOutput struct {
	ID            uint                  `json:"id"`
	DeliveryID    string                `json:"delivery_id"`
	RequestID     string                `json:"request_id"`
	ClientID      uint                  `json:"client_id"`
	Event         string                `json:"event"`
	Status        WebhookDeliveryStatus `json:"status"`
	AttemptCount  int                   `json:"attempt_count"`
	LastAttemptAt time.Time             `json:"last_attempt_at"`
	HTTPStatus    *int                  `json:"http_status,omitempty"`
	ErrorMessage  string                `json:"error_message,omitempty"`
	CreatedAt     time.Time             `json:"created_at"`
	UpdatedAt     time.Time             `json:"updated_at"`
}

// ListWebhookDeliveriesOutput is the operator list response for callback deliveries.
type ListWebhookDeliveriesOutput struct {
	Items []WebhookDeliveryOutput `json:"items"`
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

	if normalized.ClientID != 0 && normalized.ExternalID != "" {
		stored, found, err := s.repository.FindResultByClientExternalID(
			ctx,
			normalized.ClientID,
			normalized.ExternalID,
		)
		if err != nil {
			return CheckOutput{}, err
		}
		if found {
			return checkOutputFromStored(stored)
		}
	}
	idempotencyKey := clientExternalIDIdempotencyKey(normalized.ClientID, normalized.ExternalID)

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
		RequestID:      requestID,
		UserID:         normalized.UserID,
		ClientID:       optionalUint(normalized.ClientID),
		IdempotencyKey: optionalString(idempotencyKey),
		Content:        normalized.Content,
		Source:         normalized.Source,
		ExternalID:     normalized.ExternalID,
		ActorID:        normalized.ActorID,
		Status:         "completed",
	}
	result := &models.ModerationResult{
		RequestID:     requestID,
		UserID:        normalized.UserID,
		ClientID:      optionalUint(normalized.ClientID),
		Provider:      provider.Provider,
		Model:         provider.Model,
		RawOutput:     suggestion.RawOutput,
		RiskScore:     decision.RiskScore,
		Labels:        string(labelsJSON),
		Decision:      string(decision.Decision),
		Reason:        decision.Reason,
		PolicyVersion: decision.PolicyVersion,
	}
	var reviewCase *models.ReviewCase
	if decision.Decision == DecisionReview {
		reviewCase = &models.ReviewCase{
			RequestID: requestID,
			UserID:    normalized.UserID,
			ClientID:  optionalUint(normalized.ClientID),
			Status:    string(ReviewStatusPending),
		}
	}

	if err := s.repository.SaveCheck(ctx, request, result, reviewCase); err != nil {
		if idempotencyKey != "" && isConflictError(err) {
			stored, found, lookupErr := s.repository.FindResultByClientExternalID(
				ctx,
				normalized.ClientID,
				normalized.ExternalID,
			)
			if lookupErr != nil {
				return CheckOutput{}, lookupErr
			}
			if found {
				return checkOutputFromStored(stored)
			}
		}
		return CheckOutput{}, err
	}
	if decision.Decision != DecisionReview {
		s.dispatchFinalDecision(ctx, StoredResult{
			Request: *request,
			Result:  *result,
		}, "", "")
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

	return resultOutputFromStored(stored)
}

func resultOutputFromStored(stored StoredResult) (ResultOutput, error) {
	labels, err := decodeLabels(stored.Result.Labels)
	if err != nil {
		return ResultOutput{}, err
	}

	return ResultOutput{
		RequestID:     stored.Request.RequestID,
		ClientID:      stored.Request.ClientID,
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

func checkOutputFromStored(stored StoredResult) (CheckOutput, error) {
	labels, err := decodeLabels(stored.Result.Labels)
	if err != nil {
		return CheckOutput{}, err
	}

	return CheckOutput{
		RequestID:     stored.Result.RequestID,
		Decision:      Decision(stored.Result.Decision),
		RiskScore:     stored.Result.RiskScore,
		Labels:        labels,
		Reason:        stored.Result.Reason,
		PolicyVersion: stored.Result.PolicyVersion,
	}, nil
}

// ListReviewCases returns moderation review cases for an authenticated operator.
func (s *Service) ListReviewCases(ctx context.Context, reviewerID uint, status string) ([]ReviewCaseOutput, error) {
	if reviewerID == 0 {
		return nil, apperrors.Unauthorized("User not authenticated")
	}
	if s.repository == nil {
		return nil, apperrors.ConfigurationError("moderation repository is not configured")
	}

	reviewStatus, err := normalizeReviewStatus(status)
	if err != nil {
		return nil, err
	}

	storedCases, err := s.repository.ListReviewCases(ctx, reviewStatus)
	if err != nil {
		return nil, err
	}

	output := make([]ReviewCaseOutput, 0, len(storedCases))
	for _, stored := range storedCases {
		item, err := mapReviewCaseOutput(stored)
		if err != nil {
			return nil, err
		}
		output = append(output, item)
	}

	return output, nil
}

// ApproveReviewCase finalizes a pending case as allowed by human review.
func (s *Service) ApproveReviewCase(
	ctx context.Context,
	caseID string,
	reviewerID uint,
	notes string,
) (ReviewCaseOutput, error) {
	return s.finalizeReviewCase(
		ctx,
		caseID,
		reviewerID,
		ReviewStatusApproved,
		DecisionAllow,
		notes,
	)
}

// RejectReviewCase finalizes a pending case as blocked by human review.
func (s *Service) RejectReviewCase(
	ctx context.Context,
	caseID string,
	reviewerID uint,
	notes string,
) (ReviewCaseOutput, error) {
	return s.finalizeReviewCase(
		ctx,
		caseID,
		reviewerID,
		ReviewStatusRejected,
		DecisionBlock,
		notes,
	)
}

// MarkReviewMistake finalizes a pending case and records that policy/provider handling was mistaken.
func (s *Service) MarkReviewMistake(
	ctx context.Context,
	caseID string,
	reviewerID uint,
	finalDecision Decision,
	notes string,
) (ReviewCaseOutput, error) {
	if finalDecision != DecisionAllow && finalDecision != DecisionBlock {
		return ReviewCaseOutput{}, apperrors.ValidationError("final_decision must be allow or block")
	}

	return s.finalizeReviewCase(
		ctx,
		caseID,
		reviewerID,
		ReviewStatusMistake,
		finalDecision,
		notes,
	)
}

func (s *Service) finalizeReviewCase(
	ctx context.Context,
	caseID string,
	reviewerID uint,
	status ReviewStatus,
	finalDecision Decision,
	notes string,
) (ReviewCaseOutput, error) {
	if reviewerID == 0 {
		return ReviewCaseOutput{}, apperrors.Unauthorized("User not authenticated")
	}
	if s.repository == nil {
		return ReviewCaseOutput{}, apperrors.ConfigurationError("moderation repository is not configured")
	}

	parsedCaseID, err := parseReviewCaseID(caseID)
	if err != nil {
		return ReviewCaseOutput{}, err
	}
	notes = strings.TrimSpace(notes)
	if len(notes) > maxReviewNotesLen {
		return ReviewCaseOutput{}, apperrors.ValidationError(
			fmt.Sprintf("review notes must not exceed %d characters", maxReviewNotesLen),
		)
	}

	stored, err := s.repository.FinalizeReviewCase(
		ctx,
		parsedCaseID,
		reviewerID,
		status,
		finalDecision,
		notes,
		time.Now().UTC(),
	)
	if err != nil {
		return ReviewCaseOutput{}, err
	}

	output, err := mapReviewCaseOutput(stored)
	if err != nil {
		return ReviewCaseOutput{}, err
	}

	s.dispatchFinalDecision(ctx, StoredResult{
		Request: stored.Request,
		Result:  stored.Result,
	}, string(status), string(finalDecision))

	return output, nil
}

// GetStats returns moderation and review operations metrics for an authenticated operator.
func (s *Service) GetStats(ctx context.Context, reviewerID uint) (StatsOutput, error) {
	if reviewerID == 0 {
		return StatsOutput{}, apperrors.Unauthorized("User not authenticated")
	}
	if s.repository == nil {
		return StatsOutput{}, apperrors.ConfigurationError("moderation repository is not configured")
	}

	stored, err := s.repository.GetStats(ctx)
	if err != nil {
		return StatsOutput{}, err
	}

	allowed := stored.PolicyAllowed + stored.ReviewFinalAllowed
	blocked := stored.PolicyBlocked + stored.ReviewFinalBlocked
	mistakeRate := 0.0
	if stored.Reviewed > 0 {
		mistakeRate = float64(stored.Mistakes) / float64(stored.Reviewed)
	}

	return StatsOutput{
		TotalModerated: stored.TotalModerated,
		Allowed:        allowed,
		Blocked:        blocked,
		PendingReview:  stored.PendingReview,
		Reviewed:       stored.Reviewed,
		Mistakes:       stored.Mistakes,
		MistakeRate:    mistakeRate,
	}, nil
}

// ListWebhookDeliveries returns recent callback delivery records for an authenticated operator.
func (s *Service) ListWebhookDeliveries(
	ctx context.Context,
	operatorID uint,
	status string,
	limit string,
) (ListWebhookDeliveriesOutput, error) {
	if operatorID == 0 {
		return ListWebhookDeliveriesOutput{}, apperrors.Unauthorized("User not authenticated")
	}
	if s.repository == nil {
		return ListWebhookDeliveriesOutput{}, apperrors.ConfigurationError("moderation repository is not configured")
	}

	normalizedStatus, err := normalizeWebhookDeliveryStatusFilter(status)
	if err != nil {
		return ListWebhookDeliveriesOutput{}, err
	}
	normalizedLimit, err := normalizeWebhookDeliveryListLimit(limit)
	if err != nil {
		return ListWebhookDeliveriesOutput{}, err
	}

	deliveries, err := s.repository.ListWebhookDeliveries(ctx, normalizedStatus, normalizedLimit)
	if err != nil {
		return ListWebhookDeliveriesOutput{}, err
	}

	items := make([]WebhookDeliveryOutput, 0, len(deliveries))
	for _, delivery := range deliveries {
		items = append(items, webhookDeliveryOutputFromModel(delivery))
	}

	return ListWebhookDeliveriesOutput{Items: items}, nil
}

// RetryWebhookDelivery re-sends a failed final-decision webhook delivery.
func (s *Service) RetryWebhookDelivery(
	ctx context.Context,
	operatorID uint,
	deliveryID string,
) (WebhookDeliveryOutput, error) {
	if operatorID == 0 {
		return WebhookDeliveryOutput{}, apperrors.Unauthorized("User not authenticated")
	}
	if s.repository == nil {
		return WebhookDeliveryOutput{}, apperrors.ConfigurationError("moderation repository is not configured")
	}
	if s.webhookDispatcher == nil {
		return WebhookDeliveryOutput{}, apperrors.ConfigurationError("webhook dispatcher is not configured")
	}

	parsedDeliveryID, err := parseWebhookDeliveryID(deliveryID)
	if err != nil {
		return WebhookDeliveryOutput{}, err
	}

	delivery, err := s.repository.ClaimFailedWebhookDelivery(ctx, parsedDeliveryID, time.Now().UTC())
	if err != nil {
		return WebhookDeliveryOutput{}, err
	}
	statusCtx, cancelStatusCtx := context.WithTimeout(
		context.WithoutCancel(ctx),
		webhookRetryStatusUpdateTimeout,
	)
	defer cancelStatusCtx()

	client, found, err := s.repository.GetClient(ctx, delivery.ClientID)
	if err != nil {
		s.recordClaimedWebhookDeliveryFailure(statusCtx, delivery.ID, err.Error())
		return WebhookDeliveryOutput{}, err
	}
	if !found || strings.TrimSpace(client.WebhookURL) == "" {
		s.recordClaimedWebhookDeliveryFailure(statusCtx, delivery.ID, "Webhook client not found")
		return WebhookDeliveryOutput{}, apperrors.RecordNotFound("Webhook client not found")
	}

	var payload webhooks.FinalDecisionPayload
	if err := json.Unmarshal([]byte(delivery.Payload), &payload); err != nil {
		s.recordClaimedWebhookDeliveryFailure(statusCtx, delivery.ID, err.Error())
		return WebhookDeliveryOutput{}, apperrors.Internal("failed to decode webhook payload").WithDetails(err.Error())
	}
	payload.DeliveryID = delivery.DeliveryID

	status, httpStatus, errorMessage := deliverFinalDecision(ctx, s.webhookDispatcher, client, payload)
	updated, err := s.repository.UpdateWebhookDeliveryAttempt(
		statusCtx,
		delivery.ID,
		status,
		httpStatus,
		errorMessage,
		time.Now().UTC(),
	)
	if err != nil {
		return WebhookDeliveryOutput{}, err
	}

	return webhookDeliveryOutputFromModel(updated), nil
}

func (s *Service) recordClaimedWebhookDeliveryFailure(ctx context.Context, deliveryID uint, message string) {
	if s.repository == nil {
		return
	}
	if _, err := s.repository.UpdateWebhookDeliveryAttempt(
		ctx,
		deliveryID,
		WebhookDeliveryFailed,
		nil,
		message,
		time.Now().UTC(),
	); err != nil {
		zap.L().Warn(
			"failed to restore claimed webhook delivery after retry error",
			zap.Uint("delivery_id", deliveryID),
			zap.Error(err),
		)
	}
}

func (s *Service) dispatchFinalDecision(
	ctx context.Context,
	stored StoredResult,
	reviewStatus string,
	finalDecision string,
) {
	if s.webhookDispatcher == nil || stored.Request.ClientID == nil {
		return
	}

	attemptCtx, cancelAttemptCtx := context.WithTimeout(
		context.WithoutCancel(ctx),
		webhookDeliveryAttemptTimeout,
	)
	defer cancelAttemptCtx()

	client, found, err := s.repository.GetClient(attemptCtx, *stored.Request.ClientID)
	if err != nil {
		zap.L().Warn(
			"failed to load webhook client",
			zap.String("request_id", stored.Request.RequestID),
			zap.Error(err),
		)
		return
	}
	if !found || strings.TrimSpace(client.WebhookURL) == "" {
		return
	}

	labels, err := decodeLabels(stored.Result.Labels)
	if err != nil {
		zap.L().Warn(
			"failed to decode webhook labels",
			zap.String("request_id", stored.Request.RequestID),
			zap.Error(err),
		)
		return
	}
	if finalDecision == "" {
		finalDecision = stored.Result.Decision
	}

	payload := webhooks.FinalDecisionPayload{
		DeliveryID:    uuid.New().String(),
		Event:         "moderation.final_decision",
		RequestID:     stored.Request.RequestID,
		ClientID:      client.ID,
		ExternalID:    stored.Request.ExternalID,
		ActorID:       stored.Request.ActorID,
		Source:        stored.Request.Source,
		Decision:      finalDecision,
		ReviewStatus:  reviewStatus,
		RiskScore:     stored.Result.RiskScore,
		Labels:        labels,
		Reason:        stored.Result.Reason,
		PolicyVersion: stored.Result.PolicyVersion,
		CreatedAt:     stored.Result.CreatedAt,
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		zap.L().Warn(
			"failed to encode webhook delivery payload",
			zap.String("request_id", stored.Request.RequestID),
			zap.Error(err),
		)
		return
	}

	status, httpStatus, errorMessage := deliverFinalDecision(attemptCtx, s.webhookDispatcher, client, payload)
	delivery := &models.WebhookDelivery{
		DeliveryID:    payload.DeliveryID,
		RequestID:     stored.Request.RequestID,
		ClientID:      client.ID,
		Event:         payload.Event,
		Status:        string(status),
		AttemptCount:  1,
		LastAttemptAt: time.Now().UTC(),
		HTTPStatus:    httpStatus,
		ErrorMessage:  errorMessage,
		Payload:       string(payloadJSON),
	}
	recordCtx, cancelRecordCtx := context.WithTimeout(
		context.WithoutCancel(ctx),
		webhookDeliveryRecordTimeout,
	)
	defer cancelRecordCtx()
	if err := s.repository.SaveWebhookDelivery(recordCtx, delivery); err != nil {
		zap.L().Warn(
			"failed to record webhook delivery",
			zap.String("request_id", stored.Request.RequestID),
			zap.Uint("client_id", client.ID),
			zap.Error(err),
		)
	}
	if errorMessage != "" {
		zap.L().Warn(
			"failed to deliver moderation webhook",
			zap.String("request_id", stored.Request.RequestID),
			zap.Uint("client_id", client.ID),
			zap.String("error", errorMessage),
		)
	}
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

func normalizeReviewStatus(status string) (ReviewStatus, error) {
	status = strings.TrimSpace(status)
	if status == "" {
		return ReviewStatusPending, nil
	}

	reviewStatus := ReviewStatus(status)
	if !isSupportedReviewStatus(reviewStatus) {
		return "", apperrors.ValidationError("status must be pending, approved, rejected, or mistake")
	}

	return reviewStatus, nil
}

func isSupportedReviewStatus(status ReviewStatus) bool {
	switch status {
	case ReviewStatusPending, ReviewStatusApproved, ReviewStatusRejected, ReviewStatusMistake:
		return true
	default:
		return false
	}
}

func parseReviewCaseID(caseID string) (uint, error) {
	caseID = strings.TrimSpace(caseID)
	if caseID == "" {
		return 0, apperrors.ValidationError("review case id is required")
	}

	parsed, err := strconv.ParseUint(caseID, 10, 0)
	if err != nil || parsed == 0 {
		return 0, apperrors.ValidationError("review case id must be a positive integer")
	}

	return uint(parsed), nil
}

func parseWebhookDeliveryID(deliveryID string) (uint, error) {
	deliveryID = strings.TrimSpace(deliveryID)
	if deliveryID == "" {
		return 0, apperrors.ValidationError("webhook delivery id is required")
	}

	parsed, err := strconv.ParseUint(deliveryID, 10, 0)
	if err != nil || parsed == 0 {
		return 0, apperrors.ValidationError("webhook delivery id must be a positive integer")
	}

	return uint(parsed), nil
}

func normalizeWebhookDeliveryStatusFilter(status string) (WebhookDeliveryStatus, error) {
	status = strings.TrimSpace(status)
	if status == "" {
		return "", nil
	}

	switch WebhookDeliveryStatus(status) {
	case WebhookDeliverySucceeded, WebhookDeliveryFailed, WebhookDeliveryRetrying:
		return WebhookDeliveryStatus(status), nil
	default:
		return "", apperrors.ValidationError("status must be succeeded, failed, or retrying")
	}
}

func normalizeWebhookDeliveryListLimit(limit string) (int, error) {
	limit = strings.TrimSpace(limit)
	if limit == "" {
		return defaultWebhookDeliveryListLimit, nil
	}

	parsed, err := strconv.Atoi(limit)
	if err != nil || parsed <= 0 {
		return 0, apperrors.ValidationError("limit must be a positive integer")
	}
	if parsed > maxWebhookDeliveryListLimit {
		return 0, apperrors.ValidationError(
			fmt.Sprintf("limit must not exceed %d", maxWebhookDeliveryListLimit),
		)
	}

	return parsed, nil
}

func deliverFinalDecision(
	ctx context.Context,
	dispatcher webhooks.Dispatcher,
	client models.ClientApplication,
	payload webhooks.FinalDecisionPayload,
) (WebhookDeliveryStatus, *int, string) {
	if err := dispatcher.DispatchFinalDecision(ctx, client, payload); err != nil {
		var deliveryErr *webhooks.DeliveryError
		var httpStatus *int
		if errors.As(err, &deliveryErr) && deliveryErr.StatusCode != 0 {
			httpStatus = &deliveryErr.StatusCode
		}
		return WebhookDeliveryFailed, httpStatus, err.Error()
	}

	return WebhookDeliverySucceeded, nil, ""
}

func webhookDeliveryOutputFromModel(delivery models.WebhookDelivery) WebhookDeliveryOutput {
	return WebhookDeliveryOutput{
		ID:            delivery.ID,
		DeliveryID:    delivery.DeliveryID,
		RequestID:     delivery.RequestID,
		ClientID:      delivery.ClientID,
		Event:         delivery.Event,
		Status:        WebhookDeliveryStatus(delivery.Status),
		AttemptCount:  delivery.AttemptCount,
		LastAttemptAt: delivery.LastAttemptAt,
		HTTPStatus:    delivery.HTTPStatus,
		ErrorMessage:  delivery.ErrorMessage,
		CreatedAt:     delivery.CreatedAt,
		UpdatedAt:     delivery.UpdatedAt,
	}
}

func optionalUint(value uint) *uint {
	if value == 0 {
		return nil
	}

	return &value
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}

	return &value
}

func clientExternalIDIdempotencyKey(clientID uint, externalID string) string {
	if clientID == 0 || externalID == "" {
		return ""
	}

	return fmt.Sprintf("%d:%s", clientID, externalID)
}

func isConflictError(err error) bool {
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		return appErr.Code == apperrors.ErrCodeConflict ||
			appErr.Code == apperrors.ErrCodeDuplicateRecord
	}

	return false
}

func mapReviewCaseOutput(stored StoredReviewCase) (ReviewCaseOutput, error) {
	labels, err := decodeLabels(stored.Result.Labels)
	if err != nil {
		return ReviewCaseOutput{}, err
	}

	return ReviewCaseOutput{
		ID:             stored.Case.ID,
		RequestID:      stored.Case.RequestID,
		UserID:         stored.Case.UserID,
		ClientID:       stored.Case.ClientID,
		Content:        stored.Request.Content,
		Source:         stored.Request.Source,
		ExternalID:     stored.Request.ExternalID,
		ActorID:        stored.Request.ActorID,
		Status:         ReviewStatus(stored.Case.Status),
		PolicyDecision: Decision(stored.Result.Decision),
		FinalDecision:  Decision(stored.Case.FinalDecision),
		RiskScore:      stored.Result.RiskScore,
		Labels:         labels,
		Reason:         stored.Result.Reason,
		PolicyVersion:  stored.Result.PolicyVersion,
		ReviewerID:     stored.Case.ReviewerID,
		ReviewNotes:    stored.Case.ReviewNotes,
		ReviewedAt:     stored.Case.ReviewedAt,
		CreatedAt:      stored.Case.CreatedAt,
	}, nil
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
