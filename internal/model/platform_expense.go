package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

var ErrPlatformExpenseImmutable = errors.New("platform expense entries are append-only")

// PlatformExpenseDate stores a calendar date without a time zone or time-of-day.
type PlatformExpenseDate string

func (d *PlatformExpenseDate) Scan(value any) error {
	var raw string
	switch v := value.(type) {
	case time.Time:
		raw = v.Format("2006-01-02")
	case string:
		raw = v
	case []byte:
		raw = string(v)
	default:
		return fmt.Errorf("cannot scan platform expense date from %T", value)
	}
	if len(raw) >= len("2006-01-02") {
		raw = raw[:len("2006-01-02")]
	}
	if _, err := time.Parse("2006-01-02", raw); err != nil {
		return fmt.Errorf("invalid platform expense date %q: %w", raw, err)
	}
	*d = PlatformExpenseDate(raw)
	return nil
}

func (d PlatformExpenseDate) Value() (driver.Value, error) {
	if _, err := time.Parse("2006-01-02", string(d)); err != nil {
		return nil, err
	}
	return string(d), nil
}

func (d PlatformExpenseDate) MarshalJSON() ([]byte, error) { return json.Marshal(string(d)) }

const (
	PlatformExpenseCategoryCloudHosting              = "cloud_hosting"
	PlatformExpenseCategoryMessagingAPI              = "messaging_api"
	PlatformExpenseCategoryLaborContractor           = "labor_contractor"
	PlatformExpenseCategoryCustomerAcquisitionTravel = "customer_acquisition_travel"
	PlatformExpenseCategoryEquipment                 = "equipment"
	PlatformExpenseCategoryOther                     = "other"
)

// PlatformExpense is an immutable record of an already incurred platform cost.
// AmountCents is always CNY fen; no plans or estimates belong in this table.
type PlatformExpense struct {
	ID             uint                `gorm:"primaryKey" json:"id"`
	CreatedAt      time.Time           `gorm:"not null" json:"created_at"`
	UpdatedAt      time.Time           `gorm:"not null" json:"updated_at"`
	Category       string              `gorm:"size:40;not null;index:idx_platform_expenses_category_date,priority:1;check:chk_platform_expenses_category,category IN ('cloud_hosting','messaging_api','labor_contractor','customer_acquisition_travel','equipment','other')" json:"category"`
	AmountCents    int64               `gorm:"not null;check:chk_platform_expenses_amount_positive,amount_cents > 0" json:"amount_cents"`
	Currency       string              `gorm:"type:char(3);not null;default:CNY;check:chk_platform_expenses_currency,currency = 'CNY'" json:"currency"`
	ExpenseDate    PlatformExpenseDate `gorm:"type:date;not null;index:idx_platform_expenses_expense_date;index:idx_platform_expenses_category_date,priority:2" json:"expense_date"`
	Memo           string              `gorm:"size:120;not null;check:chk_platform_expenses_memo,LENGTH(TRIM(memo)) BETWEEN 1 AND 120" json:"memo"`
	CreatedBy      uint                `gorm:"not null;index:idx_platform_expenses_created_by" json:"created_by"`
	IdempotencyKey string              `gorm:"type:char(36);not null;uniqueIndex:uk_platform_expenses_idempotency_key" json:"-"`
}

func (PlatformExpense) TableName() string { return "platform_expenses" }

func (PlatformExpense) BeforeUpdate(*gorm.DB) error { return ErrPlatformExpenseImmutable }

func (PlatformExpense) BeforeDelete(*gorm.DB) error { return ErrPlatformExpenseImmutable }

// PlatformExpenseReversal is an immutable partial or full reversal linked to
// one original expense. Its reporting date is independent of the expense date.
type PlatformExpenseReversal struct {
	ID             uint                `gorm:"primaryKey" json:"id"`
	CreatedAt      time.Time           `gorm:"not null" json:"created_at"`
	UpdatedAt      time.Time           `gorm:"not null" json:"updated_at"`
	ExpenseID      uint                `gorm:"not null;index:idx_platform_expense_reversals_expense_id" json:"expense_id"`
	AmountCents    int64               `gorm:"not null;check:chk_platform_expense_reversals_amount_positive,amount_cents > 0" json:"amount_cents"`
	ReversalDate   PlatformExpenseDate `gorm:"type:date;not null;index:idx_platform_expense_reversals_date" json:"reversal_date"`
	Reason         string              `gorm:"size:500;not null;check:chk_platform_expense_reversals_reason,LENGTH(TRIM(reason)) BETWEEN 1 AND 500" json:"reason"`
	CreatedBy      uint                `gorm:"not null;index:idx_platform_expense_reversals_created_by" json:"created_by"`
	IdempotencyKey string              `gorm:"type:char(36);not null;uniqueIndex:uk_platform_expense_reversals_idempotency_key" json:"-"`
	Expense        *PlatformExpense    `gorm:"foreignKey:ExpenseID;references:ID" json:"-"`
}

func (PlatformExpenseReversal) TableName() string { return "platform_expense_reversals" }

func (PlatformExpenseReversal) BeforeUpdate(*gorm.DB) error { return ErrPlatformExpenseImmutable }

func (PlatformExpenseReversal) BeforeDelete(*gorm.DB) error { return ErrPlatformExpenseImmutable }
