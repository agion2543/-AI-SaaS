package schema

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"
)

func TestOrderItemsV11DefinitionIsChecksumPinned(t *testing.T) {
	if err := validateOrderItemsV11Definition(); err != nil {
		t.Fatal(err)
	}
	if OrderItemsV11Version != "schema-v11-order-items-20261127" {
		t.Fatalf("v11 version=%q", OrderItemsV11Version)
	}
	if len(orderItemsV11Statements) != 2 ||
		!strings.Contains(orderItemsV11Statements[0], "CREATE TABLE `order_items`") ||
		!strings.Contains(orderItemsV11Statements[0], "`product_name_snapshot` varchar(128)") ||
		!strings.Contains(orderItemsV11Statements[1], "ALTER TABLE `orders` ADD COLUMN `channel` varchar(24)") {
		t.Fatalf("unexpected v11 DDL inventory: %v", orderItemsV11Statements)
	}
	if !strings.Contains(orderItemsV11TableBackfill, "JOIN `table_order_details`") ||
		!strings.Contains(orderItemsV11TableBackfill, "SET o.`channel` = 'table'") {
		t.Fatalf("unexpected v11 table-order backfill: %q", orderItemsV11TableBackfill)
	}
}

func TestMySQLOrderItemsV11Migration(t *testing.T) {
	control, baseConfig := openSafeMySQLTestServer(t)
	ctx := context.Background()

	t.Run("exact v10 upgrade backfills table orders and leaves store orders alone", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		createExactV10OrderItemsSchema(t, ctx, db)
		merchantID, subscriptionOrderID, _ := insertSyntheticV1MerchantOrder(t, db)
		storeResult, err := db.Exec(`INSERT INTO stores(created_at, updated_at, merchant_id, name, status, is_open, order_mode, auto_accept) VALUES(?, ?, ?, 'synthetic-v11-store', 'active', 1, 'pay_first', 0)`, time.Now(), time.Now(), merchantID)
		if err != nil {
			t.Fatalf("insert synthetic store: %v", err)
		}
		storeID, err := storeResult.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO store_table_configs(created_at, updated_at, store_id, table_count) VALUES(?, ?, ?, 1)`, time.Now(), time.Now(), storeID); err != nil {
			t.Fatalf("insert synthetic table config: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO store_tables(created_at, updated_at, store_id, table_no, table_code) VALUES(?, ?, ?, 1, 'synthetic-v11-table-code')`, time.Now(), time.Now(), storeID); err != nil {
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
		tableOrderResult, err := db.Exec(`INSERT INTO orders(order_no, order_type, merchant_id, store_id, amount, total_amount, status) VALUES(?, 'store_order', ?, ?, 1200, 1200, 'pending')`, "SYNTHETIC-V11-TABLE-ORDER", merchantID, storeID)
		if err != nil {
			t.Fatalf("insert synthetic pre-v11 table order: %v", err)
		}
		tableOrderID, err := tableOrderResult.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO table_order_details(created_at, updated_at, order_id, session_id, table_no) VALUES(?, ?, ?, ?, 1)`, time.Now(), time.Now(), tableOrderID, sessionID); err != nil {
			t.Fatalf("insert synthetic pre-v11 order detail: %v", err)
		}
		storeOrderResult, err := db.Exec(`INSERT INTO orders(order_no, order_type, merchant_id, store_id, amount, total_amount, status) VALUES(?, 'store_order', ?, ?, 800, 800, 'pending')`, "SYNTHETIC-V11-STORE-ORDER", merchantID, storeID)
		if err != nil {
			t.Fatalf("insert synthetic pre-v11 store order: %v", err)
		}
		storeOrderID, err := storeOrderResult.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		_ = subscriptionOrderID

		if result, err := ApplyLatest(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("upgrade exact v10 to v11: result=%q err=%v", result, err)
		}
		var tableChannel, storeChannel, subscriptionChannel string
		if err := db.QueryRow(`SELECT channel FROM orders WHERE id=?`, tableOrderID).Scan(&tableChannel); err != nil {
			t.Fatalf("read backfilled table order channel: %v", err)
		}
		if tableChannel != "table" {
			t.Fatalf("table order channel=%q, want %q", tableChannel, "table")
		}
		if err := db.QueryRow(`SELECT channel FROM orders WHERE id=?`, storeOrderID).Scan(&storeChannel); err != nil {
			t.Fatalf("read store order channel: %v", err)
		}
		if storeChannel != "store_qr" {
			t.Fatalf("store order channel=%q, want %q", storeChannel, "store_qr")
		}
		if err := db.QueryRow(`SELECT channel FROM orders WHERE id=?`, subscriptionOrderID).Scan(&subscriptionChannel); err != nil {
			t.Fatalf("read subscription order channel: %v", err)
		}
		if subscriptionChannel != "store_qr" {
			t.Fatalf("subscription order channel=%q, want %q", subscriptionChannel, "store_qr")
		}
		var itemColumns int
		if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='order_items'`).Scan(&itemColumns); err != nil {
			t.Fatal(err)
		}
		if itemColumns != 9 {
			t.Fatalf("order_items columns=%d, want 9", itemColumns)
		}
		// ApplyLatest now continues to v13, so re-applying v11 against a v13
		// schema must be refused fail-closed (it is no longer an exact v11 schema).
		if _, err := ApplyOrderItemsV11(ctx, db); err == nil {
			t.Fatal("v11 migration unexpectedly accepted a v13 schema")
		}
		if result, err := ApplyLatest(ctx, db); err != nil || result != ResultAlready {
			t.Fatalf("latest verification after v13 upgrade: result=%q err=%v", result, err)
		}
		assertLatestInventory(t, db)
	})

	t.Run("v10 schema drift is refused without adding order items", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		createExactV10OrderItemsSchema(t, ctx, db)
		if _, err := db.Exec(`ALTER TABLE orders ADD COLUMN unexpected_v10_marker INT NULL`); err != nil {
			t.Fatalf("create synthetic v10 drift: %v", err)
		}
		before := snapshotDatabaseTables(t, db)
		if _, err := ApplyOrderItemsV11(ctx, db); err == nil {
			t.Fatal("v11 unexpectedly accepted a drifted v10 schema")
		}
		if after := snapshotDatabaseTables(t, db); after != before {
			t.Fatalf("v11 drift refusal mutated the database:\nBEFORE:\n%s\nAFTER:\n%s", before, after)
		}
		if countWhere(t, db, "app_schema_migrations", "version = ?", OrderItemsV11Version) != 0 {
			t.Fatal("v11 ledger row was written after refusing v10 drift")
		}
	})
}

func createExactV10OrderItemsSchema(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	createExactV9TableLabelsSchema(t, ctx, db)
	if result, err := ApplyTableLabelsV10(ctx, db); err != nil || result != ResultApplied {
		t.Fatalf("create exact v10 schema: result=%q err=%v", result, err)
	}
}
