<template>
  <main class="customer-store">
    <section v-if="loading" class="phone-shell page-panel">
      <el-skeleton :rows="8" animated />
    </section>

    <section v-else-if="error" class="phone-shell empty-state">
      <div class="empty-icon">!</div>
      <h1>门店暂不可访问</h1>
      <p>{{ error }}</p>
      <el-button type="primary" round @click="load">重新加载</el-button>
    </section>

    <section v-else class="phone-shell">
      <header class="store-hero">
        <div class="hero-top">
          <span>{{ merchant.name || '本地生活商家' }}</span>
          <el-tag :type="available ? 'success' : 'info'" effect="light">
            {{ available ? '营业接单中' : '暂停接单' }}
          </el-tag>
        </div>
        <h1>{{ store.name || '扫码点单' }}</h1>
        <p>{{ store.address || '商家暂未填写门店地址' }}</p>
        <div class="hero-meta">
          <div>
            <small>联系电话</small>
            <strong>{{ contactPhone || '暂无' }}</strong>
          </div>
          <div>
            <small>营业时间</small>
            <strong>{{ store.business_hours || '商家暂未设置' }}</strong>
          </div>
        </div>
      </header>

      <section v-if="isTableOrder" class="table-session-banner" :class="{ locked: tableSessionLocked }" role="status">
        <div>
          <span>共享桌点单 · {{ tableSession?.table_label || tableNoLabel }}</span>
          <strong>{{ tableSessionLocked ? '本桌已发起结算，购物车已锁定' : '同桌顾客可共同选购' }}</strong>
          <small v-if="tableSession?.checkout_order_no">结算订单 {{ tableSession.checkout_order_no }}</small>
          <small v-else>共享购物车会自动同步</small>
        </div>
        <el-tag :type="tableSessionLocked ? 'warning' : 'success'">{{ tableSessionLocked ? '已锁定' : '可加购' }}</el-tag>
      </section>

      <section v-if="!available" class="store-unavailable-card">
        <div>
          <span>{{ unavailableReason.label }}</span>
          <strong>{{ unavailableReason.title }}</strong>
          <p>{{ unavailableReason.text }}</p>
        </div>
        <el-button plain round :disabled="!contactPhone" @click="callStore">联系商家</el-button>
      </section>

      <section v-if="activePromotions.length" class="promo-scroll">
        <article v-for="promotion in activePromotions" :key="promotion.id" class="promo-card">
          <div class="promo-badge">{{ promotionTypeLabel(promotion.type) }}</div>
          <strong>{{ promotion.title || promotion.name || '优惠活动' }}</strong>
          <span>{{ promotionText(promotion) }}</span>
        </article>
      </section>

      <section v-if="activeGroupBuyLinks.length" class="group-buy-section" aria-labelledby="group-buy-title">
        <div class="group-buy-heading">
          <div>
            <span>EXTERNAL PLATFORM</span>
            <h2 id="group-buy-title">平台团购</h2>
          </div>
          <small>外部平台入口</small>
        </div>
        <p class="group-buy-intro">选择下方平台查看团购；购买及后续履约由对应平台处理。</p>
        <div class="group-buy-grid">
          <a
            v-for="link in activeGroupBuyLinks"
            :key="link.channel"
            class="group-buy-card"
            :class="`channel-${link.channel}`"
            :href="groupBuyRedirectUrl(link.code)"
            target="_blank"
            rel="noopener noreferrer"
          >
            <span class="group-buy-platform">{{ groupBuyPlatformName(link.channel) }}</span>
            <strong>前往{{ groupBuyPlatformName(link.channel) }}团购</strong>
            <span class="group-buy-action">打开平台 <span aria-hidden="true">↗</span></span>
          </a>
        </div>
        <p class="group-buy-disclaimer">本入口只记录成功跳转次数；点击不代表订单、销售、核销、退款或新增顾客。</p>
      </section>

      <section v-if="activeOrderNo" class="active-order-strip">
        <div>
          <span>{{ appendableActiveOrder ? '继续追加到当前订单' : '当前订单不可追加' }}</span>
          <strong>{{ activeOrderSummary }}</strong>
          <p>{{ appendableActiveOrder ? '本次选择会合并到同一笔未结算订单，商家端可看到追加记录。' : '该订单可能已进入付款确认或已结束；可以新开一笔订单。' }}</p>
        </div>
        <el-button text type="primary" @click="clearActiveOrder">新开一单</el-button>
      </section>

      <section v-else-if="recentOrderNo" class="active-order-strip recent">
        <div>
          <span>最近订单</span>
          <strong>{{ recentOrderNo }}</strong>
          <p>如果刚刚离开了付款或订单状态页，可以继续查看订单进度。</p>
        </div>
        <div class="strip-actions">
          <el-button text type="primary" @click="openRecentOrder">查看</el-button>
          <el-button text @click="clearRecentOrder">隐藏</el-button>
        </div>
      </section>

      <section v-if="recommendCards.length" class="recommend-section">
        <div class="section-title compact">
          <div>
            <span>SMART PICK</span>
            <h2>先看这些</h2>
          </div>
          <small>{{ bestDealText }}</small>
        </div>
        <div class="recommend-scroll">
          <button
            v-for="item in recommendCards"
            :key="item.product.id"
            type="button"
            class="recommend-card"
            @click="addToCart(item.product)"
          >
            <span>{{ item.label }}</span>
            <strong>{{ item.product.name }}</strong>
            <small>{{ yuan(item.product.price) }} · {{ item.hint }}</small>
          </button>
        </div>
      </section>

      <section class="toolbar-card">
        <el-input v-model="keyword" clearable placeholder="搜索商品或服务">
          <template #prefix>
            <span class="input-icon">⌕</span>
          </template>
        </el-input>
        <div v-if="categoryGroups.length > 1" class="category-scroll">
          <button
            v-for="group in categoryGroups"
            :key="group.category"
            type="button"
            :class="['category-chip', { active: group.category === activeCategory }]"
            @click="activeCategory = group.category"
          >
            <span>{{ group.category }}</span>
            <b>{{ group.products.length }}</b>
          </button>
        </div>
      </section>

      <section class="product-section">
        <div class="section-title">
          <div>
            <span>SCAN ORDER</span>
            <h2>{{ activeCategory || '全部商品' }}</h2>
          </div>
          <small>{{ visibleProducts.length }} 件</small>
        </div>

        <div v-if="productLoadError" class="empty-products product-load-error" role="status">
          <div class="empty-food" aria-hidden="true">!</div>
          <h3>商品列表暂时加载失败</h3>
          <p>这不代表门店没有商品。请重试加载后再浏览和下单。</p>
          <el-button type="primary" round :loading="productLoading" @click="loadProducts">重试加载</el-button>
        </div>
        <div v-else-if="productLoading && !loading" class="empty-products product-loading" role="status">
          <el-skeleton :rows="3" animated />
          <p>正在重新加载商品…</p>
        </div>
        <div v-else-if="visibleProducts.length" class="product-list">
          <article
            v-for="product in visibleProducts"
            :key="product.id"
            :class="productCardClass(product)"
          >
            <div class="product-image">
              <img
                v-if="shouldShowProductImage(product)"
                :src="product.image_url"
                :alt="product.name"
                @error="markImageBroken(product.id)"
              />
              <span v-else>{{ shortName(product.name) }}</span>
              <em v-if="productRibbon(product)" class="product-ribbon">{{ productRibbon(product) }}</em>
              <b v-if="productQuantity(product.id) > 0" class="product-count">x{{ productQuantity(product.id) }}</b>
            </div>
            <div class="product-info">
              <div class="product-title-row">
                <h3>{{ product.name }}</h3>
                <el-tag size="small" effect="plain">{{ product.category || defaultCategory }}</el-tag>
              </div>
              <p>{{ product.description || '商家暂未填写商品说明。' }}</p>
              <div class="product-meta-line">
                <span v-for="badge in productBadges(product).slice(0, 2)" :key="badge">{{ badge }}</span>
                <span v-if="productQuantity(product.id) > 0">已选 {{ productQuantity(product.id) }} 份</span>
              </div>
              <div class="product-bottom">
                <strong>{{ yuan(product.price) }}</strong>
                <div v-if="productQuantity(product.id) > 0" class="product-stepper">
                  <button type="button" :disabled="!canEditTableCart" @click="decreaseProduct(product.id)">-</button>
                  <span>{{ productQuantity(product.id) }}</span>
                  <button type="button" :disabled="!canEditTableCart" @click="addToCart(product)">+</button>
                </div>
                <el-button v-else type="primary" round :disabled="!canEditTableCart" @click="addToCart(product)">
                  加入
                </el-button>
              </div>
            </div>
          </article>
        </div>

        <div v-else class="empty-products">
          <div class="empty-food">品</div>
          <h3>{{ emptyProductTitle }}</h3>
          <p>{{ emptyProductText }}</p>
        </div>
      </section>

      <button v-if="totalCount > 0" type="button" class="cart-nudge" :class="{ good: discountTotal > 0 }" @click="openCart">
        <span>{{ cartNudge.label }}</span>
        <strong>{{ cartNudge.title }}</strong>
        <small>{{ cartNudge.text }}</small>
      </button>

      <button :class="['cart-bar', { 'has-items': totalCount > 0 }]" type="button" @click="openCart">
        <div class="cart-icon-wrap">
          <span class="cart-symbol">购</span>
          <div class="cart-badge">{{ totalCount }}</div>
        </div>
        <div class="cart-copy">
          <strong>{{ totalCount ? `${totalCount} 件商品，去确认` : '购物车是空的' }}</strong>
          <span v-if="selectedCouponDiscount > 0">奖励券已抵扣 {{ yuan(selectedCouponDiscount) }}</span>
          <span v-else-if="estimatedDiscount > 0">活动已优惠 {{ yuan(estimatedDiscount) }}</span>
          <span v-else>{{ available ? '选好后可直接下单' : unavailableText }}</span>
        </div>
        <div class="cart-pay">
          <small>{{ payableLabel }}</small>
          <strong>{{ yuan(estimatedPayable) }}</strong>
        </div>
      </button>

      <el-drawer
        v-model="cartVisible"
        title="确认订单"
        direction="btt"
        size="88%"
        class="mobile-drawer"
      >
        <div class="drawer-body">
          <div v-if="!cartItems.length" class="empty-cart">
            <strong>购物车还是空的</strong>
            <p>先去挑几个商品，选好后这里会自动计算优惠和实付金额。</p>
          </div>

          <div v-else class="cart-list">
            <div class="cart-list-head">
              <span>已选 {{ totalCount }} 件</span>
              <el-button text type="danger" :disabled="tableSessionLocked || cartBusy" @click="clearCart">清空</el-button>
            </div>
            <article v-for="item in cartItems" :key="item.product_id" class="cart-item">
              <div>
                <strong>{{ item.name }}</strong>
                <span>{{ yuan(item.price) }}</span>
              </div>
              <div class="cart-actions">
                <el-input-number
                  :model-value="item.quantity"
                  :min="1"
                  :step="1"
                  size="small"
                  :disabled="tableSessionLocked || cartBusy"
                  @change="(value) => setCartQuantity(item.product_id, value)"
                />
                <el-button text type="danger" :disabled="tableSessionLocked || cartBusy" @click="removeFromCart(item.product_id)">删除</el-button>
              </div>
            </article>
          </div>

          <div class="checkout-panel">
            <div class="checkout-trust">
              <div>
                <span>{{ checkoutTrustCards[0].label }}</span>
                <strong>{{ checkoutTrustCards[0].title }}</strong>
              </div>
              <div>
                <span>{{ checkoutTrustCards[1].label }}</span>
                <strong>{{ checkoutTrustCards[1].title }}</strong>
              </div>
            </div>
            <el-input v-model="customerPhone" maxlength="11" placeholder="手机号选填，便于商家联系和查询奖励券" />
            <p v-if="phoneTip" :class="['field-tip', { danger: phoneInvalid }]">{{ phoneTip }}</p>
            <div v-if="!appendableActiveOrder" class="coupon-panel">
              <div class="coupon-head">
                <span>奖励券 / 复购券</span>
                <el-button text :disabled="!/^1\\d{10}$/.test(customerPhone.trim())" @click="loadCoupons">
                  查询可用券
                </el-button>
              </div>
              <el-select v-model="selectedCouponNo" clearable placeholder="输入手机号后可选择奖励券" class="coupon-select">
                <el-option
                  v-for="coupon in coupons"
                  :key="coupon.coupon_no"
                  :label="`${coupon.title || '奖励券'}：满 ${yuan(coupon.threshold)} 减 ${yuan(coupon.amount)}`"
                  :value="coupon.coupon_no"
                />
              </el-select>
              <div v-if="selectedCoupon" class="coupon-selected-card">
                <div>
                  <span>{{ selectedCoupon.title || '奖励券' }}</span>
                  <strong>满 {{ yuan(selectedCoupon.threshold) }} 减 {{ yuan(selectedCoupon.amount) }}</strong>
                </div>
                <b v-if="selectedCouponDiscount > 0">本单抵扣 {{ yuan(selectedCouponDiscount) }}</b>
                <b v-else class="muted">还差 {{ yuan(couponGap) }} 可用</b>
              </div>
              <p v-if="couponTip" :class="['field-tip', { danger: selectedCoupon && !selectedCouponUsable }]">{{ couponTip }}</p>
            </div>
            <el-input
              v-model="customerNote"
              type="textarea"
              :rows="2"
              maxlength="200"
              show-word-limit
              placeholder="备注选填，例如规格偏好、预约时间、特殊要求"
            />
            <el-radio-group v-if="!isSubmitLaterMode && !isTableOrder" v-model="payMode" class="pay-mode">
              <el-radio-button label="page">跳转支付</el-radio-button>
              <el-radio-button label="qr">扫码支付</el-radio-button>
            </el-radio-group>
            <p class="field-tip">{{ checkoutModeTip }}</p>

            <div class="checkout-status-card" :class="{ good: totalCount > 0 }">
              <div>
                <span>{{ checkoutStatusLabel }}</span>
                <strong>{{ checkoutStatusTitle }}</strong>
              </div>
              <p>{{ checkoutStatusText }}</p>
            </div>

            <div class="price-box">
              <div>
                <span>商品小计</span>
                <strong>{{ yuan(totalAmount) }}</strong>
              </div>
              <div v-if="estimatedDiscount > 0">
                <span>活动优惠</span>
                <strong>-{{ yuan(estimatedDiscount) }}</strong>
              </div>
              <div v-if="selectedCouponDiscount > 0">
                <span>奖励券抵扣</span>
                <strong>-{{ yuan(selectedCouponDiscount) }}</strong>
              </div>
              <div class="payable">
                <span>{{ payableLabel }}</span>
                <strong>{{ yuan(estimatedPayable) }}</strong>
              </div>
              <p class="pay-confirm">{{ checkoutConfirmText }}</p>
            </div>

            <div v-if="dealHint" class="deal-hint" :class="{ good: discountTotal > 0 }">
              <strong>{{ dealHint.title }}</strong>
              <p>{{ dealHint.text }}</p>
            </div>

            <div class="drawer-submit-bar">
              <div>
                <span>{{ payableLabel }}</span>
                <strong>{{ yuan(estimatedPayable) }}</strong>
                <small v-if="discountTotal > 0">已减 {{ yuan(discountTotal) }}</small>
                <small v-else>{{ isSubmitLaterMode ? '提交后可继续查看进度' : '支付后订单会发送给商家' }}</small>
              </div>
              <el-button
                type="primary"
                size="large"
                :loading="submitting"
                :disabled="checkoutDisabled"
                @click="checkout"
              >
                {{ checkoutButtonText }}
              </el-button>
            </div>
          </div>
        </div>
      </el-drawer>

      <el-dialog v-model="qrDialogVisible" title="支付宝扫码支付" width="92%">
        <div class="qr-wrap">
          <img :src="qrImageUrl" alt="支付宝扫码支付" />
          <p>请使用支付宝扫码完成支付，支付完成后会自动跳转订单状态页。</p>
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
  appendCustomerOrderItems,
  createCustomerStoreOrder,
  fetchCustomerOrder,
  fetchCustomerStore,
  fetchCustomerStoreCoupons,
  fetchCustomerStoreProducts,
  joinCustomerTableSession,
  fetchCustomerTableSession,
  updateCustomerTableCartItem,
  clearCustomerTableCart,
  checkoutCustomerTableSession,
  recoverCustomerOrderAppend,
  recoverCustomerStoreOrder,
  trackShareScan
} from '../../api/modules'
import { customerOrderAccessPath, getCustomerOrderAccessToken, saveCustomerOrderAccessToken } from '../../utils/customerOrderAccess'

