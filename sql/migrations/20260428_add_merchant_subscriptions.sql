ALTER TABLE merchants
  ADD COLUMN subscription_plan VARCHAR(40) NOT NULL DEFAULT 'none' AFTER status,
  ADD COLUMN subscription_status VARCHAR(20) NOT NULL DEFAULT 'inactive' AFTER subscription_plan,
  ADD COLUMN subscription_expired_at DATETIME NULL AFTER subscription_status,
  ADD COLUMN subscription_note VARCHAR(255) NOT NULL DEFAULT '' AFTER subscription_expired_at,
  ADD INDEX idx_merchants_subscription_status (subscription_status);

UPDATE merchants
SET subscription_plan = 'none',
    subscription_status = 'inactive'
WHERE subscription_plan IS NULL OR subscription_plan = '';
