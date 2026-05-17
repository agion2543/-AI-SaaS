package model

import "time"

type CustomerLead struct {
	BaseModel
	MerchantID    uint     `gorm:"not null;index" json:"merchant_id"`
	Merchant      Merchant `json:"merchant,omitempty"`
	StoreID       uint     `gorm:"not null;index" json:"store_id"`
	Store         Store    `json:"store,omitempty"`
	CustomerName  string   `gorm:"size:60" json:"customer_name"`
	CustomerPhone string   `gorm:"size:30;not null;index" json:"customer_phone"`
	Message       string   `gorm:"size:500" json:"message"`
	FollowUpNote  string   `gorm:"size:500" json:"follow_up_note"`
	Source        string   `gorm:"size:40;default:store_qr" json:"source"`
	Status        string   `gorm:"size:20;default:new;index" json:"status"`
}

type ShareCampaign struct {
	BaseModel
	MerchantID       uint       `gorm:"not null;index" json:"merchant_id"`
	Merchant         Merchant   `json:"merchant,omitempty"`
	StoreID          uint       `gorm:"not null;index" json:"store_id"`
	Store            Store      `json:"store,omitempty"`
	OrderID          *uint      `gorm:"index" json:"order_id"`
	Order            *Order     `json:"order,omitempty"`
	ShareCode        string     `gorm:"size:40;uniqueIndex" json:"share_code"`
	CustomerPhone    string     `gorm:"size:30;index" json:"customer_phone"`
	PosterTitle      string     `gorm:"size:120" json:"poster_title"`
	PosterCopy       string     `gorm:"size:500" json:"poster_copy"`
	ScanCount        int        `gorm:"default:0" json:"scan_count"`
	LeadCount        int        `gorm:"default:0" json:"lead_count"`
	ConversionCount  int        `gorm:"default:0" json:"conversion_count"`
	ConversionAmount int64      `gorm:"default:0" json:"conversion_amount"`
	Status           string     `gorm:"size:20;default:active;index" json:"status"`
	LastScannedAt    *time.Time `json:"last_scanned_at"`
}

type ShareActivityConfig struct {
	BaseModel
	MerchantID              uint     `gorm:"not null;uniqueIndex" json:"merchant_id"`
	Merchant                Merchant `json:"merchant,omitempty"`
	Enabled                 bool     `gorm:"default:false;index" json:"enabled"`
	PosterTitle             string   `gorm:"size:120" json:"poster_title"`
	PosterCopy              string   `gorm:"size:500" json:"poster_copy"`
	FriendCouponAmount      int64    `gorm:"not null;default:500" json:"friend_coupon_amount"`
	FriendCouponThreshold   int64    `gorm:"not null;default:3000" json:"friend_coupon_threshold"`
	ReferrerCouponAmount    int64    `gorm:"not null;default:500" json:"referrer_coupon_amount"`
	ReferrerCouponThreshold int64    `gorm:"not null;default:3000" json:"referrer_coupon_threshold"`
	ValidDays               int      `gorm:"not null;default:30" json:"valid_days"`
	Status                  string   `gorm:"size:20;default:active;index" json:"status"`
}

type ReferralCoupon struct {
	BaseModel
	MerchantID      uint           `gorm:"not null;index" json:"merchant_id"`
	Merchant        Merchant       `json:"merchant,omitempty"`
	StoreID         uint           `gorm:"not null;index" json:"store_id"`
	Store           Store          `json:"store,omitempty"`
	ShareCampaignID *uint          `gorm:"index" json:"share_campaign_id"`
	ShareCampaign   *ShareCampaign `json:"share_campaign,omitempty"`
	OrderID         *uint          `gorm:"index" json:"order_id"`
	CouponNo        string         `gorm:"size:60;uniqueIndex" json:"coupon_no"`
	OwnerPhone      string         `gorm:"size:30;index" json:"owner_phone"`
	OwnerType       string         `gorm:"size:30;index" json:"owner_type"`
	Title           string         `gorm:"size:120" json:"title"`
	Amount          int64          `gorm:"not null;default:0" json:"amount"`
	Threshold       int64          `gorm:"not null;default:0" json:"threshold"`
	Status          string         `gorm:"size:20;default:unused;index" json:"status"`
	ValidFrom       *time.Time     `json:"valid_from"`
	ValidTo         *time.Time     `json:"valid_to"`
	UsedAt          *time.Time     `json:"used_at"`
	Remark          string         `gorm:"size:255" json:"remark"`
}
