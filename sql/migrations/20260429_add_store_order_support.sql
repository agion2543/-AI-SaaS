START TRANSACTION;

ALTER TABLE orders
  MODIFY COLUMN user_id BIGINT UNSIGNED NULL,
  MODIFY COLUMN package_id BIGINT UNSIGNED NULL;

SET @order_type_exists := (
  SELECT COUNT(1) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'orders' AND COLUMN_NAME = 'order_type'
);
SET @order_type_sql := IF(
  @order_type_exists = 0,
  'ALTER TABLE orders ADD COLUMN order_type VARCHAR(40) NOT NULL DEFAULT ''user_membership'' AFTER order_no',
  'SELECT 1'
);
PREPARE order_type_stmt FROM @order_type_sql;
EXECUTE order_type_stmt;
DEALLOCATE PREPARE order_type_stmt;

SET @merchant_id_exists := (
  SELECT COUNT(1) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'orders' AND COLUMN_NAME = 'merchant_id'
);
SET @merchant_id_sql := IF(
  @merchant_id_exists = 0,
  'ALTER TABLE orders ADD COLUMN merchant_id BIGINT UNSIGNED NULL AFTER user_id',
  'SELECT 1'
);
PREPARE merchant_id_stmt FROM @merchant_id_sql;
EXECUTE merchant_id_stmt;
DEALLOCATE PREPARE merchant_id_stmt;

SET @store_id_exists := (
  SELECT COUNT(1) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'orders' AND COLUMN_NAME = 'store_id'
);
SET @store_id_sql := IF(
  @store_id_exists = 0,
  'ALTER TABLE orders ADD COLUMN store_id BIGINT UNSIGNED NULL AFTER merchant_id',
  'SELECT 1'
);
PREPARE store_id_stmt FROM @store_id_sql;
EXECUTE store_id_stmt;
DEALLOCATE PREPARE store_id_stmt;

SET @customer_phone_exists := (
  SELECT COUNT(1) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'orders' AND COLUMN_NAME = 'customer_phone'
);
SET @customer_phone_sql := IF(
  @customer_phone_exists = 0,
  'ALTER TABLE orders ADD COLUMN customer_phone VARCHAR(30) NOT NULL DEFAULT '''' AFTER store_id',
  'SELECT 1'
);
PREPARE customer_phone_stmt FROM @customer_phone_sql;
EXECUTE customer_phone_stmt;
DEALLOCATE PREPARE customer_phone_stmt;

SET @total_amount_exists := (
  SELECT COUNT(1) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'orders' AND COLUMN_NAME = 'total_amount'
);
SET @total_amount_sql := IF(
  @total_amount_exists = 0,
  'ALTER TABLE orders ADD COLUMN total_amount BIGINT NOT NULL DEFAULT 0 AFTER amount',
  'SELECT 1'
);
PREPARE total_amount_stmt FROM @total_amount_sql;
EXECUTE total_amount_stmt;
DEALLOCATE PREPARE total_amount_stmt;

SET @items_exists := (
  SELECT COUNT(1) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'orders' AND COLUMN_NAME = 'items'
);
SET @items_sql := IF(
  @items_exists = 0,
  'ALTER TABLE orders ADD COLUMN items JSON NULL AFTER total_amount',
  'SELECT 1'
);
PREPARE items_stmt FROM @items_sql;
EXECUTE items_stmt;
DEALLOCATE PREPARE items_stmt;

SET @store_idx_exists := (
  SELECT COUNT(1) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'orders' AND INDEX_NAME = 'idx_orders_store_id'
);
SET @store_idx_sql := IF(
  @store_idx_exists = 0,
  'CREATE INDEX idx_orders_store_id ON orders (store_id)',
  'SELECT 1'
);
PREPARE store_idx_stmt FROM @store_idx_sql;
EXECUTE store_idx_stmt;
DEALLOCATE PREPARE store_idx_stmt;

SET @status_idx_exists := (
  SELECT COUNT(1) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'orders' AND INDEX_NAME = 'idx_orders_status'
);
SET @status_idx_sql := IF(
  @status_idx_exists = 0,
  'CREATE INDEX idx_orders_status ON orders (status)',
  'SELECT 1'
);
PREPARE status_idx_stmt FROM @status_idx_sql;
EXECUTE status_idx_stmt;
DEALLOCATE PREPARE status_idx_stmt;

COMMIT;
