package schema

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestTableOrderingV9DefinitionIsChecksumPinned(t *testing.T) {
	definition, err := loadTableOrderingV9()
	if err != nil {
		t.Fatalf("load v9 table-ordering definition: %v", err)
	}
	if definition.version != TableOrderingV9Version || definition.checksum != TableOrderingV9Checksum {
		t.Fatalf("loaded v9 identity=%q/%q, want %q/%q", definition.version, definition.checksum, TableOrderingV9Version, TableOrderingV9Checksum)
	}
	if len(definition.tables) != 5 {
		t.Fatalf("v9 table count=%d, want 5", len(definition.tables))
	}
	for _, fragment := range []string{"FOREIGN KEY", "CHECK", "ON DELETE CASCADE", "ON DELETE RESTRICT"} {
		found := false
		for _, statement := range definition.tables {
			if strings.Contains(statement, fragment) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("v9 DDL inventory is missing %q", fragment)
		}
	}
}

func TestNormalizeCheckClauseMySQLMetadataRendering(t *testing.T) {
	got := normalizeCheckClause(`(status IN (_utf8mb4\'open\', _utf8mb4\'checkout_locked\'))`)
	want := `statusin'open','checkout_locked'`
	if got != want {
		t.Fatalf("normalized MySQL CHECK clause=%q, want %q", got, want)
	}
}

