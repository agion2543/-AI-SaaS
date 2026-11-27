package model

import "time"

// 公告、维护计划与 P3 数据底座（P2）模型。口径红线：
//   - 公告是真正的可发布记录（草稿 → 已发布 → 已归档），不是系统配置里
//     的一行静态 notice 文本；商家只能看到已发布且在生效期内的公告。
//   - 维护计划的状态（active）就是维护模式开关：进行中的维护不中断在途
//     订单/支付/退款，只拦截新下单；窗口结束且未显式完成时不再拦截。
//   - 活动目标/复盘、建议执行、团购手工核销四张表是 P3 经营看板的数据底
//     座：目标是人工设定值，实际值由复盘快照落库，估算与手工录入一律
//     标注来源，绝不把点击说成到店、把估算说成实际。

const (
	// AnnouncementTypeSystemUpdate 系统更新公告。
	AnnouncementTypeSystemUpdate = "system_update"
	// AnnouncementTypePromo 活动促销公告。
	AnnouncementTypePromo = "promo"
	// AnnouncementTypePriceChange 价格变动公告。
	AnnouncementTypePriceChange = "price_change"

	// AnnouncementStatusDraft 草稿，未发布。
	AnnouncementStatusDraft = "draft"
	// AnnouncementStatusPublished 已发布，商家可见（须同时满足发布时间与生效期）。
	AnnouncementStatusPublished = "published"
	// AnnouncementStatusArchived 已归档，不再向商家展示。
	AnnouncementStatusArchived = "archived"

	// MaintenanceStatusDraft 草稿。
	MaintenanceStatusDraft = "draft"
	// MaintenanceStatusScheduled 已排期，尚未开始。
	MaintenanceStatusScheduled = "scheduled"
	// MaintenanceStatusActive 维护进行中：新下单按受影响模块拦截，在途流程不受影响。
	MaintenanceStatusActive = "active"
	// MaintenanceStatusCompleted 已完成，恢复正常。
	MaintenanceStatusCompleted = "completed"
	// MaintenanceStatusCancelled 已取消。
	MaintenanceStatusCancelled = "cancelled"

	// CampaignTypePromotion 活动指 promotions 表的满减/折扣活动。
	CampaignTypePromotion = "promotion"
	// CampaignTypeShareCampaign 活动指 share_campaigns 表的分享裂变活动。
	CampaignTypeShareCampaign = "share_campaign"

	// SuggestionExecutionPending 待处理：建议已生成，商家未表态。
	SuggestionExecutionPending = "pending"
	// SuggestionExecutionAdopted 已采纳：商家确认按建议执行。
	SuggestionExecutionAdopted = "adopted"
	// SuggestionExecutionIgnored 已忽略：商家明确不执行。
	SuggestionExecutionIgnored = "ignored"
	// SuggestionExecutionExecuted 已执行：商家已执行，效果待复盘。
	SuggestionExecutionExecuted = "executed"

	// SuggestionGeneratedBySystem 系统定时生成。
	SuggestionGeneratedBySystem = "system"
	// SuggestionGeneratedByAI AI 生成。
	SuggestionGeneratedByAI = "ai"
	// SuggestionGeneratedByManual 人工录入。
	SuggestionGeneratedByManual = "manual"
)

// Announcement 是一条可发布的平台公告。PublishAt 为空时以 PublishedAt 为
// 准；EffectiveFrom/EffectiveTo 为空表示不限制生效期。
type Announcement struct {
	BaseModel
	Type         string     `gorm:"size:24;not null;index:idx_announcements_type" json:"type"`
	Title        string     `gorm:"size:200;not null" json:"title"`
	Body         string     `gorm:"size:5000;not null;default:''" json:"body"`
	Status       string     `gorm:"size:16;not null;default:draft;index:idx_announcements_status_publish,priority:1" json:"status"`
	PublishAt    *time.Time `json:"publish_at"`
	EffectiveFrom *time.Time `json:"effective_from"`
	EffectiveTo  *time.Time `json:"effective_to"`
	CreatedBy    uint       `gorm:"not null" json:"created_by"`
	PublishedBy  uint       `gorm:"not null;default:0" json:"published_by"`
	PublishedAt  *time.Time `gorm:"index:idx_announcements_status_publish,priority:2" json:"published_at"`
}

func (Announcement) TableName() string { return "announcements" }

// MaintenancePlan 是一次维护窗口。AffectedModules 为逗号分隔的受影响模块，
// "all" 表示全部；"ordering" 表示下单链路。AnnouncementID 关联向商家提前
// 告知的公告（可为空）；维护进行中（status=active）且未过窗口结束时间时，
// 新下单按受影响模块拦截，在途订单/支付/退款继续完成。
type MaintenancePlan struct {
	BaseModel
	Title           string `gorm:"size:200;not null" json:"title"`
	WindowStart     time.Time `gorm:"not null;index:idx_maintenance_plans_status_window,priority:2" json:"window_start"`
	WindowEnd       time.Time `gorm:"not null" json:"window_end"`
	AffectedModules string `gorm:"size:200;not null;default:all" json:"affected_modules"`
	Status          string `gorm:"size:16;not null;default:draft;index:idx_maintenance_plans_status_window,priority:1" json:"status"`
	AnnouncementID  *uint  `gorm:"index:idx_maintenance_plans_announcement" json:"announcement_id"`
	CreatedBy       uint   `gorm:"not null" json:"created_by"`
	UpdatedBy       uint   `gorm:"not null;default:0" json:"updated_by"`
}

func (MaintenancePlan) TableName() string { return "maintenance_plans" }

