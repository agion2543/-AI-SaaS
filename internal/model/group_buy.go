package model

import "time"

// MerchantGroupBuyLink stores one merchant-owned external group-buy destination
// per store and platform. Only aggregate redirect counts are retained.
type MerchantGroupBuyLink struct {
	BaseModel
	MerchantID       uint       `gorm:"not null;uniqueIndex:idx_merchant_store_channel,priority:1;index" json:"merchant_id"`
	StoreID          uint       `gorm:"not null;uniqueIndex:idx_merchant_store_channel,priority:2;index" json:"store_id"`
	Channel          string     `gorm:"size:24;not null;uniqueIndex:idx_merchant_store_channel,priority:3" json:"channel"`
	DestinationURL   string     `gorm:"size:1000;not null" json:"destination_url"`
	Code             string     `gorm:"size:32;not null;uniqueIndex" json:"code"`
	Enabled          bool       `gorm:"not null;index" json:"enabled"`
	ClickCount       uint64     `gorm:"not null;default:0" json:"click_count"`
	LastClickedAt    *time.Time `json:"last_clicked_at,omitempty"`
	ReviewStatus     string     `gorm:"size:20;not null;default:pending;index" json:"review_status"`
	ReviewMethod     string     `gorm:"size:16;index" json:"review_method"`
	ReviewRuleID     string     `gorm:"size:32;index" json:"review_rule_id"`
	ReviewReason     string     `gorm:"size:255" json:"review_reason"`
	ReviewedBy       *uint      `gorm:"index" json:"reviewed_by"`
	ReviewedAt       *time.Time `json:"reviewed_at"`
	AdminSuspended   bool       `gorm:"not null;default:false;index" json:"admin_suspended"`
	SuspensionReason string     `gorm:"size:255" json:"suspension_reason"`
	SuspendedBy      *uint      `gorm:"index" json:"suspended_by"`
	SuspendedAt      *time.Time `json:"suspended_at"`
}

// MerchantGroupBuyLinkAudit is append-only in the application: there are no
// update/delete endpoints, and each state transition records before/after JSON.
type MerchantGroupBuyLinkAudit struct {
	BaseModel
	LinkID      uint   `gorm:"not null;index" json:"link_id"`
	MerchantID  uint   `gorm:"not null;index" json:"merchant_id"`
	StoreID     uint   `gorm:"not null;index" json:"store_id"`
	Channel     string `gorm:"size:24;not null;index" json:"channel"`
	Action      string `gorm:"size:40;not null;index" json:"action"`
	RuleID      string `gorm:"size:32;index" json:"rule_id"`
	ActorType   string `gorm:"size:30;not null;index" json:"actor_type"`
	ActorID     uint   `gorm:"index" json:"actor_id"`
	ActorName   string `gorm:"size:120" json:"actor_name"`
	Reason      string `gorm:"size:255" json:"reason"`
	BeforeState string `gorm:"type:json" json:"before_state"`
	AfterState  string `gorm:"type:json" json:"after_state"`
}
