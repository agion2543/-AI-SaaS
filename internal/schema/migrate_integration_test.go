package schema

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

const integrationDSNEnv = "SCHEMA_MIGRATION_IT_DSN"

func TestMySQLCandidateSchemaMigration(t *testing.T) {
	control, baseConfig := openSafeMySQLTestServer(t)
	ctx := context.Background()

	t.Run("empty initialization and repeated invocation", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		first, err := ApplyCandidate(ctx, db)
		if err != nil {
			t.Fatalf("first apply: %v", err)
		}
		if first != ResultApplied {
			t.Fatalf("first result = %q, want %q", first, ResultApplied)
		}
		second, err := ApplyCandidate(ctx, db)
		if err != nil {
			t.Fatalf("repeat apply: %v", err)
		}
		if second != ResultAlready {
			t.Fatalf("repeat result = %q, want %q", second, ResultAlready)
		}
		var tables, triggers, ledgerRows int
		if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_TYPE='BASE TABLE'`).Scan(&tables); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE()`).Scan(&triggers); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow(`SELECT COUNT(*) FROM app_schema_migrations WHERE version=? AND checksum=? AND state='applied' AND schema_fingerprint IS NOT NULL`, BaselineVersion, BaselineChecksum).Scan(&ledgerRows); err != nil {
			t.Fatal(err)
		}
		if tables != 30 || triggers != 2 || ledgerRows != 1 {
			t.Fatalf("installed inventory tables=%d triggers=%d matching ledger rows=%d, want 30, 2, 1", tables, triggers, ledgerRows)
		}
	})

	t.Run("managed schema drift refused", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		if _, err := ApplyCandidate(ctx, db); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`ALTER TABLE users ADD COLUMN unexpected_marker INT NULL`); err != nil {
			t.Fatal(err)
		}
		_, err := ApplyCandidate(ctx, db)
		if err == nil || !strings.Contains(err.Error(), "schema drift") {
			t.Fatalf("ApplyCandidate error = %v, want schema drift refusal", err)
		}
	})

	t.Run("unknown non-empty schema refused without mutation", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		if _, err := db.Exec(`CREATE TABLE keep_me (id INT PRIMARY KEY, marker VARCHAR(20) NOT NULL)`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO keep_me (id, marker) VALUES (7, 'untouched')`); err != nil {
			t.Fatal(err)
		}
		_, err := ApplyCandidate(ctx, db)
		if err == nil || !strings.Contains(err.Error(), "refusing non-empty database") {
			t.Fatalf("ApplyCandidate error = %v, want refusal", err)
		}
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM keep_me WHERE id=7 AND marker='untouched'`).Scan(&count); err != nil {
			t.Fatal(err)
		}
		var tables, ledger int
		if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_TYPE='BASE TABLE'`).Scan(&tables); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=?`, ledgerTable).Scan(&ledger); err != nil {
			t.Fatal(err)
		}
		if count != 1 || tables != 1 || ledger != 0 {
			t.Fatalf("refusal changed schema/data: sentinel=%d tables=%d ledger=%d; want 1,1,0", count, tables, ledger)
		}
	})

	t.Run("ledger checksum mismatch refused", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		if _, err := ApplyCandidate(ctx, db); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`UPDATE app_schema_migrations SET checksum=REPEAT('0', 64) WHERE version=?`, BaselineVersion); err != nil {
			t.Fatal(err)
		}
		_, err := ApplyCandidate(ctx, db)
		if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
			t.Fatalf("ApplyCandidate error = %v, want checksum mismatch refusal", err)
		}
		var wrongChecksum string
		if err := db.QueryRow(`SELECT checksum FROM app_schema_migrations WHERE version=?`, BaselineVersion).Scan(&wrongChecksum); err != nil {
			t.Fatal(err)
		}
		if wrongChecksum != strings.Repeat("0", 64) {
			t.Fatalf("refusal modified ledger checksum: %s", wrongChecksum)
		}
	})

	t.Run("incomplete ledger refused", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		if _, err := db.Exec(`CREATE TABLE app_schema_migrations (
			version VARCHAR(128) NOT NULL PRIMARY KEY,
			checksum CHAR(64) NOT NULL,
			state VARCHAR(20) NOT NULL,
			schema_fingerprint CHAR(64) NULL,
			started_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			applied_at TIMESTAMP NULL DEFAULT NULL
		) ENGINE=InnoDB`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO app_schema_migrations(version, checksum, state) VALUES (?, ?, 'applying')`, BaselineVersion, BaselineChecksum); err != nil {
			t.Fatal(err)
		}
		_, err := ApplyCandidate(ctx, db)
		if err == nil || !strings.Contains(err.Error(), "table inventory mismatch") {
			t.Fatalf("ApplyCandidate error = %v, want incomplete inventory refusal", err)
		}
		var tables int
		if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_TYPE='BASE TABLE'`).Scan(&tables); err != nil {
			t.Fatal(err)
		}
		if tables != 1 {
			t.Fatalf("refusal mutated partial database: found %d tables, want 1", tables)
		}
	})
}

