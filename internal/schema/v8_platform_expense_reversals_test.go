package schema

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"
)

func TestPlatformExpenseReversalsV8DefinitionIsChecksumPinned(t *testing.T) {
	definition, err := loadPlatformExpenseReversalsV8()
	if err != nil {
		t.Fatalf("load v8 platform expense reversals definition: %v", err)
	}
	if definition.version != PlatformExpenseReversalsV8Version || definition.checksum != PlatformExpenseReversalsV8Checksum {
		t.Fatalf("loaded v8 identity=%q/%q, want %q/%q", definition.version, definition.checksum, PlatformExpenseReversalsV8Version, PlatformExpenseReversalsV8Checksum)
	}
	for _, fragment := range []string{"platform_expense_reversals", "expense_id", "amount_cents", "reversal_date", "reason", "fk_platform_expense_reversals_expense_id", "ON DELETE RESTRICT"} {
		if !strings.Contains(definition.table, fragment) {
			t.Errorf("v8 reversal table DDL is missing %q", fragment)
		}
	}
	if strings.Contains(strings.ToLower(definition.table), "insert into") || strings.Contains(strings.ToLower(definition.table), "drop ") || strings.Contains(strings.ToLower(definition.table), "update ") {
		t.Fatalf("v8 table DDL must only create an empty additive ledger: %s", definition.table)
	}
	if len(definition.triggers) != 2 {
		t.Fatalf("v8 must define only UPDATE/DELETE append-only triggers, got %d", len(definition.triggers))
	}
}

