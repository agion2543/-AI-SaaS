<template>
  <div class="subscription-revenue-page">
    <PageHero
      eyebrow="SUBSCRIPTION REVENUE"
      title="订阅营收管理"
      description="专门管理平台向商家收取的 SaaS 订阅费：待确认收款、已确认收入、续费记录和即将到期商家。顾客扫码交易流水不在这里处理。"
      compact
    >
      <template #actions>
        <el-button v-if="auth.hasPermission('customer_order_export')" plain :loading="exporting" @click="exportRevenue">导出订阅账单</el-button>
        <el-button type="primary" :loading="loading" @click="load">刷新</el-button>
      </template>
    </PageHero>

    <section class="metric-grid">
      <MetricCard label="已确认订阅收入" :value="formatMoney(confirmedIncome)" hint="按既有 paid 订单口径；人工核验另列、不计入此数。" tone="primary" />
      <MetricCard label="待确认收款" :value="formatMoney(pendingAmount)" :hint="`${pendingOrders.length} 笔待平台核对`" tone="warning" />
      <MetricCard label="订阅中商家" :value="activeMerchantCount" hint="当前仍在有效期内的商家。" tone="success" />
      <MetricCard label="7 天内到期" :value="expiringMerchants.length" hint="建议提前跟进续费。" tone="orange" />
    </section>

    <section class="scope-alert">
      <strong>本页只处理平台 SaaS 订阅费</strong>
      <span>确认到账后会把订阅订单改为已确认收入，并同步开通或续期商家工作台。顾客扫码点单流水只在订单财务和商家财务中审查，不在这里确认到账。</span>
    </section>

    <section v-if="lastConfirmed" class="confirm-result">
      <div>
        <span>刚刚已开通</span>
        <strong>{{ lastConfirmed.merchant_name }}</strong>
        <small>{{ lastConfirmed.order_no }} / {{ lastConfirmed.plan_name }}</small>
      </div>
      <div>
        <span>到期时间</span>
        <strong>{{ formatTime(lastConfirmed.expire_at) }}</strong>
        <small>已写入商家服务状态和订阅订单记录</small>
      </div>
      <el-button plain @click="lastConfirmed = null">收起</el-button>
    </section>

    <section class="summary-strip">
      <button
        v-for="item in quickFilters"
        :key="item.value"
        type="button"
        :class="{ active: statusFilter === item.value }"
        @click="statusFilter = item.value"
      >
        <span>{{ item.label }}</span>
        <strong>{{ item.count }}</strong>
        <small>{{ item.hint }}</small>
      </button>
    </section>

    <section class="workflow-panel">
      <article v-for="item in workflowCards" :key="item.title">
        <span>{{ item.label }}</span>
        <strong>{{ item.title }}</strong>
        <p>{{ item.text }}</p>
      </article>
    </section>

    <DataPanel title="筛选条件" description="只筛选商家订阅订单，用于平台营收核对和续费跟进。">
      <div class="filters-row">
        <el-date-picker
          v-model="dateRange"
          type="daterange"
          unlink-panels
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          value-format="YYYY-MM-DD"
        />
        <el-input v-model="keyword" clearable placeholder="搜索订单号 / 商家 / 套餐" />
        <el-select v-model="statusFilter" clearable placeholder="订阅订单状态">
          <el-option label="全部" value="" />
          <el-option label="待确认收款" value="pending" />
          <el-option label="已过期未付款（平台静态码）" value="expired_unpaid" />
          <el-option label="已确认收入" value="paid" />
          <el-option label="人工核验（外部凭证）" value="manually_verified" />
          <el-option label="支付失败" value="failed" />
        </el-select>
        <el-button @click="resetFilters">重置</el-button>
      </div>
    </DataPanel>

    <DataPanel title="待确认订阅收款" description="这里是订阅到账确认的主操作页。订单财务只做总审查，顾客扫码订单不会在这里确认。">
      <el-alert
        v-if="pendingSnapshotReviewCount"
        class="snapshot-review-alert"
        type="warning"
        show-icon
        :closable="false"
        :title="`${pendingSnapshotReviewCount} 笔待确认订单缺少有效下单快照，不能按当前套餐推断；请人工核对原始付款记录。`"
        description="无快照订单的普通确认已禁用。独立核验流程要求管理员确认已核对原始证据、且历史外部付款确已发生，并按所填原始条款更新订阅；它不会调用支付渠道或伪造渠道支付记录。"
      />
      <el-table :data="pendingOrders" empty-text="暂无待确认订阅收款">
        <el-table-column prop="order_no" label="订阅订单号" min-width="190" />
        <el-table-column label="商家" min-width="160">
          <template #default="{ row }">{{ row.merchant?.name || row.merchant_id || '-' }}</template>
        </el-table-column>
        <el-table-column label="套餐" width="130">
          <template #default="{ row }">{{ planName(row) }}</template>
        </el-table-column>
        <el-table-column label="金额" width="120">
          <template #default="{ row }">
            <span v-if="hasValidSubscriptionSnapshot(row)">{{ formatMoney(orderAmount(row)) }}</span>
            <el-tag v-else type="warning">需人工复核</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="付款方式" width="130">
          <template #default="{ row }">{{ paymentChannelLabel(row.payment_channel) }}</template>
        </el-table-column>
        <el-table-column label="开通后到期" min-width="160">
          <template #default="{ row }">{{ expectedExpireText(row) }}</template>
        </el-table-column>
        <el-table-column label="处理状态" min-width="180">
          <template #default="{ row }">
            <el-tag :type="subscriptionPaymentMarked(row) ? 'primary' : 'warning'">
              {{ subscriptionPaymentMarked(row) ? '商家已标记付款' : '待核对平台账户到账' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" min-width="170">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="250" fixed="right">
          <template #default="{ row }">
            <el-button
              v-if="hasValidSubscriptionSnapshot(row)"
              size="small"
              type="primary"
              :loading="confirmingOrderId === row.id"
              @click="confirmSubscription(row)"
            >
              确认到账并开通
            </el-button>
            <div v-else class="manual-review-actions">
              <el-tag type="warning">需人工复核</el-tag>
              <el-button size="small" disabled>普通确认已禁用</el-button>
              <el-button
                v-if="canOpenManualReviewDraft(row)"
                size="small"
                type="warning"
                plain
                @click="openManualReviewDraft(row)"
              >
                人工复核
              </el-button>
              <el-button v-else size="small" disabled>快照异常，暂不可填草稿</el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>
    </DataPanel>

    <DataPanel title="已过期未付款订阅订单（平台静态码）" description="30 分钟内未确认付款的静态码订单已关闭；只有管理员核对平台收款账户确认实际到账后，才能在此人工对账并开通权益。">
      <el-alert
        type="warning"
        show-icon
        :closable="false"
        title="这些订单仍是未付款状态"
        description="平台静态二维码不会自动验证付款；页面回跳或商家自报付款均不会确认到账或开通订阅。请先核对平台收款账户的实际流水，再执行人工确认。"
      />
      <el-table :data="expiredUnpaidOrders" empty-text="暂无已过期未付款订阅订单">
        <el-table-column prop="order_no" label="订阅订单号" min-width="190" />
        <el-table-column label="商家" min-width="160">
          <template #default="{ row }">{{ row.merchant?.name || row.merchant_id || '-' }}</template>
        </el-table-column>
        <el-table-column label="套餐" width="130">
          <template #default="{ row }">{{ planName(row) }}</template>
        </el-table-column>
        <el-table-column label="金额" width="120">
          <template #default="{ row }">{{ hasValidSubscriptionSnapshot(row) ? formatMoney(orderAmount(row)) : '需核对原始条款' }}</template>
        </el-table-column>
        <el-table-column label="状态" width="140">
          <template #default><el-tag type="warning">已过期 · 未付款</el-tag></template>
        </el-table-column>
        <el-table-column label="付款方式" width="130">
          <template #default="{ row }">{{ paymentChannelLabel(row.payment_channel) }}</template>
        </el-table-column>
        <el-table-column label="创建时间" min-width="170">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <el-button
              v-if="canConfirmSubscription(row)"
              size="small"
              type="warning"
              :loading="confirmingOrderId === row.id"
              @click="confirmSubscription(row)"
            >
              核对到账并开通
            </el-button>
            <el-button
              v-else-if="canOpenManualReviewDraft(row)"
              size="small"
              type="danger"
              plain
              @click="openManualReviewDraft(row)"
            >
              核验原始凭证
            </el-button>
            <el-tag v-else-if="!hasValidSubscriptionSnapshot(row)" type="danger">快照缺失，普通确认禁用</el-tag>
            <el-tag v-else type="info">此付款渠道需专属人工核对</el-tag>
          </template>
        </el-table-column>
      </el-table>
    </DataPanel>

    <DataPanel title="人工核验（外部凭证）" description="本区独立展示管理员核验记录与权益变更审计；该状态不计入已确认支付收入。">
      <el-table :data="manualReviewedOrders" empty-text="暂无已保存的历史订阅人工复核">
        <el-table-column prop="order_no" label="订单号" min-width="175" />
        <el-table-column label="商家 / 原计划" min-width="185">
          <template #default="{ row }"><strong>{{ row.merchant?.name || row.merchant_id || '-' }}</strong><small>{{ row.manual_review.plan_name || '—' }} · {{ row.manual_review.duration_days || '—' }} 天</small></template>
        </el-table-column>
        <el-table-column label="原始金额" width="115" align="right">
          <template #default="{ row }">{{ formatMoney(row.manual_review.amount_cents) }}</template>
        </el-table-column>
        <el-table-column label="复核结果" min-width="160">
          <template #default="{ row }"><el-tag type="warning">人工核验（外部凭证）</el-tag><small>非渠道确认 · {{ row.status }}</small></template>
        </el-table-column>
        <el-table-column label="证据引用 / 操作者" min-width="210">
          <template #default="{ row }">{{ row.manual_review_audit.evidence_reference || row.manual_review.evidence_reference || '—' }}<small>{{ row.manual_review_audit.actor_role || row.manual_review.actor_role || '—' }} #{{ row.manual_review_audit.actor_id || row.manual_review.actor_id || '—' }}</small></template>
        </el-table-column>
        <el-table-column label="理由 / 前后状态" min-width="260">
          <template #default="{ row }">{{ row.manual_review_audit.reason || row.manual_review.reason || '—' }}<small>订单：{{ row.manual_review_audit.order_status_before || '—' }} → {{ row.manual_review_audit.order_status_after || row.status }}；订阅：{{ row.manual_review_audit.subscription_status_before || '—' }} → {{ row.manual_review_audit.subscription_status_after || '—' }}</small></template>
        </el-table-column>
        <el-table-column label="到期时间 / 复核时间" min-width="180">
          <template #default="{ row }">{{ formatTime(row.subscription_end_at) }}<small>复核：{{ formatTime(row.manual_review_audit.reviewed_at || row.manual_review.created_at) }}</small></template>
        </el-table-column>
      </el-table>
      <el-alert class="saved-review-note" type="info" :closable="false" show-icon>该标签记录的是管理员对外部凭证的核验声明；系统没有访问或验证外部凭证，也不代表支付渠道回调、自动收款或渠道确认。此区与已确认收入分开，金额不进入渠道确认收入合计。</el-alert>
    </DataPanel>

    <section class="split-grid">
      <DataPanel title="已确认订阅收入" description="用于核对平台 SaaS 订阅营收。">
        <el-table :data="paidOrders" empty-text="暂无已确认订阅收入">
          <el-table-column prop="order_no" label="订单号" min-width="180" />
          <el-table-column label="商家" min-width="150">
            <template #default="{ row }">{{ row.merchant?.name || '-' }}</template>
          </el-table-column>
          <el-table-column label="套餐" width="120">
            <template #default="{ row }">{{ planName(row) }}</template>
          </el-table-column>
          <el-table-column label="金额" width="110">
            <template #default="{ row }">{{ formatMoney(orderAmount(row)) }}</template>
          </el-table-column>
          <el-table-column label="确认时间" min-width="160">
            <template #default="{ row }">{{ formatTime(row.paid_at || row.updated_at) }}</template>
          </el-table-column>
          <el-table-column label="开通到期" min-width="160">
            <template #default="{ row }">{{ formatTime(row.subscription_end_at) }}</template>
          </el-table-column>
          <el-table-column label="服务状态" width="120">
            <template #default="{ row }">
              <el-tag :type="merchantSubscriptionType(row)">{{ merchantSubscriptionLabel(row) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="入账口径" min-width="150">
            <template #default>平台 SaaS 订阅收入</template>
          </el-table-column>
        </el-table>
      </DataPanel>

      <DataPanel title="即将到期商家" description="7 天内到期的订阅商家，适合提前提醒续费。">
        <el-table :data="expiringMerchants" empty-text="暂无即将到期商家">
          <el-table-column prop="name" label="商家" min-width="150" />
          <el-table-column label="套餐" width="120">
            <template #default="{ row }">{{ row.subscription_plan || row.merchant_plan?.name || '-' }}</template>
          </el-table-column>
          <el-table-column label="到期时间" min-width="160">
            <template #default="{ row }">{{ formatTime(row.subscription_expire_at || row.subscription_expired_at) }}</template>
          </el-table-column>
          <el-table-column label="建议" min-width="150">
            <template #default>提前联系续费，避免商家端功能中断。</template>
          </el-table-column>
        </el-table>
      </DataPanel>
    </section>

    <DataPanel title="订阅订单明细" description="当前筛选范围内的所有商家订阅订单。">
      <el-table :data="filteredSubscriptionOrders" empty-text="暂无订阅订单">
        <el-table-column prop="order_no" label="订单号" min-width="190" />
        <el-table-column label="商家" min-width="160">
          <template #default="{ row }">{{ row.merchant?.name || row.merchant_id || '-' }}</template>
        </el-table-column>
        <el-table-column label="套餐" width="140">
          <template #default="{ row }">{{ planName(row) }}</template>
        </el-table-column>
        <el-table-column label="金额" width="120">
          <template #default="{ row }">
            <span v-if="hasValidSubscriptionSnapshot(row) || row.manual_review">{{ formatMoney(orderAmount(row)) }}</span>
            <el-tag v-else type="warning">需人工复核</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="120">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)">{{ statusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="支付方式" width="130">
          <template #default="{ row }">{{ paymentChannelLabel(row.payment_channel) }}</template>
        </el-table-column>
        <el-table-column label="开通到期" min-width="160">
          <template #default="{ row }">{{ formatTime(row.subscription_end_at) }}</template>
        </el-table-column>
        <el-table-column label="商家服务" width="120">
          <template #default="{ row }">
            <el-tag :type="merchantSubscriptionType(row)">{{ merchantSubscriptionLabel(row) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" min-width="170">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
      </el-table>
    </DataPanel>

    <el-dialog
      v-model="manualReviewDraftVisible"
      title="历史订阅单人工复核"
      width="min(720px, 94vw)"
      destroy-on-close
      @closed="clearManualReviewDraft"
    >
      <el-alert
        class="manual-draft-warning"
        type="error"
        show-icon
        :closable="false"
        title="独立人工核验：需确认历史外部付款确已发生"
        description="提交后会保存管理员对原始证据和外部付款发生的明确断言、原始条款、来源、理由及前后状态审计，并按填写期限更新订阅。系统不会调用支付渠道，也不会伪造 provider PaymentRecord、PaidAt 或渠道交易号。不得依据当前可变套餐推断历史条款。"
      />
      <section v-if="manualReviewDraftOrder" class="manual-draft-order">
        <div><span>订单号</span><strong>{{ manualReviewDraftOrder.order_no }}</strong></div>
        <div><span>商家</span><strong>{{ manualReviewDraftOrder.merchant?.name || manualReviewDraftOrder.merchant_id || '-' }}</strong></div>
        <div><span>当前状态</span><el-tag type="warning">{{ statusLabel(manualReviewDraftOrder.status) }} · 需人工复核</el-tag></div>
        <div><span>订单金额（须精确匹配）</span><strong>{{ formatMoney(Number(manualReviewDraftOrder.total_amount ?? manualReviewDraftOrder.amount ?? 0)) }}</strong></div>
      </section>
      <el-form
        ref="manualReviewDraftFormRef"
        :model="manualReviewDraftForm"
        :rules="manualReviewDraftRules"
        label-position="top"
        class="manual-draft-form"
      >
        <el-form-item label="原计划名称（依据原始凭证）" prop="plan_name">
          <el-input v-model="manualReviewDraftForm.plan_name" maxlength="80" show-word-limit placeholder="填写凭证对应的原计划名称" />
        </el-form-item>
        <el-form-item label="原始金额（元，必须与订单金额完全一致）" prop="amount_yuan">
          <el-input v-model="manualReviewDraftForm.amount_yuan" inputmode="decimal" placeholder="例如 199.00；按原始付款凭证核对" />
          <small class="manual-draft-hint">订单记录金额：{{ formatMoney(Number(manualReviewDraftOrder?.total_amount ?? manualReviewDraftOrder?.amount ?? 0)) }}。此草稿校验输入金额与订单 amount、total_amount 两项都一致。</small>
        </el-form-item>
        <el-form-item label="原始订阅期限（天）" prop="duration_days">
          <el-input v-model="manualReviewDraftForm.duration_days" inputmode="numeric" placeholder="填写原付款对应的天数" />
        </el-form-item>
        <el-form-item prop="payment_occurred_attested">
          <el-checkbox v-model="manualReviewDraftForm.payment_occurred_attested">我已核对原始收款证据，并确认历史外部付款已经实际发生。</el-checkbox>
          <small class="manual-draft-hint">这是管理员的明确断言；系统不会访问外部渠道核验该声明。</small>
        </el-form-item>
        <el-form-item label="证据来源 / 原始凭证引用" prop="evidence_reference">
          <el-input v-model="manualReviewDraftForm.evidence_reference" type="textarea" :rows="2" maxlength="255" show-word-limit placeholder="填写可追溯的原始收款凭证或流水引用；不要填写推测值" />
        </el-form-item>
        <el-form-item label="核验原因" prop="reason">
          <el-input v-model="manualReviewDraftForm.reason" type="textarea" :rows="3" maxlength="1000" show-word-limit placeholder="说明为什么需要人工核验，以及依据了哪些原始材料" />
        </el-form-item>
      </el-form>
      <section v-if="manualReviewDraftPreview" class="manual-draft-preview">
        <strong>本地草稿预览（尚未保存）</strong>
        <p>{{ manualReviewDraftPreview.plan_name }} · {{ manualReviewDraftPreview.duration_days }} 天 · {{ formatMoney(manualReviewDraftPreview.amount_cents) }}</p>
        <p>凭证引用：{{ manualReviewDraftPreview.evidence_reference }}</p>
        <p>提交后订单状态会变为 manually_verified；管理员声明历史外部付款已发生，但系统仍不生成渠道付款流水，且订阅到期按当前有效期与所填期限计算。</p>
      </section>
      <section v-if="manualReviewSavedResult" class="manual-review-saved">
        <div class="saved-review-heading"><strong>已保存人工核验与审计记录</strong><el-tag type="warning">manually_verified · 非渠道自动确认</el-tag></div>
        <p><b>原始条款：</b>{{ manualReviewSavedResult.manual_review?.plan_name }} · {{ manualReviewSavedResult.manual_review?.duration_days }} 天 · {{ formatMoney(manualReviewSavedResult.manual_review?.amount_cents) }}</p>
        <p><b>付款断言：</b>{{ manualReviewSavedResult.manual_review?.payment_occurred_attested ? '管理员确认历史外部付款已实际发生' : '未声明' }}</p>
        <p><b>证据 / 理由：</b>{{ manualReviewSavedResult.manual_review?.evidence_reference }} · {{ manualReviewSavedResult.manual_review?.reason }}</p>
        <p><b>操作者：</b>{{ manualReviewSavedResult.manual_review?.actor_role }} #{{ manualReviewSavedResult.manual_review?.actor_id }} · {{ formatTime(manualReviewSavedResult.audit?.reviewed_at) }}</p>
        <p><b>订单状态：</b>{{ manualReviewSavedResult.audit?.order_status_before }} → {{ manualReviewSavedResult.audit?.order_status_after }}；PaidAt 保持空值。</p>
        <p><b>订阅：</b>{{ manualReviewSavedResult.audit?.subscription_status_before }} → {{ manualReviewSavedResult.audit?.subscription_status_after }}；到期 {{ formatTime(manualReviewSavedResult.audit?.subscription_expire_at_before) }} → {{ formatTime(manualReviewSavedResult.audit?.subscription_expire_at_after) }}</p>
        <p>管理员承担所填证据核验与付款发生声明；系统没有验证外部凭证，也未生成渠道交易号、PaidAt 或 provider PaymentRecord。</p>
      </section>
      <template #footer>
        <el-button @click="manualReviewDraftVisible = false">关闭</el-button>
        <el-button v-if="!manualReviewSavedResult" type="warning" :disabled="Boolean(manualReviewDraftPreview)" @click="prepareManualReviewDraft">整理本页草稿</el-button>
        <el-button v-if="manualReviewDraftPreview && !manualReviewSavedResult" type="danger" :loading="savingManualReview" @click="submitManualReview">提交人工复核并更新订阅</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { confirmMerchantSubscriptionPayment, exportAdminOrders, fetchManualSubscriptionReviews, fetchMerchants, fetchOrders, reviewLegacyMerchantSubscriptionOrder } from '../../api/modules'
import DataPanel from '../../components/design/DataPanel.vue'
import MetricCard from '../../components/design/MetricCard.vue'
import PageHero from '../../components/design/PageHero.vue'
import { useAuthStore } from '../../stores/auth'
import { downloadBlob } from '../../utils/download'
import {
  hasValidMerchantSubscriptionSnapshot,
  isLegacyMerchantSubscriptionOrderWithoutSnapshot,
  merchantSubscriptionSnapshotLabel,
  merchantSubscriptionSnapshotPrice
} from '../../utils/merchantSubscriptionOrderSnapshot'
import { providerPaidSubscriptionOrders, providerPaidSubscriptionRevenueCents } from '../../utils/subscriptionRevenue'

const loading = ref(false)
const exporting = ref(false)
const auth = useAuthStore()
const route = useRoute()
const confirmingOrderId = ref(null)
const orders = ref([])
// 不再拉全量商家数组（旧 fetchMerchants page_size=500 实际只回最新 10 家，
// 老商家订阅数/到期数全漏计）。订阅中商家数与 7 天内到期列表改为调
// 服务端筛选（见 load），merchantById 只查到期列表缓存作回落。
const activeMerchantCount = ref(0)
const expiringMerchants = ref([])
const manualReviews = ref([])
const dateRange = ref([])
const keyword = ref('')
const statusFilter = ref('')
const lastConfirmed = ref(null)
const manualReviewDraftVisible = ref(false)
const manualReviewDraftFormRef = ref(null)
const manualReviewDraftOrder = ref(null)
const manualReviewDraftPreview = ref(null)
const manualReviewSavedResult = ref(null)
const savingManualReview = ref(false)
const emptyManualReviewDraftForm = () => ({
  plan_name: '',
  amount_yuan: '',
  duration_days: '',
  evidence_reference: '',
  reason: '',
  payment_occurred_attested: false
})
const manualReviewDraftForm = ref(emptyManualReviewDraftForm())
const manualReviewDraftRules = {
  plan_name: [
    { required: true, whitespace: true, message: '请填写原计划名称', trigger: 'blur' },
    { max: 80, message: '原计划名称不能超过 80 个字符', trigger: 'blur' }
  ],
  amount_yuan: [{ required: true, validator: validateDraftAmount, trigger: 'blur' }],
  duration_days: [{ required: true, validator: validateDraftDuration, trigger: 'blur' }],
  payment_occurred_attested: [{ validator: validatePaymentAttestation, trigger: 'change' }],
  evidence_reference: [
    { required: true, whitespace: true, message: '请填写原始凭证来源或引用', trigger: 'blur' },
    { max: 255, message: '凭证引用不能超过 255 个字符', trigger: 'blur' }
  ],
  reason: [
    { required: true, whitespace: true, message: '请填写核验原因', trigger: 'blur' },
    { max: 1000, message: '核验原因不能超过 1000 个字符', trigger: 'blur' }
  ]
}
const subscriptionOrders = computed(() => orders.value.filter((item) => item.order_type === 'merchant_subscription'))
const filteredSubscriptionOrders = computed(() => subscriptionOrders.value.filter(matchFilters))
const pendingOrders = computed(() => filteredSubscriptionOrders.value.filter((item) => item.status === 'pending'))
const expiredUnpaidOrders = computed(() => filteredSubscriptionOrders.value.filter(isExpiredUnpaidOrder))
const pendingSnapshotReviewCount = computed(() => pendingOrders.value.filter((item) => !hasValidSubscriptionSnapshot(item)).length)
const paidOrders = computed(() => providerPaidSubscriptionOrders(filteredSubscriptionOrders.value))
const manualReviewedOrders = computed(() => manualReviews.value.filter(matchFilters))
const confirmedIncome = computed(() => providerPaidSubscriptionRevenueCents(paidOrders.value, orderAmount))
const pendingAmount = computed(() => pendingOrders.value.reduce((sum, item) => sum + orderAmount(item), 0))
const expiringMerchantIds = computed(() => new Set(expiringMerchants.value.map((item) => Number(item.id))))
const quickFilters = computed(() => [
  { label: '全部订阅', value: '', count: filteredSubscriptionOrders.value.length, hint: '当前筛选范围' },
  { label: '待确认', value: 'pending', count: subscriptionOrders.value.filter((item) => item.status === 'pending').length, hint: '需要核对到账' },
  { label: '静态码过期未付款', value: 'expired_unpaid', count: subscriptionOrders.value.filter(isExpiredUnpaidOrder).length, hint: '等待管理员核对实际到账' },
  { label: '已确认', value: 'paid', count: subscriptionOrders.value.filter((item) => item.status === 'paid').length, hint: '进入平台收入' },
  { label: '人工核验', value: 'manually_verified', count: manualReviews.value.length, hint: '独立台账，不计收入' },
  { label: '即将到期', value: 'expiring', count: expiringMerchants.value.length, hint: '7 天内需续费跟进' }
])
const workflowCards = [
  { label: 'STEP 1', title: '商家生成付款单', text: '商家端选择月付、年付或技术支持版后，生成平台收款码订阅订单。' },
  { label: 'STEP 2', title: '平台核对到账', text: '平台只核对自有支付宝或微信账户是否到账，不处理顾客和商家的门店交易款。' },
  { label: 'STEP 3', title: '确认并开通', text: '点击确认到账后，系统记录订阅收入，并自动开通或续期商家工作台。' }
]

const loadExpiringMerchants = async () => {
  const collected = []
  let page = 1
  let total = 0
  do {
    const res = await fetchMerchants({ page, page_size: 100, subscription: 'active', expiring_within_days: 7, sort_by: 'subscription_expire_at', sort_order: 'asc' })
    const list = res.data?.list || []
    total = res.data?.total ?? 0
    if (!list.length) break
    collected.push(...list)
    page += 1
  } while (collected.length < total)
  return collected
}

const load = async () => {
  loading.value = true
  try {
    const [ordersRes, activeRes, expiring, manualReviewsRes] = await Promise.all([
      fetchOrders(),
      fetchMerchants({ page: 1, page_size: 1, subscription: 'active' }),
      loadExpiringMerchants(),
      fetchManualSubscriptionReviews()
    ])
    orders.value = ordersRes.data || []
    activeMerchantCount.value = activeRes.data?.total ?? 0
    expiringMerchants.value = expiring
    manualReviews.value = manualReviewsRes.data?.list || []
  } finally {
    loading.value = false
  }
}

const matchFilters = (item) => {
  if (statusFilter.value === 'expiring') {
    if (!item.merchant_id || !expiringMerchantIds.value.has(Number(item.merchant_id))) return false
  }
  if (statusFilter.value === 'expired_unpaid' && !isExpiredUnpaidOrder(item)) return false
  if (statusFilter.value && !['expiring', 'expired_unpaid'].includes(statusFilter.value) && item.status !== statusFilter.value) return false
  if (!matchDate(item.created_at)) return false
  if (keyword.value) {
    const text = `${item.order_no || ''} ${item.merchant?.name || ''} ${planName(item)}`.toLowerCase()
    if (!text.includes(keyword.value.toLowerCase())) return false
  }
  return true
}

const resetFilters = () => {
  dateRange.value = []
  keyword.value = ''
  statusFilter.value = ''
}

const confirmSubscription = async (row) => {
  if (!canConfirmSubscription(row)) {
    ElMessage.warning('该订单不符合普通确认条件；请先核验原始条款和收款凭证。')
    return
  }
  const expiredUnpaid = isExpiredUnpaidOrder(row)
  const confirmationMessage = expiredUnpaid
    ? `订单 ${row.order_no} 已因超过 30 分钟未确认付款而关闭。请先核对平台静态二维码收款账户的实际流水；只有确认这笔款项已到账后，才继续人工对账并开通“${row.merchant?.name || row.merchant_id || '-'}”的订阅。`
    : `确认“${row.merchant?.name || row.merchant_id || '-'}”订阅款已到账，并立即开通套餐吗？`
  await ElMessageBox.confirm(confirmationMessage, expiredUnpaid ? '人工核对过期订阅收款' : '确认订阅收款', {
    type: 'warning',
    confirmButtonText: expiredUnpaid ? '已核对到账并开通' : '确认到账并开通',
    cancelButtonText: '返回核对'
  })
  confirmingOrderId.value = row.id
  try {
    const res = await confirmMerchantSubscriptionPayment(row.id, { remark: '订阅营收页确认到账' })
    const order = res.data?.order || {}
    lastConfirmed.value = {
      order_no: order.order_no || row.order_no,
      merchant_name: order.merchant?.name || row.merchant?.name || row.merchant_id || '-',
      plan_name: planName(order.id ? order : row),
      expire_at: order.subscription_end_at
    }
    ElMessage.success(`已确认到账并开通订阅，到期时间：${formatTime(order.subscription_end_at)}`)
    await load()
  } catch (error) {
    ElMessage.error(error?.response?.data?.message || '确认到账失败，请刷新后重试')
  } finally {
    confirmingOrderId.value = null
  }
}

const exportRevenue = async () => {
  exporting.value = true
  try {
    if (statusFilter.value === 'expired_unpaid') {
      ElMessage.info('已过期未付款订单的专用导出暂未提供；请在本页表格中进行人工核对。')
      return
    }
    if (statusFilter.value === 'expiring' && !expiringMerchantIds.value.size) {
      ElMessage.info('当前没有 7 天内到期的商家可导出。')
      return
    }
    if (statusFilter.value === 'manually_verified') {
      ElMessage.info('人工核验是独立台账，不属于订阅订单导出范围。')
      return
    }
    const merchantIds = statusFilter.value === 'expiring' ? Array.from(expiringMerchantIds.value).join(',') : undefined
    const blob = await exportAdminOrders({
      order_type: 'merchant_subscription',
      date_from: dateRange.value?.[0] || undefined,
      date_to: dateRange.value?.[1] || undefined,
      keyword: keyword.value || undefined,
      status: ['expiring', 'manually_verified'].includes(statusFilter.value) ? undefined : statusFilter.value || undefined,
      merchant_ids: merchantIds
    })
    downloadBlob(blob, 'platform-subscription-revenue.xlsx')
    ElMessage.success('订阅账单已由服务端按当前筛选生成并记录审计。')
  } catch (error) {
    ElMessage.error(error?.response?.data?.message || '订阅账单导出失败')
  } finally {
    exporting.value = false
  }
}

const orderAmount = (row) => row?.manual_review?.amount_cents ?? merchantSubscriptionSnapshotPrice(row) ?? Number(row.total_amount ?? row.amount ?? 0)
// 仅查 7 天内到期列表缓存：主路径是订单自带的 row.merchant
// （服务端已嵌套订阅状态与到期日），这里只是回落。
const merchantById = (id) => expiringMerchants.value.find((item) => Number(item.id) === Number(id)) || null
const hasValidSubscriptionSnapshot = (row) => hasValidMerchantSubscriptionSnapshot(row)
const isExpiredUnpaidOrder = (row) => row?.order_type === 'merchant_subscription' && row?.payment_channel === 'platform_qr' && row?.expired_unpaid === true && row?.paid_at == null
const canConfirmSubscription = (row) => hasValidSubscriptionSnapshot(row) && (row?.status === 'pending' || (isExpiredUnpaidOrder(row) && row?.payment_channel === 'platform_qr'))
const canOpenManualReviewDraft = (row) => row?.order_type === 'merchant_subscription' && (row.status === 'pending' || (isExpiredUnpaidOrder(row) && row?.payment_channel === 'platform_qr')) && isLegacyMerchantSubscriptionOrderWithoutSnapshot(row)
const openManualReviewDraft = (row) => {
  if (!canOpenManualReviewDraft(row)) return
  manualReviewDraftOrder.value = row
  manualReviewDraftForm.value = emptyManualReviewDraftForm()
  manualReviewDraftPreview.value = null
  manualReviewSavedResult.value = null
  manualReviewDraftVisible.value = true
}
const clearManualReviewDraft = () => {
  manualReviewDraftOrder.value = null
  manualReviewDraftForm.value = emptyManualReviewDraftForm()
  manualReviewDraftPreview.value = null
  manualReviewSavedResult.value = null
  manualReviewDraftFormRef.value?.clearValidate?.()
}
function parseDraftYuanToCents(text) {
  const match = String(text ?? '').trim().match(/^(\d+)(?:\.(\d{1,2}))?$/)
  if (!match) return null
  const cents = Number(match[1]) * 100 + Number((match[2] || '').padEnd(2, '0'))
  return Number.isSafeInteger(cents) ? cents : null
}

function validateDraftAmount(_rule, value, callback) {
  const text = String(value ?? '').trim()
  const amountCents = parseDraftYuanToCents(text)
  const order = manualReviewDraftOrder.value
  const orderAmount = Number(order?.amount)
  const orderTotal = Number(order?.total_amount)
  if (amountCents === null || amountCents <= 0) {
    callback(new Error('请输入大于 0、最多精确到分的金额'))
    return
  }
  if (!Number.isSafeInteger(orderAmount) || !Number.isSafeInteger(orderTotal) || amountCents !== orderAmount || amountCents !== orderTotal) {
    callback(new Error('金额必须同时与订单 amount 和 total_amount 完全一致'))
    return
  }
  callback()
}
function validateDraftDuration(_rule, value, callback) {
  const duration = Number(value)
  if (!Number.isInteger(duration) || duration <= 0) {
    callback(new Error('期限必须为大于 0 的整数天数'))
    return
  }
  callback()
}
function validatePaymentAttestation(_rule, value, callback) {
  if (value !== true) {
    callback(new Error('必须确认已核对原始证据且历史外部付款已经实际发生'))
    return
  }
  callback()
}
const prepareManualReviewDraft = () => {
  if (!manualReviewDraftFormRef.value || !manualReviewDraftOrder.value) return
  manualReviewDraftFormRef.value.validate((valid) => {
    if (!valid) return
    manualReviewDraftPreview.value = {
      order_no: manualReviewDraftOrder.value.order_no,
      plan_name: manualReviewDraftForm.value.plan_name.trim(),
      amount_cents: parseDraftYuanToCents(manualReviewDraftForm.value.amount_yuan),
      duration_days: Number(manualReviewDraftForm.value.duration_days),
      evidence_reference: manualReviewDraftForm.value.evidence_reference.trim(),
      reason: manualReviewDraftForm.value.reason.trim()
    }
    ElMessage.success('草稿仅保存在当前页面；尚未保存复核记录或更新订阅')
  })
}
const submitManualReview = async () => {
  if (!manualReviewDraftOrder.value || !manualReviewDraftPreview.value || !manualReviewDraftFormRef.value || savingManualReview.value) return
  let valid = false
  try {
    valid = await manualReviewDraftFormRef.value.validate()
  } catch {
    return
  }
  if (!valid) return
  const amountCents = parseDraftYuanToCents(manualReviewDraftForm.value.amount_yuan)
  if (!Number.isSafeInteger(amountCents)) return
  try {
    await ElMessageBox.confirm(
      `你将声明已核对历史订单 ${manualReviewDraftOrder.value.order_no} 的原始证据，并确认外部付款已经实际发生；系统会保存该断言和审计，按 ${manualReviewDraftForm.value.duration_days} 天更新订阅。系统不会调用支付渠道、生成 provider PaymentRecord、PaidAt 或渠道交易号。确认提交？`,
      '提交历史订阅人工复核',
      { type: 'warning', confirmButtonText: '保存复核并更新订阅', cancelButtonText: '返回检查' }
    )
  } catch {
    return
  }
  savingManualReview.value = true
  try {
    const res = await reviewLegacyMerchantSubscriptionOrder(manualReviewDraftOrder.value.id, {
      plan_name: manualReviewDraftForm.value.plan_name.trim(),
      amount_cents: amountCents,
      duration_days: Number(manualReviewDraftForm.value.duration_days),
      payment_occurred_attested: manualReviewDraftForm.value.payment_occurred_attested === true,
      evidence_reference: manualReviewDraftForm.value.evidence_reference.trim(),
      reason: manualReviewDraftForm.value.reason.trim()
    })
    manualReviewSavedResult.value = res.data || res
    manualReviewDraftPreview.value = null
    ElMessage.success('已保存 manually_verified 与审计记录；不会创建渠道支付流水')
    try { await load() } catch { /* The saved response remains visible for this session. */ }
  } catch (error) {
    ElMessage.error(error?.response?.data?.message || '保存人工复核失败；请刷新后核对订单状态')
  } finally {
    savingManualReview.value = false
  }
}
const planName = (row) => row?.manual_review?.plan_name
  ? `${row.manual_review.plan_name} · ${row.manual_review.duration_days} 天（人工核验）`
  : merchantSubscriptionSnapshotLabel(row)
const expectedExpireText = (row) => {
  if (!hasValidSubscriptionSnapshot(row)) return '快照缺失，需人工复核'
  const merchant = row.merchant || merchantById(row.merchant_id)
  const currentExpire = dateValue(merchant?.subscription_expire_at || merchant?.subscription_expired_at)
  const duration = Number(row.merchant_plan_duration_days_snapshot || 0)
  if (!duration) return '快照缺失，需人工复核'
  const base = Math.max(currentExpire, Date.now())
  return formatTime(new Date(base + duration * 24 * 3600000).toISOString())
}
const merchantSubscriptionLabel = (row) => {
  const merchant = row.merchant || merchantById(row.merchant_id)
  if (!merchant) return row.status === 'paid' ? '商家状态待核对' : '待确认'
  if (isSubscriptionActive(merchant)) return '服务中'
  return merchant.subscription_status === 'paused' ? '已暂停' : '未生效'
}
const merchantSubscriptionType = (row) => merchantSubscriptionLabel(row) === '服务中' ? 'success' : 'warning'
const subscriptionPaymentMarked = (row) => Array.isArray(row.operation_logs) && row.operation_logs.some((item) => item.action === 'merchant_subscription_paid_marked')
const matchDate = (value) => {
  if (!dateRange.value?.length || !value) return true
  const day = String(value).slice(0, 10)
  return day >= dateRange.value[0] && day <= dateRange.value[1]
}
const isSubscriptionActive = (row) => {
  const expire = dateValue(row.subscription_expire_at || row.subscription_expired_at)
  return row.subscription_status === 'active' && expire > Date.now()
}
const dateValue = (value) => value ? new Date(value).getTime() || 0 : 0
const formatMoney = (value) => `¥${(Number(value || 0) / 100).toFixed(2)}`
const formatTime = (value) => value ? String(value).replace('T', ' ').slice(0, 19) : '-'
const statusLabel = (status) => ({ pending: '待确认收款', paid: '已确认收入', manually_verified: '人工核验（外部凭证）', failed: '支付失败', closed: '已关闭' }[status] || status || '-')
const statusType = (status) => ({ pending: 'warning', paid: 'success', manually_verified: 'warning', failed: 'danger', closed: 'info' }[status] || 'info')
const paymentChannelLabel = (channel) => ({
  platform_qr: '平台收款码',
  alipay: '支付宝',
  mock_wechat: '微信/模拟',
  merchant_qr: '商家收款码',
  admin_manual_verified: '管理员人工核验（非渠道自动确认）'
}[channel] || channel || '-')

onMounted(() => {
  keyword.value = route.query.keyword || ''
  statusFilter.value = route.query.status || ''
  load()
})
</script>

<style scoped>
.subscription-revenue-page {
  display: grid;
  gap: 18px;
}

.metric-grid,
.scope-alert,
.confirm-result,
.summary-strip,
.workflow-panel,
.split-grid {
  display: grid;
  gap: 14px;
}

.metric-grid {
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.manual-review-actions {
  display: grid;
  justify-items: start;
  gap: 6px;
}

.manual-draft-warning {
  margin-bottom: 14px;
}

.manual-draft-order {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  margin: 14px 0 18px;
  padding: 14px;
  border: 1px solid #fed7aa;
  border-radius: 8px;
  background: #fff7ed;
}

.manual-draft-order span,
.manual-draft-order strong {
  display: block;
}

.manual-draft-order span,
.manual-draft-hint {
  color: #64748b;
  font-size: 12px;
}

.manual-draft-order strong {
  margin-top: 5px;
  color: #0f172a;
}

.manual-draft-hint {
  display: block;
  line-height: 1.5;
}

.manual-draft-preview {
  padding: 12px 14px;
  border: 1px solid #bfdbfe;
  border-radius: 8px;
  background: #eff6ff;
  color: #1e3a8a;
}

.manual-draft-preview p {
  margin: 6px 0 0;
}

.manual-review-saved {
  margin-top: 14px;
  padding: 14px 16px;
  border: 1px solid #fbbf24;
  border-radius: 8px;
  background: #fffbeb;
  color: #78350f;
}

.manual-review-saved p {
  margin: 8px 0 0;
  line-height: 1.55;
}

.saved-review-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.saved-review-note {
  margin-top: 12px;
}

.scope-alert,
.confirm-result {
  align-items: center;
  border-radius: 8px;
}

.scope-alert {
  grid-template-columns: 220px minmax(0, 1fr);
  padding: 16px;
  border: 1px solid #bfdbfe;
  background: linear-gradient(90deg, #eff6ff, #ffffff);
  color: #475569;
}

.scope-alert strong {
  color: #0f172a;
}

.confirm-result {
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) auto;
  padding: 16px;
  border: 1px solid #bbf7d0;
  background: #f0fdf4;
}

.confirm-result span,
.confirm-result strong,
.confirm-result small {
  display: block;
}

.confirm-result span {
  color: #16a34a;
  font-size: 12px;
  font-weight: 900;
}

.confirm-result strong {
  margin-top: 6px;
  color: #0f172a;
  font-size: 18px;
}

.confirm-result small {
  margin-top: 4px;
  color: #64748b;
}

.summary-strip {
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.workflow-panel {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.summary-strip button {
  min-height: 108px;
  padding: 14px;
  text-align: left;
  cursor: pointer;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  background: #ffffff;
  transition: 0.2s ease;
}

.summary-strip button.active {
  border-color: #2563eb;
  background: #eff6ff;
  box-shadow: 0 12px 28px rgba(37, 99, 235, 0.12);
}

.summary-strip span,
.summary-strip strong,
.summary-strip small {
  display: block;
}

.summary-strip span {
  color: #64748b;
  font-size: 12px;
  font-weight: 800;
}

.summary-strip strong {
  margin: 8px 0 4px;
  color: #0f172a;
  font-size: 24px;
}

.summary-strip small {
  color: #64748b;
}

.workflow-panel article {
  min-height: 116px;
  padding: 16px;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  background: #ffffff;
}

.workflow-panel span,
.workflow-panel strong,
.workflow-panel p {
  display: block;
}

.workflow-panel span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
}

.workflow-panel strong {
  margin-top: 8px;
  color: #0f172a;
  font-size: 16px;
}

.workflow-panel p {
  margin: 8px 0 0;
  color: #64748b;
  line-height: 1.55;
}

.filters-row {
  display: grid;
  grid-template-columns: minmax(260px, 1.2fr) minmax(180px, 1fr) 180px auto;
  gap: 10px;
  align-items: center;
}

.split-grid {
  grid-template-columns: minmax(0, 1.15fr) minmax(0, 0.85fr);
}

@media (max-width: 1100px) {
  .metric-grid,
  .scope-alert,
  .confirm-result,
  .summary-strip,
  .filters-row,
  .workflow-panel,
  .split-grid {
    grid-template-columns: 1fr;
  }
}
</style>