func TestLegacySchemaRejectionLeavesStateUntouched(t *testing.T) {
	control, baseConfig := openSafeMySQLTestServer(t)
	ctx := context.Background()
	usersFromInitSQL := `CREATE TABLE users (
		id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
		created_at DATETIME(3) NULL,
		updated_at DATETIME(3) NULL,
		uuid VARCHAR(36) NOT NULL UNIQUE,
		email VARCHAR(120) NOT NULL UNIQUE,
		phone VARCHAR(30) NOT NULL UNIQUE,
		password_hash VARCHAR(255) NOT NULL,
		display_name VARCHAR(80) NOT NULL,
		status VARCHAR(20) NOT NULL DEFAULT 'active',
		member_level VARCHAR(30) NOT NULL DEFAULT 'free',
		current_package_id BIGINT UNSIGNED NULL,
		expired_at DATETIME NULL,
		remaining_quota INT NOT NULL DEFAULT 0,
		bound_devices TEXT NULL,
		auto_renew TINYINT(1) NOT NULL DEFAULT 0,
		reset_token VARCHAR(120) NOT NULL DEFAULT ''
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`

	t.Run("populated partial sql/init.sql base", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		for _, statement := range []string{
			usersFromInitSQL,
			`INSERT INTO users (uuid, email, phone, password_hash, display_name, status, member_level, remaining_quota)
			 VALUES ('legacy-user-7', 'legacy@example.test', '13800000007', 'synthetic-hash', 'Legacy User', 'active', 'free', 11)`,
		} {
			if _, err := db.Exec(statement); err != nil {
				t.Fatalf("create sql/init.sql fixture: %v", err)
			}
		}
		assertLegacyRejectedUnchanged(t, ctx, db)
	})

	t.Run("interrupted multi-tenant legacy migration without ledger", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		for _, statement := range []string{
			usersFromInitSQL,
			`INSERT INTO users (uuid, email, phone, password_hash, display_name, status, member_level, remaining_quota)
			 VALUES ('legacy-user-8', 'legacy8@example.test', '13800000008', 'synthetic-hash', 'Interrupted User', 'active', 'free', 5)`,
			`CREATE TABLE merchants (
				id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
				name VARCHAR(120) NOT NULL,
				contact_phone VARCHAR(30) NULL,
				contact_email VARCHAR(120) NULL,
				status ENUM('pending', 'active', 'suspended') NOT NULL DEFAULT 'pending',
				created_at DATETIME(3) NULL,
				updated_at DATETIME(3) NULL,
				UNIQUE KEY uk_merchants_name (name),
				KEY idx_merchants_status (status)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
			`ALTER TABLE users ADD COLUMN merchant_id BIGINT UNSIGNED NULL AFTER phone,
			 ADD COLUMN role ENUM('super_admin', 'merchant_admin', 'customer') NOT NULL DEFAULT 'customer' AFTER merchant_id`,
		} {
			if _, err := db.Exec(statement); err != nil {
				t.Fatalf("create interrupted legacy migration fixture: %v", err)
			}
		}
		assertLegacyRejectedUnchanged(t, ctx, db)
	})
}

func assertLegacyRejectedUnchanged(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	before := snapshotDatabaseTables(t, db)
	if _, err := ApplyCandidate(ctx, db); err == nil || !strings.Contains(err.Error(), "refusing non-empty database") {
		t.Fatalf("ApplyCandidate error = %v, want refusal of an unmanaged non-empty database", err)
	}
	after := snapshotDatabaseTables(t, db)
	if after != before {
		t.Fatalf("refusing legacy schema changed table definitions or rows:\nBEFORE:\n%s\nAFTER:\n%s", before, after)
	}
	var ledgerCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=?`, ledgerTable).Scan(&ledgerCount); err != nil {
		t.Fatal(err)
	}
	if ledgerCount != 0 {
		t.Fatalf("refusing legacy schema created a false v1 ledger (count=%d)", ledgerCount)
	}
}

