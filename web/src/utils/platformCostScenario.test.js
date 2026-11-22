import test from 'node:test'
import assert from 'node:assert/strict'
import { calculatePlatformCostScenario } from './platformCostScenario.js'

const assumptions = {
  monthlyAverageSubscriptionRevenuePerMerchantYuan: 129,
  payingMerchantCount: 10,
  fixedMonthlyCostsYuan: 500,
  variableCostPerMerchantYuan: 9.99
}

test('calculates monthly projections and rounds break-even merchant count up', () => {
  assert.deepEqual(calculatePlatformCostScenario(assumptions), {
    projectedMonthlyRevenueCents: 129000,
    projectedMonthlyVariableCostCents: 9990,
    fixedMonthlyCostsCents: 50000,
    projectedMonthlyTotalCostCents: 59990,
    projectedMonthlyOperatingResultCents: 69010,
    contributionPerMerchantCents: 11901,
    breakEvenReachable: true,
    breakEvenPayingMerchantCount: 5
  })
})

test('shows a projected monthly deficit below break-even', () => {
  const result = calculatePlatformCostScenario({ ...assumptions, payingMerchantCount: 2 })
  assert.equal(result.projectedMonthlyOperatingResultCents, -26198)
  assert.equal(result.breakEvenPayingMerchantCount, 5)
})

test('zero fixed costs need zero merchants only when contribution is positive', () => {
  const result = calculatePlatformCostScenario({
    monthlyAverageSubscriptionRevenuePerMerchantYuan: 100,
    payingMerchantCount: 0,
    fixedMonthlyCostsYuan: 0,
    variableCostPerMerchantYuan: 20
  })
  assert.equal(result.breakEvenReachable, true)
  assert.equal(result.breakEvenPayingMerchantCount, 0)
})

test('break-even is unreachable when per-merchant contribution is zero or negative', () => {
  for (const variableCostPerMerchantYuan of [129, 150]) {
    const result = calculatePlatformCostScenario({ ...assumptions, variableCostPerMerchantYuan })
    assert.equal(result.contributionPerMerchantCents <= 0, true)
    assert.equal(result.breakEvenReachable, false)
    assert.equal(result.breakEvenPayingMerchantCount, null)
  }
})

test('rejects invalid amounts and non-integer or negative merchant counts', () => {
  assert.throws(() => calculatePlatformCostScenario({ ...assumptions, fixedMonthlyCostsYuan: -1 }), /固定月成本/)
  assert.throws(() => calculatePlatformCostScenario({ ...assumptions, monthlyAverageSubscriptionRevenuePerMerchantYuan: Number.NaN }), /月均订阅收入/)
  assert.throws(() => calculatePlatformCostScenario({ ...assumptions, payingMerchantCount: 1.5 }), /付费商家数/)
  assert.throws(() => calculatePlatformCostScenario({ ...assumptions, payingMerchantCount: -1 }), /付费商家数/)
})

test('rejects projections whose integer-cent results are not safely representable', () => {
  assert.throws(() => calculatePlatformCostScenario({
    monthlyAverageSubscriptionRevenuePerMerchantYuan: 100000000,
    payingMerchantCount: 100000000,
    fixedMonthlyCostsYuan: 0,
    variableCostPerMerchantYuan: 0
  }), /安全计算范围/)
})
