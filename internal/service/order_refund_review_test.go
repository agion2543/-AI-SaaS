package service

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

func TestRefundReviewThresholdDefaultAndSet(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.SystemConfig{}, &model.AuditLog{}); err != nil {
		t.Fatalf("migrate system config table: %v", err)
	}
	admin := NewAdminService(&config.Config{}, db)

	got, err := admin.GetRefundReviewThreshold()
	if err != nil {
		t.Fatalf("default threshold read: %v", err)
	}
	if got != RefundReviewThresholdDefaultCents {
		t.Fatalf("default threshold=%d, want %d cents (1000 yuan)", got, RefundReviewThresholdDefaultCents)
	}

	if err := admin.SetRefundReviewThreshold(50000, AuditActor{ID: 1, Type: "super_admin"}); err != nil {
		t.Fatalf("set threshold: %v", err)
	}
	got, err = admin.GetRefundReviewThreshold()
	if err != nil || got != 50000 {
		t.Fatalf("threshold after set=%d err=%v, want 50000", got, err)
	}
	for _, bad := range []int64{-1, RefundReviewThresholdMaxCents + 1} {
		if err := admin.SetRefundReviewThreshold(bad, AuditActor{ID: 1, Type: "super_admin"}); err == nil {
			t.Fatalf("threshold %d must be rejected", bad)
		}
	}
	if got, err := admin.GetRefundReviewThreshold(); err != nil || got != 50000 {
		t.Fatalf("rejected write must not change threshold, got %d err=%v", got, err)
	}
}

func TestRefundAboveThresholdWaitsForReview(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.AuditLog{}, &model.AdminUser{}, &model.SystemConfig{}); err != nil {
		t.Fatalf("migrate audit tables: %v", err)
	}
	actor := createAuditTestActor(t, db)
	paidAt := time.Now().UTC()
	merchant, _, _, order := createSyntheticStoreOrder(t, db, "SYNTHETIC-REFUND-REVIEW-001", "accepted", 200000, &paidAt)
	order.TransactionNo = "synthetic-paid-transaction"
	if err := db.Save(&order).Error; err != nil {
		t.Fatal(err)
	}
	providerCalls := 0
	orderService := NewOrderService(db, newLocalAlipayRefundService(t, &providerCalls))
	event := newAuditTestEvent(actor, "merchant_order_refund", map[string]interface{}{"reason": "大额退款复核"})

	// 200000 cents (2000 yuan) > default threshold 100000 cents.
	returned, pendingReview, err := orderService.RefundMerchantOrder(merchant.ID, order.ID, dto.RefundOrderRequest{Full: true, Reason: "大额退款复核"}, 42, "merchant", event)
	if err != nil {
		t.Fatalf("above-threshold refund apply: %v", err)
	}
	if !pendingReview {
		t.Fatalf("above-threshold refund must report pending review, got pendingReview=%v", pendingReview)
	}
	if providerCalls != 0 {
		t.Fatalf("pending-review refund must not call the provider, calls=%d", providerCalls)
	}
	if returned.RefundStatus != "processing" {
		t.Fatalf("pending-review order refund status=%q, want processing (reservation held)", returned.RefundStatus)
	}
	var record model.RefundRecord
	if err := db.Where("order_id = ?", order.ID).First(&record).Error; err != nil {
		t.Fatal(err)
	}
	if record.Status != "pending_review" || record.Amount != 200000 || record.OperatorID != 42 || record.OperatorRole != "merchant" {
		t.Fatalf("pending-review reservation mismatch: %+v", record)
	}
	// A second refund while the first waits for review must be blocked by
	// the reserved amount, not dispatched.
	if _, _, err := orderService.RefundMerchantOrder(merchant.ID, order.ID, dto.RefundOrderRequest{Amount: 1, Reason: "重复退款"}, 42, "merchant"); err == nil || !strings.Contains(err.Error(), "no refundable amount") {
		t.Fatalf("second refund during pending review must be blocked by reserved amount, got %v", err)
	}
	if providerCalls != 0 {
		t.Fatalf("blocked second refund must not call the provider, calls=%d", providerCalls)
	}
	pending, err := orderService.PendingReviewRefunds()
	if err != nil {
		t.Fatalf("list pending review: %v", err)
	}
	if len(pending) != 1 || pending[0].ID != record.ID {
		t.Fatalf("pending list=%d, want the one pending refund", len(pending))
	}
	if countAuditLogs(t, db) != 1 {
		t.Fatalf("apply step must write exactly one audit row, got %d", countAuditLogs(t, db))
	}
	var logs []OrderOperationLog
	if err := json.Unmarshal([]byte(returned.OperationLogs), &logs); err != nil {
		t.Fatalf("decode order audit log: %v", err)
	}
	if len(logs) != 1 || logs[0].Action != "refund_pending_review" {
		t.Fatalf("order log must record the pending-review application: %+v", logs)
	}
}

