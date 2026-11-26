import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'
import {
  getMerchantMobileActiveTab,
  getMerchantPcActivePath
} from '../src/utils/navigation.js'

const mobileRoutes = await readFile(new URL('../src/router/mobile.js', import.meta.url), 'utf8')
const pcRoutes = await readFile(new URL('../src/router/index.js', import.meta.url), 'utf8')

test('PC order and store-product detail pages highlight their parent menu', () => {
  assert.equal(getMerchantPcActivePath('/merchant/orders/ord-42'), '/merchant/orders')
  assert.equal(getMerchantPcActivePath('/merchant/orders/ord-42?from=dashboard'), '/merchant/orders')
  assert.equal(getMerchantPcActivePath('/merchant/stores/12/products'), '/merchant/stores')
  assert.equal(getMerchantPcActivePath('/merchant/stores/12/products?category=food'), '/merchant/stores')
  assert.equal(getMerchantPcActivePath('/merchant/finance/'), '/merchant/finance')
})

test('H5 routes select a stable parent tab, including secondary merchant pages', () => {
  assert.equal(getMerchantMobileActiveTab('/m/merchant/home'), 0)
  assert.equal(getMerchantMobileActiveTab('/m/merchant/ai'), 0)
  assert.equal(getMerchantMobileActiveTab('/m/merchant/settings'), 0)
  assert.equal(getMerchantMobileActiveTab('/m/merchant/orders'), 1)
  assert.equal(getMerchantMobileActiveTab('/m/merchant/orders/42'), 1)
  assert.equal(getMerchantMobileActiveTab('/m/merchant/products?group=group-buy'), 2)
  assert.equal(getMerchantMobileActiveTab('/m/merchant/stores'), 2)
  assert.equal(getMerchantMobileActiveTab('/m/merchant/coupons'), 2)
  assert.equal(getMerchantMobileActiveTab('/m/merchant/promotions'), 2)
  assert.equal(getMerchantMobileActiveTab('/m/merchant/group-buy'), 2)
  assert.equal(getMerchantMobileActiveTab('/m/merchant/finance/reconciliation'), 3)
})

test('existing affected PC and H5 routes remain declared', () => {
  for (const route of ["path: 'orders/:id'", "path: 'stores/:storeId/products'"]) {
    assert.ok(pcRoutes.includes(route), `Missing PC route declaration: ${route}`)
  }

  for (const route of [
    "path: 'orders/:id'",
    "path: 'products'",
    "path: 'stores'",
    "path: 'finance'",
    "path: 'coupons'",
    "path: 'ai'",
    "path: 'settings'"
  ]) {
    assert.ok(mobileRoutes.includes(route), `Missing H5 route declaration: ${route}`)
  }
})
