import test from 'node:test'
import assert from 'node:assert/strict'
import {
  isSupportTicketCategoryValid,
  isSupportTicketFormValid,
  isSupportTicketReplyValid,
  supportTicketCategoryLabel,
  supportTicketContentError,
  supportTicketNextStatuses,
  supportTicketReplyError,
  supportTicketStatusActionLabel,
  supportTicketStatusLabel,
  supportTicketStatusType,
  supportTicketTitleError
} from './supportTickets.js'

test('support ticket category labels cover the whole whitelist', () => {
  assert.equal(supportTicketCategoryLabel('order'), '订单')
  assert.equal(supportTicketCategoryLabel('payment'), '支付')
  assert.equal(supportTicketCategoryLabel('subscription'), '订阅')
  assert.equal(supportTicketCategoryLabel('store'), '门店商品')
  assert.equal(supportTicketCategoryLabel('account'), '账号')
  assert.equal(supportTicketCategoryLabel('other'), '其他')
  assert.equal(isSupportTicketCategoryValid('urgent'), false)
  assert.equal(isSupportTicketCategoryValid('order'), true)
})

test('support ticket status labels, types and next statuses mirror the backend machine', () => {
  assert.equal(supportTicketStatusLabel('open'), '待处理')
  assert.equal(supportTicketStatusLabel('in_progress'), '处理中')
  assert.equal(supportTicketStatusLabel('resolved'), '已解决')
  assert.equal(supportTicketStatusLabel('closed'), '已关闭')
  assert.equal(supportTicketStatusType('open'), 'warning')
  assert.equal(supportTicketStatusType('resolved'), 'success')
  assert.equal(supportTicketStatusType('closed'), 'info')
  assert.deepEqual(supportTicketNextStatuses('open'), ['in_progress', 'resolved', 'closed'])
  assert.deepEqual(supportTicketNextStatuses('in_progress'), ['resolved', 'closed'])
  assert.deepEqual(supportTicketNextStatuses('resolved'), ['in_progress', 'closed'])
  assert.deepEqual(supportTicketNextStatuses('closed'), [])
  assert.equal(supportTicketStatusActionLabel('in_progress'), '开始处理')
  assert.equal(supportTicketStatusActionLabel('resolved'), '标记已解决')
  assert.equal(supportTicketStatusActionLabel('closed'), '关闭工单')
})

test('support ticket title/content/reply validation enforces the rune limits', () => {
  assert.equal(supportTicketTitleError(''), '请填写工单标题')
  assert.equal(supportTicketTitleError('x'.repeat(120)), '')
  assert.equal(supportTicketTitleError('x'.repeat(121)), '工单标题不能超过 120 字')
  assert.equal(supportTicketContentError(''), '请填写工单内容')
  assert.equal(supportTicketContentError('x'.repeat(1000)), '')
  assert.equal(supportTicketContentError('x'.repeat(1001)), '工单内容不能超过 1000 字')
  assert.equal(supportTicketReplyError(''), '请填写回复内容')
  assert.equal(supportTicketReplyError('x'.repeat(1000)), '')
  assert.equal(supportTicketReplyError('x'.repeat(1001)), '回复内容不能超过 1000 字')
  assert.equal(isSupportTicketReplyValid('已收到'), true)
  assert.equal(isSupportTicketFormValid({ title: '标题', content: '内容', category: 'order' }), true)
  assert.equal(isSupportTicketFormValid({ title: '标题', content: '内容', category: 'urgent' }), false)
  assert.equal(isSupportTicketFormValid({ title: '', content: '内容', category: 'order' }), false)
})
