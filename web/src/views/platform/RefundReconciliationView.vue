<template>
  <div class="page-wrap">
    <header class="page-heading">
      <div>
        <p class="eyebrow">PLATFORM FINANCE · REFUND EXCEPTIONS</p>
        <h1>退款异常与人工核对</h1>
        <p class="intro">处理中超过 15 分钟才可转入人工核对；系统不会自动重试、查询渠道或释放预留金额。</p>
      </div>
      <el-button type="primary" :loading="loading" @click="loadList">刷新队列</el-button>
    </header>

    <el-alert
      v-if="isLocalDemo"
      class="demo-alert"
      type="warning"
      :closable="false"
      show-icon
      title="本地演示界面 · 页面中的示例数据为合成数据，不连接真实支付渠道或资金。"
      description="人工记录仅表示管理员依据支付渠道后台等外部证据作出的确认，不是系统调用渠道 API 核验。"
    />
    <el-alert v-else class="demo-alert" type="warning" :closable="false" show-icon title="人工对账不是渠道 API 核验" description="仅依据已单独核对的支付渠道证据记录结果；未核实的请求继续保留退款额度。" />

    <section class="summary-grid">
      <article class="summary-card"><span>退款异常队列</span><strong>{{ rows.length }} 条</strong></article>
      <article class="summary-card warning"><span>逾期处理中（≥15 分钟）</span><strong>{{ staleCount }} 条</strong></article>
      <article class="summary-card"><span>队列内继续预留额度</span><strong>{{ money(reservedAmount) }}</strong></article>
      <article class="summary-card"><span>处理原则</span><strong class="principle">未核实不释放</strong></article>
    </section>

    <section class="panel table-panel">
      <div class="table-heading">
        <div><strong>处理中 / 结果未知 · 额度继续预留</strong><span>逾期只代表本地预留长时间未更新，不代表渠道退款成功或失败。</span></div>
        <el-tag type="warning" effect="light">{{ staleCount }} 条逾期</el-tag>
      </div>
      <el-table v-loading="loading" :data="rows" row-key="refund_id" @row-click="openDetail">
        <el-table-column label="退款 / 订单" min-width="185">
          <template #default="{ row }"><strong>{{ row.refund_no }}</strong><small>{{ row.order_no || `订单 #${row.order_id}` }} · 退款 #{{ row.refund_id }}</small></template>
        </el-table-column>
        <el-table-column label="退款金额" width="125" align="right">
          <template #default="{ row }"><strong>{{ money(row.amount) }}</strong><small>请求：{{ row.idempotency_key || '未提供幂等键' }}</small></template>
        </el-table-column>
        <el-table-column label="申请原因" min-width="145"><template #default="{ row }">{{ row.reason || '—' }}</template></el-table-column>
        <el-table-column label="预留状态 / 逾期提示" min-width="230">
          <template #default="{ row }"><el-tag :type="row.is_stale ? 'danger' : row.reservation_status === 'unknown' ? 'warning' : 'info'" size="small">{{ row.is_stale ? `逾期 · ${row.stale_after_minutes} 分钟+` : reservationLabel(row.reservation_status) }}</el-tag><small class="unknown-reason">{{ row.unknown_reason || (row.is_stale ? '仅按本地更新时间标记逾期，结果仍未知' : row.reservation_status === 'processing' ? '尚未达到人工处理阈值' : '未保存渠道返回原因') }}</small></template>
        </el-table-column>
        <el-table-column label="预留更新时间" min-width="165"><template #default="{ row }">{{ formatTime(row.updated_at || row.created_at) }}<small v-if="row.reservation_status === 'processing'">满 {{ row.stale_after_minutes }} 分钟后才可转人工</small></template></el-table-column>
        <el-table-column label="操作" width="82" fixed="right"><template #default="{ row }"><el-button link type="primary" @click.stop="openDetail(row)">查看 / 对账</el-button></template></el-table-column>
        <template #empty><el-empty description="当前没有处理中或未知的退款预留" /></template>
      </el-table>
    </section>

    <el-drawer v-model="drawer" title="退款预留详情与对账记录" size="min(680px, 100%)" destroy-on-close>
      <div v-if="detail" class="detail-content">
        <div class="decision-banner" :class="detail.reservation_status === 'success' || detail.reservation_status === 'failed' ? 'is-done' : detail.is_stale ? 'is-overdue' : 'is-pending'">
          <div><strong>{{ detail.reservation_status === 'processing' && detail.is_stale ? `处理中已逾期 · ${detail.stale_after_minutes} 分钟+` : detail.reservation_status === 'unknown' ? '结果未知 · 额度仍被预留' : `当前状态：${reservationLabel(detail.reservation_status)}` }}</strong><p>订单退款状态：{{ detail.order_refund_status || '—' }} · 预留更新时间：{{ formatTime(detail.updated_at || detail.created_at) }}</p></div>
          <el-tag :type="detail.is_stale ? 'danger' : detail.reservation_status === 'unknown' ? 'warning' : detail.reservation_status === 'success' ? 'success' : 'info'">{{ detail.is_stale ? '逾期处理中' : reservationLabel(detail.reservation_status) }}</el-tag>
        </div>
        <el-alert type="info" :closable="false" show-icon class="evidence-alert">
          逾期仅由本地更新时间判断。转入人工核对不会释放额度、重试退款或查询支付渠道；只有另行核对外部凭证后，才能记录确定成功或失败。
        </el-alert>
        <el-descriptions :column="1" border>
          <el-descriptions-item label="退款申请号 / 记录 ID">{{ detail.refund_no }} / {{ detail.refund_id }}</el-descriptions-item>
          <el-descriptions-item label="订单号 / 订单 ID">{{ detail.order_no || '—' }} / {{ detail.order_id }}</el-descriptions-item>
          <el-descriptions-item label="金额">{{ money(detail.amount) }}</el-descriptions-item>
          <el-descriptions-item label="幂等键">{{ detail.idempotency_key || '未提供' }}</el-descriptions-item>
          <el-descriptions-item label="申请原因">{{ detail.reason || '—' }}</el-descriptions-item>
          <el-descriptions-item label="创建时间 / 最近更新">{{ formatTime(detail.created_at) }} / {{ formatTime(detail.updated_at) }}</el-descriptions-item>
          <el-descriptions-item label="商家 / 门店">{{ detail.merchant_name || '—' }} / {{ detail.store_name || '—' }}</el-descriptions-item>
          <el-descriptions-item label="渠道结果未知原因"><pre class="raw-reason">{{ detail.unknown_reason || '未保存渠道返回原因' }}</pre></el-descriptions-item>
        </el-descriptions>

        <section v-if="detail.reservation_status === 'processing'" class="resolution-box">
          <template v-if="detail.is_stale">
            <h3>逾期预留 · 转入人工核对</h3>
            <p>仅把状态从“处理中”移至“结果未知”；金额继续预留。请填写转入原因，随后仍须提供渠道侧凭证才能记录成功或确定失败。</p>
            <el-form label-position="top"><el-form-item label="转入人工核对的原因（必填）"><el-input v-model="manualReviewNote" type="textarea" :rows="3" maxlength="1000" show-word-limit placeholder="例如：退款预留已超过 15 分钟，待操作员在渠道后台核查" /></el-form-item></el-form>
            <el-button type="warning" :loading="acting" :disabled="!manualReviewNote.trim()" @click="submitManualReview">转入人工核对（继续预留额度）</el-button>
          </template>
          <template v-else>
            <h3>退款请求仍在处理中</h3>
            <p>距最近更新未满 {{ detail.stale_after_minutes }} 分钟，暂不可强制转入人工核对。请勿重复退款；系统不会自动重试或查询渠道。</p>
          </template>
        </section>
        <section v-else-if="detail.reservation_status === 'unknown'" class="resolution-box">
          <h3>记录人工核对结果</h3>
          <p>没有确认前不要操作：该退款额度会继续预留，系统不会允许重复退款占用这部分额度。</p>
          <div class="outcome-buttons">
            <el-button type="success" plain @click="beginResolution('success')">依据凭证记录为已退款</el-button>
            <el-button type="danger" plain @click="beginResolution('failed')">依据凭证记录为确定未退款</el-button>
          </div>
          <div v-if="selectedOutcome" class="resolution-form">
            <el-form label-position="top">
              <el-form-item label="渠道侧退款 / 查询凭证号（必填）">
                <el-input v-model="form.provider_reference" maxlength="120" placeholder="填写已在渠道后台核对的退款单号或参考号" />
              </el-form-item>
              <el-form-item label="核对说明与依据（必填）">
                <el-input v-model="form.reason" type="textarea" :rows="3" maxlength="1000" show-word-limit placeholder="说明核对来源、看到的状态及判断依据" />
              </el-form-item>
            </el-form>
            <el-button
              :type="selectedOutcome === 'success' ? 'success' : 'danger'"
              :loading="acting"
              :disabled="!form.provider_reference.trim() || !form.reason.trim()"
              @click="submitResolution"
            >{{ selectedOutcome === 'success' ? '确认记录为已退款' : '确认记录为确定未退款' }}</el-button>
            <el-button :disabled="acting" @click="cancelResolution">取消</el-button>
          </div>
        </section>
        <el-alert v-else type="success" :closable="false" show-icon class="done-alert" title="此预留已有人工对账结果；再次提交相同记录只会幂等返回，不会重复计入退款金额。" />

        <section class="history">
          <h3>不可修改的人工对账审计</h3>
          <el-timeline v-if="detail.audit?.length">
            <el-timeline-item v-for="event in detail.audit" :key="event.id" :timestamp="formatTime(event.created_at)" placement="top" :type="event.new_status === 'success' ? 'success' : 'danger'">
              <strong>{{ event.old_status }} → {{ event.new_status }}</strong>
              <p>操作员 #{{ event.actor_id }}（{{ event.actor_role }}） · {{ money(event.amount) }}</p>
              <p v-if="event.provider_reference">外部凭证：{{ event.provider_reference }}</p>
              <p>{{ event.old_status === 'processing' && event.new_status === 'unknown' ? '转入说明：' : '核对说明：' }}{{ event.reason }}</p>
            </el-timeline-item>
          </el-timeline>
          <el-empty v-else description="尚无终态人工对账记录" :image-size="72" />
        </section>
      </div>
    </el-drawer>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { fetchAdminRefundReconciliations, fetchAdminRefundReconciliationDetail, markAdminRefundForManualReview, resolveAdminRefundReconciliation } from '../../api/modules'