// CampaignTarget 是活动目标（人工设定值）：某商家某活动在某周期内的订单
// 数、交易额（分）、新增顾客目标。实际值不在此表，由复盘快照落库。
type CampaignTarget struct {
	BaseModel
	MerchantID         uint                `gorm:"not null;uniqueIndex:uk_campaign_targets_scope,priority:1;index:idx_campaign_targets_merchant" json:"merchant_id"`
	CampaignType       string              `gorm:"size:24;not null;uniqueIndex:uk_campaign_targets_scope,priority:2" json:"campaign_type"`
	CampaignID         uint                `gorm:"not null;uniqueIndex:uk_campaign_targets_scope,priority:3" json:"campaign_id"`
	TargetOrders       int64               `gorm:"not null;default:0" json:"target_orders"`
	TargetAmountCents  int64               `gorm:"not null;default:0" json:"target_amount_cents"`
	TargetNewCustomers int64               `gorm:"not null;default:0" json:"target_new_customers"`
	PeriodStart        PlatformExpenseDate `gorm:"type:date;not null;uniqueIndex:uk_campaign_targets_scope,priority:4" json:"period_start"`
	PeriodEnd          PlatformExpenseDate `gorm:"type:date;not null" json:"period_end"`
	CreatedBy          uint                `gorm:"not null" json:"created_by"`
}

func (CampaignTarget) TableName() string { return "campaign_targets" }

// CampaignPostmortem 是活动复盘快照：目标值（快照自设定时）与实际值（生成
// 时统计）并列，偏差与结论落库，供 P3 经营看板直接读取，不现场重算。
// 实际订单数/交易额按统一交易口径统计（paid_at 不为空且金额大于 0）。
type CampaignPostmortem struct {
	BaseModel
	MerchantID         uint                `gorm:"not null;index:idx_campaign_postmortems_merchant" json:"merchant_id"`
	CampaignType       string              `gorm:"size:24;not null;index:idx_campaign_postmortems_campaign,priority:1" json:"campaign_type"`
	CampaignID         uint                `gorm:"not null;index:idx_campaign_postmortems_campaign,priority:2" json:"campaign_id"`
	PeriodStart        PlatformExpenseDate `gorm:"type:date;not null;index:idx_campaign_postmortems_campaign,priority:3" json:"period_start"`
	PeriodEnd          PlatformExpenseDate `gorm:"type:date;not null" json:"period_end"`
	TargetOrders       int64               `gorm:"not null;default:0" json:"target_orders"`
	TargetAmountCents  int64               `gorm:"not null;default:0" json:"target_amount_cents"`
	ActualOrders       int64               `gorm:"not null;default:0" json:"actual_orders"`
	ActualAmountCents  int64               `gorm:"not null;default:0" json:"actual_amount_cents"`
	ActualNewCustomers int64               `gorm:"not null;default:0" json:"actual_new_customers"`
	Conclusion         string              `gorm:"size:2000;not null;default:''" json:"conclusion"`
	GeneratedBy        string              `gorm:"size:16;not null;default:system" json:"generated_by"`
	GeneratedAt        time.Time           `gorm:"not null" json:"generated_at"`
}

func (CampaignPostmortem) TableName() string { return "campaign_postmortems" }

// AISuggestionExecution 是 AI 建议的执行跟踪：建议内容快照 + 商家操作
// （采纳/忽略/执行）+ 执行时间 + 关联订单/券 + 事后效果。没有这张表，
// P3 的“建议 → 执行 → 效果”闭环就没有数据。
type AISuggestionExecution struct {
	BaseModel
	MerchantID      uint   `gorm:"not null;index:idx_ai_suggestion_executions_merchant" json:"merchant_id"`
	StoreID         *uint  `json:"store_id"`
	Source          string `gorm:"size:80;not null;index:idx_ai_suggestion_executions_source" json:"source"`
	Title           string `gorm:"size:200;not null" json:"title"`
	ContentSnapshot string `gorm:"size:2000;not null;default:''" json:"content_snapshot"`
	Status          string `gorm:"size:16;not null;default:pending;index:idx_ai_suggestion_executions_status" json:"status"`
	ExecutedAt      *time.Time `json:"executed_at"`
	RelatedOrderID  *uint  `json:"related_order_id"`
	RelatedCouponID *uint  `json:"related_coupon_id"`
	EffectNote      string `gorm:"size:1000;not null;default:''" json:"effect_note"`
	UpdatedBy       uint   `gorm:"not null;default:0" json:"updated_by"`
}

func (AISuggestionExecution) TableName() string { return "ai_suggestion_executions" }

// GroupBuyManualRedemption 是抖音团购手工核销录入：来客 App 核销后由商家
// 或超管手工录入的核销数与核销金额。平台无抖音核销 API，录入数是人工
// 事实陈述，只能与链接点击数对照参考，绝不可说成平台验证的核销。
type GroupBuyManualRedemption struct {
	BaseModel
	MerchantID            uint                `gorm:"not null;index:idx_group_buy_manual_redemptions_merchant" json:"merchant_id"`
	StoreID               *uint               `json:"store_id"`
	LinkID                uint                `gorm:"not null;index:idx_group_buy_manual_redemptions_link,priority:1" json:"link_id"`
	RedemptionDate        PlatformExpenseDate `gorm:"type:date;not null;index:idx_group_buy_manual_redemptions_link,priority:2" json:"redemption_date"`
	RedemptionCount       int64               `gorm:"not null;default:0" json:"redemption_count"`
	RedemptionAmountCents int64               `gorm:"not null;default:0" json:"redemption_amount_cents"`
	Note                  string              `gorm:"size:500;not null;default:''" json:"note"`
	RecordedBy            uint                `gorm:"not null" json:"recorded_by"`
}

func (GroupBuyManualRedemption) TableName() string { return "group_buy_manual_redemptions" }
