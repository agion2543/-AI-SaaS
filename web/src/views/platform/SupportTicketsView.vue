<template>
  <div class="page-wrap">
    <header class="page-heading">
      <div>
        <p class="eyebrow">PLATFORM · SUPPORT TICKETS</p>
        <h1>服务工单</h1>
        <p class="intro">商家提交的服务请求按提交时间从早到晚处理；每次只保留最新一条超管回复，商家追问请开新单。商家提交后无推送，超管在此页查看未处理数。</p>
      </div>
      <div class="heading-actions">
        <el-tag effect="light" type="warning">仅超管可见</el-tag>
        <el-button type="primary" :loading="loading" @click="loadAll">刷新队列</el-button>
      </div>
    </header>

    <el-alert
      v-if="!isSuperAdmin"
      class="demo-alert"
      type="error"
      :closable="false"
      show-icon
      title="仅超管可见"
      description="当前账号不是超管，无权查看或处理服务工单。"
    />
    <template v-else>
      <section class="summary-grid">
        <article class="summary-card warning"><span>未处理</span><strong>{{ statusCounts.open }} 条</strong></article>
        <article class="summary-card"><span>处理中</span><strong>{{ statusCounts.in_progress }} 条</strong></article>
        <article class="summary-card"><span>已解决</span><strong>{{ statusCounts.resolved }} 条</strong></article>
        <article class="summary-card"><span>全部工单</span><strong>{{ statusCounts.total }} 条</strong></article>
      </section>

      <section class="quick-strip">
        <button
          v-for="item in quickFilters"
          :key="item.value || 'all'"
          type="button"
          :class="{ active: statusFilter === item.value }"
          @click="applyQuick(item.value)"
        >
          <span>{{ item.label }}</span>
          <strong>{{ item.count }}</strong>
        </button>
      </section>

      <section class="panel table-panel">
        <div class="table-heading">
          <div><strong>工单队列</strong><span>按提交时间从早到晚排列；行点击查看内容与回复。</span></div>
          <el-tag type="warning" effect="light">{{ total }} 条</el-tag>
        </div>
        <div class="filters-row">
          <el-select v-model="statusFilter" clearable placeholder="状态" @change="reloadList">
            <el-option label="待处理" value="open" />
            <el-option label="处理中" value="in_progress" />
            <el-option label="已解决" value="resolved" />
            <el-option label="已关闭" value="closed" />
          </el-select>
          <el-select v-model="categoryFilter" clearable placeholder="分类" @change="reloadList">
            <el-option v-for="category in supportTicketCategories" :key="category" :label="supportTicketCategoryLabel(category)" :value="category" />
          </el-select>
          <el-button @click="resetFilters">重置</el-button>
        </div>
        <el-table v-loading="loading" :data="rows" row-key="id" @row-click="openDetail">
          <el-table-column label="工单号" width="100">
            <template #default="{ row }"><strong>#{{ row.id }}</strong></template>
          </el-table-column>
          <el-table-column label="商家" min-width="160">
            <template #default="{ row }">
              <el-button text type="primary" @click.stop="router.push(`/admin/merchants/${row.merchant_id}`)">{{ row.merchant?.name || `商家 #${row.merchant_id}` }}</el-button>
            </template>
          </el-table-column>
          <el-table-column label="分类" width="100"><template #default="{ row }">{{ supportTicketCategoryLabel(row.category) }}</template></el-table-column>
          <el-table-column label="标题" min-width="220" show-overflow-tooltip>
            <template #default="{ row }"><strong>{{ row.title }}</strong></template>
          </el-table-column>
          <el-table-column label="状态" width="100">
            <template #default="{ row }"><el-tag :type="supportTicketStatusType(row.status)" size="small">{{ supportTicketStatusLabel(row.status) }}</el-tag></template>
          </el-table-column>
          <el-table-column label="提交时间" min-width="165"><template #default="{ row }">{{ formatTime(row.created_at) }}</template></el-table-column>
          <el-table-column label="操作" width="240" fixed="right">
            <template #default="{ row }">
              <el-button
                v-for="next in supportTicketNextStatuses(row.status)"
                :key="next"
                link
                :type="next === 'closed' ? 'danger' : next === 'resolved' ? 'success' : 'primary'"
                :disabled="acting"
                @click.stop="changeStatus(row, next)"
              >{{ supportTicketStatusActionLabel(next) }}</el-button>
              <el-button link type="primary" :disabled="acting" @click.stop="openReply(row)">回复</el-button>
            </template>
          </el-table-column>
          <template #empty><el-empty description="当前没有符合条件的工单" /></template>
        </el-table>
        <el-pagination
          v-if="total > pageSize"
          class="table-pagination"
          :current-page="page"
          :page-size="pageSize"
          :total="total"
          layout="total, prev, pager, next"
          @current-change="handlePageChange"
        />
      </section>

      <el-drawer v-model="drawer" title="工单详情" size="min(680px, 100%)" destroy-on-close>
        <div v-if="detail" class="detail-content">
          <el-descriptions :column="1" border>
            <el-descriptions-item label="工单号">#{{ detail.id }}</el-descriptions-item>
            <el-descriptions-item label="商家">{{ detail.merchant?.name || `商家 #${detail.merchant_id}` }}</el-descriptions-item>
            <el-descriptions-item label="分类">{{ supportTicketCategoryLabel(detail.category) }}</el-descriptions-item>
            <el-descriptions-item label="标题">{{ detail.title }}</el-descriptions-item>
            <el-descriptions-item label="内容"><span class="pre-wrap">{{ detail.content }}</span></el-descriptions-item>
            <el-descriptions-item label="状态"><el-tag :type="supportTicketStatusType(detail.status)" size="small">{{ supportTicketStatusLabel(detail.status) }}</el-tag></el-descriptions-item>
            <el-descriptions-item label="提交时间">{{ formatTime(detail.created_at) }}</el-descriptions-item>
            <el-descriptions-item label="最新回复">
              <span v-if="detail.reply" class="pre-wrap">{{ detail.reply }}</span>
              <span v-else>暂无回复</span>
              <small v-if="detail.replied_at">（超管 #{{ detail.replied_by }} · {{ formatTime(detail.replied_at) }}）</small>
            </el-descriptions-item>
          </el-descriptions>
          <p class="dialog-hint">只保留最新一条回复：再次回复会覆盖上面这条，旧回复被覆盖后不再显示；商家追问须开新单。</p>
          <div class="outcome-buttons">
            <el-button
              v-for="next in supportTicketNextStatuses(detail.status)"
              :key="next"
              :type="next === 'closed' ? 'danger' : next === 'resolved' ? 'success' : 'primary'"
              :plain="next === 'closed'"
              :loading="acting"
              @click="changeStatus(detail, next)"
            >{{ supportTicketStatusActionLabel(next) }}</el-button>
            <el-button type="primary" plain :disabled="acting || detail.status === 'closed'" @click="openReply(detail)">回复工单</el-button>
          </div>
        </div>
      </el-drawer>

      <el-dialog v-model="replyDialog" title="回复工单" width="min(520px, 92vw)" destroy-on-close>
        <el-form ref="replyFormRef" :model="replyForm" :rules="replyRules" label-position="top" @submit.prevent>
          <el-form-item label="回复内容（必填，1000 字以内）" prop="reply">
            <el-input v-model="replyForm.reply" type="textarea" :rows="4" maxlength="1000" show-word-limit placeholder="例如：已核查订单收款记录，收款已确认，订单状态已更新" />
          </el-form-item>
        </el-form>
        <p class="dialog-hint">提交后覆盖该工单此前的回复，旧回复不再显示；商家追问须开新单。</p>
        <template #footer>
          <el-button :disabled="acting" @click="replyDialog = false">取消</el-button>
          <el-button type="primary" :loading="acting" :disabled="!isSupportTicketReplyValid(replyForm.reply)" @click="submitReply">确认回复</el-button>
        </template>
      </el-dialog>
    </template>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  fetchAdminSupportTicket,
  fetchAdminSupportTickets,
  replyAdminSupportTicket,
  updateAdminSupportTicketStatus
} from '../../api/modules'
import { useAuthStore } from '../../stores/auth'
import {
  isSupportTicketReplyValid,
  supportTicketCategories,
  supportTicketCategoryLabel,
  supportTicketNextStatuses,
  supportTicketStatusActionLabel,
  supportTicketStatusLabel,
  supportTicketStatusType
} from '../../utils/supportTickets'

