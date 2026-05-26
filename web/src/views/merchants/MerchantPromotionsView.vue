<template>
  <div class="promotion-page">
    <PageHero
      eyebrow="MARKETING CENTER"
      title="优惠活动"
      description="把 AI 经营建议落到真实活动里。发布后的满减、折扣活动会进入顾客扫码页，并影响后续下单实付金额。"
      tone="brand"
    >
      <template #actions>
        <el-button plain @click="loadPromotions">刷新活动</el-button>
        <el-button plain @click="exportPromotionEffects">导出效果 XLSX</el-button>
        <el-button type="primary" size="large" @click="openCreate">新增活动</el-button>
      </template>
    </PageHero>

    <section class="metric-grid">
      <MetricCard label="活动总数" :value="total" hint="当前商家已创建的营销活动。" tone="blue" />
      <MetricCard label="发布中" :value="publishedCount" hint="顾客扫码页可见且可用。" tone="green" />
      <MetricCard label="草稿" :value="draftCount" hint="待完善后发布。" tone="orange" />
      <MetricCard label="停用" :value="inactiveCount" hint="已暂停展示或过期活动。" tone="slate" />
    </section>

    <section v-if="showAiDraftTip" class="ai-draft-tip">
      <div>
        <span>AI 草稿已生成</span>
        <strong>请先检查门店、有效期、门槛金额和优惠力度</strong>
        <p>确认无误后点击“编辑”补充细节，再发布到顾客扫码页。列表中已为你高亮最新 AI 草稿。</p>
      </div>
      <el-button type="primary" plain @click="openLatestDraft">编辑最新草稿</el-button>
    </section>

    <section class="effect-summary">
      <div>
        <span>活动带单</span>
        <strong>{{ promotionSummary.order_count }}</strong>
        <small>已关联优惠活动的扫码订单</small>
      </div>
      <div>
        <span>有效支付</span>
        <strong>{{ promotionSummary.paid_count }}</strong>
        <small>已支付 / 已接单 / 已完成</small>
      </div>
      <div>
        <span>优惠金额</span>
        <strong>{{ formatCurrency(promotionSummary.discount_amount) }}</strong>
        <small>活动实际让利金额</small>
      </div>
      <div>
        <span>活动实收</span>
        <strong>{{ formatCurrency(promotionSummary.revenue_amount) }}</strong>
        <small>扣除优惠后的订单实收</small>
      </div>
    </section>

    <section class="marketing-dashboard" :class="marketingTone">
      <div class="dashboard-main">
        <span>MARKETING ROI</span>
        <strong>{{ marketingBrief.title }}</strong>
        <p>{{ marketingBrief.text }}</p>
      </div>
      <div class="dashboard-grid">
        <div v-for="item in marketingMetrics" :key="item.label" :class="item.tone">
          <span>{{ item.label }}</span>
          <strong>{{ item.value }}</strong>
          <small>{{ item.hint }}</small>
        </div>
      </div>
    </section>

    <section class="action-board">
      <button
        v-for="item in marketingActions"
        :key="item.key"
        type="button"
        :class="['action-card-lite', item.tone]"
        @click="item.go"
      >
        <span>{{ item.label }}</span>
        <strong>{{ item.title }}</strong>
        <small>{{ item.hint }}</small>
      </button>
    </section>

    <DataPanel
      title="活动列表"
      subtitle="支持满减活动和自定义折扣活动。过期或测试活动可以删除，避免商家端堆积无效活动。"
    >
      <el-table
        v-loading="loading"
        :data="promotions"
        empty-text="暂无优惠活动"
        :row-class-name="promotionRowClass"
      >
        <el-table-column prop="title" label="活动名称" min-width="180" />
        <el-table-column label="活动类型" width="110">
          <template #default="{ row }">
            <el-tag :type="row.type === 'discount' ? 'warning' : 'success'">
              {{ row.type === 'discount' ? '折扣' : '满减' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="适用门店" min-width="150">
          <template #default="{ row }">{{ row.store?.name || '全部门店' }}</template>
        </el-table-column>
        <el-table-column label="优惠力度" min-width="170">
          <template #default="{ row }">{{ formatDiscount(row) }}</template>
        </el-table-column>
        <el-table-column label="活动效果" min-width="230">
          <template #default="{ row }">
            <div class="effect-cell">
              <strong>{{ promotionEffect(row).paid_count }} / {{ promotionEffect(row).order_count }} 单</strong>
              <span>优惠 {{ formatCurrency(promotionEffect(row).discount_amount) }} · 实收 {{ formatCurrency(promotionEffect(row).revenue_amount) }}</span>
              <small>{{ promotionEffectAdvice(row) }}</small>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="ROI 判断" min-width="170">
          <template #default="{ row }">
            <div class="roi-cell">
              <strong>{{ promotionRoi(row).label }}</strong>
              <span>{{ promotionRoi(row).hint }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)">{{ statusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="有效期" min-width="210">
          <template #default="{ row }">{{ formatDate(row.valid_from) }} - {{ formatDate(row.valid_to) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="280" fixed="right">
          <template #default="{ row }">
            <div class="action-row">
              <el-button size="small" @click="openEdit(row)">编辑</el-button>
              <el-button
                v-if="row.status !== 'published'"
                size="small"
                type="success"
                @click="changeStatus(row, 'published')"
              >
                发布
              </el-button>
              <el-button v-else size="small" type="warning" @click="changeStatus(row, 'inactive')">停用</el-button>
              <el-button size="small" type="primary" plain @click="reviewPromotionWithAI(row)">AI复盘</el-button>
              <el-button size="small" type="danger" plain @click="removePromotion(row)">删除</el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <div class="pager">
        <el-pagination
          background
          layout="prev, pager, next"
          :total="total"
          :page-size="pageSize"
          :current-page="page"
          @current-change="handlePageChange"
        />
      </div>
    </DataPanel>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑优惠活动' : '新增优惠活动'" width="680px">
      <el-form label-position="top">
        <div v-if="aiReviewNote" class="ai-review-note">
          <div>
            <span>AI 复盘建议</span>
            <strong>可根据建议调整门槛、优惠力度、有效期或活动状态</strong>
          </div>
          <pre>{{ aiReviewNote }}</pre>
          <el-button size="small" plain @click="aiReviewNote = ''">收起建议</el-button>
        </div>

        <el-form-item label="活动名称">
          <el-input v-model="form.title" maxlength="120" placeholder="例如：新客到店立减活动" />
        </el-form-item>

        <el-form-item label="活动说明">
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="3"
            maxlength="500"
            placeholder="写清楚适用规则，方便顾客扫码后理解"
          />
        </el-form-item>

        <el-row :gutter="14">
          <el-col :span="12">
            <el-form-item label="适用门店">
              <el-select v-model="form.store_id" clearable placeholder="全部门店">
                <el-option v-for="store in stores" :key="store.id" :label="store.name" :value="store.id" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="状态">
              <el-select v-model="form.status">
                <el-option label="草稿" value="draft" />
                <el-option label="发布中" value="published" />
                <el-option label="已停用" value="inactive" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>

        <el-form-item label="活动类型">
          <el-radio-group v-model="form.type" class="type-switch">
            <el-radio-button label="amount">满减活动</el-radio-button>
            <el-radio-button label="discount">折扣活动</el-radio-button>
          </el-radio-group>
        </el-form-item>

        <div class="discount-box">
          <el-row :gutter="14">
            <el-col :span="12">
              <el-form-item label="门槛金额（元）">
                <el-input-number v-model="form.threshold_yuan" :min="0" :precision="2" :step="10" class="full-input" />
              </el-form-item>
            </el-col>
            <el-col v-if="form.type === 'amount'" :span="12">
              <el-form-item label="优惠金额（元）">
                <el-input-number v-model="form.discount_yuan" :min="0" :precision="2" :step="5" class="full-input" />
              </el-form-item>
            </el-col>
            <el-col v-else :span="12">
              <el-form-item label="自定义折率">
                <el-input-number v-model="form.discount_rate" :min="1" :max="99" :step="1" class="full-input" />
                <div class="form-tip">例如填写 85 表示 8.5 折，填写 70 表示 7 折。</div>
              </el-form-item>
            </el-col>
          </el-row>
        </div>

        <el-row :gutter="14">
          <el-col :span="12">
            <el-form-item label="开始日期">
              <el-date-picker v-model="form.valid_from" type="date" value-format="YYYY-MM-DD" placeholder="不限" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="结束日期">
              <el-date-picker v-model="form.valid_to" type="date" value-format="YYYY-MM-DD" placeholder="不限" />
            </el-form-item>
          </el-col>
        </el-row>

        <div class="publish-check">
          <div class="check-head">
            <span>发布前检查</span>
            <strong>{{ publishChecks.blockers.length ? '需要处理后再发布' : '基础检查通过' }}</strong>
          </div>
          <div class="check-list">
            <div
              v-for="item in publishChecks.items"
              :key="item.text"
              class="check-item"
              :class="item.ok ? 'ok' : 'risk'"
            >
              <span>{{ item.ok ? '通过' : '注意' }}</span>
              <p>{{ item.text }}</p>
            </div>
          </div>
        </div>
      </el-form>

      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="savePromotion">保存活动</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  createMerchantPromotion,
  deleteMerchantPromotion,
  fetchMerchantOrders,
  fetchMerchantPromotions,
  fetchMerchantStores,
  updateMerchantPromotion
} from '../../api/modules'
import DataPanel from '../../components/design/DataPanel.vue'
import MetricCard from '../../components/design/MetricCard.vue'
import PageHero from '../../components/design/PageHero.vue'
import { exportRowsToXlsx } from '../../utils/xlsx'

const route = useRoute()
const router = useRouter()
const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const promotions = ref([])
const orders = ref([])
const stores = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 10
const editingId = ref(null)
const showAiDraftTip = ref(false)
const aiReviewNote = ref('')
const paidStatuses = ['received', 'accepted', 'completed', 'closed']

const form = reactive({
  store_id: null,
  title: '',
  description: '',
  type: 'amount',
  threshold_yuan: 0,
  discount_yuan: 0,
  discount_rate: 85,
  status: 'draft',
  valid_from: '',
  valid_to: ''
})

const publishedCount = computed(() => promotions.value.filter((item) => item.status === 'published').length)
const draftCount = computed(() => promotions.value.filter((item) => item.status === 'draft').length)
const inactiveCount = computed(() => promotions.value.filter((item) => item.status === 'inactive').length)
const latestAiDraft = computed(() => promotions.value.find((item) => (
  item.status === 'draft' && String(item.title || '').includes('AI')
)))
const promotionEffects = computed(() => {
  const map = {}
  orders.value.forEach((order) => {
    const promotionId = Number(order.promotion_id || order.promotion?.id || 0)
    if (!promotionId || order.order_type !== 'store_order') return
    if (!map[promotionId]) {
      map[promotionId] = {
        order_count: 0,
        paid_count: 0,
        discount_amount: 0,
        revenue_amount: 0
      }
    }
    const bucket = map[promotionId]
    bucket.order_count += 1
    bucket.discount_amount += Number(order.discount_amount || 0)
    if (paidStatuses.includes(order.status)) {
      bucket.paid_count += 1
      bucket.revenue_amount += orderPaidAmount(order)
    }
  })
  return map
})
const promotionSummary = computed(() => Object.values(promotionEffects.value).reduce((acc, item) => {
  acc.order_count += item.order_count
  acc.paid_count += item.paid_count
  acc.discount_amount += item.discount_amount
  acc.revenue_amount += item.revenue_amount
  return acc
}, { order_count: 0, paid_count: 0, discount_amount: 0, revenue_amount: 0 }))
const marketingConversionRate = computed(() => promotionSummary.value.order_count
  ? Math.round((promotionSummary.value.paid_count / promotionSummary.value.order_count) * 100)
  : 0)
const marketingDiscountRate = computed(() => {
  const base = promotionSummary.value.discount_amount + promotionSummary.value.revenue_amount
  return base > 0 ? Math.round((promotionSummary.value.discount_amount / base) * 100) : 0
})
const bestPromotion = computed(() => {
  const rows = promotions.value.map((item) => ({ item, effect: promotionEffect(item) }))
  rows.sort((a, b) => b.effect.revenue_amount - a.effect.revenue_amount)
  return rows.find((row) => row.effect.paid_count > 0)
})
const weakPromotionCount = computed(() => promotions.value.filter((item) => {
  const effect = promotionEffect(item)
  return item.status === 'published' && effect.order_count === 0
}).length)
const marketingTone = computed(() => {
  if (!promotionSummary.value.order_count) return 'quiet'
  if (marketingConversionRate.value >= 60 && marketingDiscountRate.value <= 25) return 'good'
  if (marketingDiscountRate.value > 35 || marketingConversionRate.value < 30) return 'warning'
  return 'normal'
})
const marketingBrief = computed(() => {
  if (!promotions.value.length) {
    return { title: '还没有活动，先做一个低风险满减测试', text: '建议先用 7 天小活动验证顾客是否愿意下单，不要一开始给太大优惠。' }
  }
  if (!promotionSummary.value.order_count) {
    return { title: '活动已配置，但还没有带来订单', text: '优先检查活动是否发布、门槛是否过高、顾客扫码页是否能看到活动。' }
  }
  if (marketingDiscountRate.value > 35) {
    return { title: '让利比例偏高，建议核对毛利', text: `当前活动让利约 ${marketingDiscountRate.value}%，如果毛利不够，建议提高门槛或缩短有效期。` }
  }
  if (marketingConversionRate.value < 30) {
    return { title: '活动转化偏低，建议重写权益表达', text: `当前支付转化率 ${marketingConversionRate.value}%，可尝试降低门槛、绑定热销商品或更换活动标题。` }
  }
  return { title: '活动正在带来有效订单', text: `当前支付转化率 ${marketingConversionRate.value}%，活动实收 ${formatCurrency(promotionSummary.value.revenue_amount)}，可继续观察复购。` }
})
const marketingMetrics = computed(() => [
  { label: '支付转化率', value: `${marketingConversionRate.value}%`, hint: '有效支付 / 活动带单', tone: marketingConversionRate.value >= 60 ? 'good' : 'normal' },
  { label: '让利比例', value: `${marketingDiscountRate.value}%`, hint: '优惠金额 / 活动成交额', tone: marketingDiscountRate.value > 35 ? 'risk' : 'normal' },
  { label: '最佳活动', value: bestPromotion.value?.item?.title || '暂无', hint: bestPromotion.value ? `实收 ${formatCurrency(bestPromotion.value.effect.revenue_amount)}` : '需要先产生支付订单', tone: 'normal' },
  { label: '低效活动', value: `${weakPromotionCount.value} 个`, hint: '发布中但暂无订单', tone: weakPromotionCount.value ? 'risk' : 'good' },
  { label: '发布中活动', value: `${publishedCount.value} 个`, hint: '顾客扫码页可见', tone: publishedCount.value ? 'normal' : 'risk' }
])
const marketingActions = computed(() => [
  {
    key: 'ai-review',
    label: '复盘',
    title: 'AI 复盘活动效果',
    hint: '带上订单、让利和实收数据生成调整建议',
    tone: 'primary',
    go: () => router.push({
      path: '/merchant/ai',
      query: {
        scenario: 'promotion_review',
        orders: promotionSummary.value.order_count,
        paid: promotionSummary.value.paid_count,
        discount: centsToYuan(promotionSummary.value.discount_amount).toFixed(2),
        revenue: centsToYuan(promotionSummary.value.revenue_amount).toFixed(2),
        conversion: `${marketingConversionRate.value}%`
      }
    })
  },
  {
    key: 'share',
    label: '裂变',
    title: '查看分享领券效果',
    hint: '把活动和裂变券一起看，判断复购价值',
    tone: 'success',
    go: () => router.push('/merchant/share')
  },
  {
    key: 'coupon',
    label: '券包',
    title: '核对券核销记录',
    hint: '查看优惠券是否被使用、过期或作废',
    tone: 'warning',
    go: () => router.push('/merchant/coupons')
  },
  {
    key: 'create',
    label: '测试',
    title: '新建低风险活动',
    hint: '建议先用短周期小让利活动验证',
    tone: 'neutral',
    go: openCreate
  }
])
const publishChecks = computed(() => buildPromotionChecks({
  title: form.title,
  description: form.description,
  type: form.type,
  threshold: yuanToCents(form.threshold_yuan),
  discount: form.type === 'amount' ? yuanToCents(form.discount_yuan) : 0,
  discount_rate: form.type === 'discount' ? Number(form.discount_rate || 0) : 0,
  valid_from: form.valid_from,
  valid_to: form.valid_to
}))

const centsToYuan = (value) => Number((Number(value || 0) / 100).toFixed(2))
const yuanToCents = (value) => Math.round(Number(value || 0) * 100)

const formatMoney = (value) => {
  const yuan = centsToYuan(value)
  return yuan % 1 === 0 ? yuan.toFixed(0) : yuan.toFixed(2)
}
const formatCurrency = (value) => `¥${centsToYuan(value).toFixed(2)}`
const orderPaidAmount = (order) => Number(order.amount ?? order.total_amount ?? 0)
const promotionEffect = (row) => promotionEffects.value[Number(row.id)] || {
  order_count: 0,
  paid_count: 0,
  discount_amount: 0,
  revenue_amount: 0
}
const promotionEffectAdvice = (row) => {
  const effect = promotionEffect(row)
  if (!effect.order_count) return '暂无订单使用，可考虑调整文案或展示位置'
  if (!effect.paid_count) return '已有顾客触达但未支付，建议检查门槛和优惠力度'
  const payRate = Math.round((effect.paid_count / effect.order_count) * 100)
  if (payRate >= 80) return `转化率 ${payRate}%，活动表现较好`
  if (payRate >= 40) return `转化率 ${payRate}%，可继续观察并优化商品搭配`
  return `转化率 ${payRate}%，建议降低门槛或换成更清晰的权益`
}
const promotionRoi = (row) => {
  const effect = promotionEffect(row)
  if (!effect.order_count) return { label: '待验证', hint: '暂无订单，先看曝光和入口' }
  if (!effect.paid_count) return { label: '触达未成交', hint: '检查门槛、商品和文案' }
  const conversion = Math.round((effect.paid_count / effect.order_count) * 100)
  const discountRate = effect.discount_amount + effect.revenue_amount > 0
    ? Math.round((effect.discount_amount / (effect.discount_amount + effect.revenue_amount)) * 100)
    : 0
  if (discountRate > 35) return { label: '让利偏高', hint: `让利约 ${discountRate}%，核对毛利` }
  if (conversion >= 70) return { label: '表现较好', hint: `转化 ${conversion}%，可继续投放` }
  if (conversion >= 40) return { label: '继续观察', hint: `转化 ${conversion}%，可优化套餐` }
  return { label: '需要调整', hint: `转化 ${conversion}%，建议重写权益` }
}

const exportPromotionEffects = () => {
  const rows = [
    {
      类型: '汇总',
      活动名称: '全部活动',
      活动状态: '-',
      活动类型: '-',
      优惠力度: '-',
      带单数: promotionSummary.value.order_count,
      有效支付数: promotionSummary.value.paid_count,
      优惠金额: centsToYuan(promotionSummary.value.discount_amount).toFixed(2),
      活动实收: centsToYuan(promotionSummary.value.revenue_amount).toFixed(2),
      转化率: promotionSummary.value.order_count ? `${Math.round((promotionSummary.value.paid_count / promotionSummary.value.order_count) * 100)}%` : '0%',
      让利比例: `${marketingDiscountRate.value}%`,
      运营建议: '用于快速判断活动整体带单、让利和实收表现。'
    },
    ...promotions.value.map((item) => {
      const effect = promotionEffect(item)
      return {
        类型: '活动明细',
        活动名称: item.title || '-',
        活动状态: statusLabel(item.status),
        活动类型: item.type === 'discount' ? '折扣' : '满减',
        优惠力度: formatDiscount(item),
        带单数: effect.order_count,
        有效支付数: effect.paid_count,
        优惠金额: centsToYuan(effect.discount_amount).toFixed(2),
        活动实收: centsToYuan(effect.revenue_amount).toFixed(2),
        转化率: effect.order_count ? `${Math.round((effect.paid_count / effect.order_count) * 100)}%` : '0%',
        让利比例: promotionRoi(item).hint,
        ROI判断: promotionRoi(item).label,
        运营建议: promotionEffectAdvice(item)
      }
    })
  ]
  exportRowsToXlsx(`优惠活动效果_${new Date().toISOString().slice(0, 10)}.xlsx`, '活动效果', rows)
}

const formatRate = (value) => {
  const rate = Number(value || 0)
  if (!rate) return '折扣待设置'
  return `${rate / 10} 折`
}

const formatDiscount = (row) => {
  if (row.type === 'discount') {
    const threshold = row.threshold ? `满 ${formatMoney(row.threshold)} 元` : '无门槛'
    return `${threshold} 享 ${formatRate(row.discount_rate)}`
  }
  if (!row.discount) return '到店咨询'
  if (!row.threshold) return `立减 ${formatMoney(row.discount)} 元`
  return `满 ${formatMoney(row.threshold)} 减 ${formatMoney(row.discount)} 元`
}

const formatDate = (value) => {
  if (!value) return '不限'
  return String(value).slice(0, 10)
}

const statusLabel = (status) => ({
  draft: '草稿',
  published: '发布中',
  inactive: '已停用'
}[status] || status)

const statusType = (status) => ({
  draft: 'info',
  published: 'success',
  inactive: 'warning'
}[status] || 'info')

const buildPromotionChecks = (data) => {
  const titleOk = Boolean(String(data.title || '').trim())
  const descriptionOk = String(data.description || '').trim().length >= 8
  const dateOk = Boolean(data.valid_from && data.valid_to)
  const dateOrderOk = !dateOk || String(data.valid_from) <= String(data.valid_to)
  const threshold = Number(data.threshold || 0)
  const discount = Number(data.discount || 0)
  const rate = Number(data.discount_rate || 0)
  const amountOk = data.type === 'amount' ? discount > 0 : rate > 0 && rate < 100
  const profitOk = data.type === 'amount' ? threshold === 0 || discount < threshold : rate >= 50
  const items = [
    { ok: titleOk, text: titleOk ? '活动名称已填写。' : '活动名称不能为空，顾客扫码页需要看到清晰标题。' },
    { ok: descriptionOk, text: descriptionOk ? '活动说明较完整。' : '建议补充活动说明，写清适用规则和使用方式。' },
    { ok: dateOk && dateOrderOk, text: dateOk ? (dateOrderOk ? '有效期设置正常。' : '结束日期不能早于开始日期。') : '建议设置开始和结束日期，避免活动长期失控。' },
    { ok: amountOk, text: amountOk ? '优惠力度已设置。' : '优惠金额或折扣不能为 0，否则顾客感知不明显。' },
    { ok: profitOk, text: profitOk ? '优惠力度未发现明显亏损风险。' : '优惠力度偏大，建议确认毛利后再发布。' }
  ]
  return {
    items,
    blockers: items.filter((item) => !item.ok)
  }
}

const resetForm = () => {
  editingId.value = null
  Object.assign(form, {
    store_id: null,
    title: '',
    description: '',
    type: 'amount',
    threshold_yuan: 0,
    discount_yuan: 0,
    discount_rate: 85,
    status: 'draft',
    valid_from: '',
    valid_to: ''
  })
}

const loadPromotions = async () => {
  loading.value = true
  try {
    const res = await fetchMerchantPromotions({ page: page.value, page_size: pageSize })
    promotions.value = res.data.list || res.data.items || []
    total.value = res.data.total || 0
    showAiDraftTip.value = route.query.ai_draft === '1' && Boolean(latestAiDraft.value)
    openRouteEditPromotion()
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '优惠活动加载失败')
  } finally {
    loading.value = false
  }
}

