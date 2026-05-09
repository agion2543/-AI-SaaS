<template>
  <div class="orders-stack">
    <section class="page-card hero-card">
      <div>
        <div class="eyebrow">ORDER CENTER</div>
        <h2 class="page-title">订单管理</h2>
        <p class="muted">这里只展示顾客扫码点单订单，不包含商家订阅付费订单，方便商家接单、核对和售后处理。</p>
      </div>
      <el-button type="primary" :loading="loading" @click="reload">刷新订单</el-button>
    </section>

    <section class="stat-grid">
      <div class="stat-card">
        <span>当前结果</span>
        <strong>{{ total }}</strong>
      </div>
      <div class="stat-card">
        <span>待接单</span>
        <strong>{{ pageStatusCount.received }}</strong>
      </div>
      <div class="stat-card">
        <span>已接单</span>
        <strong>{{ pageStatusCount.accepted }}</strong>
      </div>
      <div class="stat-card highlight">
        <span>当前页净实收</span>
        <strong>{{ formatMoney(pageNetReceived) }}</strong>
      </div>
    </section>

    <section class="page-card table-card">
      <div class="filter-row">
        <el-input
          v-model="filters.keyword"
          clearable
          placeholder="搜索订单号、顾客手机号、商品"
          class="keyword-input"
          @keyup.enter="reload"
        />
        <el-select v-model="filters.store_id" clearable placeholder="全部门店" class="filter-item" @change="reload">
          <el-option v-for="store in stores" :key="store.id" :label="store.name" :value="store.id" />
        </el-select>
        <el-select v-model="filters.status" clearable placeholder="全部状态" class="filter-item" @change="reload">
          <el-option label="待支付" value="pending" />
          <el-option label="待接单" value="received" />
          <el-option label="已接单" value="accepted" />
          <el-option label="已完成" value="completed" />
          <el-option label="已关闭" value="closed" />
        </el-select>
        <el-button type="primary" @click="reload">查询</el-button>
        <el-button @click="resetFilters">重置</el-button>
      </div>

      <el-alert
        class="order-tip"
        type="info"
        show-icon
        :closable="false"
        title="净实收 = 已支付扫码点单金额 - 已记录退款金额；待支付订单不计入实收。"
      />

      <el-table v-loading="loading" :data="orders" empty-text="暂无扫码点单订单">
        <el-table-column label="订单号" min-width="190">
          <template #default="{ row }">
            <el-link type="primary" @click="goDetail(row)">{{ row.order_no }}</el-link>
          </template>
        </el-table-column>
        <el-table-column label="门店" min-width="130">
          <template #default="{ row }">{{ row.store?.name || '-' }}</template>
        </el-table-column>
        <el-table-column label="顾客" min-width="130">
          <template #default="{ row }">{{ row.customer_phone || '-' }}</template>
        </el-table-column>
        <el-table-column label="商品" min-width="260">
          <template #default="{ row }">
            <div class="items-preview">
              <div v-for="item in row.items?.slice(0, 3) || []" :key="`${row.id}-${item.product_id}-${item.name}`">
                {{ item.name }} x {{ item.quantity }}
              </div>
              <span v-if="(row.items?.length || 0) > 3" class="muted">还有 {{ row.items.length - 3 }} 项</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="订单金额" width="130">
          <template #default="{ row }">
            <strong>{{ formatMoney(row.total_amount || row.amount) }}</strong>
          </template>
        </el-table-column>
        <el-table-column label="已退款" width="120">
          <template #default="{ row }">{{ formatMoney(row.refunded_amount) }}</template>
        </el-table-column>
        <el-table-column label="净实收" width="120">
          <template #default="{ row }">
            <strong>{{ formatMoney(netReceived(row)) }}</strong>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)">{{ statusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="备注" min-width="150" show-overflow-tooltip>
          <template #default="{ row }">{{ row.merchant_note || '-' }}</template>
        </el-table-column>
        <el-table-column label="创建时间" min-width="170">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="360" fixed="right">
          <template #default="{ row }">
            <div class="action-row">
              <el-button size="small" @click="goDetail(row)">详情</el-button>
              <el-button v-if="row.status === 'received'" size="small" type="primary" @click="accept(row)">接单</el-button>
              <el-button v-if="row.status === 'accepted'" size="small" type="success" @click="complete(row)">完成</el-button>
              <el-button v-if="canClose(row)" size="small" type="danger" plain @click="close(row)">关闭</el-button>
              <el-button size="small" @click="openNote(row)">备注</el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <div class="orders-pagination">
        <el-pagination
          background
          layout="total, prev, pager, next"
          :total="total"
          :page-size="pageSize"
          :current-page="page"
          @current-change="changePage"
        />
      </div>
    </section>

    <el-dialog v-model="noteDialogVisible" title="订单备注" width="520px">
      <el-input v-model="noteForm.merchant_note" type="textarea" :rows="4" maxlength="500" show-word-limit />
      <template #footer>
        <el-button @click="noteDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveNote">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  acceptMerchantOrder,
  closeMerchantOrder,
  completeMerchantOrder,
  fetchMerchantOrders,
  fetchMerchantStores,
  updateMerchantOrderNote
} from '../../api/modules'

