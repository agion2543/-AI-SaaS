package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/service"
)

// 渲染验证发现的保本试算参数名不一致回归测试：成本页前端发送
// avg_revenue_per_merchant_cents / avg_variable_cost_per_merchant_cents，
// 而后端只读 merchant_monthly_*，导致页面上的保本试算永远缺参数。
// 修复后后端两个名字都认，前端改发规范名；本测试覆盖控制器层的参数解析。
func TestGetPlatformBreakevenAcceptsLegacyAvgParams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open SQLite test database: %v", err)
	}
	if err := db.AutoMigrate(&model.PlatformExpense{}, &model.PlatformTimeLog{}); err != nil {
		t.Fatalf("migrate test schema: %v", err)
	}
	if err := db.Exec(`CREATE TABLE platform_expense_reversals (
		id INTEGER PRIMARY KEY AUTOINCREMENT, created_at datetime NOT NULL, updated_at datetime NOT NULL,
		expense_id INTEGER NOT NULL REFERENCES platform_expenses(id) ON DELETE RESTRICT,
		amount_cents INTEGER NOT NULL CHECK (amount_cents > 0), reversal_date date NOT NULL,
		reason varchar(500) NOT NULL CHECK (LENGTH(TRIM(reason)) BETWEEN 1 AND 500),
		created_by INTEGER NOT NULL, idempotency_key char(36) NOT NULL UNIQUE
	)`).Error; err != nil {
		t.Fatalf("create isolated synthetic reversal table: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get SQL test database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	// 当月固定现金成本 ¥200（20000 分），与试算的单商家贡献毛利
	// 12900 - 1000 = 11900 分对应保本 2 家：ceil(20000/11900) = 2。
	if err := db.Create(&model.PlatformExpense{
		Category: "cloud_hosting", AmountCents: 20000, Currency: "CNY",
		ExpenseDate: model.PlatformExpenseDate("2026-11-10"), Memo: "合成固定成本",
		CostType: model.PlatformExpenseCostTypeFixed, IsCashCost: true, IsEconomicCost: true,
		CreatedBy: 1, IdempotencyKey: uuid.NewString(),
	}).Error; err != nil {
		t.Fatalf("create synthetic fixed expense: %v", err)
	}

	admin := service.NewAdminService(nil, db)
	controller := NewAdminController(admin, nil, nil)
	router := gin.New()
	router.GET("/admin/platform-finance/breakeven", controller.GetPlatformBreakeven)

	call := func(query string) (int, service.PlatformBreakevenResult) {
		t.Helper()
		request := httptest.NewRequest(http.MethodGet, "/admin/platform-finance/breakeven?"+query, nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		var envelope struct {
			Code int                             `json:"code"`
			Data service.PlatformBreakevenResult `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
			t.Fatalf("decode breakeven response for %q: %v (body=%s)", query, err, response.Body.String())
		}
		return response.Code, envelope.Data
	}

	// 1. 前端旧参数名 avg_* 也必须算出结果。
	code, result := call("month=2026-11&avg_revenue_per_merchant_cents=12900&avg_variable_cost_per_merchant_cents=1000")
	if code != http.StatusOK {
		t.Fatalf("legacy avg params: status = %d, want %d", code, http.StatusOK)
	}
	if result.ParamsMissing || result.BreakevenMerchants == nil || *result.BreakevenMerchants != 2 {
		t.Fatalf("legacy avg params: params_missing=%v breakeven_merchants=%v, want 2", result.ParamsMissing, result.BreakevenMerchants)
	}
	if result.FixedCashCostCents != 20000 || result.ContributionMarginCents == nil || *result.ContributionMarginCents != 11900 {
		t.Fatalf("legacy avg params: fixed=%d margin=%v, want 20000/11900", result.FixedCashCostCents, result.ContributionMarginCents)
	}

	// 2. 规范名 merchant_monthly_* 同样算出结果。
	code, result = call("month=2026-11&merchant_monthly_revenue_cents=12900&merchant_monthly_variable_cents=1000")
	if code != http.StatusOK || result.ParamsMissing || result.BreakevenMerchants == nil || *result.BreakevenMerchants != 2 {
		t.Fatalf("canonical params: status=%d params_missing=%v breakeven=%v, want 200/2", code, result.ParamsMissing, result.BreakevenMerchants)
	}

	// 3. 规范名优先于旧名。
	code, result = call("month=2026-11&merchant_monthly_revenue_cents=20000&avg_revenue_per_merchant_cents=12900&merchant_monthly_variable_cents=5000&avg_variable_cost_per_merchant_cents=1000")
	if code != http.StatusOK || result.ContributionMarginCents == nil || *result.ContributionMarginCents != 15000 {
		t.Fatalf("canonical precedence: status=%d margin=%v, want 15000", code, result.ContributionMarginCents)
	}

	// 4. 不给参数仍返回缺参数（不报错、不输出保本结论）。
	code, result = call("month=2026-11")
	if code != http.StatusOK || !result.ParamsMissing || result.BreakevenMerchants != nil {
		t.Fatalf("missing params: status=%d params_missing=%v breakeven=%v, want 200/true/nil", code, result.ParamsMissing, result.BreakevenMerchants)
	}

	// 5. 旧名非法值仍返回 400。
	request := httptest.NewRequest(http.MethodGet, "/admin/platform-finance/breakeven?month=2026-11&avg_revenue_per_merchant_cents=abc", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid legacy param: status = %d, want %d; body=%s", response.Code, http.StatusBadRequest, response.Body.String())
	}
}
