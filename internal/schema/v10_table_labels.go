package schema

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	TableLabelsV10Version  = "schema-v10-merchant-table-labels-20261124"
	TableLabelsV10Checksum = "c9769345b9d994a7a42e856fcb05dc9f56e9c001a1e7fe7cbd9f4d09aa5ab7c9"
)

var tableLabelsV10Statements = []string{
	"ALTER TABLE `store_tables` ADD COLUMN `table_label` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT ''",
	"ALTER TABLE `table_order_details` ADD COLUMN `table_label_snapshot` varchar(64) COLLATE utf8mb4_unicode_ci NULL",
}

// ApplyTableLabelsV10 appends only the merchant-facing label and immutable
// checkout-time label snapshot columns to an exact managed v9 schema.
func ApplyTableLabelsV10(ctx context.Context, db *sql.DB) (Result, error) {
	base, err := loadBaseline()
	if err != nil {
		return "", err
	}
	v2, err := loadManualReviewV2()
	if err != nil {
		return "", err
	}
	v3, err := loadAdminAuthV3()
	if err != nil {
		return "", err
	}
	v4, err := loadOTPSafetyV4()
	if err != nil {
		return "", err
	}
	v5, err := loadEmailOTPSafetyV5()
	if err != nil {
		return "", err
	}
	v6, err := loadWholesaleRetailV6()
	if err != nil {
		return "", err
	}
	v7, err := loadPlatformExpensesV7()
	if err != nil {
		return "", err
	}
	v8, err := loadPlatformExpenseReversalsV8()
	if err != nil {
		return "", err
	}
	v9, err := loadTableOrderingV9()
	if err != nil {
		return "", err
	}
	if err := validateTableLabelsV10Definition(); err != nil {
		return "", err
	}
	if db == nil {
		return "", errors.New("database handle is required")
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return "", fmt.Errorf("get dedicated database connection: %w", err)
	}
	defer conn.Close()
	var databaseName sql.NullString
	if err := conn.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&databaseName); err != nil {
		return "", fmt.Errorf("read selected database: %w", err)
	}
	if !databaseName.Valid || strings.TrimSpace(databaseName.String) == "" {
		return "", errors.New("MYSQL_DSN must select an existing target database")
	}
	lockName := advisoryLockName(databaseName.String)
	var lockResult sql.NullInt64
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, 0)", lockName).Scan(&lockResult); err != nil {
		return "", fmt.Errorf("acquire schema migration lock: %w", err)
	}
	if !lockResult.Valid || lockResult.Int64 != 1 {
		return "", fmt.Errorf("another schema migration holds the lock for database %q", databaseName.String)
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(releaseCtx, "SELECT RELEASE_LOCK(?)", lockName)
	}()

	objects, err := inspectObjects(ctx, conn)
	if err != nil {
		return "", err
	}
	if err := verifyManagedSchemaV9(ctx, conn, objects, base, v2, v3, v4, v5, v6, v7, v8, v9); err == nil {
		if err := applyTableLabelsV10(ctx, conn, base, v2, v3, v4, v5, v6, v7, v8, v9); err != nil {
			return "", err
		}
		return ResultApplied, nil
	}
	if err := verifyManagedSchemaV10(ctx, conn, objects, base, v2, v3, v4, v5, v6, v7, v8, v9); err != nil {
		return "", fmt.Errorf("refusing non-exact managed schema for v10 merchant table labels migration: %w", err)
	}
	return ResultAlready, nil
}

func validateTableLabelsV10Definition() error {
	material := append([]string{TableLabelsV10Version}, tableLabelsV10Statements...)
	sum := sha256.Sum256([]byte(strings.Join(material, "\n")))
	if got := fmt.Sprintf("%x", sum); got != TableLabelsV10Checksum {
		return fmt.Errorf("table labels v10 checksum mismatch: definition %s, expected %s", got, TableLabelsV10Checksum)
	}
	return nil
}

