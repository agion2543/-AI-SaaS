SET @store_order_mode_exists := (
  SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'stores' AND COLUMN_NAME = 'order_mode'
);
SET @store_order_mode_sql := IF(
  @store_order_mode_exists = 0,
  'ALTER TABLE stores ADD COLUMN order_mode VARCHAR(30) NOT NULL DEFAULT ''pay_first'' AFTER pause_reason',
  'SELECT 1'
);
PREPARE store_order_mode_stmt FROM @store_order_mode_sql;
EXECUTE store_order_mode_stmt;
DEALLOCATE PREPARE store_order_mode_stmt;

SET @store_auto_accept_exists := (
  SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'stores' AND COLUMN_NAME = 'auto_accept'
);
SET @store_auto_accept_sql := IF(
  @store_auto_accept_exists = 0,
  'ALTER TABLE stores ADD COLUMN auto_accept TINYINT(1) NOT NULL DEFAULT 0 AFTER order_mode',
  'SELECT 1'
);
PREPARE store_auto_accept_stmt FROM @store_auto_accept_sql;
EXECUTE store_auto_accept_stmt;
DEALLOCATE PREPARE store_auto_accept_stmt;
