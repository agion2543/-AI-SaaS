<template>
  <div class="platform-cost-page">
    <PageHero
      eyebrow="PLATFORM COST ACCOUNTING"
      title="平台成本核算"
      description="系统确认实收与管理员人工核验申报金额分开展示。商家顾客订单款、计划成本和估算金额不计入平台成本差额。"
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
      <section class="summary-grid">
        <article class="summary-card primary">
          <div class="card-label">本月系统确认订阅净实收</div>
          <strong>{{ money(overview.current_month?.net_received_cents) }}</strong>
          <p>已到账 {{ money(overview.current_month?.gross_received_cents) }}，已确认退款 {{ money(overview.current_month?.refunded_cents) }}</p>
          <small>{{ monthLabel }} · {{ overview.current_month?.paid_order_count || 0 }} 笔已确认订单</small>
        </article>
        <article class="summary-card manual">
          <div class="card-label">本月人工核验申报金额</div>
          <strong>{{ manualSummaryAmount(overview.current_month_manual_verified_subscription) }}</strong>
          <p>管理员根据外部凭证核验并声明付款发生；系统未独立确认银行到账。</p>
          <small>按核验时间 · {{ manualSummaryCount(overview.current_month_manual_verified_subscription) }} 笔；不并入系统实收</small>
        </article>
        <article class="summary-card">
          <div class="card-label">累计系统确认订阅净实收</div>
          <strong>{{ money(overview.lifetime?.net_received_cents) }}</strong>
          <p>已到账 {{ money(overview.lifetime?.gross_received_cents) }}，已确认退款 {{ money(overview.lifetime?.refunded_cents) }}</p>
          <small>涉及 {{ overview.lifetime?.merchant_count || 0 }} 家商家</small>
        </article>
        <article class="summary-card manual">
          <div class="card-label">累计人工核验申报金额</div>
          <strong>{{ manualSummaryAmount(overview.lifetime_manual_verified_subscription) }}</strong>
          <p>单独记录管理员对历史外部凭证的核验结果，不代表系统独立核实银行结算。</p>
          <small>累计 {{ manualSummaryCount(overview.lifetime_manual_verified_subscription) }} 笔；不并入实收或成本差额</small>
        </article>
        <article class="summary-card warning">
          <div class="card-label">AI 估算成本（估算）</div>
          <strong>{{ aiStats ? `${money(aiStats.total_estimated_cost_cents)}（估算）` : '暂不可核算' }}</strong>
          <p>近 30 天调用记录 × 模型价格表重算的估算成本；不是供应商实际账单。</p>
          <small>实际账单需超管手工录入后在下方核对</small>
        </article>
        <article class="summary-card muted">
          <div class="card-label">平台经营利润</div>
          <strong>暂不可完整计算</strong>
          <p>仅能查看订阅净实收减已登记实际费用的阶段性差额。</p>
          <small>该差额不等于利润</small>
        </article>
      </section>

      <DataPanel
        title="期间核算"
        description="选择最多 366 天；费用按费用发生日归期，冲正按冲正发生日抵减，不回溯原费用期间。"
        eyebrow="PERIOD SUMMARY"
      >
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

        <div class="period-grid">
          <article class="period-stat income">
            <span>已确认订阅到账</span>
            <strong>{{ money(selectedPeriod.subscription_gross_received_cents) }}</strong>
          </article>
          <article class="period-stat refund">
            <span>已确认退款</span>
            <strong>{{ money(selectedPeriod.subscription_confirmed_refunds_cents) }}</strong>
          </article>
          <article class="period-stat net">
            <span>订阅净实收</span>
            <strong>{{ money(selectedPeriod.subscription_net_received_cents) }}</strong>
          </article>
          <article class="period-stat manual">
            <span>人工核验申报金额（{{ manualSummaryCount(selectedPeriod.manual_verified_subscription) }} 笔）</span>
            <strong>{{ manualSummaryAmount(selectedPeriod.manual_verified_subscription) }}</strong>
          </article>
          <article class="period-stat cost">
            <span>已登记费用净额（{{ selectedPeriod.actual_expense_count || 0 }} 笔费用 · {{ selectedPeriod.expense_reversal_count || 0 }} 笔冲正）</span>
            <strong>{{ selectedPeriod.actual_expense_total_cents == null ? '暂无已录入成本' : money(selectedPeriod.actual_expense_total_cents) }}</strong>
          </article>
          <article class="period-stat balance">
            <span>扣已登记成本后的差额</span>
            <strong>{{ selectedPeriod.net_after_recorded_expenses_cents == null ? '利润暂不可算' : money(selectedPeriod.net_after_recorded_expenses_cents) }}</strong>
          </article>
        </div>

        <div class="manual-attestation-note">
          <strong>人工核验口径单独列示</strong>
          <span>按管理员审核时间归期；金额来自管理员对外部凭证和付款发生的核验声明，不是系统独立确认的银行到账，也不并入订阅净实收或扣已登记成本后的差额。</span>
        </div>

        <el-alert
          class="period-alert"
          :type="selectedPeriod.actual_expense_count || selectedPeriod.expense_reversal_count ? 'info' : 'warning'"
          show-icon
          :closable="false"
          :title="selectedPeriod.profit_status_note || '暂无已录入成本，利润暂不可算。'"
        />
        <div class="plan-note">
          <strong>计划/估算成本单独处理</strong>
          <span>{{ selectedPeriod.plans_note || '计划/估算费用不纳入实际成本或本期差额。' }}</span>
        </div>
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
      </DataPanel>

      <div class="work-grid">
        <DataPanel
          title="登记已发生的平台费用"
          description="只录已经发生、可核验的支出；金额以分保存，固定币种为 CNY。"
          eyebrow="ACTUAL EXPENSE ENTRY"
        >
          <el-form class="expense-form" label-position="top" @submit.prevent="submitExpense">
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
          <el-empty v-else description="本期暂无已录入成本；这不代表实际成本为零" :image-size="72" />
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
        <el-empty v-else description="本期暂无冲正记录" :image-size="72" />
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
        <el-empty v-else description="所选期间暂无已登记费用" :image-size="72" />
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
            <p>估算成本来自调用记录 × 模型价格表重算（按调用日期取当日最晚生效价）；实际账单为超管手工录入的供应商真实金额。两者口径不同，差额仅供核对参考，不代表欠款或退款；门店维度无归属的调用记为“未指定门店”。</p>
          </article>
          <article class="basis-item result">
            <span class="basis-index">03 · 利润结论</span>
            <strong>保持未完成状态</strong>
            <p>{{ overview.profit_status_note }}</p>
          </article>
        </div>
      </DataPanel>

      <DataPanel
        title="AI 成本控制"
        description="用量统计、模型价格、供应商账单、商家预算、预算预警与紧急关闭。估算成本来自调用记录 × 模型价格表（模型估算），与供应商实际账单严格分开；估算不是账单。"
        eyebrow="AI COST CONTROL"
      >
        <el-alert
          v-if="emergencySwitch?.disabled"
          class="ai-emergency-banner"
          type="error"
          show-icon
          :closable="false"
          title="AI 已由超管紧急关闭：所有商家 AI 调用会直接拒绝"
          :description="emergencySwitch?.switch?.reason ? `关闭原因：${emergencySwitch.switch.reason}` : '恢复后立即恢复调用。'"
        />
        <el-tabs v-model="aiTab" @tab-change="onAiTabChange">
          <el-tab-pane label="用量统计" name="usage">
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
                <span>{{ aiStats.estimated_cost_source || '调用记录 × 模型价格表重算。' }}</span>
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
              <div class="table-footnote">门店维度无归属的调用记为“未指定门店”，不虚构归属。{{ aiStats.period_start }} 至 {{ aiStats.period_end }}。</div>
            </template>
          </el-tab-pane>

          <el-tab-pane label="模型价格表" name="prices">
            <div class="ai-form-note">
              <strong>调价请新增一行</strong>
              <span>同一模型 + 供应商 + 生效时间重复提交会原地更新单价；历史调用的估算成本仍按调用时间的生效价重算，不改写历史行。</span>
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
          </el-tab-pane>

          <el-tab-pane label="供应商账单" name="bills">
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
                <span>{{ reconciliation.note || '估算成本来自调用记录 × 价格表（模型估算），实际账单为超管手工录入的供应商真实金额；两者口径不同，差额仅供核对参考，不代表欠款或退款。' }}</span>
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
          </el-tab-pane>

          <el-tab-pane label="商家预算" name="budgets">
            <div class="ai-form-note">
              <strong>0 表示不限额</strong>
              <span>每日 / 月度预算任一为 0 即该周期不限额。无单独预算的商家适用全局默认行；全局默认也未设置时不限额。超额策略由后端在 AI 调用入口强制执行。</span>
            </div>
            <el-form class="ai-inline-form" label-position="top" @submit.prevent="submitBudget">
              <div class="form-row ai-budget-form-row">
                <el-form-item label="商家" required>
                  <el-select v-model="budgetForm.merchantId" class="full-width" placeholder="选择商家或全局默认" filterable>
                    <el-option :value="0" label="全局默认（无单独预算的商家）" />
                    <el-option v-for="item in merchantsForBudget" :key="item.id" :label="item.name || `商家 #${item.id}`" :value="item.id" />
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
                <template #default="{ row }">{{ budgetMerchantLabel(row.merchant_id, merchantNameOf(row.merchant_id)) }}</template>
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
            <el-empty v-else description="暂无商家预算；未设置预算的商家不限额" :image-size="72" />
            <div v-if="budgetStatus" class="ai-budget-status">
              <div class="ai-budget-status-head">
                <strong>{{ budgetMerchantLabel(budgetStatus.status.merchant_id, merchantNameOf(budgetStatus.status.merchant_id)) }} · 当前预算状态</strong>
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
              <small v-if="budgetStatus.status.has_budget" class="ai-budget-status-note">超额策略：{{ overLimitActionLabel(budgetStatus.status.effective_action) }}。已用金额为调用记录上的估算成本累计（预算执行口径），与统计页按价格表重算的口径来源不同。</small>
            </div>
          </el-tab-pane>

          <el-tab-pane label="预算预警" name="alerts">
            <div class="period-toolbar">
              <el-select v-model="alertFilter.merchantId" placeholder="全部商家" clearable class="ai-filter-select" @change="loadBudgetAlerts">
                <el-option v-for="item in merchantsForBudget" :key="item.id" :label="item.name || `商家 #${item.id}`" :value="item.id" />
              </el-select>
              <el-select v-model="alertFilter.threshold" placeholder="全部阈值" clearable class="ai-filter-select" @change="loadBudgetAlerts">
                <el-option v-for="item in budgetAlertThresholds" :key="item" :label="`${item}%`" :value="item" />
              </el-select>
              <el-select v-model="alertFilter.periodType" placeholder="全部周期" clearable class="ai-filter-select" @change="loadBudgetAlerts">
                <el-option value="daily" label="每日" />
                <el-option value="monthly" label="每月" />
              </el-select>
              <el-button type="primary" plain :loading="alertsLoading" @click="loadBudgetAlerts">刷新预警</el-button>
              <span class="currency-hint">预警在用量首次达到阈值时产生一条，不重复产生</span>
            </div>
            <el-table v-if="budgetAlerts.length" :data="budgetAlerts" row-key="id" stripe class="expense-table">
              <el-table-column label="商家" min-width="180">
                <template #default="{ row }">{{ merchantDisplayName(row.merchant_id, merchantNameOf(row.merchant_id)) }}</template>
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
            <el-empty v-else description="暂无预算预警记录" :image-size="72" />
          </el-tab-pane>

          <el-tab-pane label="紧急关闭" name="emergency">
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
          </el-tab-pane>
        </el-tabs>
      </DataPanel>

      <DataPanel
        title="时间成本与收入台账"
        description="只记你自己的时间与平台真实收入；测算参数一律标假设，不写入真实记录。全部数据仅超管可见。"
        eyebrow="TIME COST AND INCOME LEDGER"
      >
        <el-tabs v-model="financeTab" @tab-change="onFinanceTabChange">
          <el-tab-pane label="时间成本" name="time-logs">
            <el-alert
              class="ai-form-note"
              type="info"
              show-icon
              :closable="false"
              title="时间成本只计经济成本，不进现金支出"
              description="金额由服务端按（小时数 × 参考小时价值）算出；参考小时价值由你自填，系统不推断真实工资。记录只追加不改不删。"
            />
            <el-form class="expense-form" label-position="top" @submit.prevent="submitTimeLog">
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
            <el-empty v-else description="暂无时间成本记录" :image-size="72" />
          </el-tab-pane>

          <el-tab-pane label="收入台账" name="income">
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
          </el-tab-pane>

          <el-tab-pane label="MRR 与保本" name="mrr">
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
            <el-divider />
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
          </el-tab-pane>
        </el-tabs>
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
      </el-dialog>

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
      </el-dialog>

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
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getPlatformCostPeriodValidationError } from '../../utils/platformCostPeriod.js'
import { calculatePlatformCostScenario } from '../../utils/platformCostScenario.js'
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
} from '../../utils/aiCostControl.js'
import DataPanel from '../../components/design/DataPanel.vue'
import PageHero from '../../components/design/PageHero.vue'
import {
  createAdminAIVendorBill,
  createAdminPlatformExpense,
  createAdminPlatformExpenseReversal,
  createAdminPlatformIncomeRecord,
  createAdminPlatformTimeLog,
  fetchAdminAIBillReconciliation,
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
} from '../../api/modules'
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
} from '../../utils/platformFinance.js'

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
const aiTab = ref('usage')
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
const merchantsForBudget = ref([])
const merchantsLoaded = ref(false)
const submittingBudget = ref(false)
const budgetForm = ref({ merchantId: 0, dailyBudgetYuan: 0, monthlyBudgetYuan: 0, overLimitAction: 'stop' })
const budgetStatus = ref(null)
const budgetAlerts = ref([])
const alertsLoading = ref(false)
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
    if (dimension === 'store') return { ...row, rowKey: `store-${row.store_id || 0}`, displayName: storeDisplayName(row.store_id) }
    if (dimension === 'model') return { ...row, rowKey: `model-${row.model || index}`, displayName: row.model || '未注明' }
    if (dimension === 'scenario') return { ...row, rowKey: `scenario-${row.scenario || index}`, displayName: scenarioLabel(row.scenario) }
    return { ...row, rowKey: `merchant-${row.merchant_id}`, displayName: merchantDisplayName(row.merchant_id, row.merchant_name) }
  })
})
const merchantNameOf = (merchantId) => merchantsForBudget.value.find((item) => item.id === merchantId)?.name || ''
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