func applyTableLabelsV10(ctx context.Context, conn *sql.Conn, base baselineDefinition, v2 incrementalMigrationDefinition, v3 adminAuthV3Definition, v4 otpSafetyV4Definition, v5 emailOTPSafetyV5Definition, v6 wholesaleRetailV6Definition, v7 platformExpensesV7Definition, v8 platformExpenseReversalsV8Definition, v9 tableOrderingV9Definition) error {
	if _, err := conn.ExecContext(ctx, `INSERT INTO `+ledgerTable+` (version, checksum, state) VALUES (?, ?, 'applying')`, TableLabelsV10Version, TableLabelsV10Checksum); err != nil {
		return fmt.Errorf("record v10 merchant table labels migration start: %w", err)
	}
	for i, statement := range tableLabelsV10Statements {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply v10 table-label change %d (v10 ledger remains incomplete): %w", i+1, err)
		}
	}
	if err := verifyTableLabelsV10Columns(ctx, conn); err != nil {
		return fmt.Errorf("verify v10 table-label columns (v10 ledger remains incomplete): %w", err)
	}
	objects, err := inspectObjects(ctx, conn)
	if err != nil {
		return err
	}
	if err := verifyObjectInventoryV9(objects, base, v2, v7, v8, v9); err != nil {
		return fmt.Errorf("post-apply v10 object inventory verification failed (v10 ledger remains incomplete): %w", err)
	}
	if err := verifyPlatformExpenseReversalsV8HistoricalFingerprints(ctx, conn, base, v2, v3, v4, v5, v6, v7, v8, tableOrderingV9Order...); err != nil {
		return fmt.Errorf("verify immutable v1-v7 schema during v10 migration (v10 ledger remains incomplete): %w", err)
	}
	v9Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, nil, nil, []schemaColumn{{table: "store_tables", column: "table_label"}, {table: "table_order_details", column: "table_label_snapshot"}})
	if err != nil {
		return fmt.Errorf("compute immutable v9 schema fingerprint during v10 migration: %w", err)
	}
	var storedV9 string
	if err := conn.QueryRowContext(ctx, "SELECT schema_fingerprint FROM "+ledgerTable+" WHERE version=? AND checksum=? AND state='applied'", v9.version, v9.checksum).Scan(&storedV9); err != nil {
		return fmt.Errorf("read immutable v9 schema fingerprint during v10 migration: %w", err)
	}
	if storedV9 != v9Fingerprint {
		return fmt.Errorf("refusing v9 schema drift during v10 migration: stored %s, actual %s", storedV9, v9Fingerprint)
	}
	fingerprint, err := schemaFingerprint(ctx, conn)
	if err != nil {
		return fmt.Errorf("compute v10 installed schema fingerprint: %w", err)
	}
	result, err := conn.ExecContext(ctx, `UPDATE `+ledgerTable+` SET state='applied', schema_fingerprint=?, applied_at=CURRENT_TIMESTAMP WHERE version=? AND checksum=? AND state='applying'`, fingerprint, TableLabelsV10Version, TableLabelsV10Checksum)
	if err != nil {
		return fmt.Errorf("complete v10 schema version ledger: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("verify v10 schema version ledger update: %w", err)
	}
	if rows != 1 {
		return errors.New("v10 schema version ledger did not transition exactly once; it remains incomplete")
	}
	return nil
}

