package bootstrap

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/model"
)

func openDemoDatasetTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open synthetic SQLite database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get synthetic SQL database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(
		&model.AdminRole{}, &model.AdminUser{}, &model.MembershipPackage{}, &model.MerchantPlan{},
		&model.Merchant{}, &model.Store{}, &model.StoreProduct{},
		&model.Order{}, &model.OrderItem{}, &model.RefundRecord{},
		&model.CustomerLead{}, &model.ShareCampaign{}, &model.ReferralCoupon{},
		&model.Promotion{}, &model.CampaignTarget{}, &model.CampaignPostmortem{},
		&model.AISuggestionExecution{}, &model.GroupBuyManualRedemption{},
		&model.CustomerProfile{}, &model.MerchantGroupBuyLink{}, &model.MerchantGroupBuyLinkAudit{},
	); err != nil {
		t.Fatalf("migrate synthetic demo dataset tables: %v", err)
	}
	return db
}

func countDemoRows(t *testing.T, db *gorm.DB, modelValue interface{}, where string, args ...interface{}) int64 {
	t.Helper()
	var count int64
	query := db.Model(modelValue)
	if where != "" {
		query = query.Where(where, args...)
	}
	if err := query.Count(&count).Error; err != nil {
		t.Fatalf("count demo rows: %v", err)
	}
	return count
}

func demoDatasetCounts(t *testing.T, db *gorm.DB) map[string]int64 {
	t.Helper()
	return map[string]int64{
		"merchants":   countDemoRows(t, db, &model.Merchant{}, "name = ?", demoCafeMerchantName),
		"stores":      countDemoRows(t, db, &model.Store{}, "name = ?", demoCafeStoreName),
		"products":    countDemoRows(t, db, &model.StoreProduct{}, ""),
		"orders":      countDemoRows(t, db, &model.Order{}, ""),
		"order_items": countDemoRows(t, db, &model.OrderItem{}, ""),
		"profiles":    countDemoRows(t, db, &model.CustomerProfile{}, ""),
		"leads":       countDemoRows(t, db, &model.CustomerLead{}, ""),
		"coupons":     countDemoRows(t, db, &model.ReferralCoupon{}, ""),
		"campaigns":   countDemoRows(t, db, &model.ShareCampaign{}, ""),
		"promotions":  countDemoRows(t, db, &model.Promotion{}, ""),
		"targets":     countDemoRows(t, db, &model.CampaignTarget{}, ""),
		"postmortems": countDemoRows(t, db, &model.CampaignPostmortem{}, ""),
		"redemptions": countDemoRows(t, db, &model.GroupBuyManualRedemption{}, ""),
		"suggestions": countDemoRows(t, db, &model.AISuggestionExecution{}, ""),
		"refunds":     countDemoRows(t, db, &model.RefundRecord{}, ""),
		"group_links": countDemoRows(t, db, &model.MerchantGroupBuyLink{}, ""),
	}
}

