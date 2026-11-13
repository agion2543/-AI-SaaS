package service

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/smartwalle/alipay/v3"
	"gorm.io/gorm"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

// SQLite is used only as a synthetic test store here. A single connection
// keeps its transaction scheduling deterministic; it does not simulate
// MySQL's row locks or prove production isolation behavior.
func useSerializedSyntheticDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get SQL test database: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
}

type concurrentCallbackResult struct {
	callback dto.PaymentCallbackRequest
	order    *model.Order
	err      error
}

func runConcurrentCallbacks(service *OrderService, callbacks []dto.PaymentCallbackRequest) []concurrentCallbackResult {
	start := make(chan struct{})
	results := make(chan concurrentCallbackResult, len(callbacks))
	for _, callback := range callbacks {
		callback := callback
		go func() {
			<-start
			order, err := service.HandlePaymentCallback(callback)
			results <- concurrentCallbackResult{callback: callback, order: order, err: err}
		}()
	}
	close(start)
	out := make([]concurrentCallbackResult, 0, len(callbacks))
	for range callbacks {
		out = append(out, <-results)
	}
	return out
}

func loadOrderPaymentRecords(t *testing.T, db *gorm.DB, orderID uint) ([]model.PaymentRecord, int64) {
	t.Helper()
	var records []model.PaymentRecord
	if err := db.Where("order_id = ?", orderID).Order("id asc").Find(&records).Error; err != nil {
		t.Fatalf("load synthetic payment records: %v", err)
	}
	var successfulAmount int64
	for _, record := range records {
		if record.Status == "success" {
			successfulAmount += record.Amount
		}
	}
	return records, successfulAmount
}

func countOrderAuditAction(t *testing.T, order model.Order, action string) int {
	t.Helper()
	var logs []OrderOperationLog
	if err := json.Unmarshal([]byte(order.OperationLogs), &logs); err != nil {
		t.Fatalf("decode synthetic order audit: %v", err)
	}
	count := 0
	for _, log := range logs {
		if log.Action == action {
			count++
		}
	}
	return count
}

