<template>
  <div class="platform-cost-page">
    <PageHero
      eyebrow="AI BUDGET AND RISK CONTROL"
      title="AI 预算与风控"
      description="商家预算、预算预警与紧急关闭。达到预算后按所选策略处理，后端校验，不绕过。"
      compact
    >
      <template #actions>
        <el-tag effect="light" type="warning">仅超管可见</el-tag>
        <el-button type="primary" :loading="aiBudgetsLoading || alertsLoading || emergencyLoading" @click="loadBudgets(); loadBudgetAlerts(); loadEmergencySwitch()">刷新数据</el-button>
      </template>
    </PageHero>

    <el-skeleton v-if="!budgetsFirstLoaded" :rows="6" animated />

    <template v-else>
    <el-alert
          v-if="emergencySwitch?.disabled"
          class="ai-emergency-banner"
          type="error"
          show-icon
          :closable="false"
          title="AI 已由超管紧急关闭：所有商家 AI 调用会直接拒绝"
          :description="emergencySwitch?.switch?.reason ? `关闭原因：${emergencySwitch.switch.reason}` : '恢复后立即恢复调用。'"
        />

    <section class="summary-grid">
      <article class="summary-card">
        <div class="card-label">已设预算商家数</div>
        <strong>{{ aiBudgets.length }}</strong>
        <p>无单独预算的商家适用全局默认行；全局默认也未设置时不限额。</p>
      </article>
      <article class="summary-card">
        <div class="card-label">预算预警记录</div>
        <strong>{{ alertsTotal }}</strong>
        <p>预警在用量首次达到阈值时产生一条，不重复产生。</p>
      </article>
      <article class="summary-card" :class="emergencySwitch?.disabled ? 'warning' : ''">
        <div class="card-label">AI 当前状态</div>
        <strong>{{ emergencySwitch ? (emergencySwitch.disabled ? '已紧急关闭' : '正常开放') : '读取中' }}</strong>
        <p>{{ emergencySwitch?.switch?.reason ? `最近一次原因：${emergencySwitch.switch.reason}` : '关闭后所有商家 AI 调用会在入口直接拒绝。' }}</p>
      </article>
    </section>

    <DataPanel
      title="商家预算"
      description="每日 / 月度预算任一为 0 即该周期不限额。超额策略由后端在 AI 调用入口强制执行。"
      eyebrow="MERCHANT BUDGETS"
    >

            <div class="ai-form-note">
              <strong>0 表示不限额</strong>
              <span>每日 / 月度预算任一为 0 即该周期不限额。无单独预算的商家适用全局默认行；全局默认也未设置时不限额。超额策略由后端在 AI 调用入口强制执行。</span>
            </div>
            <el-form id="budget-form" class="ai-inline-form" label-position="top" @submit.prevent="submitBudget">
              <div class="form-row ai-budget-form-row">
                <el-form-item label="商家" required>
                  <el-select v-model="budgetForm.merchantId" class="full-width" placeholder="输入商家名搜索" filterable remote :remote-method="searchBudgetMerchants" :loading="budgetMerchantSearchLoading">
                    <el-option :value="0" label="全局默认（无单独预算的商家）" />
                    <el-option v-for="item in budgetMerchantOptions" :key="item.id" :label="item.name || `商家 #${item.id}`" :value="item.id" />
                  </el-select>
                </el-form-item>
                <el-form-item label="每日预算（元，0 不限额）" required>
                  <el-input-number v-model="budgetForm.dailyBudgetYuan" class="full-width" :min="0" :max="1000000000" :precision="2" :step="10" controls-position="right" />
                </el-form-item>
                <el-form-item label="月度预算（元，0 不限额）" required>
                  <el-input-number v-model="budgetForm.monthlyBudgetYuan" class="full-width" :min="0" :max="1000000000" :precision="2" :step="10" controls-position="right" />
                </el-form-item>
                <el-form-item label="超额策略" required>
                  <el-select v-model="budgetForm.overLimitAction" class="full-width">
                    <el-option v-for="item in overLimitActions" :key="item.value" :label="item.label" :value="item.value" />
                  </el-select>
                </el-form-item>
              </div>
              <div class="form-actions">
                <span class="audit-note">保存后立即生效；达到预算后按所选策略处理，后端校验，不绕过。</span>
                <el-button @click="resetBudgetForm">清空</el-button>
                <el-button type="primary" :loading="submittingBudget" @click="submitBudget">保存预算</el-button>
              </div>
            </el-form>
            <el-table v-if="aiBudgets.length" :data="aiBudgets" row-key="id" stripe class="expense-table ai-table-gap">
              <el-table-column label="商家" min-width="200">
                <template #default="{ row }">{{ budgetMerchantLabel(row.merchant_id, row.merchant_name) }}</template>
              </el-table-column>
              <el-table-column label="每日预算" min-width="130" align="right">
                <template #default="{ row }">{{ row.daily_budget_cents > 0 ? money(row.daily_budget_cents) : '不限额' }}</template>
              </el-table-column>
              <el-table-column label="月度预算" min-width="130" align="right">
                <template #default="{ row }">{{ row.monthly_budget_cents > 0 ? money(row.monthly_budget_cents) : '不限额' }}</template>
              </el-table-column>
              <el-table-column label="超额策略" min-width="150">
                <template #default="{ row }">{{ overLimitActionLabel(row.over_limit_action) }}</template>
              </el-table-column>
              <el-table-column label="更新时间" min-width="170">
                <template #default="{ row }">{{ formatDateTime(row.updated_at) }}</template>
              </el-table-column>
              <el-table-column label="操作" min-width="170" align="center">
                <template #default="{ row }">
                  <el-button type="primary" link @click="editBudget(row)">编辑</el-button>
                  <el-button v-if="row.merchant_id !== 0" type="primary" link @click="loadBudgetStatus(row.merchant_id)">查看状态</el-button>
                </template>
              </el-table-column>
            </el-table>
            <el-empty v-else description="暂无商家预算；未设置预算的商家不限额" :image-size="72">
              <el-button type="primary" @click="scrollToId('budget-form')">去设置预算</el-button>
            </el-empty>
            <div v-if="budgetStatus" class="ai-budget-status">
              <div class="ai-budget-status-head">
                <strong>{{ budgetMerchantLabel(budgetStatus.status.merchant_id, budgetStatus.status.merchant_name) }} · 当前预算状态</strong>
                <el-tag v-if="budgetStatus.status.global_default" type="info" effect="light">适用全局默认</el-tag>
                <el-tag v-if="budgetStatus.status.over_limit" type="danger" effect="light">已超预算</el-tag>
                <el-tag v-else-if="budgetStatus.status.has_budget" type="success" effect="light">预算内</el-tag>
                <el-tag v-else type="info" effect="light">未设置预算</el-tag>
              </div>
              <div class="ai-budget-status-grid">
                <article>
                  <span>今日已用（估算成本） / 每日预算</span>
                  <strong>{{ money(budgetStatus.status.daily_used_cost_cents) }} / {{ budgetStatus.status.daily_budget_cents > 0 ? money(budgetStatus.status.daily_budget_cents) : '不限额' }}</strong>
                  <el-progress v-if="budgetStatus.status.daily_budget_cents > 0" :percentage="Math.min(100, budgetStatus.status.daily_ratio_percent)" :status="budgetStatus.status.daily_ratio_percent >= 100 ? 'exception' : ''" />
                  <small>今日 {{ budgetStatus.status.daily_used_tokens || 0 }} Token</small>
                </article>
                <article>
                  <span>本月已用（估算成本） / 月度预算</span>
                  <strong>{{ money(budgetStatus.status.monthly_used_cost_cents) }} / {{ budgetStatus.status.monthly_budget_cents > 0 ? money(budgetStatus.status.monthly_budget_cents) : '不限额' }}</strong>
                  <el-progress v-if="budgetStatus.status.monthly_budget_cents > 0" :percentage="Math.min(100, budgetStatus.status.monthly_ratio_percent)" :status="budgetStatus.status.monthly_ratio_percent >= 100 ? 'exception' : ''" />
                  <small>本月 {{ budgetStatus.status.monthly_used_tokens || 0 }} Token</small>
                </article>
              </div>
              <small v-if="budgetStatus.status.has_budget" class="ai-budget-status-note">超额策略：{{ overLimitActionLabel(budgetStatus.status.effective_action) }}。已用金额为调用记录上的估算成本累计，与统计页、核对页同一口径。</small>
            </div>
              </DataPanel>

    <DataPanel
      title="预算预警"
      description="用量首次达到阈值时产生一条预警，不重复产生。"
      eyebrow="BUDGET ALERTS"
    >

            <div class="period-toolbar">
              <el-select v-model="alertFilter.merchantId" placeholder="输入商家名搜索" clearable filterable remote :remote-method="searchAlertMerchants" :loading="alertMerchantSearchLoading" class="ai-filter-select" @change="reloadBudgetAlerts">
                <el-option v-for="item in alertMerchantOptions" :key="item.id" :label="item.name || `商家 #${item.id}`" :value="item.id" />
              </el-select>
              <el-select v-model="alertFilter.threshold" placeholder="全部阈值" clearable class="ai-filter-select" @change="reloadBudgetAlerts">
                <el-option v-for="item in budgetAlertThresholds" :key="item" :label="`${item}%`" :value="item" />
              </el-select>
              <el-select v-model="alertFilter.periodType" placeholder="全部周期" clearable class="ai-filter-select" @change="reloadBudgetAlerts">
                <el-option value="daily" label="每日" />
                <el-option value="monthly" label="每月" />
              </el-select>
              <el-button type="primary" plain :loading="alertsLoading" @click="loadBudgetAlerts">刷新预警</el-button>
              <span class="currency-hint">预警在用量首次达到阈值时产生一条，不重复产生</span>
            </div>
            <el-table v-if="budgetAlerts.length" :data="budgetAlerts" row-key="id" stripe class="expense-table">
              <el-table-column label="商家" min-width="180">
                <template #default="{ row }">{{ merchantDisplayName(row.merchant_id, row.merchant_name) }}</template>
              </el-table-column>
              <el-table-column label="周期" min-width="90" align="center">
                <template #default="{ row }">{{ periodTypeLabel(row.period_type) }}</template>
              </el-table-column>
              <el-table-column label="周期开始" min-width="120">
                <template #default="{ row }">{{ row.period_start }}</template>
              </el-table-column>
              <el-table-column label="阈值" min-width="100" align="center">
                <template #default="{ row }">
                  <el-tag :type="row.threshold >= 100 ? 'danger' : row.threshold >= 90 ? 'warning' : 'info'" effect="light">{{ row.threshold }}%</el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="used_tokens" label="当时用量（Token）" min-width="150" align="right" />
              <el-table-column label="当时估算成本（估算）" min-width="160" align="right">
                <template #default="{ row }">{{ money(row.estimated_cost_cents) }}</template>
              </el-table-column>
              <el-table-column label="产生时间" min-width="170">
                <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
              </el-table-column>
            </el-table>
            <el-empty v-else description="所选期间内暂无预算预警；用量未达到任何预警阈值时这里为空" :image-size="72" />
            <div v-if="alertsTotal > 0" class="pager">
              <el-pagination
                background
                layout="total, sizes, prev, pager, next"
                :total="alertsTotal"
                :page-size="alertPageSize"
                :current-page="alertPage"
                :page-sizes="[10, 20, 50]"
                @size-change="alertPageSize = $event; alertPage = 1; loadBudgetAlerts()"
                @current-change="alertPage = $event; loadBudgetAlerts()"
              />
            </div>
              </DataPanel>

    <DataPanel
      title="紧急关闭"
      description="全平台紧急手段，变更会写入操作审计。日常限额请用商家预算与超额策略，不要长期依赖紧急关闭。"
      eyebrow="EMERGENCY SWITCH"
    >

            <div class="ai-emergency-panel">
              <div class="ai-emergency-status">
                <span>当前状态</span>
                <strong v-if="emergencySwitch?.disabled" class="ai-emergency-off">AI 已紧急关闭</strong>
                <strong v-else class="ai-emergency-on">AI 正常开放</strong>
                <small v-if="emergencySwitch?.switch?.reason">最近一次原因：{{ emergencySwitch.switch.reason }}</small>
                <small v-if="emergencySwitch?.switch?.updated_at">更新于 {{ formatDateTime(emergencySwitch.switch.updated_at) }}</small>
              </div>
              <el-alert
                class="ai-emergency-note"
                type="warning"
                show-icon
                :closable="false"
                title="关闭后，所有商家 AI 调用会在入口直接拒绝；恢复后立即恢复调用。"
                description="该开关是全平台紧急手段，变更会写入操作审计。日常限额请用商家预算与超额策略，不要长期依赖紧急关闭。"
              />
              <div class="form-actions">
                <span class="audit-note">切换前会二次确认，并要求填写原因（关闭时必填）。</span>
                <el-button
                  :type="emergencySwitch?.disabled ? 'success' : 'danger'"
                  :loading="submittingEmergency"
                  @click="openEmergencyDialog"
                >{{ emergencySwitch?.disabled ? '恢复 AI 调用' : '紧急关闭 AI' }}</el-button>
              </div>
            </div>
              </DataPanel>
    </template>

