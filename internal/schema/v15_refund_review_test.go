package schema

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func TestRefundReviewV15DefinitionIsChecksumPinned(t *testing.T) {
	if err := validateRefundReviewV15Definition(); err != nil {
		t.Fatal(err)
	}
	if RefundReviewV15Version != "schema-v15-refund-review-20261128" {
		t.Fatalf("v15 version=%q", RefundReviewV15Version)
	}
	if len(refundReviewV15Statements) != 6 ||
		!strings.Contains(refundReviewV15Statements[0], "ADD COLUMN `operator_id`") ||
		!strings.Contains(refundReviewV15Statements[1], "ADD COLUMN `operator_role`") ||
		!strings.Contains(refundReviewV15Statements[2], "ADD COLUMN `reviewer_id`") ||
		!strings.Contains(refundReviewV15Statements[5], "ADD COLUMN `reviewed_at`") {
		t.Fatalf("unexpected v15 DDL inventory: %v", refundReviewV15Statements)
	}
	if len(refundReviewV15NewColumns) != 6 {
		t.Fatalf("unexpected v15 new columns: %v", refundReviewV15NewColumns)
	}
}

func TestMySQLRefundReviewV15Migration(t *testing.T) {
	control, baseConfig := openSafeMySQLTestServer(t)
	ctx := context.Background()

	t.Run("exact v14 upgrade adds operator and review columns", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		createExactV14RefundReviewSchema(t, ctx, db)

		// Pre-v15 rows keep their legacy defaults after the upgrade: no
		// operator on the payment record, no reviewer on the refund.
		if _, err := db.Exec(`INSERT INTO orders (order_no, order_type, merchant_id, amount, total_amount, status, created_at, updated_at) VALUES ('V15-ORDER-1', 'store_order', 1, 50000, 50000, 'received', NOW(3), NOW(3))`); err != nil {
			t.Fatalf("insert legacy v14 order: %v", err)
		}
		var orderID int64
		if err := db.QueryRow(`SELECT id FROM orders WHERE order_no='V15-ORDER-1'`).Scan(&orderID); err != nil {
			t.Fatalf("read legacy v14 order id: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO payment_records (created_at, updated_at, order_id, payment_channel, transaction_no, amount, status, raw_payload) VALUES (NOW(3), NOW(3), ?, 'merchant_qr', 'manual-V15-ORDER-1', 50000, 'success', '{}')`, orderID); err != nil {
			t.Fatalf("insert legacy v14 payment record: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO refund_records (created_at, updated_at, order_id, merchant_id, refund_no, transaction_no, amount, reason, operator_role, operator_id, status, request_fingerprint, raw_payload) VALUES (NOW(3), NOW(3), ?, 1, 'RFV15LEGACY', 'manual-V15-ORDER-1', 3000, 'legacy refund', 'merchant', 42, 'success', REPEAT('a',64), '{}')`, orderID); err != nil {
			t.Fatalf("insert legacy v14 refund record: %v", err)
		}

		if result, err := ApplyLatest(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("upgrade exact v14 to v15: result=%q err=%v", result, err)
		}
		var paymentColumns, refundColumns int
		for _, check := range []struct {
			table string
			want  int
			into  *int
		}{
			{"payment_records", 11, &paymentColumns},
			{"refund_records", 20, &refundColumns},
		} {
			if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=?`, check.table).Scan(check.into); err != nil {
				t.Fatal(err)
			}
			if *check.into != check.want {
				t.Fatalf("%s columns=%d, want %d", check.table, *check.into, check.want)
			}
		}
		// Legacy rows: operator 0/"", reviewer 0/""/"" with NULL time.
		var legacyOperatorID int64
		var legacyOperatorRole string
		if err := db.QueryRow(`SELECT operator_id, operator_role FROM payment_records WHERE transaction_no='manual-V15-ORDER-1'`).Scan(&legacyOperatorID, &legacyOperatorRole); err != nil {
			t.Fatalf("read legacy payment record after v15: %v", err)
		}
		if legacyOperatorID != 0 || legacyOperatorRole != "" {
			t.Fatalf("legacy payment record operator changed: id=%d role=%q", legacyOperatorID, legacyOperatorRole)
		}
		var legacyReviewerID int64
		var legacyReviewerRole, legacyReviewReason string
		var legacyReviewedAt sql.NullTime
		if err := db.QueryRow(`SELECT reviewer_id, reviewer_role, review_reason, reviewed_at FROM refund_records WHERE refund_no='RFV15LEGACY'`).Scan(&legacyReviewerID, &legacyReviewerRole, &legacyReviewReason, &legacyReviewedAt); err != nil {
			t.Fatalf("read legacy refund record after v15: %v", err)
		}
		if legacyReviewerID != 0 || legacyReviewerRole != "" || legacyReviewReason != "" || legacyReviewedAt.Valid {
			t.Fatalf("legacy refund record review fields changed: id=%d role=%q reason=%q reviewed_at=%v", legacyReviewerID, legacyReviewerRole, legacyReviewReason, legacyReviewedAt)
		}
		// The new columns accept values: a reviewed refund and an
		// operator-stamped payment record insert cleanly.
		if _, err := db.Exec(`UPDATE payment_records SET operator_id=7, operator_role='merchant' WHERE transaction_no='manual-V15-ORDER-1'`); err != nil {
			t.Fatalf("stamp legacy payment record operator: %v", err)
		}
		if _, err := db.Exec(`UPDATE refund_records SET reviewer_id=3, reviewer_role='super_admin', review_reason='approved', reviewed_at=NOW(3) WHERE refund_no='RFV15LEGACY'`); err != nil {
			t.Fatalf("stamp legacy refund record review: %v", err)
		}
		if result, err := ApplyRefundReviewV15(ctx, db); err != nil || result != ResultAlready {
			t.Fatalf("repeat exact v15 migration: result=%q err=%v", result, err)
		}
		if result, err := ApplyLatest(ctx, db); err != nil || result != ResultAlready {
			t.Fatalf("latest verification after v15 upgrade: result=%q err=%v", result, err)
		}
		assertLatestInventory(t, db)
	})

	t.Run("v14 schema drift is refused without adding v15 columns", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		createExactV14RefundReviewSchema(t, ctx, db)
		if _, err := db.Exec(`ALTER TABLE platform_expenses ADD COLUMN unexpected_v14_marker INT NULL`); err != nil {
			t.Fatalf("create synthetic v14 drift: %v", err)
		}
		before := snapshotDatabaseTables(t, db)
		if _, err := ApplyRefundReviewV15(ctx, db); err == nil {
			t.Fatal("v15 unexpectedly accepted a drifted v14 schema")
		}
		if after := snapshotDatabaseTables(t, db); after != before {
			t.Fatalf("v15 drift refusal mutated the database:\nBEFORE:\n%s\nAFTER:\n%s", before, after)
		}
		if countWhere(t, db, "app_schema_migrations", "version = ?", RefundReviewV15Version) != 0 {
			t.Fatal("v15 ledger row was written after refusing v14 drift")
		}
	})
}

func createExactV14RefundReviewSchema(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	createExactV13PlatformFinanceSchema(t, ctx, db)
	if result, err := ApplyPlatformFinanceV14(ctx, db); err != nil || result != ResultApplied {
		t.Fatalf("create exact v14 schema: result=%q err=%v", result, err)
	}
}
