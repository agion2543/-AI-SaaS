SET @payment_alipay_qr_exists := (
  SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'merchant_payment_configs' AND COLUMN_NAME = 'alipay_qr_code'
);
SET @payment_alipay_qr_sql := IF(
  @payment_alipay_qr_exists = 0,
  'ALTER TABLE merchant_payment_configs ADD COLUMN alipay_qr_code VARCHAR(1000) NOT NULL DEFAULT '''' AFTER account_no',
  'SELECT 1'
);
PREPARE payment_alipay_qr_stmt FROM @payment_alipay_qr_sql;
EXECUTE payment_alipay_qr_stmt;
DEALLOCATE PREPARE payment_alipay_qr_stmt;

SET @payment_wechat_qr_exists := (
  SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'merchant_payment_configs' AND COLUMN_NAME = 'wechat_qr_code'
);
SET @payment_wechat_qr_sql := IF(
  @payment_wechat_qr_exists = 0,
  'ALTER TABLE merchant_payment_configs ADD COLUMN wechat_qr_code VARCHAR(1000) NOT NULL DEFAULT '''' AFTER alipay_qr_code',
  'SELECT 1'
);
PREPARE payment_wechat_qr_stmt FROM @payment_wechat_qr_sql;
EXECUTE payment_wechat_qr_stmt;
DEALLOCATE PREPARE payment_wechat_qr_stmt;
