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

	"go-web-gin-health/internal/model"
)

func TestMerchantSubscriptionMiddlewareRequiresActiveStatusAndFutureExpiry(t *testing.T) {
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
	if err := db.AutoMigrate(&model.Merchant{}); err != nil {
		t.Fatalf("migrate merchant test schema: %v", err)
	}

	now := time.Date(2030, time.January, 2, 3, 4, 5, 0, time.UTC)
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)
	cases := []struct {
		name               string
		merchantStatus     string
		subscriptionStatus string
		expireAt           *time.Time
		legacyExpireAt     *time.Time
		allowed            bool
	}{
		{name: "active with future primary expiry", merchantStatus: "active", subscriptionStatus: "active", expireAt: &future, allowed: true},
		{name: "active with legacy expiry only", merchantStatus: "active", subscriptionStatus: "active", legacyExpireAt: &future, allowed: true},
		{name: "active exactly at expiry boundary", merchantStatus: "active", subscriptionStatus: "active", expireAt: &now},
		{name: "active but expired", merchantStatus: "active", subscriptionStatus: "active", expireAt: &past},
		{name: "inactive despite future expiry", merchantStatus: "active", subscriptionStatus: "inactive", expireAt: &future},
		{name: "paused despite future expiry", merchantStatus: "active", subscriptionStatus: "paused", expireAt: &future},
		{name: "suspended subscription despite future expiry", merchantStatus: "active", subscriptionStatus: "suspended", expireAt: &future},
		{name: "pending or unpaid subscription despite future expiry", merchantStatus: "active", subscriptionStatus: "pending", expireAt: &future},
		{name: "unknown subscription state despite future expiry", merchantStatus: "active", subscriptionStatus: "unknown", expireAt: &future},
		{name: "active state without expiry", merchantStatus: "active", subscriptionStatus: "active"},
		{name: "pending merchant with active subscription", merchantStatus: "pending", subscriptionStatus: "active", expireAt: &future},
		{name: "suspended merchant with active subscription", merchantStatus: "suspended", subscriptionStatus: "active", expireAt: &future},
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
	engine.GET("/paid", merchantSubscriptionMiddlewareAt(db, func() time.Time { return now }), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	engine.GET("/basic", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			merchant := model.Merchant{
				Name:                  "synthetic-" + test.name,
				Status:                test.merchantStatus,
				SubscriptionStatus:    test.subscriptionStatus,
				SubscriptionExpireAt:  test.expireAt,
				SubscriptionExpiredAt: test.legacyExpireAt,
			}
			if err := db.Create(&merchant).Error; err != nil {
				t.Fatalf("create synthetic merchant: %v", err)
			}

			request := httptest.NewRequest(http.MethodGet, "/paid", nil)
			request.Header.Set("X-Test-Merchant-ID", strconv.FormatUint(uint64(merchant.ID), 10))
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			want := http.StatusForbidden
			if test.allowed {
				want = http.StatusNoContent
			}
			if response.Code != want {
				t.Fatalf("paid feature status=%d, want %d; body=%s", response.Code, want, response.Body.String())
			}

			basicRequest := httptest.NewRequest(http.MethodGet, "/basic", nil)
			basicRequest.Header.Set("X-Test-Merchant-ID", strconv.FormatUint(uint64(merchant.ID), 10))
			basicResponse := httptest.NewRecorder()
			engine.ServeHTTP(basicResponse, basicRequest)
			if basicResponse.Code != http.StatusNoContent {
				t.Fatalf("basic endpoint status=%d, want %d", basicResponse.Code, http.StatusNoContent)
			}
		})
	}
}

