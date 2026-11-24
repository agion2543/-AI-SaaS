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
        <div class="header-actions">
          <van-button size="small" plain type="primary" @click="router.push('/m/merchant/subscription')">订阅状态 / 续费</van-button>
            <van-icon v-if="canWrite" name="setting-o" size="24" @click="$router.push('/m/merchant/settings')" />
        </div>
      </div>
    </div>

    <!-- 加载状态 -->
    <div v-if="loading" class="loading-container">
      <van-loading type="spinner" size="40px">加载中...</van-loading>
    </div>

    <template v-else>
      <div v-if="subscriptionState === 'expired'" class="readonly-banner" role="status">
        订阅已到期：当前页面仅展示可只读查看的数据，订单处理等写操作已关闭。
        <van-button size="small" type="warning" plain @click="router.push('/m/merchant/subscription')">查看续费</van-button>
      </div>

      <van-cell-group class="ai-entry">
        <van-cell
          title="商擎 AI 运营助手"
          label="经营洞察、营销文案与活动策划 · AI 服务尚未接入"
          icon="chat-o"
          is-link
          value="了解"
          @click="router.push('/m/merchant/ai')"
        />
      </van-cell-group>

      <section class="activity-intro" aria-label="活动与效果概览">
        <div class="activity-kicker">活动与效果报告</div>
        <div class="section-title">经营与活动概览</div>
        <p>先看系统内订单和收款摘要；外部团购跳转点击单独列示，不代表订单、付款或收入归因。</p>
      </section>

      <!-- 订单统计卡片 -->
      <section class="stats-section" aria-labelledby="order-stats-title">
        <div class="stats-heading">
          <div id="order-stats-title" class="section-title">订单与收款速览</div>
          <span class="stats-scope">快捷参考</span>
        </div>
        <p class="stats-note">按商家全部门店当前读取的最多 20 笔订单记录汇总（非全量）；订单数按创建时间，收款按付款确认时间统计且未扣退款；不含外部团购平台订单与核销数据。</p>
        <div v-if="orderSummaryStatus === 'loading'" class="order-summary-state" role="status">
          <van-loading size="18px">正在加载订单摘要…</van-loading>
        </div>
        <div v-else-if="orderSummaryStatus === 'error'" class="order-summary-state order-summary-error" role="alert">
          <span>订单摘要加载失败，暂时无法确认统计数据。</span>
          <van-button size="small" plain type="primary" @click="loadOrders">重试</van-button>
        </div>
        <van-grid v-else :column="2" :border="false">
          <van-grid-item>
            <div class="stat-item">
              <div class="stat-value">{{ stats.todayOrders }}</div>
              <div class="stat-label">今日订单</div>
            </div>
          </van-grid-item>
          <van-grid-item>
            <div class="stat-item">
              <div class="stat-value text-primary">{{ stats.todayRevenue }}</div>
              <div class="stat-label">今日确认收款</div>
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
              <div class="stat-label">有退款金额</div>
            </div>
          </van-grid-item>
        </van-grid>
      </section>

      <!-- 快捷入口 -->
      <div class="quick-section">
        <div class="section-title">日常管理</div>
        <van-grid :column="3" :gutter="10">
          <van-grid-item icon="orders-o" text="订单" @click="$router.push('/m/merchant/orders')" />
          <van-grid-item icon="shop-o" text="商品" @click="$router.push('/m/merchant/products')" />
          <van-grid-item v-if="canWrite" icon="coupon-o" text="优惠券" @click="$router.push('/m/merchant/coupons')" />
        </van-grid>
      </div>

      <div v-if="canWrite" class="quick-section">
        <div class="section-title">更多功能</div>
        <van-grid :column="3" :gutter="10">
          <van-grid-item v-if="canWrite" icon="setting-o" text="商家设置" @click="$router.push('/m/merchant/settings')" />
        </van-grid>
      </div>

      <!-- 待处理订单 -->
      <div class="pending-section">
        <div class="section-header">
          <span class="section-title">待处理订单</span>
          <van-tag v-if="orderSummaryStatus === 'ready' && pendingOrders.length" type="danger" size="medium">
            {{ stats.pendingOrders }} 单（当前读取记录）
          </van-tag>
        </div>

        <van-list v-if="orderSummaryStatus === 'ready'" finished>
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
              <div class="order-price">{{ formatCents(order.total_amount ?? order.amount ?? 0) }}</div>
            </template>
            <template #footer>
              <van-button
                v-if="canWrite && order.status === 'payment_confirming'"
                size="mini"
                type="warning"
                @click.stop="confirmPayment(order.id)"
              >
                确认收款
              </van-button>
              <van-button
                v-if="canWrite && ['submitted', 'received'].includes(order.status) && (order.status === 'submitted' || isConfirmedPaid(order) || Number(order.total_amount ?? order.amount ?? 0) === 0)"
                size="mini"
                type="primary"
                @click.stop="acceptOrder(order.id)"
              >
                接单
              </van-button>
              <van-button
                v-if="canWrite && ['submitted', 'received', 'payment_confirming'].includes(order.status)"
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
        <div v-else-if="orderSummaryStatus === 'loading'" class="pending-order-state" role="status">
          正在加载待处理订单…
        </div>
        <div v-else class="pending-order-state" role="alert">
          订单加载失败，无法确认是否有待处理订单。
        </div>

        <van-button
          v-if="orderSummaryStatus === 'ready' && pendingOrders.length > 0"
          block
          plain
          class="view-more"
          @click="$router.push('/m/merchant/orders')"
        >
          查看全部订单
        </van-button>
      </div>

      <!-- 本月订单与收款概览 -->
      <div class="overview-section">
        <div class="section-title">本月订单与收款</div>
        <div v-if="orderSummaryStatus === 'loading'" class="order-summary-state" role="status">
          正在加载本月订单与收款…
        </div>
        <div v-else-if="orderSummaryStatus === 'error'" class="order-summary-state order-summary-error" role="alert">
          订单摘要加载失败，请点击上方重试。
        </div>
        <van-grid v-else :column="2" :gutter="10">
          <van-grid-item>
            <div class="overview-card">
              <div class="overview-value">{{ stats.monthOrders }}</div>
              <div class="overview-label">本月订单</div>
            </div>
          </van-grid-item>
          <van-grid-item>
            <div class="overview-card">
              <div class="overview-value text-primary">{{ stats.monthRevenue }}</div>
              <div class="overview-label">本月确认收款</div>
            </div>
          </van-grid-item>
        </van-grid>
      </div>

      <!-- 次级外部渠道入口；仅配置官方链接和统计跳转点击，不读取平台交易 -->
      <section class="promotion-secondary" aria-labelledby="promotion-secondary-title">
        <div class="promotion-kicker"><van-icon name="qr" /> 外部平台跳转 · 次级入口</div>
        <h2 id="promotion-secondary-title">团购链接与点击</h2>
        <p class="promotion-description">按已读取的最多 100 家门店配置抖音生活服务、美团 / 大众点评官方团购链接统计外部跳转；门店超过 100 家时，首页不含其余门店。</p>
        <div class="promotion-summary" aria-live="polite">
          <div v-if="promotionSummary.status === 'loading'" class="promotion-summary-state">正在读取团购数据…</div>
          <div v-else-if="promotionSummary.status === 'error'" class="promotion-summary-state">团购数据暂不可读，可进入渠道页重试</div>
          <div v-else-if="stores.length === 0" class="promotion-summary-state">添加门店后即可配置团购入口</div>
          <div v-else class="promotion-metrics">
            <div class="promotion-metric">
              <span>已配置入口</span>
              <strong>{{ promotionSummary.configuredEntries }}<small> 个</small></strong>
            </div>
            <div class="promotion-metric">
              <span>已读取门店累计跳转点击</span>
              <strong>{{ promotionSummary.totalClicks.toLocaleString('zh-CN') }}<small> 次</small></strong>
            </div>
          </div>
        </div>
        <van-button class="promotion-button" type="primary" plain round icon="arrow" @click="router.push('/m/merchant/group-buy')">
          管理外部团购跳转
        </van-button>
        <p class="promotion-disclaimer">本平台不读取团购订单、付款或核销数据；点击量不代表独立顾客数、订单、付款或收入。</p>
      </section>
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
  confirmMerchantOrderPayment,
  closeMerchantOrder,
  fetchMerchantStores,
  fetchMerchantGroupBuyLinks
} from '@/api/modules'
import { mobileMerchantSubscriptionState, mobileSubscriptionActions } from '@/utils/mobileMerchantSubscription'
import { formatCents } from '@/utils/productPricing'

