package service

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/repository"
)

func newSyntheticAICostDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open synthetic SQLite database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get synthetic SQLite database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(
		&model.Merchant{}, &model.MerchantAIUsageLog{},
		&model.AIModelPrice{}, &model.AIVendorBill{},
		&model.MerchantAIBudget{}, &model.MerchantAIBudgetAlert{},
		&model.AIEmergencySwitch{}, &model.AuditLog{},
	); err != nil {
		t.Fatalf("migrate synthetic AI cost schema: %v", err)
	}
	return db
}

func createSyntheticAICostMerchant(t *testing.T, db *gorm.DB, name string) model.Merchant {
	t.Helper()
	merchant := model.Merchant{Name: name, Status: "active"}
	if err := db.Create(&merchant).Error; err != nil {
		t.Fatalf("create synthetic merchant %q: %v", name, err)
	}
	return merchant
}

func insertSyntheticAIUsage(t *testing.T, db *gorm.DB, merchantID uint, storeID *uint, modelName, scenario string, prompt, completion, cached, total int, storedCost int64, usedAt time.Time) {
	t.Helper()
	log := model.MerchantAIUsageLog{
		MerchantID: merchantID, StoreID: storeID, Scenario: scenario, Model: modelName,
		Provider: "synthetic-provider", Success: true,
		PromptTokens: prompt, CompletionTokens: completion, CachedTokens: cached,
		TotalTokens: total, EstimatedCostCents: storedCost, UsedAt: usedAt,
	}
	if err := db.Create(&log).Error; err != nil {
		t.Fatalf("insert synthetic AI usage: %v", err)
	}
}

func syntheticSuperAdmin() AuditActor { return AuditActor{ID: 1, Type: "super_admin"} }

func saveSyntheticModelPrice(t *testing.T, admin *AdminService, modelName string, input, output, cached int64, effectiveFrom time.Time) {
	t.Helper()
	_, _, err := admin.SaveModelPrice(dto.SaveAIModelPriceRequest{
		Model: modelName, Provider: "synthetic-provider",
		InputPriceCentsPerMillion: input, OutputPriceCentsPerMillion: output,
		CachedPriceCentsPerMillion: cached, Currency: "CNY",
		EffectiveFrom: effectiveFrom.Format("2006-01-02 15:04:05"), IsActive: true,
	}, syntheticSuperAdmin())
	if err != nil {
		t.Fatalf("save synthetic model price %q: %v", modelName, err)
	}
}

func TestAIModelPriceEstimateSplitsInputOutputCached(t *testing.T) {
	db := newSyntheticAICostDB(t)
	guard := NewAICostGuard(db)
	now := time.Now()
	saveSyntheticModelPrice(t, NewAdminService(&config.Config{}, db), "synthetic-model", 100, 400, 10, now.Add(-time.Hour))

	cost, currency, priced, err := guard.EstimateAICostCents("synthetic-provider", "synthetic-model", 1_000_000, 500_000, 1_000_000, now)
	if err != nil || !priced || currency != "CNY" || cost != 310 {
		t.Fatalf("estimate=%d currency=%q priced=%v err=%v, want 310/CNY/true", cost, currency, priced, err)
	}
	if _, _, priced, err := guard.EstimateAICostCents("", "unknown-model", 100, 100, 0, now); err != nil || priced {
		t.Fatalf("unknown model must be unpriced: priced=%v err=%v", priced, err)
	}
}

func TestAIModelPriceEffectiveTimeSelectsLatest(t *testing.T) {
	db := newSyntheticAICostDB(t)
	admin := NewAdminService(&config.Config{}, db)
	guard := NewAICostGuard(db)
	now := time.Now()
	saveSyntheticModelPrice(t, admin, "synthetic-model", 100, 100, 0, now.AddDate(0, 0, -10))
	saveSyntheticModelPrice(t, admin, "synthetic-model", 300, 300, 0, now.AddDate(0, 0, -1))

	cost, _, priced, err := guard.EstimateAICostCents("", "synthetic-model", 1_000_000, 0, 0, now)
	if err != nil || !priced || cost != 300 {
		t.Fatalf("latest price estimate=%d priced=%v err=%v, want 300", cost, priced, err)
	}
	cost, _, priced, err = guard.EstimateAICostCents("", "synthetic-model", 1_000_000, 0, 0, now.AddDate(0, 0, -5))
	if err != nil || !priced || cost != 100 {
		t.Fatalf("older price estimate=%d priced=%v err=%v, want 100", cost, priced, err)
	}
}

