<template>
  <div class="share-page">
    <section class="share-hero">
      <div>
        <p class="eyebrow light">REFERRAL GROWTH</p>
        <h1>营销裂变看板</h1>
        <p class="hero-copy">
          用分享海报、好友券和复购奖励把一次下单变成二次传播，持续观察扫码、领券、下单和复购转化。
        </p>
      </div>
      <div class="hero-actions">
        <el-date-picker
          v-model="dateRange"
          type="daterange"
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          value-format="YYYY-MM-DD"
          :clearable="false"
          @change="load"
        />
        <el-tag :type="config.enabled ? 'success' : 'info'" size="large">
          {{ config.enabled ? '活动已开启' : '活动未开启' }}
        </el-tag>
        <el-button @click="load">刷新数据</el-button>
      </div>
    </section>

    <section v-if="aiPrefillNote" class="ai-prefill-note">
      <div>
        <p class="eyebrow">AI DRAFT</p>
        <h2>AI 海报草稿已填入</h2>
        <p>{{ aiPrefillNote }}</p>
      </div>
      <el-button plain @click="aiPrefillNote = ''">收起</el-button>
    </section>

    <section class="metric-grid">
      <article class="metric-card">
        <span>分享海报</span>
        <strong>{{ stats.total_campaigns || 0 }}</strong>
        <small>顾客支付成功后生成的海报</small>
      </article>
      <article class="metric-card">
        <span>扫码访问</span>
        <strong>{{ stats.scan_count || 0 }}</strong>
        <small>好友通过海报进入门店页</small>
      </article>
      <article class="metric-card">
        <span>发券数量</span>
        <strong>{{ stats.reward_coupon_count || 0 }}</strong>
        <small>好友券与分享人奖励券</small>
      </article>
      <article class="metric-card">
        <span>转化订单</span>
        <strong>{{ stats.conversion_count || 0 }}</strong>
        <small>通过分享码完成支付的订单</small>
      </article>
      <article class="metric-card accent">
        <span>转化金额</span>
        <strong>{{ yuan(stats.conversion_amount || 0) }}</strong>
        <small>裂变活动带来的交易额</small>
      </article>
    </section>

    <section class="insight-grid">
      <el-card class="panel-card" shadow="never">
        <template #header>
          <div class="panel-title">
            <div>
              <p class="eyebrow">CONVERSION FUNNEL</p>
              <h2>扫码转化漏斗</h2>
            </div>
            <el-tag type="primary" effect="plain">当前筛选区间</el-tag>
          </div>
        </template>
        <div class="funnel">
          <div v-for="step in funnelSteps" :key="step.label" class="funnel-step">
            <div class="funnel-main">
              <span>{{ step.label }}</span>
              <strong>{{ step.value }}</strong>
            </div>
            <el-progress :percentage="step.percent" :stroke-width="12" :show-text="false" />
            <small>{{ step.tip }}</small>
          </div>
        </div>
      </el-card>

      <el-card class="panel-card" shadow="never">
        <template #header>
          <div>
            <p class="eyebrow">COUPON ROI</p>
            <h2>优惠券使用率</h2>
          </div>
        </template>
        <div class="coupon-usage">
          <el-progress type="dashboard" :percentage="couponUsageRate" :width="154">
            <template #default="{ percentage }">
              <div class="usage-rate">{{ percentage }}%</div>
              <small>已核销</small>
            </template>
          </el-progress>
          <div class="usage-list">
            <div><span>未使用</span><strong>{{ stats.unused_coupon_count || 0 }}</strong></div>
            <div><span>已核销</span><strong>{{ stats.used_coupon_count || 0 }}</strong></div>
            <div><span>已过期</span><strong>{{ stats.expired_coupon_count || 0 }}</strong></div>
            <div><span>已作废</span><strong>{{ stats.voided_coupon_count || 0 }}</strong></div>
          </div>
        </div>
      </el-card>
    </section>

    <section class="review-grid">
      <el-card class="panel-card review-card" shadow="never">
        <template #header>
          <div class="panel-title">
            <div>
              <p class="eyebrow">AI REVIEW</p>
              <h2>AI 活动效果复盘</h2>
            </div>
            <div class="ai-copy-actions">
              <small v-if="aiQuota.remaining !== undefined">今日剩余 {{ aiQuota.remaining }} 次</small>
              <el-button type="primary" :loading="reviewLoading" @click="generateShareReview">
                AI 复盘当前活动
              </el-button>
            </div>
          </div>
        </template>

        <div v-if="shareReview" class="review-body">
          <div class="review-decision">
            <el-tag :type="reviewTagType(shareReview.level)" size="large">{{ shareReview.decision }}</el-tag>
            <strong>扫码转化率 {{ percentText(shareReview.scan_conversion_rate) }}</strong>
            <strong>券核销率 {{ percentText(shareReview.coupon_use_rate) }}</strong>
            <strong>客单转化 {{ yuan(shareReview.avg_conversion_amount || 0) }}</strong>
          </div>

          <div class="review-panels">
            <article>
              <span>表现最好海报</span>
              <h3>{{ shareReview.best_campaign?.title || '暂无可复用海报' }}</h3>
              <p>{{ shareReview.best_campaign?.suggestion || '先开启活动并积累扫码、领券和下单数据。' }}</p>
            </article>
            <article>
              <span>优惠券判断</span>
              <h3>核销表现</h3>
              <p>{{ shareReview.coupon_advice }}</p>
            </article>
            <article>
              <span>建议停用/重写</span>
              <h3>{{ shareReview.stop_campaign?.title || '暂无明显低效海报' }}</h3>
              <p>{{ shareReview.stop_campaign?.suggestion || '继续观察不同海报和文案的扫码、下单差异。' }}</p>
            </article>
          </div>

          <div class="review-actions">
            <article v-for="item in shareReview.actions || []" :key="item.title">
              <strong>{{ item.title }}</strong>
              <span>{{ item.detail }}</span>
            </article>
          </div>

          <div class="review-execute">
            <el-button type="success" :loading="executingAction === 'amplify'" @click="executeReviewAction('amplify')">
              一键加大优惠
            </el-button>
            <el-button type="primary" :loading="executingAction === 'optimize'" @click="executeReviewAction('optimize')">
              一键优化海报
            </el-button>
            <el-button type="danger" plain :loading="executingAction === 'disable'" @click="executeReviewAction('disable')">
              停用低效海报
            </el-button>
          </div>
        </div>
        <el-empty v-else description="点击 AI 复盘后，系统会判断是否加大优惠、继续观察、优化文案或停用活动。" />
      </el-card>
    </section>

    <section class="content-grid">
      <el-card class="panel-card" shadow="never">
        <template #header>
          <div class="panel-title">
            <div>
              <p class="eyebrow">ACTIVITY SETUP</p>
              <h2>裂变活动设置</h2>
            </div>
            <el-switch v-model="form.enabled" active-text="开启" inactive-text="关闭" />
          </div>
        </template>

        <el-form label-position="top" class="share-form">
          <el-form-item label="海报标题">
            <el-input v-model="form.poster_title" maxlength="120" show-word-limit placeholder="例如：好友扫码领券" />
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
            <el-button @click="router.push('/merchant/coupons')">券包/核销管理</el-button>
          </div>
        </el-form>
      </el-card>

      <el-card class="panel-card preview-card" shadow="never">
        <template #header>
          <div>
            <p class="eyebrow">POSTER PREVIEW</p>
            <h2>顾客端海报预览</h2>
          </div>
        </template>
        <div class="poster-preview" :class="{ disabled: !form.enabled }">
          <div class="poster-badge">{{ form.enabled ? '好友专享' : '活动未开启' }}</div>
          <h3>{{ form.poster_title || '好友扫码领券' }}</h3>
          <p>{{ form.poster_copy || defaultCopy }}</p>
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

    <section class="action-grid">
      <el-card class="panel-card" shadow="never">
        <template #header>
          <div class="panel-title">
            <div>
              <p class="eyebrow">ACTION PLAYBOOK</p>
              <h2>一键可用营销动作</h2>
            </div>
            <div class="ai-copy-actions">
              <small v-if="aiQuota.remaining !== undefined">今日剩余 {{ aiQuota.remaining }} 次</small>
              <el-button type="primary" :loading="generatingCopy" @click="generateReferralCopy">
                AI 生成多版本文案
              </el-button>
            </div>
          </div>
        </template>
        <div class="action-list">
          <article v-for="action in displayActionCards" :key="action.title" class="action-card">
            <span>{{ action.type }}</span>
            <h3>{{ action.title }}</h3>
            <p>{{ action.text }}</p>
            <div class="action-buttons">
              <el-button type="primary" plain @click="copyText(action.copy)">复制内容</el-button>
              <el-button type="success" plain :loading="saving" @click="applyToPoster(action)">应用到海报</el-button>
              <el-button type="warning" plain :loading="creatingPromotion" @click="createPromotionFromAction(action)">转为优惠活动</el-button>
            </div>
          </article>
        </div>
        <div v-if="aiVariants.length" class="material-pack">
          <div>
            <p class="eyebrow">MATERIAL PACK</p>
            <strong>朋友圈素材包</strong>
            <span>把三版文案合并成一份，方便发给店员、代运营或直接保存到社群素材库。</span>
          </div>
          <el-button type="primary" @click="copyMaterialPack">复制整包素材</el-button>
        </div>
      </el-card>
    </section>

    <section class="table-grid">
      <el-card class="panel-card" shadow="never">
        <template #header>
          <div class="panel-title">
            <div>
              <p class="eyebrow">RANKING</p>
              <h2>海报贡献排行</h2>
            </div>
            <small>默认按下单数、扫码数排序</small>
          </div>
        </template>
        <el-table v-loading="loading" :data="rankedCampaigns" empty-text="暂无海报活动">
          <el-table-column label="排名" width="78">
            <template #default="{ $index }">#{{ $index + 1 }}</template>
          </el-table-column>
          <el-table-column prop="store.name" label="门店" min-width="140" />
          <el-table-column prop="poster_title" label="海报标题" min-width="180" show-overflow-tooltip />
          <el-table-column prop="customer_phone" label="分享人" min-width="130" />
          <el-table-column prop="scan_count" label="扫码" width="90" sortable />
          <el-table-column prop="lead_count" label="发券" width="90" sortable />
          <el-table-column prop="conversion_count" label="下单" width="90" sortable />
          <el-table-column label="转化金额" width="120" sortable>
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
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  amplifyMerchantShareOffer,
  createMerchantPromotion,
  disableLowMerchantShareCampaign,
  fetchMerchantShareConfig,
  fetchMerchantShareStats,
  generateMerchantAIReferralCopy,
  generateMerchantAIShareReview,
  optimizeMerchantSharePoster,
  saveMerchantShareConfig
} from '../../api/modules'

