<template>
  <div class="finance-page">
    <section class="page-card toolbar">
      <div>
        <h2 class="page-title">订单财务</h2>
        <p class="page-desc">统一查看平台订阅收入、顾客扫码交易流水、支付记录和退款售后记录。</p>
      </div>
      <div class="toolbar-actions">
        <el-button @click="exportOrdersCsv">导出订单</el-button>
        <el-button type="primary" :loading="loading" @click="load">刷新</el-button>
      </div>
    </section>

    <section class="finance-tips page-card">
      <div>
        <strong>资金流建议</strong>
        <span>顾客扫码点单款建议直接进入商家收款账户，平台只记录订单与流水，降低代收代付合规压力。</span>
      </div>
      <div>
        <strong>平台收入</strong>
        <span>平台主要收取商家 SaaS 订阅费，顾客交易流水单独核对，不默认并入平台可支配收入。</span>
      </div>
      <div>
        <strong>售后记录</strong>
        <span>当前退款为系统财务留痕。真实退款仍需由实际收款账户或支付渠道执行。</span>
      </div>
    </section>

    <section class="page-card filters-card">
      <el-date-picker
        v-model="filters.dateRange"
        type="daterange"
        unlink-panels
        range-separator="至"
        start-placeholder="开始日期"
        end-placeholder="结束日期"
        value-format="YYYY-MM-DD"
      />
      <el-input v-model="filters.keyword" clearable placeholder="搜索订单号、商家、手机号" />
      <el-select v-model="filters.orderType" clearable placeholder="全部类型">
        <el-option label="商家订阅" value="merchant_subscription" />
        <el-option label="扫码点单" value="store_order" />
        <el-option label="用户会员" value="user_membership" />
      </el-select>
      <el-select v-model="filters.status" clearable placeholder="全部状态">
        <el-option label="待支付" value="pending" />
        <el-option label="已支付" value="paid" />
        <el-option label="待接单" value="received" />
        <el-option label="已接单" value="accepted" />
        <el-option label="已完成" value="completed" />
        <el-option label="已关闭" value="closed" />
      </el-select>
      <el-button @click="resetFilters">重置</el-button>
    </section>

    <section class="finance-summary">
      <div class="summary-card">
        <span>平台订阅收入</span>
        <strong>{{ formatMoney(platformSubscriptionIncome) }}</strong>
        <small>商家月付/年付已支付订单</small>
      </div>
      <div class="summary-card">
        <span>顾客交易流水</span>
        <strong>{{ formatMoney(customerTradeIncome) }}</strong>
        <small>顾客扫码点单已支付金额</small>
      </div>
      <div class="summary-card">
        <span>售后退款</span>
        <strong>{{ formatMoney(totalRefunded) }}</strong>
        <small>系统记录的退款金额</small>
      </div>
      <div class="summary-card highlight">
        <span>顾客交易净额</span>
        <strong>{{ formatMoney(customerTradeNet) }}</strong>
        <small>交易流水减退款，便于商家对账</small>
      </div>
    </section>

    <section class="page-card">
      <el-tabs>
        <el-tab-pane label="订单列表">
          <el-table :data="filteredOrders" empty-text="暂无订单">
            <el-table-column prop="order_no" label="订单号" min-width="190" />
            <el-table-column label="订单类型" width="140">
              <template #default="{ row }">{{ orderTypeLabel(row.order_type) }}</template>
            </el-table-column>
            <el-table-column label="商家 / 用户" min-width="180">
              <template #default="{ row }">{{ row.merchant?.name || row.user?.display_name || row.user?.phone || '-' }}</template>
            </el-table-column>
            <el-table-column label="金额" width="120">
              <template #default="{ row }">{{ formatMoney(row.total_amount || row.amount) }}</template>
            </el-table-column>
            <el-table-column label="已退款" width="120">
              <template #default="{ row }">{{ formatMoney(row.refunded_amount) }}</template>
            </el-table-column>
            <el-table-column label="退款状态" width="120">
              <template #default="{ row }">
                <el-tag :type="refundType(row.refund_status)">{{ refundLabel(row.refund_status) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="状态" width="120">
              <template #default="{ row }">
                <el-tag :type="statusType(row.status)">{{ statusLabel(row.status) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="payment_channel" label="支付方式" width="120" />
            <el-table-column label="支付时间" min-width="170">
              <template #default="{ row }">{{ formatTime(row.paid_at) }}</template>
            </el-table-column>
            <el-table-column label="售后" width="120" fixed="right">
              <template #default="{ row }">
                <el-button size="small" :disabled="!canRefund(row)" @click="openRefund(row)">退款</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>

        <el-tab-pane label="支付记录">
          <el-table :data="filteredPayments" empty-text="暂无支付记录">
            <el-table-column prop="transaction_no" label="交易号" min-width="220" />
            <el-table-column label="关联订单" min-width="180">
              <template #default="{ row }">{{ row.order?.order_no || row.order_id }}</template>
            </el-table-column>
            <el-table-column label="金额" width="120">
              <template #default="{ row }">{{ formatMoney(row.amount) }}</template>
            </el-table-column>
            <el-table-column prop="payment_channel" label="渠道" width="120" />
            <el-table-column prop="status" label="状态" width="100" />
            <el-table-column label="创建时间" min-width="170">
              <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
            </el-table-column>
            <el-table-column label="售后" width="120" fixed="right">
              <template #default="{ row }">
                <el-button size="small" :disabled="!canRefund(row.order || {})" @click="openRefund(row.order)">
                  退款
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>

        <el-tab-pane label="退款记录">
          <el-table :data="filteredRefunds" empty-text="暂无退款记录">
            <el-table-column prop="refund_no" label="退款单号" min-width="190" />
            <el-table-column label="关联订单" min-width="190">
              <template #default="{ row }">{{ row.order?.order_no || row.order_id }}</template>
            </el-table-column>
            <el-table-column label="商家 / 门店" min-width="180">
              <template #default="{ row }">{{ row.order?.merchant?.name || '-' }} / {{ row.order?.store?.name || '-' }}</template>
            </el-table-column>
            <el-table-column label="退款金额" width="120">
              <template #default="{ row }">{{ formatMoney(row.amount) }}</template>
            </el-table-column>
            <el-table-column prop="reason" label="退款原因" min-width="180" show-overflow-tooltip />
            <el-table-column label="操作方" width="120">
              <template #default="{ row }">{{ row.operator_role === 'admin' ? '平台后台' : '商家端' }}</template>
            </el-table-column>
            <el-table-column prop="status" label="状态" width="100" />
            <el-table-column label="创建时间" min-width="170">
              <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
            </el-table-column>
          </el-table>
        </el-tab-pane>
      </el-tabs>
    </section>

    <el-dialog v-model="refundDialogVisible" title="订单售后退款" width="520px">
      <el-alert
        class="refund-alert"
        type="warning"
        show-icon
        :closable="false"
        title="当前操作会创建财务退款记录。真实退款仍需在实际支付渠道或商家收款账户中同步处理。"
      />
      <el-form label-width="100px">
        <el-form-item label="订单号">
          <el-input :model-value="refundOrder?.order_no || '-'" disabled />
        </el-form-item>
        <el-form-item label="可退金额">
          <el-input :model-value="formatMoney(refundableAmount)" disabled />
        </el-form-item>
        <el-form-item label="退款方式">
          <el-radio-group v-model="refundForm.full">
            <el-radio-button :label="true">全额退款</el-radio-button>
            <el-radio-button :label="false">自定义金额</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="!refundForm.full" label="退款金额">
          <el-input-number v-model="refundForm.amount_yuan" :min="0.01" :max="refundableAmount / 100" :precision="2" />
        </el-form-item>
        <el-form-item label="退款原因">
          <el-input v-model="refundForm.reason" type="textarea" :rows="3" maxlength="120" show-word-limit />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="refundDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="refundSaving" @click="submitRefund">确认退款</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { fetchOrders, fetchPayments, fetchRefunds, refundAdminOrder } from '../../api/modules'

const orders = ref([])
const payments = ref([])
const refunds = ref([])
const loading = ref(false)
const refundDialogVisible = ref(false)
const refundSaving = ref(false)
const refundOrder = ref(null)
const refundForm = reactive({ full: true, amount_yuan: 0, reason: '' })
const filters = reactive({ dateRange: [], keyword: '', orderType: '', status: '' })

const paidStoreStatuses = ['received', 'accepted', 'completed', 'closed']
const filteredOrders = computed(() => orders.value.filter((item) => matchOrderFilters(item)))
const filteredPayments = computed(() => payments.value.filter((item) => {
  if (!matchDate(item.created_at)) return false
  const text = `${item.transaction_no || ''} ${item.order?.order_no || ''} ${item.order?.merchant?.name || ''}`.toLowerCase()
  return !filters.keyword || text.includes(filters.keyword.toLowerCase())
}))
const filteredRefunds = computed(() => refunds.value.filter((item) => {
  if (!matchDate(item.created_at)) return false
  const text = `${item.refund_no || ''} ${item.order?.order_no || ''} ${item.order?.merchant?.name || ''}`.toLowerCase()
  return !filters.keyword || text.includes(filters.keyword.toLowerCase())
}))
const refundableAmount = computed(() => {
  const row = refundOrder.value || {}
  return Math.max(Number(row.total_amount || row.amount || 0) - Number(row.refunded_amount || 0), 0)
})
const platformSubscriptionIncome = computed(() => filteredOrders.value
  .filter((item) => item.order_type === 'merchant_subscription' && item.status === 'paid')
  .reduce((sum, item) => sum + Number(item.total_amount || item.amount || 0), 0))
const customerTradeIncome = computed(() => filteredOrders.value
  .filter((item) => item.order_type === 'store_order' && paidStoreStatuses.includes(item.status))
  .reduce((sum, item) => sum + Number(item.total_amount || item.amount || 0), 0))
const totalRefunded = computed(() => filteredRefunds.value.reduce((sum, item) => sum + Number(item.amount || 0), 0))
const customerTradeNet = computed(() => Math.max(customerTradeIncome.value - totalRefunded.value, 0))

const load = async () => {
  loading.value = true
  try {
    const [ordersRes, paymentsRes, refundsRes] = await Promise.all([fetchOrders(), fetchPayments(), fetchRefunds()])
    orders.value = ordersRes.data || []
    payments.value = paymentsRes.data || []
    refunds.value = refundsRes.data || []
  } finally {
    loading.value = false
  }
}

const resetFilters = () => {
  filters.dateRange = []
  filters.keyword = ''
  filters.orderType = ''
  filters.status = ''
}

const matchOrderFilters = (item) => {
  if (!matchDate(item.created_at)) return false
  if (filters.orderType && item.order_type !== filters.orderType) return false
  if (filters.status && item.status !== filters.status) return false
  if (filters.keyword) {
    const text = `${item.order_no || ''} ${item.merchant?.name || ''} ${item.user?.phone || ''} ${item.customer_phone || ''}`.toLowerCase()
    if (!text.includes(filters.keyword.toLowerCase())) return false
  }
  return true
}

const matchDate = (value) => {
  if (!filters.dateRange?.length || !value) return true
  const day = String(value).slice(0, 10)
  return day >= filters.dateRange[0] && day <= filters.dateRange[1]
}

const canRefund = (row) => {
  if (!row || row.order_type !== 'store_order') return false
  const refundable = Number(row.total_amount || row.amount || 0) - Number(row.refunded_amount || 0)
  return refundable > 0 && paidStoreStatuses.includes(row.status)
}

const openRefund = (row) => {
  if (!row?.id) return
  refundOrder.value = row
  refundForm.full = true
  refundForm.amount_yuan = refundableAmount.value / 100
  refundForm.reason = ''
  refundDialogVisible.value = true
}

const submitRefund = async () => {
  if (!refundOrder.value) return
  const amount = refundForm.full ? 0 : Math.round(Number(refundForm.amount_yuan || 0) * 100)
  await ElMessageBox.confirm('请确认真实支付渠道已可执行该退款。本操作会写入平台退款记录用于财务核对。', '确认退款', { type: 'warning' })
  refundSaving.value = true
  try {
    await refundAdminOrder(refundOrder.value.id, {
      full: refundForm.full,
      amount,
      reason: refundForm.reason
    })
    ElMessage.success('退款记录已创建')
    refundDialogVisible.value = false
    await load()
  } finally {
    refundSaving.value = false
  }
}

const exportOrdersCsv = () => {
  const rows = filteredOrders.value.map((item) => ({
    订单号: item.order_no,
    类型: orderTypeLabel(item.order_type),
    商家或用户: item.merchant?.name || item.user?.display_name || item.user?.phone || '',
    金额: (Number(item.total_amount || item.amount || 0) / 100).toFixed(2),
    已退款: (Number(item.refunded_amount || 0) / 100).toFixed(2),
    状态: statusLabel(item.status),
    创建时间: formatTime(item.created_at)
  }))
  downloadCsv('platform-orders-finance.csv', rows)
}

const downloadCsv = (filename, rows) => {
  const headers = Object.keys(rows[0] || { 暂无数据: '' })
  const body = rows.length ? rows : [{ 暂无数据: '' }]
  const csv = [headers.join(','), ...body.map((row) => headers.map((key) => `"${String(row[key] ?? '').replace(/"/g, '""')}"`).join(','))].join('\n')
  const blob = new Blob([`\uFEFF${csv}`], { type: 'text/csv;charset=utf-8;' })
  const link = document.createElement('a')
  link.href = URL.createObjectURL(blob)
  link.download = filename
  link.click()
  URL.revokeObjectURL(link.href)
}

const formatMoney = (value) => `¥${(Number(value || 0) / 100).toFixed(2)}`
const formatTime = (value) => value ? String(value).replace('T', ' ').slice(0, 19) : '-'
const orderTypeLabel = (type) => ({
  user_membership: '用户会员',
  merchant_subscription: '商家订阅',
  store_order: '扫码点单'
}[type] || type || '-')
const statusLabel = (status) => ({
  pending: '待支付',
  paid: '已支付',
  received: '待接单',
  accepted: '已接单',
  completed: '已完成',
  closed: '已关闭',
  failed: '支付失败'
}[status] || status || '-')
const statusType = (status) => ({
  pending: 'warning',
  paid: 'success',
  received: 'primary',
  accepted: 'success',
  completed: 'success',
  closed: 'info',
  failed: 'danger'
}[status] || 'info')
const refundLabel = (status) => ({ none: '未退款', partial: '部分退款', full: '全额退款' }[status] || '未退款')
const refundType = (status) => ({ partial: 'warning', full: 'success' }[status] || 'info')

onMounted(load)
</script>

<style scoped>
.finance-page {
  display: grid;
  gap: 18px;
}

.toolbar {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: flex-start;
}

.toolbar-actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.page-desc {
  margin: 6px 0 0;
  color: var(--muted);
}

.finance-tips {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 12px;
  padding: 16px;
}

.finance-tips div {
  padding: 14px;
  border-radius: 14px;
  background: #f8fbff;
  border: 1px solid #e5edf9;
}

.finance-tips strong,
.finance-tips span {
  display: block;
}

.finance-tips span {
  margin-top: 6px;
  color: #64748b;
  line-height: 1.6;
}

.filters-card {
  display: flex;
  gap: 12px;
  align-items: center;
  flex-wrap: wrap;
}

.filters-card :deep(.el-input),
.filters-card :deep(.el-select) {
  width: 220px;
}

.finance-summary {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 14px;
}

.summary-card {
  padding: 18px;
  border-radius: 16px;
  border: 1px solid #e5edf9;
  background: #f8fbff;
}

.summary-card span,
.summary-card strong,
.summary-card small {
  display: block;
}

.summary-card span,
.summary-card small {
  color: #64748b;
}

.summary-card strong {
  margin-top: 8px;
  font-size: 26px;
}

.summary-card small {
  margin-top: 6px;
}

.summary-card.highlight {
  background: linear-gradient(135deg, #ecfdf5, #ffffff);
  border-color: #bbf7d0;
}

.refund-alert {
  margin-bottom: 16px;
}
</style>
