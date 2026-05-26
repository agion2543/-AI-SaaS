<template>
  <div class="orders-page">
    <PageHero
      eyebrow="ORDER CENTER"
      title="订单管理"
      description="这里只展示顾客扫码点单订单，不混入商家订阅订单。商家可以在这里接单、完成、关闭、备注和进入售后处理。"
      compact
    >
      <template #actions>
        <el-button plain @click="router.push('/merchant/finance')">查看财务对账</el-button>
        <el-button plain @click="router.push('/merchant/orders?quick=pay_issue')">支付异常</el-button>
        <el-button type="primary" :loading="loading" @click="reload">刷新订单</el-button>
      </template>
    </PageHero>

    <section class="orders-summary">
      <MetricCard label="当前结果" :value="total" hint="符合当前筛选条件的扫码点单订单。" tone="primary" />
      <MetricCard label="待确认收款" :value="formatMoney(pageConfirmingAmount)" hint="顾客已标记付款，需要核对到账。" tone="warning" />
      <MetricCard label="已确认实收" :value="formatMoney(pageNetReceived)" hint="已确认收款扣除已记录退款。" tone="success" />
      <el-collapse class="summary-more">
        <el-collapse-item title="展开收银口径明细" name="ledger">
          <div class="metric-grid compact">
            <MetricCard label="当前页应收" :value="formatMoney(pageReceivableAmount)" hint="订单原始应收金额，含待付款和待确认。" tone="cyan" />
            <MetricCard label="待顾客付款" :value="formatMoney(pagePendingPayAmount)" hint="待支付、支付失败订单暂不计入实收。" tone="warning" />
          </div>
        </el-collapse-item>
      </el-collapse>
    </section>

    <section v-if="orderNotice.visible" class="order-notice" :class="orderNotice.tone">
      <div>
        <span>{{ orderNotice.label }}</span>
        <strong>{{ orderNotice.title }}</strong>
        <p>{{ orderNotice.text }}</p>
      </div>
      <el-button type="primary" @click="orderNotice.go">{{ orderNotice.button }}</el-button>
    </section>

    <section class="refresh-strip">
      <div>
        <span>自动刷新</span>
        <strong>{{ autoRefreshText }}</strong>
        <p>订单中心会定时同步新订单、待确认收款和支付异常，适合门店营业时常开。</p>
      </div>
      <div class="refresh-actions">
        <el-switch
          v-model="autoRefreshEnabled"
          active-text="开启"
          inactive-text="暂停"
          @change="toggleAutoRefresh"
        />
        <el-button text type="primary" :loading="loading" @click="loadOrders()">立即同步</el-button>
      </div>
    </section>

    <section class="cashier-handoff">
      <div class="handoff-main">
        <span>CASHIER HANDOFF</span>
        <strong>{{ cashierHandoff.title }}</strong>
        <p>{{ cashierHandoff.text }}</p>
      </div>
      <div class="handoff-ledger">
        <article v-for="item in cashierCards" :key="item.label" :class="item.tone">
          <span>{{ item.label }}</span>
          <strong>{{ item.value }}</strong>
          <small>{{ item.hint }}</small>
        </article>
      </div>
      <div class="handoff-actions">
        <el-button type="primary" @click="setQuickMode(cashierHandoff.quick)">{{ cashierHandoff.button }}</el-button>
        <el-button plain @click="router.push('/merchant/finance')">去财务对账</el-button>
      </div>
    </section>

    <section v-if="urgentQueue.length" class="queue-board">
      <div class="queue-head">
        <div>
          <span>LIVE QUEUE</span>
          <strong>营业待办队列</strong>
          <p>按确认收款、待处理、处理中、售后顺序排前 {{ urgentQueue.length }} 笔，适合营业高峰快速处理。</p>
        </div>
        <el-button plain type="primary" @click="setQuickMode(handoffSummary.quick)">查看当前优先项</el-button>
      </div>
      <div class="queue-list">
        <article v-for="row in urgentQueue" :key="`queue-${row.id}`" :class="queueTone(row)">
          <div class="queue-main">
            <span>{{ queueLabel(row) }}</span>
            <strong>{{ pickupCode(row) }} · {{ formatMoney(receivableAmount(row)) }}</strong>
            <p>{{ row.store?.name || '-' }} · {{ itemQuantity(row) }} 件 · {{ row.customer_note || '无备注' }}</p>
          </div>
          <div class="queue-actions">
            <el-button v-if="row.status === 'payment_confirming'" size="small" type="warning" @click="confirmPayment(row)">确认收款</el-button>
            <el-button v-if="['submitted', 'received'].includes(row.status)" size="small" type="primary" @click="acceptAndPrint(row)">接单打印</el-button>
            <el-button v-if="['preparing', 'accepted'].includes(row.status)" size="small" type="success" @click="complete(row)">完成</el-button>
            <el-button size="small" plain @click="goDetail(row)">详情</el-button>
          </div>
        </article>
      </div>
    </section>

    <DataPanel
      eyebrow="ORDER WORKFLOW"
      title="订单列表"
      description="支持按订单号、顾客手机号、商品、门店和状态快速筛选。列表区分应收、待确认和退款后净额，方便营业中快速核对。"
    >
      <div class="status-tabs">
        <button
          v-for="item in statusTabs"
          :key="item.value"
          type="button"
          :class="{ active: filters.status === item.value }"
          @click="setStatus(item.value)"
        >
          <span>{{ item.label }}</span>
          <strong>{{ item.count }}</strong>
        </button>
      </div>

      <el-collapse class="workflow-more">
        <el-collapse-item title="展开快捷筛选和交班提示" name="quick">
          <div class="quick-workbench">
            <button
              v-for="item in quickModes"
              :key="item.value"
              type="button"
              :class="['quick-card', item.tone, { active: activeQuickMode === item.value }]"
              @click="setQuickMode(item.value)"
            >
              <span>{{ item.label }}</span>
              <strong>{{ item.count }}</strong>
              <small>{{ item.hint }}</small>
            </button>
          </div>

          <div class="handoff-strip">
            <div>
              <span>今日处理顺序</span>
              <strong>{{ handoffSummary.title }}</strong>
              <p>{{ handoffSummary.text }}</p>
            </div>
            <el-button type="primary" plain @click="setQuickMode(handoffSummary.quick)">
              {{ handoffSummary.button }}
            </el-button>
          </div>
        </el-collapse-item>
      </el-collapse>

      <div class="filter-row">
        <el-input
          v-model="filters.keyword"
          clearable
          placeholder="搜索订单号、顾客手机号、商品"
          class="keyword-input"
          @keyup.enter="reload"
        />
        <el-select v-model="filters.store_id" clearable placeholder="全部门店" class="filter-item" @change="reload">
          <el-option v-for="store in stores" :key="store.id" :label="store.name" :value="store.id" />
        </el-select>
        <el-select v-model="filters.status" clearable placeholder="全部状态" class="filter-item" @change="reload">
          <el-option label="待支付" value="pending" />
          <el-option label="已提交" value="submitted" />
          <el-option label="待确认收款" value="payment_confirming" />
          <el-option label="处理中" value="preparing" />
          <el-option label="待接单" value="received" />
          <el-option label="已接单" value="accepted" />
          <el-option label="已完成" value="completed" />
          <el-option label="已关闭" value="closed" />
          <el-option label="支付失败" value="failed" />
        </el-select>
        <el-button type="primary" @click="reload">查询</el-button>
        <el-button @click="resetFilters">重置</el-button>
      </div>

      <el-alert
        class="order-tip"
        :type="workflowNotice.type"
        show-icon
        :closable="false"
        :title="workflowNotice.title"
      />

      <div v-if="selectedRows.length" class="batch-strip">
        <div class="batch-summary">
          <span>已选择 {{ selectedRows.length }} 笔订单</span>
          <strong>待处理 {{ selectedReceivedRows.length }} 笔 · 处理中 {{ selectedAcceptedRows.length }} 笔</strong>
          <small>适合高峰期按核对码连续处理，支付异常订单建议先进入详情核对。</small>
        </div>
        <div class="batch-actions">
          <el-button type="primary" :disabled="!selectedReceivedRows.length" @click="batchAccept">
            批量接单
          </el-button>
          <el-button type="success" :disabled="!selectedAcceptedRows.length" @click="batchComplete">
            批量完成
          </el-button>
          <el-button plain @click="batchPrint">批量打印</el-button>
          <el-button text @click="clearSelection">清空</el-button>
        </div>
      </div>

      <el-table
        ref="tableRef"
        v-loading="loading"
        :data="visibleOrders"
        :row-class-name="orderRowClass"
        :empty-text="emptyText"
        @selection-change="handleSelectionChange"
      >
        <el-table-column type="selection" width="46" />
        <el-table-column label="订单 / 核对码" min-width="230">
          <template #default="{ row }">
            <div class="order-main">
              <el-link type="primary" @click="goDetail(row)">{{ row.order_no }}</el-link>
              <div class="pickup-code" :class="{ urgent: ['submitted', 'payment_confirming', 'received'].includes(row.status) }">核对码 {{ pickupCode(row) }}</div>
              <small>{{ formatTime(row.created_at) }}</small>
              <small v-if="isActionableOrder(row)" :class="['wait-time', { urgent: orderWaitMinutes(row) >= 10 }]">
                {{ actionAgeLabel(row) }} {{ waitText(row) }}
              </small>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="门店 / 顾客" min-width="170">
          <template #default="{ row }">
            <div class="store-cell">
              <strong>{{ row.store?.name || '-' }}</strong>
              <div class="phone-line">
                <span>{{ row.customer_phone || '未留手机号' }}</span>
                <el-button v-if="row.customer_phone" size="small" text type="primary" @click="callCustomer(row)">
                  联系
                </el-button>
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="商品快照" min-width="260">
          <template #default="{ row }">
            <div class="items-preview">
              <strong class="items-count">共 {{ itemQuantity(row) }} 件</strong>
              <div v-for="item in row.items?.slice(0, 3) || []" :key="`${row.id}-${item.product_id}-${item.name}`">
                {{ item.name }} x {{ item.quantity }}
              </div>
              <span v-if="(row.items?.length || 0) > 3" class="muted">还有 {{ row.items.length - 3 }} 项</span>
              <span v-if="!(row.items || []).length" class="muted">暂无商品明细</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="顾客备注" min-width="160" show-overflow-tooltip>
          <template #default="{ row }">
            <span :class="['customer-note', { active: row.customer_note }]">
              {{ row.customer_note || '无备注' }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="收银口径" width="190">
          <template #default="{ row }">
            <div class="money-cell">
              <span>应收 {{ formatMoney(receivableAmount(row)) }}</span>
              <strong :class="['ledger-title', paymentLedgerTone(row)]">{{ paymentLedgerTitle(row) }}</strong>
              <small>{{ paymentLedgerHint(row) }}</small>
              <small v-if="Number(row.refunded_amount || 0) > 0">已退 {{ formatMoney(row.refunded_amount) }}</small>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="退款后净额" width="135">
          <template #default="{ row }">
            <div class="money-cell">
              <strong>{{ formatMoney(netReceived(row)) }}</strong>
              <small>{{ netAmountHint(row) }}</small>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="状态 / 售后" width="140">
          <template #default="{ row }">
            <div class="status-cell">
              <el-tag :type="statusType(row.status)">{{ statusLabel(row.status) }}</el-tag>
              <el-tag size="small" :type="refundType(row.refund_status)">
                {{ refundLabel(row.refund_status) }}
              </el-tag>
              <small v-if="Number(row.refunded_amount || 0) > 0" class="risk-text">需核对退款</small>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="备注" min-width="150" show-overflow-tooltip>
          <template #default="{ row }">{{ row.merchant_note || '-' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="320" fixed="right">
          <template #default="{ row }">
            <div class="action-row">
              <el-button size="small" @click="goDetail(row)">详情</el-button>
              <el-button v-if="['pending', 'failed'].includes(row.status)" size="small" type="warning" plain @click="goDetail(row)">核对支付</el-button>
              <el-button v-if="row.status === 'payment_confirming'" size="small" type="warning" @click="confirmPayment(row)">确认收款</el-button>
              <el-button v-if="['submitted', 'received'].includes(row.status)" size="small" type="primary" @click="acceptAndPrint(row)">接单并打印</el-button>
              <el-button v-if="['preparing', 'accepted'].includes(row.status)" size="small" type="success" @click="complete(row)">完成</el-button>
              <el-button v-if="hasRefundRisk(row)" size="small" type="warning" plain @click="goDetail(row)">售后</el-button>
              <el-button v-if="canClose(row)" size="small" type="danger" plain @click="close(row)">关闭</el-button>
              <el-button size="small" @click="openNote(row)">备注</el-button>
              <el-button size="small" plain @click="printReceipt(row)">打印</el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <div class="orders-pagination">
        <el-pagination
          background
          layout="total, prev, pager, next"
          :total="total"
          :page-size="pageSize"
          :current-page="page"
          @current-change="changePage"
        />
      </div>
    </DataPanel>

    <el-dialog v-model="noteDialogVisible" title="订单内部备注" width="520px">
      <el-input
        v-model="noteForm.merchant_note"
        type="textarea"
        :rows="4"
        maxlength="500"
        show-word-limit
        placeholder="可记录顾客偏好、异常情况、履约提醒等，仅商家端可见。"
      />
      <template #footer>
        <el-button @click="noteDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveNote">保存备注</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  acceptMerchantOrder,
  closeMerchantOrder,
  confirmMerchantOrderPayment,
  completeMerchantOrder,
  fetchMerchantOrders,
  fetchMerchantStores,
  updateMerchantOrderNote
} from '../../api/modules'
import DataPanel from '../../components/design/DataPanel.vue'
import MetricCard from '../../components/design/MetricCard.vue'
import PageHero from '../../components/design/PageHero.vue'
import { printStoreOrderReceipt } from '../../utils/orderReceipt'

