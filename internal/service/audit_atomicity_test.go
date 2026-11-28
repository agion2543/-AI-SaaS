package service

import (
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

func createAuditTestActor(t *testing.T, db *gorm.DB) AuditActor {
	t.Helper()
	admin := model.AdminUser{Username: "audit-atomicity-admin", DisplayName: "合成审计管理员", Status: "active"}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create synthetic audit actor: %v", err)
	}
	return AuditActor{ID: admin.ID, Type: "admin", IP: "127.0.0.1", UserAgent: "audit-atomicity-test"}
}

func newAuditTestEvent(actor AuditActor, action string, detail map[string]interface{}) AuditEvent {
	return AuditEvent{Actor: actor, Input: AuditRecordInput{Action: action, Detail: detail}}
}

func failAuditInserts(t *testing.T, db *gorm.DB) func() {
	t.Helper()
	const trigger = "synthetic_fail_audit_insert"
	if err := db.Exec("CREATE TRIGGER " + trigger + " BEFORE INSERT ON audit_logs BEGIN SELECT RAISE(ABORT, 'synthetic audit storage failure'); END").Error; err != nil {
		t.Fatalf("install synthetic audit failure trigger: %v", err)
	}
	return func() {
		if err := db.Exec("DROP TRIGGER IF EXISTS " + trigger).Error; err != nil {
			t.Fatalf("remove synthetic audit failure trigger: %v", err)
		}
	}
}

func countAuditLogs(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var count int64
	if err := db.Model(&model.AuditLog{}).Count(&count).Error; err != nil {
		t.Fatalf("count audit logs: %v", err)
	}
	return count
}

