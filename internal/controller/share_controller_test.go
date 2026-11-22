package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/service"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestTrackShareScanReturnsOnlyPublicCampaignFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open synthetic share database: %v", err)
	}
	if err := db.AutoMigrate(&model.Merchant{}, &model.Store{}, &model.ShareCampaign{}); err != nil {
		t.Fatalf("migrate synthetic share schema: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get synthetic share database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	merchant := model.Merchant{
		Name:             "Synthetic Share Merchant",
		ContactPhone:     "13800000001",
		ContactEmail:     "merchant-private-email@example.invalid",
		SubscriptionNote: "merchant-private-subscription-note",
	}
	if err := db.Create(&merchant).Error; err != nil {
		t.Fatalf("create synthetic merchant: %v", err)
	}
	store := model.Store{
		MerchantID:   merchant.ID,
		Name:         "Synthetic Share Store",
		Address:      "store-private-address-北京市合成路1号",
		ContactPhone: "13800000002",
		Status:       "active",
	}
	if err := db.Create(&store).Error; err != nil {
		t.Fatalf("create synthetic store: %v", err)
	}
	campaign := model.ShareCampaign{
		MerchantID:       merchant.ID,
		StoreID:          store.ID,
		ShareCode:        "SYNTHETIC-SHARE-CODE-001",
		CustomerPhone:    "13800000003",
		PosterTitle:      "合成公开海报标题",
		PosterCopy:       "合成公开海报文案",
		ScanCount:        7,
		LeadCount:        2,
		ConversionCount:  1,
		ConversionAmount: 12345,
		Status:           "active",
	}
	if err := db.Create(&campaign).Error; err != nil {
		t.Fatalf("create synthetic share campaign: %v", err)
	}

	router := gin.New()
	router.POST("/share/:code/scan", NewShareController(service.NewShareService(db)).TrackShareScan)
	request := httptest.NewRequest(http.MethodPost, "/share/"+campaign.ShareCode+"/scan", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("scan response status=%d, want 200; body=%s", response.Code, response.Body.String())
	}

	var payload struct {
		Data struct {
			Campaign map[string]json.RawMessage `json:"campaign"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode scan response: %v (%s)", err, response.Body.String())
	}
	if got, want := len(payload.Data.Campaign), 3; got != want {
		t.Fatalf("public campaign contains %d fields, want exactly %d: %s", got, want, response.Body.String())
	}
	for key, want := range map[string]string{
		"share_code":   campaign.ShareCode,
		"poster_title": campaign.PosterTitle,
		"poster_copy":  campaign.PosterCopy,
	} {
		var got string
		if raw, ok := payload.Data.Campaign[key]; !ok {
			t.Errorf("public campaign is missing allowed field %q", key)
		} else if err := json.Unmarshal(raw, &got); err != nil || got != want {
			t.Errorf("public campaign field %q = %q, want %q (decode error: %v)", key, got, want, err)
		}
	}
	for _, privateValue := range []string{
		campaign.CustomerPhone,
		merchant.ContactPhone,
		merchant.ContactEmail,
		merchant.SubscriptionNote,
		store.Address,
		store.ContactPhone,
	} {
		if strings.Contains(response.Body.String(), privateValue) {
			t.Errorf("public scan response leaked private value %q: %s", privateValue, response.Body.String())
		}
	}
	for _, privateField := range []string{
		"customer_phone", "merchant_id", "store_id", "order_id", "scan_count", "lead_count",
		"conversion_count", "conversion_amount", "merchant", "store", "order", "created_at", "updated_at",
	} {
		if _, exists := payload.Data.Campaign[privateField]; exists {
			t.Errorf("public campaign must not include field %q", privateField)
		}
	}

	var updated model.ShareCampaign
	if err := db.First(&updated, campaign.ID).Error; err != nil {
		t.Fatalf("reload scanned campaign: %v", err)
	}
	if updated.ScanCount != campaign.ScanCount+1 {
		t.Errorf("scan_count after request=%d, want %d", updated.ScanCount, campaign.ScanCount+1)
	}
}
