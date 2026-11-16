package service

import (
	"encoding/json"
	"strings"
	"testing"

	"go-web-gin-health/internal/model"
)

func TestAdminAccessAuditIsAtomicAndContainsOnlyReviewMetadata(t *testing.T) {
	db := openAdminAccessTestDB(t)
	service := NewAdminAccessService(db)
	superRole := model.AdminRole{Name: "合成超管角色", Code: "super_admin", Permissions: "system_admin"}
	if err := db.Create(&superRole).Error; err != nil {
		t.Fatalf("seed synthetic super-admin role: %v", err)
	}
	superAdmin := model.AdminUser{Username: "synthetic-audit-super", DisplayName: "合成超管", Status: "active", RoleID: superRole.ID}
	if err := db.Create(&superAdmin).Error; err != nil {
		t.Fatalf("seed synthetic super-admin: %v", err)
	}
	actor := AuditActor{ID: superAdmin.ID, Type: "super_admin"}

	role := model.AdminRole{Name: "合成运营角色", Code: "operator", Permissions: "operations"}
	otherRole := model.AdminRole{Name: "合成只读角色", Code: "reader", Permissions: "read_only"}
	for _, seedRole := range []*model.AdminRole{&role, &otherRole} {
		if err := db.Create(seedRole).Error; err != nil {
			t.Fatalf("seed synthetic role: %v", err)
		}
	}
	admin := model.AdminUser{
		Username: "synthetic-audit-existing", DisplayName: "合成现有账号", PasswordHash: "synthetic-existing-password-hash",
		Status: "active", RoleID: role.ID,
	}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("seed synthetic admin: %v", err)
	}

	if _, err := service.CreateRole(SaveAdminRoleInput{Name: "无权创建", Code: "denied_role", Permissions: []string{"read_only"}}, AuditActor{ID: 702, Type: "admin"}); err == nil {
		t.Fatal("ordinary admin unexpectedly passed the service-level super_admin check")
	}
	var unauthorizedRoles int64
	if err := db.Model(&model.AdminRole{}).Where("code = ?", "denied_role").Count(&unauthorizedRoles).Error; err != nil {
		t.Fatal(err)
	}
	if unauthorizedRoles != 0 || countAuditLogs(t, db) != 0 {
		t.Fatalf("unauthorized operation left state or audit: roles=%d audits=%d", unauthorizedRoles, countAuditLogs(t, db))
	}
	if _, err := service.CreateRole(SaveAdminRoleInput{Name: "伪造操作者", Code: "forged_role", Permissions: []string{"read_only"}}, AuditActor{ID: admin.ID, Type: "super_admin"}); err == nil {
		t.Fatal("ordinary admin with a forged super_admin actor type unexpectedly created a role")
	}
	var forgedRoles int64
	if err := db.Model(&model.AdminRole{}).Where("code = ?", "forged_role").Count(&forgedRoles).Error; err != nil {
		t.Fatal(err)
	}
	if forgedRoles != 0 || countAuditLogs(t, db) != 0 {
		t.Fatalf("forged actor operation left state or audit: roles=%d audits=%d", forgedRoles, countAuditLogs(t, db))
	}

	dropFailure := failAuditInserts(t, db)
	if _, err := service.CreateRole(SaveAdminRoleInput{Name: "审计失败角色", Code: "audit_role", Permissions: []string{"finance"}}, actor); err == nil {
		t.Fatal("role creation succeeded despite audit failure")
	}
	if _, err := service.CreateAdmin(CreateAdminAccountInput{Username: "synthetic-audit-new", DisplayName: "合成新账号", Password: "synthetic-password-123", RoleID: otherRole.ID}, actor); err == nil {
		t.Fatal("admin creation succeeded despite audit failure")
	}
	if _, err := service.UpdateRole(role.ID, SaveAdminRoleInput{Name: role.Name, Permissions: []string{"finance"}}, actor); err == nil {
		t.Fatal("role permission update succeeded despite audit failure")
	}
	if _, err := service.UpdateAdmin(admin.ID, UpdateAdminAccountInput{DisplayName: "合成更新账号", Status: "disabled", RoleID: otherRole.ID}, actor); err == nil {
		t.Fatal("admin account update succeeded despite audit failure")
	}

	var roleAfterFailure model.AdminRole
	var adminAfterFailure model.AdminUser
	var newAccounts int64
	var newRoles int64
	if err := db.First(&roleAfterFailure, role.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&adminAfterFailure, admin.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.AdminUser{}).Where("username = ?", "synthetic-audit-new").Count(&newAccounts).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.AdminRole{}).Where("code = ?", "audit_role").Count(&newRoles).Error; err != nil {
		t.Fatal(err)
	}
	if roleAfterFailure.Permissions != "operations" || adminAfterFailure.Status != "active" || adminAfterFailure.RoleID != role.ID || adminAfterFailure.AuthVersion != admin.AuthVersion || newAccounts != 0 || newRoles != 0 || countAuditLogs(t, db) != 0 {
		t.Fatalf("audit failure left partial changes: role=%+v admin=%+v new_accounts=%d new_roles=%d audits=%d", roleAfterFailure, adminAfterFailure, newAccounts, newRoles, countAuditLogs(t, db))
	}
	dropFailure()

	if _, err := service.CreateRole(SaveAdminRoleInput{Name: "审计角色", Code: "audit_role", Permissions: []string{"finance"}}, actor); err != nil {
		t.Fatalf("create audited role: %v", err)
	}
	if _, err := service.CreateAdmin(CreateAdminAccountInput{Username: "synthetic-audit-new", DisplayName: "合成新账号", Password: "synthetic-password-123", RoleID: otherRole.ID}, actor); err != nil {
		t.Fatalf("create audited admin: %v", err)
	}
	if _, err := service.UpdateRole(role.ID, SaveAdminRoleInput{Name: "合成运营角色", Permissions: []string{"finance"}}, actor); err != nil {
		t.Fatalf("update audited role: %v", err)
	}
	if _, err := service.UpdateAdmin(admin.ID, UpdateAdminAccountInput{DisplayName: "合成更新账号", Status: "disabled", RoleID: otherRole.ID}, actor); err != nil {
		t.Fatalf("update audited admin: %v", err)
	}

	var logs []model.AuditLog
	if err := db.Order("id asc").Find(&logs).Error; err != nil {
		t.Fatal(err)
	}
	if len(logs) != 4 {
		t.Fatalf("successful mutations produced %d audit rows, want 4", len(logs))
	}
	wantActions := []string{"admin_role_create", "admin_account_create", "admin_role_update", "admin_account_update"}
	for i, log := range logs {
		if log.Action != wantActions[i] || log.ActorID != actor.ID || log.ActorType != "super_admin" || log.TargetID == 0 || log.TargetName != "" || log.ActorName != "" || log.IP != "" || log.UserAgent != "" {
			t.Fatalf("audit row %d has unexpected attribution or identifiers: %+v", i, log)
		}
		var detail map[string]interface{}
		if err := json.Unmarshal([]byte(log.Detail), &detail); err != nil {
			t.Fatalf("decode audit row %d detail: %v (%s)", i, err, log.Detail)
		}
		if detail["purpose"] == "" {
			t.Fatalf("audit row %d has no purpose: %s", i, log.Detail)
		}
		if strings.Contains(strings.ToLower(log.Detail), "password") || strings.Contains(log.Detail, "synthetic-audit-existing") || strings.Contains(log.Detail, "synthetic-audit-new") || strings.Contains(log.Detail, "合成现有账号") || strings.Contains(log.Detail, "合成新账号") {
			t.Fatalf("audit row %d contains account identity or password material: %s", i, log.Detail)
		}
		before, _ := detail["before"].(map[string]interface{})
		after, _ := detail["after"].(map[string]interface{})
		switch log.Action {
		case "admin_role_create":
			if detail["before"] != nil || after["code"] != "audit_role" || after["name"] != "审计角色" {
				t.Fatalf("role creation audit snapshot mismatch: %s", log.Detail)
			}
		case "admin_account_create":
			if detail["before"] != nil || after["status"] != "active" || after["role_code"] != "reader" || after["role_id"] != float64(otherRole.ID) {
				t.Fatalf("admin creation audit snapshot mismatch: %s", log.Detail)
			}
		case "admin_role_update":
			if before["name"] != "合成运营角色" || after["permissions"] == nil || before["permissions"] == nil {
				t.Fatalf("role update audit snapshot mismatch: %s", log.Detail)
			}
		case "admin_account_update":
			if before["status"] != "active" || before["role_code"] != "operator" || after["status"] != "disabled" || after["role_code"] != "reader" {
				t.Fatalf("admin update audit snapshot mismatch: %s", log.Detail)
			}
		}
	}

	var updatedAdmin model.AdminUser
	if err := db.First(&updatedAdmin, admin.ID).Error; err != nil {
		t.Fatal(err)
	}
	if updatedAdmin.AuthVersion != admin.AuthVersion+2 {
		t.Fatalf("permission and account changes auth_version=%d, want %d", updatedAdmin.AuthVersion, admin.AuthVersion+2)
	}
}
