export function buildMerchantStorePayload(store, overrides = {}) {
  const source = { ...store, ...overrides }
  return {
    name: String(source.name ?? '').trim(),
    address: String(source.address ?? '').trim(),
    contact_phone: String(source.contact_phone ?? '').trim(),
    is_open: source.is_open !== false,
    business_hours: String(source.business_hours ?? '').trim()
  }
}

export function isValidMerchantStorePhone(phone) {
  return /^1\d{10}$/.test(String(phone ?? '').trim())
}
