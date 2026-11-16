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
	AdminAuthV3Version  = "schema-v3-admin-auth-version-20261114"
	AdminAuthV3Checksum = "1398762fd14d351eeb61e2b26f98513a18401bbc77e6da677b91e2f5481f23d4"
	adminAuthV3DDL      = "ALTER TABLE `admin_users` ADD COLUMN `auth_version` bigint unsigned NOT NULL DEFAULT 1"
)

type adminAuthV3Definition struct {
	version  string
	checksum string
	ddl      string
}

func loadAdminAuthV3() (adminAuthV3Definition, error) {
	definition := adminAuthV3Definition{version: AdminAuthV3Version, ddl: adminAuthV3DDL}
	material := definition.version + "\n" + definition.ddl
	sum := sha256.Sum256([]byte(material))
	definition.checksum = fmt.Sprintf("%x", sum)
	if definition.version != AdminAuthV3Version || definition.checksum != AdminAuthV3Checksum {
		return adminAuthV3Definition{}, fmt.Errorf("admin auth v3 definition checksum mismatch: definition %s, expected %s", definition.checksum, AdminAuthV3Checksum)
	}
	return definition, nil
}

// ApplyAdminAuthVersionV3 adds the persistent administrator auth version to an
// exact managed v2 schema. It accepts an exact managed v3 as a verified no-op;
// all unknown or drifted schemas remain untouched and fail closed.
func ApplyAdminAuthVersionV3(ctx context.Context, db *sql.DB) (Result, error) {
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
	if err := verifyManagedSchemaV2(ctx, conn, objects, base, v2); err == nil {
		if err := applyAdminAuthV3(ctx, conn, base, v2, v3); err != nil {
			return "", err
		}
		return ResultApplied, nil
	}
	if err := verifyManagedSchemaV3(ctx, conn, objects, base, v2, v3); err != nil {
		return "", fmt.Errorf("refusing non-exact managed schema for v3 admin auth version migration: %w", err)
	}
	return ResultAlready, nil
}

