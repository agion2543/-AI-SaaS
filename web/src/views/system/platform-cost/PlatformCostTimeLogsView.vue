<template>
  <div class="platform-cost-page">
    <PageHero
      eyebrow="TIME COST"
      title="时间成本"
      description="只记你自己的时间与平台真实收入；时间成本只计经济成本，不进现金支出。全部数据仅超管可见。"
      compact
    >
      <template #actions>
        <el-tag effect="light" type="warning">仅超管可见</el-tag>
        <el-button type="primary" :loading="timeLogsLoading" @click="loadTimeLogs">刷新数据</el-button>
      </template>
    </PageHero>

    <el-skeleton v-if="timeLogsLoading && !timeLogList" :rows="4" animated />

    <section v-else-if="timeLogList" class="summary-grid">
      <article class="summary-card">
        <div class="card-label">时间成本记录</div>
        <strong>{{ timeLogList?.rows?.length || 0 }} 笔</strong>
        <p>只追加不改不删；金额由服务端按小时数 × 参考小时价值算出。</p>
      </article>
      <article class="summary-card">
        <div class="card-label">合计小时</div>
        <strong>{{ hoursFromScaled(timeLogList?.total_hours_scaled || 0) }} 小时</strong>
        <p>服务端合计口径；下方表格列示最近 50 条记录。</p>
      </article>
      <article class="summary-card">
        <div class="card-label">经济成本</div>
        <strong>{{ money(timeLogList?.total_amount_cents || 0) }}</strong>
        <p>时间成本只计经济成本，不进现金支出。</p>
      </article>
    </section>

    <DataPanel
      title="时间成本"
      description="只记你自己的时间；金额由服务端按（小时数 × 参考小时价值）算出，参考小时价值由你自填，系统不推断真实工资。"
      eyebrow="TIME COST"
    >

            <el-alert
              class="ai-form-note"
              type="info"
              show-icon
              :closable="false"
              title="时间成本只计经济成本，不进现金支出"
              description="金额由服务端按（小时数 × 参考小时价值）算出；参考小时价值由你自填，系统不推断真实工资。记录只追加不改不删。"
            />
            <el-form id="time-log-form" class="expense-form" label-position="top" @submit.prevent="submitTimeLog">
              <div class="form-row two-col">
                <el-form-item label="日期" required>
                  <el-date-picker
                    v-model="timeLogForm.logDate"
                    type="date"
                    class="full-width"
                    format="YYYY-MM-DD"
                    value-format="YYYY-MM-DD"
                    :disabled-date="disableFutureDate"
                    placeholder="选择日期"
                  />
                </el-form-item>
                <el-form-item label="工作类型" required>
                  <el-select v-model="timeLogForm.workType" class="full-width" placeholder="选择工作类型">
                    <el-option v-for="item in timeLogWorkTypes" :key="item.value" :label="item.label" :value="item.value" />
                  </el-select>
                </el-form-item>
              </div>
              <div class="form-row two-col">
                <el-form-item label="小时数" required>
                  <el-input-number
                    v-model="timeLogForm.hours"
                    class="full-width"
                    :min="0.01"
                    :max="24"
                    :precision="2"
                    :step="0.5"
                    controls-position="right"
                  />
                </el-form-item>
                <el-form-item label="参考小时价值（人民币元 / 小时）" required>
                  <el-input-number
                    v-model="timeLogForm.referenceHourlyValueYuan"
                    class="full-width"
                    :min="0.01"
                    :max="100000"
                    :precision="2"
                    :step="10"
                    controls-position="right"
                  />
                </el-form-item>
              </div>
              <el-form-item label="内容" required>
                <el-input v-model="timeLogForm.content" maxlength="500" show-word-limit placeholder="例如：成本中心 v14 联调" />
              </el-form-item>
              <el-form-item label="项目模块">
                <el-input v-model="timeLogForm.module" maxlength="80" placeholder="例如：成本中心（可空）" />
              </el-form-item>
              <div class="form-actions">
                <span class="audit-note">金额 = 小时数 × 参考小时价值（服务端整数分算出，四舍五入到分）。</span>
                <el-button type="primary" :loading="submittingTimeLog" @click="submitTimeLog">登记时间成本</el-button>
              </div>
            </el-form>
            <div class="table-footnote">合计 {{ hoursFromScaled(timeLogList?.total_hours_scaled || 0) }} 小时 · 经济成本 {{ money(timeLogList?.total_amount_cents || 0) }}</div>
            <el-table v-if="timeLogList?.rows?.length" :data="timeLogList.rows" row-key="id" stripe class="expense-table ai-table-gap">
              <el-table-column prop="log_date" label="日期" min-width="112" />
              <el-table-column label="工作类型" min-width="130">
                <template #default="{ row }">{{ timeLogWorkTypeLabel(row.work_type) }}</template>
              </el-table-column>
              <el-table-column prop="content" label="内容" min-width="200" show-overflow-tooltip />
              <el-table-column label="小时数" min-width="90" align="right">
                <template #default="{ row }">{{ hoursFromScaled(row.hours_scaled) }}</template>
              </el-table-column>
              <el-table-column label="参考小时价值" min-width="140" align="right">
                <template #default="{ row }">{{ money(row.reference_hourly_value_cents) }} / 小时</template>
              </el-table-column>
              <el-table-column label="经济成本" min-width="130" align="right">
                <template #default="{ row }">{{ money(row.amount_cents) }}</template>
              </el-table-column>
              <el-table-column prop="module" label="模块" min-width="120">
                <template #default="{ row }">{{ row.module || '—' }}</template>
              </el-table-column>
            </el-table>
            <el-empty v-else-if="!timeLogsLoading" description="暂无时间成本记录" :image-size="72">
              <el-button type="primary" @click="scrollToId('time-log-form')">去登记时间成本</el-button>
            </el-empty>
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
  timeLogsLoading,
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
  loadTimeLogs()
})

// 空状态主操作按钮：回到本页的录入表单（豆包 §3.1 / 规范 §3.5）。
const scrollToId = (id) => {
  document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}
</script>

<style scoped src="./platform-cost.css"></style>
