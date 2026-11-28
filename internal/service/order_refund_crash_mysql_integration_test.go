package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

const (
	refundCrashChildEnv        = "CUE_REFUND_CRASH_CHILD"
	refundCrashOrderIDEnv      = "CUE_REFUND_CRASH_ORDER_ID"
	refundCrashMerchantEnv     = "CUE_REFUND_CRASH_MERCHANT_ID"
	refundCrashTableEnv        = "CUE_REFUND_CRASH_DISPATCH_TABLE"
	refundCrashIdempotency     = "synthetic-worker-crash-reservation-key"
	refundCrashDispatchExit    = 86
	refundCrashModeExit        = "crash"
	refundRetryModeExit        = "retry"
	refundProviderCrashMode    = "provider-outcome-crash"
	refundProviderOutcomeEnv   = "CUE_REFUND_PROVIDER_OUTCOME"
	refundProviderReferenceEnv = "CUE_REFUND_PROVIDER_REFERENCE"
	refundProviderAmountEnv    = "CUE_REFUND_PROVIDER_AMOUNT"
	refundProviderKeyEnv       = "CUE_REFUND_PROVIDER_IDEMPOTENCY_KEY"
	refundProviderReasonEnv    = "CUE_REFUND_PROVIDER_REASON"
)

type exitAfterSyntheticDispatchGateway struct {
	db            *gorm.DB
	dispatchTable string
}

func (*exitAfterSyntheticDispatchGateway) Enabled() bool { return true }

// Refund writes only to a test-owned table in the disposable MySQL database,
// then terminates the child test process before the provider call can return.
func (g *exitAfterSyntheticDispatchGateway) Refund(_ *model.Order, refundNo string, _ int64, _ string) (string, error) {
	query := "INSERT INTO `" + g.dispatchTable + "` (refund_no) VALUES (?)"
	if err := g.db.Exec(query, refundNo).Error; err != nil {
		fmt.Fprintf(os.Stderr, "synthetic dispatch recorder failed: %v\n", err)
		os.Exit(refundCrashDispatchExit + 1)
	}
	os.Exit(refundCrashDispatchExit)
	return "", nil
}

type recordingSyntheticRefundGateway struct {
	db            *gorm.DB
	dispatchTable string
}

func (*recordingSyntheticRefundGateway) Enabled() bool { return true }

func (g *recordingSyntheticRefundGateway) Refund(_ *model.Order, refundNo string, _ int64, _ string) (string, error) {
	query := "INSERT INTO `" + g.dispatchTable + "` (refund_no) VALUES (?)"
	if err := g.db.Exec(query, refundNo).Error; err != nil {
		return "", fmt.Errorf("record synthetic provider dispatch: %w", err)
	}
	return "", fmt.Errorf("synthetic provider transport returned an unknown outcome")
}

type exitAfterSyntheticProviderOutcomeGateway struct {
	db                *gorm.DB
	ledgerTable       string
	outcome           string
	providerReference string
}

func (*exitAfterSyntheticProviderOutcomeGateway) Enabled() bool { return true }

// Refund commits a synthetic provider-side outcome, then kills the worker before
// the application can receive that result or finalize its local reservation.
func (g *exitAfterSyntheticProviderOutcomeGateway) Refund(_ *model.Order, refundNo string, amount int64, _ string) (string, error) {
	query := "INSERT INTO `" + g.ledgerTable + "` (refund_no, amount, outcome, provider_reference) VALUES (?, ?, ?, ?)"
	if err := g.db.Exec(query, refundNo, amount, g.outcome, g.providerReference).Error; err != nil {
		fmt.Fprintf(os.Stderr, "synthetic provider outcome recorder failed: %v\n", err)
		os.Exit(refundCrashDispatchExit + 1)
	}
	os.Exit(refundCrashDispatchExit)
	return "", nil
}

