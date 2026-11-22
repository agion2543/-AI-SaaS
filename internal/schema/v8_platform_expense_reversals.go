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
	PlatformExpenseReversalsV8Version  = "schema-v8-platform-expense-reversals-20261119"
	PlatformExpenseReversalsV8Checksum = "54b9e6454f13a524d816ce96c04fa3cf0a4ef3325c77d1a1845b8c06b431e40f"
)

const platformExpenseReversalsV8TableDDL = `CREATE TABLE ` + "`platform_expense_reversals`" + ` (
	` + "`id`" + ` bigint unsigned NOT NULL AUTO_INCREMENT,
	` + "`created_at`" + ` datetime(3) NOT NULL,
		` + "`updated_at`" + ` datetime(3) NOT NULL,
		` + "`expense_id`" + ` bigint unsigned NOT NULL,
		` + "`amount_cents`" + ` bigint NOT NULL,
		` + "`reversal_date`" + ` date NOT NULL,
	` + "`reason`" + ` varchar(500) COLLATE utf8mb4_unicode_ci NOT NULL,
	` + "`created_by`" + ` bigint unsigned NOT NULL,
	` + "`idempotency_key`" + ` char(36) COLLATE utf8mb4_unicode_ci NOT NULL,
	PRIMARY KEY (` + "`id`" + `),
		KEY ` + "`idx_platform_expense_reversals_expense_id`" + ` (` + "`expense_id`" + `),
	UNIQUE KEY ` + "`uk_platform_expense_reversals_idempotency_key`" + ` (` + "`idempotency_key`" + `),
	KEY ` + "`idx_platform_expense_reversals_date`" + ` (` + "`reversal_date`" + `),
	KEY ` + "`idx_platform_expense_reversals_created_by`" + ` (` + "`created_by`" + `),
		CONSTRAINT ` + "`fk_platform_expense_reversals_expense_id`" + ` FOREIGN KEY (` + "`expense_id`" + `) REFERENCES ` + "`platform_expenses`" + ` (` + "`id`" + `) ON DELETE RESTRICT,
		CONSTRAINT ` + "`chk_platform_expense_reversals_amount_positive`" + ` CHECK (` + "`amount_cents`" + ` > 0),
		CONSTRAINT ` + "`chk_platform_expense_reversals_reason`" + ` CHECK (LENGTH(TRIM(` + "`reason`" + `)) BETWEEN 1 AND 500)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`

type platformExpenseReversalsV8Definition struct {
	version  string
	checksum string
	table    string
	triggers map[string]triggerDefinition
}

func loadPlatformExpenseReversalsV8() (platformExpenseReversalsV8Definition, error) {
	definition := platformExpenseReversalsV8Definition{
		version: PlatformExpenseReversalsV8Version,
		table:   platformExpenseReversalsV8TableDDL,
		triggers: map[string]triggerDefinition{
			"trg_platform_expense_reversals_no_update": {
				name: "trg_platform_expense_reversals_no_update", table: "platform_expense_reversals", timing: "BEFORE", event: "UPDATE",
				body: "SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'platform expense reversals are append-only';",
			},
			"trg_platform_expense_reversals_no_delete": {
				name: "trg_platform_expense_reversals_no_delete", table: "platform_expense_reversals", timing: "BEFORE", event: "DELETE",
				body: "SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'platform expense reversals are append-only';",
			},
		},
	}
	for name, trigger := range definition.triggers {
		trigger.stmt = fmt.Sprintf("CREATE TRIGGER `%s` %s %s ON `%s` FOR EACH ROW BEGIN\n%s\nEND", trigger.name, trigger.timing, trigger.event, trigger.table, trigger.body)
		definition.triggers[name] = trigger
	}
	if err := validatePlatformExpenseReversalsV8Definition(&definition); err != nil {
		return platformExpenseReversalsV8Definition{}, err
	}
	return definition, nil
}

