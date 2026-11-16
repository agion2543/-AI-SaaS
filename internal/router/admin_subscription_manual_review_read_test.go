package router

import (
	"bytes"
	"encoding/json"
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

	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/service"
	"go-web-gin-health/internal/utils"
)

func TestManualSubscriptionReviewReadRouteIsAdminOnlyReadOnlyAndSeparate(t *testing.T) {
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
		t.Fatalf("migrate synthetic read schema: %v", err)
	}
	seedSyntheticSuperAdmin(t, db, 7, "synthetic-manual-read-super")

	merchant := &model.Merchant{Name: "合成审计商家", Status: "active", SubscriptionStatus: "active"}
	if err := db.Create(merchant).Error; err != nil {
		t.Fatal(err)
	}
	order := &model.Order{
		OrderNo:        "SYNTHETIC-MANUAL-READ-ONLY",
		OrderType:      "merchant_subscription",
		MerchantID:     &merchant.ID,
		Amount:         19900,
		TotalAmount:    19900,
		Status:         "manually_verified",
		PaymentChannel: "admin_manual_verified",
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatal(err)
	}
	reviewedAt := time.Date(2026, 11, 13, 7, 55, 0, 0, time.UTC)
	review := &model.MerchantSubscriptionManualReview{
		ReviewKey:               "synthetic-read-only-review-key",
		OrderID:                 order.ID,
		OrderNo:                 order.OrderNo,
		MerchantID:              merchant.ID,
		ActorID:                 17,
		ActorRole:               "admin",
		PlanName:                "合成历史月付",
		AmountCents:             19900,
		DurationDays:            30,
		PaymentOccurredAttested: true,
		EvidenceReference:       "SYNTHETIC-EXTERNAL-EVIDENCE-17",
		Reason:                  "合成测试：仅验证独立人工审计展示",
		ReviewStatus:            "manually_verified",
	}
	if err := db.Create(review).Error; err != nil {
		t.Fatal(err)
	}
	audit := &model.MerchantSubscriptionManualReviewAudit{
		ReviewID:                  review.ID,
		OrderID:                   order.ID,
		MerchantID:                merchant.ID,
		ActorID:                   17,
		ActorRole:                 "admin",
		EvidenceReference:         "SYNTHETIC-EXTERNAL-EVIDENCE-17",
		Reason:                    "合成审计原因：核对外部凭证引用",
		PlanName:                  "合成历史月付",
		AmountCents:               19900,
		DurationDays:              30,
		PaymentOccurredAttested:   true,
		OrderStatusBefore:         "pending",
		OrderStatusAfter:          "manually_verified",
		OrderUpdatedAtBefore:      reviewedAt.Add(-time.Minute),
		OrderUpdatedAtAfter:       reviewedAt,
		SubscriptionStatusBefore:  "inactive",
		SubscriptionStatusAfter:   "active",
		SubscriptionPlanBefore:    "none",
		SubscriptionPlanAfter:     "历史复核",
		SubscriptionExpireAtAfter: reviewedAt.Add(30 * 24 * time.Hour),
		ReviewedAt:                reviewedAt,
		CreatedAt:                 reviewedAt,
	}
	if err := db.Create(audit).Error; err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{JWTSecret: "synthetic-manual-review-read-secret", FrontendURL: "http://localhost:5173", RateLimitPerMinute: 1000}
	engine := gin.New()
	Register(engine, db, cfg)
	merchantID := merchant.ID
	merchantToken, err := utils.GenerateToken(cfg.JWTSecret, 21, "merchant_admin", "merchant_admin", &merchantID, 1)
	if err != nil {
		t.Fatal(err)
	}
	userToken, err := utils.GenerateToken(cfg.JWTSecret, 22, "user", "user", nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	adminToken := syntheticAdminToken(t, db, cfg, 7)
	path := "/api/v1/admin/subscription-manual-reviews"
	request := func(token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, req)
		return response
	}
	if response := request(""); response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status=%d, want 401", response.Code)
	}
	if response := request(merchantToken); response.Code != http.StatusForbidden {
		t.Fatalf("merchant status=%d, want 403", response.Code)
	}
	if response := request(userToken); response.Code != http.StatusForbidden {
		t.Fatalf("customer status=%d, want 403", response.Code)
	}

	var beforeOrder model.Order
	if err := db.First(&beforeOrder, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	response := request(adminToken)
	if response.Code != http.StatusOK {
		t.Fatalf("platform admin status=%d, want 200; body=%s", response.Code, response.Body.String())
	}
	var envelope struct {
		Data struct {
			List []dto.ManualSubscriptionReviewOrder `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode read-only report: %v; body=%s", err, response.Body.String())
	}
	if len(envelope.Data.List) != 1 {
		t.Fatalf("manual audit list length=%d, want 1; body=%s", len(envelope.Data.List), response.Body.String())
	}
	row := envelope.Data.List[0]
	if row.Status != "manually_verified" || row.OrderNo != order.OrderNo || row.Merchant == nil || row.Merchant.Name != merchant.Name {
		t.Fatalf("wrong order summary in report: %+v", row)
	}
	if row.ManualReview.ActorID != 17 || row.ManualReview.EvidenceReference != "SYNTHETIC-EXTERNAL-EVIDENCE-17" || !row.ManualReview.PaymentOccurredAttested {
		t.Fatalf("wrong source review data: %+v", row.ManualReview)
	}
	if row.ManualReviewAudit.ActorID != 17 || row.ManualReviewAudit.ActorRole != "admin" || row.ManualReviewAudit.EvidenceReference != "SYNTHETIC-EXTERNAL-EVIDENCE-17" || row.ManualReviewAudit.Reason != "合成审计原因：核对外部凭证引用" || !row.ManualReviewAudit.ReviewedAt.Equal(reviewedAt) {
		t.Fatalf("wrong immutable audit data: %+v", row.ManualReviewAudit)
	}
	if row.ManualReviewAudit.OrderStatusBefore != "pending" || row.ManualReviewAudit.OrderStatusAfter != "manually_verified" || row.ManualReviewAudit.SubscriptionStatusAfter != "active" {
		t.Fatalf("wrong before/after audit display fields: %+v", row.ManualReviewAudit)
	}
	for _, forbidden := range []string{"alipay_qr_code", "wechat_qr_code", "app_secret", "payment_config", "review_key"} {
		if bytes.Contains(response.Body.Bytes(), []byte(forbidden)) {
			t.Fatalf("report unexpectedly exposes %q: %s", forbidden, response.Body.String())
		}
	}
	if !strings.Contains(response.Body.String(), "SYNTHETIC-EXTERNAL-EVIDENCE-17") {
		t.Fatal("report does not show the synthetic evidence reference")
	}

	var afterOrder model.Order
	if err := db.First(&afterOrder, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	if afterOrder.Status != beforeOrder.Status || afterOrder.PaidAt != nil || afterOrder.TransactionNo != "" || afterOrder.UpdatedAt != beforeOrder.UpdatedAt {
		t.Fatalf("GET report mutated order: before=%+v after=%+v", beforeOrder, afterOrder)
	}
	var reviewCount, auditCount, paymentCount int64
	if err := db.Model(&model.MerchantSubscriptionManualReview{}).Where("order_id = ?", order.ID).Count(&reviewCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.MerchantSubscriptionManualReviewAudit{}).Where("order_id = ?", order.ID).Count(&auditCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.PaymentRecord{}).Where("order_id = ?", order.ID).Count(&paymentCount).Error; err != nil {
		t.Fatal(err)
	}
	if reviewCount != 1 || auditCount != 1 || paymentCount != 0 {
		t.Fatalf("GET changed immutable review/audit/provider payment counts=%d/%d/%d", reviewCount, auditCount, paymentCount)
	}
	var providerPaidRevenue int64
	if err := db.Model(&model.Order{}).Where("status = ? AND paid_at IS NOT NULL AND order_type = ?", "paid", "merchant_subscription").Select("COALESCE(SUM(total_amount),0)").Scan(&providerPaidRevenue).Error; err != nil {
		t.Fatal(err)
	}
	if providerPaidRevenue != 0 {
		t.Fatalf("manual-only fixture changed the existing paid+paid_at revenue total: %d", providerPaidRevenue)
	}

	summary, err := service.NewDashboardService(db).SummaryWithRange("2026-11-13", "2026-11-13")
	if err != nil {
		t.Fatalf("read dashboard summary: %v", err)
	}
	if got, ok := summary["platform_subscription_revenue"].(int64); !ok || got != 0 {
		t.Fatalf("dashboard provider-confirmed subscription revenue=%v (%T), want 0", summary["platform_subscription_revenue"], summary["platform_subscription_revenue"])
	}
}
