package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

func newSyntheticPlatformFinanceDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open synthetic SQLite database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get synthetic SQLite database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&model.Merchant{}, &model.Order{}, &model.PaymentRecord{}, &model.RefundRecord{}, &model.MerchantSubscriptionManualReview{}, &model.MerchantSubscriptionManualReviewAudit{}, &model.PlatformExpense{}, &model.PlatformTimeLog{}, &model.PlatformIncomeRecord{}, &model.AuditLog{}); err != nil {
		t.Fatalf("migrate synthetic platform finance schema: %v", err)
	}
	if err := db.Exec(`CREATE TABLE platform_expense_reversals (
		id INTEGER PRIMARY KEY AUTOINCREMENT, created_at datetime NOT NULL, updated_at datetime NOT NULL,
		expense_id INTEGER NOT NULL REFERENCES platform_expenses(id) ON DELETE RESTRICT,
		amount_cents INTEGER NOT NULL CHECK (amount_cents > 0), reversal_date date NOT NULL,
		reason varchar(500) NOT NULL CHECK (LENGTH(TRIM(reason)) BETWEEN 1 AND 500),
		created_by INTEGER NOT NULL, idempotency_key char(36) NOT NULL UNIQUE
	)`).Error; err != nil {
		t.Fatalf("create isolated synthetic reversal table with explicit DDL: %v", err)
	}
	return db
}

func financeActor() AuditActor { return AuditActor{ID: 77, Type: "super_admin"} }

func validSyntheticTimeLogRequest(t *testing.T) dto.CreatePlatformTimeLogRequest {
	t.Helper()
	return dto.CreatePlatformTimeLogRequest{
		LogDate:                   "2026-11-20",
		WorkType:                  model.PlatformTimeLogWorkDevelopment,
		Content:                   "synthetic cost centre development",
		Hours:                     1.5,
		ReferenceHourlyValueCents: 10000,
		Module:                    "cost-centre",
		IdempotencyKey:            uuid.NewString(),
	}
}

func validSyntheticIncomeRequest(t *testing.T) dto.CreatePlatformIncomeRecordRequest {
	t.Helper()
	start, end := "2026-11-01", "2026-11-30"
	return dto.CreatePlatformIncomeRecordRequest{
		IncomeType:            model.PlatformIncomeTypeSubscriptionBasic,
		AmountCents:           19900,
		Currency:              "CNY",
		OccurredDate:          "2026-11-10",
		Status:                model.PlatformIncomeStatusConfirmed,
		BillingPeriodMonths:   1,
		SubscriptionStartDate: &start,
		SubscriptionEndDate:   &end,
		Note:                  "synthetic subscription income",
		IdempotencyKey:        uuid.NewString(),
	}
}

func createSyntheticFinanceExpense(t *testing.T, service *AdminService, category, costType string, amount int64, date string, cash, economic bool) *model.PlatformExpense {
	t.Helper()
	entry, created, err := service.CreatePlatformExpense(dto.CreatePlatformExpenseRequest{
		Category: category, AmountCents: amount, Currency: "CNY", ExpenseDate: date,
		Memo: "synthetic finance expense", IdempotencyKey: uuid.NewString(),
		CostType: costType, PaymentStatus: model.PlatformExpensePaymentStatusPaid,
		IsCashCost: &cash, IsEconomicCost: &economic,
	}, financeActor())
	if err != nil || !created {
		t.Fatalf("create synthetic finance expense: created=%v err=%v", created, err)
	}
	return entry
}

func TestPlatformTimeLogAmountIsIntegerMath(t *testing.T) {
	cases := []struct {
		hoursScaled, hourly, want int64
	}{
		{150, 10000, 15000},
		{33, 333, 110}, // 33*333=10989, half-up to 110 fen
		{1, 99, 1},     // 99+50=149, /100=1
		{2400, 10000, 240000},
	}
	for _, tc := range cases {
		if got := model.PlatformTimeLogAmountCents(tc.hoursScaled, tc.hourly); got != tc.want {
			t.Fatalf("amount(%d, %d)=%d, want %d", tc.hoursScaled, tc.hourly, got, tc.want)
		}
	}
}

func TestCreatePlatformTimeLogComputesAmountAndIsIdempotent(t *testing.T) {
	db := newSyntheticPlatformFinanceDB(t)
	service := NewAdminService(&config.Config{}, db)
	request := validSyntheticTimeLogRequest(t)
	entry, created, err := service.CreatePlatformTimeLog(request, financeActor())
	if err != nil || !created {
		t.Fatalf("create time log: created=%v err=%v", created, err)
	}
	if entry.HoursScaled != 150 || entry.AmountCents != 15000 || entry.LogDate != "2026-11-20" {
		t.Fatalf("time log mismatch: %+v", entry)
	}
	replayed, created, err := service.CreatePlatformTimeLog(request, financeActor())
	if err != nil || created || replayed.ID != entry.ID {
		t.Fatalf("replay time log: created=%v id=%d want %d err=%v", created, replayed.ID, entry.ID, err)
	}
	conflict := request
	conflict.Hours = 2
	if _, _, err := service.CreatePlatformTimeLog(conflict, financeActor()); !errors.Is(err, ErrPlatformTimeLogIdempotencyConflict) {
		t.Fatalf("conflicting replay should fail, got %v", err)
	}
}

