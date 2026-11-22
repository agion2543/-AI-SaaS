package router

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
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

func TestMerchantCannotReadOrWriteAnotherMerchantsResources(t *testing.T) {
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
		&model.Merchant{}, &model.User{}, &model.Store{}, &model.StoreProduct{}, &model.MerchantPlan{}, &model.CustomerLead{}, &model.AuditLog{},
		&model.MembershipPackage{}, &model.Order{}, &model.MerchantSettlement{}, &model.PaymentRecord{}, &model.RefundRecord{},
		&model.Promotion{}, &model.MerchantPaymentConfig{}, &model.MerchantGroupBuyLink{},
		&model.MerchantGroupBuyLinkAudit{}, &model.ShareCampaign{}, &model.ReferralCoupon{},
	); err != nil {
		t.Fatalf("migrate synthetic route schema: %v", err)
	}

	expiresAt := time.Now().Add(24 * time.Hour)
	merchantA := model.Merchant{Name: "合成商家 A", ContactPhone: "15500009999", ContactEmail: "private-merchant@example.invalid", Status: "active", SubscriptionStatus: "active", SubscriptionExpireAt: &expiresAt}
	merchantB := model.Merchant{Name: "合成商家 B", Status: "active", SubscriptionStatus: "active", SubscriptionExpireAt: &expiresAt}
	for _, merchant := range []*model.Merchant{&merchantA, &merchantB} {
		if err := db.Create(merchant).Error; err != nil {
			t.Fatalf("create synthetic merchant: %v", err)
		}
	}
	merchantAID := merchantA.ID
	userA := model.User{UUID: uuid.NewString(), Email: "merchant-a@example.invalid", Phone: "13800138001", MerchantID: &merchantAID, Role: "merchant_admin", Status: "active"}
	if err := db.Create(&userA).Error; err != nil {
		t.Fatalf("create synthetic merchant A user: %v", err)
	}

	storeA := model.Store{MerchantID: merchantA.ID, Name: "A 自有门店", Address: "private store address", ContactPhone: "16600009998", Status: "active", IsOpen: true}
	storeB := model.Store{MerchantID: merchantB.ID, Name: "B 私有门店", Status: "active", IsOpen: true}
	for _, store := range []*model.Store{&storeA, &storeB} {
		if err := db.Create(store).Error; err != nil {
			t.Fatalf("create synthetic store: %v", err)
		}
	}
	productA := model.StoreProduct{StoreID: storeA.ID, Name: "A 自有商品", Price: 100, Status: "active"}
	productB := model.StoreProduct{StoreID: storeB.ID, Name: "B 私有商品", Price: 200, Status: "active"}
	for _, product := range []*model.StoreProduct{&productA, &productB} {
		if err := db.Create(product).Error; err != nil {
			t.Fatalf("create synthetic product: %v", err)
		}
	}

	paidAt := time.Now()
	orderFor := func(merchantID, storeID uint, orderNo string) model.Order {
		return model.Order{OrderNo: orderNo, OrderType: "store_order", MerchantID: &merchantID, StoreID: &storeID, Amount: 100, TotalAmount: 100, Status: "received"}
	}
	orderA := orderFor(merchantA.ID, storeA.ID, "SYNTHETIC-MERCHANT-A-ORDER")
	orderA.CustomerPhone = "16600001111"
	orderA.TransactionNo = "synthetic-private-transaction"
	orderA.PaymentChannel = "alipay"
	orderA.PaidAt = &paidAt
	orderA.Status = "completed"
	orderA.CustomerNote = "synthetic customer note"
	orderA.MerchantNote = "synthetic internal note"
	orderA.Items = `[{"product_id":1,"name":"合成商品","price":100,"quantity":1,"line_amount":100,"description":"synthetic item description","image_url":"/synthetic.png"}]`
	orderA.OperationLogs = `[{"action":"accepted","text":"synthetic operation log","time":"2026-11-15T00:00:00Z"}]`
	orderB := orderFor(merchantB.ID, storeB.ID, "SYNTHETIC-MERCHANT-B-ORDER")
	orderConfirm := orderFor(merchantA.ID, storeA.ID, "SYNTHETIC-MERCHANT-CONFIRM-ORDER")
	orderConfirm.CustomerPhone = "17700001234"
	orderConfirm.Status = "payment_confirming"
	orderConfirm.Items = `[{"product_id":1,"name":"合成商品","price":100,"quantity":1,"line_amount":100}]`
	for _, order := range []*model.Order{&orderA, &orderB, &orderConfirm} {
		if err := db.Create(order).Error; err != nil {
			t.Fatalf("create synthetic order: %v", err)
		}
	}
	refundRecord := model.RefundRecord{
		OrderID: orderA.ID, MerchantID: &merchantAID, StoreID: &storeA.ID,
		RefundNo: "SYNTHETIC-REFUND-001", TransactionNo: "synthetic-private-refund-transaction",
		Amount: 25, Reason: "synthetic refund reason", OperatorRole: "merchant", OperatorID: userA.ID,
		Status: "success", RawPayload: `{"mode":"manual_record","private_payload":"synthetic-private-payload"}`,
	}
	if err := db.Create(&refundRecord).Error; err != nil {
		t.Fatalf("create synthetic refund record: %v", err)
	}

	lead := model.CustomerLead{MerchantID: merchantA.ID, StoreID: storeA.ID, CustomerName: "合成顾客", CustomerPhone: "15500001111", Message: "synthetic customer note", FollowUpNote: "synthetic follow-up note", Source: "store_qr", Status: "new"}
	if err := db.Create(&lead).Error; err != nil {
		t.Fatalf("create synthetic customer lead: %v", err)
	}

	validTo := time.Now().Add(24 * time.Hour)
	couponA := model.ReferralCoupon{MerchantID: merchantA.ID, StoreID: storeA.ID, CouponNo: "SYNTHETIC-A-COUPON", OwnerPhone: "13800138001", OwnerType: "referrer", Title: "A 自有优惠券", Status: "unused", ValidTo: &validTo}
	couponB := model.ReferralCoupon{MerchantID: merchantB.ID, StoreID: storeB.ID, CouponNo: "SYNTHETIC-B-PRIVATE-COUPON", OwnerPhone: "13800138002", OwnerType: "referrer", Title: "B 私有优惠券", Status: "unused", ValidTo: &validTo}
	for _, coupon := range []*model.ReferralCoupon{&couponA, &couponB} {
		if err := db.Create(coupon).Error; err != nil {
			t.Fatalf("create synthetic coupon: %v", err)
		}
	}

	cfg := &config.Config{JWTSecret: "test-only-secret", FrontendURL: "http://localhost:5173", RateLimitPerMinute: 100}
	engine := gin.New()
	Register(engine, db, cfg)
	token, err := utils.GenerateUserToken(cfg.JWTSecret, userA, 1)
	if err != nil {
		t.Fatalf("create synthetic merchant A token: %v", err)
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

	groupBuyFixtures := []model.MerchantGroupBuyLink{
		{MerchantID: merchantA.ID, StoreID: storeA.ID, Channel: "douyin", DestinationURL: "https://life.douyin.com/synthetic/deal", Code: "11111111111111111111111111111111", Enabled: true, ReviewStatus: "approved"},
		{MerchantID: merchantA.ID, StoreID: storeA.ID, Channel: "meituan", DestinationURL: "https://i.meituan.com/synthetic/deal", Code: "22222222222222222222222222222222", Enabled: true, ReviewStatus: "pending"},
		{MerchantID: merchantA.ID, StoreID: storeA.ID, Channel: "paused", DestinationURL: "https://life.douyin.com/synthetic/paused", Code: "33333333333333333333333333333333", Enabled: true, ReviewStatus: "approved", AdminSuspended: true},
		{MerchantID: merchantA.ID, StoreID: storeA.ID, Channel: "disabled", DestinationURL: "https://life.douyin.com/synthetic/disabled", Code: "44444444444444444444444444444444", Enabled: false, ReviewStatus: "approved"},
		{MerchantID: merchantB.ID, StoreID: storeB.ID, Channel: "douyin", DestinationURL: "https://life.douyin.com/synthetic/other-store", Code: "55555555555555555555555555555555", Enabled: true, ReviewStatus: "approved"},
	}
	for _, fixture := range groupBuyFixtures {
		if err := db.Create(&fixture).Error; err != nil {
			t.Fatalf("create synthetic group-buy fixture: %v", err)
		}
	}

	publicStorePath := "/api/v1/customer/stores/" + strconv.FormatUint(uint64(storeA.ID), 10)
	publicStoreResponse := request(http.MethodGet, publicStorePath, "")
	if publicStoreResponse.Code != http.StatusOK || !strings.Contains(publicStoreResponse.Body.String(), merchantA.ContactPhone) || strings.Contains(publicStoreResponse.Body.String(), merchantA.ContactEmail) {
		t.Errorf("public store must keep merchant phone and omit email: status=%d body=%s", publicStoreResponse.Code, publicStoreResponse.Body.String())
	}
	if body := publicStoreResponse.Body.String(); !strings.Contains(body, `"group_buy_links":[{"channel":"douyin","code":"11111111111111111111111111111111"}]`) || strings.Contains(body, "22222222222222222222222222222222") || strings.Contains(body, "33333333333333333333333333333333") || strings.Contains(body, "44444444444444444444444444444444") || strings.Contains(body, "55555555555555555555555555555555") || strings.Contains(body, "destination_url") || strings.Contains(body, "click_count") {
		t.Errorf("public store should expose only the approved active same-store redirect code: %s", body)
	}
	publicLeadPath := "/api/v1/customer/stores/" + strconv.FormatUint(uint64(storeA.ID), 10) + "/leads"
	publicLeadBody := `{"customer_name":"合成提交人","customer_phone":"17700000000","message":"synthetic public note"}`
	if response := request(http.MethodPost, publicLeadPath, publicLeadBody); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"submitted":true`) || strings.Contains(response.Body.String(), "17700000000") || strings.Contains(response.Body.String(), "synthetic public note") || strings.Contains(response.Body.String(), "合成提交人") {
		t.Errorf("public lead submission should return success only: status=%d body=%s", response.Code, response.Body.String())
	}
	storeAPath := "/api/v1/merchant/stores/" + strconv.FormatUint(uint64(storeA.ID), 10) + "/products"
	if response := request(http.MethodGet, storeAPath, ""); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "A 自有商品") {
		t.Fatalf("merchant A cannot read own store products: status=%d body=%s", response.Code, response.Body.String())
	}
	storeBProductsPath := "/api/v1/merchant/stores/" + strconv.FormatUint(uint64(storeB.ID), 10) + "/products"
	if response := request(http.MethodGet, storeBProductsPath, ""); response.Code != http.StatusBadRequest || strings.Contains(response.Body.String(), "B 私有商品") {
		t.Errorf("merchant A read another merchant's store products: status=%d body=%s", response.Code, response.Body.String())
	}
	storeBPath := "/api/v1/merchant/stores/" + strconv.FormatUint(uint64(storeB.ID), 10)
	if response := request(http.MethodPut, storeBPath, `{"name":"被 A 篡改","contact_phone":"13800138003"}`); response.Code != http.StatusBadRequest {
		t.Errorf("merchant A update another merchant's store status=%d, want %d; body=%s", response.Code, http.StatusBadRequest, response.Body.String())
	}
	var storedStoreB model.Store
	if err := db.First(&storedStoreB, storeB.ID).Error; err != nil {
		t.Fatalf("reload merchant B store: %v", err)
	}
	if storedStoreB.Name != "B 私有门店" {
		t.Errorf("merchant B store was changed by merchant A: name=%q", storedStoreB.Name)
	}

	orderAPath := "/api/v1/merchant/orders/" + strconv.FormatUint(uint64(orderA.ID), 10)
	if response := request(http.MethodGet, orderAPath, ""); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"customer_phone":"16600001111"`) || strings.Contains(response.Body.String(), orderA.TransactionNo) || strings.Contains(response.Body.String(), storeA.ContactPhone) || !strings.Contains(response.Body.String(), `"customer_note":"synthetic customer note"`) || !strings.Contains(response.Body.String(), `"merchant_note":"synthetic internal note"`) || !strings.Contains(response.Body.String(), `"operation_logs"`) || !strings.Contains(response.Body.String(), `"can_refund":true`) {
		t.Fatalf("merchant A must retain full phone on completed order detail without raw transaction ID: status=%d body=%s", response.Code, response.Body.String())
	}
	var orderAccess model.AuditLog
	if err := db.Where("action = ? AND target_id = ?", "merchant_order_detail_view", orderA.ID).First(&orderAccess).Error; err != nil {
		t.Fatalf("load order-detail access audit: %v", err)
	}
	if orderAccess.ActorID != userA.ID || orderAccess.ActorType != userA.Role || orderAccess.ActorName != "" || orderAccess.TargetName != "" || orderAccess.IP != "" || orderAccess.UserAgent != "" || !strings.Contains(orderAccess.Detail, "履约与售后查询") || strings.Contains(orderAccess.Detail, orderA.CustomerPhone) {
		t.Errorf("order access audit must contain only actor/object/action/time/purpose: %+v", orderAccess)
	}
	if response := request(http.MethodGet, "/api/v1/merchant/orders?page=1&page_size=20", ""); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"customer_phone":"166****1111"`) || strings.Contains(response.Body.String(), orderA.CustomerPhone) || strings.Contains(response.Body.String(), orderA.TransactionNo) || strings.Contains(response.Body.String(), storeA.ContactPhone) || strings.Contains(response.Body.String(), "synthetic customer note") || strings.Contains(response.Body.String(), "synthetic internal note") || strings.Contains(response.Body.String(), "synthetic operation log") || strings.Contains(response.Body.String(), `"transaction_no"`) || strings.Contains(response.Body.String(), `"user"`) || strings.Contains(response.Body.String(), `"merchant"`) || strings.Contains(response.Body.String(), `"contact_phone"`) || !strings.Contains(response.Body.String(), `"has_customer_note":true`) || !strings.Contains(response.Body.String(), `"has_merchant_note":true`) || !strings.Contains(response.Body.String(), `"can_refund":true`) {
		t.Errorf("merchant order list must return only masked summary fields: status=%d body=%s", response.Code, response.Body.String())
	}

	noteResponse := request(http.MethodPut, orderAPath+"/note", `{"merchant_note":"synthetic updated internal note"}`)
	if noteResponse.Code != http.StatusOK || !strings.Contains(noteResponse.Body.String(), `"customer_phone":"166****1111"`) || strings.Contains(noteResponse.Body.String(), orderA.CustomerPhone) || strings.Contains(noteResponse.Body.String(), "synthetic updated internal note") || strings.Contains(noteResponse.Body.String(), `"customer_note"`) || strings.Contains(noteResponse.Body.String(), `"merchant_note"`) || strings.Contains(noteResponse.Body.String(), `"operation_logs"`) || strings.Contains(noteResponse.Body.String(), `"transaction_no"`) {
		t.Errorf("merchant note update response must use masked summary: status=%d body=%s", noteResponse.Code, noteResponse.Body.String())
	}

	confirmPath := "/api/v1/merchant/orders/" + strconv.FormatUint(uint64(orderConfirm.ID), 10) + "/confirm-payment"
	confirmResponse := request(http.MethodPut, confirmPath, "")
	if confirmResponse.Code != http.StatusOK || !strings.Contains(confirmResponse.Body.String(), `"customer_phone":"177****1234"`) || strings.Contains(confirmResponse.Body.String(), orderConfirm.CustomerPhone) || strings.Contains(confirmResponse.Body.String(), `"transaction_no"`) || strings.Contains(confirmResponse.Body.String(), `"operation_logs"`) || strings.Contains(confirmResponse.Body.String(), `"customer_note"`) || strings.Contains(confirmResponse.Body.String(), `"merchant_note"`) {
		t.Errorf("merchant payment confirmation response must use masked summary: status=%d body=%s", confirmResponse.Code, confirmResponse.Body.String())
	}

	refundResponse := request(http.MethodGet, "/api/v1/merchant/refunds", "")
	if refundResponse.Code != http.StatusOK || !strings.Contains(refundResponse.Body.String(), refundRecord.RefundNo) || !strings.Contains(refundResponse.Body.String(), `"manual_record":true`) || !strings.Contains(refundResponse.Body.String(), refundRecord.Reason) || strings.Contains(refundResponse.Body.String(), orderA.CustomerPhone) || strings.Contains(refundResponse.Body.String(), orderA.TransactionNo) || strings.Contains(refundResponse.Body.String(), refundRecord.TransactionNo) || strings.Contains(refundResponse.Body.String(), "synthetic-private-payload") || strings.Contains(refundResponse.Body.String(), `"raw_payload"`) || strings.Contains(refundResponse.Body.String(), `"operator_id"`) {
		t.Errorf("merchant refund list must return whitelisted summary fields only: status=%d body=%s", refundResponse.Code, refundResponse.Body.String())
	}
	orderBPath := "/api/v1/merchant/orders/" + strconv.FormatUint(uint64(orderB.ID), 10)
	if response := request(http.MethodGet, orderBPath, ""); response.Code != http.StatusNotFound {
		t.Errorf("merchant A read another merchant's order status=%d, want %d; body=%s", response.Code, http.StatusNotFound, response.Body.String())
	}
	orderBNotePath := orderBPath + "/note"
	if response := request(http.MethodPut, orderBNotePath, `{"merchant_note":"被 A 篡改"}`); response.Code != http.StatusBadRequest {
		t.Errorf("merchant A update another merchant's order status=%d, want %d; body=%s", response.Code, http.StatusBadRequest, response.Body.String())
	}
	var storedOrderB model.Order
	if err := db.First(&storedOrderB, orderB.ID).Error; err != nil {
		t.Fatalf("reload merchant B order: %v", err)
	}
	if storedOrderB.MerchantNote != "" {
		t.Errorf("merchant B order was changed by merchant A: note=%q", storedOrderB.MerchantNote)
	}

	if response := request(http.MethodGet, "/api/v1/merchant/leads?page=1&page_size=5", ""); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), lead.CustomerPhone) || !strings.Contains(response.Body.String(), lead.CustomerName) || !strings.Contains(response.Body.String(), lead.Status) || strings.Contains(response.Body.String(), lead.Message) || strings.Contains(response.Body.String(), lead.FollowUpNote) || strings.Contains(response.Body.String(), "private-merchant@example.invalid") {
		t.Errorf("merchant lead summary must keep visible fields only: status=%d body=%s", response.Code, response.Body.String())
	}
	var leadAccess model.AuditLog
	if err := db.Where("action = ?", "merchant_customer_lead_list_view").First(&leadAccess).Error; err != nil {
		t.Fatalf("load lead-list access audit: %v", err)
	}
	if strings.Contains(leadAccess.Detail, lead.CustomerPhone) || strings.Contains(leadAccess.Detail, lead.Message) || !strings.Contains(leadAccess.Detail, "客户跟进") {
		t.Errorf("lead-list audit must contain purpose only, got %s", leadAccess.Detail)
	}
	if response := request(http.MethodGet, "/api/v1/merchant/coupons", ""); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), couponA.CouponNo) || strings.Contains(response.Body.String(), couponB.CouponNo) {
		t.Errorf("merchant coupon list is not scoped to the current merchant: status=%d body=%s", response.Code, response.Body.String())
	}
	couponBVoidPath := "/api/v1/merchant/coupons/" + strconv.FormatUint(uint64(couponB.ID), 10) + "/void"
	if response := request(http.MethodPut, couponBVoidPath, `{"remark":"被 A 篡改"}`); response.Code != http.StatusBadRequest {
		t.Errorf("merchant A void another merchant's coupon status=%d, want %d; body=%s", response.Code, http.StatusBadRequest, response.Body.String())
	}
	var storedCouponB model.ReferralCoupon
	if err := db.First(&storedCouponB, couponB.ID).Error; err != nil {
		t.Fatalf("reload merchant B coupon: %v", err)
	}
	if storedCouponB.Status != "unused" || strings.Contains(storedCouponB.Remark, "被 A 篡改") {
		t.Errorf("merchant B coupon was changed by merchant A: status=%q remark=%q", storedCouponB.Status, storedCouponB.Remark)
	}
}
