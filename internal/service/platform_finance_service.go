package service

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

const (
	maxPlatformTimeLogHourlyValueCents int64 = 10_000_000 // CNY 100,000 per hour, reference value only.
	maxPlatformFinanceNoteRunes              = 500
	platformFinanceListLimit                 = 500
)

var (
	ErrInvalidPlatformTimeLog             = errors.New("invalid platform time log")
	ErrPlatformTimeLogIdempotencyConflict = errors.New("platform time log idempotency key conflicts with an existing entry")
	ErrInvalidPlatformIncomeRecord        = errors.New("invalid platform income record")
	ErrPlatformIncomeRecordNotFound       = errors.New("platform income record was not found")
	ErrPlatformIncomeTransitionNotAllowed = errors.New("platform income status transition is not allowed")
	ErrPlatformIncomeIdempotencyConflict  = errors.New("platform income idempotency key conflicts with an existing entry")
	ErrInvalidPlatformFinancePeriod       = errors.New("invalid platform finance period")
)

// CreatePlatformTimeLog appends one operator time block as economic cost
// only. The amount is computed server-side from integer hundredths of an
// hour; the client-supplied hours value never enters money math directly.
func (s *AdminService) CreatePlatformTimeLog(request dto.CreatePlatformTimeLogRequest, actor AuditActor) (*model.PlatformTimeLog, bool, error) {
	if s == nil || s.db == nil {
		return nil, false, errors.New("platform cost database is unavailable")
	}
	entry, err := validatePlatformTimeLogRequest(request, actor.ID)
	if err != nil {
		return nil, false, err
	}
	if actor.ID == 0 || strings.TrimSpace(actor.Type) != "super_admin" {
		return nil, false, ErrInvalidPlatformTimeLog
	}

	created := false
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var existing model.PlatformTimeLog
		lookupErr := tx.Where("idempotency_key = ?", entry.IdempotencyKey).Take(&existing).Error
		if lookupErr == nil {
			if !samePlatformTimeLogPayload(existing, *entry) {
				return ErrPlatformTimeLogIdempotencyConflict
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
			Input: AuditRecordInput{Action: "platform_time_log_create", TargetType: "platform_time_log", TargetID: entry.ID},
		}); err != nil {
			return err
		}
		created = true
		return nil
	})
	if err == nil {
		return entry, created, nil
	}
	if errors.Is(err, ErrPlatformTimeLogIdempotencyConflict) {
		return nil, false, err
	}
	var existing model.PlatformTimeLog
	if lookupErr := s.db.Where("idempotency_key = ?", entry.IdempotencyKey).Take(&existing).Error; lookupErr == nil {
		if samePlatformTimeLogPayload(existing, *entry) {
			return &existing, false, nil
		}
		return nil, false, ErrPlatformTimeLogIdempotencyConflict
	}
	return nil, false, err
}

func validatePlatformTimeLogRequest(request dto.CreatePlatformTimeLogRequest, actorID uint) (*model.PlatformTimeLog, error) {
	if actorID == 0 {
		return nil, fmt.Errorf("%w: actor is required", ErrInvalidPlatformTimeLog)
	}
	workType := strings.TrimSpace(request.WorkType)
	switch workType {
	case model.PlatformTimeLogWorkProductPlanning, model.PlatformTimeLogWorkRequirementAnalysis,
		model.PlatformTimeLogWorkDevelopment, model.PlatformTimeLogWorkTesting,
		model.PlatformTimeLogWorkDeploymentMaintenance, model.PlatformTimeLogWorkBusinessCommunication,
		model.PlatformTimeLogWorkContentCreation:
	default:
		return nil, fmt.Errorf("%w: unsupported work_type", ErrInvalidPlatformTimeLog)
	}
	logDay, err := time.Parse("2006-01-02", strings.TrimSpace(request.LogDate))
	if err != nil || logDay.Format("2006-01-02") != strings.TrimSpace(request.LogDate) {
		return nil, fmt.Errorf("%w: log_date must use YYYY-MM-DD", ErrInvalidPlatformTimeLog)
	}
	if logDay.Format("2006-01-02") > time.Now().In(time.Local).Format("2006-01-02") {
		return nil, fmt.Errorf("%w: log_date cannot be in the future", ErrInvalidPlatformTimeLog)
	}
	content := strings.TrimSpace(request.Content)
	if content == "" || utf8.RuneCountInString(content) > maxPlatformFinanceNoteRunes {
		return nil, fmt.Errorf("%w: content must contain 1 to %d characters", ErrInvalidPlatformTimeLog, maxPlatformFinanceNoteRunes)
	}
	for _, r := range content {
		if unicode.IsControl(r) || r == '<' || r == '>' {
			return nil, fmt.Errorf("%w: content cannot contain control characters or HTML markup", ErrInvalidPlatformTimeLog)
		}
	}
	if math.IsNaN(request.Hours) || math.IsInf(request.Hours, 0) || request.Hours <= 0 || request.Hours > 24 {
		return nil, fmt.Errorf("%w: hours must be between 0 and 24", ErrInvalidPlatformTimeLog)
	}
	hoursScaled := int64(math.Round(request.Hours * 100))
	if math.Abs(request.Hours*100-float64(hoursScaled)) > 1e-9 || hoursScaled < 1 || hoursScaled > 2400 {
		return nil, fmt.Errorf("%w: hours accepts at most two decimals and at most 24 hours per entry", ErrInvalidPlatformTimeLog)
	}
	if request.ReferenceHourlyValueCents <= 0 || request.ReferenceHourlyValueCents > maxPlatformTimeLogHourlyValueCents {
		return nil, fmt.Errorf("%w: reference_hourly_value_cents must be positive and within the supported range", ErrInvalidPlatformTimeLog)
	}
	module := strings.TrimSpace(request.Module)
	if utf8.RuneCountInString(module) > 80 {
		return nil, fmt.Errorf("%w: module must contain at most 80 characters", ErrInvalidPlatformTimeLog)
	}
	for _, r := range module {
		if unicode.IsControl(r) || r == '<' || r == '>' {
			return nil, fmt.Errorf("%w: module cannot contain control characters or HTML markup", ErrInvalidPlatformTimeLog)
		}
	}
	key, err := uuid.Parse(strings.TrimSpace(request.IdempotencyKey))
	if err != nil || key.String() == uuid.Nil.String() {
		return nil, fmt.Errorf("%w: idempotency_key must be a non-zero UUID", ErrInvalidPlatformTimeLog)
	}
	amount := model.PlatformTimeLogAmountCents(hoursScaled, request.ReferenceHourlyValueCents)
	if amount <= 0 {
		return nil, fmt.Errorf("%w: computed amount rounds to zero; increase hours or the reference hourly value", ErrInvalidPlatformTimeLog)
	}
	return &model.PlatformTimeLog{
		LogDate:                   model.PlatformExpenseDate(logDay.Format("2006-01-02")),
		WorkType:                  workType,
		Content:                   content,
		HoursScaled:               hoursScaled,
		ReferenceHourlyValueCents: request.ReferenceHourlyValueCents,
		AmountCents:               amount,
		Module:                    module,
		CreatedBy:                 actorID,
		IdempotencyKey:            key.String(),
	}, nil
}