const router = useRouter()

const loading = ref(true)
const merchantInfo = ref({})
const subscription = ref(null)
const orders = ref([])
const orderSummaryStatus = ref('loading')
const stores = ref([])
const storeListLoadFailed = ref(false)
const promotionSummary = ref({ status: 'loading', configuredEntries: 0, totalClicks: 0 })
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
  const state = mobileMerchantSubscriptionState(subscription.value)
  if (state === 'active') return { type: 'success', text: '服务中' }
  if (state === 'expired') return { type: 'warning', text: '已过期 · 只读' }
  if (state === 'paused') return { type: 'danger', text: '业务已暂停' }
  return { type: 'default', text: '状态待确认' }
})
const subscriptionState = computed(() => mobileMerchantSubscriptionState(subscription.value))
const canWrite = computed(() => mobileSubscriptionActions(subscriptionState.value).canWriteBusiness)

const pendingOrders = computed(() => {
  return orders.value.filter(o => ['pending', 'failed', 'payment_confirming', 'submitted', 'received'].includes(o.status)).slice(0, 5)
})
const isConfirmedPaid = (order) => Boolean(order?.paid_at) && Number(order.total_amount ?? order.amount ?? 0) > 0

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

async function confirmPayment(id) {
  try {
    await confirmMerchantOrderPayment(id)
    showSuccessToast('已确认收款')
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
  let subRes
  try {
    subRes = await fetchMerchantSubscription()
  } catch {
    await router.replace('/m/merchant/subscription')
    return false
  }
  subscription.value = subRes?.data || null
  const state = mobileMerchantSubscriptionState(subscription.value)
  if (!mobileSubscriptionActions(state).canReadBusiness) {
    await router.replace('/m/merchant/subscription')
    return false
  }
  try {
    const [infoRes, storesRes] = await Promise.all([
      fetchMerchantInfo().catch(() => ({ data: {} })),
      fetchMerchantStores({ page: 1, page_size: 100 }).catch(() => {
        storeListLoadFailed.value = true
        return { data: { list: [] } }
      })
    ])
    merchantInfo.value = infoRes.data?.merchant || infoRes.data || {}
    const storeData = storesRes?.data
    const storeList = Array.isArray(storeData?.list) ? storeData.list : (Array.isArray(storeData) ? storeData : null)
    if (storeList) {
      stores.value = storeList
    } else {
      storeListLoadFailed.value = true
      stores.value = []
    }
    stats.value.totalStores = stores.value.length
  } catch (e) {
    console.error('加载商家信息失败:', e)
  }
  return true
}

async function loadPromotionSummary() {
  if (storeListLoadFailed.value) {
    promotionSummary.value = { status: 'error', configuredEntries: 0, totalClicks: 0 }
    return
  }
  const storeIds = stores.value
    .map((store) => Number(store.id))
    .filter((storeId) => Number.isSafeInteger(storeId) && storeId > 0)
  if (storeIds.length !== stores.value.length) {
    promotionSummary.value = { status: 'error', configuredEntries: 0, totalClicks: 0 }
    return
  }
  if (storeIds.length === 0) {
    promotionSummary.value = { status: 'ready', configuredEntries: 0, totalClicks: 0 }
    return
  }

  try {
    const responses = await Promise.all(storeIds.map((storeId) => fetchMerchantGroupBuyLinks(storeId)))
    const links = responses.flatMap((response) => {
      const list = response.data?.links
      if (list == null) return []
      if (!Array.isArray(list)) throw new Error('团购接口未返回有效的链接列表')
      return list
    })
    const configuredEntries = links.filter((link) => Boolean(link.code)).length
    const totalClicks = links.reduce((sum, link) => {
      const count = Number(link.click_count)
      if (!Number.isSafeInteger(count) || count < 0 || !Number.isSafeInteger(sum + count)) {
        throw new Error('团购点击计数超出安全范围')
      }
      return sum + count
    }, 0)
    promotionSummary.value = { status: 'ready', configuredEntries, totalClicks }
  } catch (error) {
    console.error('加载团购点击摘要失败:', error)
    promotionSummary.value = { status: 'error', configuredEntries: 0, totalClicks: 0 }
  }
}

async function loadOrders() {
  orderSummaryStatus.value = 'loading'
  try {
    const res = await fetchMerchantOrders({ status: '', page: 1, page_size: 20 })
    const orderList = Array.isArray(res?.data?.list)
      ? res.data.list
      : (Array.isArray(res?.data?.items) ? res.data.items : (Array.isArray(res?.data) ? res.data : null))
    if (!orderList) throw new Error('订单接口未返回有效列表')
    orders.value = orderList

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
      }
      if (orderDate >= monthStart) {
        monthOrders++
      }
      const paidDate = order.paid_at ? new Date(order.paid_at) : null
      const payable = Number(order.total_amount ?? order.amount ?? 0)
      if (paidDate && payable > 0) {
        if (paidDate.toDateString() === today) todayRevenue += payable
        if (paidDate >= monthStart) monthRevenue += payable
      }
      if (['pending', 'failed', 'payment_confirming', 'submitted', 'received'].includes(order.status)) {
        pendingOrders++
      }
      if (Number(order.refunded_amount || 0) > 0) {
        refundRequests++
      }
    })

    stats.value.todayOrders = todayOrders
    stats.value.todayRevenue = formatCents(todayRevenue)
    stats.value.pendingOrders = pendingOrders
    stats.value.refundRequests = refundRequests
    stats.value.monthOrders = monthOrders
    stats.value.monthRevenue = formatCents(monthRevenue)
    orderSummaryStatus.value = 'ready'
  } catch (e) {
    console.error('加载订单失败:', e)
    orderSummaryStatus.value = 'error'
  }
}

