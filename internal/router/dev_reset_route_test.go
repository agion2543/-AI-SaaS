package router

import (
	"testing"

	"github.com/gin-gonic/gin"

	"go-web-gin-health/internal/config"
)

func TestDevResetPasswordRouteOnlyRegisteredInDevelopment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name                    string
		appEnv                  string
		devPasswordResetEnabled bool
		wantRoute               bool
	}{
		{name: "development explicitly selected", appEnv: "development", devPasswordResetEnabled: true, wantRoute: true},
		{name: "case and whitespace normalized", appEnv: " Development ", devPasswordResetEnabled: true, wantRoute: true},
		{name: "development default without explicit APP_ENV", appEnv: "development", wantRoute: false},
		{name: "staging", appEnv: "staging", devPasswordResetEnabled: true, wantRoute: false},
		{name: "prod alias", appEnv: "prod", devPasswordResetEnabled: true, wantRoute: false},
		{name: "production", appEnv: "production", devPasswordResetEnabled: true, wantRoute: false},
		{name: "empty", appEnv: "", devPasswordResetEnabled: true, wantRoute: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, closeDB := newClientIPTrustTestDB(t)
			defer closeDB()
			engine := gin.New()
			cfg := &config.Config{
				AppEnv:                  test.appEnv,
				DevPasswordResetEnabled: test.devPasswordResetEnabled,
				FrontendURL:             "http://localhost:5173",
				JWTSecret:               "synthetic-dev-reset-route-secret",
				RateLimitPerMinute:      1000,
			}
			Register(engine, db, cfg)

			found := false
			for _, route := range engine.Routes() {
				if route.Method == "POST" && route.Path == "/api/v1/auth/dev/reset-password" {
					found = true
					break
				}
			}
			if found != test.wantRoute {
				t.Fatalf("dev reset route registered=%v for APP_ENV=%q, want %v", found, test.appEnv, test.wantRoute)
			}
		})
	}
}
