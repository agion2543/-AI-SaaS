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
	PlatformFinanceV14Version  = "schema-v14-platform-finance-20261128"
	PlatformFinanceV14Checksum = "02d6b99f1f79bb19b7cadfded33c827ba83de73cb50a0e17d18d24e8335b12f4"
)

// platformFinanceV14NewTables lists the tables v14 creates.
var platformFinanceV14NewTables = []string{
	"platform_time_logs",
	"platform_income_records",
}

// platformFinanceV14Statements is the exact DDL of the v14 migration, in
// apply order: widen the platform_expenses category CHECK to the two new
// categories, append six new columns to platform_expenses, then create the
// time-log and income-record tables.
//
// The six new platform_expenses columns deliberately carry no CHECK
// constraint and no index: platform_expenses is a v7 table, so a constraint
// or index on it would change every pinned historical schema fingerprint
// (the fingerprint excludes the new columns themselves, but not constraints
// or indexes on their table). Their value ranges are enforced in the service
// layer. The category CHECK keeps its v7 name, so the fingerprint — which
// records constraint names, not clause text — is unchanged by the widened
// clause. History is not rewritten: pre-v14 rows keep cost_type ” (never
// classified) and vendor ” (never recorded); payment_status defaults to
// 'paid' because v7 rows are already-incurred actuals, and both cost flags
// default to 1 because a recorded cash outlay is both a cash and an
// economic cost.
var platformFinanceV14Statements = []string{
	"ALTER TABLE `platform_expenses` DROP CHECK `chk_platform_expenses_category`",
	"ALTER TABLE `platform_expenses` ADD CONSTRAINT `chk_platform_expenses_category` CHECK (`category` IN ('cloud_hosting','messaging_api','labor_contractor','customer_acquisition_travel','equipment','other','ai_services','third_party_services'))",
	"ALTER TABLE `platform_expenses` ADD COLUMN `cost_type` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT ''",
	"ALTER TABLE `platform_expenses` ADD COLUMN `vendor` varchar(120) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT ''",
	"ALTER TABLE `platform_expenses` ADD COLUMN `payment_status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'paid'",
	"ALTER TABLE `platform_expenses` ADD COLUMN `is_recurring` tinyint(1) NOT NULL DEFAULT 0",
	"ALTER TABLE `platform_expenses` ADD COLUMN `is_cash_cost` tinyint(1) NOT NULL DEFAULT 1",
	"ALTER TABLE `platform_expenses` ADD COLUMN `is_economic_cost` tinyint(1) NOT NULL DEFAULT 1",
	`CREATE TABLE ` + "`platform_time_logs`" + ` (
				` + "`id`" + ` bigint unsigned NOT NULL AUTO_INCREMENT,
				` + "`created_at`" + ` datetime(3) NOT NULL,
				` + "`updated_at`" + ` datetime(3) NOT NULL,
				` + "`log_date`" + ` date NOT NULL,
				` + "`work_type`" + ` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL,
				` + "`content`" + ` varchar(500) COLLATE utf8mb4_unicode_ci NOT NULL,
				` + "`hours_scaled`" + ` bigint NOT NULL,
				` + "`reference_hourly_value_cents`" + ` bigint NOT NULL,
				` + "`amount_cents`" + ` bigint NOT NULL,
				` + "`module`" + ` varchar(80) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
				` + "`created_by`" + ` bigint unsigned NOT NULL,
				` + "`idempotency_key`" + ` char(36) COLLATE utf8mb4_unicode_ci NOT NULL,
				PRIMARY KEY (` + "`id`" + `),
				UNIQUE KEY ` + "`uk_platform_time_logs_idempotency_key`" + ` (` + "`idempotency_key`" + `),
				KEY ` + "`idx_platform_time_logs_log_date`" + ` (` + "`log_date`" + `),
				KEY ` + "`idx_platform_time_logs_work_type`" + ` (` + "`work_type`" + `),
				KEY ` + "`idx_platform_time_logs_created_by`" + ` (` + "`created_by`" + `),
				CONSTRAINT ` + "`chk_platform_time_logs_work_type`" + ` CHECK (` + "`work_type`" + ` IN ('product_planning','requirement_analysis','development','testing','deployment_maintenance','business_communication','content_creation')),
				CONSTRAINT ` + "`chk_platform_time_logs_hours_scaled`" + ` CHECK (` + "`hours_scaled`" + ` BETWEEN 1 AND 2400),
				CONSTRAINT ` + "`chk_platform_time_logs_reference_value`" + ` CHECK (` + "`reference_hourly_value_cents`" + ` > 0),
				CONSTRAINT ` + "`chk_platform_time_logs_amount`" + ` CHECK (` + "`amount_cents`" + ` > 0)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	`CREATE TABLE ` + "`platform_income_records`" + ` (
				` + "`id`" + ` bigint unsigned NOT NULL AUTO_INCREMENT,
				` + "`created_at`" + ` datetime(3) NOT NULL,
				` + "`updated_at`" + ` datetime(3) NOT NULL,
				` + "`merchant_id`" + ` bigint unsigned NULL,
				` + "`package_id`" + ` bigint unsigned NULL,
				` + "`income_type`" + ` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
				` + "`amount_cents`" + ` bigint NOT NULL,
				` + "`currency`" + ` char(3) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'CNY',
				` + "`occurred_date`" + ` date NOT NULL,
				` + "`received_date`" + ` date NULL,
				` + "`status`" + ` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'ordered',
				` + "`refund_amount_cents`" + ` bigint NOT NULL DEFAULT 0,
				` + "`billing_period_months`" + ` int NOT NULL DEFAULT 1,
				` + "`subscription_start_date`" + ` date NULL,
				` + "`subscription_end_date`" + ` date NULL,
				` + "`is_cancelled`" + ` tinyint(1) NOT NULL DEFAULT 0,
				` + "`order_id`" + ` bigint unsigned NULL,
				` + "`note`" + ` varchar(500) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
				` + "`created_by`" + ` bigint unsigned NOT NULL,
				` + "`idempotency_key`" + ` char(36) COLLATE utf8mb4_unicode_ci NOT NULL,
				PRIMARY KEY (` + "`id`" + `),
				UNIQUE KEY ` + "`uk_platform_income_records_idempotency_key`" + ` (` + "`idempotency_key`" + `),
				KEY ` + "`idx_platform_income_records_merchant`" + ` (` + "`merchant_id`" + `),
				KEY ` + "`idx_platform_income_records_status_date`" + ` (` + "`status`" + `, ` + "`occurred_date`" + `),
				KEY ` + "`idx_platform_income_records_created_by`" + ` (` + "`created_by`" + `),
				CONSTRAINT ` + "`chk_platform_income_records_type`" + ` CHECK (` + "`income_type`" + ` IN ('subscription_basic','subscription_premium','addon_module','deployment_setup','ai_usage_package','other')),
				CONSTRAINT ` + "`chk_platform_income_records_amount`" + ` CHECK (` + "`amount_cents`" + ` > 0),
				CONSTRAINT ` + "`chk_platform_income_records_currency`" + ` CHECK (` + "`currency`" + ` = 'CNY'),
				CONSTRAINT ` + "`chk_platform_income_records_status`" + ` CHECK (` + "`status`" + ` IN ('ordered','confirmed','received','pending_receipt','refunded','refunding')),
				CONSTRAINT ` + "`chk_platform_income_records_refund_amount`" + ` CHECK (` + "`refund_amount_cents`" + ` >= 0),
				CONSTRAINT ` + "`chk_platform_income_records_billing_period`" + ` CHECK (` + "`billing_period_months`" + ` BETWEEN 1 AND 12),
				CONSTRAINT ` + "`chk_platform_income_records_subscription_period`" + ` CHECK (` + "`subscription_end_date`" + ` IS NULL OR ` + "`subscription_start_date`" + ` IS NULL OR ` + "`subscription_end_date`" + ` >= ` + "`subscription_start_date`" + `)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
}

// platformFinanceV14NewColumns lists the columns v14 adds to pre-existing
// tables, so historical fingerprints can exclude them.
var platformFinanceV14NewColumns = []schemaColumn{
	{table: "platform_expenses", column: "cost_type"},
	{table: "platform_expenses", column: "vendor"},
	{table: "platform_expenses", column: "payment_status"},
	{table: "platform_expenses", column: "is_recurring"},
	{table: "platform_expenses", column: "is_cash_cost"},
	{table: "platform_expenses", column: "is_economic_cost"},
}

func validatePlatformFinanceV14Definition() error {
	material := append([]string{PlatformFinanceV14Version}, platformFinanceV14Statements...)
	sum := sha256.Sum256([]byte(strings.Join(material, "\n")))
	if got := fmt.Sprintf("%x", sum); got != PlatformFinanceV14Checksum {
		return fmt.Errorf("platform finance v14 checksum mismatch: definition %s, expected %s", got, PlatformFinanceV14Checksum)
	}
	return nil
}

// ApplyPlatformFinanceV14 widens the platform_expenses categories, appends
// the six v14 columns to it, and creates the time-log and income-record
// tables on an exact managed v13 schema. The v1-v13 schema and fingerprints
// remain untouched.
func ApplyPlatformFinanceV14(ctx context.Context, db *sql.DB) (Result, error) {
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
	if err := validatePlatformFinanceV14Definition(); err != nil {
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
	if err := verifyManagedSchemaV13(ctx, conn, objects, base, v2, v3, v4, v5, v6, v7, v8, v9); err == nil {
		if err := applyPlatformFinanceV14(ctx, conn, base, v2, v3, v4, v5, v6, v7, v8, v9); err != nil {
			return "", err
		}
		return ResultApplied, nil
	}
	if err := verifyManagedSchemaV14(ctx, conn, objects, base, v2, v3, v4, v5, v6, v7, v8, v9); err != nil {
		return "", fmt.Errorf("refusing non-exact managed schema for v14 platform finance migration: %w", err)
	}
	return ResultAlready, nil
}

func applyPlatformFinanceV14(ctx context.Context, conn *sql.Conn, base baselineDefinition, v2 incrementalMigrationDefinition, v3 adminAuthV3Definition, v4 otpSafetyV4Definition, v5 emailOTPSafetyV5Definition, v6 wholesaleRetailV6Definition, v7 platformExpensesV7Definition, v8 platformExpenseReversalsV8Definition, v9 tableOrderingV9Definition) error {
	if _, err := conn.ExecContext(ctx, `INSERT INTO `+ledgerTable+` (version, checksum, state) VALUES (?, ?, 'applying')`, PlatformFinanceV14Version, PlatformFinanceV14Checksum); err != nil {
		return fmt.Errorf("record v14 platform finance migration start: %w", err)
	}
	for i, statement := range platformFinanceV14Statements {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply v14 platform finance change %d (v14 ledger remains incomplete): %w", i+1, err)
		}
	}
	if err := verifyPlatformFinanceV14Tables(ctx, conn); err != nil {
		return fmt.Errorf("verify v14 platform finance tables (v14 ledger remains incomplete): %w", err)
	}
	objects, err := inspectObjects(ctx, conn)
	if err != nil {
		return err
	}
	if err := verifyObjectInventoryV14(objects, base, v2, v7, v8, v9); err != nil {
		return fmt.Errorf("post-apply v14 inventory verification failed (v14 ledger remains incomplete): %w", err)
	}
	// The stored v13 fingerprint is computed over the schema without the v14
	// tables and without the six new platform_expenses columns; the widened
	// category CHECK keeps its v7 name, so its clause text is not part of
	// any fingerprint and needs no exclusion.
	v13Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, platformFinanceV14NewTables, nil, platformFinanceV14NewColumns)
	if err != nil {
		return fmt.Errorf("compute immutable v13 schema fingerprint during v14 migration: %w", err)
	}
	var storedV13 string
	if err := conn.QueryRowContext(ctx, "SELECT schema_fingerprint FROM "+ledgerTable+" WHERE version=? AND checksum=? AND state='applied'", AnnouncementsMaintenanceV13Version, AnnouncementsMaintenanceV13Checksum).Scan(&storedV13); err != nil {
		return fmt.Errorf("read immutable v13 schema fingerprint during v14 migration: %w", err)
	}
	if storedV13 != v13Fingerprint {
		return fmt.Errorf("refusing v13 schema drift during v14 migration: stored %s, actual %s", storedV13, v13Fingerprint)
	}
	fingerprint, err := schemaFingerprint(ctx, conn)
	if err != nil {
		return fmt.Errorf("compute v14 installed schema fingerprint: %w", err)
	}
	result, err := conn.ExecContext(ctx, `UPDATE `+ledgerTable+` SET state='applied', schema_fingerprint=?, applied_at=CURRENT_TIMESTAMP WHERE version=? AND checksum=? AND state='applying'`, fingerprint, PlatformFinanceV14Version, PlatformFinanceV14Checksum)
	if err != nil {
		return fmt.Errorf("complete v14 schema version ledger: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("verify v14 schema version ledger update: %w", err)
	}
	if rows != 1 {
		return errors.New("v14 schema version ledger did not transition exactly once; it remains incomplete")
	}
	_ = v3
	_ = v4
	_ = v5
	_ = v6
	return nil
}

func verifyManagedSchemaV14(ctx context.Context, conn *sql.Conn, objects schemaObjects, base baselineDefinition, v2 incrementalMigrationDefinition, v3 adminAuthV3Definition, v4 otpSafetyV4Definition, v5 emailOTPSafetyV5Definition, v6 wholesaleRetailV6Definition, v7 platformExpensesV7Definition, v8 platformExpenseReversalsV8Definition, v9 tableOrderingV9Definition) error {
	if err := verifyPlatformFinanceV14Tables(ctx, conn); err != nil {
		return err
	}
	if err := verifyObjectInventoryV14(objects, base, v2, v7, v8, v9); err != nil {
		return err
	}
	combinedNewColumns := append(append(append([]schemaColumn(nil), orderItemsV11NewColumns...), aiCostControlV12NewColumns...), platformFinanceV14NewColumns...)
	futureTables := append(append(append(append(append([]string(nil), tableOrderingV9Order...), "order_items"), aiCostControlV12NewTables...), announcementsMaintenanceV13NewTables...), platformFinanceV14NewTables...)
	if err := verifyPlatformExpenseReversalsV8HistoricalFingerprintsExcludingColumns(ctx, conn, base, v2, v3, v4, v5, v6, v7, v8, combinedNewColumns, futureTables...); err != nil {
		return err
	}
	rows, err := conn.QueryContext(ctx, "SELECT version, checksum, state, schema_fingerprint FROM "+ledgerTable+" ORDER BY version")
	if err != nil {
		return fmt.Errorf("read v14 schema version ledger: %w", err)
	}
	type ledgerEntry struct {
		version, checksum, state string
		fingerprint              sql.NullString
	}
	entries := make([]ledgerEntry, 0, 14)
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
	if len(entries) != 14 {
		return fmt.Errorf("refusing v14 ledger with %d rows; expected exactly immutable v1-v14 rows", len(entries))
	}
	expected := []struct{ version, checksum string }{
		{BaselineVersion, base.checksum}, {v2.version, v2.checksum}, {v3.version, v3.checksum}, {v4.version, v4.checksum},
		{v5.version, v5.checksum}, {v6.version, v6.checksum}, {v7.version, v7.checksum}, {v8.version, v8.checksum},
		{v9.version, v9.checksum}, {TableLabelsV10Version, TableLabelsV10Checksum}, {OrderItemsV11Version, OrderItemsV11Checksum},
		{AICostControlV12Version, AICostControlV12Checksum},
		{AnnouncementsMaintenanceV13Version, AnnouncementsMaintenanceV13Checksum},
		{PlatformFinanceV14Version, PlatformFinanceV14Checksum},
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
	// v9 check: exclude v12/v13/v14 tables + order_items, and v10/v11/v12/v14 new columns.
	v9ExcludedColumns := append(append([]schemaColumn(nil), combinedNewColumns...), v10NewColumns...)
	v9ExcludedTables := append(append(append(append([]string(nil), "order_items"), aiCostControlV12NewTables...), announcementsMaintenanceV13NewTables...), platformFinanceV14NewTables...)
	v9Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, v9ExcludedTables, nil, v9ExcludedColumns)
	if err != nil {
		return fmt.Errorf("verify immutable v9 schema fingerprint: %w", err)
	}
	if stored := byVersion[v9.version].fingerprint.String; stored != v9Fingerprint {
		return fmt.Errorf("refusing v9 schema drift: stored fingerprint %s, actual %s", stored, v9Fingerprint)
	}
	// v10 check: same tables, v11/v12/v14 new columns.
	v10Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, v9ExcludedTables, nil, combinedNewColumns)
	if err != nil {
		return fmt.Errorf("verify immutable v10 schema fingerprint: %w", err)
	}
	if stored := byVersion[TableLabelsV10Version].fingerprint.String; stored != v10Fingerprint {
		return fmt.Errorf("refusing v10 schema drift: stored fingerprint %s, actual %s", stored, v10Fingerprint)
	}
	// v11 check: exclude the v12/v13/v14 tables, and the v12/v14 columns
	// (orders.channel is part of the stored v11 fingerprint; v13 added no
	// columns to pre-existing tables).
	v11ExcludedColumns := append(append([]schemaColumn(nil), aiCostControlV12NewColumns...), platformFinanceV14NewColumns...)
	v11ExcludedTables := append(append(append([]string(nil), aiCostControlV12NewTables...), announcementsMaintenanceV13NewTables...), platformFinanceV14NewTables...)
	v11Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, v11ExcludedTables, nil, v11ExcludedColumns)
	if err != nil {
		return fmt.Errorf("verify immutable v11 schema fingerprint: %w", err)
	}
	if stored := byVersion[OrderItemsV11Version].fingerprint.String; stored != v11Fingerprint {
		return fmt.Errorf("refusing v11 schema drift: stored fingerprint %s, actual %s", stored, v11Fingerprint)
	}
	// v12 check: exclude the v13/v14 tables and the v14 columns (the v12
	// columns are part of the stored v12 fingerprint).
	v12Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, append(append([]string(nil), announcementsMaintenanceV13NewTables...), platformFinanceV14NewTables...), nil, platformFinanceV14NewColumns)
	if err != nil {
		return fmt.Errorf("verify immutable v12 schema fingerprint: %w", err)
	}
	if stored := byVersion[AICostControlV12Version].fingerprint.String; stored != v12Fingerprint {
		return fmt.Errorf("refusing v12 schema drift: stored fingerprint %s, actual %s", stored, v12Fingerprint)
	}
	// v13 check: exclude only the v14 tables and the v14 columns (v13 added
	// no columns to pre-existing tables).
	v13Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, platformFinanceV14NewTables, nil, platformFinanceV14NewColumns)
	if err != nil {
		return fmt.Errorf("verify immutable v13 schema fingerprint: %w", err)
	}
	if stored := byVersion[AnnouncementsMaintenanceV13Version].fingerprint.String; stored != v13Fingerprint {
		return fmt.Errorf("refusing v13 schema drift: stored fingerprint %s, actual %s", stored, v13Fingerprint)
	}
	actualV14, err := schemaFingerprint(ctx, conn)
	if err != nil {
		return fmt.Errorf("verify v14 managed schema fingerprint: %w", err)
	}
	if stored := byVersion[PlatformFinanceV14Version].fingerprint.String; stored != actualV14 {
		return fmt.Errorf("refusing v14 schema drift: stored fingerprint %s, actual %s", stored, actualV14)
	}
	_ = v6
	return nil
}

