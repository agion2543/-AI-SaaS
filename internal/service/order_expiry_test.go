package service

import (
	"testing"
	"time"

	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

func TestExpireUnpaidCheckoutOrdersAtThirtyMinuteDeadline(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	now := time.Date(2026, time.November, 23, 14, 0, 0, 0, time.UTC)
	paidAt := now.Add(-time.Minute)
	orders := []model.Order{
		{BaseModel: model.BaseModel{CreatedAt: now.Add(-checkoutOrderExpiryDuration)}, OrderNo: "SYNTHETIC-EXPIRY-BOUNDARY", OrderType: "user_membership", TotalAmount: 1000, Amount: 1000, PaymentChannel: "alipay", Status: "pending"},
		{BaseModel: model.BaseModel{CreatedAt: now.Add(-checkoutOrderExpiryDuration - time.Second)}, OrderNo: "SYNTHETIC-EXPIRY-FAILED", OrderType: "store_order", TotalAmount: 1000, Amount: 1000, PaymentChannel: "alipay", Status: "failed"},
		{BaseModel: model.BaseModel{CreatedAt: now.Add(-checkoutOrderExpiryDuration + time.Second)}, OrderNo: "SYNTHETIC-EXPIRY-STILL-FRESH", OrderType: "user_membership", TotalAmount: 1000, Amount: 1000, PaymentChannel: "alipay", Status: "pending"},
		{BaseModel: model.BaseModel{CreatedAt: now.Add(-checkoutOrderExpiryDuration - time.Hour)}, OrderNo: "SYNTHETIC-EXPIRY-STATIC-QR", OrderType: "merchant_subscription", TotalAmount: 1000, Amount: 1000, PaymentChannel: "platform_qr", Status: "pending"},
		{BaseModel: model.BaseModel{CreatedAt: now.Add(-checkoutOrderExpiryDuration - time.Hour)}, OrderNo: "SYNTHETIC-EXPIRY-PAID", OrderType: "user_membership", TotalAmount: 1000, Amount: 1000, PaymentChannel: "alipay", Status: "pending", PaidAt: &paidAt},
	}
	for i := range orders {
		if err := db.Create(&orders[i]).Error; err != nil {
			t.Fatalf("create synthetic order %q: %v", orders[i].OrderNo, err)
		}
	}

	service := NewOrderService(db)
	expired, err := service.ExpireUnpaidCheckoutOrders(now)
	if err != nil {
		t.Fatalf("expire unpaid checkout orders: %v", err)
	}
	if expired != 3 {
		t.Fatalf("expired count = %d, want 3 including static QR subscription checkout", expired)
	}
	for _, test := range []struct {
		orderNo string
		status  string
		logged  bool
	}{
		{"SYNTHETIC-EXPIRY-BOUNDARY", "closed", true},
		{"SYNTHETIC-EXPIRY-FAILED", "closed", true},
		{"SYNTHETIC-EXPIRY-STILL-FRESH", "pending", false},
		{"SYNTHETIC-EXPIRY-STATIC-QR", "closed", true},
		{"SYNTHETIC-EXPIRY-PAID", "pending", false},
	} {
		var current model.Order
		if err := db.Where("order_no = ?", test.orderNo).First(&current).Error; err != nil {
			t.Fatalf("reload synthetic order %q: %v", test.orderNo, err)
		}
		if current.Status != test.status || orderHasAction(current.OperationLogs, "checkout_expired") != test.logged {
			t.Errorf("order %q status=%q expiry_log=%t, want status=%q expiry_log=%t", test.orderNo, current.Status, orderHasAction(current.OperationLogs, "checkout_expired"), test.status, test.logged)
		}
	}
	if repeated, err := service.ExpireUnpaidCheckoutOrders(now); err != nil || repeated != 0 {
		t.Fatalf("repeated expiry sweep should be idempotent: count=%d err=%v", repeated, err)
	}
}

func TestPublicOrderDetailClosesExpiredCheckoutOnRefresh(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	_, _, _, order := createSyntheticStoreOrder(t, db, "SYNTHETIC-EXPIRED-CUSTOMER-REFRESH", "pending", 900, nil)
	accessToken := "synthetic-customer-expiry-refresh-token"
	if err := db.Model(&model.Order{}).Where("id = ?", order.ID).Updates(map[string]interface{}{
		"created_at":                 time.Now().Add(-checkoutOrderExpiryDuration),
		"customer_access_token_hash": hashCustomerOrderAccessToken(accessToken),
	}).Error; err != nil {
		t.Fatalf("age and authorize synthetic order: %v", err)
	}

	refreshed, err := NewOrderService(db).PublicOrderDetail(order.OrderNo, accessToken)
	if err != nil {
		t.Fatalf("refresh expired customer order: %v", err)
	}
	if refreshed.Status != "closed" || refreshed.PaidAt != nil || !orderHasAction(refreshed.OperationLogs, "checkout_expired") {
		t.Fatalf("expired customer refresh should return closed/unpaid state: status=%q paid_at=%v logs=%s", refreshed.Status, refreshed.PaidAt, refreshed.OperationLogs)
	}
	var paymentCount int64
	if err := db.Model(&model.PaymentRecord{}).Where("order_id = ?", order.ID).Count(&paymentCount).Error; err != nil {
		t.Fatal(err)
	}
	if paymentCount != 0 {
		t.Fatalf("customer refresh must not confirm or record payment, got %d records", paymentCount)
	}
}

func TestExpiredCheckoutCannotBeRetriedBeforeSweep(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	_, _, _, order := createSyntheticStoreOrder(t, db, "SYNTHETIC-EXPIRED-RETRY", "failed", 900, nil)
	accessToken := "synthetic-expired-retry-access-token"
	if err := db.Model(&model.Order{}).Where("id = ?", order.ID).Updates(map[string]interface{}{
		"created_at":                 time.Now().Add(-checkoutOrderExpiryDuration - time.Second),
		"customer_access_token_hash": hashCustomerOrderAccessToken(accessToken),
	}).Error; err != nil {
		t.Fatalf("age and authorize synthetic retry order: %v", err)
	}
	if _, err := NewOrderService(db).PrepareCustomerOrderRetry(order.OrderNo, accessToken); err == nil {
		t.Fatal("expired checkout order must not be retried before the scheduled sweep")
	}
	var closed model.Order
	if err := db.First(&closed, order.ID).Error; err != nil {
		t.Fatalf("reload expired retry order: %v", err)
	}
	if closed.Status != "closed" || !orderHasAction(closed.OperationLogs, "checkout_expired") {
		t.Fatalf("retry request after expiry must close and audit order: status=%q logs=%s", closed.Status, closed.OperationLogs)
	}
}

func TestLateVerifiedSuccessIsPreservedForManualReconciliationAndIdempotent(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	_, _, product, order := createSyntheticStoreOrder(t, db, "SYNTHETIC-LATE-SUCCESS-001", "pending", 900, nil)
	createdAt := time.Now().Add(-checkoutOrderExpiryDuration - time.Minute)
	if err := db.Model(&model.Order{}).Where("id = ?", order.ID).Update("created_at", createdAt).Error; err != nil {
		t.Fatalf("age synthetic checkout order: %v", err)
	}
	service := NewOrderService(db)
	callback := dto.PaymentCallbackRequest{
		OrderNo:        order.OrderNo,
		TransactionNo:  "synthetic-late-success-trade",
		PaymentChannel: "alipay",
		Amount:         order.TotalAmount,
		Status:         "success",
	}

	first, err := service.HandlePaymentCallback(callback)
	if err != nil {
		t.Fatalf("process verified late success callback: %v", err)
	}
	if first.Status != "closed" || first.PaidAt == nil || first.TransactionNo != callback.TransactionNo {
		t.Fatalf("late success must preserve the closed order and verified payment fact: status=%q paid_at=%v transaction=%q", first.Status, first.PaidAt, first.TransactionNo)
	}
	if !orderHasAction(first.OperationLogs, "checkout_expired") || !orderHasAction(first.OperationLogs, "late_payment_exception") {
		t.Fatalf("late success must record expiry and manual reconciliation flags: %s", first.OperationLogs)
	}
	if orderHasAction(first.OperationLogs, "stock_deducted") {
		t.Fatalf("late payment reconciliation must not create a stock deduction entry: %s", first.OperationLogs)
	}

	var payment model.PaymentRecord
	if err := db.Where("transaction_no = ?", callback.TransactionNo).First(&payment).Error; err != nil {
		t.Fatalf("load preserved late-success payment record: %v", err)
	}
	if payment.Status != model.PaymentStatusPaidReviewRequired || payment.Amount != order.TotalAmount || payment.PaymentChannel != "alipay" {
		t.Fatalf("late provider success not retained as a reconciliation exception: %+v", payment)
	}
	if len(payment.Status) > 20 {
		t.Fatalf("manual-review status exceeds the existing VARCHAR(20): %q", payment.Status)
	}
	var updatedProduct model.StoreProduct
	if err := db.First(&updatedProduct, product.ID).Error; err != nil {
		t.Fatal(err)
	}
	if updatedProduct.Stock == nil || *updatedProduct.Stock != 10 {
		t.Fatalf("late payment must not trigger fulfillment or deduct stock: %v", updatedProduct.Stock)
	}
	var refundCount int64
	if err := db.Model(&model.RefundRecord{}).Where("order_id = ?", order.ID).Count(&refundCount).Error; err != nil {
		t.Fatal(err)
	}
	if refundCount != 0 {
		t.Fatalf("late payment must not trigger an automatic refund, refunds=%d", refundCount)
	}

	duplicate, err := service.HandlePaymentCallback(callback)
	if err != nil {
		t.Fatalf("repeat late-success callback: %v", err)
	}
	var paymentCount int64
	if err := db.Model(&model.PaymentRecord{}).Where("order_id = ?", order.ID).Count(&paymentCount).Error; err != nil {
		t.Fatal(err)
	}
	if duplicate.Status != "closed" || duplicate.PaidAt == nil || paymentCount != 1 {
		t.Fatalf("duplicate late callback must remain idempotent: status=%q paid_at=%v payment_rows=%d", duplicate.Status, duplicate.PaidAt, paymentCount)
	}
}

func TestLateSubscriptionSuccessIsRecordedWithoutActivatingEntitlements(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	merchant, _, order := createMerchantSubscriptionTestFixture(t, db)
	createdAt := time.Now().Add(-checkoutOrderExpiryDuration - time.Minute)
	if err := db.Model(&model.Order{}).Where("id = ?", order.ID).Updates(map[string]interface{}{
		"payment_channel": "alipay",
		"created_at":      createdAt,
	}).Error; err != nil {
		t.Fatalf("age synthetic subscription checkout order: %v", err)
	}

	confirmed, err := NewOrderService(db).HandlePaymentCallback(dto.PaymentCallbackRequest{
		OrderNo:        order.OrderNo,
		TransactionNo:  "synthetic-late-subscription-trade",
		PaymentChannel: "alipay",
		Amount:         order.TotalAmount,
		Status:         "success",
	})
	if err != nil {
		t.Fatalf("record verified late subscription payment: %v", err)
	}
	if confirmed.Status != "closed" || confirmed.PaidAt == nil || confirmed.SubscriptionEndAt != nil || !orderHasAction(confirmed.OperationLogs, "late_payment_exception") {
		t.Fatalf("late subscription payment must remain paid-but-closed for review, without an entitlement end date: status=%q paid_at=%v subscription_end=%v logs=%s", confirmed.Status, confirmed.PaidAt, confirmed.SubscriptionEndAt, confirmed.OperationLogs)
	}
	var payment model.PaymentRecord
	if err := db.Where("order_id = ?", order.ID).First(&payment).Error; err != nil {
		t.Fatalf("load late subscription payment record: %v", err)
	}
	if payment.Status != model.PaymentStatusPaidReviewRequired {
		t.Fatalf("late subscription payment status=%q, want paid review required", payment.Status)
	}
	var currentMerchant model.Merchant
	if err := db.First(&currentMerchant, merchant.ID).Error; err != nil {
		t.Fatalf("reload subscription merchant: %v", err)
	}
	if currentMerchant.SubscriptionStatus != "inactive" || currentMerchant.SubscriptionPlanID != nil || currentMerchant.SubscriptionExpireAt != nil {
		t.Fatalf("late payment must not activate merchant subscription entitlements: status=%q plan=%v expiry=%v", currentMerchant.SubscriptionStatus, currentMerchant.SubscriptionPlanID, currentMerchant.SubscriptionExpireAt)
	}
}

func TestStaticQRSubscriptionCallbackRemainsManualOnly(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	merchant, _, order := createMerchantSubscriptionTestFixture(t, db)
	if err := db.Model(&model.Order{}).Where("id = ?", order.ID).Update("payment_channel", "platform_qr").Error; err != nil {
		t.Fatalf("set synthetic static platform QR channel: %v", err)
	}
	callback := dto.PaymentCallbackRequest{
		OrderNo:        order.OrderNo,
		TransactionNo:  "synthetic-static-qr-should-not-auto-confirm",
		PaymentChannel: "alipay",
		Amount:         order.TotalAmount,
		Status:         "success",
	}
	if _, err := NewOrderService(db).HandlePaymentCallback(callback); err == nil {
		t.Fatal("static platform QR subscription payment must remain subject to manual reconciliation")
	}
	var unchanged model.Order
	if err := db.First(&unchanged, order.ID).Error; err != nil {
		t.Fatalf("reload static QR subscription order: %v", err)
	}
	var paymentCount int64
	if err := db.Model(&model.PaymentRecord{}).Where("order_id = ?", order.ID).Count(&paymentCount).Error; err != nil {
		t.Fatal(err)
	}
	var currentMerchant model.Merchant
	if err := db.First(&currentMerchant, merchant.ID).Error; err != nil {
		t.Fatal(err)
	}
	if unchanged.Status != "pending" || unchanged.PaidAt != nil || paymentCount != 0 || currentMerchant.SubscriptionStatus != "inactive" || currentMerchant.SubscriptionPlanID != nil || currentMerchant.SubscriptionExpireAt != nil {
		t.Fatalf("static QR callback attempt must not record automatic payment or activate benefits: order=%+v payments=%d merchant_status=%q plan=%v expiry=%v", unchanged, paymentCount, currentMerchant.SubscriptionStatus, currentMerchant.SubscriptionPlanID, currentMerchant.SubscriptionExpireAt)
	}
}

func TestExpiredStaticQRSubscriptionRequiresAdminReconciliation(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	merchant, _, order := createMerchantSubscriptionTestFixture(t, db)
	now := time.Now()
	if err := db.Model(&model.Order{}).Where("id = ?", order.ID).Updates(map[string]interface{}{
		"payment_channel": "platform_qr",
		"created_at":      now.Add(-checkoutOrderExpiryDuration - time.Minute),
	}).Error; err != nil {
		t.Fatalf("age synthetic static QR subscription order: %v", err)
	}
	merchantService := NewMerchantService(&config.Config{}, db)
	marked, err := merchantService.MarkSubscriptionOrderPaid(merchant.ID, order.OrderNo)
	if err != nil {
		t.Fatalf("record synthetic merchant payment reminder: %v", err)
	}
	if marked.Status != "pending" || marked.PaidAt != nil || !orderHasAction(marked.OperationLogs, "merchant_subscription_paid_marked") {
		t.Fatalf("merchant self-report must remain a reminder, not payment confirmation: status=%q paid_at=%v logs=%s", marked.Status, marked.PaidAt, marked.OperationLogs)
	}

	service := NewOrderService(db)
	if count, err := service.ExpireUnpaidCheckoutOrders(now); err != nil || count != 1 {
		t.Fatalf("expire static QR subscription checkout: count=%d err=%v", count, err)
	}
	var expired model.Order
	if err := db.First(&expired, order.ID).Error; err != nil {
		t.Fatalf("reload expired static QR subscription: %v", err)
	}
	if expired.Status != "closed" || expired.PaidAt != nil || !orderHasAction(expired.OperationLogs, "checkout_expired") {
		t.Fatalf("static QR subscription must close as explicitly unpaid: status=%q paid_at=%v logs=%s", expired.Status, expired.PaidAt, expired.OperationLogs)
	}
	var paymentCount int64
	if err := db.Model(&model.PaymentRecord{}).Where("order_id = ?", order.ID).Count(&paymentCount).Error; err != nil {
		t.Fatal(err)
	}
	var beforeAdmin model.Merchant
	if err := db.First(&beforeAdmin, merchant.ID).Error; err != nil {
		t.Fatal(err)
	}
	if paymentCount != 0 || beforeAdmin.SubscriptionStatus != "inactive" || beforeAdmin.SubscriptionPlanID != nil || beforeAdmin.SubscriptionExpireAt != nil {
		t.Fatalf("expiry must not verify payment or activate benefits: payments=%d status=%q plan=%v expire=%v", paymentCount, beforeAdmin.SubscriptionStatus, beforeAdmin.SubscriptionPlanID, beforeAdmin.SubscriptionExpireAt)
	}

	callback := dto.PaymentCallbackRequest{
		OrderNo: order.OrderNo, TransactionNo: "synthetic-expired-static-qr-callback",
		PaymentChannel: "alipay", Amount: order.TotalAmount, Status: "success",
	}
	if _, err := service.HandlePaymentCallback(callback); err == nil {
		t.Fatal("static QR callback must not auto-confirm an expired subscription order")
	}
	if _, err := merchantService.MarkSubscriptionOrderPaid(merchant.ID, order.OrderNo); err == nil {
		t.Fatal("merchant self-report must not confirm a closed static QR order")
	}
	if err := db.Model(&model.PaymentRecord{}).Where("order_id = ?", order.ID).Count(&paymentCount).Error; err != nil {
		t.Fatal(err)
	}
	if paymentCount != 0 {
		t.Fatalf("callback or merchant self-report created payment rows before admin reconciliation: %d", paymentCount)
	}
	var stillInactive model.Merchant
	if err := db.First(&stillInactive, merchant.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stillInactive.SubscriptionStatus != "inactive" || stillInactive.SubscriptionPlanID != nil || stillInactive.SubscriptionExpireAt != nil {
		t.Fatalf("callback or self-report activated subscription before admin reconciliation: %+v", stillInactive)
	}

	confirmed, err := service.ConfirmMerchantSubscriptionPayment(order.ID, "已核对平台静态二维码账户实际到账")
	if err != nil {
		t.Fatalf("admin reconciliation of expired static QR payment: %v", err)
	}
	if confirmed.Status != "paid" || confirmed.PaidAt == nil || confirmed.SubscriptionEndAt == nil || !orderHasAction(confirmed.OperationLogs, "subscription_payment_reconciled_after_expiry") {
		t.Fatalf("admin reconciliation must be explicit and auditable: status=%q paid_at=%v end=%v logs=%s", confirmed.Status, confirmed.PaidAt, confirmed.SubscriptionEndAt, confirmed.OperationLogs)
	}
	var reconciledPayment model.PaymentRecord
	if err := db.Where("order_id = ?", order.ID).First(&reconciledPayment).Error; err != nil {
		t.Fatalf("load admin-confirmed platform QR payment: %v", err)
	}
	if reconciledPayment.Status != "success" || reconciledPayment.PaymentChannel != "platform_qr" {
		t.Fatalf("admin reconciliation payment record = %+v", reconciledPayment)
	}
	var activeMerchant model.Merchant
	if err := db.First(&activeMerchant, merchant.ID).Error; err != nil {
		t.Fatal(err)
	}
	if activeMerchant.SubscriptionStatus != "active" || activeMerchant.SubscriptionPlanID == nil || activeMerchant.SubscriptionExpireAt == nil {
		t.Fatalf("only explicit admin reconciliation should activate subscription: status=%q plan=%v expire=%v", activeMerchant.SubscriptionStatus, activeMerchant.SubscriptionPlanID, activeMerchant.SubscriptionExpireAt)
	}
}
