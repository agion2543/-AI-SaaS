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
      <van-notice-bar v-if="isTableMode" :type="tableLocked ? 'warning' : 'primary'" :scrollable="false">
        {{ tableLabel || `${tableNo}号桌` }} · {{ tableLocked ? '该桌已有顾客结算，不能重复提交' : '共享购物车结算' }}
      </van-notice-bar>
      <van-notice-bar v-if="activeMaintenancePlan" type="warning" :text="maintenanceBlockText(activeMaintenancePlan)" wrapable />
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
          :label="orderItemLabel(item)"
        >
          <template #value>
            <span class="item-price">¥{{ formatPrice(getProductUnitPrice(item, item.count) * item.count) }}</span>
          </template>
        </van-cell>
      </van-cell-group>
      <van-notice-bar v-if="hasMOQViolation" type="warning" :text="minimumOrderIssue" wrapable />

      <!-- 订单备注 -->
      <van-cell-group inset title="备注">
        <van-field
          v-model="remark"
          placeholder="有什么特殊需求？如：少辣、不要葱等"
          maxlength="100"
          show-word-limit
        />
      </van-cell-group>

      <van-cell-group inset title="顾客手机（选填）">
        <van-field v-model="customerPhone" type="tel" maxlength="11" placeholder="填写手机号后可查询奖励券">
          <template #button>
            <van-button size="small" type="primary" @click="loadCoupons">查询奖励券</van-button>
          </template>
        </van-field>
      </van-cell-group>

      <!-- 优惠信息 -->
      <van-cell-group inset title="优惠">
        <van-cell title="优惠券" is-link :value="selectedCoupon ? `-¥${formatPrice(selectedCoupon.value)}` : '暂无可用'" @click="showCouponPicker = true" />
      </van-cell-group>

      <!-- 价格明细 -->
      <van-cell-group inset title="费用明细">
        <van-cell title="商品总价">
          <span>¥{{ formatPrice(originalPrice) }}</span>
        </van-cell>
        <van-cell title="优惠金额">
          <span class="discount-price">-¥{{ formatPrice(discountAmount) }}</span>
        </van-cell>
        <van-cell title="订单金额">
          <span class="final-price">¥{{ formatPrice(finalPrice) }}</span>
        </van-cell>
      </van-cell-group>

      <p class="payment-boundary-note">提交订单不等于完成付款；本页不会直接扣款。是否需要预付及具体付款方式，以门店说明和后续页面提示为准。</p>

      <van-notice-bar v-if="isSyntheticPreview" class="dev-notice" left-icon="info-o" :scrollable="false">
        合成演示数据 · 非真实交易
      </van-notice-bar>

      <van-notice-bar v-if="isDevMockPaymentEnabled" class="dev-notice" left-icon="info-o" :scrollable="false">
        开发标记 · 不模拟付款/不标记已付款
      </van-notice-bar>

      <!-- 底部提交栏 -->
      <van-submit-bar
        :price="finalPrice"
        label="订单金额"
        button-text="提交订单"
        :loading="submitting"
        :disabled="submitting || hasMOQViolation || (isTableMode && tableLocked) || !!activeMaintenancePlan"
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
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { showToast, showSuccessToast, showFailToast } from 'vant'
import { fetchCustomerStore, fetchCustomerStoreProducts, fetchCustomerStoreCoupons, createCustomerStoreOrder, recoverCustomerStoreOrder, fetchCustomerTableSession, checkoutCustomerTableSession, fetchPublicMaintenanceStatus } from '@/api/modules'
import { mobileCustomerOrderAccessPath, saveCustomerOrderAccessToken } from '@/utils/customerOrderAccess'
import { clearCustomerOrderAttempt, getOrCreateCustomerOrderAttempt, readCustomerOrderAttempt } from '@/utils/customerOrderAttempt'
import { buildCustomerOrderPayload, customerResponseList, fetchCustomerCouponsForPhone, isDevMockPaymentFlagEnabled, isValidCustomerPhone } from '@/utils/mobileCustomerOrder'
import { describeProductPricing, getMinimumOrderQuantity, getProductUnitPrice } from '@/utils/productPricing'
import { maintenanceBlockText } from '@/utils/maintenance'

const route = useRoute()
const router = useRouter()
const storeId = route.query.storeId
const tableSessionId = Number(route.query.session_id || 0)
const tableNo = Number(route.query.table_no || 0)
const tableCode = String(route.query.table_code || '')
const isTableMode = Number.isInteger(tableSessionId) && tableSessionId > 0 && Number.isInteger(tableNo) && tableNo > 0 && /^[a-zA-Z0-9_-]{32,64}$/.test(tableCode)
const tableLocked = ref(false)
const activeMaintenancePlan = ref(null)
const tableLabel = ref("")
const pendingCheckoutAttempt = ref(null)

