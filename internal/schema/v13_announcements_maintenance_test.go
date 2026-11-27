package schema

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func TestAnnouncementsMaintenanceV13DefinitionIsChecksumPinned(t *testing.T) {
	if err := validateAnnouncementsMaintenanceV13Definition(); err != nil {
		t.Fatal(err)
	}
	if AnnouncementsMaintenanceV13Version != "schema-v13-announcements-maintenance-20261128" {
		t.Fatalf("v13 version=%q", AnnouncementsMaintenanceV13Version)
	}
	if len(announcementsMaintenanceV13Statements) != 6 ||
		!strings.Contains(announcementsMaintenanceV13Statements[0], "CREATE TABLE `announcements`") ||
		!strings.Contains(announcementsMaintenanceV13Statements[1], "CREATE TABLE `maintenance_plans`") ||
		!strings.Contains(announcementsMaintenanceV13Statements[2], "CREATE TABLE `campaign_targets`") ||
		!strings.Contains(announcementsMaintenanceV13Statements[3], "CREATE TABLE `campaign_postmortems`") ||
		!strings.Contains(announcementsMaintenanceV13Statements[4], "CREATE TABLE `ai_suggestion_executions`") ||
		!strings.Contains(announcementsMaintenanceV13Statements[5], "CREATE TABLE `group_buy_manual_redemptions`") {
		t.Fatalf("unexpected v13 DDL inventory: %v", announcementsMaintenanceV13Statements)
	}
	if len(announcementsMaintenanceV13NewTables) != 6 || len(announcementsMaintenanceV13NewColumns) != 0 {
		t.Fatalf("unexpected v13 new tables/columns: %v / %v", announcementsMaintenanceV13NewTables, announcementsMaintenanceV13NewColumns)
	}
}

func TestMySQLAnnouncementsMaintenanceV13Migration(t *testing.T) {
	control, baseConfig := openSafeMySQLTestServer(t)
	ctx := context.Background()

	t.Run("exact v12 upgrade creates announcements, maintenance and P3 tables", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		createExactV12AnnouncementsMaintenanceSchema(t, ctx, db)

		if result, err := ApplyLatest(ctx, db); err != nil || result != ResultApplied {
			t.Fatalf("upgrade exact v12 to v13: result=%q err=%v", result, err)
		}
		var announcementColumns, maintenancePlanColumns, campaignTargetColumns, campaignPostmortemColumns, suggestionExecutionColumns, manualRedemptionColumns int
		for _, check := range []struct {
			table string
			want  int
			into  *int
		}{
			{"announcements", 13, &announcementColumns},
			{"maintenance_plans", 11, &maintenancePlanColumns},
			{"campaign_targets", 12, &campaignTargetColumns},
			{"campaign_postmortems", 16, &campaignPostmortemColumns},
			{"ai_suggestion_executions", 14, &suggestionExecutionColumns},
			{"group_buy_manual_redemptions", 11, &manualRedemptionColumns},
		} {
			if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=?`, check.table).Scan(check.into); err != nil {
				t.Fatal(err)
			}
			if *check.into != check.want {
				t.Fatalf("%s columns=%d, want %d", check.table, *check.into, check.want)
			}
		}
		if result, err := ApplyAnnouncementsMaintenanceV13(ctx, db); err != nil || result != ResultAlready {
			t.Fatalf("repeat exact v13 migration: result=%q err=%v", result, err)
		}
		if result, err := ApplyLatest(ctx, db); err != nil || result != ResultAlready {
			t.Fatalf("latest verification after v13 upgrade: result=%q err=%v", result, err)
		}
		assertLatestInventory(t, db)
	})

	t.Run("v12 schema drift is refused without adding v13 tables", func(t *testing.T) {
		db := createTestDatabase(t, control, baseConfig)
		createExactV12AnnouncementsMaintenanceSchema(t, ctx, db)
		if _, err := db.Exec(`ALTER TABLE ai_model_prices ADD COLUMN unexpected_v12_marker INT NULL`); err != nil {
			t.Fatalf("create synthetic v12 drift: %v", err)
		}
		before := snapshotDatabaseTables(t, db)
		if _, err := ApplyAnnouncementsMaintenanceV13(ctx, db); err == nil {
			t.Fatal("v13 unexpectedly accepted a drifted v12 schema")
		}
		if after := snapshotDatabaseTables(t, db); after != before {
			t.Fatalf("v13 drift refusal mutated the database:\nBEFORE:\n%s\nAFTER:\n%s", before, after)
		}
		if countWhere(t, db, "app_schema_migrations", "version = ?", AnnouncementsMaintenanceV13Version) != 0 {
			t.Fatal("v13 ledger row was written after refusing v12 drift")
		}
	})
}

func createExactV12AnnouncementsMaintenanceSchema(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	createExactV11AICostControlSchema(t, ctx, db)
	if result, err := ApplyAICostControlV12(ctx, db); err != nil || result != ResultApplied {
		t.Fatalf("create exact v12 schema: result=%q err=%v", result, err)
	}
}
