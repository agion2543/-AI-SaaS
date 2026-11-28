package service

// AI 用量归属默认门店（2026-11-28）：recordAIUsage 写入时解析商家的默认门店
// （最早创建且在用的那家），失败调用（Success=false）同样带归属。
// 全部用 Disabled provider：Generate 必然返回 ErrAIUnavailable，
// 入口在失败分支记一条用量行，正好验证归属。

import (
	"context"
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

func newAIUsageStoreTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open synthetic AI usage store database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get synthetic AI usage store database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(
		&model.Merchant{}, &model.MerchantPlan{}, &model.Store{}, &model.StoreProduct{},
		&model.Order{}, &model.RefundRecord{},
		&model.ShareActivityConfig{}, &model.ShareCampaign{}, &model.ReferralCoupon{},
		&model.MerchantAIUsageLog{},
		&model.AIModelPrice{}, &model.MerchantAIBudget{}, &model.MerchantAIBudgetAlert{},
		&model.AIEmergencySwitch{}, &model.SystemConfig{},
	); err != nil {
		t.Fatalf("migrate synthetic AI usage store schema: %v", err)
	}
	return db
}

func createAIUsageStoreMerchant(t *testing.T, db *gorm.DB, name string) model.Merchant {
	t.Helper()
	merchant := model.Merchant{Name: name, Status: "active"}
	if err := db.Create(&merchant).Error; err != nil {
		t.Fatalf("create synthetic merchant %q: %v", name, err)
	}
	return merchant
}

func createAIUsageStore(t *testing.T, db *gorm.DB, merchantID uint, name, status string) model.Store {
	t.Helper()
	store := model.Store{MerchantID: merchantID, Name: name, Status: status, IsOpen: true}
	if err := db.Create(&store).Error; err != nil {
		t.Fatalf("create synthetic store %q: %v", name, err)
	}
	return store
}

func latestAIUsageLog(t *testing.T, db *gorm.DB, merchantID uint, scenario string) model.MerchantAIUsageLog {
	t.Helper()
	var log model.MerchantAIUsageLog
	if err := db.Where("merchant_id = ? AND scenario = ?", merchantID, scenario).Order("id DESC").First(&log).Error; err != nil {
		t.Fatalf("usage log merchant=%d scenario=%q not found: %v", merchantID, scenario, err)
	}
	return log
}

func TestRecordAIUsageAttributesToDefaultStore(t *testing.T) {
	t.Run("single store merchant records that store", func(t *testing.T) {
		db := newAIUsageStoreTestDB(t)
		svc := NewMerchantService(&config.Config{}, db)
		merchant := createAIUsageStoreMerchant(t, db, "synthetic-single-store-merchant")
		store := createAIUsageStore(t, db, merchant.ID, "合成单店", "active")

		if err := svc.recordAIUsage(merchant.ID, "synthetic-scenario", nil, errors.New("synthetic failure"), time.Millisecond); err != nil {
			t.Fatalf("recordAIUsage: %v", err)
		}
		log := latestAIUsageLog(t, db, merchant.ID, "synthetic-scenario")
		if log.StoreID == nil || *log.StoreID != store.ID {
			t.Fatalf("store_id=%v, want %d", log.StoreID, store.ID)
		}
		if log.CreatedAt.IsZero() {
			t.Fatalf("created_at must be populated by the explicit-column create: %+v", log)
		}
		if log.Success {
			t.Fatalf("failed call must be recorded with success=false: %+v", log)
		}
	})

	t.Run("multi store merchant records earliest active store", func(t *testing.T) {
		db := newAIUsageStoreTestDB(t)
		svc := NewMerchantService(&config.Config{}, db)
		merchant := createAIUsageStoreMerchant(t, db, "synthetic-multi-store-merchant")
		// 停用的店建得最早（id 最小），必须被跳过；最早在用的才是归属。
		createAIUsageStore(t, db, merchant.ID, "合成已停用店", "inactive")
		earliest := createAIUsageStore(t, db, merchant.ID, "合成最早在用店", "active")
		createAIUsageStore(t, db, merchant.ID, "合成较晚在用店", "active")

		if err := svc.recordAIUsage(merchant.ID, "synthetic-scenario", nil, errors.New("synthetic failure"), time.Millisecond); err != nil {
			t.Fatalf("recordAIUsage: %v", err)
		}
		log := latestAIUsageLog(t, db, merchant.ID, "synthetic-scenario")
		if log.StoreID == nil || *log.StoreID != earliest.ID {
			t.Fatalf("store_id=%v, want earliest active store %d", log.StoreID, earliest.ID)
		}
	})

	t.Run("merchant without stores still records NULL", func(t *testing.T) {
		db := newAIUsageStoreTestDB(t)
		svc := NewMerchantService(&config.Config{}, db)
		merchant := createAIUsageStoreMerchant(t, db, "synthetic-no-store-merchant")

		if err := svc.recordAIUsage(merchant.ID, "synthetic-scenario", nil, errors.New("synthetic failure"), time.Millisecond); err != nil {
			t.Fatalf("recordAIUsage: %v", err)
		}
		log := latestAIUsageLog(t, db, merchant.ID, "synthetic-scenario")
		if log.StoreID != nil {
			t.Fatalf("store_id=%v, want NULL for merchant without stores", *log.StoreID)
		}
	})

	t.Run("successful call is attributed the same way", func(t *testing.T) {
		db := newAIUsageStoreTestDB(t)
		svc := NewMerchantService(&config.Config{}, db)
		merchant := createAIUsageStoreMerchant(t, db, "synthetic-success-merchant")
		store := createAIUsageStore(t, db, merchant.ID, "合成成功调用店", "active")

		result := &AIResult{Provider: "synthetic-provider", Model: "synthetic-model", PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30}
		if err := svc.recordAIUsage(merchant.ID, "synthetic-scenario", result, nil, time.Millisecond); err != nil {
			t.Fatalf("recordAIUsage: %v", err)
		}
		log := latestAIUsageLog(t, db, merchant.ID, "synthetic-scenario")
		if !log.Success || log.StoreID == nil || *log.StoreID != store.ID {
			t.Fatalf("success call log mismatch: %+v, want success=true store_id=%d", log, store.ID)
		}
	})
}

