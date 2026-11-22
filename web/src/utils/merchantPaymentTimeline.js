export const merchantPaymentTimelineStage = (status, isConfirmedPaid) => {
  if (status === 'payment_confirming') {
    return {
      title: '订单提交',
      description: '顾客已标记付款，等待商家确认收款。'
    }
  }

  if (['submitted', 'preparing'].includes(status)) {
    return {
      title: '订单提交',
      description: '订单已提交给商家，后续再结算。'
    }
  }

  if (isConfirmedPaid) {
    return {
      title: '支付完成',
      description: '支付成功后直接进入商家处理，无需商家审核付款。'
    }
  }

  return {
    title: '订单提交',
    description: '订单已提交给商家，当前未确认收款。'
  }
}
