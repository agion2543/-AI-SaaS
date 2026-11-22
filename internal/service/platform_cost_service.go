package service

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

const (
	maxPlatformExpenseCents     int64 = 100_000_000_000 // CNY 1,000,000,000 per entry; leaves ample int64 headroom.
	maxPlatformExpenseMemoRunes       = 120
	platformExpenseListLimit          = 500
)

var (
	ErrInvalidPlatformExpense                 = errors.New("invalid platform expense")
	ErrPlatformExpenseIdempotencyConflict     = errors.New("platform expense idempotency key conflicts with an existing entry")
	ErrPlatformExpenseNotFound                = errors.New("original platform expense was not found")
	ErrPlatformExpenseReversalExceedsOriginal = errors.New("cumulative platform expense reversals exceed the original expense")
	ErrPlatformExpenseReversalKeyConflict     = errors.New("platform expense reversal idempotency key conflicts with an existing entry")
)

type PlatformRevenueSummary struct {
	PaidOrderCount     int64 `json:"paid_order_count"`
	MerchantCount      int64 `json:"merchant_count"`
	GrossReceivedCents int64 `json:"gross_received_cents"`
	RefundedCents      int64 `json:"refunded_cents"`
	NetReceivedCents   int64 `json:"net_received_cents"`
}

// PlatformManualVerifiedSubscriptionSummary reports an administrator's
// evidence-backed attestation separately from provider-confirmed receipts.
// It is not independently verified bank settlement and is never added to net receipts.
type PlatformManualVerifiedSubscriptionSummary struct {
	ReviewCount         int64 `json:"review_count"`
	MerchantCount       int64 `json:"merchant_count"`
	AttestedAmountCents int64 `json:"attested_amount_cents"`
}

type PlatformAIUsageAvailability struct {
	Status            string                    `json:"status"`
	ProviderConnected bool                      `json:"provider_connected"`
	DataAvailable     bool                      `json:"data_available"`
	RequestCount      *int64                    `json:"request_count"`
	PromptTokens      *int64                    `json:"prompt_tokens"`
	CompletionTokens  *int64                    `json:"completion_tokens"`
	TotalTokens       *int64                    `json:"total_tokens"`
	CostCents         *int64                    `json:"cost_cents"`
	MerchantRanking   []PlatformMerchantAIUsage `json:"merchant_ranking"`
	Note              string                    `json:"note"`
}

type PlatformMerchantAIUsage struct {
	MerchantID   uint   `json:"merchant_id"`
	MerchantName string `json:"merchant_name"`
	RequestCount int64  `json:"request_count"`
	TotalTokens  int64  `json:"total_tokens"`
	CostCents    int64  `json:"cost_cents"`
}

type PlatformExpensePeriodSummary struct {
	StartDate                    string                                    `json:"start_date"`
	EndDate                      string                                    `json:"end_date"`
	SubscriptionGrossReceived    int64                                     `json:"subscription_gross_received_cents"`
	SubscriptionConfirmedRefunds int64                                     `json:"subscription_confirmed_refunds_cents"`
	SubscriptionNetReceived      int64                                     `json:"subscription_net_received_cents"`
	ManualVerifiedSubscription   PlatformManualVerifiedSubscriptionSummary `json:"manual_verified_subscription"`
	ActualExpenseCount           int64                                     `json:"actual_expense_count"`
	ActualExpenseTotalCents      *int64                                    `json:"actual_expense_total_cents"`
	ExpenseReversalCount         int64                                     `json:"expense_reversal_count"`
	ExpenseReversalTotalCents    *int64                                    `json:"expense_reversal_total_cents"`
	NetAfterRecordedExpenses     *int64                                    `json:"net_after_recorded_expenses_cents"`
	ExpenseRows                  []PlatformExpenseSummaryRow               `json:"expense_rows"`
	ExpenseReversalRows          []PlatformExpenseReversalSummaryRow       `json:"expense_reversal_rows"`
	ExpenseRowsTruncated         bool                                      `json:"expense_rows_truncated"`
	ExpenseReversalRowsTruncated bool                                      `json:"expense_reversal_rows_truncated"`
	ProfitStatus                 string                                    `json:"profit_status"`
	ProfitStatusNote             string                                    `json:"profit_status_note"`
	PlansStatus                  string                                    `json:"plans_status"`
	PlansNote                    string                                    `json:"plans_note"`
}

