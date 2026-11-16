package service

import (
	"testing"
	"time"

	"go-web-gin-health/internal/model"
)

func TestMarkReviewedOnlyUpdatesPendingReviewMetadataAndPreservesFirstReviewer(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.AuditLog{}); err != nil {
		t.Fatalf("migrate synthetic audit table: %v", err)
	}

	createdAt := time.Date(2026, time.January, 12, 9, 30, 0, 123456789, time.UTC)
	updatedAt := time.Date(2026, time.January, 12, 9, 45, 0, 987654321, time.UTC)
	merchantID := uint(27)
	original := model.AuditLog{
		BaseModel:    model.BaseModel{CreatedAt: createdAt, UpdatedAt: updatedAt},
		ActorID:      41,
		ActorType:    "admin",
		ActorName:    "synthetic actor",
		Action:       "merchant_order_refund",
		TargetType:   "order",
		TargetID:     81,
		TargetName:   "synthetic order",
		MerchantID:   &merchantID,
		IP:           "192.0.2.44",
		UserAgent:    "synthetic-agent",
		Detail:       `{"purpose":"synthetic audit event"}`,
		ReviewStatus: "pending",
		ReviewRemark: "existing synthetic remark",
	}
	if err := db.Create(&original).Error; err != nil {
		t.Fatalf("create synthetic audit event: %v", err)
	}

	service := NewAuditService(db)
	firstReviewerID := uint(52)
	first, err := service.MarkReviewed(original.ID, firstReviewerID, "ignored first remark")
	if err != nil {
		t.Fatalf("mark synthetic audit event reviewed: %v", err)
	}
	assertAuditEventFieldsUnchanged(t, first, &original)
	if first.ReviewStatus != "reviewed" || first.ReviewedBy == nil || *first.ReviewedBy != firstReviewerID || first.ReviewedAt == nil {
		t.Fatalf("first review metadata was not stored: %+v", first)
	}
	firstReviewedAt := *first.ReviewedAt

	second, err := service.MarkReviewed(original.ID, 73, "ignored second remark")
	if err != nil {
		t.Fatalf("repeat review of synthetic audit event: %v", err)
	}
	assertAuditEventFieldsUnchanged(t, second, &original)
	if second.ReviewStatus != "reviewed" || second.ReviewedBy == nil || *second.ReviewedBy != firstReviewerID {
		t.Fatalf("repeat review overwrote the first reviewer: %+v", second)
	}
	if second.ReviewedAt == nil || !second.ReviewedAt.Equal(firstReviewedAt) {
		t.Fatalf("repeat review overwrote the first review time: first=%v second=%v", firstReviewedAt, second.ReviewedAt)
	}
}

func assertAuditEventFieldsUnchanged(t *testing.T, got, want *model.AuditLog) {
	t.Helper()
	if got.ID != want.ID || got.ActorID != want.ActorID || got.ActorType != want.ActorType || got.ActorName != want.ActorName ||
		got.Action != want.Action || got.TargetType != want.TargetType || got.TargetID != want.TargetID || got.TargetName != want.TargetName ||
		got.IP != want.IP || got.UserAgent != want.UserAgent || got.Detail != want.Detail || got.ReviewRemark != want.ReviewRemark ||
		!got.CreatedAt.Equal(want.CreatedAt) || !got.UpdatedAt.Equal(want.UpdatedAt) {
		t.Errorf("review changed immutable audit event fields: got=%+v want=%+v", got, want)
	}
	if got.MerchantID == nil || want.MerchantID == nil || *got.MerchantID != *want.MerchantID {
		t.Errorf("review changed audit merchant reference: got=%v want=%v", got.MerchantID, want.MerchantID)
	}
}
