package router_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/router"
)

// /health 不依赖数据库：路由注册时只做服务装配，请求本身不碰数据库。
// 覆盖“写死默认密码被删后路由仍可正常注册、健康检查仍可用”。
func TestHealthEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{FrontendURL: "http://localhost:5173", RateLimitPerMinute: 120}
	engine := gin.New()
	router.Register(engine, nil, cfg)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /health = %d, want %d", w.Code, http.StatusOK)
	}
}