type PlatformExpenseSummaryRow struct {
	ID                       uint                      `json:"id"`
	CreatedAt                time.Time                 `json:"created_at"`
	UpdatedAt                time.Time                 `json:"updated_at"`
	ExpenseDate              model.PlatformExpenseDate `json:"expense_date"`
	Category                 string                    `json:"category"`
	AmountCents              int64                     `json:"amount_cents"`
	Currency                 string                    `json:"currency"`
	Memo                     string                    `json:"memo"`
	CreatedBy                uint                      `json:"created_by"`
	ReversalCount            int64                     `json:"reversal_count"`
	ReversedAmountCents      int64                     `json:"reversed_amount_cents"`
	RemainingReversibleCents int64                     `json:"remaining_reversible_cents"`
}

type PlatformExpenseReversalSummaryRow struct {
	ID                  uint                      `json:"id"`
	ExpenseID           uint                      `json:"expense_id"`
	ReversalDate        model.PlatformExpenseDate `json:"reversal_date"`
	AmountCents         int64                     `json:"amount_cents"`
	OriginalExpenseDate model.PlatformExpenseDate `json:"original_expense_date"`
	OriginalCategory    string                    `json:"original_category"`
	OriginalMemo        string                    `json:"original_memo"`
	Reason              string                    `json:"reason"`
}

type PlatformCostSnapshot struct {
	GeneratedAt                time.Time                                 `json:"generated_at"`
	CurrentMonthStart          time.Time                                 `json:"current_month_start"`
	CurrentMonth               PlatformRevenueSummary                    `json:"current_month"`
	CurrentMonthManualVerified PlatformManualVerifiedSubscriptionSummary `json:"current_month_manual_verified_subscription"`
	Lifetime                   PlatformRevenueSummary                    `json:"lifetime"`
	LifetimeManualVerified     PlatformManualVerifiedSubscriptionSummary `json:"lifetime_manual_verified_subscription"`
	SelectedPeriod             PlatformExpensePeriodSummary              `json:"selected_period"`
	AIUsage                    PlatformAIUsageAvailability               `json:"ai_usage"`
	ProfitStatus               string                                    `json:"profit_status"`
	ProfitStatusNote           string                                    `json:"profit_status_note"`
	RevenueBasis               string                                    `json:"revenue_basis"`
}

// PlatformCostOverview returns the current calendar month through today. AI
// usage remains unavailable until real provider data is connected and verified.
func (s *AdminService) PlatformCostOverview() (*PlatformCostSnapshot, error) {
	now := time.Now().In(time.Local)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	return s.PlatformCostOverviewForPeriod(monthStart, now)
}

