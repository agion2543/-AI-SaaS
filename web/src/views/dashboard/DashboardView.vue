<template>
  <div class="dashboard-page">
    <PageHero
      eyebrow="SAAS OPERATIONS"
      title="平台运营总览"
      description="默认展示今日数据，可切换任意时间段，统一查看平台订阅收入、顾客交易流水、退款、商家排行、结算与审计。"
      compact
    >
      <template #actions>
        <el-date-picker
          v-model="dateRange"
          type="daterange"
          unlink-panels
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          value-format="YYYY-MM-DD"
        />
        <el-button type="primary" :loading="loading" @click="load">查看</el-button>
        <el-button plain @click="useToday">今日</el-button>
      </template>
    </PageHero>

    <section class="metric-grid">
      <MetricCard
        v-for="item in cards"
        :key="item.label"
        :label="item.label"
        :value="item.value"
        :hint="item.hint"
        :tone="item.tone"
      />
    </section>

    <section class="action-strip">
      <ActionCard
        v-for="item in actionCards"
        :key="item.title"
        :label="item.kicker"
        :title="item.value"
        :description="item.title"
        :tone="item.tone"
      >
        <template #actions>
          <el-button size="small" :type="item.buttonType" plain @click="item.action">{{ item.buttonText }}</el-button>
        </template>
      </ActionCard>
    </section>

    <section class="risk-overview">
      <div
        v-for="item in riskSummaryCards"
        :key="item.label"
        class="risk-summary-card"
        :class="`risk-summary-card--${item.tone}`"
      >
        <span>{{ item.kicker }}</span>
        <strong>{{ item.value }}</strong>
        <p>{{ item.label }}</p>
        <small>{{ item.hint }}</small>
      </div>
    </section>

    <section class="chart-grid">
      <DataPanel title="平台收入结构" :description="`当前统计周期：${rangeText}`" eyebrow="REVENUE MIX">
        <div class="donut-wrap">
          <div class="donut" :style="{ background: revenueDonutGradient }">
            <div class="donut-hole">
              <strong>{{ formatMoney(rangeRevenue) }}</strong>
              <span>时间段流水</span>
            </div>
          </div>
          <div class="legend-list">
            <div v-for="item in revenueMixRows" :key="item.type" class="legend-item">
              <i :style="{ background: item.color }"></i>
              <span>{{ item.label }}</span>
              <strong>{{ formatMoney(item.amount) }}</strong>
            </div>
            <div class="legend-item refund-line">
              <i style="background:#ef4444"></i>
              <span>退款金额</span>
              <strong>-{{ formatMoney(dashboard.range_refund_amount) }}</strong>
            </div>
            <div class="legend-item net-line">
              <i style="background:#0f172a"></i>
              <span>净额参考</span>
              <strong>{{ formatMoney(dashboard.range_net_revenue) }}</strong>
            </div>
          </div>
        </div>
      </DataPanel>

      <DataPanel title="收入趋势" description="按日展示商家订阅收入与顾客扫码交易流水趋势。" eyebrow="TREND">
        <div class="bar-chart">
          <div v-for="item in trendBars" :key="item.date" class="bar-item">
            <div class="bar-value">{{ formatCompactMoney(item.total) }}</div>
            <div class="stack-track">
              <div class="stack-fill merchant-fill" :style="{ height: `${item.merchantPercent}%` }"></div>
              <div class="stack-fill trade-fill" :style="{ height: `${item.tradePercent}%` }"></div>
            </div>
            <div class="bar-label">{{ shortDate(item.date) }}</div>
          </div>
          <el-empty v-if="!trendBars.length" description="暂无趋势数据" />
        </div>
      </DataPanel>
    </section>

    <DataPanel
      title="商家经营排行"
      description="默认展示前 20 名，按交易额和订单数排序，帮助平台快速识别高价值商家与经营异常。"
      eyebrow="MERCHANT RANKING"
    >
      <el-table :data="merchantOpsRows" empty-text="暂无商家经营数据">
        <el-table-column label="排名" width="80">
          <template #default="{ $index }">#{{ $index + 1 }}</template>
        </el-table-column>
        <el-table-column prop="merchant_name" label="商家" min-width="160" />
        <el-table-column prop="order_count" label="订单数" width="100" />
        <el-table-column label="交易额" width="130">
          <template #default="{ row }">{{ formatMoney(row.trade_amount) }}</template>
        </el-table-column>
        <el-table-column label="退款" width="120">
          <template #default="{ row }">{{ formatMoney(row.refund_amount) }}</template>
        </el-table-column>
        <el-table-column label="净收入" width="130">
          <template #default="{ row }"><strong>{{ formatMoney(row.net_amount) }}</strong></template>
        </el-table-column>
        <el-table-column label="客单价" width="130">
          <template #default="{ row }">{{ formatMoney(row.avg_order_amount) }}</template>
        </el-table-column>
        <el-table-column label="接单率" width="120">
          <template #default="{ row }">{{ percent(row.accept_rate) }}</template>
        </el-table-column>
        <el-table-column label="取消率" width="120">
          <template #default="{ row }">
            <el-tag :type="row.cancel_rate > 0.3 ? 'danger' : 'info'">{{ percent(row.cancel_rate) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="风险" width="120">
          <template #default="{ row }">
            <el-tag :type="riskLevel(row).type">{{ riskLevel(row).label }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="120">
          <template #default="{ row }">
            <el-tag :type="row.status === 'active' ? 'success' : 'danger'">{{ merchantStatus(row.status) }}</el-tag>
          </template>
        </el-table-column>
      </el-table>
    </DataPanel>

    <section class="chart-grid">
      <DataPanel title="商家风险预警" description="按退款率、取消率、接单率、净额和冻结状态综合判断，优先处理高风险商家。" eyebrow="RISK">
        <el-table :data="riskRows" empty-text="暂无异常商家">
          <el-table-column prop="merchant_name" label="商家" min-width="150" />
          <el-table-column label="等级" width="90">
            <template #default="{ row }">
              <el-tag :type="riskLevel(row).type">{{ riskLevel(row).label }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="order_count" label="订单" width="80" />
          <el-table-column label="退款率" width="100">
            <template #default="{ row }">
              <el-tag :type="row.refund_rate > 0.2 ? 'danger' : 'warning'">{{ percent(row.refund_rate) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="取消率" width="100">
            <template #default="{ row }">
              <el-tag :type="row.cancel_rate > 0.3 ? 'danger' : 'warning'">{{ percent(row.cancel_rate) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="原因" min-width="180">
            <template #default="{ row }">
              <div class="risk-tags">
                <el-tag v-for="tag in riskTags(row)" :key="tag" size="small" type="warning">{{ tag }}</el-tag>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="处理建议" min-width="200">
            <template #default="{ row }">{{ riskAdvice(row) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="100" fixed="right">
            <template #default="{ row }">
              <el-button size="small" text type="primary" @click="router.push(`/admin/merchants/${row.merchant_id}`)">跟进</el-button>
            </template>
          </el-table-column>
        </el-table>
      </DataPanel>

      <DataPanel title="结算与退款巡检" description="把人工结算和退款记录放在首页，方便每天核对资金与售后状态。" eyebrow="FINANCE PATROL">
        <el-tabs>
          <el-tab-pane label="最近结算">
            <el-table :data="dashboard.settlement_patrol || []" empty-text="暂无结算记录">
              <el-table-column prop="merchant_name" label="商家" min-width="130" />
              <el-table-column prop="order_count" label="订单数" width="90" />
              <el-table-column label="净结算" width="120">
                <template #default="{ row }">{{ formatMoney(row.net_amount_cents) }}</template>
              </el-table-column>
              <el-table-column label="状态" width="100">
                <template #default="{ row }">
                  <el-tag :type="row.status === 'paid' ? 'success' : 'warning'">
                    {{ row.status === 'paid' ? '已付款' : '待付款' }}
                  </el-tag>
                </template>
              </el-table-column>
            </el-table>
          </el-tab-pane>
          <el-tab-pane label="最近退款">
            <el-table :data="dashboard.refund_patrol || []" empty-text="暂无退款记录">
              <el-table-column prop="refund_no" label="退款号" min-width="160" />
              <el-table-column prop="merchant_name" label="商家" min-width="120" />
              <el-table-column label="金额" width="110">
                <template #default="{ row }">{{ formatMoney(row.amount) }}</template>
              </el-table-column>
              <el-table-column prop="reason" label="原因" min-width="160" show-overflow-tooltip />
            </el-table>
          </el-tab-pane>
        </el-tabs>
      </DataPanel>
    </section>

    <section class="chart-grid">
      <DataPanel title="平台商品 / 订单巡检" description="快速查看最近商品与订单，后续可接入投诉、异常支付和敏感词巡检。" eyebrow="PATROL">
        <el-tabs>
          <el-tab-pane label="最近商品">
            <el-table :data="dashboard.product_patrol || []" empty-text="暂无商品">
              <el-table-column prop="merchant_name" label="商家" min-width="120" />
              <el-table-column prop="store_name" label="门店" min-width="120" />
              <el-table-column prop="product_name" label="商品" min-width="140" />
              <el-table-column label="价格" width="100">
                <template #default="{ row }">{{ formatMoney(row.price) }}</template>
              </el-table-column>
              <el-table-column prop="status" label="状态" width="90" />
            </el-table>
          </el-tab-pane>
          <el-tab-pane label="最近订单">
            <el-table :data="dashboard.order_patrol || []" empty-text="暂无订单">
              <el-table-column prop="order_no" label="订单号" min-width="180" />
              <el-table-column prop="merchant_name" label="商家" min-width="120" />
              <el-table-column prop="store_name" label="门店" min-width="120" />
              <el-table-column label="金额" width="100">
                <template #default="{ row }">{{ formatMoney(row.total_amount) }}</template>
              </el-table-column>
              <el-table-column prop="status" label="状态" width="100" />
            </el-table>
          </el-tab-pane>
        </el-tabs>
      </DataPanel>

      <DataPanel title="最近操作审计" description="关键动作留痕，包括审核收款、开通订阅、冻结商家、退款售后等。" eyebrow="AUDIT">
        <template #actions>
          <el-button size="small" plain @click="router.push('/admin/audit-logs')">查看全部</el-button>
        </template>
        <el-table :data="dashboard.recent_audit_logs || []" empty-text="暂无审计记录">
          <el-table-column prop="actor_name" label="操作人" width="120" />
          <el-table-column prop="action" label="动作" min-width="150" />
          <el-table-column prop="target_name" label="对象" min-width="150" show-overflow-tooltip />
          <el-table-column label="时间" width="160">
            <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
          </el-table-column>
        </el-table>
      </DataPanel>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { fetchDashboard } from '../../api/modules'
import ActionCard from '../../components/design/ActionCard.vue'
import DataPanel from '../../components/design/DataPanel.vue'
import MetricCard from '../../components/design/MetricCard.vue'
import PageHero from '../../components/design/PageHero.vue'

const router = useRouter()
const now = new Date()
const today = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`
const loading = ref(false)
const dateRange = ref([today, today])
const dashboard = ref({})
const colors = ['#2563eb', '#16a34a', '#f97316', '#9333ea']

const load = async () => {
  loading.value = true
  try {
    const [start, end] = dateRange.value?.length === 2 ? dateRange.value : [today, today]
    const res = await fetchDashboard({ start_date: start, end_date: end })
    dashboard.value = res.data || {}
  } finally {
    loading.value = false
  }
}

const useToday = () => {
  dateRange.value = [today, today]
  load()
}

const formatMoney = (value) => `¥${(Number(value || 0) / 100).toFixed(2)}`
const formatCompactMoney = (value) => {
  const yuan = Number(value || 0) / 100
  return yuan >= 10000 ? `¥${(yuan / 10000).toFixed(1)}万` : `¥${yuan.toFixed(0)}`
}
const percent = (value) => `${Math.round(Number(value || 0) * 100)}%`
const shortDate = (date) => String(date || '').slice(5)
const formatDate = (date) => date ? String(date).replace('T', ' ').slice(0, 19) : '-'

const rangeText = computed(() => {
  const [start, end] = dateRange.value?.length === 2 ? dateRange.value : [today, today]
  return `${start} 至 ${end}`
})
const rangeRevenue = computed(() => Number(dashboard.value.range_revenue || 0))

const cards = computed(() => [
  { label: '商家总数', value: dashboard.value.merchant_count || 0, hint: '平台入驻商家规模', tone: 'primary' },
  { label: '平台订阅收入', value: formatMoney(dashboard.value.platform_subscription_revenue), hint: '商家 SaaS 月付/年付收入', tone: 'primary' },
  { label: '顾客交易流水', value: formatMoney(dashboard.value.customer_trade_revenue), hint: '顾客扫码点单支付流水', tone: 'success' },
  { label: '累计退款金额', value: formatMoney(dashboard.value.total_refund_amount), hint: '售后退款留痕金额', tone: 'warning' },
  { label: '顾客交易净额', value: formatMoney(dashboard.value.customer_trade_net), hint: '顾客流水 - 退款金额', tone: 'cyan' },
  { label: '时间段净额', value: formatMoney(dashboard.value.range_net_revenue), hint: '当前筛选时间段收入净额', tone: '' }
])

const actionCards = computed(() => [
  {
    kicker: 'SETTLEMENT',
    title: '待人工结算金额',
    value: `${dashboard.value.pending_settlement_count || 0} 笔 / ${formatMoney(dashboard.value.pending_settlement_amount)}`,
    buttonText: '去商家管理',
    buttonType: 'primary',
    tone: 'blue',
    action: () => router.push('/admin/merchants')
  },
  {
    kicker: 'REFUND',
    title: '当前时间段退款记录',
    value: `${dashboard.value.range_refund_count || 0} 笔 / ${formatMoney(dashboard.value.range_refund_amount)}`,
    buttonText: '去订单财务',
    buttonType: 'warning',
    tone: 'orange',
    action: () => router.push('/admin/orders')
  },
  {
    kicker: 'AUDIT',
    title: '最近关键操作留痕',
    value: `${(dashboard.value.recent_audit_logs || []).length} 条`,
    buttonText: '查看审计',
    buttonType: 'info',
    tone: 'slate',
    action: () => router.push('/admin/audit-logs')
  }
])

const countByStatus = (rows, status) => rows.filter((item) => item.status === status).length
const orderPatrolRows = computed(() => dashboard.value.order_patrol || [])
const settlementPatrolRows = computed(() => dashboard.value.settlement_patrol || [])

const revenueMixRows = computed(() => {
  const labelMap = {
    merchant_subscription: '商家订阅收入',
    store_order: '顾客交易流水',
    user_membership: '用户会员充值'
  }
  return (dashboard.value.revenue_mix || []).map((item, index) => ({
    type: item.order_type || 'unknown',
    label: labelMap[item.order_type] || item.order_type || '其他收入',
    amount: Number(item.amount || 0),
    count: Number(item.count || 0),
    color: colors[index % colors.length]
  }))
})

const revenueDonutGradient = computed(() => {
  const total = revenueMixRows.value.reduce((sum, item) => sum + item.amount, 0)
  if (!total) return 'conic-gradient(#e2e8f0 0 100%)'
  let cursor = 0
  return `conic-gradient(${revenueMixRows.value.map((item) => {
    const start = cursor
    const size = (item.amount / total) * 100
    cursor += size
    return `${item.color} ${start}% ${cursor}%`
  }).join(', ')})`
})

const trendBars = computed(() => {
  const map = new Map()
  ;(dashboard.value.merchant_revenue_trend || []).forEach((item) => {
    map.set(item.date, { date: item.date, merchant: Number(item.amount || 0), trade: 0 })
  })
  ;(dashboard.value.store_order_trend || []).forEach((item) => {
    const row = map.get(item.date) || { date: item.date, merchant: 0, trade: 0 }
    row.trade = Number(item.amount || 0)
    map.set(item.date, row)
  })
  const list = Array.from(map.values()).sort((a, b) => String(a.date).localeCompare(String(b.date)))
  const max = Math.max(...list.map((item) => item.merchant + item.trade), 1)
  return list.map((item) => {
    const total = item.merchant + item.trade
    return {
      ...item,
      total,
      merchantPercent: Math.max(0, Math.round((item.merchant / max) * 100)),
      tradePercent: Math.max(total > 0 ? 6 : 0, Math.round((item.trade / max) * 100))
    }
  })
})

const merchantOpsRows = computed(() => (dashboard.value.merchant_ops || []).map((row) => {
  const orderCount = Number(row.order_count || 0)
  const acceptedCount = Number(row.accepted_count || 0)
  const closedCount = Number(row.closed_count || 0)
  const tradeAmount = Number(row.trade_amount || 0)
  const refundAmount = Number(row.refund_amount || 0)
  const netAmount = tradeAmount - refundAmount
  return {
    ...row,
    trade_amount: tradeAmount,
    refund_amount: refundAmount,
    net_amount: Math.max(netAmount, 0),
    avg_order_amount: Number(row.avg_order_amount || 0),
    accept_rate: orderCount ? acceptedCount / orderCount : 0,
    cancel_rate: orderCount ? closedCount / orderCount : 0,
    refund_rate: tradeAmount ? refundAmount / tradeAmount : 0,
    net_negative: netAmount < 0
  }
}).slice(0, 20))

const riskTags = (row) => {
  const tags = []
  const orderCount = Number(row.order_count || 0)
  if (row.status === 'suspended') tags.push('商家冻结')
  if (row.refund_rate > 0.2) tags.push('退款偏高')
  if (row.cancel_rate > 0.3) tags.push('取消偏高')
  if (orderCount >= 3 && row.accept_rate < 0.5) tags.push('接单偏低')
  if (row.net_negative || (Number(row.trade_amount || 0) > 0 && Number(row.net_amount || 0) <= 0)) tags.push('净额异常')
  if (!orderCount && row.status !== 'active') tags.push('状态待处理')
  return tags
}

const riskLevel = (row) => {
  const tags = riskTags(row)
  if (row.status === 'suspended' || row.refund_rate > 0.35 || row.cancel_rate > 0.5 || row.net_negative) {
    return { label: '高风险', type: 'danger', score: 3 }
  }
  if (tags.length >= 2 || row.refund_rate > 0.2 || row.cancel_rate > 0.3) {
    return { label: '中风险', type: 'warning', score: 2 }
  }
  if (tags.length) return { label: '观察', type: 'info', score: 1 }
  return { label: '正常', type: 'success', score: 0 }
}

const riskRows = computed(() => merchantOpsRows.value
  .filter((row) => riskLevel(row).score > 0)
  .sort((a, b) => riskLevel(b).score - riskLevel(a).score || Number(b.trade_amount || 0) - Number(a.trade_amount || 0)))

const riskSummaryCards = computed(() => {
  const pendingOrders = countByStatus(orderPatrolRows.value, 'pending')
  const failedOrders = countByStatus(orderPatrolRows.value, 'failed')
  const receivedOrders = countByStatus(orderPatrolRows.value, 'received')
  const pendingSettlements = settlementPatrolRows.value.filter((item) => item.status !== 'paid')
  const highRiskCount = merchantOpsRows.value.filter((row) => riskLevel(row).score >= 3).length
  const refundRiskCount = merchantOpsRows.value.filter((row) => row.refund_rate > 0.2).length
  return [
    {
      kicker: 'HIGH RISK',
      label: '高风险商家',
      value: `${highRiskCount} 家`,
      hint: '冻结、退款过高、取消过高或净额异常',
      tone: highRiskCount ? 'danger' : 'safe'
    },
    {
      kicker: 'REFUND',
      label: '退款率偏高商家',
      value: `${refundRiskCount} 家`,
      hint: '退款率超过 20%，建议核对售后原因',
      tone: refundRiskCount ? 'warning' : 'safe'
    },
    {
      kicker: 'ORDER',
      label: '支付/履约待处理',
      value: `${pendingOrders + failedOrders + receivedOrders} 单`,
      hint: `待支付 ${pendingOrders}，支付失败 ${failedOrders}，待接单 ${receivedOrders}`,
      tone: pendingOrders + failedOrders + receivedOrders ? 'warning' : 'safe'
    },
    {
      kicker: 'SETTLEMENT',
      label: '待结算巡检',
      value: `${pendingSettlements.length} 笔`,
      hint: `待结算金额 ${formatMoney(dashboard.value.pending_settlement_amount)}`,
      tone: pendingSettlements.length ? 'primary' : 'safe'
    }
  ]
})

const merchantStatus = (status) => ({ pending: '待审核', active: '正常', suspended: '已冻结' }[status] || status || '-')
const riskAdvice = (row) => {
  if (row.status === 'suspended') return '先复核冻结原因，确认是否恢复营业或下线商家'
  if (row.refund_rate > 0.35 || row.net_negative) return '优先核对退款记录、顾客投诉和结算金额'
  if (row.cancel_rate > 0.5) return '联系商家确认库存、营业时间和接单流程'
  if (row.refund_rate > 0.2) return '查看退款原因，必要时暂停活动投放'
  if (row.cancel_rate > 0.3) return '巡检订单取消原因，优化商品可售状态'
  if (Number(row.order_count || 0) >= 3 && row.accept_rate < 0.5) return '提醒商家及时接单，检查通知和人员排班'
  if (Number(row.order_count || 0) === 0) return '暂无订单，观察推广'
  return '正常观察'
}

onMounted(load)
</script>

<style scoped>
.dashboard-page {
  display: grid;
  gap: 20px;
}

.metric-grid,
.chart-grid,
.action-strip {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 18px;
}

.risk-overview {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
}

.risk-summary-card {
  display: grid;
  gap: 6px;
  min-height: 132px;
  padding: 18px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #fff;
  box-shadow: 0 12px 32px rgba(15, 23, 42, 0.06);
}

.risk-summary-card span {
  color: #64748b;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.08em;
}

.risk-summary-card strong {
  color: #0f172a;
  font-size: 28px;
  line-height: 1;
}

.risk-summary-card p {
  margin: 0;
  color: #1e293b;
  font-size: 14px;
  font-weight: 900;
}

.risk-summary-card small {
  color: #64748b;
  line-height: 1.55;
}

.risk-summary-card--danger {
  border-color: #fecaca;
  background: #fff7f7;
}

.risk-summary-card--warning {
  border-color: #fed7aa;
  background: #fffaf3;
}

.risk-summary-card--primary {
  border-color: #bfdbfe;
  background: #f8fbff;
}

.risk-summary-card--safe {
  border-color: #bbf7d0;
  background: #f8fff9;
}

.risk-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.metric-grid {
  grid-template-columns: repeat(6, minmax(0, 1fr));
}

.chart-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.donut-wrap {
  display: grid;
  grid-template-columns: 220px minmax(0, 1fr);
  gap: 24px;
  align-items: center;
}

.donut {
  width: 220px;
  height: 220px;
  display: grid;
  place-items: center;
  border-radius: 50%;
}

.donut-hole {
  width: 132px;
  height: 132px;
  display: grid;
  place-items: center;
  align-content: center;
  border-radius: 50%;
  background: #fff;
  box-shadow: inset 0 0 0 1px #e2e8f0;
}

.donut-hole strong {
  font-size: 22px;
}

.donut-hole span {
  color: #64748b;
  font-size: 13px;
}

.legend-list {
  display: grid;
  gap: 12px;
}

.legend-item {
  display: grid;
  grid-template-columns: auto 1fr auto;
  gap: 10px;
  align-items: center;
}

.legend-item i {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.refund-line,
.net-line {
  padding-top: 8px;
  border-top: 1px solid #e2e8f0;
}

.bar-chart {
  display: flex;
  align-items: end;
  gap: 14px;
  min-height: 260px;
  overflow-x: auto;
}

.bar-item {
  min-width: 58px;
  display: grid;
  justify-items: center;
  gap: 8px;
}

.stack-track {
  width: 36px;
  height: 180px;
  display: flex;
  flex-direction: column-reverse;
  align-items: stretch;
  overflow: hidden;
  border-radius: 999px;
  background: #e2e8f0;
}

.stack-fill {
  width: 100%;
}

.merchant-fill {
  background: #2563eb;
}

.trade-fill {
  background: #22c55e;
}

.bar-value,
.bar-label {
  color: #475569;
  font-size: 12px;
  font-weight: 800;
}

@media (max-width: 860px) {
  .metric-grid,
  .chart-grid,
  .action-strip,
  .risk-overview {
    grid-template-columns: 1fr;
  }

  .donut-wrap {
    grid-template-columns: 1fr;
    justify-items: center;
  }
}

@media (min-width: 861px) and (max-width: 1380px) {
  .metric-grid,
  .risk-overview {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}
</style>