func TestSeedDemoMerchantDatasetIsIdempotent(t *testing.T) {
	db := openDemoDatasetTestDB(t)

	if err := seedDemoMerchantDataset(db); err != nil {
		t.Fatalf("seed synthetic demo merchant dataset: %v", err)
	}
	before := demoDatasetCounts(t, db)
	if err := seedDemoMerchantDataset(db); err != nil {
		t.Fatalf("reseed synthetic demo merchant dataset: %v", err)
	}
	after := demoDatasetCounts(t, db)
	for key, want := range before {
		if after[key] != want {
			t.Errorf("reseed changed %s count: before=%d after=%d", key, want, after[key])
		}
	}

	// Fixed expected shape: 15 products, 16 orders (13 recent + 3
	// dormant), 28 detail rows, 15 profiles (pending seq 9 has none).
	want := map[string]int64{
		"merchants": 1, "stores": 1, "products": 15, "orders": 16,
		"order_items": 28, "profiles": 15, "leads": 6, "coupons": 4,
		"campaigns": 1, "promotions": 1, "targets": 1, "postmortems": 1,
		"redemptions": 1, "suggestions": 2, "refunds": 1, "group_links": 4,
	}
	for key, wantCount := range want {
		if after[key] != wantCount {
			t.Errorf("got %d %s, want %d", after[key], key, wantCount)
		}
	}

	// Demo merchant must be active with a live subscription (the H1
	// subscription gate blocks new store orders otherwise) and the store
	// must be submit_later (pay_first stalls at pending, P2 lesson).
	var merchant model.Merchant
	if err := db.Where("name = ?", demoCafeMerchantName).First(&merchant).Error; err != nil {
		t.Fatalf("load synthetic demo merchant: %v", err)
	}
	if merchant.Status != "active" {
		t.Errorf("demo merchant status=%q, want active", merchant.Status)
	}
	if merchant.SubscriptionStatus != "active" || merchant.SubscriptionExpireAt == nil || merchant.SubscriptionExpireAt.Before(time.Now().AddDate(0, 0, 7)) {
		t.Errorf("demo merchant subscription is not live: status=%q expire=%v", merchant.SubscriptionStatus, merchant.SubscriptionExpireAt)
	}
	var store model.Store
	if err := db.Where("merchant_id = ? AND name = ?", merchant.ID, demoCafeStoreName).First(&store).Error; err != nil {
		t.Fatalf("load synthetic demo store: %v", err)
	}
	if store.OrderMode != "submit_later" || store.Status != "active" || !store.IsOpen {
		t.Errorf("demo store mode=%q status=%q open=%v, want submit_later/active/true", store.OrderMode, store.Status, store.IsOpen)
	}

	// Paid-today numbers the daily summary reads: 3 orders (seq 11-13),
	// trade 5800+4000+3800=13600, refund 3800 (seq 13 fully refunded).
	var todayPaidCount int64
	var todayTrade int64
	today := time.Now().In(demoBeijingLocation)
	dayStart := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, demoBeijingLocation)
	dayEnd := dayStart.AddDate(0, 0, 1)
	if err := db.Model(&model.Order{}).
		Where("merchant_id = ? AND order_type = ? AND paid_at IS NOT NULL AND total_amount > 0", merchant.ID, "store_order").
		Where("paid_at >= ? AND paid_at < ?", dayStart, dayEnd).
		Count(&todayPaidCount).Error; err != nil {
		t.Fatalf("count synthetic today paid orders: %v", err)
	}
	if todayPaidCount != 3 {
		t.Errorf("got %d paid orders today, want 3", todayPaidCount)
	}
	if err := db.Model(&model.Order{}).
		Where("merchant_id = ? AND order_type = ? AND paid_at IS NOT NULL AND total_amount > 0", merchant.ID, "store_order").
		Where("paid_at >= ? AND paid_at < ?", dayStart, dayEnd).
		Select("COALESCE(SUM(total_amount),0)").Scan(&todayTrade).Error; err != nil {
		t.Fatalf("sum synthetic today trade: %v", err)
	}
	if todayTrade != 13600 {
		t.Errorf("got today trade %d cents, want 13600", todayTrade)
	}
	var refund model.RefundRecord
	if err := db.Where("refund_no = ?", "DEMO-REFUND-001").First(&refund).Error; err != nil {
		t.Fatalf("load synthetic demo refund: %v", err)
	}
	if refund.Status != "success" || refund.Amount != 3800 {
		t.Errorf("demo refund status=%q amount=%d, want success/3800", refund.Status, refund.Amount)
	}

	// Channels: 10 paid store_qr, 3 paid table, 2 paid share.
	if got := countDemoRows(t, db, &model.Order{}, "channel = ? AND paid_at IS NOT NULL", model.OrderChannelStoreQR); got != 10 {
		t.Errorf("got %d paid store_qr orders, want 10", got)
	}
	if got := countDemoRows(t, db, &model.Order{}, "channel = ? AND paid_at IS NOT NULL", model.OrderChannelTable); got != 3 {
		t.Errorf("got %d paid table orders, want 3", got)
	}
	if got := countDemoRows(t, db, &model.Order{}, "channel = ? AND paid_at IS NOT NULL", model.OrderChannelShare); got != 2 {
		t.Errorf("got %d paid share orders, want 2", got)
	}

	// Detail rows must add up to the gross order amounts (77800 cents
	// across all 16 orders; the 500-cent coupon discount is not a detail).
	var detailSum int64
	if err := db.Model(&model.OrderItem{}).Select("COALESCE(SUM(amount),0)").Scan(&detailSum).Error; err != nil {
		t.Fatalf("sum synthetic order detail amounts: %v", err)
	}
	if detailSum != 77800 {
		t.Errorf("got order detail sum %d cents, want 77800", detailSum)
	}

	// Dormant tiers read last_paid_at from the materialized profiles:
	// one customer per tier (45 / 75 / 120 days ago).
	var profiles []model.CustomerProfile
	if err := db.Where("merchant_id = ?", merchant.ID).Find(&profiles).Error; err != nil {
		t.Fatalf("load synthetic demo profiles: %v", err)
	}
	tiers := map[string]int{"light": 0, "medium": 0, "deep": 0}
	for _, p := range profiles {
		if p.LastPaidAt == nil {
			t.Errorf("profile %s has no last_paid_at", p.CustomerPhone)
			continue
		}
		last := p.LastPaidAt.In(demoBeijingLocation)
		lastDate := time.Date(last.Year(), last.Month(), last.Day(), 0, 0, 0, 0, demoBeijingLocation)
		days := int(dayStart.Sub(lastDate).Hours() / 24)
		switch {
		case days >= 31 && days <= 60:
			tiers["light"]++
		case days >= 61 && days <= 90:
			tiers["medium"]++
		case days >= 91:
			tiers["deep"]++
		}
	}
	for tier, wantCount := range map[string]int{"light": 1, "medium": 1, "deep": 1} {
		if tiers[tier] != wantCount {
			t.Errorf("dormant tier %s has %d customers, want %d", tier, tiers[tier], wantCount)
		}
	}

	// Postmortem actuals must match the orders in its 7-day period:
	// 12 paid orders, 58500 cents, 12 new customers.
	var postmortem model.CampaignPostmortem
	if err := db.Where("merchant_id = ?", merchant.ID).First(&postmortem).Error; err != nil {
		t.Fatalf("load synthetic demo postmortem: %v", err)
	}
	if postmortem.ActualOrders != 12 || postmortem.ActualAmountCents != 58500 || postmortem.ActualNewCustomers != 12 {
		t.Errorf("postmortem actuals orders=%d amount=%d new=%d, want 12/58500/12",
			postmortem.ActualOrders, postmortem.ActualAmountCents, postmortem.ActualNewCustomers)
	}
	if postmortem.TargetOrders != 15 || postmortem.TargetAmountCents != 80000 {
		t.Errorf("postmortem targets orders=%d amount=%d, want 15/80000",
			postmortem.TargetOrders, postmortem.TargetAmountCents)
	}

	// Share campaign + used coupon line up with the converted order.
	var campaign model.ShareCampaign
	if err := db.Where("share_code = ?", demoShareCode).First(&campaign).Error; err != nil {
		t.Fatalf("load synthetic demo share campaign: %v", err)
	}
	if campaign.ConversionCount != 1 || campaign.ConversionAmount != 4500 {
		t.Errorf("share campaign conversions=%d amount=%d, want 1/4500", campaign.ConversionCount, campaign.ConversionAmount)
	}
	var usedCoupon model.ReferralCoupon
	if err := db.Where("coupon_no = ?", "DEMO-CPN-003").First(&usedCoupon).Error; err != nil {
		t.Fatalf("load synthetic used demo coupon: %v", err)
	}
	if usedCoupon.Status != "used" || usedCoupon.OrderID == nil {
		t.Errorf("used demo coupon status=%q order=%v, want used/non-nil", usedCoupon.Status, usedCoupon.OrderID)
	}

	// Manual Douyin redemption: 3 redemptions, 15000 cents, merchant-entered.
	var redemption model.GroupBuyManualRedemption
	if err := db.Where("merchant_id = ?", merchant.ID).First(&redemption).Error; err != nil {
		t.Fatalf("load synthetic demo manual redemption: %v", err)
	}
	if redemption.RedemptionCount != 3 || redemption.RedemptionAmountCents != 15000 {
		t.Errorf("manual redemption count=%d amount=%d, want 3/15000", redemption.RedemptionCount, redemption.RedemptionAmountCents)
	}
}

