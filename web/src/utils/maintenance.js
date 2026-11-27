// 公告与维护计划（P2）前端纯函数：标签、可见性与拦截判断。
// 口径红线：公告可见性与后端 ListVisibleAnnouncements 的规则一致
// （已发布 + 发布时间已到 + 生效期内）；维护拦截与后端
// MaintenanceBlocksModule 的规则一致（active 状态 + 未过窗口结束时间
// + 受影响模块含 all 或该模块）；在途订单不受影响，只拦新下单。

export const ANNOUNCEMENT_TYPES = [
  { value: 'system_update', label: '系统更新' },
  { value: 'promo', label: '活动促销' },
  { value: 'price_change', label: '价格变动' },
]

export const ANNOUNCEMENT_STATUSES = [
  { value: 'draft', label: '草稿' },
  { value: 'published', label: '已发布' },
  { value: 'archived', label: '已归档' },
]

export const MAINTENANCE_STATUSES = [
  { value: 'draft', label: '草稿' },
  { value: 'scheduled', label: '已排期' },
  { value: 'active', label: '进行中' },
  { value: 'completed', label: '已完成' },
  { value: 'cancelled', label: '已取消' },
]

export function announcementTypeLabel(type) {
  return ANNOUNCEMENT_TYPES.find((item) => item.value === type)?.label || type || '未注明'
}

export function announcementStatusLabel(status) {
  return ANNOUNCEMENT_STATUSES.find((item) => item.value === status)?.label || status || '未注明'
}

export function maintenanceStatusLabel(status) {
  return MAINTENANCE_STATUSES.find((item) => item.value === status)?.label || status || '未注明'
}

// 受影响模块是逗号分隔列表，"all" 表示全部模块。
export function maintenanceModulesLabel(modules) {
  const list = String(modules || '')
    .split(',')
    .map((item) => item.trim().toLowerCase())
    .filter(Boolean)
  if (list.includes('all')) return '全部模块'
  if (!list.length) return '未注明'
  return list.join('、')
}

function parseTime(value) {
  if (!value) return null
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return null
  return parsed
}

// 与后端 ListVisibleAnnouncements 一致：已发布 + 发布时间（publish_at，
// 为空时取 published_at）已到 + 生效期内。
export function announcementVisibleAt(row, now = new Date()) {
  if (!row || row.status !== 'published') return false
  const publishAt = parseTime(row.publish_at) || parseTime(row.published_at)
  if (publishAt && publishAt > now) return false
  const effectiveFrom = parseTime(row.effective_from)
  if (effectiveFrom && effectiveFrom > now) return false
  const effectiveTo = parseTime(row.effective_to)
  if (effectiveTo && effectiveTo < now) return false
  return true
}

// 与后端 MaintenanceBlocksModule 一致：active 状态 + 未过窗口结束时间 +
// 受影响模块含 all 或该模块。窗口已过的 active 计划不拦截（后端同样处理）。
export function maintenanceBlocksModule(plan, module, now = new Date()) {
  if (!plan || plan.status !== 'active') return false
  const windowEnd = parseTime(plan.window_end)
  if (windowEnd && windowEnd < now) return false
  const affected = String(plan.affected_modules || '')
    .split(',')
    .map((item) => item.trim().toLowerCase())
    .filter(Boolean)
  const target = String(module || '').trim().toLowerCase()
  return affected.includes('all') || (Boolean(target) && affected.includes(target))
}

// 维护进行中时给顾客看的提示：含预计恢复时间；在途订单不受影响。
export function maintenanceBlockText(plan) {
  if (!plan) return ''
  const windowEnd = parseTime(plan.window_end)
  const when = windowEnd
    ? `${windowEnd.getFullYear()}-${String(windowEnd.getMonth() + 1).padStart(2, '0')}-${String(windowEnd.getDate()).padStart(2, '0')} ${String(windowEnd.getHours()).padStart(2, '0')}:${String(windowEnd.getMinutes()).padStart(2, '0')}`
    : '维护结束后'
  return `平台维护中，暂停下单，预计 ${when} 前恢复；已下单的订单不受影响`
}