func validatePlatformExpenseReversalsV8Definition(definition *platformExpenseReversalsV8Definition) error {
	if definition.version != PlatformExpenseReversalsV8Version || definition.table == "" || len(definition.triggers) != 2 {
		return errors.New("platform expense reversals v8 definition inventory mismatch")
	}
	for name, trigger := range definition.triggers {
		if trigger.name != name || trigger.table != "platform_expense_reversals" || trigger.timing != "BEFORE" || (trigger.event != "UPDATE" && trigger.event != "DELETE") || !strings.Contains(strings.ToUpper(trigger.body), "SIGNAL SQLSTATE '45000'") || !strings.Contains(trigger.body, "append-only") {
			return fmt.Errorf("platform expense reversals v8 trigger %q is not append-only", name)
		}
	}
	material := []string{definition.version, definition.table}
	triggerNames := make([]string, 0, len(definition.triggers))
	for name := range definition.triggers {
		triggerNames = append(triggerNames, name)
	}
	sort.Strings(triggerNames)
	for _, name := range triggerNames {
		material = append(material, definition.triggers[name].stmt)
	}
	sum := sha256.Sum256([]byte(strings.Join(material, "\n")))
	definition.checksum = fmt.Sprintf("%x", sum)
	if PlatformExpenseReversalsV8Checksum == "TODO" || definition.checksum != PlatformExpenseReversalsV8Checksum {
		return fmt.Errorf("platform expense reversals v8 checksum mismatch: definition %s, expected %s", definition.checksum, PlatformExpenseReversalsV8Checksum)
	}
	return nil
}

// ApplyPlatformExpenseReversalsV8 adds an empty append-only reversal ledger only
// to the exact managed v7 schema. Historical versions and fingerprints remain immutable.
func ApplyPlatformExpenseReversalsV8(ctx context.Context, db *sql.DB) (Result, error) {
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
	if err := verifyManagedSchemaV7(ctx, conn, objects, base, v2, v3, v4, v5, v6, v7); err == nil {
		if err := applyPlatformExpenseReversalsV8(ctx, conn, base, v2, v3, v4, v5, v6, v7, v8); err != nil {
			return "", err
		}
		return ResultApplied, nil
	}
	if err := verifyManagedSchemaV8(ctx, conn, objects, base, v2, v3, v4, v5, v6, v7, v8); err != nil {
		return "", fmt.Errorf("refusing non-exact managed schema for v8 platform expense reversals migration: %w", err)
	}
	return ResultAlready, nil
}

