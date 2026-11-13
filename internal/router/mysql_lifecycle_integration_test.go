package router

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	mysqlDriver "github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"

	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/schema"
	"go-web-gin-health/internal/utils"
)

const (
	lifecycleDSNEnv      = "SCHEMA_MIGRATION_IT_DSN"
	lifecycleRootEnv     = "SCHEMA_MIGRATION_IT_ROOT"
	lifecycleMigratorEnv = "SCHEMA_MIGRATION_IT_MIGRATOR_BIN"
)

// TestMySQLManualReviewBackupRestoreLifecycle is opt-in. It refuses TCP,
// credentialed DSNs, server datadirs outside the caller's unique temporary
// root, and any server that has not disabled networking.
func TestMySQLManualReviewBackupRestoreLifecycle(t *testing.T) {
	if strings.TrimSpace(os.Getenv(lifecycleDSNEnv)) == "" {
		t.Skipf("set %s and %s to a disposable socket-only MySQL instance to run", lifecycleDSNEnv, lifecycleRootEnv)
	}
	if strings.TrimSpace(os.Getenv("MYSQL_DSN")) != "" {
		t.Fatal("refusing to run while MYSQL_DSN is set; this harness must not use an application or production DSN")
	}

	control, baseCfg, root, socket, migrator := openSafeLifecycleMySQL(t)
	t.Cleanup(func() { _ = control.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	sourceName := fmt.Sprintf("cue_schema_migration_it_source_%d", time.Now().UnixNano())
	targetName := fmt.Sprintf("cue_schema_migration_it_restore_%d", time.Now().UnixNano())
	source := createLifecycleDatabase(t, control, baseCfg, sourceName)

	if result, err := schema.ApplyCandidate(ctx, source); err != nil || result != schema.ResultApplied {
		t.Fatalf("install exact v1 baseline: result=%q err=%v", result, err)
	}
	if result, err := schema.ApplyCandidate(ctx, source); err != nil || result != schema.ResultAlready {
		t.Fatalf("idempotent v1 rerun: result=%q err=%v", result, err)
	}
	merchantID, orderID, orderNo := insertLifecycleV1Order(t, source)

	migrateDSN := baseCfg.Clone()
	migrateDSN.DBName = sourceName
	runSchemaMigrator(t, root, migrator, migrateDSN.FormatDSN(), "applied")
	runSchemaMigrator(t, root, migrator, migrateDSN.FormatDSN(), "already-managed")
	assertLifecycleInventory(t, source)

	reviewID, auditID := insertLifecycleManualReview(t, source, merchantID, orderID, orderNo)
	assertLifecycleData(t, source, orderID, orderNo, reviewID, auditID)
	assertAppendOnlyLifecycle(t, source, reviewID, auditID)

	dumpFile, err := os.CreateTemp(root, "synthetic-mysql-backup-*.sql")
	if err != nil {
		t.Fatalf("create dump file inside isolated temp root: %v", err)
	}
	dumpPath := dumpFile.Name()
	if err := dumpFile.Close(); err != nil {
		t.Fatalf("close temporary dump file: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(dumpPath) })
	if !pathIsWithin(t, dumpPath, root) {
		t.Fatalf("refusing dump path outside isolated temp root")
	}

	dump := exec.CommandContext(ctx, "mysqldump",
		"--no-defaults", "--protocol=SOCKET", "--socket="+socket, "--user=root",
		"--default-character-set=utf8mb4", "--single-transaction", "--routines", "--triggers",
		sourceName,
	)
	dump.Env = isolatedCommandEnv(root)
	var dumpStderr bytes.Buffer
	dump.Stderr = &dumpStderr
	dumpOut, err := os.OpenFile(dumpPath, os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		t.Fatalf("open isolated dump file: %v", err)
	}
	dump.Stdout = dumpOut
	dumpErr := dump.Run()
	closeErr := dumpOut.Close()
	if dumpErr != nil {
		t.Fatalf("mysqldump from isolated socket failed: %v (%s)", dumpErr, strings.TrimSpace(dumpStderr.String()))
	}
	if closeErr != nil {
		t.Fatalf("close dump output: %v", closeErr)
	}
	if info, err := os.Stat(dumpPath); err != nil || info.Size() == 0 {
		t.Fatalf("isolated mysqldump is empty or unreadable: stat err=%v", err)
	}

	target := createLifecycleDatabase(t, control, baseCfg, targetName)
	if err := target.PingContext(ctx); err != nil {
		t.Fatalf("ping fresh restore schema: %v", err)
	}
	input, err := os.Open(dumpPath)
	if err != nil {
		t.Fatalf("open isolated dump for restore: %v", err)
	}
	restore := exec.CommandContext(ctx, "mysql",
		"--no-defaults", "--protocol=SOCKET", "--socket="+socket, "--user=root",
		"--default-character-set=utf8mb4", targetName,
	)
	restore.Env = isolatedCommandEnv(root)
	restore.Stdin = input
	var restoreStderr bytes.Buffer
	restore.Stderr = &restoreStderr
	restoreErr := restore.Run()
	inputCloseErr := input.Close()
	if restoreErr != nil {
		t.Fatalf("restore isolated mysqldump into second fresh schema failed: %v (%s)", restoreErr, strings.TrimSpace(restoreStderr.String()))
	}
	if inputCloseErr != nil {
		t.Fatalf("close isolated dump input: %v", inputCloseErr)
	}

	if result, err := schema.ApplyLatest(ctx, target); err != nil || result != schema.ResultAlready {
		t.Fatalf("verify restored managed schema and fingerprints: result=%q err=%v", result, err)
	}
	assertLifecycleInventory(t, target)
	assertMatchingLedgers(t, source, target)
	assertLifecycleData(t, target, orderID, orderNo, reviewID, auditID)
	assertAppendOnlyLifecycle(t, target, reviewID, auditID)
	assertLifecycleReadEndpoints(t, targetName, orderNo)
}

func openSafeLifecycleMySQL(t *testing.T) (*sql.DB, *mysqlDriver.Config, string, string, string) {
	t.Helper()
	raw := strings.TrimSpace(os.Getenv(lifecycleDSNEnv))
	rootValue := strings.TrimSpace(os.Getenv(lifecycleRootEnv))
	migrator := strings.TrimSpace(os.Getenv(lifecycleMigratorEnv))
	if raw == "" || rootValue == "" || migrator == "" {
		t.Fatalf("require explicit isolated %s, %s, and %s", lifecycleDSNEnv, lifecycleRootEnv, lifecycleMigratorEnv)
	}
	root, err := filepath.EvalSymlinks(rootValue)
	if err != nil {
		t.Fatalf("resolve isolated temporary root: %v", err)
	}
	if !filepath.IsAbs(root) || !strings.HasPrefix(root, "/tmp/cue-schema-migrate-") {
		t.Fatal("refusing lifecycle root outside a unique /tmp/cue-schema-migrate-* directory")
	}
	rootInfo, err := os.Lstat(rootValue)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 || filepath.Clean(rootValue) != root {
		t.Fatal("refusing non-canonical or symlinked isolated temporary root")
	}

	cfg, err := mysqlDriver.ParseDSN(raw)
	if err != nil {
		t.Fatal("refusing malformed lifecycle DSN")
	}
	if cfg.Net != "unix" || cfg.User != "root" || cfg.Passwd != "" || !strings.HasPrefix(cfg.DBName, "cue_schema_migration_it_") {
		t.Fatal("refusing lifecycle DSN: require passwordless root over Unix socket and a cue_schema_migration_it_* database name")
	}
	socket := filepath.Clean(cfg.Addr)
	if !filepath.IsAbs(socket) || !pathWithinRoot(socket, root) {
		t.Fatal("refusing lifecycle socket outside the explicit isolated temporary root")
	}
	migratorPath, err := filepath.EvalSymlinks(migrator)
	if err != nil || !pathWithinRoot(migratorPath, root) {
		t.Fatal("refusing migrator binary outside the isolated temporary root")
	}
	if info, err := os.Stat(migratorPath); err != nil || !info.Mode().IsRegular() || info.Mode()&0111 == 0 {
		t.Fatal("isolated schema-migrator binary is missing or not executable")
	}

	controlCfg := cfg.Clone()
	controlCfg.DBName = ""
	control, err := sql.Open("mysql", controlCfg.FormatDSN())
	if err != nil {
		t.Fatalf("open isolated Unix-socket MySQL control connection: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := control.PingContext(ctx); err != nil {
		control.Close()
		t.Fatalf("connect to isolated Unix-socket MySQL: %v", err)
	}
	var skipNetworking int
	var datadir, serverSocket, version string
	if err := control.QueryRowContext(ctx, "SELECT @@skip_networking, @@datadir, @@socket, VERSION()").Scan(&skipNetworking, &datadir, &serverSocket, &version); err != nil {
		control.Close()
		t.Fatalf("inspect isolated MySQL safety settings: %v", err)
	}
	resolvedDatadir, err := filepath.EvalSymlinks(filepath.Clean(datadir))
	if err != nil || !pathWithinRoot(resolvedDatadir, root) {
		control.Close()
		t.Fatal("refusing MySQL server whose datadir is outside the explicit isolated temporary root")
	}
	resolvedSocket, err := filepath.Abs(filepath.Clean(serverSocket))
	if err != nil || resolvedSocket != socket || !pathWithinRoot(resolvedSocket, root) {
		control.Close()
		t.Fatal("refusing MySQL server whose socket does not match the explicit isolated socket path")
	}
	if skipNetworking != 1 {
		control.Close()
		t.Fatalf("refusing MySQL server with TCP/networking enabled (skip_networking=%d)", skipNetworking)
	}
	if !strings.HasPrefix(version, "8.") {
		control.Close()
		t.Fatalf("requires MySQL 8.x; server reports %q", version)
	}
	return control, cfg, root, socket, migratorPath
}

func createLifecycleDatabase(t *testing.T, control *sql.DB, base *mysqlDriver.Config, name string) *sql.DB {
	t.Helper()
	if !strings.HasPrefix(name, "cue_schema_migration_it_") || strings.ContainsAny(name, "`\x00") {
		t.Fatal("refusing database name outside the synthetic lifecycle prefix")
	}
	var exists int
	if err := control.QueryRow("SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME=?", name).Scan(&exists); err != nil {
		t.Fatalf("check synthetic database name before creation: %v", err)
	}
	if exists != 0 {
		t.Fatalf("refusing to reuse pre-existing database %q", name)
	}
	if _, err := control.Exec("CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		t.Fatalf("create fresh synthetic database: %v", err)
	}
	cfg := base.Clone()
	cfg.DBName = name
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		_, _ = control.Exec("DROP DATABASE `" + name + "`")
		t.Fatalf("open fresh synthetic database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		_, _ = control.Exec("DROP DATABASE `" + name + "`")
	})
	if err := db.Ping(); err != nil {
		t.Fatalf("ping fresh synthetic database: %v", err)
	}
	return db
}

func insertLifecycleV1Order(t *testing.T, db *sql.DB) (uint64, uint64, string) {
	t.Helper()
	merchantResult, err := db.Exec(`INSERT INTO merchants(name, status, subscription_status) VALUES('synthetic-lifecycle-merchant','active','inactive')`)
	if err != nil {
		t.Fatalf("insert synthetic v1 merchant: %v", err)
	}
	merchantID, err := merchantResult.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	orderNo := fmt.Sprintf("SYNTHETIC-LIFECYCLE-%d", time.Now().UnixNano())
	orderResult, err := db.Exec(`INSERT INTO orders(order_no, order_type, merchant_id, amount, total_amount, status) VALUES(?, 'merchant_subscription', ?, 19900, 19900, 'pending')`, orderNo, merchantID)
	if err != nil {
		t.Fatalf("insert synthetic v1 subscription order: %v", err)
	}
	orderID, err := orderResult.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return uint64(merchantID), uint64(orderID), orderNo
}

func insertLifecycleManualReview(t *testing.T, db *sql.DB, merchantID, orderID uint64, orderNo string) (uint64, uint64) {
	t.Helper()
	result, err := db.Exec(`INSERT INTO merchant_subscription_manual_reviews(
review_key, order_id, order_no, merchant_id, actor_id, actor_role, plan_name,
amount_cents, duration_days, payment_occurred_attested, evidence_reference, reason,
request_fingerprint, review_status, created_at, updated_at
) VALUES(?, ?, ?, ?, 17, 'admin', 'synthetic-original-plan', 19900, 30, 1,
'synthetic-evidence-001', 'synthetic immutable review', REPEAT('a',64), 'manually_verified', NOW(3), NOW(3))`,
		fmt.Sprintf("MSUB-MANUAL-REVIEW-%d", orderID), orderID, orderNo, merchantID)
	if err != nil {
		t.Fatalf("insert synthetic immutable manual review: %v", err)
	}
	reviewID, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	result, err = db.Exec(`INSERT INTO merchant_subscription_manual_review_audits(
review_id, order_id, merchant_id, actor_id, actor_role, evidence_reference, reason,
plan_name, amount_cents, duration_days, payment_occurred_attested,
order_status_before, order_status_after, order_paid_at_before, order_paid_at_after,
order_updated_at_before, order_updated_at_after, subscription_status_before,
subscription_status_after, subscription_plan_before, subscription_plan_after,
subscription_expire_at_before, subscription_expire_at_after, reviewed_at, created_at
) VALUES(?, ?, ?, 17, 'admin', 'synthetic-evidence-001', 'synthetic audit',
'synthetic-original-plan', 19900, 30, 1, 'pending', 'manually_verified', NULL, NULL,
NOW(3), NOW(3), 'inactive', 'active', 'none', 'historical-manual-review', NULL,
DATE_ADD(NOW(3), INTERVAL 30 DAY), NOW(3), NOW(3))`, reviewID, orderID, merchantID)
	if err != nil {
		t.Fatalf("insert synthetic immutable manual-review audit: %v", err)
	}
	auditID, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE orders SET status='manually_verified', payment_channel='admin_manual_verified', paid_at=NULL, transaction_no='', updated_at=NOW(3) WHERE id=? AND status='pending'`, orderID); err != nil {
		t.Fatalf("set synthetic order to manually_verified without provider payment: %v", err)
	}
	return uint64(reviewID), uint64(auditID)
}

func assertLifecycleInventory(t *testing.T, db *sql.DB) {
	t.Helper()
	var tables, triggers, v1Rows, v2Rows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_TYPE='BASE TABLE'`).Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE()`).Scan(&triggers); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM app_schema_migrations WHERE version=? AND checksum=? AND state='applied' AND schema_fingerprint IS NOT NULL`, schema.BaselineVersion, schema.BaselineChecksum).Scan(&v1Rows); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM app_schema_migrations WHERE version=? AND checksum=? AND state='applied' AND schema_fingerprint IS NOT NULL`, schema.ManualReviewV2Version, schema.ManualReviewV2Checksum).Scan(&v2Rows); err != nil {
		t.Fatal(err)
	}
	if tables != 32 || triggers != 6 || v1Rows != 1 || v2Rows != 1 {
		t.Fatalf("managed schema inventory tables/triggers/v1/v2=%d/%d/%d/%d; want 32/6/1/1", tables, triggers, v1Rows, v2Rows)
	}
}

func assertLifecycleData(t *testing.T, db *sql.DB, orderID uint64, orderNo string, reviewID, auditID uint64) {
	t.Helper()
	var orders, reviews, audits, payments int
	if err := db.QueryRow(`SELECT COUNT(*) FROM orders WHERE id=? AND order_no=? AND status='manually_verified' AND payment_channel='admin_manual_verified' AND paid_at IS NULL AND transaction_no=''`, orderID, orderNo).Scan(&orders); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM merchant_subscription_manual_reviews WHERE id=? AND order_id=? AND order_no=? AND review_status='manually_verified' AND evidence_reference='synthetic-evidence-001' AND reason='synthetic immutable review'`, reviewID, orderID, orderNo).Scan(&reviews); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM merchant_subscription_manual_review_audits WHERE id=? AND review_id=? AND order_id=? AND order_status_before='pending' AND order_status_after='manually_verified' AND order_paid_at_after IS NULL AND payment_occurred_attested=1`, auditID, reviewID, orderID).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM payment_records WHERE order_id=?`, orderID).Scan(&payments); err != nil {
		t.Fatal(err)
	}
	if orders != 1 || reviews != 1 || audits != 1 || payments != 0 {
		t.Fatalf("synthetic lifecycle order/review/audit/provider-payment counts=%d/%d/%d/%d; want 1/1/1/0", orders, reviews, audits, payments)
	}
}

