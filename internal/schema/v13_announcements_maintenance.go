package schema

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	AnnouncementsMaintenanceV13Version  = "schema-v13-announcements-maintenance-20261128"
	AnnouncementsMaintenanceV13Checksum = "4b6eab6a5d4dc3d830777361ca804b04dec4c267e127b04b3ce782ac67432900"
)

// announcementsMaintenanceV13NewTables lists the tables v13 creates.
var announcementsMaintenanceV13NewTables = []string{
	"announcements",
	"maintenance_plans",
	"campaign_targets",
	"campaign_postmortems",
	"ai_suggestion_executions",
	"group_buy_manual_redemptions",
}

// announcementsMaintenanceV13Statements is the exact DDL of the v13 migration, in apply
// order: announcements, maintenance plans, campaign targets, campaign
// postmortems, AI suggestion executions, group-buy manual redemptions.
// All six tables are new, so no pre-existing table is altered and no
// historical fingerprint changes except by the new tables themselves.
var announcementsMaintenanceV13Statements = []string{
	`CREATE TABLE ` + "`announcements`" + ` (
				` + "`id`" + ` bigint unsigned NOT NULL AUTO_INCREMENT,
				` + "`created_at`" + ` datetime(3) NOT NULL,
				` + "`updated_at`" + ` datetime(3) NOT NULL,
				` + "`type`" + ` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL,
				` + "`title`" + ` varchar(200) COLLATE utf8mb4_unicode_ci NOT NULL,
				` + "`body`" + ` varchar(5000) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
				` + "`status`" + ` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'draft',
				` + "`publish_at`" + ` datetime(3) NULL,
				` + "`effective_from`" + ` datetime(3) NULL,
				` + "`effective_to`" + ` datetime(3) NULL,
				` + "`created_by`" + ` bigint unsigned NOT NULL,
				` + "`published_by`" + ` bigint unsigned NOT NULL DEFAULT 0,
				` + "`published_at`" + ` datetime(3) NULL,
				PRIMARY KEY (` + "`id`" + `),
				KEY ` + "`idx_announcements_status_publish`" + ` (` + "`status`" + `, ` + "`publish_at`" + `),
				KEY ` + "`idx_announcements_type`" + ` (` + "`type`" + `),
				CONSTRAINT ` + "`chk_announcements_type`" + ` CHECK (` + "`type`" + ` IN ('system_update', 'promo', 'price_change')),
				CONSTRAINT ` + "`chk_announcements_status`" + ` CHECK (` + "`status`" + ` IN ('draft', 'published', 'archived'))
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	`CREATE TABLE ` + "`maintenance_plans`" + ` (
				` + "`id`" + ` bigint unsigned NOT NULL AUTO_INCREMENT,
				` + "`created_at`" + ` datetime(3) NOT NULL,
				` + "`updated_at`" + ` datetime(3) NOT NULL,
				` + "`title`" + ` varchar(200) COLLATE utf8mb4_unicode_ci NOT NULL,
				` + "`window_start`" + ` datetime(3) NOT NULL,
				` + "`window_end`" + ` datetime(3) NOT NULL,
				` + "`affected_modules`" + ` varchar(200) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'all',
				` + "`status`" + ` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'draft',
				` + "`announcement_id`" + ` bigint unsigned NULL,
				` + "`created_by`" + ` bigint unsigned NOT NULL,
				` + "`updated_by`" + ` bigint unsigned NOT NULL DEFAULT 0,
				PRIMARY KEY (` + "`id`" + `),
				KEY ` + "`idx_maintenance_plans_status_window`" + ` (` + "`status`" + `, ` + "`window_start`" + `),
				KEY ` + "`idx_maintenance_plans_announcement`" + ` (` + "`announcement_id`" + `),
				CONSTRAINT ` + "`chk_maintenance_plans_status`" + ` CHECK (` + "`status`" + ` IN ('draft', 'scheduled', 'active', 'completed', 'cancelled')),
				CONSTRAINT ` + "`chk_maintenance_plans_window`" + ` CHECK (` + "`window_end`" + ` > window_start)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	`CREATE TABLE ` + "`campaign_targets`" + ` (
				` + "`id`" + ` bigint unsigned NOT NULL AUTO_INCREMENT,
				` + "`created_at`" + ` datetime(3) NOT NULL,
				` + "`updated_at`" + ` datetime(3) NOT NULL,
				` + "`merchant_id`" + ` bigint unsigned NOT NULL,
				` + "`campaign_type`" + ` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL,
				` + "`campaign_id`" + ` bigint unsigned NOT NULL,
				` + "`target_orders`" + ` bigint NOT NULL DEFAULT 0,
				` + "`target_amount_cents`" + ` bigint NOT NULL DEFAULT 0,
				` + "`target_new_customers`" + ` bigint NOT NULL DEFAULT 0,
				` + "`period_start`" + ` date NOT NULL,
				` + "`period_end`" + ` date NOT NULL,
				` + "`created_by`" + ` bigint unsigned NOT NULL,
				PRIMARY KEY (` + "`id`" + `),
				UNIQUE KEY ` + "`uk_campaign_targets_scope`" + ` (` + "`merchant_id`" + `, ` + "`campaign_type`" + `, ` + "`campaign_id`" + `, ` + "`period_start`" + `),
				KEY ` + "`idx_campaign_targets_merchant`" + ` (` + "`merchant_id`" + `),
				CONSTRAINT ` + "`chk_campaign_targets_type`" + ` CHECK (` + "`campaign_type`" + ` IN ('promotion', 'share_campaign')),
				CONSTRAINT ` + "`chk_campaign_targets_orders`" + ` CHECK (` + "`target_orders`" + ` >= 0),
				CONSTRAINT ` + "`chk_campaign_targets_amount`" + ` CHECK (` + "`target_amount_cents`" + ` >= 0),
				CONSTRAINT ` + "`chk_campaign_targets_customers`" + ` CHECK (` + "`target_new_customers`" + ` >= 0),
				CONSTRAINT ` + "`chk_campaign_targets_period`" + ` CHECK (` + "`period_end`" + ` >= period_start)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	`CREATE TABLE ` + "`campaign_postmortems`" + ` (
				` + "`id`" + ` bigint unsigned NOT NULL AUTO_INCREMENT,
				` + "`created_at`" + ` datetime(3) NOT NULL,
				` + "`updated_at`" + ` datetime(3) NOT NULL,
				` + "`merchant_id`" + ` bigint unsigned NOT NULL,
				` + "`campaign_type`" + ` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL,
				` + "`campaign_id`" + ` bigint unsigned NOT NULL,
				` + "`period_start`" + ` date NOT NULL,
				` + "`period_end`" + ` date NOT NULL,
				` + "`target_orders`" + ` bigint NOT NULL DEFAULT 0,
				` + "`target_amount_cents`" + ` bigint NOT NULL DEFAULT 0,
				` + "`actual_orders`" + ` bigint NOT NULL DEFAULT 0,
				` + "`actual_amount_cents`" + ` bigint NOT NULL DEFAULT 0,
				` + "`actual_new_customers`" + ` bigint NOT NULL DEFAULT 0,
				` + "`conclusion`" + ` varchar(2000) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
				` + "`generated_by`" + ` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'system',
				` + "`generated_at`" + ` datetime(3) NOT NULL,
				PRIMARY KEY (` + "`id`" + `),
				KEY ` + "`idx_campaign_postmortems_merchant`" + ` (` + "`merchant_id`" + `),
				KEY ` + "`idx_campaign_postmortems_campaign`" + ` (` + "`campaign_type`" + `, ` + "`campaign_id`" + `, ` + "`period_start`" + `),
				CONSTRAINT ` + "`chk_campaign_postmortems_type`" + ` CHECK (` + "`campaign_type`" + ` IN ('promotion', 'share_campaign')),
				CONSTRAINT ` + "`chk_campaign_postmortems_target_orders`" + ` CHECK (` + "`target_orders`" + ` >= 0),
				CONSTRAINT ` + "`chk_campaign_postmortems_target_amount`" + ` CHECK (` + "`target_amount_cents`" + ` >= 0),
				CONSTRAINT ` + "`chk_campaign_postmortems_actual_orders`" + ` CHECK (` + "`actual_orders`" + ` >= 0),
				CONSTRAINT ` + "`chk_campaign_postmortems_actual_amount`" + ` CHECK (` + "`actual_amount_cents`" + ` >= 0),
				CONSTRAINT ` + "`chk_campaign_postmortems_actual_customers`" + ` CHECK (` + "`actual_new_customers`" + ` >= 0),
				CONSTRAINT ` + "`chk_campaign_postmortems_generated_by`" + ` CHECK (` + "`generated_by`" + ` IN ('system', 'ai', 'manual')),
				CONSTRAINT ` + "`chk_campaign_postmortems_period`" + ` CHECK (` + "`period_end`" + ` >= period_start)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	`CREATE TABLE ` + "`ai_suggestion_executions`" + ` (
				` + "`id`" + ` bigint unsigned NOT NULL AUTO_INCREMENT,
				` + "`created_at`" + ` datetime(3) NOT NULL,
				` + "`updated_at`" + ` datetime(3) NOT NULL,
				` + "`merchant_id`" + ` bigint unsigned NOT NULL,
				` + "`store_id`" + ` bigint unsigned NULL,
				` + "`source`" + ` varchar(80) COLLATE utf8mb4_unicode_ci NOT NULL,
				` + "`title`" + ` varchar(200) COLLATE utf8mb4_unicode_ci NOT NULL,
				` + "`content_snapshot`" + ` varchar(2000) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
				` + "`status`" + ` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending',
				` + "`executed_at`" + ` datetime(3) NULL,
				` + "`related_order_id`" + ` bigint unsigned NULL,
				` + "`related_coupon_id`" + ` bigint unsigned NULL,
				` + "`effect_note`" + ` varchar(1000) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
				` + "`updated_by`" + ` bigint unsigned NOT NULL DEFAULT 0,
				PRIMARY KEY (` + "`id`" + `),
				KEY ` + "`idx_ai_suggestion_executions_merchant`" + ` (` + "`merchant_id`" + `),
				KEY ` + "`idx_ai_suggestion_executions_status`" + ` (` + "`status`" + `),
				KEY ` + "`idx_ai_suggestion_executions_source`" + ` (` + "`source`" + `),
				CONSTRAINT ` + "`chk_ai_suggestion_executions_status`" + ` CHECK (` + "`status`" + ` IN ('pending', 'adopted', 'ignored', 'executed'))
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	`CREATE TABLE ` + "`group_buy_manual_redemptions`" + ` (
				` + "`id`" + ` bigint unsigned NOT NULL AUTO_INCREMENT,
				` + "`created_at`" + ` datetime(3) NOT NULL,
				` + "`updated_at`" + ` datetime(3) NOT NULL,
				` + "`merchant_id`" + ` bigint unsigned NOT NULL,
				` + "`store_id`" + ` bigint unsigned NULL,
				` + "`link_id`" + ` bigint unsigned NOT NULL,
				` + "`redemption_date`" + ` date NOT NULL,
				` + "`redemption_count`" + ` bigint NOT NULL DEFAULT 0,
				` + "`redemption_amount_cents`" + ` bigint NOT NULL DEFAULT 0,
				` + "`note`" + ` varchar(500) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
				` + "`recorded_by`" + ` bigint unsigned NOT NULL,
				PRIMARY KEY (` + "`id`" + `),
				KEY ` + "`idx_group_buy_manual_redemptions_link`" + ` (` + "`link_id`" + `, ` + "`redemption_date`" + `),
				KEY ` + "`idx_group_buy_manual_redemptions_merchant`" + ` (` + "`merchant_id`" + `),
				CONSTRAINT ` + "`chk_group_buy_manual_redemptions_count`" + ` CHECK (` + "`redemption_count`" + ` >= 0),
				CONSTRAINT ` + "`chk_group_buy_manual_redemptions_amount`" + ` CHECK (` + "`redemption_amount_cents`" + ` >= 0)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
}

// announcementsMaintenanceV13NewColumns lists the columns v13 adds to
// pre-existing tables. v13 only creates new tables, so the list is empty
// and no historical fingerprint needs a column exclusion for v13.
var announcementsMaintenanceV13NewColumns = []schemaColumn{}

func validateAnnouncementsMaintenanceV13Definition() error {
	material := append([]string{AnnouncementsMaintenanceV13Version}, announcementsMaintenanceV13Statements...)
	sum := sha256.Sum256([]byte(strings.Join(material, "\n")))
	if got := fmt.Sprintf("%x", sum); got != AnnouncementsMaintenanceV13Checksum {
		return fmt.Errorf("announcements and maintenance v13 checksum mismatch: definition %s, expected %s", got, AnnouncementsMaintenanceV13Checksum)
	}
	return nil
}

// ApplyAnnouncementsMaintenanceV13 appends the announcements, maintenance
// plans, campaign targets, campaign postmortems, AI suggestion executions
// and group-buy manual redemptions tables to an exact managed v12 schema.
// The v1-v12 schema and fingerprints remain untouched.
func ApplyAnnouncementsMaintenanceV13(ctx context.Context, db *sql.DB) (Result, error) {
	base, err := loadBaseline()
	if err != nil {
		return "", err
	}
	v2, err := loadManualReviewV2()
	if err != nil {
		return "", err
	}
	v3, err := loadAdminAuthV3()
	if err != nil {
		return "", err
	}
	v4, err := loadOTPSafetyV4()
	if err != nil {
		return "", err
	}
	v5, err := loadEmailOTPSafetyV5()
	if err != nil {
		return "", err
	}
	v6, err := loadWholesaleRetailV6()
	if err != nil {
		return "", err
	}
	v7, err := loadPlatformExpensesV7()
	if err != nil {
		return "", err
	}
	v8, err := loadPlatformExpenseReversalsV8()
	if err != nil {
		return "", err
	}
	v9, err := loadTableOrderingV9()
	if err != nil {
		return "", err
	}
	if err := validateAnnouncementsMaintenanceV13Definition(); err != nil {
		return "", err
	}
	if db == nil {
		return "", errors.New("database handle is required")
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return "", fmt.Errorf("get dedicated database connection: %w", err)
	}
	defer conn.Close()
	var databaseName sql.NullString
	if err := conn.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&databaseName); err != nil {
		return "", fmt.Errorf("read selected database: %w", err)
	}
	if !databaseName.Valid || strings.TrimSpace(databaseName.String) == "" {
		return "", errors.New("MYSQL_DSN must select an existing target database")
	}
	lockName := advisoryLockName(databaseName.String)
	var lockResult sql.NullInt64
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, 0)", lockName).Scan(&lockResult); err != nil {
		return "", fmt.Errorf("acquire schema migration lock: %w", err)
	}
	if !lockResult.Valid || lockResult.Int64 != 1 {
		return "", fmt.Errorf("another schema migration holds the lock for database %q", databaseName.String)
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(releaseCtx, "SELECT RELEASE_LOCK(?)", lockName)
	}()

	objects, err := inspectObjects(ctx, conn)
	if err != nil {
		return "", err
	}
	if err := verifyManagedSchemaV12(ctx, conn, objects, base, v2, v3, v4, v5, v6, v7, v8, v9); err == nil {
		if err := applyAnnouncementsMaintenanceV13(ctx, conn, base, v2, v3, v4, v5, v6, v7, v8, v9); err != nil {
			return "", err
		}
		return ResultApplied, nil
	}
	if err := verifyManagedSchemaV13(ctx, conn, objects, base, v2, v3, v4, v5, v6, v7, v8, v9); err != nil {
		return "", fmt.Errorf("refusing non-exact managed schema for v13 announcements and maintenance migration: %w", err)
	}
	return ResultAlready, nil
}

