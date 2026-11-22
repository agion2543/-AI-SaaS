package dto

type CreatePlatformExpenseRequest struct {
	Category       string `json:"category" binding:"required,oneof=cloud_hosting messaging_api labor_contractor customer_acquisition_travel equipment other"`
	AmountCents    int64  `json:"amount_cents" binding:"required,gt=0,lte=100000000000"`
	Currency       string `json:"currency" binding:"required,eq=CNY"`
	ExpenseDate    string `json:"expense_date" binding:"required,len=10"`
	Memo           string `json:"memo" binding:"required,max=120"`
	IdempotencyKey string `json:"idempotency_key" binding:"required,len=36"`
}

type CreatePlatformExpenseReversalRequest struct {
	ReversalDate   string `json:"reversal_date" binding:"required,len=10"`
	AmountCents    int64  `json:"amount_cents" binding:"required,gt=0,lte=100000000000"`
	Reason         string `json:"reason" binding:"required,max=500"`
	IdempotencyKey string `json:"idempotency_key" binding:"required,len=36"`
}
