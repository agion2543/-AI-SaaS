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
	OTPSafetyV4Version  = "schema-v4-otp-safety-20261114"
	otpSafetyV4DDL      = "ALTER TABLE `verification_codes` ADD COLUMN `code_hash` char(64) COLLATE utf8mb4_unicode_ci NULL, ADD COLUMN `failed_attempts` int unsigned NOT NULL DEFAULT 0"
	otpSafetyV4Scrub    = "UPDATE `verification_codes` SET `code`='REDACTED', `code_hash`=REPEAT('0',64), `consumed_at`=COALESCE(`consumed_at`,CURRENT_TIMESTAMP(3))"
	OTPSafetyV4Checksum = "90aba9d894e7e6af5ecc9904745501472ceb1c20f74248fe360649eca371dde0"
)

type otpSafetyV4Definition struct {
	version  string
	checksum string
	ddl      string
	scrub    string
}

func loadOTPSafetyV4() (otpSafetyV4Definition, error) {
	definition := otpSafetyV4Definition{
		version: OTPSafetyV4Version,
		ddl:     otpSafetyV4DDL,
		scrub:   otpSafetyV4Scrub,
	}
	material := definition.version + "\n" + definition.ddl + "\n" + definition.scrub
	sum := sha256.Sum256([]byte(material))
	definition.checksum = fmt.Sprintf("%x", sum)
	if definition.version != OTPSafetyV4Version || definition.checksum != OTPSafetyV4Checksum {
		return otpSafetyV4Definition{}, fmt.Errorf("OTP safety v4 definition checksum mismatch: definition %s, expected %s", definition.checksum, OTPSafetyV4Checksum)
	}
	return definition, nil
}

// ApplyOTPSafetyV4 changes phone verification storage only on an exact managed
// v3 schema. It accepts exact v4 as a verified no-op and refuses every other
// schema without attempting to repair it.
func ApplyOTPSafetyV4(ctx context.Context, db *sql.DB) (Result, error) {
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
	if err := verifyManagedSchemaV3(ctx, conn, objects, base, v2, v3); err == nil {
		if err := applyOTPSafetyV4(ctx, conn, objects, base, v2, v3, v4); err != nil {
			return "", err
		}
		return ResultApplied, nil
	}
	if err := verifyManagedSchemaV4(ctx, conn, objects, base, v2, v3, v4); err != nil {
		return "", fmt.Errorf("refusing non-exact managed schema for v4 OTP safety migration: %w", err)
	}
	return ResultAlready, nil
}

