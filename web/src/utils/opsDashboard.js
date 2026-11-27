// P3 经营看板纯函数：偏差、标签、状态流转。金额格式化复用
// aiCostControl.js 的 formatCents（分转元），本文件不另造口径。

// opsDeviationPercent returns the completion rate of actual vs target as a
// rounded integer percent, or null when no target was set. Over 100 is
// allowed (over-achieved) and shown as-is.
export function opsDeviationPercent(actual, target) {
  const a = Number(actual) || 0
  const t = Number(target) || 0
  if (t <= 0) return null
  return Math.round((a * 100) / t)
}

// opsDeviationText renders the completion rate for the postmortem table.
export function opsDeviationText(actual, target) {
  const pct = opsDeviationPercent(actual, target)
  if (pct === null) return '未设目标'
  return `完成率 ${pct}%`
}

// opsDeviationType maps the rate to an Element Plus tag type: green when
// the target is met, orange when close (60-99%), red when far below.
export function opsDeviationType(actual, target) {
  const pct = opsDeviationPercent(actual, target)
  if (pct === null) return 'info'
  if (pct >= 100) return 'success'
  if (pct >= 60) return 'warning'
  return 'danger'
}

// opsProductStatusLabel maps a product-health status to its display name.
export function opsProductStatusLabel(status) {
  switch (status) {
    case 'bestseller':
      return '畅销'
    case 'normal':
      return '正常'
    case 'slow':
      return '滞销'
    case 'unsold':
      return '未动销'
    default:
      return '未知'
  }
}

// opsProductStatusType maps a product-health status to a tag type.
export function opsProductStatusType(status) {
  switch (status) {
    case 'bestseller':
      return 'success'
    case 'normal':
      return ''
    case 'slow':
      return 'warning'
    case 'unsold':
      return 'danger'
    default:
      return 'info'
  }
}

// opsSuggestionStatusLabel maps a suggestion-execution status to its name.
export function opsSuggestionStatusLabel(status) {
  switch (status) {
    case 'pending':
      return '待处理'
    case 'adopted':
      return '已采纳'
    case 'ignored':
      return '已忽略'
    case 'executed':
      return '已执行'
    default:
      return '未知'
  }
}

// opsNextSuggestionActions returns the legal next actions for a suggestion
// in this status (pending → adopt/ignore, adopted → execute, terminal
// states → none), mirroring the backend state machine.
export function opsNextSuggestionActions(status) {
  switch (status) {
    case 'pending':
      return [
        { status: 'adopted', label: '采纳' },
        { status: 'ignored', label: '忽略' },
      ]
    case 'adopted':
      return [{ status: 'executed', label: '标记已执行' }]
    default:
      return []
  }
}

// opsDormantTierLabel maps a dormant tier key to its display name.
export function opsDormantTierLabel(tier) {
  switch (tier) {
    case 'light':
      return '轻度沉睡'
    case 'medium':
      return '中度沉睡'
    case 'deep':
      return '深度沉睡'
    default:
      return '未知'
  }
}

// opsTodayDate returns today's date (Asia/Shanghai) as YYYY-MM-DD for the
// daily summary date input default.
export function opsTodayDate() {
  return new Date().toLocaleDateString('en-CA', { timeZone: 'Asia/Shanghai' })
}