const router = useRouter()
const orders = ref([])
const stores = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 10
const loading = ref(false)
const noteDialogVisible = ref(false)
const editingOrderId = ref(null)
const noteForm = reactive({ merchant_note: '' })
const filters = reactive({ keyword: '', status: '', store_id: undefined })

const paidStatuses = ['received', 'accepted', 'completed', 'closed']
const pageNetReceived = computed(() => orders.value.reduce((sum, item) => sum + netReceived(item), 0))
const pageStatusCount = computed(() => orders.value.reduce((acc, item) => {
  acc[item.status] = (acc[item.status] || 0) + 1
  return acc
}, { received: 0, accepted: 0 }))

const netReceived = (row) => {
  if (!paidStatuses.includes(row.status)) return 0
  return Math.max(Number(row.total_amount || row.amount || 0) - Number(row.refunded_amount || 0), 0)
}

const loadStores = async () => {
  const res = await fetchMerchantStores({ page: 1, page_size: 100 })
  stores.value = res.data.list || []
}

const loadOrders = async () => {
  loading.value = true
  try {
    const params = { page: page.value, page_size: pageSize }
    if (filters.keyword) params.keyword = filters.keyword
    if (filters.status) params.status = filters.status
    if (filters.store_id) params.store_id = filters.store_id
    const res = await fetchMerchantOrders(params)
    orders.value = res.data.list || []
    total.value = res.data.total || 0
  } finally {
    loading.value = false
  }
}

const reload = async () => {
  page.value = 1
  await loadOrders()
}

const resetFilters = async () => {
  filters.keyword = ''
  filters.status = ''
  filters.store_id = undefined
  await reload()
}

const changePage = async (nextPage) => {
  page.value = nextPage
  await loadOrders()
}

const goDetail = (row) => {
  router.push(`/merchant/orders/${row.id}`)
}

const accept = async (row) => {
  await acceptMerchantOrder(row.id)
  ElMessage.success('订单已接单')
  await loadOrders()
}

const complete = async (row) => {
  await completeMerchantOrder(row.id)
  ElMessage.success('订单已完成')
  await loadOrders()
}

const close = async (row) => {
  await ElMessageBox.confirm('确认关闭这笔订单吗？关闭后不会计入待接单。', '关闭订单', { type: 'warning' })
  await closeMerchantOrder(row.id)
  ElMessage.success('订单已关闭')
  await loadOrders()
}

const openNote = (row) => {
  editingOrderId.value = row.id
  noteForm.merchant_note = row.merchant_note || ''
  noteDialogVisible.value = true
}

const saveNote = async () => {
  await updateMerchantOrderNote(editingOrderId.value, { merchant_note: noteForm.merchant_note })
  ElMessage.success('备注已保存')
  noteDialogVisible.value = false
  await loadOrders()
}

const canClose = (row) => ['received', 'accepted'].includes(row.status)
const formatMoney = (value) => `¥${(Number(value || 0) / 100).toFixed(2)}`
const formatTime = (value) => value ? String(value).replace('T', ' ').slice(0, 19) : '-'
const statusLabel = (status) => ({
  pending: '待支付',
  received: '待接单',
  accepted: '已接单',
  completed: '已完成',
  closed: '已关闭',
  failed: '支付失败'
}[status] || status || '-')
const statusType = (status) => ({
  pending: 'warning',
  received: 'primary',
  accepted: 'success',
  completed: 'success',
  closed: 'info',
  failed: 'danger'
}[status] || 'info')

onMounted(async () => {
  await Promise.all([loadStores(), loadOrders()])
})
</script>

<style scoped>
.orders-stack {
  display: grid;
  gap: 18px;
}

.hero-card {
  display: flex;
  justify-content: space-between;
  gap: 18px;
  align-items: flex-start;
  padding: 22px;
}

.eyebrow {
  color: #0ea5e9;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.16em;
}

.muted {
  margin: 0;
  color: var(--muted);
  line-height: 1.7;
}

.stat-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 14px;
}

.stat-card {
  padding: 18px;
  border: 1px solid #e5edf9;
  border-radius: 16px;
  background: #f8fbff;
}

.stat-card.highlight {
  background: linear-gradient(135deg, #ecfdf5, #ffffff);
  border-color: #bbf7d0;
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

.filter-row,
.action-row {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.filter-row {
  margin-bottom: 12px;
}

.order-tip {
  margin-bottom: 14px;
}

.keyword-input {
  width: 280px;
}

.filter-item {
  width: 160px;
}

.items-preview {
  line-height: 1.7;
}

.orders-pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 18px;
}

@media (max-width: 860px) {
  .hero-card {
    flex-direction: column;
  }
}
</style>
