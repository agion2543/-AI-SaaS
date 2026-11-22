package service

import (
	"strings"
	"testing"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

func TestRecoverCustomerStoreOrderRequiresRequestKeyAndAccessToken(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	_, store, product := createSyntheticPricingProduct(t, db, 250, 1, nil)
	service := NewOrderService(db)
	request := dto.CreateCustomerOrderRequest{
		StoreID:     store.ID,
		RequestKey:  strings.Repeat("a1", 32),
		AccessToken: strings.Repeat("b2", 32),
		Items:       []dto.CustomerOrderItemRequest{{ProductID: product.ID, Quantity: 2}},
	}
	created, err := service.CreateCustomerStoreOrder(request)
	if err != nil {
		t.Fatalf("create synthetic order: %v", err)
	}

	recovered, err := service.RecoverCustomerStoreOrder(request.RequestKey, request.AccessToken)
	if err != nil {
		t.Fatalf("recover committed synthetic order: %v", err)
	}
	if recovered.ID != created.ID || recovered.OrderNo != created.OrderNo || recovered.CustomerAccessToken != request.AccessToken {
		t.Fatalf("recovery returned a different order or did not restore access: created=%+v recovered=%+v", created, recovered)
	}
	if _, err := service.RecoverCustomerStoreOrder(request.RequestKey, strings.Repeat("c3", 32)); err == nil {
		t.Fatal("recovery must reject a mismatched access token")
	}
	if _, err := service.RecoverCustomerStoreOrder(strings.Repeat("d4", 32), request.AccessToken); err == nil {
		t.Fatal("recovery must not report an order for an unused request key")
	}

	var count int64
	if err := db.Model(&model.Order{}).Where("order_type = ?", "store_order").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("read-only recovery changed order count: %d", count)
	}
}

func TestRecoverCustomerStoreOrderAppendFindsCommittedAppendWithoutPayload(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	_, store, product := createSyntheticPricingProduct(t, db, 100, 1, nil)
	service := NewOrderService(db)
	created := createSyntheticPricingOrder(t, db, store, dto.CustomerOrderItemRequest{ProductID: product.ID, Quantity: 1})
	requestKey := strings.Repeat("e5", 32)
	appended, err := service.AppendCustomerStoreOrderItems(created.OrderNo, created.CustomerAccessToken, dto.AppendCustomerOrderItemsRequest{
		RequestKey: requestKey,
		Items:      []dto.CustomerOrderItemRequest{{ProductID: product.ID, Quantity: 2}},
	})
	if err != nil {
		t.Fatalf("append synthetic items: %v", err)
	}
	if err := db.Model(&model.Order{}).Where("id = ?", created.ID).Update("status", "closed").Error; err != nil {
		t.Fatal(err)
	}

	recovered, err := service.RecoverCustomerStoreOrderAppend(created.OrderNo, created.CustomerAccessToken, requestKey)
	if err != nil {
		t.Fatalf("recover committed append without replaying its payload: %v", err)
	}
	if recovered.ID != appended.ID || recovered.Status != "closed" || recovered.Amount != appended.Amount || recovered.TotalAmount != appended.TotalAmount {
		t.Fatalf("append recovery changed or failed to return the current order: appended=%+v recovered=%+v", appended, recovered)
	}
	items := decodePricingSnapshots(t, recovered.Items)
	if len(items) != 2 || items[1].Quantity != 2 || items[1].Price != 100 {
		t.Fatalf("append recovery must not duplicate items: %+v", items)
	}
	if _, err := service.RecoverCustomerStoreOrderAppend(created.OrderNo, "wrong synthetic access token", requestKey); err == nil {
		t.Fatal("append recovery must reject a mismatched order access token")
	}
	if _, err := service.RecoverCustomerStoreOrderAppend(created.OrderNo, created.CustomerAccessToken, strings.Repeat("f6", 32)); err == nil {
		t.Fatal("append recovery must reject an unused request key")
	}
}
