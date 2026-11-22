package router

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/utils"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMerchantExpiredReadOnlyPauseLockAndRenewalRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open SQLite test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get SQL test database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(
		&model.Merchant{}, &model.User{}, &model.Store{}, &model.MerchantPlan{},
		&model.MembershipPackage{}, &model.Order{}, &model.MerchantSettlement{},
		&model.PaymentRecord{}, &model.Promotion{}, &model.MerchantPaymentConfig{},
		&model.MerchantGroupBuyLink{}, &model.MerchantGroupBuyLinkAudit{},
	); err != nil {
		t.Fatalf("migrate synthetic route schema: %v", err)
	}

	past := time.Now().Add(-time.Hour)
	merchant := model.Merchant{Name: "合成到期只读路由测试", Status: "active", SubscriptionStatus: "active", SubscriptionExpireAt: &past}
	if err := db.Create(&merchant).Error; err != nil {
		t.Fatalf("create synthetic merchant: %v", err)
	}
	merchantID := merchant.ID
	user := model.User{UUID: uuid.NewString(), Email: "synthetic-merchant@example.invalid", Phone: "13800138000", MerchantID: &merchantID, Role: "merchant_admin", Status: "active"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create synthetic merchant user: %v", err)
	}
	plan := model.MerchantPlan{Name: "合成月付", PriceCents: 100, DurationDays: 30}
	if err := db.Create(&plan).Error; err != nil {
		t.Fatalf("create synthetic merchant plan: %v", err)
	}
	fallback := model.MembershipPackage{Name: "合成占位套餐", Code: "synthetic-merchant-subscription-fallback", DurationDays: 30, Price: 100, Status: "published"}
	if err := db.Create(&fallback).Error; err != nil {
		t.Fatalf("create synthetic fallback package: %v", err)
	}
	store := model.Store{MerchantID: merchant.ID, Name: "既有门店", Status: "active", IsOpen: true}
	if err := db.Create(&store).Error; err != nil {
		t.Fatalf("create synthetic existing store: %v", err)
	}

	cfg := &config.Config{JWTSecret: "test-only-secret", FrontendURL: "http://localhost:5173", RateLimitPerMinute: 100}
	engine := gin.New()
	Register(engine, db, cfg)
	token, err := utils.GenerateUserToken(cfg.JWTSecret, user, 1)
	if err != nil {
		t.Fatalf("create synthetic merchant token: %v", err)
	}
	request := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, req)
		return response
	}

	if response := request(http.MethodGet, "/api/v1/merchant/stores", ""); response.Code != http.StatusOK {
		t.Fatalf("expired merchant could not read existing stores: status=%d body=%s", response.Code, response.Body.String())
	}
	if response := request(http.MethodGet, "/api/v1/merchant/group-buy-links?store_id="+strconv.FormatUint(uint64(store.ID), 10), ""); response.Code != http.StatusOK {
		t.Fatalf("expired merchant could not read existing group-buy links: status=%d body=%s", response.Code, response.Body.String())
	}
	for _, test := range []struct{ method, path, body string }{
		{http.MethodPost, "/api/v1/merchant/stores", `{"name":"不应创建","contact_phone":"13800138000"}`},
		{http.MethodPut, "/api/v1/merchant/stores/1", `{"name":"不应更新","contact_phone":"13800138000"}`},
		{http.MethodDelete, "/api/v1/merchant/stores/1", ""},
		{http.MethodGet, "/api/v1/merchant/coupons", ""},
	} {
		if response := request(test.method, test.path, test.body); response.Code != http.StatusForbidden {
			t.Errorf("expired merchant %s %s status=%d, want 403 body=%s", test.method, test.path, response.Code, response.Body.String())
		}
	}
	for _, path := range []string{
		"/api/v1/merchant/share/config", "/api/v1/merchant/ai/share-stats",
	} {
		if response := request(http.MethodGet, path, ""); response.Code != http.StatusForbidden {
			t.Errorf("expired merchant side-effectful GET %s status=%d, want 403 body=%s", path, response.Code, response.Body.String())
		}
	}
	if response := request(http.MethodGet, "/api/v1/merchant/check-subscription", ""); response.Code != http.StatusOK {
		t.Fatalf("expired merchant lost subscription recovery status endpoint: status=%d body=%s", response.Code, response.Body.String())
	}
	if response := request(http.MethodPost, "/api/v1/merchant/subscription/orders", `{"plan_id":1,"payment_channel":"platform_qr"}`); response.Code != http.StatusOK {
		t.Fatalf("expired merchant could not create renewal order: status=%d body=%s", response.Code, response.Body.String())
	}
	var order model.Order
	if err := db.Where("merchant_id = ? AND order_type = ?", merchant.ID, "merchant_subscription").First(&order).Error; err != nil {
		t.Fatalf("load synthetic renewal order: %v", err)
	}
	if response := request(http.MethodPost, "/api/v1/merchant/subscription/orders/"+order.OrderNo+"/paid", ""); response.Code != http.StatusOK {
		t.Fatalf("expired merchant could not submit payment notification: status=%d body=%s", response.Code, response.Body.String())
	}

	if err := db.Model(&model.Merchant{}).Where("id = ?", merchant.ID).Update("subscription_status", "paused").Error; err != nil {
		t.Fatalf("set synthetic safety pause: %v", err)
	}
	if response := request(http.MethodGet, "/api/v1/merchant/stores", ""); response.Code != http.StatusForbidden {
		t.Fatalf("paused merchant business read status=%d, want 403 body=%s", response.Code, response.Body.String())
	}
	if response := request(http.MethodGet, "/api/v1/merchant/check-subscription", ""); response.Code != http.StatusOK {
		t.Fatalf("paused merchant lost subscription status endpoint: status=%d body=%s", response.Code, response.Body.String())
	}
	if response := request(http.MethodPost, "/api/v1/merchant/subscription/orders", `{"plan_id":1,"payment_channel":"platform_qr"}`); response.Code != http.StatusOK {
		t.Fatalf("paused merchant could not create a renewal order: status=%d body=%s", response.Code, response.Body.String())
	}

	future := time.Now().Add(time.Hour)
	if err := db.Model(&model.Merchant{}).Where("id = ?", merchant.ID).Updates(map[string]interface{}{"subscription_status": "active", "subscription_expire_at": future}).Error; err != nil {
		t.Fatalf("restore synthetic active subscription: %v", err)
	}
	if response := request(http.MethodPost, "/api/v1/merchant/stores", `{"name":"有效期内创建","contact_phone":"13800138000"}`); response.Code != http.StatusOK {
		t.Fatalf("active merchant behavior changed: status=%d body=%s", response.Code, response.Body.String())
	}
	if err := db.Model(&model.Merchant{}).Where("id = ?", merchant.ID).Update("status", "pending").Error; err != nil {
		t.Fatalf("set synthetic merchant to pending: %v", err)
	}
	if response := request(http.MethodGet, "/api/v1/merchant/stores", ""); response.Code != http.StatusForbidden {
		t.Errorf("pending merchant business read status=%d, want %d; body=%s", response.Code, http.StatusForbidden, response.Body.String())
	}
	if response := request(http.MethodGet, "/api/v1/merchant/check-subscription", ""); response.Code != http.StatusOK {
		t.Errorf("pending merchant lost subscription status recovery route: status=%d body=%s", response.Code, response.Body.String())
	}
}
