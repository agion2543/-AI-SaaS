<template>
  <div class="ai-page">
    <section class="ai-cockpit">
      <div class="cockpit-copy">
        <div class="eyebrow">AI OPERATING COPILOT</div>
        <h1>AI 智能经营驾驶舱</h1>
        <p>
          用真实订单、商品、顾客和券包数据，自动给出今日可执行的经营动作。
          重点不是“看报表”，而是帮助商家马上做引流、复购和内容营销。
        </p>
        <div class="cockpit-actions">
          <el-button class="ghost-button" @click="load">刷新分析</el-button>
          <el-button type="primary" :disabled="!canUseAI" :loading="copyLoading" @click="generateMarketingCopy('referral_poster')">生成裂变方案</el-button>
          <el-button type="success" :disabled="!canUseAI" :loading="videoLoading" @click="generateVideoScript">生成短视频脚本</el-button>
        </div>
      </div>

      <div class="quota-orb" :class="quotaStatus">
        <span>今日 AI 可用额度</span>
        <strong>{{ aiQuota.remaining ?? 0 }}</strong>
        <small>/ {{ aiQuota.limit ?? 0 }} 次</small>
        <div class="quota-progress"><i :style="{ width: quotaPercent + '%' }"></i></div>
        <p>{{ quotaHint }}</p>
        <el-button v-if="!canUseAI" type="primary" plain @click="router.push('/merchant/subscription')">查看订阅额度</el-button>
      </div>
    </section>

    <section v-if="aiError.visible" class="ai-error-card">
      <div>
        <span>{{ aiError.title }}</span>
        <strong>{{ aiError.message }}</strong>
        <p>{{ aiError.suggestion }}</p>
      </div>
      <div class="ai-error-actions">
        <el-button v-if="!canUseAI" type="primary" @click="router.push('/merchant/subscription')">升级/续费套餐</el-button>
        <el-button plain @click="load">刷新额度</el-button>
        <el-button text @click="aiError.visible = false">知道了</el-button>
      </div>
    </section>

    <section class="module-grid">
      <button
        v-for="item in aiModules"
        :key="item.key"
        type="button"
        :class="['module-card', { active: activeModule === item.key }]"
        @click="activateAIModule(item)"
      >
        <span>{{ item.label }}</span>
        <strong>{{ item.title }}</strong>
        <small>{{ item.desc }}</small>
      </button>
    </section>

    <section v-if="scenarioContext.visible" class="scenario-context">
      <div>
        <span>{{ scenarioContext.label }}</span>
        <strong>{{ scenarioContext.title }}</strong>
        <p>{{ scenarioContext.desc }}</p>
      </div>
        <el-button type="primary" :disabled="!canUseAI" :loading="scenarioContext.loading" @click="runCurrentScenario">
        {{ scenarioContext.button }}
      </el-button>
    </section>

    <section class="execution-strip">
      <div>
        <span>1</span>
        <strong>看经营数据</strong>
        <small>订单、顾客、商品、券包统一进驾驶舱</small>
      </div>
      <div>
        <span>2</span>
        <strong>选 AI 场景</strong>
        <small>复盘、文案、海报、脚本、商品优化</small>
      </div>
      <div>
        <span>3</span>
        <strong>生成执行素材</strong>
        <small>直接复制或生成活动草稿</small>
      </div>
      <div>
        <span>4</span>
        <strong>回到裂变追踪</strong>
        <small>观察扫码、领券、下单和核销</small>
      </div>
    </section>

    <section class="metric-grid">
      <div class="metric-card primary">
        <span>今日交易额</span>
        <strong>¥{{ formatYuan(insights.order_stats?.trade_amount) }}</strong>
        <small>顾客扫码点单收入</small>
      </div>
      <div class="metric-card">
        <span>订单数</span>
        <strong>{{ insights.order_stats?.order_count || 0 }}</strong>
        <small>用于判断今日经营活跃度</small>
      </div>
      <div class="metric-card">
        <span>取消率</span>
        <strong>{{ cancelRate }}</strong>
        <small>偏高时建议检查履约体验</small>
      </div>
      <div class="metric-card">
        <span>顾客档案</span>
        <strong>{{ insights.customer_summary?.total || 0 }}</strong>
        <small>已沉淀手机号和消费记录</small>
      </div>
      <div class="metric-card accent">
        <span>高价值顾客</span>
        <strong>{{ insights.customer_summary?.high_value || 0 }}</strong>
        <small>适合投放专属券</small>
      </div>
    </section>

    <section class="workbench-grid">
      <div class="assist-row">
        <div class="panel-card assist-card">
          <div class="section-head">
            <div>
              <div class="eyebrow dark">NEXT BEST ACTION</div>
              <h2>今日 AI 行动清单</h2>
              <p>每一条建议都对应一个可执行按钮，方便商家直接落地。</p>
            </div>
          </div>
          <div class="action-list">
            <div v-for="item in todayActions" :key="item.title" class="action-card" :class="item.tone">
              <div class="action-icon">{{ item.icon }}</div>
              <div>
                <span>{{ item.label }}</span>
                <strong>{{ item.title }}</strong>
                <p>{{ item.desc }}</p>
              </div>
              <el-button size="small" type="primary" text :disabled="!canUseAI" :loading="item.loading" @click="item.action">{{ item.button }}</el-button>
            </div>
          </div>
        </div>

        <div class="panel-card loop-card assist-card">
          <div class="section-head compact">
            <div>
              <div class="eyebrow dark">REFERRAL LOOP</div>
              <h2>裂变海报闭环</h2>
              <p>付款后分享海报，好友领券下单，下次消费自动抵扣。</p>
            </div>
            <el-button @click="router.push('/merchant/coupons')">券包/核销</el-button>
          </div>
          <div class="poster-grid">
            <div class="poster-card">
              <span>分享次数</span>
              <strong>{{ shareStats.summary?.total_shares || 0 }}</strong>
              <small>海报或链接访问</small>
            </div>
            <div class="poster-card">
              <span>发券数量</span>
              <strong>{{ shareStats.summary?.issued_coupons || 0 }}</strong>
              <small>裂变奖励券</small>
            </div>
            <div class="poster-card">
              <span>核销数量</span>
              <strong>{{ shareStats.summary?.used_coupons || 0 }}</strong>
              <small>订单抵扣券</small>
            </div>
          </div>
        </div>
      </div>

      <div class="studio-row">
        <div class="panel-card generator-card">
          <div class="section-head">
            <div>
              <div class="eyebrow dark">{{ studioConfig.eyebrow }}</div>
              <h2>{{ studioConfig.title }}</h2>
              <p>{{ studioConfig.description }}</p>
            </div>
          </div>
          <el-form label-position="top" class="studio-form">
            <div class="form-row">
              <el-form-item :label="studioConfig.scenarioLabel">
                <el-select v-model="copyForm.scenario">
                  <el-option
                    v-for="option in studioConfig.scenarioOptions"
                    :key="option.value"
                    :label="option.label"
                    :value="option.value"
                  />
                </el-select>
              </el-form-item>
              <el-form-item :label="studioConfig.audienceLabel">
                <el-select v-model="copyForm.customer_tag">
                  <el-option label="新顾客" value="new_customer" />
                  <el-option label="复购顾客" value="repeat_customer" />
                  <el-option label="沉睡顾客" value="dormant_customer" />
                  <el-option label="高价值顾客" value="high_value_customer" />
                  <el-option label="附近潜在顾客" value="nearby_customer" />
                </el-select>
              </el-form-item>
            </div>
            <el-form-item :label="studioConfig.subjectLabel">
              <el-input v-model="copyForm.product_name" :placeholder="studioConfig.subjectPlaceholder" />
            </el-form-item>
            <el-form-item :label="studioConfig.goalLabel">
              <el-input v-model="copyForm.goal" type="textarea" :rows="3" />
            </el-form-item>
            <div class="form-actions">
              <el-button type="primary" :disabled="!canUseAI" :loading="copyLoading" @click="generateMarketingCopy()">{{ studioConfig.primaryAction }}</el-button>
              <el-button v-if="studioConfig.showDraftButton" :loading="generating" @click="generateDraft()">生成活动草稿</el-button>
            </div>
          </el-form>
        </div>

        <div class="panel-card result-card">
          <div class="section-head compact">
            <div>
              <div class="eyebrow dark">AI OUTPUT</div>
              <h2>生成结果</h2>
            </div>
            <el-tag v-if="aiResult.provider">{{ aiResult.fallback ? '模板兜底' : aiResult.provider }}</el-tag>
          </div>
          <div v-if="aiResult.content" class="output-meta">
            <span>模型：{{ aiResult.model || aiQuota.model || '-' }}</span>
            <span>Token：{{ aiResult.total_tokens || 0 }}</span>
            <span>预估成本：{{ formatCost(aiResult.estimated_cost_cents) }}</span>
          </div>
          <div v-if="structuredOutput.length" class="structured-output">
            <article v-for="item in structuredOutput" :key="item.key">
              <span>{{ item.label }}</span>
              <strong>{{ item.title }}</strong>
              <p>{{ item.text }}</p>
              <div class="structured-actions">
                <el-button size="small" type="primary" plain @click="copyText(item.copy || item.text)">复制</el-button>
                <el-button v-if="item.action === 'promotion'" size="small" type="success" plain @click="prefillPromotionFromStructured(item)">填入活动表单</el-button>
                <el-button v-if="item.action === 'share'" size="small" plain @click="prefillShareFromStructured(item)">填入海报设置</el-button>
                <el-button v-if="item.action === 'product'" size="small" plain @click="prefillProductFromStructured(item)">填入商品表单</el-button>
              </div>
            </article>
          </div>
          <div v-if="aiResult.content" class="copy-box" :class="{ compact: studioConfig.resultCompact }">
            <pre>{{ aiResult.content }}</pre>
            <div class="inline-actions">
              <el-button type="primary" @click="copyText(aiResult.content)">{{ studioConfig.copyAction }}</el-button>
              <el-button v-if="studioConfig.showPromotionActions" type="success" :loading="generating" @click="createPromotionFromAI">一键生成活动草稿</el-button>
              <el-button v-if="studioConfig.showPromotionActions" plain @click="prefillPromotionFromStructured()">填入活动表单</el-button>
              <el-button v-if="studioConfig.showProductActions" plain @click="prefillProductFromStructured()">填入商品表单</el-button>
              <el-button v-if="studioConfig.showShareActions" plain @click="prefillShareFromStructured()">填入海报设置</el-button>
              <el-button v-if="route.query.promotion_id" type="warning" plain @click="backToPromotionEdit">
                按建议调整活动
              </el-button>
              <el-button v-if="studioConfig.showSocialPack" @click="copySocialPack(aiResult.content)">复制朋友圈素材</el-button>
              <el-button v-if="studioConfig.showCouponLink" @click="router.push('/merchant/coupons')">查看券包核销</el-button>
            </div>
          </div>
          <div v-else class="empty-result">
            <strong>等待生成 AI 方案</strong>
            <p>选择场景后，系统会结合经营数据生成一份可执行的营销动作。</p>
          </div>
        </div>
      </div>
    </section>

    <section class="bottom-grid">
      <div class="panel-card video-card">
        <div class="section-head">
          <div>
            <div class="eyebrow dark">SHORT VIDEO SCRIPT</div>
            <h2>AI 短视频营销脚本</h2>
            <p>生成手机就能拍的短视频脚本，适合抖音、视频号、小红书和快手。</p>
          </div>
        </div>
        <el-form label-position="top">
          <div class="form-row">
            <el-form-item label="发布平台">
              <el-select v-model="videoForm.platform">
                <el-option label="抖音" value="douyin" />
                <el-option label="视频号" value="wechat_channels" />
                <el-option label="小红书" value="xiaohongshu" />
                <el-option label="快手" value="kuaishou" />
              </el-select>
            </el-form-item>
            <el-form-item label="视频时长">
              <el-select v-model="videoForm.duration">
                <el-option label="15 秒" value="15s" />
                <el-option label="30 秒" value="30s" />
                <el-option label="60 秒" value="60s" />
              </el-select>
            </el-form-item>
          </div>
          <el-form-item label="视频主题">
            <el-input v-model="videoForm.topic" placeholder="例如：适合下班聚餐的烧烤双人套餐" />
          </el-form-item>
          <el-form-item label="核心卖点">
            <el-input v-model="videoForm.selling_points" type="textarea" :rows="3" />
          </el-form-item>
          <el-button type="primary" :disabled="!canUseAI" :loading="videoLoading" @click="generateVideoScript">生成短视频脚本</el-button>
        </el-form>
      </div>

      <div class="panel-card">
        <div class="section-head compact">
          <div>
            <div class="eyebrow dark">VIDEO RESULT</div>
            <h2>短视频脚本结果</h2>
          </div>
        </div>
        <div v-if="videoResult.content" class="copy-box">
          <div class="output-meta dark">
            <span>模型：{{ videoResult.model || aiQuota.model || '-' }}</span>
            <span>Token：{{ videoResult.total_tokens || 0 }}</span>
            <span>预估成本：{{ formatCost(videoResult.estimated_cost_cents) }}</span>
          </div>
          <pre>{{ videoResult.content }}</pre>
          <div class="inline-actions">
            <el-button type="primary" @click="copyText(videoResult.content)">复制脚本</el-button>
            <el-button @click="copyVideoPack(videoResult.content)">复制短视频素材包</el-button>
            <el-button @click="router.push('/merchant/share')">去裂变海报</el-button>
          </div>
        </div>
        <div v-else class="empty-result">
          <strong>还没有生成脚本</strong>
          <p>填写主题和卖点后，AI 会输出开头钩子、分镜、口播和发布标题。</p>
        </div>
      </div>

      <div class="panel-card">
        <div class="section-head compact">
          <div>
            <div class="eyebrow dark">PRODUCT SIGNALS</div>
            <h2>商品经营信号</h2>
            <p>根据热销商品判断下一步主推方向。</p>
          </div>
        </div>
        <div class="signal-list">
          <div v-for="item in hotProducts.slice(0, 5)" :key="item.name" class="signal-row">
            <span>{{ item.name }}</span>
            <strong>已售 {{ item.quantity || item.sales || 0 }}</strong>
          </div>
          <el-empty v-if="!hotProducts.length" description="暂无商品销售数据。" />
        </div>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  fetchMerchantAIInsights,
  fetchMerchantAIQuota,
  fetchMerchantShareStats,
  createMerchantPromotion,
  generateMerchantAIMarketingCopy,
  generateMerchantPromotionDraft
} from '../../api/modules'