const openRouteEditPromotion = () => {
  const editId = Number(route.query.edit_id || 0)
  if (!editId || dialogVisible.value) return
  const target = promotions.value.find((item) => Number(item.id) === editId)
  if (!target) return
  aiReviewNote.value = route.query.ai_review === '1'
    ? (sessionStorage.getItem('ai_promotion_review_note') || '')
    : ''
  openEdit(target)
  router.replace('/merchant/promotions')
}

const loadStores = async () => {
  try {
    const res = await fetchMerchantStores({ page: 1, page_size: 100 })
    stores.value = res.data.list || res.data.items || []
  } catch {
    stores.value = []
  }
}

const loadPromotionOrders = async () => {
  try {
    const res = await fetchMerchantOrders({ page: 1, page_size: 500 })
    orders.value = res.data?.list || res.data?.items || []
  } catch {
    orders.value = []
  }
}

const openCreate = () => {
  resetForm()
  dialogVisible.value = true
}

const openLatestDraft = () => {
  if (!latestAiDraft.value) {
    ElMessage.warning('暂未找到 AI 活动草稿')
    return
  }
  openEdit(latestAiDraft.value)
}

const openEdit = (row) => {
  editingId.value = row.id
  Object.assign(form, {
    store_id: row.store_id || null,
    title: row.title || '',
    description: row.description || '',
    type: row.type || 'amount',
    threshold_yuan: centsToYuan(row.threshold),
    discount_yuan: centsToYuan(row.discount),
    discount_rate: row.discount_rate || 85,
    status: row.status || 'draft',
    valid_from: formatDate(row.valid_from) === '不限' ? '' : formatDate(row.valid_from),
    valid_to: formatDate(row.valid_to) === '不限' ? '' : formatDate(row.valid_to)
  })
  dialogVisible.value = true
}

