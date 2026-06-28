package moderation

import (
	"context"
	stderrors "errors"
	"strings"
	"time"

	apperrors "hatesentry/internal/errors"
	"hatesentry/internal/models"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GormRepository stores moderation records through GORM.
type GormRepository struct {
	db *gorm.DB
}

// NewGormRepository creates a repository backed by GORM.
func NewGormRepository(db *gorm.DB) *GormRepository {
	return &GormRepository{db: db}
}

// SaveCheck persists a request/result pair atomically.
func (r *GormRepository) SaveCheck(
	ctx context.Context,
	request *models.ModerationRequest,
	result *models.ModerationResult,
	reviewCase *models.ReviewCase,
) error {
	if r == nil || r.db == nil {
		return apperrors.ConfigurationError("moderation database is not configured")
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(request).Error; err != nil {
			if isDuplicateKeyError(err) {
				return apperrors.Conflict("Moderation request already exists for this client external_id")
			}
			return err
		}
		if err := tx.Create(result).Error; err != nil {
			return err
		}
		if reviewCase != nil {
			if err := tx.Create(reviewCase).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		var appErr *apperrors.AppError
		if stderrors.As(err, &appErr) {
			return appErr
		}
		return apperrors.DatabaseError(err, "failed to save moderation records")
	}

	return nil
}

// GetResult retrieves a user-owned moderation request and result pair.
func (r *GormRepository) GetResult(ctx context.Context, userID uint, requestID string) (StoredResult, error) {
	if r == nil || r.db == nil {
		return StoredResult{}, apperrors.ConfigurationError("moderation database is not configured")
	}

	var request models.ModerationRequest
	if err := userScopedResultQuery(r.db.WithContext(ctx), userID, requestID).
		First(&request).Error; err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return StoredResult{}, apperrors.RecordNotFound("Moderation result not found")
		}
		return StoredResult{}, apperrors.DatabaseError(err, "failed to retrieve moderation request")
	}

	var result models.ModerationResult
	if err := userScopedResultQuery(r.db.WithContext(ctx), userID, requestID).
		First(&result).Error; err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return StoredResult{}, apperrors.RecordNotFound("Moderation result not found")
		}
		return StoredResult{}, apperrors.DatabaseError(err, "failed to retrieve moderation result")
	}

	return StoredResult{
		Request: request,
		Result:  result,
	}, nil
}

// FindResultByClientExternalID retrieves an existing client-owned result for idempotency.
func (r *GormRepository) FindResultByClientExternalID(
	ctx context.Context,
	clientID uint,
	externalID string,
) (StoredResult, bool, error) {
	if r == nil || r.db == nil {
		return StoredResult{}, false, apperrors.ConfigurationError("moderation database is not configured")
	}

	var request models.ModerationRequest
	err := clientExternalIDQuery(r.db.WithContext(ctx), clientID, externalID).
		Order("created_at DESC").
		First(&request).Error
	if err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return StoredResult{}, false, nil
		}
		return StoredResult{}, false, apperrors.DatabaseError(err, "failed to retrieve moderation request")
	}

	var result models.ModerationResult
	err = userScopedResultQuery(r.db.WithContext(ctx), request.UserID, request.RequestID).
		First(&result).Error
	if err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return StoredResult{}, false, nil
		}
		return StoredResult{}, false, apperrors.DatabaseError(err, "failed to retrieve moderation result")
	}

	return StoredResult{
		Request: request,
		Result:  result,
	}, true, nil
}

// ListHistory returns recent moderation records for operator audit views.
func (r *GormRepository) ListHistory(
	ctx context.Context,
	filter HistoryFilter,
) ([]StoredHistoryItem, error) {
	if r == nil || r.db == nil {
		return nil, apperrors.ConfigurationError("moderation database is not configured")
	}

	db := r.db.WithContext(ctx)
	query := moderationHistoryQuery(db, filter)

	var results []models.ModerationResult
	if err := query.Find(&results).Error; err != nil {
		return nil, apperrors.DatabaseError(err, "failed to list moderation history")
	}
	if len(results) == 0 {
		return []StoredHistoryItem{}, nil
	}

	requestIDs := make([]string, 0, len(results))
	for _, result := range results {
		requestIDs = append(requestIDs, result.RequestID)
	}

	requestsByID, err := loadModerationRequestsByID(ctx, db, requestIDs)
	if err != nil {
		return nil, err
	}
	reviewCasesByID, err := loadReviewCasesByRequestID(ctx, db, requestIDs)
	if err != nil {
		return nil, err
	}

	items := make([]StoredHistoryItem, 0, len(results))
	for _, result := range results {
		request, found := requestsByID[result.RequestID]
		if !found {
			return nil, apperrors.RecordNotFound("Moderation request not found")
		}

		var reviewCase *models.ReviewCase
		if foundReviewCase, ok := reviewCasesByID[result.RequestID]; ok {
			copiedReviewCase := foundReviewCase
			reviewCase = &copiedReviewCase
		}
		items = append(items, StoredHistoryItem{
			Request:    request,
			Result:     result,
			ReviewCase: reviewCase,
		})
	}

	return items, nil
}

