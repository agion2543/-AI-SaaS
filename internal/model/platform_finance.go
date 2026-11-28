package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

var ErrPlatformTimeLogImmutable = errors.New("platform time logs are append-only")

// Work types for platform time logs (design v1.1 §2). The reference hourly
// value is always entered by the super admin; the system never infers the
// user's real wage from it.
const (
	PlatformTimeLogWorkProductPlanning       = "product_planning"
	PlatformTimeLogWorkRequirementAnalysis   = "requirement_analysis"
	PlatformTimeLogWorkDevelopment           = "development"
	PlatformTimeLogWorkTesting               = "testing"
	PlatformTimeLogWorkDeploymentMaintenance = "deployment_maintenance"
	PlatformTimeLogWorkBusinessCommunication = "business_communication"
	PlatformTimeLogWorkContentCreation       = "content_creation"
)

// PlatformTimeLog records one block of the operator's own time as an economic
// cost. AmountCents is always computed server-side as
// (HoursScaled * ReferenceHourlyValueCents + 50) / 100, i.e. hours are stored
// as hundredths of an hour and the amount is rounded half-up to the fen.
// HoursScaled is 1..2400 (0.01h..24h per entry). Time cost is economic cost
// only and never enters cash expenditure.
type PlatformTimeLog struct {
	ID                        uint                `gorm:"primaryKey" json:"id"`
	CreatedAt                 time.Time           `gorm:"not null" json:"created_at"`
	UpdatedAt                 time.Time           `gorm:"not null" json:"updated_at"`
	LogDate                   PlatformExpenseDate `gorm:"type:date;not null;index:idx_platform_time_logs_log_date" json:"log_date"`
	WorkType                  string              `gorm:"size:24;not null;index:idx_platform_time_logs_work_type;check:chk_platform_time_logs_work_type,work_type IN ('product_planning','requirement_analysis','development','testing','deployment_maintenance','business_communication','content_creation')" json:"work_type"`
	Content                   string              `gorm:"size:500;not null" json:"content"`
	HoursScaled               int64               `gorm:"not null;check:chk_platform_time_logs_hours_scaled,hours_scaled BETWEEN 1 AND 2400" json:"hours_scaled"`
	ReferenceHourlyValueCents int64               `gorm:"not null;check:chk_platform_time_logs_reference_value,reference_hourly_value_cents > 0" json:"reference_hourly_value_cents"`
	AmountCents               int64               `gorm:"not null;check:chk_platform_time_logs_amount,amount_cents > 0" json:"amount_cents"`
	Module                    string              `gorm:"size:80;not null" json:"module"`
	CreatedBy                 uint                `gorm:"not null;index:idx_platform_time_logs_created_by" json:"created_by"`
	IdempotencyKey            string              `gorm:"type:char(36);not null;uniqueIndex:uk_platform_time_logs_idempotency_key" json:"-"`
}

func (PlatformTimeLog) TableName() string { return "platform_time_logs" }

func (PlatformTimeLog) BeforeUpdate(*gorm.DB) error { return ErrPlatformTimeLogImmutable }

func (PlatformTimeLog) BeforeDelete(*gorm.DB) error { return ErrPlatformTimeLogImmutable }

// PlatformTimeLogAmountCents computes the economic amount for a time log in
// integer fen only: hours are hundredths of an hour, so the exact product is
// divided by 100 with half-up rounding. No float participates.
func PlatformTimeLogAmountCents(hoursScaled, referenceHourlyValueCents int64) int64 {
	return (hoursScaled*referenceHourlyValueCents + 50) / 100
}

// Income types and statuses for platform income records (design v1.1 §3).
// An order amount is not income: only the status flow below turns a record
// into recognised income, and MRR only counts confirmed/received, uncancelled
// subscription records whose subscription period covers the target month.
const (
	PlatformIncomeTypeSubscriptionBasic   = "subscription_basic"
	PlatformIncomeTypeSubscriptionPremium = "subscription_premium"
	PlatformIncomeTypeAddonModule         = "addon_module"
	PlatformIncomeTypeDeploymentSetup     = "deployment_setup"
	PlatformIncomeTypeAIUsagePackage      = "ai_usage_package"
	PlatformIncomeTypeOther               = "other"
)

