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
	sqlDB, err := sql.Open(
		"mysql",
		"hatesentry:hatesentry@tcp(localhost:3306)/hatesentry?charset=utf8mb4&parseTime=True&loc=Local",
	)
	if err != nil {
		t.Fatalf("open sql handle: %v", err)
	}
	defer sqlDB.Close()

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
