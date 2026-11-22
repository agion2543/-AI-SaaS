package router

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/model"
)

func TestPlatformCostOverviewIsSuperAdminOnlyOnBothAPIPrefixes(t *testing.T) {
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
	if err := db.AutoMigrate(&model.AdminRole{}, &model.AdminUser{}, &model.MerchantPlan{}, &model.Merchant{}, &model.Order{}, &model.PaymentRecord{}, &model.RefundRecord{}, &model.MerchantSubscriptionManualReview{}, &model.MerchantSubscriptionManualReviewAudit{}, &model.PlatformExpense{}, &model.AuditLog{}); err != nil {
		t.Fatalf("migrate synthetic platform cost schema: %v", err)
	}
	createSyntheticReversalTable(t, db)

	finance := createPlatformCostPermissionAdmin(t, db, "synthetic-platform-cost-finance", "finance")
	system := createPlatformCostPermissionAdmin(t, db, "synthetic-platform-cost-system", "system_admin")
	cfg := &config.Config{JWTSecret: "synthetic-platform-cost-permission-secret", FrontendURL: "http://localhost:5173", RateLimitPerMinute: 1000}
	superAdmin := seedSyntheticSuperAdmin(t, db, 0, "synthetic-platform-cost-super")
	engine := gin.New()
	Register(engine, db, cfg)

	financeToken := syntheticAdminToken(t, db, cfg, finance.ID)
	systemToken := syntheticAdminToken(t, db, cfg, system.ID)
	superToken := syntheticAdminToken(t, db, cfg, superAdmin.ID)
	for _, path := range []string{"/api/admin/platform-costs/overview", "/api/v1/admin/platform-costs/overview"} {
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
					if test.wantStatus == http.StatusOK && (!strings.Contains(response.Body.String(), `"status":"not_connected"`) || !strings.Contains(response.Body.String(), `"data_available":false`)) {
						t.Fatalf("expected explicit unavailable AI usage state, body=%s", response.Body.String())
					}
				})
			}
		})
	}
}

func createPlatformCostPermissionAdmin(t *testing.T, db *gorm.DB, username, permission string) model.AdminUser {
	t.Helper()
	role := model.AdminRole{Name: "Synthetic " + username, Code: username, Permissions: permission}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("create synthetic role %q: %v", username, err)
	}
	admin := model.AdminUser{Username: username, DisplayName: "Synthetic Cost Permission Admin", Status: "active", RoleID: role.ID}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create synthetic administrator %q: %v", username, err)
	}
	return admin
}

func TestPlatformExpenseWriteIsSuperAdminOnlyOnBothAPIPrefixes(t *testing.T) {
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
	if err := db.AutoMigrate(&model.AdminRole{}, &model.AdminUser{}, &model.MerchantPlan{}, &model.Merchant{}, &model.Order{}, &model.PaymentRecord{}, &model.RefundRecord{}, &model.PlatformExpense{}, &model.AuditLog{}); err != nil {
		t.Fatalf("migrate synthetic expense authorization schema: %v", err)
	}
	createSyntheticReversalTable(t, db)

	finance := createPlatformCostPermissionAdmin(t, db, "synthetic-expense-finance", "finance")
	system := createPlatformCostPermissionAdmin(t, db, "synthetic-expense-system", "system_admin")
	cfg := &config.Config{JWTSecret: "synthetic-expense-write-permission-secret", FrontendURL: "http://localhost:5173", RateLimitPerMinute: 1000}
	superAdmin := seedSyntheticSuperAdmin(t, db, 0, "synthetic-expense-super")
	engine := gin.New()
	Register(engine, db, cfg)
	financeToken := syntheticAdminToken(t, db, cfg, finance.ID)
	systemToken := syntheticAdminToken(t, db, cfg, system.ID)
	superToken := syntheticAdminToken(t, db, cfg, superAdmin.ID)
	requestBody := `{"category":"cloud_hosting","amount_cents":1234,"currency":"CNY","expense_date":"` + time.Now().In(time.Local).Format("2006-01-02") + `","memo":"synthetic authorization test","idempotency_key":"` + uuid.NewString() + `"}`

	for _, path := range []string{"/api/admin/platform-costs/expenses", "/api/v1/admin/platform-costs/expenses"} {
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
					request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(requestBody))
					request.Header.Set("Authorization", "Bearer "+test.token)
					request.Header.Set("Content-Type", "application/json")
					response := httptest.NewRecorder()
					engine.ServeHTTP(response, request)
					if response.Code != test.wantStatus {
						t.Fatalf("status=%d want=%d body=%s", response.Code, test.wantStatus, response.Body.String())
					}
				})
			}
		})
	}
	var expenses, audits int64
	if err := db.Model(&model.PlatformExpense{}).Count(&expenses).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.AuditLog{}).Where("action = ?", "platform_expense_create").Count(&audits).Error; err != nil {
		t.Fatal(err)
	}
	if expenses != 1 || audits != 1 {
		t.Fatalf("authorized duplicate retry should leave one expense/audit, got %d/%d", expenses, audits)
	}
}