func applyPlatformExpenseReversalsV8(ctx context.Context, conn *sql.Conn, base baselineDefinition, v2 incrementalMigrationDefinition, v3 adminAuthV3Definition, v4 otpSafetyV4Definition, v5 emailOTPSafetyV5Definition, v6 wholesaleRetailV6Definition, v7 platformExpensesV7Definition, v8 platformExpenseReversalsV8Definition) error {
	if _, err := conn.ExecContext(ctx, `INSERT INTO `+ledgerTable+` (version, checksum, state) VALUES (?, ?, 'applying')`, v8.version, v8.checksum); err != nil {
		return fmt.Errorf("record v8 platform expense reversals migration start: %w", err)
	}
	if _, err := conn.ExecContext(ctx, v8.table); err != nil {
		return fmt.Errorf("create v8 platform expense reversals table (v8 ledger remains incomplete): %w", err)
	}
	triggerNames := make([]string, 0, len(v8.triggers))
	for name := range v8.triggers {
		triggerNames = append(triggerNames, name)
	}
	sort.Strings(triggerNames)
	for _, name := range triggerNames {
		if _, err := conn.ExecContext(ctx, v8.triggers[name].stmt); err != nil {
			return fmt.Errorf("create v8 append-only trigger %q (v8 ledger remains incomplete): %w", name, err)
		}
	}
	if err := verifyPlatformExpenseReversalsV8Table(ctx, conn); err != nil {
		return fmt.Errorf("verify v8 platform expense reversals table (v8 ledger remains incomplete): %w", err)
	}
	objects, err := inspectObjects(ctx, conn)
	if err != nil {
		return err
	}
	if err := verifyObjectInventoryV8(objects, base, v2, v7, v8); err != nil {
		return fmt.Errorf("post-apply v8 inventory verification failed (v8 ledger remains incomplete): %w", err)
	}
	if err := verifyPlatformExpenseReversalsV8HistoricalFingerprints(ctx, conn, base, v2, v3, v4, v5, v6, v7, v8); err != nil {
		return fmt.Errorf("verify immutable v1-v7 schema during v8 migration (v8 ledger remains incomplete): %w", err)
	}
	fingerprint, err := schemaFingerprint(ctx, conn)
	if err != nil {
		return fmt.Errorf("compute v8 installed schema fingerprint: %w", err)
	}
	result, err := conn.ExecContext(ctx, `UPDATE `+ledgerTable+`
SET state='applied', schema_fingerprint=?, applied_at=CURRENT_TIMESTAMP
WHERE version=? AND checksum=? AND state='applying'`, fingerprint, v8.version, v8.checksum)
	if err != nil {
		return fmt.Errorf("complete v8 schema version ledger: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("verify v8 schema version ledger update: %w", err)
	}
	if rows != 1 {
		return errors.New("v8 schema version ledger did not transition exactly once; it remains incomplete")
	}
	return nil
}

func verifyManagedSchemaV8(ctx context.Context, conn *sql.Conn, objects schemaObjects, base baselineDefinition, v2 incrementalMigrationDefinition, v3 adminAuthV3Definition, v4 otpSafetyV4Definition, v5 emailOTPSafetyV5Definition, v6 wholesaleRetailV6Definition, v7 platformExpensesV7Definition, v8 platformExpenseReversalsV8Definition) error {
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
	if err := verifyPlatformExpensesV7Table(ctx, conn); err != nil {
		return err
	}
	if err := verifyPlatformExpenseReversalsV8Table(ctx, conn); err != nil {
		return err
	}
	if err := verifyObjectInventoryV8(objects, base, v2, v7, v8); err != nil {
		return err
	}

	rows, err := conn.QueryContext(ctx, "SELECT version, checksum, state, schema_fingerprint FROM "+ledgerTable+" ORDER BY version")
	if err != nil {
		return fmt.Errorf("read v8 schema version ledger: %w", err)
	}
	type ledgerEntry struct {
		version, checksum, state string
		fingerprint              sql.NullString
	}
	entries := make([]ledgerEntry, 0, 8)
	for rows.Next() {
		var entry ledgerEntry
		if err := rows.Scan(&entry.version, &entry.checksum, &entry.state, &entry.fingerprint); err != nil {
			rows.Close()
			return fmt.Errorf("scan v8 schema version ledger: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("read v8 schema version ledger rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close v8 schema version ledger rows: %w", err)
	}
	if len(entries) != 8 {
		return fmt.Errorf("refusing v8 ledger with %d rows; expected exactly the immutable v1-v8 rows", len(entries))
	}
	entryByVersion := make(map[string]ledgerEntry, len(entries))
	for _, entry := range entries {
		entryByVersion[entry.version] = entry
	}
	expected := []struct{ version, checksum string }{
		{BaselineVersion, base.checksum}, {v2.version, v2.checksum}, {v3.version, v3.checksum},
		{v4.version, v4.checksum}, {v5.version, v5.checksum}, {v6.version, v6.checksum},
		{v7.version, v7.checksum}, {v8.version, v8.checksum},
	}
	for _, want := range expected {
		entry, ok := entryByVersion[want.version]
		if !ok || entry.checksum != want.checksum || entry.state != "applied" || !entry.fingerprint.Valid || entry.fingerprint.String == "" {
			return fmt.Errorf("refusing missing, altered, or incomplete %s ledger entry", want.version)
		}
	}
	if err := verifyPlatformExpenseReversalsV8HistoricalFingerprints(ctx, conn, base, v2, v3, v4, v5, v6, v7, v8); err != nil {
		return err
	}
	actualFingerprint, err := schemaFingerprint(ctx, conn)
	if err != nil {
		return fmt.Errorf("verify v8 managed schema fingerprint: %w", err)
	}
	if stored := entryByVersion[v8.version].fingerprint.String; stored != actualFingerprint {
		return fmt.Errorf("refusing v8 schema drift: stored fingerprint %s, actual %s", stored, actualFingerprint)
	}
	return nil
}

func verifyPlatformExpenseReversalsV8Table(ctx context.Context, conn *sql.Conn) error {
	var columns int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='platform_expense_reversals' AND (
(COLUMN_NAME='id' AND COLUMN_TYPE='bigint unsigned' AND IS_NULLABLE='NO' AND EXTRA='auto_increment') OR
(COLUMN_NAME='created_at' AND COLUMN_TYPE='datetime(3)' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT IS NULL) OR
(COLUMN_NAME='updated_at' AND COLUMN_TYPE='datetime(3)' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT IS NULL) OR
	(COLUMN_NAME='expense_id' AND COLUMN_TYPE='bigint unsigned' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT IS NULL) OR
	(COLUMN_NAME='amount_cents' AND COLUMN_TYPE='bigint' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT IS NULL) OR
(COLUMN_NAME='reversal_date' AND COLUMN_TYPE='date' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT IS NULL) OR
(COLUMN_NAME='reason' AND COLUMN_TYPE='varchar(500)' AND IS_NULLABLE='NO' AND COALESCE(COLLATION_NAME,'')='utf8mb4_unicode_ci' AND COLUMN_DEFAULT IS NULL) OR
(COLUMN_NAME='created_by' AND COLUMN_TYPE='bigint unsigned' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT IS NULL) OR
(COLUMN_NAME='idempotency_key' AND COLUMN_TYPE='char(36)' AND IS_NULLABLE='NO' AND COALESCE(COLLATION_NAME,'')='utf8mb4_unicode_ci' AND COLUMN_DEFAULT IS NULL)
)`).Scan(&columns); err != nil {
		return fmt.Errorf("inspect platform_expense_reversals v8 columns: %w", err)
	}
	if columns != 9 {
		return fmt.Errorf("platform_expense_reversals v8 has %d expected columns, want 9", columns)
	}
	var indexes int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(DISTINCT INDEX_NAME) FROM information_schema.STATISTICS
WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='platform_expense_reversals' AND INDEX_NAME IN
	('PRIMARY','idx_platform_expense_reversals_expense_id','uk_platform_expense_reversals_idempotency_key','idx_platform_expense_reversals_date','idx_platform_expense_reversals_created_by')`).Scan(&indexes); err != nil {
		return fmt.Errorf("inspect platform_expense_reversals v8 indexes: %w", err)
	}
	if indexes != 5 {
		return fmt.Errorf("platform_expense_reversals v8 has %d expected indexes, want 5", indexes)
	}
	var checks int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
	WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='platform_expense_reversals' AND CONSTRAINT_TYPE='CHECK'
	AND CONSTRAINT_NAME IN ('chk_platform_expense_reversals_reason','chk_platform_expense_reversals_amount_positive')`).Scan(&checks); err != nil {
		return fmt.Errorf("inspect platform_expense_reversals v8 check constraint: %w", err)
	}
	if checks != 2 {
		return fmt.Errorf("platform_expense_reversals v8 has %d expected check constraints, want 2", checks)
	}
	var checkClause string
	if err := conn.QueryRowContext(ctx, `SELECT cc.CHECK_CLAUSE FROM information_schema.CHECK_CONSTRAINTS AS cc
WHERE cc.CONSTRAINT_SCHEMA=DATABASE() AND cc.CONSTRAINT_NAME='chk_platform_expense_reversals_reason'`).Scan(&checkClause); err != nil {
		return fmt.Errorf("inspect platform_expense_reversals v8 check expression: %w", err)
	}
	if got, want := normalizePlatformExpenseCheck(checkClause), "lengthtrimreasonbetween1and500"; got != want {
		return fmt.Errorf("platform expense reversal reason check drifted: got %q want %q", got, want)
	}
	if err := conn.QueryRowContext(ctx, `SELECT cc.CHECK_CLAUSE FROM information_schema.CHECK_CONSTRAINTS AS cc
	WHERE cc.CONSTRAINT_SCHEMA=DATABASE() AND cc.CONSTRAINT_NAME='chk_platform_expense_reversals_amount_positive'`).Scan(&checkClause); err != nil {
		return fmt.Errorf("inspect platform_expense_reversals v8 positive amount check expression: %w", err)
	}
	if got, want := normalizePlatformExpenseCheck(checkClause), "amount_cents>0"; got != want {
		return fmt.Errorf("platform expense reversal amount check drifted: got %q want %q", got, want)
	}
	var expenseIndexNonUnique, idempotencyIndexUnique int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='platform_expense_reversals' AND INDEX_NAME='idx_platform_expense_reversals_expense_id' AND NON_UNIQUE=1`).Scan(&expenseIndexNonUnique); err != nil {
		return fmt.Errorf("inspect platform_expense_reversals v8 non-unique expense index: %w", err)
	}
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='platform_expense_reversals' AND INDEX_NAME='uk_platform_expense_reversals_idempotency_key' AND NON_UNIQUE=0`).Scan(&idempotencyIndexUnique); err != nil {
		return fmt.Errorf("inspect platform_expense_reversals v8 unique idempotency index: %w", err)
	}
	if expenseIndexNonUnique != 1 || idempotencyIndexUnique != 1 {
		return fmt.Errorf("platform_expense_reversals v8 indexes do not allow multiple linked rows or enforce unique idempotency: expense=%d idempotency=%d", expenseIndexNonUnique, idempotencyIndexUnique)
	}
	var foreignKeys int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.KEY_COLUMN_USAGE
WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='platform_expense_reversals'
AND CONSTRAINT_NAME='fk_platform_expense_reversals_expense_id' AND COLUMN_NAME='expense_id'
AND REFERENCED_TABLE_NAME='platform_expenses' AND REFERENCED_COLUMN_NAME='id'`).Scan(&foreignKeys); err != nil {
		return fmt.Errorf("inspect platform_expense_reversals v8 foreign key: %w", err)
	}
	if foreignKeys != 1 {
		return fmt.Errorf("platform_expense_reversals v8 has %d expected original-expense foreign keys, want 1", foreignKeys)
	}
	var deleteRule string
	if err := conn.QueryRowContext(ctx, `SELECT DELETE_RULE FROM information_schema.REFERENTIAL_CONSTRAINTS
WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='platform_expense_reversals'
AND CONSTRAINT_NAME='fk_platform_expense_reversals_expense_id'`).Scan(&deleteRule); err != nil {
		return fmt.Errorf("inspect platform_expense_reversals v8 foreign-key delete rule: %w", err)
	}
	if deleteRule != "RESTRICT" {
		return fmt.Errorf("platform_expense_reversals v8 foreign-key delete rule is %q, want RESTRICT", deleteRule)
	}
	return nil
}

