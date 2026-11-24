package model

import "time"

// StoreTableConfig keeps the merchant-selected number of active table QR codes
// separate from the immutable store schema and historical orders.
type StoreTableConfig struct {
	BaseModel
	StoreID    uint `gorm:"not null;uniqueIndex" json:"store_id"`
	TableCount int  `gorm:"not null;default:0" json:"table_count"`
}

// StoreTable represents a numbered table, a merchant-editable display label,
// and its QR-code bearer secret. The secret is returned only to the authenticated
// merchant managing this store; TableNo remains the stable QR/session identity.
type StoreTable struct {
	BaseModel
	StoreID    uint   `gorm:"not null;uniqueIndex:idx_store_tables_store_no;index" json:"store_id"`
	TableNo    int    `gorm:"not null;uniqueIndex:idx_store_tables_store_no" json:"table_no"`
	TableLabel string `gorm:"size:64;not null;default:''" json:"table_label"`
	TableCode  string `gorm:"size:64;not null;uniqueIndex" json:"-"`
}

// TableSession is the shared, server-authoritative cart for one active table.
// It is locked at checkout and remains locked until the linked order is closed
// or completed, preventing another phone from creating a duplicate payment.
type TableSession struct {
	BaseModel
	StoreID         uint       `gorm:"not null;uniqueIndex:idx_table_sessions_store_no" json:"store_id"`
	TableNo         int        `gorm:"not null;uniqueIndex:idx_table_sessions_store_no" json:"table_no"`
	Status          string     `gorm:"size:24;not null;default:open;index" json:"status"`
	CartVersion     uint64     `gorm:"not null;default:0" json:"cart_version"`
	CheckoutOrderID *uint      `gorm:"index" json:"-"`
	LockedAt        *time.Time `json:"locked_at,omitempty"`
}

// TableCartItem stores only product identity and quantity. Price and product
// data are always reloaded and validated from the merchant catalog at checkout.
type TableCartItem struct {
	BaseModel
	SessionID uint `gorm:"not null;uniqueIndex:idx_table_cart_session_product;index" json:"session_id"`
	ProductID uint `gorm:"not null;uniqueIndex:idx_table_cart_session_product" json:"product_id"`
	Quantity  int  `gorm:"not null" json:"quantity"`
}

// TableOrderDetail preserves the stable table number and its checkout-time label
// without mutating the immutable orders-table schema baseline.
type TableOrderDetail struct {
	BaseModel
	OrderID            uint    `gorm:"not null;uniqueIndex" json:"order_id"`
	SessionID          uint    `gorm:"not null;index" json:"session_id"`
	TableNo            int     `gorm:"not null" json:"table_no"`
	TableLabelSnapshot *string `gorm:"size:64" json:"table_label_snapshot,omitempty"`
}
