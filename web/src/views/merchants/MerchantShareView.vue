<template>
  <div class="share-page">
    <section class="share-hero">
      <div>
        <p class="eyebrow">REFERRAL GROWTH</p>
        <h1>营销裂变看板</h1>
        <p class="hero-copy">
          开启后，顾客支付成功页会展示分享海报与好友券；关闭时顾客端不展示任何裂变活动内容。
        </p>
      </div>
      <div class="hero-actions">
        <el-tag :type="config.enabled ? 'success' : 'info'" size="large">
          {{ config.enabled ? '活动已开启' : '活动未开启' }}
        </el-tag>
        <el-button @click="load">刷新数据</el-button>
      </div>
    </section>

    <section class="metric-grid">
      <article class="metric-card">
        <span>海报活动</span>
        <strong>{{ stats.total_campaigns || 0 }}</strong>
        <small>顾客订单生成的分享海报数</small>
      </article>
      <article class="metric-card">
        <span>扫码访问</span>
        <strong>{{ stats.scan_count || 0 }}</strong>
        <small>好友通过海报进入门店页次数</small>
      </article>
      <article class="metric-card">
        <span>发券数量</span>
        <strong>{{ stats.reward_coupon_count || 0 }}</strong>
        <small>好友券与复购奖励券合计</small>
      </article>
      <article class="metric-card">
        <span>转化订单</span>
        <strong>{{ stats.conversion_count || 0 }}</strong>
        <small>通过分享码完成下单的订单</small>
      </article>
      <article class="metric-card accent">
        <span>转化金额</span>
        <strong>{{ yuan(stats.conversion_amount || 0) }}</strong>
        <small>裂变带来的顾客交易额</small>
      </article>
    </section>

    <section class="content-grid">
      <el-card class="panel-card" shadow="never">
        <template #header>
          <div class="panel-title">
            <div>
              <p class="eyebrow">ACTIVITY SETUP</p>
              <h2>活动设置</h2>
            </div>
            <el-switch
              v-model="form.enabled"
              active-text="开启"
              inactive-text="关闭"
            />
          </div>
        </template>

        <el-form label-position="top" class="share-form">
          <el-form-item label="海报标题">
            <el-input v-model="form.poster_title" placeholder="例如：好友扫码领券" maxlength="120" show-word-limit />
          </el-form-item>
          <el-form-item label="海报文案">
            <el-input
              v-model="form.poster_copy"
              type="textarea"
              :rows="3"
              maxlength="500"
              show-word-limit
              placeholder="告诉顾客为什么要分享给好友"
            />
          </el-form-item>

          <div class="form-row">
            <el-form-item label="好友券金额（元）">
              <el-input-number v-model="form.friend_coupon_amount_yuan" :min="0" :precision="2" :step="1" controls-position="right" />
            </el-form-item>
            <el-form-item label="好友券使用门槛（元）">
              <el-input-number v-model="form.friend_coupon_threshold_yuan" :min="0" :precision="2" :step="5" controls-position="right" />
            </el-form-item>
          </div>

          <div class="form-row">
            <el-form-item label="分享人奖励券（元）">
              <el-input-number v-model="form.referrer_coupon_amount_yuan" :min="0" :precision="2" :step="1" controls-position="right" />
            </el-form-item>
            <el-form-item label="奖励券使用门槛（元）">
              <el-input-number v-model="form.referrer_coupon_threshold_yuan" :min="0" :precision="2" :step="5" controls-position="right" />
            </el-form-item>
          </div>

          <el-form-item label="券有效期（天）">
            <el-input-number v-model="form.valid_days" :min="1" :max="365" :step="1" controls-position="right" />
          </el-form-item>

          <div class="form-actions">
            <el-button type="primary" :loading="saving" @click="save">保存活动设置</el-button>
            <el-button @click="router.push('/merchant/coupons')">查看券包/核销</el-button>
          </div>
        </el-form>
      </el-card>

      <el-card class="panel-card preview-card" shadow="never">
        <template #header>
          <div>
            <p class="eyebrow">POSTER PREVIEW</p>
            <h2>顾客端展示预览</h2>
          </div>
        </template>
        <div class="poster-preview" :class="{ disabled: !form.enabled }">
          <div class="poster-badge">{{ form.enabled ? '好友专享' : '未开启' }}</div>
          <h3>{{ form.poster_title || '好友扫码领券' }}</h3>
          <p>{{ form.poster_copy || '分享给好友，好友扫码领券下单，你也可以获得复购奖励。' }}</p>
          <div class="coupon-ticket">
            <span>好友券</span>
            <strong>{{ yuan(yuanToFen(form.friend_coupon_amount_yuan)) }}</strong>
            <small>满 {{ yuan(yuanToFen(form.friend_coupon_threshold_yuan)) }} 可用</small>
          </div>
          <div class="reward-line">
            分享人奖励：{{ yuan(yuanToFen(form.referrer_coupon_amount_yuan)) }} 复购券
          </div>
          <div class="qr-box">门店码</div>
        </div>
      </el-card>
    </section>

    <section class="table-grid">
      <el-card class="panel-card" shadow="never">
        <template #header>
          <div class="panel-title">
            <div>
              <p class="eyebrow">CAMPAIGNS</p>
              <h2>最近海报活动</h2>
            </div>
          </div>
        </template>
        <el-table v-loading="loading" :data="campaigns" empty-text="暂无海报活动">
          <el-table-column prop="store.name" label="门店" min-width="140" />
          <el-table-column prop="poster_title" label="海报标题" min-width="180" show-overflow-tooltip />
          <el-table-column prop="scan_count" label="扫码" width="90" />
          <el-table-column prop="conversion_count" label="下单" width="90" />
          <el-table-column label="转化金额" width="120">
            <template #default="{ row }">{{ yuan(row.conversion_amount || 0) }}</template>
          </el-table-column>
          <el-table-column prop="created_at" label="创建时间" min-width="160">
            <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
          </el-table-column>
        </el-table>
      </el-card>

      <el-card class="panel-card" shadow="never">
        <template #header>
          <div>
            <p class="eyebrow">COUPONS</p>
            <h2>最近奖励券</h2>
          </div>
        </template>
        <el-table :data="coupons" empty-text="暂无奖励券">
          <el-table-column prop="owner_phone" label="手机号" min-width="120" />
          <el-table-column prop="owner_type" label="类型" width="110">
            <template #default="{ row }">{{ ownerTypeLabel(row.owner_type) }}</template>
          </el-table-column>
          <el-table-column label="金额" width="100">
            <template #default="{ row }">{{ yuan(row.amount || 0) }}</template>
          </el-table-column>
          <el-table-column prop="status" label="状态" width="100">
            <template #default="{ row }">
              <el-tag :type="couponStatusType(row.status)" size="small">{{ couponStatusLabel(row.status) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="valid_to" label="有效期" min-width="150">
            <template #default="{ row }">{{ formatTime(row.valid_to) }}</template>
          </el-table-column>
        </el-table>
      </el-card>
    </section>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  fetchMerchantShareConfig,
  fetchMerchantShareStats,
  saveMerchantShareConfig
} from '../../api/modules'

const router = useRouter()
const loading = ref(false)
const saving = ref(false)
const stats = ref({})
const campaigns = ref([])
const coupons = ref([])
const config = ref({})

const form = reactive({
  enabled: false,
  poster_title: '好友扫码领券',
  poster_copy: '分享给好友，好友扫码领券下单，你也可以获得复购奖励。',
  friend_coupon_amount_yuan: 5,
  friend_coupon_threshold_yuan: 30,
  referrer_coupon_amount_yuan: 5,
  referrer_coupon_threshold_yuan: 30,
  valid_days: 30
})

const yuan = (cents) => `¥${(Number(cents || 0) / 100).toFixed(2)}`
const yuanToFen = (amount) => Math.round(Number(amount || 0) * 100)
const fenToYuan = (amount) => Number(((Number(amount || 0)) / 100).toFixed(2))

const applyConfig = (next = {}) => {
  config.value = next
  form.enabled = Boolean(next.enabled)
  form.poster_title = next.poster_title || '好友扫码领券'
  form.poster_copy = next.poster_copy || '分享给好友，好友扫码领券下单，你也可以获得复购奖励。'
  form.friend_coupon_amount_yuan = fenToYuan(next.friend_coupon_amount || 500)
  form.friend_coupon_threshold_yuan = fenToYuan(next.friend_coupon_threshold || 3000)
  form.referrer_coupon_amount_yuan = fenToYuan(next.referrer_coupon_amount || 500)
  form.referrer_coupon_threshold_yuan = fenToYuan(next.referrer_coupon_threshold || 3000)
  form.valid_days = next.valid_days || 30
}

const load = async () => {
  loading.value = true
  try {
    const [statsRes, configRes] = await Promise.all([
      fetchMerchantShareStats(),
      fetchMerchantShareConfig()
    ])
    stats.value = statsRes.data?.stats || {}
    campaigns.value = statsRes.data?.list || []
    coupons.value = statsRes.data?.coupons || []
    applyConfig(configRes.data?.config || statsRes.data?.config || {})
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '裂变看板加载失败')
  } finally {
    loading.value = false
  }
}

