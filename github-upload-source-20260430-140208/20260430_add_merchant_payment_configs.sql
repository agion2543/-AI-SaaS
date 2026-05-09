CREATE TABLE IF NOT EXISTS merchant_payment_configs (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  deleted_at DATETIME(3) NULL,
  merchant_id BIGINT UNSIGNED NOT NULL,
  channel VARCHAR(30) NOT NULL DEFAULT 'alipay',
  mode VARCHAR(30) NOT NULL DEFAULT 'direct',
  account_name VARCHAR(120) NOT NULL DEFAULT '',
  account_no VARCHAR(120) NOT NULL DEFAULT '',
  app_id VARCHAR(120) NOT NULL DEFAULT '',
  status VARCHAR(20) NOT NULL DEFAULT 'disabled',
  audit_status VARCHAR(20) NOT NULL DEFAULT 'pending',
  audit_remark VARCHAR(255) NOT NULL DEFAULT '',
  contact_phone VARCHAR(30) NOT NULL DEFAULT '',
  remark VARCHAR(255) NOT NULL DEFAULT '',
  UNIQUE KEY uk_merchant_payment_configs_merchant_id (merchant_id),
  KEY idx_merchant_payment_configs_deleted_at (deleted_at),
  KEY idx_merchant_payment_configs_status (status),
  KEY idx_merchant_payment_configs_audit_status (audit_status),
  CONSTRAINT fk_merchant_payment_configs_merchant
    FOREIGN KEY (merchant_id) REFERENCES merchants(id)
);
