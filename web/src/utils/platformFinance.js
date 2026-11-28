// 平台财务 v14（仅超管）：时间成本、收入台账、MRR 与保本测算的纯函数。
// 金额一律整数分；小时数以百分小时（hours_scaled）存取，页面只做展示换算。

export const TIME_LOG_WORK_TYPES = [
  { value: 'product_planning', label: '产品规划' },
  { value: 'requirement_analysis', label: '需求分析' },
  { value: 'development', label: '开发' },
  { value: 'testing', label: '测试' },
  { value: 'deployment_maintenance', label: '部署维护' },
  { value: 'business_communication', label: '商务沟通' },
  { value: 'content_creation', label: '内容制作' },
]

export const INCOME_TYPES = [
  { value: 'subscription_basic', label: '基础订阅' },
  { value: 'subscription_premium', label: '高级套餐' },
  { value: 'addon_module', label: '增值模块' },
  { value: 'deployment_setup', label: '部署配置' },
  { value: 'ai_usage_package', label: 'AI 用量包' },
  { value: 'other', label: '其他收入' },
]

export const INCOME_STATUSES = [
  { value: 'ordered', label: '已下单' },
  { value: 'confirmed', label: '已确认' },
  { value: 'received', label: '已收款' },
  { value: 'pending_receipt', label: '待收款' },
  { value: 'refunded', label: '已退款' },
  { value: 'refunding', label: '退款中' },
]

export const EXPENSE_COST_TYPES = [
  { value: 'fixed', label: '固定成本' },
  { value: 'variable', label: '变动成本' },
]

export const EXPENSE_PAYMENT_STATUSES = [
  { value: 'paid', label: '已付' },
  { value: 'unpaid', label: '未付' },
  { value: 'partial', label: '部分付' },
]

export const EXPENSE_CATEGORIES_V14 = [
  { value: 'cloud_hosting', label: '云服务 / 主机' },
  { value: 'messaging_api', label: '短信 / 消息 / API' },
  { value: 'labor_contractor', label: '人员 / 外包' },
  { value: 'customer_acquisition_travel', label: '获客 / 差旅' },
  { value: 'equipment', label: '设备' },
  { value: 'ai_services', label: 'AI 服务' },
  { value: 'third_party_services', label: '第三方服务' },
  { value: 'other', label: '其他' },
]

const labelOf = (list, value, fallback) => list.find((item) => item.value === value)?.label || fallback || value || ''

export function timeLogWorkTypeLabel(value) {
  return labelOf(TIME_LOG_WORK_TYPES, value, '未知类型')
}

export function incomeTypeLabel(value) {
  return labelOf(INCOME_TYPES, value, '未知类型')
}

export function incomeStatusLabel(value) {
  return labelOf(INCOME_STATUSES, value, '未知状态')
}

export function expenseCostTypeLabel(value) {
  if (!value) return '未分类（v14 前记录）'
  return labelOf(EXPENSE_COST_TYPES, value, value)
}

export function expensePaymentStatusLabel(value) {
  return labelOf(EXPENSE_PAYMENT_STATUSES, value, value || '—')
}

// 百分小时转展示小时数（只做展示换算，不参与金额计算）。
export function hoursFromScaled(hoursScaled) {
  if (!Number.isFinite(Number(hoursScaled))) return 0
  return Number(hoursScaled) / 100
}

// 服务端金额算式（整数分，四舍五入到分）：(百分小时 × 参考分价 + 50) / 100。
export function timeLogAmountCents(hoursScaled, referenceHourlyValueCents) {
  const scaled = Math.trunc(Number(hoursScaled))
  const value = Math.trunc(Number(referenceHourlyValueCents))
  if (!Number.isSafeInteger(scaled) || !Number.isSafeInteger(value) || scaled < 1 || value < 1) return null
  return Math.floor((scaled * value + 50) / 100)
}

// 订阅月化金额（整数分，年付按十二分之一向下取整到分）。
export function monthlyAmountCents(amountCents, billingPeriodMonths) {
  const amount = Math.trunc(Number(amountCents))
  const months = Math.trunc(Number(billingPeriodMonths))
  if (!Number.isSafeInteger(amount) || !Number.isSafeInteger(months) || amount <= 0 || months < 1 || months > 12) return null
  return Math.floor(amount / months)
}

// 保本商家数：月固定成本 ÷ 单商家贡献毛利，向上取整；贡献毛利 ≤ 0 时返回 null（无法保本）。
export function breakevenMerchants(fixedCostCents, contributionMarginCents) {
  const fixed = Math.trunc(Number(fixedCostCents))
  const margin = Math.trunc(Number(contributionMarginCents))
  if (!Number.isSafeInteger(fixed) || !Number.isSafeInteger(margin)) return null
  if (fixed <= 0) return 0
  if (margin <= 0) return null
  return Math.ceil(fixed / margin)
}

// 收入状态流转（与后端 platformIncomeAllowedTransitions 一致）。
export const INCOME_ALLOWED_TRANSITIONS = {
  ordered: ['confirmed', 'pending_receipt'],
  confirmed: ['received', 'pending_receipt'],
  pending_receipt: ['received'],
  received: ['refunding'],
  refunding: ['refunded', 'received'],
  refunded: [],
}

export function incomeNextStatuses(status) {
  return [...(INCOME_ALLOWED_TRANSITIONS[status] || [])]
}

export function incomeCanCancel(status) {
  return status === 'ordered' || status === 'confirmed' || status === 'pending_receipt'
}
