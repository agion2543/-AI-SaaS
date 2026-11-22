<template>
  <div class="promotion-calculator-page">
    <PageHero
      eyebrow="PROMOTION PLANNER"
      title="单场活动测算"
      description="用每笔促销的成交价和直接成本，快速估算活动贡献与覆盖一次性费用所需的成交笔数。"
      tone="brand"
    />

    <section class="estimate-notice" aria-label="估算说明">
      <span class="notice-mark">估</span>
      <div>
        <strong>这是估算，不是实际利润报表</strong>
        <p>仅根据你在本页填写的假设计算；不读取订单、账本或门店数据，也不会保存这些输入。固定门店费用（如租金、人工）和税费未计入，结果不代表净利润。</p>
      </div>
    </section>

    <div class="calculator-grid">
      <DataPanel
        title="填写活动假设"
        description="所有金额均按人民币元填写。请按一笔促销成交的口径估算直接成本。"
      >
        <div class="form-section">
          <div class="section-heading">
            <span>01</span>
            <div>
              <strong>每笔促销成交</strong>
              <small>按一笔促销成交对应一份商品/服务填写</small>
            </div>
          </div>
          <div class="field-grid">
            <label class="calc-field">
              <span>促销成交价 <b>必填</b></span>
              <el-input-number
                v-model="form.promotionSellingPriceYuan"
                :min="0"
                :precision="2"
                :step="1"
                :controls="false"
                placeholder="例如 59.90"
                aria-label="促销成交价，元"
              />
              <small>顾客实际支付的促销单价</small>
            </label>
            <label class="calc-field">
              <span>商品成本 <b>必填</b></span>
              <el-input-number
                v-model="form.productCostYuan"
                :min="0"
                :precision="2"
                :step="1"
                :controls="false"
                placeholder="例如 22.00"
                aria-label="每笔商品成本，元"
              />
              <small>每笔成交对应的商品/服务成本</small>
            </label>
            <label class="calc-field">
              <span>赠品成本 <b>必填</b></span>
              <el-input-number
                v-model="form.giftCostYuan"
                :min="0"
                :precision="2"
                :step="1"
                :controls="false"
                placeholder="没有赠品填 0"
                aria-label="每笔赠品成本，元"
              />
              <small>没有赠品时填写 0</small>
            </label>
            <label class="calc-field">
              <span>包装成本 <b>必填</b></span>
              <el-input-number
                v-model="form.packagingCostYuan"
                :min="0"
                :precision="2"
                :step="1"
                :controls="false"
                placeholder="没有额外包装填 0"
                aria-label="每笔包装成本，元"
              />
              <small>每笔成交额外产生的包装费用</small>
            </label>
            <label class="calc-field field-wide">
              <span>单笔渠道费用 <b>必填</b></span>
              <el-input-number
                v-model="form.perUnitChannelFeeYuan"
                :min="0"
                :precision="2"
                :step="1"
                :controls="false"
                placeholder="没有渠道费用填 0"
                aria-label="单笔渠道费用，元"
              />
              <small>佣金、支付/平台按单收取的可变费用；若按比例收费，请按预计金额换算为每笔费用。</small>
            </label>
          </div>
        </div>

        <div class="form-section campaign-section">
          <div class="section-heading">
            <span>02</span>
            <div>
              <strong>活动计划</strong>
              <small>用于估算整体活动贡献</small>
            </div>
          </div>
          <div class="field-grid">
            <label class="calc-field">
              <span>预计有效成交笔数 <b>必填</b></span>
              <el-input-number
                v-model="form.plannedValidQuantity"
                :min="0"
                :precision="0"
                :step="10"
                :controls="false"
                placeholder="例如 100"
                aria-label="预计有效成交笔数"
              />
              <small>填写预计促销成交笔数，允许填写 0</small>
            </label>
            <label class="calc-field">
              <span>一次性活动费用 <b>必填</b></span>
              <el-input-number
                v-model="form.oneOffCampaignCostYuan"
                :min="0"
                :precision="2"
                :step="100"
                :controls="false"
                placeholder="没有一次性费用填 0"
                aria-label="一次性活动费用，元"
              />
              <small>例如一次性投放或活动布置费用；没有则填 0</small>
            </label>
          </div>
        </div>

        <div class="form-actions">
          <span>输入不会保存；离开或刷新页面后需要重新填写。</span>
          <el-button plain @click="clearForm">清空重算</el-button>
        </div>
      </DataPanel>

      <DataPanel
        title="测算结果"
        description="结果只反映本次填写的直接成本与活动费用。"
        class="result-panel"
      >
        <div v-if="!isReady" class="empty-result">
          <div class="empty-icon">＋</div>
          <strong>填写全部 7 项估算后查看结果</strong>
          <p>无需提交或保存，输入后会在本页即时计算。</p>
        </div>
        <template v-else-if="result">
          <div class="result-highlight" :class="resultTone">
            <span>预计活动贡献（估算）</span>
            <strong>{{ formatCurrency(result.projectedContributionCents) }}</strong>
            <small>已扣除单笔直接成本与一次性活动费用；不是净利润。</small>
          </div>

          <div class="result-metrics">
            <article :class="result.contributionPerUnitCents >= 0 ? 'positive' : 'negative'">
              <span>单笔促销贡献</span>
              <strong>{{ formatCurrency(result.contributionPerUnitCents) }}</strong>
              <small>成交价减去四项单笔直接成本</small>
            </article>
            <article :class="result.breakEvenAchievable ? 'positive' : 'negative'">
              <span>覆盖一次性费用所需成交</span>
              <strong v-if="result.breakEvenAchievable">{{ formatQuantity(result.breakEvenQuantity) }}</strong>
              <strong v-else>当前无法达到</strong>
              <small v-if="result.breakEvenAchievable">按单笔贡献测算，向上取整</small>
              <small v-else>单笔贡献为 0 或负数，增加成交笔数也无法覆盖</small>
            </article>
          </div>

          <div class="formula-box">
            <strong>计算口径</strong>
            <p>单笔促销贡献 = 促销成交价 − 商品成本 − 赠品成本 − 包装成本 − 单笔渠道费用</p>
            <p>预计活动贡献 = 单笔促销贡献 × 预计有效成交笔数 − 一次性活动费用</p>
          </div>
        </template>
        <div v-else class="calculation-error" role="alert">{{ calculationError }}</div>
      </DataPanel>
    </div>

    <p class="bottom-note">提示：实际经营结果还会受退款、核销、折扣承担方式及其他成本影响；请勿将本估算替代财务对账或实际经营记录。</p>
  </div>
