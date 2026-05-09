<template>
  <div class="dashboard-stack">
    <section class="hero-panel">
      <div>
        <div class="eyebrow">MERCHANT WORKBENCH</div>
        <h1>欢迎，{{ merchantName }}</h1>
        <p>这里聚合今日订单、实收、退款、热销商品和 AI 经营提醒，帮助商家快速判断今天该先处理什么。</p>
      </div>
      <div class="hero-actions">
        <el-button plain @click="router.push('/merchant/finance')">财务对账</el-button>
        <el-button type="primary" :loading="loading" @click="load">刷新看板</el-button>
      </div>
    </section>

    <section class="today-grid">
      <article class="metric-card primary">
        <span>今日订单</span>
        <strong>{{ todayOrders.length }}</strong>
        <small>只统计顾客扫码点单，不包含商家订阅订单</small>
      </article>
      <article class="metric-card success">
        <span>今日净实收</span>
        <strong>{{ formatMoney(todayNetIncome) }}</strong>
        <small>已支付订单金额 - 今日退款记录</small>
      </article>
      <article class="metric-card warning">
        <span>今日退款</span>
        <strong>{{ formatMoney(todayRefundAmount) }}</strong>
        <small>用于售后核对和异常复盘</small>
      </article>
      <article class="metric-card">
        <span>客单价</span>
        <strong>{{ formatMoney(todayAvgOrderAmount) }}</strong>
        <small>今日已支付订单平均实付金额</small>
      </article>
    </section>

    <section class="quick-grid">
      <button class="quick-card" type="button" @click="router.push('/merchant/orders')">
        <strong>{{ statusCount.received }}</strong>
        <span>待接单</span>
      </button>
      <button class="quick-card" type="button" @click="router.push('/merchant/orders')">
        <strong>{{ statusCount.accepted }}</strong>
        <span>履约中</span>
      </button>
      <button class="quick-card" type="button" @click="router.push('/merchant/promotions')">
        <strong>{{ recallCustomerCount }}</strong>
        <span>建议召回顾客</span>
      </button>
      <button class="quick-card" type="button" @click="router.push('/merchant/payment-settings')">
        <strong>{{ paymentTip }}</strong>
        <span>收款配置</span>
      </button>
    </section>

    <section class="grid-2">
      <div class="page-card">
        <div class="card-toolbar">
          <div>
            <h2 class="page-title">今日优先动作</h2>
            <p class="muted">根据订单、退款和顾客画像，优先展示当前最该处理的事项。</p>
          </div>
          <el-button text type="primary" @click="router.push('/merchant/ai')">查看 AI 分析</el-button>
        </div>
        <div class="todo-list">
          <div v-for="(item, index) in actionItems" :key="`${item.title}-${index}`" class="todo-item">
            <el-tag :type="item.type" size="small">{{ item.label }}</el-tag>
            <div>
              <strong>{{ item.title }}</strong>
              <p>{{ item.content }}</p>
            </div>
          </div>
        </div>
      </div>

      <div class="page-card">
        <h2 class="page-title">顾客经营</h2>
        <div class="customer-grid">
          <div>
            <span>顾客档案</span>
            <strong>{{ customerSummary.total || 0 }}</strong>
          </div>
          <div>
            <span>高价值顾客</span>
            <strong>{{ customerSummary.high_value || 0 }}</strong>
          </div>
          <div>
            <span>复购顾客</span>
            <strong>{{ customerSummary.repeat || 0 }}</strong>
          </div>
          <div>
            <span>需召回顾客</span>
            <strong>{{ recallCustomerCount }}</strong>
          </div>
        </div>
      </div>
    </section>

    <section class="grid-2">
      <div class="page-card">
        <div class="card-toolbar">
          <h2 class="page-title">热销商品</h2>
          <el-button text type="primary" @click="router.push('/merchant/stores')">管理商品</el-button>
        </div>
        <el-table :data="hotProducts" empty-text="暂无商品销量数据">
          <el-table-column prop="name" label="商品" min-width="150" />
          <el-table-column prop="quantity" label="销量" width="90" />
          <el-table-column label="销售额" width="120">
            <template #default="{ row }">{{ formatMoney(row.amount) }}</template>
          </el-table-column>
        </el-table>
      </div>

      <div class="page-card">
        <h2 class="page-title">低动销商品</h2>
        <el-table :data="slowProducts" empty-text="暂无低动销商品">
          <el-table-column prop="name" label="商品" min-width="150" />
          <el-table-column prop="quantity" label="销量" width="90" />
          <el-table-column label="建议" min-width="170">
            <template #default>优化图片/标题，或设置折扣活动测试</template>
          </el-table-column>
        </el-table>
      </div>
    </section>

    <section class="grid-2">
      <div class="page-card">
        <div class="card-toolbar">
          <h2 class="page-title">最近订单</h2>
          <el-button text type="primary" @click="router.push('/merchant/orders')">查看全部</el-button>
        </div>
        <el-table :data="recentOrders" empty-text="暂无订单">
          <el-table-column prop="order_no" label="订单号" min-width="180" show-overflow-tooltip />
          <el-table-column label="金额" width="110">
            <template #default="{ row }">{{ formatMoney(row.total_amount || row.amount) }}</template>
          </el-table-column>
          <el-table-column label="状态" width="110">
            <template #default="{ row }">
              <el-tag :type="orderStatusType(row.status)">{{ orderStatusLabel(row.status) }}</el-tag>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <div class="page-card">
        <div class="card-toolbar">
          <h2 class="page-title">最近线索</h2>
          <el-button text type="primary" @click="router.push('/merchant/ai')">查看分析</el-button>
        </div>
        <el-table :data="recentLeads" empty-text="暂无线索">
          <el-table-column prop="customer_name" label="称呼" min-width="100">
            <template #default="{ row }">{{ row.customer_name || '-' }}</template>
          </el-table-column>
          <el-table-column prop="customer_phone" label="手机号" min-width="130" />
          <el-table-column prop="status" label="状态" width="100">
            <template #default="{ row }">
              <el-tag :type="leadStatusType(row.status)">{{ leadStatusLabel(row.status) }}</el-tag>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  fetchMerchantAIInsights,
  fetchMerchantInfo,
  fetchMerchantLeads,
  fetchMerchantOrders,
  fetchMerchantPaymentConfig,
  fetchMerchantRefunds
} from '../../api/modules'
import { useMerchantAuthStore } from '../../stores/merchantAuth'

