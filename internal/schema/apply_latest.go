package schema

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ApplyLatest installs v1 into a genuinely empty schema, then appends the
// subscription manual-review v2, administrator auth-version v3, phone OTP
// safety v4, email OTP safety v5, and light wholesale/retail v6 migrations. Existing schemas are accepted
// only when they are exact managed versions; legacy and unknown databases stay
// fail-closed.
func ApplyLatest(ctx context.Context, db *sql.DB) (Result, error) {
	var err error
	v1Result, v1Err := ApplyCandidate(ctx, db)
	if v1Err != nil {
		// First accept or advance the latest exact version. If this fails, probe
		// earlier exact versions in reverse order; every probe is read-only until
		// its own source version has been fully verified.
		v6Result, v6Err := ApplyWholesaleRetailV6(ctx, db)
		if v6Err == nil {
			return v6Result, nil
		}
		v5Result, v5Err := ApplyEmailOTPSafetyV5(ctx, db)
		if v5Err == nil {
			v6Result, err := ApplyWholesaleRetailV6(ctx, db)
			if err != nil {
				return "", err
			}
			if v5Result == ResultApplied || v6Result == ResultApplied {
				return ResultApplied, nil
			}
			return ResultAlready, nil
		}
		v4Result, v4Err := ApplyOTPSafetyV4(ctx, db)
		if v4Err == nil {
			v5Result, err = ApplyEmailOTPSafetyV5(ctx, db)
			if err != nil {
				return "", err
			}
			v6Result, err = ApplyWholesaleRetailV6(ctx, db)
			if err != nil {
				return "", err
			}
			if v4Result == ResultApplied || v5Result == ResultApplied || v6Result == ResultApplied {
				return ResultApplied, nil
			}
			return ResultAlready, nil
		}
		if db != nil {
			var laterLedgerRows int64
			if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM app_schema_migrations WHERE version IN (?, ?, ?, ?)`, AdminAuthV3Version, OTPSafetyV4Version, EmailOTPSafetyV5Version, WholesaleRetailV6Version).Scan(&laterLedgerRows); err == nil && laterLedgerRows > 0 {
				return "", v6Err
			}
		}
		v2Result, err := ApplyManualReviewV2(ctx, db)
		if err != nil {
			return "", err
		}
		v3Result, err := ApplyAdminAuthVersionV3(ctx, db)
		if err != nil {
			return "", err
		}
		v4Result, err = ApplyOTPSafetyV4(ctx, db)
		if err != nil {
			return "", err
		}
		v5Result, err = ApplyEmailOTPSafetyV5(ctx, db)
		if err != nil {
			return "", err
		}
		v6Result, err = ApplyWholesaleRetailV6(ctx, db)
		if err != nil {
			return "", err
		}
		if v2Result == ResultApplied || v3Result == ResultApplied || v4Result == ResultApplied || v5Result == ResultApplied || v6Result == ResultApplied {
			return ResultApplied, nil
		}
		return ResultAlready, nil
	}

	v2Result, err := ApplyManualReviewV2(ctx, db)
	if err != nil {
		return "", err
	}
	v3Result, err := ApplyAdminAuthVersionV3(ctx, db)
	if err != nil {
		return "", err
	}
	v4Result, err := ApplyOTPSafetyV4(ctx, db)
	if err != nil {
		return "", err
	}
	v5Result, err := ApplyEmailOTPSafetyV5(ctx, db)
	if err != nil {
		return "", err
	}
	v6Result, err := ApplyWholesaleRetailV6(ctx, db)
	if err != nil {
		return "", err
	}
	if v1Result == ResultApplied || v2Result == ResultApplied || v3Result == ResultApplied || v4Result == ResultApplied || v5Result == ResultApplied || v6Result == ResultApplied {
		return ResultApplied, nil
	}
	return ResultAlready, nil
}

// ApplyManualReviewV2 adds the narrowly scoped append-only historical
// subscription review record and audit tables to a checksum-pinned managed v1.
func ApplyManualReviewV2(ctx context.Context, db *sql.DB) (Result, error) {
	base, err := loadBaseline()
	if err != nil {
		return "", err
	}
	migration, err := loadManualReviewV2()
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
	if verifyObjectInventory(objects, base) == nil {
		if err := verifyManagedSchema(ctx, conn, objects, base); err != nil {
			return "", fmt.Errorf("refusing v2 migration from an invalid v1 database: %w", err)
		}
		if err := applyManualReviewV2(ctx, conn, base, migration); err != nil {
			return "", err
		}
		return ResultApplied, nil
	}
	if err := verifyManagedSchemaV2(ctx, conn, objects, base, migration); err != nil {
		return "", fmt.Errorf("refusing non-exact managed schema for v2 migration: %w", err)
	}
	return ResultAlready, nil
}

func applyManualReviewV2(ctx context.Context, conn *sql.Conn, base baselineDefinition, migration incrementalMigrationDefinition) error {
	if _, err := conn.ExecContext(ctx, `INSERT INTO `+ledgerTable+` (version, checksum, state) VALUES (?, ?, 'applying')`, migration.version, migration.checksum); err != nil {
		return fmt.Errorf("record v2 subscription manual review migration start: %w", err)
	}
	// The audit table has a restrictive FK to the immutable review record, so
	// create the parent first rather than relying on lexical map ordering.
	tableNames := []string{"merchant_subscription_manual_reviews", "merchant_subscription_manual_review_audits"}
	for _, name := range tableNames {
		if _, err := conn.ExecContext(ctx, migration.tables[name]); err != nil {
			return fmt.Errorf("apply v2 subscription manual review table %q (v2 ledger remains incomplete): %w", name, err)
		}
	}
	triggerNames := make([]string, 0, len(migration.triggers))
	for name := range migration.triggers {
		triggerNames = append(triggerNames, name)
	}
	sort.Strings(triggerNames)
	for _, name := range triggerNames {
		if _, err := conn.ExecContext(ctx, migration.triggers[name].stmt); err != nil {
			return fmt.Errorf("apply v2 subscription manual review trigger %q (v2 ledger remains incomplete): %w", name, err)
		}
	}
	objects, err := inspectObjects(ctx, conn)
	if err != nil {
		return err
	}
	if err := verifyObjectInventoryV2(objects, base, migration); err != nil {
		return fmt.Errorf("post-apply v2 inventory verification failed (v2 ledger remains incomplete): %w", err)
	}
	v1Fingerprint, err := schemaFingerprintExcluding(ctx, conn, manualReviewV2TableNames(), manualReviewV2TriggerNames())
	if err != nil {
		return fmt.Errorf("verify v1 schema remained unchanged during v2 migration: %w", err)
	}
	var storedV1Fingerprint string
	if err := conn.QueryRowContext(ctx, "SELECT schema_fingerprint FROM "+ledgerTable+" WHERE version=? AND checksum=? AND state='applied'", BaselineVersion, base.checksum).Scan(&storedV1Fingerprint); err != nil {
		return fmt.Errorf("read immutable v1 fingerprint after v2 migration: %w", err)
	}
	if storedV1Fingerprint != v1Fingerprint {
		return fmt.Errorf("refusing v2 completion because the exact v1 schema fingerprint changed: stored %s, actual %s", storedV1Fingerprint, v1Fingerprint)
	}
	fingerprint, err := schemaFingerprint(ctx, conn)
	if err != nil {
		return fmt.Errorf("compute v2 installed schema fingerprint: %w", err)
	}
	result, err := conn.ExecContext(ctx, `UPDATE `+ledgerTable+`
		SET state='applied', schema_fingerprint=?, applied_at=CURRENT_TIMESTAMP
		WHERE version=? AND checksum=? AND state='applying'`, fingerprint, migration.version, migration.checksum)
	if err != nil {
		return fmt.Errorf("complete v2 schema version ledger: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("verify v2 schema version ledger update: %w", err)
	}
	if rows != 1 {
		return errors.New("v2 schema version ledger did not transition exactly once; it remains incomplete")
	}
	return nil
}

func verifyManagedSchemaV2(ctx context.Context, conn *sql.Conn, objects schemaObjects, base baselineDefinition, migration incrementalMigrationDefinition) error {
	if err := verifyObjectInventoryV2(objects, base, migration); err != nil {
		return err
	}
	rows, err := conn.QueryContext(ctx, "SELECT version, checksum, state, schema_fingerprint FROM "+ledgerTable+" ORDER BY version")
	if err != nil {
		return fmt.Errorf("read v2 schema version ledger: %w", err)
	}
	defer rows.Close()
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
			return fmt.Errorf("scan v2 schema version ledger: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read v2 schema version ledger rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close v2 schema version ledger rows: %w", err)
	}
	if len(entries) != 2 {
		return fmt.Errorf("refusing v2 ledger with %d rows; expected exactly the immutable v1 and v2 rows", len(entries))
	}
	entryByVersion := make(map[string]ledgerEntry, len(entries))
	for _, entry := range entries {
		entryByVersion[entry.version] = entry
	}
	v1, ok := entryByVersion[BaselineVersion]
	if !ok || v1.checksum != base.checksum || v1.state != "applied" || !v1.fingerprint.Valid || v1.fingerprint.String == "" {
		return errors.New("refusing missing, altered, or incomplete immutable v1 ledger entry")
	}
	v2, ok := entryByVersion[migration.version]
	if !ok || v2.checksum != migration.checksum || v2.state != "applied" || !v2.fingerprint.Valid || v2.fingerprint.String == "" {
		return errors.New("refusing missing, altered, or incomplete v2 ledger entry")
	}
	v1Fingerprint, err := schemaFingerprintExcluding(ctx, conn, manualReviewV2TableNames(), manualReviewV2TriggerNames())
	if err != nil {
		return fmt.Errorf("verify original v1 schema fingerprint: %w", err)
	}
	if v1.fingerprint.String != v1Fingerprint {
		return fmt.Errorf("refusing v1 schema drift: stored fingerprint %s, actual %s", v1.fingerprint.String, v1Fingerprint)
	}
	actualFingerprint, err := schemaFingerprint(ctx, conn)
	if err != nil {
		return fmt.Errorf("verify v2 managed schema fingerprint: %w", err)
	}
	if v2.fingerprint.String != actualFingerprint {
		return fmt.Errorf("refusing v2 schema drift: stored fingerprint %s, actual %s", v2.fingerprint.String, actualFingerprint)
	}
	return nil
}

func verifyObjectInventoryV2(objects schemaObjects, base baselineDefinition, migration incrementalMigrationDefinition) error {
	wantTables := make([]string, 0, len(base.tables)+len(migration.tables)+1)
	for name := range base.tables {
		wantTables = append(wantTables, name)
	}
	for name := range migration.tables {
		wantTables = append(wantTables, name)
	}
	wantTables = append(wantTables, ledgerTable)
	sort.Strings(wantTables)
	if !equalStringSets(objects.tables, wantTables) {
		return fmt.Errorf("table inventory mismatch: found %v, expected %v", objects.tables, wantTables)
	}
	wantTriggers := make([]string, 0, len(base.triggers)+len(migration.triggers))
	for name := range base.triggers {
		wantTriggers = append(wantTriggers, name)
	}
	for name := range migration.triggers {
		wantTriggers = append(wantTriggers, name)
	}
	sort.Strings(wantTriggers)
	if !equalStringSets(objects.triggers, wantTriggers) {
		return fmt.Errorf("trigger inventory mismatch: found %v, expected %v", objects.triggers, wantTriggers)
	}
	if len(objects.views) != 0 || len(objects.routines) != 0 || len(objects.events) != 0 {
		return fmt.Errorf("unexpected views, routines, or events found (views=%v routines=%v events=%v)", objects.views, objects.routines, objects.events)
	}
	return nil
}

func manualReviewV2TableNames() []string {
	return []string{"merchant_subscription_manual_review_audits", "merchant_subscription_manual_reviews"}
}

func equalStringSets(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	leftCopy := append([]string(nil), left...)
	rightCopy := append([]string(nil), right...)
	sort.Strings(leftCopy)
	sort.Strings(rightCopy)
	return equalStrings(leftCopy, rightCopy)
}

func manualReviewV2TriggerNames() []string {
	return []string{
		"trg_subscription_manual_review_audits_no_delete",
		"trg_subscription_manual_review_audits_no_update",
		"trg_subscription_manual_reviews_no_delete",
		"trg_subscription_manual_reviews_no_update",
	}
}
