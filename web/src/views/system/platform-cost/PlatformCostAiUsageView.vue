<template>
  <div class="platform-cost-page">
    <PageHero
      eyebrow="AI USAGE AND BILLS"
      title="AI 用量与账单"
      description="用量统计、模型价格、供应商账单。估算成本来自调用记录 × 模型价格表（模型估算），与供应商实际账单严格分开；估算不是账单。"
      compact
    >
      <template #actions>
        <el-tag effect="light" type="warning">仅超管可见</el-tag>
        <el-button type="primary" :loading="aiStatsLoading || reconciliationLoading" @click="loadAiUsageStats(); loadModelPrices(); loadVendorBills()">刷新数据</el-button>
      </template>
    </PageHero>

    <el-skeleton v-if="aiStatsLoading && !aiStats" :rows="5" animated />

    <section v-else-if="aiStats" class="summary-grid">
      <article class="summary-card">
        <div class="card-label">调用次数（所选期间）</div>
        <strong>{{ aiStats.total_requests || 0 }}</strong>
        <p>成功 / 失败按商家、门店、模型、场景维度见下方统计表。</p>
      </article>
      <article class="summary-card">
        <div class="card-label">Token 总量（所选期间）</div>
        <strong>{{ aiStats.total_tokens || 0 }}</strong>
        <p>输入 {{ aiStats.total_prompt_tokens || 0 }} · 输出 {{ aiStats.total_completion_tokens || 0 }} · 缓存 {{ aiStats.total_cached_tokens || 0 }}</p>
      </article>
      <article class="summary-card warning">
        <div class="card-label">AI 估算成本（估算）</div>
        <strong>{{ money(aiStats.total_estimated_cost_cents) }}（估算）</strong>
        <p>调用记录上存的估算成本累计；不是供应商实际账单。</p>
      </article>
    </section>

    <DataPanel
      title="用量统计"
      description="调用记录 × 模型价格表的模型估算；估算不是账单。"
      eyebrow="AI USAGE"
    >

            <div class="period-toolbar">
              <el-date-picker
                v-model="aiDateRange"
                type="daterange"
                range-separator="至"
                start-placeholder="开始日期"
                end-placeholder="结束日期"
                format="YYYY-MM-DD"
                value-format="YYYY-MM-DD"
                :disabled-date="disableFutureDate"
                :clearable="false"
                unlink-panels
              />
              <el-button type="primary" plain :loading="aiStatsLoading" @click="loadAiUsageStats">查看期间</el-button>
              <span class="currency-hint">默认最近 30 天 · 最多 366 天 · 金额单位：人民币 CNY</span>
            </div>
            <el-alert v-if="aiStatsError" class="page-alert" type="error" show-icon :closable="false" :title="aiStatsError" />
            <el-skeleton v-else-if="aiStatsLoading && !aiStats" :rows="5" animated />
            <template v-else-if="aiStats">
              <div class="period-grid ai-stats-grid">
                <article class="period-stat"><span>调用次数</span><strong>{{ aiStats.total_requests || 0 }}</strong></article>
                <article class="period-stat"><span>输入 Token</span><strong>{{ aiStats.total_prompt_tokens || 0 }}</strong></article>
                <article class="period-stat"><span>输出 Token</span><strong>{{ aiStats.total_completion_tokens || 0 }}</strong></article>
                <article class="period-stat"><span>缓存 Token</span><strong>{{ aiStats.total_cached_tokens || 0 }}</strong></article>
                <article class="period-stat"><span>Token 总量</span><strong>{{ aiStats.total_tokens || 0 }}</strong></article>
                <article class="period-stat cost"><span>估算成本（估算）</span><strong>{{ money(aiStats.total_estimated_cost_cents) }}</strong></article>
              </div>
              <div class="plan-note">
                <strong>估算成本来源</strong>
                <span>{{ aiStats.estimated_cost_source || '调用记录上的估算成本累计（写入时按当时生效价或内置费率估算）。' }}</span>
                <span v-if="aiStats.unpriced_fallback_estimated_cost_cents">其中 {{ money(aiStats.unpriced_fallback_estimated_cost_cents) }} 无生效价，回落到调用记录上的估算成本。</span>
              </div>
              <div class="ai-dimension-toolbar">
                <el-radio-group v-model="aiStatsDimension" size="default">
                  <el-radio-button v-for="item in aiUsageDimensions" :key="item.value" :label="item.value">{{ item.label }}</el-radio-button>
                </el-radio-group>
                <span class="currency-hint">成本列均为估算成本（估算），不是供应商实际账单</span>
              </div>
              <el-table :data="aiStatsRows" row-key="rowKey" stripe class="expense-table">
                <el-table-column :label="aiDimensionLabel" min-width="180">
                  <template #default="{ row }">{{ row.displayName }}</template>
                </el-table-column>
                <el-table-column prop="request_count" label="调用次数" min-width="100" align="right" />
                <el-table-column label="成功 / 失败" min-width="120" align="center">
                  <template #default="{ row }">{{ row.success_count }} / {{ row.failed_count }}</template>
                </el-table-column>
                <el-table-column prop="prompt_tokens" label="输入 Token" min-width="120" align="right" />
                <el-table-column prop="completion_tokens" label="输出 Token" min-width="120" align="right" />
                <el-table-column prop="cached_tokens" label="缓存 Token" min-width="120" align="right" />
                <el-table-column prop="total_tokens" label="Token 总量" min-width="120" align="right" />
                <el-table-column label="估算成本（估算）" min-width="150" align="right">
                  <template #default="{ row }">{{ money(row.estimated_cost_cents) }}</template>
                </el-table-column>
              </el-table>
              <el-empty v-if="!aiStatsRows.length" description="本期暂无 AI 调用记录；这不代表成本为零" :image-size="72" />
              <div class="table-footnote">2026-11-28 前的调用无门店归属，记为“未指定门店”，不虚构归属。{{ aiStats.period_start }} 至 {{ aiStats.period_end }}。</div>
            </template>
              </DataPanel>

    <DataPanel
      title="模型价格表"
      description="模型输入 / 输出 / 缓存单价（元 / 百万 Token），币种固定 CNY。"
      eyebrow="MODEL PRICES"
    >

            <div class="ai-form-note">
              <strong>调价请新增一行</strong>
              <span>同一模型 + 供应商 + 生效时间重复提交会原地更新单价；历史调用的估算成本以写入时价冻结，改价不影响已记成本。</span>
            </div>
            <el-form class="ai-inline-form" label-position="top" @submit.prevent="submitModelPrice">
              <div class="form-row ai-price-form-row">
                <el-form-item label="模型" required>
                  <el-input v-model="priceForm.model" maxlength="120" placeholder="例如 gpt-4o-mini" />
                </el-form-item>
                <el-form-item label="供应商">
                  <el-input v-model="priceForm.provider" maxlength="80" placeholder="例如 openai（可空）" />
                </el-form-item>
                <el-form-item label="生效时间" required>
                  <el-date-picker
                    v-model="priceForm.effectiveFrom"
                    type="datetime"
                    class="full-width"
                    format="YYYY-MM-DD HH:mm:ss"
                    value-format="YYYY-MM-DD HH:mm:ss"
                    placeholder="选择生效时间"
                  />
                </el-form-item>
              </div>
              <div class="form-row ai-price-form-row">
                <el-form-item label="输入单价（元 / 百万 Token）" required>
                  <el-input-number v-model="priceForm.inputPriceYuan" class="full-width" :min="0" :max="1000000" :precision="2" :step="0.1" controls-position="right" />
                </el-form-item>
                <el-form-item label="输出单价（元 / 百万 Token）" required>
                  <el-input-number v-model="priceForm.outputPriceYuan" class="full-width" :min="0" :max="1000000" :precision="2" :step="0.1" controls-position="right" />
                </el-form-item>
                <el-form-item label="缓存单价（元 / 百万 Token）" required>
                  <el-input-number v-model="priceForm.cachedPriceYuan" class="full-width" :min="0" :max="1000000" :precision="2" :step="0.1" controls-position="right" />
                </el-form-item>
                <el-form-item label="状态">
                  <el-switch v-model="priceForm.isActive" active-text="在用" inactive-text="已停用" />
                </el-form-item>
              </div>
              <div class="form-actions">
                <span class="audit-note">仅 super_admin 可维护；操作会写入审计。币种固定 CNY。</span>
                <el-button type="primary" :loading="submittingPrice" @click="submitModelPrice">保存价格</el-button>
              </div>
            </el-form>
            <el-table v-if="modelPrices.length" :data="modelPrices" row-key="id" stripe class="expense-table ai-table-gap">
              <el-table-column prop="model" label="模型" min-width="170" />
              <el-table-column prop="provider" label="供应商" min-width="110">
                <template #default="{ row }">{{ row.provider || '—' }}</template>
              </el-table-column>
              <el-table-column label="输入单价" min-width="150" align="right">
                <template #default="{ row }">{{ formatPricePerMillion(row.input_price_cents_per_million) }}</template>
              </el-table-column>
              <el-table-column label="输出单价" min-width="150" align="right">
                <template #default="{ row }">{{ formatPricePerMillion(row.output_price_cents_per_million) }}</template>
              </el-table-column>
              <el-table-column label="缓存单价" min-width="150" align="right">
                <template #default="{ row }">{{ formatPricePerMillion(row.cached_price_cents_per_million) }}</template>
              </el-table-column>
              <el-table-column label="生效时间" min-width="180">
                <template #default="{ row }">{{ formatDateTime(row.effective_from) }}</template>
              </el-table-column>
              <el-table-column label="状态" min-width="100" align="center">
                <template #default="{ row }">
                  <el-tag :type="row.is_active ? 'success' : 'info'" effect="light">{{ row.is_active ? '在用' : '已停用' }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="操作" min-width="110" align="center">
                <template #default="{ row }">
                  <el-button type="primary" link @click="toggleModelPrice(row)">{{ row.is_active ? '停用' : '启用' }}</el-button>
                </template>
              </el-table-column>
            </el-table>
            <el-empty v-else description="暂无模型价格；无生效价的调用按记录上的估算成本回落" :image-size="72" />
              </DataPanel>

    <DataPanel
      title="供应商账单"
      description="超管手工录入的供应商真实金额；与估算成本口径不同，差额仅供核对参考，不代表欠款或退款。"
      eyebrow="VENDOR BILLS"
    >

            <div class="period-toolbar">
              <el-date-picker
                v-model="aiDateRange"
                type="daterange"
                range-separator="至"
                start-placeholder="开始日期"
                end-placeholder="结束日期"
                format="YYYY-MM-DD"
                value-format="YYYY-MM-DD"
                :disabled-date="disableFutureDate"
                :clearable="false"
                unlink-panels
              />
              <el-button type="primary" plain :loading="reconciliationLoading" @click="loadVendorBills">查看期间</el-button>
              <span class="currency-hint">核对周期与用量统计共用同一日期范围</span>
            </div>
            <template v-if="reconciliation">
              <div class="period-grid ai-recon-grid">
                <article class="period-stat cost"><span>估算成本（估算）</span><strong>{{ money(reconciliation.estimated_cost_cents) }}</strong></article>
                <article class="period-stat manual"><span>实际账单合计（实际账单 · {{ reconciliation.actual_bill_count || 0 }} 笔）</span><strong>{{ money(reconciliation.actual_bill_total_cents) }}</strong></article>
                <article class="period-stat balance"><span>差额（实际 − 估算）</span><strong>{{ money(reconciliation.difference_cents_actual_minus_estimated) }}</strong></article>
              </div>
              <div class="plan-note">
                <strong>核对口径</strong>
                <span>{{ reconciliation.note || '估算成本来自调用记录上存的估算成本累计（写入时按当时生效价或内置费率估算），实际账单为超管手工录入的供应商真实金额；两者口径不同，差额仅供核对参考，不代表欠款或退款。' }}</span>
                <span v-if="reconciliation.unpriced_fallback_estimated_cost_cents">估算成本中 {{ money(reconciliation.unpriced_fallback_estimated_cost_cents) }} 无生效价，回落到调用记录上的估算成本。</span>
              </div>
            </template>
            <el-form class="ai-inline-form" label-position="top" @submit.prevent="submitVendorBill">
              <div class="form-row ai-bill-form-row">
                <el-form-item label="供应商" required>
                  <el-input v-model="billForm.vendor" maxlength="120" placeholder="例如 OpenAI / DeepSeek" />
                </el-form-item>
                <el-form-item label="账单周期" required>
                  <el-date-picker
                    v-model="billForm.period"
                    type="daterange"
                    range-separator="至"
                    start-placeholder="开始日期"
                    end-placeholder="结束日期"
                    format="YYYY-MM-DD"
                    value-format="YYYY-MM-DD"
                    :disabled-date="disableFutureDate"
                    :clearable="false"
                    unlink-panels
                  />
                </el-form-item>
                <el-form-item label="实际账单金额（人民币元）" required>
                  <el-input-number v-model="billForm.amountYuan" class="full-width" :min="0.01" :max="1000000000" :precision="2" :step="1" controls-position="right" />
                </el-form-item>
              </div>
              <el-form-item label="备注">
                <el-input v-model="billForm.note" maxlength="500" show-word-limit placeholder="例如：2026-11 OpenAI 月账单；录错请在备注说明并重录，本页不提供修改" />
              </el-form-item>
              <div class="form-actions">
                <span class="audit-note">录入的是供应商真实账单金额，不是估算；录入后不可修改，录错请重录并在备注中说明。</span>
                <el-button type="primary" :loading="submittingBill" @click="submitVendorBill">录入实际账单</el-button>
              </div>
            </el-form>
            <el-table v-if="vendorBills.length" :data="vendorBills" row-key="id" stripe class="expense-table ai-table-gap">
              <el-table-column prop="vendor" label="供应商" min-width="150" />
              <el-table-column label="账单周期" min-width="210">
                <template #default="{ row }">{{ row.period_start }} 至 {{ row.period_end }}</template>
              </el-table-column>
              <el-table-column label="实际账单金额（实际账单）" min-width="170" align="right">
                <template #default="{ row }">{{ money(row.amount_cents) }}</template>
              </el-table-column>
              <el-table-column prop="note" label="备注" min-width="200" show-overflow-tooltip />
              <el-table-column label="录入时间" min-width="170">
                <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
              </el-table-column>
            </el-table>
            <el-empty v-else description="本期暂无录入的供应商实际账单；无账单不代表实际成本为零" :image-size="72" />
              </DataPanel>
  </div>
</template>

<script setup>
import DataPanel from '../../../components/design/DataPanel.vue'
import PageHero from '../../../components/design/PageHero.vue'
import { usePlatformCost } from './usePlatformCost.js'
import { onMounted } from 'vue'

const {
  loading,
  submittingExpense,
  submittingReversal,
  reversalDialogVisible,
  selectedExpense,
  overview,
  error,
  dateRange,
  scenarioAssumptions,
  expenseCategories,
  expenseForm,
  reversalForm,
  selectedPeriod,
  aiUsageDimensions,
  overLimitActions,
  budgetAlertThresholds,
  aiStats,
  aiStatsLoading,
  aiStatsError,
  aiStatsDimension,
  aiDateRange,
  modelPrices,
  modelPricesLoading,
  submittingPrice,
  priceForm,
  vendorBills,
  reconciliation,
  reconciliationLoading,
  submittingBill,
  billForm,
  aiBudgets,
  budgetMerchantOptions,
  alertMerchantOptions,
  budgetMerchantSearchLoading,
  alertMerchantSearchLoading,
  merchantNameCache,
  submittingBudget,
  budgetForm,
  budgetStatus,
  budgetAlerts,
  alertsLoading,
  alertPage,
  alertPageSize,
  alertsTotal,
  alertFilter,
  emergencySwitch,
  submittingEmergency,
  emergencyDialogVisible,
  emergencyTargetDisabled,
  emergencyReason,
  aiDimensionLabel,
  aiStatsRows,
  scenarioCalculation,
  monthLabel,
  money,
  manualSummaryAmount,
  manualSummaryCount,
  categoryLabel,
  formatDateTime,
  disableFutureDate,
  resetScenario,
  loadOverview,
  submitExpense,
  expenseCostTypes,
  expensePaymentStatuses,
  timeLogWorkTypes,
  incomeTypes,
  incomeStatuses,
  expenseStatsStartMonth,
  expenseStatsEndMonth,
  expenseStats,
  expenseStatsLoading,
  expenseStatRowKey,
  loadExpenseStats,
  timeLogList,
  submittingTimeLog,
  timeLogForm,
  loadTimeLogs,
  submitTimeLog,
  incomeList,
  submittingIncome,
  incomeForm,
  isSubscriptionIncomeType,
  loadIncomeRecords,
  submitIncome,
  incomeStatusTagType,
  incomeDialogVisible,
  incomeDialogRecord,
  incomeDialog,
  submittingIncomeUpdate,
  incomeDialogTargets,
  openIncomeDialog,
  submitIncomeUpdate,
  cancelIncomeRecord,
  mrrMonth,
  mrr,
  mrrLoading,
  breakevenForm,
  breakeven,
  breakevenLoading,
  loadMRR,
  runBreakeven,
  openReversal,
  submitReversal,
  loadAiUsageStats,
  loadModelPrices,
  submitModelPrice,
  toggleModelPrice,
  loadVendorBills,
  submitVendorBill,
  searchMerchantsRemote,
  searchBudgetMerchants,
  searchAlertMerchants,
  loadBudgets,
  resetBudgetForm,
  editBudget,
  submitBudget,
  loadBudgetStatus,
  loadBudgetAlerts,
  reloadBudgetAlerts,
  loadEmergencySwitch,
  openEmergencyDialog,
  submitEmergencySwitch,
  getPlatformCostPeriodValidationError,
  calculatePlatformCostScenario,
  AI_USAGE_DIMENSIONS,
  BUDGET_ALERT_THRESHOLDS,
  OVER_LIMIT_ACTIONS,
  budgetMerchantLabel,
  centsToYuan,
  formatPricePerMillion,
  localDateTimeString,
  merchantDisplayName,
  newUUID,
  overLimitActionLabel,
  periodTypeLabel,
  scenarioLabel,
  storeDisplayName,
  yuanToCents,
  createAdminAIVendorBill,
  createAdminPlatformExpense,
  createAdminPlatformExpenseReversal,
  createAdminPlatformIncomeRecord,
  createAdminPlatformTimeLog,
  fetchAdminAIBillReconciliation,
  fetchAdminAIVendorBills,
  fetchAdminAIBudgetAlerts,
  fetchAdminAIBudgets,
  fetchAdminAIBudgetStatus,
  fetchAdminAIEmergencySwitch,
  fetchAdminAIModelPrices,
  fetchAdminAIUsageStats,
  fetchAdminPlatformBreakeven,
  fetchAdminPlatformCostOverview,
  fetchAdminPlatformExpenseStats,
  fetchAdminPlatformIncomeRecords,
  fetchAdminPlatformMRR,
  fetchAdminPlatformTimeLogs,
  fetchMerchants,
  saveAdminAIBudget,
  saveAdminAIEmergencySwitch,
  saveAdminAIModelPrice,
  updateAdminPlatformIncomeRecord,
  EXPENSE_COST_TYPES,
  EXPENSE_PAYMENT_STATUSES,
  INCOME_STATUSES,
  INCOME_TYPES,
  TIME_LOG_WORK_TYPES,
  expenseCostTypeLabel,
  expensePaymentStatusLabel,
  hoursFromScaled,
  incomeCanCancel,
  incomeNextStatuses,
  incomeStatusLabel,
  incomeTypeLabel,
  timeLogWorkTypeLabel,
} = usePlatformCost()

onMounted(() => {
  loadAiUsageStats()
  loadModelPrices()
  loadVendorBills()
})
</script>

<style scoped src="./platform-cost.css"></style>
