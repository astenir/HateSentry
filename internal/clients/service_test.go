package clients

import (
	"context"
	"strings"
	"testing"
	"time"

	"hatesentry/internal/auth"
	"hatesentry/internal/models"
)

func TestServiceCreateClientStoresHashedAPIKey(t *testing.T) {
	repository := &fakeRepository{}
	service := NewServiceWithPolicyValidator(repository, &fakePolicyValidator{
		allowed: map[string]bool{"default-v1": true},
	})

	output, err := service.CreateClient(context.Background(), CreateInput{
		UserID:        42,
		Name:          " Blog comments ",
		WebhookURL:    "https://example.com/moderation",
		PolicyVersion: "default-v1",
	})
	if err != nil {
		t.Fatalf("CreateClient() error = %v", err)
	}

	if output.APIKey == "" {
		t.Fatal("APIKey is empty")
	}
	if output.WebhookSecret == "" {
		t.Fatal("WebhookSecret is empty when webhook_url is configured")
	}
	if !strings.HasPrefix(output.WebhookSecret, "whsec_") {
		t.Fatalf("WebhookSecret = %q, want whsec_ prefix", output.WebhookSecret)
	}
	if !strings.HasPrefix(output.APIKey, "hs_live_") {
		t.Fatalf("APIKey = %q, want hs_live_ prefix", output.APIKey)
	}
	if output.Name != "Blog comments" {
		t.Fatalf("Name = %q, want trimmed name", output.Name)
	}
	if repository.client == nil {
		t.Fatal("client was not persisted")
	}
	if repository.client.UserID != 42 {
		t.Fatalf("UserID = %d, want 42", repository.client.UserID)
	}
	if repository.client.APIKeyHash == "" {
		t.Fatal("APIKeyHash was not persisted")
	}
	if strings.Contains(repository.client.APIKeyHash, output.APIKey) {
		t.Fatal("persisted APIKeyHash contains raw key")
	}
	if repository.client.APIKeyHash != auth.HashAPIKey(output.APIKey) {
		t.Fatal("persisted APIKeyHash does not match returned key")
	}
	if repository.client.APIKeyPrefix != output.APIKeyPrefix {
		t.Fatalf("APIKeyPrefix = %q, want %q", repository.client.APIKeyPrefix, output.APIKeyPrefix)
	}
	if repository.client.WebhookSecret != output.WebhookSecret {
		t.Fatal("persisted WebhookSecret does not match returned secret")
	}
	if repository.client.PolicyVersion != "default-v1" {
		t.Fatalf("PolicyVersion = %q, want default-v1", repository.client.PolicyVersion)
	}
}

func TestServiceCreateClientRejectsUnknownPolicyVersion(t *testing.T) {
	repository := &fakeRepository{}
	service := NewServiceWithPolicyValidator(repository, &fakePolicyValidator{
		allowed: map[string]bool{"default-v1": true},
	})

	_, err := service.CreateClient(context.Background(), CreateInput{
		UserID:        42,
		Name:          "blog",
		PolicyVersion: "missing-v1",
	})
	if err == nil {
		t.Fatal("CreateClient() error = nil, want policy validation error")
	}
	if !strings.Contains(err.Error(), "invalid policy_version") {
		t.Fatalf("CreateClient() error = %q, want invalid policy_version", err.Error())
	}
	if repository.client != nil {
		t.Fatal("client should not be persisted for unknown policy_version")
	}
}

