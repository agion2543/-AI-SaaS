package model

import "time"

// PaymentStatusPaidReviewRequired records a verified provider success that
// arrived after the order became ineligible for automatic fulfillment.
const PaymentStatusPaidReviewRequired = "paid_review_required"

// OrderChannel records where a store order came from. It is a plain
// varchar column on orders (v11): no database CHECK constraint, because a
// CHECK on the v1 orders table would change every pinned historical schema
// fingerprint. The allowed set is enforced in service code instead
// (see service.ValidOrderChannel).
const (
	OrderChannelStoreQR = "store_qr" // customer scanned the store QR code
	OrderChannelTable   = "table"    // table-QR checkout (table_order_details)
	OrderChannelShare   = "share"    // order placed through a share link/code
)

type Order struct {
	BaseModel
	OrderNo                  string                                 `gorm:"size:50;uniqueIndex" json:"order_no"`
	CustomerAccessTokenHash  string                                 `gorm:"size:64;not null;default:''" json:"-"`
	CustomerAccessToken      string                                 `gorm:"-" json:"-"`
	OrderType                string                                 `gorm:"size:40;default:user_membership;index" json:"order_type"`
	UserID                   *uint                                  `json:"user_id"`
	User                     *User                                  `json:"user,omitempty"`
	MerchantID               *uint                                  `gorm:"index" json:"merchant_id"`
	Merchant                 *Merchant                              `json:"merchant,omitempty"`
	SettlementID             *uint                                  `gorm:"index" json:"settlement_id"`
	Settlement               *MerchantSettlement                    `json:"settlement,omitempty"`
	StoreID                  *uint                                  `gorm:"index" json:"store_id"`
	Store                    *Store                                 `json:"store,omitempty"`
	TableNo                  int                                    `gorm:"-" json:"table_no"`
	TableLabel               string                                 `gorm:"-" json:"table_label,omitempty"`
	CustomerPhone            string                                 `gorm:"size:30" json:"customer_phone"`
	PackageID                *uint                                  `json:"package_id"`
	Package                  *MembershipPackage                     `json:"package,omitempty"`
	MerchantPlanID           *uint                                  `gorm:"index" json:"merchant_plan_id"`
	MerchantPlan             *MerchantPlan                          `json:"merchant_plan,omitempty"`
	PlanNameSnapshot         *string                                `gorm:"column:merchant_plan_name_snapshot;size:80" json:"merchant_plan_name_snapshot,omitempty"`
	PlanPriceCentsSnapshot   *int64                                 `gorm:"column:merchant_plan_price_cents_snapshot" json:"merchant_plan_price_cents_snapshot,omitempty"`
	PlanDurationDaysSnapshot *int                                   `gorm:"column:merchant_plan_duration_days_snapshot" json:"merchant_plan_duration_days_snapshot,omitempty"`
	Amount                   int64                                  `json:"amount"`
	TotalAmount              int64                                  `json:"total_amount"`
	DiscountAmount           int64                                  `gorm:"default:0" json:"discount_amount"`
	RefundedAmount           int64                                  `gorm:"default:0" json:"refunded_amount"`
	RefundStatus             string                                 `gorm:"size:20;default:none;index" json:"refund_status"`
	PromotionID              *uint                                  `gorm:"index" json:"promotion_id"`
	Promotion                *Promotion                             `json:"promotion,omitempty"`
	CouponID                 *uint                                  `gorm:"column:referral_coupon_id;index" json:"referral_coupon_id"`
	CouponNo                 string                                 `gorm:"size:60;index" json:"coupon_no"`
	Items                    string                                 `gorm:"type:json" json:"items"`
	CustomerNote             string                                 `gorm:"size:500" json:"customer_note"`
	MerchantNote             string                                 `gorm:"size:500" json:"merchant_note"`
	CustomerRating           int                                    `gorm:"default:0" json:"customer_rating"`
	CustomerReview           string                                 `gorm:"size:1000" json:"customer_review"`
	ReviewedAt               *time.Time                             `json:"reviewed_at"`
	Channel                  string                                 `gorm:"size:24;not null;default:store_qr" json:"channel"`
	ShareCode                string                                 `gorm:"size:40;index" json:"share_code"`
	OperationLogs            string                                 `gorm:"type:text" json:"operation_logs"`
	Status                   string                                 `gorm:"size:20;default:pending" json:"status"`
	PaymentChannel           string                                 `gorm:"size:40" json:"payment_channel"`
	AutoRenew                bool                                   `gorm:"default:false" json:"auto_renew"`
	PaidAt                   *time.Time                             `json:"paid_at"`
	TransactionNo            string                                 `gorm:"size:80" json:"transaction_no"`
	SubscriptionEndAt        *time.Time                             `json:"subscription_end_at"`
	ManualReview             *MerchantSubscriptionManualReview      `gorm:"-" json:"manual_review,omitempty"`
	ManualReviewAudit        *MerchantSubscriptionManualReviewAudit `gorm:"-" json:"manual_review_audit,omitempty"`
}

