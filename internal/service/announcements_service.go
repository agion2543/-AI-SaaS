package service

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

const beijingDateTimeLayout = "2006-01-02 15:04:05"
const beijingDateLayout = "2006-01-02"

// parseBeijingDateTime parses an optional Beijing datetime ("2006-01-02
// 15:04:05"); "" parses to nil without error.
func parseBeijingDateTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.ParseInLocation(beijingDateTimeLayout, value, beijingTimeZone)
	if err != nil {
		return nil, errors.New("时间格式无效，请使用 YYYY-MM-DD HH:mm:ss（北京时间）")
	}
	return &parsed, nil
}

// parseBeijingDate parses a required Beijing date ("2006-01-02").
func parseBeijingDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	parsed, err := time.ParseInLocation(beijingDateLayout, value, beijingTimeZone)
	if err != nil {
		return time.Time{}, errors.New("日期格式无效，请使用 YYYY-MM-DD")
	}
	return parsed, nil
}

func appendAuditForAnnouncements(tx *gorm.DB, actor AuditActor, action, targetType string, targetID uint) error {
	return AppendAuditTx(tx, AuditEvent{
		Actor: actor,
		Input: AuditRecordInput{Action: action, TargetType: targetType, TargetID: targetID},
	})
}

// --- 公告 ---

// CreateAnnouncement creates a draft announcement. Only the super-admin
// publishes; merchants only ever see published rows (see
// ListVisibleAnnouncements).
func (s *AdminService) CreateAnnouncement(req dto.CreateAnnouncementRequest, actor AuditActor) (*model.Announcement, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("announcement database is unavailable")
	}
	publishAt, err := parseBeijingDateTime(req.PublishAt)
	if err != nil {
		return nil, err
	}
	effectiveFrom, err := parseBeijingDateTime(req.EffectiveFrom)
	if err != nil {
		return nil, err
	}
	effectiveTo, err := parseBeijingDateTime(req.EffectiveTo)
	if err != nil {
		return nil, err
	}
	if effectiveFrom != nil && effectiveTo != nil && !effectiveTo.After(*effectiveFrom) {
		return nil, errors.New("生效结束时间必须晚于生效开始时间")
	}
	entry := &model.Announcement{
		Type:          strings.TrimSpace(req.Type),
		Title:         strings.TrimSpace(req.Title),
		Body:          strings.TrimSpace(req.Body),
		Status:        model.AnnouncementStatusDraft,
		PublishAt:     publishAt,
		EffectiveFrom: effectiveFrom,
		EffectiveTo:   effectiveTo,
		CreatedBy:     actor.ID,
	}
	if entry.Title == "" || entry.Body == "" {
		return nil, errors.New("公告标题与正文不能为空")
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(entry).Error; err != nil {
			return err
		}
		return appendAuditForAnnouncements(tx, actor, "announcement_create", "announcement", entry.ID)
	}); err != nil {
		return nil, err
	}
	return entry, nil
}

