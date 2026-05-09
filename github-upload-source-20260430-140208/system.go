package model

type SystemConfig struct {
	BaseModel
	ConfigKey   string `gorm:"size:80;uniqueIndex" json:"config_key"`
	ConfigValue string `gorm:"type:text" json:"config_value"`
	IsEncrypted bool   `gorm:"default:false" json:"is_encrypted"`
}

type UsageRecord struct {
	BaseModel
	UserID      uint   `json:"user_id"`
	Scene       string `gorm:"size:80" json:"scene"`
	Description string `gorm:"size:255" json:"description"`
	QuotaUsed   int    `gorm:"default:0" json:"quota_used"`
	DeviceID    string `gorm:"size:100" json:"device_id"`
}