// PlatformCostOverviewForPeriod treats endDate as inclusive and accepts at most
// 366 calendar days. The returned balance is only subscription net receipts less
// recorded actual expenses; it is never represented as complete profit.
func (s *AdminService) PlatformCostOverviewForPeriod(startDate, endDate time.Time) (*PlatformCostSnapshot, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("platform cost database is unavailable")
	}
	start, end, err := normalizePlatformExpensePeriod(startDate, endDate)
	if err != nil {
		return nil, err
	}
	now := time.Now().In(time.Local)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	tomorrow := today.AddDate(0, 0, 1)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)

	currentMonth, err := s.platformRevenueSummary(&monthStart, &tomorrow)
	if err != nil {
		return nil, err
	}
	currentMonthManualVerified, err := s.platformManualVerifiedSubscriptionSummary(&monthStart, &tomorrow)
	if err != nil {
		return nil, err
	}
	lifetime, err := s.platformRevenueSummary(nil, nil)
	if err != nil {
		return nil, err
	}
	lifetimeManualVerified, err := s.platformManualVerifiedSubscriptionSummary(nil, nil)
	if err != nil {
		return nil, err
	}
	periodRevenue, err := s.platformRevenueSummary(&start, &end)
	if err != nil {
		return nil, err
	}
	periodManualVerified, err := s.platformManualVerifiedSubscriptionSummary(&start, &end)
	if err != nil {
		return nil, err
	}
	expenseSummary, err := s.platformExpenseSummary(start, end)
	if err != nil {
		return nil, err
	}
	if expenseSummary.ActualExpenseTotalCents != nil {
		netAfterRecorded, err := subtractPlatformExpenseCents(periodRevenue.NetReceivedCents, *expenseSummary.ActualExpenseTotalCents)
		if err != nil {
			return nil, fmt.Errorf("net after recorded platform expenses exceeds supported integer range: %w", err)
		}
		expenseSummary.NetAfterRecordedExpenses = &netAfterRecorded
	}

	periodNote := "本期仅统计已确认订阅净实收和实际费用登记；计划/估算不纳入，AI 成本未知。"
	if expenseSummary.ActualExpenseCount == 0 && expenseSummary.ExpenseReversalCount == 0 {
		periodNote = "暂无已录入成本，利润暂不可算。"
	} else {
		periodNote = "当前差额仅为订阅净实收减去本期已登记费用净额（费用减本期冲正），不等于利润；未登记成本及 AI 成本仍未知。"
	}
	return &PlatformCostSnapshot{
		GeneratedAt:                now,
		CurrentMonthStart:          monthStart,
		CurrentMonth:               currentMonth,
		CurrentMonthManualVerified: currentMonthManualVerified,
		Lifetime:                   lifetime,
		LifetimeManualVerified:     lifetimeManualVerified,
		SelectedPeriod: PlatformExpensePeriodSummary{
			StartDate:                    start.Format("2006-01-02"),
			EndDate:                      end.AddDate(0, 0, -1).Format("2006-01-02"),
			SubscriptionGrossReceived:    periodRevenue.GrossReceivedCents,
			SubscriptionConfirmedRefunds: periodRevenue.RefundedCents,
			SubscriptionNetReceived:      periodRevenue.NetReceivedCents,
			ManualVerifiedSubscription:   periodManualVerified,
			ActualExpenseCount:           expenseSummary.ActualExpenseCount,
			ActualExpenseTotalCents:      expenseSummary.ActualExpenseTotalCents,
			ExpenseReversalCount:         expenseSummary.ExpenseReversalCount,
			ExpenseReversalTotalCents:    expenseSummary.ExpenseReversalTotalCents,
			NetAfterRecordedExpenses:     expenseSummary.NetAfterRecordedExpenses,
			ExpenseRows:                  expenseSummary.ExpenseRows,
			ExpenseReversalRows:          expenseSummary.ExpenseReversalRows,
			ExpenseRowsTruncated:         expenseSummary.ExpenseRowsTruncated,
			ExpenseReversalRowsTruncated: expenseSummary.ExpenseReversalRowsTruncated,
			ProfitStatus:                 "incomplete",
			ProfitStatusNote:             periodNote,
			PlansStatus:                  "not_recorded",
			PlansNote:                    "计划/估算费用暂未建账；不显示、不计入本期实际成本或差额。",
		},
		AIUsage: PlatformAIUsageAvailability{
			Status:            "not_connected",
			ProviderConnected: false,
			DataAvailable:     false,
			MerchantRanking:   []PlatformMerchantAIUsage{},
			Note:              "AI 供应商尚未接入；当前日志中的估算值或默认 0 值不作为实际 Token 用量和成本。",
		},
		ProfitStatus:     "incomplete",
		ProfitStatusNote: "利润仍不可完整核算：AI provider/token 成本尚无真实数据，且未登记的实际运营成本未知。页面差额不是净利润。",
		RevenueBasis:     "系统确认实收只统计 merchant_subscription 类型、paid 状态、已记录到账时间且关联成功支付记录的订单，并扣除成功退款；顾客订单、待付款、历史人工核验不并入。人工核验金额单独按管理员核验时间展示，代表管理员对外部凭证及付款发生的声明，不是系统独立核实的银行结算。",
	}, nil
}

type platformExpenseAggregate struct {
	Count int64  `gorm:"column:expense_count"`
	Total string `gorm:"column:expense_total_cents"`
}

type platformExpenseSummary struct {
	ActualExpenseCount           int64
	ActualExpenseTotalCents      *int64
	ExpenseReversalCount         int64
	ExpenseReversalTotalCents    *int64
	NetAfterRecordedExpenses     *int64
	ExpenseRows                  []PlatformExpenseSummaryRow
	ExpenseReversalRows          []PlatformExpenseReversalSummaryRow
	ExpenseRowsTruncated         bool
	ExpenseReversalRowsTruncated bool
}