func samePlatformTimeLogPayload(existing, candidate model.PlatformTimeLog) bool {
	return existing.LogDate == candidate.LogDate && existing.WorkType == candidate.WorkType &&
		existing.Content == candidate.Content && existing.HoursScaled == candidate.HoursScaled &&
		existing.ReferenceHourlyValueCents == candidate.ReferenceHourlyValueCents &&
		existing.AmountCents == candidate.AmountCents && existing.Module == candidate.Module &&
		existing.CreatedBy == candidate.CreatedBy
}

type PlatformTimeLogListResult struct {
	Rows             []model.PlatformTimeLog `json:"rows"`
	TotalHoursScaled int64                   `json:"total_hours_scaled"`
	TotalAmountCents int64                   `json:"total_amount_cents"`
	Truncated        bool                    `json:"truncated"`
}

// ListPlatformTimeLogs lists time logs, optionally filtered by month
// (YYYY-MM) and work type. Totals cover the same filter, not just the page.
func (s *AdminService) ListPlatformTimeLogs(month, workType string) (*PlatformTimeLogListResult, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("platform cost database is unavailable")
	}
	start, end, err := platformFinanceMonthRange(month)
	if err != nil {
		return nil, err
	}
	workType = strings.TrimSpace(workType)
	if workType != "" {
		switch workType {
		case model.PlatformTimeLogWorkProductPlanning, model.PlatformTimeLogWorkRequirementAnalysis,
			model.PlatformTimeLogWorkDevelopment, model.PlatformTimeLogWorkTesting,
			model.PlatformTimeLogWorkDeploymentMaintenance, model.PlatformTimeLogWorkBusinessCommunication,
			model.PlatformTimeLogWorkContentCreation:
		default:
			return nil, fmt.Errorf("%w: unsupported work_type", ErrInvalidPlatformTimeLog)
		}
	}
	query := s.db.Model(&model.PlatformTimeLog{})
	if start != nil {
		query = query.Where("log_date >= ? AND log_date < ?", start.Format("2006-01-02"), end.Format("2006-01-02"))
	}
	if workType != "" {
		query = query.Where("work_type = ?", workType)
	}
	var total struct {
		Hours  string `gorm:"column:total_hours"`
		Amount string `gorm:"column:total_amount"`
		Count  int64  `gorm:"column:row_count"`
	}
	if err := query.Select(`COUNT(*) AS row_count, CAST(COALESCE(SUM(hours_scaled), 0) AS CHAR) AS total_hours, CAST(COALESCE(SUM(amount_cents), 0) AS CHAR) AS total_amount`).Scan(&total).Error; err != nil {
		return nil, fmt.Errorf("summarize platform time logs: %w", err)
	}
	rows := make([]model.PlatformTimeLog, 0)
	listQuery := s.db.Model(&model.PlatformTimeLog{})
	if start != nil {
		listQuery = listQuery.Where("log_date >= ? AND log_date < ?", start.Format("2006-01-02"), end.Format("2006-01-02"))
	}
	if workType != "" {
		listQuery = listQuery.Where("work_type = ?", workType)
	}
	if err := listQuery.Order("log_date DESC, id DESC").Limit(platformFinanceListLimit).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list platform time logs: %w", err)
	}
	hours, err := parseAggregateCents(total.Hours)
	if err != nil {
		return nil, fmt.Errorf("time log hours exceed supported integer range: %w", err)
	}
	amount, err := parseAggregateCents(total.Amount)
	if err != nil {
		return nil, fmt.Errorf("time log amount exceeds supported integer range: %w", err)
	}
	return &PlatformTimeLogListResult{
		Rows:             rows,
		TotalHoursScaled: hours,
		TotalAmountCents: amount,
		Truncated:        total.Count > int64(len(rows)),
	}, nil
}

// CreatePlatformIncomeRecord appends one income entry. Only the status flow
// may change it afterwards; amount, type, merchant and dates are immutable.
func (s *AdminService) CreatePlatformIncomeRecord(request dto.CreatePlatformIncomeRecordRequest, actor AuditActor) (*model.PlatformIncomeRecord, bool, error) {
	if s == nil || s.db == nil {
		return nil, false, errors.New("platform cost database is unavailable")
	}
	entry, err := validatePlatformIncomeRecordRequest(request, actor.ID)
	if err != nil {
		return nil, false, err
	}
	if actor.ID == 0 || strings.TrimSpace(actor.Type) != "super_admin" {
		return nil, false, ErrInvalidPlatformIncomeRecord
	}

	created := false
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var existing model.PlatformIncomeRecord
		lookupErr := tx.Where("idempotency_key = ?", entry.IdempotencyKey).Take(&existing).Error
		if lookupErr == nil {
			if !samePlatformIncomeRecordPayload(existing, *entry) {
				return ErrPlatformIncomeIdempotencyConflict
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
			Input: AuditRecordInput{Action: "platform_income_record_create", TargetType: "platform_income_record", TargetID: entry.ID},
		}); err != nil {
			return err
		}
		created = true
		return nil
	})
	if err == nil {
		return entry, created, nil
	}
	if errors.Is(err, ErrPlatformIncomeIdempotencyConflict) {
		return nil, false, err
	}
	var existing model.PlatformIncomeRecord
	if lookupErr := s.db.Where("idempotency_key = ?", entry.IdempotencyKey).Take(&existing).Error; lookupErr == nil {
		if samePlatformIncomeRecordPayload(existing, *entry) {
			return &existing, false, nil
		}
		return nil, false, ErrPlatformIncomeIdempotencyConflict
	}
	return nil, false, err
}

