import test from 'node:test'
import assert from 'node:assert/strict'
import { webcrypto } from 'node:crypto'
import {
  clearCustomerOrderAttempt,
  getOrCreateCustomerOrderAttempt,
  readCustomerOrderAttempt
} from './customerOrderAttempt.js'

const memoryStorage = () => {
  const values = new Map()
  return {
    getItem: (key) => values.get(key) || null,
    setItem: (key, value) => values.set(key, value),
    removeItem: (key) => values.delete(key)
  }
}

test('customer checkout attempt is stable across item ordering and client price serialization', async () => {
  const storage = memoryStorage()
  const payload = {
    store_id: 17,
    customer_phone: ' 13800138000 ',
    customer_note: ' 少辣 ',
    coupon_no: 'SYNTHETIC-COUPON',
    items: [
      { product_id: 2, quantity: 1, price: 100 },
      { product_id: 1, quantity: 3, price: 200 }
    ]
  }
  const first = await getOrCreateCustomerOrderAttempt(payload, null, { storage, cryptoImpl: webcrypto })
  const retry = await getOrCreateCustomerOrderAttempt({
    ...payload,
    customer_phone: '13800138000',
    customer_note: '少辣',
    items: [
      { product_id: 1, quantity: 3, price: 999999 },
      { product_id: 2, quantity: 1, price: 0 }
    ]
  }, null, { storage, cryptoImpl: webcrypto })

  assert.equal(retry.request_key, first.request_key)
  assert.equal(retry.access_token, first.access_token)
  assert.match(first.request_key, /^[a-f\d]{64}$/)
  assert.match(first.access_token, /^[a-f\d]{64}$/)
  assert.deepEqual(readCustomerOrderAttempt(17, storage), first)
  const storedAttempt = storage.getItem('customer_checkout_attempt_17')
  assert.doesNotMatch(storedAttempt, /13800138000|少辣|SYNTHETIC-COUPON/)
})

test('changed order intent receives a new attempt and clear only removes the matching attempt', async () => {
  const storage = memoryStorage()
  const payload = { store_id: 19, items: [{ product_id: 4, quantity: 1 }] }
  const first = await getOrCreateCustomerOrderAttempt(payload, null, { storage, cryptoImpl: webcrypto })
  const changed = await getOrCreateCustomerOrderAttempt({ ...payload, customer_note: '不同备注' }, null, { storage, cryptoImpl: webcrypto })

  assert.notEqual(changed.request_key, first.request_key)
  clearCustomerOrderAttempt(first, storage)
  assert.deepEqual(readCustomerOrderAttempt(19, storage), changed)
  clearCustomerOrderAttempt(changed, storage)
  assert.equal(readCustomerOrderAttempt(19, storage), null)
})

test('malformed saved credentials are ignored', () => {
  const storage = memoryStorage()
  storage.setItem('customer_checkout_attempt_21', JSON.stringify({
    store_id: 21,
    fingerprint: 'synthetic',
    request_key: 'not-random',
    access_token: 'also-not-random'
  }))
  assert.equal(readCustomerOrderAttempt(21, storage), null)
})
