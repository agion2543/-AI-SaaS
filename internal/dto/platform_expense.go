package dto

type CreatePlatformExpenseRequest struct {
	Category       string `json:"category" binding:"required,oneof=cloud_hosting messaging_api labor_contractor customer_acquisition_travel equipment other ai_services third_party_services"`
	AmountCents    int64  `json:"amount_cents" binding:"required,gt=0,lte=100000000000"`
	Currency       string `json:"currency" binding:"required,eq=CNY"`
	ExpenseDate    string `json:"expense_date" binding:"required,len=10"`
	Memo           string `json:"memo" binding:"required,max=120"`
	IdempotencyKey string `json:"idempotency_key" binding:"required,len=36"`
	// v14 fields. CostType is required for new records (fixed/variable);
	// PaymentStatus defaults to paid; the two cost flags default to true
	// (a recorded cash outlay is both a cash and an economic cost) and at
	// least one of them must stay true.
	CostType       string `json:"cost_type" binding:"required,oneof=fixed variable"`
	Vendor         string `json:"vendor" binding:"max=120"`
	PaymentStatus  string `json:"payment_status" binding:"omitempty,oneof=paid unpaid partial"`
	IsRecurring    bool   `json:"is_recurring"`
	IsCashCost     *bool  `json:"is_cash_cost"`
	IsEconomicCost *bool  `json:"is_economic_cost"`
}

type CreatePlatformExpenseReversalRequest struct {
	ReversalDate   string `json:"reversal_date" binding:"required,len=10"`
	AmountCents    int64  `json:"amount_cents" binding:"required,gt=0,lte=100000000000"`
	Reason         string `json:"reason" binding:"required,max=500"`
	IdempotencyKey string `json:"idempotency_key" binding:"required,len=36"`
}
