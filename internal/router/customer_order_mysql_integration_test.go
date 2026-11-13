package router

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	mysqlDriver "github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/service"
)

const (
	customerOrderITDSNEnv  = "MYSQL_CUSTOMER_ORDER_IT_DSN"
	customerOrderITRootEnv = "MYSQL_CUSTOMER_ORDER_IT_ROOT"
)

type customerOrderAPIEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// TestMySQLMobileCustomerOrderAPIContract runs against the real Gin routes and
// the real MySQL-backed order service. The companion shell script owns a fresh,
// socket-only temporary MySQL 8 server; this test refuses any other target.
func TestMySQLMobileCustomerOrderAPIContract(t *testing.T) {
	db := openSafeCustomerOrderMySQL(t)
	if err := db.AutoMigrate(
		&model.Merchant{},
		&model.MerchantPlan{},
		&model.MerchantPaymentConfig{},
		&model.Store{},
		&model.StoreProduct{},
		&model.ShareActivityConfig{},
		&model.ShareCampaign{},
		&model.ReferralCoupon{},
		&model.Promotion{},
		&model.User{},
		&model.MembershipPackage{},
		&model.Order{},
		&model.PaymentRecord{},
	); err != nil {
		t.Fatalf("migrate disposable customer-order schema: %v", err)
	}

	phone := "10000000000" // Synthetic-only phone; not a customer record.
	merchant, store, product, validCoupon, otherPhoneCoupon, otherTenantCoupon, usedCoupon := createCustomerOrderFixtures(t, db, phone)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	Register(engine, db, &config.Config{
		AppName:            "synthetic customer-order integration test",
		AppEnv:             "test",
		AppHost:            "127.0.0.1",
		AppPort:            "0",
		JWTSecret:          "synthetic-only-jwt-secret-not-used-by-public-routes",
		AESSecret:          "synthetic-only-aes-secret-not-used-by-public-routes",
		RateLimitPerMinute: 10000,
		FrontendURL:        "http://localhost.invalid",
		// Deliberately omit Alipay credentials. The create route must not call a
		// provider; it returns an unavailable-checkout hint and leaves the order unpaid.
	})

	productsResponse := customerOrderAPIRequest(t, engine, http.MethodGet,
		fmt.Sprintf("/api/v1/customer/store/%d/products", store.ID), "", nil)
	if productsResponse.Code != http.StatusOK {
		t.Fatalf("GET public products: status=%d body=%s", productsResponse.Code, productsResponse.Body.String())
	}
	productData := customerOrderAPIData(t, productsResponse)
	var productList struct {
		List []struct {
			ID    uint  `json:"id"`
			Price int64 `json:"price"`
		} `json:"list"`
	}
	if err := json.Unmarshal(productData, &productList); err != nil {
		t.Fatalf("decode public product list: %v", err)
	}
	if len(productList.List) != 1 || productList.List[0].ID != product.ID || productList.List[0].Price != 1250 {
		t.Fatalf("public product contract must return list with price in cents: %+v", productList.List)
	}

	couponResponse := customerOrderAPIRequest(t, engine, http.MethodGet,
		fmt.Sprintf("/api/v1/customer/store/%d/coupons?phone=%s", store.ID, phone), "", nil)
	if couponResponse.Code != http.StatusOK {
		t.Fatalf("GET coupons for explicitly supplied synthetic phone: status=%d body=%s", couponResponse.Code, couponResponse.Body.String())
	}
	couponData := customerOrderAPIData(t, couponResponse)
	var couponList struct {
		List []model.ReferralCoupon `json:"list"`
	}
	if err := json.Unmarshal(couponData, &couponList); err != nil {
		t.Fatalf("decode public coupon list: %v", err)
	}
	if len(couponList.List) != 1 || couponList.List[0].CouponNo != validCoupon.CouponNo {
		t.Fatalf("phone/store-scoped coupon lookup returned unexpected coupons: %+v", couponList.List)
	}

	// Match the corrected mobile DTO field names and cents-based product value.
	// This synthetic store uses submit_later mode, so the route must not build a
	// payment payload or contact any provider while creating the unpaid order.
	createPayload := map[string]any{
		"store_id":        store.ID,
		"customer_phone":  phone,
		"customer_note":   "合成集成测试备注",
		"coupon_no":       validCoupon.CouponNo,
		"payment_channel": "alipay",
		"pay_mode":        "page",
		"items": []map[string]any{{
			"product_id": product.ID,
			"quantity":   1,
			"price":      productList.List[0].Price,
		}},
	}
	createResponse := customerOrderAPIRequest(t, engine, http.MethodPost, "/api/v1/customer/orders", "", createPayload)
	if createResponse.Code != http.StatusOK {
		t.Fatalf("POST mobile customer order using corrected DTO: status=%d body=%s", createResponse.Code, createResponse.Body.String())
	}
	createData := customerOrderAPIData(t, createResponse)
	var created struct {
		Order struct {
			OrderNo        string  `json:"order_no"`
			Status         string  `json:"status"`
			Amount         int64   `json:"subtotal_amount"`
			TotalAmount    int64   `json:"total_amount"`
			Discount       int64   `json:"discount_amount"`
			PaymentChannel string  `json:"payment_channel"`
			PaidAt         *string `json:"paid_at"`
		} `json:"order"`
		AccessToken     string `json:"access_token"`
		PaymentError    string `json:"payment_error"`
		PaymentRequired *bool  `json:"payment_required"`
	}
	if err := json.Unmarshal(createData, &created); err != nil {
		t.Fatalf("decode customer order create response: %v", err)
	}
	if created.Order.OrderNo == "" || created.AccessToken == "" {
		t.Fatalf("create response must include order number and one-time access token: %s", createData)
	}
	if created.Order.Status != "submitted" || created.Order.Amount != 1250 || created.Order.Discount != 200 || created.Order.TotalAmount != 1050 {
		t.Fatalf("created order must use server product price in cents and the matching coupon: %+v", created.Order)
	}
	if created.Order.PaidAt != nil || created.PaymentError != "" || created.PaymentRequired == nil || *created.PaymentRequired {
		t.Fatalf("submit-later order must be unpaid and explicitly require no advance payment: %+v", created)
	}
	var createFields map[string]json.RawMessage
	if err := json.Unmarshal(createData, &createFields); err != nil {
		t.Fatal(err)
	}
	if _, hasPaymentPayload := createFields["payment"]; hasPaymentPayload {
		t.Fatal("submit-later order must not return a live payment payload")
	}

	var stored model.Order
	if err := db.Where("order_no = ?", created.Order.OrderNo).First(&stored).Error; err != nil {
		t.Fatalf("load persisted customer order: %v", err)
	}
	tokenHash := sha256.Sum256([]byte(created.AccessToken))
	if stored.CustomerAccessTokenHash != hex.EncodeToString(tokenHash[:]) || stored.CustomerAccessTokenHash == created.AccessToken {
		t.Fatal("database must store only the SHA-256 hash of the one-time order access token")
	}
	var rawTokenColumnCount int64
	if err := db.Raw("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'orders' AND column_name = 'customer_access_token'").Scan(&rawTokenColumnCount).Error; err != nil {
		t.Fatalf("inspect order token schema: %v", err)
	}
	if rawTokenColumnCount != 0 {
		t.Fatal("raw customer access token must not have a database column")
	}
	if stored.CustomerPhone != phone || stored.CouponNo != validCoupon.CouponNo || stored.CouponID == nil || *stored.CouponID != validCoupon.ID {
		t.Fatalf("persisted order does not match synthetic customer/coupon ownership: %+v", stored)
	}

	orderPath := "/api/v1/customer/orders/" + stored.OrderNo
	for _, tc := range []struct {
		name  string
		token string
	}{
		{name: "missing token"},
		{name: "wrong token", token: "synthetic-wrong-access-token"},
	} {
		t.Run("secure lookup rejects "+tc.name, func(t *testing.T) {
			response := customerOrderAPIRequest(t, engine, http.MethodGet, orderPath, tc.token, nil)
			if response.Code != http.StatusNotFound {
				t.Fatalf("unauthorized order lookup status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
	queryTokenResponse := customerOrderAPIRequest(t, engine, http.MethodGet, orderPath+"?access_token=synthetic-query-only-token", "", nil)
	if queryTokenResponse.Code != http.StatusNotFound {
		t.Fatalf("query string alone must not authorize a public order lookup: status=%d", queryTokenResponse.Code)
	}

	detailResponse := customerOrderAPIRequest(t, engine, http.MethodGet, orderPath, created.AccessToken, nil)
	if detailResponse.Code != http.StatusOK {
		t.Fatalf("GET order detail with secure header token: status=%d body=%s", detailResponse.Code, detailResponse.Body.String())
	}
	detailData := customerOrderAPIData(t, detailResponse)
	var detail struct {
		Order map[string]json.RawMessage `json:"order"`
	}
	if err := json.Unmarshal(detailData, &detail); err != nil {
		t.Fatalf("decode public order detail: %v", err)
	}
	for _, forbidden := range []string{
		"customer_phone", "merchant_note", "transaction_no", "customer_access_token",
		"customer_access_token_hash", "user", "merchant", "package", "subscription",
	} {
		if _, exists := detail.Order[forbidden]; exists {
			t.Errorf("public order response must not expose %q", forbidden)
		}
	}
	if _, exists := detail.Order["order_no"]; !exists {
		t.Fatal("public order response is missing the whitelisted order number")
	}

	// Phone is optional when the customer is not requesting a coupon lookup or
	// applying a coupon; the mobile payload may send it as an empty string.
	withoutCouponPayload := make(map[string]any, len(createPayload))
	for key, value := range createPayload {
		withoutCouponPayload[key] = value
	}
	withoutCouponPayload["customer_phone"] = ""
	withoutCouponPayload["coupon_no"] = ""
	withoutCouponResponse := customerOrderAPIRequest(t, engine, http.MethodPost, "/api/v1/customer/orders", "", withoutCouponPayload)
	if withoutCouponResponse.Code != http.StatusOK {
		t.Fatalf("order without phone/coupon should remain available: status=%d body=%s", withoutCouponResponse.Code, withoutCouponResponse.Body.String())
	}
	withoutCouponData := customerOrderAPIData(t, withoutCouponResponse)
	var withoutCoupon struct {
		Order struct {
			OrderNo     string `json:"order_no"`
			Status      string `json:"status"`
			TotalAmount int64  `json:"total_amount"`
		} `json:"order"`
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(withoutCouponData, &withoutCoupon); err != nil {
		t.Fatalf("decode no-phone/no-coupon order: %v", err)
	}
	if withoutCoupon.Order.OrderNo == "" || withoutCoupon.AccessToken == "" || withoutCoupon.Order.Status != "submitted" || withoutCoupon.Order.TotalAmount != 1250 {
		t.Fatalf("no-phone/no-coupon order did not follow optional-phone contract: %+v", withoutCoupon)
	}
	var noPhoneOrder model.Order
	if err := db.Where("order_no = ?", withoutCoupon.Order.OrderNo).First(&noPhoneOrder).Error; err != nil {
		t.Fatalf("load no-phone/no-coupon order: %v", err)
	}
	if noPhoneOrder.CustomerPhone != "" || noPhoneOrder.CouponID != nil || noPhoneOrder.PaidAt != nil {
		t.Fatalf("no-phone/no-coupon order must not infer customer/coupon/payment facts: phone=%q coupon=%v paid_at=%v", noPhoneOrder.CustomerPhone, noPhoneOrder.CouponID, noPhoneOrder.PaidAt)
	}

	// A customer saying they paid only queues a confirmation; it is not a payment
	// fact. This endpoint is exercised before any local server-side confirmation.
	markPaidResponse := customerOrderAPIRequest(t, engine, http.MethodPost, orderPath+"/paid", created.AccessToken, nil)
	if markPaidResponse.Code != http.StatusOK {
		t.Fatalf("customer mark-paid request: status=%d body=%s", markPaidResponse.Code, markPaidResponse.Body.String())
	}
	var waiting model.Order
	if err := db.First(&waiting, stored.ID).Error; err != nil {
		t.Fatalf("reload order after customer mark-paid request: %v", err)
	}
	if waiting.Status != "payment_confirming" || waiting.PaidAt != nil || waiting.TransactionNo != "" {
		t.Fatalf("customer report must remain awaiting merchant confirmation, got status=%q paid_at=%v transaction=%q", waiting.Status, waiting.PaidAt, waiting.TransactionNo)
	}
	var paymentRecords int64
	if err := db.Model(&model.PaymentRecord{}).Where("order_id = ?", stored.ID).Count(&paymentRecords).Error; err != nil {
		t.Fatal(err)
	}
	if paymentRecords != 0 {
		t.Fatalf("customer mark-paid must not create a payment record, found %d", paymentRecords)
	}
	var stillUnused model.ReferralCoupon
	if err := db.First(&stillUnused, validCoupon.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stillUnused.Status != "unused" {
		t.Fatalf("coupon must not be consumed before payment confirmation, status=%q", stillUnused.Status)
	}
	orderService := service.NewOrderService(db)
	if err := orderService.AcceptMerchantOrder(merchant.ID, stored.ID); err == nil {
		t.Fatal("fulfillment must not proceed while a deferred-payment order awaits confirmation")
	}

	// This is an in-process synthetic server-side confirmation against the
	// disposable DB only; it makes no provider/network call and uses no real merchant.
	confirmed, err := orderService.ConfirmMerchantOrderPayment(merchant.ID, stored.ID)
	if err != nil {
		t.Fatalf("synthetic local server confirmation: %v", err)
	}
	if confirmed.PaidAt == nil || confirmed.Status != "received" || confirmed.PaymentChannel != "merchant_qr" {
		t.Fatalf("server confirmation should record payment but keep fulfillment separate: status=%q paid_at=%v channel=%q", confirmed.Status, confirmed.PaidAt, confirmed.PaymentChannel)
	}
	var consumed model.ReferralCoupon
	if err := db.First(&consumed, validCoupon.ID).Error; err != nil {
		t.Fatal(err)
	}
	if consumed.Status != "used" || consumed.UsedAt == nil {
		t.Fatalf("coupon should be consumed only by the confirmed-payment path: status=%q used_at=%v", consumed.Status, consumed.UsedAt)
	}

	// Repeating the server confirmation is idempotent in the current service:
	// no second payment row, no second stock deduction, and no coupon re-use.
	if _, err := orderService.ConfirmMerchantOrderPayment(merchant.ID, stored.ID); err != nil {
		t.Fatalf("repeat synthetic server confirmation: %v", err)
	}
	if err := db.Model(&model.PaymentRecord{}).Where("order_id = ?", stored.ID).Count(&paymentRecords).Error; err != nil {
		t.Fatal(err)
	}
	if paymentRecords != 1 {
		t.Fatalf("repeated confirmation must not duplicate payment records, found %d", paymentRecords)
	}
	var currentProduct model.StoreProduct
	if err := db.First(&currentProduct, product.ID).Error; err != nil {
		t.Fatal(err)
	}
	if currentProduct.Stock == nil || *currentProduct.Stock != 9 {
		t.Fatalf("repeated confirmation must not deduct stock twice, stock=%v", currentProduct.Stock)
	}

	orderCountBeforeInvalidCoupons := countCustomerStoreOrders(t, db)
	for _, tc := range []struct {
		name     string
		couponNo string
	}{
		{name: "coupon owned by a different phone", couponNo: otherPhoneCoupon.CouponNo},
		{name: "coupon belonging to another store and tenant", couponNo: otherTenantCoupon.CouponNo},
		{name: "already-used coupon", couponNo: usedCoupon.CouponNo},
		{name: "re-use after server-confirmed redemption", couponNo: validCoupon.CouponNo},
	} {
		t.Run("reject "+tc.name, func(t *testing.T) {
			payload := map[string]any{}
			for key, value := range createPayload {
				payload[key] = value
			}
			payload["coupon_no"] = tc.couponNo
			response := customerOrderAPIRequest(t, engine, http.MethodPost, "/api/v1/customer/orders", "", payload)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("invalid coupon was not rejected: status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
	if got := countCustomerStoreOrders(t, db); got != orderCountBeforeInvalidCoupons {
		t.Fatalf("coupon validation failures must not create orders: before=%d after=%d", orderCountBeforeInvalidCoupons, got)
	}

	t.Logf("mobile API contract passed: order=%s, amount=%d cents, discount=%d cents; customer mark-paid remained unpaid; synthetic server confirmation consumed coupon once; provider calls=0", stored.OrderNo, stored.TotalAmount, stored.DiscountAmount)
}

func openSafeCustomerOrderMySQL(t *testing.T) *gorm.DB {
	t.Helper()
	if strings.TrimSpace(os.Getenv("MYSQL_DSN")) != "" || strings.TrimSpace(os.Getenv("MYSQL_PWD")) != "" {
		t.Fatal("refusing customer-order integration test while MYSQL_DSN or MYSQL_PWD is set")
	}
	raw := strings.TrimSpace(os.Getenv(customerOrderITDSNEnv))
	rootValue := strings.TrimSpace(os.Getenv(customerOrderITRootEnv))
	if raw == "" || rootValue == "" {
		t.Skipf("set %s and %s from scripts/test-customer-order-mysql.sh to run", customerOrderITDSNEnv, customerOrderITRootEnv)
	}
	rootInfo, err := os.Lstat(rootValue)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		t.Fatal("refusing missing, non-directory, or symlinked temporary MySQL root")
	}
	root, err := filepath.EvalSymlinks(rootValue)
	if err != nil || filepath.Clean(rootValue) != root || !strings.HasPrefix(root, "/tmp/cue-customer-order-mysql-") {
		t.Fatal("refusing MySQL root outside a canonical /tmp/cue-customer-order-mysql-* directory")
	}
	if _, err := os.Stat(filepath.Join(root, ".cue-customer-order-it-owned")); err != nil {
		t.Fatal("refusing MySQL root without the test-run ownership marker")
	}

	parsed, err := mysqlDriver.ParseDSN(raw)
	if err != nil {
		t.Fatal("refusing malformed disposable MySQL DSN")
	}
	if parsed.Net != "unix" || parsed.User != "root" || parsed.Passwd != "" || !strings.HasPrefix(parsed.DBName, "cue_customer_order_it_") {
		t.Fatal("refusing DSN: require passwordless root, Unix socket, and cue_customer_order_it_* database")
	}
	socket := filepath.Clean(parsed.Addr)
	if !filepath.IsAbs(socket) || !pathWithinRoot(socket, root) {
		t.Fatal("refusing socket outside the dedicated temporary MySQL root")
	}

	db, err := gorm.Open(gormmysql.Open(raw), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open disposable MySQL socket: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(16)
	t.Cleanup(func() { _ = sqlDB.Close() })

	var skipNetworking int
	var datadir, serverSocket, version string
	if err := db.Raw("SELECT @@skip_networking, @@datadir, @@socket, VERSION()").Row().Scan(&skipNetworking, &datadir, &serverSocket, &version); err != nil {
		t.Fatalf("inspect isolated MySQL safety settings: %v", err)
	}
	resolvedData, dataErr := filepath.EvalSymlinks(filepath.Clean(datadir))
	resolvedSocket, socketErr := filepath.Abs(filepath.Clean(serverSocket))
	if skipNetworking != 1 || !strings.HasPrefix(version, "8.") || dataErr != nil || !pathWithinRoot(resolvedData, root) || socketErr != nil || resolvedSocket != socket || !pathWithinRoot(resolvedSocket, root) {
		t.Fatalf("unsafe MySQL server: skip_networking=%d version=%q datadir=%q socket=%q", skipNetworking, version, datadir, serverSocket)
	}
	return db
}

func createCustomerOrderFixtures(t *testing.T, db *gorm.DB, phone string) (model.Merchant, model.Store, model.StoreProduct, model.ReferralCoupon, model.ReferralCoupon, model.ReferralCoupon, model.ReferralCoupon) {
	t.Helper()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	merchant := model.Merchant{Name: "synthetic-customer-order-merchant-" + suffix, Status: "active"}
	if err := db.Create(&merchant).Error; err != nil {
		t.Fatalf("create synthetic merchant: %v", err)
	}
	store := model.Store{
		MerchantID: merchant.ID,
		Name:       "synthetic-customer-order-store-" + suffix,
		Status:     "active",
		IsOpen:     true,
		OrderMode:  "submit_later",
	}
	if err := db.Create(&store).Error; err != nil {
		t.Fatalf("create synthetic store: %v", err)
	}
	stock := 10
	product := model.StoreProduct{StoreID: store.ID, Name: "synthetic-product", Price: 1250, Status: "active", Stock: &stock}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create synthetic product: %v", err)
	}

	foreignMerchant := model.Merchant{Name: "synthetic-foreign-merchant-" + suffix, Status: "active"}
	if err := db.Create(&foreignMerchant).Error; err != nil {
		t.Fatalf("create synthetic foreign merchant: %v", err)
	}
	foreignStore := model.Store{MerchantID: foreignMerchant.ID, Name: "synthetic-foreign-store-" + suffix, Status: "active", IsOpen: true, OrderMode: "submit_later"}
	if err := db.Create(&foreignStore).Error; err != nil {
		t.Fatalf("create synthetic foreign store: %v", err)
	}

	now := time.Now().UTC()
	validFrom, validTo := now.Add(-time.Minute), now.Add(time.Hour)
	validCoupon := model.ReferralCoupon{
		MerchantID: merchant.ID, StoreID: store.ID, CouponNo: "SYNTH-CUSTOMER-VALID-" + suffix,
		OwnerPhone: phone, OwnerType: "new_customer", Title: "synthetic coupon", Amount: 200,
		Threshold: 1000, Status: "unused", ValidFrom: &validFrom, ValidTo: &validTo,
	}
	otherPhoneCoupon := model.ReferralCoupon{
		MerchantID: merchant.ID, StoreID: store.ID, CouponNo: "SYNTH-CUSTOMER-OTHER-PHONE-" + suffix,
		OwnerPhone: "10000000001", OwnerType: "new_customer", Title: "synthetic other-phone coupon", Amount: 200,
		Threshold: 1000, Status: "unused", ValidFrom: &validFrom, ValidTo: &validTo,
	}
	otherTenantCoupon := model.ReferralCoupon{
		MerchantID: foreignMerchant.ID, StoreID: foreignStore.ID, CouponNo: "SYNTH-CUSTOMER-OTHER-TENANT-" + suffix,
		OwnerPhone: phone, OwnerType: "new_customer", Title: "synthetic foreign coupon", Amount: 200,
		Threshold: 1000, Status: "unused", ValidFrom: &validFrom, ValidTo: &validTo,
	}
	usedAt := now.Add(-time.Minute)
	usedCoupon := model.ReferralCoupon{
		MerchantID: merchant.ID, StoreID: store.ID, CouponNo: "SYNTH-CUSTOMER-USED-" + suffix,
		OwnerPhone: phone, OwnerType: "new_customer", Title: "synthetic used coupon", Amount: 200,
		Threshold: 1000, Status: "used", UsedAt: &usedAt, ValidFrom: &validFrom, ValidTo: &validTo,
	}
	for _, coupon := range []*model.ReferralCoupon{&validCoupon, &otherPhoneCoupon, &otherTenantCoupon, &usedCoupon} {
		if err := db.Create(coupon).Error; err != nil {
			t.Fatalf("create synthetic coupon %q: %v", coupon.CouponNo, err)
		}
	}
	return merchant, store, product, validCoupon, otherPhoneCoupon, otherTenantCoupon, usedCoupon
}

func customerOrderAPIRequest(t *testing.T, engine http.Handler, method, path, accessToken string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var requestBody *strings.Reader
	if body == nil {
		requestBody = strings.NewReader("")
	} else {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("encode customer API request: %v", err)
		}
		requestBody = strings.NewReader(string(encoded))
	}
	request := httptest.NewRequest(method, path, requestBody)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if accessToken != "" {
		request.Header.Set("X-Order-Access-Token", accessToken)
	}
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	return response
}

func customerOrderAPIData(t *testing.T, response *httptest.ResponseRecorder) json.RawMessage {
	t.Helper()
	var envelope customerOrderAPIEnvelope
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode customer API envelope: %v; body=%s", err, response.Body.String())
	}
	if envelope.Code != 0 || len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		t.Fatalf("customer API returned non-success envelope: status=%d code=%d message=%q body=%s", response.Code, envelope.Code, envelope.Message, response.Body.String())
	}
	return envelope.Data
}

func countCustomerStoreOrders(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var count int64
	if err := db.Model(&model.Order{}).Where("order_type = ?", "store_order").Count(&count).Error; err != nil {
		t.Fatalf("count synthetic customer orders: %v", err)
	}
	return count
}
