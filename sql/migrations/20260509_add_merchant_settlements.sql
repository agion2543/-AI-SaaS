CREATE TABLE IF NOT EXISTS merchant_settlements (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  deleted_at DATETIME(3) NULL,
  merchant_id BIGINT UNSIGNED NOT NULL,
  settlement_period_start DATETIME(3) NULL,
  settlement_period_end DATETIME(3) NULL,
  order_count BIGINT DEFAULT 0,
  total_amount_cents BIGINT DEFAULT 0,
  refund_amount_cents BIGINT DEFAULT 0,
  net_amount_cents BIGINT DEFAULT 0,
  status VARCHAR(20) DEFAULT 'pending',
  paid_at DATETIME(3) NULL,
  remark VARCHAR(255) DEFAULT '',
  PRIMARY KEY (id),
  INDEX idx_merchant_settlements_deleted_at (deleted_at),
  INDEX idx_merchant_settlements_merchant_id (merchant_id),
  INDEX idx_merchant_settlements_status (status),
  CONSTRAINT fk_merchant_settlements_merchant
    FOREIGN KEY (merchant_id) REFERENCES merchants(id)
    ON DELETE RESTRICT ON UPDATE CASCADE
);

ALTER TABLE orders
  ADD COLUMN settlement_id BIGINT UNSIGNED NULL,
  ADD INDEX idx_orders_settlement_id (settlement_id);

ALTER TABLE orders
  ADD CONSTRAINT fk_orders_settlement
    FOREIGN KEY (settlement_id) REFERENCES merchant_settlements(id)
    ON DELETE SET NULL ON UPDATE CASCADE;