const route = useRoute()
const router = useRouter()
const defaultCategory = '默认分类'
const allCategory = '全部商品'
const loading = ref(true)
const submitting = ref(false)
const recoveryChecking = ref(false)
const pendingCheckoutAttempt = ref(null)
const pendingAppendAttempt = ref(null)
const error = ref('')
const store = ref({})
const merchant = ref({})
const products = ref([])
const productLoading = ref(false)
const productLoadError = ref(false)
const promotions = ref([])
const groupBuyLinks = ref([])
const activeCategory = ref('')
const keyword = ref('')
const brokenImages = ref(new Set())
const cartVisible = ref(false)
const qrDialogVisible = ref(false)
const qrImageUrl = ref('')
const customerPhone = ref('')
const customerNote = ref('')
const payMode = ref('page')
const cart = ref({})
const tableSession = ref(null)
const cartBusy = ref(false)
let cartMutationQueue = Promise.resolve()
const coupons = ref([])
const selectedCouponNo = ref('')
const activeOrder = ref(null)
const recentOrderNo = ref(localStorage.getItem(`customer_recent_order_${route.params.storeId}`) || '')
let pollTimer = null
let tablePollTimer = null

const tableNo = computed(() => Number(route.query.table_no || route.query.tableNo || 0))
const tableCode = computed(() => String(route.query.table_code || route.query.tableCode || route.query.code || '').trim())
const isTableOrder = computed(() => tableNo.value > 0 && Boolean(tableCode.value))
const tableSessionLocked = computed(() => Boolean(tableSession.value?.locked || tableSession.value?.status === 'checkout_locked'))
const canEditTableCart = computed(() => available.value && !tableSessionLocked.value && !cartBusy.value)
const tableNoLabel = computed(() => tableNo.value ? `桌号 ${tableNo.value}` : '桌位')
const shareCode = computed(() => String(route.query.share_code || '').trim())
const activeOrderNo = computed(() => String(route.query.active_order || '').trim())
const contactPhone = computed(() => store.value.contact_phone || merchant.value.contact_phone || '')
const unavailableText = computed(() => store.value.pause_reason || '商家当前暂停接单')
const available = computed(() => store.value.status === 'active' && store.value.is_open !== false && merchant.value.status === 'active')
const isSubmitLaterMode = computed(() => store.value.order_mode === 'submit_later')
const unavailableReason = computed(() => {
  if (merchant.value.status && merchant.value.status !== 'active') {
    return { label: '商家暂不可用', title: '当前商家暂未开放接单', text: '可以稍后再来，或联系商家确认服务时间。' }
  }
  if (store.value.status && store.value.status !== 'active') {
    return { label: '门店已停用', title: '当前门店暂不可下单', text: '该门店可能正在调整，建议联系商家确认可用门店。' }
  }
  return { label: '暂停接单', title: unavailableText.value, text: '你仍可查看商品和金额；恢复接单后即可提交订单。' }
})
const cartItems = computed(() => Object.values(cart.value))
const totalCount = computed(() => cartItems.value.reduce((sum, item) => sum + item.quantity, 0))
const totalAmount = computed(() => cartItems.value.reduce((sum, item) => sum + item.price * item.quantity, 0))
const activePromotions = computed(() => promotions.value.filter((item) => item.status === 'active' || !item.status))
const activeGroupBuyLinks = computed(() => groupBuyLinks.value
  .filter((link) => ['douyin', 'meituan'].includes(link.channel) && link.code)
  .sort((a, b) => ['douyin', 'meituan'].indexOf(a.channel) - ['douyin', 'meituan'].indexOf(b.channel)))
