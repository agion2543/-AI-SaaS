package service

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

func newAnnouncementsTestService(t *testing.T) *AdminService {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:announcements-p2-test-"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open synthetic announcements database: %v", err)
	}
	if err := db.AutoMigrate(
		&model.AuditLog{}, &model.Announcement{}, &model.MaintenancePlan{},
		&model.CampaignTarget{}, &model.CampaignPostmortem{},
		&model.AISuggestionExecution{}, &model.GroupBuyManualRedemption{},
		&model.Merchant{}, &model.Promotion{}, &model.MerchantGroupBuyLink{},
		&model.Store{}, &model.Order{},
	); err != nil {
		t.Fatalf("migrate synthetic announcements tables: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get synthetic announcements database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return &AdminService{db: db}
}

func beijingNow() time.Time { return time.Now().In(beijingTimeZone) }

func TestAnnouncementLifecycleDraftPublishArchive(t *testing.T) {
	s := newAnnouncementsTestService(t)
	actor := AuditActor{ID: 7, Type: "admin"}

	created, err := s.CreateAnnouncement(dto.CreateAnnouncementRequest{
		Type: "system_update", Title: "系统更新", Body: "本周日晚间更新。",
	}, actor)
	if err != nil {
		t.Fatalf("create announcement: %v", err)
	}
	if created.Status != model.AnnouncementStatusDraft {
		t.Fatalf("new announcement status=%q, want draft", created.Status)
	}
	visible, err := s.ListVisibleAnnouncements(10)
	if err != nil {
		t.Fatalf("list visible announcements: %v", err)
	}
	if len(visible) != 0 {
		t.Fatalf("draft announcement must not be visible, got %d", len(visible))
	}

	updated, err := s.UpdateAnnouncement(created.ID, dto.UpdateAnnouncementRequest{Title: "系统更新（改）"}, actor)
	if err != nil {
		t.Fatalf("update draft announcement: %v", err)
	}
	if updated.Title != "系统更新（改）" {
		t.Fatalf("updated title=%q", updated.Title)
	}

	published, err := s.PublishAnnouncement(created.ID, actor)
	if err != nil {
		t.Fatalf("publish announcement: %v", err)
	}
	if published.Status != model.AnnouncementStatusPublished || published.PublishedAt == nil {
		t.Fatalf("published announcement=%+v", published)
	}
	if _, err := s.UpdateAnnouncement(created.ID, dto.UpdateAnnouncementRequest{Title: "再改"}, actor); err == nil {
		t.Fatal("published announcement must be immutable")
	}
	visible, err = s.ListVisibleAnnouncements(10)
	if err != nil {
		t.Fatalf("list visible announcements: %v", err)
	}
	if len(visible) != 1 || visible[0].ID != created.ID {
		t.Fatalf("published announcement should be visible, got %d", len(visible))
	}

	archived, err := s.ArchiveAnnouncement(created.ID, actor)
	if err != nil {
		t.Fatalf("archive announcement: %v", err)
	}
	if archived.Status != model.AnnouncementStatusArchived {
		t.Fatalf("archived status=%q", archived.Status)
	}
	visible, err = s.ListVisibleAnnouncements(10)
	if err != nil {
		t.Fatalf("list visible announcements: %v", err)
	}
	if len(visible) != 0 {
		t.Fatalf("archived announcement must not be visible, got %d", len(visible))
	}
}

func TestAnnouncementScheduledPublishAndEffectiveWindow(t *testing.T) {
	s := newAnnouncementsTestService(t)
	actor := AuditActor{ID: 7, Type: "admin"}
	now := beijingNow()

	future, err := s.CreateAnnouncement(dto.CreateAnnouncementRequest{
		Type: "promo", Title: "未来促销", Body: "下周开始。",
		PublishAt: now.Add(2 * time.Hour).Format(beijingDateTimeLayout),
	}, actor)
	if err != nil {
		t.Fatalf("create future announcement: %v", err)
	}
	if _, err := s.PublishAnnouncement(future.ID, actor); err != nil {
		t.Fatalf("publish future announcement: %v", err)
	}
	visible, err := s.ListVisibleAnnouncements(10)
	if err != nil {
		t.Fatalf("list visible announcements: %v", err)
	}
	if len(visible) != 0 {
		t.Fatalf("future publish_at announcement must not be visible yet, got %d", len(visible))
	}

	expired, err := s.CreateAnnouncement(dto.CreateAnnouncementRequest{
		Type: "price_change", Title: "过期价格", Body: "已过期。",
		EffectiveFrom: now.Add(-3 * time.Hour).Format(beijingDateTimeLayout),
		EffectiveTo:   now.Add(-1 * time.Hour).Format(beijingDateTimeLayout),
	}, actor)
	if err != nil {
		t.Fatalf("create expired announcement: %v", err)
	}
	if _, err := s.PublishAnnouncement(expired.ID, actor); err != nil {
		t.Fatalf("publish expired announcement: %v", err)
	}
	visible, err = s.ListVisibleAnnouncements(10)
	if err != nil {
		t.Fatalf("list visible announcements: %v", err)
	}
	if len(visible) != 0 {
		t.Fatalf("expired announcement must not be visible, got %d", len(visible))
	}
}

func TestMaintenancePlanLifecycleAndOrderGate(t *testing.T) {
	s := newAnnouncementsTestService(t)
	actor := AuditActor{ID: 7, Type: "admin"}
	now := beijingNow()

	plan, err := s.CreateMaintenancePlan(dto.CreateMaintenancePlanRequest{
		Title:       "夜间维护",
		WindowStart: now.Add(1 * time.Hour).Format(beijingDateTimeLayout),
		WindowEnd:   now.Add(2 * time.Hour).Format(beijingDateTimeLayout),
	}, actor)
	if err != nil {
		t.Fatalf("create maintenance plan: %v", err)
	}
	if plan.Status != model.MaintenanceStatusDraft || plan.AffectedModules != "all" {
		t.Fatalf("new plan=%+v", plan)
	}
	if _, err := s.SetMaintenancePlanStatus(plan.ID, dto.SetMaintenancePlanStatusRequest{Status: "active"}, actor); err == nil {
		t.Fatal("draft plan must not jump straight to active")
	}
	if _, err := s.SetMaintenancePlanStatus(plan.ID, dto.SetMaintenancePlanStatusRequest{Status: "scheduled"}, actor); err != nil {
		t.Fatalf("schedule plan: %v", err)
	}
	active, err := s.SetMaintenancePlanStatus(plan.ID, dto.SetMaintenancePlanStatusRequest{Status: "active"}, actor)
	if err != nil {
		t.Fatalf("activate plan: %v", err)
	}
	if active.Status != model.MaintenanceStatusActive {
		t.Fatalf("plan status=%q", active.Status)
	}

	blocked, blocking, err := MaintenanceBlocksModule(s.db, "ordering")
	if err != nil {
		t.Fatalf("check ordering block: %v", err)
	}
	if !blocked || blocking == nil || blocking.ID != plan.ID {
		t.Fatalf("active all-modules plan should block ordering, blocked=%v", blocked)
	}
	if _, err := s.SetMaintenancePlanStatus(plan.ID, dto.SetMaintenancePlanStatusRequest{Status: "scheduled"}, actor); err == nil {
		t.Fatal("active plan must not go back to scheduled")
	}

	second, err := s.CreateMaintenancePlan(dto.CreateMaintenancePlanRequest{
		Title:       "另一维护",
		WindowStart: now.Add(3 * time.Hour).Format(beijingDateTimeLayout),
		WindowEnd:   now.Add(4 * time.Hour).Format(beijingDateTimeLayout),
	}, actor)
	if err != nil {
		t.Fatalf("create second plan: %v", err)
	}
	if _, err := s.SetMaintenancePlanStatus(second.ID, dto.SetMaintenancePlanStatusRequest{Status: "scheduled"}, actor); err != nil {
		t.Fatalf("schedule second plan: %v", err)
	}
	if _, err := s.SetMaintenancePlanStatus(second.ID, dto.SetMaintenancePlanStatusRequest{Status: "active"}, actor); err == nil {
		t.Fatal("second plan must not activate while another is active")
	}

	if _, err := s.SetMaintenancePlanStatus(plan.ID, dto.SetMaintenancePlanStatusRequest{Status: "completed"}, actor); err != nil {
		t.Fatalf("complete plan: %v", err)
	}
	blocked, _, err = MaintenanceBlocksModule(s.db, "ordering")
	if err != nil {
		t.Fatalf("check ordering block after completion: %v", err)
	}
	if blocked {
		t.Fatal("completed plan must not block ordering")
	}

	narrow, err := s.CreateMaintenancePlan(dto.CreateMaintenancePlanRequest{
		Title:           "仅支付维护",
		WindowStart:     now.Add(1 * time.Hour).Format(beijingDateTimeLayout),
		WindowEnd:       now.Add(2 * time.Hour).Format(beijingDateTimeLayout),
		AffectedModules: "payment",
	}, actor)
	if err != nil {
		t.Fatalf("create narrow plan: %v", err)
	}
	if _, err := s.SetMaintenancePlanStatus(narrow.ID, dto.SetMaintenancePlanStatusRequest{Status: "scheduled"}, actor); err != nil {
		t.Fatalf("schedule narrow plan: %v", err)
	}
	if _, err := s.SetMaintenancePlanStatus(narrow.ID, dto.SetMaintenancePlanStatusRequest{Status: "active"}, actor); err != nil {
		t.Fatalf("activate narrow plan: %v", err)
	}
	blocked, _, err = MaintenanceBlocksModule(s.db, "ordering")
	if err != nil {
		t.Fatalf("check ordering block for payment-only plan: %v", err)
	}
	if blocked {
		t.Fatal("payment-only plan must not block ordering")
	}
	blocked, _, err = MaintenanceBlocksModule(s.db, "payment")
	if err != nil {
		t.Fatalf("check payment block for payment-only plan: %v", err)
	}
	if !blocked {
		t.Fatal("payment-only plan should block payment")
	}
}

func TestMaintenancePlanExpiredWindowStopsBlocking(t *testing.T) {
	s := newAnnouncementsTestService(t)
	actor := AuditActor{ID: 7, Type: "admin"}
	now := beijingNow()
	plan, err := s.CreateMaintenancePlan(dto.CreateMaintenancePlanRequest{
		Title:       "已过窗口",
		WindowStart: now.Add(-2 * time.Hour).Format(beijingDateTimeLayout),
		WindowEnd:   now.Add(-1 * time.Hour).Format(beijingDateTimeLayout),
	}, actor)
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	// Force the plan to active directly: the service refuses activating a
	// plan whose window already ended only through the gate, not the status.
	if err := s.db.Model(&model.MaintenancePlan{}).Where("id = ?", plan.ID).Update("status", model.MaintenanceStatusActive).Error; err != nil {
		t.Fatalf("force plan active: %v", err)
	}
	blocked, _, err := MaintenanceBlocksModule(s.db, "ordering")
	if err != nil {
		t.Fatalf("check ordering block: %v", err)
	}
	if blocked {
		t.Fatal("plan whose window already ended must not block ordering")
	}
	active, err := s.ActiveMaintenancePlan()
	if err != nil {
		t.Fatalf("active maintenance plan: %v", err)
	}
	if active != nil {
		t.Fatal("expired plan must not count as active")
	}
}

func seedP2Merchant(t *testing.T, s *AdminService) (uint, uint) {
	t.Helper()
	merchant := model.Merchant{Name: "P2 测试商家", Status: "active"}
	if err := s.db.Create(&merchant).Error; err != nil {
		t.Fatalf("create synthetic merchant: %v", err)
	}
	promo := model.Promotion{MerchantID: merchant.ID, Title: "P2 测试活动"}
	if err := s.db.Create(&promo).Error; err != nil {
		t.Fatalf("create synthetic promotion: %v", err)
	}
	return merchant.ID, promo.ID
}

func TestCampaignTargetAndPostmortemActuals(t *testing.T) {
	s := newAnnouncementsTestService(t)
	actor := AuditActor{ID: 7, Type: "admin"}
	merchantID, promoID := seedP2Merchant(t, s)
	now := beijingNow()
	start := now.AddDate(0, 0, -7).Format(beijingDateLayout)
	end := now.Format(beijingDateLayout)

	target, err := s.CreateCampaignTarget(merchantID, dto.CreateCampaignTargetRequest{
		CampaignType: "promotion", CampaignID: promoID,
		TargetOrders: 10, TargetAmountCents: 100000, TargetNewCustomers: 5,
		PeriodStart: start, PeriodEnd: end,
	}, actor)
	if err != nil {
		t.Fatalf("create campaign target: %v", err)
	}
	if _, err := s.CreateCampaignTarget(merchantID, dto.CreateCampaignTargetRequest{
		CampaignType: "promotion", CampaignID: promoID,
		TargetOrders: 1, TargetAmountCents: 1, TargetNewCustomers: 1,
		PeriodStart: start, PeriodEnd: end,
	}, actor); err == nil {
		t.Fatal("duplicate target scope must be refused")
	}
	updated, err := s.UpdateCampaignTarget(target.ID, dto.CreateCampaignTargetRequest{
		TargetOrders: 12, TargetAmountCents: 120000, TargetNewCustomers: 6,
	}, actor)
	if err != nil {
		t.Fatalf("update campaign target: %v", err)
	}
	if updated.TargetOrders != 12 {
		t.Fatalf("updated target orders=%d", updated.TargetOrders)
	}

	// Seed orders: two valid trades in period (one new customer, one repeat
	// customer who also paid before the period), one unpaid order, one
	// zero-amount order. Only the two valid trades count.
	paidAt := func(daysAgo int) *time.Time {
		when := now.AddDate(0, 0, -daysAgo)
		return &when
	}
	orders := []model.Order{
		{OrderNo: "P2-1", OrderType: "store_order", MerchantID: &merchantID, CustomerPhone: "13800000001", TotalAmount: 5000, PaidAt: paidAt(3)},
		{OrderNo: "P2-2", OrderType: "store_order", MerchantID: &merchantID, CustomerPhone: "13800000002", TotalAmount: 8000, PaidAt: paidAt(2)},
		{OrderNo: "P2-0", OrderType: "store_order", MerchantID: &merchantID, CustomerPhone: "13800000002", TotalAmount: 9000, PaidAt: paidAt(30)},
		{OrderNo: "P2-3", OrderType: "store_order", MerchantID: &merchantID, CustomerPhone: "13800000003", TotalAmount: 7000},
		{OrderNo: "P2-4", OrderType: "store_order", MerchantID: &merchantID, CustomerPhone: "13800000004", TotalAmount: 0, PaidAt: paidAt(1)},
	}
	for i := range orders {
		if err := s.db.Create(&orders[i]).Error; err != nil {
			t.Fatalf("seed order %s: %v", orders[i].OrderNo, err)
		}
	}
	postmortem, err := s.CreateCampaignPostmortem(merchantID, dto.CreateCampaignPostmortemRequest{
		CampaignType: "promotion", CampaignID: promoID,
		PeriodStart: start, PeriodEnd: end, Conclusion: "达标一半。",
	}, actor)
	if err != nil {
		t.Fatalf("create campaign postmortem: %v", err)
	}
	if postmortem.TargetOrders != 12 || postmortem.TargetAmountCents != 120000 {
		t.Fatalf("postmortem should snapshot the updated target, got %+v", postmortem)
	}
	if postmortem.ActualOrders != 2 || postmortem.ActualAmountCents != 13000 {
		t.Fatalf("postmortem actuals orders=%d amount=%d, want 2/13000", postmortem.ActualOrders, postmortem.ActualAmountCents)
	}
	if postmortem.ActualNewCustomers != 1 {
		t.Fatalf("postmortem new customers=%d, want 1 (only 13800000001 is new)", postmortem.ActualNewCustomers)
	}
}

func TestSuggestionExecutionTracking(t *testing.T) {
	s := newAnnouncementsTestService(t)
	actor := AuditActor{ID: 7, Type: "admin"}
	merchantID, _ := seedP2Merchant(t, s)

	recorded, err := s.RecordAISuggestionExecution(merchantID, dto.RecordAISuggestionExecutionRequest{
		Source: "daily_summary", Title: "上架新品", ContentSnapshot: "建议上架一款低价引流菜。",
	}, actor)
	if err != nil {
		t.Fatalf("record suggestion: %v", err)
	}
	if recorded.Status != model.SuggestionExecutionPending {
		t.Fatalf("new suggestion status=%q", recorded.Status)
	}
	if _, err := s.UpdateAISuggestionExecution(recorded.ID, dto.UpdateAISuggestionExecutionRequest{Status: "executed"}, actor); err == nil {
		t.Fatal("pending suggestion must not jump straight to executed")
	}
	adopted, err := s.UpdateAISuggestionExecution(recorded.ID, dto.UpdateAISuggestionExecutionRequest{Status: "adopted"}, actor)
	if err != nil {
		t.Fatalf("adopt suggestion: %v", err)
	}
	if adopted.Status != model.SuggestionExecutionAdopted {
		t.Fatalf("adopted status=%q", adopted.Status)
	}
	executed, err := s.UpdateAISuggestionExecution(recorded.ID, dto.UpdateAISuggestionExecutionRequest{
		Status: "executed", EffectNote: "上架后三天多卖 20 单。",
	}, actor)
	if err != nil {
		t.Fatalf("execute suggestion: %v", err)
	}
	if executed.Status != model.SuggestionExecutionExecuted || executed.ExecutedAt == nil || executed.EffectNote == "" {
		t.Fatalf("executed suggestion=%+v", executed)
	}
	if _, err := s.UpdateAISuggestionExecution(recorded.ID, dto.UpdateAISuggestionExecutionRequest{Status: "ignored"}, actor); err == nil {
		t.Fatal("executed suggestion is terminal")
	}
}

func TestGroupBuyManualRedemption(t *testing.T) {
	s := newAnnouncementsTestService(t)
	actor := AuditActor{ID: 7, Type: "admin"}
	merchantID, _ := seedP2Merchant(t, s)
	now := beijingNow()
	link := model.MerchantGroupBuyLink{MerchantID: merchantID, Channel: "douyin", DestinationURL: "https://example.invalid/douyin", Code: "P2-LINK-1"}
	if err := s.db.Create(&link).Error; err != nil {
		t.Fatalf("create synthetic group buy link: %v", err)
	}

	if _, err := s.CreateGroupBuyManualRedemption(merchantID, dto.CreateGroupBuyManualRedemptionRequest{
		LinkID: link.ID, RedemptionDate: now.AddDate(0, 0, 1).Format(beijingDateLayout),
		RedemptionCount: 3, RedemptionAmountCents: 15000,
	}, actor); err == nil {
		t.Fatal("future redemption date must be refused")
	}
	if _, err := s.CreateGroupBuyManualRedemption(merchantID, dto.CreateGroupBuyManualRedemptionRequest{
		LinkID: link.ID + 999, RedemptionDate: now.Format(beijingDateLayout),
		RedemptionCount: 3, RedemptionAmountCents: 15000,
	}, actor); err == nil {
		t.Fatal("redemption for another merchant's link must be refused")
	}
	first, err := s.CreateGroupBuyManualRedemption(merchantID, dto.CreateGroupBuyManualRedemptionRequest{
		LinkID: link.ID, RedemptionDate: now.Format(beijingDateLayout),
		RedemptionCount: 3, RedemptionAmountCents: 15000, Note: "来客核销",
	}, actor)
	if err != nil {
		t.Fatalf("create manual redemption: %v", err)
	}
	if first.RedemptionCount != 3 {
		t.Fatalf("redemption count=%d", first.RedemptionCount)
	}
	if _, err := s.CreateGroupBuyManualRedemption(merchantID, dto.CreateGroupBuyManualRedemptionRequest{
		LinkID: link.ID, RedemptionDate: now.AddDate(0, 0, -1).Format(beijingDateLayout),
		RedemptionCount: 2, RedemptionAmountCents: 9000,
	}, actor); err != nil {
		t.Fatalf("create second manual redemption: %v", err)
	}
	list, err := s.ListGroupBuyManualRedemptions(merchantID, link.ID, 10, 0)
	if err != nil {
		t.Fatalf("list manual redemptions: %v", err)
	}
	if len(list.Rows) != 2 || list.TotalCount != 5 || list.TotalAmountCents != 24000 {
		t.Fatalf("redemption list rows=%d total=%d amount=%d, want 2/5/24000", len(list.Rows), list.TotalCount, list.TotalAmountCents)
	}
	if list.SourceNote == "" {
		t.Fatal("redemption list must carry the manual-source note")
	}
}
