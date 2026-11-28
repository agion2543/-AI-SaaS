<template>
  <div class="platform-cost-page">
    <PageHero
      eyebrow="INCOME LEDGER"
      title="收入台账"
      description="平台真实收入台账；订单金额不等于收入，只有状态流转到已确认或已收款的记录才算作收入。"
      compact
    >
      <template #actions>
        <el-tag effect="light" type="warning">仅超管可见</el-tag>
        <el-button type="primary" @click="loadIncomeRecords">刷新数据</el-button>
      </template>
    </PageHero>

    <section class="summary-grid">
      <article class="summary-card">
        <div class="card-label">收入记录</div>
        <strong>{{ incomeRecordCount }} 笔</strong>
        <p>按状态汇总见下方表格脚注；金额、类型与日期一旦登记不可再改。</p>
      </article>
      <article class="summary-card">
        <div class="card-label">已收款金额</div>
        <strong>{{ money(incomeReceivedCents) }}</strong>
        <p>状态为已收款的记录合计。</p>
      </article>
      <article class="summary-card">
        <div class="card-label">待收款金额</div>
        <strong>{{ money(incomePendingCents) }}</strong>
        <p>状态为待收款的记录合计。</p>
      </article>
    </section>

    <DataPanel
      title="收入台账"
      description="只有状态流转到已确认或已收款的记录才算作收入；已退款与已取消不计入 MRR。"
      eyebrow="INCOME LEDGER"
    >

            <el-alert
              class="ai-form-note"
              type="info"
              show-icon
              :closable="false"
              title="订单金额不等于收入"
              description="只有状态流转到已确认或已收款的记录才算作收入；已退款与已取消不计入 MRR。金额、类型与日期一旦登记不可再改，只能走状态流转。"
            />
            <el-form class="expense-form" label-position="top" @submit.prevent="submitIncome">
              <div class="form-row two-col">
                <el-form-item label="收入类型" required>
                  <el-select v-model="incomeForm.incomeType" class="full-width" placeholder="选择收入类型">
                    <el-option v-for="item in incomeTypes" :key="item.value" :label="item.label" :value="item.value" />
                  </el-select>
                </el-form-item>
                <el-form-item label="金额（人民币元）" required>
                  <el-input-number
                    v-model="incomeForm.amountYuan"
                    class="full-width"
                    :min="0.01"
                    :max="1000000000"
                    :precision="2"
                    :step="1"
                    controls-position="right"
                  />
                </el-form-item>
              </div>
              <div class="form-row two-col">
                <el-form-item label="发生日期" required>
                  <el-date-picker
                    v-model="incomeForm.occurredDate"
                    type="date"
                    class="full-width"
                    format="YYYY-MM-DD"
                    value-format="YYYY-MM-DD"
                    :disabled-date="disableFutureDate"
                    placeholder="选择发生日期"
                  />
                </el-form-item>
                <el-form-item label="初始状态">
                  <el-select v-model="incomeForm.status" class="full-width">
                    <el-option v-for="item in incomeStatuses" :key="item.value" :label="item.label" :value="item.value" />
                  </el-select>
                </el-form-item>
              </div>
              <div v-if="incomeForm.status === 'received'" class="form-row two-col">
                <el-form-item label="收款日期" required>
                  <el-date-picker
                    v-model="incomeForm.receivedDate"
                    type="date"
                    class="full-width"
                    format="YYYY-MM-DD"
                    value-format="YYYY-MM-DD"
                    :disabled-date="disableFutureDate"
                    placeholder="选择收款日期"
                  />
                </el-form-item>
                <el-form-item label="商家 ID（可空）">
                  <el-input-number v-model="incomeForm.merchantId" class="full-width" :min="1" :precision="0" :value-on-clear="null" controls-position="right" placeholder="可空" />
                </el-form-item>
              </div>
              <div v-else class="form-row two-col">
                <el-form-item label="商家 ID（可空）">
                  <el-input-number v-model="incomeForm.merchantId" class="full-width" :min="1" :precision="0" :value-on-clear="null" controls-position="right" placeholder="可空" />
                </el-form-item>
                <el-form-item label="备注">
                  <el-input v-model="incomeForm.note" maxlength="500" placeholder="可空" />
                </el-form-item>
              </div>
              <div v-if="isSubscriptionIncomeType(incomeForm.incomeType)" class="form-row two-col">
                <el-form-item label="订阅开始日期" required>
                  <el-date-picker
                    v-model="incomeForm.subscriptionStartDate"
                    type="date"
                    class="full-width"
                    format="YYYY-MM-DD"
                    value-format="YYYY-MM-DD"
                    placeholder="订阅期开始"
                  />
                </el-form-item>
                <el-form-item label="订阅结束日期" required>
                  <el-date-picker
                    v-model="incomeForm.subscriptionEndDate"
                    type="date"
                    class="full-width"
                    format="YYYY-MM-DD"
                    value-format="YYYY-MM-DD"
                    placeholder="订阅期结束"
                  />
                </el-form-item>
              </div>
              <div class="form-row two-col">
                <el-form-item label="计费周期（月）">
                  <el-select v-model="incomeForm.billingPeriodMonths" class="full-width">
                    <el-option :value="1" label="按月（1 个月）" />
                    <el-option :value="3" label="按季（3 个月）" />
                    <el-option :value="6" label="半年（6 个月）" />
                    <el-option :value="12" label="按年（12 个月）" />
                  </el-select>
                </el-form-item>
                <el-form-item label="备注">
                  <el-input v-model="incomeForm.note" maxlength="500" placeholder="可空" />
                </el-form-item>
              </div>
              <div class="form-actions">
                <span class="audit-note">订阅类收入必须填订阅期，MRR 按订阅期摊销；年付按十二分之一摊。</span>
                <el-button type="primary" :loading="submittingIncome" @click="submitIncome">登记收入</el-button>
              </div>
            </el-form>
            <el-table v-if="incomeList?.rows?.length" :data="incomeList.rows" row-key="id" stripe class="expense-table ai-table-gap">
              <el-table-column prop="occurred_date" label="发生日期" min-width="112" />
              <el-table-column label="类型" min-width="120">
                <template #default="{ row }">{{ incomeTypeLabel(row.income_type) }}</template>
              </el-table-column>
              <el-table-column label="金额" min-width="120" align="right">
                <template #default="{ row }">{{ money(row.amount_cents) }}</template>
              </el-table-column>
              <el-table-column label="状态" min-width="110" align="center">
                <template #default="{ row }">
                  <el-tag :type="incomeStatusTagType(row.status)" effect="light">{{ incomeStatusLabel(row.status) }}</el-tag>
                  <el-tag v-if="row.is_cancelled" type="info" effect="light">已取消</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="退款金额" min-width="110" align="right">
                <template #default="{ row }">{{ row.refund_amount_cents > 0 ? money(row.refund_amount_cents) : '—' }}</template>
              </el-table-column>
              <el-table-column prop="received_date" label="收款日期" min-width="112">
                <template #default="{ row }">{{ row.received_date || '—' }}</template>
              </el-table-column>
              <el-table-column prop="note" label="备注" min-width="160" show-overflow-tooltip />
              <el-table-column label="操作" min-width="150" align="center">
                <template #default="{ row }">
                  <el-button v-if="incomeNextStatuses(row.status).length" type="primary" link @click="openIncomeDialog(row)">流转状态</el-button>
                  <el-button v-if="incomeCanCancel(row.status) && !row.is_cancelled" type="danger" link @click="cancelIncomeRecord(row)">取消</el-button>
                  <span v-if="!incomeNextStatuses(row.status).length && !(incomeCanCancel(row.status) && !row.is_cancelled)">—</span>
                </template>
              </el-table-column>
            </el-table>
            <el-empty v-else description="暂无收入记录" :image-size="72" />
            <div v-if="incomeList?.totals_by_status" class="table-footnote">
              <span v-for="(total, status) in incomeList.totals_by_status" :key="status">{{ incomeStatusLabel(status) }} {{ total.count }} 笔 · {{ money(total.amount_cents) }}；</span>
            </div>
              </DataPanel>

