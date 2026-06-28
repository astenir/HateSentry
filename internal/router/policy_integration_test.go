//go:build integration

package router

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
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
		bytes.NewBufferString(`{"name":"policy update client","policy_version":"default-v1"}`),
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
	if created.PolicyVersion != "default-v1" {
		t.Fatalf("created policy_version = %q, want default-v1", created.PolicyVersion)
	}

	unknownUpdateRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/clients/"+createdID(created.ID)+"/policy",
		bytes.NewBufferString(`{"policy_version":"missing-v1"}`),
	)
	unknownUpdateRequest.Header.Set("Authorization", "Bearer "+token)
	unknownUpdateRequest.Header.Set("Content-Type", "application/json")
	unknownUpdateRecorder := httptest.NewRecorder()

	engine.ServeHTTP(unknownUpdateRecorder, unknownUpdateRequest)

	if unknownUpdateRecorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"unknown update status = %d, want 400, body = %s",
			unknownUpdateRecorder.Code,
			unknownUpdateRecorder.Body.String(),
		)
	}
	if !strings.Contains(unknownUpdateRecorder.Body.String(), "invalid policy_version") {
		t.Fatalf("unknown update body = %s, want invalid policy_version", unknownUpdateRecorder.Body.String())
	}

	updateRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/clients/"+createdID(created.ID)+"/policy",
		bytes.NewBufferString(`{"policy_version":"strict-v1"}`),
	)
	updateRequest.Header.Set("Authorization", "Bearer "+token)
	updateRequest.Header.Set("Content-Type", "application/json")
	updateRecorder := httptest.NewRecorder()

	engine.ServeHTTP(updateRecorder, updateRequest)

	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("update status = %d, want 200, body = %s", updateRecorder.Code, updateRecorder.Body.String())
	}
	var updated clients.ListOutput
	if err := json.Unmarshal(updateRecorder.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if updated.PolicyVersion != "strict-v1" {
		t.Fatalf("updated policy_version = %q, want strict-v1", updated.PolicyVersion)
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

	resetRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/clients/"+createdID(created.ID)+"/policy",
		bytes.NewBufferString(`{"policy_version":""}`),
	)
	resetRequest.Header.Set("Authorization", "Bearer "+token)
	resetRequest.Header.Set("Content-Type", "application/json")
	resetRecorder := httptest.NewRecorder()

	engine.ServeHTTP(resetRecorder, resetRequest)

	if resetRecorder.Code != http.StatusOK {
		t.Fatalf("reset status = %d, want 200, body = %s", resetRecorder.Code, resetRecorder.Body.String())
	}
	var reset clients.ListOutput
	if err := json.Unmarshal(resetRecorder.Body.Bytes(), &reset); err != nil {
		t.Fatalf("decode reset response: %v", err)
	}
	if reset.PolicyVersion != "" {
		t.Fatalf("reset policy_version = %q, want default reset", reset.PolicyVersion)
	}

	defaultCheckRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/moderation/check",
		bytes.NewBufferString(`{"content":"please check this comment again","source":"comment","external_id":"`+suffix+`-default"}`),
	)
	defaultCheckRequest.Header.Set("X-API-Key", created.APIKey)
	defaultCheckRequest.Header.Set("Content-Type", "application/json")
	defaultCheckRecorder := httptest.NewRecorder()

	engine.ServeHTTP(defaultCheckRecorder, defaultCheckRequest)

	if defaultCheckRecorder.Code != http.StatusOK {
		t.Fatalf(
			"default check status = %d, want 200, body = %s",
			defaultCheckRecorder.Code,
			defaultCheckRecorder.Body.String(),
		)
	}
	var defaultCheckOutput moderation.CheckOutput
	if err := json.Unmarshal(defaultCheckRecorder.Body.Bytes(), &defaultCheckOutput); err != nil {
		t.Fatalf("decode default check response: %v", err)
	}
	if defaultCheckOutput.Decision != moderation.DecisionReview {
		t.Fatalf("decision = %q, want review from default policy", defaultCheckOutput.Decision)
	}
	if defaultCheckOutput.PolicyVersion != "default-v1" {
		t.Fatalf("policy_version = %q, want default-v1", defaultCheckOutput.PolicyVersion)
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

func createdID(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}