const promotionRowClass = ({ row }) => {
  if (showAiDraftTip.value && latestAiDraft.value?.id === row.id) return 'ai-draft-row'
  return ''
}

const buildPayload = (override = {}) => ({
  store_id: form.store_id || null,
  title: form.title,
  description: form.description,
  type: form.type,
  threshold: yuanToCents(form.threshold_yuan),
  discount: form.type === 'amount' ? yuanToCents(form.discount_yuan) : 0,
  discount_rate: form.type === 'discount' ? Number(form.discount_rate || 0) : 0,
  status: form.status,
  valid_from: form.valid_from || '',
  valid_to: form.valid_to || '',
  ...override
})

const confirmPublishChecks = async (checks) => {
  if (!checks.blockers.length) return true
  const tips = checks.blockers.map((item) => `• ${item.text}`).join('\n')
  await ElMessageBox.confirm(
    `当前活动仍有以下风险：\n\n${tips}\n\n建议先修正后再发布。是否继续保存为草稿？`,
    '发布前检查未通过',
    {
      confirmButtonText: '保存为草稿',
      cancelButtonText: '继续编辑',
      type: 'warning'
    }
  )
  return false
}

const savePromotion = async () => {
  saving.value = true
  try {
    const payload = buildPayload()
    if (payload.status === 'published') {
      const canPublish = await confirmPublishChecks(publishChecks.value)
      if (!canPublish) {
        payload.status = 'draft'
        form.status = 'draft'
      }
    }
    if (editingId.value) {
      await updateMerchantPromotion(editingId.value, payload)
    } else {
      await createMerchantPromotion(payload)
    }
    ElMessage.success('优惠活动已保存')
    dialogVisible.value = false
    await loadPromotions()
    await loadPromotionOrders()
    if (showAiDraftTip.value) {
      router.replace('/merchant/promotions')
      showAiDraftTip.value = false
    }
  } catch (err) {
    if (err !== 'cancel' && err !== 'close') {
      ElMessage.error(err.response?.data?.message || '保存失败')
    }
  } finally {
    saving.value = false
  }
}

