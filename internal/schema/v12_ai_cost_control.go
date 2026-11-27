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
	AICostControlV12Version  = "schema-v12-ai-cost-control-20261127"
	AICostControlV12Checksum = "ece3b16eb1d693320d5d3f808111a67fc49b0f31cf5afeb2ab232ffaca67bae6"
)

// aiCostControlV12NewTables lists the tables v12 creates.
var aiCostControlV12NewTables = []string{
	"ai_model_prices",
	"ai_vendor_bills",
	"merchant_ai_budgets",
	"merchant_ai_budget_alerts",
	"ai_emergency_switch",
}

// aiCostControlV12Statements is the exact DDL of the v12 migration, in apply
// order: five new cost-control tables, two new columns on the v1
// merchant_ai_usage_logs table (store attribution, cached tokens), then the
// single emergency-switch seed row (id=1, AI enabled).
// The new usage-log columns deliberately carry no CHECK constraint and the
// store_id column carries no foreign key: merchant_ai_usage_logs is a v1
// table, so a constraint or FK on it would change every pinned historical
// schema fingerprint (the fingerprint excludes the new columns themselves,
// but not constraints on their table). Store attribution is NULL when an AI
// call carries no store context; it is never invented.
var aiCostControlV12Statements = []string{
	`CREATE TABLE ` + "`ai_model_prices`" + ` (
				` + "`id`" + ` bigint unsigned NOT NULL AUTO_INCREMENT,
				` + "`created_at`" + ` datetime(3) NOT NULL,
				` + "`updated_at`" + ` datetime(3) NOT NULL,
				` + "`model`" + ` varchar(120) COLLATE utf8mb4_unicode_ci NOT NULL,
				` + "`provider`" + ` varchar(80) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
				` + "`input_price_cents_per_million`" + ` bigint NOT NULL,
				` + "`output_price_cents_per_million`" + ` bigint NOT NULL,
				` + "`cached_price_cents_per_million`" + ` bigint NOT NULL DEFAULT 0,
				` + "`currency`" + ` char(3) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'CNY',
				` + "`effective_from`" + ` datetime(3) NOT NULL,
				` + "`created_by`" + ` bigint unsigned NOT NULL,
				` + "`updated_by`" + ` bigint unsigned NOT NULL DEFAULT 0,
				` + "`is_active`" + ` tinyint(1) NOT NULL DEFAULT 1,
				PRIMARY KEY (` + "`id`" + `),
				KEY ` + "`idx_ai_model_prices_model_effective`" + ` (` + "`model`" + `, ` + "`effective_from`" + `),
				UNIQUE KEY ` + "`uk_ai_model_prices_model_provider_effective`" + ` (` + "`model`" + `, ` + "`provider`" + `, ` + "`effective_from`" + `),
				CONSTRAINT ` + "`chk_ai_model_prices_input`" + ` CHECK (` + "`input_price_cents_per_million`" + ` >= 0),
				CONSTRAINT ` + "`chk_ai_model_prices_output`" + ` CHECK (` + "`output_price_cents_per_million`" + ` >= 0),
				CONSTRAINT ` + "`chk_ai_model_prices_cached`" + ` CHECK (` + "`cached_price_cents_per_million`" + ` >= 0),
				CONSTRAINT ` + "`chk_ai_model_prices_currency`" + ` CHECK (` + "`currency`" + ` IN ('CNY', 'USD'))
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	`CREATE TABLE ` + "`ai_vendor_bills`" + ` (
				` + "`id`" + ` bigint unsigned NOT NULL AUTO_INCREMENT,
				` + "`created_at`" + ` datetime(3) NOT NULL,
				` + "`updated_at`" + ` datetime(3) NOT NULL,
				` + "`vendor`" + ` varchar(120) COLLATE utf8mb4_unicode_ci NOT NULL,
				` + "`period_start`" + ` date NOT NULL,
				` + "`period_end`" + ` date NOT NULL,
				` + "`amount_cents`" + ` bigint NOT NULL,
				` + "`currency`" + ` char(3) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'CNY',
				` + "`note`" + ` varchar(500) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
				` + "`created_by`" + ` bigint unsigned NOT NULL,
				` + "`idempotency_key`" + ` char(36) COLLATE utf8mb4_unicode_ci NOT NULL,
				PRIMARY KEY (` + "`id`" + `),
				UNIQUE KEY ` + "`uk_ai_vendor_bills_idempotency_key`" + ` (` + "`idempotency_key`" + `),
				KEY ` + "`idx_ai_vendor_bills_vendor_period`" + ` (` + "`vendor`" + `, ` + "`period_start`" + `),
				KEY ` + "`idx_ai_vendor_bills_period_start`" + ` (` + "`period_start`" + `),
				CONSTRAINT ` + "`chk_ai_vendor_bills_amount`" + ` CHECK (` + "`amount_cents`" + ` > 0),
				CONSTRAINT ` + "`chk_ai_vendor_bills_currency`" + ` CHECK (` + "`currency`" + ` IN ('CNY', 'USD')),
				CONSTRAINT ` + "`chk_ai_vendor_bills_period`" + ` CHECK (` + "`period_end`" + ` >= ` + "`period_start`" + `)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	`CREATE TABLE ` + "`merchant_ai_budgets`" + ` (
				` + "`id`" + ` bigint unsigned NOT NULL AUTO_INCREMENT,
				` + "`created_at`" + ` datetime(3) NOT NULL,
				` + "`updated_at`" + ` datetime(3) NOT NULL,
				` + "`merchant_id`" + ` bigint unsigned NOT NULL,
				` + "`daily_budget_cents`" + ` bigint NOT NULL DEFAULT 0,
				` + "`monthly_budget_cents`" + ` bigint NOT NULL DEFAULT 0,
				` + "`over_limit_action`" + ` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'stop',
				` + "`updated_by`" + ` bigint unsigned NOT NULL,
				PRIMARY KEY (` + "`id`" + `),
				UNIQUE KEY ` + "`uk_merchant_ai_budgets_merchant`" + ` (` + "`merchant_id`" + `),
				CONSTRAINT ` + "`chk_merchant_ai_budgets_daily`" + ` CHECK (` + "`daily_budget_cents`" + ` >= 0),
				CONSTRAINT ` + "`chk_merchant_ai_budgets_monthly`" + ` CHECK (` + "`monthly_budget_cents`" + ` >= 0),
				CONSTRAINT ` + "`chk_merchant_ai_budgets_action`" + ` CHECK (` + "`over_limit_action`" + ` IN ('stop', 'downgrade', 'restrict_expensive', 'manual_review'))
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	`CREATE TABLE ` + "`merchant_ai_budget_alerts`" + ` (
				` + "`id`" + ` bigint unsigned NOT NULL AUTO_INCREMENT,
				` + "`created_at`" + ` datetime(3) NOT NULL,
				` + "`updated_at`" + ` datetime(3) NOT NULL,
				` + "`merchant_id`" + ` bigint unsigned NOT NULL,
				` + "`period_type`" + ` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL,
				` + "`period_start`" + ` date NOT NULL,
				` + "`threshold`" + ` int NOT NULL,
				` + "`used_tokens`" + ` bigint NOT NULL DEFAULT 0,
				` + "`estimated_cost_cents`" + ` bigint NOT NULL DEFAULT 0,
				PRIMARY KEY (` + "`id`" + `),
				KEY ` + "`idx_merchant_ai_budget_alerts_merchant`" + ` (` + "`merchant_id`" + `),
				KEY ` + "`idx_merchant_ai_budget_alerts_period`" + ` (` + "`period_start`" + `),
				UNIQUE KEY ` + "`uk_merchant_ai_budget_alerts_scope`" + ` (` + "`merchant_id`" + `, ` + "`period_type`" + `, ` + "`period_start`" + `, ` + "`threshold`" + `),
				CONSTRAINT ` + "`chk_merchant_ai_budget_alerts_period`" + ` CHECK (` + "`period_type`" + ` IN ('daily', 'monthly')),
				CONSTRAINT ` + "`chk_merchant_ai_budget_alerts_threshold`" + ` CHECK (` + "`threshold`" + ` IN (70, 90, 100)),
				CONSTRAINT ` + "`chk_merchant_ai_budget_alerts_used_tokens`" + ` CHECK (` + "`used_tokens`" + ` >= 0),
				CONSTRAINT ` + "`chk_merchant_ai_budget_alerts_cost`" + ` CHECK (` + "`estimated_cost_cents`" + ` >= 0)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	`CREATE TABLE ` + "`ai_emergency_switch`" + ` (
				` + "`id`" + ` bigint unsigned NOT NULL,
				` + "`created_at`" + ` datetime(3) NOT NULL,
				` + "`updated_at`" + ` datetime(3) NOT NULL,
				` + "`disabled`" + ` tinyint(1) NOT NULL DEFAULT 0,
				` + "`reason`" + ` varchar(500) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
				` + "`updated_by`" + ` bigint unsigned NOT NULL DEFAULT 0,
				PRIMARY KEY (` + "`id`" + `)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	"ALTER TABLE `merchant_ai_usage_logs` ADD COLUMN `store_id` bigint unsigned NULL",
	"ALTER TABLE `merchant_ai_usage_logs` ADD COLUMN `cached_tokens` bigint NOT NULL DEFAULT 0",
	"INSERT INTO `ai_emergency_switch` (`id`, `created_at`, `updated_at`, `disabled`, `reason`, `updated_by`) VALUES (1, NOW(3), NOW(3), 0, '', 0)",
}

// aiCostControlV12NewColumns lists the columns v12 adds to pre-existing
// tables, so historical fingerprints can exclude them.
var aiCostControlV12NewColumns = []schemaColumn{
	{table: "merchant_ai_usage_logs", column: "store_id"},
	{table: "merchant_ai_usage_logs", column: "cached_tokens"},
}

func validateAICostControlV12Definition() error {
	material := append([]string{AICostControlV12Version}, aiCostControlV12Statements...)
	sum := sha256.Sum256([]byte(strings.Join(material, "\n")))
	if got := fmt.Sprintf("%x", sum); got != AICostControlV12Checksum {
		return fmt.Errorf("ai cost control v12 checksum mismatch: definition %s, expected %s", got, AICostControlV12Checksum)
	}
	return nil
}

// ApplyAICostControlV12 appends the AI cost-control tables (model prices,
// vendor bills, merchant budgets, budget alerts, emergency switch) and the
// merchant_ai_usage_logs store/cached-token columns to an exact managed v11
// schema. The v1-v11 schema and fingerprints remain untouched.
func ApplyAICostControlV12(ctx context.Context, db *sql.DB) (Result, error) {
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
	if err := validateAICostControlV12Definition(); err != nil {
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
	if err := verifyManagedSchemaV11(ctx, conn, objects, base, v2, v3, v4, v5, v6, v7, v8, v9); err == nil {
		if err := applyAICostControlV12(ctx, conn, base, v2, v3, v4, v5, v6, v7, v8, v9); err != nil {
			return "", err
		}
		return ResultApplied, nil
	}
	if err := verifyManagedSchemaV12(ctx, conn, objects, base, v2, v3, v4, v5, v6, v7, v8, v9); err != nil {
		return "", fmt.Errorf("refusing non-exact managed schema for v12 ai cost control migration: %w", err)
	}
	return ResultAlready, nil
}

func applyAICostControlV12(ctx context.Context, conn *sql.Conn, base baselineDefinition, v2 incrementalMigrationDefinition, v3 adminAuthV3Definition, v4 otpSafetyV4Definition, v5 emailOTPSafetyV5Definition, v6 wholesaleRetailV6Definition, v7 platformExpensesV7Definition, v8 platformExpenseReversalsV8Definition, v9 tableOrderingV9Definition) error {
	if _, err := conn.ExecContext(ctx, `INSERT INTO `+ledgerTable+` (version, checksum, state) VALUES (?, ?, 'applying')`, AICostControlV12Version, AICostControlV12Checksum); err != nil {
		return fmt.Errorf("record v12 ai cost control migration start: %w", err)
	}
	for i, statement := range aiCostControlV12Statements {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply v12 ai cost control change %d (v12 ledger remains incomplete): %w", i+1, err)
		}
	}
	if err := verifyAICostControlV12Tables(ctx, conn); err != nil {
		return fmt.Errorf("verify v12 ai cost control tables (v12 ledger remains incomplete): %w", err)
	}
	objects, err := inspectObjects(ctx, conn)
	if err != nil {
		return err
	}
	if err := verifyObjectInventoryV12(objects, base, v2, v7, v8, v9); err != nil {
		return fmt.Errorf("post-apply v12 object inventory verification failed (v12 ledger remains incomplete): %w", err)
	}
	// The stored v11 fingerprint covers orders.channel (added by v11), so
	// only the v12 columns are excluded here; excluding channel would break
	// the comparison.
	v11Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, aiCostControlV12NewTables, nil, aiCostControlV12NewColumns)
	if err != nil {
		return fmt.Errorf("compute immutable v11 schema fingerprint during v12 migration: %w", err)
	}
	var storedV11 string
	if err := conn.QueryRowContext(ctx, "SELECT schema_fingerprint FROM "+ledgerTable+" WHERE version=? AND checksum=? AND state='applied'", OrderItemsV11Version, OrderItemsV11Checksum).Scan(&storedV11); err != nil {
		return fmt.Errorf("read immutable v11 schema fingerprint during v12 migration: %w", err)
	}
	if storedV11 != v11Fingerprint {
		return fmt.Errorf("refusing v11 schema drift during v12 migration: stored %s, actual %s", storedV11, v11Fingerprint)
	}
	fingerprint, err := schemaFingerprint(ctx, conn)
	if err != nil {
		return fmt.Errorf("compute v12 installed schema fingerprint: %w", err)
	}
	result, err := conn.ExecContext(ctx, `UPDATE `+ledgerTable+` SET state='applied', schema_fingerprint=?, applied_at=CURRENT_TIMESTAMP WHERE version=? AND checksum=? AND state='applying'`, fingerprint, AICostControlV12Version, AICostControlV12Checksum)
	if err != nil {
		return fmt.Errorf("complete v12 schema version ledger: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("verify v12 schema version ledger update: %w", err)
	}
	if rows != 1 {
		return errors.New("v12 schema version ledger did not transition exactly once; it remains incomplete")
	}
	_ = v3
	_ = v4
	_ = v5
	_ = v6
	return nil
}

func verifyManagedSchemaV12(ctx context.Context, conn *sql.Conn, objects schemaObjects, base baselineDefinition, v2 incrementalMigrationDefinition, v3 adminAuthV3Definition, v4 otpSafetyV4Definition, v5 emailOTPSafetyV5Definition, v6 wholesaleRetailV6Definition, v7 platformExpensesV7Definition, v8 platformExpenseReversalsV8Definition, v9 tableOrderingV9Definition) error {
	if err := verifyAICostControlV12Tables(ctx, conn); err != nil {
		return err
	}
	if err := verifyObjectInventoryV12(objects, base, v2, v7, v8, v9); err != nil {
		return err
	}
	combinedNewColumns := append(append([]schemaColumn(nil), orderItemsV11NewColumns...), aiCostControlV12NewColumns...)
	futureTables := append(append(append([]string(nil), tableOrderingV9Order...), "order_items"), aiCostControlV12NewTables...)
	if err := verifyPlatformExpenseReversalsV8HistoricalFingerprintsExcludingColumns(ctx, conn, base, v2, v3, v4, v5, v6, v7, v8, combinedNewColumns, futureTables...); err != nil {
		return err
	}
	rows, err := conn.QueryContext(ctx, "SELECT version, checksum, state, schema_fingerprint FROM "+ledgerTable+" ORDER BY version")
	if err != nil {
		return fmt.Errorf("read v12 schema version ledger: %w", err)
	}
	type ledgerEntry struct {
		version, checksum, state string
		fingerprint              sql.NullString
	}
	entries := make([]ledgerEntry, 0, 12)
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
	if len(entries) != 12 {
		return fmt.Errorf("refusing v12 ledger with %d rows; expected exactly immutable v1-v12 rows", len(entries))
	}
	expected := []struct{ version, checksum string }{
		{BaselineVersion, base.checksum}, {v2.version, v2.checksum}, {v3.version, v3.checksum}, {v4.version, v4.checksum},
		{v5.version, v5.checksum}, {v6.version, v6.checksum}, {v7.version, v7.checksum}, {v8.version, v8.checksum},
		{v9.version, v9.checksum}, {TableLabelsV10Version, TableLabelsV10Checksum}, {OrderItemsV11Version, OrderItemsV11Checksum},
		{AICostControlV12Version, AICostControlV12Checksum},
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
	// v9 check: exclude v12 tables + order_items, and v10/v11/v12 new columns.
	v9ExcludedColumns := append(append([]schemaColumn(nil), combinedNewColumns...), v10NewColumns...)
	v9Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, append(append([]string(nil), "order_items"), aiCostControlV12NewTables...), nil, v9ExcludedColumns)
	if err != nil {
		return fmt.Errorf("verify immutable v9 schema fingerprint: %w", err)
	}
	if stored := byVersion[v9.version].fingerprint.String; stored != v9Fingerprint {
		return fmt.Errorf("refusing v9 schema drift: stored fingerprint %s, actual %s", stored, v9Fingerprint)
	}
	v10ExcludedColumns := combinedNewColumns
	v10Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, append(append([]string(nil), "order_items"), aiCostControlV12NewTables...), nil, v10ExcludedColumns)
	if err != nil {
		return fmt.Errorf("verify immutable v10 schema fingerprint: %w", err)
	}
	if stored := byVersion[TableLabelsV10Version].fingerprint.String; stored != v10Fingerprint {
		return fmt.Errorf("refusing v10 schema drift: stored fingerprint %s, actual %s", stored, v10Fingerprint)
	}
	// v11 check: only the v12 columns are excluded (orders.channel is part
	// of the stored v11 fingerprint). v10/v9 checks below exclude the
	// combined (v11+v12) columns, as in verifyManagedSchemaV11.
	v11Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, aiCostControlV12NewTables, nil, aiCostControlV12NewColumns)
	if err != nil {
		return fmt.Errorf("verify immutable v11 schema fingerprint: %w", err)
	}
	if stored := byVersion[OrderItemsV11Version].fingerprint.String; stored != v11Fingerprint {
		return fmt.Errorf("refusing v11 schema drift: stored fingerprint %s, actual %s", stored, v11Fingerprint)
	}
	actualV12, err := schemaFingerprint(ctx, conn)
	if err != nil {
		return fmt.Errorf("verify v12 managed schema fingerprint: %w", err)
	}
	if stored := byVersion[AICostControlV12Version].fingerprint.String; stored != actualV12 {
		return fmt.Errorf("refusing v12 schema drift: stored fingerprint %s, actual %s", stored, actualV12)
	}
	_ = v6
	return nil
}

// verifyAICostControlV12Tables checks the exact shape of the five new tables,
// the two new merchant_ai_usage_logs columns, and the emergency-switch seed row.
func verifyAICostControlV12Tables(ctx context.Context, conn *sql.Conn) error {
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
			name: "ai_model_prices",
			columns: []columnSpec{
				col("id", "bigint unsigned", "NO", "", false, "auto_increment", ""),
				col("created_at", "datetime(3)", "NO", "", false, "", ""),
				col("updated_at", "datetime(3)", "NO", "", false, "", ""),
				col("model", "varchar(120)", "NO", "", false, "", "utf8mb4_unicode_ci"),
				col("provider", "varchar(80)", "NO", "", true, "", "utf8mb4_unicode_ci"),
				col("input_price_cents_per_million", "bigint", "NO", "", false, "", ""),
				col("output_price_cents_per_million", "bigint", "NO", "", false, "", ""),
				col("cached_price_cents_per_million", "bigint", "NO", "0", true, "", ""),
				col("currency", "char(3)", "NO", "CNY", true, "", "utf8mb4_unicode_ci"),
				col("effective_from", "datetime(3)", "NO", "", false, "", ""),
				col("created_by", "bigint unsigned", "NO", "", false, "", ""),
				col("updated_by", "bigint unsigned", "NO", "0", true, "", ""),
				col("is_active", "tinyint(1)", "NO", "1", true, "", ""),
			},
			indexes: []string{
				"PRIMARY|0|1|id|NULL|BTREE",
				"idx_ai_model_prices_model_effective|1|1|model|NULL|BTREE",
				"idx_ai_model_prices_model_effective|1|2|effective_from|NULL|BTREE",
				"uk_ai_model_prices_model_provider_effective|0|1|model|NULL|BTREE",
				"uk_ai_model_prices_model_provider_effective|0|2|provider|NULL|BTREE",
				"uk_ai_model_prices_model_provider_effective|0|3|effective_from|NULL|BTREE",
			},
			checks: []string{
				"chk_ai_model_prices_cached|ai_model_prices|cached_price_cents_per_million>=0|YES",
				"chk_ai_model_prices_currency|ai_model_prices|currencyin'cny','usd'|YES",
				"chk_ai_model_prices_input|ai_model_prices|input_price_cents_per_million>=0|YES",
				"chk_ai_model_prices_output|ai_model_prices|output_price_cents_per_million>=0|YES",
			},
		},
		{
			name: "ai_vendor_bills",
			columns: []columnSpec{
				col("id", "bigint unsigned", "NO", "", false, "auto_increment", ""),
				col("created_at", "datetime(3)", "NO", "", false, "", ""),
				col("updated_at", "datetime(3)", "NO", "", false, "", ""),
				col("vendor", "varchar(120)", "NO", "", false, "", "utf8mb4_unicode_ci"),
				col("period_start", "date", "NO", "", false, "", ""),
				col("period_end", "date", "NO", "", false, "", ""),
				col("amount_cents", "bigint", "NO", "", false, "", ""),
				col("currency", "char(3)", "NO", "CNY", true, "", "utf8mb4_unicode_ci"),
				col("note", "varchar(500)", "NO", "", true, "", "utf8mb4_unicode_ci"),
				col("created_by", "bigint unsigned", "NO", "", false, "", ""),
				col("idempotency_key", "char(36)", "NO", "", false, "", "utf8mb4_unicode_ci"),
			},
			indexes: []string{
				"PRIMARY|0|1|id|NULL|BTREE",
				"idx_ai_vendor_bills_period_start|1|1|period_start|NULL|BTREE",
				"idx_ai_vendor_bills_vendor_period|1|1|vendor|NULL|BTREE",
				"idx_ai_vendor_bills_vendor_period|1|2|period_start|NULL|BTREE",
				"uk_ai_vendor_bills_idempotency_key|0|1|idempotency_key|NULL|BTREE",
			},
			checks: []string{
				"chk_ai_vendor_bills_amount|ai_vendor_bills|amount_cents>0|YES",
				"chk_ai_vendor_bills_currency|ai_vendor_bills|currencyin'cny','usd'|YES",
				"chk_ai_vendor_bills_period|ai_vendor_bills|period_end>=period_start|YES",
			},
		},
		{
			name: "merchant_ai_budgets",
			columns: []columnSpec{
				col("id", "bigint unsigned", "NO", "", false, "auto_increment", ""),
				col("created_at", "datetime(3)", "NO", "", false, "", ""),
				col("updated_at", "datetime(3)", "NO", "", false, "", ""),
				col("merchant_id", "bigint unsigned", "NO", "", false, "", ""),
				col("daily_budget_cents", "bigint", "NO", "0", true, "", ""),
				col("monthly_budget_cents", "bigint", "NO", "0", true, "", ""),
				col("over_limit_action", "varchar(24)", "NO", "stop", true, "", "utf8mb4_unicode_ci"),
				col("updated_by", "bigint unsigned", "NO", "", false, "", ""),
			},
			indexes: []string{
				"PRIMARY|0|1|id|NULL|BTREE",
				"uk_merchant_ai_budgets_merchant|0|1|merchant_id|NULL|BTREE",
			},
			checks: []string{
				"chk_merchant_ai_budgets_action|merchant_ai_budgets|over_limit_actionin'stop','downgrade','restrict_expensive','manual_review'|YES",
				"chk_merchant_ai_budgets_daily|merchant_ai_budgets|daily_budget_cents>=0|YES",
				"chk_merchant_ai_budgets_monthly|merchant_ai_budgets|monthly_budget_cents>=0|YES",
			},
		},
		{
			name: "merchant_ai_budget_alerts",
			columns: []columnSpec{
				col("id", "bigint unsigned", "NO", "", false, "auto_increment", ""),
				col("created_at", "datetime(3)", "NO", "", false, "", ""),
				col("updated_at", "datetime(3)", "NO", "", false, "", ""),
				col("merchant_id", "bigint unsigned", "NO", "", false, "", ""),
				col("period_type", "varchar(16)", "NO", "", false, "", "utf8mb4_unicode_ci"),
				col("period_start", "date", "NO", "", false, "", ""),
				col("threshold", "int", "NO", "", false, "", ""),
				col("used_tokens", "bigint", "NO", "0", true, "", ""),
				col("estimated_cost_cents", "bigint", "NO", "0", true, "", ""),
			},
			indexes: []string{
				"PRIMARY|0|1|id|NULL|BTREE",
				"idx_merchant_ai_budget_alerts_merchant|1|1|merchant_id|NULL|BTREE",
				"idx_merchant_ai_budget_alerts_period|1|1|period_start|NULL|BTREE",
				"uk_merchant_ai_budget_alerts_scope|0|1|merchant_id|NULL|BTREE",
				"uk_merchant_ai_budget_alerts_scope|0|2|period_type|NULL|BTREE",
				"uk_merchant_ai_budget_alerts_scope|0|3|period_start|NULL|BTREE",
				"uk_merchant_ai_budget_alerts_scope|0|4|threshold|NULL|BTREE",
			},
			checks: []string{
				"chk_merchant_ai_budget_alerts_cost|merchant_ai_budget_alerts|estimated_cost_cents>=0|YES",
				"chk_merchant_ai_budget_alerts_period|merchant_ai_budget_alerts|period_typein'daily','monthly'|YES",
				"chk_merchant_ai_budget_alerts_threshold|merchant_ai_budget_alerts|thresholdin70,90,100|YES",
				"chk_merchant_ai_budget_alerts_used_tokens|merchant_ai_budget_alerts|used_tokens>=0|YES",
			},
		},
		{
			name: "ai_emergency_switch",
			columns: []columnSpec{
				col("id", "bigint unsigned", "NO", "", false, "", ""),
				col("created_at", "datetime(3)", "NO", "", false, "", ""),
				col("updated_at", "datetime(3)", "NO", "", false, "", ""),
				col("disabled", "tinyint(1)", "NO", "0", true, "", ""),
				col("reason", "varchar(500)", "NO", "", true, "", "utf8mb4_unicode_ci"),
				col("updated_by", "bigint unsigned", "NO", "0", true, "", ""),
			},
			indexes: []string{
				"PRIMARY|0|1|id|NULL|BTREE",
			},
			checks: []string{},
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

	// New merchant_ai_usage_logs columns.
	var storeIDType, storeIDNullable, storeIDDefault string
	var storeIDHasDefault bool
	var storeIDDefaultValue sql.NullString
	if err := conn.QueryRowContext(ctx, `SELECT COLUMN_TYPE, IS_NULLABLE, COLUMN_DEFAULT FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='merchant_ai_usage_logs' AND COLUMN_NAME='store_id'`).Scan(&storeIDType, &storeIDNullable, &storeIDDefaultValue); err != nil {
		return fmt.Errorf("inspect merchant_ai_usage_logs.store_id column: %w", err)
	}
	storeIDHasDefault = storeIDDefaultValue.Valid
	if storeIDDefaultValue.Valid {
		storeIDDefault = storeIDDefaultValue.String
	}
	if storeIDType != "bigint unsigned" || storeIDNullable != "YES" || storeIDHasDefault || storeIDDefault != "" {
		return fmt.Errorf("merchant_ai_usage_logs.store_id column has unexpected shape: type=%s nullable=%s hasDefault=%v default=%q", storeIDType, storeIDNullable, storeIDHasDefault, storeIDDefault)
	}
	var cachedType, cachedNullable, cachedDefault string
	if err := conn.QueryRowContext(ctx, `SELECT COLUMN_TYPE, IS_NULLABLE, COALESCE(CAST(COLUMN_DEFAULT AS CHAR), '<NULL>') FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='merchant_ai_usage_logs' AND COLUMN_NAME='cached_tokens'`).Scan(&cachedType, &cachedNullable, &cachedDefault); err != nil {
		return fmt.Errorf("inspect merchant_ai_usage_logs.cached_tokens column: %w", err)
	}
	if cachedType != "bigint" || cachedNullable != "NO" || cachedDefault != "0" {
		return fmt.Errorf("merchant_ai_usage_logs.cached_tokens column has unexpected shape: type=%s nullable=%s default=%q", cachedType, cachedNullable, cachedDefault)
	}

	// Emergency-switch seed row (id=1, AI enabled).
	var seedRows int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM `ai_emergency_switch` WHERE `id` = 1 AND `disabled` = 0").Scan(&seedRows); err != nil {
		return fmt.Errorf("inspect ai_emergency_switch seed row: %w", err)
	}
	if seedRows != 1 {
		return fmt.Errorf("ai_emergency_switch seed row is missing: found %d enabled id=1 rows, expected 1", seedRows)
	}
	return nil
}

func verifyObjectInventoryV12(objects schemaObjects, base baselineDefinition, v2 incrementalMigrationDefinition, v7 platformExpensesV7Definition, v8 platformExpenseReversalsV8Definition, v9 tableOrderingV9Definition) error {
	filtered := objects
	for _, name := range aiCostControlV12NewTables {
		if !containsSchemaName(objects.tables, name) {
			return fmt.Errorf("v12 table %q is missing", name)
		}
		filtered.tables = removeSchemaName(filtered.tables, name)
	}
	if err := verifyObjectInventoryV11(filtered, base, v2, v7, v8, v9); err != nil {
		return err
	}
	return nil
}
