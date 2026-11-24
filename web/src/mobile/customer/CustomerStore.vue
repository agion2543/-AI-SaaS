<template>
  <div class="customer-store">
    <!-- 顶部导航 -->
    <van-nav-bar
      :title="storeInfo.name || '门店'"
      left-arrow
      fixed
      placeholder
      @click-left="onBack"
    >
      <template #right>
        <van-icon name="search" size="20" @click="showSearch = true" />
      </template>
    </van-nav-bar>

    <!-- 加载状态 -->
    <div v-if="loading" class="loading-container">
      <van-loading type="spinner" size="40px">加载中...</van-loading>
    </div>

    <!-- 错误状态 -->
    <div v-else-if="error" class="error-container">
      <van-empty description="门店加载失败" />
      <van-button type="primary" size="small" @click="loadStoreData">重试</van-button>
    </div>

    <!-- 门店内容 -->
    <template v-else>
      <van-notice-bar v-if="isTableMode" :type="tableSession?.locked ? 'warning' : 'primary'" :scrollable="false">
        {{ `桌号：${tableSession?.table_label || `${tableNo}号桌`}` }}{{ tableSession?.locked ? ' · 已有顾客结算，购物车已锁定' : ' · 共享点单' }}
      </van-notice-bar>
      <!-- 门店公告 -->
      <van-notice-bar
        v-if="storeInfo.notice"
        :text="storeInfo.notice"
        left-icon="volume-o"
        scrollable
      />

      <!-- 分类 + 商品列表 -->
      <div class="store-content">
        <!-- 左侧分类 -->
        <van-sidebar v-model="activeCategory" class="category-sidebar">
          <van-sidebar-item
            v-for="cat in categories"
            :key="cat.id"
            :title="cat.name"
            :badge="getCategoryCount(cat.id)"
          />
        </van-sidebar>

        <!-- 右侧商品 -->
        <div class="product-list">
          <van-cell-group inset>
            <div
              v-for="product in filteredProducts"
              :key="product.id"
              class="product-item"
            >
              <van-card
                :price="formatPrice(getProductUnitPrice(product, cart[product.id] || getMinimumOrderQuantity(product)))"
                :title="product.name"
                :desc="productDescription(product)"
                :thumb="product.image_url || product.image || defaultProductImg"
                @click="showProductDetail(product)"
              >
                <template #tags>
                  <van-tag v-if="getProductMinimum(product) > 1" type="warning">{{ getProductMinimum(product) }} 起订</van-tag>
                  <van-tag v-if="getProductPriceTiers(product).length" type="primary">阶梯价</van-tag>
                  <van-tag v-if="product.discount" type="success">立减{{ product.discount }}元</van-tag>
                </template>
                <template #num>
                  <van-stepper
                    v-model="cart[product.id]"
                    :min="0"
                    @change="(val) => updateCart(product, val)"
                    @click.stop
                  />
                </template>
              </van-card>
            </div>
          </van-cell-group>

          <van-empty
            v-if="filteredProducts.length === 0"
            :description="searchKeyword ? '未找到商品' : '暂无商品'"
          />
        </div>
      </div>

      <!-- 底部购物车栏 -->
      <van-submit-bar
        v-if="totalCount > 0"
        :price="totalPrice"
        label="合计"
        button-text="去结算"
        :disabled="Boolean(tableSession?.locked)"
        suffix-label=""
        @submit="goToCart"
      >
        <van-badge
          :content="totalCount"
          :max="99"
          class="cart-badge"
        >
          <van-icon name="shopping-cart-o" size="24" @click="showCartPopup = true" />
        </van-badge>
      </van-submit-bar>

      <!-- 空购物车提示 -->
      <van-submit-bar v-else class="empty-cart-bar">
        <van-icon name="shopping-cart-o" size="20" class="empty-cart-icon" />
        <span class="empty-cart-text">购物车是空的</span>
      </van-submit-bar>
    </template>

    <!-- 搜索弹窗 -->
    <van-popup v-model:show="showSearch" position="top" style="height: 100%; padding-top: 50px; background: #f7f8fa;">
      <div class="search-container">
        <van-search
          v-model="searchKeyword"
          placeholder="搜索商品名称"
          show-action
          autofocus
          @search="onSearch"
        >
          <template #action>
            <div @click="onSearch">搜索</div>
          </template>
        </van-search>

        <div v-if="searchResults.length > 0" class="search-results">
          <van-cell
            v-for="p in searchResults"
            :key="p.id"
            :title="p.name"
            :label="`${formatPrice(getProductUnitPrice(p, getMinimumOrderQuantity(p)))}${describeProductPricing(p) ? ` · ${describeProductPricing(p)}` : ''}`"
            :thumb="p.image_url || p.image || defaultProductImg"
            is-link
            @click="showProductDetail(p)"
          />
        </div>
        <van-empty v-else-if="searchKeyword && searched" description="未找到相关商品" />
      </div>
    </van-popup>

    <!-- 商品详情弹窗 -->
    <van-popup v-model:show="showProductPopup" position="bottom" round style="max-height: 70%;">
      <div v-if="currentProduct" class="product-detail">
        <van-image
          :src="currentProduct.image_url || currentProduct.image || defaultProductImg"
          width="100%"
          height="200"
          fit="cover"
        />
        <div class="product-info">
          <h3>{{ currentProduct.name }}</h3>
          <p class="product-desc">{{ currentProduct.description || '暂无描述' }}</p>
          <div class="product-meta">
            <span class="product-price">¥{{ formatPrice(getProductUnitPrice(currentProduct, addCount)) }}</span>
          </div>
          <p v-if="describeProductPricing(currentProduct)" class="product-pricing">{{ describeProductPricing(currentProduct) }}</p>
        </div>
        <div class="product-actions">
          <van-stepper v-model="addCount" :min="getMinimumOrderQuantity(currentProduct)" />
          <van-button
            type="primary"
            size="small"
            @click="addToCartFromDetail"
          >
            加入购物车
          </van-button>
        </div>
      </div>
    </van-popup>

    <!-- 购物车详情弹窗 -->
    <van-popup v-model:show="showCartPopup" position="bottom" round style="max-height: 60%;">
      <div class="cart-popup">
        <div class="cart-header">
          <span>购物车</span>
          <van-button size="small" text="清空" type="danger" @click="clearCart">清空</van-button>
        </div>
        <van-card
          v-for="item in cartItems"
          :key="item.id"
          :price="formatPrice(getProductUnitPrice(item, item.count))"
          :title="item.name"
          :desc="productDescription(item)"
          :thumb="item.image_url || item.image || defaultProductImg"
          :num="cart[item.id]"
        >
          <template #footer>
            <van-button size="mini" @click="removeFromCart(item)">删除</van-button>
          </template>
        </van-card>
        <van-empty v-if="cartItems.length === 0" description="购物车是空的" />
      </div>
    </van-popup>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { showToast, showSuccessToast } from 'vant'
