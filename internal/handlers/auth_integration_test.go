//go:build integration

package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"hatesentry/internal/auth"
	"hatesentry/internal/config"
	apperrors "hatesentry/internal/errors"
	"hatesentry/internal/models"
	"os"
	"testing"
	"time"

	mysqlconfig "github.com/go-sql-driver/mysql"
	mysqlDriver "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestAuthHandlerCreateRegisteredUserBootstrapsAdminIntegration(t *testing.T) {
	db := openIsolatedAuthDB(t)
	handler := NewAuthHandlerWithConfig(
		db,
		auth.NewJWTManager(&config.JWTConfig{
			Secret:      "test-secret",
			ExpireHours: 1,
			Issuer:      "hatesentry-test",
		}),
		config.AuthConfig{AdminBootstrapToken: "bootstrap-secret"},
	)

	passwordHash, err := auth.HashPassword("password123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	_, appErr := handler.createRegisteredUser(
		context.Background(),
		RegisterRequest{
			Username: "missing-token",
			Email:    "missing-token@example.com",
		},
		passwordHash,
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
			AdminBootstrapToken: "bootstrap-secret",
		},
		passwordHash,
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
		},
		passwordHash,
	)
	if appErr != nil {
		t.Fatalf("create second user error = %v", appErr)
	}
	if user.Role != "user" {
		t.Fatalf("second user role = %q, want user", user.Role)
	}
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
