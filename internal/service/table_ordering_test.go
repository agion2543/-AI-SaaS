package service

import (
	"strings"
	"testing"
	"time"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
	"gorm.io/gorm"
)

type tableOrderingFixture struct {
	db        *gorm.DB
	service   *TableOrderingService
	merchant  model.Merchant
	store     model.Store
	product   model.StoreProduct
	tableNo   int
	tableCode string
	sessionID uint
}

func newTableOrderingFixture(t *testing.T) tableOrderingFixture {
	t.Helper()
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(
		&model.StoreTableConfig{},
		&model.StoreTable{},
		&model.TableSession{},
		&model.TableCartItem{},
		&model.TableOrderDetail{},
		&model.MerchantPaymentConfig{},
	); err != nil {
		t.Fatalf("migrate synthetic table-ordering schema: %v", err)
	}
	merchant, store, product := createSyntheticPricingProduct(t, db, 250, 1, nil)
	if err := db.Create(&model.MerchantPaymentConfig{
		MerchantID: merchant.ID, Channel: "alipay", Mode: "direct", Status: "enabled", AuditStatus: "approved",
		AccountName: "synthetic merchant", AlipayQRCode: "https://merchant.example.invalid/approved-qr.png",
	}).Error; err != nil {
		t.Fatalf("create approved synthetic merchant QR config: %v", err)
	}
	if err := db.Model(&store).Update("order_mode", "pay_first").Error; err != nil {
		t.Fatalf("set synthetic store to pay-first mode: %v", err)
	}
	store.OrderMode = "pay_first"
	service := NewTableOrderingService(db, NewOrderService(db))
	config, err := service.ConfigureStoreTables(merchant.ID, store.ID, 1, nil, func(_ int, code string) string { return code })
	if err != nil {
		t.Fatalf("configure one synthetic table: %v", err)
	}
	if config.TableCount != 1 || len(config.Tables) != 1 || config.Tables[0].TableNo != 1 {
		t.Fatalf("unexpected synthetic table configuration: %+v", config)
	}
	code := config.Tables[0].URL
	joined, err := service.JoinTableSession(dto.JoinTableSessionRequest{StoreID: store.ID, TableNo: 1, TableCode: code})
	if err != nil {
		t.Fatalf("join synthetic shared table session: %v", err)
	}
	return tableOrderingFixture{
		db: db, service: service, merchant: merchant, store: store, product: product,
		tableNo: 1, tableCode: code, sessionID: joined.ID,
	}
}

func tableCheckoutRequest(requestKey, accessToken string) dto.TableCheckoutRequest {
	return dto.TableCheckoutRequest{
		RequestKey: requestKey, AccessToken: accessToken, PaymentChannel: "alipay",
	}
}

