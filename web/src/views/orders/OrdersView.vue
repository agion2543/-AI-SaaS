<template>
  <div class="finance-page">
    <PageHero
      eyebrow="PLATFORM FINANCE"
      title="订单财务"
      description="统一审查平台订阅收入、顾客扫码交易流水、支付记录和退款售后记录。订阅确认到账统一进入订阅营收管理处理，避免和顾客订单混在一起。"
      compact
    >
      <template #actions>
        <el-button plain @click="goSubscriptionRevenue">订阅营收管理</el-button>
        <el-button plain @click="exportOrdersXlsx">导出订单 XLSX</el-button>
        <el-button type="primary" :loading="loading" @click="load">刷新</el-button>
      </template>
    </PageHero>

    <section class="tips-grid">
      <ActionCard
        label="资金口径"
        title="收入分类核算"
        description="平台订阅收入、顾客交易流水、退款金额和商家净收入分开统计，避免 SaaS 订阅和门店订单混在一起。"
        tone="blue"
      />
      <ActionCard
        label="顾客交易"
        title="只统计已支付"
        description="扫码点单订单以已支付、待接单、已接单、已完成等状态计入交易流水，待支付订单不计入实收。"
        tone="green"
      />
      <ActionCard
        label="售后记录"
        title="退款先做留痕"
        description="当前退款先做财务留痕，真实退款仍需在支付宝、微信或实际收款账户中同步处理。"
        tone="orange"
      />
    </section>

    <DataPanel title="筛选条件" description="按时间、关键词、订单类型和状态查看平台订单财务。">
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
          <el-option label="支付失败" value="failed" />
        </el-select>
        <el-select v-model="filters.exception" clearable placeholder="异常类型">
          <el-option label="订阅待确认" value="subscription_pending" />
          <el-option label="支付异常" value="payment" />
          <el-option label="待接单" value="received" />
          <el-option label="退款售后" value="refund" />
          <el-option label="待结算" value="settlement" />
          <el-option label="关闭订单" value="closed" />
        </el-select>
        <el-button @click="resetFilters">重置</el-button>
      </div>
    </DataPanel>

    <section class="metric-grid">
      <MetricCard label="平台订阅收入" :value="formatMoney(platformSubscriptionIncome)" hint="商家月付/年付已支付订单。" tone="primary" />
      <MetricCard label="顾客交易流水" :value="formatMoney(customerTradeIncome)" hint="顾客扫码点单已支付金额。" tone="success" />
      <MetricCard label="售后退款" :value="formatMoney(totalRefunded)" hint="系统记录的退款金额。" tone="warning" />
      <MetricCard label="顾客交易净额" :value="formatMoney(customerTradeNet)" hint="交易流水减退款，用于商家结算参考。" tone="cyan" />
    </section>

    <section class="exception-strip">
      <button
        v-for="item in exceptionQuickFilters"
        :key="item.value || 'all'"
        type="button"
        :class="{ active: filters.exception === item.value }"
        @click="filters.exception = item.value"
      >
        <span>{{ item.label }}</span>
        <strong>{{ item.count }}</strong>
        <small>{{ item.hint }}</small>
      </button>
    </section>

    <DataPanel title="财务明细" description="订单列表、支付记录和退款售后集中管理。">
      <el-tabs>
        <el-tab-pane :label="`订阅收款待确认 ${pendingSubscriptionOrders.length}`">
          <el-table :data="pendingSubscriptionOrders" empty-text="暂无待确认订阅收款">
            <el-table-column prop="order_no" label="订阅订单号" min-width="190" />
            <el-table-column label="商家" min-width="180">
              <template #default="{ row }">{{ row.merchant?.name || row.merchant_id || '-' }}</template>
            </el-table-column>
            <el-table-column label="套餐" width="140">
              <template #default="{ row }">{{ row.merchant_plan?.name || '-' }}</template>
            </el-table-column>
            <el-table-column label="金额" width="120">
              <template #default="{ row }">{{ formatMoney(row.total_amount || row.amount) }}</template>
            </el-table-column>
            <el-table-column label="支付方式" width="130">
              <template #default="{ row }">{{ paymentChannelLabel(row.payment_channel) }}</template>
            </el-table-column>
            <el-table-column label="状态" width="120">
              <template #default="{ row }">
                <el-tag type="warning">{{ statusLabel(row.status) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="处理说明" min-width="260">
              <template #default>商家已生成订阅付款单，平台核对收款账户到账后再确认开通。</template>
            </el-table-column>
            <el-table-column label="创建时间" min-width="170">
              <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
            </el-table-column>
            <el-table-column label="操作" width="150" fixed="right">
              <template #default="{ row }">
                <el-button
                  size="small"
                  type="primary"
                  plain
                  @click="goSubscriptionRevenue(row)"
                >
                  去确认
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>

        <el-tab-pane label="异常中心">
          <el-table :data="exceptionOrders" empty-text="暂无异常订单">
            <el-table-column label="风险" width="120">
              <template #default="{ row }">
                <el-tag :type="exceptionLevel(row).type">{{ exceptionLevel(row).label }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="order_no" label="订单号" min-width="190" />
            <el-table-column label="商家 / 门店" min-width="190">
              <template #default="{ row }">{{ row.merchant?.name || '-' }} / {{ row.store?.name || '-' }}</template>
            </el-table-column>
            <el-table-column label="异常原因" min-width="220">
              <template #default="{ row }">
                <div class="exception-tags">
                  <el-tag v-for="tag in exceptionTags(row)" :key="tag" size="small" :type="tagType(tag)">{{ tag }}</el-tag>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="金额" width="120">
              <template #default="{ row }">{{ formatMoney(row.total_amount || row.amount) }}</template>
            </el-table-column>
            <el-table-column label="状态" width="110">
              <template #default="{ row }">
                <el-tag :type="statusType(row.status)">{{ statusLabel(row.status) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="处理建议" min-width="240">
              <template #default="{ row }">{{ exceptionAdvice(row) }}</template>
            </el-table-column>
            <el-table-column label="创建时间" min-width="170">
              <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
            </el-table-column>
            <el-table-column label="操作" width="210" fixed="right">
              <template #default="{ row }">
                <div class="table-actions">
                  <el-button size="small" type="primary" plain :disabled="!merchantID(row)" @click="createExceptionFollowUp(row)">跟进</el-button>
                  <el-button size="small" :disabled="!canRefund(row)" @click="openRefund(row)">退款</el-button>
                </div>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>

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
            <el-table-column label="操作" width="210" fixed="right">
              <template #default="{ row }">
                <div class="table-actions">
                  <el-button
                    v-if="canConfirmSubscription(row)"
                    size="small"
                    type="primary"
                    plain
                    @click="goSubscriptionRevenue(row)"
                  >
                    去订阅营收确认
                  </el-button>
                  <el-button size="small" :disabled="!canRefund(row)" @click="openRefund(row)">退款</el-button>
                </div>
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
                <el-button size="small" :disabled="!canRefund(row.order || {})" @click="openRefund(row.order)">退款</el-button>
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
    </DataPanel>

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
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  createAdminMerchantFollowUp,
  fetchOrders,
  fetchPayments,
  fetchRefunds,
  refundAdminOrder
} from '../../api/modules'
import ActionCard from '../../components/design/ActionCard.vue'
import DataPanel from '../../components/design/DataPanel.vue'
import MetricCard from '../../components/design/MetricCard.vue'
import PageHero from '../../components/design/PageHero.vue'
import { exportRowsToXlsx } from '../../utils/xlsx'

const orders = ref([])
const route = useRoute()
const router = useRouter()
const payments = ref([])
const refunds = ref([])
const loading = ref(false)
const refundDialogVisible = ref(false)
const refundSaving = ref(false)
const refundOrder = ref(null)
const refundForm = reactive({ full: true, amount_yuan: 0, reason: '' })
const filters = reactive({ dateRange: [], keyword: '', orderType: '', status: '', exception: '' })

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
const pendingSubscriptionOrders = computed(() => filteredOrders.value
  .filter((item) => item.order_type === 'merchant_subscription' && item.status === 'pending')
  .sort((a, b) => dateValue(b.created_at) - dateValue(a.created_at)))
const customerTradeIncome = computed(() => filteredOrders.value
  .filter((item) => item.order_type === 'store_order' && paidStoreStatuses.includes(item.status))
  .reduce((sum, item) => sum + Number(item.total_amount || item.amount || 0), 0))
const totalRefunded = computed(() => filteredRefunds.value.reduce((sum, item) => sum + Number(item.amount || 0), 0))
const customerTradeNet = computed(() => Math.max(customerTradeIncome.value - totalRefunded.value, 0))
const allExceptionOrders = computed(() => orders.value
  .filter((item) => item.order_type === 'store_order' && exceptionTags(item).length)
  .sort((a, b) => exceptionLevel(b).score - exceptionLevel(a).score || dateValue(b.created_at) - dateValue(a.created_at)))
const exceptionOrders = computed(() => filteredOrders.value
  .filter((item) => item.order_type === 'store_order' && exceptionTags(item).length)
  .sort((a, b) => exceptionLevel(b).score - exceptionLevel(a).score || dateValue(b.created_at) - dateValue(a.created_at)))
const exceptionQuickFilters = computed(() => [
  { label: '全部异常', value: '', count: allExceptionOrders.value.length, hint: '按风险优先展示' },
  { label: '订阅待确认', value: 'subscription_pending', count: orders.value.filter((item) => item.order_type === 'merchant_subscription' && item.status === 'pending').length, hint: '平台收款码待核对' },
  { label: '支付异常', value: 'payment', count: orders.value.filter((item) => item.order_type === 'store_order' && ['pending', 'failed'].includes(item.status)).length, hint: '待支付或支付失败' },
  { label: '待接单', value: 'received', count: orders.value.filter((item) => item.order_type === 'store_order' && item.status === 'received').length, hint: '顾客已付款，商家未处理' },
  { label: '退款售后', value: 'refund', count: orders.value.filter((item) => Number(item.refunded_amount || 0) > 0 || ['partial', 'full'].includes(item.refund_status)).length, hint: '需核对真实退款渠道' },
  { label: '待结算', value: 'settlement', count: orders.value.filter((item) => isUnsettledOrder(item)).length, hint: '已形成实收，未入结算' }
])

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
  filters.exception = ''
}

const matchOrderFilters = (item) => {
  if (!matchDate(item.created_at)) return false
  if (filters.orderType && item.order_type !== filters.orderType) return false
  if (filters.status && item.status !== filters.status) return false
  if (filters.exception && !matchExceptionFilter(item, filters.exception)) return false
  if (filters.keyword) {
    const text = `${item.order_no || ''} ${item.merchant?.name || ''} ${item.user?.phone || ''} ${item.customer_phone || ''}`.toLowerCase()
    if (!text.includes(filters.keyword.toLowerCase())) return false
  }
  return true
}

const matchExceptionFilter = (item, value) => {
  if (value === 'subscription_pending') return item.order_type === 'merchant_subscription' && item.status === 'pending'
  if (value === 'payment') return ['pending', 'failed'].includes(item.status)
  if (value === 'received') return item.status === 'received'
  if (value === 'refund') return Number(item.refunded_amount || 0) > 0 || ['partial', 'full'].includes(item.refund_status)
  if (value === 'settlement') return isUnsettledOrder(item)
  if (value === 'closed') return item.status === 'closed'
  return true
}

const matchDate = (value) => {
  if (!filters.dateRange?.length || !value) return true
  const day = String(value).slice(0, 10)
  return day >= filters.dateRange[0] && day <= filters.dateRange[1]
}

const isUnsettledOrder = (row) => row?.order_type === 'store_order' && !row.settlement_id && ['received', 'accepted', 'completed'].includes(row.status)
const exceptionTags = (row) => {
  const tags = []
  if (row.status === 'failed') tags.push('支付失败')
  if (row.status === 'pending') tags.push('待支付')
  if (row.status === 'received') tags.push('待接单')
  if (row.status === 'closed') tags.push('已关闭')
  if (Number(row.refunded_amount || 0) > 0 || ['partial', 'full'].includes(row.refund_status)) tags.push('退款售后')
  if (isUnsettledOrder(row)) tags.push('待结算')
  return tags
}
const exceptionLevel = (row) => {
  if (row.status === 'failed' || (row.status === 'received' && orderAgeHours(row) >= 2) || Number(row.refunded_amount || 0) >= Number(row.total_amount || row.amount || 0)) {
    return { label: '高风险', type: 'danger', score: 3 }
  }
  if (row.status === 'received' || Number(row.refunded_amount || 0) > 0 || row.status === 'closed') {
    return { label: '需处理', type: 'warning', score: 2 }
  }
  if (row.status === 'pending' || isUnsettledOrder(row)) {
    return { label: '观察', type: 'info', score: 1 }
  }
  return { label: '正常', type: 'success', score: 0 }
}
const exceptionAdvice = (row) => {
  if (row.status === 'failed') return '引导顾客回订单页重新支付；核对支付回调和交易号，避免重复付款。'
  if (row.status === 'pending') return '订单未付款，不计入实收；顾客咨询时引导继续支付或重新下单。'
  if (row.status === 'received') return '顾客已付款，需提醒商家尽快接单；超时可联系商家或协助退款。'
  if (Number(row.refunded_amount || 0) > 0) return '核对退款原因、真实渠道到账和结算抵扣，必要时补充跟进记录。'
  if (row.status === 'closed') return '核对关闭原因，如已付款后关闭需确认退款或补偿方案。'
  if (isUnsettledOrder(row)) return '已形成顾客实收，等待进入商家结算单。'
  return '保持观察。'
}
const tagType = (tag) => ({
  支付失败: 'danger',
  待支付: 'warning',
  待接单: 'primary',
  已关闭: 'info',
  退款售后: 'warning',
  待结算: 'success'
}[tag] || 'info')
const orderAgeHours = (row) => {
  const ts = dateValue(row.created_at)
  return ts ? (Date.now() - ts) / 3600000 : 0
}
const merchantID = (row) => row?.merchant_id || row?.merchant?.id
const followUpType = (row) => {
  if (row.status === 'failed' || row.status === 'pending') return 'payment'
  if (Number(row.refunded_amount || 0) > 0 || ['partial', 'full'].includes(row.refund_status)) return 'refund'
  if (isUnsettledOrder(row)) return 'settlement'
  return 'risk'
}
const followUpPriority = (row) => exceptionLevel(row).score >= 3 ? 'high' : 'normal'
const createExceptionFollowUp = async (row) => {
  const id = merchantID(row)
  if (!id) return
  await createAdminMerchantFollowUp(id, {
    type: followUpType(row),
    priority: followUpPriority(row),
    source: 'order_exception',
    source_id: row.id,
    order_id: row.id,
    order_no: row.order_no || '',
    content: [
      `异常订单：${row.order_no || row.id}`,
      `异常原因：${exceptionTags(row).join('、') || '待核对'}`,
      `订单金额：${formatMoney(row.total_amount || row.amount)}，状态：${statusLabel(row.status)}`,
      `处理建议：${exceptionAdvice(row)}`
    ].join('\n')
  })
  ElMessage.success('已生成商家跟进记录')
}

const canRefund = (row) => {
  if (!row || row.order_type !== 'store_order') return false
  const refundable = Number(row.total_amount || row.amount || 0) - Number(row.refunded_amount || 0)
  return refundable > 0 && paidStoreStatuses.includes(row.status)
}

const canConfirmSubscription = (row) => row?.order_type === 'merchant_subscription' && row.status === 'pending'

const goSubscriptionRevenue = (row = {}) => {
  router.push({
    path: '/admin/subscriptions',
    query: {
      status: 'pending',
      keyword: row.order_no || ''
    }
  })
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

const exportOrdersXlsx = () => {
  const rows = filteredOrders.value.map((item) => ({
    订单号: item.order_no,
    类型: orderTypeLabel(item.order_type),
    商家或用户: item.merchant?.name || item.user?.display_name || item.user?.phone || '',
    金额: (Number(item.total_amount || item.amount || 0) / 100).toFixed(2),
    已退款: (Number(item.refunded_amount || 0) / 100).toFixed(2),
    退款状态: refundLabel(item.refund_status),
    状态: statusLabel(item.status),
    支付方式: item.payment_channel || '',
    支付时间: formatTime(item.paid_at),
    创建时间: formatTime(item.created_at)
  }))
  exportRowsToXlsx('platform-orders-finance.xlsx', '订单财务', rows)
}

const formatMoney = (value) => `¥${(Number(value || 0) / 100).toFixed(2)}`
const formatTime = (value) => value ? String(value).replace('T', ' ').slice(0, 19) : '-'
const dateValue = (value) => value ? new Date(value).getTime() || 0 : 0
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
const paymentChannelLabel = (channel) => ({
  platform_qr: '平台收款码',
  alipay: '支付宝',
  mock_wechat: '微信/模拟',
  merchant_qr: '商家收款码'
}[channel] || channel || '-')
const refundLabel = (status) => ({ none: '未退款', partial: '部分退款', full: '全额退款' }[status] || '未退款')
const refundType = (status) => ({ partial: 'warning', full: 'success' }[status] || 'info')

onMounted(() => {
  filters.keyword = route.query.keyword || ''
  filters.exception = route.query.exception || ''
  load()
})
</script>

<style scoped>
.finance-page {
  display: grid;
  gap: 20px;
}

.tips-grid,
.metric-grid,
.exception-strip {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 18px;
}

.metric-grid {
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.exception-strip {
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 12px;
}

.exception-strip button {
  display: grid;
  gap: 6px;
  min-height: 104px;
  padding: 14px;
  text-align: left;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #fff;
  cursor: pointer;
}

.exception-strip button.active {
  border-color: #2563eb;
  background: #eff6ff;
  box-shadow: 0 10px 26px rgba(37, 99, 235, 0.12);
}

.exception-strip span {
  color: #64748b;
  font-size: 13px;
  font-weight: 900;
}

.exception-strip strong {
  color: #0f172a;
  font-size: 24px;
}

.exception-strip small {
  color: #64748b;
  line-height: 1.45;
}

.exception-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.table-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.filters-row {
  display: flex;
  gap: 12px;
  align-items: center;
  flex-wrap: wrap;
}

.filters-row :deep(.el-input),
.filters-row :deep(.el-select) {
  width: 220px;
}

.refund-alert {
  margin-bottom: 16px;
}

@media (max-width: 1180px) {
  .tips-grid,
  .metric-grid,
  .exception-strip {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 720px) {
  .tips-grid,
  .metric-grid,
  .exception-strip {
    grid-template-columns: 1fr;
  }
}
</style>
