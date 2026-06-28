package moderation

import (
	"context"
	stderrors "errors"

	apperrors "hatesentry/internal/errors"
	"hatesentry/internal/models"

	"gorm.io/gorm"
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
) error {
	if r == nil || r.db == nil {
		return apperrors.ConfigurationError("moderation database is not configured")
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(request).Error; err != nil {
			return err
		}
		if err := tx.Create(result).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
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

func userScopedResultQuery(db *gorm.DB, userID uint, requestID string) *gorm.DB {
	return db.Where("user_id = ? AND request_id = ?", userID, requestID)
}