func applyAnnouncementsMaintenanceV13(ctx context.Context, conn *sql.Conn, base baselineDefinition, v2 incrementalMigrationDefinition, v3 adminAuthV3Definition, v4 otpSafetyV4Definition, v5 emailOTPSafetyV5Definition, v6 wholesaleRetailV6Definition, v7 platformExpensesV7Definition, v8 platformExpenseReversalsV8Definition, v9 tableOrderingV9Definition) error {
	if _, err := conn.ExecContext(ctx, `INSERT INTO `+ledgerTable+` (version, checksum, state) VALUES (?, ?, 'applying')`, AnnouncementsMaintenanceV13Version, AnnouncementsMaintenanceV13Checksum); err != nil {
		return fmt.Errorf("record v13 announcements and maintenance migration start: %w", err)
	}
	for i, statement := range announcementsMaintenanceV13Statements {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply v13 announcements and maintenance change %d (v13 ledger remains incomplete): %w", i+1, err)
		}
	}
	if err := verifyAnnouncementsMaintenanceV13Tables(ctx, conn); err != nil {
		return fmt.Errorf("verify v13 announcements and maintenance tables (v13 ledger remains incomplete): %w", err)
	}
	objects, err := inspectObjects(ctx, conn)
	if err != nil {
		return err
	}
	if err := verifyObjectInventoryV13(objects, base, v2, v7, v8, v9); err != nil {
		return fmt.Errorf("post-apply v13 object inventory verification failed (v13 ledger remains incomplete): %w", err)
	}
	// The stored v12 fingerprint is computed over the schema without the v13
	// tables; v13 adds no columns to pre-existing tables, so no column
	// exclusion is needed here.
	v12Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, announcementsMaintenanceV13NewTables, nil, nil)
	if err != nil {
		return fmt.Errorf("compute immutable v12 schema fingerprint during v13 migration: %w", err)
	}
	var storedV12 string
	if err := conn.QueryRowContext(ctx, "SELECT schema_fingerprint FROM "+ledgerTable+" WHERE version=? AND checksum=? AND state='applied'", AICostControlV12Version, AICostControlV12Checksum).Scan(&storedV12); err != nil {
		return fmt.Errorf("read immutable v12 schema fingerprint during v13 migration: %w", err)
	}
	if storedV12 != v12Fingerprint {
		return fmt.Errorf("refusing v12 schema drift during v13 migration: stored %s, actual %s", storedV12, v12Fingerprint)
	}
	fingerprint, err := schemaFingerprint(ctx, conn)
	if err != nil {
		return fmt.Errorf("compute v13 installed schema fingerprint: %w", err)
	}
	result, err := conn.ExecContext(ctx, `UPDATE `+ledgerTable+` SET state='applied', schema_fingerprint=?, applied_at=CURRENT_TIMESTAMP WHERE version=? AND checksum=? AND state='applying'`, fingerprint, AnnouncementsMaintenanceV13Version, AnnouncementsMaintenanceV13Checksum)
	if err != nil {
		return fmt.Errorf("complete v13 schema version ledger: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("verify v13 schema version ledger update: %w", err)
	}
	if rows != 1 {
		return errors.New("v13 schema version ledger did not transition exactly once; it remains incomplete")
	}
	_ = v3
	_ = v4
	_ = v5
	_ = v6
	return nil
}

