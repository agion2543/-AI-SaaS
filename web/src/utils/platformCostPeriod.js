const ISO_DATE = /^\d{4}-\d{2}-\d{2}$/
const MILLISECONDS_PER_DAY = 24 * 60 * 60 * 1000

function parseISODate(value) {
  if (typeof value !== 'string' || !ISO_DATE.test(value)) return null
  const timestamp = Date.parse(`${value}T00:00:00.000Z`)
  if (!Number.isFinite(timestamp) || new Date(timestamp).toISOString().slice(0, 10) !== value) return null
  return timestamp
}

export function getPlatformCostPeriodValidationError(startDate, endDate, todayDate) {
  if (!startDate || !endDate) return '请选择完整的开始日期和结束日期。'

  const start = parseISODate(startDate)
  const end = parseISODate(endDate)
  const today = parseISODate(todayDate)
  if (start === null || end === null || today === null) return '日期必须使用有效的 YYYY-MM-DD 格式。'
  if (end < start) return '结束日期不能早于开始日期。'
  if (end > today) return '结束日期不能晚于今天。'

  const inclusiveDays = (end - start) / MILLISECONDS_PER_DAY + 1
  if (inclusiveDays > 366) return '统计区间最多为 366 天。'
  return ''
}