const route = useRoute()
const router = useRouter()
const orders = ref([])
const stores = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 10
const loading = ref(false)
const tableRef = ref(null)
const selectedRows = ref([])
const noteDialogVisible = ref(false)
const editingOrderId = ref(null)
const noteForm = reactive({ merchant_note: '' })
const filters = reactive({ keyword: '', status: route.query.status || '', store_id: undefined })
const paidStatuses = ['received', 'accepted', 'completed', 'closed']
const activeQuickMode = ref(route.query.quick || 'all')
const autoRefreshEnabled = ref(true)
const lastRefreshAt = ref('')
const previousActionCount = ref(null)
let refreshTimer = null
const refreshIntervalMs = 20000

const pageReceivableAmount = computed(() => orders.value.reduce((sum, item) => sum + receivableAmount(item), 0))
const pagePendingPayAmount = computed(() => orders.value
  .filter((item) => ['pending', 'failed'].includes(item.status))
  .reduce((sum, item) => sum + receivableAmount(item), 0))
const pageConfirmingAmount = computed(() => orders.value
  .filter((item) => item.status === 'payment_confirming')
  .reduce((sum, item) => sum + receivableAmount(item), 0))
const pageNetReceived = computed(() => orders.value.reduce((sum, item) => sum + netReceived(item), 0))
const pageRefundAmount = computed(() => orders.value.reduce((sum, item) => sum + Number(item.refunded_amount || 0), 0))
const pageStatusCount = computed(() => orders.value.reduce((acc, item) => {
  acc[item.status] = (acc[item.status] || 0) + 1
  return acc
}, { pending: 0, failed: 0, submitted: 0, payment_confirming: 0, preparing: 0, received: 0, accepted: 0, completed: 0, closed: 0 }))
const pageRefundCount = computed(() => orders.value.filter((item) => hasRefundRisk(item)).length)
const pageNoteCount = computed(() => orders.value.filter((item) => Boolean(item.customer_note)).length)
const pagePaymentIssueCount = computed(() => orders.value.filter((item) => ['pending', 'failed'].includes(item.status)).length)
const overdueActionRows = computed(() => orders.value.filter((item) => isActionableOrder(item) && orderWaitMinutes(item) >= 10))
const urgentQueue = computed(() => priorityOrders.value
  .filter((item) => ['payment_confirming', 'submitted', 'received', 'preparing', 'accepted'].includes(item.status) || hasRefundRisk(item))
  .slice(0, 4))