func TestSensitiveAdminMutationsRollbackWhenAuditInsertFails(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.AuditLog{}, &model.AdminUser{}, &model.MerchantPaymentConfig{}); err != nil {
		t.Fatalf("migrate audit regression schema: %v", err)
	}
	actor := createAuditTestActor(t, db)
	merchant := model.Merchant{Name: "合成原子性商家", Status: "active", SubscriptionStatus: "inactive"}
	if err := db.Create(&merchant).Error; err != nil {
		t.Fatal(err)
	}
	plan := model.MerchantPlan{Name: "月付", PriceCents: 19900, DurationDays: 30}
	if err := db.Create(&plan).Error; err != nil {
		t.Fatal(err)
	}
	user := model.User{Email: "audit-atomicity@example.test", DisplayName: "合成会员", Status: "active", MemberLevel: "free", RemainingQuota: 4}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	paymentConfig := model.MerchantPaymentConfig{MerchantID: merchant.ID, AccountName: "合成收款账户", Status: "disabled", AuditStatus: "pending"}
	if err := db.Create(&paymentConfig).Error; err != nil {
		t.Fatal(err)
	}
	admin := NewAdminService(nil, db)
	merchantService := NewMerchantService(nil, db)

	dropFailure := failAuditInserts(t, db)
	if err := admin.UpdateMerchantStatus(merchant.ID, "suspended", newAuditTestEvent(actor, "merchant_status_update", nil)); err == nil {
		t.Fatal("merchant status change succeeded despite audit storage failure")
	}
	if _, err := admin.OpenMerchantSubscription(merchant.ID, dto.OpenMerchantSubscriptionRequest{Plan: "month"}, newAuditTestEvent(actor, "merchant_subscription_open", nil)); err == nil {
		t.Fatal("subscription opening succeeded despite audit storage failure")
	}
	if err := admin.UpdateUserStatus(user.ID, "disabled", newAuditTestEvent(actor, "user_status_update", nil)); err == nil {
		t.Fatal("user status change succeeded despite audit storage failure")
	}
	memberRequest := dto.ManualOpenMemberRequest{DurationDays: 7, QuotaDelta: 2, AutoRenew: true}
	if _, err := admin.AdjustUserMember(user.ID, memberRequest, newAuditTestEvent(actor, "user_member_adjust", nil)); err == nil {
		t.Fatal("member entitlement change succeeded despite audit storage failure")
	}
	if _, err := merchantService.ReviewPaymentConfig(merchant.ID, dto.ReviewMerchantPaymentConfigRequest{AuditStatus: "approved", AuditRemark: "synthetic approval"}, newAuditTestEvent(actor, "merchant_payment_config_review", nil)); err == nil {
		t.Fatal("payment configuration approval succeeded despite audit storage failure")
	}
	var storedMerchant model.Merchant
	if err := db.First(&storedMerchant, merchant.ID).Error; err != nil {
		t.Fatal(err)
	}
	var storedUser model.User
	if err := db.First(&storedUser, user.ID).Error; err != nil {
		t.Fatal(err)
	}
	var storedConfig model.MerchantPaymentConfig
	if err := db.First(&storedConfig, paymentConfig.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedMerchant.Status != "active" || storedMerchant.SubscriptionStatus != "inactive" || storedMerchant.SubscriptionExpireAt != nil {
		t.Fatalf("audit failure left merchant mutation: %+v", storedMerchant)
	}
	if storedUser.Status != "active" || storedUser.MemberLevel != "free" || storedUser.RemainingQuota != 4 || storedUser.ExpiredAt != nil || storedUser.AutoRenew {
		t.Fatalf("audit failure left user/entitlement mutation: %+v", storedUser)
	}
	if storedConfig.Status != "disabled" || storedConfig.AuditStatus != "pending" {
		t.Fatalf("audit failure left payment configuration mutation: %+v", storedConfig)
	}
	if countAuditLogs(t, db) != 0 {
		t.Fatal("failed audit inserts unexpectedly left audit rows")
	}

	dropFailure()
	if err := admin.UpdateMerchantStatus(merchant.ID, "suspended", newAuditTestEvent(actor, "merchant_status_update", nil)); err != nil {
		t.Fatalf("successful audited merchant status change: %v", err)
	}
	if _, err := admin.OpenMerchantSubscription(merchant.ID, dto.OpenMerchantSubscriptionRequest{Plan: "month"}, newAuditTestEvent(actor, "merchant_subscription_open", nil)); err != nil {
		t.Fatalf("successful audited subscription opening: %v", err)
	}
	var activeSubscription model.Merchant
	if err := db.First(&activeSubscription, merchant.ID).Error; err != nil {
		t.Fatal(err)
	}
	dropFailure = failAuditInserts(t, db)
	if _, err := admin.StopMerchantSubscription(merchant.ID, newAuditTestEvent(actor, "merchant_subscription_stop", nil)); err == nil {
		t.Fatal("subscription stop succeeded despite audit storage failure")
	}
	var afterFailedStop model.Merchant
	if err := db.First(&afterFailedStop, merchant.ID).Error; err != nil {
		t.Fatal(err)
	}
	if afterFailedStop.SubscriptionStatus != "active" || afterFailedStop.SubscriptionExpireAt == nil || activeSubscription.SubscriptionExpireAt == nil || !afterFailedStop.SubscriptionExpireAt.Equal(*activeSubscription.SubscriptionExpireAt) {
		t.Fatalf("audit failure left subscription stop mutation: before=%+v after=%+v", activeSubscription, afterFailedStop)
	}
	dropFailure()
	if _, err := admin.StopMerchantSubscription(merchant.ID, newAuditTestEvent(actor, "merchant_subscription_stop", nil)); err != nil {
		t.Fatalf("successful audited subscription stop: %v", err)
	}
	if err := admin.UpdateUserStatus(user.ID, "disabled", newAuditTestEvent(actor, "user_status_update", nil)); err != nil {
		t.Fatalf("successful audited user status change: %v", err)
	}
	if _, err := admin.AdjustUserMember(user.ID, memberRequest, newAuditTestEvent(actor, "user_member_adjust", nil)); err != nil {
		t.Fatalf("successful audited membership change: %v", err)
	}
	if _, err := merchantService.ReviewPaymentConfig(merchant.ID, dto.ReviewMerchantPaymentConfigRequest{AuditStatus: "approved", AuditRemark: "synthetic approval"}, newAuditTestEvent(actor, "merchant_payment_config_review", nil)); err != nil {
		t.Fatalf("successful audited payment configuration approval: %v", err)
	}
	if countAuditLogs(t, db) != 6 {
		t.Fatalf("successful operations produced %d audit rows, want 6", countAuditLogs(t, db))
	}
}

