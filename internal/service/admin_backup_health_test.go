package service

import (
	"strings"
	"testing"

	"go-web-gin-health/internal/config"
)

func TestBackupCheckDoesNotPassWithoutRecordedEvidence(t *testing.T) {
	item := (&AdminService{}).checkBackupScripts()
	if item.Key != "backup" || item.Status != "warning" {
		t.Fatalf("backup check must remain unverified without recorded evidence: %+v", item)
	}
	if !strings.Contains(item.Description, "未记录") || !strings.Contains(item.Description, "脚本文件存在不代表") {
		t.Fatalf("backup check must explain its evidence gap: %+v", item)
	}
}

func TestDeploymentBackupMetricsDoNotPassWithoutRecordedEvidence(t *testing.T) {
	service := &AdminService{cfg: &config.Config{
		AppEnv:      "production",
		AppURL:      "https://synthetic.example.invalid",
		FrontendURL: "https://synthetic.example.invalid",
		JWTSecret:   strings.Repeat("j", 40),
		AESSecret:   strings.Repeat("a", 32),
	}}
	block := service.deploymentReadinessHealthBlock()
	if block.Status != "warning" {
		t.Fatalf("deployment must not report pass without backup evidence: %+v", block)
	}

	wantLabels := map[string]bool{
		"仓库脚本文件": false,
		"备份执行证据": false,
		"恢复演练证据": false,
	}
	for _, metric := range block.Metrics {
		if _, tracked := wantLabels[metric.Label]; !tracked {
			continue
		}
		if metric.Status != "warning" {
			t.Errorf("%s should remain warning without evidence, got %q", metric.Label, metric.Status)
		}
		if metric.Label != "仓库脚本文件" && metric.Value != 0 {
			t.Errorf("%s should report no system-recorded evidence, got %d", metric.Label, metric.Value)
		}
		wantLabels[metric.Label] = true
	}
	for label, found := range wantLabels {
		if !found {
			t.Errorf("missing explicit backup evidence metric %q", label)
		}
	}
}