const groupBuyPlatformName = (channel) => ({ douyin: '抖音', meituan: '美团 / 大众点评' }[channel] || '团购平台')
const groupBuyRedirectUrl = (code) => `${window.location.origin}/api/v1/group-buy/r/${encodeURIComponent(code)}`
const activeProducts = computed(() => [...products.value]
  .filter((product) => !product.status || product.status === 'active')
  .sort((a, b) => {
    const sortDiff = Number(a.sort || 100) - Number(b.sort || 100)
    if (sortDiff !== 0) return sortDiff
    return Number(a.id || 0) - Number(b.id || 0)
  }))
const orderableProducts = computed(() => activeProducts.value)

const categoryGroups = computed(() => {
  const map = new Map()
  activeProducts.value.forEach((product) => {
    const category = product.category || defaultCategory
    if (!map.has(category)) map.set(category, [])
    map.get(category).push(product)
  })
  const groups = Array.from(map.entries()).map(([category, list]) => ({ category, products: list }))
  return [{ category: allCategory, products: activeProducts.value }, ...groups]
})

const visibleProducts = computed(() => {
  const selected = categoryGroups.value.find((group) => group.category === activeCategory.value)
  const list = selected?.products || activeProducts.value
  const word = keyword.value.trim().toLowerCase()
  if (!word) return list
  return list.filter((product) => `${product.name || ''}${product.description || ''}${product.category || ''}`.toLowerCase().includes(word))
})
const recommendCards = computed(() => {
  const cards = []
  const selectedIds = new Set()
  const bySort = orderableProducts.value.slice(0, 3)
  bySort.forEach((product, index) => {
    if (selectedIds.has(product.id)) return
    selectedIds.add(product.id)
    cards.push({
      product,
      label: index === 0 ? '招牌推荐' : '优先展示',
      hint: product.description ? '商家推荐靠前' : '适合先试试'
    })
  })
  const promoTarget = orderableProducts.value.find((product) => activePromotions.value.some((promotion) => Number(product.price || 0) >= Number(promotion.threshold || promotion.threshold_amount || 0)))
  if (promoTarget && !selectedIds.has(promoTarget.id)) {
    cards.unshift({ product: promoTarget, label: '凑优惠', hint: bestPromotionText.value || '更容易触发活动优惠' })
  }
  return cards.slice(0, 4)
})
const bestPromotion = computed(() => {
  if (!activePromotions.value.length) return null
  return activePromotions.value
    .map((promotion) => ({
      promotion,
      discount: calcPromotionDiscount(promotion),
      gap: promotionGap(promotion)
    }))
    .sort((a, b) => b.discount - a.discount || a.gap - b.gap)[0]?.promotion || activePromotions.value[0]
})
const bestPromotionText = computed(() => bestPromotion.value ? promotionText(bestPromotion.value) : '')
const bestDealText = computed(() => {
  if (discountTotal.value > 0) return `已省 ${yuan(discountTotal.value)}`
  if (bestPromotion.value) {
    const gap = promotionGap(bestPromotion.value)
    return gap > 0 ? `差 ${yuan(gap)} 享优惠` : bestPromotionText.value
  }
  return '按商家排序推荐'
})
const emptyProductTitle = computed(() => keyword.value.trim() ? '没有找到相关商品' : '暂无可点商品')
const emptyProductText = computed(() => keyword.value.trim() ? '可以换个关键词或切换分类看看。' : '商家还没有上架商品，可以稍后再来。')
const appendableActiveOrder = computed(() => !isTableOrder.value && Boolean(activeOrderNo.value && activeOrder.value && ['submitted', 'preparing'].includes(activeOrder.value.status) && !activeOrder.value.paid_at))
const activeOrderSummary = computed(() => {
  if (!activeOrderNo.value) return ''
  if (!activeOrder.value) return `订单 ${activeOrderNo.value}`
  const quantity = (activeOrder.value.items || []).reduce((sum, item) => sum + Number(item.quantity || 0), 0)
  return `${activeOrder.value.order_no || activeOrderNo.value} · 已有 ${quantity} 件 · ${yuan(activeOrder.value.total_amount ?? activeOrder.value.amount ?? 0)}`
})

const estimatedDiscount = computed(() => {
  if (appendableActiveOrder.value) return 0
  if (!totalAmount.value) return 0
  return activePromotions.value.reduce((best, promotion) => Math.max(best, calcPromotionDiscount(promotion)), 0)
})
const selectedCoupon = computed(() => coupons.value.find((item) => item.coupon_no === selectedCouponNo.value))
const phoneInvalid = computed(() => Boolean(customerPhone.value.trim()) && !/^1\d{10}$/.test(customerPhone.value.trim()))
const phoneTip = computed(() => {
  if (!customerPhone.value.trim()) return '手机号可不填；如需使用奖励券或方便商家联系，建议填写 11 位手机号。'
  if (phoneInvalid.value) return '手机号格式不正确，应为 11 位数字。'
  return '手机号格式正确，可查询并使用奖励券。'
})
const selectedCouponDiscount = computed(() => {
  if (appendableActiveOrder.value) return 0
  const coupon = selectedCoupon.value
  if (!coupon) return 0
  const afterPromotion = Math.max(totalAmount.value - estimatedDiscount.value, 0)
  if (afterPromotion < Number(coupon.threshold || 0)) return 0
  return Math.min(Number(coupon.amount || 0), afterPromotion)
})
const selectedCouponUsable = computed(() => Boolean(selectedCoupon.value) && selectedCouponDiscount.value > 0)
const couponGap = computed(() => {
  if (!selectedCoupon.value) return 0
  const afterPromotion = Math.max(totalAmount.value - estimatedDiscount.value, 0)
  return Math.max(Number(selectedCoupon.value.threshold || 0) - afterPromotion, 0)
})
const couponTip = computed(() => {
  if (!customerPhone.value.trim()) return ''
  if (phoneInvalid.value) return '手机号正确后才能查询奖励券。'
  if (!coupons.value.length) return '暂无可用奖励券，可继续直接下单。'
  if (!selectedCoupon.value) return `已查询到 ${coupons.value.length} 张券，可选择符合条件的券抵扣。`
  if (!selectedCouponUsable.value) return `当前订单未达到该券门槛：满 ${yuan(selectedCoupon.value.threshold)} 可用。`
  return `本单已使用奖励券抵扣 ${yuan(selectedCouponDiscount.value)}。`
})
const discountTotal = computed(() => estimatedDiscount.value + selectedCouponDiscount.value)
const estimatedPayable = computed(() => Math.max(totalAmount.value - discountTotal.value, 0))
const payableLabel = computed(() => (isSubmitLaterMode.value || appendableActiveOrder.value ? '预计应付' : '应付金额'))
const checkoutStatusLabel = computed(() => (discountTotal.value > 0 ? '优惠已生效' : '结算确认'))
const checkoutStatusTitle = computed(() => {
  if (appendableActiveOrder.value) return '将追加到当前订单'
  if (isSubmitLaterMode.value) return '提交后可继续查看进度'
  if (isTableOrder.value) return '提交后使用商家收款码'
  return '提交后进入付款'
})
const checkoutStatusText = computed(() => {
  if (discountTotal.value > 0 && isTableOrder.value && !isSubmitLaterMode.value) return `本单已抵扣 ${yuan(discountTotal.value)}；款项直接进入商家账户，付款后需门店人工核对到账。`
  if (discountTotal.value > 0) return `本单已抵扣 ${yuan(discountTotal.value)}，提交后以订单状态页金额为准。`
  if (appendableActiveOrder.value) return '本次商品会合并到同一笔未结算订单，商家端可看到追加记录。'
  if (isSubmitLaterMode.value) return '订单会先发送给商家，后续可在状态页继续结算或查看处理进度。'
  if (isTableOrder.value) return '款项直接进入商家自己的支付宝或微信账户；付款后需门店人工核对，页面返回不代表已收款。'
  return '请确认商品、备注和应付金额，付款成功后订单会发送给商家处理。'
})
const dealHint = computed(() => {
  if (!cartItems.value.length) return null
  if (selectedCouponDiscount.value > 0) {
    return { title: `奖励券已抵扣 ${yuan(selectedCouponDiscount.value)}`, text: '已按当前订单金额计算抵扣，提交支付后以订单页为准。' }
  }
  if (estimatedDiscount.value > 0) {
    return { title: `活动已优惠 ${yuan(estimatedDiscount.value)}`, text: bestPromotionText.value || '已自动匹配当前最优活动。' }
  }
  if (bestPromotion.value) {
    const gap = promotionGap(bestPromotion.value)
    if (gap > 0) return { title: `再加 ${yuan(gap)} 可享优惠`, text: bestPromotionText.value }
  }
  return { title: '当前无优惠抵扣', text: '可继续下单；如有奖励券，填写手机号后可查询。' }
})
const cartNudge = computed(() => {
  if (discountTotal.value > 0) {
    return { label: '优惠已生效', title: `已为你抵扣 ${yuan(discountTotal.value)}`, text: `${payableLabel.value} ${yuan(estimatedPayable.value)}` }
  }
  if (bestPromotion.value) {
    const gap = promotionGap(bestPromotion.value)
    if (gap > 0) return { label: '凑单提示', title: `再加 ${yuan(gap)} 可享优惠`, text: bestPromotionText.value || '点开购物车查看明细' }
  }
  return { label: isSubmitLaterMode.value ? '先提交后结算' : '准备结算', title: `${totalCount.value} 件商品`, text: `${payableLabel.value} ${yuan(estimatedPayable.value)}` }
})
const checkoutDisabled = computed(() => {
  if (!cartItems.value.length || !available.value || (isTableOrder.value && tableSessionLocked.value)) return true
  if (phoneInvalid.value) return true
  if (!appendableActiveOrder.value && selectedCoupon.value && !selectedCouponUsable.value) return true
  return submitting.value || recoveryChecking.value
})
const checkoutButtonText = computed(() => {
  if (!available.value) return '暂停接单'
  if (isTableOrder.value && tableSessionLocked.value) return '本桌已有人发起结算'
  if (!cartItems.value.length) return '请先选商品'
  if (phoneInvalid.value) return '手机号格式不正确'
  if (selectedCoupon.value && !selectedCouponUsable.value) return '优惠券未达门槛'
  if (appendableActiveOrder.value) return '追加到当前订单'
  if (isSubmitLaterMode.value) return '提交订单'
  return isTableOrder.value ? '提交并扫码付款' : '提交并付款'
})
const checkoutModeTip = computed(() => {
  if (appendableActiveOrder.value) return '当前为继续追加模式，本次商品会合并到同一笔未结算订单。'
  if (isSubmitLaterMode.value) return '当前门店为先提交后结算模式，提交后可在订单状态页查看进度，后续再完成结算。'
  if (isTableOrder.value) return '桌位订单使用门店已审核的商家收款码；平台不代收，门店核对到账前不会标记为已收款。'
  return '建议在手机上使用“跳转支付”；如果是在电脑或店内平板，可选择扫码支付。'
})
const checkoutConfirmText = computed(() => {
  if (appendableActiveOrder.value) return '确认后会把本次商品追加到当前订单，商家端可看到追加记录。'
  if (isSubmitLaterMode.value) return '确认后将创建订单并发送给商家；后续可在订单状态页查看进度和结算。'
  if (isTableOrder.value) return '确认后将锁定本桌共享购物车；请在订单页使用商家收款码付款，并等待门店人工核对到账。'
  return '确认后将创建订单并进入支付；支付成功后可在订单状态页查看接单进度。'
})
const checkoutTrustCards = computed(() => {
  if (appendableActiveOrder.value) {
    return [
      { label: '追加订单', title: '合并到当前进行中的订单' },
      { label: '订单可追踪', title: '状态页可查看全部商品明细' }
    ]
  }
  if (isSubmitLaterMode.value) {
    return [
      { label: '先提交订单', title: '订单先发送给商家处理' },
      { label: '后续可结算', title: '状态页可继续查看和付款' }
    ]
  }
  return [
    { label: '付款后处理', title: '支付成功后进入商家处理' },
    { label: '订单可追踪', title: '状态页可查看接单和核对码' }
  ]
})

