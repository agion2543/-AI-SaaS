package controller

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

	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/service"
)

func TestManualReviewControllerValidatesRequestAndReturnsSavedAttestation(t *testing.T) {
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
		t.Fatalf("migrate synthetic test schema: %v", err)
	}
	merchant := &model.Merchant{Name: "合成控制器复核商家", Status: "active", SubscriptionStatus: "inactive"}
	if err := db.Create(merchant).Error; err != nil {
		t.Fatal(err)
	}
	order := &model.Order{OrderNo: "SYNTHETIC-CONTROLLER-MANUAL-REVIEW", OrderType: "merchant_subscription", MerchantID: &merchant.ID, Amount: 12000, TotalAmount: 12000, Status: "pending"}
	if err := db.Create(order).Error; err != nil {
		t.Fatal(err)
	}
	ctl := NewOrderController(service.NewOrderService(db), nil, nil)
	engine := gin.New()
	engine.POST("/admin/orders/:id/review-historical-subscription", func(c *gin.Context) {
		c.Set("user_id", uint(33))
		c.Set("role", "admin")
		ctl.ManualReviewLegacyMerchantSubscription(c)
	})
	path := "/admin/orders/" + fmt.Sprint(order.ID) + "/review-historical-subscription"
	doRequest := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, req)
		return response
	}
	for _, body := range []string{
		`{"plan_name":"原计划","amount_cents":12000,"duration_days":30,"evidence_reference":"synthetic-ref","reason":"synthetic reason"}`,
		`{"plan_name":"原计划","amount_cents":12000,"duration_days":30,"payment_occurred_attested":false,"evidence_reference":"synthetic-ref","reason":"synthetic reason"}`,
		`{broken-json`,
	} {
		if response := doRequest(body); response.Code != http.StatusBadRequest {
			t.Fatalf("invalid request status=%d, want 400; body=%s", response.Code, response.Body.String())
		}
	}
	var unchanged model.Order
	if err := db.First(&unchanged, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	if unchanged.Status != "pending" || unchanged.PaidAt != nil {
		t.Fatalf("invalid controller request changed order: %+v", unchanged)
	}

	response := doRequest(`{"plan_name":"原计划","amount_cents":12000,"duration_days":30,"payment_occurred_attested":true,"evidence_reference":"synthetic-ref-001","reason":"合成控制器测试：管理员核对原始证据并声明付款发生"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("valid manual verification status=%d, want 200; body=%s", response.Code, response.Body.String())
	}
	var stored model.Order
	if err := db.First(&stored, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != "manually_verified" || stored.PaidAt != nil || stored.TransactionNo != "" {
		t.Fatalf("controller result must be manually verified without provider markers: %+v", stored)
	}
	var reviews, audits, payments int64
	if err := db.Model(&model.MerchantSubscriptionManualReview{}).Where("order_id = ?", order.ID).Count(&reviews).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.MerchantSubscriptionManualReviewAudit{}).Where("order_id = ?", order.ID).Count(&audits).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.PaymentRecord{}).Where("order_id = ?", order.ID).Count(&payments).Error; err != nil {
		t.Fatal(err)
	}
	if reviews != 1 || audits != 1 || payments != 0 {
		t.Fatalf("saved review/audit/provider payment counts=%d/%d/%d", reviews, audits, payments)
	}
	if !bytes.Contains(response.Body.Bytes(), []byte(`"external_payment_attested":true`)) || !bytes.Contains(response.Body.Bytes(), []byte(`"provider_payment_confirmed":false`)) || !bytes.Contains(response.Body.Bytes(), []byte(`"payment_occurred_attested":true`)) {
		t.Fatalf("response does not expose saved attestation and non-provider status: %s", response.Body.String())
	}
}
