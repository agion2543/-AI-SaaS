import test from 'node:test'
import assert from 'node:assert/strict'
import {
  breakevenMerchants,
  expenseCostTypeLabel,
  expensePaymentStatusLabel,
  hoursFromScaled,
  incomeCanCancel,
  incomeNextStatuses,
  incomeStatusLabel,
  incomeTypeLabel,
  monthlyAmountCents,
  timeLogAmountCents,
  timeLogWorkTypeLabel,
} from './platformFinance.js'

test('time log labels cover all work types', () => {
  assert.equal(timeLogWorkTypeLabel('development'), '开发')
  assert.equal(timeLogWorkTypeLabel('content_creation'), '内容制作')
  assert.equal(timeLogWorkTypeLabel('nope'), '未知类型')
})

test('income labels and transitions match the backend flow', () => {
  assert.equal(incomeTypeLabel('subscription_premium'), '高级套餐')
  assert.equal(incomeStatusLabel('pending_receipt'), '待收款')
  assert.deepEqual(incomeNextStatuses('ordered'), ['confirmed', 'pending_receipt'])
  assert.deepEqual(incomeNextStatuses('refunding'), ['refunded', 'received'])
  assert.deepEqual(incomeNextStatuses('refunded'), [])
  assert.equal(incomeCanCancel('confirmed'), true)
  assert.equal(incomeCanCancel('received'), false)
  assert.equal(incomeCanCancel('refunded'), false)
})

test('expense cost type and payment labels', () => {
  assert.equal(expenseCostTypeLabel('fixed'), '固定成本')
  assert.equal(expenseCostTypeLabel('variable'), '变动成本')
  assert.equal(expenseCostTypeLabel(''), '未分类（v14 前记录）')
  assert.equal(expensePaymentStatusLabel('partial'), '部分付')
})

test('hours scale converts hundredths of an hour for display only', () => {
  assert.equal(hoursFromScaled(150), 1.5)
  assert.equal(hoursFromScaled(33), 0.33)
  assert.equal(hoursFromScaled(null), 0)
})

test('time log amount uses integer fen with half-up rounding', () => {
  assert.equal(timeLogAmountCents(150, 10000), 15000)
  assert.equal(timeLogAmountCents(33, 333), 110)
  assert.equal(timeLogAmountCents(1, 99), 1)
  assert.equal(timeLogAmountCents(0, 10000), null)
  assert.equal(timeLogAmountCents(150, 0), null)
})

test('monthly amount floors annual payments to the fen', () => {
  assert.equal(monthlyAmountCents(238800, 12), 19900)
  assert.equal(monthlyAmountCents(10000, 12), 833)
  assert.equal(monthlyAmountCents(19900, 1), 19900)
  assert.equal(monthlyAmountCents(19900, 2), 9950)
  assert.equal(monthlyAmountCents(19900, 13), null)
  assert.equal(monthlyAmountCents(0, 1), null)
})

test('breakeven rounds up and reports unreachable margins', () => {
  assert.equal(breakevenMerchants(90000, 15000), 6)
  assert.equal(breakevenMerchants(90001, 15000), 7)
  assert.equal(breakevenMerchants(0, 15000), 0)
  assert.equal(breakevenMerchants(90000, 0), null)
  assert.equal(breakevenMerchants(90000, -100), null)
})
