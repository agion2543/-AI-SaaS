package service

import (
	"testing"
	"time"

	"gorm.io/gorm"

	"go-web-gin-health/internal/model"
)

func shareReportScopeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.ShareCampaign{}, &model.ReferralCoupon{}); err != nil {
		t.Fatalf("migrate share report test schema: %v", err)
	}
	return db
}

func TestShareStatsByMerchantFiltersCreationCohortAndSumsCampaignCounters(t *testing.T) {
	db := shareReportScopeTestDB(t)
	merchant := model.Merchant{Name: "合成报表商家", Status: "active"}
	if err := db.Create(&merchant).Error; err != nil {
		t.Fatalf("create synthetic merchant: %v", err)
	}
	otherMerchant := model.Merchant{Name: "另一合成商家", Status: "active"}
	if err := db.Create(&otherMerchant).Error; err != nil {
		t.Fatalf("create other synthetic merchant: %v", err)
	}
	store := model.Store{MerchantID: merchant.ID, Name: "合成报表门店", Status: "active", IsOpen: true}
	if err := db.Create(&store).Error; err != nil {
		t.Fatalf("create synthetic store: %v", err)
	}
	otherStore := model.Store{MerchantID: otherMerchant.ID, Name: "另一合成门店", Status: "active", IsOpen: true}
	if err := db.Create(&otherStore).Error; err != nil {
		t.Fatalf("create other synthetic store: %v", err)
	}

	start := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, time.January, 31, 23, 59, 59, 999999999, time.UTC)
	inside := time.Date(2025, time.January, 15, 12, 0, 0, 0, time.UTC)
	outside := time.Date(2024, time.December, 31, 12, 0, 0, 0, time.UTC)

	campaigns := []model.ShareCampaign{
		{
			BaseModel:  model.BaseModel{CreatedAt: inside, UpdatedAt: inside},
			MerchantID: merchant.ID, StoreID: store.ID, ShareCode: "SYNTHETIC-IN-RANGE", Status: "active",
			ScanCount: 7, LeadCount: 3, ConversionCount: 2, ConversionAmount: 12800,
		},
		{
			BaseModel:  model.BaseModel{CreatedAt: outside, UpdatedAt: outside},
			MerchantID: merchant.ID, StoreID: store.ID, ShareCode: "SYNTHETIC-OLD", Status: "active",
			ScanCount: 90, LeadCount: 40, ConversionCount: 18, ConversionAmount: 99000,
		},
		{
			BaseModel:  model.BaseModel{CreatedAt: inside, UpdatedAt: inside},
			MerchantID: otherMerchant.ID, StoreID: otherStore.ID, ShareCode: "SYNTHETIC-OTHER-MERCHANT", Status: "active",
			ScanCount: 1000, LeadCount: 500, ConversionCount: 300, ConversionAmount: 500000,
		},
	}
	for i := range campaigns {
		if err := db.Create(&campaigns[i]).Error; err != nil {
			t.Fatalf("create synthetic campaign %q: %v", campaigns[i].ShareCode, err)
		}
	}

	coupons := []model.ReferralCoupon{
		{BaseModel: model.BaseModel{CreatedAt: inside, UpdatedAt: inside}, MerchantID: merchant.ID, StoreID: store.ID, CouponNo: "SYNTHETIC-COUPON-USED", Status: "used"},
		{BaseModel: model.BaseModel{CreatedAt: inside, UpdatedAt: inside}, MerchantID: merchant.ID, StoreID: store.ID, CouponNo: "SYNTHETIC-COUPON-UNUSED", Status: "unused"},
		{BaseModel: model.BaseModel{CreatedAt: outside, UpdatedAt: outside}, MerchantID: merchant.ID, StoreID: store.ID, CouponNo: "SYNTHETIC-COUPON-OLD", Status: "used"},
		{BaseModel: model.BaseModel{CreatedAt: inside, UpdatedAt: inside}, MerchantID: otherMerchant.ID, StoreID: otherStore.ID, CouponNo: "SYNTHETIC-COUPON-OTHER", Status: "used"},
	}
	for i := range coupons {
		if err := db.Create(&coupons[i]).Error; err != nil {
			t.Fatalf("create synthetic coupon %q: %v", coupons[i].CouponNo, err)
		}
	}

	stats, listedCampaigns, listedCoupons, err := NewShareService(db).StatsByMerchant(merchant.ID, ShareStatsParams{StartAt: &start, EndAt: &end})
	if err != nil {
		t.Fatalf("load synthetic share stats: %v", err)
	}
	if stats.TotalCampaigns != 1 || stats.ScanCount != 7 || stats.LeadCount != 3 || stats.ConversionCount != 2 || stats.ConversionAmount != 12800 {
		t.Fatalf("date-filtered campaign stats = %+v; want only the in-range campaign's cumulative counters", stats)
	}
	if len(listedCampaigns) != 1 || listedCampaigns[0].ShareCode != "SYNTHETIC-IN-RANGE" {
		t.Fatalf("date-filtered campaigns = %+v; want only the in-range campaign", listedCampaigns)
	}
	if stats.RewardCouponCount != 2 || stats.UsedCouponCount != 1 || stats.UnusedCouponCount != 1 || len(listedCoupons) != 2 {
		t.Fatalf("date-filtered coupon stats = %+v with %d coupons; want the two in-range coupon records", stats, len(listedCoupons))
	}
}

func TestTrackScanCountsEachRequestWithoutDeduplication(t *testing.T) {
	db := shareReportScopeTestDB(t)
	merchant := model.Merchant{Name: "合成扫码计数商家", Status: "active"}
	if err := db.Create(&merchant).Error; err != nil {
		t.Fatalf("create synthetic merchant: %v", err)
	}
	store := model.Store{MerchantID: merchant.ID, Name: "合成扫码门店", Status: "active", IsOpen: true}
	if err := db.Create(&store).Error; err != nil {
		t.Fatalf("create synthetic store: %v", err)
	}
	campaign := model.ShareCampaign{
		MerchantID: merchant.ID,
		StoreID:    store.ID,
		ShareCode:  "SYNTHETIC-REPEATED-SCAN",
		Status:     "active",
	}
	if err := db.Create(&campaign).Error; err != nil {
		t.Fatalf("create synthetic campaign: %v", err)
	}

	shares := NewShareService(db)
	for i := 0; i < 2; i++ {
		if _, err := shares.TrackScan(campaign.ShareCode); err != nil {
			t.Fatalf("track synthetic scan request %d: %v", i+1, err)
		}
	}
	var stored model.ShareCampaign
	if err := db.First(&stored, campaign.ID).Error; err != nil {
		t.Fatalf("reload synthetic campaign: %v", err)
	}
	if stored.ScanCount != 2 || stored.LastScannedAt == nil {
		t.Fatalf("stored scan count/time = %d/%v; want two request counts and a last-scanned timestamp", stored.ScanCount, stored.LastScannedAt)
	}
}
