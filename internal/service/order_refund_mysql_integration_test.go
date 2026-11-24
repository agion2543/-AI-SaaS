package service

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mysqlDriver "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
)

const mysqlRefundTestDSNEnv = "MYSQL_REFUND_IT_DSN"

type mysqlSQLCapture struct {
	mu sync.Mutex
	b  strings.Builder
}

func (c *mysqlSQLCapture) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.b.Write(p)
}

func (c *mysqlSQLCapture) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.b.String()
}

type blockingSyntheticRefundGateway struct {
	calls   atomic.Int32
	entered chan struct{}
	release chan struct{}
	err     error
	once    sync.Once
}

func newBlockingSyntheticRefundGateway() *blockingSyntheticRefundGateway {
	return &blockingSyntheticRefundGateway{
		entered: make(chan struct{}, 4),
		release: make(chan struct{}),
		err:     errors.New("synthetic transport timeout; provider outcome unknown"),
	}
}

func (*blockingSyntheticRefundGateway) Enabled() bool { return true }

func (g *blockingSyntheticRefundGateway) Refund(_ *model.Order, _ string, _ int64, _ string) (string, error) {
	g.calls.Add(1)
	select {
	case g.entered <- struct{}{}:
	default:
	}
	<-g.release
	return "", g.err
}

func (g *blockingSyntheticRefundGateway) releaseAll() {
	g.once.Do(func() { close(g.release) })
}

func mysqlRefundTestDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv(mysqlRefundTestDSNEnv)
	if dsn == "" {
		t.Skip("set MYSQL_REFUND_IT_DSN to a disposable MySQL 8.0.46 Unix-socket database to run integration tests")
	}
	unixStart := strings.Index(dsn, "@unix(")
	if unixStart < 0 {
		t.Fatalf("refusing integration DSN without a local Unix socket: %q", dsn)
	}
	socketStart := unixStart + len("@unix(")
	socketEndRelative := strings.Index(dsn[socketStart:], ")")
	if socketEndRelative < 0 {
		t.Fatalf("malformed Unix-socket DSN")
	}
	socketPath := dsn[socketStart : socketStart+socketEndRelative]
	if !filepath.IsAbs(socketPath) || !strings.HasPrefix(socketPath, "/tmp/cue-refund-mysql-") {
		t.Fatalf("refusing socket outside the dedicated temporary MySQL directory: %q", socketPath)
	}
	dbMarker := strings.LastIndex(dsn, ")/")
	if dbMarker < 0 {
		t.Fatalf("malformed MySQL DSN database component")
	}
	database := strings.SplitN(dsn[dbMarker+2:], "?", 2)[0]
	if !strings.HasPrefix(database, "cue_refund_p1_") {
		t.Fatalf("refusing database without the dedicated disposable prefix cue_refund_p1_: %q", database)
	}
	return dsn
}

func newMySQLRefundIntegrationDB(t *testing.T) (*gorm.DB, *mysqlSQLCapture) {
	t.Helper()
	return openMySQLRefundIntegrationDB(t, mysqlRefundTestDSN(t))
}

