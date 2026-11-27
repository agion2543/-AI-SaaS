package dto

// 公告、维护计划与 P3 数据底座（P2）请求体。时间统一用北京时间
// "2006-01-02 15:04:05"，为空表示不设置；金额一律为分（CNY）。

type CreateAnnouncementRequest struct {
	Type          string `json:"type" binding:"required,oneof=system_update promo price_change"`
	Title         string `json:"title" binding:"required,max=200"`
	Body          string `json:"body" binding:"required,max=5000"`
	PublishAt     string `json:"publish_at" binding:"omitempty,len=19"`
	EffectiveFrom string `json:"effective_from" binding:"omitempty,len=19"`
	EffectiveTo   string `json:"effective_to" binding:"omitempty,len=19"`
}

type UpdateAnnouncementRequest struct {
	Title         string `json:"title" binding:"omitempty,max=200"`
	Body          string `json:"body" binding:"omitempty,max=5000"`
	PublishAt     string `json:"publish_at" binding:"omitempty,len=19"`
	EffectiveFrom string `json:"effective_from" binding:"omitempty,len=19"`
	EffectiveTo   string `json:"effective_to" binding:"omitempty,len=19"`
}

type CreateMaintenancePlanRequest struct {
	Title           string `json:"title" binding:"required,max=200"`
	WindowStart     string `json:"window_start" binding:"required,len=19"`
	WindowEnd       string `json:"window_end" binding:"required,len=19"`
	AffectedModules string `json:"affected_modules" binding:"omitempty,max=200"`
	AnnouncementID  *uint  `json:"announcement_id"`
}

type SetMaintenancePlanStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=scheduled active completed cancelled"`
}

type CreateCampaignTargetRequest struct {
	CampaignType       string `json:"campaign_type" binding:"required,oneof=promotion share_campaign"`
	CampaignID         uint   `json:"campaign_id" binding:"required"`
	TargetOrders       int64  `json:"target_orders" binding:"gte=0,lte=1000000000"`
	TargetAmountCents  int64  `json:"target_amount_cents" binding:"gte=0,lte=1000000000000"`
	TargetNewCustomers int64  `json:"target_new_customers" binding:"gte=0,lte=1000000000"`
	PeriodStart        string `json:"period_start" binding:"required,len=10"`
	PeriodEnd          string `json:"period_end" binding:"required,len=10"`
}

type CreateCampaignPostmortemRequest struct {
	CampaignType string `json:"campaign_type" binding:"required,oneof=promotion share_campaign"`
	CampaignID   uint   `json:"campaign_id" binding:"required"`
	PeriodStart  string `json:"period_start" binding:"required,len=10"`
	PeriodEnd    string `json:"period_end" binding:"required,len=10"`
	Conclusion   string `json:"conclusion" binding:"max=2000"`
	GeneratedBy  string `json:"generated_by" binding:"omitempty,oneof=system ai manual"`
}

type RecordAISuggestionExecutionRequest struct {
	StoreID         *uint  `json:"store_id"`
	Source          string `json:"source" binding:"required,max=80"`
	Title           string `json:"title" binding:"required,max=200"`
	ContentSnapshot string `json:"content_snapshot" binding:"max=2000"`
}

type UpdateAISuggestionExecutionRequest struct {
	Status          string `json:"status" binding:"required,oneof=adopted ignored executed"`
	EffectNote      string `json:"effect_note" binding:"max=1000"`
	RelatedOrderID  *uint  `json:"related_order_id"`
	RelatedCouponID *uint  `json:"related_coupon_id"`
}

type CreateGroupBuyManualRedemptionRequest struct {
	StoreID               *uint  `json:"store_id"`
	LinkID                uint   `json:"link_id" binding:"required"`
	RedemptionDate        string `json:"redemption_date" binding:"required,len=10"`
	RedemptionCount       int64  `json:"redemption_count" binding:"required,gt=0,lte=1000000000"`
	RedemptionAmountCents int64  `json:"redemption_amount_cents" binding:"gte=0,lte=1000000000000"`
	Note                  string `json:"note" binding:"max=500"`
}
