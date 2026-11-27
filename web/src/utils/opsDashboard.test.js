import test from 'node:test'
import assert from 'node:assert/strict'
import {
  opsDeviationPercent,
  opsDeviationText,
  opsDeviationType,
  opsProductStatusLabel,
  opsProductStatusType,
  opsSuggestionStatusLabel,
  opsNextSuggestionActions,
  opsDormantTierLabel,
  opsTodayDate,
} from './opsDashboard.js'

test('deviation percent: null without a target, rounded otherwise', () => {
  assert.equal(opsDeviationPercent(50, 0), null)
  assert.equal(opsDeviationPercent(50, null), null)
  assert.equal(opsDeviationPercent(50000, 100000), 50)
  assert.equal(opsDeviationPercent(100000, 100000), 100)
  assert.equal(opsDeviationPercent(120000, 100000), 120)
  assert.equal(opsDeviationPercent(1, 3), 33)
})

test('deviation text and type follow the rate', () => {
  assert.equal(opsDeviationText(50000, 100000), '完成率 50%')
  assert.equal(opsDeviationText(50000, 0), '未设目标')
  assert.equal(opsDeviationType(120000, 100000), 'success')
  assert.equal(opsDeviationType(80000, 100000), 'warning')
  assert.equal(opsDeviationType(30000, 100000), 'danger')
  assert.equal(opsDeviationType(30000, 0), 'info')
})

test('product status labels and tag types', () => {
  assert.equal(opsProductStatusLabel('bestseller'), '畅销')
  assert.equal(opsProductStatusLabel('slow'), '滞销')
  assert.equal(opsProductStatusLabel('unsold'), '未动销')
  assert.equal(opsProductStatusLabel('normal'), '正常')
  assert.equal(opsProductStatusType('bestseller'), 'success')
  assert.equal(opsProductStatusType('unsold'), 'danger')
  assert.equal(opsProductStatusLabel('nope'), '未知')
})

test('suggestion status labels and legal next actions', () => {
  assert.equal(opsSuggestionStatusLabel('pending'), '待处理')
  assert.equal(opsSuggestionStatusLabel('executed'), '已执行')
  assert.deepEqual(opsNextSuggestionActions('pending'), [
    { status: 'adopted', label: '采纳' },
    { status: 'ignored', label: '忽略' },
  ])
  assert.deepEqual(opsNextSuggestionActions('adopted'), [{ status: 'executed', label: '标记已执行' }])
  assert.deepEqual(opsNextSuggestionActions('executed'), [])
  assert.deepEqual(opsNextSuggestionActions('ignored'), [])
})

test('dormant tier labels and today date shape', () => {
  assert.equal(opsDormantTierLabel('light'), '轻度沉睡')
  assert.equal(opsDormantTierLabel('deep'), '深度沉睡')
  assert.match(opsTodayDate(), /^\d{4}-\d{2}-\d{2}$/)
})