const (
	PlatformIncomeStatusOrdered        = "ordered"
	PlatformIncomeStatusConfirmed      = "confirmed"
	PlatformIncomeStatusReceived       = "received"
	PlatformIncomeStatusPendingReceipt = "pending_receipt"
	PlatformIncomeStatusRefunded       = "refunded"
	PlatformIncomeStatusRefunding      = "refunding"
)

// PlatformIncomeRecord is one platform income entry. Amount, type, merchant
// and dates are immutable after creation; only the status flow (plus
// received date, refund amount and cancellation) may change, enforced by the
// service layer. A refunded record never counts as income; a refunding one
// still counts until the refund completes.
type PlatformIncomeRecord struct {
	ID                    uint                 `gorm:"primaryKey" json:"id"`
	CreatedAt             time.Time            `gorm:"not null" json:"created_at"`
	UpdatedAt             time.Time            `gorm:"not null" json:"updated_at"`
	MerchantID            *uint                `gorm:"index:idx_platform_income_records_merchant" json:"merchant_id"`
	PackageID             *uint                `json:"package_id"`
	IncomeType            string               `gorm:"size:32;not null;check:chk_platform_income_records_type,income_type IN ('subscription_basic','subscription_premium','addon_module','deployment_setup','ai_usage_package','other')" json:"income_type"`
	AmountCents           int64                `gorm:"not null;check:chk_platform_income_records_amount,amount_cents > 0" json:"amount_cents"`
	Currency              string               `gorm:"type:char(3);not null;default:CNY;check:chk_platform_income_records_currency,currency = 'CNY'" json:"currency"`
	OccurredDate          PlatformExpenseDate  `gorm:"type:date;not null;index:idx_platform_income_records_status_date,priority:2" json:"occurred_date"`
	ReceivedDate          *PlatformExpenseDate `gorm:"type:date" json:"received_date"`
	Status                string               `gorm:"size:16;not null;index:idx_platform_income_records_status_date,priority:1;default:ordered;check:chk_platform_income_records_status,status IN ('ordered','confirmed','received','pending_receipt','refunded','refunding')" json:"status"`
	RefundAmountCents     int64                `gorm:"not null;default:0;check:chk_platform_income_records_refund_amount,refund_amount_cents >= 0" json:"refund_amount_cents"`
	BillingPeriodMonths   int                  `gorm:"not null;default:1;check:chk_platform_income_records_billing_period,billing_period_months BETWEEN 1 AND 12" json:"billing_period_months"`
	SubscriptionStartDate *PlatformExpenseDate `gorm:"type:date" json:"subscription_start_date"`
	SubscriptionEndDate   *PlatformExpenseDate `gorm:"type:date" json:"subscription_end_date"`
	IsCancelled           bool                 `gorm:"not null" json:"is_cancelled"`
	OrderID               *uint                `json:"order_id"`
	Note                  string               `gorm:"size:500;not null" json:"note"`
	CreatedBy             uint                 `gorm:"not null;index:idx_platform_income_records_created_by" json:"created_by"`
	IdempotencyKey        string               `gorm:"type:char(36);not null;uniqueIndex:uk_platform_income_records_idempotency_key" json:"-"`
}

func (PlatformIncomeRecord) TableName() string { return "platform_income_records" }

// PlatformIncomeMonthlyAmountCents amortises one subscription record to a
// monthly amount in integer fen (annual payments divide by 12, floored to the
// fen; the remainder is dropped and the rounding rule is stated on the page).
// Non-subscription records and periods outside 1..12 months return 0.
func PlatformIncomeMonthlyAmountCents(record PlatformIncomeRecord) int64 {
	switch record.IncomeType {
	case PlatformIncomeTypeSubscriptionBasic, PlatformIncomeTypeSubscriptionPremium:
	default:
		return 0
	}
	if record.BillingPeriodMonths < 1 || record.BillingPeriodMonths > 12 {
		return 0
	}
	return record.AmountCents / int64(record.BillingPeriodMonths)
}
