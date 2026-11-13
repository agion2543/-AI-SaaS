package schema

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
	"time"
)

type Result string

const (
	ResultApplied Result = "applied"
	ResultAlready Result = "already-managed"
)

type schemaObjects struct {
	tables   []string
	views    []string
	triggers []string
	routines []string
	events   []string
}

// ApplyCandidate initializes only an empty MySQL schema from the embedded,
// checksum-pinned candidate. A completed matching ledger makes repeat calls a
// verified no-op. MySQL DDL is not transactional; an interrupted apply is left
// visibly incomplete and is deliberately refused on the next invocation.
func ApplyCandidate(ctx context.Context, db *sql.DB) (Result, error) {
	definition, err := loadBaseline()
	if err != nil {
		return "", err
	}
	if db == nil {
		return "", fmt.Errorf("database handle is required")
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
		return "", fmt.Errorf("MYSQL_DSN must select an existing target database")
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
	if isEmpty(objects) {
		if err := initializeEmptySchema(ctx, conn, definition); err != nil {
			return "", err
		}
		return ResultApplied, nil
	}
	if err := verifyManagedSchema(ctx, conn, objects, definition); err != nil {
		return "", err
	}
	return ResultAlready, nil
}

func initializeEmptySchema(ctx context.Context, conn *sql.Conn, definition baselineDefinition) error {
	_, err := conn.ExecContext(ctx, `CREATE TABLE `+ledgerTable+` (
		version VARCHAR(128) NOT NULL PRIMARY KEY,
		checksum CHAR(64) NOT NULL,
		state VARCHAR(20) NOT NULL,
		schema_fingerprint CHAR(64) NULL,
		started_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		applied_at TIMESTAMP NULL DEFAULT NULL
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`)
	if err != nil {
		return fmt.Errorf("create schema version ledger: %w", err)
	}
	if _, err := conn.ExecContext(ctx, "INSERT INTO "+ledgerTable+" (version, checksum, state) VALUES (?, ?, 'applying')", BaselineVersion, definition.checksum); err != nil {
		return fmt.Errorf("record baseline apply start: %w", err)
	}

	if _, err := conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=0"); err != nil {
		return fmt.Errorf("temporarily disable foreign key checks: %w", err)
	}
	foreignKeysDisabled := true
	defer func() {
		if foreignKeysDisabled {
			_, _ = conn.ExecContext(context.Background(), "SET FOREIGN_KEY_CHECKS=1")
		}
	}()

	tableNames := make([]string, 0, len(definition.tables))
	for name := range definition.tables {
		tableNames = append(tableNames, name)
	}
	sort.Strings(tableNames)
	for _, name := range tableNames {
		if _, err := conn.ExecContext(ctx, definition.tables[name]); err != nil {
			return fmt.Errorf("apply candidate table %q (ledger remains incomplete): %w", name, err)
		}
	}
	triggerNames := make([]string, 0, len(definition.triggers))
	for name := range definition.triggers {
		triggerNames = append(triggerNames, name)
	}
	sort.Strings(triggerNames)
	for _, name := range triggerNames {
		if _, err := conn.ExecContext(ctx, definition.triggers[name].stmt); err != nil {
			return fmt.Errorf("apply candidate trigger %q (ledger remains incomplete): %w", name, err)
		}
	}
	if _, err := conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=1"); err != nil {
		return fmt.Errorf("restore foreign key checks: %w", err)
	}
	foreignKeysDisabled = false

	objects, err := inspectObjects(ctx, conn)
	if err != nil {
		return err
	}
	if err := verifyObjectInventory(objects, definition); err != nil {
		return fmt.Errorf("post-apply verification failed (ledger remains incomplete): %w", err)
	}
	fingerprint, err := schemaFingerprint(ctx, conn)
	if err != nil {
		return fmt.Errorf("compute installed schema fingerprint: %w", err)
	}
	result, err := conn.ExecContext(ctx, `UPDATE `+ledgerTable+`
		SET state='applied', schema_fingerprint=?, applied_at=CURRENT_TIMESTAMP
		WHERE version=? AND checksum=? AND state='applying'`, fingerprint, BaselineVersion, definition.checksum)
	if err != nil {
		return fmt.Errorf("complete schema version ledger: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("verify schema version ledger update: %w", err)
	}
	if rows != 1 {
		return fmt.Errorf("schema version ledger did not transition exactly once; it remains incomplete")
	}
	return nil
}

