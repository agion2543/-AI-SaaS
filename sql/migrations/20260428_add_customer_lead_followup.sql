ALTER TABLE customer_leads
  ADD COLUMN follow_up_note VARCHAR(500) NOT NULL DEFAULT '' AFTER message;

UPDATE customer_leads
SET status = 'new'
WHERE status IS NULL OR status = '';
