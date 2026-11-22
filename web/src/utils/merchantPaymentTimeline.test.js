import test from 'node:test'
import assert from 'node:assert/strict'
import { merchantPaymentTimelineStage } from './merchantPaymentTimeline.js'

test('accepted unpaid order is shown as submitted, not payment complete', () => {
  const stage = merchantPaymentTimelineStage('accepted', false)
  assert.deepEqual(stage, {
    title: '订单提交',
    description: '订单已提交给商家，当前未确认收款。'
  })
  assert.doesNotMatch(`${stage.title} ${stage.description}`, /支付完成|支付成功/)
})

test('confirmed paid order retains its payment-complete timeline label', () => {
  assert.deepEqual(merchantPaymentTimelineStage('accepted', true), {
    title: '支付完成',
    description: '支付成功后直接进入商家处理，无需商家审核付款。'
  })
})

test('payment confirmation and pay-later statuses keep their distinct explanations', () => {
  assert.deepEqual(merchantPaymentTimelineStage('payment_confirming', false), {
    title: '订单提交',
    description: '顾客已标记付款，等待商家确认收款。'
  })
  assert.deepEqual(merchantPaymentTimelineStage('submitted', false), {
    title: '订单提交',
    description: '订单已提交给商家，后续再结算。'
  })
  assert.deepEqual(merchantPaymentTimelineStage('preparing', false), {
    title: '订单提交',
    description: '订单已提交给商家，后续再结算。'
  })
})
