export function getMinimumOrderQuantity(product) {
  const quantity = Number(product?.minimum_order_quantity || 1)
  return Number.isInteger(quantity) && quantity > 0 ? quantity : 1
}

export function getProductPriceTiers(product) {
  if (!Array.isArray(product?.price_tiers)) return []
  return product.price_tiers
    .map((tier) => ({ min_quantity: Number(tier.min_quantity), price: Number(tier.price) }))
    .filter((tier) => Number.isInteger(tier.min_quantity) && tier.min_quantity >= 2 && Number.isFinite(tier.price) && tier.price >= 0)
    .sort((a, b) => a.min_quantity - b.min_quantity)
}

export function getProductUnitPrice(product, quantity = 1) {
  const basePrice = Number(product?.price || 0)
  const requestedQuantity = Math.max(Number(quantity) || 1, 1)
  return getProductPriceTiers(product).reduce((price, tier) => (
    requestedQuantity >= tier.min_quantity ? tier.price : price
  ), basePrice)
}

export function describeProductPricing(product) {
  const pieces = []
  if (product?.specification) pieces.push(String(product.specification))
  if (product?.sales_unit) pieces.push(`按${product.sales_unit}计价`)
  const minimum = getMinimumOrderQuantity(product)
  if (minimum > 1) pieces.push(`最低起订 ${minimum}${product?.sales_unit || '件'}`)
  const tiers = getProductPriceTiers(product)
  if (tiers.length) {
    pieces.push(tiers.map((tier) => `${tier.min_quantity}+ ${formatCents(tier.price)}${product?.sales_unit ? `/${product.sales_unit}` : ''}`).join(' · '))
  }
  return pieces.join(' · ')
}

export function formatCents(value) {
  return `¥${(Number(value || 0) / 100).toFixed(2)}`
}
