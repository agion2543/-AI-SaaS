package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/service"
)

func TestPauseMerchantSubscriptionRejectsMissingReasonWithoutAudit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open SQLite test database: %v", err)
	}
	if err := db.AutoMigrate(&model.Merchant{}, &model.AuditLog{}, &model.AdminUser{}); err != nil {
		t.Fatalf("migrate test schema: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get SQL test database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	expiry := time.Now().Add(48 * time.Hour)
	merchant := &model.Merchant{Name: "合成缺理由测试商家", Status: "active", SubscriptionStatus: "active", SubscriptionExpireAt: &expiry}
	if err := db.Create(merchant).Error; err != nil {
		t.Fatalf("create synthetic merchant: %v", err)
	}
	admin := service.NewAdminService(nil, db)
	audit := service.NewAuditService(db)
	controller := NewAdminController(admin, nil, audit)
	router := gin.New()
	router.POST("/merchants/:id/subscription/pause", controller.PauseMerchantSubscription)
	request := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/merchants/%d/subscription/pause", merchant.ID), strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("response status = %d, want %d; body=%s", response.Code, http.StatusBadRequest, response.Body.String())
	}

	var stored model.Merchant
	if err := db.First(&stored, merchant.ID).Error; err != nil {
		t.Fatalf("reload merchant: %v", err)
	}
	if stored.SubscriptionStatus != "active" || stored.SubscriptionPauseReason != "" || stored.SubscriptionPausedAt != nil || stored.SubscriptionExpireAt == nil || !stored.SubscriptionExpireAt.Equal(expiry) {
		t.Fatalf("missing-reason request partially changed merchant: %+v", stored)
	}
	var auditCount int64
	if err := db.Model(&model.AuditLog{}).Where("action = ?", "merchant_subscription_pause").Count(&auditCount).Error; err != nil {
		t.Fatalf("count pause audit records: %v", err)
	}
	if auditCount != 0 {
		t.Fatalf("pause audit count = %d, want 0", auditCount)
	}
}
