package moderation

import (
	"database/sql"
	"reflect"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"

	"hatesentry/internal/models"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestUserScopedResultQueryFiltersByUserAndRequestID(t *testing.T) {
	db := openDryRunDB(t)

	stmt := userScopedResultQuery(db, 7, "request-123").
		Find(&models.ModerationRequest{}).
		Statement

	sql := stmt.SQL.String()
	if !strings.Contains(sql, "user_id") {
		t.Fatalf("SQL = %q, want user_id filter", sql)
	}
	if !strings.Contains(sql, "request_id") {
		t.Fatalf("SQL = %q, want request_id filter", sql)
	}

	wantVars := []interface{}{uint(7), "request-123"}
	if !reflect.DeepEqual(stmt.Vars, wantVars) {
		t.Fatalf("Vars = %#v, want %#v", stmt.Vars, wantVars)
	}
}

func TestReviewCaseListQueryFiltersByStatus(t *testing.T) {
	db := openDryRunDB(t)

	stmt := reviewCaseListQuery(db, ReviewStatusPending).
		Find(&models.ReviewCase{}).
		Statement

	sql := stmt.SQL.String()
	if !strings.Contains(sql, "status") {
		t.Fatalf("SQL = %q, want status filter", sql)
	}
	if !strings.Contains(sql, "ORDER BY created_at ASC") {
		t.Fatalf("SQL = %q, want created_at ordering", sql)
	}

	wantVars := []interface{}{string(ReviewStatusPending)}
	if !reflect.DeepEqual(stmt.Vars, wantVars) {
		t.Fatalf("Vars = %#v, want %#v", stmt.Vars, wantVars)
	}
}

func TestReviewCaseByIDQueryFiltersByCaseID(t *testing.T) {
	db := openDryRunDB(t)

	stmt := reviewCaseByIDQuery(db, 3).
		Find(&models.ReviewCase{}).
		Statement

	sql := stmt.SQL.String()
	if !strings.Contains(sql, "id") {
		t.Fatalf("SQL = %q, want id filter", sql)
	}

	wantVars := []interface{}{uint(3)}
	if !reflect.DeepEqual(stmt.Vars, wantVars) {
		t.Fatalf("Vars = %#v, want %#v", stmt.Vars, wantVars)
	}
}

func TestClientExternalIDQueryFiltersByClientAndExternalID(t *testing.T) {
	db := openDryRunDB(t)

	stmt := clientExternalIDQuery(db, 11, "comment_123").
		Find(&models.ModerationRequest{}).
		Statement

	sql := stmt.SQL.String()
	if !strings.Contains(sql, "client_id") {
		t.Fatalf("SQL = %q, want client_id filter", sql)
	}
	if !strings.Contains(sql, "external_id") {
		t.Fatalf("SQL = %q, want external_id filter", sql)
	}

	wantVars := []interface{}{uint(11), "comment_123"}
	if !reflect.DeepEqual(stmt.Vars, wantVars) {
		t.Fatalf("Vars = %#v, want %#v", stmt.Vars, wantVars)
	}
}

func openDryRunDB(t *testing.T) *gorm.DB {
	t.Helper()

	sqlDB, err := sql.Open(
		"mysql",
		"hatesentry:hatesentry@tcp(localhost:3306)/hatesentry?charset=utf8mb4&parseTime=True&loc=Local",
	)
	if err != nil {
		t.Fatalf("open sql handle: %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Fatalf("close sql handle: %v", err)
		}
	})

	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{
		DryRun:               true,
		DisableAutomaticPing: true,
	})
	if err != nil {
		t.Fatalf("open dry-run db: %v", err)
	}

	return db
}