func platformIncomeOptionalID(value *uint) *uint {
	if value == nil || *value == 0 {
		return nil
	}
	return value
}

func platformIncomeOptionalDate(value *string) (*model.PlatformExpenseDate, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	day, err := time.Parse("2006-01-02", strings.TrimSpace(*value))
	if err != nil || day.Format("2006-01-02") != strings.TrimSpace(*value) {
		return nil, fmt.Errorf("date must use YYYY-MM-DD")
	}
	date := model.PlatformExpenseDate(day.Format("2006-01-02"))
	return &date, nil
}

func validatePlatformIncomeRecordRequest(request dto.CreatePlatformIncomeRecordRequest, actorID uint) (*model.PlatformIncomeRecord, error) {
	if actorID == 0 {
		return nil, fmt.Errorf("%w: actor is required", ErrInvalidPlatformIncomeRecord)
	}
	incomeType := strings.TrimSpace(request.IncomeType)
	switch incomeType {
	case model.PlatformIncomeTypeSubscriptionBasic, model.PlatformIncomeTypeSubscriptionPremium,
		model.PlatformIncomeTypeAddonModule, model.PlatformIncomeTypeDeploymentSetup,
		model.PlatformIncomeTypeAIUsagePackage, model.PlatformIncomeTypeOther:
	default:
		return nil, fmt.Errorf("%w: unsupported income_type", ErrInvalidPlatformIncomeRecord)
	}
	if request.AmountCents <= 0 || request.AmountCents > maxPlatformExpenseCents {
		return nil, fmt.Errorf("%w: amount_cents must be positive and within the supported range", ErrInvalidPlatformIncomeRecord)
	}
	if strings.TrimSpace(request.Currency) != "CNY" {
		return nil, fmt.Errorf("%w: currency must be CNY", ErrInvalidPlatformIncomeRecord)
	}
	occurredDay, err := time.Parse("2006-01-02", strings.TrimSpace(request.OccurredDate))
	if err != nil || occurredDay.Format("2006-01-02") != strings.TrimSpace(request.OccurredDate) {
		return nil, fmt.Errorf("%w: occurred_date must use YYYY-MM-DD", ErrInvalidPlatformIncomeRecord)
	}
	if occurredDay.Format("2006-01-02") > time.Now().In(time.Local).Format("2006-01-02") {
		return nil, fmt.Errorf("%w: occurred_date cannot be in the future", ErrInvalidPlatformIncomeRecord)
	}
	status := strings.TrimSpace(request.Status)
	if status == "" {
		status = model.PlatformIncomeStatusOrdered
	}
	switch status {
	case model.PlatformIncomeStatusOrdered, model.PlatformIncomeStatusConfirmed, model.PlatformIncomeStatusReceived,
		model.PlatformIncomeStatusPendingReceipt, model.PlatformIncomeStatusRefunded, model.PlatformIncomeStatusRefunding:
	default:
		return nil, fmt.Errorf("%w: unsupported status", ErrInvalidPlatformIncomeRecord)
	}
	receivedDate, err := platformIncomeOptionalDate(request.ReceivedDate)
	if err != nil {
		return nil, fmt.Errorf("%w: received_date %v", ErrInvalidPlatformIncomeRecord, err)
	}
	if receivedDate != nil {
		if string(*receivedDate) > time.Now().In(time.Local).Format("2006-01-02") {
			return nil, fmt.Errorf("%w: received_date cannot be in the future", ErrInvalidPlatformIncomeRecord)
		}
		if string(*receivedDate) < occurredDay.Format("2006-01-02") {
			return nil, fmt.Errorf("%w: received_date cannot precede occurred_date", ErrInvalidPlatformIncomeRecord)
		}
	}
	if status == model.PlatformIncomeStatusReceived && receivedDate == nil {
		return nil, fmt.Errorf("%w: received_date is required once the status is received", ErrInvalidPlatformIncomeRecord)
	}
	if request.RefundAmountCents < 0 || request.RefundAmountCents > request.AmountCents {
		return nil, fmt.Errorf("%w: refund_amount_cents must be between 0 and amount_cents", ErrInvalidPlatformIncomeRecord)
	}
	switch status {
	case model.PlatformIncomeStatusRefunding, model.PlatformIncomeStatusRefunded:
		if request.RefundAmountCents <= 0 {
			return nil, fmt.Errorf("%w: refund_amount_cents is required while refunding or refunded", ErrInvalidPlatformIncomeRecord)
		}
	default:
		if request.RefundAmountCents != 0 {
			return nil, fmt.Errorf("%w: refund_amount_cents must be 0 before a refund starts", ErrInvalidPlatformIncomeRecord)
		}
	}
	months := request.BillingPeriodMonths
	if months == 0 {
		months = 1
	}
	switch months {
	case 1, 3, 6, 12:
	default:
		return nil, fmt.Errorf("%w: billing_period_months must be 1, 3, 6 or 12", ErrInvalidPlatformIncomeRecord)
	}
	startDate, err := platformIncomeOptionalDate(request.SubscriptionStartDate)
	if err != nil {
		return nil, fmt.Errorf("%w: subscription_start_date %v", ErrInvalidPlatformIncomeRecord, err)
	}
	endDate, err := platformIncomeOptionalDate(request.SubscriptionEndDate)
	if err != nil {
		return nil, fmt.Errorf("%w: subscription_end_date %v", ErrInvalidPlatformIncomeRecord, err)
	}
	isSubscription := incomeType == model.PlatformIncomeTypeSubscriptionBasic || incomeType == model.PlatformIncomeTypeSubscriptionPremium
	if isSubscription {
		if startDate == nil || endDate == nil {
			return nil, fmt.Errorf("%w: subscription records require a subscription period", ErrInvalidPlatformIncomeRecord)
		}
		if string(*endDate) < string(*startDate) {
			return nil, fmt.Errorf("%w: subscription_end_date cannot precede subscription_start_date", ErrInvalidPlatformIncomeRecord)
		}
	} else if startDate != nil || endDate != nil {
		return nil, fmt.Errorf("%w: only subscription records carry a subscription period", ErrInvalidPlatformIncomeRecord)
	}
	note := strings.TrimSpace(request.Note)
	if utf8.RuneCountInString(note) > maxPlatformFinanceNoteRunes {
		return nil, fmt.Errorf("%w: note must contain at most %d characters", ErrInvalidPlatformIncomeRecord, maxPlatformFinanceNoteRunes)
	}
	for _, r := range note {
		if unicode.IsControl(r) || r == '<' || r == '>' {
			return nil, fmt.Errorf("%w: note cannot contain control characters or HTML markup", ErrInvalidPlatformIncomeRecord)
		}
	}
	key, err := uuid.Parse(strings.TrimSpace(request.IdempotencyKey))
	if err != nil || key.String() == uuid.Nil.String() {
		return nil, fmt.Errorf("%w: idempotency_key must be a non-zero UUID", ErrInvalidPlatformIncomeRecord)
	}
	return &model.PlatformIncomeRecord{
		MerchantID:            platformIncomeOptionalID(request.MerchantID),
		PackageID:             platformIncomeOptionalID(request.PackageID),
		IncomeType:            incomeType,
		AmountCents:           request.AmountCents,
		Currency:              "CNY",
		OccurredDate:          model.PlatformExpenseDate(occurredDay.Format("2006-01-02")),
		ReceivedDate:          receivedDate,
		Status:                status,
		RefundAmountCents:     request.RefundAmountCents,
		BillingPeriodMonths:   months,
		SubscriptionStartDate: startDate,
		SubscriptionEndDate:   endDate,
		OrderID:               platformIncomeOptionalID(request.OrderID),
		Note:                  note,
		CreatedBy:             actorID,
		IdempotencyKey:        key.String(),
	}, nil
}