func openMySQLRefundIntegrationDB(t *testing.T, dsn string) (*gorm.DB, *mysqlSQLCapture) {
	t.Helper()
	capture := &mysqlSQLCapture{}
	db, err := gorm.Open(mysqlDriver.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   logger.New(log.New(capture, "", 0), logger.Config{LogLevel: logger.Info, Colorful: false}),
	})
	if err != nil {
		t.Fatalf("open disposable MySQL database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(32)
	t.Cleanup(func() { _ = sqlDB.Close() })

	var version string
	if err := db.Raw("SELECT VERSION()").Scan(&version).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(version, "8.0.46") {
		t.Fatalf("integration tests require MySQL 8.0.46, got %q", version)
	}
	var skipNetworking int
	if err := db.Raw("SELECT @@skip_networking").Scan(&skipNetworking).Error; err != nil {
		t.Fatal(err)
	}
	if skipNetworking != 1 {
		t.Fatalf("integration server must have TCP networking disabled, @@skip_networking=%d", skipNetworking)
	}

	if err := db.AutoMigrate(
		&model.Merchant{},
		&model.MerchantPlan{},
		&model.Store{},
		&model.StoreProduct{},
		&model.User{},
		&model.MembershipPackage{},
		&model.Promotion{},
		&model.Order{},
		&model.PaymentRecord{},
		&model.RefundRecord{},
		&model.RefundReconciliationAudit{},
		&model.MerchantSettlement{},
	); err != nil {
		t.Fatalf("GORM AutoMigrate synthetic MySQL schema: %v", err)
	}
	applyMySQLMigrationFile(t, db, "../../sql/migrations/20261113_add_refund_reservation_idempotency.sql")
	applyMySQLMigrationFile(t, db, "../../sql/migrations/20261113_add_refund_reconciliation_audits.sql")
	assertRefundIdempotencySchema(t, db)
	assertAuditTriggers(t, db)

	var engine string
	if err := db.Raw("SELECT engine FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'orders'").Scan(&engine).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(engine, "InnoDB") {
		t.Fatalf("row-lock tests require InnoDB orders table, got engine %q", engine)
	}
	return db, capture
}

func applyMySQLMigrationFile(t *testing.T, db *gorm.DB, relativePath string) {
	t.Helper()
	body, err := os.ReadFile(relativePath)
	if err != nil {
		t.Fatalf("read migration %s: %v", relativePath, err)
	}
	if err := executeMySQLScript(db, string(body)); err != nil {
		t.Fatalf("execute migration %s: %v", relativePath, err)
	}
}

// executeMySQLScript supports the MySQL client DELIMITER directive and keeps all
// statements on one pinned connection (required for session variables/PREPARE).
func executeMySQLScript(db *gorm.DB, script string) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	conn, err := sqlDB.Conn(context.Background())
	if err != nil {
		return err
	}
	defer conn.Close()

	delimiter := ";"
	var pending strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(script))
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "DELIMITER ") {
			delimiter = strings.TrimSpace(strings.TrimPrefix(trimmed, "DELIMITER "))
			continue
		}
		if strings.HasPrefix(trimmed, "--") || trimmed == "" {
			continue
		}
		pending.WriteString(line)
		pending.WriteByte('\n')
		if !strings.HasSuffix(trimmed, delimiter) {
			continue
		}
		statement := strings.TrimSpace(pending.String())
		statement = strings.TrimSpace(strings.TrimSuffix(statement, delimiter))
		if statement != "" {
			if _, err := conn.ExecContext(context.Background(), statement); err != nil {
				return fmt.Errorf("execute SQL statement: %w; SQL: %s", err, statement)
			}
		}
		pending.Reset()
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(pending.String()) != "" {
		return fmt.Errorf("unterminated SQL statement: %s", strings.TrimSpace(pending.String()))
	}
	return nil
}

func assertRefundIdempotencySchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, column := range []string{"idempotency_key", "request_fingerprint"} {
		var count int64
		if err := db.Raw("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'refund_records' AND column_name = ?", column).Scan(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("expected exactly one %s column, found %d", column, count)
		}
	}
	rows, err := db.Raw("SELECT column_name, seq_in_index FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'refund_records' AND index_name = 'idx_refund_order_idempotency' AND non_unique = 0 ORDER BY seq_in_index").Rows()
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var indexColumns []string
	for rows.Next() {
		var column string
		var sequence int
		if err := rows.Scan(&column, &sequence); err != nil {
			t.Fatal(err)
		}
		indexColumns = append(indexColumns, column)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(indexColumns) != 2 || indexColumns[0] != "order_id" || indexColumns[1] != "idempotency_key" {
		t.Fatalf("composite unique idempotency index mismatch: %v", indexColumns)
	}
}

func assertAuditTriggers(t *testing.T, db *gorm.DB) {
	t.Helper()
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM information_schema.triggers WHERE trigger_schema = DATABASE() AND trigger_name IN ('trg_refund_reconciliation_audits_no_update', 'trg_refund_reconciliation_audits_no_delete')").Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected both database append-only triggers, found %d", count)
	}
}