func TestSeedDemoMerchantDatasetLeavesGroupBuyLinksUntouched(t *testing.T) {
	db := openDemoDatasetTestDB(t)
	if err := seedDemoGroupBuyLinks(db); err != nil {
		t.Fatalf("seed synthetic demo group-buy links: %v", err)
	}
	var before []model.MerchantGroupBuyLink
	if err := db.Order("id").Find(&before).Error; err != nil {
		t.Fatalf("load group-buy links before dataset: %v", err)
	}
	if err := seedDemoMerchantDataset(db); err != nil {
		t.Fatalf("seed synthetic demo merchant dataset: %v", err)
	}
	var after []model.MerchantGroupBuyLink
	if err := db.Order("id").Find(&after).Error; err != nil {
		t.Fatalf("load group-buy links after dataset: %v", err)
	}
	if len(before) != len(after) {
		t.Fatalf("dataset changed group-buy link count: before=%d after=%d", len(before), len(after))
	}
	for i := range before {
		// Compare by value: pointer fields (ReviewedBy etc.) differ by
		// address after a reload even when the rows are identical.
		if !demoGroupBuyLinkEqual(before[i], after[i]) {
			t.Errorf("dataset mutated group-buy link %d: before=%+v after=%+v", before[i].ID, before[i], after[i])
		}
	}
}

