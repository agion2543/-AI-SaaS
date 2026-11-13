package dto

// ManualReviewLegacyMerchantSubscriptionRequest contains the original plan
// terms recorded from externally checked historical evidence. Amount is in
// integer minor units (cents); it is not derived from the current mutable plan.
type ManualReviewLegacyMerchantSubscriptionRequest struct {
	PlanName                string `json:"plan_name" binding:"required,max=80"`
	AmountCents             int64  `json:"amount_cents" binding:"required,gt=0"`
	DurationDays            int64  `json:"duration_days" binding:"required,gt=0"`
	PaymentOccurredAttested bool   `json:"payment_occurred_attested" binding:"required"`
	EvidenceReference       string `json:"evidence_reference" binding:"required,max=255"`
	Reason                  string `json:"reason" binding:"required,max=1000"`
}
