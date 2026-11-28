import test from 'node:test'
import assert from 'node:assert/strict'
import {
  centsToYuan,
  isRejectReasonValid,
  isThresholdCentsValid,
  refundRecordStatusLabel,
  refundRecordStatusType,
  refundSubmitMessage,
  rejectReasonError,
  thresholdYuanError,
  yuanToCents
} from './refundReview.js'

test('拒绝原因空值或仅空白不提交，255 字内可提交', () => {
  assert.equal(isRejectReasonValid(''), false)
  assert.equal(isRejectReasonValid('   '), false)
  assert.equal(isRejectReasonValid(null), false)
  assert.equal(rejectReasonError(''), '请填写拒绝原因')
  assert.equal(isRejectReasonValid('金额与订单不符'), true)
  assert.equal(isRejectReasonValid('拒'.repeat(255)), true)
  assert.equal(isRejectReasonValid('拒'.repeat(256)), false)
  assert.equal(rejectReasonError('拒'.repeat(256)), '拒绝原因不能超过 255 字')
})

test('阈值元→分换算为整数分，浮点输入不丢分', () => {
  assert.equal(yuanToCents(1000), 100000)
  assert.equal(yuanToCents(0), 0)
  assert.equal(yuanToCents('1000.00'), 100000)
  assert.equal(yuanToCents(0.29), 29)
  assert.equal(yuanToCents(123.45), 12345)
  assert.equal(Number.isInteger(yuanToCents(99.99)), true)
  assert.equal(centsToYuan(100000), '1000.00')
  assert.equal(centsToYuan(0), '0.00')
})

test('阈值元输入校验：允许 0，超过 1000000 元拒绝', () => {
  assert.equal(thresholdYuanError(0), '')
  assert.equal(thresholdYuanError(1000), '')
  assert.equal(thresholdYuanError(1000000), '')
  assert.notEqual(thresholdYuanError(1000000.01), '')
  assert.notEqual(thresholdYuanError(-1), '')
  assert.equal(isThresholdCentsValid(0), true)
  assert.equal(isThresholdCentsValid(100000), true)
  assert.equal(isThresholdCentsValid(100000000), true)
  assert.equal(isThresholdCentsValid(100000001), false)
  assert.equal(isThresholdCentsValid(-1), false)
})

test('review_pending 时提示待复核，不提示退款已确认成功', () => {
  assert.equal(refundSubmitMessage({ review_pending: true }), '已提交待超管复核')
  assert.equal(refundSubmitMessage({ review_pending: true, refunded: false }), '已提交待超管复核')
  assert.equal(refundSubmitMessage({ refunded: true }), '支付宝退款已确认成功')
  assert.equal(refundSubmitMessage({}), '支付宝退款已确认成功')
  assert.equal(refundSubmitMessage(null), '支付宝退款已确认成功')
})

test('退款记录状态含 pending_review / review_rejected 中文映射', () => {
  assert.equal(refundRecordStatusLabel('pending_review'), '待超管复核')
  assert.equal(refundRecordStatusLabel('review_rejected'), '复核已拒绝')
  assert.equal(refundRecordStatusLabel('success'), '渠道已确认')
  assert.equal(refundRecordStatusType('pending_review'), 'warning')
  assert.equal(refundRecordStatusType('review_rejected'), 'danger')
})
