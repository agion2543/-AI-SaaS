package adminrecovery

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/utils"
)

func openRecoveryTestDB(t *testing.T) *gorm.DB {
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
	if err := db.AutoMigrate(&model.AdminRole{}, &model.AdminUser{}); err != nil {
		t.Fatalf("migrate synthetic admin tables: %v", err)
	}
	if err := db.Exec(`CREATE TABLE audit_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		created_at DATETIME NULL,
		updated_at DATETIME NULL,
		deleted_at DATETIME NULL,
		actor_id INTEGER NOT NULL DEFAULT 0,
		actor_type VARCHAR(30) NULL,
		actor_name VARCHAR(120) NULL,
		action VARCHAR(80) NULL,
		target_type VARCHAR(60) NULL,
		target_id INTEGER NOT NULL DEFAULT 0,
		target_name VARCHAR(160) NULL,
		merchant_id INTEGER NULL,
		ip VARCHAR(80) NULL,
		user_agent VARCHAR(255) NULL,
		detail JSON NULL
	)`).Error; err != nil {
		t.Fatalf("create synthetic legacy audit table: %v", err)
	}
	return db
}

func TestValidateLocalTarget(t *testing.T) {
	cases := []struct {
		name            string
		env             string
		dsn             string
		allowProduction bool
		valid           bool
	}{
		{name: "local loopback", env: "development", dsn: "user:password@tcp(127.0.0.1:3306)/test", valid: true},
		{name: "local IPv4 loopback range", env: "development", dsn: "user:password@tcp(127.0.0.2:3306)/test", valid: true},
		{name: "local IPv6 loopback", env: "test", dsn: "user:password@tcp([::1]:3306)/test", valid: true},
		{name: "local tcp4 loopback", env: "test", dsn: "user:password@tcp4(127.0.0.1:3306)/test", valid: true},
		{name: "local tcp6 loopback", env: "test", dsn: "user:password@tcp6([::1]:3306)/test", valid: true},
		{name: "local Unix socket", env: "development", dsn: "user:password@unix(/tmp/mysql.sock)/test", valid: true},
		{name: "production refused", env: "production", dsn: "user:password@tcp(127.0.0.1:3306)/test", valid: false},
		{name: "production explicitly confirmed locally", env: "production", dsn: "user:password@tcp(127.0.0.1:3306)/test", allowProduction: true, valid: true},
		{name: "production still refuses remote", env: "production", dsn: "user:password@tcp(db.example.invalid:3306)/test", allowProduction: true, valid: false},
		{name: "hostname refused", env: "development", dsn: "user:password@tcp(localhost:3306)/test", valid: false},
		{name: "remote host refused", env: "development", dsn: "user:password@tcp(db.example.invalid:3306)/test", valid: false},
		{name: "remote IPv4 refused", env: "development", dsn: "user:password@tcp(192.0.2.10:3306)/test", valid: false},
		{name: "loopback marker in password does not allow remote", env: "development", dsn: "user:secret@tcp(127.0.0.1:3306)@tcp(db.example.invalid:3306)/test", valid: false},
		{name: "invalid port refused", env: "development", dsn: "user:password@tcp(127.0.0.1:invalid)/test", valid: false},
		{name: "malformed address refused", env: "development", dsn: "user:password@tcp([::1:3306)/test", valid: false},
		{name: "malformed DSN refused", env: "development", dsn: "user:password@tcp(127.0.0.1:3306)", valid: false},
		{name: "malformed DSN parameter refused", env: "development", dsn: "user:password@tcp(127.0.0.1:3306)/test?parseTime=invalid", valid: false},
		{name: "empty DSN refused", env: "development", dsn: "", valid: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateLocalTarget(tc.env, tc.dsn, tc.allowProduction)
			if tc.valid && err != nil {
				t.Fatalf("ValidateLocalTarget() error = %v", err)
			}
			if !tc.valid && err == nil {
				t.Fatal("ValidateLocalTarget() unexpectedly accepted unsafe target")
			}
		})
	}
}

