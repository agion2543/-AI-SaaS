package router

import (
	"bytes"
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

func TestSubscriptionPaymentConfirmationRouteRequiresPlatformAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open SQLite test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get SQL test database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&model.Merchant{}, &model.MerchantPlan{}, &model.Order{}, &model.PaymentRecord{}, &model.AuditLog{}, &model.AdminUser{}); err != nil {
		t.Fatalf("migrate synthetic subscription schema: %v", err)
	}
	merchant := &model.Merchant{Name: "合成订阅确认鉴权商家", Status: "active", SubscriptionStatus: "inactive"}
	if err := db.Create(merchant).Error; err != nil {
		t.Fatalf("create synthetic merchant: %v", err)
	}
	plan := &model.MerchantPlan{Name: "合成月付", PriceCents: 19900, DurationDays: 30}
	if err := db.Create(plan).Error; err != nil {
		t.Fatalf("create synthetic plan: %v", err)
	}
	planName, planPrice, planDays := plan.Name, plan.PriceCents, plan.DurationDays
	order := &model.Order{
		OrderNo:                  "SYNTHETIC-SUBSCRIPTION-AUTH-ORDER",
		OrderType:                "merchant_subscription",
		MerchantID:               &merchant.ID,
		MerchantPlanID:           &plan.ID,
		PlanNameSnapshot:         &planName,
		PlanPriceCentsSnapshot:   &planPrice,
		PlanDurationDaysSnapshot: &planDays,
		Amount:                   planPrice,
		TotalAmount:              planPrice,
		Status:                   "pending",
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create synthetic subscription order: %v", err)
	}
	admin := &model.AdminUser{Username: "synthetic-platform-admin", DisplayName: "合成平台管理员", Status: "active"}
	if err := db.Create(admin).Error; err != nil {
		t.Fatalf("create synthetic platform admin: %v", err)
	}

	cfg := &config.Config{JWTSecret: "test-only-subscription-payment-secret", FrontendURL: "http://localhost:5173", RateLimitPerMinute: 1000}
	engine := gin.New()
	Register(engine, db, cfg)
	merchantID := merchant.ID
	merchantToken, err := utils.GenerateToken(cfg.JWTSecret, 202, "merchant_admin", "merchant_admin", &merchantID, 1)
	if err != nil {
		t.Fatalf("create merchant token: %v", err)
	}
	adminToken, err := utils.GenerateToken(cfg.JWTSecret, admin.ID, "admin", "admin", nil, 1)
	if err != nil {
		t.Fatalf("create platform admin token: %v", err)
	}
	path := "/api/v1/admin/orders/" + fmt.Sprint(order.ID) + "/confirm-subscription-payment"
	request := func(token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(`{"remark":"synthetic authorization test"}`))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, req)
		return response
	}
	if response := request(""); response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated confirmation status=%d, want 401", response.Code)
	}
	if response := request(merchantToken); response.Code != http.StatusForbidden {
		t.Fatalf("merchant confirmation status=%d, want 403; body=%s", response.Code, response.Body.String())
	}
	var unchanged model.Order
	if err := db.First(&unchanged, order.ID).Error; err != nil {
		t.Fatalf("reload order after forbidden attempt: %v", err)
	}
	var payments int64
	if err := db.Model(&model.PaymentRecord{}).Where("order_id = ?", order.ID).Count(&payments).Error; err != nil {
		t.Fatalf("count payment records: %v", err)
	}
	if unchanged.Status != "pending" || unchanged.PaidAt != nil || payments != 0 {
		t.Fatalf("forbidden merchant request changed financial state: status=%q paid_at=%v payments=%d", unchanged.Status, unchanged.PaidAt, payments)
	}
	if err := db.Exec(`CREATE TRIGGER synthetic_fail_audit BEFORE INSERT ON audit_logs BEGIN SELECT RAISE(ABORT, 'synthetic audit storage failure'); END`).Error; err != nil {
		t.Fatalf("create audit failure trigger: %v", err)
	}
	if response := request(adminToken); response.Code != http.StatusBadRequest {
		t.Fatalf("admin confirmation with unavailable audit status=%d, want 400; body=%s", response.Code, response.Body.String())
	}
	if err := db.First(&unchanged, order.ID).Error; err != nil {
		t.Fatalf("reload order after audit failure: %v", err)
	}
	if err := db.Model(&model.PaymentRecord{}).Where("order_id = ?", order.ID).Count(&payments).Error; err != nil {
		t.Fatalf("count payment records after audit failure: %v", err)
	}
	if unchanged.Status != "pending" || unchanged.PaidAt != nil || payments != 0 {
		t.Fatalf("audit failure left partial financial state: status=%q paid_at=%v payments=%d", unchanged.Status, unchanged.PaidAt, payments)
	}
	if err := db.Exec(`DROP TRIGGER synthetic_fail_audit`).Error; err != nil {
		t.Fatalf("drop audit failure trigger: %v", err)
	}
	if response := request(adminToken); response.Code != http.StatusOK {
		t.Fatalf("platform admin confirmation status=%d, want 200; body=%s", response.Code, response.Body.String())
	}
	if err := db.First(&unchanged, order.ID).Error; err != nil {
		t.Fatalf("reload confirmed order: %v", err)
	}
	if err := db.Model(&model.PaymentRecord{}).Where("order_id = ?", order.ID).Count(&payments).Error; err != nil {
		t.Fatalf("count confirmed payment records: %v", err)
	}
	if unchanged.Status != "paid" || unchanged.PaidAt == nil || payments != 1 {
		t.Fatalf("admin confirmation did not persist one synthetic payment: status=%q paid_at=%v payments=%d", unchanged.Status, unchanged.PaidAt, payments)
	}
	var audits int64
	if err := db.Model(&model.AuditLog{}).Where("action = ? AND target_id = ?", "merchant_subscription_payment_confirm", order.ID).Count(&audits).Error; err != nil {
		t.Fatalf("count admin confirmation audits: %v", err)
	}
	if audits != 1 {
		t.Fatalf("admin confirmation audit count=%d, want 1", audits)
	}
}
