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
	PlatformExpensesV7Version  = "schema-v7-platform-expenses-20261116"
	PlatformExpensesV7Checksum = "33c1ff24ae4a18f13823c28a9fd904d818fe95a57be1cdc6d1320e23add02a9e"
)

const platformExpensesV7TableDDL = `CREATE TABLE ` + "`platform_expenses`" + ` (
	` + "`id`" + ` bigint unsigned NOT NULL AUTO_INCREMENT,
	` + "`created_at`" + ` datetime(3) NOT NULL,
	` + "`updated_at`" + ` datetime(3) NOT NULL,
	` + "`category`" + ` varchar(40) COLLATE utf8mb4_unicode_ci NOT NULL,
	` + "`amount_cents`" + ` bigint NOT NULL,
	` + "`currency`" + ` char(3) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'CNY',
	` + "`expense_date`" + ` date NOT NULL,
	` + "`memo`" + ` varchar(120) COLLATE utf8mb4_unicode_ci NOT NULL,
	` + "`created_by`" + ` bigint unsigned NOT NULL,
	` + "`idempotency_key`" + ` char(36) COLLATE utf8mb4_unicode_ci NOT NULL,
	PRIMARY KEY (` + "`id`" + `),
	UNIQUE KEY ` + "`uk_platform_expenses_idempotency_key`" + ` (` + "`idempotency_key`" + `),
	KEY ` + "`idx_platform_expenses_expense_date`" + ` (` + "`expense_date`" + `),
	KEY ` + "`idx_platform_expenses_category_date`" + ` (` + "`category`" + `, ` + "`expense_date`" + `),
	KEY ` + "`idx_platform_expenses_created_by`" + ` (` + "`created_by`" + `),
	CONSTRAINT ` + "`chk_platform_expenses_category`" + ` CHECK (` + "`category`" + ` IN ('cloud_hosting','messaging_api','labor_contractor','customer_acquisition_travel','equipment','other')),
	CONSTRAINT ` + "`chk_platform_expenses_amount_positive`" + ` CHECK (` + "`amount_cents`" + ` > 0),
	CONSTRAINT ` + "`chk_platform_expenses_currency`" + ` CHECK (` + "`currency`" + ` = 'CNY'),
	CONSTRAINT ` + "`chk_platform_expenses_memo`" + ` CHECK (LENGTH(TRIM(` + "`memo`" + `)) > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`

type platformExpensesV7Definition struct {
	version  string
	checksum string
	table    string
	triggers map[string]triggerDefinition
}

func loadPlatformExpensesV7() (platformExpensesV7Definition, error) {
	definition := platformExpensesV7Definition{
		version: PlatformExpensesV7Version,
		table:   platformExpensesV7TableDDL,
		triggers: map[string]triggerDefinition{
			"trg_platform_expenses_no_update": {
				name: "trg_platform_expenses_no_update", table: "platform_expenses", timing: "BEFORE", event: "UPDATE",
				body: "SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'platform expense entries are append-only';",
			},
			"trg_platform_expenses_no_delete": {
				name: "trg_platform_expenses_no_delete", table: "platform_expenses", timing: "BEFORE", event: "DELETE",
				body: "SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'platform expense entries are append-only';",
			},
		},
	}
	for name, trigger := range definition.triggers {
		trigger.stmt = fmt.Sprintf("CREATE TRIGGER `%s` %s %s ON `%s` FOR EACH ROW BEGIN\n%s\nEND", trigger.name, trigger.timing, trigger.event, trigger.table, trigger.body)
		definition.triggers[name] = trigger
	}
	if err := validatePlatformExpensesV7Definition(&definition); err != nil {
		return platformExpensesV7Definition{}, err
	}
	return definition, nil
}

