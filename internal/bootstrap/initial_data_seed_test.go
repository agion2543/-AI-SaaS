package bootstrap

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/model"
)

func openInitialDataSeedTestDB(t *testing.T) *gorm.DB {
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
	if err := db.AutoMigrate(&model.AdminRole{}, &model.AdminUser{}, &model.MembershipPackage{}, &model.MerchantPlan{}); err != nil {
		t.Fatalf("migrate synthetic seed tables: %v", err)
	}
	return db
}

func runInitialDataSeed(t *testing.T, db *gorm.DB) {
	t.Helper()
	cfg := &config.Config{
		AppEnv:                 "production",
		InitialAdminUser:       "synthetic-admin",
		InitialAdminPass:       "synthetic-initial-seed-password",
		InitialDataSeedEnabled: true,
	}
	if err := seedConfiguredInitialData(db, cfg); err != nil {
		t.Fatalf("seed synthetic initial data: %v", err)
	}
}

func TestSeedConfiguredInitialDataDisabledDoesNotMutateManagedUpgradeData(t *testing.T) {
	db := openInitialDataSeedTestDB(t)
	existingRole := model.AdminRole{
		Name:        "Operator-managed role",
		Code:        "super_admin",
		Permissions: "custom_permissions",
	}
	if err := db.Create(&existingRole).Error; err != nil {
		t.Fatalf("create synthetic existing role: %v", err)
	}

	cfg := &config.Config{
		AppEnv:                 "production",
		InitialAdminUser:       "synthetic-admin",
		InitialAdminPass:       "synthetic-initial-seed-password",
		InitialDataSeedEnabled: false,
	}
	if err := seedConfiguredInitialData(db, cfg); err != nil {
		t.Fatalf("skip synthetic startup seeds: %v", err)
	}

	var savedRole model.AdminRole
	if err := db.First(&savedRole, existingRole.ID).Error; err != nil {
		t.Fatalf("load existing role after skipped seeds: %v", err)
	}
	if savedRole.Name != existingRole.Name || savedRole.Permissions != existingRole.Permissions {
		t.Fatalf("skipped seeds changed existing role: before=%#v after=%#v", existingRole, savedRole)
	}

	for name, modelValue := range map[string]interface{}{
		"admin users":         &model.AdminUser{},
		"membership packages": &model.MembershipPackage{},
		"merchant plans":      &model.MerchantPlan{},
	} {
		var count int64
		if err := db.Model(modelValue).Count(&count).Error; err != nil {
			t.Fatalf("count %s after skipped seeds: %v", name, err)
		}
		if count != 0 {
			t.Errorf("skipped seeds created %d %s; want none", count, name)
		}
	}
}

func TestSeedInitialDataCreatesPackageAndMerchantPlanDefaults(t *testing.T) {
	db := openInitialDataSeedTestDB(t)

	runInitialDataSeed(t, db)

	var packageCount int64
	if err := db.Model(&model.MembershipPackage{}).Count(&packageCount).Error; err != nil {
		t.Fatal(err)
	}
	if packageCount != 5 {
		t.Fatalf("got %d membership packages after initial seed, want 5", packageCount)
	}
	var monthlyPackage model.MembershipPackage
	if err := db.Where("code = ?", "month").First(&monthlyPackage).Error; err != nil {
		t.Fatal(err)
	}
	if monthlyPackage.Name != "月卡" || monthlyPackage.Price != 12900 || monthlyPackage.Quota != 500 {
		t.Fatalf("unexpected monthly package defaults: %#v", monthlyPackage)
	}

	var planCount int64
	if err := db.Model(&model.MerchantPlan{}).Count(&planCount).Error; err != nil {
		t.Fatal(err)
	}
	if planCount != 2 {
		t.Fatalf("got %d merchant plans after initial seed, want 2", planCount)
	}
	var monthlyPlan model.MerchantPlan
	if err := db.Where("name = ?", "月付").First(&monthlyPlan).Error; err != nil {
		t.Fatal(err)
	}
	if monthlyPlan.PriceCents != 19900 || monthlyPlan.DurationDays != 30 {
		t.Fatalf("unexpected monthly merchant plan defaults: %#v", monthlyPlan)
	}
}

