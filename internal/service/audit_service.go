package service

import (
	"encoding/json"
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

func NewAuditService(db *gorm.DB) *AuditService {
	return &AuditService{db: db}
}

func (s *AuditService) Record(c *gin.Context, input AuditRecordInput) {
	if s == nil || s.db == nil || c == nil || strings.TrimSpace(input.Action) == "" {
		return
	}

	detail := "{}"
	if len(input.Detail) > 0 {
		if raw, err := json.Marshal(input.Detail); err == nil {
			detail = string(raw)
		}
	}

	actorID := c.GetUint("user_id")
	actorType, _ := c.Get("role")
	actorTypeText, _ := actorType.(string)
	if actorTypeText == "" {
		actorTypeText = "unknown"
	}

	log := &model.AuditLog{
		ActorID:    actorID,
		ActorType:  actorTypeText,
		ActorName:  s.actorName(actorID, actorTypeText),
		Action:     strings.TrimSpace(input.Action),
		TargetType: strings.TrimSpace(input.TargetType),
		TargetID:   input.TargetID,
		TargetName: strings.TrimSpace(input.TargetName),
		MerchantID: input.MerchantID,
		IP:         c.ClientIP(),
		UserAgent:  trimTo(c.GetHeader("User-Agent"), 255),
		Detail:     detail,
	}
	_ = s.db.Create(log).Error
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
	if actorID == 0 {
		return "system"
	}
	if actorType == "super_admin" || actorType == "admin" {
		var admin model.AdminUser
		if err := s.db.First(&admin, actorID).Error; err == nil {
			if admin.DisplayName != "" {
				return admin.DisplayName
			}
			return admin.Username
		}
	}
	var user model.User
	if err := s.db.First(&user, actorID).Error; err == nil {
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
