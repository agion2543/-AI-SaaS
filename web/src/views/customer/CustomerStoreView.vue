<template>
  <main class="customer-store">
    <section v-if="loading" class="phone-shell page-panel">
      <el-skeleton :rows="8" animated />
    </section>

    <section v-else-if="error" class="phone-shell empty-state">
      <div class="empty-icon">!</div>
      <h1>{{ t.storeUnavailable }}</h1>
      <p>{{ error }}</p>
    </section>

    <section v-else class="phone-shell">
      <header class="store-hero">
        <div class="hero-top">
          <span>{{ merchant.name || t.localMerchant }}</span>
          <el-tag :type="available ? 'success' : 'info'" effect="light">
            {{ available ? t.opening : t.paused }}
          </el-tag>
        </div>
        <h1>{{ store.name || t.storeOrder }}</h1>
        <p>{{ store.address || t.noAddress }}</p>
        <div class="hero-meta">
          <div>
            <small>{{ t.phone }}</small>
            <strong>{{ contactPhone || t.none }}</strong>
          </div>
          <div>
            <small>{{ t.hours }}</small>
            <strong>{{ store.business_hours || t.noHours }}</strong>
          </div>
        </div>
      </header>

      <section v-if="activePromotions.length" class="promo-scroll">
        <article v-for="promotion in activePromotions" :key="promotion.id" class="promo-card">
          <strong>{{ promotion.title || promotion.name || t.promotion }}</strong>
          <span>{{ promotionText(promotion) }}</span>
        </article>
      </section>

      <section class="toolbar-card">
        <el-input v-model="keyword" clearable :placeholder="t.searchProduct" />
        <div v-if="categoryGroups.length > 1" class="category-scroll">
          <button
            v-for="group in categoryGroups"
            :key="group.category"
            type="button"
            :class="['category-chip', { active: group.category === activeCategory }]"
            @click="activeCategory = group.category"
          >
            {{ group.category }}
          </button>
        </div>
      </section>

      <section class="product-section">
        <div class="section-title">
          <div>
            <span>{{ t.scanOrder }}</span>
            <h2>{{ activeCategory || t.products }}</h2>
          </div>
          <small>{{ visibleProducts.length }} {{ t.productUnit }}</small>
        </div>

        <div v-if="visibleProducts.length" class="product-list">
          <article v-for="product in visibleProducts" :key="product.id" class="product-card">
            <div class="product-image">
              <img v-if="product.image_url" :src="product.image_url" :alt="product.name" />
              <span v-else>{{ shortName(product.name) }}</span>
            </div>
            <div class="product-info">
              <h3>{{ product.name }}</h3>
              <p>{{ product.description || t.noProductDesc }}</p>
              <div class="product-bottom">
                <strong>{{ yuan(product.price) }}</strong>
                <el-button
                  type="primary"
                  circle
                  :disabled="!available"
                  @click="addToCart(product)"
                >
                  +
                </el-button>
              </div>
            </div>
          </article>
        </div>

        <div v-else class="empty-products">
          <div class="empty-food">🍽</div>
          <h3>{{ t.noProducts }}</h3>
          <p>{{ t.noProductsTip }}</p>
        </div>
      </section>

      <button class="cart-bar" type="button" @click="openCart">
        <div class="cart-badge">{{ totalCount }}</div>
        <div class="cart-copy">
          <strong>{{ totalCount ? t.viewCart : t.cartEmpty }}</strong>
          <span v-if="estimatedDiscount > 0">{{ t.discounted }} {{ yuan(estimatedDiscount) }}</span>
          <span v-else>{{ available ? t.readyToOrder : unavailableText }}</span>
        </div>
        <div class="cart-pay">{{ yuan(estimatedPayable) }}</div>
      </button>

      <el-drawer
        v-model="cartVisible"
        :title="t.confirmOrder"
        direction="btt"
        size="78%"
        class="mobile-drawer"
      >
        <div v-if="!cartItems.length" class="empty-cart">{{ t.cartEmptyLong }}</div>

        <div v-else class="cart-list">
          <article v-for="item in cartItems" :key="item.product_id" class="cart-item">
            <div>
              <strong>{{ item.name }}</strong>
              <span>{{ yuan(item.price) }}</span>
            </div>
            <div class="cart-actions">
              <el-input-number v-model="item.quantity" :min="1" :step="1" size="small" @change="syncCart" />
              <el-button text type="danger" @click="removeFromCart(item.product_id)">
                {{ t.remove }}
              </el-button>
            </div>
          </article>
        </div>

        <div class="checkout-panel">
          <el-input v-model="customerPhone" maxlength="11" :placeholder="t.phonePlaceholder" />
          <el-input
            v-model="customerNote"
            type="textarea"
            :rows="2"
            maxlength="200"
            show-word-limit
            :placeholder="t.notePlaceholder"
          />
          <el-radio-group v-model="payMode" class="pay-mode">
            <el-radio-button label="page">{{ t.alipayPage }}</el-radio-button>
            <el-radio-button label="qr">{{ t.alipayQr }}</el-radio-button>
          </el-radio-group>

          <div class="price-box">
            <div>
              <span>{{ t.subtotal }}</span>
              <strong>{{ yuan(totalAmount) }}</strong>
            </div>
            <div v-if="estimatedDiscount > 0">
              <span>{{ t.estimateDiscount }}</span>
              <strong>-{{ yuan(estimatedDiscount) }}</strong>
            </div>
            <div class="payable">
              <span>{{ t.estimatePay }}</span>
              <strong>{{ yuan(estimatedPayable) }}</strong>
            </div>
          </div>

          <el-button
            type="primary"
            size="large"
            :loading="submitting"
            :disabled="!cartItems.length || !available"
            @click="checkout"
          >
            {{ t.checkout }}
          </el-button>
        </div>
      </el-drawer>

      <el-dialog v-model="qrDialogVisible" :title="t.qrTitle" width="92%">
        <div class="qr-wrap">
          <img :src="qrImageUrl" :alt="t.qrTitle" />
          <p>{{ t.qrTip }}</p>
        </div>
      </el-dialog>
    </section>
  </main>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  createCustomerStoreOrder,
  fetchCustomerOrder,
  fetchCustomerStore,
  fetchCustomerStoreProducts
} from '../../api/modules'