func verifyManagedSchemaV13(ctx context.Context, conn *sql.Conn, objects schemaObjects, base baselineDefinition, v2 incrementalMigrationDefinition, v3 adminAuthV3Definition, v4 otpSafetyV4Definition, v5 emailOTPSafetyV5Definition, v6 wholesaleRetailV6Definition, v7 platformExpensesV7Definition, v8 platformExpenseReversalsV8Definition, v9 tableOrderingV9Definition) error {
	if err := verifyAnnouncementsMaintenanceV13Tables(ctx, conn); err != nil {
		return err
	}
	if err := verifyObjectInventoryV13(objects, base, v2, v7, v8, v9); err != nil {
		return err
	}
	combinedNewColumns := append(append([]schemaColumn(nil), orderItemsV11NewColumns...), aiCostControlV12NewColumns...)
	futureTables := append(append(append(append([]string(nil), tableOrderingV9Order...), "order_items"), aiCostControlV12NewTables...), announcementsMaintenanceV13NewTables...)
	if err := verifyPlatformExpenseReversalsV8HistoricalFingerprintsExcludingColumns(ctx, conn, base, v2, v3, v4, v5, v6, v7, v8, combinedNewColumns, futureTables...); err != nil {
		return err
	}
	rows, err := conn.QueryContext(ctx, "SELECT version, checksum, state, schema_fingerprint FROM "+ledgerTable+" ORDER BY version")
	if err != nil {
		return fmt.Errorf("read v13 schema version ledger: %w", err)
	}
	type ledgerEntry struct {
		version, checksum, state string
		fingerprint              sql.NullString
	}
	entries := make([]ledgerEntry, 0, 13)
	for rows.Next() {
		var entry ledgerEntry
		if err := rows.Scan(&entry.version, &entry.checksum, &entry.state, &entry.fingerprint); err != nil {
			rows.Close()
			return err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if len(entries) != 13 {
		return fmt.Errorf("refusing v13 ledger with %d rows; expected exactly immutable v1-v13 rows", len(entries))
	}
	expected := []struct{ version, checksum string }{
		{BaselineVersion, base.checksum}, {v2.version, v2.checksum}, {v3.version, v3.checksum}, {v4.version, v4.checksum},
		{v5.version, v5.checksum}, {v6.version, v6.checksum}, {v7.version, v7.checksum}, {v8.version, v8.checksum},
		{v9.version, v9.checksum}, {TableLabelsV10Version, TableLabelsV10Checksum}, {OrderItemsV11Version, OrderItemsV11Checksum},
		{AICostControlV12Version, AICostControlV12Checksum},
		{AnnouncementsMaintenanceV13Version, AnnouncementsMaintenanceV13Checksum},
	}
	byVersion := make(map[string]ledgerEntry, len(entries))
	for _, entry := range entries {
		byVersion[entry.version] = entry
	}
	for _, want := range expected {
		entry, ok := byVersion[want.version]
		if !ok || entry.checksum != want.checksum || entry.state != "applied" || !entry.fingerprint.Valid || entry.fingerprint.String == "" {
			return fmt.Errorf("refusing missing, altered, or incomplete %s ledger entry", want.version)
		}
	}
	v10NewColumns := []schemaColumn{{table: "store_tables", column: "table_label"}, {table: "table_order_details", column: "table_label_snapshot"}}
	// v9 check: exclude v12/v13 tables + order_items, and v10/v11/v12 new columns.
	v9ExcludedColumns := append(append([]schemaColumn(nil), combinedNewColumns...), v10NewColumns...)
	v9Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, append(append(append([]string(nil), "order_items"), aiCostControlV12NewTables...), announcementsMaintenanceV13NewTables...), nil, v9ExcludedColumns)
	if err != nil {
		return fmt.Errorf("verify immutable v9 schema fingerprint: %w", err)
	}
	if stored := byVersion[v9.version].fingerprint.String; stored != v9Fingerprint {
		return fmt.Errorf("refusing v9 schema drift: stored fingerprint %s, actual %s", stored, v9Fingerprint)
	}
	v10ExcludedColumns := combinedNewColumns
	v10Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, append(append(append([]string(nil), "order_items"), aiCostControlV12NewTables...), announcementsMaintenanceV13NewTables...), nil, v10ExcludedColumns)
	if err != nil {
		return fmt.Errorf("verify immutable v10 schema fingerprint: %w", err)
	}
	if stored := byVersion[TableLabelsV10Version].fingerprint.String; stored != v10Fingerprint {
		return fmt.Errorf("refusing v10 schema drift: stored fingerprint %s, actual %s", stored, v10Fingerprint)
	}
	// v11 check: only the v12 columns are excluded (orders.channel is part
	// of the stored v11 fingerprint), and the v12/v13 tables are excluded.
	// v10/v9 checks above exclude the combined (v11+v12) columns, as in
	// verifyManagedSchemaV12.
	v11Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, append(append([]string(nil), aiCostControlV12NewTables...), announcementsMaintenanceV13NewTables...), nil, aiCostControlV12NewColumns)
	if err != nil {
		return fmt.Errorf("verify immutable v11 schema fingerprint: %w", err)
	}
	if stored := byVersion[OrderItemsV11Version].fingerprint.String; stored != v11Fingerprint {
		return fmt.Errorf("refusing v11 schema drift: stored fingerprint %s, actual %s", stored, v11Fingerprint)
	}
	// v12 check: only the v13 tables are excluded (the v12 columns are part
	// of the stored v12 fingerprint); v13 adds no columns to old tables.
	v12Fingerprint, err := schemaFingerprintExcludingColumns(ctx, conn, announcementsMaintenanceV13NewTables, nil, nil)
	if err != nil {
		return fmt.Errorf("verify immutable v12 schema fingerprint: %w", err)
	}
	if stored := byVersion[AICostControlV12Version].fingerprint.String; stored != v12Fingerprint {
		return fmt.Errorf("refusing v12 schema drift: stored fingerprint %s, actual %s", stored, v12Fingerprint)
	}
	actualV13, err := schemaFingerprint(ctx, conn)
	if err != nil {
		return fmt.Errorf("verify v13 managed schema fingerprint: %w", err)
	}
	if stored := byVersion[AnnouncementsMaintenanceV13Version].fingerprint.String; stored != actualV13 {
		return fmt.Errorf("refusing v13 schema drift: stored fingerprint %s, actual %s", stored, actualV13)
	}
	_ = v6
	return nil
}

