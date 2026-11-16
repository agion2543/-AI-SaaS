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

func TestHistoricalSubscriptionManualReviewRouteRequiresPlatformAdmin(t *testing.T) {
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
	if err := db.AutoMigrate(&model.Merchant{}, &model.MerchantPlan{}, &model.Order{}, &model.PaymentRecord{}, &model.MerchantSubscriptionManualReview{}, &model.MerchantSubscriptionManualReviewAudit{}); err != nil {
		t.Fatalf("migrate synthetic manual-review schema: %v", err)
	}
	seedSyntheticSuperAdmin(t, db, 7, "synthetic-manual-review-super")
	merchant := &model.Merchant{Name: "合成路由鉴权商家", Status: "active", SubscriptionStatus: "inactive"}
	if err := db.Create(merchant).Error; err != nil {
		t.Fatal(err)
	}
	order := &model.Order{OrderNo: "SYNTHETIC-ROUTE-MANUAL-REVIEW", OrderType: "merchant_subscription", MerchantID: &merchant.ID, Amount: 25000, TotalAmount: 25000, Status: "pending"}
	if err := db.Create(order).Error; err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{JWTSecret: "synthetic-manual-review-route-secret", FrontendURL: "http://localhost:5173", RateLimitPerMinute: 1000}
	engine := gin.New()
	Register(engine, db, cfg)
	merchantID := merchant.ID
	merchantToken, err := utils.GenerateToken(cfg.JWTSecret, 210, "merchant_admin", "merchant_admin", &merchantID, 1)
	if err != nil {
		t.Fatal(err)
	}
	userToken, err := utils.GenerateToken(cfg.JWTSecret, 211, "user", "user", nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	adminToken := syntheticAdminToken(t, db, cfg, 7)
	path := "/api/v1/admin/orders/" + fmt.Sprint(order.ID) + "/review-historical-subscription"
	body := `{"plan_name":"历史原计划","amount_cents":25000,"duration_days":30,"payment_occurred_attested":true,"evidence_reference":"synthetic-source-001","reason":"合成路由测试：原始历史凭证已核对"}`
	request := func(token, payload string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, req)
		return response
	}
	if response := request("", body); response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status=%d, want 401", response.Code)
	}
	if response := request(merchantToken, body); response.Code != http.StatusForbidden {
		t.Fatalf("merchant tenant status=%d, want 403; body=%s", response.Code, response.Body.String())
	}
	if response := request(userToken, body); response.Code != http.StatusForbidden {
		t.Fatalf("customer user status=%d, want 403; body=%s", response.Code, response.Body.String())
	}
	var unchanged model.Order
	if err := db.First(&unchanged, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	if unchanged.Status != "pending" || unchanged.PaidAt != nil {
		t.Fatalf("denied requests changed order: %+v", unchanged)
	}
	response := request(adminToken, body)
	if response.Code != http.StatusOK {
		t.Fatalf("platform admin status=%d, want 200; body=%s", response.Code, response.Body.String())
	}
	if err := db.First(&unchanged, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	if unchanged.Status != "manually_verified" || unchanged.PaidAt != nil || unchanged.TransactionNo != "" {
		t.Fatalf("admin route fabricated provider payment markers: %+v", unchanged)
	}
	var payments, reviews, audits int64
	if err := db.Model(&model.PaymentRecord{}).Where("order_id = ?", order.ID).Count(&payments).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.MerchantSubscriptionManualReview{}).Where("order_id = ?", order.ID).Count(&reviews).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.MerchantSubscriptionManualReviewAudit{}).Where("order_id = ?", order.ID).Count(&audits).Error; err != nil {
		t.Fatal(err)
	}
	if payments != 0 || reviews != 1 || audits != 1 {
		t.Fatalf("provider payment/manual-review/audit counts=%d/%d/%d", payments, reviews, audits)
	}
}
