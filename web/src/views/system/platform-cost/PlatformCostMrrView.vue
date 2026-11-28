<template>
  <div class="platform-cost-page">
    <PageHero
      eyebrow="MRR AND BREAK-EVEN"
      title="MRR 与保本"
      description="订阅月化收入与保本测算；测算参数一律标假设，不写入真实记录。"
      compact
    >
      <template #actions>
        <el-tag effect="light" type="warning">仅超管可见</el-tag>
        <el-button type="primary" :loading="mrrLoading" @click="loadMRR">刷新数据</el-button>
      </template>
    </PageHero>

    <el-skeleton v-if="mrrLoading && !mrr" :rows="4" animated />

    <section v-else-if="mrr" class="summary-grid">
      <article class="summary-card">
        <div class="card-label">MRR（当月有效订阅月化金额之和）</div>
        <strong>{{ money(mrr.mrr_cents) }}</strong>
        <p>{{ mrr.formula }}</p>
      </article>
      <article class="summary-card">
        <div class="card-label">计入订阅数</div>
        <strong>{{ mrr.included_count }} 笔</strong>
        <p>剔除：已取消 / 已退款 / 不在当月 / 状态不符 / 非订阅共 {{ (mrr.excluded_cancelled || 0) + (mrr.excluded_refunded || 0) + (mrr.excluded_out_of_period || 0) + (mrr.excluded_wrong_status || 0) + (mrr.excluded_non_subscription || 0) }} 笔。</p>
      </article>
      <article class="summary-card">
        <div class="card-label">保本商家数</div>
        <strong>{{ breakeven ? (breakeven.params_missing ? '缺参数' : breakeven.can_break_even ? `${breakeven.breakeven_merchants} 家` : '增加商家数无法保本') : '未试算' }}</strong>
        <p>在下方“保本试算”填单商家月均收入与月变动成本后给出结论。</p>
      </article>
    </section>

    <DataPanel
      title="MRR"
      description="当月有效订阅月化金额之和；年付按十二分之一摊。"
      eyebrow="MRR"
    >

            <div class="period-toolbar">
              <el-date-picker v-model="mrrMonth" type="month" format="YYYY-MM" value-format="YYYY-MM" placeholder="选择月份" />
              <el-button type="primary" plain :loading="mrrLoading" @click="loadMRR">查看 MRR</el-button>
              <span class="currency-hint">金额单位：人民币 CNY</span>
            </div>
            <div v-if="mrr" class="period-grid">
              <article class="period-stat income">
                <span>MRR（当月有效订阅月化金额之和）</span>
                <strong>{{ money(mrr.mrr_cents) }}</strong>
              </article>
              <article class="period-stat net">
                <span>计入订阅数</span>
                <strong>{{ mrr.included_count }} 笔</strong>
              </article>
              <article class="period-stat manual">
                <span>剔除：已取消 / 已退款 / 不在当月 / 状态不符 / 非订阅</span>
                <strong>{{ mrr.excluded_cancelled }} / {{ mrr.excluded_refunded }} / {{ mrr.excluded_out_of_period }} / {{ mrr.excluded_wrong_status }} / {{ mrr.excluded_non_subscription }}</strong>
              </article>
            </div>
            <div v-if="mrr" class="table-footnote">{{ mrr.formula }}。{{ mrr.note }}</div>
            
              </DataPanel>

    <DataPanel
      title="保本试算"
      description="假设试算：单商家数字是你输入的假设，不写入真实收入或成本记录；月固定成本取自当月已登记的固定现金费用减当月冲正。"
      eyebrow="BREAK-EVEN"
    >

            <div class="scenario-notice">
              <el-tag type="warning" effect="light">假设试算</el-tag>
              <span>以下两个单商家数字是你输入的假设，不写入真实收入或成本记录；月固定成本取自当月已登记的固定现金费用减当月冲正。</span>
            </div>
            <div class="form-row two-col">
              <el-form-item label="单商家月均收入（元，假设）">
                <el-input-number
                  v-model="breakevenForm.revenueYuan"
                  class="full-width"
                  :min="0"
                  :max="10000000"
                  :precision="2"
                  :step="10"
                  :value-on-clear="null"
                  controls-position="right"
                />
              </el-form-item>
              <el-form-item label="单商家月变动成本（元，假设）">
                <el-input-number
                  v-model="breakevenForm.variableYuan"
                  class="full-width"
                  :min="0"
                  :max="10000000"
                  :precision="2"
                  :step="10"
                  :value-on-clear="null"
                  controls-position="right"
                />
              </el-form-item>
            </div>
            <div class="form-actions">
              <span class="audit-note">两项都填后才给出结论；缺参数时不输出保本结论。</span>
              <el-button type="primary" plain :loading="breakevenLoading" @click="runBreakeven">试算保本商家数</el-button>
            </div>
            <div v-if="breakeven" class="period-grid ai-table-gap">
              <article class="period-stat cost">
                <span>月固定现金成本</span>
                <strong>{{ money(breakeven.fixed_cash_cost_cents) }}</strong>
              </article>
              <article class="period-stat manual">
                <span>当月时间经济成本（不计入现金保本口径）</span>
                <strong>{{ money(breakeven.time_log_economic_cents) }}</strong>
              </article>
              <article class="period-stat net">
                <span>单商家贡献毛利（假设）</span>
                <strong>{{ breakeven.contribution_margin_cents == null ? '缺参数' : money(breakeven.contribution_margin_cents) }}</strong>
              </article>
              <article class="period-stat balance">
                <span>保本商家数</span>
                <strong>{{ breakeven.params_missing ? '缺参数' : breakeven.can_break_even ? `${breakeven.breakeven_merchants} 家` : '增加商家数无法保本' }}</strong>
              </article>
            </div>
            <div v-if="breakeven" class="table-footnote">{{ breakeven.formula }}。{{ breakeven.note }}</div>
              </DataPanel>

