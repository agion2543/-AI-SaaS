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
	WholesaleRetailV6Version  = "schema-v6-wholesale-retail-20261116"
	WholesaleRetailV6DDL      = "ALTER TABLE `store_products` ADD COLUMN `sales_unit` varchar(30) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '', ADD COLUMN `specification` varchar(80) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '', ADD COLUMN `minimum_order_quantity` int unsigned NOT NULL DEFAULT 1, ADD COLUMN `price_tiers` json NULL"
	WholesaleRetailV6Checksum = "a925b154315d9d34475e08911fcc0abb25b7986a14c3da11d3bc251f9adc65f0"
)

type wholesaleRetailV6Definition struct {
	version  string
	checksum string
	ddl      string
}

func loadWholesaleRetailV6() (wholesaleRetailV6Definition, error) {
	definition := wholesaleRetailV6Definition{version: WholesaleRetailV6Version, ddl: WholesaleRetailV6DDL}
	material := definition.version + "\n" + definition.ddl
	sum := sha256.Sum256([]byte(material))
	definition.checksum = fmt.Sprintf("%x", sum)
	if definition.version != WholesaleRetailV6Version || definition.checksum != WholesaleRetailV6Checksum {
		return wholesaleRetailV6Definition{}, fmt.Errorf("wholesale/retail v6 definition checksum mismatch: definition %s, expected %s", definition.checksum, WholesaleRetailV6Checksum)
	}
	return definition, nil
}

// ApplyWholesaleRetailV6 adds optional product unit/specification, minimum order quantity,
// and merchant-configured quantity-tier prices only to an exact managed v5 schema.
func ApplyWholesaleRetailV6(ctx context.Context, db *sql.DB) (Result, error) {
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
	if err := verifyManagedSchemaV5(ctx, conn, objects, base, v2, v3, v4, v5); err == nil {
		if err := applyWholesaleRetailV6(ctx, conn, objects, base, v2, v3, v4, v5, v6); err != nil {
			return "", err
		}
		return ResultApplied, nil
	}
	if err := verifyManagedSchemaV6(ctx, conn, objects, base, v2, v3, v4, v5, v6); err != nil {
		return "", fmt.Errorf("refusing non-exact managed schema for v6 wholesale/retail migration: %w", err)
	}
	return ResultAlready, nil
}

