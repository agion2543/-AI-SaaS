package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// MerchantSubscriptionManualReview is the unique reconciliation record for one
// legacy subscription order. It records an administrator's evidence-check
// assertion; it is not a provider payment record and must never be updated or
// deleted.
type MerchantSubscriptionManualReview struct {
	ID                      uint      `gorm:"primaryKey" json:"id"`
	ReviewKey               string    `gorm:"size:100;not null;uniqueIndex" json:"review_key"`
	OrderID                 uint      `gorm:"not null;uniqueIndex" json:"order_id"`
	OrderNo                 string    `gorm:"size:50;not null" json:"order_no"`
	MerchantID              uint      `gorm:"not null;index" json:"merchant_id"`
	ActorID                 uint      `gorm:"not null;index" json:"actor_id"`
	ActorRole               string    `gorm:"size:30;not null" json:"actor_role"`
	PlanName                string    `gorm:"size:80;not null" json:"plan_name"`
	AmountCents             int64     `gorm:"not null" json:"amount_cents"`
	DurationDays            int64     `gorm:"not null" json:"duration_days"`
	PaymentOccurredAttested bool      `gorm:"not null" json:"payment_occurred_attested"`
	EvidenceReference       string    `gorm:"size:255;not null" json:"evidence_reference"`
	Reason                  string    `gorm:"size:1000;not null" json:"reason"`
	RequestFingerprint      string    `gorm:"size:64;not null" json:"-"`
	ReviewStatus            string    `gorm:"size:32;not null" json:"review_status"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

func (MerchantSubscriptionManualReview) TableName() string {
	return "merchant_subscription_manual_reviews"
}

func (MerchantSubscriptionManualReview) BeforeUpdate(*gorm.DB) error {
	return errors.New("merchant subscription manual review is append-only")
}

func (MerchantSubscriptionManualReview) BeforeDelete(*gorm.DB) error {
	return errors.New("merchant subscription manual review is append-only")
}

// MerchantSubscriptionManualReviewAudit is an immutable before/after audit
// event. The event explicitly preserves that no provider payment was confirmed.
type MerchantSubscriptionManualReviewAudit struct {
	ID                         uint       `gorm:"primaryKey" json:"id"`
	ReviewID                   uint       `gorm:"not null;uniqueIndex" json:"review_id"`
	OrderID                    uint       `gorm:"not null;index" json:"order_id"`
	MerchantID                 uint       `gorm:"not null;index" json:"merchant_id"`
	ActorID                    uint       `gorm:"not null;index" json:"actor_id"`
	ActorRole                  string     `gorm:"size:30;not null" json:"actor_role"`
	EvidenceReference          string     `gorm:"size:255;not null" json:"evidence_reference"`
	Reason                     string     `gorm:"size:1000;not null" json:"reason"`
	PlanName                   string     `gorm:"size:80;not null" json:"plan_name"`
	AmountCents                int64      `gorm:"not null" json:"amount_cents"`
	DurationDays               int64      `gorm:"not null" json:"duration_days"`
	PaymentOccurredAttested    bool       `gorm:"not null" json:"payment_occurred_attested"`
	OrderStatusBefore          string     `gorm:"size:20;not null" json:"order_status_before"`
	OrderStatusAfter           string     `gorm:"size:20;not null" json:"order_status_after"`
	OrderPaidAtBefore          *time.Time `json:"order_paid_at_before"`
	OrderPaidAtAfter           *time.Time `json:"order_paid_at_after"`
	OrderUpdatedAtBefore       time.Time  `gorm:"not null" json:"order_updated_at_before"`
	OrderUpdatedAtAfter        time.Time  `gorm:"not null" json:"order_updated_at_after"`
	SubscriptionStatusBefore   string     `gorm:"size:20;not null" json:"subscription_status_before"`
	SubscriptionStatusAfter    string     `gorm:"size:20;not null" json:"subscription_status_after"`
	SubscriptionPlanBefore     string     `gorm:"size:40;not null" json:"subscription_plan_before"`
	SubscriptionPlanAfter      string     `gorm:"size:40;not null" json:"subscription_plan_after"`
	SubscriptionExpireAtBefore *time.Time `json:"subscription_expire_at_before"`
	SubscriptionExpireAtAfter  time.Time  `gorm:"not null" json:"subscription_expire_at_after"`
	ReviewedAt                 time.Time  `gorm:"not null" json:"reviewed_at"`
	CreatedAt                  time.Time  `gorm:"not null" json:"created_at"`
}

func (MerchantSubscriptionManualReviewAudit) TableName() string {
	return "merchant_subscription_manual_review_audits"
}

func (MerchantSubscriptionManualReviewAudit) BeforeUpdate(*gorm.DB) error {
	return errors.New("merchant subscription manual review audit is append-only")
}

func (MerchantSubscriptionManualReviewAudit) BeforeDelete(*gorm.DB) error {
	return errors.New("merchant subscription manual review audit is append-only")
}
