CREATE TABLE IF NOT EXISTS merchant_ai_usage_logs (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  merchant_id BIGINT UNSIGNED NOT NULL,
  user_id BIGINT UNSIGNED NULL,
  scenario VARCHAR(120) NOT NULL DEFAULT '',
  model VARCHAR(120) NOT NULL DEFAULT '',
  provider VARCHAR(80) NOT NULL DEFAULT '',
  fallback TINYINT(1) NOT NULL DEFAULT 0,
  used_at DATETIME(3) NOT NULL,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  INDEX idx_merchant_ai_usage_logs_merchant_id (merchant_id),
  INDEX idx_merchant_ai_usage_logs_user_id (user_id),
  INDEX idx_merchant_ai_usage_logs_used_at (used_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
