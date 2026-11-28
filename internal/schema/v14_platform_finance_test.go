package schema

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func TestPlatformFinanceV14DefinitionIsChecksumPinned(t *testing.T) {
	if err := validatePlatformFinanceV14Definition(); err != nil {
		t.Fatal(err)
	}
	if PlatformFinanceV14Version != "schema-v14-platform-finance-20261128" {
		t.Fatalf("v14 version=%q", PlatformFinanceV14Version)
	}
	if len(platformFinanceV14Statements) != 10 ||
		!strings.Contains(platformFinanceV14Statements[0], "DROP CHECK `chk_platform_expenses_category`") ||
		!strings.Contains(platformFinanceV14Statements[1], "ADD CONSTRAINT `chk_platform_expenses_category`") ||
		!strings.Contains(platformFinanceV14Statements[1], "'ai_services','third_party_services'") ||
		!strings.Contains(platformFinanceV14Statements[2], "ADD COLUMN `cost_type`") ||
		!strings.Contains(platformFinanceV14Statements[8], "CREATE TABLE `platform_time_logs`") ||
		!strings.Contains(platformFinanceV14Statements[9], "CREATE TABLE `platform_income_records`") {
		t.Fatalf("unexpected v14 DDL inventory: %v", platformFinanceV14Statements)
	}
	if len(platformFinanceV14NewTables) != 2 || len(platformFinanceV14NewColumns) != 6 {
		t.Fatalf("unexpected v14 new tables/columns: %v / %v", platformFinanceV14NewTables, platformFinanceV14NewColumns)
	}
}

