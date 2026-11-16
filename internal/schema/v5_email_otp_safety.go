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
	EmailOTPSafetyV5Version  = "schema-v5-email-otp-safety-20261115"
	emailOTPSafetyV5DDL      = "ALTER TABLE `email_verification_codes` ADD COLUMN `code_hash` char(64) COLLATE utf8mb4_unicode_ci NULL, ADD COLUMN `failed_attempts` int unsigned NOT NULL DEFAULT 0"
	emailOTPSafetyV5Scrub    = "UPDATE `email_verification_codes` SET `code`='REDACTED', `code_hash`=REPEAT('0',64), `consumed_at`=COALESCE(`consumed_at`,CURRENT_TIMESTAMP(3))"
	EmailOTPSafetyV5Checksum = "59a48027c17bc375c3c1d15e22c0db966b3454de8980e9416c3c65d1eff69dbc"
)

type emailOTPSafetyV5Definition struct {
	version  string
	checksum string
	ddl      string
	scrub    string
}

func loadEmailOTPSafetyV5() (emailOTPSafetyV5Definition, error) {
	definition := emailOTPSafetyV5Definition{
		version: EmailOTPSafetyV5Version,
		ddl:     emailOTPSafetyV5DDL,
		scrub:   emailOTPSafetyV5Scrub,
	}
	material := definition.version + "\n" + definition.ddl + "\n" + definition.scrub
	sum := sha256.Sum256([]byte(material))
	definition.checksum = fmt.Sprintf("%x", sum)
	if definition.version != EmailOTPSafetyV5Version || definition.checksum != EmailOTPSafetyV5Checksum {
		return emailOTPSafetyV5Definition{}, fmt.Errorf("email OTP safety v5 definition checksum mismatch: definition %s, expected %s", definition.checksum, EmailOTPSafetyV5Checksum)
	}
	return definition, nil
}

// ApplyEmailOTPSafetyV5 adds hashed, attempt-limited storage to email OTPs only
// on an exact managed v4 schema. Existing plaintext email codes are overwritten
// and invalidated; unknown or drifted schemas remain untouched and fail closed.
func ApplyEmailOTPSafetyV5(ctx context.Context, db *sql.DB) (Result, error) {
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
	if err := verifyManagedSchemaV4(ctx, conn, objects, base, v2, v3, v4); err == nil {
		if err := applyEmailOTPSafetyV5(ctx, conn, objects, base, v2, v3, v4, v5); err != nil {
			return "", err
		}
		return ResultApplied, nil
	}
	if err := verifyManagedSchemaV5(ctx, conn, objects, base, v2, v3, v4, v5); err != nil {
		return "", fmt.Errorf("refusing non-exact managed schema for v5 email OTP safety migration: %w", err)
	}
	return ResultAlready, nil
}

