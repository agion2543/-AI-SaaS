// AI 成本控制（P1 MVP）超管端纯函数：标签、金额换算与展示口径。
// 口径红线：估算成本与供应商实际账单严格分开；估算永远标注“估算”，
// 实际账单只来自超管手工录入；门店维度无归属时只显示“未指定门店”，不虚构。

export const OVER_LIMIT_ACTIONS = [
  { value: 'stop', label: '停止 AI 调用' },
  { value: 'downgrade', label: '降级到低成本模型' },
  { value: 'restrict_expensive', label: '限制高成本功能' },
  { value: 'manual_review', label: '人工审核后恢复' },
]

export const AI_USAGE_DIMENSIONS = [
  { value: 'merchant', label: '按商家' },
  { value: 'store', label: '按门店' },
  { value: 'model', label: '按模型' },
  { value: 'scenario', label: '按功能' },
]

export const BUDGET_ALERT_THRESHOLDS = [70, 90, 100]

// 后端对高成本功能的约定（restrict_expensive 策略下被拒绝的场景）。
const HIGH_COST_SCENARIO_LABELS = {
  referral_multi_copy: '多文案批量生成',
  share_campaign_review: '分享活动复盘',
}

export function overLimitActionLabel(action) {
  return OVER_LIMIT_ACTIONS.find((item) => item.value === action)?.label || action || '未设置'
}

export function periodTypeLabel(periodType) {
  if (periodType === 'daily') return '每日'
  if (periodType === 'monthly') return '每月'
  return periodType || '未注明'
}

export function scenarioLabel(scenario) {
  const value = String(scenario || '').trim()
  if (!value) return '未注明'
  return HIGH_COST_SCENARIO_LABELS[value] || value
}

export function merchantDisplayName(merchantId, merchantName) {
  const name = String(merchantName || '').trim()
  if (name) return name
  if (merchantId === 0 || merchantId === '0') return '全局默认'
  return `商家 #${merchantId}`
}

export function budgetMerchantLabel(merchantId, merchantName) {
  if (merchantId === 0 || merchantId === '0') return '全局默认（无单独预算的商家）'
  return merchantDisplayName(merchantId, merchantName)
}

// 门店维度：后端 store_id 为 NULL 时统计归入 0，页面照实显示“未指定门店”；
// 有门店 id 且后端给了真名时优先显示真名，无名时只显示编号，不虚构门店名。
// 自 2026-11-28 起新写入的调用按默认门店归属，历史 NULL 行仍显示“未指定门店”。
export function storeDisplayName(storeId, storeName) {
  if (!storeId) return '未指定门店'
  const name = String(storeName || '').trim()
  if (name) return name
  return `门店 #${storeId}`
}

export function formatCents(cents) {
  if (cents === null || cents === undefined || !Number.isFinite(Number(cents))) return '—'
  return new Intl.NumberFormat('zh-CN', { style: 'currency', currency: 'CNY' }).format(Number(cents) / 100)
}

// 每百万 Token 单价（后端单位：分 / 百万 Token）→ “¥x.xx / 百万 Token”。
export function formatPricePerMillion(centsPerMillion) {
  if (centsPerMillion === null || centsPerMillion === undefined || !Number.isFinite(Number(centsPerMillion))) return '—'
  return `¥${(Number(centsPerMillion) / 100).toFixed(2)} / 百万 Token`
}

const MAX_CENTS = 100000000000

// 表单输入的元 → 后端单位分；非法输入返回 null，由调用方提示而非静默截断。
export function yuanToCents(yuan) {
  if (yuan === null || yuan === undefined || yuan === '') return null
  const value = Number(yuan)
  if (!Number.isFinite(value) || value < 0) return null
  const cents = Math.round(value * 100)
  if (!Number.isSafeInteger(cents) || cents > MAX_CENTS) return null
  return cents
}

export function centsToYuan(cents) {
  if (cents === null || cents === undefined || !Number.isFinite(Number(cents))) return null
  return Number(cents) / 100
}

export function newUUID() {
  if (globalThis.crypto?.randomUUID) return globalThis.crypto.randomUUID()
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (char) => {
    const value = Math.floor(Math.random() * 16)
    return (char === 'x' ? value : (value & 0x3) | 0x8).toString(16)
  })
}

// “YYYY-MM-DD HH:MM:SS”本地时间，用于模型价格生效时间。
export function localDateTimeString(date) {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hour = String(date.getHours()).padStart(2, '0')
  const minute = String(date.getMinutes()).padStart(2, '0')
  const second = String(date.getSeconds()).padStart(2, '0')
  return `${year}-${month}-${day} ${hour}:${minute}:${second}`
}
