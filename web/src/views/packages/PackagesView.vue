<template>
  <div class="plans-stack">
    <section class="page-card hero-card">
      <div>
        <div class="eyebrow">SAAS PLANS</div>
        <h2 class="page-title">平台套餐</h2>
        <p class="muted">这里配置平台卖给商家的 SaaS 订阅服务。当前建议先按“月付服务版 / 年付服务版 / 技术支持版”表达，功能默认完整可用，后续再做权益限制。</p>
      </div>
      <el-button type="primary" @click="openDialog()">新增平台套餐</el-button>
    </section>

    <section class="metric-grid">
      <article>
        <span>套餐数量</span>
        <strong>{{ plans.length }}</strong>
        <p>商家订阅页会按排序展示。</p>
      </article>
      <article>
        <span>推荐套餐</span>
        <strong>{{ recommendedPlan?.name || '-' }}</strong>
        <p>{{ recommendedPlan ? `年化价格 ${formatYuan(recommendedPlan.price_cents)} 元` : '建议配置一个年付服务版。' }}</p>
      </article>
      <article>
        <span>最低月成本</span>
        <strong>{{ minMonthlyCost }}</strong>
        <p>方便商家快速判断试用门槛。</p>
      </article>
      <article>
        <span>当前策略</span>
        <strong>完整功能</strong>
        <p>暂不做复杂功能限制，先跑通收费闭环。</p>
      </article>
    </section>

    <section class="plan-preview-grid">
      <article v-for="plan in sortedPlans" :key="plan.id" class="plan-preview-card" :class="{ recommended: isRecommended(plan) }">
        <div class="plan-preview-head">
          <div>
            <span>{{ planCategory(plan) }}</span>
            <strong>{{ displayPlanName(plan) }}</strong>
          </div>
          <el-tag :type="isRecommended(plan) ? 'success' : 'info'">{{ isRecommended(plan) ? '推荐' : serviceCycle(plan) }}</el-tag>
        </div>
        <div class="preview-price">¥{{ formatYuan(plan.price_cents) }}</div>
        <p>{{ planPitch(plan) }}</p>
        <div class="feature-list">
          <span v-for="feature in planFeatures(plan)" :key="feature">{{ feature }}</span>
        </div>
        <el-button plain type="primary" @click="openDialog(plan)">编辑套餐</el-button>
      </article>
    </section>

    <section class="page-card module-note">
      <div><strong>现阶段策略</strong><span>月付和年付都开放完整功能，避免权限体系未完善时影响商家体验。</span></div>
      <div><strong>销售表达</strong><span>月付适合试用，年付适合稳定门店，技术支持版可用于人工部署、培训或定制服务。</span></div>
      <div><strong>后续权益版</strong><span>若要做基础版/高级版，需要补套餐权益表、菜单权限、接口权限和降级策略。</span></div>
    </section>

    <section class="page-card table-card">
      <div class="card-heading">
        <div>
          <h3>套餐配置表</h3>
          <p class="muted">这里仍是最终配置入口，修改价格和周期会影响商家订阅付款金额。</p>
        </div>
      </div>
      <el-table :data="plans">
        <el-table-column label="套餐名称" min-width="180">
          <template #default="{ row }">
            <strong>{{ displayPlanName(row) }}</strong>
            <div class="table-sub">{{ row.name }}</div>
          </template>
        </el-table-column>
        <el-table-column label="定位" min-width="180">
          <template #default="{ row }">{{ planCategory(row) }}</template>
        </el-table-column>
        <el-table-column label="价格" width="140">
          <template #default="{ row }">¥{{ formatYuan(row.price_cents) }}</template>
        </el-table-column>
        <el-table-column label="服务周期" width="140">
          <template #default="{ row }">{{ serviceCycle(row) }}</template>
        </el-table-column>
        <el-table-column label="适用场景" min-width="240">
          <template #default="{ row }">{{ planPitch(row) }}</template>
        </el-table-column>
        <el-table-column prop="sort" label="排序" width="100" />
        <el-table-column prop="created_at" label="创建时间" min-width="170" />
        <el-table-column label="操作" width="120" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="openDialog(row)">编辑</el-button>
          </template>
        </el-table-column>
      </el-table>
    </section>

    <el-dialog v-model="dialogVisible" :title="form.id ? '编辑平台套餐' : '新增平台套餐'" width="520px">
      <el-form label-position="top">
        <el-form-item label="套餐名称">
          <el-input v-model="form.name" placeholder="例如：月付、年付、技术支持版" />
        </el-form-item>
        <el-row :gutter="14">
          <el-col :span="12">
            <el-form-item label="价格（元）">
              <el-input-number v-model="form.price_yuan" :min="0" :precision="2" class="full-input" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="服务周期（天）">
              <el-input-number v-model="form.duration_days" :min="1" :max="3650" class="full-input" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="排序">
          <el-input-number v-model="form.sort" :min="0" :max="9999" class="full-input" />
        </el-form-item>
        <el-alert
          type="info"
          show-icon
          :closable="false"
          title="当前套餐只控制价格和服务周期"
          description="功能权益暂时默认完整可用。后续如果要做基础版/高级版，需要再增加套餐权益字段和权限控制。"
        />
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { createMerchantPlan, fetchMerchantPlans, updateMerchantPlan } from '../../api/modules'

