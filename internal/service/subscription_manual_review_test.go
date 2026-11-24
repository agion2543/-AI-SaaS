package service

import (
	"fmt"
	"testing"
	"time"

	"gorm.io/gorm"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

func createLegacyManualReviewFixture(t *testing.T, db *gorm.DB) (*model.Merchant, *model.MerchantPlan, *model.Order) {
	t.Helper()
	merchant := &model.Merchant{Name: "合成历史复核商家", Status: "active", SubscriptionStatus: "inactive", SubscriptionPlan: "旧状态"}
	if err := db.Create(merchant).Error; err != nil {
		t.Fatalf("create synthetic merchant: %v", err)
	}
	plan := &model.MerchantPlan{Name: "当前可变套餐", PriceCents: 99900, DurationDays: 365}
	if err := db.Create(plan).Error; err != nil {
		t.Fatalf("create current mutable plan: %v", err)
	}
	order := &model.Order{
		OrderNo:        fmt.Sprintf("SYNTHETIC-LEGACY-SUB-%d", time.Now().UnixNano()),
		OrderType:      "merchant_subscription",
		MerchantID:     &merchant.ID,
		MerchantPlanID: &plan.ID,
		Amount:         19900,
		TotalAmount:    19900,
		Status:         "pending",
		PaymentChannel: "platform_qr",
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create synthetic historical subscription order: %v", err)
	}
	return merchant, plan, order
}

func validManualReviewRequest() dto.ManualReviewLegacyMerchantSubscriptionRequest {
	return dto.ManualReviewLegacyMerchantSubscriptionRequest{
		PlanName:                "原始月付（历史）",
		AmountCents:             19900,
		DurationDays:            30,
		PaymentOccurredAttested: true,
		EvidenceReference:       "synthetic-evidence-reference-001",
		Reason:                  "合成测试：管理员核对了提供的原始历史材料",
	}
}

func platformManualReviewActor() ManualSubscriptionReviewActor {
	return ManualSubscriptionReviewActor{ID: 41, Role: "admin"}
}

func TestManualReviewLegacySubscriptionValidatesRequiredInputs(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*dto.ManualReviewLegacyMerchantSubscriptionRequest)
	}{
		{name: "missing plan name", mutate: func(req *dto.ManualReviewLegacyMerchantSubscriptionRequest) { req.PlanName = "  " }},
		{name: "non-positive amount", mutate: func(req *dto.ManualReviewLegacyMerchantSubscriptionRequest) { req.AmountCents = 0 }},
		{name: "non-positive term", mutate: func(req *dto.ManualReviewLegacyMerchantSubscriptionRequest) { req.DurationDays = 0 }},
		{name: "missing external payment assertion", mutate: func(req *dto.ManualReviewLegacyMerchantSubscriptionRequest) { req.PaymentOccurredAttested = false }},
		{name: "missing evidence reference", mutate: func(req *dto.ManualReviewLegacyMerchantSubscriptionRequest) { req.EvidenceReference = " " }},
		{name: "missing reason", mutate: func(req *dto.ManualReviewLegacyMerchantSubscriptionRequest) { req.Reason = "\n " }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			db := newOrderBoundaryTestDB(t)
			merchant, _, order := createLegacyManualReviewFixture(t, db)
			req := validManualReviewRequest()
			test.mutate(&req)
			if _, err := NewOrderService(db).ReviewLegacyMerchantSubscriptionOrder(order.ID, req, platformManualReviewActor()); err == nil {
				t.Fatal("invalid manual review input was accepted")
			}
			assertManualReviewUnchanged(t, db, merchant.ID, order.ID)
		})
	}
}

