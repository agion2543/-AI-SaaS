package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPlatformQRCodeAndConfigRequireSystemAdminOnLegacyAndV1(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open synthetic QR permission database: %v", err)
	}
	if err := db.AutoMigrate(&model.AdminRole{}, &model.AdminUser{}); err != nil {
		t.Fatalf("migrate synthetic QR permission schema: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get synthetic QR permission database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	finance := createPlatformQRPermissionAdmin(t, db, "synthetic-qr-finance", "finance")
	system := createPlatformQRPermissionAdmin(t, db, "synthetic-qr-system", "system_admin")
	cfg := &config.Config{JWTSecret: "synthetic-platform-qr-permission-secret", FrontendURL: "http://localhost:5173", RateLimitPerMinute: 1000}
	engine := gin.New()
	Register(engine, db, cfg)
	financeToken := syntheticAdminToken(t, db, cfg, finance.ID)
	systemToken := syntheticAdminToken(t, db, cfg, system.ID)

	tests := []struct {
		name        string
		method      string
		path        string
		contentType string
		body        string
	}{
		{name: "legacy upload", method: http.MethodPost, path: "/api/admin/payment-qrcode/upload"},
		{name: "v1 upload", method: http.MethodPost, path: "/api/v1/admin/payment-qrcode/upload"},
		{name: "legacy config save", method: http.MethodPost, path: "/api/admin/configs", contentType: "application/json", body: "{"},
		{name: "v1 config save", method: http.MethodPost, path: "/api/v1/admin/configs", contentType: "application/json", body: "{"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			financeResponse := callPlatformQRPermissionRoute(engine, test.method, test.path, test.body, test.contentType, financeToken)
			if financeResponse.Code != http.StatusForbidden {
				t.Fatalf("finance-only account status=%d, want 403; body=%s", financeResponse.Code, financeResponse.Body.String())
			}
			systemResponse := callPlatformQRPermissionRoute(engine, test.method, test.path, test.body, test.contentType, systemToken)
			if systemResponse.Code != http.StatusBadRequest {
				t.Fatalf("system_admin did not reach the handler (status=%d, want handler's 400); body=%s", systemResponse.Code, systemResponse.Body.String())
			}
		})
	}
}

func createPlatformQRPermissionAdmin(t *testing.T, db *gorm.DB, username, permissions string) model.AdminUser {
	t.Helper()
	role := model.AdminRole{Name: "Synthetic " + username, Code: username, Permissions: permissions}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("create synthetic role: %v", err)
	}
	admin := model.AdminUser{Username: username, DisplayName: "Synthetic QR Permission Admin", Status: "active", RoleID: role.ID}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create synthetic admin: %v", err)
	}
	return admin
}

func callPlatformQRPermissionRoute(engine *gin.Engine, method, path, body, contentType, token string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	return response
}
