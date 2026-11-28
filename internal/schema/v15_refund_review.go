package schema

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	RefundReviewV15Version  = "schema-v15-refund-review-20261128"
	RefundReviewV15Checksum = "1ada0cec641389045bd07be4c9de558bdc1203a282dc962365a0be6d812772cc"
)

// refundReviewV15NewColumns lists the columns v15 adds to pre-existing
// tables, so historical fingerprints can exclude them.
var refundReviewV15NewColumns = []schemaColumn{
	{table: "payment_records", column: "operator_id"},
	{table: "payment_records", column: "operator_role"},
	{table: "refund_records", column: "reviewer_id"},
	{table: "refund_records", column: "reviewer_role"},
	{table: "refund_records", column: "review_reason"},
	{table: "refund_records", column: "reviewed_at"},
}

// refundReviewV15Statements is the exact DDL of the v15 migration, in
// apply order. No new tables, no new indexes, no CHECK constraints: the
// operator/reviewer value ranges are enforced in the service layer, and
// the pending-review list filters on the existing refund_records status
// index.
//
// History is not rewritten: pre-v15 payment records keep operator 0/""
// (no human operator was recorded for them; the operator is recoverable
// from the audit rows where M1 wrote one), and pre-v15 refund records keep
// reviewer 0/""/"" with a NULL review time (below-threshold refunds never
// went through a review).
var refundReviewV15Statements = []string{
	"ALTER TABLE `payment_records` ADD COLUMN `operator_id` bigint unsigned NOT NULL DEFAULT 0",
	"ALTER TABLE `payment_records` ADD COLUMN `operator_role` varchar(30) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT ''",
	"ALTER TABLE `refund_records` ADD COLUMN `reviewer_id` bigint unsigned NOT NULL DEFAULT 0",
	"ALTER TABLE `refund_records` ADD COLUMN `reviewer_role` varchar(30) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT ''",
	"ALTER TABLE `refund_records` ADD COLUMN `review_reason` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT ''",
	"ALTER TABLE `refund_records` ADD COLUMN `reviewed_at` datetime(3) NULL",
}

func validateRefundReviewV15Definition() error {
	material := append([]string{RefundReviewV15Version}, refundReviewV15Statements...)
	sum := sha256.Sum256([]byte(strings.Join(material, "\n")))
	if got := fmt.Sprintf("%x", sum); got != RefundReviewV15Checksum {
		return fmt.Errorf("refund review v15 checksum mismatch: definition %s, expected %s", got, RefundReviewV15Checksum)
	}
	return nil
}

