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
		assertLatestInventory(t, db)
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
		assertLatestInventory(t, db)

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

	t.Run("exact managed v2 upgrades to admin auth version v3", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		if result, err := ApplyCandidate(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("create exact managed v1: result=%q err=%v", result, err)
		}
		if result, err := ApplyManualReviewV2(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("create exact managed v2: result=%q err=%v", result, err)
		}
		result, err := ApplyLatest(ctx, db)
		if err != nil {
			t.Fatalf("upgrade exact managed v2 to v3: %v", err)
		}
		if result != ResultApplied {
			t.Fatalf("upgrade result=%q, want %q", result, ResultApplied)
		}
		assertLatestInventory(t, db)
	})

	t.Run("exact managed v3 upgrades SMS and email OTP storage and invalidates legacy codes", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		if result, err := ApplyCandidate(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("create exact managed v1: result=%q err=%v", result, err)
		}
		if result, err := ApplyManualReviewV2(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("create exact managed v2: result=%q err=%v", result, err)
		}
		if result, err := ApplyAdminAuthVersionV3(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("create exact managed v3: result=%q err=%v", result, err)
		}
		if _, err := db.Exec(`INSERT INTO verification_codes(phone, scene, code, expires_at, created_at, updated_at) VALUES('13800000001','login','123456',DATE_ADD(NOW(3), INTERVAL 10 MINUTE),NOW(3),NOW(3))`); err != nil {
			t.Fatalf("insert synthetic legacy plaintext OTP: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO email_verification_codes(email, scene, code, expires_at, created_at, updated_at) VALUES('otp-safety@example.test','bind_email','654321',DATE_ADD(NOW(3), INTERVAL 10 MINUTE),NOW(3),NOW(3))`); err != nil {
			t.Fatalf("insert synthetic legacy plaintext email OTP: %v", err)
		}

		result, err := ApplyLatest(ctx, db)
		if err != nil {
			t.Fatalf("upgrade exact managed v3 through v5: %v", err)
		}
		if result != ResultApplied {
			t.Fatalf("upgrade result=%q, want %q", result, ResultApplied)
		}
		var storedCode, storedHash string
		var failedAttempts uint
		var consumedAt sql.NullTime
		if err := db.QueryRow(`SELECT code, code_hash, failed_attempts, consumed_at FROM verification_codes WHERE phone='13800000001' AND scene='login'`).Scan(&storedCode, &storedHash, &failedAttempts, &consumedAt); err != nil {
			t.Fatalf("read scrubbed synthetic OTP: %v", err)
		}
		if storedCode != "REDACTED" || storedHash != strings.Repeat("0", 64) || failedAttempts != 0 || !consumedAt.Valid {
			t.Fatalf("legacy OTP was not safely scrubbed and invalidated: code=%q hash=%q attempts=%d consumed=%v", storedCode, storedHash, failedAttempts, consumedAt)
		}
		if err := db.QueryRow(`SELECT code, code_hash, failed_attempts, consumed_at FROM email_verification_codes WHERE email='otp-safety@example.test' AND scene='bind_email'`).Scan(&storedCode, &storedHash, &failedAttempts, &consumedAt); err != nil {
			t.Fatalf("read scrubbed synthetic email OTP: %v", err)
		}
		if storedCode != "REDACTED" || storedHash != strings.Repeat("0", 64) || failedAttempts != 0 || !consumedAt.Valid {
			t.Fatalf("legacy email OTP was not safely scrubbed and invalidated: code=%q hash=%q attempts=%d consumed=%v", storedCode, storedHash, failedAttempts, consumedAt)
		}
		assertLatestInventory(t, db)
		if result, err := ApplyLatest(ctx, db); err != nil || result != ResultAlready {
			t.Fatalf("repeat latest migration: result=%q err=%v", result, err)
		}
	})

	t.Run("exact managed v4 upgrades email OTP storage to v5", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		for _, apply := range []struct {
			name string
			fn   func(context.Context, *sql.DB) (Result, error)
		}{
			{"v1", ApplyCandidate},
			{"v2", ApplyManualReviewV2},
			{"v3", ApplyAdminAuthVersionV3},
			{"v4", ApplyOTPSafetyV4},
		} {
			if result, err := apply.fn(ctx, db); err != nil || result != ResultApplied {
				t.Fatalf("create exact managed %s: result=%q err=%v", apply.name, result, err)
			}
		}
		if _, err := db.Exec(`INSERT INTO email_verification_codes(email, scene, code, expires_at, created_at, updated_at) VALUES('otp-v5@example.test','bind_email','123456',DATE_ADD(NOW(3), INTERVAL 10 MINUTE),NOW(3),NOW(3))`); err != nil {
			t.Fatalf("insert synthetic v4 plaintext email OTP: %v", err)
		}

		result, err := ApplyEmailOTPSafetyV5(ctx, db)
		if err != nil || result != ResultApplied {
			t.Fatalf("upgrade exact managed v4 to v5: result=%q err=%v", result, err)
		}
		var code, digest string
		var attempts uint
		var consumedAt sql.NullTime
		if err := db.QueryRow(`SELECT code, code_hash, failed_attempts, consumed_at FROM email_verification_codes WHERE email='otp-v5@example.test' AND scene='bind_email'`).Scan(&code, &digest, &attempts, &consumedAt); err != nil {
			t.Fatalf("read scrubbed synthetic email OTP: %v", err)
		}
		if code != "REDACTED" || digest != strings.Repeat("0", 64) || attempts != 0 || !consumedAt.Valid {
			t.Fatalf("v5 must scrub and invalidate prior email OTPs: code=%q hash=%q attempts=%d consumed=%v", code, digest, attempts, consumedAt)
		}
		if result, err := ApplyEmailOTPSafetyV5(ctx, db); err != nil || result != ResultAlready {
			t.Fatalf("repeat v5 migration: result=%q err=%v", result, err)
		}
		if result, err := ApplyWholesaleRetailV6(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("upgrade exact managed v5 to v6: result=%q err=%v", result, err)
		}
		if result, err := ApplyPlatformExpensesV7(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("upgrade exact managed v6 to v7: result=%q err=%v", result, err)
		}
		if result, err := ApplyPlatformExpenseReversalsV8(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("upgrade exact managed v7 to v8: result=%q err=%v", result, err)
		}
		if result, err := ApplyTableOrderingV9(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("upgrade exact managed v8 to v9: result=%q err=%v", result, err)
		}
		if result, err := ApplyTableLabelsV10(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("upgrade exact managed v9 to v10: result=%q err=%v", result, err)
		}
		if result, err := ApplyOrderItemsV11(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("upgrade exact managed v10 to v11: result=%q err=%v", result, err)
		}
		if result, err := ApplyAICostControlV12(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("upgrade exact managed v11 to v12: result=%q err=%v", result, err)
		}
		if result, err := ApplyAnnouncementsMaintenanceV13(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("upgrade exact managed v12 to v13: result=%q err=%v", result, err)
		}
		if result, err := ApplyPlatformFinanceV14(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("upgrade exact managed v13 to v14: result=%q err=%v", result, err)
		}
		if result, err := ApplyRefundReviewV15(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("upgrade exact managed v14 to v15: result=%q err=%v", result, err)
		}
		if result, err := ApplyCustomerProfilesV16(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("upgrade exact managed v15 to v16: result=%q err=%v", result, err)
		}
		assertLatestInventory(t, db)
	})

	t.Run("exact managed v5 upgrades wholesale and retail product pricing to v6", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		for _, apply := range []struct {
			name string
			fn   func(context.Context, *sql.DB) (Result, error)
		}{
			{"v1", ApplyCandidate},
			{"v2", ApplyManualReviewV2},
			{"v3", ApplyAdminAuthVersionV3},
			{"v4", ApplyOTPSafetyV4},
			{"v5", ApplyEmailOTPSafetyV5},
		} {
			if result, err := apply.fn(ctx, db); err != nil || result != ResultApplied {
				t.Fatalf("create exact managed %s: result=%q err=%v", apply.name, result, err)
			}
		}
		result, err := ApplyWholesaleRetailV6(ctx, db)
		if err != nil || result != ResultApplied {
			t.Fatalf("upgrade exact managed v5 to v6: result=%q err=%v", result, err)
		}
		if result, err := ApplyPlatformExpensesV7(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("upgrade exact managed v6 to v7: result=%q err=%v", result, err)
		}
		if result, err := ApplyPlatformExpenseReversalsV8(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("upgrade exact managed v7 to v8: result=%q err=%v", result, err)
		}
		if result, err := ApplyTableOrderingV9(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("upgrade exact managed v8 to v9: result=%q err=%v", result, err)
		}
		if result, err := ApplyTableLabelsV10(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("upgrade exact managed v9 to v10: result=%q err=%v", result, err)
		}
		if result, err := ApplyOrderItemsV11(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("upgrade exact managed v10 to v11: result=%q err=%v", result, err)
		}
		if result, err := ApplyAICostControlV12(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("upgrade exact managed v11 to v12: result=%q err=%v", result, err)
		}
		if result, err := ApplyAnnouncementsMaintenanceV13(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("upgrade exact managed v12 to v13: result=%q err=%v", result, err)
		}
		if result, err := ApplyPlatformFinanceV14(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("upgrade exact managed v13 to v14: result=%q err=%v", result, err)
		}
		if result, err := ApplyRefundReviewV15(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("upgrade exact managed v14 to v15: result=%q err=%v", result, err)
		}
		if result, err := ApplyCustomerProfilesV16(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("upgrade exact managed v15 to v16: result=%q err=%v", result, err)
		}
		assertLatestInventory(t, db)
		if result, err := ApplyLatest(ctx, db); err != nil || result != ResultAlready {
			t.Fatalf("repeat latest v16 migration: result=%q err=%v", result, err)
		}
	})

	t.Run("v3 schema drift is refused without applying v4", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		if _, err := ApplyCandidate(ctx, db); err != nil {
			t.Fatal(err)
		}
		if _, err := ApplyManualReviewV2(ctx, db); err != nil {
			t.Fatal(err)
		}
		if _, err := ApplyAdminAuthVersionV3(ctx, db); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`ALTER TABLE verification_codes ADD COLUMN unexpected_v3_marker INT NULL`); err != nil {
			t.Fatal(err)
		}
		before := snapshotDatabaseTables(t, db)
		if _, err := ApplyOTPSafetyV4(ctx, db); err == nil {
			t.Fatal("v4 migration unexpectedly accepted a drifted v3 schema")
		}
		if after := snapshotDatabaseTables(t, db); after != before {
			t.Fatalf("v3 drift refusal mutated database:\nBEFORE:\n%s\nAFTER:\n%s", before, after)
		}
		if countWhere(t, db, "app_schema_migrations", "version = ?", OTPSafetyV4Version) != 0 {
			t.Fatal("v4 ledger row was written after refusing v3 drift")
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
		if _, err := ApplyLatest(ctx, db); err == nil || !strings.Contains(err.Error(), "ledger entry") {
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

func assertLatestInventory(t *testing.T, db *sql.DB) {
	t.Helper()
	var tables, triggers, v1Rows, v2Rows, v3Rows, v4Rows, v5Rows, v6Rows, v7Rows, v8Rows, v9Rows, v10Rows, v11Rows, v12Rows, v13Rows, v14Rows, v15Rows, v16Rows, authVersionColumns, otpColumns, emailOTPColumns, productPricingColumns, platformExpenseColumns, platformExpenseReversalColumns, tableOrderingColumns, tableLabelColumns, orderItemsColumns, orderChannelColumns, aiModelPriceColumns, aiVendorBillColumns, aiBudgetColumns, aiBudgetAlertColumns, aiEmergencySwitchColumns, aiUsageLogNewColumns, announcementColumns, maintenancePlanColumns, campaignTargetColumns, campaignPostmortemColumns, suggestionExecutionColumns, manualRedemptionColumns, timeLogColumns, incomeRecordColumns, paymentRecordColumns, refundRecordColumns, customerProfileColumns int
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
	if err := db.QueryRow(`SELECT COUNT(*) FROM app_schema_migrations WHERE version=? AND checksum=? AND state='applied' AND schema_fingerprint IS NOT NULL`, AdminAuthV3Version, AdminAuthV3Checksum).Scan(&v3Rows); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM app_schema_migrations WHERE version=? AND checksum=? AND state='applied' AND schema_fingerprint IS NOT NULL`, OTPSafetyV4Version, OTPSafetyV4Checksum).Scan(&v4Rows); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM app_schema_migrations WHERE version=? AND checksum=? AND state='applied' AND schema_fingerprint IS NOT NULL`, EmailOTPSafetyV5Version, EmailOTPSafetyV5Checksum).Scan(&v5Rows); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM app_schema_migrations WHERE version=? AND checksum=? AND state='applied' AND schema_fingerprint IS NOT NULL`, WholesaleRetailV6Version, WholesaleRetailV6Checksum).Scan(&v6Rows); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM app_schema_migrations WHERE version=? AND checksum=? AND state='applied' AND schema_fingerprint IS NOT NULL`, PlatformExpensesV7Version, PlatformExpensesV7Checksum).Scan(&v7Rows); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM app_schema_migrations WHERE version=? AND checksum=? AND state='applied' AND schema_fingerprint IS NOT NULL`, PlatformExpenseReversalsV8Version, PlatformExpenseReversalsV8Checksum).Scan(&v8Rows); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM app_schema_migrations WHERE version=? AND checksum=? AND state='applied' AND schema_fingerprint IS NOT NULL`, TableOrderingV9Version, TableOrderingV9Checksum).Scan(&v9Rows); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM app_schema_migrations WHERE version=? AND checksum=? AND state='applied' AND schema_fingerprint IS NOT NULL`, TableLabelsV10Version, TableLabelsV10Checksum).Scan(&v10Rows); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM app_schema_migrations WHERE version=? AND checksum=? AND state='applied' AND schema_fingerprint IS NOT NULL`, OrderItemsV11Version, OrderItemsV11Checksum).Scan(&v11Rows); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM app_schema_migrations WHERE version=? AND checksum=? AND state='applied' AND schema_fingerprint IS NOT NULL`, AICostControlV12Version, AICostControlV12Checksum).Scan(&v12Rows); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM app_schema_migrations WHERE version=? AND checksum=? AND state='applied' AND schema_fingerprint IS NOT NULL`, AnnouncementsMaintenanceV13Version, AnnouncementsMaintenanceV13Checksum).Scan(&v13Rows); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM app_schema_migrations WHERE version=? AND checksum=? AND state='applied' AND schema_fingerprint IS NOT NULL`, PlatformFinanceV14Version, PlatformFinanceV14Checksum).Scan(&v14Rows); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM app_schema_migrations WHERE version=? AND checksum=? AND state='applied' AND schema_fingerprint IS NOT NULL`, RefundReviewV15Version, RefundReviewV15Checksum).Scan(&v15Rows); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM app_schema_migrations WHERE version=? AND checksum=? AND state='applied' AND schema_fingerprint IS NOT NULL`, CustomerProfilesV16Version, CustomerProfilesV16Checksum).Scan(&v16Rows); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='admin_users' AND COLUMN_NAME='auth_version' AND COLUMN_TYPE='bigint unsigned' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='1'`).Scan(&authVersionColumns); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='verification_codes' AND ((COLUMN_NAME='code_hash' AND COLUMN_TYPE='char(64)' AND IS_NULLABLE='YES') OR (COLUMN_NAME='failed_attempts' AND COLUMN_TYPE='int unsigned' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='0'))`).Scan(&otpColumns); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='email_verification_codes' AND ((COLUMN_NAME='code_hash' AND COLUMN_TYPE='char(64)' AND IS_NULLABLE='YES') OR (COLUMN_NAME='failed_attempts' AND COLUMN_TYPE='int unsigned' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='0'))`).Scan(&emailOTPColumns); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='store_products' AND ((COLUMN_NAME='sales_unit' AND COLUMN_TYPE='varchar(30)' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='') OR (COLUMN_NAME='specification' AND COLUMN_TYPE='varchar(80)' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='') OR (COLUMN_NAME='minimum_order_quantity' AND COLUMN_TYPE='int unsigned' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='1') OR (COLUMN_NAME='price_tiers' AND COLUMN_TYPE='json' AND IS_NULLABLE='YES' AND COLUMN_DEFAULT IS NULL))`).Scan(&productPricingColumns); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='platform_expenses'`).Scan(&platformExpenseColumns); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='platform_expense_reversals'`).Scan(&platformExpenseReversalColumns); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN (?, ?, ?, ?, ?)`, tableOrderingV9Order[0], tableOrderingV9Order[1], tableOrderingV9Order[2], tableOrderingV9Order[3], tableOrderingV9Order[4]).Scan(&tableOrderingColumns); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND ((TABLE_NAME='store_tables' AND COLUMN_NAME='table_label') OR (TABLE_NAME='table_order_details' AND COLUMN_NAME='table_label_snapshot'))`).Scan(&tableLabelColumns); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='order_items'`).Scan(&orderItemsColumns); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='orders' AND COLUMN_NAME='channel' AND COLUMN_TYPE='varchar(24)' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='store_qr'`).Scan(&orderChannelColumns); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='ai_model_prices'`).Scan(&aiModelPriceColumns); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='ai_vendor_bills'`).Scan(&aiVendorBillColumns); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='merchant_ai_budgets'`).Scan(&aiBudgetColumns); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='merchant_ai_budget_alerts'`).Scan(&aiBudgetAlertColumns); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='ai_emergency_switch'`).Scan(&aiEmergencySwitchColumns); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='merchant_ai_usage_logs' AND COLUMN_NAME IN ('store_id', 'cached_tokens')`).Scan(&aiUsageLogNewColumns); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='announcements'`).Scan(&announcementColumns); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='maintenance_plans'`).Scan(&maintenancePlanColumns); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='campaign_targets'`).Scan(&campaignTargetColumns); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='campaign_postmortems'`).Scan(&campaignPostmortemColumns); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='ai_suggestion_executions'`).Scan(&suggestionExecutionColumns); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='group_buy_manual_redemptions'`).Scan(&manualRedemptionColumns); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='platform_time_logs'`).Scan(&timeLogColumns); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='platform_income_records'`).Scan(&incomeRecordColumns); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='payment_records'`).Scan(&paymentRecordColumns); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='refund_records'`).Scan(&refundRecordColumns); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='customer_profiles'`).Scan(&customerProfileColumns); err != nil {
		t.Fatal(err)
	}
	if tables != 54 || triggers != 10 || v1Rows != 1 || v2Rows != 1 || v3Rows != 1 || v4Rows != 1 || v5Rows != 1 || v6Rows != 1 || v7Rows != 1 || v8Rows != 1 || v9Rows != 1 || v10Rows != 1 || v11Rows != 1 || v12Rows != 1 || v13Rows != 1 || v14Rows != 1 || v15Rows != 1 || v16Rows != 1 || authVersionColumns != 1 || otpColumns != 2 || emailOTPColumns != 2 || productPricingColumns != 4 || platformExpenseColumns != 16 || platformExpenseReversalColumns != 9 || tableOrderingColumns != 34 || tableLabelColumns != 2 || orderItemsColumns != 9 || orderChannelColumns != 1 || aiModelPriceColumns != 13 || aiVendorBillColumns != 11 || aiBudgetColumns != 8 || aiBudgetAlertColumns != 9 || aiEmergencySwitchColumns != 6 || aiUsageLogNewColumns != 2 || announcementColumns != 13 || maintenancePlanColumns != 11 || campaignTargetColumns != 12 || campaignPostmortemColumns != 16 || suggestionExecutionColumns != 14 || manualRedemptionColumns != 11 || timeLogColumns != 12 || incomeRecordColumns != 20 || paymentRecordColumns != 11 || refundRecordColumns != 20 || customerProfileColumns != 15 {
		t.Fatalf("latest inventory tables=%d triggers=%d v1-v16 ledger=%d/%d/%d/%d/%d/%d/%d/%d/%d/%d/%d/%d/%d/%d/%d/%d auth_version=%d sms_otp_columns=%d email_otp_columns=%d product_pricing_columns=%d platform_expense_columns=%d platform_expense_reversal_columns=%d table_ordering_columns=%d label_columns=%d order_items_columns=%d order_channel_columns=%d ai_model_price_columns=%d ai_vendor_bill_columns=%d ai_budget_columns=%d ai_budget_alert_columns=%d ai_emergency_switch_columns=%d ai_usage_log_new_columns=%d announcement_columns=%d maintenance_plan_columns=%d campaign_target_columns=%d campaign_postmortem_columns=%d suggestion_execution_columns=%d manual_redemption_columns=%d time_log_columns=%d income_record_columns=%d payment_record_columns=%d refund_record_columns=%d customer_profile_columns=%d, want 54,10,sixteen applied rows and 1/2/2/4/16/9/34/2/9/1/13/11/8/9/6/2/13/11/12/16/14/11/12/20/11/20/15 columns", tables, triggers, v1Rows, v2Rows, v3Rows, v4Rows, v5Rows, v6Rows, v7Rows, v8Rows, v9Rows, v10Rows, v11Rows, v12Rows, v13Rows, v14Rows, v15Rows, v16Rows, authVersionColumns, otpColumns, emailOTPColumns, productPricingColumns, platformExpenseColumns, platformExpenseReversalColumns, tableOrderingColumns, tableLabelColumns, orderItemsColumns, orderChannelColumns, aiModelPriceColumns, aiVendorBillColumns, aiBudgetColumns, aiBudgetAlertColumns, aiEmergencySwitchColumns, aiUsageLogNewColumns, announcementColumns, maintenancePlanColumns, campaignTargetColumns, campaignPostmortemColumns, suggestionExecutionColumns, manualRedemptionColumns, timeLogColumns, incomeRecordColumns, paymentRecordColumns, refundRecordColumns, customerProfileColumns)
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