</template>

<script setup>
import { computed, reactive } from 'vue'
import PageHero from '../../components/design/PageHero.vue'
import DataPanel from '../../components/design/DataPanel.vue'
import { calculateMerchantPromotionScenario } from '../../utils/merchantPromotionScenario.js'

const emptyForm = () => ({
  promotionSellingPriceYuan: undefined,
  productCostYuan: undefined,
  giftCostYuan: undefined,
  packagingCostYuan: undefined,
  perUnitChannelFeeYuan: undefined,
  plannedValidQuantity: undefined,
  oneOffCampaignCostYuan: undefined
})
const form = reactive(emptyForm())
const fields = [
  'promotionSellingPriceYuan',
  'productCostYuan',
  'giftCostYuan',
  'packagingCostYuan',
  'perUnitChannelFeeYuan',
  'plannedValidQuantity',
  'oneOffCampaignCostYuan'
]
const isReady = computed(() => fields.every((key) => form[key] !== undefined && form[key] !== null && form[key] !== ''))
const calculation = computed(() => {
  if (!isReady.value) return { result: null, error: '' }
  try {
    return { result: calculateMerchantPromotionScenario({ ...form }), error: '' }
  } catch (error) {
    return { result: null, error: error.message || '无法完成测算，请检查输入并调小数值。' }
  }
})
const result = computed(() => calculation.value.result)
const calculationError = computed(() => calculation.value.error)
const resultTone = computed(() => result.value?.projectedContributionCents >= 0 ? 'positive' : 'negative')

function formatCurrency(cents) {
  const amount = cents / 100
  return `¥${new Intl.NumberFormat('zh-CN', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2
  }).format(amount)}`
}

function formatQuantity(quantity) {
  return `${new Intl.NumberFormat('zh-CN').format(quantity)} 笔`
}

function clearForm() {
  Object.assign(form, emptyForm())
}
</script>