// ApplyRefundReviewV15 appends the payment-record operator columns and the
// refund-review columns on an exact managed v14 schema. The v1-v14 schema
// and fingerprints remain untouched.
func ApplyRefundReviewV15(ctx context.Context, db *sql.DB) (Result, error) {
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
	if err := validateRefundReviewV15Definition(); err != nil {
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
	if err := verifyManagedSchemaV14(ctx, conn, objects, base, v2, v3, v4, v5, v6, v7, v8, v9); err == nil {
		if err := applyRefundReviewV15(ctx, conn, base, v2, v3, v4, v5, v6, v7, v8, v9); err != nil {
			return "", err
		}
		return ResultApplied, nil
	}
	if err := verifyManagedSchemaV15(ctx, conn, objects, base, v2, v3, v4, v5, v6, v7, v8, v9); err != nil {
		return "", fmt.Errorf("refusing non-exact managed schema for v15 refund review migration: %w", err)
	}
	return ResultAlready, nil
}

func applyRefundReviewV15(ctx context.Context, conn *sql.Conn, base baselineDefinition, v2 incrementalMigrationDefinition, v3 adminAuthV3Definition, v4 otpSafetyV4Definition, v5 emailOTPSafetyV5Definition, v6 wholesaleRetailV6Definition, v7 platformExpensesV7Definition, v8 platformExpenseReversalsV8Definition, v9 tableOrderingV9Definition) error {
	if _, err := conn.ExecContext(ctx, `INSERT INTO `+ledgerTable+` (version, checksum, state) VALUES (?, ?, 'applying')`, RefundReviewV15Version, RefundReviewV15Checksum); err != nil {
		return fmt.Errorf("record v15 refund review migration start: %w", err)
	}
	for i, statement := range refundReviewV15Statements {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply v15 refund review change %d (v15 ledger remains incomplete): %w", i+1, err)
		}
	}
	if err := verifyRefundReviewV15Tables(ctx, conn); err != nil {
		return fmt.Errorf("verify v15 refund review tables (v15 ledger remains incomplete): %w", err)
	}
	objects, err := inspectObjects(ctx, conn)
	if err != nil {
		return err
	}
	if err := verifyObjectInventoryV15(objects, base, v2, v7, v8, v9); err != nil {
		return fmt.Errorf("post-apply v15 inventory verification failed (v15 ledger remains incomplete): %w", err)
	}
	// The stored v14 fingerprint is computed over the schema without the
	// six v15 columns; the new columns are excluded here.
	v14Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, nil, nil, refundReviewV15NewColumns)
	if err != nil {
		return fmt.Errorf("compute immutable v14 schema fingerprint during v15 migration: %w", err)
	}
	var storedV14 string
	if err := conn.QueryRowContext(ctx, "SELECT schema_fingerprint FROM "+ledgerTable+" WHERE version=? AND checksum=? AND state='applied'", PlatformFinanceV14Version, PlatformFinanceV14Checksum).Scan(&storedV14); err != nil {
		return fmt.Errorf("read immutable v14 schema fingerprint during v15 migration: %w", err)
	}
	if storedV14 != v14Fingerprint {
		return fmt.Errorf("refusing v14 schema drift during v15 migration: stored %s, actual %s", storedV14, v14Fingerprint)
	}
	fingerprint, err := schemaFingerprint(ctx, conn)
	if err != nil {
		return fmt.Errorf("compute v15 installed schema fingerprint: %w", err)
	}
	result, err := conn.ExecContext(ctx, `UPDATE `+ledgerTable+` SET state='applied', schema_fingerprint=?, applied_at=CURRENT_TIMESTAMP WHERE version=? AND checksum=? AND state='applying'`, fingerprint, RefundReviewV15Version, RefundReviewV15Checksum)
	if err != nil {
		return fmt.Errorf("complete v15 schema version ledger: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("verify v15 schema version ledger update: %w", err)
	}
	if rows != 1 {
		return errors.New("v15 schema version ledger did not transition exactly once; it remains incomplete")
	}
	_ = v3
	_ = v4
	_ = v5
	_ = v6
	return nil
}