const save = async () => {
  saving.value = true
  try {
    const payload = {
      enabled: form.enabled,
      poster_title: form.poster_title,
      poster_copy: form.poster_copy,
      friend_coupon_amount: yuanToFen(form.friend_coupon_amount_yuan),
      friend_coupon_threshold: yuanToFen(form.friend_coupon_threshold_yuan),
      referrer_coupon_amount: yuanToFen(form.referrer_coupon_amount_yuan),
      referrer_coupon_threshold: yuanToFen(form.referrer_coupon_threshold_yuan),
      valid_days: form.valid_days
    }
    const res = await saveMerchantShareConfig(payload)
    applyConfig(res.data?.config || {})
    ElMessage.success(form.enabled ? '裂变活动已开启，顾客端将展示分享海报' : '裂变活动已关闭，顾客端不再展示活动')
    await load()
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '活动设置保存失败')
  } finally {
    saving.value = false
  }
}

const formatTime = (value) => {
  if (!value) return '-'
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

const ownerTypeLabel = (type) => {
  if (type === 'referrer') return '分享奖励'
  if (type === 'new_customer') return '好友券'
  return type || '-'
}

const couponStatusLabel = (status) => {
  const map = { unused: '未使用', used: '已核销', expired: '已过期', voided: '已作废' }
  return map[status] || status || '-'
}

const couponStatusType = (status) => {
  const map = { unused: 'success', used: 'info', expired: 'warning', voided: 'danger' }
  return map[status] || 'info'
}

onMounted(load)
</script>

<style scoped>
.share-page {
  display: grid;
  gap: 18px;
}

.share-hero {
  display: flex;
  justify-content: space-between;
  gap: 18px;
  padding: 28px;
  border-radius: 28px;
  color: #fff;
  background:
    radial-gradient(circle at 92% 10%, rgba(125, 211, 252, 0.45), transparent 28%),
    linear-gradient(135deg, #09203f 0%, #2563eb 52%, #06b6d4 100%);
  box-shadow: 0 22px 46px rgba(37, 99, 235, 0.22);
}

.share-hero h1,
.panel-card h2 {
  margin: 0;
}

.share-hero h1 {
  font-size: 34px;
  letter-spacing: -0.04em;
}

.hero-copy {
  max-width: 760px;
  margin: 10px 0 0;
  color: rgba(255, 255, 255, 0.82);
  line-height: 1.8;
}

.hero-actions,
.panel-title,
.form-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.metric-grid {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 14px;
}

.metric-card,
.panel-card {
  border: 1px solid rgba(148, 163, 184, 0.18);
  border-radius: 22px;
  background: rgba(255, 255, 255, 0.88);
  box-shadow: 0 16px 36px rgba(15, 23, 42, 0.06);
}

.metric-card {
  padding: 18px;
  display: grid;
  gap: 8px;
}

.metric-card span,
.metric-card small,
.hero-copy {
  color: #64748b;
}

.metric-card strong {
  font-size: 28px;
  color: #0f172a;
}

.metric-card.accent {
  background: linear-gradient(135deg, #ecfeff 0%, #eff6ff 100%);
}

.content-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.2fr) minmax(360px, 0.8fr);
  gap: 18px;
}

.table-grid {
  display: grid;
  gap: 18px;
}

.panel-card :deep(.el-card__header) {
  border-bottom: 1px solid rgba(148, 163, 184, 0.12);
  padding: 18px 20px;
}

.panel-card :deep(.el-card__body) {
  padding: 20px;
}

.eyebrow {
  margin: 0 0 6px;
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.16em;
}

.share-form {
  display: grid;
  gap: 10px;
}

.form-row {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.form-actions {
  justify-content: flex-start;
}

.preview-card {
  overflow: hidden;
}

.poster-preview {
  min-height: 420px;
  padding: 24px;
  border-radius: 26px;
  color: #fff;
  background:
    radial-gradient(circle at top right, rgba(191, 219, 254, 0.5), transparent 34%),
    linear-gradient(160deg, #0f172a 0%, #1d4ed8 56%, #06b6d4 100%);
  display: grid;
  gap: 16px;
  align-content: start;
}

.poster-preview.disabled {
  filter: grayscale(0.6);
  opacity: 0.72;
}

.poster-preview h3 {
  margin: 0;
  font-size: 30px;
}

.poster-preview p {
  margin: 0;
  color: rgba(255, 255, 255, 0.82);
  line-height: 1.8;
}

.poster-badge,
.reward-line,
.qr-box {
  width: fit-content;
  padding: 8px 12px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.16);
  border: 1px solid rgba(255, 255, 255, 0.22);
}

.coupon-ticket {
  padding: 18px;
  border-radius: 22px;
  background: rgba(255, 255, 255, 0.92);
  color: #0f172a;
  display: grid;
  gap: 4px;
}

.coupon-ticket strong {
  font-size: 34px;
  color: #2563eb;
}

.qr-box {
  width: 112px;
  height: 112px;
  border-radius: 22px;
  display: grid;
  place-items: center;
  background: #fff;
  color: #1d4ed8;
  font-weight: 900;
}

@media (max-width: 1200px) {
  .metric-grid,
  .content-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 720px) {
  .share-hero,
  .panel-title {
    flex-direction: column;
    align-items: flex-start;
  }

  .metric-grid,
  .content-grid,
  .form-row {
    grid-template-columns: 1fr;
  }
}
</style>