const zh = {
  localMerchant: '\u672c\u5730\u751f\u6d3b\u5546\u5bb6',
  storeOrder: '\u95e8\u5e97\u70b9\u5355',
  storeUnavailable: '\u95e8\u5e97\u6682\u4e0d\u53ef\u8bbf\u95ee',
  noAddress: '\u5546\u5bb6\u6682\u672a\u586b\u5199\u95e8\u5e97\u5730\u5740',
  opening: '\u8425\u4e1a\u63a5\u5355\u4e2d',
  paused: '\u6682\u505c\u63a5\u5355',
  phone: '\u8054\u7cfb\u7535\u8bdd',
  hours: '\u8425\u4e1a\u65f6\u95f4',
  none: '\u6682\u65e0',
  noHours: '\u5546\u5bb6\u6682\u672a\u8bbe\u7f6e',
  promotion: '\u4f18\u60e0\u6d3b\u52a8',
  searchProduct: '\u641c\u7d22\u5546\u54c1\u540d\u79f0',
  scanOrder: 'SCAN ORDER',
  products: '\u5168\u90e8\u5546\u54c1',
  productUnit: '\u4ef6\u5546\u54c1',
  noProductDesc: '\u5546\u5bb6\u6682\u672a\u586b\u5199\u5546\u54c1\u8bf4\u660e\u3002',
  noProducts: '\u6682\u65e0\u53ef\u70b9\u5546\u54c1',
  noProductsTip: '\u5546\u5bb6\u8fd8\u6ca1\u6709\u4e0a\u67b6\u5546\u54c1\uff0c\u53ef\u4ee5\u7a0d\u540e\u518d\u6765\u3002',
  viewCart: '\u67e5\u770b\u8d2d\u7269\u8f66',
  cartEmpty: '\u8d2d\u7269\u8f66',
  discounted: '\u5df2\u4f18\u60e0',
  readyToOrder: '\u9009\u597d\u540e\u53ef\u76f4\u63a5\u4e0b\u5355',
  confirmOrder: '\u786e\u8ba4\u8ba2\u5355',
  cartEmptyLong: '\u8d2d\u7269\u8f66\u8fd8\u662f\u7a7a\u7684\uff0c\u5148\u53bb\u6311\u51e0\u4e2a\u5546\u54c1\u5427\u3002',
  remove: '\u5220\u9664',
  phonePlaceholder: '\u624b\u673a\u53f7\u9009\u586b\uff0c\u65b9\u4fbf\u5546\u5bb6\u8054\u7cfb',
  notePlaceholder: '\u5907\u6ce8\u9009\u586b\uff0c\u4f8b\u5982\u5c11\u7cd6\u3001\u9884\u7ea6\u65f6\u95f4\u3001\u7279\u6b8a\u9700\u6c42',
  alipayPage: '\u8df3\u8f6c\u652f\u4ed8\u5b9d',
  alipayQr: '\u626b\u7801\u652f\u4ed8',
  subtotal: '\u5546\u54c1\u5c0f\u8ba1',
  estimateDiscount: '\u9884\u8ba1\u4f18\u60e0',
  estimatePay: '\u9884\u8ba1\u5b9e\u4ed8',
  checkout: '\u53bb\u7ed3\u7b97',
  qrTitle: '\u652f\u4ed8\u5b9d\u626b\u7801\u652f\u4ed8',
  qrTip: '\u8bf7\u4f7f\u7528\u652f\u4ed8\u5b9d\u626b\u7801\u5b8c\u6210\u652f\u4ed8\uff0c\u652f\u4ed8\u5b8c\u6210\u540e\u4f1a\u81ea\u52a8\u8df3\u8f6c\u3002',
  defaultCategory: '\u9ed8\u8ba4\u5206\u7c7b',
  storeLoadFailed: '\u95e8\u5e97\u4fe1\u606f\u4e0d\u5b58\u5728\u6216\u6682\u4e0d\u53ef\u8bbf\u95ee',
  addSuccess: '\u5df2\u52a0\u5165\u8d2d\u7269\u8f66\uff1a',
  chooseProduct: '\u8bf7\u5148\u9009\u62e9\u5546\u54c1',
  phoneInvalid: '\u624b\u673a\u53f7\u9700\u8981\u662f 11 \u4f4d\u6570\u5b57',
  orderCreated: '\u8ba2\u5355\u5df2\u521b\u5efa\uff0c\u7b49\u5f85\u652f\u4ed8\u7ed3\u679c',
  orderFailed: '\u4e0b\u5355\u5931\u8d25',
  pausedReason: '\u5546\u5bb6\u5f53\u524d\u6682\u505c\u63a5\u5355',
  fullReduction: '\u6ee1\u51cf',
  discount: '\u6298\u6263',
  coupon: '\u4f18\u60e0\u5238',
  activity: '\u6d3b\u52a8'
}

