package service

import (
	"encoding/json"
	"errors"

	"gorm.io/gorm"

	"go-web-gin-health/internal/adminaccess"
	"go-web-gin-health/internal/model"
)

type adminAccessAuditSnapshot struct {
	Name        string   `json:"name,omitempty"`
	Code        string   `json:"code,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	Status      string   `json:"status,omitempty"`
	RoleID      uint     `json:"role_id,omitempty"`
	RoleCode    string   `json:"role_code,omitempty"`
}

func requireAdminAccessSuperAdmin(actor AuditActor) error {
	if actor.ID == 0 || actor.Type != "super_admin" {
		return errors.New("仅超级管理员可管理管理员角色和账号")
	}
	return nil
}

func roleAdminAccessSnapshot(role model.AdminRole) *adminAccessAuditSnapshot {
	return &adminAccessAuditSnapshot{
		Name:        role.Name,
		Code:        role.Code,
		Permissions: adminaccess.ParsePermissions(role.Permissions),
	}
}

func adminAdminAccessSnapshot(admin model.AdminUser, role model.AdminRole) *adminAccessAuditSnapshot {
	return &adminAccessAuditSnapshot{
		Status:   admin.Status,
		RoleID:   admin.RoleID,
		RoleCode: role.Code,
	}
}

func appendAdminAccessAuditTx(tx *gorm.DB, actor AuditActor, action, targetType string, targetID uint, before, after *adminAccessAuditSnapshot) error {
	if tx == nil {
		return errors.New("audit transaction is unavailable")
	}
	if err := verifyAdminAccessSuperAdminTx(tx, actor); err != nil {
		return err
	}
	if action == "" || targetType == "" || targetID == 0 || after == nil {
		return errors.New("admin access audit requires action, target, and after state")
	}
	detail, err := json.Marshal(struct {
		Purpose string                    `json:"purpose"`
		Before  *adminAccessAuditSnapshot `json:"before"`
		After   *adminAccessAuditSnapshot `json:"after"`
	}{
		Purpose: auditPurposeForAction(action),
		Before:  before,
		After:   after,
	})
	if err != nil {
		return err
	}
	return tx.Create(&model.AuditLog{
		ActorID:    actor.ID,
		ActorType:  actor.Type,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Detail:     string(detail),
	}).Error
}

func verifyAdminAccessSuperAdminTx(tx *gorm.DB, actor AuditActor) error {
	if err := requireAdminAccessSuperAdmin(actor); err != nil {
		return err
	}
	var current model.AdminUser
	if err := tx.Preload("Role").First(&current, actor.ID).Error; err != nil || current.Status != "active" || current.Role.Code != "super_admin" {
		return errors.New("仅启用中的超级管理员可管理管理员角色和账号")
	}
	return nil
}
