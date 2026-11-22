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
  if (!Number.isSafeInteger(result) || result > MAX_SAFE_CENTS) {
    throw new RangeError(`${label}超出安全计算范围，请调小假设。`)
  }
  return result
}

function safeSum(left, right, label) {
  const result = left + right
  if (!Number.isSafeInteger(result) || result > MAX_SAFE_CENTS) {
    throw new RangeError(`${label}超出安全计算范围，请调小假设。`)
  }
  return result
}

/**
 * Calculates a monthly platform scenario from user-entered assumptions only.
 * Monetary inputs are yuan; all returned monetary values are integer CNY cents.
 * This function has no access to actual ledger, merchant, API, or database data.
 */
export function calculatePlatformCostScenario({
  monthlyAverageSubscriptionRevenuePerMerchantYuan,
  payingMerchantCount,
  fixedMonthlyCostsYuan,
  variableCostPerMerchantYuan
}) {
  if (!Number.isSafeInteger(payingMerchantCount) || payingMerchantCount < 0) {
    throw new RangeError('付费商家数必须是大于或等于 0 的整数。')
  }

  const revenuePerMerchantCents = amountToCents(
    monthlyAverageSubscriptionRevenuePerMerchantYuan,
    '月均订阅收入 / 付费商家'
  )
  const fixedMonthlyCostsCents = amountToCents(fixedMonthlyCostsYuan, '固定月成本')
  const variableCostPerMerchantCents = amountToCents(variableCostPerMerchantYuan, '单商家月变动成本')

  const projectedMonthlyRevenueCents = safeProduct(
    revenuePerMerchantCents,
    payingMerchantCount,
    '预计月收入'
  )
  const projectedMonthlyVariableCostCents = safeProduct(
    variableCostPerMerchantCents,
    payingMerchantCount,
    '预计月变动成本'
  )
  const projectedMonthlyTotalCostCents = safeSum(
    fixedMonthlyCostsCents,
    projectedMonthlyVariableCostCents,
    '预计月总成本'
  )
  const projectedMonthlyOperatingResultCents = projectedMonthlyRevenueCents - projectedMonthlyTotalCostCents
  if (!Number.isSafeInteger(projectedMonthlyOperatingResultCents)) {
    throw new RangeError('预计月度结余超出安全计算范围，请调小假设。')
  }

  const contributionPerMerchantCents = revenuePerMerchantCents - variableCostPerMerchantCents
  const breakEvenReachable = contributionPerMerchantCents > 0
  const breakEvenPayingMerchantCount = breakEvenReachable
    ? Math.ceil(fixedMonthlyCostsCents / contributionPerMerchantCents)
    : null
  if (breakEvenPayingMerchantCount !== null && !Number.isSafeInteger(breakEvenPayingMerchantCount)) {
    throw new RangeError('盈亏平衡商家数超出安全计算范围，请调小假设。')
  }

  return {
    projectedMonthlyRevenueCents,
    projectedMonthlyVariableCostCents,
    fixedMonthlyCostsCents,
    projectedMonthlyTotalCostCents,
    projectedMonthlyOperatingResultCents,
    contributionPerMerchantCents,
    breakEvenReachable,
    breakEvenPayingMerchantCount
  }
}
