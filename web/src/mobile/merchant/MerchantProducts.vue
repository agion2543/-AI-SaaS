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
        <van-button size="small" type="primary" @click="showAddProduct">添加商品</van-button>
      </template>
    </van-nav-bar>

    <!-- 门店选择 -->
    <van-tabs v-if="stores.length > 1" v-model:active="currentStoreIndex" sticky @change="onStoreChange">
      <van-tab v-for="store in stores" :key="store.id" :title="store.name" />
    </van-tabs>

    <!-- 筛选和搜索 -->
    <div class="filter-bar">
      <van-search
        v-model="keyword"
        placeholder="搜索商品"
        @search="searchProducts"
      />
    </div>

    <!-- 商品列表 -->
    <van-list
      v-model:loading="loading"
      :finished="finished"
      finished-text="没有更多了"
      @load="loadProducts"
    >
      <div v-if="products.length === 0 && !loading" class="empty-products">
        <van-empty description="暂无商品">
          <van-button type="primary" size="small" @click="showAddProduct">添加商品</van-button>
        </van-empty>
      </div>

      <van-cell-group inset>
        <div v-for="product in products" :key="product.id" class="product-item">
          <van-swipe-cell>
            <van-card
              :price="formatPrice(product.price)"
              :title="product.name"
              :desc="product.description || '暂无描述'"
              :thumb="product.image || defaultProductImg"
              class="product-card"
            >
              <template #tags>
                <van-tag v-if="!product.is_active" type="warning">已下架</van-tag>
                <van-tag v-if="product.stock <= 0" type="danger">已售罄</van-tag>
                <van-tag v-else-if="product.stock <= 5" type="warning">库存紧张</van-tag>
              </template>
              <template #footer>
                <div class="product-actions">
                  <van-tag :type="product.is_active ? 'success' : 'default'">
                    {{ product.is_active ? '上架' : '下架' }}
                  </van-tag>
                  <span class="stock">库存: {{ product.stock }}</span>
                </div>
              </template>
            </van-card>

            <template #right>
              <van-button
                square
                type="primary"
                text="编辑"
                @click="editProduct(product)"
              />
              <van-button
                square
                :type="product.is_active ? 'warning' : 'success'"
                :text="product.is_active ? '下架' : '上架'"
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

        <van-form ref="productForm" @submit="saveProduct">
          <van-cell-group inset>
            <van-field
              v-model="productForm.name"
              label="商品名称"
              placeholder="请输入商品名称"
              :rules="[{ required: true, message: '请输入商品名称' }]"
            />
            <van-field
              v-model="productForm.price"
              type="number"
              label="价格"
              placeholder="请输入价格"
              :rules="[{ required: true, message: '请输入价格' }]"
            />
            <van-field
              v-model="productForm.stock"
              type="number"
              label="库存"
              placeholder="请输入库存"
              :rules="[{ required: true, message: '请输入库存' }]"
            />
            <van-field
              v-model="productForm.description"
              type="textarea"
              label="描述"
              placeholder="请输入商品描述"
              rows="2"
            />
            <van-field
              label="商品图片"
              placeholder="请输入图片URL"
            >
              <template #input>
                <van-uploader
                  v-model="productForm.image"
                  :max-count="1"
                  :after-read="afterRead"
                >
                  <van-image
                    v-if="productForm.imageUrl"
                    :src="productForm.imageUrl"
                    width="80"
                    height="80"
                  />
                  <van-button v-else size="small" type="primary">上传图片</van-button>
                </van-uploader>
              </template>
            </van-field>
          </van-cell-group>

          <div class="form-actions">
            <van-button block type="primary" native-type="submit" :loading="saving">
              保存
            </van-button>
          </div>
        </van-form>
      </div>
    </van-popup>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { showToast, showSuccessToast, showFailToast, showDialog } from 'vant'
import {
  fetchMerchantStores,
  fetchMerchantStoreProducts,
  createMerchantStoreProduct,
  updateMerchantStoreProduct,
  deleteMerchantStoreProduct
} from '@/api/modules'

const router = useRouter()

const loading = ref(false)
const finished = ref(false)
const stores = ref([])
const currentStoreIndex = ref(0)
const products = ref([])
const keyword = ref('')
const page = ref(1)
const pageSize = 20

const showProductPopup = ref(false)
const isEditing = ref(false)
const saving = ref(false)
const currentProductId = ref(null)
const productForm = ref({
  name: '',
  price: '',
  stock: '',
  description: '',
  image: [],
  imageUrl: ''
})

const defaultProductImg = 'https://via.placeholder.com/100x100?text=商品'

function formatPrice(price) {
  return parseFloat(price || 0).toFixed(2)
}

function onStoreChange() {
  refreshProducts()
}

function searchProducts() {
  refreshProducts()
}

async function loadStores() {
  try {
    const res = await fetchMerchantStores()
    stores.value = res.data || []
    if (stores.value.length > 0 && currentStoreIndex.value === 0) {
      loadProducts()
    }
  } catch (e) {
    console.error('加载门店失败:', e)
  }
}

async function loadProducts() {
  if (stores.value.length === 0) return

  loading.value = true
  try {
    const storeId = stores.value[currentStoreIndex.value]?.id
    const res = await fetchMerchantStoreProducts(storeId, {
      keyword: keyword.value,
      page: page.value,
      page_size: pageSize
    })

    const data = res.data?.items || res.data || []

    if (page.value === 1) {
      products.value = data
    } else {
      products.value.push(...data)
    }

    if (data.length < pageSize) {
      finished.value = true
    } else {
      page.value++
    }
  } catch (e) {
    console.error('加载商品失败:', e)
    showToast('加载失败')
  } finally {
    loading.value = false
  }
}

function refreshProducts() {
  page.value = 1
  finished.value = false
  products.value = []
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
  productForm.value = {
    name: '',
    price: '',
    stock: '',
    description: '',
    image: [],
    imageUrl: ''
  }
  showProductPopup.value = true
}

function editProduct(product) {
  isEditing.value = true
  currentProductId.value = product.id
  productForm.value = {
    name: product.name,
    price: product.price.toString(),
    stock: product.stock.toString(),
    description: product.description || '',
    image: [],
    imageUrl: product.image || ''
  }
  showProductPopup.value = true
}

async function toggleProductStatus(product) {
  try {
    await updateMerchantStoreProduct(product.id, {
      is_active: !product.is_active
    })
    showSuccessToast(product.is_active ? '已下架' : '已上架')
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

function afterRead(file) {
  productForm.value.imageUrl = file.content
}

async function saveProduct() {
  saving.value = true
  try {
    const data = {
      name: productForm.value.name,
      price: parseFloat(productForm.value.price),
      stock: parseInt(productForm.value.stock),
      description: productForm.value.description,
      image: productForm.value.imageUrl,
      store_id: stores.value[currentStoreIndex.value]?.id
    }

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
    showFailToast(e.response?.data?.message || '保存失败')
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  loadStores()
})
</script>

<style scoped>
.merchant-products {
  min-height: 100vh;
  background: #f7f8fa;
  padding-bottom: 20px;
}

.filter-bar {
  background: white;
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

.stock {
  font-size: 12px;
  color: #999;
}

.product-form {
  padding: 16px;
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
