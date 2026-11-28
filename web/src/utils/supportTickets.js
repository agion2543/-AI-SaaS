// Support tickets (P3-2, v18): lightweight merchant→platform service
// requests. Single latest reply only — no multi-round conversation, no
// priority. Status machine (super-admin only): open → in_progress →
// resolved → closed; open may jump straight to resolved/closed, resolved
// may reopen to in_progress; closed is terminal.
export const supportTicketCategories = ['order', 'payment', 'subscription', 'store', 'account', 'other']

export const supportTicketCategoryLabel = (value) => ({
  order: '订单',
  payment: '支付',
  subscription: '订阅',
  store: '门店商品',
  account: '账号',
  other: '其他'
}[value] || value || '-')

export const supportTicketStatusLabel = (value) => ({
  open: '待处理',
  in_progress: '处理中',
  resolved: '已解决',
  closed: '已关闭'
}[value] || value || '-')

export const supportTicketStatusType = (value) => ({
  open: 'warning',
  in_progress: '',
  resolved: 'success',
  closed: 'info'
}[value] || 'info')

// Next statuses the super-admin may move to from the current status.
// Mirrors the backend transition map; the backend still enforces it.
export const supportTicketNextStatuses = (value) => ({
  open: ['in_progress', 'resolved', 'closed'],
  in_progress: ['resolved', 'closed'],
  resolved: ['in_progress', 'closed'],
  closed: []
}[value] || [])

export const supportTicketStatusActionLabel = (value) => ({
  in_progress: '开始处理',
  resolved: '标记已解决',
  closed: '关闭工单'
}[value] || value || '')

export const isSupportTicketCategoryValid = (value) => supportTicketCategories.includes(value)

export const supportTicketTitleError = (value) => {
  const text = String(value || '').trim()
  if (!text) return '请填写工单标题'
  if ([...text].length > 120) return '工单标题不能超过 120 字'
  return ''
}

export const supportTicketContentError = (value) => {
  const text = String(value || '').trim()
  if (!text) return '请填写工单内容'
  if ([...text].length > 1000) return '工单内容不能超过 1000 字'
  return ''
}

export const supportTicketReplyError = (value) => {
  const text = String(value || '').trim()
  if (!text) return '请填写回复内容'
  if ([...text].length > 1000) return '回复内容不能超过 1000 字'
  return ''
}

export const isSupportTicketFormValid = (form = {}) =>
  supportTicketTitleError(form.title) === '' &&
  supportTicketContentError(form.content) === '' &&
  isSupportTicketCategoryValid(form.category)

export const isSupportTicketReplyValid = (value) => supportTicketReplyError(value) === ''
