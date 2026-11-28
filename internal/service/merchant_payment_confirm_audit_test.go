package service

import (
	"testing"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

// TestMerchantPaymentConfirmWritesAudit verifies M1: confirming receipt of a
// customer payment records who did it (audit_logs row in the same
// transaction as the PaymentRecord), not just a text line in the order log.
func TestMerchantPaymentConfirmWritesAudit(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.AuditLog{}); err != nil {
		t.Fatalf("migrate synthetic audit schema: %v", err)
	}
	merchant, store, product, _ := createSyntheticStoreOrder(t, db, "SYNTHETIC-PAYCONF-AUDIT-001", "submitted", 1200, nil)
	store.OrderMode = "submit_later"
	if err := db.Save(&store).Error; err != nil {
		t.Fatal(err)
	}
	orderService := NewOrderService(db)
	order, err := orderService.CreateCustomerStoreOrder(dto.CreateCustomerOrderRequest{
		StoreID: store.ID,
		Items:   []dto.CustomerOrderItemRequest{{ProductID: product.ID, Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("create synthetic order to confirm: %v", err)
	}
	confirmed, err := orderService.ConfirmMerchantOrderPayment(merchant.ID, order.ID, AuditEvent{
		Actor: AuditActor{ID: 7, Type: "merchant_admin"},
		Input: AuditRecordInput{Action: "merchant_order_payment_confirm", TargetType: "order"},
	})
	if err != nil {
		t.Fatalf("confirm synthetic merchant payment: %v", err)
	}
	if confirmed.PaidAt == nil {
		t.Fatal("confirmed order must carry PaidAt")
	}
	var payment model.PaymentRecord
	if err := db.Where("order_id = ?", order.ID).First(&payment).Error; err != nil {
		t.Fatalf("confirmed payment must write a payment record: %v", err)
	}
	var logs []model.AuditLog
	if err := db.Where("action = ?", "merchant_order_payment_confirm").Find(&logs).Error; err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 {
		t.Fatalf("want exactly 1 payment-confirm audit row, got %d", len(logs))
	}
	if logs[0].ActorID != 7 || logs[0].ActorType != "merchant_admin" || logs[0].TargetID != order.ID || logs[0].MerchantID == nil || *logs[0].MerchantID != merchant.ID {
		t.Fatalf("payment-confirm audit row does not identify the operator: %+v", logs[0])
	}
}

// TestMerchantDisablesWriteAudit verifies L1: a merchant disabling their own
// store/product or deleting a promotion leaves an audit row with the actor.
func TestMerchantDisablesWriteAudit(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.AuditLog{}); err != nil {
		t.Fatalf("migrate synthetic audit schema: %v", err)
	}
	merchant, store, product, _ := createSyntheticStoreOrder(t, db, "SYNTHETIC-DISABLE-AUDIT-001", "submitted", 1200, nil)
	merchantService := NewMerchantService(nil, db)
	actor := AuditActor{ID: 9, Type: "merchant_admin"}

	if err := merchantService.DisableStoreProduct(merchant.ID, product.ID, AuditEvent{
		Actor: actor, Input: AuditRecordInput{Action: "merchant_product_disable", TargetType: "store_product"},
	}); err != nil {
		t.Fatalf("disable synthetic product: %v", err)
	}
	var savedProduct model.StoreProduct
	if err := db.First(&savedProduct, product.ID).Error; err != nil {
		t.Fatal(err)
	}
	if savedProduct.Status != "inactive" {
		t.Fatalf("disabled product status=%q, want inactive", savedProduct.Status)
	}

	promotion := model.Promotion{MerchantID: merchant.ID, Title: "合成促销", Type: "amount", Status: "draft"}
	if err := db.Create(&promotion).Error; err != nil {
		t.Fatalf("create synthetic promotion: %v", err)
	}
	if err := merchantService.DeletePromotion(merchant.ID, promotion.ID, AuditEvent{
		Actor: actor, Input: AuditRecordInput{Action: "merchant_promotion_delete", TargetType: "promotion"},
	}); err != nil {
		t.Fatalf("delete synthetic promotion: %v", err)
	}
	var promotionCount int64
	if err := db.Model(&model.Promotion{}).Where("id = ?", promotion.ID).Count(&promotionCount).Error; err != nil {
		t.Fatal(err)
	}
	if promotionCount != 0 {
		t.Fatalf("deleted promotion still present: count=%d", promotionCount)
	}

	if err := merchantService.DisableStore(merchant.ID, store.ID, AuditEvent{
		Actor: actor, Input: AuditRecordInput{Action: "merchant_store_disable", TargetType: "store"},
	}); err != nil {
		t.Fatalf("disable synthetic store: %v", err)
	}
	var savedStore model.Store
	if err := db.First(&savedStore, store.ID).Error; err != nil {
		t.Fatal(err)
	}
	if savedStore.Status != "inactive" {
		t.Fatalf("disabled store status=%q, want inactive", savedStore.Status)
	}

	var logs []model.AuditLog
	if err := db.Order("id ASC").Find(&logs).Error; err != nil {
		t.Fatal(err)
	}
	want := []string{"merchant_product_disable", "merchant_promotion_delete", "merchant_store_disable"}
	if len(logs) != len(want) {
		t.Fatalf("want %d audit rows, got %d", len(want), len(logs))
	}
	for i, action := range want {
		if logs[i].Action != action || logs[i].ActorID != 9 || logs[i].ActorType != "merchant_admin" || logs[i].MerchantID == nil || *logs[i].MerchantID != merchant.ID {
			t.Fatalf("audit row %d mismatch: %+v", i, logs[i])
		}
	}
}
