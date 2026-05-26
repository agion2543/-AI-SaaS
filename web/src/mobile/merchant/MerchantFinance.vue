<template>
  <div class="merchant-finance">
    <!-- 顶部导航 -->
    <van-nav-bar
      title="财务管理"
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

    <!-- 加载状态 -->
    <div v-if="loading" class="loading-container">
      <van-loading type="spinner" size="40px">加载中...</van-loading>
    </div>

    <template v-else>
      <!-- 概览卡片 -->
      <div class="overview-card">
        <div class="overview-header">
          <span class="overview-title">本月概览</span>
        </div>
        <div class="overview-content">
          <div class="overview-item">
            <div class="item-value text-primary">¥{{ formatPrice(stats.monthRevenue) }}</div>
            <div class="item-label">本月营收</div>
          </div>
          <div class="overview-item">
            <div class="item-value">{{ stats.monthOrders }}</div>
            <div class="item-label">本月订单</div>
          </div>
          <div class="overview-item">
            <div class="item-value">¥{{ formatPrice(stats.avgOrderValue) }}</div>
            <div class="item-label">客单价</div>
          </div>
        </div>
      </div>

      <!-- 收支明细 -->
      <van-cell-group inset title="收支明细">
        <van-cell title="商品销售">
          <template #value>
            <span class="income">+¥{{ formatPrice(stats.productSales) }}</span>
          </template>
        </van-cell>
        <van-cell title="优惠抵扣">
          <template #value>
            <span class="expense">-¥{{ formatPrice(stats.discountAmount) }}</span>
          </template>
        </van-cell>
        <van-cell title="退款">
          <template #value>
            <span class="expense">-¥{{ formatPrice(stats.refundAmount) }}</span>
          </template>
        </van-cell>
        <van-cell title="实际收入">
          <template #value>
            <span class="income-bold">¥{{ formatPrice(stats.monthRevenue) }}</span>
          </template>
        </van-cell>
      </van-cell-group>

      <!-- 每日趋势 -->
      <div class="trend-section">
        <div class="section-title">每日订单趋势</div>
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

      <!-- 订单统计 -->
      <van-cell-group inset title="订单统计">
        <van-cell title="已完成订单">
          <template #value>
            <span class="text-success">{{ orderStats.completed }}</span>
          </template>
        </van-cell>
        <van-cell title="已退款订单">
          <template #value>
            <span class="text-danger">{{ orderStats.refunded }}</span>
          </template>
        </van-cell>
        <van-cell title="退款率">
          <template #value>
            <span>{{ orderStats.refundRate }}%</span>
          </template>
        </van-cell>
      </van-cell-group>

      <!-- 对账单 -->
      <div class="settlement-section">
        <van-cell-group inset>
          <van-cell
            title="对账单"
            is-link
            value="查看全部"
            @click="viewAllStatements"
          />
        </van-cell-group>
      </div>
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

const router = useRouter()

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

  const completedOrders = monthOrders.filter(o =>
    ['paid', 'preparing', 'ready', 'completed'].includes(o.status)
  )
  const refundedOrders = monthOrders.filter(o =>
    ['refunded', 'cancelled'].includes(o.status)
  )

  const monthRevenue = completedOrders.reduce((sum, o) => sum + parseFloat(o.amount || 0), 0)
  const refundAmount = refundedOrders.reduce((sum, o) => sum + parseFloat(o.amount || 0), 0)

  return {
    monthOrders: monthOrders.length,
    monthRevenue,
    avgOrderValue: monthOrders.length > 0 ? monthRevenue / monthOrders.length : 0,
    productSales: monthRevenue,
    discountAmount: 0,
    refundAmount
  }
})

const orderStats = computed(() => {
  const monthOrders = orders.value.filter(o => {
    const orderDate = new Date(o.created_at)
    return orderDate.getFullYear() === parseInt(currentDate.value[0]) &&
           (orderDate.getMonth() + 1) === parseInt(currentDate.value[1])
  })

  const completed = monthOrders.filter(o =>
    ['preparing', 'ready', 'completed'].includes(o.status)
  ).length
  const refunded = monthOrders.filter(o =>
    ['refunded', 'cancelled'].includes(o.status)
  ).length

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
      const orderDate = new Date(o.created_at)
      return orderDate.getFullYear() === parseInt(year) &&
             orderDate.getMonth() + 1 === parseInt(month) &&
             orderDate.getDate() === day &&
             ['paid', 'preparing', 'ready', 'completed'].includes(o.status)
    })
    result.push({
      day: day.toString().padStart(2, '0'),
      orders: dayOrders.length,
      revenue: dayOrders.reduce((sum, o) => sum + parseFloat(o.amount || 0), 0)
    })
  }
  return result.slice(-14) // 只显示最近14天
})

function formatPrice(price) {
  return parseFloat(price || 0).toFixed(2)
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

function viewAllStatements() {
  showToast('对账单功能开发中')
}

async function loadOrders() {
  loading.value = true
  try {
    const res = await fetchMerchantOrders({ status: '', page: 1, page_size: 500 })
    orders.value = res.data?.items || res.data || []
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
