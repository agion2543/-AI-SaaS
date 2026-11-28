package schema

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func TestSupportTicketsV18DefinitionIsChecksumPinned(t *testing.T) {
	if err := validateSupportTicketsV18Definition(); err != nil {
		t.Fatal(err)
	}
	if SupportTicketsV18Version != "schema-v18-support-tickets-20261128" {
		t.Fatalf("v18 version=%q", SupportTicketsV18Version)
	}
	if len(supportTicketsV18Statements) != 1 ||
		!strings.Contains(supportTicketsV18Statements[0], "CREATE TABLE `support_tickets`") ||
		!strings.Contains(supportTicketsV18Statements[0], "idx_support_tickets_merchant_id") ||
		!strings.Contains(supportTicketsV18Statements[0], "idx_support_tickets_status") {
		t.Fatalf("unexpected v18 DDL inventory: %v", supportTicketsV18Statements)
	}
	if len(supportTicketsV18NewTables) != 1 || supportTicketsV18NewTables[0] != "support_tickets" {
		t.Fatalf("unexpected v18 new tables: %v", supportTicketsV18NewTables)
	}
}

func TestMySQLSupportTicketsV18Migration(t *testing.T) {
	control, baseConfig := openSafeMySQLTestServer(t)
	ctx := context.Background()

	t.Run("exact v17 upgrade creates the support tickets table", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		createExactV17SupportTicketsSchema(t, ctx, db)

		if result, err := ApplySupportTicketsV18(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("upgrade exact v17 to v18: result=%q err=%v", result, err)
		}
		var columns int
		if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='support_tickets'`).Scan(&columns); err != nil {
			t.Fatal(err)
		}
		if columns != 11 {
			t.Fatalf("support_tickets columns=%d, want 11", columns)
		}
		// A ticket can be recorded with the v18 shape: open by default,
		// empty reply, reviewer 0, NULL reply time.
		if _, err := db.Exec(`INSERT INTO merchants (id, name, created_at, updated_at) VALUES (1, 'V18 工单商家', NOW(3), NOW(3))`); err != nil {
			t.Fatalf("insert legacy v17 merchant: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO support_tickets (merchant_id, title, content, category, created_at, updated_at) VALUES (1, '订单无法确认收款', '顾客已付款但订单仍显示待确认', 'order', NOW(3), NOW(3))`); err != nil {
			t.Fatalf("insert synthetic v18 ticket: %v", err)
		}
		var status, reply string
		var repliedBy int64
		var repliedNotNull int
		if err := db.QueryRow(`SELECT status, reply, replied_by, replied_at IS NOT NULL FROM support_tickets WHERE merchant_id=1`).Scan(&status, &reply, &repliedBy, &repliedNotNull); err != nil {
			t.Fatal(err)
		}
		if status != "open" || reply != "" || repliedBy != 0 || repliedNotNull != 0 {
			t.Fatalf("new ticket defaults status/reply/replied_by/replied_at=%q/%q/%d/%d, want open/\"\"/0/NULL", status, reply, repliedBy, repliedNotNull)
		}
		if result, err := ApplySupportTicketsV18(ctx, db); err != nil || result != ResultAlready {
			t.Fatalf("repeat exact v18 migration: result=%q err=%v", result, err)
		}
		if result, err := ApplyLatest(ctx, db); err != nil || result != ResultAlready {
			t.Fatalf("latest verification after v18 upgrade: result=%q err=%v", result, err)
		}
		if countWhere(t, db, "app_schema_migrations", "version = ? AND checksum = ? AND state = 'applied'", SupportTicketsV18Version, SupportTicketsV18Checksum) != 1 {
			t.Fatal("v18 ledger row is missing after upgrade")
		}
		assertLatestInventory(t, db)
	})

	t.Run("v17 schema drift is refused without adding v18 table", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		createExactV17SupportTicketsSchema(t, ctx, db)
		if _, err := db.Exec(`ALTER TABLE platform_expenses ADD COLUMN unexpected_v17_marker INT NULL`); err != nil {
			t.Fatalf("create synthetic v17 drift: %v", err)
		}
		before := snapshotDatabaseTables(t, db)
		if _, err := ApplySupportTicketsV18(ctx, db); err == nil {
			t.Fatal("v18 unexpectedly accepted a drifted v17 schema")
		}
		if after := snapshotDatabaseTables(t, db); after != before {
			t.Fatalf("v17 drift refusal mutated the database:\nBEFORE:\n%s\nAFTER:\n%s", before, after)
		}
		if countWhere(t, db, "app_schema_migrations", "version = ?", SupportTicketsV18Version) != 0 {
			t.Fatal("v18 ledger row was written after refusing v17 drift")
		}
	})

	t.Run("non-exact v17 database is refused", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		createExactV17SupportTicketsSchema(t, ctx, db)
		// A v17 ledger row with a tampered checksum is not an exact
		// managed v17 schema: the upgrade must fail closed.
		if _, err := db.Exec(`UPDATE app_schema_migrations SET checksum=REPEAT('0',64) WHERE version=?`, MerchantOnboardingV17Version); err != nil {
			t.Fatal(err)
		}
		before := snapshotDatabaseTables(t, db)
		if _, err := ApplySupportTicketsV18(ctx, db); err == nil {
			t.Fatal("v18 unexpectedly accepted a non-exact v17 database")
		}
		if after := snapshotDatabaseTables(t, db); after != before {
			t.Fatalf("non-exact v17 refusal mutated the database:\nBEFORE:\n%s\nAFTER:\n%s", before, after)
		}
		if countWhere(t, db, "app_schema_migrations", "version = ?", SupportTicketsV18Version) != 0 {
			t.Fatal("v18 ledger row was written after refusing a non-exact v17 database")
		}
	})
}

func createExactV17SupportTicketsSchema(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	createExactV16MerchantOnboardingSchema(t, ctx, db)
	if result, err := ApplyMerchantOnboardingV17(ctx, db); err != nil || result != ResultApplied {
		t.Fatalf("create exact v17 schema: result=%q err=%v", result, err)
	}
}
