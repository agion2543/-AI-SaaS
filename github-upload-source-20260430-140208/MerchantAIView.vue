<template>
  <div class="ai-stack">
    <section class="page-card hero-card">
      <div class="card-toolbar">
        <div>
          <h2 class="page-title">AI 经营分析</h2>
          <p class="muted">
            基于订单、商品销量、顾客画像和线索跟进数据，给商家生成可执行的经营建议。
          </p>
        </div>
        <div class="hero-actions">
          <el-button @click="load">刷新分析</el-button>
          <el-button type="primary" :loading="generating" @click="generateDraft">生成活动草稿</el-button>
        </div>
      </div>

      <div class="summary-grid">
        <div class="summary-item">
          <span>订单数</span>
          <strong>{{ insights.order_stats?.order_count || 0 }}</strong>
        </div>
        <div class="summary-item">
          <span>交易额</span>
          <strong>¥{{ formatYuan(insights.order_stats?.trade_amount) }}</strong>
        </div>
        <div class="summary-item">
          <span>取消率</span>
          <strong>{{ cancelRate }}</strong>
        </div>
        <div class="summary-item">
          <span>顾客档案</span>
          <strong>{{ insights.customer_summary?.total || 0 }}</strong>
        </div>
        <div class="summary-item">
          <span>高价值顾客</span>
          <strong>{{ insights.customer_summary?.high_value || 0 }}</strong>
        </div>
        <div class="summary-item">
          <span>需召回顾客</span>
          <strong>{{ recallCustomerCount }}</strong>
        </div>
        <div class="summary-item">
          <span>线索总数</span>
          <strong>{{ insights.summary?.total_leads || 0 }}</strong>
        </div>
        <div class="summary-item">
          <span>分析引擎</span>
          <strong>{{ insights.engine || '-' }}</strong>
        </div>
      </div>
    </section>

    <section class="page-card">
      <div class="section-title">
        <div>
          <h2 class="page-title">顾客画像与 AI 维护建议</h2>
          <p class="muted">只展示当前商家自己的顾客数据，按消费金额和最近行为排序。</p>
        </div>
      </div>
      <el-table :data="insights.customer_profiles || []" empty-text="暂无顾客订单数据">
        <el-table-column prop="customer_phone" label="顾客手机号" min-width="140" />
        <el-table-column prop="store_name" label="最近门店" min-width="150" />
        <el-table-column prop="order_count" label="消费次数" width="100" />
        <el-table-column label="累计消费" width="120">
          <template #default="{ row }">¥{{ formatYuan(row.total_amount) }}</template>
        </el-table-column>
        <el-table-column label="AI 标签" width="120">
          <template #default="{ row }">
            <el-tag :type="tagType(row.ai_tag)">{{ row.ai_tag || '待分析' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="ai_suggestion" label="建议动作" min-width="260" show-overflow-tooltip />
        <el-table-column label="活动生成" width="140">
          <template #default="{ row }">
            <el-button size="small" type="primary" text :loading="generating" @click="generateDraft(row.ai_tag)">
              转为优惠活动
            </el-button>
          </template>
        </el-table-column>
        <el-table-column label="最近下单" min-width="170">
          <template #default="{ row }">{{ formatTime(row.last_order_at) }}</template>
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
          <el-table-column label="建议" min-width="150">
            <template #default>优化图片/标题，或设置折扣活动测试</template>
          </el-table-column>
        </el-table>
      </div>
    </section>

    <section class="page-card">
      <h2 class="page-title">经营建议</h2>
      <div class="insight-list">
        <div
          v-for="(item, index) in insights.insights || []"
          :key="`${item.title}-${index}`"
          class="insight-item"
          :class="`insight-${item.type}`"
        >
          <el-tag size="small" :type="insightTagType(item.type)">{{ insightTypeLabel(item.type) }}</el-tag>
          <div>
            <strong>{{ item.title }}</strong>
            <p>{{ item.content }}</p>
            <el-button
              v-if="item.type === 'action' || item.type === 'info'"
              class="inline-action"
              type="primary"
              text
              :loading="generating"
              @click="generateDraft()"
            >
              转为优惠活动
            </el-button>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { fetchMerchantAIInsights, generateMerchantPromotionDraft } from '../../api/modules'

const router = useRouter()
const insights = ref({})
const generating = ref(false)

const load = async () => {
  const res = await fetchMerchantAIInsights()
  insights.value = res.data || {}
}

const generateDraft = async (customerTag = '') => {
  generating.value = true
  try {
    await generateMerchantPromotionDraft({ customer_tag: customerTag })
    ElMessage.success('已生成优惠活动草稿，可继续编辑后发布')
    router.push('/merchant/promotions')
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '生成草稿失败')
  } finally {
    generating.value = false
  }
}

const formatRate = (value) => `${Math.round((Number(value) || 0) * 100)}%`
const formatYuan = (value) => (Number(value || 0) / 100).toFixed(2)
const formatTime = (value) => value ? String(value).replace('T', ' ').slice(0, 19) : '-'
const cancelRate = computed(() => {
  const total = Number(insights.value.order_stats?.order_count || 0)
  const closed = Number(insights.value.order_stats?.closed_count || 0)
  return total ? formatRate(closed / total) : '0%'
})
const recallCustomerCount = computed(() => {
  const summary = insights.value.customer_summary || {}
  return Number(summary.sleeping || 0) + Number(summary.risk || 0)
})

const insightTagType = (type) => ({
  warning: 'warning',
  action: 'primary',
  success: 'success',
  info: 'info'
}[type] || 'info')

const insightTypeLabel = (type) => ({
  warning: '风险',
  action: '建议',
  success: '亮点',
  info: '提示'
}[type] || '提示')

const tagType = (tag) => ({
  高价值顾客: 'danger',
  复购顾客: 'success',
  沉睡顾客: 'warning',
  流失风险: 'warning',
  新顾客: 'primary'
}[tag] || 'info')

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

.card-toolbar,
.section-title {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: flex-start;
}

.hero-actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.muted {
  margin: 0;
  color: var(--muted);
  line-height: 1.7;
}

.summary-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 14px;
  margin-top: 18px;
}

.summary-item {
  padding: 16px;
  border: 1px solid #e5edf9;
  border-radius: 14px;
  background: rgba(248, 251, 255, 0.9);
}

.summary-item span {
  display: block;
  color: var(--muted);
  font-size: 13px;
}

.summary-item strong {
  display: block;
  margin-top: 8px;
  font-size: 22px;
}

.insight-list {
  display: grid;
  gap: 12px;
}

.grid-2 {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 20px;
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
  color: var(--muted);
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

@media (max-width: 720px) {
  .card-toolbar,
  .section-title,
  .insight-item {
    grid-template-columns: 1fr;
  }

  .card-toolbar,
  .section-title {
    flex-direction: column;
  }
}
</style>
