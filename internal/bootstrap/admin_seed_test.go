package bootstrap

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/utils"
)

func openAdminSeedTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open synthetic SQLite database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get synthetic SQL database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&model.AdminRole{}, &model.AdminUser{}); err != nil {
		t.Fatalf("migrate synthetic admin tables: %v", err)
	}
	return db
}

func TestSeedAdminUserRejectsFreshInstallWithoutInitialPassword(t *testing.T) {
	db := openAdminSeedTestDB(t)
	cfg := &config.Config{InitialAdminUser: "admin"}

	err := seedAdminUser(db, cfg, 1)
	if err == nil || !strings.Contains(err.Error(), "INITIAL_ADMIN_PASSWORD") {
		t.Fatalf("expected explicit INITIAL_ADMIN_PASSWORD error, got %v", err)
	}
	var count int64
	if err := db.Model(&model.AdminUser{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("fresh install unexpectedly created %d admin accounts", count)
	}
}

func TestSeedAdminUserCreatesFreshAdminWithExplicitPassword(t *testing.T) {
	db := openAdminSeedTestDB(t)
	role := model.AdminRole{Name: "Super Admin", Code: "super_admin"}
	if err := db.Create(&role).Error; err != nil {
		t.Fatal(err)
	}
	password := "synthetic-fresh-admin-password-246"
	cfg := &config.Config{InitialAdminUser: "admin", InitialAdminPass: password}

	if err := seedAdminUser(db, cfg, role.ID); err != nil {
		t.Fatalf("create admin with explicit credentials: %v", err)
	}
	var admin model.AdminUser
	if err := db.First(&admin, "username = ?", "admin").Error; err != nil {
		t.Fatal(err)
	}
	if admin.PasswordHash == "" || admin.RoleID != role.ID || admin.Status != "active" || !utils.CheckPassword(admin.PasswordHash, password) {
		t.Fatalf("created admin has unexpected credentials or status: %#v", admin)
	}
}

func TestSeedAdminUserPreservesExistingCredentialsRoleAndStatus(t *testing.T) {
	db := openAdminSeedTestDB(t)
	currentRole := model.AdminRole{Name: "Synthetic Existing Role", Code: "synthetic_existing"}
	configuredRole := model.AdminRole{Name: "Synthetic New Role", Code: "synthetic_new"}
	if err := db.Create(&currentRole).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&configuredRole).Error; err != nil {
		t.Fatal(err)
	}
	currentHash, err := utils.HashPassword("synthetic-existing-password-456")
	if err != nil {
		t.Fatal(err)
	}
	existing := model.AdminUser{
		Username:     "admin",
		PasswordHash: currentHash,
		DisplayName:  "Synthetic Existing Admin",
		Status:       "disabled",
		RoleID:       currentRole.ID,
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		InitialAdminUser: "admin",
		InitialAdminPass: "synthetic-new-password-789",
	}

	if err := seedAdminUser(db, cfg, configuredRole.ID); err != nil {
		t.Fatalf("seed existing admin: %v", err)
	}
	var saved model.AdminUser
	if err := db.First(&saved, existing.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.PasswordHash != currentHash || saved.RoleID != currentRole.ID || saved.Status != "disabled" || saved.Username != existing.Username || saved.DisplayName != existing.DisplayName {
		t.Fatalf("existing admin was changed: before=%#v after=%#v", existing, saved)
	}
}

func TestSeedAdminUserStartsWhenExistingAdminHasDifferentUsername(t *testing.T) {
	db := openAdminSeedTestDB(t)
	role := model.AdminRole{Name: "Synthetic Existing Role", Code: "synthetic_existing"}
	if err := db.Create(&role).Error; err != nil {
		t.Fatal(err)
	}
	hash, err := utils.HashPassword("synthetic-existing-password-012")
	if err != nil {
		t.Fatal(err)
	}
	existing := model.AdminUser{Username: "existing-owner", PasswordHash: hash, DisplayName: "Synthetic Owner", Status: "active", RoleID: role.ID}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatal(err)
	}

	if err := seedAdminUser(db, &config.Config{}, role.ID); err != nil {
		t.Fatalf("existing installation should start without creating a new admin: %v", err)
	}
	var count int64
	if err := db.Model(&model.AdminUser{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("startup changed the existing admin count: got %d, want 1", count)
	}
}