const buildPayloadFromRow = (row, override = {}) => ({
  store_id: row.store_id || null,
  title: row.title,
  description: row.description,
  type: row.type || 'amount',
  threshold: row.threshold || 0,
  discount: row.discount || 0,
  discount_rate: row.discount_rate || 0,
  status: row.status || 'draft',
  valid_from: formatDate(row.valid_from) === '不限' ? '' : formatDate(row.valid_from),
  valid_to: formatDate(row.valid_to) === '不限' ? '' : formatDate(row.valid_to),
  ...override
})

const changeStatus = async (row, status) => {
  try {
    let nextStatus = status
    if (status === 'published') {
      const checks = buildPromotionChecks(row)
      const canPublish = await confirmPublishChecks(checks)
      if (!canPublish) nextStatus = 'draft'
    }
    await updateMerchantPromotion(row.id, buildPayloadFromRow(row, { status: nextStatus }))
    ElMessage.success(nextStatus === 'published' ? '活动已发布' : (status === 'published' ? '已保留为草稿' : '活动已停用'))
    await loadPromotions()
    await loadPromotionOrders()
  } catch (err) {
    if (err !== 'cancel' && err !== 'close') {
      ElMessage.error(err.response?.data?.message || '操作失败')
    }
  }
}

const removePromotion = async (row) => {
  try {
    await ElMessageBox.confirm(`确认删除“${row.title}”吗？删除后不会在列表中恢复。`, '删除优惠活动', {
      confirmButtonText: '确认删除',
      cancelButtonText: '取消',
      type: 'warning'
    })
    await deleteMerchantPromotion(row.id)
    ElMessage.success('活动已删除')
    await loadPromotions()
  } catch (err) {
    if (err !== 'cancel' && err !== 'close') {
      ElMessage.error(err.response?.data?.message || '删除失败')
    }
  }
}

