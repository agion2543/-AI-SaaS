package dto

// AI 成本控制（P1 MVP）请求体。金额一律为分（CNY）；0 表示不限额（仅预算）。

type SaveAIModelPriceRequest struct {
	Model                      string `json:"model" binding:"required,max=120"`
	Provider                   string `json:"provider" binding:"max=80"`
	InputPriceCentsPerMillion  int64  `json:"input_price_cents_per_million" binding:"gte=0,lte=100000000"`
	OutputPriceCentsPerMillion int64  `json:"output_price_cents_per_million" binding:"gte=0,lte=100000000"`
	CachedPriceCentsPerMillion int64  `json:"cached_price_cents_per_million" binding:"gte=0,lte=100000000"`
	Currency                   string `json:"currency" binding:"required,eq=CNY"`
	EffectiveFrom              string `json:"effective_from" binding:"required,len=19"`
	IsActive                   bool   `json:"is_active"`
}

type CreateAIVendorBillRequest struct {
	Vendor         string `json:"vendor" binding:"required,max=120"`
	PeriodStart    string `json:"period_start" binding:"required,len=10"`
	PeriodEnd      string `json:"period_end" binding:"required,len=10"`
	AmountCents    int64  `json:"amount_cents" binding:"required,gt=0,lte=100000000000"`
	Currency       string `json:"currency" binding:"required,eq=CNY"`
	Note           string `json:"note" binding:"max=500"`
	IdempotencyKey string `json:"idempotency_key" binding:"required,len=36"`
}

type SaveMerchantAIBudgetRequest struct {
	DailyBudgetCents   int64  `json:"daily_budget_cents" binding:"gte=0,lte=100000000000"`
	MonthlyBudgetCents int64  `json:"monthly_budget_cents" binding:"gte=0,lte=100000000000"`
	OverLimitAction    string `json:"over_limit_action" binding:"required,oneof=stop downgrade restrict_expensive manual_review"`
}

type SetAIEmergencySwitchRequest struct {
	Disabled bool   `json:"disabled"`
	Reason   string `json:"reason" binding:"max=500"`
}
