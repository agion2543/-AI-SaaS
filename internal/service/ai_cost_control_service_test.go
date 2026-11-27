package service

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
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
	// A: cheap-model, 1M prompt + 1M completion + 1M cached = 100+200+10 = 310 (priced).
	insertSyntheticAIUsage(t, db, merchantA.ID, &storeID, "cheap-model", "scenario-one", 1_000_000, 1_000_000, 1_000_000, 3_000_000, 999, now)
	// B: unpriced model falls back to the stored estimate 555.
	insertSyntheticAIUsage(t, db, merchantB.ID, nil, "unpriced-model", "scenario-two", 100, 200, 0, 300, 555, now)

	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	stats, err := guard.AIUsageStats(dayStart, dayStart.AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("usage stats: %v", err)
	}
	if stats.TotalRequests != 2 || stats.TotalPromptTokens != 1_000_100 || stats.TotalCompletionTokens != 1_000_200 ||
		stats.TotalCachedTokens != 1_000_000 || stats.TotalTokens != 3_000_300 || stats.TotalEstimatedCostCents != 865 {
		t.Fatalf("totals mismatch: %+v, want requests=2 prompt=1000100 completion=1000200 cached=1000000 total=3000300 cost=865", stats)
	}
	if stats.UnpricedFallbackEstimatedCents != 555 {
		t.Fatalf("unpriced fallback=%d, want 555", stats.UnpricedFallbackEstimatedCents)
	}
	if len(stats.ByMerchant) != 2 || stats.ByMerchant[0].MerchantID != merchantB.ID || stats.ByMerchant[0].EstimatedCostCents != 555 {
		t.Fatalf("by-merchant ranking mismatch: %+v", stats.ByMerchant)
	}
	if len(stats.ByModel) != 2 || stats.ByModel[0].Model != "unpriced-model" {
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
	// Estimated 310 (price table) vs actual 500 (manual bill): difference +190.
	if result.EstimatedCostCents != 310 || result.ActualBillTotalCents != 500 || result.DifferenceCents != 190 || result.ActualBillCount != 1 {
		t.Fatalf("reconciliation mismatch: %+v", result)
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