const router = useRouter()
const authStore = useAuthStore()
const isSuperAdmin = computed(() => authStore.isSuperAdmin)
const rows = ref([])
const total = ref(0)
const statusCounts = ref({ open: 0, in_progress: 0, resolved: 0, closed: 0, total: 0 })
const statusFilter = ref('')
const categoryFilter = ref('')
const page = ref(1)
const pageSize = ref(10)
const loading = ref(false)
const acting = ref(false)
const drawer = ref(false)
const detail = ref(null)
const replyDialog = ref(false)
const replyRow = ref(null)
const replyFormRef = ref(null)
const replyForm = reactive({ reply: '' })
const replyRules = {
  reply: [
    { required: true, message: '请填写回复内容', trigger: 'blur' },
    { max: 1000, message: '回复内容不能超过 1000 字', trigger: 'blur' }
  ]
}

const quickFilters = computed(() => [
  { label: '全部工单', value: '', count: statusCounts.value.total },
  { label: '待处理', value: 'open', count: statusCounts.value.open },
  { label: '处理中', value: 'in_progress', count: statusCounts.value.in_progress },
  { label: '已解决', value: 'resolved', count: statusCounts.value.resolved },
  { label: '已关闭', value: 'closed', count: statusCounts.value.closed }
])

const loadList = async () => {
  loading.value = true
  try {
    const { data } = await fetchAdminSupportTickets({
      status: statusFilter.value || undefined,
      category: categoryFilter.value || undefined,
      page: page.value,
      page_size: pageSize.value
    })
    rows.value = Array.isArray(data?.list) ? data.list : []
    total.value = Number(data?.total || 0)
  } catch (error) {
    ElMessage.error(error?.response?.data?.message || '加载工单列表失败')
  } finally {
    loading.value = false
  }
}

