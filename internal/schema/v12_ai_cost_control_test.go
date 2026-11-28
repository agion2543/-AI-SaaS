package schema

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func TestAICostControlV12DefinitionIsChecksumPinned(t *testing.T) {
	if err := validateAICostControlV12Definition(); err != nil {
		t.Fatal(err)
	}
	if AICostControlV12Version != "schema-v12-ai-cost-control-20261127" {
		t.Fatalf("v12 version=%q", AICostControlV12Version)
	}
	if len(aiCostControlV12Statements) != 8 ||
		!strings.Contains(aiCostControlV12Statements[0], "CREATE TABLE `ai_model_prices`") ||
		!strings.Contains(aiCostControlV12Statements[1], "CREATE TABLE `ai_vendor_bills`") ||
		!strings.Contains(aiCostControlV12Statements[2], "CREATE TABLE `merchant_ai_budgets`") ||
		!strings.Contains(aiCostControlV12Statements[3], "CREATE TABLE `merchant_ai_budget_alerts`") ||
		!strings.Contains(aiCostControlV12Statements[4], "CREATE TABLE `ai_emergency_switch`") ||
		!strings.Contains(aiCostControlV12Statements[5], "ALTER TABLE `merchant_ai_usage_logs` ADD COLUMN `store_id`") ||
		!strings.Contains(aiCostControlV12Statements[6], "ALTER TABLE `merchant_ai_usage_logs` ADD COLUMN `cached_tokens`") ||
		!strings.Contains(aiCostControlV12Statements[7], "INSERT INTO `ai_emergency_switch`") {
		t.Fatalf("unexpected v12 DDL inventory: %v", aiCostControlV12Statements)
	}
	if len(aiCostControlV12NewTables) != 5 || len(aiCostControlV12NewColumns) != 2 {
		t.Fatalf("unexpected v12 new tables/columns: %v / %v", aiCostControlV12NewTables, aiCostControlV12NewColumns)
	}
}

func TestMySQLAICostControlV12Migration(t *testing.T) {
	control, baseConfig := openSafeMySQLTestServer(t)
	ctx := context.Background()

	t.Run("exact v11 upgrade creates cost control tables and usage log columns", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		createExactV11AICostControlSchema(t, ctx, db)

		if result, err := ApplyAICostControlV12(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("upgrade exact v11 to v12: result=%q err=%v", result, err)
		}
		var modelPriceColumns, vendorBillColumns, budgetColumns, alertColumns, switchColumns, usageLogNewColumns int
		for _, check := range []struct {
			table string
			want  int
			into  *int
		}{
			{"ai_model_prices", 13, &modelPriceColumns},
			{"ai_vendor_bills", 11, &vendorBillColumns},
			{"merchant_ai_budgets", 8, &budgetColumns},
			{"merchant_ai_budget_alerts", 9, &alertColumns},
			{"ai_emergency_switch", 6, &switchColumns},
		} {
			if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=?`, check.table).Scan(check.into); err != nil {
				t.Fatal(err)
			}
			if *check.into != check.want {
				t.Fatalf("%s columns=%d, want %d", check.table, *check.into, check.want)
			}
		}
		if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='merchant_ai_usage_logs' AND COLUMN_NAME IN ('store_id', 'cached_tokens')`).Scan(&usageLogNewColumns); err != nil {
			t.Fatal(err)
		}
		if usageLogNewColumns != 2 {
			t.Fatalf("merchant_ai_usage_logs new columns=%d, want 2", usageLogNewColumns)
		}
		var seedRows int
		if err := db.QueryRow(`SELECT COUNT(*) FROM ai_emergency_switch WHERE id = 1 AND disabled = 0`).Scan(&seedRows); err != nil {
			t.Fatalf("read emergency switch seed row: %v", err)
		}
		if seedRows != 1 {
			t.Fatalf("emergency switch seed rows=%d, want 1", seedRows)
		}
		if result, err := ApplyAICostControlV12(ctx, db); err != nil || result != ResultAlready {
			t.Fatalf("repeat exact v12 migration: result=%q err=%v", result, err)
		}
		if result, err := ApplyLatest(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("latest applies v13 after v12 upgrade: result=%q err=%v", result, err)
		}
		// v17 is the current latest, so ApplyLatest first upgrades this
		// exact v12 schema to latest, and only then reports Already.
		if result, err := ApplyLatest(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("upgrade exact v12 to latest: result=%q err=%v", result, err)
		}
		if result, err := ApplyLatest(ctx, db); err != nil || result != ResultAlready {
			t.Fatalf("latest verification after v12 upgrade: result=%q err=%v", result, err)
		}
		assertLatestInventory(t, db)
	})

	t.Run("v11 schema drift is refused without adding cost control tables", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		createExactV11AICostControlSchema(t, ctx, db)
		if _, err := db.Exec(`ALTER TABLE merchant_ai_usage_logs ADD COLUMN unexpected_v11_marker INT NULL`); err != nil {
			t.Fatalf("create synthetic v11 drift: %v", err)
		}
		before := snapshotDatabaseTables(t, db)
		if _, err := ApplyAICostControlV12(ctx, db); err == nil {
			t.Fatal("v12 unexpectedly accepted a drifted v11 schema")
		}
		if after := snapshotDatabaseTables(t, db); after != before {
			t.Fatalf("v12 drift refusal mutated the database:\nBEFORE:\n%s\nAFTER:\n%s", before, after)
		}
		if countWhere(t, db, "app_schema_migrations", "version = ?", AICostControlV12Version) != 0 {
			t.Fatal("v12 ledger row was written after refusing v11 drift")
		}
	})
}

func createExactV11AICostControlSchema(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	createExactV10OrderItemsSchema(t, ctx, db)
	if result, err := ApplyOrderItemsV11(ctx, db); err != nil || result != ResultApplied {
		t.Fatalf("create exact v11 schema: result=%q err=%v", result, err)
	}
}