func TestManualReviewLegacySubscriptionRequiresPlatformAdmin(t *testing.T) {
	for _, actor := range []ManualSubscriptionReviewActor{{ID: 9, Role: "merchant_admin"}, {ID: 0, Role: "admin"}, {ID: 9, Role: "customer"}} {
		db := newOrderBoundaryTestDB(t)
		merchant, _, order := createLegacyManualReviewFixture(t, db)
		if _, err := NewOrderService(db).ReviewLegacyMerchantSubscriptionOrder(order.ID, validManualReviewRequest(), actor); err == nil {
			t.Fatalf("actor %+v unexpectedly passed platform admin check", actor)
		}
		assertManualReviewUnchanged(t, db, merchant.ID, order.ID)
	}
}

func TestManualReviewLegacySubscriptionRejectsIneligibleOrders(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*testing.T, *gorm.DB, *model.Order)
	}{
		{
			name: "wrong order type",
			mutate: func(t *testing.T, db *gorm.DB, order *model.Order) {
				if err := db.Model(&model.Order{}).Where("id = ?", order.ID).Update("order_type", "store_order").Error; err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "not pending",
			mutate: func(t *testing.T, db *gorm.DB, order *model.Order) {
				if err := db.Model(&model.Order{}).Where("id = ?", order.ID).Update("status", "paid").Error; err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "partial snapshot",
			mutate: func(t *testing.T, db *gorm.DB, order *model.Order) {
				name := "partial only"
				if err := db.Model(&model.Order{}).Where("id = ?", order.ID).Update("merchant_plan_name_snapshot", name).Error; err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "complete snapshot",
			mutate: func(t *testing.T, db *gorm.DB, order *model.Order) {
				name, amount, days := "new order snapshot", order.Amount, 30
				if err := db.Model(&model.Order{}).Where("id = ?", order.ID).Updates(map[string]interface{}{"merchant_plan_name_snapshot": name, "merchant_plan_price_cents_snapshot": amount, "merchant_plan_duration_days_snapshot": days}).Error; err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "stored amount mismatch",
			mutate: func(t *testing.T, db *gorm.DB, order *model.Order) {
				if err := db.Model(&model.Order{}).Where("id = ?", order.ID).Update("total_amount", order.TotalAmount+1).Error; err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "payment marker present",
			mutate: func(t *testing.T, db *gorm.DB, order *model.Order) {
				if err := db.Model(&model.Order{}).Where("id = ?", order.ID).Update("transaction_no", "synthetic-existing-trade").Error; err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "payment record exists",
			mutate: func(t *testing.T, db *gorm.DB, order *model.Order) {
				if err := db.Create(&model.PaymentRecord{OrderID: order.ID, PaymentChannel: "synthetic", TransactionNo: "synthetic-existing-payment", Amount: order.Amount, Status: "pending"}).Error; err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			db := newOrderBoundaryTestDB(t)
			merchant, _, order := createLegacyManualReviewFixture(t, db)
			test.mutate(t, db, order)
			if _, err := NewOrderService(db).ReviewLegacyMerchantSubscriptionOrder(order.ID, validManualReviewRequest(), platformManualReviewActor()); err == nil {
				t.Fatal("ineligible subscription order was accepted")
			}
			var reviews, audits int64
			if err := db.Model(&model.MerchantSubscriptionManualReview{}).Where("order_id = ?", order.ID).Count(&reviews).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Model(&model.MerchantSubscriptionManualReviewAudit{}).Where("order_id = ?", order.ID).Count(&audits).Error; err != nil {
				t.Fatal(err)
			}
			if reviews != 0 || audits != 0 {
				t.Fatalf("ineligible order wrote review/audit: %d/%d", reviews, audits)
			}
			var stored model.Order
			if err := db.First(&stored, order.ID).Error; err != nil {
				t.Fatal(err)
			}
			if stored.Status == "manually_verified" || stored.SubscriptionEndAt != nil {
				t.Fatalf("ineligible order was modified: %+v", stored)
			}
			var storedMerchant model.Merchant
			if err := db.First(&storedMerchant, merchant.ID).Error; err != nil {
				t.Fatal(err)
			}
			if storedMerchant.SubscriptionStatus != "inactive" {
				t.Fatalf("ineligible order changed merchant subscription: %+v", storedMerchant)
			}
		})
	}
}

func TestManualReviewLegacySubscriptionUsesEnteredTermsAndPersistsAtomicAuditWithoutPayment(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	merchant, plan, order := createLegacyManualReviewFixture(t, db)
	oldExpiry := time.Now().UTC().AddDate(0, 0, 5).Truncate(time.Millisecond)
	merchant.SubscriptionExpireAt = &oldExpiry
	merchant.SubscriptionExpiredAt = &oldExpiry
	merchant.SubscriptionStatus = "active"
	if err := db.Save(merchant).Error; err != nil {
		t.Fatalf("save pre-existing subscription state: %v", err)
	}
	// Mutating the current plan cannot alter the administrator-supplied original terms.
	if err := db.Model(&model.MerchantPlan{}).Where("id = ?", plan.ID).Updates(map[string]interface{}{"name": "当前套餐已变更", "price_cents": 99900, "duration_days": 365}).Error; err != nil {
		t.Fatal(err)
	}

	req := validManualReviewRequest()
	result, err := NewOrderService(db).ReviewLegacyMerchantSubscriptionOrder(order.ID, req, platformManualReviewActor())
	if err != nil {
		t.Fatalf("save synthetic manual review: %v", err)
	}
	if result == nil || result.Idempotent || result.Order == nil || result.Review == nil || result.Audit == nil {
		t.Fatalf("incomplete first review result: %+v", result)
	}
	if result.Review.PlanName != req.PlanName || result.Review.AmountCents != req.AmountCents || result.Review.DurationDays != req.DurationDays || !result.Review.PaymentOccurredAttested || result.Review.EvidenceReference != req.EvidenceReference || result.Review.Reason != req.Reason || result.Review.ActorID != 41 || result.Review.ReviewStatus != "manually_verified" {
		t.Fatalf("review record did not preserve exact supplied terms/actor: %+v", result.Review)
	}
	if result.Order.Status != "manually_verified" || result.Order.PaidAt != nil || result.Order.TransactionNo != "" || result.Order.PaymentChannel != "admin_manual_verified" {
		t.Fatalf("review must not claim payment: status=%q paid_at=%v transaction=%q channel=%q", result.Order.Status, result.Order.PaidAt, result.Order.TransactionNo, result.Order.PaymentChannel)
	}
	wantExpiry := oldExpiry.AddDate(0, 0, int(req.DurationDays))
	if result.Order.SubscriptionEndAt == nil || !result.Order.SubscriptionEndAt.Equal(wantExpiry) {
		t.Fatalf("order expiry=%v, want %v", result.Order.SubscriptionEndAt, wantExpiry)
	}
	if result.Order.PlanNameSnapshot != nil || result.Order.PlanPriceCentsSnapshot != nil || result.Order.PlanDurationDaysSnapshot != nil {
		t.Fatal("manual review must not rewrite original snapshot columns")
	}
	if result.Audit.OrderStatusBefore != "pending" || result.Audit.OrderStatusAfter != "manually_verified" || result.Audit.OrderPaidAtAfter != nil || !result.Audit.PaymentOccurredAttested || result.Audit.SubscriptionStatusBefore != "active" || result.Audit.SubscriptionStatusAfter != "active" || result.Audit.SubscriptionPlanAfter != manualReviewSubscriptionPlanMarker || result.Audit.SubscriptionExpireAtBefore == nil || !result.Audit.SubscriptionExpireAtBefore.Equal(oldExpiry) || !result.Audit.SubscriptionExpireAtAfter.Equal(wantExpiry) {
		t.Fatalf("audit missing before/after status or time details: %+v", result.Audit)
	}
	var storedMerchant model.Merchant
	if err := db.First(&storedMerchant, merchant.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedMerchant.SubscriptionStatus != "active" || storedMerchant.SubscriptionExpireAt == nil || !storedMerchant.SubscriptionExpireAt.Equal(wantExpiry) || storedMerchant.SubscriptionPlanID != nil || storedMerchant.SubscriptionPlan != manualReviewSubscriptionPlanMarker {
		t.Fatalf("merchant subscription was not activated from entered terms: %+v", storedMerchant)
	}
	var payments, reviews, audits int64
	if err := db.Model(&model.PaymentRecord{}).Where("order_id = ?", order.ID).Count(&payments).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.MerchantSubscriptionManualReview{}).Where("order_id = ?", order.ID).Count(&reviews).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.MerchantSubscriptionManualReviewAudit{}).Where("order_id = ?", order.ID).Count(&audits).Error; err != nil {
		t.Fatal(err)
	}
	if payments != 0 || reviews != 1 || audits != 1 {
		t.Fatalf("payment/review/audit counts=%d/%d/%d, want 0/1/1", payments, reviews, audits)
	}
}

func TestManualReviewLegacyExpiredStaticQRSubscriptionAfterExpiry(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	merchant, _, order := createLegacyManualReviewFixture(t, db)
	now := time.Now()
	if err := db.Model(&model.Order{}).Where("id = ?", order.ID).Update("created_at", now.Add(-checkoutOrderExpiryDuration-time.Minute)).Error; err != nil {
		t.Fatalf("age synthetic legacy subscription order: %v", err)
	}
	service := NewOrderService(db)
	if count, err := service.ExpireUnpaidCheckoutOrders(now); err != nil || count != 1 {
		t.Fatalf("expire legacy static QR subscription: count=%d err=%v", count, err)
	}
	result, err := service.ReviewLegacyMerchantSubscriptionOrder(order.ID, validManualReviewRequest(), platformManualReviewActor())
	if err != nil {
		t.Fatalf("manually reconcile expired legacy subscription against supplied evidence: %v", err)
	}
	if result.Order.Status != "manually_verified" || result.Order.PaidAt != nil || result.Order.TransactionNo != "" || result.Order.PaymentChannel != "admin_manual_verified" {
		t.Fatalf("historical evidence review must not fabricate channel payment facts: status=%q paid_at=%v transaction=%q channel=%q", result.Order.Status, result.Order.PaidAt, result.Order.TransactionNo, result.Order.PaymentChannel)
	}
	if result.Audit.OrderStatusBefore != "closed" || result.Audit.OrderStatusAfter != "manually_verified" {
		t.Fatalf("audit must preserve expiry-to-manual-review transition: before=%q after=%q", result.Audit.OrderStatusBefore, result.Audit.OrderStatusAfter)
	}
	var paymentCount int64
	if err := db.Model(&model.PaymentRecord{}).Where("order_id = ?", order.ID).Count(&paymentCount).Error; err != nil {
		t.Fatal(err)
	}
	var reviewedMerchant model.Merchant
	if err := db.First(&reviewedMerchant, merchant.ID).Error; err != nil {
		t.Fatal(err)
	}
	if paymentCount != 0 || reviewedMerchant.SubscriptionStatus != "active" || reviewedMerchant.SubscriptionExpireAt == nil {
		t.Fatalf("only the explicit evidence review may update entitlements, with no provider payment row: payments=%d merchant=%+v", paymentCount, reviewedMerchant)
	}
}

func TestManualReviewLegacySubscriptionAuditFailureRollsBackAllEffects(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	merchant, _, order := createLegacyManualReviewFixture(t, db)
	if err := db.Exec(`CREATE TRIGGER fail_manual_review_audit BEFORE INSERT ON merchant_subscription_manual_review_audits BEGIN SELECT RAISE(ABORT, 'synthetic audit failure'); END`).Error; err != nil {
		t.Fatalf("create synthetic audit failure trigger: %v", err)
	}
	if _, err := NewOrderService(db).ReviewLegacyMerchantSubscriptionOrder(order.ID, validManualReviewRequest(), platformManualReviewActor()); err == nil {
		t.Fatal("audit persistence failure did not fail the manual review")
	}
	assertManualReviewUnchanged(t, db, merchant.ID, order.ID)
}

func TestManualReviewLegacySubscriptionDuplicateRequestIsIdempotent(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	merchant, _, order := createLegacyManualReviewFixture(t, db)
	service := NewOrderService(db)
	request := validManualReviewRequest()
	first, err := service.ReviewLegacyMerchantSubscriptionOrder(order.ID, request, platformManualReviewActor())
	if err != nil {
		t.Fatalf("first manual review: %v", err)
	}
	firstExpiry := *first.Order.SubscriptionEndAt
	second, err := service.ReviewLegacyMerchantSubscriptionOrder(order.ID, request, platformManualReviewActor())
	if err != nil {
		t.Fatalf("retry identical manual review: %v", err)
	}
	if !second.Idempotent || second.Review.ID != first.Review.ID || second.Audit.ID != first.Audit.ID || second.Order.SubscriptionEndAt == nil || !second.Order.SubscriptionEndAt.Equal(firstExpiry) {
		t.Fatalf("identical request did not return saved outcome idempotently: %+v", second)
	}
	changed := request
	changed.DurationDays++
	if _, err := service.ReviewLegacyMerchantSubscriptionOrder(order.ID, changed, platformManualReviewActor()); err == nil {
		t.Fatal("different terms unexpectedly replaced an immutable review")
	}
	var payments, reviews, audits int64
	if err := db.Model(&model.PaymentRecord{}).Where("order_id = ?", order.ID).Count(&payments).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.MerchantSubscriptionManualReview{}).Where("order_id = ?", order.ID).Count(&reviews).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.MerchantSubscriptionManualReviewAudit{}).Where("order_id = ?", order.ID).Count(&audits).Error; err != nil {
		t.Fatal(err)
	}
	if payments != 0 || reviews != 1 || audits != 1 {
		t.Fatalf("duplicate request counts payment/review/audit=%d/%d/%d", payments, reviews, audits)
	}
	var storedMerchant model.Merchant
	if err := db.First(&storedMerchant, merchant.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedMerchant.SubscriptionExpireAt == nil || !storedMerchant.SubscriptionExpireAt.Equal(firstExpiry) {
		t.Fatalf("duplicate extended subscription: %v, want %v", storedMerchant.SubscriptionExpireAt, firstExpiry)
	}
}

func assertManualReviewUnchanged(t *testing.T, db *gorm.DB, merchantID, orderID uint) {
	t.Helper()
	var order model.Order
	if err := db.First(&order, orderID).Error; err != nil {
		t.Fatal(err)
	}
	var merchant model.Merchant
	if err := db.First(&merchant, merchantID).Error; err != nil {
		t.Fatal(err)
	}
	var payments, reviews, audits int64
	if err := db.Model(&model.PaymentRecord{}).Where("order_id = ?", orderID).Count(&payments).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.MerchantSubscriptionManualReview{}).Where("order_id = ?", orderID).Count(&reviews).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.MerchantSubscriptionManualReviewAudit{}).Where("order_id = ?", orderID).Count(&audits).Error; err != nil {
		t.Fatal(err)
	}
	if order.Status != "pending" || order.PaidAt != nil || order.TransactionNo != "" || order.SubscriptionEndAt != nil || merchant.SubscriptionStatus != "inactive" || merchant.SubscriptionExpireAt != nil || payments != 0 || reviews != 0 || audits != 0 {
		t.Fatalf("failed review left partial effects: order=%+v merchant=%+v payment/review/audit=%d/%d/%d", order, merchant, payments, reviews, audits)
	}
}
