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
	// v14 categories: AI services and third-party services are recorded in the
	// same ledger as any other cost; they are not a separate cost centre.
	PlatformExpenseCategoryAIServices         = "ai_services"
	PlatformExpenseCategoryThirdPartyServices = "third_party_services"
)

// PlatformExpenseCostTypeFixed and PlatformExpenseCostTypeVariable are the
// only accepted cost types for new records. An empty CostType means the row
// predates v14 and was never classified; it is never written for new rows.
const (
	PlatformExpenseCostTypeFixed    = "fixed"
	PlatformExpenseCostTypeVariable = "variable"
)

const (
	PlatformExpensePaymentStatusPaid    = "paid"
	PlatformExpensePaymentStatusUnpaid  = "unpaid"
	PlatformExpensePaymentStatusPartial = "partial"
)

// PlatformExpense is an immutable record of an already incurred platform cost.
// AmountCents is always CNY fen; no plans or estimates belong in this table.
type PlatformExpense struct {
	ID             uint                `gorm:"primaryKey" json:"id"`
	CreatedAt      time.Time           `gorm:"not null" json:"created_at"`
	UpdatedAt      time.Time           `gorm:"not null" json:"updated_at"`
	Category       string              `gorm:"size:40;not null;index:idx_platform_expenses_category_date,priority:1;check:chk_platform_expenses_category,category IN ('cloud_hosting','messaging_api','labor_contractor','customer_acquisition_travel','equipment','other','ai_services','third_party_services')" json:"category"`
	AmountCents    int64               `gorm:"not null;check:chk_platform_expenses_amount_positive,amount_cents > 0" json:"amount_cents"`
	Currency       string              `gorm:"type:char(3);not null;default:CNY;check:chk_platform_expenses_currency,currency = 'CNY'" json:"currency"`
	ExpenseDate    PlatformExpenseDate `gorm:"type:date;not null;index:idx_platform_expenses_expense_date;index:idx_platform_expenses_category_date,priority:2" json:"expense_date"`
	Memo           string              `gorm:"size:120;not null;check:chk_platform_expenses_memo,LENGTH(TRIM(memo)) BETWEEN 1 AND 120" json:"memo"`
	CreatedBy      uint                `gorm:"not null;index:idx_platform_expenses_created_by" json:"created_by"`
	IdempotencyKey string              `gorm:"type:char(36);not null;uniqueIndex:uk_platform_expenses_idempotency_key" json:"-"`
	// v14 fields. They deliberately carry no CHECK constraint and no index:
	// platform_expenses is a v7 table, so a constraint or index on it would
	// change every pinned historical schema fingerprint. The value ranges
	// are enforced in the service layer instead. Empty CostType / Vendor on
	// a row means it predates v14 and was never classified.
	CostType       string `gorm:"size:16;not null" json:"cost_type"`
	Vendor         string `gorm:"size:120;not null" json:"vendor"`
	PaymentStatus  string `gorm:"size:16;not null" json:"payment_status"`
	IsRecurring    bool   `gorm:"not null" json:"is_recurring"`
	IsCashCost     bool   `gorm:"not null" json:"is_cash_cost"`
	IsEconomicCost bool   `gorm:"not null" json:"is_economic_cost"`
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
