package service

import (
	"testing"
	"time"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

func TestPaymentCallbackDoesNotAcceptOrCompleteStoreOrder(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	_, _, _, order := createSyntheticStoreOrder(t, db, "SYNTHETIC-LIFECYCLE-PAY-001", "pending", 1800, nil)

	paid, err := NewOrderService(db).HandlePaymentCallback(dto.PaymentCallbackRequest{
		OrderNo:        order.OrderNo,
		TransactionNo:  "synthetic-lifecycle-trade-001",
		PaymentChannel: "alipay",
		Amount:         order.TotalAmount,
		Status:         "success",
	})
	if err != nil {
		t.Fatalf("synthetic payment callback: %v", err)
	}
	if paid.Status != "received" || paid.PaidAt == nil {
		t.Fatalf("payment callback should confirm payment and leave fulfillment awaiting merchant action: status=%q paid_at=%v", paid.Status, paid.PaidAt)
	}
	if paid.Status == "accepted" || paid.Status == "completed" || orderHasAction(paid.OperationLogs, "accepted") || orderHasAction(paid.OperationLogs, "completed") {
		t.Fatalf("payment callback must not accept or complete fulfillment: status=%q logs=%s", paid.Status, paid.OperationLogs)
	}
}

func TestDeferredSettlementOrderCannotCompleteBeforePaymentConfirmation(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	merchant, store, product, _ := createSyntheticStoreOrder(t, db, "SYNTHETIC-LIFECYCLE-DEFER-001", "submitted", 2600, nil)
	store.OrderMode = "submit_later"
	store.AutoAccept = false
	if err := db.Save(&store).Error; err != nil {
		t.Fatal(err)
	}
	service := NewOrderService(db)
	order, err := service.CreateCustomerStoreOrder(dto.CreateCustomerOrderRequest{
		StoreID: store.ID,
		Items:   []dto.CustomerOrderItemRequest{{ProductID: product.ID, Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("create synthetic deferred-settlement order: %v", err)
	}
	if order.Status != "submitted" || order.PaidAt != nil || !orderHasAction(order.OperationLogs, "submitted") {
		t.Fatalf("deferred-settlement creation should submit without payment: status=%q paid_at=%v logs=%s", order.Status, order.PaidAt, order.OperationLogs)
	}

	if err := service.AcceptMerchantOrder(merchant.ID, order.ID); err != nil {
		t.Fatalf("deferred-settlement order should be accepted without upfront payment: %v", err)
	}
	if err := service.CompleteMerchantOrder(merchant.ID, order.ID); err == nil {
		t.Fatal("unpaid deferred-settlement order must not be marked complete")
	}
	var awaitingPayment model.Order
	if err := db.First(&awaitingPayment, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	if awaitingPayment.Status != "accepted" || awaitingPayment.PaidAt != nil {
		t.Fatalf("rejected completion must preserve unpaid accepted state: status=%q paid_at=%v", awaitingPayment.Status, awaitingPayment.PaidAt)
	}
	var paymentCount int64
	if err := db.Model(&model.PaymentRecord{}).Where("order_id = ?", order.ID).Count(&paymentCount).Error; err != nil {
		t.Fatal(err)
	}
	if paymentCount != 0 {
		t.Fatalf("unpaid deferred order must not create a payment record, got %d", paymentCount)
	}

	// An unpaid order without the deferred-settlement submission marker remains blocked.
	unmarked := model.Order{
		OrderNo:     "SYNTHETIC-LIFECYCLE-UNPAID-ACCEPTED-001",
		OrderType:   "store_order",
		MerchantID:  &merchant.ID,
		StoreID:     &store.ID,
		Amount:      900,
		TotalAmount: 900,
		Status:      "accepted",
	}
	if err := db.Create(&unmarked).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.CompleteMerchantOrder(merchant.ID, unmarked.ID); err == nil {
		t.Fatal("ordinary unpaid order without deferred-settlement provenance must not be completed")
	}
	var unchanged model.Order
	if err := db.First(&unchanged, unmarked.ID).Error; err != nil {
		t.Fatal(err)
	}
	if unchanged.Status != "accepted" || unchanged.PaidAt != nil {
		t.Fatalf("rejected completion must leave ordinary unpaid order unchanged: status=%q paid_at=%v", unchanged.Status, unchanged.PaidAt)
	}

	admin := NewAdminService(nil, db)
	prepared, err := admin.PrepareMerchantSettlement(merchant.ID)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.OrderCount != 0 || prepared.TotalAmountCents != 0 {
		t.Fatalf("completed-but-unpaid orders must not enter settlement preparation: %+v", prepared)
	}
	if _, _, err := admin.CreateMerchantSettlement(merchant.ID, dto.CreateMerchantSettlementRequest{Remark: "synthetic unpaid exclusion"}); err == nil {
		t.Fatal("settlement creation must reject a merchant with only unpaid orders")
	}
	var settlementCount int64
	if err := db.Model(&model.MerchantSettlement{}).Where("merchant_id = ?", merchant.ID).Count(&settlementCount).Error; err != nil {
		t.Fatal(err)
	}
	if settlementCount != 0 {
		t.Fatalf("unpaid orders must not create settlement records, got %d", settlementCount)
	}
}

func TestDeferredSettlementCanCompleteAfterMerchantConfirmsPayment(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	merchant, store, product, _ := createSyntheticStoreOrder(t, db, "SYNTHETIC-LIFECYCLE-CONFIRMED-001", "submitted", 2600, nil)
	store.OrderMode = "submit_later"
	if err := db.Save(&store).Error; err != nil {
		t.Fatal(err)
	}
	service := NewOrderService(db)
	order, err := service.CreateCustomerStoreOrder(dto.CreateCustomerOrderRequest{
		StoreID: store.ID,
		Items:   []dto.CustomerOrderItemRequest{{ProductID: product.ID, Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("create synthetic deferred-settlement order: %v", err)
	}
	if err := service.AcceptMerchantOrder(merchant.ID, order.ID); err != nil {
		t.Fatalf("accept deferred-settlement order: %v", err)
	}
	if err := service.CompleteMerchantOrder(merchant.ID, order.ID); err == nil {
		t.Fatal("accepted but unpaid order must remain incomplete")
	}

	confirmed, err := service.ConfirmMerchantOrderPayment(merchant.ID, order.ID)
	if err != nil {
		t.Fatalf("merchant-confirmed receipt on accepted order: %v", err)
	}
	if confirmed.Status != "accepted" || confirmed.PaidAt == nil || confirmed.PaymentChannel != "merchant_qr" {
		t.Fatalf("merchant confirmation must record receipt without regressing fulfillment: status=%q paid_at=%v channel=%q", confirmed.Status, confirmed.PaidAt, confirmed.PaymentChannel)
	}
	if err := service.CompleteMerchantOrder(merchant.ID, order.ID); err != nil {
		t.Fatalf("confirmed order should be completable: %v", err)
	}
	var completed model.Order
	if err := db.First(&completed, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	if completed.Status != "completed" || completed.PaidAt == nil || !orderHasAction(completed.OperationLogs, "merchant_payment_confirmed") {
		t.Fatalf("completed order must retain confirmed-payment evidence: status=%q paid_at=%v logs=%s", completed.Status, completed.PaidAt, completed.OperationLogs)
	}
}

func TestCustomerPaymentReportDoesNotSetConfirmedReceipt(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	_, store, product, _ := createSyntheticStoreOrder(t, db, "SYNTHETIC-LIFECYCLE-REPORT-001", "submitted", 1200, nil)
	store.OrderMode = "submit_later"
	if err := db.Save(&store).Error; err != nil {
		t.Fatal(err)
	}
	service := NewOrderService(db)
	order, err := service.CreateCustomerStoreOrder(dto.CreateCustomerOrderRequest{
		StoreID: store.ID,
		Items:   []dto.CustomerOrderItemRequest{{ProductID: product.ID, Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("create synthetic deferred-settlement order: %v", err)
	}
	marked, err := service.MarkCustomerStoreOrderPaid(order.OrderNo, order.CustomerAccessToken)
	if err != nil {
		t.Fatalf("mark customer-reported payment: %v", err)
	}
	if marked.Status != "payment_confirming" || marked.PaidAt != nil || orderPaymentSatisfied(*marked) {
		t.Fatalf("customer report must remain distinct from confirmed receipt: status=%q paid_at=%v", marked.Status, marked.PaidAt)
	}
}

func TestSettlementCreationIncludesOnlyConfirmedPaidOrdersForRequestedTenant(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	merchant := model.Merchant{Name: "合成结算商户-生命周期", Status: "active"}
	otherMerchant := model.Merchant{Name: "合成其他商户-生命周期", Status: "active"}
	if err := db.Create(&merchant).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&otherMerchant).Error; err != nil {
		t.Fatal(err)
	}
	paidAt := time.Now().UTC()
	orders := []model.Order{
		{OrderNo: "SYNTHETIC-SETTLEMENT-PAID-001", OrderType: "store_order", MerchantID: &merchant.ID, Status: "completed", TotalAmount: 1700, PaidAt: &paidAt},
		{OrderNo: "SYNTHETIC-SETTLEMENT-UNPAID-COMPLETED-001", OrderType: "store_order", MerchantID: &merchant.ID, Status: "completed", TotalAmount: 2800},
		{OrderNo: "SYNTHETIC-SETTLEMENT-UNPAID-ACCEPTED-001", OrderType: "store_order", MerchantID: &merchant.ID, Status: "accepted", TotalAmount: 3600},
		{OrderNo: "SYNTHETIC-SETTLEMENT-OTHER-TENANT-001", OrderType: "store_order", MerchantID: &otherMerchant.ID, Status: "completed", TotalAmount: 4900, PaidAt: &paidAt},
	}
	if err := db.Create(&orders).Error; err != nil {
		t.Fatalf("create synthetic settlement orders: %v", err)
	}

	admin := NewAdminService(nil, db)
	prepared, err := admin.PrepareMerchantSettlement(merchant.ID)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.OrderCount != 1 || prepared.TotalAmountCents != 1700 || len(prepared.Orders) != 1 || prepared.Orders[0].OrderNo != "SYNTHETIC-SETTLEMENT-PAID-001" {
		t.Fatalf("settlement preparation must include only the requested merchant's confirmed-paid eligible order: %+v", prepared)
	}

	settlement, settledOrders, err := admin.CreateMerchantSettlement(merchant.ID, dto.CreateMerchantSettlementRequest{Remark: "synthetic tenant/payment boundary"})
	if err != nil {
		t.Fatalf("create synthetic settlement: %v", err)
	}
	if settlement.OrderCount != 1 || settlement.TotalAmountCents != 1700 || settlement.NetAmountCents != 1700 || len(settledOrders) != 1 || settledOrders[0].OrderNo != "SYNTHETIC-SETTLEMENT-PAID-001" {
		t.Fatalf("created settlement contains unexpected orders or amounts: settlement=%+v orders=%+v", settlement, settledOrders)
	}
	var current []model.Order
	if err := db.Where("id IN ?", []uint{orders[0].ID, orders[1].ID, orders[2].ID, orders[3].ID}).Find(&current).Error; err != nil {
		t.Fatal(err)
	}
	for _, order := range current {
		if order.OrderNo == "SYNTHETIC-SETTLEMENT-PAID-001" {
			if order.SettlementID == nil || *order.SettlementID != settlement.ID {
				t.Errorf("confirmed-paid order was not attached to its settlement: %+v", order)
			}
			continue
		}
		if order.SettlementID != nil {
			t.Errorf("unpaid or cross-tenant order entered another merchant's settlement: %+v", order)
		}
	}
}
