package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

func newSupportTicketTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open SQLite test database: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Merchant{},
		&model.AuditLog{},
		&model.SupportTicket{},
	); err != nil {
		t.Fatalf("migrate support ticket test schema: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get SQL test database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func createSupportTicketTestMerchant(t *testing.T, db *gorm.DB, name string) *model.Merchant {
	t.Helper()
	merchant := &model.Merchant{Name: name, Status: "active"}
	if err := db.Create(merchant).Error; err != nil {
		t.Fatalf("create support ticket test merchant: %v", err)
	}
	return merchant
}

func createSupportTicket(t *testing.T, db *gorm.DB, merchantID uint, title string) *model.SupportTicket {
	t.Helper()
	ticket, err := NewMerchantService(nil, db).CreateSupportTicket(merchantID, dto.CreateSupportTicketRequest{
		Title: title, Content: "工单内容", Category: "order",
	}, AuditEvent{
		Actor: AuditActor{ID: 11, Type: "merchant_admin"},
		Input: AuditRecordInput{Action: "support_ticket_create", TargetType: "support_ticket", MerchantID: &merchantID},
	})
	if err != nil {
		t.Fatalf("create support ticket %q: %v", title, err)
	}
	return ticket
}

func TestCreateSupportTicketValidatesInputAndIsolatesMerchants(t *testing.T) {
	db := newSupportTicketTestDB(t)
	merchantA := createSupportTicketTestMerchant(t, db, "工单商家甲")
	merchantB := createSupportTicketTestMerchant(t, db, "工单商家乙")
	merchantService := NewMerchantService(nil, db)
	create := func(merchantID uint, req dto.CreateSupportTicketRequest) (*model.SupportTicket, error) {
		return merchantService.CreateSupportTicket(merchantID, req, AuditEvent{
			Actor: AuditActor{ID: 11, Type: "merchant_admin"},
			Input: AuditRecordInput{Action: "support_ticket_create", TargetType: "support_ticket", MerchantID: &merchantID},
		})
	}

	// Validation: empty title/content, over-long title/content, bad category.
	if _, err := create(merchantA.ID, dto.CreateSupportTicketRequest{Title: "  ", Content: "内容", Category: "order"}); err == nil {
		t.Fatal("create unexpectedly succeeded with an empty title")
	}
	if _, err := create(merchantA.ID, dto.CreateSupportTicketRequest{Title: "标题", Content: "  ", Category: "order"}); err == nil {
		t.Fatal("create unexpectedly succeeded with empty content")
	}
	if _, err := create(merchantA.ID, dto.CreateSupportTicketRequest{Title: strings.Repeat("标", 121), Content: "内容", Category: "order"}); err == nil {
		t.Fatal("create unexpectedly succeeded with a 121-rune title")
	}
	if _, err := create(merchantA.ID, dto.CreateSupportTicketRequest{Title: "标题", Content: strings.Repeat("内", 1001), Category: "order"}); err == nil {
		t.Fatal("create unexpectedly succeeded with 1001-rune content")
	}
	if _, err := create(merchantA.ID, dto.CreateSupportTicketRequest{Title: "标题", Content: "内容", Category: "urgent"}); err == nil {
		t.Fatal("create unexpectedly succeeded with an unknown category")
	}
	if _, err := create(999999, dto.CreateSupportTicketRequest{Title: "标题", Content: "内容", Category: "order"}); err == nil {
		t.Fatal("create unexpectedly succeeded for a missing merchant")
	}

	ticket, err := create(merchantA.ID, dto.CreateSupportTicketRequest{Title: "订单无法确认收款", Content: "顾客已付款但订单仍显示待确认", Category: "order"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if ticket.Status != "open" || ticket.Reply != "" || ticket.RepliedBy != 0 || ticket.RepliedAt != nil {
		t.Fatalf("new ticket=%+v, want open with empty reply, reviewer 0, NULL replied_at", ticket)
	}
	// Audit: the create event exists in the same transaction, but the
	// audit minimization design stores only {"purpose": ...} — the
	// category/title passed in Detail are deliberately NOT stored.
	var audit model.AuditLog
	if err := db.Where("action = ? AND merchant_id = ?", "support_ticket_create", merchantA.ID).First(&audit).Error; err != nil {
		t.Fatalf("ticket create audit row missing: %v", err)
	}
	if audit.TargetType != "support_ticket" || audit.TargetID != ticket.ID {
		t.Fatalf("create audit target=%s/%d, want support_ticket/%d", audit.TargetType, audit.TargetID, ticket.ID)
	}
	if audit.Detail != `{"purpose":"商家提交服务工单"}` || strings.Contains(audit.Detail, "订单无法确认收款") || strings.Contains(audit.Detail, "order") {
		t.Fatalf("create audit detail must store only a purpose, got %s", audit.Detail)
	}

	// Isolation: merchant B lists nothing and cannot read A's ticket — the
	// missing-ticket and not-yours cases both read as "工单不存在".
	rows, total, err := merchantService.ListSupportTickets(merchantB.ID, 1, 10)
	if err != nil || total != 0 || len(rows) != 0 {
		t.Fatalf("merchant B list=%d rows total=%d err=%v, want empty", len(rows), total, err)
	}
	if _, err := merchantService.GetSupportTicket(merchantB.ID, ticket.ID); err == nil || err.Error() != "工单不存在" {
		t.Fatalf("merchant B read of A's ticket err=%v, want 工单不存在", err)
	}
	if _, err := merchantService.GetSupportTicket(merchantA.ID, 999999); err == nil || err.Error() != "工单不存在" {
		t.Fatalf("missing ticket read err=%v, want 工单不存在", err)
	}
	got, err := merchantService.GetSupportTicket(merchantA.ID, ticket.ID)
	if err != nil || got.Title != "订单无法确认收款" {
		t.Fatalf("merchant A read of own ticket=%+v err=%v", got, err)
	}
}

func TestSupportTicketStatusMachine(t *testing.T) {
	db := newSupportTicketTestDB(t)
	merchant := createSupportTicketTestMerchant(t, db, "工单状态机商家")
	admin := NewAdminService(nil, db)
	reviewer := AuditActor{ID: 7, Type: "super_admin"}
	move := func(id uint, status string) (*model.SupportTicket, error) {
		return admin.UpdateSupportTicketStatus(id, status, AuditEvent{
			Actor: reviewer,
			Input: AuditRecordInput{Action: "support_ticket_status_update", TargetType: "support_ticket"},
		})
	}

	// open → resolved direct jump is allowed.
	ticket := createSupportTicket(t, db, merchant.ID, "可直跳已解决")
	moved, err := move(ticket.ID, "resolved")
	if err != nil || moved.Status != "resolved" {
		t.Fatalf("open→resolved: %+v err=%v", moved, err)
	}
	// resolved → in_progress reopen is allowed.
	moved, err = move(ticket.ID, "in_progress")
	if err != nil || moved.Status != "in_progress" {
		t.Fatalf("resolved→in_progress: %+v err=%v", moved, err)
	}
	// in_progress → resolved, then resolved → closed.
	if moved, err = move(ticket.ID, "resolved"); err != nil || moved.Status != "resolved" {
		t.Fatalf("in_progress→resolved: %+v err=%v", moved, err)
	}
	if moved, err = move(ticket.ID, "closed"); err != nil || moved.Status != "closed" {
		t.Fatalf("resolved→closed: %+v err=%v", moved, err)
	}
	// Closed is terminal: no transition out, not even back to open.
	if _, err := move(ticket.ID, "open"); err == nil {
		t.Fatal("closed→open unexpectedly succeeded")
	}
	if _, err := move(ticket.ID, "in_progress"); err == nil {
		t.Fatal("closed→in_progress unexpectedly succeeded")
	}
	// open → closed direct jump (invalid ticket) is allowed.
	invalid := createSupportTicket(t, db, merchant.ID, "无效工单直接关")
	if moved, err := move(invalid.ID, "closed"); err != nil || moved.Status != "closed" {
		t.Fatalf("open→closed: %+v err=%v", moved, err)
	}
	// Same-status, unknown target and missing ticket are all rejected.
	open := createSupportTicket(t, db, merchant.ID, "仍待处理")
	if _, err := move(open.ID, "open"); err == nil {
		t.Fatal("same-status transition unexpectedly succeeded")
	}
	if _, err := move(open.ID, "done"); err == nil {
		t.Fatal("unknown target status unexpectedly succeeded")
	}
	if _, err := move(open.ID, "closed"); err != nil {
		t.Fatalf("open→closed on third ticket: %v", err)
	}
	if _, err := move(999999, "resolved"); err == nil {
		t.Fatal("status update unexpectedly succeeded on a missing ticket")
	}
	// The status audit records the event with purpose only.
	var audit model.AuditLog
	if err := db.Where("action = ? AND target_id = ?", "support_ticket_status_update", ticket.ID).First(&audit).Error; err != nil {
		t.Fatalf("status update audit row missing: %v", err)
	}
	if audit.Detail != `{"purpose":"更新服务工单状态"}` || audit.MerchantID == nil || *audit.MerchantID != merchant.ID {
		t.Fatalf("status audit=%+v, want purpose-only detail for merchant %d", audit, merchant.ID)
	}
}

func TestReplySupportTicketOverwritesLatestOnly(t *testing.T) {
	db := newSupportTicketTestDB(t)
	merchant := createSupportTicketTestMerchant(t, db, "工单回复商家")
	admin := NewAdminService(nil, db)
	reviewer := AuditActor{ID: 7, Type: "super_admin"}
	reply := func(id uint, text string) (*model.SupportTicket, error) {
		return admin.ReplySupportTicket(id, text, reviewer.ID, AuditEvent{
			Actor: reviewer,
			Input: AuditRecordInput{Action: "support_ticket_reply", TargetType: "support_ticket"},
		})
	}

	ticket := createSupportTicket(t, db, merchant.ID, "需要回复的工单")
	if _, err := reply(ticket.ID, "  "); err == nil {
		t.Fatal("reply unexpectedly succeeded with empty content")
	}
	if _, err := reply(ticket.ID, strings.Repeat("回", 1001)); err == nil {
		t.Fatal("reply unexpectedly succeeded with 1001-rune content")
	}
	replied, err := reply(ticket.ID, "已收到，正在核查订单收款记录")
	if err != nil {
		t.Fatalf("reply: %v", err)
	}
	if replied.Reply != "已收到，正在核查订单收款记录" || replied.RepliedBy != 7 || replied.RepliedAt == nil {
		t.Fatalf("replied ticket=%+v, want reply text, replier 7, reply time set", replied)
	}
	// A second reply overwrites the first on the ticket row.
	replied, err = reply(ticket.ID, "核查完毕，收款已确认，订单已更新")
	if err != nil {
		t.Fatalf("second reply: %v", err)
	}
	if replied.Reply != "核查完毕，收款已确认，订单已更新" {
		t.Fatalf("second reply did not overwrite: %+v", replied)
	}
	// The overwritten first reply is NOT recoverable from the audit log:
	// the audit stores only the purpose, never the reply text.
	var audits []model.AuditLog
	if err := db.Where("action = ?", "support_ticket_reply").Order("id asc").Find(&audits).Error; err != nil {
		t.Fatalf("load reply audits: %v", err)
	}
	if len(audits) != 2 {
		t.Fatalf("reply audits=%d, want 2", len(audits))
	}
	for _, audit := range audits {
		if audit.Detail != `{"purpose":"超管回复服务工单"}` ||
			strings.Contains(audit.Detail, "正在核查") || strings.Contains(audit.Detail, "核查完毕") {
			t.Fatalf("reply audit detail must store only a purpose, got %s", audit.Detail)
		}
		if audit.MerchantID == nil || *audit.MerchantID != merchant.ID || audit.TargetID != ticket.ID {
			t.Fatalf("reply audit target=%s/%d merchant=%v, want support_ticket/%d/%d", audit.TargetType, audit.TargetID, audit.MerchantID, ticket.ID, merchant.ID)
		}
	}
	// A closed ticket cannot be replied to.
	closed := createSupportTicket(t, db, merchant.ID, "已关闭的工单")
	if _, err := admin.UpdateSupportTicketStatus(closed.ID, "closed", AuditEvent{
		Actor: reviewer,
		Input: AuditRecordInput{Action: "support_ticket_status_update", TargetType: "support_ticket"},
	}); err != nil {
		t.Fatalf("close ticket: %v", err)
	}
	if _, err := reply(closed.ID, "还想再回复一句"); err == nil {
		t.Fatal("reply unexpectedly succeeded on a closed ticket")
	}
	if _, err := reply(999999, "不存在的工单"); err == nil {
		t.Fatal("reply unexpectedly succeeded on a missing ticket")
	}
}

func TestAdminListSupportTicketsFiltersAndOrder(t *testing.T) {
	db := newSupportTicketTestDB(t)
	merchantA := createSupportTicketTestMerchant(t, db, "工单列表商家甲")
	merchantB := createSupportTicketTestMerchant(t, db, "工单列表商家乙")
	admin := NewAdminService(nil, db)
	first := createSupportTicket(t, db, merchantA.ID, "最早一单")
	second := createSupportTicket(t, db, merchantB.ID, "第二单")
	third := createSupportTicket(t, db, merchantA.ID, "第三单")
	if _, err := admin.UpdateSupportTicketStatus(second.ID, "resolved", AuditEvent{
		Actor: AuditActor{ID: 7, Type: "super_admin"},
		Input: AuditRecordInput{Action: "support_ticket_status_update", TargetType: "support_ticket"},
	}); err != nil {
		t.Fatalf("resolve second ticket: %v", err)
	}

	// Oldest first: the work queue surfaces the earliest ticket on top.
	rows, total, err := admin.ListSupportTickets("", "", 0, 1, 10)
	if err != nil || total != 3 || len(rows) != 3 {
		t.Fatalf("admin list=%d rows total=%d err=%v, want 3", len(rows), total, err)
	}
	if rows[0].ID != first.ID || rows[1].ID != second.ID || rows[2].ID != third.ID {
		t.Fatalf("admin list order=%d/%d/%d, want oldest first %d/%d/%d", rows[0].ID, rows[1].ID, rows[2].ID, first.ID, second.ID, third.ID)
	}
	if rows[0].Merchant.Name != "工单列表商家甲" || rows[1].Merchant.Name != "工单列表商家乙" {
		t.Fatalf("admin list merchant names=%q/%q, want preloaded merchant names", rows[0].Merchant.Name, rows[1].Merchant.Name)
	}
	rows, total, err = admin.ListSupportTickets("resolved", "", 0, 1, 10)
	if err != nil || total != 1 || len(rows) != 1 || rows[0].ID != second.ID {
		t.Fatalf("status filter resolved=%d rows total=%d err=%v, want only the second ticket", len(rows), total, err)
	}
	rows, total, err = admin.ListSupportTickets("", "", merchantA.ID, 1, 10)
	if err != nil || total != 2 || len(rows) != 2 {
		t.Fatalf("merchant filter A=%d rows total=%d err=%v, want 2", len(rows), total, err)
	}
	rows, total, err = admin.ListSupportTickets("", "order", 0, 1, 10)
	if err != nil || total != 3 || len(rows) != 3 {
		t.Fatalf("category filter order=%d rows total=%d err=%v, want 3", len(rows), total, err)
	}
	rows, total, err = admin.ListSupportTickets("", "payment", 0, 1, 10)
	if err != nil || total != 0 || len(rows) != 0 {
		t.Fatalf("category filter payment=%d rows total=%d err=%v, want 0", len(rows), total, err)
	}
	// Pagination: page 2 of size 2 holds only the third ticket.
	rows, total, err = admin.ListSupportTickets("", "", 0, 2, 2)
	if err != nil || total != 3 || len(rows) != 1 || rows[0].ID != third.ID {
		t.Fatalf("page 2=%d rows total=%d err=%v, want only the third ticket", len(rows), total, err)
	}
	got, err := admin.GetSupportTicket(first.ID)
	if err != nil || got.Merchant.Name != "工单列表商家甲" {
		t.Fatalf("admin detail=%+v err=%v, want preloaded merchant", got, err)
	}
	if _, err := admin.GetSupportTicket(999999); err == nil {
		t.Fatal("admin detail unexpectedly succeeded on a missing ticket")
	}
}
