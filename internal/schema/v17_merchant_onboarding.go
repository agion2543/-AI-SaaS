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
	MerchantOnboardingV17Version  = "schema-v17-merchant-onboarding-20261128"
	MerchantOnboardingV17Checksum = "9df04dcd4f04299595d57d64a610f4b0846d70cbd6a802fb5cb0dba52a752960"
)

// merchantOnboardingV17NewColumns lists the columns v17 adds to the
// pre-existing merchants table, so historical fingerprints can exclude them.
var merchantOnboardingV17NewColumns = []schemaColumn{
	{table: "merchants", column: "review_reason"},
	{table: "merchants", column: "reviewed_by"},
	{table: "merchants", column: "reviewed_at"},
}

// merchantOnboardingV17Statements is the exact DDL of the v17 migration, in
// apply order. No new tables, no new indexes, no CHECK constraints: the
// review value ranges are enforced in the service layer.
//
// History is not rewritten: pre-v17 merchants keep review_reason "" with
// reviewer 0 and a NULL review time (they were created before onboarding
// review existed; existing rows are active/suspended, so no data migration
// is needed and old merchants are unaffected). A rejected merchant stays
// status=pending with a non-empty review_reason and a non-NULL reviewed_at;
// resubmission clears all three columns back to a clean pending state.
var merchantOnboardingV17Statements = []string{
	"ALTER TABLE `merchants` ADD COLUMN `review_reason` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT ''",
	"ALTER TABLE `merchants` ADD COLUMN `reviewed_by` bigint unsigned NOT NULL DEFAULT 0",
	"ALTER TABLE `merchants` ADD COLUMN `reviewed_at` datetime(3) NULL",
}

func validateMerchantOnboardingV17Definition() error {
	material := append([]string{MerchantOnboardingV17Version}, merchantOnboardingV17Statements...)
	sum := sha256.Sum256([]byte(strings.Join(material, "\n")))
	if got := fmt.Sprintf("%x", sum); got != MerchantOnboardingV17Checksum {
		return fmt.Errorf("merchant onboarding v17 checksum mismatch: definition %s, expected %s", got, MerchantOnboardingV17Checksum)
	}
	return nil
}

// ApplyMerchantOnboardingV17 appends the onboarding-review columns to
// merchants on an exact managed v16 schema. The v1-v16 schema and
// fingerprints remain untouched.
func ApplyMerchantOnboardingV17(ctx context.Context, db *sql.DB) (Result, error) {
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
	if err := validateMerchantOnboardingV17Definition(); err != nil {
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
	if err := verifyManagedSchemaV16(ctx, conn, objects, base, v2, v3, v4, v5, v6, v7, v8, v9); err == nil {
		if err := applyMerchantOnboardingV17(ctx, conn, base, v2, v3, v4, v5, v6, v7, v8, v9); err != nil {
			return "", err
		}
		return ResultApplied, nil
	}
	if err := verifyManagedSchemaV17(ctx, conn, objects, base, v2, v3, v4, v5, v6, v7, v8, v9); err != nil {
		return "", fmt.Errorf("refusing non-exact managed schema for v17 merchant onboarding migration: %w", err)
	}
	return ResultAlready, nil
}

func applyMerchantOnboardingV17(ctx context.Context, conn *sql.Conn, base baselineDefinition, v2 incrementalMigrationDefinition, v3 adminAuthV3Definition, v4 otpSafetyV4Definition, v5 emailOTPSafetyV5Definition, v6 wholesaleRetailV6Definition, v7 platformExpensesV7Definition, v8 platformExpenseReversalsV8Definition, v9 tableOrderingV9Definition) error {
	if _, err := conn.ExecContext(ctx, `INSERT INTO `+ledgerTable+` (version, checksum, state) VALUES (?, ?, 'applying')`, MerchantOnboardingV17Version, MerchantOnboardingV17Checksum); err != nil {
		return fmt.Errorf("record v17 merchant onboarding migration start: %w", err)
	}
	for i, statement := range merchantOnboardingV17Statements {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply v17 merchant onboarding change %d (v17 ledger remains incomplete): %w", i+1, err)
		}
	}
	if err := verifyMerchantOnboardingV17Tables(ctx, conn); err != nil {
		return fmt.Errorf("verify v17 merchant onboarding tables (v17 ledger remains incomplete): %w", err)
	}
	objects, err := inspectObjects(ctx, conn)
	if err != nil {
		return err
	}
	if err := verifyObjectInventoryV17(objects, base, v2, v7, v8, v9); err != nil {
		return fmt.Errorf("post-apply v17 inventory verification failed (v17 ledger remains incomplete): %w", err)
	}
	// The stored v16 fingerprint is computed over the schema without the
	// three v17 columns.
	v16Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, nil, nil, merchantOnboardingV17NewColumns)
	if err != nil {
		return fmt.Errorf("compute immutable v16 schema fingerprint during v17 migration: %w", err)
	}
	var storedV16 string
	if err := conn.QueryRowContext(ctx, "SELECT schema_fingerprint FROM "+ledgerTable+" WHERE version=? AND checksum=? AND state='applied'", CustomerProfilesV16Version, CustomerProfilesV16Checksum).Scan(&storedV16); err != nil {
		return fmt.Errorf("read immutable v16 schema fingerprint during v17 migration: %w", err)
	}
	if storedV16 != v16Fingerprint {
		return fmt.Errorf("refusing v16 schema drift during v17 migration: stored %s, actual %s", storedV16, v16Fingerprint)
	}
	fingerprint, err := schemaFingerprint(ctx, conn)
	if err != nil {
		return fmt.Errorf("compute v17 installed schema fingerprint: %w", err)
	}
	result, err := conn.ExecContext(ctx, `UPDATE `+ledgerTable+` SET state='applied', schema_fingerprint=?, applied_at=CURRENT_TIMESTAMP WHERE version=? AND checksum=? AND state='applying'`, fingerprint, MerchantOnboardingV17Version, MerchantOnboardingV17Checksum)
	if err != nil {
		return fmt.Errorf("complete v17 schema version ledger: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("verify v17 schema version ledger update: %w", err)
	}
	if rows != 1 {
		return errors.New("v17 schema version ledger did not transition exactly once; it remains incomplete")
	}
	_ = v3
	_ = v4
	_ = v5
	_ = v6
	return nil
}