func TestFinancialSettlementMutationAndAuditAreAtomic(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.AuditLog{}, &model.AdminUser{}); err != nil {
		t.Fatalf("migrate audit tables: %v", err)
	}
	actor := createAuditTestActor(t, db)
	merchant := model.Merchant{Name: "合成结算商家", Status: "active"}
	if err := db.Create(&merchant).Error; err != nil {
		t.Fatal(err)
	}
	paidAt := time.Now().UTC()
	merchantID := merchant.ID
	order := model.Order{OrderNo: "SYNTHETIC-AUDIT-SETTLEMENT", OrderType: "store_order", MerchantID: &merchantID, Amount: 1700, TotalAmount: 1700, Status: "accepted", PaidAt: &paidAt}
	if err := db.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	admin := NewAdminService(nil, db)

	dropFailure := failAuditInserts(t, db)
	if _, _, err := admin.CreateMerchantSettlement(merchant.ID, dto.CreateMerchantSettlementRequest{Remark: "synthetic audit test"}, newAuditTestEvent(actor, "merchant_settlement_create", nil)); err == nil {
		t.Fatal("settlement creation succeeded despite audit storage failure")
	}
	var afterOrder model.Order
	if err := db.First(&afterOrder, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	var settlements int64
	if err := db.Model(&model.MerchantSettlement{}).Count(&settlements).Error; err != nil {
		t.Fatal(err)
	}
	if afterOrder.SettlementID != nil || settlements != 0 {
		t.Fatalf("audit failure left settlement mutation: order settlement=%v settlements=%d", afterOrder.SettlementID, settlements)
	}
	dropFailure()
	settlement, settledOrders, err := admin.CreateMerchantSettlement(merchant.ID, dto.CreateMerchantSettlementRequest{Remark: "synthetic audit test"}, newAuditTestEvent(actor, "merchant_settlement_create", nil))
	if err != nil {
		t.Fatalf("successful audited settlement creation: %v", err)
	}
	if settlement.ID == 0 || len(settledOrders) != 1 || settledOrders[0].ID != order.ID || countAuditLogs(t, db) != 1 {
		t.Fatalf("successful settlement missing linked order or audit: settlement=%+v orders=%+v audit_count=%d", settlement, settledOrders, countAuditLogs(t, db))
	}

	dropFailure = failAuditInserts(t, db)
	if _, err := admin.MarkMerchantSettlementPaid(settlement.ID, dto.MarkMerchantSettlementPaidRequest{Remark: "synthetic paid"}, newAuditTestEvent(actor, "merchant_settlement_paid", nil)); err == nil {
		t.Fatal("settlement paid mark succeeded despite audit storage failure")
	}
	if err := db.First(&settlement, settlement.ID).Error; err != nil {
		t.Fatal(err)
	}
	if settlement.Status != "pending" || settlement.PaidAt != nil || countAuditLogs(t, db) != 1 {
		t.Fatalf("audit failure left settlement paid: status=%q paid_at=%v audits=%d", settlement.Status, settlement.PaidAt, countAuditLogs(t, db))
	}
	dropFailure()
	if _, err := admin.MarkMerchantSettlementPaid(settlement.ID, dto.MarkMerchantSettlementPaidRequest{Remark: "synthetic paid"}, newAuditTestEvent(actor, "merchant_settlement_paid", nil)); err != nil {
		t.Fatalf("successful audited settlement payment mark: %v", err)
	}
	if err := db.First(&settlement, settlement.ID).Error; err != nil {
		t.Fatal(err)
	}
	if settlement.Status != "paid" || settlement.PaidAt == nil || countAuditLogs(t, db) != 2 {
		t.Fatalf("successful paid mark missing state/audit: status=%q paid_at=%v audits=%d", settlement.Status, settlement.PaidAt, countAuditLogs(t, db))
	}
}