func verifyManagedSchemaV15(ctx context.Context, conn *sql.Conn, objects schemaObjects, base baselineDefinition, v2 incrementalMigrationDefinition, v3 adminAuthV3Definition, v4 otpSafetyV4Definition, v5 emailOTPSafetyV5Definition, v6 wholesaleRetailV6Definition, v7 platformExpensesV7Definition, v8 platformExpenseReversalsV8Definition, v9 tableOrderingV9Definition) error {
	// Historical fingerprints first: the v8 helper re-pins v1-v8, and
	// verifyRefundReviewV15Tables re-pins the altered payment/refund
	// shapes, so a drift in an old table trips the check that names the
	// drifted version (e.g. "v7 schema drift"), which ApplyLatest prefers
	// when refusing.
	combinedNewColumns := append(append(append(append([]schemaColumn(nil), orderItemsV11NewColumns...), aiCostControlV12NewColumns...), platformFinanceV14NewColumns...), refundReviewV15NewColumns...)
	futureTables := append(append(append(append(append([]string(nil), tableOrderingV9Order...), "order_items"), aiCostControlV12NewTables...), announcementsMaintenanceV13NewTables...), platformFinanceV14NewTables...)
	if err := verifyPlatformExpenseReversalsV8HistoricalFingerprintsExcludingColumns(ctx, conn, base, v2, v3, v4, v5, v6, v7, v8, combinedNewColumns, futureTables...); err != nil {
		return err
	}
	if err := verifyRefundReviewV15Tables(ctx, conn); err != nil {
		return err
	}
	if err := verifyObjectInventoryV15(objects, base, v2, v7, v8, v9); err != nil {
		return err
	}
	rows, err := conn.QueryContext(ctx, "SELECT version, checksum, state, schema_fingerprint FROM "+ledgerTable+" ORDER BY version")
	if err != nil {
		return fmt.Errorf("read v15 schema version ledger: %w", err)
	}
	type ledgerEntry struct {
		version, checksum, state string
		fingerprint              sql.NullString
	}
	entries := make([]ledgerEntry, 0, 15)
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
	if len(entries) != 15 {
		return fmt.Errorf("refusing v15 ledger with %d rows; expected exactly immutable v1-v15 rows", len(entries))
	}
	expected := []struct{ version, checksum string }{
		{BaselineVersion, base.checksum}, {v2.version, v2.checksum}, {v3.version, v3.checksum}, {v4.version, v4.checksum},
		{v5.version, v5.checksum}, {v6.version, v6.checksum}, {v7.version, v7.checksum}, {v8.version, v8.checksum},
		{v9.version, v9.checksum}, {TableLabelsV10Version, TableLabelsV10Checksum}, {OrderItemsV11Version, OrderItemsV11Checksum},
		{AICostControlV12Version, AICostControlV12Checksum},
		{AnnouncementsMaintenanceV13Version, AnnouncementsMaintenanceV13Checksum},
		{PlatformFinanceV14Version, PlatformFinanceV14Checksum},
		{RefundReviewV15Version, RefundReviewV15Checksum},
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
	v10NewColumns := []schemaColumn{{table: "store_tables", column: "table_label"}, {table: "table_order_details", column: "table_label_snapshot"}}
	// v9 check: exclude v12/v13/v14 tables + order_items, and v10/v11/v12/v14/v15 new columns.
	v9ExcludedColumns := append(append([]schemaColumn(nil), combinedNewColumns...), v10NewColumns...)
	v9ExcludedTables := append(append(append(append([]string(nil), "order_items"), aiCostControlV12NewTables...), announcementsMaintenanceV13NewTables...), platformFinanceV14NewTables...)
	v9Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, v9ExcludedTables, nil, v9ExcludedColumns)
	if err != nil {
		return fmt.Errorf("verify immutable v9 schema fingerprint: %w", err)
	}
	if stored := byVersion[v9.version].fingerprint.String; stored != v9Fingerprint {
		return fmt.Errorf("refusing v9 schema drift: stored fingerprint %s, actual %s", stored, v9Fingerprint)
	}
	// v10 check: same tables, v11/v12/v14/v15 new columns.
	v10Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, v9ExcludedTables, nil, combinedNewColumns)
	if err != nil {
		return fmt.Errorf("verify immutable v10 schema fingerprint: %w", err)
	}
	if stored := byVersion[TableLabelsV10Version].fingerprint.String; stored != v10Fingerprint {
		return fmt.Errorf("refusing v10 schema drift: stored fingerprint %s, actual %s", stored, v10Fingerprint)
	}
	// v11 check: exclude the v12/v13/v14 tables, and the v12/v14/v15 columns
	// (orders.channel is part of the stored v11 fingerprint; v13 added no
	// columns to pre-existing tables).
	v11ExcludedColumns := append(append(append([]schemaColumn(nil), aiCostControlV12NewColumns...), platformFinanceV14NewColumns...), refundReviewV15NewColumns...)
	v11ExcludedTables := append(append(append([]string(nil), aiCostControlV12NewTables...), announcementsMaintenanceV13NewTables...), platformFinanceV14NewTables...)
	v11Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, v11ExcludedTables, nil, v11ExcludedColumns)
	if err != nil {
		return fmt.Errorf("verify immutable v11 schema fingerprint: %w", err)
	}
	if stored := byVersion[OrderItemsV11Version].fingerprint.String; stored != v11Fingerprint {
		return fmt.Errorf("refusing v11 schema drift: stored fingerprint %s, actual %s", stored, v11Fingerprint)
	}
	// v12 check: exclude the v13/v14 tables and the v14/v15 columns (the v12
	// columns are part of the stored v12 fingerprint).
	v12ExcludedColumns := append(append([]schemaColumn(nil), platformFinanceV14NewColumns...), refundReviewV15NewColumns...)
	v12Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, append(append([]string(nil), announcementsMaintenanceV13NewTables...), platformFinanceV14NewTables...), nil, v12ExcludedColumns)
	if err != nil {
		return fmt.Errorf("verify immutable v12 schema fingerprint: %w", err)
	}
	if stored := byVersion[AICostControlV12Version].fingerprint.String; stored != v12Fingerprint {
		return fmt.Errorf("refusing v12 schema drift: stored fingerprint %s, actual %s", stored, v12Fingerprint)
	}
	// v13 check: exclude only the v14 tables and the v14/v15 columns (v13 added
	// no columns to pre-existing tables).
	v13ExcludedColumns := append(append([]schemaColumn(nil), platformFinanceV14NewColumns...), refundReviewV15NewColumns...)
	v13Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, platformFinanceV14NewTables, nil, v13ExcludedColumns)
	if err != nil {
		return fmt.Errorf("verify immutable v13 schema fingerprint: %w", err)
	}
	if stored := byVersion[AnnouncementsMaintenanceV13Version].fingerprint.String; stored != v13Fingerprint {
		return fmt.Errorf("refusing v13 schema drift: stored fingerprint %s, actual %s", stored, v13Fingerprint)
	}
	// v14 check: exclude only the v15 columns (v15 adds no tables).
	v14Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, nil, nil, refundReviewV15NewColumns)
	if err != nil {
		return fmt.Errorf("verify immutable v14 schema fingerprint: %w", err)
	}
	if stored := byVersion[PlatformFinanceV14Version].fingerprint.String; stored != v14Fingerprint {
		return fmt.Errorf("refusing v14 schema drift: stored fingerprint %s, actual %s", stored, v14Fingerprint)
	}
	actualV15, err := schemaFingerprint(ctx, conn)
	if err != nil {
		return fmt.Errorf("verify v15 managed schema fingerprint: %w", err)
	}
	if stored := byVersion[RefundReviewV15Version].fingerprint.String; stored != actualV15 {
		return fmt.Errorf("refusing v15 schema drift: stored fingerprint %s, actual %s", stored, actualV15)
	}
	_ = v6
	return nil
}