func applyWholesaleRetailV6(ctx context.Context, conn *sql.Conn, objects schemaObjects, base baselineDefinition, v2 incrementalMigrationDefinition, v3 adminAuthV3Definition, v4 otpSafetyV4Definition, v5 emailOTPSafetyV5Definition, v6 wholesaleRetailV6Definition) error {
	if _, err := conn.ExecContext(ctx, `INSERT INTO `+ledgerTable+` (version, checksum, state) VALUES (?, ?, 'applying')`, v6.version, v6.checksum); err != nil {
		return fmt.Errorf("record v6 wholesale/retail migration start: %w", err)
	}
	if _, err := conn.ExecContext(ctx, v6.ddl); err != nil {
		return fmt.Errorf("apply v6 product pricing columns (v6 ledger remains incomplete): %w", err)
	}
	if err := verifyWholesaleRetailV6Columns(ctx, conn); err != nil {
		return fmt.Errorf("verify v6 product pricing columns (v6 ledger remains incomplete): %w", err)
	}
	objects, err := inspectObjects(ctx, conn)
	if err != nil {
		return err
	}
	if err := verifyObjectInventoryV2(objects, base, v2); err != nil {
		return fmt.Errorf("post-apply v6 inventory verification failed (v6 ledger remains incomplete): %w", err)
	}
	if err := verifyWholesaleRetailHistoricalFingerprints(ctx, conn, base, v2, v3, v4, v5); err != nil {
		return fmt.Errorf("verify immutable v1-v5 schema during v6 migration (v6 ledger remains incomplete): %w", err)
	}
	fingerprint, err := schemaFingerprint(ctx, conn)
	if err != nil {
		return fmt.Errorf("compute v6 installed schema fingerprint: %w", err)
	}
	result, err := conn.ExecContext(ctx, `UPDATE `+ledgerTable+`
SET state='applied', schema_fingerprint=?, applied_at=CURRENT_TIMESTAMP
WHERE version=? AND checksum=? AND state='applying'`, fingerprint, v6.version, v6.checksum)
	if err != nil {
		return fmt.Errorf("complete v6 schema version ledger: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("verify v6 schema version ledger update: %w", err)
	}
	if rows != 1 {
		return errors.New("v6 schema version ledger did not transition exactly once; it remains incomplete")
	}
	return nil
}

func verifyManagedSchemaV6(ctx context.Context, conn *sql.Conn, objects schemaObjects, base baselineDefinition, v2 incrementalMigrationDefinition, v3 adminAuthV3Definition, v4 otpSafetyV4Definition, v5 emailOTPSafetyV5Definition, v6 wholesaleRetailV6Definition) error {
	if err := verifyAdminAuthV3Column(ctx, conn); err != nil {
		return err
	}
	if err := verifyOTPSafetyV4Columns(ctx, conn); err != nil {
		return err
	}
	if err := verifyEmailOTPSafetyV5Columns(ctx, conn); err != nil {
		return err
	}
	if err := verifyWholesaleRetailV6Columns(ctx, conn); err != nil {
		return err
	}
	if err := verifyObjectInventoryV2(objects, base, v2); err != nil {
		return err
	}

	rows, err := conn.QueryContext(ctx, "SELECT version, checksum, state, schema_fingerprint FROM "+ledgerTable+" ORDER BY version")
	if err != nil {
		return fmt.Errorf("read v6 schema version ledger: %w", err)
	}
	type ledgerEntry struct {
		version, checksum, state string
		fingerprint              sql.NullString
	}
	entries := make([]ledgerEntry, 0, 6)
	for rows.Next() {
		var entry ledgerEntry
		if err := rows.Scan(&entry.version, &entry.checksum, &entry.state, &entry.fingerprint); err != nil {
			rows.Close()
			return fmt.Errorf("scan v6 schema version ledger: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("read v6 schema version ledger rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close v6 schema version ledger rows: %w", err)
	}
	if len(entries) != 6 {
		return fmt.Errorf("refusing v6 ledger with %d rows; expected exactly the immutable v1-v6 rows", len(entries))
	}
	entryByVersion := make(map[string]ledgerEntry, len(entries))
	for _, entry := range entries {
		entryByVersion[entry.version] = entry
	}
	expected := []struct{ version, checksum string }{
		{BaselineVersion, base.checksum}, {v2.version, v2.checksum}, {v3.version, v3.checksum},
		{v4.version, v4.checksum}, {v5.version, v5.checksum}, {v6.version, v6.checksum},
	}
	for _, want := range expected {
		entry, ok := entryByVersion[want.version]
		if !ok || entry.checksum != want.checksum || entry.state != "applied" || !entry.fingerprint.Valid || entry.fingerprint.String == "" {
			return fmt.Errorf("refusing missing, altered, or incomplete %s ledger entry", want.version)
		}
	}
	if err := verifyWholesaleRetailHistoricalFingerprints(ctx, conn, base, v2, v3, v4, v5); err != nil {
		return err
	}
	actualFingerprint, err := schemaFingerprint(ctx, conn)
	if err != nil {
		return fmt.Errorf("verify v6 managed schema fingerprint: %w", err)
	}
	if stored := entryByVersion[v6.version].fingerprint.String; stored != actualFingerprint {
		return fmt.Errorf("refusing v6 schema drift: stored fingerprint %s, actual %s", stored, actualFingerprint)
	}
	return nil
}

func verifyWholesaleRetailV6Columns(ctx context.Context, conn *sql.Conn) error {
	var count int
	err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='store_products' AND (
(COLUMN_NAME='sales_unit' AND COLUMN_TYPE='varchar(30)' AND IS_NULLABLE='NO' AND COALESCE(COLLATION_NAME,'')='utf8mb4_unicode_ci' AND COLUMN_DEFAULT='') OR
(COLUMN_NAME='specification' AND COLUMN_TYPE='varchar(80)' AND IS_NULLABLE='NO' AND COALESCE(COLLATION_NAME,'')='utf8mb4_unicode_ci' AND COLUMN_DEFAULT='') OR
(COLUMN_NAME='minimum_order_quantity' AND COLUMN_TYPE='int unsigned' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='1') OR
(COLUMN_NAME='price_tiers' AND COLUMN_TYPE='json' AND IS_NULLABLE='YES' AND COLUMN_DEFAULT IS NULL)
)`).Scan(&count)
	if err != nil {
		return fmt.Errorf("inspect store_products v6 pricing columns: %w", err)
	}
	if count != 4 {
		return errors.New("store_products v6 pricing column definitions mismatch")
	}
	return nil
}

func verifyWholesaleRetailHistoricalFingerprints(ctx context.Context, conn *sql.Conn, base baselineDefinition, v2 incrementalMigrationDefinition, v3 adminAuthV3Definition, v4 otpSafetyV4Definition, v5 emailOTPSafetyV5Definition) error {
	return verifyWholesaleRetailHistoricalFingerprintsExcluding(ctx, conn, base, v2, v3, v4, v5, nil, nil)
}

func verifyWholesaleRetailHistoricalFingerprintsExcluding(ctx context.Context, conn *sql.Conn, base baselineDefinition, v2 incrementalMigrationDefinition, v3 adminAuthV3Definition, v4 otpSafetyV4Definition, v5 emailOTPSafetyV5Definition, additionalTables, additionalTriggers []string) error {
	otpColumns := []schemaColumn{
		{table: "verification_codes", column: "code_hash"},
		{table: "verification_codes", column: "failed_attempts"},
		{table: "email_verification_codes", column: "code_hash"},
		{table: "email_verification_codes", column: "failed_attempts"},
	}
	v6Columns := []schemaColumn{
		{table: "store_products", column: "sales_unit"},
		{table: "store_products", column: "specification"},
		{table: "store_products", column: "minimum_order_quantity"},
		{table: "store_products", column: "price_tiers"},
	}
	for _, check := range []struct {
		version, checksum, label string
		excludedTables           []string
		excludedTriggers         []string
		excludedColumns          []schemaColumn
	}{
		{version: BaselineVersion, checksum: base.checksum, label: "v1", excludedTables: append(manualReviewV2TableNames(), additionalTables...), excludedTriggers: append(manualReviewV2TriggerNames(), additionalTriggers...), excludedColumns: append(append([]schemaColumn{{table: "admin_users", column: "auth_version"}}, otpColumns...), v6Columns...)},
		{version: v2.version, checksum: v2.checksum, label: "v2", excludedTables: additionalTables, excludedTriggers: additionalTriggers, excludedColumns: append(append([]schemaColumn{{table: "admin_users", column: "auth_version"}}, otpColumns...), v6Columns...)},
		{version: v3.version, checksum: v3.checksum, label: "v3", excludedTables: additionalTables, excludedTriggers: additionalTriggers, excludedColumns: append(append([]schemaColumn(nil), otpColumns...), v6Columns...)},
		{version: v4.version, checksum: v4.checksum, label: "v4", excludedTables: additionalTables, excludedTriggers: additionalTriggers, excludedColumns: append([]schemaColumn{{table: "email_verification_codes", column: "code_hash"}, {table: "email_verification_codes", column: "failed_attempts"}}, v6Columns...)},
		{version: v5.version, checksum: v5.checksum, label: "v5", excludedTables: additionalTables, excludedTriggers: additionalTriggers, excludedColumns: v6Columns},
	} {
		actual, err := schemaFingerprintExcludingColumns(ctx, conn, check.excludedTables, check.excludedTriggers, check.excludedColumns)
		if err != nil {
			return fmt.Errorf("compute immutable %s schema fingerprint: %w", check.label, err)
		}
		var stored string
		if err := conn.QueryRowContext(ctx, "SELECT schema_fingerprint FROM "+ledgerTable+" WHERE version=? AND checksum=? AND state='applied'", check.version, check.checksum).Scan(&stored); err != nil {
			return fmt.Errorf("read immutable %s fingerprint: %w", check.label, err)
		}
		if stored != actual {
			return fmt.Errorf("refusing %s schema drift: stored fingerprint %s, actual %s", check.label, stored, actual)
		}
	}
	return nil
}

func wholesaleRetailV6ColumnNames() []string {
	columns := []string{"sales_unit", "specification", "minimum_order_quantity", "price_tiers"}
	sort.Strings(columns)
	return columns
}
