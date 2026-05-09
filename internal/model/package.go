package model

type MembershipPackage struct {
	BaseModel
	Name          string `gorm:"size:80" json:"name"`
	Code          string `gorm:"size:40;uniqueIndex" json:"code"`
	DurationDays  int    `json:"duration_days"`
	Price         int64  `json:"price"`
	OriginalPrice int64  `json:"original_price"`
	Quota         int    `json:"quota"`
	Sort          int    `gorm:"default:0" json:"sort"`
	Status        string `gorm:"size:20;default:published" json:"status"`
	IsLifetime    bool   `gorm:"default:false" json:"is_lifetime"`
	Description   string `gorm:"type:text" json:"description"`
}
