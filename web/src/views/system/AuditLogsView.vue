<template>
  <div class="audit-page">
    <section class="page-card hero">
      <div>
        <div class="eyebrow">AUDIT TRAIL</div>
        <h1>风险追踪与操作审计</h1>
        <p>把关键操作翻译成可处理的风险事项：订阅收款、商家冻结、收款审核、退款与结算都能快速追溯。</p>
      </div>
      <el-button type="primary" @click="loadLogs">刷新日志</el-button>
    </section>

    <section class="summary-grid">
      <article v-for="item in summaryCards" :key="item.label" :class="item.tone">
        <span>{{ item.label }}</span>
        <strong>{{ item.value }}</strong>
        <p>{{ item.hint }}</p>
      </article>
    </section>

    <section class="page-card filters">
      <el-input v-model="filters.keyword" clearable placeholder="搜索操作人 / 对象 / 动作" />
      <el-select v-model="filters.action" clearable placeholder="操作类型">
        <el-option label="收款审核" value="merchant_payment_config_review" />
        <el-option label="商家状态调整" value="merchant_status_update" />
        <el-option label="确认订阅收款" value="merchant_subscription_payment_confirm" />
        <el-option label="开通商家订阅" value="merchant_subscription_open" />
        <el-option label="停止商家订阅" value="merchant_subscription_stop" />
        <el-option label="平台退款" value="admin_order_refund" />
        <el-option label="商家退款" value="merchant_order_refund" />
        <el-option label="用户状态调整" value="user_status_update" />
        <el-option label="会员权益调整" value="user_member_adjust" />
      </el-select>
      <el-select v-model="filters.target_type" clearable placeholder="对象类型">
        <el-option label="商家" value="merchant" />
        <el-option label="收款配置" value="merchant_payment_config" />
        <el-option label="订单" value="order" />
        <el-option label="用户" value="user" />
      </el-select>
      <el-select v-model="filters.risk" clearable placeholder="风险等级">
        <el-option label="高风险" value="danger" />
        <el-option label="中风险" value="warning" />
        <el-option label="低风险" value="info" />
      </el-select>
      <el-select v-model="filters.review_status" clearable placeholder="复核状态">
        <el-option label="未复核" value="pending" />
        <el-option label="已复核" value="reviewed" />
      </el-select>
      <el-button type="primary" @click="search">查询</el-button>
      <el-button @click="reset">重置</el-button>
    </section>

    <section class="ops-board">
      <article v-for="item in operationCards" :key="item.title" :class="item.tone">
        <span>{{ item.label }}</span>
        <strong>{{ item.title }}</strong>
        <p>{{ item.text }}</p>
        <el-button size="small" plain @click="item.action">{{ item.button }}</el-button>
      </article>
    </section>

    <section class="page-card">
      <el-table :data="displayLogs" v-loading="loading" stripe>
        <el-table-column label="时间" min-width="170">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column prop="actor_name" label="操作人" min-width="120" />
        <el-table-column label="身份" width="110">
          <template #default="{ row }">{{ actorLabel(row.actor_type) }}</template>
        </el-table-column>
        <el-table-column label="风险" width="110">
          <template #default="{ row }">
            <el-tag :type="riskMeta(row).type">{{ riskMeta(row).label }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" min-width="150">
          <template #default="{ row }">
            <el-tag effect="plain">{{ actionLabel(row.action) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="对象" min-width="180">
          <template #default="{ row }">
            <div class="target-name">{{ targetDisplay(row) }}</div>
            <div class="muted">{{ targetLabel(row.target_type) }} #{{ row.target_id || '-' }}</div>
          </template>
        </el-table-column>
        <el-table-column label="商家ID" width="90">
          <template #default="{ row }">{{ row.merchant_id || '-' }}</template>
        </el-table-column>
        <el-table-column prop="ip" label="IP" min-width="120" />
        <el-table-column label="详情" min-width="240">
          <template #default="{ row }">
            <div class="detail-summary">{{ detailSummary(row) }}</div>
          </template>
        </el-table-column>
        <el-table-column label="复核" width="100">
          <template #default="{ row }">
            <el-tag :type="isReviewed(row) ? 'success' : 'info'">{{ isReviewed(row) ? '已复核' : '未复核' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="处理" width="130" fixed="right">
          <template #default="{ row }">
            <el-button size="small" type="primary" plain @click="openDetail(row)">查看追溯</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="pager">
        <el-pagination
          background
          layout="total, prev, pager, next"
          :total="total"
          :page-size="pagination.page_size"
          v-model:current-page="pagination.page"
          @current-change="loadLogs"
        />
      </div>
    </section>

    <el-drawer v-model="detailVisible" title="审计追溯详情" size="520px">
      <div v-if="activeLog" class="audit-drawer">
        <el-tag :type="riskMeta(activeLog).type">{{ riskMeta(activeLog).label }}</el-tag>
        <h3>{{ actionLabel(activeLog.action) }}</h3>
        <p>{{ detailSummary(activeLog) }}</p>
        <div class="drawer-grid">
          <div><span>操作人</span><strong>{{ activeLog.actor_name || '-' }}</strong></div>
          <div><span>关联对象</span><strong>{{ targetDisplay(activeLog) }}</strong></div>
          <div><span>商家ID</span><strong>{{ activeLog.merchant_id || '-' }}</strong></div>
          <div><span>时间</span><strong>{{ formatTime(activeLog.created_at) }}</strong></div>
          <div><span>复核状态</span><strong>{{ reviewStatusText(activeLog) }}</strong></div>
          <div v-if="activeLog.review_remark"><span>复核备注</span><strong>{{ activeLog.review_remark }}</strong></div>
        </div>
        <div class="detail-fields">
          <div v-for="field in detailFields(activeLog)" :key="field.label">
            <span>{{ field.label }}</span>
            <strong>{{ field.value }}</strong>
          </div>
        </div>
        <el-collapse class="raw-detail">
          <el-collapse-item title="技术原始信息" name="raw">
            <pre>{{ stringifyDetail(activeLog.detail) }}</pre>
          </el-collapse-item>
        </el-collapse>
        <div class="action-advice">
          <span>处理建议</span>
          <strong>{{ actionAdvice(activeLog) }}</strong>
        </div>
        <div class="drawer-actions">
          <el-button type="primary" @click="goRelated(activeLog)">查看相关业务</el-button>
          <el-button type="success" plain @click="markReviewed(activeLog)">标记已复核</el-button>
          <el-button plain @click="detailVisible = false">关闭</el-button>
        </div>
      </div>
    </el-drawer>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { fetchAuditLogs, reviewAuditLog } from '../../api/modules'

const route = useRoute()
const router = useRouter()
const loading = ref(false)
const logs = ref([])
const total = ref(0)
const detailVisible = ref(false)
const activeLog = ref(null)

const filters = reactive({
  keyword: '',
  action: '',
  target_type: '',
  risk: '',
  review_status: ''
})

const pagination = reactive({
  page: 1,
  page_size: 20
})

const actionText = {
  merchant_payment_config_review: '收款审核',
  merchant_status_update: '商家状态调整',
  merchant_subscription_open: '开通商家订阅',
  merchant_subscription_payment_confirm: '确认订阅收款',
  merchant_subscription_stop: '停止商家订阅',
  merchant_follow_up_create: '生成商家跟进',
  merchant_settlement_create: '生成商家结算',
  admin_order_refund: '平台退款',
  merchant_order_refund: '商家退款',
  user_status_update: '用户状态调整',
  user_member_adjust: '会员权益调整'
}

const targetText = {
  merchant: '商家',
  merchant_payment_config: '收款配置',
  merchant_follow_up: '商家跟进',
  merchant_settlement: '商家结算',
  order: '订单',
  user: '用户'
}

const actorText = {
  super_admin: '平台管理员',
  admin: '管理员',
  merchant_admin: '商家'
}

const summaryCards = computed(() => [
  { label: '高风险', value: displayLogs.value.filter((item) => riskMeta(item).type === 'danger').length, hint: '退款、冻结或状态调整类操作', tone: 'danger' },
  { label: '订阅收款', value: displayLogs.value.filter((item) => item.action === 'merchant_subscription_payment_confirm').length, hint: '平台确认商家订阅款', tone: 'primary' },
  { label: '商家变更', value: displayLogs.value.filter((item) => String(item.target_type || '').includes('merchant')).length, hint: '商家状态、收款和订阅调整', tone: 'normal' },
  { label: '当前日志', value: displayLogs.value.length, hint: '当前页筛选结果数量', tone: 'normal' }
])
const displayLogs = computed(() => logs.value.filter((item) => !filters.risk || riskMeta(item).type === filters.risk))
const operationCards = computed(() => [
  {
    label: 'SUBSCRIPTION',
    title: '订阅收款确认',
    text: '订阅付款确认、开通和续期记录需要和订阅营收页互相校验。',
    button: '去订阅营收',
    tone: 'primary',
    action: () => router.push('/admin/subscriptions?status=pending')
  },
  {
    label: 'REFUND',
    title: '退款与售后复盘',
    text: '退款动作需要核对真实支付渠道、订单状态和商家后续跟进。',
    button: '看退款订单',
    tone: 'warning',
    action: () => router.push('/admin/orders?exception=refund')
  },
  {
    label: 'MERCHANT',
    title: '商家状态与收款审核',
    text: '冻结、解冻、收款配置审核会影响商家是否能正常经营。',
    button: '看商家运营',
    tone: 'normal',
    action: () => router.push('/admin/merchants')
  }
])

const loadLogs = async () => {
  loading.value = true
  try {
    const res = await fetchAuditLogs({
      keyword: filters.keyword,
      action: filters.action,
      target_type: filters.target_type,
      review_status: filters.review_status,
      page: pagination.page,
      page_size: pagination.page_size
    })
    logs.value = res.data.list || []
    total.value = res.data.total || 0
  } catch (error) {
    ElMessage.error(error?.response?.data?.message || '审计日志加载失败')
  } finally {
    loading.value = false
  }
}

const search = () => {
  pagination.page = 1
  loadLogs()
}

const reset = () => {
  filters.keyword = ''
  filters.action = ''
  filters.target_type = ''
  filters.risk = ''
  filters.review_status = ''
  pagination.page = 1
  loadLogs()
}

const formatTime = (value) => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '-'
const actionLabel = (value) => actionText[value] || value || '-'
const targetLabel = (value) => targetText[value] || value || '-'
const targetDisplay = (row) => {
  if (row.target_type === 'merchant_follow_up') return `跟进记录 #${row.target_id || '-'}`
  if (row.target_type === 'merchant_settlement') return `结算单 #${row.target_id || '-'}`
  if (row.target_type === 'order') return row.target_name || `订单 #${row.target_id || '-'}`
  if (row.target_type === 'merchant') return row.target_name || `商家 #${row.target_id || '-'}`
  if (row.target_name && !String(row.target_name).includes('_')) return row.target_name
  return `${targetLabel(row.target_type)} #${row.target_id || '-'}`
}
const actorLabel = (value) => actorText[value] || value || '-'
const riskMeta = (row) => {
  if (['admin_order_refund', 'merchant_order_refund', 'merchant_status_update'].includes(row.action)) return { label: '高', type: 'danger' }
  if (['merchant_subscription_payment_confirm', 'merchant_payment_config_review', 'merchant_subscription_stop'].includes(row.action)) return { label: '中', type: 'warning' }
  return { label: '低', type: 'info' }
}

const parseDetail = (value) => {
  if (!value) return {}
  if (typeof value === 'object') return value
  try {
    return JSON.parse(value)
  } catch {
    return { content: value }
  }
}

const detailSummary = (row) => {
  const detail = parseDetail(row.detail)
  if (row.action === 'merchant_subscription_payment_confirm') return `已确认订阅收款 ${formatFen(detail.amount)}，${detailMerchantName(detail) ? `${detailMerchantName(detail)} ` : ''}开通至 ${formatTime(detail.expire_at)}。`
  if (row.action === 'merchant_follow_up_create') return detail.content || '已生成商家跟进记录。'
  if (row.action === 'merchant_payment_config_review') return `收款配置审核结果：${auditStatusLabel(detail.audit_status)}，启用状态：${statusText(detail.status)}。`
  if (row.action === 'merchant_status_update') return `商家状态调整为：${statusText(detail.status)}。`
  if (row.action === 'merchant_settlement_create') return `生成结算单，订单 ${detail.order_count || 0} 笔，净额 ${formatFen(detail.net_amount)}。`
  if (row.action?.includes('refund')) return `退款相关操作，金额 ${formatFen(detail.amount)}，原因：${detail.reason || detail.content || '-'}。`
  return detail.content || detail.remark || stringifyDetail(row.detail).slice(0, 120)
}

const detailFields = (row) => {
  const detail = parseDetail(row?.detail)
  const fields = []
  if (detail.order_no) fields.push({ label: '关联订单', value: detail.order_no })
  if (detailMerchantName(detail)) fields.push({ label: '关联商家', value: detailMerchantName(detail) })
  if (detailPlanName(detail)) fields.push({ label: '订阅套餐', value: detailPlanName(detail) })
  if (detail.amount !== undefined) fields.push({ label: '金额', value: formatFen(detail.amount) })
  if (detail.expire_at) fields.push({ label: '订阅到期', value: formatTime(detail.expire_at) })
  if (detail.payment_mode) fields.push({ label: '收款方式', value: paymentModeLabel(detail.payment_mode) })
  if (detail.result) fields.push({ label: '处理结果', value: resultLabel(detail.result) })
  if (detail.audit_status) fields.push({ label: '审核结果', value: auditStatusLabel(detail.audit_status) })
  if (detail.status) fields.push({ label: '状态', value: statusText(detail.status) })
  if (detail.reason) fields.push({ label: '原因', value: detail.reason })
  if (detail.remark) fields.push({ label: '备注', value: detail.remark })
  if (detail.content) fields.push({ label: '内容', value: detail.content })
  if (detail.source) fields.push({ label: '来源', value: sourceLabel(detail.source) })
  if (fields.length) return fields
  return [{ label: '记录说明', value: detailSummary(row) || '暂无可读详情' }]
}

const actionAdvice = (row) => {
  if (row.action === 'merchant_subscription_payment_confirm') return '到订阅营收页核对该商家订阅订单是否已入账，并确认到期时间是否正确。'
  if (row.action === 'merchant_payment_config_review') return '进入商家详情核对收款配置、审核备注和启用状态，避免影响顾客付款。'
  if (row.action === 'merchant_status_update') return '进入商家运营页查看冻结或解冻原因，并检查是否需要补充跟进记录。'
  if (row.action?.includes('refund')) return '进入订单财务核对退款记录、真实渠道退款和商家结算抵扣。'
  if (row.action === 'merchant_settlement_create') return '进入订单财务或商家详情核对结算订单范围和净额。'
  return '保留审计记录，用于后续复盘、排查和责任追踪。'
}

const goRelated = (row) => {
  if (row.action === 'merchant_subscription_payment_confirm' || row.action?.includes('subscription')) {
    router.push({ path: '/admin/subscriptions', query: { keyword: row.target_name || row.target_id || '' } })
    return
  }
  if (row.action?.includes('refund') || row.target_type === 'order') {
    router.push({ path: '/admin/orders', query: { keyword: row.target_name || row.target_id || '' } })
    return
  }
  if (row.merchant_id || row.target_type === 'merchant') {
    router.push(row.merchant_id || row.target_id ? `/admin/merchants/${row.merchant_id || row.target_id}` : '/admin/merchants')
    return
  }
  router.push('/admin/dashboard')
}

const openDetail = (row) => {
  activeLog.value = row
  detailVisible.value = true
}

const isReviewed = (row) => row?.review_status === 'reviewed' || !!row?.reviewed_at
const reviewStatusText = (row) => isReviewed(row) ? `已复核 ${formatTime(row.reviewed_at)}` : '未复核'
const markReviewed = async (row) => {
  if (!row?.id || isReviewed(row)) {
    detailVisible.value = false
    return
  }
  const res = await reviewAuditLog(row.id, { remark: '平台已复核该审计记录' })
  const updated = res.data?.log
  if (updated) {
    const index = logs.value.findIndex((item) => item.id === row.id)
    if (index >= 0) logs.value[index] = updated
    activeLog.value = updated
  }
  ElMessage.success('已标记为复核')
}

const formatFen = (value) => {
  if (value === undefined || value === null || value === '') return '-'
  return `¥${(Number(value || 0) / 100).toFixed(2)}`
}

const detailMerchantName = (detail) => detail?.merchant?.name || detail?.merchant_name || ''
const detailPlanName = (detail) => detail?.plan?.name || detail?.merchant_plan?.name || detail?.plan_name || ''
const statusText = (value) => ({
  active: '正常',
  inactive: '未启用',
  disabled: '已停用',
  frozen: '已冻结',
  enabled: '已启用',
  pending: '待处理',
  paid: '已支付',
  approved: '已通过',
  rejected: '已驳回'
}[value] || value || '-')
const auditStatusLabel = (value) => ({
  pending: '待审核',
  approved: '审核通过',
  rejected: '审核驳回'
}[value] || value || '-')
const paymentModeLabel = (value) => ({
  platform_qr: '平台收款码',
  merchant_qr: '商家收款码',
  merchant_direct_mvp: '商家收款码模式',
  admin_manual_confirm: '平台人工确认'
}[value] || value || '-')
const sourceLabel = (value) => ({
  admin_manual_confirm: '平台人工确认',
  order_exception: '订单异常巡检',
  merchant_ops: '商家运营'
}[value] || value || '-')
const resultLabel = (value) => ({
  subscription_opened: '订阅已开通或续期',
  payment_config_approved: '收款配置已启用',
  reviewed: '已复核'
}[value] || value || '-')

const stringifyDetail = (value) => {
  if (!value) return '-'
  if (typeof value === 'object') return JSON.stringify(value, null, 2)
  try {
    return JSON.stringify(JSON.parse(value), null, 2)
  } catch {
    return value
  }
}

onMounted(() => {
  filters.keyword = route.query.keyword || ''
  filters.action = route.query.action || ''
  filters.target_type = route.query.target_type || ''
  filters.risk = route.query.risk || ''
  filters.review_status = route.query.review_status || ''
  loadLogs()
})
</script>

<style scoped>
.audit-page {
  display: grid;
  gap: 18px;
}

.hero {
  display: flex;
  justify-content: space-between;
  gap: 18px;
  align-items: center;
  padding: 26px;
}

.eyebrow {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.18em;
}

h1 {
  margin: 6px 0 10px;
  font-size: 28px;
}

p,
.muted {
  color: #64748b;
}

.filters {
  display: grid;
  grid-template-columns: minmax(220px, 1fr) 180px 180px 150px 150px auto auto;
  gap: 10px;
  align-items: center;
  padding: 16px;
}

.ops-board,
.summary-grid {
  display: grid;
  gap: 14px;
}

.summary-grid {
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.ops-board {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.summary-grid article {
  min-height: 112px;
  padding: 16px;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  background: #ffffff;
}

.summary-grid article.danger {
  border-color: #fecaca;
  background: #fef2f2;
}

.summary-grid article.primary {
  border-color: #bfdbfe;
  background: #eff6ff;
}

.ops-board article {
  display: grid;
  gap: 8px;
  min-height: 150px;
  padding: 16px;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  background: #ffffff;
  align-content: start;
}

.ops-board article.primary {
  border-color: #bfdbfe;
  background: #eff6ff;
}

.ops-board article.warning {
  border-color: #fed7aa;
  background: #fff7ed;
}

.summary-grid span,
.summary-grid strong,
.summary-grid p,
.ops-board span,
.ops-board strong,
.ops-board p {
  display: block;
}

.summary-grid span,
.ops-board span {
  color: #64748b;
  font-size: 12px;
  font-weight: 800;
}

.summary-grid strong {
  margin-top: 8px;
  color: #0f172a;
  font-size: 26px;
}

.ops-board strong {
  color: #0f172a;
  font-size: 17px;
}

.summary-grid p,
.ops-board p {
  margin: 8px 0 0;
  color: #64748b;
  line-height: 1.5;
}

.target-name {
  font-weight: 800;
}

.detail-summary {
  color: #334155;
  line-height: 1.6;
}

.audit-drawer h3 {
  margin: 12px 0 8px;
}

.audit-drawer p {
  line-height: 1.7;
}

.drawer-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  margin: 16px 0;
}

.drawer-grid div {
  padding: 12px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #f8fafc;
}

.drawer-grid span,
.drawer-grid strong {
  display: block;
}

.drawer-grid span {
  color: #64748b;
  font-size: 12px;
}

.detail-fields {
  display: grid;
  gap: 10px;
  margin-bottom: 16px;
}

.detail-fields div {
  padding: 12px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #ffffff;
}

.detail-fields span,
.detail-fields strong {
  display: block;
}

.detail-fields span {
  color: #64748b;
  font-size: 12px;
}

.detail-fields strong {
  margin-top: 6px;
  color: #0f2747;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
}

.action-advice {
  padding: 14px;
  border: 1px solid #bfdbfe;
  border-radius: 8px;
  background: #eff6ff;
}

.raw-detail {
  margin-bottom: 16px;
}

.raw-detail pre {
  max-height: 220px;
  margin: 0;
  overflow: auto;
  color: #334155;
  white-space: pre-wrap;
  word-break: break-word;
}

.action-advice span,
.action-advice strong {
  display: block;
}

.action-advice span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
}

.action-advice strong {
  margin-top: 8px;
  color: #0f2747;
  line-height: 1.6;
}

.drawer-actions {
  display: flex;
  gap: 10px;
  margin-top: 16px;
}

.pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

@media (max-width: 900px) {
  .hero,
  .filters,
  .ops-board,
  .summary-grid,
  .drawer-grid {
    grid-template-columns: 1fr;
    display: grid;
  }
}
</style>
