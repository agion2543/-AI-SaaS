package schema

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"
)

func TestPlatformExpensesV7DefinitionIsChecksumPinned(t *testing.T) {
	definition, err := loadPlatformExpensesV7()
	if err != nil {
		t.Fatalf("load v7 platform expenses definition: %v", err)
	}
	if definition.version != PlatformExpensesV7Version || definition.checksum != PlatformExpensesV7Checksum {
		t.Fatalf("loaded v7 identity=%q/%q, want %q/%q", definition.version, definition.checksum, PlatformExpensesV7Version, PlatformExpensesV7Checksum)
	}
	for _, fragment := range []string{"platform_expenses", "amount_cents", "currency", "'CNY'", "expense_date", "memo", "chk_platform_expenses_category", "chk_platform_expenses_amount_positive", "chk_platform_expenses_currency", "chk_platform_expenses_memo"} {
		if !strings.Contains(definition.table, fragment) {
			t.Errorf("v7 table DDL is missing %q", fragment)
		}
	}
	if strings.Contains(strings.ToLower(definition.table), "insert into") || strings.Contains(strings.ToLower(definition.table), "drop ") || strings.Contains(strings.ToLower(definition.table), "update ") {
		t.Fatalf("v7 table DDL must only create an empty additive ledger: %s", definition.table)
	}
	if len(definition.triggers) != 2 {
		t.Fatalf("v7 must define only UPDATE/DELETE append-only triggers, got %d", len(definition.triggers))
	}
}

func TestNormalizePlatformExpenseCheckHandlesMySQLCharsetAndEscapedQuotes(t *testing.T) {
	clause := `(` + "`category`" + ` IN (_utf8mb4\'cloud_hosting\',_utf8mb4\'other\'))`
	if got, want := normalizePlatformExpenseCheck(clause), `categoryin'cloud_hosting','other'`; got != want {
		t.Fatalf("normalized CHECK clause=%q want=%q", got, want)
	}
}