const router = useRouter()
const route = useRoute()
const insights = ref({})
const shareStats = ref({})
const aiQuota = ref({})
const generating = ref(false)
const copyLoading = ref(false)
const videoLoading = ref(false)
const aiResult = ref({})
const videoResult = ref({})
const activeModule = ref('review')
const aiError = reactive({
  visible: false,
  title: 'AI 生成未完成',
  message: '',
  suggestion: ''
})

const copyForm = reactive({
  scenario: 'referral_poster',
  customer_tag: 'new_customer',
  product_name: '',
  goal: '提升本周到店转化和复购率。'
})

const videoForm = reactive({
  platform: 'douyin',
  duration: '30s',
  topic: '',
  selling_points: '实惠、好吃、方便，适合朋友聚餐或下班消费。'
})

const scenarioPresets = {
  campaign: {
    scenario: 'referral_poster',
    customer_tag: 'repeat_customer',
    product_name: '',
    goal: '根据今日订单和顾客数据，生成一套能提升复购和到店转化的优惠活动方案。'
  },
  product_optimize: {
    scenario: 'product_optimize',
    customer_tag: 'nearby_customer',
    product_name: '',
    goal: '针对缺图、缺描述、低动销或排序靠后的商品，生成商品标题、描述、主推理由和扫码页优化建议。'
  },
  refund_review: {
    scenario: 'dormant_recall',
    customer_tag: 'repeat_customer',
    product_name: '',
    goal: '复盘近期退款和售后原因，输出降低退款率、优化履约沟通、补偿券设置和顾客挽回建议。'
  },
  finance_review: {
    scenario: 'vip_campaign',
    customer_tag: 'high_value_customer',
    product_name: '',
    goal: '根据经营流水、退款和净收入情况，给出本周提客单价、控退款和提升利润的行动建议。'
  },
  daily_report: {
    scenario: 'social_post',
    customer_tag: 'repeat_customer',
    product_name: '',
    goal: '根据今日经营日报，输出今日复盘、明日行动建议、适合投放的优惠活动和朋友圈/社群话术。'
  },
  promotion_review: {
    scenario: 'vip_campaign',
    customer_tag: 'repeat_customer',
    product_name: '',
    goal: '复盘当前优惠活动的订单、实收、让利和转化表现，判断是否应该继续投放、提高门槛、降低让利或改成复购券。'
  },
  short_video: {
    scenario: 'social_post',
    customer_tag: 'nearby_customer',
    product_name: '',
    goal: '生成适合门店本周引流的短视频选题、口播脚本和发布标题。',
    video: true
  }
}

