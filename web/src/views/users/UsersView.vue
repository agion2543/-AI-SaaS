<template>
  <div class="customers-stack">
    <section class="page-card hero-card">
      <div>
        <div class="eyebrow">CUSTOMER DATA</div>
        <h2 class="page-title">顾客数据</h2>
        <p class="muted">沉淀扫码点单顾客，按手机号、商家、门店、消费次数和 AI 标签进行管理。</p>
      </div>
      <el-button type="primary" :loading="loading" @click="load">刷新数据</el-button>
    </section>

    <section class="page-card filter-card">
      <div class="filter-grid">
        <el-input
          v-model="filters.keyword"
          clearable
          placeholder="搜索手机号 / 商家名称 / 门店名称"
          @keyup.enter="search"
        />
        <el-select v-model="filters.ai_tag" clearable placeholder="AI 顾客标签" @change="search">
          <el-option label="新顾客" value="新顾客" />
          <el-option label="复购顾客" value="复购顾客" />
          <el-option label="高价值顾客" value="高价值顾客" />
          <el-option label="沉睡顾客" value="沉睡顾客" />
          <el-option label="流失风险" value="流失风险" />
        </el-select>
        <el-select v-model="filters.status" clearable placeholder="最近订单状态" @change="search">
          <el-option label="待接单" value="received" />
          <el-option label="已接单" value="accepted" />
          <el-option label="已完成" value="completed" />
          <el-option label="已关闭" value="closed" />
        </el-select>
        <el-select v-model="filters.sort_by" placeholder="排序字段" @change="search">
          <el-option label="最近下单时间" value="last_order_at" />
          <el-option label="累计消费" value="total_amount" />
          <el-option label="消费次数" value="order_count" />
          <el-option label="优惠金额" value="discount_amount" />
          <el-option label="手机号" value="customer_phone" />
        </el-select>
        <el-select v-model="filters.sort_order" placeholder="排序方式" @change="search">
          <el-option label="降序" value="desc" />
          <el-option label="升序" value="asc" />
        </el-select>
        <div class="filter-actions">
          <el-button type="primary" @click="search">查询</el-button>
          <el-button @click="resetFilters">重置</el-button>
        </div>
      </div>
    </section>

    <section class="stat-grid">
      <div class="stat-card">
        <span>顾客档案</span>
        <strong>{{ total }}</strong>
      </div>
      <div class="stat-card">
        <span>当前页消费次数</span>
        <strong>{{ pageOrderCount }}</strong>
      </div>
      <div class="stat-card">
        <span>当前页累计消费</span>
        <strong>¥{{ formatYuan(pageTotalAmount) }}</strong>
      </div>
      <div class="stat-card">
        <span>当前页优惠金额</span>
        <strong>¥{{ formatYuan(pageDiscountAmount) }}</strong>
      </div>
    </section>

    <section class="page-card table-card">
      <el-table
        v-loading="loading"
        :data="customers"
        empty-text="暂无顾客数据"
        @sort-change="handleTableSort"
      >
        <el-table-column label="排名" width="80">
          <template #default="{ $index }">#{{ (page - 1) * pageSize + $index + 1 }}</template>
        </el-table-column>
        <el-table-column prop="customer_phone" label="顾客手机号" min-width="140" sortable="custom" />
        <el-table-column prop="merchant_name" label="所属商家" min-width="150" />
        <el-table-column prop="store_name" label="最近关联门店" min-width="150" />
        <el-table-column prop="order_count" label="消费次数" width="110" sortable="custom" />
        <el-table-column prop="total_amount" label="累计消费" width="120" sortable="custom">
          <template #default="{ row }">¥{{ formatYuan(row.total_amount) }}</template>
        </el-table-column>
        <el-table-column prop="discount_amount" label="累计优惠" width="120" sortable="custom">
          <template #default="{ row }">¥{{ formatYuan(row.discount_amount) }}</template>
        </el-table-column>
        <el-table-column label="AI 标签" width="120">
          <template #default="{ row }">
            <el-tag :type="tagType(row.ai_tag)">{{ row.ai_tag || '待分析' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="ai_suggestion" label="AI 运营建议" min-width="260" show-overflow-tooltip />
        <el-table-column label="最近状态" width="120">
          <template #default="{ row }">
            <el-tag :type="statusType(row.last_status)">{{ statusLabel(row.last_status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="last_order_at" label="最近下单时间" min-width="170" sortable="custom">
          <template #default="{ row }">{{ formatTime(row.last_order_at) }}</template>
        </el-table-column>
      </el-table>

      <div class="pager">
        <el-pagination
          background
          layout="total, sizes, prev, pager, next"
          :total="total"
          :page-size="pageSize"
          :current-page="page"
          :page-sizes="[10, 20, 50]"
          @size-change="handleSizeChange"
          @current-change="handlePageChange"
        />
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { fetchCustomerProfiles } from '../../api/modules'

const customers = ref([])
const loading = ref(false)
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const filters = reactive({
  keyword: '',
  ai_tag: '',
  status: '',
  sort_by: 'last_order_at',
  sort_order: 'desc'
})

const pageOrderCount = computed(() => customers.value.reduce((sum, item) => sum + Number(item.order_count || 0), 0))
const pageTotalAmount = computed(() => customers.value.reduce((sum, item) => sum + Number(item.total_amount || 0), 0))
const pageDiscountAmount = computed(() => customers.value.reduce((sum, item) => sum + Number(item.discount_amount || 0), 0))

const load = async () => {
  loading.value = true
  try {
    const res = await fetchCustomerProfiles({
      page: page.value,
      page_size: pageSize.value,
      keyword: filters.keyword,
      ai_tag: filters.ai_tag,
      status: filters.status,
      sort_by: filters.sort_by,
      sort_order: filters.sort_order
    })
    customers.value = res.data.list || []
    total.value = res.data.total || 0
  } finally {
    loading.value = false
  }
}

const search = () => {
  page.value = 1
  load()
}

const resetFilters = () => {
  filters.keyword = ''
  filters.ai_tag = ''
  filters.status = ''
  filters.sort_by = 'last_order_at'
  filters.sort_order = 'desc'
  search()
}

const handleTableSort = ({ prop, order }) => {
  if (!prop) return
  filters.sort_by = prop
  filters.sort_order = order === 'ascending' ? 'asc' : 'desc'
  search()
}

const handlePageChange = (nextPage) => {
  page.value = nextPage
  load()
}

const handleSizeChange = (nextSize) => {
  pageSize.value = nextSize
  page.value = 1
  load()
}

const formatYuan = (value) => (Number(value || 0) / 100).toFixed(2)
const formatTime = (value) => value ? String(value).replace('T', ' ').slice(0, 19) : '-'
const statusLabel = (status) => ({
  received: '待接单',
  accepted: '已接单',
  completed: '已完成',
  closed: '已关闭'
}[status] || status || '-')
const statusType = (status) => ({
  received: 'primary',
  accepted: 'success',
  completed: 'success',
  closed: 'info'
}[status] || 'info')
const tagType = (tag) => ({
  高价值顾客: 'danger',
  复购顾客: 'success',
  沉睡顾客: 'warning',
  流失风险: 'warning',
  新顾客: 'primary'
}[tag] || 'info')

onMounted(load)
</script>

<style scoped>
.customers-stack {
  display: grid;
  gap: 20px;
}

.hero-card {
  display: flex;
  justify-content: space-between;
  gap: 20px;
  align-items: flex-start;
  padding: 24px;
}

.eyebrow {
  color: #0284c7;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.16em;
}

.muted {
  color: #64748b;
  margin: 0;
  line-height: 1.7;
}

.filter-card,
.table-card {
  padding: 20px;
}

.filter-grid {
  display: grid;
  grid-template-columns: minmax(260px, 1.6fr) repeat(4, minmax(140px, 1fr)) auto;
  gap: 10px;
  align-items: center;
}

.filter-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
}

.stat-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 14px;
}

.stat-card {
  padding: 16px;
  border-radius: 14px;
  background: #f8fbff;
  border: 1px solid #e5edf9;
}

.stat-card span,
.stat-card strong {
  display: block;
}

.stat-card span {
  color: var(--muted);
  font-size: 13px;
}

.stat-card strong {
  margin-top: 8px;
  font-size: 26px;
}

.pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

@media (max-width: 1100px) {
  .filter-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 720px) {
  .hero-card {
    flex-direction: column;
  }

  .filter-grid {
    grid-template-columns: 1fr;
  }
}
</style>
