<template>
  <div class="ai-stack">
    <section class="page-card hero-card">
      <div class="card-toolbar">
        <div>
          <div class="eyebrow">AI GROWTH ENGINE</div>
          <h2 class="page-title">AI 经营增长中心</h2>
          <p class="muted">
            基于真实订单、商品销量、顾客画像、退款和裂变数据，生成可直接执行的经营建议、活动方案和短视频营销脚本。
          </p>
        </div>
        <div class="hero-actions">
          <el-button @click="load">刷新分析</el-button>
          <el-button type="primary" :loading="videoLoading" @click="generateVideoScript">生成短视频脚本</el-button>
          <el-button type="success" :loading="generating" @click="generateDraft()">生成优惠活动</el-button>
        </div>
      </div>

      <div class="summary-grid">
        <div class="summary-item"><span>订单数</span><strong>{{ insights.order_stats?.order_count || 0 }}</strong></div>
        <div class="summary-item"><span>交易额</span><strong>¥{{ formatYuan(insights.order_stats?.trade_amount) }}</strong></div>
        <div class="summary-item"><span>取消率</span><strong>{{ cancelRate }}</strong></div>
        <div class="summary-item"><span>顾客档案</span><strong>{{ insights.customer_summary?.total || 0 }}</strong></div>
        <div class="summary-item"><span>高价值顾客</span><strong>{{ insights.customer_summary?.high_value || 0 }}</strong></div>
        <div class="summary-item"><span>需召回顾客</span><strong>{{ recallCustomerCount }}</strong></div>
      </div>
    </section>

    <section class="page-card">
      <div class="section-title">
        <div>
          <h2>今日 AI 行动清单</h2>
          <p class="muted">把经营数据转成可执行动作，优先做能直接带来复购、引流和转化的事情。</p>
        </div>
      </div>
      <div class="action-card-grid">
        <div v-for="item in todayActions" :key="item.title" class="action-card" :class="item.tone">
          <span>{{ item.label }}</span>
          <strong>{{ item.title }}</strong>
          <p>{{ item.desc }}</p>
          <el-button size="small" type="primary" text :loading="item.loading" @click="item.action">
            {{ item.button }}
          </el-button>
        </div>
      </div>
    </section>

    <section class="grid-2">
      <div class="page-card ai-panel">
        <div class="section-title">
          <div>
            <h2>AI 营销方案生成器</h2>
            <p class="muted">生成裂变海报、朋友圈文案、召回券说明和门店推广话术。</p>
          </div>
        </div>
        <el-form label-position="top">
          <el-form-item label="使用场景">
            <el-select v-model="copyForm.scenario">
              <el-option label="裂变海报" value="裂变海报" />
              <el-option label="新客引流" value="新客引流" />
              <el-option label="沉睡顾客召回" value="沉睡顾客召回" />
              <el-option label="高价值顾客专属活动" value="高价值顾客专属活动" />
              <el-option label="朋友圈短文案" value="朋友圈短文案" />
            </el-select>
          </el-form-item>
          <el-form-item label="目标人群">
            <el-select v-model="copyForm.customer_tag">
              <el-option label="新客" value="新客" />
              <el-option label="复购顾客" value="复购顾客" />
              <el-option label="沉睡顾客" value="沉睡顾客" />
              <el-option label="高价值顾客" value="高价值顾客" />
              <el-option label="附近潜在顾客" value="附近潜在顾客" />
            </el-select>
          </el-form-item>
          <el-form-item label="主推商品 / 服务">
            <el-input v-model="copyForm.product_name" placeholder="例如：招牌烤串、双人套餐、到店护理服务" />
          </el-form-item>
          <el-form-item label="营销目标">
            <el-input
              v-model="copyForm.goal"
              type="textarea"
              :rows="3"
              placeholder="例如：老客分享海报，好友扫码领券下单，分享人获得复购奖励"
            />
          </el-form-item>
          <el-button type="primary" :loading="copyLoading" @click="generateMarketingCopy()">生成 AI 营销方案</el-button>
        </el-form>
      </div>

      <div class="page-card ai-output">
        <div class="section-title">
          <div>
            <h2>营销方案结果</h2>
            <p class="muted">可复制到海报、活动说明、朋友圈或商家群发消息中。</p>
          </div>
          <el-tag v-if="aiResult.provider">{{ aiResult.fallback ? '模板兜底' : aiResult.provider }}</el-tag>
        </div>
        <div v-if="aiResult.content" class="copy-box">
          <pre>{{ aiResult.content }}</pre>
          <div class="action-row">
            <el-button type="primary" @click="copyText(aiResult.content)">复制文案</el-button>
            <el-button @click="router.push('/merchant/promotions')">去活动页发布</el-button>
          </div>
        </div>
        <el-empty v-else description="先选择场景，生成一份可执行的 AI 营销方案。" />
      </div>
    </section>

    <section class="page-card video-card">
      <div class="section-title">
        <div>
          <div class="eyebrow">SHORT VIDEO MVP</div>
          <h2>AI 短视频营销脚本</h2>
          <p class="muted">
            先不直接生成视频，优先生成商家能马上拍摄的脚本：开头钩子、分镜、口播、画面建议和发布标题。这个功能适合做成付费版卖点。
          </p>
        </div>
      </div>
      <div class="video-grid">
        <el-form class="video-form" label-position="top">
          <div class="form-row">
            <el-form-item label="发布平台">
              <el-select v-model="videoForm.platform">
                <el-option label="抖音" value="抖音" />
                <el-option label="视频号" value="视频号" />
                <el-option label="小红书" value="小红书" />
                <el-option label="快手" value="快手" />
              </el-select>
            </el-form-item>
            <el-form-item label="视频时长">
              <el-select v-model="videoForm.duration">
                <el-option label="15 秒" value="15秒" />
                <el-option label="30 秒" value="30秒" />
                <el-option label="60 秒" value="60秒" />
              </el-select>
            </el-form-item>
          </div>
          <el-form-item label="视频风格">
            <el-select v-model="videoForm.style">
              <el-option label="探店种草" value="探店种草" />
              <el-option label="老板口播" value="老板口播" />
              <el-option label="新品上架" value="新品上架" />
              <el-option label="优惠促销" value="优惠促销" />
              <el-option label="顾客好评" value="顾客好评" />
            </el-select>
          </el-form-item>
          <el-form-item label="本次主推内容">
            <el-input v-model="videoForm.product_name" placeholder="例如：炭火小院烧烤、羊肉串、双人宵夜套餐" />
          </el-form-item>
          <el-form-item label="希望顾客做什么">
            <el-input v-model="videoForm.goal" type="textarea" :rows="3" placeholder="例如：扫码领券，到店下单，分享给附近朋友" />
          </el-form-item>
          <el-button type="primary" :loading="videoLoading" @click="generateVideoScript">生成短视频脚本</el-button>
        </el-form>
        <div class="copy-box video-output">
          <pre v-if="videoResult.content">{{ videoResult.content }}</pre>
          <el-empty v-else description="生成后会得到标题、分镜、口播和发布话题。" />
          <div v-if="videoResult.content" class="action-row">
            <el-button type="primary" @click="copyText(videoResult.content)">复制脚本</el-button>
            <el-button @click="generateMarketingCopy('朋友圈短文案')">配套生成朋友圈文案</el-button>
          </div>
        </div>
      </div>
    </section>

    <section class="page-card">
      <div class="section-title">
        <div>
          <h2>裂变海报与奖励券</h2>
          <p class="muted">顾客支付后生成分享海报；好友通过分享链接完成支付后，系统自动给新客和分享人发放奖励券。</p>
        </div>
        <el-button @click="router.push('/merchant/coupons')">进入券包 / 核销管理</el-button>
      </div>
      <div class="share-stats">
        <div><span>海报数</span><strong>{{ shareData.stats?.total_campaigns || 0 }}</strong></div>
        <div><span>扫码次数</span><strong>{{ shareData.stats?.scan_count || 0 }}</strong></div>
        <div><span>转化订单</span><strong>{{ shareData.stats?.conversion_count || 0 }}</strong></div>
        <div><span>转化金额</span><strong>¥{{ formatYuan(shareData.stats?.conversion_amount) }}</strong></div>
        <div><span>奖励券发放</span><strong>{{ shareData.stats?.reward_coupon_count || 0 }}</strong></div>
        <div><span>待使用券</span><strong>{{ shareData.stats?.unused_coupon_count || 0 }}</strong></div>
      </div>

      <el-tabs>
        <el-tab-pane label="海报效果">
          <el-table :data="shareData.list || []" empty-text="暂无裂变海报数据">
            <el-table-column prop="share_code" label="分享码" width="120" />
            <el-table-column label="门店" min-width="140">
              <template #default="{ row }">{{ row.store?.name || '-' }}</template>
            </el-table-column>
            <el-table-column prop="poster_title" label="海报标题" min-width="180" show-overflow-tooltip />
            <el-table-column prop="scan_count" label="扫码" width="80" />
            <el-table-column prop="conversion_count" label="转化" width="80" />
            <el-table-column label="转化金额" width="130">
              <template #default="{ row }">¥{{ formatYuan(row.conversion_amount) }}</template>
            </el-table-column>
          </el-table>
        </el-tab-pane>
        <el-tab-pane label="奖励券记录">
          <el-table :data="shareData.coupons || []" empty-text="暂无奖励券记录">
            <el-table-column prop="coupon_no" label="券号" min-width="170" />
            <el-table-column label="归属" width="110">
              <template #default="{ row }">{{ ownerTypeLabel(row.owner_type) }}</template>
            </el-table-column>
            <el-table-column prop="owner_phone" label="手机号" min-width="130" />
            <el-table-column prop="title" label="券名称" min-width="180" />
            <el-table-column label="优惠" width="140">
              <template #default="{ row }">满 ¥{{ formatYuan(row.threshold) }} 减 ¥{{ formatYuan(row.amount) }}</template>
            </el-table-column>
            <el-table-column label="状态" width="100">
              <template #default="{ row }">
                <el-tag :type="couponStatusType(row.status)">{{ couponStatusLabel(row.status) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="有效期" min-width="180">
              <template #default="{ row }">{{ formatTime(row.valid_to) }}</template>
            </el-table-column>
          </el-table>
        </el-tab-pane>
      </el-tabs>
    </section>

    <section class="page-card">
      <div class="section-title">
        <div>
          <h2>顾客画像与 AI 维护建议</h2>
          <p class="muted">按消费金额和最近行为识别高价值、复购、沉睡和流失风险顾客。</p>
        </div>
      </div>
      <el-table :data="insights.customer_profiles || []" empty-text="暂无顾客订单数据">
        <el-table-column prop="customer_phone" label="顾客手机号" min-width="140" />
        <el-table-column prop="store_name" label="最近门店" min-width="150" />
        <el-table-column prop="order_count" label="消费次数" width="100" />
        <el-table-column label="累计消费" width="120">
          <template #default="{ row }">¥{{ formatYuan(row.total_amount) }}</template>
        </el-table-column>
        <el-table-column label="AI 标签" width="130">
          <template #default="{ row }">
            <el-tag :type="tagType(row.ai_tag)">{{ row.ai_tag || '待分析' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="ai_suggestion" label="建议动作" min-width="260" show-overflow-tooltip />
        <el-table-column label="快捷生成" width="170">
          <template #default="{ row }">
            <el-button size="small" type="primary" text :loading="copyLoading" @click="generateMarketingCopy('专属活动', row.ai_tag)">
              生成专属文案
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </section>

    <section class="grid-2">
      <div class="page-card">
        <h2 class="page-title">热销商品</h2>
        <el-table :data="insights.hot_products || []" empty-text="暂无商品销量数据">
          <el-table-column prop="name" label="商品" />
          <el-table-column prop="quantity" label="销量" width="90" />
          <el-table-column label="销售额" width="120">
            <template #default="{ row }">¥{{ formatYuan(row.amount) }}</template>
          </el-table-column>
        </el-table>
      </div>
      <div class="page-card">
        <h2 class="page-title">低动销商品</h2>
        <el-table :data="insights.slow_products || []" empty-text="暂无低动销商品">
          <el-table-column prop="name" label="商品" />
          <el-table-column prop="quantity" label="销量" width="90" />
          <el-table-column label="建议" min-width="180">
            <template #default>优化图片 / 标题，或设置折扣活动测试。</template>
          </el-table-column>
        </el-table>
      </div>
    </section>

    <section class="page-card">
      <h2 class="page-title">经营建议</h2>
      <div class="insight-list">
        <div v-for="(item, index) in insights.insights || []" :key="`${item.title}-${index}`" class="insight-item" :class="`insight-${item.type}`">
          <el-tag size="small" :type="insightTagType(item.type)">{{ insightTypeLabel(item.type) }}</el-tag>
          <div>
            <strong>{{ item.title }}</strong>
            <p>{{ item.content }}</p>
            <el-button class="inline-action" type="primary" text :loading="generating" @click="generateDraft()">
              转为优惠活动
            </el-button>
          </div>
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
  fetchMerchantShareStats,
  generateMerchantAIMarketingCopy,
  generateMerchantPromotionDraft
} from '../../api/modules'

const router = useRouter()
const insights = ref({})
const shareData = ref({ stats: {}, list: [], coupons: [] })
const generating = ref(false)
const copyLoading = ref(false)
const videoLoading = ref(false)
const aiResult = ref({})
const videoResult = ref({})

const copyForm = reactive({
  scenario: '裂变海报',
  customer_tag: '新客',
  product_name: '',
  goal: '老客分享海报，好友扫码领券下单，分享人获得复购奖励。'
})

const videoForm = reactive({
  platform: '抖音',
  duration: '30秒',
  style: '探店种草',
  product_name: '',
  goal: '让附近顾客扫码领券，到店下单并分享给朋友。'
})

const load = async () => {
  const [insightRes, shareRes] = await Promise.all([
    fetchMerchantAIInsights(),
    fetchMerchantShareStats().catch(() => ({ data: { stats: {}, list: [], coupons: [] } }))
  ])
  insights.value = insightRes.data || {}
  shareData.value = shareRes.data || { stats: {}, list: [], coupons: [] }
}

const generateDraft = async (customerTag = '') => {
  generating.value = true
  try {
    await generateMerchantPromotionDraft({ customer_tag: customerTag })
    ElMessage.success('已生成优惠活动草稿，可继续编辑后发布')
    router.push('/merchant/promotions')
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '生成活动草稿失败')
  } finally {
    generating.value = false
  }
}

const generateMarketingCopy = async (scenario = '', customerTag = '') => {
  copyLoading.value = true
  try {
    const payload = {
      scenario: scenario || copyForm.scenario,
      customer_tag: customerTag || copyForm.customer_tag,
      product_name: copyForm.product_name,
      goal: copyForm.goal
    }
    const res = await generateMerchantAIMarketingCopy(payload)
    aiResult.value = res.data.result || {}
    ElMessage.success(aiResult.value.fallback ? '已生成模板方案，配置 AI Key 后可启用真实模型' : 'AI 营销方案已生成')
  } catch (err) {
    ElMessage.error(err.response?.data?.message || 'AI 生成失败')
  } finally {
    copyLoading.value = false
  }
}

const generateVideoScript = async () => {
  videoLoading.value = true
  try {
    const payload = {
      scenario: `短视频脚本 / ${videoForm.platform} / ${videoForm.duration} / ${videoForm.style}`,
      customer_tag: copyForm.customer_tag || '附近潜在顾客',
      product_name: videoForm.product_name || copyForm.product_name || topHotProduct.value || '门店招牌商品',
      goal: `${videoForm.goal} 请输出可直接拍摄的脚本，包含标题、前 3 秒钩子、分镜、口播、画面建议、发布文案和话题。`
    }
    const res = await generateMerchantAIMarketingCopy(payload)
    videoResult.value = res.data.result || {}
    ElMessage.success(videoResult.value.fallback ? '已生成短视频脚本模板' : 'AI 短视频脚本已生成')
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '短视频脚本生成失败')
  } finally {
    videoLoading.value = false
  }
}

const copyText = async (content) => {
  await navigator.clipboard?.writeText(content || '')
  ElMessage.success('内容已复制')
}

const formatRate = (value) => `${Math.round((Number(value) || 0) * 100)}%`
const formatYuan = (value) => (Number(value || 0) / 100).toFixed(2)
const formatTime = (value) => value ? new Date(value).toLocaleString('zh-CN') : '-'

const cancelRate = computed(() => {
  const total = Number(insights.value.order_stats?.order_count || 0)
  const closed = Number(insights.value.order_stats?.closed_count || 0)
  return total ? formatRate(closed / total) : '0%'
})

const recallCustomerCount = computed(() => {
  const summary = insights.value.customer_summary || {}
  return Number(summary.sleeping || 0) + Number(summary.risk || 0)
})

const topHotProduct = computed(() => insights.value.hot_products?.[0]?.name || '')

const todayActions = computed(() => [
  {
    label: '热销放大',
    title: topHotProduct.value ? `围绕「${topHotProduct.value}」做短视频种草` : '先选择一个招牌商品做短视频',
    desc: '把热销商品做成短视频脚本和裂变券，更容易形成“看见内容 -> 领券 -> 下单”的闭环。',
    button: '生成脚本',
    tone: 'tone-blue',
    loading: videoLoading.value,
    action: generateVideoScript
  },
  {
    label: '复购提升',
    title: '给老客发一张复购奖励券',
    desc: '对已下单顾客推送满减券或专属福利，优先提升二次消费，而不是只追求新流量。',
    button: '生成活动',
    tone: 'tone-green',
    loading: generating.value,
    action: () => generateDraft('复购顾客')
  },
  {
    label: '流失召回',
    title: `${recallCustomerCount.value} 位顾客建议召回`,
    desc: '沉睡和流失风险顾客适合低门槛券、限时福利和老板口播式提醒。',
    button: '生成召回文案',
    tone: 'tone-orange',
    loading: copyLoading.value,
    action: () => generateMarketingCopy('沉睡顾客召回', '沉睡顾客')
  },
  {
    label: '裂变引流',
    title: '让支付成功页变成分享入口',
    desc: '顾客付款后分享海报，好友领券下单，原顾客得奖励券，形成低成本新客来源。',
    button: '生成裂变文案',
    tone: 'tone-purple',
    loading: copyLoading.value,
    action: () => generateMarketingCopy('裂变海报', '新客')
  }
])

const insightTagType = (type) => ({ warning: 'warning', action: 'primary', success: 'success', info: 'info' }[type] || 'info')
const insightTypeLabel = (type) => ({ warning: '风险', action: '建议', success: '亮点', info: '提示' }[type] || '提示')
const tagType = (tag) => ({ 高价值顾客: 'danger', 复购顾客: 'success', 沉睡顾客: 'warning', 流失风险: 'warning', 新顾客: 'primary' }[tag] || 'info')
const ownerTypeLabel = (type) => ({ referrer: '分享人', new_customer: '新客' }[type] || type || '-')
const couponStatusLabel = (status) => ({ unused: '待使用', used: '已使用', expired: '已过期', disabled: '已作废' }[status] || status || '-')
const couponStatusType = (status) => ({ unused: 'success', used: 'info', expired: 'warning', disabled: 'danger' }[status] || 'info')

onMounted(load)
</script>

<style scoped>
.ai-stack {
  display: grid;
  gap: 20px;
}

.hero-card {
  border: 1px solid #dbeafe;
  background:
    radial-gradient(circle at top right, rgba(14, 165, 233, 0.16), transparent 30%),
    #ffffff;
}

.video-card {
  border: 1px solid #bbf7d0;
  background:
    radial-gradient(circle at top left, rgba(34, 197, 94, 0.16), transparent 28%),
    #ffffff;
}

.card-toolbar,
.section-title {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: flex-start;
}

.eyebrow {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.16em;
}

.hero-actions,
.action-row {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.muted {
  margin: 0;
  color: #64748b;
  line-height: 1.7;
}

.summary-grid,
.share-stats {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 14px;
  margin-top: 18px;
}

.summary-item,
.share-stats div {
  padding: 16px;
  border: 1px solid #e5edf9;
  border-radius: 14px;
  background: rgba(248, 251, 255, 0.9);
}

.summary-item span,
.share-stats span {
  display: block;
  color: #64748b;
  font-size: 13px;
}

.summary-item strong,
.share-stats strong {
  display: block;
  margin-top: 8px;
  font-size: 22px;
}

.action-card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(230px, 1fr));
  gap: 14px;
  margin-top: 18px;
}

