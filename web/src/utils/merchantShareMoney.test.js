import test from 'node:test'
import assert from 'node:assert/strict'
import { shareConfigAmountYuan } from './merchantShareMoney.js'

test('keeps explicitly configured zero amounts instead of replacing them with defaults', () => {
  const config = {
    friend_coupon_amount: 0,
    friend_coupon_threshold: 0,
    referrer_coupon_amount: 0,
    referrer_coupon_threshold: 0
  }
  assert.deepEqual([
    shareConfigAmountYuan(config.friend_coupon_amount, 500),
    shareConfigAmountYuan(config.friend_coupon_threshold, 3000),
    shareConfigAmountYuan(config.referrer_coupon_amount, 500),
    shareConfigAmountYuan(config.referrer_coupon_threshold, 3000)
  ], [0, 0, 0, 0])
})

test('uses defaults only when a configured amount is missing and converts fen to yuan', () => {
  assert.equal(shareConfigAmountYuan(undefined, 500), 5)
  assert.equal(shareConfigAmountYuan(null, 3000), 30)
  assert.equal(shareConfigAmountYuan(1250, 500), 12.5)
})
