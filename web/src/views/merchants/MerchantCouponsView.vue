<template>
  <div class="coupon-page">
    <section class="coupon-hero page-card">
      <div>
        <p class="eyebrow">COUPON WALLET</p>
        <h2>券包 / 核销管理</h2>
        <p>集中查看 AI 裂变海报、好友下单和复购奖励产生的券，支持按手机号查询、手动核销、作废和追溯来源订单。</p>
      </div>
      <el-button type="primary" @click="loadCoupons">刷新券包</el-button>
    </section>

    <section class="stats-grid">
      <div v-for="item in statCards" :key="item.label" class="stat-card page-card">
        <span>{{ item.label }}</span>
        <strong>{{ item.value }}</strong>
        <small>{{ item.hint }}</small>
      </div>
    </section>

    <section class="filter-card page-card">
      <el-input
        v-model="filters.phone"
        clearable
        maxlength="11"
        placeholder="输入顾客手机号查询券"
        class="filter-input"
        @keyup.enter="search"
      />
      <el-input
        v-model="filters.keyword"
        clearable
        placeholder="券码 / 券名称 / 备注"
        class="filter-input"
        @keyup.enter="search"
      />
      <el-select v-model="filters.status" clearable placeholder="券状态" class="filter-select">
        <el-option label="未使用" value="unused" />
        <el-option label="已核销" value="used" />
        <el-option label="已过期" value="expired" />
        <el-option label="已作废" value="voided" />
      </el-select>
      <el-select v-model="filters.store_id" clearable placeholder="门店" class="filter-select">
        <el-option v-for="store in stores" :key="store.id" :label="store.name" :value="store.id" />
      </el-select>
      <el-button type="primary" @click="search">查询</el-button>
      <el-button @click="reset">重置</el-button>
    </section>

    <section class="page-card table-card">
      <el-table v-loading="loading" :data="coupons" empty-text="暂无奖励券">
        <el-table-column label="券信息" min-width="220">
          <template #default="{ row }">
            <div class="coupon-title">{{ row.title || '-' }}</div>
            <div class="muted">券码：{{ row.coupon_no }}</div>
            <div class="muted">归属手机号：{{ row.owner_phone || '-' }}</div>
          </template>
        </el-table-column>
        <el-table-column label="金额 / 门槛" min-width="130">
          <template #default="{ row }">
            <strong class="money">-{{ formatMoney(row.amount) }}</strong>
            <div class="muted">满 {{ formatMoney(row.threshold) }} 可用</div>
          </template>
        </el-table-column>
        <el-table-column label="门店" min-width="140">
          <template #default="{ row }">{{ row.store?.name || '-' }}</template>
        </el-table-column>
        <el-table-column label="来源" min-width="220">
          <template #default="{ row }">
            <div>{{ ownerTypeLabel(row.owner_type) }}</div>
            <el-button
              v-if="row.source_order?.id"
              text
              type="primary"
              class="source-link"
              @click="goOrder(row.source_order.id)"
            >
              来源订单：{{ row.source_order.order_no }}
            </el-button>
            <div v-else class="muted">暂无来源订单</div>
          </template>
        </el-table-column>
        <el-table-column label="有效期" min-width="180">
          <template #default="{ row }">
            <div>{{ formatDate(row.valid_from) }}</div>
            <div class="muted">至 {{ formatDate(row.valid_to) }}</div>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)">{{ statusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="备注" min-width="180">
          <template #default="{ row }">{{ row.remark || '-' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="190" fixed="right">
          <template #default="{ row }">
            <el-button
              v-if="row.status === 'unused'"
              size="small"
              type="success"
              @click="redeem(row)"
            >
              手动核销
            </el-button>
            <el-button
              v-if="row.status === 'unused'"
              size="small"
              type="danger"
              plain
              @click="voidCoupon(row)"
            >
              作废
            </el-button>
            <span v-if="row.status !== 'unused'" class="muted">已结束</span>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-row">
        <el-pagination
          v-model:current-page="pagination.page"
          v-model:page-size="pagination.page_size"
          layout="total, sizes, prev, pager, next"
          :page-sizes="[10, 20, 50]"
          :total="pagination.total"
          @current-change="loadCoupons"
          @size-change="search"
        />
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  fetchMerchantCoupons,
  fetchMerchantStores,
  redeemMerchantCoupon,
  voidMerchantCoupon
} from '../../api/modules'

const router = useRouter()
const loading = ref(false)
const coupons = ref([])
const stores = ref([])
const stats = ref({})
const filters = reactive({
  phone: '',
  keyword: '',
  status: '',
  store_id: ''
})
const pagination = reactive({
  page: 1,
  page_size: 20,
  total: 0
})

