package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestOTPSendRateLimitSharesRoutesAndIsolatesIPs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(OTPSendRateLimit(2))
	paths := []string{
		"/api/merchant/register/send-code",
		"/api/merchant/password/send-code",
		"/api/v1/merchant/register/send-code",
		"/api/v1/merchant/password/send-code",
		"/api/v1/auth/register/send-code",
		"/api/v1/auth/login/send-code",
		"/api/v1/auth/password/send-code",
		"/api/v1/user/phone/send-code",
		"/api/v1/user/email/send-code",
	}
	for _, path := range paths {
		engine.POST(path, func(c *gin.Context) { c.Status(http.StatusNoContent) })
	}
	engine.POST("/api/v1/auth/register", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := func(path, ip string) int {
		t.Helper()
		request := httptest.NewRequest(http.MethodPost, path, nil)
		request.RemoteAddr = ip + ":42000"
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		return response.Code
	}

	for index, path := range paths[:2] {
		if status := req(path, "198.51.100.21"); status != http.StatusNoContent {
			t.Fatalf("same-IP OTP request %d should pass up to shared limit: status=%d", index+1, status)
		}
	}
	if status := req(paths[8], "198.51.100.21"); status != http.StatusTooManyRequests {
		t.Fatalf("another send-code route must share the same IP bucket; status=%d", status)
	}
	if status := req(paths[8], "203.0.113.22"); status != http.StatusNoContent {
		t.Fatalf("a different IP should have an independent bucket; status=%d", status)
	}
	if status := req("/api/v1/auth/register", "198.51.100.21"); status != http.StatusNoContent {
		t.Fatalf("non-OTP route must not consume or be blocked by the OTP-only bucket; status=%d", status)
	}
}

func TestGlobalAndOTPLimitsIgnoreForwardedHeadersFromUntrustedPeer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for name, limiter := range map[string]gin.HandlerFunc{
		"global": RateLimit(1),
		"otp":    OTPSendRateLimit(1),
	} {
		t.Run(name, func(t *testing.T) {
			engine := gin.New()
			if err := engine.SetTrustedProxies(nil); err != nil {
				t.Fatalf("disable proxy trust: %v", err)
			}
			engine.Use(limiter)
			path := "/api/v1/test"
			if name == "otp" {
				path = "/api/v1/test/send-code"
			}
			engine.POST(path, func(c *gin.Context) { c.Status(http.StatusNoContent) })

			request := func(forwardedFor, realIP string) int {
				req := httptest.NewRequest(http.MethodPost, path, nil)
				req.RemoteAddr = "198.51.100.81:42000"
				req.Header.Set("X-Forwarded-For", forwardedFor)
				req.Header.Set("X-Real-IP", realIP)
				response := httptest.NewRecorder()
				engine.ServeHTTP(response, req)
				return response.Code
			}

			if status := request("203.0.113.10", "203.0.113.10"); status != http.StatusNoContent {
				t.Fatalf("first request should pass, got status %d", status)
			}
			if status := request("203.0.113.11", "203.0.113.11"); status != http.StatusTooManyRequests {
				t.Fatalf("spoofed forwarding headers must not create a new IP bucket; status=%d", status)
			}
		})
	}
}

func TestOTPLimitUsesForwardedAddressOnlyFromConfiguredProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	if err := engine.SetTrustedProxies([]string{"10.24.0.0/16"}); err != nil {
		t.Fatalf("set synthetic trusted proxy CIDR: %v", err)
	}
	engine.Use(OTPSendRateLimit(1))
	engine.POST("/api/v1/auth/send-code", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	request := func(remoteIP, forwardedFor string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/send-code", nil)
		req.RemoteAddr = remoteIP + ":42000"
		req.Header.Set("X-Forwarded-For", forwardedFor)
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, req)
		return response.Code
	}
	if status := request("10.24.2.9", "203.0.113.10"); status != http.StatusNoContent {
		t.Fatalf("first client behind configured proxy should pass, status=%d", status)
	}
	if status := request("10.24.2.9", "203.0.113.11"); status != http.StatusNoContent {
		t.Fatalf("different forwarded client from configured proxy should get a separate bucket, status=%d", status)
	}
	if status := request("10.24.2.9", "203.0.113.10"); status != http.StatusTooManyRequests {
		t.Fatalf("repeated forwarded client behind trusted proxy should be limited, status=%d", status)
	}
	if status := request("198.51.100.81", "203.0.113.10"); status != http.StatusNoContent {
		t.Fatalf("untrusted peer must use its socket address, not forwarded client IP; status=%d", status)
	}
	if status := request("198.51.100.81", "203.0.113.11"); status != http.StatusTooManyRequests {
		t.Fatalf("untrusted peer cannot evade its socket-IP bucket by changing XFF; status=%d", status)
	}
}
