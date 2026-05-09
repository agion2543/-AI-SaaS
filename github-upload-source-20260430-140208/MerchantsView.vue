<template>
  <div class="merchant-page">
    <section class="page-card hero-card">
      <div>
        <div class="eyebrow">MERCHANT OPS</div>
        <h2 class="page-title">商家运营</h2>
        <p class="muted">管理商家入驻、订阅状态、冻结状态与后台开通权限。</p>
      </div>
      <el-button type="primary" :loading="loading" @click="load">刷新同步</el-button>
    </section>

    <el-alert
      v-if="loadError"
      class="error-alert"
      type="error"
      show-icon
      :closable="false"
      :title="loadError"
    />

    <section class="stats-grid">
      <div class="stat-card">
        <span>商家总数</span>
        <strong>{{ merchants.length }}</strong>
      </div>
      <div class="stat-card">
        <span>已订阅</span>
        <strong>{{ subscribedCount }}</strong>
      </div>
      <div class="stat-card">
        <span>未订阅</span>
        <strong>{{ unsubscribedCount }}</strong>
      </div>
      <div class="stat-card">
        <span>最新注册</span>
        <strong>{{ newestMerchant?.name || '-' }}</strong>
      </div>
    </section>

    <section class="page-card table-card">
      <div class="filter-grid">
        <el-input v-model="keyword" clearable placeholder="搜索商家名称或手机号" @keyup.enter="search" />
        <el-select v-model="status" clearable placeholder="商家状态" @change="search">
          <el-option label="待处理" value="pending" />
          <el-option label="正常" value="active" />
          <el-option label="已冻结" value="suspended" />
        </el-select>
        <el-select v-model="subscriptionFilter" clearable placeholder="订阅状态" @change="search">
          <el-option label="订阅中" value="active" />
          <el-option label="未订阅/过期" value="inactive" />
        </el-select>
        <el-select v-model="sortBy" placeholder="排序字段" @change="search">
          <el-option label="注册时间" value="created_at" />
          <el-option label="到期时间" value="subscription_expire_at" />
          <el-option label="商家名称" value="name" />
          <el-option label="商家 ID" value="id" />
        </el-select>
        <el-select v-model="sortOrder" placeholder="排序方式" @change="search">
          <el-option label="降序" value="desc" />
          <el-option label="升序" value="asc" />
        </el-select>
        <div class="filter-actions">
          <el-button type="primary" @click="search">查询</el-button>
          <el-button @click="reset">重置</el-button>
        </div>
      </div>

      <el-table
        v-loading="loading"
        :data="visibleMerchants"
        empty-text="暂无商家数据"
        @sort-change="handleTableSort"
      >
        <el-table-column label="排名" width="80">
          <template #default="{ $index }">#{{ (page - 1) * pageSize + $index + 1 }}</template>
        </el-table-column>
        <el-table-column prop="id" label="ID" width="80" sortable="custom" />
        <el-table-column prop="name" label="商家名称" min-width="180" sortable="custom">
          <template #default="{ row }">
            <div class="name-cell">
              <strong>{{ row.name }}</strong>
              <el-tag v-if="newestMerchant?.id === row.id" size="small" type="success">最新注册</el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="contact_phone" label="联系人手机号" min-width="150" />
        <el-table-column label="商家状态" width="120">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)">{{ statusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="订阅状态" width="140">
          <template #default="{ row }">
            <el-tag :type="subscriptionValid(row) ? 'success' : 'info'">
              {{ subscriptionValid(row) ? '订阅中' : '未订阅' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="subscription_expire_at" label="到期时间" min-width="180" sortable="custom">
          <template #default="{ row }">{{ formatTime(row.subscription_expire_at || row.subscription_expired_at) }}</template>
        </el-table-column>
        <el-table-column prop="created_at" label="注册时间" min-width="180" sortable="custom">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="430" fixed="right">
          <template #default="{ row }">
            <div class="action-row">
              <el-button size="small" @click="goDetail(row)">详情</el-button>
              <el-button v-if="row.status !== 'active'" type="primary" size="small" @click="changeStatus(row, 'active')">
                通过/恢复
              </el-button>
              <el-button v-if="row.status === 'active'" type="danger" plain size="small" @click="changeStatus(row, 'suspended')">
                冻结
              </el-button>
              <el-button size="small" type="success" plain @click="quickOpen(row, 'month')">开通月付</el-button>
              <el-button size="small" type="success" plain @click="quickOpen(row, 'year')">开通年付</el-button>
              <el-button size="small" type="warning" plain @click="openCustomDialog(row)">自定义时长</el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <div class="pager">
        <el-pagination
          background
          layout="total, sizes, prev, pager, next"
          :total="filteredMerchants.length"
          :page-size="pageSize"
          :current-page="page"
          :page-sizes="[10, 20, 50]"
          @size-change="handleSizeChange"
          @current-change="page = $event"
        />
      </div>
    </section>

    <el-dialog v-model="dialogVisible" title="自定义开通/调整订阅" width="520px">
      <el-form label-width="110px">
        <el-form-item label="商家">
          <el-input :model-value="currentMerchant?.name || '-'" disabled />
        </el-form-item>
        <el-form-item label="套餐">
          <el-select v-model="subscriptionForm.plan">
            <el-option label="月付" value="month" />
            <el-option label="年付" value="year" />
          </el-select>
        </el-form-item>
        <el-form-item label="调整天数">
          <el-input-number v-model="subscriptionForm.duration_days" :min="-3650" :max="3650" />
          <span class="form-tip">正数增加，负数减少，0 按套餐默认天数</span>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="subscriptionForm.note" maxlength="120" placeholder="例如：测试开通、补偿延期、人工扣减" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="savingSubscription" @click="submitCustomSubscription">保存调整</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { fetchMerchants, openMerchantSubscription, updateMerchantStatus } from '../../api/modules'

const merchants = ref([])
const keyword = ref('')
const status = ref('')
const subscriptionFilter = ref('')
const sortBy = ref('created_at')
const sortOrder = ref('desc')
const page = ref(1)
const pageSize = ref(20)
const loading = ref(false)
const loadError = ref('')
const dialogVisible = ref(false)
const currentMerchant = ref(null)
const savingSubscription = ref(false)
const router = useRouter()

const subscriptionForm = reactive({
  plan: 'month',
  duration_days: 30,
  note: ''
})

const newestMerchant = computed(() => [...merchants.value].sort((a, b) => dateValue(b.created_at) - dateValue(a.created_at))[0] || null)
const subscribedCount = computed(() => merchants.value.filter(subscriptionValid).length)
const unsubscribedCount = computed(() => merchants.value.length - subscribedCount.value)

const load = async () => {
  loading.value = true
  loadError.value = ''
  try {
    const res = await fetchMerchants({ page: 1, page_size: 500, t: Date.now() })
    merchants.value = res.data?.list || []
    if (!merchants.value.length) loadError.value = '接口请求成功，但没有返回商家数据。请确认当前登录的是平台后台账号。'
  } catch (err) {
    merchants.value = []
    loadError.value = err.response?.data?.message || '商家列表加载失败，请重新登录后台或检查后端服务。'
    ElMessage.error(loadError.value)
  } finally {
    loading.value = false
  }
}

const filteredMerchants = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  const list = merchants.value.filter((item) => {
    const matchKeyword = !kw || `${item.name || ''} ${item.contact_phone || ''}`.toLowerCase().includes(kw)
    const matchStatus = !status.value || item.status === status.value
    const matchSubscription = !subscriptionFilter.value ||
      (subscriptionFilter.value === 'active' ? subscriptionValid(item) : !subscriptionValid(item))
    return matchKeyword && matchStatus && matchSubscription
  })
  return [...list].sort((a, b) => compareMerchant(a, b))
})

