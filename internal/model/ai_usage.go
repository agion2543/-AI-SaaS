package model

import "time"

type MerchantAIUsageLog struct {
	BaseModel
	MerchantID uint     `gorm:"index;not null" json:"merchant_id"`
	Merchant   Merchant `json:"merchant,omitempty"`
	UserID     *uint    `gorm:"index" json:"user_id"`
	// StoreID 为调用归属门店；商家 AI 调用目前不带门店上下文时为 NULL，
	// 按门店统计时归入“未指定门店”，不虚构归属。
	StoreID          *uint  `gorm:"index" json:"store_id"`
	Scenario         string `gorm:"size:120" json:"scenario"`
	Model            string `gorm:"size:120" json:"model"`
	Provider         string `gorm:"size:80" json:"provider"`
	Fallback         bool   `gorm:"default:false" json:"fallback"`
	Success          bool   `gorm:"default:true" json:"success"`
	Error            string `gorm:"size:500" json:"error"`
	DurationMS       int    `gorm:"default:0" json:"duration_ms"`
	PromptTokens     int    `gorm:"default:0" json:"prompt_tokens"`
	CompletionTokens int    `gorm:"default:0" json:"completion_tokens"`
	// CachedTokens 为命中缓存的输入 Token 数（供应商返回时才有，无则为 0），
	// 估算成本按缓存单价单独计，与输入 Token 分开。
	CachedTokens       int       `gorm:"default:0" json:"cached_tokens"`
	TotalTokens        int       `gorm:"default:0" json:"total_tokens"`
	EstimatedCostCents int64     `gorm:"default:0" json:"estimated_cost_cents"`
	UsedAt             time.Time `gorm:"index" json:"used_at"`
}