func samePlatformIncomeRecordPayload(existing, candidate model.PlatformIncomeRecord) bool {
	sameDate := func(a, b *model.PlatformExpenseDate) bool {
		if a == nil || b == nil {
			return a == nil && b == nil
		}
		return *a == *b
	}
	sameID := func(a, b *uint) bool {
		if a == nil || b == nil {
			return a == nil && b == nil
		}
		return *a == *b
	}
	return sameID(existing.MerchantID, candidate.MerchantID) && sameID(existing.PackageID, candidate.PackageID) &&
		existing.IncomeType == candidate.IncomeType && existing.AmountCents == candidate.AmountCents &&
		existing.Currency == candidate.Currency && existing.OccurredDate == candidate.OccurredDate &&
		sameDate(existing.ReceivedDate, candidate.ReceivedDate) && existing.Status == candidate.Status &&
		existing.RefundAmountCents == candidate.RefundAmountCents &&
		existing.BillingPeriodMonths == candidate.BillingPeriodMonths &&
		sameDate(existing.SubscriptionStartDate, candidate.SubscriptionStartDate) &&
		sameDate(existing.SubscriptionEndDate, candidate.SubscriptionEndDate) &&
		sameID(existing.OrderID, candidate.OrderID) && existing.Note == candidate.Note &&
		existing.CreatedBy == candidate.CreatedBy
}

var platformIncomeAllowedTransitions = map[string][]string{
	model.PlatformIncomeStatusOrdered:        {model.PlatformIncomeStatusConfirmed, model.PlatformIncomeStatusPendingReceipt},
	model.PlatformIncomeStatusConfirmed:      {model.PlatformIncomeStatusReceived, model.PlatformIncomeStatusPendingReceipt},
	model.PlatformIncomeStatusPendingReceipt: {model.PlatformIncomeStatusReceived},
	model.PlatformIncomeStatusReceived:       {model.PlatformIncomeStatusRefunding},
	model.PlatformIncomeStatusRefunding:      {model.PlatformIncomeStatusRefunded, model.PlatformIncomeStatusReceived},
	model.PlatformIncomeStatusRefunded:       {},
}