watch(categoryGroups, (groups) => {
  if (!activeCategory.value && groups.length) activeCategory.value = groups[0].category
  if (activeCategory.value && !groups.some((group) => group.category === activeCategory.value)) {
    activeCategory.value = groups[0]?.category || ''
  }
}, { immediate: true })

watch(customerPhone, () => {
  selectedCouponNo.value = ''
  coupons.value = []
})

const formatYuan = (value) => (Number(value || 0) / 100).toFixed(2)
const yuan = (value) => `¥${formatYuan(value)}`
const shortName = (value = '') => value.slice(0, 2) || '商品'
const productQuantity = (productId) => Number(cart.value[productId]?.quantity || 0)
const productCardClass = (product) => ['product-card', {
  selected: productQuantity(product.id) > 0,
  recommended: productBadges(product).includes('推荐')
}]
const productRibbon = (product) => (productBadges(product).includes('推荐') ? '推荐' : '')
const productById = (productId) => products.value.find((product) => Number(product.id) === Number(productId))
const shouldShowProductImage = (product) => Boolean(product.image_url) && !brokenImages.value.has(product.id)
const markImageBroken = (productId) => {
  const next = new Set(brokenImages.value)
  next.add(productId)
  brokenImages.value = next
}

const promotionTypeLabel = (type) => ({
  full_reduction: '满减',
  discount: '折扣',
  coupon: '优惠券'
}[type] || '活动')

const promotionText = (promotion) => {
  const threshold = Number(promotion.threshold || promotion.threshold_amount || 0)
  const discount = Number(promotion.discount || promotion.discount_amount || 0)
  const rate = Number(promotion.discount_rate || 0)
  if (promotion.type === 'discount' && rate > 0) {
    const text = (rate / 10).toFixed(rate % 10 ? 1 : 0)
    return `下单享 ${text} 折`
  }
  if (threshold > 0 && discount > 0) return `满 ${yuan(threshold)} 减 ${yuan(discount)}`
  if (discount > 0) return `立减 ${yuan(discount)}`
  return promotion.description || promotionTypeLabel(promotion.type)
}

const promotionGap = (promotion) => {
  const threshold = Number(promotion.threshold || promotion.threshold_amount || 0)
  return Math.max(threshold - totalAmount.value, 0)
}

const calcPromotionDiscount = (promotion) => {
  const threshold = Number(promotion.threshold || promotion.threshold_amount || 0)
  const discount = Number(promotion.discount || promotion.discount_amount || 0)
  const rate = Number(promotion.discount_rate || 0)
  if (threshold > 0 && totalAmount.value < threshold) return 0
  if (promotion.type === 'discount' && rate > 0) {
    return Math.max(0, totalAmount.value - Math.floor(totalAmount.value * rate / 100))
  }
  return Math.min(discount, totalAmount.value)
}

const productBadges = (product) => {
  const badges = []
  if (recommendCards.value.some((item) => item.product.id === product.id)) badges.push('推荐')
  if (activePromotions.value.some((promotion) => Number(product.price || 0) >= Number(promotion.threshold || promotion.threshold_amount || 0))) badges.push('可触发优惠')
  if (Number(product.sort || 100) <= 30) badges.push('商家优选')
  return badges
}

const load = async () => {
  loading.value = true
  error.value = ''
  productLoadError.value = false
  try {
    const storeRes = await fetchCustomerStore(route.params.storeId)
    store.value = storeRes.data.store || {}
    merchant.value = storeRes.data.merchant || {}
    promotions.value = storeRes.data.promotions || []
    groupBuyLinks.value = storeRes.data.group_buy_links || []
    await loadProducts()
    if (isTableOrder.value) await joinTableSession()
    await loadActiveOrder()
    if (shareCode.value) trackShareScan(shareCode.value).catch(() => {})
  } catch (err) {
    error.value = err.response?.data?.message || '门店信息不存在或暂不可访问'
  } finally {
    loading.value = false
  }
}

const loadProducts = async () => {
  productLoading.value = true
  try {
    const productsRes = await fetchCustomerStoreProducts(route.params.storeId)
    products.value = Array.isArray(productsRes.data.list) ? productsRes.data.list : []
    productLoadError.value = false
    pruneCartByProducts()
  } catch {
    productLoadError.value = true
  } finally {
    productLoading.value = false
  }
}

const loadActiveOrder = async () => {
  activeOrder.value = null
  if (!activeOrderNo.value) return
  try {
    const res = await fetchCustomerOrder(activeOrderNo.value)
    const nextOrder = res.data.order || null
    if (nextOrder && Number(nextOrder.store_id || nextOrder.store?.id || 0) === Number(route.params.storeId)) {
      activeOrder.value = nextOrder
    }
  } catch {
    activeOrder.value = null
  }
}

const clearActiveOrder = () => {
  activeOrder.value = null
  router.replace(`/customer/store/${route.params.storeId}`)
}

const saveRecentOrder = (orderNo) => {
  if (!orderNo) return
  recentOrderNo.value = orderNo
  localStorage.setItem(`customer_recent_order_${route.params.storeId}`, orderNo)
}

const openRecentOrder = () => {
  if (recentOrderNo.value) {
    router.push(customerOrderAccessPath(recentOrderNo.value, getCustomerOrderAccessToken(recentOrderNo.value)))
  }
}

const clearRecentOrder = () => {
  recentOrderNo.value = ''
  localStorage.removeItem(`customer_recent_order_${route.params.storeId}`)
}

const applyTableSnapshot = (snapshot) => {
  if (!snapshot) return
  tableSession.value = snapshot
  const next = {}
  ;(snapshot.items || []).forEach((item) => {
    next[item.product_id] = { ...item, quantity: Number(item.quantity || 0) }
  })
  cart.value = next
}

const joinTableSession = async () => {
  if (!isTableOrder.value) return
  try {
    const res = await joinCustomerTableSession({ store_id: Number(route.params.storeId), table_no: tableNo.value, table_code: tableCode.value })
    applyTableSnapshot(res.data.session)
    startTablePolling()
  } catch (err) {
    error.value = err.response?.data?.message || '桌码无效或桌位暂不可用，请重新扫码'
  }
}

const refreshTableSession = async () => {
  const session = tableSession.value
  if (!session?.id || !isTableOrder.value) return
  try {
    const res = await fetchCustomerTableSession(session.id, tableCode.value)
    const latest = res.data.session
    if (!tableSession.value || Number(latest.cart_version || 0) !== Number(tableSession.value.cart_version || 0) || latest.locked !== tableSession.value.locked) {
      applyTableSnapshot(latest)
    }
  } catch { /* intermittent polling failures are retried */ }
}

const startTablePolling = () => {
  stopTablePolling()
  tablePollTimer = window.setInterval(refreshTableSession, 2500)
}
const stopTablePolling = () => {
  if (tablePollTimer) window.clearInterval(tablePollTimer)
  tablePollTimer = null
}