const router = useRouter()
const authStore = useMerchantAuthStore()
const loading = ref(false)
const insights = ref({})
const recentLeads = ref([])
const orders = ref([])
const refunds = ref([])
const paymentConfig = ref({})

const paidStatuses = ['received', 'accepted', 'completed', 'closed']
const merchant = computed(() => authStore.merchant || {})
const merchantName = computed(() => merchant.value?.name || authStore.profile?.display_name || '商家')
const orderStats = computed(() => insights.value.order_stats || {})
const customerSummary = computed(() => insights.value.customer_summary || {})
const hotProducts = computed(() => (insights.value.hot_products || []).slice(0, 8))
const slowProducts = computed(() => (insights.value.slow_products || []).slice(0, 8))
const recallCustomerCount = computed(() => Number(customerSummary.value.sleeping || 0) + Number(customerSummary.value.risk || 0))
const today = computed(() => new Date().toISOString().slice(0, 10))
const todayOrders = computed(() => orders.value.filter((item) => String(item.created_at || '').slice(0, 10) === today.value))
const paidTodayOrders = computed(() => todayOrders.value.filter((item) => paidStatuses.includes(item.status)))
const todayRefunds = computed(() => refunds.value.filter((item) => String(item.created_at || '').slice(0, 10) === today.value))
const todayPaidAmount = computed(() => paidTodayOrders.value.reduce((sum, item) => sum + Number(item.total_amount || item.amount || 0), 0))
const todayRefundAmount = computed(() => todayRefunds.value.reduce((sum, item) => sum + Number(item.amount || 0), 0))
const todayNetIncome = computed(() => Math.max(todayPaidAmount.value - todayRefundAmount.value, 0))
const todayAvgOrderAmount = computed(() => paidTodayOrders.value.length ? Math.round(todayPaidAmount.value / paidTodayOrders.value.length) : 0)
const recentOrders = computed(() => orders.value.slice(0, 6))
const statusCount = computed(() => orders.value.reduce((acc, item) => {
  acc[item.status] = (acc[item.status] || 0) + 1
  return acc
}, { received: 0, accepted: 0 }))
const paymentTip = computed(() => paymentConfig.value.audit_status === 'approved' ? '已通过' : paymentConfig.value.id ? '待审核' : '未填写')

const actionItems = computed(() => {
  const list = []
  if (statusCount.value.received > 0) {
    list.push({
      label: '接单',
      type: 'primary',
      title: `有 ${statusCount.value.received} 笔订单待接单`,
      content: '建议优先处理待接单订单，减少顾客等待时间。'
    })
  }
  if (todayRefundAmount.value > 0) {
    list.push({
      label: '售后',
      type: 'warning',
      title: `今日退款 ${formatMoney(todayRefundAmount.value)}`,
      content: '建议复盘退款原因，检查商品描述、履约速度或顾客沟通是否存在问题。'
    })
  }
  if (recallCustomerCount.value > 0) {
    list.push({
      label: '召回',
      type: 'success',
      title: `${recallCustomerCount.value} 位顾客适合做召回`,
      content: '可在优惠活动中生成沉睡召回券或复购券，提升再次下单概率。'
    })
  }
  if (paymentConfig.value.audit_status !== 'approved') {
    list.push({
      label: '收款',
      type: 'info',
      title: '收款配置还未审核通过',
      content: '建议完善收款设置并等待平台审核，后续可用于直连收款和财务核对。'
    })
  }
  return list.length ? list.slice(0, 4) : [
    {
      label: '经营',
      type: 'success',
      title: '当前经营状态平稳',
      content: '可以继续优化热销商品图片、上新套餐，并用 AI 分析生成下一轮活动。'
    }
  ]
})