func applyAdminAuthV3(ctx context.Context, conn *sql.Conn, base baselineDefinition, v2 incrementalMigrationDefinition, v3 adminAuthV3Definition) error {
	if _, err := conn.ExecContext(ctx, `INSERT INTO `+ledgerTable+` (version, checksum, state) VALUES (?, ?, 'applying')`, v3.version, v3.checksum); err != nil {
		return fmt.Errorf("record v3 admin auth version migration start: %w", err)
	}
	if _, err := conn.ExecContext(ctx, v3.ddl); err != nil {
		return fmt.Errorf("apply v3 admin auth version column (v3 ledger remains incomplete): %w", err)
	}
	if err := verifyAdminAuthV3Column(ctx, conn); err != nil {
		return fmt.Errorf("verify v3 admin auth version column (v3 ledger remains incomplete): %w", err)
	}
	objects, err := inspectObjects(ctx, conn)
	if err != nil {
		return err
	}
	if err := verifyObjectInventoryV2(objects, base, v2); err != nil {
		return fmt.Errorf("post-apply v3 inventory verification failed (v3 ledger remains incomplete): %w", err)
	}
	v1Fingerprint, err := schemaFingerprintExcludingColumn(ctx, conn, manualReviewV2TableNames(), manualReviewV2TriggerNames(), "admin_users", "auth_version")
	if err != nil {
		return fmt.Errorf("verify v1 schema remained unchanged during v3 migration: %w", err)
	}
	var storedV1Fingerprint string
	if err := conn.QueryRowContext(ctx, "SELECT schema_fingerprint FROM "+ledgerTable+" WHERE version=? AND checksum=? AND state='applied'", BaselineVersion, base.checksum).Scan(&storedV1Fingerprint); err != nil {
		return fmt.Errorf("read immutable v1 fingerprint after v3 migration: %w", err)
	}
	if storedV1Fingerprint != v1Fingerprint {
		return fmt.Errorf("refusing v3 completion because the exact v1 schema fingerprint changed: stored %s, actual %s", storedV1Fingerprint, v1Fingerprint)
	}
	v2Fingerprint, err := schemaFingerprintExcludingColumn(ctx, conn, nil, nil, "admin_users", "auth_version")
	if err != nil {
		return fmt.Errorf("verify v2 schema remained unchanged during v3 migration: %w", err)
	}
	var storedV2Fingerprint string
	if err := conn.QueryRowContext(ctx, "SELECT schema_fingerprint FROM "+ledgerTable+" WHERE version=? AND checksum=? AND state='applied'", v2.version, v2.checksum).Scan(&storedV2Fingerprint); err != nil {
		return fmt.Errorf("read immutable v2 fingerprint after v3 migration: %w", err)
	}
	if storedV2Fingerprint != v2Fingerprint {
		return fmt.Errorf("refusing v3 completion because the exact v2 schema fingerprint changed: stored %s, actual %s", storedV2Fingerprint, v2Fingerprint)
	}
	fingerprint, err := schemaFingerprint(ctx, conn)
	if err != nil {
		return fmt.Errorf("compute v3 installed schema fingerprint: %w", err)
	}
	result, err := conn.ExecContext(ctx, `UPDATE `+ledgerTable+`
		SET state='applied', schema_fingerprint=?, applied_at=CURRENT_TIMESTAMP
		WHERE version=? AND checksum=? AND state='applying'`, fingerprint, v3.version, v3.checksum)
	if err != nil {
		return fmt.Errorf("complete v3 schema version ledger: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("verify v3 schema version ledger update: %w", err)
	}
	if rows != 1 {
		return errors.New("v3 schema version ledger did not transition exactly once; it remains incomplete")
	}
	return nil
}

func verifyManagedSchemaV3(ctx context.Context, conn *sql.Conn, objects schemaObjects, base baselineDefinition, v2 incrementalMigrationDefinition, v3 adminAuthV3Definition) error {
	if err := verifyAdminAuthV3Column(ctx, conn); err != nil {
		return err
	}
	if err := verifyObjectInventoryV2(objects, base, v2); err != nil {
		return err
	}
	rows, err := conn.QueryContext(ctx, "SELECT version, checksum, state, schema_fingerprint FROM "+ledgerTable+" ORDER BY version")
	if err != nil {
		return fmt.Errorf("read v3 schema version ledger: %w", err)
	}
	type ledgerEntry struct {
		version     string
		checksum    string
		state       string
		fingerprint sql.NullString
	}
	entries := make([]ledgerEntry, 0, 3)
	for rows.Next() {
		var entry ledgerEntry
		if err := rows.Scan(&entry.version, &entry.checksum, &entry.state, &entry.fingerprint); err != nil {
			rows.Close()
			return fmt.Errorf("scan v3 schema version ledger: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("read v3 schema version ledger rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close v3 schema version ledger rows: %w", err)
	}
	if len(entries) != 3 {
		return fmt.Errorf("refusing v3 ledger with %d rows; expected exactly the immutable v1, v2, and v3 rows", len(entries))
	}
	entryByVersion := make(map[string]ledgerEntry, len(entries))
	for _, entry := range entries {
		entryByVersion[entry.version] = entry
	}
	for _, expected := range []struct{ version, checksum string }{
		{BaselineVersion, base.checksum},
		{v2.version, v2.checksum},
		{v3.version, v3.checksum},
	} {
		entry, ok := entryByVersion[expected.version]
		if !ok || entry.checksum != expected.checksum || entry.state != "applied" || !entry.fingerprint.Valid || entry.fingerprint.String == "" {
			return fmt.Errorf("refusing missing, altered, or incomplete %s ledger entry", expected.version)
		}
	}

	v1Fingerprint, err := schemaFingerprintExcludingColumn(ctx, conn, manualReviewV2TableNames(), manualReviewV2TriggerNames(), "admin_users", "auth_version")
	if err != nil {
		return fmt.Errorf("verify original v1 schema fingerprint: %w", err)
	}
	if entryByVersion[BaselineVersion].fingerprint.String != v1Fingerprint {
		return fmt.Errorf("refusing v1 schema drift: stored fingerprint %s, actual %s", entryByVersion[BaselineVersion].fingerprint.String, v1Fingerprint)
	}
	v2Fingerprint, err := schemaFingerprintExcludingColumn(ctx, conn, nil, nil, "admin_users", "auth_version")
	if err != nil {
		return fmt.Errorf("verify original v2 schema fingerprint: %w", err)
	}
	if entryByVersion[v2.version].fingerprint.String != v2Fingerprint {
		return fmt.Errorf("refusing v2 schema drift: stored fingerprint %s, actual %s", entryByVersion[v2.version].fingerprint.String, v2Fingerprint)
	}
	actualFingerprint, err := schemaFingerprint(ctx, conn)
	if err != nil {
		return fmt.Errorf("verify v3 managed schema fingerprint: %w", err)
	}
	if entryByVersion[v3.version].fingerprint.String != actualFingerprint {
		return fmt.Errorf("refusing v3 schema drift: stored fingerprint %s, actual %s", entryByVersion[v3.version].fingerprint.String, actualFingerprint)
	}
	return nil
}

func verifyAdminAuthV3Column(ctx context.Context, conn *sql.Conn) error {
	var count int
	err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='admin_users' AND COLUMN_NAME='auth_version'
		AND COLUMN_TYPE='bigint unsigned' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='1' AND EXTRA=''`).Scan(&count)
	if err != nil {
		return fmt.Errorf("inspect admin_users.auth_version: %w", err)
	}
	if count != 1 {
		return fmt.Errorf("admin_users.auth_version definition mismatch")
	}
	return nil
}
