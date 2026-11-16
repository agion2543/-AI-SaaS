package service

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"go-web-gin-health/internal/model"
)

func openAdminAccessTestDB(t *testing.T) *gorm.DB {
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
	if err := db.AutoMigrate(&model.AdminRole{}, &model.AdminUser{}, &model.AuditLog{}); err != nil {
		t.Fatalf("migrate synthetic admin tables: %v", err)
	}
	return db
}

func TestAdminAccessServiceManagesOnlyOrdinaryAdminsWithExplicitPermissions(t *testing.T) {
	db := openAdminAccessTestDB(t)
	service := NewAdminAccessService(db)
	superRole := model.AdminRole{Name: "Super Admin", Code: "super_admin", Permissions: "read_only,operations,finance,system_admin"}
	if err := db.Create(&superRole).Error; err != nil {
		t.Fatal(err)
	}
	super := model.AdminUser{Username: "synthetic-owner", DisplayName: "合成所有者", Status: "active", RoleID: superRole.ID}
	if err := db.Create(&super).Error; err != nil {
		t.Fatal(err)
	}
	actor := AuditActor{ID: super.ID, Type: "super_admin"}

	roleView, err := service.CreateRole(SaveAdminRoleInput{Name: "只读专员", Code: "read_operator", Permissions: []string{"read_only", "read_only"}}, actor)
	if err != nil {
		t.Fatalf("create ordinary role: %v", err)
	}
	if len(roleView.Permissions) != 1 || roleView.Permissions[0] != "read_only" {
		t.Fatalf("permissions were not normalized: %#v", roleView.Permissions)
	}
	accountView, err := service.CreateAdmin(CreateAdminAccountInput{
		Username: "synthetic-reader", DisplayName: "合成只读", Password: "synthetic-password-123", RoleID: roleView.ID,
	}, actor)
	if err != nil {
		t.Fatalf("create ordinary admin: %v", err)
	}
	if len(accountView.Permissions) != 1 || accountView.Permissions[0] != "read_only" || accountView.Status != "active" {
		t.Fatalf("created account has unexpected permissions/status: %#v", accountView)
	}
	var initialAdmin model.AdminUser
	if err := db.First(&initialAdmin, accountView.ID).Error; err != nil {
		t.Fatal(err)
	}
	if initialAdmin.AuthVersion != 1 {
		t.Fatalf("new admin auth_version=%d, want 1", initialAdmin.AuthVersion)
	}
	roles, err := service.ListRoles()
	if err != nil || len(roles) != 1 || roles[0].Code == "super_admin" {
		t.Fatalf("ordinary role list=%#v err=%v", roles, err)
	}
	admins, err := service.ListAdmins()
	if err != nil || len(admins) != 1 || admins[0].Username != accountView.Username {
		t.Fatalf("ordinary account list=%#v err=%v", admins, err)
	}

	updatedRole, err := service.UpdateRole(roleView.ID, SaveAdminRoleInput{Name: "只读与运营", Permissions: []string{"read_only", "operations"}}, actor)
	if err != nil {
		t.Fatalf("update ordinary role permissions: %v", err)
	}
	if len(updatedRole.Permissions) != 2 {
		t.Fatalf("updated permissions=%#v", updatedRole.Permissions)
	}
	var afterRoleUpdate model.AdminUser
	if err := db.First(&afterRoleUpdate, accountView.ID).Error; err != nil {
		t.Fatal(err)
	}
	if afterRoleUpdate.AuthVersion != initialAdmin.AuthVersion+1 {
		t.Fatalf("role permission change auth_version=%d, want %d", afterRoleUpdate.AuthVersion, initialAdmin.AuthVersion+1)
	}
	updatedAdmin, err := service.UpdateAdmin(accountView.ID, UpdateAdminAccountInput{DisplayName: "停用的合成运营", Status: "disabled", RoleID: roleView.ID}, actor)
	if err != nil {
		t.Fatalf("update ordinary account: %v", err)
	}
	if updatedAdmin.Status != "disabled" || len(updatedAdmin.Permissions) != 2 {
		t.Fatalf("updated account=%#v", updatedAdmin)
	}
	var afterAccountUpdate model.AdminUser
	if err := db.First(&afterAccountUpdate, accountView.ID).Error; err != nil {
		t.Fatal(err)
	}
	if afterAccountUpdate.AuthVersion != afterRoleUpdate.AuthVersion+1 {
		t.Fatalf("account status/role change auth_version=%d, want %d", afterAccountUpdate.AuthVersion, afterRoleUpdate.AuthVersion+1)
	}

	if _, err := service.CreateRole(SaveAdminRoleInput{Name: "伪超管", Code: "super_admin", Permissions: []string{"system_admin"}}, actor); err == nil {
		t.Fatal("creation of reserved super_admin role unexpectedly succeeded")
	}
	if _, err := service.CreateAdmin(CreateAdminAccountInput{Username: "synthetic-promote", DisplayName: "不应晋升", Password: "synthetic-password-456", RoleID: superRole.ID}, actor); err == nil {
		t.Fatal("creation of superadmin account unexpectedly succeeded")
	}
	if _, err := service.UpdateAdmin(super.ID, UpdateAdminAccountInput{DisplayName: "不应变更", Status: "disabled", RoleID: roleView.ID}, actor); err == nil {
		t.Fatal("update of superadmin account unexpectedly succeeded")
	}
	if _, err := service.UpdateRole(superRole.ID, SaveAdminRoleInput{Name: "被改写超管", Permissions: []string{}}, actor); err == nil {
		t.Fatal("update of superadmin role unexpectedly succeeded")
	}

	var savedSuper model.AdminUser
	if err := db.Preload("Role").First(&savedSuper, super.ID).Error; err != nil {
		t.Fatal(err)
	}
	if savedSuper.Status != "active" || savedSuper.RoleID != superRole.ID || savedSuper.Role.Code != "super_admin" {
		t.Fatalf("superadmin protected fields changed: %#v", savedSuper)
	}
	if savedSuper.PasswordHash != "" {
		t.Fatal("superadmin password hash changed unexpectedly")
	}
}
