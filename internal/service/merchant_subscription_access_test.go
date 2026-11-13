package service

import (
	"testing"
	"time"

	"go-web-gin-health/internal/model"
)

func TestCheckSubscriptionRequiresActiveStatusAndFutureExpiry(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Hour)
	cases := []struct {
		name           string
		status         string
		expireAt       *time.Time
		legacyExpireAt *time.Time
		wantActive     bool
	}{
		{name: "active future", status: "active", expireAt: &future, wantActive: true},
		{name: "active legacy future", status: "active", legacyExpireAt: &future, wantActive: true},
		{name: "inactive future", status: "inactive", expireAt: &future},
		{name: "paused future", status: "paused", expireAt: &future},
		{name: "suspended future", status: "suspended", expireAt: &future},
		{name: "pending unpaid future", status: "pending", expireAt: &future},
		{name: "active expired", status: "active", expireAt: &past},
		{name: "active without expiry", status: "active"},
	}
	service := NewMerchantService(nil, db)
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			merchant := model.Merchant{
				Name:                  "subscription-check-" + test.name,
				Status:                "active",
				SubscriptionStatus:    test.status,
				SubscriptionExpireAt:  test.expireAt,
				SubscriptionExpiredAt: test.legacyExpireAt,
			}
			if err := db.Create(&merchant).Error; err != nil {
				t.Fatalf("create synthetic merchant: %v", err)
			}
			active, err := service.CheckSubscription(merchant.ID)
			if err != nil {
				t.Fatalf("check subscription: %v", err)
			}
			if active != test.wantActive {
				t.Fatalf("active=%v, want %v", active, test.wantActive)
			}
		})
	}
}

func TestSyncMerchantSubscriptionFieldsDoesNotReactivateNonActiveStatus(t *testing.T) {
	future := time.Now().Add(time.Hour)
	for _, status := range []string{"inactive", "paused", "suspended", "pending", "unknown"} {
		t.Run(status, func(t *testing.T) {
			merchant := &model.Merchant{
				SubscriptionStatus:   status,
				SubscriptionExpireAt: &future,
			}
			syncMerchantSubscriptionFields(merchant, nil)
			if merchant.SubscriptionStatus != status {
				t.Fatalf("subscription status changed to %q; want it to remain %q", merchant.SubscriptionStatus, status)
			}
		})
	}
}
