<template>
  <div class="page-wrap">
    <header class="page-heading">
      <div>
        <p class="eyebrow">PLATFORM FINANCE · REFUND REVIEW</p>
        <h1>退款复核</h1>
        <p class="intro">超过复核阈值的退款需超管通过或拒绝后才能执行；通过即提交渠道执行，拒绝后退款不执行。</p>
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
      description="当前账号不是超管，无权查看或处理退款复核。"
    />
    <template v-else>
      <section class="panel threshold-panel">
        <div class="table-heading">
          <div>
            <strong>复核阈值</strong>
            <span>单笔退款金额超过该阈值才需复核（等于阈值不复核）；0 = 全部退款都要复核。</span>
          </div>
          <el-tag type="info" effect="light">默认值 ¥{{ centsToYuan(threshold?.default_cents) }}</el-tag>
        </div>
        <div v-if="threshold" class="threshold-grid">
          <article class="summary-card"><span>当前阈值</span><strong>¥{{ centsToYuan(threshold.threshold_cents) }}</strong><small>后端已换算：{{ threshold.threshold_yuan }} 元</small></article>
          <article class="summary-card"><span>默认值</span><strong>¥{{ centsToYuan(threshold.default_cents) }}</strong><small>{{ threshold.default_cents }} 分</small></article>
          <article class="summary-card"><span>上限</span><strong>¥{{ centsToYuan(threshold.max_cents) }}</strong><small>{{ threshold.max_cents }} 分</small></article>
          <article class="summary-card">
            <span>修改阈值（元）</span>
            <div class="threshold-edit">
              <el-input-number v-model="thresholdYuan" :min="0" :max="1000000" :precision="2" :disabled="savingThreshold" controls-position="right" />
              <el-button type="primary" :loading="savingThreshold" :disabled="thresholdYuanError(thresholdYuan) !== ''" @click="submitThreshold">保存阈值</el-button>
            </div>
            <small v-if="thresholdYuanError(thresholdYuan)">{{ thresholdYuanError(thresholdYuan) }}</small>
            <small v-else>保存时按元 ×100 转为整数分提交，允许 0。</small>
          </article>
        </div>
        <el-skeleton v-else :rows="2" animated />
      </section>

      <section class="summary-grid">
        <article class="summary-card"><span>待复核退款</span><strong>{{ rows.length }} 条</strong></article>
        <article class="summary-card warning"><span>待复核总金额</span><strong>{{ money(totalAmount) }}</strong></article>
        <article class="summary-card"><span>处理原则</span><strong class="principle">通过即执行 · 拒绝须留原因</strong></article>
      </section>

      <section class="panel table-panel">
        <div class="table-heading">
          <div><strong>待复核队列</strong><span>按申请时间从早到晚排列；通过后提交渠道执行，执行失败也可能复核已生效，请以列表为准。</span></div>
          <el-tag type="warning" effect="light">{{ rows.length }} 条待复核</el-tag>
        </div>
        <el-table v-loading="loading" :data="rows" row-key="id" @row-click="openDetail">
          <el-table-column label="退款 / 订单" min-width="200">
            <template #default="{ row }">
              <strong>{{ row.refund_no }}</strong>
              <small>{{ row.order?.order_no || `订单 #${row.order_id}` }} · {{ row.order?.merchant?.name || '—' }} / {{ row.order?.store?.name || '—' }}</small>
            </template>
          </el-table-column>
          <el-table-column label="退款金额" width="120" align="right">
            <template #default="{ row }"><strong>{{ money(row.amount) }}</strong><small>订单总额 {{ money(row.order?.total_amount) }}</small></template>
          </el-table-column>
          <el-table-column label="申请原因" min-width="150"><template #default="{ row }">{{ row.reason || '—' }}</template></el-table-column>
          <el-table-column label="申请人" width="130">
            <template #default="{ row }">{{ operatorRoleLabel(row.operator_role) }}<small>#{{ row.operator_id }}</small></template>
          </el-table-column>
          <el-table-column label="订单退款情况" min-width="150">
            <template #default="{ row }"><el-tag :type="refundStatusType(row.order?.refund_status)" size="small">{{ refundStatusLabel(row.order?.refund_status) }}</el-tag><small>已退 {{ money(row.order?.refunded_amount) }}</small></template>
          </el-table-column>
          <el-table-column label="申请时间" min-width="165"><template #default="{ row }">{{ formatTime(row.created_at) }}</template></el-table-column>
          <el-table-column label="状态" width="90"><template #default="{ row }"><el-tag type="warning" size="small">待复核</el-tag></template></el-table-column>
          <el-table-column label="操作" width="150" fixed="right">
            <template #default="{ row }">
              <el-button link type="success" :disabled="acting" @click.stop="approve(row)">通过</el-button>
              <el-button link type="danger" :disabled="acting" @click.stop="openReject(row)">拒绝</el-button>
              <el-button link type="primary" @click.stop="openDetail(row)">详情</el-button>
            </template>
          </el-table-column>
          <template #empty><el-empty description="当前没有待复核的退款" /></template>
        </el-table>
      </section>

      <el-drawer v-model="drawer" title="待复核退款详情" size="min(680px, 100%)" destroy-on-close>
        <div v-if="detail" class="detail-content">
          <el-alert type="warning" :closable="false" show-icon class="evidence-alert" title="通过即提交渠道执行" description="通过后系统立即调用渠道执行退款；若渠道执行失败，复核本身可能已生效，请以待复核列表为准，不要直接重试。" />
          <el-descriptions :column="1" border>
            <el-descriptions-item label="退款申请号">{{ detail.refund_no }}</el-descriptions-item>
            <el-descriptions-item label="订单号">{{ detail.order?.order_no || '—' }} / 订单 #{{ detail.order_id }}</el-descriptions-item>
            <el-descriptions-item label="商家 / 门店">{{ detail.order?.merchant?.name || '—' }} / {{ detail.order?.store?.name || '—' }}</el-descriptions-item>
            <el-descriptions-item label="退款金额">{{ money(detail.amount) }}</el-descriptions-item>
            <el-descriptions-item label="订单总额 / 已退">{{ money(detail.order?.total_amount) }} / {{ money(detail.order?.refunded_amount) }}</el-descriptions-item>
            <el-descriptions-item label="订单退款状态">{{ refundStatusLabel(detail.order?.refund_status) }}</el-descriptions-item>
            <el-descriptions-item label="渠道交易号">{{ detail.transaction_no || '—' }}</el-descriptions-item>
            <el-descriptions-item label="申请原因">{{ detail.reason || '—' }}</el-descriptions-item>
            <el-descriptions-item label="申请人">{{ operatorRoleLabel(detail.operator_role) }} #{{ detail.operator_id }}</el-descriptions-item>
            <el-descriptions-item label="申请时间">{{ formatTime(detail.created_at) }}</el-descriptions-item>
          </el-descriptions>
          <div class="outcome-buttons">
            <el-button type="success" :loading="acting" @click="approve(detail)">通过复核并执行退款</el-button>
            <el-button type="danger" plain :disabled="acting" @click="openReject(detail)">拒绝复核</el-button>
          </div>
        </div>
      </el-drawer>

      <el-dialog v-model="rejectDialog" title="拒绝退款复核" width="min(520px, 92vw)" destroy-on-close>
        <el-form ref="rejectFormRef" :model="rejectForm" :rules="rejectRules" label-position="top" @submit.prevent>
          <el-form-item label="拒绝原因（必填，255 字以内）" prop="reason">
            <el-input v-model="rejectForm.reason" type="textarea" :rows="4" maxlength="255" show-word-limit placeholder="例如：金额与订单不符，请核对后重新申请" />
          </el-form-item>
        </el-form>
        <p class="dialog-hint">拒绝后该笔退款不执行，记录状态变为复核已拒绝，审计会记录拒绝原因。</p>
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
import { ElMessage, ElMessageBox } from 'element-plus'
import { fetchPendingReviewRefunds, fetchRefundReviewThreshold, reviewRefund, saveRefundReviewThreshold } from '../../api/modules'
import { useAuthStore } from '../../stores/auth'
import {
  centsToYuan,
  isRejectReasonValid,
  operatorRoleLabel,
  refundStatusLabel,
  thresholdYuanError,
  yuanToCents
} from '../../utils/refundReview'