func TestMySQLRefundWorkerCrashAfterCommittedReservation(t *testing.T) {
	if mode := os.Getenv(refundCrashChildEnv); mode != "" {
		switch mode {
		case refundCrashModeExit:
			runRefundCrashChild(t)
		case refundRetryModeExit:
			runRefundRestartRetryChild(t)
		case refundProviderCrashMode:
			runRefundProviderOutcomeCrashChild(t)
		default:
			t.Fatalf("unsupported subprocess mode %q", mode)
		}
		return
	}

	db, _ := newMySQLRefundIntegrationDB(t)
	merchant, _, _, order := createSyntheticStoreOrder(t, db, uniqueSyntheticMySQLOrderNo(t, "WORKER-CRASH"), "accepted", 10000, timePtr(time.Now().UTC()))
	order.TransactionNo = "SYNTHETIC-MYSQL-PAID-TXN-WORKER-CRASH"
	if err := db.Save(&order).Error; err != nil {
		t.Fatal(err)
	}

	dispatchTable := fmt.Sprintf("synthetic_refund_dispatches_%d", time.Now().UnixNano())
	if err := db.Exec("CREATE TABLE `" + dispatchTable + "` (id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, refund_no VARCHAR(60) NOT NULL, PRIMARY KEY (id)) ENGINE=InnoDB").Error; err != nil {
		t.Fatalf("create disposable fake-provider dispatch log: %v", err)
	}
	defer func() { _ = db.Exec("DROP TABLE IF EXISTS `" + dispatchTable + "`").Error }()

	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	output, childErr := runRefundTestSubprocess(t, binary, refundCrashModeExit, order.ID, merchant.ID, dispatchTable)
	exitErr, ok := childErr.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != refundCrashDispatchExit {
		t.Fatalf("fake provider child should exit abruptly at dispatch with code %d; err=%v output=%s", refundCrashDispatchExit, childErr, output)
	}
	t.Logf("worker subprocess exited abruptly at fake-provider call site (exit=%d); no provider response/finalization ran", exitErr.ExitCode())

	// Close the parent's original pool as well, then reopen the same durable
	// database to prove the reservation survives both worker and connection loss.
	initialSQLDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := initialSQLDB.Close(); err != nil {
		t.Fatalf("close pre-crash database pool: %v", err)
	}
	db, _ = newMySQLRefundIntegrationDB(t)
	service := NewOrderService(db)
	var refunds []model.RefundRecord
	if err := db.Where("order_id = ?", order.ID).Find(&refunds).Error; err != nil {
		t.Fatal(err)
	}
	if len(refunds) != 1 || refunds[0].Status != "processing" || refunds[0].Amount != 2400 {
		t.Fatalf("crashed worker must leave exactly one committed processing reservation: %+v", refunds)
	}
	var currentOrder model.Order
	if err := db.First(&currentOrder, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	if currentOrder.RefundedAmount != 0 || currentOrder.RefundStatus != "processing" {
		t.Fatalf("crashed worker must hold but not claim the refund: refunded=%d status=%q", currentOrder.RefundedAmount, currentOrder.RefundStatus)
	}
	var successRows int64
	if err := db.Model(&model.RefundRecord{}).Where("order_id = ? AND status = ?", order.ID, "success").Count(&successRows).Error; err != nil {
		t.Fatal(err)
	}
	var dispatches int64
	if err := db.Table(dispatchTable).Count(&dispatches).Error; err != nil {
		t.Fatal(err)
	}
	if successRows != 0 || dispatches != 1 {
		t.Fatalf("post-crash durable state mismatch: successful refund rows=%d fake-provider dispatches=%d", successRows, dispatches)
	}

	for restart := 1; restart <= 3; restart++ {
		// Read-only queue/detail refreshes must leave the reservation untouched.
		queue, err := service.UnknownRefundReservations()
		if err != nil {
			t.Fatal(err)
		}
		detail, err := service.RefundReconciliationDetail(refunds[0].ID)
		if err != nil {
			t.Fatal(err)
		}
		if detail.ReservationStatus != "processing" {
			t.Fatalf("refresh before restart %d changed crash reservation to %q", restart, detail.ReservationStatus)
		}
		found := false
		for _, item := range queue {
			if item.RefundID == refunds[0].ID && item.ReservationStatus == "processing" {
				found = true
			}
		}
		if !found {
			t.Fatalf("refresh before restart %d did not retain processing reservation: %+v", restart, queue)
		}

		// Each retry runs in a brand-new OS process, reopens the same database,
		// and has a fake gateway that would durably record any duplicate dispatch.
		output, retryErr := runRefundTestSubprocess(t, binary, refundRetryModeExit, order.ID, merchant.ID, dispatchTable)
		if retryErr != nil {
			t.Fatalf("fresh worker process %d could not safely refuse duplicate dispatch: %v; output=%s", restart, retryErr, output)
		}
		if err := db.Table(dispatchTable).Count(&dispatches).Error; err != nil {
			t.Fatal(err)
		}
		if dispatches != 1 {
			t.Fatalf("fresh worker process %d dispatched again after crash: fake-provider call count=%d", restart, dispatches)
		}
	}

	if err := db.Model(&model.RefundRecord{}).Where("id = ?", refunds[0].ID).UpdateColumn("updated_at", time.Now().UTC().Add(-20*time.Minute)).Error; err != nil {
		t.Fatalf("age synthetic crash reservation past review threshold: %v", err)
	}
	queue, err := service.UnknownRefundReservations()
	if err != nil {
		t.Fatal(err)
	}
	staleFound := false
	for _, item := range queue {
		if item.RefundID == refunds[0].ID {
			staleFound = item.IsStale && item.ReservationStatus == "processing" && item.StaleAfterMinutes == 15
		}
	}
	if !staleFound {
		t.Fatalf("aged crash reservation should appear overdue in manual queue: %+v", queue)
	}

	note := "合成故障注入：worker 在 provider 响应前退出，转人工核对"
	auditBefore := time.Now().UTC()
	detail, err := service.MarkStaleProcessingRefundUnknown(refunds[0].ID, note, 981, "super_admin")
	if err != nil {
		t.Fatalf("move overdue crash reservation to manual review: %v", err)
	}
	if detail.ReservationStatus != "unknown" || len(detail.Audit) != 1 {
		t.Fatalf("manual-review transition/audit missing: %+v", detail)
	}
	audit := detail.Audit[0]
	if audit.OldStatus != "processing" || audit.NewStatus != "unknown" || audit.ActorID != 981 || audit.ActorRole != "super_admin" || audit.Reason != note || audit.CreatedAt.Before(auditBefore) || audit.CreatedAt.After(time.Now().UTC().Add(time.Second)) {
		t.Fatalf("manual-review audit actor/time/reason mismatch: %+v", audit)
	}
	var afterReview model.Order
	if err := db.First(&afterReview, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	var reservedAmount int64
	if err := db.Model(&model.RefundRecord{}).Where("order_id = ? AND status IN ?", order.ID, []string{"processing", "unknown"}).Select("COALESCE(SUM(amount), 0)").Scan(&reservedAmount).Error; err != nil {
		t.Fatal(err)
	}
	if afterReview.RefundedAmount != 0 || afterReview.RefundStatus != "processing" || reservedAmount != 2400 {
		t.Fatalf("manual-review transition must retain the entire reservation without claiming a refund: refunded=%d status=%q reserved=%d", afterReview.RefundedAmount, afterReview.RefundStatus, reservedAmount)
	}
	if err := db.Table(dispatchTable).Count(&dispatches).Error; err != nil {
		t.Fatal(err)
	}
	if dispatches != 1 {
		t.Fatalf("manual review must not query/re-dispatch the provider; calls=%d", dispatches)
	}

	// The triage transition to unknown is not a provider-side resolution; a new
	// process must still refuse to send a duplicate request until a human records
	// a substantiated success/failure outcome.
	output, retryErr := runRefundTestSubprocess(t, binary, refundRetryModeExit, order.ID, merchant.ID, dispatchTable)
	if retryErr != nil {
		t.Fatalf("worker process after manual triage should still refuse dispatch: %v; output=%s", retryErr, output)
	}
	if err := db.Table(dispatchTable).Count(&dispatches).Error; err != nil {
		t.Fatal(err)
	}
	if dispatches != 1 {
		t.Fatalf("manual triage must not trigger provider dispatch; calls=%d", dispatches)
	}
	t.Logf("verified reservation after real subprocess exit; three new worker processes caused no duplicate call; aged row moved to unknown with immutable audit; reserved=%d cents; fake-provider dispatches=%d", reservedAmount, dispatches)
}

func TestMySQLRefundProviderOutcomeLostToCrashReconcilesSuccessAndFailure(t *testing.T) {
	db, dsn := newDisposableMySQLRefundIntegrationDB(t)
	paidAt := time.Now().UTC()
	successMerchant, _, _, successOrder := createSyntheticStoreOrder(t, db, uniqueSyntheticMySQLOrderNo(t, "RECOVERY-SUCCESS"), "accepted", 10000, &paidAt)
	successOrder.TransactionNo = "SYNTHETIC-MYSQL-PAID-TXN-RECOVERY-SUCCESS"
	if err := db.Save(&successOrder).Error; err != nil {
		t.Fatal(err)
	}
	failureMerchant, _, _, failureOrder := createSyntheticStoreOrder(t, db, uniqueSyntheticMySQLOrderNo(t, "RECOVERY-FAILURE"), "accepted", 10000, &paidAt)
	failureOrder.TransactionNo = "SYNTHETIC-MYSQL-PAID-TXN-RECOVERY-FAILURE"
	if err := db.Save(&failureOrder).Error; err != nil {
		t.Fatal(err)
	}

	ledgerTable := fmt.Sprintf("synthetic_provider_ledger_%d", time.Now().UnixNano())
	if !strings.HasPrefix(ledgerTable, "synthetic_provider_ledger_") {
		t.Fatal("refusing to create a non-synthetic provider ledger")
	}
	if err := db.Exec("CREATE TABLE `" + ledgerTable + "` (id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, refund_no VARCHAR(60) NOT NULL, amount BIGINT NOT NULL DEFAULT 0, outcome VARCHAR(20) NOT NULL DEFAULT 'duplicate_dispatch', provider_reference VARCHAR(120) NOT NULL DEFAULT '', PRIMARY KEY (id)) ENGINE=InnoDB").Error; err != nil {
		t.Fatalf("create disposable synthetic provider ledger: %v", err)
	}
	defer func() { _ = db.Exec("DROP TABLE IF EXISTS `" + ledgerTable + "`").Error }()

	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	successRequest := dto.RefundOrderRequest{Amount: 2400, Reason: "synthetic accepted refund after lost response", IdempotencyKey: "synthetic-post-crash-success-key"}
	failureRequest := dto.RefundOrderRequest{Amount: 1600, Reason: "synthetic rejected refund after lost response", IdempotencyKey: "synthetic-post-crash-failure-key"}
	successReference := "SYNTHETIC-CHANNEL-REF-ACCEPTED-001"
	failureReference := "SYNTHETIC-CHANNEL-REF-REJECTED-001"

	closeRefundTestDBPool(t, db)
	assertProviderOutcomeCrash(t, binary, dsn, successOrder.ID, successMerchant.ID, ledgerTable, "success", successReference, successRequest)
	db, _ = openMySQLRefundIntegrationDB(t, dsn)
	successRefund, _ := assertCrashedRefundReservation(t, db, successOrder.ID, successRequest.Amount)
	assertSyntheticProviderEvidence(t, db, ledgerTable, successRefund.RefundNo, "success", successRequest.Amount, successReference)
	assertSyntheticProviderDispatchCount(t, db, ledgerTable, 1)
	db = assertFreshWorkerRefusesRedispatch(t, binary, dsn, db, successOrder.ID, successMerchant.ID, ledgerTable, successRequest)
	assertSyntheticProviderDispatchCount(t, db, ledgerTable, 1)
	successRefund, _ = assertCrashedRefundReservation(t, db, successOrder.ID, successRequest.Amount)

	service := NewOrderService(db)
	successReviewReason := "合成演练：服务商已接受退款，但 worker 未收到响应，转管理员核验"
	successManualStart := time.Now().UTC()
	_ = ageAndMoveRecoveryRefundToUnknown(t, db, service, successRefund, 901, "super_admin", successReviewReason)
	successManualEnd := time.Now().UTC()
	assertSyntheticProviderEvidence(t, db, ledgerTable, successRefund.RefundNo, "success", successRequest.Amount, successReference)
	successResolveReason := "已核对合成渠道流水，确认该请求退款成功"
	successResolveStart := time.Now().UTC()
	successResolved, err := service.ResolveUnknownRefund(successRefund.ID, dto.RefundReconciliationRequest{
		Outcome: "success", ProviderReference: successReference, Reason: successResolveReason,
	}, 902, "admin")
	successResolveEnd := time.Now().UTC()
	if err != nil {
		t.Fatalf("resolve post-crash synthetic provider success: %v", err)
	}
	assertRecoveryAuditTrail(t, successResolved.Audit, successRefund, "success", 901, "super_admin", 902, "admin", successReference, successReviewReason, successResolveReason, successManualStart, successManualEnd, successResolveStart, successResolveEnd)
	assertTerminalRecoveryState(t, db, successOrder.ID, successRefund.ID, "success", successRequest.Amount, "partial", "accepted")
	successRetry, err := service.ResolveUnknownRefund(successRefund.ID, dto.RefundReconciliationRequest{
		Outcome: "success", ProviderReference: successReference, Reason: successResolveReason,
	}, 902, "admin")
	if err != nil || len(successRetry.Audit) != 2 {
		t.Fatalf("exact success reconciliation retry must be idempotent: detail=%+v error=%v", successRetry, err)
	}
	service.alipay = &recordingSyntheticRefundGateway{db: db, dispatchTable: ledgerTable}
	replayedSuccess, _, err := service.RefundMerchantOrder(successMerchant.ID, successOrder.ID, successRequest, 42, "merchant")
	if err != nil || replayedSuccess.RefundedAmount != successRequest.Amount {
		t.Fatalf("exact terminal success request replay mismatch: order=%+v error=%v", replayedSuccess, err)
	}
	assertSyntheticProviderDispatchCount(t, db, ledgerTable, 1)
	assertTerminalRecoveryState(t, db, successOrder.ID, successRefund.ID, "success", successRequest.Amount, "partial", "accepted")

	closeRefundTestDBPool(t, db)
	assertProviderOutcomeCrash(t, binary, dsn, failureOrder.ID, failureMerchant.ID, ledgerTable, "failed", failureReference, failureRequest)
	db, _ = openMySQLRefundIntegrationDB(t, dsn)
	failureRefund, _ := assertCrashedRefundReservation(t, db, failureOrder.ID, failureRequest.Amount)
	assertSyntheticProviderEvidence(t, db, ledgerTable, failureRefund.RefundNo, "failed", failureRequest.Amount, failureReference)
	assertSyntheticProviderDispatchCount(t, db, ledgerTable, 2)
	db = assertFreshWorkerRefusesRedispatch(t, binary, dsn, db, failureOrder.ID, failureMerchant.ID, ledgerTable, failureRequest)
	assertSyntheticProviderDispatchCount(t, db, ledgerTable, 2)
	failureRefund, _ = assertCrashedRefundReservation(t, db, failureOrder.ID, failureRequest.Amount)

	service = NewOrderService(db)
	failureReviewReason := "合成演练：worker 未收到渠道明确拒绝的响应，转管理员核验"
	failureManualStart := time.Now().UTC()
	_ = ageAndMoveRecoveryRefundToUnknown(t, db, service, failureRefund, 903, "admin", failureReviewReason)
	failureManualEnd := time.Now().UTC()
	assertSyntheticProviderEvidence(t, db, ledgerTable, failureRefund.RefundNo, "failed", failureRequest.Amount, failureReference)
	failureResolveReason := "已核对合成渠道记录，确认该退款请求明确失败"
	failureResolveStart := time.Now().UTC()
	failureResolved, err := service.ResolveUnknownRefund(failureRefund.ID, dto.RefundReconciliationRequest{
		Outcome: "failed", ProviderReference: failureReference, Reason: failureResolveReason,
	}, 904, "super_admin")
	failureResolveEnd := time.Now().UTC()
	if err != nil {
		t.Fatalf("resolve post-crash synthetic provider failure: %v", err)
	}
	assertRecoveryAuditTrail(t, failureResolved.Audit, failureRefund, "failed", 903, "admin", 904, "super_admin", failureReference, failureReviewReason, failureResolveReason, failureManualStart, failureManualEnd, failureResolveStart, failureResolveEnd)
	assertTerminalRecoveryState(t, db, failureOrder.ID, failureRefund.ID, "failed", failureRequest.Amount, "none", "accepted")
	failureRetry, err := service.ResolveUnknownRefund(failureRefund.ID, dto.RefundReconciliationRequest{
		Outcome: "failed", ProviderReference: failureReference, Reason: failureResolveReason,
	}, 904, "super_admin")
	if err != nil || len(failureRetry.Audit) != 2 {
		t.Fatalf("exact failed reconciliation retry must be idempotent: detail=%+v error=%v", failureRetry, err)
	}
	_, _, err = service.RefundMerchantOrder(failureMerchant.ID, failureOrder.ID, failureRequest, 42, "merchant")
	if err == nil || !strings.Contains(err.Error(), "definitively rejected") {
		t.Fatalf("exact failed request replay must report terminal rejection without dispatch: %v", err)
	}
	assertSyntheticProviderDispatchCount(t, db, ledgerTable, 2)
	assertTerminalRecoveryState(t, db, failureOrder.ID, failureRefund.ID, "failed", failureRequest.Amount, "none", "accepted")
	t.Logf("post-crash reconciliation complete in a separate disposable MySQL database: synthetic provider dispatches=2; success refunded=%d cents once; failure released=%d cents without refund; both audit trails contain two immutable transitions", successRequest.Amount, failureRequest.Amount)
}

func newDisposableMySQLRefundIntegrationDB(t *testing.T) (*gorm.DB, string) {
	t.Helper()
	baseDSN := mysqlRefundTestDSN(t)
	baseDB, _ := newMySQLRefundIntegrationDB(t)
	baseSQL, err := baseDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	database := fmt.Sprintf("cue_refund_p1_journey_%d", time.Now().UnixNano())
	if _, err := baseSQL.Exec("CREATE DATABASE `" + database + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		t.Fatalf("create separate disposable refund-recovery database: %v", err)
	}
	t.Cleanup(func() {
		if _, err := baseSQL.Exec("DROP DATABASE IF EXISTS `" + database + "`"); err != nil {
			t.Errorf("drop disposable refund-recovery database: %v", err)
		}
	})
	dsn := mysqlDSNForDatabase(baseDSN, database)
	db, _ := openMySQLRefundIntegrationDB(t, dsn)
	return db, dsn
}

func closeRefundTestDBPool(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close MySQL pool before worker process crash/restart: %v", err)
	}
}

func assertProviderOutcomeCrash(t *testing.T, binary, dsn string, orderID, merchantID uint, ledgerTable, outcome, reference string, request dto.RefundOrderRequest) {
	t.Helper()
	output, err := runRefundTestSubprocessWithDSN(t, binary, refundProviderCrashMode, orderID, merchantID, ledgerTable, dsn, outcome, reference, request.Amount, request.IdempotencyKey, request.Reason)
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != refundCrashDispatchExit {
		t.Fatalf("worker must die after synthetic provider records %q but before app receives it (want exit %d): err=%v output=%s", outcome, refundCrashDispatchExit, err, output)
	}
	t.Logf("worker process exited at fake-provider response boundary with synthetic outcome %q (exit=%d)", outcome, exitErr.ExitCode())
}

func assertFreshWorkerRefusesRedispatch(t *testing.T, binary, dsn string, db *gorm.DB, orderID, merchantID uint, ledgerTable string, request dto.RefundOrderRequest) *gorm.DB {
	t.Helper()
	closeRefundTestDBPool(t, db)
	output, err := runRefundTestSubprocessWithDSN(t, binary, refundRetryModeExit, orderID, merchantID, ledgerTable, dsn, "", "", request.Amount, request.IdempotencyKey, request.Reason)
	if err != nil {
		t.Fatalf("fresh process must safely refuse redispatch of the exact unresolved request: %v; output=%s", err, output)
	}
	db, _ = openMySQLRefundIntegrationDB(t, dsn)
	return db
}

func assertCrashedRefundReservation(t *testing.T, db *gorm.DB, orderID uint, amount int64) (model.RefundRecord, model.Order) {
	t.Helper()
	var refunds []model.RefundRecord
	if err := db.Where("order_id = ?", orderID).Find(&refunds).Error; err != nil {
		t.Fatal(err)
	}
	if len(refunds) != 1 || refunds[0].Status != "processing" || refunds[0].Amount != amount {
		t.Fatalf("post-crash order must retain exactly one processing reservation: %+v", refunds)
	}
	var order model.Order
	if err := db.First(&order, orderID).Error; err != nil {
		t.Fatal(err)
	}
	var successCount, reserved int64
	if err := db.Model(&model.RefundRecord{}).Where("order_id = ? AND status = ?", orderID, "success").Count(&successCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.RefundRecord{}).Where("order_id = ? AND status IN ?", orderID, []string{"processing", "unknown"}).Select("COALESCE(SUM(amount), 0)").Scan(&reserved).Error; err != nil {
		t.Fatal(err)
	}
	if successCount != 0 || order.RefundedAmount != 0 || order.RefundStatus != "processing" || reserved != amount {
		t.Fatalf("provider result lost to crash must not claim success or release reservation: success_rows=%d refunded=%d order_status=%q reserved=%d", successCount, order.RefundedAmount, order.RefundStatus, reserved)
	}
	return refunds[0], order
}

func assertSyntheticProviderEvidence(t *testing.T, db *gorm.DB, ledgerTable, refundNo, outcome string, amount int64, reference string) {
	t.Helper()
	var record struct {
		RefundNo          string
		Amount            int64
		Outcome           string
		ProviderReference string
	}
	if err := db.Table(ledgerTable).Where("refund_no = ?", refundNo).Take(&record).Error; err != nil {
		t.Fatalf("load synthetic provider-side evidence: %v", err)
	}
	var rows int64
	if err := db.Table(ledgerTable).Where("refund_no = ?", refundNo).Count(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if rows != 1 || record.Amount != amount || record.Outcome != outcome || record.ProviderReference != reference {
		t.Fatalf("synthetic provider evidence mismatch: rows=%d record=%+v expected outcome=%s amount=%d reference=%s", rows, record, outcome, amount, reference)
	}
}

func assertSyntheticProviderDispatchCount(t *testing.T, db *gorm.DB, ledgerTable string, want int64) {
	t.Helper()
	var count int64
	if err := db.Table(ledgerTable).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("unexpected synthetic provider dispatch count: got=%d want=%d", count, want)
	}
}

func ageAndMoveRecoveryRefundToUnknown(t *testing.T, db *gorm.DB, service *OrderService, refund model.RefundRecord, actorID uint, actorRole, reason string) *RefundReconciliationDetail {
	t.Helper()
	if err := db.Model(&model.RefundRecord{}).Where("id = ?", refund.ID).UpdateColumn("updated_at", time.Now().UTC().Add(-20*time.Minute)).Error; err != nil {
		t.Fatalf("age crashed reservation past the manual-review threshold: %v", err)
	}
	queue, err := service.UnknownRefundReservations()
	if err != nil {
		t.Fatal(err)
	}
	foundStale := false
	for _, item := range queue {
		if item.RefundID == refund.ID {
			foundStale = item.IsStale && item.ReservationStatus == "processing" && item.StaleAfterMinutes == 15
		}
	}
	if !foundStale {
		t.Fatalf("aged crashed refund must appear as overdue processing: %+v", queue)
	}
	start := time.Now().UTC()
	detail, err := service.MarkStaleProcessingRefundUnknown(refund.ID, reason, actorID, actorRole)
	end := time.Now().UTC()
	if err != nil {
		t.Fatalf("move aged crashed reservation to unknown: %v", err)
	}
	if detail.ReservationStatus != "unknown" || len(detail.Audit) != 1 {
		t.Fatalf("manual review must preserve a single audited processing-to-unknown transition: %+v", detail)
	}
	audit := detail.Audit[0]
	if audit.OldStatus != "processing" || audit.NewStatus != "unknown" || audit.ActorID != actorID || audit.ActorRole != actorRole || audit.Amount != refund.Amount || audit.ProviderReference != "" || audit.Reason != reason || audit.CreatedAt.Before(start.Add(-time.Second)) || audit.CreatedAt.After(end.Add(time.Second)) {
		t.Fatalf("manual-review audit must preserve actor, time, amount and reason without claiming provider evidence: %+v", audit)
	}
	retry, err := service.MarkStaleProcessingRefundUnknown(refund.ID, reason, actorID, actorRole)
	if err != nil || len(retry.Audit) != 1 {
		t.Fatalf("exact processing-to-unknown retry must be idempotent: detail=%+v error=%v", retry, err)
	}
	var order model.Order
	if err := db.First(&order, refund.OrderID).Error; err != nil {
		t.Fatal(err)
	}
	var reserved int64
	if err := db.Model(&model.RefundRecord{}).Where("order_id = ? AND status IN ?", refund.OrderID, []string{"processing", "unknown"}).Select("COALESCE(SUM(amount), 0)").Scan(&reserved).Error; err != nil {
		t.Fatal(err)
	}
	if order.RefundedAmount != 0 || order.RefundStatus != "processing" || reserved != refund.Amount {
		t.Fatalf("manual review must retain reservation without claiming success: refunded=%d order_refund_status=%q reserved=%d", order.RefundedAmount, order.RefundStatus, reserved)
	}
	return detail
}

func assertRecoveryAuditTrail(t *testing.T, audits []model.RefundReconciliationAudit, refund model.RefundRecord, finalStatus string, reviewActor uint, reviewRole string, resolveActor uint, resolveRole, reference, reviewReason, resolveReason string, reviewStart, reviewEnd, resolveStart, resolveEnd time.Time) {
	t.Helper()
	if len(audits) != 2 {
		t.Fatalf("recovery trail must have exactly two transitions, got %+v", audits)
	}
	review, resolved := audits[0], audits[1]
	if review.OldStatus != "processing" || review.NewStatus != "unknown" || review.ActorID != reviewActor || review.ActorRole != reviewRole || review.Amount != refund.Amount || review.ProviderReference != "" || review.Reason != reviewReason || review.CreatedAt.Before(reviewStart.Add(-time.Second)) || review.CreatedAt.After(reviewEnd.Add(time.Second)) {
		t.Fatalf("processing-to-unknown audit mismatch: %+v", review)
	}
	if resolved.OldStatus != "unknown" || resolved.NewStatus != finalStatus || resolved.ActorID != resolveActor || resolved.ActorRole != resolveRole || resolved.Amount != refund.Amount || resolved.ProviderReference != reference || resolved.Reason != resolveReason || resolved.CreatedAt.Before(resolveStart.Add(-time.Second)) || resolved.CreatedAt.After(resolveEnd.Add(time.Second)) || resolved.CreatedAt.Before(review.CreatedAt) {
		t.Fatalf("unknown-to-%s audit mismatch: %+v", finalStatus, resolved)
	}
}

func assertTerminalRecoveryState(t *testing.T, db *gorm.DB, orderID, refundID uint, refundStatus string, amount int64, orderRefundStatus, orderStatus string) {
	t.Helper()
	var refund model.RefundRecord
	if err := db.First(&refund, refundID).Error; err != nil {
		t.Fatal(err)
	}
	var order model.Order
	if err := db.First(&order, orderID).Error; err != nil {
		t.Fatal(err)
	}
	var reserved int64
	if err := db.Model(&model.RefundRecord{}).Where("order_id = ? AND status IN ?", orderID, []string{"processing", "unknown"}).Select("COALESCE(SUM(amount), 0)").Scan(&reserved).Error; err != nil {
		t.Fatal(err)
	}
	expectedRefunded := int64(0)
	if refundStatus == "success" {
		expectedRefunded = amount
	}
	if refund.Status != refundStatus || refund.Amount != amount || reserved != 0 || order.RefundedAmount != expectedRefunded || order.RefundStatus != orderRefundStatus || order.Status != orderStatus {
		t.Fatalf("terminal recovery state mismatch: refund=%q/%d reserved=%d refunded=%d order_refund_status=%q order_status=%q", refund.Status, refund.Amount, reserved, order.RefundedAmount, order.RefundStatus, order.Status)
	}
	var auditCount int64
	if err := db.Model(&model.RefundReconciliationAudit{}).Where("refund_id = ?", refundID).Count(&auditCount).Error; err != nil {
		t.Fatal(err)
	}
	if auditCount != 2 {
		t.Fatalf("terminal recovery must preserve both transitions, audit rows=%d", auditCount)
	}
}

func runRefundTestSubprocess(t *testing.T, binary, mode string, orderID, merchantID uint, dispatchTable string) ([]byte, error) {
	t.Helper()
	return runRefundTestSubprocessWithDSN(t, binary, mode, orderID, merchantID, dispatchTable, os.Getenv(mysqlRefundTestDSNEnv), "", "", 0, "", "")
}

func runRefundTestSubprocessWithDSN(t *testing.T, binary, mode string, orderID, merchantID uint, dispatchTable, dsn, providerOutcome, providerReference string, amount int64, idempotencyKey, reason string) ([]byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, binary, "-test.run=^TestMySQLRefundWorkerCrashAfterCommittedReservation$")
	child.Env = replaceTestEnvironment(os.Environ(), mysqlRefundTestDSNEnv, dsn)
	child.Env = append(child.Env,
		refundCrashChildEnv+"="+mode,
		refundCrashOrderIDEnv+"="+strconv.FormatUint(uint64(orderID), 10),
		refundCrashMerchantEnv+"="+strconv.FormatUint(uint64(merchantID), 10),
		refundCrashTableEnv+"="+dispatchTable,
	)
	if idempotencyKey != "" {
		child.Env = append(child.Env,
			refundProviderOutcomeEnv+"="+providerOutcome,
			refundProviderReferenceEnv+"="+providerReference,
			refundProviderAmountEnv+"="+strconv.FormatInt(amount, 10),
			refundProviderKeyEnv+"="+idempotencyKey,
			refundProviderReasonEnv+"="+reason,
		)
	}
	output, err := child.CombinedOutput()
	if ctx.Err() != nil {
		return output, fmt.Errorf("subprocess timed out: %w", ctx.Err())
	}
	return output, err
}

