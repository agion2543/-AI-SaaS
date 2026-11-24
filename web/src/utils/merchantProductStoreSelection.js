export function resolveMerchantProductStoreIndex(stores, requestedStoreId) {
  if (!Array.isArray(stores) || stores.length === 0) return 0

  const requestedId = Array.isArray(requestedStoreId)
    ? requestedStoreId.find(value => value !== null && value !== undefined && String(value).trim() !== '')
    : requestedStoreId
  if (requestedId === null || requestedId === undefined || String(requestedId).trim() === '') return 0

  const normalizedId = String(requestedId).trim()
  const index = stores.findIndex(store => String(store?.id ?? '') === normalizedId)
  return index < 0 ? 0 : index
}