// UpdatePlatformIncomeRecord moves one record along its status flow or
// cancels it. Amount, type, merchant and dates never change here.
func (s *AdminService) UpdatePlatformIncomeRecord(id uint, request dto.UpdatePlatformIncomeRecordRequest, actor AuditActor) (*model.PlatformIncomeRecord, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("platform cost database is unavailable")
	}
	if id == 0 {
		return nil, fmt.Errorf("%w: income record id is required", ErrInvalidPlatformIncomeRecord)
	}
	if actor.ID == 0 || strings.TrimSpace(actor.Type) != "super_admin" {
		return nil, ErrInvalidPlatformIncomeRecord
	}
	var updated model.PlatformIncomeRecord
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var record model.PlatformIncomeRecord
		if err := tx.First(&record, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPlatformIncomeRecordNotFound
			}
			return fmt.Errorf("load platform income record: %w", err)
		}
		next, err := applyPlatformIncomeUpdate(&record, request)
		if err != nil {
			return err
		}
		if err := tx.Save(&record).Error; err != nil {
			return err
		}
		if err := AppendAuditTx(tx, AuditEvent{
			Actor: actor,
			Input: AuditRecordInput{Action: "platform_income_record_update", TargetType: "platform_income_record", TargetID: record.ID},
		}); err != nil {
			return err
		}
		updated = record
		_ = next
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func applyPlatformIncomeUpdate(record *model.PlatformIncomeRecord, request dto.UpdatePlatformIncomeRecordRequest) (string, error) {
	cancel := request.Cancel != nil && *request.Cancel
	hasStatus := request.Status != nil && strings.TrimSpace(*request.Status) != ""
	if cancel && hasStatus {
		return "", fmt.Errorf("%w: cancel and status cannot be set together", ErrInvalidPlatformIncomeRecord)
	}
	if !cancel && !hasStatus && request.Note == nil {
		return "", fmt.Errorf("%w: nothing to update", ErrInvalidPlatformIncomeRecord)
	}
	if request.Note != nil {
		note := strings.TrimSpace(*request.Note)
		if utf8.RuneCountInString(note) > maxPlatformFinanceNoteRunes {
			return "", fmt.Errorf("%w: note must contain at most %d characters", ErrInvalidPlatformIncomeRecord, maxPlatformFinanceNoteRunes)
		}
		for _, r := range note {
			if unicode.IsControl(r) || r == '<' || r == '>' {
				return "", fmt.Errorf("%w: note cannot contain control characters or HTML markup", ErrInvalidPlatformIncomeRecord)
			}
		}
		record.Note = note
	}
	if cancel {
		if record.IsCancelled {
			return "", fmt.Errorf("%w: record is already cancelled", ErrInvalidPlatformIncomeRecord)
		}
		switch record.Status {
		case model.PlatformIncomeStatusOrdered, model.PlatformIncomeStatusConfirmed, model.PlatformIncomeStatusPendingReceipt:
		default:
			return "", fmt.Errorf("%w: only ordered, confirmed or pending-receipt records can be cancelled", ErrPlatformIncomeTransitionNotAllowed)
		}
		record.IsCancelled = true
		return record.Status, nil
	}
	if record.IsCancelled {
		return "", fmt.Errorf("%w: a cancelled record accepts no status change", ErrPlatformIncomeTransitionNotAllowed)
	}
	if !hasStatus {
		return record.Status, nil
	}
	next := strings.TrimSpace(*request.Status)
	allowed, ok := platformIncomeAllowedTransitions[record.Status]
	if !ok {
		return "", fmt.Errorf("%w: unknown current status %q", ErrInvalidPlatformIncomeRecord, record.Status)
	}
	allowedNext := false
	for _, candidate := range allowed {
		if candidate == next {
			allowedNext = true
			break
		}
	}
	if !allowedNext {
		return "", fmt.Errorf("%w: %s cannot move to %s", ErrPlatformIncomeTransitionNotAllowed, record.Status, next)
	}
	switch next {
	case model.PlatformIncomeStatusReceived:
		receivedDate, err := platformIncomeOptionalDate(request.ReceivedDate)
		if err != nil {
			return "", fmt.Errorf("%w: received_date %v", ErrInvalidPlatformIncomeRecord, err)
		}
		if receivedDate == nil {
			if record.ReceivedDate != nil {
				receivedDate = record.ReceivedDate
			} else {
				today := model.PlatformExpenseDate(time.Now().In(time.Local).Format("2006-01-02"))
				receivedDate = &today
			}
		}
		if string(*receivedDate) > time.Now().In(time.Local).Format("2006-01-02") {
			return "", fmt.Errorf("%w: received_date cannot be in the future", ErrInvalidPlatformIncomeRecord)
		}
		if string(*receivedDate) < string(record.OccurredDate) {
			return "", fmt.Errorf("%w: received_date cannot precede occurred_date", ErrInvalidPlatformIncomeRecord)
		}
		record.ReceivedDate = receivedDate
		record.RefundAmountCents = 0
	case model.PlatformIncomeStatusRefunding, model.PlatformIncomeStatusRefunded:
		if request.RefundAmountCents == nil || *request.RefundAmountCents <= 0 || *request.RefundAmountCents > record.AmountCents {
			return "", fmt.Errorf("%w: refund_amount_cents must be between 1 and amount_cents for this transition", ErrInvalidPlatformIncomeRecord)
		}
		record.RefundAmountCents = *request.RefundAmountCents
	default:
		if request.RefundAmountCents != nil && *request.RefundAmountCents != 0 {
			return "", fmt.Errorf("%w: refund_amount_cents must be 0 for this transition", ErrInvalidPlatformIncomeRecord)
		}
	}
	record.Status = next
	return next, nil
}

type PlatformIncomeStatusTotal struct {
	Count       int64 `json:"count"`
	AmountCents int64 `json:"amount_cents"`
}

type PlatformIncomeListResult struct {
	Rows           []model.PlatformIncomeRecord         `json:"rows"`
	TotalsByStatus map[string]PlatformIncomeStatusTotal `json:"totals_by_status"`
	Truncated      bool                                 `json:"truncated"`
}

// ListPlatformIncomeRecords lists income records, optionally filtered by
// status and occurred month (YYYY-MM). Totals cover the same filter.
func (s *AdminService) ListPlatformIncomeRecords(status, month string) (*PlatformIncomeListResult, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("platform cost database is unavailable")
	}
	start, end, err := platformFinanceMonthRange(month)
	if err != nil {
		return nil, err
	}
	status = strings.TrimSpace(status)
	if status != "" {
		switch status {
		case model.PlatformIncomeStatusOrdered, model.PlatformIncomeStatusConfirmed, model.PlatformIncomeStatusReceived,
			model.PlatformIncomeStatusPendingReceipt, model.PlatformIncomeStatusRefunded, model.PlatformIncomeStatusRefunding:
		default:
			return nil, fmt.Errorf("%w: unsupported status", ErrInvalidPlatformIncomeRecord)
		}
	}
	applyFilters := func(query *gorm.DB) *gorm.DB {
		if start != nil {
			query = query.Where("occurred_date >= ? AND occurred_date < ?", start.Format("2006-01-02"), end.Format("2006-01-02"))
		}
		if status != "" {
			query = query.Where("status = ?", status)
		}
		return query
	}
	var totals []struct {
		Status      string `gorm:"column:status"`
		Count       int64  `gorm:"column:row_count"`
		AmountCents string `gorm:"column:total_amount"`
	}
	if err := applyFilters(s.db.Model(&model.PlatformIncomeRecord{})).
		Select(`status, COUNT(*) AS row_count, CAST(COALESCE(SUM(amount_cents), 0) AS CHAR) AS total_amount`).
		Group("status").Scan(&totals).Error; err != nil {
		return nil, fmt.Errorf("summarize platform income records: %w", err)
	}
	rows := make([]model.PlatformIncomeRecord, 0)
	if err := applyFilters(s.db.Model(&model.PlatformIncomeRecord{})).
		Order("occurred_date DESC, id DESC").Limit(platformFinanceListLimit).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list platform income records: %w", err)
	}
	result := &PlatformIncomeListResult{Rows: rows, TotalsByStatus: make(map[string]PlatformIncomeStatusTotal, len(totals))}
	var totalCount int64
	for _, total := range totals {
		amount, err := parseAggregateCents(total.AmountCents)
		if err != nil {
			return nil, fmt.Errorf("income total exceeds supported integer range: %w", err)
		}
		result.TotalsByStatus[total.Status] = PlatformIncomeStatusTotal{Count: total.Count, AmountCents: amount}
		totalCount += total.Count
	}
	result.Truncated = totalCount > int64(len(rows))
	return result, nil
}