// ListAnnouncements lists announcements for the super-admin, newest first.
// status "" lists all.
func (s *AdminService) ListAnnouncements(status string, limit, offset int) ([]model.Announcement, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("announcement database is unavailable")
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	query := s.db.Model(&model.Announcement{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var rows []model.Announcement
	if err := query.Order("id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// GetAnnouncement reads one announcement by id.
func (s *AdminService) GetAnnouncement(id uint) (*model.Announcement, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("announcement database is unavailable")
	}
	var row model.Announcement
	if err := s.db.First(&row, id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// UpdateAnnouncement edits a draft announcement. Published or archived rows
// are immutable: correct them by archiving and publishing a new one, so the
// published history merchants already saw is never silently rewritten.
func (s *AdminService) UpdateAnnouncement(id uint, req dto.UpdateAnnouncementRequest, actor AuditActor) (*model.Announcement, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("announcement database is unavailable")
	}
	var entry model.Announcement
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&entry, id).Error; err != nil {
			return err
		}
		if entry.Status != model.AnnouncementStatusDraft {
			return errors.New("只有草稿公告可以修改，已发布或已归档的公告请归档后重新发布")
		}
		if title := strings.TrimSpace(req.Title); title != "" {
			entry.Title = title
		}
		if body := strings.TrimSpace(req.Body); body != "" {
			entry.Body = body
		}
		publishAt, err := parseBeijingDateTime(req.PublishAt)
		if err != nil {
			return err
		}
		effectiveFrom, err := parseBeijingDateTime(req.EffectiveFrom)
		if err != nil {
			return err
		}
		effectiveTo, err := parseBeijingDateTime(req.EffectiveTo)
		if err != nil {
			return err
		}
		if publishAt != nil {
			entry.PublishAt = publishAt
		}
		if effectiveFrom != nil {
			entry.EffectiveFrom = effectiveFrom
		}
		if effectiveTo != nil {
			entry.EffectiveTo = effectiveTo
		}
		if entry.EffectiveFrom != nil && entry.EffectiveTo != nil && !entry.EffectiveTo.After(*entry.EffectiveFrom) {
			return errors.New("生效结束时间必须晚于生效开始时间")
		}
		if err := tx.Save(&entry).Error; err != nil {
			return err
		}
		return appendAuditForAnnouncements(tx, actor, "announcement_update", "announcement", entry.ID)
	}); err != nil {
		return nil, err
	}
	return &entry, nil
}

// PublishAnnouncement publishes a draft. published_at is the actual publish
// time; publish_at (when set in the future) is the scheduled visibility
// time, so publishing early never leaks a future announcement.
func (s *AdminService) PublishAnnouncement(id uint, actor AuditActor) (*model.Announcement, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("announcement database is unavailable")
	}
	var entry model.Announcement
	now := time.Now()
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&entry, id).Error; err != nil {
			return err
		}
		if entry.Status != model.AnnouncementStatusDraft {
			return errors.New("只有草稿公告可以发布")
		}
		entry.Status = model.AnnouncementStatusPublished
		entry.PublishedBy = actor.ID
		entry.PublishedAt = &now
		if err := tx.Save(&entry).Error; err != nil {
			return err
		}
		return appendAuditForAnnouncements(tx, actor, "announcement_publish", "announcement", entry.ID)
	}); err != nil {
		return nil, err
	}
	return &entry, nil
}

// ArchiveAnnouncement archives a published announcement. Archived rows are
// no longer shown to merchants.
func (s *AdminService) ArchiveAnnouncement(id uint, actor AuditActor) (*model.Announcement, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("announcement database is unavailable")
	}
	var entry model.Announcement
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&entry, id).Error; err != nil {
			return err
		}
		if entry.Status != model.AnnouncementStatusPublished {
			return errors.New("只有已发布公告可以归档")
		}
		entry.Status = model.AnnouncementStatusArchived
		if err := tx.Save(&entry).Error; err != nil {
			return err
		}
		return appendAuditForAnnouncements(tx, actor, "announcement_archive", "announcement", entry.ID)
	}); err != nil {
		return nil, err
	}
	return &entry, nil
}

// ListVisibleAnnouncements is the merchant-facing list: published rows whose
// scheduled publish time (publish_at, falling back to the actual
// published_at) has passed and whose effective window covers now. Drafts and
// archived rows never appear here.
func (s *AdminService) ListVisibleAnnouncements(limit int) ([]model.Announcement, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("announcement database is unavailable")
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	now := time.Now()
	var rows []model.Announcement
	err := s.db.Model(&model.Announcement{}).
		Where("status = ?", model.AnnouncementStatusPublished).
		Where("COALESCE(publish_at, published_at) <= ?", now).
		Where("effective_from IS NULL OR effective_from <= ?", now).
		Where("effective_to IS NULL OR effective_to >= ?", now).
		Order("COALESCE(publish_at, published_at) DESC, id DESC").
		Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}