func verifyManagedSchema(ctx context.Context, conn *sql.Conn, objects schemaObjects, definition baselineDefinition) error {
	if err := verifyObjectInventory(objects, definition); err != nil {
		return fmt.Errorf("refusing non-empty database: %w", err)
	}

	var rowCount int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+ledgerTable).Scan(&rowCount); err != nil {
		return fmt.Errorf("refusing database with unreadable schema version ledger: %w", err)
	}
	if rowCount != 1 {
		return fmt.Errorf("refusing incomplete or unknown schema version ledger: found %d rows, expected exactly 1", rowCount)
	}
	var version, checksum, state string
	var storedFingerprint sql.NullString
	if err := conn.QueryRowContext(ctx, "SELECT version, checksum, state, schema_fingerprint FROM "+ledgerTable+" LIMIT 1").Scan(&version, &checksum, &state, &storedFingerprint); err != nil {
		return fmt.Errorf("refusing unreadable schema version ledger: %w", err)
	}
	if version != BaselineVersion {
		return fmt.Errorf("refusing unknown schema version %q", version)
	}
	if checksum != definition.checksum {
		return fmt.Errorf("refusing schema baseline checksum mismatch: ledger has %s, expected %s", checksum, definition.checksum)
	}
	if state != "applied" || !storedFingerprint.Valid || storedFingerprint.String == "" {
		return fmt.Errorf("refusing incomplete schema migration state %q", state)
	}
	actualFingerprint, err := schemaFingerprint(ctx, conn)
	if err != nil {
		return fmt.Errorf("verify managed schema fingerprint: %w", err)
	}
	if storedFingerprint.String != actualFingerprint {
		return fmt.Errorf("refusing schema drift: stored fingerprint %s, actual %s", storedFingerprint.String, actualFingerprint)
	}
	return nil
}

func verifyObjectInventory(objects schemaObjects, definition baselineDefinition) error {
	wantTables := make([]string, 0, len(definition.tables)+1)
	for table := range definition.tables {
		wantTables = append(wantTables, table)
	}
	wantTables = append(wantTables, ledgerTable)
	sort.Strings(wantTables)
	if !equalStrings(objects.tables, wantTables) {
		return fmt.Errorf("table inventory mismatch: found %v, expected %v", objects.tables, wantTables)
	}
	wantTriggers := make([]string, 0, len(definition.triggers))
	for trigger := range definition.triggers {
		wantTriggers = append(wantTriggers, trigger)
	}
	sort.Strings(wantTriggers)
	if !equalStrings(objects.triggers, wantTriggers) {
		return fmt.Errorf("trigger inventory mismatch: found %v, expected %v", objects.triggers, wantTriggers)
	}
	if len(objects.views) != 0 || len(objects.routines) != 0 || len(objects.events) != 0 {
		return fmt.Errorf("unexpected views, routines, or events found (views=%v routines=%v events=%v)", objects.views, objects.routines, objects.events)
	}
	return nil
}

