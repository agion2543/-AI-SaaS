const pollableStatuses = new Set([
  'pending',
  'submitted',
  'payment_confirming',
  'paid',
  'received',
  'accepted',
  'preparing',
  'ready'
])

const fastPollAttemptLimit = 40
const fastPollIntervalMs = 5000
const slowPollIntervalMs = 15000

export function isCustomerOrderStatusPollable(status) {
  return pollableStatuses.has(status)
}

export function customerOrderPollIntervalMs(attempts) {
  return attempts >= fastPollAttemptLimit ? slowPollIntervalMs : fastPollIntervalMs
}