const statCards = computed(() => [
  { label: '全部券', value: stats.value.total || 0, hint: '累计发放奖励券' },
  { label: '可用券', value: stats.value.unused || 0, hint: '顾客可下单抵扣' },
  { label: '已核销', value: stats.value.used || 0, hint: '订单支付或人工核销' },
  { label: '已过期/作废', value: Number(stats.value.expired || 0) + Number(stats.value.voided || 0), hint: '不可继续使用' }
])

const loadStores = async () => {
  try {
    const res = await fetchMerchantStores({ page: 1, page_size: 100 })
    stores.value = res.data?.list || []
  } catch {
    stores.value = []
  }
}

const loadCoupons = async () => {
  loading.value = true
  try {
    const res = await fetchMerchantCoupons({
      ...filters,
      page: pagination.page,
      page_size: pagination.page_size
    })
    coupons.value = res.data?.list || []
    stats.value = res.data?.stats || {}
    pagination.total = res.data?.total || 0
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '奖励券加载失败')
  } finally {
    loading.value = false
  }
}

const search = () => {
  pagination.page = 1
  loadCoupons()
}

const reset = () => {
  filters.phone = ''
  filters.keyword = ''
  filters.status = ''
  filters.store_id = ''
  search()
}

const redeem = async (row) => {
  const { value } = await ElMessageBox.prompt(
    `确认手动核销 ${row.owner_phone || ''} 的“${row.title}”吗？`,
    '手动核销奖励券',
    {
      confirmButtonText: '确认核销',
      cancelButtonText: '取消',
      inputPlaceholder: '可填写核销备注，例如：到店消费核销'
    }
  )
  await redeemMerchantCoupon(row.id, { remark: value || '商家手动核销' })
  ElMessage.success('奖励券已核销')
  loadCoupons()
}

const voidCoupon = async (row) => {
  const { value } = await ElMessageBox.prompt(
    `确认作废“${row.title}”吗？作废后顾客不能继续抵扣。`,
    '作废奖励券',
    {
      confirmButtonText: '确认作废',
      cancelButtonText: '取消',
      inputPlaceholder: '请填写作废原因，便于后续追溯'
    }
  )
  await voidMerchantCoupon(row.id, { remark: value || '商家手动作废' })
  ElMessage.success('奖励券已作废')
  loadCoupons()
}

const goOrder = (id) => {
  router.push(`/merchant/orders/${id}`)
}

const formatMoney = (value) => `¥${(Number(value || 0) / 100).toFixed(2)}`
const formatDate = (value) => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '-'
const statusLabel = (status) => ({
  unused: '未使用',
  used: '已核销',
  expired: '已过期',
  voided: '已作废'
}[status] || status || '-')
const statusType = (status) => ({
  unused: 'success',
  used: 'primary',
  expired: 'info',
  voided: 'danger'
}[status] || 'info')
const ownerTypeLabel = (type) => ({
  new_customer: '好友新客券',
  referrer: '分享人奖励券'
}[type] || '奖励券')

onMounted(() => {
  loadStores()
  loadCoupons()
})
</script>

<style scoped>
.coupon-page {
  display: grid;
  gap: 18px;
}

.coupon-hero {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
  padding: 28px;
  background:
    radial-gradient(circle at 92% 12%, rgba(34, 197, 94, 0.18), transparent 30%),
    linear-gradient(135deg, #ffffff 0%, #f5fbff 100%);
}

.coupon-hero h2 {
  margin: 6px 0 10px;
  font-size: 28px;
  color: #0f172a;
}

.coupon-hero p {
  margin: 0;
  color: #64748b;
}

.eyebrow {
  margin: 0;
  font-size: 12px;
  font-weight: 800;
  color: #0ea5e9;
  letter-spacing: 0.18em;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
}

.stat-card {
  padding: 18px;
}

.stat-card span,
.muted {
  color: #64748b;
  font-size: 13px;
}

.stat-card strong {
  display: block;
  margin: 8px 0 4px;
  font-size: 26px;
  color: #0f172a;
}

.stat-card small {
  color: #94a3b8;
}

.filter-card {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  padding: 16px;
}

.filter-input {
  width: 240px;
}

.filter-select {
  width: 160px;
}

.table-card {
  padding: 18px;
}

.coupon-title {
  font-weight: 800;
  color: #0f172a;
}

.money {
  color: #16a34a;
}

.source-link {
  padding: 0;
}

.pagination-row {
  display: flex;
  justify-content: flex-end;
  margin-top: 18px;
}

@media (max-width: 900px) {
  .coupon-hero {
    flex-direction: column;
  }

  .stats-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .filter-input,
  .filter-select {
    width: 100%;
  }
}
</style>
