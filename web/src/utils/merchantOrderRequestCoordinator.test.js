import test from 'node:test'
import assert from 'node:assert/strict'
import { createMerchantOrderRequestCoordinator } from './merchantOrderRequestCoordinator.js'

test('shares an in-flight request instead of starting a duplicate page load', () => {
  const coordinator = createMerchantOrderRequestCoordinator()
  const first = coordinator.begin(1)
  first.request.promise = Promise.resolve('page one')

  const duplicate = coordinator.begin(2)

  assert.equal(first.isNew, true)
  assert.equal(duplicate.isNew, false)
  assert.strictEqual(duplicate.request, first.request)
  assert.equal(duplicate.request.page, 1)
  assert.strictEqual(duplicate.request.promise, first.request.promise)
})

test('invalidates stale responses without allowing them to finish the latest request', () => {
  const coordinator = createMerchantOrderRequestCoordinator()
  const stale = coordinator.begin(2).request

  coordinator.invalidate()
  const current = coordinator.begin(1).request

  assert.equal(coordinator.isCurrent(stale), false)
  assert.equal(coordinator.finish(stale), false)
  assert.equal(coordinator.isCurrent(current), true)
  assert.equal(coordinator.finish(current), true)
  assert.equal(coordinator.isCurrent(current), false)
})

test('allows the next page to start after the prior request has finished', () => {
  const coordinator = createMerchantOrderRequestCoordinator()
  const firstPage = coordinator.begin(1).request

  assert.equal(coordinator.finish(firstPage), true)
  const secondPage = coordinator.begin(2)

  assert.equal(secondPage.isNew, true)
  assert.equal(secondPage.request.page, 2)
})
