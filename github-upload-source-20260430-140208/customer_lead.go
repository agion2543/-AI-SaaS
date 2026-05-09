package model

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
