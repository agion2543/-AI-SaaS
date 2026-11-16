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
		AppEnv:           "production",
		InitialAdminUser: "synthetic-admin",
		InitialAdminPass: "synthetic-initial-seed-password",
	}
	if err := seedInitialData(db, cfg); err != nil {
		t.Fatalf("seed synthetic initial data: %v", err)
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