// platformFinanceMonthRange parses an optional YYYY-MM filter into a
// half-open [month-start, next-month-start) range in local time.
func platformFinanceMonthRange(month string) (*time.Time, *time.Time, error) {
	month = strings.TrimSpace(month)
	if month == "" {
		return nil, nil, nil
	}
	parsed, err := time.ParseInLocation("2006-01", month, time.Local)
	if err != nil || parsed.Format("2006-01") != month {
		return nil, nil, fmt.Errorf("%w: month must use YYYY-MM", ErrInvalidPlatformFinancePeriod)
	}
	start := time.Date(parsed.Year(), parsed.Month(), 1, 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 1, 0)
	return &start, &end, nil
}

type PlatformMRRRow struct {
	ID              uint   `json:"id"`
	MerchantID      *uint  `json:"merchant_id"`
	IncomeType      string `json:"income_type"`
	MonthlyCents    int64  `json:"monthly_cents"`
	SubscriptionEnd string `json:"subscription_end_date"`
}

type PlatformMRRResult struct {
	Month                   string           `json:"month"`
	MRRCents                int64            `json:"mrr_cents"`
	IncludedCount           int64            `json:"included_count"`
	Included                []PlatformMRRRow `json:"included"`
	ExcludedCancelled       int64            `json:"excluded_cancelled"`
	ExcludedRefunded        int64            `json:"excluded_refunded"`
	ExcludedOutOfPeriod     int64            `json:"excluded_out_of_period"`
	ExcludedWrongStatus     int64            `json:"excluded_wrong_status"`
	ExcludedNonSubscription int64            `json:"excluded_non_subscription"`
	Formula                 string           `json:"formula"`
	Note                    string           `json:"note"`
}

// PlatformMRR computes monthly recurring revenue for one month: the sum of
// the amortised monthly amounts of confirmed/received, uncancelled
// subscription records whose subscription period covers the month. Annual
// payments divide by 12, floored to the fen. Cancelled and refunded records
// are excluded. The formula and the exclusion counts are returned with the
// number so the page can state the basis.
func (s *AdminService) PlatformMRR(month string) (*PlatformMRRResult, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("platform cost database is unavailable")
	}
	start, end, err := platformFinanceMonthRange(month)
	if err != nil {
		return nil, err
	}
	if start == nil {
		now := time.Now().In(time.Local)
		monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
		start = &monthStart
		endMonth := monthStart.AddDate(0, 1, 0)
		end = &endMonth
		month = monthStart.Format("2006-01")
	}
	monthStartText := start.Format("2006-01-02")
	monthEndText := end.AddDate(0, 0, -1).Format("2006-01-02")
	var records []model.PlatformIncomeRecord
	if err := s.db.Model(&model.PlatformIncomeRecord{}).Order("id ASC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("load platform income records for MRR: %w", err)
	}
	result := &PlatformMRRResult{
		Month:    month,
		Included: make([]PlatformMRRRow, 0),
		Formula:  "MRR = 当期有效订阅的月化金额之和；年付按十二分之一摊（向下取整到分），已取消与已退款剔除",
		Note:     "只统计已确认或已收款的订阅收入；待收款、已下单不计入；非订阅收入不计入；订阅期须覆盖当月。",
	}
	for _, record := range records {
		isSubscription := record.IncomeType == model.PlatformIncomeTypeSubscriptionBasic || record.IncomeType == model.PlatformIncomeTypeSubscriptionPremium
		if !isSubscription {
			result.ExcludedNonSubscription++
			continue
		}
		if record.IsCancelled {
			result.ExcludedCancelled++
			continue
		}
		if record.Status == model.PlatformIncomeStatusRefunded {
			result.ExcludedRefunded++
			continue
		}
		if record.Status != model.PlatformIncomeStatusConfirmed && record.Status != model.PlatformIncomeStatusReceived {
			result.ExcludedWrongStatus++
			continue
		}
		periodStart := string(record.OccurredDate)
		if record.SubscriptionStartDate != nil {
			periodStart = string(*record.SubscriptionStartDate)
		}
		periodEnd := string(record.OccurredDate)
		if record.SubscriptionEndDate != nil {
			periodEnd = string(*record.SubscriptionEndDate)
		}
		if periodStart > monthEndText || periodEnd < monthStartText {
			result.ExcludedOutOfPeriod++
			continue
		}
		monthly := model.PlatformIncomeMonthlyAmountCents(record)
		result.MRRCents += monthly
		result.IncludedCount++
		result.Included = append(result.Included, PlatformMRRRow{
			ID:              record.ID,
			MerchantID:      record.MerchantID,
			IncomeType:      record.IncomeType,
			MonthlyCents:    monthly,
			SubscriptionEnd: periodEnd,
		})
	}
	return result, nil
}

type PlatformBreakevenResult struct {
	Month                        string `json:"month"`
	Assumption                   bool   `json:"assumption"`
	FixedCashCostCents           int64  `json:"fixed_cash_cost_cents"`
	FixedCostSource              string `json:"fixed_cost_source"`
	TimeLogEconomicCents         int64  `json:"time_log_economic_cents"`
	MerchantMonthlyRevenueCents  *int64 `json:"merchant_monthly_revenue_cents"`
	MerchantMonthlyVariableCents *int64 `json:"merchant_monthly_variable_cents"`
	ContributionMarginCents      *int64 `json:"contribution_margin_cents"`
	BreakevenMerchants           *int64 `json:"breakeven_merchants"`
	CanBreakEven                 *bool  `json:"can_break_even"`
	ParamsMissing                bool   `json:"params_missing"`
	Formula                      string `json:"formula"`
	Note                         string `json:"note"`
}

