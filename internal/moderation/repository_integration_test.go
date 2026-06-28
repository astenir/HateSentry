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
	user := createIntegrationUser(t, ctx, db, "review-workflow")

	t.Cleanup(func() {
		db.Unscoped().Where("request_id = ?", requestID).Delete(&models.ReviewCase{})
		db.Unscoped().Where("request_id = ?", requestID).Delete(&models.ModerationResult{})
		db.Unscoped().Where("request_id = ?", requestID).Delete(&models.ModerationRequest{})
		db.Unscoped().Delete(&models.User{}, user.ID)
	})

	request := &models.ModerationRequest{
		RequestID: requestID,
		UserID:    user.ID,
		Content:   "needs review",
		Source:    "comment",
		Status:    "completed",
	}
	result := &models.ModerationResult{
		RequestID:     requestID,
		UserID:        user.ID,
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
		UserID:    user.ID,
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
	if finalized.Case.UserID != user.ID {
		t.Fatalf("finalized user id = %d, want submitter id %d", finalized.Case.UserID, user.ID)
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

func TestGormRepositoryGetStatsIntegration(t *testing.T) {
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
	ctx := context.Background()
	prefix := uuid.New().String()
	user := createIntegrationUser(t, ctx, db, "stats")

	t.Cleanup(func() {
		db.Unscoped().Where("request_id LIKE ?", prefix+"%").Delete(&models.ReviewCase{})
		db.Unscoped().Where("request_id LIKE ?", prefix+"%").Delete(&models.ModerationResult{})
		db.Unscoped().Where("request_id LIKE ?", prefix+"%").Delete(&models.ModerationRequest{})
		db.Unscoped().Delete(&models.User{}, user.ID)
	})

	baseline, err := repository.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats() baseline error = %v", err)
	}

	seeds := []statsSeed{
		{
			suffix:   "auto-allow",
			decision: DecisionAllow,
		},
		{
			suffix:   "auto-block",
			decision: DecisionBlock,
		},
		{
			suffix:       "pending-review",
			decision:     DecisionReview,
			reviewStatus: ReviewStatusPending,
		},
		{
			suffix:        "approved-review",
			decision:      DecisionReview,
			reviewStatus:  ReviewStatusApproved,
			finalDecision: DecisionAllow,
		},
		{
			suffix:        "rejected-review",
			decision:      DecisionReview,
			reviewStatus:  ReviewStatusRejected,
			finalDecision: DecisionBlock,
		},
		{
			suffix:        "mistake-review",
			decision:      DecisionReview,
			reviewStatus:  ReviewStatusMistake,
			finalDecision: DecisionAllow,
		},
		{
			suffix:       "soft-deleted-block",
			decision:     DecisionBlock,
			softDeleted:  true,
			reviewStatus: ReviewStatusPending,
		},
	}

	for _, seed := range seeds {
		if err := seedStatsRecord(ctx, db, prefix, user.ID, seed); err != nil {
			t.Fatalf("seed stats record %q: %v", seed.suffix, err)
		}
	}

	stats, err := repository.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}

	assertStatsDelta(t, "TotalModerated", stats.TotalModerated, baseline.TotalModerated, 6)
	assertStatsDelta(t, "PolicyAllowed", stats.PolicyAllowed, baseline.PolicyAllowed, 1)
	assertStatsDelta(t, "PolicyBlocked", stats.PolicyBlocked, baseline.PolicyBlocked, 1)
	assertStatsDelta(t, "ReviewFinalAllowed", stats.ReviewFinalAllowed, baseline.ReviewFinalAllowed, 2)
	assertStatsDelta(t, "ReviewFinalBlocked", stats.ReviewFinalBlocked, baseline.ReviewFinalBlocked, 1)
	assertStatsDelta(t, "PendingReview", stats.PendingReview, baseline.PendingReview, 1)
	assertStatsDelta(t, "Reviewed", stats.Reviewed, baseline.Reviewed, 3)
	assertStatsDelta(t, "Mistakes", stats.Mistakes, baseline.Mistakes, 1)
}

func containsReviewCase(cases []StoredReviewCase, requestID string) bool {
	for _, reviewCase := range cases {
		if reviewCase.Case.RequestID == requestID {
			return true
		}
	}

	return false
}

type statsSeed struct {
	suffix        string
	decision      Decision
	reviewStatus  ReviewStatus
	finalDecision Decision
	softDeleted   bool
}

func createIntegrationUser(t *testing.T, ctx context.Context, db *gorm.DB, suffix string) models.User {
	t.Helper()

	id := strings.ReplaceAll(uuid.New().String(), "-", "")[:12]
	user := models.User{
		Username: "it-" + suffix + "-" + id,
		Email:    "it-" + suffix + "-" + id + "@example.test",
		Password: "not-used",
		Role:     "user",
		Status:   "active",
	}
	if err := db.WithContext(ctx).Create(&user).Error; err != nil {
		t.Fatalf("create integration user: %v", err)
	}

	return user
}

func seedStatsRecord(ctx context.Context, db *gorm.DB, prefix string, userID uint, seed statsSeed) error {
	requestID := prefix + "-" + seed.suffix
	request := &models.ModerationRequest{
		RequestID: requestID,
		UserID:    userID,
		Content:   "stats fixture " + seed.suffix,
		Source:    "comment",
		Status:    "completed",
	}
	result := &models.ModerationResult{
		RequestID:     requestID,
		UserID:        userID,
		Provider:      "test-provider",
		Model:         "test-model",
		RiskScore:     0.8,
		Labels:        `["harassment"]`,
		Decision:      string(seed.decision),
		Reason:        "Stats fixture.",
		PolicyVersion: "default-v1",
	}

	if err := db.WithContext(ctx).Create(request).Error; err != nil {
		return err
	}
	if err := db.WithContext(ctx).Create(result).Error; err != nil {
		return err
	}

	var reviewCase *models.ReviewCase
	if seed.reviewStatus != "" {
		reviewCase = &models.ReviewCase{
			RequestID: requestID,
			UserID:    userID,
			Status:    string(seed.reviewStatus),
		}
		if seed.reviewStatus != ReviewStatusPending {
			reviewerID := uint(99)
			reviewedAt := time.Date(2026, 6, 28, 13, 0, 0, 0, time.UTC)
			reviewCase.ReviewerID = &reviewerID
			reviewCase.FinalDecision = string(seed.finalDecision)
			reviewCase.ReviewNotes = "stats fixture final decision"
			reviewCase.ReviewedAt = &reviewedAt
		}
		if err := db.WithContext(ctx).Create(reviewCase).Error; err != nil {
			return err
		}
	}

	if seed.softDeleted {
		if reviewCase != nil {
			if err := db.WithContext(ctx).Delete(reviewCase).Error; err != nil {
				return err
			}
		}
		if err := db.WithContext(ctx).Delete(result).Error; err != nil {
			return err
		}
		if err := db.WithContext(ctx).Delete(request).Error; err != nil {
			return err
		}
	}

	return nil
}

func assertStatsDelta(t *testing.T, name string, got, baseline, wantDelta int64) {
	t.Helper()

	if got-baseline != wantDelta {
		t.Fatalf("%s delta = %d, want %d (got %d, baseline %d)", name, got-baseline, wantDelta, got, baseline)
	}
}
