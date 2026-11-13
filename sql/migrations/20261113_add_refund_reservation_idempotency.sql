-- Existing successful refunds have no idempotency key; NULL keeps them valid.
-- GORM AutoMigrate also owns these fields/index. This migration is intentionally
-- safe both before and after application startup (and safe to rerun).

SET @refund_migration_sql = IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE() AND table_name = 'refund_records'
     AND column_name = 'idempotency_key') > 0,
  'SELECT 1',
  'ALTER TABLE `refund_records` ADD COLUMN `idempotency_key` VARCHAR(120) NULL AFTER `refund_no`'
);
PREPARE refund_migration_stmt FROM @refund_migration_sql;
EXECUTE refund_migration_stmt;
DEALLOCATE PREPARE refund_migration_stmt;

SET @refund_migration_sql = IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE() AND table_name = 'refund_records'
     AND column_name = 'request_fingerprint') > 0,
  'SELECT 1',
  'ALTER TABLE `refund_records` ADD COLUMN `request_fingerprint` VARCHAR(64) NOT NULL DEFAULT '''' AFTER `idempotency_key`'
);
PREPARE refund_migration_stmt FROM @refund_migration_sql;
EXECUTE refund_migration_stmt;
DEALLOCATE PREPARE refund_migration_stmt;

SET @refund_migration_sql = IF(
  (SELECT COUNT(*) FROM information_schema.statistics
   WHERE table_schema = DATABASE() AND table_name = 'refund_records'
     AND index_name = 'idx_refund_order_idempotency') > 0,
  'SELECT 1',
  'ALTER TABLE `refund_records` ADD UNIQUE KEY `idx_refund_order_idempotency` (`order_id`, `idempotency_key`)'
);
PREPARE refund_migration_stmt FROM @refund_migration_sql;
EXECUTE refund_migration_stmt;
DEALLOCATE PREPARE refund_migration_stmt;
