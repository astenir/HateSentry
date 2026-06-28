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

type clientHandlerRepository struct {
	client              *models.ClientApplication
	clients             []models.ClientApplication
	statusClient        models.ClientApplication
	statusClientID      uint
	status              string
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