const handlePageChange = (nextPage) => {
  page.value = nextPage
  loadPromotions()
}

const reviewPromotionWithAI = (row) => {
  const effect = promotionEffect(row)
  router.push({
    path: '/merchant/ai',
    query: {
      scenario: 'promotion_review',
      promotion_id: row.id,
      promotion: row.title || '',
      orders: effect.order_count,
      paid: effect.paid_count,
      discount: centsToYuan(effect.discount_amount).toFixed(2),
      revenue: centsToYuan(effect.revenue_amount).toFixed(2)
    }
  })
}

onMounted(() => {
  loadPromotions()
  loadStores()
  loadPromotionOrders()
})
</script>

<style scoped>
.promotion-page {
  display: grid;
  gap: 20px;
}

.metric-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(210px, 1fr));
  gap: 18px;
}

.effect-summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
}

.effect-summary > div {
  padding: 18px;
  border: 1px solid rgba(37, 99, 235, 0.12);
  border-radius: 20px;
  background:
    radial-gradient(circle at top right, rgba(37, 99, 235, 0.1), transparent 35%),
    #fff;
  box-shadow: 0 14px 34px rgba(15, 39, 71, 0.06);
}

.effect-summary span,
.effect-summary strong,
.effect-summary small {
  display: block;
}

