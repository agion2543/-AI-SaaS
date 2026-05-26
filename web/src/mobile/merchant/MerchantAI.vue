<template>
  <div class="merchant-ai">
    <!-- 顶部导航 -->
    <van-nav-bar
      title="AI 运营助手"
      left-arrow
      fixed
      placeholder
      @click-left="router.back()"
    />

    <!-- AI 洞察 -->
    <div class="ai-insights">
      <div class="section-title">经营洞察</div>
      <div v-if="insights.length > 0" class="insights-list">
        <div v-for="(insight, index) in insights" :key="index" class="insight-item">
          <div class="insight-icon">
            <van-icon :name="getInsightIcon(insight.type)" />
          </div>
          <div class="insight-content">
            <div class="insight-title">{{ insight.title }}</div>
            <div class="insight-desc">{{ insight.description }}</div>
          </div>
        </div>
      </div>
      <van-skeleton v-else title :row="3" />
    </div>

    <!-- AI 工具 -->
    <div class="ai-tools">
      <div class="section-title">AI 工具</div>
      <van-grid :column="2" :gutter="10">
        <van-grid-item
          icon="chat-o"
          text="营销文案"
          @click="openAIMarketing"
        />
        <van-grid-item
          icon="coupon-o"
          text="活动策划"
          @click="openAIActivity"
        />
        <van-grid-item
          icon="bulb-o"
          text="经营建议"
          @click="openAISuggestions"
        />
        <van-grid-item
          icon="description"
          text="自动生成"
          @click="openAIAutoGenerate"
        />
      </van-grid>
    </div>

    <!-- AI 营销文案 -->
    <van-popup v-model:show="showMarketingDialog" position="bottom" round style="height: 80%;">
      <div class="ai-dialog">
        <div class="dialog-header">
          <span>AI 营销文案</span>
          <van-icon name="cross" @click="showMarketingDialog = false" />
        </div>

        <div class="dialog-content">
          <van-cell-group inset>
            <van-field
              v-model="marketingForm.type"
              label="文案类型"
              readonly
              is-link
              placeholder="请选择类型"
              @click="showTypePicker = true"
            />
            <van-field
              v-model="marketingForm.store_name"
              label="门店名称"
              placeholder="请输入门店名称"
            />
            <van-field
              v-model="marketingForm.products"
              label="主打商品"
              placeholder="请输入商品名称，多个用逗号分隔"
            />
            <van-field
              v-model="marketingForm.keywords"
              label="关键词"
              placeholder="如: 新品上市、限时优惠"
            />
          </van-cell-group>

          <div class="generate-btn">
            <van-button
              type="primary"
              block
              :loading="generating"
              @click="generateMarketingCopy"
            >
              生成文案
            </van-button>
          </div>

          <div v-if="generatedCopy" class="generated-content">
            <div class="content-header">
              <span>生成的文案</span>
              <van-button size="mini" @click="copyContent">复制</van-button>
            </div>
            <div class="content-text">{{ generatedCopy }}</div>
          </div>
        </div>
      </div>
    </van-popup>

    <!-- 类型选择 -->
    <van-popup v-model:show="showTypePicker" position="bottom" style="height: 50%;">
      <van-picker
        title="选择文案类型"
        :columns="copyTypes"
        @confirm="onTypeConfirm"
        @cancel="showTypePicker = false"
      />
    </van-popup>

    <!-- AI 活动策划 -->
    <van-popup v-model:show="showActivityDialog" position="bottom" round style="height: 80%;">
      <div class="ai-dialog">
        <div class="dialog-header">
          <span>AI 活动策划</span>
          <van-icon name="cross" @click="showActivityDialog = false" />
        </div>

        <div class="dialog-content">
          <van-cell-group inset>
            <van-field
              v-model="activityForm.theme"
              label="活动主题"
              placeholder="请输入活动主题"
            />
            <van-field
              v-model="activityForm.budget"
              label="预算"
              placeholder="请输入预算金额"
            />
            <van-field
              v-model="activityForm.duration"
              label="活动时长"
              placeholder="如: 3天、一周"
            />
          </van-cell-group>

          <div class="generate-btn">
            <van-button
              type="primary"
              block
              :loading="generatingActivity"
              @click="generateActivity"
            >
              生成方案
            </van-button>
          </div>

          <div v-if="generatedActivity" class="generated-content">
            <div class="content-header">
              <span>活动方案</span>
              <van-button size="mini" @click="copyActivity">复制</van-button>
            </div>
            <div class="content-text">{{ generatedActivity }}</div>
          </div>
        </div>
      </div>
    </van-popup>

    <!-- AI 建议 -->
    <van-popup v-model:show="showSuggestionsDialog" position="bottom" round style="height: 80%;">
      <div class="ai-dialog">
        <div class="dialog-header">
          <span>AI 经营建议</span>
          <van-icon name="cross" @click="showSuggestionsDialog = false" />
        </div>

        <div class="dialog-content">
          <van-loading v-if="loadingSuggestions" type="spinner" style="text-align: center; padding: 40px;">
            AI 分析中...
          </van-loading>

          <div v-else-if="suggestions.length > 0" class="suggestions-list">
            <div v-for="(s, index) in suggestions" :key="index" class="suggestion-item">
              <div class="suggestion-title">
                <van-icon :name="getSuggestionIcon(s.priority)" />
                {{ s.title }}
              </div>
              <div class="suggestion-desc">{{ s.description }}</div>
              <van-tag v-if="s.action" type="primary" size="small">{{ s.action }}</van-tag>
            </div>
          </div>
        </div>
      </div>
    </van-popup>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { showToast, showSuccessToast } from 'vant'