const t = zh
const route = useRoute()
const router = useRouter()
const loading = ref(true)
const submitting = ref(false)
const error = ref('')
const store = ref({})
const merchant = ref({})
const products = ref([])
const promotions = ref([])
const activeCategory = ref('')
const keyword = ref('')
const cartVisible = ref(false)
const qrDialogVisible = ref(false)
const qrImageUrl = ref('')
const customerPhone = ref('')
const customerNote = ref('')
const payMode = ref('page')
const cart = ref({})
let pollTimer = null

const contactPhone = computed(() => store.value.contact_phone || merchant.value.contact_phone || '')
const unavailableText = computed(() => store.value.pause_reason || t.pausedReason)
const available = computed(() => store.value.status === 'active' && store.value.is_open !== false && merchant.value.status === 'active')
const cartItems = computed(() => Object.values(cart.value))
const totalCount = computed(() => cartItems.value.reduce((sum, item) => sum + item.quantity, 0))
const totalAmount = computed(() => cartItems.value.reduce((sum, item) => sum + item.price * item.quantity, 0))
const activePromotions = computed(() => promotions.value.filter((item) => item.status === 'active' || !item.status))

const categoryGroups = computed(() => {
  const map = new Map()
  products.value.forEach((product) => {
    const category = product.category || t.defaultCategory
    if (!map.has(category)) map.set(category, [])
    map.get(category).push(product)
  })
  return Array.from(map.entries()).map(([category, list]) => ({ category, products: list }))
})

const visibleProducts = computed(() => {
  const selected = categoryGroups.value.find((group) => group.category === activeCategory.value)
  const list = selected?.products || products.value
  const word = keyword.value.trim().toLowerCase()
  if (!word) return list
  return list.filter((product) => `${product.name || ''}${product.description || ''}`.toLowerCase().includes(word))
})

