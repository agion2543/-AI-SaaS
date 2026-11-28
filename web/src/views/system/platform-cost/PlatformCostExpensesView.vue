<template>
  <div class="platform-cost-page">
    <PageHero
      eyebrow="PLATFORM EXPENSES"
      title="平台支出"
      description="登记已发生的平台费用，核对费用与冲正；费用按发生日归期，冲正按冲正日抵减，不回溯原费用期间。"
      compact
    >
      <template #actions>
        <el-tag effect="light" type="warning">仅超管可见</el-tag>
        <el-button type="primary" :loading="loading" @click="loadOverview">刷新数据</el-button>
      </template>
    </PageHero>

    <el-alert
      v-if="error"
      class="page-alert"
      type="error"
      show-icon
      :closable="false"
      :title="error"
    />

    <el-skeleton v-if="loading && !overview" :rows="6" animated />

    <template v-else-if="overview">
<div class="period-toolbar">
          <el-date-picker
            v-model="dateRange"
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
          <el-button type="primary" plain :loading="loading" @click="loadOverview">查看期间</el-button>
          <span class="currency-hint">金额单位：人民币 CNY</span>
        </div>
      <section class="summary-grid">
        <article class="summary-card">
          <div class="card-label">订阅净实收（所选期间）</div>
          <strong>{{ money(selectedPeriod.subscription_net_received_cents) }}</strong>
          <p>已确认订阅到账 {{ money(selectedPeriod.subscription_gross_received_cents) }}，已确认退款 {{ money(selectedPeriod.subscription_confirmed_refunds_cents) }}</p>
        </article>
        <article class="summary-card">
          <div class="card-label">已登记费用净额（所选期间）</div>
          <strong>{{ selectedPeriod.actual_expense_total_cents == null ? '暂无已录入成本' : money(selectedPeriod.actual_expense_total_cents) }}</strong>
          <p>{{ selectedPeriod.actual_expense_count || 0 }} 笔费用 · {{ selectedPeriod.expense_reversal_count || 0 }} 笔冲正</p>
        </article>
        <article class="summary-card">
          <div class="card-label">扣已登记成本后的差额</div>
          <strong>{{ selectedPeriod.net_after_recorded_expenses_cents == null ? '利润暂不可算' : money(selectedPeriod.net_after_recorded_expenses_cents) }}</strong>
          <p>订阅净实收减已登记实际费用的阶段性差额，该差额不等于利润。</p>
        </article>
      </section>

<el-alert
          class="period-alert"
          :type="selectedPeriod.actual_expense_count || selectedPeriod.expense_reversal_count ? 'info' : 'warning'"
          show-icon
          :closable="false"
          :title="selectedPeriod.profit_status_note || '暂无已录入成本，利润暂不可算。'"
        />
