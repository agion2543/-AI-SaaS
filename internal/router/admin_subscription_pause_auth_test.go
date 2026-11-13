package router

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/utils"
)

func TestSubscriptionPauseRouteRequiresPlatformAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:subscription-pause-route?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open SQLite test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get SQL test database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&model.Merchant{}, &model.AuditLog{}, &model.AdminUser{}); err != nil {
		t.Fatalf("migrate test schema: %v", err)
	}

	expiry := time.Now().Add(48 * time.Hour)
	merchant := &model.Merchant{Name: "合成路由鉴权测试商家", Status: "active", SubscriptionStatus: "active", SubscriptionExpireAt: &expiry}
	if err := db.Create(merchant).Error; err != nil {
		t.Fatalf("create synthetic merchant: %v", err)
	}
	cfg := &config.Config{JWTSecret: "test-only-subscription-secret", FrontendURL: "http://localhost:5173", RateLimitPerMinute: 100}
	engine := gin.New()
	Register(engine, db, cfg)
	merchantID := merchant.ID
	merchantToken, err := utils.GenerateToken(cfg.JWTSecret, 202, "merchant_admin", "merchant_admin", &merchantID, 1)
	if err != nil {
		t.Fatalf("create merchant token: %v", err)
	}
	adminToken, err := utils.GenerateToken(cfg.JWTSecret, 7, "admin", "admin", nil, 1)
	if err != nil {
		t.Fatalf("create admin token: %v", err)
	}

	request := func(token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/merchants/"+uintString(merchant.ID)+"/subscription/pause", bytes.NewBufferString(`{"reason":"合成鉴权测试"}`))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, req)
		return response
	}
	if response := request(""); response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated pause status = %d, want 401", response.Code)
	}
	if response := request(merchantToken); response.Code != http.StatusForbidden {
		t.Fatalf("merchant role pause status = %d, want 403", response.Code)
	}
	if response := request(adminToken); response.Code != http.StatusOK {
		t.Fatalf("admin pause status = %d, want 200; body=%s", response.Code, response.Body.String())
	}
	var stored model.Merchant
	if err := db.First(&stored, merchant.ID).Error; err != nil {
		t.Fatalf("reload merchant: %v", err)
	}
	if stored.SubscriptionStatus != "paused" || stored.SubscriptionPauseReason != "合成鉴权测试" {
		t.Fatalf("admin request did not pause merchant: %+v", stored)
	}
}
