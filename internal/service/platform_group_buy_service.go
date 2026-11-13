package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"go-web-gin-health/internal/model"
	"gorm.io/gorm"
)

var ErrAdminGroupBuyLinkNotFound = errors.New("团购入口不存在")
var ErrInvalidGroupBuyAction = errors.New("不支持的团购入口管理操作")

type PlatformGroupBuyService struct {
	db *gorm.DB
}

func NewPlatformGroupBuyService(db *gorm.DB) *PlatformGroupBuyService {
	return &PlatformGroupBuyService{db: db}
}

type PlatformGroupBuyFilter struct {
	Keyword      string
	Channel      string
	ReviewStatus string
	ReviewMethod string
	EntryState   string
}

type PlatformGroupBuyLinkRow struct {
	model.MerchantGroupBuyLink `gorm:"embedded"`
	MerchantName               string `json:"merchant_name"`
	MerchantStatus             string `json:"merchant_status"`
	StoreName                  string `json:"store_name"`
	StoreStatus                string `json:"store_status"`
	Domain                     string `json:"domain"`
	LastChangedAt              string `json:"last_changed_at"`
	LastChangedBy              string `json:"last_changed_by"`
	LastAction                 string `json:"last_action"`
}

type PlatformGroupBuySummary struct {
	TotalLinks        int64  `json:"total_links"`
	PendingCount      int64  `json:"pending_count"`
	ManualReviewCount int64  `json:"manual_review_count"`
	AutoApprovedCount int64  `json:"auto_approved_count"`
	AutoRejectedCount int64  `json:"auto_rejected_count"`
	SuspendedCount    int64  `json:"suspended_count"`
	ClickTotal        uint64 `json:"click_total"`
}

type PlatformGroupBuyDetail struct {
	Link    PlatformGroupBuyLinkRow           `json:"link"`
	History []model.MerchantGroupBuyLinkAudit `json:"history"`
}

func (s *PlatformGroupBuyService) List(filter PlatformGroupBuyFilter, page, pageSize int) ([]PlatformGroupBuyLinkRow, int64, PlatformGroupBuySummary, error) {
	query := s.filteredLinks(filter)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, PlatformGroupBuySummary{}, err
	}

	var summary PlatformGroupBuySummary
	if err := s.filteredLinks(filter).Select(`COUNT(*) AS total_links,
		COALESCE(SUM(CASE WHEN gb.review_status = 'pending' THEN 1 ELSE 0 END), 0) AS pending_count,
		COALESCE(SUM(CASE WHEN gb.review_status = 'pending' AND gb.review_method = 'manual' THEN 1 ELSE 0 END), 0) AS manual_review_count,
		COALESCE(SUM(CASE WHEN gb.review_status = 'approved' AND gb.review_method = 'auto' THEN 1 ELSE 0 END), 0) AS auto_approved_count,
		COALESCE(SUM(CASE WHEN gb.review_status = 'rejected' AND gb.review_method = 'auto' THEN 1 ELSE 0 END), 0) AS auto_rejected_count,
		COALESCE(SUM(CASE WHEN gb.admin_suspended = 1 THEN 1 ELSE 0 END), 0) AS suspended_count,
		COALESCE(SUM(gb.click_count), 0) AS click_total`).Scan(&summary).Error; err != nil {
		return nil, 0, PlatformGroupBuySummary{}, err
	}

	var rows []PlatformGroupBuyLinkRow
	if err := query.Select(`gb.*,
		merchants.name AS merchant_name,
		merchants.status AS merchant_status,
		stores.name AS store_name,
		stores.status AS store_status,
		(SELECT MAX(a.created_at) FROM merchant_group_buy_link_audits a WHERE a.link_id = gb.id) AS last_changed_at,
		(SELECT a.actor_name FROM merchant_group_buy_link_audits a WHERE a.link_id = gb.id ORDER BY a.id DESC LIMIT 1) AS last_changed_by,
		(SELECT a.action FROM merchant_group_buy_link_audits a WHERE a.link_id = gb.id ORDER BY a.id DESC LIMIT 1) AS last_action`).
		Order("gb.updated_at DESC, gb.id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, PlatformGroupBuySummary{}, err
	}
	for i := range rows {
		rows[i].Domain = groupBuyLinkDomain(rows[i].DestinationURL)
	}
	return rows, total, summary, nil
}

