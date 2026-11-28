// 退款复核（M2）纯逻辑：金额一律整数分，禁止浮点参与财务计算。
// 视图（RefundReviewView.vue）与订单页（OrdersView.vue）共用，便于 node --test 覆盖。

export const REFUND_REVIEW_REJECT_REASON_MAX = 255
export const REFUND_REVIEW_THRESHOLD_MAX_CENTS = 100000000
export const REFUND_REVIEW_THRESHOLD_MAX_YUAN = 1000000

// 元 → 分：先 ×100 再四舍五入取整，避免 0.29*100=28.999… 这类浮点误差。
export const yuanToCents = (yuan) => Math.round(Number(yuan || 0) * 100)

export const centsToYuan = (cents) => (Number(cents || 0) / 100).toFixed(2)

// 拒绝原因校验：空（仅空白也算空）不提交；超过 255 字后端会 400。
export const rejectReasonError = (reason) => {
  const value = String(reason ?? '').trim()
  if (!value) return '请填写拒绝原因'
  if (value.length > REFUND_REVIEW_REJECT_REASON_MAX) return '拒绝原因不能超过 255 字'
  return ''
}
export const isRejectReasonValid = (reason) => rejectReasonError(reason) === ''

// 阈值（元）输入校验：允许 0（= 全部退款都要复核），上限 1000000 元。
export const thresholdYuanError = (yuan) => {
  const value = Number(yuan)
  if (!Number.isFinite(value) || value < 0) return '阈值不能小于 0 元'
  if (value > REFUND_REVIEW_THRESHOLD_MAX_YUAN) return '阈值不能超过 1000000 元'
  return ''
}

// 退款阈值是否越界（分）：后端 PUT 失败文案为
// "refund review threshold must be between 0 and 100000000 cents"。
export const isThresholdCentsValid = (cents) =>
  Number.isInteger(cents) && cents >= 0 && cents <= REFUND_REVIEW_THRESHOLD_MAX_CENTS

// 管理员退款提交后的提示：超阈值时后端返回 review_pending=true，
// 此时退款尚未执行、只是已提交待超管复核，绝不能提示“已确认成功”。
export const refundSubmitMessage = (data) =>
  data?.review_pending ? '已提交待超管复核' : '支付宝退款已确认成功'

// 退款记录状态中文映射：含 M2 新增的 pending_review / review_rejected。
export const refundRecordStatusLabel = (status) => ({
  success: '渠道已确认',
  processing: '处理中',
  unknown: '结果待核查',
  failed: '渠道明确拒绝',
  pending_review: '待超管复核',
  review_rejected: '复核已拒绝'
}[status] || status || '-')
export const refundRecordStatusType = (status) => ({
  success: 'success',
  processing: 'warning',
  unknown: 'danger',
  failed: 'info',
  pending_review: 'warning',
  review_rejected: 'danger'
}[status] || 'info')

export const operatorRoleLabel = (role) =>
  role === 'admin' ? '平台后台' : role === 'merchant' ? '商家端' : role || '—'

export const refundStatusLabel = (status) => ({
  none: '未退款',
  partial: '部分退款',
  full: '全额退款',
  processing: '退款待核查'
}[status] || '未退款')
