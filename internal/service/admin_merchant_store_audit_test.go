package service

import (
	"encoding/json"
	"strings"
	"testing"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

func spoofedMerchantStoreAuditEvent(actor AuditActor) AuditEvent {
	spoofedMerchantID := uint(99999)
	return AuditEvent{
		Actor: actor,
		Input: AuditRecordInput{
			Action:     "untrusted_action",
			TargetType: "customer",
			TargetID:   88888,
			TargetName: "SYNTHETIC-UNTRUSTED-TARGET-NAME",
			MerchantID: &spoofedMerchantID,
			Detail: map[string]interface{}{
				"store_name": "SYNTHETIC-UNTRUSTED-STORE-NAME",
				"phone":      "15500009999",
				"secret":     "SYNTHETIC-UNTRUSTED-SECRET",
			},
		},
	}
}

func TestAdminMerchantStoreMutationsAreAtomicAndPIISafe(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.AuditLog{}); err != nil {
		t.Fatalf("migrate synthetic store audit table: %v", err)
	}
	merchant := model.Merchant{Name: "合成门店审计商家", Status: "active"}
	if err := db.Create(&merchant).Error; err != nil {
		t.Fatalf("create synthetic merchant: %v", err)
	}
	actor := AuditActor{ID: 713, Type: "admin", IP: "192.0.2.91", UserAgent: "synthetic-store-audit-test"}
	admin := NewAdminService(nil, db)

	const initialName = "SYNTHETIC-PRIVATE-STORE-NAME"
	const initialAddress = "SYNTHETIC-PRIVATE-STORE-ADDRESS"
	const initialPhone = "15500001234"
	const updatedName = "SYNTHETIC-UPDATED-PRIVATE-STORE-NAME"
	const updatedAddress = "SYNTHETIC-UPDATED-PRIVATE-STORE-ADDRESS"
	const updatedPhone = "15500005678"
	initialIsOpen := true
	createRequest := dto.AdminSaveStoreRequest{
		Name: initialName, Address: initialAddress, ContactPhone: initialPhone,
		Status: "active", IsOpen: &initialIsOpen, BusinessHours: "SYNTHETIC-PRIVATE-HOURS",
		PauseReason: "SYNTHETIC-PRIVATE-PAUSE-REASON",
	}

	dropFailure := failAuditInserts(t, db)
	if _, err := admin.SaveMerchantStore(merchant.ID, 0, createRequest, spoofedMerchantStoreAuditEvent(actor)); err == nil {
		t.Fatal("store creation succeeded despite audit storage failure")
	}
	var storesAfterFailedCreate int64
	if err := db.Model(&model.Store{}).Where("merchant_id = ?", merchant.ID).Count(&storesAfterFailedCreate).Error; err != nil {
		t.Fatalf("count stores after failed create: %v", err)
	}
	if storesAfterFailedCreate != 0 || countAuditLogs(t, db) != 0 {
		t.Fatalf("failed create left a store or audit row: stores=%d audits=%d", storesAfterFailedCreate, countAuditLogs(t, db))
	}
	dropFailure()

	created, err := admin.SaveMerchantStore(merchant.ID, 0, createRequest, spoofedMerchantStoreAuditEvent(actor))
	if err != nil {
		t.Fatalf("create store with audit: %v", err)
	}
	if created.ID == 0 || created.Name != initialName || countAuditLogs(t, db) != 1 {
		t.Fatalf("successful create missing store or audit: store=%+v audits=%d", created, countAuditLogs(t, db))
	}

	updatedIsOpen := false
	updateRequest := dto.AdminSaveStoreRequest{
		Name: updatedName, Address: updatedAddress, ContactPhone: updatedPhone,
		Status: "active", IsOpen: &updatedIsOpen, BusinessHours: "SYNTHETIC-UPDATED-HOURS",
		PauseReason: "SYNTHETIC-UPDATED-PAUSE-REASON",
	}
	dropFailure = failAuditInserts(t, db)
	if _, err := admin.SaveMerchantStore(merchant.ID, created.ID, updateRequest, spoofedMerchantStoreAuditEvent(actor)); err == nil {
		t.Fatal("store update succeeded despite audit storage failure")
	}
	var afterFailedUpdate model.Store
	if err := db.First(&afterFailedUpdate, created.ID).Error; err != nil {
		t.Fatalf("load store after failed update: %v", err)
	}
	if afterFailedUpdate.Name != initialName || afterFailedUpdate.Address != initialAddress || afterFailedUpdate.ContactPhone != initialPhone || !afterFailedUpdate.IsOpen || countAuditLogs(t, db) != 1 {
		t.Fatalf("audit failure left partial store update: store=%+v audits=%d", afterFailedUpdate, countAuditLogs(t, db))
	}
	dropFailure()

	updated, err := admin.SaveMerchantStore(merchant.ID, created.ID, updateRequest, spoofedMerchantStoreAuditEvent(actor))
	if err != nil {
		t.Fatalf("update store with audit: %v", err)
	}
	if updated.Name != updatedName || updated.Address != updatedAddress || updated.ContactPhone != updatedPhone || updated.IsOpen || countAuditLogs(t, db) != 2 {
		t.Fatalf("successful update missing persisted fields or audit: store=%+v audits=%d", updated, countAuditLogs(t, db))
	}

	dropFailure = failAuditInserts(t, db)
	if err := admin.DisableMerchantStore(merchant.ID, created.ID, spoofedMerchantStoreAuditEvent(actor)); err == nil {
		t.Fatal("store disable succeeded despite audit storage failure")
	}
	var afterFailedDisable model.Store
	if err := db.First(&afterFailedDisable, created.ID).Error; err != nil {
		t.Fatalf("load store after failed disable: %v", err)
	}
	if afterFailedDisable.Status != "active" || countAuditLogs(t, db) != 2 {
		t.Fatalf("audit failure left partial store disable: status=%q audits=%d", afterFailedDisable.Status, countAuditLogs(t, db))
	}
	dropFailure()

	if err := admin.DisableMerchantStore(merchant.ID, created.ID, spoofedMerchantStoreAuditEvent(actor)); err != nil {
		t.Fatalf("disable store with audit: %v", err)
	}
	var disabled model.Store
	if err := db.First(&disabled, created.ID).Error; err != nil {
		t.Fatalf("load disabled store: %v", err)
	}
	if disabled.Status != "inactive" || countAuditLogs(t, db) != 3 {
		t.Fatalf("successful disable missing persisted status or audit: status=%q audits=%d", disabled.Status, countAuditLogs(t, db))
	}

	wantActions := []string{"admin_merchant_store_create", "admin_merchant_store_update", "admin_merchant_store_disable"}
	wantPurposes := []string{"平台创建商家门店", "平台更新商家门店信息", "平台停用商家门店"}
	var audits []model.AuditLog
	if err := db.Order("id asc").Find(&audits).Error; err != nil {
		t.Fatalf("load store audit rows: %v", err)
	}
	if len(audits) != len(wantActions) {
		t.Fatalf("loaded %d store audits, want %d", len(audits), len(wantActions))
	}
	for i, audit := range audits {
		if audit.ActorID != actor.ID || audit.ActorType != actor.Type || audit.Action != wantActions[i] || audit.TargetType != "merchant_store" || audit.TargetID != created.ID || audit.MerchantID == nil || *audit.MerchantID != merchant.ID {
			t.Errorf("audit row %d has unexpected actor/target attribution: %+v", i, audit)
		}
		if audit.ActorName != "" || audit.TargetName != "" || audit.IP != "" || audit.UserAgent != "" {
			t.Errorf("audit row %d must not store names, IP, or user-agent: %+v", i, audit)
		}
		if audit.Detail != `{"purpose":"`+wantPurposes[i]+`"}` {
			t.Errorf("audit row %d should contain only its purpose, got %s", i, audit.Detail)
		}
		encoded, err := json.Marshal(audit)
		if err != nil {
			t.Fatalf("serialize audit row %d: %v", i, err)
		}
		for _, forbidden := range []string{
			initialName, initialAddress, initialPhone, updatedName, updatedAddress, updatedPhone,
			"SYNTHETIC-PRIVATE-HOURS", "SYNTHETIC-PRIVATE-PAUSE-REASON",
			"SYNTHETIC-UPDATED-HOURS", "SYNTHETIC-UPDATED-PAUSE-REASON",
			"SYNTHETIC-UNTRUSTED-TARGET-NAME", "SYNTHETIC-UNTRUSTED-STORE-NAME",
			"15500009999", "SYNTHETIC-UNTRUSTED-SECRET",
		} {
			if strings.Contains(string(encoded), forbidden) {
				t.Errorf("audit row %d contains private store value %q: %s", i, forbidden, encoded)
			}
		}
	}
}