func TestCreatePlatformTimeLogValidation(t *testing.T) {
	db := newSyntheticPlatformFinanceDB(t)
	service := NewAdminService(&config.Config{}, db)
	base := validSyntheticTimeLogRequest(t)
	cases := []struct {
		name   string
		mutate func(*dto.CreatePlatformTimeLogRequest)
	}{
		{"bad work type", func(r *dto.CreatePlatformTimeLogRequest) { r.WorkType = "sleeping" }},
		{"future date", func(r *dto.CreatePlatformTimeLogRequest) { r.LogDate = "2999-01-01" }},
		{"bad date", func(r *dto.CreatePlatformTimeLogRequest) { r.LogDate = "2026-13-01" }},
		{"zero hours", func(r *dto.CreatePlatformTimeLogRequest) { r.Hours = 0 }},
		{"over 24 hours", func(r *dto.CreatePlatformTimeLogRequest) { r.Hours = 24.5 }},
		{"three decimals", func(r *dto.CreatePlatformTimeLogRequest) { r.Hours = 1.234 }},
		{"zero reference value", func(r *dto.CreatePlatformTimeLogRequest) { r.ReferenceHourlyValueCents = 0 }},
		{"empty content", func(r *dto.CreatePlatformTimeLogRequest) { r.Content = "  " }},
		{"content too long", func(r *dto.CreatePlatformTimeLogRequest) { r.Content = strings.Repeat("x", 501) }},
		{"amount rounds to zero", func(r *dto.CreatePlatformTimeLogRequest) { r.Hours = 0.01; r.ReferenceHourlyValueCents = 1 }},
		{"bad idempotency key", func(r *dto.CreatePlatformTimeLogRequest) { r.IdempotencyKey = "not-a-uuid" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := base
			request.IdempotencyKey = uuid.NewString()
			tc.mutate(&request)
			if _, _, err := service.CreatePlatformTimeLog(request, financeActor()); !errors.Is(err, ErrInvalidPlatformTimeLog) {
				t.Fatalf("expected invalid time log, got %v", err)
			}
		})
	}
	if _, _, err := service.CreatePlatformTimeLog(base, AuditActor{ID: 0, Type: "super_admin"}); !errors.Is(err, ErrInvalidPlatformTimeLog) {
		t.Fatalf("missing actor should fail, got %v", err)
	}
}

