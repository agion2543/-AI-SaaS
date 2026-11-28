package service

import (
	"errors"
	"time"

	"go-web-gin-health/internal/model"
)

// errMerchantSubscriptionBlocksNewOrders is the customer-facing refusal for
// new store intake (orders, table checkouts, appended items, leads) when the
// merchant's subscription is not currently active. It deliberately says
// nothing about the merchant's internal status: the customer only needs to
// know the store cannot take new orders right now.
var errMerchantSubscriptionBlocksNewOrders = errors.New("商家服务已到期，暂不接受新订单，请联系门店续费")

// merchantAcceptsNewStoreOrders reports whether a merchant may take NEW store
// intake right now. It is the same rule the merchant-side subscription
// middleware enforces (model.Merchant.HasActiveSubscriptionAt): the merchant
// must be active and the subscription must be active with a future expiry.
//
// A trial merchant is an active merchant with a future expiry, so trials
// pass this gate; expired, paused, inactive and suspended merchants do not.
// In-flight orders are not affected: this gate is only consulted when new
// intake is created, never when an existing order is paid, confirmed or
// refunded.
func merchantAcceptsNewStoreOrders(merchant model.Merchant, now time.Time) bool {
	if merchant.Status != "active" {
		return false
	}
	return merchant.HasActiveSubscriptionAt(now)
}

// requireMerchantAcceptsNewStoreOrders returns
// errMerchantSubscriptionBlocksNewOrders unless the merchant may take new
// store intake.
func requireMerchantAcceptsNewStoreOrders(merchant model.Merchant, now time.Time) error {
	if merchantAcceptsNewStoreOrders(merchant, now) {
		return nil
	}
	return errMerchantSubscriptionBlocksNewOrders
}
