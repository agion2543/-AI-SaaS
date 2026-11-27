package service

import (
	"strings"

	"go-web-gin-health/internal/model"
)

// ValidTradeAmountCondition is the single frozen definition of a valid
// (effectively paid) trade amount, shared by every trade-amount query in
// the codebase: an order counts toward trade revenue exactly when it has a
// real payment time and a positive total amount. Do not introduce a second
// definition (for example a status-enum one) alongside it; any new
// trade-amount query must build on this constant or on one of the helpers
// below so the dashboard, customer profiles, merchant marketing stats and
// the merchant turnover endpoint can never disagree again.
//
// Amounts are in the same unit as Order.TotalAmount (cents). The condition
// intentionally says nothing about refunds: net amounts subtract successful
// refunds separately at each call site.
const ValidTradeAmountCondition = "paid_at IS NOT NULL AND total_amount > 0"

// ValidTradeAmountConditionFor returns the valid-trade condition qualified
// with a table (or alias) name, for use in raw SQL / joined queries where
// the column names would otherwise be ambiguous. Pass "" for the
// unqualified form.
func ValidTradeAmountConditionFor(table string) string {
	table = strings.TrimSpace(table)
	if table == "" {
		return ValidTradeAmountCondition
	}
	return table + ".paid_at IS NOT NULL AND " + table + ".total_amount > 0"
}

// ValidTradeAmountSumExpression returns a COALESCE(SUM(CASE ...)) SELECT
// expression that sums total_amount only for valid trades, for the given
// table (or alias). The caller appends its own " AS <alias>".
func ValidTradeAmountSumExpression(table string) string {
	table = strings.TrimSpace(table)
	if table == "" {
		table = "orders"
	}
	return "COALESCE(SUM(CASE WHEN " + ValidTradeAmountConditionFor(table) + " THEN " + table + ".total_amount ELSE 0 END),0)"
}

// ValidTradeAmountAvgExpression returns the matching COALESCE(AVG(CASE ...))
// expression (NULL for non-trades, so they do not drag the average down).
// The caller appends its own " AS <alias>".
func ValidTradeAmountAvgExpression(table string) string {
	table = strings.TrimSpace(table)
	if table == "" {
		table = "orders"
	}
	return "COALESCE(AVG(CASE WHEN " + ValidTradeAmountConditionFor(table) + " THEN " + table + ".total_amount ELSE NULL END),0)"
}

// ValidOrderChannel reports whether channel is one of the known order
// channels (model.OrderChannel*). Unknown values must never be written:
// the orders.channel column has no database CHECK constraint (a CHECK on
// the v1 orders table would change every pinned historical schema
// fingerprint), so the allowed set is enforced here, in service code.
func ValidOrderChannel(channel string) bool {
	switch strings.TrimSpace(channel) {
	case model.OrderChannelStoreQR, model.OrderChannelTable, model.OrderChannelShare:
		return true
	default:
		return false
	}
}

// OrderChannelForShareCode derives the order channel from the share code
// carried by a customer order request: an order placed through a share
// link/code is a share order, anything else placed by scanning is a
// store-QR order. Table-QR checkouts set model.OrderChannelTable directly
// at creation and never go through this helper.
func OrderChannelForShareCode(shareCode string) string {
	if strings.TrimSpace(shareCode) != "" {
		return model.OrderChannelShare
	}
	return model.OrderChannelStoreQR
}
