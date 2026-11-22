import test from 'node:test'
import assert from 'node:assert/strict'
import { getPlatformCostPeriodValidationError } from './platformCostPeriod.js'

test('accepts a valid period of exactly 366 inclusive calendar days', () => {
  assert.equal(getPlatformCostPeriodValidationError('2024-01-01', '2024-12-31', '2026-11-19'), '')
})

test('rejects periods longer than the server limit', () => {
  assert.equal(
    getPlatformCostPeriodValidationError('2024-01-01', '2025-01-01', '2026-11-19'),
    '统计区间最多为 366 天。',
  )
})

test('rejects reversed, future, malformed, and incomplete date ranges', () => {
  assert.equal(getPlatformCostPeriodValidationError('2026-11-10', '2026-11-09', '2026-11-19'), '结束日期不能早于开始日期。')
  assert.equal(getPlatformCostPeriodValidationError('2026-11-10', '2026-11-20', '2026-11-19'), '结束日期不能晚于今天。')
  assert.equal(getPlatformCostPeriodValidationError('2026-02-30', '2026-03-01', '2026-11-19'), '日期必须使用有效的 YYYY-MM-DD 格式。')
  assert.equal(getPlatformCostPeriodValidationError('2026-11-10', '', '2026-11-19'), '请选择完整的开始日期和结束日期。')
})