.effect-summary span {
  color: #64748b;
  font-size: 12px;
  font-weight: 800;
}

.effect-summary strong {
  margin-top: 8px;
  color: #0f2747;
  font-size: 24px;
}

.effect-summary small {
  margin-top: 6px;
  color: #64748b;
}

.marketing-dashboard {
  display: grid;
  grid-template-columns: minmax(260px, 0.8fr) minmax(0, 1.6fr);
  gap: 14px;
  padding: 18px 20px;
  border: 1px solid #dbeafe;
  border-radius: 22px;
  background: #ffffff;
  box-shadow: 0 14px 36px rgba(15, 39, 71, 0.07);
}

.marketing-dashboard.good {
  border-color: #bbf7d0;
  background: linear-gradient(135deg, #ecfdf5, #ffffff);
}

.marketing-dashboard.warning {
  border-color: #fed7aa;
  background: linear-gradient(135deg, #fff7ed, #ffffff);
}

.marketing-dashboard.quiet {
  background: linear-gradient(135deg, #f8fbff, #ffffff);
}

.dashboard-main,
.dashboard-grid > div {
  padding: 14px;
  border: 1px solid rgba(203, 213, 225, 0.82);
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.78);
}

.dashboard-main span,
.dashboard-main strong,
.dashboard-main p,
.dashboard-grid span,
.dashboard-grid strong,
.dashboard-grid small {
  display: block;
}

.dashboard-main span,
.dashboard-grid span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.08em;
}

.dashboard-main strong {
  margin-top: 6px;
  color: #0f2747;
  font-size: 22px;
  line-height: 1.25;
}

.dashboard-main p {
  margin: 8px 0 0;
  color: #64748b;
  line-height: 1.65;
}

.dashboard-grid {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 10px;
}

.dashboard-grid strong {
  margin-top: 7px;
  color: #0f2747;
  font-size: 19px;
  line-height: 1.25;
}

.dashboard-grid small {
  margin-top: 6px;
  color: #64748b;
  line-height: 1.45;
}

.dashboard-grid .good {
  border-color: #bbf7d0;
  background: #ecfdf5;
}

.dashboard-grid .risk {
  border-color: #fed7aa;
  background: #fff7ed;
}

.action-board {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
}

.action-card-lite {
  min-height: 108px;
  padding: 15px;
  text-align: left;
  border: 1px solid #dbeafe;
  border-radius: 18px;
  background: #ffffff;
  box-shadow: 0 12px 30px rgba(15, 39, 71, 0.06);
  cursor: pointer;
  transition: 0.2s ease;
}

.action-card-lite:hover {
  transform: translateY(-2px);
  border-color: #93c5fd;
  box-shadow: 0 18px 38px rgba(15, 39, 71, 0.1);
}

.action-card-lite span,
.action-card-lite strong,
.action-card-lite small {
  display: block;
}

.action-card-lite span {
  color: #64748b;
  font-size: 12px;
  font-weight: 800;
}

.action-card-lite strong {
  margin: 7px 0 5px;
  color: #0f2747;
  font-size: 18px;
  line-height: 1.25;
}

.action-card-lite small {
  color: #64748b;
  line-height: 1.45;
}

.action-card-lite.success {
  border-color: #bbf7d0;
  background: #ecfdf5;
}

.action-card-lite.warning {
  border-color: #fed7aa;
  background: #fff7ed;
}

.ai-draft-tip {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
  padding: 18px 20px;
  border: 1px solid rgba(37, 99, 235, 0.18);
  border-radius: 22px;
  background:
    radial-gradient(circle at top left, rgba(37, 99, 235, 0.14), transparent 32%),
    linear-gradient(135deg, #f8fbff 0%, #eef7ff 100%);
  box-shadow: 0 18px 45px rgba(15, 39, 71, 0.08);
}

.ai-draft-tip span,
.ai-draft-tip strong,
.ai-draft-tip p {
  display: block;
}

.ai-draft-tip span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.ai-draft-tip strong {
  margin-top: 6px;
  color: #0f2747;
  font-size: 18px;
}

.ai-draft-tip p {
  margin: 6px 0 0;
  color: #64748b;
}

.ai-review-note {
  display: grid;
  gap: 10px;
  margin-bottom: 16px;
  padding: 14px;
  border: 1px solid rgba(37, 99, 235, 0.18);
  border-radius: 16px;
  background:
    radial-gradient(circle at top left, rgba(37, 99, 235, 0.12), transparent 35%),
    #f8fbff;
}

.ai-review-note span,
.ai-review-note strong {
  display: block;
}

.ai-review-note span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.08em;
}

.ai-review-note strong {
  margin-top: 4px;
  color: #0f2747;
}

.ai-review-note pre {
  overflow: auto;
  max-height: 180px;
  margin: 0;
  padding: 12px;
  border-radius: 12px;
  background: #0f172a;
  color: #e0f2fe;
  font-family: inherit;
  line-height: 1.65;
  white-space: pre-wrap;
}

:deep(.ai-draft-row) {
  --el-table-tr-bg-color: #eff6ff;
}

:deep(.ai-draft-row td:first-child) {
  box-shadow: inset 4px 0 0 #2563eb;
}

.action-row {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.effect-cell {
  display: grid;
  gap: 4px;
}

.effect-cell strong {
  color: #0f2747;
}

.effect-cell span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 700;
}

.effect-cell small {
  color: #64748b;
  line-height: 1.45;
}

.roi-cell {
  display: grid;
  gap: 4px;
}

.roi-cell strong {
  color: #0f2747;
}

.roi-cell span {
  color: #64748b;
  font-size: 12px;
  line-height: 1.45;
}

.pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

.type-switch {
  width: 100%;
}

.discount-box {
  margin-bottom: 18px;
  padding: 16px 14px 6px;
  border: 1px solid #bfdbfe;
  border-radius: 16px;
  background:
    radial-gradient(circle at 100% 0%, rgba(37, 99, 235, 0.1), transparent 32%),
    #f8fbff;
}

.publish-check {
  margin-top: 4px;
  padding: 14px;
  border: 1px solid rgba(37, 99, 235, 0.14);
  border-radius: 16px;
  background: #f8fbff;
}

.check-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 10px;
}

