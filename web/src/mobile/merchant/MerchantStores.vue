<template>
  <div class="merchant-stores">
    <!-- 顶部导航 -->
    <van-nav-bar
      title="门店管理"
      left-arrow
      fixed
      placeholder
      @click-left="router.back()"
    >
      <template #right>
        <van-button size="small" type="primary" @click="showAddStore">添加门店</van-button>
      </template>
    </van-nav-bar>

    <!-- 门店列表 -->
    <van-list
      v-model:loading="loading"
      :finished="finished"
      finished-text="没有更多了"
      @load="loadStores"
    >
      <div v-if="stores.length === 0 && !loading" class="empty-stores">
        <van-empty description="暂无门店">
          <van-button type="primary" size="small" @click="showAddStore">添加门店</van-button>
        </van-empty>
      </div>

      <van-cell-group inset>
        <div v-for="store in stores" :key="store.id" class="store-item">
          <van-cell
            :title="store.name"
            :label="store.address || store.description || '暂无地址'"
            is-link
            @click="editStore(store)"
          >
            <template #icon>
              <van-image
                round
                width="48"
                height="48"
                :src="store.image || defaultStoreImg"
                style="margin-right: 12px;"
              />
            </template>
            <template #value>
              <div class="store-status">
                <van-tag :type="store.is_active ? 'success' : 'default'">
                  {{ store.is_active ? '营业中' : '休息中' }}
                </van-tag>
                <div class="store-actions">
                  <van-switch
                    :model-value="store.is_active"
                    size="20"
                    @change="toggleStoreStatus(store)"
                  />
                </div>
              </div>
            </template>
          </van-cell>

          <div class="store-info">
            <div class="store-stats">
              <div class="stat-item">
                <span class="stat-value">{{ store.today_orders || 0 }}</span>
                <span class="stat-label">今日订单</span>
              </div>
              <div class="stat-item">
                <span class="stat-value">¥{{ store.today_revenue || '0.00' }}</span>
                <span class="stat-label">今日营收</span>
              </div>
            </div>
            <div class="store-buttons">
              <van-button size="mini" plain @click="editStore(store)">编辑</van-button>
              <van-button size="mini" plain @click="viewProducts(store)">商品</van-button>
            </div>
          </div>
        </div>
      </van-cell-group>
    </van-list>

    <!-- 添加/编辑门店弹窗 -->
    <van-popup
      v-model:show="showStorePopup"
      position="bottom"
      round
      style="max-height: 90%;"
    >
      <div class="store-form">
        <div class="form-header">
          <span>{{ isEditing ? '编辑门店' : '添加门店' }}</span>
          <van-icon name="cross" @click="showStorePopup = false" />
        </div>

        <van-form @submit="saveStore">
          <van-cell-group inset>
            <van-field
              v-model="storeForm.name"
              label="门店名称"
              placeholder="请输入门店名称"
              :rules="[{ required: true, message: '请输入门店名称' }]"
            />
            <van-field
              v-model="storeForm.address"
              label="门店地址"
              placeholder="请输入门店地址"
            />
            <van-field
              v-model="storeForm.phone"
              label="联系电话"
              placeholder="请输入联系电话"
            />
            <van-field
              v-model="storeForm.description"
              type="textarea"
              label="门店简介"
              placeholder="请输入门店简介"
              rows="2"
            />
            <van-field
              v-model="storeForm.notice"
              type="textarea"
              label="公告"
              placeholder="门店公告，如营业时间、优惠信息等"
              rows="2"
            />
            <van-field label="门店图标">
              <template #input>
                <van-uploader
                  v-model="storeForm.image"
                  :max-count="1"
                  :after-read="afterImageRead"
                >
                  <van-image
                    v-if="storeForm.imageUrl"
                    :src="storeForm.imageUrl"
                    width="80"
                    height="80"
                    round
                  />
                  <van-button v-else size="small" type="primary">上传图标</van-button>
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
import { fetchMerchantStores, createMerchantStore, updateMerchantStore } from '@/api/modules'

const router = useRouter()

const loading = ref(false)
const finished = ref(true)
const stores = ref([])

const showStorePopup = ref(false)
const isEditing = ref(false)
const saving = ref(false)
const currentStoreId = ref(null)
const storeForm = ref({
  name: '',
  address: '',
  phone: '',
  description: '',
  notice: '',
  image: [],
  imageUrl: ''
})

const defaultStoreImg = 'https://via.placeholder.com/48x48?text=门店'

async function loadStores() {
  loading.value = true
  try {
    const res = await fetchMerchantStores()
    stores.value = res.data || []
  } catch (e) {
    console.error('加载门店失败:', e)
    showToast('加载失败')
  } finally {
    loading.value = false
  }
}

function showAddStore() {
  isEditing.value = false
  currentStoreId.value = null
  storeForm.value = {
    name: '',
    address: '',
    phone: '',
    description: '',
    notice: '',
    image: [],
    imageUrl: ''
  }
  showStorePopup.value = true
}

function editStore(store) {
  isEditing.value = true
  currentStoreId.value = store.id
  storeForm.value = {
    name: store.name,
    address: store.address || '',
    phone: store.phone || '',
    description: store.description || '',
    notice: store.notice || '',
    image: [],
    imageUrl: store.image || ''
  }
  showStorePopup.value = true
}

async function toggleStoreStatus(store) {
  try {
    await updateMerchantStore(store.id, {
      is_active: !store.is_active
    })
    showSuccessToast(store.is_active ? '已休息' : '已营业')
    loadStores()
  } catch (e) {
    showFailToast(e.response?.data?.message || '操作失败')
  }
}

function viewProducts(store) {
  router.push(`/m/merchant/products?storeId=${store.id}`)
}

function afterImageRead(file) {
  storeForm.value.imageUrl = file.content
}

async function saveStore() {
  saving.value = true
  try {
    const data = {
      name: storeForm.value.name,
      address: storeForm.value.address,
      phone: storeForm.value.phone,
      description: storeForm.value.description,
      notice: storeForm.value.notice,
      image: storeForm.value.imageUrl
    }

    if (isEditing.value) {
      await updateMerchantStore(currentStoreId.value, data)
      showSuccessToast('修改成功')
    } else {
      await createMerchantStore(data)
      showSuccessToast('添加成功')
    }

    showStorePopup.value = false
    loadStores()
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
.merchant-stores {
  min-height: 100vh;
  background: #f7f8fa;
  padding-bottom: 20px;
}

.empty-stores {
  padding: 60px 0;
}

.store-item {
  margin-bottom: 12px;
  background: white;
  border-radius: 8px;
  overflow: hidden;
}

.store-status {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 8px;
}

.store-stats {
  display: flex;
  gap: 24px;
  padding: 12px 16px;
  background: #f7f8fa;
}

.stat-item {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.stat-value {
  font-size: 16px;
  font-weight: bold;
  color: #333;
}

.stat-label {
  font-size: 12px;
  color: #999;
}

.store-buttons {
  display: flex;
  gap: 8px;
  padding: 12px 16px;
  justify-content: flex-end;
}

.store-form {
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