import { fetchMerchantAIInsights, generateMerchantAIMarketingCopy, fetchMerchantShareStats } from '@/api/modules'

const router = useRouter()

const loading = ref(true)
const insights = ref([])
const loadingSuggestions = ref(false)
const suggestions = ref([])

const showMarketingDialog = ref(false)
const showActivityDialog = ref(false)
const showSuggestionsDialog = ref(false)
const showTypePicker = ref(false)
const generating = ref(false)
const generatingActivity = ref(false)
const generatedCopy = ref('')
const generatedActivity = ref('')

const marketingForm = ref({
  type: '朋友圈文案',
  store_name: '',
  products: '',
  keywords: ''
})

const activityForm = ref({
  theme: '',
  budget: '',
  duration: ''
})

const copyTypes = [
  { text: '朋友圈文案', value: '朋友圈文案' },
  { text: '小红书笔记', value: '小红书笔记' },
  { text: '抖音短视频脚本', value: '抖音短视频脚本' },
  { text: '公众号推文', value: '公众号推文' },
  { text: '店铺公告', value: '店铺公告' }
]

function getInsightIcon(type) {
  const iconMap = {
    sales: 'chart-trending-o',
    customer: 'friends-o',
    product: 'shop-o',
    promotion: 'coupon-o'
  }
  return iconMap[type] || 'bulb-o'
}

function getSuggestionIcon(priority) {
  const iconMap = {
    high: 'warning-o',
    medium: 'info-o',
    low: 'success-o'
  }
  return iconMap[priority] || 'info-o'
}

async function loadInsights() {
  loading.value = true
  try {
    const [insightsRes, statsRes] = await Promise.all([
      fetchMerchantAIInsights().catch(() => ({ data: [] })),
      fetchMerchantShareStats().catch(() => ({ data: {} }))
    ])

    insights.value = insightsRes.data || []

    // 添加分享数据洞察
    const stats = statsRes.data || {}
    if (stats.total_sessions || stats.total_orders) {
      insights.value.push({
        type: 'sales',
        title: '分享效果',
        description: `分享带来 ${stats.total_sessions || 0} 次访问，${stats.total_orders || 0} 个订单`
      })
    }
  } catch (e) {
    console.error('加载洞察失败:', e)
  } finally {
    loading.value = false
  }
}

function openAIMarketing() {
  showMarketingDialog.value = true
}

function openAIActivity() {
  showActivityDialog.value = true
}

async function openAISuggestions() {
  showSuggestionsDialog.value = true
  loadingSuggestions.value = true

  try {
    // 模拟 AI 建议
    await new Promise(resolve => setTimeout(resolve, 1500))

    suggestions.value = [
      {
        title: '优化商品价格',
        description: '根据市场调研，建议适当降低 2-3 款主力商品价格，可提升约 15% 的订单转化率',
        priority: 'high',
        action: '查看详情'
      },
      {
        title: '增加午间优惠',
        description: '11:00-14:00 时段订单较少，建议推出午市特惠套餐，吸引更多顾客',
        priority: 'medium',
        action: '创建活动'
      },
      {
        title: '会员积分体系',
        description: '建议开启会员积分功能，提升用户复购率',
        priority: 'low',
        action: '了解更多'
      }
    ]
  } catch (e) {
    showToast('加载失败')
  } finally {
    loadingSuggestions.value = false
  }
}

function openAIAutoGenerate() {
  showToast('功能开发中')
}