// verifyPlatformFinanceV14Tables checks the exact shape of the two new
// tables and of the altered platform_expenses table (16 columns, the five
// original indexes, and the four CHECK constraints with the widened
// category clause).
func verifyPlatformFinanceV14Tables(ctx context.Context, conn *sql.Conn) error {
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
			name: "platform_time_logs",
			columns: []columnSpec{
				col("id", "bigint unsigned", "NO", "", false, "auto_increment", ""),
				col("created_at", "datetime(3)", "NO", "", false, "", ""),
				col("updated_at", "datetime(3)", "NO", "", false, "", ""),
				col("log_date", "date", "NO", "", false, "", ""),
				col("work_type", "varchar(24)", "NO", "", false, "", "utf8mb4_unicode_ci"),
				col("content", "varchar(500)", "NO", "", false, "", "utf8mb4_unicode_ci"),
				col("hours_scaled", "bigint", "NO", "", false, "", ""),
				col("reference_hourly_value_cents", "bigint", "NO", "", false, "", ""),
				col("amount_cents", "bigint", "NO", "", false, "", ""),
				col("module", "varchar(80)", "NO", "", true, "", "utf8mb4_unicode_ci"),
				col("created_by", "bigint unsigned", "NO", "", false, "", ""),
				col("idempotency_key", "char(36)", "NO", "", false, "", "utf8mb4_unicode_ci"),
			},
			indexes: []string{
				"PRIMARY|0|1|id|NULL|BTREE",
				"uk_platform_time_logs_idempotency_key|0|1|idempotency_key|NULL|BTREE",
				"idx_platform_time_logs_log_date|1|1|log_date|NULL|BTREE",
				"idx_platform_time_logs_work_type|1|1|work_type|NULL|BTREE",
				"idx_platform_time_logs_created_by|1|1|created_by|NULL|BTREE",
			},
			checks: []string{
				"chk_platform_time_logs_amount|platform_time_logs|amount_cents>0|YES",
				"chk_platform_time_logs_hours_scaled|platform_time_logs|hours_scaledbetween1and2400|YES",
				"chk_platform_time_logs_reference_value|platform_time_logs|reference_hourly_value_cents>0|YES",
				"chk_platform_time_logs_work_type|platform_time_logs|work_typein'product_planning','requirement_analysis','development','testing','deployment_maintenance','business_communication','content_creation'|YES",
			},
		},
		{
			name: "platform_income_records",
			columns: []columnSpec{
				col("id", "bigint unsigned", "NO", "", false, "auto_increment", ""),
				col("created_at", "datetime(3)", "NO", "", false, "", ""),
				col("updated_at", "datetime(3)", "NO", "", false, "", ""),
				col("merchant_id", "bigint unsigned", "YES", "", false, "", ""),
				col("package_id", "bigint unsigned", "YES", "", false, "", ""),
				col("income_type", "varchar(32)", "NO", "", false, "", "utf8mb4_unicode_ci"),
				col("amount_cents", "bigint", "NO", "", false, "", ""),
				col("currency", "char(3)", "NO", "CNY", true, "", "utf8mb4_unicode_ci"),
				col("occurred_date", "date", "NO", "", false, "", ""),
				col("received_date", "date", "YES", "", false, "", ""),
				col("status", "varchar(16)", "NO", "ordered", true, "", "utf8mb4_unicode_ci"),
				col("refund_amount_cents", "bigint", "NO", "0", true, "", ""),
				col("billing_period_months", "int", "NO", "1", true, "", ""),
				col("subscription_start_date", "date", "YES", "", false, "", ""),
				col("subscription_end_date", "date", "YES", "", false, "", ""),
				col("is_cancelled", "tinyint(1)", "NO", "0", true, "", ""),
				col("order_id", "bigint unsigned", "YES", "", false, "", ""),
				col("note", "varchar(500)", "NO", "", true, "", "utf8mb4_unicode_ci"),
				col("created_by", "bigint unsigned", "NO", "", false, "", ""),
				col("idempotency_key", "char(36)", "NO", "", false, "", "utf8mb4_unicode_ci"),
			},
			indexes: []string{
				"PRIMARY|0|1|id|NULL|BTREE",
				"uk_platform_income_records_idempotency_key|0|1|idempotency_key|NULL|BTREE",
				"idx_platform_income_records_merchant|1|1|merchant_id|NULL|BTREE",
				"idx_platform_income_records_status_date|1|1|status|NULL|BTREE",
				"idx_platform_income_records_status_date|1|2|occurred_date|NULL|BTREE",
				"idx_platform_income_records_created_by|1|1|created_by|NULL|BTREE",
			},
			checks: []string{
				"chk_platform_income_records_amount|platform_income_records|amount_cents>0|YES",
				"chk_platform_income_records_billing_period|platform_income_records|billing_period_monthsbetween1and12|YES",
				"chk_platform_income_records_currency|platform_income_records|currency='cny'|YES",
				"chk_platform_income_records_refund_amount|platform_income_records|refund_amount_cents>=0|YES",
				"chk_platform_income_records_status|platform_income_records|statusin'ordered','confirmed','received','pending_receipt','refunded','refunding'|YES",
				"chk_platform_income_records_subscription_period|platform_income_records|subscription_end_dateisnullorsubscription_start_dateisnullorsubscription_end_date>=subscription_start_date|YES",
				"chk_platform_income_records_type|platform_income_records|income_typein'subscription_basic','subscription_premium','addon_module','deployment_setup','ai_usage_package','other'|YES",
			},
		},
		{
			name: "platform_expenses",
			columns: []columnSpec{
				col("id", "bigint unsigned", "NO", "", false, "auto_increment", ""),
				col("created_at", "datetime(3)", "NO", "", false, "", ""),
				col("updated_at", "datetime(3)", "NO", "", false, "", ""),
				col("category", "varchar(40)", "NO", "", false, "", "utf8mb4_unicode_ci"),
				col("amount_cents", "bigint", "NO", "", false, "", ""),
				col("currency", "char(3)", "NO", "CNY", true, "", "utf8mb4_unicode_ci"),
				col("expense_date", "date", "NO", "", false, "", ""),
				col("memo", "varchar(120)", "NO", "", false, "", "utf8mb4_unicode_ci"),
				col("created_by", "bigint unsigned", "NO", "", false, "", ""),
				col("idempotency_key", "char(36)", "NO", "", false, "", "utf8mb4_unicode_ci"),
				col("cost_type", "varchar(16)", "NO", "", true, "", "utf8mb4_unicode_ci"),
				col("vendor", "varchar(120)", "NO", "", true, "", "utf8mb4_unicode_ci"),
				col("payment_status", "varchar(16)", "NO", "paid", true, "", "utf8mb4_unicode_ci"),
				col("is_recurring", "tinyint(1)", "NO", "0", true, "", ""),
				col("is_cash_cost", "tinyint(1)", "NO", "1", true, "", ""),
				col("is_economic_cost", "tinyint(1)", "NO", "1", true, "", ""),
			},
			indexes: []string{
				"PRIMARY|0|1|id|NULL|BTREE",
				"uk_platform_expenses_idempotency_key|0|1|idempotency_key|NULL|BTREE",
				"idx_platform_expenses_expense_date|1|1|expense_date|NULL|BTREE",
				"idx_platform_expenses_category_date|1|1|category|NULL|BTREE",
				"idx_platform_expenses_category_date|1|2|expense_date|NULL|BTREE",
				"idx_platform_expenses_created_by|1|1|created_by|NULL|BTREE",
			},
			checks: []string{
				"chk_platform_expenses_amount_positive|platform_expenses|amount_cents>0|YES",
				"chk_platform_expenses_category|platform_expenses|categoryin'cloud_hosting','messaging_api','labor_contractor','customer_acquisition_travel','equipment','other','ai_services','third_party_services'|YES",
				"chk_platform_expenses_currency|platform_expenses|currency='cny'|YES",
				"chk_platform_expenses_memo|platform_expenses|lengthtrimmemo>0|YES",
			},
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

func verifyObjectInventoryV14(objects schemaObjects, base baselineDefinition, v2 incrementalMigrationDefinition, v7 platformExpensesV7Definition, v8 platformExpenseReversalsV8Definition, v9 tableOrderingV9Definition) error {
	filtered := objects
	for _, name := range platformFinanceV14NewTables {
		if !containsSchemaName(objects.tables, name) {
			return fmt.Errorf("v14 table %q is missing", name)
		}
		filtered.tables = removeSchemaName(filtered.tables, name)
	}
	if err := verifyObjectInventoryV13(filtered, base, v2, v7, v8, v9); err != nil {
		return err
	}
	return nil
}
