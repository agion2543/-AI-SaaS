import test from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { createDebouncedAction, createRequestVersionGuard } from './merchantProductSearch.js'

const merchantProductsPage = await readFile(new URL('../mobile/merchant/MerchantProducts.vue', import.meta.url), 'utf8')

function createFakeTimers() {
  let nextId = 0
  const pending = new Map()

  return {
    setTimeout(callback, delay) {
      const id = ++nextId
      pending.set(id, { callback, delay })
      return id
    },
    clearTimeout(id) {
      pending.delete(id)
    },
    runAll() {
      const callbacks = [...pending.values()]
      pending.clear()
      for (const { callback } of callbacks) callback()
    },
    get pendingCount() {
      return pending.size
    },
    get scheduledDelays() {
      return [...pending.values()].map(({ delay }) => delay)
    }
  }
}

test('debounces rapid typing and treats clearing the field as the latest search', () => {
  const timers = createFakeTimers()
  const searches = []
  const debounced = createDebouncedAction(value => searches.push(value), 300, timers)

  debounced.schedule('咖')
  debounced.schedule('咖啡')
  debounced.schedule('')

  assert.equal(timers.pendingCount, 1)
  assert.deepEqual(timers.scheduledDelays, [300])
  timers.runAll()
  assert.deepEqual(searches, [''])
})

test('cancels a pending search on store change and flushes Enter without a duplicate call', () => {
  const timers = createFakeTimers()
  const searches = []
  const debounced = createDebouncedAction(value => searches.push(value), 300, timers)

  debounced.schedule('旧门店关键词')
  debounced.cancel()
  timers.runAll()
  assert.deepEqual(searches, [])

  debounced.schedule('面包')
  debounced.flush('面包')
  timers.runAll()
  debounced.flush('面包')
  assert.deepEqual(searches, ['面包'])

  debounced.schedule('豆浆')
  timers.runAll()
  debounced.flush('豆浆')
  assert.deepEqual(searches, ['面包', '豆浆'])
})

test('a new request version prevents an older response from replacing current results', () => {
  const guard = createRequestVersionGuard()
  const staleVersion = guard.current()
  const currentVersion = guard.next()
  const applied = []

  if (guard.isCurrent(staleVersion)) applied.push('旧结果')
  if (guard.isCurrent(currentVersion)) applied.push('新结果')

  assert.deepEqual(applied, ['新结果'])
})

test('pauses product pagination during debounced search and resumes after refresh', () => {
  assert.ok(merchantProductsPage.includes(':disabled="searchPending"'))
  assert.ok(merchantProductsPage.includes('searchPending.value = true'))
  assert.ok(merchantProductsPage.includes('if (searchPending.value) return'))
  assert.ok(merchantProductsPage.includes('searchPending.value = false'))
})