func TestPlatformExpenseReversalIsSuperAdminOnlyOnBothAPIPrefixes(t *testing.T) {
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
	if err := db.AutoMigrate(&model.AdminRole{}, &model.AdminUser{}, &model.MerchantPlan{}, &model.Merchant{}, &model.Order{}, &model.PaymentRecord{}, &model.RefundRecord{}, &model.PlatformExpense{}, &model.AuditLog{}); err != nil {
		t.Fatalf("migrate synthetic reversal authorization schema: %v", err)
	}
	createSyntheticReversalTable(t, db)

	finance := createPlatformCostPermissionAdmin(t, db, "synthetic-reversal-finance", "finance")
	system := createPlatformCostPermissionAdmin(t, db, "synthetic-reversal-system", "system_admin")
	cfg := &config.Config{JWTSecret: "synthetic-expense-reversal-permission-secret", FrontendURL: "http://localhost:5173", RateLimitPerMinute: 1000}
	superAdmin := seedSyntheticSuperAdmin(t, db, 0, "synthetic-reversal-super")
	expense := model.PlatformExpense{
		Category: model.PlatformExpenseCategoryCloudHosting, AmountCents: 1234, Currency: "CNY",
		ExpenseDate: model.PlatformExpenseDate(time.Now().In(time.Local).Format("2006-01-02")),
		Memo:        "synthetic reversal authorization expense", CreatedBy: superAdmin.ID, IdempotencyKey: uuid.NewString(),
	}
	if err := db.Create(&expense).Error; err != nil {
		t.Fatalf("create synthetic original platform expense: %v", err)
	}
	engine := gin.New()
	Register(engine, db, cfg)
	financeToken := syntheticAdminToken(t, db, cfg, finance.ID)
	systemToken := syntheticAdminToken(t, db, cfg, system.ID)
	superToken := syntheticAdminToken(t, db, cfg, superAdmin.ID)
	requestBody := `{"reversal_date":"` + time.Now().In(time.Local).Format("2006-01-02") + `","amount_cents":500,"reason":"synthetic API reversal reason","idempotency_key":"` + uuid.NewString() + `"}`

	for _, path := range []string{"/api/admin/platform-costs/expenses/" + strconv.Itoa(int(expense.ID)) + "/reversals", "/api/v1/admin/platform-costs/expenses/" + strconv.Itoa(int(expense.ID)) + "/reversals"} {
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
					request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(requestBody))
					request.Header.Set("Authorization", "Bearer "+test.token)
					request.Header.Set("Content-Type", "application/json")
					response := httptest.NewRecorder()
					engine.ServeHTTP(response, request)
					if response.Code != test.wantStatus {
						t.Fatalf("status=%d want=%d body=%s", response.Code, test.wantStatus, response.Body.String())
					}
				})
			}
		})
	}
	path := "/api/v1/admin/platform-costs/expenses/" + strconv.Itoa(int(expense.ID)) + "/reversals"
	for _, tc := range []struct {
		name string
		body string
		want int
	}{
		{"over remaining balance", `{"reversal_date":"` + time.Now().In(time.Local).Format("2006-01-02") + `","amount_cents":735,"reason":"synthetic amount above remaining","idempotency_key":"` + uuid.NewString() + `"}`, http.StatusConflict},
		{"missing amount", `{"reversal_date":"` + time.Now().In(time.Local).Format("2006-01-02") + `","reason":"synthetic missing amount","idempotency_key":"` + uuid.NewString() + `"}`, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(tc.body))
			request.Header.Set("Authorization", "Bearer "+superToken)
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			if response.Code != tc.want {
				t.Fatalf("status=%d want=%d body=%s", response.Code, tc.want, response.Body.String())
			}
		})
	}
	var reversals, audits int64
	if err := db.Model(&model.PlatformExpenseReversal{}).Count(&reversals).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.AuditLog{}).Where("action = ?", "platform_expense_reversal_create").Count(&audits).Error; err != nil {
		t.Fatal(err)
	}
	if reversals != 1 || audits != 1 {
		t.Fatalf("authorization or idempotent route replay mismatch: reversals=%d audits=%d", reversals, audits)
	}
}

func createSyntheticReversalTable(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec(`CREATE TABLE platform_expense_reversals (
		id INTEGER PRIMARY KEY AUTOINCREMENT, created_at datetime NOT NULL, updated_at datetime NOT NULL,
		expense_id INTEGER NOT NULL REFERENCES platform_expenses(id) ON DELETE RESTRICT,
		amount_cents INTEGER NOT NULL CHECK (amount_cents > 0), reversal_date date NOT NULL,
		reason varchar(500) NOT NULL CHECK (LENGTH(TRIM(reason)) BETWEEN 1 AND 500),
		created_by INTEGER NOT NULL, idempotency_key char(36) NOT NULL UNIQUE
	)`).Error; err != nil {
		t.Fatalf("create isolated synthetic reversal table with explicit DDL: %v", err)
	}
}
