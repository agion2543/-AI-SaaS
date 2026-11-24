import test from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import {
  buildMerchantProductPayload,
  createMerchantProductForm,
  extractMerchantProductList,
  isValidMerchantProductPrice
} from './merchantProductContract.js'

const h5ProductForm = await readFile(new URL('../mobile/merchant/MerchantProducts.vue', import.meta.url), 'utf8')
const pcProductForm = await readFile(new URL('../views/merchants/MerchantStoreProductsView.vue', import.meta.url), 'utf8')
const customerOrderingViews = await Promise.all([
  '../mobile/customer/CustomerStore.vue',
  '../mobile/customer/CustomerCart.vue',
  '../mobile/customer/CustomerOrderConfirm.vue',
  '../views/customer/CustomerStoreView.vue'
].map((path) => readFile(new URL(path, import.meta.url), 'utf8')))

test('H5 presents image and optional flavor/specification before advanced settings', () => {
  const imageIndex = h5ProductForm.indexOf('label="商品图片"')
  const fieldIndex = h5ProductForm.indexOf('v-model="productForm.specification"')
  const advancedIndex = h5ProductForm.indexOf('<section class="advanced-settings">')
  assert.ok(imageIndex >= 0)
  assert.ok(imageIndex < fieldIndex)
  assert.ok(fieldIndex >= 0)
  assert.ok(fieldIndex < advancedIndex)
  assert.ok(h5ProductForm.includes('label="口味/规格（选填）"'))
})

test('H5 product list distinguishes load failure from an empty catalog and offers retry', () => {
  assert.ok(h5ProductForm.includes('v-model:error="productLoadError"'))
  assert.ok(h5ProductForm.includes('error-text="加载失败，点击重试"'))
  assert.ok(h5ProductForm.includes(":description=\"productLoadError ? '商品列表加载失败' : '暂无商品'\""))
  assert.ok(h5ProductForm.includes('v-if="!productLoadError && canWrite"'))
  assert.ok(h5ProductForm.includes('productLoadError.value = true'))
})

test('merchant product editors do not expose stock quantities', () => {
  assert.doesNotMatch(h5ProductForm, /stock|库存|售罄|sold.?out/i)
  assert.doesNotMatch(pcProductForm, /stock|库存|售罄|sold.?out/i)
})

test('customer menus, carts, and checkout do not gate orders by legacy stock', () => {
  for (const source of customerOrderingViews) {
    assert.doesNotMatch(source, /stock|库存|售罄|sold.?out/i)
  }
})

test('H5 price validation rejects empty, negative, invalid, and unsafe amounts while allowing zero', () => {
  for (const price of ['0', '0.01', '12.34']) {
    assert.equal(isValidMerchantProductPrice(price), true, `${price} should be accepted`)
  }
  for (const price of ['', '  ', '-0.01', 'not-a-price', '90071992547410']) {
    assert.equal(isValidMerchantProductPrice(price), false, `${price} should be rejected`)
    const form = createMerchantProductForm()
    form.price = price
    assert.throws(() => buildMerchantProductPayload(form, 7), /价格必须为有效的非负金额/)
  }
  assert.match(h5ProductForm, /validator:\s*isValidMerchantProductPrice/)
})

test('new H5 product maps yuan and visible fields to the backend DTO without stock quantities', () => {
  const form = createMerchantProductForm()
  form.name = ' 合成商品 '
  form.price = '12.34'
  form.description = ' 合成描述 '
  form.image_url = 'https://example.invalid/product.webp'
  form.specification = '微辣'

  const payload = buildMerchantProductPayload(form, 7)
  assert.deepEqual(payload, {
    store_id: 7,
    name: '合成商品',
    price: 1234,
    description: '合成描述',
    image_url: 'https://example.invalid/product.webp',
    category: '默认分类',
    status: 'active',
    sort: 100,
    sales_unit: '',
    specification: '微辣',
    minimum_order_quantity: 1,
    price_tiers: []
  })
  assert.equal('image' in payload, false)
  assert.equal('is_active' in payload, false)
})

test('new products have no stock field and send no stock quantity', () => {
  const form = createMerchantProductForm()
  form.price = '1.00'
  assert.equal(Object.hasOwn(form, 'stock'), false)
  assert.equal(Object.hasOwn(form, 'stock_enabled'), false)
  assert.equal(Object.hasOwn(buildMerchantProductPayload(form, 7), 'stock'), false)
})

test('editing maps backend fields while ignoring legacy stock values', () => {
  const product = {
    id: 21,
    store_id: 7,
    name: '合成商品',
    price: 1299,
    description: '原描述',
    image_url: 'https://example.invalid/old.webp',
    category: '主食',
    status: 'inactive',
    stock: null,
    sort: 23,
    sales_unit: '份',
    specification: '大份',
    minimum_order_quantity: 2,
    price_tiers: [{ min_quantity: 5, price: 999 }]
  }
  const form = createMerchantProductForm(product)
  assert.equal(form.price, '12.99')
  assert.equal(Object.hasOwn(form, 'stock'), false)

  const payload = buildMerchantProductPayload(form, product.store_id)
  assert.deepEqual(payload, {
    store_id: 7,
    name: '合成商品',
    price: 1299,
    description: '原描述',
    image_url: 'https://example.invalid/old.webp',
    category: '主食',
    status: 'inactive',
    sort: 23,
    sales_unit: '份',
    specification: '大份',
    minimum_order_quantity: 2,
    price_tiers: [{ min_quantity: 5, price: 999 }]
  })

  const toggled = buildMerchantProductPayload(form, product.store_id, { status: 'active' })
  assert.equal(toggled.status, 'active')
  assert.equal(Object.hasOwn(toggled, 'stock'), false)
  assert.equal(toggled.name, product.name)
  assert.equal(toggled.store_id, product.store_id)
  assert.equal(toggled.price, product.price)
  assert.deepEqual(toggled.price_tiers, product.price_tiers)
})

test('omits optional edit fields absent from the API response instead of clearing stored values', () => {
  const form = createMerchantProductForm({
    store_id: 3,
    name: '旧商品',
    price: 500,
    description: '',
    image_url: '',
    category: '默认分类',
    status: 'active',
    stock: 4,
    sort: 100
  })
  const payload = buildMerchantProductPayload(form, 3)
  assert.equal(Object.hasOwn(payload, 'stock'), false)
  for (const key of ['sales_unit', 'specification', 'minimum_order_quantity', 'price_tiers']) {
    assert.equal(Object.hasOwn(payload, key), false, `${key} should be omitted when absent`)
  }
})

test('extracts rows from the backend list envelope only', () => {
  const rows = [{ id: 1, name: '合成商品' }]
  assert.deepEqual(extractMerchantProductList({ data: { list: rows, total: 1 } }), rows)
  assert.deepEqual(extractMerchantProductList({ data: { items: rows } }), [])
  assert.deepEqual(extractMerchantProductList({ data: [] }), [])
})