const authStore = useAuthStore()
const isSuperAdmin = computed(() => authStore.isSuperAdmin)
const rows = ref([])
const threshold = ref(null)
const thresholdYuan = ref(1000)
const loading = ref(false)
const acting = ref(false)
const savingThreshold = ref(false)
const drawer = ref(false)
const detail = ref(null)
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
const totalAmount = computed(() => rows.value.reduce((sum, row) => sum + Number(row.amount || 0), 0))

const loadList = async () => {
  loading.value = true
  try {
    const { data } = await fetchPendingReviewRefunds()
    rows.value = Array.isArray(data) ? data : []
  } catch (error) {
    ElMessage.error(error?.response?.data?.message || '加载待复核退款失败')
  } finally {
    loading.value = false
  }
}

const loadThreshold = async () => {
  try {
    const { data } = await fetchRefundReviewThreshold()
    threshold.value = data
    thresholdYuan.value = Number(data?.threshold_cents || 0) / 100
  } catch (error) {
    ElMessage.error(error?.response?.data?.message || '加载复核阈值失败')
  }
}

const loadAll = async () => { await Promise.all([loadList(), loadThreshold()]) }

const openDetail = (row) => {
  if (!row) return
  detail.value = row
  drawer.value = true
}

const approve = async (row) => {
  if (!row?.id || acting.value) return
  try {
    await ElMessageBox.confirm(
      `通过后将立即提交渠道执行退款 ${money(row.amount)}（${row.refund_no}）。若渠道执行失败，复核本身可能已生效，请以待复核列表为准。`,
      '通过退款复核？',
      { type: 'warning', confirmButtonText: '通过并执行退款', cancelButtonText: '返回检查' }
    )
  } catch { return }
  acting.value = true
  try {
    await reviewRefund(row.id, { approve: true })
    ElMessage.success('已通过复核，退款已提交渠道执行')
    drawer.value = false
    await loadList()
  } catch (error) {
    // 时序坑：通过时后端先在事务里把状态改为 processing 并提交，再调渠道；
    // 渠道失败也会返回 400，但复核可能已生效。先重拉列表再下结论。
    await loadList()
    ElMessage.error(error?.response?.data?.message || '复核操作失败；请以待复核列表为准，不要直接重试')
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
      `拒绝后该笔退款（${rejectRow.value.refund_no}，${money(rejectRow.value.amount)}）将不执行。`,
      '拒绝退款复核？',
      { type: 'warning', confirmButtonText: '确认拒绝', cancelButtonText: '返回检查' }
    )
  } catch { return }
  acting.value = true
  try {
    await reviewRefund(rejectRow.value.id, { approve: false, reason })
    ElMessage.success('已拒绝该笔退款复核')
    rejectDialog.value = false
    drawer.value = false
    await loadList()
  } catch (error) {
    await loadList()
    ElMessage.error(error?.response?.data?.message || '拒绝复核失败；请以待复核列表为准')
  } finally {
    acting.value = false
  }
}

