import test from 'node:test'
import assert from 'node:assert/strict'
import { promotionPaidAmountFen } from './merchantPromotionMoney.js'

test('uses the post-discount total amount for paid promotion revenue', () => {
  assert.equal(promotionPaidAmountFen({ amount: 10000, total_amount: 8000 }), 8000)
})

test('falls back to amount only when total_amount is absent', () => {
  assert.equal(promotionPaidAmountFen({ amount: 10000 }), 10000)
  assert.equal(promotionPaidAmountFen({ amount: 10000, total_amount: 0 }), 0)
  assert.equal(promotionPaidAmountFen({}), 0)
})
