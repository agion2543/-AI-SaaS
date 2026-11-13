package service

import (
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

func createMerchantSubscriptionTestFixture(t *testing.T, db *gorm.DB) (*model.Merchant, *model.MerchantPlan, *model.Order) {
	t.Helper()

	merchant := &model.Merchant{Name: "订阅快照测试商家", Status: "active", SubscriptionStatus: "inactive"}
	if err := db.Create(merchant).Error; err != nil {
		t.Fatalf("create merchant: %v", err)
	}
	plan := &model.MerchantPlan{Name: "原月付", PriceCents: 19900, DurationDays: 30}
	if err := db.Create(plan).Error; err != nil {
		t.Fatalf("create merchant plan: %v", err)
	}
	fallbackPackage := &model.MembershipPackage{Name: "测试占位套餐", Code: "subscription-test-fallback", DurationDays: 30, Price: 1000, Status: "published"}
	if err := db.Create(fallbackPackage).Error; err != nil {
		t.Fatalf("create fallback package: %v", err)
	}

	order, err := NewMerchantService(nil, db).CreateSubscriptionOrder(0, merchant.ID, dto.CreateMerchantSubscriptionOrderRequest{PlanID: plan.ID})
	if err != nil {
		t.Fatalf("create subscription order: %v", err)
	}
	return merchant, plan, order
}

func TestMerchantSubscriptionOrderSnapshotSurvivesPlanChanges(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	merchant := &model.Merchant{Name: "订阅快照改价商家", Status: "active", SubscriptionStatus: "inactive"}
	if err := db.Create(merchant).Error; err != nil {
		t.Fatalf("create merchant: %v", err)
	}
	plan := &model.MerchantPlan{Name: "原月付", PriceCents: 19900, DurationDays: 30}
	if err := db.Create(plan).Error; err != nil {
		t.Fatalf("create merchant plan: %v", err)
	}
	fallbackPackage := &model.MembershipPackage{Name: "测试占位套餐", Code: "subscription-test-snapshot", DurationDays: 30, Price: 1000, Status: "published"}
	if err := db.Create(fallbackPackage).Error; err != nil {
		t.Fatalf("create fallback package: %v", err)
	}

	order, err := NewMerchantService(nil, db).CreateSubscriptionOrder(0, merchant.ID, dto.CreateMerchantSubscriptionOrderRequest{PlanID: plan.ID})
	if err != nil {
		t.Fatalf("create subscription order: %v", err)
	}
	if order.PlanNameSnapshot == nil || *order.PlanNameSnapshot != "原月付" || order.PlanPriceCentsSnapshot == nil || *order.PlanPriceCentsSnapshot != 19900 || order.PlanDurationDaysSnapshot == nil || *order.PlanDurationDaysSnapshot != 30 {
		t.Fatalf("order snapshots = (%v, %v, %v), want original plan name, price, and 30-day term", order.PlanNameSnapshot, order.PlanPriceCentsSnapshot, order.PlanDurationDaysSnapshot)
	}

	if err := db.Model(&model.MerchantPlan{}).Where("id = ?", plan.ID).Updates(map[string]interface{}{"name": "已改名套餐", "price_cents": 99900, "duration_days": 90}).Error; err != nil {
		t.Fatalf("modify merchant plan: %v", err)
	}
	confirmed, err := NewOrderService(db).ConfirmMerchantSubscriptionPayment(order.ID, "test confirmation")
	if err != nil {
		t.Fatalf("confirm subscription order: %v", err)
	}
	if confirmed.Status != "paid" {
		t.Fatalf("confirmed order status = %q, want paid", confirmed.Status)
	}
	if confirmed.SubscriptionEndAt == nil || confirmed.PaidAt == nil || !confirmed.SubscriptionEndAt.Equal(confirmed.PaidAt.AddDate(0, 0, 30)) {
		t.Fatalf("subscription end = %v, paid at = %v; confirmation must use the original 30-day term", confirmed.SubscriptionEndAt, confirmed.PaidAt)
	}
	var updatedMerchant model.Merchant
	if err := db.First(&updatedMerchant, merchant.ID).Error; err != nil {
		t.Fatalf("load updated merchant: %v", err)
	}
	if updatedMerchant.SubscriptionPlan != "原月付" {
		t.Fatalf("merchant subscription plan = %q, want original plan name", updatedMerchant.SubscriptionPlan)
	}
	var payment model.PaymentRecord
	if err := db.Where("order_id = ?", order.ID).First(&payment).Error; err != nil {
		t.Fatalf("load payment record: %v", err)
	}
	if payment.Amount != 19900 {
		t.Fatalf("payment amount = %d, want snapshotted price 19900", payment.Amount)
	}
}

func TestMerchantSubscriptionCallbackUsesImmutableSnapshotAndIsIdempotent(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	merchant, plan, order := createMerchantSubscriptionTestFixture(t, db)
	if err := db.Model(&model.MerchantPlan{}).Where("id = ?", plan.ID).Updates(map[string]interface{}{
		"name":          "已改名年付",
		"price_cents":   99900,
		"duration_days": 365,
	}).Error; err != nil {
		t.Fatalf("modify current plan: %v", err)
	}
	callback := dto.PaymentCallbackRequest{
		OrderNo:        order.OrderNo,
		TransactionNo:  "synthetic-subscription-callback-trade",
		PaymentChannel: "alipay",
		Amount:         order.TotalAmount,
		Status:         "success",
	}
	service := NewOrderService(db)
	paid, err := service.HandlePaymentCallback(callback)
	if err != nil {
		t.Fatalf("successful subscription callback: %v", err)
	}
	if paid.Status != "paid" || paid.PaidAt == nil || paid.SubscriptionEndAt == nil || !paid.SubscriptionEndAt.Equal(paid.PaidAt.AddDate(0, 0, 30)) {
		t.Fatalf("callback should apply original 30-day snapshot: status=%q paid_at=%v expires=%v", paid.Status, paid.PaidAt, paid.SubscriptionEndAt)
	}
	var updatedMerchant model.Merchant
	if err := db.First(&updatedMerchant, merchant.ID).Error; err != nil {
		t.Fatalf("reload merchant: %v", err)
	}
	if updatedMerchant.SubscriptionPlan != "原月付" || updatedMerchant.SubscriptionStatus != "active" || updatedMerchant.SubscriptionExpireAt == nil || !updatedMerchant.SubscriptionExpireAt.Equal(*paid.SubscriptionEndAt) {
		t.Fatalf("callback did not apply snapshotted merchant subscription: %+v", updatedMerchant)
	}
	var paymentCount int64
	if err := db.Model(&model.PaymentRecord{}).Where("order_id = ?", order.ID).Count(&paymentCount).Error; err != nil {
		t.Fatalf("count callback payment records: %v", err)
	}
	if paymentCount != 1 {
		t.Fatalf("payment records = %d, want one", paymentCount)
	}

	duplicate, err := service.HandlePaymentCallback(callback)
	if err != nil {
		t.Fatalf("repeated identical callback: %v", err)
	}
	if duplicate.SubscriptionEndAt == nil || !duplicate.SubscriptionEndAt.Equal(*paid.SubscriptionEndAt) {
		t.Fatalf("repeat callback changed subscription expiry: got %v, want %v", duplicate.SubscriptionEndAt, paid.SubscriptionEndAt)
	}
	if err := db.Model(&model.PaymentRecord{}).Where("order_id = ?", order.ID).Count(&paymentCount).Error; err != nil {
		t.Fatalf("count payment records after repeat callback: %v", err)
	}
	if paymentCount != 1 {
		t.Fatalf("repeated callback created duplicate payment record; count=%d", paymentCount)
	}
}

func TestMerchantSubscriptionCallbackRejectsMissingOrInvalidSnapshotAtomically(t *testing.T) {
	for _, test := range []struct {
		name   string
		update func(*testing.T, *gorm.DB, *model.Order)
	}{
		{
			name: "legacy order missing snapshot",
			update: func(t *testing.T, db *gorm.DB, order *model.Order) {
				t.Helper()
				if err := db.Model(&model.Order{}).Where("id = ?", order.ID).Updates(map[string]interface{}{
					"merchant_plan_name_snapshot":          nil,
					"merchant_plan_price_cents_snapshot":   nil,
					"merchant_plan_duration_days_snapshot": nil,
				}).Error; err != nil {
					t.Fatalf("remove order snapshots: %v", err)
				}
			},
		},
		{
			name: "snapshot price mismatch",
			update: func(t *testing.T, db *gorm.DB, order *model.Order) {
				t.Helper()
				if err := db.Model(&model.Order{}).Where("id = ?", order.ID).Update("merchant_plan_price_cents_snapshot", order.TotalAmount+1).Error; err != nil {
					t.Fatalf("corrupt snapshot price: %v", err)
				}
			},
		},
		{
			name: "invalid duration snapshot",
			update: func(t *testing.T, db *gorm.DB, order *model.Order) {
				t.Helper()
				if err := db.Model(&model.Order{}).Where("id = ?", order.ID).Update("merchant_plan_duration_days_snapshot", 0).Error; err != nil {
					t.Fatalf("corrupt snapshot duration: %v", err)
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := newOrderBoundaryTestDB(t)
			merchant, _, order := createMerchantSubscriptionTestFixture(t, db)
			test.update(t, db, order)
			_, err := NewOrderService(db).HandlePaymentCallback(dto.PaymentCallbackRequest{
				OrderNo:        order.OrderNo,
				TransactionNo:  "synthetic-invalid-subscription-trade-" + strings.ReplaceAll(test.name, " ", "-"),
				PaymentChannel: "alipay",
				Amount:         order.TotalAmount,
				Status:         "success",
			})
			if err == nil {
				t.Fatal("callback accepted a missing or invalid subscription snapshot")
			}
			var storedOrder model.Order
			if err := db.First(&storedOrder, order.ID).Error; err != nil {
				t.Fatalf("reload subscription order: %v", err)
			}
			var payments int64
			if err := db.Model(&model.PaymentRecord{}).Where("order_id = ?", order.ID).Count(&payments).Error; err != nil {
				t.Fatalf("count payment records: %v", err)
			}
			var storedMerchant model.Merchant
			if err := db.First(&storedMerchant, merchant.ID).Error; err != nil {
				t.Fatalf("reload merchant: %v", err)
			}
			if storedOrder.Status != "pending" || storedOrder.PaidAt != nil || storedOrder.SubscriptionEndAt != nil || payments != 0 {
				t.Fatalf("rejected callback partially changed order/payment state: status=%q paid_at=%v expiry=%v payments=%d", storedOrder.Status, storedOrder.PaidAt, storedOrder.SubscriptionEndAt, payments)
			}
			if storedMerchant.SubscriptionStatus != "inactive" || storedMerchant.SubscriptionExpireAt != nil {
				t.Fatalf("rejected callback partially activated merchant: status=%q expiry=%v", storedMerchant.SubscriptionStatus, storedMerchant.SubscriptionExpireAt)
			}
		})
	}
}

func TestMerchantSubscriptionConfirmationRejectsInvalidOrderAtomically(t *testing.T) {
	for _, test := range []struct {
		name   string
		update func(*testing.T, *gorm.DB, *model.Order)
	}{
		{
			name: "amount mismatch",
			update: func(t *testing.T, db *gorm.DB, order *model.Order) {
				t.Helper()
				if err := db.Model(&model.Order{}).Where("id = ?", order.ID).Update("amount", order.Amount+1).Error; err != nil {
					t.Fatalf("corrupt order amount: %v", err)
				}
			},
		},
		{
			name: "total amount mismatch",
			update: func(t *testing.T, db *gorm.DB, order *model.Order) {
				t.Helper()
				if err := db.Model(&model.Order{}).Where("id = ?", order.ID).Update("total_amount", order.TotalAmount+1).Error; err != nil {
					t.Fatalf("corrupt order total amount: %v", err)
				}
			},
		},
		{
			name: "invalid duration snapshot",
			update: func(t *testing.T, db *gorm.DB, order *model.Order) {
				t.Helper()
				if err := db.Model(&model.Order{}).Where("id = ?", order.ID).Update("merchant_plan_duration_days_snapshot", 0).Error; err != nil {
					t.Fatalf("corrupt order duration snapshot: %v", err)
				}
			},
		},
		{
			name: "missing legacy snapshot",
			update: func(t *testing.T, db *gorm.DB, order *model.Order) {
				t.Helper()
				if err := db.Model(&model.Order{}).Where("id = ?", order.ID).Updates(map[string]interface{}{
					"merchant_plan_name_snapshot":          nil,
					"merchant_plan_price_cents_snapshot":   nil,
					"merchant_plan_duration_days_snapshot": nil,
				}).Error; err != nil {
					t.Fatalf("remove order snapshots: %v", err)
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := newOrderBoundaryTestDB(t)
			merchant, _, order := createMerchantSubscriptionTestFixture(t, db)
			test.update(t, db, order)

			if _, err := NewOrderService(db).ConfirmMerchantSubscriptionPayment(order.ID, "test confirmation"); err == nil {
				t.Fatal("confirmation succeeded for invalid subscription order")
			}
			var payments int64
			if err := db.Model(&model.PaymentRecord{}).Where("order_id = ?", order.ID).Count(&payments).Error; err != nil {
				t.Fatalf("count payment records: %v", err)
			}
			if payments != 0 {
				t.Fatalf("payment records = %d, want 0 after rejected confirmation", payments)
			}
			var storedOrder model.Order
			if err := db.First(&storedOrder, order.ID).Error; err != nil {
				t.Fatalf("reload rejected order: %v", err)
			}
			if storedOrder.Status != "pending" || storedOrder.PaidAt != nil {
				t.Fatalf("rejected order was partially changed: status=%q paid_at=%v", storedOrder.Status, storedOrder.PaidAt)
			}
			var storedMerchant model.Merchant
			if err := db.First(&storedMerchant, merchant.ID).Error; err != nil {
				t.Fatalf("reload merchant: %v", err)
			}
			if storedMerchant.SubscriptionStatus != "inactive" || storedMerchant.SubscriptionExpireAt != nil {
				t.Fatalf("merchant was partially subscribed: status=%q expire_at=%v", storedMerchant.SubscriptionStatus, storedMerchant.SubscriptionExpireAt)
			}
		})
	}
}

func TestMerchantSubscriptionDuplicateConfirmationDoesNotDuplicatePaymentOrExtend(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	merchant, _, order := createMerchantSubscriptionTestFixture(t, db)
	service := NewOrderService(db)

	confirmed, err := service.ConfirmMerchantSubscriptionPayment(order.ID, "first confirmation")
	if err != nil {
		t.Fatalf("first confirmation: %v", err)
	}
	firstExpiry := *confirmed.SubscriptionEndAt
	if _, err := service.ConfirmMerchantSubscriptionPayment(order.ID, "duplicate confirmation"); err == nil {
		t.Fatal("duplicate confirmation unexpectedly succeeded")
	}

	var payments int64
	if err := db.Model(&model.PaymentRecord{}).Where("order_id = ?", order.ID).Count(&payments).Error; err != nil {
		t.Fatalf("count payment records: %v", err)
	}
	if payments != 1 {
		t.Fatalf("payment records = %d, want exactly one", payments)
	}
	var storedOrder model.Order
	if err := db.First(&storedOrder, order.ID).Error; err != nil {
		t.Fatalf("reload order: %v", err)
	}
	if storedOrder.Status != "paid" || storedOrder.SubscriptionEndAt == nil || !storedOrder.SubscriptionEndAt.Equal(firstExpiry) {
		t.Fatalf("duplicate confirmation changed order: status=%q expiry=%v, first expiry=%v", storedOrder.Status, storedOrder.SubscriptionEndAt, firstExpiry)
	}
	var storedMerchant model.Merchant
	if err := db.First(&storedMerchant, merchant.ID).Error; err != nil {
		t.Fatalf("reload merchant: %v", err)
	}
	if storedMerchant.SubscriptionExpireAt == nil || !storedMerchant.SubscriptionExpireAt.Equal(firstExpiry) {
		t.Fatalf("duplicate confirmation extended merchant subscription: %v, want %v", storedMerchant.SubscriptionExpireAt, firstExpiry)
	}
}

func TestMerchantSubscriptionOrdersRemainMerchantScoped(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	owner, _, order := createMerchantSubscriptionTestFixture(t, db)
	other := &model.Merchant{Name: "其他商家", Status: "active", SubscriptionStatus: "inactive"}
	if err := db.Create(other).Error; err != nil {
		t.Fatalf("create other merchant: %v", err)
	}
	service := NewMerchantService(nil, db)
	if order.Status != "pending" {
		t.Fatalf("new subscription order status = %q, want pending", order.Status)
	}
	if active, err := service.CheckSubscription(owner.ID); err != nil || active {
		t.Fatalf("unpaid pending subscription order must not grant access: active=%v err=%v", active, err)
	}
	if _, err := service.SubscriptionOrderStatus(owner.ID, order.OrderNo); err != nil {
		t.Fatalf("owner could not read own subscription order: %v", err)
	}
	if _, err := service.SubscriptionOrderStatus(other.ID, order.OrderNo); err == nil {
		t.Fatal("other merchant could read subscription order status")
	}
	if _, err := service.MarkSubscriptionOrderPaid(other.ID, order.OrderNo); err == nil {
		t.Fatal("other merchant could mark subscription order as paid")
	}
	var storedOrder model.Order
	if err := db.First(&storedOrder, order.ID).Error; err != nil {
		t.Fatalf("reload subscription order: %v", err)
	}
	if storedOrder.Status != "pending" {
		t.Fatalf("cross-merchant request changed order status to %q", storedOrder.Status)
	}
}

func TestOpenMerchantSubscriptionZeroDurationUsesPlanDefault(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	merchant := &model.Merchant{Name: "管理员开通默认期测试商家", Status: "active", SubscriptionStatus: "inactive"}
	if err := db.Create(merchant).Error; err != nil {
		t.Fatalf("create merchant: %v", err)
	}
	plan := &model.MerchantPlan{Name: "月付", PriceCents: 19900, DurationDays: 30}
	if err := db.Create(plan).Error; err != nil {
		t.Fatalf("create merchant plan: %v", err)
	}

	opened, err := NewAdminService(nil, db).OpenMerchantSubscription(merchant.ID, dto.OpenMerchantSubscriptionRequest{Plan: "month", DurationDays: 0})
	if err != nil {
		t.Fatalf("open subscription using default duration: %v", err)
	}
	if opened.SubscriptionExpireAt == nil {
		t.Fatal("subscription expiry was not set")
	}
	if remaining := time.Until(*opened.SubscriptionExpireAt); remaining < 29*24*time.Hour || remaining > 31*24*time.Hour {
		t.Fatalf("default subscription expiry is not approximately 30 days away: %v", remaining)
	}
}

func TestSubscriptionInfoDoesNotPersistLegacyFieldSyncWhenUnavailable(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.SystemConfig{}); err != nil {
		t.Fatalf("migrate system config for subscription info: %v", err)
	}
	plan := model.MerchantPlan{Name: "月付", PriceCents: 100, DurationDays: 30}
	if err := db.Create(&plan).Error; err != nil {
		t.Fatalf("create synthetic plan: %v", err)
	}
	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(time.Hour)
	for _, test := range []struct {
		name   string
		status string
		expiry *time.Time
	}{
		{name: "expired", status: "active", expiry: &past},
		{name: "paused", status: "paused", expiry: &future},
	} {
		t.Run(test.name, func(t *testing.T) {
			merchant := model.Merchant{
				Name:                 "订阅只读映射-" + test.name,
				Status:               "active",
				SubscriptionPlan:     "month",
				SubscriptionStatus:   test.status,
				SubscriptionExpireAt: test.expiry,
			}
			if err := db.Create(&merchant).Error; err != nil {
				t.Fatalf("create synthetic merchant: %v", err)
			}
			info, err := NewMerchantService(nil, db).SubscriptionInfo(merchant.ID)
			if err != nil {
				t.Fatalf("read subscription info: %v", err)
			}
			viewMerchant := info["merchant"].(*model.Merchant)
			if viewMerchant.SubscriptionPlanID == nil || *viewMerchant.SubscriptionPlanID != plan.ID {
				t.Fatalf("in-memory compatibility view did not resolve plan ID: %v", viewMerchant.SubscriptionPlanID)
			}
			var stored model.Merchant
			if err := db.First(&stored, merchant.ID).Error; err != nil {
				t.Fatalf("reload merchant: %v", err)
			}
			if stored.SubscriptionPlanID != nil {
				t.Fatalf("unavailable subscription status read persisted a field sync: plan_id=%v", stored.SubscriptionPlanID)
			}
		})
	}
}
