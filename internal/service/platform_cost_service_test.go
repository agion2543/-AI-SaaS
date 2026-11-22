package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

func TestPlatformCostOverviewCountsOnlyConfirmedSubscriptionPayments(t *testing.T) {
	db := newSyntheticPlatformCostDB(t)
	merchantA := createSyntheticCostMerchant(t, db, "SYNTHETIC-COST-MERCHANT-A")
	merchantB := createSyntheticCostMerchant(t, db, "SYNTHETIC-COST-MERCHANT-B")
	merchantC := createSyntheticCostMerchant(t, db, "SYNTHETIC-COST-MERCHANT-C")
	now := time.Now()

	confirmedCurrent := createSyntheticCostOrder(t, db, merchantA.ID, "SYNTHETIC-COST-PAID-CURRENT", "merchant_subscription", "paid", "platform_qr", now, 19900, 4900, "success", 19900)
	createSyntheticCostOrder(t, db, merchantB.ID, "SYNTHETIC-COST-PAID-OLDER", "merchant_subscription", "paid", "platform_qr", now.AddDate(0, -2, 0), 7900, 0, "success", 7900)
	createSyntheticCostOrder(t, db, merchantC.ID, "SYNTHETIC-COST-FULL-REFUND", "merchant_subscription", "paid", "platform_qr", now, 3000, 3000, "success", 3000)

	// The order's denormalized refund amount is deliberately larger than its
	// successful RefundRecord. Pending/unknown refunds are reservations, not cash.
	if err := db.Model(&model.Order{}).Where("id = ?", confirmedCurrent.ID).Update("refunded_amount", 14900).Error; err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"processing", "unknown"} {
		if err := db.Create(&model.RefundRecord{OrderID: confirmedCurrent.ID, RefundNo: fmt.Sprintf("SYNTHETIC-COST-%s", strings.ToUpper(status)), Amount: 2500, Status: status}).Error; err != nil {
			t.Fatalf("create synthetic %s refund reservation: %v", status, err)
		}
	}

	// Payment reminders, historical manual reviews, store orders, and rows without
	// a successful payment record must never become SaaS subscription revenue.
	createSyntheticCostOrder(t, db, merchantA.ID, "SYNTHETIC-COST-PENDING", "merchant_subscription", "pending", "platform_qr", time.Time{}, 99000, 0, "success", 99000)
	createSyntheticCostOrder(t, db, merchantA.ID, "SYNTHETIC-COST-MANUAL-REVIEW", "merchant_subscription", "manually_verified", "admin_manual_verified", time.Time{}, 49000, 0, "", 0)
	createSyntheticCostOrder(t, db, merchantA.ID, "SYNTHETIC-COST-STORE-ORDER", "store_order", "paid", "merchant_qr", now, 88000, 0, "success", 88000)
	createSyntheticCostOrder(t, db, merchantA.ID, "SYNTHETIC-COST-NO-PAYMENT-RECORD", "merchant_subscription", "paid", "platform_qr", now, 25000, 0, "", 0)
	createSyntheticCostOrder(t, db, merchantA.ID, "SYNTHETIC-COST-MANUAL-CHANNEL", "merchant_subscription", "paid", "admin_manual_verified", now, 18000, 0, "success", 18000)
	createSyntheticCostOrder(t, db, merchantA.ID, "SYNTHETIC-COST-PENDING-RECORD", "merchant_subscription", "paid", "platform_qr", now, 17000, 0, "pending", 17000)

	service := NewAdminService(&config.Config{}, db)
	snapshot, err := service.PlatformCostOverview()
	if err != nil {
		t.Fatalf("read synthetic platform cost snapshot: %v", err)
	}

	if got := snapshot.CurrentMonth; got.PaidOrderCount != 2 || got.MerchantCount != 2 || got.GrossReceivedCents != 22900 || got.RefundedCents != 7900 || got.NetReceivedCents != 15000 {
		t.Fatalf("current-month confirmed subscription summary mismatch: %+v", got)
	}
	if got := snapshot.Lifetime; got.PaidOrderCount != 3 || got.MerchantCount != 3 || got.GrossReceivedCents != 30800 || got.RefundedCents != 7900 || got.NetReceivedCents != 22900 {
		t.Fatalf("lifetime confirmed subscription summary mismatch: %+v", got)
	}
	if got := snapshot.SelectedPeriod; got.ActualExpenseCount != 0 || got.ActualExpenseTotalCents != nil || got.NetAfterRecordedExpenses != nil || got.ProfitStatusNote != "暂无已录入成本，利润暂不可算。" {
		t.Fatalf("empty expense period must not imply zero cost or profit: %+v", got)
	}
	if confirmedCurrent.ID == 0 {
		t.Fatal("synthetic confirmed subscription order was not persisted")
	}
	if snapshot.AIUsage.Status != "not_connected" || snapshot.AIUsage.ProviderConnected || snapshot.AIUsage.DataAvailable || len(snapshot.AIUsage.MerchantRanking) != 0 {
		t.Fatalf("unavailable AI usage was represented as actual data: %+v", snapshot.AIUsage)
	}
	if snapshot.AIUsage.RequestCount != nil || snapshot.AIUsage.TotalTokens != nil || snapshot.AIUsage.CostCents != nil {
		t.Fatalf("unavailable AI values must remain null, not zero: %+v", snapshot.AIUsage)
	}
	if snapshot.ProfitStatus != "incomplete" {
		t.Fatalf("profit must remain unavailable while costs are unknown, got %q", snapshot.ProfitStatus)
	}
}