import { fetchCustomerStore, fetchCustomerStoreProducts, joinCustomerTableSession, fetchCustomerTableSession, updateCustomerTableCartItem, clearCustomerTableCart } from '@/api/modules'
import { customerResponseList } from '@/utils/mobileCustomerOrder'
import { describeProductPricing, getMinimumOrderQuantity, getProductPriceTiers, getProductUnitPrice } from '@/utils/productPricing'

const route = useRoute()
const router = useRouter()
const storeId = route.params.storeId
const tableNo = Number(route.query.table_no || 0)
const tableCode = String(route.query.table_code || '')
const isTableMode = Number.isInteger(tableNo) && tableNo > 0 && /^[a-zA-Z0-9_-]{32,64}$/.test(tableCode)
const tableSession = ref(null)
let tablePollTimer = null
let cartWriteChain = Promise.resolve()

// 状态
const loading = ref(true)
const error = ref(false)
const storeInfo = ref({})
const products = ref([])
const categories = ref([])
const activeCategory = ref(0)
const cart = ref({})
const showSearch = ref(false)
const searchKeyword = ref('')
const searchResults = ref([])
const searched = ref(false)
const showProductPopup = ref(false)
const currentProduct = ref(null)
const addCount = ref(1)
const showCartPopup = ref(false)

