package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

const couponLookupTestSecret = "synthetic-coupon-lookup-hmac-key-for-tests-only"

func TestCustomerCouponLookupRateLimitUsesPhoneBodyAndPreservesManualLookup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter, ok := newCustomerCouponLookupLimiter(couponLookupTestSecret, 10, 2, 100, time.Minute)
	if !ok {
		t.Fatal("synthetic HMAC secret should enable the lookup limiter")
	}
	engine := gin.New()
	engine.POST("/coupons", customerCouponLookupProtection(limiter, ok), func(c *gin.Context) {
		if c.Request.URL.RawQuery != "" {
			t.Errorf("customer phone must not be carried in the URL: %q", c.Request.URL.RawQuery)
		}
		c.JSON(http.StatusOK, gin.H{"coupon_no": "SYNTHETIC-COUPON-DETAILS"})
	})

	phone := "13800138000"
	for attempt, ip := range []string{"198.51.100.10", "198.51.100.11"} {
		response := couponLookupRequest(engine, ip, "/coupons", `{"phone":"`+phone+`"}`)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "SYNTHETIC-COUPON-DETAILS") {
			t.Fatalf("manually requested lookup %d should return its normal coupon response: status=%d body=%s", attempt+1, response.Code, response.Body.String())
		}
	}

	limited := couponLookupRequest(engine, "198.51.100.12", "/coupons", `{"phone":"`+phone+`"}`)
	if limited.Code != http.StatusTooManyRequests {
		t.Fatalf("third per-phone request should be throttled: status=%d body=%s", limited.Code, limited.Body.String())
	}
	if strings.Contains(limited.Body.String(), "SYNTHETIC-COUPON-DETAILS") {
		t.Fatalf("over-limit response leaked coupon details: %s", limited.Body.String())
	}

	invalid := couponLookupRequest(engine, "203.0.113.10", "/coupons", `{"phone":"not-a-phone"}`)
	if invalid.Code != http.StatusBadRequest || strings.Contains(invalid.Body.String(), "SYNTHETIC-COUPON-DETAILS") {
		t.Fatalf("invalid lookup must fail without coupon details: status=%d body=%s", invalid.Code, invalid.Body.String())
	}
	unknownField := couponLookupRequest(engine, "203.0.113.11", "/coupons", `{"phone":"13800138001","extra":"synthetic"}`)
	if unknownField.Code != http.StatusBadRequest || strings.Contains(unknownField.Body.String(), "SYNTHETIC-COUPON-DETAILS") {
		t.Fatalf("lookup must reject unrecognized fields without coupon details: status=%d body=%s", unknownField.Code, unknownField.Body.String())
	}
	queryPhone := couponLookupRequest(engine, "203.0.113.12", "/coupons?phone="+phone, `{"phone":"13800138002"}`)
	if queryPhone.Code != http.StatusBadRequest || strings.Contains(queryPhone.Body.String(), "SYNTHETIC-COUPON-DETAILS") {
		t.Fatalf("phone in URL must be rejected without coupon details: status=%d body=%s", queryPhone.Code, queryPhone.Body.String())
	}

	for key := range limiter.buckets {
		if strings.Contains(key, phone) {
			t.Fatalf("raw phone was retained in a throttle key: %q", key)
		}
	}
	if key := limiter.bucketKey("phone", phone); strings.Contains(key, phone) || len(key) != len("phone:")+64 {
		t.Fatalf("phone throttle key must be a fixed-size keyed digest: %q", key)
	}
}

func TestCustomerCouponLookupRateLimitAppliesPerIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter, ok := newCustomerCouponLookupLimiter(couponLookupTestSecret, 2, 100, 100, time.Minute)
	if !ok {
		t.Fatal("synthetic HMAC secret should enable the lookup limiter")
	}
	engine := gin.New()
	engine.POST("/coupons", customerCouponLookupProtection(limiter, ok), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	for attempt, phone := range []string{"13800138010", "13800138011", "13800138012"} {
		response := couponLookupRequest(engine, "192.0.2.44", "/coupons", `{"phone":"`+phone+`"}`)
		expected := http.StatusNoContent
		if attempt == 2 {
			expected = http.StatusTooManyRequests
		}
		if response.Code != expected {
			t.Fatalf("per-IP request %d: expected %d, got %d; body=%s", attempt+1, expected, response.Code, response.Body.String())
		}
	}
}

