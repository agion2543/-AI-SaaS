package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/model"
)

func newOpsDashboardTestService(t *testing.T) (*MerchantService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:ops-dashboard-test-"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open synthetic ops dashboard database: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Order{}, &model.OrderItem{}, &model.Store{}, &model.StoreProduct{},
		&model.RefundRecord{}, &model.GroupBuyManualRedemption{},
		&model.CampaignPostmortem{}, &model.AISuggestionExecution{},
		&model.MerchantAIUsageLog{},
	); err != nil {
		t.Fatalf("migrate synthetic ops dashboard tables: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get synthetic ops dashboard database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return NewMerchantService(&config.Config{}, db), db
}

func opsBeijingDay(t *testing.T, daysAgo int) time.Time {
	t.Helper()
	now := time.Now().In(beijingTimeZone)
	day := time.Date(now.Year(), now.Month(), now.Day(), 10, 0, 0, 0, beijingTimeZone)
	return day.AddDate(0, 0, -daysAgo)
}

func seedOpsDashboardOrders(t *testing.T, db *gorm.DB) {
	t.Helper()
	paid := func(daysAgo int) *time.Time {
		v := opsBeijingDay(t, daysAgo)
		return &v
	}
	mkOrder := func(phone, channel string, amount int64, daysAgo int, paidAt *time.Time) model.Order {
		return model.Order{
			OrderNo: "OPS-" + uuid.NewString()[:8], OrderType: "store_order",
			MerchantID: ptrUint(1), CustomerPhone: phone, Channel: channel,
			TotalAmount: amount, PaidAt: paidAt,
		}
	}
	today := opsBeijingDay(t, 0)
	_ = today
	orders := []model.Order{
		mkOrder("13800000001", "store_qr", 10000, 0, paid(0)),    // A, today
		mkOrder("13800000002", "table", 5000, 0, paid(0)),        // B, today, new
		mkOrder("13800000001", "share", 3000, 0, paid(0)),        // A again today
		mkOrder("13800000001", "store_qr", 8000, 1, paid(1)),     // A yesterday: not new
		mkOrder("13800000003", "store_qr", 6000, 1, paid(1)),     // C yesterday
		mkOrder("13800000004", "store_qr", 7000, 40, paid(40)),   // D, light dormant
		mkOrder("13800000005", "store_qr", 9000, 70, paid(70)),   // E, medium dormant
		mkOrder("13800000006", "store_qr", 4000, 100, paid(100)), // F, deep dormant
		mkOrder("13800000007", "store_qr", 9999, 0, nil),         // unpaid: excluded
		mkOrder("13800000008", "store_qr", 0, 0, paid(0)),        // zero amount: excluded
	}
	for i := range orders {
		if err := db.Create(&orders[i]).Error; err != nil {
			t.Fatalf("seed ops order %d: %v", i, err)
		}
	}
	// order_items for the three paid orders today + one 10 days ago.
	item := func(orderID, productID uint, name string, qty int, amount int64) model.OrderItem {
		return model.OrderItem{OrderID: orderID, ProductID: productID, ProductNameSnapshot: name, Quantity: qty, UnitPrice: amount / int64(qty), Amount: amount}
	}
	oldOrder := mkOrder("13800000009", "store_qr", 6000, 10, paid(10))
	if err := db.Create(&oldOrder).Error; err != nil {
		t.Fatalf("seed ops old order: %v", err)
	}
	items := []model.OrderItem{
		item(orders[0].ID, 1, "招牌汉堡", 5, 10000),
		item(orders[1].ID, 2, "薯条", 2, 5000),
		item(orders[2].ID, 3, "沙拉", 1, 3000),
		item(oldOrder.ID, 5, "饮料", 3, 6000),
	}
	for i := range items {
		if err := db.Create(&items[i]).Error; err != nil {
			t.Fatalf("seed ops order item %d: %v", i, err)
		}
	}
	if err := db.Create(&model.Store{MerchantID: 1, Name: "测试店"}).Error; err != nil {
		t.Fatalf("seed ops store: %v", err)
	}
	var store model.Store
	if err := db.Where("merchant_id = ?", 1).First(&store).Error; err != nil {
		t.Fatalf("read ops store: %v", err)
	}
	products := []model.StoreProduct{
		{StoreID: store.ID, Name: "招牌汉堡", Price: 2000, Status: "active"},
		{StoreID: store.ID, Name: "薯条", Price: 2500, Status: "active"},
		{StoreID: store.ID, Name: "沙拉", Price: 3000, Status: "active"},
		{StoreID: store.ID, Name: "老汤面", Price: 1500, Status: "active"},
		{StoreID: store.ID, Name: "饮料", Price: 2000, Status: "active"},
	}
	for i := range products {
		if err := db.Create(&products[i]).Error; err != nil {
			t.Fatalf("seed ops product %d: %v", i, err)
		}
	}
	// Link order_items to the seeded product ids (created in order 1..5).
	// The item rows above used product ids 1,2,3,5 already in creation order.
	// CreatedAt 显式写北京时间当日：GORM 自动值用的是本机时区（本机为 UTC），
	// 与日报的北京时间日范围对不上；生产约定与 MerchantTurnover 一致——
	// 服务跑在北京时间上，created_at 按北京时间归属。
	refundAt := opsBeijingDay(t, 0)
	if err := db.Create(&model.RefundRecord{
		BaseModel: model.BaseModel{CreatedAt: refundAt},
		OrderID:   orders[0].ID, MerchantID: ptrUint(1), RefundNo: "OPS-R1",
		Amount: 2000, Status: "success",
	}).Error; err != nil {
		t.Fatalf("seed ops refund: %v", err)
	}
	todayStr := time.Now().In(beijingTimeZone).Format("2006-01-02")
	if err := db.Create(&model.GroupBuyManualRedemption{
		MerchantID: 1, LinkID: 1, RedemptionDate: model.PlatformExpenseDate(todayStr),
		RedemptionCount: 3, RedemptionAmountCents: 15000, RecordedBy: 1,
	}).Error; err != nil {
		t.Fatalf("seed ops manual redemption: %v", err)
	}
	if err := db.Create(&model.CampaignPostmortem{
		MerchantID: 1, CampaignType: "promotion", CampaignID: 1,
		PeriodStart: model.PlatformExpenseDate("2026-11-01"), PeriodEnd: model.PlatformExpenseDate("2026-11-07"),
		TargetAmountCents: 100000, ActualAmountCents: 50000, ActualOrders: 5,
		GeneratedAt: time.Now(),
	}).Error; err != nil {
		t.Fatalf("seed ops postmortem: %v", err)
	}
}

func TestOpsDailySummaryNumbersAndSources(t *testing.T) {
	s, db := newOpsDashboardTestService(t)
	seedOpsDashboardOrders(t, db)
	todayStr := time.Now().In(beijingTimeZone).Format("2006-01-02")

	summary, err := s.OpsDailySummary(1, todayStr)
	if err != nil {
		t.Fatalf("daily summary: %v", err)
	}
	if summary.OrderCount != 3 {
		t.Fatalf("order count=%d, want 3 (unpaid and zero-amount excluded)", summary.OrderCount)
	}
	if summary.TradeAmount != 18000 {
		t.Fatalf("trade amount=%d, want 18000", summary.TradeAmount)
	}
	if summary.RefundAmount != 2000 || summary.NetAmount != 16000 {
		t.Fatalf("refund=%d net=%d, want 2000/16000", summary.RefundAmount, summary.NetAmount)
	}
	if summary.NewCustomers != 1 {
		t.Fatalf("new customers=%d, want 1 (only B; A traded yesterday)", summary.NewCustomers)
	}
	if summary.TradeSource != EvidenceSystemRecorded {
		t.Fatalf("trade source=%q, want 系统记录", summary.TradeSource)
	}
	if len(summary.ByChannel) != 3 {
		t.Fatalf("channels=%d, want 3", len(summary.ByChannel))
	}
	byChannel := map[string]int64{}
	for _, row := range summary.ByChannel {
		byChannel[row.Channel] = row.AmountCents
		if row.Source != EvidenceSystemRecorded {
			t.Fatalf("channel %s source=%q, want 系统记录", row.Channel, row.Source)
		}
	}
	if byChannel["store_qr"] != 10000 || byChannel["table"] != 5000 || byChannel["share"] != 3000 {
		t.Fatalf("channel amounts=%v, want store_qr 10000/table 5000/share 3000", byChannel)
	}
	if summary.ManualRedemptionCount != 3 || summary.ManualRedemptionAmountCents != 15000 {
		t.Fatalf("manual redemption=%d/%d, want 3/15000", summary.ManualRedemptionCount, summary.ManualRedemptionAmountCents)
	}
	if summary.ManualRedemptionSource != EvidenceMerchantInput {
		t.Fatalf("manual source=%q, want 商家自填", summary.ManualRedemptionSource)
	}
	// Merchant isolation: another merchant sees nothing.
	other, err := s.OpsDailySummary(2, todayStr)
	if err != nil {
		t.Fatalf("daily summary merchant 2: %v", err)
	}
	if other.OrderCount != 0 || other.TradeAmount != 0 {
		t.Fatalf("merchant 2 must see zero trades, got %d orders / %d", other.OrderCount, other.TradeAmount)
	}
}

func TestOpsProductHealthClassification(t *testing.T) {
	s, db := newOpsDashboardTestService(t)
	seedOpsDashboardOrders(t, db)

	health, err := s.OpsProductHealth(1, 30)
	if err != nil {
		t.Fatalf("product health: %v", err)
	}
	if health.Source != EvidenceSystemRecorded {
		t.Fatalf("health source=%q, want 系统记录", health.Source)
	}
	byName := map[string]OpsProductHealthRow{}
	for _, row := range health.Rows {
		byName[row.ProductName] = row
	}
	if got := byName["招牌汉堡"].Status; got != "bestseller" {
		t.Fatalf("burger status=%q, want bestseller", got)
	}
	if got := byName["饮料"].Status; got != "normal" {
		t.Fatalf("drink status=%q, want normal", got)
	}
	if got := byName["薯条"].Status; got != "slow" {
		t.Fatalf("fries status=%q, want slow", got)
	}
	if got := byName["沙拉"].Status; got != "slow" {
		t.Fatalf("salad status=%q, want slow", got)
	}
	if got := byName["老汤面"].Status; got != "unsold" {
		t.Fatalf("noodles status=%q, want unsold", got)
	}
	if byName["老汤面"].Quantity != 0 || byName["老汤面"].AmountCents != 0 {
		t.Fatalf("unsold row must be zero, got %+v", byName["老汤面"])
	}
	for _, row := range health.Rows {
		if row.Suggestion == "" || row.SuggestionFrom != EvidenceTemplate {
			t.Fatalf("row %s must carry a template suggestion, got %+v", row.ProductName, row)
		}
	}
}

func TestOpsDormantCustomersTiersAndMasking(t *testing.T) {
	s, db := newOpsDashboardTestService(t)
	seedOpsDashboardOrders(t, db)

	dormant, err := s.OpsDormantCustomers(1)
	if err != nil {
		t.Fatalf("dormant customers: %v", err)
	}
	byTier := map[string]OpsDormantTier{}
	for _, tier := range dormant.Tiers {
		byTier[tier.Tier] = tier
	}
	if byTier["light"].Count != 1 || byTier["medium"].Count != 1 || byTier["deep"].Count != 1 {
		t.Fatalf("tier counts light/medium/deep=%d/%d/%d, want 1/1/1",
			byTier["light"].Count, byTier["medium"].Count, byTier["deep"].Count)
	}
	for _, tier := range dormant.Tiers {
		for _, phone := range tier.SamplePhones {
			if len(phone) != len("138****0004") || phone[3:7] != "****" {
				t.Fatalf("phone %q must be masked as 138****0004 style", phone)
			}
		}
		if tier.Draft == "" || tier.DraftSource != EvidenceTemplate {
			t.Fatalf("tier %s must carry a template draft, got %+v", tier.Tier, tier)
		}
	}
	if byTier["light"].SamplePhones[0] != "138****0004" {
		t.Fatalf("light sample=%v, want 138****0004", byTier["light"].SamplePhones)
	}
}

func TestOpsContentScriptsTemplateFallback(t *testing.T) {
	s, _ := newOpsDashboardTestService(t)
	script, err := s.OpsContentScripts(context.Background(), 1, "new_dish")
	if err != nil {
		t.Fatalf("content script: %v", err)
	}
	if !script.Fallback || script.GeneratedBy != "template" {
		t.Fatalf("no model wired: must be template fallback, got %+v", script)
	}
	if script.Title != "新品上市" || len(script.Lines) == 0 || script.Caption == "" {
		t.Fatalf("template script incomplete: %+v", script)
	}
	if _, err := s.OpsContentScripts(context.Background(), 1, "nope"); err == nil {
		t.Fatalf("unknown scenario must fail")
	}
}

func TestGenerateOpsSuggestionsLoopAndDedup(t *testing.T) {
	s, db := newOpsDashboardTestService(t)
	seedOpsDashboardOrders(t, db)
	// Pre-seed one pending suggestion identical to one the generator would
	// create, to prove the second run skips instead of duplicating.
	if err := db.Create(&model.AISuggestionExecution{
		MerchantID: 1, Source: "system", Title: "处理滞销商品：薯条",
		ContentSnapshot: "seeded", Status: model.SuggestionExecutionPending,
	}).Error; err != nil {
		t.Fatalf("seed pending suggestion: %v", err)
	}

	first, err := s.GenerateOpsSuggestions(1)
	if err != nil {
		t.Fatalf("generate suggestions: %v", err)
	}
	// 3 product rows (fries skipped as duplicate, salad + noodles created) +
	// 3 dormant tiers + 1 postmortem follow-up = 6 created, 1 skipped.
	if first.Created != 6 {
		t.Fatalf("first run created=%d, want 6", first.Created)
	}
	if first.Skipped != 1 {
		t.Fatalf("first run skipped=%d, want 1", first.Skipped)
	}
	second, err := s.GenerateOpsSuggestions(1)
	if err != nil {
		t.Fatalf("generate suggestions second run: %v", err)
	}
	if second.Created != 0 {
		t.Fatalf("second run created=%d, want 0 (dedup)", second.Created)
	}
	if second.Skipped != 7 {
		t.Fatalf("second run skipped=%d, want 7", second.Skipped)
	}
	var pending int64
	if err := db.Model(&model.AISuggestionExecution{}).Where("merchant_id = ? AND status = ?", 1, "pending").Count(&pending).Error; err != nil {
		t.Fatalf("count pending: %v", err)
	}
	if pending != 7 {
		t.Fatalf("pending suggestions=%d, want 7", pending)
	}
}

func TestMaskPhone(t *testing.T) {
	if got := maskPhone("13800000004"); got != "138****0004" {
		t.Fatalf("maskPhone=%q, want 138****0004", got)
	}
	if got := maskPhone("123"); got != "***" {
		t.Fatalf("short maskPhone=%q, want ***", got)
	}
}
