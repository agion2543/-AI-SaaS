package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

// Support ticket categories (P3-2, v18). The whitelist is enforced in the
// service layer; there is no priority — tickets are handled first-come
// first-served by submission time.
var supportTicketCategories = map[string]bool{
	"order":        true,
	"payment":      true,
	"subscription": true,
	"store":        true,
	"account":      true,
	"other":        true,
}

const (
	// SupportTicketTitleMaxRunes / SupportTicketContentMaxRunes mirror the
	// 1000-rune follow-up content limit for the ticket body.
	SupportTicketTitleMaxRunes   = 120
	SupportTicketContentMaxRunes = 1000
	SupportTicketReplyMaxRunes   = 1000
)

// supportTicketAllowedTransitions is the super-admin-only status machine:
// open → in_progress → resolved → closed, with open allowed to jump
// straight to resolved (fixed without a separate "in progress" step) or
// closed (invalid ticket), and resolved allowed to reopen to in_progress.
// Closed is terminal. A merchant may not change any status; a merchant
// who feels an issue is unresolved opens a new ticket instead.
var supportTicketAllowedTransitions = map[string]map[string]bool{
	"open":        {"in_progress": true, "resolved": true, "closed": true},
	"in_progress": {"resolved": true, "closed": true},
	"resolved":    {"in_progress": true, "closed": true},
	"closed":      {},
}

func validSupportTicketCategory(category string) bool {
	return supportTicketCategories[strings.TrimSpace(category)]
}

func validateSupportTicketInput(title, content, category string) (string, string, string, error) {
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	category = strings.TrimSpace(category)
	if title == "" {
		return "", "", "", errors.New("工单标题不能为空")
	}
	if len([]rune(title)) > SupportTicketTitleMaxRunes {
		return "", "", "", fmt.Errorf("工单标题不能超过 %d 字", SupportTicketTitleMaxRunes)
	}
	if content == "" {
		return "", "", "", errors.New("工单内容不能为空")
	}
	if len([]rune(content)) > SupportTicketContentMaxRunes {
		return "", "", "", fmt.Errorf("工单内容不能超过 %d 字", SupportTicketContentMaxRunes)
	}
	if !validSupportTicketCategory(category) {
		return "", "", "", errors.New("工单分类无效")
	}
	return title, content, category, nil
}

// CreateSupportTicket files a merchant service request (P3-2, v18). The
// merchant identity comes from the caller's token, never from the request
// body. The creation is recorded in the same transaction as the audit
// event (support_ticket_create); the audit stores only actor, object,
// action, time and purpose — the ticket content itself lives on the
// ticket row, not in the audit detail.
func (s *MerchantService) CreateSupportTicket(merchantID uint, req dto.CreateSupportTicketRequest, audit AuditEvent) (*model.SupportTicket, error) {
	title, content, category, err := validateSupportTicketInput(req.Title, req.Content, req.Category)
	if err != nil {
		return nil, err
	}
	if _, err := s.merchants.FindByID(merchantID); err != nil {
		return nil, errors.New("商家信息不存在")
	}
	ticket := &model.SupportTicket{
		MerchantID: merchantID,
		Title:      title,
		Content:    content,
		Category:   category,
		Status:     "open",
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(ticket).Error; err != nil {
			return err
		}
		event := audit
		event.Input.TargetType = "support_ticket"
		event.Input.TargetID = ticket.ID
		event.Input.MerchantID = &merchantID
		if event.Input.Detail == nil {
			event.Input.Detail = map[string]interface{}{}
		}
		event.Input.Detail["category"] = category
		event.Input.Detail["title"] = title
		return appendOptionalAuditTx(tx, []AuditEvent{event})
	})
	if err != nil {
		return nil, err
	}
	return ticket, nil
}

