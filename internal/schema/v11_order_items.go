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
	OrderItemsV11Version  = "schema-v11-order-items-20261127"
	OrderItemsV11Checksum = "d5caa30c03838c6c90b975ef7673a4c67000e2e1c503e3395a51bcb20df5b322"
)

// orderItemsV11Statements is the exact DDL of the v11 migration, in apply
// order: the new order_items detail table, then the orders.channel column.
// The channel column deliberately carries no index and no CHECK constraint:
// orders is a v1 table, so an index or CHECK on it would change every pinned
// historical schema fingerprint (the fingerprint excludes the new column
// itself, but not indexes or constraints on its table). The allowed channel
// values (store_qr / table / share) are enforced in service code instead
// (see service.ValidOrderChannel).
var orderItemsV11Statements = []string{
	`CREATE TABLE ` + "`order_items`" + ` (
				` + "`id`" + ` bigint unsigned NOT NULL AUTO_INCREMENT,
				` + "`created_at`" + ` datetime(3) NOT NULL,
				` + "`updated_at`" + ` datetime(3) NOT NULL,
				` + "`order_id`" + ` bigint unsigned NOT NULL,
				` + "`product_id`" + ` bigint unsigned NOT NULL,
				` + "`product_name_snapshot`" + ` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
				` + "`quantity`" + ` int NOT NULL,
				` + "`unit_price`" + ` bigint NOT NULL,
				` + "`amount`" + ` bigint NOT NULL,
				PRIMARY KEY (` + "`id`" + `),
				KEY ` + "`idx_order_items_order_id`" + ` (` + "`order_id`" + `),
				KEY ` + "`idx_order_items_product_id`" + ` (` + "`product_id`" + `),
				CONSTRAINT ` + "`fk_order_items_order_id`" + ` FOREIGN KEY (` + "`order_id`" + `) REFERENCES ` + "`orders`" + ` (` + "`id`" + `) ON DELETE RESTRICT,
				CONSTRAINT ` + "`fk_order_items_product_id`" + ` FOREIGN KEY (` + "`product_id`" + `) REFERENCES ` + "`store_products`" + ` (` + "`id`" + `) ON DELETE RESTRICT,
				CONSTRAINT ` + "`chk_order_items_quantity`" + ` CHECK (` + "`quantity`" + ` BETWEEN 1 AND 999),
				CONSTRAINT ` + "`chk_order_items_unit_price`" + ` CHECK (` + "`unit_price`" + ` >= 0),
				CONSTRAINT ` + "`chk_order_items_amount`" + ` CHECK (` + "`amount`" + ` >= 0)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	"ALTER TABLE `orders` ADD COLUMN `channel` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'store_qr'",
}

// orderItemsV11TableBackfill marks every pre-v11 order that has a
// table_order_details row as a table order. All other pre-v11 orders keep
// the 'store_qr' default: they were placed by scanning a store QR code
// (share orders did not exist as a distinct channel before v11, and no
// historical share attribution is invented here).
const orderItemsV11TableBackfill = "UPDATE `orders` o JOIN `table_order_details` d ON d.`order_id` = o.`id` SET o.`channel` = 'table' WHERE o.`channel` = 'store_qr'"

// orderItemsV11NewColumns lists the columns v11 adds to pre-existing
// tables, so historical fingerprints can exclude them.
var orderItemsV11NewColumns = []schemaColumn{{table: "orders", column: "channel"}}

func validateOrderItemsV11Definition() error {
	material := append([]string{OrderItemsV11Version}, orderItemsV11Statements...)
	material = append(material, orderItemsV11TableBackfill)
	sum := sha256.Sum256([]byte(strings.Join(material, "\n")))
	if got := fmt.Sprintf("%x", sum); got != OrderItemsV11Checksum {
		return fmt.Errorf("order items v11 checksum mismatch: definition %s, expected %s", got, OrderItemsV11Checksum)
	}
	return nil
}

// ApplyOrderItemsV11 appends the order_items detail table and the
// orders.channel column (plus the table-order backfill) to an exact managed
// v10 schema. The v1-v10 schema and fingerprints remain untouched.
func ApplyOrderItemsV11(ctx context.Context, db *sql.DB) (Result, error) {
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
	if err := validateOrderItemsV11Definition(); err != nil {
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
	if err := verifyManagedSchemaV10(ctx, conn, objects, base, v2, v3, v4, v5, v6, v7, v8, v9); err == nil {
		if err := applyOrderItemsV11(ctx, conn, base, v2, v3, v4, v5, v6, v7, v8, v9); err != nil {
			return "", err
		}
		return ResultApplied, nil
	}
	if err := verifyManagedSchemaV11(ctx, conn, objects, base, v2, v3, v4, v5, v6, v7, v8, v9); err != nil {
		return "", fmt.Errorf("refusing non-exact managed schema for v11 order items migration: %w", err)
	}
	return ResultAlready, nil
}

func applyOrderItemsV11(ctx context.Context, conn *sql.Conn, base baselineDefinition, v2 incrementalMigrationDefinition, v3 adminAuthV3Definition, v4 otpSafetyV4Definition, v5 emailOTPSafetyV5Definition, v6 wholesaleRetailV6Definition, v7 platformExpensesV7Definition, v8 platformExpenseReversalsV8Definition, v9 tableOrderingV9Definition) error {
	if _, err := conn.ExecContext(ctx, `INSERT INTO `+ledgerTable+` (version, checksum, state) VALUES (?, ?, 'applying')`, OrderItemsV11Version, OrderItemsV11Checksum); err != nil {
		return fmt.Errorf("record v11 order items migration start: %w", err)
	}
	for i, statement := range orderItemsV11Statements {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply v11 order-items change %d (v11 ledger remains incomplete): %w", i+1, err)
		}
	}
	if _, err := conn.ExecContext(ctx, orderItemsV11TableBackfill); err != nil {
		return fmt.Errorf("backfill v11 table order channels (v11 ledger remains incomplete): %w", err)
	}
	var unbackfilled int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM `orders` o JOIN `table_order_details` d ON d.`order_id` = o.`id` WHERE o.`channel` <> 'table'").Scan(&unbackfilled); err != nil {
		return fmt.Errorf("verify v11 table order channel backfill (v11 ledger remains incomplete): %w", err)
	}
	if unbackfilled != 0 {
		return fmt.Errorf("v11 table order channel backfill left %d orders unmarked (v11 ledger remains incomplete)", unbackfilled)
	}
	if err := verifyOrderItemsV11Tables(ctx, conn); err != nil {
		return fmt.Errorf("verify v11 tables (v11 ledger remains incomplete): %w", err)
	}
	objects, err := inspectObjects(ctx, conn)
	if err != nil {
		return err
	}
	if err := verifyObjectInventoryV11(objects, base, v2, v7, v8, v9); err != nil {
		return fmt.Errorf("post-apply v11 object inventory verification failed (v11 ledger remains incomplete): %w", err)
	}
	v10Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, []string{"order_items"}, nil, orderItemsV11NewColumns)
	if err != nil {
		return fmt.Errorf("compute immutable v10 schema fingerprint during v11 migration: %w", err)
	}
	var storedV10 string
	if err := conn.QueryRowContext(ctx, "SELECT schema_fingerprint FROM "+ledgerTable+" WHERE version=? AND checksum=? AND state='applied'", TableLabelsV10Version, TableLabelsV10Checksum).Scan(&storedV10); err != nil {
		return fmt.Errorf("read immutable v10 schema fingerprint during v11 migration: %w", err)
	}
	if storedV10 != v10Fingerprint {
		return fmt.Errorf("refusing v10 schema drift during v11 migration: stored %s, actual %s", storedV10, v10Fingerprint)
	}
	fingerprint, err := schemaFingerprint(ctx, conn)
	if err != nil {
		return fmt.Errorf("compute v11 installed schema fingerprint: %w", err)
	}
	result, err := conn.ExecContext(ctx, `UPDATE `+ledgerTable+` SET state='applied', schema_fingerprint=?, applied_at=CURRENT_TIMESTAMP WHERE version=? AND checksum=? AND state='applying'`, fingerprint, OrderItemsV11Version, OrderItemsV11Checksum)
	if err != nil {
		return fmt.Errorf("complete v11 schema version ledger: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("verify v11 schema version ledger update: %w", err)
	}
	if rows != 1 {
		return errors.New("v11 schema version ledger did not transition exactly once; it remains incomplete")
	}
	_ = v3
	_ = v4
	_ = v5
	_ = v6
	return nil
}

func verifyManagedSchemaV11(ctx context.Context, conn *sql.Conn, objects schemaObjects, base baselineDefinition, v2 incrementalMigrationDefinition, v3 adminAuthV3Definition, v4 otpSafetyV4Definition, v5 emailOTPSafetyV5Definition, v6 wholesaleRetailV6Definition, v7 platformExpensesV7Definition, v8 platformExpenseReversalsV8Definition, v9 tableOrderingV9Definition) error {
	if err := verifyOrderItemsV11Tables(ctx, conn); err != nil {
		return err
	}
	if err := verifyObjectInventoryV11(objects, base, v2, v7, v8, v9); err != nil {
		return err
	}
	futureTables := append(append([]string(nil), tableOrderingV9Order...), "order_items")
	if err := verifyPlatformExpenseReversalsV8HistoricalFingerprintsExcludingColumns(ctx, conn, base, v2, v3, v4, v5, v6, v7, v8, orderItemsV11NewColumns, futureTables...); err != nil {
		return err
	}
	rows, err := conn.QueryContext(ctx, "SELECT version, checksum, state, schema_fingerprint FROM "+ledgerTable+" ORDER BY version")
	if err != nil {
		return fmt.Errorf("read v11 schema version ledger: %w", err)
	}
	type ledgerEntry struct {
		version, checksum, state string
		fingerprint              sql.NullString
	}
	entries := make([]ledgerEntry, 0, 11)
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
	if len(entries) != 11 {
		return fmt.Errorf("refusing v11 ledger with %d rows; expected exactly immutable v1-v11 rows", len(entries))
	}
	expected := []struct{ version, checksum string }{
		{BaselineVersion, base.checksum}, {v2.version, v2.checksum}, {v3.version, v3.checksum}, {v4.version, v4.checksum},
		{v5.version, v5.checksum}, {v6.version, v6.checksum}, {v7.version, v7.checksum}, {v8.version, v8.checksum},
		{v9.version, v9.checksum}, {TableLabelsV10Version, TableLabelsV10Checksum}, {OrderItemsV11Version, OrderItemsV11Checksum},
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
	v10ExcludedColumns := orderItemsV11NewColumns
	v9ExcludedColumns := append(append([]schemaColumn(nil), orderItemsV11NewColumns...), schemaColumn{table: "store_tables", column: "table_label"}, schemaColumn{table: "table_order_details", column: "table_label_snapshot"})
	v9Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, []string{"order_items"}, nil, v9ExcludedColumns)
	if err != nil {
		return fmt.Errorf("verify immutable v9 schema fingerprint: %w", err)
	}
	if stored := byVersion[v9.version].fingerprint.String; stored != v9Fingerprint {
		return fmt.Errorf("refusing v9 schema drift: stored fingerprint %s, actual %s", stored, v9Fingerprint)
	}
	v10Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, []string{"order_items"}, nil, v10ExcludedColumns)
	if err != nil {
		return fmt.Errorf("verify immutable v10 schema fingerprint: %w", err)
	}
	if stored := byVersion[TableLabelsV10Version].fingerprint.String; stored != v10Fingerprint {
		return fmt.Errorf("refusing v10 schema drift: stored fingerprint %s, actual %s", stored, v10Fingerprint)
	}
	actualV11, err := schemaFingerprint(ctx, conn)
	if err != nil {
		return fmt.Errorf("verify v11 managed schema fingerprint: %w", err)
	}
	if stored := byVersion[OrderItemsV11Version].fingerprint.String; stored != actualV11 {
		return fmt.Errorf("refusing v11 schema drift: stored fingerprint %s, actual %s", stored, actualV11)
	}
	_ = v6
	return nil
}

func verifyOrderItemsV11Tables(ctx context.Context, conn *sql.Conn) error {
	type columnShape struct {
		name, columnType, nullable string
		defaultValue               string
		hasDefault                 bool
		extra, collation           string
	}
	wantColumns := []columnShape{
		{name: "id", columnType: "bigint unsigned", nullable: "NO", extra: "auto_increment"},
		{name: "created_at", columnType: "datetime(3)", nullable: "NO"},
		{name: "updated_at", columnType: "datetime(3)", nullable: "NO"},
		{name: "order_id", columnType: "bigint unsigned", nullable: "NO"},
		{name: "product_id", columnType: "bigint unsigned", nullable: "NO"},
		{name: "product_name_snapshot", columnType: "varchar(128)", nullable: "NO", defaultValue: "", hasDefault: true, collation: "utf8mb4_unicode_ci"},
		{name: "quantity", columnType: "int", nullable: "NO"},
		{name: "unit_price", columnType: "bigint", nullable: "NO"},
		{name: "amount", columnType: "bigint", nullable: "NO"},
	}
	var engine, tableCollation string
	if err := conn.QueryRowContext(ctx, "SELECT ENGINE, COALESCE(TABLE_COLLATION,'') FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='order_items'").Scan(&engine, &tableCollation); err != nil {
		return fmt.Errorf("inspect order_items engine and collation: %w", err)
	}
	if engine != "InnoDB" || tableCollation != "utf8mb4_unicode_ci" {
		return fmt.Errorf("order_items engine/collation mismatch: found %s/%s, expected InnoDB/utf8mb4_unicode_ci", engine, tableCollation)
	}
	rows, err := conn.QueryContext(ctx, "SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_DEFAULT, EXTRA, COALESCE(COLLATION_NAME,'') FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='order_items' ORDER BY ORDINAL_POSITION")
	if err != nil {
		return fmt.Errorf("inspect order_items columns: %w", err)
	}
	actual := make([]columnShape, 0)
	for rows.Next() {
		var got columnShape
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
	if len(actual) != len(wantColumns) {
		return fmt.Errorf("order_items column count mismatch: found %d, expected %d", len(actual), len(wantColumns))
	}
	for i, want := range wantColumns {
		if actual[i] != want {
			return fmt.Errorf("order_items column %d mismatch: found %+v, expected %+v", i+1, actual[i], want)
		}
	}

	wantIndexes := []string{
		"PRIMARY|0|1|id|NULL|BTREE",
		"idx_order_items_order_id|1|1|order_id|NULL|BTREE",
		"idx_order_items_product_id|1|1|product_id|NULL|BTREE",
	}
	rows, err = conn.QueryContext(ctx, "SELECT INDEX_NAME, NON_UNIQUE, SEQ_IN_INDEX, COLUMN_NAME, SUB_PART, INDEX_TYPE FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='order_items' ORDER BY INDEX_NAME, SEQ_IN_INDEX")
	if err != nil {
		return fmt.Errorf("inspect order_items indexes: %w", err)
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
	sort.Strings(wantIndexes)
	if !equalStrings(actualIndexes, wantIndexes) {
		return fmt.Errorf("order_items exact index definition mismatch: found %v, expected %v", actualIndexes, wantIndexes)
	}

	wantForeignKeys := []string{
		"fk_order_items_order_id|order_items|order_id|orders|id|1|RESTRICT|NO ACTION",
		"fk_order_items_product_id|order_items|product_id|store_products|id|1|RESTRICT|NO ACTION",
	}
	rows, err = conn.QueryContext(ctx, `SELECT k.CONSTRAINT_NAME, k.TABLE_NAME, k.COLUMN_NAME, k.REFERENCED_TABLE_NAME, k.REFERENCED_COLUMN_NAME, k.ORDINAL_POSITION, r.DELETE_RULE, r.UPDATE_RULE
		FROM information_schema.KEY_COLUMN_USAGE k
		JOIN information_schema.REFERENTIAL_CONSTRAINTS r
		ON r.CONSTRAINT_SCHEMA=k.CONSTRAINT_SCHEMA AND r.CONSTRAINT_NAME=k.CONSTRAINT_NAME AND r.TABLE_NAME=k.TABLE_NAME
		WHERE k.TABLE_SCHEMA=DATABASE() AND k.TABLE_NAME='order_items' AND k.REFERENCED_TABLE_NAME IS NOT NULL
		ORDER BY k.CONSTRAINT_NAME, k.ORDINAL_POSITION`)
	if err != nil {
		return fmt.Errorf("inspect order_items foreign keys: %w", err)
	}
	actualForeignKeys := make([]string, 0)
	for rows.Next() {
		var name, table, column, referencedTable, referencedColumn, deleteRule, updateRule string
		var ordinal int
		if err := rows.Scan(&name, &table, &column, &referencedTable, &referencedColumn, &ordinal, &deleteRule, &updateRule); err != nil {
			rows.Close()
			return err
		}
		actualForeignKeys = append(actualForeignKeys, fmt.Sprintf("%s|%s|%s|%s|%s|%d|%s|%s", name, table, column, referencedTable, referencedColumn, ordinal, deleteRule, updateRule))
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	sort.Strings(actualForeignKeys)
	sort.Strings(wantForeignKeys)
	if !equalStrings(actualForeignKeys, wantForeignKeys) {
		return fmt.Errorf("order_items exact foreign-key definition mismatch: found %v, expected %v", actualForeignKeys, wantForeignKeys)
	}

	wantChecks := []string{
		"chk_order_items_amount|order_items|amount>=0|YES",
		"chk_order_items_quantity|order_items|quantitybetween1and999|YES",
		"chk_order_items_unit_price|order_items|unit_price>=0|YES",
	}
	rows, err = conn.QueryContext(ctx, `SELECT tc.CONSTRAINT_NAME, tc.TABLE_NAME, cc.CHECK_CLAUSE, tc.ENFORCED
		FROM information_schema.TABLE_CONSTRAINTS tc
		JOIN information_schema.CHECK_CONSTRAINTS cc
		ON cc.CONSTRAINT_SCHEMA=tc.CONSTRAINT_SCHEMA AND cc.CONSTRAINT_NAME=tc.CONSTRAINT_NAME
		WHERE tc.TABLE_SCHEMA=DATABASE() AND tc.TABLE_NAME='order_items' AND tc.CONSTRAINT_TYPE='CHECK'
		ORDER BY tc.CONSTRAINT_NAME`)
	if err != nil {
		return fmt.Errorf("inspect order_items CHECK constraints: %w", err)
	}
	actualChecks := make([]string, 0)
	for rows.Next() {
		var name, table, clause, enforced string
		if err := rows.Scan(&name, &table, &clause, &enforced); err != nil {
			rows.Close()
			return err
		}
		actualChecks = append(actualChecks, fmt.Sprintf("%s|%s|%s|%s", name, table, normalizeCheckClause(clause), enforced))
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	sort.Strings(actualChecks)
	sort.Strings(wantChecks)
	if !equalStrings(actualChecks, wantChecks) {
		return fmt.Errorf("order_items exact CHECK-constraint definition mismatch: found %v, expected %v", actualChecks, wantChecks)
	}

	var channelType, channelNullable, channelDefault, channelCollation string
	if err := conn.QueryRowContext(ctx, `SELECT COLUMN_TYPE, IS_NULLABLE, COALESCE(CAST(COLUMN_DEFAULT AS CHAR), '<NULL>'), COALESCE(COLLATION_NAME, '') FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='orders' AND COLUMN_NAME='channel'`).Scan(&channelType, &channelNullable, &channelDefault, &channelCollation); err != nil {
		return fmt.Errorf("inspect orders.channel column: %w", err)
	}
	if channelType != "varchar(24)" || channelNullable != "NO" || channelDefault != "store_qr" || channelCollation != "utf8mb4_unicode_ci" {
		return fmt.Errorf("orders.channel column has unexpected shape: type=%s nullable=%s default=%q collation=%s", channelType, channelNullable, channelDefault, channelCollation)
	}
	return nil
}

func verifyObjectInventoryV11(objects schemaObjects, base baselineDefinition, v2 incrementalMigrationDefinition, v7 platformExpensesV7Definition, v8 platformExpenseReversalsV8Definition, v9 tableOrderingV9Definition) error {
	filtered := objects
	if !containsSchemaName(objects.tables, "order_items") {
		return errors.New(`v11 table "order_items" is missing`)
	}
	filtered.tables = removeSchemaName(filtered.tables, "order_items")
	if err := verifyObjectInventoryV9(filtered, base, v2, v7, v8, v9); err != nil {
		return err
	}
	return nil
}
