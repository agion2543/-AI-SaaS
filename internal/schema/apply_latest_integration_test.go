package schema

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestMySQLManualReviewV2Migration(t *testing.T) {
	control, baseConfig := openSafeMySQLTestServer(t)
	ctx := context.Background()

	t.Run("empty current install and rerun", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		first, err := ApplyLatest(ctx, db)
		if err != nil {
			t.Fatalf("first latest migration: %v", err)
		}
		if first != ResultApplied {
			t.Fatalf("first latest result=%q, want %q", first, ResultApplied)
		}
		second, err := ApplyLatest(ctx, db)
		if err != nil {
			t.Fatalf("repeat latest migration: %v", err)
		}
		if second != ResultAlready {
			t.Fatalf("repeat latest result=%q, want %q", second, ResultAlready)
		}
		assertV2Inventory(t, db)
	})

	t.Run("exact managed v1 upgrade preserves data and creates append-only records", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		if result, err := ApplyCandidate(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("create exact managed v1: result=%q err=%v", result, err)
		}
		merchantID, orderID, orderNo := insertSyntheticV1MerchantOrder(t, db)
		result, err := ApplyLatest(ctx, db)
		if err != nil {
			t.Fatalf("upgrade exact managed v1: %v", err)
		}
		if result != ResultApplied {
			t.Fatalf("upgrade result=%q, want %q", result, ResultApplied)
		}
		if got := countWhere(t, db, "merchants", "id = ? AND name = ?", merchantID, "synthetic-v2-merchant"); got != 1 {
			t.Fatalf("v1 merchant row count after upgrade=%d", got)
		}
		if got := countWhere(t, db, "orders", "id = ? AND order_no = ?", orderID, orderNo); got != 1 {
			t.Fatalf("v1 order row count after upgrade=%d", got)
		}
		assertV2Inventory(t, db)

		reviewID := insertSyntheticManualReview(t, db, merchantID, orderID, orderNo)
		auditID := insertSyntheticManualReviewAudit(t, db, reviewID, merchantID, orderID)
		for _, statement := range []struct {
			label string
			sql   string
			args  []any
		}{
			{"review update", `UPDATE merchant_subscription_manual_reviews SET reason='tampered' WHERE id=?`, []any{reviewID}},
			{"review delete", `DELETE FROM merchant_subscription_manual_reviews WHERE id=?`, []any{reviewID}},
			{"audit update", `UPDATE merchant_subscription_manual_review_audits SET reason='tampered' WHERE id=?`, []any{auditID}},
			{"audit delete", `DELETE FROM merchant_subscription_manual_review_audits WHERE id=?`, []any{auditID}},
		} {
			if _, err := db.Exec(statement.sql, statement.args...); err == nil {
				t.Fatalf("append-only trigger allowed %s", statement.label)
			}
		}
		if countWhere(t, db, "merchant_subscription_manual_reviews", "id = ?", reviewID) != 1 || countWhere(t, db, "merchant_subscription_manual_review_audits", "id = ?", auditID) != 1 {
			t.Fatal("failed update/delete changed immutable review or audit records")
		}
		if result, err := ApplyLatest(ctx, db); err != nil || result != ResultAlready {
			t.Fatalf("latest rerun after upgrade: result=%q err=%v", result, err)
		}
	})

	t.Run("v1 checksum drift refused without applying v2", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		if _, err := ApplyCandidate(ctx, db); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`UPDATE app_schema_migrations SET checksum=REPEAT('0',64) WHERE version=?`, BaselineVersion); err != nil {
			t.Fatal(err)
		}
		before := snapshotDatabaseTables(t, db)
		if _, err := ApplyLatest(ctx, db); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
			t.Fatalf("ApplyLatest error=%v, want v1 checksum refusal", err)
		}
		if after := snapshotDatabaseTables(t, db); after != before {
			t.Fatalf("checksum refusal mutated database:\nBEFORE:\n%s\nAFTER:\n%s", before, after)
		}
		if countWhere(t, db, "app_schema_migrations", "version = ?", ManualReviewV2Version) != 0 {
			t.Fatal("v2 ledger row was written after v1 checksum mismatch")
		}
	})

	t.Run("v1 schema drift refused without applying v2", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		if _, err := ApplyCandidate(ctx, db); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`ALTER TABLE users ADD COLUMN unexpected_v1_marker INT NULL`); err != nil {
			t.Fatal(err)
		}
		before := snapshotDatabaseTables(t, db)
		if _, err := ApplyLatest(ctx, db); err == nil {
			t.Fatal("ApplyLatest unexpectedly accepted drifted v1")
		}
		if after := snapshotDatabaseTables(t, db); after != before {
			t.Fatalf("v1 drift refusal mutated database:\nBEFORE:\n%s\nAFTER:\n%s", before, after)
		}
		if countWhere(t, db, "app_schema_migrations", "version = ?", ManualReviewV2Version) != 0 {
			t.Fatal("v2 ledger row was written after v1 schema drift")
		}
	})

	t.Run("v2 checksum drift refused without mutation", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		if _, err := ApplyLatest(ctx, db); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`UPDATE app_schema_migrations SET checksum=REPEAT('0',64) WHERE version=?`, ManualReviewV2Version); err != nil {
			t.Fatal(err)
		}
		before := snapshotDatabaseTables(t, db)
		if _, err := ApplyLatest(ctx, db); err == nil || !strings.Contains(err.Error(), "v2 ledger entry") {
			t.Fatalf("ApplyLatest error=%v, want v2 checksum refusal", err)
		}
		if after := snapshotDatabaseTables(t, db); after != before {
			t.Fatalf("v2 checksum refusal mutated database:\nBEFORE:\n%s\nAFTER:\n%s", before, after)
		}
	})

	t.Run("v2 table drift refused without mutation", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		if _, err := ApplyLatest(ctx, db); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`ALTER TABLE merchant_subscription_manual_review_audits ADD COLUMN unexpected_v2_marker INT NULL`); err != nil {
			t.Fatal(err)
		}
		before := snapshotDatabaseTables(t, db)
		if _, err := ApplyLatest(ctx, db); err == nil || !strings.Contains(err.Error(), "schema drift") {
			t.Fatalf("ApplyLatest error=%v, want v2 schema drift refusal", err)
		}
		if after := snapshotDatabaseTables(t, db); after != before {
			t.Fatalf("v2 schema drift refusal mutated database:\nBEFORE:\n%s\nAFTER:\n%s", before, after)
		}
	})

	t.Run("unknown unmanaged database remains untouched", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		if _, err := db.Exec(`CREATE TABLE keep_me (id INT PRIMARY KEY, marker VARCHAR(20) NOT NULL)`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO keep_me(id, marker) VALUES(7, 'untouched')`); err != nil {
			t.Fatal(err)
		}
		before := snapshotDatabaseTables(t, db)
		if _, err := ApplyLatest(ctx, db); err == nil {
			t.Fatal("ApplyLatest unexpectedly accepted unknown unmanaged database")
		}
		if after := snapshotDatabaseTables(t, db); after != before {
			t.Fatalf("unmanaged refusal mutated database:\nBEFORE:\n%s\nAFTER:\n%s", before, after)
		}
	})
}

func assertV2Inventory(t *testing.T, db *sql.DB) {
	t.Helper()
	var tables, triggers, v1Rows, v2Rows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_TYPE='BASE TABLE'`).Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE()`).Scan(&triggers); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM app_schema_migrations WHERE version=? AND checksum=? AND state='applied' AND schema_fingerprint IS NOT NULL`, BaselineVersion, BaselineChecksum).Scan(&v1Rows); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM app_schema_migrations WHERE version=? AND checksum=? AND state='applied' AND schema_fingerprint IS NOT NULL`, ManualReviewV2Version, ManualReviewV2Checksum).Scan(&v2Rows); err != nil {
		t.Fatal(err)
	}
	if tables != 32 || triggers != 6 || v1Rows != 1 || v2Rows != 1 {
		t.Fatalf("v2 inventory tables=%d triggers=%d v1/v2 ledger=%d/%d, want 32,6,1,1", tables, triggers, v1Rows, v2Rows)
	}
}

func insertSyntheticV1MerchantOrder(t *testing.T, db *sql.DB) (uint64, uint64, string) {
	t.Helper()
	merchantResult, err := db.Exec(`INSERT INTO merchants(name, status, subscription_status) VALUES('synthetic-v2-merchant','active','inactive')`)
	if err != nil {
		t.Fatalf("insert synthetic v1 merchant: %v", err)
	}
	merchantID, err := merchantResult.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	orderNo := fmt.Sprintf("SYNTHETIC-V2-ORDER-%d", time.Now().UnixNano())
	orderResult, err := db.Exec(`INSERT INTO orders(order_no, order_type, merchant_id, amount, total_amount, status) VALUES(?, 'merchant_subscription', ?, 19900, 19900, 'pending')`, orderNo, merchantID)
	if err != nil {
		t.Fatalf("insert synthetic v1 subscription order: %v", err)
	}
	orderID, err := orderResult.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return uint64(merchantID), uint64(orderID), orderNo
}

func insertSyntheticManualReview(t *testing.T, db *sql.DB, merchantID, orderID uint64, orderNo string) uint64 {
	t.Helper()
	result, err := db.Exec(`INSERT INTO merchant_subscription_manual_reviews(
		review_key, order_id, order_no, merchant_id, actor_id, actor_role, plan_name,
		amount_cents, duration_days, payment_occurred_attested, evidence_reference, reason,
		request_fingerprint, review_status, created_at, updated_at
	) VALUES(?, ?, ?, ?, 17, 'admin', 'synthetic-original-plan', 19900, 30, 1,
		'synthetic-evidence-001', 'synthetic immutable review', REPEAT('a',64), 'manually_verified', NOW(3), NOW(3))`,
		fmt.Sprintf("MSUB-MANUAL-REVIEW-%d", orderID), orderID, orderNo, merchantID)
	if err != nil {
		t.Fatalf("insert synthetic manual review: %v", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return uint64(id)
}

func insertSyntheticManualReviewAudit(t *testing.T, db *sql.DB, reviewID, merchantID, orderID uint64) uint64 {
	t.Helper()
	result, err := db.Exec(`INSERT INTO merchant_subscription_manual_review_audits(
		review_id, order_id, merchant_id, actor_id, actor_role, evidence_reference, reason,
		plan_name, amount_cents, duration_days, payment_occurred_attested,
		order_status_before, order_status_after, order_paid_at_before, order_paid_at_after,
		order_updated_at_before, order_updated_at_after, subscription_status_before,
		subscription_status_after, subscription_plan_before, subscription_plan_after,
		subscription_expire_at_before, subscription_expire_at_after, reviewed_at, created_at
	) VALUES(?, ?, ?, 17, 'admin', 'synthetic-evidence-001', 'synthetic audit',
		'synthetic-original-plan', 19900, 30, 1, 'pending', 'manually_verified', NULL, NULL,
		NOW(3), NOW(3), 'inactive', 'active', 'none', '历史复核', NULL,
		DATE_ADD(NOW(3), INTERVAL 30 DAY), NOW(3), NOW(3))`, reviewID, orderID, merchantID)
	if err != nil {
		t.Fatalf("insert synthetic manual review audit: %v", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return uint64(id)
}

func countWhere(t *testing.T, db *sql.DB, table, where string, args ...any) int {
	t.Helper()
	query := "SELECT COUNT(*) FROM `" + strings.ReplaceAll(table, "`", "``") + "` WHERE " + where
	var count int
	if err := db.QueryRow(query, args...).Scan(&count); err != nil {
		t.Fatalf("count from %s: %v", table, err)
	}
	return count
}
