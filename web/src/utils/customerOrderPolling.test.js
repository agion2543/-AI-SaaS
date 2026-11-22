import test from 'node:test'
import assert from 'node:assert/strict'
import { customerOrderPollIntervalMs, isCustomerOrderStatusPollable } from './customerOrderPolling.js'

test('keeps polling every non-terminal customer order status that can still change', () => {
  for (const status of [
    'pending',
    'submitted',
    'payment_confirming',
    'paid',
    'received',
    'accepted',
    'preparing',
    'ready'
  ]) {
    assert.equal(isCustomerOrderStatusPollable(status), true, `${status} should be pollable`)
  }
})

test('stops polling terminal and unknown customer order statuses', () => {
  for (const status of ['completed', 'cancelled', 'closed', 'failed', 'refunded', 'unknown', undefined]) {
    assert.equal(isCustomerOrderStatusPollable(status), false, `${status} should not be pollable`)
  }
})

test('slows polling after the first 40 requests instead of leaving an open order stale', () => {
  assert.equal(customerOrderPollIntervalMs(0), 5000)
  assert.equal(customerOrderPollIntervalMs(39), 5000)
  assert.equal(customerOrderPollIntervalMs(40), 15000)
  assert.equal(customerOrderPollIntervalMs(100), 15000)
})