const hotProducts = computed(() => insights.value.hot_products || insights.value.top_products || [])
const canUseAI = computed(() => Number(aiQuota.value.remaining ?? 0) > 0)
const quotaPercent = computed(() => {
  const limit = Number(aiQuota.value.limit || 0)
  const used = Number(aiQuota.value.used || 0)
  if (!limit) return 0
  return Math.min(100, Math.round((used / limit) * 100))
})
const quotaStatus = computed(() => {
  if (!canUseAI.value) return 'exhausted'
  if (aiQuota.value.near_limit || quotaPercent.value >= 80) return 'warning'
  return 'normal'
})
const quotaHint = computed(() => {
  if (!canUseAI.value) return '今日 AI 额度已用完，可明天继续使用，或升级/续费获得更高额度。'
  if (quotaStatus.value === 'warning') return `已使用 ${aiQuota.value.used ?? 0} 次，接近今日额度，建议优先生成最需要落地的内容。`
  return `已使用 ${aiQuota.value.used ?? 0} 次，当前套餐今日额度正常。`
})
const structuredOutput = computed(() => {
  if (Array.isArray(aiResult.value.structured) && aiResult.value.structured.length) {
    return aiResult.value.structured
  }
  return buildStructuredOutput(aiResult.value.content || '', copyForm.scenario)
})
const studioMode = computed(() => {
  if (activeModule.value === 'product' || copyForm.scenario === 'product_optimize') return 'product'
  if (activeModule.value === 'share' || copyForm.scenario === 'referral_poster') return 'share'
  if (activeModule.value === 'review' || ['daily_report', 'finance_review', 'refund_review', 'promotion_review'].includes(String(route.query.scenario || ''))) return 'review'
  return 'campaign'
})
const studioConfig = computed(() => {
  const base = {
    eyebrow: 'AI CAMPAIGN STUDIO',
    title: 'AI 营销方案生成器',
    description: '生成裂变海报、召回券、朋友圈和门店推广话术。',
    scenarioLabel: '使用场景',
    audienceLabel: '目标人群',
    subjectLabel: '主推商品 / 服务',
    subjectPlaceholder: '例如：招牌烤串、双人套餐、到店服务',
    goalLabel: '营销目标',
    primaryAction: '生成 AI 方案',
    copyAction: '复制文案',
    showDraftButton: true,
    showPromotionActions: true,
    showProductActions: false,
    showShareActions: false,
    showSocialPack: true,
    showCouponLink: true,
    resultCompact: false,
    scenarioOptions: [
      { label: '裂变海报', value: 'referral_poster' },
      { label: '新客引流', value: 'new_customer' },
      { label: '沉睡顾客召回', value: 'dormant_recall' },
      { label: '高价值顾客专属活动', value: 'vip_campaign' },
      { label: '朋友圈短文案', value: 'social_post' }
    ]
  }
  if (studioMode.value === 'product') {
    return {
      ...base,
      eyebrow: 'AI PRODUCT OPTIMIZER',
      title: 'AI 商品优化助手',
      description: '根据缺图、缺描述、售罄、低库存和排序情况，生成商品标题、描述、主推和菜单调整建议。',
      scenarioLabel: '优化场景',
      audienceLabel: '顾客类型',
      subjectLabel: '优先优化商品',
      subjectPlaceholder: '例如：当前缺图商品、低库存商品、招牌套餐',
      goalLabel: '商品问题与优化目标',
      primaryAction: '生成商品优化建议',
      copyAction: '复制优化建议',
      showDraftButton: false,
      showPromotionActions: false,
      showProductActions: true,
      showSocialPack: false,
      showCouponLink: false,
      resultCompact: true,
      scenarioOptions: [
        { label: '菜单商品优化', value: 'product_optimize' },
        { label: '商品描述优化', value: 'product_description' },
        { label: '主推商品排序', value: 'product_sorting' },
        { label: '套餐组合建议', value: 'product_bundle' }
      ]
    }
  }
  if (studioMode.value === 'share') {
    return {
      ...base,
      eyebrow: 'AI REFERRAL STUDIO',
      title: 'AI 裂变海报方案',
      description: '围绕分享海报、好友券和复购奖励生成可编辑文案，方便回填到裂变设置。',
      scenarioLabel: '裂变场景',
      subjectLabel: '主推权益 / 服务',
      goalLabel: '裂变目标',
      primaryAction: '生成裂变方案',
      showDraftButton: false,
      showPromotionActions: false,
      showShareActions: true,
      showCouponLink: true,
      scenarioOptions: [
        { label: '裂变海报', value: 'referral_poster' },
        { label: '新客领券', value: 'new_customer' },
        { label: '老客分享复购', value: 'dormant_recall' }
      ]
    }
  }
  if (studioMode.value === 'review') {
    return {
      ...base,
      eyebrow: 'AI BUSINESS REVIEW',
      title: 'AI 经营复盘建议',
      description: '根据订单、实收、退款、客单价和热销商品，生成当天复盘和下一步动作。',
      scenarioLabel: '复盘类型',
      subjectLabel: '关注对象',
      goalLabel: '复盘范围与问题',
      primaryAction: '生成复盘建议',
      copyAction: '复制复盘',
      showDraftButton: false,
      showPromotionActions: true,
      showSocialPack: false,
      resultCompact: true,
      scenarioOptions: [
        { label: '今日经营日报', value: 'daily_report' },
        { label: '财务收入复盘', value: 'finance_review' },
        { label: '退款售后复盘', value: 'refund_review' },
        { label: '优惠活动复盘', value: 'promotion_review' }
      ]
    }
  }
  return base
})
const aiModules = computed(() => [
  { key: 'review', label: '经营复盘', title: '今日复盘建议', desc: '总结订单、退款、客单价和明日动作', scenario: 'daily_report' },
  { key: 'campaign', label: '营销文案', title: '活动方案生成', desc: '输出朋友圈、社群、到店转化文案', scenario: 'campaign' },
  { key: 'share', label: '裂变活动', title: '海报和券包闭环', desc: '保留裂变活动为独立子模块', path: '/merchant/share' },
  { key: 'video', label: '短视频', title: '手机拍摄脚本', desc: '生成钩子、分镜、口播和标题', scenario: 'short_video' },
  { key: 'product', label: '商品优化', title: '菜单商品优化', desc: '补标题、描述、卖点和主推方向', scenario: 'product_optimize' }
])
const moduleByScenario = computed(() => Object.fromEntries(
  aiModules.value
    .filter((item) => item.scenario)
    .map((item) => [item.scenario, item])
))
const scenarioContext = computed(() => {
  const scenario = String(route.query.scenario || '')
  const contextMap = {
    daily_report: {
      label: '来自经营看板',
      title: '今日经营日报 AI 建议',
      desc: '已带入今日订单、实收、退款、客单价和当前经营结论，可直接生成复盘和明日动作。',
      button: '生成日报建议',
      loading: copyLoading.value
    },
    product_optimize: {
      label: '来自商品管理',
      title: '菜单商品优化建议',
      desc: '已带入商品缺图、缺描述、售罄、低库存和菜单健康度，可直接生成菜单优化方案。',
      button: '生成菜单优化',
      loading: copyLoading.value
    },
    refund_review: {
      label: '来自售后/财务',
      title: '退款与履约复盘建议',
      desc: '用于复盘退款原因、履约沟通和补偿券设置，帮助降低售后风险。',
      button: '生成退款复盘',
      loading: copyLoading.value
    },
    finance_review: {
      label: '来自财务对账',
      title: '经营收入优化建议',
      desc: '用于根据实收、退款和客单价生成提客单、控退款和提升利润的动作。',
      button: '生成财务建议',
      loading: copyLoading.value
    },
    short_video: {
      label: '来自经营建议',
      title: '短视频脚本建议',
      desc: '已切换到短视频场景，可生成手机就能拍的选题、分镜、口播和标题。',
      button: '生成脚本',
      loading: videoLoading.value
    },
    campaign: {
      label: '来自经营建议',
      title: '营销活动方案建议',
      desc: '已切换到营销方案场景，可结合订单、顾客和热销商品生成活动草稿。',
      button: '生成营销方案',
      loading: copyLoading.value
    },
    dormant_recall: {
      label: '来自顾客经营',
      title: '沉睡顾客召回建议',
      desc: '用于生成召回券、社群话术和复购提醒，适合近期下单减少时使用。',
      button: '生成召回方案',
      loading: copyLoading.value
    }
  }
  const context = contextMap[scenario]
  if (!context) return { visible: false }
  return { visible: true, ...context }
})
const cancelRate = computed(() => {
  const stats = insights.value.order_stats || {}
  const total = Number(stats.order_count || 0)
  const cancelled = Number(stats.cancelled_count || 0)
  if (!total) return '0%'
  return `${Math.round((cancelled / total) * 100)}%`
})