const loadCounts = async () => {
  const statuses = ['open', 'in_progress', 'resolved', 'closed']
  const results = await Promise.all(statuses.map(async (status) => {
    try {
      const { data } = await fetchAdminSupportTickets({ status, page: 1, page_size: 1 })
      return Number(data?.total || 0)
    } catch {
      return 0
    }
  }))
  statusCounts.value = {
    open: results[0],
    in_progress: results[1],
    resolved: results[2],
    closed: results[3],
    total: results[0] + results[1] + results[2] + results[3]
  }
}

const loadAll = async () => { await Promise.all([loadList(), loadCounts()]) }
const reloadList = async () => { page.value = 1; await loadList() }
const handlePageChange = async (nextPage) => { page.value = nextPage; await loadList() }

const applyQuick = async (value) => {
  statusFilter.value = value
  page.value = 1
  await loadList()
}

const resetFilters = async () => {
  statusFilter.value = ''
  categoryFilter.value = ''
  page.value = 1
  await loadList()
}

const openDetail = async (row) => {
  if (!row?.id) return
  try {
    const { data } = await fetchAdminSupportTicket(row.id)
    detail.value = data?.ticket || row
  } catch {
    detail.value = row
  }
  drawer.value = true
}

const changeStatus = async (row, next) => {
  if (!row?.id || acting.value) return
  try {
    await ElMessageBox.confirm(
      `将工单 #${row.id}「${row.title}」改为「${supportTicketStatusLabel(next)}」？`,
      `${supportTicketStatusActionLabel(next)}？`,
      { type: 'warning', confirmButtonText: '确认', cancelButtonText: '返回检查' }
    )
  } catch { return }
  acting.value = true
  try {
    await updateAdminSupportTicketStatus(row.id, { status: next })
    ElMessage.success(`工单已改为「${supportTicketStatusLabel(next)}」`)
    if (detail.value?.id === row.id) {
      const { data } = await fetchAdminSupportTicket(row.id)
      detail.value = data?.ticket || detail.value
    }
    await loadAll()
  } catch (error) {
    await loadList()
    ElMessage.error(error?.response?.data?.message || '工单状态修改失败；请以工单列表为准，不要直接重试')
  } finally {
    acting.value = false
  }
}

