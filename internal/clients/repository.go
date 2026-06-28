package clients

import (
	"context"
	stderrors "errors"

	"hatesentry/internal/auth"
	apperrors "hatesentry/internal/errors"
	"hatesentry/internal/models"

	"gorm.io/gorm"
)

// GormRepository stores client application records through GORM.
type GormRepository struct {
	db *gorm.DB
}

// NewGormRepository creates a client repository backed by GORM.
func NewGormRepository(db *gorm.DB) *GormRepository {
	return &GormRepository{db: db}
}

// CreateClient stores a new external client.
func (r *GormRepository) CreateClient(ctx context.Context, client *models.ClientApplication) error {
	if r == nil || r.db == nil {
		return apperrors.ConfigurationError("client database is not configured")
	}

	if err := r.db.WithContext(ctx).Create(client).Error; err != nil {
		return apperrors.DatabaseError(err, "failed to create client")
	}

	return nil
}

// ListClients returns client records ordered by most recently created first.
func (r *GormRepository) ListClients(ctx context.Context) ([]models.ClientApplication, error) {
	if r == nil || r.db == nil {
		return nil, apperrors.ConfigurationError("client database is not configured")
	}

	var clients []models.ClientApplication
	if err := r.db.WithContext(ctx).
		Order("created_at DESC").
		Find(&clients).Error; err != nil {
		return nil, apperrors.DatabaseError(err, "failed to list clients")
	}

	return clients, nil
}

// AuthenticateAPIKey validates an external client API key.
func (r *GormRepository) AuthenticateAPIKey(ctx context.Context, key string) (auth.APIKeyPrincipal, error) {
	if r == nil || r.db == nil {
		return auth.APIKeyPrincipal{}, apperrors.ConfigurationError("client database is not configured")
	}

	keyHash := auth.HashAPIKey(key)
	var client models.ClientApplication
	err := r.db.WithContext(ctx).
		Where("api_key_hash = ? AND status = ?", keyHash, StatusActive).
		First(&client).Error
	if err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return auth.APIKeyPrincipal{}, apperrors.Unauthorized("Invalid API key")
		}
		return auth.APIKeyPrincipal{}, apperrors.DatabaseError(err, "failed to authenticate api key")
	}

	return auth.APIKeyPrincipal{
		ClientID: client.ID,
		UserID:   client.UserID,
		Name:     client.Name,
	}, nil
}