func TestListPlatformTimeLogsTotalsAndFilters(t *testing.T) {
	db := newSyntheticPlatformFinanceDB(t)
	service := NewAdminService(&config.Config{}, db)
	logs := []dto.CreatePlatformTimeLogRequest{
		{LogDate: "2026-11-05", WorkType: model.PlatformTimeLogWorkDevelopment, Content: "dev nov", Hours: 2, ReferenceHourlyValueCents: 10000, Module: "m1", IdempotencyKey: uuid.NewString()},
		{LogDate: "2026-11-06", WorkType: model.PlatformTimeLogWorkTesting, Content: "test nov", Hours: 1, ReferenceHourlyValueCents: 5000, Module: "m1", IdempotencyKey: uuid.NewString()},
		{LogDate: "2026-10-06", WorkType: model.PlatformTimeLogWorkDevelopment, Content: "dev oct", Hours: 3, ReferenceHourlyValueCents: 10000, Module: "m2", IdempotencyKey: uuid.NewString()},
	}
	for _, request := range logs {
		if _, _, err := service.CreatePlatformTimeLog(request, financeActor()); err != nil {
			t.Fatalf("create time log: %v", err)
		}
	}
	nov, err := service.ListPlatformTimeLogs("2026-11", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(nov.Rows) != 2 || nov.TotalHoursScaled != 300 || nov.TotalAmountCents != 25000 {
		t.Fatalf("november totals mismatch: rows=%d hours=%d amount=%d", len(nov.Rows), nov.TotalHoursScaled, nov.TotalAmountCents)
	}
	dev, err := service.ListPlatformTimeLogs("", model.PlatformTimeLogWorkDevelopment)
	if err != nil {
		t.Fatal(err)
	}
	if len(dev.Rows) != 2 || dev.TotalAmountCents != 50000 {
		t.Fatalf("development filter mismatch: rows=%d amount=%d", len(dev.Rows), dev.TotalAmountCents)
	}
	if _, err := service.ListPlatformTimeLogs("2026-13", ""); !errors.Is(err, ErrInvalidPlatformFinancePeriod) {
		t.Fatalf("bad month should fail, got %v", err)
	}
}

func TestCreatePlatformIncomeRecordAndStatusFlow(t *testing.T) {
	db := newSyntheticPlatformFinanceDB(t)
	service := NewAdminService(&config.Config{}, db)
	orderedRequest := validSyntheticIncomeRequest(t)
	orderedRequest.Status = model.PlatformIncomeStatusOrdered
	entry, created, err := service.CreatePlatformIncomeRecord(orderedRequest, financeActor())
	if err != nil || !created {
		t.Fatalf("create income record: created=%v err=%v", created, err)
	}
	if entry.Status != model.PlatformIncomeStatusOrdered || entry.BillingPeriodMonths != 1 {
		t.Fatalf("income record mismatch: %+v", entry)
	}
	// ordered -> received is not an allowed jump.
	if _, err := service.UpdatePlatformIncomeRecord(entry.ID, dto.UpdatePlatformIncomeRecordRequest{Status: strptr(model.PlatformIncomeStatusReceived)}, financeActor()); !errors.Is(err, ErrPlatformIncomeTransitionNotAllowed) {
		t.Fatalf("ordered->received should be rejected, got %v", err)
	}
	// ordered -> confirmed, then confirmed -> received.
	confirmed, err := service.UpdatePlatformIncomeRecord(entry.ID, dto.UpdatePlatformIncomeRecordRequest{Status: strptr(model.PlatformIncomeStatusConfirmed)}, financeActor())
	if err != nil {
		t.Fatalf("ordered->confirmed: %v", err)
	}
	if confirmed.Status != model.PlatformIncomeStatusConfirmed {
		t.Fatalf("confirmed record mismatch: %+v", confirmed)
	}
	// confirmed -> received, received_date defaults to today.
	received, err := service.UpdatePlatformIncomeRecord(entry.ID, dto.UpdatePlatformIncomeRecordRequest{Status: strptr(model.PlatformIncomeStatusReceived)}, financeActor())
	if err != nil {
		t.Fatalf("confirm->receive: %v", err)
	}
	if received.Status != model.PlatformIncomeStatusReceived || received.ReceivedDate == nil {
		t.Fatalf("received record mismatch: %+v", received)
	}
	// received -> refunding requires a refund amount within the total.
	if _, err := service.UpdatePlatformIncomeRecord(entry.ID, dto.UpdatePlatformIncomeRecordRequest{Status: strptr(model.PlatformIncomeStatusRefunding), RefundAmountCents: int64ptr(999999)}, financeActor()); !errors.Is(err, ErrInvalidPlatformIncomeRecord) {
		t.Fatalf("over-amount refund should fail, got %v", err)
	}
	refunding, err := service.UpdatePlatformIncomeRecord(entry.ID, dto.UpdatePlatformIncomeRecordRequest{Status: strptr(model.PlatformIncomeStatusRefunding), RefundAmountCents: int64ptr(5000)}, financeActor())
	if err != nil {
		t.Fatalf("receive->refunding: %v", err)
	}
	if refunding.RefundAmountCents != 5000 {
		t.Fatalf("refund amount mismatch: %+v", refunding)
	}
	refunded, err := service.UpdatePlatformIncomeRecord(entry.ID, dto.UpdatePlatformIncomeRecordRequest{Status: strptr(model.PlatformIncomeStatusRefunded), RefundAmountCents: int64ptr(5000)}, financeActor())
	if err != nil {
		t.Fatalf("refunding->refunded: %v", err)
	}
	if refunded.Status != model.PlatformIncomeStatusRefunded {
		t.Fatalf("refunded record mismatch: %+v", refunded)
	}
	// refunded is terminal.
	if _, err := service.UpdatePlatformIncomeRecord(entry.ID, dto.UpdatePlatformIncomeRecordRequest{Status: strptr(model.PlatformIncomeStatusReceived)}, financeActor()); !errors.Is(err, ErrPlatformIncomeTransitionNotAllowed) {
		t.Fatalf("refunded should be terminal, got %v", err)
	}
	// A received record cannot be cancelled.
	second, _, err := service.CreatePlatformIncomeRecord(validSyntheticIncomeRequest(t), financeActor())
	if err != nil {
		t.Fatalf("create second income record: %v", err)
	}
	if _, err := service.UpdatePlatformIncomeRecord(second.ID, dto.UpdatePlatformIncomeRecordRequest{Status: strptr(model.PlatformIncomeStatusReceived)}, financeActor()); err != nil {
		t.Fatalf("second record receive: %v", err)
	}
	if _, err := service.UpdatePlatformIncomeRecord(second.ID, dto.UpdatePlatformIncomeRecordRequest{Cancel: boolptr(true)}, financeActor()); !errors.Is(err, ErrPlatformIncomeTransitionNotAllowed) {
		t.Fatalf("cancelling a received record should fail, got %v", err)
	}
	// But an ordered record can be cancelled, and then accepts no status change.
	thirdRequest := validSyntheticIncomeRequest(t)
	thirdRequest.Status = model.PlatformIncomeStatusOrdered
	third, _, err := service.CreatePlatformIncomeRecord(thirdRequest, financeActor())
	if err != nil {
		t.Fatalf("create third income record: %v", err)
	}
	cancelled, err := service.UpdatePlatformIncomeRecord(third.ID, dto.UpdatePlatformIncomeRecordRequest{Cancel: boolptr(true)}, financeActor())
	if err != nil || !cancelled.IsCancelled {
		t.Fatalf("cancel ordered record: %v %+v", err, cancelled)
	}
	if _, err := service.UpdatePlatformIncomeRecord(third.ID, dto.UpdatePlatformIncomeRecordRequest{Status: strptr(model.PlatformIncomeStatusConfirmed)}, financeActor()); !errors.Is(err, ErrPlatformIncomeTransitionNotAllowed) {
		t.Fatalf("cancelled record should reject status change, got %v", err)
	}
	// Unknown record.
	if _, err := service.UpdatePlatformIncomeRecord(999999, dto.UpdatePlatformIncomeRecordRequest{Status: strptr(model.PlatformIncomeStatusConfirmed)}, financeActor()); !errors.Is(err, ErrPlatformIncomeRecordNotFound) {
		t.Fatalf("unknown record should fail, got %v", err)
	}
}

func TestCreatePlatformIncomeRecordValidation(t *testing.T) {
	db := newSyntheticPlatformFinanceDB(t)
	service := NewAdminService(&config.Config{}, db)
	base := validSyntheticIncomeRequest(t)
	start, end := "2026-11-01", "2026-11-30"
	cases := []struct {
		name   string
		mutate func(*dto.CreatePlatformIncomeRecordRequest)
	}{
		{"subscription without period", func(r *dto.CreatePlatformIncomeRecordRequest) {
			r.SubscriptionStartDate = nil
			r.SubscriptionEndDate = nil
		}},
		{"end before start", func(r *dto.CreatePlatformIncomeRecordRequest) {
			r.SubscriptionStartDate = &end
			r.SubscriptionEndDate = &start
		}},
		{"non-subscription with period", func(r *dto.CreatePlatformIncomeRecordRequest) { r.IncomeType = model.PlatformIncomeTypeAddonModule }},
		{"received without received date", func(r *dto.CreatePlatformIncomeRecordRequest) { r.Status = model.PlatformIncomeStatusReceived }},
		{"refund amount before refund", func(r *dto.CreatePlatformIncomeRecordRequest) { r.RefundAmountCents = 100 }},
		{"refunding without refund amount", func(r *dto.CreatePlatformIncomeRecordRequest) { r.Status = model.PlatformIncomeStatusRefunding }},
		{"refund over amount", func(r *dto.CreatePlatformIncomeRecordRequest) {
			r.Status = model.PlatformIncomeStatusRefunded
			r.RefundAmountCents = 999999
		}},
		{"bad billing period", func(r *dto.CreatePlatformIncomeRecordRequest) { r.BillingPeriodMonths = 2 }},
		{"future occurred date", func(r *dto.CreatePlatformIncomeRecordRequest) { r.OccurredDate = "2999-01-01" }},
		{"zero amount", func(r *dto.CreatePlatformIncomeRecordRequest) { r.AmountCents = 0 }},
		{"bad idempotency key", func(r *dto.CreatePlatformIncomeRecordRequest) { r.IdempotencyKey = "not-a-uuid" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := base
			request.IdempotencyKey = uuid.NewString()
			tc.mutate(&request)
			if _, _, err := service.CreatePlatformIncomeRecord(request, financeActor()); !errors.Is(err, ErrInvalidPlatformIncomeRecord) {
				t.Fatalf("expected invalid income record, got %v", err)
			}
		})
	}
	// Idempotent replay returns the same record; a conflicting payload fails.
	request := base
	entry, created, err := service.CreatePlatformIncomeRecord(request, financeActor())
	if err != nil || !created {
		t.Fatalf("create income record: %v", err)
	}
	replayed, created, err := service.CreatePlatformIncomeRecord(request, financeActor())
	if err != nil || created || replayed.ID != entry.ID {
		t.Fatalf("replay income record: created=%v id=%d want %d err=%v", created, replayed.ID, entry.ID, err)
	}
	conflict := request
	conflict.AmountCents = 1
	if _, _, err := service.CreatePlatformIncomeRecord(conflict, financeActor()); !errors.Is(err, ErrPlatformIncomeIdempotencyConflict) {
		t.Fatalf("conflicting replay should fail, got %v", err)
	}
}

func TestPlatformMRR(t *testing.T) {
	db := newSyntheticPlatformFinanceDB(t)
	service := NewAdminService(&config.Config{}, db)
	novStart, novEnd := "2026-11-01", "2026-11-30"
	decStart, decEnd := "2026-12-01", "2026-12-31"
	received := "2026-11-15"
	create := func(req dto.CreatePlatformIncomeRecordRequest) *model.PlatformIncomeRecord {
		t.Helper()
		entry, _, err := service.CreatePlatformIncomeRecord(req, financeActor())
		if err != nil {
			t.Fatalf("create MRR income record: %v", err)
		}
		return entry
	}
	// Included: monthly 19900, annual 238800/12=19900, annual 10000/12=833.
	create(dto.CreatePlatformIncomeRecordRequest{IncomeType: model.PlatformIncomeTypeSubscriptionBasic, AmountCents: 19900, Currency: "CNY", OccurredDate: "2026-11-10", Status: model.PlatformIncomeStatusConfirmed, BillingPeriodMonths: 1, SubscriptionStartDate: &novStart, SubscriptionEndDate: &novEnd, IdempotencyKey: uuid.NewString()})
	create(dto.CreatePlatformIncomeRecordRequest{IncomeType: model.PlatformIncomeTypeSubscriptionPremium, AmountCents: 238800, Currency: "CNY", OccurredDate: "2026-11-10", Status: model.PlatformIncomeStatusReceived, ReceivedDate: &received, BillingPeriodMonths: 12, SubscriptionStartDate: &novStart, SubscriptionEndDate: &decEnd, IdempotencyKey: uuid.NewString()})
	create(dto.CreatePlatformIncomeRecordRequest{IncomeType: model.PlatformIncomeTypeSubscriptionBasic, AmountCents: 10000, Currency: "CNY", OccurredDate: "2026-11-10", Status: model.PlatformIncomeStatusReceived, ReceivedDate: &received, BillingPeriodMonths: 12, SubscriptionStartDate: &novStart, SubscriptionEndDate: &novEnd, IdempotencyKey: uuid.NewString()})
	// Excluded: cancelled, refunded, ordered, pending receipt, December-only, non-subscription.
	cancelled := create(dto.CreatePlatformIncomeRecordRequest{IncomeType: model.PlatformIncomeTypeSubscriptionBasic, AmountCents: 19900, Currency: "CNY", OccurredDate: "2026-11-10", Status: model.PlatformIncomeStatusOrdered, BillingPeriodMonths: 1, SubscriptionStartDate: &novStart, SubscriptionEndDate: &novEnd, IdempotencyKey: uuid.NewString()})
	if _, err := service.UpdatePlatformIncomeRecord(cancelled.ID, dto.UpdatePlatformIncomeRecordRequest{Cancel: boolptr(true)}, financeActor()); err != nil {
		t.Fatalf("cancel MRR record: %v", err)
	}
	refunded := create(dto.CreatePlatformIncomeRecordRequest{IncomeType: model.PlatformIncomeTypeSubscriptionBasic, AmountCents: 19900, Currency: "CNY", OccurredDate: "2026-11-10", Status: model.PlatformIncomeStatusReceived, ReceivedDate: &received, BillingPeriodMonths: 1, SubscriptionStartDate: &novStart, SubscriptionEndDate: &novEnd, IdempotencyKey: uuid.NewString()})
	if _, err := service.UpdatePlatformIncomeRecord(refunded.ID, dto.UpdatePlatformIncomeRecordRequest{Status: strptr(model.PlatformIncomeStatusRefunding), RefundAmountCents: int64ptr(19900)}, financeActor()); err != nil {
		t.Fatalf("refund MRR record step 1: %v", err)
	}
	if _, err := service.UpdatePlatformIncomeRecord(refunded.ID, dto.UpdatePlatformIncomeRecordRequest{Status: strptr(model.PlatformIncomeStatusRefunded), RefundAmountCents: int64ptr(19900)}, financeActor()); err != nil {
		t.Fatalf("refund MRR record step 2: %v", err)
	}
	create(dto.CreatePlatformIncomeRecordRequest{IncomeType: model.PlatformIncomeTypeSubscriptionBasic, AmountCents: 19900, Currency: "CNY", OccurredDate: "2026-11-10", Status: model.PlatformIncomeStatusOrdered, BillingPeriodMonths: 1, SubscriptionStartDate: &novStart, SubscriptionEndDate: &novEnd, IdempotencyKey: uuid.NewString()})
	create(dto.CreatePlatformIncomeRecordRequest{IncomeType: model.PlatformIncomeTypeSubscriptionBasic, AmountCents: 19900, Currency: "CNY", OccurredDate: "2026-11-10", Status: model.PlatformIncomeStatusPendingReceipt, BillingPeriodMonths: 1, SubscriptionStartDate: &novStart, SubscriptionEndDate: &novEnd, IdempotencyKey: uuid.NewString()})
	create(dto.CreatePlatformIncomeRecordRequest{IncomeType: model.PlatformIncomeTypeSubscriptionBasic, AmountCents: 19900, Currency: "CNY", OccurredDate: "2026-11-10", Status: model.PlatformIncomeStatusConfirmed, BillingPeriodMonths: 1, SubscriptionStartDate: &decStart, SubscriptionEndDate: &decEnd, IdempotencyKey: uuid.NewString()})
	create(dto.CreatePlatformIncomeRecordRequest{IncomeType: model.PlatformIncomeTypeAddonModule, AmountCents: 50000, Currency: "CNY", OccurredDate: "2026-11-10", Status: model.PlatformIncomeStatusReceived, ReceivedDate: &received, BillingPeriodMonths: 1, IdempotencyKey: uuid.NewString()})

	result, err := service.PlatformMRR("2026-11")
	if err != nil {
		t.Fatal(err)
	}
	if result.MRRCents != 40633 || result.IncludedCount != 3 {
		t.Fatalf("MRR mismatch: mrr=%d included=%d, want 40633/3", result.MRRCents, result.IncludedCount)
	}
	if result.ExcludedCancelled != 1 || result.ExcludedRefunded != 1 || result.ExcludedOutOfPeriod != 1 || result.ExcludedWrongStatus != 2 || result.ExcludedNonSubscription != 1 {
		t.Fatalf("MRR exclusions mismatch: %+v", result)
	}
	if result.Formula == "" || result.Note == "" {
		t.Fatalf("MRR must carry its formula and basis note")
	}
	if _, err := service.PlatformMRR("2026-13"); !errors.Is(err, ErrInvalidPlatformFinancePeriod) {
		t.Fatalf("bad MRR month should fail, got %v", err)
	}
}

func TestPlatformBreakeven(t *testing.T) {
	db := newSyntheticPlatformFinanceDB(t)
	service := NewAdminService(&config.Config{}, db)
	// Fixed cash 100000 in November; variable, economic-only and October
	// rows must not enter the fixed cash cost; a November reversal of 10000
	// brings it to 90000. Time logs (20000 economic) are reported apart.
	createSyntheticFinanceExpense(t, service, model.PlatformExpenseCategoryCloudHosting, model.PlatformExpenseCostTypeFixed, 100000, "2026-11-10", true, true)
	createSyntheticFinanceExpense(t, service, model.PlatformExpenseCategoryAIServices, model.PlatformExpenseCostTypeVariable, 30000, "2026-11-11", true, true)
	createSyntheticFinanceExpense(t, service, model.PlatformExpenseCategoryEquipment, model.PlatformExpenseCostTypeFixed, 20000, "2026-11-12", false, true)
	fixed := createSyntheticFinanceExpense(t, service, model.PlatformExpenseCategoryCloudHosting, model.PlatformExpenseCostTypeFixed, 50000, "2026-10-10", true, true)
	_ = fixed
	if _, _, err := service.CreatePlatformExpenseReversal(mustFindFinanceExpense(t, db, 100000), dto.CreatePlatformExpenseReversalRequest{ReversalDate: "2026-11-15", AmountCents: 10000, Reason: "synthetic november reversal", IdempotencyKey: uuid.NewString()}, financeActor()); err != nil {
		t.Fatalf("create november reversal: %v", err)
	}
	if _, _, err := service.CreatePlatformTimeLog(dto.CreatePlatformTimeLogRequest{LogDate: "2026-11-16", WorkType: model.PlatformTimeLogWorkDevelopment, Content: "breakeven time", Hours: 2, ReferenceHourlyValueCents: 10000, Module: "m", IdempotencyKey: uuid.NewString()}, financeActor()); err != nil {
		t.Fatalf("create breakeven time log: %v", err)
	}

	result, err := service.PlatformBreakeven("2026-11", int64ptr(20000), int64ptr(5000))
	if err != nil {
		t.Fatal(err)
	}
	if result.FixedCashCostCents != 90000 || result.TimeLogEconomicCents != 20000 {
		t.Fatalf("breakeven fixed/time mismatch: fixed=%d time=%d", result.FixedCashCostCents, result.TimeLogEconomicCents)
	}
	if !result.Assumption || result.ContributionMarginCents == nil || *result.ContributionMarginCents != 15000 {
		t.Fatalf("breakeven margin mismatch: %+v", result)
	}
	if result.BreakevenMerchants == nil || *result.BreakevenMerchants != 6 {
		t.Fatalf("breakeven merchants mismatch: %+v", result)
	}
	if result.CanBreakEven == nil || !*result.CanBreakEven {
		t.Fatalf("breakeven flag mismatch: %+v", result)
	}

	// Zero or negative margin can never break even.
	flat, err := service.PlatformBreakeven("2026-11", int64ptr(5000), int64ptr(5000))
	if err != nil {
		t.Fatal(err)
	}
	if flat.CanBreakEven == nil || *flat.CanBreakEven || flat.BreakevenMerchants != nil || !strings.Contains(flat.Note, "无法保本") {
		t.Fatalf("zero margin should report cannot-break-even: %+v", flat)
	}
	// Missing assumptions report no conclusion.
	missing, err := service.PlatformBreakeven("2026-11", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !missing.ParamsMissing || missing.BreakevenMerchants != nil {
		t.Fatalf("missing params should report no conclusion: %+v", missing)
	}
	// A month with no fixed cash cost needs zero merchants.
	empty, err := service.PlatformBreakeven("2026-12", int64ptr(20000), int64ptr(5000))
	if err != nil {
		t.Fatal(err)
	}
	if empty.FixedCashCostCents != 0 || empty.BreakevenMerchants == nil || *empty.BreakevenMerchants != 0 {
		t.Fatalf("empty month breakeven mismatch: %+v", empty)
	}
	if _, err := service.PlatformBreakeven("2026-13", int64ptr(1), int64ptr(0)); !errors.Is(err, ErrInvalidPlatformFinancePeriod) {
		t.Fatalf("bad breakeven month should fail, got %v", err)
	}
}

func mustFindFinanceExpense(t *testing.T, db *gorm.DB, amount int64) uint {
	t.Helper()
	var expense model.PlatformExpense
	if err := db.Where("amount_cents = ?", amount).First(&expense).Error; err != nil {
		t.Fatalf("find synthetic expense %d: %v", amount, err)
	}
	return expense.ID
}

func TestPlatformExpenseStatsByCategoryAndMonth(t *testing.T) {
	db := newSyntheticPlatformFinanceDB(t)
	service := NewAdminService(&config.Config{}, db)
	createSyntheticFinanceExpense(t, service, model.PlatformExpenseCategoryCloudHosting, model.PlatformExpenseCostTypeFixed, 10000, "2026-11-10", true, true)
	createSyntheticFinanceExpense(t, service, model.PlatformExpenseCategoryAIServices, model.PlatformExpenseCostTypeVariable, 5000, "2026-11-11", true, false)
	createSyntheticFinanceExpense(t, service, model.PlatformExpenseCategoryCloudHosting, model.PlatformExpenseCostTypeFixed, 3000, "2026-11-12", false, true)
	createSyntheticFinanceExpense(t, service, model.PlatformExpenseCategoryCloudHosting, model.PlatformExpenseCostTypeFixed, 7000, "2026-10-10", true, true)
	cloudNov := mustFindFinanceExpense(t, db, 10000)
	if _, _, err := service.CreatePlatformExpenseReversal(cloudNov, dto.CreatePlatformExpenseReversalRequest{ReversalDate: "2026-11-15", AmountCents: 2000, Reason: "synthetic stats reversal", IdempotencyKey: uuid.NewString()}, financeActor()); err != nil {
		t.Fatalf("create stats reversal: %v", err)
	}

	stats, err := service.PlatformExpenseStats("2026-10", "2026-11")
	if err != nil {
		t.Fatal(err)
	}
	byKey := make(map[string]PlatformExpenseStatRow, len(stats.Rows))
	for _, row := range stats.Rows {
		byKey[row.Month+"|"+row.Category+"|"+row.CostType] = row
	}
	cloudFixed := byKey["2026-11|cloud_hosting|fixed"]
	if cloudFixed.ExpenseCount != 2 || cloudFixed.CashCents != 10000 || cloudFixed.EconomicCents != 13000 || cloudFixed.TotalCents != 13000 {
		t.Fatalf("november cloud fixed mismatch: %+v", cloudFixed)
	}
	aiVariable := byKey["2026-11|ai_services|variable"]
	if aiVariable.ExpenseCount != 1 || aiVariable.CashCents != 5000 || aiVariable.EconomicCents != 0 || aiVariable.TotalCents != 5000 {
		t.Fatalf("november ai variable mismatch: %+v", aiVariable)
	}
	if oct, ok := byKey["2026-10|cloud_hosting|fixed"]; !ok || oct.TotalCents != 7000 {
		t.Fatalf("october row mismatch: %+v", oct)
	}
	if stats.ReversalCentsByMonth["2026-11"] != 2000 {
		t.Fatalf("reversal by month mismatch: %v", stats.ReversalCentsByMonth)
	}
	if _, err := service.PlatformExpenseStats("2026-11", "2026-10"); !errors.Is(err, ErrInvalidPlatformFinancePeriod) {
		t.Fatalf("reversed range should fail, got %v", err)
	}
	if _, err := service.PlatformExpenseStats("2025-01", "2026-11"); !errors.Is(err, ErrInvalidPlatformFinancePeriod) {
		t.Fatalf("over-12-month range should fail, got %v", err)
	}
}

func TestCreatePlatformExpenseAcceptsNewCategoriesAndFields(t *testing.T) {
	db := newSyntheticPlatformFinanceDB(t)
	service := NewAdminService(&config.Config{}, db)
	today := time.Now().In(time.Local).Format("2006-01-02")
	cash, economic := true, true
	entry, created, err := service.CreatePlatformExpense(dto.CreatePlatformExpenseRequest{
		Category: model.PlatformExpenseCategoryAIServices, AmountCents: 4200, Currency: "CNY",
		ExpenseDate: today, Memo: "synthetic ai services", IdempotencyKey: uuid.NewString(),
		CostType: model.PlatformExpenseCostTypeVariable, Vendor: "TestVendor",
		PaymentStatus: model.PlatformExpensePaymentStatusPartial, IsRecurring: true,
		IsCashCost: &cash, IsEconomicCost: &economic,
	}, financeActor())
	if err != nil || !created {
		t.Fatalf("create ai_services expense: created=%v err=%v", created, err)
	}
	if entry.Vendor != "TestVendor" || entry.PaymentStatus != "partial" || !entry.IsRecurring || entry.CostType != "variable" {
		t.Fatalf("new expense fields mismatch: %+v", entry)
	}
	third, _, err := service.CreatePlatformExpense(dto.CreatePlatformExpenseRequest{
		Category: model.PlatformExpenseCategoryThirdPartyServices, AmountCents: 100, Currency: "CNY",
		ExpenseDate: today, Memo: "synthetic third party", IdempotencyKey: uuid.NewString(),
		CostType: model.PlatformExpenseCostTypeFixed,
	}, financeActor())
	if err != nil {
		t.Fatalf("create third_party_services expense: %v", err)
	}
	if third.CostType != "fixed" || !third.IsCashCost || !third.IsEconomicCost || third.PaymentStatus != "paid" {
		t.Fatalf("third-party expense defaults mismatch: %+v", third)
	}
	// Overview rows now carry the new fields.
	todayDay := time.Now().In(time.Local)
	snapshot, err := service.PlatformCostOverviewForPeriod(todayDay, todayDay)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.SelectedPeriod.ExpenseRows) != 2 {
		t.Fatalf("overview expense rows=%d, want 2", len(snapshot.SelectedPeriod.ExpenseRows))
	}
	for _, row := range snapshot.SelectedPeriod.ExpenseRows {
		if row.Category == model.PlatformExpenseCategoryAIServices && (row.Vendor != "TestVendor" || row.PaymentStatus != "partial") {
			t.Fatalf("overview row missing new fields: %+v", row)
		}
	}
	// Validation: bad cost type, bad payment status, neither cost flag.
	bad := func(mutate func(*dto.CreatePlatformExpenseRequest)) error {
		request := validSyntheticExpenseRequest(t)
		mutate(&request)
		_, _, err := service.CreatePlatformExpense(request, financeActor())
		return err
	}
	falseValue := false
	if err := bad(func(r *dto.CreatePlatformExpenseRequest) { r.CostType = "sometimes" }); !errors.Is(err, ErrInvalidPlatformExpense) {
		t.Fatalf("bad cost_type should fail, got %v", err)
	}
	if err := bad(func(r *dto.CreatePlatformExpenseRequest) { r.PaymentStatus = "later" }); !errors.Is(err, ErrInvalidPlatformExpense) {
		t.Fatalf("bad payment_status should fail, got %v", err)
	}
	if err := bad(func(r *dto.CreatePlatformExpenseRequest) { r.IsCashCost = &falseValue; r.IsEconomicCost = &falseValue }); !errors.Is(err, ErrInvalidPlatformExpense) {
		t.Fatalf("neither cost flag should fail, got %v", err)
	}
	if err := bad(func(r *dto.CreatePlatformExpenseRequest) { r.Category = "not_a_category" }); !errors.Is(err, ErrInvalidPlatformExpense) {
		t.Fatalf("bad category should fail, got %v", err)
	}
}

func strptr(value string) *string { return &value }

func int64ptr(value int64) *int64 { return &value }

func boolptr(value bool) *bool { return &value }