const isLocalDemo = import.meta.env.DEV && import.meta.env.VITE_REFUND_RECONCILIATION_DEMO === 'true'
const rows = ref([])
const loading = ref(false)
const acting = ref(false)
const drawer = ref(false)
const detail = ref(null)
const selectedOutcome = ref('')
const manualReviewNote = ref('')
const form = reactive({ provider_reference: '', reason: '' })
const reservedAmount = computed(() => rows.value.reduce((sum, row) => sum + Number(row.amount || 0), 0))
const staleCount = computed(() => rows.value.filter(row => row.is_stale).length)

const loadList = async () => {
  loading.value = true
  try {
    const { data } = await fetchAdminRefundReconciliations()
    rows.value = data.list || []
  } catch (error) {
    ElMessage.error(error?.response?.data?.message || '加载未知退款队列失败')
  } finally {
    loading.value = false
  }
}

const openDetail = async row => {
  drawer.value = true
  selectedOutcome.value = ''
  detail.value = { ...row, audit: [] }
  try {
    const { data } = await fetchAdminRefundReconciliationDetail(row.refund_id)
    detail.value = data.detail
  } catch (error) {
    ElMessage.error(error?.response?.data?.message || '加载退款对账详情失败')
  }
}

const beginResolution = outcome => {
  selectedOutcome.value = outcome
  form.provider_reference = ''
  form.reason = ''
}