func assertAppendOnlyLifecycle(t *testing.T, db *sql.DB, reviewID, auditID uint64) {
	t.Helper()
	for _, statement := range []struct {
		label string
		sql   string
		id    uint64
	}{
		{"review UPDATE", `UPDATE merchant_subscription_manual_reviews SET reason='tampered' WHERE id=?`, reviewID},
		{"review DELETE", `DELETE FROM merchant_subscription_manual_reviews WHERE id=?`, reviewID},
		{"audit UPDATE", `UPDATE merchant_subscription_manual_review_audits SET reason='tampered' WHERE id=?`, auditID},
		{"audit DELETE", `DELETE FROM merchant_subscription_manual_review_audits WHERE id=?`, auditID},
	} {
		_, err := db.Exec(statement.sql, statement.id)
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "append-only") {
			t.Fatalf("append-only database trigger did not reject %s with its expected message: %v", statement.label, err)
		}
	}
	if countLifecycle(t, db, `SELECT COUNT(*) FROM merchant_subscription_manual_reviews WHERE id=? AND reason='synthetic immutable review'`, reviewID) != 1 ||
		countLifecycle(t, db, `SELECT COUNT(*) FROM merchant_subscription_manual_review_audits WHERE id=? AND reason='synthetic audit'`, auditID) != 1 {
		t.Fatal("append-only rejection changed the synthetic manual review or audit")
	}
}