const router = useRouter()
const route = useRoute()
const loading = ref(false)
const saving = ref(false)
const generatingCopy = ref(false)
const creatingPromotion = ref(false)
const reviewLoading = ref(false)
const executingAction = ref('')
const stats = ref({})
const campaigns = ref([])
const coupons = ref([])
const config = ref({})
const aiVariants = ref([])
const aiPrefillNote = ref('')
const aiQuota = ref({})
const shareReview = ref(null)
const defaultCopy = '分享给好友，好友扫码领券下单，你也可以获得复购奖励。'

const today = new Date()
const thirtyDaysAgo = new Date(today)
thirtyDaysAgo.setDate(today.getDate() - 29)
const dateRange = ref([formatDate(thirtyDaysAgo), formatDate(today)])

const form = reactive({
  enabled: false,
  poster_title: '好友扫码领券',
  poster_copy: defaultCopy,
  friend_coupon_amount_yuan: 5,
  friend_coupon_threshold_yuan: 30,
  referrer_coupon_amount_yuan: 5,
  referrer_coupon_threshold_yuan: 30,
  valid_days: 30
})

const yuan = (cents) => `¥${(Number(cents || 0) / 100).toFixed(2)}`
const yuanToFen = (amount) => Math.round(Number(amount || 0) * 100)
const fenToYuan = (amount) => Number(((Number(amount || 0)) / 100).toFixed(2))

