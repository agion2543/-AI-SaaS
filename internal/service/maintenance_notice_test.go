package service

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

func newMaintenanceNoticeTestService(t *testing.T) *AdminService {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:maintenance-notice-test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open synthetic maintenance notice database: %v", err)
	}
	if err := db.AutoMigrate(&model.SystemConfig{}, &model.AuditLog{}); err != nil {
		t.Fatalf("migrate synthetic maintenance notice tables: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get synthetic maintenance notice database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return &AdminService{db: db}
}

func TestMaintenanceNoticeCanBePublishedAheadOfWindowAndWithdrawn(t *testing.T) {
	service := newMaintenanceNoticeTestService(t)
	now := time.Now().In(beijingTimeZone)
	notice := dto.MerchantMaintenanceNotice{
		Published:      true,
		Title:          "计划系统更新",
		WindowStart:    now.Add(2 * time.Hour).Format(maintenanceDateTimeLayout),
		WindowEnd:      now.Add(3 * time.Hour).Format(maintenanceDateTimeLayout),
		ExpectedImpact: "登录和订单处理可能短时不可用。",
		Preparation:    "请提前完成紧急订单处理。",
	}

	saved, err := service.SaveMaintenanceNotice(notice, AuditActor{ID: 7, Type: "admin"})
	if err != nil {
		t.Fatalf("publish synthetic maintenance notice: %v", err)
	}
	if !saved.Published || saved.Title != notice.Title {
		t.Fatalf("saved notice = %#v, want published title %q", saved, notice.Title)
	}
	active, err := service.ActiveMaintenanceNotice()
	if err != nil {
		t.Fatalf("load upcoming notice for merchants: %v", err)
	}
	if active == nil || active.Title != notice.Title {
		t.Fatalf("upcoming notice should be visible before its maintenance window, got %#v", active)
	}

	var auditCount int64
	if err := service.db.Model(&model.AuditLog{}).Where("action = ?", "merchant_maintenance_notice_update").Count(&auditCount).Error; err != nil {
		t.Fatalf("count maintenance notice audit rows: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("maintenance notice save created %d audit rows, want 1", auditCount)
	}

	notice.Published = false
	if _, err := service.SaveMaintenanceNotice(notice, AuditActor{ID: 7, Type: "admin"}); err != nil {
		t.Fatalf("withdraw synthetic maintenance notice: %v", err)
	}
	active, err = service.ActiveMaintenanceNotice()
	if err != nil {
		t.Fatalf("load withdrawn notice: %v", err)
	}
	if active != nil {
		t.Fatalf("withdrawn notice should not be visible to merchants, got %#v", active)
	}
}

func TestMaintenanceNoticeRejectsIncompleteOrInvalidPublishedWindow(t *testing.T) {
	service := newMaintenanceNoticeTestService(t)
	now := time.Now().In(beijingTimeZone)
	start := now.Add(2 * time.Hour).Format(maintenanceDateTimeLayout)
	end := now.Add(3 * time.Hour).Format(maintenanceDateTimeLayout)
	base := dto.MerchantMaintenanceNotice{
		Published:      true,
		Title:          "计划维护",
		WindowStart:    start,
		WindowEnd:      end,
		ExpectedImpact: "可能短时不可用。",
		Preparation:    "请提前处理紧急订单。",
	}

	cases := []struct {
		name   string
		change func(*dto.MerchantMaintenanceNotice)
	}{
		{name: "missing impact", change: func(n *dto.MerchantMaintenanceNotice) { n.ExpectedImpact = " " }},
		{name: "missing preparation", change: func(n *dto.MerchantMaintenanceNotice) { n.Preparation = " " }},
		{name: "end before start", change: func(n *dto.MerchantMaintenanceNotice) { n.WindowEnd = start }},
		{name: "malformed start", change: func(n *dto.MerchantMaintenanceNotice) { n.WindowStart = "tomorrow" }},
		{name: "expired window", change: func(n *dto.MerchantMaintenanceNotice) {
			n.WindowStart = now.Add(-3 * time.Hour).Format(maintenanceDateTimeLayout)
			n.WindowEnd = now.Add(-2 * time.Hour).Format(maintenanceDateTimeLayout)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			notice := base
			tc.change(&notice)
			if _, err := service.SaveMaintenanceNotice(notice, AuditActor{ID: 7, Type: "admin"}); err == nil {
				t.Fatal("invalid published notice was accepted")
			}
		})
	}
}
