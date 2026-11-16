package service

import (
	"testing"
	"time"

	"go-web-gin-health/internal/model"
)

func TestPrivacyRetentionDryRunIsReadOnlyAndUsesExplicitTimeAnchors(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.VerificationCode{}, &model.EmailVerificationCode{}, &model.AuditLog{}); err != nil {
		t.Fatalf("migrate synthetic retention tables: %v", err)
	}
	now := time.Date(2026, time.November, 15, 12, 0, 0, 0, time.UTC)
	codes := []model.VerificationCode{
		{Phone: "15500000001", Scene: "login", Code: "REDACTED", ExpiresAt: now.Add(-25 * time.Hour)},
		{Phone: "15500000002", Scene: "login", Code: "REDACTED", ExpiresAt: now.Add(-23 * time.Hour)},
	}
	for i := range codes {
		if err := db.Create(&codes[i]).Error; err != nil {
			t.Fatalf("create synthetic SMS verification record: %v", err)
		}
	}
	emails := []model.EmailVerificationCode{
		{Email: "expired@example.invalid", Scene: "bind_email", Code: "synthetic-code", ExpiresAt: now.Add(-25 * time.Hour)},
		{Email: "recent@example.invalid", Scene: "bind_email", Code: "synthetic-code", ExpiresAt: now.Add(-23 * time.Hour)},
	}
	for i := range emails {
		if err := db.Create(&emails[i]).Error; err != nil {
			t.Fatalf("create synthetic email verification record: %v", err)
		}
	}
	audits := []model.AuditLog{
		{BaseModel: model.BaseModel{CreatedAt: now.AddDate(-3, 0, -1), UpdatedAt: now.AddDate(-3, 0, -1)}, Action: "merchant_order_detail_view"},
		{BaseModel: model.BaseModel{CreatedAt: now.AddDate(-2, 0, 0), UpdatedAt: now.AddDate(-2, 0, 0)}, Action: "merchant_order_detail_view"},
		{BaseModel: model.BaseModel{CreatedAt: now.AddDate(-2, 0, -1), UpdatedAt: now.AddDate(-2, 0, -1)}, Action: "merchant_status_update"},
		{BaseModel: model.BaseModel{CreatedAt: now.AddDate(-1, 0, 0), UpdatedAt: now.AddDate(-1, 0, 0)}, Action: "merchant_status_update"},
	}
	for i := range audits {
		if err := db.Create(&audits[i]).Error; err != nil {
			t.Fatalf("create synthetic audit record: %v", err)
		}
	}

	plan, err := NewPrivacyRetentionService(db).DryRun(now)
	if err != nil {
		t.Fatalf("run synthetic retention dry-run: %v", err)
	}
	if plan.SMSOTPWouldExpire != 1 || plan.EmailOTPWouldExpire != 1 || plan.SensitiveAuditsWouldExpire != 1 || plan.GeneralAuditsWouldExpire != 1 {
		t.Fatalf("unexpected dry-run plan counts: %+v", plan)
	}
	var total int64
	for _, table := range []interface{}{&model.VerificationCode{}, &model.EmailVerificationCode{}, &model.AuditLog{}} {
		var count int64
		if err := db.Model(table).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		total += count
	}
	if total != 8 {
		t.Fatalf("dry-run must not delete or alter rows; remaining=%d, want 8", total)
	}
}