func TestManualVerifiedSubscriptionAttestationsStaySeparateAndAuditBound(t *testing.T) {
	db := newSyntheticPlatformCostDB(t)
	merchantA := createSyntheticCostMerchant(t, db, "SYNTHETIC-MANUAL-REVENUE-MERCHANT-A")
	merchantB := createSyntheticCostMerchant(t, db, "SYNTHETIC-MANUAL-REVENUE-MERCHANT-B")
	now := time.Now().In(time.Local)
	reviewedAt := now.UTC().Truncate(time.Millisecond)

	currentManual := createSyntheticCostOrder(t, db, merchantA.ID, "SYNTHETIC-MANUAL-CURRENT", "merchant_subscription", "manually_verified", "admin_manual_verified", time.Time{}, 49000, 0, "", 0)
	createSyntheticManualReview(t, db, currentManual, reviewedAt, true, "")
	oldManual := createSyntheticCostOrder(t, db, merchantB.ID, "SYNTHETIC-MANUAL-OLD", "merchant_subscription", "manually_verified", "admin_manual_verified", time.Time{}, 25000, 0, "", 0)
	createSyntheticManualReview(t, db, oldManual, reviewedAt.AddDate(-1, 0, 0), true, "")

	missingAudit := createSyntheticCostOrder(t, db, merchantA.ID, "SYNTHETIC-MANUAL-NO-AUDIT", "merchant_subscription", "manually_verified", "admin_manual_verified", time.Time{}, 13000, 0, "", 0)
	createSyntheticManualReview(t, db, missingAudit, reviewedAt, false, "")
	evidenceMismatch := createSyntheticCostOrder(t, db, merchantA.ID, "SYNTHETIC-MANUAL-EVIDENCE-MISMATCH", "merchant_subscription", "manually_verified", "admin_manual_verified", time.Time{}, 17000, 0, "", 0)
	createSyntheticManualReview(t, db, evidenceMismatch, reviewedAt, true, "SYNTHETIC-MISMATCHED-EVIDENCE")
	manualWithPaymentRow := createSyntheticCostOrder(t, db, merchantA.ID, "SYNTHETIC-MANUAL-WITH-PAYMENT-ROW", "merchant_subscription", "manually_verified", "admin_manual_verified", time.Time{}, 35000, 0, "success", 35000)
	createSyntheticManualReview(t, db, manualWithPaymentRow, reviewedAt, true, "")
	createSyntheticCostOrder(t, db, merchantA.ID, "SYNTHETIC-PROVIDER-CONFIRMED", "merchant_subscription", "paid", "platform_qr", now, 100000, 0, "success", 100000)

	service := NewAdminService(&config.Config{}, db)
	request := validSyntheticExpenseRequest(t)
	request.AmountCents = 5000
	if _, created, err := service.CreatePlatformExpense(request, AuditActor{ID: 77, Type: "super_admin"}); err != nil || !created {
		t.Fatalf("create isolated synthetic expense: created=%v err=%v", created, err)
	}
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	snapshot, err := service.PlatformCostOverviewForPeriod(monthStart, today)
	if err != nil {
		t.Fatalf("read synthetic manual verification summary: %v", err)
	}
	if got := snapshot.CurrentMonth; got.NetReceivedCents != 100000 || got.GrossReceivedCents != 100000 {
		t.Fatalf("manual attestations must not be merged into provider-confirmed receipts: %+v", got)
	}
	if got := snapshot.CurrentMonthManualVerified; got.ReviewCount != 1 || got.MerchantCount != 1 || got.AttestedAmountCents != 49000 {
		t.Fatalf("current-month manual verification summary mismatch: %+v", got)
	}
	if got := snapshot.LifetimeManualVerified; got.ReviewCount != 2 || got.MerchantCount != 2 || got.AttestedAmountCents != 74000 {
		t.Fatalf("lifetime manual verification summary mismatch: %+v", got)
	}
	period := snapshot.SelectedPeriod
	if got := period.ManualVerifiedSubscription; got.ReviewCount != 1 || got.MerchantCount != 1 || got.AttestedAmountCents != 49000 {
		t.Fatalf("selected-period manual verification summary mismatch: %+v", got)
	}
	if period.SubscriptionNetReceived != 100000 || period.NetAfterRecordedExpenses == nil || *period.NetAfterRecordedExpenses != 95000 {
		t.Fatalf("manual attestations must not change confirmed net receipts or cost difference: %+v", period)
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("encode synthetic summary: %v", err)
	}
	if strings.Contains(string(encoded), "SYNTHETIC-EVIDENCE") || strings.Contains(string(encoded), "SYNTHETIC-MISMATCHED-EVIDENCE") || strings.Contains(string(encoded), "synthetic manual-review reason") {
		t.Fatalf("manual-review evidence and reason must not leak into the cost summary: %s", encoded)
	}
}

