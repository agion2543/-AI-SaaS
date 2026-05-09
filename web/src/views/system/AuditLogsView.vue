<template>
  <div class="audit-page">
    <section class="page-card hero">
      <div>
        <div class="eyebrow">AUDIT TRAIL</div>
        <h1>操作审计</h1>
        <p>记录平台关键操作：收款审核、商家开通、冻结、退款与会员调整，方便后续追责和风控复盘。</p>
      </div>
      <el-button type="primary" @click="loadLogs">刷新日志</el-button>
    </section>

    <section class="page-card filters">
      <el-input v-model="filters.keyword" clearable placeholder="搜索操作人 / 对象 / 动作" />
      <el-select v-model="filters.action" clearable placeholder="操作类型">
        <el-option label="收款审核" value="merchant_payment_config_review" />
        <el-option label="商家状态调整" value="merchant_status_update" />
        <el-option label="开通商家订阅" value="merchant_subscription_open" />
        <el-option label="停止商家订阅" value="merchant_subscription_stop" />
        <el-option label="平台退款" value="admin_order_refund" />
        <el-option label="商家退款" value="merchant_order_refund" />
        <el-option label="用户状态调整" value="user_status_update" />
        <el-option label="会员权益调整" value="user_member_adjust" />
      </el-select>
      <el-select v-model="filters.target_type" clearable placeholder="对象类型">
        <el-option label="商家" value="merchant" />
        <el-option label="收款配置" value="merchant_payment_config" />
        <el-option label="订单" value="order" />
        <el-option label="用户" value="user" />
      </el-select>
      <el-button type="primary" @click="search">查询</el-button>
      <el-button @click="reset">重置</el-button>
    </section>

    <section class="page-card">
      <el-table :data="logs" v-loading="loading" stripe>
        <el-table-column label="时间" min-width="170">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column prop="actor_name" label="操作人" min-width="120" />
        <el-table-column label="身份" width="110">
          <template #default="{ row }">{{ actorLabel(row.actor_type) }}</template>
        </el-table-column>
        <el-table-column label="操作" min-width="150">
          <template #default="{ row }">
            <el-tag effect="plain">{{ actionLabel(row.action) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="对象" min-width="180">
          <template #default="{ row }">
            <div class="target-name">{{ row.target_name || '-' }}</div>
            <div class="muted">{{ targetLabel(row.target_type) }} #{{ row.target_id || '-' }}</div>
          </template>
        </el-table-column>
        <el-table-column label="商家ID" width="90">
          <template #default="{ row }">{{ row.merchant_id || '-' }}</template>
        </el-table-column>
        <el-table-column prop="ip" label="IP" min-width="120" />
        <el-table-column label="详情" min-width="240">
          <template #default="{ row }">
            <pre class="detail">{{ stringifyDetail(row.detail) }}</pre>
          </template>
        </el-table-column>
      </el-table>

      <div class="pager">
        <el-pagination
          background
          layout="total, prev, pager, next"
          :total="total"
          :page-size="pagination.page_size"
          v-model:current-page="pagination.page"
          @current-change="loadLogs"
        />
      </div>
    </section>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { fetchAuditLogs } from '../../api/modules'

const loading = ref(false)
const logs = ref([])
const total = ref(0)

const filters = reactive({
  keyword: '',
  action: '',
  target_type: ''
})

const pagination = reactive({
  page: 1,
  page_size: 20
})

const actionText = {
  merchant_payment_config_review: '收款审核',
  merchant_status_update: '商家状态调整',
  merchant_subscription_open: '开通商家订阅',
  merchant_subscription_stop: '停止商家订阅',
  admin_order_refund: '平台退款',
  merchant_order_refund: '商家退款',
  user_status_update: '用户状态调整',
  user_member_adjust: '会员权益调整'
}

const targetText = {
  merchant: '商家',
  merchant_payment_config: '收款配置',
  order: '订单',
  user: '用户'
}

const actorText = {
  super_admin: '平台管理员',
  admin: '管理员',
  merchant_admin: '商家'
}

const loadLogs = async () => {
  loading.value = true
  try {
    const res = await fetchAuditLogs({
      ...filters,
      page: pagination.page,
      page_size: pagination.page_size
    })
    logs.value = res.data.list || []
    total.value = res.data.total || 0
  } catch (error) {
    ElMessage.error(error?.response?.data?.message || '审计日志加载失败')
  } finally {
    loading.value = false
  }
}

const search = () => {
  pagination.page = 1
  loadLogs()
}

const reset = () => {
  filters.keyword = ''
  filters.action = ''
  filters.target_type = ''
  pagination.page = 1
  loadLogs()
}

const formatTime = (value) => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '-'
const actionLabel = (value) => actionText[value] || value || '-'
const targetLabel = (value) => targetText[value] || value || '-'
const actorLabel = (value) => actorText[value] || value || '-'

const stringifyDetail = (value) => {
  if (!value) return '-'
  if (typeof value === 'object') return JSON.stringify(value, null, 2)
  try {
    return JSON.stringify(JSON.parse(value), null, 2)
  } catch {
    return value
  }
}

onMounted(loadLogs)
</script>

<style scoped>
.audit-page {
  display: grid;
  gap: 18px;
}

.hero {
  display: flex;
  justify-content: space-between;
  gap: 18px;
  align-items: center;
  padding: 26px;
}

.eyebrow {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.18em;
}

h1 {
  margin: 6px 0 10px;
  font-size: 28px;
}

p,
.muted {
  color: #64748b;
}

.filters {
  display: grid;
  grid-template-columns: minmax(220px, 1fr) 180px 180px auto auto;
  gap: 10px;
  align-items: center;
  padding: 16px;
}

.target-name {
  font-weight: 800;
}

.detail {
  max-height: 120px;
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  color: #334155;
  font-size: 12px;
  font-family: "JetBrains Mono", Consolas, monospace;
}

.pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

@media (max-width: 900px) {
  .hero,
  .filters {
    grid-template-columns: 1fr;
    display: grid;
  }
}
</style>
