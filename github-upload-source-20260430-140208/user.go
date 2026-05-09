package model

import "time"

type User struct {
	BaseModel
	UUID             string     `gorm:"type:char(36);uniqueIndex" json:"uuid"`
	Email            string     `gorm:"size:120;uniqueIndex" json:"email"`
	Phone            string     `gorm:"size:30;uniqueIndex" json:"phone"`
	MerchantID       *uint      `gorm:"index" json:"merchant_id"`
	Merchant         *Merchant  `json:"merchant,omitempty"`
	Role             string     `gorm:"size:20;default:customer;index" json:"role"`
	PasswordHash     string     `gorm:"size:255" json:"-"`
	DisplayName      string     `gorm:"size:80" json:"display_name"`
	Status           string     `gorm:"size:20;default:active" json:"status"`
	MemberLevel      string     `gorm:"size:30;default:free" json:"member_level"`
	CurrentPackageID *uint      `json:"current_package_id"`
	ExpiredAt        *time.Time `json:"expired_at"`
	RemainingQuota   int        `gorm:"default:0" json:"remaining_quota"`
	BoundDevices     string     `gorm:"type:text" json:"bound_devices"`
	AutoRenew        bool       `gorm:"default:false" json:"auto_renew"`
	ResetToken       string     `gorm:"size:120" json:"-"`
}

type VerificationCode struct {
	BaseModel
	Phone      string     `gorm:"size:30;index:idx_phone_scene,priority:1" json:"phone"`
	Scene      string     `gorm:"size:40;index:idx_phone_scene,priority:2" json:"scene"`
	Code       string     `gorm:"size:12" json:"-"`
	UserID     *uint      `json:"user_id,omitempty"`
	ExpiresAt  time.Time  `json:"expires_at"`
	ConsumedAt *time.Time `json:"consumed_at,omitempty"`
}

type EmailVerificationCode struct {
	BaseModel
	Email      string     `gorm:"size:120;index:idx_email_scene,priority:1" json:"email"`
	Scene      string     `gorm:"size:40;index:idx_email_scene,priority:2" json:"scene"`
	Code       string     `gorm:"size:12" json:"-"`
	UserID     *uint      `json:"user_id,omitempty"`
	ExpiresAt  time.Time  `json:"expires_at"`
	ConsumedAt *time.Time `json:"consumed_at,omitempty"`
}
