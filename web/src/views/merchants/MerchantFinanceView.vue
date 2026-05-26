<template>
  <div class="finance-page">
    <PageHero
      eyebrow="FINANCE CENTER"
      title="财务对账"
      description="只处理顾客扫码点单经营流水，商家订阅费用不计入本页；应收、待付款、待确认、实收和退款口径与订单中心保持一致。"
      compact
    >
      <template #actions>
        <el-button plain @click="exportIncomeXlsx">导出收入 XLSX</el-button>
        <el-button plain @click="exportSettlementXlsx">导出结算 XLSX</el-button>
        <el-button type="primary" :loading="loading" @click="load">刷新财务</el-button>
      </template>
    </PageHero>

    <section class="reconcile-focus">
      <div class="focus-head">
        <div>
          <span>RECONCILE WORKBENCH</span>
          <strong>{{ reconcileBrief.title }}</strong>
          <p>{{ reconcileBrief.content }}</p>
        </div>
        <div class="focus-actions">
          <el-button plain @click="activeTab = 'exception'">看待核对</el-button>
          <el-button plain @click="activeTab = 'settlement'">看结算单</el-button>
          <el-button type="primary" plain @click="exportIncomeXlsx">导出当前账单</el-button>
        </div>
      </div>

      <div class="finance-lanes">
        <button
          v-for="item in financeLanes"
          :key="item.key"
          type="button"
          :class="['finance-lane', item.tone]"
          @click="item.go"
        >
          <span>{{ item.label }}</span>
          <strong>{{ item.title }}</strong>
          <small>{{ item.hint }}</small>
        </button>
      </div>

      <el-collapse class="finance-more">
        <el-collapse-item title="展开查看核账金额明细" name="metrics">
          <div class="focus-metrics">
            <div v-for="item in primaryMetrics" :key="item.label" :class="['focus-metric', item.tone]">
              <span>{{ item.label }}</span>
              <strong>{{ item.value }}</strong>
              <small>{{ item.hint }}</small>
            </div>
          </div>
        </el-collapse-item>
      </el-collapse>
    </section>

    <DataPanel title="筛选条件" description="财务页主要用于核对和导出；今日经营趋势、待处理提醒请在经营看板查看。导出 XLSX 时会沿用当前筛选结果。">
      <div class="filters-row">
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
          <el-option label="商家收款码" value="merchant_qr" />
          <el-option label="其他渠道" value="other" />
        </el-select>
        <el-select v-model="filters.status" clearable placeholder="全部订单状态">
          <el-option label="待支付" value="pending" />
          <el-option label="支付失败" value="failed" />
          <el-option label="已提交" value="submitted" />
          <el-option label="待确认收款" value="payment_confirming" />
          <el-option label="处理中" value="preparing" />
          <el-option label="待接单" value="received" />
          <el-option label="已接单" value="accepted" />
          <el-option label="已完成" value="completed" />
          <el-option label="已关闭" value="closed" />
        </el-select>
        <el-button type="primary" plain @click="load">查询</el-button>
        <el-button @click="resetFilters">重置</el-button>
      </div>
    </DataPanel>

    <DataPanel title="结算进度" description="展示平台与商家之间的人工结算状态，不包含商家订阅 SaaS 费用。">
      <section class="settlement-summary">
        <div>
          <span>待结算订单</span>
          <strong>{{ settlementSummary.pending_order_count || 0 }} 笔</strong>
          <small>已接单/已完成但未生成结算单</small>
        </div>
        <div>
          <span>待结算实付</span>
          <strong>{{ formatMoney(settlementSummary.pending_order_amount) }}</strong>
          <small>未归集订单的顾客实付金额</small>
        </div>
        <div>
          <span>待结算退款</span>
          <strong>{{ formatMoney(settlementSummary.pending_refund_amount) }}</strong>
          <small>未归集订单中已发生的退款</small>
        </div>
        <div>
          <span>待结算净额</span>
          <strong>{{ formatMoney(settlementSummary.pending_net_amount) }}</strong>
          <small>平台后续应结给商家的参考金额</small>
        </div>
      </section>
    </DataPanel>

    <DataPanel title="对账明细" description="收入、退款和结算记录集中查看，方便商家每天核账。">
      <el-tabs v-model="activeTab">
        <el-tab-pane label="收入明细" name="income">
          <el-table :data="filteredPaidOrders" empty-text="暂无已支付扫码订单">
            <el-table-column prop="order_no" label="订单号" min-width="190" />
            <el-table-column label="门店" min-width="130">
              <template #default="{ row }">{{ row.store?.name || storeName(row.store_id) }}</template>
            </el-table-column>
            <el-table-column prop="customer_phone" label="顾客手机号" min-width="130" />
            <el-table-column label="支付渠道" width="110">
              <template #default="{ row }">{{ channelLabel(row.payment_channel) }}</template>
            </el-table-column>
            <el-table-column label="实付" width="110">
              <template #default="{ row }">{{ formatMoney(orderPaidAmount(row)) }}</template>
            </el-table-column>
            <el-table-column label="退款" width="110">
              <template #default="{ row }">{{ formatMoney(row.refunded_amount) }}</template>
            </el-table-column>
            <el-table-column label="净额" width="110">
              <template #default="{ row }"><strong>{{ formatMoney(netReceived(row)) }}</strong></template>
            </el-table-column>
            <el-table-column label="结算状态" width="120">
              <template #default="{ row }">
                <el-tag :type="settlementTagType(row)">{{ settlementLabel(row) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="订单状态" width="110">
              <template #default="{ row }">
                <el-tag :type="statusType(row.status)">{{ statusLabel(row.status) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="支付时间" min-width="170">
              <template #default="{ row }">{{ formatTime(row.paid_at || row.updated_at) }}</template>
            </el-table-column>
          </el-table>
        </el-tab-pane>

        <el-tab-pane :label="`异常核对 ${exceptionOrders.length}`" name="exception">
          <el-table :data="exceptionOrders" empty-text="当前筛选内暂无异常订单">
            <el-table-column prop="order_no" label="订单号" min-width="190" />
            <el-table-column label="异常类型" min-width="150">
              <template #default="{ row }">
                <el-tag :type="exceptionType(row).tag">{{ exceptionType(row).label }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="门店" min-width="130">
              <template #default="{ row }">{{ row.store?.name || storeName(row.store_id) }}</template>
            </el-table-column>
            <el-table-column label="订单金额" width="120">
              <template #default="{ row }">{{ formatMoney(orderPaidAmount(row) || row.total_amount || row.amount) }}</template>
            </el-table-column>
            <el-table-column label="退款" width="110">
              <template #default="{ row }">{{ formatMoney(row.refunded_amount) }}</template>
            </el-table-column>
            <el-table-column label="建议动作" min-width="260">
              <template #default="{ row }">{{ exceptionType(row).hint }}</template>
            </el-table-column>
            <el-table-column label="订单状态" width="110">
              <template #default="{ row }">
                <el-tag :type="statusType(row.status)">{{ statusLabel(row.status) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="110">
              <template #default="{ row }">
                <el-button size="small" type="primary" plain @click="router.push(`/merchant/orders/${row.id}`)">核对</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>

        <el-tab-pane label="退款售后" name="refund">
          <el-alert
            class="refund-ledger-alert"
            type="warning"
            show-icon
            :closable="false"
            title="退款记录用于财务核对；商家收款码订单仍需在支付宝、微信或线下账户实际退款。"
          />
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
            <el-table-column label="渠道同步" min-width="170">
              <template #default="{ row }">
                <div class="refund-sync-cell">
                  <el-tag :type="refundSyncType(row)" size="small">{{ refundSyncLabel(row) }}</el-tag>
                  <small>{{ refundSyncHint(row) }}</small>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="操作方" width="120">
              <template #default="{ row }">{{ row.operator_role === 'admin' ? '平台后台' : '商家端' }}</template>
            </el-table-column>
            <el-table-column label="状态" width="100">
              <template #default="{ row }"><el-tag type="success">{{ row.status || 'success' }}</el-tag></template>
            </el-table-column>
            <el-table-column label="创建时间" min-width="170">
              <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
            </el-table-column>
          </el-table>
        </el-tab-pane>

        <el-tab-pane label="结算记录" name="settlement">
          <el-alert
            class="settlement-ledger-alert"
            type="info"
            show-icon
            :closable="false"
            title="结算单按净额核对：订单实付 - 已记录退款 = 净结算。商家收款码模式下，平台结算仅作对账参考。"
          />
          <el-table :data="settlements" empty-text="暂无结算单">
            <el-table-column prop="id" label="结算ID" width="90" />
            <el-table-column label="周期" min-width="220">
              <template #default="{ row }">{{ formatDate(row.settlement_period_start) }} - {{ formatDate(row.settlement_period_end) }}</template>
            </el-table-column>
            <el-table-column prop="order_count" label="订单数" width="100" />
            <el-table-column label="订单实付" width="120">
              <template #default="{ row }">{{ formatMoney(row.total_amount_cents) }}</template>
            </el-table-column>
            <el-table-column label="退款" width="120">
              <template #default="{ row }">{{ formatMoney(row.refund_amount_cents) }}</template>
            </el-table-column>
            <el-table-column label="净结算" width="130">
              <template #default="{ row }"><strong>{{ formatMoney(row.net_amount_cents) }}</strong></template>
            </el-table-column>
            <el-table-column label="退款影响" min-width="150">
              <template #default="{ row }">
                <el-tag :type="Number(row.refund_amount_cents || 0) > 0 ? 'warning' : 'success'">
                  {{ Number(row.refund_amount_cents || 0) > 0 ? '已扣退款' : '无退款' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="状态" width="110">
              <template #default="{ row }">
                <el-tag :type="row.status === 'paid' ? 'success' : 'warning'">{{ row.status === 'paid' ? '已付款' : '待付款' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="付款时间" min-width="170">
              <template #default="{ row }">{{ formatTime(row.paid_at) }}</template>
            </el-table-column>
            <el-table-column prop="remark" label="备注/凭证" min-width="220" show-overflow-tooltip />
          </el-table>
        </el-tab-pane>
      </el-tabs>
    </DataPanel>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  fetchMerchantOrders,
  fetchMerchantRefunds,
  fetchMerchantSettlements,
  fetchMerchantStores
} from '../../api/modules'
import DataPanel from '../../components/design/DataPanel.vue'
import PageHero from '../../components/design/PageHero.vue'
import { exportRowsToXlsx } from '../../utils/xlsx'

const router = useRouter()
const loading = ref(false)
const activeTab = ref('income')
const orders = ref([])
const refunds = ref([])
const stores = ref([])
const settlements = ref([])
const settlementSummary = ref({})
const filters = reactive({ dateRange: [], storeId: undefined, status: '', paymentChannel: '' })
const paidStatuses = ['received', 'accepted', 'completed', 'closed']
const periodMode = ref('today')

const paidOrders = computed(() => orders.value.filter((item) => item.order_type === 'store_order' && paidStatuses.includes(item.status)))
const filteredPaidOrders = computed(() => paidOrders.value.filter((item) => matchFilters(item)))
const filteredAllStoreOrders = computed(() => orders.value.filter((item) => item.order_type === 'store_order' && matchFilters(item)))
const pendingCustomerPayOrders = computed(() => filteredAllStoreOrders.value.filter((item) => ['pending', 'failed'].includes(item.status)))
const unconfirmedOrders = computed(() => filteredAllStoreOrders.value.filter((item) => item.status === 'payment_confirming'))
const confirmedPaidOrders = computed(() => filteredPaidOrders.value.filter((item) => paidStatuses.includes(item.status)))
const filteredRefunds = computed(() => refunds.value.filter((item) => {
  if (!matchDate(item.created_at)) return false
  if (filters.storeId && Number(item.store_id || item.order?.store_id) !== Number(filters.storeId)) return false
  return true
}))
const pendingSettlementOrders = computed(() => filteredPaidOrders.value.filter((item) => settlementLabel(item) === '待结算'))
const exceptionOrders = computed(() => filteredAllStoreOrders.value.filter((item) => {
  if (['pending', 'failed', 'payment_confirming'].includes(item.status)) return true
  if (Number(item.refunded_amount || 0) > 0 || ['partial', 'full'].includes(item.refund_status)) return true
  if (['accepted', 'completed'].includes(item.status) && !item.settlement_id) return true
  return false
}))
const totalPaid = computed(() => filteredPaidOrders.value.reduce((sum, item) => sum + orderPaidAmount(item), 0))
const totalReceivable = computed(() => filteredAllStoreOrders.value.reduce((sum, item) => sum + Number(item.total_amount || item.amount || 0), 0))
const customerPendingAmount = computed(() => pendingCustomerPayOrders.value.reduce((sum, item) => sum + Number(item.total_amount || item.amount || 0), 0))
const unconfirmedAmount = computed(() => unconfirmedOrders.value.reduce((sum, item) => sum + orderPaidAmount(item), 0))
const totalRefunded = computed(() => filteredRefunds.value.reduce((sum, item) => sum + Number(item.amount || 0), 0))
const netIncome = computed(() => Math.max(totalPaid.value - totalRefunded.value, 0))
const refundRate = computed(() => totalPaid.value > 0 ? Math.round((totalRefunded.value / totalPaid.value) * 100) : 0)
const primaryMetrics = computed(() => [
  {
    label: '应收金额',
    value: formatMoney(totalReceivable.value),
    hint: '当前筛选内订单账面金额',
    tone: 'primary'
  },
  {
    label: '待顾客付款',
    value: formatMoney(customerPendingAmount.value),
    hint: `${pendingCustomerPayOrders.value.length} 笔待支付/支付失败，不计入实收`,
    tone: pendingCustomerPayOrders.value.length ? 'warning' : 'muted'
  },
  {
    label: '待确认收款',
    value: formatMoney(unconfirmedAmount.value),
    hint: `${unconfirmedOrders.value.length} 笔顾客已标记付款，需核对到账`,
    tone: unconfirmedOrders.value.length ? 'warning' : 'muted'
  },
  {
    label: '已确认实收',
    value: formatMoney(totalPaid.value),
    hint: `${confirmedPaidOrders.value.length} 笔已进入经营流水`,
    tone: 'success'
  },
  {
    label: '退款金额',
    value: formatMoney(totalRefunded.value),
    hint: `退款率 ${refundRate.value}%`,
    tone: totalRefunded.value ? 'warning' : 'muted'
  },
  {
    label: '净实收',
    value: formatMoney(netIncome.value),
    hint: '确认实收 - 已记录退款',
    tone: 'success'
  }
])
const financeLanes = computed(() => [
  {
    key: 'today',
    label: '今日核账',
    title: formatMoney(netIncome.value),
    hint: `净实收；${unconfirmedOrders.value.length} 笔待确认收款`,
    tone: unconfirmedOrders.value.length ? 'warning' : 'success',
    go: setTodayReconcile
  },
  {
    key: 'exception',
    label: '异常核对',
    title: `${exceptionOrders.value.length} 笔`,
    hint: exceptionOrders.value.length ? '待支付、待确认、退款或待结算需要复核' : '当前筛选范围无异常',
    tone: exceptionOrders.value.length ? 'warning' : 'muted',
    go: setExceptionReconcile
  },
  {
    key: 'export',
    label: '导出明细',
    title: `${filteredPaidOrders.value.length} 笔`,
    hint: '导出当前筛选范围收入明细，便于每日留档',
    tone: 'primary',
    go: exportIncomeXlsx
  },
  {
    key: 'settlement',
    label: '结算参考',
    title: formatMoney(settlementSummary.value?.pending_net_amount),
    hint: `${pendingSettlementOrders.value.length} 笔订单待进入结算`,
    tone: pendingSettlementOrders.value.length ? 'warning' : 'muted',
    go: setSettlementReference
  }
])
const reconcileShortcuts = computed(() => [
  {
    key: 'unconfirmed',
    label: '待确认收款',
    value: `${unconfirmedOrders.value.length} 笔`,
    hint: unconfirmedOrders.value.length ? `${formatMoney(unconfirmedAmount.value)} 需核对到账` : '当前无需核对收款',
    tone: unconfirmedOrders.value.length ? 'warning' : 'muted',
    go: () => { activeTab.value = 'exception'; filters.status = 'payment_confirming' }
  },
  {
    key: 'refund',
    label: '退款/售后',
    value: `${filteredRefunds.value.length} 条`,
    hint: filteredRefunds.value.length ? `${formatMoney(totalRefunded.value)} 需核对原因` : '当前无退款记录',
    tone: filteredRefunds.value.length ? 'warning' : 'muted',
    go: () => { activeTab.value = 'refund' }
  },
  {
    key: 'settlement',
    label: '待结算',
    value: `${pendingSettlementOrders.value.length} 笔`,
    hint: `${formatMoney(settlementSummary.value?.pending_net_amount)} 参考净额`,
    tone: pendingSettlementOrders.value.length ? 'warning' : 'muted',
    go: () => { activeTab.value = 'settlement' }
  }
])
const reconcileBrief = computed(() => {
  if (unconfirmedOrders.value.length > 0) {
    return {
      title: `${unconfirmedOrders.value.length} 笔订单等待确认收款`,
      content: `待确认金额 ${formatMoney(unconfirmedAmount.value)}。这些订单暂不计入经营净收入，请先核对支付宝/微信到账，再到订单详情确认收款。`
    }
  }
  if (!filteredPaidOrders.value.length) {
    return {
      title: '当前筛选范围暂无已支付扫码订单',
      content: '可以调整日期、门店或订单状态筛选；商家订阅费用不会计入本页，避免和顾客交易流水混淆。'
    }
  }
  if (refundRate.value >= 20) {
    return {
      title: `退款率 ${refundRate.value}% 偏高，建议优先复盘售后`,
      content: `当前筛选内 ${filteredPaidOrders.value.length} 笔已支付订单，净收入 ${formatMoney(netIncome.value)}，退款 ${formatMoney(totalRefunded.value)}。建议查看退款售后标签页定位原因。`
    }
  }
  if (pendingSettlementOrders.value.length > 0) {
    return {
      title: `${pendingSettlementOrders.value.length} 笔订单待进入结算`,
      content: `当前净收入 ${formatMoney(netIncome.value)}，待结算净额 ${formatMoney(settlementSummary.value?.pending_net_amount)}。可导出账单后与平台结算记录核对。`
    }
  }
  return {
    title: `当前净收入 ${formatMoney(netIncome.value)}，账面状态稳定`,
    content: `当前筛选内 ${filteredPaidOrders.value.length} 笔已支付订单，退款率 ${refundRate.value}%；可直接导出 XLSX 留存或与平台结算单核对。`
  }
})

const load = async () => {
  loading.value = true
  try {
    const [ordersRes, refundsRes, storesRes, settlementsRes] = await Promise.all([
      fetchMerchantOrders({ page: 1, page_size: 500 }),
      fetchMerchantRefunds(),
      fetchMerchantStores({ page: 1, page_size: 100 }),
      fetchMerchantSettlements()
    ])
    orders.value = ordersRes.data.list || []
    refunds.value = Array.isArray(refundsRes.data) ? refundsRes.data : (refundsRes.data?.list || [])
    stores.value = storesRes.data.list || []
    settlements.value = settlementsRes.data.list || []
    settlementSummary.value = settlementsRes.data.summary || {}
  } finally {
    loading.value = false
  }
}

const resetFilters = () => {
  filters.dateRange = []
  filters.storeId = undefined
  filters.status = ''
  filters.paymentChannel = ''
  load()
}

function setTodayReconcile() {
  periodMode.value = 'today'
  applyPeriodMode()
  filters.status = ''
  activeTab.value = unconfirmedOrders.value.length ? 'exception' : 'income'
}

function setExceptionReconcile() {
  activeTab.value = 'exception'
  filters.status = ''
}

function setSettlementReference() {
  activeTab.value = 'settlement'
  filters.status = ''
}

const applyPeriodMode = () => {
  const now = new Date()
  if (periodMode.value === 'all') {
    filters.dateRange = []
    return
  }
  if (periodMode.value === 'today') {
    const today = formatInputDate(now)
    filters.dateRange = [today, today]
    return
  }
  if (periodMode.value === 'week') {
    const day = now.getDay() || 7
    const start = new Date(now)
    start.setDate(now.getDate() - day + 1)
    filters.dateRange = [formatInputDate(start), formatInputDate(now)]
    return
  }
  if (periodMode.value === 'month') {
    const start = new Date(now.getFullYear(), now.getMonth(), 1)
    filters.dateRange = [formatInputDate(start), formatInputDate(now)]
  }
}

const matchFilters = (item, options = {}) => {
  if (!matchDate(item.paid_at || item.updated_at || item.created_at)) return false
  if (filters.storeId && Number(item.store_id) !== Number(filters.storeId)) return false
  if (!options.includeStatus && filters.status && item.status !== filters.status) return false
  if (!options.includeChannel && filters.paymentChannel && normalizedChannel(item.payment_channel) !== filters.paymentChannel) return false
  return true
}

const matchDate = (value) => {
  if (!filters.dateRange || filters.dateRange.length !== 2 || !value) return true
  const target = new Date(value).getTime()
  const start = new Date(`${filters.dateRange[0]} 00:00:00`).getTime()
  const end = new Date(`${filters.dateRange[1]} 23:59:59`).getTime()
  return target >= start && target <= end
}

const orderPaidAmount = (row) => Number(row.paid_amount ?? row.total_amount ?? 0)
const netReceived = (row) => Math.max(orderPaidAmount(row) - Number(row.refunded_amount || 0), 0)
const centsToYuan = (value) => (Number(value || 0) / 100).toFixed(2)
const formatMoney = (value) => `¥${centsToYuan(value)}`
const formatDate = (value) => value ? new Date(value).toLocaleDateString('zh-CN') : '-'
const formatTime = (value) => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '-'
const formatInputDate = (value) => {
  const date = new Date(value)
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${date.getFullYear()}-${month}-${day}`
}
const filterRangeLabel = computed(() => {
  const date = filters.dateRange?.length === 2 ? `${filters.dateRange[0]} 至 ${filters.dateRange[1]}` : '全部日期'
  const store = filters.storeId ? storeName(filters.storeId) : '全部门店'
  const channel = filters.paymentChannel ? channelLabel(filters.paymentChannel) : '全部渠道'
  const status = filters.status ? statusLabel(filters.status) : '全部状态'
  return `${date} / ${store} / ${channel} / ${status}`
})

const storeName = (id) => {
  const store = stores.value.find((item) => Number(item.id) === Number(id))
  return store?.name || '-'
}

const normalizedChannel = (channel) => {
  if (!channel) return 'other'
  if (String(channel).includes('merchant_qr')) return 'merchant_qr'
  if (String(channel).includes('alipay')) return 'alipay'
  if (String(channel).includes('wechat') || String(channel).includes('mock')) return 'mock_wechat'
  return 'other'
}

const channelLabel = (channel) => {
  const current = normalizedChannel(channel)
  if (current === 'merchant_qr') return '商家收款码'
  if (current === 'alipay') return '支付宝'
  if (current === 'mock_wechat') return '微信/模拟'
  return '其他'
}

const settlementLabel = (row) => {
  if (row.settlement_id) return '已进结算'
  if (['accepted', 'completed'].includes(row.status)) return '待结算'
  return '暂不可结算'
}

const settlementTagType = (row) => {
  if (row.settlement_id) return 'success'
  if (['accepted', 'completed'].includes(row.status)) return 'warning'
  return 'info'
}

const exceptionType = (row) => {
  if (row.status === 'payment_confirming') {
    return { label: '待确认收款', tag: 'warning', hint: '顾客已标记付款，请核对商家收款账户到账后再确认收款。' }
  }
  if (row.status === 'pending') {
    return { label: '待支付', tag: 'warning', hint: '未付款订单不计入实收，顾客可继续支付，商家不要提前制作。' }
  }
  if (row.status === 'failed') {
    return { label: '支付失败', tag: 'danger', hint: '顾客可回到订单页重新支付，商家只需保留订单记录。' }
  }
  if (Number(row.refunded_amount || 0) > 0 || ['partial', 'full'].includes(row.refund_status)) {
    return { label: '退款核对', tag: 'warning', hint: '核对退款原因、退款金额和支付渠道到账状态。' }
  }
  if (['accepted', 'completed'].includes(row.status) && !row.settlement_id) {
    return { label: '待结算', tag: 'primary', hint: '订单已形成实收但未进入结算单，可在结算周期内归集。' }
  }
  return { label: '需核对', tag: 'info', hint: '请进入订单详情核对状态。' }
}

const statusLabel = (status) => ({
  pending: '待支付',
  submitted: '已提交',
  payment_confirming: '待确认收款',
  preparing: '处理中',
  received: '待接单',
  accepted: '已接单',
  completed: '已完成',
  closed: '已关闭',
  refunded: '已退款'
}[status] || status || '-')

const statusType = (status) => ({
  pending: 'warning',
  submitted: 'primary',
  payment_confirming: 'warning',
  preparing: 'success',
  received: 'warning',
  accepted: 'primary',
  completed: 'success',
  closed: 'info',
  refunded: 'danger'
}[status] || 'info')

const exportIncomeXlsx = () => {
  const summaryRows = [
    incomeExportRow({ 类型: '汇总', 项目: '筛选范围', 内容: filterRangeLabel.value }),
    incomeExportRow({ 类型: '汇总', 项目: '应收金额', 内容: centsToYuan(totalReceivable.value) }),
    incomeExportRow({ 类型: '汇总', 项目: '待顾客付款', 内容: `${pendingCustomerPayOrders.value.length} 笔 / ${centsToYuan(customerPendingAmount.value)}` }),
    incomeExportRow({ 类型: '汇总', 项目: '待确认收款', 内容: `${unconfirmedOrders.value.length} 笔 / ${centsToYuan(unconfirmedAmount.value)}` }),
    incomeExportRow({ 类型: '汇总', 项目: '已确认收款订单数', 内容: `${filteredPaidOrders.value.length} 笔` }),
    incomeExportRow({ 类型: '汇总', 项目: '已确认收款金额', 内容: centsToYuan(totalPaid.value) }),
    incomeExportRow({ 类型: '汇总', 项目: '退款金额', 内容: centsToYuan(totalRefunded.value) }),
    incomeExportRow({ 类型: '汇总', 项目: '退款处理说明', 内容: '商家收款码订单的退款记录仅用于系统财务核对，真实退款需在支付宝、微信或线下收款账户执行。' }),
    incomeExportRow({ 类型: '汇总', 项目: '经营净收入', 内容: centsToYuan(netIncome.value) }),
    incomeExportRow({ 类型: '汇总', 项目: '退款率', 内容: `${refundRate.value}%` }),
    incomeExportRow({ 类型: '汇总', 项目: '待结算订单', 内容: `${pendingSettlementOrders.value.length} 笔` }),
    incomeExportRow({ 类型: '汇总', 项目: '异常核对订单', 内容: `${exceptionOrders.value.length} 笔` }),
    incomeExportRow({})
  ]
  const detailRows = filteredPaidOrders.value.map((row) => incomeExportRow({
    类型: '明细',
    项目: '扫码订单',
    内容: '',
    订单号: row.order_no,
    门店: row.store?.name || storeName(row.store_id),
    顾客手机号: row.customer_phone || '-',
    支付渠道: channelLabel(row.payment_channel),
    实付金额: centsToYuan(orderPaidAmount(row)),
    退款金额: centsToYuan(row.refunded_amount),
    净收入: centsToYuan(netReceived(row)),
    异常类型: exceptionOrders.value.some((item) => item.id === row.id) ? exceptionType(row).label : '',
    结算状态: settlementLabel(row),
    订单状态: statusLabel(row.status),
    支付时间: formatTime(row.paid_at || row.updated_at)
  }))
  exportRowsToXlsx('商家财务收入明细.xlsx', '收入明细', [...summaryRows, ...detailRows])
}

const incomeExportRow = (row) => ({
  类型: '',
  项目: '',
  内容: '',
  订单号: '',
  门店: '',
  顾客手机号: '',
  支付渠道: '',
  实付金额: '',
  退款金额: '',
  净收入: '',
  异常类型: '',
  结算状态: '',
  订单状态: '',
  支付时间: '',
  ...row
})

const refundRawPayload = (row) => {
  const raw = row.raw_payload
  if (!raw) return {}
  if (typeof raw === 'object') return raw
  try {
    return JSON.parse(raw)
  } catch {
    return {}
  }
}
const refundIsManualRecord = (row) => refundRawPayload(row).mode === 'manual_record'
const refundSyncLabel = (row) => (refundIsManualRecord(row) ? '需人工核对' : '渠道已处理')
const refundSyncType = (row) => (refundIsManualRecord(row) ? 'warning' : 'success')
const refundSyncHint = (row) => {
  if (refundIsManualRecord(row)) return '系统已记账，实际退款请以收款账户为准'
  return '已调用支付渠道退款或返回渠道结果'
}

const exportSettlementXlsx = () => {
  exportRowsToXlsx('商家结算记录.xlsx', '结算记录', settlements.value.map((row) => ({
    结算ID: row.id,
    结算周期: `${formatDate(row.settlement_period_start)} - ${formatDate(row.settlement_period_end)}`,
    订单数: row.order_count || 0,
    订单实付: centsToYuan(row.total_amount_cents),
    退款金额: centsToYuan(row.refund_amount_cents),
    净结算: centsToYuan(row.net_amount_cents),
    状态: row.status === 'paid' ? '已付款' : '待付款',
    付款时间: formatTime(row.paid_at),
    备注: row.remark || '-'
  })))
}

onMounted(() => {
  applyPeriodMode()
  load()
})
</script>

<style scoped>
.finance-page {
  display: grid;
  gap: 16px;
}

.reconcile-focus {
  display: grid;
  gap: 16px;
  padding: 18px 20px;
  border: 1px solid #dbeafe;
  border-radius: 24px;
  background: #ffffff;
  box-shadow: 0 16px 40px rgba(15, 39, 71, 0.08);
}

.focus-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 18px;
}

.focus-head span,
.focus-head strong,
.focus-head p,
.focus-metric span,
.focus-metric strong,
.focus-metric small,
.finance-lane span,
.finance-lane strong,
.finance-lane small,
.queue-card span,
.queue-card strong,
.queue-card small {
  display: block;
}

.focus-head span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.08em;
}

.focus-head strong {
  margin-top: 6px;
  color: #0f2747;
  font-size: 24px;
  line-height: 1.25;
}

.focus-head p {
  max-width: 860px;
  margin: 8px 0 0;
  color: #64748b;
  line-height: 1.65;
}

.focus-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  justify-content: flex-end;
}

.finance-lanes {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.finance-lane {
  min-height: 112px;
  padding: 15px;
  text-align: left;
  cursor: pointer;
  border: 1px solid #dbeafe;
  border-radius: 16px;
  background: #f8fbff;
  transition: 0.2s ease;
}

.finance-lane:hover {
  transform: translateY(-1px);
  border-color: #93c5fd;
  box-shadow: 0 12px 28px rgba(15, 23, 42, 0.08);
}

.finance-lane span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
}

.finance-lane strong {
  margin: 8px 0 6px;
  color: #0f2747;
  font-size: 22px;
  line-height: 1.15;
}

.finance-lane small {
  color: #64748b;
  line-height: 1.45;
}

.finance-lane.success {
  border-color: #bbf7d0;
  background: #ecfdf5;
}

.finance-lane.warning {
  border-color: #fed7aa;
  background: #fff7ed;
}

.finance-lane.primary {
  border-color: #bfdbfe;
  background: #eff6ff;
}

.finance-lane.muted {
  opacity: 0.84;
}

.finance-more {
  border: 1px solid #dbeafe;
  border-radius: 14px;
  overflow: hidden;
  background: #ffffff;
}

.focus-metrics {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: 12px;
}

.focus-metric {
  min-height: 104px;
  padding: 15px;
  border: 1px solid #e2e8f0;
  border-radius: 16px;
  background: #f8fafc;
}

.focus-metric span,
.queue-card span {
  color: #64748b;
  font-size: 12px;
  font-weight: 800;
}

.focus-metric strong,
.queue-card strong {
  margin: 8px 0 5px;
  color: #0f172a;
  font-size: 22px;
  line-height: 1.15;
}

.focus-metric small,
.queue-card small {
  color: #64748b;
  line-height: 1.45;
}

.focus-metric.primary,
.queue-card.primary {
  border-color: #bfdbfe;
  background: #eff6ff;
}

.focus-metric.success,
.queue-card.success {
  border-color: #bbf7d0;
  background: #ecfdf5;
}

.focus-metric.warning,
.queue-card.warning {
  border-color: #fed7aa;
  background: #fff7ed;
}

.focus-metric.danger,
.queue-card.danger {
  border-color: #fecaca;
  background: #fef2f2;
}

.focus-metric.muted,
.queue-card.muted {
  opacity: 0.82;
}

.focus-queue {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.queue-card {
  min-height: 96px;
  padding: 14px;
  text-align: left;
  cursor: pointer;
  border: 1px solid #e2e8f0;
  border-radius: 16px;
  background: #f8fafc;
  transition: 0.2s ease;
}

.queue-card:hover {
  transform: translateY(-1px);
  box-shadow: 0 12px 28px rgba(15, 23, 42, 0.08);
}

.filters-row {
  display: grid;
  grid-template-columns: minmax(280px, 1.4fr) repeat(3, minmax(150px, 1fr)) auto auto;
  gap: 10px;
  align-items: center;
}

.settlement-summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.settlement-summary div {
  min-height: 92px;
  padding: 14px;
  border: 1px solid #dce8f5;
  border-radius: 16px;
  background: linear-gradient(135deg, #f8fbff, #eef7ff);
}

.settlement-summary span,
.settlement-summary small {
  display: block;
  color: #64748b;
  font-size: 12px;
  line-height: 1.5;
}

.settlement-summary strong {
  display: block;
  margin: 8px 0 4px;
  color: #0f172a;
  font-size: 22px;
}

.refund-ledger-alert {
  margin-bottom: 12px;
}

.settlement-ledger-alert {
  margin-bottom: 12px;
}

.refund-sync-cell {
  display: grid;
  gap: 5px;
}

.refund-sync-cell small {
  color: #64748b;
  line-height: 1.4;
}

@media (max-width: 1280px) {
  .finance-lanes,
  .focus-queue {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .focus-metrics {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .filters-row {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .focus-head {
    align-items: flex-start;
    flex-direction: column;
  }

  .focus-actions {
    justify-content: flex-start;
  }
}

@media (max-width: 900px) {
  .finance-lanes,
  .focus-metrics,
  .focus-queue,
  .settlement-summary,
  .filters-row {
    grid-template-columns: 1fr;
  }
}
</style>
