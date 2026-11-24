import test from 'node:test'
import assert from 'node:assert/strict'
import {
  describeProductPricing,
  formatCents,
  getMinimumOrderQuantity,
  getProductUnitPrice,
} from './productPricing.js'

test('legacy retail product without tier configuration keeps its existing unit price', () => {
  const product = { price: 1299 }
  assert.equal(getMinimumOrderQuantity(product), 1)
  assert.equal(getProductUnitPrice(product, 1), 1299)
  assert.equal(getProductUnitPrice(product, 8), 1299)
})
test('integer cent amounts format as yuan with two decimal places', () => {
  assert.equal(formatCents(1299), '¥12.99')
  assert.equal(formatCents(5), '¥0.05')
  assert.equal(formatCents(0), '¥0.00')
})

test('quantity-tier price changes only at merchant-configured thresholds', () => {
  const product = {
    price: 1000,
    price_tiers: [
      { min_quantity: 5, price: 800 },
      { min_quantity: 3, price: 900 },
    ],
  }
  assert.equal(getProductUnitPrice(product, 2), 1000)
  assert.equal(getProductUnitPrice(product, 3), 900)
  assert.equal(getProductUnitPrice(product, 4), 900)
  assert.equal(getProductUnitPrice(product, 5), 800)
})

test('customer pricing summary includes specification, unit, MOQ, and merchant tiers', () => {
  const product = {
    price: 1000,
    sales_unit: '袋',
    specification: '500g/袋',
    minimum_order_quantity: 2,
    price_tiers: [{ min_quantity: 5, price: 800 }],
  }
  assert.equal(getMinimumOrderQuantity(product), 2)
  assert.equal(describeProductPricing(product), '500g/袋 · 按袋计价 · 最低起订 2袋 · 5+ ¥8.00/袋')
})
