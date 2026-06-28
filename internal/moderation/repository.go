package moderation

import (
	"context"

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
