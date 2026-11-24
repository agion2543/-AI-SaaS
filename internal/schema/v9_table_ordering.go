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
	TableOrderingV9Version  = "schema-v9-table-ordering-20261124"
	TableOrderingV9Checksum = "63a3ea3cba7de9d208fd82d8ccc0fe6957854098aa31a16be768ed35826b1efb"
)

var tableOrderingV9Order = []string{
	"store_table_configs",
	"store_tables",
	"table_sessions",
	"table_cart_items",
	"table_order_details",
}

type tableOrderingV9Definition struct {
	version  string
	checksum string
	tables   map[string]string
}

func loadTableOrderingV9() (tableOrderingV9Definition, error) {
	definition := tableOrderingV9Definition{
		version: TableOrderingV9Version,
		tables: map[string]string{
			"store_table_configs": `CREATE TABLE ` + "`store_table_configs`" + ` (
				` + "`id`" + ` bigint unsigned NOT NULL AUTO_INCREMENT,
				` + "`created_at`" + ` datetime(3) NOT NULL,
				` + "`updated_at`" + ` datetime(3) NOT NULL,
				` + "`store_id`" + ` bigint unsigned NOT NULL,
				` + "`table_count`" + ` int NOT NULL DEFAULT 0,
				PRIMARY KEY (` + "`id`" + `),
				UNIQUE KEY ` + "`uk_store_table_configs_store_id`" + ` (` + "`store_id`" + `),
				CONSTRAINT ` + "`fk_store_table_configs_store_id`" + ` FOREIGN KEY (` + "`store_id`" + `) REFERENCES ` + "`stores`" + ` (` + "`id`" + `) ON DELETE CASCADE,
				CONSTRAINT ` + "`chk_store_table_configs_count`" + ` CHECK (` + "`table_count`" + ` BETWEEN 0 AND 300)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
			"store_tables": `CREATE TABLE ` + "`store_tables`" + ` (
				` + "`id`" + ` bigint unsigned NOT NULL AUTO_INCREMENT,
				` + "`created_at`" + ` datetime(3) NOT NULL,
				` + "`updated_at`" + ` datetime(3) NOT NULL,
				` + "`store_id`" + ` bigint unsigned NOT NULL,
				` + "`table_no`" + ` int NOT NULL,
				` + "`table_code`" + ` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
				PRIMARY KEY (` + "`id`" + `),
				UNIQUE KEY ` + "`uk_store_tables_store_table_no`" + ` (` + "`store_id`" + `,` + "`table_no`" + `),
				UNIQUE KEY ` + "`uk_store_tables_table_code`" + ` (` + "`table_code`" + `),
				CONSTRAINT ` + "`fk_store_tables_store_id`" + ` FOREIGN KEY (` + "`store_id`" + `) REFERENCES ` + "`stores`" + ` (` + "`id`" + `) ON DELETE CASCADE,
				CONSTRAINT ` + "`chk_store_tables_table_no`" + ` CHECK (` + "`table_no`" + ` BETWEEN 1 AND 300)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
			"table_sessions": `CREATE TABLE ` + "`table_sessions`" + ` (
				` + "`id`" + ` bigint unsigned NOT NULL AUTO_INCREMENT,
				` + "`created_at`" + ` datetime(3) NOT NULL,
				` + "`updated_at`" + ` datetime(3) NOT NULL,
				` + "`store_id`" + ` bigint unsigned NOT NULL,
				` + "`table_no`" + ` int NOT NULL,
				` + "`status`" + ` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'open',
				` + "`cart_version`" + ` bigint unsigned NOT NULL DEFAULT 0,
				` + "`checkout_order_id`" + ` bigint unsigned DEFAULT NULL,
				` + "`locked_at`" + ` datetime(3) DEFAULT NULL,
				PRIMARY KEY (` + "`id`" + `),
				UNIQUE KEY ` + "`uk_table_sessions_store_table_no`" + ` (` + "`store_id`" + `,` + "`table_no`" + `),
				KEY ` + "`idx_table_sessions_checkout_order_id`" + ` (` + "`checkout_order_id`" + `),
				KEY ` + "`idx_table_sessions_status`" + ` (` + "`status`" + `),
				CONSTRAINT ` + "`fk_table_sessions_store_table`" + ` FOREIGN KEY (` + "`store_id`" + `,` + "`table_no`" + `) REFERENCES ` + "`store_tables`" + ` (` + "`store_id`" + `,` + "`table_no`" + `) ON DELETE RESTRICT,
				CONSTRAINT ` + "`fk_table_sessions_checkout_order_id`" + ` FOREIGN KEY (` + "`checkout_order_id`" + `) REFERENCES ` + "`orders`" + ` (` + "`id`" + `) ON DELETE RESTRICT,
				CONSTRAINT ` + "`chk_table_sessions_status`" + ` CHECK (` + "`status`" + ` IN ('open','checkout_locked'))
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
			"table_cart_items": `CREATE TABLE ` + "`table_cart_items`" + ` (
				` + "`id`" + ` bigint unsigned NOT NULL AUTO_INCREMENT,
				` + "`created_at`" + ` datetime(3) NOT NULL,
				` + "`updated_at`" + ` datetime(3) NOT NULL,
				` + "`session_id`" + ` bigint unsigned NOT NULL,
				` + "`product_id`" + ` bigint unsigned NOT NULL,
				` + "`quantity`" + ` int NOT NULL,
				PRIMARY KEY (` + "`id`" + `),
				UNIQUE KEY ` + "`uk_table_cart_items_session_product`" + ` (` + "`session_id`" + `,` + "`product_id`" + `),
				KEY ` + "`idx_table_cart_items_product_id`" + ` (` + "`product_id`" + `),
				CONSTRAINT ` + "`fk_table_cart_items_session_id`" + ` FOREIGN KEY (` + "`session_id`" + `) REFERENCES ` + "`table_sessions`" + ` (` + "`id`" + `) ON DELETE CASCADE,
				CONSTRAINT ` + "`fk_table_cart_items_product_id`" + ` FOREIGN KEY (` + "`product_id`" + `) REFERENCES ` + "`store_products`" + ` (` + "`id`" + `) ON DELETE RESTRICT,
				CONSTRAINT ` + "`chk_table_cart_items_quantity`" + ` CHECK (` + "`quantity`" + ` BETWEEN 1 AND 99)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
			"table_order_details": `CREATE TABLE ` + "`table_order_details`" + ` (
				` + "`id`" + ` bigint unsigned NOT NULL AUTO_INCREMENT,
				` + "`created_at`" + ` datetime(3) NOT NULL,
				` + "`updated_at`" + ` datetime(3) NOT NULL,
				` + "`order_id`" + ` bigint unsigned NOT NULL,
				` + "`session_id`" + ` bigint unsigned NOT NULL,
				` + "`table_no`" + ` int NOT NULL,
				PRIMARY KEY (` + "`id`" + `),
				UNIQUE KEY ` + "`uk_table_order_details_order_id`" + ` (` + "`order_id`" + `),
				KEY ` + "`idx_table_order_details_session_id`" + ` (` + "`session_id`" + `),
				CONSTRAINT ` + "`fk_table_order_details_order_id`" + ` FOREIGN KEY (` + "`order_id`" + `) REFERENCES ` + "`orders`" + ` (` + "`id`" + `) ON DELETE RESTRICT,
				CONSTRAINT ` + "`fk_table_order_details_session_id`" + ` FOREIGN KEY (` + "`session_id`" + `) REFERENCES ` + "`table_sessions`" + ` (` + "`id`" + `) ON DELETE RESTRICT,
				CONSTRAINT ` + "`chk_table_order_details_table_no`" + ` CHECK (` + "`table_no`" + ` BETWEEN 1 AND 300)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		},
	}
	if err := validateTableOrderingV9Definition(&definition); err != nil {
		return tableOrderingV9Definition{}, err
	}
	return definition, nil
}

func validateTableOrderingV9Definition(definition *tableOrderingV9Definition) error {
	if definition.version != TableOrderingV9Version || len(definition.tables) != len(tableOrderingV9Order) {
		return errors.New("table ordering v9 definition inventory mismatch")
	}
	material := []string{definition.version}
	for _, name := range tableOrderingV9Order {
		statement, ok := definition.tables[name]
		if !ok || !strings.HasPrefix(statement, "CREATE TABLE ") {
			return fmt.Errorf("table ordering v9 table %q is missing", name)
		}
		material = append(material, name, statement)
	}
	sum := sha256.Sum256([]byte(strings.Join(material, "\n")))
	definition.checksum = fmt.Sprintf("%x", sum)
	if TableOrderingV9Checksum == "TODO" || definition.checksum != TableOrderingV9Checksum {
		return fmt.Errorf("table ordering v9 checksum mismatch: definition %s, expected %s", definition.checksum, TableOrderingV9Checksum)
	}
	return nil
}

// ApplyTableOrderingV9 appends only new tables to an exact managed v8 schema.
// The v1-v8 schema and fingerprints remain untouched.
func ApplyTableOrderingV9(ctx context.Context, db *sql.DB) (Result, error) {
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
	if err := verifyManagedSchemaV8(ctx, conn, objects, base, v2, v3, v4, v5, v6, v7, v8); err == nil {
		if err := applyTableOrderingV9(ctx, conn, base, v2, v3, v4, v5, v6, v7, v8, v9); err != nil {
			return "", err
		}
		return ResultApplied, nil
	}
	if err := verifyManagedSchemaV9(ctx, conn, objects, base, v2, v3, v4, v5, v6, v7, v8, v9); err != nil {
		return "", fmt.Errorf("refusing non-exact managed schema for v9 table ordering migration: %w", err)
	}
	return ResultAlready, nil
}

func applyTableOrderingV9(ctx context.Context, conn *sql.Conn, base baselineDefinition, v2 incrementalMigrationDefinition, v3 adminAuthV3Definition, v4 otpSafetyV4Definition, v5 emailOTPSafetyV5Definition, v6 wholesaleRetailV6Definition, v7 platformExpensesV7Definition, v8 platformExpenseReversalsV8Definition, v9 tableOrderingV9Definition) error {
	if _, err := conn.ExecContext(ctx, `INSERT INTO `+ledgerTable+` (version, checksum, state) VALUES (?, ?, 'applying')`, v9.version, v9.checksum); err != nil {
		return fmt.Errorf("record v9 table ordering migration start: %w", err)
	}
	for _, name := range tableOrderingV9Order {
		if _, err := conn.ExecContext(ctx, v9.tables[name]); err != nil {
			return fmt.Errorf("create v9 table %q (v9 ledger remains incomplete): %w", name, err)
		}
	}
	if err := verifyTableOrderingV9Tables(ctx, conn); err != nil {
		return fmt.Errorf("verify v9 tables (v9 ledger remains incomplete): %w", err)
	}
	objects, err := inspectObjects(ctx, conn)
	if err != nil {
		return err
	}
	if err := verifyObjectInventoryV9(objects, base, v2, v7, v8, v9); err != nil {
		return fmt.Errorf("post-apply v9 inventory verification failed (v9 ledger remains incomplete): %w", err)
	}
	v8Fingerprint, err := schemaFingerprintExcluding(ctx, conn, tableOrderingV9Order, nil)
	if err != nil {
		return fmt.Errorf("compute immutable v8 schema fingerprint during v9 migration: %w", err)
	}
	var storedV8 string
	if err := conn.QueryRowContext(ctx, "SELECT schema_fingerprint FROM "+ledgerTable+" WHERE version=? AND checksum=? AND state='applied'", v8.version, v8.checksum).Scan(&storedV8); err != nil {
		return fmt.Errorf("read immutable v8 schema fingerprint during v9 migration: %w", err)
	}
	if storedV8 != v8Fingerprint {
		return fmt.Errorf("refusing v8 schema drift during v9 migration: stored %s, actual %s", storedV8, v8Fingerprint)
	}
	fingerprint, err := schemaFingerprint(ctx, conn)
	if err != nil {
		return fmt.Errorf("compute v9 installed schema fingerprint: %w", err)
	}
	result, err := conn.ExecContext(ctx, `UPDATE `+ledgerTable+` SET state='applied', schema_fingerprint=?, applied_at=CURRENT_TIMESTAMP WHERE version=? AND checksum=? AND state='applying'`, fingerprint, v9.version, v9.checksum)
	if err != nil {
		return fmt.Errorf("complete v9 schema version ledger: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("verify v9 schema version ledger update: %w", err)
	}
	if rows != 1 {
		return errors.New("v9 schema version ledger did not transition exactly once; it remains incomplete")
	}
	_ = v3
	_ = v4
	_ = v5
	_ = v6
	return nil
}

func verifyManagedSchemaV9(ctx context.Context, conn *sql.Conn, objects schemaObjects, base baselineDefinition, v2 incrementalMigrationDefinition, v3 adminAuthV3Definition, v4 otpSafetyV4Definition, v5 emailOTPSafetyV5Definition, v6 wholesaleRetailV6Definition, v7 platformExpensesV7Definition, v8 platformExpenseReversalsV8Definition, v9 tableOrderingV9Definition) error {
	if err := verifyTableOrderingV9Tables(ctx, conn); err != nil {
		return err
	}
	if err := verifyObjectInventoryV9(objects, base, v2, v7, v8, v9); err != nil {
		return err
	}
	if err := verifyPlatformExpenseReversalsV8HistoricalFingerprints(ctx, conn, base, v2, v3, v4, v5, v6, v7, v8, tableOrderingV9Order...); err != nil {
		return err
	}
	rows, err := conn.QueryContext(ctx, "SELECT version, checksum, state, schema_fingerprint FROM "+ledgerTable+" ORDER BY version")
	if err != nil {
		return fmt.Errorf("read v9 schema version ledger: %w", err)
	}
	type ledgerEntry struct {
		version, checksum, state string
		fingerprint              sql.NullString
	}
	entries := make([]ledgerEntry, 0, 9)
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
	if len(entries) != 9 {
		return fmt.Errorf("refusing v9 ledger with %d rows; expected exactly immutable v1-v9 rows", len(entries))
	}
	byVersion := make(map[string]ledgerEntry, len(entries))
	for _, entry := range entries {
		byVersion[entry.version] = entry
	}
	expected := []struct{ version, checksum string }{{BaselineVersion, base.checksum}, {v2.version, v2.checksum}, {v3.version, v3.checksum}, {v4.version, v4.checksum}, {v5.version, v5.checksum}, {v6.version, v6.checksum}, {v7.version, v7.checksum}, {v8.version, v8.checksum}, {v9.version, v9.checksum}}
	for _, want := range expected {
		entry, ok := byVersion[want.version]
		if !ok || entry.checksum != want.checksum || entry.state != "applied" || !entry.fingerprint.Valid || entry.fingerprint.String == "" {
			return fmt.Errorf("refusing missing, altered, or incomplete %s ledger entry", want.version)
		}
	}
	actualV8, err := schemaFingerprintExcluding(ctx, conn, tableOrderingV9Order, nil)
	if err != nil {
		return fmt.Errorf("verify immutable v8 schema fingerprint: %w", err)
	}
	if stored := byVersion[v8.version].fingerprint.String; stored != actualV8 {
		return fmt.Errorf("refusing v8 schema drift: stored fingerprint %s, actual %s", stored, actualV8)
	}
	actualV9, err := schemaFingerprint(ctx, conn)
	if err != nil {
		return fmt.Errorf("verify v9 managed schema fingerprint: %w", err)
	}
	if stored := byVersion[v9.version].fingerprint.String; stored != actualV9 {
		return fmt.Errorf("refusing v9 schema drift: stored fingerprint %s, actual %s", stored, actualV9)
	}
	_ = v6
	return nil
}

func verifyTableOrderingV9Tables(ctx context.Context, conn *sql.Conn) error {
	type columnShape struct {
		name, columnType, nullable string
		defaultValue               string
		hasDefault                 bool
		extra, collation           string
	}
	columns := map[string][]columnShape{
		"store_table_configs": {
			{name: "id", columnType: "bigint unsigned", nullable: "NO", extra: "auto_increment"},
			{name: "created_at", columnType: "datetime(3)", nullable: "NO"},
			{name: "updated_at", columnType: "datetime(3)", nullable: "NO"},
			{name: "store_id", columnType: "bigint unsigned", nullable: "NO"},
			{name: "table_count", columnType: "int", nullable: "NO", defaultValue: "0", hasDefault: true},
		},
		"store_tables": {
			{name: "id", columnType: "bigint unsigned", nullable: "NO", extra: "auto_increment"},
			{name: "created_at", columnType: "datetime(3)", nullable: "NO"},
			{name: "updated_at", columnType: "datetime(3)", nullable: "NO"},
			{name: "store_id", columnType: "bigint unsigned", nullable: "NO"},
			{name: "table_no", columnType: "int", nullable: "NO"},
			{name: "table_code", columnType: "varchar(64)", nullable: "NO", collation: "utf8mb4_unicode_ci"},
		},
		"table_sessions": {
			{name: "id", columnType: "bigint unsigned", nullable: "NO", extra: "auto_increment"},
			{name: "created_at", columnType: "datetime(3)", nullable: "NO"},
			{name: "updated_at", columnType: "datetime(3)", nullable: "NO"},
			{name: "store_id", columnType: "bigint unsigned", nullable: "NO"},
			{name: "table_no", columnType: "int", nullable: "NO"},
			{name: "status", columnType: "varchar(24)", nullable: "NO", defaultValue: "open", hasDefault: true, collation: "utf8mb4_unicode_ci"},
			{name: "cart_version", columnType: "bigint unsigned", nullable: "NO", defaultValue: "0", hasDefault: true},
			{name: "checkout_order_id", columnType: "bigint unsigned", nullable: "YES"},
			{name: "locked_at", columnType: "datetime(3)", nullable: "YES"},
		},
		"table_cart_items": {
			{name: "id", columnType: "bigint unsigned", nullable: "NO", extra: "auto_increment"},
			{name: "created_at", columnType: "datetime(3)", nullable: "NO"},
			{name: "updated_at", columnType: "datetime(3)", nullable: "NO"},
			{name: "session_id", columnType: "bigint unsigned", nullable: "NO"},
			{name: "product_id", columnType: "bigint unsigned", nullable: "NO"},
			{name: "quantity", columnType: "int", nullable: "NO"},
		},
		"table_order_details": {
			{name: "id", columnType: "bigint unsigned", nullable: "NO", extra: "auto_increment"},
			{name: "created_at", columnType: "datetime(3)", nullable: "NO"},
			{name: "updated_at", columnType: "datetime(3)", nullable: "NO"},
			{name: "order_id", columnType: "bigint unsigned", nullable: "NO"},
			{name: "session_id", columnType: "bigint unsigned", nullable: "NO"},
			{name: "table_no", columnType: "int", nullable: "NO"},
		},
	}
	for _, table := range tableOrderingV9Order {
		var engine, tableCollation string
		if err := conn.QueryRowContext(ctx, "SELECT ENGINE, COALESCE(TABLE_COLLATION,'') FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=?", table).Scan(&engine, &tableCollation); err != nil {
			return fmt.Errorf("inspect %s engine and collation: %w", table, err)
		}
		if engine != "InnoDB" || tableCollation != "utf8mb4_unicode_ci" {
			return fmt.Errorf("%s engine/collation mismatch: found %s/%s, expected InnoDB/utf8mb4_unicode_ci", table, engine, tableCollation)
		}
		rows, err := conn.QueryContext(ctx, "SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_DEFAULT, EXTRA, COALESCE(COLLATION_NAME,'') FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? ORDER BY ORDINAL_POSITION", table)
		if err != nil {
			return fmt.Errorf("inspect %s columns: %w", table, err)
		}
		actual := make([]columnShape, 0)
		for rows.Next() {
			var got columnShape
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
		if len(actual) != len(columns[table]) {
			return fmt.Errorf("%s column count mismatch: found %d, expected %d", table, len(actual), len(columns[table]))
		}
		for i, want := range columns[table] {
			if actual[i] != want {
				return fmt.Errorf("%s column %d mismatch: found %+v, expected %+v", table, i+1, actual[i], want)
			}
		}
	}

	wantIndexes := map[string][]string{
		"store_table_configs": {"PRIMARY|0|1|id|NULL|BTREE", "uk_store_table_configs_store_id|0|1|store_id|NULL|BTREE"},
		"store_tables":        {"PRIMARY|0|1|id|NULL|BTREE", "uk_store_tables_store_table_no|0|1|store_id|NULL|BTREE", "uk_store_tables_store_table_no|0|2|table_no|NULL|BTREE", "uk_store_tables_table_code|0|1|table_code|NULL|BTREE"},
		"table_sessions":      {"PRIMARY|0|1|id|NULL|BTREE", "uk_table_sessions_store_table_no|0|1|store_id|NULL|BTREE", "uk_table_sessions_store_table_no|0|2|table_no|NULL|BTREE", "idx_table_sessions_checkout_order_id|1|1|checkout_order_id|NULL|BTREE", "idx_table_sessions_status|1|1|status|NULL|BTREE"},
		"table_cart_items":    {"PRIMARY|0|1|id|NULL|BTREE", "uk_table_cart_items_session_product|0|1|session_id|NULL|BTREE", "uk_table_cart_items_session_product|0|2|product_id|NULL|BTREE", "idx_table_cart_items_product_id|1|1|product_id|NULL|BTREE"},
		"table_order_details": {"PRIMARY|0|1|id|NULL|BTREE", "uk_table_order_details_order_id|0|1|order_id|NULL|BTREE", "idx_table_order_details_session_id|1|1|session_id|NULL|BTREE"},
	}
	for _, table := range tableOrderingV9Order {
		rows, err := conn.QueryContext(ctx, "SELECT INDEX_NAME, NON_UNIQUE, SEQ_IN_INDEX, COLUMN_NAME, SUB_PART, INDEX_TYPE FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? ORDER BY INDEX_NAME, SEQ_IN_INDEX", table)
		if err != nil {
			return fmt.Errorf("inspect %s indexes: %w", table, err)
		}
		actual := make([]string, 0)
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
			actual = append(actual, fmt.Sprintf("%s|%d|%d|%s|%s|%s", name, nonUnique, sequence, column, subPartValue, indexType))
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		want := append([]string(nil), wantIndexes[table]...)
		sort.Strings(actual)
		sort.Strings(want)
		if !equalStrings(actual, want) {
			return fmt.Errorf("%s exact index definition mismatch: found %v, expected %v", table, actual, want)
		}
	}

	wantForeignKeys := []string{
		"fk_store_table_configs_store_id|store_table_configs|store_id|stores|id|1|CASCADE|NO ACTION",
		"fk_store_tables_store_id|store_tables|store_id|stores|id|1|CASCADE|NO ACTION",
		"fk_table_sessions_store_table|table_sessions|store_id|store_tables|store_id|1|RESTRICT|NO ACTION",
		"fk_table_sessions_store_table|table_sessions|table_no|store_tables|table_no|2|RESTRICT|NO ACTION",
		"fk_table_sessions_checkout_order_id|table_sessions|checkout_order_id|orders|id|1|RESTRICT|NO ACTION",
		"fk_table_cart_items_session_id|table_cart_items|session_id|table_sessions|id|1|CASCADE|NO ACTION",
		"fk_table_cart_items_product_id|table_cart_items|product_id|store_products|id|1|RESTRICT|NO ACTION",
		"fk_table_order_details_order_id|table_order_details|order_id|orders|id|1|RESTRICT|NO ACTION",
		"fk_table_order_details_session_id|table_order_details|session_id|table_sessions|id|1|RESTRICT|NO ACTION",
	}
	rows, err := conn.QueryContext(ctx, `SELECT k.CONSTRAINT_NAME, k.TABLE_NAME, k.COLUMN_NAME, k.REFERENCED_TABLE_NAME, k.REFERENCED_COLUMN_NAME, k.ORDINAL_POSITION, r.DELETE_RULE, r.UPDATE_RULE
		FROM information_schema.KEY_COLUMN_USAGE k
		JOIN information_schema.REFERENTIAL_CONSTRAINTS r
		ON r.CONSTRAINT_SCHEMA=k.CONSTRAINT_SCHEMA AND r.CONSTRAINT_NAME=k.CONSTRAINT_NAME AND r.TABLE_NAME=k.TABLE_NAME
		WHERE k.TABLE_SCHEMA=DATABASE() AND k.TABLE_NAME IN (?, ?, ?, ?, ?) AND k.REFERENCED_TABLE_NAME IS NOT NULL
		ORDER BY k.CONSTRAINT_NAME, k.ORDINAL_POSITION`, tableOrderingV9Order[0], tableOrderingV9Order[1], tableOrderingV9Order[2], tableOrderingV9Order[3], tableOrderingV9Order[4])
	if err != nil {
		return fmt.Errorf("inspect v9 foreign keys: %w", err)
	}
	actualForeignKeys := make([]string, 0)
	for rows.Next() {
		var name, table, column, referencedTable, referencedColumn, deleteRule, updateRule string
		var ordinal int
		if err := rows.Scan(&name, &table, &column, &referencedTable, &referencedColumn, &ordinal, &deleteRule, &updateRule); err != nil {
			rows.Close()
			return err
		}
		actualForeignKeys = append(actualForeignKeys, fmt.Sprintf("%s|%s|%s|%s|%s|%d|%s|%s", name, table, column, referencedTable, referencedColumn, ordinal, deleteRule, updateRule))
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	sort.Strings(actualForeignKeys)
	sort.Strings(wantForeignKeys)
	if !equalStrings(actualForeignKeys, wantForeignKeys) {
		return fmt.Errorf("v9 exact foreign-key definition mismatch: found %v, expected %v", actualForeignKeys, wantForeignKeys)
	}

	wantChecks := []string{
		"chk_store_table_configs_count|store_table_configs|table_countbetween0and300|YES",
		"chk_store_tables_table_no|store_tables|table_nobetween1and300|YES",
		"chk_table_sessions_status|table_sessions|statusin'open','checkout_locked'|YES",
		"chk_table_cart_items_quantity|table_cart_items|quantitybetween1and99|YES",
		"chk_table_order_details_table_no|table_order_details|table_nobetween1and300|YES",
	}
	rows, err = conn.QueryContext(ctx, `SELECT tc.CONSTRAINT_NAME, tc.TABLE_NAME, cc.CHECK_CLAUSE, tc.ENFORCED
		FROM information_schema.TABLE_CONSTRAINTS tc
		JOIN information_schema.CHECK_CONSTRAINTS cc
		ON cc.CONSTRAINT_SCHEMA=tc.CONSTRAINT_SCHEMA AND cc.CONSTRAINT_NAME=tc.CONSTRAINT_NAME
		WHERE tc.TABLE_SCHEMA=DATABASE() AND tc.TABLE_NAME IN (?, ?, ?, ?, ?) AND tc.CONSTRAINT_TYPE='CHECK'
		ORDER BY tc.CONSTRAINT_NAME`, tableOrderingV9Order[0], tableOrderingV9Order[1], tableOrderingV9Order[2], tableOrderingV9Order[3], tableOrderingV9Order[4])
	if err != nil {
		return fmt.Errorf("inspect v9 CHECK constraints: %w", err)
	}
	actualChecks := make([]string, 0)
	for rows.Next() {
		var name, table, clause, enforced string
		if err := rows.Scan(&name, &table, &clause, &enforced); err != nil {
			rows.Close()
			return err
		}
		actualChecks = append(actualChecks, fmt.Sprintf("%s|%s|%s|%s", name, table, normalizeCheckClause(clause), enforced))
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	sort.Strings(actualChecks)
	sort.Strings(wantChecks)
	if !equalStrings(actualChecks, wantChecks) {
		return fmt.Errorf("v9 exact CHECK-constraint definition mismatch: found %v, expected %v", actualChecks, wantChecks)
	}
	return nil
}

func normalizeCheckClause(clause string) string {
	clause = strings.ToLower(clause)
	clause = strings.ReplaceAll(clause, "_utf8mb4", "")
	clause = strings.ReplaceAll(clause, `\'`, "'")
	clause = strings.ReplaceAll(clause, "`", "")
	clause = strings.ReplaceAll(clause, " ", "")
	clause = strings.ReplaceAll(clause, "	", "")
	clause = strings.ReplaceAll(clause, "\n", "")
	clause = strings.ReplaceAll(clause, "(", "")
	clause = strings.ReplaceAll(clause, ")", "")
	return clause
}
func verifyObjectInventoryV9(objects schemaObjects, base baselineDefinition, v2 incrementalMigrationDefinition, v7 platformExpensesV7Definition, v8 platformExpenseReversalsV8Definition, v9 tableOrderingV9Definition) error {
	filtered := objects
	for _, name := range tableOrderingV9Order {
		if !containsSchemaName(objects.tables, name) {
			return fmt.Errorf("v9 table %q is missing", name)
		}
		filtered.tables = removeSchemaName(filtered.tables, name)
	}
	if err := verifyObjectInventoryV8(filtered, base, v2, v7, v8); err != nil {
		return err
	}
	_ = v9
	return nil
}
