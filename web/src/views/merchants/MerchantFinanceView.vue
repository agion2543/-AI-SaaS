<template>
  <div class="finance-stack">
    <section class="page-card hero-card">
      <div>
        <div class="eyebrow">FINANCE CENTER</div>
        <h2 class="page-title">财务对账</h2>
        <p class="muted">
          仅统计顾客扫码点单的已支付订单，商家订阅费用不计入本页，方便商家核对真实经营收入。
        </p>
      </div>
      <div class="hero-actions">
        <el-button @click="exportIncomeCsv">导出收入 CSV</el-button>
        <el-button @click="exportRefundCsv">导出退款 CSV</el-button>
        <el-button type="primary" :loading="loading" @click="load">刷新财务</el-button>
      </div>
    </section>

    <section class="finance-note page-card">
      <div>
        <strong>收款口径</strong>
        <span>当前建议顾客付款直达商家自己的支付宝/微信账户，平台只做订单流水和对账记录。</span>
      </div>
      <div>
        <strong>净收入</strong>
        <span>净收入 = 已支付订单实付金额 - 已记录退款金额，用于核对门店实际经营结果。</span>
      </div>
      <div>
        <strong>后续扩展</strong>
        <span>如果后续接官方服务商分账，可继续增加结算批次、提现申请和到账状态。</span>
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
      <el-select v-model="filters.storeId" clearable placeholder="全部门店">
        <el-option v-for="store in stores" :key="store.id" :label="store.name" :value="store.id" />
      </el-select>
      <el-select v-model="filters.paymentChannel" clearable placeholder="全部支付渠道">
        <el-option label="支付宝" value="alipay" />
        <el-option label="微信/模拟" value="mock_wechat" />
        <el-option label="其他渠道" value="other" />
      </el-select>
      <el-select v-model="filters.status" clearable placeholder="全部订单状态">
        <el-option label="待接单" value="received" />
        <el-option label="已接单" value="accepted" />
        <el-option label="已完成" value="completed" />
        <el-option label="已关闭" value="closed" />
      </el-select>
      <el-button type="primary" plain @click="load">查询</el-button>
      <el-button @click="resetFilters">重置</el-button>
    </section>

    <section class="stat-grid">
      <div class="stat-card">
        <span>已支付订单</span>
        <strong>{{ filteredPaidOrders.length }}</strong>
      </div>
      <div class="stat-card">
        <span>订单实付</span>
        <strong>{{ formatMoney(totalPaid) }}</strong>
      </div>
      <div class="stat-card">
        <span>退款金额</span>
        <strong>{{ formatMoney(totalRefunded) }}</strong>
      </div>
      <div class="stat-card highlight">
        <span>净收入</span>
        <strong>{{ formatMoney(netIncome) }}</strong>
      </div>
      <div class="stat-card">
        <span>客单价</span>
        <strong>{{ formatMoney(avgPaid) }}</strong>
      </div>
    </section>

    <section class="page-card">
      <el-tabs>
        <el-tab-pane label="收入明细">
          <el-table :data="filteredPaidOrders" empty-text="暂无已支付扫码订单">
            <el-table-column prop="order_no" label="订单号" min-width="190" />
            <el-table-column label="门店" min-width="130">
              <template #default="{ row }">{{ row.store?.name || '-' }}</template>
            </el-table-column>
            <el-table-column prop="customer_phone" label="顾客手机号" min-width="130" />
            <el-table-column label="支付渠道" width="120">
              <template #default="{ row }">{{ channelLabel(row.payment_channel) }}</template>
            </el-table-column>
            <el-table-column label="订单实付" width="120">
              <template #default="{ row }">{{ formatMoney(orderPaidAmount(row)) }}</template>
            </el-table-column>
            <el-table-column label="退款金额" width="120">
              <template #default="{ row }">{{ formatMoney(row.refunded_amount) }}</template>
            </el-table-column>
            <el-table-column label="净收入" width="120">
              <template #default="{ row }"><strong>{{ formatMoney(netReceived(row)) }}</strong></template>
            </el-table-column>
            <el-table-column label="状态" width="110">
              <template #default="{ row }">
                <el-tag :type="statusType(row.status)">{{ statusLabel(row.status) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="支付时间" min-width="170">
              <template #default="{ row }">{{ formatTime(row.paid_at || row.updated_at) }}</template>
            </el-table-column>
          </el-table>
        </el-tab-pane>

        <el-tab-pane label="退款售后">
          <el-table :data="filteredRefunds" empty-text="暂无退款记录">
            <el-table-column prop="refund_no" label="退款单号" min-width="190" />
            <el-table-column label="关联订单" min-width="190">
              <template #default="{ row }">{{ row.order?.order_no || row.order_id }}</template>
            </el-table-column>
            <el-table-column label="门店" min-width="130">
              <template #default="{ row }">{{ storeName(row.store_id || row.order?.store_id) }}</template>
            </el-table-column>
            <el-table-column label="退款金额" width="120">
              <template #default="{ row }">{{ formatMoney(row.amount) }}</template>
            </el-table-column>
            <el-table-column prop="reason" label="退款原因" min-width="180" show-overflow-tooltip />
            <el-table-column label="操作方" width="120">
              <template #default="{ row }">{{ row.operator_role === 'admin' ? '平台后台' : '商家端' }}</template>
            </el-table-column>
            <el-table-column label="状态" width="100">
              <template #default="{ row }">
                <el-tag type="success">{{ row.status || 'success' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="创建时间" min-width="170">
              <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
            </el-table-column>
          </el-table>
        </el-tab-pane>

        <el-tab-pane label="结算说明">
          <div class="settlement-card">
            <h3>当前结算建议</h3>
            <p>
              前期采用“顾客直付商家账户”更稳妥，平台不碰经营流水，只收 SaaS 订阅费并保存订单、退款和对账数据。
              这样能降低二清与资金清分风险。
            </p>
            <div class="settlement-grid">
              <div><span>可核对净收入</span><strong>{{ formatMoney(netIncome) }}</strong></div>
              <div><span>结算方式</span><strong>商家自有收款账户</strong></div>
              <div><span>平台处理</span><strong>记录流水，不代收</strong></div>
            </div>
          </div>
        </el-tab-pane>
      </el-tabs>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { fetchMerchantOrders, fetchMerchantRefunds, fetchMerchantStores } from '../../api/modules'

const loading = ref(false)
const orders = ref([])
const refunds = ref([])
const stores = ref([])
const filters = reactive({ dateRange: [], storeId: undefined, status: '', paymentChannel: '' })
const paidStatuses = ['received', 'accepted', 'completed', 'closed']

const paidOrders = computed(() => orders.value.filter((item) => item.order_type === 'store_order' && paidStatuses.includes(item.status)))
const filteredPaidOrders = computed(() => paidOrders.value.filter((item) => matchFilters(item)))
const filteredRefunds = computed(() => refunds.value.filter((item) => {
  if (!matchDate(item.created_at)) return false
  if (filters.storeId && Number(item.store_id || item.order?.store_id) !== Number(filters.storeId)) return false
  return true
}))
const totalPaid = computed(() => filteredPaidOrders.value.reduce((sum, item) => sum + orderPaidAmount(item), 0))
const totalRefunded = computed(() => filteredRefunds.value.reduce((sum, item) => sum + Number(item.amount || 0), 0))
const netIncome = computed(() => Math.max(totalPaid.value - totalRefunded.value, 0))
const avgPaid = computed(() => filteredPaidOrders.value.length ? Math.round(totalPaid.value / filteredPaidOrders.value.length) : 0)

const load = async () => {
  loading.value = true
  try {
    const [ordersRes, refundsRes, storesRes] = await Promise.all([
      fetchMerchantOrders({ page: 1, page_size: 500 }),
      fetchMerchantRefunds(),
      fetchMerchantStores({ page: 1, page_size: 100 })
    ])
    orders.value = ordersRes.data.list || []
    refunds.value = refundsRes.data || []
    stores.value = storesRes.data.list || []
  } finally {
    loading.value = false
  }
}

const resetFilters = () => {
  filters.dateRange = []
  filters.storeId = undefined
  filters.status = ''
  filters.paymentChannel = ''
}

const matchFilters = (item) => {
  if (!matchDate(item.paid_at || item.updated_at)) return false
  if (filters.storeId && Number(item.store_id) !== Number(filters.storeId)) return false
  if (filters.status && item.status !== filters.status) return false
  if (filters.paymentChannel && normalizedChannel(item.payment_channel) !== filters.paymentChannel) return false
  return true
}

const matchDate = (value) => {
  if (!filters.dateRange?.length || !value) return true
  const day = String(value).slice(0, 10)
  return day >= filters.dateRange[0] && day <= filters.dateRange[1]
}

const orderPaidAmount = (row) => Number(row.total_amount || row.amount || 0)
const netReceived = (row) => Math.max(orderPaidAmount(row) - Number(row.refunded_amount || 0), 0)

const exportIncomeCsv = () => {
  const rows = filteredPaidOrders.value.map((item) => ({
    订单号: item.order_no,
    门店: item.store?.name || '',
    顾客手机号: item.customer_phone || '',
    支付渠道: channelLabel(item.payment_channel),
    订单实付元: centsToYuan(orderPaidAmount(item)),
    退款金额元: centsToYuan(item.refunded_amount),
    净收入元: centsToYuan(netReceived(item)),
    状态: statusLabel(item.status),
    支付时间: formatTime(item.paid_at || item.updated_at)
  }))
  downloadCsv('merchant-income.csv', rows)
}

const exportRefundCsv = () => {
  const rows = filteredRefunds.value.map((item) => ({
    退款单号: item.refund_no,
    关联订单: item.order?.order_no || item.order_id,
    门店: storeName(item.store_id || item.order?.store_id),
    退款金额元: centsToYuan(item.amount),
    退款原因: item.reason || '',
    操作方: item.operator_role === 'admin' ? '平台后台' : '商家端',
    状态: item.status || '',
    创建时间: formatTime(item.created_at)
  }))
  downloadCsv('merchant-refunds.csv', rows)
}

const downloadCsv = (filename, rows) => {
  const body = rows.length ? rows : [{ 暂无数据: '' }]
  const headers = Object.keys(body[0])
  const csv = [headers.join(','), ...body.map((row) => headers.map((key) => `"${String(row[key] ?? '').replace(/"/g, '""')}"`).join(','))].join('\n')
  const blob = new Blob([`\uFEFF${csv}`], { type: 'text/csv;charset=utf-8;' })
  const link = document.createElement('a')
  link.href = URL.createObjectURL(blob)
  link.download = filename
  link.click()
  URL.revokeObjectURL(link.href)
}

const storeName = (storeId) => stores.value.find((item) => Number(item.id) === Number(storeId))?.name || '-'
const normalizedChannel = (value) => {
  if (value === 'alipay') return 'alipay'
  if (value === 'mock_wechat') return 'mock_wechat'
  return 'other'
}
const channelLabel = (value) => ({
  alipay: '支付宝',
  mock_wechat: '微信/模拟',
  other: '其他渠道'
}[normalizedChannel(value)])
const centsToYuan = (value) => (Number(value || 0) / 100).toFixed(2)
const formatMoney = (value) => `¥${centsToYuan(value)}`
const formatTime = (value) => value ? String(value).replace('T', ' ').slice(0, 19) : '-'
const statusLabel = (status) => ({
  received: '待接单',
  accepted: '已接单',
  completed: '已完成',
  closed: '已关闭'
}[status] || status || '-')
const statusType = (status) => ({
  received: 'primary',
  accepted: 'success',
  completed: 'success',
  closed: 'info'
}[status] || 'info')

onMounted(load)
</script>

<style scoped>
.finance-stack {
  display: grid;
  gap: 18px;
}

.hero-card {
  display: flex;
  justify-content: space-between;
  gap: 18px;
  align-items: flex-start;
  padding: 22px;
}

.hero-actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.eyebrow {
  color: #0ea5e9;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.16em;
}

.muted {
  color: #64748b;
  margin: 0;
}

.finance-note,
.stat-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 14px;
}

.finance-note {
  padding: 16px;
}

.finance-note div,
.stat-card {
  padding: 16px;
  border-radius: 16px;
  background: #f8fbff;
  border: 1px solid #e5edf9;
}

.finance-note strong,
.finance-note span,
.stat-card span,
.stat-card strong {
  display: block;
}

.finance-note span,
.stat-card span {
  margin-top: 6px;
  color: #64748b;
  line-height: 1.6;
}

.stat-card strong {
  margin-top: 8px;
  font-size: 26px;
}

.stat-card.highlight {
  background: linear-gradient(135deg, #ecfdf5, #ffffff);
  border-color: #bbf7d0;
}

.filters-card {
  display: flex;
  gap: 12px;
  align-items: center;
  flex-wrap: wrap;
}

.settlement-card {
  padding: 22px;
  border-radius: 18px;
  background: #f8fbff;
  border: 1px solid #e5edf9;
}

.settlement-card p {
  color: #64748b;
}

.settlement-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 12px;
  margin-top: 16px;
}

.settlement-grid div {
  padding: 14px;
  border-radius: 14px;
  background: #fff;
}

.settlement-grid span,
.settlement-grid strong {
  display: block;
}

.settlement-grid span {
  color: #64748b;
}

.settlement-grid strong {
  margin-top: 8px;
}

@media (max-width: 768px) {
  .hero-card {
    display: grid;
  }
}
</style>
