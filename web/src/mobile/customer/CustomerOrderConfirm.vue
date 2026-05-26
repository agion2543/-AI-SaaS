<template>
  <div class="customer-order-confirm">
    <!-- 顶部导航 -->
    <van-nav-bar
      title="确认订单"
      left-arrow
      fixed
      placeholder
      @click-left="router.back()"
    />

    <!-- 加载状态 -->
    <div v-if="loading" class="loading-container">
      <van-loading type="spinner" size="40px">加载中...</van-loading>
    </div>

    <template v-else>
      <!-- 订单商品 -->
      <van-cell-group inset title="订单信息">
        <van-cell title="订单门店">
          <span>{{ storeInfo.name }}</span>
        </van-cell>
        <van-cell title="商品数量">
          <span>{{ totalCount }} 件</span>
        </van-cell>
      </van-cell-group>

      <!-- 商品列表 -->
      <van-cell-group inset title="商品明细">
        <van-cell
          v-for="item in cartItems"
          :key="item.id"
          :title="item.name"
          :label="`¥${formatPrice(item.price)} x ${item.count}`"
        >
          <template #value>
            <span class="item-price">¥{{ formatPrice(item.price * item.count) }}</span>
          </template>
        </van-cell>
      </van-cell-group>

      <!-- 订单备注 -->
      <van-cell-group inset title="备注">
        <van-field
          v-model="remark"
          placeholder="有什么特殊需求？如：少辣、不要葱等"
          maxlength="100"
          show-word-limit
        />
      </van-cell-group>

      <!-- 优惠信息 -->
      <van-cell-group inset title="优惠">
        <van-cell title="优惠券" is-link :value="selectedCoupon ? `-¥${selectedCoupon.value}` : '暂无可用'" @click="showCouponPicker = true" />
      </van-cell-group>

      <!-- 价格明细 -->
      <van-cell-group inset title="费用明细">
        <van-cell title="商品总价">
          <span>¥{{ formatPrice(originalPrice) }}</span>
        </van-cell>
        <van-cell title="优惠金额">
          <span class="discount-price">-¥{{ formatPrice(discountAmount) }}</span>
        </van-cell>
        <van-cell title="实付金额">
          <span class="final-price">¥{{ formatPrice(finalPrice) }}</span>
        </van-cell>
      </van-cell-group>

      <!-- 模拟支付提示 -->
      <van-notice-bar v-if="isDevMode" class="dev-notice" left-icon="info-o" scrollable>
        开发环境：当前为模拟支付，无需真实付款
      </van-notice-bar>

      <!-- 底部提交栏 -->
      <van-submit-bar
        :price="finalPrice * 100"
        label="实付"
        button-text="确认支付"
        :loading="submitting"
        :disabled="submitting"
        @submit="submitOrder"
      />
    </template>

    <!-- 优惠券弹窗 -->
    <van-popup v-model:show="showCouponPicker" position="bottom" style="max-height: 60%;">
      <div class="coupon-picker">
        <div class="coupon-header">
          <span>选择优惠券</span>
          <van-icon name="cross" @click="showCouponPicker = false" />
        </div>
        <van-coupon-list
          :coupons="availableCoupons"
          :chosen-coupon="chosenCouponIndex"
          @change="onCouponChange"
        />
      </div>
    </van-popup>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { showToast, showSuccessToast, showFailToast } from 'vant'
import { fetchCustomerStore, fetchCustomerStoreProducts, fetchCustomerStoreCoupons, createCustomerStoreOrder } from '@/api/modules'

const route = useRoute()
const router = useRouter()
const storeId = route.query.storeId

// 状态
const loading = ref(true)
const submitting = ref(false)
const storeInfo = ref({})
const products = ref([])
const cart = ref({})
const remark = ref('')
const coupons = ref([])
const selectedCoupon = ref(null)
const chosenCouponIndex = ref(-1)
const showCouponPicker = ref(false)
const isDevMode = ref(import.meta.env.DEV || true) // 开发环境默认模拟支付