func TestMySQLPlatformExpenseReversalsV8Migration(t *testing.T) {
	control, baseConfig := openSafeMySQLTestServer(t)
	ctx := context.Background()

	t.Run("exact v7 upgrade creates an empty linked append-only reversal ledger", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		createExactV6PlatformExpensesSchema(t, ctx, db)
		if result, err := ApplyPlatformExpensesV7(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("apply v7 from exact v6: result=%q err=%v", result, err)
		}
		if result, err := ApplyPlatformExpenseReversalsV8(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("apply v8 from exact v7: result=%q err=%v", result, err)
		}
		var rows int64
		if err := db.QueryRow(`SELECT COUNT(*) FROM platform_expense_reversals`).Scan(&rows); err != nil || rows != 0 {
			t.Fatalf("new v8 reversal rows=%d err=%v, want empty", rows, err)
		}
		expenseID := insertSyntheticV8Expense(t, db)
		insertReversal := func(originalID uint64, amount int64, date, reason, key string) error {
			_, err := db.Exec(`INSERT INTO platform_expense_reversals(created_at, updated_at, expense_id, amount_cents, reversal_date, reason, created_by, idempotency_key) VALUES(?,?,?,?,?,?,?,?)`, time.Now(), time.Now(), originalID, amount, date, reason, 1, key)
			return err
		}
		if err := insertReversal(expenseID, 4000, time.Now().Format("2006-01-02"), "synthetic partial reversal A", "fc2f22bf-09e4-4a67-b90e-43d0b42d434b"); err != nil {
			t.Fatalf("insert first isolated synthetic v8 partial reversal: %v", err)
		}
		if err := insertReversal(expenseID, 8345, time.Now().Format("2006-01-02"), "synthetic partial reversal B", "42d336f4-3f4a-4a26-9a39-9cfa08d7a3df"); err != nil {
			t.Fatalf("insert second isolated synthetic v8 partial reversal: %v", err)
		}
		for _, bad := range []struct {
			label, query string
			args         []any
		}{
			{"missing original expense", `INSERT INTO platform_expense_reversals(created_at, updated_at, expense_id, amount_cents, reversal_date, reason, created_by, idempotency_key) VALUES(?,?,?,?,?,?,?,?)`, []any{time.Now(), time.Now(), 99999999, 1, time.Now().Format("2006-01-02"), "synthetic missing original", 1, "feb0bf1e-86c1-4e44-8f62-d1d0a2e9b979"}},
			{"non-positive amount", `INSERT INTO platform_expense_reversals(created_at, updated_at, expense_id, amount_cents, reversal_date, reason, created_by, idempotency_key) VALUES(?,?,?,?,?,?,?,?)`, []any{time.Now(), time.Now(), expenseID, 0, time.Now().Format("2006-01-02"), "synthetic zero amount", 1, "8e4577c6-801f-4e3b-96d3-e1ca3d0ea2e4"}},
			{"empty reason", `INSERT INTO platform_expense_reversals(created_at, updated_at, expense_id, amount_cents, reversal_date, reason, created_by, idempotency_key) VALUES(?,?,?,?,?,?,?,?)`, []any{time.Now(), time.Now(), expenseID, 1, time.Now().Format("2006-01-02"), "   ", 1, "579c3ab8-443d-47fb-b487-369ad6346976"}},
		} {
			if _, err := db.Exec(bad.query, bad.args...); err == nil {
				t.Fatalf("v8 database constraints allowed %s", bad.label)
			}
		}
		for _, statement := range []struct{ label, sql string }{
			{"update", `UPDATE platform_expense_reversals SET reason='tampered' WHERE expense_id=?`},
			{"delete", `DELETE FROM platform_expense_reversals WHERE expense_id=?`},
		} {
			if _, err := db.Exec(statement.sql, expenseID); err == nil || !strings.Contains(strings.ToLower(err.Error()), "append-only") {
				t.Fatalf("v8 append-only trigger did not reject %s as expected: %v", statement.label, err)
			}
		}
		var reversalTotal int64
		if err := db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(amount_cents),0) FROM platform_expense_reversals WHERE expense_id=?`, expenseID).Scan(&rows, &reversalTotal); err != nil || rows != 2 || reversalTotal != 12345 {
			t.Fatalf("failed constraint/update/delete changed synthetic v8 partial reversal ledger: rows=%d total=%d err=%v", rows, reversalTotal, err)
		}
		if result, err := ApplyLatest(ctx, db); err != nil || result != ResultAlready {
			t.Fatalf("v8 exact rerun: result=%q err=%v", result, err)
		}
		assertLatestInventory(t, db)
	})

	t.Run("fresh install reaches v8 with no reversal seed rows", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		if result, err := ApplyLatest(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("fresh latest install: result=%q err=%v", result, err)
		}
		var rows int64
		if err := db.QueryRow(`SELECT COUNT(*) FROM platform_expense_reversals`).Scan(&rows); err != nil || rows != 0 {
			t.Fatalf("fresh v8 reversal rows=%d err=%v, want empty", rows, err)
		}
		assertLatestInventory(t, db)
	})

	t.Run("unexpected object refuses v8 without writing a migration row", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		createExactV6PlatformExpensesSchema(t, ctx, db)
		if _, err := ApplyPlatformExpensesV7(ctx, db); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`CREATE TABLE unexpected_platform_expense_reversal_object(id INT PRIMARY KEY)`); err != nil {
			t.Fatal(err)
		}
		if _, err := ApplyPlatformExpenseReversalsV8(ctx, db); err == nil {
			t.Fatal("v8 unexpectedly accepted an unmanaged extra table")
		}
		if countWhere(t, db, "app_schema_migrations", "version = ?", PlatformExpenseReversalsV8Version) != 0 {
			t.Fatal("v8 ledger row was written after refusing an unmanaged extra object")
		}
		var reversalTables int
		if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='platform_expense_reversals'`).Scan(&reversalTables); err != nil {
			t.Fatal(err)
		}
		if reversalTables != 0 {
			t.Fatal("v8 reversal table was created after refusing an unmanaged extra object")
		}
	})

	t.Run("incomplete applying v8 ledger remains fail closed", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		createExactV6PlatformExpensesSchema(t, ctx, db)
		if _, err := ApplyPlatformExpensesV7(ctx, db); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO app_schema_migrations(version, checksum, state) VALUES(?, ?, 'applying')`, PlatformExpenseReversalsV8Version, PlatformExpenseReversalsV8Checksum); err != nil {
			t.Fatal(err)
		}
		if _, err := ApplyLatest(ctx, db); err == nil {
			t.Fatal("ApplyLatest unexpectedly repaired an incomplete v8 migration")
		}
		var reversalTables int
		if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='platform_expense_reversals'`).Scan(&reversalTables); err != nil {
			t.Fatal(err)
		}
		if reversalTables != 0 {
			t.Fatal("incomplete v8 migration was automatically repaired")
		}
	})
}

func insertSyntheticV8Expense(t *testing.T, db *sql.DB) uint64 {
	t.Helper()
	now := time.Now()
	result, err := db.Exec(`INSERT INTO platform_expenses(created_at, updated_at, category, amount_cents, currency, expense_date, memo, created_by, idempotency_key) VALUES(?,?,?,?,?,?,?,?,?)`, now, now, "cloud_hosting", 12345, "CNY", now.Format("2006-01-02"), "synthetic v8 source expense", 1, "2f18ce98-4f71-4f29-9409-3450d05eec74")
	if err != nil {
		t.Fatalf("insert isolated synthetic v8 source expense: %v", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return uint64(id)
}
