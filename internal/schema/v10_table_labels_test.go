package schema

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"
)

func TestTableLabelsV10DefinitionIsChecksumPinned(t *testing.T) {
	if err := validateTableLabelsV10Definition(); err != nil {
		t.Fatal(err)
	}
	if TableLabelsV10Version != "schema-v10-merchant-table-labels-20261124" {
		t.Fatalf("v10 version=%q", TableLabelsV10Version)
	}
	if len(tableLabelsV10Statements) != 2 ||
		!strings.Contains(tableLabelsV10Statements[0], "store_tables` ADD COLUMN `table_label` varchar(64)") ||
		!strings.Contains(tableLabelsV10Statements[1], "table_order_details` ADD COLUMN `table_label_snapshot` varchar(64)") {
		t.Fatalf("unexpected v10 DDL inventory: %v", tableLabelsV10Statements)
	}
}

func TestMySQLTableLabelsV10Migration(t *testing.T) {
	control, baseConfig := openSafeMySQLTestServer(t)
	ctx := context.Background()

	t.Run("exact v9 upgrade preserves old order details without backfill", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		createExactV9TableLabelsSchema(t, ctx, db)
		merchantID, orderID, _ := insertSyntheticV1MerchantOrder(t, db)
		storeResult, err := db.Exec(`INSERT INTO stores(created_at, updated_at, merchant_id, name, status, is_open, order_mode, auto_accept) VALUES(?, ?, ?, 'synthetic-v10-store', 'active', 1, 'pay_first', 0)`, time.Now(), time.Now(), merchantID)
		if err != nil {
			t.Fatalf("insert synthetic store: %v", err)
		}
		storeID, err := storeResult.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec(`INSERT INTO store_table_configs(created_at, updated_at, store_id, table_count) VALUES(?, ?, ?, 1)`, time.Now(), time.Now(), storeID)
		if err != nil {
			t.Fatalf("insert synthetic table config: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO store_tables(created_at, updated_at, store_id, table_no, table_code) VALUES(?, ?, ?, 1, 'synthetic-v10-table-code')`, time.Now(), time.Now(), storeID); err != nil {
			t.Fatalf("insert synthetic table code: %v", err)
		}
		sessionResult, err := db.Exec(`INSERT INTO table_sessions(created_at, updated_at, store_id, table_no, status, cart_version) VALUES(?, ?, ?, 1, 'open', 0)`, time.Now(), time.Now(), storeID)
		if err != nil {
			t.Fatalf("insert synthetic table session: %v", err)
		}
		sessionID, err := sessionResult.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO table_order_details(created_at, updated_at, order_id, session_id, table_no) VALUES(?, ?, ?, ?, 1)`, time.Now(), time.Now(), orderID, sessionID); err != nil {
			t.Fatalf("insert synthetic pre-v10 order detail: %v", err)
		}

		if result, err := ApplyTableLabelsV10(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("upgrade exact v9 to v10: result=%q err=%v", result, err)
		}
		var storedLabel string
		var snapshot sql.NullString
		if err := db.QueryRow(`SELECT table_label FROM store_tables WHERE store_id=? AND table_no=1`, storeID).Scan(&storedLabel); err != nil {
			t.Fatalf("read new merchant label: %v", err)
		}
		if storedLabel != "" {
			t.Fatalf("pre-existing table label=%q, want empty default", storedLabel)
		}
		if err := db.QueryRow(`SELECT table_label_snapshot FROM table_order_details WHERE order_id=?`, orderID).Scan(&snapshot); err != nil {
			t.Fatalf("read legacy order snapshot: %v", err)
		}
		if snapshot.Valid {
			t.Fatalf("v10 backfilled a historical order label snapshot: %q", snapshot.String)
		}
		if result, err := ApplyTableLabelsV10(ctx, db); err != nil || result != ResultAlready {
			t.Fatalf("repeat exact v10 migration: result=%q err=%v", result, err)
		}
		if result, err := ApplyLatest(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("upgrade exact v10 to v11: result=%q err=%v", result, err)
		}
		if result, err := ApplyLatest(ctx, db); err != nil || result != ResultAlready {
			t.Fatalf("latest verification after v10 upgrade: result=%q err=%v", result, err)
		}
		assertLatestInventory(t, db)
	})

	t.Run("v9 schema drift is refused without adding v10 columns", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		createExactV9TableLabelsSchema(t, ctx, db)
		if _, err := db.Exec(`ALTER TABLE store_tables ADD COLUMN unexpected_v9_marker INT NULL`); err != nil {
			t.Fatalf("create synthetic v9 drift: %v", err)
		}
		before := snapshotDatabaseTables(t, db)
		if _, err := ApplyTableLabelsV10(ctx, db); err == nil {
			t.Fatal("v10 unexpectedly accepted a drifted v9 schema")
		}
		if after := snapshotDatabaseTables(t, db); after != before {
			t.Fatalf("v10 drift refusal mutated the database:\nBEFORE:\n%s\nAFTER:\n%s", before, after)
		}
		if countWhere(t, db, "app_schema_migrations", "version = ?", TableLabelsV10Version) != 0 {
			t.Fatal("v10 ledger row was written after refusing v9 drift")
		}
	})
}

func createExactV9TableLabelsSchema(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	createExactV6PlatformExpensesSchema(t, ctx, db)
	for _, apply := range []struct {
		name string
		fn   func(context.Context, *sql.DB) (Result, error)
	}{
		{"v7", ApplyPlatformExpensesV7},
		{"v8", ApplyPlatformExpenseReversalsV8},
		{"v9", ApplyTableOrderingV9},
	} {
		if result, err := apply.fn(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("create exact %s schema: result=%q err=%v", apply.name, result, err)
		}
	}
}
