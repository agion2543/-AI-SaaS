<template>
  <div class="merchant-tickets">
    <section class="page-card hero">
      <div>
        <div class="eyebrow">SUPPORT</div>
        <h1>我的工单</h1>
        <p>遇到订单、支付、订阅、门店或账号问题，在这里提交给平台处理；处理结果以最新一条超管回复为准。如问题未解决，请再开一单并在标题注明“跟进 #N 单”。</p>
      </div>
      <el-button type="primary" :loading="loading" @click="loadList">刷新</el-button>
    </section>

    <section class="page-card">
      <div class="section-heading"><strong>提交新工单</strong><span>提交后请在下方列表查看处理进度与超管回复。</span></div>
      <el-form ref="formRef" :model="form" :rules="formRules" label-position="top" @submit.prevent>
        <el-form-item label="分类" prop="category">
          <el-select v-model="form.category" placeholder="请选择问题分类" clearable>
            <el-option v-for="category in supportTicketCategories" :key="category" :label="supportTicketCategoryLabel(category)" :value="category" />
          </el-select>
        </el-form-item>
        <el-form-item label="标题（120 字以内）" prop="title">
          <el-input v-model="form.title" maxlength="120" show-word-limit placeholder="例如：顾客已付款但订单仍显示待确认" />
        </el-form-item>
        <el-form-item label="内容（1000 字以内）" prop="content">
          <el-input v-model="form.content" type="textarea" :rows="4" maxlength="1000" show-word-limit placeholder="请写清楚发生了什么、涉及哪个订单或门店、希望平台帮你做什么" />
        </el-form-item>
        <el-button type="primary" :loading="submitting" :disabled="!isSupportTicketFormValid(form)" @click="submitTicket">提交工单</el-button>
      </el-form>
    </section>

    <section class="page-card">
      <div class="section-heading"><strong>我的工单</strong><el-tag type="info" effect="light">{{ total }} 单</el-tag></div>
      <el-table v-loading="loading" :data="rows" row-key="id" @row-click="openDetail">
        <el-table-column label="工单号" width="100">
          <template #default="{ row }"><strong>#{{ row.id }}</strong></template>
        </el-table-column>
        <el-table-column label="分类" width="100"><template #default="{ row }">{{ supportTicketCategoryLabel(row.category) }}</template></el-table-column>
        <el-table-column label="标题" min-width="220" show-overflow-tooltip>
          <template #default="{ row }"><strong>{{ row.title }}</strong></template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }"><el-tag :type="supportTicketStatusType(row.status)" size="small">{{ supportTicketStatusLabel(row.status) }}</el-tag></template>
        </el-table-column>
        <el-table-column label="超管回复" min-width="200" show-overflow-tooltip>
          <template #default="{ row }">{{ row.reply || '—' }}</template>
        </el-table-column>
        <el-table-column label="提交时间" min-width="165"><template #default="{ row }">{{ formatTime(row.created_at) }}</template></el-table-column>
        <template #empty><el-empty description="还没有提交过工单" /></template>
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
          <el-descriptions-item label="分类">{{ supportTicketCategoryLabel(detail.category) }}</el-descriptions-item>
          <el-descriptions-item label="标题">{{ detail.title }}</el-descriptions-item>
          <el-descriptions-item label="内容"><span class="pre-wrap">{{ detail.content }}</span></el-descriptions-item>
          <el-descriptions-item label="状态"><el-tag :type="supportTicketStatusType(detail.status)" size="small">{{ supportTicketStatusLabel(detail.status) }}</el-tag></el-descriptions-item>
          <el-descriptions-item label="提交时间">{{ formatTime(detail.created_at) }}</el-descriptions-item>
          <el-descriptions-item label="超管回复">
            <span v-if="detail.reply" class="pre-wrap">{{ detail.reply }}</span>
            <span v-else>暂无回复，请稍后再来查看</span>
            <small v-if="detail.replied_at">（{{ formatTime(detail.replied_at) }}）</small>
          </el-descriptions-item>
        </el-descriptions>
        <p class="dialog-hint">如问题未解决，请再开一单并在标题注明“跟进 #{{ detail.id }} 单”。</p>
      </div>
    </el-drawer>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import {
  createMerchantSupportTicket,
  fetchMerchantSupportTicket,
  fetchMerchantSupportTickets
} from '../../api/modules'
import {
  isSupportTicketFormValid,
  supportTicketCategories,
  supportTicketCategoryLabel,
  supportTicketStatusLabel,
  supportTicketStatusType
} from '../../utils/supportTickets'

