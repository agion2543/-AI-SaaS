package schema

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func TestCustomerProfilesV16DefinitionIsChecksumPinned(t *testing.T) {
	if err := validateCustomerProfilesV16Definition(); err != nil {
		t.Fatal(err)
	}
	if CustomerProfilesV16Version != "schema-v16-customer-profiles-20261128" {
		t.Fatalf("v16 version=%q", CustomerProfilesV16Version)
	}
	if len(customerProfilesV16Statements) != 1 ||
		!strings.Contains(customerProfilesV16Statements[0], "CREATE TABLE `customer_profiles`") ||
		!strings.Contains(customerProfilesV16Statements[0], "uk_customer_profiles_merchant_phone") {
		t.Fatalf("unexpected v16 DDL inventory: %v", customerProfilesV16Statements)
	}
	if !strings.Contains(customerProfilesV16Backfill, "INSERT INTO `customer_profiles`") ||
		!strings.Contains(customerProfilesV16Backfill, "GROUP_CONCAT") {
		t.Fatalf("unexpected v16 backfill: %s", customerProfilesV16Backfill)
	}
	if len(customerProfilesV16NewTables) != 1 || customerProfilesV16NewTables[0] != "customer_profiles" {
		t.Fatalf("unexpected v16 new tables: %v", customerProfilesV16NewTables)
	}
}

