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
	service := NewService(repository)

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

type fakeRepository struct {
	client         *models.ClientApplication
	clients        []models.ClientApplication
	statusClient   models.ClientApplication
	statusClientID uint
	status         string
	err            error
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
