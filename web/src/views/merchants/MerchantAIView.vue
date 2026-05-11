<template>
  <div class="ai-stack">
    <section class="page-card hero-card">
      <div class="hero-row">
        <div>
          <div class="eyebrow">AI GROWTH ENGINE</div>
          <h2 class="page-title">AI Growth Center</h2>
          <p class="muted">
            Turn order, product, customer and coupon data into practical actions:
            campaign copy, short-video scripts, referral posters and coupon ideas.
          </p>
        </div>
        <div class="hero-actions">
          <el-button @click="load">Refresh</el-button>
          <el-button type="primary" :loading="videoLoading" @click="generateVideoScript">Video Script</el-button>
          <el-button type="success" :loading="generating" @click="generateDraft()">Create Campaign</el-button>
        </div>
      </div>

      <div class="summary-grid">
        <div class="summary-item"><span>Orders</span><strong>{{ insights.order_stats?.order_count || 0 }}</strong></div>
        <div class="summary-item"><span>Revenue</span><strong>¥{{ formatYuan(insights.order_stats?.trade_amount) }}</strong></div>
        <div class="summary-item"><span>Cancel Rate</span><strong>{{ cancelRate }}</strong></div>
        <div class="summary-item"><span>Customers</span><strong>{{ insights.customer_summary?.total || 0 }}</strong></div>
        <div class="summary-item"><span>High Value</span><strong>{{ insights.customer_summary?.high_value || 0 }}</strong></div>
        <div class="summary-item quota-item">
          <span>AI Daily Quota</span>
          <strong>{{ aiQuota.remaining ?? 0 }} / {{ aiQuota.limit ?? 0 }}</strong>
          <small>Used {{ aiQuota.used ?? 0 }} times today. Quota follows subscription plan.</small>
        </div>
      </div>
    </section>

    <section class="page-card">
      <div class="section-title">
        <div>
          <h2>Today AI Action List</h2>
          <p class="muted">Focus on actions that can bring repeat purchase, traffic and conversion.</p>
        </div>
      </div>
      <div class="action-card-grid">
        <div v-for="item in todayActions" :key="item.title" class="action-card" :class="item.tone">
          <span>{{ item.label }}</span>
          <strong>{{ item.title }}</strong>
          <p>{{ item.desc }}</p>
          <el-button size="small" type="primary" text :loading="item.loading" @click="item.action">{{ item.button }}</el-button>
        </div>
      </div>
    </section>

    <section class="grid-2">
      <div class="page-card">
        <div class="section-title">
          <div>
            <h2>Marketing Copy Generator</h2>
            <p class="muted">Create referral poster copy, recall coupon copy and store promotion scripts.</p>
          </div>
        </div>
        <el-form label-position="top">
          <el-form-item label="Scenario">
            <el-select v-model="copyForm.scenario">
              <el-option label="Referral poster" value="referral_poster" />
              <el-option label="New customer campaign" value="new_customer" />
              <el-option label="Dormant customer recall" value="dormant_recall" />
              <el-option label="VIP customer campaign" value="vip_campaign" />
              <el-option label="Social post" value="social_post" />
            </el-select>
          </el-form-item>
          <el-form-item label="Customer Segment">
            <el-select v-model="copyForm.customer_tag">
              <el-option label="New customer" value="new_customer" />
              <el-option label="Repeat customer" value="repeat_customer" />
              <el-option label="Dormant customer" value="dormant_customer" />
              <el-option label="High value customer" value="high_value_customer" />
              <el-option label="Nearby potential customer" value="nearby_customer" />
            </el-select>
          </el-form-item>
          <el-form-item label="Main Product / Service">
            <el-input v-model="copyForm.product_name" placeholder="Example: signature BBQ skewers, set meal, in-store service" />
          </el-form-item>
          <el-form-item label="Goal">
            <el-input v-model="copyForm.goal" type="textarea" :rows="3" />
          </el-form-item>
          <el-button type="primary" :loading="copyLoading" @click="generateMarketingCopy()">Generate AI Plan</el-button>
        </el-form>
      </div>

      <div class="page-card">
        <div class="section-title">
          <div>
            <h2>Generated Plan</h2>
            <p class="muted">Copy it to posters, activity pages, social posts or customer groups.</p>
          </div>
          <el-tag v-if="aiResult.provider">{{ aiResult.fallback ? 'Template fallback' : aiResult.provider }}</el-tag>
        </div>
        <div v-if="aiResult.content" class="copy-box">
          <pre>{{ aiResult.content }}</pre>
          <div class="action-row">
            <el-button type="primary" @click="copyText(aiResult.content)">Copy</el-button>
            <el-button @click="router.push('/merchant/promotions')">Go Campaigns</el-button>
          </div>
        </div>
        <el-empty v-else description="Choose a scenario and generate an actionable AI plan." />
      </div>
    </section>

    <section class="page-card video-card">
      <div class="section-title">
        <div>
          <div class="eyebrow">SHORT VIDEO MVP</div>
          <h2>AI Short Video Script</h2>
          <p class="muted">Generate a script ordinary merchants can shoot right away: hook, scenes, narration, title and hashtags.</p>
        </div>
      </div>
      <div class="video-grid">
        <el-form label-position="top">
          <div class="form-row">
            <el-form-item label="Platform">
              <el-select v-model="videoForm.platform">
                <el-option label="Douyin" value="douyin" />
                <el-option label="WeChat Channels" value="wechat_channels" />
                <el-option label="Xiaohongshu" value="xiaohongshu" />
                <el-option label="Kuaishou" value="kuaishou" />
              </el-select>
            </el-form-item>
            <el-form-item label="Duration">
              <el-select v-model="videoForm.duration">
                <el-option label="15 seconds" value="15s" />
                <el-option label="30 seconds" value="30s" />
                <el-option label="60 seconds" value="60s" />
              </el-select>
            </el-form-item>
          </div>
          <el-form-item label="Topic">
            <el-input v-model="videoForm.topic" placeholder="Example: BBQ dinner set for office workers" />
          </el-form-item>
          <el-form-item label="Selling Points">
            <el-input v-model="videoForm.selling_points" type="textarea" :rows="3" />
          </el-form-item>
          <el-button type="primary" :loading="videoLoading" @click="generateVideoScript">Generate Video Script</el-button>
        </el-form>
        <div class="copy-box video-result" v-if="videoResult.content">
          <pre>{{ videoResult.content }}</pre>
          <div class="action-row">
            <el-button type="primary" @click="copyText(videoResult.content)">Copy Script</el-button>
          </div>
        </div>
        <el-empty v-else description="Generate a short-video script from your real business data." />
      </div>
    </section>

    <section class="grid-2">
      <div class="page-card">
        <div class="section-title">
          <div>
            <h2>Referral Poster Loop</h2>
            <p class="muted">Customer pays, shares a poster, friend claims coupon, next order gets discount.</p>
          </div>
          <el-button size="small" @click="router.push('/merchant/coupons')">Coupon Wallet</el-button>
        </div>
        <div class="poster-grid">
          <div class="poster-card">
            <span>Share Events</span>
            <strong>{{ shareStats.summary?.total_shares || 0 }}</strong>
            <small>Referral links or poster visits.</small>
          </div>
          <div class="poster-card">
            <span>Coupons Issued</span>
            <strong>{{ shareStats.summary?.issued_coupons || 0 }}</strong>
            <small>Reward coupons from referral actions.</small>
          </div>
          <div class="poster-card">
            <span>Coupons Used</span>
            <strong>{{ shareStats.summary?.used_coupons || 0 }}</strong>
            <small>Coupons deducted in customer orders.</small>
          </div>
        </div>
      </div>

      <div class="page-card">
        <div class="section-title">
          <div>
            <h2>Product Signals</h2>
            <p class="muted">Use hot and slow-moving products to decide promotion focus.</p>
          </div>
        </div>
        <div class="signal-list">
          <div v-for="item in hotProducts.slice(0, 4)" :key="item.name" class="signal-row">
            <span>{{ item.name }}</span>
            <strong>{{ item.quantity || item.sales || 0 }} sold</strong>
          </div>
          <el-empty v-if="!hotProducts.length" description="No product sales data yet." />
        </div>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  fetchMerchantAIInsights,
  fetchMerchantAIQuota,
  fetchMerchantShareStats,
  generateMerchantAIMarketingCopy,
  generateMerchantPromotionDraft
} from '../../api/modules'