func (s *AdminService) platformExpenseSummary(start, endExclusive time.Time) (platformExpenseSummary, error) {
	endInclusive := endExclusive.AddDate(0, 0, -1).Format("2006-01-02")
	startDate := start.Format("2006-01-02")
	var expensesAggregate, reversalsAggregate platformExpenseAggregate
	if err := s.db.Model(&model.PlatformExpense{}).
		Select(`COUNT(*) AS expense_count, CAST(COALESCE(SUM(amount_cents), 0) AS CHAR) AS expense_total_cents`).
		Where("expense_date >= ? AND expense_date <= ? AND currency = ?", startDate, endInclusive, "CNY").
		Scan(&expensesAggregate).Error; err != nil {
		return platformExpenseSummary{}, fmt.Errorf("summarize recorded platform expenses: %w", err)
	}
	if err := s.db.Table("platform_expense_reversals AS reversal").
		Select(`COUNT(*) AS expense_count, CAST(COALESCE(SUM(reversal.amount_cents), 0) AS CHAR) AS expense_total_cents`).
		Joins("JOIN platform_expenses AS original_expense ON original_expense.id = reversal.expense_id").
		Where("reversal.reversal_date >= ? AND reversal.reversal_date <= ?", startDate, endInclusive).
		Scan(&reversalsAggregate).Error; err != nil {
		return platformExpenseSummary{}, fmt.Errorf("summarize platform expense reversals: %w", err)
	}
	expenseRows := make([]PlatformExpenseSummaryRow, 0)
	if err := s.db.Table("platform_expenses").
		Select(`platform_expenses.id, platform_expenses.created_at, platform_expenses.updated_at,
		platform_expenses.expense_date, platform_expenses.category,
		platform_expenses.amount_cents, platform_expenses.currency, platform_expenses.memo, platform_expenses.created_by,
		COALESCE(reversal_totals.reversal_count, 0) AS reversal_count,
		COALESCE(reversal_totals.reversed_amount_cents, 0) AS reversed_amount_cents,
		platform_expenses.amount_cents - COALESCE(reversal_totals.reversed_amount_cents, 0) AS remaining_reversible_cents`).
		Joins(`LEFT JOIN (
			SELECT expense_id, COUNT(*) AS reversal_count, SUM(amount_cents) AS reversed_amount_cents
			FROM platform_expense_reversals GROUP BY expense_id
		) AS reversal_totals ON reversal_totals.expense_id = platform_expenses.id`).
		Where("platform_expenses.expense_date >= ? AND platform_expenses.expense_date <= ? AND platform_expenses.currency = ?", startDate, endInclusive, "CNY").
		Order("platform_expenses.expense_date DESC, platform_expenses.id DESC").Limit(platformExpenseListLimit).Scan(&expenseRows).Error; err != nil {
		return platformExpenseSummary{}, fmt.Errorf("list recorded platform expenses: %w", err)
	}
	reversalRows := make([]PlatformExpenseReversalSummaryRow, 0)
	if err := s.db.Table("platform_expense_reversals AS reversal").
		Select(`reversal.id AS id, reversal.expense_id AS expense_id, reversal.reversal_date AS reversal_date,
		reversal.amount_cents AS amount_cents, original_expense.expense_date AS original_expense_date,
		original_expense.category AS original_category, original_expense.memo AS original_memo, reversal.reason AS reason`).
		Joins("JOIN platform_expenses AS original_expense ON original_expense.id = reversal.expense_id").
		Where("reversal.reversal_date >= ? AND reversal.reversal_date <= ?", startDate, endInclusive).
		Order("reversal.reversal_date DESC, reversal.id DESC").Limit(platformExpenseListLimit).Scan(&reversalRows).Error; err != nil {
		return platformExpenseSummary{}, fmt.Errorf("list platform expense reversals: %w", err)
	}
	result := platformExpenseSummary{
		ActualExpenseCount:           expensesAggregate.Count,
		ExpenseReversalCount:         reversalsAggregate.Count,
		ExpenseRows:                  expenseRows,
		ExpenseReversalRows:          reversalRows,
		ExpenseRowsTruncated:         expensesAggregate.Count > int64(len(expenseRows)),
		ExpenseReversalRowsTruncated: reversalsAggregate.Count > int64(len(reversalRows)),
	}
	if reversalsAggregate.Count > 0 {
		total, err := parseAggregateCents(reversalsAggregate.Total)
		if err != nil {
			return platformExpenseSummary{}, fmt.Errorf("expense reversal sum exceeds supported integer range: %w", err)
		}
		if total < 0 {
			return platformExpenseSummary{}, errors.New("expense reversal total cannot be negative")
		}
		result.ExpenseReversalTotalCents = &total
	}
	if expensesAggregate.Count > 0 || reversalsAggregate.Count > 0 {
		var expenseTotal int64
		if expensesAggregate.Count > 0 {
			parsedExpenseTotal, parseErr := parseAggregateCents(expensesAggregate.Total)
			if parseErr != nil {
				return platformExpenseSummary{}, fmt.Errorf("expense sum exceeds supported integer range: %w", parseErr)
			}
			expenseTotal = parsedExpenseTotal
		}
		reversalTotal := int64(0)
		if result.ExpenseReversalTotalCents != nil {
			reversalTotal = *result.ExpenseReversalTotalCents
		}
		netCost, err := subtractPlatformExpenseCents(expenseTotal, reversalTotal)
		if err != nil {
			return platformExpenseSummary{}, fmt.Errorf("net platform expense total exceeds supported integer range: %w", err)
		}
		result.ActualExpenseTotalCents = &netCost
	}
	return result, nil
}

