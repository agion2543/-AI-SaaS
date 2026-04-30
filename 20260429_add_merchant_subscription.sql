CREATE TABLE IF NOT EXISTS merchant_plans (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  name VARCHAR(80) NOT NULL,
  price_cents BIGINT NOT NULL,
  duration_days BIGINT NOT NULL,
  sort BIGINT DEFAULT 0,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  INDEX idx_merchant_plans_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT INTO merchant_plans (name, price_cents, duration_days, sort, created_at, updated_at)
SELECT '月付', 19900, 30, 1, NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM merchant_plans WHERE name = '月付');

INSERT INTO merchant_plans (name, price_cents, duration_days, sort, created_at, updated_at)
SELECT '年付', 199900, 365, 2, NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM merchant_plans WHERE name = '年付');

ALTER TABLE merchants
  ADD COLUMN subscription_plan_id BIGINT UNSIGNED NULL,
  ADD COLUMN subscription_expire_at DATETIME(3) NULL,
  ADD INDEX idx_merchants_subscription_plan_id (subscription_plan_id);

ALTER TABLE orders
  ADD COLUMN order_type VARCHAR(40) NOT NULL DEFAULT 'user_membership',
  ADD COLUMN merchant_id BIGINT UNSIGNED NULL,
  ADD COLUMN merchant_plan_id BIGINT UNSIGNED NULL,
  ADD INDEX idx_orders_order_type (order_type),
  ADD INDEX idx_orders_merchant_id (merchant_id),
  ADD INDEX idx_orders_merchant_plan_id (merchant_plan_id);
