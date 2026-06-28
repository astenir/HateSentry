package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"hatesentry/internal/auth"
	"hatesentry/internal/clients"
	"hatesentry/internal/models"
)

func TestClientHandlerDeactivate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repository := &clientHandlerRepository{
		statusClient: models.ClientApplication{
			ID:            11,
			Name:          "blog",
			Status:        clients.StatusInactive,
			APIKeyHash:    "secret-hash",
			APIKeyPrefix:  "hs_live_abc",
			WebhookSecret: "whsec_secret",
			WebhookURL:    "https://example.com/moderation",
			PolicyVersion: "default-v1",
			CreatedAt:     time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC),
			UpdatedAt:     time.Date(2026, 6, 28, 12, 5, 0, 0, time.UTC),
		},
	}
	handler := NewClientHandler(clients.NewService(repository))

	engine := gin.New()
	engine.POST("/api/v1/admin/clients/:id/deactivate", func(c *gin.Context) {
		c.Set(auth.UserContextKey, &auth.Claims{
			UserID:   42,
			Username: "admin",
			Role:     "admin",
		})
		handler.Deactivate(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/clients/11/deactivate", nil)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if strings.Contains(body, "secret-hash") || strings.Contains(body, "whsec_secret") {
		t.Fatalf("response leaked secret material: %s", body)
	}

	var response clients.ListOutput
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ID != 11 {
		t.Fatalf("ID = %d, want 11", response.ID)
	}
	if response.Status != clients.StatusInactive {
		t.Fatalf("Status = %q, want inactive", response.Status)
	}
	if repository.statusClientID != 11 {
		t.Fatalf("status client id = %d, want 11", repository.statusClientID)
	}
	if repository.status != clients.StatusInactive {
		t.Fatalf("status = %q, want inactive", repository.status)
	}
}

func TestClientHandlerDeactivateRequiresUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewClientHandler(clients.NewService(&clientHandlerRepository{}))
	engine := gin.New()
	engine.POST("/api/v1/admin/clients/:id/deactivate", handler.Deactivate)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/clients/11/deactivate", nil)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}

func TestClientHandlerRotateAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repository := &clientHandlerRepository{
		rotatedClient: models.ClientApplication{
			ID:            11,
			Name:          "blog",
			Status:        clients.StatusActive,
			APIKeyHash:    "secret-hash",
			APIKeyPrefix:  "hs_live_new",
			WebhookSecret: "whsec_secret",
			WebhookURL:    "https://example.com/moderation",
			PolicyVersion: "default-v1",
			UpdatedAt:     time.Date(2026, 6, 28, 12, 5, 0, 0, time.UTC),
		},
	}
	handler := NewClientHandler(clients.NewService(repository))

	engine := gin.New()
	engine.POST("/api/v1/admin/clients/:id/api-key/rotate", func(c *gin.Context) {
		c.Set(auth.UserContextKey, &auth.Claims{
			UserID:   42,
			Username: "admin",
			Role:     "admin",
		})
		handler.RotateAPIKey(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/clients/11/api-key/rotate", nil)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if strings.Contains(body, "secret-hash") || strings.Contains(body, "whsec_secret") {
		t.Fatalf("response leaked secret material: %s", body)
	}

	var response clients.RotateAPIKeyOutput
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ID != 11 {
		t.Fatalf("ID = %d, want 11", response.ID)
	}
	if response.APIKey == "" {
		t.Fatal("APIKey is empty")
	}
	if !strings.HasPrefix(response.APIKey, "hs_live_") {
		t.Fatalf("APIKey = %q, want hs_live_ prefix", response.APIKey)
	}
	if repository.rotateClientID != 11 {
		t.Fatalf("rotate client id = %d, want 11", repository.rotateClientID)
	}
	if repository.rotatedAPIKeyHash != auth.HashAPIKey(response.APIKey) {
		t.Fatal("rotated API key hash does not match returned key")
	}
}

func TestClientHandlerRotateAPIKeyRequiresUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewClientHandler(clients.NewService(&clientHandlerRepository{}))
	engine := gin.New()
	engine.POST("/api/v1/admin/clients/:id/api-key/rotate", handler.RotateAPIKey)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/clients/11/api-key/rotate", nil)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}

func TestClientHandlerUpdatePolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repository := &clientHandlerRepository{
		policyClient: models.ClientApplication{
			ID:            11,
			Name:          "blog",
			Status:        clients.StatusActive,
			APIKeyHash:    "secret-hash",
			APIKeyPrefix:  "hs_live_abc",
			WebhookSecret: "whsec_secret",
			WebhookURL:    "https://example.com/moderation",
			PolicyVersion: "default-v1",
			UpdatedAt:     time.Date(2026, 6, 28, 12, 10, 0, 0, time.UTC),
		},
	}
	handler := NewClientHandler(clients.NewService(repository))

	engine := gin.New()
	engine.POST("/api/v1/admin/clients/:id/policy", func(c *gin.Context) {
		c.Set(auth.UserContextKey, &auth.Claims{
			UserID:   42,
			Username: "admin",
			Role:     "admin",
		})
		handler.UpdatePolicy(c)
	})

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/clients/11/policy",
		strings.NewReader(`{"policy_version":"strict-v1"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if strings.Contains(body, "secret-hash") || strings.Contains(body, "whsec_secret") {
		t.Fatalf("response leaked secret material: %s", body)
	}

	var response clients.ListOutput
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ID != 11 {
		t.Fatalf("ID = %d, want 11", response.ID)
	}
	if response.PolicyVersion != "strict-v1" {
		t.Fatalf("PolicyVersion = %q, want strict-v1", response.PolicyVersion)
	}
	if repository.policyClientID != 11 {
		t.Fatalf("policy client id = %d, want 11", repository.policyClientID)
	}
	if repository.policyVersion != "strict-v1" {
		t.Fatalf("policy version = %q, want strict-v1", repository.policyVersion)
	}
}

func TestClientHandlerUpdatePolicyRequiresUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewClientHandler(clients.NewService(&clientHandlerRepository{}))
	engine := gin.New()
	engine.POST("/api/v1/admin/clients/:id/policy", handler.UpdatePolicy)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/clients/11/policy",
		strings.NewReader(`{"policy_version":"strict-v1"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}

func TestClientHandlerUpdateWebhook(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repository := &clientHandlerRepository{
		webhookClient: models.ClientApplication{
			ID:            11,
			Name:          "blog",
			Status:        clients.StatusActive,
			APIKeyHash:    "secret-hash",
			APIKeyPrefix:  "hs_live_abc",
			WebhookSecret: "old-secret",
			WebhookURL:    "https://old.example.com/moderation",
			PolicyVersion: "default-v1",
			CreatedAt:     time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC),
			UpdatedAt:     time.Date(2026, 6, 28, 12, 15, 0, 0, time.UTC),
		},
	}
	handler := NewClientHandler(clients.NewService(repository))

	engine := gin.New()
	engine.POST("/api/v1/admin/clients/:id/webhook", func(c *gin.Context) {
		c.Set(auth.UserContextKey, &auth.Claims{
			UserID:   42,
			Username: "admin",
			Role:     "admin",
		})
		handler.UpdateWebhook(c)
	})

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/clients/11/webhook",
		strings.NewReader(`{"webhook_url":"https://example.com/moderation"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if strings.Contains(body, "secret-hash") || strings.Contains(body, "old-secret") {
		t.Fatalf("response leaked stored secret material: %s", body)
	}

	var response clients.UpdateWebhookOutput
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ID != 11 {
		t.Fatalf("ID = %d, want 11", response.ID)
	}
	if response.WebhookURL != "https://example.com/moderation" {
		t.Fatalf("WebhookURL = %q, want updated URL", response.WebhookURL)
	}
	if response.WebhookSecret == "" {
		t.Fatal("WebhookSecret is empty")
	}
	if !strings.HasPrefix(response.WebhookSecret, "whsec_") {
		t.Fatalf("WebhookSecret = %q, want whsec_ prefix", response.WebhookSecret)
	}
	if repository.webhookClientID != 11 {
		t.Fatalf("webhook client id = %d, want 11", repository.webhookClientID)
	}
	if repository.webhookURL != "https://example.com/moderation" {
		t.Fatalf("webhook URL = %q, want updated URL", repository.webhookURL)
	}
	if repository.webhookSecret != response.WebhookSecret {
		t.Fatal("persisted webhook secret does not match response")
	}
}

func TestClientHandlerUpdateWebhookAllowsExplicitClear(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repository := &clientHandlerRepository{
		webhookClient: models.ClientApplication{
			ID:           11,
			Name:         "blog",
			Status:       clients.StatusActive,
			APIKeyPrefix: "hs_live_abc",
			WebhookURL:   "https://old.example.com/moderation",
		},
	}
	handler := NewClientHandler(clients.NewService(repository))

	engine := gin.New()
	engine.POST("/api/v1/admin/clients/:id/webhook", func(c *gin.Context) {
		c.Set(auth.UserContextKey, &auth.Claims{
			UserID:   42,
			Username: "admin",
			Role:     "admin",
		})
		handler.UpdateWebhook(c)
	})

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/clients/11/webhook",
		strings.NewReader(`{"webhook_url":""}`),
	)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if repository.webhookURL != "" {
		t.Fatalf("webhook URL = %q, want cleared", repository.webhookURL)
	}
	if repository.webhookSecret != "" {
		t.Fatalf("webhook secret = %q, want cleared", repository.webhookSecret)
	}

	var response clients.UpdateWebhookOutput
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.WebhookURL != "" {
		t.Fatalf("WebhookURL = %q, want empty", response.WebhookURL)
	}
	if response.WebhookSecret != "" {
		t.Fatalf("WebhookSecret = %q, want empty", response.WebhookSecret)
	}
}

func TestClientHandlerUpdateWebhookRequiresExplicitField(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repository := &clientHandlerRepository{}
	handler := NewClientHandler(clients.NewService(repository))

	engine := gin.New()
	engine.POST("/api/v1/admin/clients/:id/webhook", func(c *gin.Context) {
		c.Set(auth.UserContextKey, &auth.Claims{
			UserID:   42,
			Username: "admin",
			Role:     "admin",
		})
		handler.UpdateWebhook(c)
	})

	tests := []struct {
		name string
		body string
	}{
		{
			name: "missing field",
			body: `{}`,
		},
		{
			name: "null field",
			body: `{"webhook_url":null}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/admin/clients/11/webhook",
				strings.NewReader(tt.body),
			)
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			engine.ServeHTTP(recorder, req)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s, want 400", recorder.Code, recorder.Body.String())
			}
			if repository.webhookClientID != 0 {
				t.Fatalf("repository was called with client id %d", repository.webhookClientID)
			}
		})
	}
}

func TestClientHandlerUpdateWebhookRequiresUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewClientHandler(clients.NewService(&clientHandlerRepository{}))
	engine := gin.New()
	engine.POST("/api/v1/admin/clients/:id/webhook", handler.UpdateWebhook)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/clients/11/webhook",
		strings.NewReader(`{"webhook_url":"https://example.com/moderation"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}

type clientHandlerRepository struct {
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

func (r *clientHandlerRepository) CreateClient(
	ctx context.Context,
	client *models.ClientApplication,
) error {
	if r.err != nil {
		return r.err
	}

	copied := *client
	copied.ID = 11
	r.client = &copied
	client.ID = copied.ID
	return nil
}

func (r *clientHandlerRepository) ListClients(ctx context.Context) ([]models.ClientApplication, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.clients, nil
}

func (r *clientHandlerRepository) UpdateClientStatus(
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

func (r *clientHandlerRepository) UpdateClientPolicyVersion(
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

func (r *clientHandlerRepository) UpdateClientWebhook(
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

func (r *clientHandlerRepository) RotateClientAPIKey(
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