func verifyManagedSchemaV10(ctx context.Context, conn *sql.Conn, objects schemaObjects, base baselineDefinition, v2 incrementalMigrationDefinition, v3 adminAuthV3Definition, v4 otpSafetyV4Definition, v5 emailOTPSafetyV5Definition, v6 wholesaleRetailV6Definition, v7 platformExpensesV7Definition, v8 platformExpenseReversalsV8Definition, v9 tableOrderingV9Definition) error {
	if err := verifyTableLabelsV10Columns(ctx, conn); err != nil {
		return err
	}
	if err := verifyObjectInventoryV9(objects, base, v2, v7, v8, v9); err != nil {
		return err
	}
	if err := verifyPlatformExpenseReversalsV8HistoricalFingerprints(ctx, conn, base, v2, v3, v4, v5, v6, v7, v8, tableOrderingV9Order...); err != nil {
		return err
	}
	rows, err := conn.QueryContext(ctx, "SELECT version, checksum, state, schema_fingerprint FROM "+ledgerTable+" ORDER BY version")
	if err != nil {
		return fmt.Errorf("read v10 schema version ledger: %w", err)
	}
	type ledgerEntry struct {
		version, checksum, state string
		fingerprint              sql.NullString
	}
	entries := make([]ledgerEntry, 0, 10)
	for rows.Next() {
		var entry ledgerEntry
		if err := rows.Scan(&entry.version, &entry.checksum, &entry.state, &entry.fingerprint); err != nil {
			rows.Close()
			return err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if len(entries) != 10 {
		return fmt.Errorf("refusing v10 ledger with %d rows; expected exactly immutable v1-v10 rows", len(entries))
	}
	expected := []struct{ version, checksum string }{
		{BaselineVersion, base.checksum}, {v2.version, v2.checksum}, {v3.version, v3.checksum}, {v4.version, v4.checksum},
		{v5.version, v5.checksum}, {v6.version, v6.checksum}, {v7.version, v7.checksum}, {v8.version, v8.checksum},
		{v9.version, v9.checksum}, {TableLabelsV10Version, TableLabelsV10Checksum},
	}
	byVersion := make(map[string]ledgerEntry, len(entries))
	for _, entry := range entries {
		byVersion[entry.version] = entry
	}
	for _, want := range expected {
		entry, ok := byVersion[want.version]
		if !ok || entry.checksum != want.checksum || entry.state != "applied" || !entry.fingerprint.Valid || entry.fingerprint.String == "" {
			return fmt.Errorf("refusing missing, altered, or incomplete %s ledger entry", want.version)
		}
	}
	v9Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, nil, nil, []schemaColumn{{table: "store_tables", column: "table_label"}, {table: "table_order_details", column: "table_label_snapshot"}})
	if err != nil {
		return fmt.Errorf("verify immutable v9 schema fingerprint: %w", err)
	}
	if stored := byVersion[v9.version].fingerprint.String; stored != v9Fingerprint {
		return fmt.Errorf("refusing v9 schema drift: stored fingerprint %s, actual %s", stored, v9Fingerprint)
	}
	actualV10, err := schemaFingerprint(ctx, conn)
	if err != nil {
		return fmt.Errorf("verify v10 managed schema fingerprint: %w", err)
	}
	if stored := byVersion[TableLabelsV10Version].fingerprint.String; stored != actualV10 {
		return fmt.Errorf("refusing v10 schema drift: stored fingerprint %s, actual %s", stored, actualV10)
	}
	return nil
}

func verifyTableLabelsV10Columns(ctx context.Context, conn *sql.Conn) error {
	want := map[string]struct{ columnType, nullable, defaultValue, collation string }{
		"store_tables.table_label":                 {columnType: "varchar(64)", nullable: "NO", defaultValue: "", collation: "utf8mb4_unicode_ci"},
		"table_order_details.table_label_snapshot": {columnType: "varchar(64)", nullable: "YES", defaultValue: "<NULL>", collation: "utf8mb4_unicode_ci"},
	}
	rows, err := conn.QueryContext(ctx, `SELECT TABLE_NAME, COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COALESCE(CAST(COLUMN_DEFAULT AS CHAR), '<NULL>'), COALESCE(COLLATION_NAME, '') FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND ((TABLE_NAME='store_tables' AND COLUMN_NAME='table_label') OR (TABLE_NAME='table_order_details' AND COLUMN_NAME='table_label_snapshot')) ORDER BY TABLE_NAME, COLUMN_NAME`)
	if err != nil {
		return fmt.Errorf("inspect v10 table-label columns: %w", err)
	}
	defer rows.Close()
	seen := make(map[string]bool, len(want))
	for rows.Next() {
		var table, column, columnType, nullable, defaultValue, collation string
		if err := rows.Scan(&table, &column, &columnType, &nullable, &defaultValue, &collation); err != nil {
			return err
		}
		name := table + "." + column
		expected, ok := want[name]
		if !ok || expected.columnType != columnType || expected.nullable != nullable || expected.defaultValue != defaultValue || expected.collation != collation {
			return fmt.Errorf("v10 column %s has unexpected shape: type=%s nullable=%s default=%q collation=%s", name, columnType, nullable, defaultValue, collation)
		}
		seen[name] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(seen) != len(want) {
		return fmt.Errorf("v10 table-label columns are incomplete: found %v", seen)
	}
	return nil
}