func inspectObjects(ctx context.Context, conn *sql.Conn) (schemaObjects, error) {
	var objects schemaObjects
	rows, err := conn.QueryContext(ctx, `SELECT TABLE_NAME, TABLE_TYPE FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() ORDER BY TABLE_NAME`)
	if err != nil {
		return objects, fmt.Errorf("inspect schema tables and views: %w", err)
	}
	for rows.Next() {
		var name, kind string
		if err := rows.Scan(&name, &kind); err != nil {
			rows.Close()
			return objects, fmt.Errorf("read schema object inventory: %w", err)
		}
		switch strings.ToUpper(kind) {
		case "BASE TABLE":
			objects.tables = append(objects.tables, name)
		case "VIEW":
			objects.views = append(objects.views, name)
		default:
			rows.Close()
			return objects, fmt.Errorf("unknown table object type %q for %q", kind, name)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return objects, fmt.Errorf("read schema object inventory: %w", err)
	}
	rows.Close()

	if err := queryNames(ctx, conn, &objects.triggers, `SELECT TRIGGER_NAME FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() ORDER BY TRIGGER_NAME`); err != nil {
		return objects, fmt.Errorf("inspect schema triggers: %w", err)
	}
	if err := queryNames(ctx, conn, &objects.routines, `SELECT ROUTINE_NAME FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA=DATABASE() ORDER BY ROUTINE_NAME`); err != nil {
		return objects, fmt.Errorf("inspect schema routines: %w", err)
	}
	if err := queryNames(ctx, conn, &objects.events, `SELECT EVENT_NAME FROM information_schema.EVENTS WHERE EVENT_SCHEMA=DATABASE() ORDER BY EVENT_NAME`); err != nil {
		return objects, fmt.Errorf("inspect schema events: %w", err)
	}
	return objects, nil
}

func queryNames(ctx context.Context, conn *sql.Conn, target *[]string, query string) error {
	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		*target = append(*target, name)
	}
	return rows.Err()
}

func isEmpty(objects schemaObjects) bool {
	return len(objects.tables) == 0 && len(objects.views) == 0 && len(objects.triggers) == 0 && len(objects.routines) == 0 && len(objects.events) == 0
}

func schemaFingerprint(ctx context.Context, conn *sql.Conn) (string, error) {
	return schemaFingerprintExcluding(ctx, conn, nil, nil)
}

