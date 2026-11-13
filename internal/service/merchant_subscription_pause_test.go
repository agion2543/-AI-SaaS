package service

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"go-web-gin-health/internal/model"
	"gorm.io/gorm"
)

type subscriptionPauseAuditDetail struct {
	Action    string                         `json:"action"`
	Reason    string                         `json:"reason"`
	Timestamp time.Time                      `json:"timestamp"`
	Before    merchantSubscriptionAuditState `json:"before"`
	After     merchantSubscriptionAuditState `json:"after"`
}

func createPauseTestMerchant(t *testing.T, db *gorm.DB, status string, expiry time.Time, reason string) *model.Merchant {
	t.Helper()
	merchant := &model.Merchant{
		Name:                    "合成订阅暂停测试商家-" + status,
		Status:                  "active",
		SubscriptionPlan:        "month",
		SubscriptionStatus:      status,
		SubscriptionExpireAt:    &expiry,
		SubscriptionPauseReason: reason,
	}
	if err := db.Create(merchant).Error; err != nil {
		t.Fatalf("create synthetic merchant: %v", err)
	}
	return merchant
}

func TestPauseMerchantSubscriptionRequiresNonEmptyReason(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.AuditLog{}); err != nil {
		t.Fatalf("migrate audit logs: %v", err)
	}
	expiry := time.Now().Add(48 * time.Hour)
	merchant := createPauseTestMerchant(t, db, "active", expiry, "")

	if _, err := NewAdminService(nil, db).PauseMerchantSubscription(merchant.ID, " \n\t ", MerchantSubscriptionActor{ID: 7, Type: "admin"}); err == nil {
		t.Fatal("pause without a non-empty reason succeeded")
	}
	var stored model.Merchant
	if err := db.First(&stored, merchant.ID).Error; err != nil {
		t.Fatalf("reload merchant: %v", err)
	}
	if stored.SubscriptionStatus != "active" || stored.SubscriptionPauseReason != "" || stored.SubscriptionPausedAt != nil || stored.SubscriptionExpireAt == nil || !stored.SubscriptionExpireAt.Equal(expiry) {
		t.Fatalf("rejected pause changed merchant: %+v", stored)
	}
	var audits int64
	if err := db.Model(&model.AuditLog{}).Count(&audits).Error; err != nil {
		t.Fatalf("count audit logs: %v", err)
	}
	if audits != 0 {
		t.Fatalf("audit count after rejected pause = %d, want 0", audits)
	}
}

