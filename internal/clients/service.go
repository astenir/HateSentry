package clients

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"hatesentry/internal/auth"
	apperrors "hatesentry/internal/errors"
	"hatesentry/internal/models"
	"hatesentry/internal/webhooks"
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
	UpdateClientStatus(ctx context.Context, clientID uint, status string) (models.ClientApplication, error)
	UpdateClientPolicyVersion(ctx context.Context, clientID uint, policyVersion string) (models.ClientApplication, error)
	UpdateClientWebhook(ctx context.Context, clientID uint, webhookURL string, webhookSecret string) (models.ClientApplication, error)
	RotateClientAPIKey(ctx context.Context, clientID uint, apiKeyHash string, apiKeyPrefix string) (models.ClientApplication, error)
}

// PolicyVersionValidator verifies configured moderation policy assignments.
type PolicyVersionValidator interface {
	ValidatePolicyVersion(version string) error
}

// Service manages external application clients.
type Service struct {
	repository      Repository
	policyValidator PolicyVersionValidator
}

// NewService creates a client management service.
func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

// NewServiceWithPolicyValidator creates a client service with policy assignment validation.
func NewServiceWithPolicyValidator(
	repository Repository,
	policyValidator PolicyVersionValidator,
) *Service {
	return &Service{
		repository:      repository,
		policyValidator: policyValidator,
	}
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
	WebhookSecret string    `json:"webhook_secret,omitempty"`
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

// RotateAPIKeyOutput returns the updated client and the one-time raw API key.
type RotateAPIKeyOutput struct {
	ID            uint      `json:"id"`
	Name          string    `json:"name"`
	Status        string    `json:"status"`
	APIKey        string    `json:"api_key"`
	APIKeyPrefix  string    `json:"api_key_prefix"`
	WebhookURL    string    `json:"webhook_url,omitempty"`
	PolicyVersion string    `json:"policy_version,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// UpdateWebhookOutput returns the updated client and a one-time webhook secret when configured.
type UpdateWebhookOutput struct {
	ID            uint      `json:"id"`
	Name          string    `json:"name"`
	Status        string    `json:"status"`
	APIKeyPrefix  string    `json:"api_key_prefix"`
	WebhookSecret string    `json:"webhook_secret,omitempty"`
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
	if s.policyValidator != nil && normalized.PolicyVersion != "" {
		if err := s.policyValidator.ValidatePolicyVersion(normalized.PolicyVersion); err != nil {
			return CreateOutput{}, apperrors.ValidationError("invalid policy_version").WithDetails(err.Error())
		}
	}

	apiKey, err := auth.GenerateAPIKey()
	if err != nil {
		return CreateOutput{}, err
	}

	webhookSecret := ""
	if normalized.WebhookURL != "" {
		webhookSecret, err = webhooks.GenerateSecret()
		if err != nil {
			return CreateOutput{}, err
		}
	}

	client := &models.ClientApplication{
		UserID:        normalized.UserID,
		Name:          normalized.Name,
		APIKeyHash:    auth.HashAPIKey(apiKey),
		APIKeyPrefix:  auth.APIKeyPrefix(apiKey),
		Status:        StatusActive,
		WebhookURL:    normalized.WebhookURL,
		WebhookSecret: webhookSecret,
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
		WebhookSecret: webhookSecret,
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
		output = append(output, clientListOutput(client))
	}

	return output, nil
}

// ActivateClient allows an external client to authenticate with its existing API key.
func (s *Service) ActivateClient(ctx context.Context, operatorID uint, clientID string) (ListOutput, error) {
	return s.updateClientStatus(ctx, operatorID, clientID, StatusActive)
}

// DeactivateClient revokes an external client's API key access without deleting audit data.
func (s *Service) DeactivateClient(ctx context.Context, operatorID uint, clientID string) (ListOutput, error) {
	return s.updateClientStatus(ctx, operatorID, clientID, StatusInactive)
}

// RotateClientAPIKey replaces a client's API key while preserving its status and settings.
func (s *Service) RotateClientAPIKey(
	ctx context.Context,
	operatorID uint,
	clientID string,
) (RotateAPIKeyOutput, error) {
	if operatorID == 0 {
		return RotateAPIKeyOutput{}, apperrors.Unauthorized("User not authenticated")
	}
	if s.repository == nil {
		return RotateAPIKeyOutput{}, apperrors.ConfigurationError("client repository is not configured")
	}

	parsedClientID, err := parseClientID(clientID)
	if err != nil {
		return RotateAPIKeyOutput{}, err
	}

	apiKey, err := auth.GenerateAPIKey()
	if err != nil {
		return RotateAPIKeyOutput{}, err
	}
	apiKeyPrefix := auth.APIKeyPrefix(apiKey)

	client, err := s.repository.RotateClientAPIKey(
		ctx,
		parsedClientID,
		auth.HashAPIKey(apiKey),
		apiKeyPrefix,
	)
	if err != nil {
		return RotateAPIKeyOutput{}, err
	}

	return RotateAPIKeyOutput{
		ID:            client.ID,
		Name:          client.Name,
		Status:        client.Status,
		APIKey:        apiKey,
		APIKeyPrefix:  apiKeyPrefix,
		WebhookURL:    client.WebhookURL,
		PolicyVersion: client.PolicyVersion,
		UpdatedAt:     client.UpdatedAt,
	}, nil
}

// UpdateClientPolicyVersion changes the policy assigned to future moderation checks from a client.
func (s *Service) UpdateClientPolicyVersion(
	ctx context.Context,
	operatorID uint,
	clientID string,
	policyVersion string,
) (ListOutput, error) {
	if operatorID == 0 {
		return ListOutput{}, apperrors.Unauthorized("User not authenticated")
	}
	if s.repository == nil {
		return ListOutput{}, apperrors.ConfigurationError("client repository is not configured")
	}

	parsedClientID, err := parseClientID(clientID)
	if err != nil {
		return ListOutput{}, err
	}
	normalizedPolicyVersion, err := validatePolicyVersionInput(policyVersion)
	if err != nil {
		return ListOutput{}, err
	}
	if s.policyValidator != nil {
		if err := s.policyValidator.ValidatePolicyVersion(normalizedPolicyVersion); err != nil {
			return ListOutput{}, apperrors.ValidationError("invalid policy_version").WithDetails(err.Error())
		}
	}

	client, err := s.repository.UpdateClientPolicyVersion(ctx, parsedClientID, normalizedPolicyVersion)
	if err != nil {
		return ListOutput{}, err
	}

	return clientListOutput(client), nil
}

// UpdateClientWebhook changes the callback URL and rotates the signing secret for future final decisions.
func (s *Service) UpdateClientWebhook(
	ctx context.Context,
	operatorID uint,
	clientID string,
	webhookURL string,
) (UpdateWebhookOutput, error) {
	if operatorID == 0 {
		return UpdateWebhookOutput{}, apperrors.Unauthorized("User not authenticated")
	}
	if s.repository == nil {
		return UpdateWebhookOutput{}, apperrors.ConfigurationError("client repository is not configured")
	}

	parsedClientID, err := parseClientID(clientID)
	if err != nil {
		return UpdateWebhookOutput{}, err
	}
	normalizedWebhookURL, err := validateWebhookURLInput(webhookURL)
	if err != nil {
		return UpdateWebhookOutput{}, err
	}

	webhookSecret := ""
	if normalizedWebhookURL != "" {
		webhookSecret, err = webhooks.GenerateSecret()
		if err != nil {
			return UpdateWebhookOutput{}, err
		}
	}

	client, err := s.repository.UpdateClientWebhook(
		ctx,
		parsedClientID,
		normalizedWebhookURL,
		webhookSecret,
	)
	if err != nil {
		return UpdateWebhookOutput{}, err
	}

	returnedWebhookSecret := ""
	if normalizedWebhookURL != "" {
		returnedWebhookSecret = client.WebhookSecret
	}

	return clientWebhookOutput(client, returnedWebhookSecret), nil
}

func (s *Service) updateClientStatus(
	ctx context.Context,
	operatorID uint,
	clientID string,
	status string,
) (ListOutput, error) {
	if operatorID == 0 {
		return ListOutput{}, apperrors.Unauthorized("User not authenticated")
	}
	if s.repository == nil {
		return ListOutput{}, apperrors.ConfigurationError("client repository is not configured")
	}

	parsedClientID, err := parseClientID(clientID)
	if err != nil {
		return ListOutput{}, err
	}
	if err := validateClientStatus(status); err != nil {
		return ListOutput{}, err
	}

	client, err := s.repository.UpdateClientStatus(ctx, parsedClientID, status)
	if err != nil {
		return ListOutput{}, err
	}

	return clientListOutput(client), nil
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
		if err := webhooks.ValidateURL(input.WebhookURL); err != nil {
			return CreateInput{}, err
		}
	}
	if len(input.PolicyVersion) > maxPolicyVersionLength {
		return CreateInput{}, apperrors.ValidationError(
			fmt.Sprintf("policy_version must not exceed %d characters", maxPolicyVersionLength),
		)
	}

	return input, nil
}

func validatePolicyVersionInput(policyVersion string) (string, error) {
	policyVersion = strings.TrimSpace(policyVersion)
	if len(policyVersion) > maxPolicyVersionLength {
		return "", apperrors.ValidationError(
			fmt.Sprintf("policy_version must not exceed %d characters", maxPolicyVersionLength),
		)
	}

	return policyVersion, nil
}

func validateWebhookURLInput(webhookURL string) (string, error) {
	webhookURL = strings.TrimSpace(webhookURL)
	if len(webhookURL) > maxWebhookURLLength {
		return "", apperrors.ValidationError(
			fmt.Sprintf("webhook_url must not exceed %d characters", maxWebhookURLLength),
		)
	}
	if webhookURL != "" {
		if err := webhooks.ValidateURL(webhookURL); err != nil {
			return "", err
		}
	}

	return webhookURL, nil
}

func clientListOutput(client models.ClientApplication) ListOutput {
	return ListOutput{
		ID:            client.ID,
		Name:          client.Name,
		Status:        client.Status,
		APIKeyPrefix:  client.APIKeyPrefix,
		WebhookURL:    client.WebhookURL,
		PolicyVersion: client.PolicyVersion,
		CreatedAt:     client.CreatedAt,
		UpdatedAt:     client.UpdatedAt,
	}
}

func clientWebhookOutput(client models.ClientApplication, webhookSecret string) UpdateWebhookOutput {
	return UpdateWebhookOutput{
		ID:            client.ID,
		Name:          client.Name,
		Status:        client.Status,
		APIKeyPrefix:  client.APIKeyPrefix,
		WebhookSecret: webhookSecret,
		WebhookURL:    client.WebhookURL,
		PolicyVersion: client.PolicyVersion,
		CreatedAt:     client.CreatedAt,
		UpdatedAt:     client.UpdatedAt,
	}
}

func parseClientID(clientID string) (uint, error) {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return 0, apperrors.ValidationError("client id is required")
	}

	parsed, err := strconv.ParseUint(clientID, 10, 0)
	if err != nil || parsed == 0 {
		return 0, apperrors.ValidationError("client id must be a positive integer")
	}

	return uint(parsed), nil
}

func validateClientStatus(status string) error {
	switch status {
	case StatusActive, StatusInactive:
		return nil
	default:
		return apperrors.ValidationError("client status must be active or inactive")
	}
}