func TestAIModelPriceSaveUpdatesSameEffectiveRow(t *testing.T) {
	db := newSyntheticAICostDB(t)
	admin := NewAdminService(&config.Config{}, db)
	now := time.Now().Truncate(time.Second)
	req := dto.SaveAIModelPriceRequest{
		Model: "synthetic-model", Provider: "synthetic-provider",
		InputPriceCentsPerMillion: 100, OutputPriceCentsPerMillion: 200,
		CachedPriceCentsPerMillion: 10, Currency: "CNY",
		EffectiveFrom: now.Format("2006-01-02 15:04:05"), IsActive: true,
	}
	row, created, err := admin.SaveModelPrice(req, syntheticSuperAdmin())
	if err != nil || !created || row.InputPriceCentsPerMillion != 100 {
		t.Fatalf("first save: created=%v row=%+v err=%v", created, row, err)
	}
	req.InputPriceCentsPerMillion = 150
	row, created, err = admin.SaveModelPrice(req, syntheticSuperAdmin())
	if err != nil || created || row.InputPriceCentsPerMillion != 150 {
		t.Fatalf("same effective time must update in place: created=%v row=%+v err=%v", created, row, err)
	}
	var count int64
	if err := db.Model(&model.AIModelPrice{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("price rows=%d err=%v, want 1", count, err)
	}
	if _, _, err := admin.SaveModelPrice(dto.SaveAIModelPriceRequest{
		Model: "synthetic-model", Currency: "USD", EffectiveFrom: now.Format("2006-01-02 15:04:05"),
	}, syntheticSuperAdmin()); !errors.Is(err, ErrInvalidAICostControl) {
		t.Fatalf("non-CNY price must be rejected: err=%v", err)
	}
}

func TestAIUsageStatsAggregatesByMerchantModelScenarioStore(t *testing.T) {
	db := newSyntheticAICostDB(t)
	admin := NewAdminService(&config.Config{}, db)
	guard := NewAICostGuard(db)
	now := time.Now()
	merchantA := createSyntheticAICostMerchant(t, db, "synthetic-merchant-a")
	merchantB := createSyntheticAICostMerchant(t, db, "synthetic-merchant-b")
	saveSyntheticModelPrice(t, admin, "cheap-model", 100, 200, 10, now.Add(-time.Hour))
	storeID := uint(7)
	// A: cheap-model, 1M prompt + 1M completion + 1M cached, stored 999.
	// B: unpriced model, stored 555. 口径统一后统计直接用存量和 999+555=1554，
	// 不再按价格表重算（旧口径为 310+555=865）。
	insertSyntheticAIUsage(t, db, merchantA.ID, &storeID, "cheap-model", "scenario-one", 1_000_000, 1_000_000, 1_000_000, 3_000_000, 999, now)
	// B: unpriced model falls back to the stored estimate 555.
	insertSyntheticAIUsage(t, db, merchantB.ID, nil, "unpriced-model", "scenario-two", 100, 200, 0, 300, 555, now)

	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	stats, err := guard.AIUsageStats(dayStart, dayStart.AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("usage stats: %v", err)
	}
	if stats.TotalRequests != 2 || stats.TotalPromptTokens != 1_000_100 || stats.TotalCompletionTokens != 1_000_200 ||
		stats.TotalCachedTokens != 1_000_000 || stats.TotalTokens != 3_000_300 || stats.TotalEstimatedCostCents != 1554 {
		t.Fatalf("totals mismatch: %+v, want requests=2 prompt=1000100 completion=1000200 cached=1000000 total=3000300 cost=1554", stats)
	}
	if stats.UnpricedFallbackEstimatedCents != 0 {
		t.Fatalf("unpriced fallback=%d, want 0 (stored-cost convention has no fallback branch)", stats.UnpricedFallbackEstimatedCents)
	}
	if len(stats.ByMerchant) != 2 || stats.ByMerchant[0].MerchantID != merchantA.ID || stats.ByMerchant[0].EstimatedCostCents != 999 {
		t.Fatalf("by-merchant ranking mismatch: %+v", stats.ByMerchant)
	}
	if len(stats.ByModel) != 2 || stats.ByModel[0].Model != "cheap-model" {
		t.Fatalf("by-model mismatch: %+v", stats.ByModel)
	}
	if len(stats.ByScenario) != 2 {
		t.Fatalf("by-scenario rows=%d, want 2", len(stats.ByScenario))
	}
	if len(stats.ByStore) != 2 {
		t.Fatalf("by-store rows=%d, want 2 (store 7 + unattributed 0)", len(stats.ByStore))
	}
	for _, row := range stats.ByStore {
		if row.StoreID == 7 && (row.PromptTokens != 1_000_000 || row.CachedTokens != 1_000_000) {
			t.Fatalf("store 7 row mismatch: %+v", row)
		}
		if row.StoreID == 0 && row.RequestCount != 1 {
			t.Fatalf("unattributed store row mismatch: %+v", row)
		}
	}
}

func TestAIBudgetAlertsFireOncePerThreshold(t *testing.T) {
	db := newSyntheticAICostDB(t)
	admin := NewAdminService(&config.Config{}, db)
	guard := NewAICostGuard(db)
	merchant := createSyntheticAICostMerchant(t, db, "synthetic-alert-merchant")
	if _, err := admin.SaveMerchantBudget(merchant.ID, dto.SaveMerchantAIBudgetRequest{
		DailyBudgetCents: 1000, MonthlyBudgetCents: 0, OverLimitAction: model.AIBudgetOverLimitStop,
	}, syntheticSuperAdmin()); err != nil {
		t.Fatalf("save budget: %v", err)
	}
	now := time.Now()
	insertSyntheticAIUsage(t, db, merchant.ID, nil, "m", "s", 100, 100, 0, 200, 750, now)
	if _, err := guard.EvaluateMerchantBudget(merchant.ID); err != nil {
		t.Fatalf("evaluate at 75%%: %v", err)
	}
	assertAlertCount(t, db, merchant.ID, 1)

	insertSyntheticAIUsage(t, db, merchant.ID, nil, "m", "s", 100, 100, 0, 200, 200, now)
	if _, err := guard.EvaluateMerchantBudget(merchant.ID); err != nil {
		t.Fatalf("evaluate at 95%%: %v", err)
	}
	assertAlertCount(t, db, merchant.ID, 2)

	insertSyntheticAIUsage(t, db, merchant.ID, nil, "m", "s", 100, 100, 0, 200, 100, now)
	status, err := guard.EvaluateMerchantBudget(merchant.ID)
	if err != nil {
		t.Fatalf("evaluate at 105%%: %v", err)
	}
	if !status.OverLimit || status.DailyRatioPercent != 105 {
		t.Fatalf("status mismatch: %+v", status)
	}
	assertAlertCount(t, db, merchant.ID, 3)
	// Re-evaluating must not duplicate alerts.
	if _, err := guard.EvaluateMerchantBudget(merchant.ID); err != nil {
		t.Fatalf("re-evaluate: %v", err)
	}
	assertAlertCount(t, db, merchant.ID, 3)

	var alert model.MerchantAIBudgetAlert
	if err := db.Where("merchant_id = ? AND threshold = ?", merchant.ID, 100).Take(&alert).Error; err != nil {
		t.Fatalf("read 100%% alert: %v", err)
	}
	if alert.PeriodType != "daily" || alert.UsedTokens != 600 || alert.EstimatedCostCents != 1050 {
		t.Fatalf("100%% alert mismatch: %+v", alert)
	}
}

func assertAlertCount(t *testing.T, db *gorm.DB, merchantID uint, want int64) {
	t.Helper()
	var count int64
	if err := db.Model(&model.MerchantAIBudgetAlert{}).Where("merchant_id = ?", merchantID).Count(&count).Error; err != nil {
		t.Fatalf("count alerts: %v", err)
	}
	if count != want {
		t.Fatalf("alert count=%d, want %d", count, want)
	}
}

func setSyntheticBudget(t *testing.T, admin *AdminService, merchantID uint, daily, monthly int64, action string) {
	t.Helper()
	if _, err := admin.SaveMerchantBudget(merchantID, dto.SaveMerchantAIBudgetRequest{
		DailyBudgetCents: daily, MonthlyBudgetCents: monthly, OverLimitAction: action,
	}, syntheticSuperAdmin()); err != nil {
		t.Fatalf("save budget for merchant %d: %v", merchantID, err)
	}
}

func TestAIBudgetOverLimitStopDenies(t *testing.T) {
	db := newSyntheticAICostDB(t)
	admin := NewAdminService(&config.Config{}, db)
	guard := NewAICostGuard(db)
	merchant := createSyntheticAICostMerchant(t, db, "synthetic-stop-merchant")
	setSyntheticBudget(t, admin, merchant.ID, 100, 0, model.AIBudgetOverLimitStop)
	insertSyntheticAIUsage(t, db, merchant.ID, nil, "m", "s", 10, 10, 0, 20, 100, time.Now())

	decision, err := guard.CheckMerchantAIAccess(merchant.ID, "s")
	if !errors.Is(err, ErrAIBudgetExceeded) || decision.Allowed {
		t.Fatalf("over-limit stop must deny: decision=%+v err=%v", decision, err)
	}
}

func TestAIBudgetOverLimitDowngradePicksCheapestModel(t *testing.T) {
	db := newSyntheticAICostDB(t)
	admin := NewAdminService(&config.Config{}, db)
	guard := NewAICostGuard(db)
	merchant := createSyntheticAICostMerchant(t, db, "synthetic-downgrade-merchant")
	now := time.Now()
	saveSyntheticModelPrice(t, admin, "expensive-model", 1000, 4000, 0, now.Add(-time.Hour))
	saveSyntheticModelPrice(t, admin, "cheap-model", 10, 20, 0, now.Add(-time.Hour))
	setSyntheticBudget(t, admin, merchant.ID, 100, 0, model.AIBudgetOverLimitDowngrade)
	insertSyntheticAIUsage(t, db, merchant.ID, nil, "m", "s", 10, 10, 0, 20, 100, now)

	decision, err := guard.CheckMerchantAIAccess(merchant.ID, "s")
	if err != nil || !decision.Allowed || decision.Action != "allow_downgraded" || decision.DowngradeModel != "cheap-model" {
		t.Fatalf("downgrade decision mismatch: %+v err=%v", decision, err)
	}
}

func TestAIBudgetOverLimitDowngradeWithoutPricesDenies(t *testing.T) {
	db := newSyntheticAICostDB(t)
	admin := NewAdminService(&config.Config{}, db)
	guard := NewAICostGuard(db)
	merchant := createSyntheticAICostMerchant(t, db, "synthetic-no-price-merchant")
	setSyntheticBudget(t, admin, merchant.ID, 100, 0, model.AIBudgetOverLimitDowngrade)
	insertSyntheticAIUsage(t, db, merchant.ID, nil, "m", "s", 10, 10, 0, 20, 100, time.Now())

	_, err := guard.CheckMerchantAIAccess(merchant.ID, "s")
	if !errors.Is(err, ErrAINoDowngradeModel) {
		t.Fatalf("downgrade without prices must deny: err=%v", err)
	}
}

func TestAIBudgetOverLimitRestrictBlocksOnlyHighCostScenarios(t *testing.T) {
	db := newSyntheticAICostDB(t)
	admin := NewAdminService(&config.Config{}, db)
	guard := NewAICostGuard(db)
	merchant := createSyntheticAICostMerchant(t, db, "synthetic-restrict-merchant")
	setSyntheticBudget(t, admin, merchant.ID, 100, 0, model.AIBudgetOverLimitRestrictExpensive)
	insertSyntheticAIUsage(t, db, merchant.ID, nil, "m", "s", 10, 10, 0, 20, 100, time.Now())

	if _, err := guard.CheckMerchantAIAccess(merchant.ID, "referral_multi_copy"); !errors.Is(err, ErrAIHighCostRestricted) {
		t.Fatalf("high-cost scenario must be denied: err=%v", err)
	}
	if _, err := guard.CheckMerchantAIAccess(merchant.ID, "share_campaign_review"); !errors.Is(err, ErrAIHighCostRestricted) {
		t.Fatalf("high-cost review scenario must be denied: err=%v", err)
	}
	decision, err := guard.CheckMerchantAIAccess(merchant.ID, "裂变海报")
	if err != nil || !decision.Allowed || decision.Action != "allow_restricted" {
		t.Fatalf("low-cost scenario must be allowed: %+v err=%v", decision, err)
	}
}

func TestAIBudgetOverLimitManualReviewDenies(t *testing.T) {
	db := newSyntheticAICostDB(t)
	admin := NewAdminService(&config.Config{}, db)
	guard := NewAICostGuard(db)
	merchant := createSyntheticAICostMerchant(t, db, "synthetic-review-merchant")
	setSyntheticBudget(t, admin, merchant.ID, 100, 0, model.AIBudgetOverLimitManualReview)
	insertSyntheticAIUsage(t, db, merchant.ID, nil, "m", "s", 10, 10, 0, 20, 100, time.Now())

	if _, err := guard.CheckMerchantAIAccess(merchant.ID, "s"); !errors.Is(err, ErrAIBudgetManualReview) {
		t.Fatalf("manual review strategy must deny: err=%v", err)
	}
}

func TestAIBudgetGlobalDefaultAppliesWithoutMerchantRow(t *testing.T) {
	db := newSyntheticAICostDB(t)
	admin := NewAdminService(&config.Config{}, db)
	guard := NewAICostGuard(db)
	merchant := createSyntheticAICostMerchant(t, db, "synthetic-global-merchant")
	setSyntheticBudget(t, admin, 0, 100, 0, model.AIBudgetOverLimitStop)
	insertSyntheticAIUsage(t, db, merchant.ID, nil, "m", "s", 10, 10, 0, 20, 100, time.Now())

	status, budget, err := admin.MerchantBudgetStatus(merchant.ID)
	if err != nil || budget == nil || !status.GlobalDefault || !status.OverLimit {
		t.Fatalf("global default must apply: status=%+v budget=%+v err=%v", status, budget, err)
	}
	if _, err := guard.CheckMerchantAIAccess(merchant.ID, "s"); !errors.Is(err, ErrAIBudgetExceeded) {
		t.Fatalf("global default over-limit must deny: err=%v", err)
	}
}

func TestAIAccessAllowedWithoutBudget(t *testing.T) {
	db := newSyntheticAICostDB(t)
	guard := NewAICostGuard(db)
	merchant := createSyntheticAICostMerchant(t, db, "synthetic-free-merchant")
	decision, err := guard.CheckMerchantAIAccess(merchant.ID, "s")
	if err != nil || !decision.Allowed || decision.Action != "allow" {
		t.Fatalf("no budget must allow: %+v err=%v", decision, err)
	}
}

func TestAIEmergencySwitchDeniesAndRestores(t *testing.T) {
	db := newSyntheticAICostDB(t)
	admin := NewAdminService(&config.Config{}, db)
	guard := NewAICostGuard(db)
	merchant := createSyntheticAICostMerchant(t, db, "synthetic-switch-merchant")

	disabled, _, err := guard.GetAIEmergencySwitch()
	if err != nil || disabled {
		t.Fatalf("default switch must be enabled: disabled=%v err=%v", disabled, err)
	}
	row, err := admin.SetAIEmergencySwitch(true, "synthetic emergency", syntheticSuperAdmin())
	if err != nil || !row.Disabled {
		t.Fatalf("disable switch: row=%+v err=%v", row, err)
	}
	if _, err := guard.CheckMerchantAIAccess(merchant.ID, "s"); !errors.Is(err, ErrAIEmergencyDisabled) {
		t.Fatalf("disabled switch must deny AI: err=%v", err)
	}
	var auditCount int64
	if err := db.Model(&model.AuditLog{}).Where("action = ?", "ai_emergency_switch_disable").Count(&auditCount).Error; err != nil || auditCount != 1 {
		t.Fatalf("switch change must be audited: count=%d err=%v", auditCount, err)
	}
	if _, err := admin.SetAIEmergencySwitch(false, "synthetic recovery", syntheticSuperAdmin()); err != nil {
		t.Fatalf("re-enable switch: %v", err)
	}
	decision, err := guard.CheckMerchantAIAccess(merchant.ID, "s")
	if err != nil || !decision.Allowed {
		t.Fatalf("re-enabled switch must allow: %+v err=%v", decision, err)
	}
}

func TestAIVendorBillReconciliationSeparatesEstimateFromActual(t *testing.T) {
	db := newSyntheticAICostDB(t)
	admin := NewAdminService(&config.Config{}, db)
	guard := NewAICostGuard(db)
	now := time.Now()
	merchant := createSyntheticAICostMerchant(t, db, "synthetic-bill-merchant")
	saveSyntheticModelPrice(t, admin, "cheap-model", 100, 200, 10, now.Add(-time.Hour))
	insertSyntheticAIUsage(t, db, merchant.ID, nil, "cheap-model", "s", 1_000_000, 1_000_000, 1_000_000, 3_000_000, 999, now)

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	bill, created, err := admin.CreateAIVendorBill(dto.CreateAIVendorBillRequest{
		Vendor: "synthetic-vendor", PeriodStart: today.Format("2006-01-02"), PeriodEnd: today.Format("2006-01-02"),
		AmountCents: 500, Currency: "CNY", Note: "synthetic bill", IdempotencyKey: uuid.NewString(),
	}, syntheticSuperAdmin())
	if err != nil || !created || bill.AmountCents != 500 {
		t.Fatalf("create bill: created=%v bill=%+v err=%v", created, bill, err)
	}
	// Same key + same payload replays without a second row.
	_, created, err = admin.CreateAIVendorBill(dto.CreateAIVendorBillRequest{
		Vendor: "synthetic-vendor", PeriodStart: today.Format("2006-01-02"), PeriodEnd: today.Format("2006-01-02"),
		AmountCents: 500, Currency: "CNY", Note: "synthetic bill", IdempotencyKey: bill.IdempotencyKey,
	}, syntheticSuperAdmin())
	if err != nil || created {
		t.Fatalf("idempotent replay must not create: created=%v err=%v", created, err)
	}
	// Same key + different payload is a conflict.
	if _, _, err := admin.CreateAIVendorBill(dto.CreateAIVendorBillRequest{
		Vendor: "synthetic-vendor", PeriodStart: today.Format("2006-01-02"), PeriodEnd: today.Format("2006-01-02"),
		AmountCents: 501, Currency: "CNY", Note: "synthetic bill", IdempotencyKey: bill.IdempotencyKey,
	}, syntheticSuperAdmin()); !errors.Is(err, ErrAIVendorBillKeyConflict) {
		t.Fatalf("conflicting replay must fail: err=%v", err)
	}

	result, err := admin.ReconcileAIBills(today, today)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	// Estimated 999 (stored cost on the usage record) vs actual 500 (manual bill):
	// difference -499. 口径统一后核对侧估算取存量，不再按价格表重算（旧口径为 310）。
	if result.EstimatedCostCents != 999 || result.ActualBillTotalCents != 500 || result.DifferenceCents != -499 || result.ActualBillCount != 1 {
		t.Fatalf("reconciliation mismatch: %+v", result)
	}
	if result.UnpricedEstimatedCents != 0 {
		t.Fatalf("unpriced fallback=%d, want 0 (stored-cost convention has no fallback branch)", result.UnpricedEstimatedCents)
	}
	_ = guard
}

func TestSaveMerchantBudgetValidation(t *testing.T) {
	db := newSyntheticAICostDB(t)
	admin := NewAdminService(&config.Config{}, db)
	merchant := createSyntheticAICostMerchant(t, db, "synthetic-validation-merchant")

	if _, err := admin.SaveMerchantBudget(merchant.ID, dto.SaveMerchantAIBudgetRequest{
		DailyBudgetCents: -1, OverLimitAction: model.AIBudgetOverLimitStop,
	}, syntheticSuperAdmin()); !errors.Is(err, ErrInvalidAICostControl) {
		t.Fatalf("negative budget must be rejected: err=%v", err)
	}
	if _, err := admin.SaveMerchantBudget(merchant.ID, dto.SaveMerchantAIBudgetRequest{
		DailyBudgetCents: 100, OverLimitAction: "explode",
	}, syntheticSuperAdmin()); !errors.Is(err, ErrInvalidAICostControl) {
		t.Fatalf("unknown action must be rejected: err=%v", err)
	}
	if _, err := admin.SaveMerchantBudget(999999, dto.SaveMerchantAIBudgetRequest{
		DailyBudgetCents: 100, OverLimitAction: model.AIBudgetOverLimitStop,
	}, syntheticSuperAdmin()); !errors.Is(err, ErrInvalidAICostControl) {
		t.Fatalf("unknown merchant must be rejected: err=%v", err)
	}
	if _, err := admin.SaveMerchantBudget(merchant.ID, dto.SaveMerchantAIBudgetRequest{
		DailyBudgetCents: 100, OverLimitAction: model.AIBudgetOverLimitStop,
	}, AuditActor{ID: 2, Type: "admin"}); !errors.Is(err, ErrInvalidAICostControl) {
		t.Fatalf("non-super admin must be rejected: err=%v", err)
	}
	// Global default row (merchant_id=0) is allowed.
	if _, err := admin.SaveMerchantBudget(0, dto.SaveMerchantAIBudgetRequest{
		DailyBudgetCents: 100, OverLimitAction: model.AIBudgetOverLimitStop,
	}, syntheticSuperAdmin()); err != nil {
		t.Fatalf("global default budget must save: %v", err)
	}
}

// --- 估算口径统一（2026-11-28）：统计/核对只用调用记录上存的估算成本，
// 价格表事后改价、补录、停用均不改写已记成本，与预算执行口径一致。 ---

func saveSyntheticModelPriceForProvider(t *testing.T, admin *AdminService, provider, modelName string, input, output, cached int64, effectiveFrom time.Time) {
	t.Helper()
	_, _, err := admin.SaveModelPrice(dto.SaveAIModelPriceRequest{
		Model: modelName, Provider: provider,
		InputPriceCentsPerMillion: input, OutputPriceCentsPerMillion: output,
		CachedPriceCentsPerMillion: cached, Currency: "CNY",
		EffectiveFrom: effectiveFrom.Format("2006-01-02 15:04:05"), IsActive: true,
	}, syntheticSuperAdmin())
	if err != nil {
		t.Fatalf("save synthetic model price %q/%q: %v", provider, modelName, err)
	}
}

func syntheticDayStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// 场景 1：事后新增更晚生效价，历史统计不变。
func TestAIUsageStatsIgnoresLaterPriceChange(t *testing.T) {
	db := newSyntheticAICostDB(t)
	admin := NewAdminService(&config.Config{}, db)
	guard := NewAICostGuard(db)
	now := time.Now()
	dayStart := syntheticDayStart(now)
	merchant := createSyntheticAICostMerchant(t, db, "synthetic-late-price-merchant")
	saveSyntheticModelPrice(t, admin, "m", 100, 200, 10, dayStart.AddDate(0, 0, -1))
	// 上午调用：1M 输入 Token，存量估算 100 分。
	insertSyntheticAIUsage(t, db, merchant.ID, nil, "m", "s", 1_000_000, 0, 0, 1_000_000, 100, dayStart.Add(10*time.Hour))
	// 午后新增更晚生效价 300 分/百万输入 Token：旧重算口径会把该日改为 300 分。
	saveSyntheticModelPrice(t, admin, "m", 300, 600, 30, dayStart.Add(12*time.Hour))

	stats, err := guard.AIUsageStats(dayStart, dayStart.AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("usage stats: %v", err)
	}
	if stats.TotalEstimatedCostCents != 100 || stats.UnpricedFallbackEstimatedCents != 0 {
		t.Fatalf("after later price change: total=%d fallback=%d, want 100/0", stats.TotalEstimatedCostCents, stats.UnpricedFallbackEstimatedCents)
	}
}

// 场景 2：同生效时间原地改价后统计不变（价格表原地更新行为本身不变）。
func TestAIUsageStatsIgnoresSameEffectiveTimePriceUpdate(t *testing.T) {
	db := newSyntheticAICostDB(t)
	admin := NewAdminService(&config.Config{}, db)
	guard := NewAICostGuard(db)
	now := time.Now()
	dayStart := syntheticDayStart(now)
	merchant := createSyntheticAICostMerchant(t, db, "synthetic-inplace-price-merchant")
	effective := dayStart.AddDate(0, 0, -1)
	saveSyntheticModelPrice(t, admin, "m", 100, 200, 10, effective)
	insertSyntheticAIUsage(t, db, merchant.ID, nil, "m", "s", 1_000_000, 0, 0, 1_000_000, 100, dayStart.Add(10*time.Hour))
	// 同 key 原地改价 100 → 150 分/百万输入 Token。
	saveSyntheticModelPrice(t, admin, "m", 150, 300, 15, effective)
	var count int64
	if err := db.Model(&model.AIModelPrice{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("price rows=%d err=%v, want 1 (in-place update, not a new row)", count, err)
	}

	stats, err := guard.AIUsageStats(dayStart, dayStart.AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("usage stats: %v", err)
	}
	if stats.TotalEstimatedCostCents != 100 {
		t.Fatalf("after in-place price update: total=%d, want 100", stats.TotalEstimatedCostCents)
	}
}

// 场景 3：补录过去生效价后统计不变（无价调用的内置费率存量原样呈现）。
func TestAIUsageStatsIgnoresBackfilledPrice(t *testing.T) {
	db := newSyntheticAICostDB(t)
	admin := NewAdminService(&config.Config{}, db)
	guard := NewAICostGuard(db)
	now := time.Now()
	dayStart := syntheticDayStart(now)
	merchant := createSyntheticAICostMerchant(t, db, "synthetic-backfill-merchant")
	// 无价时写入的调用：内置费率存量 555 分。
	insertSyntheticAIUsage(t, db, merchant.ID, nil, "unpriced-model", "s", 100, 200, 0, 300, 555, dayStart.Add(10*time.Hour))
	// 事后补录过去日期生效价：旧重算口径会改写该笔为 1 分。
	saveSyntheticModelPrice(t, admin, "unpriced-model", 100, 200, 10, dayStart.AddDate(0, 0, -1))

	stats, err := guard.AIUsageStats(dayStart, dayStart.AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("usage stats: %v", err)
	}
	if stats.TotalEstimatedCostCents != 555 || stats.UnpricedFallbackEstimatedCents != 0 {
		t.Fatalf("after backfilled price: total=%d fallback=%d, want 555/0", stats.TotalEstimatedCostCents, stats.UnpricedFallbackEstimatedCents)
	}
}

// 场景 4：同一天上午调用、午后改价，预算页与统计页同日数相等。
func TestAIBudgetAndStatsAgreeWithinDayAfterPriceChange(t *testing.T) {
	db := newSyntheticAICostDB(t)
	admin := NewAdminService(&config.Config{}, db)
	guard := NewAICostGuard(db)
	now := time.Now()
	dayStart := syntheticDayStart(now)
	merchant := createSyntheticAICostMerchant(t, db, "synthetic-sameday-merchant")
	setSyntheticBudget(t, admin, merchant.ID, 1000, 0, model.AIBudgetOverLimitStop)
	saveSyntheticModelPrice(t, admin, "m", 100, 200, 10, dayStart.AddDate(0, 0, -1))
	insertSyntheticAIUsage(t, db, merchant.ID, nil, "m", "s", 1_000_000, 0, 0, 1_000_000, 100, dayStart.Add(9*time.Hour))
	saveSyntheticModelPrice(t, admin, "m", 300, 600, 30, dayStart.Add(15*time.Hour))

	status, err := guard.EvaluateMerchantBudget(merchant.ID)
	if err != nil {
		t.Fatalf("evaluate budget: %v", err)
	}
	stats, err := guard.AIUsageStats(dayStart, dayStart.AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("usage stats: %v", err)
	}
	if status.DailyUsedCents != 100 || stats.TotalEstimatedCostCents != 100 {
		t.Fatalf("budget daily=%d stats total=%d, want both 100", status.DailyUsedCents, stats.TotalEstimatedCostCents)
	}
}

// 场景 5a：分组取整差异消除——3 笔各 1 输入 Token，统计 = 存量逐笔 ceil 之和 3 分。
func TestAIUsageStatsUsesStoredPerCallCeil(t *testing.T) {
	db := newSyntheticAICostDB(t)
	admin := NewAdminService(&config.Config{}, db)
	guard := NewAICostGuard(db)
	now := time.Now()
	dayStart := syntheticDayStart(now)
	merchant := createSyntheticAICostMerchant(t, db, "synthetic-ceil-merchant")
	saveSyntheticModelPrice(t, admin, "m", 100, 200, 10, dayStart.AddDate(0, 0, -1))
	for i := 0; i < 3; i++ {
		insertSyntheticAIUsage(t, db, merchant.ID, nil, "m", "s", 1, 0, 0, 1, 1, dayStart.Add(10*time.Hour))
	}

	stats, err := guard.AIUsageStats(dayStart, dayStart.AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("usage stats: %v", err)
	}
	if stats.TotalEstimatedCostCents != 3 {
		t.Fatalf("per-call ceil total=%d, want 3 (grouped ceil would give 1)", stats.TotalEstimatedCostCents)
	}
}

// 场景 5b/5c：同模型多供应商、含缓存 Token 的调用，统计 = 存量值。
func TestAIUsageStatsIgnoresOtherProviderPriceAndCachedTokens(t *testing.T) {
	db := newSyntheticAICostDB(t)
	admin := NewAdminService(&config.Config{}, db)
	guard := NewAICostGuard(db)
	now := time.Now()
	dayStart := syntheticDayStart(now)
	merchant := createSyntheticAICostMerchant(t, db, "synthetic-multi-provider-merchant")
	// 写入供应商价：1M 输入 + 1M 输出 + 1M 缓存 = 100+200+10 = 310 分（存量）。
	saveSyntheticModelPrice(t, admin, "multi-model", 100, 200, 10, dayStart.AddDate(0, 0, -1))
	// 另一供应商同模型更晚生效价 1000+2000+100 = 3100 分：旧重算口径取任一供应商最新价会改写。
	saveSyntheticModelPriceForProvider(t, admin, "other-provider", "multi-model", 1000, 2000, 100, dayStart.Add(12*time.Hour))
	insertSyntheticAIUsage(t, db, merchant.ID, nil, "multi-model", "s", 1_000_000, 1_000_000, 1_000_000, 3_000_000, 310, dayStart.Add(10*time.Hour))

	stats, err := guard.AIUsageStats(dayStart, dayStart.AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("usage stats: %v", err)
	}
	if stats.TotalEstimatedCostCents != 310 {
		t.Fatalf("multi-provider/cached total=%d, want 310 (stored)", stats.TotalEstimatedCostCents)
	}
}

// 场景 6（核心断言）：同一商家同日，预算状态、统计、核对三处同数。
func TestAIBudgetStatsAndReconciliationAgree(t *testing.T) {
	db := newSyntheticAICostDB(t)
	admin := NewAdminService(&config.Config{}, db)
	guard := NewAICostGuard(db)
	now := time.Now()
	dayStart := syntheticDayStart(now)
	merchant := createSyntheticAICostMerchant(t, db, "synthetic-three-way-merchant")
	setSyntheticBudget(t, admin, merchant.ID, 1000, 0, model.AIBudgetOverLimitStop)
	saveSyntheticModelPrice(t, admin, "m", 100, 200, 10, dayStart.AddDate(0, 0, -1))
	insertSyntheticAIUsage(t, db, merchant.ID, nil, "m", "s", 1_000_000, 0, 0, 1_000_000, 100, dayStart.Add(10*time.Hour))

	status, _, err := admin.MerchantBudgetStatus(merchant.ID)
	if err != nil {
		t.Fatalf("budget status: %v", err)
	}
	stats, err := guard.AIUsageStats(dayStart, dayStart.AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("usage stats: %v", err)
	}
	result, err := admin.ReconcileAIBills(dayStart, dayStart)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	var byMerchant int64 = -1
	for _, row := range stats.ByMerchant {
		if row.MerchantID == merchant.ID {
			byMerchant = row.EstimatedCostCents
		}
	}
	if status.DailyUsedCents != 100 || byMerchant != 100 || result.EstimatedCostCents != 100 {
		t.Fatalf("three-way mismatch: budget daily=%d stats by-merchant=%d reconcile=%d, want all 100",
			status.DailyUsedCents, byMerchant, result.EstimatedCostCents)
	}
}

// 场景 8：触发 70% 预警后改价，预警快照、预算状态、统计三方一致。
func TestAIBudgetAlertSnapshotSurvivesPriceChange(t *testing.T) {
	db := newSyntheticAICostDB(t)
	admin := NewAdminService(&config.Config{}, db)
	guard := NewAICostGuard(db)
	now := time.Now()
	dayStart := syntheticDayStart(now)
	merchant := createSyntheticAICostMerchant(t, db, "synthetic-alert-price-merchant")
	setSyntheticBudget(t, admin, merchant.ID, 100, 0, model.AIBudgetOverLimitStop)
	saveSyntheticModelPrice(t, admin, "m", 100, 200, 10, dayStart.AddDate(0, 0, -1))
	insertSyntheticAIUsage(t, db, merchant.ID, nil, "m", "s", 100, 100, 0, 200, 75, dayStart.Add(10*time.Hour))
	if _, err := guard.EvaluateMerchantBudget(merchant.ID); err != nil {
		t.Fatalf("evaluate budget: %v", err)
	}
	// 触发 70% 预警后再改价。
	saveSyntheticModelPrice(t, admin, "m", 300, 600, 30, dayStart.Add(12*time.Hour))

	alerts, _, err := admin.ListBudgetAlerts(merchant.ID, "daily", 70, time.Time{}, time.Time{}, 1, 20)
	if err != nil {
		t.Fatalf("list alerts: %v", err)
	}
	if len(alerts) != 1 || alerts[0].EstimatedCostCents != 75 {
		t.Fatalf("alert snapshot mismatch: %+v, want 1 alert at 75", alerts)
	}
	status, err := guard.EvaluateMerchantBudget(merchant.ID)
	if err != nil {
		t.Fatalf("re-evaluate budget: %v", err)
	}
	stats, err := guard.AIUsageStats(dayStart, dayStart.AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("usage stats: %v", err)
	}
	if status.DailyUsedCents != 75 || stats.TotalEstimatedCostCents != 75 {
		t.Fatalf("after price change: budget daily=%d stats total=%d, want both 75", status.DailyUsedCents, stats.TotalEstimatedCostCents)
	}
}

// 2026-11-28 P1 第 5 件：预算/预警行带服务端拼出的商家名（C 方案），
// 不再靠前端二次拉商家列表补名字。
func TestAIBudgetRowsCarryMerchantName(t *testing.T) {
	db := newSyntheticAICostDB(t)
	admin := NewAdminService(&config.Config{}, db)
	names := []string{"synthetic-budget-a", "synthetic-budget-b", "synthetic-budget-c"}
	merchants := make([]model.Merchant, 0, len(names))
	for _, name := range names {
		merchants = append(merchants, createSyntheticAICostMerchant(t, db, name))
	}
	setSyntheticBudget(t, admin, 0, 100, 0, model.AIBudgetOverLimitStop)
	for _, merchant := range merchants {
		setSyntheticBudget(t, admin, merchant.ID, 100, 0, model.AIBudgetOverLimitStop)
	}

	rows, err := admin.ListMerchantBudgets()
	if err != nil {
		t.Fatalf("list budgets: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("budget rows = %d, want 4 (3 merchants + global default)", len(rows))
	}
	byID := make(map[uint]string, len(rows))
	for _, row := range rows {
		byID[row.MerchantID] = row.MerchantName
	}
	for _, merchant := range merchants {
		if byID[merchant.ID] != merchant.Name {
			t.Fatalf("budget row merchant %d name = %q, want %q", merchant.ID, byID[merchant.ID], merchant.Name)
		}
	}
	if byID[0] != "" {
		t.Fatalf("global default row merchant_name = %q, want empty", byID[0])
	}

	// 商家改名后接口返回新名：名字现查现填，无缓存。
	if err := db.Model(&model.Merchant{}).Where("id = ?", merchants[1].ID).Update("name", "synthetic-budget-b-renamed").Error; err != nil {
		t.Fatalf("rename merchant: %v", err)
	}
	rows, err = admin.ListMerchantBudgets()
	if err != nil {
		t.Fatalf("list budgets after rename: %v", err)
	}
	renamed := ""
	for _, row := range rows {
		if row.MerchantID == merchants[1].ID {
			renamed = row.MerchantName
		}
	}
	if renamed != "synthetic-budget-b-renamed" {
		t.Fatalf("renamed merchant budget row name = %q, want %q", renamed, "synthetic-budget-b-renamed")
	}
}

func TestAIBudgetAlertsCarryMerchantName(t *testing.T) {
	db := newSyntheticAICostDB(t)
	admin := NewAdminService(&config.Config{}, db)
	guard := NewAICostGuard(db)
	now := time.Now()
	dayStart := syntheticDayStart(now)
	merchants := []model.Merchant{
		createSyntheticAICostMerchant(t, db, "synthetic-alert-a"),
		createSyntheticAICostMerchant(t, db, "synthetic-alert-b"),
	}
	saveSyntheticModelPrice(t, admin, "m", 100, 200, 10, dayStart.AddDate(0, 0, -1))
	for _, merchant := range merchants {
		setSyntheticBudget(t, admin, merchant.ID, 100, 0, model.AIBudgetOverLimitStop)
		insertSyntheticAIUsage(t, db, merchant.ID, nil, "m", "s", 100, 100, 0, 200, 75, dayStart.Add(10*time.Hour))
		if _, err := guard.EvaluateMerchantBudget(merchant.ID); err != nil {
			t.Fatalf("evaluate budget for merchant %d: %v", merchant.ID, err)
		}
	}

	alerts, _, err := admin.ListBudgetAlerts(0, "", 0, time.Time{}, time.Time{}, 1, 20)
	if err != nil {
		t.Fatalf("list alerts: %v", err)
	}
	if len(alerts) != 2 {
		t.Fatalf("alerts = %d, want 2", len(alerts))
	}
	byID := make(map[uint]string, len(alerts))
	for _, alert := range alerts {
		byID[alert.MerchantID] = alert.MerchantName
	}
	for _, merchant := range merchants {
		if byID[merchant.ID] != merchant.Name {
			t.Fatalf("alert merchant %d name = %q, want %q", merchant.ID, byID[merchant.ID], merchant.Name)
		}
	}

	// 按 merchant_id 筛选后名字仍正确。
	filtered, _, err := admin.ListBudgetAlerts(merchants[1].ID, "daily", 70, time.Time{}, time.Time{}, 1, 20)
	if err != nil {
		t.Fatalf("list filtered alerts: %v", err)
	}
	if len(filtered) != 1 || filtered[0].MerchantName != merchants[1].Name {
		t.Fatalf("filtered alerts = %+v, want 1 row named %q", filtered, merchants[1].Name)
	}
}

func TestAIBudgetStatusCarriesMerchantName(t *testing.T) {
	db := newSyntheticAICostDB(t)
	admin := NewAdminService(&config.Config{}, db)
	merchant := createSyntheticAICostMerchant(t, db, "synthetic-status-merchant")
	setSyntheticBudget(t, admin, merchant.ID, 100, 0, model.AIBudgetOverLimitStop)

	status, _, err := admin.MerchantBudgetStatus(merchant.ID)
	if err != nil {
		t.Fatalf("budget status: %v", err)
	}
	if status.MerchantName != merchant.Name {
		t.Fatalf("status merchant_name = %q, want %q", status.MerchantName, merchant.Name)
	}

	// 无单独预算的商家走全局默认，名字也不丢。
	plain := createSyntheticAICostMerchant(t, db, "synthetic-status-plain")
	setSyntheticBudget(t, admin, 0, 100, 0, model.AIBudgetOverLimitStop)
	status, _, err = admin.MerchantBudgetStatus(plain.ID)
	if err != nil {
		t.Fatalf("budget status for plain merchant: %v", err)
	}
	if !status.GlobalDefault || status.MerchantName != plain.Name {
		t.Fatalf("plain merchant status = %+v, want global default named %q", status, plain.Name)
	}

	// merchant_id=0 的全局默认行：不报错，名字为空。
	status, _, err = admin.MerchantBudgetStatus(0)
	if err != nil {
		t.Fatalf("budget status for merchant 0: %v", err)
	}
	if status.MerchantName != "" {
		t.Fatalf("merchant 0 status merchant_name = %q, want empty", status.MerchantName)
	}
}

// 2026-11-28 P2 第 2 件：预警表改服务端分页，旧的 500 行硬上限删除。
// 501 条预警：total=501，第 1 页 20 行，第 26 页 1 行；带 merchant_id
// 筛选时 total 为筛选后计数；排序 period_start DESC, id DESC 跨页连续。
func TestAIBudgetAlertListPaginates(t *testing.T) {
	db := newSyntheticAICostDB(t)
	admin := NewAdminService(&config.Config{}, db)
	merchant := createSyntheticAICostMerchant(t, db, "synthetic-truncate-merchant")
	other := createSyntheticAICostMerchant(t, db, "synthetic-truncate-other")
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)
	for i := 0; i < 501; i++ {
		alert := model.MerchantAIBudgetAlert{
			MerchantID:         merchant.ID,
			PeriodType:         "daily",
			PeriodStart:        model.PlatformExpenseDate(base.AddDate(0, 0, -i).Format("2006-01-02")),
			Threshold:          70,
			UsedTokens:         int64(i),
			EstimatedCostCents: int64(i),
		}
		if err := db.Create(&alert).Error; err != nil {
			t.Fatalf("insert alert %d: %v", i, err)
		}
	}
	if err := db.Create(&model.MerchantAIBudgetAlert{
		MerchantID: other.ID, PeriodType: "daily",
		PeriodStart: model.PlatformExpenseDate(base.Format("2006-01-02")),
		Threshold:   70, UsedTokens: 1, EstimatedCostCents: 1,
	}).Error; err != nil {
		t.Fatalf("insert other merchant alert: %v", err)
	}

	page1, total, err := admin.ListBudgetAlerts(0, "", 0, time.Time{}, time.Time{}, 1, 20)
	if err != nil {
		t.Fatalf("list alerts page 1: %v", err)
	}
	if total != 502 || len(page1) != 20 {
		t.Fatalf("page 1: total=%d rows=%d, want total 502 rows 20", total, len(page1))
	}
	if page1[0].MerchantName != other.Name {
		t.Fatalf("page 1 first row merchant_name = %q, want %q (newest period_start, highest id)", page1[0].MerchantName, other.Name)
	}
	lastPage, total, err := admin.ListBudgetAlerts(0, "", 0, time.Time{}, time.Time{}, 26, 20)
	if err != nil {
		t.Fatalf("list alerts page 26: %v", err)
	}
	if total != 502 || len(lastPage) != 2 {
		t.Fatalf("page 26: total=%d rows=%d, want total 502 rows 2 (502 = 25*20 + 2)", total, len(lastPage))
	}

	// 带 merchant_id 筛选：total 为筛选后计数。
	filtered, filteredTotal, err := admin.ListBudgetAlerts(merchant.ID, "", 0, time.Time{}, time.Time{}, 1, 20)
	if err != nil {
		t.Fatalf("list filtered alerts: %v", err)
	}
	if filteredTotal != 501 || len(filtered) != 20 {
		t.Fatalf("filtered: total=%d rows=%d, want total 501 rows 20", filteredTotal, len(filtered))
	}
	filteredLast, _, err := admin.ListBudgetAlerts(merchant.ID, "", 0, time.Time{}, time.Time{}, 26, 20)
	if err != nil {
		t.Fatalf("list filtered alerts page 26: %v", err)
	}
	if len(filteredLast) != 1 {
		t.Fatalf("filtered page 26 rows=%d, want 1 (501 = 25*20 + 1)", len(filteredLast))
	}

	// 跨页连续：逐页收齐 501 条，(period_start, id) 全程非递增且无重复。
	seen := make(map[uint]bool, 501)
	prevStart, prevID := "", uint(0)
	for page := 1; page <= 26; page++ {
		rows, _, err := admin.ListBudgetAlerts(merchant.ID, "", 0, time.Time{}, time.Time{}, page, 20)
		if err != nil {
			t.Fatalf("list filtered alerts page %d: %v", page, err)
		}
		for _, row := range rows {
			if seen[row.ID] {
				t.Fatalf("alert id %d repeated across pages", row.ID)
			}
			seen[row.ID] = true
			start := string(row.PeriodStart)
			if prevStart != "" && (start > prevStart || (start == prevStart && row.ID > prevID)) {
				t.Fatalf("order broke across pages at id %d period_start %s after %s/%d", row.ID, start, prevStart, prevID)
			}
			prevStart, prevID = start, row.ID
		}
	}
	if len(seen) != 501 {
		t.Fatalf("collected %d distinct alerts across pages, want 501", len(seen))
	}
}

// 规模：2000 家商家时预算/预警接口仍一次返回（拼名字只是一次 SELECT id,name）。
// 600 家是 500 阈值之上的量级，2000 家覆盖之；耗时只记录，不做硬性断言
// （本机 DDL/IO 波动大，硬阈值易误报）。
func TestAIBudgetRowsScale2000Merchants(t *testing.T) {
	db := newSyntheticAICostDB(t)
	admin := NewAdminService(&config.Config{}, db)
	guard := NewAICostGuard(db)
	start := time.Now()
	merchants := make([]model.Merchant, 0, 2000)
	for i := 0; i < 2000; i++ {
		merchants = append(merchants, createSyntheticAICostMerchant(t, db, fmt.Sprintf("synthetic-scale-merchant-%04d", i)))
	}
	// 预算行只给第 1 家、第 1000 家、第 2000 家（最老与最新都覆盖）。
	for _, index := range []int{0, 999, 1999} {
		setSyntheticBudget(t, admin, merchants[index].ID, 100, 0, model.AIBudgetOverLimitStop)
	}
	now := time.Now()
	dayStart := syntheticDayStart(now)
	saveSyntheticModelPrice(t, admin, "m", 100, 200, 10, dayStart.AddDate(0, 0, -1))
	// 给最老的一家（第 1 家）造一条 70% 预警：id 最小，旧分页下一定漏名。
	insertSyntheticAIUsage(t, db, merchants[0].ID, nil, "m", "s", 100, 100, 0, 200, 75, dayStart.Add(10*time.Hour))
	if _, err := guard.EvaluateMerchantBudget(merchants[0].ID); err != nil {
		t.Fatalf("evaluate budget for oldest merchant: %v", err)
	}

	rows, err := admin.ListMerchantBudgets()
	if err != nil {
		t.Fatalf("list budgets at scale: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("budget rows = %d, want 3", len(rows))
	}
	wantNames := make(map[uint]string, len(merchants))
	for _, merchant := range merchants {
		wantNames[merchant.ID] = merchant.Name
	}
	for _, row := range rows {
		if row.MerchantName != wantNames[row.MerchantID] {
			t.Fatalf("budget row merchant %d name = %q, want %q", row.MerchantID, row.MerchantName, wantNames[row.MerchantID])
		}
	}
	alerts, _, err := admin.ListBudgetAlerts(merchants[0].ID, "daily", 70, time.Time{}, time.Time{}, 1, 20)
	if err != nil {
		t.Fatalf("list alerts at scale: %v", err)
	}
	if len(alerts) != 1 || alerts[0].MerchantName != merchants[0].Name {
		t.Fatalf("oldest merchant alerts = %+v, want 1 row named %q", alerts, merchants[0].Name)
	}
	t.Logf("2000 merchants: budgets+alerts returned in %s (setup included)", time.Since(start))
}

// 商家列表 keyword 模糊搜：下拉框远程搜索用。ListMerchants 会查
// orders / merchant_settlements / merchant_follow_ups /
// merchant_payment_configs 四张聚合表，合成库须一并建表。
func TestMerchantListKeywordSearch(t *testing.T) {
	db := newSyntheticAICostDB(t)
	if err := db.AutoMigrate(&model.Order{}, &model.MerchantSettlement{}, &model.MerchantFollowUp{}, &model.MerchantPaymentConfig{}); err != nil {
		t.Fatalf("migrate merchant list tables: %v", err)
	}
	admin := NewAdminService(&config.Config{}, db)
	for _, name := range []string{"山风咖啡馆", "山风面馆", "江边小店", "百分百%_店"} {
		createSyntheticAICostMerchant(t, db, name)
	}

	rows, total, err := admin.ListMerchants(1, 10, repository.MerchantListFilter{Keyword: "山风"})
	if err != nil {
		t.Fatalf("list merchants keyword 山风: %v", err)
	}
	if total != 2 || len(rows) != 2 {
		t.Fatalf("keyword 山风: total=%d rows=%d, want 2", total, len(rows))
	}
	rows, total, err = admin.ListMerchants(1, 10, repository.MerchantListFilter{Keyword: "江边"})
	if err != nil || total != 1 || len(rows) != 1 || rows[0].Name != "江边小店" {
		t.Fatalf("keyword 江边: total=%d rows=%+v err=%v, want 1 row 江边小店", total, rows, err)
	}
	// % 与 _ 按字面匹配，不当通配符：搜 "%" 只命中名字里真有 % 的一家。
	rows, total, err = admin.ListMerchants(1, 10, repository.MerchantListFilter{Keyword: "%"})
	if err != nil || total != 1 || len(rows) != 1 || rows[0].Name != "百分百%_店" {
		t.Fatalf("keyword %%: total=%d rows=%+v err=%v, want 1 row 百分百%%_店", total, rows, err)
	}
	// 空关键字不过滤，全量计数。
	rows, total, err = admin.ListMerchants(1, 10, repository.MerchantListFilter{})
	if err != nil || total != 4 || len(rows) != 4 {
		t.Fatalf("empty keyword: total=%d rows=%d err=%v, want 4", total, len(rows), err)
	}
	// 关键字 + 分页：total 是筛选后计数。
	rows, total, err = admin.ListMerchants(1, 1, repository.MerchantListFilter{Keyword: "山风"})
	if err != nil || total != 2 || len(rows) != 1 {
		t.Fatalf("keyword 山风 page 1 size 1: total=%d rows=%d err=%v, want total 2 rows 1", total, len(rows), err)
	}
}

// 2026-11-28 P2 第 2 件 B：商家列表服务端筛选/排序/统计下推。
// 旧行为是前端拉 page_size=500（后端静默压成 10）再本地过滤排序，
// 第 11 家及更老的商家搜不到、指标卡全按 10 家算；此处验证服务端
// status/subscription/keyword（含手机号）/risk 筛选、聚合排序与统计。
func TestMerchantListServerFiltersSortAndStats(t *testing.T) {
	db := newSyntheticAICostDB(t)
	if err := db.AutoMigrate(&model.Order{}, &model.MerchantSettlement{}, &model.MerchantFollowUp{}, &model.MerchantPaymentConfig{}); err != nil {
		t.Fatalf("migrate merchant list tables: %v", err)
	}
	admin := NewAdminService(&config.Config{}, db)
	now := time.Now()
	expireAt := func(d time.Duration) *time.Time { v := now.Add(d); return &v }
	mk := func(name, status, subStatus string, expire *time.Time, phone string) model.Merchant {
		m := createSyntheticAICostMerchant(t, db, name)
		if err := db.Model(&model.Merchant{}).Where("id = ?", m.ID).Updates(map[string]interface{}{
			"status": status, "subscription_status": subStatus,
			"subscription_expire_at": expire, "contact_phone": phone,
		}).Error; err != nil {
			t.Fatalf("update merchant %q: %v", name, err)
		}
		m.Status, m.SubscriptionStatus, m.SubscriptionExpireAt, m.ContactPhone = status, subStatus, expire, phone
		return m
	}
	active := mk("filter-active", "active", "active", expireAt(30*24*time.Hour), "13800000001")
	expired := mk("filter-expired", "active", "active", expireAt(-24*time.Hour), "13800000002")
	suspended := mk("filter-suspended", "suspended", "inactive", nil, "13800000003")
	plain := mk("filter-plain", "active", "inactive", nil, "13800000004")
	expiring := mk("filter-expiring", "active", "active", expireAt(3*24*time.Hour), "13800000005")
	paidAt := now
	mkOrder := func(no string, merchantID uint, total int64) {
		if err := db.Create(&model.Order{
			OrderNo: no, MerchantID: &merchantID, OrderType: "store_order",
			Status: "completed", TotalAmount: total, PaidAt: &paidAt,
		}).Error; err != nil {
			t.Fatalf("create order %q: %v", no, err)
		}
	}
	mkOrder("FILTER-ORDER-1", active.ID, 10000)
	mkOrder("FILTER-ORDER-2", expiring.ID, 5000)
	if err := db.Create(&model.MerchantFollowUp{MerchantID: plain.ID, Status: "open", Content: "synthetic follow-up"}).Error; err != nil {
		t.Fatalf("create follow-up: %v", err)
	}
	if err := db.Create(&model.MerchantPaymentConfig{MerchantID: suspended.ID, Mode: "direct", Status: "disabled", AuditStatus: "approved"}).Error; err != nil {
		t.Fatalf("create payment config: %v", err)
	}

	list := func(filter repository.MerchantListFilter, page, size int) ([]MerchantListRow, int64) {
		rows, total, err := admin.ListMerchants(page, size, filter)
		if err != nil {
			t.Fatalf("list merchants %+v: %v", filter, err)
		}
		return rows, total
	}
	names := func(rows []MerchantListRow) map[string]bool {
		got := make(map[string]bool, len(rows))
		for _, row := range rows {
			got[row.Name] = true
		}
		return got
	}

	if _, total := list(repository.MerchantListFilter{}, 1, 10); total != 5 {
		t.Fatalf("no filter total=%d, want 5", total)
	}
	if rows, total := list(repository.MerchantListFilter{Status: "suspended"}, 1, 10); total != 1 || !names(rows)["filter-suspended"] {
		t.Fatalf("status suspended total=%d rows=%+v, want 1 suspended", total, rows)
	}
	if rows, total := list(repository.MerchantListFilter{Subscription: "active"}, 1, 10); total != 2 || !names(rows)["filter-active"] || !names(rows)["filter-expiring"] {
		t.Fatalf("subscription active total=%d rows=%+v, want active+expiring", total, rows)
	}
	if _, total := list(repository.MerchantListFilter{Subscription: "inactive"}, 1, 10); total != 3 {
		t.Fatalf("subscription inactive total=%d, want 3", total)
	}
	if rows, total := list(repository.MerchantListFilter{Subscription: "active", ExpiringWithinDays: 7}, 1, 10); total != 1 || !names(rows)["filter-expiring"] {
		t.Fatalf("expiring within 7d total=%d rows=%+v, want 1 expiring", total, rows)
	}
	if rows, total := list(repository.MerchantListFilter{Keyword: "13800000001"}, 1, 10); total != 1 || !names(rows)["filter-active"] {
		t.Fatalf("keyword phone total=%d rows=%+v, want 1 active", total, rows)
	}
	if rows, total := list(repository.MerchantListFilter{Risk: "high"}, 1, 10); total != 1 || !names(rows)["filter-suspended"] {
		t.Fatalf("risk high total=%d rows=%+v, want 1 suspended", total, rows)
	}
	if rows, total := list(repository.MerchantListFilter{Risk: "follow_up"}, 1, 10); total != 1 || !names(rows)["filter-plain"] {
		t.Fatalf("risk follow_up total=%d rows=%+v, want 1 plain", total, rows)
	}
	if rows, total := list(repository.MerchantListFilter{SortBy: "trade_amount", SortOrder: "desc"}, 1, 10); total != 5 || rows[0].Name != "filter-active" || rows[0].TradeAmount != 10000 || rows[1].Name != "filter-expiring" || rows[1].TradeAmount != 5000 {
		t.Fatalf("trade_amount desc total=%d first=%+v second=%+v, want active 10000 then expiring 5000", total, rows[0], rows[1])
	}
	if rows, _ := list(repository.MerchantListFilter{SortBy: "trade_amount", SortOrder: "desc"}, 2, 1); len(rows) != 1 || rows[0].Name != "filter-expiring" {
		t.Fatalf("trade_amount desc page 2 size 1 rows=%+v, want expiring", rows)
	}
	if rows, _ := list(repository.MerchantListFilter{SortBy: "risk_score", SortOrder: "desc"}, 1, 2); len(rows) != 2 || rows[0].Name != "filter-suspended" || rows[1].Name != "filter-plain" {
		t.Fatalf("risk_score desc page 1 rows=%+v, want suspended then plain", rows)
	}
	_ = expired

	stats, err := admin.MerchantStats()
	if err != nil {
		t.Fatalf("merchant stats: %v", err)
	}
	if stats.Total != 5 || stats.SubscribedCount != 2 || stats.HighRiskCount != 1 || stats.WatchCount != 1 ||
		stats.OpenFollowUpCount != 1 || stats.PaymentMissingQRCount != 1 || stats.PendingSettlementMerchantCount != 2 {
		t.Fatalf("merchant stats = %+v, want total 5 subscribed 2 high 1 watch 1 follow-ups 1 missing-qr 1 settlement 2 (two paid-but-unsettled orders)", stats)
	}
}