const defaultProductImg = 'https://via.placeholder.com/100x100?text=商品'

// 计算属性
const cartItems = computed(() => {
  return Object.entries(cart.value)
    .filter(([_, count]) => count > 0)
    .map(([id, count]) => {
      const product = products.value.find(p => p.id.toString() === id)
      if (product) {
        return { ...product, count }
      }
      return null
    })
    .filter(Boolean)
})

const totalCount = computed(() => {
  return Object.values(cart.value).reduce((sum, count) => sum + (count || 0), 0)
})

const originalPrice = computed(() => {
  return cartItems.value.reduce((sum, item) => sum + item.price * item.count, 0)
})

const discountAmount = computed(() => {
  if (!selectedCoupon.value) return 0
  return Math.min(selectedCoupon.value.value || 0, originalPrice.value)
})

const finalPrice = computed(() => {
  return Math.max(0, originalPrice.value - discountAmount.value)
})

const availableCoupons = computed(() => {
  return coupons.value
    .filter(c => c.status === 'available' && (c.min_amount || 0) <= originalPrice.value)
    .map(c => ({
      ...c,
      value: c.value || 0,
      name: c.name || `满${c.min_amount || 0}减${c.value || 0}`,
      startAt: new Date(c.start_time).getTime() / 1000,
      endAt: new Date(c.end_time).getTime() / 1000,
      originCondition: c.min_amount || 0,
      reason: (c.min_amount || 0) > originalPrice.value ? `满${c.min_amount}可用` : ''
    }))
})

// 方法
function formatPrice(price) {
  return parseFloat(price || 0).toFixed(2)
}

function loadCart() {
  try {
    const saved = localStorage.getItem(`cart_${storeId}`)
    if (saved) {
      cart.value = JSON.parse(saved)
    }
  } catch (e) {
    cart.value = {}
  }
}

function onCouponChange(index) {
  chosenCouponIndex.value = index
  if (index >= 0) {
    selectedCoupon.value = availableCoupons.value[index]
  } else {
    selectedCoupon.value = null
  }
  showCouponPicker.value = false
}

async function submitOrder() {
  if (cartItems.value.length === 0) {
    showToast('购物车是空的')
    return
  }

  submitting.value = true
  try {
    const orderItems = cartItems.value.map(item => ({
      product_id: item.id,
      quantity: item.count,
      price: item.price
    }))

    const res = await createCustomerStoreOrder({
      store_id: storeId,
      items: orderItems,
      remark: remark.value,
      coupon_id: selectedCoupon.value?.id
    })

    // 清除购物车
    cart.value = {}
    localStorage.setItem(`cart_${storeId}`, JSON.stringify({}))

    showSuccessToast('订单创建成功')
    router.replace(`/m/order/${res.data.order_no}`)
  } catch (e) {
    console.error('创建订单失败:', e)
    showFailToast(e.response?.data?.message || '创建订单失败')
  } finally {
    submitting.value = false
  }
}

async function loadData() {
  loading.value = true
  try {
    const [storeRes, productsRes, couponsRes] = await Promise.all([
      fetchCustomerStore(storeId),
      fetchCustomerStoreProducts(storeId),
      fetchCustomerStoreCoupons(storeId).catch(() => ({ data: [] }))
    ])

    storeInfo.value = storeRes.data || {}
    products.value = productsRes.data || []
    coupons.value = couponsRes.data || []
    loadCart()
  } catch (e) {
    console.error('加载数据失败:', e)
    showToast('加载失败')
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  if (!storeId) {
    showToast('参数错误')
    router.back()
    return
  }
  loadData()
})
</script>

<style scoped>
.customer-order-confirm {
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

.item-price {
  color: #666;
}

.discount-price {
  color: #07c160;
}

.final-price {
  color: #ee0a24;
  font-weight: bold;
  font-size: 16px;
}

.dev-notice {
  margin: 12px 16px;
}

.coupon-picker {
  padding: 16px;
}

.coupon-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
  font-weight: bold;
  font-size: 16px;
}
</style>
