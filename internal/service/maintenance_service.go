package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

// MaintenanceModuleOrdering is the module name the order-creation paths
// check: a maintenance plan blocks new orders exactly when its
// affected_modules list contains "all" or "ordering".
const MaintenanceModuleOrdering = "ordering"

// --- 维护计划 ---

// CreateMaintenancePlan creates a draft maintenance plan. AffectedModules
// defaults to "all"; pass a comma-separated list (e.g. "ordering,payment")
// to narrow it. AnnouncementID links the advance notice shown to merchants.
func (s *AdminService) CreateMaintenancePlan(req dto.CreateMaintenancePlanRequest, actor AuditActor) (*model.MaintenancePlan, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("maintenance database is unavailable")
	}
	start, err := parseBeijingDateTime(req.WindowStart)
	if err != nil {
		return nil, err
	}
	end, err := parseBeijingDateTime(req.WindowEnd)
	if err != nil {
		return nil, err
	}
	if start == nil || end == nil {
		return nil, errors.New("维护开始与结束时间必填")
	}
	if !end.After(*start) {
		return nil, errors.New("维护结束时间必须晚于开始时间")
	}
	modules := normalizeMaintenanceModules(req.AffectedModules)
	if modules == "" {
		modules = "all"
	}
	entry := &model.MaintenancePlan{
		Title:           strings.TrimSpace(req.Title),
		WindowStart:     *start,
		WindowEnd:       *end,
		AffectedModules: modules,
		Status:          model.MaintenanceStatusDraft,
		AnnouncementID:  req.AnnouncementID,
		CreatedBy:       actor.ID,
	}
	if entry.Title == "" {
		return nil, errors.New("维护标题不能为空")
	}
	if entry.AnnouncementID != nil {
		var announcement model.Announcement
		if err := s.db.First(&announcement, *entry.AnnouncementID).Error; err != nil {
			return nil, errors.New("关联的公告不存在")
		}
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(entry).Error; err != nil {
			return err
		}
		return appendAuditForAnnouncements(tx, actor, "maintenance_plan_create", "maintenance_plan", entry.ID)
	}); err != nil {
		return nil, err
	}
	return entry, nil
}

// ListMaintenancePlans lists maintenance plans for the super-admin, newest first.
func (s *AdminService) ListMaintenancePlans(status string, limit, offset int) ([]model.MaintenancePlan, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("maintenance database is unavailable")
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	query := s.db.Model(&model.MaintenancePlan{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var rows []model.MaintenancePlan
	if err := query.Order("id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// GetMaintenancePlan reads one maintenance plan by id.
func (s *AdminService) GetMaintenancePlan(id uint) (*model.MaintenancePlan, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("maintenance database is unavailable")
	}
	var row model.MaintenancePlan
	if err := s.db.First(&row, id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// SetMaintenancePlanStatus moves a plan along its lifecycle:
// draft → scheduled → active → completed, with cancelled reachable from
// draft/scheduled/active. Completed and cancelled are terminal. Starting a
// plan (→ active) is refused while another plan is already active, so there
// is at most one active maintenance window at a time.
func (s *AdminService) SetMaintenancePlanStatus(id uint, req dto.SetMaintenancePlanStatusRequest, actor AuditActor) (*model.MaintenancePlan, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("maintenance database is unavailable")
	}
	target := strings.TrimSpace(req.Status)
	var entry model.MaintenancePlan
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&entry, id).Error; err != nil {
			return err
		}
		if !maintenanceStatusTransitionAllowed(entry.Status, target) {
			return fmt.Errorf("维护计划不能从 %s 改为 %s", entry.Status, target)
		}
		if target == model.MaintenanceStatusActive {
			var activeCount int64
			if err := tx.Model(&model.MaintenancePlan{}).Where("status = ? AND id <> ?", model.MaintenanceStatusActive, id).Count(&activeCount).Error; err != nil {
				return err
			}
			if activeCount > 0 {
				return errors.New("已有进行中的维护计划，请先完成或取消它")
			}
		}
		entry.Status = target
		entry.UpdatedBy = actor.ID
		if err := tx.Save(&entry).Error; err != nil {
			return err
		}
		return appendAuditForAnnouncements(tx, actor, "maintenance_plan_status_"+target, "maintenance_plan", entry.ID)
	}); err != nil {
		return nil, err
	}
	return &entry, nil
}

func maintenanceStatusTransitionAllowed(from, to string) bool {
	switch from {
	case model.MaintenanceStatusDraft:
		return to == model.MaintenanceStatusScheduled || to == model.MaintenanceStatusCancelled
	case model.MaintenanceStatusScheduled:
		return to == model.MaintenanceStatusActive || to == model.MaintenanceStatusCancelled
	case model.MaintenanceStatusActive:
		return to == model.MaintenanceStatusCompleted || to == model.MaintenanceStatusCancelled
	default:
		return false
	}
}

// ActiveMaintenancePlan returns the single active maintenance plan, or nil.
// A plan whose announced window has already ended stops counting as active
// even if nobody marked it completed, so a forgotten plan cannot block
// orders forever; the admin still completes it to close the record.
func (s *AdminService) ActiveMaintenancePlan() (*model.MaintenancePlan, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("maintenance database is unavailable")
	}
	return activeMaintenancePlan(s.db)
}

func activeMaintenancePlan(db *gorm.DB) (*model.MaintenancePlan, error) {
	var row model.MaintenancePlan
	err := db.Where("status = ?", model.MaintenanceStatusActive).Order("id ASC").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if time.Now().After(row.WindowEnd) {
		return nil, nil
	}
	return &row, nil
}

// MaintenanceBlocksModule reports whether an active maintenance plan blocks
// new work for the given module ("ordering", "payment", ...). In-flight
// orders, payments and refunds are never blocked by this check: callers only
// consult it when creating new orders, and existing flows keep running.
func MaintenanceBlocksModule(db *gorm.DB, module string) (bool, *model.MaintenancePlan, error) {
	plan, err := activeMaintenancePlan(db)
	if err != nil {
		return false, nil, err
	}
	if plan == nil {
		return false, nil, nil
	}
	return maintenancePlanBlocksModule(plan, module), plan, nil
}

func maintenancePlanBlocksModule(plan *model.MaintenancePlan, module string) bool {
	module = strings.ToLower(strings.TrimSpace(module))
	for _, affected := range strings.Split(plan.AffectedModules, ",") {
		affected = strings.ToLower(strings.TrimSpace(affected))
		if affected == "all" || (module != "" && affected == module) {
			return true
		}
	}
	return false
}

func normalizeMaintenanceModules(raw string) string {
	parts := make([]string, 0, 4)
	for _, part := range strings.Split(raw, ",") {
		part = strings.ToLower(strings.TrimSpace(part))
		if part == "" {
			continue
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, ",")
}

// MaintenanceOrderBlockError is the customer-facing message when a new order
// is refused by an active maintenance window. It names the expected recovery
// time so the customer knows when to retry.
func MaintenanceOrderBlockError(plan *model.MaintenancePlan) error {
	return fmt.Errorf("平台维护中，暂停下单，预计 %s 前恢复；已下单的订单不受影响", plan.WindowEnd.In(beijingTimeZone).Format("2006-01-02 15:04"))
}