func applyEmailOTPSafetyV5(ctx context.Context, conn *sql.Conn, objects schemaObjects, base baselineDefinition, v2 incrementalMigrationDefinition, v3 adminAuthV3Definition, v4 otpSafetyV4Definition, v5 emailOTPSafetyV5Definition) error {
	if _, err := conn.ExecContext(ctx, `INSERT INTO `+ledgerTable+` (version, checksum, state) VALUES (?, ?, 'applying')`, v5.version, v5.checksum); err != nil {
		return fmt.Errorf("record v5 email OTP safety migration start: %w", err)
	}
	if _, err := conn.ExecContext(ctx, v5.ddl); err != nil {
		return fmt.Errorf("apply v5 email OTP safety columns (v5 ledger remains incomplete): %w", err)
	}
	if _, err := conn.ExecContext(ctx, v5.scrub); err != nil {
		return fmt.Errorf("invalidate and scrub legacy email OTPs (v5 ledger remains incomplete): %w", err)
	}
	if err := verifyEmailOTPSafetyV5Columns(ctx, conn); err != nil {
		return fmt.Errorf("verify v5 email OTP safety columns (v5 ledger remains incomplete): %w", err)
	}
	objects, err := inspectObjects(ctx, conn)
	if err != nil {
		return err
	}
	if err := verifyObjectInventoryV2(objects, base, v2); err != nil {
		return fmt.Errorf("post-apply v5 inventory verification failed (v5 ledger remains incomplete): %w", err)
	}
	if err := verifyEmailOTPHistoricalFingerprints(ctx, conn, base, v2, v3, v4); err != nil {
		return fmt.Errorf("verify immutable v1-v4 schema during v5 migration (v5 ledger remains incomplete): %w", err)
	}

	fingerprint, err := schemaFingerprint(ctx, conn)
	if err != nil {
		return fmt.Errorf("compute v5 installed schema fingerprint: %w", err)
	}
	result, err := conn.ExecContext(ctx, `UPDATE `+ledgerTable+`
		SET state='applied', schema_fingerprint=?, applied_at=CURRENT_TIMESTAMP
		WHERE version=? AND checksum=? AND state='applying'`, fingerprint, v5.version, v5.checksum)
	if err != nil {
		return fmt.Errorf("complete v5 schema version ledger: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("verify v5 schema version ledger update: %w", err)
	}
	if rows != 1 {
		return errors.New("v5 schema version ledger did not transition exactly once; it remains incomplete")
	}
	return nil
}

func verifyManagedSchemaV5(ctx context.Context, conn *sql.Conn, objects schemaObjects, base baselineDefinition, v2 incrementalMigrationDefinition, v3 adminAuthV3Definition, v4 otpSafetyV4Definition, v5 emailOTPSafetyV5Definition) error {
	if err := verifyAdminAuthV3Column(ctx, conn); err != nil {
		return err
	}
	if err := verifyOTPSafetyV4Columns(ctx, conn); err != nil {
		return err
	}
	if err := verifyEmailOTPSafetyV5Columns(ctx, conn); err != nil {
		return err
	}
	if err := verifyObjectInventoryV2(objects, base, v2); err != nil {
		return err
	}

	rows, err := conn.QueryContext(ctx, "SELECT version, checksum, state, schema_fingerprint FROM "+ledgerTable+" ORDER BY version")
	if err != nil {
		return fmt.Errorf("read v5 schema version ledger: %w", err)
	}
	type ledgerEntry struct {
		version, checksum, state string
		fingerprint              sql.NullString
	}
	entries := make([]ledgerEntry, 0, 5)
	for rows.Next() {
		var entry ledgerEntry
		if err := rows.Scan(&entry.version, &entry.checksum, &entry.state, &entry.fingerprint); err != nil {
			rows.Close()
			return fmt.Errorf("scan v5 schema version ledger: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("read v5 schema version ledger rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close v5 schema version ledger: %w", err)
	}
	if len(entries) != 5 {
		return fmt.Errorf("refusing v5 ledger with %d rows; expected exactly the immutable v1, v2, v3, v4, and v5 rows", len(entries))
	}
	entryByVersion := make(map[string]ledgerEntry, len(entries))
	for _, entry := range entries {
		entryByVersion[entry.version] = entry
	}
	expected := []struct{ version, checksum string }{
		{BaselineVersion, base.checksum},
		{v2.version, v2.checksum},
		{v3.version, v3.checksum},
		{v4.version, v4.checksum},
		{v5.version, v5.checksum},
	}
	for _, want := range expected {
		entry, ok := entryByVersion[want.version]
		if !ok || entry.checksum != want.checksum || entry.state != "applied" || !entry.fingerprint.Valid || entry.fingerprint.String == "" {
			return fmt.Errorf("refusing missing, altered, or incomplete %s ledger entry", want.version)
		}
	}
	if err := verifyEmailOTPHistoricalFingerprints(ctx, conn, base, v2, v3, v4); err != nil {
		return err
	}
	actualFingerprint, err := schemaFingerprint(ctx, conn)
	if err != nil {
		return fmt.Errorf("verify v5 managed schema fingerprint: %w", err)
	}
	if stored := entryByVersion[v5.version].fingerprint.String; stored != actualFingerprint {
		return fmt.Errorf("refusing v5 schema drift: stored fingerprint %s, actual %s", stored, actualFingerprint)
	}
	return nil
}

func verifyEmailOTPHistoricalFingerprints(ctx context.Context, conn *sql.Conn, base baselineDefinition, v2 incrementalMigrationDefinition, v3 adminAuthV3Definition, v4 otpSafetyV4Definition) error {
	allOTPColumns := []schemaColumn{
		{table: "verification_codes", column: "code_hash"},
		{table: "verification_codes", column: "failed_attempts"},
		{table: "email_verification_codes", column: "code_hash"},
		{table: "email_verification_codes", column: "failed_attempts"},
	}
	for _, check := range []struct {
		version, checksum, label string
		excludedTables           []string
		excludedTriggers         []string
		excludedColumns          []schemaColumn
	}{
		{
			version: BaselineVersion, checksum: base.checksum, label: "v1",
			excludedTables: manualReviewV2TableNames(), excludedTriggers: manualReviewV2TriggerNames(),
			excludedColumns: append([]schemaColumn{{table: "admin_users", column: "auth_version"}}, allOTPColumns...),
		},
		{
			version: v2.version, checksum: v2.checksum, label: "v2",
			excludedColumns: append([]schemaColumn{{table: "admin_users", column: "auth_version"}}, allOTPColumns...),
		},
		{
			version: v3.version, checksum: v3.checksum, label: "v3",
			excludedColumns: allOTPColumns,
		},
		{
			version: v4.version, checksum: v4.checksum, label: "v4",
			excludedColumns: []schemaColumn{{table: "email_verification_codes", column: "code_hash"}, {table: "email_verification_codes", column: "failed_attempts"}},
		},
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

func verifyEmailOTPSafetyV5Columns(ctx context.Context, conn *sql.Conn) error {
	var count int
	err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='email_verification_codes' AND (
			(COLUMN_NAME='code_hash' AND COLUMN_TYPE='char(64)' AND IS_NULLABLE='YES' AND COALESCE(COLLATION_NAME,'')='utf8mb4_unicode_ci' AND COLUMN_DEFAULT IS NULL AND EXTRA='') OR
			(COLUMN_NAME='failed_attempts' AND COLUMN_TYPE='int unsigned' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='0' AND EXTRA='')
		)`).Scan(&count)
	if err != nil {
		return fmt.Errorf("inspect email_verification_codes OTP safety columns: %w", err)
	}
	if count != 2 {
		return errors.New("email_verification_codes OTP safety column definition mismatch")
	}
	return nil
}