const couponUsageRate = computed(() => {
  const total = Number(stats.value.reward_coupon_count || 0)
  if (!total) return 0
  return Math.round((Number(stats.value.used_coupon_count || 0) / total) * 100)
})

const rankedCampaigns = computed(() => {
  return [...campaigns.value].sort((a, b) => {
    const orderDiff = Number(b.conversion_count || 0) - Number(a.conversion_count || 0)
    if (orderDiff !== 0) return orderDiff
    const scanDiff = Number(b.scan_count || 0) - Number(a.scan_count || 0)
    if (scanDiff !== 0) return scanDiff
    return Number(b.conversion_amount || 0) - Number(a.conversion_amount || 0)
  })
})

const funnelSteps = computed(() => {
  const posters = Number(stats.value.total_campaigns || 0)
  const scans = Number(stats.value.scan_count || 0)
  const couponsIssued = Number(stats.value.reward_coupon_count || 0)
  const orders = Number(stats.value.conversion_count || 0)
  return [
    { label: '生成海报', value: posters, percent: 100, tip: '顾客支付成功后生成可分享海报' },
    { label: '好友扫码', value: scans, percent: percent(scans, Math.max(posters, 1)), tip: `扫码/海报：${ratio(scans, posters)}` },
    { label: '自动发券', value: couponsIssued, percent: percent(couponsIssued, Math.max(scans * 2, 1)), tip: `发券/扫码：${ratio(couponsIssued, scans)}` },
    { label: '转化下单', value: orders, percent: percent(orders, Math.max(scans, 1)), tip: `下单/扫码：${ratio(orders, scans)}` }
  ]
})