const router = useRouter()
const insights = ref({})
const shareStats = ref({})
const aiQuota = ref({})
const generating = ref(false)
const copyLoading = ref(false)
const videoLoading = ref(false)
const aiResult = ref({})
const videoResult = ref({})

const copyForm = reactive({
  scenario: 'referral_poster',
  customer_tag: 'new_customer',
  product_name: '',
  goal: 'Increase store visits and repeat purchases this week.'
})

const videoForm = reactive({
  platform: 'douyin',
  duration: '30s',
  topic: '',
  selling_points: 'Affordable, tasty, convenient, suitable for friends or office workers.'
})

const hotProducts = computed(() => insights.value.hot_products || insights.value.top_products || [])
const cancelRate = computed(() => {
  const stats = insights.value.order_stats || {}
  const total = Number(stats.order_count || 0)
  const cancelled = Number(stats.cancelled_count || 0)
  if (!total) return '0%'
  return `${Math.round((cancelled / total) * 100)}%`
})

const todayActions = computed(() => [
  {
    label: 'Traffic',
    title: 'Create a referral poster',
    desc: 'Use store code and reward coupon to encourage customers to share.',
    button: 'Generate Copy',
    tone: 'blue',
    loading: copyLoading.value,
    action: () => generateMarketingCopy('referral_poster')
  },
  {
    label: 'Retention',
    title: 'Recall dormant customers',
    desc: 'Turn silent customers into coupon targets with a clear reason to return.',
    button: 'Generate Recall',
    tone: 'orange',
    loading: copyLoading.value,
    action: () => generateMarketingCopy('dormant_recall')
  },
  {
    label: 'Content',
    title: 'Shoot one short video',
    desc: 'Generate an easy script for a phone-shot video.',
    button: 'Generate Script',
    tone: 'green',
    loading: videoLoading.value,
    action: generateVideoScript
  }
])

