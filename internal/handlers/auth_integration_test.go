//go:build integration

package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"hatesentry/internal/auth"
	"hatesentry/internal/config"
	apperrors "hatesentry/internal/errors"
	"hatesentry/internal/models"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	mysqlconfig "github.com/go-sql-driver/mysql"
	mysqlDriver "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestAuthHandlerCreateRegisteredUserBootstrapsAdminIntegration(t *testing.T) {
	db := openIsolatedAuthDB(t)
	handler := newTestAuthHandler(db)

	_, appErr := handler.createRegisteredUser(
		context.Background(),
		RegisterRequest{
			Username: "missing-token",
			Email:    "missing-token@example.com",
			Password: "password123",
		},
	)
	if appErr == nil {
		t.Fatal("createRegisteredUser() error = nil, want bootstrap token error")
	}
	if appErr.Code != apperrors.ErrCodeForbidden {
		t.Fatalf("createRegisteredUser() code = %s, want %s", appErr.Code, apperrors.ErrCodeForbidden)
	}

	var userCount int64
	if err := db.Model(&models.User{}).Count(&userCount).Error; err != nil {
		t.Fatalf("count users after rejected bootstrap: %v", err)
	}
	if userCount != 0 {
		t.Fatalf("user count after rejected bootstrap = %d, want 0", userCount)
	}

	admin, appErr := handler.createRegisteredUser(
		context.Background(),
		RegisterRequest{
			Username:            "admin",
			Email:               "admin@example.com",
			Password:            "password123",
			AdminBootstrapToken: "bootstrap-secret",
		},
	)
	if appErr != nil {
		t.Fatalf("create admin error = %v", appErr)
	}
	if admin.Role != "admin" {
		t.Fatalf("admin role = %q, want admin", admin.Role)
	}

	user, appErr := handler.createRegisteredUser(
		context.Background(),
		RegisterRequest{
			Username: "operator",
			Email:    "operator@example.com",
			Password: "password123",
		},
	)
	if appErr != nil {
		t.Fatalf("create second user error = %v", appErr)
	}
	if user.Role != "user" {
		t.Fatalf("second user role = %q, want user", user.Role)
	}
}

func TestAuthHandlerCreateRegisteredUserSerializesConcurrentBootstrapIntegration(t *testing.T) {
	db := openIsolatedAuthDB(t)
	handlers := []*AuthHandler{
		newTestAuthHandler(db),
		newTestAuthHandler(db),
	}

	type result struct {
		user   models.User
		appErr *apperrors.AppError
	}

	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan result, len(handlers))
	for i, handler := range handlers {
		i := i
		handler := handler
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			user, appErr := handler.createRegisteredUser(
				ctx,
				RegisterRequest{
					Username:            fmt.Sprintf("bootstrap-%d", i),
					Email:               fmt.Sprintf("bootstrap-%d@example.com", i),
					Password:            "password123",
					AdminBootstrapToken: "bootstrap-secret",
				},
			)
			results <- result{user: user, appErr: appErr}
		}()
	}

	close(start)
	wg.Wait()
	close(results)

	roleCounts := map[string]int{}
	for got := range results {
		if got.appErr != nil {
			t.Fatalf("concurrent create user error = %v", got.appErr)
		}
		roleCounts[got.user.Role]++
	}

	if roleCounts["admin"] != 1 {
		t.Fatalf("admin count = %d, want 1", roleCounts["admin"])
	}
	if roleCounts["user"] != 1 {
		t.Fatalf("user count = %d, want 1", roleCounts["user"])
	}

	var dbAdminCount int64
	if err := db.Model(&models.User{}).Where("role = ?", "admin").Count(&dbAdminCount).Error; err != nil {
		t.Fatalf("count admin users: %v", err)
	}
	if dbAdminCount != 1 {
		t.Fatalf("database admin count = %d, want 1", dbAdminCount)
	}
}

func TestAuthHandlerRegisterHTTPIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openIsolatedAuthDB(t)
	handler := newTestAuthHandler(db)
	router := gin.New()
	router.POST("/register", handler.Register)

	status, _ := postRegister(t, router, RegisterRequest{
		Username: "missing-token",
		Email:    "missing-token@example.com",
		Password: "password123",
	})
	if status != http.StatusForbidden {
		t.Fatalf("missing token status = %d, want %d", status, http.StatusForbidden)
	}

	status, admin := postRegister(t, router, RegisterRequest{
		Username:            "admin",
		Email:               "admin@example.com",
		Password:            "password123",
		AdminBootstrapToken: "bootstrap-secret",
	})
	if status != http.StatusCreated {
		t.Fatalf("admin status = %d, want %d", status, http.StatusCreated)
	}
	if admin.User.Role != "admin" {
		t.Fatalf("admin role = %q, want admin", admin.User.Role)
	}

	status, user := postRegister(t, router, RegisterRequest{
		Username: "operator",
		Email:    "operator@example.com",
		Password: "password123",
	})
	if status != http.StatusCreated {
		t.Fatalf("user status = %d, want %d", status, http.StatusCreated)
	}
	if user.User.Role != "user" {
		t.Fatalf("user role = %q, want user", user.User.Role)
	}

	status, _ = postRegister(t, router, RegisterRequest{
		Username: "operator",
		Email:    "other-operator@example.com",
		Password: "password123",
	})
	if status != http.StatusConflict {
		t.Fatalf("duplicate status = %d, want %d", status, http.StatusConflict)
	}
}

func postRegister(t *testing.T, router *gin.Engine, req RegisterRequest) (int, LoginResponse) {
	t.Helper()

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal register request: %v", err)
	}

	httpReq := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httpReq)

	var response LoginResponse
	if recorder.Code == http.StatusCreated {
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode register response: %v", err)
		}
	}

	return recorder.Code, response
}

func newTestAuthHandler(db *gorm.DB) *AuthHandler {
	return NewAuthHandlerWithConfig(
		db,
		auth.NewJWTManager(&config.JWTConfig{
			Secret:      "test-secret",
			ExpireHours: 1,
			Issuer:      "hatesentry-test",
		}),
		config.AuthConfig{AdminBootstrapToken: "bootstrap-secret"},
	)
}

func openIsolatedAuthDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("HATESENTRY_TEST_DSN")
	if dsn == "" {
		t.Skip("HATESENTRY_TEST_DSN is not set")
	}

	mysqlConfig, err := mysqlconfig.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("parse test dsn: %v", err)
	}

	adminDB, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open admin db: %v", err)
	}
	t.Cleanup(func() {
		if err := adminDB.Close(); err != nil {
			t.Fatalf("close admin db: %v", err)
		}
	})

	databaseName := fmt.Sprintf("hatesentry_auth_%d", time.Now().UnixNano())
	// security: databaseName is generated locally from a fixed prefix and timestamp.
	if _, err := adminDB.Exec("CREATE DATABASE `" + databaseName + "`"); err != nil {
		t.Fatalf("create isolated database: %v", err)
	}
	t.Cleanup(func() {
		if _, err := adminDB.Exec("DROP DATABASE `" + databaseName + "`"); err != nil {
			t.Fatalf("drop isolated database: %v", err)
		}
	})

	mysqlConfig.DBName = databaseName
	db, err := gorm.Open(mysqlDriver.Open(mysqlConfig.FormatDSN()), &gorm.Config{})
	if err != nil {
		t.Fatalf("open isolated gorm db: %v", err)
	}

	if err := db.AutoMigrate(&models.User{}, &models.SystemLock{}); err != nil {
		t.Fatalf("auto migrate auth tables: %v", err)
	}

	return db
}
