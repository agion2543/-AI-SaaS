<template>
  <div class="merchant-orders">
    <!-- 顶部导航 -->
    <van-nav-bar
      title="订单管理"
      left-arrow
      fixed
      placeholder
      @click-left="router.back()"
    />

    <div v-if="!canWrite" class="readonly-banner" role="status">
      订阅已到期：订单仅可只读查看，处理订单和退款的操作已关闭。
      <van-button size="small" type="warning" plain @click="router.push('/m/merchant/subscription')">查看续费</van-button>
    </div>

    <!-- 订单筛选 -->
    <van-tabs v-model:active="activeTab" sticky offset-top="46">
      <van-tab title="全部" name="all" />
      <van-tab title="待付款" name="pending_payment" />
      <van-tab title="待处理" name="pending" />
      <van-tab title="进行中" name="paid" />
      <van-tab title="已完成" name="completed" />
      <van-tab title="已退款" name="refunded" />
    </van-tabs>

    <!-- 订单列表 -->
    <van-list
      v-model:loading="loading"
      :finished="finished"
      finished-text="没有更多了"
      @load="loadOrders"
    >
      <div v-if="visibleOrders.length === 0 && !loading" class="empty-orders">
        <van-empty description="暂无订单" />
      </div>

      <div v-for="order in visibleOrders" :key="order.id" class="order-item">
        <van-cell-group inset>
          <van-cell>
            <template #title>
              <div class="order-header">
                <span class="order-no">订单号: {{ order.order_no }}</span>
                <van-tag :type="getOrderType(order.status)">
                  {{ getStatusText(order.status) }}
                </van-tag>
              </div>
            </template>
            <template #value>
              <span class="order-time">{{ formatTime(order.created_at) }}</span>
            </template>
          </van-cell>

          <van-cell>
            <template #label>
              <div class="order-products">
                <div v-for="item in order.items" :key="item.id" class="product-item">
                  <span>{{ item.product_name || item.name }}</span>
                  <span>x{{ item.quantity }}</span>
                  <span>¥{{ formatPrice(item.price) }}</span>
                </div>
              </div>
            </template>
            <template #value>
              <div class="order-price">
                <span>应付</span>
                <span class="price">¥{{ formatPrice(order.total_amount ?? order.amount ?? 0) }}</span>
              </div>
            </template>
          </van-cell>

          <van-cell v-if="order.remark">
            <template #label>
              <div class="order-remark">
                <van-icon name="comment-o" />
                备注: {{ order.remark }}
              </div>
            </template>
          </van-cell>

          <van-cell class="order-actions">
            <template #value>
              <div class="actions">
                <van-button
                  v-if="canWrite && order.status === 'payment_confirming'"
                  size="small"
                  type="warning"
                  @click="confirmPayment(order.id)"
                >
                  确认收款
                </van-button>
                <van-button
                  v-if="canWrite && ['submitted', 'received'].includes(order.status) && (order.status === 'submitted' || isConfirmedPaid(order) || Number(order.total_amount ?? order.amount ?? 0) === 0)"
                  size="small"
                  type="primary"
                  @click="acceptOrder(order.id)"
                >
                  接单
                </van-button>
                <van-button
                  v-if="canWrite && ['submitted', 'received', 'payment_confirming'].includes(order.status)"
                  size="small"
                  type="danger"
                  plain
                  @click="showRejectDialog(order.id)"
                >
                  拒单
                </van-button>
                <van-button
                  v-if="canWrite && ['accepted', 'preparing'].includes(order.status)"
                  size="small"
                  type="success"
                  @click="completeOrder(order.id)"
                >
                  完成
                </van-button>
                <van-button
                  v-if="canWrite && canRefund(order)"
                  size="small"
                  type="warning"
                  @click="handleRefund(order)"
                >
                  处理退款
                </van-button>
                <van-button
                  size="small"
                  plain
                  @click="viewOrderDetail(order.id)"
                >
                  查看详情
                </van-button>
              </div>
            </template>
          </van-cell>
        </van-cell-group>
      </div>
    </van-list>

    <!-- 拒单弹窗 -->
    <van-dialog
      v-model:show="rejectDialogVisible"
      title="拒单原因"
      show-cancel-button
      @confirm="confirmReject"
    >
      <van-field
        v-model="rejectReason"
        type="textarea"
        placeholder="请输入拒单原因"
        rows="3"
      />
    </van-dialog>

    <!-- 退款弹窗 -->
    <van-dialog
      v-model:show="refundDialogVisible"
      title="处理退款"
      show-cancel-button
      @confirm="confirmRefund"
    >
      <div class="refund-content">
        <p>订单金额: ¥{{ formatPrice(currentOrder?.total_amount ?? currentOrder?.amount ?? 0) }}</p>
        <van-radio-group v-model="refundAction">
          <van-radio name="approve">同意退款</van-radio>
          <van-radio name="reject">拒绝退款</van-radio>
        </van-radio-group>
        <van-field
          v-if="refundAction === 'reject'"
          v-model="refundReason"
          type="textarea"
          placeholder="请输入拒绝原因"
          rows="2"
        />
      </div>
    </van-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { showToast, showSuccessToast, showFailToast } from 'vant'
