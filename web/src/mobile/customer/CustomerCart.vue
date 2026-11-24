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
        :price="formatPrice(getProductUnitPrice(item, item.count))"
        :title="item.name"
        :desc="productDescription(item)"
        :thumb="item.image_url || item.image || defaultProductImg"
        class="cart-item"
      >
        <template #num>
          <van-stepper
            v-model="item.count"
            :min="getMinimumOrderQuantity(item)"
            @change="(val) => updateCount(item, val)"
          />
        </template>
        <template #footer>
          <van-button size="mini" type="danger" plain @click="removeItem(item)">删除</van-button>
        </template>
      </van-card>

      <van-notice-bar v-if="hasMOQViolation" type="warning" :text="minimumOrderIssue" wrapable />

      <!-- 订单备注 -->
      <van-cell-group inset title="订单备注" class="remark-group">
        <van-field
          v-model="remark"
          placeholder="有什么特殊需求？如：少辣、不要葱等"
          maxlength="100"
          show-word-limit
        />
      </van-cell-group>

      <van-cell-group inset title="顾客手机（选填）" class="coupon-group">
        <van-field v-model="customerPhone" type="tel" maxlength="11" placeholder="填写手机号后可查询奖励券">
          <template #button>
            <van-button size="small" type="primary" @click="loadCoupons">查询奖励券</van-button>
          </template>
        </van-field>
      </van-cell-group>

      <!-- 优惠券 -->
      <van-cell-group inset title="优惠" class="coupon-group">
        <van-cell
          title="优惠券"
          is-link
          :value="selectedCoupon ? `-¥${formatPrice(selectedCoupon.value)}` : '暂无可用'"
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
        :price="finalPrice"
        label="合计"
        button-text="提交订单"
        :disabled="cartItems.length === 0 || hasMOQViolation"
        :tip="hasMOQViolation ? minimumOrderIssue : ''"
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
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { showToast, showSuccessToast, showFailToast, showDialog } from 'vant'
import { fetchCustomerStoreProducts, fetchCustomerStoreCoupons, createCustomerStoreOrder, recoverCustomerStoreOrder } from '@/api/modules'
import { mobileCustomerOrderAccessPath, saveCustomerOrderAccessToken } from '@/utils/customerOrderAccess'
import { clearCustomerOrderAttempt, getOrCreateCustomerOrderAttempt, readCustomerOrderAttempt } from '@/utils/customerOrderAttempt'
import { buildCustomerOrderPayload, customerResponseList, fetchCustomerCouponsForPhone, isValidCustomerPhone } from '@/utils/mobileCustomerOrder'
import { describeProductPricing, getMinimumOrderQuantity, getProductUnitPrice } from '@/utils/productPricing'

const route = useRoute()
const router = useRouter()
const storeId = route.params.storeId
const pendingCheckoutAttempt = ref(null)

// 状态
const loading = ref(true)
const submitting = ref(false)
const products = ref([])
const cart = ref({})
const remark = ref('')
const customerPhone = ref('')
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
  return cartItems.value.reduce((sum, item) => sum + getProductUnitPrice(item, item.count) * item.count, 0)
})

const isMOQInvalid = (item) => item.count < getMinimumOrderQuantity(item)
const hasMOQViolation = computed(() => cartItems.value.some(isMOQInvalid))
const minimumOrderIssue = computed(() => {
  const invalid = cartItems.value.find(isMOQInvalid)
  if (!invalid) return ''
  const minimum = getMinimumOrderQuantity(invalid)
  return `${invalid.name} 最低起订量为 ${minimum}${invalid.sales_unit || '件'}`
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
    .filter(c => c.status === 'unused' && Number(c.threshold || 0) <= originalPrice.value)
    .map(c => ({
      ...c,
      value: Number(c.amount || 0),
      name: c.title || `满${formatPrice(c.threshold)}减${formatPrice(c.amount)}`,
      startAt: c.valid_from ? new Date(c.valid_from).getTime() / 1000 : undefined,
      endAt: c.valid_to ? new Date(c.valid_to).getTime() / 1000 : undefined,
      denominations: Number(c.amount || 0),
      originCondition: Number(c.threshold || 0),
      reason: Number(c.threshold || 0) > originalPrice.value ? `满${formatPrice(c.threshold)}可用` : ''
    }))
})

// 方法
function formatPrice(price) {
  return (Number(price || 0) / 100).toFixed(2)
}

