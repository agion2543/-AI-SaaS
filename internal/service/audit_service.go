package service

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"go-web-gin-health/internal/model"
)

type AuditService struct {
	db *gorm.DB
}

type AuditListFilter struct {
	Action       string
	ActorType    string
	TargetType   string
	ReviewStatus string
	Keyword      string
	MerchantID   uint
}

type AuditRecordInput struct {
	Action     string
	TargetType string
	TargetID   uint
	TargetName string
	MerchantID *uint
	Detail     map[string]interface{}
}

type AuditActor struct {
	ID        uint
	Type      string
	IP        string
	UserAgent string
}

type AuditEvent struct {
	Actor AuditActor
	Input AuditRecordInput
}

func NewAuditService(db *gorm.DB) *AuditService {
	return &AuditService{db: db}
}

func AuditActorFromContext(c *gin.Context) AuditActor {
	if c == nil {
		return AuditActor{}
	}
	role, _ := c.Get("role")
	roleText, _ := role.(string)
	return AuditActor{
		ID:        c.GetUint("user_id"),
		Type:      roleText,
		IP:        c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
	}
}

// Record persists an audit row on its own DB handle and returns storage errors.
// It cannot make a previously committed business mutation atomic with the audit.
func (s *AuditService) Record(c *gin.Context, input AuditRecordInput) error {
	if s == nil || s.db == nil {
		return errors.New("audit database is unavailable")
	}
	if c == nil {
		return errors.New("audit request context is unavailable")
	}
	return AppendAuditTx(s.db, AuditEvent{Actor: AuditActorFromContext(c), Input: input})
}

// RecordSensitiveRead intentionally stores only the actor, object, action,
// timestamp, and purpose; it does not copy phone numbers, names, notes, IPs, or
// user-agent strings into the access audit.
func (s *AuditService) RecordSensitiveRead(c *gin.Context, action, targetType string, targetID, merchantID uint, purpose string) error {
	if s == nil || s.db == nil {
		return errors.New("audit database is unavailable")
	}
	if c == nil {
		return errors.New("audit request context is unavailable")
	}
	action = strings.TrimSpace(action)
	targetType = strings.TrimSpace(targetType)
	purpose = strings.TrimSpace(purpose)
	if action == "" || targetType == "" || purpose == "" {
		return errors.New("sensitive access audit requires action, target, and purpose")
	}
	detail, err := json.Marshal(map[string]string{"purpose": purpose})
	if err != nil {
		return err
	}
	actorType, _ := c.Get("role")
	role, _ := actorType.(string)
	if strings.TrimSpace(role) == "" {
		role = "unknown"
	}
	merchantRef := merchantID
	return s.db.Create(&model.AuditLog{
		ActorID:    c.GetUint("user_id"),
		ActorType:  role,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		MerchantID: &merchantRef,
		Detail:     string(detail),
	}).Error
}

// AppendAuditTx appends one audit row using the caller's transaction. Sensitive
// mutations must call it before commit and return its error to roll back state.
func AppendAuditTx(tx *gorm.DB, event AuditEvent) error {
	if tx == nil {
		return errors.New("audit transaction is unavailable")
	}
	input := event.Input
	action := strings.TrimSpace(input.Action)
	if action == "" {
		return errors.New("audit action is required")
	}
	detail, err := json.Marshal(map[string]string{"purpose": auditPurposeForAction(action)})
	if err != nil {
		return err
	}
	actorType := strings.TrimSpace(event.Actor.Type)
	if actorType == "" {
		actorType = "unknown"
	}
	log := &model.AuditLog{
		ActorID:    event.Actor.ID,
		ActorType:  actorType,
		Action:     action,
		TargetType: strings.TrimSpace(input.TargetType),
		TargetID:   input.TargetID,
		MerchantID: input.MerchantID,
		Detail:     string(detail),
	}
	return tx.Create(log).Error
}