func TestMySQLCustomerProfilesV16Migration(t *testing.T) {
	control, baseConfig := openSafeMySQLTestServer(t)
	ctx := context.Background()

	t.Run("exact v15 upgrade creates and backfills customer profiles", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		createExactV15CustomerProfilesSchema(t, ctx, db)

		// Historical orders (v15 shape): two paid orders for one customer
		// at two stores, one unpaid order, one zero-amount order and one
		// non-store order — only the two paid store orders backfill.
		if _, err := db.Exec(`INSERT INTO merchants (id, name, created_at, updated_at) VALUES (1, 'V16 合成商户', NOW(3), NOW(3))`); err != nil {
			t.Fatalf("insert legacy v15 merchant: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO stores (id, merchant_id, name, created_at, updated_at) VALUES (1, 1, 'V16 老店', NOW(3), NOW(3)), (2, 1, 'V16 新店', NOW(3), NOW(3))`); err != nil {
			t.Fatalf("insert legacy v15 stores: %v", err)
		}
		seed := []struct {
			no, status string
			storeID    int
			amount     int64
			paid       string
			created    string
		}{
			{"V16-ORDER-1", "completed", 1, 6000, "2026-11-20 10:00:00.000", "2026-11-20 10:00:00.000"},
			{"V16-ORDER-2", "received", 2, 4000, "2026-11-21 11:00:00.000", "2026-11-21 11:00:00.000"},
			{"V16-ORDER-3", "pending", 2, 9000, "NULL", "2026-11-22 12:00:00.000"},
			{"V16-ORDER-4", "received", 2, 0, "2026-11-22 12:00:00.000", "2026-11-22 12:00:00.000"},
		}
		for _, o := range seed {
			paid := "NULL"
			if o.paid != "NULL" {
				paid = "'" + o.paid + "'"
			}
			if _, err := db.Exec(`INSERT INTO orders (order_no, order_type, merchant_id, store_id, customer_phone, amount, total_amount, discount_amount, status, paid_at, created_at, updated_at) VALUES (?, 'store_order', 1, ?, '13800000001', ?, ?, 500, ?, `+paid+`, ?, NOW(3))`,
				o.no, o.storeID, o.amount, o.amount, o.status, o.created); err != nil {
				t.Fatalf("insert legacy v15 order %s: %v", o.no, err)
			}
		}
		if _, err := db.Exec(`INSERT INTO orders (order_no, order_type, merchant_id, store_id, customer_phone, amount, total_amount, discount_amount, status, paid_at, created_at, updated_at) VALUES ('V16-ORDER-5', 'merchant_subscription', 1, 2, '13800000001', 19900, 19900, 0, 'paid', '2026-11-21 12:00:00.000', '2026-11-21 12:00:00.000', NOW(3))`); err != nil {
			t.Fatalf("insert legacy v15 subscription order: %v", err)
		}

		if result, err := ApplyCustomerProfilesV16(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("upgrade exact v15 to v16: result=%q err=%v", result, err)
		}
		var columns int
		if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='customer_profiles'`).Scan(&columns); err != nil {
			t.Fatal(err)
		}
		if columns != 15 {
			t.Fatalf("customer_profiles columns=%d, want 15", columns)
		}
		// Backfill equivalence against the old GROUP BY definition: two
		// paid store orders only (gross amounts, refund-blind), latest
		// order's store (V16 新店, not MAX name), latest status received.
		var merchantID int64
		var phone, storeName, lastStatus string
		var orderCount, total, discount int64
		var firstPaid, lastPaid, lastOrderAt string
		var lastOrderID int64
		if err := db.QueryRow(`SELECT merchant_id, customer_phone, last_store_name, order_count, total_amount_cents, discount_amount_cents, DATE_FORMAT(first_paid_at,'%Y-%m-%d %H:%i:%s'), DATE_FORMAT(last_paid_at,'%Y-%m-%d %H:%i:%s'), DATE_FORMAT(last_order_at,'%Y-%m-%d %H:%i:%s'), last_order_id, last_status FROM customer_profiles`).Scan(
			&merchantID, &phone, &storeName, &orderCount, &total, &discount, &firstPaid, &lastPaid, &lastOrderAt, &lastOrderID, &lastStatus); err != nil {
			t.Fatalf("read backfilled customer profile: %v", err)
		}
		if merchantID != 1 || phone != "13800000001" || storeName != "V16 新店" || orderCount != 2 || total != 10000 || discount != 1000 || lastStatus != "received" {
			t.Fatalf("backfilled profile mismatch: merchant=%d phone=%q store=%q count=%d total=%d discount=%d last_status=%q",
				merchantID, phone, storeName, orderCount, total, discount, lastStatus)
		}
		if firstPaid != "2026-11-20 10:00:00" || lastPaid != "2026-11-21 11:00:00" || lastOrderAt != "2026-11-21 11:00:00" {
			t.Fatalf("backfilled profile times first/last_paid/last_order=%s/%s/%s, want 2026-11-20 10:00:00 / 2026-11-21 11:00:00 / 2026-11-21 11:00:00",
				firstPaid, lastPaid, lastOrderAt)
		}
		var wantLastOrderID int64
		if err := db.QueryRow(`SELECT id FROM orders WHERE order_no='V16-ORDER-2'`).Scan(&wantLastOrderID); err != nil {
			t.Fatal(err)
		}
		if lastOrderID != wantLastOrderID {
			t.Fatalf("backfilled last_order_id=%d, want %d (V16-ORDER-2)", lastOrderID, wantLastOrderID)
		}
		// Backfill must match the old GROUP BY row-for-row (except the
		// deliberate store_name change, which the seed makes visible:
		// MAX(stores.name) would have been V16 老店).
		var oldCount, oldTotal, oldDiscount int64
		var oldFirstPaid, oldLastPaid, oldLastOrder, oldLastStatus string
		if err := db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(total_amount),0), COALESCE(SUM(discount_amount),0), DATE_FORMAT(MIN(paid_at),'%Y-%m-%d %H:%i:%s'), DATE_FORMAT(MAX(paid_at),'%Y-%m-%d %H:%i:%s'), DATE_FORMAT(MAX(created_at),'%Y-%m-%d %H:%i:%s'), SUBSTRING_INDEX(GROUP_CONCAT(status ORDER BY created_at DESC), ',', 1) FROM orders WHERE order_type='store_order' AND customer_phone<>'' AND paid_at IS NOT NULL AND total_amount > 0 GROUP BY merchant_id, customer_phone`).Scan(
			&oldCount, &oldTotal, &oldDiscount, &oldFirstPaid, &oldLastPaid, &oldLastOrder, &oldLastStatus); err != nil {
			t.Fatalf("read old GROUP BY profile: %v", err)
		}
		if oldCount != orderCount || oldTotal != total || oldDiscount != discount || oldFirstPaid != firstPaid || oldLastPaid != lastPaid || oldLastOrder != lastOrderAt || oldLastStatus != lastStatus {
			t.Fatalf("backfill disagrees with old GROUP BY: old=%d/%d/%d/%s/%s/%s/%s new=%d/%d/%d/%s/%s/%s/%s",
				oldCount, oldTotal, oldDiscount, oldFirstPaid, oldLastPaid, oldLastOrder, oldLastStatus,
				orderCount, total, discount, firstPaid, lastPaid, lastOrderAt, lastStatus)
		}
		if result, err := ApplyCustomerProfilesV16(ctx, db); err != nil || result != ResultAlready {
			t.Fatalf("repeat exact v16 migration: result=%q err=%v", result, err)
		}
		if result, err := ApplyLatest(ctx, db); err != nil || result != ResultAlready {
			t.Fatalf("latest verification after v16 upgrade: result=%q err=%v", result, err)
		}
		assertLatestInventory(t, db)
	})

	t.Run("v15 schema drift is refused without adding v16 table", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		createExactV15CustomerProfilesSchema(t, ctx, db)
		if _, err := db.Exec(`ALTER TABLE platform_expenses ADD COLUMN unexpected_v15_marker INT NULL`); err != nil {
			t.Fatalf("create synthetic v15 drift: %v", err)
		}
		before := snapshotDatabaseTables(t, db)
		if _, err := ApplyCustomerProfilesV16(ctx, db); err == nil {
			t.Fatal("v16 unexpectedly accepted a drifted v15 schema")
		}
		if after := snapshotDatabaseTables(t, db); after != before {
			t.Fatalf("v15 drift refusal mutated the database:\nBEFORE:\n%s\nAFTER:\n%s", before, after)
		}
		if countWhere(t, db, "app_schema_migrations", "version = ?", CustomerProfilesV16Version) != 0 {
			t.Fatal("v16 ledger row was written after refusing v15 drift")
		}
	})

	t.Run("non-exact v15 database is refused", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		createExactV15CustomerProfilesSchema(t, ctx, db)
		// A v15 ledger row with a tampered checksum is not an exact
		// managed v15 schema: the upgrade must fail closed.
		if _, err := db.Exec(`UPDATE app_schema_migrations SET checksum=REPEAT('0',64) WHERE version=?`, RefundReviewV15Version); err != nil {
			t.Fatal(err)
		}
		before := snapshotDatabaseTables(t, db)
		if _, err := ApplyCustomerProfilesV16(ctx, db); err == nil {
			t.Fatal("v16 unexpectedly accepted a non-exact v15 database")
		}
		if after := snapshotDatabaseTables(t, db); after != before {
			t.Fatalf("non-exact v15 refusal mutated the database:\nBEFORE:\n%s\nAFTER:\n%s", before, after)
		}
		if countWhere(t, db, "app_schema_migrations", "version = ?", CustomerProfilesV16Version) != 0 {
			t.Fatal("v16 ledger row was written after refusing a non-exact v15 database")
		}
	})
}

func createExactV15CustomerProfilesSchema(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	createExactV14RefundReviewSchema(t, ctx, db)
	if result, err := ApplyRefundReviewV15(ctx, db); err != nil || result != ResultApplied {
		t.Fatalf("create exact v15 schema: result=%q err=%v", result, err)
	}
}
