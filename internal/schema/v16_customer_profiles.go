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
	CustomerProfilesV16Version  = "schema-v16-customer-profiles-20261128"
	CustomerProfilesV16Checksum = "de9c0c0967b94abfd750cd15e681227ac60728486ab1611a305d0387ef116c96"
)

// customerProfilesV16NewTables lists the tables v16 creates.
var customerProfilesV16NewTables = []string{
	"customer_profiles",
}

// customerProfilesV16Statements is the exact DDL of the v16 migration: one
// new materialized table, nothing else. It stores facts only (no derived
// AITag, no refund amounts): the read-time insight rules are unchanged.
//
// last_order_at keeps the old MAX(created_at) definition (order time, not
// payment time); last_paid_at is stored alongside it so the dormant tiers,
// which used MAX(paid_at), read the same row. last_store_name is the store
// of the latest order (last_order_id), not MAX(stores.name): the old MAX()
// was non-deterministic when a customer ordered at several stores of one
// merchant, so this is a deliberate small behaviour change, documented in
// the v16 section of docs/database-migration-status.md.
//
// The three CHECK constraints only guard the non-negative counters; the
// value ranges of last_status and the store name are enforced in the
// service layer, and orders.customer_phone keeps its v1 shape (no new
// index there: v1 fingerprints are pinned and orders has no usable phone
// index by design).
var customerProfilesV16Statements = []string{
	`CREATE TABLE ` + "`customer_profiles`" + ` (
				` + "`id`" + ` bigint unsigned NOT NULL AUTO_INCREMENT,
				` + "`created_at`" + ` datetime(3) NOT NULL,
				` + "`updated_at`" + ` datetime(3) NOT NULL,
				` + "`merchant_id`" + ` bigint unsigned NOT NULL,
				` + "`customer_phone`" + ` varchar(30) COLLATE utf8mb4_unicode_ci NOT NULL,
				` + "`last_store_id`" + ` bigint unsigned NULL,
				` + "`last_store_name`" + ` varchar(120) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
				` + "`order_count`" + ` bigint NOT NULL DEFAULT 0,
				` + "`total_amount_cents`" + ` bigint NOT NULL DEFAULT 0,
				` + "`discount_amount_cents`" + ` bigint NOT NULL DEFAULT 0,
				` + "`first_paid_at`" + ` datetime(3) NULL,
				` + "`last_paid_at`" + ` datetime(3) NULL,
				` + "`last_order_at`" + ` datetime(3) NULL,
				` + "`last_order_id`" + ` bigint unsigned NOT NULL DEFAULT 0,
				` + "`last_status`" + ` varchar(20) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
				PRIMARY KEY (` + "`id`" + `),
				UNIQUE KEY ` + "`uk_customer_profiles_merchant_phone`" + ` (` + "`merchant_id`" + `, ` + "`customer_phone`" + `),
				KEY ` + "`idx_customer_profiles_merchant_last_order`" + ` (` + "`merchant_id`" + `, ` + "`last_order_at`" + `),
				KEY ` + "`idx_customer_profiles_last_paid_at`" + ` (` + "`last_paid_at`" + `),
				CONSTRAINT ` + "`chk_customer_profiles_order_count`" + ` CHECK (` + "`order_count`" + ` >= 0),
				CONSTRAINT ` + "`chk_customer_profiles_total_amount`" + ` CHECK (` + "`total_amount_cents`" + ` >= 0),
				CONSTRAINT ` + "`chk_customer_profiles_discount_amount`" + ` CHECK (` + "`discount_amount_cents`" + ` >= 0)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
}

// customerProfilesV16Backfill rebuilds the table from historical orders with
// the exact old GROUP BY definition (order_type='store_order',
// customer_phone<>'', paid_at IS NOT NULL AND total_amount > 0), so a
// pre-v16 database gets the same profiles the live aggregation used to
// compute. last_status uses the same GROUP_CONCAT trick as the old read;
// last_store_id / last_store_name come from the latest order by
// (created_at, id), which is the one deliberate change from MAX(stores.name).
const customerProfilesV16Backfill = "INSERT INTO `customer_profiles` (`merchant_id`, `customer_phone`, `last_store_id`, `last_store_name`, `order_count`, `total_amount_cents`, `discount_amount_cents`, `first_paid_at`, `last_paid_at`, `last_order_at`, `last_order_id`, `last_status`, `created_at`, `updated_at`) " +
	"SELECT o.`merchant_id`, o.`customer_phone`, " +
	"(SELECT o2.`store_id` FROM `orders` o2 WHERE o2.`merchant_id` = o.`merchant_id` AND o2.`customer_phone` = o.`customer_phone` AND o2.`order_type` = 'store_order' AND o2.`paid_at` IS NOT NULL AND o2.`total_amount` > 0 ORDER BY o2.`created_at` DESC, o2.`id` DESC LIMIT 1), " +
	"COALESCE((SELECT s.`name` FROM `orders` o2 LEFT JOIN `stores` s ON s.`id` = o2.`store_id` WHERE o2.`merchant_id` = o.`merchant_id` AND o2.`customer_phone` = o.`customer_phone` AND o2.`order_type` = 'store_order' AND o2.`paid_at` IS NOT NULL AND o2.`total_amount` > 0 ORDER BY o2.`created_at` DESC, o2.`id` DESC LIMIT 1), ''), " +
	"COUNT(o.`id`), COALESCE(SUM(o.`total_amount`), 0), COALESCE(SUM(o.`discount_amount`), 0), " +
	"MIN(o.`paid_at`), MAX(o.`paid_at`), MAX(o.`created_at`), " +
	"(SELECT o2.`id` FROM `orders` o2 WHERE o2.`merchant_id` = o.`merchant_id` AND o2.`customer_phone` = o.`customer_phone` AND o2.`order_type` = 'store_order' AND o2.`paid_at` IS NOT NULL AND o2.`total_amount` > 0 ORDER BY o2.`created_at` DESC, o2.`id` DESC LIMIT 1), " +
	"SUBSTRING_INDEX(GROUP_CONCAT(o.`status` ORDER BY o.`created_at` DESC, o.`id` DESC), ',', 1), " +
	"NOW(3), NOW(3) " +
	"FROM `orders` o " +
	"WHERE o.`order_type` = 'store_order' AND o.`customer_phone` <> '' AND o.`merchant_id` IS NOT NULL AND o.`paid_at` IS NOT NULL AND o.`total_amount` > 0 " +
	"GROUP BY o.`merchant_id`, o.`customer_phone`"

func validateCustomerProfilesV16Definition() error {
	material := append([]string{CustomerProfilesV16Version}, customerProfilesV16Statements...)
	material = append(material, customerProfilesV16Backfill)
	sum := sha256.Sum256([]byte(strings.Join(material, "\n")))
	if got := fmt.Sprintf("%x", sum); got != CustomerProfilesV16Checksum {
		return fmt.Errorf("customer profiles v16 checksum mismatch: definition %s, expected %s", got, CustomerProfilesV16Checksum)
	}
	return nil
}

// ApplyCustomerProfilesV16 creates the customer_profiles materialized table
// and backfills it from historical valid store orders on an exact managed
// v15 schema. The v1-v15 schema and fingerprints remain untouched.
func ApplyCustomerProfilesV16(ctx context.Context, db *sql.DB) (Result, error) {
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
	if err := validateCustomerProfilesV16Definition(); err != nil {
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
	if err := verifyManagedSchemaV15(ctx, conn, objects, base, v2, v3, v4, v5, v6, v7, v8, v9); err == nil {
		if err := applyCustomerProfilesV16(ctx, conn, base, v2, v3, v4, v5, v6, v7, v8, v9); err != nil {
			return "", err
		}
		return ResultApplied, nil
	}
	if err := verifyManagedSchemaV16(ctx, conn, objects, base, v2, v3, v4, v5, v6, v7, v8, v9); err != nil {
		return "", fmt.Errorf("refusing non-exact managed schema for v16 customer profiles migration: %w", err)
	}
	return ResultAlready, nil
}

func applyCustomerProfilesV16(ctx context.Context, conn *sql.Conn, base baselineDefinition, v2 incrementalMigrationDefinition, v3 adminAuthV3Definition, v4 otpSafetyV4Definition, v5 emailOTPSafetyV5Definition, v6 wholesaleRetailV6Definition, v7 platformExpensesV7Definition, v8 platformExpenseReversalsV8Definition, v9 tableOrderingV9Definition) error {
	if _, err := conn.ExecContext(ctx, `INSERT INTO `+ledgerTable+` (version, checksum, state) VALUES (?, ?, 'applying')`, CustomerProfilesV16Version, CustomerProfilesV16Checksum); err != nil {
		return fmt.Errorf("record v16 customer profiles migration start: %w", err)
	}
	for i, statement := range customerProfilesV16Statements {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply v16 customer profiles change %d (v16 ledger remains incomplete): %w", i+1, err)
		}
	}
	if _, err := conn.ExecContext(ctx, customerProfilesV16Backfill); err != nil {
		return fmt.Errorf("apply v16 customer profiles backfill (v16 ledger remains incomplete): %w", err)
	}
	if err := verifyCustomerProfilesV16Tables(ctx, conn); err != nil {
		return fmt.Errorf("verify v16 customer profiles tables (v16 ledger remains incomplete): %w", err)
	}
	objects, err := inspectObjects(ctx, conn)
	if err != nil {
		return err
	}
	if err := verifyObjectInventoryV16(objects, base, v2, v7, v8, v9); err != nil {
		return fmt.Errorf("post-apply v16 inventory verification failed (v16 ledger remains incomplete): %w", err)
	}
	// The stored v15 fingerprint is computed over the schema without the
	// v16 table; the backfill only inserts rows, so it cannot change it.
	v15Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, customerProfilesV16NewTables, nil, nil)
	if err != nil {
		return fmt.Errorf("compute immutable v15 schema fingerprint during v16 migration: %w", err)
	}
	var storedV15 string
	if err := conn.QueryRowContext(ctx, "SELECT schema_fingerprint FROM "+ledgerTable+" WHERE version=? AND checksum=? AND state='applied'", RefundReviewV15Version, RefundReviewV15Checksum).Scan(&storedV15); err != nil {
		return fmt.Errorf("read immutable v15 schema fingerprint during v16 migration: %w", err)
	}
	if storedV15 != v15Fingerprint {
		return fmt.Errorf("refusing v15 schema drift during v16 migration: stored %s, actual %s", storedV15, v15Fingerprint)
	}
	fingerprint, err := schemaFingerprint(ctx, conn)
	if err != nil {
		return fmt.Errorf("compute v16 installed schema fingerprint: %w", err)
	}
	result, err := conn.ExecContext(ctx, `UPDATE `+ledgerTable+` SET state='applied', schema_fingerprint=?, applied_at=CURRENT_TIMESTAMP WHERE version=? AND checksum=? AND state='applying'`, fingerprint, CustomerProfilesV16Version, CustomerProfilesV16Checksum)
	if err != nil {
		return fmt.Errorf("complete v16 schema version ledger: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("verify v16 schema version ledger update: %w", err)
	}
	if rows != 1 {
		return errors.New("v16 schema version ledger did not transition exactly once; it remains incomplete")
	}
	_ = v3
	_ = v4
	_ = v5
	_ = v6
	return nil
}

func verifyManagedSchemaV16(ctx context.Context, conn *sql.Conn, objects schemaObjects, base baselineDefinition, v2 incrementalMigrationDefinition, v3 adminAuthV3Definition, v4 otpSafetyV4Definition, v5 emailOTPSafetyV5Definition, v6 wholesaleRetailV6Definition, v7 platformExpensesV7Definition, v8 platformExpenseReversalsV8Definition, v9 tableOrderingV9Definition) error {
	// Historical fingerprints first: the v8 helper re-pins v1-v8, and
	// verifyCustomerProfilesV16Tables re-pins the new table shape, so a
	// drift in an old table trips the check that names the drifted version
	// (e.g. "v7 schema drift"), which ApplyLatest prefers when refusing.
	// v16 adds no columns to pre-existing tables, so the combined new
	// columns are exactly the v15 set; the new table is a future table for
	// every earlier fingerprint.
	combinedNewColumns := append(append(append(append([]schemaColumn(nil), orderItemsV11NewColumns...), aiCostControlV12NewColumns...), platformFinanceV14NewColumns...), refundReviewV15NewColumns...)
	futureTables := append(append(append(append(append(append([]string(nil), tableOrderingV9Order...), "order_items"), aiCostControlV12NewTables...), announcementsMaintenanceV13NewTables...), platformFinanceV14NewTables...), customerProfilesV16NewTables...)
	if err := verifyPlatformExpenseReversalsV8HistoricalFingerprintsExcludingColumns(ctx, conn, base, v2, v3, v4, v5, v6, v7, v8, combinedNewColumns, futureTables...); err != nil {
		return err
	}
	if err := verifyCustomerProfilesV16Tables(ctx, conn); err != nil {
		return err
	}
	if err := verifyObjectInventoryV16(objects, base, v2, v7, v8, v9); err != nil {
		return err
	}
	rows, err := conn.QueryContext(ctx, "SELECT version, checksum, state, schema_fingerprint FROM "+ledgerTable+" ORDER BY version")
	if err != nil {
		return fmt.Errorf("read v16 schema version ledger: %w", err)
	}
	type ledgerEntry struct {
		version, checksum, state string
		fingerprint              sql.NullString
	}
	entries := make([]ledgerEntry, 0, 16)
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
	if len(entries) != 16 {
		return fmt.Errorf("refusing v16 ledger with %d rows; expected exactly immutable v1-v16 rows", len(entries))
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
	// v10/v11/v12/v14/v15 new columns.
	v9ExcludedColumns := append(append([]schemaColumn(nil), combinedNewColumns...), v10NewColumns...)
	v9ExcludedTables := append(append(append(append(append([]string(nil), "order_items"), aiCostControlV12NewTables...), announcementsMaintenanceV13NewTables...), platformFinanceV14NewTables...), customerProfilesV16NewTables...)
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
	// v11 check: exclude the v12/v13/v14/v16 tables, and the v12/v14/v15
	// columns (orders.channel is part of the stored v11 fingerprint; v13
	// added no columns to pre-existing tables).
	v11ExcludedColumns := append(append(append([]schemaColumn(nil), aiCostControlV12NewColumns...), platformFinanceV14NewColumns...), refundReviewV15NewColumns...)
	v11ExcludedTables := append(append(append(append([]string(nil), aiCostControlV12NewTables...), announcementsMaintenanceV13NewTables...), platformFinanceV14NewTables...), customerProfilesV16NewTables...)
	v11Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, v11ExcludedTables, nil, v11ExcludedColumns)
	if err != nil {
		return fmt.Errorf("verify immutable v11 schema fingerprint: %w", err)
	}
	if stored := byVersion[OrderItemsV11Version].fingerprint.String; stored != v11Fingerprint {
		return fmt.Errorf("refusing v11 schema drift: stored fingerprint %s, actual %s", stored, v11Fingerprint)
	}
	// v12 check: exclude the v13/v14/v16 tables and the v14/v15 columns
	// (the v12 columns are part of the stored v12 fingerprint).
	v12ExcludedColumns := append(append([]schemaColumn(nil), platformFinanceV14NewColumns...), refundReviewV15NewColumns...)
	v12Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, append(append(append([]string(nil), announcementsMaintenanceV13NewTables...), platformFinanceV14NewTables...), customerProfilesV16NewTables...), nil, v12ExcludedColumns)
	if err != nil {
		return fmt.Errorf("verify immutable v12 schema fingerprint: %w", err)
	}
	if stored := byVersion[AICostControlV12Version].fingerprint.String; stored != v12Fingerprint {
		return fmt.Errorf("refusing v12 schema drift: stored fingerprint %s, actual %s", stored, v12Fingerprint)
	}
	// v13 check: exclude only the v14/v16 tables and the v14/v15 columns
	// (v13 added no columns to pre-existing tables).
	v13ExcludedColumns := append(append([]schemaColumn(nil), platformFinanceV14NewColumns...), refundReviewV15NewColumns...)
	v13Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, append(append([]string(nil), platformFinanceV14NewTables...), customerProfilesV16NewTables...), nil, v13ExcludedColumns)
	if err != nil {
		return fmt.Errorf("verify immutable v13 schema fingerprint: %w", err)
	}
	if stored := byVersion[AnnouncementsMaintenanceV13Version].fingerprint.String; stored != v13Fingerprint {
		return fmt.Errorf("refusing v13 schema drift: stored fingerprint %s, actual %s", stored, v13Fingerprint)
	}
	// v14 check: exclude only the v16 table and the v15 columns (v15 adds
	// no tables).
	v14Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, customerProfilesV16NewTables, nil, refundReviewV15NewColumns)
	if err != nil {
		return fmt.Errorf("verify immutable v14 schema fingerprint: %w", err)
	}
	if stored := byVersion[PlatformFinanceV14Version].fingerprint.String; stored != v14Fingerprint {
		return fmt.Errorf("refusing v14 schema drift: stored fingerprint %s, actual %s", stored, v14Fingerprint)
	}
	// v15 check: exclude only the v16 table (v16 adds no columns).
	v15Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, customerProfilesV16NewTables, nil, nil)
	if err != nil {
		return fmt.Errorf("verify immutable v15 schema fingerprint: %w", err)
	}
	if stored := byVersion[RefundReviewV15Version].fingerprint.String; stored != v15Fingerprint {
		return fmt.Errorf("refusing v15 schema drift: stored fingerprint %s, actual %s", stored, v15Fingerprint)
	}
	actualV16, err := schemaFingerprint(ctx, conn)
	if err != nil {
		return fmt.Errorf("verify v16 managed schema fingerprint: %w", err)
	}
	if stored := byVersion[CustomerProfilesV16Version].fingerprint.String; stored != actualV16 {
		return fmt.Errorf("refusing v16 schema drift: stored fingerprint %s, actual %s", stored, actualV16)
	}
	_ = v6
	return nil
}

// verifyCustomerProfilesV16Tables checks the exact shape of the new
// customer_profiles table (15 columns, the merchant+phone unique key, the
// merchant/last-order and last-paid indexes, and the three non-negative
// CHECK constraints).
func verifyCustomerProfilesV16Tables(ctx context.Context, conn *sql.Conn) error {
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
			name: "customer_profiles",
			columns: []columnSpec{
				col("id", "bigint unsigned", "NO", "", false, "auto_increment", ""),
				col("created_at", "datetime(3)", "NO", "", false, "", ""),
				col("updated_at", "datetime(3)", "NO", "", false, "", ""),
				col("merchant_id", "bigint unsigned", "NO", "", false, "", ""),
				col("customer_phone", "varchar(30)", "NO", "", false, "", "utf8mb4_unicode_ci"),
				col("last_store_id", "bigint unsigned", "YES", "", false, "", ""),
				col("last_store_name", "varchar(120)", "NO", "", true, "", "utf8mb4_unicode_ci"),
				col("order_count", "bigint", "NO", "0", true, "", ""),
				col("total_amount_cents", "bigint", "NO", "0", true, "", ""),
				col("discount_amount_cents", "bigint", "NO", "0", true, "", ""),
				col("first_paid_at", "datetime(3)", "YES", "", false, "", ""),
				col("last_paid_at", "datetime(3)", "YES", "", false, "", ""),
				col("last_order_at", "datetime(3)", "YES", "", false, "", ""),
				col("last_order_id", "bigint unsigned", "NO", "0", true, "", ""),
				col("last_status", "varchar(20)", "NO", "", true, "", "utf8mb4_unicode_ci"),
			},
			indexes: []string{
				"PRIMARY|0|1|id|NULL|BTREE",
				"uk_customer_profiles_merchant_phone|0|1|merchant_id|NULL|BTREE",
				"uk_customer_profiles_merchant_phone|0|2|customer_phone|NULL|BTREE",
				"idx_customer_profiles_merchant_last_order|1|1|merchant_id|NULL|BTREE",
				"idx_customer_profiles_merchant_last_order|1|2|last_order_at|NULL|BTREE",
				"idx_customer_profiles_last_paid_at|1|1|last_paid_at|NULL|BTREE",
			},
			checks: []string{
				"chk_customer_profiles_discount_amount|customer_profiles|discount_amount_cents>=0|YES",
				"chk_customer_profiles_order_count|customer_profiles|order_count>=0|YES",
				"chk_customer_profiles_total_amount|customer_profiles|total_amount_cents>=0|YES",
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

func verifyObjectInventoryV16(objects schemaObjects, base baselineDefinition, v2 incrementalMigrationDefinition, v7 platformExpensesV7Definition, v8 platformExpenseReversalsV8Definition, v9 tableOrderingV9Definition) error {
	filtered := objects
	for _, name := range customerProfilesV16NewTables {
		if !containsSchemaName(objects.tables, name) {
			return fmt.Errorf("v16 table %q is missing", name)
		}
		filtered.tables = removeSchemaName(filtered.tables, name)
	}
	if err := verifyObjectInventoryV15(filtered, base, v2, v7, v8, v9); err != nil {
		return err
	}
	return nil
}