func replaceTestEnvironment(environment []string, name, value string) []string {
	prefix := name + "="
	filtered := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		if !strings.HasPrefix(entry, prefix) {
			filtered = append(filtered, entry)
		}
	}
	return append(filtered, prefix+value)
}

func runRefundCrashChild(t *testing.T) {
	db, _ := newMySQLRefundIntegrationDB(t)
	orderID, merchantID, dispatchTable := refundCrashChildArguments(t)
	service := NewOrderService(db)
	service.alipay = &exitAfterSyntheticDispatchGateway{db: db, dispatchTable: dispatchTable}
	_, _, err := service.RefundMerchantOrder(merchantID, orderID, dto.RefundOrderRequest{
		Amount:         2400,
		Reason:         "synthetic worker crash test",
		IdempotencyKey: refundCrashIdempotency,
	}, 42, "merchant")
	t.Fatalf("child refund unexpectedly returned before abrupt fake-provider exit: %v", err)
}

func runRefundRestartRetryChild(t *testing.T) {
	db, _ := newMySQLRefundIntegrationDB(t)
	orderID, merchantID, dispatchTable := refundCrashChildArguments(t)
	service := NewOrderService(db)
	service.alipay = &recordingSyntheticRefundGateway{db: db, dispatchTable: dispatchTable}
	request := dto.RefundOrderRequest{
		Amount:         2400,
		Reason:         "synthetic worker crash test",
		IdempotencyKey: refundCrashIdempotency,
	}
	if rawAmount := os.Getenv(refundProviderAmountEnv); rawAmount != "" {
		amount, err := strconv.ParseInt(rawAmount, 10, 64)
		if err != nil || amount <= 0 {
			t.Fatalf("invalid synthetic retry amount: %q", rawAmount)
		}
		request.Amount = amount
		request.Reason = os.Getenv(refundProviderReasonEnv)
		request.IdempotencyKey = os.Getenv(refundProviderKeyEnv)
	}
	_, _, err := service.RefundMerchantOrder(merchantID, orderID, request, 42, "merchant")
	if err == nil || !strings.Contains(err.Error(), "must be reconciled") {
		t.Fatalf("restarted worker must refuse an existing processing/unknown reservation before dispatch: %v", err)
	}
}

