//go:build integration

package clients

import (
	"context"
	"os"
	"strings"
	"testing"

	"hatesentry/internal/auth"
	"hatesentry/internal/models"

	"github.com/google/uuid"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestGormRepositoryUpdateClientStatusRevokesAPIKeyIntegration(t *testing.T) {
	dsn := os.Getenv("HATESENTRY_TEST_DSN")
	if strings.TrimSpace(dsn) == "" {
		t.Skip("HATESENTRY_TEST_DSN is required for integration repository tests")
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := db.AutoMigrate(
		&models.User{},
		&models.ClientApplication{},
	); err != nil {
		t.Fatalf("auto migrate test database: %v", err)
	}

	ctx := context.Background()
	repository := NewGormRepository(db)
	suffix := strings.ReplaceAll(uuid.New().String(), "-", "")[:12]
	user := models.User{
		Username: "it-client-" + suffix,
		Email:    "it-client-" + suffix + "@example.test",
		Password: "not-used",
		Role:     "admin",
		Status:   "active",
		APIKey:   "it_client_" + suffix,
	}
	if err := db.WithContext(ctx).Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	apiKey, err := auth.GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey() error = %v", err)
	}
	client := models.ClientApplication{
		UserID:        user.ID,
		Name:          "integration client",
		APIKeyHash:    auth.HashAPIKey(apiKey),
		APIKeyPrefix:  auth.APIKeyPrefix(apiKey),
		Status:        StatusActive,
		WebhookURL:    "https://example.com/moderation/webhook",
		WebhookSecret: "whsec_integration",
		PolicyVersion: "default-v1",
	}
	if err := db.WithContext(ctx).Create(&client).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	originalAPIKeyHash := client.APIKeyHash
	originalAPIKeyPrefix := client.APIKeyPrefix
	originalWebhookURL := client.WebhookURL
	originalWebhookSecret := client.WebhookSecret
	originalPolicyVersion := client.PolicyVersion

	t.Cleanup(func() {
		db.Unscoped().Delete(&models.ClientApplication{}, client.ID)
		db.Unscoped().Delete(&models.User{}, user.ID)
	})

	principal, err := repository.AuthenticateAPIKey(ctx, apiKey)
	if err != nil {
		t.Fatalf("AuthenticateAPIKey() before deactivation error = %v", err)
	}
	if principal.ClientID != client.ID {
		t.Fatalf("principal ClientID = %d, want %d", principal.ClientID, client.ID)
	}

	updated, err := repository.UpdateClientStatus(ctx, client.ID, StatusInactive)
	if err != nil {
		t.Fatalf("UpdateClientStatus() deactivate error = %v", err)
	}
	if updated.Status != StatusInactive {
		t.Fatalf("updated status = %q, want inactive", updated.Status)
	}
	assertClientStatusUpdatePreservedFields(
		t,
		updated,
		originalAPIKeyHash,
		originalAPIKeyPrefix,
		originalWebhookURL,
		originalWebhookSecret,
		originalPolicyVersion,
	)

	updated, err = repository.UpdateClientStatus(ctx, client.ID, StatusInactive)
	if err != nil {
		t.Fatalf("UpdateClientStatus() idempotent deactivate error = %v", err)
	}
	if updated.Status != StatusInactive {
		t.Fatalf("updated status after idempotent deactivate = %q, want inactive", updated.Status)
	}
	assertClientStatusUpdatePreservedFields(
		t,
		updated,
		originalAPIKeyHash,
		originalAPIKeyPrefix,
		originalWebhookURL,
		originalWebhookSecret,
		originalPolicyVersion,
	)

	_, err = repository.AuthenticateAPIKey(ctx, apiKey)
	if err == nil {
		t.Fatal("AuthenticateAPIKey() after deactivation error = nil, want unauthorized")
	}
	if !strings.Contains(err.Error(), "Invalid API key") {
		t.Fatalf("AuthenticateAPIKey() after deactivation error = %q, want invalid api key", err.Error())
	}

	updated, err = repository.UpdateClientStatus(ctx, client.ID, StatusActive)
	if err != nil {
		t.Fatalf("UpdateClientStatus() activate error = %v", err)
	}
	if updated.Status != StatusActive {
		t.Fatalf("updated status = %q, want active", updated.Status)
	}
	assertClientStatusUpdatePreservedFields(
		t,
		updated,
		originalAPIKeyHash,
		originalAPIKeyPrefix,
		originalWebhookURL,
		originalWebhookSecret,
		originalPolicyVersion,
	)

	if _, err := repository.AuthenticateAPIKey(ctx, apiKey); err != nil {
		t.Fatalf("AuthenticateAPIKey() after reactivation error = %v", err)
	}
}

func assertClientStatusUpdatePreservedFields(
	t *testing.T,
	client models.ClientApplication,
	apiKeyHash string,
	apiKeyPrefix string,
	webhookURL string,
	webhookSecret string,
	policyVersion string,
) {
	t.Helper()

	if client.APIKeyHash != apiKeyHash {
		t.Fatalf("APIKeyHash changed after status update")
	}
	if client.APIKeyPrefix != apiKeyPrefix {
		t.Fatalf("APIKeyPrefix = %q, want %q", client.APIKeyPrefix, apiKeyPrefix)
	}
	if client.WebhookURL != webhookURL {
		t.Fatalf("WebhookURL = %q, want %q", client.WebhookURL, webhookURL)
	}
	if client.WebhookSecret != webhookSecret {
		t.Fatalf("WebhookSecret changed after status update")
	}
	if client.PolicyVersion != policyVersion {
		t.Fatalf("PolicyVersion = %q, want %q", client.PolicyVersion, policyVersion)
	}
}