const plans = ref([])
const dialogVisible = ref(false)
const form = reactive({
  id: 0,
  name: '',
  price_yuan: 0,
  duration_days: 30,
  sort: 0
})

const load = async () => {
  const res = await fetchMerchantPlans()
  plans.value = res.data || []
}

const sortedPlans = computed(() => [...plans.value].sort((a, b) => Number(a.sort || 0) - Number(b.sort || 0) || Number(a.price_cents || 0) - Number(b.price_cents || 0)))
const recommendedPlan = computed(() => sortedPlans.value.find((item) => Number(item.duration_days || 0) >= 365) || sortedPlans.value[0])
const minMonthlyCost = computed(() => {
  if (!plans.value.length) return '-'
  const costs = plans.value
    .filter((item) => Number(item.duration_days || 0) > 0)
    .map((item) => Number(item.price_cents || 0) / Number(item.duration_days || 1) * 30)
  if (!costs.length) return '-'
  return `¥${formatYuan(Math.min(...costs))}`
})
const formatYuan = (value) => (Number(value || 0) / 100).toFixed(2)
const yuanToFen = (value) => Math.round(Number(value || 0) * 100)
const isRecommended = (plan) => recommendedPlan.value?.id === plan.id
const displayPlanName = (plan) => {
  const name = String(plan?.name || '')
  if (name.includes('技术') || Number(plan?.duration_days || 0) === 0) return '技术支持版'
  if (Number(plan?.duration_days || 0) >= 365 || name.includes('年')) return '年付服务版'
  if (Number(plan?.duration_days || 0) >= 30 || name.includes('月')) return '月付服务版'
  return name || '订阅服务版'
}
const serviceCycle = (plan) => Number(plan?.duration_days || 0) > 0 ? `${plan.duration_days} 天` : '人工服务'
const planCategory = (plan) => {
  const name = displayPlanName(plan)
  if (name.includes('年')) return '稳定经营门店'
  if (name.includes('技术')) return '部署培训/定制协助'
  return '试用起步门店'
}
const planPitch = (plan) => {
  const name = displayPlanName(plan)
  if (name.includes('年')) return '适合已经稳定使用的商家，按年续费更省心，减少频繁付款。'
  if (name.includes('技术')) return '适合需要平台协助部署、培训、数据迁移或定制配置的商家。'
  return '适合新商家先低成本试用，跑通扫码点单、订单管理和财务对账。'
}
const planFeatures = (plan) => {
  const name = displayPlanName(plan)
  if (name.includes('技术')) return ['部署协助', '配置指导', '使用培训', '上线检查']
  return ['扫码点单', '订单管理', '商品管理', '财务对账', '优惠活动', 'AI 建议']
}

