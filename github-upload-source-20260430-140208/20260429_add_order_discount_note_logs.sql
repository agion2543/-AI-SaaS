SET @discount_amount_exists := (
  SELECT COUNT(1) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'orders' AND COLUMN_NAME = 'discount_amount'
);
SET @discount_amount_sql := IF(
  @discount_amount_exists = 0,
  'ALTER TABLE orders ADD COLUMN discount_amount BIGINT NOT NULL DEFAULT 0 AFTER total_amount',
  'SELECT 1'
);
PREPARE discount_amount_stmt FROM @discount_amount_sql;
EXECUTE discount_amount_stmt;
DEALLOCATE PREPARE discount_amount_stmt;

SET @promotion_id_exists := (
  SELECT COUNT(1) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'orders' AND COLUMN_NAME = 'promotion_id'
);
SET @promotion_id_sql := IF(
  @promotion_id_exists = 0,
  'ALTER TABLE orders ADD COLUMN promotion_id BIGINT UNSIGNED NULL AFTER discount_amount, ADD INDEX idx_orders_promotion_id (promotion_id)',
  'SELECT 1'
);
PREPARE promotion_id_stmt FROM @promotion_id_sql;
EXECUTE promotion_id_stmt;
DEALLOCATE PREPARE promotion_id_stmt;

SET @customer_note_exists := (
  SELECT COUNT(1) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'orders' AND COLUMN_NAME = 'customer_note'
);
SET @customer_note_sql := IF(
  @customer_note_exists = 0,
  'ALTER TABLE orders ADD COLUMN customer_note VARCHAR(500) NOT NULL DEFAULT '''' AFTER items',
  'SELECT 1'
);
PREPARE customer_note_stmt FROM @customer_note_sql;
EXECUTE customer_note_stmt;
DEALLOCATE PREPARE customer_note_stmt;

SET @operation_logs_exists := (
  SELECT COUNT(1) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'orders' AND COLUMN_NAME = 'operation_logs'
);
SET @operation_logs_sql := IF(
  @operation_logs_exists = 0,
  'ALTER TABLE orders ADD COLUMN operation_logs TEXT NULL AFTER merchant_note',
  'SELECT 1'
);
PREPARE operation_logs_stmt FROM @operation_logs_sql;
EXECUTE operation_logs_stmt;
DEALLOCATE PREPARE operation_logs_stmt;
