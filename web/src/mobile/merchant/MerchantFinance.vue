<template>
  <div class="merchant-finance">
    <!-- 顶部导航 -->
    <van-nav-bar
      title="订单汇总"
      left-arrow
      fixed
      placeholder
      @click-left="router.back()"
    >
      <template #right>
        <van-button size="small" @click="showDatePicker = true">
          {{ currentMonth }}
        </van-button>
      </template>
    </van-nav-bar>

    <div v-if="!canWrite" class="readonly-banner" role="status">
      订阅已到期：本页订单汇总仅供查看。<van-button size="small" type="warning" plain @click="router.push('/m/merchant/subscription')">查看续费</van-button>
    </div>

    <!-- 加载状态 -->
    <div v-if="loading" class="loading-container">
      <van-loading type="spinner" size="40px">加载中...</van-loading>
    </div>

    <template v-else>
      <!-- 概览卡片 -->
      <div class="overview-card">
        <div class="overview-header">
          <span class="overview-title">所选月份订单汇总</span>
        </div>
        <div class="overview-content">
          <div class="overview-item">
            <div class="item-value text-primary">¥{{ formatPrice(stats.paidAmount) }}</div>
            <div class="item-label">系统记录付款金额</div>
          </div>
          <div class="overview-item">
            <div class="item-value">{{ stats.monthOrders }}</div>
            <div class="item-label">按下单时间统计的订单数</div>
          </div>
          <div class="overview-item">
            <div class="item-value">¥{{ formatPrice(stats.avgPaidOrderAmount) }}</div>
            <div class="item-label">付款订单均额</div>
          </div>
        </div>
      </div>

      <p class="scope-note">
        口径说明：订单数、已完成订单数、有退款记录订单数和退款金额按订单创建时间归入所选月份；付款金额、付款订单均额及趋势按付款时间（paid_at）归月。退款订单占比按订单笔数计算，不是退款金额比例。金额字段以分记录，本页换算为元显示；金额来自系统订单/退款记录，不代表支付渠道对账或净利润。付款与退款可能归入不同月份，不应直接相减。当前仅统计页面请求到的第 1 页订单（请求上限 500 条）。
      </p>

      <!-- 退款金额只展示订单中已有的记录，不代表渠道到账核验 -->
      <van-cell-group inset title="退款记录汇总">
        <van-cell title="订单中已记录的退款金额">
          <template #value>
            <span class="expense">-¥{{ formatPrice(stats.refundAmount) }}</span>
          </template>
        </van-cell>
      </van-cell-group>

      <!-- 每日趋势 -->
      <div class="trend-section">
        <div class="section-title">所选月付款订单趋势（仅显示月末 14 天）</div>
        <div class="trend-chart">
          <div
            v-for="(day, index) in dailyStats"
            :key="index"
            class="trend-bar"
          >
            <div class="bar-fill" :style="{ height: `${getBarHeight(day.orders)}%` }">
              <span class="bar-value">{{ day.orders }}</span>
            </div>
            <span class="bar-label">{{ day.day }}</span>
          </div>
        </div>
      </div>

      <!-- 订单数量指标按下单时间归月 -->
      <van-cell-group inset title="订单统计">
        <van-cell title="已完成订单">
          <template #value>
            <span class="text-success">{{ orderStats.completed }}</span>
          </template>
        </van-cell>
        <van-cell title="有退款记录订单">
          <template #value>
            <span class="text-danger">{{ orderStats.refunded }}</span>
          </template>
        </van-cell>
        <van-cell title="有退款记录订单占比">
          <template #value>
            <span>{{ orderStats.refundRate }}%</span>
          </template>
        </van-cell>
      </van-cell-group>

    </template>

    <!-- 日期选择 -->
    <van-popup v-model:show="showDatePicker" position="bottom" style="height: 50%;">
      <van-date-picker
        v-model="currentDate"
        type="year-month"
        title="选择月份"
        @confirm="onDateConfirm"
        @cancel="showDatePicker = false"
      />
    </van-popup>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { showToast } from 'vant'
import { fetchMerchantOrders } from '@/api/modules'
import { cachedMobileMerchantSubscriptionState, mobileSubscriptionActions } from '@/utils/mobileMerchantSubscription'

const router = useRouter()
const canWrite = mobileSubscriptionActions(cachedMobileMerchantSubscriptionState()).canWriteBusiness

const loading = ref(true)
const orders = ref([])
const showDatePicker = ref(false)
const currentDate = ref(['2024', '01'])

const currentMonth = computed(() => {
  const [year, month] = currentDate.value
  return `${year}-${month}`
})

const stats = computed(() => {
  const monthOrders = orders.value.filter(o => {
    const orderDate = new Date(o.created_at)
    return orderDate.getFullYear() === parseInt(currentDate.value[0]) &&
           (orderDate.getMonth() + 1) === parseInt(currentDate.value[1])
  })

  const paidOrdersInMonth = orders.value.filter(o => {
    const paidDate = o.paid_at ? new Date(o.paid_at) : null
    return paidDate && paidDate.getFullYear() === parseInt(currentDate.value[0]) && (paidDate.getMonth() + 1) === parseInt(currentDate.value[1]) && Number(o.total_amount ?? o.amount ?? 0) > 0
  })
  const refundedOrders = monthOrders.filter(o => Number(o.refunded_amount || 0) > 0)

  const paidAmount = paidOrdersInMonth.reduce((sum, o) => sum + Number(o.total_amount ?? o.amount ?? 0), 0)
  const refundAmount = refundedOrders.reduce((sum, o) => sum + Number(o.refunded_amount || 0), 0)

  return {
    monthOrders: monthOrders.length,
    paidAmount,
    avgPaidOrderAmount: paidOrdersInMonth.length > 0 ? paidAmount / paidOrdersInMonth.length : 0,
    refundAmount
  }
})

