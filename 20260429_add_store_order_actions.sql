SET @merchant_note_exists := (
  SELECT COUNT(1) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'orders' AND COLUMN_NAME = 'merchant_note'
);
SET @merchant_note_sql := IF(
  @merchant_note_exists = 0,
  'ALTER TABLE orders ADD COLUMN merchant_note VARCHAR(500) NOT NULL DEFAULT '''' AFTER items',
  'SELECT 1'
);
PREPARE merchant_note_stmt FROM @merchant_note_sql;
EXECUTE merchant_note_stmt;
DEALLOCATE PREPARE merchant_note_stmt;