func TestMySQLPlatformFinanceV14Migration(t *testing.T) {
	control, baseConfig := openSafeMySQLTestServer(t)
	ctx := context.Background()

	t.Run("exact v13 upgrade widens expense categories and creates finance tables", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		createExactV13PlatformFinanceSchema(t, ctx, db)

		// A pre-v14 expense keeps its legacy defaults after the upgrade.
		if _, err := db.Exec(`INSERT INTO platform_expenses (created_at, updated_at, category, amount_cents, currency, expense_date, memo, created_by, idempotency_key) VALUES (NOW(3), NOW(3), 'cloud_hosting', 1000, 'CNY', '2026-11-20', 'legacy expense', 1, UUID())`); err != nil {
			t.Fatalf("insert legacy v13 expense: %v", err)
		}

		if result, err := ApplyLatest(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("upgrade exact v13 to v14: result=%q err=%v", result, err)
		}
		var expenseColumns, timeLogColumns, incomeRecordColumns int
		for _, check := range []struct {
			table string
			want  int
			into  *int
		}{
			{"platform_expenses", 16, &expenseColumns},
			{"platform_time_logs", 12, &timeLogColumns},
			{"platform_income_records", 20, &incomeRecordColumns},
		} {
			if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=?`, check.table).Scan(check.into); err != nil {
				t.Fatal(err)
			}
			if *check.into != check.want {
				t.Fatalf("%s columns=%d, want %d", check.table, *check.into, check.want)
			}
		}
		// Legacy row: cost_type/vendor untouched, paid + both cost flags on.
		var legacyCostType, legacyVendor, legacyPaymentStatus string
		var legacyRecurring, legacyCash, legacyEconomic int
		if err := db.QueryRow(`SELECT cost_type, vendor, payment_status, is_recurring, is_cash_cost, is_economic_cost FROM platform_expenses WHERE category='cloud_hosting'`).Scan(&legacyCostType, &legacyVendor, &legacyPaymentStatus, &legacyRecurring, &legacyCash, &legacyEconomic); err != nil {
			t.Fatalf("read legacy expense after v14: %v", err)
		}
		if legacyCostType != "" || legacyVendor != "" || legacyPaymentStatus != "paid" || legacyRecurring != 0 || legacyCash != 1 || legacyEconomic != 1 {
			t.Fatalf("legacy expense defaults changed: cost_type=%q vendor=%q payment_status=%q recurring=%d cash=%d economic=%d", legacyCostType, legacyVendor, legacyPaymentStatus, legacyRecurring, legacyCash, legacyEconomic)
		}
		// The widened category CHECK accepts the two new categories and
		// still rejects an unknown one.
		if _, err := db.Exec(`INSERT INTO platform_expenses (created_at, updated_at, category, amount_cents, currency, expense_date, memo, created_by, idempotency_key, cost_type, vendor, payment_status, is_recurring, is_cash_cost, is_economic_cost) VALUES (NOW(3), NOW(3), 'ai_services', 500, 'CNY', '2026-11-28', 'ai services', 1, UUID(), 'variable', 'TestVendor', 'paid', 0, 1, 1)`); err != nil {
			t.Fatalf("insert ai_services expense: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO platform_expenses (created_at, updated_at, category, amount_cents, currency, expense_date, memo, created_by, idempotency_key) VALUES (NOW(3), NOW(3), 'third_party_services', 700, 'CNY', '2026-11-28', 'third party', 1, UUID())`); err != nil {
			t.Fatalf("insert third_party_services expense: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO platform_expenses (created_at, updated_at, category, amount_cents, currency, expense_date, memo, created_by, idempotency_key) VALUES (NOW(3), NOW(3), 'not_a_category', 700, 'CNY', '2026-11-28', 'bad category', 1, UUID())`); err == nil {
			t.Fatal("widened category CHECK accepted an unknown category")
		}
		// A time log and an income record insert under the new CHECKs.
		if _, err := db.Exec(`INSERT INTO platform_time_logs (created_at, updated_at, log_date, work_type, content, hours_scaled, reference_hourly_value_cents, amount_cents, module, created_by, idempotency_key) VALUES (NOW(3), NOW(3), '2026-11-28', 'development', 'cost centre v14', 150, 10000, 15000, 'cost-centre', 1, UUID())`); err != nil {
			t.Fatalf("insert time log: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO platform_income_records (created_at, updated_at, income_type, amount_cents, currency, occurred_date, status, billing_period_months, subscription_start_date, subscription_end_date, created_by, idempotency_key) VALUES (NOW(3), NOW(3), 'subscription_basic', 19900, 'CNY', '2026-11-28', 'received', 1, '2026-11-01', '2026-11-30', 1, UUID())`); err != nil {
			t.Fatalf("insert income record: %v", err)
		}
		if result, err := ApplyPlatformFinanceV14(ctx, db); err != nil || result != ResultAlready {
			t.Fatalf("repeat exact v14 migration: result=%q err=%v", result, err)
		}
		if result, err := ApplyLatest(ctx, db); err != nil || result != ResultAlready {
			t.Fatalf("latest verification after v14 upgrade: result=%q err=%v", result, err)
		}
		assertLatestInventory(t, db)
	})

	t.Run("v13 schema drift is refused without adding v14 tables", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		createExactV13PlatformFinanceSchema(t, ctx, db)
		if _, err := db.Exec(`ALTER TABLE ai_model_prices ADD COLUMN unexpected_v13_marker INT NULL`); err != nil {
			t.Fatalf("create synthetic v13 drift: %v", err)
		}
		before := snapshotDatabaseTables(t, db)
		if _, err := ApplyPlatformFinanceV14(ctx, db); err == nil {
			t.Fatal("v14 unexpectedly accepted a drifted v13 schema")
		}
		if after := snapshotDatabaseTables(t, db); after != before {
			t.Fatalf("v14 drift refusal mutated the database:\nBEFORE:\n%s\nAFTER:\n%s", before, after)
		}
		if countWhere(t, db, "app_schema_migrations", "version = ?", PlatformFinanceV14Version) != 0 {
			t.Fatal("v14 ledger row was written after refusing v13 drift")
		}
	})
}

func createExactV13PlatformFinanceSchema(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	createExactV12AnnouncementsMaintenanceSchema(t, ctx, db)
	if result, err := ApplyAnnouncementsMaintenanceV13(ctx, db); err != nil || result != ResultApplied {
		t.Fatalf("create exact v13 schema: result=%q err=%v", result, err)
	}
}
