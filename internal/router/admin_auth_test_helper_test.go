package router

import (
	"errors"
	"fmt"
	"testing"

	"gorm.io/gorm"

	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/utils"
)

func seedSyntheticSuperAdmin(t *testing.T, db *gorm.DB, id uint, username string) model.AdminUser {
	t.Helper()
	if err := db.AutoMigrate(&model.AdminRole{}, &model.AdminUser{}); err != nil {
		t.Fatalf("migrate synthetic admin auth tables: %v", err)
	}
	role := model.AdminRole{Name: "Synthetic Super Admin", Code: "super_admin", Permissions: "read_only,operations,finance,system_admin"}
	if err := db.Where(model.AdminRole{Code: role.Code}).FirstOrCreate(&role).Error; err != nil {
		t.Fatalf("create synthetic superadmin role: %v", err)
	}
	if username == "" {
		username = fmt.Sprintf("synthetic-superadmin-%d", id)
	}
	var admin model.AdminUser
	var err error
	if id > 0 {
		err = db.First(&admin, id).Error
	} else {
		err = db.Where("username = ?", username).First(&admin).Error
	}
	if err == nil {
		if updateErr := db.Model(&admin).Updates(map[string]any{"role_id": role.ID, "status": "active"}).Error; updateErr != nil {
			t.Fatalf("assign synthetic superadmin role: %v", updateErr)
		}
		admin.RoleID, admin.Status = role.ID, "active"
		admin.Role = role
		return admin
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("load synthetic admin: %v", err)
	}
	admin = model.AdminUser{
		BaseModel: model.BaseModel{ID: id},
		Username:  username, DisplayName: "合成平台超管", Status: "active", RoleID: role.ID,
	}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create synthetic superadmin account: %v", err)
	}
	admin.Role = role
	return admin
}

func syntheticAdminToken(t *testing.T, db *gorm.DB, cfg *config.Config, adminID uint) string {
	t.Helper()
	var admin model.AdminUser
	if err := db.Preload("Role").First(&admin, adminID).Error; err != nil {
		t.Fatalf("load synthetic administrator for token: %v", err)
	}
	role := "admin"
	if admin.Role.Code == "super_admin" {
		role = "super_admin"
	}
	version := utils.AdminCredentialVersion(cfg.JWTSecret, admin.AuthVersion, admin.PasswordHash, admin.Status, admin.RoleID, admin.Role.Code, admin.Role.Permissions)
	token, err := utils.GenerateAdminToken(cfg.JWTSecret, admin.ID, admin.Username, role, version, 1)
	if err != nil {
		t.Fatalf("create synthetic administrator token: %v", err)
	}
	return token
}
