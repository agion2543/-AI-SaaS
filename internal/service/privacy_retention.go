package service

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"go-web-gin-health/internal/model"
)

const retentionDryRunOTPWindow = 24 * time.Hour

var sensitiveAccessAuditActions = []string{
	"merchant_order_detail_view",
	"merchant_customer_lead_list_view",
}

type PrivacyRetentionPlan struct {
	AsOf                       time.Time `json:"as_of"`
	SMSOTPExpiryCutoff         time.Time `json:"sms_otp_expiry_cutoff"`
	EmailOTPExpiryCutoff       time.Time `json:"email_otp_expiry_cutoff"`
	SensitiveAuditCutoff       time.Time `json:"sensitive_audit_cutoff"`
	GeneralAuditCutoff         time.Time `json:"general_audit_cutoff"`
	SMSOTPWouldExpire          int64     `json:"sms_otp_would_expire"`
	EmailOTPWouldExpire        int64     `json:"email_otp_would_expire"`
	SensitiveAuditsWouldExpire int64     `json:"sensitive_audits_would_expire"`
	GeneralAuditsWouldExpire   int64     `json:"general_audits_would_expire"`
}

type PrivacyRetentionService struct {
	db *gorm.DB
}

func NewPrivacyRetentionService(db *gorm.DB) *PrivacyRetentionService {
	return &PrivacyRetentionService{db: db}
}

// DryRun is read-only. It reports counts and cutoffs for a proposed retention
// policy; it never deletes, updates, or anonymizes records.
func (s *PrivacyRetentionService) DryRun(now time.Time) (PrivacyRetentionPlan, error) {
	if s == nil || s.db == nil {
		return PrivacyRetentionPlan{}, errors.New("retention database is unavailable")
	}
	plan := PrivacyRetentionPlan{
		AsOf:                 now,
		SMSOTPExpiryCutoff:   now.Add(-retentionDryRunOTPWindow),
		EmailOTPExpiryCutoff: now.Add(-retentionDryRunOTPWindow),
		SensitiveAuditCutoff: now.AddDate(-3, 0, 0),
		GeneralAuditCutoff:   now.AddDate(-2, 0, 0),
	}
	queries := []struct {
		model interface{}
		where string
		args  []interface{}
		count *int64
	}{
		{&model.VerificationCode{}, "expires_at <= ?", []interface{}{plan.SMSOTPExpiryCutoff}, &plan.SMSOTPWouldExpire},
		{&model.EmailVerificationCode{}, "expires_at <= ?", []interface{}{plan.EmailOTPExpiryCutoff}, &plan.EmailOTPWouldExpire},
		{&model.AuditLog{}, "action IN ? AND created_at < ?", []interface{}{sensitiveAccessAuditActions, plan.SensitiveAuditCutoff}, &plan.SensitiveAuditsWouldExpire},
		{&model.AuditLog{}, "(action IS NULL OR action NOT IN ?) AND created_at < ?", []interface{}{sensitiveAccessAuditActions, plan.GeneralAuditCutoff}, &plan.GeneralAuditsWouldExpire},
	}
	for _, query := range queries {
		if err := s.db.Model(query.model).Where(query.where, query.args...).Count(query.count).Error; err != nil {
			return PrivacyRetentionPlan{}, err
		}
	}
	return plan, nil
}
