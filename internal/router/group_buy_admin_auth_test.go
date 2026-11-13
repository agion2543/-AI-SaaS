package router

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/utils"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAdminGroupBuyRoutesRequireAdminAndExposeNoMerchantAuditWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:group-buy-admin-route?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if err := db.AutoMigrate(&model.Merchant{}, &model.Store{}, &model.AdminUser{}, &model.MerchantGroupBuyLink{}, &model.MerchantGroupBuyLinkAudit{}); err != nil {
		t.Fatal(err)
	}
	merchant := model.Merchant{Name: "路由测试商家", Status: "active"}
	if err := db.Create(&merchant).Error; err != nil {
		t.Fatal(err)
	}
	store := model.Store{MerchantID: merchant.ID, Name: "路由测试门店", Status: "active"}
	if err := db.Create(&store).Error; err != nil {
		t.Fatal(err)
	}
	link := model.MerchantGroupBuyLink{MerchantID: merchant.ID, StoreID: store.ID, Channel: "douyin", DestinationURL: "https://life.douyin.com/route-test", Code: "00000000000000000000000000000011", Enabled: true, ReviewStatus: "pending"}
	if err := db.Create(&link).Error; err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{JWTSecret: "test-only-secret", FrontendURL: "http://localhost:5173", RateLimitPerMinute: 100}
	engine := gin.New()
	Register(engine, db, cfg)

	merchantID := merchant.ID
	merchantToken, err := utils.GenerateToken(cfg.JWTSecret, 202, "merchant_admin", "merchant_admin", &merchantID, 1)
	if err != nil {
		t.Fatal(err)
	}
	adminToken, err := utils.GenerateToken(cfg.JWTSecret, 7, "admin", "admin", nil, 1)
	if err != nil {
		t.Fatal(err)
	}

	request := func(method, path, token, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, req)
		return response
	}

	const listPath = "/api/v1/admin/group-buy-links"
	if got := request(http.MethodGet, listPath, "", "").Code; got != http.StatusUnauthorized {
		t.Fatalf("unauthenticated admin-list request status=%d, want 401", got)
	}
	if got := request(http.MethodGet, listPath, merchantToken, "").Code; got != http.StatusForbidden {
		t.Fatalf("merchant must not read platform link list: status=%d, want 403", got)
	}
	if got := request(http.MethodGet, listPath+"/"+uintString(link.ID), merchantToken, "").Code; got != http.StatusForbidden {
		t.Fatalf("merchant must not read platform review/audit detail: status=%d, want 403", got)
	}
	if got := request(http.MethodPut, listPath+"/"+uintString(link.ID)+"/review", merchantToken, `{"action":"approve"}`).Code; got != http.StatusForbidden {
		t.Fatalf("merchant must not modify review state: status=%d, want 403", got)
	}
	if got := request(http.MethodPut, listPath+"/"+uintString(link.ID)+"/audit", merchantToken, `{"action":"rewrite"}`).Code; got != http.StatusNotFound {
		t.Fatalf("there must be no audit-history mutation endpoint: status=%d, want 404", got)
	}
	if response := request(http.MethodGet, listPath, adminToken, ""); response.Code != http.StatusOK {
		t.Fatalf("admin should read platform link list: status=%d body=%s", response.Code, response.Body.String())
	}
	if response := request(http.MethodPut, listPath+"/"+uintString(link.ID)+"/review", adminToken, `{"action":"approve","reason":"已核对"}`); response.Code != http.StatusOK {
		t.Fatalf("admin should approve link: status=%d body=%s", response.Code, response.Body.String())
	}
	if response := request(http.MethodGet, listPath+"/"+uintString(link.ID), adminToken, ""); response.Code != http.StatusOK {
		t.Fatalf("admin should read audited detail: status=%d body=%s", response.Code, response.Body.String())
	}
}

func uintString(value uint) string {
	return strconv.FormatUint(uint64(value), 10)
}
