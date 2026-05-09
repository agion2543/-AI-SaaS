package model

import "time"

type Merchant struct {
	BaseModel
	Name                  string     `gorm:"size:120;uniqueIndex" json:"name"`
	ContactPhone          string     `gorm:"size:30" json:"contact_phone"`
	ContactEmail          string     `gorm:"size:120" json:"contact_email"`
	Status                string     `gorm:"size:20;default:pending" json:"status"`
	SubscriptionPlan      string     `gorm:"size:40;default:none" json:"subscription_plan"`
	SubscriptionStatus    string     `gorm:"size:20;default:inactive;index" json:"subscription_status"`
	SubscriptionExpiredAt *time.Time `json:"subscription_expired_at"`
	SubscriptionNote      string     `gorm:"size:255" json:"subscription_note"`
	SubscriptionPlanID    *uint      `gorm:"index" json:"subscription_plan_id"`
	SubscriptionExpireAt  *time.Time `json:"subscription_expire_at"`
}

type MerchantPlan struct {
	BaseModel
	Name         string `gorm:"size:80;not null" json:"name"`
	PriceCents   int64  `gorm:"not null" json:"price_cents"`
	DurationDays int    `gorm:"not null" json:"duration_days"`
	Sort         int    `gorm:"default:0" json:"sort"`
}

type MerchantPaymentConfig struct {
	BaseModel
	MerchantID   uint     `gorm:"uniqueIndex" json:"merchant_id"`
	Merchant     Merchant `json:"merchant,omitempty"`
	Channel      string   `gorm:"size:30;default:alipay" json:"channel"`
	Mode         string   `gorm:"size:30;default:direct" json:"mode"`
	AccountName  string   `gorm:"size:120" json:"account_name"`
	AccountNo    string   `gorm:"size:120" json:"account_no"`
	AppID        string   `gorm:"size:120" json:"app_id"`
	Status       string   `gorm:"size:20;default:disabled;index" json:"status"`
	AuditStatus  string   `gorm:"size:20;default:pending;index" json:"audit_status"`
	AuditRemark  string   `gorm:"size:255" json:"audit_remark"`
	ContactPhone string   `gorm:"size:30" json:"contact_phone"`
	Remark       string   `gorm:"size:255" json:"remark"`
}

type MerchantSettlement struct {
	BaseModel
	MerchantID            uint       `gorm:"index;not null" json:"merchant_id"`
	Merchant              Merchant   `json:"merchant,omitempty"`
	SettlementPeriodStart *time.Time `json:"settlement_period_start"`
	SettlementPeriodEnd   *time.Time `json:"settlement_period_end"`
	OrderCount            int        `gorm:"default:0" json:"order_count"`
	TotalAmountCents      int64      `gorm:"default:0" json:"total_amount_cents"`
	RefundAmountCents     int64      `gorm:"default:0" json:"refund_amount_cents"`
	NetAmountCents        int64      `gorm:"default:0" json:"net_amount_cents"`
	Status                string     `gorm:"size:20;default:pending;index" json:"status"`
	PaidAt                *time.Time `json:"paid_at"`
	Remark                string     `gorm:"size:255" json:"remark"`
}