type PaymentRecord struct {
	BaseModel
	OrderID        uint   `json:"order_id"`
	Order          Order  `json:"order"`
	PaymentChannel string `gorm:"size:40" json:"payment_channel"`
	TransactionNo  string `gorm:"size:80;uniqueIndex" json:"transaction_no"`
	Amount         int64  `json:"amount"`
	Status         string `gorm:"size:20;default:success" json:"status"`
	RawPayload     string `gorm:"type:text" json:"raw_payload"`
}

type RefundRecord struct {
	BaseModel
	OrderID            uint    `gorm:"index;uniqueIndex:idx_refund_order_idempotency" json:"order_id"`
	Order              Order   `json:"order"`
	MerchantID         *uint   `gorm:"index" json:"merchant_id"`
	StoreID            *uint   `gorm:"index" json:"store_id"`
	RefundNo           string  `gorm:"size:60;uniqueIndex" json:"refund_no"`
	IdempotencyKey     *string `gorm:"size:120;uniqueIndex:idx_refund_order_idempotency" json:"idempotency_key,omitempty"`
	RequestFingerprint string  `gorm:"size:64;not null;default:''" json:"-"`
	TransactionNo      string  `gorm:"size:80;index" json:"transaction_no"`
	Amount             int64   `gorm:"not null" json:"amount"`
	Reason             string  `gorm:"size:255" json:"reason"`
	OperatorRole       string  `gorm:"size:30" json:"operator_role"`
	OperatorID         uint    `json:"operator_id"`
	Status             string  `gorm:"size:20;default:success;index" json:"status"`
	RawPayload         string  `gorm:"type:text" json:"raw_payload"`
}

// OrderItem is one priced line of a store order (schema v11 table
// order_items). Amounts are in the same unit as Order.TotalAmount (cents).
// ProductNameSnapshot freezes the product name at order time; the live
// product name may change later. The table is the source for per-product
// sales aggregation. Rows are written by the order-creation paths
// (CreateCustomerStoreOrder, table-order checkout, and the append path) in
// the same transaction as the order; historical orders created before the
// wiring have no detail rows and are not backfilled.
type OrderItem struct {
	BaseModel
	OrderID             uint          `gorm:"not null;index" json:"order_id"`
	Order               Order         `json:"order,omitempty"`
	ProductID           uint          `gorm:"not null;index" json:"product_id"`
	Product             *StoreProduct `json:"product,omitempty"`
	ProductNameSnapshot string        `gorm:"size:128;not null;default:''" json:"product_name_snapshot"`
	Quantity            int           `gorm:"not null" json:"quantity"`
	UnitPrice           int64         `gorm:"not null" json:"unit_price"`
	Amount              int64         `gorm:"not null" json:"amount"`
}