// PlatformBreakeven answers how many paying merchants cover the month's
// fixed cash cost. The fixed cost comes from recorded expenses
// (cost_type=固定, cash cost) less reversals dated in the month; the two
// per-merchant figures are caller-supplied assumptions, never stored and
// never written into real income or cost records. When the contribution
// margin is zero or negative, adding merchants cannot break even.
func (s *AdminService) PlatformBreakeven(month string, merchantMonthlyRevenueCents, merchantMonthlyVariableCents *int64) (*PlatformBreakevenResult, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("platform cost database is unavailable")
	}
	start, end, err := platformFinanceMonthRange(month)
	if err != nil {
		return nil, err
	}
	if start == nil {
		now := time.Now().In(time.Local)
		monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
		start = &monthStart
		endMonth := monthStart.AddDate(0, 1, 0)
		end = &endMonth
		month = monthStart.Format("2006-01")
	}
	startText := start.Format("2006-01-02")
	endText := end.Format("2006-01-02")
	var fixed struct {
		Total string `gorm:"column:fixed_total"`
	}
	if err := s.db.Model(&model.PlatformExpense{}).
		Select(`CAST(COALESCE(SUM(amount_cents), 0) AS CHAR) AS fixed_total`).
		Where("cost_type = ? AND is_cash_cost = ? AND expense_date >= ? AND expense_date < ?", model.PlatformExpenseCostTypeFixed, true, startText, endText).
		Scan(&fixed).Error; err != nil {
		return nil, fmt.Errorf("sum fixed platform expenses: %w", err)
	}
	fixedTotal, err := parseAggregateCents(fixed.Total)
	if err != nil {
		return nil, fmt.Errorf("fixed expense total exceeds supported integer range: %w", err)
	}
	var reversals struct {
		Total string `gorm:"column:reversal_total"`
	}
	if err := s.db.Table("platform_expense_reversals AS reversal").
		Select(`CAST(COALESCE(SUM(reversal.amount_cents), 0) AS CHAR) AS reversal_total`).
		Where("reversal.reversal_date >= ? AND reversal.reversal_date < ?", startText, endText).
		Scan(&reversals).Error; err != nil {
		return nil, fmt.Errorf("sum platform expense reversals: %w", err)
	}
	reversalTotal, err := parseAggregateCents(reversals.Total)
	if err != nil {
		return nil, fmt.Errorf("reversal total exceeds supported integer range: %w", err)
	}
	fixedCash, err := subtractPlatformExpenseCents(fixedTotal, reversalTotal)
	if err != nil {
		return nil, fmt.Errorf("fixed cash cost exceeds supported integer range: %w", err)
	}
	var timeLogs struct {
		Total string `gorm:"column:time_total"`
	}
	if err := s.db.Model(&model.PlatformTimeLog{}).
		Select(`CAST(COALESCE(SUM(amount_cents), 0) AS CHAR) AS time_total`).
		Where("log_date >= ? AND log_date < ?", startText, endText).
		Scan(&timeLogs).Error; err != nil {
		return nil, fmt.Errorf("sum platform time log amounts: %w", err)
	}
	timeTotal, err := parseAggregateCents(timeLogs.Total)
	if err != nil {
		return nil, fmt.Errorf("time log total exceeds supported integer range: %w", err)
	}
	result := &PlatformBreakevenResult{
		Month:                month,
		Assumption:           true,
		FixedCashCostCents:   fixedCash,
		FixedCostSource:      "当月 cost_type=固定且现金成本的已登记费用合计，减当月冲正（按冲正日期归属）；v14 前未分类费用不计入",
		TimeLogEconomicCents: timeTotal,
		Formula:              "保本商家数 = 月固定成本 ÷ 单商家贡献毛利；单商家贡献毛利 = 单商家月均收入 − 单商家月变动成本",
		Note:                 "单商家月均收入与单商家月变动成本均为假设（调用时传入），不写入真实收入或成本记录；时间成本只计经济成本，不计入现金保本口径。",
	}
	if merchantMonthlyRevenueCents == nil || merchantMonthlyVariableCents == nil {
		result.ParamsMissing = true
		result.Note = "缺参数：请提供单商家月均收入与单商家月变动成本（假设）后再测算；未给参数时不输出保本结论。"
		return result, nil
	}
	if *merchantMonthlyRevenueCents < 0 || *merchantMonthlyVariableCents < 0 {
		return nil, fmt.Errorf("%w: per-merchant figures cannot be negative", ErrInvalidPlatformFinancePeriod)
	}
	revenue := *merchantMonthlyRevenueCents
	variable := *merchantMonthlyVariableCents
	result.MerchantMonthlyRevenueCents = &revenue
	result.MerchantMonthlyVariableCents = &variable
	margin := revenue - variable
	result.ContributionMarginCents = &margin
	canBreakEven := margin > 0
	result.CanBreakEven = &canBreakEven
	if margin <= 0 {
		result.Note = "增加商家数无法保本：单商家贡献毛利小于等于零，每多一个商家反而扩大亏损。"
		return result, nil
	}
	if fixedCash <= 0 {
		zero := int64(0)
		result.BreakevenMerchants = &zero
		result.Note = "当月固定现金成本为零（或冲正后为零），已有收入即可覆盖固定成本，保本商家数为 0。"
		return result, nil
	}
	breakeven := (fixedCash + margin - 1) / margin
	result.BreakevenMerchants = &breakeven
	return result, nil
}

type PlatformExpenseStatRow struct {
	Month         string `json:"month"`
	Category      string `json:"category"`
	CostType      string `json:"cost_type"`
	ExpenseCount  int64  `json:"expense_count"`
	CashCents     int64  `json:"cash_cents"`
	EconomicCents int64  `json:"economic_cents"`
	TotalCents    int64  `json:"total_cents"`
}