func TestConcurrentSameTransactionPaymentCallbacksAreIdempotent(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	useSerializedSyntheticDB(t, db)
	_, _, product, order := createSyntheticStoreOrder(t, db, "SYNTHETIC-CONCURRENT-SAME-TX-001", "pending", 500, nil)
	service := NewOrderService(db)
	callbacks := make([]dto.PaymentCallbackRequest, 12)
	for i := range callbacks {
		callbacks[i] = dto.PaymentCallbackRequest{
			OrderNo:        order.OrderNo,
			TransactionNo:  "synthetic-concurrent-same-transaction",
			PaymentChannel: "alipay",
			Amount:         order.TotalAmount,
			Status:         "success",
		}
	}

	results := runConcurrentCallbacks(service, callbacks)
	for _, result := range results {
		if result.err != nil {
			t.Errorf("same-transaction callback %q failed: %v", result.callback.TransactionNo, result.err)
		}
	}
	var current model.Order
	if err := db.First(&current, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	records, successfulAmount := loadOrderPaymentRecords(t, db, order.ID)
	var updatedProduct model.StoreProduct
	if err := db.First(&updatedProduct, product.ID).Error; err != nil {
		t.Fatal(err)
	}
	if current.Status != "received" || current.PaidAt == nil {
		t.Errorf("same transaction must settle order once: status=%q paid_at=%v", current.Status, current.PaidAt)
	}
	if len(records) != 1 || successfulAmount != order.TotalAmount {
		t.Errorf("same transaction must produce one successful ledger row: rows=%d successful_amount=%d want=%d", len(records), successfulAmount, order.TotalAmount)
	}
	if updatedProduct.Stock == nil || *updatedProduct.Stock != 8 {
		t.Errorf("same transaction must deduct stock once: stock=%v", updatedProduct.Stock)
	}
	if countOrderAuditAction(t, current, "paid") != 1 || countOrderAuditAction(t, current, "stock_deducted") != 1 {
		t.Errorf("same transaction must leave one payment and stock audit entry: %s", current.OperationLogs)
	}
}

func TestConcurrentDistinctSuccessfulPaymentCallbacksRecordExceptionWithoutDuplicateFulfillment(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	useSerializedSyntheticDB(t, db)
	_, _, product, order := createSyntheticStoreOrder(t, db, "SYNTHETIC-CONCURRENT-DISTINCT-TX-001", "pending", 500, nil)
	service := NewOrderService(db)
	callbacks := []dto.PaymentCallbackRequest{
		{OrderNo: order.OrderNo, TransactionNo: "synthetic-distinct-trade-a", PaymentChannel: "alipay", Amount: order.TotalAmount, Status: "success"},
		{OrderNo: order.OrderNo, TransactionNo: "synthetic-distinct-trade-b", PaymentChannel: "alipay", Amount: order.TotalAmount, Status: "success"},
	}

	results := runConcurrentCallbacks(service, callbacks)
	for _, result := range results {
		if result.err != nil {
			t.Errorf("distinct successful callback %q failed: %v", result.callback.TransactionNo, result.err)
		}
	}
	var current model.Order
	if err := db.First(&current, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	records, successfulAmount := loadOrderPaymentRecords(t, db, order.ID)
	adminPaymentRows, err := service.AllPayments()
	if err != nil {
		t.Fatalf("load administrator finance payment rows: %v", err)
	}
	var adminSawNormalPayment, adminSawDuplicateException bool
	for _, row := range adminPaymentRows {
		if row.OrderID != order.ID {
			continue
		}
		if row.Order.OrderNo != order.OrderNo || row.Amount != order.TotalAmount || row.TransactionNo == "" {
			t.Errorf("administrator finance row must expose order number, transaction number, and amount: %+v", row)
		}
		if row.Status == "success" {
			adminSawNormalPayment = true
		}
		if row.Status == "duplicate_success" {
			adminSawDuplicateException = true
		}
	}
	if !adminSawNormalPayment || !adminSawDuplicateException {
		t.Errorf("administrator finance list must expose both the ordinary payment and duplicate-success exception for order %s", order.OrderNo)
	}
	var updatedProduct model.StoreProduct
	if err := db.First(&updatedProduct, product.ID).Error; err != nil {
		t.Fatal(err)
	}
	var normalSuccesses, duplicateExceptions int
	for _, record := range records {
		switch record.Status {
		case "success":
			normalSuccesses++
		case "duplicate_success":
			duplicateExceptions++
		}
	}
	if current.Status != "received" || current.PaidAt == nil {
		t.Errorf("order should settle only once: status=%q paid_at=%v", current.Status, current.PaidAt)
	}
	if len(records) != 2 || normalSuccesses != 1 || duplicateExceptions != 1 || successfulAmount != order.TotalAmount {
		t.Errorf("both external charges must be preserved, with one normal payment and one exception: rows=%d normal=%d duplicate=%d successful_amount=%d transactions=%v", len(records), normalSuccesses, duplicateExceptions, successfulAmount, paymentTransactionNumbers(records))
	}
	if updatedProduct.Stock == nil || *updatedProduct.Stock != 8 {
		t.Errorf("fulfillment stock must be deducted once: stock=%v", updatedProduct.Stock)
	}
	if countOrderAuditAction(t, current, "paid") != 1 || countOrderAuditAction(t, current, "stock_deducted") != 1 || countOrderAuditAction(t, current, "duplicate_payment_exception") != 1 {
		t.Errorf("distinct callbacks must leave one fulfillment and one visible duplicate-payment audit entry: %s", current.OperationLogs)
	}
}

func paymentTransactionNumbers(records []model.PaymentRecord) []string {
	transactions := make([]string, 0, len(records))
	for _, record := range records {
		transactions = append(transactions, record.TransactionNo)
	}
	return transactions
}

func syntheticRefundResponse(request *http.Request) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"alipay_cert_sn":"synthetic-cert","sign":"c2lnbmF0dXJl","alipay_trade_refund_response":{"code":"10000","msg":"Success","trade_no":"synthetic-provider-trade","out_trade_no":"synthetic-order","refund_fee":"6.00"}}`)),
		Request:    request,
	}
}

func newSyntheticAlipayRefundService(t *testing.T, transport http.RoundTripper) *AlipayService {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generate synthetic signing key: %v", err)
	}
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	client, err := alipay.New("synthetic-app", string(privateKeyPEM), true,
		alipay.WithProductionGateway("https://alipay.invalid/gateway.do"),
		alipay.WithVerifier("synthetic-cert", acceptingSyntheticAlipayVerifier{}),
		alipay.WithHTTPClient(&http.Client{Transport: transport}),
	)
	if err != nil {
		t.Fatalf("create local synthetic payment client: %v", err)
	}
	return &AlipayService{client: client}
}

func newHeldSyntheticAlipayRefundService(t *testing.T, calls *atomic.Int32) (*AlipayService, <-chan struct{}, func()) {
	t.Helper()
	entered := make(chan struct{})
	release := make(chan struct{})
	var enteredOnce sync.Once
	var releaseOnce sync.Once
	transport := orderTestRoundTripper(func(request *http.Request) (*http.Response, error) {
		calls.Add(1)
		if request.URL.Host != "alipay.invalid" || request.Method != http.MethodPost {
			return nil, fmt.Errorf("unexpected synthetic gateway request: %s %s", request.Method, request.URL)
		}
		enteredOnce.Do(func() { close(entered) })
		select {
		case <-release:
			return syntheticRefundResponse(request), nil
		case <-time.After(5 * time.Second):
			return nil, errors.New("synthetic refund response was not released")
		}
	})
	service := newSyntheticAlipayRefundService(t, transport)
	return service, entered, func() { releaseOnce.Do(func() { close(release) }) }
}

func TestConcurrentRefundsReserveAmountBeforeExternalSuccess(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	useSerializedSyntheticDB(t, db)
	paidAt := time.Now().UTC()
	merchant, _, _, order := createSyntheticStoreOrder(t, db, "SYNTHETIC-CONCURRENT-REFUND-001", "accepted", 1000, &paidAt)
	order.TransactionNo = "synthetic-already-paid-transaction"
	if err := db.Save(&order).Error; err != nil {
		t.Fatal(err)
	}
	var providerCalls atomic.Int32
	provider, providerEntered, releaseProvider := newHeldSyntheticAlipayRefundService(t, &providerCalls)
	defer releaseProvider()
	service := NewOrderService(db, provider)
	start := make(chan struct{})
	results := make(chan error, 2)
	for operatorID := uint(10); operatorID <= 11; operatorID++ {
		operatorID := operatorID
		go func() {
			<-start
			_, err := service.RefundMerchantOrder(merchant.ID, order.ID, dto.RefundOrderRequest{Amount: 600, Reason: fmt.Sprintf("合成并发部分退款-%d", operatorID)}, operatorID, "merchant")
			results <- err
		}()
	}
	close(start)
	select {
	case <-providerEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("no synthetic refund reached the in-process provider")
	}

	// The first provider request is held open. The other caller must observe the
	// durable reservation and fail locally instead of dispatching a second call.
	var competingErr error
	select {
	case competingErr = <-results:
	case <-time.After(5 * time.Second):
		t.Fatal("competing refund did not finish while the first provider request was held")
	}
	if competingErr == nil {
		t.Fatal("competing partial refund should be rejected while 600 is reserved")
	}
	if providerCalls.Load() != 1 {
		t.Fatalf("provider must see only the one already-reserved refund while that request is in flight, calls=%d", providerCalls.Load())
	}
	releaseProvider()
	var firstErr error
	select {
	case firstErr = <-results:
	case <-time.After(5 * time.Second):
		t.Fatal("reserved refund did not finish after synthetic provider release")
	}
	if firstErr != nil {
		t.Fatalf("reserved synthetic refund should succeed: %v", firstErr)
	}

	var current model.Order
	if err := db.First(&current, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	var refunds []model.RefundRecord
	if err := db.Where("order_id = ?", order.ID).Order("id asc").Find(&refunds).Error; err != nil {
		t.Fatalf("load synthetic refund records: %v", err)
	}
	var recordedRefundAmount int64
	for _, refund := range refunds {
		if refund.Status == "success" {
			recordedRefundAmount += refund.Amount
		}
	}
	if current.RefundedAmount > current.TotalAmount || recordedRefundAmount > current.TotalAmount {
		t.Errorf("persisted refund total exceeded payment: order_refunded=%d records_total=%d paid=%d", current.RefundedAmount, recordedRefundAmount, current.TotalAmount)
	}
	if providerCalls.Load() != 1 {
		t.Errorf("only one 600/1000 refund fits; synthetic provider calls=%d", providerCalls.Load())
	}
	if len(refunds) != 1 || refunds[0].Status != "success" || recordedRefundAmount != 600 || current.RefundedAmount != 600 || current.RefundStatus != "partial" {
		t.Errorf("local ledger should retain one successful partial refund: rows=%d record_sum=%d refunded=%d status=%q records=%+v", len(refunds), recordedRefundAmount, current.RefundedAmount, current.RefundStatus, refunds)
	}
	if countOrderAuditAction(t, current, "refund_processing") != 1 || countOrderAuditAction(t, current, "refund") != 1 {
		t.Errorf("reservation and success audit entries should each appear once: %s", current.OperationLogs)
	}
}

func TestRefundIdempotencyKeyReplaysSuccessWithoutSecondProviderDispatch(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	paidAt := time.Now().UTC()
	merchant, _, _, order := createSyntheticStoreOrder(t, db, "SYNTHETIC-IDEMPOTENT-REFUND-001", "accepted", 1000, &paidAt)
	order.TransactionNo = "synthetic-idempotent-paid-transaction"
	if err := db.Save(&order).Error; err != nil {
		t.Fatal(err)
	}
	var providerCalls atomic.Int32
	provider := newSyntheticAlipayRefundService(t, orderTestRoundTripper(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != "alipay.invalid" || request.Method != http.MethodPost {
			return nil, fmt.Errorf("unexpected synthetic gateway request: %s %s", request.Method, request.URL)
		}
		providerCalls.Add(1)
		return syntheticRefundResponse(request), nil
	}))
	service := NewOrderService(db, provider)
	request := dto.RefundOrderRequest{Amount: 300, Reason: "合成幂等退款", IdempotencyKey: "synthetic-refund-key-001"}
	first, err := service.RefundMerchantOrder(merchant.ID, order.ID, request, 42, "merchant")
	if err != nil {
		t.Fatalf("first synthetic refund: %v", err)
	}
	second, err := service.RefundMerchantOrder(merchant.ID, order.ID, request, 42, "merchant")
	if err != nil {
		t.Fatalf("idempotent replay: %v", err)
	}
	var refunds []model.RefundRecord
	if err := db.Where("order_id = ?", order.ID).Find(&refunds).Error; err != nil {
		t.Fatal(err)
	}
	if providerCalls.Load() != 1 || len(refunds) != 1 || refunds[0].Status != "success" || refunds[0].IdempotencyKey == nil || *refunds[0].IdempotencyKey != request.IdempotencyKey {
		t.Fatalf("same key must return existing success without a second dispatch: calls=%d rows=%+v", providerCalls.Load(), refunds)
	}
	if first.RefundedAmount != 300 || second.RefundedAmount != 300 {
		t.Fatalf("idempotent replay must not increment refunded amount twice: first=%d second=%d", first.RefundedAmount, second.RefundedAmount)
	}
	changed := request
	changed.Amount = 400
	if _, err := service.RefundMerchantOrder(merchant.ID, order.ID, changed, 42, "merchant"); err == nil || !strings.Contains(err.Error(), "different request") {
		t.Fatalf("reusing a key with a different payload must be rejected, got %v", err)
	}
	if providerCalls.Load() != 1 {
		t.Fatalf("mismatched idempotency key must not dispatch again, calls=%d", providerCalls.Load())
	}
}

func TestUnknownRefundOutcomeRetainsReservationAndBlocksRetry(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	paidAt := time.Now().UTC()
	merchant, _, _, order := createSyntheticStoreOrder(t, db, "SYNTHETIC-UNKNOWN-REFUND-001", "accepted", 1000, &paidAt)
	order.TransactionNo = "synthetic-unknown-paid-transaction"
	if err := db.Save(&order).Error; err != nil {
		t.Fatal(err)
	}
	var providerCalls atomic.Int32
	provider := newSyntheticAlipayRefundService(t, orderTestRoundTripper(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != "alipay.invalid" || request.Method != http.MethodPost {
			return nil, fmt.Errorf("unexpected synthetic gateway request: %s %s", request.Method, request.URL)
		}
		providerCalls.Add(1)
		return nil, errors.New("synthetic timeout after request dispatch")
	}))
	service := NewOrderService(db, provider)
	request := dto.RefundOrderRequest{Amount: 600, Reason: "合成超时退款", IdempotencyKey: "synthetic-unknown-key-001"}
	if _, err := service.RefundMerchantOrder(merchant.ID, order.ID, request, 42, "merchant"); err == nil || !strings.Contains(err.Error(), "outcome is unknown") {
		t.Fatalf("timeout must leave an explicit unknown result, got %v", err)
	}
	var refunds []model.RefundRecord
	if err := db.Where("order_id = ?", order.ID).Find(&refunds).Error; err != nil {
		t.Fatal(err)
	}
	var current model.Order
	if err := db.First(&current, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	if len(refunds) != 1 || refunds[0].Status != "unknown" || refunds[0].RefundNo == "" || refunds[0].Amount != 600 || current.RefundedAmount != 0 || current.RefundStatus != "processing" {
		t.Fatalf("ambiguous provider result must keep a durable 600 reservation without claiming success: rows=%+v order_refunded=%d refund_status=%q", refunds, current.RefundedAmount, current.RefundStatus)
	}
	if refunds[0].IdempotencyKey == nil || *refunds[0].IdempotencyKey != request.IdempotencyKey {
		t.Fatalf("unknown reservation must retain idempotency key: %+v", refunds[0])
	}
	if _, err := service.RefundMerchantOrder(merchant.ID, order.ID, request, 42, "merchant"); err == nil || !strings.Contains(err.Error(), "must be reconciled") {
		t.Fatalf("same-key retry must be blocked for reconciliation, got %v", err)
	}
	if _, err := service.RefundMerchantOrder(merchant.ID, order.ID, dto.RefundOrderRequest{Amount: 100, Reason: "另一笔合成退款", IdempotencyKey: "synthetic-unknown-key-002"}, 42, "merchant"); err == nil || !strings.Contains(err.Error(), "must be reconciled before any further refund") {
		t.Fatalf("another request must wait for reconciliation even within remaining balance, got %v", err)
	}
	if providerCalls.Load() != 1 {
		t.Fatalf("unknown refund must never be blindly dispatched again; calls=%d", providerCalls.Load())
	}
	if countOrderAuditAction(t, current, "refund_unknown") != 1 {
		t.Fatalf("unknown result must be visible in order audit: %s", current.OperationLogs)
	}
}

func TestDefinitiveProviderRejectionReleasesReservationForNewKey(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	paidAt := time.Now().UTC()
	merchant, _, _, order := createSyntheticStoreOrder(t, db, "SYNTHETIC-FAILED-REFUND-001", "accepted", 1000, &paidAt)
	order.TransactionNo = "synthetic-definitive-failure-paid-transaction"
	if err := db.Save(&order).Error; err != nil {
		t.Fatal(err)
	}
	var providerCalls atomic.Int32
	provider := newSyntheticAlipayRefundService(t, orderTestRoundTripper(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != "alipay.invalid" || request.Method != http.MethodPost {
			return nil, fmt.Errorf("unexpected synthetic gateway request: %s %s", request.Method, request.URL)
		}
		if providerCalls.Add(1) == 1 {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"alipay_cert_sn":"synthetic-cert","sign":"c2lnbmF0dXJl","alipay_trade_refund_response":{"code":"40004","msg":"Business Failed","sub_code":"ACQ.TRADE_NOT_EXIST","sub_msg":"synthetic definitive rejection"}}`)),
				Request:    request,
			}, nil
		}
		return syntheticRefundResponse(request), nil
	}))
	service := NewOrderService(db, provider)
	firstRequest := dto.RefundOrderRequest{Amount: 600, Reason: "合成确定失败退款", IdempotencyKey: "synthetic-failed-key-001"}
	if _, err := service.RefundMerchantOrder(merchant.ID, order.ID, firstRequest, 42, "merchant"); err == nil || !strings.Contains(err.Error(), "definitively rejected") {
		t.Fatalf("provider's explicit rejection must be reported as definitive, got %v", err)
	}
	var afterFailure model.Order
	if err := db.First(&afterFailure, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	if afterFailure.RefundedAmount != 0 || afterFailure.RefundStatus != "none" {
		t.Fatalf("definitive rejection must not count as refunded or keep the reservation: refunded=%d status=%q", afterFailure.RefundedAmount, afterFailure.RefundStatus)
	}
	var failed model.RefundRecord
	if err := db.Where("order_id = ? AND idempotency_key = ?", order.ID, firstRequest.IdempotencyKey).First(&failed).Error; err != nil {
		t.Fatal(err)
	}
	if failed.Status != "failed" {
		t.Fatalf("explicit provider rejection should be persisted as failed, got %q", failed.Status)
	}
	secondRequest := dto.RefundOrderRequest{Amount: 600, Reason: "合成重新发起退款", IdempotencyKey: "synthetic-failed-key-002"}
	if _, err := service.RefundMerchantOrder(merchant.ID, order.ID, secondRequest, 42, "merchant"); err != nil {
		t.Fatalf("new key should be allowed after a definitive provider rejection: %v", err)
	}
	if providerCalls.Load() != 2 {
		t.Fatalf("expected the rejected request and one explicit new-key retry, calls=%d", providerCalls.Load())
	}
}