func TestPauseAndResumeMerchantSubscriptionPreserveExpiryAndAuditState(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.AuditLog{}, &model.AdminUser{}); err != nil {
		t.Fatalf("migrate audit tables: %v", err)
	}
	expiry := time.Now().Add(72 * time.Hour).Truncate(time.Microsecond)
	merchant := createPauseTestMerchant(t, db, "active", expiry, "")
	actor := MerchantSubscriptionActor{ID: 17, Type: "admin", IP: "127.0.0.1", UserAgent: "subscription-test"}
	admin := NewAdminService(nil, db)

	paused, err := admin.PauseMerchantSubscription(merchant.ID, "商家申请临时暂停", actor)
	if err != nil {
		t.Fatalf("pause subscription: %v", err)
	}
	if paused.SubscriptionStatus != "paused" || paused.SubscriptionPauseReason != "商家申请临时暂停" || paused.SubscriptionPausedAt == nil {
		t.Fatalf("pause result does not contain explicit pause state: %+v", paused)
	}
	if paused.SubscriptionExpireAt == nil || !paused.SubscriptionExpireAt.Equal(expiry) {
		t.Fatalf("pause changed expiry to %v, want unchanged %v", paused.SubscriptionExpireAt, expiry)
	}
	if paused.HasActiveSubscriptionAt(time.Now()) {
		t.Fatal("paused subscription still grants paid access")
	}
	if _, err := admin.PauseMerchantSubscription(merchant.ID, "重复暂停", actor); err == nil {
		t.Fatal("repeated pause unexpectedly succeeded")
	}

	resumed, err := admin.ResumeMerchantSubscription(merchant.ID, actor)
	if err != nil {
		t.Fatalf("resume subscription: %v", err)
	}
	if resumed.SubscriptionStatus != "active" || resumed.SubscriptionPauseReason != "" || resumed.SubscriptionPausedAt != nil {
		t.Fatalf("resume result did not clear pause-only state: %+v", resumed)
	}
	if resumed.SubscriptionExpireAt == nil || !resumed.SubscriptionExpireAt.Equal(expiry) {
		t.Fatalf("resume changed expiry to %v, want unchanged %v", resumed.SubscriptionExpireAt, expiry)
	}
	if !resumed.HasActiveSubscriptionAt(time.Now()) {
		t.Fatal("resumed subscription did not restore paid access")
	}
	if _, err := admin.ResumeMerchantSubscription(merchant.ID, actor); err == nil {
		t.Fatal("repeated resume unexpectedly succeeded")
	}

	var logs []model.AuditLog
	if err := db.Order("created_at asc").Find(&logs).Error; err != nil {
		t.Fatalf("load audit logs: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("audit log count = %d, want pause and resume records", len(logs))
	}
	for i, wantAction := range []string{"merchant_subscription_pause", "merchant_subscription_resume"} {
		log := logs[i]
		if log.ActorID != actor.ID || log.ActorType != actor.Type || log.Action != wantAction || log.TargetID != merchant.ID || log.MerchantID == nil || *log.MerchantID != merchant.ID || log.CreatedAt.IsZero() {
			t.Fatalf("audit log %d missing actor, target, action, or timestamp: %+v", i, log)
		}
		var detail subscriptionPauseAuditDetail
		if err := json.Unmarshal([]byte(log.Detail), &detail); err != nil {
			t.Fatalf("decode audit detail: %v", err)
		}
		if detail.Action != wantAction || detail.Reason != "商家申请临时暂停" || detail.Timestamp.IsZero() {
			t.Fatalf("audit detail %d missing action/reason/timestamp: %+v", i, detail)
		}
		if detail.Before.Status == detail.After.Status || detail.Before.ExpireAt == nil || detail.After.ExpireAt == nil || !detail.Before.ExpireAt.Equal(expiry) || !detail.After.ExpireAt.Equal(expiry) {
			t.Fatalf("audit detail %d has invalid before/after states: %+v", i, detail)
		}
	}
	if logs[0].Action != "merchant_subscription_pause" {
		t.Fatalf("first audit action = %q, want pause", logs[0].Action)
	}
	var pauseDetail subscriptionPauseAuditDetail
	if err := json.Unmarshal([]byte(logs[0].Detail), &pauseDetail); err != nil {
		t.Fatalf("decode pause audit: %v", err)
	}
	if pauseDetail.Before.Status != "active" || pauseDetail.After.Status != "paused" {
		t.Fatalf("pause audit states = %q -> %q, want active -> paused", pauseDetail.Before.Status, pauseDetail.After.Status)
	}
	var resumeDetail subscriptionPauseAuditDetail
	if err := json.Unmarshal([]byte(logs[1].Detail), &resumeDetail); err != nil {
		t.Fatalf("decode resume audit: %v", err)
	}
	if resumeDetail.Before.Status != "paused" || resumeDetail.After.Status != "active" {
		t.Fatalf("resume audit states = %q -> %q, want paused -> active", resumeDetail.Before.Status, resumeDetail.After.Status)
	}
}