// ListSupportTickets returns one merchant's own tickets, newest first. The
// merchant scope is forced here: every query carries merchant_id from the
// token, so a merchant can never list another merchant's tickets.
func (s *MerchantService) ListSupportTickets(merchantID uint, page, pageSize int) ([]model.SupportTicket, int64, error) {
	var rows []model.SupportTicket
	query := s.db.Model(&model.SupportTicket{}).Where("merchant_id = ?", merchantID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("created_at desc, id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// GetSupportTicket loads one of the merchant's own tickets. The lookup is
// by id + merchant_id, so a ticket that does not exist and a ticket that
// belongs to another merchant both read as "工单不存在" (no enumeration).
func (s *MerchantService) GetSupportTicket(merchantID, id uint) (*model.SupportTicket, error) {
	var ticket model.SupportTicket
	if err := s.db.Where("id = ? AND merchant_id = ?", id, merchantID).First(&ticket).Error; err != nil {
		return nil, errors.New("工单不存在")
	}
	return &ticket, nil
}

// ListSupportTickets returns the super-admin work queue, oldest first, so
// the earliest-submitted unhandled ticket is always on top. Optional
// filters: status, category and merchant_id; an empty status/category
// means all statuses/categories.
func (s *AdminService) ListSupportTickets(status, category string, merchantID uint, page, pageSize int) ([]model.SupportTicket, int64, error) {
	var rows []model.SupportTicket
	query := s.db.Preload("Merchant").Model(&model.SupportTicket{})
	if value := strings.TrimSpace(status); value != "" {
		query = query.Where("status = ?", value)
	}
	if value := strings.TrimSpace(category); value != "" {
		query = query.Where("category = ?", value)
	}
	if merchantID > 0 {
		query = query.Where("merchant_id = ?", merchantID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("created_at asc, id asc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// GetSupportTicket loads one ticket for the super-admin, with its merchant.
func (s *AdminService) GetSupportTicket(id uint) (*model.SupportTicket, error) {
	var ticket model.SupportTicket
	if err := s.db.Preload("Merchant").First(&ticket, id).Error; err != nil {
		return nil, errors.New("工单不存在")
	}
	return &ticket, nil
}

// UpdateSupportTicketStatus moves a ticket along the super-admin-only
// status machine. The transition and the audit event
// (support_ticket_status_update) commit in the same transaction.
func (s *AdminService) UpdateSupportTicketStatus(id uint, status string, audit AuditEvent) (*model.SupportTicket, error) {
	status = strings.TrimSpace(status)
	var ticket model.SupportTicket
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&ticket, id).Error; err != nil {
			return errors.New("工单不存在")
		}
		allowed, ok := supportTicketAllowedTransitions[ticket.Status]
		if !ok {
			return fmt.Errorf("工单当前状态 %q 无效", ticket.Status)
		}
		if status == ticket.Status {
			return fmt.Errorf("工单已是 %q 状态", ticket.Status)
		}
		if !allowed[status] {
			return fmt.Errorf("工单不能从 %q 流转到 %q", ticket.Status, status)
		}
		before := ticket.Status
		ticket.Status = status
		if err := tx.Save(&ticket).Error; err != nil {
			return err
		}
		event := audit
		event.Input.TargetType = "support_ticket"
		event.Input.TargetID = ticket.ID
		event.Input.MerchantID = &ticket.MerchantID
		if event.Input.Detail == nil {
			event.Input.Detail = map[string]interface{}{}
		}
		event.Input.Detail["status_before"] = before
		event.Input.Detail["status"] = status
		return appendOptionalAuditTx(tx, []AuditEvent{event})
	})
	if err != nil {
		return nil, err
	}
	return &ticket, nil
}

// ReplySupportTicket stores the latest super-admin reply on the ticket
// row (reply / replied_by / replied_at). Only one reply is kept: a new
// reply overwrites the previous one, and the overwritten text is not
// visible on the ticket afterwards — and it is not recoverable from the
// audit log either, because the audit minimization design stores only
// {"purpose": ...} in the detail (see audit_minimization_test.go). The
// reply event (support_ticket_reply) still records who replied and when.
// A closed ticket cannot be replied to; the merchant opens a new ticket
// if the issue continues.
func (s *AdminService) ReplySupportTicket(id uint, reply string, replierID uint, audit AuditEvent) (*model.SupportTicket, error) {
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return nil, errors.New("回复内容不能为空")
	}
	if len([]rune(reply)) > SupportTicketReplyMaxRunes {
		return nil, fmt.Errorf("回复内容不能超过 %d 字", SupportTicketReplyMaxRunes)
	}
	var ticket model.SupportTicket
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&ticket, id).Error; err != nil {
			return errors.New("工单不存在")
		}
		if ticket.Status == "closed" {
			return errors.New("工单已关闭，无法回复；如需继续沟通请开新单")
		}
		now := time.Now()
		ticket.Reply = reply
		ticket.RepliedBy = replierID
		ticket.RepliedAt = &now
		if err := tx.Save(&ticket).Error; err != nil {
			return err
		}
		event := audit
		event.Input.TargetType = "support_ticket"
		event.Input.TargetID = ticket.ID
		event.Input.MerchantID = &ticket.MerchantID
		return appendOptionalAuditTx(tx, []AuditEvent{event})
	})
	if err != nil {
		return nil, err
	}
	return &ticket, nil
}