func auditPurposeForAction(action string) string {
	switch action {
	case "admin_role_create":
		return "创建管理员角色"
	case "admin_role_update":
		return "调整管理员角色与权限"
	case "admin_account_create":
		return "创建普通管理员账号"
	case "admin_account_update":
		return "调整普通管理员账号状态或角色"
	case "merchant_order_refund":
		return "商家退款处理"
	case "merchant_order_payment_confirm":
		return "商家确认收到顾客付款"
	case "merchant_store_disable":
		return "商家停用门店"
	case "merchant_product_disable":
		return "商家下架商品"
	case "merchant_promotion_delete":
		return "商家删除促销活动"
	case "admin_order_refund":
		return "平台退款处理"
	case "merchant_order_detail_view":
		return "履约与售后查询"
	case "merchant_customer_lead_list_view":
		return "客户跟进"
	case "admin_customer_lead_update":
		return "更新商家顾客线索状态或跟进备注"
	case "admin_merchant_store_create":
		return "平台创建商家门店"
	case "admin_merchant_store_update":
		return "平台更新商家门店信息"
	case "admin_merchant_store_disable":
		return "平台停用商家门店"
	case "merchant_follow_up_create":
		return "创建商家跟进记录"
	case "merchant_follow_up_status_update":
		return "更新商家跟进状态"
	case "merchant_subscription_payment_confirm":
		return "确认订阅收款"
	case "platform_expense_create":
		return "登记平台已发生运营费用"
	case "platform_expense_reversal_create":
		return "冲正已登记的平台运营费用"
	case "merchant_payment_config_review":
		return "收款配置审核"
	case "merchant_payment_config_save":
		return "保存商家收款配置"
	case "merchant_settlement_create":
		return "生成商家结算"
	case "merchant_settlement_paid":
		return "标记商家结算已付款"
	case "merchant_status_update":
		return "调整商家状态"
	case "merchant_onboarding_submitted":
		return "提交商家入驻审核"
	case "merchant_onboarding_approved":
		return "通过商家入驻审核"
	case "merchant_onboarding_rejected":
		return "拒绝商家入驻审核"
	case "merchant_onboarding_resubmitted":
		return "重新提交商家入驻审核"
	case "support_ticket_create":
		return "商家提交服务工单"
	case "support_ticket_reply":
		return "超管回复服务工单"
	case "support_ticket_status_update":
		return "更新服务工单状态"
	case "merchant_subscription_open":
		return "开通商家订阅"
	case "merchant_subscription_stop":
		return "停止商家订阅"
	case "user_status_update":
		return "调整用户状态"
	case "user_phone_update":
		return "修改用户手机号"
	case "user_member_adjust":
		return "调整用户会员权益"
	default:
		return action
	}
}

func appendOptionalAuditTx(tx *gorm.DB, events []AuditEvent) error {
	if len(events) == 0 {
		return nil
	}
	if len(events) != 1 {
		return errors.New("exactly one audit event is allowed")
	}
	return AppendAuditTx(tx, events[0])
}

func (s *AuditService) List(filter AuditListFilter, page, pageSize int) ([]model.AuditLog, int64, error) {
	var logs []model.AuditLog
	query := s.db.Model(&model.AuditLog{})

	if filter.Action != "" {
		query = query.Where("action = ?", filter.Action)
	}
	if filter.ActorType != "" {
		query = query.Where("actor_type = ?", filter.ActorType)
	}
	if filter.TargetType != "" {
		query = query.Where("target_type = ?", filter.TargetType)
	}
	if filter.ReviewStatus != "" {
		query = query.Where("review_status = ?", filter.ReviewStatus)
	}
	if filter.MerchantID > 0 {
		query = query.Where("merchant_id = ?", filter.MerchantID)
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("actor_name LIKE ? OR target_name LIKE ? OR action LIKE ? OR detail LIKE ?", like, like, like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("created_at desc, id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

func (s *AuditService) MarkReviewed(id uint, reviewerID uint, _ string) (*model.AuditLog, error) {
	var log model.AuditLog
	if err := s.db.First(&log, id).Error; err != nil {
		return nil, err
	}
	now := time.Now()
	if err := s.db.Model(&model.AuditLog{}).
		Where("id = ? AND review_status = ?", id, "pending").
		UpdateColumns(map[string]interface{}{
			"review_status": "reviewed",
			"reviewed_by":   reviewerID,
			"reviewed_at":   now,
		}).Error; err != nil {
		return nil, err
	}
	if err := s.db.First(&log, id).Error; err != nil {
		return nil, err
	}
	return &log, nil
}

func (s *AuditService) actorName(actorID uint, actorType string) string {
	return auditActorName(s.db, actorID, actorType)
}

func auditActorName(db *gorm.DB, actorID uint, actorType string) string {
	if actorID == 0 {
		return "system"
	}
	if actorType == "super_admin" || actorType == "admin" {
		var admin model.AdminUser
		if err := db.First(&admin, actorID).Error; err == nil {
			if admin.DisplayName != "" {
				return admin.DisplayName
			}
			return admin.Username
		}
	}
	var user model.User
	if err := db.First(&user, actorID).Error; err == nil {
		if user.DisplayName != "" {
			return user.DisplayName
		}
		if user.Phone != "" {
			return user.Phone
		}
		return user.Email
	}
	return "unknown"
}

func trimTo(value string, max int) string {
	if len([]rune(value)) <= max {
		return value
	}
	runes := []rune(value)
	return string(runes[:max])
}
