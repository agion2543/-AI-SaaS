<template>
  <div class="dashboard-stack">
    <section class="dashboard-hero page-card">
      <div>
        <div class="eyebrow">SAAS OPERATIONS</div>
        <h2 class="page-title">平台运营总览</h2>
        <p class="muted">默认展示今日数据，可切换任意时间段，统一查看平台收入、顾客交易流水、退款、商家排行与风控巡检。</p>
      </div>
      <div class="range-tools">
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
        <el-button @click="useToday">今日</el-button>
      </div>
    </section>

    <section class="stat-grid">
      <div v-for="item in cards" :key="item.label" class="stat-card" :class="item.className">
        <div class="stat-label">{{ item.label }}</div>
        <div class="stat-value">{{ item.value }}</div>
        <div class="stat-hint">{{ item.hint }}</div>
      </div>
    </section>

    <section class="chart-grid">
      <div class="page-card chart-card">
        <div class="chart-title-row">
          <div>
            <h3>平台收入结构</h3>
            <p>商家订阅收入、顾客交易流水、退款和净额分开看，避免平台收入与商家流水混在一起。</p>
          </div>
          <el-tag type="success">{{ rangeText }}</el-tag>
        </div>
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
      </div>

      <div class="page-card chart-card">
        <div class="chart-title-row">
          <div>
            <h3>收入趋势</h3>
            <p>按日展示商家订阅收入与顾客扫码交易流水趋势。</p>
          </div>
          <el-tag type="primary">趋势图</el-tag>
        </div>
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
      </div>
    </section>

    <section class="page-card chart-card">
      <div class="chart-title-row">
        <div>
          <h3>商家经营排行</h3>
          <p>默认展示前 20 名，按交易额和订单数排序，帮助平台快速识别高价值商家与运营异常。</p>
        </div>
        <el-tag type="warning">经营排行</el-tag>
      </div>
      <el-table :data="merchantOpsRows" class="table-block" empty-text="暂无商家经营数据">
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
        <el-table-column label="状态" width="120">
          <template #default="{ row }">
            <el-tag :type="row.status === 'active' ? 'success' : 'danger'">{{ merchantStatus(row.status) }}</el-tag>
          </template>
        </el-table-column>
      </el-table>
    </section>

    <section class="chart-grid">
      <div class="page-card chart-card">
        <div class="chart-title-row">
          <div>
            <h3>商家风控与状态管理</h3>
            <p>重点关注取消率异常、订单量异常或已冻结商家，后续可接入投诉与异常支付记录。</p>
          </div>
          <el-tag type="danger">风控</el-tag>
        </div>
        <el-table :data="riskRows" class="table-block" empty-text="暂无异常商家">
          <el-table-column prop="merchant_name" label="商家" min-width="150" />
          <el-table-column prop="order_count" label="订单" width="80" />
          <el-table-column prop="closed_count" label="取消" width="80" />
          <el-table-column label="取消率" width="110">
            <template #default="{ row }">
              <el-tag :type="row.cancel_rate > 0.3 ? 'danger' : 'warning'">{{ percent(row.cancel_rate) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="建议" min-width="170">
            <template #default="{ row }">{{ riskAdvice(row) }}</template>
          </el-table-column>
        </el-table>
      </div>

      <div class="page-card chart-card">
        <div class="chart-title-row">
          <div>
            <h3>平台商品 / 订单巡检</h3>
            <p>快速查看最近商品与订单，后续可接入投诉、异常支付和敏感词巡检。</p>
          </div>
          <el-tag type="info">巡检</el-tag>
        </div>
        <el-tabs>
          <el-tab-pane label="最近商品">
            <el-table :data="dashboard.product_patrol || []" class="table-block" empty-text="暂无商品">
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
            <el-table :data="dashboard.order_patrol || []" class="table-block" empty-text="暂无订单">
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
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { fetchDashboard } from '../../api/modules'

const today = new Date().toISOString().slice(0, 10)
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

const rangeText = computed(() => {
  const [start, end] = dateRange.value?.length === 2 ? dateRange.value : [today, today]
  return `${start} 至 ${end}`
})
const rangeRevenue = computed(() => Number(dashboard.value.range_revenue || 0))

const cards = computed(() => [
  { label: '商家总数', value: dashboard.value.merchant_count || 0, hint: '平台入驻商家规模', className: '' },
  { label: '平台订阅收入', value: formatMoney(dashboard.value.platform_subscription_revenue), hint: '商家 SaaS 月付/年付收入', className: 'primary' },
  { label: '顾客交易流水', value: formatMoney(dashboard.value.customer_trade_revenue), hint: '顾客扫码点单支付流水', className: 'success' },
  { label: '累计退款金额', value: formatMoney(dashboard.value.total_refund_amount), hint: '售后退款留痕金额', className: 'warning' },
  { label: '顾客交易净额', value: formatMoney(dashboard.value.customer_trade_net), hint: '顾客流水 - 退款金额', className: 'dark' },
  { label: '时间段净额', value: formatMoney(dashboard.value.range_net_revenue), hint: '当前筛选时间段收入净额', className: '' }
])

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
  return {
    ...row,
    trade_amount: tradeAmount,
    refund_amount: refundAmount,
    net_amount: Math.max(tradeAmount - refundAmount, 0),
    avg_order_amount: Number(row.avg_order_amount || 0),
    accept_rate: orderCount ? acceptedCount / orderCount : 0,
    cancel_rate: orderCount ? closedCount / orderCount : 0
  }
}).slice(0, 20))

