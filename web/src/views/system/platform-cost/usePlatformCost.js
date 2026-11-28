// 平台成本核算共享逻辑：6 个成本中心子页共用。
// 由 PlatformCostView.vue 的 <script setup> 原样迁移：接口、计算逻辑、权限口径均未改，
// 只去掉原页的两个页签状态（aiTab / financeTab）与整页统一加载，
// 各子页在 onMounted 里只加载自己需要的数据。
// 另补一个原文件遗漏的导入：loadVendorBills 用到的 fetchAdminAIVendorBills
// 在原文件里被引用却未导入，账单页签点“查看期间”会报 ReferenceError。
import { computed, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getPlatformCostPeriodValidationError } from '../../../utils/platformCostPeriod.js'
import { calculatePlatformCostScenario } from '../../../utils/platformCostScenario.js'
import {
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
} from '../../../utils/aiCostControl.js'
import {
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
} from '../../../api/modules'
import {
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
} from '../../../utils/platformFinance.js'

export function usePlatformCost() {
  const loading = ref(false)
  const submittingExpense = ref(false)
  const submittingReversal = ref(false)
  const reversalDialogVisible = ref(false)
  const selectedExpense = ref(null)
  const overview = ref(null)
  const error = ref('')
  const today = new Date()
  const todayString = localDateString(today)
  const currentMonthString = todayString.slice(0, 7)
  const monthStart = new Date(today.getFullYear(), today.getMonth(), 1)
  const dateRange = ref([localDateString(monthStart), todayString])
  const emptyScenarioAssumptions = () => ({
    monthlyAverageSubscriptionRevenuePerMerchantYuan: null,
    payingMerchantCount: null,
    fixedMonthlyCostsYuan: null,
    variableCostPerMerchantYuan: null,
  })
  const scenarioAssumptions = ref(emptyScenarioAssumptions())
  const expenseCategories = [
    { value: 'cloud_hosting', label: '云服务 / 主机' },
    { value: 'messaging_api', label: '短信 / 消息 / API' },
    { value: 'labor_contractor', label: '人员 / 外包' },
    { value: 'customer_acquisition_travel', label: '获客 / 差旅' },
    { value: 'equipment', label: '设备' },
    { value: 'ai_services', label: 'AI 服务' },
    { value: 'third_party_services', label: '第三方服务' },
    { value: 'other', label: '其他' },
  ]

  const newIdempotencyKey = () => {
    if (globalThis.crypto?.randomUUID) return globalThis.crypto.randomUUID()
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (char) => {
      const value = Math.floor(Math.random() * 16)
      return (char === 'x' ? value : (value & 0x3) | 0x8).toString(16)
    })
  }
  const expenseForm = ref({
    category: '',
    expenseDate: todayString,
    amountYuan: null,
    memo: '',
    idempotencyKey: newIdempotencyKey(),
    costType: '',
    vendor: '',
    paymentStatus: 'paid',
    isRecurring: false,
    isCashCost: true,
    isEconomicCost: true,
  })
  const reversalForm = ref({ reversalDate: todayString, amountYuan: null, reason: '', idempotencyKey: newIdempotencyKey() })
  const selectedPeriod = computed(() => overview.value?.selected_period || {})

  // --- AI 成本控制（P1 MVP）---
  const aiUsageDimensions = AI_USAGE_DIMENSIONS
  const overLimitActions = OVER_LIMIT_ACTIONS
  const budgetAlertThresholds = BUDGET_ALERT_THRESHOLDS
  const aiStats = ref(null)
  const aiStatsLoading = ref(false)
  const aiStatsError = ref('')
  const aiStatsDimension = ref('merchant')
  const aiDateRange = ref([localDateString(new Date(today.getFullYear(), today.getMonth(), today.getDate() - 29)), todayString])
  const modelPrices = ref([])
  const modelPricesLoading = ref(false)
  const submittingPrice = ref(false)
  const priceForm = ref({
    model: '',
    provider: '',
    inputPriceYuan: null,
    outputPriceYuan: null,
    cachedPriceYuan: null,
    effectiveFrom: `${todayString} 00:00:00`,
    isActive: true,
  })
  const vendorBills = ref([])
  const reconciliation = ref(null)
  const reconciliationLoading = ref(false)
  const submittingBill = ref(false)
  const billForm = ref({ vendor: '', period: [localDateString(new Date(today.getFullYear(), today.getMonth(), 1)), todayString], amountYuan: null, note: '' })
  const aiBudgets = ref([])
  // 下拉框不再一次性拉商家全量（后端会把 page_size 压成 10，旧做法只剩最新 10 家）：
  // 预算表/预警表的名字由后端随行返回；两个下拉框改为远程搜索，按关键字取。
  const budgetMerchantOptions = ref([])
  const alertMerchantOptions = ref([])
  const budgetMerchantSearchLoading = ref(false)
  const alertMerchantSearchLoading = ref(false)
  // 已选商家的 id→名缓存：远程搜索换一批候选后，已选项仍要显示真名而不是编号。
  const merchantNameCache = ref({})
  let budgetMerchantSearchTimer = null
  let alertMerchantSearchTimer = null
  const submittingBudget = ref(false)
  const budgetForm = ref({ merchantId: 0, dailyBudgetYuan: 0, monthlyBudgetYuan: 0, overLimitAction: 'stop' })
  const budgetStatus = ref(null)
  const budgetAlerts = ref([])
  const alertsLoading = ref(false)
  const alertPage = ref(1)
  const alertPageSize = ref(20)
  const alertsTotal = ref(0)
  const alertFilter = ref({ merchantId: '', threshold: '', periodType: '' })
  const emergencySwitch = ref(null)
  const submittingEmergency = ref(false)
  const emergencyDialogVisible = ref(false)
  const emergencyTargetDisabled = ref(false)
  const emergencyReason = ref('')

  const aiDimensionLabel = computed(() => aiUsageDimensions.find((item) => item.value === aiStatsDimension.value)?.label || '按商家')
  const aiStatsRows = computed(() => {
    if (!aiStats.value) return []
    const dimension = aiStatsDimension.value
    const rows = dimension === 'store'
      ? aiStats.value.by_store || []
      : dimension === 'model'
        ? aiStats.value.by_model || []
        : dimension === 'scenario'
          ? aiStats.value.by_scenario || []
          : aiStats.value.by_merchant || []
    return rows.map((row, index) => {
      if (dimension === 'store') return { ...row, rowKey: `store-${row.store_id || 0}`, displayName: storeDisplayName(row.store_id, row.store_name) }
      if (dimension === 'model') return { ...row, rowKey: `model-${row.model || index}`, displayName: row.model || '未注明' }
      if (dimension === 'scenario') return { ...row, rowKey: `scenario-${row.scenario || index}`, displayName: scenarioLabel(row.scenario) }
      return { ...row, rowKey: `merchant-${row.merchant_id}`, displayName: merchantDisplayName(row.merchant_id, row.merchant_name) }
    })
  })
  const scenarioCalculation = computed(() => {
    const values = Object.values(scenarioAssumptions.value)
    if (values.some((value) => value === null || typeof value !== 'number' || !Number.isFinite(value))) return null
    try {
      return { result: calculatePlatformCostScenario(scenarioAssumptions.value) }
    } catch (err) {
      return { error: err?.message || '当前假设超出安全计算范围，请调小数值。' }
    }
  })
  const monthLabel = computed(() => {
    if (!overview.value?.current_month_start) return '本月'
    return new Intl.DateTimeFormat('zh-CN', { year: 'numeric', month: 'long' }).format(new Date(overview.value.current_month_start))
  })

  function localDateString(date) {
    const year = date.getFullYear()
    const month = String(date.getMonth() + 1).padStart(2, '0')
    const day = String(date.getDate()).padStart(2, '0')
    return `${year}-${month}-${day}`
  }

  const money = (cents) => {
    if (cents === null || cents === undefined) return '暂不可核算'
    return new Intl.NumberFormat('zh-CN', { style: 'currency', currency: 'CNY' }).format(Number(cents) / 100)
  }
  const manualSummaryAmount = (summary) => summary && Number.isFinite(Number(summary.attested_amount_cents))
    ? money(summary.attested_amount_cents)
    : '数据不可用'
  const manualSummaryCount = (summary) => summary && Number.isFinite(Number(summary.review_count))
    ? summary.review_count
    : '数据不可用'
  const categoryLabel = (value) => expenseCategories.find((item) => item.value === value)?.label || value || '其他'
  const formatDateTime = (value) => {
    if (!value) return '—'
    const date = new Date(value)
    if (Number.isNaN(date.getTime())) return '—'
    return new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short' }).format(date)
  }
  const disableFutureDate = (date) => date.getTime() > new Date(`${todayString}T23:59:59`).getTime()
  const resetScenario = () => { scenarioAssumptions.value = emptyScenarioAssumptions() }

  const loadOverview = async () => {
    error.value = ''
    const periodError = getPlatformCostPeriodValidationError(dateRange.value?.[0], dateRange.value?.[1], todayString)
    if (periodError) {
      error.value = periodError
      ElMessage.warning(periodError)
      return
    }
    loading.value = true
    try {
      const params = dateRange.value?.length === 2
        ? { start_date: dateRange.value[0], end_date: dateRange.value[1] }
        : undefined
      const response = await fetchAdminPlatformCostOverview(params)
      overview.value = response?.data || null
      if (!overview.value) throw new Error('核算数据暂时不可用')
    } catch (err) {
      error.value = err?.response?.data?.message || err?.message || '加载平台核算数据失败'
      ElMessage.error(error.value)
    } finally {
      loading.value = false
    }
  }

  const submitExpense = async () => {
    if (!expenseForm.value.category || !expenseForm.value.costType || !expenseForm.value.expenseDate || !expenseForm.value.memo.trim()) {
      ElMessage.warning('请补全费用类别、固定/变动、发生日期和简短备注')
      return
    }
    if (!expenseForm.value.isCashCost && !expenseForm.value.isEconomicCost) {
      ElMessage.warning('现金成本与经济成本至少选一项')
      return
    }
    const amountCents = Math.round(Number(expenseForm.value.amountYuan) * 100)
    if (!Number.isSafeInteger(amountCents) || amountCents < 1 || amountCents > 100000000000) {
      ElMessage.warning('金额必须为正数，且不超过单笔上限')
      return
    }
    submittingExpense.value = true
    try {
      await createAdminPlatformExpense({
        category: expenseForm.value.category,
        amount_cents: amountCents,
        currency: 'CNY',
        expense_date: expenseForm.value.expenseDate,
        memo: expenseForm.value.memo.trim(),
        idempotency_key: expenseForm.value.idempotencyKey,
        cost_type: expenseForm.value.costType,
        vendor: expenseForm.value.vendor.trim(),
        payment_status: expenseForm.value.paymentStatus,
        is_recurring: expenseForm.value.isRecurring,
        is_cash_cost: expenseForm.value.isCashCost,
        is_economic_cost: expenseForm.value.isEconomicCost,
      })
      ElMessage.success('实际费用已登记')
      expenseForm.value = { ...expenseForm.value, category: '', amountYuan: null, memo: '', idempotencyKey: newIdempotencyKey() }
      await loadOverview()
      await loadExpenseStats()
    } catch (err) {
      ElMessage.error(err?.response?.data?.message || '费用未保存，请检查数据后重试')
    } finally {
      submittingExpense.value = false
    }
  }

  // ---- 平台财务 v14：费用统计、时间成本、收入台账、MRR 与保本 ----
  const expenseCostTypes = EXPENSE_COST_TYPES
  const expensePaymentStatuses = EXPENSE_PAYMENT_STATUSES
  const timeLogWorkTypes = TIME_LOG_WORK_TYPES
  const incomeTypes = INCOME_TYPES
  const incomeStatuses = INCOME_STATUSES

  const expenseStatsStartMonth = ref(currentMonthString)
  const expenseStatsEndMonth = ref(currentMonthString)
  const expenseStats = ref(null)
  const expenseStatsLoading = ref(false)

  const expenseStatRowKey = (row) => `${row.month}|${row.category}|${row.cost_type || ''}`

  const loadExpenseStats = async () => {
    if (!expenseStatsStartMonth.value || !expenseStatsEndMonth.value) return
    if (expenseStatsStartMonth.value > expenseStatsEndMonth.value) {
      ElMessage.warning('开始月份不能晚于结束月份')
      return
    }
    expenseStatsLoading.value = true
    try {
      const response = await fetchAdminPlatformExpenseStats({ start_month: expenseStatsStartMonth.value, end_month: expenseStatsEndMonth.value })
      expenseStats.value = response?.data || null
    } catch (err) {
      ElMessage.error(err?.response?.data?.message || '加载费用统计失败')
    } finally {
      expenseStatsLoading.value = false
    }
  }

  const timeLogList = ref(null)
  const submittingTimeLog = ref(false)
  const timeLogForm = ref({
    logDate: todayString,
    workType: '',
    content: '',
    hours: null,
    referenceHourlyValueYuan: null,
    module: '',
  })

  const loadTimeLogs = async () => {
    try {
      const response = await fetchAdminPlatformTimeLogs({ limit: 50 })
      timeLogList.value = response?.data || null
    } catch (err) {
      ElMessage.error(err?.response?.data?.message || '加载时间成本失败')
    }
  }

  const submitTimeLog = async () => {
    if (!timeLogForm.value.logDate || !timeLogForm.value.workType || !timeLogForm.value.content.trim()) {
      ElMessage.warning('请补全日期、工作类型和内容')
      return
    }
    const hours = Number(timeLogForm.value.hours)
    const referenceCents = Math.round(Number(timeLogForm.value.referenceHourlyValueYuan) * 100)
    if (!Number.isFinite(hours) || hours <= 0 || hours > 24) {
      ElMessage.warning('小时数需在 0.01–24 之间')
      return
    }
    if (!Number.isSafeInteger(referenceCents) || referenceCents < 1) {
      ElMessage.warning('参考小时价值必须为正数')
      return
    }
    submittingTimeLog.value = true
    try {
      await createAdminPlatformTimeLog({
        log_date: timeLogForm.value.logDate,
        work_type: timeLogForm.value.workType,
        content: timeLogForm.value.content.trim(),
        hours,
        reference_hourly_value_cents: referenceCents,
        module: timeLogForm.value.module.trim(),
        idempotency_key: newUUID(),
      })
      ElMessage.success('时间成本已登记（仅经济成本）')
      timeLogForm.value = { ...timeLogForm.value, content: '', hours: null, module: '' }
      await loadTimeLogs()
    } catch (err) {
      ElMessage.error(err?.response?.data?.message || '时间成本未保存，请检查数据后重试')
    } finally {
      submittingTimeLog.value = false
    }
  }

  const incomeList = ref(null)
  const submittingIncome = ref(false)
  const incomeForm = ref({
    incomeType: '',
    amountYuan: null,
    occurredDate: todayString,
    status: 'ordered',
    receivedDate: todayString,
    merchantId: null,
    billingPeriodMonths: 1,
    subscriptionStartDate: '',
    subscriptionEndDate: '',
    note: '',
  })

  const isSubscriptionIncomeType = (type) => type === 'subscription_basic' || type === 'subscription_premium'

  const loadIncomeRecords = async () => {
    try {
      const response = await fetchAdminPlatformIncomeRecords({ limit: 50 })
      incomeList.value = response?.data || null
    } catch (err) {
      ElMessage.error(err?.response?.data?.message || '加载收入台账失败')
    }
  }

  const submitIncome = async () => {
    if (!incomeForm.value.incomeType || !incomeForm.value.occurredDate) {
      ElMessage.warning('请补全收入类型与发生日期')
      return
    }
    const amountCents = Math.round(Number(incomeForm.value.amountYuan) * 100)
    if (!Number.isSafeInteger(amountCents) || amountCents < 1 || amountCents > 100000000000) {
      ElMessage.warning('金额必须为正数，且不超过单笔上限')
      return
    }
    if (incomeForm.value.status === 'received' && !incomeForm.value.receivedDate) {
      ElMessage.warning('已收款状态需要填收款日期')
      return
    }
    const isSubscription = isSubscriptionIncomeType(incomeForm.value.incomeType)
    if (isSubscription && (!incomeForm.value.subscriptionStartDate || !incomeForm.value.subscriptionEndDate)) {
      ElMessage.warning('订阅类收入必须填订阅起止日期')
      return
    }
    submittingIncome.value = true
    try {
      const payload = {
        income_type: incomeForm.value.incomeType,
        amount_cents: amountCents,
        currency: 'CNY',
        occurred_date: incomeForm.value.occurredDate,
        status: incomeForm.value.status,
        billing_period_months: incomeForm.value.billingPeriodMonths,
        note: incomeForm.value.note.trim(),
        idempotency_key: newUUID(),
      }
      if (incomeForm.value.status === 'received') payload.received_date = incomeForm.value.receivedDate
      if (incomeForm.value.merchantId) payload.merchant_id = incomeForm.value.merchantId
      if (isSubscription) {
        payload.subscription_start_date = incomeForm.value.subscriptionStartDate
        payload.subscription_end_date = incomeForm.value.subscriptionEndDate
      }
      await createAdminPlatformIncomeRecord(payload)
      ElMessage.success('收入已登记')
      incomeForm.value = { ...incomeForm.value, incomeType: '', amountYuan: null, note: '', subscriptionStartDate: '', subscriptionEndDate: '' }
      await loadIncomeRecords()
    } catch (err) {
      ElMessage.error(err?.response?.data?.message || '收入未保存，请检查数据后重试')
    } finally {
      submittingIncome.value = false
    }
  }

  const incomeStatusTagType = (status) => {
    if (status === 'received') return 'success'
    if (status === 'refunded') return 'danger'
    if (status === 'refunding' || status === 'pending_receipt') return 'warning'
    return 'info'
  }

  const incomeDialogVisible = ref(false)
  const incomeDialogRecord = ref(null)
  const incomeDialog = ref({ targetStatus: '', receivedDate: todayString, refundAmountYuan: null, note: '' })
  const submittingIncomeUpdate = ref(false)
  const incomeDialogTargets = computed(() => (incomeDialogRecord.value ? incomeNextStatuses(incomeDialogRecord.value.status) : []))

  const openIncomeDialog = (row) => {
    incomeDialogRecord.value = row
    incomeDialog.value = { targetStatus: incomeNextStatuses(row.status)[0] || '', receivedDate: todayString, refundAmountYuan: null, note: '' }
    incomeDialogVisible.value = true
  }

  const submitIncomeUpdate = async () => {
    const record = incomeDialogRecord.value
    if (!record || !incomeDialog.value.targetStatus) return
    const payload = { status: incomeDialog.value.targetStatus }
    if (incomeDialog.value.targetStatus === 'received') {
      if (!incomeDialog.value.receivedDate) {
        ElMessage.warning('请填收款日期')
        return
      }
      payload.received_date = incomeDialog.value.receivedDate
    }
    if (incomeDialog.value.targetStatus === 'refunding' || incomeDialog.value.targetStatus === 'refunded') {
      const refundCents = Math.round(Number(incomeDialog.value.refundAmountYuan) * 100)
      if (!Number.isSafeInteger(refundCents) || refundCents < 1 || refundCents > record.amount_cents) {
        ElMessage.warning('退款金额必须大于 0 且不超过收入金额')
        return
      }
      payload.refund_amount_cents = refundCents
    }
    if (incomeDialog.value.note.trim()) payload.note = incomeDialog.value.note.trim()
    submittingIncomeUpdate.value = true
    try {
      await updateAdminPlatformIncomeRecord(record.id, payload)
      ElMessage.success('收入状态已更新')
      incomeDialogVisible.value = false
      await loadIncomeRecords()
    } catch (err) {
      ElMessage.error(err?.response?.data?.message || '状态更新失败')
    } finally {
      submittingIncomeUpdate.value = false
    }
  }

  const cancelIncomeRecord = async (row) => {
    try {
      await ElMessageBox.confirm(`取消收入 #${row.id}（${incomeTypeLabel(row.income_type)} · ${money(row.amount_cents)}）后将不再计入 MRR，该操作会写入审计。`, '确认取消收入', { confirmButtonText: '确认取消', cancelButtonText: '返回', type: 'warning' })
    } catch {
      return
    }
    try {
      await updateAdminPlatformIncomeRecord(row.id, { cancel: true })
      ElMessage.success('收入已取消')
      await loadIncomeRecords()
    } catch (err) {
      ElMessage.error(err?.response?.data?.message || '取消失败')
    }
  }

  const mrrMonth = ref(currentMonthString)
  const mrr = ref(null)
  const mrrLoading = ref(false)
  const breakevenForm = ref({ revenueYuan: null, variableYuan: null })
  const breakeven = ref(null)
  const breakevenLoading = ref(false)

  const loadMRR = async () => {
    if (!mrrMonth.value) return
    mrrLoading.value = true
    try {
      const response = await fetchAdminPlatformMRR({ month: mrrMonth.value })
      mrr.value = response?.data || null
    } catch (err) {
      ElMessage.error(err?.response?.data?.message || '加载 MRR 失败')
    } finally {
      mrrLoading.value = false
    }
  }

  const runBreakeven = async () => {
    if (!mrrMonth.value) return
    const params = { month: mrrMonth.value }
    if (breakevenForm.value.revenueYuan != null && breakevenForm.value.revenueYuan !== '') {
      // 渲染验证发现：后端只认 merchant_monthly_*，之前这里发的 avg_* 参数名
      // 被后端忽略，保本试算永远缺参数；改发规范名（后端仍兼容旧名）。
      params.merchant_monthly_revenue_cents = Math.round(Number(breakevenForm.value.revenueYuan) * 100)
    }
    if (breakevenForm.value.variableYuan != null && breakevenForm.value.variableYuan !== '') {
      params.merchant_monthly_variable_cents = Math.round(Number(breakevenForm.value.variableYuan) * 100)
    }
    breakevenLoading.value = true
    try {
      const response = await fetchAdminPlatformBreakeven(params)
      breakeven.value = response?.data || null
    } catch (err) {
      ElMessage.error(err?.response?.data?.message || '保本试算失败')
    } finally {
      breakevenLoading.value = false
    }
  }

  const openReversal = (expense) => {
    selectedExpense.value = expense
    reversalForm.value = {
      reversalDate: todayString,
      amountYuan: Number(expense.remaining_reversible_cents) / 100,
      reason: '',
      idempotencyKey: newIdempotencyKey(),
    }
    reversalDialogVisible.value = true
  }

  const submitReversal = async () => {
    if (!selectedExpense.value || !reversalForm.value.reversalDate || !reversalForm.value.reason.trim()) {
      ElMessage.warning('请补全冲正金额、发生日期和原因')
      return
    }
    const amountCents = Math.round(Number(reversalForm.value.amountYuan) * 100)
    if (!Number.isSafeInteger(amountCents) || amountCents < 1 || amountCents > selectedExpense.value.remaining_reversible_cents) {
      ElMessage.warning('本次金额必须为正数且不超过剩余可冲金额')
      return
    }
    submittingReversal.value = true
    try {
      await createAdminPlatformExpenseReversal(selectedExpense.value.id, {
        reversal_date: reversalForm.value.reversalDate,
        amount_cents: amountCents,
        reason: reversalForm.value.reason.trim(),
        idempotency_key: reversalForm.value.idempotencyKey,
      })
      ElMessage.success('冲正记录已追加；原费用保持不变')
      reversalDialogVisible.value = false
      selectedExpense.value = null
      await loadOverview()
    } catch (err) {
      ElMessage.error(err?.response?.data?.message || '冲正未保存，请检查记录后重试')
    } finally {
      submittingReversal.value = false
    }
  }

  const aiPeriodParams = () => (aiDateRange.value?.length === 2
    ? { start_date: aiDateRange.value[0], end_date: aiDateRange.value[1] }
    : undefined)
  const aiPeriodError = () => getPlatformCostPeriodValidationError(aiDateRange.value?.[0], aiDateRange.value?.[1], todayString)

  const loadAiUsageStats = async () => {
    aiStatsError.value = ''
    const periodError = aiPeriodError()
    if (periodError) {
      aiStatsError.value = periodError
      ElMessage.warning(periodError)
      return
    }
    aiStatsLoading.value = true
    try {
      const response = await fetchAdminAIUsageStats(aiPeriodParams())
      aiStats.value = response?.data || null
      if (!aiStats.value) throw new Error('AI 用量统计暂不可用')
    } catch (err) {
      aiStatsError.value = err?.response?.data?.message || err?.message || '加载 AI 用量统计失败'
      ElMessage.error(aiStatsError.value)
    } finally {
      aiStatsLoading.value = false
    }
  }

  const loadModelPrices = async () => {
    modelPricesLoading.value = true
    try {
      const response = await fetchAdminAIModelPrices()
      modelPrices.value = response?.data || []
    } catch (err) {
      ElMessage.error(err?.response?.data?.message || '加载模型价格表失败')
    } finally {
      modelPricesLoading.value = false
    }
  }

  const submitModelPrice = async () => {
    const inputCents = yuanToCents(priceForm.value.inputPriceYuan)
    const outputCents = yuanToCents(priceForm.value.outputPriceYuan)
    const cachedCents = yuanToCents(priceForm.value.cachedPriceYuan)
    if (!priceForm.value.model.trim() || !priceForm.value.effectiveFrom) {
      ElMessage.warning('请填写模型与生效时间')
      return
    }
    if (inputCents === null || outputCents === null || cachedCents === null) {
      ElMessage.warning('三项单价必须为不小于 0 的金额（元 / 百万 Token）')
      return
    }
    submittingPrice.value = true
    try {
      await saveAdminAIModelPrice({
        model: priceForm.value.model.trim(),
        provider: priceForm.value.provider.trim(),
        input_price_cents_per_million: inputCents,
        output_price_cents_per_million: outputCents,
        cached_price_cents_per_million: cachedCents,
        currency: 'CNY',
        effective_from: priceForm.value.effectiveFrom,
        is_active: priceForm.value.isActive,
      })
      ElMessage.success('模型价格已保存')
      priceForm.value = { model: '', provider: '', inputPriceYuan: null, outputPriceYuan: null, cachedPriceYuan: null, effectiveFrom: `${todayString} 00:00:00`, isActive: true }
      await loadModelPrices()
    } catch (err) {
      ElMessage.error(err?.response?.data?.message || '模型价格未保存，请检查数据后重试')
    } finally {
      submittingPrice.value = false
    }
  }

  const toggleModelPrice = async (row) => {
    try {
      await ElMessageBox.confirm(
        row.is_active ? `停用 ${row.model}（${row.provider || '未指定供应商'}）${localDateTimeString(new Date(row.effective_from))} 生效的价格？停用后新调用不再按该价估算。` : `重新启用 ${row.model} 的该条价格？`,
        row.is_active ? '确认停用模型价格' : '确认启用模型价格',
        { confirmButtonText: '确认', cancelButtonText: '取消', type: 'warning' },
      )
    } catch {
      return
    }
    try {
      await saveAdminAIModelPrice({
        model: row.model,
        provider: row.provider || '',
        input_price_cents_per_million: row.input_price_cents_per_million,
        output_price_cents_per_million: row.output_price_cents_per_million,
        cached_price_cents_per_million: row.cached_price_cents_per_million,
        currency: 'CNY',
        effective_from: localDateTimeString(new Date(row.effective_from)),
        is_active: !row.is_active,
      })
      ElMessage.success(row.is_active ? '模型价格已停用' : '模型价格已启用')
      await loadModelPrices()
    } catch (err) {
      ElMessage.error(err?.response?.data?.message || '模型价格状态未更新')
    }
  }

  const loadVendorBills = async () => {
    const periodError = aiPeriodError()
    if (periodError) {
      ElMessage.warning(periodError)
      return
    }
    reconciliationLoading.value = true
    try {
      const params = aiPeriodParams()
      const [billsResponse, reconciliationResponse] = await Promise.all([
        fetchAdminAIVendorBills(params),
        fetchAdminAIBillReconciliation(params),
      ])
      vendorBills.value = billsResponse?.data || []
      reconciliation.value = reconciliationResponse?.data || null
    } catch (err) {
      ElMessage.error(err?.response?.data?.message || '加载供应商账单失败')
    } finally {
      reconciliationLoading.value = false
    }
  }

  const submitVendorBill = async () => {
    const amountCents = yuanToCents(billForm.value.amountYuan)
    if (!billForm.value.vendor.trim() || billForm.value.period?.length !== 2) {
      ElMessage.warning('请填写供应商与账单周期')
      return
    }
    if (amountCents === null || amountCents < 1) {
      ElMessage.warning('实际账单金额必须为正数')
      return
    }
    submittingBill.value = true
    try {
      const response = await createAdminAIVendorBill({
        vendor: billForm.value.vendor.trim(),
        period_start: billForm.value.period[0],
        period_end: billForm.value.period[1],
        amount_cents: amountCents,
        currency: 'CNY',
        note: billForm.value.note.trim(),
        idempotency_key: newUUID(),
      })
      ElMessage.success(response?.data?.created === false ? '该账单已存在，未重复录入' : '供应商实际账单已录入')
      billForm.value = { vendor: '', period: [localDateString(new Date(today.getFullYear(), today.getMonth(), 1)), todayString], amountYuan: null, note: '' }
      await loadVendorBills()
    } catch (err) {
      ElMessage.error(err?.response?.data?.message || '账单未保存，请检查数据后重试')
    } finally {
      submittingBill.value = false
    }
  }

  const searchMerchantsRemote = async (keyword, currentId, loadingRef, optionsRef) => {
    const query = String(keyword || '').trim()
    if (!query) {
      // 空关键字不发请求：只保留已选项，避免无意义地拉最新 10 家。
      const selected = currentId && merchantNameCache.value[currentId]
        ? [{ id: currentId, name: merchantNameCache.value[currentId] }]
        : []
      optionsRef.value = selected
      return
    }
    loadingRef.value = true
    try {
      const response = await fetchMerchants({ page: 1, page_size: 20, keyword: query })
      const list = response?.data?.list || []
      list.forEach((item) => { merchantNameCache.value[item.id] = item.name })
      // 已选项不在本批结果里时补回，否则选中态只剩编号。
      if (currentId && !list.some((item) => item.id === currentId) && merchantNameCache.value[currentId]) {
        list.unshift({ id: currentId, name: merchantNameCache.value[currentId] })
      }
      optionsRef.value = list
    } catch (err) {
      ElMessage.error(err?.response?.data?.message || '搜索商家失败')
    } finally {
      loadingRef.value = false
    }
  }

  const searchBudgetMerchants = (query) => {
    clearTimeout(budgetMerchantSearchTimer)
    budgetMerchantSearchTimer = setTimeout(() => {
      searchMerchantsRemote(query, budgetForm.value.merchantId, budgetMerchantSearchLoading, budgetMerchantOptions)
    }, 300)
  }

  const searchAlertMerchants = (query) => {
    clearTimeout(alertMerchantSearchTimer)
    alertMerchantSearchTimer = setTimeout(() => {
      searchMerchantsRemote(query, alertFilter.value.merchantId, alertMerchantSearchLoading, alertMerchantOptions)
    }, 300)
  }

  const loadBudgets = async () => {
    try {
      const response = await fetchAdminAIBudgets()
      aiBudgets.value = response?.data || []
    } catch (err) {
      ElMessage.error(err?.response?.data?.message || '加载商家预算失败')
    }
  }

  const resetBudgetForm = () => {
    budgetForm.value = { merchantId: 0, dailyBudgetYuan: 0, monthlyBudgetYuan: 0, overLimitAction: 'stop' }
  }

  const editBudget = (row) => {
    budgetForm.value = {
      merchantId: row.merchant_id,
      dailyBudgetYuan: centsToYuan(row.daily_budget_cents),
      monthlyBudgetYuan: centsToYuan(row.monthly_budget_cents),
      overLimitAction: row.over_limit_action,
    }
    // 被编辑的商家可能不在当前搜索候选里（例如老商家）：把行里后端给的真名
    // 补进下拉框，否则选中态只剩“商家 #id”。
    if (row.merchant_id) {
      if (row.merchant_name) merchantNameCache.value[row.merchant_id] = row.merchant_name
      budgetMerchantOptions.value = [
        { id: row.merchant_id, name: row.merchant_name || merchantNameCache.value[row.merchant_id] || '' },
        ...budgetMerchantOptions.value.filter((item) => item.id !== row.merchant_id),
      ]
    }
    budgetStatus.value = null
  }

  const submitBudget = async () => {
    if (budgetForm.value.merchantId === null || budgetForm.value.merchantId === undefined || budgetForm.value.merchantId === '') {
      ElMessage.warning('请选择商家或全局默认')
      return
    }
    const dailyCents = yuanToCents(budgetForm.value.dailyBudgetYuan)
    const monthlyCents = yuanToCents(budgetForm.value.monthlyBudgetYuan)
    if (dailyCents === null || monthlyCents === null) {
      ElMessage.warning('每日 / 月度预算必须为不小于 0 的金额，0 表示不限额')
      return
    }
    submittingBudget.value = true
    try {
      await saveAdminAIBudget(budgetForm.value.merchantId, {
        daily_budget_cents: dailyCents,
        monthly_budget_cents: monthlyCents,
        over_limit_action: budgetForm.value.overLimitAction,
      })
      ElMessage.success('商家预算已保存，立即生效')
      await loadBudgets()
    } catch (err) {
      ElMessage.error(err?.response?.data?.message || '预算未保存，请检查数据后重试')
    } finally {
      submittingBudget.value = false
    }
  }

  const loadBudgetStatus = async (merchantId) => {
    try {
      const response = await fetchAdminAIBudgetStatus(merchantId)
      budgetStatus.value = response?.data || null
      if (!budgetStatus.value) throw new Error('商家预算状态暂不可用')
    } catch (err) {
      ElMessage.error(err?.response?.data?.message || err?.message || '加载商家预算状态失败')
    }
  }

  const loadBudgetAlerts = async () => {
    alertsLoading.value = true
    try {
      const params = { ...aiPeriodParams(), page: alertPage.value, page_size: alertPageSize.value }
      if (alertFilter.value.merchantId !== '' && alertFilter.value.merchantId !== null) params.merchant_id = alertFilter.value.merchantId
      if (alertFilter.value.threshold !== '' && alertFilter.value.threshold !== null) params.threshold = alertFilter.value.threshold
      if (alertFilter.value.periodType) params.period_type = alertFilter.value.periodType
      const response = await fetchAdminAIBudgetAlerts(params)
      const data = response?.data
      // 服务端已分页：{list, total}；旧后端若仍返回裸数组则按全量回落。
      budgetAlerts.value = Array.isArray(data) ? data : (data?.list || [])
      alertsTotal.value = Array.isArray(data) ? data.length : (data?.total ?? 0)
    } catch (err) {
      ElMessage.error(err?.response?.data?.message || '加载预算预警失败')
    } finally {
      alertsLoading.value = false
    }
  }

  // 筛选变化回第 1 页再拉取；单纯翻页/换每页条数保持当前筛选。
  const reloadBudgetAlerts = async () => {
    alertPage.value = 1
    await loadBudgetAlerts()
  }

  const loadEmergencySwitch = async () => {
    try {
      const response = await fetchAdminAIEmergencySwitch()
      emergencySwitch.value = response?.data || null
    } catch (err) {
      ElMessage.error(err?.response?.data?.message || '加载紧急关闭开关失败')
    }
  }

  const openEmergencyDialog = () => {
    emergencyTargetDisabled.value = !emergencySwitch.value?.disabled
    emergencyReason.value = ''
    emergencyDialogVisible.value = true
  }

  const submitEmergencySwitch = async () => {
    if (emergencyTargetDisabled.value && !emergencyReason.value.trim()) {
      ElMessage.warning('关闭 AI 请填写原因，原因会随审计记录')
      return
    }
    try {
      await ElMessageBox.confirm(
        emergencyTargetDisabled.value
          ? '关闭后，所有商家 AI 调用会直接拒绝，确认紧急关闭吗？'
          : '确认恢复所有商家 AI 调用吗？恢复后仍受商家预算与超额策略约束。',
        emergencyTargetDisabled.value ? '确认紧急关闭 AI' : '确认恢复 AI 调用',
        { confirmButtonText: '确认', cancelButtonText: '取消', type: 'warning' },
      )
    } catch {
      return
    }
    submittingEmergency.value = true
    try {
      await saveAdminAIEmergencySwitch({ disabled: emergencyTargetDisabled.value, reason: emergencyReason.value.trim() })
      ElMessage.success(emergencyTargetDisabled.value ? 'AI 已紧急关闭' : 'AI 调用已恢复')
      emergencyDialogVisible.value = false
      await loadEmergencySwitch()
    } catch (err) {
      ElMessage.error(err?.response?.data?.message || '紧急开关未更新')
    } finally {
      submittingEmergency.value = false
    }
  }

  return {
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
  }
}
