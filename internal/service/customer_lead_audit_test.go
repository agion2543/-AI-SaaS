package service

import (
	"encoding/json"
	"strings"
	"testing"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

func TestAdminCustomerLeadUpdateAndAuditAreAtomicAndPIISafe(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.CustomerLead{}, &model.AuditLog{}, &model.AdminUser{}); err != nil {
		t.Fatalf("migrate synthetic customer lead audit schema: %v", err)
	}
	actor := createAuditTestActor(t, db)
	merchant := model.Merchant{Name: "合成线索审计商家", Status: "active"}
	otherMerchant := model.Merchant{Name: "合成无关商家", Status: "active"}
	if err := db.Create(&merchant).Error; err != nil {
		t.Fatalf("create synthetic merchant: %v", err)
	}
	if err := db.Create(&otherMerchant).Error; err != nil {
		t.Fatalf("create unrelated synthetic merchant: %v", err)
	}

	const privateName = "SYNTHETIC-PRIVATE-CUSTOMER-NAME"
	const privatePhone = "15500001234"
	const privateMessage = "SYNTHETIC-PRIVATE-LEAD-MESSAGE"
	const oldPrivateNote = "SYNTHETIC-OLD-PRIVATE-FOLLOW-UP-NOTE"
	const newPrivateNote = "SYNTHETIC-NEW-PRIVATE-FOLLOW-UP-NOTE"
	lead := model.CustomerLead{
		MerchantID:    merchant.ID,
		StoreID:       17,
		CustomerName:  privateName,
		CustomerPhone: privatePhone,
		Message:       privateMessage,
		FollowUpNote:  oldPrivateNote,
		Source:        "synthetic_test",
		Status:        "contacted",
	}
	if err := db.Create(&lead).Error; err != nil {
		t.Fatalf("create synthetic customer lead: %v", err)
	}

	admin := NewAdminService(nil, db)
	request := dto.UpdateCustomerLeadRequest{Status: "converted", FollowUpNote: newPrivateNote}
	if _, err := admin.UpdateMerchantLead(otherMerchant.ID, lead.ID, request, newAuditTestEvent(actor, "admin_customer_lead_update", nil)); err == nil {
		t.Fatal("customer lead update crossed the merchant boundary")
	}
	if countAuditLogs(t, db) != 0 {
		t.Fatal("cross-merchant update unexpectedly produced an audit row")
	}

	spoofedMerchantID := otherMerchant.ID
	event := newAuditTestEvent(actor, "untrusted_action", map[string]interface{}{
		"customer_name":  privateName,
		"customer_phone": privatePhone,
		"message":        privateMessage,
		"follow_up_note": newPrivateNote,
	})
	event.Input.TargetName = privateName
	event.Input.TargetType = "untrusted_target"
	event.Input.TargetID = 9999
	event.Input.MerchantID = &spoofedMerchantID
	updated, err := admin.UpdateMerchantLead(merchant.ID, lead.ID, request, event)
	if err != nil {
		t.Fatalf("update customer lead with audit: %v", err)
	}
	if updated.Status != "converted" || updated.FollowUpNote != newPrivateNote || updated.CustomerName != privateName || updated.CustomerPhone != privatePhone || updated.Message != privateMessage {
		t.Fatalf("successful update changed unintended lead fields: %+v", updated)
	}
	if countAuditLogs(t, db) != 1 {
		t.Fatalf("successful update produced %d audit rows, want 1", countAuditLogs(t, db))
	}

	var audit model.AuditLog
	if err := db.Where("action = ?", "admin_customer_lead_update").First(&audit).Error; err != nil {
		t.Fatalf("load customer lead update audit: %v", err)
	}
	if audit.ActorID != actor.ID || audit.ActorType != actor.Type || audit.Action != "admin_customer_lead_update" || audit.TargetType != "merchant_customer_lead" || audit.TargetID != lead.ID || audit.MerchantID == nil || *audit.MerchantID != merchant.ID {
		t.Fatalf("customer lead audit actor/target metadata mismatch: %+v", audit)
	}
	if audit.ActorName != "" || audit.TargetName != "" || audit.IP != "" || audit.UserAgent != "" {
		t.Errorf("customer lead audit must not store names, IP, or user-agent: %+v", audit)
	}
	if audit.Detail != `{"purpose":"更新商家顾客线索状态或跟进备注"}` {
		t.Errorf("customer lead audit should contain only its purpose, got %s", audit.Detail)
	}
	serialized, err := json.Marshal(audit)
	if err != nil {
		t.Fatalf("serialize customer lead audit for PII assertion: %v", err)
	}
	for _, privateValue := range []string{privateName, privatePhone, privateMessage, oldPrivateNote, newPrivateNote} {
		if strings.Contains(string(serialized), privateValue) {
			t.Errorf("customer lead audit contains private value %q: %s", privateValue, serialized)
		}
	}
}

func TestAdminCustomerLeadUpdateRollsBackWhenAuditInsertFails(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.CustomerLead{}, &model.AuditLog{}, &model.AdminUser{}); err != nil {
		t.Fatalf("migrate synthetic customer lead rollback schema: %v", err)
	}
	actor := createAuditTestActor(t, db)
	merchant := model.Merchant{Name: "合成回滚线索商家", Status: "active"}
	if err := db.Create(&merchant).Error; err != nil {
		t.Fatalf("create synthetic merchant: %v", err)
	}
	lead := model.CustomerLead{
		MerchantID:    merchant.ID,
		StoreID:       23,
		CustomerName:  "SYNTHETIC-ROLLBACK-CUSTOMER",
		CustomerPhone: "15500004321",
		Message:       "SYNTHETIC-ROLLBACK-MESSAGE",
		FollowUpNote:  "SYNTHETIC-ROLLBACK-OLD-NOTE",
		Source:        "synthetic_test",
		Status:        "new",
	}
	if err := db.Create(&lead).Error; err != nil {
		t.Fatalf("create synthetic customer lead: %v", err)
	}

	dropFailure := failAuditInserts(t, db)
	_, err := NewAdminService(nil, db).UpdateMerchantLead(merchant.ID, lead.ID,
		dto.UpdateCustomerLeadRequest{Status: "contacted", FollowUpNote: "SYNTHETIC-ROLLBACK-NEW-NOTE"},
		newAuditTestEvent(actor, "admin_customer_lead_update", nil),
	)
	dropFailure()
	if err == nil || !strings.Contains(err.Error(), "persist customer lead update audit") {
		t.Fatalf("update error=%v, want fail-closed audit persistence error", err)
	}

	var stored model.CustomerLead
	if err := db.First(&stored, lead.ID).Error; err != nil {
		t.Fatalf("reload customer lead after failed audit: %v", err)
	}
	if stored.Status != "new" || stored.FollowUpNote != "SYNTHETIC-ROLLBACK-OLD-NOTE" || stored.CustomerName != lead.CustomerName || stored.CustomerPhone != lead.CustomerPhone || stored.Message != lead.Message {
		t.Fatalf("audit failure left a partial customer lead mutation: %+v", stored)
	}
	if countAuditLogs(t, db) != 0 {
		t.Fatalf("failed audit insert left %d audit rows, want 0", countAuditLogs(t, db))
	}
}