const openReply = (row) => {
  if (!row?.id) return
  replyRow.value = row
  replyForm.reply = ''
  replyDialog.value = true
}

const submitReply = async () => {
  if (!replyRow.value || acting.value) return
  if (!isSupportTicketReplyValid(replyForm.reply)) return
  try { await replyFormRef.value?.validate() } catch { return }
  const reply = replyForm.reply.trim()
  acting.value = true
  try {
    await replyAdminSupportTicket(replyRow.value.id, { reply })
    ElMessage.success('已回复该工单')
    replyDialog.value = false
    if (detail.value?.id === replyRow.value.id) {
      const { data } = await fetchAdminSupportTicket(replyRow.value.id)
      detail.value = data?.ticket || detail.value
    }
    await loadAll()
  } catch (error) {
    await loadList()
    ElMessage.error(error?.response?.data?.message || '回复失败；请以工单列表为准')
  } finally {
    acting.value = false
  }
}

const formatTime = (value) => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '—'
onMounted(loadAll)
</script>

<style scoped>
.page-wrap{display:grid;gap:16px;color:#172b4d}.page-heading{display:flex;justify-content:space-between;align-items:flex-start;gap:16px}.heading-actions{display:flex;align-items:center;gap:10px}.eyebrow{margin:0 0 6px;color:#2563eb;font-size:11px;font-weight:800;letter-spacing:.13em}.page-heading h1{margin:0;font-size:27px}.intro{margin:7px 0 0;color:#64748b}.demo-alert{border-radius:13px}.summary-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:10px}.summary-card,.panel{background:#fff;border:1px solid #e2eaf4;border-radius:15px;box-shadow:0 5px 18px rgba(15,23,42,.035)}.summary-card{padding:14px 16px;display:grid;gap:7px;align-content:start}.summary-card span{font-size:12px;color:#64748b}.summary-card strong{font-size:22px;color:#17365d}.summary-card.warning strong{color:#b45309}.quick-strip{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:12px}.quick-strip button{display:grid;gap:6px;min-height:84px;padding:14px;text-align:left;border:1px solid #e2e8f0;border-radius:8px;background:#fff;cursor:pointer}.quick-strip button.active{border-color:#2563eb;background:#eff6ff;box-shadow:0 10px 26px rgba(37,99,235,.12)}.quick-strip span{color:#64748b;font-size:13px;font-weight:900}.quick-strip strong{color:#0f172a;font-size:24px}.table-panel{padding:8px 12px 14px}.table-heading{display:flex;justify-content:space-between;align-items:center;gap:12px;padding:9px 6px 13px}.table-heading div{display:grid;gap:5px}.table-heading strong{font-size:15px}.table-heading span{color:#7b8798;font-size:12px}.filters-row{display:flex;flex-wrap:wrap;gap:10px;align-items:center;margin-bottom:12px}.filters-row :deep(.el-select){width:180px}.table-panel :deep(.el-table){cursor:pointer}.table-panel :deep(.el-table td){padding:11px 0}.table-panel small{display:block;margin-top:5px;color:#8491a4;font-size:11px;line-height:1.5}.table-pagination{justify-content:flex-end;margin-top:12px}.detail-content{display:grid;gap:14px;padding-bottom:28px}.detail-content .pre-wrap{white-space:pre-wrap}.outcome-buttons{display:flex;flex-wrap:wrap;gap:8px}.dialog-hint{margin:0;color:#64748b;font-size:12px;line-height:1.6}@media(max-width:960px){.summary-grid,.quick-strip{grid-template-columns:repeat(2,minmax(0,1fr))}}@media(max-width:760px){.page-heading{display:grid}.page-heading h1{font-size:23px}.summary-grid,.quick-strip{grid-template-columns:1fr}.table-panel{overflow:hidden;padding:6px}.table-panel :deep(.el-table){font-size:12px}.table-heading{align-items:flex-start}.outcome-buttons{display:grid}.outcome-buttons .el-button{margin:0}.detail-content :deep(.el-descriptions__label){width:110px}}
</style>