// CreatePlatformExpense appends an actual CNY expense and its privacy-minimized
// audit in one transaction. Replaying the same UUID and identical payload is
// idempotent and does not create another row or audit.
func (s *AdminService) CreatePlatformExpense(request dto.CreatePlatformExpenseRequest, actor AuditActor) (*model.PlatformExpense, bool, error) {
	if s == nil || s.db == nil {
		return nil, false, errors.New("platform cost database is unavailable")
	}
	entry, err := validatePlatformExpenseRequest(request, actor.ID)
	if err != nil {
		return nil, false, err
	}
	if actor.ID == 0 || strings.TrimSpace(actor.Type) != "super_admin" {
		return nil, false, ErrInvalidPlatformExpense
	}

	created := false
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var existing model.PlatformExpense
		lookupErr := tx.Where("idempotency_key = ?", entry.IdempotencyKey).Take(&existing).Error
		if lookupErr == nil {
			if !samePlatformExpensePayload(existing, *entry) {
				return ErrPlatformExpenseIdempotencyConflict
			}
			*entry = existing
			return nil
		}
		if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			return lookupErr
		}
		if err := tx.Create(entry).Error; err != nil {
			return err
		}
		if err := AppendAuditTx(tx, AuditEvent{
			Actor: actor,
			Input: AuditRecordInput{Action: "platform_expense_create", TargetType: "platform_expense", TargetID: entry.ID},
		}); err != nil {
			return err
		}
		created = true
		return nil
	})
	if err == nil {
		return entry, created, nil
	}
	if errors.Is(err, ErrPlatformExpenseIdempotencyConflict) {
		return nil, false, err
	}
	// If concurrent retries race on the unique key, read the committed row and
	// apply the same payload check. A database failure without a committed row
	// remains an error and the transaction (including audit) is rolled back.
	var existing model.PlatformExpense
	if lookupErr := s.db.Where("idempotency_key = ?", entry.IdempotencyKey).Take(&existing).Error; lookupErr == nil {
		if samePlatformExpensePayload(existing, *entry) {
			return &existing, false, nil
		}
		return nil, false, ErrPlatformExpenseIdempotencyConflict
	}
	return nil, false, err
}

// CreatePlatformExpenseReversal appends one partial or full reversal for an
// immutable expense. The original row is locked while its remaining balance is
// checked so concurrent reversals cannot collectively exceed the source amount.
// The reversal date, rather than the original expense date, determines the period.
func (s *AdminService) CreatePlatformExpenseReversal(expenseID uint, request dto.CreatePlatformExpenseReversalRequest, actor AuditActor) (*model.PlatformExpenseReversal, bool, error) {
	if s == nil || s.db == nil {
		return nil, false, errors.New("platform cost database is unavailable")
	}
	entry, err := validatePlatformExpenseReversalRequest(expenseID, request, actor.ID)
	if err != nil {
		return nil, false, err
	}
	if actor.ID == 0 || strings.TrimSpace(actor.Type) != "super_admin" {
		return nil, false, ErrInvalidPlatformExpense
	}

	created := false
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var original model.PlatformExpense
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&original, expenseID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPlatformExpenseNotFound
			}
			return fmt.Errorf("load original platform expense: %w", err)
		}
		var existing model.PlatformExpenseReversal
		lookupErr := tx.Where("idempotency_key = ?", entry.IdempotencyKey).Take(&existing).Error
		if lookupErr == nil {
			if !samePlatformExpenseReversalPayload(existing, *entry) {
				return ErrPlatformExpenseReversalKeyConflict
			}
			*entry = existing
			return nil
		}
		if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			return lookupErr
		}
		var reversedAmountCents int64
		if err := tx.Model(&model.PlatformExpenseReversal{}).Where("expense_id = ?", expenseID).
			Select("COALESCE(SUM(amount_cents), 0)").Scan(&reversedAmountCents).Error; err != nil {
			return fmt.Errorf("sum existing platform expense reversals: %w", err)
		}
		if original.AmountCents <= 0 || reversedAmountCents < 0 || reversedAmountCents > original.AmountCents || entry.AmountCents > original.AmountCents-reversedAmountCents {
			return ErrPlatformExpenseReversalExceedsOriginal
		}
		if err := tx.Create(entry).Error; err != nil {
			return err
		}
		if err := AppendAuditTx(tx, AuditEvent{
			Actor: actor,
			Input: AuditRecordInput{Action: "platform_expense_reversal_create", TargetType: "platform_expense_reversal", TargetID: entry.ID},
		}); err != nil {
			return err
		}
		created = true
		return nil
	})
	if err == nil {
		return entry, created, nil
	}
	if errors.Is(err, ErrPlatformExpenseReversalKeyConflict) || errors.Is(err, ErrPlatformExpenseReversalExceedsOriginal) {
		return nil, false, err
	}
	var existing model.PlatformExpenseReversal
	if lookupErr := s.db.Where("idempotency_key = ?", entry.IdempotencyKey).Take(&existing).Error; lookupErr == nil {
		if samePlatformExpenseReversalPayload(existing, *entry) {
			return &existing, false, nil
		}
		return nil, false, ErrPlatformExpenseReversalKeyConflict
	}
	return nil, false, err
}

