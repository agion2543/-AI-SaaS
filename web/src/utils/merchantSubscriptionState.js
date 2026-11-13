export const merchantSubscriptionExpiry = (merchant = {}) =>
  merchant.subscription_expire_at || merchant.subscription_expired_at || null

export const isMerchantSubscriptionActive = (merchant = {}, now = Date.now()) => {
  if (merchant.subscription_status !== 'active') return false
  const expiry = merchantSubscriptionExpiry(merchant)
  if (!expiry) return false
  const expiresAt = new Date(expiry).getTime()
  return Number.isFinite(expiresAt) && expiresAt > now
}

export const merchantSubscriptionAccessState = (merchant = {}, now = Date.now()) => {
  if (merchant.subscription_status === 'paused') return 'paused'
  if (isMerchantSubscriptionActive(merchant, now)) return 'active'
  if (merchant.subscription_status === 'active') {
    const expiry = merchantSubscriptionExpiry(merchant)
    const expiresAt = expiry ? new Date(expiry).getTime() : NaN
    if (Number.isFinite(expiresAt) && expiresAt <= now) return 'expired'
  }
  return 'unavailable'
}

export const merchantSubscriptionDisplayState = (merchant = {}, now = Date.now()) => {
  const status = String(merchant.subscription_status || '')
  const expiry = merchantSubscriptionExpiry(merchant)
  const expiresAt = expiry ? new Date(expiry).getTime() : 0

  if (status === 'paused') {
    if (Number.isFinite(expiresAt) && expiresAt > now) {
      return {
        title: '已暂停',
        hint: merchant.subscription_pause_reason || '订阅仍在有效期内，但当前已暂停使用'
      }
    }
    return { title: '已过期', hint: '暂停期间订阅已到期，续费后可以恢复' }
  }
  if (expiry && (!Number.isFinite(expiresAt) || expiresAt <= now)) {
    return { title: '已过期', hint: '续费后可恢复商家工作台服务' }
  }
  if (status === 'active') {
    if (!expiry) return { title: '状态异常', hint: '订阅缺少到期时间，当前不可使用' }
    const daysRemaining = Math.ceil((expiresAt - now) / 86400000)
    if (daysRemaining <= 7) return { title: '即将到期', hint: '建议提前续费，避免影响使用' }
    return { title: '服务中', hint: '当前订阅可正常使用' }
  }
  if (status === 'pending') return { title: '待确认', hint: '付款确认后才会开通商家工作台' }
  return { title: '未开通', hint: '选择服务版本并完成付款后开通' }
}
