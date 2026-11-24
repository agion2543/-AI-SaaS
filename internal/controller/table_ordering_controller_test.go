package controller

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/service"
)

func TestTableCheckoutReturnsApprovedMerchantQRCodeWithoutGatewayPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open synthetic table checkout database: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Merchant{}, &model.MerchantPlan{}, &model.Store{}, &model.StoreProduct{},
		&model.User{}, &model.MembershipPackage{}, &model.Promotion{}, &model.Order{},
		&model.PaymentRecord{}, &model.RefundRecord{}, &model.MerchantSettlement{},
		&model.MerchantSubscriptionManualReview{}, &model.MerchantSubscriptionManualReviewAudit{},
		&model.MerchantPaymentConfig{}, &model.ReferralCoupon{}, &model.StoreTableConfig{},
		&model.StoreTable{}, &model.TableSession{}, &model.TableCartItem{}, &model.TableOrderDetail{},
	); err != nil {
		t.Fatalf("migrate synthetic table checkout schema: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get synthetic SQL database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	merchant := model.Merchant{Name: "合成桌位收款商家", Status: "active"}
	if err := db.Create(&merchant).Error; err != nil {
		t.Fatalf("create synthetic merchant: %v", err)
	}
	store := model.Store{MerchantID: merchant.ID, Name: "合成桌位收款门店", Status: "active", IsOpen: true, OrderMode: "pay_first"}
	if err := db.Create(&store).Error; err != nil {
		t.Fatalf("create synthetic store: %v", err)
	}
	product := model.StoreProduct{StoreID: store.ID, Name: "合成餐品", Price: 1234, Status: "active"}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create synthetic restaurant product: %v", err)
	}
	const merchantQRCode = "https://merchant.example.invalid/synthetic-approved-qr.png"
	if err := db.Create(&model.MerchantPaymentConfig{
		MerchantID: merchant.ID, Channel: "alipay", Mode: "direct", Status: "enabled", AuditStatus: "approved",
		AccountName: "合成商家收款账户", AlipayQRCode: merchantQRCode,
	}).Error; err != nil {
		t.Fatalf("create approved synthetic merchant QR config: %v", err)
	}

	tableService := service.NewTableOrderingService(db, service.NewOrderService(db))
	tableConfig, err := tableService.ConfigureStoreTables(merchant.ID, store.ID, 1, []dto.ConfigureStoreTableLabelItem{{TableNo: 1, TableLabel: "A01"}}, func(_ int, code string) string { return code })
	if err != nil || len(tableConfig.Tables) != 1 {
		t.Fatalf("configure synthetic table QR: config=%+v err=%v", tableConfig, err)
	}
	tableCode := tableConfig.Tables[0].URL
	session, err := tableService.JoinTableSession(dto.JoinTableSessionRequest{StoreID: store.ID, TableNo: 1, TableCode: tableCode})
	if err != nil {
		t.Fatalf("join synthetic table session: %v", err)
	}
	if _, err := tableService.UpdateCartItem(session.ID, tableCode, dto.UpdateTableCartItemRequest{ProductID: product.ID, Action: "add", Quantity: 1}); err != nil {
		t.Fatalf("add synthetic item to shared cart: %v", err)
	}

	requestKey, accessToken := strings.Repeat("a", 64), strings.Repeat("b", 64)
	body, err := json.Marshal(dto.TableCheckoutRequest{
		RequestKey: requestKey, AccessToken: accessToken, PaymentChannel: "alipay", PayMode: "page",
	})
	if err != nil {
		t.Fatalf("marshal checkout request: %v", err)
	}
	controller := NewTableOrderingController(tableService)
	responseRecorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(responseRecorder)
	ctx.Request = httptest.NewRequest("POST", "/customer/table-sessions/checkout", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Request.Header.Set("X-Table-Code", tableCode)
	ctx.Params = gin.Params{{Key: "sessionId", Value: "1"}}
	controller.Checkout(ctx)
	if responseRecorder.Code != 200 {
		t.Fatalf("table checkout status=%d body=%s", responseRecorder.Code, responseRecorder.Body.String())
	}

	var envelope struct {
		Code int `json:"code"`
		Data struct {
			Order struct {
				Status         string `json:"status"`
				PaymentChannel string `json:"payment_channel"`
				TotalAmount    int64  `json:"total_amount"`
				TableNo        int    `json:"table_no"`
				TableLabel     string `json:"table_label"`
			} `json:"order"`
			PaymentMethod                string         `json:"payment_method"`
			PaymentRequired              bool           `json:"payment_required"`
			ManualReconciliationRequired bool           `json:"manual_reconciliation_required"`
			PaymentConfig                map[string]any `json:"payment_config"`
			Payment                      map[string]any `json:"payment"`
		} `json:"data"`
	}
	if err := json.Unmarshal(responseRecorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode table checkout response: %v; body=%s", err, responseRecorder.Body.String())
	}
	if envelope.Code != 0 || envelope.Data.PaymentMethod != "merchant_qr" || !envelope.Data.PaymentRequired || !envelope.Data.ManualReconciliationRequired {
		t.Fatalf("table checkout must declare a pending merchant-QR/manual-review path: %+v", envelope)
	}
	if envelope.Data.Order.Status != "pending" || envelope.Data.Order.PaymentChannel != "merchant_qr" || envelope.Data.Order.TotalAmount != product.Price || envelope.Data.Order.TableNo != 1 || envelope.Data.Order.TableLabel != "A01" {
		t.Fatalf("table order must remain unpaid on the merchant QR channel: %+v", envelope.Data.Order)
	}
	if got := envelope.Data.PaymentConfig["alipay_qr_code"]; got != merchantQRCode {
		t.Fatalf("response must contain the approved merchant-owned QR, got %#v", got)
	}
	if envelope.Data.Payment != nil {
		t.Fatalf("table checkout must not return an application-level gateway payload: %#v", envelope.Data.Payment)
	}
}
