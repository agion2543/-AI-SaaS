ALTER TABLE orders
  ADD COLUMN customer_rating INT NOT NULL DEFAULT 0 AFTER merchant_note,
  ADD COLUMN customer_review VARCHAR(1000) NULL AFTER customer_rating,
  ADD COLUMN reviewed_at DATETIME NULL AFTER customer_review;

