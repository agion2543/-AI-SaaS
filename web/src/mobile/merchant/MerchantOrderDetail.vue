<template>
  <div class="merchant-order-detail">
    <!-- 顶部导航 -->
    <van-nav-bar
      title="订单详情"
      left-arrow
      fixed
      placeholder
      @click-left="router.back()"
    />

    <!-- 加载状态 -->
    <div v-if="loading" class="loading-container">
      <van-loading type="spinner" size="40px">加载中...</van-loading>
    </div>

    <template v-else-if="order">
      <!-- 订单状态 -->
      <div class="order-status" :class="statusClass">
        <div class="status-icon">
          <van-icon :name="statusIcon" size="40" />
        </div>
        <div class="status-text">{{ statusText }}</div>
      </div>

      <!-- 订单信息 -->
      <van-cell-group inset title="订单信息">
        <van-cell title="订单编号" :value="order.order_no" />
        <van-cell title="下单时间" :value="formatTime(order.created_at)" />
        <van-cell title="门店" :value="order.store_name || '-'" />
        <van-cell v-if="order.pay_type" title="支付方式" :value="getPayTypeText(order.pay_type)" />
        <van-cell v-if="order.paid_at" title="支付时间" :value="formatTime(order.paid_at)" />
      </van-cell-group>

      <!-- 商品明细 -->
      <van-cell-group inset title="商品明细">
        <van-cell
          v-for="item in order.items"
          :key="item.id"
        >
          <template #title>
            <div class="product-info">
              <span class="product-name">{{ item.product_name || item.name }}</span>
              <span class="product-price">¥{{ formatPrice(item.price) }}</span>
            </div>
          </template>
          <template #label>
            <span class="product-quantity">x{{ item.quantity }}</span>
            <span class="product-subtotal">小计: ¥{{ formatPrice(item.price * item.quantity) }}</span>
          </template>
        </van-cell>

        <van-cell title="订单总价">
          <template #value>
            <span class="total-price">¥{{ formatPrice(order.total_amount || order.amount) }}</span>
          </template>
        </van-cell>
        <van-cell v-if="order.discount_amount > 0" title="优惠">
          <template #value>
            <span class="discount">-¥{{ formatPrice(order.discount_amount) }}</span>
          </template>
        </van-cell>
        <van-cell title="实付金额">
          <template #value>
            <span class="final-price">¥{{ formatPrice(order.amount) }}</span>
          </template>
        </van-cell>
      </van-cell-group>

      <!-- 备注信息 -->
      <van-cell-group v-if="order.remark" inset title="备注">
        <van-cell>
          <van-icon name="comment-o" />
          {{ order.remark }}
        </van-cell>
      </van-cell-group>

      <!-- 用户信息 -->
      <van-cell-group v-if="order.customer" inset title="用户信息">
        <van-cell title="用户ID" :value="order.customer.id || '-'" />
        <van-cell v-if="order.customer.nickname" title="昵称" :value="order.customer.nickname" />
      </van-cell-group>

      <!-- 操作按钮 -->
      <div class="action-buttons">
        <van-button
          v-if="order.status === 'pending'"
          type="primary"
          block
          @click="acceptOrder"
        >
          接单
        </van-button>
        <van-button
          v-if="order.status === 'pending'"
          type="danger"
          plain
          block
          @click="showRejectDialog = true"
        >
          拒单
        </van-button>
        <van-button
          v-if="order.status === 'paid'"
          type="success"
          block
          @click="completeOrder"
        >
          完成订单
        </van-button>
        <van-button
          v-if="order.status === 'refund_requested'"
          type="warning"
          block
          @click="showRefundDialog = true"
        >
          处理退款
        </van-button>
      </div>
    </template>

    <van-empty v-else description="订单不存在" />

    <!-- 拒单弹窗 -->
    <van-dialog
      v-model:show="showRejectDialog"
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
      v-model:show="showRefundDialog"
      title="处理退款"
      show-cancel-button
      @confirm="confirmRefund"
    >
      <div style="padding: 16px;">
        <p>订单金额: ¥{{ formatPrice(order?.amount || 0) }}</p>
        <van-radio-group v-model="refundAction">
          <van-cell-group>
            <van-cell title="同意退款" clickable @click="refundAction = 'approve'">
              <template #right-icon>
                <van-radio name="approve" />
              </template>
            </van-cell>
            <van-cell title="拒绝退款" clickable @click="refundAction = 'reject'">
              <template #right-icon>
                <van-radio name="reject" />
              </template>
            </van-cell>
          </van-cell-group>
        </van-radio-group>
        <van-field
          v-if="refundAction === 'reject'"
          v-model="refundReason"
          type="textarea"
          placeholder="请输入拒绝原因"
          rows="2"
          style="margin-top: 12px;"
        />
      </div>
    </van-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { showToast, showSuccessToast, showFailToast } from 'vant'