const orderNotice = computed(() => {
  if (pageStatusCount.value.payment_confirming > 0) {
    return {
      visible: true,
      tone: 'warning',
      label: '优先处理',
      title: `${pageStatusCount.value.payment_confirming} 笔订单待确认收款`,
      text: '顾客已标记付款，确认到账后才会计入净实收。建议先处理这些订单。',
      button: '只看确认收款',
      go: () => setStatus('payment_confirming')
    }
  }
  const waitingCount = (pageStatusCount.value.submitted || 0) + (pageStatusCount.value.received || 0)
  if (overdueActionRows.value.length > 0) {
    return {
      visible: true,
      tone: 'warning',
      label: '超时待办',
      title: `${overdueActionRows.value.length} 笔订单已等待超过 10 分钟`,
      text: '建议先处理等待时间较长的订单，避免顾客反复咨询或漏单。',
      button: '只看超时',
      go: () => setQuickMode('overdue')
    }
  }
  if (waitingCount > 0) {
    return {
      visible: true,
      tone: 'primary',
      label: '待处理',
      title: `${waitingCount} 笔订单等待处理`,
      text: '建议优先确认订单并打印小票，避免顾客长时间等待。',
      button: '只看待处理',
      go: () => setQuickMode('received')
    }
  }
  if (pagePaymentIssueCount.value > 0) {
    return {
      visible: true,
      tone: 'pay',
      label: '支付异常',
      title: `${pagePaymentIssueCount.value} 笔订单支付未完成`,
      text: '这些订单暂不计入收入，顾客咨询时可引导其继续支付。',
      button: '只看异常',
      go: () => setQuickMode('pay_issue')
    }
  }
  return { visible: false }
})
const statusTabs = computed(() => [
  { label: '全部', value: '', count: total.value },
  { label: '待支付', value: 'pending', count: pageStatusCount.value.pending || 0 },
  { label: '已提交', value: 'submitted', count: pageStatusCount.value.submitted || 0 },
  { label: '待确认收款', value: 'payment_confirming', count: pageStatusCount.value.payment_confirming || 0 },
  { label: '处理中', value: 'preparing', count: pageStatusCount.value.preparing || 0 },
  { label: '待接单', value: 'received', count: pageStatusCount.value.received || 0 },
  { label: '已接单', value: 'accepted', count: pageStatusCount.value.accepted || 0 },
  { label: '已完成', value: 'completed', count: pageStatusCount.value.completed || 0 },
  { label: '已关闭', value: 'closed', count: pageStatusCount.value.closed || 0 },
  { label: '失败', value: 'failed', count: pageStatusCount.value.failed || 0 }
])
const quickModes = computed(() => [
  { label: '当前页全部', value: 'all', count: orders.value.length, hint: '按处理优先级排序', tone: 'primary' },
  { label: '支付异常', value: 'pay_issue', count: pagePaymentIssueCount.value, hint: '待支付/失败不处理', tone: 'pay' },
  { label: '确认收款', value: 'payment_confirming', count: pageStatusCount.value.payment_confirming || 0, hint: '核对到账后确认', tone: 'warning' },
  { label: '超时待办', value: 'overdue', count: overdueActionRows.value.length, hint: '等待超过 10 分钟', tone: 'danger' },
  { label: '待处理', value: 'received', count: (pageStatusCount.value.received || 0) + (pageStatusCount.value.submitted || 0), hint: '优先确认并打印', tone: 'warning' },
  { label: '处理中', value: 'accepted', count: (pageStatusCount.value.accepted || 0) + (pageStatusCount.value.preparing || 0), hint: '完成后及时标记', tone: 'success' },
  { label: '售后异常', value: 'refund', count: pageRefundCount.value, hint: '核对退款与实收', tone: 'danger' },
  { label: '顾客备注', value: 'note', count: pageNoteCount.value, hint: '处理前重点看', tone: 'note' }
])
const priorityOrders = computed(() => [...orders.value].sort((a, b) => {
  const priority = { payment_confirming: 0, submitted: 1, received: 1, preparing: 2, accepted: 2, pending: 3, failed: 4, completed: 5, closed: 6 }
  const priorityDiff = (priority[a.status] ?? 9) - (priority[b.status] ?? 9)
  if (priorityDiff !== 0) return priorityDiff
  if (isActionableOrder(a) || isActionableOrder(b)) return orderWaitMinutes(b) - orderWaitMinutes(a)
  return Number(new Date(b.created_at || 0)) - Number(new Date(a.created_at || 0))
}))
const visibleOrders = computed(() => {
  if (activeQuickMode.value === 'pay_issue') return priorityOrders.value.filter((item) => ['pending', 'failed'].includes(item.status))
  if (activeQuickMode.value === 'payment_confirming') return priorityOrders.value.filter((item) => item.status === 'payment_confirming')
  if (activeQuickMode.value === 'overdue') return priorityOrders.value.filter((item) => isActionableOrder(item) && orderWaitMinutes(item) >= 10)
  if (activeQuickMode.value === 'received') return priorityOrders.value.filter((item) => ['submitted', 'received'].includes(item.status))
  if (activeQuickMode.value === 'accepted') return priorityOrders.value.filter((item) => ['preparing', 'accepted'].includes(item.status))
  if (activeQuickMode.value === 'refund') return priorityOrders.value.filter((item) => hasRefundRisk(item))
  if (activeQuickMode.value === 'note') return priorityOrders.value.filter((item) => Boolean(item.customer_note))
  return priorityOrders.value
})
const selectedReceivedRows = computed(() => selectedRows.value.filter((item) => ['submitted', 'received'].includes(item.status)))
const selectedAcceptedRows = computed(() => selectedRows.value.filter((item) => ['preparing', 'accepted'].includes(item.status)))
const workflowNotice = computed(() => {
  if (pagePaymentIssueCount.value && activeQuickMode.value === 'pay_issue') {
    return { type: 'warning', title: `当前页有 ${pagePaymentIssueCount.value} 笔支付未完成订单。它们不计入实收，顾客可在订单状态页继续支付，商家无需提前处理。` }
  }
  if (pageStatusCount.value.payment_confirming) {
    return { type: 'warning', title: `当前页有 ${pageStatusCount.value.payment_confirming} 笔订单等待确认收款。请先核对支付宝/微信到账，再点击确认。` }
  }
  const waitingCount = (pageStatusCount.value.submitted || 0) + (pageStatusCount.value.received || 0)
  if (overdueActionRows.value.length) {
    return { type: 'warning', title: `当前页有 ${overdueActionRows.value.length} 笔超时待办。建议按等待时间优先处理，减少漏单和顾客催单。` }
  }
  if (waitingCount) {
    return { type: 'warning', title: `当前页有 ${waitingCount} 笔待处理订单，建议优先使用“接单并打印”，避免顾客等待。` }
  }
  if (pageRefundCount.value) {
    return { type: 'warning', title: `当前页有 ${pageRefundCount.value} 笔售后/退款订单，建议核对退款渠道和净实收。` }
  }
  return { type: 'info', title: '应收用于核对订单规模；待顾客付款、待确认收款不计入实收；退款后净额 = 已确认实收 - 已记录退款。' }
})
const emptyText = computed(() => activeQuickMode.value === 'all' ? '暂无扫码点单订单' : '当前快捷视图暂无订单')
const autoRefreshText = computed(() => (
  autoRefreshEnabled.value
    ? `已开启，每 ${Math.round(refreshIntervalMs / 1000)} 秒同步一次${lastRefreshAt.value ? `，上次 ${lastRefreshAt.value}` : ''}`
    : `已暂停${lastRefreshAt.value ? `，上次 ${lastRefreshAt.value}` : ''}`
))
const handoffSummary = computed(() => {
  const waitingCount = (pageStatusCount.value.submitted || 0) + (pageStatusCount.value.received || 0)
  const processingCount = (pageStatusCount.value.preparing || 0) + (pageStatusCount.value.accepted || 0)
  if (pageStatusCount.value.payment_confirming > 0) {
    return {
      title: `先确认 ${pageStatusCount.value.payment_confirming} 笔收款`,
      text: '顾客已标记付款，确认到账后订单才会计入实收和财务对账。',
      button: '只看确认收款',
      quick: 'payment_confirming'
    }
  }
  if (overdueActionRows.value.length > 0) {
    return {
      title: `先处理 ${overdueActionRows.value.length} 笔超时待办`,
      text: '这些订单已等待超过 10 分钟，建议按队列优先处理，避免漏单。',
      button: '只看超时待办',
      quick: 'overdue'
    }
  }
  if (waitingCount > 0) {
    return {
      title: `先处理 ${waitingCount} 笔待处理`,
      text: '订单已进入商家处理队列，建议确认后打印小票或继续处理。',
      button: '只看待处理',
      quick: 'received'
    }
  }
  if (processingCount > 0) {
    return {
      title: `核对 ${processingCount} 笔处理中订单`,
      text: '这些订单已经接单，完成后及时标记，财务和复购数据才准确。',
      button: '只看处理中',
      quick: 'accepted'
    }
  }
  if (pagePaymentIssueCount.value > 0) {
    return {
      title: `${pagePaymentIssueCount.value} 笔支付未完成`,
      text: '支付异常订单只做核对和顾客引导，不建议提前处理或计入收入。',
      button: '只看异常',
      quick: 'pay_issue'
    }
  }
  if (pageRefundCount.value > 0) {
    return {
      title: `${pageRefundCount.value} 笔售后需要核对`,
      text: '建议进入详情核对退款金额、原因和订单净实收。',
      button: '只看售后',
      quick: 'refund'
    }
  }
  return {
    title: '当前页暂无紧急待办',
    text: '可以继续查看财务对账或优化商品和活动。',
    button: '查看全部',
    quick: 'all'
  }
})
const cashierCards = computed(() => [
  { label: '应收', value: formatMoney(pageReceivableAmount.value), hint: `${orders.value.length} 笔订单账面金额`, tone: 'primary' },
  { label: '待付款', value: formatMoney(pagePendingPayAmount.value), hint: `${pagePaymentIssueCount.value} 笔未完成支付`, tone: pagePaymentIssueCount.value ? 'warning' : 'muted' },
  { label: '待确认', value: formatMoney(pageConfirmingAmount.value), hint: `${pageStatusCount.value.payment_confirming || 0} 笔需核对到账`, tone: pageStatusCount.value.payment_confirming ? 'warning' : 'muted' },
  { label: '已退款', value: formatMoney(pageRefundAmount.value), hint: `${pageRefundCount.value} 笔售后记录`, tone: pageRefundCount.value ? 'danger' : 'muted' },
  { label: '净实收', value: formatMoney(pageNetReceived.value), hint: '已确认实收 - 已记录退款', tone: 'success' }
])
const cashierHandoff = computed(() => {
  if (pageStatusCount.value.payment_confirming > 0) {
    return {
      title: '先核对顾客已标记付款的订单',
      text: `当前筛选范围还有 ${pageStatusCount.value.payment_confirming} 笔待确认收款，确认前不计入净实收。`,
      button: '只看待确认收款',
      quick: 'payment_confirming'
    }
  }
  if (pagePaymentIssueCount.value > 0) {
    return {
      title: '还有订单未完成支付',
      text: `${pagePaymentIssueCount.value} 笔待付款或支付失败订单暂不处理履约，顾客咨询时引导继续支付。`,
      button: '只看支付异常',
      quick: 'pay_issue'
    }
  }
  if (pageRefundCount.value > 0) {
    return {
      title: '退款订单需要交班核对',
      text: `当前筛选范围已记录退款 ${formatMoney(pageRefundAmount.value)}，请确认真实退款渠道和净实收一致。`,
      button: '只看售后异常',
      quick: 'refund'
    }
  }
  return {
    title: '当前筛选范围收银口径清楚',
    text: `净实收 ${formatMoney(pageNetReceived.value)}，暂无待确认收款。收店前可进入财务对账做日期维度复核。`,
    button: '查看全部订单',
    quick: 'all'
  }
})