func mysqlDatabaseName(dsn string) string {
	marker := strings.LastIndex(dsn, ")/")
	if marker < 0 {
		return ""
	}
	return strings.SplitN(dsn[marker+2:], "?", 2)[0]
}

func mysqlDSNForDatabase(dsn, database string) string {
	marker := strings.LastIndex(dsn, ")/")
	if marker < 0 {
		return ""
	}
	query := ""
	if suffix := dsn[marker+2:]; strings.Contains(suffix, "?") {
		query = "?" + strings.SplitN(suffix, "?", 2)[1]
	}
	return dsn[:marker+2] + database + query
}

func TestMySQLRefundIdempotencyMigrationBeforeAutoMigrate(t *testing.T) {
	baseDSN := mysqlRefundTestDSN(t)
	baseDB, _ := newMySQLRefundIntegrationDB(t)
	baseSQL, err := baseDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("cue_refund_p1_before_%d", time.Now().UnixNano())
	if !strings.HasPrefix(name, "cue_refund_p1_") {
		t.Fatal("refusing to create database outside the synthetic prefix")
	}
	if _, err := baseSQL.Exec("CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		t.Fatalf("create disposable migration-order database: %v", err)
	}
	alt, err := gorm.Open(mysqlDriver.Open(mysqlDSNForDatabase(baseDSN, name)), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		_, _ = baseSQL.Exec("DROP DATABASE IF EXISTS `" + name + "`")
		t.Fatalf("open migration-order database: %v", err)
	}
	altSQL, err := alt.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = altSQL.Close()
		_, _ = baseSQL.Exec("DROP DATABASE IF EXISTS `" + name + "`")
	})

	if err := alt.Exec("CREATE TABLE refund_records (id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, created_at DATETIME(3) NULL, updated_at DATETIME(3) NULL, order_id BIGINT UNSIGNED NOT NULL, refund_no VARCHAR(60) NOT NULL, PRIMARY KEY (id), UNIQUE KEY idx_refund_records_refund_no (refund_no)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci").Error; err != nil {
		t.Fatalf("create synthetic legacy refund table: %v", err)
	}
	applyMySQLMigrationFile(t, alt, "../../sql/migrations/20261113_add_refund_reservation_idempotency.sql")
	if err := alt.AutoMigrate(&model.RefundRecord{}); err != nil {
		t.Fatalf("GORM AutoMigrate after SQL-first migration: %v", err)
	}
	assertRefundIdempotencySchema(t, alt)
}

func TestMySQLRefundMigrationsInteroperateAfterAutoMigrateAndRerun(t *testing.T) {
	db, _ := newMySQLRefundIntegrationDB(t)
	if err := db.AutoMigrate(&model.RefundRecord{}, &model.RefundReconciliationAudit{}); err != nil {
		t.Fatalf("repeat AutoMigrate: %v", err)
	}
	applyMySQLMigrationFile(t, db, "../../sql/migrations/20261113_add_refund_reservation_idempotency.sql")
	applyMySQLMigrationFile(t, db, "../../sql/migrations/20261113_add_refund_reconciliation_audits.sql")
	assertRefundIdempotencySchema(t, db)
	assertAuditTriggers(t, db)
}

