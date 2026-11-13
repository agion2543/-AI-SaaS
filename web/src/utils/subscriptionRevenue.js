export const providerPaidSubscriptionOrders = (orders) =>
  (Array.isArray(orders) ? orders : []).filter((order) => order?.status === 'paid')

export const providerPaidSubscriptionRevenueCents = (orders, amountOfOrder = (order) => order?.total_amount ?? order?.amount ?? 0) =>
  providerPaidSubscriptionOrders(orders).reduce((total, order) => total + Number(amountOfOrder(order) || 0), 0)
