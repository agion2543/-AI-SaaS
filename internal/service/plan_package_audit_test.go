package service

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"gorm.io/gorm"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

func TestMerchantPlanSaveIsAtomicAndAuditsOnlyPlanTerms(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.AuditLog{}); err != nil {
		t.Fatalf("migrate synthetic audit schema: %v", err)
	}
	actor := AuditActor{ID: 17, Type: "admin", IP: "synthetic-ip", UserAgent: "synthetic-agent"}
	admin := NewAdminService(nil, db)
	request := dto.SaveMerchantPlanRequest{Name: "合成订阅月付", PriceCents: 21900, DurationDays: 30, Sort: 8}

	dropFailure := failAuditInserts(t, db)
	if _, err := admin.SaveMerchantPlan(0, request, actor); err == nil {
		t.Fatal("merchant plan creation succeeded despite audit storage failure")
	}
	var plans int64
	if err := db.Model(&model.MerchantPlan{}).Count(&plans).Error; err != nil {
		t.Fatal(err)
	}
	if plans != 0 || countAuditLogs(t, db) != 0 {
		t.Fatalf("failed creation left plan/audit rows: plans=%d audits=%d", plans, countAuditLogs(t, db))
	}
	dropFailure()

	plan, err := admin.SaveMerchantPlan(0, request, actor)
	if err != nil {
		t.Fatalf("create synthetic merchant plan: %v", err)
	}
	assertPlanConfigAudit(t, db, "merchant_plan_create", "merchant_plan", plan.ID, actor, nil, map[string]interface{}{
		"name": "合成订阅月付", "price_cents": float64(21900), "duration_days": float64(30),
	})

	dropFailure = failAuditInserts(t, db)
	if _, err := admin.SaveMerchantPlan(plan.ID, dto.SaveMerchantPlanRequest{Name: "合成订阅年付", PriceCents: 199900, DurationDays: 365, Sort: 2}, actor); err == nil {
		t.Fatal("merchant plan update succeeded despite audit storage failure")
	}
	var afterFailedUpdate model.MerchantPlan
	if err := db.First(&afterFailedUpdate, plan.ID).Error; err != nil {
		t.Fatal(err)
	}
	if afterFailedUpdate.Name != "合成订阅月付" || afterFailedUpdate.PriceCents != 21900 || afterFailedUpdate.DurationDays != 30 || countAuditLogs(t, db) != 1 {
		t.Fatalf("failed update left partial plan state: %+v audits=%d", afterFailedUpdate, countAuditLogs(t, db))
	}
	dropFailure()

	updated, err := admin.SaveMerchantPlan(plan.ID, dto.SaveMerchantPlanRequest{Name: "合成订阅年付", PriceCents: 199900, DurationDays: 365, Sort: 2}, actor)
	if err != nil {
		t.Fatalf("update synthetic merchant plan: %v", err)
	}
	assertPlanConfigAudit(t, db, "merchant_plan_update", "merchant_plan", updated.ID, actor,
		map[string]interface{}{"name": "合成订阅月付", "price_cents": float64(21900), "duration_days": float64(30)},
		map[string]interface{}{"name": "合成订阅年付", "price_cents": float64(199900), "duration_days": float64(365)},
	)
}