func TestTableOrderingSharedCartAndOffShelfRemoval(t *testing.T) {
	fixture := newTableOrderingFixture(t)
	firstPhone, err := fixture.service.JoinTableSession(dto.JoinTableSessionRequest{
		StoreID: fixture.store.ID, TableNo: fixture.tableNo, TableCode: fixture.tableCode,
	})
	if err != nil {
		t.Fatalf("first customer joins: %v", err)
	}
	secondPhone, err := fixture.service.JoinTableSession(dto.JoinTableSessionRequest{
		StoreID: fixture.store.ID, TableNo: fixture.tableNo, TableCode: fixture.tableCode,
	})
	if err != nil {
		t.Fatalf("second customer joins: %v", err)
	}
	if firstPhone.ID != secondPhone.ID {
		t.Fatalf("same table QR created different sessions: %d and %d", firstPhone.ID, secondPhone.ID)
	}

	firstView, err := fixture.service.UpdateCartItem(firstPhone.ID, fixture.tableCode, dto.UpdateTableCartItemRequest{
		ProductID: fixture.product.ID, Action: "add", Quantity: 2,
	})
	if err != nil {
		t.Fatalf("first customer adds an item: %v", err)
	}
	secondView, err := fixture.service.SessionSnapshot(secondPhone.ID, fixture.tableCode)
	if err != nil {
		t.Fatalf("second customer refreshes shared session: %v", err)
	}
	if len(secondView.Items) != 1 || secondView.Items[0].ProductID != fixture.product.ID || secondView.Items[0].Quantity != 2 || secondView.CartVersion != firstView.CartVersion {
		t.Fatalf("second phone did not see the shared cart update: first=%+v second=%+v", firstView, secondView)
	}

	if err := fixture.db.Model(&model.StoreProduct{}).Where("id = ?", fixture.product.ID).Update("status", "inactive").Error; err != nil {
		t.Fatalf("take synthetic cart product off shelf: %v", err)
	}
	removed, err := fixture.service.SessionSnapshot(secondPhone.ID, fixture.tableCode)
	if err != nil {
		t.Fatalf("refresh after the product goes off shelf: %v", err)
	}
	if len(removed.Items) != 0 || removed.CartVersion <= secondView.CartVersion {
		t.Fatalf("off-shelf product was not removed from the shared cart: %+v", removed)
	}
	var remaining int64
	if err := fixture.db.Model(&model.TableCartItem{}).Where("session_id = ?", secondPhone.ID).Count(&remaining).Error; err != nil || remaining != 0 {
		t.Fatalf("off-shelf cart row count=%d err=%v, want 0", remaining, err)
	}
	firstRefresh, err := fixture.service.SessionSnapshot(firstPhone.ID, fixture.tableCode)
	if err != nil {
		t.Fatalf("first customer refreshes after removal: %v", err)
	}
	if len(firstRefresh.Items) != 0 || firstRefresh.CartVersion != removed.CartVersion {
		t.Fatalf("shared removal did not propagate to the first phone: %+v", firstRefresh)
	}
}

