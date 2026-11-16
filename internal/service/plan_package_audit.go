package service

import (
	"encoding/json"
	"errors"
	"strings"

	"gorm.io/gorm"

	"go-web-gin-health/internal/model"
)

type merchantPlanAuditSnapshot struct {
	Name         string `json:"name"`
	PriceCents   int64  `json:"price_cents"`
	DurationDays int    `json:"duration_days"`
}

type membershipPackageAuditSnapshot struct {
	Name          string `json:"name"`
	Price         int64  `json:"price"`
	OriginalPrice int64  `json:"original_price"`
	DurationDays  int    `json:"duration_days"`
	Quota         int    `json:"quota"`
	Status        string `json:"status"`
	IsLifetime    bool   `json:"is_lifetime"`
}

func merchantPlanSnapshot(plan model.MerchantPlan) merchantPlanAuditSnapshot {
	return merchantPlanAuditSnapshot{
		Name:         plan.Name,
		PriceCents:   plan.PriceCents,
		DurationDays: plan.DurationDays,
	}
}

func membershipPackageSnapshot(pkg model.MembershipPackage) membershipPackageAuditSnapshot {
	return membershipPackageAuditSnapshot{
		Name:          pkg.Name,
		Price:         pkg.Price,
		OriginalPrice: pkg.OriginalPrice,
		DurationDays:  pkg.DurationDays,
		Quota:         pkg.Quota,
		Status:        pkg.Status,
		IsLifetime:    pkg.IsLifetime,
	}
}

func appendPlanPackageAuditTx(tx *gorm.DB, actor AuditActor, action, targetType string, targetID uint, before, after interface{}) error {
	if tx == nil {
		return errors.New("套餐配置审计事务不可用")
	}
	if targetID == 0 || targetType == "" || action == "" || after == nil {
		return errors.New("套餐配置审计缺少目标或变更后状态")
	}
	purpose := map[string]string{
		"merchant_plan_create":      "创建商家订阅计划",
		"merchant_plan_update":      "更新商家订阅计划",
		"membership_package_create": "创建会员套餐",
		"membership_package_update": "更新会员套餐",
	}[action]
	if purpose == "" {
		return errors.New("套餐配置审计动作无效")
	}
	actorType := strings.TrimSpace(actor.Type)
	if actorType == "" {
		actorType = "unknown"
	}
	detail, err := json.Marshal(struct {
		Purpose string      `json:"purpose"`
		Before  interface{} `json:"before"`
		After   interface{} `json:"after"`
	}{Purpose: purpose, Before: before, After: after})
	if err != nil {
		return err
	}
	return tx.Create(&model.AuditLog{
		ActorID:    actor.ID,
		ActorType:  actorType,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Detail:     string(detail),
	}).Error
}