const defaultProductImg = 'https://via.placeholder.com/100x100?text=商品'

// 计算属性
const filteredProducts = computed(() => {
  if (searchKeyword.value) {
    return products.value
  }
  const catId = categories.value[activeCategory.value]?.id
  if (!catId) return products.value
  return products.value.filter(p => p.category_id === catId)
})

const totalCount = computed(() => {
  return Object.entries(cart.value)
    .reduce((sum, [_, count]) => sum + (count || 0), 0)
})

const totalPrice = computed(() => {
  return products.value.reduce((sum, p) => {
    const count = cart.value[p.id] || 0
    return sum + count * getProductUnitPrice(p, count || getMinimumOrderQuantity(p))
  }, 0)
})

const cartItems = computed(() => {
  return products.value
    .filter(p => cart.value[p.id] > 0)
    .map(p => ({ ...p, count: cart.value[p.id] }))
})

// 方法
function formatPrice(price) {
  return (Number(price || 0) / 100).toFixed(2)
}


function getProductMinimum(product) {
  return getMinimumOrderQuantity(product)
}

function productDescription(product) {
  return [product?.description, describeProductPricing(product)].filter(Boolean).join(' · ')
}

function getCategoryCount(catId) {
  return products.value.filter(p => p.category_id === catId).length
}

function onBack() {
  if (window.history.length > 1) {
    router.back()
  } else {
    router.push('/')
  }
}

function showProductDetail(product) {
  currentProduct.value = product
  addCount.value = getMinimumOrderQuantity(product)
  showProductPopup.value = true
}

function addToCartFromDetail() {
  const product = currentProduct.value
  const newCount = (cart.value[product.id] || 0) + addCount.value
  showProductPopup.value = false
  updateCart(product, newCount)
}

function applyTableSnapshot(snapshot) {
  if (!snapshot) return
  tableSession.value = snapshot
  const next = {}
  for (const item of snapshot.items || []) next[item.product_id] = item.quantity
  cart.value = next
}
async function syncTableSession() {
  if (!isTableMode || !tableSession.value?.id) return
  try {
    const res = await fetchCustomerTableSession(tableSession.value.id, tableCode)
    applyTableSnapshot(res.data?.session)
  } catch (e) { if (e.response?.status === 409) tableSession.value = { ...tableSession.value, locked: true } }
}
function updateCart(product, value) {
  if (value > 0) {
    const minimum = getMinimumOrderQuantity(product)
    if (value < minimum) {
      cart.value[product.id] = minimum
      showToast(`最低起订量为 ${minimum}${product.sales_unit || '件'}`)
    } else cart.value[product.id] = value
  } else delete cart.value[product.id]
  if (isTableMode && tableSession.value) {
    if (tableSession.value.locked) { showToast('该桌已结算，购物车已锁定'); syncTableSession(); return }
    const quantity = cart.value[product.id] || 0
    cartWriteChain = cartWriteChain.then(async () => {
      const res = await updateCustomerTableCartItem(tableSession.value.id, tableCode, { product_id: product.id, action: 'set', quantity })
      applyTableSnapshot(res.data?.session)
    }).catch((e) => { showToast(e.response?.data?.message || '共享购物车更新失败'); syncTableSession() })
  } else saveCart()
}
function removeFromCart(product) {
  if (isTableMode) { updateCart(product, 0); return }
  delete cart.value[product.id]
  saveCart()
}
async function clearCart() {
  if (isTableMode && tableSession.value) {
    if (tableSession.value.locked) { showToast('该桌已结算，购物车已锁定'); return }
    try { const res = await clearCustomerTableCart(tableSession.value.id, tableCode); applyTableSnapshot(res.data?.session) }
    catch (e) { showToast(e.response?.data?.message || '清空共享购物车失败') }
  } else { cart.value = {}; saveCart() }
  showCartPopup.value = false
}

