const credentialPattern = /^[a-f\d]{64}$/i

const normalizedPayload = (payload) => ({
  store_id: Number(payload.store_id),
  customer_phone: String(payload.customer_phone || '').trim(),
  customer_note: String(payload.customer_note || '').trim(),
  share_code: String(payload.share_code || '').trim(),
  coupon_no: String(payload.coupon_no || '').trim(),
  payment_channel: payload.payment_channel || 'alipay',
  items: [...(payload.items || [])]
    .map(({ product_id, quantity }) => ({ product_id: Number(product_id), quantity: Number(quantity) }))
    .sort((a, b) => a.product_id - b.product_id)
})

const fingerprintFor = async (payload, cryptoImpl) => {
  const serialized = JSON.stringify(normalizedPayload(payload))
  if (!cryptoImpl?.subtle || typeof TextEncoder === 'undefined') return { value: serialized, persistent: false }
  const digest = await cryptoImpl.subtle.digest('SHA-256', new TextEncoder().encode(serialized))
  return {
    value: Array.from(new Uint8Array(digest), (value) => value.toString(16).padStart(2, '0')).join(''),
    persistent: true
  }
}

const secureCredential = (cryptoImpl) => {
  const bytes = new Uint8Array(32)
  if (!cryptoImpl?.getRandomValues) throw new Error('当前浏览器无法安全创建订单凭据，请更新浏览器后重试')
  cryptoImpl.getRandomValues(bytes)
  return Array.from(bytes, (value) => value.toString(16).padStart(2, '0')).join('')
}

const storageKey = (storeId) => `customer_checkout_attempt_${Number(storeId)}`

export const readCustomerOrderAttempt = (storeId, storage = globalThis.sessionStorage) => {
  if (!storeId || !storage) return null
  try {
    const saved = JSON.parse(storage.getItem(storageKey(storeId)) || 'null')
    if (Number(saved?.store_id) !== Number(storeId) ||
      !credentialPattern.test(saved?.request_key || '') ||
      !credentialPattern.test(saved?.access_token || '')) return null
    return saved
  } catch {
    return null
  }
}

export const getOrCreateCustomerOrderAttempt = async (payload, currentAttempt = null, {
  storage = globalThis.sessionStorage,
  cryptoImpl = globalThis.crypto
} = {}) => {
  const fingerprint = await fingerprintFor(payload, cryptoImpl)
  if (currentAttempt?.fingerprint === fingerprint.value) return currentAttempt

  const storeId = Number(payload.store_id)
  if (fingerprint.persistent && storage) {
    const saved = readCustomerOrderAttempt(storeId, storage)
    if (saved?.fingerprint === fingerprint.value) return saved
  }

  const attempt = {
    store_id: storeId,
    fingerprint: fingerprint.value,
    request_key: secureCredential(cryptoImpl),
    access_token: secureCredential(cryptoImpl)
  }
  if (fingerprint.persistent && storage) {
    try {
      storage.setItem(storageKey(storeId), JSON.stringify(attempt))
    } catch {
      // In-page retries still reuse the in-memory credentials.
    }
  }
  return attempt
}

export const clearCustomerOrderAttempt = (attempt, storage = globalThis.sessionStorage) => {
  if (!attempt || !storage) return
  try {
    const saved = JSON.parse(storage.getItem(storageKey(attempt.store_id)) || 'null')
    if (saved?.request_key === attempt.request_key) storage.removeItem(storageKey(attempt.store_id))
  } catch {
    // Nothing to clear when session storage is unavailable.
  }
}
