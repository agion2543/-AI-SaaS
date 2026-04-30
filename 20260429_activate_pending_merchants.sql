UPDATE merchants
SET status = 'active', updated_at = NOW(3)
WHERE status = 'pending';
