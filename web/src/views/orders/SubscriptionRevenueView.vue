<template>
  <div class="subscription-revenue-page">
    <PageHero
      eyebrow="SUBSCRIPTION REVENUE"
      title="订阅营收管理"
      description="专门管理平台向商家收取的 SaaS 订阅费：待确认收款、已确认收入、续费记录和即将到期商家。顾客扫码交易流水不在这里处理。"
      compact
    >
      <template #actions>
        <el-button plain @click="exportRevenue">导出订阅账单</el-button>
        <el-button type="primary" :loading="loading" @click="load">刷新</el-button>
      </template>
    </PageHero>

    <section class="metric-grid">
      <MetricCard label="已确认订阅收入" :value="formatMoney(confirmedIncome)" hint="商家订阅已确认到账订单。" tone="primary" />
      <MetricCard label="待确认收款" :value="formatMoney(pendingAmount)" :hint="`${pendingOrders.length} 笔待平台核对`" tone="warning" />
      <MetricCard label="订阅中商家" :value="activeMerchantCount" hint="当前仍在有效期内的商家。" tone="success" />
      <MetricCard label="7 天内到期" :value="expiringMerchants.length" hint="建议提前跟进续费。" tone="orange" />
    </section>

    <section class="scope-alert">
      <strong>本页只处理平台 SaaS 订阅费</strong>
      <span>确认到账后会把订阅订单改为已确认收入，并同步开通或续期商家工作台。顾客扫码点单流水只在订单财务和商家财务中审查，不在这里确认到账。</span>
    </section>

    <section v-if="lastConfirmed" class="confirm-result">
      <div>
        <span>刚刚已开通</span>
        <strong>{{ lastConfirmed.merchant_name }}</strong>
        <small>{{ lastConfirmed.order_no }} / {{ lastConfirmed.plan_name }}</small>
      </div>
      <div>
        <span>到期时间</span>
        <strong>{{ formatTime(lastConfirmed.expire_at) }}</strong>
        <small>已写入商家服务状态和订阅订单记录</small>
      </div>
      <el-button plain @click="lastConfirmed = null">收起</el-button>
    </section>

    <section class="summary-strip">
      <button
        v-for="item in quickFilters"
        :key="item.value"
        type="button"
        :class="{ active: statusFilter === item.value }"
        @click="statusFilter = item.value"
      >
        <span>{{ item.label }}</span>
        <strong>{{ item.count }}</strong>
        <small>{{ item.hint }}</small>
      </button>
    </section>

    <section class="workflow-panel">
      <article v-for="item in workflowCards" :key="item.title">
        <span>{{ item.label }}</span>
        <strong>{{ item.title }}</strong>
        <p>{{ item.text }}</p>
      </article>
    </section>

    <DataPanel title="筛选条件" description="只筛选商家订阅订单，用于平台营收核对和续费跟进。">
      <div class="filters-row">
        <el-date-picker
          v-model="dateRange"
          type="daterange"
          unlink-panels
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          value-format="YYYY-MM-DD"
        />
        <el-input v-model="keyword" clearable placeholder="搜索订单号 / 商家 / 套餐" />
        <el-select v-model="statusFilter" clearable placeholder="订阅订单状态">
          <el-option label="全部" value="" />
          <el-option label="待确认收款" value="pending" />
          <el-option label="已确认收入" value="paid" />
          <el-option label="支付失败" value="failed" />
        </el-select>
        <el-button @click="resetFilters">重置</el-button>
      </div>
    </DataPanel>

    <DataPanel title="待确认订阅收款" description="这里是订阅到账确认的主操作页。订单财务只做总审查，顾客扫码订单不会在这里确认。">
      <el-table :data="pendingOrders" empty-text="暂无待确认订阅收款">
        <el-table-column prop="order_no" label="订阅订单号" min-width="190" />
        <el-table-column label="商家" min-width="160">
          <template #default="{ row }">{{ row.merchant?.name || row.merchant_id || '-' }}</template>
        </el-table-column>
        <el-table-column label="套餐" width="130">
          <template #default="{ row }">{{ planName(row) }}</template>
        </el-table-column>
        <el-table-column label="金额" width="120">
          <template #default="{ row }">{{ formatMoney(orderAmount(row)) }}</template>
        </el-table-column>
        <el-table-column label="付款方式" width="130">
          <template #default="{ row }">{{ paymentChannelLabel(row.payment_channel) }}</template>
        </el-table-column>
        <el-table-column label="开通后到期" min-width="160">
          <template #default="{ row }">{{ expectedExpireText(row) }}</template>
        </el-table-column>
        <el-table-column label="处理状态" min-width="180">
          <template #default="{ row }">
            <el-tag :type="subscriptionPaymentMarked(row) ? 'primary' : 'warning'">
              {{ subscriptionPaymentMarked(row) ? '商家已标记付款' : '待核对平台账户到账' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" min-width="170">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="160" fixed="right">
          <template #default="{ row }">
            <el-button
              size="small"
              type="primary"
              :loading="confirmingOrderId === row.id"
              @click="confirmSubscription(row)"
            >
              确认到账并开通
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </DataPanel>

    <section class="split-grid">
      <DataPanel title="已确认订阅收入" description="用于核对平台 SaaS 订阅营收。">
        <el-table :data="paidOrders" empty-text="暂无已确认订阅收入">
          <el-table-column prop="order_no" label="订单号" min-width="180" />
          <el-table-column label="商家" min-width="150">
            <template #default="{ row }">{{ row.merchant?.name || '-' }}</template>
          </el-table-column>
          <el-table-column label="套餐" width="120">
            <template #default="{ row }">{{ planName(row) }}</template>
          </el-table-column>
          <el-table-column label="金额" width="110">
            <template #default="{ row }">{{ formatMoney(orderAmount(row)) }}</template>
          </el-table-column>
          <el-table-column label="确认时间" min-width="160">
            <template #default="{ row }">{{ formatTime(row.paid_at || row.updated_at) }}</template>
          </el-table-column>
          <el-table-column label="开通到期" min-width="160">
            <template #default="{ row }">{{ formatTime(row.subscription_end_at) }}</template>
          </el-table-column>
          <el-table-column label="服务状态" width="120">
            <template #default="{ row }">
              <el-tag :type="merchantSubscriptionType(row)">{{ merchantSubscriptionLabel(row) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="入账口径" min-width="150">
            <template #default>平台 SaaS 订阅收入</template>
          </el-table-column>
        </el-table>
      </DataPanel>

      <DataPanel title="即将到期商家" description="7 天内到期的订阅商家，适合提前提醒续费。">
        <el-table :data="expiringMerchants" empty-text="暂无即将到期商家">
          <el-table-column prop="name" label="商家" min-width="150" />
          <el-table-column label="套餐" width="120">
            <template #default="{ row }">{{ row.subscription_plan || row.merchant_plan?.name || '-' }}</template>
          </el-table-column>
          <el-table-column label="到期时间" min-width="160">
            <template #default="{ row }">{{ formatTime(row.subscription_expire_at || row.subscription_expired_at) }}</template>
          </el-table-column>
          <el-table-column label="建议" min-width="150">
            <template #default>提前联系续费，避免商家端功能中断。</template>
          </el-table-column>
        </el-table>
      </DataPanel>
    </section>

    <DataPanel title="订阅订单明细" description="当前筛选范围内的所有商家订阅订单。">
      <el-table :data="filteredSubscriptionOrders" empty-text="暂无订阅订单">
        <el-table-column prop="order_no" label="订单号" min-width="190" />
        <el-table-column label="商家" min-width="160">
          <template #default="{ row }">{{ row.merchant?.name || row.merchant_id || '-' }}</template>
        </el-table-column>
        <el-table-column label="套餐" width="140">
          <template #default="{ row }">{{ planName(row) }}</template>
        </el-table-column>
        <el-table-column label="金额" width="120">
          <template #default="{ row }">{{ formatMoney(orderAmount(row)) }}</template>
        </el-table-column>
        <el-table-column label="状态" width="120">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)">{{ statusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="支付方式" width="130">
          <template #default="{ row }">{{ paymentChannelLabel(row.payment_channel) }}</template>
        </el-table-column>
        <el-table-column label="开通到期" min-width="160">
          <template #default="{ row }">{{ formatTime(row.subscription_end_at) }}</template>
        </el-table-column>
        <el-table-column label="商家服务" width="120">
          <template #default="{ row }">
            <el-tag :type="merchantSubscriptionType(row)">{{ merchantSubscriptionLabel(row) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" min-width="170">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
      </el-table>
    </DataPanel>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { confirmMerchantSubscriptionPayment, fetchMerchants, fetchOrders } from '../../api/modules'
import DataPanel from '../../components/design/DataPanel.vue'
import MetricCard from '../../components/design/MetricCard.vue'
import PageHero from '../../components/design/PageHero.vue'
import { exportRowsToXlsx } from '../../utils/xlsx'

const loading = ref(false)
const route = useRoute()
const confirmingOrderId = ref(null)
const orders = ref([])
const merchants = ref([])
const dateRange = ref([])
const keyword = ref('')
const statusFilter = ref('')
const lastConfirmed = ref(null)

const subscriptionOrders = computed(() => orders.value.filter((item) => item.order_type === 'merchant_subscription'))
const filteredSubscriptionOrders = computed(() => subscriptionOrders.value.filter(matchFilters))
const pendingOrders = computed(() => filteredSubscriptionOrders.value.filter((item) => item.status === 'pending'))
const paidOrders = computed(() => filteredSubscriptionOrders.value.filter((item) => item.status === 'paid'))
const confirmedIncome = computed(() => paidOrders.value.reduce((sum, item) => sum + orderAmount(item), 0))
const pendingAmount = computed(() => pendingOrders.value.reduce((sum, item) => sum + orderAmount(item), 0))
const activeMerchantCount = computed(() => merchants.value.filter(isSubscriptionActive).length)
const expiringMerchants = computed(() => {
  const now = Date.now()
  const end = now + 7 * 24 * 3600000
  return merchants.value
    .filter((item) => {
      const expire = dateValue(item.subscription_expire_at || item.subscription_expired_at)
      return item.subscription_status === 'active' && expire >= now && expire <= end
    })
    .sort((a, b) => dateValue(a.subscription_expire_at || a.subscription_expired_at) - dateValue(b.subscription_expire_at || b.subscription_expired_at))
})
const expiringMerchantIds = computed(() => new Set(expiringMerchants.value.map((item) => Number(item.id))))
const quickFilters = computed(() => [
  { label: '全部订阅', value: '', count: filteredSubscriptionOrders.value.length, hint: '当前筛选范围' },
  { label: '待确认', value: 'pending', count: subscriptionOrders.value.filter((item) => item.status === 'pending').length, hint: '需要核对到账' },
  { label: '已确认', value: 'paid', count: subscriptionOrders.value.filter((item) => item.status === 'paid').length, hint: '进入平台收入' },
  { label: '即将到期', value: 'expiring', count: expiringMerchants.value.length, hint: '7 天内需续费跟进' }
])
const workflowCards = [
  { label: 'STEP 1', title: '商家生成付款单', text: '商家端选择月付、年付或技术支持版后，生成平台收款码订阅订单。' },
  { label: 'STEP 2', title: '平台核对到账', text: '平台只核对自有支付宝或微信账户是否到账，不处理顾客和商家的门店交易款。' },
  { label: 'STEP 3', title: '确认并开通', text: '点击确认到账后，系统记录订阅收入，并自动开通或续期商家工作台。' }
]

const load = async () => {
  loading.value = true
  try {
    const [ordersRes, merchantsRes] = await Promise.all([
      fetchOrders(),
      fetchMerchants({ page: 1, page_size: 500 })
    ])
    orders.value = ordersRes.data || []
    merchants.value = merchantsRes.data?.list || merchantsRes.data || []
  } finally {
    loading.value = false
  }
}

const matchFilters = (item) => {
  if (statusFilter.value === 'expiring') {
    if (!item.merchant_id || !expiringMerchantIds.value.has(Number(item.merchant_id))) return false
  }
  if (statusFilter.value && statusFilter.value !== 'expiring' && item.status !== statusFilter.value) return false
  if (!matchDate(item.created_at)) return false
  if (keyword.value) {
    const text = `${item.order_no || ''} ${item.merchant?.name || ''} ${planName(item)}`.toLowerCase()
    if (!text.includes(keyword.value.toLowerCase())) return false
  }
  return true
}

const resetFilters = () => {
  dateRange.value = []
  keyword.value = ''
  statusFilter.value = ''
}

const confirmSubscription = async (row) => {
  await ElMessageBox.confirm(`确认“${row.merchant?.name || row.merchant_id || '-'}”订阅款已到账，并立即开通套餐吗？`, '确认订阅收款', { type: 'warning' })
  confirmingOrderId.value = row.id
  try {
    const res = await confirmMerchantSubscriptionPayment(row.id, { remark: '订阅营收页确认到账' })
    const order = res.data?.order || {}
    lastConfirmed.value = {
      order_no: order.order_no || row.order_no,
      merchant_name: order.merchant?.name || row.merchant?.name || row.merchant_id || '-',
      plan_name: planName(order.id ? order : row),
      expire_at: order.subscription_end_at
    }
    ElMessage.success(`已确认到账并开通订阅，到期时间：${formatTime(order.subscription_end_at)}`)
    await load()
  } catch (error) {
    ElMessage.error(error?.response?.data?.message || '确认到账失败，请刷新后重试')
  } finally {
    confirmingOrderId.value = null
  }
}

const exportRevenue = () => {
  exportRowsToXlsx('platform-subscription-revenue.xlsx', '订阅营收', filteredSubscriptionOrders.value.map((item) => ({
    订单号: item.order_no,
    商家: item.merchant?.name || item.merchant_id || '',
    套餐: planName(item),
    金额: (orderAmount(item) / 100).toFixed(2),
    状态: statusLabel(item.status),
    支付方式: paymentChannelLabel(item.payment_channel),
    创建时间: formatTime(item.created_at),
    确认时间: formatTime(item.paid_at)
  })))
}

const orderAmount = (row) => Number(row.total_amount || row.amount || 0)
const merchantById = (id) => merchants.value.find((item) => Number(item.id) === Number(id)) || null
const planName = (row) => {
  const plan = row.merchant_plan || row.merchantPlan || {}
  const name = String(plan.name || '')
  const duration = Number(plan.duration_days || 0)
  if (duration >= 365 || name.includes('年')) return '年付服务版'
  if (duration >= 30 || name.includes('月')) return '月付服务版'
  if (name.includes('技术')) return '技术支持版'
  return name || '-'
}
const expectedExpireText = (row) => {
  const merchant = merchantById(row.merchant_id)
  const currentExpire = dateValue(merchant?.subscription_expire_at || merchant?.subscription_expired_at)
  const plan = row.merchant_plan || row.merchantPlan || {}
  const duration = Number(plan.duration_days || 0)
  if (!duration) return '确认后自动计算'
  const base = Math.max(currentExpire, Date.now())
  return formatTime(new Date(base + duration * 24 * 3600000).toISOString())
}
const merchantSubscriptionLabel = (row) => {
  const merchant = row.merchant || merchantById(row.merchant_id)
  if (!merchant) return row.status === 'paid' ? '已开通' : '待确认'
  return isSubscriptionActive(merchant) ? '服务中' : '未生效'
}
const merchantSubscriptionType = (row) => merchantSubscriptionLabel(row) === '服务中' || row.status === 'paid' ? 'success' : 'warning'
const subscriptionPaymentMarked = (row) => Array.isArray(row.operation_logs) && row.operation_logs.some((item) => item.action === 'merchant_subscription_paid_marked')
const matchDate = (value) => {
  if (!dateRange.value?.length || !value) return true
  const day = String(value).slice(0, 10)
  return day >= dateRange.value[0] && day <= dateRange.value[1]
}
const isSubscriptionActive = (row) => {
  const expire = dateValue(row.subscription_expire_at || row.subscription_expired_at)
  return row.subscription_status === 'active' && expire > Date.now()
}
const dateValue = (value) => value ? new Date(value).getTime() || 0 : 0
const formatMoney = (value) => `¥${(Number(value || 0) / 100).toFixed(2)}`
const formatTime = (value) => value ? String(value).replace('T', ' ').slice(0, 19) : '-'
const statusLabel = (status) => ({ pending: '待确认收款', paid: '已确认收入', failed: '支付失败', closed: '已关闭' }[status] || status || '-')
const statusType = (status) => ({ pending: 'warning', paid: 'success', failed: 'danger', closed: 'info' }[status] || 'info')
const paymentChannelLabel = (channel) => ({
  platform_qr: '平台收款码',
  alipay: '支付宝',
  mock_wechat: '微信/模拟',
  merchant_qr: '商家收款码'
}[channel] || channel || '-')

onMounted(() => {
  keyword.value = route.query.keyword || ''
  statusFilter.value = route.query.status || ''
  load()
})
</script>

<style scoped>
.subscription-revenue-page {
  display: grid;
  gap: 18px;
}

.metric-grid,
.scope-alert,
.confirm-result,
.summary-strip,
.workflow-panel,
.split-grid {
  display: grid;
  gap: 14px;
}

.metric-grid {
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.scope-alert,
.confirm-result {
  align-items: center;
  border-radius: 8px;
}

.scope-alert {
  grid-template-columns: 220px minmax(0, 1fr);
  padding: 16px;
  border: 1px solid #bfdbfe;
  background: linear-gradient(90deg, #eff6ff, #ffffff);
  color: #475569;
}

.scope-alert strong {
  color: #0f172a;
}

.confirm-result {
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) auto;
  padding: 16px;
  border: 1px solid #bbf7d0;
  background: #f0fdf4;
}

.confirm-result span,
.confirm-result strong,
.confirm-result small {
  display: block;
}

.confirm-result span {
  color: #16a34a;
  font-size: 12px;
  font-weight: 900;
}

.confirm-result strong {
  margin-top: 6px;
  color: #0f172a;
  font-size: 18px;
}

.confirm-result small {
  margin-top: 4px;
  color: #64748b;
}

.summary-strip {
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.workflow-panel {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.summary-strip button {
  min-height: 108px;
  padding: 14px;
  text-align: left;
  cursor: pointer;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  background: #ffffff;
  transition: 0.2s ease;
}

.summary-strip button.active {
  border-color: #2563eb;
  background: #eff6ff;
  box-shadow: 0 12px 28px rgba(37, 99, 235, 0.12);
}

.summary-strip span,
.summary-strip strong,
.summary-strip small {
  display: block;
}

.summary-strip span {
  color: #64748b;
  font-size: 12px;
  font-weight: 800;
}

.summary-strip strong {
  margin: 8px 0 4px;
  color: #0f172a;
  font-size: 24px;
}

.summary-strip small {
  color: #64748b;
}

.workflow-panel article {
  min-height: 116px;
  padding: 16px;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  background: #ffffff;
}

.workflow-panel span,
.workflow-panel strong,
.workflow-panel p {
  display: block;
}

.workflow-panel span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
}

.workflow-panel strong {
  margin-top: 8px;
  color: #0f172a;
  font-size: 16px;
}

.workflow-panel p {
  margin: 8px 0 0;
  color: #64748b;
  line-height: 1.55;
}

.filters-row {
  display: grid;
  grid-template-columns: minmax(260px, 1.2fr) minmax(180px, 1fr) 180px auto;
  gap: 10px;
  align-items: center;
}

.split-grid {
  grid-template-columns: minmax(0, 1.15fr) minmax(0, 0.85fr);
}

@media (max-width: 1100px) {
  .metric-grid,
  .scope-alert,
  .confirm-result,
  .summary-strip,
  .filters-row,
  .workflow-panel,
  .split-grid {
    grid-template-columns: 1fr;
  }
}
</style>
