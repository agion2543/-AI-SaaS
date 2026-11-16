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
	"go-web-gin-health/internal/model"
)

func TestCustomerCouponLookupPostPrivacyAndThrottle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousWriter := gin.DefaultWriter
	var accessLog bytes.Buffer
	gin.DefaultWriter = &accessLog
	t.Cleanup(func() { gin.DefaultWriter = previousWriter })

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open synthetic coupon lookup database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get synthetic coupon lookup database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&model.Merchant{}, &model.Store{}, &model.ShareCampaign{}, &model.Order{}, &model.ReferralCoupon{}); err != nil {
		t.Fatalf("migrate synthetic coupon lookup schema: %v", err)
	}

	merchant := model.Merchant{Name: "synthetic-coupon-lookup-merchant", Status: "active"}
	if err := db.Create(&merchant).Error; err != nil {
		t.Fatalf("create synthetic merchant: %v", err)
	}
	store := model.Store{MerchantID: merchant.ID, Name: "synthetic-coupon-lookup-store", Status: "active", IsOpen: true}
	if err := db.Create(&store).Error; err != nil {
		t.Fatalf("create synthetic store: %v", err)
	}
	phone := "13800138000"
	coupon := model.ReferralCoupon{
		MerchantID: merchant.ID,
		StoreID:    store.ID,
		CouponNo:   "SYNTHETIC-COUPON-LOOKUP-ONLY",
		OwnerPhone: phone,
		OwnerType:  "referrer",
		Title:      "synthetic coupon",
		Amount:     500,
		Threshold:  2000,
		Status:     "unused",
		ValidFrom:  timePtr(time.Now().Add(-time.Minute)),
		ValidTo:    timePtr(time.Now().Add(time.Hour)),
	}
	if err := db.Create(&coupon).Error; err != nil {
		t.Fatalf("create synthetic coupon: %v", err)
	}

	engine := gin.New()
	Register(engine, db, &config.Config{
		AppName:            "synthetic customer coupon lookup test",
		AppEnv:             "test",
		JWTSecret:          "synthetic-customer-coupon-lookup-hmac-key-for-tests",
		AESSecret:          "synthetic-customer-coupon-lookup-aes-key-000000",
		RateLimitPerMinute: 1000,
		FrontendURL:        "http://localhost.invalid",
	})
	path := fmt.Sprintf("/api/v1/customer/store/%d/coupons", store.ID)

	response := customerOrderAPIRequest(t, engine, http.MethodPost, path, "", map[string]string{"phone": phone})
	if response.Code != http.StatusOK {
		t.Fatalf("POST JSON coupon lookup: status=%d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Data struct {
			List []model.ReferralCoupon `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode synthetic coupon response: %v; body=%s", err, response.Body.String())
	}
	if len(body.Data.List) != 1 || body.Data.List[0].CouponNo != coupon.CouponNo {
		t.Fatalf("manual phone-based lookup should retain normal coupon behavior: %+v", body.Data.List)
	}

	for attempt := 2; attempt <= 5; attempt++ {
		response = customerOrderAPIRequest(t, engine, http.MethodPost, path, "", map[string]string{"phone": phone})
		if response.Code != http.StatusOK {
			t.Fatalf("phone lookup %d within limit should succeed: status=%d body=%s", attempt, response.Code, response.Body.String())
		}
	}
	response = customerOrderAPIRequest(t, engine, http.MethodPost, path, "", map[string]string{"phone": phone})
	if response.Code != http.StatusTooManyRequests || strings.Contains(response.Body.String(), coupon.CouponNo) || strings.Contains(response.Body.String(), coupon.Title) {
		t.Fatalf("over-limit lookup must fail without coupon details: status=%d body=%s", response.Code, response.Body.String())
	}

	invalidResponse := customerOrderAPIRequest(t, engine, http.MethodPost, path, "", map[string]string{"phone": "invalid"})
	if invalidResponse.Code != http.StatusBadRequest || strings.Contains(invalidResponse.Body.String(), coupon.CouponNo) || strings.Contains(invalidResponse.Body.String(), coupon.Title) {
		t.Fatalf("invalid phone must fail without coupon details: status=%d body=%s", invalidResponse.Code, invalidResponse.Body.String())
	}
	queryPhoneResponse := customerOrderAPIRequest(t, engine, http.MethodPost, path+"?phone="+phone, "", map[string]string{"phone": phone})
	if queryPhoneResponse.Code != http.StatusBadRequest || strings.Contains(queryPhoneResponse.Body.String(), coupon.CouponNo) {
		t.Fatalf("phone in URL must be rejected without coupon details: status=%d body=%s", queryPhoneResponse.Code, queryPhoneResponse.Body.String())
	}

	legacyRequest := httptest.NewRequest(http.MethodGet, path+"?phone="+phone, nil)
	legacyResponse := httptest.NewRecorder()
	engine.ServeHTTP(legacyResponse, legacyRequest)
	if legacyResponse.Code != http.StatusNotFound || strings.Contains(legacyResponse.Body.String(), coupon.CouponNo) {
		t.Fatalf("insecure GET lookup path must no longer be active: status=%d body=%s", legacyResponse.Code, legacyResponse.Body.String())
	}
	if strings.Contains(accessLog.String(), phone) || strings.Contains(accessLog.String(), "?phone=") {
		t.Fatalf("access logs must not include phone query parameters: %q", accessLog.String())
	}
}

func timePtr(value time.Time) *time.Time { return &value }