// GetClient retrieves an active client application for webhook delivery.
func (r *GormRepository) GetClient(ctx context.Context, clientID uint) (models.ClientApplication, bool, error) {
	if r == nil || r.db == nil {
		return models.ClientApplication{}, false, apperrors.ConfigurationError("moderation database is not configured")
	}

	var client models.ClientApplication
	err := r.db.WithContext(ctx).
		Where("id = ? AND status = ?", clientID, "active").
		First(&client).Error
	if err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return models.ClientApplication{}, false, nil
		}
		return models.ClientApplication{}, false, apperrors.DatabaseError(err, "failed to retrieve webhook client")
	}

	return client, true, nil
}

// SaveWebhookDelivery records one webhook final-decision delivery attempt.
func (r *GormRepository) SaveWebhookDelivery(ctx context.Context, delivery *models.WebhookDelivery) error {
	if r == nil || r.db == nil {
		return apperrors.ConfigurationError("moderation database is not configured")
	}
	if err := r.db.WithContext(ctx).Create(delivery).Error; err != nil {
		return apperrors.DatabaseError(err, "failed to save webhook delivery")
	}

	return nil
}

// GetWebhookDelivery retrieves a persisted webhook delivery by database ID.
func (r *GormRepository) GetWebhookDelivery(ctx context.Context, deliveryID uint) (models.WebhookDelivery, error) {
	if r == nil || r.db == nil {
		return models.WebhookDelivery{}, apperrors.ConfigurationError("moderation database is not configured")
	}

	var delivery models.WebhookDelivery
	err := r.db.WithContext(ctx).
		Where("id = ?", deliveryID).
		First(&delivery).Error
	if err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return models.WebhookDelivery{}, apperrors.RecordNotFound("Webhook delivery not found")
		}
		return models.WebhookDelivery{}, apperrors.DatabaseError(err, "failed to retrieve webhook delivery")
	}

	return delivery, nil
}

// ListWebhookDeliveries returns recent webhook delivery records for operators.
func (r *GormRepository) ListWebhookDeliveries(
	ctx context.Context,
	status WebhookDeliveryStatus,
	limit int,
) ([]models.WebhookDelivery, error) {
	if r == nil || r.db == nil {
		return nil, apperrors.ConfigurationError("moderation database is not configured")
	}

	query := r.db.WithContext(ctx).Model(&models.WebhookDelivery{})
	if status != "" {
		query = query.Where("status = ?", string(status))
	}
	if limit > 0 {
		query = query.Limit(limit)
	}

	var deliveries []models.WebhookDelivery
	if err := query.Order("updated_at DESC").Find(&deliveries).Error; err != nil {
		return nil, apperrors.DatabaseError(err, "failed to list webhook deliveries")
	}

	return deliveries, nil
}

