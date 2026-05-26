<template>
  <div class="merchant-home">
    <!-- 顶部区域 -->
    <div class="header">
      <div class="header-content">
        <div class="user-info">
          <van-image
            round
            width="48"
            height="48"
            :src="merchantInfo.avatar || defaultAvatar"
          />
          <div class="user-detail">
            <div class="user-name">{{ merchantInfo.name || '商家' }}</div>
            <div class="subscription-status">
              <van-tag :type="subscriptionStatus.type">
                {{ subscriptionStatus.text }}
              </van-tag>
            </div>
          </div>
        </div>
        <van-icon name="setting-o" size="24" @click="$router.push('/m/merchant/settings')" />
      </div>
    </div>

    <!-- 加载状态 -->
    <div v-if="loading" class="loading-container">
      <van-loading type="spinner" size="40px">加载中...</van-loading>
    </div>

    <template v-else>
      <!-- 统计卡片 -->
      <div class="stats-section">
        <van-grid :column="4" :border="false">
          <van-grid-item>
            <div class="stat-item">
              <div class="stat-value">{{ stats.todayOrders }}</div>
              <div class="stat-label">今日订单</div>
            </div>
          </van-grid-item>
          <van-grid-item>
            <div class="stat-item">
              <div class="stat-value text-primary">¥{{ stats.todayRevenue }}</div>
              <div class="stat-label">今日营收</div>
            </div>
          </van-grid-item>
          <van-grid-item>
            <div class="stat-item">
              <div class="stat-value text-warning">{{ stats.pendingOrders }}</div>
              <div class="stat-label">待处理</div>
            </div>
          </van-grid-item>
          <van-grid-item>
            <div class="stat-item">
              <div class="stat-value text-danger">{{ stats.refundRequests }}</div>
              <div class="stat-label">退款申请</div>
            </div>
          </van-grid-item>
        </van-grid>
      </div>

      <!-- 快捷入口 -->
      <div class="quick-section">
        <div class="section-title">快捷操作</div>
        <van-grid :column="4" :gutter="10">
          <van-grid-item icon="orders-o" text="订单" @click="$router.push('/m/merchant/orders')" />
          <van-grid-item icon="shop-o" text="商品" @click="$router.push('/m/merchant/products')" />
          <van-grid-item icon="coupon-o" text="优惠券" @click="$router.push('/m/merchant/coupons')" />
          <van-grid-item icon="chat-o" text="AI助手" @click="$router.push('/m/merchant/ai')" />
        </van-grid>
      </div>

      <!-- 待处理订单 -->
      <div class="pending-section">
        <div class="section-header">
          <span class="section-title">待处理订单</span>
          <van-tag v-if="pendingOrders.length" type="danger" size="medium">
            {{ pendingOrders.length }}单
          </van-tag>
        </div>

        <van-list finished>
          <div v-if="pendingOrders.length === 0" class="empty-pending">
            <van-empty description="暂无待处理订单" />
          </div>
          <van-card
            v-for="order in pendingOrders"
            :key="order.id"
            :title="`订单号: ${order.order_no}`"
            :desc="`${order.items?.length || 0} 件商品`"
            class="order-card"
            @click="viewOrder(order.id)"
          >
            <template #tags>
              <van-tag :type="getOrderType(order.status)">
                {{ getStatusText(order.status) }}
              </van-tag>
            </template>
            <template #num>
              <div class="order-price">¥{{ formatPrice(order.amount) }}</div>
            </template>
            <template #footer>
              <van-button
                v-if="order.status === 'pending'"
                size="mini"
                type="primary"
                @click.stop="acceptOrder(order.id)"
              >
                接单
              </van-button>
              <van-button
                v-if="order.status === 'pending'"
                size="mini"
                type="danger"
                plain
                @click.stop="rejectOrder(order.id)"
              >
                拒单
              </van-button>
            </template>
          </van-card>
        </van-list>

        <van-button
          v-if="pendingOrders.length > 0"
          block
          plain
          class="view-more"
          @click="$router.push('/m/merchant/orders')"
        >
          查看全部订单
        </van-button>
      </div>

      <!-- 经营概览 -->
      <div class="overview-section">
        <div class="section-title">经营概览</div>
        <van-grid :column="2" :gutter="10">
          <van-grid-item>
            <div class="overview-card">
              <div class="overview-value">{{ stats.monthOrders }}</div>
              <div class="overview-label">本月订单</div>
            </div>
          </van-grid-item>
          <van-grid-item>
            <div class="overview-card">
              <div class="overview-value text-primary">¥{{ stats.monthRevenue }}</div>
              <div class="overview-label">本月营收</div>
            </div>
          </van-grid-item>
          <van-grid-item>
            <div class="overview-card">
              <div class="overview-value">{{ stats.totalProducts }}</div>
              <div class="overview-label">在售商品</div>
            </div>
          </van-grid-item>
          <van-grid-item>
            <div class="overview-card">
              <div class="overview-value">{{ stats.totalStores }}</div>
              <div class="overview-label">门店数量</div>
            </div>
          </van-grid-item>
        </van-grid>
      </div>
    </template>

    <!-- 退款弹窗 -->
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
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { showToast, showSuccessToast, showFailToast } from 'vant'
import {
  fetchMerchantInfo,
  fetchMerchantSubscription,
  fetchMerchantOrders,
  acceptMerchantOrder,
  closeMerchantOrder,
  fetchMerchantStores
} from '@/api/modules'

