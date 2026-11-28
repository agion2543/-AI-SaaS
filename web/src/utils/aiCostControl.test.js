import test from 'node:test'
import assert from 'node:assert/strict'
import {
  OVER_LIMIT_ACTIONS,
  budgetMerchantLabel,
  centsToYuan,
  formatCents,
  formatPricePerMillion,
  merchantDisplayName,
  newUUID,
  overLimitActionLabel,
  periodTypeLabel,
  scenarioLabel,
  storeDisplayName,
  yuanToCents,
} from './aiCostControl.js'

test('over-limit actions cover all four backend strategies', () => {
  assert.deepEqual(
    OVER_LIMIT_ACTIONS.map((item) => item.value),
    ['stop', 'downgrade', 'restrict_expensive', 'manual_review'],
  )
  assert.equal(overLimitActionLabel('stop'), '停止 AI 调用')
  assert.equal(overLimitActionLabel('downgrade'), '降级到低成本模型')
  assert.equal(overLimitActionLabel('restrict_expensive'), '限制高成本功能')
  assert.equal(overLimitActionLabel('manual_review'), '人工审核后恢复')
  assert.equal(overLimitActionLabel('unknown'), 'unknown')
})

test('period type and scenario labels stay honest about unknown values', () => {
  assert.equal(periodTypeLabel('daily'), '每日')
  assert.equal(periodTypeLabel('monthly'), '每月')
  assert.equal(periodTypeLabel('weekly'), 'weekly')
  assert.equal(scenarioLabel('referral_multi_copy'), '多文案批量生成')
  assert.equal(scenarioLabel('share_campaign_review'), '分享活动复盘')
  assert.equal(scenarioLabel('裂变海报'), '裂变海报')
  assert.equal(scenarioLabel(''), '未注明')
})

test('store dimension never invents an owner: NULL store shows 未指定门店', () => {
  assert.equal(storeDisplayName(0), '未指定门店')
  assert.equal(storeDisplayName(null), '未指定门店')
  assert.equal(storeDisplayName(undefined), '未指定门店')
  assert.equal(storeDisplayName(12), '门店 #12')
  assert.equal(storeDisplayName(0, '望京店'), '未指定门店')
})

test('store dimension prefers the real store name when the backend provides one', () => {
  assert.equal(storeDisplayName(12, '望京店'), '望京店')
  assert.equal(storeDisplayName(12, ''), '门店 #12')
  assert.equal(storeDisplayName(12, '  '), '门店 #12')
  assert.equal(storeDisplayName('12', '望京店'), '望京店')
})

test('merchant labels fall back to id and flag the global default row', () => {
  assert.equal(merchantDisplayName(3, '山风咖啡馆'), '山风咖啡馆')
  assert.equal(merchantDisplayName(3, ''), '商家 #3')
  assert.equal(merchantDisplayName(0, ''), '全局默认')
  assert.equal(budgetMerchantLabel(0, ''), '全局默认（无单独预算的商家）')
  assert.equal(budgetMerchantLabel(3, '山风咖啡馆'), '山风咖啡馆')
})

test('cents formatting keeps the estimated/actual split to the caller', () => {
  assert.equal(formatCents(12345), '¥123.45')
  assert.equal(formatCents(0), '¥0.00')
  assert.equal(formatCents(-500), '-¥5.00')
  assert.equal(formatCents(null), '—')
  assert.equal(formatCents(undefined), '—')
  assert.equal(formatPricePerMillion(250), '¥2.50 / 百万 Token')
  assert.equal(formatPricePerMillion(null), '—')
})

test('yuan to cents conversion rejects invalid input instead of truncating', () => {
  assert.equal(yuanToCents(12.34), 1234)
  assert.equal(yuanToCents(0), 0)
  assert.equal(yuanToCents('99.99'), 9999)
  assert.equal(yuanToCents(-1), null)
  assert.equal(yuanToCents(NaN), null)
  assert.equal(yuanToCents(''), null)
  assert.equal(yuanToCents(null), null)
  assert.equal(yuanToCents(1000000001), null)
  assert.equal(centsToYuan(12345), 123.45)
  assert.equal(centsToYuan(null), null)
})

test('generated idempotency keys are UUID shaped', () => {
  assert.match(
    newUUID(),
    /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/,
  )
})