// ClaimFailedWebhookDelivery atomically reserves one failed delivery for retry.
func (r *GormRepository) ClaimFailedWebhookDelivery(
	ctx context.Context,
	deliveryID uint,
	attemptedAt time.Time,
) (models.WebhookDelivery, error) {
	if r == nil || r.db == nil {
		return models.WebhookDelivery{}, apperrors.ConfigurationError("moderation database is not configured")
	}

	staleRetryingBefore := attemptedAt.Add(-webhookRetryLease)
	result := r.db.WithContext(ctx).
		Model(&models.WebhookDelivery{}).
		Where(
			"id = ? AND (status = ? OR (status = ? AND last_attempt_at < ?))",
			deliveryID,
			string(WebhookDeliveryFailed),
			string(WebhookDeliveryRetrying),
			staleRetryingBefore,
		).
		Updates(map[string]interface{}{
			"status":          string(WebhookDeliveryRetrying),
			"last_attempt_at": attemptedAt,
			"http_status":     nil,
			"error_message":   "",
		})
	if result.Error != nil {
		return models.WebhookDelivery{}, apperrors.DatabaseError(result.Error, "failed to claim webhook delivery")
	}
	if result.RowsAffected != 1 {
		var delivery models.WebhookDelivery
		err := r.db.WithContext(ctx).
			Where("id = ?", deliveryID).
			First(&delivery).Error
		if err != nil {
			if stderrors.Is(err, gorm.ErrRecordNotFound) {
				return models.WebhookDelivery{}, apperrors.RecordNotFound("Webhook delivery not found")
			}
			return models.WebhookDelivery{}, apperrors.DatabaseError(err, "failed to retrieve webhook delivery")
		}
		return models.WebhookDelivery{}, apperrors.Conflict("Webhook delivery is not failed")
	}

	return r.GetWebhookDelivery(ctx, deliveryID)
}

// UpdateWebhookDeliveryAttempt stores the latest retry outcome and increments attempts.
func (r *GormRepository) UpdateWebhookDeliveryAttempt(
	ctx context.Context,
	deliveryID uint,
	status WebhookDeliveryStatus,
	httpStatus *int,
	errorMessage string,
	attemptedAt time.Time,
) (models.WebhookDelivery, error) {
	if r == nil || r.db == nil {
		return models.WebhookDelivery{}, apperrors.ConfigurationError("moderation database is not configured")
	}

	var updated models.WebhookDelivery
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var delivery models.WebhookDelivery
		err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", deliveryID).
			First(&delivery).Error
		if err != nil {
			if stderrors.Is(err, gorm.ErrRecordNotFound) {
				return apperrors.RecordNotFound("Webhook delivery not found")
			}
			return err
		}

		delivery.Status = string(status)
		delivery.AttemptCount++
		delivery.LastAttemptAt = attemptedAt
		delivery.HTTPStatus = httpStatus
		delivery.ErrorMessage = errorMessage

		if err := tx.Save(&delivery).Error; err != nil {
			return err
		}
		updated = delivery
		return nil
	})
	if err != nil {
		if _, ok := err.(*apperrors.AppError); ok {
			return models.WebhookDelivery{}, err
		}
		return models.WebhookDelivery{}, apperrors.DatabaseError(err, "failed to update webhook delivery")
	}

	return updated, nil
}

// GetStats returns aggregate moderation and review workflow counts.
func (r *GormRepository) GetStats(ctx context.Context) (StoredStats, error) {
	if r == nil || r.db == nil {
		return StoredStats{}, apperrors.ConfigurationError("moderation database is not configured")
	}

	db := r.db.WithContext(ctx)
	var stats StoredStats
	counts := []statsCountQuery{
		{
			name:  "total moderated",
			query: totalModeratedStatsQuery(db),
			dest:  &stats.TotalModerated,
		},
		{
			name:  "policy allowed",
			query: policyDecisionStatsQuery(db, DecisionAllow),
			dest:  &stats.PolicyAllowed,
		},
		{
			name:  "policy blocked",
			query: policyDecisionStatsQuery(db, DecisionBlock),
			dest:  &stats.PolicyBlocked,
		},
		{
			name:  "review final allowed",
			query: reviewFinalDecisionStatsQuery(db, DecisionAllow),
			dest:  &stats.ReviewFinalAllowed,
		},
		{
			name:  "review final blocked",
			query: reviewFinalDecisionStatsQuery(db, DecisionBlock),
			dest:  &stats.ReviewFinalBlocked,
		},
		{
			name:  "pending review",
			query: reviewStatusStatsQuery(db, ReviewStatusPending),
			dest:  &stats.PendingReview,
		},
		{
			name:  "reviewed",
			query: reviewedStatsQuery(db),
			dest:  &stats.Reviewed,
		},
		{
			name:  "mistakes",
			query: reviewStatusStatsQuery(db, ReviewStatusMistake),
			dest:  &stats.Mistakes,
		},
	}

	for _, count := range counts {
		if err := count.query.Count(count.dest).Error; err != nil {
			return StoredStats{}, apperrors.DatabaseError(err, "failed to count "+count.name)
		}
	}

	return stats, nil
}