<div class="work-grid" id="expense-records">
        <DataPanel
          title="登记已发生的平台费用"
          description="只录已经发生、可核验的支出；金额以分保存，固定币种为 CNY。"
          eyebrow="ACTUAL EXPENSE ENTRY"
        >
          <el-form id="expense-entry-form" class="expense-form" label-position="top" @submit.prevent="submitExpense">
            <div class="form-row two-col">
              <el-form-item label="费用类别" required>
                <el-select v-model="expenseForm.category" class="full-width" placeholder="选择费用类别">
                  <el-option v-for="item in expenseCategories" :key="item.value" :label="item.label" :value="item.value" />
                </el-select>
              </el-form-item>
              <el-form-item label="发生日期" required>
                <el-date-picker
                  v-model="expenseForm.expenseDate"
                  type="date"
                  class="full-width"
                  format="YYYY-MM-DD"
                  value-format="YYYY-MM-DD"
                  :disabled-date="disableFutureDate"
                  placeholder="选择费用发生日期"
                />
              </el-form-item>
            </div>
            <div class="form-row two-col">
              <el-form-item label="金额（人民币元）" required>
                <el-input-number
                  v-model="expenseForm.amountYuan"
                  class="full-width"
                  :min="0.01"
                  :max="1000000000"
                  :precision="2"
                  :step="1"
                  controls-position="right"
                />
              </el-form-item>
              <el-form-item label="币种">
                <el-input model-value="CNY · 人民币" disabled />
              </el-form-item>
            </div>
            <div class="form-row two-col">
              <el-form-item label="固定 / 变动" required>
                <el-select v-model="expenseForm.costType" class="full-width" placeholder="选择固定或变动">
                  <el-option v-for="item in expenseCostTypes" :key="item.value" :label="item.label" :value="item.value" />
                </el-select>
              </el-form-item>
              <el-form-item label="供应商">
                <el-input v-model="expenseForm.vendor" maxlength="120" placeholder="例如：阿里云（可空）" />
              </el-form-item>
            </div>
            <div class="form-row two-col">
              <el-form-item label="付款状态">
                <el-select v-model="expenseForm.paymentStatus" class="full-width">
                  <el-option v-for="item in expensePaymentStatuses" :key="item.value" :label="item.label" :value="item.value" />
                </el-select>
              </el-form-item>
              <el-form-item label="成本属性">
                <div class="cost-flag-row">
                  <el-checkbox v-model="expenseForm.isRecurring">经常性</el-checkbox>
                  <el-checkbox v-model="expenseForm.isCashCost">现金成本</el-checkbox>
                  <el-checkbox v-model="expenseForm.isEconomicCost">经济成本</el-checkbox>
                </div>
              </el-form-item>
            </div>
            <el-form-item label="简短备注" required>
              <el-input
                v-model="expenseForm.memo"
                maxlength="120"
                show-word-limit
                placeholder="例如：2026-11 云主机月账单"
              />
            </el-form-item>
            <el-alert
              class="immutable-note"
              type="warning"
              :closable="false"
              title="记录为追加不可变：不提供编辑或删除。需要更正时可追加一笔部分或全额冲正；原费用保留，按每笔冲正发生日期计入对应期间。"
            />
            <div class="form-actions">
              <span class="audit-note">仅 super_admin 可登记；操作会写入不包含金额/备注的用途审计。</span>
              <el-button type="primary" :loading="submittingExpense" @click="submitExpense">保存实际费用</el-button>
            </div>
          </el-form>
        </DataPanel>

        <DataPanel
          title="本期已登记费用"
          description="按原费用发生日列示；原费用及期间不回写。可分次冲正，但累计金额不超过原费用。"
          eyebrow="RECORDED COSTS"
        >
          <el-table v-if="selectedPeriod.expense_rows?.length" :data="selectedPeriod.expense_rows" row-key="id" stripe class="expense-table">
            <el-table-column prop="expense_date" label="日期" min-width="112" />
            <el-table-column label="类别" min-width="142">
              <template #default="{ row }">{{ categoryLabel(row.category) }}</template>
            </el-table-column>
            <el-table-column label="金额" min-width="132" align="right">
              <template #default="{ row }">{{ money(row.amount_cents) }}</template>
            </el-table-column>
            <el-table-column prop="memo" label="备注" min-width="190" show-overflow-tooltip />
            <el-table-column label="供应商" min-width="120">
              <template #default="{ row }">{{ row.vendor || '—' }}</template>
            </el-table-column>
            <el-table-column label="属性" min-width="220">
              <template #default="{ row }">
                <div class="expense-flags">
                  <el-tag effect="light">{{ expenseCostTypeLabel(row.cost_type) }}</el-tag>
                  <el-tag effect="light" type="info">{{ expensePaymentStatusLabel(row.payment_status) }}</el-tag>
                  <el-tag v-if="row.is_recurring" effect="light" type="warning">经常性</el-tag>
                  <el-tag v-if="row.is_cash_cost" effect="light" type="success">现金</el-tag>
                  <el-tag v-if="row.is_economic_cost" effect="light">经济</el-tag>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="冲正余额" min-width="230" align="center">
              <template #default="{ row }">
                <div class="reversal-balance">
                  <span v-if="row.reversal_count">已冲正 {{ money(row.reversed_amount_cents) }} / {{ money(row.amount_cents) }}（{{ row.reversal_count }} 笔）</span>
                  <span v-else>尚未冲正</span>
                  <strong v-if="row.remaining_reversible_cents > 0">剩余可冲 {{ money(row.remaining_reversible_cents) }}</strong>
                  <el-tag v-else type="info" effect="light">已冲清</el-tag>
                  <el-button v-if="row.remaining_reversible_cents > 0" type="warning" link @click="openReversal(row)">登记冲正</el-button>
                </div>
              </template>
            </el-table-column>
          </el-table>
          <el-empty v-else description="本期暂无已录入成本；这不代表实际成本为零" :image-size="72">
            <el-button type="primary" @click="scrollToId('expense-entry-form')">去登记费用</el-button>
          </el-empty>
          <div v-if="selectedPeriod.expense_rows_truncated" class="table-footnote">本期记录超过 500 笔，表格只显示最近的 500 条；汇总仍统计完整期间。</div>
        </DataPanel>
      </div>
