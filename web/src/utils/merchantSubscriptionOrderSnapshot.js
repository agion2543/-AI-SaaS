export const hasValidMerchantSubscriptionSnapshot = (order = {}) => {
  const name = String(order.merchant_plan_name_snapshot || '').trim()
  const price = Number(order.merchant_plan_price_cents_snapshot)
  const duration = Number(order.merchant_plan_duration_days_snapshot)
  return Boolean(
    name &&
    Number.isFinite(price) &&
    price >= 0 &&
    Number.isInteger(duration) &&
    duration > 0 &&
    Number(order.amount) === price &&
    Number(order.total_amount) === price
  )
}

export const isLegacyMerchantSubscriptionOrderWithoutSnapshot = (order = {}) => {
  const values = [
    order.merchant_plan_name_snapshot,
    order.merchant_plan_price_cents_snapshot,
    order.merchant_plan_duration_days_snapshot
  ]
  return values.every((value) => value === null || value === undefined || value === '')
}

export const merchantSubscriptionSnapshotLabel = (order = {}) => {
  if (!hasValidMerchantSubscriptionSnapshot(order)) return '订单快照缺失或无效（需人工复核）'
  return `${String(order.merchant_plan_name_snapshot).trim()} · ${Number(order.merchant_plan_duration_days_snapshot)} 天`
}

export const merchantSubscriptionSnapshotPrice = (order = {}) =>
  hasValidMerchantSubscriptionSnapshot(order) ? Number(order.merchant_plan_price_cents_snapshot) : null