const mutateTableCart = (productId, action, quantity = 0) => {
  if (!isTableOrder.value) return Promise.resolve()
  if (tableSessionLocked.value) { ElMessage.warning('本桌已有人发起结算，购物车已锁定'); return Promise.resolve() }
  const product = productById(productId)
  if (!product && action !== 'remove') return Promise.resolve()
  cartBusy.value = true
  cartMutationQueue = cartMutationQueue.then(async () => {
    const session = tableSession.value
    if (!session?.id) throw new Error('桌位会话尚未就绪，请稍后重试')
    const res = await updateCustomerTableCartItem(session.id, tableCode.value, { product_id: Number(productId), action, quantity })
    applyTableSnapshot(res.data.session)
  }).catch((err) => {
    ElMessage.error(err.response?.data?.message || '共享购物车更新失败，正在同步最新状态')
    return refreshTableSession()
  }).finally(() => { cartBusy.value = false })
  return cartMutationQueue
}

const pruneCartByProducts = () => {
  const productIds = new Set(activeProducts.value.map((product) => product.id))
  const next = {}
  Object.values(cart.value).forEach((item) => {
    if (!productIds.has(item.product_id)) return
    next[item.product_id] = { ...item, quantity: Math.max(Number(item.quantity || 1), 1) }
  })
  cart.value = next
}

const loadCoupons = async () => {
  if (!/^1\d{10}$/.test(customerPhone.value.trim())) {
    ElMessage.warning('手机号需要是 11 位数字')
    return
  }
  try {
    const res = await fetchCustomerStoreCoupons(route.params.storeId, { phone: customerPhone.value.trim() })
    coupons.value = res.data.list || []
    if (!coupons.value.length) {
      selectedCouponNo.value = ''
      ElMessage.info('当前手机号暂无可用奖励券')
      return
    }
    ElMessage.success(`已找到 ${coupons.value.length} 张可用奖励券`)
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '奖励券查询失败')
  }
}

const addToCart = (product) => {
  if (isTableOrder.value) {
    if (!canEditTableCart.value) return
    mutateTableCart(product.id, 'add', 1)
    return
  }
  const nextQuantity = (cart.value[product.id]?.quantity || 0) + 1
  cart.value = {
    ...cart.value,
    [product.id]: cart.value[product.id]
      ? { ...cart.value[product.id], quantity: nextQuantity }
      : { product_id: product.id, name: product.name, price: product.price, quantity: 1 }
  }
  ElMessage.success(`已加入购物车：${product.name}`)
}

const removeFromCart = (productId) => {
  if (isTableOrder.value) { mutateTableCart(productId, 'remove'); return }
  const next = { ...cart.value }
  delete next[productId]
  cart.value = next
}

const decreaseProduct = (productId) => {
  const item = cart.value[productId]
  if (!item) return
  if (isTableOrder.value) {
    mutateTableCart(productId, Number(item.quantity) <= 1 ? 'remove' : 'set', Math.max(Number(item.quantity) - 1, 0))
    return
  }
  const next = { ...cart.value }
  if (Number(item.quantity || 0) <= 1) delete next[productId]
  else next[productId] = { ...item, quantity: Number(item.quantity || 0) - 1 }
  cart.value = next
}

const setCartQuantity = (productId, quantity) => {
  const value = Number(quantity || 0)
  if (isTableOrder.value) { mutateTableCart(productId, value <= 0 ? 'remove' : 'set', value); return }
  const item = cart.value[productId]
  if (!item) return
  if (value <= 0) removeFromCart(productId)
  else cart.value = { ...cart.value, [productId]: { ...item, quantity: value } }
}

const clearCart = async () => {
  if (isTableOrder.value) {
    if (tableSessionLocked.value || cartBusy.value) return
    cartBusy.value = true
    try {
      const res = await clearCustomerTableCart(tableSession.value.id, tableCode.value)
      applyTableSnapshot(res.data.session)
    } catch (err) {
      ElMessage.error(err.response?.data?.message || '清空共享购物车失败')
      await refreshTableSession()
      return
    } finally { cartBusy.value = false }
  } else { cart.value = {} }
  selectedCouponNo.value = ''
  ElMessage.success('购物车已清空')
}

const syncCart = () => {
  if (isTableOrder.value) return refreshTableSession()
  const next = { ...cart.value }
  Object.values(next).forEach((item) => {
    if (item.quantity <= 0 || !productById(item.product_id)) delete next[item.product_id]
  })
  cart.value = next
}

const openCart = () => {
  cartVisible.value = true
}

