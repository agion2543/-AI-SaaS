package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"go-web-gin-health/internal/adminrecovery"
	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/service"
	"go-web-gin-health/internal/utils"
)

func issueSyntheticAdminToken(t *testing.T, cfg *config.Config, db *gorm.DB, adminID uint) string {
	t.Helper()
	var admin model.AdminUser
	if err := db.Preload("Role").First(&admin, adminID).Error; err != nil {
		t.Fatalf("load synthetic admin for token: %v", err)
	}
	role := "admin"
	if admin.Role.Code == "super_admin" {
		role = "super_admin"
	}
	version := utils.AdminCredentialVersion(cfg.JWTSecret, admin.AuthVersion, admin.PasswordHash, admin.Status, admin.RoleID, admin.Role.Code, admin.Role.Permissions)
	token, err := utils.GenerateAdminToken(cfg.JWTSecret, admin.ID, admin.Username, role, version, 1)
	if err != nil {
		t.Fatalf("issue synthetic admin token: %v", err)
	}
	return token
}

func TestAdminTokensRevokeOnRecoveryRolePermissionAndStatusChanges(t *testing.T) {
	gin.SetMode(gin.TestMode)
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

	superRole := model.AdminRole{Name: "Synthetic Super Admin", Code: "super_admin", Permissions: "read_only,operations,finance,system_admin"}
	operatorRole := model.AdminRole{Name: "Synthetic Operator", Code: "operator", Permissions: "operations"}
	readerRole := model.AdminRole{Name: "Synthetic Reader", Code: "reader", Permissions: "read_only"}
	for _, role := range []*model.AdminRole{&superRole, &operatorRole, &readerRole} {
		if err := db.Create(role).Error; err != nil {
			t.Fatal(err)
		}
	}
	superPasswordHash, err := utils.HashPassword("synthetic-super-original-password-123")
	if err != nil {
		t.Fatal(err)
	}
	operatorPasswordHash, err := utils.HashPassword("synthetic-operator-password-456")
	if err != nil {
		t.Fatal(err)
	}
	superAdmin := model.AdminUser{Username: "synthetic-session-super", DisplayName: "合成超管", PasswordHash: superPasswordHash, Status: "active", RoleID: superRole.ID}
	operator := model.AdminUser{Username: "synthetic-session-operator", DisplayName: "合成运营", PasswordHash: operatorPasswordHash, Status: "active", RoleID: operatorRole.ID}
	for _, admin := range []*model.AdminUser{&superAdmin, &operator} {
		if err := db.Create(admin).Error; err != nil {
			t.Fatal(err)
		}
	}
	deletedAdmin := model.AdminUser{Username: "synthetic-session-deleted", DisplayName: "待删除合成管理员", PasswordHash: operatorPasswordHash, Status: "active", RoleID: operatorRole.ID}
	if err := db.Create(&deletedAdmin).Error; err != nil {
		t.Fatalf("create synthetic admin for deletion check: %v", err)
	}

	cfg := &config.Config{AppEnv: "development", DevPasswordResetEnabled: true, JWTSecret: "synthetic-admin-session-secret"}
	engine := gin.New()
	adminGroup := engine.Group("/api/v1/admin")
	adminGroup.Use(Auth(cfg, "admin", db), AdminAuthorization())
	adminGroup.GET("/access/admins", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	adminGroup.GET("/merchants/:id", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	adminGroup.PUT("/merchants/:id/status", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	request := func(method, path, token string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, req)
		return response
	}

	deletedToken := issueSyntheticAdminToken(t, cfg, db, deletedAdmin.ID)
	if err := db.Delete(&model.AdminUser{}, deletedAdmin.ID).Error; err != nil {
		t.Fatalf("delete synthetic admin: %v", err)
	}
	if response := request(http.MethodGet, "/api/v1/admin/access/admins", deletedToken); response.Code != http.StatusUnauthorized {
		t.Fatalf("deleted-admin token status=%d, want 401; body=%s", response.Code, response.Body.String())
	}

	superToken := issueSyntheticAdminToken(t, cfg, db, superAdmin.ID)
	if response := request(http.MethodGet, "/api/v1/admin/access/admins", superToken); response.Code != http.StatusNoContent {
		t.Fatalf("current synthetic super-admin token status=%d, want 204; body=%s", response.Code, response.Body.String())
	}
	legacyToken, err := utils.GenerateToken(cfg.JWTSecret, superAdmin.ID, superAdmin.Username, "super_admin", nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if response := request(http.MethodGet, "/api/v1/admin/access/admins", legacyToken); response.Code != http.StatusUnauthorized {
		t.Fatalf("legacy token without credential version status=%d, want 401; body=%s", response.Code, response.Body.String())
	}

	if err := adminrecovery.ResetExistingSuperAdminPassword(db, superAdmin.Username, "synthetic-recovered-password-789"); err != nil {
		t.Fatalf("reset synthetic super-admin password through recovery path: %v", err)
	}
	if response := request(http.MethodGet, "/api/v1/admin/access/admins", superToken); response.Code != http.StatusUnauthorized {
		t.Fatalf("pre-recovery token status=%d, want 401; body=%s", response.Code, response.Body.String())
	}
	currentSuperToken := issueSyntheticAdminToken(t, cfg, db, superAdmin.ID)
	if response := request(http.MethodGet, "/api/v1/admin/access/admins", currentSuperToken); response.Code != http.StatusNoContent {
		t.Fatalf("post-recovery token status=%d, want 204; body=%s", response.Code, response.Body.String())
	}
	for _, appEnv := range []string{"production", "prod", "staging", ""} {
		blockedCfg := *cfg
		blockedCfg.AppEnv = appEnv
		err := service.NewAuthService(&blockedCfg, db).DevResetPassword(dto.DevResetPasswordRequest{
			AccountType: "admin", Account: superAdmin.Username, NewPassword: "synthetic-blocked-reset-password",
		})
		if err == nil {
			t.Fatalf("dev password reset unexpectedly succeeded for APP_ENV=%q", appEnv)
		}
		if response := request(http.MethodGet, "/api/v1/admin/access/admins", currentSuperToken); response.Code != http.StatusNoContent {
			t.Fatalf("blocked APP_ENV=%q reset changed current token status=%d; body=%s", appEnv, response.Code, response.Body.String())
		}
	}
	implicitDevCfg := *cfg
	implicitDevCfg.DevPasswordResetEnabled = false
	if err := service.NewAuthService(&implicitDevCfg, db).DevResetPassword(dto.DevResetPasswordRequest{
		AccountType: "admin", Account: superAdmin.Username, NewPassword: "synthetic-implicit-reset-password",
	}); err == nil {
		t.Fatal("dev password reset unexpectedly succeeded without explicit development configuration")
	}
	if err := service.NewAuthService(cfg, db).DevResetPassword(dto.DevResetPasswordRequest{
		AccountType: "admin", Account: superAdmin.Username, NewPassword: "synthetic-dev-reset-password-000",
	}); err != nil {
		t.Fatalf("reset synthetic admin password in development: %v", err)
	}
	if response := request(http.MethodGet, "/api/v1/admin/access/admins", currentSuperToken); response.Code != http.StatusUnauthorized {
		t.Fatalf("pre-dev-reset token status=%d, want 401; body=%s", response.Code, response.Body.String())
	}
	devResetToken := issueSyntheticAdminToken(t, cfg, db, superAdmin.ID)
	if response := request(http.MethodGet, "/api/v1/admin/access/admins", devResetToken); response.Code != http.StatusNoContent {
		t.Fatalf("post-dev-reset token status=%d, want 204; body=%s", response.Code, response.Body.String())
	}

	access := service.NewAdminAccessService(db)
	accessActor := service.AuditActor{ID: superAdmin.ID, Type: "super_admin"}
	operatorToken := issueSyntheticAdminToken(t, cfg, db, operator.ID)
	const merchantStatusPath = "/api/v1/admin/merchants/17/status"
	if response := request(http.MethodPut, merchantStatusPath, operatorToken); response.Code != http.StatusNoContent {
		t.Fatalf("current operations token status=%d, want 204; body=%s", response.Code, response.Body.String())
	}
	if _, err := access.UpdateAdmin(operator.ID, service.UpdateAdminAccountInput{DisplayName: "合成只读", Status: "active", RoleID: readerRole.ID}, accessActor); err != nil {
		t.Fatalf("change synthetic admin role: %v", err)
	}
	if response := request(http.MethodPut, merchantStatusPath, operatorToken); response.Code != http.StatusUnauthorized {
		t.Fatalf("pre-demotion token status=%d, want 401; body=%s", response.Code, response.Body.String())
	}
	readerToken := issueSyntheticAdminToken(t, cfg, db, operator.ID)
	if response := request(http.MethodPut, merchantStatusPath, readerToken); response.Code != http.StatusForbidden {
		t.Fatalf("current read-only token on operations route status=%d, want 403; body=%s", response.Code, response.Body.String())
	}
	if response := request(http.MethodGet, "/api/v1/admin/merchants/17", readerToken); response.Code != http.StatusNoContent {
		t.Fatalf("current read-only token on permitted route status=%d, want 204; body=%s", response.Code, response.Body.String())
	}

	if _, err := access.UpdateRole(readerRole.ID, service.SaveAdminRoleInput{Name: readerRole.Name, Permissions: []string{"finance"}}, accessActor); err != nil {
		t.Fatalf("change synthetic role permissions: %v", err)
	}
	if response := request(http.MethodGet, "/api/v1/admin/merchants/17", readerToken); response.Code != http.StatusUnauthorized {
		t.Fatalf("pre-permission-change token status=%d, want 401; body=%s", response.Code, response.Body.String())
	}
	financeToken := issueSyntheticAdminToken(t, cfg, db, operator.ID)
	if response := request(http.MethodGet, "/api/v1/admin/merchants/17", financeToken); response.Code != http.StatusForbidden {
		t.Fatalf("current finance-only token on operations route status=%d, want 403; body=%s", response.Code, response.Body.String())
	}

	if _, err := access.UpdateAdmin(operator.ID, service.UpdateAdminAccountInput{DisplayName: "已停用合成管理员", Status: "disabled", RoleID: readerRole.ID}, accessActor); err != nil {
		t.Fatalf("disable synthetic administrator: %v", err)
	}
	if response := request(http.MethodGet, "/api/v1/admin/merchants/17", financeToken); response.Code != http.StatusForbidden {
		t.Fatalf("disabled-account token status=%d, want 403; body=%s", response.Code, response.Body.String())
	}
	disabledToken := issueSyntheticAdminToken(t, cfg, db, operator.ID)
	if response := request(http.MethodGet, "/api/v1/admin/merchants/17", disabledToken); response.Code != http.StatusForbidden {
		t.Fatalf("token issued for disabled account status=%d, want 403; body=%s", response.Code, response.Body.String())
	}
	if _, err := access.UpdateAdmin(operator.ID, service.UpdateAdminAccountInput{DisplayName: "重新启用合成管理员", Status: "active", RoleID: readerRole.ID}, accessActor); err != nil {
		t.Fatalf("re-enable synthetic administrator: %v", err)
	}
	if response := request(http.MethodGet, "/api/v1/admin/merchants/17", disabledToken); response.Code != http.StatusUnauthorized {
		t.Fatalf("pre-reactivation token status=%d, want 401; body=%s", response.Code, response.Body.String())
	}
	if response := request(http.MethodGet, "/api/v1/admin/merchants/17", financeToken); response.Code != http.StatusUnauthorized {
		t.Fatalf("pre-disable token status=%d, want 401; body=%s", response.Code, response.Body.String())
	}
	activeFinanceToken := issueSyntheticAdminToken(t, cfg, db, operator.ID)
	if response := request(http.MethodGet, "/api/v1/admin/merchants/17", activeFinanceToken); response.Code != http.StatusForbidden {
		t.Fatalf("fresh finance-only token status=%d, want 403; body=%s", response.Code, response.Body.String())
	}
}