type statsCountQuery struct {
	name  string
	query *gorm.DB
	dest  *int64
}

// ListReviewCases retrieves review cases by workflow status.
func (r *GormRepository) ListReviewCases(
	ctx context.Context,
	status ReviewStatus,
) ([]StoredReviewCase, error) {
	if r == nil || r.db == nil {
		return nil, apperrors.ConfigurationError("moderation database is not configured")
	}

	var cases []models.ReviewCase
	if err := reviewCaseListQuery(r.db.WithContext(ctx), status).
		Find(&cases).Error; err != nil {
		return nil, apperrors.DatabaseError(err, "failed to list review cases")
	}

	storedCases := make([]StoredReviewCase, 0, len(cases))
	for _, reviewCase := range cases {
		stored, err := r.loadStoredReviewCase(ctx, reviewCase)
		if err != nil {
			return nil, err
		}
		storedCases = append(storedCases, stored)
	}

	return storedCases, nil
}

// GetReviewCase retrieves one review case with its moderation request/result details.
func (r *GormRepository) GetReviewCase(ctx context.Context, caseID uint) (StoredReviewCase, error) {
	if r == nil || r.db == nil {
		return StoredReviewCase{}, apperrors.ConfigurationError("moderation database is not configured")
	}

	var reviewCase models.ReviewCase
	err := reviewCaseByIDQuery(r.db.WithContext(ctx), caseID).First(&reviewCase).Error
	if err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return StoredReviewCase{}, apperrors.RecordNotFound("Review case not found")
		}
		return StoredReviewCase{}, apperrors.DatabaseError(err, "failed to retrieve review case")
	}

	return r.loadStoredReviewCase(ctx, reviewCase)
}

// FinalizeReviewCase records the human final decision for a pending review case.
func (r *GormRepository) FinalizeReviewCase(
	ctx context.Context,
	caseID uint,
	reviewerID uint,
	status ReviewStatus,
	finalDecision Decision,
	notes string,
	reviewedAt time.Time,
) (StoredReviewCase, error) {
	if r == nil || r.db == nil {
		return StoredReviewCase{}, apperrors.ConfigurationError("moderation database is not configured")
	}

	var stored StoredReviewCase
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var reviewCase models.ReviewCase
		err := reviewCaseByIDQuery(tx, caseID).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&reviewCase).Error
		if err != nil {
			if stderrors.Is(err, gorm.ErrRecordNotFound) {
				return apperrors.RecordNotFound("Review case not found")
			}
			return err
		}
		if reviewCase.Status != string(ReviewStatusPending) {
			return apperrors.Conflict("Review case is already finalized")
		}

		reviewCase.Status = string(status)
		reviewCase.ReviewerID = &reviewerID
		reviewCase.FinalDecision = string(finalDecision)
		reviewCase.ReviewNotes = notes
		reviewCase.ReviewedAt = &reviewedAt

		if err := tx.Save(&reviewCase).Error; err != nil {
			return err
		}

		loaded, err := loadStoredReviewCase(ctx, tx, reviewCase)
		if err != nil {
			return err
		}
		stored = loaded
		return nil
	})
	if err != nil {
		if _, ok := err.(*apperrors.AppError); ok {
			return StoredReviewCase{}, err
		}
		return StoredReviewCase{}, apperrors.DatabaseError(err, "failed to finalize review case")
	}

	return stored, nil
}

func userScopedResultQuery(db *gorm.DB, userID uint, requestID string) *gorm.DB {
	return db.Where("user_id = ? AND request_id = ?", userID, requestID)
}

func clientExternalIDQuery(db *gorm.DB, clientID uint, externalID string) *gorm.DB {
	return db.Where("client_id = ? AND external_id = ?", clientID, externalID)
}

func moderationHistoryQuery(db *gorm.DB, filter HistoryFilter) *gorm.DB {
	query := db.Model(&models.ModerationResult{})
	if filter.Decision != "" {
		query = query.Where("moderation_results.decision = ?", string(filter.Decision))
	}
	if filter.ClientID != nil {
		query = query.Where("moderation_results.client_id = ?", *filter.ClientID)
	}
	if filter.ExternalID != "" {
		query = query.Joins(
			"JOIN moderation_requests ON moderation_requests.request_id = moderation_results.request_id AND moderation_requests.deleted_at IS NULL",
		).Where("moderation_requests.external_id = ?", filter.ExternalID)
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}

	return query.Order("moderation_results.created_at DESC")
}