const actionCards = computed(() => {
  const bestCampaign = rankedCampaigns.value[0]
  const posterTitle = form.poster_title || '好友扫码领券'
  const friendCoupon = yuan(yuanToFen(form.friend_coupon_amount_yuan))
  const threshold = yuan(yuanToFen(form.friend_coupon_threshold_yuan))
  const bestStore = bestCampaign?.store?.name || '本店'
  return [
    {
      type: '朋友圈文案',
      title: '老客分享领券',
      text: `突出 ${friendCoupon} 好友券，引导老顾客把门店推荐给朋友。`,
      copy: `${posterTitle}\n我在${bestStore}刚下单，体验不错。好友扫码可领${friendCoupon}优惠券，满${threshold}可用，适合第一次来试试。`
    },
    {
      type: '社群话术',
      title: '沉睡顾客召回',
      text: '适合发到微信群或私域群，强调限时和朋友一起用。',
      copy: `本周福利：好友扫码领${friendCoupon}券，满${threshold}可用。带朋友来下单，分享人也有复购奖励，数量有限，先到先得。`
    },
    {
      type: '短视频脚本',
      title: '15 秒门店引流脚本',
      text: '给商家拍手机短视频用，低成本执行。',
      copy: `镜头1：展示门店招牌和热销商品。\n旁白：今天给老顾客准备了一个隐藏福利。\n镜头2：展示扫码海报。\n旁白：把海报发给朋友，朋友扫码领${friendCoupon}券下单。\n镜头3：展示出餐或服务过程。\n旁白：朋友省钱，你也能拿复购奖励，来店直接用。`
    }
  ]
})

const displayActionCards = computed(() => {
  if (aiVariants.value.length) return aiVariants.value
  return actionCards.value
})

