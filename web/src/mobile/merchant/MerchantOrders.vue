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

    <!-- 订单筛选 -->
    <van-tabs v-model:active="activeTab" sticky offset-top="46">
      <van-tab title="全部" name="all" />
      <van-tab title="待接单" name="pending" />
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
      <div v-if="orders.length === 0 && !loading" class="empty-orders">
        <van-empty description="暂无订单" />
      </div>

      <div v-for="order in orders" :key="order.id" class="order-item">
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
                <span>实付</span>
                <span class="price">¥{{ formatPrice(order.amount) }}</span>
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
                  v-if="order.status === 'pending'"
                  size="small"
                  type="primary"
                  @click="acceptOrder(order.id)"
                >
                  接单
                </van-button>
                <van-button
                  v-if="order.status === 'pending'"
                  size="small"
                  type="danger"
                  plain
                  @click="showRejectDialog(order.id)"
                >
                  拒单
                </van-button>
                <van-button
                  v-if="order.status === 'paid'"
                  size="small"
                  type="success"
                  @click="completeOrder(order.id)"
                >
                  完成
                </van-button>
                <van-button
                  v-if="order.status === 'refund_requested'"
                  size="small"
                  type="warning"
                  @click="handleRefund(order.id)"
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
        <p>订单金额: ¥{{ formatPrice(currentOrder?.amount || 0) }}</p>
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
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { showToast, showSuccessToast, showFailToast } from 'vant'
import {
  fetchMerchantOrders,
  acceptMerchantOrder,
  completeMerchantOrder,
  closeMerchantOrder,
  refundMerchantOrder
} from '@/api/modules'

const router = useRouter()

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
    paid: 'primary',
    preparing: 'primary',
    ready: 'success',
    completed: 'success',
    cancelled: 'default',
    refunded: 'danger',
    refund_requested: 'warning'
  }
  return typeMap[status] || 'default'
}

function getStatusText(status) {
  const textMap = {
    pending: '待接单',
    paid: '已支付',
    preparing: '制作中',
    ready: '待取餐',
    completed: '已完成',
    cancelled: '已取消',
    refunded: '已退款',
    refund_requested: '退款申请'
  }
  return textMap[status] || status
}

async function loadOrders() {
  loading.value = true
  try {
    const statusMap = {
      all: '',
      pending: 'pending',
      paid: 'paid',
      completed: 'completed',
      refunded: 'refunded'
    }

    const res = await fetchMerchantOrders({
      status: statusMap[activeTab.value],
      page: page.value,
      page_size: pageSize
    })

    const data = res.data?.items || res.data || []

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
  refundDialogVisible.value = true
}

async function confirmRefund() {
  try {
    if (refundAction.value === 'approve') {
      await refundMerchantOrder(currentOrder.value.id, { reason: '同意退款' })
      showSuccessToast('已同意退款')
    } else {
      await closeMerchantOrder(currentOrder.value.id)
      showSuccessToast('已拒绝退款')
    }
    refreshOrders()
  } catch (e) {
    showFailToast(e.response?.data?.message || '操作失败')
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
