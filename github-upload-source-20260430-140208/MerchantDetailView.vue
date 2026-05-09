<template>
  <div class="detail-stack">
    <section class="page-card hero-card">
      <div>
        <el-button text @click="router.push('/admin/merchants')">返回商家列表</el-button>
        <div class="eyebrow">MERCHANT PROFILE</div>
        <h2 class="page-title">{{ merchant.name || '商家详情' }}</h2>
        <p class="page-desc">统一查看商家订阅、收款配置、门店、订单流水、退款与 AI 经营摘要。</p>
      </div>
      <div class="hero-actions">
        <el-tag size="large" :type="statusType(merchant.status)">{{ statusLabel(merchant.status) }}</el-tag>
        <el-button type="primary" :loading="loading" @click="load">刷新档案</el-button>
      </div>
    </section>

    <section class="summary-grid">
      <div class="summary-card">
        <span>联系人手机号</span>
        <strong>{{ merchant.contact_phone || '-' }}</strong>
      </div>
      <div class="summary-card">
        <span>订阅状态</span>
        <strong>{{ subscriptionValid ? '订阅中' : '未订阅 / 已过期' }}</strong>
      </div>
      <div class="summary-card">
        <span>到期时间</span>
        <strong>{{ formatTime(merchant.subscription_expire_at || merchant.subscription_expired_at) }}</strong>
      </div>
      <div class="summary-card">
        <span>门店数量</span>
        <strong>{{ stores.length }}</strong>
      </div>
      <div class="summary-card">
        <span>扫码订单</span>
        <strong>{{ aiInsights.order_stats?.order_count || storeOrders.length || 0 }}</strong>
      </div>
      <div class="summary-card">
        <span>交易额</span>
        <strong>{{ formatMoney(aiInsights.order_stats?.trade_amount || storeOrderAmount) }}</strong>
      </div>
    </section>

    <section class="grid-2">
      <div class="page-card">
        <div class="toolbar">
          <div>
            <h2 class="page-title">订阅权限</h2>
            <p class="page-desc">平台可在测试或售后场景下为商家手动开通、延长或停用订阅。</p>
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

      <div class="page-card">
        <div class="toolbar">
          <div>
            <h2 class="page-title">收款配置审核</h2>
            <p class="page-desc">审核商家提交的支付宝、微信或银行卡收款资料。</p>
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
          <div><span>APPID</span><strong>{{ paymentConfig.app_id || '-' }}</strong></div>
          <div><span>启用状态</span><strong>{{ paymentConfig.status === 'enabled' ? '已启用' : '未启用' }}</strong></div>
          <div><span>联系电话</span><strong>{{ paymentConfig.contact_phone || '-' }}</strong></div>
          <div><span>审核备注</span><strong>{{ paymentConfig.audit_remark || '-' }}</strong></div>
        </div>
        <div class="action-row payment-actions">
          <el-button type="success" :disabled="!paymentConfig.id" @click="reviewPayment('approved')">审核通过</el-button>
          <el-button type="danger" plain :disabled="!paymentConfig.id" @click="reviewPayment('rejected')">驳回</el-button>
          <el-button plain :disabled="!paymentConfig.id" @click="reviewPayment('pending')">设为待审核</el-button>
        </div>
      </div>
    </section>

    <section class="page-card">
      <div class="toolbar">
        <div>
          <h2 class="page-title">AI 经营摘要</h2>
          <p class="page-desc">基于顾客档案、订单和线索生成的经营提示，后续会继续接入更多真实经营数据。</p>
        </div>
      </div>
      <div class="ai-stats">
        <div><span>顾客档案</span><strong>{{ aiInsights.customer_summary?.total || 0 }}</strong></div>
        <div><span>高价值顾客</span><strong>{{ aiInsights.customer_summary?.high_value || 0 }}</strong></div>
        <div><span>需召回顾客</span><strong>{{ recallCount }}</strong></div>
        <div><span>线索数量</span><strong>{{ aiInsights.summary?.total_leads || 0 }}</strong></div>
      </div>
      <div class="insight-list">
        <div v-for="(item, index) in aiInsights.insights || []" :key="`${item.title}-${index}`" class="insight-item">
          <el-tag :type="insightType(item.type)" size="small">{{ insightLabel(item.type) }}</el-tag>
          <div>
            <strong>{{ item.title }}</strong>
            <p>{{ item.content }}</p>
          </div>
        </div>
        <el-empty v-if="!(aiInsights.insights || []).length" description="暂无 AI 建议" />
      </div>
    </section>

    <section class="grid-2">
      <div class="page-card">
        <div class="toolbar">
          <h2 class="page-title">门店入口</h2>
          <el-button type="primary" @click="openCreateStore">新增门店</el-button>
        </div>
        <el-table :data="stores" empty-text="暂无门店">
          <el-table-column prop="name" label="门店" min-width="130" />
          <el-table-column prop="contact_phone" label="电话" min-width="120" />
          <el-table-column label="状态" width="100">
            <template #default="{ row }">
              <el-tag :type="row.status === 'active' ? 'success' : 'info'">{{ row.status === 'active' ? '启用' : '停用' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="190">
            <template #default="{ row }">
              <el-button size="small" @click="copyStoreLink(row)">复制链接</el-button>
              <el-button size="small" @click="openEditStore(row)">编辑</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <div class="page-card">
        <h2 class="page-title">订阅记录</h2>
        <el-table :data="subscriptionOrders" empty-text="暂无订阅记录">
          <el-table-column label="套餐" min-width="100">
            <template #default="{ row }">{{ row.merchant_plan?.name || '-' }}</template>
          </el-table-column>
          <el-table-column label="金额" width="110">
            <template #default="{ row }">{{ formatMoney(row.total_amount || row.amount) }}</template>
          </el-table-column>
          <el-table-column label="状态" width="100">
            <template #default="{ row }">
              <el-tag :type="orderStatusType(row.status)">{{ orderStatusLabel(row.status) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="创建时间" min-width="150">
            <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
          </el-table-column>
        </el-table>
      </div>
    </section>

    <section class="page-card">
      <h2 class="page-title">最近门店订单</h2>
      <el-table :data="storeOrders" empty-text="暂无门店订单">
        <el-table-column prop="order_no" label="订单号" min-width="180" />
        <el-table-column label="门店" min-width="130">
          <template #default="{ row }">{{ row.store?.name || '-' }}</template>
        </el-table-column>
        <el-table-column prop="customer_phone" label="顾客手机号" min-width="130" />
        <el-table-column label="实付金额" width="120">
          <template #default="{ row }">{{ formatMoney(row.total_amount || row.amount) }}</template>
        </el-table-column>
        <el-table-column label="已退款" width="110">
          <template #default="{ row }">{{ formatMoney(row.refunded_amount) }}</template>
        </el-table-column>
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-tag :type="orderStatusType(row.status)">{{ orderStatusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" min-width="170">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
      </el-table>
    </section>

    <section class="page-card">
      <h2 class="page-title">顾客画像</h2>
      <el-table :data="aiInsights.customer_profiles || []" empty-text="暂无顾客画像">
        <el-table-column prop="customer_phone" label="手机号" min-width="130" />
        <el-table-column prop="store_name" label="最近门店" min-width="140" />
        <el-table-column prop="order_count" label="消费次数" width="100" />
        <el-table-column label="累计消费" width="120">
          <template #default="{ row }">{{ formatMoney(row.total_amount) }}</template>
        </el-table-column>
        <el-table-column prop="ai_tag" label="AI 标签" width="130" />
        <el-table-column prop="ai_suggestion" label="建议" min-width="260" show-overflow-tooltip />
      </el-table>
    </section>

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
          <span class="form-tip">正数增加，负数减少，0 按默认周期</span>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="subscriptionForm.note" maxlength="120" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="subscriptionDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submitSubscription">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="storeDialogVisible" :title="editingStore?.id ? '编辑门店' : '新增门店'" width="520px">
      <el-form label-width="90px">
        <el-form-item label="门店名称"><el-input v-model="storeForm.name" /></el-form-item>
        <el-form-item label="地址"><el-input v-model="storeForm.address" /></el-form-item>
        <el-form-item label="电话"><el-input v-model="storeForm.contact_phone" /></el-form-item>
        <el-form-item label="状态">
          <el-select v-model="storeForm.status">
            <el-option label="启用" value="active" />
            <el-option label="停用" value="inactive" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="storeDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveStore">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  createAdminMerchantStore,
  fetchAdminMerchantAIInsights,
  fetchAdminMerchantPaymentConfig,
  fetchAdminMerchantStoreOrders,
  fetchAdminMerchantStores,
  fetchAdminMerchantSubscriptionOrders,
  fetchMerchantDetail,
  openMerchantSubscription,
  reviewAdminMerchantPaymentConfig,
  stopMerchantSubscription,
  updateAdminMerchantStore
} from '../../api/modules'

const route = useRoute()
const router = useRouter()
const loading = ref(false)
const merchant = ref({})
const paymentConfig = ref({})
const stores = ref([])
const subscriptionOrders = ref([])
const storeOrders = ref([])
const aiInsights = ref({})
const subscriptionDialogVisible = ref(false)
const storeDialogVisible = ref(false)
const editingStore = ref(null)

const subscriptionForm = reactive({ plan: 'month', duration_days: 30, note: '' })
const storeForm = reactive({ name: '', address: '', contact_phone: '', status: 'active' })
const merchantID = () => route.params.id

const subscriptionValid = computed(() => {
  const expire = merchant.value.subscription_expire_at || merchant.value.subscription_expired_at
  return merchant.value.subscription_status === 'active' && expire && new Date(expire).getTime() > Date.now()
})
const recallCount = computed(() => Number(aiInsights.value.customer_summary?.sleeping || 0) + Number(aiInsights.value.customer_summary?.risk || 0))
const storeOrderAmount = computed(() => storeOrders.value.reduce((sum, item) => sum + Number(item.total_amount || item.amount || 0), 0))

const load = async () => {
  loading.value = true
  try {
    const [merchantRes, paymentRes, storesRes, subscriptionRes, ordersRes, aiRes] = await Promise.all([
      fetchMerchantDetail(merchantID()),
      fetchAdminMerchantPaymentConfig(merchantID()),
      fetchAdminMerchantStores(merchantID(), { page: 1, page_size: 50 }),
      fetchAdminMerchantSubscriptionOrders(merchantID()),
      fetchAdminMerchantStoreOrders(merchantID(), { limit: 10 }),
      fetchAdminMerchantAIInsights(merchantID())
    ])
    merchant.value = merchantRes.data || {}
    paymentConfig.value = paymentRes.data.config || {}
    stores.value = storesRes.data.list || []
    subscriptionOrders.value = subscriptionRes.data.list || []
    storeOrders.value = ordersRes.data.list || []
    aiInsights.value = aiRes.data || {}
  } finally {
    loading.value = false
  }
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
    ? '平台审核通过，可作为后续直连收款配置'
    : auditStatus === 'rejected'
      ? '平台审核驳回，请商家修改后重新提交'
      : '平台已重新设为待审核'
  await reviewAdminMerchantPaymentConfig(merchantID(), {
    audit_status: auditStatus,
    status,
    audit_remark: remark
  })
  ElMessage.success('收款配置审核状态已更新')
  load()
}

const openCreateStore = () => {
  editingStore.value = null
  Object.assign(storeForm, { name: '', address: '', contact_phone: merchant.value.contact_phone || '', status: 'active' })
  storeDialogVisible.value = true
}

const openEditStore = (row) => {
  editingStore.value = row
  Object.assign(storeForm, {
    name: row.name,
    address: row.address || '',
    contact_phone: row.contact_phone || '',
    status: row.status || 'active'
  })
  storeDialogVisible.value = true
}

const saveStore = async () => {
  if (editingStore.value?.id) {
    await updateAdminMerchantStore(merchantID(), editingStore.value.id, { ...storeForm })
  } else {
    await createAdminMerchantStore(merchantID(), { ...storeForm })
  }
  ElMessage.success('门店已保存')
  storeDialogVisible.value = false
  load()
}

const copyStoreLink = async (row) => {
  await navigator.clipboard.writeText(`${window.location.origin}/customer/store/${row.id}`)
  ElMessage.success('链接已复制')
}

const statusLabel = (status) => ({ pending: '待审核', active: '正常', suspended: '已冻结' }[status] || status || '-')
const statusType = (status) => ({ pending: 'warning', active: 'success', suspended: 'danger' }[status] || 'info')
const planLabel = (plan) => ({ month: '月付', year: '年付', none: '未开通' }[plan] || '未开通')
const insightType = (type) => ({ warning: 'warning', action: 'primary', success: 'success', info: 'info' }[type] || 'info')
const insightLabel = (type) => ({ warning: '风险', action: '建议', success: '亮点', info: '提示' }[type] || '提示')
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

.page-desc {
  margin: 6px 0 0;
  color: var(--muted);
  line-height: 1.7;
}

.summary-grid,
.grid-2,
.ai-stats,
.payment-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 14px;
}

.summary-card,
.info-list div,
.ai-stats div,
.payment-grid div {
  padding: 16px;
  border: 1px solid #e5edf9;
  border-radius: 14px;
  background: #f8fbff;
}

.summary-card span,
.info-list span,
.ai-stats span,
.payment-grid span {
  display: block;
  color: var(--muted);
  font-size: 13px;
}

.summary-card strong,
.info-list strong,
.ai-stats strong,
.payment-grid strong {
  display: block;
  margin-top: 8px;
  font-size: 20px;
}

.info-list,
.insight-list {
  display: grid;
  gap: 12px;
}

.action-row {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.payment-actions {
  margin-top: 16px;
}

.insight-list {
  margin-top: 16px;
}

.insight-item {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 12px;
  padding: 14px;
  border: 1px solid #e5edf9;
  border-radius: 14px;
  background: #f8fafc;
}

.insight-item p {
  margin: 6px 0 0;
  color: var(--muted);
  line-height: 1.6;
}

.form-tip {
  margin-left: 12px;
  color: #64748b;
  font-size: 12px;
}

@media (max-width: 720px) {
  .hero-card,
  .toolbar {
    flex-direction: column;
  }
}
</style>