const submitManualReview = async () => {
  if (!detail.value || !detail.value.is_stale || detail.value.reservation_status !== 'processing' || !manualReviewNote.value.trim() || acting.value) return
  try {
    await ElMessageBox.confirm(
      '此操作只将逾期状态转为“结果未知”，并保留原退款额度；不会调用渠道、自动重试或认定退款结果。',
      '转入人工核对队列？',
      { type: 'warning', confirmButtonText: '转入人工核对', cancelButtonText: '返回检查' }
    )
  } catch { return }
  acting.value = true
  try {
    const refundID = detail.value.refund_id
    await markAdminRefundForManualReview(refundID, { reason: manualReviewNote.value.trim() })
    ElMessage.success('已转入人工核对；退款额度仍在预留，审计记录已追加')
    manualReviewNote.value = ''
    const { data } = await fetchAdminRefundReconciliationDetail(refundID)
    detail.value = data.detail
    await loadList()
  } catch (error) {
    ElMessage.error(error?.response?.data?.message || '转入失败；请刷新后确认当前状态')
  } finally {
    acting.value = false
  }
}

const cancelResolution = () => { selectedOutcome.value = ''; form.provider_reference = ''; form.reason = '' }

const submitResolution = async () => {
  if (!detail.value || !selectedOutcome.value || !form.provider_reference.trim() || !form.reason.trim() || acting.value) return
  const isSuccess = selectedOutcome.value === 'success'
  const consequence = isSuccess
    ? '该退款金额将计入订单已退款金额一次。'
    : '该笔退款预留将被释放，但不会计入已退款金额。'
  try {
    await ElMessageBox.confirm(
      `请确认已在支付渠道等外部来源核对凭证“${form.provider_reference.trim()}”。${consequence}系统不会调用渠道 API 验证此凭证。`,
      isSuccess ? '确认人工记录为已退款？' : '确认人工记录为确定未退款？',
      { type: 'warning', confirmButtonText: isSuccess ? '记录退款成功' : '记录确定失败', cancelButtonText: '返回核对' }
    )
  } catch { return }
  acting.value = true
  try {
    await resolveAdminRefundReconciliation(detail.value.refund_id, {
      outcome: selectedOutcome.value,
      provider_reference: form.provider_reference.trim(),
      reason: form.reason.trim()
    })
    ElMessage.success('人工对账结果已记录，审计已追加')
    cancelResolution()
    const { data } = await fetchAdminRefundReconciliationDetail(detail.value.refund_id)
    detail.value = data.detail
    await loadList()
  } catch (error) {
    ElMessage.error(error?.response?.data?.message || '对账操作失败；请刷新检查当前状态')
  } finally {
    acting.value = false
  }
}