func TestTableOrderingCheckoutLocksCartAndIsIdempotent(t *testing.T) {
	fixture := newTableOrderingFixture(t)
	stock := 0
	if err := fixture.db.Model(&model.StoreProduct{}).Where("id = ?", fixture.product.ID).Update("stock", stock).Error; err != nil {
		t.Fatalf("set legacy inventory to zero: %v", err)
	}
	if _, err := fixture.service.UpdateCartItem(fixture.sessionID, fixture.tableCode, dto.UpdateTableCartItemRequest{
		ProductID: fixture.product.ID, Action: "add", Quantity: 2,
	}); err != nil {
		t.Fatalf("add item to shared table cart: %v", err)
	}

	requestKey := strings.Repeat("a", 64)
	accessToken := strings.Repeat("b", 64)
	order, created, err := fixture.service.Checkout(fixture.sessionID, fixture.tableCode, tableCheckoutRequest(requestKey, accessToken))
	if err != nil {
		t.Fatalf("first customer checks out: %v", err)
	}
	if !created || order.TableNo != fixture.tableNo || order.Amount != 500 || order.TotalAmount != 500 || order.Status != "pending" || order.PaymentChannel != "merchant_qr" {
		t.Fatalf("unexpected table checkout order: created=%v order=%+v", created, order)
	}

	if _, _, err := fixture.service.Checkout(fixture.sessionID, fixture.tableCode, tableCheckoutRequest(strings.Repeat("c", 64), strings.Repeat("d", 64))); err == nil {
		t.Fatal("second customer was allowed to create another checkout order")
	}
	if _, err := fixture.service.UpdateCartItem(fixture.sessionID, fixture.tableCode, dto.UpdateTableCartItemRequest{
		ProductID: fixture.product.ID, Action: "add", Quantity: 1,
	}); err == nil {
		t.Fatal("shared cart remained editable after checkout was locked")
	}
	if _, err := fixture.service.ClearCart(fixture.sessionID, fixture.tableCode); err == nil {
		t.Fatal("shared cart remained clearable after checkout was locked")
	}

	retry, created, err := fixture.service.Checkout(fixture.sessionID, fixture.tableCode, tableCheckoutRequest(requestKey, accessToken))
	if err != nil {
		t.Fatalf("same checkout request should be idempotent: %v", err)
	}
	if created || retry.ID != order.ID {
		t.Fatalf("same request created a duplicate order: original=%d retry=%d created=%v", order.ID, retry.ID, created)
	}
	var orderCount int64
	if err := fixture.db.Model(&model.Order{}).Count(&orderCount).Error; err != nil {
		t.Fatal(err)
	}
	if orderCount != 1 {
		t.Fatalf("table checkout created %d orders, want exactly one", orderCount)
	}
	var tableDetail model.TableOrderDetail
	if err := fixture.db.Where("order_id = ?", order.ID).First(&tableDetail).Error; err != nil {
		t.Fatalf("read order-to-table association: %v", err)
	}
	if tableDetail.TableNo != fixture.tableNo || tableDetail.SessionID != fixture.sessionID || tableDetail.TableLabelSnapshot == nil || *tableDetail.TableLabelSnapshot != "1号桌" {
		t.Fatalf("order-to-table association mismatch: %+v", tableDetail)
	}

	var stored model.Order
	if err := fixture.db.First(&stored, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := fixture.service.AttachTableNumber(&stored); err != nil || stored.TableNo != fixture.tableNo {
		t.Fatalf("single-order table number association=%d err=%v", stored.TableNo, err)
	}
	orders := []model.Order{{BaseModel: model.BaseModel{ID: order.ID}}}
	if err := fixture.service.AttachTableNumbers(orders); err != nil || orders[0].TableNo != fixture.tableNo {
		t.Fatalf("batch-order table number association=%d err=%v", orders[0].TableNo, err)
	}
	var retainedStock int
	if err := fixture.db.Model(&model.StoreProduct{}).Select("stock").Where("id = ?", fixture.product.ID).Scan(&retainedStock).Error; err != nil {
		t.Fatal(err)
	}
	if retainedStock != 0 {
		t.Fatalf("restaurant checkout should not mutate legacy stock, got %d", retainedStock)
	}
}

func TestTableOrderingCheckoutRequiresApprovedMerchantQRCode(t *testing.T) {
	fixture := newTableOrderingFixture(t)
	if err := fixture.db.Model(&model.MerchantPaymentConfig{}).
		Where("merchant_id = ?", fixture.merchant.ID).
		Update("audit_status", "pending").Error; err != nil {
		t.Fatalf("disable approval on synthetic merchant QR: %v", err)
	}
	if _, err := fixture.service.UpdateCartItem(fixture.sessionID, fixture.tableCode, dto.UpdateTableCartItemRequest{
		ProductID: fixture.product.ID, Action: "add", Quantity: 1,
	}); err != nil {
		t.Fatalf("add item before blocked checkout: %v", err)
	}

	order, created, err := fixture.service.Checkout(fixture.sessionID, fixture.tableCode, tableCheckoutRequest(strings.Repeat("1", 64), strings.Repeat("2", 64)))
	if err == nil || created || order != nil || !strings.Contains(err.Error(), "已审核的商家收款码") {
		t.Fatalf("checkout without an approved merchant QR must fail before order creation: order=%+v created=%v err=%v", order, created, err)
	}
	var orderCount int64
	if err := fixture.db.Model(&model.Order{}).Count(&orderCount).Error; err != nil || orderCount != 0 {
		t.Fatalf("unapproved payment config must not create an order: count=%d err=%v", orderCount, err)
	}
	snapshot, err := fixture.service.SessionSnapshot(fixture.sessionID, fixture.tableCode)
	if err != nil || snapshot.Locked || len(snapshot.Items) != 1 {
		t.Fatalf("failed checkout should leave the shared table cart open: snapshot=%+v err=%v", snapshot, err)
	}
}

func TestTableOrderingMerchantQRReportRemainsUnconfirmedAndCannotRetryPlatformPayment(t *testing.T) {
	fixture := newTableOrderingFixture(t)
	if _, err := fixture.service.UpdateCartItem(fixture.sessionID, fixture.tableCode, dto.UpdateTableCartItemRequest{
		ProductID: fixture.product.ID, Action: "add", Quantity: 1,
	}); err != nil {
		t.Fatalf("add item before merchant QR checkout: %v", err)
	}
	requestKey, accessToken := strings.Repeat("3", 64), strings.Repeat("4", 64)
	order, created, err := fixture.service.Checkout(fixture.sessionID, fixture.tableCode, tableCheckoutRequest(requestKey, accessToken))
	if err != nil || !created || order.PaymentChannel != "merchant_qr" || order.Status != "pending" {
		t.Fatalf("table checkout must remain pending on the merchant QR path: order=%+v created=%v err=%v", order, created, err)
	}
	config, err := fixture.service.PublicPaymentConfigForOrder(order)
	if err != nil || config == nil || config.AlipayQRCode != "https://merchant.example.invalid/approved-qr.png" {
		t.Fatalf("customer-visible config should contain only the approved merchant QR: config=%+v err=%v", config, err)
	}
	if _, err := fixture.service.orders.PrepareCustomerOrderRetry(order.OrderNo, accessToken); err == nil || !strings.Contains(err.Error(), "不可使用平台支付入口") {
		t.Fatalf("table order must not be retryable through platform payment: err=%v", err)
	}

	marked, err := fixture.service.orders.MarkCustomerStoreOrderPaid(order.OrderNo, accessToken)
	if err != nil {
		t.Fatalf("customer may notify the merchant after QR transfer: %v", err)
	}
	if marked.Status != "payment_confirming" || marked.PaidAt != nil {
		t.Fatalf("customer declaration must remain unconfirmed: status=%q paid_at=%v", marked.Status, marked.PaidAt)
	}
	var records int64
	if err := fixture.db.Model(&model.PaymentRecord{}).Where("order_id = ?", order.ID).Count(&records).Error; err != nil || records != 0 {
		t.Fatalf("customer declaration must not create a successful payment record: count=%d err=%v", records, err)
	}
}

func TestTableOrderingExpiredUnpaidCheckoutReopensSession(t *testing.T) {
	fixture := newTableOrderingFixture(t)
	if _, err := fixture.service.UpdateCartItem(fixture.sessionID, fixture.tableCode, dto.UpdateTableCartItemRequest{
		ProductID: fixture.product.ID, Action: "add", Quantity: 1,
	}); err != nil {
		t.Fatalf("add item before checkout: %v", err)
	}
	order, created, err := fixture.service.Checkout(fixture.sessionID, fixture.tableCode, tableCheckoutRequest(strings.Repeat("e", 64), strings.Repeat("f", 64)))
	if err != nil || !created {
		t.Fatalf("create unpaid checkout order: order=%+v created=%v err=%v", order, created, err)
	}
	if err := fixture.db.Model(&model.Order{}).Where("id = ?", order.ID).UpdateColumn("created_at", time.Now().Add(-31*time.Minute)).Error; err != nil {
		t.Fatalf("age unpaid checkout beyond its deadline: %v", err)
	}

	snapshot, err := fixture.service.SessionSnapshot(fixture.sessionID, fixture.tableCode)
	if err != nil {
		t.Fatalf("refresh expired shared session: %v", err)
	}
	if snapshot.Locked || snapshot.Status != tableSessionOpen || len(snapshot.Items) != 0 || snapshot.CheckoutOrderNo != "" {
		t.Fatalf("expired checkout did not reopen and clear the shared session: %+v", snapshot)
	}
	var closedOrder model.Order
	if err := fixture.db.First(&closedOrder, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	if closedOrder.Status != "closed" || closedOrder.PaidAt != nil {
		t.Fatalf("expired order should close without marking paid: status=%q paid_at=%v", closedOrder.Status, closedOrder.PaidAt)
	}
	var session model.TableSession
	if err := fixture.db.First(&session, fixture.sessionID).Error; err != nil {
		t.Fatal(err)
	}
	if session.Status != tableSessionOpen || session.CheckoutOrderID != nil {
		t.Fatalf("session lock was not cleared after expiry: %+v", session)
	}
}

func TestTableOrderingLabelConfigurationAndCheckoutSnapshot(t *testing.T) {
	fixture := newTableOrderingFixture(t)
	makeURL := func(_ int, code string) string { return "table/" + code }
	configured, err := fixture.service.ConfigureStoreTables(fixture.merchant.ID, fixture.store.ID, 1, []dto.ConfigureStoreTableLabelItem{{TableNo: 1, TableLabel: "A01"}}, makeURL)
	if err != nil {
		t.Fatalf("set custom table label: %v", err)
	}
	if configured.Tables[0].TableLabel != "A01" || configured.Tables[0].URL != "table/"+fixture.tableCode {
		t.Fatalf("custom label changed QR identity or was not returned: %+v", configured.Tables[0])
	}
	snapshot, err := fixture.service.SessionSnapshot(fixture.sessionID, fixture.tableCode)
	if err != nil || snapshot.TableLabel != "A01" {
		t.Fatalf("session did not return current custom label: snapshot=%+v err=%v", snapshot, err)
	}
	if _, err := fixture.service.UpdateCartItem(fixture.sessionID, fixture.tableCode, dto.UpdateTableCartItemRequest{ProductID: fixture.product.ID, Action: "add", Quantity: 1}); err != nil {
		t.Fatalf("add item before custom-label checkout: %v", err)
	}
	requestKey, accessToken := strings.Repeat("7", 64), strings.Repeat("8", 64)
	order, created, err := fixture.service.Checkout(fixture.sessionID, fixture.tableCode, tableCheckoutRequest(requestKey, accessToken))
	if err != nil || !created || order.TableLabel != "A01" {
		t.Fatalf("checkout did not capture label snapshot: order=%+v created=%v err=%v", order, created, err)
	}
	if _, err := fixture.service.ConfigureStoreTables(fixture.merchant.ID, fixture.store.ID, 1, []dto.ConfigureStoreTableLabelItem{{TableNo: 1, TableLabel: "包间"}}, makeURL); err != nil {
		t.Fatalf("rename current table after checkout: %v", err)
	}
	retry, created, err := fixture.service.Checkout(fixture.sessionID, fixture.tableCode, tableCheckoutRequest(requestKey, accessToken))
	if err != nil || created || retry.TableLabel != "A01" {
		t.Fatalf("idempotent retry changed the historical label: order=%+v created=%v err=%v", retry, created, err)
	}
	var saved model.Order
	if err := fixture.db.First(&saved, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := fixture.service.AttachTableNumber(&saved); err != nil || saved.TableLabel != "A01" {
		t.Fatalf("order read returned current label instead of checkout snapshot: order=%+v err=%v", saved, err)
	}
	var detail model.TableOrderDetail
	if err := fixture.db.Where("order_id = ?", order.ID).First(&detail).Error; err != nil {
		t.Fatal(err)
	}
	if err := fixture.db.Model(&detail).Update("table_label_snapshot", nil).Error; err != nil {
		t.Fatalf("simulate historical order without label snapshot: %v", err)
	}
	saved.TableLabel = ""
	if err := fixture.service.AttachTableNumber(&saved); err != nil || saved.TableLabel != "1号桌" {
		t.Fatalf("legacy order should fall back to its table number: order=%+v err=%v", saved, err)
	}
}

func TestTableOrderingRejectsDuplicateCustomLabels(t *testing.T) {
	fixture := newTableOrderingFixture(t)
	_, err := fixture.service.ConfigureStoreTables(fixture.merchant.ID, fixture.store.ID, 2, []dto.ConfigureStoreTableLabelItem{{TableNo: 1, TableLabel: "A01"}, {TableNo: 2, TableLabel: "a01"}}, func(_ int, code string) string { return code })
	if err == nil || !strings.Contains(err.Error(), "重复") {
		t.Fatalf("duplicate case-insensitive table labels should be rejected, got %v", err)
	}
}