function formatDate(date) {
  const y = date.getFullYear()
  const m = String(date.getMonth() + 1).padStart(2, '0')
  const d = String(date.getDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}

const applyConfig = (next = {}) => {
  config.value = next
  form.enabled = Boolean(next.enabled)
  form.poster_title = next.poster_title || '好友扫码领券'
  form.poster_copy = next.poster_copy || defaultCopy
  form.friend_coupon_amount_yuan = fenToYuan(next.friend_coupon_amount || 500)
  form.friend_coupon_threshold_yuan = fenToYuan(next.friend_coupon_threshold || 3000)
  form.referrer_coupon_amount_yuan = fenToYuan(next.referrer_coupon_amount || 500)
  form.referrer_coupon_threshold_yuan = fenToYuan(next.referrer_coupon_threshold || 3000)
  form.valid_days = next.valid_days || 30
}

const applyAIShareDraft = () => {
  if (route.query.ai_prefill !== '1') return
  const raw = sessionStorage.getItem('merchant_ai_share_draft')
  if (!raw) return
  try {
    const draft = JSON.parse(raw)
    form.enabled = true
    form.poster_title = String(draft.poster_title || '好友扫码领券').slice(0, 80)
    form.poster_copy = String(draft.poster_copy || '').slice(0, 500) || form.poster_copy
    aiPrefillNote.value = String(draft.ai_note || '已把 AI 建议填入海报标题和分享文案，请确认优惠金额后保存。').slice(0, 800)
    sessionStorage.removeItem('merchant_ai_share_draft')
    router.replace('/merchant/share')
    ElMessage.success('已填入 AI 海报草稿，请确认后保存')
  } catch {
    sessionStorage.removeItem('merchant_ai_share_draft')
    ElMessage.warning('AI 海报草稿读取失败，请重新生成')
  }
}

const load = async () => {
  loading.value = true
  try {
    const params = {
      start_date: dateRange.value?.[0],
      end_date: dateRange.value?.[1]
    }
    const [statsRes, configRes] = await Promise.all([
      fetchMerchantShareStats(params),
      fetchMerchantShareConfig()
    ])
    stats.value = statsRes.data?.stats || {}
    campaigns.value = statsRes.data?.list || []
    coupons.value = statsRes.data?.coupons || []
    applyConfig(configRes.data?.config || statsRes.data?.config || {})
    applyAIShareDraft()
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

const generateReferralCopy = async () => {
  generatingCopy.value = true
  try {
    const bestCampaign = rankedCampaigns.value[0]
    const payload = {
      product_name: bestCampaign?.store?.name || '',
      goal: `提升裂变扫码、领券、下单和复购转化。当前扫码 ${stats.value.scan_count || 0} 次，发券 ${stats.value.reward_coupon_count || 0} 张，下单 ${stats.value.conversion_count || 0} 单。`,
      tone: '亲切、可信、适合本地生活商家，文案要能直接复制发布'
    }
    const res = await generateMerchantAIReferralCopy(payload)
    aiVariants.value = res.data?.variants || []
    aiQuota.value = res.data?.quota || {}
    if (!aiVariants.value.length) {
      ElMessage.warning('AI 已返回结果，但暂未解析到多版本文案，已保留默认模板')
      return
    }
    ElMessage.success('已生成朋友圈版、社群版和短视频口播版，并消耗 1 次 AI 额度')
  } catch (err) {
    ElMessage.error(err.response?.data?.message || 'AI 多版本文案生成失败')
  } finally {
    generatingCopy.value = false
  }
}

const applyToPoster = async (action) => {
  form.enabled = true
  form.poster_title = action.title || form.poster_title
  form.poster_copy = trimText(action.copy || action.text || defaultCopy, 480)
  await save()
  ElMessage.success('已把 AI 文案应用到裂变海报，顾客支付成功页会按新海报展示')
}

const createPromotionFromAction = async (action) => {
  creatingPromotion.value = true
  try {
    const now = new Date()
    const end = new Date(now)
    end.setDate(now.getDate() + Number(form.valid_days || 30))
    const payload = {
      title: trimText(action.title || 'AI 裂变优惠活动', 60),
      description: trimText(action.copy || action.text || '由 AI 裂变文案一键生成的优惠活动。', 280),
      type: 'amount',
      threshold: yuanToFen(form.friend_coupon_threshold_yuan),
      discount: yuanToFen(form.friend_coupon_amount_yuan),
      status: 'published',
      valid_from: formatDate(now),
      valid_to: formatDate(end)
    }
    await createMerchantPromotion(payload)
    ElMessage.success('已创建并发布优惠活动，可在“优惠活动”菜单继续调整')
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '创建优惠活动失败')
  } finally {
    creatingPromotion.value = false
  }
}

const copyMaterialPack = async () => {
  const source = displayActionCards.value
  const pack = source.map((item, index) => {
    return `${index + 1}. ${item.type}｜${item.title}\n${item.copy || item.text || ''}`
  }).join('\n\n')
  await copyText(`【${form.poster_title || '裂变营销素材包'}】\n\n${pack}`)
}

const generateShareReview = async () => {
  reviewLoading.value = true
  try {
    const payload = {
      start_date: dateRange.value?.[0],
      end_date: dateRange.value?.[1]
    }
    const res = await generateMerchantAIShareReview(payload)
    shareReview.value = res.data?.review || null
    aiQuota.value = res.data?.quota || aiQuota.value
    ElMessage.success('AI 活动复盘已生成，并消耗 1 次 AI 额度')
  } catch (err) {
    ElMessage.error(err.response?.data?.message || 'AI 活动复盘失败')
  } finally {
    reviewLoading.value = false
  }
}

const executeReviewAction = async (action) => {
  executingAction.value = action
  try {
    if (action === 'amplify') {
      const res = await amplifyMerchantShareOffer()
      applyConfig(res.data?.config || {})
      ElMessage.success('已加大好友券和分享人奖励，新的顾客海报会使用更新后的优惠')
    }
    if (action === 'optimize') {
      const res = await optimizeMerchantSharePoster()
      applyConfig(res.data?.config || {})
      ElMessage.success('已优化裂变海报文案，可在海报预览中查看')
    }
    if (action === 'disable') {
      await disableLowMerchantShareCampaign()
      ElMessage.success('已停用一张有扫码但无下单的低效海报')
    }
    await load()
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '执行复盘动作失败')
  } finally {
    executingAction.value = ''
  }
}

const trimText = (value, max) => {
  const text = String(value || '').trim()
  if (text.length <= max) return text
  return `${text.slice(0, max - 1)}…`
}

const copyText = async (text) => {
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success('已复制，可直接发朋友圈、社群或短视频脚本')
  } catch {
    ElMessage.error('复制失败，请手动选择内容复制')
  }
}

