package router

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"go-web-gin-health/internal/adminaccess"
	"go-web-gin-health/internal/config"
)

func TestEveryRegisteredAdminRouteHasExplicitAuthorizationMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open synthetic SQLite database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get synthetic SQL database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	engine := gin.New()
	Register(engine, db, &config.Config{JWTSecret: "synthetic-route-map-test-secret", FrontendURL: "http://localhost:5173", RateLimitPerMinute: 1000})

	checked := 0
	for _, route := range engine.Routes() {
		if !strings.HasPrefix(route.Path, "/api/admin/") && !strings.HasPrefix(route.Path, "/api/v1/admin/") {
			continue
		}
		if strings.HasSuffix(route.Path, "/auth/login") {
			continue
		}
		if _, ok := adminaccess.ResolveAdminRoute(route.Method, route.Path); !ok {
			t.Errorf("unmapped administrator endpoint: %s %s", route.Method, route.Path)
		}
		checked++
	}
	if checked < 20 {
		t.Fatalf("only checked %d registered admin routes; expected both admin API surfaces", checked)
	}
}