func schemaFingerprintExcluding(ctx context.Context, conn *sql.Conn, excludedTables, excludedTriggers []string) (string, error) {
	h := sha256.New()
	tableFilter := " AND TABLE_NAME<>?"
	tableArgs := []any{ledgerTable}
	constraintFilter := " AND tc.TABLE_NAME<>?"
	constraintArgs := []any{ledgerTable}
	for _, table := range excludedTables {
		tableFilter += " AND TABLE_NAME<>?"
		tableArgs = append(tableArgs, table)
		constraintFilter += " AND tc.TABLE_NAME<>?"
		constraintArgs = append(constraintArgs, table)
	}
	triggerFilter := ""
	triggerArgs := make([]any, 0, len(excludedTriggers))
	for _, trigger := range excludedTriggers {
		triggerFilter += " AND TRIGGER_NAME<>?"
		triggerArgs = append(triggerArgs, trigger)
	}
	queries := []struct {
		label string
		query string
		args  []any
	}{
		{"tables", fmt.Sprintf(`SELECT TABLE_NAME, COALESCE(ENGINE, '<NULL>'), COALESCE(TABLE_COLLATION, '<NULL>') FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_TYPE='BASE TABLE'%s ORDER BY TABLE_NAME`, tableFilter), tableArgs},
		{"columns", fmt.Sprintf(`SELECT TABLE_NAME, COLUMN_NAME, CAST(ORDINAL_POSITION AS CHAR), COLUMN_TYPE, IS_NULLABLE, COALESCE(CAST(COLUMN_DEFAULT AS CHAR), '<NULL>'), EXTRA, COALESCE(COLLATION_NAME, '<NULL>') FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE()%s ORDER BY TABLE_NAME, ORDINAL_POSITION`, tableFilter), tableArgs},
		{"indexes", fmt.Sprintf(`SELECT TABLE_NAME, INDEX_NAME, CAST(NON_UNIQUE AS CHAR), CAST(SEQ_IN_INDEX AS CHAR), COALESCE(COLUMN_NAME, '<NULL>'), COALESCE(CAST(SUB_PART AS CHAR), '<NULL>'), COALESCE(COLLATION, '<NULL>'), INDEX_TYPE, COALESCE(NULLABLE, '<NULL>'), IS_VISIBLE FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE()%s ORDER BY TABLE_NAME, INDEX_NAME, SEQ_IN_INDEX`, tableFilter), tableArgs},
		{"constraints", fmt.Sprintf(`SELECT tc.TABLE_NAME, tc.CONSTRAINT_NAME, tc.CONSTRAINT_TYPE, COALESCE(kcu.COLUMN_NAME, '<NULL>'), COALESCE(CAST(kcu.ORDINAL_POSITION AS CHAR), '<NULL>'), COALESCE(kcu.REFERENCED_TABLE_NAME, '<NULL>'), COALESCE(kcu.REFERENCED_COLUMN_NAME, '<NULL>'), COALESCE(rc.MATCH_OPTION, '<NULL>'), COALESCE(rc.UPDATE_RULE, '<NULL>'), COALESCE(rc.DELETE_RULE, '<NULL>') FROM information_schema.TABLE_CONSTRAINTS tc LEFT JOIN information_schema.KEY_COLUMN_USAGE kcu ON kcu.CONSTRAINT_SCHEMA=tc.CONSTRAINT_SCHEMA AND kcu.TABLE_NAME=tc.TABLE_NAME AND kcu.CONSTRAINT_NAME=tc.CONSTRAINT_NAME LEFT JOIN information_schema.REFERENTIAL_CONSTRAINTS rc ON rc.CONSTRAINT_SCHEMA=tc.CONSTRAINT_SCHEMA AND rc.TABLE_NAME=tc.TABLE_NAME AND rc.CONSTRAINT_NAME=tc.CONSTRAINT_NAME WHERE tc.CONSTRAINT_SCHEMA=DATABASE()%s ORDER BY tc.TABLE_NAME, tc.CONSTRAINT_NAME, kcu.ORDINAL_POSITION`, constraintFilter), constraintArgs},
		{"triggers", fmt.Sprintf(`SELECT TRIGGER_NAME, EVENT_MANIPULATION, EVENT_OBJECT_TABLE, ACTION_TIMING, ACTION_STATEMENT FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE()%s ORDER BY TRIGGER_NAME`, triggerFilter), triggerArgs},
	}
	for _, item := range queries {
		if _, err := h.Write([]byte(item.label)); err != nil {
			return "", err
		}
		var rows *sql.Rows
		var err error
		rows, err = conn.QueryContext(ctx, item.query, item.args...)
		if err != nil {
			return "", fmt.Errorf("query %s for schema fingerprint: %w", item.label, err)
		}
		columns, err := rows.Columns()
		if err != nil {
			rows.Close()
			return "", err
		}
		for rows.Next() {
			values := make([]sql.NullString, len(columns))
			dest := make([]any, len(values))
			for i := range values {
				dest[i] = &values[i]
			}
			if err := rows.Scan(dest...); err != nil {
				rows.Close()
				return "", fmt.Errorf("scan %s schema fingerprint row: %w", item.label, err)
			}
			for _, value := range values {
				text := "<NULL>"
				if value.Valid {
					text = value.String
				}
				var length [8]byte
				binary.BigEndian.PutUint64(length[:], uint64(len(text)))
				_, _ = h.Write(length[:])
				_, _ = h.Write([]byte(text))
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return "", fmt.Errorf("read %s schema fingerprint rows: %w", item.label, err)
		}
		if err := rows.Close(); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func advisoryLockName(databaseName string) string {
	sum := sha256.Sum256([]byte(databaseName))
	return fmt.Sprintf("cue-schema-v1-%x", sum[:12])
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