const estimatedDiscount = computed(() => {
  if (!totalAmount.value) return 0
  return activePromotions.value.reduce((best, promotion) => Math.max(best, calcPromotionDiscount(promotion)), 0)
})
const estimatedPayable = computed(() => Math.max(totalAmount.value - estimatedDiscount.value, 0))

watch(categoryGroups, (groups) => {
  if (!activeCategory.value && groups.length) activeCategory.value = groups[0].category
}, { immediate: true })

const formatYuan = (value) => (Number(value || 0) / 100).toFixed(2)
const yuan = (value) => `\u00a5${formatYuan(value)}`
const shortName = (value = '') => value.slice(0, 2) || '\u5546\u54c1'

const promotionTypeLabel = (type) => ({
  full_reduction: t.fullReduction,
  discount: t.discount,
  coupon: t.coupon
}[type] || t.activity)

const promotionText = (promotion) => {
  const threshold = Number(promotion.threshold || promotion.threshold_amount || 0)
  const discount = Number(promotion.discount || promotion.discount_amount || 0)
  const rate = Number(promotion.discount_rate || 0)
  if (promotion.type === 'discount' && rate > 0) return `\u4e0b\u5355\u4eab ${rate} \u6298`
  if (threshold > 0 && discount > 0) return `\u6ee1 ${yuan(threshold)} \u51cf ${yuan(discount)}`
  if (discount > 0) return `\u7acb\u51cf ${yuan(discount)}`
  return promotion.description || promotionTypeLabel(promotion.type)
}

const calcPromotionDiscount = (promotion) => {
  const threshold = Number(promotion.threshold || promotion.threshold_amount || 0)
  const discount = Number(promotion.discount || promotion.discount_amount || 0)
  const rate = Number(promotion.discount_rate || 0)
  if (threshold > 0 && totalAmount.value < threshold) return 0
  if (promotion.type === 'discount' && rate > 0) {
    return Math.floor(totalAmount.value * Math.max(0, 10 - rate) / 10)
  }
  return Math.min(discount, totalAmount.value)
}

const load = async () => {
  loading.value = true
  error.value = ''
  try {
    const [storeRes, productsRes] = await Promise.all([
      fetchCustomerStore(route.params.storeId),
      fetchCustomerStoreProducts(route.params.storeId)
    ])
    store.value = storeRes.data.store || {}
    merchant.value = storeRes.data.merchant || {}
    promotions.value = storeRes.data.promotions || []
    products.value = productsRes.data.list || []
  } catch (err) {
    error.value = err.response?.data?.message || t.storeLoadFailed
  } finally {
    loading.value = false
  }
}

const addToCart = (product) => {
  cart.value = {
    ...cart.value,
    [product.id]: cart.value[product.id]
      ? { ...cart.value[product.id], quantity: cart.value[product.id].quantity + 1 }
      : { product_id: product.id, name: product.name, price: product.price, quantity: 1 }
  }
  ElMessage.success(`${t.addSuccess}${product.name}`)
}

const removeFromCart = (productId) => {
  const next = { ...cart.value }
  delete next[productId]
  cart.value = next
}

const syncCart = () => {
  const next = { ...cart.value }
  Object.values(next).forEach((item) => {
    if (item.quantity <= 0) delete next[item.product_id]
  })
  cart.value = next
}

const openCart = () => {
  cartVisible.value = true
}

const buildQrImage = (value) => `https://api.qrserver.com/v1/create-qr-code/?size=260x260&data=${encodeURIComponent(value)}`
const stopPolling = () => {
  if (pollTimer) {
    window.clearInterval(pollTimer)
    pollTimer = null
  }
}

const startPolling = (orderNo) => {
  stopPolling()
  pollTimer = window.setInterval(async () => {
    try {
      const res = await fetchCustomerOrder(orderNo)
      const status = res.data.order?.status
      if (['received', 'accepted', 'completed'].includes(status)) {
        stopPolling()
        qrDialogVisible.value = false
        cartVisible.value = false
        router.push(`/customer/orders/${encodeURIComponent(orderNo)}`)
      }
    } catch {
      // Keep the QR dialog open if a temporary polling request fails.
    }
  }, 3000)
}

const submitAlipayPage = (paymentUrl) => {
  if (!paymentUrl) return false
  if (paymentUrl.trim().startsWith('<form')) {
    document.open()
    document.write(paymentUrl)
    document.close()
    document.querySelector('form')?.submit()
    return true
  }
  window.location.href = paymentUrl
  return true
}

