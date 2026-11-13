import test from 'node:test'
import assert from 'node:assert/strict'
import {
  buildCustomerOrderPayload,
  customerResponseList,
  fetchCustomerCouponsForPhone,
  isCustomerOrderPaid,
  isDevMockPaymentFlagEnabled,
  isValidCustomerPhone
} from './mobileCustomerOrder.js'

test('parses the backend list envelope and rejects non-list shapes', () => {
  const products = [{ id: 7, price: 1250 }]
  assert.deepEqual(customerResponseList({ data: { list: products } }), products)
  assert.deepEqual(customerResponseList({ data: products }), [])
  assert.deepEqual(customerResponseList({ data: { list: null } }), [])
})

test('coupon query requires a valid phone and sends only the existing phone query parameter', async () => {
  const calls = []
  const fetchCoupons = async (...args) => {
    calls.push(args)
    return { data: { list: [{ coupon_no: 'SYN-COUPON-01', amount: 500 }] } }
  }

  assert.equal(isValidCustomerPhone('13800000000'), true)
  assert.equal(isValidCustomerPhone(' 13800000000 '), true)
  assert.equal(isValidCustomerPhone('1380000000'), false)
  assert.equal(await fetchCustomerCouponsForPhone(fetchCoupons, 12, ''), null)
  assert.equal(await fetchCustomerCouponsForPhone(fetchCoupons, 12, '1380000000'), null)
  assert.deepEqual(calls, [])

  const coupons = await fetchCustomerCouponsForPhone(fetchCoupons, '12', ' 13800000000 ')
  assert.deepEqual(coupons, [{ coupon_no: 'SYN-COUPON-01', amount: 500 }])
  assert.deepEqual(calls, [['12', { phone: '13800000000' }]])
})

test('builds only fields supported by the customer order DTO and preserves the Alipay page path', () => {
  const payload = buildCustomerOrderPayload({
    storeId: '12',
    customerPhone: ' 13800000000 ',
    customerNote: ' 少辣 ',
    shareCode: ' SHARE-SYN ',
    couponNo: ' COUPON-SYN ',
    items: [{ id: '7', count: '2', price: '1250' }]
  })

  assert.deepEqual(payload, {
    store_id: 12,
    customer_phone: '13800000000',
    customer_note: '少辣',
    share_code: 'SHARE-SYN',
    coupon_no: 'COUPON-SYN',
    payment_channel: 'alipay',
    pay_mode: 'page',
    items: [{ product_id: 7, quantity: 2, price: 1250 }]
  })
  assert.equal('coupon_id' in payload, false)
  assert.equal('remark' in payload, false)
})

test('mock-payment development indicator requires DEV and an explicit opt-in', () => {
  assert.equal(isDevMockPaymentFlagEnabled({ DEV: true, VITE_ENABLE_MOCK_PAYMENT: 'true' }), true)
  assert.equal(isDevMockPaymentFlagEnabled({ DEV: true, VITE_ENABLE_MOCK_PAYMENT: 'false' }), false)
  assert.equal(isDevMockPaymentFlagEnabled({ DEV: true }), false)
  assert.equal(isDevMockPaymentFlagEnabled({ DEV: false, VITE_ENABLE_MOCK_PAYMENT: 'true' }), false)
  assert.equal(isDevMockPaymentFlagEnabled({ DEV: 'true', VITE_ENABLE_MOCK_PAYMENT: 'true' }), false)
})

test('an order remains unpaid until the backend supplies a genuine paid_at value', () => {
  assert.equal(isCustomerOrderPaid({ status: 'pending', paid_at: null }), false)
  assert.equal(isCustomerOrderPaid({ status: 'payment_confirming', paid_at: null }), false)
  assert.equal(isCustomerOrderPaid({ status: 'received' }), false)
  assert.equal(isCustomerOrderPaid({ status: 'received', paid_at: '2026-11-13T12:00:00Z' }), true)
})
