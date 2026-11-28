package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

// createGateTestStore builds a merchant+store+product whose subscription is in
// the requested state. The "active" case models a trial merchant: status
// active with a future expiry, which must pass the gate.
func createGateTestStore(t *testing.T, db *gorm.DB, merchantStatus, subscriptionStatus string, expireAt *time.Time) (model.Store, model.StoreProduct) {
	t.Helper()
	merchant := model.Merchant{
		Name:                 "合成门控测试商家-" + uuid.NewString(),
		Status:               merchantStatus,
		SubscriptionStatus:   subscriptionStatus,
		SubscriptionExpireAt: expireAt,
	}
	if err := db.Create(&merchant).Error; err != nil {
		t.Fatalf("create synthetic gate merchant: %v", err)
	}
	store := model.Store{MerchantID: merchant.ID, Name: "合成门控测试门店", Status: "active", IsOpen: true}
	if err := db.Create(&store).Error; err != nil {
		t.Fatalf("create synthetic gate store: %v", err)
	}
	stock := 10
	product := model.StoreProduct{StoreID: store.ID, Name: "合成商品", Price: 500, Status: "active", Stock: &stock}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create synthetic gate product: %v", err)
	}
	return store, product
}

func TestSubscriptionGateBlocksNewCustomerOrders(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Hour)
	cases := []struct {
		name               string
		merchantStatus     string
		subscriptionStatus string
		expireAt           *time.Time
		allowed            bool
	}{
		{name: "trial-like active subscription with future expiry", merchantStatus: "active", subscriptionStatus: "active", expireAt: &future, allowed: true},
		{name: "expired subscription", merchantStatus: "active", subscriptionStatus: "active", expireAt: &past},
		{name: "paused subscription with future expiry", merchantStatus: "active", subscriptionStatus: "paused", expireAt: &future},
		{name: "inactive subscription with future expiry", merchantStatus: "active", subscriptionStatus: "inactive", expireAt: &future},
		{name: "active subscription without expiry", merchantStatus: "active", subscriptionStatus: "active"},
		{name: "suspended merchant with future expiry", merchantStatus: "suspended", subscriptionStatus: "active", expireAt: &future},
		{name: "pending merchant with future expiry", merchantStatus: "pending", subscriptionStatus: "active", expireAt: &future},
	}
	service := NewOrderService(db)
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			store, product := createGateTestStore(t, db, test.merchantStatus, test.subscriptionStatus, test.expireAt)
			_, err := service.CreateCustomerStoreOrder(dto.CreateCustomerOrderRequest{
				StoreID: store.ID,
				Items:   []dto.CustomerOrderItemRequest{{ProductID: product.ID, Quantity: 1}},
			})
			if test.allowed {
				if err != nil {
					t.Fatalf("subscribed merchant order should be accepted: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("unsubscribed merchant must not take new customer orders")
			}
			if test.merchantStatus == "active" && !errors.Is(err, errMerchantSubscriptionBlocksNewOrders) {
				t.Fatalf("refusal should be the subscription gate error; got %v", err)
			}
		})
	}
}

func TestSubscriptionGateBlocksTableCheckoutForExpiredMerchant(t *testing.T) {
	fixture := newTableOrderingFixture(t)
	// Expire the fixture merchant's subscription: the store itself stays
	// active and open, so only the subscription gate can refuse checkout.
	past := time.Now().Add(-time.Hour)
	if err := fixture.db.Model(&model.Merchant{}).Where("id = ?", fixture.merchant.ID).Updates(map[string]interface{}{
		"subscription_status":  "active",
		"subscription_expire_at": past,
	}).Error; err != nil {
		t.Fatalf("expire synthetic fixture subscription: %v", err)
	}
	if _, err := fixture.service.UpdateCartItem(fixture.sessionID, fixture.tableCode, dto.UpdateTableCartItemRequest{
		ProductID: fixture.product.ID, Action: "add", Quantity: 1,
	}); err != nil {
		t.Fatalf("add synthetic cart item: %v", err)
	}
	_, _, err := fixture.service.Checkout(fixture.sessionID, fixture.tableCode, tableCheckoutRequest(strings.Repeat("a", 64), strings.Repeat("b", 64)))
	if err == nil {
		t.Fatal("expired merchant table checkout must be refused")
	}
	if !errors.Is(err, errMerchantSubscriptionBlocksNewOrders) {
		t.Fatalf("refusal should be the subscription gate error; got %v", err)
	}
}

func TestSubscriptionGateBlocksNewLeadsForExpiredMerchant(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.CustomerLead{}); err != nil {
		t.Fatalf("migrate synthetic lead schema: %v", err)
	}
	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Hour)
	merchantService := NewMerchantService(nil, db)

	activeStore, _ := createGateTestStore(t, db, "active", "active", &future)
	if _, err := merchantService.CreateCustomerLead(activeStore.ID, dto.CreateCustomerLeadRequest{
		CustomerName: "合成顾客", CustomerPhone: "13800138000", Message: "想了解",
	}); err != nil {
		t.Fatalf("subscribed merchant should accept leads: %v", err)
	}

	expiredStore, _ := createGateTestStore(t, db, "active", "active", &past)
	if _, err := merchantService.CreateCustomerLead(expiredStore.ID, dto.CreateCustomerLeadRequest{
		CustomerName: "合成顾客", CustomerPhone: "13800138001", Message: "想了解",
	}); err == nil {
		t.Fatal("expired merchant must not accept new leads")
	}
}