<style scoped>
.promotion-calculator-page {
  display: grid;
  gap: 18px;
}
.estimate-notice {
  display: grid;
  grid-template-columns: 38px minmax(0, 1fr);
  gap: 12px;
  align-items: start;
  padding: 16px 18px;
  border: 1px solid #bfdbfe;
  border-radius: 18px;
  background: linear-gradient(100deg, #eff6ff, #f8fbff 70%);
}
.notice-mark {
  display: grid;
  width: 36px;
  height: 36px;
  place-items: center;
  border-radius: 12px;
  color: #fff;
  background: #2563eb;
  font-size: 14px;
  font-weight: 900;
}
.estimate-notice strong {
  display: block;
  color: #153b71;
  font-size: 14px;
}
.estimate-notice p {
  margin: 5px 0 0;
  color: #526981;
  font-size: 13px;
  line-height: 1.65;
}
.calculator-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.16fr) minmax(340px, 0.84fr);
  gap: 18px;
  align-items: start;
}
.form-section + .form-section {
  margin-top: 22px;
  padding-top: 22px;
  border-top: 1px solid #e7edf5;
}
.section-heading {
  display: flex;
  align-items: center;
  gap: 11px;
  margin-bottom: 16px;
}
.section-heading > span {
  display: grid;
  width: 34px;
  height: 34px;
  place-items: center;
  border: 1px solid #bfdbfe;
  border-radius: 11px;
  color: #2563eb;
  background: #eff6ff;
  font-size: 12px;
  font-weight: 900;
}
.section-heading strong,
.section-heading small {
  display: block;
}
.section-heading strong {
  color: #172b4d;
  font-size: 15px;
}
.section-heading small {
  margin-top: 3px;
  color: #7a8ba0;
  font-size: 12px;
}
.field-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px 14px;
}
.calc-field {
  display: grid;
  min-width: 0;
  gap: 7px;
  color: #26384d;
  font-size: 13px;
  font-weight: 750;
}
.calc-field > span {
  display: flex;
  align-items: center;
  gap: 7px;
}
.calc-field b {
  color: #2563eb;
  font-size: 10px;
  font-weight: 800;
}
.calc-field :deep(.el-input-number) {
  width: 100%;
}
.calc-field :deep(.el-input__wrapper) {
  min-height: 42px;
  border-radius: 11px;
  box-shadow: 0 0 0 1px #d7e1ed inset;
}
.calc-field :deep(.el-input__wrapper:hover),
.calc-field :deep(.el-input__wrapper.is-focus) {
  box-shadow: 0 0 0 1px #3b82f6 inset;
}
.calc-field small {
  min-height: 16px;
  color: #8795a7;
  font-size: 11px;
  font-weight: 450;
  line-height: 1.45;
}
.field-wide {
  grid-column: 1 / -1;
}
.form-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: 20px;
  padding-top: 16px;
  border-top: 1px solid #e7edf5;
}
.form-actions > span {
  color: #8795a7;
  font-size: 11px;
  line-height: 1.5;
}
.result-panel {
  position: sticky;
  top: 18px;
}
.empty-result {
  display: grid;
  min-height: 300px;
  align-content: center;
  justify-items: center;
  padding: 24px;
  text-align: center;
}
.empty-icon {
  display: grid;
  width: 54px;
  height: 54px;
  place-items: center;
  margin-bottom: 14px;
  border: 1px dashed #9bb9df;
  border-radius: 18px;
  color: #2563eb;
  background: #f2f7ff;
  font-size: 26px;
  font-weight: 400;
}
.empty-result strong {
  color: #203651;
  font-size: 14px;
}
.empty-result p {
  margin: 7px 0 0;
  color: #8190a2;
  font-size: 12px;
  line-height: 1.6;
}
.result-highlight {
  padding: 18px;
  border: 1px solid;
  border-radius: 17px;
}
.result-highlight.positive {
  border-color: #bbf7d0;
  background: linear-gradient(135deg, #f0fdf4, #f7fff9);
}
.result-highlight.negative {
  border-color: #fecaca;
  background: linear-gradient(135deg, #fff1f2, #fffafa);
}
.result-highlight span,
.result-highlight small {
  display: block;
}
.result-highlight span {
  color: #50647d;
  font-size: 12px;
  font-weight: 700;
}
.result-highlight strong {
  display: block;
  margin: 8px 0 6px;
  color: #166534;
  font-size: clamp(27px, 3vw, 36px);
  letter-spacing: -0.05em;
  line-height: 1.12;
}
.result-highlight.negative strong {
  color: #b42318;
}
.result-highlight small {
  color: #73849a;
  font-size: 11px;
  line-height: 1.5;
}
.result-metrics {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 11px;
  margin-top: 12px;
}
.result-metrics article {
  min-width: 0;
  padding: 14px;
  border: 1px solid #e2eaf3;
  border-radius: 15px;
  background: #fff;
}
.result-metrics article.positive {
  border-color: #d1fae5;
}
.result-metrics article.negative {
  border-color: #fee2e2;
}
.result-metrics span,
.result-metrics strong,
.result-metrics small {
  display: block;
}
.result-metrics span {
  color: #74849a;
  font-size: 11px;
  line-height: 1.45;
}
.result-metrics strong {
  margin: 8px 0 5px;
  color: #17643b;
  font-size: clamp(17px, 1.7vw, 22px);
  line-height: 1.25;
}
.result-metrics .negative strong {
  color: #b42318;
}
.result-metrics small {
  color: #8795a7;
  font-size: 10px;
  line-height: 1.45;
}
.formula-box {
  margin-top: 14px;
  padding: 14px;
  border-radius: 14px;
  background: #f6f8fb;
}
.formula-box strong {
  display: block;
  margin-bottom: 8px;
  color: #364b64;
  font-size: 12px;
}
.formula-box p {
  margin: 5px 0 0;
  color: #718198;
  font-size: 11px;
  line-height: 1.6;
}
.calculation-error {
  padding: 16px;
  border: 1px solid #fecaca;
  border-radius: 14px;
  color: #b42318;
  background: #fff7f7;
  font-size: 13px;
  line-height: 1.6;
}
.bottom-note {
  margin: 0;
  padding: 0 4px;
  color: #78879a;
  font-size: 11px;
  line-height: 1.6;
}
@media (max-width: 1100px) {
  .calculator-grid {
    grid-template-columns: minmax(0, 1fr);
  }
  .result-panel {
    position: static;
  }
}
@media (max-width: 650px) {
  .field-grid,
  .result-metrics {
    grid-template-columns: minmax(0, 1fr);
  }
  .field-wide {
    grid-column: auto;
  }
  .form-actions {
    align-items: flex-start;
  }
  .form-actions > span {
    max-width: 60%;
  }
}
</style>
