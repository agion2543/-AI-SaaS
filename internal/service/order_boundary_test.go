package service

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/smartwalle/alipay/v3"
	"github.com/smartwalle/nsign"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

type orderTestRoundTripper func(*http.Request) (*http.Response, error)

func (roundTrip orderTestRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func newOrderBoundaryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open SQLite test database: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Merchant{},
		&model.MerchantPlan{},
		&model.Store{},
		&model.StoreProduct{},
		&model.User{},
		&model.MembershipPackage{},
		&model.Promotion{},
			&model.Order{},
			&model.PaymentRecord{},
			&model.RefundRecord{},
			&model.MerchantSettlement{},
			&model.MerchantSubscriptionManualReview{},
			&model.MerchantSubscriptionManualReviewAudit{},
		); err != nil {
		t.Fatalf("migrate order test schema: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get SQL test database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func createSyntheticStoreOrder(t *testing.T, db *gorm.DB, orderNo, status string, amount int64, paidAt *time.Time) (model.Merchant, model.Store, model.StoreProduct, model.Order) {
	t.Helper()
	merchant := model.Merchant{Name: "合成商户-" + orderNo, Status: "active"}
	if err := db.Create(&merchant).Error; err != nil {
		t.Fatalf("create synthetic merchant: %v", err)
	}
	store := model.Store{MerchantID: merchant.ID, Name: "合成门店-" + orderNo, Status: "active", IsOpen: true}
	if err := db.Create(&store).Error; err != nil {
		t.Fatalf("create synthetic store: %v", err)
	}
	stock := 10
	product := model.StoreProduct{StoreID: store.ID, Name: "合成商品", Price: amount, Status: "active", Stock: &stock}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create synthetic product: %v", err)
	}
	items, err := json.Marshal([]StoreOrderItemSnapshot{{ProductID: product.ID, Name: product.Name, Price: amount, Quantity: 2, LineAmount: amount * 2}})
	if err != nil {
		t.Fatal(err)
	}
	order := model.Order{
		OrderNo:        orderNo,
		OrderType:      "store_order",
		MerchantID:     &merchant.ID,
		StoreID:        &store.ID,
		Amount:         amount,
		TotalAmount:    amount,
		Items:          string(items),
		Status:         status,
		PaidAt:         paidAt,
		PaymentChannel: "alipay",
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create synthetic order: %v", err)
	}
	return merchant, store, product, order
}