func TestAdminMerchantStoreMutationsCannotCrossMerchantBoundary(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.AuditLog{}); err != nil {
		t.Fatalf("migrate synthetic store audit table: %v", err)
	}
	owner := model.Merchant{Name: "合成门店归属商家", Status: "active"}
	other := model.Merchant{Name: "合成其他商家", Status: "active"}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatalf("create store owner: %v", err)
	}
	if err := db.Create(&other).Error; err != nil {
		t.Fatalf("create unrelated merchant: %v", err)
	}
	store := model.Store{MerchantID: owner.ID, Name: "SYNTHETIC-OWNED-STORE", ContactPhone: "15500001111", Status: "active"}
	if err := db.Create(&store).Error; err != nil {
		t.Fatalf("create synthetic store: %v", err)
	}
	admin := NewAdminService(nil, db)
	actor := AuditActor{ID: 714, Type: "admin"}
	request := dto.AdminSaveStoreRequest{Name: "SYNTHETIC-CROSS-MERCHANT-UPDATE", ContactPhone: "15500002222", Status: "inactive"}
	if _, err := admin.SaveMerchantStore(other.ID, store.ID, request, spoofedMerchantStoreAuditEvent(actor)); err == nil {
		t.Fatal("store update crossed its merchant boundary")
	}
	if err := admin.DisableMerchantStore(other.ID, store.ID, spoofedMerchantStoreAuditEvent(actor)); err == nil {
		t.Fatal("store disable crossed its merchant boundary")
	}
	var unchanged model.Store
	if err := db.First(&unchanged, store.ID).Error; err != nil {
		t.Fatalf("load store after cross-merchant attempts: %v", err)
	}
	if unchanged.MerchantID != owner.ID || unchanged.Name != store.Name || unchanged.ContactPhone != store.ContactPhone || unchanged.Status != "active" {
		t.Fatalf("cross-merchant attempt changed store: %+v", unchanged)
	}
	if countAuditLogs(t, db) != 0 {
		t.Fatalf("cross-merchant attempts unexpectedly produced %d audit rows", countAuditLogs(t, db))
	}
}