func TestSeedInitialDataPreservesEditsCreatesMissingPackagesAndDoesNotRecreateMerchantPlanDefaults(t *testing.T) {
	db := openInitialDataSeedTestDB(t)
	runInitialDataSeed(t, db)

	var pkg model.MembershipPackage
	if err := db.Where("code = ?", "day").First(&pkg).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&pkg).Updates(map[string]interface{}{
		"name":  "自定义日卡",
		"price": 1777,
		"quota": 17,
	}).Error; err != nil {
		t.Fatalf("customize synthetic membership package: %v", err)
	}

	var plan model.MerchantPlan
	if err := db.Where("name = ?", "月付").First(&plan).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&plan).Updates(map[string]interface{}{
		"name":          "自定义月付",
		"price_cents":   18888,
		"duration_days": 33,
	}).Error; err != nil {
		t.Fatalf("customize synthetic merchant plan: %v", err)
	}
	if err := db.Where("name = ?", "年付").Delete(&model.MerchantPlan{}).Error; err != nil {
		t.Fatalf("remove synthetic merchant plan seed: %v", err)
	}

	if err := db.Where("code = ?", "week").Delete(&model.MembershipPackage{}).Error; err != nil {
		t.Fatalf("remove synthetic package seed: %v", err)
	}

	runInitialDataSeed(t, db)

	var savedPackage model.MembershipPackage
	if err := db.Where("code = ?", "day").First(&savedPackage).Error; err != nil {
		t.Fatal(err)
	}
	if savedPackage.ID != pkg.ID || savedPackage.Name != "自定义日卡" || savedPackage.Price != 1777 || savedPackage.Quota != 17 {
		t.Fatalf("edited membership package changed during reseed: %#v", savedPackage)
	}

	var savedPlan model.MerchantPlan
	if err := db.First(&savedPlan, plan.ID).Error; err != nil {
		t.Fatal(err)
	}
	if savedPlan.Name != "自定义月付" || savedPlan.PriceCents != 18888 || savedPlan.DurationDays != 33 {
		t.Fatalf("edited merchant plan changed during reseed: %#v", savedPlan)
	}

	var recreatedPackage model.MembershipPackage
	if err := db.Where("code = ?", "week").First(&recreatedPackage).Error; err != nil {
		t.Fatalf("missing package seed was not recreated: %v", err)
	}
	if recreatedPackage.Name != "周卡" || recreatedPackage.Price != 4990 || recreatedPackage.Quota != 100 {
		t.Fatalf("recreated package has unexpected defaults: %#v", recreatedPackage)
	}

	var savedPlanCount int64
	if err := db.Model(&model.MerchantPlan{}).Count(&savedPlanCount).Error; err != nil {
		t.Fatal(err)
	}
	if savedPlanCount != 1 {
		t.Fatalf("got %d merchant plans after reseed with an existing renamed plan, want only the original plan", savedPlanCount)
	}
	var defaultPlanCount int64
	if err := db.Model(&model.MerchantPlan{}).Where("name IN ?", []string{"月付", "年付"}).Count(&defaultPlanCount).Error; err != nil {
		t.Fatal(err)
	}
	if defaultPlanCount != 0 {
		t.Fatalf("reseed recreated %d named default merchant plans after one was renamed and another deleted", defaultPlanCount)
	}
}

func TestSeedAutoActivationRequiresExplicitDevelopment(t *testing.T) {
	db := openInitialDataSeedTestDB(t)
	if err := db.AutoMigrate(&model.Merchant{}, &model.Store{}, &model.MerchantGroupBuyLink{}, &model.MerchantGroupBuyLinkAudit{}); err != nil {
		t.Fatalf("migrate synthetic seed merchant tables: %v", err)
	}
	pending := model.Merchant{Name: "合成待审核商家", Status: "pending"}
	if err := db.Create(&pending).Error; err != nil {
		t.Fatalf("create synthetic pending merchant: %v", err)
	}
	base := &config.Config{
		AppEnv:                 "development",
		InitialAdminUser:       "synthetic-admin",
		InitialAdminPass:       "synthetic-initial-seed-password",
		InitialDataSeedEnabled: true,
	}

	// H2: AppEnv reads as development only because it is the default when
	// APP_ENV is omitted. Pending merchants must stay pending.
	implicit := *base
	if err := seedInitialData(db, &implicit); err != nil {
		t.Fatalf("seed with implicit development default: %v", err)
	}
	var kept model.Merchant
	if err := db.First(&kept, pending.ID).Error; err != nil {
		t.Fatal(err)
	}
	if kept.Status != "pending" {
		t.Fatalf("implicit development default auto-activated merchant: status=%q", kept.Status)
	}

	// Explicit APP_ENV=development keeps the old local-dev behaviour.
	explicit := *base
	explicit.DevPasswordResetEnabled = true
	if err := seedInitialData(db, &explicit); err != nil {
		t.Fatalf("seed with explicit development: %v", err)
	}
	var activated model.Merchant
	if err := db.First(&activated, pending.ID).Error; err != nil {
		t.Fatal(err)
	}
	if activated.Status != "active" {
		t.Fatalf("explicit development should auto-activate pending merchants: status=%q", activated.Status)
	}
}
