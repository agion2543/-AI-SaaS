<template>
  <div class="merchant-coupons">
    <!-- 顶部导航 -->
    <van-nav-bar
      title="优惠券管理"
      left-arrow
      fixed
      placeholder
      @click-left="router.back()"
    >
      <template #right>
        <van-button size="small" type="primary" @click="showAddCoupon">创建优惠券</van-button>
      </template>
    </van-nav-bar>

    <!-- 优惠券列表 -->
    <van-tabs v-model:active="activeTab" sticky @change="onTabChange">
      <van-tab title="全部" name="all" />
      <van-tab title="生效中" name="active" />
      <van-tab title="未开始" name="pending" />
      <van-tab title="已失效" name="expired" />
    </van-tabs>

    <van-list
      v-model:loading="loading"
      :finished="finished"
      finished-text="没有更多了"
      @load="loadCoupons"
    >
      <div v-if="coupons.length === 0 && !loading" class="empty-coupons">
        <van-empty description="暂无优惠券">
          <van-button type="primary" size="small" @click="showAddCoupon">创建优惠券</van-button>
        </van-empty>
      </div>

      <div v-for="coupon in coupons" :key="coupon.id" class="coupon-item">
        <van-card class="coupon-card">
          <template #title>
            <div class="coupon-header">
              <div class="coupon-value">
                <span class="currency">¥</span>
                <span class="amount">{{ coupon.value }}</span>
                <span class="desc">优惠券</span>
              </div>
              <div class="coupon-status">
                <van-tag :type="getStatusType(coupon)">
                  {{ getStatusText(coupon) }}
                </van-tag>
              </div>
            </div>
          </template>
          <template #desc>
            <div class="coupon-info">
              <div class="coupon-name">{{ coupon.name || '满减优惠券' }}</div>
              <div class="coupon-condition">满{{ coupon.min_amount }}元可用</div>
              <div class="coupon-time">
                {{ formatTime(coupon.start_time) }} - {{ formatTime(coupon.end_time) }}
              </div>
            </div>
          </template>
          <template #footer>
            <div class="coupon-stats">
              <span>已使用: {{ coupon.used_count || 0 }}</span>
              <span>剩余: {{ coupon.stock - (coupon.used_count || 0) }}</span>
            </div>
            <div class="coupon-actions">
              <van-button
                v-if="coupon.status === 'active'"
                size="mini"
                plain
                type="danger"
                @click="disableCoupon(coupon)"
              >
                禁用
              </van-button>
              <van-button
                v-if="coupon.status === 'disabled'"
                size="mini"
                plain
                type="success"
                @click="enableCoupon(coupon)"
              >
                启用
              </van-button>
              <van-button size="mini" plain @click="editCoupon(coupon)">
                编辑
              </van-button>
            </div>
          </template>
        </van-card>
      </div>
    </van-list>

    <!-- 创建/编辑优惠券弹窗 -->
    <van-popup
      v-model:show="showCouponPopup"
      position="bottom"
      round
      style="max-height: 90%;"
    >
      <div class="coupon-form">
        <div class="form-header">
          <span>{{ isEditing ? '编辑优惠券' : '创建优惠券' }}</span>
          <van-icon name="cross" @click="showCouponPopup = false" />
        </div>

        <van-form @submit="saveCoupon">
          <van-cell-group inset>
            <van-field
              v-model="couponForm.name"
              label="优惠券名称"
              placeholder="如: 新用户专享"
            />
            <van-field
              v-model="couponForm.value"
              type="number"
              label="优惠金额"
              placeholder="请输入优惠金额"
              :rules="[{ required: true, message: '请输入优惠金额' }]"
            />
            <van-field
              v-model="couponForm.min_amount"
              type="number"
              label="使用门槛"
              placeholder="满多少元可用"
              :rules="[{ required: true, message: '请输入使用门槛' }]"
            />
            <van-field
              v-model="couponForm.stock"
              type="number"
              label="发放数量"
              placeholder="请输入发放数量"
              :rules="[{ required: true, message: '请输入发放数量' }]"
            />
            <van-field
              v-model="couponForm.start_time"
              is-link
              readonly
              label="开始时间"
              placeholder="请选择开始时间"
              @click="showStartPicker = true"
            />
            <van-field
              v-model="couponForm.end_time"
              is-link
              readonly
              label="结束时间"
              placeholder="请选择结束时间"
              @click="showEndPicker = true"
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

    <!-- 日期选择 -->
    <van-popup v-model:show="showStartPicker" position="bottom">
      <van-date-picker
        v-model="startDate"
        title="选择开始时间"
        @confirm="onStartConfirm"
        @cancel="showStartPicker = false"
      />
    </van-popup>

    <van-popup v-model:show="showEndPicker" position="bottom">
      <van-date-picker
        v-model="endDate"
        title="选择结束时间"
        @confirm="onEndConfirm"
        @cancel="showEndPicker = false"
      />
    </van-popup>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { showToast, showSuccessToast, showFailToast, showDialog } from 'vant'
import { fetchMerchantCoupons, voidMerchantCoupon, redeemMerchantCoupon } from '@/api/modules'

const router = useRouter()

const loading = ref(false)
const finished = ref(false)
const activeTab = ref('all')
const coupons = ref([])

const showCouponPopup = ref(false)
const isEditing = ref(false)
const saving = ref(false)
const currentCouponId = ref(null)
const couponForm = ref({
  name: '',
  value: '',
  min_amount: '',
  stock: '',
  start_time: '',
  end_time: ''
})

