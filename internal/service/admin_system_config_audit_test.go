package service

import (
	"encoding/json"
	"strings"
	"testing"

	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

func TestSaveSystemConfigIsAtomicAndAuditsOnlySafeMetadata(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.SystemConfig{}, &model.AuditLog{}, &model.AdminUser{}); err != nil {
		t.Fatalf("migrate synthetic system-config audit schema: %v", err)
	}
	actor := createAuditTestActor(t, db)
	if err := db.Create(&model.SystemConfig{ConfigKey: "site_name", ConfigValue: "之前的站点名"}).Error; err != nil {
		t.Fatalf("create synthetic prior configuration: %v", err)
	}
	service := NewAdminService(&config.Config{AESSecret: strings.Repeat("s", 32)}, db)
	req := dto.SaveSystemConfigRequest{
		SiteName:                 "合成新站点",
		PaymentGateway:           "SYNTHETIC-GATEWAY-SECRET-DO-NOT-AUDIT",
		PlatformAlipayQRCode:     "https://synthetic.invalid/alipay-qr-secret",
		PlatformWechatQRCode:     "https://synthetic.invalid/wechat-qr-secret",
		PlatformSubscriptionNote: "合成订阅说明",
		FilingInfo:               "合成备案内容",
		Notice:                   "合成公告",
	}

	dropFailure := failAuditInserts(t, db)
	if err := service.SaveSystemConfig(req, actor); err == nil {
		t.Fatal("system configuration save succeeded despite audit storage failure")
	}
	var prior model.SystemConfig
	if err := db.Where("config_key = ?", "site_name").First(&prior).Error; err != nil {
		t.Fatalf("reload synthetic prior configuration: %v", err)
	}
	var configCount int64
	if err := db.Model(&model.SystemConfig{}).Count(&configCount).Error; err != nil {
		t.Fatal(err)
	}
	if prior.ConfigValue != "之前的站点名" || configCount != 1 || countAuditLogs(t, db) != 0 {
		t.Fatalf("audit failure left partial system config state: prior=%q configs=%d audits=%d", prior.ConfigValue, configCount, countAuditLogs(t, db))
	}
	dropFailure()

	if err := service.SaveSystemConfig(req, actor); err != nil {
		t.Fatalf("save synthetic system configuration with audit: %v", err)
	}
	if err := db.Model(&model.SystemConfig{}).Count(&configCount).Error; err != nil {
		t.Fatal(err)
	}
	if configCount != 7 || countAuditLogs(t, db) != 1 {
		t.Fatalf("successful system config save produced configs=%d audits=%d, want 7 and 1", configCount, countAuditLogs(t, db))
	}
	var storedGateway model.SystemConfig
	if err := db.Where("config_key = ?", "payment_gateway").First(&storedGateway).Error; err != nil {
		t.Fatal(err)
	}
	if !storedGateway.IsEncrypted || strings.Contains(storedGateway.ConfigValue, req.PaymentGateway) {
		t.Fatal("payment gateway encryption semantics changed")
	}
	var audit model.AuditLog
	if err := db.Where("action = ?", "platform_system_config_update").First(&audit).Error; err != nil {
		t.Fatalf("load system config audit: %v", err)
	}
	assertSafeSystemConfigAudit(t, audit, actor, "平台系统配置更新", []string{
		"site_name", "payment_gateway", "platform_alipay_qr_code", "platform_wechat_qr_code",
		"platform_subscription_note", "filing_info", "notice",
	}, req.PaymentGateway, req.PlatformAlipayQRCode, req.PlatformWechatQRCode, req.SiteName, req.FilingInfo, req.Notice)
}