func TestResumeMerchantSubscriptionRejectsExpiredAndUnpausedSubscriptions(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.AuditLog{}); err != nil {
		t.Fatalf("migrate audit logs: %v", err)
	}
	future := time.Now().Add(48 * time.Hour)
	past := time.Now().Add(-time.Hour)
	cases := []struct {
		name   string
		status string
		expiry time.Time
	}{
		{name: "inactive cannot be resumed", status: "inactive", expiry: future},
		{name: "active cannot be resumed", status: "active", expiry: future},
		{name: "expired pause cannot be resumed", status: "paused", expiry: past},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			merchant := createPauseTestMerchant(t, db, test.status, test.expiry, "synthetic prior reason")
			if _, err := NewAdminService(nil, db).ResumeMerchantSubscription(merchant.ID, MerchantSubscriptionActor{ID: 7, Type: "admin"}); err == nil {
				t.Fatal("resume unexpectedly succeeded")
			}
			var stored model.Merchant
			if err := db.First(&stored, merchant.ID).Error; err != nil {
				t.Fatalf("reload merchant: %v", err)
			}
			if stored.SubscriptionStatus != test.status || stored.SubscriptionPauseReason != "synthetic prior reason" || stored.SubscriptionExpireAt == nil || !stored.SubscriptionExpireAt.Equal(test.expiry) {
				t.Fatalf("rejected resume changed subscription: %+v", stored)
			}
		})
	}
	var audits int64
	if err := db.Model(&model.AuditLog{}).Count(&audits).Error; err != nil {
		t.Fatalf("count audit logs: %v", err)
	}
	if audits != 0 {
		t.Fatalf("audit count after rejected resumes = %d, want 0", audits)
	}
}

func TestPauseMerchantSubscriptionRejectsExpiredOrNonActiveState(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	future := time.Now().Add(48 * time.Hour)
	past := time.Now().Add(-time.Hour)
	for _, test := range []struct {
		name   string
		status string
		expiry time.Time
	}{
		{name: "inactive", status: "inactive", expiry: future},
		{name: "expired active", status: "active", expiry: past},
		{name: "already paused", status: "paused", expiry: future},
	} {
		t.Run(test.name, func(t *testing.T) {
			merchant := createPauseTestMerchant(t, db, test.status, test.expiry, "")
			if _, err := NewAdminService(nil, db).PauseMerchantSubscription(merchant.ID, "test reason", MerchantSubscriptionActor{ID: 7, Type: "admin"}); err == nil {
				t.Fatal("pause unexpectedly succeeded")
			}
			var stored model.Merchant
			if err := db.First(&stored, merchant.ID).Error; err != nil {
				t.Fatalf("reload merchant: %v", err)
			}
			if stored.SubscriptionStatus != test.status || stored.SubscriptionPauseReason != "" || stored.SubscriptionPausedAt != nil || stored.SubscriptionExpireAt == nil || !stored.SubscriptionExpireAt.Equal(test.expiry) {
				t.Fatalf("rejected pause changed subscription: %+v", stored)
			}
		})
	}
}

func TestPauseMerchantSubscriptionRollsBackWhenAuditWriteFails(t *testing.T) {
	db := newOrderBoundaryTestDB(t) // Deliberately omit the audit_logs table.
	expiry := time.Now().Add(48 * time.Hour)
	merchant := createPauseTestMerchant(t, db, "active", expiry, "")
	_, err := NewAdminService(nil, db).PauseMerchantSubscription(merchant.ID, "audit must succeed", MerchantSubscriptionActor{ID: 7, Type: "admin"})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "audit_logs") {
		t.Fatalf("pause error = %v, want audit table write failure", err)
	}
	var stored model.Merchant
	if err := db.First(&stored, merchant.ID).Error; err != nil {
		t.Fatalf("reload merchant: %v", err)
	}
	if stored.SubscriptionStatus != "active" || stored.SubscriptionPauseReason != "" || stored.SubscriptionPausedAt != nil || stored.SubscriptionExpireAt == nil || !stored.SubscriptionExpireAt.Equal(expiry) {
		t.Fatalf("failed audit write left a partial pause: %+v", stored)
	}
}