const showStartPicker = ref(false)
const showEndPicker = ref(false)
const startDate = ref(['2024', '01', '01'])
const endDate = ref(['2024', '12', '31'])

function getStatusType(coupon) {
  const now = new Date()
  const start = new Date(coupon.start_time)
  const end = new Date(coupon.end_time)

  if (coupon.is_void || coupon.status === 'disabled') return 'default'
  if (now < start) return 'primary'
  if (now > end) return 'warning'
  return 'success'
}

function getStatusText(coupon) {
  const now = new Date()
  const start = new Date(coupon.start_time)
  const end = new Date(coupon.end_time)

  if (coupon.is_void || coupon.status === 'disabled') return '已禁用'
  if (now < start) return '未开始'
  if (now > end) return '已过期'
  return '生效中'
}

function formatTime(time) {
  if (!time) return '-'
  return new Date(time).toLocaleDateString('zh-CN')
}

async function loadCoupons() {
  loading.value = true
  try {
    const statusMap = {
      all: '',
      active: 'active',
      pending: 'pending',
      expired: 'expired'
    }

    const res = await fetchMerchantCoupons({ status: statusMap[activeTab.value] })
    coupons.value = res.data || []
  } catch (e) {
    console.error('加载优惠券失败:', e)
    showToast('加载失败')
  } finally {
    loading.value = false
    finished.value = true
  }
}

function onTabChange() {
  loadCoupons()
}

function showAddCoupon() {
  isEditing.value = false
  currentCouponId.value = null
  couponForm.value = {
    name: '',
    value: '',
    min_amount: '',
    stock: '',
    start_time: '',
    end_time: ''
  }
  const now = new Date()
  const nextMonth = new Date(now.getFullYear(), now.getMonth() + 1, 1)
  startDate.value = [now.getFullYear(), now.getMonth() + 1, now.getDate()].map(String)
  endDate.value = [nextMonth.getFullYear(), nextMonth.getMonth() + 1, 1].map(String)
  couponForm.value.start_time = formatDate(startDate.value)
  couponForm.value.end_time = formatDate(endDate.value)
  showCouponPopup.value = true
}

function editCoupon(coupon) {
  isEditing.value = true
  currentCouponId.value = coupon.id
  couponForm.value = {
    name: coupon.name || '',
    value: coupon.value?.toString() || '',
    min_amount: coupon.min_amount?.toString() || '',
    stock: coupon.stock?.toString() || '',
    start_time: formatDate(new Date(coupon.start_time).toISOString().split('T')[0].split('-')),
    end_time: formatDate(new Date(coupon.end_time).toISOString().split('T')[0].split('-'))
  }
  startDate.value = new Date(coupon.start_time).toISOString().split('T')[0].split('-')
  endDate.value = new Date(coupon.end_time).toISOString().split('T')[0].split('-')
  showCouponPopup.value = true
}

function formatDate(dateArr) {
  return dateArr.map(d => d.padStart(2, '0')).join('-')
}

function onStartConfirm({ selectedValues }) {
  couponForm.value.start_time = formatDate(selectedValues)
  startDate.value = selectedValues
  showStartPicker.value = false
}

function onEndConfirm({ selectedValues }) {
  couponForm.value.end_time = formatDate(selectedValues)
  endDate.value = selectedValues
  showEndPicker.value = false
}

async function saveCoupon() {
  saving.value = true
  try {
    // TODO: 调用 API 创建/更新优惠券
    showSuccessToast(isEditing.value ? '修改成功' : '创建成功')
    showCouponPopup.value = false
    loadCoupons()
  } catch (e) {
    showFailToast(e.response?.data?.message || '保存失败')
  } finally {
    saving.value = false
  }
}

async function disableCoupon(coupon) {
  try {
    await voidMerchantCoupon(coupon.id)
    showSuccessToast('已禁用')
    loadCoupons()
  } catch (e) {
    showFailToast(e.response?.data?.message || '操作失败')
  }
}

async function enableCoupon(coupon) {
  try {
    await redeemMerchantCoupon(coupon.id)
    showSuccessToast('已启用')
    loadCoupons()
  } catch (e) {
    showFailToast(e.response?.data?.message || '操作失败')
  }
}

onMounted(() => {
  loadCoupons()
})
</script>

<style scoped>
.merchant-coupons {
  min-height: 100vh;
  background: #f7f8fa;
  padding-bottom: 20px;
}

.empty-coupons {
  padding: 60px 0;
}

.coupon-item {
  margin: 12px;
}

.coupon-card {
  border-radius: 8px;
}

.coupon-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.coupon-value {
  display: flex;
  align-items: baseline;
}

.currency {
  font-size: 16px;
  color: #ee0a24;
}

.amount {
  font-size: 28px;
  font-weight: bold;
  color: #ee0a24;
  margin: 0 4px;
}

.desc {
  font-size: 14px;
  color: #666;
}

.coupon-info {
  padding: 8px 0;
}

.coupon-name {
  font-size: 14px;
  font-weight: bold;
  margin-bottom: 4px;
}

.coupon-condition {
  font-size: 12px;
  color: #666;
  margin-bottom: 4px;
}

.coupon-time {
  font-size: 12px;
  color: #999;
}

.coupon-stats {
  display: flex;
  gap: 16px;
  font-size: 12px;
  color: #999;
  margin-bottom: 8px;
}

.coupon-actions {
  display: flex;
  gap: 8px;
}

.coupon-form {
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
