package service

import (
	"encoding/json"
	"strings"
	"testing"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

func TestGenerateCardsCreatesSafeAuditAtomically(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.CardCode{}, &model.AuditLog{}, &model.AdminUser{}); err != nil {
		t.Fatalf("migrate synthetic card audit schema: %v", err)
	}
	actor := createAuditTestActor(t, db)
	pkg := model.MembershipPackage{
		Name:         "合成月卡",
		Code:         "SYNTHETIC-PACKAGE-CODE-DO-NOT-AUDIT",
		DurationDays: 30,
		Quota:        100,
		Status:       "published",
		Description:  "SYNTHETIC-PACKAGE-DESCRIPTION-DO-NOT-AUDIT",
	}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("create synthetic membership package: %v", err)
	}

	request := dto.BatchCardRequest{PackageID: pkg.ID, Count: 3, Quota: 12}
	cards, err := NewAdminService(nil, db).GenerateCards(request, actor)
	if err != nil {
		t.Fatalf("generate synthetic card batch: %v", err)
	}
	if len(cards) != request.Count {
		t.Fatalf("generated %d cards, want %d", len(cards), request.Count)
	}
	seenCodes := make(map[string]bool, len(cards))
	batchNo := cards[0].BatchNo
	for _, card := range cards {
		if card.ID == 0 || card.PackageID != pkg.ID || card.Quota != request.Quota || card.Status != "unused" || card.ExpiredAt != nil {
			t.Fatalf("generated card semantics changed: %+v", card)
		}
		if card.BatchNo != batchNo || card.Code == "" || seenCodes[card.Code] {
			t.Fatalf("generated card batch/code invalid or duplicated: %+v", card)
		}
		seenCodes[card.Code] = true
	}
	var persistedCards int64
	if err := db.Model(&model.CardCode{}).Count(&persistedCards).Error; err != nil {
		t.Fatal(err)
	}
	if persistedCards != int64(request.Count) || countAuditLogs(t, db) != 1 {
		t.Fatalf("successful batch persisted cards=%d audits=%d, want %d/1", persistedCards, countAuditLogs(t, db), request.Count)
	}

	var audit model.AuditLog
	if err := db.Where("action = ?", "admin_card_batch_generate").First(&audit).Error; err != nil {
		t.Fatalf("load card batch audit: %v", err)
	}
	if audit.ActorID != actor.ID || audit.ActorType != actor.Type || audit.TargetType != "membership_package" || audit.TargetID != pkg.ID || audit.ActorName != "" || audit.TargetName != "" || audit.MerchantID != nil || audit.IP != "" || audit.UserAgent != "" {
		t.Fatalf("card batch audit attribution or target unexpected: %+v", audit)
	}
	var detail struct {
		Purpose      string `json:"purpose"`
		PackageID    uint   `json:"package_id"`
		Count        int    `json:"count"`
		Quota        int    `json:"quota"`
		DurationDays int    `json:"duration_days"`
		IsLifetime   bool   `json:"is_lifetime"`
	}
	if err := json.Unmarshal([]byte(audit.Detail), &detail); err != nil {
		t.Fatalf("decode card batch audit detail: %v (%s)", err, audit.Detail)
	}
	if detail.Purpose != "批量生成会员卡密" || detail.PackageID != pkg.ID || detail.Count != request.Count || detail.Quota != request.Quota || detail.DurationDays != pkg.DurationDays || detail.IsLifetime != pkg.IsLifetime {
		t.Fatalf("card batch audit safe metadata mismatch: %+v", detail)
	}
	for _, forbidden := range []string{pkg.Code, pkg.Description, "CARD-", batchNo} {
		if strings.Contains(audit.Detail, forbidden) {
			t.Fatalf("card batch audit contains forbidden value %q: %s", forbidden, audit.Detail)
		}
	}
	for _, card := range cards {
		if strings.Contains(audit.Detail, card.Code) {
			t.Fatalf("card batch audit contains redeemable card code: %s", audit.Detail)
		}
	}
}

func TestGenerateCardsRollsBackWhenAuditInsertFails(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.CardCode{}, &model.AuditLog{}, &model.AdminUser{}); err != nil {
		t.Fatalf("migrate synthetic card audit schema: %v", err)
	}
	actor := createAuditTestActor(t, db)
	pkg := model.MembershipPackage{Name: "合成回滚套餐", Code: "synthetic-rollback-package", DurationDays: 7, Status: "published"}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("create synthetic rollback package: %v", err)
	}

	dropFailure := failAuditInserts(t, db)
	defer dropFailure()
	cards, err := NewAdminService(nil, db).GenerateCards(dto.BatchCardRequest{PackageID: pkg.ID, Count: 4, Quota: 5}, actor)
	if err == nil {
		t.Fatal("card batch generation succeeded despite audit storage failure")
	}
	if len(cards) != 0 {
		t.Fatalf("failed card batch returned redeemable codes: count=%d", len(cards))
	}
	var persistedCards int64
	if err := db.Model(&model.CardCode{}).Count(&persistedCards).Error; err != nil {
		t.Fatal(err)
	}
	if persistedCards != 0 || countAuditLogs(t, db) != 0 {
		t.Fatalf("audit failure left partial card batch: cards=%d audits=%d", persistedCards, countAuditLogs(t, db))
	}
}
