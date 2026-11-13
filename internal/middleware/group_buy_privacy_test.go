package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGroupBuyRedirectSkipsIPAccessLog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousWriter := gin.DefaultWriter
	var output bytes.Buffer
	gin.DefaultWriter = &output
	t.Cleanup(func() { gin.DefaultWriter = previousWriter })

	engine := gin.New()
	engine.Use(Logger())
	engine.GET("/api/v1/group-buy/r/:code", func(c *gin.Context) { c.Status(http.StatusFound) })
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		request := httptest.NewRequest(method, "/api/v1/group-buy/r/0123456789abcdef0123456789abcdef", nil)
		request.RemoteAddr = "198.51.100.52:42000"
		engine.ServeHTTP(httptest.NewRecorder(), request)
	}
	if output.Len() != 0 {
		t.Fatalf("group-buy redirect must not produce default access log: %q", output.String())
	}
}

func TestGroupBuyRedirectRateLimitIsPerEntryNotIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(RateLimit(1))
	engine.GET("/api/v1/group-buy/r/:code", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	for index, ip := range []string{"198.51.100.52:42000", "203.0.113.90:42000"} {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/group-buy/r/entry-a", nil)
		request.RemoteAddr = ip
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		expected := http.StatusNoContent
		if index == 1 {
			expected = http.StatusTooManyRequests
		}
		if response.Code != expected {
			t.Fatalf("request %d: expected %d, got %d", index, expected, response.Code)
		}
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/group-buy/r/entry-b", nil)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("separate entry should use a separate anonymous bucket: %s", strings.TrimSpace(response.Body.String()))
	}
}