func verifyManagedSchemaV17(ctx context.Context, conn *sql.Conn, objects schemaObjects, base baselineDefinition, v2 incrementalMigrationDefinition, v3 adminAuthV3Definition, v4 otpSafetyV4Definition, v5 emailOTPSafetyV5Definition, v6 wholesaleRetailV6Definition, v7 platformExpensesV7Definition, v8 platformExpenseReversalsV8Definition, v9 tableOrderingV9Definition) error {
	// Historical fingerprints first: the v8 helper re-pins v1-v8, and
	// verifyMerchantOnboardingV17Tables re-pins the altered merchants
	// shape, so a drift in an old table trips the check that names the
	// drifted version (e.g. "v7 schema drift"), which ApplyLatest prefers
	// when refusing.
	combinedNewColumns := append(append(append(append(append([]schemaColumn(nil), orderItemsV11NewColumns...), aiCostControlV12NewColumns...), platformFinanceV14NewColumns...), refundReviewV15NewColumns...), merchantOnboardingV17NewColumns...)
	futureTables := append(append(append(append(append(append([]string(nil), tableOrderingV9Order...), "order_items"), aiCostControlV12NewTables...), announcementsMaintenanceV13NewTables...), platformFinanceV14NewTables...), customerProfilesV16NewTables...)
	if err := verifyPlatformExpenseReversalsV8HistoricalFingerprintsExcludingColumns(ctx, conn, base, v2, v3, v4, v5, v6, v7, v8, combinedNewColumns, futureTables...); err != nil {
		return err
	}
	if err := verifyMerchantOnboardingV17Tables(ctx, conn); err != nil {
		return err
	}
	if err := verifyObjectInventoryV17(objects, base, v2, v7, v8, v9); err != nil {
		return err
	}
	rows, err := conn.QueryContext(ctx, "SELECT version, checksum, state, schema_fingerprint FROM "+ledgerTable+" ORDER BY version")
	if err != nil {
		return fmt.Errorf("read v17 schema version ledger: %w", err)
	}
	type ledgerEntry struct {
		version, checksum, state string
		fingerprint              sql.NullString
	}
	entries := make([]ledgerEntry, 0, 17)
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
	if len(entries) != 17 {
		return fmt.Errorf("refusing v17 ledger with %d rows; expected exactly immutable v1-v17 rows", len(entries))
	}
	expected := []struct{ version, checksum string }{
		{BaselineVersion, base.checksum}, {v2.version, v2.checksum}, {v3.version, v3.checksum}, {v4.version, v4.checksum},
		{v5.version, v5.checksum}, {v6.version, v6.checksum}, {v7.version, v7.checksum}, {v8.version, v8.checksum},
		{v9.version, v9.checksum}, {TableLabelsV10Version, TableLabelsV10Checksum}, {OrderItemsV11Version, OrderItemsV11Checksum},
		{AICostControlV12Version, AICostControlV12Checksum},
		{AnnouncementsMaintenanceV13Version, AnnouncementsMaintenanceV13Checksum},
		{PlatformFinanceV14Version, PlatformFinanceV14Checksum},
		{RefundReviewV15Version, RefundReviewV15Checksum},
		{CustomerProfilesV16Version, CustomerProfilesV16Checksum},
		{MerchantOnboardingV17Version, MerchantOnboardingV17Checksum},
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
	// v9 check: exclude v12/v13/v14/v16 tables + order_items, and
	// v10/v11/v12/v14/v15/v17 new columns.
	v9ExcludedColumns := append(append([]schemaColumn(nil), combinedNewColumns...), v10NewColumns...)
	v9ExcludedTables := append(append(append(append(append([]string(nil), "order_items"), aiCostControlV12NewTables...), announcementsMaintenanceV13NewTables...), platformFinanceV14NewTables...), customerProfilesV16NewTables...)
	v9Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, v9ExcludedTables, nil, v9ExcludedColumns)
	if err != nil {
		return fmt.Errorf("verify immutable v9 schema fingerprint: %w", err)
	}
	if stored := byVersion[v9.version].fingerprint.String; stored != v9Fingerprint {
		return fmt.Errorf("refusing v9 schema drift: stored fingerprint %s, actual %s", stored, v9Fingerprint)
	}
	// v10 check: same tables, v11/v12/v14/v15/v17 new columns.
	v10Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, v9ExcludedTables, nil, combinedNewColumns)
	if err != nil {
		return fmt.Errorf("verify immutable v10 schema fingerprint: %w", err)
	}
	if stored := byVersion[TableLabelsV10Version].fingerprint.String; stored != v10Fingerprint {
		return fmt.Errorf("refusing v10 schema drift: stored fingerprint %s, actual %s", stored, v10Fingerprint)
	}
	// v11 check: exclude the v12/v13/v14/v16 tables, and the v12/v14/v15/v17
	// columns (orders.channel is part of the stored v11 fingerprint; v13
	// added no columns to pre-existing tables).
	v11ExcludedColumns := append(append(append(append([]schemaColumn(nil), aiCostControlV12NewColumns...), platformFinanceV14NewColumns...), refundReviewV15NewColumns...), merchantOnboardingV17NewColumns...)
	v11ExcludedTables := append(append(append(append([]string(nil), aiCostControlV12NewTables...), announcementsMaintenanceV13NewTables...), platformFinanceV14NewTables...), customerProfilesV16NewTables...)
	v11Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, v11ExcludedTables, nil, v11ExcludedColumns)
	if err != nil {
		return fmt.Errorf("verify immutable v11 schema fingerprint: %w", err)
	}
	if stored := byVersion[OrderItemsV11Version].fingerprint.String; stored != v11Fingerprint {
		return fmt.Errorf("refusing v11 schema drift: stored fingerprint %s, actual %s", stored, v11Fingerprint)
	}
	// v12 check: exclude the v13/v14/v16 tables and the v14/v15/v17 columns
	// (the v12 columns are part of the stored v12 fingerprint).
	v12ExcludedColumns := append(append(append([]schemaColumn(nil), platformFinanceV14NewColumns...), refundReviewV15NewColumns...), merchantOnboardingV17NewColumns...)
	v12Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, append(append(append([]string(nil), announcementsMaintenanceV13NewTables...), platformFinanceV14NewTables...), customerProfilesV16NewTables...), nil, v12ExcludedColumns)
	if err != nil {
		return fmt.Errorf("verify immutable v12 schema fingerprint: %w", err)
	}
	if stored := byVersion[AICostControlV12Version].fingerprint.String; stored != v12Fingerprint {
		return fmt.Errorf("refusing v12 schema drift: stored fingerprint %s, actual %s", stored, v12Fingerprint)
	}
	// v13 check: exclude only the v14/v16 tables and the v14/v15/v17 columns
	// (v13 added no columns to pre-existing tables).
	v13ExcludedColumns := append(append(append([]schemaColumn(nil), platformFinanceV14NewColumns...), refundReviewV15NewColumns...), merchantOnboardingV17NewColumns...)
	v13Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, append(append([]string(nil), platformFinanceV14NewTables...), customerProfilesV16NewTables...), nil, v13ExcludedColumns)
	if err != nil {
		return fmt.Errorf("verify immutable v13 schema fingerprint: %w", err)
	}
	if stored := byVersion[AnnouncementsMaintenanceV13Version].fingerprint.String; stored != v13Fingerprint {
		return fmt.Errorf("refusing v13 schema drift: stored fingerprint %s, actual %s", stored, v13Fingerprint)
	}
	// v14 check: exclude only the v16 table and the v15/v17 columns (v15
	// adds no tables).
	v14Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, customerProfilesV16NewTables, nil, append(append([]schemaColumn(nil), refundReviewV15NewColumns...), merchantOnboardingV17NewColumns...))
	if err != nil {
		return fmt.Errorf("verify immutable v14 schema fingerprint: %w", err)
	}
	if stored := byVersion[PlatformFinanceV14Version].fingerprint.String; stored != v14Fingerprint {
		return fmt.Errorf("refusing v14 schema drift: stored fingerprint %s, actual %s", stored, v14Fingerprint)
	}
	// v15 check: exclude only the v16 table and the v17 columns (v17 adds
	// no tables).
	v15Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, customerProfilesV16NewTables, nil, merchantOnboardingV17NewColumns)
	if err != nil {
		return fmt.Errorf("verify immutable v15 schema fingerprint: %w", err)
	}
	if stored := byVersion[RefundReviewV15Version].fingerprint.String; stored != v15Fingerprint {
		return fmt.Errorf("refusing v15 schema drift: stored fingerprint %s, actual %s", stored, v15Fingerprint)
	}
	// v16 check: exclude only the v17 columns (v17 adds no tables).
	v16Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, nil, nil, merchantOnboardingV17NewColumns)
	if err != nil {
		return fmt.Errorf("verify immutable v16 schema fingerprint: %w", err)
	}
	if stored := byVersion[CustomerProfilesV16Version].fingerprint.String; stored != v16Fingerprint {
		return fmt.Errorf("refusing v16 schema drift: stored fingerprint %s, actual %s", stored, v16Fingerprint)
	}
	actualV17, err := schemaFingerprint(ctx, conn)
	if err != nil {
		return fmt.Errorf("verify v17 managed schema fingerprint: %w", err)
	}
	if stored := byVersion[MerchantOnboardingV17Version].fingerprint.String; stored != actualV17 {
		return fmt.Errorf("refusing v17 schema drift: stored fingerprint %s, actual %s", stored, actualV17)
	}
	_ = v6
	return nil
}