func validatePlatformExpenseReversalRequest(expenseID uint, request dto.CreatePlatformExpenseReversalRequest, actorID uint) (*model.PlatformExpenseReversal, error) {
	if expenseID == 0 || actorID == 0 {
		return nil, fmt.Errorf("%w: expense and actor are required", ErrInvalidPlatformExpense)
	}
	if request.AmountCents <= 0 || request.AmountCents > maxPlatformExpenseCents {
		return nil, fmt.Errorf("%w: amount_cents must be positive and within the supported range", ErrInvalidPlatformExpense)
	}
	reversalDateText := strings.TrimSpace(request.ReversalDate)
	reversalDay, err := time.Parse("2006-01-02", reversalDateText)
	if err != nil || reversalDay.Format("2006-01-02") != reversalDateText {
		return nil, fmt.Errorf("%w: reversal_date must use YYYY-MM-DD", ErrInvalidPlatformExpense)
	}
	if reversalDateText > time.Now().In(time.Local).Format("2006-01-02") {
		return nil, fmt.Errorf("%w: reversal_date cannot be in the future", ErrInvalidPlatformExpense)
	}
	reason := strings.TrimSpace(request.Reason)
	if reason == "" || utf8.RuneCountInString(reason) > 500 {
		return nil, fmt.Errorf("%w: reason must contain 1 to 500 characters", ErrInvalidPlatformExpense)
	}
	for _, r := range reason {
		if unicode.IsControl(r) {
			return nil, fmt.Errorf("%w: reason cannot contain control characters", ErrInvalidPlatformExpense)
		}
	}
	key, err := uuid.Parse(strings.TrimSpace(request.IdempotencyKey))
	if err != nil || key.String() == uuid.Nil.String() {
		return nil, fmt.Errorf("%w: idempotency_key must be a non-zero UUID", ErrInvalidPlatformExpense)
	}
	return &model.PlatformExpenseReversal{
		ExpenseID:      expenseID,
		AmountCents:    request.AmountCents,
		ReversalDate:   model.PlatformExpenseDate(reversalDateText),
		Reason:         reason,
		CreatedBy:      actorID,
		IdempotencyKey: key.String(),
	}, nil
}

func samePlatformExpenseReversalPayload(existing, candidate model.PlatformExpenseReversal) bool {
	return existing.ExpenseID == candidate.ExpenseID && existing.AmountCents == candidate.AmountCents && existing.ReversalDate == candidate.ReversalDate && existing.Reason == candidate.Reason && existing.CreatedBy == candidate.CreatedBy
}

func subtractPlatformExpenseCents(left, right int64) (int64, error) {
	result := new(big.Int).Sub(big.NewInt(left), big.NewInt(right))
	if !result.IsInt64() {
		return 0, errors.New("result is not a signed 64-bit integer")
	}
	return result.Int64(), nil
}

func validatePlatformExpenseRequest(request dto.CreatePlatformExpenseRequest, actorID uint) (*model.PlatformExpense, error) {
	category := strings.TrimSpace(request.Category)
	switch category {
	case model.PlatformExpenseCategoryCloudHosting, model.PlatformExpenseCategoryMessagingAPI,
		model.PlatformExpenseCategoryLaborContractor, model.PlatformExpenseCategoryCustomerAcquisitionTravel,
		model.PlatformExpenseCategoryEquipment, model.PlatformExpenseCategoryOther:
	default:
		return nil, fmt.Errorf("%w: unsupported category", ErrInvalidPlatformExpense)
	}
	if request.AmountCents <= 0 || request.AmountCents > maxPlatformExpenseCents {
		return nil, fmt.Errorf("%w: amount_cents must be between 1 and %d", ErrInvalidPlatformExpense, maxPlatformExpenseCents)
	}
	if strings.TrimSpace(request.Currency) != "CNY" {
		return nil, fmt.Errorf("%w: currency must be CNY", ErrInvalidPlatformExpense)
	}
	expenseDay, err := time.Parse("2006-01-02", strings.TrimSpace(request.ExpenseDate))
	if err != nil || expenseDay.Format("2006-01-02") != strings.TrimSpace(request.ExpenseDate) {
		return nil, fmt.Errorf("%w: expense_date must use YYYY-MM-DD", ErrInvalidPlatformExpense)
	}
	if expenseDay.Format("2006-01-02") > time.Now().In(time.Local).Format("2006-01-02") {
		return nil, fmt.Errorf("%w: expense_date cannot be in the future", ErrInvalidPlatformExpense)
	}
	memo := strings.TrimSpace(request.Memo)
	if memo == "" || utf8.RuneCountInString(memo) > maxPlatformExpenseMemoRunes {
		return nil, fmt.Errorf("%w: memo must contain 1 to %d characters", ErrInvalidPlatformExpense, maxPlatformExpenseMemoRunes)
	}
	for _, r := range memo {
		if unicode.IsControl(r) || r == '<' || r == '>' {
			return nil, fmt.Errorf("%w: memo cannot contain control characters or HTML markup", ErrInvalidPlatformExpense)
		}
	}
	key, err := uuid.Parse(strings.TrimSpace(request.IdempotencyKey))
	if err != nil || key.String() == uuid.Nil.String() {
		return nil, fmt.Errorf("%w: idempotency_key must be a non-zero UUID", ErrInvalidPlatformExpense)
	}
	return &model.PlatformExpense{
		Category:       category,
		AmountCents:    request.AmountCents,
		Currency:       "CNY",
		ExpenseDate:    model.PlatformExpenseDate(expenseDay.Format("2006-01-02")),
		Memo:           memo,
		CreatedBy:      actorID,
		IdempotencyKey: key.String(),
	}, nil
}

