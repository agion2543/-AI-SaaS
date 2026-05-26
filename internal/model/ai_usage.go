package model

import "time"

type MerchantAIUsageLog struct {
	BaseModel
	MerchantID uint      `gorm:"index;not null" json:"merchant_id"`
	Merchant   Merchant  `json:"merchant,omitempty"`
	UserID     *uint     `gorm:"index" json:"user_id"`
	Scenario   string    `gorm:"size:120" json:"scenario"`
	Model      string    `gorm:"size:120" json:"model"`
	Provider   string    `gorm:"size:80" json:"provider"`
	Fallback   bool      `gorm:"default:false" json:"fallback"`
	Success    bool      `gorm:"default:true" json:"success"`
	Error      string    `gorm:"size:500" json:"error"`
	DurationMS int       `gorm:"default:0" json:"duration_ms"`
	UsedAt     time.Time `gorm:"index" json:"used_at"`
}