func verifyObjectInventoryV8(objects schemaObjects, base baselineDefinition, v2 incrementalMigrationDefinition, v7 platformExpensesV7Definition, v8 platformExpenseReversalsV8Definition) error {
	if !containsSchemaName(objects.tables, "platform_expense_reversals") {
		return errors.New("platform_expense_reversals table is missing")
	}
	for name := range v8.triggers {
		if !containsSchemaName(objects.triggers, name) {
			return fmt.Errorf("platform expense reversals trigger %q is missing", name)
		}
	}
	filtered := objects
	filtered.tables = removeSchemaName(objects.tables, "platform_expense_reversals")
	for name := range v8.triggers {
		filtered.triggers = removeSchemaName(filtered.triggers, name)
	}
	if err := verifyObjectInventoryV7(filtered, base, v2, v7); err != nil {
		return err
	}
	return nil
}

func containsSchemaName(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func removeSchemaName(values []string, remove string) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if value != remove {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func verifyPlatformExpenseReversalsV8HistoricalFingerprints(ctx context.Context, conn *sql.Conn, base baselineDefinition, v2 incrementalMigrationDefinition, v3 adminAuthV3Definition, v4 otpSafetyV4Definition, v5 emailOTPSafetyV5Definition, v6 wholesaleRetailV6Definition, v7 platformExpensesV7Definition, v8 platformExpenseReversalsV8Definition) error {
	v7Tables := []string{"platform_expenses"}
	v7Triggers := []string{"trg_platform_expenses_no_delete", "trg_platform_expenses_no_update"}
	v8Tables := []string{"platform_expense_reversals"}
	v8Triggers := []string{"trg_platform_expense_reversals_no_delete", "trg_platform_expense_reversals_no_update"}
	allNewTables := append(append([]string(nil), v7Tables...), v8Tables...)
	allNewTriggers := append(append([]string(nil), v7Triggers...), v8Triggers...)
	if err := verifyWholesaleRetailHistoricalFingerprintsExcluding(ctx, conn, base, v2, v3, v4, v5, allNewTables, allNewTriggers); err != nil {
		return err
	}
	for _, check := range []struct {
		version, checksum, label string
		excludedTables           []string
		excludedTriggers         []string
	}{
		{version: v6.version, checksum: v6.checksum, label: "v6", excludedTables: allNewTables, excludedTriggers: allNewTriggers},
		{version: v7.version, checksum: v7.checksum, label: "v7", excludedTables: v8Tables, excludedTriggers: v8Triggers},
	} {
		actual, err := schemaFingerprintExcluding(ctx, conn, check.excludedTables, check.excludedTriggers)
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
	_ = v8
	return nil
}
