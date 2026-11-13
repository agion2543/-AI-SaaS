package service

import (
	"strings"
	"testing"
	"time"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
	"gorm.io/gorm"
)

func createUnknownRefundReservation(t *testing.T, db *gorm.DB, order model.Order, refundNo string, amount int64) model.RefundRecord {
	t.Helper()
	key := "synthetic-idem-" + refundNo
	refund := model.RefundRecord{
		OrderID:        order.ID,
		MerchantID:     order.MerchantID,
		StoreID:        order.StoreID,
		RefundNo:       refundNo,
		IdempotencyKey: &key,
		Amount:         amount,
		Reason:         "合成测试退款原因",
		OperatorRole:   "merchant",
		OperatorID:     42,
		Status:         "unknown",
		RawPayload:     "synthetic gateway timeout; provider outcome unknown",
	}
	if err := db.Create(&refund).Error; err != nil {
		t.Fatalf("create synthetic unknown refund: %v", err)
	}
	if err := db.Model(&model.Order{}).Where("id = ?", order.ID).Update("refund_status", "processing").Error; err != nil {
		t.Fatalf("mark synthetic refund reservation: %v", err)
	}
	return refund
}

func newRefundReconciliationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.RefundReconciliationAudit{}); err != nil {
		t.Fatalf("migrate reconciliation audit test schema: %v", err)
	}
	return db
}

func TestRefundReconciliationRequiresProviderEvidenceAndReason(t *testing.T) {
	db := newRefundReconciliationTestDB(t)
	paidAt := time.Now().UTC()
	_, _, _, order := createSyntheticStoreOrder(t, db, "SYNTHETIC-RECON-REQUIRED-001", "accepted", 10000, &paidAt)
	refund := createUnknownRefundReservation(t, db, order, "SYNTHETIC-RF-REQUIRED-001", 2500)
	service := NewOrderService(db)

	checks := []struct {
		name string
		req  dto.RefundReconciliationRequest
		want string
	}{
		{name: "missing provider reference", req: dto.RefundReconciliationRequest{Outcome: "success", Reason: "已查看外部渠道记录"}, want: "provider-side refund/reference number is required"},
		{name: "missing note", req: dto.RefundReconciliationRequest{Outcome: "success", ProviderReference: "SYNTHETIC-PROVIDER-REF"}, want: "reconciliation note/reason is required"},
		{name: "merchant role", req: dto.RefundReconciliationRequest{Outcome: "success", ProviderReference: "SYNTHETIC-PROVIDER-REF", Reason: "已核对"}, want: "only platform administrators"},
		{name: "generic clear is unsupported", req: dto.RefundReconciliationRequest{Outcome: "clear", ProviderReference: "SYNTHETIC-PROVIDER-REF", Reason: "不得直接清空预留"}, want: "outcome must be success or failed"},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			_, err := service.ResolveUnknownRefund(refund.ID, check.req, 7, map[bool]string{true: "merchant", false: "admin"}[check.name == "merchant role"])
			if err == nil || !strings.Contains(err.Error(), check.want) {
				t.Fatalf("error=%v, want substring %q", err, check.want)
			}
		})
	}
	var current model.RefundRecord
	if err := db.First(&current, refund.ID).Error; err != nil {
		t.Fatal(err)
	}
	var orderAfter model.Order
	if err := db.First(&orderAfter, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	var auditCount int64
	if err := db.Model(&model.RefundReconciliationAudit{}).Where("refund_id = ?", refund.ID).Count(&auditCount).Error; err != nil {
		t.Fatal(err)
	}
	if current.Status != "unknown" || orderAfter.RefundedAmount != 0 || orderAfter.RefundStatus != "processing" || auditCount != 0 {
		t.Fatalf("invalid attempts must leave the unknown reservation intact: refund=%q refunded=%d status=%q audits=%d", current.Status, orderAfter.RefundedAmount, orderAfter.RefundStatus, auditCount)
	}
}