func TestResetExistingSuperAdminPasswordAdvancesAuthVersion(t *testing.T) {
	db := openRecoveryTestDB(t)
	superRole := model.AdminRole{Name: "Super Admin", Code: "super_admin"}
	ordinaryRole := model.AdminRole{Name: "Read Only", Code: "read_only", Permissions: "read_only"}
	if err := db.Create(&superRole).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&ordinaryRole).Error; err != nil {
		t.Fatal(err)
	}
	oldHash, err := utils.HashPassword("old-test-password-123")
	if err != nil {
		t.Fatal(err)
	}
	admin := model.AdminUser{Username: "synthetic-super", DisplayName: "合成超管", PasswordHash: oldHash, Status: "disabled", RoleID: superRole.ID}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatal(err)
	}

	if err := ResetExistingSuperAdminPassword(db, admin.Username, "new-test-password-456"); err != nil {
		t.Fatalf("reset existing synthetic superadmin: %v", err)
	}
	var saved model.AdminUser
	if err := db.Preload("Role").First(&saved, admin.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.Status != "disabled" || saved.RoleID != superRole.ID || saved.Role.Code != "super_admin" || saved.Username != admin.Username || saved.DisplayName != admin.DisplayName {
		t.Fatalf("recovery changed protected account fields: %#v", saved)
	}
	if saved.AuthVersion != admin.AuthVersion+1 {
		t.Fatalf("recovery auth_version=%d, want %d", saved.AuthVersion, admin.AuthVersion+1)
	}
	if saved.PasswordHash == oldHash || !utils.CheckPassword(saved.PasswordHash, "new-test-password-456") {
		t.Fatal("new password hash was not applied correctly")
	}
	var audit model.AuditLog
	if err := db.Where("action = ?", "admin_password_recovery").First(&audit).Error; err != nil {
		t.Fatalf("load synthetic recovery audit: %v", err)
	}
	if audit.ActorID != 0 || audit.ActorType != "local_recovery" || audit.TargetType != "admin_user" || audit.TargetID != admin.ID || audit.CreatedAt.IsZero() || audit.Detail != `{"result":"success"}` {
		t.Fatalf("recovery audit metadata mismatch: %+v", audit)
	}
	if audit.ActorName != "" || audit.TargetName != "" || audit.IP != "" || audit.UserAgent != "" || audit.MerchantID != nil {
		t.Fatalf("recovery audit contains unnecessary identifying metadata: %+v", audit)
	}
}

func TestResetExistingSuperAdminPasswordRollsBackWhenAuditInsertFails(t *testing.T) {
	db := openRecoveryTestDB(t)
	role := model.AdminRole{Name: "Super Admin", Code: "super_admin"}
	if err := db.Create(&role).Error; err != nil {
		t.Fatal(err)
	}
	hash, err := utils.HashPassword("synthetic-original-recovery-password-123")
	if err != nil {
		t.Fatal(err)
	}
	admin := model.AdminUser{Username: "synthetic-recovery-audit-failure", DisplayName: "合成超管", PasswordHash: hash, Status: "active", RoleID: role.ID}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TRIGGER synthetic_fail_admin_recovery_audit BEFORE INSERT ON audit_logs BEGIN SELECT RAISE(ABORT, 'synthetic audit storage failure'); END").Error; err != nil {
		t.Fatalf("install synthetic audit failure trigger: %v", err)
	}

	if err := ResetExistingSuperAdminPassword(db, admin.Username, "synthetic-new-recovery-password-456"); err == nil {
		t.Fatal("recovery succeeded despite audit storage failure")
	}
	var saved model.AdminUser
	if err := db.First(&saved, admin.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.PasswordHash != hash || saved.AuthVersion != admin.AuthVersion {
		t.Fatalf("audit failure left account changed: password_hash_changed=%t auth_version=%d want=%d", saved.PasswordHash != hash, saved.AuthVersion, admin.AuthVersion)
	}
	var auditCount int64
	if err := db.Model(&model.AuditLog{}).Count(&auditCount).Error; err != nil {
		t.Fatalf("count synthetic recovery audits: %v", err)
	}
	if auditCount != 0 {
		t.Fatalf("failed recovery left audit records: count=%d", auditCount)
	}
}

func TestResetExistingSuperAdminPasswordRefusesMissingOrOrdinaryAccount(t *testing.T) {
	db := openRecoveryTestDB(t)
	role := model.AdminRole{Name: "Operator", Code: "operator", Permissions: "operations"}
	if err := db.Create(&role).Error; err != nil {
		t.Fatal(err)
	}
	hash, err := utils.HashPassword("ordinary-test-password-123")
	if err != nil {
		t.Fatal(err)
	}
	ordinary := model.AdminUser{Username: "synthetic-operator", DisplayName: "合成运营", PasswordHash: hash, Status: "active", RoleID: role.ID}
	if err := db.Create(&ordinary).Error; err != nil {
		t.Fatal(err)
	}

	if err := ResetExistingSuperAdminPassword(db, "missing-super", "new-test-password-456"); err == nil {
		t.Fatal("missing account unexpectedly accepted")
	}
	if err := ResetExistingSuperAdminPassword(db, ordinary.Username, "new-test-password-456"); err == nil {
		t.Fatal("ordinary admin unexpectedly accepted")
	}
	var saved model.AdminUser
	if err := db.First(&saved, ordinary.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.PasswordHash != hash || saved.RoleID != role.ID || saved.Status != "active" {
		t.Fatal("refused recovery changed ordinary admin data")
	}
	var count int64
	if err := db.Model(&model.AdminUser{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("recovery created an account: count=%d", count)
	}
}
