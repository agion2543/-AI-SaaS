ALTER TABLE merchant_ai_usage_logs
  ADD COLUMN prompt_tokens INT NOT NULL DEFAULT 0 AFTER duration_ms,
  ADD COLUMN completion_tokens INT NOT NULL DEFAULT 0 AFTER prompt_tokens,
  ADD COLUMN total_tokens INT NOT NULL DEFAULT 0 AFTER completion_tokens,
  ADD COLUMN estimated_cost_cents BIGINT NOT NULL DEFAULT 0 AFTER total_tokens,
  ADD INDEX idx_merchant_ai_usage_logs_cost (estimated_cost_cents);