const router = useRouter()

const loading = ref(true)
const merchantInfo = ref({})
const subscription = ref(null)
const orders = ref([])
const stores = ref([])
const stats = ref({
  todayOrders: 0,
  todayRevenue: '0.00',
  pendingOrders: 0,
  refundRequests: 0,
  monthOrders: 0,
  monthRevenue: '0.00',
  totalProducts: 0,
  totalStores: 0
})
const showRejectDialog = ref(false)
const currentRejectOrderId = ref(null)
const rejectReason = ref('')

const defaultAvatar = 'https://via.placeholder.com/48x48?text=商家'

const subscriptionStatus = computed(() => {
  if (!subscription.value) return { type: 'default', text: '未订阅' }
  const endDate = new Date(subscription.value.end_date)
  if (endDate < new Date()) {
    return { type: 'danger', text: '已过期' }
  }
  return { type: 'success', text: '使用中' }
})

const pendingOrders = computed(() => {
  return orders.value.filter(o => ['pending', 'paid'].includes(o.status)).slice(0, 5)
})

function formatPrice(price) {
  return parseFloat(price || 0).toFixed(2)
}

function getOrderType(status) {
  const typeMap = {
    pending: 'warning',
    paid: 'primary',
    preparing: 'primary',
    ready: 'success',
    completed: 'success',
    cancelled: 'default',
    refunded: 'danger'
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
    refunded: '已退款'
  }
  return textMap[status] || status
}

function viewOrder(id) {
  router.push(`/m/merchant/orders/${id}`)
}

async function acceptOrder(id) {
  try {
    await acceptMerchantOrder(id)
    showSuccessToast('已接单')
    loadOrders()
  } catch (e) {
    showFailToast(e.response?.data?.message || '操作失败')
  }
}

function rejectOrder(id) {
  currentRejectOrderId.value = id
  rejectReason.value = ''
  showRejectDialog.value = true
}

async function confirmReject() {
  if (!rejectReason.value.trim()) {
    showToast('请输入拒单原因')
    return
  }
  try {
    await closeMerchantOrder(currentRejectOrderId.value)
    showSuccessToast('已拒单')
    loadOrders()
  } catch (e) {
    showFailToast(e.response?.data?.message || '操作失败')
  }
}