func TestMembershipPackageSaveIsAtomicAndAuditsOnlyNecessaryMetadata(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.AuditLog{}); err != nil {
		t.Fatalf("migrate synthetic audit schema: %v", err)
	}
	actor := AuditActor{ID: 23, Type: "admin", IP: "synthetic-ip", UserAgent: "synthetic-agent"}
	packages := NewPackageService(db)
	request := dto.SavePackageRequest{
		Name: "合成会员月卡", Code: "synthetic-month", DurationDays: 30, Price: 990,
		OriginalPrice: 1290, Quota: 100, Sort: 7, Status: "published", Description: "DO-NOT-AUDIT synthetic description",
	}

	dropFailure := failAuditInserts(t, db)
	if _, err := packages.Save(0, request, actor); err == nil {
		t.Fatal("membership package creation succeeded despite audit storage failure")
	}
	var packageCount int64
	if err := db.Model(&model.MembershipPackage{}).Count(&packageCount).Error; err != nil {
		t.Fatal(err)
	}
	if packageCount != 0 || countAuditLogs(t, db) != 0 {
		t.Fatalf("failed creation left package/audit rows: packages=%d audits=%d", packageCount, countAuditLogs(t, db))
	}
	dropFailure()

	pkg, err := packages.Save(0, request, actor)
	if err != nil {
		t.Fatalf("create synthetic membership package: %v", err)
	}
	assertPlanConfigAudit(t, db, "membership_package_create", "membership_package", pkg.ID, actor, nil, map[string]interface{}{
		"name": "合成会员月卡", "price": float64(990), "original_price": float64(1290),
		"duration_days": float64(30), "quota": float64(100), "status": "published", "is_lifetime": false,
	})
	var createAudit model.AuditLog
	if err := db.Where("action = ?", "membership_package_create").First(&createAudit).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(createAudit.Detail, request.Description) || strings.Contains(createAudit.Detail, request.Code) {
		t.Fatalf("package audit contains descriptive text or package code: %s", createAudit.Detail)
	}

	dropFailure = failAuditInserts(t, db)
	if _, err := packages.Save(pkg.ID, dto.SavePackageRequest{
		Name: "合成会员年卡", Code: "synthetic-year", DurationDays: 365, Price: 7990,
		OriginalPrice: 9990, Quota: 2000, Sort: 3, Status: "hidden", IsLifetime: true,
		Description: "DO-NOT-AUDIT updated description",
	}, actor); err == nil {
		t.Fatal("membership package update succeeded despite audit storage failure")
	}
	var afterFailedUpdate model.MembershipPackage
	if err := db.First(&afterFailedUpdate, pkg.ID).Error; err != nil {
		t.Fatal(err)
	}
	if afterFailedUpdate.Name != request.Name || afterFailedUpdate.Price != request.Price || afterFailedUpdate.Status != request.Status || afterFailedUpdate.Description != request.Description || countAuditLogs(t, db) != 1 {
		t.Fatalf("failed update left partial package state: %+v audits=%d", afterFailedUpdate, countAuditLogs(t, db))
	}
	dropFailure()

	updated, err := packages.Save(pkg.ID, dto.SavePackageRequest{
		Name: "合成会员年卡", Code: "synthetic-year", DurationDays: 365, Price: 7990,
		OriginalPrice: 9990, Quota: 2000, Sort: 3, Status: "hidden", IsLifetime: true,
		Description: "DO-NOT-AUDIT updated description",
	}, actor)
	if err != nil {
		t.Fatalf("update synthetic membership package: %v", err)
	}
	assertPlanConfigAudit(t, db, "membership_package_update", "membership_package", updated.ID, actor,
		map[string]interface{}{
			"name": "合成会员月卡", "price": float64(990), "original_price": float64(1290),
			"duration_days": float64(30), "quota": float64(100), "status": "published", "is_lifetime": false,
		},
		map[string]interface{}{
			"name": "合成会员年卡", "price": float64(7990), "original_price": float64(9990),
			"duration_days": float64(365), "quota": float64(2000), "status": "hidden", "is_lifetime": true,
		},
	)
}

func assertPlanConfigAudit(t *testing.T, db *gorm.DB, action, targetType string, targetID uint, actor AuditActor, wantBefore, wantAfter map[string]interface{}) {
	t.Helper()
	var audit model.AuditLog
	if err := db.Where("action = ?", action).First(&audit).Error; err != nil {
		t.Fatalf("load %s audit: %v", action, err)
	}
	if audit.ActorID != actor.ID || audit.ActorType != actor.Type || audit.Action != action || audit.TargetType != targetType || audit.TargetID != targetID || audit.ActorName != "" || audit.TargetName != "" || audit.IP != "" || audit.UserAgent != "" {
		t.Fatalf("%s audit contains unexpected attribution or identifying metadata: %+v", action, audit)
	}
	var detail struct {
		Purpose string                 `json:"purpose"`
		Before  map[string]interface{} `json:"before"`
		After   map[string]interface{} `json:"after"`
	}
	if err := json.Unmarshal([]byte(audit.Detail), &detail); err != nil {
		t.Fatalf("decode %s audit: %v (%s)", action, err, audit.Detail)
	}
	if detail.Purpose == "" || !reflect.DeepEqual(detail.Before, wantBefore) || !reflect.DeepEqual(detail.After, wantAfter) {
		t.Fatalf("%s audit before/after mismatch: before=%v after=%v want before=%v after=%v", action, detail.Before, detail.After, wantBefore, wantAfter)
	}
}