func TestMySQLTableOrderingV9Migration(t *testing.T) {
	control, baseConfig := openSafeMySQLTestServer(t)
	ctx := context.Background()

	t.Run("exact v8 upgrades through v10 and preserves existing rows", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		createExactV6PlatformExpensesSchema(t, ctx, db)
		if result, err := ApplyPlatformExpensesV7(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("apply v7: result=%q err=%v", result, err)
		}
		if result, err := ApplyPlatformExpenseReversalsV8(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("apply v8: result=%q err=%v", result, err)
		}
		merchantID, orderID, orderNo := insertSyntheticV1MerchantOrder(t, db)

		if result, err := ApplyLatest(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("upgrade exact v8 through v10: result=%q err=%v", result, err)
		}
		if got := countWhere(t, db, "orders", "id = ? AND order_no = ?", orderID, orderNo); got != 1 {
			t.Fatalf("existing v1 order rows after v9=%d, want 1", got)
		}
		if got := countWhere(t, db, "merchants", "id = ?", merchantID); got != 1 {
			t.Fatalf("existing merchant rows after v9=%d, want 1", got)
		}
		for _, table := range tableOrderingV9Order {
			if got := countWhere(t, db, table, "1=1"); got != 0 {
				t.Fatalf("new v9 table %s contains seeded rows: %d", table, got)
			}
		}
		if got := countWhere(t, db, "app_schema_migrations", "version = ? AND checksum = ? AND state = 'applied' AND schema_fingerprint IS NOT NULL", TableOrderingV9Version, TableOrderingV9Checksum); got != 1 {
			t.Fatalf("v9 applied ledger rows=%d, want 1", got)
		}
		if result, err := ApplyLatest(ctx, db); err != nil || result != ResultAlready {
			t.Fatalf("repeat latest migration: result=%q err=%v", result, err)
		}
		assertLatestInventory(t, db)

		storeResult, err := db.Exec(`INSERT INTO stores(created_at, updated_at, merchant_id, name, status, is_open, order_mode, auto_accept) VALUES(?, ?, ?, 'synthetic-v9-store', 'active', 1, 'pay_first', 0)`, time.Now(), time.Now(), merchantID)
		if err != nil {
			t.Fatalf("insert synthetic v9 store: %v", err)
		}
		storeID, err := storeResult.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO store_table_configs(created_at, updated_at, store_id, table_count) VALUES(?, ?, ?, 2)`, time.Now(), time.Now(), storeID); err != nil {
			t.Fatalf("insert valid table configuration: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO store_tables(created_at, updated_at, store_id, table_no, table_code) VALUES(?, ?, ?, 1, 'synthetic-table-code-v9')`, time.Now(), time.Now(), storeID); err != nil {
			t.Fatalf("insert valid table code: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO store_tables(created_at, updated_at, store_id, table_no, table_code) VALUES(?, ?, ?, 2, 'synthetic-table-code-v9-2')`, time.Now(), time.Now(), storeID); err != nil {
			t.Fatalf("insert second valid table code: %v", err)
		}
		tableResult, err := db.Exec(`INSERT INTO table_sessions(created_at, updated_at, store_id, table_no, status, cart_version) VALUES(?, ?, ?, 1, 'open', 0)`, time.Now(), time.Now(), storeID)
		if err != nil {
			t.Fatalf("insert valid shared table session: %v", err)
		}
		sessionID, err := tableResult.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO table_order_details(created_at, updated_at, order_id, session_id, table_no) VALUES(?, ?, ?, ?, 1)`, time.Now(), time.Now(), orderID, sessionID); err != nil {
			t.Fatalf("insert valid order-to-table detail: %v", err)
		}
		var tableNo int
		if err := db.QueryRow(`SELECT table_no FROM table_order_details WHERE order_id = ?`, orderID).Scan(&tableNo); err != nil || tableNo != 1 {
			t.Fatalf("read synthetic order table number=%d err=%v, want 1", tableNo, err)
		}
		for _, invalid := range []struct {
			label string
			query string
			args  []any
		}{
			{"table count above limit", `UPDATE store_table_configs SET table_count=301 WHERE store_id=?`, []any{storeID}},
			{"table number below limit", `INSERT INTO store_tables(created_at, updated_at, store_id, table_no, table_code) VALUES(?, ?, ?, 0, 'synthetic-invalid-table-code-v9')`, []any{time.Now(), time.Now(), storeID}},
			{"invalid session state", `INSERT INTO table_sessions(created_at, updated_at, store_id, table_no, status, cart_version) VALUES(?, ?, ?, 2, 'invalid', 0)`, []any{time.Now(), time.Now(), storeID}},
		} {
			if _, err := db.Exec(invalid.query, invalid.args...); err == nil {
				t.Errorf("v9 database constraints allowed %s", invalid.label)
			}
		}
	})

	t.Run("v9 exact structure drift is refused without repair", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		if result, err := ApplyLatest(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("install latest v10: result=%q err=%v", result, err)
		}
		if _, err := db.Exec(`ALTER TABLE store_tables ADD COLUMN unexpected_v9_marker INT NULL`); err != nil {
			t.Fatalf("create synthetic v9 schema drift: %v", err)
		}
		before := snapshotDatabaseTables(t, db)
		if _, err := ApplyLatest(ctx, db); err == nil {
			t.Fatal("ApplyLatest unexpectedly accepted a v9 schema drift")
		}
		if after := snapshotDatabaseTables(t, db); after != before {
			t.Fatalf("v9 drift refusal mutated database:\nBEFORE:\n%s\nAFTER:\n%s", before, after)
		}
		if got := countWhere(t, db, "app_schema_migrations", "version = ?", TableOrderingV9Version); got != 1 {
			t.Fatalf("drift refusal changed v9 ledger row count to %d", got)
		}
	})

	t.Run("v9 checksum drift is refused without mutation", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		if _, err := ApplyLatest(ctx, db); err != nil {
			t.Fatalf("install latest v10: %v", err)
		}
		if _, err := db.Exec(`UPDATE app_schema_migrations SET checksum=REPEAT('0',64) WHERE version=?`, TableOrderingV9Version); err != nil {
			t.Fatal(err)
		}
		before := snapshotDatabaseTables(t, db)
		if _, err := ApplyLatest(ctx, db); err == nil {
			t.Fatal("ApplyLatest unexpectedly accepted a v9 checksum mismatch")
		}
		if after := snapshotDatabaseTables(t, db); after != before {
			t.Fatalf("v9 checksum refusal mutated database:\nBEFORE:\n%s\nAFTER:\n%s", before, after)
		}
	})
}
