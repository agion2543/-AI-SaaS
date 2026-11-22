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
        <van-button v-if="canWrite" size="small" type="primary" @click="showAddStore">添加门店</van-button>
      </template>
    </van-nav-bar>

    <div v-if="!canWrite" class="readonly-banner" role="status">
      订阅已到期：门店信息仅可查看，新增、编辑与营业状态修改已关闭。
      <van-button size="small" type="warning" plain @click="router.push('/m/merchant/subscription')">查看续费</van-button>
    </div>

    <!-- 门店列表 -->
    <van-list
      v-model:loading="loading"
      :finished="finished"
      finished-text="没有更多了"
      @load="loadStores"
    >
      <div v-if="stores.length === 0 && !loading" class="empty-stores">
        <van-empty description="暂无门店">
          <van-button v-if="canWrite" type="primary" size="small" @click="showAddStore">添加门店</van-button>
        </van-empty>
      </div>

      <van-cell-group inset>
        <div v-for="store in stores" :key="store.id" class="store-item">
          <van-cell
            :title="store.name"
            :label="store.address || '暂无地址'"
            :is-link="canWrite"
            @click="canWrite && editStore(store)"
          >
            <template #value>
              <div class="store-status">
                <van-tag :type="store.is_open ? 'success' : 'default'">
                  {{ store.is_open ? '营业中' : '休息中' }}
                </van-tag>
                <div class="store-actions">
                  <van-switch
                    :model-value="store.is_open"
                    size="20"
                    :disabled="!canWrite"
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
              <van-button v-if="canWrite" size="mini" plain @click="editStore(store)">编辑</van-button>
              <van-button size="mini" plain @click="viewProducts(store)">商品</van-button>
            </div>
          </div>
        </div>
      </van-cell-group>
    </van-list>

    <!-- 添加/编辑门店弹窗 -->
    <van-popup
      v-if="canWrite"
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
              v-model="storeForm.contact_phone"
              type="tel"
              maxlength="11"
              label="联系电话"
              placeholder="请输入 11 位手机号"
              :rules="[
                { required: true, message: '请输入联系电话' },
                { pattern: /^1\d{10}$/, message: '请输入有效的 11 位手机号' }
              ]"
            />
            <van-field
              v-model="storeForm.business_hours"
              type="textarea"
              label="营业时间"
              placeholder="例如：周一至周日 09:00-21:00"
              maxlength="120"
              rows="2"
            />
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
import { showToast, showSuccessToast, showFailToast } from 'vant'
import { fetchMerchantStores, createMerchantStore, updateMerchantStore } from '@/api/modules'
import { buildMerchantStorePayload, isValidMerchantStorePhone } from '@/utils/merchantStoreContract'
import { cachedMobileMerchantSubscriptionState, mobileSubscriptionActions } from '@/utils/mobileMerchantSubscription'

const router = useRouter()
const canWrite = mobileSubscriptionActions(cachedMobileMerchantSubscriptionState()).canWriteBusiness

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
  contact_phone: '',
  is_open: true,
  business_hours: ''
})

async function loadStores() {
  loading.value = true
  try {
    const res = await fetchMerchantStores()
    stores.value = Array.isArray(res.data?.list) ? res.data.list : (Array.isArray(res.data) ? res.data : [])
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
    contact_phone: '',
    is_open: true,
    business_hours: ''
  }
  showStorePopup.value = true
}

function editStore(store) {
  isEditing.value = true
  currentStoreId.value = store.id
  storeForm.value = {
    name: store.name,
    address: store.address || '',
    contact_phone: store.contact_phone || '',
    is_open: store.is_open !== false,
    business_hours: store.business_hours || ''
  }
  showStorePopup.value = true
}

async function toggleStoreStatus(store) {
  try {
    const payload = buildMerchantStorePayload(store, { is_open: !store.is_open })
    await updateMerchantStore(store.id, payload)
    showSuccessToast(store.is_open ? '已休息' : '已营业')
    loadStores()
  } catch (e) {
    showFailToast(e.response?.data?.message || '操作失败')
  }
}

function viewProducts(store) {
  router.push(`/m/merchant/products?storeId=${store.id}`)
}

async function saveStore() {
  saving.value = true
  try {
    if (!isValidMerchantStorePhone(storeForm.value.contact_phone)) {
      showToast('请输入有效的 11 位手机号')
      return
    }

    const data = buildMerchantStorePayload(storeForm.value)

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
.readonly-banner { display: flex; align-items: center; justify-content: space-between; gap: 10px; margin: 10px 12px; padding: 10px 12px; border: 1px solid #f0d49b; border-radius: 12px; color: #80591b; background: #fff8e8; font-size: 12px; line-height: 1.5; }

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
