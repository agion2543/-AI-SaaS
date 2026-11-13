-- Record explicit platform subscription pauses without changing the existing expiry.
SET @subscription_pause_sql = IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE() AND table_name = 'merchants'
     AND column_name = 'subscription_pause_reason') > 0,
  'SELECT 1',
  'ALTER TABLE `merchants` ADD COLUMN `subscription_pause_reason` VARCHAR(255) NULL'
);
PREPARE subscription_pause_stmt FROM @subscription_pause_sql;
EXECUTE subscription_pause_stmt;
DEALLOCATE PREPARE subscription_pause_stmt;

SET @subscription_pause_sql = IF(
  (SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = DATABASE() AND table_name = 'merchants'
     AND column_name = 'subscription_paused_at') > 0,
  'SELECT 1',
  'ALTER TABLE `merchants` ADD COLUMN `subscription_paused_at` DATETIME NULL'
);
PREPARE subscription_pause_stmt FROM @subscription_pause_sql;
EXECUTE subscription_pause_stmt;
DEALLOCATE PREPARE subscription_pause_stmt;
