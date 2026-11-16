package service

import (
	"encoding/json"
	"errors"
	"strings"

	"gorm.io/gorm"

	"go-web-gin-health/internal/model"
)

func appendCardBatchAuditTx(tx *gorm.DB, actor AuditActor, pkg model.MembershipPackage, count, quota int) error {
	if tx == nil {
		return errors.New("卡密批次审计事务不可用")
	}
	if pkg.ID == 0 || count < 1 {
		return errors.New("卡密批次审计缺少套餐或生成数量")
	}

	detail, err := json.Marshal(struct {
		Purpose      string `json:"purpose"`
		PackageID    uint   `json:"package_id"`
		Count        int    `json:"count"`
		Quota        int    `json:"quota"`
		DurationDays int    `json:"duration_days"`
		IsLifetime   bool   `json:"is_lifetime"`
	}{
		Purpose:      "批量生成会员卡密",
		PackageID:    pkg.ID,
		Count:        count,
		Quota:        quota,
		DurationDays: pkg.DurationDays,
		IsLifetime:   pkg.IsLifetime,
	})
	if err != nil {
		return err
	}

	actorType := strings.TrimSpace(actor.Type)
	if actorType == "" {
		actorType = "unknown"
	}
	return tx.Create(&model.AuditLog{
		ActorID:    actor.ID,
		ActorType:  actorType,
		Action:     "admin_card_batch_generate",
		TargetType: "membership_package",
		TargetID:   pkg.ID,
		Detail:     string(detail),
	}).Error
}