onMounted(async () => {
  loading.value = true
  const canLoadBusinessData = await loadMerchantInfo()
  if (canLoadBusinessData) await Promise.all([loadOrders(), loadPromotionSummary()])
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
.header-actions { display: flex; align-items: center; gap: 10px; }
.readonly-banner { display: grid; gap: 8px; margin: 12px 16px; padding: 12px; border: 1px solid #f0d49b; border-radius: 12px; color: #80591b; background: #fff8e8; font-size: 12px; line-height: 1.5; }

.ai-entry { margin: 12px 16px; overflow: hidden; border: 1px solid #cfe4ff; border-radius: 12px; }
.ai-entry :deep(.van-cell) { background: #f2f8ff; }
.activity-intro { margin: 12px 16px; padding: 16px; border-radius: 12px; background: #fff; box-shadow: 0 2px 10px rgba(0, 0, 0, .04); }
.activity-kicker { margin-bottom: 6px; color: #1989fa; font-size: 11px; font-weight: 700; letter-spacing: .06em; }
.activity-intro .section-title { margin-bottom: 6px; }
.activity-intro p { margin: 0; color: #77868d; font-size: 12px; line-height: 1.6; }
.promotion-secondary { margin: 12px 16px; padding: 16px; border: 1px solid #e9eff1; border-radius: 12px; background: #fff; }
.promotion-kicker { display: flex; align-items: center; gap: 5px; color: #718087; font-size: 11px; font-weight: 700; }
.promotion-secondary h2 { margin: 8px 0 6px; color: #25363c; font-size: 16px; line-height: 1.4; }
.promotion-description { margin: 0; color: #718087; font-size: 12px; line-height: 1.6; }
.promotion-summary { margin-top: 12px; padding: 11px 12px; border: 1px solid #e9eff1; border-radius: 10px; background: #f8fafb; }
.promotion-summary-state { color: #64747a; font-size: 12px; line-height: 1.5; }
.promotion-metrics { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.promotion-metric { display: grid; gap: 4px; min-width: 0; }
.promotion-metric + .promotion-metric { padding-left: 12px; border-left: 1px solid #e1e8eb; }
.promotion-metric span { color: #718087; font-size: 10px; }
.promotion-metric strong { color: #25363c; font-size: 20px; line-height: 1.2; }
.promotion-metric small { color: #718087; font-size: 11px; font-weight: 500; }
.promotion-button { width: 100%; margin-top: 12px; font-weight: 700; }
.promotion-disclaimer { margin: 9px 0 0; color: #718087; font-size: 10px; line-height: 1.5; }

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
  margin: 12px 16px;
  border-radius: 12px;
  padding: 16px;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.05);
}
.stats-heading { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.stats-heading .section-title { margin-bottom: 0; }
.stats-scope { flex: 0 0 auto; color: #85939a; font-size: 10px; }
.stats-note { margin: 7px 0 10px; color: #7e8d94; font-size: 11px; line-height: 1.5; }
.order-summary-state, .pending-order-state { display: flex; align-items: center; justify-content: center; gap: 8px; padding: 16px 8px; color: #7e8d94; font-size: 12px; line-height: 1.5; text-align: center; }
.order-summary-error { flex-wrap: wrap; color: #b44b3d; }

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
