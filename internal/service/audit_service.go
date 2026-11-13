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
	detail := "{}"
	if len(input.Detail) > 0 {
		raw, err := json.Marshal(input.Detail)
		if err != nil {
			return err
		}
		detail = string(raw)
	}
	actorType := strings.TrimSpace(event.Actor.Type)
	if actorType == "" {
		actorType = "unknown"
	}
	log := &model.AuditLog{
		ActorID:    event.Actor.ID,
		ActorType:  actorType,
		ActorName:  auditActorName(tx, event.Actor.ID, actorType),
		Action:     action,
		TargetType: strings.TrimSpace(input.TargetType),
		TargetID:   input.TargetID,
		TargetName: strings.TrimSpace(input.TargetName),
		MerchantID: input.MerchantID,
		IP:         trimTo(event.Actor.IP, 80),
		UserAgent:  trimTo(event.Actor.UserAgent, 255),
		Detail:     detail,
	}
	return tx.Create(log).Error
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
	if err := query.Order("created_at desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

func (s *AuditService) MarkReviewed(id uint, reviewerID uint, remark string) (*model.AuditLog, error) {
	var log model.AuditLog
	if err := s.db.First(&log, id).Error; err != nil {
		return nil, err
	}
	now := time.Now()
	log.ReviewStatus = "reviewed"
	log.ReviewRemark = trimTo(strings.TrimSpace(remark), 255)
	log.ReviewedBy = &reviewerID
	log.ReviewedAt = &now
	if err := s.db.Save(&log).Error; err != nil {
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
