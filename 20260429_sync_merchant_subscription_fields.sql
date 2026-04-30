UPDATE merchants m
LEFT JOIN merchant_plans mp
  ON (m.subscription_plan = 'month' AND mp.name = '月付')
  OR (m.subscription_plan = 'year' AND mp.name = '年付')
SET
  m.subscription_expire_at = COALESCE(m.subscription_expire_at, m.subscription_expired_at),
  m.subscription_expired_at = COALESCE(m.subscription_expired_at, m.subscription_expire_at),
  m.subscription_plan_id = COALESCE(m.subscription_plan_id, mp.id),
  m.subscription_status = CASE
    WHEN COALESCE(m.subscription_expire_at, m.subscription_expired_at) > NOW() THEN 'active'
    ELSE m.subscription_status
  END
WHERE m.subscription_expire_at IS NULL
   OR m.subscription_plan_id IS NULL
   OR m.subscription_expired_at IS NULL;