func TestAIEntryPointsAttributeUsageToDefaultStore(t *testing.T) {
	db := newAIUsageStoreTestDB(t)
	svc := NewMerchantService(&config.Config{}, db)
	merchant := createAIUsageStoreMerchant(t, db, "synthetic-entry-merchant")
	createAIUsageStore(t, db, merchant.ID, "合成已停用店", "inactive")
	earliest := createAIUsageStore(t, db, merchant.ID, "合成最早在用店", "active")
	createAIUsageStore(t, db, merchant.ID, "合成较晚在用店", "active")

	// Disabled provider：Generate 必然失败，入口在失败分支记用量行
	// （Success=false），归属仍应是默认门店。
	assertAttributed := func(scenario string) {
		t.Helper()
		log := latestAIUsageLog(t, db, merchant.ID, scenario)
		if log.Success {
			t.Fatalf("scenario %q: disabled provider call must be recorded as failed", scenario)
		}
		if log.StoreID == nil || *log.StoreID != earliest.ID {
			t.Fatalf("scenario %q: store_id=%v, want default store %d", scenario, log.StoreID, earliest.ID)
		}
	}

	if _, err := svc.GenerateAIMarketingCopy(merchant.ID, dto.GenerateAIMarketingCopyRequest{Scenario: "裂变海报"}); err == nil {
		t.Fatalf("non-product marketing copy must fail with disabled provider")
	}
	assertAttributed("裂变海报")

	if _, err := svc.GenerateAIMarketingCopy(merchant.ID, dto.GenerateAIMarketingCopyRequest{Scenario: "商品优化"}); err == nil {
		t.Fatalf("product marketing copy must fail with disabled provider")
	}
	assertAttributed("商品优化")

	if _, err := svc.GenerateAIReferralCopies(merchant.ID, dto.GenerateAIReferralCopyRequest{}); err == nil {
		t.Fatalf("referral copies must fail with disabled provider")
	}
	assertAttributed("referral_multi_copy")

	if _, err := svc.GenerateAIShareReview(merchant.ID, dto.GenerateAIShareReviewRequest{}); err == nil {
		t.Fatalf("share review must fail with disabled provider")
	}
	assertAttributed("share_campaign_review")

	// OpsContentScripts 走模板兜底、本身返回 nil error，但失败的模型调用
	// 同样要记一条带归属的用量行。
	script, err := svc.OpsContentScripts(context.Background(), merchant.ID, "new_dish")
	if err != nil || script == nil {
		t.Fatalf("ops content scripts must fall back to template: script=%v err=%v", script, err)
	}
	assertAttributed("ops_content_script")
}

func TestAIUsageStatsGroupsNewRowsByRealStore(t *testing.T) {
	db := newSyntheticAICostDB(t)
	if err := db.AutoMigrate(&model.Store{}); err != nil {
		t.Fatalf("migrate synthetic store table: %v", err)
	}
	svc := NewMerchantService(&config.Config{}, db)
	guard := NewAICostGuard(db)
	now := time.Now()
	merchant := createSyntheticAICostMerchant(t, db, "synthetic-stats-store-merchant")
	store := createAIUsageStore(t, db, merchant.ID, "合成望京店", "active")

	// 历史行：2026-11-28 前直接插入的 NULL 归属行，保持原样。
	insertSyntheticAIUsage(t, db, merchant.ID, nil, "synthetic-model", "synthetic-history", 100, 100, 0, 200, 0, now)
	// 新行：经 recordAIUsage 写入，应归属默认门店。
	if err := svc.recordAIUsage(merchant.ID, "synthetic-new-one", nil, errors.New("synthetic failure"), time.Millisecond); err != nil {
		t.Fatalf("recordAIUsage new one: %v", err)
	}
	if err := svc.recordAIUsage(merchant.ID, "synthetic-new-two", nil, errors.New("synthetic failure"), time.Millisecond); err != nil {
		t.Fatalf("recordAIUsage new two: %v", err)
	}

	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	stats, err := guard.AIUsageStats(dayStart, dayStart.AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("usage stats: %v", err)
	}
	if len(stats.ByStore) != 2 {
		t.Fatalf("by-store rows=%d, want 2 (real store + unattributed 0)", len(stats.ByStore))
	}
	for _, row := range stats.ByStore {
		switch row.StoreID {
		case store.ID:
			if row.RequestCount != 2 || row.StoreName != "合成望京店" {
				t.Fatalf("real store row mismatch: %+v, want requests=2 name=合成望京店", row)
			}
		case 0:
			// 0 行只剩历史 NULL 行。
			if row.RequestCount != 1 || row.StoreName != "" {
				t.Fatalf("unattributed store row mismatch: %+v, want requests=1 name empty", row)
			}
		default:
			t.Fatalf("unexpected by-store row: %+v", row)
		}
	}
}
