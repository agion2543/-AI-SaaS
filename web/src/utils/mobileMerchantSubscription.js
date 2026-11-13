const validMerchant = (merchant) => merchant && typeof merchant === 'object' && !Array.isArray(merchant)

export const merchantFromSubscriptionPayload = (payload) =>
  validMerchant(payload?.merchant) ? payload.merchant : null

export const mobileMerchantSubscriptionState = (payload, now = Date.now()) => {
  const merchant = merchantFromSubscriptionPayload(payload)
  if (!merchant) return 'unknown'
  if (merchant.status === 'suspended' || merchant.subscription_status === 'paused') return 'paused'
  if (merchant.subscription_status !== 'active') return 'unknown'

  const expiry = merchant.subscription_expire_at || merchant.subscription_expired_at
  if (!expiry) return 'unknown'
  const expiresAt = new Date(expiry).getTime()
  if (!Number.isFinite(expiresAt)) return 'unknown'
  return expiresAt > now ? 'active' : 'expired'
}

export const mobileSubscriptionActions = (state) => ({
  canReadBusiness: state === 'active' || state === 'expired',
  canWriteBusiness: state === 'active',
  canRenew: state === 'active' || state === 'expired',
  canRecheck: state === 'paused' || state === 'unknown' || state === 'error',
  showSupportPath: state === 'paused'
})

export const cachedMobileMerchantSubscriptionState = (now = Date.now()) => {
  try {
    const merchant = JSON.parse(localStorage.getItem('merchant_current') || 'null')
    return mobileMerchantSubscriptionState({ merchant }, now)
  } catch {
    return 'unknown'
  }
}