const todayActions = computed(() => [
  {
    icon: '↗',
    label: '引流',
    title: '生成裂变海报',
    desc: '结合门店码和奖励券，引导顾客分享给好友。',
    button: '生成文案',
    tone: 'blue',
    loading: copyLoading.value,
    action: () => generateMarketingCopy('referral_poster')
  },
  {
    icon: '⟳',
    label: '复购',
    title: '召回沉睡顾客',
    desc: '把近期未下单顾客转成优惠券触达对象。',
    button: '生成召回',
    tone: 'orange',
    loading: copyLoading.value,
    action: () => generateMarketingCopy('dormant_recall')
  },
  {
    icon: '▶',
    label: '内容',
    title: '拍一条短视频',
    desc: '生成普通手机就能拍的短视频脚本。',
    button: '生成脚本',
    tone: 'green',
    loading: videoLoading.value,
    action: generateVideoScript
  }
])

const formatYuan = (value) => (Number(value || 0) / 100).toFixed(2)
const formatCost = (value) => `¥${(Number(value || 0) / 100).toFixed(2)}`
const scenarioTitle = (scenario) => ({
  referral_poster: 'AI裂变引流活动',
  new_customer: 'AI新客到店活动',
  dormant_recall: 'AI沉睡顾客召回券',
  vip_campaign: 'AI高价值顾客专属活动',
  social_post: 'AI朋友圈转化活动',
  product_optimize: 'AI商品优化草稿',
  product_description: 'AI商品描述草稿',
  product_sorting: 'AI商品排序建议',
  product_bundle: 'AI套餐组合建议',
  daily_report: 'AI经营复盘建议',
  finance_review: 'AI财务复盘建议',
  refund_review: 'AI退款售后复盘',
  promotion_review: 'AI活动复盘建议'
}[scenario] || 'AI经营活动草稿')

