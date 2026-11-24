<template>
  <div class="merchant-products">
    <!-- 顶部导航 -->
    <van-nav-bar
      title="商品管理"
      left-arrow
      fixed
      placeholder
      @click-left="router.back()"
    >
      <template #right>
        <van-button v-if="canWrite" size="small" type="primary" @click="showAddProduct">添加商品</van-button>
      </template>
    </van-nav-bar>

    <div v-if="!canWrite" class="readonly-banner" role="status">
      订阅已到期：商品仅可只读查看，新增、编辑、上下架和删除已关闭。
      <van-button size="small" type="warning" plain @click="router.push('/m/merchant/subscription')">查看续费</van-button>
    </div>

    <!-- 门店选择 -->
    <van-tabs v-if="stores.length > 1" v-model:active="currentStoreIndex" sticky @change="onStoreChange">
      <van-tab v-for="store in stores" :key="store.id" :title="store.name" />
    </van-tabs>

    <!-- 筛选和搜索 -->
    <div class="filter-bar">
      <van-search
        v-model="keyword"
        placeholder="搜索商品"
        @update:model-value="scheduleProductSearch"
        @search="searchProducts"
      />
    </div>

    <p v-if="canWrite && products.length > 0" class="product-swipe-hint">
      左滑商品可编辑、上架/下架或删除
    </p>

    <!-- 商品列表 -->
    <van-list
      v-model:loading="loading"
      v-model:error="productLoadError"
      :finished="finished"
      :disabled="searchPending"
      finished-text="没有更多了"
      error-text="加载失败，点击重试"
      @load="loadProducts"
    >
      <div v-if="products.length === 0 && !loading" class="empty-products">
        <van-empty :description="productLoadError ? '商品列表加载失败' : '暂无商品'">
          <van-button v-if="!productLoadError && canWrite" type="primary" size="small" @click="showAddProduct">添加商品</van-button>
        </van-empty>
      </div>

      <van-cell-group inset>
        <div v-for="product in products" :key="product.id" class="product-item">
          <van-swipe-cell :disabled="!canWrite">
            <van-card
              :price="formatPrice(product.price)"
              :title="product.name"
              :desc="product.description || '暂无描述'"
              :thumb="product.image_url || defaultProductImg"
              class="product-card"
            >
              <template #tags>
                <van-tag v-if="product.status !== 'active'" type="warning">已下架</van-tag>
              </template>
              <template #footer>
                <div class="product-actions">
                  <van-tag :type="product.status === 'active' ? 'success' : 'default'">
                    {{ product.status === 'active' ? '上架' : '下架' }}
                  </van-tag>
                </div>
              </template>
            </van-card>

            <template v-if="canWrite" #right>
              <van-button
                square
                type="primary"
                text="编辑"
                @click="editProduct(product)"
              />
              <van-button
                square
                :type="product.status === 'active' ? 'warning' : 'success'"
                :text="product.status === 'active' ? '下架' : '上架'"
                @click="toggleProductStatus(product)"
              />
              <van-button
                square
                type="danger"
                text="删除"
                @click="deleteProduct(product)"
              />
            </template>
          </van-swipe-cell>
        </div>
      </van-cell-group>
    </van-list>

    <!-- 添加/编辑商品弹窗 -->
    <van-popup
      v-if="canWrite"
      v-model:show="showProductPopup"
      position="bottom"
      round
      style="max-height: 90%;"
    >
      <div class="product-form">
        <div class="form-header">
          <span>{{ isEditing ? '编辑商品' : '添加商品' }}</span>
          <van-icon name="cross" @click="showProductPopup = false" />
        </div>

        <van-form ref="productFormRef" @submit="saveProduct">
          <van-cell-group inset>
            <van-field
              v-model="productForm.name"
              name="name"
              label="商品名称"
              placeholder="请输入商品名称"
              :rules="[{ required: true, message: '请输入商品名称' }]"
            />
            <van-field
              v-model="productForm.price"
              name="price"
              type="number"
              label="价格（元）"
              placeholder="请输入价格（元）"
              :rules="[
                { required: true, message: '请输入价格' },
                { validator: isValidMerchantProductPrice, message: '价格必须为有效的非负金额' }
              ]"
            />
            <van-field
              label="商品图片"
              placeholder="请输入图片URL"
            >
              <template #input>
                <van-uploader
                  v-model="productForm.imageFiles"
                  :max-count="1"
                  :after-read="afterRead"
                >
                  <van-image
                    v-if="productForm.image_url"
                    :src="productForm.image_url"
                    width="80"
                    height="80"
                  />
                  <van-button v-else size="small" type="primary">上传图片</van-button>
                </van-uploader>
              </template>
            </van-field>
            <van-field
              v-model="productForm.specification"
              name="specification"
              label="口味/规格（选填）"
              placeholder="如：微辣、少辣、500g/袋"
              maxlength="80"
            />
            <van-field
              v-model="productForm.description"
              name="description"
              type="textarea"
              label="描述"
              placeholder="请输入商品描述"
              rows="2"
            />
          </van-cell-group>

          <section class="advanced-settings">
            <button
              type="button"
              class="advanced-settings-toggle"
              :aria-expanded="advancedSettingsExpanded"
              aria-controls="merchant-h5-product-advanced-fields"
              @click="advancedSettingsExpanded = !advancedSettingsExpanded"
            >
              <span class="advanced-settings-title">数量定价设置（高级）</span>
              <small class="advanced-settings-summary">{{ advancedSettingsSummary }}</small>
              <span class="advanced-settings-action">{{ advancedSettingsExpanded ? '收起' : '展开' }}</span>
            </button>
            <div id="merchant-h5-product-advanced-fields" v-show="advancedSettingsExpanded" class="advanced-settings-content">
              <van-cell-group inset>
              </van-cell-group>
              <div class="advanced-preserved-values">
                <div><strong>最低购买量：</strong>{{ productForm.minimum_order_quantity ?? '接口未返回，更新时不覆盖' }}</div>
                <div><strong>数量阶梯价：</strong>{{ Array.isArray(productForm.price_tiers) ? `${productForm.price_tiers.length} 档` : '接口未返回，更新时不覆盖' }}</div>
                <p>当前 H5 没有起订量和阶梯价编辑控件；编辑保存会保留接口返回的原值。</p>
              </div>
            </div>
          </section>

          <div class="form-actions">
            <van-button block type="primary" native-type="submit" :loading="saving || uploadingImage" :disabled="uploadingImage">
              保存
            </van-button>
          </div>
        </van-form>
      </div>
    </van-popup>
  </div>