const receivableAmount = (row) => Number(row.total_amount ?? row.amount ?? 0)
const orderAmount = (row) => Number(row.paid_amount ?? row.total_amount ?? row.amount ?? 0)
const isPaidOrder = (row) => Boolean(row.paid_at) || paidStatuses.includes(row.status)
const netReceived = (row) => {
  if (!isPaidOrder(row)) return 0
  return Math.max(orderAmount(row) - Number(row.refunded_amount || 0), 0)
}
const paymentLedgerTitle = (row) => {
  if (['pending', 'failed'].includes(row.status)) return '待顾客付款'
  if (row.status === 'payment_confirming') return '待确认收款'
  if (isPaidOrder(row)) return '已确认实收'
  if (['submitted', 'preparing'].includes(row.status)) return '已提交未结算'
  return '不计入实收'
}
const paymentLedgerTone = (row) => {
  if (['pending', 'failed'].includes(row.status)) return 'pending'
  if (row.status === 'payment_confirming') return 'confirming'
  if (isPaidOrder(row)) return 'confirmed'
  if (['submitted', 'preparing'].includes(row.status)) return 'submitted'
  return 'muted'
}
const paymentLedgerHint = (row) => {
  if (['pending', 'failed'].includes(row.status)) return '顾客未完成付款'
  if (row.status === 'payment_confirming') return '需核对到账后确认'
  if (isPaidOrder(row)) return '已进入财务实收'
  if (['submitted', 'preparing'].includes(row.status)) return '可后续统一结算'
  return '关闭或异常订单'
}
const netAmountHint = (row) => {
  if (!isPaidOrder(row)) return '未确认收款'
  if (Number(row.refunded_amount || 0) > 0) return '已扣除退款'
  return '无退款'
}
const hasRefundRisk = (row) => ['partial', 'full'].includes(row.refund_status) || Number(row.refunded_amount || 0) > 0
const itemQuantity = (row) => (Array.isArray(row.items) ? row.items : []).reduce((sum, item) => sum + Number(item.quantity || 0), 0)
const pickupCode = (row) => {
  const text = String(row.order_no || row.id || '')
  return text ? text.slice(-4).toUpperCase() : '-'
}
const isActionableOrder = (row) => ['payment_confirming', 'submitted', 'received', 'preparing', 'accepted'].includes(row.status)
const escapeHtml = (value) => String(value ?? '').replace(/[&<>"']/g, (char) => ({
  '&': '&amp;',
  '<': '&lt;',
  '>': '&gt;',
  '"': '&quot;',
  "'": '&#39;'
}[char]))
const confirmPaymentHtml = (row) => `
  <div class="confirm-payment-card">
    <p>请先打开商家的支付宝/微信收款账户，确认该笔款项已经到账，再点击确认。</p>
    <div><span>应收金额</span><strong>${escapeHtml(formatMoney(row.total_amount || row.amount))}</strong></div>
    <div><span>订单号</span><strong>${escapeHtml(row.order_no || '-')}</strong></div>
    <div><span>核对码</span><strong>${escapeHtml(pickupCode(row))}</strong></div>
    <div><span>顾客手机号</span><strong>${escapeHtml(row.customer_phone || '未留手机号')}</strong></div>
    <div><span>商品数量</span><strong>${escapeHtml(itemQuantity(row))} 件</strong></div>
  </div>
`
const orderWaitMinutes = (row) => {
  const baseTime = row.paid_at || row.created_at
  const timestamp = baseTime ? new Date(baseTime).getTime() : 0
  if (!timestamp) return 0
  return Math.max(Math.floor((Date.now() - timestamp) / 60000), 0)
}
const waitText = (row) => {
  const minutes = orderWaitMinutes(row)
  if (minutes < 60) return `${minutes} 分钟`
  const hours = Math.floor(minutes / 60)
  const rest = minutes % 60
  return rest ? `${hours} 小时 ${rest} 分钟` : `${hours} 小时`
}
const actionAgeLabel = (row) => {
  if (row.status === 'payment_confirming') return '待确认'
  if (['submitted', 'received'].includes(row.status)) return '待处理'
  if (['preparing', 'accepted'].includes(row.status)) return '处理中'
  return '已等待'
}
const orderRowClass = ({ row }) => {
  if (['payment_confirming', 'submitted', 'received'].includes(row.status) && orderWaitMinutes(row) >= 10) return 'row-urgent'
  if (['payment_confirming', 'submitted', 'received'].includes(row.status)) return 'row-received'
  if (Number(row.refunded_amount || 0) > 0) return 'row-refunded'
  return ''
}
const queueLabel = (row) => {
  if (row.status === 'payment_confirming') return '待确认收款'
  if (['submitted', 'received'].includes(row.status)) return '待处理'
  if (['preparing', 'accepted'].includes(row.status)) return '处理中'
  if (hasRefundRisk(row)) return '售后核对'
  return statusLabel(row.status)
}
const queueTone = (row) => {
  if (row.status === 'payment_confirming') return 'confirming'
  if (['submitted', 'received'].includes(row.status)) return orderWaitMinutes(row) >= 10 ? 'urgent' : 'received'
  if (['preparing', 'accepted'].includes(row.status)) return 'processing'
  if (hasRefundRisk(row)) return 'refund'
  return ''
}

