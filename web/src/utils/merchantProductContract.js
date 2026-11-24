const DEFAULT_CATEGORY = '默认分类'
const hasOwn = (value, key) => Object.prototype.hasOwnProperty.call(value ?? {}, key)

function centsToYuanInput(value) {
  const cents = Number(value)
  return Number.isFinite(cents) ? (cents / 100).toFixed(2) : ''
}

export function isValidMerchantProductPrice(value) {
  const text = String(value ?? '').trim()
  if (!text) return false
  const yuan = Number(text)
  return Number.isFinite(yuan) && yuan >= 0 && Number.isSafeInteger(Math.round(yuan * 100))
}

export function createMerchantProductForm(product = null) {
  const isExisting = product !== null && typeof product === 'object'
  const tiers = isExisting && Array.isArray(product.price_tiers)
    ? product.price_tiers.map((tier) => ({
        min_quantity: Number(tier.min_quantity),
        price: Number(tier.price)
      }))
    : (isExisting ? undefined : [])

  return {
    name: String(product?.name ?? ''),
    price: isExisting ? centsToYuanInput(product.price) : '',
    description: String(product?.description ?? ''),
    image_url: String(product?.image_url ?? ''),
    imageFiles: [],
    category: String(product?.category ?? DEFAULT_CATEGORY) || DEFAULT_CATEGORY,
    status: product?.status === 'inactive' ? 'inactive' : 'active',
    sort: Number.isFinite(Number(product?.sort)) ? Number(product.sort) : 100,
    sales_unit: isExisting
      ? (hasOwn(product, 'sales_unit') ? String(product.sales_unit ?? '') : undefined)
      : '',
    specification: isExisting
      ? (hasOwn(product, 'specification') ? String(product.specification ?? '') : undefined)
      : '',
    minimum_order_quantity: isExisting
      ? (hasOwn(product, 'minimum_order_quantity') && product.minimum_order_quantity != null
          ? Number(product.minimum_order_quantity)
          : undefined)
      : 1,
    price_tiers: tiers
  }
}

export function buildMerchantProductPayload(form, storeId, overrides = {}) {
  const source = { ...form, ...overrides }
  if (!isValidMerchantProductPrice(source.price)) {
    throw new Error('价格必须为有效的非负金额')
  }
  const priceYuan = Number(source.price ?? 0)
  const priceCents = Math.round(priceYuan * 100)
  const payload = {
    store_id: Number(source.store_id ?? storeId ?? 0),
    name: String(source.name ?? '').trim(),
    price: priceCents,
    description: String(source.description ?? '').trim(),
    image_url: String(source.image_url ?? '').trim(),
    category: String(source.category ?? DEFAULT_CATEGORY).trim() || DEFAULT_CATEGORY,
    status: source.status === 'inactive' ? 'inactive' : 'active',
    sort: Number.isFinite(Number(source.sort)) ? Number(source.sort) : 100
  }

  if (source.sales_unit !== undefined) payload.sales_unit = String(source.sales_unit ?? '').trim()
  if (source.specification !== undefined) payload.specification = String(source.specification ?? '').trim()
  if (source.minimum_order_quantity !== undefined && source.minimum_order_quantity !== null) {
    const minimum = Number(source.minimum_order_quantity)
    if (Number.isInteger(minimum) && minimum > 0) payload.minimum_order_quantity = minimum
  }
  if (Array.isArray(source.price_tiers)) {
    payload.price_tiers = source.price_tiers.map((tier) => ({
      min_quantity: Number(tier.min_quantity),
      price: Number(tier.price)
    }))
  }

  return payload
}

export function extractMerchantProductList(response) {
  return Array.isArray(response?.data?.list) ? response.data.list : []
}