func TestSaveAIConfigPolicyIsAtomicAndAuditedWithoutEnablingAI(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.SystemConfig{}, &model.AuditLog{}, &model.AdminUser{}); err != nil {
		t.Fatalf("migrate synthetic AI-policy audit schema: %v", err)
	}
	actor := createAuditTestActor(t, db)
	if err := db.Create(&model.SystemConfig{ConfigKey: "ai_policy_provider", ConfigValue: "previous-provider"}).Error; err != nil {
		t.Fatalf("create synthetic prior AI policy: %v", err)
	}
	service := NewAdminService(nil, db)
	req := dto.SaveAIConfigPolicyRequest{
		Provider:                "synthetic-provider",
		BaseURL:                 "https://synthetic.invalid/api?token=SYNTHETIC-AI-TOKEN-DO-NOT-AUDIT",
		Model:                   "synthetic-model",
		DailyQuotaPerMerchant:   12,
		DailyCostLimitCents:     250,
		FailureRateAlertPercent: 15,
		TimeoutSeconds:          18,
		Remark:                  "SYNTHETIC-AI-REMARK-DO-NOT-AUDIT",
	}

	dropFailure := failAuditInserts(t, db)
	if _, err := service.SaveAIConfigPolicy(req, actor); err == nil {
		t.Fatal("AI policy save succeeded despite audit storage failure")
	}
	var prior model.SystemConfig
	if err := db.Where("config_key = ?", "ai_policy_provider").First(&prior).Error; err != nil {
		t.Fatalf("reload synthetic prior AI policy: %v", err)
	}
	var configCount int64
	if err := db.Model(&model.SystemConfig{}).Count(&configCount).Error; err != nil {
		t.Fatal(err)
	}
	if prior.ConfigValue != "previous-provider" || configCount != 1 || countAuditLogs(t, db) != 0 {
		t.Fatalf("audit failure left partial AI policy state: prior=%q configs=%d audits=%d", prior.ConfigValue, configCount, countAuditLogs(t, db))
	}
	dropFailure()

	policy, err := service.SaveAIConfigPolicy(req, actor)
	if err != nil {
		t.Fatalf("save synthetic AI policy with audit: %v", err)
	}
	if policy.Enabled || policy.Provider != "synthetic-provider" {
		t.Fatalf("AI provider usability semantics changed: %+v", policy)
	}
	if err := db.Model(&model.SystemConfig{}).Count(&configCount).Error; err != nil {
		t.Fatal(err)
	}
	if configCount != 9 || countAuditLogs(t, db) != 1 {
		t.Fatalf("successful AI policy save produced configs=%d audits=%d, want 9 and 1", configCount, countAuditLogs(t, db))
	}
	var enabled model.SystemConfig
	if err := db.Where("config_key = ?", "ai_policy_enabled").First(&enabled).Error; err != nil {
		t.Fatal(err)
	}
	if enabled.ConfigValue != "false" {
		t.Fatalf("AI policy unexpectedly enabled: stored value=%q", enabled.ConfigValue)
	}
	var audit model.AuditLog
	if err := db.Where("action = ?", "ai_config_policy_update").First(&audit).Error; err != nil {
		t.Fatalf("load AI policy audit: %v", err)
	}
	assertSafeSystemConfigAudit(t, audit, actor, "AI 策略配置更新", []string{
		"ai_policy_provider", "ai_policy_base_url", "ai_policy_model", "ai_policy_enabled",
		"ai_policy_daily_quota_per_merchant", "ai_policy_daily_cost_limit_cents",
		"ai_policy_failure_rate_alert_percent", "ai_policy_timeout_seconds", "ai_policy_remark",
	}, req.BaseURL, "SYNTHETIC-AI-TOKEN-DO-NOT-AUDIT", req.Model, req.Remark)

	req.Enabled = true
	if _, err := service.SaveAIConfigPolicy(req, actor); err == nil || !strings.Contains(err.Error(), "尚未接入") {
		t.Fatalf("unavailable AI provider was enabled or returned unexpected error: %v", err)
	}
	if countAuditLogs(t, db) != 1 {
		t.Fatal("rejected AI enable attempt unexpectedly wrote an audit record")
	}
}

func assertSafeSystemConfigAudit(t *testing.T, audit model.AuditLog, actor AuditActor, purpose string, wantKeys []string, forbiddenValues ...string) {
	t.Helper()
	var detail struct {
		Purpose      string   `json:"purpose"`
		UpdatedKeys  []string `json:"updated_keys"`
		UpdatedCount int      `json:"updated_count"`
	}
	if err := json.Unmarshal([]byte(audit.Detail), &detail); err != nil {
		t.Fatalf("decode config audit detail: %v (%s)", err, audit.Detail)
	}
	if audit.ActorID != actor.ID || audit.ActorType != actor.Type || audit.TargetType != "platform_system_config" || audit.TargetID != 0 {
		t.Fatalf("config audit attribution or target mismatch: %+v", audit)
	}
	if detail.Purpose != purpose || detail.UpdatedCount != len(wantKeys) || len(detail.UpdatedKeys) != len(wantKeys) {
		t.Fatalf("config audit purpose/count mismatch: %+v", detail)
	}
	for i := range wantKeys {
		if detail.UpdatedKeys[i] != wantKeys[i] {
			t.Fatalf("config audit key %d=%q, want %q", i, detail.UpdatedKeys[i], wantKeys[i])
		}
	}
	for _, value := range forbiddenValues {
		if value != "" && strings.Contains(audit.Detail, value) {
			t.Fatalf("config audit leaked sensitive value %q: %s", value, audit.Detail)
		}
	}
	if audit.IP != "" || audit.UserAgent != "" || audit.ActorName != "" || audit.TargetName != "" {
		t.Fatalf("config audit stored unnecessary identifying data: %+v", audit)
	}
}
