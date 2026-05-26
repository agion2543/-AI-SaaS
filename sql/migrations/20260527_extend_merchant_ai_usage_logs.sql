ALTER TABLE merchant_ai_usage_logs
  ADD COLUMN success TINYINT(1) NOT NULL DEFAULT 1 AFTER fallback,
  ADD COLUMN error VARCHAR(500) NOT NULL DEFAULT '' AFTER success,
  ADD COLUMN duration_ms INT NOT NULL DEFAULT 0 AFTER error,
  ADD INDEX idx_merchant_ai_usage_logs_success (success),
  ADD INDEX idx_merchant_ai_usage_logs_scenario (scenario);
