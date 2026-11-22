package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"go-web-gin-health/internal/config"
)

func TestRegisterUsesExplicitTrustedProxyBoundary(t *testing.T) {
	previousMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(previousMode) })

	tests := []struct {
		name           string
		trustedProxies []string
		remoteIP       string
		forwardedFor   string
		realIP         string
		wantClientIP   string
	}{
		{
			name:         "forwarding headers ignored by default",
			remoteIP:     "198.51.100.44",
			forwardedFor: "203.0.113.77",
			realIP:       "203.0.113.78",
			wantClientIP: "198.51.100.44",
		},
		{
			name:           "configured proxy supplies client address",
			trustedProxies: []string{"10.24.0.0/16"},
			remoteIP:       "10.24.2.9",
			forwardedFor:   "203.0.113.77",
			realIP:         "203.0.113.78",
			wantClientIP:   "203.0.113.77",
		},
		{
			name:           "untrusted peer cannot supply client address",
			trustedProxies: []string{"10.24.0.0/16"},
			remoteIP:       "198.51.100.44",
			forwardedFor:   "203.0.113.77",
			realIP:         "203.0.113.78",
			wantClientIP:   "198.51.100.44",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, closeDB := newClientIPTrustTestDB(t)
			defer closeDB()
			engine := gin.New()
			cfg := &config.Config{
				AppEnv:                         "development",
				FrontendURL:                    "http://localhost:5173",
				JWTSecret:                      strings.Repeat("j", 40),
				RateLimitPerMinute:             1000,
				OTPSendRateLimitPerIPPerMinute: config.DefaultOTPSendRateLimitPerIPPerMinute,
				TrustedProxies:                 test.trustedProxies,
			}
			Register(engine, db, cfg)
			engine.GET("/_synthetic/client-ip", func(c *gin.Context) { c.String(http.StatusOK, c.ClientIP()) })

			request := httptest.NewRequest(http.MethodGet, "/_synthetic/client-ip", nil)
			request.RemoteAddr = test.remoteIP + ":41414"
			request.Header.Set("X-Forwarded-For", test.forwardedFor)
			request.Header.Set("X-Real-IP", test.realIP)
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			if response.Code != http.StatusOK || strings.TrimSpace(response.Body.String()) != test.wantClientIP {
				t.Fatalf("client IP status=%d value=%q, want %q", response.Code, response.Body.String(), test.wantClientIP)
			}
		})
	}
}

func newClientIPTrustTestDB(t *testing.T) (*gorm.DB, func()) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open synthetic router test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get synthetic router test connection: %v", err)
	}
	return db, func() { _ = sqlDB.Close() }
}