func assertMatchingLedgers(t *testing.T, source, restored *sql.DB) {
	t.Helper()
	read := func(db *sql.DB) map[string][3]string {
		rows, err := db.Query(`SELECT version, checksum, state, schema_fingerprint FROM app_schema_migrations ORDER BY version`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		out := make(map[string][3]string)
		for rows.Next() {
			var version, checksum, state, fingerprint string
			if err := rows.Scan(&version, &checksum, &state, &fingerprint); err != nil {
				t.Fatal(err)
			}
			out[version] = [3]string{checksum, state, fingerprint}
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return out
	}
	left, right := read(source), read(restored)
	if len(left) != 2 || len(right) != 2 {
		t.Fatalf("source/restored migration ledger entries=%d/%d; want 2/2", len(left), len(right))
	}
	for _, version := range []string{schema.BaselineVersion, schema.ManualReviewV2Version} {
		if left[version] != right[version] {
			t.Fatalf("source/restored ledger fingerprint differs for %s", version)
		}
	}
}

func assertLifecycleReadEndpoints(t *testing.T, databaseName, orderNo string) {
	t.Helper()
	baseCfg, err := mysqlDriver.ParseDSN(os.Getenv(lifecycleDSNEnv))
	if err != nil {
		t.Fatal("parse isolated DSN for restored read-only API")
	}
	baseCfg.DBName = databaseName
	baseCfg.ParseTime = true
	baseCfg.Loc = time.UTC
	previousGinMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(previousGinMode) })
	gormDB, err := gorm.Open(gormmysql.New(gormmysql.Config{DSN: baseCfg.FormatDSN()}), &gorm.Config{})
	if err != nil {
		t.Fatalf("open restored MySQL schema through the application ORM: %v", err)
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		t.Fatalf("get restored application DB pool: %v", err)
	}
	sqlDB.SetMaxOpenConns(4)
	t.Cleanup(func() { _ = sqlDB.Close() })
	engine := gin.New()
	cfg := &config.Config{JWTSecret: "synthetic-lifecycle-only-secret", FrontendURL: "http://localhost", RateLimitPerMinute: 1000}
	Register(engine, gormDB, cfg)
	token, err := utils.GenerateToken(cfg.JWTSecret, 7, "admin", "admin", nil, 1)
	if err != nil {
		t.Fatalf("create synthetic-only administrator token: %v", err)
	}
	request := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, req)
		return response
	}

	manualResponse := request("/api/v1/admin/subscription-manual-reviews")
	if manualResponse.Code != http.StatusOK {
		t.Fatalf("restored manual-review read endpoint status=%d body=%s", manualResponse.Code, manualResponse.Body.String())
	}
	var manualEnvelope struct {
		Data struct {
			List []dto.ManualSubscriptionReviewOrder `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(manualResponse.Body.Bytes(), &manualEnvelope); err != nil {
		t.Fatalf("decode restored manual-review endpoint response: %v", err)
	}
	if len(manualEnvelope.Data.List) != 1 {
		t.Fatalf("restored manual-review endpoint returned %d records; body=%s", len(manualEnvelope.Data.List), manualResponse.Body.String())
	}
	row := manualEnvelope.Data.List[0]
	if row.OrderNo != orderNo || row.Status != "manually_verified" || row.Merchant == nil || row.Merchant.Name != "synthetic-lifecycle-merchant" ||
		row.ManualReview.EvidenceReference != "synthetic-evidence-001" ||
		row.ManualReviewAudit.OrderStatusAfter != "manually_verified" || row.ManualReviewAudit.Reason != "synthetic audit" {
		t.Fatalf("restored manual-review endpoint returned the wrong synthetic record: %+v", row)
	}
	for _, forbidden := range []string{"payment_config", "app_secret", "review_key"} {
		if bytes.Contains(manualResponse.Body.Bytes(), []byte(forbidden)) {
			t.Fatalf("restored manual-review endpoint unexpectedly exposed %q", forbidden)
		}
	}

	date := time.Now().Format("2006-01-02")
	dashboardResponse := request("/api/v1/admin/dashboard?start_date=" + date + "&end_date=" + date)
	if dashboardResponse.Code != http.StatusOK {
		t.Fatalf("restored dashboard read endpoint status=%d body=%s", dashboardResponse.Code, dashboardResponse.Body.String())
	}
	var dashboardEnvelope struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(dashboardResponse.Body.Bytes(), &dashboardEnvelope); err != nil {
		t.Fatalf("decode restored dashboard response: %v", err)
	}
	for _, key := range []string{"platform_subscription_revenue", "total_revenue"} {
		raw, ok := dashboardEnvelope.Data[key]
		if !ok {
			t.Fatalf("restored dashboard response is missing %q", key)
		}
		var amount int64
		if err := json.Unmarshal(raw, &amount); err != nil || amount != 0 {
			t.Fatalf("restored dashboard %s=%s; want zero (decode error=%v)", key, string(raw), err)
		}
	}
}

func runSchemaMigrator(t *testing.T, root, binary, dsn, want string) {
	t.Helper()
	cmd := exec.Command(binary)
	cmd.Dir = root // The command must not discover the repository's .env file.
	cmd.Env = isolatedCommandEnv(root, "MYSQL_DSN="+dsn)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("isolated schema-migrate command failed: %v (stderr=%s)", err, strings.TrimSpace(stderr.String()))
	}
	if !strings.Contains(stdout.String(), ": "+want) {
		t.Fatalf("isolated schema-migrate result=%q; expected %q", strings.TrimSpace(stdout.String()), want)
	}
}

func isolatedCommandEnv(root string, extra ...string) []string {
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + root,
		"TMPDIR=" + root,
		"GOTOOLCHAIN=local",
		"GOPROXY=off",
		"GOSUMDB=off",
	}
	return append(env, extra...)
}

func countLifecycle(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var count int
	if err := db.QueryRow(query, args...).Scan(&count); err != nil {
		t.Fatalf("count lifecycle rows: %v", err)
	}
	return count
}

func pathIsWithin(t *testing.T, path, root string) bool {
	t.Helper()
	resolved, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("resolve temporary path: %v", err)
	}
	return pathWithinRoot(resolved, root)
}

func pathWithinRoot(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
