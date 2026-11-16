package router

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/utils"
)

func TestRefundReconciliationRoutesAreAdminOnlyAndCannotBeMutatedByTenantOrCustomer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if err := db.AutoMigrate(&model.Merchant{}, &model.Store{}, &model.Order{}, &model.RefundRecord{}, &model.RefundReconciliationAudit{}); err != nil {
		t.Fatal(err)
	}
	seedSyntheticSuperAdmin(t, db, 7, "synthetic-refund-super")
	merchant := model.Merchant{Name: "合成对账商户", Status: "active"}
	if err := db.Create(&merchant).Error; err != nil {
		t.Fatal(err)
	}
	store := model.Store{MerchantID: merchant.ID, Name: "合成对账门店", Status: "active"}
	if err := db.Create(&store).Error; err != nil {
		t.Fatal(err)
	}
	paidAt := time.Now().UTC()
	order := model.Order{OrderNo: "SYNTHETIC-AUTH-ORDER", OrderType: "store_order", MerchantID: &merchant.ID, StoreID: &store.ID, TotalAmount: 6000, Status: "accepted", PaidAt: &paidAt, RefundStatus: "processing"}
	if err := db.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	key := "synthetic-auth-idempotency-key"
	refund := model.RefundRecord{OrderID: order.ID, MerchantID: &merchant.ID, StoreID: &store.ID, RefundNo: "SYNTHETIC-AUTH-REFUND", IdempotencyKey: &key, Amount: 2000, Reason: "合成鉴权测试", Status: "unknown", RawPayload: "synthetic provider timeout"}
	if err := db.Create(&refund).Error; err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{JWTSecret: "refund-reconciliation-test-secret", FrontendURL: "http://localhost:5173", RateLimitPerMinute: 1000}
	engine := gin.New()
	Register(engine, db, cfg)
	merchantID := merchant.ID
	merchantToken, err := utils.GenerateToken(cfg.JWTSecret, 202, "merchant_admin", "merchant_admin", &merchantID, 1)
	if err != nil {
		t.Fatal(err)
	}
	userToken, err := utils.GenerateToken(cfg.JWTSecret, 303, "user", "user", nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	adminToken := syntheticAdminToken(t, db, cfg, 7)
	path := "/api/v1/admin/refund-reconciliations"
	resolvePath := path + "/" + uintString(refund.ID) + "/resolve"
	request := func(method, requestPath, token, body, customerAccessToken string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, requestPath, bytes.NewBufferString(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if customerAccessToken != "" {
			req.Header.Set("X-Order-Access-Token", customerAccessToken)
		}
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, req)
		return response
	}

	if response := request(http.MethodGet, path, "", "", ""); response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated reconciliation list status=%d, want 401", response.Code)
	}
	if response := request(http.MethodGet, path, merchantToken, "", ""); response.Code != http.StatusForbidden {
		t.Fatalf("merchant must not read platform reconciliation queue: status=%d body=%s", response.Code, response.Body.String())
	}
	if response := request(http.MethodPost, resolvePath, merchantToken, `{"outcome":"success","provider_reference":"SYNTHETIC-REF","reason":"synthetic evidence"}`, ""); response.Code != http.StatusForbidden {
		t.Fatalf("merchant must not resolve platform refund: status=%d body=%s", response.Code, response.Body.String())
	}
	if response := request(http.MethodPost, resolvePath, userToken, `{"outcome":"success","provider_reference":"SYNTHETIC-REF","reason":"synthetic evidence"}`, ""); response.Code != http.StatusForbidden {
		t.Fatalf("customer account must not resolve platform refund: status=%d body=%s", response.Code, response.Body.String())
	}
	if response := request(http.MethodPost, resolvePath, "", `{"outcome":"success","provider_reference":"SYNTHETIC-REF","reason":"synthetic evidence"}`, "synthetic-customer-order-access-token"); response.Code != http.StatusUnauthorized {
		t.Fatalf("customer order access token must not authorize platform reconciliation: status=%d", response.Code)
	}
	if response := request(http.MethodPost, resolvePath, adminToken, `{"outcome":"success","reason":"evidence but no provider reference"}`, ""); response.Code != http.StatusBadRequest {
		t.Fatalf("admin request without external provider reference must be rejected: status=%d body=%s", response.Code, response.Body.String())
	}
	if response := request(http.MethodPost, resolvePath, adminToken, `{"outcome":"success","provider_reference":"SYNTHETIC-REF"}`, ""); response.Code != http.StatusBadRequest {
		t.Fatalf("admin request without reconciliation note must be rejected: status=%d body=%s", response.Code, response.Body.String())
	}
	if response := request(http.MethodGet, path, adminToken, "", ""); response.Code != http.StatusOK {
		t.Fatalf("admin should read the unknown reservation queue: status=%d body=%s", response.Code, response.Body.String())
	}

	var unchanged model.RefundRecord
	if err := db.First(&unchanged, refund.ID).Error; err != nil {
		t.Fatal(err)
	}
	var orderBeforeAdmin model.Order
	if err := db.First(&orderBeforeAdmin, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	var auditCount int64
	if err := db.Model(&model.RefundReconciliationAudit{}).Where("refund_id = ?", refund.ID).Count(&auditCount).Error; err != nil {
		t.Fatal(err)
	}
	if unchanged.Status != "unknown" || orderBeforeAdmin.RefundedAmount != 0 || auditCount != 0 {
		t.Fatalf("unauthorized or incomplete attempts changed financial state: refund=%q refunded=%d audits=%d", unchanged.Status, orderBeforeAdmin.RefundedAmount, auditCount)
	}

	response := request(http.MethodPost, resolvePath, adminToken, `{"outcome":"success","provider_reference":"SYNTHETIC-CHANNEL-REF","reason":"合成数据：已依据外部渠道记录确认"}`, "")
	if response.Code != http.StatusOK {
		t.Fatalf("authorized platform admin should resolve with evidence and reason: status=%d body=%s", response.Code, response.Body.String())
	}
	if err := db.First(&unchanged, refund.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.RefundReconciliationAudit{}).Where("refund_id = ?", refund.ID).Count(&auditCount).Error; err != nil {
		t.Fatal(err)
	}
	if unchanged.Status != "success" || auditCount != 1 {
		t.Fatalf("authorized admin outcome and audit not persisted: status=%q audits=%d", unchanged.Status, auditCount)
	}

	manualKey := "synthetic-auth-manual-review-key"
	staleRefund := model.RefundRecord{OrderID: order.ID, MerchantID: &merchant.ID, StoreID: &store.ID, RefundNo: "SYNTHETIC-AUTH-STALE-REFUND", IdempotencyKey: &manualKey, Amount: 1000, Reason: "合成逾期鉴权测试", Status: "processing"}
	if err := db.Create(&staleRefund).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE refund_records SET updated_at = ? WHERE id = ?", time.Now().UTC().Add(-20*time.Minute), staleRefund.ID).Error; err != nil {
		t.Fatal(err)
	}
	manualPath := path + "/" + uintString(staleRefund.ID) + "/manual-review"
	if response := request(http.MethodPost, manualPath, merchantToken, `{"reason":"合成原因"}`, ""); response.Code != http.StatusForbidden {
		t.Fatalf("merchant must not move a refund to manual review: status=%d body=%s", response.Code, response.Body.String())
	}
	if response := request(http.MethodPost, manualPath, userToken, `{"reason":"合成原因"}`, ""); response.Code != http.StatusForbidden {
		t.Fatalf("customer must not move a refund to manual review: status=%d body=%s", response.Code, response.Body.String())
	}
	if response := request(http.MethodPost, manualPath, adminToken, `{}`, ""); response.Code != http.StatusBadRequest {
		t.Fatalf("operator note is required for the status transition: status=%d body=%s", response.Code, response.Body.String())
	}
	response = request(http.MethodPost, manualPath, adminToken, `{"reason":"合成记录：本地预留已逾期，转入人工核查"}`, "")
	if response.Code != http.StatusOK {
		t.Fatalf("platform admin should move only the stale reservation into manual review: status=%d body=%s", response.Code, response.Body.String())
	}
	if err := db.First(&staleRefund, staleRefund.ID).Error; err != nil {
		t.Fatal(err)
	}
	if staleRefund.Status != "unknown" {
		t.Fatalf("authorized stale reservation should move to unknown, got %q", staleRefund.Status)
	}
	if err := db.Model(&model.RefundReconciliationAudit{}).Where("refund_id = ? AND old_status = ? AND new_status = ?", staleRefund.ID, "processing", "unknown").Count(&auditCount).Error; err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("platform admin manual-review transition must create one audit record, count=%d", auditCount)
	}
}