func samePlatformExpensePayload(existing, candidate model.PlatformExpense) bool {
	return existing.CreatedBy == candidate.CreatedBy && existing.Category == candidate.Category && existing.AmountCents == candidate.AmountCents && existing.Currency == candidate.Currency && existing.ExpenseDate == candidate.ExpenseDate && existing.Memo == candidate.Memo
}

func normalizePlatformExpensePeriod(startDate, endDate time.Time) (time.Time, time.Time, error) {
	if startDate.IsZero() || endDate.IsZero() {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: start_date and end_date are required", ErrInvalidPlatformExpense)
	}
	start := time.Date(startDate.In(time.Local).Year(), startDate.In(time.Local).Month(), startDate.In(time.Local).Day(), 0, 0, 0, 0, time.Local)
	endDay := endDate.In(time.Local)
	end := time.Date(endDay.Year(), endDay.Month(), endDay.Day(), 0, 0, 0, 0, time.Local)
	if end.Before(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: end_date precedes start_date", ErrInvalidPlatformExpense)
	}
	today := time.Now().In(time.Local)
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.Local)
	if end.After(today) {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: end_date cannot be in the future", ErrInvalidPlatformExpense)
	}
	endExclusive := end.AddDate(0, 0, 1)
	if endExclusive.After(start.AddDate(0, 0, 366)) {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: period cannot exceed 366 days", ErrInvalidPlatformExpense)
	}
	return start, endExclusive, nil
}

func parseAggregateCents(raw string) (int64, error) {
	value := new(big.Int)
	if _, ok := value.SetString(strings.TrimSpace(raw), 10); !ok || !value.IsInt64() {
		return 0, errors.New("aggregate is not a signed 64-bit integer")
	}
	return value.Int64(), nil
}

func (s *AdminService) platformRevenueSummary(start, end *time.Time) (PlatformRevenueSummary, error) {
	var row struct {
		PaidOrderCount     int64  `gorm:"column:paid_order_count"`
		MerchantCount      int64  `gorm:"column:merchant_count"`
		GrossReceivedCents string `gorm:"column:gross_received_cents"`
		RefundedCents      string `gorm:"column:refunded_cents"`
	}

	query := s.db.Table("orders AS o").
		Joins(`JOIN (
			SELECT order_id, SUM(amount) AS paid_amount
			FROM payment_records
			WHERE status = ? AND amount > 0
			GROUP BY order_id
		) AS confirmed_payments ON confirmed_payments.order_id = o.id`, "success").
		Joins(`LEFT JOIN (
			SELECT order_id, SUM(amount) AS refunded_amount
			FROM refund_records
			WHERE status = ? AND amount > 0
			GROUP BY order_id
		) AS confirmed_refunds ON confirmed_refunds.order_id = o.id`, "success").
		Where("o.order_type = ? AND o.status = ? AND o.paid_at IS NOT NULL AND o.total_amount > 0", "merchant_subscription", "paid").
		Where("(o.payment_channel IS NULL OR o.payment_channel <> ?)", "admin_manual_verified")
	if start != nil {
		query = query.Where("o.paid_at >= ?", *start)
	}
	if end != nil {
		query = query.Where("o.paid_at < ?", *end)
	}

	selectSQL := `COUNT(DISTINCT o.id) AS paid_order_count,
		COUNT(DISTINCT o.merchant_id) AS merchant_count,
		CAST(COALESCE(SUM(confirmed_payments.paid_amount), 0) AS CHAR) AS gross_received_cents,
		CAST(COALESCE(SUM(CASE
			WHEN COALESCE(confirmed_refunds.refunded_amount, 0) > confirmed_payments.paid_amount THEN confirmed_payments.paid_amount
			ELSE COALESCE(confirmed_refunds.refunded_amount, 0)
		END), 0) AS CHAR) AS refunded_cents`
	if err := query.Select(selectSQL).Scan(&row).Error; err != nil {
		return PlatformRevenueSummary{}, err
	}
	gross, err := parseAggregateCents(row.GrossReceivedCents)
	if err != nil {
		return PlatformRevenueSummary{}, fmt.Errorf("confirmed subscription receipts exceed supported integer range: %w", err)
	}
	refunded, err := parseAggregateCents(row.RefundedCents)
	if err != nil {
		return PlatformRevenueSummary{}, fmt.Errorf("confirmed subscription refunds exceed supported integer range: %w", err)
	}
	if gross < 0 || refunded < 0 {
		return PlatformRevenueSummary{}, errors.New("confirmed subscription totals cannot be negative")
	}
	if refunded > gross {
		refunded = gross
	}
	return PlatformRevenueSummary{
		PaidOrderCount:     row.PaidOrderCount,
		MerchantCount:      row.MerchantCount,
		GrossReceivedCents: gross,
		RefundedCents:      refunded,
		NetReceivedCents:   gross - refunded,
	}, nil
}

