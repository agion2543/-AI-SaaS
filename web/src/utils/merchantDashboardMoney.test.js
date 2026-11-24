import test from 'node:test'
import assert from 'node:assert/strict'
import { successfulRefundsOnDate } from './merchantDashboardMoney.js'

test('counts only successful refunds created on the selected date', () => {
  const refunds = [
    { id: 1, amount: 200, status: 'success', created_at: '2026-11-23T09:00:00Z' },
    { id: 2, amount: 300, status: 'processing', created_at: '2026-11-23T10:00:00Z' },
    { id: 3, amount: 400, status: 'failed', created_at: '2026-11-23T11:00:00Z' },
    { id: 4, amount: 500, status: 'unknown', created_at: '2026-11-23T12:00:00Z' },
    { id: 5, amount: 600, status: 'success', created_at: '2026-11-22T23:59:00Z' }
  ]

  const result = successfulRefundsOnDate(refunds, '2026-11-23')
  assert.deepEqual(result.map((refund) => refund.id), [1])
  assert.equal(result.reduce((sum, refund) => sum + refund.amount, 0), 200)
})

test('returns an empty result for missing or non-array refund payloads', () => {
  assert.deepEqual(successfulRefundsOnDate(undefined, '2026-11-23'), [])
  assert.deepEqual(successfulRefundsOnDate({}, '2026-11-23'), [])
})