func TestMySQLPlatformExpensesV7Migration(t *testing.T) {
	control, baseConfig := openSafeMySQLTestServer(t)
	ctx := context.Background()

	t.Run("exact v6 upgrade adds an empty append-only ledger", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		createExactV6PlatformExpensesSchema(t, ctx, db)
		if result, err := ApplyPlatformExpensesV7(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("apply v7 from exact v6: result=%q err=%v", result, err)
		}
		var rows int64
		if err := db.QueryRow(`SELECT COUNT(*) FROM platform_expenses`).Scan(&rows); err != nil || rows != 0 {
			t.Fatalf("new v7 ledger rows=%d err=%v, want empty", rows, err)
		}
		insertExpense := func(category string, amount int64, currency, memo, key string) error {
			_, err := db.Exec(`INSERT INTO platform_expenses(created_at, updated_at, category, amount_cents, currency, expense_date, memo, created_by, idempotency_key) VALUES(?,?,?,?,?,?,?,?,?)`, time.Now(), time.Now(), category, amount, currency, time.Now().Format("2006-01-02"), memo, 1, key)
			return err
		}
		for _, bad := range []struct {
			label, category string
			amount          int64
			currency, memo  string
			key             string
		}{
			{"category", "invalid", 1234, "CNY", "synthetic invalid category", "c9f7640f-0800-44f0-9c8f-78dd6e5ef570"},
			{"amount", "cloud_hosting", 0, "CNY", "synthetic invalid amount", "d62f8c18-c00b-4a5c-b54e-a735a6861f1e"},
			{"currency", "cloud_hosting", 1234, "USD", "synthetic invalid currency", "a93a6dd1-ae14-4fe1-96e3-a26181c2c1c7"},
			{"memo", "cloud_hosting", 1234, "CNY", "   ", "e16c52b5-fdbc-44a4-9e93-bd22697d886d"},
		} {
			if err := insertExpense(bad.category, bad.amount, bad.currency, bad.memo, bad.key); err == nil {
				t.Fatalf("v7 database constraint allowed invalid %s", bad.label)
			}
		}
		if err := db.QueryRow(`SELECT COUNT(*) FROM platform_expenses`).Scan(&rows); err != nil || rows != 0 {
			t.Fatalf("invalid synthetic inserts changed v7 row count=%d err=%v", rows, err)
		}
		validKey := "c9f7640f-0800-44f0-9c8f-78dd6e5ef570"
		if err := insertExpense("cloud_hosting", 1234, "CNY", "synthetic lifecycle row", validKey); err != nil {
			t.Fatalf("insert isolated synthetic v7 row: %v", err)
		}
		if err := insertExpense("cloud_hosting", 1234, "CNY", "synthetic duplicate key", validKey); err == nil {
			t.Fatal("v7 unique idempotency key constraint allowed a duplicate")
		}
		for _, statement := range []struct{ label, sql string }{
			{"update", `UPDATE platform_expenses SET memo='tampered' WHERE id=1`},
			{"delete", `DELETE FROM platform_expenses WHERE id=1`},
		} {
			if _, err := db.Exec(statement.sql); err == nil {
				t.Fatalf("v7 append-only trigger allowed %s", statement.label)
			}
		}
		if err := db.QueryRow(`SELECT COUNT(*) FROM platform_expenses WHERE memo='synthetic lifecycle row'`).Scan(&rows); err != nil || rows != 1 {
			t.Fatalf("failed update/delete changed synthetic ledger row count=%d err=%v", rows, err)
		}
		if result, err := ApplyPlatformExpensesV7(ctx, db); err != nil || result != ResultAlready {
			t.Fatalf("v7 exact rerun: result=%q err=%v", result, err)
		}
		if result, err := ApplyLatest(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("latest v10 upgrade from exact v7: result=%q err=%v", result, err)
		}
		assertLatestInventory(t, db)
	})

	t.Run("latest fresh install reaches v10 without expense seed rows", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		if result, err := ApplyLatest(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("fresh latest install: result=%q err=%v", result, err)
		}
		var rows int64
		if err := db.QueryRow(`SELECT COUNT(*) FROM platform_expenses`).Scan(&rows); err != nil || rows != 0 {
			t.Fatalf("fresh migration expense rows=%d err=%v, want 0", rows, err)
		}
		assertLatestInventory(t, db)
	})

	t.Run("unexpected object refuses v7 without writing a migration row", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		createExactV6PlatformExpensesSchema(t, ctx, db)
		if _, err := db.Exec(`CREATE TABLE unexpected_platform_cost_object(id INT PRIMARY KEY)`); err != nil {
			t.Fatal(err)
		}
		if _, err := ApplyPlatformExpensesV7(ctx, db); err == nil {
			t.Fatal("v7 unexpectedly accepted an unmanaged extra table")
		}
		if countWhere(t, db, "app_schema_migrations", "version = ?", PlatformExpensesV7Version) != 0 {
			t.Fatal("v7 ledger row was written after refusing an unmanaged extra object")
		}
		var expenseTables int
		if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='platform_expenses'`).Scan(&expenseTables); err != nil {
			t.Fatal(err)
		}
		if expenseTables != 0 {
			t.Fatal("v7 expense table was created after refusing an unmanaged extra object")
		}
	})

	t.Run("incomplete applying ledger remains fail closed", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		createExactV6PlatformExpensesSchema(t, ctx, db)
		if _, err := db.Exec(`INSERT INTO app_schema_migrations(version, checksum, state) VALUES(?, ?, 'applying')`, PlatformExpensesV7Version, PlatformExpensesV7Checksum); err != nil {
			t.Fatal(err)
		}
		if _, err := ApplyLatest(ctx, db); err == nil {
			t.Fatal("ApplyLatest unexpectedly repaired an incomplete v7 migration")
		}
		var expenseTables int
		if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='platform_expenses'`).Scan(&expenseTables); err != nil {
			t.Fatal(err)
		}
		if expenseTables != 0 {
			t.Fatal("incomplete v7 migration was automatically repaired")
		}
	})

	t.Run("v7 schema drift refuses latest verification", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		if _, err := ApplyLatest(ctx, db); err != nil {
			t.Fatalf("install latest schema: %v", err)
		}
		if _, err := db.Exec(`ALTER TABLE platform_expenses MODIFY COLUMN memo VARCHAR(120) NOT NULL DEFAULT ''`); err != nil {
			t.Fatalf("create synthetic v7 schema drift: %v", err)
		}
		if _, err := ApplyLatest(ctx, db); err == nil || !strings.Contains(err.Error(), "v7") {
			t.Fatalf("v7 schema drift was not refused: %v", err)
		}
	})
}

func createExactV6PlatformExpensesSchema(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	for _, apply := range []struct {
		name string
		fn   func(context.Context, *sql.DB) (Result, error)
	}{
		{"v1", ApplyCandidate},
		{"v2", ApplyManualReviewV2},
		{"v3", ApplyAdminAuthVersionV3},
		{"v4", ApplyOTPSafetyV4},
		{"v5", ApplyEmailOTPSafetyV5},
		{"v6", ApplyWholesaleRetailV6},
	} {
		if result, err := apply.fn(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("create exact %s schema: result=%q err=%v", apply.name, result, err)
		}
	}
}