import {
  fetchMerchantOrders,
  acceptMerchantOrder,
  confirmMerchantOrderPayment,
  completeMerchantOrder,
  closeMerchantOrder,
  refundMerchantOrder
} from '@/api/modules'
import { clearRefundIdempotency, getOrCreateRefundIdempotency } from '@/utils/refundIdempotency'
import { cachedMobileMerchantSubscriptionState, mobileSubscriptionActions } from '@/utils/mobileMerchantSubscription'

const router = useRouter()
const canWrite = computed(() => mobileSubscriptionActions(cachedMobileMerchantSubscriptionState()).canWriteBusiness)

const loading = ref(false)
const finished = ref(false)
const orders = ref([])
const activeTab = ref('all')
const page = ref(1)
const pageSize = 10

const rejectDialogVisible = ref(false)
const currentRejectOrderId = ref(null)
const rejectReason = ref('')

const refundDialogVisible = ref(false)
const currentOrder = ref(null)
const refundAction = ref('approve')
const refundReason = ref('')
const refundIdempotencyKey = ref('')
const refundIdempotencyPayload = ref('')

const visibleOrders = computed(() => orders.value.filter((order) => {
  if (activeTab.value === 'pending_payment') return ['pending', 'failed'].includes(order.status)
  if (activeTab.value === 'pending') return ['submitted', 'received', 'payment_confirming'].includes(order.status)
  if (activeTab.value === 'paid') return ['accepted', 'preparing'].includes(order.status)
  if (activeTab.value === 'completed') return ['completed', 'closed'].includes(order.status)
  if (activeTab.value === 'refunded') return Number(order.refunded_amount || 0) > 0
  return true
}))
const isConfirmedPaid = (order) => Boolean(order?.paid_at) && Number(order.total_amount ?? order.amount ?? 0) > 0
const canRefund = (order) => order.refund_status !== 'processing' && isConfirmedPaid(order) && order.payment_channel === 'alipay' && Boolean(order.transaction_no) && Number(order.refunded_amount || 0) < Number(order.total_amount ?? order.amount ?? 0)

function formatPrice(price) {
  return parseFloat(price || 0).toFixed(2)
}

function formatTime(time) {
  if (!time) return '-'
  return new Date(time).toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  })
}

function getOrderType(status) {
  const typeMap = {
    pending: 'warning',
    failed: 'danger',
    payment_confirming: 'warning',
    submitted: 'primary',
    received: 'primary',
    accepted: 'primary',
    preparing: 'primary',
    completed: 'success',
    closed: 'default'
  }
  return typeMap[status] || 'default'
}

function getStatusText(status) {
  const textMap = {
    pending: '待付款',
    failed: '支付失败',
    payment_confirming: '待确认收款',
    submitted: '已提交/后结算',
    received: '已收款待接单',
    accepted: '已接单',
    preparing: '制作中',
    completed: '已完成',
    closed: '已关闭'
  }
  return textMap[status] || status
}

async function loadOrders() {
  loading.value = true
  try {
    const res = await fetchMerchantOrders({
      status: '',
      page: page.value,
      page_size: pageSize
    })

    const data = Array.isArray(res.data?.list) ? res.data.list : (Array.isArray(res.data?.items) ? res.data.items : (Array.isArray(res.data) ? res.data : []))

    if (page.value === 1) {
      orders.value = data
    } else {
      orders.value.push(...data)
    }

    // 如果数据少于一页，说明没有更多了
    if (data.length < pageSize) {
      finished.value = true
    } else {
      page.value++
    }
  } catch (e) {
    console.error('加载订单失败:', e)
    showToast('加载失败')
  } finally {
    loading.value = false
  }
}