<el-dialog v-model="incomeDialogVisible" title="收入状态流转" width="min(560px, 94vw)" top="3vh" :close-on-click-modal="false">
        <template v-if="incomeDialogRecord">
          <div class="reversal-source-card">
            <strong>收入 #{{ incomeDialogRecord.id }} · {{ incomeTypeLabel(incomeDialogRecord.income_type) }}</strong>
            <span>{{ incomeDialogRecord.occurred_date }} · {{ money(incomeDialogRecord.amount_cents) }} · 当前状态：{{ incomeStatusLabel(incomeDialogRecord.status) }}</span>
          </div>
          <el-form label-position="top">
            <el-form-item label="目标状态" required>
              <el-select v-model="incomeDialog.targetStatus" class="full-width">
                <el-option v-for="item in incomeDialogTargets" :key="item" :label="incomeStatusLabel(item)" :value="item" />
              </el-select>
            </el-form-item>
            <el-form-item v-if="incomeDialog.targetStatus === 'received'" label="收款日期" required>
              <el-date-picker
                v-model="incomeDialog.receivedDate"
                type="date"
                class="full-width"
                format="YYYY-MM-DD"
                value-format="YYYY-MM-DD"
                :disabled-date="disableFutureDate"
                placeholder="选择收款日期"
              />
            </el-form-item>
            <el-form-item v-if="incomeDialog.targetStatus === 'refunding' || incomeDialog.targetStatus === 'refunded'" label="退款金额（元）" required>
              <el-input-number
                v-model="incomeDialog.refundAmountYuan"
                class="full-width"
                :min="0.01"
                :max="incomeDialogRecord.amount_cents / 100"
                :precision="2"
                :step="0.01"
                controls-position="right"
              />
              <div class="input-hint">不超过收入金额 {{ money(incomeDialogRecord.amount_cents) }}。</div>
            </el-form-item>
            <el-form-item label="备注">
              <el-input v-model="incomeDialog.note" maxlength="500" placeholder="可空；留空保持原备注" />
            </el-form-item>
          </el-form>
        </template>
        <template #footer>
          <el-button @click="incomeDialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="submittingIncomeUpdate" @click="submitIncomeUpdate">确认流转</el-button>
        </template>
      </el-dialog>  </div>
</template>

<script setup>
import DataPanel from '../../../components/design/DataPanel.vue'
import PageHero from '../../../components/design/PageHero.vue'
import { usePlatformCost } from './usePlatformCost.js'
import { computed, onMounted } from 'vue'

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

const incomeTotals = computed(() => incomeList.value?.totals_by_status || {})
const incomeRecordCount = computed(() => Object.values(incomeTotals.value).reduce((sum, item) => sum + (item?.count || 0), 0))
const incomeReceivedCents = computed(() => incomeTotals.value.received?.amount_cents || 0)
const incomePendingCents = computed(() => incomeTotals.value.pending_receipt?.amount_cents || 0)

onMounted(() => {
  loadIncomeRecords()
})
</script>

<style scoped src="./platform-cost.css"></style>