// 状态
const loading = ref(true)
const submitting = ref(false)
const storeInfo = ref({})
const products = ref([])
const cart = ref({})
const remark = ref('')
const customerPhone = ref('')
const coupons = ref([])
const selectedCoupon = ref(null)
const chosenCouponIndex = ref(-1)
const showCouponPicker = ref(false)
const isDevMockPaymentEnabled = isDevMockPaymentFlagEnabled(import.meta.env)
const isSyntheticPreview = import.meta.env.DEV && route.query.synthetic === '1'

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
  return cartItems.value.reduce((sum, item) => sum + getProductUnitPrice(item, item.count) * item.count, 0)
})
const isMOQInvalid = (item) => item.count < getMinimumOrderQuantity(item)
const hasMOQViolation = computed(() => cartItems.value.some(isMOQInvalid))
const minimumOrderIssue = computed(() => {
  const invalid = cartItems.value.find(isMOQInvalid)
  if (!invalid) return ''
  const minimum = getMinimumOrderQuantity(invalid)
  return `${invalid.name} 最低起订量为 ${minimum}${invalid.sales_unit || '件'}，请返回购物车调整`
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
    .filter(c => c.status === 'unused' && Number(c.threshold || 0) <= originalPrice.value)
    .map(c => ({
      ...c,
      value: Number(c.amount || 0),
      name: c.title || `满${formatPrice(c.threshold)}减${formatPrice(c.amount)}`,
      startAt: c.valid_from ? new Date(c.valid_from).getTime() / 1000 : undefined,
      endAt: c.valid_to ? new Date(c.valid_to).getTime() / 1000 : undefined,
      originCondition: Number(c.threshold || 0),
      reason: Number(c.threshold || 0) > originalPrice.value ? `满${formatPrice(c.threshold)}可用` : ''
    }))
})

// 方法
function formatPrice(price) {
  return (Number(price || 0) / 100).toFixed(2)
}

function orderItemLabel(item) {
  const unit = item.sales_unit ? `/${item.sales_unit}` : ''
  const quantityUnit = item.sales_unit ? ` ${item.sales_unit}` : ''
  const details = [describeProductPricing(item)]
  details.push(`¥${formatPrice(getProductUnitPrice(item, item.count))}${unit} × ${item.count}${quantityUnit}`)
  return details.filter(Boolean).join(' · ')
}

function loadCart() {
  try {
    if (isTableMode) return
    const saved = localStorage.getItem(`cart_${storeId}`)
    if (saved) { cart.value = JSON.parse(saved) }
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
    selectedCoupon.value = null
    chosenCouponIndex.value = -1
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
    if (!isTableMode) localStorage.setItem(`cart_${storeId}`, JSON.stringify({}))
    showSuccessToast('已找回刚才提交的订单')
    router.replace({
      path: `/m/order/${encodeURIComponent(orderNo)}`,
      query: { access_token: accessToken, ...(isSyntheticPreview ? { synthetic: '1' } : {}) }
    })
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
    if (!isTableMode && await recoverPendingOrder(true)) return
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
    const res = isTableMode
      ? await checkoutCustomerTableSession(tableSessionId, tableCode, { request_key: attempt.request_key, access_token: attempt.access_token, customer_phone: customerPhone.value.trim(), customer_note: remark.value, coupon_no: selectedCoupon.value?.coupon_no || '' })
      : await createCustomerStoreOrder({ ...payload, request_key: attempt.request_key, access_token: attempt.access_token })
    const order = res.data.order || res.data
    const orderNo = order.order_no
    const accessToken = res.data.access_token
    if (!orderNo || !accessToken) throw new Error('订单安全凭证未返回，请勿使用订单号单独查询')
    saveCustomerOrderAccessToken(orderNo, accessToken)
    clearCustomerOrderAttempt(attempt)
    pendingCheckoutAttempt.value = null

    // 清除购物车
    cart.value = {}
    if (!isTableMode) localStorage.setItem(`cart_${storeId}`, JSON.stringify({}))

    if (res.data.payment_error) {
      showFailToast('订单已创建，但支付宝付款入口暂不可用；订单仍未确认收款')
      router.replace({
        path: `/m/order/${encodeURIComponent(orderNo)}`,
        query: {
          access_token: accessToken,
          payment_setup_unavailable: '1',
          ...(isSyntheticPreview ? { synthetic: '1' } : {})
        }
      })
      return
    }

    if (res.data.payment_config_error) {
      showFailToast(res.data.payment_config_error)
      router.replace(mobileCustomerOrderAccessPath(orderNo, accessToken))
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

    if (res.data.payment_method === 'merchant_qr' && res.data.payment_required) {
      showToast('订单已创建，请在状态页扫描商家收款码；付款后还需门店人工核对到账')
    } else if (res.data.payment_required === false || ['submitted', 'preparing'].includes(order.status)) {
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
    const requests = [fetchCustomerStore(storeId), fetchCustomerStoreProducts(storeId)]
    if (isTableMode) requests.push(fetchCustomerTableSession(tableSessionId, tableCode))
    const [storeRes, productsRes, sessionRes] = await Promise.all(requests)

    storeInfo.value = storeRes.data?.store || storeRes.data || {}
    products.value = customerResponseList(productsRes)
    coupons.value = []
    if (isTableMode) {
      const snapshot = sessionRes?.data?.session
      tableLocked.value = Boolean(snapshot?.locked)
      tableLabel.value = String(snapshot?.table_label || "")
      cart.value = Object.fromEntries((snapshot?.items || []).map(item => [String(item.product_id), Number(item.quantity)]))
    } else loadCart()
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
  if (!storeId || (route.query.table_code && !isTableMode)) {
    showToast('参数错误')
    router.back()
    return
  }
  if (!isTableMode && await recoverPendingOrder()) return
  loadData()
  // 维护横幅：公开接口只读维护状态，失败时不挡下单（后端拦截仍是最终关口）。
  try {
    const status = await fetchPublicMaintenanceStatus()
    activeMaintenancePlan.value = status?.data?.maintenance ? status?.data?.plan : null
  } catch (err) {
    activeMaintenancePlan.value = null
  }
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

.payment-boundary-note {
  margin: 12px 16px;
  padding: 10px 12px;
  border-radius: 8px;
  background: #fff7e8;
  color: #8a5a00;
  font-size: 13px;
  line-height: 1.6;
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