func TestExpiredMerchantReadOnlyRoutesAndPausedFullLock(t *testing.T) {
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
	if err := db.AutoMigrate(&model.Merchant{}); err != nil {
		t.Fatalf("migrate merchant test schema: %v", err)
	}

	past := time.Now().Add(-time.Hour)
	expired := model.Merchant{Name: "expired-read-only", Status: "active", SubscriptionStatus: "active", SubscriptionExpireAt: &past}
	pausedExpiry := time.Now().Add(time.Hour)
	paused := model.Merchant{Name: "paused-full-lock", Status: "active", SubscriptionStatus: "paused", SubscriptionExpireAt: &pausedExpiry}
	activeExpiry := time.Now().Add(time.Hour)
	active := model.Merchant{Name: "active-unexpired", Status: "active", SubscriptionStatus: "active", SubscriptionExpireAt: &activeExpiry}
	for _, merchant := range []*model.Merchant{&expired, &paused, &active} {
		if err := db.Create(merchant).Error; err != nil {
			t.Fatalf("create synthetic merchant: %v", err)
		}
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
	guard := MerchantSubscriptionMiddleware(db)
	read := func(c *gin.Context) { c.Status(http.StatusNoContent) }
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		engine.Handle(method, "/api/v1/merchant/orders/:id", guard, read)
	}
	for _, route := range []string{
		"/leads", "/leads/stats", "/payment-config", "/stores", "/stores/:storeId/products",
		"/orders", "/refunds", "/settlements", "/promotions", "/group-buy-links",
		"/coupons", "/ai/insights", "/ai/quota", "/ai/share-stats", "/share/config",
	} {
		engine.GET("/api/v1/merchant"+route, guard, read)
	}

	request := func(method, path string, merchantID uint) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("X-Test-Merchant-ID", strconv.FormatUint(uint64(merchantID), 10))
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, req)
		return response
	}

	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		got := request(method, "/api/v1/merchant/orders/42", expired.ID).Code
		want := http.StatusForbidden
		if method == http.MethodGet {
			want = http.StatusNoContent
		}
		if got != want {
			t.Errorf("expired merchant %s /orders/:id status=%d, want %d", method, got, want)
		}
	}
	for _, route := range []string{
		"/leads", "/leads/stats", "/payment-config", "/stores", "/stores/:storeId/products",
		"/orders", "/orders/42", "/refunds", "/settlements", "/promotions", "/group-buy-links",
	} {
		if got := request(http.MethodGet, "/api/v1/merchant"+route, expired.ID).Code; got != http.StatusNoContent {
			t.Errorf("expired merchant GET %s status=%d, want %d", route, got, http.StatusNoContent)
		}
	}
	for _, path := range []string{
		"/api/v1/merchant/coupons", "/api/v1/merchant/ai/insights", "/api/v1/merchant/ai/quota",
		"/api/v1/merchant/ai/share-stats", "/api/v1/merchant/share/config",
	} {
		if got := request(http.MethodGet, path, expired.ID).Code; got != http.StatusForbidden {
			t.Errorf("expired merchant GET %s status=%d, want %d", path, got, http.StatusForbidden)
		}
	}
	if got := request(http.MethodGet, "/api/v1/merchant/stores", paused.ID).Code; got != http.StatusForbidden {
		t.Errorf("paused merchant business read status=%d, want %d", got, http.StatusForbidden)
	}
	if got := request(http.MethodPost, "/api/v1/merchant/orders/42", paused.ID).Code; got != http.StatusForbidden {
		t.Errorf("paused merchant business write status=%d, want %d", got, http.StatusForbidden)
	}
	if got := request(http.MethodPost, "/api/v1/merchant/orders/42", active.ID).Code; got != http.StatusNoContent {
		t.Errorf("active unexpired merchant write status=%d, want %d", got, http.StatusNoContent)
	}

	if err := db.Migrator().DropTable(&model.Merchant{}); err != nil {
		t.Fatalf("drop merchant table to simulate state lookup failure: %v", err)
	}
	if got := request(http.MethodPost, "/api/v1/merchant/orders/42", expired.ID).Code; got != http.StatusForbidden {
		t.Errorf("subscription lookup failure write status=%d, want %d", got, http.StatusForbidden)
	}
}