func applyOTPSafetyV4(ctx context.Context, conn *sql.Conn, objects schemaObjects, base baselineDefinition, v2 incrementalMigrationDefinition, v3 adminAuthV3Definition, v4 otpSafetyV4Definition) error {
	if _, err := conn.ExecContext(ctx, `INSERT INTO `+ledgerTable+` (version, checksum, state) VALUES (?, ?, 'applying')`, v4.version, v4.checksum); err != nil {
		return fmt.Errorf("record v4 OTP safety migration start: %w", err)
	}
	if _, err := conn.ExecContext(ctx, v4.ddl); err != nil {
		return fmt.Errorf("apply v4 OTP safety columns (v4 ledger remains incomplete): %w", err)
	}
	if _, err := conn.ExecContext(ctx, v4.scrub); err != nil {
		return fmt.Errorf("invalidate and scrub legacy OTPs (v4 ledger remains incomplete): %w", err)
	}
	if err := verifyOTPSafetyV4Columns(ctx, conn); err != nil {
		return fmt.Errorf("verify v4 OTP safety columns (v4 ledger remains incomplete): %w", err)
	}
	objects, err := inspectObjects(ctx, conn)
	if err != nil {
		return err
	}
	if err := verifyObjectInventoryV2(objects, base, v2); err != nil {
		return fmt.Errorf("post-apply v4 inventory verification failed (v4 ledger remains incomplete): %w", err)
	}

	for _, check := range []struct {
		version, checksum, label         string
		excludedTables, excludedTriggers []string
		excludedColumns                  []schemaColumn
	}{
		{
			version: BaselineVersion, checksum: base.checksum, label: "v1",
			excludedTables: manualReviewV2TableNames(), excludedTriggers: manualReviewV2TriggerNames(),
			excludedColumns: []schemaColumn{{table: "admin_users", column: "auth_version"}, {table: "verification_codes", column: "code_hash"}, {table: "verification_codes", column: "failed_attempts"}},
		},
		{
			version: v2.version, checksum: v2.checksum, label: "v2",
			excludedColumns: []schemaColumn{{table: "admin_users", column: "auth_version"}, {table: "verification_codes", column: "code_hash"}, {table: "verification_codes", column: "failed_attempts"}},
		},
		{
			version: v3.version, checksum: v3.checksum, label: "v3",
			excludedColumns: otpSafetyV4SchemaColumns(),
		},
	} {
		actual, err := schemaFingerprintExcludingColumns(ctx, conn, check.excludedTables, check.excludedTriggers, check.excludedColumns)
		if err != nil {
			return fmt.Errorf("verify original %s schema remained unchanged during v4 migration: %w", check.label, err)
		}
		var stored string
		if err := conn.QueryRowContext(ctx, "SELECT schema_fingerprint FROM "+ledgerTable+" WHERE version=? AND checksum=? AND state='applied'", check.version, check.checksum).Scan(&stored); err != nil {
			return fmt.Errorf("read immutable %s fingerprint after v4 migration: %w", check.label, err)
		}
		if stored != actual {
			return fmt.Errorf("refusing v4 completion because the exact %s schema fingerprint changed: stored %s, actual %s", check.label, stored, actual)
		}
	}

	fingerprint, err := schemaFingerprint(ctx, conn)
	if err != nil {
		return fmt.Errorf("compute v4 installed schema fingerprint: %w", err)
	}
	result, err := conn.ExecContext(ctx, `UPDATE `+ledgerTable+`
		SET state='applied', schema_fingerprint=?, applied_at=CURRENT_TIMESTAMP
		WHERE version=? AND checksum=? AND state='applying'`, fingerprint, v4.version, v4.checksum)
	if err != nil {
		return fmt.Errorf("complete v4 schema version ledger: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("verify v4 schema version ledger update: %w", err)
	}
	if rows != 1 {
		return errors.New("v4 schema version ledger did not transition exactly once; it remains incomplete")
	}
	return nil
}

func verifyManagedSchemaV4(ctx context.Context, conn *sql.Conn, objects schemaObjects, base baselineDefinition, v2 incrementalMigrationDefinition, v3 adminAuthV3Definition, v4 otpSafetyV4Definition) error {
	if err := verifyAdminAuthV3Column(ctx, conn); err != nil {
		return err
	}
	if err := verifyOTPSafetyV4Columns(ctx, conn); err != nil {
		return err
	}
	if err := verifyObjectInventoryV2(objects, base, v2); err != nil {
		return err
	}
	rows, err := conn.QueryContext(ctx, "SELECT version, checksum, state, schema_fingerprint FROM "+ledgerTable+" ORDER BY version")
	if err != nil {
		return fmt.Errorf("read v4 schema version ledger: %w", err)
	}
	type ledgerEntry struct {
		version, checksum, state string
		fingerprint              sql.NullString
	}
	entries := make([]ledgerEntry, 0, 4)
	for rows.Next() {
		var entry ledgerEntry
		if err := rows.Scan(&entry.version, &entry.checksum, &entry.state, &entry.fingerprint); err != nil {
			rows.Close()
			return fmt.Errorf("scan v4 schema version ledger: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("read v4 schema version ledger rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close v4 schema version ledger: %w", err)
	}
	if len(entries) != 4 {
		return fmt.Errorf("refusing v4 ledger with %d rows; expected exactly the immutable v1, v2, v3, and v4 rows", len(entries))
	}
	entryByVersion := make(map[string]ledgerEntry, len(entries))
	for _, entry := range entries {
		entryByVersion[entry.version] = entry
	}
	for _, expected := range []struct{ version, checksum string }{
		{BaselineVersion, base.checksum},
		{v2.version, v2.checksum},
		{v3.version, v3.checksum},
		{v4.version, v4.checksum},
	} {
		entry, ok := entryByVersion[expected.version]
		if !ok || entry.checksum != expected.checksum || entry.state != "applied" || !entry.fingerprint.Valid || entry.fingerprint.String == "" {
			return fmt.Errorf("refusing missing, altered, or incomplete %s ledger entry", expected.version)
		}
	}

	for _, check := range []struct {
		version, label                   string
		excludedTables, excludedTriggers []string
		excludedColumns                  []schemaColumn
	}{
		{
			version: BaselineVersion, label: "v1",
			excludedTables: manualReviewV2TableNames(), excludedTriggers: manualReviewV2TriggerNames(),
			excludedColumns: []schemaColumn{{table: "admin_users", column: "auth_version"}, {table: "verification_codes", column: "code_hash"}, {table: "verification_codes", column: "failed_attempts"}},
		},
		{
			version: v2.version, label: "v2",
			excludedColumns: []schemaColumn{{table: "admin_users", column: "auth_version"}, {table: "verification_codes", column: "code_hash"}, {table: "verification_codes", column: "failed_attempts"}},
		},
		{
			version: v3.version, label: "v3",
			excludedColumns: otpSafetyV4SchemaColumns(),
		},
	} {
		fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, check.excludedTables, check.excludedTriggers, check.excludedColumns)
		if err != nil {
			return fmt.Errorf("verify original %s schema fingerprint: %w", check.label, err)
		}
		if stored := entryByVersion[check.version].fingerprint.String; stored != fingerprint {
			return fmt.Errorf("refusing %s schema drift: stored fingerprint %s, actual %s", check.label, stored, fingerprint)
		}
	}
	actualFingerprint, err := schemaFingerprint(ctx, conn)
	if err != nil {
		return fmt.Errorf("verify v4 managed schema fingerprint: %w", err)
	}
	if entryByVersion[v4.version].fingerprint.String != actualFingerprint {
		return fmt.Errorf("refusing v4 schema drift: stored fingerprint %s, actual %s", entryByVersion[v4.version].fingerprint.String, actualFingerprint)
	}
	return nil
}

func verifyOTPSafetyV4Columns(ctx context.Context, conn *sql.Conn) error {
	var count int
	err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='verification_codes' AND (
			(COLUMN_NAME='code_hash' AND COLUMN_TYPE='char(64)' AND IS_NULLABLE='YES' AND COALESCE(COLLATION_NAME,'')='utf8mb4_unicode_ci' AND COLUMN_DEFAULT IS NULL AND EXTRA='') OR
			(COLUMN_NAME='failed_attempts' AND COLUMN_TYPE='int unsigned' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='0' AND EXTRA='')
		)`).Scan(&count)
	if err != nil {
		return fmt.Errorf("inspect verification_codes OTP safety columns: %w", err)
	}
	if count != 2 {
		return fmt.Errorf("verification_codes OTP safety column definition mismatch")
	}
	return nil
}

func otpSafetyV4SchemaColumns() []schemaColumn {
	return []schemaColumn{{table: "verification_codes", column: "code_hash"}, {table: "verification_codes", column: "failed_attempts"}}
}