const visibleMerchants = computed(() => filteredMerchants.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value))

const search = () => {
  page.value = 1
}

const reset = () => {
  keyword.value = ''
  status.value = ''
  subscriptionFilter.value = ''
  sortBy.value = 'created_at'
  sortOrder.value = 'desc'
  search()
}

const handleTableSort = ({ prop, order }) => {
  if (!prop) return
  sortBy.value = prop === 'subscription_expire_at' ? 'subscription_expire_at' : prop
  sortOrder.value = order === 'ascending' ? 'asc' : 'desc'
  search()
}

const handleSizeChange = (nextSize) => {
  pageSize.value = nextSize
  page.value = 1
}

const compareMerchant = (a, b) => {
  let av
  let bv
  if (sortBy.value === 'name') {
    av = a.name || ''
    bv = b.name || ''
    return sortOrder.value === 'asc' ? av.localeCompare(bv) : bv.localeCompare(av)
  }
  if (sortBy.value === 'subscription_expire_at') {
    av = dateValue(a.subscription_expire_at || a.subscription_expired_at)
    bv = dateValue(b.subscription_expire_at || b.subscription_expired_at)
  } else if (sortBy.value === 'id') {
    av = Number(a.id || 0)
    bv = Number(b.id || 0)
  } else {
    av = dateValue(a.created_at)
    bv = dateValue(b.created_at)
  }
  return sortOrder.value === 'asc' ? av - bv : bv - av
}

