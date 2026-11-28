<template>
  <div class="page-wrap">
    <header class="page-heading">
      <div>
        <p class="eyebrow">PLATFORM · MERCHANT REVIEW</p>
        <h1>商家入驻审核</h1>
        <p class="intro">新注册商家处于待审核状态，需超管通过后才能开通订阅并营业；拒绝后商家保持待审核，可修改资料后重新提交。</p>
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
      description="当前账号不是超管，无权查看或处理商家入驻审核。"
    />
    <template v-else>
      <section class="summary-grid">
        <article class="summary-card warning"><span>待审核商家</span><strong>{{ total }} 个</strong></article>
        <article class="summary-card"><span>商家总数</span><strong>{{ merchantStats.total }} 个</strong></article>
        <article class="summary-card"><span>处理原则</span><strong class="principle">通过即营业 · 拒绝须留原因</strong></article>
      </section>

      <section class="panel table-panel">
        <div class="table-heading">
          <div><strong>待审核队列</strong><span>按注册时间从早到晚排列；通过后商家状态变为正常，可开通订阅并营业。</span></div>
          <el-tag type="warning" effect="light">{{ total }} 个待审核</el-tag>
        </div>
        <el-table v-loading="loading" :data="rows" row-key="id" @row-click="openDetail">
          <el-table-column label="商家名称" min-width="180">
            <template #default="{ row }"><strong>{{ row.name }}</strong><small>商家 #{{ row.id }}</small></template>
          </el-table-column>
          <el-table-column label="联系电话" width="150"><template #default="{ row }">{{ row.contact_phone || '—' }}</template></el-table-column>
          <el-table-column label="注册时间" min-width="165"><template #default="{ row }">{{ formatTime(row.created_at) }}</template></el-table-column>
          <el-table-column label="状态" width="120">
            <template #default="{ row }">
              <el-tag type="warning" size="small">待审核</el-tag>
              <el-tag v-if="String(row.review_reason || '').trim() !== ''" type="danger" size="small">已拒绝</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="150" fixed="right">
            <template #default="{ row }">
              <el-button link type="success" :disabled="acting" @click.stop="approve(row)">通过</el-button>
              <el-button link type="danger" :disabled="acting" @click.stop="openReject(row)">拒绝</el-button>
              <el-button link type="primary" @click.stop="openDetail(row)">详情</el-button>
            </template>
          </el-table-column>
          <template #empty><el-empty description="当前没有待审核的商家" /></template>
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

      <el-dialog v-model="rejectDialog" title="拒绝入驻审核" width="min(520px, 92vw)" destroy-on-close>
        <el-form ref="rejectFormRef" :model="rejectForm" :rules="rejectRules" label-position="top" @submit.prevent>
          <el-form-item label="拒绝原因（必填，255 字以内）" prop="reason">
            <el-input v-model="rejectForm.reason" type="textarea" :rows="4" maxlength="255" show-word-limit placeholder="例如：营业执照信息不完整，请补齐后重新提交" />
          </el-form-item>
        </el-form>
        <p class="dialog-hint">拒绝后商家保持待审核状态，商家可在资料页看到拒绝原因并重新提交审核。</p>
        <template #footer>
          <el-button :disabled="acting" @click="rejectDialog = false">取消</el-button>
          <el-button type="danger" :loading="acting" :disabled="!isRejectReasonValid(rejectForm.reason)" @click="submitReject">确认拒绝</el-button>
        </template>
      </el-dialog>
    </template>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { fetchMerchants, fetchMerchantStats, reviewMerchantOnboarding } from '../../api/modules'
import { useAuthStore } from '../../stores/auth'
import { isRejectReasonValid } from '../../utils/refundReview'

const router = useRouter()
const authStore = useAuthStore()
const isSuperAdmin = computed(() => authStore.isSuperAdmin)
const rows = ref([])
const total = ref(0)
const merchantStats = ref({ total: 0 })
const page = ref(1)
const pageSize = ref(10)
const loading = ref(false)
const acting = ref(false)
const rejectDialog = ref(false)
const rejectRow = ref(null)
const rejectFormRef = ref(null)
const rejectForm = reactive({ reason: '' })
const rejectRules = {
  reason: [
    { required: true, message: '请填写拒绝原因', trigger: 'blur' },
    { max: 255, message: '拒绝原因不能超过 255 字', trigger: 'blur' }
  ]
}

const loadList = async () => {
  loading.value = true
  try {
    const { data } = await fetchMerchants({ status: 'pending', page: page.value, page_size: pageSize.value })
    rows.value = Array.isArray(data?.list) ? data.list : []
    total.value = Number(data?.total || 0)
  } catch (error) {
    ElMessage.error(error?.response?.data?.message || '加载待审核商家失败')
  } finally {
    loading.value = false
  }
}