const loadStores = async () => {
  const res = await fetchMerchantStores({ page: 1, page_size: 100 })
  stores.value = res.data.list || []
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
      message: `订单待办增加 ${nextCount - previousCount} 笔，请优先核对收款或处理订单。`,
      showClose: true,
      duration: 5000
    })
  }
}

const loadOrders = async ({ silent = false, notify = false } = {}) => {
  if (!silent) loading.value = true
  try {
    const params = { page: page.value, page_size: pageSize }
    if (filters.keyword) params.keyword = filters.keyword
    if (filters.status) params.status = filters.status
    if (filters.store_id) params.store_id = filters.store_id
    const res = await fetchMerchantOrders(params)
    const nextOrders = res.data.list || []
    orders.value = nextOrders
    total.value = res.data.total || 0
    updateRefreshSnapshot(nextOrders, notify)
  } finally {
    if (!silent) loading.value = false
  }
}

const reload = async () => {
  page.value = 1
  await loadOrders()
}

const setStatus = async (status) => {
  filters.status = status
  activeQuickMode.value = 'all'
  await reload()
}

const setQuickMode = async (mode) => {
  activeQuickMode.value = mode
  if (filters.status) {
    filters.status = ''
    page.value = 1
    await loadOrders()
  }
}

const resetFilters = async () => {
  filters.keyword = ''
  filters.status = ''
  filters.store_id = undefined
  activeQuickMode.value = 'all'
  await reload()
}