const submitThreshold = async () => {
  if (thresholdYuanError(thresholdYuan.value) !== '' || savingThreshold.value) return
  const cents = yuanToCents(thresholdYuan.value)
  try {
    await ElMessageBox.confirm(
      `将复核阈值改为 ¥${centsToYuan(cents)}（${cents} 分）。超过该阈值的退款需超管复核；0 = 全部退款都要复核。`,
      '修改复核阈值？',
      { type: 'warning', confirmButtonText: '保存阈值', cancelButtonText: '取消' }
    )
  } catch { return }
  savingThreshold.value = true
  try {
    await saveRefundReviewThreshold({ threshold_cents: cents })
    ElMessage.success('复核阈值已保存')
    await loadThreshold()
  } catch (error) {
    ElMessage.error(error?.response?.data?.message || '保存复核阈值失败')
  } finally {
    savingThreshold.value = false
  }
}

const refundStatusType = (status) => ({ partial: 'warning', full: 'success', processing: 'danger' }[status] || 'info')
const money = (value) => `¥${(Number(value || 0) / 100).toFixed(2)}`
const formatTime = (value) => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '—'
onMounted(loadAll)
</script>

<style scoped>
.page-wrap{display:grid;gap:16px;color:#172b4d}.page-heading{display:flex;justify-content:space-between;align-items:flex-start;gap:16px}.heading-actions{display:flex;align-items:center;gap:10px}.eyebrow{margin:0 0 6px;color:#2563eb;font-size:11px;font-weight:800;letter-spacing:.13em}.page-heading h1{margin:0;font-size:27px}.intro{margin:7px 0 0;color:#64748b}.demo-alert{border-radius:13px}.summary-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:10px}.summary-card,.panel{background:#fff;border:1px solid #e2eaf4;border-radius:15px;box-shadow:0 5px 18px rgba(15,23,42,.035)}.summary-card{padding:14px 16px;display:grid;gap:7px;align-content:start}.summary-card span{font-size:12px;color:#64748b}.summary-card strong{font-size:22px;color:#17365d}.summary-card.warning strong{color:#b45309}.summary-card .principle{font-size:17px;color:#b45309}.summary-card small{color:#8491a4;font-size:12px}.threshold-panel{padding:8px 12px 14px}.threshold-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:10px;padding:0 6px 6px}.threshold-grid .summary-card{border:1px solid #e2eaf4;border-radius:13px;box-shadow:none}.threshold-edit{display:flex;gap:8px}.threshold-edit :deep(.el-input-number){width:150px}.table-panel{padding:8px 12px 14px}.table-heading{display:flex;justify-content:space-between;align-items:center;gap:12px;padding:9px 6px 13px}.table-heading div{display:grid;gap:5px}.table-heading strong{font-size:15px}.table-heading span{color:#7b8798;font-size:12px}.table-panel :deep(.el-table){cursor:pointer}.table-panel :deep(.el-table td){padding:11px 0}.table-panel small{display:block;margin-top:5px;color:#8491a4;font-size:11px;line-height:1.5}.detail-content{display:grid;gap:14px;padding-bottom:28px}.evidence-alert{border-radius:11px}.outcome-buttons{display:flex;flex-wrap:wrap;gap:8px}.dialog-hint{margin:0;color:#64748b;font-size:12px;line-height:1.6}@media(max-width:960px){.summary-grid,.threshold-grid{grid-template-columns:repeat(2,minmax(0,1fr))}}@media(max-width:760px){.page-heading{display:grid}.page-heading h1{font-size:23px}.summary-grid,.threshold-grid{grid-template-columns:1fr}.table-panel{overflow:hidden;padding:6px}.table-panel :deep(.el-table){font-size:12px}.table-heading{align-items:flex-start}.outcome-buttons{display:grid}.outcome-buttons .el-button{margin:0}.detail-content :deep(.el-descriptions__label){width:135px}}
</style>