// verifyMerchantOnboardingV17Tables checks the exact shape of the altered
// merchants table (18 columns: the 15 baseline columns plus the three v17
// review columns appended at the end). The baseline columns keep their v1
// nullability and defaults; the three new columns are review_reason "" (NOT
// NULL), reviewed_by 0 (NOT NULL) and reviewed_at NULL.
func verifyMerchantOnboardingV17Tables(ctx context.Context, conn *sql.Conn) error {
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
			name: "merchants",
			columns: []columnSpec{
				col("id", "bigint unsigned", "NO", "", false, "auto_increment", ""),
				col("created_at", "datetime(3)", "YES", "", false, "", ""),
				col("updated_at", "datetime(3)", "YES", "", false, "", ""),
				col("name", "varchar(120)", "YES", "", false, "", "utf8mb4_unicode_ci"),
				col("contact_phone", "varchar(30)", "YES", "", false, "", "utf8mb4_unicode_ci"),
				col("contact_email", "varchar(120)", "YES", "", false, "", "utf8mb4_unicode_ci"),
				col("status", "varchar(20)", "YES", "pending", true, "", "utf8mb4_unicode_ci"),
				col("subscription_plan", "varchar(40)", "YES", "none", true, "", "utf8mb4_unicode_ci"),
				col("subscription_status", "varchar(20)", "YES", "inactive", true, "", "utf8mb4_unicode_ci"),
				col("subscription_expired_at", "datetime(3)", "YES", "", false, "", ""),
				col("subscription_note", "varchar(255)", "YES", "", false, "", "utf8mb4_unicode_ci"),
				col("subscription_plan_id", "bigint unsigned", "YES", "", false, "", ""),
				col("subscription_expire_at", "datetime(3)", "YES", "", false, "", ""),
				col("subscription_pause_reason", "varchar(255)", "YES", "", false, "", "utf8mb4_unicode_ci"),
				col("subscription_paused_at", "datetime(3)", "YES", "", false, "", ""),
				col("review_reason", "varchar(255)", "NO", "", true, "", "utf8mb4_unicode_ci"),
				col("reviewed_by", "bigint unsigned", "NO", "0", true, "", ""),
				col("reviewed_at", "datetime(3)", "YES", "", false, "", ""),
			},
			indexes: []string{
				"PRIMARY|0|1|id|NULL|BTREE",
				"idx_merchants_name|0|1|name|NULL|BTREE",
				"idx_merchants_subscription_plan_id|1|1|subscription_plan_id|NULL|BTREE",
				"idx_merchants_subscription_status|1|1|subscription_status|NULL|BTREE",
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

func verifyObjectInventoryV17(objects schemaObjects, base baselineDefinition, v2 incrementalMigrationDefinition, v7 platformExpensesV7Definition, v8 platformExpenseReversalsV8Definition, v9 tableOrderingV9Definition) error {
	// v17 adds columns only, no tables, triggers, views, routines or
	// events: the inventory is exactly the v16 inventory.
	return verifyObjectInventoryV16(objects, base, v2, v7, v8, v9)
}
