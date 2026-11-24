import test from 'node:test'
import assert from 'node:assert/strict'
import { couponAmountYuan, couponThresholdYuan } from './merchantCouponMoney.js'

test('formats merchant coupon API amount and threshold from fen to yuan', () => {
  const coupon = { amount: 500, threshold: 3000 }
  assert.equal(couponAmountYuan(coupon), '5.00')
  assert.equal(couponThresholdYuan(coupon), '30.00')
})

test('preserves a zero coupon value and safely formats absent DTO values', () => {
  assert.equal(couponAmountYuan({ amount: 0 }), '0.00')
  assert.equal(couponThresholdYuan({ threshold: 0 }), '0.00')
  assert.equal(couponAmountYuan({}), '0.00')
  assert.equal(couponThresholdYuan(null), '0.00')
})