func TestCustomerCouponLookupRateLimitIgnoresSpoofedForwardingHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter, ok := newCustomerCouponLookupLimiter(couponLookupTestSecret, 2, 100, 100, time.Minute)
	if !ok {
		t.Fatal("synthetic HMAC secret should enable the lookup limiter")
	}
	engine := gin.New()
	engine.POST("/coupons", customerCouponLookupProtection(limiter, ok), func(c *gin.Context) { c.Status(http.StatusNoContent) })

	spoofedHeaders := []map[string]string{
		{"X-Forwarded-For": "203.0.113.10", "X-Real-IP": "198.51.100.10"},
		{"X-Forwarded-For": "203.0.113.11", "X-Real-IP": "198.51.100.11"},
		{"X-Forwarded-For": "203.0.113.12", "X-Real-IP": "198.51.100.12"},
	}
	for attempt, headers := range spoofedHeaders {
		response := couponLookupRequestFromPeer(engine, "192.0.2.44:43210", "/coupons", fmt.Sprintf(`{"phone":"138001380%02d"}`, attempt+20), headers)
		expected := http.StatusNoContent
		if attempt == 2 {
			expected = http.StatusTooManyRequests
		}
		if response.Code != expected {
			t.Fatalf("request %d from the same TCP peer should share one IP quota despite spoofed forwarding headers: expected %d, got %d; body=%s", attempt+1, expected, response.Code, response.Body.String())
		}
	}
	if bucket, exists := limiter.buckets[limiter.bucketKey("ip", "192.0.2.44")]; !exists || bucket.count != 3 {
		t.Fatalf("IP quota must be keyed by the TCP peer, got bucket=%+v exists=%t", bucket, exists)
	}
}

func TestCustomerCouponLookupRateLimitFailsClosedForInvalidRemoteAddr(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter, ok := newCustomerCouponLookupLimiter(couponLookupTestSecret, 10, 10, 100, time.Minute)
	if !ok {
		t.Fatal("synthetic HMAC secret should enable the lookup limiter")
	}
	engine := gin.New()
	handlerCalled := false
	engine.POST("/coupons", customerCouponLookupProtection(limiter, ok), func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusNoContent)
	})

	for _, remoteAddr := range []string{"", "192.0.2.44", "not-an-ip:43210", "192.0.2.44:not-a-port", "[2001:db8::44]:not-a-port"} {
		response := couponLookupRequestFromPeer(engine, remoteAddr, "/coupons", `{"phone":"13800138000"}`, nil)
		if response.Code != http.StatusBadRequest {
			t.Errorf("invalid RemoteAddr %q must fail closed with 400, got %d; body=%s", remoteAddr, response.Code, response.Body.String())
		}
	}
	if handlerCalled {
		t.Fatal("handler must not run when RemoteAddr is missing or malformed")
	}
	if len(limiter.buckets) != 0 {
		t.Fatalf("invalid RemoteAddr must not create rate-limit buckets, got %d", len(limiter.buckets))
	}
}

func TestCustomerCouponLookupLimiterStateIsBoundedAndExpires(t *testing.T) {
	limiter, ok := newCustomerCouponLookupLimiter(couponLookupTestSecret, 1, 1, 4, time.Minute)
	if !ok {
		t.Fatal("synthetic HMAC secret should enable the lookup limiter")
	}
	start := time.Now()
	for i := 0; i < 100; i++ {
		limiter.take("phone", fmt.Sprintf("synthetic-phone-%03d", i), 1, start)
	}
	if got := len(limiter.buckets); got > limiter.maxBuckets {
		t.Fatalf("throttle state exceeded its hard bound: %d > %d", got, limiter.maxBuckets)
	}
	if limiter.take("phone", "synthetic-phone-after-expiry", 1, start.Add(2*time.Minute)) {
		if got := len(limiter.buckets); got > limiter.maxBuckets {
			t.Fatalf("expired-state cleanup exceeded the hard bound: %d > %d", got, limiter.maxBuckets)
		}
	} else {
		t.Fatal("expired throttle state should be reclaimed for a fresh request")
	}
	if got := len(limiter.buckets); got > limiter.maxBuckets {
		t.Fatalf("throttle state exceeded its hard bound after expiry: %d > %d", got, limiter.maxBuckets)
	}
}

func TestCustomerCouponLookupRateLimitFailsClosedWithoutSafeKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/coupons", CustomerCouponLookupRateLimit("replace-with-a-very-strong-secret"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"coupon_no": "SYNTHETIC-COUPON-DETAILS"})
	})
	response := couponLookupRequest(engine, "192.0.2.80", "/coupons", `{"phone":"13800138000"}`)
	if response.Code != http.StatusServiceUnavailable || strings.Contains(response.Body.String(), "SYNTHETIC-COUPON-DETAILS") {
		t.Fatalf("lookup must fail closed when a safe throttle key is unavailable: status=%d body=%s", response.Code, response.Body.String())
	}
}

func couponLookupRequest(engine http.Handler, ip, path, body string) *httptest.ResponseRecorder {
	return couponLookupRequestFromPeer(engine, ip+":43210", path, body, nil)
}

func couponLookupRequestFromPeer(engine http.Handler, remoteAddr, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.RemoteAddr = remoteAddr
	request.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	return response
}
