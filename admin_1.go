package model

type AdminRole struct {
	BaseModel
	Name        string `gorm:"size:60;uniqueIndex" json:"name"`
	Code        string `gorm:"size:60;uniqueIndex" json:"code"`
	Permissions string `gorm:"type:text" json:"permissions"`
}

type AdminUser struct {
	BaseModel
	Username     string `gorm:"size:60;uniqueIndex" json:"username"`
	PasswordHash string `gorm:"size:255" json:"-"`
	DisplayName  string `gorm:"size:80" json:"display_name"`
	Status       string `gorm:"size:20;default:active" json:"status"`
	RoleID       uint   `json:"role_id"`
	Role         AdminRole
}
