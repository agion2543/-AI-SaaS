<template>
  <div class="merchant-page">
    <PageHero
      eyebrow="MERCHANT OPS"
      title="商家运营"
      description="管理商家入驻、订阅状态、冻结状态与后台开通权限。这里是平台方日常运营商家的主工作台。"
      compact
    >
      <template #actions>
        <el-button type="primary" :loading="loading" @click="load">刷新同步</el-button>
      </template>
    </PageHero>

    <el-alert v-if="loadError" class="error-alert" type="error" show-icon :closable="false" :title="loadError" />

    <section class="metric-grid">
      <MetricCard label="商家总数" :value="merchants.length" hint="当前平台入驻商家数量。" tone="primary" />
      <MetricCard label="已订阅" :value="subscribedCount" hint="订阅仍在有效期内的商家。" tone="success" />
      <MetricCard label="高风险商家" :value="highRiskCount" hint="冻结、退款/取消偏高或支付异常。" tone="warning" />
      <MetricCard label="收款待审" :value="paymentPendingCount" hint="未审核通过前顾客端不展示收款码。" tone="warning" />
      <MetricCard label="未处理跟进" :value="openFollowUpCount" hint="平台运营待处理事项总数。" tone="cyan" />
    </section>

    <section class="risk-strip">
      <button
        v-for="item in riskQuickFilters"
        :key="item.value || 'all'"
        type="button"
        :class="{ active: riskFilter === item.value }"
        @click="riskFilter = item.value; search()"
      >
        <span>{{ item.label }}</span>
        <strong>{{ item.valueText }}</strong>
        <small>{{ item.hint }}</small>
      </button>
    </section>

    <DataPanel title="商家列表" description="支持搜索、状态筛选、订阅筛选、排序、冻结恢复与后台开通订阅。">
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
        <el-select v-model="riskFilter" clearable placeholder="风险筛选" @change="search">
          <el-option label="高风险" value="high" />
          <el-option label="需关注" value="watch" />
          <el-option label="有未处理跟进" value="follow_up" />
          <el-option label="待结算" value="settlement" />
          <el-option label="收款待审" value="payment_pending" />
          <el-option label="收款缺码" value="payment_missing_qr" />
        </el-select>
        <el-select v-model="sortBy" placeholder="排序字段" @change="search">
          <el-option label="风险优先" value="risk_score" />
          <el-option label="未处理跟进" value="open_follow_up_count" />
          <el-option label="交易额" value="trade_amount" />
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
        <el-table-column label="风险等级" width="120">
          <template #default="{ row }">
            <el-tag :type="riskLevel(row).type">{{ riskLevel(row).label }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="商家状态" width="120">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)">{{ statusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="交易/退款" min-width="150" sortable="custom" prop="trade_amount">
          <template #default="{ row }">
            <div class="ops-cell">
              <strong>{{ formatMoney(row.trade_amount) }}</strong>
              <small>退款 {{ percent(refundRate(row)) }}</small>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="异常" min-width="190">
          <template #default="{ row }">
            <div class="risk-tags">
              <el-tag v-if="row.failed_order_count" size="small" type="danger">支付失败 {{ row.failed_order_count }}</el-tag>
              <el-tag v-if="row.received_order_count" size="small" type="warning">待接单 {{ row.received_order_count }}</el-tag>
              <el-tag v-if="row.pending_settlement_count || row.unsettled_order_count" size="small" type="warning">待结算 {{ row.pending_settlement_count || row.unsettled_order_count }}</el-tag>
              <el-tag v-if="row.open_follow_up_count" size="small" type="primary">跟进 {{ row.open_follow_up_count }}</el-tag>
              <span v-if="!riskTags(row).length" class="muted">暂无异常</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="订阅状态" width="140">
          <template #default="{ row }">
            <el-tag :type="subscriptionValid(row) ? 'success' : 'info'">
              {{ subscriptionValid(row) ? '订阅中' : '未订阅' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="收款审核" min-width="170">
          <template #default="{ row }">
            <div class="ops-cell">
              <el-tag :type="paymentAuditType(row)">{{ paymentAuditLabel(row) }}</el-tag>
              <small>{{ paymentQrSummary(row) }}</small>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="subscription_expire_at" label="到期时间" min-width="180" sortable="custom">
          <template #default="{ row }">{{ formatTime(row.subscription_expire_at || row.subscription_expired_at) }}</template>
        </el-table-column>
        <el-table-column prop="created_at" label="注册时间" min-width="180" sortable="custom">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="470" fixed="right">
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
    </DataPanel>

    <el-dialog v-model="dialogVisible" title="自定义开通 / 调整订阅" width="520px">
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
          <span class="form-tip">正数增加，负数减少，0 按套餐默认天数。</span>
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
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { fetchMerchants, openMerchantSubscription, updateMerchantStatus } from '../../api/modules'
import DataPanel from '../../components/design/DataPanel.vue'
import MetricCard from '../../components/design/MetricCard.vue'
import PageHero from '../../components/design/PageHero.vue'

const merchants = ref([])
const keyword = ref('')
const status = ref('')
const subscriptionFilter = ref('')
const riskFilter = ref('')
const sortBy = ref('risk_score')
const sortOrder = ref('desc')
const page = ref(1)
const pageSize = ref(20)
const loading = ref(false)
const loadError = ref('')
const dialogVisible = ref(false)
const currentMerchant = ref(null)
const savingSubscription = ref(false)
const route = useRoute()
const router = useRouter()

const subscriptionForm = reactive({
  plan: 'month',
  duration_days: 30,
  note: ''
})

const newestMerchant = computed(() => [...merchants.value].sort((a, b) => dateValue(b.created_at) - dateValue(a.created_at))[0] || null)
const subscribedCount = computed(() => merchants.value.filter(subscriptionValid).length)
const unsubscribedCount = computed(() => merchants.value.length - subscribedCount.value)
const highRiskCount = computed(() => merchants.value.filter((item) => riskLevel(item).score >= 3).length)
const watchRiskCount = computed(() => merchants.value.filter((item) => riskLevel(item).score === 2).length)
const openFollowUpCount = computed(() => merchants.value.reduce((sum, item) => sum + Number(item.open_follow_up_count || 0), 0))
const pendingSettlementMerchantCount = computed(() => merchants.value.filter((item) => Number(item.pending_settlement_count || item.unsettled_order_count || 0) > 0).length)
const paymentPendingCount = computed(() => merchants.value.filter((item) => item.payment_config_audit === 'pending').length)
const paymentMissingQrCount = computed(() => merchants.value.filter((item) => paymentMissingQr(item)).length)
const riskQuickFilters = computed(() => [
  { label: '全部商家', value: '', valueText: merchants.value.length, hint: '按风险优先排序' },
  { label: '高风险', value: 'high', valueText: highRiskCount.value, hint: '优先联系处理' },
  { label: '需关注', value: 'watch', valueText: watchRiskCount.value, hint: '退款、接单或订阅异常' },
  { label: '未处理跟进', value: 'follow_up', valueText: openFollowUpCount.value, hint: '人工事项未关闭' },
  { label: '收款待审', value: 'payment_pending', valueText: paymentPendingCount.value, hint: '影响顾客端付款入口' },
  { label: '收款缺码', value: 'payment_missing_qr', valueText: paymentMissingQrCount.value, hint: '商家收款码未补齐' },
  { label: '待结算', value: 'settlement', valueText: pendingSettlementMerchantCount.value, hint: '资金需要核对' }
])

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
    const matchRisk = !riskFilter.value || matchRiskFilter(item, riskFilter.value)
    return matchKeyword && matchStatus && matchSubscription && matchRisk
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
  riskFilter.value = ''
  sortBy.value = 'risk_score'
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
  if (sortBy.value === 'risk_score') {
    av = riskLevel(a).score
    bv = riskLevel(b).score
  } else if (sortBy.value === 'open_follow_up_count') {
    av = Number(a.open_follow_up_count || 0)
    bv = Number(b.open_follow_up_count || 0)
  } else if (sortBy.value === 'trade_amount') {
    av = Number(a.trade_amount || 0)
    bv = Number(b.trade_amount || 0)
  } else if (sortBy.value === 'subscription_expire_at') {
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

const matchRiskFilter = (row, value) => {
  if (value === 'high') return riskLevel(row).score >= 3
  if (value === 'watch') return riskLevel(row).score === 2
  if (value === 'follow_up') return Number(row.open_follow_up_count || 0) > 0
  if (value === 'settlement') return Number(row.pending_settlement_count || row.unsettled_order_count || 0) > 0
  if (value === 'payment_pending') return row.payment_config_audit === 'pending'
  if (value === 'payment_missing_qr') return paymentMissingQr(row)
  return true
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
const refundRate = (row) => {
  const trade = Number(row.trade_amount || 0)
  return trade ? Number(row.refund_amount || 0) / trade : 0
}
const cancelRate = (row) => {
  const count = Number(row.order_count || 0)
  return count ? Number(row.closed_order_count || 0) / count : 0
}
const riskTags = (row) => {
  const tags = []
  if (row.status === 'suspended') tags.push('冻结')
  if (refundRate(row) >= 0.2) tags.push('退款高')
  if (cancelRate(row) >= 0.3) tags.push('取消高')
  if (Number(row.failed_order_count || 0) > 0) tags.push('支付失败')
  if (Number(row.received_order_count || 0) > 0) tags.push('待接单')
  if (Number(row.pending_settlement_count || row.unsettled_order_count || 0) > 0) tags.push('待结算')
  if (Number(row.open_follow_up_count || 0) > 0) tags.push('待跟进')
  if (!subscriptionValid(row)) tags.push('订阅异常')
  if (row.payment_config_audit === 'pending') tags.push('收款待审')
  if (paymentMissingQr(row)) tags.push('收款缺码')
  return tags
}
const riskLevel = (row) => {
  if (row.status === 'suspended' || refundRate(row) >= 0.35 || cancelRate(row) >= 0.5 || Number(row.failed_order_count || 0) >= 3 || paymentMissingQr(row)) {
    return { label: '高风险', type: 'danger', score: 3 }
  }
  if (refundRate(row) >= 0.2 || cancelRate(row) >= 0.3 || Number(row.open_follow_up_count || 0) > 0 || Number(row.received_order_count || 0) > 0 || row.payment_config_audit === 'pending') {
    return { label: '需关注', type: 'warning', score: 2 }
  }
  if (!subscriptionValid(row) || Number(row.pending_settlement_count || row.unsettled_order_count || 0) > 0) {
    return { label: '观察', type: 'info', score: 1 }
  }
  return { label: '正常', type: 'success', score: 0 }
}
const formatMoney = (value) => `¥${(Number(value || 0) / 100).toFixed(2)}`
const percent = (value) => `${Math.round(Number(value || 0) * 100)}%`
const formatTime = (value) => value ? String(value).replace('T', ' ').slice(0, 19) : '-'
const paymentMissingQr = (row) => row.payment_config_mode === 'direct' && !row.has_alipay_qr_code && !row.has_wechat_qr_code
const paymentAuditLabel = (row) => {
  if (!row.payment_config_audit) return '未配置'
  if (row.payment_config_audit === 'approved' && row.payment_config_status === 'enabled') return '已启用'
  return ({ pending: '待审核', approved: '审核通过', rejected: '已驳回' }[row.payment_config_audit] || row.payment_config_audit)
}
const paymentAuditType = (row) => {
  if (paymentMissingQr(row) || row.payment_config_audit === 'rejected') return 'danger'
  if (row.payment_config_audit === 'pending') return 'warning'
  if (row.payment_config_audit === 'approved' && row.payment_config_status === 'enabled') return 'success'
  return 'info'
}
const paymentQrSummary = (row) => {
  if (!row.payment_config_audit) return '商家未提交收款资料'
  const codes = []
  if (row.has_alipay_qr_code) codes.push('支付宝')
  if (row.has_wechat_qr_code) codes.push('微信')
  if (!codes.length) return '未上传收款码'
  return `${codes.join('/')}收款码`
}

onMounted(async () => {
  if (route.query.risk) {
    riskFilter.value = String(route.query.risk)
  }
  await load()
})
</script>

<style scoped>
.merchant-page {
  display: grid;
  gap: 20px;
}

.metric-grid {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 18px;
}

.risk-strip {
  display: grid;
  grid-template-columns: repeat(7, minmax(0, 1fr));
  gap: 12px;
}

.risk-strip button {
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

.risk-strip button.active {
  border-color: #2563eb;
  background: #eff6ff;
  box-shadow: 0 10px 26px rgba(37, 99, 235, 0.12);
}

.risk-strip span,
.risk-strip small,
.ops-cell small,
.muted {
  color: #64748b;
}

.risk-strip span {
  font-size: 13px;
  font-weight: 900;
}

.risk-strip strong {
  color: #0f172a;
  font-size: 24px;
}

.risk-strip small,
.ops-cell small {
  line-height: 1.45;
}

.filter-grid {
  display: grid;
  grid-template-columns: minmax(220px, 1.3fr) repeat(5, minmax(120px, 1fr)) auto;
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

.ops-cell {
  display: grid;
  gap: 4px;
}

.risk-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
}

.muted {
  font-size: 12px;
}

.form-tip {
  margin-left: 12px;
  color: #64748b;
  font-size: 12px;
}

@media (max-width: 1100px) {
  .metric-grid,
  .risk-strip {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .filter-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 720px) {
  .metric-grid,
  .risk-strip {
    grid-template-columns: 1fr;
  }

  .filter-grid {
    grid-template-columns: 1fr;
  }
}
</style>