const rows = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(10)
const loading = ref(false)
const submitting = ref(false)
const drawer = ref(false)
const detail = ref(null)
const formRef = ref(null)
const form = reactive({ title: '', content: '', category: '' })
const formRules = {
  title: [
    { required: true, message: '请填写工单标题', trigger: 'blur' },
    { max: 120, message: '工单标题不能超过 120 字', trigger: 'blur' }
  ],
  content: [
    { required: true, message: '请填写工单内容', trigger: 'blur' },
    { max: 1000, message: '工单内容不能超过 1000 字', trigger: 'blur' }
  ],
  category: [{ required: true, message: '请选择问题分类', trigger: 'change' }]
}

const loadList = async () => {
  loading.value = true
  try {
    const { data } = await fetchMerchantSupportTickets({ page: page.value, page_size: pageSize.value })
    rows.value = Array.isArray(data?.list) ? data.list : []
    total.value = Number(data?.total || 0)
  } catch (error) {
    ElMessage.error(error?.response?.data?.message || '加载工单列表失败')
  } finally {
    loading.value = false
  }
}

const handlePageChange = async (nextPage) => {
  page.value = nextPage
  await loadList()
}

const submitTicket = async () => {
  if (submitting.value) return
  if (!isSupportTicketFormValid(form)) return
  try { await formRef.value?.validate() } catch { return }
  submitting.value = true
  try {
    const { data } = await createMerchantSupportTicket({
      title: form.title.trim(),
      content: form.content.trim(),
      category: form.category
    })
    ElMessage.success(`工单 #${data?.ticket?.id || ''} 已提交，平台会按提交时间处理`)
    form.title = ''
    form.content = ''
    form.category = ''
    page.value = 1
    await loadList()
  } catch (error) {
    ElMessage.error(error?.response?.data?.message || '提交工单失败')
  } finally {
    submitting.value = false
  }
}

const openDetail = async (row) => {
  if (!row?.id) return
  try {
    const { data } = await fetchMerchantSupportTicket(row.id)
    detail.value = data?.ticket || row
  } catch {
    detail.value = row
  }
  drawer.value = true
}

const formatTime = (value) => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '—'
onMounted(loadList)
</script>

<style scoped>
.merchant-tickets{display:grid;gap:18px}.page-card{padding:20px;border:1px solid #dce8f5;border-radius:18px;background:#fff}.hero{display:flex;justify-content:space-between;align-items:center;gap:16px}.eyebrow{color:#2563eb;font-size:12px;font-weight:900;letter-spacing:.14em}.hero h1{margin:6px 0;font-size:24px}.hero p{margin:0;color:#64748b;font-size:13px;line-height:1.6}.section-heading{display:flex;justify-content:space-between;align-items:center;gap:12px;margin-bottom:14px}.section-heading strong{font-size:15px}.section-heading span{color:#7b8798;font-size:12px}.merchant-tickets :deep(.el-table){cursor:pointer}.table-pagination{justify-content:flex-end;margin-top:12px}.detail-content{display:grid;gap:14px;padding-bottom:28px}.detail-content .pre-wrap{white-space:pre-wrap}.dialog-hint{margin:0;color:#64748b;font-size:12px;line-height:1.6}.detail-content :deep(.el-descriptions__label){width:110px}
</style>
