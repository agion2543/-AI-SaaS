import test from 'node:test'
import assert from 'node:assert/strict'
import {
  checkoutExpiryTimestamp,
  isCheckoutDeadlineReached,
  isUnpaidCheckoutOrder,
  remainingCheckoutTime
} from './customerOrderExpiry.js'

const expireTime = '2026-11-23T14:30:00.000Z'
const deadline = Date.parse(expireTime)

for (const status of ['pending', 'failed']) {
  test(`shows a checkout deadline for positive unpaid ${status} orders`, () => {
    const order = { status, total_amount: 1200, paid_at: null, expire_time: expireTime }
    assert.equal(isUnpaidCheckoutOrder(order), true)
    assert.equal(checkoutExpiryTimestamp(order), deadline)
    assert.equal(isCheckoutDeadlineReached(order, deadline - 1), false)
    assert.equal(remainingCheckoutTime(order, deadline - 1000), 1000)
    assert.equal(isCheckoutDeadlineReached(order, deadline), true)
    assert.equal(remainingCheckoutTime(order, deadline), 0)
  })
}

test('does not expose a retry countdown for zero-amount, paid, closed, or non-checkout orders', () => {
  const cases = [
    { status: 'pending', total_amount: 0, expire_time: expireTime },
    { status: 'pending', total_amount: 1200, paid_at: '2026-11-23T14:05:00.000Z', expire_time: expireTime },
    { status: 'closed', total_amount: 1200, expire_time: expireTime },
    { status: 'payment_confirming', total_amount: 1200, expire_time: expireTime }
  ]
  for (const order of cases) {
    assert.equal(isUnpaidCheckoutOrder(order), false)
    assert.equal(checkoutExpiryTimestamp(order), null)
    assert.equal(isCheckoutDeadlineReached(order, deadline), false)
    assert.equal(remainingCheckoutTime(order, deadline), null)
  }
})

test('does not invent a client deadline if the server omitted expire_time', () => {
  const order = { status: 'pending', total_amount: 1200, created_at: '2026-11-23T14:00:00.000Z' }
  assert.equal(checkoutExpiryTimestamp(order), null)
  assert.equal(remainingCheckoutTime(order, deadline - 1000), null)
})
