package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// RefundReconciliationAudit is append-only: application code may create records,
// but there is intentionally no update or delete workflow for this model.
type RefundReconciliationAudit struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	RefundID          uint      `gorm:"not null;index;uniqueIndex:idx_refund_reconciliation_transition,priority:1" json:"refund_id"`
	RefundNo          string    `gorm:"size:60;not null" json:"refund_no"`
	OrderID           uint      `gorm:"not null;index" json:"order_id"`
	ActorID           uint      `gorm:"not null;index" json:"actor_id"`
	ActorRole         string    `gorm:"size:30;not null" json:"actor_role"`
	OldStatus         string    `gorm:"size:20;not null;uniqueIndex:idx_refund_reconciliation_transition,priority:2" json:"old_status"`
	NewStatus         string    `gorm:"size:20;not null;uniqueIndex:idx_refund_reconciliation_transition,priority:3" json:"new_status"`
	Amount            int64     `gorm:"not null" json:"amount"`
	ProviderReference string    `gorm:"size:120;not null" json:"provider_reference"`
	Reason            string    `gorm:"size:1000;not null" json:"reason"`
	CreatedAt         time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (RefundReconciliationAudit) BeforeUpdate(*gorm.DB) error {
	return errors.New("refund reconciliation audit is append-only")
}

func (RefundReconciliationAudit) BeforeDelete(*gorm.DB) error {
	return errors.New("refund reconciliation audit is append-only")
}