const reservationLabel = value => ({ unknown: '未知 · 额度预留中', processing: '处理中 · 额度预留中', success: '已确认成功', failed: '确定失败 · 额度已释放' }[value] || value || '—')
const money = value => `¥${(Number(value || 0) / 100).toFixed(2)}`
const formatTime = value => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '—'
onMounted(loadList)
</script>

<style scoped>
.page-wrap{display:grid;gap:16px;color:#172b4d}.page-heading{display:flex;justify-content:space-between;align-items:flex-start;gap:16px}.eyebrow{margin:0 0 6px;color:#2563eb;font-size:11px;font-weight:800;letter-spacing:.13em}.page-heading h1{margin:0;font-size:27px}.intro{margin:7px 0 0;color:#64748b}.demo-alert{border-radius:13px}.summary-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:10px}.summary-card,.panel{background:#fff;border:1px solid #e2eaf4;border-radius:15px;box-shadow:0 5px 18px rgba(15,23,42,.035)}.summary-card{padding:14px 16px;display:grid;gap:7px}.summary-card span{font-size:12px;color:#64748b}.summary-card strong{font-size:22px;color:#17365d}.summary-card.warning strong{color:#b45309}.summary-card .principle{font-size:17px;color:#b45309}.table-panel{padding:8px 12px 14px}.table-heading{display:flex;justify-content:space-between;align-items:center;gap:12px;padding:9px 6px 13px}.table-heading div{display:grid;gap:5px}.table-heading strong{font-size:15px}.table-heading span{color:#7b8798;font-size:12px}.table-panel :deep(.el-table){cursor:pointer}.table-panel :deep(.el-table td){padding:11px 0}.table-panel small{display:block;margin-top:5px;color:#8491a4;font-size:11px;line-height:1.5}.unknown-reason{max-width:340px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:#a16207!important}.detail-content{display:grid;gap:14px;padding-bottom:28px}.decision-banner{display:flex;justify-content:space-between;align-items:center;gap:10px;padding:13px 15px;border:1px solid;border-radius:12px}.decision-banner.is-pending{background:#fffbeb;border-color:#fde68a}.decision-banner.is-overdue{background:#fff1f2;border-color:#fecdd3}.decision-banner.is-done{background:#f0fdf4;border-color:#bbf7d0}.decision-banner strong{font-size:14px}.decision-banner p{margin:5px 0 0;color:#64748b;font-size:12px}.evidence-alert,.done-alert{border-radius:11px}.raw-reason{margin:0;white-space:pre-wrap;overflow-wrap:anywhere;font-family:inherit;color:#7c4a03;font-size:12px;line-height:1.55}.resolution-box{padding:14px;border:1px solid #e2e8f0;border-radius:13px;background:#fbfdff}.resolution-box h3,.history h3{margin:0 0 8px;font-size:15px}.resolution-box>p{margin:0 0 12px;color:#64748b;font-size:12px;line-height:1.6}.outcome-buttons{display:flex;flex-wrap:wrap;gap:8px}.resolution-form{margin-top:14px;padding-top:14px;border-top:1px solid #e5eaf1}.resolution-form :deep(.el-form-item){margin-bottom:12px}.history{padding-top:4px}.history :deep(.el-timeline-item__content){color:#334155}.history p{margin:5px 0;color:#64748b;font-size:12px;line-height:1.5}@media(max-width:960px){.summary-grid{grid-template-columns:repeat(2,minmax(0,1fr))}}@media(max-width:760px){.page-heading{display:grid}.page-heading h1{font-size:23px}.page-heading>.el-button{width:100%}.summary-grid{grid-template-columns:1fr}.table-panel{overflow:hidden;padding:6px}.table-panel :deep(.el-table){font-size:12px}.table-heading{align-items:flex-start}.outcome-buttons{display:grid}.outcome-buttons .el-button{margin:0}.detail-content :deep(.el-descriptions__label){width:135px}}
</style>
