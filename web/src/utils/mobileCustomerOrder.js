export const isValidCustomerPhone = (value) => /^1\d{10}$/.test(String(value || '').trim())

export const customerResponseList = (response) => {
  const list = response?.data?.list
  return Array.isArray(list) ? list : []
}

export const fetchCustomerCouponsForPhone = async (fetchCoupons, storeId, phone) => {
  const normalizedPhone = String(phone || '').trim()
  if (!isValidCustomerPhone(normalizedPhone)) return null
  const response = await fetchCoupons(storeId, { phone: normalizedPhone })
  return customerResponseList(response)
}

export const buildCustomerOrderPayload = ({
  storeId,
  customerPhone = '',
  customerNote = '',
  shareCode = '',
  couponNo = '',
  paymentChannel = 'alipay',
  payMode = 'page',
  items = []
}) => ({
  store_id: Number(storeId),
  customer_phone: String(customerPhone || '').trim(),
  customer_note: String(customerNote || '').trim(),
  share_code: String(shareCode || '').trim(),
  coupon_no: String(couponNo || '').trim(),
  payment_channel: paymentChannel,
  pay_mode: payMode,
  items: items.map((item) => ({
    product_id: Number(item.product_id ?? item.id),
    quantity: Number(item.quantity ?? item.count),
    price: Number(item.price)
  }))
})

export const isDevMockPaymentFlagEnabled = (env) => (
  env?.DEV === true && env?.VITE_ENABLE_MOCK_PAYMENT === 'true'
)

export const isCustomerOrderPaid = (order) => Boolean(order?.paid_at)
