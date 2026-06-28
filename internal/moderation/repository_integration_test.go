//go:build integration

package moderation

import (
	"context"
	"os"
	"strings"
	"sync"
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

	storedCase, err := repository.GetReviewCase(ctx, reviewCase.ID)
	if err != nil {
		t.Fatalf("GetReviewCase() error = %v", err)
	}
	if storedCase.Case.ID != reviewCase.ID {
		t.Fatalf("review case id = %d, want %d", storedCase.Case.ID, reviewCase.ID)
	}
	if storedCase.Request.RequestID != requestID {
		t.Fatalf("stored request id = %q, want %q", storedCase.Request.RequestID, requestID)
	}
	if storedCase.Result.Decision != string(DecisionReview) {
		t.Fatalf("stored decision = %q, want review", storedCase.Result.Decision)
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

func TestGormRepositoryWebhookDeliveryClaimIntegration(t *testing.T) {
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
		&models.ClientApplication{},
		&models.WebhookDelivery{},
	); err != nil {
		t.Fatalf("auto migrate test database: %v", err)
	}

	repository := NewGormRepository(db)
	ctx := context.Background()
	user := createIntegrationUser(t, ctx, db, "webhook-claim")
	client := models.ClientApplication{
		UserID:        user.ID,
		Name:          "Webhook Claim Test",
		APIKeyHash:    uuid.New().String(),
		APIKeyPrefix:  "hs_test",
		Status:        "active",
		WebhookURL:    "https://example.com/moderation/webhook",
		WebhookSecret: "whsec_test",
	}
	if err := db.WithContext(ctx).Create(&client).Error; err != nil {
		t.Fatalf("create client application: %v", err)
	}
	otherClient := models.ClientApplication{
		UserID:        user.ID,
		Name:          "Webhook Claim Other Client",
		APIKeyHash:    uuid.New().String(),
		APIKeyPrefix:  "hs_other",
		Status:        "active",
		WebhookURL:    "https://example.com/moderation/webhook",
		WebhookSecret: "whsec_other",
	}
	if err := db.WithContext(ctx).Create(&otherClient).Error; err != nil {
		t.Fatalf("create other client application: %v", err)
	}

	requestID := uuid.New().String()
	otherRequestID := uuid.New().String()
	httpStatus := 500
	delivery := models.WebhookDelivery{
		DeliveryID:    uuid.New().String(),
		RequestID:     requestID,
		ClientID:      client.ID,
		Event:         "moderation.final_decision",
		Status:        string(WebhookDeliveryFailed),
		AttemptCount:  1,
		LastAttemptAt: time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC),
		HTTPStatus:    &httpStatus,
		ErrorMessage:  "webhook returned status 500",
		Payload:       `{"event":"moderation.final_decision","request_id":"` + requestID + `"}`,
	}
	if err := db.WithContext(ctx).Create(&delivery).Error; err != nil {
		t.Fatalf("create webhook delivery: %v", err)
	}
	sameClientOtherRequestDelivery := models.WebhookDelivery{
		DeliveryID:    uuid.New().String(),
		RequestID:     otherRequestID,
		ClientID:      client.ID,
		Event:         "moderation.final_decision",
		Status:        string(WebhookDeliveryFailed),
		AttemptCount:  1,
		LastAttemptAt: time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC),
		ErrorMessage:  "webhook returned status 500",
		Payload:       `{"event":"moderation.final_decision","request_id":"` + otherRequestID + `"}`,
	}
	if err := db.WithContext(ctx).Create(&sameClientOtherRequestDelivery).Error; err != nil {
		t.Fatalf("create same-client other-request webhook delivery: %v", err)
	}
	otherClientSameRequestDelivery := models.WebhookDelivery{
		DeliveryID:    uuid.New().String(),
		RequestID:     requestID,
		ClientID:      otherClient.ID,
		Event:         "moderation.final_decision",
		Status:        string(WebhookDeliveryFailed),
		AttemptCount:  1,
		LastAttemptAt: time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC),
		ErrorMessage:  "webhook returned status 500",
		Payload:       `{"event":"moderation.final_decision","request_id":"` + requestID + `"}`,
	}
	if err := db.WithContext(ctx).Create(&otherClientSameRequestDelivery).Error; err != nil {
		t.Fatalf("create other-client same-request webhook delivery: %v", err)
	}

	t.Cleanup(func() {
		db.Unscoped().
			Where("id IN ?", []uint{
				delivery.ID,
				sameClientOtherRequestDelivery.ID,
				otherClientSameRequestDelivery.ID,
			}).
			Delete(&models.WebhookDelivery{})
		db.Unscoped().Delete(&models.ClientApplication{}, otherClient.ID)
		db.Unscoped().Delete(&models.ClientApplication{}, client.ID)
		db.Unscoped().Delete(&models.User{}, user.ID)
	})

	listed, err := repository.ListWebhookDeliveries(ctx, WebhookDeliveryFilter{
		Status: WebhookDeliveryFailed,
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("ListWebhookDeliveries() error = %v", err)
	}
	if !containsWebhookDelivery(listed, delivery.ID) {
		t.Fatalf("failed webhook deliveries did not include delivery id %d", delivery.ID)
	}

	filtered, err := repository.ListWebhookDeliveries(ctx, WebhookDeliveryFilter{
		Status:    WebhookDeliveryFailed,
		ClientID:  &client.ID,
		RequestID: requestID,
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("ListWebhookDeliveries() filtered error = %v", err)
	}
	if len(filtered) != 1 || filtered[0].ID != delivery.ID {
		t.Fatalf("filtered deliveries = %#v, want delivery id %d", filtered, delivery.ID)
	}

	const workers = 8
	claimedAt := time.Date(2026, 6, 28, 12, 5, 0, 0, time.UTC)
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan webhookClaimResult, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			claimed, claimErr := repository.ClaimFailedWebhookDelivery(ctx, delivery.ID, claimedAt)
			results <- webhookClaimResult{
				delivery: claimed,
				err:      claimErr,
			}
		}()
	}

	close(start)
	wg.Wait()
	close(results)

	successes := 0
	conflicts := 0
	var claimed models.WebhookDelivery
	for result := range results {
		if result.err == nil {
			successes++
			claimed = result.delivery
			continue
		}
		if strings.Contains(result.err.Error(), "Webhook delivery is not failed") {
			conflicts++
			continue
		}
		t.Fatalf("ClaimFailedWebhookDelivery() unexpected error = %v", result.err)
	}
	if successes != 1 {
		t.Fatalf("successful claims = %d, want 1", successes)
	}
	if conflicts != workers-1 {
		t.Fatalf("conflicting claims = %d, want %d", conflicts, workers-1)
	}
	if claimed.Status != string(WebhookDeliveryRetrying) {
		t.Fatalf("claimed status = %q, want retrying", claimed.Status)
	}
	if claimed.HTTPStatus != nil {
		t.Fatalf("claimed HTTPStatus = %#v, want nil", claimed.HTTPStatus)
	}
	if claimed.ErrorMessage != "" {
		t.Fatalf("claimed ErrorMessage = %q, want empty", claimed.ErrorMessage)
	}

	successStatus := 204
	updatedAt := claimedAt.Add(time.Minute)
	updated, err := repository.UpdateWebhookDeliveryAttempt(
		ctx,
		delivery.ID,
		WebhookDeliverySucceeded,
		&successStatus,
		"",
		updatedAt,
	)
	if err != nil {
		t.Fatalf("UpdateWebhookDeliveryAttempt() error = %v", err)
	}
	if updated.Status != string(WebhookDeliverySucceeded) {
		t.Fatalf("updated status = %q, want succeeded", updated.Status)
	}
	if updated.AttemptCount != 2 {
		t.Fatalf("updated attempt count = %d, want 2", updated.AttemptCount)
	}
	if updated.HTTPStatus == nil || *updated.HTTPStatus != successStatus {
		t.Fatalf("updated HTTPStatus = %#v, want %d", updated.HTTPStatus, successStatus)
	}
	if updated.ErrorMessage != "" {
		t.Fatalf("updated ErrorMessage = %q, want empty", updated.ErrorMessage)
	}

	staleDelivery := models.WebhookDelivery{
		DeliveryID:    uuid.New().String(),
		RequestID:     uuid.New().String(),
		ClientID:      client.ID,
		Event:         "moderation.final_decision",
		Status:        string(WebhookDeliveryRetrying),
		AttemptCount:  1,
		LastAttemptAt: claimedAt.Add(-webhookRetryLease - time.Second),
		ErrorMessage:  "retry interrupted",
		Payload:       `{"event":"moderation.final_decision"}`,
	}
	if err := db.WithContext(ctx).Create(&staleDelivery).Error; err != nil {
		t.Fatalf("create stale webhook delivery: %v", err)
	}
	reclaimed, err := repository.ClaimFailedWebhookDelivery(ctx, staleDelivery.ID, claimedAt)
	if err != nil {
		t.Fatalf("ClaimFailedWebhookDelivery() stale retrying error = %v", err)
	}
	if reclaimed.Status != string(WebhookDeliveryRetrying) {
		t.Fatalf("reclaimed status = %q, want retrying", reclaimed.Status)
	}
	if reclaimed.ErrorMessage != "" {
		t.Fatalf("reclaimed ErrorMessage = %q, want empty", reclaimed.ErrorMessage)
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

func TestGormRepositoryListHistoryIntegration(t *testing.T) {
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
		&models.ClientApplication{},
		&models.ModerationRequest{},
		&models.ModerationResult{},
		&models.ReviewCase{},
	); err != nil {
		t.Fatalf("auto migrate test database: %v", err)
	}

	repository := NewGormRepository(db)
	ctx := context.Background()
	prefix := uuid.New().String()
	user := createIntegrationUser(t, ctx, db, "history")
	client := models.ClientApplication{
		UserID:       user.ID,
		Name:         "History Test",
		APIKeyHash:   uuid.New().String(),
		APIKeyPrefix: "hs_hist",
		Status:       "active",
	}
	if err := db.WithContext(ctx).Create(&client).Error; err != nil {
		t.Fatalf("create client application: %v", err)
	}

	t.Cleanup(func() {
		db.Unscoped().Where("request_id LIKE ?", prefix+"%").Delete(&models.ReviewCase{})
		db.Unscoped().Where("request_id LIKE ?", prefix+"%").Delete(&models.ModerationResult{})
		db.Unscoped().Where("request_id LIKE ?", prefix+"%").Delete(&models.ModerationRequest{})
		db.Unscoped().Delete(&models.ClientApplication{}, client.ID)
		db.Unscoped().Delete(&models.User{}, user.ID)
	})

	matchingRequestID := prefix + "-matching"
	filteredRequestID := prefix + "-filtered"
	matchingRequest := &models.ModerationRequest{
		RequestID:  matchingRequestID,
		UserID:     user.ID,
		ClientID:   &client.ID,
		Content:    "history review content",
		Source:     "comment",
		ExternalID: "comment_123",
		ActorID:    "user_456",
		Status:     "completed",
	}
	matchingResult := &models.ModerationResult{
		RequestID:     matchingRequestID,
		UserID:        user.ID,
		ClientID:      &client.ID,
		Provider:      "test-provider",
		Model:         "test-model",
		RawOutput:     `{"risk_score":0.6}`,
		RiskScore:     0.6,
		Labels:        `["harassment"]`,
		Decision:      string(DecisionReview),
		Reason:        "Needs operator review.",
		PolicyVersion: "default-v1",
	}
	reviewerID := uint(99)
	reviewCase := &models.ReviewCase{
		RequestID:     matchingRequestID,
		UserID:        user.ID,
		ClientID:      &client.ID,
		Status:        string(ReviewStatusApproved),
		ReviewerID:    &reviewerID,
		FinalDecision: string(DecisionAllow),
	}
	if err := db.WithContext(ctx).Create(matchingRequest).Error; err != nil {
		t.Fatalf("create matching request: %v", err)
	}
	if err := db.WithContext(ctx).Create(matchingResult).Error; err != nil {
		t.Fatalf("create matching result: %v", err)
	}
	if err := db.WithContext(ctx).Create(reviewCase).Error; err != nil {
		t.Fatalf("create matching review case: %v", err)
	}

	filteredRequest := &models.ModerationRequest{
		RequestID:  filteredRequestID,
		UserID:     user.ID,
		ClientID:   &client.ID,
		Content:    "history block content",
		Source:     "comment",
		ExternalID: "comment_999",
		Status:     "completed",
	}
	filteredResult := &models.ModerationResult{
		RequestID:     filteredRequestID,
		UserID:        user.ID,
		ClientID:      &client.ID,
		Provider:      "test-provider",
		Model:         "test-model",
		RiskScore:     0.9,
		Labels:        `["hate"]`,
		Decision:      string(DecisionBlock),
		Reason:        "Policy threshold exceeded.",
		PolicyVersion: "default-v1",
	}
	if err := db.WithContext(ctx).Create(filteredRequest).Error; err != nil {
		t.Fatalf("create filtered request: %v", err)
	}
	if err := db.WithContext(ctx).Create(filteredResult).Error; err != nil {
		t.Fatalf("create filtered result: %v", err)
	}

	items, err := repository.ListHistory(ctx, HistoryFilter{
		Decision:   DecisionReview,
		ClientID:   &client.ID,
		ExternalID: "comment_123",
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("ListHistory() error = %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("history item count = %d, want 1", len(items))
	}
	item := items[0]
	if item.Request.RequestID != matchingRequestID {
		t.Fatalf("RequestID = %q, want %q", item.Request.RequestID, matchingRequestID)
	}
	if item.Request.Content != "history review content" {
		t.Fatalf("Content = %q, want history review content", item.Request.Content)
	}
	if item.Result.Decision != string(DecisionReview) {
		t.Fatalf("Decision = %q, want review", item.Result.Decision)
	}
	if item.Result.Labels != `["harassment"]` {
		t.Fatalf("Labels = %q, want harassment", item.Result.Labels)
	}
	if item.ReviewCase == nil {
		t.Fatal("ReviewCase = nil, want approved review case")
	}
	if item.ReviewCase.Status != string(ReviewStatusApproved) {
		t.Fatalf("ReviewCase.Status = %q, want approved", item.ReviewCase.Status)
	}
	if item.ReviewCase.FinalDecision != string(DecisionAllow) {
		t.Fatalf("ReviewCase.FinalDecision = %q, want allow", item.ReviewCase.FinalDecision)
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

func containsWebhookDelivery(deliveries []models.WebhookDelivery, id uint) bool {
	for _, delivery := range deliveries {
		if delivery.ID == id {
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

type webhookClaimResult struct {
	delivery models.WebhookDelivery
	err      error
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
		APIKey:   "it_" + id,
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
