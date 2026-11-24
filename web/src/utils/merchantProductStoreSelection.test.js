import test from 'node:test'
import assert from 'node:assert/strict'
import { resolveMerchantProductStoreIndex } from './merchantProductStoreSelection.js'

const stores = [
  { id: 11, name: '首店' },
  { id: '22', name: '目标店' },
  { id: 33, name: '第三店' }
]

test('selects the requested store whether query and API IDs are strings or numbers', () => {
  assert.equal(resolveMerchantProductStoreIndex(stores, '22'), 1)
  assert.equal(resolveMerchantProductStoreIndex(stores, 33), 2)
})

test('uses the first usable repeated query value and falls back to the first store when unmatched', () => {
  assert.equal(resolveMerchantProductStoreIndex(stores, ['', '33']), 2)
  assert.equal(resolveMerchantProductStoreIndex(stores, '999'), 0)
})

test('keeps the existing first-store default when the query or store list is absent', () => {
  for (const requestedId of [undefined, null, '', [], [null, '']]) {
    assert.equal(resolveMerchantProductStoreIndex(stores, requestedId), 0)
  }
  assert.equal(resolveMerchantProductStoreIndex([], '22'), 0)
})