// platformManualVerifiedSubscriptionSummary reads the existing immutable
// manual-review and audit ledgers. These administrator attestations are
// periodized by the audited review time and never merged into provider receipts.
func (s *AdminService) platformManualVerifiedSubscriptionSummary(start, end *time.Time) (PlatformManualVerifiedSubscriptionSummary, error) {
	var row struct {
		ReviewCount         int64  `gorm:"column:review_count"`
		MerchantCount       int64  `gorm:"column:merchant_count"`
		AttestedAmountCents string `gorm:"column:attested_amount_cents"`
	}

	query := s.db.Table("merchant_subscription_manual_reviews AS review").
		Joins(`JOIN merchant_subscription_manual_review_audits AS review_audit
			ON review_audit.review_id = review.id
			AND review_audit.order_id = review.order_id
			AND review_audit.merchant_id = review.merchant_id`).
		Joins("JOIN orders AS o ON o.id = review.order_id AND o.merchant_id = review.merchant_id").
		Where("o.order_type = ? AND o.status = ? AND o.payment_channel = ?", "merchant_subscription", "manually_verified", "admin_manual_verified").
		Where("o.paid_at IS NULL AND COALESCE(o.transaction_no, '') = ''").
		Where("o.amount > 0 AND o.total_amount = o.amount AND review.amount_cents = o.amount").
		Where("review.review_status = ? AND review.payment_occurred_attested = ?", "manually_verified", true).
		Where("review_audit.order_status_before = ? AND review_audit.order_status_after = ? AND review_audit.order_paid_at_after IS NULL", "pending", "manually_verified").
		Where("review_audit.payment_occurred_attested = ? AND review_audit.amount_cents = review.amount_cents", true).
		Where("review_audit.actor_id = review.actor_id AND review_audit.actor_role = review.actor_role").
		Where("review_audit.plan_name = review.plan_name AND review_audit.duration_days = review.duration_days").
		Where("review_audit.evidence_reference = review.evidence_reference AND TRIM(review.evidence_reference) <> ''").
		Where("review_audit.reason = review.reason").
		Where("NOT EXISTS (SELECT 1 FROM payment_records AS p WHERE p.order_id = o.id)")
	if start != nil {
		query = query.Where("review_audit.reviewed_at >= ?", start.UTC())
	}
	if end != nil {
		query = query.Where("review_audit.reviewed_at < ?", end.UTC())
	}

	selectSQL := `COUNT(DISTINCT review.id) AS review_count,
		COUNT(DISTINCT review.merchant_id) AS merchant_count,
		CAST(COALESCE(SUM(review_audit.amount_cents), 0) AS CHAR) AS attested_amount_cents`
	if err := query.Select(selectSQL).Scan(&row).Error; err != nil {
		return PlatformManualVerifiedSubscriptionSummary{}, fmt.Errorf("summarize audited manual subscription verifications: %w", err)
	}
	amount, err := parseAggregateCents(row.AttestedAmountCents)
	if err != nil {
		return PlatformManualVerifiedSubscriptionSummary{}, fmt.Errorf("audited manual subscription amount exceeds supported integer range: %w", err)
	}
	if amount < 0 {
		return PlatformManualVerifiedSubscriptionSummary{}, errors.New("audited manual subscription amount cannot be negative")
	}
	return PlatformManualVerifiedSubscriptionSummary{
		ReviewCount:         row.ReviewCount,
		MerchantCount:       row.MerchantCount,
		AttestedAmountCents: amount,
	}, nil
}
