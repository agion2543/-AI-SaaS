-- Snapshot the quoted merchant plan terms on each subscription order.
-- Existing orders intentionally remain NULL: historical terms cannot be
-- reconstructed safely from a plan row that may already have been edited.
SET @subscription_snapshot_sql = IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE() AND table_name = 'orders'
     AND column_name = 'merchant_plan_name_snapshot') > 0,
  'SELECT 1',
  'ALTER TABLE `orders` ADD COLUMN `merchant_plan_name_snapshot` VARCHAR(80) NULL'
);
PREPARE subscription_snapshot_stmt FROM @subscription_snapshot_sql;
EXECUTE subscription_snapshot_stmt;
DEALLOCATE PREPARE subscription_snapshot_stmt;

SET @subscription_snapshot_sql = IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE() AND table_name = 'orders'
     AND column_name = 'merchant_plan_price_cents_snapshot') > 0,
  'SELECT 1',
  'ALTER TABLE `orders` ADD COLUMN `merchant_plan_price_cents_snapshot` BIGINT NULL'
);
PREPARE subscription_snapshot_stmt FROM @subscription_snapshot_sql;
EXECUTE subscription_snapshot_stmt;
DEALLOCATE PREPARE subscription_snapshot_stmt;

SET @subscription_snapshot_sql = IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE() AND table_name = 'orders'
     AND column_name = 'merchant_plan_duration_days_snapshot') > 0,
  'SELECT 1',
  'ALTER TABLE `orders` ADD COLUMN `merchant_plan_duration_days_snapshot` BIGINT NULL'
);
PREPARE subscription_snapshot_stmt FROM @subscription_snapshot_sql;
EXECUTE subscription_snapshot_stmt;
DEALLOCATE PREPARE subscription_snapshot_stmt;