const riskRows = computed(() => (dashboard.value.risk_merchants || []).map((row) => {
  const orderCount = Number(row.order_count || 0)
  const closedCount = Number(row.closed_count || 0)
  return {
    ...row,
    cancel_rate: orderCount ? closedCount / orderCount : 0
  }
}))

const merchantStatus = (status) => ({ pending: '待审核', active: '正常', suspended: '已冻结' }[status] || status || '-')
const riskAdvice = (row) => {
  if (row.status === 'suspended') return '已冻结，等待复核'
  if (row.cancel_rate > 0.3) return '取消率偏高，建议巡检订单'
  if (Number(row.order_count || 0) === 0) return '暂无订单，观察推广'
  return '正常观察'
}

onMounted(load)
</script>

<style scoped>
.dashboard-stack {
  display: grid;
  gap: 20px;
}

.dashboard-hero {
  display: flex;
  justify-content: space-between;
  gap: 20px;
  align-items: flex-start;
  padding: 24px;
  background:
    radial-gradient(circle at top right, rgba(59, 130, 246, 0.18), transparent 28%),
    linear-gradient(135deg, #ffffff, #f8fbff);
}

.eyebrow {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.16em;
}

.muted,
.chart-title-row p,
.stat-hint {
  color: #64748b;
}

.range-tools {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  justify-content: flex-end;
  align-items: center;
}

.stat-grid,
.chart-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 16px;
}

.stat-card {
  padding: 20px;
  border-radius: 18px;
  background: #ffffff;
  border: 1px solid #e2e8f0;
  box-shadow: 0 18px 44px rgba(15, 23, 42, 0.06);
}

.stat-card.primary {
  background: #eff6ff;
  border-color: #bfdbfe;
}

.stat-card.success {
  background: #ecfdf5;
  border-color: #bbf7d0;
}

.stat-card.warning {
  background: #fffbeb;
  border-color: #fde68a;
}

.stat-card.dark {
  background: #0f172a;
  border-color: #0f172a;
  color: #fff;
}

.stat-card.dark .stat-hint,
.stat-card.dark .stat-label {
  color: #cbd5e1;
}

.stat-label {
  color: #64748b;
  font-weight: 700;
}

.stat-value {
  margin-top: 10px;
  font-size: 30px;
  font-weight: 900;
}

.stat-hint {
  margin-top: 8px;
  font-size: 13px;
}

.chart-card {
  padding: 22px;
}

.chart-title-row {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: flex-start;
}

.chart-title-row h3 {
  margin: 0;
  font-size: 22px;
}

.donut-wrap {
  display: grid;
  grid-template-columns: 220px minmax(0, 1fr);
  gap: 24px;
  align-items: center;
  margin-top: 24px;
}

.donut {
  width: 220px;
  height: 220px;
  border-radius: 50%;
  display: grid;
  place-items: center;
}

.donut-hole {
  width: 132px;
  height: 132px;
  display: grid;
  place-items: center;
  align-content: center;
  border-radius: 50%;
  background: #ffffff;
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
  margin-top: 24px;
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
  border-radius: 999px;
  background: #e2e8f0;
  overflow: hidden;
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

.table-block {
  margin-top: 18px;
}

@media (max-width: 860px) {
  .dashboard-hero,
  .chart-title-row {
    flex-direction: column;
  }

  .donut-wrap {
    grid-template-columns: 1fr;
    justify-items: center;
  }
}
</style>
