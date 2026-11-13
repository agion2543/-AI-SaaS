package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/service"
)

func TestSubscriptionPauseResumeControlsProtectedAccessAndExpiredPauseNeedsRenewal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open SQLite test database: %v", err)
	}
	if err := db.AutoMigrate(&model.Merchant{}, &model.MerchantPlan{}, &model.AuditLog{}); err != nil {
		t.Fatalf("migrate subscription test schema: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get SQL test database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	expiry := time.Now().Add(48 * time.Hour).Truncate(time.Microsecond)
	merchant := &model.Merchant{
		Name:                 "合成暂停恢复访问测试商家",
		Status:               "active",
		SubscriptionPlan:     "month",
		SubscriptionStatus:   "active",
		SubscriptionExpireAt: &expiry,
	}
	if err := db.Create(merchant).Error; err != nil {
		t.Fatalf("create synthetic merchant: %v", err)
	}

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		id, parseErr := strconv.ParseUint(c.GetHeader("X-Test-Merchant-ID"), 10, 64)
		if parseErr != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		merchantID := uint(id)
		c.Set("role", "merchant_admin")
		c.Set("merchant_id", &merchantID)
		c.Next()
	})
	engine.GET("/paid", MerchantSubscriptionMiddleware(db), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	protectedStatus := func(id uint) int {
		request := httptest.NewRequest(http.MethodGet, "/paid", nil)
		request.Header.Set("X-Test-Merchant-ID", strconv.FormatUint(uint64(id), 10))
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		return response.Code
	}

	if got := protectedStatus(merchant.ID); got != http.StatusNoContent {
		t.Fatalf("active unexpired subscription access status=%d, want %d", got, http.StatusNoContent)
	}
	admin := service.NewAdminService(nil, db)
	actor := service.MerchantSubscriptionActor{Type: "admin", IP: "127.0.0.1"}
	paused, err := admin.PauseMerchantSubscription(merchant.ID, "合成验证：商家申请暂停", actor)
	if err != nil {
		t.Fatalf("pause synthetic subscription: %v", err)
	}
	if paused.SubscriptionExpireAt == nil || !paused.SubscriptionExpireAt.Equal(expiry) {
		t.Fatalf("pause changed expiry: got %v, want %v", paused.SubscriptionExpireAt, expiry)
	}
	if got := protectedStatus(merchant.ID); got != http.StatusForbidden {
		t.Fatalf("paused subscription access status=%d, want %d", got, http.StatusForbidden)
	}

	resumed, err := admin.ResumeMerchantSubscription(merchant.ID, actor)
	if err != nil {
		t.Fatalf("resume unexpired synthetic subscription: %v", err)
	}
	if resumed.SubscriptionExpireAt == nil || !resumed.SubscriptionExpireAt.Equal(expiry) {
		t.Fatalf("resume changed expiry: got %v, want %v", resumed.SubscriptionExpireAt, expiry)
	}
	if got := protectedStatus(merchant.ID); got != http.StatusNoContent {
		t.Fatalf("resumed subscription access status=%d, want %d", got, http.StatusNoContent)
	}

	past := time.Now().Add(-time.Hour).Truncate(time.Microsecond)
	pausedAt := time.Now().Add(-2 * time.Hour)
	expired := &model.Merchant{
		Name:                    "合成过期暂停续订测试商家",
		Status:                  "active",
		SubscriptionPlan:        "month",
		SubscriptionStatus:      "paused",
		SubscriptionExpireAt:    &past,
		SubscriptionPauseReason: "合成过期暂停原因",
		SubscriptionPausedAt:    &pausedAt,
	}
	if err := db.Create(expired).Error; err != nil {
		t.Fatalf("create synthetic expired paused merchant: %v", err)
	}
	if _, err := admin.ResumeMerchantSubscription(expired.ID, actor); err == nil {
		t.Fatal("expired paused subscription unexpectedly resumed")
	}
	if got := protectedStatus(expired.ID); got != http.StatusForbidden {
		t.Fatalf("expired paused subscription access status=%d, want %d", got, http.StatusForbidden)
	}
	plan := &model.MerchantPlan{Name: "月付", PriceCents: 19900, DurationDays: 30}
	if err := db.Create(plan).Error; err != nil {
		t.Fatalf("create synthetic monthly plan: %v", err)
	}
	renewed, err := admin.OpenMerchantSubscription(expired.ID, dto.OpenMerchantSubscriptionRequest{Plan: "month", DurationDays: 30, Note: "合成续订验证"})
	if err != nil {
		t.Fatalf("renew expired paused subscription: %v", err)
	}
	if renewed.SubscriptionStatus != "active" || renewed.SubscriptionExpireAt == nil || !renewed.SubscriptionExpireAt.After(time.Now()) {
		t.Fatalf("renewal did not create an active future subscription: %+v", renewed)
	}
	if renewed.SubscriptionPauseReason != "" || renewed.SubscriptionPausedAt != nil {
		t.Fatalf("renewal retained stale pause metadata: reason=%q paused_at=%v", renewed.SubscriptionPauseReason, renewed.SubscriptionPausedAt)
	}
	if got := protectedStatus(expired.ID); got != http.StatusNoContent {
		t.Fatalf("renewed subscription access status=%d, want %d", got, http.StatusNoContent)
	}
}
