export const promotionPaidAmountFen = (order) => Number(order?.total_amount ?? order?.amount ?? 0)
