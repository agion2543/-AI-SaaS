ALTER TABLE orders
  ADD COLUMN refunded_amount BIGINT NOT NULL DEFAULT 0 AFTER discount_amount,
  ADD COLUMN refund_status VARCHAR(20) NOT NULL DEFAULT 'none' AFTER refunded_amount,
  ADD INDEX idx_orders_refund_status (refund_status);

CREATE TABLE IF NOT EXISTS refund_records (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  order_id BIGINT UNSIGNED NOT NULL,
  merchant_id BIGINT UNSIGNED NULL,
  store_id BIGINT UNSIGNED NULL,
  refund_no VARCHAR(60) NOT NULL,
  transaction_no VARCHAR(80) DEFAULT '',
  amount BIGINT NOT NULL,
  reason VARCHAR(255) DEFAULT '',
  operator_role VARCHAR(30) DEFAULT '',
  operator_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  status VARCHAR(20) NOT NULL DEFAULT 'success',
  raw_payload TEXT,
  PRIMARY KEY (id),
  UNIQUE KEY idx_refund_records_refund_no (refund_no),
  KEY idx_refund_records_order_id (order_id),
  KEY idx_refund_records_merchant_id (merchant_id),
  KEY idx_refund_records_store_id (store_id),
  KEY idx_refund_records_transaction_no (transaction_no),
  KEY idx_refund_records_status (status),
  CONSTRAINT fk_refund_records_order FOREIGN KEY (order_id) REFERENCES orders(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