</template>

<script setup>
import { computed, ref, onMounted, onBeforeUnmount, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { showToast, showSuccessToast, showFailToast, showDialog } from 'vant'
import {
  fetchMerchantStores,
  fetchMerchantStoreProducts,
  createMerchantStoreProduct,
  updateMerchantStoreProduct,
  uploadMerchantProductImage,
  deleteMerchantStoreProduct
} from '@/api/modules'
import { cachedMobileMerchantSubscriptionState, mobileSubscriptionActions } from '@/utils/mobileMerchantSubscription'
import {
  buildMerchantProductPayload,
  createMerchantProductForm,
  extractMerchantProductList,
  isValidMerchantProductPrice
} from '@/utils/merchantProductContract'
import { createDebouncedAction, createRequestVersionGuard } from '@/utils/merchantProductSearch'
import { resolveMerchantProductStoreIndex } from '@/utils/merchantProductStoreSelection'

const router = useRouter()
const route = useRoute()
const canWrite = mobileSubscriptionActions(cachedMobileMerchantSubscriptionState()).canWriteBusiness

const loading = ref(false)
const productLoadError = ref(false)
const searchPending = ref(false)
const finished = ref(false)
const stores = ref([])
const currentStoreIndex = ref(0)
const products = ref([])
const keyword = ref('')
const page = ref(1)
const pageSize = 20
const productRequestVersion = createRequestVersionGuard()
const debouncedSearch = createDebouncedAction(() => refreshProducts(), 300)
let activeProductsRequest = null

const showProductPopup = ref(false)
const isEditing = ref(false)
const saving = ref(false)
const uploadingImage = ref(false)
const currentProductId = ref(null)
const productFormRef = ref(null)
const advancedSettingsExpanded = ref(false)
const productForm = ref(createMerchantProductForm())

const defaultProductImg = 'https://via.placeholder.com/100x100?text=商品'

function formatPrice(price) {
  return (Number(price || 0) / 100).toFixed(2)
}

const advancedSettingsSummary = computed(() => {
  const minimum = productForm.value.minimum_order_quantity ?? '原值保留'
  const tiers = Array.isArray(productForm.value.price_tiers)
    ? `${productForm.value.price_tiers.length} 档阶梯价`
    : '阶梯价原值保留'
  return `起订量 ${minimum} · ${tiers}`
})

function onStoreChange() {
  refreshProducts()
}

function searchProducts() {
  debouncedSearch.flush()
}

function scheduleProductSearch() {
  searchPending.value = true
  productRequestVersion.next()
  debouncedSearch.schedule()
}

async function loadStores() {
  try {
    const res = await fetchMerchantStores()
    stores.value = Array.isArray(res.data?.list) ? res.data.list : (Array.isArray(res.data) ? res.data : [])
    if (stores.value.length > 0) {
      currentStoreIndex.value = resolveMerchantProductStoreIndex(stores.value, route.query.storeId)
      loadProducts()
    }
  } catch (e) {
    console.error('加载门店失败:', e)
  }
}

async function loadProducts() {
  if (searchPending.value) return
  if (stores.value.length === 0) return

  const storeId = stores.value[currentStoreIndex.value]?.id
  if (!storeId) return

  const version = productRequestVersion.current()
  if (activeProductsRequest?.version === version) return activeProductsRequest.promise

  const requestPage = page.value
  const request = { version, promise: null }
  activeProductsRequest = request
  loading.value = true
  productLoadError.value = false
  request.promise = (async () => {
    try {
      const res = await fetchMerchantStoreProducts(storeId, {
        keyword: keyword.value,
        page: requestPage,
        page_size: pageSize
      })
      if (!productRequestVersion.isCurrent(version)) return

      const data = extractMerchantProductList(res)
      if (requestPage === 1) {
        products.value = data
      } else {
        products.value.push(...data)
      }

      if (data.length < pageSize) {
        finished.value = true
      } else {
        page.value = requestPage + 1
      }
    } catch (e) {
      if (productRequestVersion.isCurrent(version)) {
        console.error('加载商品失败:', e)
        productLoadError.value = true
        showToast('加载失败')
      }
    } finally {
      if (activeProductsRequest === request) {
        activeProductsRequest = null
        loading.value = false
      }
    }
  })()
  return request.promise
}

function refreshProducts() {
  debouncedSearch.cancel()
  searchPending.value = false
  productRequestVersion.next()
  page.value = 1
  finished.value = false
  products.value = []
  loading.value = false
  loadProducts()
}

function showAddProduct() {
  if (stores.value.length === 0) {
    showToast('请先添加门店')
    router.push('/m/merchant/stores')
    return
  }
  isEditing.value = false
  currentProductId.value = null
  advancedSettingsExpanded.value = false
  productForm.value = createMerchantProductForm()
  showProductPopup.value = true
}

function editProduct(product) {
  isEditing.value = true
  currentProductId.value = product.id
  advancedSettingsExpanded.value = false
  productForm.value = createMerchantProductForm(product)
  showProductPopup.value = true
}

async function toggleProductStatus(product) {
  try {
    const nextStatus = product.status === 'active' ? 'inactive' : 'active'
    const payload = buildMerchantProductPayload(
      createMerchantProductForm(product),
      product.store_id || stores.value[currentStoreIndex.value]?.id,
      { status: nextStatus }
    )
    await updateMerchantStoreProduct(product.id, payload)
    showSuccessToast(nextStatus === 'inactive' ? '已下架' : '已上架')
    refreshProducts()
  } catch (e) {
    showFailToast(e.response?.data?.message || '操作失败')
  }
}

function deleteProduct(product) {
  showDialog({
    title: '确认删除',
    message: `确定要删除商品 "${product.name}" 吗？`,
    showCancelButton: true,
  }).then(async () => {
    try {
      await deleteMerchantStoreProduct(product.id)
      showSuccessToast('已删除')
      refreshProducts()
    } catch (e) {
      showFailToast(e.response?.data?.message || '操作失败')
    }
  }).catch(() => {})
}

async function afterRead(file) {
  const rawFile = file?.file
  if (!rawFile) {
    showFailToast('读取图片失败，请重新选择')
    return
  }
  const formData = new FormData()
  formData.append('file', rawFile)
  uploadingImage.value = true
  file.status = 'uploading'
  try {
    const res = await uploadMerchantProductImage(formData)
    const imageURL = String(res.data?.url || '')
    if (!imageURL) throw new Error('图片上传接口未返回 URL')
    productForm.value.image_url = imageURL
    file.status = 'done'
    file.message = '已上传'
  } catch (e) {
    file.status = 'failed'
    file.message = '上传失败'
    showFailToast(e.response?.data?.message || e.message || '图片上传失败')
  } finally {
    uploadingImage.value = false
  }
}


async function saveProduct() {
  if (uploadingImage.value) {
    showToast('请等待图片上传完成')
    return
  }
  saving.value = true
  try {
    const data = buildMerchantProductPayload(productForm.value, stores.value[currentStoreIndex.value]?.id)

    if (isEditing.value) {
      await updateMerchantStoreProduct(currentProductId.value, data)
      showSuccessToast('修改成功')
    } else {
      await createMerchantStoreProduct(data)
      showSuccessToast('添加成功')
    }

    showProductPopup.value = false
    refreshProducts()
  } catch (e) {
    showFailToast(e.response?.data?.message || e.message || '保存失败')
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  loadStores()
})

watch(
  () => route.query.storeId,
  (requestedStoreId) => {
    if (stores.value.length === 0) return
    const nextStoreIndex = resolveMerchantProductStoreIndex(stores.value, requestedStoreId)
    if (nextStoreIndex === currentStoreIndex.value) return
    currentStoreIndex.value = nextStoreIndex
    refreshProducts()
  }
)

onBeforeUnmount(() => {
  debouncedSearch.cancel()
  productRequestVersion.next()
  activeProductsRequest = null
})
</script>

<style scoped>
.merchant-products {
  min-height: 100vh;
  background: #f7f8fa;
  padding-bottom: 20px;
}
.readonly-banner { display: flex; align-items: center; justify-content: space-between; gap: 10px; margin: 10px 12px; padding: 10px 12px; border: 1px solid #f0d49b; border-radius: 12px; color: #80591b; background: #fff8e8; font-size: 12px; line-height: 1.5; }

.filter-bar {
  background: white;
}

.product-swipe-hint {
  margin: 8px 16px;
  color: #718096;
  font-size: 12px;
  line-height: 1.4;
}

.empty-products {
  padding: 60px 0;
}

.product-item {
  margin-bottom: 2px;
}

.product-card {
  border-radius: 0;
}

.product-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}


.product-form {
  padding: 16px;
  max-height: 90vh;
  overflow-y: auto;
  padding-bottom: calc(16px + env(safe-area-inset-bottom));
}

.advanced-settings {
  margin: 12px 0;
  overflow: hidden;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  background: #fff;
}

.advanced-settings-toggle {
  display: flex;
  width: 100%;
  align-items: center;
  gap: 8px;
  padding: 12px;
  border: 0;
  background: #f8fafc;
  color: #1f2937;
  cursor: pointer;
  text-align: left;
}

.advanced-settings-title {
  font-weight: 600;
  white-space: nowrap;
}

.advanced-settings-summary {
  flex: 1;
  color: #64748b;
  font-size: 11px;
  line-height: 1.4;
}

.advanced-settings-action {
  color: #2563eb;
  font-size: 12px;
  white-space: nowrap;
}

.advanced-settings-content {
  padding: 8px 0 12px;
}

.advanced-preserved-values {
  margin: 10px 12px 0;
  padding: 10px 12px;
  border-radius: 8px;
  background: #f8fafc;
  color: #475569;
  font-size: 12px;
  line-height: 1.6;
}

.advanced-preserved-values p {
  margin: 6px 0 0;
  color: #64748b;
}

.form-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 16px;
  font-weight: bold;
  margin-bottom: 16px;
}

.form-actions {
  padding: 16px;
}
</style>