const loadStats = async () => {
  try {
    const { data } = await fetchMerchantStats()
    merchantStats.value = data || { total: 0 }
  } catch {
    merchantStats.value = { total: 0 }
  }
}

const loadAll = async () => { await Promise.all([loadList(), loadStats()]) }

const handlePageChange = async (nextPage) => {
  page.value = nextPage
  await loadList()
}

const openDetail = (row) => {
  if (!row?.id) return
  router.push(`/admin/merchants/${row.id}`)
}

const approve = async (row) => {
  if (!row?.id || acting.value) return
  try {
    await ElMessageBox.confirm(
      `通过后商家「${row.name}」状态变为正常，可开通订阅并营业。`,
      '通过入驻审核？',
      { type: 'warning', confirmButtonText: '确认通过', cancelButtonText: '返回检查' }
    )
  } catch { return }
  acting.value = true
  try {
    await reviewMerchantOnboarding(row.id, { approve: true })
    ElMessage.success('已通过该商家入驻审核')
    await loadAll()
  } catch (error) {
    await loadList()
    ElMessage.error(error?.response?.data?.message || '审核操作失败；请以待审核列表为准，不要直接重试')
  } finally {
    acting.value = false
  }
}

const openReject = (row) => {
  if (!row?.id) return
  rejectRow.value = row
  rejectForm.reason = ''
  rejectDialog.value = true
}

const submitReject = async () => {
  if (!rejectRow.value || acting.value) return
  if (!isRejectReasonValid(rejectForm.reason)) return
  try { await rejectFormRef.value?.validate() } catch { return }
  const reason = rejectForm.reason.trim()
  try {
    await ElMessageBox.confirm(
      `拒绝后商家「${rejectRow.value.name}」保持待审核状态，可修改资料后重新提交。`,
      '拒绝入驻审核？',
      { type: 'warning', confirmButtonText: '确认拒绝', cancelButtonText: '返回检查' }
    )
  } catch { return }
  acting.value = true
  try {
    await reviewMerchantOnboarding(rejectRow.value.id, { approve: false, reason })
    ElMessage.success('已拒绝该商家入驻审核')
    rejectDialog.value = false
    await loadAll()
  } catch (error) {
    await loadList()
    ElMessage.error(error?.response?.data?.message || '拒绝审核失败；请以待审核列表为准')
  } finally {
    acting.value = false
  }
}

const formatTime = (value) => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '—'
onMounted(loadAll)
</script>

<style scoped>
.page-wrap{display:grid;gap:16px;color:#172b4d}.page-heading{display:flex;justify-content:space-between;align-items:flex-start;gap:16px}.heading-actions{display:flex;align-items:center;gap:10px}.eyebrow{margin:0 0 6px;color:#2563eb;font-size:11px;font-weight:800;letter-spacing:.13em}.page-heading h1{margin:0;font-size:27px}.intro{margin:7px 0 0;color:#64748b}.demo-alert{border-radius:13px}.summary-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:10px}.summary-card,.panel{background:#fff;border:1px solid #e2eaf4;border-radius:15px;box-shadow:0 5px 18px rgba(15,23,42,.035)}.summary-card{padding:14px 16px;display:grid;gap:7px;align-content:start}.summary-card span{font-size:12px;color:#64748b}.summary-card strong{font-size:22px;color:#17365d}.summary-card.warning strong{color:#b45309}.summary-card .principle{font-size:17px;color:#b45309}.table-panel{padding:8px 12px 14px}.table-heading{display:flex;justify-content:space-between;align-items:center;gap:12px;padding:9px 6px 13px}.table-heading div{display:grid;gap:5px}.table-heading strong{font-size:15px}.table-heading span{color:#7b8798;font-size:12px}.table-panel :deep(.el-table){cursor:pointer}.table-panel :deep(.el-table td){padding:11px 0}.table-panel small{display:block;margin-top:5px;color:#8491a4;font-size:11px;line-height:1.5}.table-pagination{justify-content:flex-end;margin-top:12px}.dialog-hint{margin:0;color:#64748b;font-size:12px;line-height:1.6}@media(max-width:960px){.summary-grid{grid-template-columns:repeat(2,minmax(0,1fr))}}@media(max-width:760px){.page-heading{display:grid}.page-heading h1{font-size:23px}.summary-grid{grid-template-columns:1fr}.table-panel{overflow:hidden;padding:6px}.table-panel :deep(.el-table){font-size:12px}.table-heading{align-items:flex-start}}
</style>