func TestSubscriptionPaymentConfirmationRollsBackWhenAuditInsertFails(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.AuditLog{}, &model.AdminUser{}); err != nil {
		t.Fatalf("migrate audit tables: %v", err)
	}
	actor := createAuditTestActor(t, db)
	merchant, _, order := createMerchantSubscriptionTestFixture(t, db)
	dropFailure := failAuditInserts(t, db)
	_, err := NewOrderService(db).ConfirmMerchantSubscriptionPayment(order.ID, "synthetic audit test", newAuditTestEvent(actor, "merchant_subscription_payment_confirm", nil))
	if err == nil || !strings.Contains(err.Error(), "persist subscription payment audit") {
		t.Fatalf("confirmation error=%v, want audit failure", err)
	}
	var afterOrder model.Order
	var afterMerchant model.Merchant
	if err := db.First(&afterOrder, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&afterMerchant, merchant.ID).Error; err != nil {
		t.Fatal(err)
	}
	var payments int64
	if err := db.Model(&model.PaymentRecord{}).Where("order_id = ?", order.ID).Count(&payments).Error; err != nil {
		t.Fatal(err)
	}
	if afterOrder.Status != "pending" || afterOrder.PaidAt != nil || afterMerchant.SubscriptionStatus != "inactive" || payments != 0 || countAuditLogs(t, db) != 0 {
		t.Fatalf("failed audit left partial payment confirmation: order=%q merchant=%q payments=%d audits=%d", afterOrder.Status, afterMerchant.SubscriptionStatus, payments, countAuditLogs(t, db))
	}

	dropFailure()
	if _, err := NewOrderService(db).ConfirmMerchantSubscriptionPayment(order.ID, "synthetic audit success", newAuditTestEvent(actor, "merchant_subscription_payment_confirm", nil)); err != nil {
		t.Fatalf("successful audited subscription payment confirmation: %v", err)
	}
	if countAuditLogs(t, db) != 1 {
		t.Fatalf("successful confirmation audit count=%d, want 1", countAuditLogs(t, db))
	}
}

func TestRefundAuditIntentFailurePreventsProviderDispatch(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.AuditLog{}, &model.AdminUser{}); err != nil {
		t.Fatalf("migrate audit tables: %v", err)
	}
	actor := createAuditTestActor(t, db)
	paidAt := time.Now().UTC()
	merchant, _, _, order := createSyntheticStoreOrder(t, db, "SYNTHETIC-AUDIT-REFUND", "accepted", 1200, &paidAt)
	order.TransactionNo = "synthetic-paid-transaction"
	if err := db.Save(&order).Error; err != nil {
		t.Fatal(err)
	}
	providerCalls := 0
	orderService := NewOrderService(db, newLocalAlipayRefundService(t, &providerCalls))
	event := newAuditTestEvent(actor, "merchant_order_refund", map[string]interface{}{"reason": "synthetic regression"})

	dropFailure := failAuditInserts(t, db)
	_, _, err := orderService.RefundMerchantOrder(merchant.ID, order.ID, dto.RefundOrderRequest{Amount: 300, Reason: "synthetic regression"}, 42, "merchant", event)
	if err == nil || !strings.Contains(err.Error(), "persist refund dispatch audit intent") {
		t.Fatalf("refund error=%v, want audit-intent failure", err)
	}
	var afterOrder model.Order
	if err := db.First(&afterOrder, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	var refunds int64
	if err := db.Model(&model.RefundRecord{}).Where("order_id = ?", order.ID).Count(&refunds).Error; err != nil {
		t.Fatal(err)
	}
	if providerCalls != 0 || refunds != 0 || afterOrder.RefundedAmount != 0 || afterOrder.RefundStatus == "processing" || countAuditLogs(t, db) != 0 {
		t.Fatalf("audit failure did not block refund dispatch/rollback: provider_calls=%d reservations=%d refunded=%d status=%q audits=%d", providerCalls, refunds, afterOrder.RefundedAmount, afterOrder.RefundStatus, countAuditLogs(t, db))
	}

	dropFailure()
	if _, _, err := orderService.RefundMerchantOrder(merchant.ID, order.ID, dto.RefundOrderRequest{Amount: 300, Reason: "synthetic regression"}, 42, "merchant", event); err != nil {
		t.Fatalf("successful audited refund request: %v", err)
	}
	if providerCalls != 1 || countAuditLogs(t, db) != 1 {
		t.Fatalf("refund success counts: provider_calls=%d audits=%d, want 1/1", providerCalls, countAuditLogs(t, db))
	}
	var log model.AuditLog
	if err := db.Where("action = ?", "merchant_order_refund").First(&log).Error; err != nil {
		t.Fatal(err)
	}
	if log.Detail != `{"purpose":"商家退款处理"}` || strings.Contains(log.Detail, "synthetic regression") {
		t.Fatalf("refund audit must retain only the purpose and omit free-text reason: %s", log.Detail)
	}
}