// snapshotDatabaseTables captures every base table's exact CREATE statement and
// a sorted encoding of all rows so rejection tests prove non-mutation, not merely
// that a sentinel row survived.
func snapshotDatabaseTables(t *testing.T, db *sql.DB) string {
	t.Helper()
	rows, err := db.Query(`SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_TYPE='BASE TABLE' ORDER BY TABLE_NAME`)
	if err != nil {
		t.Fatal(err)
	}
	var tableNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		tableNames = append(tableNames, name)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		t.Fatal(err)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}

	var snapshot strings.Builder
	for _, name := range tableNames {
		quotedName := "`" + strings.ReplaceAll(name, "`", "``") + "`"
		var shownName, createSQL string
		if err := db.QueryRow("SHOW CREATE TABLE "+quotedName).Scan(&shownName, &createSQL); err != nil {
			t.Fatalf("snapshot CREATE TABLE %q: %v", name, err)
		}
		fmt.Fprintf(&snapshot, "TABLE %s\n%s\n", shownName, createSQL)

		dataRows, err := db.Query("SELECT * FROM " + quotedName)
		if err != nil {
			t.Fatalf("snapshot rows from %q: %v", name, err)
		}
		columns, err := dataRows.Columns()
		if err != nil {
			dataRows.Close()
			t.Fatal(err)
		}
		encodedRows := make([]string, 0)
		for dataRows.Next() {
			values := make([]any, len(columns))
			dest := make([]any, len(columns))
			for i := range values {
				dest[i] = &values[i]
			}
			if err := dataRows.Scan(dest...); err != nil {
				dataRows.Close()
				t.Fatalf("scan rows from %q: %v", name, err)
			}
			encoded, err := json.Marshal(values)
			if err != nil {
				dataRows.Close()
				t.Fatalf("encode row from %q: %v", name, err)
			}
			encodedRows = append(encodedRows, string(encoded))
		}
		if err := dataRows.Err(); err != nil {
			dataRows.Close()
			t.Fatalf("read rows from %q: %v", name, err)
		}
		if err := dataRows.Close(); err != nil {
			t.Fatalf("close rows from %q: %v", name, err)
		}
		sort.Strings(encodedRows)
		for _, row := range encodedRows {
			fmt.Fprintf(&snapshot, "ROW %s %s\n", name, row)
		}
	}
	return snapshot.String()
}

func openSafeMySQLTestServer(t *testing.T) (*sql.DB, *mysqlDriver.Config) {
	t.Helper()
	raw := strings.TrimSpace(os.Getenv(integrationDSNEnv))
	if raw == "" {
		t.Skipf("set %s to a disposable MySQL 8 Unix-socket DSN under /tmp/cue-schema-migrate-* to run", integrationDSNEnv)
	}
	cfg, err := mysqlDriver.ParseDSN(raw)
	if err != nil {
		t.Fatalf("parse %s: %v", integrationDSNEnv, err)
	}
	if cfg.Net != "unix" || !strings.HasPrefix(filepath.Clean(cfg.Addr), "/tmp/cue-schema-migrate-") || !strings.HasPrefix(cfg.DBName, "cue_schema_migration_it") {
		t.Fatalf("refusing unsafe integration DSN: require unix socket under /tmp/cue-schema-migrate-* and database prefix cue_schema_migration_it; got net=%q addr=%q database=%q", cfg.Net, cfg.Addr, cfg.DBName)
	}
	controlConfig := cfg.Clone()
	controlConfig.DBName = ""
	control, err := sql.Open("mysql", controlConfig.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := control.PingContext(ctx); err != nil {
		control.Close()
		t.Fatalf("connect to disposable MySQL test server: %v", err)
	}
	t.Cleanup(func() { _ = control.Close() })
	return control, cfg
}

func createTestDatabase(t *testing.T, control *sql.DB, base *mysqlDriver.Config) *sql.DB {
	t.Helper()
	databaseName := fmt.Sprintf("cue_schema_migration_it_%d", time.Now().UnixNano())
	if _, err := control.Exec("CREATE DATABASE `" + databaseName + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		t.Fatalf("create isolated test database: %v", err)
	}
	cfg := base.Clone()
	cfg.DBName = databaseName
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		_, _ = control.Exec("DROP DATABASE `" + databaseName + "`")
		t.Fatalf("open isolated test database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		_, _ = control.Exec("DROP DATABASE `" + databaseName + "`")
	})
	return db
}
