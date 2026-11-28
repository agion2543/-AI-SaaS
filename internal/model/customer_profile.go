package model

import "time"

// CustomerProfile is the materialized per-(merchant, customer_phone)
// aggregation of valid store orders (order_type='store_order', paid_at set,
// total_amount > 0). It stores facts only — never a derived label: AITag /
// AISuggestion are still computed at read time by buildCustomerInsight from
// these facts plus the current time, exactly as the old live GROUP BY did.
//
// last_order_at follows the old MAX(created_at) definition (order time, not
// payment time); last_paid_at is stored alongside it so the dormant-customer
// tiers (which used MAX(paid_at)) can read the same row without a second
// aggregation. store_name is the store of the latest order (last_order_id),
// not MAX(stores.name): the old MAX() was non-deterministic when a customer
// ordered at several stores of one merchant.
//
// Amounts are integer fen (cents). Refunds never touch this table: the old
// profile totals were gross amounts too, so total_amount_cents deliberately
// has no refund deduction.
type CustomerProfile struct {
	ID                  uint       `gorm:"primaryKey" json:"id"`
	CreatedAt           time.Time  `gorm:"not null" json:"created_at"`
	UpdatedAt           time.Time  `gorm:"not null" json:"updated_at"`
	MerchantID          uint       `gorm:"not null;uniqueIndex:uk_customer_profiles_merchant_phone,priority:1;index:idx_customer_profiles_merchant_last_order,priority:1" json:"merchant_id"`
	CustomerPhone       string     `gorm:"size:30;not null;uniqueIndex:uk_customer_profiles_merchant_phone,priority:2" json:"customer_phone"`
	LastStoreID         *uint      `json:"last_store_id"`
	LastStoreName       string     `gorm:"size:120;not null;default:''" json:"last_store_name"`
	OrderCount          int64      `gorm:"not null;default:0;check:chk_customer_profiles_order_count,order_count >= 0" json:"order_count"`
	TotalAmountCents    int64      `gorm:"not null;default:0;check:chk_customer_profiles_total_amount,total_amount_cents >= 0" json:"total_amount_cents"`
	DiscountAmountCents int64      `gorm:"not null;default:0;check:chk_customer_profiles_discount_amount,discount_amount_cents >= 0" json:"discount_amount_cents"`
	FirstPaidAt         *time.Time `json:"first_paid_at"`
	LastPaidAt          *time.Time `json:"last_paid_at"`
	LastOrderAt         *time.Time `gorm:"index:idx_customer_profiles_merchant_last_order,priority:2;index:idx_customer_profiles_last_paid_at" json:"last_order_at"`
	LastOrderID         uint       `gorm:"not null;default:0" json:"last_order_id"`
	LastStatus          string     `gorm:"size:20;not null;default:''" json:"last_status"`
}

func (CustomerProfile) TableName() string { return "customer_profiles" }
