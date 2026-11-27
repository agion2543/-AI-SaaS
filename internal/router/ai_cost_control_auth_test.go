package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/model"
)

func TestAICostControlIsSuperAdminOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open synthetic SQLite database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get synthetic SQLite database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(
		&model.AdminRole{}, &model.AdminUser{}, &model.MerchantPlan{}, &model.Merchant{},
		&model.MerchantAIUsageLog{}, &model.AIModelPrice{}, &model.AIVendorBill{},
		&model.MerchantAIBudget{}, &model.MerchantAIBudgetAlert{},
		&model.AIEmergencySwitch{}, &model.AuditLog{},
	); err != nil {
		t.Fatalf("migrate synthetic AI cost control schema: %v", err)
	}

	finance := createPlatformCostPermissionAdmin(t, db, "synthetic-ai-cost-finance", "finance")
	system := createPlatformCostPermissionAdmin(t, db, "synthetic-ai-cost-system", "system_admin")
	cfg := &config.Config{JWTSecret: "synthetic-ai-cost-permission-secret", FrontendURL: "http://localhost:5173", RateLimitPerMinute: 1000}
	superAdmin := seedSyntheticSuperAdmin(t, db, 0, "synthetic-ai-cost-super")
	engine := gin.New()
	Register(engine, db, cfg)

	financeToken := syntheticAdminToken(t, db, cfg, finance.ID)
	systemToken := syntheticAdminToken(t, db, cfg, system.ID)
	superToken := syntheticAdminToken(t, db, cfg, superAdmin.ID)
	for _, path := range []string{
		"/api/v1/admin/ai-cost-control/usage-stats",
		"/api/v1/admin/ai-cost-control/model-prices",
		"/api/v1/admin/ai-cost-control/vendor-bills",
		"/api/v1/admin/ai-cost-control/bills/reconciliation",
		"/api/v1/admin/ai-cost-control/budgets",
		"/api/v1/admin/ai-cost-control/budget-alerts",
		"/api/v1/admin/ai-cost-control/emergency-switch",
	} {
		t.Run(path, func(t *testing.T) {
			for _, test := range []struct {
				name       string
				token      string
				wantStatus int
			}{
				{name: "finance role denied", token: financeToken, wantStatus: http.StatusForbidden},
				{name: "system role denied", token: systemToken, wantStatus: http.StatusForbidden},
				{name: "super admin allowed", token: superToken, wantStatus: http.StatusOK},
			} {
				t.Run(test.name, func(t *testing.T) {
					request := httptest.NewRequest(http.MethodGet, path, nil)
					request.Header.Set("Authorization", "Bearer "+test.token)
					response := httptest.NewRecorder()
					engine.ServeHTTP(response, request)
					if response.Code != test.wantStatus {
						t.Fatalf("status=%d want=%d body=%s", response.Code, test.wantStatus, response.Body.String())
					}
				})
			}
		})
	}
}