const dateString = (date) => {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

const addDays = (days) => {
  const date = new Date()
  date.setDate(date.getDate() + days)
  return dateString(date)
}

const textValue = (value, max = 500) => String(value || '').trim().slice(0, max)

const pickStructuredItem = (action) => {
  if (action) {
    const matched = structuredOutput.value.find((item) => item.action === action)
    if (matched) return matched
  }
  return structuredOutput.value[0] || null
}

const saveAIDraft = (key, payload) => {
  sessionStorage.setItem(key, JSON.stringify({
    ...payload,
    source: 'merchant_ai_center',
    created_at: new Date().toISOString()
  }))
}

const promotionPayloadFromAI = (item = pickStructuredItem('promotion')) => {
  const isVip = copyForm.scenario === 'vip_campaign'
  return {
    title: textValue(item?.title || scenarioTitle(copyForm.scenario), 120),
    description: textValue(item?.copy || item?.text || aiResult.value.content || copyForm.goal, 500),
    type: isVip ? 'discount' : 'amount',
    threshold_yuan: isVip ? 88 : 50,
    discount_yuan: isVip ? 0 : 8,
    discount_rate: isVip ? 88 : 85,
    valid_from: dateString(new Date()),
    valid_to: addDays(14),
    ai_note: textValue(item?.text || item?.copy || aiResult.value.content || '', 800)
  }
}

const prefillPromotionFromStructured = (item) => {
  saveAIDraft('merchant_ai_promotion_draft', promotionPayloadFromAI(item))
  ElMessage.success('AI 活动建议已填入活动表单，可先编辑再保存')
  router.push('/merchant/promotions?ai_prefill=1')
}

const prefillShareFromStructured = (item = pickStructuredItem('share')) => {
  saveAIDraft('merchant_ai_share_draft', {
    poster_title: textValue(item?.title || '好友扫码领券', 80),
    poster_copy: textValue(item?.copy || item?.text || aiResult.value.content || copyForm.goal, 500),
    ai_note: textValue(item?.text || item?.copy || aiResult.value.content || '', 800)
  })
  ElMessage.success('AI 海报文案已填入裂变设置，可先编辑再保存')
  router.push('/merchant/share?ai_prefill=1')
}

const productSection = (keys = [], labels = []) => structuredOutput.value.find((section) => {
  const key = String(section?.key || '').toLowerCase()
  const label = String(section?.label || section?.title || '')
  return keys.some((item) => key.includes(String(item).toLowerCase())) ||
    labels.some((item) => label.includes(String(item)))
})

const productSectionText = (keys = [], labels = [], max = 500) => {
  const section = productSection(keys, labels)
  return textValue(section?.copy || section?.text || section?.title || '', max)
}

const inferProductSort = (hint = '') => {
  const raw = String(hint || '')
  const matched = raw.match(/(?:排序|sort|调到|设为|设置为)[^\d]*(\d{1,3})/i)
  if (matched) return Math.min(100, Math.max(1, Number(matched[1])))
  if (/主推|优先|首屏|靠前|前面|置顶/.test(raw)) return 20
  if (/下架|售罄|缺图|低库存|靠后/.test(raw)) return 100
  return 20
}

const prefillProductFromStructured = (item = pickStructuredItem('product')) => {
  const titleText = productSectionText(['product_title', 'title'], ['商品标题', '标题'], 120)
  const descriptionText = productSectionText(['product_description', 'description'], ['商品描述', '描述'], 500)
  const mainReason = productSectionText(['main_reason', 'reason', 'selling_point'], ['主推理由', '卖点', '推荐理由'], 500)
  const sortSuggestion = productSectionText(['sort_suggestion', 'sort'], ['排序建议', '排序'], 500)
  const bundleSuggestion = productSectionText(['bundle_suggestion', 'bundle'], ['组合建议', '套餐建议', '搭配建议'], 500)
  const fallbackText = textValue(item?.copy || item?.text || aiResult.value.content || copyForm.goal, 500)
  saveAIDraft('merchant_ai_product_draft', {
    name: textValue(route.query.product || titleText || item?.title || copyForm.product_name || '', 80),
    description: textValue(descriptionText || mainReason || fallbackText, 500),
    category: textValue(route.query.category || '推荐商品', 40),
    sort: inferProductSort(sortSuggestion || mainReason),
    main_reason: mainReason,
    sort_hint: sortSuggestion,
    bundle_hint: bundleSuggestion,
    ai_note: textValue([
      titleText ? `商品标题：${titleText}` : '',
      descriptionText ? `商品描述：${descriptionText}` : '',
      mainReason ? `主推理由：${mainReason}` : '',
      sortSuggestion ? `排序建议：${sortSuggestion}` : '',
      bundleSuggestion ? `组合建议：${bundleSuggestion}` : ''
    ].filter(Boolean).join('\n') || item?.text || item?.copy || aiResult.value.content || '', 1000)
  })
  const targetStoreId = Number(route.query.store_id || route.query.storeId || 0)
  if (targetStoreId) {
    router.push(`/merchant/stores/${targetStoreId}/products?ai_prefill=1`)
    return
  }
  ElMessage.info('AI 商品草稿已保存，请选择门店后进入商品管理继续编辑')
  router.push('/merchant/stores')
}

async function load() {
  const [insightRes, shareRes, quotaRes] = await Promise.all([
    fetchMerchantAIInsights(),
    fetchMerchantShareStats(),
    fetchMerchantAIQuota()
  ])
  insights.value = insightRes.data || {}
  shareStats.value = shareRes.data || {}
  aiQuota.value = quotaRes.data || {}
  if (canUseAI.value) aiError.visible = false
}

async function generateDraft() {
  generating.value = true
  try {
    await generateMerchantPromotionDraft({})
    ElMessage.success('优惠活动草稿已生成')
    router.push('/merchant/promotions')
  } finally {
    generating.value = false
  }
}

async function createPromotionFromAI() {
  generating.value = true
  try {
    const primary = structuredOutput.value.find((item) => item.action === 'promotion') || structuredOutput.value[0]
    const content = primary?.copy || primary?.text || aiResult.value.content || copyForm.goal
    const isVip = copyForm.scenario === 'vip_campaign'
    await createMerchantPromotion({
      store_id: null,
      title: primary?.title || scenarioTitle(copyForm.scenario),
      description: String(content || '').slice(0, 500),
      type: isVip ? 'discount' : 'amount',
      threshold: isVip ? 8800 : 5000,
      discount: isVip ? 0 : 800,
      discount_rate: isVip ? 88 : 0,
      status: 'draft',
      valid_from: dateString(new Date()),
      valid_to: addDays(14)
    })
    ElMessage.success('已生成优惠活动草稿，可继续编辑后发布')
    router.push('/merchant/promotions?ai_draft=1')
  } catch (error) {
    ElMessage.error(error?.response?.data?.message || '生成活动草稿失败')
  } finally {
    generating.value = false
  }
}

async function createPromotionFromStructured(item) {
  generating.value = true
  try {
    const isVip = copyForm.scenario === 'vip_campaign'
    await createMerchantPromotion({
      store_id: null,
      title: item.title || scenarioTitle(copyForm.scenario),
      description: String(item.copy || item.text || aiResult.value.content || '').slice(0, 500),
      type: isVip ? 'discount' : 'amount',
      threshold: isVip ? 8800 : 5000,
      discount: isVip ? 0 : 800,
      discount_rate: isVip ? 88 : 0,
      status: 'draft',
      valid_from: dateString(new Date()),
      valid_to: addDays(14)
    })
    ElMessage.success('已按该建议生成活动草稿')
    router.push('/merchant/promotions?ai_draft=1')
  } catch (error) {
    ElMessage.error(error?.response?.data?.message || '生成活动草稿失败')
  } finally {
    generating.value = false
  }
}

async function generateMarketingCopy(scenario) {
  if (!canUseAI.value) {
    showQuotaError()
    return
  }
  if (scenario) copyForm.scenario = scenario
  copyLoading.value = true
  try {
    const res = await generateMerchantAIMarketingCopy(copyForm)
    aiResult.value = res.data?.result || res.data || {}
    if (res.data?.quota) aiQuota.value = res.data.quota
    aiError.visible = false
    ElMessage.success('AI 营销方案已生成')
  } catch (error) {
    await handleAIError(error)
  } finally {
    copyLoading.value = false
  }
}

async function generateVideoScript() {
  if (!canUseAI.value) {
    showQuotaError()
    return
  }
  videoLoading.value = true
  try {
    const res = await generateMerchantAIMarketingCopy({
      scenario: 'short_video_script',
      customer_tag: copyForm.customer_tag,
      product_name: videoForm.topic || copyForm.product_name,
      goal: `发布平台：${videoForm.platform}；视频时长：${videoForm.duration}；核心卖点：${videoForm.selling_points}`
    })
    videoResult.value = res.data?.result || res.data || {}
    if (res.data?.quota) aiQuota.value = res.data.quota
    aiError.visible = false
    ElMessage.success('短视频脚本已生成')
  } catch (error) {
    await handleAIError(error)
  } finally {
    videoLoading.value = false
  }
}

function showQuotaError() {
  aiError.visible = true
  aiError.title = 'AI 额度已用完'
  aiError.message = '今日 AI 生成次数已经用完。'
  aiError.suggestion = '可以先复制已有结果继续执行，或到订阅页升级/续费，明天额度会自动恢复。'
  ElMessage.warning(aiError.message)
}

async function handleAIError(error) {
  const message = error?.response?.data?.message || 'AI 生成失败，请稍后再试'
  aiError.visible = true
  aiError.title = message.includes('额度') || message.includes('次数') ? 'AI 额度不足' : 'AI 暂时不可用'
  aiError.message = message
  aiError.suggestion = message.includes('额度') || message.includes('次数')
    ? '优先使用已有生成结果；如果需要继续生成，可升级套餐或等待次日额度恢复。'
    : '系统已保留页面内容，可稍后重试；如果连续失败，请联系平台检查模型 Key、额度和网络。'
  try {
    const quotaRes = await fetchMerchantAIQuota()
    aiQuota.value = quotaRes.data || aiQuota.value
  } catch (_) {}
  ElMessage.error(message)
}

async function runCurrentScenario() {
  const scenario = String(route.query.scenario || copyForm.scenario)
  if (scenario === 'short_video' || scenarioPresets[scenario]?.video) {
    await generateVideoScript()
    return
  }
  await generateMarketingCopy()
}

async function copyText(text) {
  await navigator.clipboard.writeText(text)
  ElMessage.success('已复制')
}

async function copySocialPack(text) {
  const pack = [
    '【朋友圈/社群素材包】',
    `主推：${copyForm.product_name || hotProducts.value[0]?.name || '门店招牌商品'}`,
    `人群：${copyForm.customer_tag}`,
    '',
    String(text || '').trim(),
    '',
    '发布建议：配门店环境图、爆款商品图或裂变海报，评论区引导“扫码领券/到店使用”。'
  ].join('\n')
  await copyText(pack)
}

async function copyVideoPack(text) {
  const pack = [
    '【短视频素材包】',
    `平台：${videoForm.platform}`,
    `时长：${videoForm.duration}`,
    `主题：${videoForm.topic || copyForm.product_name || '门店本周主推'}`,
    '',
    String(text || '').trim(),
    '',
    '拍摄建议：前三秒展示成品或优惠，镜头包含门头、制作过程、顾客取餐/用餐和扫码领券。'
  ].join('\n')
  await copyText(pack)
}

function buildStructuredOutput(content, scenario) {
  const text = String(content || '').trim()
  if (!text) return []
  const lines = text.split(/\n+/).map((line) => line.trim()).filter(Boolean)
  const sections = []
  let current = null
  for (const line of lines) {
    const match = line.match(/^(?:\d+[.、]\s*)?【?([^：:】]{2,18})】?[：:]\s*(.*)$/)
    if (match) {
      current = {
        key: `section_${sections.length}`,
        rawTitle: match[1],
        title: cleanTitle(match[2] || match[1]),
        body: match[2] ? [] : []
      }
      sections.push(current)
      continue
    }
    if (current) current.body.push(line)
  }
  const parsed = sections
    .map((section, index) => normalizeStructuredSection(section, index, scenario))
    .filter((item) => item.text)
    .slice(0, 6)
  if (parsed.length >= 3) return parsed
  return fallbackStructuredOutput(text, scenario)
}

function normalizeStructuredSection(section, index, scenario) {
  const titleText = `${section.rawTitle || ''} ${section.title || ''}`
  const body = section.body.join('\n').trim()
  const text = body || section.title || ''
  const label = structuredLabel(titleText, index)
  return {
    key: section.key,
    label,
    title: cleanTitle(section.title || label),
    text,
    copy: `${cleanTitle(section.title || label)}\n${text}`,
    action: structuredAction(titleText, scenario)
  }
}

function fallbackStructuredOutput(text, scenario) {
  const chunks = text.split(/\n{2,}/).map((item) => item.trim()).filter(Boolean)
  const first = chunks[0] || text.slice(0, 220)
  const second = chunks[1] || text.slice(220, 520)
  const third = chunks[2] || text.slice(520, 820)
  return [
    {
      key: 'fallback_promotion',
      label: '活动建议',
      title: scenarioTitle(scenario),
      text: first,
      copy: first,
      action: 'promotion'
    },
    {
      key: 'fallback_copy',
      label: '传播文案',
      title: '可复制文案',
      text: second || first,
      copy: second || first,
      action: 'share'
    },
    {
      key: 'fallback_steps',
      label: '执行步骤',
      title: '下一步动作',
      text: third || '复制文案后，可进入优惠活动或裂变海报继续落地。',
      copy: third || text,
      action: scenario === 'social_post' ? 'share' : 'promotion'
    }
  ].filter((item) => item.text)
}

function structuredLabel(title, index) {
  if (/标题|活动/.test(title)) return '活动标题'
  if (/海报|朋友圈|社群|文案|话术/.test(title)) return '传播文案'
  if (/优惠|奖励|券|机制/.test(title)) return '优惠建议'
  if (/步骤|执行|动作|下一步/.test(title)) return '执行步骤'
  if (/风险|成本|提醒/.test(title)) return '风险提醒'
  return ['活动建议', '传播文案', '优惠建议', '执行步骤', '风险提醒'][index] || '建议'
}

function structuredAction(title, scenario) {
  if (/商品|菜单|菜品|描述|标题/.test(title) || scenario === 'product_optimize') return 'product'
  if (/海报|朋友圈|社群|裂变|分享/.test(title)) return 'share'
  if (/优惠|活动|券|奖励|标题|方案/.test(title)) return 'promotion'
  return scenario === 'social_post' ? 'share' : 'promotion'
}

function cleanTitle(value) {
  return String(value || '').replace(/^[-*\s\d.、]+/, '').replace(/[。；;：:]+$/, '').trim() || 'AI 建议'
}

function backToPromotionEdit() {
  if (aiResult.value?.content) {
    sessionStorage.setItem('ai_promotion_review_note', aiResult.value.content)
  }
  router.push({
    path: '/merchant/promotions',
    query: {
      edit_id: route.query.promotion_id,
      ai_review: '1'
    }
  })
}

function applyScenarioPreset(scenario) {
  const preset = scenarioPresets[scenario]
  if (!preset) {
    if (['referral_poster', 'new_customer', 'dormant_recall', 'vip_campaign', 'social_post'].includes(scenario)) {
      activeModule.value = 'campaign'
      copyForm.scenario = scenario
      copyForm.customer_tag = scenario === 'dormant_recall' ? 'dormant_customer' : copyForm.customer_tag
      copyForm.goal = scenario === 'dormant_recall'
        ? '根据近期顾客沉淀和下单情况，生成一套沉睡顾客召回券、社群话术和复购提醒。'
        : '根据门店经营数据，生成一套可以马上使用的营销活动方案。'
      ElMessage.info('已根据来源页面预填 AI 经营场景')
    }
    return
  }
  const module = moduleByScenario.value[scenario]
  if (module) activeModule.value = module.key
  copyForm.scenario = preset.scenario
  copyForm.customer_tag = preset.customer_tag
  copyForm.product_name = preset.product_name
  copyForm.goal = preset.goal
  if (scenario === 'promotion_review') {
    copyForm.product_name = String(route.query.promotion || '')
    copyForm.goal = [
      `请复盘优惠活动：${route.query.promotion || '未命名活动'}。`,
      `活动带单 ${route.query.orders || 0} 笔，其中有效支付 ${route.query.paid || 0} 笔。`,
      `优惠让利 ¥${route.query.discount || '0.00'}，活动实收 ¥${route.query.revenue || '0.00'}。`,
      '请判断活动是否值得继续，并给出下一步动作：继续投放、调整门槛、减少让利、改成复购券或停用。'
    ].join('\n')
  }
  if (scenario === 'daily_report') {
    copyForm.product_name = String(route.query.product || '')
    copyForm.goal = [
      `请基于 ${route.query.date || '今日'} 的商家经营日报生成一份可执行复盘。`,
      `今日订单：${route.query.orders || 0} 单，净实收：${route.query.net || '¥0.00'}，退款：${route.query.refund || '¥0.00'}，客单价：${route.query.avg || '¥0.00'}。`,
      `热销商品：${route.query.product || '暂无热销商品'}，待接单：${route.query.pending || 0} 单。`,
      `系统当前结论：${route.query.brief || '暂无'}。`,
      '请输出：1）今日经营结论；2）明日三条优先动作；3）适合发给顾客的朋友圈/社群文案；4）是否需要做优惠活动或裂变海报。'
    ].join('\n')
  }
  if (scenario === 'product_optimize') {
    copyForm.product_name = String(route.query.product || hotProducts.value[0]?.name || '')
    copyForm.goal = [
      '请根据当前门店商品管理情况，生成一份可执行的菜单优化建议。',
      `商品总数：${route.query.total || 0} 个，上架：${route.query.active || 0} 个，分类：${route.query.categories || 0} 个。`,
      `缺图：${route.query.missing_image || 0} 个，缺描述：${route.query.missing_desc || 0} 个，售罄：${route.query.sold_out || 0} 个，低库存：${route.query.low_stock || 0} 个。`,
      `菜单健康度：${route.query.health || '0%'}，当前结论：${route.query.brief || '暂无'}。`,
      '请输出：1）最先处理的 3 个菜单问题；2）商品标题/描述优化方向；3）哪些商品适合排前；4）适合做套餐或活动的建议。'
    ].join('\n')
  }
  if (preset.video) {
    videoForm.topic = preset.goal
    videoForm.selling_points = '突出门店特色、爆款商品、优惠券和到店体验。'
  }
  ElMessage.info('已根据来源页面预填 AI 经营场景')
}

function activateAIModule(item) {
  activeModule.value = item.key
  if (item.path) {
    router.push(item.path)
    return
  }
  applyScenarioPreset(item.scenario)
}

onMounted(async () => {
  const scenario = String(route.query.scenario || '')
  await load()
  applyScenarioPreset(scenario)
  if (!copyForm.product_name && hotProducts.value[0]?.name) {
    copyForm.product_name = hotProducts.value[0].name
  }
  if (route.query.auto === '1') {
    await runCurrentScenario()
  }
})

watch(() => route.query.scenario, (scenario) => {
  applyScenarioPreset(String(scenario || ''))
})
</script>

<style scoped>
.ai-page {
  width: min(100%, 1680px);
  margin: 0 auto;
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.ai-cockpit {
  position: relative;
  display: grid;
  grid-template-columns: minmax(0, 1fr) 330px;
  gap: 24px;
  min-height: 260px;
  padding: 30px;
  overflow: hidden;
  border: 1px solid rgba(147, 197, 253, 0.34);
  border-radius: 32px;
  background:
    radial-gradient(circle at 72% 16%, rgba(56, 189, 248, 0.42), transparent 28%),
    radial-gradient(circle at 8% 88%, rgba(37, 99, 235, 0.35), transparent 32%),
    linear-gradient(135deg, #061a33 0%, #0f4da8 56%, #06b6d4 100%);
  box-shadow: 0 28px 70px rgba(15, 39, 71, 0.18);
  color: #fff;
}

.ai-cockpit::after {
  content: "";
  position: absolute;
  inset: 18px;
  border-radius: 26px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  pointer-events: none;
}

.cockpit-copy,
.quota-orb {
  position: relative;
  z-index: 1;
}

.cockpit-copy {
  display: flex;
  flex-direction: column;
  justify-content: center;
  max-width: 840px;
}

.eyebrow {
  color: #a7f3d0;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.16em;
  text-transform: uppercase;
}

.eyebrow.dark {
  color: #2563eb;
}

.cockpit-copy h1 {
  margin: 12px 0;
  font-size: clamp(32px, 4vw, 54px);
  line-height: 1.05;
  letter-spacing: -0.06em;
}

.cockpit-copy p {
  max-width: 760px;
  margin: 0;
  color: rgba(255, 255, 255, 0.78);
  font-size: 16px;
  line-height: 1.9;
}

.cockpit-actions,
.form-actions,
.inline-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.cockpit-actions {
  margin-top: 24px;
}

.ghost-button {
  color: #fff;
  border-color: rgba(255, 255, 255, 0.38);
  background: rgba(255, 255, 255, 0.12);
}

.quota-orb {
  align-self: stretch;
  display: flex;
  flex-direction: column;
  justify-content: center;
  border-radius: 28px;
  padding: 24px;
  background: rgba(255, 255, 255, 0.92);
  color: #0f172a;
  box-shadow: inset 0 0 0 1px rgba(37, 99, 235, 0.08);
}

.quota-orb span,
.metric-card span,
.poster-card span,
.action-card span {
  color: #64748b;
  font-size: 13px;
}

.quota-orb strong {
  margin-top: 8px;
  font-size: 64px;
  line-height: 0.95;
  letter-spacing: -0.07em;
}

.quota-orb small {
  margin-top: 6px;
  color: #2563eb;
  font-weight: 800;
}

.quota-orb p {
  margin: 12px 0 0;
  color: #64748b;
}

.quota-orb.warning {
  border: 1px solid #f59e0b;
  background: #fffbeb;
}

.quota-orb.exhausted {
  border: 1px solid #fecaca;
  background: #fff1f2;
}

.quota-progress {
  height: 8px;
  margin-top: 14px;
  overflow: hidden;
  border-radius: 999px;
  background: #e2e8f0;
}

.quota-progress i {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, #2563eb, #06b6d4);
}

.quota-orb.warning .quota-progress i {
  background: linear-gradient(90deg, #f59e0b, #f97316);
}

.quota-orb.exhausted .quota-progress i {
  background: linear-gradient(90deg, #ef4444, #f97316);
}

.ai-error-card {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: center;
  padding: 16px 18px;
  border: 1px solid #fecaca;
  border-radius: 8px;
  background: #fff7ed;
}

.ai-error-card span,
.ai-error-card strong,
.ai-error-card p {
  display: block;
}

.ai-error-card span {
  color: #ea580c;
  font-size: 12px;
  font-weight: 900;
}

.ai-error-card strong {
  margin-top: 6px;
  color: #0f172a;
  font-size: 18px;
}

.ai-error-card p {
  margin: 6px 0 0;
  color: #64748b;
}

.ai-error-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  justify-content: flex-end;
}

.metric-grid {
  display: grid;
  grid-template-columns: 1.25fr repeat(4, 1fr);
  gap: 14px;
}

.module-grid {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 14px;
}

.module-card {
  min-height: 126px;
  padding: 16px;
  text-align: left;
  border: 1px solid #dce8f5;
  border-radius: 8px;
  background: #ffffff;
  box-shadow: 0 14px 36px rgba(15, 39, 71, 0.06);
  cursor: pointer;
  transition: 0.2s ease;
}

.module-card:hover,
.module-card.active {
  transform: translateY(-2px);
  border-color: #2563eb;
  background: #eff6ff;
  box-shadow: 0 18px 42px rgba(37, 99, 235, 0.12);
}

.module-card span,
.module-card strong,
.module-card small {
  display: block;
}

.module-card span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
}

.module-card strong {
  margin: 7px 0 6px;
  color: #0f172a;
  font-size: 18px;
}

.module-card small {
  color: #64748b;
  line-height: 1.55;
}

.scenario-context {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: center;
  padding: 18px 20px;
  border: 1px solid #bfdbfe;
  border-radius: 8px;
  background:
    radial-gradient(circle at 100% 0%, rgba(14, 165, 233, 0.13), transparent 34%),
    #eff6ff;
}

.scenario-context span,
.scenario-context strong,
.scenario-context p {
  display: block;
}

.scenario-context span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
}

.scenario-context strong {
  margin-top: 6px;
  color: #0f2747;
  font-size: 20px;
}

.scenario-context p {
  margin: 7px 0 0;
  color: #64748b;
  line-height: 1.6;
}

.execution-strip {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.execution-strip div {
  display: grid;
  gap: 6px;
  padding: 14px;
  border: 1px solid #dce8f5;
  border-radius: 8px;
  background: #f8fbff;
}

.execution-strip span {
  width: 26px;
  height: 26px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  color: #fff;
  background: #2563eb;
  font-weight: 900;
}

.execution-strip strong {
  color: #0f172a;
}

.execution-strip small {
  color: #64748b;
  line-height: 1.45;
}

.metric-card,
.panel-card {
  border: 1px solid #dce8f5;
  border-radius: 26px;
  background: rgba(255, 255, 255, 0.9);
  box-shadow: 0 18px 46px rgba(15, 39, 71, 0.07);
}

.metric-card {
  min-height: 138px;
  padding: 18px;
}

.metric-card.primary {
  background: linear-gradient(135deg, #eff6ff, #dbeafe);
}

.metric-card.accent {
  background: linear-gradient(135deg, #ecfeff, #e0f2fe);
}

.metric-card strong {
  display: block;
  margin: 12px 0 8px;
  color: #0f172a;
  font-size: 30px;
  letter-spacing: -0.04em;
}

.metric-card small {
  color: #64748b;
}

.workbench-grid {
  display: grid;
  gap: 18px;
}

.assist-row {
  display: grid;
  grid-template-columns: minmax(0, 1.35fr) minmax(360px, 0.75fr);
  gap: 18px;
}

.studio-row {
  display: grid;
  grid-template-columns: minmax(420px, 0.88fr) minmax(560px, 1.12fr);
  gap: 18px;
  align-items: start;
}

.panel-card {
  padding: 22px;
}

.assist-card {
  min-height: 0;
}

.section-head {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 18px;
}

.section-head.compact {
  align-items: center;
}

.section-head h2 {
  margin: 6px 0 8px;
  color: #0f172a;
  font-size: 24px;
  letter-spacing: -0.03em;
}

.section-head p {
  margin: 0;
  color: #64748b;
  line-height: 1.75;
}

.action-list {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.action-card {
  display: grid;
  grid-template-columns: 38px minmax(0, 1fr);
  align-items: start;
  gap: 12px;
  border: 1px solid #dce8f5;
  border-radius: 16px;
  padding: 14px;
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

.action-icon {
  display: grid;
  width: 38px;
  height: 38px;
  place-items: center;
  border-radius: 14px;
  color: #fff;
  background: linear-gradient(135deg, #2563eb, #06b6d4);
  font-size: 18px;
  font-weight: 900;
}

.action-card strong {
  display: block;
  margin: 4px 0;
  color: #0f172a;
  font-size: 16px;
}

.action-card p {
  margin: 0;
  color: #64748b;
  line-height: 1.6;
}

.action-card .el-button {
  grid-column: 2;
  justify-self: start;
  padding-left: 0;
}

.generator-card {
  background:
    radial-gradient(circle at right top, rgba(37, 99, 235, 0.08), transparent 34%),
    #fff;
}

.form-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.studio-form :deep(.el-input__wrapper),
.studio-form :deep(.el-textarea__inner),
.video-card :deep(.el-input__wrapper),
.video-card :deep(.el-textarea__inner) {
  border-radius: 14px;
}

.result-card {
  min-height: 0;
}

.output-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin: -6px 0 14px;
}

.output-meta span {
  padding: 5px 9px;
  border: 1px solid #dbeafe;
  border-radius: 999px;
  color: #2563eb;
  background: #eff6ff;
  font-size: 12px;
  font-weight: 700;
}

.output-meta.dark {
  margin: 0 0 12px;
}

.output-meta.dark span {
  color: #dbeafe;
  border-color: rgba(219, 234, 254, 0.24);
  background: rgba(255, 255, 255, 0.08);
}

.structured-output {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 12px;
  margin-bottom: 14px;
}

.structured-output article {
  min-height: 132px;
  padding: 14px;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  background: #f8fbff;
}

.structured-output span,
.structured-output strong,
.structured-output p {
  display: block;
}

.structured-output span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
}

.structured-output strong {
  margin-top: 6px;
  color: #0f172a;
  font-size: 17px;
}

.structured-output p {
  display: -webkit-box;
  min-height: 52px;
  margin: 8px 0 12px;
  overflow: hidden;
  color: #64748b;
  line-height: 1.55;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
}

.structured-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.copy-box {
  min-height: 210px;
  border-radius: 22px;
  padding: 18px;
  color: #dbeafe;
  background:
    radial-gradient(circle at top right, rgba(37, 99, 235, 0.36), transparent 26%),
    #0f172a;
}

.copy-box.compact {
  min-height: 180px;
}

.copy-box.compact pre {
  max-height: 260px;
  overflow: auto;
}

.copy-box pre {
  margin: 0 0 16px;
  white-space: pre-wrap;
  word-break: break-word;
  line-height: 1.75;
  font-family: inherit;
}

.empty-result {
  display: grid;
  min-height: 190px;
  place-content: center;
  border: 1px dashed #bfd4ef;
  border-radius: 22px;
  background: linear-gradient(135deg, #f8fbff, #eef6ff);
  text-align: center;
  color: #64748b;
}

.empty-result strong {
  color: #0f172a;
  font-size: 18px;
}

.bottom-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.1fr) minmax(0, 1.1fr) minmax(320px, 0.8fr);
  gap: 18px;
}

.poster-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.poster-card {
  border-radius: 18px;
  padding: 16px;
  border: 1px solid #dce8f5;
  background: #f8fbff;
}

.poster-card strong {
  display: block;
  margin: 8px 0;
  color: #0f172a;
  font-size: 28px;
}

.poster-card small {
  color: #64748b;
}

.signal-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.signal-row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 0;
  border-bottom: 1px solid #e2e8f0;
}

.signal-row strong {
  color: #2563eb;
}

@media (max-width: 1320px) {
  .metric-grid,
  .bottom-grid,
  .module-grid,
  .execution-strip,
  .action-list {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .assist-row,
  .studio-row {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 820px) {
  .ai-cockpit,
  .metric-grid,
  .bottom-grid,
  .module-grid,
  .execution-strip,
  .form-row,
  .structured-output,
  .action-list,
  .poster-grid {
    grid-template-columns: 1fr;
  }

  .ai-cockpit {
    padding: 22px;
  }

  .action-card {
    grid-template-columns: 38px minmax(0, 1fr);
  }

  .ai-error-card {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
