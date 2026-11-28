package dto

// CreatePlatformTimeLogRequest records one block of the operator's own time.
// Hours is entered in hours with at most two decimals (0.01h..24h); the
// service converts it to integer hundredths of an hour and computes the
// amount in integer fen. The reference hourly value is entered by the super
// admin; the system never infers the user's real wage.
type CreatePlatformTimeLogRequest struct {
	LogDate                   string  `json:"log_date" binding:"required,len=10"`
	WorkType                  string  `json:"work_type" binding:"required,oneof=product_planning requirement_analysis development testing deployment_maintenance business_communication content_creation"`
	Content                   string  `json:"content" binding:"required,max=500"`
	Hours                     float64 `json:"hours" binding:"required,gt=0,lte=24"`
	ReferenceHourlyValueCents int64   `json:"reference_hourly_value_cents" binding:"required,gt=0,lte=10000000"`
	Module                    string  `json:"module" binding:"max=80"`
	IdempotencyKey            string  `json:"idempotency_key" binding:"required,len=36"`
}

// CreatePlatformIncomeRecordRequest records one platform income entry. An
// order amount is not income: only the status flow turns a record into
// recognised income. Subscription records must carry a subscription period
// (start/end dates) so MRR can amortise them; other types must not.
type CreatePlatformIncomeRecordRequest struct {
	MerchantID            *uint   `json:"merchant_id"`
	PackageID             *uint   `json:"package_id"`
	IncomeType            string  `json:"income_type" binding:"required,oneof=subscription_basic subscription_premium addon_module deployment_setup ai_usage_package other"`
	AmountCents           int64   `json:"amount_cents" binding:"required,gt=0,lte=100000000000"`
	Currency              string  `json:"currency" binding:"required,eq=CNY"`
	OccurredDate          string  `json:"occurred_date" binding:"required,len=10"`
	ReceivedDate          *string `json:"received_date"`
	Status                string  `json:"status" binding:"omitempty,oneof=ordered confirmed received pending_receipt refunded refunding"`
	RefundAmountCents     int64   `json:"refund_amount_cents" binding:"gte=0,lte=100000000000"`
	BillingPeriodMonths   int     `json:"billing_period_months" binding:"omitempty,oneof=1 3 6 12"`
	SubscriptionStartDate *string `json:"subscription_start_date"`
	SubscriptionEndDate   *string `json:"subscription_end_date"`
	OrderID               *uint   `json:"order_id"`
	Note                  string  `json:"note" binding:"max=500"`
	IdempotencyKey        string  `json:"idempotency_key" binding:"required,len=36"`
}

// UpdatePlatformIncomeRecordRequest moves one income record along its status
// flow, or cancels it. Only one of Status / Cancel may be set per call.
// Amount, type, merchant and dates never change after creation.
type UpdatePlatformIncomeRecordRequest struct {
	Status            *string `json:"status" binding:"omitempty,oneof=ordered confirmed received pending_receipt refunded refunding"`
	ReceivedDate      *string `json:"received_date"`
	RefundAmountCents *int64  `json:"refund_amount_cents" binding:"omitempty,gte=0,lte=100000000000"`
	Cancel            *bool   `json:"cancel"`
	Note              *string `json:"note" binding:"omitempty,max=500"`
}
