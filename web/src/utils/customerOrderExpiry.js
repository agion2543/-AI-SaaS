export function isUnpaidCheckoutOrder(order) {
  const amount = Number(order?.total_amount ?? order?.amount ?? 0)
  return amount > 0 && !order?.paid_at && ['pending', 'failed'].includes(order?.status)
}

export function checkoutExpiryTimestamp(order) {
  if (!isUnpaidCheckoutOrder(order)) return null
  const timestamp = Date.parse(order?.expire_time ?? '')
  return Number.isFinite(timestamp) ? timestamp : null
}

export function isCheckoutDeadlineReached(order, now = Date.now()) {
  const expiresAt = checkoutExpiryTimestamp(order)
  return expiresAt !== null && expiresAt <= Number(now)
}

export function remainingCheckoutTime(order, now = Date.now()) {
  const expiresAt = checkoutExpiryTimestamp(order)
  if (expiresAt === null) return null
  return Math.max(0, expiresAt - Number(now))
}