<el-dialog v-model="emergencyDialogVisible" :title="emergencyTargetDisabled ? '确认紧急关闭 AI' : '确认恢复 AI 调用'" width="min(560px, 94vw)" top="3vh" :close-on-click-modal="false">
        <el-alert
          v-if="emergencyTargetDisabled"
          type="error"
          show-icon
          :closable="false"
          title="关闭后，所有商家 AI 调用会直接拒绝，包括营销文案、分享复盘等全部功能。"
          description="请确认已通知相关商家或已有替代安排；恢复后立即恢复调用。"
        />
        <el-alert
          v-else
          type="success"
          show-icon
          :closable="false"
          title="恢复后，所有商家 AI 调用立即恢复（仍受商家预算与超额策略约束）。"
        />
        <el-form label-position="top" class="ai-dialog-form">
          <el-form-item :label="emergencyTargetDisabled ? '关闭原因（必填）' : '恢复原因（可选）'" :required="emergencyTargetDisabled">
            <el-input
              v-model="emergencyReason"
              type="textarea"
              :rows="3"
              maxlength="500"
              show-word-limit
              placeholder="例如：供应商账单异常，暂停 AI 调用排查"
            />
          </el-form-item>
        </el-form>
        <template #footer>
          <el-button @click="emergencyDialogVisible = false">取消</el-button>
          <el-button :type="emergencyTargetDisabled ? 'danger' : 'success'" :loading="submittingEmergency" @click="submitEmergencySwitch">确认{{ emergencyTargetDisabled ? '关闭' : '恢复' }}</el-button>
        </template>
      </el-dialog>  </div>
</template>

<script setup>
import DataPanel from '../../../components/design/DataPanel.vue'
import PageHero from '../../../components/design/PageHero.vue'
import { usePlatformCost } from './usePlatformCost.js'
import { onMounted, ref } from 'vue'

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
  aiBudgetsLoading,
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
  emergencyLoading,
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

// 首屏加载标记（视图本地状态，不进共享 composable）。
const budgetsFirstLoaded = ref(false)

onMounted(async () => {
  // 首屏三路数据齐了再出内容，之前只出骨架屏，避免 0 笔/暂无数据的空态误闪。
  await Promise.all([loadBudgets(), loadBudgetAlerts(), loadEmergencySwitch()])
  budgetsFirstLoaded.value = true
})

// 空状态主操作按钮：回到本页的预算表单（豆包 §3.1 / 规范 §3.5）。
const scrollToId = (id) => {
  document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}
</script>

<style scoped src="./platform-cost.css"></style>