func loadModerationRequestsByID(
	ctx context.Context,
	db *gorm.DB,
	requestIDs []string,
) (map[string]models.ModerationRequest, error) {
	var requests []models.ModerationRequest
	if err := db.WithContext(ctx).
		Where("request_id IN ?", requestIDs).
		Find(&requests).Error; err != nil {
		return nil, apperrors.DatabaseError(err, "failed to load moderation requests")
	}

	requestsByID := make(map[string]models.ModerationRequest, len(requests))
	for _, request := range requests {
		requestsByID[request.RequestID] = request
	}

	return requestsByID, nil
}

func loadReviewCasesByRequestID(
	ctx context.Context,
	db *gorm.DB,
	requestIDs []string,
) (map[string]models.ReviewCase, error) {
	var reviewCases []models.ReviewCase
	if err := db.WithContext(ctx).
		Where("request_id IN ?", requestIDs).
		Find(&reviewCases).Error; err != nil {
		return nil, apperrors.DatabaseError(err, "failed to load review cases")
	}

	reviewCasesByID := make(map[string]models.ReviewCase, len(reviewCases))
	for _, reviewCase := range reviewCases {
		reviewCasesByID[reviewCase.RequestID] = reviewCase
	}

	return reviewCasesByID, nil
}

func isDuplicateKeyError(err error) bool {
	var mysqlErr *mysql.MySQLError
	if stderrors.As(err, &mysqlErr) {
		return mysqlErr.Number == 1062
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate entry") ||
		strings.Contains(message, "unique constraint") ||
		strings.Contains(message, "duplicate key")
}

func reviewCaseListQuery(db *gorm.DB, status ReviewStatus) *gorm.DB {
	return db.
		Where("status = ?", string(status)).
		Order("created_at ASC")
}

func reviewCaseByIDQuery(db *gorm.DB, caseID uint) *gorm.DB {
	return db.Where("id = ?", caseID)
}

func totalModeratedStatsQuery(db *gorm.DB) *gorm.DB {
	return db.Model(&models.ModerationResult{})
}

func policyDecisionStatsQuery(db *gorm.DB, decision Decision) *gorm.DB {
	return db.Model(&models.ModerationResult{}).Where("decision = ?", string(decision))
}

func reviewFinalDecisionStatsQuery(db *gorm.DB, decision Decision) *gorm.DB {
	return db.Model(&models.ReviewCase{}).
		Where("status <> ? AND final_decision = ?", string(ReviewStatusPending), string(decision))
}

func reviewStatusStatsQuery(db *gorm.DB, status ReviewStatus) *gorm.DB {
	return db.Model(&models.ReviewCase{}).Where("status = ?", string(status))
}

func reviewedStatsQuery(db *gorm.DB) *gorm.DB {
	return db.Model(&models.ReviewCase{}).Where("status <> ?", string(ReviewStatusPending))
}

func (r *GormRepository) loadStoredReviewCase(
	ctx context.Context,
	reviewCase models.ReviewCase,
) (StoredReviewCase, error) {
	return loadStoredReviewCase(ctx, r.db.WithContext(ctx), reviewCase)
}

func loadStoredReviewCase(ctx context.Context, db *gorm.DB, reviewCase models.ReviewCase) (StoredReviewCase, error) {
	var request models.ModerationRequest
	if err := userScopedResultQuery(db.WithContext(ctx), reviewCase.UserID, reviewCase.RequestID).
		First(&request).Error; err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return StoredReviewCase{}, apperrors.RecordNotFound("Moderation request not found")
		}
		return StoredReviewCase{}, apperrors.DatabaseError(err, "failed to retrieve moderation request")
	}

	var result models.ModerationResult
	if err := userScopedResultQuery(db.WithContext(ctx), reviewCase.UserID, reviewCase.RequestID).
		First(&result).Error; err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return StoredReviewCase{}, apperrors.RecordNotFound("Moderation result not found")
		}
		return StoredReviewCase{}, apperrors.DatabaseError(err, "failed to retrieve moderation result")
	}

	return StoredReviewCase{
		Case:    reviewCase,
		Request: request,
		Result:  result,
	}, nil
}