const orderStats = computed(() => {
  const monthOrders = orders.value.filter(o => {
    const orderDate = new Date(o.created_at)
    return orderDate.getFullYear() === parseInt(currentDate.value[0]) &&
           (orderDate.getMonth() + 1) === parseInt(currentDate.value[1])
  })

  const completed = monthOrders.filter(o => o.status === 'completed').length
  const refunded = monthOrders.filter(o => Number(o.refunded_amount || 0) > 0).length

  return {
    completed,
    refunded,
    refundRate: monthOrders.length > 0 ? ((refunded / monthOrders.length) * 100).toFixed(1) : '0'
  }
})

const dailyStats = computed(() => {
  const [year, month] = currentDate.value
  const daysInMonth = new Date(parseInt(year), parseInt(month), 0).getDate()

  const result = []
  for (let day = 1; day <= daysInMonth; day++) {
    const dayOrders = orders.value.filter(o => {
      const orderDate = new Date(o.paid_at || o.created_at)
      return orderDate.getFullYear() === parseInt(year) &&
             orderDate.getMonth() + 1 === parseInt(month) &&
             orderDate.getDate() === day &&
             Boolean(o.paid_at) && Number(o.total_amount ?? o.amount ?? 0) > 0
    })
    result.push({
      day: day.toString().padStart(2, '0'),
      orders: dayOrders.length,
      revenue: dayOrders.reduce((sum, o) => sum + Number(o.total_amount ?? o.amount ?? 0), 0)
    })
  }
  return result.slice(-14) // 只显示最近14天
})

function formatPrice(price) {
  return (Number(price || 0) / 100).toFixed(2)
}

function getBarHeight(orders) {
  const maxOrders = Math.max(...dailyStats.value.map(d => d.orders), 1)
  return (orders / maxOrders) * 100
}

function onDateConfirm({ selectedValues }) {
  currentDate.value = selectedValues
  showDatePicker.value = false
  loadOrders()
}

async function loadOrders() {
  loading.value = true
  try {
    const res = await fetchMerchantOrders({ status: '', page: 1, page_size: 500 })
    orders.value = Array.isArray(res.data?.list) ? res.data.list : (Array.isArray(res.data?.items) ? res.data.items : (Array.isArray(res.data) ? res.data : []))
  } catch (e) {
    console.error('加载订单失败:', e)
    showToast('加载失败')
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  // 设置当前月份
  const now = new Date()
  currentDate.value = [
    now.getFullYear().toString(),
    (now.getMonth() + 1).toString().padStart(2, '0')
  ]
  loadOrders()
})
</script>

<style scoped>
.merchant-finance {
  min-height: 100vh;
  background: #f7f8fa;
  padding-bottom: 20px;
}
.readonly-banner { display: flex; align-items: center; justify-content: space-between; gap: 10px; margin: 10px 12px; padding: 10px 12px; border: 1px solid #f0d49b; border-radius: 12px; color: #80591b; background: #fff8e8; font-size: 12px; line-height: 1.5; }

.loading-container {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 50vh;
}

.overview-card {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: white;
  padding: 20px 16px;
  margin: 12px;
  border-radius: 12px;
}

.overview-header {
  margin-bottom: 16px;
}

.scope-note {
  margin: 12px;
  padding: 12px;
  border: 1px solid #dbeafe;
  border-radius: 10px;
  background: #eff6ff;
  color: #475569;
  font-size: 12px;
  line-height: 1.7;
}

.overview-title {
  font-size: 16px;
  font-weight: bold;
}

.overview-content {
  display: flex;
  justify-content: space-around;
}

.overview-item {
  text-align: center;
}

.item-value {
  font-size: 22px;
  font-weight: bold;
  margin-bottom: 4px;
}

.item-label {
  font-size: 12px;
  opacity: 0.9;
}

.text-primary {
  color: #fff;
}

.income {
  color: #07c160;
}

.expense {
  color: #ee0a24;
}

.income-bold {
  color: #07c160;
  font-weight: bold;
  font-size: 16px;
}

.text-success {
  color: #07c160;
  font-weight: bold;
}

.text-danger {
  color: #ee0a24;
  font-weight: bold;
}

.trend-section {
  background: white;
  margin: 12px;
  padding: 16px;
  border-radius: 12px;
}

.section-title {
  font-size: 16px;
  font-weight: bold;
  margin-bottom: 16px;
}

.trend-chart {
  display: flex;
  justify-content: space-around;
  align-items: flex-end;
  height: 120px;
  padding-top: 20px;
}

.trend-bar {
  display: flex;
  flex-direction: column;
  align-items: center;
  flex: 1;
}

.bar-fill {
  width: 20px;
  background: linear-gradient(180deg, #667eea 0%, #764ba2 100%);
  border-radius: 4px 4px 0 0;
  min-height: 4px;
  display: flex;
  align-items: flex-start;
  justify-content: center;
  margin-bottom: 4px;
}

.bar-value {
  font-size: 10px;
  color: white;
  padding: 2px;
}

.bar-label {
  font-size: 10px;
  color: #999;
}

.settlement-section {
  margin-top: 12px;
}
</style>