const changePage = async (nextPage) => {
  page.value = nextPage
  await loadOrders()
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
    loadOrders({ silent: true, notify: true })
  }, refreshIntervalMs)
}

const toggleAutoRefresh = () => {
  if (autoRefreshEnabled.value) {
    loadOrders({ silent: true })
    startAutoRefresh()
    return
  }
  stopAutoRefresh()
}

const goDetail = (row) => {
  router.push(`/merchant/orders/${row.id}`)
}

const handleSelectionChange = (rows) => {
  selectedRows.value = rows
}

const clearSelection = () => {
  tableRef.value?.clearSelection?.()
  selectedRows.value = []
}

const accept = async (row) => {
  await acceptMerchantOrder(row.id)
  ElMessage.success('订单已接单')
  await loadOrders()
}

const confirmPayment = async (row) => {
  await ElMessageBox.confirm(confirmPaymentHtml(row), '确认收款核对', {
    type: 'warning',
    dangerouslyUseHTMLString: true,
    confirmButtonText: '已核对到账，确认收款',
    cancelButtonText: '先不确认'
  })
  await confirmMerchantOrderPayment(row.id)
  ElMessage.success('已确认收款，订单进入已付款处理流程')
  await loadOrders()
}

const acceptAndPrint = async (row) => {
  if (printReceipt(row)) {
    await accept(row)
  }
}

const complete = async (row) => {
  await completeMerchantOrder(row.id)
  ElMessage.success('订单已完成')
  await loadOrders()
}

const batchAccept = async () => {
  const rows = [...selectedReceivedRows.value]
  if (!rows.length) return
  await ElMessageBox.confirm(`确认批量接单 ${rows.length} 笔订单？`, '批量接单', { type: 'warning' })
  for (const row of rows) {
    await acceptMerchantOrder(row.id)
  }
  ElMessage.success(`已接单 ${rows.length} 笔订单`)
  clearSelection()
  await loadOrders()
}

const batchComplete = async () => {
  const rows = [...selectedAcceptedRows.value]
  if (!rows.length) return
  await ElMessageBox.confirm(`确认批量完成 ${rows.length} 笔履约中订单？`, '批量完成', { type: 'warning' })
  for (const row of rows) {
    await completeMerchantOrder(row.id)
  }
  ElMessage.success(`已完成 ${rows.length} 笔订单`)
  clearSelection()
  await loadOrders()
}

const close = async (row) => {
  await ElMessageBox.confirm('确认关闭这笔订单吗？关闭后不再计入待接单。', '关闭订单', { type: 'warning' })
  await closeMerchantOrder(row.id)
  ElMessage.success('订单已关闭')
  await loadOrders()
}

const openNote = (row) => {
  editingOrderId.value = row.id
  noteForm.merchant_note = row.merchant_note || ''
  noteDialogVisible.value = true
}

const saveNote = async () => {
  await updateMerchantOrderNote(editingOrderId.value, { merchant_note: noteForm.merchant_note })
  ElMessage.success('备注已保存')
  noteDialogVisible.value = false
  await loadOrders()
}

const printReceipt = (row) => {
  if (!printStoreOrderReceipt(row)) {
    ElMessage.warning('浏览器阻止了打印窗口，请允许弹窗后重试')
    return false
  }
  return true
}

const batchPrint = () => {
  let printedCount = 0
  selectedRows.value.forEach((row) => {
    if (printReceipt(row)) printedCount += 1
  })
  if (printedCount) ElMessage.success(`已打开 ${printedCount} 张小票打印窗口`)
}

const callCustomer = (row) => {
  if (!row.customer_phone) return
  window.location.href = `tel:${row.customer_phone}`
}