func TestServiceCreateClientRejectsInvalidInput(t *testing.T) {
	service := NewService(&fakeRepository{})

	tests := []struct {
		name    string
		input   CreateInput
		wantErr string
	}{
		{
			name: "missing user",
			input: CreateInput{
				Name: "blog",
			},
			wantErr: "User not authenticated",
		},
		{
			name: "missing name",
			input: CreateInput{
				UserID: 42,
			},
			wantErr: "name is required",
		},
		{
			name: "bad webhook scheme",
			input: CreateInput{
				UserID:     42,
				Name:       "blog",
				WebhookURL: "ftp://example.com/hook",
			},
			wantErr: "webhook_url must use https",
		},
		{
			name: "plain http webhook",
			input: CreateInput{
				UserID:     42,
				Name:       "blog",
				WebhookURL: "http://example.com/hook",
			},
			wantErr: "webhook_url must use https",
		},
		{
			name: "localhost webhook",
			input: CreateInput{
				UserID:     42,
				Name:       "blog",
				WebhookURL: "https://localhost/hook",
			},
			wantErr: "webhook_url must not target localhost",
		},
		{
			name: "loopback webhook",
			input: CreateInput{
				UserID:     42,
				Name:       "blog",
				WebhookURL: "https://127.0.0.1/hook",
			},
			wantErr: "webhook_url must not target private or local addresses",
		},
		{
			name: "metadata webhook",
			input: CreateInput{
				UserID:     42,
				Name:       "blog",
				WebhookURL: "https://169.254.169.254/latest/meta-data",
			},
			wantErr: "webhook_url must not target private or local addresses",
		},
		{
			name: "rfc1918 webhook",
			input: CreateInput{
				UserID:     42,
				Name:       "blog",
				WebhookURL: "https://10.0.0.5/hook",
			},
			wantErr: "webhook_url must not target private or local addresses",
		},
		{
			name: "ipv6 loopback webhook",
			input: CreateInput{
				UserID:     42,
				Name:       "blog",
				WebhookURL: "https://[::1]/hook",
			},
			wantErr: "webhook_url must not target private or local addresses",
		},
		{
			name: "ipv6 private webhook",
			input: CreateInput{
				UserID:     42,
				Name:       "blog",
				WebhookURL: "https://[fd00::1]/hook",
			},
			wantErr: "webhook_url must not target private or local addresses",
		},
		{
			name: "multicast webhook",
			input: CreateInput{
				UserID:     42,
				Name:       "blog",
				WebhookURL: "https://224.0.0.1/hook",
			},
			wantErr: "webhook_url must not target private or local addresses",
		},
		{
			name: "relative webhook",
			input: CreateInput{
				UserID:     42,
				Name:       "blog",
				WebhookURL: "/hook",
			},
			wantErr: "webhook_url must be a valid absolute URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.CreateClient(context.Background(), tt.input)
			if err == nil {
				t.Fatal("CreateClient() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("CreateClient() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestServiceListClientsDoesNotExposeSecrets(t *testing.T) {
	createdAt := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	service := NewService(&fakeRepository{
		clients: []models.ClientApplication{
			{
				ID:            11,
				Name:          "blog",
				Status:        StatusActive,
				APIKeyHash:    "secret-hash",
				APIKeyPrefix:  "hs_live_abc",
				WebhookSecret: "whsec_secret",
				CreatedAt:     createdAt,
				UpdatedAt:     createdAt,
			},
		},
	})

	output, err := service.ListClients(context.Background())
	if err != nil {
		t.Fatalf("ListClients() error = %v", err)
	}

	if len(output) != 1 {
		t.Fatalf("len(output) = %d, want 1", len(output))
	}
	if output[0].ID != 11 {
		t.Fatalf("ID = %d, want 11", output[0].ID)
	}
	if output[0].APIKeyPrefix != "hs_live_abc" {
		t.Fatalf("APIKeyPrefix = %q, want hs_live_abc", output[0].APIKeyPrefix)
	}
}

func TestServiceUpdateClientStatus(t *testing.T) {
	createdAt := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	repository := &fakeRepository{
		statusClient: models.ClientApplication{
			ID:            11,
			Name:          "blog",
			Status:        StatusInactive,
			APIKeyHash:    "secret-hash",
			APIKeyPrefix:  "hs_live_abc",
			WebhookSecret: "whsec_secret",
			WebhookURL:    "https://example.com/moderation",
			PolicyVersion: "default-v1",
			CreatedAt:     createdAt,
			UpdatedAt:     createdAt,
		},
	}
	service := NewService(repository)

	output, err := service.DeactivateClient(context.Background(), 42, "11")
	if err != nil {
		t.Fatalf("DeactivateClient() error = %v", err)
	}

	if repository.statusClientID != 11 {
		t.Fatalf("status client id = %d, want 11", repository.statusClientID)
	}
	if repository.status != StatusInactive {
		t.Fatalf("status = %q, want inactive", repository.status)
	}
	if output.ID != 11 {
		t.Fatalf("output ID = %d, want 11", output.ID)
	}
	if output.Status != StatusInactive {
		t.Fatalf("output Status = %q, want inactive", output.Status)
	}
	if output.APIKeyPrefix != "hs_live_abc" {
		t.Fatalf("APIKeyPrefix = %q, want hs_live_abc", output.APIKeyPrefix)
	}
	if strings.Contains(output.APIKeyPrefix, "secret-hash") {
		t.Fatal("output exposed API key hash")
	}

	_, err = service.ActivateClient(context.Background(), 42, "11")
	if err != nil {
		t.Fatalf("ActivateClient() error = %v", err)
	}
	if repository.status != StatusActive {
		t.Fatalf("status after activate = %q, want active", repository.status)
	}
}

func TestServiceUpdateClientStatusRejectsInvalidInput(t *testing.T) {
	service := NewService(&fakeRepository{})

	tests := []struct {
		name       string
		operatorID uint
		clientID   string
		wantErr    string
	}{
		{
			name:     "missing operator",
			clientID: "11",
			wantErr:  "User not authenticated",
		},
		{
			name:       "missing client id",
			operatorID: 42,
			wantErr:    "client id is required",
		},
		{
			name:       "bad client id",
			operatorID: 42,
			clientID:   "abc",
			wantErr:    "client id must be a positive integer",
		},
		{
			name:       "zero client id",
			operatorID: 42,
			clientID:   "0",
			wantErr:    "client id must be a positive integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.DeactivateClient(context.Background(), tt.operatorID, tt.clientID)
			if err == nil {
				t.Fatal("DeactivateClient() error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("DeactivateClient() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestServiceUpdateClientPolicyVersion(t *testing.T) {
	updatedAt := time.Date(2026, 6, 28, 12, 10, 0, 0, time.UTC)
	repository := &fakeRepository{
		policyClient: models.ClientApplication{
			ID:            11,
			Name:          "blog",
			Status:        StatusActive,
			APIKeyHash:    "secret-hash",
			APIKeyPrefix:  "hs_live_abc",
			WebhookSecret: "whsec_secret",
			WebhookURL:    "https://example.com/moderation",
			UpdatedAt:     updatedAt,
		},
	}
	validator := &fakePolicyValidator{
		allowed: map[string]bool{
			"":          true,
			"strict-v1": true,
		},
	}
	service := NewServiceWithPolicyValidator(repository, validator)

	output, err := service.UpdateClientPolicyVersion(context.Background(), 42, "11", " strict-v1 ")
	if err != nil {
		t.Fatalf("UpdateClientPolicyVersion() error = %v", err)
	}

	if repository.policyClientID != 11 {
		t.Fatalf("policy client id = %d, want 11", repository.policyClientID)
	}
	if repository.policyVersion != "strict-v1" {
		t.Fatalf("policy version = %q, want strict-v1", repository.policyVersion)
	}
	if len(validator.calls) != 1 || validator.calls[0] != "strict-v1" {
		t.Fatalf("validator calls = %#v, want strict-v1", validator.calls)
	}
	if output.ID != 11 {
		t.Fatalf("output ID = %d, want 11", output.ID)
	}
	if output.PolicyVersion != "strict-v1" {
		t.Fatalf("output PolicyVersion = %q, want strict-v1", output.PolicyVersion)
	}
	if output.APIKeyPrefix != "hs_live_abc" {
		t.Fatalf("APIKeyPrefix = %q, want hs_live_abc", output.APIKeyPrefix)
	}
	if strings.Contains(output.APIKeyPrefix, "secret-hash") {
		t.Fatal("output exposed API key hash")
	}
}

func TestServiceUpdateClientPolicyVersionAllowsDefaultReset(t *testing.T) {
	repository := &fakeRepository{
		policyClient: models.ClientApplication{
			ID:            11,
			Name:          "blog",
			Status:        StatusActive,
			APIKeyPrefix:  "hs_live_abc",
			PolicyVersion: "strict-v1",
		},
	}
	validator := &fakePolicyValidator{
		allowed: map[string]bool{
			"":          true,
			"strict-v1": true,
		},
	}
	service := NewServiceWithPolicyValidator(repository, validator)

	output, err := service.UpdateClientPolicyVersion(context.Background(), 42, "11", "   ")
	if err != nil {
		t.Fatalf("UpdateClientPolicyVersion() reset error = %v", err)
	}

	if repository.policyVersion != "" {
		t.Fatalf("policy version = %q, want default reset", repository.policyVersion)
	}
	if output.PolicyVersion != "" {
		t.Fatalf("output PolicyVersion = %q, want empty default reset", output.PolicyVersion)
	}
	if len(validator.calls) != 1 || validator.calls[0] != "" {
		t.Fatalf("validator calls = %#v, want empty version", validator.calls)
	}
}

func TestServiceUpdateClientPolicyVersionRejectsUnknownPolicyVersion(t *testing.T) {
	repository := &fakeRepository{}
	service := NewServiceWithPolicyValidator(repository, &fakePolicyValidator{
		allowed: map[string]bool{"strict-v1": true},
	})

	_, err := service.UpdateClientPolicyVersion(context.Background(), 42, "11", "missing-v1")
	if err == nil {
		t.Fatal("UpdateClientPolicyVersion() error = nil, want policy validation error")
	}
	if !strings.Contains(err.Error(), "invalid policy_version") {
		t.Fatalf("UpdateClientPolicyVersion() error = %q, want invalid policy_version", err.Error())
	}
	if repository.policyClientID != 0 {
		t.Fatalf("policy client id = %d, want no repository write", repository.policyClientID)
	}
}

func TestServiceUpdateClientPolicyVersionRejectsInvalidInput(t *testing.T) {
	service := NewService(&fakeRepository{})

	tests := []struct {
		name          string
		operatorID    uint
		clientID      string
		policyVersion string
		wantErr       string
	}{
		{
			name:          "missing operator",
			clientID:      "11",
			policyVersion: "strict-v1",
			wantErr:       "User not authenticated",
		},
		{
			name:          "missing client id",
			operatorID:    42,
			policyVersion: "strict-v1",
			wantErr:       "client id is required",
		},
		{
			name:          "bad client id",
			operatorID:    42,
			clientID:      "abc",
			policyVersion: "strict-v1",
			wantErr:       "client id must be a positive integer",
		},
		{
			name:          "zero client id",
			operatorID:    42,
			clientID:      "0",
			policyVersion: "strict-v1",
			wantErr:       "client id must be a positive integer",
		},
		{
			name:          "policy version too long",
			operatorID:    42,
			clientID:      "11",
			policyVersion: strings.Repeat("a", maxPolicyVersionLength+1),
			wantErr:       "policy_version must not exceed 50 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.UpdateClientPolicyVersion(
				context.Background(),
				tt.operatorID,
				tt.clientID,
				tt.policyVersion,
			)
			if err == nil {
				t.Fatal("UpdateClientPolicyVersion() error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("UpdateClientPolicyVersion() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestServiceUpdateClientWebhook(t *testing.T) {
	updatedAt := time.Date(2026, 6, 28, 12, 15, 0, 0, time.UTC)
	repository := &fakeRepository{
		webhookClient: models.ClientApplication{
			ID:            11,
			Name:          "blog",
			Status:        StatusActive,
			APIKeyHash:    "secret-hash",
			APIKeyPrefix:  "hs_live_abc",
			WebhookSecret: "old-secret",
			WebhookURL:    "https://old.example.com/moderation",
			PolicyVersion: "default-v1",
			UpdatedAt:     updatedAt,
		},
	}
	service := NewService(repository)

	output, err := service.UpdateClientWebhook(
		context.Background(),
		42,
		"11",
		" https://example.com/moderation ",
	)
	if err != nil {
		t.Fatalf("UpdateClientWebhook() error = %v", err)
	}

	if repository.webhookClientID != 11 {
		t.Fatalf("webhook client id = %d, want 11", repository.webhookClientID)
	}
	if repository.webhookURL != "https://example.com/moderation" {
		t.Fatalf("webhook URL = %q, want trimmed URL", repository.webhookURL)
	}
	if repository.webhookSecret == "" {
		t.Fatal("webhook secret was not generated")
	}
	if !strings.HasPrefix(repository.webhookSecret, "whsec_") {
		t.Fatalf("webhook secret = %q, want whsec_ prefix", repository.webhookSecret)
	}
	if repository.webhookSecret == "old-secret" {
		t.Fatal("webhook secret was not rotated")
	}
	if output.WebhookURL != "https://example.com/moderation" {
		t.Fatalf("output WebhookURL = %q, want updated URL", output.WebhookURL)
	}
	if output.WebhookSecret != repository.webhookSecret {
		t.Fatal("output WebhookSecret does not match persisted secret")
	}
	if output.APIKeyPrefix != "hs_live_abc" {
		t.Fatalf("APIKeyPrefix = %q, want hs_live_abc", output.APIKeyPrefix)
	}
	if strings.Contains(output.APIKeyPrefix, "secret-hash") {
		t.Fatal("output exposed API key hash")
	}
}

func TestServiceUpdateClientWebhookAllowsClearingURL(t *testing.T) {
	repository := &fakeRepository{
		webhookClient: models.ClientApplication{
			ID:            11,
			Name:          "blog",
			Status:        StatusActive,
			APIKeyPrefix:  "hs_live_abc",
			WebhookURL:    "https://old.example.com/moderation",
			WebhookSecret: "old-secret",
			PolicyVersion: "default-v1",
		},
	}
	service := NewService(repository)

	output, err := service.UpdateClientWebhook(context.Background(), 42, "11", "   ")
	if err != nil {
		t.Fatalf("UpdateClientWebhook() clear error = %v", err)
	}

	if repository.webhookURL != "" {
		t.Fatalf("webhook URL = %q, want cleared", repository.webhookURL)
	}
	if repository.webhookSecret != "" {
		t.Fatalf("webhook secret = %q, want cleared", repository.webhookSecret)
	}
	if output.WebhookURL != "" {
		t.Fatalf("output WebhookURL = %q, want empty", output.WebhookURL)
	}
	if output.WebhookSecret != "" {
		t.Fatalf("output WebhookSecret = %q, want empty", output.WebhookSecret)
	}
}

func TestServiceUpdateClientWebhookRejectsInvalidInput(t *testing.T) {
	service := NewService(&fakeRepository{})

	tests := []struct {
		name       string
		operatorID uint
		clientID   string
		webhookURL string
		wantErr    string
	}{
		{
			name:       "missing operator",
			clientID:   "11",
			webhookURL: "https://example.com/moderation",
			wantErr:    "User not authenticated",
		},
		{
			name:       "missing client id",
			operatorID: 42,
			webhookURL: "https://example.com/moderation",
			wantErr:    "client id is required",
		},
		{
			name:       "bad client id",
			operatorID: 42,
			clientID:   "abc",
			webhookURL: "https://example.com/moderation",
			wantErr:    "client id must be a positive integer",
		},
		{
			name:       "zero client id",
			operatorID: 42,
			clientID:   "0",
			webhookURL: "https://example.com/moderation",
			wantErr:    "client id must be a positive integer",
		},
		{
			name:       "webhook URL too long",
			operatorID: 42,
			clientID:   "11",
			webhookURL: "https://example.com/" + strings.Repeat("a", maxWebhookURLLength),
			wantErr:    "webhook_url must not exceed 500 characters",
		},
		{
			name:       "plain http webhook",
			operatorID: 42,
			clientID:   "11",
			webhookURL: "http://example.com/moderation",
			wantErr:    "webhook_url must use https",
		},
		{
			name:       "localhost webhook",
			operatorID: 42,
			clientID:   "11",
			webhookURL: "https://localhost/moderation",
			wantErr:    "webhook_url must not target localhost",
		},
		{
			name:       "relative webhook",
			operatorID: 42,
			clientID:   "11",
			webhookURL: "/moderation",
			wantErr:    "webhook_url must be a valid absolute URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.UpdateClientWebhook(
				context.Background(),
				tt.operatorID,
				tt.clientID,
				tt.webhookURL,
			)
			if err == nil {
				t.Fatal("UpdateClientWebhook() error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("UpdateClientWebhook() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestServiceRotateClientAPIKey(t *testing.T) {
	updatedAt := time.Date(2026, 6, 28, 12, 5, 0, 0, time.UTC)
	repository := &fakeRepository{
		rotatedClient: models.ClientApplication{
			ID:            11,
			Name:          "blog",
			Status:        StatusInactive,
			APIKeyPrefix:  "old-prefix",
			WebhookURL:    "https://example.com/moderation",
			WebhookSecret: "whsec_secret",
			PolicyVersion: "default-v1",
			UpdatedAt:     updatedAt,
		},
	}
	service := NewService(repository)

	output, err := service.RotateClientAPIKey(context.Background(), 42, "11")
	if err != nil {
		t.Fatalf("RotateClientAPIKey() error = %v", err)
	}

	if output.APIKey == "" {
		t.Fatal("APIKey is empty")
	}
	if !strings.HasPrefix(output.APIKey, "hs_live_") {
		t.Fatalf("APIKey = %q, want hs_live_ prefix", output.APIKey)
	}
	if repository.rotateClientID != 11 {
		t.Fatalf("rotate client id = %d, want 11", repository.rotateClientID)
	}
	if repository.rotatedAPIKeyHash != auth.HashAPIKey(output.APIKey) {
		t.Fatal("persisted API key hash does not match returned key")
	}
	if repository.rotatedAPIKeyPrefix != auth.APIKeyPrefix(output.APIKey) {
		t.Fatalf("persisted API key prefix = %q, want returned key prefix", repository.rotatedAPIKeyPrefix)
	}
	if output.APIKeyPrefix != auth.APIKeyPrefix(output.APIKey) {
		t.Fatalf("APIKeyPrefix = %q, want new key prefix", output.APIKeyPrefix)
	}
	if output.Status != StatusInactive {
		t.Fatalf("Status = %q, want existing inactive status", output.Status)
	}
	if output.WebhookURL != "https://example.com/moderation" {
		t.Fatalf("WebhookURL = %q, want existing webhook URL", output.WebhookURL)
	}
	if output.PolicyVersion != "default-v1" {
		t.Fatalf("PolicyVersion = %q, want default-v1", output.PolicyVersion)
	}
}

func TestServiceRotateClientAPIKeyRejectsInvalidInput(t *testing.T) {
	service := NewService(&fakeRepository{})

	tests := []struct {
		name       string
		operatorID uint
		clientID   string
		wantErr    string
	}{
		{
			name:     "missing operator",
			clientID: "11",
			wantErr:  "User not authenticated",
		},
		{
			name:       "missing client id",
			operatorID: 42,
			wantErr:    "client id is required",
		},
		{
			name:       "bad client id",
			operatorID: 42,
			clientID:   "abc",
			wantErr:    "client id must be a positive integer",
		},
		{
			name:       "zero client id",
			operatorID: 42,
			clientID:   "0",
			wantErr:    "client id must be a positive integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.RotateClientAPIKey(context.Background(), tt.operatorID, tt.clientID)
			if err == nil {
				t.Fatal("RotateClientAPIKey() error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("RotateClientAPIKey() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

type fakeRepository struct {
	client              *models.ClientApplication
	clients             []models.ClientApplication
	statusClient        models.ClientApplication
	statusClientID      uint
	status              string
	policyClient        models.ClientApplication
	policyClientID      uint
	policyVersion       string
	webhookClient       models.ClientApplication
	webhookClientID     uint
	webhookURL          string
	webhookSecret       string
	rotatedClient       models.ClientApplication
	rotateClientID      uint
	rotatedAPIKeyHash   string
	rotatedAPIKeyPrefix string
	err                 error
}

type fakePolicyValidator struct {
	allowed map[string]bool
	calls   []string
}

func (v *fakePolicyValidator) ValidatePolicyVersion(version string) error {
	v.calls = append(v.calls, version)
	if v.allowed[version] {
		return nil
	}

	return errString("policy_version " + version + " is not configured")
}

type errString string

func (e errString) Error() string {
	return string(e)
}

func (r *fakeRepository) CreateClient(ctx context.Context, client *models.ClientApplication) error {
	if r.err != nil {
		return r.err
	}

	copied := *client
	copied.ID = 11
	r.client = &copied
	client.ID = copied.ID
	return nil
}

func (r *fakeRepository) ListClients(ctx context.Context) ([]models.ClientApplication, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.clients, nil
}

func (r *fakeRepository) UpdateClientStatus(
	ctx context.Context,
	clientID uint,
	status string,
) (models.ClientApplication, error) {
	if r.err != nil {
		return models.ClientApplication{}, r.err
	}

	r.statusClientID = clientID
	r.status = status
	r.statusClient.ID = clientID
	r.statusClient.Status = status
	return r.statusClient, nil
}

func (r *fakeRepository) UpdateClientPolicyVersion(
	ctx context.Context,
	clientID uint,
	policyVersion string,
) (models.ClientApplication, error) {
	if r.err != nil {
		return models.ClientApplication{}, r.err
	}

	r.policyClientID = clientID
	r.policyVersion = policyVersion
	r.policyClient.ID = clientID
	r.policyClient.PolicyVersion = policyVersion
	return r.policyClient, nil
}

func (r *fakeRepository) UpdateClientWebhook(
	ctx context.Context,
	clientID uint,
	webhookURL string,
	webhookSecret string,
) (models.ClientApplication, error) {
	if r.err != nil {
		return models.ClientApplication{}, r.err
	}

	r.webhookClientID = clientID
	r.webhookURL = webhookURL
	r.webhookSecret = webhookSecret
	r.webhookClient.ID = clientID
	r.webhookClient.WebhookURL = webhookURL
	r.webhookClient.WebhookSecret = webhookSecret
	return r.webhookClient, nil
}

func (r *fakeRepository) RotateClientAPIKey(
	ctx context.Context,
	clientID uint,
	apiKeyHash string,
	apiKeyPrefix string,
) (models.ClientApplication, error) {
	if r.err != nil {
		return models.ClientApplication{}, r.err
	}

	r.rotateClientID = clientID
	r.rotatedAPIKeyHash = apiKeyHash
	r.rotatedAPIKeyPrefix = apiKeyPrefix
	r.rotatedClient.ID = clientID
	r.rotatedClient.APIKeyHash = apiKeyHash
	r.rotatedClient.APIKeyPrefix = apiKeyPrefix
	return r.rotatedClient, nil
}
