package schema

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func TestMerchantOnboardingV17DefinitionIsChecksumPinned(t *testing.T) {
	if err := validateMerchantOnboardingV17Definition(); err != nil {
		t.Fatal(err)
	}
	if MerchantOnboardingV17Version != "schema-v17-merchant-onboarding-20261128" {
		t.Fatalf("v17 version=%q", MerchantOnboardingV17Version)
	}
	if len(merchantOnboardingV17Statements) != 3 ||
		!strings.Contains(merchantOnboardingV17Statements[0], "ADD COLUMN `review_reason`") ||
		!strings.Contains(merchantOnboardingV17Statements[1], "ADD COLUMN `reviewed_by`") ||
		!strings.Contains(merchantOnboardingV17Statements[2], "ADD COLUMN `reviewed_at`") {
		t.Fatalf("unexpected v17 DDL inventory: %v", merchantOnboardingV17Statements)
	}
	if len(merchantOnboardingV17NewColumns) != 3 || merchantOnboardingV17NewColumns[0].table != "merchants" {
		t.Fatalf("unexpected v17 new columns: %v", merchantOnboardingV17NewColumns)
	}
}

func TestMySQLMerchantOnboardingV17Migration(t *testing.T) {
	control, baseConfig := openSafeMySQLTestServer(t)
	ctx := context.Background()

	t.Run("exact v16 upgrade adds onboarding review columns without rewriting history", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		createExactV16MerchantOnboardingSchema(t, ctx, db)

		// Pre-v17 merchants keep their legacy review defaults after the
		// upgrade: no reason, reviewer 0, NULL review time.
		if _, err := db.Exec(`INSERT INTO merchants (id, name, contact_phone, status, created_at, updated_at) VALUES (1, 'V17 老商家', '13800000001', 'active', NOW(3), NOW(3)), (2, 'V17 待审核商家', '13800000002', 'pending', NOW(3), NOW(3))`); err != nil {
			t.Fatalf("insert legacy v16 merchants: %v", err)
		}

		if result, err := ApplyMerchantOnboardingV17(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("upgrade exact v16 to v17: result=%q err=%v", result, err)
		}
		var columns int
		if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='merchants'`).Scan(&columns); err != nil {
			t.Fatal(err)
		}
		if columns != 18 {
			t.Fatalf("merchants columns=%d, want 18", columns)
		}
		rows, err := db.Query(`SELECT id, status, review_reason, reviewed_by, reviewed_at FROM merchants ORDER BY id`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		type legacyRow struct {
			id, status, reason string
			reviewedBy         int64
			reviewedAt         sql.NullString
		}
		got := make([]legacyRow, 0, 2)
		for rows.Next() {
			var row legacyRow
			if err := rows.Scan(&row.id, &row.status, &row.reason, &row.reviewedBy, &row.reviewedAt); err != nil {
				t.Fatal(err)
			}
			got = append(got, row)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 {
			t.Fatalf("legacy merchants=%d, want 2", len(got))
		}
		for _, row := range got {
			if row.reason != "" || row.reviewedBy != 0 || row.reviewedAt.Valid {
				t.Fatalf("legacy merchant %s kept review state %+v, want empty reason, reviewer 0, NULL reviewed_at", row.id, row)
			}
		}
		if got[0].status != "active" || got[1].status != "pending" {
			t.Fatalf("legacy merchant statuses=%q/%q, want active/pending", got[0].status, got[1].status)
		}
		// A rejected pending merchant can be recorded with the new columns.
		if _, err := db.Exec(`UPDATE merchants SET review_reason='资料不完整', reviewed_by=7, reviewed_at=NOW(3) WHERE id=2`); err != nil {
			t.Fatalf("record synthetic v17 rejection: %v", err)
		}
		var reason string
		var reviewedBy int64
		var reviewedNotNull int
		if err := db.QueryRow(`SELECT review_reason, reviewed_by, reviewed_at IS NOT NULL FROM merchants WHERE id=2`).Scan(&reason, &reviewedBy, &reviewedNotNull); err != nil {
			t.Fatal(err)
		}
		if reason != "资料不完整" || reviewedBy != 7 || reviewedNotNull != 1 {
			t.Fatalf("rejected merchant review columns=%q/%d/%d, want 资料不完整/7/1", reason, reviewedBy, reviewedNotNull)
		}
		if result, err := ApplyMerchantOnboardingV17(ctx, db); err != nil || result != ResultAlready {
			t.Fatalf("repeat exact v17 migration: result=%q err=%v", result, err)
		}
		if result, err := ApplyLatest(ctx, db); err != nil || result != ResultAlready {
			t.Fatalf("latest verification after v17 upgrade: result=%q err=%v", result, err)
		}
		if countWhere(t, db, "app_schema_migrations", "version = ? AND checksum = ? AND state = 'applied'", MerchantOnboardingV17Version, MerchantOnboardingV17Checksum) != 1 {
			t.Fatal("v17 ledger row is missing after upgrade")
		}
		assertLatestInventory(t, db)
	})

	t.Run("v16 schema drift is refused without adding v17 columns", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		createExactV16MerchantOnboardingSchema(t, ctx, db)
		if _, err := db.Exec(`ALTER TABLE platform_expenses ADD COLUMN unexpected_v16_marker INT NULL`); err != nil {
			t.Fatalf("create synthetic v16 drift: %v", err)
		}
		before := snapshotDatabaseTables(t, db)
		if _, err := ApplyMerchantOnboardingV17(ctx, db); err == nil {
			t.Fatal("v17 unexpectedly accepted a drifted v16 schema")
		}
		if after := snapshotDatabaseTables(t, db); after != before {
			t.Fatalf("v16 drift refusal mutated the database:\nBEFORE:\n%s\nAFTER:\n%s", before, after)
		}
		if countWhere(t, db, "app_schema_migrations", "version = ?", MerchantOnboardingV17Version) != 0 {
			t.Fatal("v17 ledger row was written after refusing v16 drift")
		}
	})

	t.Run("non-exact v16 database is refused", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		createExactV16MerchantOnboardingSchema(t, ctx, db)
		// A v16 ledger row with a tampered checksum is not an exact
		// managed v16 schema: the upgrade must fail closed.
		if _, err := db.Exec(`UPDATE app_schema_migrations SET checksum=REPEAT('0',64) WHERE version=?`, CustomerProfilesV16Version); err != nil {
			t.Fatal(err)
		}
		before := snapshotDatabaseTables(t, db)
		if _, err := ApplyMerchantOnboardingV17(ctx, db); err == nil {
			t.Fatal("v17 unexpectedly accepted a non-exact v16 database")
		}
		if after := snapshotDatabaseTables(t, db); after != before {
			t.Fatalf("non-exact v16 refusal mutated the database:\nBEFORE:\n%s\nAFTER:\n%s", before, after)
		}
		if countWhere(t, db, "app_schema_migrations", "version = ?", MerchantOnboardingV17Version) != 0 {
			t.Fatal("v17 ledger row was written after refusing a non-exact v16 database")
		}
	})
}

func createExactV16MerchantOnboardingSchema(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	createExactV15CustomerProfilesSchema(t, ctx, db)
	if result, err := ApplyCustomerProfilesV16(ctx, db); err != nil || result != ResultApplied {
		t.Fatalf("create exact v16 schema: result=%q err=%v", result, err)
	}
}
