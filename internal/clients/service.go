package clients

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"hatesentry/internal/auth"
	apperrors "hatesentry/internal/errors"
	"hatesentry/internal/models"
)

const (
	StatusActive   = "active"
	StatusInactive = "inactive"

	maxNameLength          = 100
	maxWebhookURLLength    = 500
	maxPolicyVersionLength = 50
)

// Repository persists external client records.
type Repository interface {
	CreateClient(ctx context.Context, client *models.ClientApplication) error
	ListClients(ctx context.Context) ([]models.ClientApplication, error)
}

// Service manages external application clients.
type Service struct {
	repository Repository
}

// NewService creates a client management service.
func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

// CreateInput is the admin request to create an external client.
type CreateInput struct {
	UserID        uint
	Name          string
	WebhookURL    string
	PolicyVersion string
}

// CreateOutput returns the created client and the one-time raw API key.
type CreateOutput struct {
	ID            uint      `json:"id"`
	Name          string    `json:"name"`
	Status        string    `json:"status"`
	APIKey        string    `json:"api_key"`
	APIKeyPrefix  string    `json:"api_key_prefix"`
	WebhookURL    string    `json:"webhook_url,omitempty"`
	PolicyVersion string    `json:"policy_version,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// ListOutput is the public admin representation of a client without secret material.
type ListOutput struct {
	ID            uint      `json:"id"`
	Name          string    `json:"name"`
	Status        string    `json:"status"`
	APIKeyPrefix  string    `json:"api_key_prefix"`
	WebhookURL    string    `json:"webhook_url,omitempty"`
	PolicyVersion string    `json:"policy_version,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CreateClient creates an active external client and returns its raw API key once.
func (s *Service) CreateClient(ctx context.Context, input CreateInput) (CreateOutput, error) {
	if s.repository == nil {
		return CreateOutput{}, apperrors.ConfigurationError("client repository is not configured")
	}

	normalized, err := validateCreateInput(input)
	if err != nil {
		return CreateOutput{}, err
	}

	apiKey, err := auth.GenerateAPIKey()
	if err != nil {
		return CreateOutput{}, err
	}

	client := &models.ClientApplication{
		UserID:        normalized.UserID,
		Name:          normalized.Name,
		APIKeyHash:    auth.HashAPIKey(apiKey),
		APIKeyPrefix:  auth.APIKeyPrefix(apiKey),
		Status:        StatusActive,
		WebhookURL:    normalized.WebhookURL,
		PolicyVersion: normalized.PolicyVersion,
	}
	if err := s.repository.CreateClient(ctx, client); err != nil {
		return CreateOutput{}, err
	}

	return CreateOutput{
		ID:            client.ID,
		Name:          client.Name,
		Status:        client.Status,
		APIKey:        apiKey,
		APIKeyPrefix:  client.APIKeyPrefix,
		WebhookURL:    client.WebhookURL,
		PolicyVersion: client.PolicyVersion,
		CreatedAt:     client.CreatedAt,
	}, nil
}

// ListClients lists external clients for admin/operator views.
func (s *Service) ListClients(ctx context.Context) ([]ListOutput, error) {
	if s.repository == nil {
		return nil, apperrors.ConfigurationError("client repository is not configured")
	}

	records, err := s.repository.ListClients(ctx)
	if err != nil {
		return nil, err
	}

	output := make([]ListOutput, 0, len(records))
	for _, client := range records {
		output = append(output, ListOutput{
			ID:            client.ID,
			Name:          client.Name,
			Status:        client.Status,
			APIKeyPrefix:  client.APIKeyPrefix,
			WebhookURL:    client.WebhookURL,
			PolicyVersion: client.PolicyVersion,
			CreatedAt:     client.CreatedAt,
			UpdatedAt:     client.UpdatedAt,
		})
	}

	return output, nil
}

func validateCreateInput(input CreateInput) (CreateInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.WebhookURL = strings.TrimSpace(input.WebhookURL)
	input.PolicyVersion = strings.TrimSpace(input.PolicyVersion)

	if input.UserID == 0 {
		return CreateInput{}, apperrors.Unauthorized("User not authenticated")
	}
	if input.Name == "" {
		return CreateInput{}, apperrors.ValidationError("name is required")
	}
	if len(input.Name) > maxNameLength {
		return CreateInput{}, apperrors.ValidationError(
			fmt.Sprintf("name must not exceed %d characters", maxNameLength),
		)
	}
	if len(input.WebhookURL) > maxWebhookURLLength {
		return CreateInput{}, apperrors.ValidationError(
			fmt.Sprintf("webhook_url must not exceed %d characters", maxWebhookURLLength),
		)
	}
	if input.WebhookURL != "" {
		parsed, err := url.ParseRequestURI(input.WebhookURL)
		if err != nil || parsed.Host == "" {
			return CreateInput{}, apperrors.ValidationError("webhook_url must be a valid absolute URL")
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return CreateInput{}, apperrors.ValidationError("webhook_url must use http or https")
		}
	}
	if len(input.PolicyVersion) > maxPolicyVersionLength {
		return CreateInput{}, apperrors.ValidationError(
			fmt.Sprintf("policy_version must not exceed %d characters", maxPolicyVersionLength),
		)
	}

	return input, nil
}