func TestPaymentCallbackDuplicateIsIdempotentAndStaleFailureDoesNotDowngrade(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	_, _, product, order := createSyntheticStoreOrder(t, db, "SYNTHETIC-DUPLICATE-001", "pending", 500, nil)
	service := NewOrderService(db)
	callback := dto.PaymentCallbackRequest{
		OrderNo:        order.OrderNo,
		TransactionNo:  "synthetic-trade-001",
		PaymentChannel: "alipay",
		Amount:         order.TotalAmount,
		Status:         "success",
	}

	first, err := service.HandlePaymentCallback(callback)
	if err != nil {
		t.Fatalf("first success callback: %v", err)
	}
	duplicate, err := service.HandlePaymentCallback(callback)
	if err != nil {
		t.Fatalf("duplicate success callback: %v", err)
	}
	if first.Status != "received" || duplicate.Status != "received" || duplicate.PaidAt == nil {
		t.Fatalf("duplicate callback changed the paid state: first=%+v duplicate=%+v", first, duplicate)
	}

	staleFailure := callback
	staleFailure.TransactionNo = "synthetic-stale-failure-001"
	staleFailure.Status = "failed"
	if _, err := service.HandlePaymentCallback(staleFailure); err != nil {
		t.Fatalf("stale failure callback: %v", err)
	}

	var current model.Order
	if err := db.First(&current, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	var paymentCount int64
	if err := db.Model(&model.PaymentRecord{}).Where("order_id = ?", order.ID).Count(&paymentCount).Error; err != nil {
		t.Fatal(err)
	}
	var updatedProduct model.StoreProduct
	if err := db.First(&updatedProduct, product.ID).Error; err != nil {
		t.Fatal(err)
	}
	if current.Status != "received" || current.PaidAt == nil || paymentCount != 2 {
		t.Fatalf("stale failure must not undo confirmed payment; status=%q paid_at=%v payment_count=%d", current.Status, current.PaidAt, paymentCount)
	}
	if updatedProduct.Stock == nil || *updatedProduct.Stock != 8 {
		t.Fatalf("duplicate success callback must deduct stock once, stock=%v", updatedProduct.Stock)
	}
	if !orderHasAction(current.OperationLogs, "paid") || !orderHasAction(current.OperationLogs, "stock_deducted") {
		t.Fatalf("successful payment audit actions missing: %s", current.OperationLogs)
	}
}

func TestPaymentCallbackFailureRetryAndSuccessTransitionsAreAudited(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	_, _, product, order := createSyntheticStoreOrder(t, db, "SYNTHETIC-RETRY-001", "pending", 700, nil)
	accessToken := "synthetic-customer-token"
	order.CustomerAccessTokenHash = hashCustomerOrderAccessToken(accessToken)
	if err := db.Save(&order).Error; err != nil {
		t.Fatal(err)
	}
	service := NewOrderService(db)
	failed, err := service.HandlePaymentCallback(dto.PaymentCallbackRequest{
		OrderNo:        order.OrderNo,
		TransactionNo:  "synthetic-failed-trade",
		PaymentChannel: "alipay",
		Amount:         order.TotalAmount,
		Status:         "failed",
	})
	if err != nil {
		t.Fatalf("failed payment callback: %v", err)
	}
	if failed.Status != "failed" || !orderHasAction(failed.OperationLogs, "payment_failed") {
		t.Fatalf("failed payment state/log missing: %+v", failed)
	}

	retried, err := service.PrepareCustomerOrderRetry(order.OrderNo, accessToken)
	if err != nil {
		t.Fatalf("prepare retry after failure: %v", err)
	}
	if retried.Status != "pending" || !orderHasAction(retried.OperationLogs, "payment_retry") {
		t.Fatalf("retry did not restore pending state and audit it: %+v", retried)
	}
	paid, err := service.HandlePaymentCallback(dto.PaymentCallbackRequest{
		OrderNo:        order.OrderNo,
		TransactionNo:  "synthetic-retry-success-trade",
		PaymentChannel: "alipay",
		Amount:         order.TotalAmount,
		Status:         "success",
	})
	if err != nil {
		t.Fatalf("success callback after explicit retry: %v", err)
	}
	var updatedProduct model.StoreProduct
	if err := db.First(&updatedProduct, product.ID).Error; err != nil {
		t.Fatal(err)
	}
	var paymentCount int64
	if err := db.Model(&model.PaymentRecord{}).Where("order_id = ?", order.ID).Count(&paymentCount).Error; err != nil {
		t.Fatal(err)
	}
	if paid.Status != "received" || paid.PaidAt == nil || paymentCount != 2 || updatedProduct.Stock == nil || *updatedProduct.Stock != 8 {
		t.Fatalf("retry success did not settle exactly once: status=%q paid_at=%v payments=%d stock=%v", paid.Status, paid.PaidAt, paymentCount, updatedProduct.Stock)
	}
	for _, action := range []string{"payment_failed", "payment_retry", "paid", "stock_deducted"} {
		if !orderHasAction(paid.OperationLogs, action) {
			t.Errorf("expected %q in operation audit: %s", action, paid.OperationLogs)
		}
	}
}

func TestPaymentCallbackRejectsAmountMismatchAndInvalidTransitionAtomically(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	_, _, _, pending := createSyntheticStoreOrder(t, db, "SYNTHETIC-AMOUNT-001", "pending", 900, nil)
	service := NewOrderService(db)
	_, err := service.HandlePaymentCallback(dto.PaymentCallbackRequest{
		OrderNo:        pending.OrderNo,
		TransactionNo:  "synthetic-wrong-amount",
		PaymentChannel: "alipay",
		Amount:         pending.TotalAmount - 1,
		Status:         "success",
	})
	if err == nil || !strings.Contains(err.Error(), "payment amount mismatch") {
		t.Fatalf("amount mismatch should be rejected, got %v", err)
	}
	var currentPending model.Order
	if err := db.First(&currentPending, pending.ID).Error; err != nil {
		t.Fatal(err)
	}
	var paymentCount int64
	if err := db.Model(&model.PaymentRecord{}).Where("order_id = ?", pending.ID).Count(&paymentCount).Error; err != nil {
		t.Fatal(err)
	}
	if currentPending.Status != "pending" || currentPending.PaidAt != nil || paymentCount != 0 {
		t.Fatalf("mismatched amount must not mutate order or payment records: status=%q paid_at=%v records=%d", currentPending.Status, currentPending.PaidAt, paymentCount)
	}

	_, _, _, closed := createSyntheticStoreOrder(t, db, "SYNTHETIC-CLOSED-001", "closed", 900, nil)
	_, err = service.HandlePaymentCallback(dto.PaymentCallbackRequest{
		OrderNo:        closed.OrderNo,
		TransactionNo:  "synthetic-closed-order-trade",
		PaymentChannel: "alipay",
		Amount:         closed.TotalAmount,
		Status:         "success",
	})
	if err != nil {
		t.Fatalf("late external success should be acknowledged after recording an exception, got %v", err)
	}
	if err := db.Model(&model.PaymentRecord{}).Where("order_id = ?", closed.ID).Count(&paymentCount).Error; err != nil {
		t.Fatal(err)
	}
	var latePayment model.PaymentRecord
	if err := db.Where("order_id = ?", closed.ID).First(&latePayment).Error; err != nil {
		t.Fatal(err)
	}
	var currentClosed model.Order
	if err := db.First(&currentClosed, closed.ID).Error; err != nil {
		t.Fatal(err)
	}
	if paymentCount != 1 || latePayment.Status != "late_success_exception" || currentClosed.Status != "closed" || currentClosed.PaidAt != nil || !orderHasAction(currentClosed.OperationLogs, "late_payment_exception") {
		t.Fatalf("closed order must retain a late-success exception without fulfillment: status=%q paid_at=%v records=%d payment_status=%q logs=%s", currentClosed.Status, currentClosed.PaidAt, paymentCount, latePayment.Status, currentClosed.OperationLogs)
	}
	adminPaymentRows, err := service.AllPayments()
	if err != nil {
		t.Fatalf("load administrator finance rows for late-success exception: %v", err)
	}
	adminCanFindLateException := false
	for _, row := range adminPaymentRows {
		if row.OrderID == closed.ID && row.TransactionNo == "synthetic-closed-order-trade" && row.Order.OrderNo == closed.OrderNo && row.Amount == closed.TotalAmount && row.Status == "late_success_exception" {
			adminCanFindLateException = true
		}
	}
	if !adminCanFindLateException {
		t.Fatalf("administrator finance rows must expose late-success order number, transaction number, amount, and exception status for manual reconciliation")
	}
}

func TestMerchantOrderServiceEnforcesTenantBoundaryForReadsAndWrites(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	owner, _, _, order := createSyntheticStoreOrder(t, db, "SYNTHETIC-TENANT-001", "submitted", 1200, nil)
	other := model.Merchant{Name: "合成无关商户", Status: "active"}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	service := NewOrderService(db)

	if _, err := service.MerchantOrderDetail(other.ID, order.ID); err == nil {
		t.Fatal("other merchant must not read order detail")
	}
	orders, total, err := service.MerchantOrders(other.ID, "", nil, "", 1, 20)
	if err != nil || total != 0 || len(orders) != 0 {
		t.Fatalf("other merchant order list must be empty: total=%d orders=%d err=%v", total, len(orders), err)
	}
	writeChecks := []struct {
		name string
		call func() error
	}{
		{name: "accept", call: func() error { return service.AcceptMerchantOrder(other.ID, order.ID) }},
		{name: "confirm payment", call: func() error { _, err := service.ConfirmMerchantOrderPayment(other.ID, order.ID); return err }},
		{name: "complete", call: func() error { return service.CompleteMerchantOrder(other.ID, order.ID) }},
		{name: "close", call: func() error { return service.CloseMerchantOrder(other.ID, order.ID) }},
		{name: "update note", call: func() error {
			_, err := service.UpdateMerchantOrderNote(other.ID, order.ID, "越权修改")
			return err
		}},
		{name: "refund", call: func() error {
			_, err := service.RefundMerchantOrder(other.ID, order.ID, dto.RefundOrderRequest{Full: true}, 17, "merchant")
			return err
		}},
	}
	for _, check := range writeChecks {
		t.Run(check.name, func(t *testing.T) {
			if err := check.call(); err == nil {
				t.Errorf("other merchant must not %s the order", check.name)
			}
		})
	}

	var unchanged model.Order
	if err := db.First(&unchanged, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	if unchanged.MerchantID == nil || *unchanged.MerchantID != owner.ID || unchanged.Status != "submitted" || unchanged.MerchantNote != "" {
		t.Fatalf("cross-tenant attempts changed order data: %+v", unchanged)
	}
	var payments, refunds int64
	if err := db.Model(&model.PaymentRecord{}).Where("order_id = ?", order.ID).Count(&payments).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.RefundRecord{}).Where("order_id = ?", order.ID).Count(&refunds).Error; err != nil {
		t.Fatal(err)
	}
	if payments != 0 || refunds != 0 {
		t.Fatalf("cross-tenant attempts created financial records: payments=%d refunds=%d", payments, refunds)
	}

	if err := db.Create(&model.RefundRecord{OrderID: order.ID, MerchantID: &owner.ID, StoreID: order.StoreID, RefundNo: "SYNTHETIC-OWNER-REFUND", Amount: 10, Reason: "仅用于租户过滤测试", OperatorRole: "admin", OperatorID: 7, Status: "success"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.MerchantSettlement{MerchantID: owner.ID, NetAmountCents: 100, Status: "pending"}).Error; err != nil {
		t.Fatal(err)
	}
	merchantRefunds, err := service.MerchantRefunds(other.ID)
	if err != nil || len(merchantRefunds) != 0 {
		t.Fatalf("other merchant refund list must not contain owner's records: count=%d err=%v", len(merchantRefunds), err)
	}
	settlementSummary, settlements, err := service.MerchantSettlements(other.ID)
	if err != nil || settlementSummary.SettlementCount != 0 || len(settlements) != 0 {
		t.Fatalf("other merchant settlement list must not contain owner's records: summary=%+v count=%d err=%v", settlementSummary, len(settlements), err)
	}
}

func TestMerchantSettlementSummaryExcludesUnpaidClosedAndAlreadySettledOrders(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	merchant := model.Merchant{Name: "合成结算商户", Status: "active"}
	otherMerchant := model.Merchant{Name: "合成其他结算商户", Status: "active"}
	if err := db.Create(&merchant).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&otherMerchant).Error; err != nil {
		t.Fatal(err)
	}
	paidAt := time.Now().UTC()
	pendingSettlement := model.MerchantSettlement{MerchantID: merchant.ID, NetAmountCents: 1200, Status: "pending"}
	paidSettlement := model.MerchantSettlement{MerchantID: merchant.ID, NetAmountCents: 600, Status: "paid"}
	if err := db.Create(&pendingSettlement).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&paidSettlement).Error; err != nil {
		t.Fatal(err)
	}
	settledID := paidSettlement.ID
	orders := []model.Order{
		{OrderNo: "SYNTHETIC-SETTLE-ELIGIBLE-1", OrderType: "store_order", MerchantID: &merchant.ID, Status: "accepted", TotalAmount: 1000, RefundedAmount: 100, PaidAt: &paidAt},
		{OrderNo: "SYNTHETIC-SETTLE-ELIGIBLE-2", OrderType: "store_order", MerchantID: &merchant.ID, Status: "completed", TotalAmount: 500, PaidAt: &paidAt},
		{OrderNo: "SYNTHETIC-SETTLE-UNPAID", OrderType: "store_order", MerchantID: &merchant.ID, Status: "accepted", TotalAmount: 3000},
		{OrderNo: "SYNTHETIC-SETTLE-CLOSED", OrderType: "store_order", MerchantID: &merchant.ID, Status: "closed", TotalAmount: 4000, PaidAt: &paidAt},
		{OrderNo: "SYNTHETIC-SETTLE-WAITING", OrderType: "store_order", MerchantID: &merchant.ID, Status: "payment_confirming", TotalAmount: 5000, PaidAt: &paidAt},
		{OrderNo: "SYNTHETIC-SETTLE-ALREADY", OrderType: "store_order", MerchantID: &merchant.ID, Status: "accepted", TotalAmount: 7000, PaidAt: &paidAt, SettlementID: &settledID},
		{OrderNo: "SYNTHETIC-SETTLE-SUBSCRIPTION", OrderType: "merchant_subscription", MerchantID: &merchant.ID, Status: "paid", TotalAmount: 8000, PaidAt: &paidAt},
		{OrderNo: "SYNTHETIC-SETTLE-OTHER-TENANT", OrderType: "store_order", MerchantID: &otherMerchant.ID, Status: "accepted", TotalAmount: 9000, PaidAt: &paidAt},
	}
	if err := db.Create(&orders).Error; err != nil {
		t.Fatalf("create settlement boundary orders: %v", err)
	}

	summary, settlements, err := NewOrderService(db).MerchantSettlements(merchant.ID)
	if err != nil {
		t.Fatal(err)
	}
	if summary.PendingOrderCount != 2 || summary.PendingOrderAmount != 1500 || summary.PendingRefundAmount != 100 || summary.PendingNetAmount != 1400 {
		t.Fatalf("unexpected pending settlement calculation: %+v", summary)
	}
	if summary.SettlementCount != 2 || summary.PendingSettlementCount != 1 || summary.PaidSettlementCount != 1 || summary.PaidSettlementAmount != 600 || len(settlements) != 2 {
		t.Fatalf("unexpected existing settlement summary: %+v rows=%d", summary, len(settlements))
	}
}

