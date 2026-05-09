package model

import "time"

type CardCode struct {
	BaseModel
	BatchNo    string            `gorm:"size:50;index" json:"batch_no"`
	Code       string            `gorm:"size:64;uniqueIndex" json:"code"`
	PackageID  uint              `json:"package_id"`
	Package    MembershipPackage `json:"package"`
	Quota      int               `json:"quota"`
	Status     string            `gorm:"size:20;default:unused" json:"status"`
	ExpiredAt  *time.Time        `json:"expired_at"`
	RedeemedBy *uint             `json:"redeemed_by"`
	RedeemedAt *time.Time        `json:"redeemed_at"`
}
