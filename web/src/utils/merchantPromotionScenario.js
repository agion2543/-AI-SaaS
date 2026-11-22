const MAX_SAFE_CENTS = Number.MAX_SAFE_INTEGER

function amountToCents(value, label) {
  if (typeof value !== 'number' || !Number.isFinite(value) || value < 0) {
    throw new RangeError(`${label}必须是大于或等于 0 的有效金额。`)
  }
  const cents = Math.round(value * 100)
  if (!Number.isSafeInteger(cents)) {
    throw new RangeError(`${label}超出安全计算范围，请调小假设。`)
  }
  return cents
}

function safeProduct(left, right, label) {
  const result = left * right
  if (!Number.isSafeInteger(result)) {
    throw new RangeError(`${label}超出安全计算范围，请调小假设。`)
  }
  return result
}

/**
 * Calculates an estimate from user-entered promotion assumptions only.
 * Monetary inputs are yuan; returned monetary values are integer CNY cents.
 * This function does not read or write merchant, order, ledger, API, or database data.
 */
export function calculateMerchantPromotionScenario({
  promotionSellingPriceYuan,
  productCostYuan,
  giftCostYuan,
  packagingCostYuan,
  perUnitChannelFeeYuan,
  plannedValidQuantity,
  oneOffCampaignCostYuan
}) {
  if (!Number.isSafeInteger(plannedValidQuantity) || plannedValidQuantity < 0) {
    throw new RangeError('预计有效成交笔数必须是大于或等于 0 的整数。')
  }

  const promotionSellingPriceCents = amountToCents(promotionSellingPriceYuan, '促销成交价')
  const productCostCents = amountToCents(productCostYuan, '商品成本')
  const giftCostCents = amountToCents(giftCostYuan, '赠品成本')
  const packagingCostCents = amountToCents(packagingCostYuan, '包装成本')
  const perUnitChannelFeeCents = amountToCents(perUnitChannelFeeYuan, '单笔渠道费用')
  const oneOffCampaignCostCents = amountToCents(oneOffCampaignCostYuan, '一次性活动费用')

  const directCostsCents = [
    productCostCents,
    giftCostCents,
    packagingCostCents,
    perUnitChannelFeeCents
  ].reduce((sum, cost) => {
    const total = sum + cost
    if (!Number.isSafeInteger(total)) {
      throw new RangeError('单笔直接成本超出安全计算范围，请调小假设。')
    }
    return total
  }, 0)
  const contributionPerUnitCents = promotionSellingPriceCents - directCostsCents
  if (!Number.isSafeInteger(contributionPerUnitCents)) {
    throw new RangeError('单笔促销贡献超出安全计算范围，请调小假设。')
  }

  const plannedContributionCents = safeProduct(
    contributionPerUnitCents,
    plannedValidQuantity,
    '预计促销贡献'
  )
  const projectedContributionCents = plannedContributionCents - oneOffCampaignCostCents
  if (!Number.isSafeInteger(projectedContributionCents)) {
    throw new RangeError('预计促销贡献超出安全计算范围，请调小假设。')
  }

  const breakEvenAchievable = contributionPerUnitCents > 0
  const breakEvenQuantity = breakEvenAchievable
    ? Math.ceil(oneOffCampaignCostCents / contributionPerUnitCents)
    : null
  if (breakEvenQuantity !== null && !Number.isSafeInteger(breakEvenQuantity)) {
    throw new RangeError('保本成交笔数超出安全计算范围，请调小假设。')
  }

  return {
    promotionSellingPriceCents,
    productCostCents,
    giftCostCents,
    packagingCostCents,
    perUnitChannelFeeCents,
    plannedValidQuantity,
    oneOffCampaignCostCents,
    contributionPerUnitCents,
    projectedContributionCents,
    breakEvenAchievable,
    breakEvenQuantity
  }
}