type PlatformExpenseStatsResult struct {
	StartMonth           string                   `json:"start_month"`
	EndMonth             string                   `json:"end_month"`
	Rows                 []PlatformExpenseStatRow `json:"rows"`
	ReversalCentsByMonth map[string]int64         `json:"reversal_cents_by_month"`
	Note                 string                   `json:"note"`
}

// PlatformExpenseStats groups recorded expenses by month, category and
// cost type. Cash and economic columns overlap by design: one record can be
// both a cash and an economic cost. Reversals are reported per month by
// reversal date and are not netted into the category rows.
func (s *AdminService) PlatformExpenseStats(startMonth, endMonth string) (*PlatformExpenseStatsResult, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("platform cost database is unavailable")
	}
	start, err := parsePlatformFinanceMonth(startMonth)
	if err != nil {
		return nil, err
	}
	end, err := parsePlatformFinanceMonth(endMonth)
	if err != nil {
		return nil, err
	}
	if start == nil || end == nil {
		return nil, fmt.Errorf("%w: start_month and end_month are required", ErrInvalidPlatformFinancePeriod)
	}
	if end.Before(*start) {
		return nil, fmt.Errorf("%w: end_month precedes start_month", ErrInvalidPlatformFinancePeriod)
	}
	if monthsBetweenPlatformFinance(*start, *end) > 12 {
		return nil, fmt.Errorf("%w: period cannot exceed 12 months", ErrInvalidPlatformFinancePeriod)
	}
	now := time.Now().In(time.Local)
	currentMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	if end.After(currentMonth) {
		return nil, fmt.Errorf("%w: end_month cannot be in the future", ErrInvalidPlatformFinancePeriod)
	}
	startText := start.Format("2006-01-02")
	endExclusive := end.AddDate(0, 1, 0)
	endText := endExclusive.Format("2006-01-02")
	var rows []struct {
		Month         string `gorm:"column:stat_month"`
		Category      string `gorm:"column:category"`
		CostType      string `gorm:"column:cost_type"`
		ExpenseCount  int64  `gorm:"column:expense_count"`
		CashCents     string `gorm:"column:cash_cents"`
		EconomicCents string `gorm:"column:economic_cents"`
		TotalCents    string `gorm:"column:total_cents"`
	}
	if err := s.db.Model(&model.PlatformExpense{}).
		Select(`SUBSTR(expense_date, 1, 7) AS stat_month, category, cost_type, COUNT(*) AS expense_count,
			CAST(COALESCE(SUM(CASE WHEN is_cash_cost THEN amount_cents ELSE 0 END), 0) AS CHAR) AS cash_cents,
			CAST(COALESCE(SUM(CASE WHEN is_economic_cost THEN amount_cents ELSE 0 END), 0) AS CHAR) AS economic_cents,
			CAST(COALESCE(SUM(amount_cents), 0) AS CHAR) AS total_cents`).
		Where("expense_date >= ? AND expense_date < ? AND currency = ?", startText, endText, "CNY").
		Group("stat_month, category, cost_type").Order("stat_month ASC, category ASC, cost_type ASC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("summarize platform expenses by category and month: %w", err)
	}
	var reversalRows []struct {
		Month string `gorm:"column:stat_month"`
		Total string `gorm:"column:reversal_total"`
	}
	if err := s.db.Table("platform_expense_reversals AS reversal").
		Select(`SUBSTR(reversal.reversal_date, 1, 7) AS stat_month, CAST(COALESCE(SUM(reversal.amount_cents), 0) AS CHAR) AS reversal_total`).
		Where("reversal.reversal_date >= ? AND reversal.reversal_date < ?", startText, endText).
		Group("stat_month").Scan(&reversalRows).Error; err != nil {
		return nil, fmt.Errorf("summarize platform expense reversals by month: %w", err)
	}
	result := &PlatformExpenseStatsResult{
		StartMonth:           startMonth,
		EndMonth:             endMonth,
		Rows:                 make([]PlatformExpenseStatRow, 0, len(rows)),
		ReversalCentsByMonth: make(map[string]int64, len(reversalRows)),
		Note:                 "现金与经济两列可重叠：一条费用可同时是现金成本与经济成本；cost_type 为空表示 v14 前未分类记录；冲正按冲正日期归属到月，未并入分类行。",
	}
	for _, row := range rows {
		cash, err := parseAggregateCents(row.CashCents)
		if err != nil {
			return nil, fmt.Errorf("expense cash total exceeds supported integer range: %w", err)
		}
		economic, err := parseAggregateCents(row.EconomicCents)
		if err != nil {
			return nil, fmt.Errorf("expense economic total exceeds supported integer range: %w", err)
		}
		total, err := parseAggregateCents(row.TotalCents)
		if err != nil {
			return nil, fmt.Errorf("expense total exceeds supported integer range: %w", err)
		}
		result.Rows = append(result.Rows, PlatformExpenseStatRow{
			Month:         row.Month,
			Category:      row.Category,
			CostType:      row.CostType,
			ExpenseCount:  row.ExpenseCount,
			CashCents:     cash,
			EconomicCents: economic,
			TotalCents:    total,
		})
	}
	for _, row := range reversalRows {
		total, err := parseAggregateCents(row.Total)
		if err != nil {
			return nil, fmt.Errorf("reversal total exceeds supported integer range: %w", err)
		}
		result.ReversalCentsByMonth[row.Month] = total
	}
	return result, nil
}

func parsePlatformFinanceMonth(month string) (*time.Time, error) {
	month = strings.TrimSpace(month)
	if month == "" {
		return nil, nil
	}
	parsed, err := time.ParseInLocation("2006-01", month, time.Local)
	if err != nil || parsed.Format("2006-01") != month {
		return nil, fmt.Errorf("%w: month must use YYYY-MM", ErrInvalidPlatformFinancePeriod)
	}
	start := time.Date(parsed.Year(), parsed.Month(), 1, 0, 0, 0, 0, time.Local)
	return &start, nil
}

func monthsBetweenPlatformFinance(start, end time.Time) int {
	return (end.Year()-start.Year())*12 + int(end.Month()-start.Month())
}