func TestMySQLConcurrentDistinctPaymentTransactionsRecordOneDuplicateWithoutSecondFulfillment(t *testing.T) {
	db, sqlCapture := newMySQLRefundIntegrationDB(t)
	_, _, product, order := createSyntheticStoreOrder(t, db, uniqueSyntheticMySQLOrderNo(t, "PAYMENT"), "pending", 10000, nil)
	service := NewOrderService(db)
	start := make(chan struct{})
	txnPrefix := fmt.Sprintf("SYNTHETIC-MYSQL-TXN-%d", time.Now().UnixNano())
	type callbackResult struct {
		transaction string
		err         error
	}
	results := make(chan callbackResult, 2)
	for _, transaction := range []string{txnPrefix + "-A", txnPrefix + "-B"} {
		transaction := transaction
		go func() {
			<-start
			_, err := service.HandlePaymentCallback(dto.PaymentCallbackRequest{
				OrderNo: order.OrderNo, TransactionNo: transaction, PaymentChannel: "alipay", Amount: order.TotalAmount, Status: "success",
			})
			results <- callbackResult{transaction: transaction, err: err}
		}()
	}
	close(start)
	for i := 0; i < 2; i++ {
		select {
		case result := <-results:
			if result.err != nil {
				t.Fatalf("synthetic callback %s failed: %v", result.transaction, result.err)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("concurrent payment callbacks timed out")
		}
	}

	var current model.Order
	if err := db.First(&current, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	var paymentStatuses []string
	if err := db.Model(&model.PaymentRecord{}).Where("order_id = ?", order.ID).Order("transaction_no").Pluck("status", &paymentStatuses).Error; err != nil {
		t.Fatal(err)
	}
	var updatedProduct model.StoreProduct
	if err := db.First(&updatedProduct, product.ID).Error; err != nil {
		t.Fatal(err)
	}
	if len(paymentStatuses) != 2 || countString(paymentStatuses, "success") != 1 || countString(paymentStatuses, "duplicate_success") != 1 {
		t.Fatalf("two distinct successful transactions must yield one normal payment and one visible duplicate exception; statuses=%v", paymentStatuses)
	}
	if current.PaidAt == nil || current.Status != "received" || !orderHasAction(current.OperationLogs, "duplicate_payment_exception") {
		t.Fatalf("order fulfillment/duplicate exception mismatch: status=%q paid_at=%v logs=%s", current.Status, current.PaidAt, current.OperationLogs)
	}
	if updatedProduct.Stock == nil || *updatedProduct.Stock != 10 {
		t.Fatalf("payment callbacks must preserve legacy stock; stock=%v", updatedProduct.Stock)
	}
	observed := sqlCapture.String()
	if !strings.Contains(strings.ToUpper(observed), "FOR UPDATE") {
		t.Fatal("GORM emitted no FOR UPDATE SQL while processing concurrent callbacks")
	}
	lockQuery := ""
	for _, line := range strings.Split(observed, "\n") {
		if strings.Contains(strings.ToUpper(line), "FOR UPDATE") && strings.Contains(line, "orders") {
			lockQuery = strings.TrimSpace(line)
			break
		}
	}
	t.Logf("MySQL/InnoDB lock verified; generated query: %s", lockQuery)
	t.Logf("payment_records statuses=%v; order.status=%s; legacy_stock=%d", paymentStatuses, current.Status, *updatedProduct.Stock)
}

func TestMySQLConcurrentPartialRefundsNeverReserveMoreThanRemaining(t *testing.T) {
	db, _ := newMySQLRefundIntegrationDB(t)
	merchant, _, _, order := createSyntheticStoreOrder(t, db, uniqueSyntheticMySQLOrderNo(t, "REFUND-CAP"), "accepted", 10000, timePtr(time.Now().UTC()))
	order.TransactionNo = "SYNTHETIC-MYSQL-PAID-TXN-CAP"
	order.RefundedAmount = 6000
	if err := db.Save(&order).Error; err != nil {
		t.Fatal(err)
	}
	gateway := newBlockingSyntheticRefundGateway()
	defer gateway.releaseAll()
	service := NewOrderService(db, &AlipayService{})
	service.alipay = gateway
	firstResult := make(chan error, 1)
	go func() {
		_, err := service.RefundMerchantOrder(merchant.ID, order.ID, dto.RefundOrderRequest{Amount: 3000, Reason: "合成部分退款 A", IdempotencyKey: "synthetic-cap-key-a"}, 41, "merchant")
		firstResult <- err
	}()
	waitForSyntheticDispatch(t, gateway)

	secondResult := make(chan error, 1)
	go func() {
		_, err := service.RefundMerchantOrder(merchant.ID, order.ID, dto.RefundOrderRequest{Amount: 2500, Reason: "合成部分退款 B", IdempotencyKey: "synthetic-cap-key-b"}, 42, "merchant")
		secondResult <- err
	}()
	select {
	case err := <-secondResult:
		if err == nil || !strings.Contains(err.Error(), "exceeds remaining refundable amount") {
			t.Fatalf("second partial refund should be rejected against the first reservation: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("second partial refund did not complete while first provider call was paused")
	}
	gateway.releaseAll()
	if err := <-firstResult; err == nil || !strings.Contains(err.Error(), "outcome is unknown") {
		t.Fatalf("synthetic provider timeout should leave the first reservation unknown: %v", err)
	}

	var refunds []model.RefundRecord
	if err := db.Where("order_id = ?", order.ID).Find(&refunds).Error; err != nil {
		t.Fatal(err)
	}
	var reserved int64
	for _, refund := range refunds {
		if refund.Status == "processing" || refund.Status == "unknown" {
			reserved += refund.Amount
		}
	}
	if len(refunds) != 1 || reserved > order.TotalAmount-order.RefundedAmount {
		t.Fatalf("concurrent partial refunds over-reserved: count=%d reserved=%d remaining=%d rows=%+v", len(refunds), reserved, order.TotalAmount-order.RefundedAmount, refunds)
	}
	t.Logf("refund reservations=%d; reserved=%d; remaining=%d; provider dispatches=%d", len(refunds), reserved, order.TotalAmount-order.RefundedAmount, gateway.calls.Load())
}

func TestMySQLSameRefundIdempotencyKeyDoesNotDispatchTwice(t *testing.T) {
	db, _ := newMySQLRefundIntegrationDB(t)
	merchant, _, _, order := createSyntheticStoreOrder(t, db, uniqueSyntheticMySQLOrderNo(t, "IDEMPOTENCY"), "accepted", 10000, timePtr(time.Now().UTC()))
	order.TransactionNo = "SYNTHETIC-MYSQL-PAID-TXN-IDEM"
	if err := db.Save(&order).Error; err != nil {
		t.Fatal(err)
	}
	gateway := newBlockingSyntheticRefundGateway()
	defer gateway.releaseAll()
	service := NewOrderService(db, &AlipayService{})
	service.alipay = gateway
	request := dto.RefundOrderRequest{Amount: 2000, Reason: "同键合成退款", IdempotencyKey: "synthetic-same-key"}
	firstResult := make(chan error, 1)
	go func() {
		_, err := service.RefundMerchantOrder(merchant.ID, order.ID, request, 51, "merchant")
		firstResult <- err
	}()
	waitForSyntheticDispatch(t, gateway)

	_, secondErr := service.RefundMerchantOrder(merchant.ID, order.ID, request, 51, "merchant")
	if secondErr == nil || !strings.Contains(secondErr.Error(), "must be reconciled") {
		t.Fatalf("same idempotency key while processing should not dispatch again: %v", secondErr)
	}
	gateway.releaseAll()
	if err := <-firstResult; err == nil || !strings.Contains(err.Error(), "outcome is unknown") {
		t.Fatalf("first synthetic refund should be recorded as unknown: %v", err)
	}
	if got := gateway.calls.Load(); got != 1 {
		t.Fatalf("same idempotency key dispatched %d times, want 1", got)
	}
	var count int64
	if err := db.Model(&model.RefundRecord{}).Where("order_id = ? AND idempotency_key = ?", order.ID, request.IdempotencyKey).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("same key must create exactly one durable reservation, found %d", count)
	}
	t.Logf("idempotency key reservation rows=1; synthetic provider dispatches=%d", gateway.calls.Load())
}

func TestMySQLUnknownRefundReservationBlocksAnotherRequest(t *testing.T) {
	db, _ := newMySQLRefundIntegrationDB(t)
	merchant, _, _, order := createSyntheticStoreOrder(t, db, uniqueSyntheticMySQLOrderNo(t, "UNKNOWN-BLOCK"), "accepted", 10000, timePtr(time.Now().UTC()))
	order.TransactionNo = "SYNTHETIC-MYSQL-PAID-TXN-UNKNOWN"
	if err := db.Save(&order).Error; err != nil {
		t.Fatal(err)
	}
	unknown := createUnknownRefundReservation(t, db, order, uniqueSyntheticMySQLOrderNo(t, "UNKNOWN-REFUND"), 3000)
	service := NewOrderService(db)
	_, err := service.RefundMerchantOrder(merchant.ID, order.ID, dto.RefundOrderRequest{Amount: 500, Reason: "未知预留阻断检查", IdempotencyKey: "synthetic-after-unknown"}, 61, "merchant")
	if err == nil || !strings.Contains(err.Error(), "must be reconciled before any further refund") {
		t.Fatalf("unknown reservation must block a new request: %v", err)
	}
	var rows int64
	if err := db.Model(&model.RefundRecord{}).Where("order_id = ?", order.ID).Count(&rows).Error; err != nil {
		t.Fatal(err)
	}
	var current model.RefundRecord
	if err := db.First(&current, unknown.ID).Error; err != nil {
		t.Fatal(err)
	}
	if rows != 1 || current.Status != "unknown" {
		t.Fatalf("blocked request must preserve exactly the unknown reservation: rows=%d status=%q", rows, current.Status)
	}
	t.Logf("unknown reservation retained; refund rows=%d; second request rejected", rows)
}

func TestMySQLReconciliationAuditIsAtomicAndDatabaseAppendOnly(t *testing.T) {
	db, _ := newMySQLRefundIntegrationDB(t)
	paidAt := time.Now().UTC()
	_, _, _, order := createSyntheticStoreOrder(t, db, uniqueSyntheticMySQLOrderNo(t, "AUDIT"), "accepted", 10000, &paidAt)
	unknown := createUnknownRefundReservation(t, db, order, uniqueSyntheticMySQLOrderNo(t, "AUDIT-REFUND"), 2500)
	service := NewOrderService(db)
	request := dto.RefundReconciliationRequest{Outcome: "success", ProviderReference: "SYNTHETIC-CHANNEL-REF-ATOMIC", Reason: "合成 MySQL 审计事务测试"}
	if err := db.Exec("CREATE TRIGGER trg_test_abort_refund_audit BEFORE INSERT ON refund_reconciliation_audits FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'synthetic audit failure'").Error; err != nil {
		t.Fatalf("install synthetic rollback trigger: %v", err)
	}
	_, transitionErr := service.ResolveUnknownRefund(unknown.ID, request, 71, "admin")
	if transitionErr == nil {
		t.Fatal("audit insert failure must fail the reconciliation transaction")
	}
	if err := db.Exec("DROP TRIGGER trg_test_abort_refund_audit").Error; err != nil {
		t.Fatal(err)
	}
	var afterFailure model.Order
	if err := db.First(&afterFailure, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	var refundAfterFailure model.RefundRecord
	if err := db.First(&refundAfterFailure, unknown.ID).Error; err != nil {
		t.Fatal(err)
	}
	var auditCount int64
	if err := db.Model(&model.RefundReconciliationAudit{}).Where("refund_id = ?", unknown.ID).Count(&auditCount).Error; err != nil {
		t.Fatal(err)
	}
	if refundAfterFailure.Status != "unknown" || afterFailure.RefundedAmount != 0 || afterFailure.RefundStatus != "processing" || auditCount != 0 {
		t.Fatalf("audit failure did not roll back atomically: refund=%q refunded=%d refund_status=%q audits=%d", refundAfterFailure.Status, afterFailure.RefundedAmount, afterFailure.RefundStatus, auditCount)
	}

	detail, err := service.ResolveUnknownRefund(unknown.ID, request, 71, "admin")
	if err != nil {
		t.Fatalf("successful synthetic reconciliation: %v", err)
	}
	if detail.ReservationStatus != "success" || len(detail.Audit) != 1 {
		t.Fatalf("committed transition/audit mismatch: status=%q audit_count=%d", detail.ReservationStatus, len(detail.Audit))
	}
	audit := detail.Audit[0]
	if err := db.Exec("UPDATE refund_reconciliation_audits SET reason = 'rewritten by direct SQL' WHERE id = ?", audit.ID).Error; err == nil {
		t.Fatal("database UPDATE trigger accepted a direct mutation")
	}
	if err := db.Exec("DELETE FROM refund_reconciliation_audits WHERE id = ?", audit.ID).Error; err == nil {
		t.Fatal("database DELETE trigger accepted a direct mutation")
	}
	var immutable model.RefundReconciliationAudit
	if err := db.First(&immutable, audit.ID).Error; err != nil {
		t.Fatal(err)
	}
	var afterSuccess model.Order
	if err := db.First(&afterSuccess, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	if immutable.Reason != request.Reason || afterSuccess.RefundedAmount != 2500 || afterSuccess.RefundStatus != "partial" {
		t.Fatalf("committed audit/order state mismatch: reason=%q refunded=%d status=%q", immutable.Reason, afterSuccess.RefundedAmount, afterSuccess.RefundStatus)
	}
	t.Logf("audit transition committed atomically; direct SQL UPDATE and DELETE rejected; refunded_amount=%d", afterSuccess.RefundedAmount)
}

func TestMySQLStaleProcessingRefundManualReviewIsAuditedAndIdempotent(t *testing.T) {
	db, _ := newMySQLRefundIntegrationDB(t)
	paidAt := time.Now().UTC()
	_, _, _, order := createSyntheticStoreOrder(t, db, uniqueSyntheticMySQLOrderNo(t, "STALE-MANUAL"), "accepted", 10000, &paidAt)
	order.RefundStatus = "processing"
	if err := db.Save(&order).Error; err != nil {
		t.Fatal(err)
	}
	key := "synthetic-mysql-stale-manual-review"
	refund := model.RefundRecord{OrderID: order.ID, RefundNo: uniqueSyntheticMySQLOrderNo(t, "STALE-REFUND"), IdempotencyKey: &key, Amount: 2400, Reason: "合成 MySQL 逾期预留", Status: "processing"}
	if err := db.Create(&refund).Error; err != nil {
		t.Fatalf("create synthetic MySQL reservation: %v", err)
	}
	if err := db.Model(&model.RefundRecord{}).Where("id = ?", refund.ID).UpdateColumn("updated_at", time.Now().UTC().Add(-20*time.Minute)).Error; err != nil {
		t.Fatalf("age synthetic MySQL reservation: %v", err)
	}
	service := NewOrderService(db)
	queue, err := service.UnknownRefundReservations()
	if err != nil {
		t.Fatal(err)
	}
	staleFound := false
	for _, item := range queue {
		if item.RefundID == refund.ID {
			staleFound = item.IsStale && item.ReservationStatus == "processing" && item.StaleAfterMinutes == 15
		}
	}
	if !staleFound {
		t.Fatalf("MySQL stale reservation should appear overdue in the admin queue: %+v", queue)
	}
	note := "合成 MySQL 核验：超时预留转人工，金额继续保留"
	if err := db.Exec("CREATE TRIGGER trg_test_abort_stale_refund_audit BEFORE INSERT ON refund_reconciliation_audits FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'synthetic stale audit failure'").Error; err != nil {
		t.Fatal(err)
	}
	_, transitionErr := service.MarkStaleProcessingRefundUnknown(refund.ID, note, 81, "super_admin")
	if err := db.Exec("DROP TRIGGER trg_test_abort_stale_refund_audit").Error; err != nil {
		t.Fatal(err)
	}
	if transitionErr == nil {
		t.Fatal("MySQL audit failure must reject the stale reservation handoff")
	}
	var afterAuditFailure model.RefundRecord
	if err := db.First(&afterAuditFailure, refund.ID).Error; err != nil {
		t.Fatal(err)
	}
	if afterAuditFailure.Status != "processing" {
		t.Fatalf("MySQL audit failure must roll back the stale status transition, got %q", afterAuditFailure.Status)
	}
	detail, err := service.MarkStaleProcessingRefundUnknown(refund.ID, note, 81, "super_admin")
	if err != nil {
		t.Fatalf("move synthetic stale reservation to manual review: %v", err)
	}
	if detail.ReservationStatus != "unknown" || len(detail.Audit) != 1 || detail.Audit[0].OldStatus != "processing" || detail.Audit[0].NewStatus != "unknown" || detail.Audit[0].ActorID != 81 || detail.Audit[0].ActorRole != "super_admin" || detail.Audit[0].Reason != note || detail.Audit[0].ProviderReference != "" {
		t.Fatalf("MySQL manual-review audit mismatch: %+v", detail)
	}
	var currentOrder model.Order
	if err := db.First(&currentOrder, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	var reserved int64
	if err := db.Model(&model.RefundRecord{}).Where("order_id = ? AND status IN ?", order.ID, []string{"processing", "unknown"}).Select("COALESCE(SUM(amount), 0)").Scan(&reserved).Error; err != nil {
		t.Fatal(err)
	}
	if currentOrder.RefundedAmount != 0 || currentOrder.RefundStatus != "processing" || reserved != refund.Amount {
		t.Fatalf("MySQL transition must keep the full reservation without claiming a refund: refunded=%d order_status=%q reserved=%d", currentOrder.RefundedAmount, currentOrder.RefundStatus, reserved)
	}
	for _, lateOutcome := range []string{"success", "failed"} {
		if _, err := service.finishRefundReservation(refund.ID, lateOutcome, "synthetic MySQL late provider result"); err == nil || !strings.Contains(err.Error(), "not processing") {
			t.Fatalf("MySQL late provider outcome %q must not overwrite unknown: %v", lateOutcome, err)
		}
	}
	retry, err := service.MarkStaleProcessingRefundUnknown(refund.ID, note, 81, "super_admin")
	if err != nil || len(retry.Audit) != 1 || retry.ReservationStatus != "unknown" {
		t.Fatalf("MySQL exact retry must remain idempotent: detail=%+v error=%v", retry, err)
	}
	var auditCount int64
	if err := db.Model(&model.RefundReconciliationAudit{}).Where("refund_id = ?", refund.ID).Count(&auditCount).Error; err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("MySQL retry must not duplicate audit, count=%d", auditCount)
	}
	t.Logf("stale reservation moved to unknown; %d cents remain reserved; audit rows=%d; provider calls=0", reserved, auditCount)
}

func waitForSyntheticDispatch(t *testing.T, gateway *blockingSyntheticRefundGateway) {
	t.Helper()
	select {
	case <-gateway.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("synthetic refund did not reach the fake provider")
	}
}

func uniqueSyntheticMySQLOrderNo(t *testing.T, label string) string {
	t.Helper()
	return fmt.Sprintf("SYNTH-MY-%s-%d", label, time.Now().UnixNano())
}

func timePtr(value time.Time) *time.Time { return &value }

func countString(values []string, target string) int {
	count := 0
	for _, value := range values {
		if value == target {
			count++
		}
	}
	return count
}

var _ interface{ Write([]byte) (int, error) } = (*mysqlSQLCapture)(nil)

func TestMySQLStaleProcessingRefundMissingUpdateTimeUsesCreationAge(t *testing.T) {
	db, _ := newMySQLRefundIntegrationDB(t)
	_, _, _, order := createSyntheticStoreOrder(t, db, uniqueSyntheticMySQLOrderNo(t, "STALE-NULL-TIME"), "accepted", 10000, timePtr(time.Now().UTC()))
	refund := model.RefundRecord{OrderID: order.ID, RefundNo: uniqueSyntheticMySQLOrderNo(t, "STALE-NULL-REFUND"), Amount: 1700, Reason: "合成 MySQL 历史空更新时间", Status: "processing"}
	if err := db.Create(&refund).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.RefundRecord{}).Where("id = ?", refund.ID).UpdateColumns(map[string]any{"created_at": time.Now().UTC().Add(-20 * time.Minute), "updated_at": nil}).Error; err != nil {
		t.Fatal(err)
	}
	service := NewOrderService(db)
	detail, err := service.RefundReconciliationDetail(refund.ID)
	if err != nil || !detail.IsStale {
		t.Fatalf("MySQL missing update time must use creation age: detail=%+v error=%v", detail, err)
	}
	detail, err = service.MarkStaleProcessingRefundUnknown(refund.ID, "合成 MySQL 历史逾期预留转人工核对", 81, "super_admin")
	if err != nil || detail.ReservationStatus != "unknown" || len(detail.Audit) != 1 || detail.Amount != 1700 {
		t.Fatalf("MySQL missing update time handoff failed: detail=%+v error=%v", detail, err)
	}
}
