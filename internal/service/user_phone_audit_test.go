package service

import (
	"encoding/json"
	"strings"
	"testing"

	"go-web-gin-health/internal/model"
)

func TestAdminUserPhoneUpdateAndAuditAreAtomic(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.AuditLog{}, &model.AdminUser{}); err != nil {
		t.Fatalf("migrate synthetic phone audit schema: %v", err)
	}
	actor := createAuditTestActor(t, db)
	const oldPhone = "13800000111"
	const newPhone = "13800000222"
	const email = "private-user@example.test"
	user := model.User{Email: email, Phone: oldPhone, DisplayName: "合成手机号测试用户", Status: "active"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create synthetic user: %v", err)
	}

	event := newAuditTestEvent(actor, "unexpected_action", map[string]interface{}{
		"old_phone": oldPhone,
		"new_phone": newPhone,
		"email":     email,
	})
	if err := NewAdminService(nil, db).UpdateUserPhone(user.ID, newPhone, event); err != nil {
		t.Fatalf("update user phone with audit: %v", err)
	}

	var updated model.User
	if err := db.First(&updated, user.ID).Error; err != nil {
		t.Fatalf("reload synthetic user: %v", err)
	}
	if updated.Phone != newPhone {
		t.Fatalf("user phone=%q, want updated value", updated.Phone)
	}
	if countAuditLogs(t, db) != 1 {
		t.Fatalf("successful phone update produced %d audit rows, want 1", countAuditLogs(t, db))
	}
	var audit model.AuditLog
	if err := db.Where("action = ?", "user_phone_update").First(&audit).Error; err != nil {
		t.Fatalf("load user phone audit: %v", err)
	}
	if audit.ActorID != actor.ID || audit.ActorType != actor.Type || audit.Action != "user_phone_update" || audit.TargetType != "user" || audit.TargetID != user.ID || audit.TargetName != "" || audit.MerchantID != nil {
		t.Fatalf("phone audit has unexpected actor or target metadata: %+v", audit)
	}
	if audit.Detail != `{"purpose":"修改用户手机号"}` {
		t.Fatalf("phone audit detail=%s, want purpose only", audit.Detail)
	}
	serialized, err := json.Marshal(audit)
	if err != nil {
		t.Fatalf("serialize audit row for PII assertion: %v", err)
	}
	for _, privateValue := range []string{oldPhone, newPhone, email} {
		if strings.Contains(string(serialized), privateValue) {
			t.Errorf("audit row contains private user value %q: %s", privateValue, serialized)
		}
	}
}

func TestAdminUserPhoneUpdateRollsBackWhenAuditInsertFails(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.AuditLog{}, &model.AdminUser{}); err != nil {
		t.Fatalf("migrate synthetic phone audit schema: %v", err)
	}
	actor := createAuditTestActor(t, db)
	const oldPhone = "13800000333"
	const newPhone = "13800000444"
	user := model.User{Email: "rollback-user@example.test", Phone: oldPhone, Status: "active"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create synthetic rollback user: %v", err)
	}

	dropFailure := failAuditInserts(t, db)
	defer dropFailure()
	if err := NewAdminService(nil, db).UpdateUserPhone(user.ID, newPhone, newAuditTestEvent(actor, "user_phone_update", nil)); err == nil {
		t.Fatal("phone update succeeded despite audit storage failure")
	}

	var stored model.User
	if err := db.First(&stored, user.ID).Error; err != nil {
		t.Fatalf("reload synthetic rollback user: %v", err)
	}
	if stored.Phone != oldPhone {
		t.Fatalf("audit failure left phone updated to %q; want original value", stored.Phone)
	}
	if countAuditLogs(t, db) != 0 {
		t.Fatalf("failed audit insert left %d audit rows, want 0", countAuditLogs(t, db))
	}
}