import {
  fetchMerchantOrderDetail,
  acceptMerchantOrder,
  completeMerchantOrder,
  closeMerchantOrder,
  refundMerchantOrder
} from '@/api/modules'

const route = useRoute()
const router = useRouter()
const orderId = route.params.id

const loading = ref(true)
const order = ref(null)
const showRejectDialog = ref(false)
const rejectReason = ref('')
const showRefundDialog = ref(false)
const refundAction = ref('approve')
const refundReason = ref('')

const statusClass = computed(() => {
  const map = {
    pending: 'status-pending',
    paid: 'status-paid',
    preparing: 'status-preparing',
    ready: 'status-ready',
    completed: 'status-completed',
    cancelled: 'status-cancelled',
    refunded: 'status-refunded'
  }
  return map[order.value?.status] || ''
})

const statusIcon = computed(() => {
  const map = {
    pending: 'clock-o',
    paid: 'paid',
    preparing: 'orders-o',
    ready: 'bag-o',
    completed: 'success',
    cancelled: 'cross',
    refunded: 'refund-o'
  }
  return map[order.value?.status] || 'info-o'
})

const statusText = computed(() => {
  const map = {
    pending: '待接单',
    paid: '已支付，等待商家接单',
    preparing: '制作中',
    ready: '待取餐',
    completed: '已完成',
    cancelled: '已取消',
    refunded: '已退款'
  }
  return map[order.value?.status] || '未知状态'
})

function formatPrice(price) {
  return parseFloat(price || 0).toFixed(2)
}

function formatTime(time) {
  if (!time) return '-'
  return new Date(time).toLocaleString('zh-CN')
}

function getPayTypeText(payType) {
  const map = {
    wechat: '微信支付',
    alipay: '支付宝',
    mock: '模拟支付'
  }
  return map[payType] || payType || '-'
}

async function loadOrder() {
  loading.value = true
  try {
    const res = await fetchMerchantOrderDetail(orderId)
    order.value = res.data
  } catch (e) {
    console.error('加载订单失败:', e)
    showToast('加载失败')
  } finally {
    loading.value = false
  }
}

async function acceptOrder() {
  try {
    await acceptMerchantOrder(orderId)
    showSuccessToast('已接单')
    loadOrder()
  } catch (e) {
    showFailToast(e.response?.data?.message || '操作失败')
  }
}

function confirmReject() {
  if (!rejectReason.value.trim()) {
    showToast('请输入拒单原因')
    return
  }
  doReject()
}

async function doReject() {
  try {
    await closeMerchantOrder(orderId)
    showSuccessToast('已拒单')
    router.back()
  } catch (e) {
    showFailToast(e.response?.data?.message || '操作失败')
  }
}

async function completeOrder() {
  try {
    await completeMerchantOrder(orderId)
    showSuccessToast('订单已完成')
    loadOrder()
  } catch (e) {
    showFailToast(e.response?.data?.message || '操作失败')
  }
}

function confirmRefund() {
  doRefund()
}

async function doRefund() {
  try {
    if (refundAction.value === 'approve') {
      await refundMerchantOrder(orderId, { reason: '同意退款' })
      showSuccessToast('已同意退款')
    } else {
      if (!refundReason.value.trim()) {
        showToast('请输入拒绝原因')
        return
      }
      await closeMerchantOrder(orderId)
      showSuccessToast('已拒绝退款')
    }
    router.back()
  } catch (e) {
    showFailToast(e.response?.data?.message || '操作失败')
  }
}

onMounted(() => {
  loadOrder()
})
</script>

<style scoped>
.merchant-order-detail {
  min-height: 100vh;
  background: #f7f8fa;
  padding-bottom: 80px;
}

.loading-container {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 50vh;
}

.order-status {
  padding: 30px 20px;
  text-align: center;
  color: white;
}

.status-pending {
  background: linear-gradient(135deg, #ff976a 0%, #ed6a0c 100%);
}

.status-paid {
  background: linear-gradient(135deg, #07c160 0%, #06ad56 100%);
}

.status-preparing {
  background: linear-gradient(135deg, #1989fa 0%, #096dd9 100%);
}

.status-ready {
  background: linear-gradient(135deg, #07c160 0%, #06ad56 100%);
}

.status-completed {
  background: linear-gradient(135deg, #1989fa 0%, #096dd9 100%);
}

.status-cancelled,
.status-refunded {
  background: linear-gradient(135deg, #999 0%, #666 100%);
}

.status-text {
  font-size: 16px;
  margin-top: 8px;
}

.product-info {
  display: flex;
  justify-content: space-between;
  width: 100%;
}

.product-name {
  flex: 1;
}

.product-price {
  margin-left: 12px;
}

.product-quantity {
  margin-right: 12px;
  color: #666;
}

.product-subtotal {
  color: #ee0a24;
}

.total-price {
  text-decoration: line-through;
  color: #999;
}

.discount {
  color: #07c160;
}

.final-price {
  color: #ee0a24;
  font-weight: bold;
  font-size: 16px;
}

.action-buttons {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
</style>