func runRefundProviderOutcomeCrashChild(t *testing.T) {
	db, _ := newMySQLRefundIntegrationDB(t)
	orderID, merchantID, ledgerTable := refundCrashChildArguments(t)
	outcome := os.Getenv(refundProviderOutcomeEnv)
	if outcome != "success" && outcome != "failed" {
		t.Fatalf("invalid synthetic provider outcome: %q", outcome)
	}
	providerReference := os.Getenv(refundProviderReferenceEnv)
	if !strings.HasPrefix(providerReference, "SYNTHETIC-CHANNEL-REF-") {
		t.Fatalf("refusing non-synthetic provider reference %q", providerReference)
	}
	amount, err := strconv.ParseInt(os.Getenv(refundProviderAmountEnv), 10, 64)
	if err != nil || amount <= 0 {
		t.Fatalf("invalid synthetic refund amount: %q", os.Getenv(refundProviderAmountEnv))
	}
	idempotencyKey := os.Getenv(refundProviderKeyEnv)
	reason := os.Getenv(refundProviderReasonEnv)
	if !strings.HasPrefix(idempotencyKey, "synthetic-post-crash-") || reason == "" {
		t.Fatalf("invalid synthetic request key/reason: key=%q reason=%q", idempotencyKey, reason)
	}
	service := NewOrderService(db)
	service.alipay = &exitAfterSyntheticProviderOutcomeGateway{
		db: db, ledgerTable: ledgerTable, outcome: outcome, providerReference: providerReference,
	}
	_, _, err = service.RefundMerchantOrder(merchantID, orderID, dto.RefundOrderRequest{
		Amount: amount, Reason: reason, IdempotencyKey: idempotencyKey,
	}, 42, "merchant")
	t.Fatalf("child refund unexpectedly returned before abrupt fake-provider exit: %v", err)
}

func refundCrashChildArguments(t *testing.T) (uint, uint, string) {
	t.Helper()
	orderID, err := strconv.ParseUint(os.Getenv(refundCrashOrderIDEnv), 10, 64)
	if err != nil || orderID == 0 {
		t.Fatalf("invalid child order id: %q", os.Getenv(refundCrashOrderIDEnv))
	}
	merchantID, err := strconv.ParseUint(os.Getenv(refundCrashMerchantEnv), 10, 64)
	if err != nil || merchantID == 0 {
		t.Fatalf("invalid child merchant id: %q", os.Getenv(refundCrashMerchantEnv))
	}
	dispatchTable := os.Getenv(refundCrashTableEnv)
	if !strings.HasPrefix(dispatchTable, "synthetic_refund_dispatches_") && !strings.HasPrefix(dispatchTable, "synthetic_provider_ledger_") {
		t.Fatalf("refusing unexpected fake-provider table %q", dispatchTable)
	}
	return uint(orderID), uint(merchantID), dispatchTable
}
