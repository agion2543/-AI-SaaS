const ORDER_STATUSES = new Set([
  'pending',
  'submitted',
  'payment_confirming',
  'preparing',
  'received',
  'accepted',
  'completed',
  'closed',
  'failed'
])

const QUICK_MODES = new Set([
  'all',
  'pay_issue',
  'payment_confirming',
  'overdue',
  'received',
  'accepted',
  'refund',
  'note'
])

const firstValue = (value) => (Array.isArray(value) ? value[0] : value)

export const normalizeMerchantOrderRouteState = (query = {}) => {
  const rawStatus = firstValue(query.status)
  const rawQuick = firstValue(query.quick)
  const status = typeof rawStatus === 'string' && ORDER_STATUSES.has(rawStatus) ? rawStatus : ''
  const quick = status
    ? 'all'
    : typeof rawQuick === 'string' && QUICK_MODES.has(rawQuick)
      ? rawQuick
      : 'all'

  return { status, quick }
}

export const shouldReloadOrdersForQuickModeChange = ({ statusWasFiltered, currentPage }) =>
  Boolean(statusWasFiltered) || Number(currentPage) > 1

export const mergeMerchantOrderRouteState = (query = {}, state = {}) => {
  const next = { ...query }
  const normalized = normalizeMerchantOrderRouteState(state)

  if (normalized.status) next.status = normalized.status
  else delete next.status

  if (!normalized.status && normalized.quick !== 'all') next.quick = normalized.quick
  else delete next.quick

  return next
}
