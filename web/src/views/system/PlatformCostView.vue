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
          <div class="card-label">AI 模型实际成本</div>
          <strong>暂不可核算</strong>
          <p>供应商尚未接入，没有可核验的真实用量或账单数据。</p>
          <small>不把预估值或默认 0 值当作实际成本</small>
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
            <strong>当前无可核验数据</strong>
            <p>{{ overview.ai_usage?.note || '供应商尚未接入，暂不计算 AI 用量与成本。' }}</p>
          </article>
          <article class="basis-item result">
            <span class="basis-index">03 · 利润结论</span>
            <strong>保持未完成状态</strong>
            <p>{{ overview.profit_status_note }}</p>
          </article>
        </div>
      </DataPanel>

      <DataPanel
        title="商家 AI 用量排行"
        description="后续可按商家统计真实请求量与 Token 用量，用于比较使用频率；当前不展示未经核验的数据。"
        eyebrow="MERCHANT AI USAGE"
      >
        <el-alert
          class="usage-alert"
          type="warning"
          show-icon
          :closable="false"
          title="AI 服务尚未接入，商家用量和排名暂不可用"
          description="当前不会调用任何模型服务，也不会从日志中的估算值或默认零值推算 Token 消耗。轻量/精细模型分配留待后续阶段。"
        />
        <el-table
          v-if="overview.ai_usage?.data_available && overview.ai_usage?.merchant_ranking?.length"
          :data="overview.ai_usage.merchant_ranking"
          class="usage-table"
          stripe
        >
          <el-table-column prop="merchant_name" label="商家" min-width="180" />
          <el-table-column prop="request_count" label="请求次数" min-width="120" />
          <el-table-column prop="total_tokens" label="Token 总量" min-width="140" />
          <el-table-column label="实际成本" min-width="140">
            <template #default="{ row }">{{ money(row.cost_cents) }}</template>
          </el-table-column>
        </el-table>
        <el-empty v-else description="暂无可核验的商家 AI 用量数据" :image-size="72" />
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
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { getPlatformCostPeriodValidationError } from '../../utils/platformCostPeriod.js'
import { calculatePlatformCostScenario } from '../../utils/platformCostScenario.js'
import DataPanel from '../../components/design/DataPanel.vue'
import PageHero from '../../components/design/PageHero.vue'
import { createAdminPlatformExpense, createAdminPlatformExpenseReversal, fetchAdminPlatformCostOverview } from '../../api/modules'

const loading = ref(false)
const submittingExpense = ref(false)
const submittingReversal = ref(false)
const reversalDialogVisible = ref(false)
const selectedExpense = ref(null)
const overview = ref(null)
const error = ref('')
const today = new Date()
const todayString = localDateString(today)
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
})
const reversalForm = ref({ reversalDate: todayString, amountYuan: null, reason: '', idempotencyKey: newIdempotencyKey() })
const selectedPeriod = computed(() => overview.value?.selected_period || {})
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
  if (!expenseForm.value.category || !expenseForm.value.expenseDate || !expenseForm.value.memo.trim()) {
    ElMessage.warning('请补全费用类别、发生日期和简短备注')
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
    })
    ElMessage.success('实际费用已登记')
    expenseForm.value = { ...expenseForm.value, category: '', amountYuan: null, memo: '', idempotencyKey: newIdempotencyKey() }
    await loadOverview()
  } catch (err) {
    ElMessage.error(err?.response?.data?.message || '费用未保存，请检查数据后重试')
  } finally {
    submittingExpense.value = false
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

onMounted(loadOverview)
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
