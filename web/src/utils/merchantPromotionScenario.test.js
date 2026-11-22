import test from 'node:test'
import assert from 'node:assert/strict'
import { calculateMerchantPromotionScenario } from './merchantPromotionScenario.js'

const assumptions = {
  promotionSellingPriceYuan: 69.9,
  productCostYuan: 25,
  giftCostYuan: 5,
  packagingCostYuan: 2,
  perUnitChannelFeeYuan: 6.99,
  plannedValidQuantity: 100,
  oneOffCampaignCostYuan: 1200
}

test('calculates per-unit and projected contribution and rounds break-even quantity up', () => {
  assert.deepEqual(calculateMerchantPromotionScenario(assumptions), {
    promotionSellingPriceCents: 6990,
    productCostCents: 2500,
    giftCostCents: 500,
    packagingCostCents: 200,
    perUnitChannelFeeCents: 699,
    plannedValidQuantity: 100,
    oneOffCampaignCostCents: 120000,
    contributionPerUnitCents: 3091,
    projectedContributionCents: 189100,
    breakEvenAchievable: true,
    breakEvenQuantity: 39
  })
})

test('shows a negative projected contribution when the plan does not cover one-off costs', () => {
  const result = calculateMerchantPromotionScenario({
    ...assumptions,
    plannedValidQuantity: 10
  })
  assert.equal(result.projectedContributionCents, -89090)
  assert.equal(result.breakEvenQuantity, 39)
})

test('break-even quantity is zero when contribution is positive and there is no one-off cost', () => {
  const result = calculateMerchantPromotionScenario({
    ...assumptions,
    oneOffCampaignCostYuan: 0
  })
  assert.equal(result.breakEvenAchievable, true)
  assert.equal(result.breakEvenQuantity, 0)
})

test('more transactions cannot cover one-off costs when per-unit contribution is zero or negative', () => {
  for (const productCostYuan of [69.9, 80]) {
    const result = calculateMerchantPromotionScenario({
      ...assumptions,
      productCostYuan
    })
    assert.equal(result.contributionPerUnitCents <= 0, true)
    assert.equal(result.breakEvenAchievable, false)
    assert.equal(result.breakEvenQuantity, null)
  }
})

test('rejects invalid monetary assumptions and missing inputs', () => {
  assert.throws(() => calculateMerchantPromotionScenario({ ...assumptions, giftCostYuan: -1 }), /赠品成本/)
  assert.throws(() => calculateMerchantPromotionScenario({ ...assumptions, oneOffCampaignCostYuan: Number.NaN }), /一次性活动费用/)
  assert.throws(() => calculateMerchantPromotionScenario({ ...assumptions, promotionSellingPriceYuan: undefined }), /促销成交价/)
})

test('requires planned valid quantity to be a non-negative safe integer', () => {
  for (const plannedValidQuantity of [-1, 1.5, Number.MAX_SAFE_INTEGER + 1]) {
    assert.throws(
      () => calculateMerchantPromotionScenario({ ...assumptions, plannedValidQuantity }),
      /预计有效成交笔数/
    )
  }
})

test('rejects projections that exceed the safe integer-cent range', () => {
  assert.throws(() => calculateMerchantPromotionScenario({
    promotionSellingPriceYuan: 100000000,
    productCostYuan: 0,
    giftCostYuan: 0,
    packagingCostYuan: 0,
    perUnitChannelFeeYuan: 0,
    plannedValidQuantity: 100000000,
    oneOffCampaignCostYuan: 0
  }), /安全计算范围/)
})
