import test from 'node:test'
import assert from 'node:assert/strict'
import {
  mergeMerchantOrderRouteState,
  normalizeMerchantOrderRouteState,
  shouldReloadOrdersForQuickModeChange
} from './merchantOrderRouteState.js'

test('normalizes supported status and gives it precedence over quick mode', () => {
  assert.deepEqual(
    normalizeMerchantOrderRouteState({ status: 'received', quick: 'pay_issue' }),
    { status: 'received', quick: 'all' }
  )
})

test('reloads when quick mode clears a status filter on the first page', () => {
  assert.equal(shouldReloadOrdersForQuickModeChange({ statusWasFiltered: true, currentPage: 1 }), true)
})

test('reloads page one when quick mode changes from a later page', () => {
  assert.equal(shouldReloadOrdersForQuickModeChange({ statusWasFiltered: false, currentPage: 2 }), true)
})

test('avoids a redundant request on unfiltered page one', () => {
  assert.equal(shouldReloadOrdersForQuickModeChange({ statusWasFiltered: false, currentPage: 1 }), false)
})

test('accepts a supported quick mode from a repeated query value', () => {
  assert.deepEqual(
    normalizeMerchantOrderRouteState({ quick: ['refund', 'pay_issue'] }),
    { status: '', quick: 'refund' }
  )
})

test('ignores unsupported route values', () => {
  assert.deepEqual(
    normalizeMerchantOrderRouteState({ status: 'drop-table', quick: 'unknown' }),
    { status: '', quick: 'all' }
  )
})

test('merges state while preserving unrelated query parameters and removes cleared filters', () => {
  assert.deepEqual(
    mergeMerchantOrderRouteState({ tab: 'orders', status: 'received', quick: 'refund' }, { status: '', quick: 'all' }),
    { tab: 'orders' }
  )
  assert.deepEqual(
    mergeMerchantOrderRouteState({ tab: 'orders', quick: 'refund' }, { status: 'completed', quick: 'all' }),
    { tab: 'orders', status: 'completed' }
  )
})