const checkout = async () => {
  if (!cartItems.value.length) {
    ElMessage.warning(t.chooseProduct)
    return
  }
  if (customerPhone.value && !/^1\d{10}$/.test(customerPhone.value.trim())) {
    ElMessage.warning(t.phoneInvalid)
    return
  }

  submitting.value = true
  try {
    const res = await createCustomerStoreOrder({
      store_id: Number(route.params.storeId),
      customer_phone: customerPhone.value.trim(),
      customer_note: customerNote.value.trim(),
      payment_channel: 'alipay',
      pay_mode: payMode.value,
      items: cartItems.value.map((item) => ({
        product_id: item.product_id,
        quantity: item.quantity,
        price: item.price
      }))
    })

    const order = res.data.order
    const payment = res.data.payment
    if (payment?.mode === 'page' && submitAlipayPage(payment.payment_url || payment.form)) return
    if (payment?.mode === 'qr' && payment?.qr_code) {
      qrImageUrl.value = buildQrImage(payment.qr_code)
      qrDialogVisible.value = true
      startPolling(order.order_no)
      return
    }
    ElMessage.success(t.orderCreated)
    if (order?.order_no) {
      router.push(`/customer/orders/${encodeURIComponent(order.order_no)}`)
    }
  } catch (err) {
    ElMessage.error(err.response?.data?.message || t.orderFailed)
  } finally {
    submitting.value = false
  }
}

onMounted(load)
onBeforeUnmount(stopPolling)
</script>