func (s *PlatformGroupBuyService) GetDetail(id uint) (*PlatformGroupBuyDetail, error) {
	var row PlatformGroupBuyLinkRow
	query := s.filteredLinks(PlatformGroupBuyFilter{})
	if err := query.Select(`gb.*,
		merchants.name AS merchant_name,
		merchants.status AS merchant_status,
		stores.name AS store_name,
		stores.status AS store_status,
		(SELECT MAX(a.created_at) FROM merchant_group_buy_link_audits a WHERE a.link_id = gb.id) AS last_changed_at,
		(SELECT a.actor_name FROM merchant_group_buy_link_audits a WHERE a.link_id = gb.id ORDER BY a.id DESC LIMIT 1) AS last_changed_by,
		(SELECT a.action FROM merchant_group_buy_link_audits a WHERE a.link_id = gb.id ORDER BY a.id DESC LIMIT 1) AS last_action`).
		Where("gb.id = ?", id).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAdminGroupBuyLinkNotFound
		}
		return nil, err
	}
	row.Domain = groupBuyLinkDomain(row.DestinationURL)
	var history []model.MerchantGroupBuyLinkAudit
	if err := s.db.Where("link_id = ?", id).Order("id DESC").Limit(100).Find(&history).Error; err != nil {
		return nil, err
	}
	if history == nil {
		history = []model.MerchantGroupBuyLinkAudit{}
	}
	return &PlatformGroupBuyDetail{Link: row, History: history}, nil
}

func (s *PlatformGroupBuyService) ApplyAdminAction(id, adminID uint, actorType, action, reason string) (*model.MerchantGroupBuyLink, error) {
	action = strings.TrimSpace(strings.ToLower(action))
	reason = strings.TrimSpace(reason)
	if len([]rune(reason)) > 255 {
		return nil, errors.New("操作原因最多 255 个字符")
	}
	if action != "approve" && action != "reject" && action != "suspend" && action != "resume" {
		return nil, ErrInvalidGroupBuyAction
	}
	if action == "reject" && reason == "" {
		return nil, errors.New("拒绝审核必须填写原因")
	}
	if action == "approve" && reason == "" {
		return nil, errors.New("人工审核通过必须填写核验理由")
	}
	if (action == "suspend" || action == "resume") && reason == "" {
		return nil, errors.New("平台暂停或恢复必须填写操作理由")
	}
	if adminID == 0 {
		return nil, errors.New("无法识别当前平台管理员")
	}
	if actorType != "admin" && actorType != "super_admin" {
		return nil, errors.New("仅平台管理员可以管理团购入口")
	}

	var saved model.MerchantGroupBuyLink
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var link model.MerchantGroupBuyLink
		if err := tx.First(&link, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAdminGroupBuyLinkNotFound
			}
			return err
		}
		before := link
		now := time.Now()
		auditRuleID := GroupBuyRuleAdminStateChange
		switch action {
		case "approve":
			if _, err := ValidateGroupBuyDestination(link.Channel, link.DestinationURL); err != nil {
				return fmt.Errorf("该链接不符合安全放行条件：%w", err)
			}
			var activeCount int64
			if err := tx.Table("stores").Joins("JOIN merchants ON merchants.id = stores.merchant_id").
				Where("stores.id = ? AND stores.merchant_id = ? AND stores.status = ? AND merchants.status = ?", link.StoreID, link.MerchantID, "active", "active").
				Count(&activeCount).Error; err != nil {
				return err
			}
			if activeCount == 0 {
				return errors.New("商家和门店均启用后才能审核通过")
			}
			auditRuleID = GroupBuyRuleAdminDecision
			link.ReviewStatus = "approved"
			link.ReviewMethod = groupBuyReviewManual
			link.ReviewRuleID = GroupBuyRuleAdminDecision
			link.ReviewReason = reason
			link.ReviewedBy = uintPtr(adminID)
			link.ReviewedAt = &now
		case "reject":
			auditRuleID = GroupBuyRuleAdminDecision
			link.ReviewStatus = "rejected"
			link.ReviewMethod = groupBuyReviewManual
			link.ReviewRuleID = GroupBuyRuleAdminDecision
			link.ReviewReason = reason
			link.ReviewedBy = uintPtr(adminID)
			link.ReviewedAt = &now
		case "suspend":
			link.AdminSuspended = true
			link.SuspensionReason = reason
			link.SuspendedBy = uintPtr(adminID)
			link.SuspendedAt = &now
		case "resume":
			if link.ReviewStatus != "approved" {
				return errors.New("只有已审核通过的入口才能恢复")
			}
			if !link.AdminSuspended {
				return errors.New("入口当前未被平台暂停")
			}
			link.AdminSuspended = false
			link.SuspensionReason = ""
			link.SuspendedBy = nil
			link.SuspendedAt = nil
		}
		if err := tx.Save(&link).Error; err != nil {
			return err
		}
		actorName := s.adminActorName(tx, adminID)
		if err := appendGroupBuyAuditWithRule(tx, &before, &link, "platform_"+action, "platform_admin", adminID, actorName, reason, auditRuleID); err != nil {
			return err
		}
		saved = link
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &saved, nil
}

