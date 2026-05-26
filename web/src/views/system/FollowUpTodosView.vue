<template>
  <div class="todos-page">
    <PageHero
      eyebrow="OPS TODO"
      title="平台待办中心"
      description="集中处理商家风险、支付异常、退款售后、结算核对和订阅续费跟进，避免事项散落在不同商家详情里。"
      compact
    >
      <template #actions>
        <el-button type="primary" :loading="loading" @click="load">刷新待办</el-button>
      </template>
    </PageHero>

    <section class="metric-grid">
      <MetricCard label="未处理" :value="openRows.length" hint="需要平台运营继续跟进的事项。" tone="warning" />
      <MetricCard label="紧急/较高" :value="priorityRows.length" hint="优先处理 high 和 urgent。" tone="primary" />
      <MetricCard label="今日到期" :value="dueTodayRows.length" hint="下次复查时间已到或已过。" tone="cyan" />
      <MetricCard label="已处理" :value="closedRows.length" hint="已关闭的跟进记录。" tone="success" />
    </section>

    <section class="quick-strip">
      <button
        v-for="item in quickFilters"
        :key="item.value || 'all'"
        type="button"
        :class="{ active: quickMode === item.value }"
        @click="quickMode = item.value"
      >
        <span>{{ item.label }}</span>
        <strong>{{ item.count }}</strong>
        <small>{{ item.hint }}</small>
      </button>
    </section>

    <DataPanel title="待办列表" description="按优先级、复查时间和创建时间排序；处理后会同步回商家详情页。">
      <div class="filters-row">
        <el-input v-model="keyword" clearable placeholder="搜索商家或跟进内容" />
        <el-select v-model="statusFilter" clearable placeholder="状态">
          <el-option label="待处理" value="open" />
          <el-option label="已处理" value="closed" />
        </el-select>
        <el-select v-model="priorityFilter" clearable placeholder="优先级">
          <el-option label="紧急" value="urgent" />
          <el-option label="较高" value="high" />
          <el-option label="普通" value="normal" />
          <el-option label="较低" value="low" />
        </el-select>
        <el-select v-model="typeFilter" clearable placeholder="类型">
          <el-option label="风险处理" value="risk" />
          <el-option label="结算跟进" value="settlement" />
          <el-option label="退款售后" value="refund" />
          <el-option label="支付异常" value="payment" />
          <el-option label="订阅续费" value="subscription" />
          <el-option label="运营沟通" value="operation" />
        </el-select>
        <el-button @click="reset">重置</el-button>
      </div>

      <el-table v-loading="loading" :data="visibleRows" empty-text="暂无待办记录">
        <el-table-column label="优先级" width="100">
          <template #default="{ row }">
            <el-tag :type="priorityType(row.priority)">{{ priorityLabel(row.priority) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="商家" min-width="160">
          <template #default="{ row }">
            <el-button text type="primary" @click="router.push(`/admin/merchants/${row.merchant_id}`)">
              {{ row.merchant?.name || `商家 #${row.merchant_id}` }}
            </el-button>
          </template>
        </el-table-column>
        <el-table-column label="类型" width="110">
          <template #default="{ row }">{{ typeLabel(row.type) }}</template>
        </el-table-column>
        <el-table-column label="来源" width="120">
          <template #default="{ row }">
            <el-tag :type="row.source === 'order_exception' ? 'warning' : 'info'">{{ sourceLabel(row.source) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="关联订单" min-width="170">
          <template #default="{ row }">
            <el-button v-if="row.order_no" text type="primary" @click="goSource(row)">{{ row.order_no }}</el-button>
            <span v-else class="muted">-</span>
          </template>
        </el-table-column>
        <el-table-column prop="content" label="跟进内容" min-width="320" show-overflow-tooltip />
        <el-table-column label="下次复查" width="160">
          <template #default="{ row }">
            <span :class="{ overdue: isDue(row) }">{{ formatTime(row.next_follow_at) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 'closed' ? 'success' : 'warning'">{{ statusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="160">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="230" fixed="right">
          <template #default="{ row }">
            <div class="table-actions">
              <el-button size="small" @click="router.push(`/admin/merchants/${row.merchant_id}`)">详情</el-button>
              <el-button v-if="row.source" size="small" type="primary" plain @click="goSource(row)">来源</el-button>
              <el-button v-if="row.status !== 'closed'" size="small" type="success" plain @click="updateStatus(row, 'closed')">完成</el-button>
              <el-button v-else size="small" plain @click="updateStatus(row, 'open')">重开</el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>
    </DataPanel>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { fetchAdminMerchantFollowUpTodos, updateAdminMerchantFollowUpStatus } from '../../api/modules'
import DataPanel from '../../components/design/DataPanel.vue'
import MetricCard from '../../components/design/MetricCard.vue'
import PageHero from '../../components/design/PageHero.vue'

const router = useRouter()
const rows = ref([])
const loading = ref(false)
const keyword = ref('')
const statusFilter = ref('')
const priorityFilter = ref('')
const typeFilter = ref('')
const quickMode = ref('')

const today = new Date().toISOString().slice(0, 10)
const openRows = computed(() => rows.value.filter((item) => item.status !== 'closed'))
const closedRows = computed(() => rows.value.filter((item) => item.status === 'closed'))
const priorityRows = computed(() => openRows.value.filter((item) => ['urgent', 'high'].includes(item.priority)))
const dueTodayRows = computed(() => openRows.value.filter(isDue))
const quickFilters = computed(() => [
  { label: '全部待办', value: '', count: rows.value.length, hint: '含已处理记录' },
  { label: '未处理', value: 'open', count: openRows.value.length, hint: '运营主工作池' },
  { label: '紧急/较高', value: 'priority', count: priorityRows.value.length, hint: '先处理这些' },
  { label: '今日到期', value: 'due', count: dueTodayRows.value.length, hint: '需要复查' },
  { label: '已处理', value: 'closed', count: closedRows.value.length, hint: '可回溯处理结果' }
])

const visibleRows = computed(() => rows.value
  .filter((item) => matchQuick(item))
  .filter((item) => !statusFilter.value || item.status === statusFilter.value)
  .filter((item) => !priorityFilter.value || item.priority === priorityFilter.value)
  .filter((item) => !typeFilter.value || item.type === typeFilter.value)
  .filter((item) => {
    const kw = keyword.value.trim().toLowerCase()
    if (!kw) return true
    return `${item.content || ''} ${item.merchant?.name || ''} ${item.order_no || ''} ${sourceLabel(item.source)}`.toLowerCase().includes(kw)
  })
  .sort((a, b) => score(b) - score(a) || dateValue(a.next_follow_at) - dateValue(b.next_follow_at) || dateValue(b.created_at) - dateValue(a.created_at)))

const load = async () => {
  loading.value = true
  try {
    const res = await fetchAdminMerchantFollowUpTodos({})
    rows.value = res.data?.list || []
  } finally {
    loading.value = false
  }
}

const reset = () => {
  keyword.value = ''
  statusFilter.value = ''
  priorityFilter.value = ''
  typeFilter.value = ''
  quickMode.value = ''
}

const updateStatus = async (row, status) => {
  await updateAdminMerchantFollowUpStatus(row.id, { status })
  ElMessage.success(status === 'closed' ? '待办已完成' : '待办已重新打开')
  load()
}

const goSource = (row) => {
  if (row.source === 'order_exception') {
    router.push({ path: '/admin/orders', query: { keyword: row.order_no || '', exception: typeToException(row.type) } })
    return
  }
  router.push(`/admin/merchants/${row.merchant_id}`)
}

const matchQuick = (item) => {
  if (quickMode.value === 'open') return item.status !== 'closed'
  if (quickMode.value === 'closed') return item.status === 'closed'
  if (quickMode.value === 'priority') return item.status !== 'closed' && ['urgent', 'high'].includes(item.priority)
  if (quickMode.value === 'due') return item.status !== 'closed' && isDue(item)
  return true
}

const score = (item) => {
  const priorityScore = { urgent: 40, high: 30, normal: 20, low: 10 }[item.priority] || 0
  return (item.status === 'closed' ? -100 : 0) + priorityScore + (isDue(item) ? 15 : 0)
}
const isDue = (item) => Boolean(item.next_follow_at) && String(item.next_follow_at).slice(0, 10) <= today
const dateValue = (value) => value ? new Date(value).getTime() || 0 : 0
const formatTime = (value) => value ? String(value).replace('T', ' ').slice(0, 19) : '-'
const statusLabel = (value) => ({ open: '待处理', closed: '已处理' }[value] || value || '-')
const typeLabel = (value) => ({
  risk: '风险处理',
  settlement: '结算跟进',
  refund: '退款售后',
  payment: '支付异常',
  subscription: '订阅续费',
  operation: '运营沟通'
}[value] || value || '-')
const sourceLabel = (value) => ({
  order_exception: '订单异常',
  merchant_detail: '商家详情',
  manual: '人工创建'
}[value] || (value ? value : '人工创建'))
const typeToException = (value) => ({
  payment: 'payment',
  refund: 'refund',
  settlement: 'settlement',
  risk: ''
}[value] || '')
const priorityLabel = (value) => ({ low: '较低', normal: '普通', high: '较高', urgent: '紧急' }[value] || value || '-')
const priorityType = (value) => ({ low: 'info', normal: 'info', high: 'warning', urgent: 'danger' }[value] || 'info')

onMounted(load)
</script>

<style scoped>
.todos-page {
  display: grid;
  gap: 20px;
}

.metric-grid,
.quick-strip {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 18px;
}

.quick-strip {
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 12px;
}

.quick-strip button {
  display: grid;
  gap: 6px;
  min-height: 104px;
  padding: 14px;
  text-align: left;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #fff;
  cursor: pointer;
}

.quick-strip button.active {
  border-color: #2563eb;
  background: #eff6ff;
  box-shadow: 0 10px 26px rgba(37, 99, 235, 0.12);
}

.quick-strip span {
  color: #64748b;
  font-size: 13px;
  font-weight: 900;
}

.quick-strip strong {
  color: #0f172a;
  font-size: 24px;
}

.quick-strip small {
  color: #64748b;
  line-height: 1.45;
}

.filters-row,
.table-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
}

.filters-row {
  margin-bottom: 16px;
}

.filters-row :deep(.el-input),
.filters-row :deep(.el-select) {
  width: 220px;
}

.overdue {
  color: #dc2626;
  font-weight: 800;
}

.muted {
  color: #94a3b8;
}

@media (max-width: 1180px) {
  .metric-grid,
  .quick-strip {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 720px) {
  .metric-grid,
  .quick-strip {
    grid-template-columns: 1fr;
  }
}
</style>
