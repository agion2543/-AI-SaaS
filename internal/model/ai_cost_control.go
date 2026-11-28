package model

import "time"

// AI 成本控制（P1 MVP）模型。口径红线：
//   - 估算成本（estimated）来自调用记录上存的 EstimatedCostCents 累计
//     （写入时按当时生效价或内置费率估算，写入即冻结），永远不是供应商真实账单；
//   - 供应商实际账单（AIVendorBill）是超管手工录入的实际金额，两者在接口与
//     返回字段上严格分开，差异仅供核对参考。
//   - 金额一律为分（CNY）；价格表单价为“分 / 百万 Token”。
//   - merchant_id = 0 的 MerchantAIBudget 行表示全局默认预算与默认超额策略。

const (
	// AIBudgetOverLimitStop 达到预算后停止 AI 调用。
	AIBudgetOverLimitStop = "stop"
	// AIBudgetOverLimitDowngrade 达到预算后降级到价格表中最便宜的在用模型。
	AIBudgetOverLimitDowngrade = "downgrade"
	// AIBudgetOverLimitRestrictExpensive 达到预算后只放行低成本功能，
	// 高成本功能（多文案批量生成、分享活动复盘）拒绝。
	AIBudgetOverLimitRestrictExpensive = "restrict_expensive"
	// AIBudgetOverLimitManualReview 达到预算后进入人工审核：调用直接拒绝，
	// 需超管调整预算或策略后方可恢复。
	AIBudgetOverLimitManualReview = "manual_review"
)

// AIModelPrice 是超管维护的模型价格表。一条记录只描述一个模型在某个
// 生效时间点的单价；调价时新增一行（新的 effective_from），不改写历史行。
// 价格表用于写入时定价与未来测算；历史调用的估算成本以调用记录上存的值为准，
// 事后改价不改写已记成本。
type AIModelPrice struct {
	BaseModel
	Model                      string    `gorm:"size:120;not null;index:idx_ai_model_prices_model_effective,priority:1;uniqueIndex:uk_ai_model_prices_model_provider_effective,priority:1" json:"model"`
	Provider                   string    `gorm:"size:80;not null;default:'';uniqueIndex:uk_ai_model_prices_model_provider_effective,priority:2" json:"provider"`
	InputPriceCentsPerMillion  int64     `gorm:"not null" json:"input_price_cents_per_million"`
	OutputPriceCentsPerMillion int64     `gorm:"not null" json:"output_price_cents_per_million"`
	CachedPriceCentsPerMillion int64     `gorm:"not null;default:0" json:"cached_price_cents_per_million"`
	Currency                   string    `gorm:"type:char(3);not null;default:CNY" json:"currency"`
	EffectiveFrom              time.Time `gorm:"not null;index:idx_ai_model_prices_model_effective,priority:2;uniqueIndex:uk_ai_model_prices_model_provider_effective,priority:3" json:"effective_from"`
	CreatedBy                  uint      `gorm:"not null" json:"created_by"`
	UpdatedBy                  uint      `gorm:"not null;default:0" json:"updated_by"`
	IsActive                   bool      `gorm:"not null;default:true" json:"is_active"`
}

func (AIModelPrice) TableName() string { return "ai_model_prices" }

// AIVendorBill 是超管手工录入的供应商实际账单（真实金额），用于与估算成本核对。
// 它不是估算值，也不是系统自动生成的账单；录入即事实陈述，不可改写，只能作废重录
// （本 MVP 不提供修改接口，录错请联系超管用冲正流程在备注中说明并重录）。
type AIVendorBill struct {
	BaseModel
	Vendor         string              `gorm:"size:120;not null;index:idx_ai_vendor_bills_vendor_period,priority:1" json:"vendor"`
	PeriodStart    PlatformExpenseDate `gorm:"type:date;not null;index:idx_ai_vendor_bills_vendor_period,priority:2;index:idx_ai_vendor_bills_period_start" json:"period_start"`
	PeriodEnd      PlatformExpenseDate `gorm:"type:date;not null" json:"period_end"`
	AmountCents    int64               `gorm:"not null" json:"amount_cents"`
	Currency       string              `gorm:"type:char(3);not null;default:CNY" json:"currency"`
	Note           string              `gorm:"size:500;not null;default:''" json:"note"`
	CreatedBy      uint                `gorm:"not null" json:"created_by"`
	IdempotencyKey string              `gorm:"type:char(36);not null;uniqueIndex:uk_ai_vendor_bills_idempotency_key" json:"-"`
}

func (AIVendorBill) TableName() string { return "ai_vendor_bills" }

// MerchantAIBudget 是商家的 AI 预算。DailyBudgetCents / MonthlyBudgetCents
// 为 0 表示该周期不限额。MerchantID 为 0 的行是全局默认预算与默认策略。
type MerchantAIBudget struct {
	BaseModel
	MerchantID         uint   `gorm:"not null;uniqueIndex:uk_merchant_ai_budgets_merchant" json:"merchant_id"`
	DailyBudgetCents   int64  `gorm:"not null;default:0" json:"daily_budget_cents"`
	MonthlyBudgetCents int64  `gorm:"not null;default:0" json:"monthly_budget_cents"`
	OverLimitAction    string `gorm:"size:24;not null;default:stop" json:"over_limit_action"`
	UpdatedBy          uint   `gorm:"not null" json:"updated_by"`
}

func (MerchantAIBudget) TableName() string { return "merchant_ai_budgets" }

// MerchantAIBudgetAlert 是预算预警记录：某商家在某周期（日/月）的估算成本
// 使用进度首次达到阈值（70/90/100）时产生一条，可查询，不重复产生。
// UsedTokens 与 EstimatedCostCents 是产生预警当时的用量与估算成本（CNY 分）。
type MerchantAIBudgetAlert struct {
	BaseModel
	MerchantID         uint                `gorm:"not null;index:idx_merchant_ai_budget_alerts_merchant;uniqueIndex:uk_merchant_ai_budget_alerts_scope,priority:1" json:"merchant_id"`
	PeriodType         string              `gorm:"size:16;not null;uniqueIndex:uk_merchant_ai_budget_alerts_scope,priority:2" json:"period_type"`
	PeriodStart        PlatformExpenseDate `gorm:"type:date;not null;index:idx_merchant_ai_budget_alerts_period;uniqueIndex:uk_merchant_ai_budget_alerts_scope,priority:3" json:"period_start"`
	Threshold          int                 `gorm:"not null;uniqueIndex:uk_merchant_ai_budget_alerts_scope,priority:4" json:"threshold"`
	UsedTokens         int64               `gorm:"not null;default:0" json:"used_tokens"`
	EstimatedCostCents int64               `gorm:"not null;default:0" json:"estimated_cost_cents"`
}

func (MerchantAIBudgetAlert) TableName() string { return "merchant_ai_budget_alerts" }

// AIEmergencySwitch 是超管紧急关闭 AI 的全局开关，全库只有 id=1 一行。
// Disabled=true 时所有商家 AI 调用在入口直接拒绝；恢复为 false 后恢复调用。
// 开关变更必须记审计（见 service.SetAIEmergencySwitch）。
type AIEmergencySwitch struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null" json:"updated_at"`
	Disabled  bool      `gorm:"not null;default:false" json:"disabled"`
	Reason    string    `gorm:"size:500;not null;default:''" json:"reason"`
	UpdatedBy uint      `gorm:"not null;default:0" json:"updated_by"`
}

func (AIEmergencySwitch) TableName() string { return "ai_emergency_switch" }