const openDialog = (row) => {
  Object.assign(form, {
    id: row?.id || 0,
    name: row?.name || '',
    price_yuan: Number(row?.price_cents || 0) / 100,
    duration_days: row?.duration_days || 30,
    sort: row?.sort || 0
  })
  dialogVisible.value = true
}

const submit = async () => {
  const payload = {
    name: form.name.trim(),
    price_cents: yuanToFen(form.price_yuan),
    duration_days: form.duration_days,
    sort: form.sort
  }
  if (form.id) {
    await updateMerchantPlan(form.id, payload)
  } else {
    await createMerchantPlan(payload)
  }
  ElMessage.success('平台套餐已保存')
  dialogVisible.value = false
  load()
}

onMounted(load)
</script>

<style scoped>
.plans-stack {
  display: grid;
  gap: 18px;
}

.hero-card {
  display: flex;
  justify-content: space-between;
  gap: 20px;
  align-items: flex-start;
  padding: 24px;
}

.eyebrow {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.16em;
}

.muted {
  color: #64748b;
  margin: 0;
  line-height: 1.7;
}

.metric-grid,
.plan-preview-grid,
.module-note {
  display: grid;
  gap: 14px;
}

.metric-grid {
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.metric-grid article {
  min-height: 112px;
  padding: 16px;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  background: #ffffff;
}

.metric-grid span,
.metric-grid strong,
.metric-grid p {
  display: block;
}

.metric-grid span {
  color: #64748b;
  font-size: 12px;
  font-weight: 800;
}

.metric-grid strong {
  margin-top: 8px;
  color: #0f172a;
  font-size: 24px;
}

.metric-grid p {
  margin: 8px 0 0;
  color: #64748b;
  line-height: 1.5;
}

.plan-preview-grid {
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
}

.plan-preview-card {
  display: grid;
  gap: 14px;
  min-height: 360px;
  padding: 20px;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  background: #ffffff;
}

.plan-preview-card.recommended {
  border-color: #86efac;
  background: linear-gradient(135deg, #f0fdf4, #ffffff);
  box-shadow: 0 16px 36px rgba(22, 163, 74, 0.12);
}

.plan-preview-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: flex-start;
}

.plan-preview-head span,
.plan-preview-head strong {
  display: block;
}

.plan-preview-head span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.08em;
}

.plan-preview-head strong {
  margin-top: 6px;
  color: #0f2747;
  font-size: 22px;
}

.preview-price {
  color: #16a34a;
  font-size: 36px;
  font-weight: 900;
}

.plan-preview-card p {
  margin: 0;
  color: #64748b;
  line-height: 1.65;
}

.feature-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-content: flex-start;
  min-height: 78px;
}

.feature-list span {
  padding: 6px 10px;
  color: #334155;
  border: 1px solid #dbeafe;
  border-radius: 999px;
  background: #f8fbff;
  font-size: 12px;
  font-weight: 700;
}

.module-note {
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  padding: 18px;
}

.module-note div {
  padding: 14px;
  border-radius: 8px;
  background: #f8fbff;
  border: 1px solid #e5edf9;
}

.module-note strong,
.module-note span {
  display: block;
}

.module-note span {
  margin-top: 6px;
  color: var(--muted);
  font-size: 13px;
}

.table-card {
  padding: 18px;
}

.card-heading {
  margin-bottom: 14px;
}

.card-heading h3 {
  margin: 0 0 4px;
}

.table-sub {
  margin-top: 4px;
  color: #64748b;
  font-size: 12px;
}

.full-input {
  width: 100%;
}

@media (max-width: 960px) {
  .hero-card {
    flex-direction: column;
  }

  .metric-grid {
    grid-template-columns: 1fr;
  }
}
</style>