const load = async () => {
  loading.value = true
  try {
    const [infoRes, aiRes, leadsRes, ordersRes, refundsRes, paymentRes] = await Promise.all([
      fetchMerchantInfo(),
      fetchMerchantAIInsights(),
      fetchMerchantLeads({ page: 1, page_size: 5 }),
      fetchMerchantOrders({ page: 1, page_size: 100 }),
      fetchMerchantRefunds(),
      fetchMerchantPaymentConfig()
    ])
    authStore.merchant = infoRes.data.merchant
    localStorage.setItem('merchant_current', JSON.stringify(infoRes.data.merchant))
    insights.value = aiRes.data || {}
    recentLeads.value = leadsRes.data.list || []
    orders.value = ordersRes.data.list || []
    refunds.value = refundsRes.data || []
    paymentConfig.value = paymentRes.data.config || {}
  } finally {
    loading.value = false
  }
}

const formatMoney = (value) => `¥${(Number(value || 0) / 100).toFixed(2)}`
const orderStatusLabel = (status) => ({
  pending: '待支付',
  received: '待接单',
  accepted: '已接单',
  completed: '已完成',
  closed: '已关闭',
  failed: '支付失败'
}[status] || status || '-')
const orderStatusType = (status) => ({
  pending: 'warning',
  received: 'primary',
  accepted: 'success',
  completed: 'success',
  closed: 'info',
  failed: 'danger'
}[status] || 'info')
const leadStatusLabel = (status) => ({ new: '新线索', contacted: '已联系', converted: '已成交', invalid: '无效' }[status] || status || '-')
const leadStatusType = (status) => ({ new: 'warning', contacted: 'primary', converted: 'success', invalid: 'danger' }[status] || 'info')

onMounted(load)
</script>

<style scoped>
.dashboard-stack {
  display: grid;
  gap: 20px;
}

.hero-panel {
  display: flex;
  justify-content: space-between;
  gap: 20px;
  align-items: flex-start;
  padding: 24px;
  border-radius: 24px;
  background:
    radial-gradient(circle at 82% 12%, rgba(56, 189, 248, 0.22), transparent 28%),
    linear-gradient(135deg, #0f172a, #1e3a8a);
  color: #fff;
}

.hero-panel h1 {
  margin: 8px 0;
  font-size: 30px;
}

.hero-panel p {
  max-width: 720px;
  margin: 0;
  color: #dbeafe;
  line-height: 1.7;
}

.hero-actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.eyebrow {
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.18em;
  color: #93c5fd;
}

.today-grid,
.quick-grid,
.grid-2 {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 18px;
}

.metric-card {
  padding: 20px;
  border-radius: 20px;
  background: #ffffff;
  border: 1px solid #e5edf9;
  box-shadow: 0 18px 40px rgba(15, 23, 42, 0.06);
}

.metric-card.primary {
  background: linear-gradient(135deg, #ecfeff, #eff6ff);
  border-color: #bae6fd;
}

.metric-card.success {
  background: linear-gradient(135deg, #ecfdf5, #ffffff);
  border-color: #bbf7d0;
}

.metric-card.warning {
  background: #fffbeb;
  border-color: #fde68a;
}

.metric-card span,
.customer-grid span {
  display: block;
  color: var(--muted);
  font-size: 13px;
}

.metric-card strong,
.customer-grid strong {
  display: block;
  margin-top: 8px;
  font-size: 28px;
}

.metric-card small {
  display: block;
  margin-top: 8px;
  color: #64748b;
}

.quick-card {
  text-align: left;
  padding: 18px;
  border: 1px solid #dbeafe;
  border-radius: 18px;
  background: #f8fbff;
  cursor: pointer;
  transition: 0.2s ease;
}

.quick-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 14px 30px rgba(15, 23, 42, 0.08);
}

.quick-card strong,
.quick-card span {
  display: block;
}

.quick-card strong {
  font-size: 24px;
}

.quick-card span {
  margin-top: 4px;
  color: #64748b;
}

.card-toolbar {
  display: flex;
  justify-content: space-between;
  gap: 14px;
  align-items: flex-start;
}

.muted {
  margin: 0;
  color: var(--muted);
  line-height: 1.7;
}

.todo-list {
  display: grid;
  gap: 12px;
  margin-top: 16px;
}

.todo-item {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 12px;
  padding: 14px;
  border-radius: 14px;
  background: #f8fafc;
  border: 1px solid #e5edf9;
}

.todo-item p {
  margin: 6px 0 0;
  color: var(--muted);
  line-height: 1.6;
}

.customer-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
  margin-top: 16px;
}

.customer-grid div {
  padding: 16px;
  border-radius: 14px;
  background: #f8fbff;
  border: 1px solid #e5edf9;
}

@media (max-width: 720px) {
  .hero-panel,
  .card-toolbar {
    flex-direction: column;
  }

  .customer-grid {
    grid-template-columns: 1fr;
  }
}
</style>
