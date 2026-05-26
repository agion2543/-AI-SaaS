package model

import "time"

type AuditLog struct {
	BaseModel
	ActorID    uint   `gorm:"index" json:"actor_id"`
	ActorType  string `gorm:"size:30;index" json:"actor_type"`
	ActorName  string `gorm:"size:120" json:"actor_name"`
	Action     string `gorm:"size:80;index" json:"action"`
	TargetType string `gorm:"size:60;index" json:"target_type"`
	TargetID   uint   `gorm:"index" json:"target_id"`
	TargetName string `gorm:"size:160" json:"target_name"`
	MerchantID *uint  `gorm:"index" json:"merchant_id"`
	IP         string `gorm:"size:80" json:"ip"`
	UserAgent  string `gorm:"size:255" json:"user_agent"`
	Detail     string `gorm:"type:json" json:"detail"`
	ReviewStatus string     `gorm:"size:20;default:pending;index" json:"review_status"`
	ReviewRemark string     `gorm:"size:255" json:"review_remark"`
	ReviewedBy   *uint      `gorm:"index" json:"reviewed_by"`
	ReviewedAt   *time.Time `json:"reviewed_at"`
}
