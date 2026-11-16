package service

import (
	"encoding/json"
	"strings"
	"testing"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

func TestMerchantFollowUpMutationAndAuditAreAtomic(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.MerchantFollowUp{}, &model.AuditLog{}, &model.AdminUser{}); err != nil {
		t.Fatalf("migrate synthetic follow-up audit schema: %v", err)
	}
	actor := createAuditTestActor(t, db)
	merchant := model.Merchant{Name: "合成跟进商家", Status: "active"}
	if err := db.Create(&merchant).Error; err != nil {
		t.Fatalf("create synthetic merchant: %v", err)
	}

	const privateNote = "SYNTHETIC-PRIVATE-FOLLOW-UP-NOTE-DO-NOT-AUDIT 15500001111"
	admin := NewAdminService(nil, db)
	request := dto.CreateMerchantFollowUpRequest{
		Type:     "refund",
		Priority: "urgent",
		Content:  privateNote,
	}
	dropFailure := failAuditInserts(t, db)
	if _, err := admin.CreateMerchantFollowUp(merchant.ID, actor.ID, "admin", request, newAuditTestEvent(actor, "merchant_follow_up_create", nil)); err == nil {
		t.Fatal("follow-up creation succeeded despite audit storage failure")
	}
	var followUpCount int64
	if err := db.Model(&model.MerchantFollowUp{}).Count(&followUpCount).Error; err != nil {
		t.Fatalf("count follow-ups after failed create: %v", err)
	}
	if followUpCount != 0 || countAuditLogs(t, db) != 0 {
		t.Fatalf("failed create left partial state: follow-ups=%d audits=%d", followUpCount, countAuditLogs(t, db))
	}
	dropFailure()

	created, err := admin.CreateMerchantFollowUp(merchant.ID, actor.ID, "admin", request, newAuditTestEvent(actor, "merchant_follow_up_create", nil))
	if err != nil {
		t.Fatalf("create follow-up with audit: %v", err)
	}
	if created.ID == 0 || created.Content != privateNote || created.Type != "refund" || created.Priority != "urgent" || created.Status != "open" {
		t.Fatalf("successful creation changed follow-up semantics: %+v", created)
	}
	if countAuditLogs(t, db) != 1 {
		t.Fatalf("successful create audit count=%d, want 1", countAuditLogs(t, db))
	}

	dropFailure = failAuditInserts(t, db)
	if _, err := admin.UpdateMerchantFollowUpStatus(created.ID, "closed", newAuditTestEvent(actor, "merchant_follow_up_status_update", nil)); err == nil {
		t.Fatal("follow-up status update succeeded despite audit storage failure")
	}
	var afterFailedUpdate model.MerchantFollowUp
	if err := db.First(&afterFailedUpdate, created.ID).Error; err != nil {
		t.Fatalf("reload follow-up after failed status update: %v", err)
	}
	if afterFailedUpdate.Status != "open" || afterFailedUpdate.Content != privateNote || countAuditLogs(t, db) != 1 {
		t.Fatalf("failed status update left partial state: status=%q audits=%d", afterFailedUpdate.Status, countAuditLogs(t, db))
	}
	dropFailure()

	updated, err := admin.UpdateMerchantFollowUpStatus(created.ID, "closed", newAuditTestEvent(actor, "merchant_follow_up_status_update", nil))
	if err != nil {
		t.Fatalf("update follow-up status with audit: %v", err)
	}
	if updated.Status != "closed" || updated.Content != privateNote || countAuditLogs(t, db) != 2 {
		t.Fatalf("successful status update missing business state/audit: status=%q audits=%d", updated.Status, countAuditLogs(t, db))
	}

	var logs []model.AuditLog
	if err := db.Order("id asc").Find(&logs).Error; err != nil {
		t.Fatalf("load synthetic follow-up audit rows: %v", err)
	}
	wantActions := []string{"merchant_follow_up_create", "merchant_follow_up_status_update"}
	wantPurposes := []string{"创建商家跟进记录", "更新商家跟进状态"}
	for i, log := range logs {
		if log.ActorID != actor.ID || log.ActorType != actor.Type || log.TargetType != "merchant_follow_up" || log.TargetID != created.ID || log.MerchantID == nil || *log.MerchantID != merchant.ID {
			t.Errorf("follow-up audit row %d has unexpected attribution/target: %+v", i, log)
		}
		if log.Action != wantActions[i] {
			t.Errorf("follow-up audit row %d action=%q, want %q", i, log.Action, wantActions[i])
		}
		var detail map[string]string
		if err := json.Unmarshal([]byte(log.Detail), &detail); err != nil {
			t.Errorf("decode follow-up audit row %d: %v (%s)", i, err, log.Detail)
			continue
		}
		if len(detail) != 1 || detail["purpose"] != wantPurposes[i] {
			t.Errorf("follow-up audit row %d should contain only its purpose: %s", i, log.Detail)
		}
		if strings.Contains(log.Detail, privateNote) || strings.Contains(log.Detail, "15500001111") {
			t.Errorf("follow-up audit row %d contains private follow-up/customer data: %s", i, log.Detail)
		}
	}
}
