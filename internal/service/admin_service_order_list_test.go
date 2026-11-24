package service

import (
	"fmt"
	"testing"
	"time"

	"go-web-gin-health/internal/model"
)

func TestAdminMerchantOrderListsUseStableScopedOrder(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	merchant := model.Merchant{Name: "稳定排序测试商家", Status: "active"}
	otherMerchant := model.Merchant{Name: "稳定排序测试其他商家", Status: "active"}
	if err := db.Create(&merchant).Error; err != nil {
		t.Fatalf("create merchant: %v", err)
	}
	if err := db.Create(&otherMerchant).Error; err != nil {
		t.Fatalf("create other merchant: %v", err)
	}

	createdAt := time.Date(2026, time.November, 22, 10, 0, 0, 0, time.UTC)
	createOrder := func(ownerID uint, orderType, orderNo string) model.Order {
		t.Helper()
		ownerIDCopy := ownerID
		order := model.Order{
			BaseModel:  model.BaseModel{CreatedAt: createdAt},
			OrderNo:    orderNo,
			OrderType:  orderType,
			MerchantID: &ownerIDCopy,
		}
		if err := db.Create(&order).Error; err != nil {
			t.Fatalf("create synthetic order %q: %v", orderNo, err)
		}
		return order
	}

	var storeOrderIDs []uint
	for i := 0; i < 25; i++ {
		order := createOrder(merchant.ID, "store_order", fmt.Sprintf("stable-store-%02d", i))
		storeOrderIDs = append(storeOrderIDs, order.ID)
	}

	var subscriptionOrderIDs []uint
	order := createOrder(merchant.ID, "merchant_subscription", "stable-store-wrong-type")
	subscriptionOrderIDs = append(subscriptionOrderIDs, order.ID)
	for i := 0; i < 25; i++ {
		order := createOrder(merchant.ID, "merchant_subscription", fmt.Sprintf("stable-subscription-%02d", i))
		subscriptionOrderIDs = append(subscriptionOrderIDs, order.ID)
	}

	order = createOrder(merchant.ID, "store_order", "stable-subscription-wrong-type")
	storeOrderIDs = append(storeOrderIDs, order.ID)
	order = createOrder(merchant.ID, "merchant_subscription", "stable-subscription-latest")
	subscriptionOrderIDs = append(subscriptionOrderIDs, order.ID)
	createOrder(otherMerchant.ID, "store_order", "stable-store-other-merchant")
	createOrder(otherMerchant.ID, "merchant_subscription", "stable-subscription-other-merchant")

	wantDescendingTail := func(ids []uint, count int) []uint {
		t.Helper()
		if count > len(ids) {
			t.Fatalf("cannot select %d IDs from %d", count, len(ids))
		}
		want := make([]uint, count)
		for i := range want {
			want[i] = ids[len(ids)-1-i]
		}
		return want
	}
	assertOrders := func(label string, got []model.Order, wantIDs []uint, wantMerchantID uint, wantOrderType string) {
		t.Helper()
		if len(got) != len(wantIDs) {
			t.Fatalf("%s count = %d, want %d", label, len(got), len(wantIDs))
		}
		for i, gotOrder := range got {
			if gotOrder.ID != wantIDs[i] {
				t.Fatalf("%s[%d].ID = %d, want %d", label, i, gotOrder.ID, wantIDs[i])
			}
			if gotOrder.MerchantID == nil || *gotOrder.MerchantID != wantMerchantID {
				t.Fatalf("%s[%d].MerchantID = %v, want %d", label, i, gotOrder.MerchantID, wantMerchantID)
			}
			if gotOrder.OrderType != wantOrderType {
				t.Fatalf("%s[%d].OrderType = %q, want %q", label, i, gotOrder.OrderType, wantOrderType)
			}
		}
	}

	admin := NewAdminService(nil, db)
	storeOrders, err := admin.ListMerchantStoreOrders(merchant.ID, 10)
	if err != nil {
		t.Fatalf("list merchant store orders: %v", err)
	}
	assertOrders("store orders", storeOrders, wantDescendingTail(storeOrderIDs, 10), merchant.ID, "store_order")

	subscriptionOrders, err := admin.ListMerchantSubscriptionOrders(merchant.ID)
	if err != nil {
		t.Fatalf("list merchant subscription orders: %v", err)
	}
	assertOrders("subscription orders", subscriptionOrders, wantDescendingTail(subscriptionOrderIDs, 20), merchant.ID, "merchant_subscription")
}
