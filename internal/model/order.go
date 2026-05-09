package model

import "time"

type Order struct {
	BaseModel
	OrderNo           string              `gorm:"size:50;uniqueIndex" json:"order_no"`
	OrderType         string              `gorm:"size:40;default:user_membership;index" json:"order_type"`
	UserID            *uint               `json:"user_id"`
	User              *User               `json:"user,omitempty"`
	MerchantID        *uint               `gorm:"index" json:"merchant_id"`
	Merchant          *Merchant           `json:"merchant,omitempty"`
	SettlementID      *uint               `gorm:"index" json:"settlement_id"`
	Settlement        *MerchantSettlement `json:"settlement,omitempty"`
	StoreID           *uint               `gorm:"index" json:"store_id"`
	Store             *Store              `json:"store,omitempty"`
	CustomerPhone     string              `gorm:"size:30" json:"customer_phone"`
	PackageID         *uint               `json:"package_id"`
	Package           *MembershipPackage  `json:"package,omitempty"`
	MerchantPlanID    *uint               `gorm:"index" json:"merchant_plan_id"`
	MerchantPlan      *MerchantPlan       `json:"merchant_plan,omitempty"`
	Amount            int64               `json:"amount"`
	TotalAmount       int64               `json:"total_amount"`
	DiscountAmount    int64               `gorm:"default:0" json:"discount_amount"`
	RefundedAmount    int64               `gorm:"default:0" json:"refunded_amount"`
	RefundStatus      string              `gorm:"size:20;default:none;index" json:"refund_status"`
	PromotionID       *uint               `gorm:"index" json:"promotion_id"`
	Promotion         *Promotion          `json:"promotion,omitempty"`
	Items             string              `gorm:"type:json" json:"items"`
	CustomerNote      string              `gorm:"size:500" json:"customer_note"`
	MerchantNote      string              `gorm:"size:500" json:"merchant_note"`
	OperationLogs     string              `gorm:"type:text" json:"operation_logs"`
	Status            string              `gorm:"size:20;default:pending" json:"status"`
	PaymentChannel    string              `gorm:"size:40" json:"payment_channel"`
	AutoRenew         bool                `gorm:"default:false" json:"auto_renew"`
	PaidAt            *time.Time          `json:"paid_at"`
	TransactionNo     string              `gorm:"size:80" json:"transaction_no"`
	SubscriptionEndAt *time.Time          `json:"subscription_end_at"`
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
	OrderID       uint   `gorm:"index" json:"order_id"`
	Order         Order  `json:"order"`
	MerchantID    *uint  `gorm:"index" json:"merchant_id"`
	StoreID       *uint  `gorm:"index" json:"store_id"`
	RefundNo      string `gorm:"size:60;uniqueIndex" json:"refund_no"`
	TransactionNo string `gorm:"size:80;index" json:"transaction_no"`
	Amount        int64  `gorm:"not null" json:"amount"`
	Reason        string `gorm:"size:255" json:"reason"`
	OperatorRole  string `gorm:"size:30" json:"operator_role"`
	OperatorID    uint   `json:"operator_id"`
	Status        string `gorm:"size:20;default:success;index" json:"status"`
	RawPayload    string `gorm:"type:text" json:"raw_payload"`
}