function onTypeConfirm({ selectedOptions }) {
  marketingForm.value.type = selectedOptions[0].text
  showTypePicker.value = false
}

async function generateMarketingCopy() {
  if (!marketingForm.value.store_name) {
    showToast('请输入门店名称')
    return
  }

  generating.value = true
  try {
    const res = await generateMerchantAIMarketingCopy({
      type: marketingForm.value.type,
      store_name: marketingForm.value.store_name,
      products: marketingForm.value.products,
      keywords: marketingForm.value.keywords
    })

    generatedCopy.value = res.data?.content || '生成的文案内容'
    showSuccessToast('文案已生成')
  } catch (e) {
    // 模拟生成
    await new Promise(resolve => setTimeout(resolve, 1000))
    generatedCopy.value = `【${marketingForm.value.store_name}】

${marketingForm.value.type === '朋友圈文案' ? '📣 新品上市！' : '✨ 好消息！'}

${marketingForm.value.products ? `我们的主打商品「${marketingForm.value.products}」` : '本店商品'}现在有优惠活动！

${marketingForm.value.keywords || '优惠多多，实惠多多'} 🛒

快来尝鲜吧~ 地址：[门店地址]

#${marketingForm.value.store_name} #美食探店`

    showSuccessToast('文案已生成')
  } finally {
    generating.value = false
  }
}

function copyContent() {
  navigator.clipboard.writeText(generatedCopy.value).then(() => {
    showSuccessToast('已复制')
  }).catch(() => {
    showToast('复制失败')
  })
}

async function generateActivity() {
  if (!activityForm.value.theme) {
    showToast('请输入活动主题')
    return
  }

  generatingActivity.value = true
  try {
    await new Promise(resolve => setTimeout(resolve, 1500))

    generatedActivity.value = `【${activityForm.value.theme}活动策划方案】

📅 活动时长: ${activityForm.value.duration || '一周'}
💰 预算: ¥${activityForm.value.budget || '1000'}

一、活动目标
- 提升门店曝光度
- 增加新顾客到店
- 促进老顾客复购

二、活动内容
1. 限时折扣：全店商品 8 折
2. 满减活动：满 50 减 10
3. 会员专享：会员额外优惠 5%

三、宣传渠道
1. 朋友圈海报
2. 小红书种草笔记
3. 抖音短视频推广

四、预期效果
- 活动期间订单量提升 30%
- 新增会员 50 人
- 营收增长 20%

请根据实际情况调整方案！`

    showSuccessToast('方案已生成')
  } catch (e) {
    showToast('生成失败')
  } finally {
    generatingActivity.value = false
  }
}

function copyActivity() {
  navigator.clipboard.writeText(generatedActivity.value).then(() => {
    showSuccessToast('已复制')
  }).catch(() => {
    showToast('复制失败')
  })
}

onMounted(() => {
  loadInsights()
})
</script>

<style scoped>
.merchant-ai {
  min-height: 100vh;
  background: #f7f8fa;
  padding-bottom: 20px;
}

.ai-insights,
.ai-tools {
  background: white;
  margin: 12px;
  border-radius: 12px;
  padding: 16px;
}

.section-title {
  font-size: 16px;
  font-weight: bold;
  margin-bottom: 16px;
}

.insights-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.insight-item {
  display: flex;
  gap: 12px;
  padding: 12px;
  background: #f7f8fa;
  border-radius: 8px;
}

.insight-icon {
  font-size: 24px;
  color: #667eea;
}

.insight-title {
  font-size: 14px;
  font-weight: bold;
  margin-bottom: 4px;
}

.insight-desc {
  font-size: 12px;
  color: #666;
}

.ai-dialog {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.dialog-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px;
  font-size: 16px;
  font-weight: bold;
  border-bottom: 1px solid #eee;
}

.dialog-content {
  flex: 1;
  overflow-y: auto;
  padding: 16px;
}

.generate-btn {
  margin-top: 16px;
}

.generated-content {
  margin-top: 20px;
  padding: 16px;
  background: #f7f8fa;
  border-radius: 8px;
}

.content-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  font-weight: bold;
}

.content-text {
  font-size: 14px;
  line-height: 1.8;
  white-space: pre-wrap;
}

.suggestions-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.suggestion-item {
  padding: 16px;
  background: #f7f8fa;
  border-radius: 8px;
}

.suggestion-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: bold;
  margin-bottom: 8px;
}

.suggestion-desc {
  font-size: 13px;
  color: #666;
  margin-bottom: 8px;
  line-height: 1.5;
}
</style>
