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

func TestModerationHistoryQueryFiltersAndOrders(t *testing.T) {
	db := openDryRunDB(t)
	clientID := uint(11)

	stmt := moderationHistoryQuery(db, HistoryFilter{
		Decision:   DecisionReview,
		ClientID:   &clientID,
		ExternalID: "comment_123",
		Limit:      25,
	}).Find(&models.ModerationResult{}).Statement

	sql := stmt.SQL.String()
	wantSQL := []string{
		"moderation_results",
		"JOIN moderation_requests",
		"moderation_requests.deleted_at IS NULL",
		"moderation_results.decision",
		"moderation_results.client_id",
		"moderation_requests.external_id",
		"ORDER BY moderation_results.created_at DESC",
		"LIMIT 25",
	}
	for _, want := range wantSQL {
		if !strings.Contains(sql, want) {
			t.Fatalf("SQL = %q, want %q", sql, want)
		}
	}

	wantVars := []interface{}{string(DecisionReview), clientID, "comment_123"}
	if !reflect.DeepEqual(stmt.Vars, wantVars) {
		t.Fatalf("Vars = %#v, want %#v", stmt.Vars, wantVars)
	}
}

func TestStatsQueriesUseExpectedFilters(t *testing.T) {
	db := openDryRunDB(t)

	tests := []struct {
		name     string
		stmt     *gorm.Statement
		wantSQL  []string
		wantVars []interface{}
	}{
		{
			name: "policy decision",
			stmt: policyDecisionStatsQuery(db, DecisionAllow).
				Count(new(int64)).
				Statement,
			wantSQL:  []string{"moderation_results", "decision"},
			wantVars: []interface{}{string(DecisionAllow)},
		},
		{
			name: "review final decision excludes pending",
			stmt: reviewFinalDecisionStatsQuery(db, DecisionBlock).
				Count(new(int64)).
				Statement,
			wantSQL:  []string{"review_cases", "status", "final_decision"},
			wantVars: []interface{}{string(ReviewStatusPending), string(DecisionBlock)},
		},
		{
			name: "pending review",
			stmt: reviewStatusStatsQuery(db, ReviewStatusPending).
				Count(new(int64)).
				Statement,
			wantSQL:  []string{"review_cases", "status"},
			wantVars: []interface{}{string(ReviewStatusPending)},
		},
		{
			name: "reviewed excludes pending",
			stmt: reviewedStatsQuery(db).
				Count(new(int64)).
				Statement,
			wantSQL:  []string{"review_cases", "status"},
			wantVars: []interface{}{string(ReviewStatusPending)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql := tt.stmt.SQL.String()
			for _, want := range tt.wantSQL {
				if !strings.Contains(sql, want) {
					t.Fatalf("SQL = %q, want %q", sql, want)
				}
			}
			if !reflect.DeepEqual(tt.stmt.Vars, tt.wantVars) {
				t.Fatalf("Vars = %#v, want %#v", tt.stmt.Vars, tt.wantVars)
			}
		})
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