const financeTab = ref('time-logs')
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
    params.avg_revenue_per_merchant_cents = Math.round(Number(breakevenForm.value.revenueYuan) * 100)
  }
  if (breakevenForm.value.variableYuan != null && breakevenForm.value.variableYuan !== '') {
    params.avg_variable_cost_per_merchant_cents = Math.round(Number(breakevenForm.value.variableYuan) * 100)
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

const onFinanceTabChange = (tab) => {
  if (tab === 'mrr' && !mrr.value) loadMRR()
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

const ensureMerchantsLoaded = async () => {
  if (merchantsLoaded.value) return
  try {
    const response = await fetchMerchants({ page: 1, page_size: 500 })
    merchantsForBudget.value = response?.data?.list || []
    merchantsLoaded.value = true
  } catch (err) {
    ElMessage.error(err?.response?.data?.message || '加载商家列表失败')
  }
}

const loadBudgets = async () => {
  await ensureMerchantsLoaded()
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
  await ensureMerchantsLoaded()
  alertsLoading.value = true
  try {
    const params = { ...aiPeriodParams() }
    if (alertFilter.value.merchantId !== '' && alertFilter.value.merchantId !== null) params.merchant_id = alertFilter.value.merchantId
    if (alertFilter.value.threshold !== '' && alertFilter.value.threshold !== null) params.threshold = alertFilter.value.threshold
    if (alertFilter.value.periodType) params.period_type = alertFilter.value.periodType
    const response = await fetchAdminAIBudgetAlerts(params)
    budgetAlerts.value = response?.data || []
  } catch (err) {
    ElMessage.error(err?.response?.data?.message || '加载预算预警失败')
  } finally {
    alertsLoading.value = false
  }
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

const onAiTabChange = (tab) => {
  if (tab === 'usage') loadAiUsageStats()
  else if (tab === 'prices') loadModelPrices()
  else if (tab === 'bills') loadVendorBills()
  else if (tab === 'budgets') loadBudgets()
  else if (tab === 'alerts') loadBudgetAlerts()
  else if (tab === 'emergency') loadEmergencySwitch()
}

onMounted(() => {
  loadOverview()
  loadAiUsageStats()
  loadEmergencySwitch()
  loadExpenseStats()
  loadTimeLogs()
  loadIncomeRecords()
})
</script>

<style scoped>
.platform-cost-page{display:grid;gap:18px}.page-alert{margin:0}.summary-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:14px}.summary-card{display:grid;align-content:start;gap:9px;min-height:174px;padding:20px;border:1px solid #dce8f5;border-radius:22px;background:rgba(255,255,255,.96);box-shadow:0 16px 38px rgba(15,39,71,.07)}.summary-card.primary{color:#fff;border-color:#2563eb;background:linear-gradient(135deg,#0f2d55 0%,#2563eb 65%,#06b6d4 100%)}.summary-card.warning{background:linear-gradient(145deg,#fff,#fff8eb);border-color:#f3d7a7}.summary-card.muted{background:linear-gradient(145deg,#fff,#f3f6fa)}.card-label{font-size:13px;font-weight:800;color:#64748b}.summary-card.primary .card-label,.summary-card.primary small,.summary-card.primary p{color:rgba(255,255,255,.78)}.summary-card strong{font-size:clamp(22px,2vw,30px);line-height:1.15;letter-spacing:-.04em;color:#0f172a}.summary-card.primary strong{color:#fff}.summary-card p,.summary-card small{margin:0;color:#64748b;font-size:12px;line-height:1.55}.period-toolbar{display:flex;align-items:center;gap:12px;flex-wrap:wrap;margin-bottom:16px}.period-toolbar :deep(.el-date-editor){max-width:360px}.currency-hint{font-size:12px;color:#64748b}.period-grid{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:12px}.period-stat{display:grid;gap:8px;padding:15px;border:1px solid #e2eaf3;border-radius:16px;background:#fbfdff;min-height:96px}.period-stat span{color:#64748b;font-size:12px;line-height:1.4}.period-stat strong{font-size:20px;line-height:1.2;color:#0f172a;overflow-wrap:anywhere}.period-stat.income{border-color:#b9e6d3;background:#f2fbf7}.period-stat.refund{border-color:#f6d2d2;background:#fff7f7}.period-stat.cost{border-color:#f1d6ad;background:#fffaf1}.period-stat.balance{border-color:#ccd8f7;background:#f7f9ff}.period-alert{margin-top:14px}.plan-note{display:flex;gap:10px;align-items:flex-start;margin-top:12px;padding:12px 14px;border-radius:12px;background:#f7f8fa;color:#64748b;font-size:12px;line-height:1.55}.plan-note strong{flex:none;color:#334155}.work-grid{display:grid;grid-template-columns:minmax(340px,.9fr) minmax(0,1.1fr);gap:18px}.expense-form{display:grid;gap:2px}.form-row{display:grid;gap:14px}.form-row.two-col{grid-template-columns:repeat(2,minmax(0,1fr))}.full-width{width:100%}.immutable-note{margin:2px 0 14px}.form-actions{display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap}.audit-note{max-width:380px;color:#64748b;font-size:12px;line-height:1.5}.expense-table{width:100%}.table-footnote{margin-top:10px;color:#64748b;font-size:12px}.basis-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:14px}.basis-item{display:grid;align-content:start;gap:9px;padding:16px;border:1px solid #e1eaf4;border-radius:16px;background:#fbfdff}.basis-item strong{font-size:16px;color:#0f172a}.basis-item p{margin:0;font-size:13px;line-height:1.65;color:#64748b;overflow-wrap:anywhere}.basis-index{font-size:11px;font-weight:900;letter-spacing:.12em;color:#2563eb;text-transform:uppercase}.basis-item.cost .basis-index{color:#b45309}.basis-item.result .basis-index{color:#64748b}.usage-alert{margin-bottom:14px}.usage-table{width:100%}.snapshot-footnote{padding:0 4px;color:#64748b;font-size:12px;line-height:1.6}
@media(max-width:1200px){.summary-grid{grid-template-columns:repeat(2,minmax(0,1fr))}.period-grid{grid-template-columns:repeat(3,minmax(0,1fr))}.work-grid{grid-template-columns:1fr}.basis-grid{grid-template-columns:1fr}}
@media(max-width:640px){.summary-grid,.period-grid{grid-template-columns:1fr}.summary-card{min-height:150px}.form-row.two-col{grid-template-columns:1fr}.form-actions{align-items:stretch;flex-direction:column}.form-actions :deep(.el-button){width:100%}.plan-note{flex-direction:column}}
</style>

<style scoped>
.scenario-notice{display:flex;align-items:center;gap:10px;margin-bottom:14px;padding:12px 14px;border:1px solid #f2d49f;border-radius:12px;background:#fff9ed;color:#7c5b23;font-size:12px;line-height:1.55}.scenario-layout{display:grid;grid-template-columns:minmax(300px,.82fr) minmax(0,1.18fr);gap:16px}.scenario-input-panel,.scenario-result-panel{min-width:0;padding:16px;border:1px solid #e2eaf3;border-radius:16px;background:#fbfdff}.scenario-fields{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px}.scenario-field{display:grid;align-content:start;gap:7px;min-width:0}.scenario-field label{color:#334155;font-size:12px;font-weight:800;line-height:1.45}.scenario-field small{color:#64748b;font-size:11px;line-height:1.45}.scenario-input-actions{display:flex;align-items:center;justify-content:space-between;gap:10px;margin-top:14px}.scenario-input-actions span{color:#64748b;font-size:11px;line-height:1.45}.scenario-result-heading{display:flex;justify-content:space-between;align-items:center;gap:10px;margin-bottom:12px}.scenario-result-heading strong{font-size:15px;color:#0f172a}.scenario-result-heading span{color:#64748b;font-size:11px}.scenario-empty{display:grid;place-items:center;min-height:170px;padding:20px;border:1px dashed #cbd8e7;border-radius:12px;color:#64748b;text-align:center;font-size:13px;line-height:1.6}.scenario-output-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:9px}.scenario-output{display:grid;align-content:start;gap:7px;min-width:0;min-height:86px;padding:12px;border:1px solid #e2eaf3;border-radius:12px;background:#fff}.scenario-output span{color:#64748b;font-size:11px;line-height:1.4}.scenario-output strong{color:#0f172a;font-size:17px;line-height:1.25;overflow-wrap:anywhere}.scenario-output.positive{border-color:#b9e6d3;background:#f2fbf7}.scenario-output.negative{border-color:#f6d2d2;background:#fff7f7}.scenario-output.result strong{font-size:19px}.scenario-output.break-even strong{font-size:14px}.scenario-break-even-alert{margin-top:10px}.scenario-break-even-note{margin-top:10px;color:#64748b;font-size:11px;line-height:1.5}.scenario-formulas{display:grid;gap:5px;margin-top:12px;padding:11px 12px;border-radius:10px;background:#f1f6fc;color:#52647b;font-size:11px;line-height:1.5}.scenario-formulas strong{color:#334155}.scenario-formulas span{overflow-wrap:anywhere}
@media(max-width:1200px){.scenario-layout{grid-template-columns:1fr}.scenario-output-grid{grid-template-columns:repeat(3,minmax(0,1fr))}}
@media(max-width:640px){.scenario-fields,.scenario-output-grid{grid-template-columns:1fr}.scenario-notice,.scenario-input-actions{align-items:flex-start;flex-direction:column}.scenario-output{min-height:74px}}
</style>

<style scoped>
.summary-grid{grid-template-columns:repeat(3,minmax(0,1fr))}
.period-grid{grid-template-columns:repeat(6,minmax(0,1fr))}
.summary-card.manual{background:linear-gradient(145deg,#fff,#f3f0ff);border-color:#d8c9ff}
.period-stat.manual{border-color:#d8c9ff;background:#faf8ff}
.manual-attestation-note{display:grid;gap:4px;margin-top:14px;padding:12px 14px;border-radius:12px;background:#f7f3ff;color:#5b4b83;font-size:12px;line-height:1.55}
.manual-attestation-note strong{color:#49347a}
.reversal-period-note{margin-top:10px;color:#64748b;font-size:12px;line-height:1.55}
.reversal-source-card{display:grid;gap:6px;margin-bottom:14px;padding:14px;border:1px solid #f1d6ad;border-radius:12px;background:#fffaf1;color:#475569;font-size:13px;line-height:1.5}
.reversal-source-card strong{color:#0f172a}
.reversal-dialog-note{margin-bottom:14px}
.reversal-balance{display:grid;justify-items:center;gap:4px;line-height:1.45;font-size:12px;color:#64748b}
.reversal-balance strong{color:#92400e;font-size:12px}
.input-hint{margin-top:5px;color:#64748b;font-size:12px;line-height:1.5}
@media(max-width:1200px){.summary-grid{grid-template-columns:repeat(2,minmax(0,1fr))}.period-grid{grid-template-columns:repeat(3,minmax(0,1fr))}}
@media(max-width:640px){.summary-grid,.period-grid{grid-template-columns:1fr}}
</style>

<style scoped>
.ai-emergency-banner{margin-bottom:14px}
.ai-stats-grid{grid-template-columns:repeat(6,minmax(0,1fr))}
.ai-recon-grid{grid-template-columns:repeat(3,minmax(0,1fr))}
.ai-dimension-toolbar{display:flex;align-items:center;gap:12px;flex-wrap:wrap;margin:16px 0 12px}
.ai-form-note{display:grid;gap:4px;margin-bottom:14px;padding:12px 14px;border-radius:12px;background:#f1f6fc;color:#52647b;font-size:12px;line-height:1.55}
.ai-form-note strong{color:#334155}
.ai-inline-form{display:grid;gap:2px;margin-top:4px}
.ai-price-form-row{grid-template-columns:repeat(3,minmax(0,1fr))}
.ai-price-form-row + .ai-price-form-row{grid-template-columns:repeat(4,minmax(0,1fr))}
.ai-bill-form-row{grid-template-columns:minmax(0,1fr) minmax(0,1.4fr) minmax(0,1fr)}
.ai-budget-form-row{grid-template-columns:repeat(4,minmax(0,1fr))}
.ai-table-gap{margin-top:16px}
.ai-filter-select{width:180px}
.ai-budget-status{display:grid;gap:14px;margin-top:16px;padding:16px;border:1px solid #ccd8f7;border-radius:16px;background:#f7f9ff}
.ai-budget-status-head{display:flex;align-items:center;gap:10px;flex-wrap:wrap}
.ai-budget-status-head strong{color:#0f172a;font-size:15px}
.ai-budget-status-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px}
.ai-budget-status-grid article{display:grid;gap:8px;padding:14px;border:1px solid #e2eaf3;border-radius:12px;background:#fff}
.ai-budget-status-grid article span{color:#64748b;font-size:12px}
.ai-budget-status-grid article strong{color:#0f172a;font-size:16px}
.ai-budget-status-grid article small{color:#64748b;font-size:12px}
.ai-budget-status-note{color:#64748b;font-size:12px;line-height:1.55}
.ai-emergency-panel{display:grid;gap:16px}
.ai-emergency-status{display:grid;gap:8px;padding:18px;border:1px solid #e2eaf3;border-radius:16px;background:#fbfdff}
.ai-emergency-status span{color:#64748b;font-size:12px}
.ai-emergency-status strong{font-size:20px}
.ai-emergency-status small{color:#64748b;font-size:12px;line-height:1.5}
.ai-emergency-off{color:#dc2626}
.ai-emergency-on{color:#15803d}
.ai-emergency-note{margin:0}
.ai-dialog-form{margin-top:14px}
.cost-flag-row{display:flex;align-items:center;gap:16px;flex-wrap:wrap;min-height:32px}
.expense-flags{display:flex;gap:6px;flex-wrap:wrap;align-items:center}
@media(max-width:1200px){.ai-stats-grid{grid-template-columns:repeat(3,minmax(0,1fr))}.ai-recon-grid{grid-template-columns:1fr}.ai-price-form-row,.ai-price-form-row + .ai-price-form-row,.ai-bill-form-row,.ai-budget-form-row{grid-template-columns:repeat(2,minmax(0,1fr))}.ai-budget-status-grid{grid-template-columns:1fr}}
@media(max-width:640px){.ai-stats-grid{grid-template-columns:1fr}.ai-price-form-row,.ai-price-form-row + .ai-price-form-row,.ai-bill-form-row,.ai-budget-form-row{grid-template-columns:1fr}.ai-filter-select{width:100%}}
</style>
