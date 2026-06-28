//go:build integration

package router

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"hatesentry/internal/auth"
	"hatesentry/internal/clients"
	"hatesentry/internal/config"
	"hatesentry/internal/models"
	"hatesentry/internal/moderation"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestRouterClientPolicyAssignmentIntegration(t *testing.T) {
	dsn := os.Getenv("HATESENTRY_TEST_DSN")
	if strings.TrimSpace(dsn) == "" {
		t.Skip("HATESENTRY_TEST_DSN is required for router integration tests")
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := db.AutoMigrate(
		&models.User{},
		&models.ClientApplication{},
		&models.ModerationRequest{},
		&models.ModerationResult{},
		&models.ReviewCase{},
		&models.WebhookDelivery{},
	); err != nil {
		t.Fatalf("auto migrate test database: %v", err)
	}

	gin.SetMode(gin.TestMode)
	jwtManager := auth.NewJWTManager(&config.JWTConfig{
		Secret:      "test-secret",
		ExpireHours: 1,
		Issuer:      "hatesentry-test",
	})
	strictPolicy, err := moderation.NewPolicy("strict-v1", 0.2, 0.5)
	if err != nil {
		t.Fatalf("NewPolicy() error = %v", err)
	}
	policies, err := moderation.NewPolicySet(moderation.DefaultPolicy(), strictPolicy)
	if err != nil {
		t.Fatalf("NewPolicySet() error = %v", err)
	}

	router := NewRouterWithPolicies(
		db,
		nil,
		nil,
		nil,
		nil,
		nil,
		jwtManager,
		policies,
		config.ModerationRateLimitConfig{},
	)
	router.moderationAnalyzer = routerPolicyAnalyzer{
		suggestion: moderation.ProviderSuggestion{
			RiskScore: 0.6,
			Labels:    []string{"harassment"},
			Reason:    "Strict policy should block this score.",
			RawOutput: `{"risk_score":0.6,"labels":["harassment"],"reason":"Strict policy should block this score."}`,
		},
		provider: moderation.ProviderInfo{
			Provider: "test-provider",
			Model:    "test-model",
		},
	}
	engine := router.Setup()

	ctx := context.Background()
	suffix := strings.ReplaceAll(uuid.New().String(), "-", "")[:12]
	user := models.User{
		Username: "it-router-policy-" + suffix,
		Email:    "it-router-policy-" + suffix + "@example.test",
		Password: "not-used",
		Role:     "admin",
		Status:   "active",
		APIKey:   "it_router_policy_" + suffix,
	}
	if err := db.WithContext(ctx).Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() {
		db.Unscoped().Where("user_id = ?", user.ID).Delete(&models.ReviewCase{})
		db.Unscoped().Where("user_id = ?", user.ID).Delete(&models.ModerationResult{})
		db.Unscoped().Where("user_id = ?", user.ID).Delete(&models.ModerationRequest{})
		db.Unscoped().Where("user_id = ?", user.ID).Delete(&models.ClientApplication{})
		db.Unscoped().Delete(&models.User{}, user.ID)
	})

	token, err := jwtManager.GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	unknownPolicyRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/clients",
		bytes.NewBufferString(`{"name":"missing policy client","policy_version":"missing-v1"}`),
	)
	unknownPolicyRequest.Header.Set("Authorization", "Bearer "+token)
	unknownPolicyRequest.Header.Set("Content-Type", "application/json")
	unknownPolicyRecorder := httptest.NewRecorder()

	engine.ServeHTTP(unknownPolicyRecorder, unknownPolicyRequest)

	if unknownPolicyRecorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"unknown policy status = %d, want 400, body = %s",
			unknownPolicyRecorder.Code,
			unknownPolicyRecorder.Body.String(),
		)
	}
	if !strings.Contains(unknownPolicyRecorder.Body.String(), "invalid policy_version") {
		t.Fatalf("unknown policy body = %s, want invalid policy_version", unknownPolicyRecorder.Body.String())
	}

	createRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/clients",
		bytes.NewBufferString(`{"name":"strict policy client","policy_version":"strict-v1"}`),
	)
	createRequest.Header.Set("Authorization", "Bearer "+token)
	createRequest.Header.Set("Content-Type", "application/json")
	createRecorder := httptest.NewRecorder()

	engine.ServeHTTP(createRecorder, createRequest)

	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201, body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	var created clients.CreateOutput
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.APIKey == "" {
		t.Fatal("created API key is empty")
	}
	if created.PolicyVersion != "strict-v1" {
		t.Fatalf("created policy_version = %q, want strict-v1", created.PolicyVersion)
	}

	checkRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/moderation/check",
		bytes.NewBufferString(`{"content":"please check this comment","source":"comment","external_id":"`+suffix+`"}`),
	)
	checkRequest.Header.Set("X-API-Key", created.APIKey)
	checkRequest.Header.Set("Content-Type", "application/json")
	checkRecorder := httptest.NewRecorder()

	engine.ServeHTTP(checkRecorder, checkRequest)

	if checkRecorder.Code != http.StatusOK {
		t.Fatalf("check status = %d, want 200, body = %s", checkRecorder.Code, checkRecorder.Body.String())
	}
	var checkOutput moderation.CheckOutput
	if err := json.Unmarshal(checkRecorder.Body.Bytes(), &checkOutput); err != nil {
		t.Fatalf("decode check response: %v", err)
	}
	if checkOutput.Decision != moderation.DecisionBlock {
		t.Fatalf("decision = %q, want block from strict policy", checkOutput.Decision)
	}
	if checkOutput.PolicyVersion != "strict-v1" {
		t.Fatalf("policy_version = %q, want strict-v1", checkOutput.PolicyVersion)
	}
}

type routerPolicyAnalyzer struct {
	suggestion moderation.ProviderSuggestion
	provider   moderation.ProviderInfo
}

func (a routerPolicyAnalyzer) AnalyzeText(
	ctx context.Context,
	content string,
) (moderation.ProviderSuggestion, moderation.ProviderInfo, error) {
	return a.suggestion, a.provider, nil
}