function viewOrderDetail(id) {
  router.push(`/m/merchant/orders/${id}`)
}

async function acceptOrder(id) {
  try {
    await acceptMerchantOrder(id)
    showSuccessToast('已接单')
    refreshOrders()
  } catch (e) {
    showFailToast(e.response?.data?.message || '操作失败')
  }
}

async function confirmPayment(id) {
  try {
    await confirmMerchantOrderPayment(id)
    showSuccessToast('已确认收款')
    refreshOrders()
  } catch (e) {
    showFailToast(e.response?.data?.message || '操作失败')
  }
}

function showRejectDialog(id) {
  currentRejectOrderId.value = id
  rejectReason.value = ''
  rejectDialogVisible.value = true
}

async function confirmReject() {
  if (!rejectReason.value.trim()) {
    showToast('请输入拒单原因')
    return
  }
  try {
    await closeMerchantOrder(currentRejectOrderId.value)
    showSuccessToast('已拒单')
    refreshOrders()
  } catch (e) {
    showFailToast(e.response?.data?.message || '操作失败')
  }
}

async function completeOrder(id) {
  try {
    await completeMerchantOrder(id)
    showSuccessToast('订单已完成')
    refreshOrders()
  } catch (e) {
    showFailToast(e.response?.data?.message || '操作失败')
  }
}

function handleRefund(order) {
  currentOrder.value = order
  refundAction.value = 'approve'
  refundReason.value = ''
  refundIdempotencyKey.value = ''
  refundIdempotencyPayload.value = ''
  refundDialogVisible.value = true
}

async function confirmRefund() {
  try {
    if (refundAction.value === 'approve') {
      const orderID = currentOrder.value.id
      const requestPayload = { full: true, reason: '同意退款' }
      const idempotency = getOrCreateRefundIdempotency(orderID, requestPayload, refundIdempotencyKey.value, refundIdempotencyPayload.value)
      if (idempotency.conflict) {
        showFailToast('上一笔退款请求仍待核查或请求内容已改变，请先完成渠道对账。')
        return
      }
      refundIdempotencyKey.value = idempotency.key
      refundIdempotencyPayload.value = idempotency.fingerprint
      await refundMerchantOrder(orderID, { ...requestPayload, idempotency_key: idempotency.key })
      showSuccessToast('退款渠道已确认处理')
      clearRefundIdempotency(orderID)
      refundIdempotencyKey.value = ''
      refundIdempotencyPayload.value = ''
    } else {
      await closeMerchantOrder(currentOrder.value.id)
      showSuccessToast('已拒绝退款')
    }
    refreshOrders()
  } catch (e) {
    const message = String(e.response?.data?.message || e.message || '')
    if (message.toLowerCase().includes('definitively rejected')) {
      clearRefundIdempotency(currentOrder.value?.id)
      refundIdempotencyKey.value = ''
      refundIdempotencyPayload.value = ''
    }
    showFailToast(message || '操作失败')
  }
}

function refreshOrders() {
  page.value = 1
  finished.value = false
  orders.value = []
  loadOrders()
}

// 监听 Tab 切换
import { watch } from 'vue'
watch(activeTab, () => {
  refreshOrders()
})

onMounted(() => {
  loadOrders()
})
</script>

<style scoped>
.merchant-orders {
  min-height: 100vh;
  background: #f7f8fa;
  padding-bottom: 20px;
}
.readonly-banner { display: flex; align-items: center; justify-content: space-between; gap: 10px; margin: 10px 12px; padding: 10px 12px; border: 1px solid #f0d49b; border-radius: 12px; color: #80591b; background: #fff8e8; font-size: 12px; line-height: 1.5; }

.empty-orders {
  padding: 60px 0;
}

.order-item {
  margin: 12px;
}

.order-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.order-no {
  font-size: 14px;
  font-weight: bold;
}

.order-time {
  font-size: 12px;
  color: #999;
}

.order-products {
  padding: 8px 0;
}

.product-item {
  display: flex;
  justify-content: space-between;
  padding: 4px 0;
  font-size: 14px;
}

.order-price {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
}

.order-price .price {
  font-size: 16px;
  font-weight: bold;
  color: #ee0a24;
}

.order-remark {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: #666;
}

.order-actions .van-cell__value {
  display: flex;
  justify-content: flex-end;
}

.actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.refund-content {
  padding: 16px;
}

.refund-content p {
  margin: 0 0 12px;
}
</style>
