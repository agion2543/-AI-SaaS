package dto

type RefundReconciliationRequest struct {
	Outcome           string `json:"outcome" binding:"required,oneof=success failed"`
	ProviderReference string `json:"provider_reference" binding:"required,max=120"`
	Reason            string `json:"reason" binding:"required,max=1000"`
}

type RefundManualReviewRequest struct {
	Reason string `json:"reason" binding:"required,max=1000"`
}
