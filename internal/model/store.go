package model

type Store struct {
	BaseModel
	MerchantID    uint     `gorm:"not null;uniqueIndex:idx_merchant_store_name;index" json:"merchant_id"`
	Merchant      Merchant `json:"merchant,omitempty"`
	Name          string   `gorm:"size:120;not null;uniqueIndex:idx_merchant_store_name" json:"name"`
	Address       string   `gorm:"size:255" json:"address"`
	ContactPhone  string   `gorm:"size:30" json:"contact_phone"`
	Status        string   `gorm:"size:20;default:active;index" json:"status"`
	IsOpen        bool     `gorm:"default:true;index" json:"is_open"`
	BusinessHours string   `gorm:"size:120" json:"business_hours"`
	PauseReason   string   `gorm:"size:255" json:"pause_reason"`
	OrderMode     string   `gorm:"size:30;default:pay_first;index" json:"order_mode"`
	AutoAccept    bool     `gorm:"default:false;index" json:"auto_accept"`
}

type StoreProduct struct {
	BaseModel
	StoreID     uint   `gorm:"not null;index" json:"store_id"`
	Store       Store  `json:"store,omitempty"`
	Name        string `gorm:"size:120;not null" json:"name"`
	Price       int64  `gorm:"not null;default:0" json:"price"`
	Description string `gorm:"size:500" json:"description"`
	ImageURL    string `gorm:"size:500" json:"image_url"`
	Category    string `gorm:"size:80;default:默认分类;index" json:"category"`
	Status      string `gorm:"size:20;default:active;index" json:"status"`
	Stock       *int   `gorm:"default:null" json:"stock"`
	Sort        int    `gorm:"default:100;index" json:"sort"`
}