<DataPanel
        title="本期费用冲正"
        description="按每笔冲正发生日列示；支持分次部分抵减，累计不超过原费用，不修改原始记录。"
        eyebrow="EXPENSE REVERSALS"
      >
        <el-table v-if="selectedPeriod.expense_reversal_rows?.length" :data="selectedPeriod.expense_reversal_rows" row-key="id" stripe class="expense-table">
          <el-table-column prop="reversal_date" label="冲正日期" min-width="112" />
          <el-table-column label="原费用" min-width="180">
            <template #default="{ row }">#{{ row.expense_id }} · {{ categoryLabel(row.original_category) }}</template>
          </el-table-column>
          <el-table-column prop="original_expense_date" label="原费用日期" min-width="112" />
          <el-table-column label="抵减金额" min-width="132" align="right">
            <template #default="{ row }">-{{ money(row.amount_cents) }}</template>
          </el-table-column>
          <el-table-column prop="reason" label="冲正原因" min-width="220" show-overflow-tooltip />
        </el-table>
        <el-empty v-else description="本期暂无冲正记录；费用未被冲正时这里为空" :image-size="72">
          <el-button type="primary" @click="scrollToId('expense-records')">查看已登记费用</el-button>
        </el-empty>
        <div v-if="selectedPeriod.expense_reversal_rows_truncated" class="table-footnote">本期冲正超过 500 笔，表格只显示最近的 500 条；汇总仍统计完整期间。</div>
        <div class="reversal-period-note">冲正只在其登记的发生期间体现抵减，不回溯重算原费用期间。</div>
      </DataPanel>
<DataPanel
        title="费用按类按月统计"
        description="按费用发生月 × 分类 × 固定/变动汇总；现金与经济两列可重叠（一条费用可同时是现金成本与经济成本），冲正按冲正日期单列到月，不并入分类行。"
        eyebrow="EXPENSE BY CATEGORY AND MONTH"
      >
        <div class="period-toolbar">
          <el-date-picker v-model="expenseStatsStartMonth" type="month" format="YYYY-MM" value-format="YYYY-MM" placeholder="开始月份" />
          <span>至</span>
          <el-date-picker v-model="expenseStatsEndMonth" type="month" format="YYYY-MM" value-format="YYYY-MM" placeholder="结束月份" />
          <el-button type="primary" plain :loading="expenseStatsLoading" @click="loadExpenseStats">查看统计</el-button>
          <span class="currency-hint">最多 12 个月 · 金额单位：人民币 CNY</span>
        </div>
        <el-table v-if="expenseStats?.rows?.length" :data="expenseStats.rows" :row-key="expenseStatRowKey" stripe class="expense-table">
          <el-table-column prop="month" label="月份" min-width="100" />
          <el-table-column label="类别" min-width="150">
            <template #default="{ row }">{{ categoryLabel(row.category) }}</template>
          </el-table-column>
          <el-table-column label="固定 / 变动" min-width="150">
            <template #default="{ row }">{{ expenseCostTypeLabel(row.cost_type) }}</template>
          </el-table-column>
          <el-table-column prop="expense_count" label="笔数" min-width="80" align="right" />
          <el-table-column label="现金成本" min-width="140" align="right">
            <template #default="{ row }">{{ money(row.cash_cents) }}</template>
          </el-table-column>
          <el-table-column label="经济成本" min-width="140" align="right">
            <template #default="{ row }">{{ money(row.economic_cents) }}</template>
          </el-table-column>
          <el-table-column label="合计" min-width="140" align="right">
            <template #default="{ row }">{{ money(row.total_cents) }}</template>
          </el-table-column>
        </el-table>
        <el-empty v-else-if="!expenseStatsLoading" description="所选期间暂无已登记费用" :image-size="72">
          <el-button type="primary" @click="scrollToId('expense-entry-form')">去登记费用</el-button>
        </el-empty>
        <div v-if="expenseStats?.reversal_cents_by_month && Object.keys(expenseStats.reversal_cents_by_month).length" class="table-footnote">
          当月冲正（按冲正日期归属）：
          <span v-for="(amount, month) in expenseStats.reversal_cents_by_month" :key="month">{{ month }} -{{ money(amount) }}；</span>
        </div>
        <div v-if="expenseStats?.note" class="table-footnote">{{ expenseStats.note }}</div>
      </DataPanel>
