<template>
  <div class="detail-stack">
    <section class="page-card hero-card">
      <div>
        <el-button text @click="router.push('/admin/merchants')">返回商家列表</el-button>
        <div class="eyebrow">MERCHANT PROFILE</div>
        <h2 class="page-title">{{ merchant.name || '商家详情' }}</h2>
        <p class="page-desc">查看商家订阅、收款资料、门店订单、退款记录和人工结算单。</p>
      </div>
      <div class="hero-actions">
        <el-tag size="large" :type="statusType(merchant.status)">{{ statusLabel(merchant.status) }}</el-tag>
        <el-button type="primary" :loading="loading" @click="load">刷新档案</el-button>
      </div>
    </section>

    <section class="summary-grid">
      <div class="summary-card"><span>联系人手机号</span><strong>{{ merchant.contact_phone || '-' }}</strong></div>
      <div class="summary-card"><span>订阅状态</span><strong>{{ subscriptionValid ? '订阅中' : '未订阅 / 已过期' }}</strong></div>
      <div class="summary-card"><span>到期时间</span><strong>{{ formatTime(merchant.subscription_expire_at || merchant.subscription_expired_at) }}</strong></div>
      <div class="summary-card"><span>门店数量</span><strong>{{ stores.length }}</strong></div>
      <div class="summary-card"><span>近期开单</span><strong>{{ storeOrders.length }}</strong></div>
      <div class="summary-card"><span>近期交易额</span><strong>{{ formatMoney(storeOrderAmount) }}</strong></div>
    </section>

    <section class="page-card">
      <el-tabs v-model="activeTab">
        <el-tab-pane label="运营概览" name="overview">
          <div class="grid-2">
            <div class="inner-card">
              <div class="toolbar">
                <div>
                  <h3>订阅权限</h3>
                  <p>平台可在测试、售后或商务场景下手动开通、延长或停用订阅。</p>
                </div>
                <div class="action-row">
                  <el-button type="primary" @click="openSubscriptionDialog">开通 / 调整</el-button>
                  <el-button v-if="merchant.subscription_status === 'active'" type="danger" plain @click="stopSubscriptionNow">停用</el-button>
                </div>
              </div>
              <div class="info-list">
                <div><span>当前套餐</span><strong>{{ planLabel(merchant.subscription_plan) }}</strong></div>
                <div><span>订阅备注</span><strong>{{ merchant.subscription_note || '-' }}</strong></div>
                <div><span>注册时间</span><strong>{{ formatTime(merchant.created_at) }}</strong></div>
              </div>
            </div>

            <div class="inner-card">
              <div class="toolbar">
                <div>
                  <h3>收款资料审核</h3>
                  <p>现阶段仅作为人工结算参考，不自动把顾客支付切到商家账户。</p>
                </div>
                <el-tag :type="paymentAuditType(paymentConfig.audit_status)" size="large">
                  {{ paymentAuditLabel(paymentConfig.audit_status) }}
                </el-tag>
              </div>
              <div class="payment-grid">
                <div><span>收款渠道</span><strong>{{ paymentChannelLabel(paymentConfig.channel) }}</strong></div>
                <div><span>收款模式</span><strong>{{ paymentModeLabel(paymentConfig.mode) }}</strong></div>
                <div><span>账户名称</span><strong>{{ paymentConfig.account_name || '-' }}</strong></div>
                <div><span>收款账号</span><strong>{{ maskAccount(paymentConfig.account_no) }}</strong></div>
                <div><span>联系电话</span><strong>{{ paymentConfig.contact_phone || '-' }}</strong></div>
                <div><span>审核备注</span><strong>{{ paymentConfig.audit_remark || '-' }}</strong></div>
              </div>
              <div class="action-row payment-actions">
                <el-button type="success" :disabled="!paymentConfig.id" @click="reviewPayment('approved')">审核通过</el-button>
                <el-button type="danger" plain :disabled="!paymentConfig.id" @click="reviewPayment('rejected')">驳回</el-button>
                <el-button plain :disabled="!paymentConfig.id" @click="reviewPayment('pending')">设为待审核</el-button>
              </div>
            </div>
          </div>
        </el-tab-pane>

        <el-tab-pane label="结算记录" name="settlements">
          <div class="toolbar settlement-toolbar">
            <div>
              <h3>人工结算</h3>
              <p>平台统一收款后，按商家生成结算单，线下人工转账后标记已付款。</p>
            </div>
            <el-button type="primary" @click="openSettlementDialog">发起结算</el-button>
          </div>

          <section class="settlement-stats">
            <div><span>待结算订单</span><strong>{{ settlementPrepare.order_count || 0 }}</strong></div>
            <div><span>订单实付</span><strong>{{ formatMoney(settlementPrepare.total_amount_cents) }}</strong></div>
            <div><span>退款金额</span><strong>{{ formatMoney(settlementPrepare.refund_amount_cents) }}</strong></div>
            <div><span>待结算净额</span><strong>{{ formatMoney(settlementPrepare.net_amount_cents) }}</strong></div>
          </section>

          <el-table :data="settlements" empty-text="暂无结算记录">
            <el-table-column prop="id" label="结算ID" width="90" />
            <el-table-column label="周期" min-width="220">
              <template #default="{ row }">
                {{ formatDate(row.settlement_period_start) }} - {{ formatDate(row.settlement_period_end) }}
              </template>
            </el-table-column>
            <el-table-column prop="order_count" label="订单数" width="90" />
            <el-table-column label="实付" width="120"><template #default="{ row }">{{ formatMoney(row.total_amount_cents) }}</template></el-table-column>
            <el-table-column label="退款" width="120"><template #default="{ row }">{{ formatMoney(row.refund_amount_cents) }}</template></el-table-column>
            <el-table-column label="净结算" width="130"><template #default="{ row }"><strong>{{ formatMoney(row.net_amount_cents) }}</strong></template></el-table-column>
            <el-table-column label="状态" width="110">
              <template #default="{ row }">
                <el-tag :type="row.status === 'paid' ? 'success' : 'warning'">{{ row.status === 'paid' ? '已付款' : '待付款' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="付款时间" min-width="170"><template #default="{ row }">{{ formatTime(row.paid_at) }}</template></el-table-column>
            <el-table-column label="操作" width="250" fixed="right">
              <template #default="{ row }">
                <el-button size="small" @click="downloadSettlement(row)">导出</el-button>
                <el-button v-if="row.status === 'pending'" size="small" type="success" @click="markSettlementPaid(row)">标记已付款</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>

        <el-tab-pane label="门店订单" name="orders">
          <el-table :data="storeOrders" empty-text="暂无门店订单">
            <el-table-column prop="order_no" label="订单号" min-width="180" />
            <el-table-column label="门店" min-width="130"><template #default="{ row }">{{ row.store?.name || '-' }}</template></el-table-column>
            <el-table-column prop="customer_phone" label="顾客手机号" min-width="130" />
            <el-table-column label="实付金额" width="120"><template #default="{ row }">{{ formatMoney(row.total_amount || row.amount) }}</template></el-table-column>
            <el-table-column label="已退款" width="110"><template #default="{ row }">{{ formatMoney(row.refunded_amount) }}</template></el-table-column>
            <el-table-column label="结算状态" width="110">
              <template #default="{ row }">
                <el-tag :type="row.settlement_id ? 'success' : 'info'">{{ row.settlement_id ? '已归集' : '未结算' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="订单状态" width="110">
              <template #default="{ row }"><el-tag :type="orderStatusType(row.status)">{{ orderStatusLabel(row.status) }}</el-tag></template>
            </el-table-column>
            <el-table-column label="创建时间" min-width="170"><template #default="{ row }">{{ formatTime(row.created_at) }}</template></el-table-column>
          </el-table>
        </el-tab-pane>

        <el-tab-pane label="订阅记录" name="subscriptionOrders">
          <el-table :data="subscriptionOrders" empty-text="暂无订阅记录">
            <el-table-column label="套餐" min-width="120"><template #default="{ row }">{{ row.merchant_plan?.name || '-' }}</template></el-table-column>
            <el-table-column label="金额" width="120"><template #default="{ row }">{{ formatMoney(row.total_amount || row.amount) }}</template></el-table-column>
            <el-table-column label="状态" width="110"><template #default="{ row }"><el-tag :type="orderStatusType(row.status)">{{ orderStatusLabel(row.status) }}</el-tag></template></el-table-column>
            <el-table-column label="创建时间" min-width="170"><template #default="{ row }">{{ formatTime(row.created_at) }}</template></el-table-column>
          </el-table>
        </el-tab-pane>
      </el-tabs>
    </section>

    <el-dialog v-model="settlementDialogVisible" title="发起人工结算" width="760px">
      <section class="settlement-stats compact">
        <div><span>订单数</span><strong>{{ settlementPrepare.order_count || 0 }}</strong></div>
        <div><span>订单实付</span><strong>{{ formatMoney(settlementPrepare.total_amount_cents) }}</strong></div>
        <div><span>退款金额</span><strong>{{ formatMoney(settlementPrepare.refund_amount_cents) }}</strong></div>
        <div><span>净结算</span><strong>{{ formatMoney(settlementPrepare.net_amount_cents) }}</strong></div>
      </section>
      <el-input v-model="settlementRemark" class="remark-input" placeholder="结算备注，例如：2026年5月第一期人工转账" />
      <el-table :data="settlementPrepare.orders || []" max-height="320" empty-text="暂无可结算订单">
        <el-table-column prop="order_no" label="订单号" min-width="180" />
        <el-table-column label="门店" min-width="120"><template #default="{ row }">{{ row.store?.name || '-' }}</template></el-table-column>
        <el-table-column label="实付" width="110"><template #default="{ row }">{{ formatMoney(row.total_amount || row.amount) }}</template></el-table-column>
        <el-table-column label="退款" width="110"><template #default="{ row }">{{ formatMoney(row.refunded_amount) }}</template></el-table-column>
        <el-table-column label="状态" width="100"><template #default="{ row }">{{ orderStatusLabel(row.status) }}</template></el-table-column>
      </el-table>
      <template #footer>
        <el-button @click="settlementDialogVisible = false">取消</el-button>
        <el-button type="primary" :disabled="!settlementPrepare.order_count" @click="createSettlement">确认生成结算单</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="subscriptionDialogVisible" title="开通 / 调整订阅" width="520px">
      <el-form label-width="100px">
        <el-form-item label="套餐">
          <el-select v-model="subscriptionForm.plan">
            <el-option label="月付" value="month" />
            <el-option label="年付" value="year" />
          </el-select>
        </el-form-item>
        <el-form-item label="调整天数">
          <el-input-number v-model="subscriptionForm.duration_days" :min="-3650" :max="3650" />
        </el-form-item>
        <el-form-item label="备注"><el-input v-model="subscriptionForm.note" maxlength="120" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="subscriptionDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submitSubscription">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  createAdminMerchantSettlement,
  exportAdminMerchantSettlement,
  fetchAdminMerchantPaymentConfig,
  fetchAdminMerchantSettlementPrepare,
  fetchAdminMerchantSettlements,
  fetchAdminMerchantStoreOrders,
  fetchAdminMerchantStores,
  fetchAdminMerchantSubscriptionOrders,
  fetchMerchantDetail,
  markAdminMerchantSettlementPaid,
  openMerchantSubscription,
  reviewAdminMerchantPaymentConfig,
  stopMerchantSubscription
} from '../../api/modules'

const route = useRoute()
const router = useRouter()
const loading = ref(false)
const activeTab = ref('overview')
const merchant = ref({})
const paymentConfig = ref({})
const stores = ref([])
const subscriptionOrders = ref([])
const storeOrders = ref([])
const settlements = ref([])
const settlementPrepare = ref({})
const subscriptionDialogVisible = ref(false)
const settlementDialogVisible = ref(false)
const settlementRemark = ref('')
const subscriptionForm = reactive({ plan: 'month', duration_days: 30, note: '' })
const merchantID = () => route.params.id

const subscriptionValid = computed(() => {
  const expire = merchant.value.subscription_expire_at || merchant.value.subscription_expired_at
  return merchant.value.subscription_status === 'active' && expire && new Date(expire).getTime() > Date.now()
})
const storeOrderAmount = computed(() => storeOrders.value.reduce((sum, item) => sum + Number(item.total_amount || item.amount || 0), 0))

const load = async () => {
  loading.value = true
  try {
    const [merchantRes, paymentRes, storesRes, subscriptionRes, ordersRes, prepareRes, settlementsRes] = await Promise.all([
      fetchMerchantDetail(merchantID()),
      fetchAdminMerchantPaymentConfig(merchantID()),
      fetchAdminMerchantStores(merchantID(), { page: 1, page_size: 50 }),
      fetchAdminMerchantSubscriptionOrders(merchantID()),
      fetchAdminMerchantStoreOrders(merchantID(), { limit: 50 }),
      fetchAdminMerchantSettlementPrepare(merchantID()),
      fetchAdminMerchantSettlements(merchantID())
    ])
    merchant.value = merchantRes.data || {}
    paymentConfig.value = paymentRes.data.config || {}
    stores.value = storesRes.data.list || []
    subscriptionOrders.value = subscriptionRes.data.list || []
    storeOrders.value = ordersRes.data.list || []
    settlementPrepare.value = prepareRes.data || {}
    settlements.value = settlementsRes.data.list || []
  } finally {
    loading.value = false
  }
}

const openSettlementDialog = async () => {
  const res = await fetchAdminMerchantSettlementPrepare(merchantID())
  settlementPrepare.value = res.data || {}
  settlementRemark.value = ''
  settlementDialogVisible.value = true
}

const createSettlement = async () => {
  await createAdminMerchantSettlement(merchantID(), { remark: settlementRemark.value })
  ElMessage.success('结算单已生成')
  settlementDialogVisible.value = false
  load()
}

const markSettlementPaid = async (row) => {
  const { value } = await ElMessageBox.prompt('请输入人工转账备注，例如付款流水号', '标记已付款', {
    confirmButtonText: '确认',
    cancelButtonText: '取消',
    inputValue: row.remark || ''
  })
  await markAdminMerchantSettlementPaid(row.id, { remark: value || '' })
  ElMessage.success('已标记为已付款')
  load()
}

const downloadSettlement = async (row) => {
  const res = await exportAdminMerchantSettlement(row.id)
  const blob = new Blob([res.data], { type: 'text/csv;charset=utf-8;' })
  const link = document.createElement('a')
  link.href = URL.createObjectURL(blob)
  link.download = `merchant-settlement-${row.id}.csv`
  link.click()
  URL.revokeObjectURL(link.href)
}

const openSubscriptionDialog = () => {
  subscriptionForm.plan = merchant.value.subscription_plan === 'year' ? 'year' : 'month'
  subscriptionForm.duration_days = subscriptionForm.plan === 'year' ? 365 : 30
  subscriptionForm.note = merchant.value.subscription_note || ''
  subscriptionDialogVisible.value = true
}

const submitSubscription = async () => {
  await openMerchantSubscription(merchantID(), { ...subscriptionForm })
  ElMessage.success('订阅已调整')
  subscriptionDialogVisible.value = false
  load()
}

const stopSubscriptionNow = async () => {
  await ElMessageBox.confirm('确认停用该商家的订阅权限吗？', '停用订阅', { type: 'warning' })
  await stopMerchantSubscription(merchantID())
  ElMessage.success('订阅已停用')
  load()
}

const reviewPayment = async (auditStatus) => {
  const status = auditStatus === 'approved' ? 'enabled' : 'disabled'
  const remark = auditStatus === 'approved'
    ? '平台审核通过，当前仅作为人工结算资料参考'
    : auditStatus === 'rejected'
      ? '平台审核驳回，请商家修改后重新提交'
      : '平台已重新设为待审核'
  await reviewAdminMerchantPaymentConfig(merchantID(), { audit_status: auditStatus, status, audit_remark: remark })
  ElMessage.success('收款资料审核状态已更新')
  load()
}

const statusLabel = (status) => ({ pending: '待审核', active: '正常', suspended: '已冻结' }[status] || status || '-')
const statusType = (status) => ({ pending: 'warning', active: 'success', suspended: 'danger' }[status] || 'info')
const planLabel = (plan) => ({ month: '月付', year: '年付', none: '未开通' }[plan] || '未开通')
const paymentAuditLabel = (value) => ({ pending: '待审核', approved: '审核通过', rejected: '审核驳回' }[value] || '未提交')
const paymentAuditType = (value) => ({ pending: 'warning', approved: 'success', rejected: 'danger' }[value] || 'info')
const paymentChannelLabel = (value) => ({ alipay: '支付宝', wechat: '微信支付', bank: '银行卡' }[value] || '-')
const paymentModeLabel = (value) => ({ direct: '顾客直付商家账户', platform: '平台统一收款', service_provider: '服务商分账模式' }[value] || '-')
const maskAccount = (value) => {
  const text = String(value || '')
  if (text.length <= 4) return text || '-'
  return `${text.slice(0, 3)}****${text.slice(-4)}`
}
const orderStatusLabel = (status) => ({
  pending: '待支付',
  paid: '已支付',
  received: '待接单',
  accepted: '已接单',
  completed: '已完成',
  closed: '已关闭',
  failed: '支付失败'
}[status] || status || '-')
const orderStatusType = (status) => ({
  pending: 'warning',
  paid: 'success',
  received: 'primary',
  accepted: 'success',
  completed: 'success',
  closed: 'info',
  failed: 'danger'
}[status] || 'info')
const formatMoney = (value) => `¥${(Number(value || 0) / 100).toFixed(2)}`
const formatTime = (value) => value ? String(value).replace('T', ' ').slice(0, 19) : '-'
const formatDate = (value) => value ? String(value).slice(0, 10) : '-'

onMounted(load)
</script>

<style scoped>
.detail-stack {
  display: grid;
  gap: 18px;
}

.hero-card,
.toolbar,
.hero-actions {
  display: flex;
  justify-content: space-between;
  gap: 14px;
  align-items: flex-start;
}

.hero-card {
  padding: 22px;
}

.eyebrow {
  margin-top: 8px;
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.16em;
}

.page-desc,
.toolbar p {
  margin: 6px 0 0;
  color: var(--muted);
  line-height: 1.7;
}

.summary-grid,
.grid-2,
.payment-grid,
.settlement-stats {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 14px;
}

.summary-card,
.inner-card,
.info-list div,
.payment-grid div,
.settlement-stats div {
  padding: 16px;
  border: 1px solid #e5edf9;
  border-radius: 14px;
  background: #f8fbff;
}

.inner-card {
  display: grid;
  gap: 14px;
}

.summary-card span,
.info-list span,
.payment-grid span,
.settlement-stats span {
  display: block;
  color: var(--muted);
  font-size: 13px;
}

.summary-card strong,
.info-list strong,
.payment-grid strong,
.settlement-stats strong {
  display: block;
  margin-top: 8px;
  font-size: 20px;
}

.settlement-stats div:last-child strong {
  color: #16a34a;
}

.info-list {
  display: grid;
  gap: 12px;
}

.action-row {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.payment-actions,
.remark-input {
  margin-top: 16px;
}

.settlement-toolbar {
  margin-bottom: 16px;
}

.compact {
  margin-bottom: 14px;
}

@media (max-width: 768px) {
  .hero-card,
  .toolbar {
    display: grid;
  }
}
</style>