func TestPlatformExpenseAppendIsAtomicIdempotentAndPrivacyMinimized(t *testing.T) {
	db := newSyntheticPlatformCostDB(t)
	service := NewAdminService(&config.Config{}, db)
	actor := AuditActor{ID: 7, Type: "super_admin", IP: "192.0.2.10", UserAgent: "synthetic-test-agent"}
	request := validSyntheticExpenseRequest(t)
	request.AmountCents = 2300
	request.Memo = "平台托管费 O'Reilly -- synthetic"

	entry, created, err := service.CreatePlatformExpense(request, actor)
	if err != nil || !created {
		t.Fatalf("append synthetic expense: entry=%+v created=%v err=%v", entry, created, err)
	}
	if entry.Currency != "CNY" || entry.AmountCents != 2300 || entry.Category != model.PlatformExpenseCategoryCloudHosting || entry.CreatedBy != actor.ID || entry.Memo != request.Memo {
		t.Fatalf("saved expense differs from validated request: %+v", entry)
	}

	var audit model.AuditLog
	if err := db.Where("action = ?", "platform_expense_create").Take(&audit).Error; err != nil {
		t.Fatalf("read synthetic audit: %v", err)
	}
	if audit.ActorID != actor.ID || audit.ActorType != "super_admin" || audit.TargetType != "platform_expense" || audit.TargetID != entry.ID || strings.Contains(audit.Detail, request.Memo) || strings.Contains(audit.Detail, "2300") {
		t.Fatalf("audit should retain actor/target/purpose only, got %+v", audit)
	}
	if strings.Contains(audit.Detail, actor.IP) || strings.Contains(audit.Detail, actor.UserAgent) {
		t.Fatalf("privacy-minimized expense audit unexpectedly retained IP/user agent: %q", audit.Detail)
	}
	replayed, created, err := service.CreatePlatformExpense(request, actor)
	if err != nil || created || replayed.ID != entry.ID {
		t.Fatalf("idempotent replay mismatch: entry=%+v created=%v err=%v", replayed, created, err)
	}
	var expenseCount, auditCount int64
	if err := db.Model(&model.PlatformExpense{}).Count(&expenseCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.AuditLog{}).Where("action = ?", "platform_expense_create").Count(&auditCount).Error; err != nil {
		t.Fatal(err)
	}
	if expenseCount != 1 || auditCount != 1 {
		t.Fatalf("idempotent replay duplicated record/audit: expenses=%d audits=%d", expenseCount, auditCount)
	}

	conflict := request
	conflict.AmountCents++
	if _, _, err := service.CreatePlatformExpense(conflict, actor); !errors.Is(err, ErrPlatformExpenseIdempotencyConflict) {
		t.Fatalf("reusing an idempotency key with changed payload must conflict, got %v", err)
	}
	if err := db.Model(&model.PlatformExpense{}).Where("id = ?", entry.ID).Update("memo", "tampered").Error; err == nil {
		t.Fatal("GORM update hook allowed an immutable platform expense update")
	}
	if err := db.Delete(&model.PlatformExpense{}, entry.ID).Error; err == nil {
		t.Fatal("GORM delete hook allowed an immutable platform expense deletion")
	}

	if err := db.Exec(`CREATE TRIGGER synthetic_audit_failure BEFORE INSERT ON audit_logs WHEN NEW.action='platform_expense_create' BEGIN SELECT RAISE(ABORT, 'synthetic audit failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	failedRequest := validSyntheticExpenseRequest(t)
	if _, _, err := service.CreatePlatformExpense(failedRequest, actor); err == nil {
		t.Fatal("expected the synthetic audit failure to abort the expense transaction")
	}
	if err := db.Model(&model.PlatformExpense{}).Count(&expenseCount).Error; err != nil {
		t.Fatal(err)
	}
	if expenseCount != 1 {
		t.Fatalf("expense row survived its failed atomic audit: rows=%d", expenseCount)
	}
}

func TestPlatformExpenseValidationRejectsInvalidInputsAndNonSuperAdmin(t *testing.T) {
	actor := AuditActor{ID: 9, Type: "super_admin"}
	base := validSyntheticExpenseRequest(t)
	future := time.Now().In(time.Local).AddDate(0, 0, 1).Format("2006-01-02")
	cases := []struct {
		name   string
		mutate func(*dto.CreatePlatformExpenseRequest)
	}{
		{"unknown category", func(r *dto.CreatePlatformExpenseRequest) { r.Category = "other; DROP TABLE platform_expenses" }},
		{"zero amount", func(r *dto.CreatePlatformExpenseRequest) { r.AmountCents = 0 }},
		{"negative amount", func(r *dto.CreatePlatformExpenseRequest) { r.AmountCents = -1 }},
		{"overflow amount", func(r *dto.CreatePlatformExpenseRequest) { r.AmountCents = maxPlatformExpenseCents + 1 }},
		{"unsupported currency", func(r *dto.CreatePlatformExpenseRequest) { r.Currency = "USD" }},
		{"malformed date", func(r *dto.CreatePlatformExpenseRequest) { r.ExpenseDate = "2026-2-01" }},
		{"future expense date", func(r *dto.CreatePlatformExpenseRequest) { r.ExpenseDate = future }},
		{"empty memo", func(r *dto.CreatePlatformExpenseRequest) { r.Memo = "   " }},
		{"oversized memo", func(r *dto.CreatePlatformExpenseRequest) {
			r.Memo = strings.Repeat("中", maxPlatformExpenseMemoRunes+1)
		}},
		{"control character in memo", func(r *dto.CreatePlatformExpenseRequest) { r.Memo = "费用\n明细" }},
		{"HTML markup in memo", func(r *dto.CreatePlatformExpenseRequest) { r.Memo = "<script>alert(1)</script>" }},
		{"invalid idempotency key", func(r *dto.CreatePlatformExpenseRequest) { r.IdempotencyKey = "not-a-uuid" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := base
			tc.mutate(&request)
			if _, err := validatePlatformExpenseRequest(request, actor.ID); !errors.Is(err, ErrInvalidPlatformExpense) {
				t.Fatalf("expected invalid-input error, got %v", err)
			}
		})
	}
	if _, err := validatePlatformExpenseRequest(base, 0); err != nil {
		t.Fatalf("valid request should reach actor-role authorization: %v", err)
	}

	db := newSyntheticPlatformCostDB(t)
	service := NewAdminService(&config.Config{}, db)
	if _, _, err := service.CreatePlatformExpense(base, AuditActor{ID: 9, Type: "finance"}); !errors.Is(err, ErrInvalidPlatformExpense) {
		t.Fatalf("non-super_admin expense write must be rejected, got %v", err)
	}
	var count int64
	if err := db.Model(&model.PlatformExpense{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("non-super_admin write created an expense: count=%d err=%v", count, err)
	}
}

func TestPlatformCostPeriodIncludesRecordedActualCostsButNotPendingRefunds(t *testing.T) {
	db := newSyntheticPlatformCostDB(t)
	merchant := createSyntheticCostMerchant(t, db, "SYNTHETIC-PERIOD-MERCHANT")
	now := time.Now().In(time.Local)
	order := createSyntheticCostOrder(t, db, merchant.ID, "SYNTHETIC-PERIOD-ORDER", "merchant_subscription", "paid", "platform_qr", now, 100000, 10000, "success", 100000)
	if err := db.Create(&model.RefundRecord{OrderID: order.ID, RefundNo: "SYNTHETIC-PERIOD-PENDING", Amount: 5000, Status: "processing"}).Error; err != nil {
		t.Fatal(err)
	}

	service := NewAdminService(&config.Config{}, db)
	actor := AuditActor{ID: 3, Type: "super_admin"}
	request := validSyntheticExpenseRequest(t)
	request.AmountCents = 2300
	request.ExpenseDate = now.Format("2006-01-02")
	if _, created, err := service.CreatePlatformExpense(request, actor); err != nil || !created {
		t.Fatalf("create synthetic expense: created=%v err=%v", created, err)
	}

	snapshot, err := service.PlatformCostOverview()
	if err != nil {
		t.Fatalf("read period summary: %v", err)
	}
	got := snapshot.SelectedPeriod
	if got.SubscriptionGrossReceived != 100000 || got.SubscriptionConfirmedRefunds != 10000 || got.SubscriptionNetReceived != 90000 {
		t.Fatalf("period revenue/refund values mismatch: %+v", got)
	}
	if got.ActualExpenseCount != 1 || got.ActualExpenseTotalCents == nil || *got.ActualExpenseTotalCents != 2300 || got.NetAfterRecordedExpenses == nil || *got.NetAfterRecordedExpenses != 87700 {
		t.Fatalf("period actual expense/net-after-recorded values mismatch: %+v", got)
	}
	if got.ProfitStatus != "incomplete" || strings.Contains(strings.ToLower(got.ProfitStatusNote), "net profit") || got.PlansStatus != "not_recorded" || len(got.ExpenseRows) != 1 {
		t.Fatalf("period summary must not imply profit or include plans: %+v", got)
	}
}

func TestPlatformCostPeriodRejectsInvalidOrOverlongRange(t *testing.T) {
	db := newSyntheticPlatformCostDB(t)
	service := NewAdminService(&config.Config{}, db)
	today := time.Now().In(time.Local)
	if _, err := service.PlatformCostOverviewForPeriod(today, today.AddDate(0, 0, 1)); !errors.Is(err, ErrInvalidPlatformExpense) {
		t.Fatalf("future period should be rejected, got %v", err)
	}
	if _, err := service.PlatformCostOverviewForPeriod(today.AddDate(0, 0, -366), today); !errors.Is(err, ErrInvalidPlatformExpense) {
		t.Fatalf("period exceeding 366 days should be rejected, got %v", err)
	}
}

func newSyntheticPlatformCostDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open synthetic SQLite database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get synthetic SQLite database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&model.MerchantPlan{}, &model.Merchant{}, &model.Order{}, &model.PaymentRecord{}, &model.RefundRecord{}, &model.MerchantSubscriptionManualReview{}, &model.MerchantSubscriptionManualReviewAudit{}, &model.PlatformExpense{}, &model.AuditLog{}); err != nil {
		t.Fatalf("migrate synthetic accounting schema: %v", err)
	}
	if err := db.Exec(`CREATE TABLE platform_expense_reversals (
		id INTEGER PRIMARY KEY AUTOINCREMENT, created_at datetime NOT NULL, updated_at datetime NOT NULL,
		expense_id INTEGER NOT NULL REFERENCES platform_expenses(id) ON DELETE RESTRICT,
		amount_cents INTEGER NOT NULL CHECK (amount_cents > 0), reversal_date date NOT NULL,
		reason varchar(500) NOT NULL CHECK (LENGTH(TRIM(reason)) BETWEEN 1 AND 500),
		created_by INTEGER NOT NULL, idempotency_key char(36) NOT NULL UNIQUE
	)`).Error; err != nil {
		t.Fatalf("create isolated synthetic reversal table with explicit DDL: %v", err)
	}
	return db
}

func validSyntheticExpenseRequest(t *testing.T) dto.CreatePlatformExpenseRequest {
	t.Helper()
	return dto.CreatePlatformExpenseRequest{
		Category:       model.PlatformExpenseCategoryCloudHosting,
		AmountCents:    2300,
		Currency:       "CNY",
		ExpenseDate:    time.Now().In(time.Local).Format("2006-01-02"),
		Memo:           "synthetic cloud hosting bill",
		IdempotencyKey: uuid.NewString(),
	}
}

func createSyntheticManualReview(t *testing.T, db *gorm.DB, order model.Order, reviewedAt time.Time, createAudit bool, auditEvidenceReference string) {
	t.Helper()
	if order.MerchantID == nil {
		t.Fatal("synthetic manual review order must have a merchant")
	}
	evidenceReference := fmt.Sprintf("SYNTHETIC-EVIDENCE-%d", order.ID)
	review := model.MerchantSubscriptionManualReview{
		ReviewKey:               fmt.Sprintf("SYNTHETIC-MANUAL-REVIEW-%d", order.ID),
		OrderID:                 order.ID,
		OrderNo:                 order.OrderNo,
		MerchantID:              *order.MerchantID,
		ActorID:                 702,
		ActorRole:               "super_admin",
		PlanName:                "synthetic historical plan",
		AmountCents:             order.Amount,
		DurationDays:            30,
		PaymentOccurredAttested: true,
		EvidenceReference:       evidenceReference,
		Reason:                  "synthetic manual-review reason",
		RequestFingerprint:      strings.Repeat("a", 64),
		ReviewStatus:            "manually_verified",
		CreatedAt:               reviewedAt,
		UpdatedAt:               reviewedAt,
	}
	if err := db.Create(&review).Error; err != nil {
		t.Fatalf("create synthetic manual review: %v", err)
	}
	if !createAudit {
		return
	}
	if auditEvidenceReference == "" {
		auditEvidenceReference = evidenceReference
	}
	audit := model.MerchantSubscriptionManualReviewAudit{
		ReviewID:                  review.ID,
		OrderID:                   order.ID,
		MerchantID:                *order.MerchantID,
		ActorID:                   review.ActorID,
		ActorRole:                 review.ActorRole,
		EvidenceReference:         auditEvidenceReference,
		Reason:                    review.Reason,
		PlanName:                  review.PlanName,
		AmountCents:               review.AmountCents,
		DurationDays:              review.DurationDays,
		PaymentOccurredAttested:   true,
		OrderStatusBefore:         "pending",
		OrderStatusAfter:          "manually_verified",
		OrderUpdatedAtBefore:      reviewedAt.Add(-time.Minute),
		OrderUpdatedAtAfter:       reviewedAt,
		SubscriptionStatusBefore:  "inactive",
		SubscriptionStatusAfter:   "active",
		SubscriptionPlanBefore:    "none",
		SubscriptionPlanAfter:     "历史复核",
		SubscriptionExpireAtAfter: reviewedAt.AddDate(0, 0, 30),
		ReviewedAt:                reviewedAt,
		CreatedAt:                 reviewedAt,
	}
	if err := db.Create(&audit).Error; err != nil {
		t.Fatalf("create synthetic manual review audit: %v", err)
	}
}

func createSyntheticCostMerchant(t *testing.T, db *gorm.DB, name string) model.Merchant {
	t.Helper()
	merchant := model.Merchant{Name: name, Status: "active"}
	if err := db.Create(&merchant).Error; err != nil {
		t.Fatalf("create synthetic merchant %q: %v", name, err)
	}
	return merchant
}

func createSyntheticCostOrder(t *testing.T, db *gorm.DB, merchantID uint, orderNo, orderType, status, paymentChannel string, paidAt time.Time, total, refunded int64, paymentStatus string, paymentAmount int64) model.Order {
	t.Helper()
	order := model.Order{
		OrderNo:        orderNo,
		OrderType:      orderType,
		MerchantID:     &merchantID,
		Status:         status,
		PaymentChannel: paymentChannel,
		TotalAmount:    total,
		Amount:         total,
		RefundedAmount: refunded,
	}
	if !paidAt.IsZero() {
		order.PaidAt = &paidAt
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create synthetic order %q: %v", orderNo, err)
	}
	if paymentStatus != "" {
		payment := model.PaymentRecord{OrderID: order.ID, Amount: paymentAmount, Status: paymentStatus, TransactionNo: orderNo + "-TXN"}
		if err := db.Create(&payment).Error; err != nil {
			t.Fatalf("create synthetic payment for %q: %v", orderNo, err)
		}
	}
	if refunded > 0 {
		refund := model.RefundRecord{OrderID: order.ID, RefundNo: orderNo + "-SUCCESS-REFUND", Amount: refunded, Status: "success"}
		if err := db.Create(&refund).Error; err != nil {
			t.Fatalf("create synthetic successful refund for %q: %v", orderNo, err)
		}
	}
	return order
}

func TestPlatformExpenseReversalIsAppendOnlyAndPeriodizedByItsOwnDate(t *testing.T) {
	db := newSyntheticPlatformCostDB(t)
	service := NewAdminService(&config.Config{}, db)
	actor := AuditActor{ID: 77, Type: "super_admin", IP: "192.0.2.77", UserAgent: "synthetic-reversal-test"}
	now := time.Now().In(time.Local)
	originalDate := now.AddDate(0, -2, 0).Format("2006-01-02")
	request := validSyntheticExpenseRequest(t)
	request.AmountCents = 2300
	request.ExpenseDate = originalDate
	entry, created, err := service.CreatePlatformExpense(request, actor)
	if err != nil || !created {
		t.Fatalf("create synthetic original expense: created=%v err=%v", created, err)
	}
	var originalAuditBefore model.AuditLog
	if err := db.Where("action = ? AND target_id = ?", "platform_expense_create", entry.ID).Take(&originalAuditBefore).Error; err != nil {
		t.Fatalf("read original expense audit: %v", err)
	}

	firstReversalDate := now.AddDate(0, -1, 0).Format("2006-01-02")
	firstRequest := dto.CreatePlatformExpenseReversalRequest{
		ReversalDate: firstReversalDate, AmountCents: 900,
		Reason: "synthetic correction after checking the original receipt", IdempotencyKey: uuid.NewString(),
	}
	first, created, err := service.CreatePlatformExpenseReversal(entry.ID, firstRequest, actor)
	if err != nil || !created {
		t.Fatalf("append first synthetic partial reversal: reversal=%+v created=%v err=%v", first, created, err)
	}
	if first.ExpenseID != entry.ID || first.AmountCents != 900 || string(first.ReversalDate) != firstReversalDate || first.Reason != firstRequest.Reason || first.CreatedBy != actor.ID {
		t.Fatalf("stored reversal differs from validated request: %+v", first)
	}
	replayed, created, err := service.CreatePlatformExpenseReversal(entry.ID, firstRequest, actor)
	if err != nil || created || replayed.ID != first.ID {
		t.Fatalf("idempotent reversal replay mismatch: reversal=%+v created=%v err=%v", replayed, created, err)
	}
	conflict := firstRequest
	conflict.AmountCents = 901
	if _, _, err := service.CreatePlatformExpenseReversal(entry.ID, conflict, actor); !errors.Is(err, ErrPlatformExpenseReversalKeyConflict) {
		t.Fatalf("same reversal idempotency key with changed amount must conflict, got %v", err)
	}

	secondDate := now.Format("2006-01-02")
	secondRequest := dto.CreatePlatformExpenseReversalRequest{
		ReversalDate: secondDate, AmountCents: 1400,
		Reason: "synthetic remaining balance correction", IdempotencyKey: uuid.NewString(),
	}
	second, created, err := service.CreatePlatformExpenseReversal(entry.ID, secondRequest, actor)
	if err != nil || !created || second.AmountCents != 1400 {
		t.Fatalf("append second synthetic partial reversal: reversal=%+v created=%v err=%v", second, created, err)
	}
	overage := secondRequest
	overage.AmountCents = 1
	overage.IdempotencyKey = uuid.NewString()
	if _, _, err := service.CreatePlatformExpenseReversal(entry.ID, overage, actor); !errors.Is(err, ErrPlatformExpenseReversalExceedsOriginal) {
		t.Fatalf("cumulative reversal above original expense must be rejected, got %v", err)
	}

	var originalAfter model.PlatformExpense
	if err := db.First(&originalAfter, entry.ID).Error; err != nil {
		t.Fatalf("reload original immutable expense: %v", err)
	}
	if originalAfter.Category != entry.Category || originalAfter.AmountCents != entry.AmountCents || originalAfter.Currency != entry.Currency || originalAfter.ExpenseDate != entry.ExpenseDate || originalAfter.Memo != entry.Memo || originalAfter.CreatedBy != entry.CreatedBy {
		t.Fatalf("reversal changed the original expense: before=%+v after=%+v", entry, originalAfter)
	}
	var originalAuditAfter model.AuditLog
	if err := db.First(&originalAuditAfter, originalAuditBefore.ID).Error; err != nil {
		t.Fatalf("reload original expense audit: %v", err)
	}
	if originalAuditAfter.ID != originalAuditBefore.ID || originalAuditAfter.Action != originalAuditBefore.Action || originalAuditAfter.TargetID != originalAuditBefore.TargetID || originalAuditAfter.Detail != originalAuditBefore.Detail || !originalAuditAfter.CreatedAt.Equal(originalAuditBefore.CreatedAt) {
		t.Fatalf("reversal changed the original expense audit: before=%+v after=%+v", originalAuditBefore, originalAuditAfter)
	}
	if strings.Contains(originalAuditAfter.Detail, firstRequest.Reason) || strings.Contains(originalAuditAfter.Detail, "2300") {
		t.Fatalf("original expense audit unexpectedly contains reversal reason or amount: %q", originalAuditAfter.Detail)
	}
	if err := db.Model(&model.PlatformExpenseReversal{}).Where("id = ?", first.ID).Update("reason", "tampered").Error; err == nil {
		t.Fatal("GORM update hook allowed an immutable reversal update")
	}
	if err := db.Delete(&model.PlatformExpenseReversal{}, first.ID).Error; err == nil {
		t.Fatal("GORM delete hook allowed an immutable reversal deletion")
	}
	var reversalAudit model.AuditLog
	if err := db.Where("action = ? AND target_id = ?", "platform_expense_reversal_create", first.ID).Take(&reversalAudit).Error; err != nil {
		t.Fatalf("read first reversal audit: %v", err)
	}
	if reversalAudit.ActorID != actor.ID || reversalAudit.TargetType != "platform_expense_reversal" || reversalAudit.TargetID != first.ID || strings.Contains(reversalAudit.Detail, firstRequest.Reason) || strings.Contains(reversalAudit.Detail, "900") || strings.Contains(reversalAudit.Detail, actor.IP) || strings.Contains(reversalAudit.Detail, actor.UserAgent) {
		t.Fatalf("reversal audit must retain target and purpose but omit reason, amount, IP and user agent: %+v", reversalAudit)
	}
	var reversalCount, originalAuditCount, reversalAuditCount int64
	if err := db.Model(&model.PlatformExpenseReversal{}).Count(&reversalCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.AuditLog{}).Where("action = ?", "platform_expense_create").Count(&originalAuditCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.AuditLog{}).Where("action = ?", "platform_expense_reversal_create").Count(&reversalAuditCount).Error; err != nil {
		t.Fatal(err)
	}
	if reversalCount != 2 || originalAuditCount != 1 || reversalAuditCount != 2 {
		t.Fatalf("partial reversal ledger/audit count mismatch: reversals=%d original audits=%d reversal audits=%d", reversalCount, originalAuditCount, reversalAuditCount)
	}

	originalDay, err := time.ParseInLocation("2006-01-02", originalDate, time.Local)
	if err != nil {
		t.Fatal(err)
	}
	originalPeriod, err := service.PlatformCostOverviewForPeriod(originalDay, originalDay)
	if err != nil {
		t.Fatalf("read original synthetic expense period: %v", err)
	}
	originalSummary := originalPeriod.SelectedPeriod
	if originalSummary.ActualExpenseCount != 1 || originalSummary.ActualExpenseTotalCents == nil || *originalSummary.ActualExpenseTotalCents != 2300 || originalSummary.ExpenseReversalCount != 0 || originalSummary.ExpenseRows[0].ReversalCount != 2 || originalSummary.ExpenseRows[0].ReversedAmountCents != 2300 || originalSummary.ExpenseRows[0].RemainingReversibleCents != 0 {
		t.Fatalf("original expense period must remain unchanged and expose cumulative reversal balance: %+v", originalSummary)
	}
	firstDay, err := time.ParseInLocation("2006-01-02", firstReversalDate, time.Local)
	if err != nil {
		t.Fatal(err)
	}
	firstPeriod, err := service.PlatformCostOverviewForPeriod(firstDay, firstDay)
	if err != nil {
		t.Fatalf("read first synthetic reversal period: %v", err)
	}
	firstSummary := firstPeriod.SelectedPeriod
	if firstSummary.ActualExpenseCount != 0 || firstSummary.ExpenseReversalCount != 1 || firstSummary.ExpenseReversalTotalCents == nil || *firstSummary.ExpenseReversalTotalCents != 900 || firstSummary.ActualExpenseTotalCents == nil || *firstSummary.ActualExpenseTotalCents != -900 || firstSummary.NetAfterRecordedExpenses == nil || *firstSummary.NetAfterRecordedExpenses != 900 || len(firstSummary.ExpenseReversalRows) != 1 || firstSummary.ExpenseReversalRows[0].ExpenseID != entry.ID || firstSummary.ExpenseReversalRows[0].AmountCents != 900 {
		t.Fatalf("first partial reversal must offset only its own date period: %+v", firstSummary)
	}
	secondDay, err := time.ParseInLocation("2006-01-02", secondDate, time.Local)
	if err != nil {
		t.Fatal(err)
	}
	secondPeriod, err := service.PlatformCostOverviewForPeriod(secondDay, secondDay)
	if err != nil {
		t.Fatalf("read second synthetic reversal period: %v", err)
	}
	secondSummary := secondPeriod.SelectedPeriod
	if secondSummary.ActualExpenseCount != 0 || secondSummary.ExpenseReversalCount != 1 || secondSummary.ExpenseReversalTotalCents == nil || *secondSummary.ExpenseReversalTotalCents != 1400 || secondSummary.ActualExpenseTotalCents == nil || *secondSummary.ActualExpenseTotalCents != -1400 || secondSummary.NetAfterRecordedExpenses == nil || *secondSummary.NetAfterRecordedExpenses != 1400 || len(secondSummary.ExpenseReversalRows) != 1 || secondSummary.ExpenseReversalRows[0].ExpenseID != entry.ID || secondSummary.ExpenseReversalRows[0].AmountCents != 1400 {
		t.Fatalf("second partial reversal must offset only its own date period: %+v", secondSummary)
	}
}

func TestPlatformExpenseReversalValidationAuthorizationAndAuditFailure(t *testing.T) {
	db := newSyntheticPlatformCostDB(t)
	service := NewAdminService(&config.Config{}, db)
	actor := AuditActor{ID: 88, Type: "super_admin"}
	original, created, err := service.CreatePlatformExpense(validSyntheticExpenseRequest(t), actor)
	if err != nil || !created {
		t.Fatalf("create synthetic source expense: created=%v err=%v", created, err)
	}
	valid := dto.CreatePlatformExpenseReversalRequest{ReversalDate: time.Now().In(time.Local).Format("2006-01-02"), AmountCents: 1000, Reason: "synthetic failure-path reason", IdempotencyKey: uuid.NewString()}
	future := time.Now().In(time.Local).AddDate(0, 0, 1).Format("2006-01-02")
	for _, tc := range []struct {
		name   string
		mutate func(*dto.CreatePlatformExpenseReversalRequest)
	}{
		{"empty reason", func(r *dto.CreatePlatformExpenseReversalRequest) { r.Reason = "   " }},
		{"oversized reason", func(r *dto.CreatePlatformExpenseReversalRequest) { r.Reason = strings.Repeat("中", 501) }},
		{"control character in reason", func(r *dto.CreatePlatformExpenseReversalRequest) { r.Reason = "synthetic\nreason" }},
		{"malformed date", func(r *dto.CreatePlatformExpenseReversalRequest) { r.ReversalDate = "2026-2-01" }},
		{"future date", func(r *dto.CreatePlatformExpenseReversalRequest) { r.ReversalDate = future }},
		{"zero amount", func(r *dto.CreatePlatformExpenseReversalRequest) { r.AmountCents = 0 }},
		{"negative amount", func(r *dto.CreatePlatformExpenseReversalRequest) { r.AmountCents = -1 }},
		{"amount above supported limit", func(r *dto.CreatePlatformExpenseReversalRequest) { r.AmountCents = maxPlatformExpenseCents + 1 }},
		{"invalid idempotency key", func(r *dto.CreatePlatformExpenseReversalRequest) { r.IdempotencyKey = "not-a-uuid" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := valid
			tc.mutate(&request)
			if _, _, err := service.CreatePlatformExpenseReversal(original.ID, request, actor); !errors.Is(err, ErrInvalidPlatformExpense) {
				t.Fatalf("invalid reversal input must be rejected, got %v", err)
			}
		})
	}
	if _, _, err := service.CreatePlatformExpenseReversal(original.ID, valid, AuditActor{ID: actor.ID, Type: "finance"}); !errors.Is(err, ErrInvalidPlatformExpense) {
		t.Fatalf("non-super_admin reversal must be rejected, got %v", err)
	}
	if _, _, err := service.CreatePlatformExpenseReversal(original.ID+9999, valid, actor); !errors.Is(err, ErrPlatformExpenseNotFound) {
		t.Fatalf("reversal of a missing original expense must be rejected, got %v", err)
	}

	if err := db.Exec(`CREATE TRIGGER synthetic_reversal_audit_failure BEFORE INSERT ON audit_logs WHEN NEW.action='platform_expense_reversal_create' BEGIN SELECT RAISE(ABORT, 'synthetic reversal audit failure'); END`).Error; err != nil {
		t.Fatalf("create synthetic audit failure trigger: %v", err)
	}
	if _, _, err := service.CreatePlatformExpenseReversal(original.ID, valid, actor); err == nil {
		t.Fatal("expected a synthetic reversal audit failure to abort the transaction")
	}
	var reversals, reversalAudits int64
	if err := db.Model(&model.PlatformExpenseReversal{}).Count(&reversals).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.AuditLog{}).Where("action = ?", "platform_expense_reversal_create").Count(&reversalAudits).Error; err != nil {
		t.Fatal(err)
	}
	if reversals != 0 || reversalAudits != 0 {
		t.Fatalf("failed audit must roll back reversal and audit together: reversals=%d audits=%d", reversals, reversalAudits)
	}
	period, err := service.PlatformCostOverview()
	if err != nil {
		t.Fatalf("read cost report after aborted reversal: %v", err)
	}
	if period.SelectedPeriod.ActualExpenseCount != 1 || period.SelectedPeriod.ActualExpenseTotalCents == nil || *period.SelectedPeriod.ActualExpenseTotalCents != original.AmountCents || period.SelectedPeriod.ExpenseReversalCount != 0 {
		t.Fatalf("failed reversal changed the original cost report: %+v", period.SelectedPeriod)
	}
}