<DataPanel
        title="核算口径与当前结论"
        description="先把能对账的收入算准；没有凭证的数据保持未知，不以 0 代替。"
        eyebrow="ACCOUNTING BASIS"
      >
        <div class="basis-grid">
          <article class="basis-item income">
            <span class="basis-index">01 · 平台收入</span>
            <strong>系统实收与人工核验分开</strong>
            <p>{{ overview.revenue_basis }}</p>
          </article>
          <article class="basis-item cost">
            <span class="basis-index">02 · AI 成本</span>
            <strong>估算与实际账单分开</strong>
            <p>估算成本来自调用记录上存的估算成本累计（写入时按当时生效价或内置费率估算）；实际账单为超管手工录入的供应商真实金额。两者口径不同，差额仅供核对参考，不代表欠款或退款；2026-11-28 前的调用无门店归属，记为“未指定门店”。</p>
          </article>
          <article class="basis-item result">
            <span class="basis-index">03 · 利润结论</span>
            <strong>保持未完成状态</strong>
            <p>{{ overview.profit_status_note }}</p>
          </article>
        </div>
      </DataPanel>
<el-dialog v-model="reversalDialogVisible" title="登记费用冲正" width="min(560px, 94vw)" top="3vh" :close-on-click-modal="false">
        <template v-if="selectedExpense">
          <div class="reversal-source-card">
            <strong>原费用 #{{ selectedExpense.id }} · {{ categoryLabel(selectedExpense.category) }}</strong>
            <span>{{ selectedExpense.expense_date }} · {{ money(selectedExpense.amount_cents) }} · {{ selectedExpense.memo }}</span>
            <span>已冲正 {{ money(selectedExpense.reversed_amount_cents) }} · 剩余可冲 {{ money(selectedExpense.remaining_reversible_cents) }}</span>
          </div>
          <el-alert
            class="reversal-dialog-note"
            type="warning"
            show-icon
            :closable="false"
            title="将追加一条部分或全额冲正记录。原费用及原审计保留不变，抵减只按下方冲正日期计入该期间。"
          />
          <el-form label-position="top">
            <el-form-item label="本次冲正金额（元）" required>
              <el-input-number
                v-model="reversalForm.amountYuan"
                class="full-width"
                :min="0.01"
                :max="selectedExpense.remaining_reversible_cents / 100"
                :precision="2"
                :step="0.01"
                controls-position="right"
              />
              <div class="input-hint">不超过剩余可冲金额 {{ money(selectedExpense.remaining_reversible_cents) }}；可以分多次登记。</div>
            </el-form-item>
            <el-form-item label="冲正发生日期" required>
              <el-date-picker
                v-model="reversalForm.reversalDate"
                type="date"
                class="full-width"
                format="YYYY-MM-DD"
                value-format="YYYY-MM-DD"
                :disabled-date="disableFutureDate"
                placeholder="选择冲正发生日期"
              />
            </el-form-item>
            <el-form-item label="冲正原因" required>
              <el-input
                v-model="reversalForm.reason"
                type="textarea"
                :rows="2"
                maxlength="500"
                show-word-limit
                placeholder="说明为什么需要冲正，例如原费用录入错误并已核对凭证"
              />
            </el-form-item>
          </el-form>
        </template>
        <template #footer>
          <el-button @click="reversalDialogVisible = false">取消</el-button>
          <el-button type="warning" :loading="submittingReversal" @click="submitReversal">追加冲正记录</el-button>
        </template>
      </el-dialog>
      <div class="snapshot-footnote">数据刷新于 {{ formatDateTime(overview.generated_at) }}。该页面不包含商家顾客交易收入，也不代表银行或第三方支付渠道的独立对账单。</div>
    </template>
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
  loadOverview()
  loadExpenseStats()
})

// 空状态主操作按钮：回到本页的录入表单（豆包 §3.1 / 规范 §3.5）。
const scrollToId = (id) => {
  document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}
</script>

<style scoped src="./platform-cost.css"></style>
