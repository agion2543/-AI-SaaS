package model

import "time"

type Promotion struct {
	BaseModel
	MerchantID   uint       `gorm:"not null;index" json:"merchant_id"`
	Merchant     Merchant   `json:"merchant,omitempty"`
	StoreID      *uint      `gorm:"index" json:"store_id"`
	Store        Store      `json:"store,omitempty"`
	Title        string     `gorm:"size:120;not null" json:"title"`
	Description  string     `gorm:"size:500" json:"description"`
	Type         string     `gorm:"size:20;default:amount" json:"type"`
	Threshold    int64      `gorm:"default:0" json:"threshold"`
	Discount     int64      `gorm:"default:0" json:"discount"`
	DiscountRate int        `gorm:"default:0" json:"discount_rate"`
	Status       string     `gorm:"size:20;default:draft;index" json:"status"`
	ValidFrom    *time.Time `json:"valid_from"`
	ValidTo      *time.Time `json:"valid_to"`
}
