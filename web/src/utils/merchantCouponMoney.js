export const couponAmountYuan = (coupon) => (Number(coupon?.amount ?? 0) / 100).toFixed(2)

export const couponThresholdYuan = (coupon) => (Number(coupon?.threshold ?? 0) / 100).toFixed(2)
