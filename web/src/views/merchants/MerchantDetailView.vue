<template>
  <div class="detail-stack">
    <section class="page-card hero-card">
      <div>
        <el-button text class="back-button" @click="router.push('/admin/merchants')">返回商家列表</el-button>
        <div class="eyebrow">MERCHANT OPS DESK</div>
        <h2 class="page-title">{{ merchant.name || '商家详情' }}</h2>
        <p class="page-desc">集中处理订阅、收款审核、人工结算、订单流水、退款售后、风险备注和操作审计。</p>
      </div>
      <div class="hero-actions">
        <el-tag size="large" :type="statusType(merchant.status)">{{ statusLabel(merchant.status) }}</el-tag>
        <el-button type="primary" :loading="loading" @click="load">刷新档案</el-button>
      </div>
    </section>

    <section class="summary-grid">
      <div v-for="item in summaryCards" :key="item.label" class="summary-card" :class="item.className">
        <span>{{ item.label }}</span>
        <strong>{{ item.value }}</strong>
        <small>{{ item.hint }}</small>
      </div>
    </section>

    <section class="page-card">
      <el-tabs v-model="activeTab">
        <el-tab-pane label="运营概览" name="overview">
          <div class="grid-2">
            <div class="inner-card">
              <div class="toolbar">
                <div>
                  <h3>订阅权限</h3>
                  <p>平台可在测试、售后或商务场景中手动开通、延长、减少或停用商家订阅。</p>
                </div>
                <div class="action-row">
                  <el-button type="primary" @click="openSubscriptionDialog">开通 / 调整</el-button>
                  <el-button v-if="merchant.subscription_status === 'active'" type="danger" plain @click="stopSubscriptionNow">停用</el-button>
                </div>
              </div>
              <div class="info-list">
                <div><span>当前套餐</span><strong>{{ currentMerchantPlanName }}</strong></div>
                <div><span>订阅状态</span><strong>{{ subscriptionValid ? '订阅有效' : '未订阅 / 已过期' }}</strong></div>
                <div><span>到期时间</span><strong>{{ formatTime(merchant.subscription_expire_at || merchant.subscription_expired_at) }}</strong></div>
                <div><span>最近开通记录</span><strong>{{ latestSubscriptionSummary }}</strong></div>
                <div><span>订阅备注</span><strong>{{ merchant.subscription_note || '-' }}</strong></div>
              </div>
            </div>

            <div class="inner-card">
              <div class="toolbar">
                <div>
                  <h3>收款资料审核</h3>
                  <p>当前阶段建议作为商家自有收款资料和人工对账依据；平台不做资金清分，后续接入服务商/分账后再自动切换。</p>
                </div>
                <el-tag :type="paymentAuditType(paymentConfig.audit_status)" size="large">
                  {{ paymentAuditLabel(paymentConfig.audit_status) }}
                </el-tag>
              </div>
              <el-alert
                class="payment-review-note"
                :type="paymentReviewTip.type"
                show-icon
                :closable="false"
                :title="paymentReviewTip.title"
                :description="paymentReviewTip.description"
              />
              <div class="payment-grid">
                <div><span>收款渠道</span><strong>{{ paymentChannelLabel(paymentConfig.channel) }}</strong></div>
                <div><span>收款模式</span><strong>{{ paymentModeLabel(paymentConfig.mode) }}</strong></div>
                <div><span>账户名称</span><strong>{{ paymentConfig.account_name || '-' }}</strong></div>
                <div><span>收款账号</span><strong>{{ maskAccount(paymentConfig.account_no) }}</strong></div>
                <div><span>支付宝收款码</span><strong>{{ paymentConfig.alipay_qr_code ? '已填写' : '未填写' }}</strong></div>
                <div><span>微信收款码</span><strong>{{ paymentConfig.wechat_qr_code ? '已填写' : '未填写' }}</strong></div>
                <div><span>联系电话</span><strong>{{ paymentConfig.contact_phone || '-' }}</strong></div>
                <div><span>审核备注</span><strong>{{ paymentConfig.audit_remark || '-' }}</strong></div>
              </div>
              <div v-if="paymentQrItems.length" class="payment-qr-review">
                <article v-for="item in paymentQrItems" :key="item.key">
                  <div class="qr-review-head">
                    <strong>{{ item.label }}</strong>
                    <el-button text type="primary" @click="copyText(item.value, `${item.label}已复制`)">复制链接</el-button>
                  </div>
                  <div class="qr-review-box">
                    <img :src="qrDisplayUrl(item.value, item.key)" :alt="item.label" @error="markQrBroken(item.key)" />
                  </div>
                  <small>{{ qrBrokenMap[item.key] ? '图片加载失败，已按链接内容生成二维码预览。' : '审核时请核对二维码主体与账户名称是否一致。' }}</small>
                </article>
              </div>
              <div class="action-row payment-actions">
                <el-button type="success" :disabled="!canApprovePayment" @click="reviewPayment('approved')">审核通过并启用</el-button>
                <el-button type="danger" plain :disabled="!paymentConfig.id" @click="reviewPayment('rejected')">驳回</el-button>
                <el-button plain :disabled="!paymentConfig.id" @click="reviewPayment('pending')">设为待审核</el-button>
              </div>
            </div>
          </div>

          <section class="risk-desk">
            <div class="risk-header">
              <div>
                <h3>风险画像</h3>
                <p>{{ riskSummary }}</p>
              </div>
              <div class="risk-actions">
                <el-tag size="large" :type="riskLevelType">{{ riskLevel }}</el-tag>
                <el-button type="primary" plain @click="openFollowUpDialog()">新增跟进</el-button>
                <el-button v-if="merchant.status !== 'suspended'" type="danger" plain @click="changeMerchantStatus('suspended')">冻结商家</el-button>
                <el-button v-else type="success" plain @click="changeMerchantStatus('active')">恢复营业</el-button>
              </div>
            </div>

            <div class="risk-metrics">
              <div v-for="item in riskMetricCards" :key="item.label" :class="item.tone">
                <span>{{ item.label }}</span>
                <strong>{{ item.value }}</strong>
                <small>{{ item.hint }}</small>
              </div>
            </div>

            <div class="risk-columns">
              <div class="risk-panel">
                <div class="mini-title">
                  <h4>处理清单</h4>
                  <span>{{ riskTasks.length }} 项</span>
                </div>
                <div v-if="riskTasks.length" class="task-list">
                  <div v-for="task in riskTasks" :key="task.title" class="task-item" :class="task.tone">
                    <strong>{{ task.title }}</strong>
                    <p>{{ task.desc }}</p>
                  </div>
                </div>
                <el-empty v-else description="暂无需要立即处理的事项" />
              </div>

              <div class="risk-panel">
                <div class="mini-title">
                  <h4>最近处理记录</h4>
                  <span>人工跟进</span>
                </div>
                <div v-if="followUpPreviewRows.length" class="audit-list">
                  <div v-for="item in followUpPreviewRows" :key="item.id" class="audit-item">
                    <strong>{{ followUpTypeLabel(item.type) }} · {{ followUpPriorityLabel(item.priority) }}</strong>
                    <p>{{ item.content }}</p>
                    <small>{{ followUpStatusLabel(item.status) }} · {{ formatTime(item.created_at) }}</small>
                  </div>
                </div>
                <el-empty v-else description="暂无人工跟进记录" />
              </div>
            </div>
          </section>
        </el-tab-pane>

        <el-tab-pane label="结算记录" name="settlements">
          <div class="toolbar settlement-toolbar">
            <div>
              <h3>人工结算</h3>
              <p>MVP 阶段结算单主要用于订单归集和人工对账；如后续启用平台统一收款，需先完成服务商/分账资质和支付通道配置。</p>
            </div>
            <el-button type="primary" @click="openSettlementDialog">发起结算</el-button>
          </div>

          <section class="settlement-stats">
            <div><span>待结算订单</span><strong>{{ settlementPrepare.order_count || 0 }}</strong></div>
            <div><span>订单实付</span><strong>{{ formatMoney(settlementPrepare.total_amount_cents) }}</strong></div>
            <div><span>退款金额</span><strong>{{ formatMoney(settlementPrepare.refund_amount_cents) }}</strong></div>
            <div><span>待结算净额</span><strong>{{ formatMoney(settlementPrepare.net_amount_cents) }}</strong></div>
          </section>
          <el-alert
            class="settlement-net-alert"
            type="info"
            show-icon
            :closable="false"
            title="结算口径：只归集已确认实收且可结算订单，净结算 = 订单实付 - 已记录退款。"
          />

          <el-table :data="settlements" empty-text="暂无结算记录">
            <el-table-column prop="id" label="结算ID" width="90" />
            <el-table-column label="周期" min-width="220">
              <template #default="{ row }">{{ formatDate(row.settlement_period_start) }} - {{ formatDate(row.settlement_period_end) }}</template>
            </el-table-column>
            <el-table-column prop="order_count" label="订单数" width="90" />
            <el-table-column label="实付" width="120"><template #default="{ row }">{{ formatMoney(row.total_amount_cents) }}</template></el-table-column>
            <el-table-column label="退款" width="120"><template #default="{ row }">{{ formatMoney(row.refund_amount_cents) }}</template></el-table-column>
            <el-table-column label="净结算" width="130"><template #default="{ row }"><strong>{{ formatMoney(row.net_amount_cents) }}</strong></template></el-table-column>
            <el-table-column label="退款影响" min-width="150">
              <template #default="{ row }">
                <el-tag :type="Number(row.refund_amount_cents || 0) > 0 ? 'warning' : 'success'">
                  {{ Number(row.refund_amount_cents || 0) > 0 ? '已扣退款' : '无退款' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="状态" width="110">
              <template #default="{ row }">
                <el-tag :type="row.status === 'paid' ? 'success' : 'warning'">{{ row.status === 'paid' ? '已付款' : '待付款' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="付款时间" min-width="170"><template #default="{ row }">{{ formatTime(row.paid_at) }}</template></el-table-column>
            <el-table-column label="操作" width="240" fixed="right">
              <template #default="{ row }">
                <el-button size="small" @click="downloadSettlement(row)">导出</el-button>
                <el-button v-if="row.status === 'pending'" size="small" type="success" @click="markSettlementPaid(row)">标记已付款</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>

        <el-tab-pane label="门店订单" name="orders">
          <el-table :data="storeOrders" empty-text="暂无门店订单">
            <el-table-column prop="order_no" label="订单号" min-width="180" />
            <el-table-column label="门店" min-width="130"><template #default="{ row }">{{ row.store?.name || '-' }}</template></el-table-column>
            <el-table-column prop="customer_phone" label="顾客手机号" min-width="130" />
            <el-table-column label="实付金额" width="120"><template #default="{ row }">{{ formatMoney(row.total_amount || row.amount) }}</template></el-table-column>
            <el-table-column label="优惠" width="110"><template #default="{ row }">{{ formatMoney(row.discount_amount) }}</template></el-table-column>
            <el-table-column label="已退款" width="110"><template #default="{ row }">{{ formatMoney(row.refunded_amount) }}</template></el-table-column>
            <el-table-column label="退款后净额" width="130"><template #default="{ row }"><strong>{{ formatMoney(orderNetAmount(row)) }}</strong></template></el-table-column>
            <el-table-column label="结算状态" width="110">
              <template #default="{ row }">
                <el-tag :type="row.settlement_id ? 'success' : 'info'">{{ row.settlement_id ? '已归集' : '未结算' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="订单状态" width="110">
              <template #default="{ row }"><el-tag :type="orderStatusType(row.status)">{{ orderStatusLabel(row.status) }}</el-tag></template>
            </el-table-column>
            <el-table-column label="创建时间" min-width="170"><template #default="{ row }">{{ formatTime(row.created_at) }}</template></el-table-column>
          </el-table>
        </el-tab-pane>

        <el-tab-pane label="退款记录" name="refunds">
          <el-table :data="merchantRefunds" empty-text="暂无退款记录">
            <el-table-column prop="refund_no" label="退款号" min-width="160" />
            <el-table-column label="关联订单" min-width="180"><template #default="{ row }">{{ row.order?.order_no || '-' }}</template></el-table-column>
            <el-table-column label="门店" min-width="120"><template #default="{ row }">{{ row.order?.store?.name || '-' }}</template></el-table-column>
            <el-table-column label="退款金额" width="120"><template #default="{ row }">{{ formatMoney(row.amount) }}</template></el-table-column>
            <el-table-column prop="reason" label="退款原因" min-width="180" show-overflow-tooltip />
            <el-table-column prop="operator_role" label="操作方" width="100" />
            <el-table-column prop="status" label="状态" width="100" />
            <el-table-column label="时间" min-width="170"><template #default="{ row }">{{ formatTime(row.created_at) }}</template></el-table-column>
          </el-table>
        </el-tab-pane>

        <el-tab-pane label="订阅记录" name="subscriptionOrders">
          <el-table :data="subscriptionOrders" empty-text="暂无订阅记录">
            <el-table-column prop="order_no" label="订单号" min-width="180" />
            <el-table-column label="套餐" min-width="120"><template #default="{ row }">{{ merchantPlanName(row.merchant_plan || row.merchantPlan || {}) }}</template></el-table-column>
            <el-table-column label="金额" width="120"><template #default="{ row }">{{ formatMoney(row.total_amount || row.amount) }}</template></el-table-column>
            <el-table-column label="状态" width="110"><template #default="{ row }"><el-tag :type="orderStatusType(row.status)">{{ orderStatusLabel(row.status) }}</el-tag></template></el-table-column>
            <el-table-column label="开通到期" min-width="170"><template #default="{ row }">{{ formatTime(row.subscription_end_at) }}</template></el-table-column>
            <el-table-column label="付款时间" min-width="170"><template #default="{ row }">{{ formatTime(row.paid_at) }}</template></el-table-column>
            <el-table-column label="创建时间" min-width="170"><template #default="{ row }">{{ formatTime(row.created_at) }}</template></el-table-column>
          </el-table>
        </el-tab-pane>

        <el-tab-pane label="操作审计" name="audit">
          <el-table :data="auditLogs" empty-text="暂无审计记录">
            <el-table-column prop="actor_name" label="操作人" width="130" />
            <el-table-column prop="action" label="动作" min-width="180" />
            <el-table-column prop="target_name" label="对象" min-width="160" show-overflow-tooltip />
            <el-table-column prop="ip" label="IP" width="130" />
            <el-table-column label="时间" min-width="170"><template #default="{ row }">{{ formatTime(row.created_at) }}</template></el-table-column>
          </el-table>
        </el-tab-pane>

        <el-tab-pane label="跟进记录" name="followUps">
          <div class="toolbar settlement-toolbar">
            <div>
              <h3>异常工单 / 跟进记录</h3>
              <p>记录平台人工处理过程，例如电话联系商家、要求补充退款说明、暂停活动或安排复查。</p>
            </div>
            <el-button type="primary" @click="openFollowUpDialog()">新增跟进</el-button>
          </div>
          <el-table :data="followUps" empty-text="暂无跟进记录">
            <el-table-column label="类型" width="110">
              <template #default="{ row }">{{ followUpTypeLabel(row.type) }}</template>
            </el-table-column>
            <el-table-column label="优先级" width="100">
              <template #default="{ row }">
                <el-tag :type="followUpPriorityType(row.priority)">{{ followUpPriorityLabel(row.priority) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="content" label="跟进内容" min-width="260" show-overflow-tooltip />
            <el-table-column label="下次复查" width="160">
              <template #default="{ row }">{{ formatTime(row.next_follow_at) }}</template>
            </el-table-column>
            <el-table-column label="状态" width="100">
              <template #default="{ row }">
                <el-tag :type="row.status === 'closed' ? 'success' : 'warning'">{{ followUpStatusLabel(row.status) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="创建时间" width="160">
              <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
            </el-table-column>
            <el-table-column label="操作" width="110" fixed="right">
              <template #default="{ row }">
                <el-button v-if="row.status !== 'closed'" size="small" text type="success" @click="closeFollowUp(row)">标记处理</el-button>
                <el-button v-else size="small" text @click="reopenFollowUp(row)">重新打开</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>
      </el-tabs>
    </section>

    <el-dialog v-model="settlementDialogVisible" title="发起人工结算" width="760px">
      <section class="settlement-stats compact">
        <div><span>订单数</span><strong>{{ settlementPrepare.order_count || 0 }}</strong></div>
        <div><span>订单实付</span><strong>{{ formatMoney(settlementPrepare.total_amount_cents) }}</strong></div>
        <div><span>退款金额</span><strong>{{ formatMoney(settlementPrepare.refund_amount_cents) }}</strong></div>
        <div><span>净结算</span><strong>{{ formatMoney(settlementPrepare.net_amount_cents) }}</strong></div>
      </section>
      <el-alert
        class="settlement-net-alert"
        type="warning"
        show-icon
        :closable="false"
        title="请按净结算金额人工打款；已记录退款会从本次结算中扣除。"
      />
      <el-input v-model="settlementRemark" class="remark-input" placeholder="结算备注，例如：2026年5月第一期人工转账" />
      <el-table :data="settlementPrepare.orders || []" max-height="320" empty-text="暂无可结算订单">
        <el-table-column prop="order_no" label="订单号" min-width="180" />
        <el-table-column label="门店" min-width="120"><template #default="{ row }">{{ row.store?.name || '-' }}</template></el-table-column>
        <el-table-column label="实付" width="110"><template #default="{ row }">{{ formatMoney(row.total_amount || row.amount) }}</template></el-table-column>
        <el-table-column label="退款" width="110"><template #default="{ row }">{{ formatMoney(row.refunded_amount) }}</template></el-table-column>
        <el-table-column label="净额" width="110"><template #default="{ row }"><strong>{{ formatMoney(orderNetAmount(row)) }}</strong></template></el-table-column>
        <el-table-column label="状态" width="100"><template #default="{ row }">{{ orderStatusLabel(row.status) }}</template></el-table-column>
      </el-table>
      <template #footer>
        <el-button @click="settlementDialogVisible = false">取消</el-button>
        <el-button type="primary" :disabled="!settlementPrepare.order_count" @click="createSettlement">确认生成结算单</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="subscriptionDialogVisible" title="开通 / 调整订阅" width="520px">
      <el-form label-width="100px">
        <el-form-item label="套餐">
          <el-select v-model="subscriptionForm.plan">
            <el-option label="月付" value="month" />
            <el-option label="年付" value="year" />
          </el-select>
        </el-form-item>
        <el-form-item label="调整天数">
          <el-input-number v-model="subscriptionForm.duration_days" :min="-3650" :max="3650" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="subscriptionForm.note" maxlength="120" placeholder="例如：测试开通、补偿延长、线下付款" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="subscriptionDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submitSubscription">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="followUpDialogVisible" title="新增跟进记录" width="560px">
      <el-form label-width="92px">
        <el-form-item label="类型">
          <el-select v-model="followUpForm.type">
            <el-option label="风险处理" value="risk" />
            <el-option label="结算跟进" value="settlement" />
            <el-option label="退款售后" value="refund" />
            <el-option label="支付异常" value="payment" />
            <el-option label="订阅续费" value="subscription" />
            <el-option label="运营沟通" value="operation" />
          </el-select>
        </el-form-item>
        <el-form-item label="优先级">
          <el-select v-model="followUpForm.priority">
            <el-option label="普通" value="normal" />
            <el-option label="较低" value="low" />
            <el-option label="较高" value="high" />
            <el-option label="紧急" value="urgent" />
          </el-select>
        </el-form-item>
        <el-form-item label="下次复查">
          <el-date-picker v-model="followUpForm.next_follow_at" type="date" value-format="YYYY-MM-DD" placeholder="可选" />
        </el-form-item>
        <el-form-item label="跟进内容">
          <el-input
            v-model="followUpForm.content"
            type="textarea"
            :rows="5"
            maxlength="1000"
            show-word-limit
            placeholder="例如：已电话联系商家，要求今晚前确认退款原因；明天复查待接单订单是否清空。"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="followUpDialogVisible = false">取消</el-button>
        <el-button type="primary" :disabled="!followUpForm.content.trim()" @click="submitFollowUp">保存记录</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  createAdminMerchantSettlement,
  createAdminMerchantFollowUp,
  exportAdminMerchantSettlement,
  fetchAdminMerchantPaymentConfig,
  fetchAdminMerchantFollowUps,
  fetchAdminMerchantSettlementPrepare,
  fetchAdminMerchantSettlements,
  fetchAdminMerchantStoreOrders,
  fetchAdminMerchantStores,
  fetchAdminMerchantSubscriptionOrders,
  fetchAuditLogs,
  fetchMerchantDetail,
  fetchRefunds,
  markAdminMerchantSettlementPaid,
  openMerchantSubscription,
  reviewAdminMerchantPaymentConfig,
  stopMerchantSubscription,
  updateAdminMerchantFollowUpStatus,
  updateMerchantStatus
} from '../../api/modules'

const route = useRoute()
const router = useRouter()
const loading = ref(false)
const activeTab = ref('overview')
const merchant = ref({})
const paymentConfig = ref({})
const qrBrokenMap = ref({})
const stores = ref([])
const subscriptionOrders = ref([])
const storeOrders = ref([])
const refunds = ref([])
const auditLogs = ref([])
const settlements = ref([])
const followUps = ref([])
const settlementPrepare = ref({})
const subscriptionDialogVisible = ref(false)
const settlementDialogVisible = ref(false)
const followUpDialogVisible = ref(false)
const settlementRemark = ref('')
const subscriptionForm = reactive({ plan: 'month', duration_days: 30, note: '' })
const followUpForm = reactive({ type: 'risk', priority: 'normal', content: '', next_follow_at: '' })
const merchantID = () => route.params.id

const merchantRefunds = computed(() => refunds.value.filter((item) => Number(item.merchant_id || item.order?.merchant_id || 0) === Number(merchantID())))
const subscriptionValid = computed(() => {
  const expire = merchant.value.subscription_expire_at || merchant.value.subscription_expired_at
  return merchant.value.subscription_status === 'active' && expire && new Date(expire).getTime() > Date.now()
})
const storeOrderAmount = computed(() => storeOrders.value.reduce((sum, item) => sum + Number(item.total_amount || item.amount || 0), 0))
const refundedAmount = computed(() => merchantRefunds.value.reduce((sum, item) => sum + Number(item.amount || 0), 0))
const refundRate = computed(() => storeOrderAmount.value ? refundedAmount.value / storeOrderAmount.value : 0)
const cancelRate = computed(() => {
  if (!storeOrders.value.length) return 0
  return storeOrders.value.filter((item) => item.status === 'closed').length / storeOrders.value.length
})
const pendingPayCount = computed(() => storeOrders.value.filter((item) => item.status === 'pending').length)
const failedPayCount = computed(() => storeOrders.value.filter((item) => item.status === 'failed').length)
const waitingAcceptCount = computed(() => storeOrders.value.filter((item) => item.status === 'received').length)
const completedCount = computed(() => storeOrders.value.filter((item) => item.status === 'completed').length)
const unsettledCount = computed(() => storeOrders.value.filter((item) => item.order_type === 'store_order' && !item.settlement_id && ['accepted', 'completed'].includes(item.status)).length)
const pendingSettlementCount = computed(() => settlements.value.filter((item) => item.status === 'pending').length)
const riskLevel = computed(() => {
  if (merchant.value.status === 'suspended' || refundRate.value >= 0.35 || cancelRate.value >= 0.3 || failedPayCount.value >= 3) return '高风险'
  if (!subscriptionValid.value || refundRate.value >= 0.15 || merchantRefunds.value.length > 0 || cancelRate.value >= 0.15 || waitingAcceptCount.value > 0 || pendingSettlementCount.value > 0) return '关注'
  return '正常'
})
const riskLevelType = computed(() => riskLevel.value === '高风险' ? 'danger' : riskLevel.value === '关注' ? 'warning' : 'success')
const riskSummary = computed(() => {
  if (merchant.value.status === 'suspended') return '商家已被冻结，建议复核收款、订单和售后记录。'
  if (refundRate.value >= 0.35) return '退款率明显偏高，优先核对退款原因、顾客投诉和结算金额。'
  if (cancelRate.value >= 0.3) return '近期取消率偏高，建议检查履约能力、支付体验或商品信息准确性。'
  if (failedPayCount.value >= 3) return '支付失败订单偏多，建议检查支付配置、顾客端提示和门店收款链路。'
  if (waitingAcceptCount.value > 0) return '存在待接单订单，需要提醒商家尽快处理，避免顾客等待。'
  if (!subscriptionValid.value) return '商家订阅未生效或已过期，业务功能可能受限。'
  if (merchantRefunds.value.length > 0) return '存在退款记录，建议结合退款原因判断是否需要售后跟进。'
  return '当前未发现明显风险，可保持正常观察。'
})

const riskMetricCards = computed(() => [
  { label: '退款率', value: percent(refundRate.value), hint: `${merchantRefunds.value.length} 笔退款 / ${formatMoney(refundedAmount.value)}`, tone: refundRate.value >= 0.2 ? 'danger' : 'safe' },
  { label: '取消率', value: percent(cancelRate.value), hint: `${storeOrders.value.filter((item) => item.status === 'closed').length} 单关闭`, tone: cancelRate.value >= 0.3 ? 'danger' : cancelRate.value >= 0.15 ? 'warning' : 'safe' },
  { label: '待接单', value: `${waitingAcceptCount.value} 单`, hint: '顾客已支付，等待商家处理', tone: waitingAcceptCount.value ? 'warning' : 'safe' },
  { label: '待结算', value: `${unsettledCount.value} 单`, hint: `${pendingSettlementCount.value} 张结算单待付款`, tone: pendingSettlementCount.value || unsettledCount.value ? 'warning' : 'safe' },
  { label: '支付异常', value: `${failedPayCount.value} 单`, hint: `${pendingPayCount.value} 单仍待支付`, tone: failedPayCount.value ? 'danger' : 'safe' },
  { label: '打开跟进', value: `${openFollowUps.value.length} 条`, hint: `${completedCount.value} 单近期完成`, tone: openFollowUps.value.length ? 'warning' : 'safe' }
])

const riskTasks = computed(() => {
  const tasks = []
  if (merchant.value.status === 'suspended') {
    tasks.push({ title: '复核冻结原因', desc: '核对最近退款、支付失败、顾客投诉和结算记录，再决定恢复或下线。', tone: 'danger' })
  }
  if (refundRate.value >= 0.2) {
    tasks.push({ title: '核对退款异常', desc: '查看退款记录里的原因、操作方和关联订单，必要时暂停该商家营销活动。', tone: 'danger' })
  }
  if (cancelRate.value >= 0.15) {
    tasks.push({ title: '检查履约能力', desc: '重点看商品库存、营业时间、商家接单速度，减少顾客下单后关闭。', tone: 'warning' })
  }
  if (waitingAcceptCount.value > 0) {
    tasks.push({ title: '催办待接单', desc: '顾客已完成支付，建议联系商家尽快接单或主动退款。', tone: 'warning' })
  }
  if (failedPayCount.value > 0) {
    tasks.push({ title: '排查支付失败', desc: '检查支付配置、回跳页提示和订单状态流转，避免顾客重复付款。', tone: 'danger' })
  }
  if (!subscriptionValid.value) {
    tasks.push({ title: '确认订阅状态', desc: '订阅过期可能影响商家继续经营，需确认续费、停用或测试开通。', tone: 'warning' })
  }
  if (pendingSettlementCount.value > 0 || unsettledCount.value > 0) {
    tasks.push({ title: '处理结算', desc: '核对待结算订单、退款抵扣和人工转账状态，确保资金账能对上。', tone: 'primary' })
  }
  return tasks
})

const openFollowUps = computed(() => followUps.value.filter((item) => item.status !== 'closed'))
const followUpPreviewRows = computed(() => followUps.value.slice(0, 5))
const latestSubscriptionOrder = computed(() => [...subscriptionOrders.value]
  .filter((item) => item.status === 'paid')
  .sort((a, b) => dateValue(b.paid_at || b.updated_at || b.created_at) - dateValue(a.paid_at || a.updated_at || a.created_at))[0] || null)
const currentMerchantPlanName = computed(() => {
  if (merchant.value.merchant_plan?.name || merchant.value.merchantPlan?.name) {
    return merchantPlanName(merchant.value.merchant_plan || merchant.value.merchantPlan)
  }
  if (latestSubscriptionOrder.value?.merchant_plan || latestSubscriptionOrder.value?.merchantPlan) {
    return merchantPlanName(latestSubscriptionOrder.value.merchant_plan || latestSubscriptionOrder.value.merchantPlan)
  }
  return planLabel(merchant.value.subscription_plan)
})
const latestSubscriptionSummary = computed(() => {
  if (!latestSubscriptionOrder.value) return '暂无已确认开通记录'
  return `${merchantPlanName(latestSubscriptionOrder.value.merchant_plan || latestSubscriptionOrder.value.merchantPlan || {})} · ${formatTime(latestSubscriptionOrder.value.paid_at)}`
})
const paymentHasAnyQr = computed(() => Boolean(paymentConfig.value.alipay_qr_code || paymentConfig.value.wechat_qr_code))
const paymentQrItems = computed(() => {
  const items = []
  if (paymentConfig.value.alipay_qr_code) items.push({ key: 'alipay', label: '支付宝收款码', value: paymentConfig.value.alipay_qr_code })
  if (paymentConfig.value.wechat_qr_code) items.push({ key: 'wechat', label: '微信收款码', value: paymentConfig.value.wechat_qr_code })
  return items
})
const canApprovePayment = computed(() => {
  if (!paymentConfig.value.id) return false
  if (paymentConfig.value.mode === 'direct') return paymentHasAnyQr.value
  return true
})
const paymentReviewTip = computed(() => {
  if (!paymentConfig.value.id) {
    return {
      type: 'info',
      title: '商家还未提交收款资料',
      description: '商家未提交前，顾客端不会展示商家收款码。可提醒商家到收款设置上传支付宝或微信收款码。'
    }
  }
  if (paymentConfig.value.mode === 'direct' && !paymentHasAnyQr.value) {
    return {
      type: 'warning',
      title: '商家收款码模式缺少收款码',
      description: '请先让商家上传支付宝或微信收款码，再审核通过；否则顾客订单页无法展示付款入口。'
    }
  }
  if (paymentConfig.value.audit_status === 'approved' && paymentConfig.value.status === 'enabled') {
    return {
      type: 'success',
      title: '收款配置已启用',
      description: '顾客端订单页会展示已通过审核的商家收款码，顾客付款后由商家核对到账并确认收款。'
    }
  }
  if (paymentConfig.value.audit_status === 'rejected') {
    return {
      type: 'error',
      title: '收款资料已驳回',
      description: '当前不会对顾客展示收款码。请商家按审核备注修改后重新提交。'
    }
  }
  return {
    type: 'warning',
    title: '收款资料待审核',
    description: '审核通过并启用后，顾客端才会展示商家收款码；平台端不处理顾客订单确认到账。'
  }
})

const summaryCards = computed(() => [
  { label: '联系人手机号', value: merchant.value.contact_phone || '-', hint: '商家注册联系人', className: '' },
  { label: '订阅状态', value: subscriptionValid.value ? '订阅中' : '未订阅 / 已过期', hint: formatTime(merchant.value.subscription_expire_at || merchant.value.subscription_expired_at), className: subscriptionValid.value ? 'success' : 'warning' },
  { label: '门店数量', value: stores.value.length, hint: '当前商家门店', className: '' },
  { label: '近期订单', value: storeOrders.value.length, hint: '最近门店订单', className: '' },
  { label: '近期交易额', value: formatMoney(storeOrderAmount.value), hint: '顾客扫码订单实付', className: 'success' },
  { label: '退款金额', value: formatMoney(refundedAmount.value), hint: '当前商家退款留痕', className: 'warning' }
])

const load = async () => {
  loading.value = true
  try {
    const [merchantRes, paymentRes, storesRes, subscriptionRes, ordersRes, prepareRes, settlementsRes, followUpsRes, refundsRes, auditRes] = await Promise.all([
      fetchMerchantDetail(merchantID()),
      fetchAdminMerchantPaymentConfig(merchantID()),
      fetchAdminMerchantStores(merchantID(), { page: 1, page_size: 50 }),
      fetchAdminMerchantSubscriptionOrders(merchantID()),
      fetchAdminMerchantStoreOrders(merchantID(), { limit: 50 }),
      fetchAdminMerchantSettlementPrepare(merchantID()),
      fetchAdminMerchantSettlements(merchantID()),
      fetchAdminMerchantFollowUps(merchantID()),
      fetchRefunds(),
      fetchAuditLogs({ merchant_id: merchantID(), page: 1, page_size: 50 })
    ])
    merchant.value = merchantRes.data || {}
    paymentConfig.value = paymentRes.data.config || {}
    stores.value = storesRes.data.list || []
    subscriptionOrders.value = subscriptionRes.data.list || []
    storeOrders.value = ordersRes.data.list || []
    settlementPrepare.value = prepareRes.data || {}
    settlements.value = settlementsRes.data.list || []
    followUps.value = followUpsRes.data.list || []
    refunds.value = refundsRes.data || []
    auditLogs.value = auditRes.data.list || []
  } finally {
    loading.value = false
  }
}

const openSettlementDialog = async () => {
  const res = await fetchAdminMerchantSettlementPrepare(merchantID())
  settlementPrepare.value = res.data || {}
  settlementRemark.value = ''
  settlementDialogVisible.value = true
}

const createSettlement = async () => {
  await createAdminMerchantSettlement(merchantID(), { remark: settlementRemark.value })
  ElMessage.success('结算单已生成')
  settlementDialogVisible.value = false
  load()
}

const markSettlementPaid = async (row) => {
  const { value } = await ElMessageBox.prompt(`本次应按净结算 ${formatMoney(row.net_amount_cents)} 打款。请输入人工转账备注，例如付款流水号`, '标记已付款', {
    confirmButtonText: '确认',
    cancelButtonText: '取消',
    inputValue: row.remark || ''
  })
  await markAdminMerchantSettlementPaid(row.id, { remark: value || '' })
  ElMessage.success('已标记为已付款')
  load()
}

const downloadSettlement = async (row) => {
  const res = await exportAdminMerchantSettlement(row.id)
  const blob = new Blob([res.data], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' })
  const link = document.createElement('a')
  link.href = URL.createObjectURL(blob)
  link.download = `merchant-settlement-${row.id}.xlsx`
  link.click()
  URL.revokeObjectURL(link.href)
}

const openSubscriptionDialog = () => {
  subscriptionForm.plan = merchant.value.subscription_plan === 'year' ? 'year' : 'month'
  subscriptionForm.duration_days = subscriptionForm.plan === 'year' ? 365 : 30
  subscriptionForm.note = merchant.value.subscription_note || ''
  subscriptionDialogVisible.value = true
}

const submitSubscription = async () => {
  await openMerchantSubscription(merchantID(), { ...subscriptionForm })
  ElMessage.success('订阅已调整')
  subscriptionDialogVisible.value = false
  load()
}

const stopSubscriptionNow = async () => {
  await ElMessageBox.confirm('确认停用该商家的订阅权限吗？', '停用订阅', { type: 'warning' })
  await stopMerchantSubscription(merchantID())
  ElMessage.success('订阅已停用')
  load()
}

const reviewPayment = async (auditStatus) => {
  if (auditStatus === 'approved' && !canApprovePayment.value) {
    ElMessage.warning('商家收款码模式需要先上传支付宝或微信收款码')
    return
  }
  const status = auditStatus === 'approved' ? 'enabled' : 'disabled'
  const remark = auditStatus === 'approved'
    ? '平台审核通过，顾客端可展示商家收款码；平台只记录订单与对账，不代收资金。'
    : auditStatus === 'rejected'
      ? '平台审核驳回，请商家修改后重新提交。'
      : '平台已重新设为待审核。'
  await reviewAdminMerchantPaymentConfig(merchantID(), { audit_status: auditStatus, status, audit_remark: remark })
  ElMessage.success('收款资料审核状态已更新')
  load()
}

const changeMerchantStatus = async (status) => {
  const actionText = status === 'suspended' ? '冻结' : '恢复'
  await ElMessageBox.confirm(`确认${actionText}该商家吗？`, `${actionText}商家`, { type: status === 'suspended' ? 'warning' : 'success' })
  await updateMerchantStatus(merchantID(), { status })
  ElMessage.success(`商家已${actionText}`)
  load()
}

const openFollowUpDialog = (preset = {}) => {
  followUpForm.type = preset.type || 'risk'
  followUpForm.priority = preset.priority || (riskLevel.value === '高风险' ? 'high' : 'normal')
  followUpForm.content = preset.content || ''
  followUpForm.next_follow_at = preset.next_follow_at || ''
  followUpDialogVisible.value = true
}

const submitFollowUp = async () => {
  await createAdminMerchantFollowUp(merchantID(), { ...followUpForm, content: followUpForm.content.trim() })
  ElMessage.success('跟进记录已保存')
  followUpDialogVisible.value = false
  load()
}

const closeFollowUp = async (row) => {
  await updateAdminMerchantFollowUpStatus(row.id, { status: 'closed' })
  ElMessage.success('已标记为处理完成')
  load()
}

const reopenFollowUp = async (row) => {
  await updateAdminMerchantFollowUpStatus(row.id, { status: 'open' })
  ElMessage.success('跟进记录已重新打开')
  load()
}

const statusLabel = (status) => ({ pending: '待审核', active: '正常', suspended: '已冻结' }[status] || status || '-')
const statusType = (status) => ({ pending: 'warning', active: 'success', suspended: 'danger' }[status] || 'info')
const planLabel = (plan) => {
  const name = String(plan || '')
  if (!name || name === 'none') return '未开通'
  if (name === 'year' || name.includes('年')) return '年付服务版'
  if (name === 'month' || name.includes('月')) return '月付服务版'
  if (name.includes('技术')) return '技术支持版'
  return name
}
const merchantPlanName = (plan = {}) => {
  const name = String(plan.name || '')
  const duration = Number(plan.duration_days || 0)
  if (duration >= 365 || name.includes('年')) return '年付服务版'
  if (duration >= 30 || name.includes('月')) return '月付服务版'
  if (name.includes('技术')) return '技术支持版'
  return name || '-'
}
const paymentAuditLabel = (value) => ({ pending: '待审核', approved: '审核通过', rejected: '审核驳回' }[value] || '未提交')
const paymentAuditType = (value) => ({ pending: 'warning', approved: 'success', rejected: 'danger' }[value] || 'info')
const paymentChannelLabel = (value) => ({ alipay: '支付宝', wechat: '微信支付', bank: '银行卡' }[value] || '-')
const paymentModeLabel = (value) => ({ direct: '商家收款码模式', platform: '平台统一收款', service_provider: '服务商分账模式' }[value] || '-')
const maskAccount = (value) => {
  const text = String(value || '')
  if (text.length <= 4) return text || '-'
  return `${text.slice(0, 3)}****${text.slice(-4)}`
}
const buildQrImage = (value) => `https://api.qrserver.com/v1/create-qr-code/?size=260x260&data=${encodeURIComponent(value)}`
const qrDisplayUrl = (value, key = '') => {
  const text = String(value || '').trim()
  if (!text) return ''
  if (key && qrBrokenMap.value[key]) return buildQrImage(text)
  if (text.startsWith('data:image/')) return text
  if (/\.(png|jpe?g|webp|gif|svg)(\?.*)?$/i.test(text)) return text
  return buildQrImage(text)
}
const markQrBroken = (key) => {
  qrBrokenMap.value = { ...qrBrokenMap.value, [key]: true }
}
const copyText = async (text, message = '已复制') => {
  if (!text) return
  try {
    await navigator.clipboard.writeText(String(text))
    ElMessage.success(message)
  } catch {
    ElMessage.warning('当前浏览器不支持自动复制，请手动复制')
  }
}
const orderGrossAmount = (row = {}) => Number(row.total_amount || row.amount || row.total_amount_cents || 0)
const orderNetAmount = (row = {}) => Math.max(orderGrossAmount(row) - Number(row.refunded_amount || 0), 0)
const orderStatusLabel = (status) => ({
  pending: '待支付',
  paid: '已支付',
  received: '待接单',
  accepted: '已接单',
  completed: '已完成',
  closed: '已关闭',
  failed: '支付失败'
}[status] || status || '-')
const orderStatusType = (status) => ({
  pending: 'warning',
  paid: 'success',
  received: 'primary',
  accepted: 'success',
  completed: 'success',
  closed: 'info',
  failed: 'danger'
}[status] || 'info')
const followUpTypeLabel = (type) => ({
  risk: '风险处理',
  settlement: '结算跟进',
  refund: '退款售后',
  payment: '支付异常',
  subscription: '订阅续费',
  operation: '运营沟通'
}[type] || type || '-')
const followUpPriorityLabel = (priority) => ({ low: '较低', normal: '普通', high: '较高', urgent: '紧急' }[priority] || priority || '-')
const followUpPriorityType = (priority) => ({ low: 'info', normal: 'info', high: 'warning', urgent: 'danger' }[priority] || 'info')
const followUpStatusLabel = (status) => ({ open: '待处理', closed: '已处理' }[status] || status || '-')
const formatMoney = (value) => `¥${(Number(value || 0) / 100).toFixed(2)}`
const percent = (value) => `${Math.round(Number(value || 0) * 100)}%`
const formatTime = (value) => value ? String(value).replace('T', ' ').slice(0, 19) : '-'
const formatDate = (value) => value ? String(value).slice(0, 10) : '-'
const dateValue = (value) => value ? new Date(value).getTime() || 0 : 0

onMounted(load)
</script>

<style scoped>
.detail-stack {
  display: grid;
  gap: 18px;
}

.hero-card,
.toolbar,
.hero-actions {
  display: flex;
  justify-content: space-between;
  gap: 14px;
  align-items: flex-start;
}

.hero-card {
  padding: 22px;
  background:
    radial-gradient(circle at top right, rgba(37, 99, 235, 0.16), transparent 32%),
    linear-gradient(135deg, #ffffff, #f8fbff);
}

.back-button {
  padding-left: 0;
}

.eyebrow {
  margin-top: 8px;
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.16em;
}

.page-desc,
.toolbar p,
.risk-desk p {
  margin: 6px 0 0;
  color: #64748b;
  line-height: 1.7;
}

.summary-grid,
.grid-2,
.payment-grid,
.settlement-stats {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 14px;
}

.summary-card,
.inner-card,
.info-list div,
.payment-grid div,
.settlement-stats div,
.risk-card {
  padding: 16px;
  border: 1px solid #e5edf9;
  border-radius: 14px;
  background: #f8fbff;
}

.summary-card.success {
  background: #ecfdf5;
  border-color: #bbf7d0;
}

.summary-card.warning {
  background: #fffbeb;
  border-color: #fde68a;
}

.inner-card {
  display: grid;
  gap: 14px;
}

.summary-card span,
.info-list span,
.payment-grid span,
.settlement-stats span {
  display: block;
  color: #64748b;
  font-size: 13px;
}

.summary-card strong,
.info-list strong,
.payment-grid strong,
.settlement-stats strong {
  display: block;
  margin-top: 8px;
  font-size: 20px;
}

.summary-card small {
  display: block;
  margin-top: 6px;
  color: #64748b;
}

.settlement-stats div:last-child strong {
  color: #16a34a;
}

.info-list {
  display: grid;
  gap: 12px;
}

.action-row {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.payment-actions,
.remark-input,
.risk-desk {
  margin-top: 16px;
}

.payment-review-note {
  margin-top: 4px;
}

.payment-qr-review {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.payment-qr-review article {
  display: grid;
  gap: 10px;
  padding: 14px;
  border: 1px solid #e5edf9;
  border-radius: 14px;
  background: #ffffff;
}

.qr-review-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.qr-review-head strong {
  color: #0f172a;
}

.qr-review-box {
  display: grid;
  place-items: center;
  min-height: 180px;
  padding: 10px;
  border: 1px dashed #bfdbfe;
  border-radius: 12px;
  background: #f8fbff;
}

.qr-review-box img {
  max-width: 180px;
  max-height: 180px;
  object-fit: contain;
}

.payment-qr-review small {
  color: #64748b;
  line-height: 1.5;
}

.settlement-net-alert {
  margin: 12px 0;
}

.risk-desk {
  display: grid;
  gap: 16px;
  padding: 18px;
  border: 1px solid #e5edf9;
  border-radius: 14px;
  background: #ffffff;
}

.risk-header {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: flex-start;
}

.risk-actions {
  display: flex;
  gap: 10px;
  align-items: center;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.risk-header h3,
.mini-title h4 {
  margin: 0;
}

.risk-metrics {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: 10px;
}

.risk-metrics div,
.risk-panel {
  padding: 14px;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  background: #f8fbff;
}

.risk-metrics div.safe {
  border-color: #bbf7d0;
  background: #f8fff9;
}

.risk-metrics div.warning {
  border-color: #fed7aa;
  background: #fffaf3;
}

.risk-metrics div.danger {
  border-color: #fecaca;
  background: #fff7f7;
}

.risk-metrics span {
  display: block;
  color: #64748b;
  font-size: 12px;
  font-weight: 800;
}

.risk-metrics strong {
  display: block;
  margin-top: 8px;
  color: #0f172a;
  font-size: 22px;
}

.risk-metrics small {
  display: block;
  margin-top: 6px;
  color: #64748b;
  line-height: 1.45;
}

.risk-columns {
  display: grid;
  grid-template-columns: minmax(0, 1.2fr) minmax(0, 0.8fr);
  gap: 14px;
}

.mini-title {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: center;
  margin-bottom: 12px;
}

.mini-title span {
  color: #64748b;
  font-size: 12px;
  font-weight: 800;
}

.task-list,
.audit-list {
  display: grid;
  gap: 10px;
}

.task-item,
.audit-item {
  padding: 12px;
  border: 1px solid #e2e8f0;
  border-radius: 10px;
  background: #fff;
}

.task-item strong,
.audit-item strong {
  color: #0f172a;
}

.task-item p,
.audit-item p {
  margin-top: 4px;
  font-size: 13px;
}

.task-item.danger {
  border-color: #fecaca;
}

.task-item.warning {
  border-color: #fed7aa;
}

.task-item.primary {
  border-color: #bfdbfe;
}

.settlement-toolbar {
  margin-bottom: 16px;
}

.compact {
  margin-bottom: 14px;
}

@media (max-width: 768px) {
  .hero-card,
  .toolbar,
  .risk-header,
  .risk-columns {
    display: grid;
  }

  .risk-actions {
    justify-content: flex-start;
  }

  .risk-metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (min-width: 769px) and (max-width: 1280px) {
  .risk-metrics {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}
</style>
