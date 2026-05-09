CREATE TABLE IF NOT EXISTS customer_leads (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  merchant_id BIGINT UNSIGNED NOT NULL,
  store_id BIGINT UNSIGNED NOT NULL,
  customer_name VARCHAR(60) NOT NULL DEFAULT '',
  customer_phone VARCHAR(30) NOT NULL,
  message VARCHAR(500) NOT NULL DEFAULT '',
  source VARCHAR(40) NOT NULL DEFAULT 'store_qr',
  status VARCHAR(20) NOT NULL DEFAULT 'new',
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  INDEX idx_customer_leads_merchant_id (merchant_id),
  INDEX idx_customer_leads_store_id (store_id),
  INDEX idx_customer_leads_customer_phone (customer_phone),
  INDEX idx_customer_leads_status (status),
  CONSTRAINT fk_customer_leads_merchant FOREIGN KEY (merchant_id) REFERENCES merchants(id),
  CONSTRAINT fk_customer_leads_store FOREIGN KEY (store_id) REFERENCES stores(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
