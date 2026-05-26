<template>
  <div class="merchant-dashboard">
    <PageHero
      eyebrow="MERCHANT COCKPIT"
      :title="`欢迎，${merchantName}`"
      description="一屏看清今日订单、实收、待办、热销商品和 AI 经营建议。目标不是堆报表，而是让商家马上知道今天先做什么。"
      compact
    >
      <template #actions>
        <el-button plain @click="router.push('/merchant/orders')">处理订单</el-button>
        <el-button plain @click="router.push('/merchant/finance')">财务对账</el-button>
        <el-button type="primary" :loading="loading" @click="load">刷新看板</el-button>
      </template>
    </PageHero>

    <section class="metric-grid">
      <MetricCard label="今日订单" :value="todayOrders.length" hint="只统计顾客扫码点单订单。" tone="primary" />
      <MetricCard label="今日应收" :value="formatMoney(todayReceivableAmount)" hint="今日扫码订单账面金额，不等于已到账。" tone="primary" />
      <MetricCard label="待确认收款" :value="formatMoney(todayUnconfirmedAmount)" hint="顾客已标记付款，需核对到账。" tone="warning" />
      <MetricCard label="今日净实收" :value="formatMoney(todayNetIncome)" hint="今日确认实收扣除今日退款。" tone="success" />
    </section>

    <section class="dashboard-cashier">
      <div class="dashboard-cashier-head">
        <div>
          <span>CASHIER TODAY</span>
          <strong>今日收银口径</strong>
          <p>首页与财务对账页使用同一套口径：未付款不进收入，已标记付款先核对，确认收款后才算实收。</p>
        </div>
        <el-button plain type="primary" @click="router.push('/merchant/finance')">看财务对账</el-button>
      </div>
      <div class="cashier-flow">
        <div v-for="item in cashierFlow" :key="item.key" :class="['cashier-step', item.tone]" @click="item.go">
          <span>{{ item.label }}</span>
          <strong>{{ item.value }}</strong>
          <small>{{ item.hint }}</small>
        </div>
      </div>
    </section>

    <section v-if="priorityNotice.visible" class="priority-notice" :class="priorityNotice.tone">
      <div>
        <span>{{ priorityNotice.label }}</span>
        <strong>{{ priorityNotice.title }}</strong>
        <p>{{ priorityNotice.text }}</p>
      </div>
      <el-button type="primary" @click="priorityNotice.go">{{ priorityNotice.button }}</el-button>
    </section>

    <section class="dashboard-refresh">
      <div>
        <span>自动同步</span>
        <strong>{{ autoRefreshText }}</strong>
        <p>看板会定时同步订单、退款和收款配置，营业时打开首页即可看到最新待办。</p>
      </div>
      <div class="dashboard-refresh-actions">
        <el-switch
          v-model="autoRefreshEnabled"
          active-text="开启"
          inactive-text="暂停"
          @change="toggleAutoRefresh"
        />
        <el-button text type="primary" :loading="loading" @click="load()">立即同步</el-button>
      </div>
    </section>

    <section class="today-brief">
      <div>
        <span>今日经营结论</span>
        <strong>{{ todayBrief.title }}</strong>
        <p>{{ todayBrief.content }}</p>
      </div>
      <el-button type="primary" plain @click="todayBrief.go">{{ todayBrief.button }}</el-button>
    </section>

    <section class="fulfillment-board">
      <div class="fulfillment-head">
        <div>
          <span>TRANSACTION QUEUE</span>
          <strong>收银履约待办</strong>
          <p>把支付、接单、履约、退款这些会影响真实成交的事项集中到一排。</p>
        </div>
        <el-button text type="primary" @click="router.push('/merchant/orders')">进入订单中心</el-button>
      </div>
      <div class="fulfillment-grid">
        <button
          v-for="item in fulfillmentQueue"
          :key="item.key"
          type="button"
          :class="['fulfillment-card', item.tone]"
          @click="item.go"
        >
          <span>{{ item.label }}</span>
          <strong>{{ item.value }}</strong>
          <small>{{ item.hint }}</small>
        </button>
      </div>
    </section>

    <DataPanel
      eyebrow="DAILY REPORT"
      title="今日经营日报"
      description="把今日订单、实收、退款、客单价和下一步动作汇总成商家每天能直接看的简报。"
    >
      <template #actions>
        <el-button text type="primary" @click="openDailyAI">生成 AI 建议</el-button>
        <el-button text type="primary" @click="copyDailyReport">复制日报</el-button>
        <el-button text type="primary" @click="exportDailyReport">导出 XLSX</el-button>
      </template>
      <div class="daily-report primary-report">
        <div v-for="item in primaryDailyItems" :key="item.label" class="daily-item">
          <span>{{ item.label }}</span>
          <strong>{{ item.value }}</strong>
          <p>{{ item.hint }}</p>
        </div>
      </div>
      <el-collapse class="daily-more">
        <el-collapse-item title="展开查看支付、履约和售后明细" name="detail">
          <div class="daily-report compact-report">
            <div v-for="item in secondaryDailyItems" :key="item.label" class="daily-item compact">
              <span>{{ item.label }}</span>
              <strong>{{ item.value }}</strong>
              <p>{{ item.hint }}</p>
            </div>
          </div>
        </el-collapse-item>
      </el-collapse>
      <div class="report-summary">{{ dailyReportText }}</div>
    </DataPanel>

    <section class="shortcut-grid">
      <button class="shortcut-card urgent" type="button" @click="router.push('/merchant/orders?status=received')">
        <span>订单待办</span>
        <strong>{{ paymentConfirmingCount + Number(statusCount.received || 0) }}</strong>
        <small>确认收款和接单优先处理</small>
      </button>
      <button class="shortcut-card warning" type="button" @click="router.push('/merchant/orders?quick=pay_issue')">
        <span>支付异常</span>
        <strong>{{ paymentIssueCount }}</strong>
        <small>待支付/失败订单不计入实收</small>
      </button>
      <button class="shortcut-card" type="button" @click="router.push('/merchant/promotions')">
        <span>营销活动</span>
        <strong>{{ recallCustomerCount }}</strong>
        <small>适合生成召回券/复购券</small>
      </button>
      <button class="shortcut-card" type="button" @click="router.push('/merchant/coupons')">
        <span>券包核销</span>
        <strong>{{ couponHint }}</strong>
        <small>按手机号查券、核销、作废</small>
      </button>
      <button class="shortcut-card" type="button" @click="router.push('/merchant/payment-settings')">
        <span>收款配置</span>
        <strong>{{ paymentTip }}</strong>
        <small>影响支付、对账和人工结算</small>
      </button>
    </section>

    <section class="main-grid">
      <DataPanel
        eyebrow="TODAY ACTIONS"
        title="今日优先动作"
        description="根据订单、退款、顾客沉淀和收款配置自动整理。先处理这些，商家每天不用到处找重点。"
      >
        <div class="action-list">
          <ActionCard
            v-for="(item, index) in actionItems"
            :key="`${item.title}-${index}`"
            :label="item.label"
            :title="item.title"
            :description="item.content"
            :tone="item.tone"
          >
            <template #action>
              <el-button size="small" type="primary" plain @click="item.go">{{ item.button }}</el-button>
            </template>
          </ActionCard>
        </div>
      </DataPanel>

      <DataPanel
        eyebrow="CUSTOMER ASSET"
        title="顾客经营资产"
        description="扫码点单不是只收钱，更重要是沉淀手机号、复购行为和可触达的顾客标签。"
      >
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
            <span>建议召回</span>
            <strong>{{ recallCustomerCount }}</strong>
          </div>
        </div>
        <div class="hint-box">{{ customerHint }}</div>
      </DataPanel>
    </section>

    <section class="main-grid">
      <DataPanel eyebrow="PRODUCTS" title="热销商品" description="优先保证热销商品图片、库存和套餐搭配，适合放在扫码页更靠前的位置。">
        <template #actions>
          <el-button text type="primary" @click="router.push('/merchant/stores')">管理商品</el-button>
        </template>
        <el-table :data="hotProducts" empty-text="暂无商品销量数据">
          <el-table-column prop="name" label="商品" min-width="150" />
          <el-table-column prop="quantity" label="销量" width="90" />
          <el-table-column label="销售额" width="120">
            <template #default="{ row }">{{ formatMoney(row.amount) }}</template>
          </el-table-column>
        </el-table>
      </DataPanel>

      <DataPanel eyebrow="OPTIMIZE" title="低动销商品" description="用于发现需要换图、改标题、降价或参与满减活动的商品。">
        <el-table :data="slowProducts" empty-text="暂无低动销商品">
          <el-table-column prop="name" label="商品" min-width="150" />
          <el-table-column prop="quantity" label="销量" width="90" />
          <el-table-column label="建议" min-width="200">
            <template #default="{ row }">{{ lowProductSuggestion(row) }}</template>
          </el-table-column>
        </el-table>
      </DataPanel>
    </section>

    <section class="main-grid">
      <DataPanel eyebrow="ORDERS" title="最近订单" description="快速查看最新扫码点单状态。">
        <template #actions>
          <el-button text type="primary" @click="router.push('/merchant/orders')">查看全部</el-button>
        </template>
        <el-table :data="recentOrders" empty-text="暂无订单">
          <el-table-column prop="order_no" label="订单号" min-width="180" show-overflow-tooltip />
          <el-table-column label="金额" width="110">
            <template #default="{ row }">{{ formatMoney(orderAmount(row)) }}</template>
          </el-table-column>
          <el-table-column label="状态" width="110">
            <template #default="{ row }">
              <el-tag :type="orderStatusType(row.status)">{{ orderStatusLabel(row.status) }}</el-tag>
            </template>
          </el-table-column>
        </el-table>
      </DataPanel>

      <DataPanel eyebrow="LEADS" title="最近线索" description="来自扫码、裂变海报和营销活动的潜在顾客。">
        <template #actions>
          <el-button text type="primary" @click="router.push('/merchant/ai?scenario=dormant_recall')">查看分析</el-button>
        </template>
        <el-table :data="recentLeads" empty-text="暂无线索">
          <el-table-column label="称呼" min-width="100">
            <template #default="{ row }">{{ row.customer_name || '-' }}</template>
          </el-table-column>
          <el-table-column prop="customer_phone" label="手机号" min-width="130" />
          <el-table-column label="状态" width="100">
            <template #default="{ row }">
              <el-tag :type="leadStatusType(row.status)">{{ leadStatusLabel(row.status) }}</el-tag>
            </template>
          </el-table-column>
        </el-table>
      </DataPanel>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  fetchMerchantAIInsights,
  fetchMerchantInfo,
  fetchMerchantLeads,
  fetchMerchantOrders,
  fetchMerchantPaymentConfig,
  fetchMerchantRefunds
} from '../../api/modules'
import ActionCard from '../../components/design/ActionCard.vue'
import DataPanel from '../../components/design/DataPanel.vue'
import MetricCard from '../../components/design/MetricCard.vue'
import PageHero from '../../components/design/PageHero.vue'
import { useMerchantAuthStore } from '../../stores/merchantAuth'
import { exportRowsToXlsx } from '../../utils/xlsx'

