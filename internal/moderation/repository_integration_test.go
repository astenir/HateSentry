//go:build integration

package moderation

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"hatesentry/internal/models"

	"github.com/google/uuid"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestGormRepositoryReviewWorkflowIntegration(t *testing.T) {
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
		&models.ModerationRequest{},
		&models.ModerationResult{},
		&models.ReviewCase{},
	); err != nil {
		t.Fatalf("auto migrate test database: %v", err)
	}

	repository := NewGormRepository(db)
	requestID := uuid.New().String()
	ctx := context.Background()

	t.Cleanup(func() {
		db.Unscoped().Where("request_id = ?", requestID).Delete(&models.ReviewCase{})
		db.Unscoped().Where("request_id = ?", requestID).Delete(&models.ModerationResult{})
		db.Unscoped().Where("request_id = ?", requestID).Delete(&models.ModerationRequest{})
	})

	request := &models.ModerationRequest{
		RequestID: requestID,
		UserID:    7,
		Content:   "needs review",
		Source:    "comment",
		Status:    "completed",
	}
	result := &models.ModerationResult{
		RequestID:     requestID,
		UserID:        7,
		Provider:      "test-provider",
		Model:         "test-model",
		RiskScore:     0.6,
		Labels:        `["harassment"]`,
		Decision:      string(DecisionReview),
		Reason:        "Needs operator review.",
		PolicyVersion: "default-v1",
	}
	reviewCase := &models.ReviewCase{
		RequestID: requestID,
		UserID:    7,
		Status:    string(ReviewStatusPending),
	}

	if err := repository.SaveCheck(ctx, request, result, reviewCase); err != nil {
		t.Fatalf("SaveCheck() error = %v", err)
	}
	if reviewCase.ID == 0 {
		t.Fatal("review case id was not populated")
	}

	cases, err := repository.ListReviewCases(ctx, ReviewStatusPending)
	if err != nil {
		t.Fatalf("ListReviewCases() error = %v", err)
	}
	if !containsReviewCase(cases, requestID) {
		t.Fatalf("pending review cases did not include request %q", requestID)
	}

	reviewedAt := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	finalized, err := repository.FinalizeReviewCase(
		ctx,
		reviewCase.ID,
		99,
		ReviewStatusApproved,
		DecisionAllow,
		"looks safe",
		reviewedAt,
	)
	if err != nil {
		t.Fatalf("FinalizeReviewCase() error = %v", err)
	}
	if finalized.Case.UserID != 7 {
		t.Fatalf("finalized user id = %d, want submitter id 7", finalized.Case.UserID)
	}
	if finalized.Case.ReviewerID == nil || *finalized.Case.ReviewerID != 99 {
		t.Fatalf("reviewer id = %#v, want 99", finalized.Case.ReviewerID)
	}
	if finalized.Case.Status != string(ReviewStatusApproved) {
		t.Fatalf("status = %q, want approved", finalized.Case.Status)
	}
	if finalized.Case.FinalDecision != string(DecisionAllow) {
		t.Fatalf("final decision = %q, want allow", finalized.Case.FinalDecision)
	}

	_, err = repository.FinalizeReviewCase(
		ctx,
		reviewCase.ID,
		99,
		ReviewStatusRejected,
		DecisionBlock,
		"second decision",
		reviewedAt,
	)
	if err == nil {
		t.Fatal("FinalizeReviewCase() second call error = nil, want conflict")
	}
	if !strings.Contains(err.Error(), "Review case is already finalized") {
		t.Fatalf("FinalizeReviewCase() second call error = %q, want already finalized", err.Error())
	}
}

func containsReviewCase(cases []StoredReviewCase, requestID string) bool {
	for _, reviewCase := range cases {
		if reviewCase.Case.RequestID == requestID {
			return true
		}
	}

	return false
}