// verifyAnnouncementsMaintenanceV13Tables checks the exact shape of the six
// new tables. v13 adds no columns to pre-existing tables and seeds no rows.
func verifyAnnouncementsMaintenanceV13Tables(ctx context.Context, conn *sql.Conn) error {
	type columnSpec struct {
		name, columnType, nullable string
		defaultValue               string
		hasDefault                 bool
		extra, collation           string
	}
	col := func(name, columnType, nullable string, defaultValue string, hasDefault bool, extra, collation string) columnSpec {
		return columnSpec{name: name, columnType: columnType, nullable: nullable, defaultValue: defaultValue, hasDefault: hasDefault, extra: extra, collation: collation}
	}
	tables := []struct {
		name    string
		columns []columnSpec
		indexes []string
		checks  []string
	}{
		{
			name: "announcements",
			columns: []columnSpec{
				col("id", "bigint unsigned", "NO", "", false, "auto_increment", ""),
				col("created_at", "datetime(3)", "NO", "", false, "", ""),
				col("updated_at", "datetime(3)", "NO", "", false, "", ""),
				col("type", "varchar(24)", "NO", "", false, "", "utf8mb4_unicode_ci"),
				col("title", "varchar(200)", "NO", "", false, "", "utf8mb4_unicode_ci"),
				col("body", "varchar(5000)", "NO", "", true, "", "utf8mb4_unicode_ci"),
				col("status", "varchar(16)", "NO", "draft", true, "", "utf8mb4_unicode_ci"),
				col("publish_at", "datetime(3)", "YES", "", false, "", ""),
				col("effective_from", "datetime(3)", "YES", "", false, "", ""),
				col("effective_to", "datetime(3)", "YES", "", false, "", ""),
				col("created_by", "bigint unsigned", "NO", "", false, "", ""),
				col("published_by", "bigint unsigned", "NO", "0", true, "", ""),
				col("published_at", "datetime(3)", "YES", "", false, "", ""),
			},
			indexes: []string{
				"PRIMARY|0|1|id|NULL|BTREE",
				"idx_announcements_status_publish|1|1|status|NULL|BTREE",
				"idx_announcements_status_publish|1|2|publish_at|NULL|BTREE",
				"idx_announcements_type|1|1|type|NULL|BTREE",
			},
			checks: []string{
				"chk_announcements_status|announcements|statusin'draft','published','archived'|YES",
				"chk_announcements_type|announcements|typein'system_update','promo','price_change'|YES",
			},
		},
		{
			name: "maintenance_plans",
			columns: []columnSpec{
				col("id", "bigint unsigned", "NO", "", false, "auto_increment", ""),
				col("created_at", "datetime(3)", "NO", "", false, "", ""),
				col("updated_at", "datetime(3)", "NO", "", false, "", ""),
				col("title", "varchar(200)", "NO", "", false, "", "utf8mb4_unicode_ci"),
				col("window_start", "datetime(3)", "NO", "", false, "", ""),
				col("window_end", "datetime(3)", "NO", "", false, "", ""),
				col("affected_modules", "varchar(200)", "NO", "all", true, "", "utf8mb4_unicode_ci"),
				col("status", "varchar(16)", "NO", "draft", true, "", "utf8mb4_unicode_ci"),
				col("announcement_id", "bigint unsigned", "YES", "", false, "", ""),
				col("created_by", "bigint unsigned", "NO", "", false, "", ""),
				col("updated_by", "bigint unsigned", "NO", "0", true, "", ""),
			},
			indexes: []string{
				"PRIMARY|0|1|id|NULL|BTREE",
				"idx_maintenance_plans_status_window|1|1|status|NULL|BTREE",
				"idx_maintenance_plans_status_window|1|2|window_start|NULL|BTREE",
				"idx_maintenance_plans_announcement|1|1|announcement_id|NULL|BTREE",
			},
			checks: []string{
				"chk_maintenance_plans_status|maintenance_plans|statusin'draft','scheduled','active','completed','cancelled'|YES",
				"chk_maintenance_plans_window|maintenance_plans|window_end>window_start|YES",
			},
		},
		{
			name: "campaign_targets",
			columns: []columnSpec{
				col("id", "bigint unsigned", "NO", "", false, "auto_increment", ""),
				col("created_at", "datetime(3)", "NO", "", false, "", ""),
				col("updated_at", "datetime(3)", "NO", "", false, "", ""),
				col("merchant_id", "bigint unsigned", "NO", "", false, "", ""),
				col("campaign_type", "varchar(24)", "NO", "", false, "", "utf8mb4_unicode_ci"),
				col("campaign_id", "bigint unsigned", "NO", "", false, "", ""),
				col("target_orders", "bigint", "NO", "0", true, "", ""),
				col("target_amount_cents", "bigint", "NO", "0", true, "", ""),
				col("target_new_customers", "bigint", "NO", "0", true, "", ""),
				col("period_start", "date", "NO", "", false, "", ""),
				col("period_end", "date", "NO", "", false, "", ""),
				col("created_by", "bigint unsigned", "NO", "", false, "", ""),
			},
			indexes: []string{
				"PRIMARY|0|1|id|NULL|BTREE",
				"uk_campaign_targets_scope|0|1|merchant_id|NULL|BTREE",
				"uk_campaign_targets_scope|0|2|campaign_type|NULL|BTREE",
				"uk_campaign_targets_scope|0|3|campaign_id|NULL|BTREE",
				"uk_campaign_targets_scope|0|4|period_start|NULL|BTREE",
				"idx_campaign_targets_merchant|1|1|merchant_id|NULL|BTREE",
			},
			checks: []string{
				"chk_campaign_targets_amount|campaign_targets|target_amount_cents>=0|YES",
				"chk_campaign_targets_customers|campaign_targets|target_new_customers>=0|YES",
				"chk_campaign_targets_orders|campaign_targets|target_orders>=0|YES",
				"chk_campaign_targets_period|campaign_targets|period_end>=period_start|YES",
				"chk_campaign_targets_type|campaign_targets|campaign_typein'promotion','share_campaign'|YES",
			},
		},
		{
			name: "campaign_postmortems",
			columns: []columnSpec{
				col("id", "bigint unsigned", "NO", "", false, "auto_increment", ""),
				col("created_at", "datetime(3)", "NO", "", false, "", ""),
				col("updated_at", "datetime(3)", "NO", "", false, "", ""),
				col("merchant_id", "bigint unsigned", "NO", "", false, "", ""),
				col("campaign_type", "varchar(24)", "NO", "", false, "", "utf8mb4_unicode_ci"),
				col("campaign_id", "bigint unsigned", "NO", "", false, "", ""),
				col("period_start", "date", "NO", "", false, "", ""),
				col("period_end", "date", "NO", "", false, "", ""),
				col("target_orders", "bigint", "NO", "0", true, "", ""),
				col("target_amount_cents", "bigint", "NO", "0", true, "", ""),
				col("actual_orders", "bigint", "NO", "0", true, "", ""),
				col("actual_amount_cents", "bigint", "NO", "0", true, "", ""),
				col("actual_new_customers", "bigint", "NO", "0", true, "", ""),
				col("conclusion", "varchar(2000)", "NO", "", true, "", "utf8mb4_unicode_ci"),
				col("generated_by", "varchar(16)", "NO", "system", true, "", "utf8mb4_unicode_ci"),
				col("generated_at", "datetime(3)", "NO", "", false, "", ""),
			},
			indexes: []string{
				"PRIMARY|0|1|id|NULL|BTREE",
				"idx_campaign_postmortems_merchant|1|1|merchant_id|NULL|BTREE",
				"idx_campaign_postmortems_campaign|1|1|campaign_type|NULL|BTREE",
				"idx_campaign_postmortems_campaign|1|2|campaign_id|NULL|BTREE",
				"idx_campaign_postmortems_campaign|1|3|period_start|NULL|BTREE",
			},
			checks: []string{
				"chk_campaign_postmortems_actual_amount|campaign_postmortems|actual_amount_cents>=0|YES",
				"chk_campaign_postmortems_actual_customers|campaign_postmortems|actual_new_customers>=0|YES",
				"chk_campaign_postmortems_actual_orders|campaign_postmortems|actual_orders>=0|YES",
				"chk_campaign_postmortems_generated_by|campaign_postmortems|generated_byin'system','ai','manual'|YES",
				"chk_campaign_postmortems_period|campaign_postmortems|period_end>=period_start|YES",
				"chk_campaign_postmortems_target_amount|campaign_postmortems|target_amount_cents>=0|YES",
				"chk_campaign_postmortems_target_orders|campaign_postmortems|target_orders>=0|YES",
				"chk_campaign_postmortems_type|campaign_postmortems|campaign_typein'promotion','share_campaign'|YES",
			},
		},
		{
			name: "ai_suggestion_executions",
			columns: []columnSpec{
				col("id", "bigint unsigned", "NO", "", false, "auto_increment", ""),
				col("created_at", "datetime(3)", "NO", "", false, "", ""),
				col("updated_at", "datetime(3)", "NO", "", false, "", ""),
				col("merchant_id", "bigint unsigned", "NO", "", false, "", ""),
				col("store_id", "bigint unsigned", "YES", "", false, "", ""),
				col("source", "varchar(80)", "NO", "", false, "", "utf8mb4_unicode_ci"),
				col("title", "varchar(200)", "NO", "", false, "", "utf8mb4_unicode_ci"),
				col("content_snapshot", "varchar(2000)", "NO", "", true, "", "utf8mb4_unicode_ci"),
				col("status", "varchar(16)", "NO", "pending", true, "", "utf8mb4_unicode_ci"),
				col("executed_at", "datetime(3)", "YES", "", false, "", ""),
				col("related_order_id", "bigint unsigned", "YES", "", false, "", ""),
				col("related_coupon_id", "bigint unsigned", "YES", "", false, "", ""),
				col("effect_note", "varchar(1000)", "NO", "", true, "", "utf8mb4_unicode_ci"),
				col("updated_by", "bigint unsigned", "NO", "0", true, "", ""),
			},
			indexes: []string{
				"PRIMARY|0|1|id|NULL|BTREE",
				"idx_ai_suggestion_executions_merchant|1|1|merchant_id|NULL|BTREE",
				"idx_ai_suggestion_executions_status|1|1|status|NULL|BTREE",
				"idx_ai_suggestion_executions_source|1|1|source|NULL|BTREE",
			},
			checks: []string{
				"chk_ai_suggestion_executions_status|ai_suggestion_executions|statusin'pending','adopted','ignored','executed'|YES",
			},
		},
		{
			name: "group_buy_manual_redemptions",
			columns: []columnSpec{
				col("id", "bigint unsigned", "NO", "", false, "auto_increment", ""),
				col("created_at", "datetime(3)", "NO", "", false, "", ""),
				col("updated_at", "datetime(3)", "NO", "", false, "", ""),
				col("merchant_id", "bigint unsigned", "NO", "", false, "", ""),
				col("store_id", "bigint unsigned", "YES", "", false, "", ""),
				col("link_id", "bigint unsigned", "NO", "", false, "", ""),
				col("redemption_date", "date", "NO", "", false, "", ""),
				col("redemption_count", "bigint", "NO", "0", true, "", ""),
				col("redemption_amount_cents", "bigint", "NO", "0", true, "", ""),
				col("note", "varchar(500)", "NO", "", true, "", "utf8mb4_unicode_ci"),
				col("recorded_by", "bigint unsigned", "NO", "", false, "", ""),
			},
			indexes: []string{
				"PRIMARY|0|1|id|NULL|BTREE",
				"idx_group_buy_manual_redemptions_link|1|1|link_id|NULL|BTREE",
				"idx_group_buy_manual_redemptions_link|1|2|redemption_date|NULL|BTREE",
				"idx_group_buy_manual_redemptions_merchant|1|1|merchant_id|NULL|BTREE",
			},
			checks: []string{
				"chk_group_buy_manual_redemptions_amount|group_buy_manual_redemptions|redemption_amount_cents>=0|YES",
				"chk_group_buy_manual_redemptions_count|group_buy_manual_redemptions|redemption_count>=0|YES",
			},
		},
	}
	for _, table := range tables {
		var engine, tableCollation string
		if err := conn.QueryRowContext(ctx, "SELECT ENGINE, COALESCE(TABLE_COLLATION,'') FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=?", table.name).Scan(&engine, &tableCollation); err != nil {
			return fmt.Errorf("inspect %s engine and collation: %w", table.name, err)
		}
		if engine != "InnoDB" || tableCollation != "utf8mb4_unicode_ci" {
			return fmt.Errorf("%s engine/collation mismatch: found %s/%s, expected InnoDB/utf8mb4_unicode_ci", table.name, engine, tableCollation)
		}
		rows, err := conn.QueryContext(ctx, "SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_DEFAULT, EXTRA, COALESCE(COLLATION_NAME,'') FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? ORDER BY ORDINAL_POSITION", table.name)
		if err != nil {
			return fmt.Errorf("inspect %s columns: %w", table.name, err)
		}
		actual := make([]columnSpec, 0)
		for rows.Next() {
			var got columnSpec
			var defaultValue sql.NullString
			if err := rows.Scan(&got.name, &got.columnType, &got.nullable, &defaultValue, &got.extra, &got.collation); err != nil {
				rows.Close()
				return err
			}
			got.hasDefault = defaultValue.Valid
			if defaultValue.Valid {
				got.defaultValue = defaultValue.String
			}
			actual = append(actual, got)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		if len(actual) != len(table.columns) {
			return fmt.Errorf("%s column count mismatch: found %d, expected %d", table.name, len(actual), len(table.columns))
		}
		for i, want := range table.columns {
			if actual[i] != want {
				return fmt.Errorf("%s column %d mismatch: found %+v, expected %+v", table.name, i+1, actual[i], want)
			}
		}
		rows, err = conn.QueryContext(ctx, "SELECT INDEX_NAME, NON_UNIQUE, SEQ_IN_INDEX, COLUMN_NAME, SUB_PART, INDEX_TYPE FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? ORDER BY INDEX_NAME, SEQ_IN_INDEX", table.name)
		if err != nil {
			return fmt.Errorf("inspect %s indexes: %w", table.name, err)
		}
		actualIndexes := make([]string, 0)
		for rows.Next() {
			var name, column, indexType string
			var nonUnique, sequence int
			var subPart sql.NullInt64
			if err := rows.Scan(&name, &nonUnique, &sequence, &column, &subPart, &indexType); err != nil {
				rows.Close()
				return err
			}
			subPartValue := "NULL"
			if subPart.Valid {
				subPartValue = fmt.Sprint(subPart.Int64)
			}
			actualIndexes = append(actualIndexes, fmt.Sprintf("%s|%d|%d|%s|%s|%s", name, nonUnique, sequence, column, subPartValue, indexType))
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		sort.Strings(actualIndexes)
		wantIndexes := append([]string(nil), table.indexes...)
		sort.Strings(wantIndexes)
		if !equalStrings(actualIndexes, wantIndexes) {
			return fmt.Errorf("%s exact index definition mismatch: found %v, expected %v", table.name, actualIndexes, wantIndexes)
		}
		rows, err = conn.QueryContext(ctx, `SELECT tc.CONSTRAINT_NAME, tc.TABLE_NAME, cc.CHECK_CLAUSE, tc.ENFORCED
			FROM information_schema.TABLE_CONSTRAINTS tc
			JOIN information_schema.CHECK_CONSTRAINTS cc
			ON cc.CONSTRAINT_SCHEMA=tc.CONSTRAINT_SCHEMA AND cc.CONSTRAINT_NAME=tc.CONSTRAINT_NAME
			WHERE tc.TABLE_SCHEMA=DATABASE() AND tc.TABLE_NAME=? AND tc.CONSTRAINT_TYPE='CHECK'
			ORDER BY tc.CONSTRAINT_NAME`, table.name)
		if err != nil {
			return fmt.Errorf("inspect %s CHECK constraints: %w", table.name, err)
		}
		actualChecks := make([]string, 0)
		for rows.Next() {
			var name, tableName, clause, enforced string
			if err := rows.Scan(&name, &tableName, &clause, &enforced); err != nil {
				rows.Close()
				return err
			}
			actualChecks = append(actualChecks, fmt.Sprintf("%s|%s|%s|%s", name, tableName, normalizeCheckClause(clause), enforced))
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		sort.Strings(actualChecks)
		wantChecks := append([]string(nil), table.checks...)
		sort.Strings(wantChecks)
		if !equalStrings(actualChecks, wantChecks) {
			return fmt.Errorf("%s exact CHECK-constraint definition mismatch: found %v, expected %v", table.name, actualChecks, wantChecks)
		}
	}
	return nil
}

func verifyObjectInventoryV13(objects schemaObjects, base baselineDefinition, v2 incrementalMigrationDefinition, v7 platformExpensesV7Definition, v8 platformExpenseReversalsV8Definition, v9 tableOrderingV9Definition) error {
	filtered := objects
	for _, name := range announcementsMaintenanceV13NewTables {
		if !containsSchemaName(objects.tables, name) {
			return fmt.Errorf("v13 table %q is missing", name)
		}
		filtered.tables = removeSchemaName(filtered.tables, name)
	}
	if err := verifyObjectInventoryV12(filtered, base, v2, v7, v8, v9); err != nil {
		return err
	}
	return nil
}