func validatePlatformExpensesV7Definition(definition *platformExpensesV7Definition) error {
	if definition.version != PlatformExpensesV7Version || definition.table == "" || len(definition.triggers) != 2 {
		return errors.New("platform expenses v7 definition inventory mismatch")
	}
	for name, trigger := range definition.triggers {
		if trigger.name != name || trigger.table != "platform_expenses" || trigger.timing != "BEFORE" || (trigger.event != "UPDATE" && trigger.event != "DELETE") || !strings.Contains(strings.ToUpper(trigger.body), "SIGNAL SQLSTATE '45000'") || !strings.Contains(trigger.body, "append-only") {
			return fmt.Errorf("platform expenses v7 trigger %q is not append-only", name)
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
	if PlatformExpensesV7Checksum == "" || definition.checksum != PlatformExpensesV7Checksum {
		return fmt.Errorf("platform expenses v7 checksum mismatch: definition %s, expected %s", definition.checksum, PlatformExpensesV7Checksum)
	}
	return nil
}

// ApplyPlatformExpensesV7 adds an empty append-only actual-expense ledger only
// to the exact managed v6 schema. Existing versions and fingerprints are never
// rewritten; an interrupted MySQL DDL migration remains fail-closed.
func ApplyPlatformExpensesV7(ctx context.Context, db *sql.DB) (Result, error) {
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
	if err := verifyManagedSchemaV6(ctx, conn, objects, base, v2, v3, v4, v5, v6); err == nil {
		if err := applyPlatformExpensesV7(ctx, conn, base, v2, v3, v4, v5, v6, v7); err != nil {
			return "", err
		}
		return ResultApplied, nil
	}
	if err := verifyManagedSchemaV7(ctx, conn, objects, base, v2, v3, v4, v5, v6, v7); err != nil {
		return "", fmt.Errorf("refusing non-exact managed schema for v7 platform expenses migration: %w", err)
	}
	return ResultAlready, nil
}

func applyPlatformExpensesV7(ctx context.Context, conn *sql.Conn, base baselineDefinition, v2 incrementalMigrationDefinition, v3 adminAuthV3Definition, v4 otpSafetyV4Definition, v5 emailOTPSafetyV5Definition, v6 wholesaleRetailV6Definition, v7 platformExpensesV7Definition) error {
	if _, err := conn.ExecContext(ctx, `INSERT INTO `+ledgerTable+` (version, checksum, state) VALUES (?, ?, 'applying')`, v7.version, v7.checksum); err != nil {
		return fmt.Errorf("record v7 platform expenses migration start: %w", err)
	}
	if _, err := conn.ExecContext(ctx, v7.table); err != nil {
		return fmt.Errorf("create v7 platform expenses table (v7 ledger remains incomplete): %w", err)
	}
	triggerNames := make([]string, 0, len(v7.triggers))
	for name := range v7.triggers {
		triggerNames = append(triggerNames, name)
	}
	sort.Strings(triggerNames)
	for _, name := range triggerNames {
		if _, err := conn.ExecContext(ctx, v7.triggers[name].stmt); err != nil {
			return fmt.Errorf("create v7 append-only trigger %q (v7 ledger remains incomplete): %w", name, err)
		}
	}
	if err := verifyPlatformExpensesV7Table(ctx, conn); err != nil {
		return fmt.Errorf("verify v7 platform expenses table (v7 ledger remains incomplete): %w", err)
	}
	objects, err := inspectObjects(ctx, conn)
	if err != nil {
		return err
	}
	if err := verifyObjectInventoryV7(objects, base, v2, v7); err != nil {
		return fmt.Errorf("post-apply v7 inventory verification failed (v7 ledger remains incomplete): %w", err)
	}
	if err := verifyPlatformExpensesV7HistoricalFingerprints(ctx, conn, base, v2, v3, v4, v5, v6); err != nil {
		return fmt.Errorf("verify immutable v1-v6 schema during v7 migration (v7 ledger remains incomplete): %w", err)
	}
	fingerprint, err := schemaFingerprint(ctx, conn)
	if err != nil {
		return fmt.Errorf("compute v7 installed schema fingerprint: %w", err)
	}
	result, err := conn.ExecContext(ctx, `UPDATE `+ledgerTable+`
SET state='applied', schema_fingerprint=?, applied_at=CURRENT_TIMESTAMP
WHERE version=? AND checksum=? AND state='applying'`, fingerprint, v7.version, v7.checksum)
	if err != nil {
		return fmt.Errorf("complete v7 schema version ledger: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("verify v7 schema version ledger update: %w", err)
	}
	if rows != 1 {
		return errors.New("v7 schema version ledger did not transition exactly once; it remains incomplete")
	}
	return nil
}

func verifyManagedSchemaV7(ctx context.Context, conn *sql.Conn, objects schemaObjects, base baselineDefinition, v2 incrementalMigrationDefinition, v3 adminAuthV3Definition, v4 otpSafetyV4Definition, v5 emailOTPSafetyV5Definition, v6 wholesaleRetailV6Definition, v7 platformExpensesV7Definition) error {
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
	if err := verifyObjectInventoryV7(objects, base, v2, v7); err != nil {
		return err
	}

	rows, err := conn.QueryContext(ctx, "SELECT version, checksum, state, schema_fingerprint FROM "+ledgerTable+" ORDER BY version")
	if err != nil {
		return fmt.Errorf("read v7 schema version ledger: %w", err)
	}
	type ledgerEntry struct {
		version, checksum, state string
		fingerprint              sql.NullString
	}
	entries := make([]ledgerEntry, 0, 7)
	for rows.Next() {
		var entry ledgerEntry
		if err := rows.Scan(&entry.version, &entry.checksum, &entry.state, &entry.fingerprint); err != nil {
			rows.Close()
			return fmt.Errorf("scan v7 schema version ledger: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("read v7 schema version ledger rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close v7 schema version ledger rows: %w", err)
	}
	if len(entries) != 7 {
		return fmt.Errorf("refusing v7 ledger with %d rows; expected exactly the immutable v1-v7 rows", len(entries))
	}
	entryByVersion := make(map[string]ledgerEntry, len(entries))
	for _, entry := range entries {
		entryByVersion[entry.version] = entry
	}
	expected := []struct{ version, checksum string }{
		{BaselineVersion, base.checksum}, {v2.version, v2.checksum}, {v3.version, v3.checksum},
		{v4.version, v4.checksum}, {v5.version, v5.checksum}, {v6.version, v6.checksum}, {v7.version, v7.checksum},
	}
	for _, want := range expected {
		entry, ok := entryByVersion[want.version]
		if !ok || entry.checksum != want.checksum || entry.state != "applied" || !entry.fingerprint.Valid || entry.fingerprint.String == "" {
			return fmt.Errorf("refusing missing, altered, or incomplete %s ledger entry", want.version)
		}
	}
	if err := verifyPlatformExpensesV7HistoricalFingerprints(ctx, conn, base, v2, v3, v4, v5, v6); err != nil {
		return err
	}
	actualFingerprint, err := schemaFingerprint(ctx, conn)
	if err != nil {
		return fmt.Errorf("verify v7 managed schema fingerprint: %w", err)
	}
	if stored := entryByVersion[v7.version].fingerprint.String; stored != actualFingerprint {
		return fmt.Errorf("refusing v7 schema drift: stored fingerprint %s, actual %s", stored, actualFingerprint)
	}
	return nil
}

func verifyPlatformExpensesV7Table(ctx context.Context, conn *sql.Conn) error {
	var columns int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='platform_expenses' AND (
(COLUMN_NAME='id' AND COLUMN_TYPE='bigint unsigned' AND IS_NULLABLE='NO' AND EXTRA='auto_increment') OR
(COLUMN_NAME='created_at' AND COLUMN_TYPE='datetime(3)' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT IS NULL) OR
(COLUMN_NAME='updated_at' AND COLUMN_TYPE='datetime(3)' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT IS NULL) OR
(COLUMN_NAME='category' AND COLUMN_TYPE='varchar(40)' AND IS_NULLABLE='NO' AND COALESCE(COLLATION_NAME,'')='utf8mb4_unicode_ci' AND COLUMN_DEFAULT IS NULL) OR
(COLUMN_NAME='amount_cents' AND COLUMN_TYPE='bigint' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT IS NULL) OR
(COLUMN_NAME='currency' AND COLUMN_TYPE='char(3)' AND IS_NULLABLE='NO' AND COALESCE(COLLATION_NAME,'')='utf8mb4_unicode_ci' AND COLUMN_DEFAULT='CNY') OR
(COLUMN_NAME='expense_date' AND COLUMN_TYPE='date' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT IS NULL) OR
(COLUMN_NAME='memo' AND COLUMN_TYPE='varchar(120)' AND IS_NULLABLE='NO' AND COALESCE(COLLATION_NAME,'')='utf8mb4_unicode_ci' AND COLUMN_DEFAULT IS NULL) OR
(COLUMN_NAME='created_by' AND COLUMN_TYPE='bigint unsigned' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT IS NULL) OR
(COLUMN_NAME='idempotency_key' AND COLUMN_TYPE='char(36)' AND IS_NULLABLE='NO' AND COALESCE(COLLATION_NAME,'')='utf8mb4_unicode_ci' AND COLUMN_DEFAULT IS NULL)
)`).Scan(&columns); err != nil {
		return fmt.Errorf("inspect platform_expenses v7 columns: %w", err)
	}
	if columns != 10 {
		return fmt.Errorf("platform_expenses v7 has %d expected columns, want 10", columns)
	}
	var indexes int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(DISTINCT INDEX_NAME) FROM information_schema.STATISTICS
WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='platform_expenses' AND INDEX_NAME IN
('PRIMARY','uk_platform_expenses_idempotency_key','idx_platform_expenses_expense_date','idx_platform_expenses_category_date','idx_platform_expenses_created_by')`).Scan(&indexes); err != nil {
		return fmt.Errorf("inspect platform_expenses v7 indexes: %w", err)
	}
	if indexes != 5 {
		return fmt.Errorf("platform_expenses v7 has %d expected indexes, want 5", indexes)
	}
	var checks int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='platform_expenses' AND CONSTRAINT_TYPE='CHECK'
AND CONSTRAINT_NAME IN ('chk_platform_expenses_category','chk_platform_expenses_amount_positive','chk_platform_expenses_currency','chk_platform_expenses_memo')`).Scan(&checks); err != nil {
		return fmt.Errorf("inspect platform_expenses v7 check constraints: %w", err)
	}
	if checks != 4 {
		return fmt.Errorf("platform_expenses v7 has %d expected check constraints, want 4", checks)
	}
	wantChecks := map[string]string{
		"chk_platform_expenses_category":        "categoryin'cloud_hosting','messaging_api','labor_contractor','customer_acquisition_travel','equipment','other'",
		"chk_platform_expenses_amount_positive": "amount_cents>0",
		"chk_platform_expenses_currency":        "currency='cny'",
		"chk_platform_expenses_memo":            "lengthtrimmemo>0",
	}
	rows, err := conn.QueryContext(ctx, `SELECT tc.CONSTRAINT_NAME, cc.CHECK_CLAUSE
FROM information_schema.TABLE_CONSTRAINTS AS tc
JOIN information_schema.CHECK_CONSTRAINTS AS cc
ON cc.CONSTRAINT_SCHEMA=tc.CONSTRAINT_SCHEMA AND cc.CONSTRAINT_NAME=tc.CONSTRAINT_NAME
WHERE tc.CONSTRAINT_SCHEMA=DATABASE() AND tc.TABLE_NAME='platform_expenses' AND tc.CONSTRAINT_TYPE='CHECK'`)
	if err != nil {
		return fmt.Errorf("inspect platform_expenses v7 check expressions: %w", err)
	}
	defer rows.Close()
	seenChecks := make(map[string]struct{}, len(wantChecks))
	for rows.Next() {
		var name, clause string
		if err := rows.Scan(&name, &clause); err != nil {
			return fmt.Errorf("read platform_expenses v7 check expression: %w", err)
		}
		want, ok := wantChecks[name]
		got := normalizePlatformExpenseCheck(clause)
		if !ok || got != want {
			return fmt.Errorf("platform_expenses v7 check constraint %q expression drifted: got %q want %q", name, got, want)
		}
		seenChecks[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate platform_expenses v7 check expressions: %w", err)
	}
	if len(seenChecks) != len(wantChecks) {
		return fmt.Errorf("platform_expenses v7 has %d verified check expressions, want %d", len(seenChecks), len(wantChecks))
	}
	return nil
}

func normalizePlatformExpenseCheck(clause string) string {
	clause = strings.ToLower(strings.ReplaceAll(clause, "\\'", "'"))
	var withoutCharsetIntroducers strings.Builder
	inString := false
	for i := 0; i < len(clause); {
		if clause[i] == '\'' {
			inString = !inString
			withoutCharsetIntroducers.WriteByte(clause[i])
			i++
			continue
		}
		if !inString && clause[i] == '_' {
			j := i + 1
			for j < len(clause) && ((clause[j] >= 'a' && clause[j] <= 'z') || (clause[j] >= '0' && clause[j] <= '9')) {
				j++
			}
			if j < len(clause) && clause[j] == '\\' && j+1 < len(clause) && clause[j+1] == '\'' {
				j++
			}
			if j > i+1 && j < len(clause) && clause[j] == '\'' {
				i = j
				continue
			}
		}
		withoutCharsetIntroducers.WriteByte(clause[i])
		i++
	}
	normalizer := strings.NewReplacer("`", "", " ", "", "\t", "", "\r", "", "\n", "", "(", "", ")", "")
	return normalizer.Replace(withoutCharsetIntroducers.String())
}

func verifyObjectInventoryV7(objects schemaObjects, base baselineDefinition, v2 incrementalMigrationDefinition, v7 platformExpensesV7Definition) error {
	wantTables := make([]string, 0, len(base.tables)+len(v2.tables)+2)
	for name := range base.tables {
		wantTables = append(wantTables, name)
	}
	for name := range v2.tables {
		wantTables = append(wantTables, name)
	}
	wantTables = append(wantTables, "platform_expenses", ledgerTable)
	sort.Strings(wantTables)
	if !equalStringSets(objects.tables, wantTables) {
		return fmt.Errorf("table inventory mismatch: found %v, expected %v", objects.tables, wantTables)
	}
	wantTriggers := make([]string, 0, len(base.triggers)+len(v2.triggers)+len(v7.triggers))
	for name := range base.triggers {
		wantTriggers = append(wantTriggers, name)
	}
	for name := range v2.triggers {
		wantTriggers = append(wantTriggers, name)
	}
	for name := range v7.triggers {
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

func verifyPlatformExpensesV7HistoricalFingerprints(ctx context.Context, conn *sql.Conn, base baselineDefinition, v2 incrementalMigrationDefinition, v3 adminAuthV3Definition, v4 otpSafetyV4Definition, v5 emailOTPSafetyV5Definition, v6 wholesaleRetailV6Definition) error {
	v7Tables := []string{"platform_expenses"}
	v7Triggers := []string{"trg_platform_expenses_no_delete", "trg_platform_expenses_no_update"}
	if err := verifyWholesaleRetailHistoricalFingerprintsExcluding(ctx, conn, base, v2, v3, v4, v5, v7Tables, v7Triggers); err != nil {
		return err
	}
	actualV6Fingerprint, err := schemaFingerprintExcluding(ctx, conn, v7Tables, v7Triggers)
	if err != nil {
		return fmt.Errorf("compute immutable v6 schema fingerprint: %w", err)
	}
	var storedV6Fingerprint string
	if err := conn.QueryRowContext(ctx, "SELECT schema_fingerprint FROM "+ledgerTable+" WHERE version=? AND checksum=? AND state='applied'", v6.version, v6.checksum).Scan(&storedV6Fingerprint); err != nil {
		return fmt.Errorf("read immutable v6 fingerprint: %w", err)
	}
	if storedV6Fingerprint != actualV6Fingerprint {
		return fmt.Errorf("refusing v6 schema drift: stored fingerprint %s, actual %s", storedV6Fingerprint, actualV6Fingerprint)
	}
	return nil
}