func TestRefundReconciliationSuccessIsAuditedAndIdempotent(t *testing.T) {
	db := newRefundReconciliationTestDB(t)
	paidAt := time.Now().UTC()
	_, _, _, order := createSyntheticStoreOrder(t, db, "SYNTHETIC-RECON-SUCCESS-001", "accepted", 10000, &paidAt)
	order.RefundedAmount = 700
	if err := db.Save(&order).Error; err != nil {
		t.Fatal(err)
	}
	refund := createUnknownRefundReservation(t, db, order, "SYNTHETIC-RF-SUCCESS-001", 2500)
	service := NewOrderService(db)
	req := dto.RefundReconciliationRequest{
		Outcome:           "success",
		ProviderReference: "SYNTHETIC-CHANNEL-REF-OK-001",
		Reason:            "已在合成渠道后台核对为退款成功",
	}

	first, err := service.ResolveUnknownRefund(refund.ID, req, 7, "admin")
	if err != nil {
		t.Fatalf("resolve synthetic success: %v", err)
	}
	if first.ReservationStatus != "success" || len(first.Audit) != 1 {
		t.Fatalf("success detail/audit mismatch: %+v", first)
	}
	var after model.Order
	if err := db.First(&after, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	if after.RefundedAmount != 3200 || after.RefundStatus != "partial" {
		t.Fatalf("success must add reserved amount once: refunded=%d refund_status=%q", after.RefundedAmount, after.RefundStatus)
	}
	if first.Audit[0].ActorID != 7 || first.Audit[0].ActorRole != "admin" || first.Audit[0].OldStatus != "unknown" || first.Audit[0].NewStatus != "success" || first.Audit[0].Amount != 2500 || first.Audit[0].ProviderReference != req.ProviderReference || first.Audit[0].Reason != req.Reason || first.Audit[0].CreatedAt.IsZero() {
		t.Fatalf("audit must retain the transition, actor, time, amount, evidence and reason: %+v", first.Audit[0])
	}
	var stored model.RefundRecord
	if err := db.First(&stored, refund.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.RawPayload != "synthetic gateway timeout; provider outcome unknown" {
		t.Fatalf("original unknown reason must be preserved, got %q", stored.RawPayload)
	}

	retry, err := service.ResolveUnknownRefund(refund.ID, req, 7, "admin")
	if err != nil {
		t.Fatalf("exact retry after response loss should be idempotent: %v", err)
	}
	if retry.ReservationStatus != "success" || len(retry.Audit) != 1 {
		t.Fatalf("idempotent retry must not append a duplicate audit: %+v", retry)
	}
	if err := db.First(&after, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	var auditCount int64
	if err := db.Model(&model.RefundReconciliationAudit{}).Where("refund_id = ?", refund.ID).Count(&auditCount).Error; err != nil {
		t.Fatal(err)
	}
	if after.RefundedAmount != 3200 || auditCount != 1 {
		t.Fatalf("idempotent retry changed financial balance/audit count: refunded=%d audits=%d", after.RefundedAmount, auditCount)
	}
}

func TestRefundReconciliationDefinitiveFailureReleasesReservationOnly(t *testing.T) {
	db := newRefundReconciliationTestDB(t)
	paidAt := time.Now().UTC()
	_, _, _, order := createSyntheticStoreOrder(t, db, "SYNTHETIC-RECON-FAILED-001", "accepted", 10000, &paidAt)
	order.RefundedAmount = 1200
	if err := db.Save(&order).Error; err != nil {
		t.Fatal(err)
	}
	refund := createUnknownRefundReservation(t, db, order, "SYNTHETIC-RF-FAILED-001", 800)
	service := NewOrderService(db)
	req := dto.RefundReconciliationRequest{
		Outcome:           "failed",
		ProviderReference: "SYNTHETIC-CHANNEL-REF-NO-001",
		Reason:            "合成渠道后台显示该退款请求明确失败",
	}
	resolved, err := service.ResolveUnknownRefund(refund.ID, req, 8, "super_admin")
	if err != nil {
		t.Fatalf("resolve synthetic definitive failure: %v", err)
	}
	var after model.Order
	if err := db.First(&after, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	if resolved.ReservationStatus != "failed" || len(resolved.Audit) != 1 || resolved.Audit[0].NewStatus != "failed" {
		t.Fatalf("failed reconciliation audit mismatch: %+v", resolved)
	}
	if after.RefundedAmount != 1200 || after.RefundStatus != "partial" {
		t.Fatalf("definitive failure may release reservation but must not add refunded amount: refunded=%d status=%q", after.RefundedAmount, after.RefundStatus)
	}
	retry, err := service.ResolveUnknownRefund(refund.ID, req, 8, "super_admin")
	if err != nil || len(retry.Audit) != 1 {
		t.Fatalf("exact failed-outcome retry must be idempotent: detail=%+v error=%v", retry, err)
	}
	if err := db.First(&after, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	if after.RefundedAmount != 1200 {
		t.Fatalf("failed-outcome retry must not change refunded amount, got %d", after.RefundedAmount)
	}
}

func TestUnknownRefundReservationBlocksAdditionalRefundUntilReconciled(t *testing.T) {
	db := newRefundReconciliationTestDB(t)
	paidAt := time.Now().UTC()
	merchant, _, _, order := createSyntheticStoreOrder(t, db, "SYNTHETIC-RECON-BLOCK-001", "accepted", 10000, &paidAt)
	order.TransactionNo = "SYNTHETIC-PAID-TRANSACTION"
	if err := db.Save(&order).Error; err != nil {
		t.Fatal(err)
	}
	refund := createUnknownRefundReservation(t, db, order, "SYNTHETIC-RF-BLOCK-001", 4000)
	service := NewOrderService(db)
	_, err := service.RefundMerchantOrder(merchant.ID, order.ID, dto.RefundOrderRequest{Amount: 100, Reason: "未知结果前不得换新键重试", IdempotencyKey: "synthetic-new-refund-key"}, 42, "merchant")
	if err == nil || !strings.Contains(err.Error(), "must be reconciled before any further refund") {
		t.Fatalf("unknown reservation must block every new refund, even within unreserved balance: %v", err)
	}
	var current model.RefundRecord
	if err := db.First(&current, refund.ID).Error; err != nil {
		t.Fatal(err)
	}
	var totalRefunds int64
	if err := db.Model(&model.RefundRecord{}).Where("order_id = ?", order.ID).Count(&totalRefunds).Error; err != nil {
		t.Fatal(err)
	}
	if current.Status != "unknown" || totalRefunds != 1 {
		t.Fatalf("rejected additional refund must leave the unresolved reservation intact: status=%q refund_count=%d", current.Status, totalRefunds)
	}
}

func TestRefundReconciliationAuditFailureRollsBackFinancialTransition(t *testing.T) {
	db := newRefundReconciliationTestDB(t)
	paidAt := time.Now().UTC()
	_, _, _, order := createSyntheticStoreOrder(t, db, "SYNTHETIC-RECON-ROLLBACK-001", "accepted", 10000, &paidAt)
	refund := createUnknownRefundReservation(t, db, order, "SYNTHETIC-RF-ROLLBACK-001", 2500)
	if err := db.Migrator().DropTable(&model.RefundReconciliationAudit{}); err != nil {
		t.Fatal(err)
	}
	_, err := NewOrderService(db).ResolveUnknownRefund(refund.ID, dto.RefundReconciliationRequest{Outcome: "success", ProviderReference: "SYNTHETIC-REF-ROLLBACK", Reason: "合成事务回滚测试"}, 7, "admin")
	if err == nil {
		t.Fatal("audit persistence failure must reject and roll back reconciliation")
	}
	var current model.RefundRecord
	if err := db.First(&current, refund.ID).Error; err != nil {
		t.Fatal(err)
	}
	var orderAfter model.Order
	if err := db.First(&orderAfter, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	if current.Status != "unknown" || orderAfter.RefundedAmount != 0 || orderAfter.RefundStatus != "processing" {
		t.Fatalf("audit failure must roll back refund and order: status=%q refunded=%d refund_status=%q", current.Status, orderAfter.RefundedAmount, orderAfter.RefundStatus)
	}
}

func TestRefundReconciliationAuditRejectsApplicationUpdateAndDelete(t *testing.T) {
	db := newRefundReconciliationTestDB(t)
	audit := model.RefundReconciliationAudit{RefundID: 1, RefundNo: "SYNTHETIC-APPEND-ONLY", OrderID: 1, ActorID: 7, ActorRole: "admin", OldStatus: "unknown", NewStatus: "success", Amount: 100, ProviderReference: "SYNTHETIC-REF-IMMUTABLE", Reason: "original evidence"}
	if err := db.Create(&audit).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&audit).Update("reason", "rewritten evidence").Error; err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("audit update must be rejected: %v", err)
	}
	if err := db.Delete(&audit).Error; err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("audit delete must be rejected: %v", err)
	}
	var stored model.RefundReconciliationAudit
	if err := db.First(&stored, audit.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Reason != "original evidence" {
		t.Fatalf("append-only audit was modified: %+v", stored)
	}
}

func TestStaleProcessingRefundMovesToManualReviewWithoutReleasingReservation(t *testing.T) {
	db := newRefundReconciliationTestDB(t)
	paidAt := time.Now().UTC()
	_, _, _, order := createSyntheticStoreOrder(t, db, "SYNTHETIC-STALE-PROCESSING-001", "accepted", 10000, &paidAt)
	order.RefundStatus = "processing"
	if err := db.Save(&order).Error; err != nil {
		t.Fatal(err)
	}
	staleKey := "synthetic-stale-processing-key"
	stale := model.RefundRecord{OrderID: order.ID, MerchantID: order.MerchantID, StoreID: order.StoreID, RefundNo: "SYNTHETIC-RF-STALE-001", IdempotencyKey: &staleKey, Amount: 2000, Reason: "合成逾期退款", OperatorRole: "merchant", OperatorID: 42, Status: "processing"}
	if err := db.Create(&stale).Error; err != nil {
		t.Fatalf("create stale synthetic refund reservation: %v", err)
	}
	recentKey := "synthetic-recent-processing-key"
	recent := model.RefundRecord{OrderID: order.ID, MerchantID: order.MerchantID, StoreID: order.StoreID, RefundNo: "SYNTHETIC-RF-RECENT-001", IdempotencyKey: &recentKey, Amount: 1000, Reason: "合成近期退款", OperatorRole: "merchant", OperatorID: 42, Status: "processing"}
	if err := db.Create(&recent).Error; err != nil {
		t.Fatalf("create recent synthetic refund reservation: %v", err)
	}
	staleUpdatedAt := time.Now().UTC().Add(-20 * time.Minute).Truncate(time.Millisecond)
	if err := db.Model(&model.RefundRecord{}).Where("id = ?", stale.ID).UpdateColumn("updated_at", staleUpdatedAt).Error; err != nil {
		t.Fatalf("age synthetic reservation: %v", err)
	}
	service := NewOrderService(db)
	queue, err := service.UnknownRefundReservations()
	if err != nil {
		t.Fatal(err)
	}
	if len(queue) != 2 {
		t.Fatalf("queue must include both unknown and actively processing reservations, got %d", len(queue))
	}
	staleFound := false
	for _, item := range queue {
		if item.RefundID == stale.ID {
			staleFound = item.IsStale && item.ReservationStatus == "processing" && item.StaleAfterMinutes == 15
		}
	}
	if !staleFound {
		t.Fatalf("15-minute stale processing reservation must be visibly flagged: %+v", queue)
	}

	if _, err := service.MarkStaleProcessingRefundUnknown(stale.ID, " ", 7, "admin"); err == nil || !strings.Contains(err.Error(), "note/reason is required") {
		t.Fatalf("manual transition without operator note must fail: %v", err)
	}
	if _, err := service.MarkStaleProcessingRefundUnknown(stale.ID, "合成原因", 42, "merchant"); err == nil || !strings.Contains(err.Error(), "only platform administrators") {
		t.Fatalf("merchant must not move reservation to manual review: %v", err)
	}
	if _, err := service.MarkStaleProcessingRefundUnknown(recent.ID, "尚未逾期", 7, "admin"); err == nil || !strings.Contains(err.Error(), "not overdue") {
		t.Fatalf("recent processing reservation must not be force-moved: %v", err)
	}

	note := "合成记录：本地预留已超过15分钟，等待操作员核查外部渠道"
	detail, err := service.MarkStaleProcessingRefundUnknown(stale.ID, note, 7, "admin")
	if err != nil {
		t.Fatalf("move stale synthetic reservation to manual review: %v", err)
	}
	if detail.ReservationStatus != "unknown" || detail.IsStale || len(detail.Audit) != 1 {
		t.Fatalf("manual-review transition/detail mismatch: %+v", detail)
	}
	audit := detail.Audit[0]
	if audit.ActorID != 7 || audit.ActorRole != "admin" || audit.OldStatus != "processing" || audit.NewStatus != "unknown" || audit.Amount != 2000 || audit.ProviderReference != "" || audit.Reason != note || audit.CreatedAt.IsZero() {
		t.Fatalf("manual-review audit must retain actor, time, status transition, amount and note without claiming external evidence: %+v", audit)
	}
	var stored model.RefundRecord
	if err := db.First(&stored, stale.ID).Error; err != nil {
		t.Fatal(err)
	}
	var after model.Order
	if err := db.First(&after, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	var reserved int64
	if err := db.Model(&model.RefundRecord{}).Where("order_id = ? AND status IN ?", order.ID, []string{"processing", "unknown"}).Select("COALESCE(SUM(amount), 0)").Scan(&reserved).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != "unknown" || after.RefundedAmount != 0 || after.RefundStatus != "processing" || reserved != 3000 {
		t.Fatalf("manual-review transition must preserve all reserved funds and avoid a success claim: refund=%q refunded=%d order_status=%q reserved=%d", stored.Status, after.RefundedAmount, after.RefundStatus, reserved)
	}
	for _, lateOutcome := range []string{"success", "failed"} {
		if _, err := service.finishRefundReservation(stale.ID, lateOutcome, "synthetic late provider result"); err == nil || !strings.Contains(err.Error(), "not processing") {
			t.Fatalf("late provider outcome %q must not overwrite unknown after the manual handoff: %v", lateOutcome, err)
		}
	}

	retry, err := service.MarkStaleProcessingRefundUnknown(stale.ID, note, 7, "admin")
	if err != nil || len(retry.Audit) != 1 || retry.ReservationStatus != "unknown" {
		t.Fatalf("exact transition retry must be idempotent: detail=%+v error=%v", retry, err)
	}
	var auditCount int64
	if err := db.Model(&model.RefundReconciliationAudit{}).Where("refund_id = ?", stale.ID).Count(&auditCount).Error; err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("exact retry must not append a duplicate audit, count=%d", auditCount)
	}
}

func TestStaleProcessingRefundAuditFailureRollsBackStatusTransition(t *testing.T) {
	db := newRefundReconciliationTestDB(t)
	paidAt := time.Now().UTC()
	_, _, _, order := createSyntheticStoreOrder(t, db, "SYNTHETIC-STALE-ROLLBACK-001", "accepted", 10000, &paidAt)
	key := "synthetic-stale-rollback-key"
	refund := model.RefundRecord{OrderID: order.ID, RefundNo: "SYNTHETIC-RF-STALE-ROLLBACK-001", IdempotencyKey: &key, Amount: 2500, Reason: "合成事务回滚测试", Status: "processing"}
	if err := db.Create(&refund).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.RefundRecord{}).Where("id = ?", refund.ID).UpdateColumn("updated_at", time.Now().UTC().Add(-20*time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropTable(&model.RefundReconciliationAudit{}); err != nil {
		t.Fatal(err)
	}
	_, err := NewOrderService(db).MarkStaleProcessingRefundUnknown(refund.ID, "合成审计失败回滚", 7, "admin")
	if err == nil {
		t.Fatal("audit persistence failure must reject and roll back the manual-review transition")
	}
	var after model.RefundRecord
	if err := db.First(&after, refund.ID).Error; err != nil {
		t.Fatal(err)
	}
	if after.Status != "processing" {
		t.Fatalf("failed audit insert must roll back refund status, got %q", after.Status)
	}
}

func TestStaleProcessingRefundMissingUpdateTimeUsesReservationCreationAge(t *testing.T) {
	db := newRefundReconciliationTestDB(t)
	paidAt := time.Now().UTC()
	_, _, _, order := createSyntheticStoreOrder(t, db, "SYNTHETIC-STALE-NULL-TIME", "accepted", 10000, &paidAt)
	refund := model.RefundRecord{OrderID: order.ID, RefundNo: "SYNTHETIC-RF-STALE-NULL-TIME", Amount: 1700, Reason: "合成历史空更新时间", Status: "processing"}
	if err := db.Create(&refund).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.RefundRecord{}).Where("id = ?", refund.ID).UpdateColumns(map[string]any{"created_at": time.Now().UTC().Add(-20 * time.Minute), "updated_at": nil}).Error; err != nil {
		t.Fatal(err)
	}
	service := NewOrderService(db)
	detail, err := service.RefundReconciliationDetail(refund.ID)
	if err != nil || !detail.IsStale {
		t.Fatalf("missing update time must use the old creation time for the stale flag: detail=%+v error=%v", detail, err)
	}
	detail, err = service.MarkStaleProcessingRefundUnknown(refund.ID, "合成历史预留已逾期，转人工核对", 7, "admin")
	if err != nil || detail.ReservationStatus != "unknown" || len(detail.Audit) != 1 || detail.Amount != 1700 {
		t.Fatalf("missing update time must not prevent a locked, audited handoff: detail=%+v error=%v", detail, err)
	}
}