func TestRefundReviewRejectReleasesReservation(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.AuditLog{}, &model.AdminUser{}, &model.SystemConfig{}); err != nil {
		t.Fatalf("migrate audit tables: %v", err)
	}
	actor := createAuditTestActor(t, db)
	paidAt := time.Now().UTC()
	merchant, _, _, order := createSyntheticStoreOrder(t, db, "SYNTHETIC-REFUND-REVIEW-002", "accepted", 200000, &paidAt)
	order.TransactionNo = "synthetic-paid-transaction"
	if err := db.Save(&order).Error; err != nil {
		t.Fatal(err)
	}
	providerCalls := 0
	orderService := NewOrderService(db, newLocalAlipayRefundService(t, &providerCalls))
	applyEvent := newAuditTestEvent(actor, "merchant_order_refund", map[string]interface{}{"reason": "大额退款复核"})
	if _, pending, err := orderService.RefundMerchantOrder(merchant.ID, order.ID, dto.RefundOrderRequest{Full: true, Reason: "大额退款复核", IdempotencyKey: "synthetic-review-apply-002"}, 42, "merchant", applyEvent); err != nil || !pending {
		t.Fatalf("apply above-threshold refund: pending=%v err=%v", pending, err)
	}
	var record model.RefundRecord
	if err := db.Where("order_id = ?", order.ID).First(&record).Error; err != nil {
		t.Fatal(err)
	}

	// Rejecting without a reason is refused.
	reviewEvent := newAuditTestEvent(actor, "admin_order_refund_review", map[string]interface{}{"approve": false})
	if _, err := orderService.ReviewRefund(record.ID, false, "", 1, "super_admin", reviewEvent); err == nil || !strings.Contains(err.Error(), "requires a reason") {
		t.Fatalf("rejection without reason must fail, got %v", err)
	}

	rejected, err := orderService.ReviewRefund(record.ID, false, "金额与订单不符", 1, "super_admin", reviewEvent)
	if err != nil {
		t.Fatalf("reject review: %v", err)
	}
	if rejected.RefundStatus != "none" || rejected.RefundedAmount != 0 {
		t.Fatalf("rejected review must release the reservation: %+v", rejected)
	}
	if err := db.First(&record, record.ID).Error; err != nil {
		t.Fatal(err)
	}
	if record.Status != "review_rejected" || record.ReviewerID != 1 || record.ReviewerRole != "super_admin" || record.ReviewReason != "金额与订单不符" || record.ReviewedAt == nil {
		t.Fatalf("rejected record mismatch: %+v", record)
	}
	if providerCalls != 0 {
		t.Fatalf("rejected refund must never call the provider, calls=%d", providerCalls)
	}
	// A rejected refund is definitive: reviewing it again fails, and
	// replaying the same idempotency key reports the rejection instead of
	// dispatching again; a retry needs a new idempotency key.
	if _, err := orderService.ReviewRefund(record.ID, true, "", 2, "super_admin"); err == nil || !strings.Contains(err.Error(), "not pending review") {
		t.Fatalf("reviewing a rejected refund must fail, got %v", err)
	}
	if _, _, err := orderService.RefundMerchantOrder(merchant.ID, order.ID, dto.RefundOrderRequest{Full: true, Reason: "大额退款复核", IdempotencyKey: "synthetic-review-apply-002"}, 42, "merchant"); err == nil || !strings.Contains(err.Error(), "was rejected during review") {
		t.Fatalf("replaying the rejected refund must fail, got %v", err)
	}
	// A fresh small refund still works after the rejection released the
	// reservation.
	if _, pending, err := orderService.RefundMerchantOrder(merchant.ID, order.ID, dto.RefundOrderRequest{Amount: 300, Reason: "小额退款", IdempotencyKey: "synthetic-review-retry-001"}, 42, "merchant"); err != nil || pending {
		t.Fatalf("small refund after rejection: pending=%v err=%v", pending, err)
	}
	if providerCalls != 1 {
		t.Fatalf("small refund after rejection must dispatch once, calls=%d", providerCalls)
	}
	if countAuditLogs(t, db) != 2 {
		t.Fatalf("apply + review must write two audit rows, got %d", countAuditLogs(t, db))
	}
}