const changeStatus = async (row, nextStatus) => {
  await updateMerchantStatus(row.id, { status: nextStatus })
  ElMessage.success('商家状态已更新')
  await load()
}

const quickOpen = async (row, plan) => {
  const planText = plan === 'year' ? '年付' : '月付'
  await ElMessageBox.confirm(`确定为“${row.name}”开通${planText}订阅吗？`, '开通订阅', { type: 'warning' })
  await openMerchantSubscription(row.id, {
    plan,
    duration_days: plan === 'year' ? 365 : 30,
    note: `后台开通${planText}`
  })
  ElMessage.success(`已为 ${row.name} 开通${planText}`)
  await load()
}

const openCustomDialog = (row) => {
  currentMerchant.value = row
  subscriptionForm.plan = row.subscription_plan === 'year' ? 'year' : 'month'
  subscriptionForm.duration_days = 30
  subscriptionForm.note = ''
  dialogVisible.value = true
}

const submitCustomSubscription = async () => {
  if (!currentMerchant.value) return
  savingSubscription.value = true
  try {
    await openMerchantSubscription(currentMerchant.value.id, { ...subscriptionForm })
    ElMessage.success('订阅时长已调整')
    dialogVisible.value = false
    await load()
  } finally {
    savingSubscription.value = false
  }
}

const goDetail = (row) => {
  router.push(`/admin/merchants/${row.id}`)
}

const dateValue = (value) => value ? new Date(value).getTime() || 0 : 0
const subscriptionValid = (row) => {
  const expire = row.subscription_expire_at || row.subscription_expired_at
  return row.subscription_status === 'active' && expire && dateValue(expire) > Date.now()
}

const statusLabel = (value) => ({ pending: '待处理', active: '正常', suspended: '已冻结' }[value] || value || '-')
const statusType = (value) => ({ pending: 'warning', active: 'success', suspended: 'danger' }[value] || 'info')
const formatTime = (value) => value ? String(value).replace('T', ' ').slice(0, 19) : '-'

onMounted(load)
</script>

<style scoped>
.merchant-page {
  display: grid;
  gap: 18px;
}

.hero-card {
  display: flex;
  justify-content: space-between;
  gap: 20px;
  align-items: center;
  padding: 20px 22px;
}

.eyebrow {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.16em;
}

.muted {
  color: #64748b;
  margin: 0;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 14px;
}

.stat-card {
  padding: 18px;
  border-radius: 16px;
  border: 1px solid #e5edf9;
  background: #f8fbff;
}

.stat-card span {
  display: block;
  color: var(--muted);
  font-size: 13px;
}

.stat-card strong {
  display: block;
  margin-top: 8px;
  font-size: 24px;
}

.table-card {
  padding: 20px;
}

.filter-grid {
  display: grid;
  grid-template-columns: minmax(240px, 1.4fr) repeat(4, minmax(130px, 1fr)) auto;
  gap: 10px;
  align-items: center;
  margin-bottom: 16px;
}

.filter-actions,
.action-row,
.name-cell {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  align-items: center;
}

.pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 18px;
}

.form-tip {
  margin-left: 12px;
  color: #64748b;
  font-size: 12px;
}

@media (max-width: 1100px) {
  .filter-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 720px) {
  .hero-card {
    flex-direction: column;
    align-items: flex-start;
  }

  .filter-grid {
    grid-template-columns: 1fr;
  }
}
</style>
