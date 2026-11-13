export const newRefundIdempotencyKey = () => {
  if (globalThis.crypto?.randomUUID) return globalThis.crypto.randomUUID()
  const bytes = new Uint8Array(16)
  if (globalThis.crypto?.getRandomValues) globalThis.crypto.getRandomValues(bytes)
  else for (let index = 0; index < bytes.length; index += 1) bytes[index] = Math.floor(Math.random() * 256)
  return `rf-${Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('')}`
}

const storageKey = (orderID) => `cue:refund-idempotency:${orderID}`

const getSessionStorage = () => {
  try {
    return globalThis.sessionStorage
  } catch {
    return null
  }
}

export const getOrCreateRefundIdempotency = (orderID, payload, currentKey = '', currentFingerprint = '') => {
  const fingerprint = JSON.stringify(payload)
  const storage = getSessionStorage()
  const key = storageKey(orderID)
  let stored = null
  if (storage) {
    try {
      stored = JSON.parse(storage.getItem(key) || 'null')
    } catch {
      stored = null
    }
  }

  if (currentKey) {
    const knownFingerprint = currentFingerprint || (stored?.key === currentKey ? stored.fingerprint : '')
    if (knownFingerprint && knownFingerprint !== fingerprint) {
      return { key: currentKey, fingerprint: knownFingerprint, conflict: true }
    }
    if (storage) {
      try {
        storage.setItem(key, JSON.stringify({ key: currentKey, fingerprint }))
      } catch {
        // Keep the in-memory key when browser storage is unavailable.
      }
    }
    return { key: currentKey, fingerprint, conflict: false }
  }

  if (stored?.key) {
    if (stored.fingerprint !== fingerprint) {
      return { key: stored.key, fingerprint: stored.fingerprint || '', conflict: true }
    }
    return { key: stored.key, fingerprint, conflict: false }
  }

  const nextKey = newRefundIdempotencyKey()
  if (storage) {
    try {
      storage.setItem(key, JSON.stringify({ key: nextKey, fingerprint }))
    } catch {
      // The request remains protected for this component lifetime.
    }
  }
  return { key: nextKey, fingerprint, conflict: false }
}

export const clearRefundIdempotency = (orderID) => {
  const storage = getSessionStorage()
  if (!storage) return
  try {
    storage.removeItem(storageKey(orderID))
  } catch {
    // Best-effort cleanup; backend idempotency remains authoritative.
  }
}