type acceptingSyntheticAlipayVerifier struct{}

func (acceptingSyntheticAlipayVerifier) VerifyValues(_ url.Values, _ []byte, _ ...nsign.SignOption) error {
	return nil
}

func (acceptingSyntheticAlipayVerifier) VerifyBytes(_ []byte, _ []byte, _ ...nsign.SignOption) error {
	return nil
}

func newLocalAlipayRefundService(t *testing.T, calls *int) *AlipayService {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generate synthetic signing key: %v", err)
	}
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	client, err := alipay.New("synthetic-app", string(privateKeyPEM), true,
		alipay.WithProductionGateway("https://alipay.invalid/gateway.do"),
		alipay.WithVerifier("synthetic-cert", acceptingSyntheticAlipayVerifier{}),
		alipay.WithHTTPClient(&http.Client{Transport: orderTestRoundTripper(func(request *http.Request) (*http.Response, error) {
			*calls++
			if request.URL.Host != "alipay.invalid" || request.Method != http.MethodPost {
				return nil, fmt.Errorf("unexpected synthetic gateway request: %s %s", request.Method, request.URL)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"alipay_cert_sn":"synthetic-cert","sign":"c2lnbmF0dXJl","alipay_trade_refund_response":{"code":"10000","msg":"Success","trade_no":"synthetic-provider-trade","out_trade_no":"synthetic-order","refund_fee":"3.00"}}`)),
				Request:    request,
			}, nil
		})}),
	)
	if err != nil {
		t.Fatalf("create local synthetic payment client: %v", err)
	}
	return &AlipayService{client: client}
}

func TestRefundPersistsActorReasonAndPartialToFullAuditState(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	paidAt := time.Now().UTC()
	merchant, _, _, order := createSyntheticStoreOrder(t, db, "SYNTHETIC-REFUND-AUDIT-001", "accepted", 1200, &paidAt)
	order.TransactionNo = "synthetic-paid-transaction"
	order.RefundedAmount = 0
	if err := db.Save(&order).Error; err != nil {
		t.Fatal(err)
	}
	providerCalls := 0
	service := NewOrderService(db, newLocalAlipayRefundService(t, &providerCalls))

	partial, err := service.RefundMerchantOrder(merchant.ID, order.ID, dto.RefundOrderRequest{Amount: 300, Reason: "商品少发"}, 42, "merchant")
	if err != nil {
		t.Fatalf("synthetic partial refund: %v", err)
	}
	if partial.RefundedAmount != 300 || partial.RefundStatus != "partial" || partial.Status != "accepted" {
		t.Fatalf("partial refund state mismatch: %+v", partial)
	}

	full, err := service.RefundMerchantOrder(merchant.ID, order.ID, dto.RefundOrderRequest{Full: true, Reason: "顾客要求全额退款"}, 7, "admin")
	if err != nil {
		t.Fatalf("synthetic remaining full refund: %v", err)
	}
	if full.RefundedAmount != order.TotalAmount || full.RefundStatus != "full" || full.Status != "accepted" {
		t.Fatalf("full refund state mismatch: %+v", full)
	}

	var refunds []model.RefundRecord
	if err := db.Where("order_id = ?", order.ID).Order("id asc").Find(&refunds).Error; err != nil {
		t.Fatal(err)
	}
	if len(refunds) != 2 {
		t.Fatalf("expected one row per successful refund, got %d", len(refunds))
	}
	if refunds[0].Amount != 300 || refunds[0].OperatorID != 42 || refunds[0].OperatorRole != "merchant" || refunds[0].Reason != "商品少发" || refunds[0].Status != "success" {
		t.Errorf("partial refund audit actor/reason/amount/status mismatch: %+v", refunds[0])
	}
	if refunds[1].Amount != 900 || refunds[1].OperatorID != 7 || refunds[1].OperatorRole != "admin" || refunds[1].Reason != "顾客要求全额退款" || refunds[1].Status != "success" {
		t.Errorf("full refund audit actor/reason/amount/status mismatch: %+v", refunds[1])
	}
	var logs []OrderOperationLog
	if err := json.Unmarshal([]byte(full.OperationLogs), &logs); err != nil {
		t.Fatalf("decode order audit log: %v", err)
	}
	if len(logs) != 4 || logs[0].Action != "refund_processing" || logs[1].Action != "refund" || !strings.Contains(logs[1].Text, "merchant") || !strings.Contains(logs[1].Text, "商品少发") || logs[2].Action != "refund_processing" || logs[3].Action != "refund" || !strings.Contains(logs[3].Text, "admin") || !strings.Contains(logs[3].Text, "顾客要求全额退款") {
		t.Fatalf("refund operation log must retain reservation, actor role, reason, and each completed state change: %+v", logs)
	}
	if providerCalls != 2 {
		t.Fatalf("expected two local synthetic gateway calls, got %d", providerCalls)
	}

	_, err = service.RefundMerchantOrder(merchant.ID, order.ID, dto.RefundOrderRequest{Amount: 1, Reason: "超额校验"}, 42, "merchant")
	if err == nil || !strings.Contains(err.Error(), "no refundable amount") {
		t.Fatalf("fully refunded order must reject further refund, got %v", err)
	}
	if providerCalls != 2 {
		t.Fatalf("invalid over-refund must not call gateway, calls=%d", providerCalls)
	}
}

func TestRefundRejectsUnpaidAndExcessAmountWithoutChangingAudit(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	merchant, _, _, unpaid := createSyntheticStoreOrder(t, db, "SYNTHETIC-REFUND-UNPAID-001", "accepted", 1000, nil)
	paidAt := time.Now().UTC()
	paidMerchant, _, _, paid := createSyntheticStoreOrder(t, db, "SYNTHETIC-REFUND-EXCESS-001", "accepted", 1000, &paidAt)
	service := NewOrderService(db)

	checks := []struct {
		name       string
		order      model.Order
		merchantID uint
		id         uint
		req        dto.RefundOrderRequest
		want       string
	}{
		{name: "unpaid", order: unpaid, merchantID: merchant.ID, id: unpaid.ID, req: dto.RefundOrderRequest{Full: true, Reason: "不可退未付款"}, want: "only confirmed paid orders"},
		{name: "excess amount", order: paid, merchantID: paidMerchant.ID, id: paid.ID, req: dto.RefundOrderRequest{Amount: 1001, Reason: "超出支付金额"}, want: "exceeds remaining refundable amount"},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			_, err := service.RefundMerchantOrder(check.merchantID, check.id, check.req, 42, "merchant")
			if err == nil || !strings.Contains(err.Error(), check.want) {
				t.Fatalf("refund validation error = %v, want substring %q", err, check.want)
			}
			var after model.Order
			if err := db.First(&after, check.id).Error; err != nil {
				t.Fatal(err)
			}
			var refundCount int64
			if err := db.Model(&model.RefundRecord{}).Where("order_id = ?", check.id).Count(&refundCount).Error; err != nil {
				t.Fatal(err)
			}
			if after.RefundedAmount != check.order.RefundedAmount || after.RefundStatus != check.order.RefundStatus || after.OperationLogs != check.order.OperationLogs || refundCount != 0 {
				t.Fatalf("rejected refund changed state or audit: order=%+v refund_count=%d", after, refundCount)
			}
		})
	}
}