func demoGroupBuyLinkEqual(a, b model.MerchantGroupBuyLink) bool {
	if a.ID != b.ID || a.MerchantID != b.MerchantID || a.StoreID != b.StoreID ||
		a.Channel != b.Channel || a.DestinationURL != b.DestinationURL || a.Code != b.Code ||
		a.Enabled != b.Enabled || a.ClickCount != b.ClickCount ||
		a.ReviewStatus != b.ReviewStatus || a.ReviewReason != b.ReviewReason ||
		a.AdminSuspended != b.AdminSuspended || a.SuspensionReason != b.SuspensionReason ||
		!a.CreatedAt.Equal(b.CreatedAt) || !a.UpdatedAt.Equal(b.UpdatedAt) {
		return false
	}
	if (a.ReviewedBy == nil) != (b.ReviewedBy == nil) {
		return false
	}
	if a.ReviewedBy != nil && *a.ReviewedBy != *b.ReviewedBy {
		return false
	}
	if (a.ReviewedAt == nil) != (b.ReviewedAt == nil) {
		return false
	}
	if a.ReviewedAt != nil && !a.ReviewedAt.Equal(*b.ReviewedAt) {
		return false
	}
	if (a.LastClickedAt == nil) != (b.LastClickedAt == nil) {
		return false
	}
	if a.LastClickedAt != nil && !a.LastClickedAt.Equal(*b.LastClickedAt) {
		return false
	}
	if (a.SuspendedBy == nil) != (b.SuspendedBy == nil) {
		return false
	}
	if a.SuspendedBy != nil && *a.SuspendedBy != *b.SuspendedBy {
		return false
	}
	if (a.SuspendedAt == nil) != (b.SuspendedAt == nil) {
		return false
	}
	if a.SuspendedAt != nil && !a.SuspendedAt.Equal(*b.SuspendedAt) {
		return false
	}
	return true
}

func TestSeedDemoMerchantDatasetSkipsPartialSchema(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open synthetic SQLite database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get synthetic SQL database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&model.Merchant{}, &model.Store{}, &model.MerchantGroupBuyLink{}, &model.MerchantGroupBuyLinkAudit{}); err != nil {
		t.Fatalf("migrate partial synthetic schema: %v", err)
	}
	// Must not fail startup seeding on a partially migrated database.
	if err := seedDemoMerchantDataset(db); err != nil {
		t.Fatalf("seed on partial schema must skip quietly: %v", err)
	}
	if got := countDemoRows(t, db, &model.Merchant{}, ""); got != 0 {
		t.Errorf("partial-schema seed created %d merchants, want 0", got)
	}
}

func TestSeedDemoMerchantDatasetStrictEntryRequiresFullSchema(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open synthetic SQLite database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get synthetic SQL database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&model.Merchant{}); err != nil {
		t.Fatalf("migrate synthetic merchant table: %v", err)
	}
	if err := SeedDemoMerchantDataset(db); err == nil {
		t.Fatalf("strict demo seed on incomplete schema must fail")
	}
}

func TestSeedInitialDataProductionNeverSeedsDemoDataset(t *testing.T) {
	db := openDemoDatasetTestDB(t)
	cfg := &config.Config{
		AppEnv:                 "production",
		InitialAdminUser:       "synthetic-admin",
		InitialAdminPass:       "synthetic-initial-seed-password",
		InitialDataSeedEnabled: true,
	}
	if err := seedInitialData(db, cfg); err != nil {
		t.Fatalf("seed synthetic initial data in production: %v", err)
	}
	if got := countDemoRows(t, db, &model.Merchant{}, "name = ?", demoCafeMerchantName); got != 0 {
		t.Errorf("production seed created %d demo merchants, want 0", got)
	}
	if got := countDemoRows(t, db, &model.Order{}, ""); got != 0 {
		t.Errorf("production seed created %d demo orders, want 0", got)
	}
}