<style scoped>
.customer-store {
  min-height: 100vh;
  padding: 18px 0 92px;
  color: #172033;
  background:
    radial-gradient(circle at 12% 0%, rgba(255, 184, 107, 0.45), transparent 30%),
    radial-gradient(circle at 96% 12%, rgba(92, 214, 255, 0.35), transparent 32%),
    linear-gradient(180deg, #fffaf1 0%, #eef7ff 100%);
}

.phone-shell {
  width: min(430px, calc(100vw - 24px));
  margin: 0 auto;
}

.page-panel,
.empty-state {
  padding: 24px;
  border-radius: 28px;
  background: rgba(255, 255, 255, 0.94);
  box-shadow: 0 18px 60px rgba(15, 23, 42, 0.12);
}

.store-hero {
  padding: 22px;
  border-radius: 30px;
  color: #fff;
  background:
    radial-gradient(circle at 85% 12%, rgba(255, 190, 92, 0.55), transparent 34%),
    linear-gradient(145deg, #111827 0%, #1d4ed8 100%);
  box-shadow: 0 24px 58px rgba(29, 78, 216, 0.28);
}

.hero-top,
.product-bottom,
.cart-bar,
.cart-item,
.price-box div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.hero-top span {
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.08em;
  opacity: 0.86;
}

.store-hero h1 {
  margin: 20px 0 8px;
  font-size: 32px;
  line-height: 1.12;
}

.store-hero p {
  margin: 0;
  color: rgba(255, 255, 255, 0.74);
}

.hero-meta {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  margin-top: 18px;
}

.hero-meta div {
  min-width: 0;
  padding: 12px;
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.13);
}

.hero-meta small {
  display: block;
  color: rgba(255, 255, 255, 0.66);
}

.hero-meta strong {
  display: block;
  margin-top: 6px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.promo-scroll,
.category-scroll {
  display: flex;
  gap: 10px;
  overflow-x: auto;
  padding-bottom: 2px;
  scrollbar-width: none;
}

.promo-scroll::-webkit-scrollbar,
.category-scroll::-webkit-scrollbar {
  display: none;
}

.promo-scroll {
  margin-top: 14px;
}

.promo-card {
  min-width: 220px;
  padding: 13px 14px;
  border-radius: 18px;
  color: #7c2d12;
  background: linear-gradient(135deg, #fff7ed, #ffffff);
  border: 1px solid #fed7aa;
  box-shadow: 0 12px 28px rgba(154, 52, 18, 0.08);
}

.promo-card strong,
.promo-card span {
  display: block;
}

.promo-card span {
  margin-top: 5px;
  font-size: 12px;
}

.toolbar-card {
  margin-top: 14px;
  padding: 12px;
  border-radius: 22px;
  background: rgba(255, 255, 255, 0.9);
  box-shadow: 0 12px 36px rgba(15, 23, 42, 0.08);
}

.category-scroll {
  margin-top: 10px;
}

.category-chip {
  flex: 0 0 auto;
  border: 0;
  border-radius: 999px;
  padding: 9px 14px;
  color: #475569;
  background: #f1f5f9;
  font-weight: 800;
}

.category-chip.active {
  color: #fff;
  background: #0f172a;
}

.product-section {
  margin-top: 16px;
}

.section-title {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  margin-bottom: 12px;
}

.section-title span,
.section-title small {
  color: #64748b;
  font-size: 12px;
}

.section-title h2 {
  margin: 4px 0 0;
  font-size: 24px;
}

.product-list {
  display: grid;
  gap: 12px;
}

.product-card {
  display: grid;
  grid-template-columns: 96px 1fr;
  gap: 12px;
  padding: 12px;
  border-radius: 22px;
  background: rgba(255, 255, 255, 0.95);
  box-shadow: 0 12px 38px rgba(15, 23, 42, 0.09);
}

.product-image {
  width: 96px;
  height: 96px;
  border-radius: 18px;
  overflow: hidden;
  display: grid;
  place-items: center;
  color: white;
  font-weight: 900;
  font-size: 22px;
  background: linear-gradient(135deg, #f97316, #f43f5e);
}

.product-image img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.product-info {
  min-width: 0;
}

.product-info h3 {
  margin: 2px 0 5px;
  font-size: 17px;
}

.product-info p {
  display: -webkit-box;
  min-height: 38px;
  margin: 0 0 8px;
  color: #64748b;
  font-size: 13px;
  line-height: 1.45;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.product-bottom strong {
  color: #ea580c;
  font-size: 20px;
}

.empty-products {
  padding: 34px 22px;
  border-radius: 26px;
  text-align: center;
  background: rgba(255, 255, 255, 0.9);
  box-shadow: 0 12px 38px rgba(15, 23, 42, 0.08);
}

.empty-food {
  font-size: 40px;
}

.empty-products h3 {
  margin: 10px 0 6px;
}

.empty-products p,
.empty-state p {
  color: #64748b;
}

.cart-bar {
  position: fixed;
  left: 50%;
  bottom: 16px;
  z-index: 20;
  width: min(400px, calc(100vw - 24px));
  transform: translateX(-50%);
  border: 0;
  border-radius: 22px;
  padding: 12px 14px;
  color: #fff;
  background: #0f172a;
  box-shadow: 0 20px 46px rgba(15, 23, 42, 0.34);
}

.cart-badge {
  width: 34px;
  height: 34px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  background: #f97316;
  font-weight: 900;
}

.cart-copy {
  flex: 1;
  text-align: left;
}

.cart-copy strong,
.cart-copy span {
  display: block;
}

.cart-copy span {
  margin-top: 2px;
  color: rgba(255, 255, 255, 0.68);
  font-size: 12px;
}

.cart-pay {
  font-size: 18px;
  font-weight: 900;
}

.empty-icon {
  width: 44px;
  height: 44px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  color: #fff;
  background: #ef4444;
  font-weight: 900;
}

.cart-list {
  display: grid;
  gap: 10px;
}

.cart-item {
  padding: 12px;
  border-radius: 16px;
  background: #f8fafc;
}

.cart-item span {
  display: block;
  margin-top: 4px;
  color: #64748b;
}

.cart-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}

.checkout-panel {
  margin-top: 16px;
  display: grid;
  gap: 12px;
}

.pay-mode {
  width: 100%;
}

.price-box {
  display: grid;
  gap: 8px;
  padding: 14px;
  border-radius: 18px;
  background: #f8fafc;
}

.price-box span {
  color: #64748b;
}

.payable strong {
  color: #16a34a;
  font-size: 20px;
}

.qr-wrap {
  text-align: center;
}

.qr-wrap img {
  width: 240px;
  height: 240px;
}

@media (min-width: 760px) {
  .customer-store {
    padding-top: 28px;
  }

  .phone-shell {
    border-radius: 34px;
  }

  .phone-shell:not(.page-panel):not(.empty-state) {
    padding: 12px;
    background: rgba(255, 255, 255, 0.36);
    box-shadow: 0 30px 90px rgba(15, 23, 42, 0.14);
  }
}
</style>