.action-card {
  min-height: 170px;
  padding: 18px;
  border-radius: 18px;
  border: 1px solid #e5edf9;
  background: #f8fafc;
}

.action-card span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.08em;
}

.action-card strong {
  display: block;
  margin-top: 10px;
  font-size: 18px;
}

.action-card p {
  color: #64748b;
  line-height: 1.7;
}

.tone-blue {
  background: linear-gradient(145deg, #eff6ff, #ffffff);
}

.tone-green {
  background: linear-gradient(145deg, #ecfdf5, #ffffff);
}

.tone-orange {
  background: linear-gradient(145deg, #fff7ed, #ffffff);
}

.tone-purple {
  background: linear-gradient(145deg, #f5f3ff, #ffffff);
}

.grid-2 {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(340px, 1fr));
  gap: 20px;
}

.ai-panel,
.ai-output {
  display: grid;
  gap: 16px;
}

.video-grid {
  display: grid;
  grid-template-columns: minmax(300px, 0.9fr) minmax(340px, 1.1fr);
  gap: 20px;
  margin-top: 18px;
}

.form-row {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.copy-box {
  display: grid;
  gap: 14px;
}

.copy-box pre {
  min-height: 320px;
  margin: 0;
  padding: 18px;
  border-radius: 16px;
  white-space: pre-wrap;
  word-break: break-word;
  line-height: 1.8;
  color: #0f172a;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
}

.video-output pre {
  min-height: 360px;
}

.insight-list {
  display: grid;
  gap: 12px;
}

.insight-item {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 12px;
  align-items: flex-start;
  padding: 16px;
  border-radius: 14px;
  border: 1px solid #e5edf9;
  background: #f8fafc;
}

.insight-item p {
  margin: 6px 0 0;
  color: #64748b;
  line-height: 1.7;
}

.inline-action {
  margin-top: 8px;
  padding-left: 0;
}

.insight-warning {
  background: #fffbeb;
  border-color: #fde68a;
}

.insight-action {
  background: #eff6ff;
  border-color: #bfdbfe;
}

.insight-success {
  background: #ecfdf5;
  border-color: #bbf7d0;
}

@media (max-width: 900px) {
  .video-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 720px) {
  .card-toolbar,
  .section-title {
    flex-direction: column;
  }

  .form-row {
    grid-template-columns: 1fr;
  }

  .insight-item {
    grid-template-columns: 1fr;
  }
}
</style>
