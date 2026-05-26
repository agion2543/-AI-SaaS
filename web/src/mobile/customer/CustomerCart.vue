<template>
  <div class="customer-cart">
    <!-- 顶部导航 -->
    <van-nav-bar
      title="购物车"
      left-arrow
      fixed
      placeholder
      @click-left="router.back()"
    >
      <template #right>
        <van-button size="small" text="清空" type="danger" @click="clearCart">清空</van-button>
      </template>
    </van-nav-bar>

    <!-- 加载状态 -->
    <div v-if="loading" class="loading-container">
      <van-loading type="spinner" size="40px">加载中...</van-loading>
    </div>

    <!-- 空购物车 -->
    <van-empty v-else-if="cartItems.length === 0" description="购物车是空的" class="empty-cart">
      <template #image>
        <van-icon name="shopping-cart-o" size="80" color="#ccc" />
      </template>
      <van-button type="primary" size="small" @click="goShopping">去逛逛</van-button>
    </van-empty>

    <!-- 购物车列表 -->
    <template v-else>
      <van-card
        v-for="item in cartItems"
        :key="item.id"
        :price="formatPrice(item.price)"
        :title="item.name"
        :desc="item.description || ''"
        :thumb="item.image || defaultProductImg"
        class="cart-item"
      >
        <template #num>
          <van-stepper
            v-model="item.count"
            :min="1"
            :max="item.stock"
            @change="(val) => updateCount(item, val)"
          />
        </template>
        <template #footer>
          <van-button size="mini" type="danger" plain @click="removeItem(item)">删除</van-button>
        </template>
      </van-card>

      <!-- 订单备注 -->
      <van-cell-group inset title="订单备注" class="remark-group">
        <van-field
          v-model="remark"
          placeholder="有什么特殊需求？如：少辣、不要葱等"
          maxlength="100"
          show-word-limit
        />
      </van-cell-group>

      <!-- 优惠券 -->
      <van-cell-group inset title="优惠" class="coupon-group">
        <van-cell
          title="优惠券"
          is-link
          :value="selectedCoupon ? `-¥${selectedCoupon.value}` : '暂无可用'"
          @click="showCouponPicker = true"
        />
      </van-cell-group>

      <!-- 价格明细 -->
      <van-cell-group inset title="价格明细" class="price-group">
        <van-cell title="商品金额">
          <span>¥{{ formatPrice(originalPrice) }}</span>
        </van-cell>
        <van-cell title="优惠">
          <span class="discount-price">-¥{{ formatPrice(discountAmount) }}</span>
        </van-cell>
        <van-cell title="实付金额">
          <span class="final-price">¥{{ formatPrice(finalPrice) }}</span>
        </van-cell>
      </van-cell-group>

      <!-- 底部结算栏 -->
      <van-submit-bar
        :price="finalPrice * 100"
        label="合计"
        button-text="提交订单"
        :disabled="cartItems.length === 0"
        @submit="submitOrder"
      />
    </template>

    <!-- 优惠券选择弹窗 -->
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
          @exchange="onCouponExchange"
        />
      </div>
    </van-popup>

    <!-- 订单提交中 -->
    <van-overlay :show="submitting">
      <div class="submitting-overlay">
        <van-loading type="spinner" size="40px">正在提交订单...</van-loading>
      </div>
    </van-overlay>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { showToast, showSuccessToast, showFailToast, showDialog } from 'vant'
import { fetchCustomerStoreProducts, fetchCustomerStoreCoupons, createCustomerStoreOrder } from '@/api/modules'

const route = useRoute()
const router = useRouter()
const storeId = route.params.storeId

// 状态
const loading = ref(true)
const submitting = ref(false)
const products = ref([])
const cart = ref({})
const remark = ref('')
const coupons = ref([])
const selectedCoupon = ref(null)
const chosenCouponIndex = ref(-1)
const showCouponPicker = ref(false)

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

const originalPrice = computed(() => {
  return cartItems.value.reduce((sum, item) => sum + item.price * item.count, 0)
})

const discountAmount = computed(() => {
  if (!selectedCoupon.value) return 0
  return Math.min(selectedCoupon.value.value, originalPrice.value)
})

const finalPrice = computed(() => {
  return Math.max(0, originalPrice.value - discountAmount.value)
})

const availableCoupons = computed(() => {
  return coupons.value
    .filter(c => c.status === 'available' && c.min_amount <= originalPrice.value)
    .map(c => ({
      ...c,
      value: c.value || 0,
      name: c.name || `满${c.min_amount}减${c.value}`,
      startAt: new Date(c.start_time).getTime() / 1000,
      endAt: new Date(c.end_time).getTime() / 1000,
      denominations: c.value,
      originCondition: c.min_amount,
      reason: c.min_amount > originalPrice.value ? `满${c.min_amount}可用` : ''
    }))
})

// 方法
function formatPrice(price) {
  return parseFloat(price || 0).toFixed(2)
}

function goShopping() {
  router.push(`/m/store/${storeId}`)
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

function updateCount(item, value) {
  if (value > 0) {
    cart.value[item.id] = value
  } else {
    delete cart.value[item.id]
  }
  saveCart()
}

function removeItem(item) {
  showDialog({
    title: '确认删除',
    message: `确定要删除 ${item.name} 吗？`,
    showCancelButton: true,
  }).then(() => {
    delete cart.value[item.id]
    saveCart()
  }).catch(() => {})
}

function clearCart() {
  showDialog({
    title: '确认清空',
    message: '确定要清空购物车吗？',
    showCancelButton: true,
  }).then(() => {
    cart.value = {}
    saveCart()
  }).catch(() => {})
}

function saveCart() {
  localStorage.setItem(`cart_${storeId}`, JSON.stringify(cart.value))
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

function onCouponExchange() {
  showToast('请输入优惠券码')
}

async function submitOrder() {
  if (cartItems.value.length === 0) {
    showToast('购物车是空的')
    return
  }

  submitting.value = true
  try {
    // 构建订单商品
    const orderItems = cartItems.value.map(item => ({
      product_id: item.id,
      quantity: item.count,
      price: item.price
    }))

    // 调用后端创建订单
    const res = await createCustomerStoreOrder({
      store_id: storeId,
      items: orderItems,
      remark: remark.value,
      coupon_id: selectedCoupon.value?.id
    })

    // 清除购物车
    cart.value = {}
    saveCart()

    // 跳转到支付页面
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
    const [productsRes, couponsRes] = await Promise.all([
      fetchCustomerStoreProducts(storeId),
      fetchCustomerStoreCoupons(storeId).catch(() => ({ data: [] }))
    ])

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
  loadData()
})
</script>

<style scoped>
.customer-cart {
  min-height: 100vh;
  background: #f7f8fa;
  padding-bottom: 60px;
}

.loading-container {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 50vh;
}

.empty-cart {
  margin-top: 100px;
}

.cart-item {
  margin: 8px;
  border-radius: 8px;
}

.remark-group,
.coupon-group,
.price-group {
  margin: 12px 0;
}

.discount-price {
  color: #07c160;
}

.final-price {
  color: #ee0a24;
  font-weight: bold;
  font-size: 16px;
}

.submitting-overlay {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  background: rgba(0, 0, 0, 0.5);
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