function saveCart() {
  localStorage.setItem(`cart_${storeId}`, JSON.stringify(cart.value))
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

function goToCart() {
  // 保存购物车到本地
  if (tableSession.value?.locked) { showToast('该桌已结算，不能重复结算'); return }
  if (!isTableMode) saveCart()
  router.push({ path: '/m/order/confirm', query: { storeId, ...(isTableMode ? { table_no: tableNo, table_code: tableCode, session_id: tableSession.value?.id } : {}) } })
}

function onSearch() {
  if (!searchKeyword.value.trim()) {
    searchResults.value = []
    return
  }
  searchResults.value = products.value.filter(p =>
    p.name.includes(searchKeyword.value)
  )
  searched.value = true
}

// 加载数据
async function loadStoreData() {
  loading.value = true
  error.value = false
  try {
    const [storeRes, productsRes] = await Promise.all([
      fetchCustomerStore(storeId),
      fetchCustomerStoreProducts(storeId)
    ])

    storeInfo.value = storeRes.data?.store || storeRes.data || {}

    // 处理商品数据，添加分类信息
    const prods = customerResponseList(productsRes)
    products.value = prods.map(p => ({ ...p, count: 0 }))

    // 从商品中提取分类
    const catMap = new Map()
    prods.forEach(p => {
      if (p.category_id && p.category_name) {
        catMap.set(p.category_id, { id: p.category_id, name: p.category_name })
      }
    })
    categories.value = Array.from(catMap.values())

    // 如果没有分类，添加"全部"分类
    if (categories.value.length === 0) {
      categories.value = [{ id: 0, name: '全部商品' }]
    }

    if (isTableMode) {
      const joined = await joinCustomerTableSession({ store_id: Number(storeId), table_no: tableNo, table_code: tableCode })
      applyTableSnapshot(joined.data?.session)
      tablePollTimer = window.setInterval(syncTableSession, 3000)
    } else loadCart()
  } catch (e) {
    console.error('加载门店数据失败:', e)
    error.value = true
    showToast('加载失败，请重试')
  } finally {
    loading.value = false
  }
}

onMounted(() => { loadStoreData() })
onUnmounted(() => { if (tablePollTimer) window.clearInterval(tablePollTimer) })
</script>

<style scoped>
.customer-store {
  min-height: 100vh;
  background: #f7f8fa;
  padding-bottom: 60px;
}

.loading-container,
.error-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 50vh;
  gap: 16px;
}

.store-content {
  display: flex;
  margin-top: 8px;
}

.category-sidebar {
  width: 80px;
  background: #f7f8fa;
  flex-shrink: 0;
}

.product-list {
  flex: 1;
  padding: 0 8px 8px;
  overflow-y: auto;
  height: calc(100vh - 130px);
}

.product-item {
  margin-bottom: 8px;
}

.van-card {
  border-radius: 8px;
}

.cart-badge {
  margin-right: 12px;
}

.empty-cart-bar {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  height: 48px;
  background: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  border-top: 1px solid #eee;
  z-index: 100;
}

.empty-cart-icon {
  color: #999;
}

.empty-cart-text {
  color: #999;
  font-size: 14px;
}

.search-container {
  padding: 16px;
}

.search-results {
  margin-top: 16px;
}

.product-detail {
  padding-bottom: 16px;
}

.product-info {
  padding: 16px;
}

.product-info h3 {
  margin: 0 0 8px;
  font-size: 18px;
}

.product-desc {
  color: #666;
  font-size: 14px;
  margin-bottom: 12px;
}

.product-meta {
  display: flex;
  align-items: center;
  gap: 12px;
}

.product-price {
  font-size: 20px;
  color: #ee0a24;
  font-weight: bold;
}


.product-pricing {
  margin: 8px 16px 0;
  color: #806217;
  font-size: 12px;
  line-height: 1.5;
}

.product-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px;
  border-top: 1px solid #eee;
}

.cart-popup {
  padding: 16px;
}

.cart-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
  font-weight: bold;
}
</style>
