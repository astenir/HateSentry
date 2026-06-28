package clients

import (
	"context"
	stderrors "errors"

	"hatesentry/internal/auth"
	apperrors "hatesentry/internal/errors"
	"hatesentry/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

// UpdateClientStatus changes a client application's authentication status.
func (r *GormRepository) UpdateClientStatus(
	ctx context.Context,
	clientID uint,
	status string,
) (models.ClientApplication, error) {
	if r == nil || r.db == nil {
		return models.ClientApplication{}, apperrors.ConfigurationError("client database is not configured")
	}

	result := r.db.WithContext(ctx).
		Model(&models.ClientApplication{}).
		Where("id = ?", clientID).
		Update("status", status)
	if result.Error != nil {
		return models.ClientApplication{}, apperrors.DatabaseError(result.Error, "failed to update client status")
	}

	var client models.ClientApplication
	if err := r.db.WithContext(ctx).First(&client, clientID).Error; err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return models.ClientApplication{}, apperrors.RecordNotFound("Client not found")
		}
		return models.ClientApplication{}, apperrors.DatabaseError(err, "failed to retrieve client")
	}

	return client, nil
}

// UpdateClientPolicyVersion changes a client's assigned moderation policy version.
func (r *GormRepository) UpdateClientPolicyVersion(
	ctx context.Context,
	clientID uint,
	policyVersion string,
) (models.ClientApplication, error) {
	if r == nil || r.db == nil {
		return models.ClientApplication{}, apperrors.ConfigurationError("client database is not configured")
	}

	result := r.db.WithContext(ctx).
		Model(&models.ClientApplication{}).
		Where("id = ?", clientID).
		Update("policy_version", policyVersion)
	if result.Error != nil {
		return models.ClientApplication{}, apperrors.DatabaseError(result.Error, "failed to update client policy version")
	}

	var client models.ClientApplication
	if err := r.db.WithContext(ctx).First(&client, clientID).Error; err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return models.ClientApplication{}, apperrors.RecordNotFound("Client not found")
		}
		return models.ClientApplication{}, apperrors.DatabaseError(err, "failed to retrieve client")
	}

	return client, nil
}

// RotateClientAPIKey replaces the stored API key hash and visible prefix.
func (r *GormRepository) RotateClientAPIKey(
	ctx context.Context,
	clientID uint,
	apiKeyHash string,
	apiKeyPrefix string,
) (models.ClientApplication, error) {
	if r == nil || r.db == nil {
		return models.ClientApplication{}, apperrors.ConfigurationError("client database is not configured")
	}

	var client models.ClientApplication
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&client, clientID).Error; err != nil {
			if stderrors.Is(err, gorm.ErrRecordNotFound) {
				return apperrors.RecordNotFound("Client not found")
			}
			return apperrors.DatabaseError(err, "failed to retrieve client")
		}

		result := tx.Model(&models.ClientApplication{}).
			Where("id = ?", clientID).
			Updates(map[string]any{
				"api_key_hash":   apiKeyHash,
				"api_key_prefix": apiKeyPrefix,
			})
		if result.Error != nil {
			return apperrors.DatabaseError(result.Error, "failed to rotate client api key")
		}

		if err := tx.First(&client, clientID).Error; err != nil {
			return apperrors.DatabaseError(err, "failed to retrieve client")
		}

		return nil
	})
	if err != nil {
		return models.ClientApplication{}, err
	}

	return client, nil
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