function productDescription(product) {
  return [product?.description, describeProductPricing(product)].filter(Boolean).join(' · ')
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
    const minimum = getMinimumOrderQuantity(item)
    if (value < minimum) {
      cart.value[item.id] = minimum
      showToast(`最低起订量为 ${minimum}${item.sales_unit || '件'}`)
    } else {
      cart.value[item.id] = value
    }
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

async function loadCoupons() {
  if (!isValidCustomerPhone(customerPhone.value)) {
    showToast('请输入 11 位手机号后查询')
    return
  }
  try {
    coupons.value = await fetchCustomerCouponsForPhone(fetchCustomerStoreCoupons, storeId, customerPhone.value)
    showToast(coupons.value.length ? `已找到 ${coupons.value.length} 张可用券` : '当前手机号暂无可用奖励券')
  } catch (e) {
    coupons.value = []
    showFailToast(e.response?.data?.message || '查询优惠券失败')
  }
}

async function recoverPendingOrder(strict = false) {
  const attempt = readCustomerOrderAttempt(storeId)
  if (!attempt) return false
  try {
    const res = await recoverCustomerStoreOrder({ request_key: attempt.request_key, access_token: attempt.access_token })
    const orderNo = res.data.order?.order_no
    const accessToken = res.data.access_token || attempt.access_token
    if (!orderNo) throw new Error('订单恢复响应缺少订单号')
    saveCustomerOrderAccessToken(orderNo, accessToken)
    clearCustomerOrderAttempt(attempt)
    pendingCheckoutAttempt.value = null
    cart.value = {}
    localStorage.setItem(`cart_${storeId}`, JSON.stringify({}))
    showSuccessToast('已找回刚才提交的订单')
    router.replace(mobileCustomerOrderAccessPath(orderNo, accessToken))
    return true
  } catch (e) {
    if (e.response?.status === 404) return false
    if (strict) throw new Error('订单结果暂未确认，请检查网络后重试；为避免重复下单，系统暂不发送新订单')
    showFailToast('订单结果暂未确认，请检查网络后再试；系统暂不发送新订单以避免重复')
    return false
  }
}

async function submitOrder() {
  if (submitting.value) return
  submitting.value = true
  try {
    if (await recoverPendingOrder(true)) return
    if (hasMOQViolation.value) {
      showToast(minimumOrderIssue.value)
      return
    }
    if (cartItems.value.length === 0) {
      showToast('购物车是空的')
      return
    }
    if (customerPhone.value.trim() && !isValidCustomerPhone(customerPhone.value)) {
      showToast('手机号需要是 11 位数字')
      return
    }

    const payload = buildCustomerOrderPayload({
      storeId,
      customerPhone: customerPhone.value,
      customerNote: remark.value,
      couponNo: selectedCoupon.value?.coupon_no,
      items: cartItems.value.map(item => ({ id: item.id, count: item.count, price: getProductUnitPrice(item, item.count) }))
    })
    const attempt = await getOrCreateCustomerOrderAttempt(payload, pendingCheckoutAttempt.value)
    pendingCheckoutAttempt.value = attempt
    const res = await createCustomerStoreOrder({
      ...payload,
      request_key: attempt.request_key,
      access_token: attempt.access_token
    })
    const order = res.data.order || res.data
    const orderNo = order.order_no
    const accessToken = res.data.access_token
    if (!orderNo || !accessToken) throw new Error('订单安全凭证未返回，请勿使用订单号单独查询')
    saveCustomerOrderAccessToken(orderNo, accessToken)
    clearCustomerOrderAttempt(attempt)
    pendingCheckoutAttempt.value = null

    // 清除购物车
    cart.value = {}
    saveCart()

    if (res.data.payment_error) {
      showFailToast('订单已创建，但支付宝付款入口暂不可用；订单仍未确认收款')
      router.replace({
        path: `/m/order/${encodeURIComponent(orderNo)}`,
        query: { access_token: accessToken, payment_setup_unavailable: '1' }
      })
      return
    }

    const payment = res.data.payment || {}
    const paymentUrl = payment.payment_url || payment.payment_form
    if (payment.mode === 'page' && paymentUrl) {
      if (paymentUrl.trim().startsWith('<form')) {
        document.open()
        document.write(paymentUrl)
        document.close()
        document.querySelector('form')?.submit()
      } else {
        window.location.href = paymentUrl
      }
      return
    }

    if (res.data.payment_required === false || ['submitted', 'preparing'].includes(order.status)) {
      showSuccessToast('订单已提交；如需预付请按状态页提示完成付款')
    } else {
      showToast('订单已创建，尚未确认收款；可在状态页继续付款')
    }
    router.replace(mobileCustomerOrderAccessPath(orderNo, accessToken))
  } catch (e) {
    console.error('创建订单失败:', e)
    showFailToast(e.response?.data?.message || e.message || '创建订单失败')
  } finally {
    submitting.value = false
  }
}

async function loadData() {
  loading.value = true
  try {
    const productsRes = await fetchCustomerStoreProducts(storeId)

    products.value = customerResponseList(productsRes)
    coupons.value = []
    loadCart()
  } catch (e) {
    console.error('加载数据失败:', e)
    showToast('加载失败')
  } finally {
    loading.value = false
  }
}

watch(customerPhone, () => {
  coupons.value = []
  selectedCoupon.value = null
  chosenCouponIndex.value = -1
})

onMounted(async () => {
  if (await recoverPendingOrder()) return
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