const formatYuan = (value) => (Number(value || 0) / 100).toFixed(2)

async function load() {
  const [insightRes, shareRes, quotaRes] = await Promise.all([
    fetchMerchantAIInsights(),
    fetchMerchantShareStats(),
    fetchMerchantAIQuota()
  ])
  insights.value = insightRes.data || {}
  shareStats.value = shareRes.data || {}
  aiQuota.value = quotaRes.data || {}
}

async function generateDraft() {
  generating.value = true
  try {
    await generateMerchantPromotionDraft({})
    ElMessage.success('Campaign draft created.')
    router.push('/merchant/promotions')
  } finally {
    generating.value = false
  }
}

async function generateMarketingCopy(scenario) {
  if (scenario) copyForm.scenario = scenario
  copyLoading.value = true
  try {
    const res = await generateMerchantAIMarketingCopy(copyForm)
    aiResult.value = res.data?.result || res.data || {}
    if (res.data?.quota) aiQuota.value = res.data.quota
    ElMessage.success('AI plan generated.')
  } catch (error) {
    ElMessage.error(error?.response?.data?.message || 'AI quota may be exhausted. Please try again later.')
  } finally {
    copyLoading.value = false
  }
}

async function generateVideoScript() {
  videoLoading.value = true
  try {
    const res = await generateMerchantAIMarketingCopy({
      scenario: 'short_video_script',
      customer_tag: copyForm.customer_tag,
      product_name: videoForm.topic || copyForm.product_name,
      goal: `Platform: ${videoForm.platform}; duration: ${videoForm.duration}; selling points: ${videoForm.selling_points}`
    })
    videoResult.value = res.data?.result || res.data || {}
    if (res.data?.quota) aiQuota.value = res.data.quota
    ElMessage.success('Short-video script generated.')
  } catch (error) {
    ElMessage.error(error?.response?.data?.message || 'AI generation failed.')
  } finally {
    videoLoading.value = false
  }
}

async function copyText(text) {
  await navigator.clipboard.writeText(text)
  ElMessage.success('Copied.')
}

onMounted(load)
</script>

<style scoped>
.ai-stack {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.hero-card {
  background: radial-gradient(circle at top right, rgba(43, 194, 123, 0.2), transparent 34%),
    linear-gradient(135deg, #082f49, #0f766e);
  color: white;
}

.hero-row,
.section-title,
.action-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
}

.hero-actions,
.action-row {
  flex-wrap: wrap;
}

.hero-card .muted {
  color: rgba(255, 255, 255, 0.78);
}

.summary-grid,
.poster-grid,
.action-card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 12px;
  margin-top: 20px;
}

.summary-item,
.poster-card,
.action-card {
  border: 1px solid rgba(148, 163, 184, 0.24);
  border-radius: 18px;
  padding: 16px;
  background: rgba(255, 255, 255, 0.82);
  color: #0f172a;
}

.summary-item span,
.poster-card span,
.action-card span {
  display: block;
  color: #64748b;
  font-size: 13px;
  margin-bottom: 8px;
}

.summary-item strong,
.poster-card strong {
  font-size: 28px;
}

.quota-item {
  background: #ecfdf5;
  border-color: #bbf7d0;
}

.quota-item small,
.poster-card small {
  display: block;
  margin-top: 6px;
  color: #64748b;
}

.grid-2,
.video-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 18px;
}

.form-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.copy-box {
  background: #0f172a;
  color: #e2e8f0;
  border-radius: 18px;
  padding: 16px;
  min-height: 220px;
}

.copy-box pre {
  white-space: pre-wrap;
  word-break: break-word;
  line-height: 1.7;
  margin: 0 0 16px;
  font-family: inherit;
}

.signal-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.signal-row {
  display: flex;
  justify-content: space-between;
  padding: 12px 0;
  border-bottom: 1px solid #e2e8f0;
}

.action-card.blue {
  background: #eff6ff;
}

.action-card.orange {
  background: #fff7ed;
}

.action-card.green {
  background: #f0fdf4;
}

@media (max-width: 900px) {
  .hero-row,
  .section-title,
  .grid-2,
  .video-grid,
  .form-row {
    display: block;
  }

  .hero-actions,
  .action-row {
    margin-top: 12px;
  }
}
</style>
