package dto

import "time"

// ManualSubscriptionReviewOrder is a deliberately narrow, read-only projection
// for the platform subscription audit view. It contains no merchant credentials
// or payment-provider configuration.
type ManualSubscriptionReviewOrder struct {
	OrderID           uint                                `json:"order_id"`
	OrderNo           string                              `json:"order_no"`
	MerchantID        *uint                               `json:"merchant_id"`
	Merchant          *ManualReviewMerchantSummary        `json:"merchant,omitempty"`
	Status            string                              `json:"status"`
	SubscriptionEndAt *time.Time                          `json:"subscription_end_at"`
	CreatedAt         time.Time                           `json:"created_at"`
	ManualReview      ManualSubscriptionReviewRecord      `json:"manual_review"`
	ManualReviewAudit ManualSubscriptionReviewAuditRecord `json:"manual_review_audit"`
}

type ManualReviewMerchantSummary struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type ManualSubscriptionReviewRecord struct {
	ID                      uint      `json:"id"`
	OrderID                 uint      `json:"order_id"`
	OrderNo                 string    `json:"order_no"`
	MerchantID              uint      `json:"merchant_id"`
	ActorID                 uint      `json:"actor_id"`
	ActorRole               string    `json:"actor_role"`
	PlanName                string    `json:"plan_name"`
	AmountCents             int64     `json:"amount_cents"`
	DurationDays            int64     `json:"duration_days"`
	PaymentOccurredAttested bool      `json:"payment_occurred_attested"`
	EvidenceReference       string    `json:"evidence_reference"`
	Reason                  string    `json:"reason"`
	ReviewStatus            string    `json:"review_status"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

type ManualSubscriptionReviewAuditRecord struct {
	ID                         uint       `json:"id"`
	ReviewID                   uint       `json:"review_id"`
	OrderID                    uint       `json:"order_id"`
	MerchantID                 uint       `json:"merchant_id"`
	ActorID                    uint       `json:"actor_id"`
	ActorRole                  string     `json:"actor_role"`
	EvidenceReference          string     `json:"evidence_reference"`
	Reason                     string     `json:"reason"`
	PlanName                   string     `json:"plan_name"`
	AmountCents                int64      `json:"amount_cents"`
	DurationDays               int64      `json:"duration_days"`
	PaymentOccurredAttested    bool       `json:"payment_occurred_attested"`
	OrderStatusBefore          string     `json:"order_status_before"`
	OrderStatusAfter           string     `json:"order_status_after"`
	OrderPaidAtBefore          *time.Time `json:"order_paid_at_before"`
	OrderPaidAtAfter           *time.Time `json:"order_paid_at_after"`
	OrderUpdatedAtBefore       time.Time  `json:"order_updated_at_before"`
	OrderUpdatedAtAfter        time.Time  `json:"order_updated_at_after"`
	SubscriptionStatusBefore   string     `json:"subscription_status_before"`
	SubscriptionStatusAfter    string     `json:"subscription_status_after"`
	SubscriptionPlanBefore     string     `json:"subscription_plan_before"`
	SubscriptionPlanAfter      string     `json:"subscription_plan_after"`
	SubscriptionExpireAtBefore *time.Time `json:"subscription_expire_at_before"`
	SubscriptionExpireAtAfter  time.Time  `json:"subscription_expire_at_after"`
	ReviewedAt                 time.Time  `json:"reviewed_at"`
	CreatedAt                  time.Time  `json:"created_at"`
}
