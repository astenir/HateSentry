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