func (s *PlatformGroupBuyService) filteredLinks(filter PlatformGroupBuyFilter) *gorm.DB {
	query := s.db.Table("merchant_group_buy_links AS gb").
		Joins("LEFT JOIN merchants ON merchants.id = gb.merchant_id").
		Joins("LEFT JOIN stores ON stores.id = gb.store_id AND stores.merchant_id = gb.merchant_id")
	if filter.Channel != "" {
		query = query.Where("gb.channel = ?", strings.ToLower(strings.TrimSpace(filter.Channel)))
	}
	if filter.ReviewStatus != "" {
		query = query.Where("gb.review_status = ?", strings.TrimSpace(filter.ReviewStatus))
	}
	if filter.ReviewMethod != "" {
		query = query.Where("gb.review_method = ?", strings.TrimSpace(filter.ReviewMethod))
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("merchants.name LIKE ? OR stores.name LIKE ? OR gb.destination_url LIKE ?", like, like, like)
	}
	switch strings.TrimSpace(filter.EntryState) {
	case "active":
		query = query.Where("gb.enabled = ? AND gb.admin_suspended = ? AND gb.review_status = ? AND merchants.status = ? AND stores.status = ?", true, false, "approved", "active", "active")
	case "paused":
		query = query.Where("NOT (gb.enabled = ? AND gb.admin_suspended = ? AND gb.review_status = ? AND merchants.status = ? AND stores.status = ?)", true, false, "approved", "active", "active")
	case "enabled":
		query = query.Where("gb.enabled = ?", true)
	case "disabled":
		query = query.Where("gb.enabled = ?", false)
	case "suspended":
		query = query.Where("gb.admin_suspended = ?", true)
	}
	return query
}

func appendMerchantGroupBuyAudit(tx *gorm.DB, before, after *model.MerchantGroupBuyLink, action, reason string) error {
	if after == nil {
		return errors.New("团购入口审计缺少目标记录")
	}
	var merchant model.Merchant
	if err := tx.Select("id", "name").First(&merchant, after.MerchantID).Error; err != nil {
		return err
	}
	return appendGroupBuyAudit(tx, before, after, action, "merchant_admin", after.MerchantID, merchant.Name, reason)
}

func appendGroupBuyAudit(tx *gorm.DB, before, after *model.MerchantGroupBuyLink, action, actorType string, actorID uint, actorName, reason string) error {
	return appendGroupBuyAuditWithRule(tx, before, after, action, actorType, actorID, actorName, reason, "")
}

func appendGroupBuyAuditWithRule(tx *gorm.DB, before, after *model.MerchantGroupBuyLink, action, actorType string, actorID uint, actorName, reason, ruleID string) error {
	if after == nil {
		return errors.New("团购入口审计缺少目标记录")
	}
	beforeJSON, err := groupBuyStateJSON(before)
	if err != nil {
		return err
	}
	afterJSON, err := groupBuyStateJSON(after)
	if err != nil {
		return err
	}
	event := model.MerchantGroupBuyLinkAudit{
		LinkID: after.ID, MerchantID: after.MerchantID, StoreID: after.StoreID,
		Channel: after.Channel, Action: action, RuleID: ruleID, ActorType: actorType, ActorID: actorID,
		ActorName: actorName, Reason: reason, BeforeState: beforeJSON, AfterState: afterJSON,
	}
	return tx.Create(&event).Error
}

type groupBuyState struct {
	DestinationURL   string `json:"destination_url"`
	ReviewStatus     string `json:"review_status"`
	ReviewMethod     string `json:"review_method"`
	ReviewRuleID     string `json:"review_rule_id"`
	ReviewReason     string `json:"review_reason"`
	Enabled          bool   `json:"enabled"`
	AdminSuspended   bool   `json:"admin_suspended"`
	SuspensionReason string `json:"suspension_reason"`
}

func groupBuyStateJSON(link *model.MerchantGroupBuyLink) (string, error) {
	if link == nil {
		return "{}", nil
	}
	raw, err := json.Marshal(groupBuyState{
		DestinationURL: link.DestinationURL, ReviewStatus: link.ReviewStatus,
		ReviewMethod: link.ReviewMethod, ReviewRuleID: link.ReviewRuleID,
		ReviewReason: link.ReviewReason, Enabled: link.Enabled,
		AdminSuspended: link.AdminSuspended, SuspensionReason: link.SuspensionReason,
	})
	return string(raw), err
}

func (s *PlatformGroupBuyService) adminActorName(tx *gorm.DB, adminID uint) string {
	if adminID == 0 {
		return "平台管理员"
	}
	var admin model.AdminUser
	result := tx.Select("id", "display_name", "username").Where("id = ?", adminID).Find(&admin)
	if result.Error == nil && result.RowsAffected > 0 {
		if admin.DisplayName != "" {
			return admin.DisplayName
		}
		if admin.Username != "" {
			return admin.Username
		}
	}
	return fmt.Sprintf("管理员 #%d", adminID)
}

func groupBuyLinkDomain(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}

func uintPtr(value uint) *uint { return &value }