async function loadMerchantInfo() {
  try {
    const [infoRes, subRes, storesRes] = await Promise.all([
      fetchMerchantInfo().catch(() => ({ data: {} })),
      fetchMerchantSubscription().catch(() => ({ data: null })),
      fetchMerchantStores().catch(() => ({ data: [] }))
    ])
    merchantInfo.value = infoRes.data || {}
    subscription.value = subRes.data
    stores.value = storesRes.data || []
    stats.value.totalStores = stores.value.length
  } catch (e) {
    console.error('加载商家信息失败:', e)
  }
}

async function loadOrders() {
  try {
    const res = await fetchMerchantOrders({ status: '', page: 1, page_size: 20 })
    orders.value = res.data?.items || res.data || []

    // 计算统计数据
    const today = new Date().toDateString()
    const monthStart = new Date(new Date().getFullYear(), new Date().getMonth(), 1)

    let todayOrders = 0
    let todayRevenue = 0
    let pendingOrders = 0
    let refundRequests = 0
    let monthOrders = 0
    let monthRevenue = 0

    orders.value.forEach(order => {
      const orderDate = new Date(order.created_at)
      if (orderDate.toDateString() === today) {
        todayOrders++
        if (order.status !== 'cancelled' && order.status !== 'refunded') {
          todayRevenue += parseFloat(order.amount || 0)
        }
      }
      if (orderDate >= monthStart) {
        monthOrders++
        if (order.status !== 'cancelled' && order.status !== 'refunded') {
          monthRevenue += parseFloat(order.amount || 0)
        }
      }
      if (['pending', 'paid'].includes(order.status)) {
        pendingOrders++
      }
      if (order.status === 'refund_requested') {
        refundRequests++
      }
    })

    stats.value.todayOrders = todayOrders
    stats.value.todayRevenue = todayRevenue.toFixed(2)
    stats.value.pendingOrders = pendingOrders
    stats.value.refundRequests = refundRequests
    stats.value.monthOrders = monthOrders
    stats.value.monthRevenue = monthRevenue.toFixed(2)
  } catch (e) {
    console.error('加载订单失败:', e)
  }
}

onMounted(async () => {
  loading.value = true
  await Promise.all([loadMerchantInfo(), loadOrders()])
  loading.value = false
})
</script>

<style scoped>
.merchant-home {
  min-height: 100vh;
  background: #f7f8fa;
  padding-bottom: 60px;
}

.header {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  padding: 20px 16px;
  color: white;
}

.header-content {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.user-info {
  display: flex;
  align-items: center;
  gap: 12px;
}

.user-name {
  font-size: 18px;
  font-weight: bold;
  margin-bottom: 4px;
}

.loading-container {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 50vh;
}

.stats-section {
  background: white;
  margin: -20px 16px 12px;
  border-radius: 12px;
  padding: 16px 0;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.1);
}

.stat-item {
  text-align: center;
}

.stat-value {
  font-size: 20px;
  font-weight: bold;
  margin-bottom: 4px;
}

.text-primary {
  color: #1989fa;
}

.text-warning {
  color: #ff976a;
}

.text-danger {
  color: #ee0a24;
}

.stat-label {
  font-size: 12px;
  color: #999;
}

.quick-section {
  background: white;
  margin: 12px 16px;
  border-radius: 12px;
  padding: 16px;
}

.section-title {
  font-size: 16px;
  font-weight: bold;
  margin-bottom: 12px;
}

.pending-section {
  background: white;
  margin: 12px 16px;
  border-radius: 12px;
  padding: 16px;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.empty-pending {
  padding: 20px 0;
}

.order-card {
  margin-bottom: 12px;
  border-radius: 8px;
}

.order-price {
  font-size: 16px;
  font-weight: bold;
  color: #ee0a24;
}

.view-more {
  margin-top: 12px;
}

.overview-section {
  background: white;
  margin: 12px 16px;
  border-radius: 12px;
  padding: 16px;
}

.overview-card {
  background: #f7f8fa;
  border-radius: 8px;
  padding: 16px;
  text-align: center;
}

.overview-value {
  font-size: 20px;
  font-weight: bold;
  margin-bottom: 4px;
}

.overview-label {
  font-size: 12px;
  color: #999;
}
</style>
