SET @store_is_open_exists := (
  SELECT COUNT(1) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'stores' AND COLUMN_NAME = 'is_open'
);
SET @store_is_open_sql := IF(
  @store_is_open_exists = 0,
  'ALTER TABLE stores ADD COLUMN is_open TINYINT(1) NOT NULL DEFAULT 1 AFTER status',
  'SELECT 1'
);
PREPARE store_is_open_stmt FROM @store_is_open_sql;
EXECUTE store_is_open_stmt;
DEALLOCATE PREPARE store_is_open_stmt;

SET @business_hours_exists := (
  SELECT COUNT(1) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'stores' AND COLUMN_NAME = 'business_hours'
);
SET @business_hours_sql := IF(
  @business_hours_exists = 0,
  'ALTER TABLE stores ADD COLUMN business_hours VARCHAR(120) NOT NULL DEFAULT '''' AFTER is_open',
  'SELECT 1'
);
PREPARE business_hours_stmt FROM @business_hours_sql;
EXECUTE business_hours_stmt;
DEALLOCATE PREPARE business_hours_stmt;

SET @pause_reason_exists := (
  SELECT COUNT(1) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'stores' AND COLUMN_NAME = 'pause_reason'
);
SET @pause_reason_sql := IF(
  @pause_reason_exists = 0,
  'ALTER TABLE stores ADD COLUMN pause_reason VARCHAR(255) NOT NULL DEFAULT '''' AFTER business_hours',
  'SELECT 1'
);
PREPARE pause_reason_stmt FROM @pause_reason_sql;
EXECUTE pause_reason_stmt;
DEALLOCATE PREPARE pause_reason_stmt;

SET @product_category_exists := (
  SELECT COUNT(1) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'store_products' AND COLUMN_NAME = 'category'
);
SET @product_category_sql := IF(
  @product_category_exists = 0,
  'ALTER TABLE store_products ADD COLUMN category VARCHAR(80) NOT NULL DEFAULT ''默认分类'' AFTER image_url',
  'SELECT 1'
);
PREPARE product_category_stmt FROM @product_category_sql;
EXECUTE product_category_stmt;
DEALLOCATE PREPARE product_category_stmt;
