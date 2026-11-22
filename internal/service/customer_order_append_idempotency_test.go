package service

import (
	"encoding/json"
	"strings"
	"testing"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

func TestCustomerOrderAppendExactRetryDoesNotDuplicateItemsOrAmount(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	_, store, product := createSyntheticPricingProduct(t, db, 100, 1, nil)
	service := NewOrderService(db)
	order := createSyntheticPricingOrder(t, db, store, dto.CustomerOrderItemRequest{ProductID: product.ID, Quantity: 1})
	request := dto.AppendCustomerOrderItemsRequest{
		CustomerNote: "追加备注",
		RequestKey:   strings.Repeat("ab", 32),
		Items: []dto.CustomerOrderItemRequest{
			{ProductID: product.ID, Quantity: 1, Price: 1},
			{ProductID: product.ID, Quantity: 1, Price: 2},
		},
	}

	first, err := service.AppendCustomerStoreOrderItems(order.OrderNo, order.CustomerAccessToken, request)
	if err != nil {
		t.Fatalf("append synthetic items: %v", err)
	}
	if first.Amount != 300 || first.TotalAmount != 300 {
		t.Fatalf("append should preserve server pricing and current total semantics: amount=%d total=%d", first.Amount, first.TotalAmount)
	}

	// The API ignores client prices. The exact same logical request remains a
	// replay even if the browser serializes duplicate lines differently.
	retry := request
	retry.Items = []dto.CustomerOrderItemRequest{{ProductID: product.ID, Quantity: 2, Price: 999999}}
	second, err := service.AppendCustomerStoreOrderItems(order.OrderNo, order.CustomerAccessToken, retry)
	if err != nil {
		t.Fatalf("replay synthetic append: %v", err)
	}
	if second.ID != first.ID || second.Amount != first.Amount || second.TotalAmount != first.TotalAmount {
		t.Fatalf("exact retry changed the order or amount: first=%+v retry=%+v", first, second)
	}
	items := decodePricingSnapshots(t, second.Items)
	if len(items) != 2 || items[1].Quantity != 2 || items[1].Price != 100 || items[1].LineAmount != 200 {
		t.Fatalf("exact retry must leave exactly one server-priced appended line: %+v", items)
	}

	var logs []OrderOperationLog
	if err := json.Unmarshal([]byte(second.OperationLogs), &logs); err != nil {
		t.Fatalf("decode synthetic order logs: %v", err)
	}
	appendLogCount := 0
	for _, log := range logs {
		if log.Action == "items_appended" {
			appendLogCount++
			if log.AppendRequestKeyHash == "" || log.AppendRequestFingerprint == "" {
				t.Fatal("successful keyed append must persist its deduplication record with the order")
			}
		}
	}
	if appendLogCount != 1 {
		t.Fatalf("exact retry duplicated append audit entries: count=%d logs=%+v", appendLogCount, logs)
	}

	conflict := request
	conflict.Items = []dto.CustomerOrderItemRequest{{ProductID: product.ID, Quantity: 3}}
	if _, err := service.AppendCustomerStoreOrderItems(order.OrderNo, order.CustomerAccessToken, conflict); err == nil || !strings.Contains(err.Error(), "different request") {
		t.Fatalf("reusing the append request key for a changed payload must fail, got %v", err)
	}
	if _, err := service.AppendCustomerStoreOrderItems(order.OrderNo, "wrong synthetic access token", request); err == nil {
		t.Fatal("an append replay must still require the order access token")
	}
	invalidKey := request
	invalidKey.RequestKey = "not-a-random-key"
	if _, err := service.AppendCustomerStoreOrderItems(order.OrderNo, order.CustomerAccessToken, invalidKey); err == nil {
		t.Fatal("invalid append request keys must be rejected")
	}

	var persisted model.Order
	if err := db.First(&persisted, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	if persisted.Amount != first.Amount || persisted.TotalAmount != first.TotalAmount || len(decodePricingSnapshots(t, persisted.Items)) != 2 {
		t.Fatalf("rejected retries changed the persisted order: %+v", persisted)
	}
}

func TestCustomerOrderAppendExactRetryReturnsCurrentOrderAfterItCloses(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	_, store, product := createSyntheticPricingProduct(t, db, 100, 1, nil)
	service := NewOrderService(db)
	order := createSyntheticPricingOrder(t, db, store, dto.CustomerOrderItemRequest{ProductID: product.ID, Quantity: 1})
	request := dto.AppendCustomerOrderItemsRequest{
		RequestKey: strings.Repeat("cd", 32),
		Items:      []dto.CustomerOrderItemRequest{{ProductID: product.ID, Quantity: 1}},
	}
	first, err := service.AppendCustomerStoreOrderItems(order.OrderNo, order.CustomerAccessToken, request)
	if err != nil {
		t.Fatalf("first synthetic append: %v", err)
	}
	if err := db.Model(&model.Order{}).Where("id = ?", order.ID).Update("status", "closed").Error; err != nil {
		t.Fatal(err)
	}
	replayed, err := service.AppendCustomerStoreOrderItems(order.OrderNo, order.CustomerAccessToken, request)
	if err != nil {
		t.Fatalf("exact retry should return the committed append even after the order closes: %v", err)
	}
	if replayed.ID != first.ID || replayed.Status != "closed" || replayed.Amount != first.Amount || len(decodePricingSnapshots(t, replayed.Items)) != 2 {
		t.Fatalf("closed-order retry did not return the current unchanged order: %+v", replayed)
	}
}