const percent = (value, base) => {
  if (!base) return 0
  return Math.max(0, Math.min(100, Math.round((Number(value || 0) / Number(base)) * 100)))
}

const ratio = (value, base) => {
  if (!base) return '0%'
  return `${Math.round((Number(value || 0) / Number(base)) * 100)}%`
}

const percentText = (value) => `${Math.round(Number(value || 0) * 100)}%`

const reviewTagType = (level) => {
  const map = { success: 'success', warning: 'warning', danger: 'danger', info: 'info' }
  return map[level] || 'primary'
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

.hero-actions {
  flex-wrap: wrap;
  justify-content: flex-end;
}

.ai-copy-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  flex-wrap: wrap;
}

.ai-copy-actions small {
  color: #64748b;
  font-weight: 700;
}

.metric-grid,
.insight-grid,
.content-grid,
.review-grid,
.table-grid {
  display: grid;
  gap: 14px;
}

.metric-grid {
  grid-template-columns: repeat(5, minmax(0, 1fr));
}

.insight-grid,
.content-grid {
  grid-template-columns: minmax(0, 1.15fr) minmax(360px, 0.85fr);
}

.metric-card,
.panel-card {
  border: 1px solid rgba(148, 163, 184, 0.18);
  border-radius: 22px;
  background: rgba(255, 255, 255, 0.9);
  box-shadow: 0 16px 36px rgba(15, 23, 42, 0.06);
}

.metric-card {
  padding: 18px;
  display: grid;
  gap: 8px;
}

.metric-card span,
.metric-card small {
  color: #64748b;
}

.metric-card strong {
  font-size: 28px;
  color: #0f172a;
}

