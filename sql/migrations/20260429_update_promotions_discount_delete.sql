ALTER TABLE promotions
  ADD COLUMN type VARCHAR(20) NOT NULL DEFAULT 'amount' AFTER description,
  ADD COLUMN discount_rate INT NOT NULL DEFAULT 0 AFTER discount;

UPDATE promotions
SET type = 'amount'
WHERE type IS NULL OR type = '';