.check-head span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.08em;
}

.check-head strong {
  color: #0f2747;
}

.check-list {
  display: grid;
  gap: 8px;
}

.check-item {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 9px 10px;
  border-radius: 12px;
}

.check-item span {
  flex: 0 0 auto;
  padding: 2px 7px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 800;
}

.check-item p {
  margin: 0;
  color: #475569;
  line-height: 1.5;
}

.check-item.ok {
  background: rgba(34, 197, 94, 0.08);
}

.check-item.ok span {
  color: #15803d;
  background: rgba(34, 197, 94, 0.14);
}

.check-item.risk {
  background: rgba(245, 158, 11, 0.1);
}

.check-item.risk span {
  color: #b45309;
  background: rgba(245, 158, 11, 0.16);
}

.full-input {
  width: 100%;
}

.form-tip {
  margin-top: 8px;
  color: #64748b;
  font-size: 12px;
  line-height: 1.5;
}

@media (max-width: 760px) {
  .marketing-dashboard {
    grid-template-columns: 1fr;
  }

  .dashboard-grid,
  .action-board,
  .effect-summary {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .ai-draft-tip {
    align-items: flex-start;
    flex-direction: column;
  }
}

@media (max-width: 520px) {
  .dashboard-grid,
  .action-board,
  .effect-summary {
    grid-template-columns: 1fr;
  }
}
</style>