const callStore = () => {
  if (contactPhone.value) window.location.href = `tel:${contactPhone.value}`
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
        router.push(customerOrderAccessPath(orderNo, getCustomerOrderAccessToken(orderNo)))
      }
    } catch {
      // 临时轮询失败不关闭二维码弹窗。
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

const secureCheckoutCredential = () => {
  const bytes = new Uint8Array(32)
  if (!window.crypto?.getRandomValues) throw new Error('当前浏览器无法安全创建订单凭据，请更新浏览器后重试')
  window.crypto.getRandomValues(bytes)
  return Array.from(bytes, (value) => value.toString(16).padStart(2, '0')).join('')
}
const checkoutAttemptFingerprint = async (payload) => {
  const stablePayload = {
    store_id: payload.store_id,
    customer_phone: payload.customer_phone,
    customer_note: payload.customer_note,
    share_code: payload.share_code,
    coupon_no: payload.coupon_no,
    payment_channel: payload.payment_channel,
    items: [...payload.items]
      .map(({ product_id, quantity }) => ({ product_id, quantity }))
      .sort((a, b) => a.product_id - b.product_id)
  }
  const serialized = JSON.stringify(stablePayload)
  if (!window.crypto?.subtle || typeof TextEncoder === 'undefined') return { value: serialized, persistent: false }
  const digest = await window.crypto.subtle.digest('SHA-256', new TextEncoder().encode(serialized))
  return {
    value: Array.from(new Uint8Array(digest), (value) => value.toString(16).padStart(2, '0')).join(''),
    persistent: true
  }
}
const getCustomerCheckoutAttempt = async (payload) => {
  const fingerprint = await checkoutAttemptFingerprint(payload)
  if (pendingCheckoutAttempt.value?.fingerprint === fingerprint.value) return pendingCheckoutAttempt.value

  const storageKey = `customer_checkout_attempt_${payload.store_id}`
  if (fingerprint.persistent) {
    try {
      const saved = JSON.parse(sessionStorage.getItem(storageKey) || 'null')
      if (saved?.store_id === payload.store_id && saved?.fingerprint === fingerprint.value && /^[a-f\d]{64}$/i.test(saved.request_key || '') && /^[a-f\d]{64}$/i.test(saved.access_token || '')) {
        pendingCheckoutAttempt.value = saved
        return saved
      }
    } catch {
      // Session storage may be unavailable in a restricted browser.
    }
  }

  const attempt = {
    store_id: payload.store_id,
    fingerprint: fingerprint.value,
    request_key: secureCheckoutCredential(),
    access_token: secureCheckoutCredential()
  }
  pendingCheckoutAttempt.value = attempt
  if (fingerprint.persistent) {
    try {
      sessionStorage.setItem(storageKey, JSON.stringify(attempt))
    } catch {
      // In-page retries still reuse this request if storage is unavailable.
    }
  }
  return attempt
}
const clearCustomerCheckoutAttempt = (attempt) => {
  if (pendingCheckoutAttempt.value?.request_key === attempt.request_key) pendingCheckoutAttempt.value = null
  const storageKey = `customer_checkout_attempt_${attempt.store_id}`
  try {
    const saved = JSON.parse(sessionStorage.getItem(storageKey) || 'null')
    if (saved?.request_key === attempt.request_key) sessionStorage.removeItem(storageKey)
  } catch {
    // Nothing to clear when session storage is unavailable.
  }
}

const readPendingCheckoutAttempt = (storeId) => {
  const inMemory = pendingCheckoutAttempt.value
  if (Number(inMemory?.store_id) === Number(storeId) &&
    /^[a-f\d]{64}$/i.test(inMemory?.request_key || '') &&
    /^[a-f\d]{64}$/i.test(inMemory?.access_token || '')) return inMemory
  try {
    const saved = JSON.parse(sessionStorage.getItem(`customer_checkout_attempt_${Number(storeId)}`) || 'null')
    if (Number(saved?.store_id) !== Number(storeId) ||
      !/^[a-f\d]{64}$/i.test(saved?.request_key || '') ||
      !/^[a-f\d]{64}$/i.test(saved?.access_token || '')) return null
    return saved
  } catch {
    return null
  }
}

const recoverPendingCheckout = async (strict = false) => {
  const attempt = readPendingCheckoutAttempt(route.params.storeId)
  if (!attempt) return false
  try {
    const res = await recoverCustomerStoreOrder({ request_key: attempt.request_key, access_token: attempt.access_token })
    const order = res.data.order
    const orderNo = order?.order_no
    const accessToken = res.data.access_token || attempt.access_token
    if (!orderNo) throw new Error('订单恢复响应缺少订单号')
    saveCustomerOrderAccessToken(orderNo, accessToken)
    saveRecentOrder(orderNo)
    clearCustomerCheckoutAttempt(attempt)
    cart.value = {}
    cartVisible.value = false
    ElMessage.success('已找回刚才提交的订单，可在状态页查看；不会自动付款')
    router.push(customerOrderAccessPath(orderNo, accessToken))
    return true
  } catch (err) {
    if (err.response?.status === 404) return false
    if (strict) throw new Error('订单结果暂未确认；请检查网络后重试，系统暂不发送新订单以避免重复')
    ElMessage.warning('订单结果暂未确认，请检查网络后重试；系统暂不提交新订单以避免重复')
    return false
  }
}

const appendAttemptFingerprint = async (orderNo, payload) => {
  const stablePayload = {
    order_no: orderNo,
    customer_note: String(payload.customer_note || '').trim(),
    items: [...payload.items]
      .map(({ product_id, quantity }) => ({ product_id: Number(product_id), quantity: Number(quantity) }))
      .sort((a, b) => a.product_id - b.product_id)
  }
  const serialized = JSON.stringify(stablePayload)
  if (!window.crypto?.subtle || typeof TextEncoder === 'undefined') return { value: serialized, persistent: false }
  const digest = await window.crypto.subtle.digest('SHA-256', new TextEncoder().encode(serialized))
  return {
    value: Array.from(new Uint8Array(digest), (value) => value.toString(16).padStart(2, '0')).join(''),
    persistent: true
  }
}
const getCustomerAppendAttempt = async (orderNo, payload) => {
  const fingerprint = await appendAttemptFingerprint(orderNo, payload)
  if (pendingAppendAttempt.value?.fingerprint === fingerprint.value) return pendingAppendAttempt.value

  const storageKey = `customer_append_attempt_${orderNo}`
  if (fingerprint.persistent) {
    try {
      const saved = JSON.parse(sessionStorage.getItem(storageKey) || 'null')
      if (saved?.order_no === orderNo && saved?.fingerprint === fingerprint.value && /^[a-f\d]{64}$/i.test(saved.request_key || '')) {
        pendingAppendAttempt.value = saved
        return saved
      }
    } catch {
      // Session storage may be unavailable in a restricted browser.
    }
  }

  const attempt = {
    order_no: orderNo,
    fingerprint: fingerprint.value,
    request_key: secureCheckoutCredential()
  }
  pendingAppendAttempt.value = attempt
  if (fingerprint.persistent) {
    try {
      sessionStorage.setItem(storageKey, JSON.stringify(attempt))
    } catch {
      // In-page retries still reuse this request if storage is unavailable.
    }
  }
  return attempt
}
const clearCustomerAppendAttempt = (attempt) => {
  if (pendingAppendAttempt.value?.request_key === attempt.request_key) pendingAppendAttempt.value = null
  const storageKey = `customer_append_attempt_${attempt.order_no}`
  try {
    const saved = JSON.parse(sessionStorage.getItem(storageKey) || 'null')
    if (saved?.request_key === attempt.request_key) sessionStorage.removeItem(storageKey)
  } catch {
    // Nothing to clear when session storage is unavailable.
  }
}

const readPendingAppendAttempt = (orderNo) => {
  const inMemory = pendingAppendAttempt.value
  if (inMemory?.order_no === orderNo && /^[a-f\d]{64}$/i.test(inMemory?.request_key || '')) return inMemory
  try {
    const saved = JSON.parse(sessionStorage.getItem(`customer_append_attempt_${orderNo}`) || 'null')
    if (saved?.order_no !== orderNo || !/^[a-f\d]{64}$/i.test(saved?.request_key || '')) return null
    return saved
  } catch {
    return null
  }
}

const recoverPendingAppend = async (strict = false) => {
  const orderNo = activeOrderNo.value
  const attempt = orderNo ? readPendingAppendAttempt(orderNo) : null
  if (!attempt) return false
  const accessToken = getCustomerOrderAccessToken(orderNo)
  if (!accessToken) {
    if (strict) throw new Error('无法恢复追加结果：缺少订单访问凭据；暂不发送新的追加请求')
    ElMessage.warning('无法确认刚才的追加结果，请检查订单访问凭据；不会自动重新追加')
    return false
  }
  try {
    const res = await recoverCustomerOrderAppend(orderNo, { request_key: attempt.request_key })
    const order = res.data.order
    const recoveredOrderNo = order?.order_no || orderNo
    clearCustomerAppendAttempt(attempt)
    saveRecentOrder(recoveredOrderNo)
    cart.value = {}
    cartVisible.value = false
    ElMessage.success('已找回刚才的追加结果；未重复追加')
    router.push(customerOrderAccessPath(recoveredOrderNo, accessToken))
    return true
  } catch (err) {
    if (err.response?.status === 404) return false
    if (strict) throw new Error('追加结果暂未确认；请检查网络后重试，系统暂不发送新的追加请求以避免重复')
    ElMessage.warning('追加结果暂未确认，请检查网络后重试；系统暂不发送新的追加请求以避免重复')
    return false
  }
}

const checkout = async () => {
  if (submitting.value) return
  submitting.value = true
  recoveryChecking.value = true
  try {
    if (await recoverPendingCheckout(true) || await recoverPendingAppend(true)) return
  } catch (err) {
    ElMessage.error(err.message || '订单结果暂未确认，请检查网络后重试')
    return
  } finally {
    submitting.value = false
    recoveryChecking.value = false
  }

  if (!cartItems.value.length) {
    ElMessage.warning('请先选择商品')
    return
  }
  syncCart()
  if (!cartItems.value.length) {
    ElMessage.warning('购物车商品已不可售，请重新选择')
    return
  }
  if (customerPhone.value && !/^1\d{10}$/.test(customerPhone.value.trim())) {
    ElMessage.warning('手机号需要是 11 位数字')
    return
  }
  if (selectedCoupon.value && !selectedCouponUsable.value) {
    ElMessage.warning('当前订单未达到奖励券使用门槛，请取消该券或继续加购')
    return
  }

  submitting.value = true
  let appendRequestInFlight = false
  try {
    if (appendableActiveOrder.value) {
      appendRequestInFlight = true
      const appendPayload = {
        customer_note: customerNote.value.trim(),
        items: cartItems.value.map((item) => ({
          product_id: item.product_id,
          quantity: item.quantity,
          price: item.price
        }))
      }
      const attempt = await getCustomerAppendAttempt(activeOrderNo.value, appendPayload)
      const res = await appendCustomerOrderItems(activeOrderNo.value, {
        ...appendPayload,
        request_key: attempt.request_key
      })
      clearCustomerAppendAttempt(attempt)
      const order = res.data.order
      ElMessage.success('已追加到当前订单')
      saveRecentOrder(order?.order_no)
      clearCart()
      cartVisible.value = false
      if (order?.order_no) router.push(customerOrderAccessPath(order.order_no, getCustomerOrderAccessToken(order.order_no)))
      return
    }

    const payload = {
      store_id: Number(route.params.storeId),
      customer_phone: customerPhone.value.trim(),
      customer_note: customerNote.value.trim(),
      payment_channel: 'alipay',
      pay_mode: payMode.value,
      share_code: shareCode.value,
      coupon_no: selectedCouponNo.value,
      items: cartItems.value.map((item) => ({
        product_id: item.product_id,
        quantity: item.quantity,
        price: item.price
      }))
    }
    const attempt = await getCustomerCheckoutAttempt(payload)
    const checkoutPayload = {
      ...payload,
      request_key: attempt.request_key,
      access_token: attempt.access_token
    }
    const res = isTableOrder.value
      ? await checkoutCustomerTableSession(tableSession.value.id, tableCode.value, {
        request_key: attempt.request_key,
        access_token: attempt.access_token,
        customer_phone: payload.customer_phone,
        customer_note: payload.customer_note,
        share_code: payload.share_code,
        coupon_no: payload.coupon_no
      })
      : await createCustomerStoreOrder(checkoutPayload)

    const order = res.data.order
    const payment = res.data.payment
    if (order?.order_no && res.data.access_token) {
      saveCustomerOrderAccessToken(order.order_no, res.data.access_token)
    }
    saveRecentOrder(order?.order_no)
    clearCustomerCheckoutAttempt(attempt)
    if (res.data.payment_required === false && order?.order_no) {
      ElMessage.success('订单已提交')
      cartVisible.value = false
      router.push(customerOrderAccessPath(order.order_no, getCustomerOrderAccessToken(order.order_no)))
      return
    }
    if (res.data.payment_error && order?.order_no) {
      ElMessage.warning('订单已创建，支付暂未拉起，可在状态页继续支付')
      cartVisible.value = false
      router.push(customerOrderAccessPath(order.order_no, getCustomerOrderAccessToken(order.order_no)))
      return
    }
    if (res.data.payment_config_error && order?.order_no) {
      ElMessage.warning(res.data.payment_config_error)
      cartVisible.value = false
      router.push(customerOrderAccessPath(order.order_no, getCustomerOrderAccessToken(order.order_no)))
      return
    }
    if (payment?.mode === 'page' && submitAlipayPage(payment.payment_url || payment.form)) return
    if (payment?.mode === 'qr' && payment?.qr_code) {
      qrImageUrl.value = buildQrImage(payment.qr_code)
      qrDialogVisible.value = true
      startPolling(order.order_no)
      return
    }
    ElMessage.success(res.data.payment_method === 'merchant_qr'
      ? '订单已创建。请在订单页扫描商家收款码，付款后等待门店核对到账。'
      : '订单已创建，等待支付结果')
    if (order?.order_no) router.push(customerOrderAccessPath(order.order_no, getCustomerOrderAccessToken(order.order_no)))
  } catch (err) {
    ElMessage.error(err.response?.data?.message || (appendRequestInFlight
      ? '追加结果暂未确认；可直接重试，系统会复用本次请求避免重复追加'
      : '下单结果暂未确认；可直接重试，系统会复用本次请求避免重复建单'))
  } finally {
    submitting.value = false
  }
}

const initializeCustomerStore = async () => {
  recoveryChecking.value = true
  try {
    if (await recoverPendingCheckout() || await recoverPendingAppend()) return
  } finally {
    recoveryChecking.value = false
  }
  await load()
}

onMounted(initializeCustomerStore)
onBeforeUnmount(() => { stopPolling(); stopTablePolling() })
</script>

<style scoped>
.customer-store {
  min-height: 100vh;
  padding: 14px 0 142px;
  color: #172033;
  background:
    radial-gradient(circle at 12% 0%, rgba(255, 184, 107, 0.42), transparent 30%),
    radial-gradient(circle at 96% 12%, rgba(92, 214, 255, 0.32), transparent 32%),
    linear-gradient(180deg, #fffaf1 0%, #eef7ff 100%);
}

.phone-shell {
  width: min(430px, calc(100vw - 24px));
  margin: 0 auto;
}

.page-panel,
.empty-state {
  padding: 24px;
  border-radius: 26px;
  background: rgba(255, 255, 255, 0.94);
  box-shadow: 0 18px 60px rgba(15, 23, 42, 0.12);
}

.empty-state {
  display: grid;
  gap: 12px;
  justify-items: start;
}

.store-hero {
  padding: 20px;
  border-radius: 28px;
  color: #fff;
  background:
    radial-gradient(circle at 86% 12%, rgba(255, 190, 92, 0.55), transparent 34%),
    linear-gradient(145deg, #111827 0%, #1d4ed8 100%);
  box-shadow: 0 22px 52px rgba(29, 78, 216, 0.24);
}

.hero-top,
.product-bottom,
.cart-bar,
.cart-item,
.price-box div,
.coupon-head,
.product-title-row {
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
  margin: 18px 0 8px;
  font-size: 30px;
  line-height: 1.12;
}

.store-hero p {
  margin: 0;
  color: rgba(255, 255, 255, 0.76);
}

.table-session-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: 12px;
  padding: 13px 15px;
  border: 1px solid #bbf7d0;
  border-radius: 18px;
  color: #14532d;
  background: linear-gradient(120deg, #f0fdf4, #ffffff);
}

.table-session-banner.locked {
  border-color: #fed7aa;
  color: #9a3412;
  background: linear-gradient(120deg, #fff7ed, #ffffff);
}

.table-session-banner span,
.table-session-banner strong,
.table-session-banner small {
  display: block;
}

.table-session-banner span { font-size: 11px; font-weight: 800; }
.table-session-banner strong { margin-top: 3px; font-size: 14px; }
.table-session-banner small { margin-top: 3px; color: #64748b; font-size: 11px; }

.store-unavailable-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: 12px;
  padding: 14px;
  border: 1px solid #fed7aa;
  border-radius: 20px;
  background: rgba(255, 247, 237, 0.96);
  box-shadow: 0 12px 34px rgba(154, 52, 18, 0.08);
}

.store-unavailable-card span,
.store-unavailable-card strong,
.store-unavailable-card p {
  display: block;
}

.store-unavailable-card span {
  color: #ea580c;
  font-size: 11px;
  font-weight: 900;
}

.store-unavailable-card strong {
  margin-top: 4px;
  color: #7c2d12;
  font-size: 16px;
  line-height: 1.35;
}

.store-unavailable-card p {
  margin: 5px 0 0;
  color: #9a3412;
  font-size: 13px;
  line-height: 1.55;
}

.hero-meta {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  margin-top: 16px;
}

.hero-meta div {
  min-width: 0;
  padding: 11px;
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.13);
}

.hero-meta small {
  display: block;
  color: rgba(255, 255, 255, 0.66);
}

.hero-meta strong {
  display: block;
  margin-top: 5px;
  line-height: 1.4;
  overflow-wrap: anywhere;
  white-space: normal;
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
  margin-top: 12px;
}

.group-buy-section {
  margin-top: 12px;
  padding: 15px;
  border: 1px solid #cbdff2;
  border-radius: 22px;
  background: linear-gradient(145deg, #f2f8ff 0%, #ffffff 72%);
  box-shadow: 0 12px 30px rgba(30, 64, 110, 0.08);
}

.group-buy-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.group-buy-heading span {
  color: #64809d;
  font-size: 10px;
  font-weight: 900;
  letter-spacing: 0.1em;
}

.group-buy-heading h2 {
  margin: 3px 0 0;
  color: #183653;
  font-size: 18px;
}

.group-buy-heading small {
  padding: 5px 8px;
  border-radius: 999px;
  color: #35638b;
  background: #e4f0fb;
  font-size: 10px;
  font-weight: 800;
}

.group-buy-intro,
.group-buy-disclaimer {
  color: #62758b;
  font-size: 11px;
  line-height: 1.55;
}

.group-buy-intro {
  margin: 7px 0 10px;
}

.group-buy-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 9px;
}

.group-buy-card {
  display: flex;
  min-width: 0;
  min-height: 102px;
  flex-direction: column;
  justify-content: space-between;
  padding: 11px;
  border: 1px solid #d6e3ef;
  border-radius: 16px;
  color: #183653;
  background: #fff;
  text-decoration: none;
  transition: transform 0.18s ease, box-shadow 0.18s ease;
}

.group-buy-card:active {
  transform: translateY(1px);
}

.group-buy-platform {
  color: #637d98;
  font-size: 10px;
  font-weight: 800;
}

.group-buy-card strong {
  margin: 8px 0;
  font-size: 13px;
  line-height: 1.35;
}

.group-buy-action {
  color: #2870b3;
  font-size: 11px;
  font-weight: 800;
}

.group-buy-card.channel-douyin {
  border-color: #d5dbe5;
  background: linear-gradient(145deg, #fff 0%, #f4f7fb 100%);
}

.group-buy-card.channel-meituan {
  border-color: #f5d6bd;
  background: linear-gradient(145deg, #fff 0%, #fff6ee 100%);
}

.group-buy-card.channel-meituan .group-buy-action {
  color: #c65a19;
}

.group-buy-disclaimer {
  margin: 10px 0 0;
  color: #77879a;
  font-size: 10px;
}

.active-order-strip {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: 12px;
  padding: 13px;
  border: 1px solid #bfdbfe;
  border-radius: 20px;
  background:
    radial-gradient(circle at 100% 0%, rgba(37, 99, 235, 0.12), transparent 34%),
    #eff6ff;
}

.active-order-strip.recent {
  border-color: #fed7aa;
  background:
    radial-gradient(circle at 100% 0%, rgba(249, 115, 22, 0.12), transparent 34%),
    #fff7ed;
}

.strip-actions {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  flex: 0 0 auto;
}

.active-order-strip span,
.active-order-strip strong,
.active-order-strip p {
  display: block;
}

.active-order-strip span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.06em;
}

.active-order-strip strong {
  margin-top: 5px;
  color: #0f2747;
  line-height: 1.35;
  word-break: break-all;
}

.active-order-strip p {
  margin: 5px 0 0;
  color: #64748b;
  font-size: 12px;
  line-height: 1.55;
}

.promo-card {
  min-width: 210px;
  padding: 12px 13px;
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

.promo-card strong {
  margin-top: 6px;
}

.promo-card span {
  margin-top: 4px;
  font-size: 12px;
}

.promo-badge {
  display: inline-flex;
  padding: 3px 8px;
  border-radius: 999px;
  color: #ea580c;
  background: #ffedd5;
  font-size: 11px;
  font-weight: 900;
}

.toolbar-card {
  position: sticky;
  top: 8px;
  z-index: 5;
  margin-top: 12px;
  padding: 11px;
  border-radius: 20px;
  background: rgba(255, 255, 255, 0.94);
  box-shadow: 0 12px 34px rgba(15, 23, 42, 0.08);
  backdrop-filter: blur(14px);
}

.input-icon {
  color: #94a3b8;
  font-weight: 900;
}

.category-scroll {
  margin-top: 10px;
}

.category-chip {
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border: 0;
  border-radius: 999px;
  padding: 8px 13px;
  color: #475569;
  background: #f1f5f9;
  font-weight: 800;
}

.category-chip b {
  min-width: 20px;
  height: 20px;
  border-radius: 999px;
  display: grid;
  place-items: center;
  padding: 0 6px;
  color: #64748b;
  background: #e2e8f0;
  font-size: 11px;
}

.category-chip.active {
  color: #fff;
  background: #0f172a;
}

.category-chip.active b {
  color: #0f172a;
  background: #fff;
}

.product-section {
  margin-top: 15px;
}

.section-title {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  margin-bottom: 11px;
}

.section-title.compact {
  margin-top: 14px;
}

.section-title span,
.section-title small {
  color: #64748b;
  font-size: 12px;
}

.section-title h2 {
  margin: 3px 0 0;
  font-size: 23px;
}

.recommend-scroll {
  display: flex;
  gap: 10px;
  overflow-x: auto;
  padding-bottom: 2px;
  scrollbar-width: none;
}

.recommend-scroll::-webkit-scrollbar {
  display: none;
}

.recommend-card {
  flex: 0 0 168px;
  min-height: 112px;
  padding: 13px;
  text-align: left;
  border: 1px solid #bfdbfe;
  border-radius: 18px;
  background:
    radial-gradient(circle at 100% 0%, rgba(37, 99, 235, 0.13), transparent 34%),
    #ffffff;
  box-shadow: 0 12px 28px rgba(15, 23, 42, 0.08);
}

.recommend-card span,
.recommend-card strong,
.recommend-card small {
  display: block;
}

.recommend-card span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
}

.recommend-card strong {
  display: -webkit-box;
  margin-top: 7px;
  color: #0f2747;
  font-size: 17px;
  line-height: 1.25;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.recommend-card small {
  margin-top: 7px;
  color: #64748b;
  line-height: 1.35;
}

.product-list {
  display: grid;
  gap: 11px;
}

.product-card {
  position: relative;
  display: grid;
  grid-template-columns: 92px 1fr;
  gap: 12px;
  padding: 11px;
  border-radius: 21px;
  background: rgba(255, 255, 255, 0.95);
  box-shadow: 0 12px 34px rgba(15, 23, 42, 0.09);
}

.product-card.selected {
  border: 1px solid rgba(37, 99, 235, 0.32);
  box-shadow: 0 16px 38px rgba(37, 99, 235, 0.14);
}

.product-card.recommended {
  border: 1px solid rgba(37, 99, 235, 0.22);
}


.product-image {
  position: relative;
  width: 92px;
  height: 92px;
  border-radius: 18px;
  overflow: hidden;
  display: grid;
  place-items: center;
  color: white;
  font-weight: 900;
  font-size: 21px;
  background: linear-gradient(135deg, #f97316, #f43f5e);
}

.product-image img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.product-ribbon {
  position: absolute;
  left: 6px;
  top: 6px;
  padding: 3px 7px;
  border-radius: 999px;
  color: #ffffff;
  background: rgba(15, 23, 42, 0.82);
  font-size: 11px;
  font-style: normal;
  font-weight: 900;
}

.product-count {
  position: absolute;
  right: 7px;
  bottom: 7px;
  min-width: 28px;
  height: 24px;
  border-radius: 999px;
  display: grid;
  place-items: center;
  padding: 0 7px;
  color: #fff;
  background: #2563eb;
  box-shadow: 0 8px 18px rgba(37, 99, 235, 0.34);
  font-size: 12px;
}

.product-info {
  min-width: 0;
}

.product-title-row h3 {
  margin: 2px 0 5px;
  font-size: 16px;
}

.product-info p {
  display: -webkit-box;
  min-height: 37px;
  margin: 0 0 8px;
  color: #64748b;
  font-size: 13px;
  line-height: 1.45;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.product-meta-line {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 8px;
}

.product-meta-line span {
  border-radius: 999px;
  padding: 3px 7px;
  color: #475569;
  background: #f1f5f9;
  font-size: 11px;
  font-weight: 800;
}


.product-bottom strong {
  color: #ea580c;
  font-size: 19px;
}

.product-stepper {
  display: inline-grid;
  grid-template-columns: 34px 34px 34px;
  align-items: center;
  overflow: hidden;
  border: 1px solid #bfdbfe;
  border-radius: 999px;
  background: #eff6ff;
}

.product-stepper button {
  width: 34px;
  height: 34px;
  border: 0;
  color: #2563eb;
  background: transparent;
  font-size: 18px;
  font-weight: 900;
}

.product-stepper button:disabled {
  color: #94a3b8;
}

.product-stepper span {
  text-align: center;
  color: #0f172a;
  font-weight: 900;
}

.empty-products {
  padding: 32px 22px;
  border-radius: 24px;
  text-align: center;
  background: rgba(255, 255, 255, 0.9);
  box-shadow: 0 12px 34px rgba(15, 23, 42, 0.08);
}

.empty-food {
  font-size: 38px;
}

.empty-products h3 {
  margin: 10px 0 6px;
}

.empty-products p,
.empty-state p {
  color: #64748b;
}

.cart-nudge {
  position: fixed;
  left: 50%;
  bottom: 82px;
  z-index: 19;
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  column-gap: 8px;
  row-gap: 2px;
  align-items: center;
  width: min(400px, calc(100vw - 24px));
  min-height: 50px;
  padding: 10px 14px;
  transform: translateX(-50%);
  border: 1px solid #bfdbfe;
  border-radius: 18px;
  text-align: left;
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 14px 34px rgba(15, 23, 42, 0.16);
}

.cart-nudge.good {
  border-color: #bbf7d0;
  background: rgba(240, 253, 244, 0.98);
}

.cart-nudge.warning {
  border-color: #fed7aa;
  background: rgba(255, 247, 237, 0.98);
}

.cart-nudge span {
  grid-row: span 2;
  padding: 4px 7px;
  border-radius: 999px;
  color: #2563eb;
  background: #eff6ff;
  font-size: 11px;
  font-weight: 900;
  white-space: nowrap;
}

.cart-nudge.good span {
  color: #047857;
  background: #dcfce7;
}

.cart-nudge.warning span {
  color: #b45309;
  background: #ffedd5;
}

.cart-nudge strong,
.cart-nudge small {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cart-nudge strong {
  color: #0f2747;
  font-size: 14px;
}

.cart-nudge small {
  color: #64748b;
  font-size: 12px;
}

.cart-bar {
  position: fixed;
  left: 50%;
  bottom: 14px;
  z-index: 20;
  width: min(400px, calc(100vw - 24px));
  transform: translateX(-50%);
  border: 0;
  border-radius: 22px;
  padding: 11px 13px;
  color: #fff;
  background: #0f172a;
  box-shadow: 0 20px 46px rgba(15, 23, 42, 0.34);
}

.cart-bar.has-items {
  background: linear-gradient(135deg, #0f2747, #2563eb 58%, #06b6d4);
}

.cart-icon-wrap {
  position: relative;
  width: 42px;
  height: 42px;
  border-radius: 16px;
  display: grid;
  place-items: center;
  background: rgba(255, 255, 255, 0.14);
}

.cart-symbol {
  font-size: 18px;
  font-weight: 900;
}

.cart-badge {
  position: absolute;
  right: -5px;
  top: -7px;
  min-width: 22px;
  height: 22px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  padding: 0 5px;
  background: #f97316;
  box-shadow: 0 8px 18px rgba(249, 115, 22, 0.38);
  font-size: 12px;
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
  display: grid;
  justify-items: end;
  gap: 1px;
}

.cart-pay small {
  color: rgba(255, 255, 255, 0.72);
  font-size: 11px;
}

.cart-pay strong {
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

.drawer-body {
  padding-bottom: 18px;
}

.empty-cart {
  display: grid;
  gap: 6px;
  padding: 24px;
  border-radius: 18px;
  color: #64748b;
  background: #f8fafc;
  text-align: center;
}

.empty-cart strong {
  color: #0f172a;
}

.empty-cart p {
  margin: 0;
  line-height: 1.6;
}

.cart-list {
  display: grid;
  gap: 10px;
}

.cart-list-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  color: #64748b;
  font-weight: 900;
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

.cart-item small {
  display: block;
  margin-top: 4px;
  color: #b45309;
  line-height: 1.4;
}

.cart-item small.danger {
  color: #dc2626;
  font-weight: 800;
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

.checkout-trust {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.checkout-trust div {
  padding: 11px;
  border: 1px solid #bbf7d0;
  border-radius: 16px;
  background: #ecfdf5;
}

.checkout-trust span,
.checkout-trust strong {
  display: block;
}

.checkout-trust span {
  color: #047857;
  font-size: 12px;
  font-weight: 900;
}

.checkout-trust strong {
  margin-top: 4px;
  color: #065f46;
  font-size: 13px;
  line-height: 1.35;
}

.coupon-panel {
  display: grid;
  gap: 8px;
}

.coupon-head {
  color: #64748b;
  font-size: 13px;
}

.field-tip,
.pay-confirm {
  margin: -2px 0 0;
  color: #64748b;
  font-size: 12px;
  line-height: 1.5;
}

.field-tip.danger {
  color: #dc2626;
}

.checkout-status-card {
  padding: 12px 14px;
  border: 1px solid #bfdbfe;
  border-radius: 16px;
  background: linear-gradient(135deg, #eff6ff, #ffffff);
}

.checkout-status-card.good {
  border-color: #bbf7d0;
  background: #ecfdf5;
}

.checkout-status-card.danger {
  border-color: #fecaca;
  background: #fef2f2;
}

.checkout-status-card div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.checkout-status-card span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
}

.checkout-status-card.good span {
  color: #047857;
}

.checkout-status-card.danger span {
  color: #dc2626;
}

.checkout-status-card strong {
  color: #0f2747;
  text-align: right;
}

.checkout-status-card p {
  margin: 6px 0 0;
  color: #64748b;
  font-size: 12px;
  line-height: 1.5;
}

.coupon-select,
.pay-mode {
  width: 100%;
}

.coupon-selected-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 11px 12px;
  border-radius: 16px;
  border: 1px solid #bfdbfe;
  background: linear-gradient(135deg, #eff6ff, #ffffff);
}

.coupon-selected-card span,
.coupon-selected-card strong {
  display: block;
}

.coupon-selected-card span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
}

.coupon-selected-card strong {
  margin-top: 3px;
  color: #0f172a;
  font-size: 13px;
}

.coupon-selected-card b {
  flex: 0 0 auto;
  color: #16a34a;
  font-size: 12px;
}

.coupon-selected-card b.muted {
  color: #f97316;
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

.pay-confirm {
  padding-top: 8px;
  border-top: 1px dashed #dbe6f3;
}

.deal-hint {
  padding: 12px 14px;
  border: 1px solid #fed7aa;
  border-radius: 16px;
  background: #fff7ed;
}

.deal-hint.good {
  border-color: #bbf7d0;
  background: #ecfdf5;
}

.deal-hint strong {
  display: block;
  color: #0f2747;
}

.deal-hint p {
  margin: 5px 0 0;
  color: #64748b;
  line-height: 1.5;
  font-size: 12px;
}

.payable strong {
  color: #16a34a;
  font-size: 20px;
}

.drawer-submit-bar {
  position: sticky;
  bottom: 0;
  z-index: 2;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin: 4px -2px 0;
  padding: 12px;
  border-radius: 20px;
  background: rgba(15, 23, 42, 0.96);
  box-shadow: 0 -14px 34px rgba(15, 23, 42, 0.2);
}

.drawer-submit-bar span,
.drawer-submit-bar small,
.drawer-submit-bar strong {
  display: block;
}

.drawer-submit-bar span,
.drawer-submit-bar small {
  color: rgba(255, 255, 255, 0.7);
  font-size: 12px;
}

.drawer-submit-bar strong {
  margin-top: 2px;
  color: #fff;
  font-size: 21px;
}

.drawer-submit-bar :deep(.el-button) {
  flex: 0 0 auto;
  min-width: 132px;
  border-radius: 14px;
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

  .phone-shell:not(.page-panel):not(.empty-state) {
    padding: 12px;
    border-radius: 34px;
    background: rgba(255, 255, 255, 0.36);
    box-shadow: 0 30px 90px rgba(15, 23, 42, 0.14);
  }
}

@media (max-width: 380px) {
  .checkout-trust {
    grid-template-columns: 1fr;
  }

  .active-order-strip {
    align-items: flex-start;
    flex-direction: column;
  }

  .strip-actions {
    flex-direction: row;
    align-items: center;
  }
}
</style>
