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

	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/utils"
)

func TestAdminAuthorizationUsesCurrentAccountAndExplicitPermissions(t *testing.T) {
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
	if err := db.AutoMigrate(&model.AdminRole{}, &model.AdminUser{}); err != nil {
		t.Fatalf("migrate synthetic admin tables: %v", err)
	}
	role := model.AdminRole{Name: "Synthetic Operator", Code: "synthetic_operator", Permissions: "operations"}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("create synthetic role: %v", err)
	}
	admin := model.AdminUser{Username: "synthetic-admin", DisplayName: "合成管理员", Status: "active", RoleID: role.ID}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create synthetic admin: %v", err)
	}
	cfg := &config.Config{JWTSecret: "synthetic-admin-permission-secret"}
	token := issueSyntheticAdminToken(t, cfg, db, admin.ID)

	engine := gin.New()
	adminGroup := engine.Group("/api/v1/admin")
	adminGroup.Use(Auth(cfg, "admin", db), AdminAuthorization())
	adminGroup.GET("/merchants/:id", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	adminGroup.GET("/access/admins", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	adminGroup.GET("/unmapped", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	request := func(path string) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		return response
	}
	if response := request("/api/v1/admin/merchants/17"); response.Code != http.StatusNoContent {
		t.Fatalf("explicit operations read route status=%d, want %d; body=%s", response.Code, http.StatusNoContent, response.Body.String())
	}
	if response := request("/api/v1/admin/access/admins"); response.Code != http.StatusForbidden {
		t.Fatalf("ordinary account management status=%d, want %d", response.Code, http.StatusForbidden)
	}
	if response := request("/api/v1/admin/unmapped"); response.Code != http.StatusForbidden {
		t.Fatalf("unmapped route status=%d, want %d", response.Code, http.StatusForbidden)
	}

	if err := db.Model(&role).Update("permissions", "finance").Error; err != nil {
		t.Fatalf("remove operations permission: %v", err)
	}
	if response := request("/api/v1/admin/merchants/17"); response.Code != http.StatusUnauthorized {
		t.Fatalf("existing token after permission change status=%d, want %d", response.Code, http.StatusUnauthorized)
	}
	token = issueSyntheticAdminToken(t, cfg, db, admin.ID)
	if response := request("/api/v1/admin/merchants/17"); response.Code != http.StatusForbidden {
		t.Fatalf("fresh token without operations permission status=%d, want %d", response.Code, http.StatusForbidden)
	}
	if err := db.Model(&model.AdminUser{}).Where("id = ?", admin.ID).Update("status", "disabled").Error; err != nil {
		t.Fatalf("disable synthetic admin: %v", err)
	}
	if response := request("/api/v1/admin/merchants/17"); response.Code != http.StatusForbidden {
		t.Fatalf("existing token after account disable status=%d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestAdminExportRequiresSeparatePermission(t *testing.T) {
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
	if err := db.AutoMigrate(&model.AdminRole{}, &model.AdminUser{}); err != nil {
		t.Fatalf("migrate synthetic admin tables: %v", err)
	}
	role := model.AdminRole{Name: "Synthetic Export Reader", Code: "synthetic_export_reader", Permissions: "operations"}
	if err := db.Create(&role).Error; err != nil {
		t.Fatal(err)
	}
	admin := model.AdminUser{Username: "synthetic-export-admin", DisplayName: "合成导出管理员", Status: "active", RoleID: role.ID}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{JWTSecret: "synthetic-admin-export-secret"}
	engine := gin.New()
	engine.GET("/api/v1/admin/customers/export", Auth(cfg, "admin", db), AdminAuthorization(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	request := func(token string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/customers/export", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, req)
		return response
	}
	token := issueSyntheticAdminToken(t, cfg, db, admin.ID)
	if response := request(token); response.Code != http.StatusForbidden {
		t.Fatalf("operations-only admin export status=%d, want 403", response.Code)
	}
	if err := db.Model(&role).Update("permissions", "operations,customer_order_export").Error; err != nil {
		t.Fatal(err)
	}
	if response := request(token); response.Code != http.StatusUnauthorized {
		t.Fatalf("token after export grant change status=%d, want 401", response.Code)
	}
	token = issueSyntheticAdminToken(t, cfg, db, admin.ID)
	if response := request(token); response.Code != http.StatusNoContent {
		t.Fatalf("operations plus export admin status=%d, want 204; body=%s", response.Code, response.Body.String())
	}
}

func TestAdminAuthRejectsTokenWhenCurrentRoleDoesNotMatchTokenRole(t *testing.T) {
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
	if err := db.AutoMigrate(&model.AdminRole{}, &model.AdminUser{}); err != nil {
		t.Fatal(err)
	}
	role := model.AdminRole{Name: "Synthetic Super Admin", Code: "super_admin"}
	if err := db.Create(&role).Error; err != nil {
		t.Fatal(err)
	}
	admin := model.AdminUser{Username: "synthetic-super", DisplayName: "合成超管", Status: "active", RoleID: role.ID}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{JWTSecret: "synthetic-role-change-secret"}
	version := utils.AdminCredentialVersion(cfg.JWTSecret, admin.AuthVersion, admin.PasswordHash, admin.Status, admin.RoleID, role.Code, role.Permissions)
	staleRoleToken, err := utils.GenerateAdminToken(cfg.JWTSecret, admin.ID, admin.Username, "admin", version, 1)
	if err != nil {
		t.Fatal(err)
	}
	engine := gin.New()
	engine.GET("/api/v1/admin/access/admins", Auth(cfg, "admin", db), AdminAuthorization(), func(c *gin.Context) {
		if c.GetString("role") != "super_admin" {
			t.Errorf("context role = %q, want current super_admin", c.GetString("role"))
		}
		c.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/access/admins", nil)
	request.Header.Set("Authorization", "Bearer "+staleRoleToken)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("token with stale admin role status=%d, want %d; body=%s", response.Code, http.StatusUnauthorized, response.Body.String())
	}
	currentRoleToken := issueSyntheticAdminToken(t, cfg, db, admin.ID)
	request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/access/admins", nil)
	request.Header.Set("Authorization", "Bearer "+currentRoleToken)
	response = httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("current super-admin token status=%d, want %d; body=%s", response.Code, http.StatusNoContent, response.Body.String())
	}
}

func TestAdminExportPermissionMatrix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, prefix := range []string{"/api/admin", "/api/v1/admin"} {
		for _, dataset := range []string{"customers", "orders"} {
			for _, test := range []struct {
				name, role, permissions string
				allowed                 bool
			}{
				{"default admin", "admin", "", false},
				{"modules only", "admin", "read_only,operations,finance", false},
				{"contact only", "admin", "customer_contact_view", false},
				{"export only", "admin", "customer_order_export", false},
				{"separate grants", "admin", "operations,finance,customer_order_export", true},
				{"super admin", "super_admin", "", true},
			} {
				t.Run(prefix+"/"+dataset+"/"+test.name, func(t *testing.T) {
					engine := gin.New()
					path := prefix + "/" + dataset + "/export"
					engine.GET(path, func(c *gin.Context) {
						c.Set("role", test.role)
						c.Set("admin_permissions", test.permissions)
						c.Next()
					}, AdminAuthorization(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
					response := httptest.NewRecorder()
					engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
					want := http.StatusForbidden
					if test.allowed {
						want = http.StatusNoContent
					}
					if response.Code != want {
						t.Fatalf("status=%d, want %d", response.Code, want)
					}
				})
			}
		}
	}
}