const router = useRouter()
const authStore = useMerchantAuthStore()

const loading = ref(false)
const insights = ref({})
const recentLeads = ref([])
const orders = ref([])
const refunds = ref([])
const paymentConfig = ref({})
const merchantInfo = ref({})
const paidStatuses = ['received', 'accepted', 'completed', 'closed']
const autoRefreshEnabled = ref(true)
const lastRefreshAt = ref('')
const previousActionCount = ref(null)
let refreshTimer = null
const refreshIntervalMs = 45000

const todayText = () => {
  const date = new Date()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${date.getFullYear()}-${month}-${day}`
}

const merchantName = computed(() => (
  merchantInfo.value?.name ||
  authStore.merchant?.name ||
  authStore.profile?.display_name ||
  '商家'
))
const customerSummary = computed(() => insights.value.customer_summary || {})
const hotProducts = computed(() => (insights.value.hot_products || []).slice(0, 6))
const slowProducts = computed(() => (insights.value.slow_products || []).slice(0, 6))
const recallCustomerCount = computed(() => (
  Number(customerSummary.value.sleeping || 0) + Number(customerSummary.value.risk || 0)
))
const today = computed(todayText)
const todayOrders = computed(() => orders.value.filter((item) => String(item.created_at || '').slice(0, 10) === today.value))
const paidTodayOrders = computed(() => orders.value.filter((item) => (
  paidStatuses.includes(item.status) && String(item.paid_at || item.updated_at || '').slice(0, 10) === today.value
)))
const todayPendingPayOrders = computed(() => todayOrders.value.filter((item) => ['pending', 'failed'].includes(item.status)))
const todayUnconfirmedOrders = computed(() => todayOrders.value.filter((item) => item.status === 'payment_confirming'))
const todayRefunds = computed(() => refunds.value.filter((item) => String(item.created_at || '').slice(0, 10) === today.value))
const todayReceivableAmount = computed(() => todayOrders.value.reduce((sum, item) => sum + orderAmount(item), 0))
const todayPendingPayAmount = computed(() => todayPendingPayOrders.value.reduce((sum, item) => sum + orderAmount(item), 0))
const todayUnconfirmedAmount = computed(() => todayUnconfirmedOrders.value.reduce((sum, item) => sum + orderAmount(item), 0))
const todayPaidAmount = computed(() => paidTodayOrders.value.reduce((sum, item) => sum + orderAmount(item), 0))
const todayRefundAmount = computed(() => todayRefunds.value.reduce((sum, item) => sum + Number(item.amount || 0), 0))
const todayNetIncome = computed(() => Math.max(todayPaidAmount.value - todayRefundAmount.value, 0))
const todayAvgOrderAmount = computed(() => (
  paidTodayOrders.value.length ? Math.round(todayPaidAmount.value / paidTodayOrders.value.length) : 0
))
const recentOrders = computed(() => orders.value.slice(0, 6))
const statusCount = computed(() => orders.value.reduce((acc, item) => {
  acc[item.status] = (acc[item.status] || 0) + 1
  return acc
}, { pending: 0, failed: 0, submitted: 0, payment_confirming: 0, preparing: 0, received: 0, accepted: 0, completed: 0, closed: 0 }))
const pendingPaymentCount = computed(() => Number(statusCount.value.pending || 0))
const failedPaymentCount = computed(() => Number(statusCount.value.failed || 0))
const paymentConfirmingCount = computed(() => Number(statusCount.value.payment_confirming || 0))
const paymentIssueCount = computed(() => pendingPaymentCount.value + failedPaymentCount.value)
const fulfillmentCount = computed(() => paymentConfirmingCount.value + Number(statusCount.value.received || 0) + Number(statusCount.value.accepted || 0))
const refundRiskCount = computed(() => orders.value.filter((item) => Number(item.refunded_amount || 0) > 0 || ['partial', 'full'].includes(item.refund_status)).length)
const autoRefreshText = computed(() => (
  autoRefreshEnabled.value
    ? `已开启，每 ${Math.round(refreshIntervalMs / 1000)} 秒同步一次${lastRefreshAt.value ? `，上次 ${lastRefreshAt.value}` : ''}`
    : `已暂停${lastRefreshAt.value ? `，上次 ${lastRefreshAt.value}` : ''}`
))
const cashierFlow = computed(() => [
  {
    key: 'receivable',
    label: '应收',
    value: formatMoney(todayReceivableAmount.value),
    hint: `${todayOrders.value.length} 笔今日订单账面金额`,
    tone: 'primary',
    go: () => router.push('/merchant/finance')
  },
  {
    key: 'pending',
    label: '待顾客付款',
    value: formatMoney(todayPendingPayAmount.value),
    hint: `${todayPendingPayOrders.value.length} 笔暂不计入收入`,
    tone: todayPendingPayOrders.value.length ? 'warning' : 'muted',
    go: () => router.push('/merchant/orders?quick=pay_issue')
  },
  {
    key: 'confirming',
    label: '已标记付款',
    value: formatMoney(todayUnconfirmedAmount.value),
    hint: `${todayUnconfirmedOrders.value.length} 笔等待核对到账`,
    tone: todayUnconfirmedOrders.value.length ? 'warning' : 'muted',
    go: () => router.push('/merchant/orders?status=payment_confirming')
  },
  {
    key: 'confirmed',
    label: '已确认实收',
    value: formatMoney(todayPaidAmount.value),
    hint: `${paidTodayOrders.value.length} 笔今日确认到账`,
    tone: 'success',
    go: () => router.push('/merchant/finance')
  },
  {
    key: 'refund',
    label: '已退款',
    value: formatMoney(todayRefundAmount.value),
    hint: `${todayRefunds.value.length} 条退款记录`,
    tone: todayRefundAmount.value ? 'danger' : 'muted',
    go: () => router.push('/merchant/finance')
  },
  {
    key: 'net',
    label: '净实收',
    value: formatMoney(todayNetIncome.value),
    hint: '已确认实收 - 今日退款',
    tone: 'success',
    go: () => router.push('/merchant/finance')
  }
])
const priorityNotice = computed(() => {
  if (paymentConfirmingCount.value > 0) {
    return {
      visible: true,
      tone: 'warning',
      label: '优先处理',
      title: `${paymentConfirmingCount.value} 笔订单等待确认收款`,
      text: '顾客已标记付款，先核对到账再确认，确认后才会计入净实收和财务对账。',
      button: '去确认收款',
      go: () => router.push('/merchant/orders?status=payment_confirming')
    }
  }
  if (Number(statusCount.value.received || 0) > 0) {
    return {
      visible: true,
      tone: 'primary',
      label: '待处理',
      title: `${statusCount.value.received} 笔订单待接单`,
      text: '这些订单已经确认收款或进入处理队列，建议尽快接单并打印小票。',
      button: '去接单',
      go: () => router.push('/merchant/orders?status=received')
    }
  }
  if (paymentIssueCount.value > 0) {
    return {
      visible: true,
      tone: 'pay',
      label: '支付异常',
      title: `${paymentIssueCount.value} 笔订单支付未完成`,
      text: '待支付或失败订单不计入实收，顾客咨询时引导继续支付即可。',
      button: '看异常',
      go: () => router.push('/merchant/orders?quick=pay_issue')
    }
  }
  return { visible: false }
})
const topProduct = computed(() => hotProducts.value[0]?.name || '暂无热销商品')
const dailyReportItems = computed(() => [
  { label: '今日订单', value: `${todayOrders.value.length} 单`, hint: '顾客扫码点单产生的订单量。' },
  { label: '今日应收', value: formatMoney(todayReceivableAmount.value), hint: '今日扫码订单账面金额，不等于实际到账。' },
  { label: '待顾客付款', value: `${todayPendingPayOrders.value.length} 单 / ${formatMoney(todayPendingPayAmount.value)}`, hint: '待支付或支付失败，不计入今日实收。' },
  { label: '已标记付款', value: `${todayUnconfirmedOrders.value.length} 单 / ${formatMoney(todayUnconfirmedAmount.value)}`, hint: '顾客已标记付款，仍需核对到账。' },
  { label: '已确认实收', value: formatMoney(todayPaidAmount.value), hint: '今日确认到账的扫码订单金额。' },
  { label: '今日净实收', value: formatMoney(todayNetIncome.value), hint: '已确认实收扣除今日退款后的金额。' },
  { label: '今日退款', value: formatMoney(todayRefundAmount.value), hint: '用于判断履约、商品描述或沟通问题。' },
  { label: '客单价', value: formatMoney(todayAvgOrderAmount.value), hint: '今日已支付订单平均金额。' },
  { label: '热销商品', value: topProduct.value, hint: '适合放在扫码页前排或做组合套餐。' },
  { label: '待确认收款', value: `${paymentConfirmingCount.value} 单`, hint: '顾客已标记付款，需核对到账。' },
  { label: '待接单', value: `${statusCount.value.received || 0} 单`, hint: '需要优先处理，避免顾客等待。' },
  { label: '履约中', value: `${statusCount.value.accepted || 0} 单`, hint: '已接单但还未完成，适合交班核对。' },
  { label: '支付异常', value: `${paymentIssueCount.value} 单`, hint: '待支付或支付失败，不计入今日实收。' },
  { label: '售后风险', value: `${refundRiskCount.value} 单`, hint: '已记录退款或部分退款，需核对原因。' }
])
const primaryDailyItems = computed(() => [
  dailyReportItems.value[0],
  dailyReportItems.value[4],
  dailyReportItems.value[5],
  dailyReportItems.value[12],
  dailyReportItems.value[8],
  dailyReportItems.value[11]
].filter(Boolean))
const secondaryDailyItems = computed(() => dailyReportItems.value.filter((item) => !primaryDailyItems.value.includes(item)))
const dailyReportText = computed(() => [
  `${today.value} 经营日报：`,
  `今日订单 ${todayOrders.value.length} 单，应收 ${formatMoney(todayReceivableAmount.value)}，已确认实收 ${formatMoney(todayPaidAmount.value)}，退款 ${formatMoney(todayRefundAmount.value)}，净实收 ${formatMoney(todayNetIncome.value)}。`,
  `待顾客付款 ${todayPendingPayOrders.value.length} 单，已标记付款 ${todayUnconfirmedOrders.value.length} 单；全部待确认收款 ${paymentConfirmingCount.value} 单，待接单 ${statusCount.value.received || 0} 单，履约中 ${statusCount.value.accepted || 0} 单。`,
  `当前热销商品：${topProduct.value}。`,
  `今日重点动作：${todayBrief.value.title}。`
].join('\n'))

const fulfillmentQueue = computed(() => [
  {
    key: 'payment_confirming',
    label: '确认收款',
    value: `${paymentConfirmingCount.value} 单`,
    hint: paymentConfirmingCount.value ? '核对到账后确认' : '暂无待确认收款',
    tone: paymentConfirmingCount.value ? 'warning' : 'muted',
    go: () => router.push('/merchant/orders?status=payment_confirming')
  },
  {
    key: 'pay_issue',
    label: '支付未完成',
    value: `${paymentIssueCount.value} 单`,
    hint: paymentIssueCount.value ? '顾客可继续支付，不计入实收' : '暂无支付异常',
    tone: paymentIssueCount.value ? 'warning' : 'muted',
    go: () => router.push('/merchant/orders?quick=pay_issue')
  },
  {
    key: 'received',
    label: '待接单',
    value: `${statusCount.value.received || 0} 单`,
    hint: statusCount.value.received ? '优先接单并打印小票' : '暂无待接单',
    tone: statusCount.value.received ? 'primary' : 'muted',
    go: () => router.push('/merchant/orders?status=received')
  },
  {
    key: 'accepted',
    label: '履约中',
    value: `${statusCount.value.accepted || 0} 单`,
    hint: statusCount.value.accepted ? '制作完成后及时标记' : '暂无履约中订单',
    tone: statusCount.value.accepted ? 'success' : 'muted',
    go: () => router.push('/merchant/orders?status=accepted')
  },
  {
    key: 'refund',
    label: '退款核对',
    value: `${refundRiskCount.value} 单`,
    hint: refundRiskCount.value ? '核对退款原因和净实收' : '暂无退款风险',
    tone: refundRiskCount.value ? 'danger' : 'muted',
    go: () => router.push('/merchant/orders?quick=refund')
  }
])

const paymentTip = computed(() => {
  if (paymentConfig.value.audit_status === 'approved') return '已通过'
  if (paymentConfig.value.id) return '待审核'
  return '未配置'
})
const couponHint = computed(() => Number(customerSummary.value.coupon_users || 0) || '查看')

const todayBrief = computed(() => {
  if (paymentConfirmingCount.value > 0) {
    return {
      title: `有 ${paymentConfirmingCount.value} 笔订单等待确认收款`,
      content: '顾客已标记付款，请先核对支付宝/微信实际到账，确认后才会计入实收和财务对账。',
      button: '确认收款',
      go: () => router.push('/merchant/orders?status=payment_confirming')
    }
  }
  if (paymentIssueCount.value > 0 && statusCount.value.received <= 0) {
    return {
      title: `有 ${paymentIssueCount.value} 笔订单支付未完成`,
      content: '这些订单不会计入实收，顾客可在订单页继续支付。建议只在顾客咨询时协助，不要提前制作。',
      button: '看异常',
      go: () => router.push('/merchant/orders?quick=pay_issue')
    }
  }
  if (statusCount.value.received > 0) {
    return {
      title: `有 ${statusCount.value.received} 笔订单需要接单`,
      content: `今日已产生 ${todayOrders.value.length} 笔订单，净实收 ${formatMoney(todayNetIncome.value)}。先处理待接单，能直接降低顾客等待和取消风险。`,
      button: '去接单',
      go: () => router.push('/merchant/orders?status=received')
    }
  }
  if (todayRefundAmount.value > 0) {
    return {
      title: `今日退款 ${formatMoney(todayRefundAmount.value)}，建议复盘原因`,
      content: `今日净实收 ${formatMoney(todayNetIncome.value)}，客单价 ${formatMoney(todayAvgOrderAmount.value)}。优先查看退款订单，找出商品描述、等待时间或沟通问题。`,
      button: '看财务',
      go: () => router.push('/merchant/ai?scenario=refund_review')
    }
  }
  if (fulfillmentCount.value > 0) {
    return {
      title: `还有 ${fulfillmentCount.value} 笔订单在处理链路中`,
      content: `待确认收款 ${paymentConfirmingCount.value} 笔，待接单 ${statusCount.value.received || 0} 笔，履约中 ${statusCount.value.accepted || 0} 笔。建议交班前全部核对一次。`,
      button: '看订单',
      go: () => router.push('/merchant/orders')
    }
  }
  if (todayOrders.value.length > 0) {
    return {
      title: `今日经营正常，净实收 ${formatMoney(todayNetIncome.value)}`,
      content: `今日 ${todayOrders.value.length} 笔订单，客单价 ${formatMoney(todayAvgOrderAmount.value)}。可以继续优化热销商品和裂变海报，提高复购。`,
      button: '看热销',
      go: () => router.push('/merchant/ai?scenario=campaign')
    }
  }
  return {
    title: '今天还没有扫码点单，先做曝光和活动',
    content: '建议检查门店二维码、商品图片和优惠活动，也可以用 AI 生成朋友圈文案或短视频脚本引流。',
    button: '做营销',
    go: () => router.push('/merchant/ai?scenario=short_video')
  }
})

const customerHint = computed(() => {
  if (recallCustomerCount.value > 0) {
    return '建议为沉睡或流失风险顾客创建召回券，降低再次下单门槛。'
  }
  if (Number(customerSummary.value.high_value || 0) > 0) {
    return '高价值顾客适合配置专属券或新品优先通知，提升复购和客单价。'
  }
  return '当前顾客数据仍在沉淀，建议引导顾客扫码下单或留资，积累可分析样本。'
})

const actionItems = computed(() => {
  const list = []
  if (paymentConfirmingCount.value > 0) {
    list.push({
      label: '收款',
      tone: 'orange',
      title: `有 ${paymentConfirmingCount.value} 笔订单待确认收款`,
      content: '顾客已标记付款，请先核对到账，再确认收款计入实收。',
      button: '去确认',
      go: () => router.push('/merchant/orders?status=payment_confirming')
    })
  }
  if (statusCount.value.received > 0) {
    list.push({
      label: '接单',
      tone: 'blue',
      title: `有 ${statusCount.value.received} 笔订单待接单`,
      content: '建议优先处理待接单订单，减少顾客等待时间。',
      button: '去接单',
      go: () => router.push('/merchant/orders?status=received')
    })
  }
  if (paymentIssueCount.value > 0) {
    list.push({
      label: '支付',
      tone: 'orange',
      title: `${paymentIssueCount.value} 笔订单支付未完成`,
      content: '待支付或支付失败订单不计入实收，顾客咨询时引导其在订单页继续支付即可。',
      button: '看异常',
      go: () => router.push('/merchant/orders?quick=pay_issue')
    })
  }
  if (statusCount.value.accepted > 0) {
    list.push({
      label: '履约',
      tone: 'green',
      title: `${statusCount.value.accepted} 笔订单正在履约`,
      content: '建议出餐或交付后及时标记完成，方便后续对账和复购分析。',
      button: '看履约',
      go: () => router.push('/merchant/orders?status=accepted')
    })
  }
  if (todayRefundAmount.value > 0) {
    list.push({
      label: '售后',
      tone: 'orange',
      title: `今日退款 ${formatMoney(todayRefundAmount.value)}`,
      content: '建议复盘退款原因，检查商品描述、履约速度或顾客沟通是否存在问题。',
      button: '看财务',
      go: () => router.push('/merchant/ai?scenario=refund_review')
    })
  }
  if (recallCustomerCount.value > 0) {
    list.push({
      label: '召回',
      tone: 'green',
      title: `${recallCustomerCount.value} 位顾客适合召回`,
      content: '可以生成沉睡召回券或复购券，提升再次下单概率。',
      button: '做活动',
      go: () => router.push('/merchant/promotions')
    })
  }
  if (paymentConfig.value.audit_status !== 'approved') {
    list.push({
      label: '收款',
      tone: 'orange',
      title: '收款配置还未审核通过',
      content: '建议完善收款设置并等待平台审核，后续用于收款和财务核对。',
      button: '去配置',
      go: () => router.push('/merchant/payment-settings')
    })
  }
  return list.length ? list.slice(0, 4) : [
    {
      label: '经营',
      tone: 'green',
      title: '当前经营状态平稳',
      content: '可以继续优化热销商品图片、上新套餐，并用 AI 生成下一轮营销活动。',
      button: 'AI建议',
      go: () => router.push('/merchant/ai?scenario=campaign')
    }
  ]
})

const normalizeList = (payload) => {
  if (Array.isArray(payload)) return payload
  if (Array.isArray(payload?.list)) return payload.list
  if (Array.isArray(payload?.data)) return payload.data
  return []
}

const copyDailyReport = async () => {
  await navigator.clipboard.writeText(dailyReportText.value)
  ElMessage.success('经营日报已复制')
}

const exportDailyReport = () => {
  const rows = dailyReportItems.value.map((item) => ({
    日期: today.value,
    指标: item.label,
    数值: item.value,
    说明: item.hint
  }))
  rows.push({
    日期: today.value,
    指标: '今日结论',
    数值: todayBrief.value.title,
    说明: todayBrief.value.content
  })
  exportRowsToXlsx(`商家经营日报_${today.value}.xlsx`, '经营日报', rows)
}

const openDailyAI = () => {
  router.push({
    path: '/merchant/ai',
    query: {
      scenario: 'daily_report',
      source: 'dashboard',
      auto: '1',
      date: today.value,
      orders: todayOrders.value.length,
      receivable: formatMoney(todayReceivableAmount.value),
      confirmed: formatMoney(todayPaidAmount.value),
      unconfirmed: formatMoney(todayUnconfirmedAmount.value),
      net: formatMoney(todayNetIncome.value),
      refund: formatMoney(todayRefundAmount.value),
      avg: formatMoney(todayAvgOrderAmount.value),
      product: topProduct.value,
      pending: statusCount.value.received || 0,
      brief: todayBrief.value.title
    }
  })
}

const actionableCount = (list) => list.filter((item) => (
  ['payment_confirming', 'submitted', 'received', 'pending', 'failed'].includes(item.status)
)).length

const updateRefreshSnapshot = (nextOrders, notify) => {
  const nextCount = actionableCount(nextOrders)
  const previousCount = previousActionCount.value
  previousActionCount.value = nextCount
  lastRefreshAt.value = new Date().toLocaleTimeString('zh-CN', { hour12: false })
  if (notify && previousCount !== null && nextCount > previousCount) {
    ElMessage({
      type: 'warning',
      message: `经营待办增加 ${nextCount - previousCount} 笔，建议进入订单中心处理。`,
      showClose: true,
      duration: 5000
    })
  }
}

const load = async ({ silent = false, notify = false } = {}) => {
  if (!silent) loading.value = true
  try {
    const [infoRes, aiRes, leadsRes, ordersRes, refundsRes, paymentRes] = await Promise.all([
      fetchMerchantInfo(),
      fetchMerchantAIInsights(),
      fetchMerchantLeads({ page: 1, page_size: 5 }),
      fetchMerchantOrders({ page: 1, page_size: 100 }),
      fetchMerchantRefunds(),
      fetchMerchantPaymentConfig()
    ])

    const merchant = infoRes.data?.merchant || infoRes.data || {}
    merchantInfo.value = merchant
    if (merchant?.id) {
      authStore.merchant = merchant
      localStorage.setItem('merchant_current', JSON.stringify(merchant))
    }

    insights.value = aiRes.data || {}
    recentLeads.value = normalizeList(leadsRes.data)
    const nextOrders = normalizeList(ordersRes.data)
    orders.value = nextOrders
    refunds.value = normalizeList(refundsRes.data)
    paymentConfig.value = paymentRes.data?.config || paymentRes.data || {}
    updateRefreshSnapshot(nextOrders, notify)
  } finally {
    if (!silent) loading.value = false
  }
}

const stopAutoRefresh = () => {
  if (!refreshTimer) return
  window.clearInterval(refreshTimer)
  refreshTimer = null
}

const startAutoRefresh = () => {
  stopAutoRefresh()
  if (!autoRefreshEnabled.value) return
  refreshTimer = window.setInterval(() => {
    load({ silent: true, notify: true })
  }, refreshIntervalMs)
}

const toggleAutoRefresh = () => {
  if (autoRefreshEnabled.value) {
    load({ silent: true })
    startAutoRefresh()
    return
  }
  stopAutoRefresh()
}

const orderAmount = (row) => Number(row.paid_amount ?? row.total_amount ?? row.amount ?? 0)
const formatMoney = (value) => `¥${(Number(value || 0) / 100).toFixed(2)}`
const orderStatusLabel = (status) => ({
  pending: '待支付',
  submitted: '已提交',
  payment_confirming: '待确认收款',
  preparing: '处理中',
  received: '待接单',
  accepted: '已接单',
  completed: '已完成',
  closed: '已关闭',
  failed: '支付失败'
}[status] || status || '-')
const orderStatusType = (status) => ({
  pending: 'warning',
  submitted: 'primary',
  payment_confirming: 'warning',
  preparing: 'success',
  received: 'primary',
  accepted: 'success',
  completed: 'success',
  closed: 'info',
  failed: 'danger'
}[status] || 'info')
const leadStatusLabel = (status) => ({
  new: '新线索',
  contacted: '已联系',
  converted: '已成交',
  invalid: '无效'
}[status] || status || '-')
const leadStatusType = (status) => ({
  new: 'warning',
  contacted: 'primary',
  converted: 'success',
  invalid: 'danger'
}[status] || 'info')
const lowProductSuggestion = (row) => (
  Number(row.quantity || 0) === 0
    ? '建议换图、改标题或加入满减活动测试'
    : '建议搭配热销商品做套餐，提高曝光'
)

onMounted(async () => {
  await load()
  startAutoRefresh()
})

onUnmounted(() => {
  stopAutoRefresh()
})
</script>

<style scoped>
.merchant-dashboard {
  display: grid;
  gap: 16px;
}

.metric-grid,
.shortcut-grid,
.main-grid {
  display: grid;
  gap: 14px;
}

.metric-grid {
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.priority-notice {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: center;
  padding: 18px 20px;
  border: 1px solid #fed7aa;
  border-radius: 22px;
  background:
    radial-gradient(circle at 100% 0%, rgba(249, 115, 22, 0.14), transparent 34%),
    linear-gradient(135deg, #fff7ed, #ffffff);
  box-shadow: 0 16px 40px rgba(146, 64, 14, 0.1);
}

.priority-notice.primary {
  border-color: #bfdbfe;
  background:
    radial-gradient(circle at 100% 0%, rgba(37, 99, 235, 0.12), transparent 34%),
    linear-gradient(135deg, #eff6ff, #ffffff);
}

.priority-notice.pay {
  border-color: #fde68a;
  background:
    radial-gradient(circle at 100% 0%, rgba(245, 158, 11, 0.14), transparent 34%),
    linear-gradient(135deg, #fffbeb, #ffffff);
}

.priority-notice span,
.priority-notice strong,
.priority-notice p {
  display: block;
}

.priority-notice span {
  color: #ea580c;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.08em;
}

.priority-notice.primary span {
  color: #2563eb;
}

.priority-notice strong {
  margin-top: 5px;
  color: #0f172a;
  font-size: 22px;
}

.priority-notice p {
  margin: 7px 0 0;
  color: #64748b;
  line-height: 1.6;
}

.dashboard-refresh {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 13px 16px;
  border: 1px solid #dbeafe;
  border-radius: 18px;
  background: #f8fbff;
}

.dashboard-refresh span,
.dashboard-refresh strong,
.dashboard-refresh p {
  display: block;
}

.dashboard-refresh span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.08em;
}

.dashboard-refresh strong {
  margin-top: 4px;
  color: #0f2747;
}

.dashboard-refresh p {
  margin: 4px 0 0;
  color: #64748b;
  line-height: 1.55;
}

.dashboard-refresh-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.dashboard-cashier {
  display: grid;
  gap: 14px;
  padding: 18px 20px;
  border: 1px solid #dbeafe;
  border-radius: 22px;
  background: #ffffff;
  box-shadow: 0 14px 36px rgba(15, 39, 71, 0.07);
}

.dashboard-cashier-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.dashboard-cashier-head span,
.dashboard-cashier-head strong,
.dashboard-cashier-head p,
.cashier-step span,
.cashier-step strong,
.cashier-step small {
  display: block;
}

.dashboard-cashier-head span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.08em;
}

.dashboard-cashier-head strong {
  margin-top: 5px;
  color: #0f2747;
  font-size: 22px;
}

.dashboard-cashier-head p {
  margin: 7px 0 0;
  color: #64748b;
  line-height: 1.65;
}

.cashier-flow {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: 10px;
}

.cashier-step {
  min-height: 108px;
  padding: 14px;
  text-align: left;
  border: 1px solid #e2e8f0;
  border-radius: 16px;
  background: #f8fafc;
  cursor: pointer;
  transition: 0.2s ease;
}

.cashier-step:hover {
  transform: translateY(-1px);
  box-shadow: 0 12px 28px rgba(15, 23, 42, 0.08);
}

.cashier-step span {
  color: #64748b;
  font-size: 12px;
  font-weight: 800;
}

.cashier-step strong {
  margin-top: 8px;
  color: #0f172a;
  font-size: 19px;
  line-height: 1.2;
}

.cashier-step small {
  margin-top: 7px;
  color: #64748b;
  line-height: 1.45;
}

.cashier-step.primary {
  border-color: #bfdbfe;
  background: #eff6ff;
}

.cashier-step.success {
  border-color: #bbf7d0;
  background: #ecfdf5;
}

.cashier-step.warning {
  border-color: #fed7aa;
  background: #fff7ed;
}

.cashier-step.danger {
  border-color: #fecaca;
  background: #fef2f2;
}

.cashier-step.muted {
  opacity: 0.82;
}

.today-brief {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 18px 20px;
  border: 1px solid #bfdbfe;
  border-radius: 24px;
  background:
    radial-gradient(circle at 100% 0%, rgba(14, 165, 233, 0.16), transparent 34%),
    linear-gradient(135deg, #eff6ff, #ffffff);
  box-shadow: 0 16px 40px rgba(15, 39, 71, 0.08);
}

.today-brief span,
.today-brief strong,
.today-brief p {
  display: block;
}

.today-brief span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.08em;
}

.today-brief strong {
  margin-top: 6px;
  color: #0f2747;
  font-size: 22px;
}

.today-brief p {
  margin: 8px 0 0;
  color: #64748b;
  line-height: 1.65;
}

.fulfillment-board {
  padding: 18px 20px;
  border: 1px solid #dbeafe;
  border-radius: 22px;
  background: #ffffff;
  box-shadow: 0 14px 36px rgba(15, 39, 71, 0.07);
}

.fulfillment-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
  margin-bottom: 14px;
}

.fulfillment-head span,
.fulfillment-head strong,
.fulfillment-head p {
  display: block;
}

.fulfillment-head span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.08em;
}

.fulfillment-head strong {
  margin-top: 5px;
  color: #0f2747;
  font-size: 20px;
}

.fulfillment-head p {
  margin: 6px 0 0;
  color: #64748b;
  line-height: 1.55;
}

.fulfillment-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}

.fulfillment-card {
  min-height: 96px;
  padding: 13px;
  text-align: left;
  border: 1px solid #e2e8f0;
  border-radius: 16px;
  background: #f8fafc;
  cursor: pointer;
  transition: 0.2s ease;
}

.fulfillment-card:hover {
  transform: translateY(-1px);
  box-shadow: 0 12px 28px rgba(15, 23, 42, 0.08);
}

.fulfillment-card span,
.fulfillment-card strong,
.fulfillment-card small {
  display: block;
}

.fulfillment-card span {
  color: #64748b;
  font-size: 12px;
  font-weight: 800;
}

.fulfillment-card strong {
  margin: 7px 0 5px;
  color: #0f172a;
  font-size: 24px;
  line-height: 1;
}

.fulfillment-card small {
  color: #64748b;
  line-height: 1.45;
}

.fulfillment-card.primary {
  border-color: #bfdbfe;
  background: #eff6ff;
}

.fulfillment-card.success {
  border-color: #bbf7d0;
  background: #ecfdf5;
}

.fulfillment-card.warning {
  border-color: #fed7aa;
  background: #fff7ed;
}

.fulfillment-card.danger {
  border-color: #fecaca;
  background: #fef2f2;
}

.fulfillment-card.muted {
  opacity: 0.8;
}

.daily-report {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.primary-report {
  grid-template-columns: repeat(6, minmax(0, 1fr));
}

.compact-report {
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.daily-more {
  margin-top: 10px;
  border: 1px solid #e2e8f0;
  border-radius: 14px;
  overflow: hidden;
}

.daily-item {
  min-height: 92px;
  padding: 14px;
  border: 1px solid #dbeafe;
  border-radius: 18px;
  background: linear-gradient(135deg, #f8fbff, #ffffff);
}

.daily-item.compact {
  min-height: 82px;
}

.daily-item span,
.daily-item strong,
.daily-item p {
  display: block;
}

.daily-item span {
  color: #64748b;
  font-size: 12px;
}

.daily-item strong {
  margin-top: 6px;
  color: #0f2747;
  font-size: 20px;
  line-height: 1.15;
}

.daily-item p {
  margin: 8px 0 0;
  color: #64748b;
  font-size: 12px;
  line-height: 1.55;
}

.report-summary {
  margin-top: 12px;
  padding: 14px 16px;
  color: #0f2747;
  line-height: 1.75;
  white-space: pre-line;
  border: 1px solid #bfdbfe;
  border-radius: 18px;
  background:
    radial-gradient(circle at 100% 0%, rgba(14, 165, 233, 0.12), transparent 34%),
    #eff6ff;
}

.shortcut-grid {
  grid-template-columns: repeat(5, minmax(0, 1fr));
}

.main-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
  align-items: start;
}

.shortcut-card {
  min-height: 104px;
  padding: 15px 16px;
  text-align: left;
  border: 1px solid #dbeafe;
  border-radius: 18px;
  background: #f8fbff;
  box-shadow: 0 12px 32px rgba(15, 39, 71, 0.06);
  cursor: pointer;
  transition: 0.2s ease;
}

.shortcut-card:hover {
  transform: translateY(-2px);
  border-color: #93c5fd;
  box-shadow: 0 18px 38px rgba(15, 39, 71, 0.1);
}

.shortcut-card.urgent {
  background:
    radial-gradient(circle at 100% 0%, rgba(37, 99, 235, 0.13), transparent 36%),
    #eff6ff;
}

.shortcut-card.warning {
  border-color: #fed7aa;
  background: #fff7ed;
}

.shortcut-card span,
.shortcut-card strong,
.shortcut-card small {
  display: block;
}

.shortcut-card span {
  color: #64748b;
  font-size: 12px;
}

.shortcut-card strong {
  margin: 7px 0 5px;
  color: #0f2747;
  font-size: 24px;
  line-height: 1;
}

.shortcut-card small {
  color: #64748b;
  line-height: 1.45;
}

.action-list {
  display: grid;
  gap: 10px;
}

.customer-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.customer-grid div {
  padding: 14px;
  border-radius: 16px;
  border: 1px solid #e5edf9;
  background: linear-gradient(135deg, #f8fbff, #ffffff);
}

.customer-grid span {
  display: block;
  color: #64748b;
  font-size: 12px;
}

.customer-grid strong {
  display: block;
  margin-top: 6px;
  color: #0f2747;
  font-size: 24px;
}

.hint-box {
  margin-top: 12px;
  padding: 12px 14px;
  color: #0f766e;
  line-height: 1.65;
  border: 1px solid #bbf7d0;
  border-radius: 16px;
  background: #ecfdf5;
}

@media (max-width: 1180px) {
  .metric-grid,
  .shortcut-grid,
  .fulfillment-grid,
  .daily-report {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .main-grid {
    grid-template-columns: 1fr;
  }

  .today-brief,
  .priority-notice,
  .dashboard-refresh,
  .dashboard-cashier-head {
    align-items: flex-start;
    flex-direction: column;
  }

  .fulfillment-head {
    flex-direction: column;
  }

  .cashier-flow {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 720px) {
  .metric-grid,
  .shortcut-grid,
  .fulfillment-grid,
  .main-grid,
  .customer-grid,
  .daily-report,
  .cashier-flow {
    grid-template-columns: 1fr;
  }
}
</style>