func TestRefundReviewApproveDispatchesAndSelfReviewBlocked(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.AuditLog{}, &model.AdminUser{}, &model.SystemConfig{}); err != nil {
		t.Fatalf("migrate audit tables: %v", err)
	}
	actor := createAuditTestActor(t, db)
	paidAt := time.Now().UTC()
	_, _, _, order := createSyntheticStoreOrder(t, db, "SYNTHETIC-REFUND-REVIEW-003", "accepted", 200000, &paidAt)
	order.TransactionNo = "synthetic-paid-transaction"
	if err := db.Save(&order).Error; err != nil {
		t.Fatal(err)
	}
	providerCalls := 0
	orderService := NewOrderService(db, newLocalAlipayRefundService(t, &providerCalls))
	// The refund is applied by super-admin 7.
	applyEvent := newAuditTestEvent(actor, "admin_order_refund", map[string]interface{}{"reason": "大额退款复核"})
	if _, pending, err := orderService.RefundAdminOrder(order.ID, dto.RefundOrderRequest{Full: true, Reason: "大额退款复核"}, 7, applyEvent); err != nil || !pending {
		t.Fatalf("apply above-threshold refund as admin: pending=%v err=%v", pending, err)
	}
	var record model.RefundRecord
	if err := db.Where("order_id = ?", order.ID).First(&record).Error; err != nil {
		t.Fatal(err)
	}
	if record.OperatorID != 7 || record.OperatorRole != "admin" {
		t.Fatalf("admin-applied refund must record the effective role from the audit actor, got %+v", record)
	}

	reviewEvent := newAuditTestEvent(actor, "admin_order_refund_review", map[string]interface{}{"approve": true})
	// Same account, same label: blocked.
	if _, err := orderService.ReviewRefund(record.ID, true, "", 7, "super_admin", reviewEvent); err == nil || !strings.Contains(err.Error(), "cannot be the refund applicant") {
		t.Fatalf("self-review must be blocked, got %v", err)
	}
	// Same account, legacy "admin" label: still blocked (same admin family).
	if _, err := orderService.ReviewRefund(record.ID, true, "", 7, "admin", reviewEvent); err == nil || !strings.Contains(err.Error(), "cannot be the refund applicant") {
		t.Fatalf("self-review under the legacy admin label must be blocked, got %v", err)
	}
	if providerCalls != 0 {
		t.Fatalf("blocked reviews must not dispatch, calls=%d", providerCalls)
	}

	approved, err := orderService.ReviewRefund(record.ID, true, "", 8, "super_admin", reviewEvent)
	if err != nil {
		t.Fatalf("approve review by another super-admin: %v", err)
	}
	if approved.RefundedAmount != 200000 || approved.RefundStatus != "full" {
		t.Fatalf("approved refund state mismatch: %+v", approved)
	}
	if err := db.First(&record, record.ID).Error; err != nil {
		t.Fatal(err)
	}
	if record.Status != "success" || record.ReviewerID != 8 || record.ReviewerRole != "super_admin" || record.ReviewedAt == nil {
		t.Fatalf("approved record mismatch: %+v", record)
	}
	if providerCalls != 1 {
		t.Fatalf("approved refund must dispatch exactly once, calls=%d", providerCalls)
	}
	if countAuditLogs(t, db) != 2 {
		t.Fatalf("apply + review must write two audit rows, got %d", countAuditLogs(t, db))
	}
}

func TestRefundBelowThresholdDispatchesImmediately(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	paidAt := time.Now().UTC()
	merchant, _, _, order := createSyntheticStoreOrder(t, db, "SYNTHETIC-REFUND-REVIEW-004", "accepted", 1200, &paidAt)
	order.TransactionNo = "synthetic-paid-transaction"
	if err := db.Save(&order).Error; err != nil {
		t.Fatal(err)
	}
	providerCalls := 0
	orderService := NewOrderService(db, newLocalAlipayRefundService(t, &providerCalls))

	// 300 cents is far below the default 100000-cent threshold.
	returned, pendingReview, err := orderService.RefundMerchantOrder(merchant.ID, order.ID, dto.RefundOrderRequest{Amount: 300, Reason: "小额退款"}, 42, "merchant")
	if err != nil || pendingReview {
		t.Fatalf("below-threshold refund must dispatch immediately: pending=%v err=%v", pendingReview, err)
	}
	if returned.RefundedAmount != 300 || returned.RefundStatus != "partial" {
		t.Fatalf("below-threshold refund state mismatch: %+v", returned)
	}
	if providerCalls != 1 {
		t.Fatalf("below-threshold refund must call the provider once, calls=%d", providerCalls)
	}
	pending, err := orderService.PendingReviewRefunds()
	if err != nil {
		t.Fatalf("list pending review: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("no refunds should be pending, got %d", len(pending))
	}
}