.metric-card.accent {
  background: linear-gradient(135deg, #ecfeff 0%, #eff6ff 100%);
}

.panel-card :deep(.el-card__header) {
  border-bottom: 1px solid rgba(148, 163, 184, 0.12);
  padding: 18px 20px;
}

.panel-card :deep(.el-card__body) {
  padding: 20px;
}

.ai-prefill-note {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 18px 20px;
  border: 1px solid rgba(59, 130, 246, 0.22);
  border-radius: 18px;
  background: linear-gradient(135deg, rgba(239, 246, 255, 0.96), rgba(236, 253, 245, 0.9));
}

.ai-prefill-note h2,
.ai-prefill-note p {
  margin: 0;
}

.ai-prefill-note p:not(.eyebrow) {
  margin-top: 6px;
  color: #475569;
  line-height: 1.7;
}

.eyebrow {
  margin: 0 0 6px;
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.16em;
}

.eyebrow.light {
  color: #a7f3d0;
}

.funnel {
  display: grid;
  gap: 16px;
}

.funnel-step {
  padding: 14px;
  border-radius: 18px;
  background: #f8fbff;
  border: 1px solid #dbeafe;
}

.funnel-main {
  display: flex;
  justify-content: space-between;
  margin-bottom: 10px;
  color: #475569;
}

.funnel-main strong {
  color: #0f172a;
  font-size: 22px;
}

.funnel-step small {
  display: block;
  margin-top: 8px;
  color: #64748b;
}

.coupon-usage {
  display: grid;
  grid-template-columns: 180px 1fr;
  align-items: center;
  gap: 18px;
}

.usage-rate {
  font-size: 24px;
  font-weight: 900;
  color: #1d4ed8;
}

.usage-list {
  display: grid;
  gap: 10px;
}

.usage-list div {
  display: flex;
  justify-content: space-between;
  padding: 12px;
  border-radius: 14px;
  background: #f8fbff;
  color: #64748b;
}

.usage-list strong {
  color: #0f172a;
}

.review-card {
  overflow: hidden;
}

.review-body {
  display: grid;
  gap: 16px;
}

.review-decision {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  padding: 14px;
  border-radius: 18px;
  background: linear-gradient(135deg, #eff6ff, #f8fbff);
}

.review-decision strong {
  color: #0f172a;
}

.review-panels,
.review-actions {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.review-panels article,
.review-actions article {
  padding: 16px;
  border: 1px solid rgba(37, 99, 235, 0.14);
  border-radius: 18px;
  background: #fff;
}

.review-panels span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
}

.review-panels h3 {
  margin: 8px 0;
}

.review-panels p,
.review-actions span {
  color: #64748b;
  line-height: 1.7;
}

.review-actions strong {
  display: block;
  margin-bottom: 6px;
  color: #0f172a;
}

.review-execute {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  padding: 14px;
  border-radius: 18px;
  background: #f8fbff;
  border: 1px dashed rgba(37, 99, 235, 0.22);
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

.action-list {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14px;
}

.action-card {
  padding: 18px;
  border-radius: 20px;
  border: 1px solid #dbeafe;
  background: linear-gradient(135deg, #f8fbff, #eef6ff);
}

.action-card span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
}

.action-card h3 {
  margin: 8px 0;
}

.action-card p {
  color: #64748b;
  line-height: 1.7;
}

.action-buttons {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 14px;
}

.material-pack {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-top: 16px;
  padding: 16px 18px;
  border: 1px solid rgba(37, 99, 235, 0.16);
  border-radius: 20px;
  background: linear-gradient(135deg, #eff6ff, #f8fbff);
}

.material-pack strong {
  display: block;
  margin-bottom: 4px;
  color: #0f172a;
}

.material-pack span {
  color: #64748b;
}

@media (max-width: 1200px) {
  .metric-grid,
  .insight-grid,
  .content-grid,
  .action-list {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .review-panels,
  .review-actions {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 720px) {
  .share-hero,
  .panel-title {
    flex-direction: column;
    align-items: flex-start;
  }

  .metric-grid,
  .insight-grid,
  .content-grid,
  .form-row,
  .action-list,
  .coupon-usage {
    grid-template-columns: 1fr;
  }
}
</style>