const canClose = (row) => ['submitted', 'payment_confirming', 'received', 'preparing', 'accepted'].includes(row.status)
const formatMoney = (value) => `¥${(Number(value || 0) / 100).toFixed(2)}`
const formatTime = (value) => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '-'
const statusLabel = (status) => ({
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
const statusType = (status) => ({
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
const refundLabel = (status) => ({
  partial: '部分退款',
  full: '全额退款',
  none: '未退款'
}[status] || '未退款')
const refundType = (status) => ({
  partial: 'warning',
  full: 'success'
}[status] || 'info')

onMounted(async () => {
  await Promise.all([loadStores(), loadOrders()])
  startAutoRefresh()
})

onUnmounted(() => {
  stopAutoRefresh()
})
</script>

<style scoped>
.orders-page {
  display: grid;
  gap: 16px;
}

.orders-summary,
.metric-grid {
  display: grid;
  gap: 14px;
}

.orders-summary {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.summary-more {
  grid-column: 1 / -1;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  overflow: hidden;
  background: #ffffff;
}

.metric-grid {
  grid-template-columns: repeat(5, minmax(0, 1fr));
}

.metric-grid.compact {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.cashier-handoff {
  display: grid;
  grid-template-columns: minmax(260px, 0.9fr) minmax(0, 1.7fr) auto;
  gap: 14px;
  align-items: stretch;
  padding: 16px;
  border: 1px solid #bfdbfe;
  border-radius: 8px;
  background: linear-gradient(90deg, #eff6ff, #ffffff);
}

.handoff-main,
.handoff-actions,
.handoff-ledger article {
  display: grid;
  align-content: center;
}

.handoff-main span,
.handoff-main strong,
.handoff-main p,
.handoff-ledger span,
.handoff-ledger strong,
.handoff-ledger small {
  display: block;
}

.handoff-main span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.14em;
}

.handoff-main strong {
  margin-top: 8px;
  color: #0f2747;
  font-size: 20px;
}

.handoff-main p {
  margin: 8px 0 0;
  color: #64748b;
  line-height: 1.6;
}

.handoff-ledger {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 10px;
}

.handoff-ledger article {
  min-height: 86px;
  padding: 12px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #ffffff;
}

.handoff-ledger article.warning {
  border-color: #fed7aa;
  background: #fff7ed;
}

.handoff-ledger article.danger {
  border-color: #fecaca;
  background: #fef2f2;
}

.handoff-ledger article.success {
  border-color: #bbf7d0;
  background: #f0fdf4;
}

.handoff-ledger article.primary {
  border-color: #bfdbfe;
  background: #eff6ff;
}

.handoff-ledger span {
  color: #64748b;
  font-size: 12px;
  font-weight: 800;
}

.handoff-ledger strong {
  margin-top: 6px;
  color: #0f2747;
  font-size: 18px;
}

.handoff-ledger small {
  margin-top: 4px;
  color: #64748b;
  line-height: 1.4;
}

.handoff-actions {
  gap: 8px;
  min-width: 132px;
}

.status-tabs {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(118px, 1fr));
  gap: 10px;
  margin-bottom: 14px;
}

.status-tabs button {
  min-height: 70px;
  padding: 12px;
  text-align: left;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  background: #f8fbff;
  cursor: pointer;
  transition: 0.2s ease;
}

.status-tabs button.active,
.status-tabs button:hover {
  border-color: #60a5fa;
  background: #eff6ff;
  box-shadow: 0 12px 28px rgba(37, 99, 235, 0.1);
}

.status-tabs span,
.status-tabs strong {
  display: block;
}

.status-tabs span {
  color: #64748b;
  font-size: 12px;
}

.status-tabs strong {
  margin-top: 6px;
  color: #0f2747;
  font-size: 22px;
}

.quick-workbench {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(148px, 1fr));
  gap: 10px;
  margin-bottom: 14px;
}

.workflow-more {
  margin-bottom: 14px;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  overflow: hidden;
  background: #ffffff;
}

.quick-card {
  min-height: 88px;
  padding: 12px;
  text-align: left;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #ffffff;
  cursor: pointer;
  transition: 0.2s ease;
}

.quick-card span,
.quick-card strong,
.quick-card small {
  display: block;
}

.quick-card span {
  color: #64748b;
  font-size: 12px;
  font-weight: 700;
}

.quick-card strong {
  margin: 6px 0 4px;
  color: #0f172a;
  font-size: 22px;
}

.quick-card small {
  color: #64748b;
  line-height: 1.4;
}

.quick-card:hover,
.quick-card.active {
  transform: translateY(-1px);
  box-shadow: 0 12px 26px rgba(15, 23, 42, 0.08);
}

.quick-card.primary.active {
  border-color: #2563eb;
  background: #eff6ff;
}

.quick-card.pay.active,
.quick-card.pay:hover {
  border-color: #f97316;
  background: #fff7ed;
}

.quick-card.warning.active,
.quick-card.warning:hover {
  border-color: #f59e0b;
  background: #fffbeb;
}

.quick-card.success.active,
.quick-card.success:hover {
  border-color: #10b981;
  background: #ecfdf5;
}

.quick-card.danger.active,
.quick-card.danger:hover {
  border-color: #ef4444;
  background: #fef2f2;
}

.quick-card.note.active,
.quick-card.note:hover {
  border-color: #f97316;
  background: #fff7ed;
}

.filter-row,
.action-row {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.refresh-strip {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 13px 16px;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  background: #f8fbff;
}

.refresh-strip span,
.refresh-strip strong,
.refresh-strip p {
  display: block;
}

.refresh-strip span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.08em;
}

.refresh-strip strong {
  margin-top: 4px;
  color: #0f2747;
}

.refresh-strip p {
  margin: 4px 0 0;
  color: #64748b;
  line-height: 1.55;
}

.refresh-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.queue-board {
  display: grid;
  gap: 12px;
  padding: 16px;
  border: 1px solid #bfdbfe;
  border-radius: 8px;
  background:
    radial-gradient(circle at 100% 0%, rgba(37, 99, 235, 0.1), transparent 34%),
    #ffffff;
}

.queue-head {
  display: flex;
  justify-content: space-between;
  gap: 14px;
  align-items: flex-start;
}

.queue-head span,
.queue-head strong,
.queue-head p,
.queue-main span,
.queue-main strong,
.queue-main p {
  display: block;
}

.queue-head span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.08em;
}

.queue-head strong {
  margin-top: 4px;
  color: #0f2747;
  font-size: 18px;
}

.queue-head p {
  margin: 5px 0 0;
  color: #64748b;
  line-height: 1.55;
}

.queue-list {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.queue-list article {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: flex-start;
  min-height: 106px;
  padding: 12px;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  background: #f8fbff;
}

.queue-list article.confirming,
.queue-list article.urgent {
  border-color: #f59e0b;
  background: #fffbeb;
}

.queue-list article.processing {
  border-color: #bbf7d0;
  background: #ecfdf5;
}

.queue-list article.refund {
  border-color: #fed7aa;
  background: #fff7ed;
}

.queue-main {
  min-width: 0;
}

.queue-main span {
  color: #64748b;
  font-size: 12px;
  font-weight: 900;
}

.queue-main strong {
  margin-top: 5px;
  color: #0f2747;
  font-size: 17px;
}

.queue-main p {
  margin: 5px 0 0;
  color: #64748b;
  line-height: 1.45;
}

.queue-actions {
  display: flex;
  flex-direction: column;
  gap: 6px;
  flex: 0 0 auto;
}

.order-notice {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 16px 18px;
  border: 1px solid #fed7aa;
  border-radius: 8px;
  background:
    radial-gradient(circle at 100% 0%, rgba(249, 115, 22, 0.13), transparent 34%),
    linear-gradient(135deg, #fff7ed, #ffffff);
  box-shadow: 0 14px 32px rgba(146, 64, 14, 0.08);
}

.order-notice.primary {
  border-color: #bfdbfe;
  background:
    radial-gradient(circle at 100% 0%, rgba(37, 99, 235, 0.12), transparent 34%),
    linear-gradient(135deg, #eff6ff, #ffffff);
}

.order-notice.pay {
  border-color: #fde68a;
  background:
    radial-gradient(circle at 100% 0%, rgba(245, 158, 11, 0.14), transparent 34%),
    linear-gradient(135deg, #fffbeb, #ffffff);
}

.order-notice span,
.order-notice strong,
.order-notice p {
  display: block;
}

.order-notice span {
  color: #f97316;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.08em;
}

.order-notice.primary span {
  color: #2563eb;
}

.order-notice strong {
  margin-top: 4px;
  color: #0f2747;
  font-size: 18px;
}

.order-notice p {
  margin: 5px 0 0;
  color: #64748b;
  line-height: 1.6;
}

.handoff-strip {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  margin-bottom: 14px;
  padding: 14px 16px;
  border: 1px solid #bfdbfe;
  border-radius: 18px;
  background: linear-gradient(135deg, #eff6ff, #ffffff);
}

.handoff-strip span,
.handoff-strip strong,
.handoff-strip p {
  display: block;
}

.handoff-strip span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.08em;
}

.handoff-strip strong {
  margin-top: 5px;
  color: #0f2747;
  font-size: 18px;
}

.handoff-strip p {
  margin: 6px 0 0;
  color: #64748b;
  line-height: 1.55;
}

.filter-row {
  margin-bottom: 12px;
}

.order-tip {
  margin-bottom: 14px;
}

.batch-strip {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  margin-bottom: 14px;
  padding: 12px 14px;
  border: 1px solid #bfdbfe;
  border-radius: 8px;
  background: #eff6ff;
}

.batch-summary,
.batch-summary span,
.batch-summary strong,
.batch-summary small {
  display: block;
}

.batch-summary span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 800;
}

.batch-summary strong {
  margin-top: 4px;
  color: #0f2747;
}

.batch-summary small {
  margin-top: 3px;
  color: #64748b;
}

.batch-actions,
.phone-line {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.keyword-input {
  width: 310px;
}

.filter-item {
  width: 160px;
}

.order-main,
.store-cell,
.money-cell,
.items-preview,
.status-cell {
  display: grid;
  gap: 3px;
  line-height: 1.55;
}

.pickup-code {
  display: inline-flex;
  width: fit-content;
  margin-top: 3px;
  padding: 3px 8px;
  border: 1px solid #bfdbfe;
  border-radius: 999px;
  color: #1d4ed8;
  background: #eff6ff;
  font-size: 12px;
  font-weight: 800;
}

.pickup-code.urgent {
  border-color: #2563eb;
  color: #ffffff;
  background: #2563eb;
}

.wait-time {
  display: inline-flex;
  width: fit-content;
  padding: 2px 6px;
  border-radius: 999px;
  background: #f8fafc;
}

.wait-time.urgent {
  color: #b45309;
  background: #fffbeb;
  font-weight: 800;
}

.items-count {
  color: #0f2747;
  font-size: 13px;
}

.order-main small,
.store-cell span,
.money-cell span,
.money-cell small,
.muted {
  color: #64748b;
  font-size: 12px;
}

.risk-text {
  color: #b45309;
  font-size: 12px;
  font-weight: 700;
}

.money-cell strong {
  color: #0f2747;
}

.ledger-title {
  width: fit-content;
  padding: 2px 7px;
  border-radius: 999px;
  font-size: 12px;
}

.ledger-title.pending,
.ledger-title.confirming {
  color: #92400e;
  background: #fffbeb;
}

.ledger-title.confirmed {
  color: #047857;
  background: #ecfdf5;
}

.ledger-title.submitted {
  color: #1d4ed8;
  background: #eff6ff;
}

.ledger-title.muted {
  color: #64748b;
  background: #f8fafc;
}

.customer-note {
  color: #94a3b8;
}

.customer-note.active {
  display: inline-flex;
  max-width: 100%;
  padding: 4px 8px;
  border: 1px solid #fed7aa;
  border-radius: 999px;
  color: #9a3412;
  background: #fff7ed;
  font-weight: 700;
}

:deep(.row-received) {
  --el-table-tr-bg-color: #eff6ff;
}

:deep(.row-received td:first-child) {
  border-left: 4px solid #2563eb;
}

:deep(.row-urgent) {
  --el-table-tr-bg-color: #fffbeb;
}

:deep(.row-urgent td:first-child) {
  border-left: 4px solid #f59e0b;
}

:deep(.row-refunded) {
  --el-table-tr-bg-color: #fff7ed;
}

:global(.confirm-payment-card) {
  display: grid;
  gap: 8px;
  color: #1f2937;
}

:global(.confirm-payment-card p) {
  margin: 0 0 4px;
  color: #92400e;
  line-height: 1.6;
}

:global(.confirm-payment-card div) {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 10px;
  border: 1px solid #fde68a;
  border-radius: 10px;
  background: #fffbeb;
}

:global(.confirm-payment-card span) {
  color: #6b7280;
}

:global(.confirm-payment-card strong) {
  text-align: right;
  color: #111827;
}

.orders-pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

@media (max-width: 1180px) {
  .orders-summary,
  .metric-grid,
  .cashier-handoff,
  .status-tabs,
  .quick-workbench {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .handoff-ledger {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 760px) {
  .orders-summary,
  .metric-grid,
  .cashier-handoff,
  .handoff-ledger,
  .status-tabs,
  .quick-workbench {
    grid-template-columns: 1fr;
  }

  .handoff-strip {
    align-items: flex-start;
    flex-direction: column;
  }

  .order-notice {
    align-items: stretch;
    flex-direction: column;
  }

  .refresh-strip {
    align-items: stretch;
    flex-direction: column;
  }

  .queue-head,
  .queue-list article {
    flex-direction: column;
  }

  .queue-list {
    grid-template-columns: 1fr;
  }

  .queue-actions {
    flex-direction: row;
    flex-wrap: wrap;
  }

  .batch-strip {
    align-items: stretch;
    flex-direction: column;
  }

  .keyword-input,
  .filter-item {
    width: 100%;
  }
}
</style>