func TestRefundFinalizationReconcilesExistingSuccessfulRecordTotal(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	paidAt := time.Now().UTC()
	merchant, _, _, order := createSyntheticStoreOrder(t, db, "SYNTHETIC-LEGACY-REFUND-TOTAL-001", "accepted", 1000, &paidAt)
	order.TransactionNo = "synthetic-legacy-refund-paid-transaction"
	if err := db.Save(&order).Error; err != nil {
		t.Fatal(err)
	}
	legacyRefund := model.RefundRecord{
		OrderID:       order.ID,
		RefundNo:      "RFLEGACY-SYNTHETIC-001",
		TransactionNo: order.TransactionNo,
		Amount:        300,
		Reason:        "历史合成退款",
		OperatorRole:  "admin",
		Status:        "success",
	}
	if err := db.Create(&legacyRefund).Error; err != nil {
		t.Fatalf("create legacy successful refund row: %v", err)
	}
	var providerCalls atomic.Int32
	provider := newSyntheticAlipayRefundService(t, orderTestRoundTripper(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != "alipay.invalid" || request.Method != http.MethodPost {
			return nil, fmt.Errorf("unexpected synthetic gateway request: %s %s", request.Method, request.URL)
		}
		providerCalls.Add(1)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"alipay_cert_sn":"synthetic-cert","sign":"c2lnbmF0dXJl","alipay_trade_refund_response":{"code":"10000","msg":"Success","trade_no":"synthetic-provider-trade","out_trade_no":"synthetic-order","refund_fee":"2.00"}}`)),
			Request:    request,
		}, nil
	}))
	service := NewOrderService(db, provider)
	updated, err := service.RefundMerchantOrder(merchant.ID, order.ID, dto.RefundOrderRequest{Amount: 200, Reason: "新的合成退款"}, 42, "merchant")
	if err != nil {
		t.Fatalf("refund after legacy success row: %v", err)
	}
	var persisted model.Order
	if err := db.First(&persisted, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	var successfulTotal int64
	if err := db.Model(&model.RefundRecord{}).Where("order_id = ? AND status = ?", order.ID, "success").Select("COALESCE(SUM(amount), 0)").Scan(&successfulTotal).Error; err != nil {
		t.Fatal(err)
	}
	if providerCalls.Load() != 1 || updated.RefundedAmount != 500 || persisted.RefundedAmount != 500 || successfulTotal != 500 || persisted.RefundStatus != "partial" {
		t.Fatalf("order cumulative refund must reflect all successful legacy and new rows: calls=%d returned=%d stored=%d rows_total=%d status=%q", providerCalls.Load(), updated.RefundedAmount, persisted.RefundedAmount, successfulTotal, persisted.RefundStatus)
	}
}