// verifyRefundReviewV15Tables checks the exact shape of the two altered
// tables (payment_records: 11 columns; refund_records: 20 columns). The
// baseline columns keep their v1 nullability and defaults; the six new
// columns are NOT NULL with defaults (operator/reviewer 0/"", review
// reason "", reviewed_at NULL).
func verifyRefundReviewV15Tables(ctx context.Context, conn *sql.Conn) error {
	type columnSpec struct {
		name, columnType, nullable string
		defaultValue               string
		hasDefault                 bool
		extra, collation           string
	}
	col := func(name, columnType, nullable string, defaultValue string, hasDefault bool, extra, collation string) columnSpec {
		return columnSpec{name: name, columnType: columnType, nullable: nullable, defaultValue: defaultValue, hasDefault: hasDefault, extra: extra, collation: collation}
	}
	tables := []struct {
		name    string
		columns []columnSpec
		indexes []string
		checks  []string
	}{
		{
			name: "payment_records",
			columns: []columnSpec{
				col("id", "bigint unsigned", "NO", "", false, "auto_increment", ""),
				col("created_at", "datetime(3)", "YES", "", false, "", ""),
				col("updated_at", "datetime(3)", "YES", "", false, "", ""),
				col("order_id", "bigint unsigned", "YES", "", false, "", ""),
				col("payment_channel", "varchar(40)", "YES", "", false, "", "utf8mb4_unicode_ci"),
				col("transaction_no", "varchar(80)", "YES", "", false, "", "utf8mb4_unicode_ci"),
				col("amount", "bigint", "YES", "", false, "", ""),
				col("status", "varchar(20)", "YES", "success", true, "", "utf8mb4_unicode_ci"),
				col("raw_payload", "text", "YES", "", false, "", "utf8mb4_unicode_ci"),
				col("operator_id", "bigint unsigned", "NO", "0", true, "", ""),
				col("operator_role", "varchar(30)", "NO", "", true, "", "utf8mb4_unicode_ci"),
			},
			indexes: []string{
				"PRIMARY|0|1|id|NULL|BTREE",
				"fk_payment_records_order|1|1|order_id|NULL|BTREE",
				"idx_payment_records_transaction_no|0|1|transaction_no|NULL|BTREE",
			},
			checks: nil,
		},
		{
			name: "refund_records",
			columns: []columnSpec{
				col("id", "bigint unsigned", "NO", "", false, "auto_increment", ""),
				col("created_at", "datetime(3)", "YES", "", false, "", ""),
				col("updated_at", "datetime(3)", "YES", "", false, "", ""),
				col("order_id", "bigint unsigned", "YES", "", false, "", ""),
				col("merchant_id", "bigint unsigned", "YES", "", false, "", ""),
				col("store_id", "bigint unsigned", "YES", "", false, "", ""),
				col("refund_no", "varchar(60)", "YES", "", false, "", "utf8mb4_unicode_ci"),
				col("idempotency_key", "varchar(120)", "YES", "", false, "", "utf8mb4_unicode_ci"),
				col("request_fingerprint", "varchar(64)", "NO", "", true, "", "utf8mb4_unicode_ci"),
				col("transaction_no", "varchar(80)", "YES", "", false, "", "utf8mb4_unicode_ci"),
				col("amount", "bigint", "NO", "", false, "", ""),
				col("reason", "varchar(255)", "YES", "", false, "", "utf8mb4_unicode_ci"),
				col("operator_role", "varchar(30)", "YES", "", false, "", "utf8mb4_unicode_ci"),
				col("operator_id", "bigint unsigned", "YES", "", false, "", ""),
				col("status", "varchar(20)", "YES", "success", true, "", "utf8mb4_unicode_ci"),
				col("raw_payload", "text", "YES", "", false, "", "utf8mb4_unicode_ci"),
				col("reviewer_id", "bigint unsigned", "NO", "0", true, "", ""),
				col("reviewer_role", "varchar(30)", "NO", "", true, "", "utf8mb4_unicode_ci"),
				col("review_reason", "varchar(255)", "NO", "", true, "", "utf8mb4_unicode_ci"),
				col("reviewed_at", "datetime(3)", "YES", "", false, "", ""),
			},
			indexes: []string{
				"PRIMARY|0|1|id|NULL|BTREE",
				// NOTE: the v1 DDL also declares CONSTRAINT
				// fk_refund_records_order on order_id, but MySQL reuses
				// the existing KEY idx_refund_records_order_id for the
				// foreign key instead of creating a second index named
				// after the constraint, so no fk_refund_records_order
				// index ever exists in information_schema. The constraint
				// itself is still covered by the fingerprint's
				// constraints section. (payment_records is different:
				// its DDL has no separate order_id KEY, so MySQL does
				// auto-create fk_payment_records_order there.)
				"idx_refund_order_idempotency|0|1|order_id|NULL|BTREE",
				"idx_refund_order_idempotency|0|2|idempotency_key|NULL|BTREE",
				"idx_refund_records_merchant_id|1|1|merchant_id|NULL|BTREE",
				"idx_refund_records_order_id|1|1|order_id|NULL|BTREE",
				"idx_refund_records_refund_no|0|1|refund_no|NULL|BTREE",
				"idx_refund_records_status|1|1|status|NULL|BTREE",
				"idx_refund_records_store_id|1|1|store_id|NULL|BTREE",
				"idx_refund_records_transaction_no|1|1|transaction_no|NULL|BTREE",
			},
			checks: nil,
		},
	}
	for _, table := range tables {
		var engine, tableCollation string
		if err := conn.QueryRowContext(ctx, "SELECT ENGINE, COALESCE(TABLE_COLLATION,'') FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=?", table.name).Scan(&engine, &tableCollation); err != nil {
			return fmt.Errorf("inspect %s engine and collation: %w", table.name, err)
		}
		if engine != "InnoDB" || tableCollation != "utf8mb4_unicode_ci" {
			return fmt.Errorf("%s engine/collation mismatch: found %s/%s, expected InnoDB/utf8mb4_unicode_ci", table.name, engine, tableCollation)
		}
		rows, err := conn.QueryContext(ctx, "SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_DEFAULT, EXTRA, COALESCE(COLLATION_NAME,'') FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? ORDER BY ORDINAL_POSITION", table.name)
		if err != nil {
			return fmt.Errorf("inspect %s columns: %w", table.name, err)
		}
		actual := make([]columnSpec, 0)
		for rows.Next() {
			var got columnSpec
			var defaultValue sql.NullString
			if err := rows.Scan(&got.name, &got.columnType, &got.nullable, &defaultValue, &got.extra, &got.collation); err != nil {
				rows.Close()
				return err
			}
			got.hasDefault = defaultValue.Valid
			if defaultValue.Valid {
				got.defaultValue = defaultValue.String
			}
			actual = append(actual, got)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		if len(actual) != len(table.columns) {
			return fmt.Errorf("%s column count mismatch: found %d, expected %d", table.name, len(actual), len(table.columns))
		}
		for i, want := range table.columns {
			if actual[i] != want {
				return fmt.Errorf("%s column %d mismatch: found %+v, expected %+v", table.name, i+1, actual[i], want)
			}
		}
		rows, err = conn.QueryContext(ctx, "SELECT INDEX_NAME, NON_UNIQUE, SEQ_IN_INDEX, COLUMN_NAME, SUB_PART, INDEX_TYPE FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? ORDER BY INDEX_NAME, SEQ_IN_INDEX", table.name)
		if err != nil {
			return fmt.Errorf("inspect %s indexes: %w", table.name, err)
		}
		actualIndexes := make([]string, 0)
		for rows.Next() {
			var name, column, indexType string
			var nonUnique, sequence int
			var subPart sql.NullInt64
			if err := rows.Scan(&name, &nonUnique, &sequence, &column, &subPart, &indexType); err != nil {
				rows.Close()
				return err
			}
			subPartValue := "NULL"
			if subPart.Valid {
				subPartValue = fmt.Sprint(subPart.Int64)
			}
			actualIndexes = append(actualIndexes, fmt.Sprintf("%s|%d|%d|%s|%s|%s", name, nonUnique, sequence, column, subPartValue, indexType))
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		sort.Strings(actualIndexes)
		wantIndexes := append([]string(nil), table.indexes...)
		sort.Strings(wantIndexes)
		if !equalStrings(actualIndexes, wantIndexes) {
			return fmt.Errorf("%s exact index definition mismatch: found %v, expected %v", table.name, actualIndexes, wantIndexes)
		}
		rows, err = conn.QueryContext(ctx, `SELECT tc.CONSTRAINT_NAME, tc.TABLE_NAME, cc.CHECK_CLAUSE, tc.ENFORCED
			FROM information_schema.TABLE_CONSTRAINTS tc
			JOIN information_schema.CHECK_CONSTRAINTS cc
			ON cc.CONSTRAINT_SCHEMA=tc.CONSTRAINT_SCHEMA AND cc.CONSTRAINT_NAME=tc.CONSTRAINT_NAME
			WHERE tc.TABLE_SCHEMA=DATABASE() AND tc.TABLE_NAME=? AND tc.CONSTRAINT_TYPE='CHECK'
			ORDER BY tc.CONSTRAINT_NAME`, table.name)
		if err != nil {
			return fmt.Errorf("inspect %s CHECK constraints: %w", table.name, err)
		}
		actualChecks := make([]string, 0)
		for rows.Next() {
			var name, tableName, clause, enforced string
			if err := rows.Scan(&name, &tableName, &clause, &enforced); err != nil {
				rows.Close()
				return err
			}
			actualChecks = append(actualChecks, fmt.Sprintf("%s|%s|%s|%s", name, tableName, normalizeCheckClause(clause), enforced))
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		sort.Strings(actualChecks)
		wantChecks := append([]string(nil), table.checks...)
		sort.Strings(wantChecks)
		if !equalStrings(actualChecks, wantChecks) {
			return fmt.Errorf("%s exact CHECK-constraint definition mismatch: found %v, expected %v", table.name, actualChecks, wantChecks)
		}
	}
	return nil
}

func verifyObjectInventoryV15(objects schemaObjects, base baselineDefinition, v2 incrementalMigrationDefinition, v7 platformExpensesV7Definition, v8 platformExpenseReversalsV8Definition, v9 tableOrderingV9Definition) error {
	// v15 adds columns only, no tables, triggers, views, routines or
	// events: the inventory is exactly the v14 inventory.
	return verifyObjectInventoryV14(objects, base, v2, v7, v8, v9)
}
