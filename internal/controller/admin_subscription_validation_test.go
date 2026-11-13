package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/service"
)

func TestOpenMerchantSubscriptionRejectsNegativeDurationWithoutAudit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open SQLite test database: %v", err)
	}
	if err := db.AutoMigrate(&model.Merchant{}, &model.MerchantPlan{}, &model.AuditLog{}); err != nil {
		t.Fatalf("migrate admin subscription test schema: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get SQL test database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	merchant := &model.Merchant{
		Name:               "管理员负时长测试商家",
		Status:             "active",
		SubscriptionPlan:   "none",
		SubscriptionStatus: "inactive",
		SubscriptionNote:   "unchanged",
	}
	if err := db.Create(merchant).Error; err != nil {
		t.Fatalf("create merchant: %v", err)
	}
	plan := &model.MerchantPlan{Name: "月付", PriceCents: 19900, DurationDays: 30}
	if err := db.Create(plan).Error; err != nil {
		t.Fatalf("create merchant plan: %v", err)
	}

	admin := service.NewAdminService(nil, db)
	audit := service.NewAuditService(db)
	controller := NewAdminController(admin, nil, audit)
	router := gin.New()
	router.POST("/merchants/:id/subscription/open", controller.OpenMerchantSubscription)
	request := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/merchants/%d/subscription/open", merchant.ID), strings.NewReader(`{"plan":"month","duration_days":-14,"note":"must not be saved"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("response status = %d, want %d; body=%s", response.Code, http.StatusBadRequest, response.Body.String())
	}

	var storedMerchant model.Merchant
	if err := db.First(&storedMerchant, merchant.ID).Error; err != nil {
		t.Fatalf("reload merchant: %v", err)
	}
	if storedMerchant.SubscriptionPlan != "none" || storedMerchant.SubscriptionStatus != "inactive" || storedMerchant.SubscriptionNote != "unchanged" || storedMerchant.SubscriptionExpireAt != nil || storedMerchant.SubscriptionExpiredAt != nil {
		t.Fatalf("invalid request partially changed merchant subscription: %+v", storedMerchant)
	}
	var auditCount int64
	if err := db.Model(&model.AuditLog{}).Where("action = ?", "merchant_subscription_open").Count(&auditCount).Error; err != nil {
		t.Fatalf("count subscription-open audit records: %v", err)
	}
	if auditCount != 0 {
		t.Fatalf("audit records = %d, want 0 for rejected request", auditCount)
	}
}