<DataPanel
        title="平台经营情景测算"
        description="手动调整月度收入与成本假设，观察规模和盈亏变化；不从实际账本回填，也不构成真实财务结果。"
        eyebrow="SCENARIO CALCULATOR"
      >
        <div class="scenario-notice">
          <el-tag type="warning" effect="light">仅供情景测算</el-tag>
          <span>这些是你自行输入的假设，不是实际收入或成本记录；不会写入数据库，离开或刷新本页后清空。</span>
        </div>
        <div class="scenario-layout">
          <div class="scenario-input-panel">
            <div class="scenario-fields">
              <div class="scenario-field">
                <label for="scenario-revenue">月均订阅收入 / 付费商家（元）</label>
                <el-input-number
                  id="scenario-revenue"
                  v-model="scenarioAssumptions.monthlyAverageSubscriptionRevenuePerMerchantYuan"
                  class="full-width"
                  :min="0"
                  :max="10000000"
                  :precision="2"
                  :step="100"
                  :value-on-clear="null"
                  controls-position="right"
                />
                <small>平台每家付费商家的月均订阅收入假设</small>
              </div>
              <div class="scenario-field">
                <label for="scenario-merchants">付费商家数（家）</label>
                <el-input-number
                  id="scenario-merchants"
                  v-model="scenarioAssumptions.payingMerchantCount"
                  class="full-width"
                  :min="0"
                  :max="100000000"
                  :precision="0"
                  :step="10"
                  :value-on-clear="null"
                  controls-position="right"
                />
                <small>用于计算本月规模的假设商家数</small>
              </div>
              <div class="scenario-field">
                <label for="scenario-fixed-cost">固定月成本（元）</label>
                <el-input-number
                  id="scenario-fixed-cost"
                  v-model="scenarioAssumptions.fixedMonthlyCostsYuan"
                  class="full-width"
                  :min="0"
                  :max="10000000"
                  :precision="2"
                  :step="100"
                  :value-on-clear="null"
                  controls-position="right"
                />
                <small>不随付费商家数变化的月度成本假设</small>
              </div>
              <div class="scenario-field">
                <label for="scenario-variable-cost">单商家月变动成本（元）</label>
                <el-input-number
                  id="scenario-variable-cost"
                  v-model="scenarioAssumptions.variableCostPerMerchantYuan"
                  class="full-width"
                  :min="0"
                  :max="10000000"
                  :precision="2"
                  :step="10"
                  :value-on-clear="null"
                  controls-position="right"
                />
                <small>每增加一家付费商家对应的月度成本假设</small>
              </div>
            </div>
            <div class="scenario-input-actions">
              <el-button plain @click="resetScenario">清空假设</el-button>
              <span>四项输入均为空时，不显示任何估算值。</span>
            </div>
          </div>

          <div class="scenario-result-panel" aria-live="polite">
            <div class="scenario-result-heading">
              <strong>月度估算结果</strong>
              <span>仅按上方假设 · CNY</span>
            </div>
            <div v-if="!scenarioCalculation" class="scenario-empty">
              填写全部四项假设后查看估算。当前没有预填数据，也不会自动读取实际收支。
            </div>
            <el-alert
              v-else-if="scenarioCalculation.error"
              type="error"
              show-icon
              :closable="false"
              :title="scenarioCalculation.error"
            />
            <template v-else>
              <div class="scenario-output-grid">
                <article class="scenario-output">
                  <span>预计月收入</span>
                  <strong>{{ money(scenarioCalculation.result.projectedMonthlyRevenueCents) }}</strong>
                </article>
                <article class="scenario-output">
                  <span>预计月变动成本</span>
                  <strong>{{ money(scenarioCalculation.result.projectedMonthlyVariableCostCents) }}</strong>
                </article>
                <article class="scenario-output">
                  <span>预计月总成本</span>
                  <strong>{{ money(scenarioCalculation.result.projectedMonthlyTotalCostCents) }}</strong>
                </article>
                <article
                  class="scenario-output result"
                  :class="scenarioCalculation.result.projectedMonthlyOperatingResultCents >= 0 ? 'positive' : 'negative'"
                >
                  <span>{{ scenarioCalculation.result.projectedMonthlyOperatingResultCents >= 0 ? '预计月度经营盈余' : '预计月度经营缺口' }}</span>
                  <strong>{{ money(scenarioCalculation.result.projectedMonthlyOperatingResultCents) }}</strong>
                </article>
                <article
                  class="scenario-output"
                  :class="scenarioCalculation.result.contributionPerMerchantCents > 0 ? 'positive' : 'negative'"
                >
                  <span>单商家贡献</span>
                  <strong>{{ money(scenarioCalculation.result.contributionPerMerchantCents) }}</strong>
                </article>
                <article
                  class="scenario-output break-even"
                  :class="scenarioCalculation.result.breakEvenReachable ? 'positive' : 'negative'"
                >
                  <span>盈亏平衡所需付费商家数</span>
                  <strong>{{ scenarioCalculation.result.breakEvenReachable ? `${scenarioCalculation.result.breakEvenPayingMerchantCount} 家` : '无法靠增加商家达到' }}</strong>
                </article>
              </div>
              <el-alert
                v-if="!scenarioCalculation.result.breakEvenReachable"
                class="scenario-break-even-alert"
                type="warning"
                show-icon
                :closable="false"
                title="单商家贡献为零或负数；继续增加付费商家也无法覆盖固定月成本。"
              />
              <div v-else class="scenario-break-even-note">
                盈亏平衡人数按固定月成本 ÷ 单商家贡献向上取整；只覆盖本表列出的假设。
              </div>
              <div class="scenario-formulas">
                <strong>计算公式</strong>
                <span>预计月收入 = 月均订阅收入 × 付费商家数</span>
                <span>预计月总成本 = 固定月成本 +（单商家月变动成本 × 付费商家数）</span>
                <span>单商家贡献 = 月均订阅收入 − 单商家月变动成本</span>
                <span>预计月度经营结余 = 预计月收入 − 预计月总成本</span>
                <span>盈亏平衡人数 = 固定月成本 ÷ 单商家贡献，向上取整（贡献≤0 时不可达）</span>
              </div>
            </template>
          </div>
        </div>
      </DataPanel>  </div>
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
  loadMRR()
})
</script>

<style scoped src="./platform-cost.css"></style>
