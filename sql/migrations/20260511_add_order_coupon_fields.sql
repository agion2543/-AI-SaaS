ALTER TABLE orders
  ADD COLUMN referral_coupon_id BIGINT UNSIGNED NULL,
  ADD COLUMN coupon_no VARCHAR(60) NULL,
  ADD INDEX idx_orders_referral_coupon_id (referral_coupon_id),
  ADD INDEX idx_orders_coupon_no (coupon_no);
