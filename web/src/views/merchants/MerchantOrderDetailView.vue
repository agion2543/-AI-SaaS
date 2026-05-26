<template>
  <div class="order-detail-page">
    <PageHero
      eyebrow="ORDER RECEIPT"
      title="订单详情 / 订单小票 / 售后"
      :description="`订单号：${order.order_no || '-'}`"
      compact
    >
      <template #actions>
        <el-button plain @click="router.push('/merchant/orders')">返回订单列表</el-button>
        <el-button plain @click="copyOrderNo">复制订单号</el-button>
        <el-button v-if="order.customer_phone" plain @click="callCustomer">联系顾客</el-button>
        <el-button type="primary" plain @click="printOrder">打印订单小票</el-button>
        <el-button type="warning" plain :disabled="!canRefund(order)" @click="openRefund">售后退款</el-button>
      </template>
    </PageHero>

    <section class="transaction-brief" :class="briefTone">
      <div class="brief-main">
        <span>ORDER NEXT STEP</span>
        <strong>{{ nextStep.title }}</strong>
        <p>{{ nextStep.text }}</p>
      </div>
      <div class="brief-metrics">
        <div>
          <span>付款状态</span>
          <strong>{{ paymentState.title }}</strong>
          <small>{{ paymentState.hint }}</small>
        </div>
        <div>
          <span>退款核对</span>
          <strong>{{ refundRiskTitle }}</strong>
          <small>{{ refundStatusDescription }}</small>
        </div>
        <div>
          <span>结算提示</span>
          <strong>{{ settlementState.title }}</strong>
          <small>{{ settlementState.hint }}</small>
        </div>
      </div>
      <div class="brief-actions">
        <el-button v-if="order.status === 'payment_confirming'" type="warning" @click="confirmPayment">确认收款</el-button>
        <el-button v-if="['submitted', 'received'].includes(order.status)" type="primary" @click="acceptAndPrint">接单并打印</el-button>
        <el-button v-if="['accepted', 'preparing'].includes(order.status)" type="success" @click="complete">完成订单</el-button>
        <el-button v-if="canRefund(order)" type="warning" plain @click="openRefund">记录退款</el-button>
        <el-button plain @click="printOrder">打印小票</el-button>
      </div>
    </section>

    <section v-if="order.status === 'payment_confirming'" class="payment-check-board">
      <div class="payment-check-main">
        <span>PAYMENT VERIFY</span>
        <strong>先核对到账，再确认收款</strong>
        <p>顾客已点击“我已付款”。请打开商家的支付宝/微信收款账户，核对应收金额、订单核对码和顾客手机号，确认到账后再点击确认收款。</p>
      </div>
      <div class="payment-check-grid">
        <div class="amount">
          <span>应收金额</span>
          <strong>{{ formatMoney(order.total_amount || order.amount) }}</strong>
          <small>请与收款账户到账金额一致</small>
        </div>
        <div>
          <span>核对码</span>
          <strong>{{ pickupCode }}</strong>
          <small>与顾客页面核对码一致</small>
        </div>
        <div>
          <span>顾客手机号</span>
          <strong>{{ order.customer_phone || '未留手机号' }}</strong>
          <small>{{ paymentMarkedAt ? `标记付款：${paymentMarkedAt}` : '暂无标记时间' }}</small>
        </div>
        <div>
          <span>商品数量</span>
          <strong>{{ totalQuantity }} 件</strong>
          <small>{{ order.customer_note ? '顾客有备注，处理前请查看' : '顾客无特殊备注' }}</small>
        </div>
      </div>
      <div class="payment-check-note">
        <span>顾客备注</span>
        <strong>{{ order.customer_note || '无特殊备注' }}</strong>
      </div>
      <div class="payment-check-actions">
        <el-button type="warning" size="large" @click="confirmPayment">已核对到账，确认收款</el-button>
        <el-button size="large" :disabled="!order.customer_phone" @click="callCustomer">联系顾客</el-button>
        <el-button size="large" plain @click="copyText(order.order_no, '订单号已复制')">复制订单号</el-button>
      </div>
    </section>

    <section class="fulfillment-strip">
      <div class="pickup-focus">
        <span>核对码</span>
        <strong>{{ pickupCode }}</strong>
        <small>{{ order.status === 'received' ? `待接单已等待 ${waitText}` : statusLabel(order.status) }}</small>
      </div>
      <div class="fulfillment-card">
        <span>顾客联系</span>
        <strong>{{ order.customer_phone || '未留手机号' }}</strong>
        <div class="inline-actions">
          <el-button v-if="order.customer_phone" size="small" text type="primary" @click="callCustomer">拨打</el-button>
          <el-button v-if="order.customer_phone" size="small" text @click="copyText(order.customer_phone, '手机号已复制')">复制</el-button>
        </div>
      </div>
      <div class="fulfillment-card">
        <span>下单时间</span>
        <strong>{{ formatTime(order.created_at) }}</strong>
        <small v-if="order.paid_at">支付：{{ formatTime(order.paid_at) }}</small>
      </div>
      <div class="fulfillment-card highlight">
        <span>确认后净额</span>
        <strong>{{ formatMoney(confirmedNetAmount) }}</strong>
        <small>{{ netAmountHint }}</small>
      </div>
    </section>

    <section v-if="stockAlertLogs.length" class="stock-alert-board">
      <div>
        <span>STOCK CHECK</span>
        <strong>{{ stockAlertTitle }}</strong>
        <p>{{ stockAlertText }}</p>
      </div>
      <div class="stock-alert-list">
        <article v-for="(log, index) in stockAlertLogs" :key="`stock-${index}`" :class="{ danger: isStockDanger(log) }">
          <strong>{{ isStockDanger(log) ? '库存异常' : '库存已扣减' }}</strong>
          <p>{{ log.text }}</p>
          <small>{{ formatTime(log.time) }}</small>
        </article>
      </div>
    </section>

    <section class="receipt-grid">
      <DataPanel eyebrow="SUMMARY" title="订单小票" description="用于商家核对顾客、门店、应收、收款状态、优惠和退款。">
        <div class="status-row">
          <el-tag size="large" :type="statusType(order.status)">{{ statusLabel(order.status) }}</el-tag>
          <el-tag size="large" :type="refundType(order.refund_status)">{{ refundLabel(order.refund_status) }}</el-tag>
          <el-tag v-if="order.settlement_id" size="large" type="success">已进结算单</el-tag>
        </div>

        <div class="kitchen-check">
          <div>
            <span>订单核对码</span>
            <strong>{{ pickupCode }}</strong>
          </div>
          <div>
            <span>商品总数</span>
            <strong>{{ totalQuantity }} 件</strong>
          </div>
          <div>
            <span>顾客备注</span>
            <strong>{{ order.customer_note || '无特殊备注' }}</strong>
          </div>
        </div>

        <div class="summary-grid printable">
          <div class="summary-item">
            <span>门店</span>
            <strong>{{ order.store?.name || '-' }}</strong>
          </div>
          <div class="summary-item">
            <span>顾客手机号</span>
            <strong>{{ order.customer_phone || '-' }}</strong>
          </div>
          <div class="summary-item">
            <span>订单原价</span>
            <strong>{{ formatMoney(order.amount) }}</strong>
          </div>
          <div class="summary-item">
            <span>优惠金额</span>
            <strong>-{{ formatMoney(order.discount_amount) }}</strong>
          </div>
          <div class="summary-item highlight">
            <span>应收金额</span>
            <strong>{{ formatMoney(receivableAmount) }}</strong>
          </div>
          <div class="summary-item">
            <span>收款状态</span>
            <strong>{{ paymentState.title }}</strong>
          </div>
          <div class="summary-item">
            <span>已退款</span>
            <strong>{{ formatMoney(order.refunded_amount) }}</strong>
          </div>
          <div class="summary-item">
            <span>可退金额</span>
            <strong>{{ formatMoney(refundableAmount) }}</strong>
          </div>
          <div class="summary-item">
            <span>支付时间</span>
            <strong>{{ formatTime(order.paid_at) }}</strong>
          </div>
        </div>
      </DataPanel>

      <DataPanel eyebrow="ACTIONS" title="订单处理" description="商家可以在这里完成接单、履约完成、关闭订单和内部备注。">
        <div class="handler-card">
          <span>{{ nextStep.label }}</span>
          <strong>{{ nextStep.title }}</strong>
          <small>{{ nextStep.text }}</small>
        </div>
        <div class="fulfillment-checklist">
          <div v-for="item in fulfillmentChecklist" :key="item.label" :class="{ done: item.done }">
            <span>{{ item.done ? '✓' : '·' }}</span>
            <p>{{ item.label }}</p>
          </div>
        </div>
        <div class="action-stack">
          <div class="primary-action-group">
            <span>推荐主动作</span>
            <el-button v-if="order.status === 'payment_confirming'" type="warning" size="large" @click="confirmPayment">已核对到账，确认收款</el-button>
            <el-button v-else-if="['submitted', 'received'].includes(order.status)" type="primary" size="large" @click="acceptAndPrint">接单并打印小票</el-button>
            <el-button v-else-if="['accepted', 'preparing'].includes(order.status)" type="success" size="large" @click="complete">履约完成，标记完成</el-button>
            <strong v-else>{{ primaryActionFallback }}</strong>
          </div>
          <div class="secondary-action-group">
            <span>辅助处理</span>
            <div>
              <el-button v-if="['submitted', 'received'].includes(order.status)" plain @click="accept">只接单不打印</el-button>
              <el-button v-if="canClose(order)" type="danger" plain @click="close">关闭订单</el-button>
              <el-button v-if="canRefund(order)" type="warning" plain @click="openRefund">记录售后退款</el-button>
              <el-button plain @click="openNote">编辑内部备注</el-button>
            </div>
          </div>
        </div>
        <div class="process-tip">
          <strong>{{ paymentState.title }}</strong>
          <p>{{ paymentState.hint }}</p>
        </div>
      </DataPanel>
    </section>

    <section class="content-grid">
      <DataPanel eyebrow="TIMELINE" title="订单进度" description="按交易流程展示订单当前所在阶段。">
        <el-timeline class="order-timeline">
          <el-timeline-item v-for="item in timeline" :key="item.status" :type="timelineType(item)" :timestamp="item.time">
            <strong>{{ item.title }}</strong>
            <p>{{ item.description }}</p>
          </el-timeline-item>
        </el-timeline>
      </DataPanel>

      <DataPanel eyebrow="NOTES" title="备注信息" description="区分顾客备注和商家内部备注，避免履约信息遗漏。">
        <div class="note-grid">
          <div>
            <span>顾客备注</span>
            <p>{{ order.customer_note || '顾客暂无备注。' }}</p>
          </div>
          <div>
            <span>商家内部备注</span>
            <p>{{ order.merchant_note || '暂无内部备注。可记录顾客偏好、异常情况或履约提醒。' }}</p>
          </div>
        </div>
      </DataPanel>
    </section>

    <section v-if="appendLogs.length" class="append-board">
      <div class="append-head">
        <div>
          <span>APPEND ITEMS</span>
          <strong>顾客追加了 {{ appendLogs.length }} 轮商品</strong>
          <p>商品明细已合并到当前订单金额；下方记录用于商家核对哪些内容是后续追加的。</p>
        </div>
        <el-button plain type="primary" @click="printOrder">打印含追加记录小票</el-button>
      </div>
      <div class="append-list">
        <article v-for="(log, index) in appendLogs" :key="`append-${index}`">
          <b>第 {{ index + 1 }} 轮追加</b>
          <p>{{ log.text || '顾客追加商品' }}</p>
          <small>{{ formatTime(log.time) }}</small>
        </article>
      </div>
    </section>

    <DataPanel eyebrow="ITEMS" title="商品明细" description="商品以创建订单时的快照为准，后续商品改价不会影响历史订单。">
      <div class="items-summary">
        <span>共 {{ (order.items || []).length }} 个商品条目 / {{ totalQuantity }} 件</span>
        <strong>应收 {{ formatMoney(receivableAmount) }}</strong>
      </div>
      <el-table :data="order.items || []" class="items-table" empty-text="暂无商品明细">
        <el-table-column label="商品" min-width="220">
          <template #default="{ row }">
            <div class="product-cell">
              <img v-if="row.image_url" :src="row.image_url" alt="" />
              <div>
                <strong>{{ row.name }}</strong>
                <p>{{ row.description || '暂无描述' }}</p>
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="单价" width="120">
          <template #default="{ row }">{{ formatMoney(row.price) }}</template>
        </el-table-column>
        <el-table-column prop="quantity" label="数量" width="100" />
        <el-table-column label="小计" width="120">
          <template #default="{ row }">{{ formatMoney(itemLineAmount(row)) }}</template>
        </el-table-column>
      </el-table>
    </DataPanel>

    <DataPanel eyebrow="AFTERSALE" title="售后退款" description="统一查看退款状态、退款金额、确认后净额和人工处理提示。">
      <div class="refund-dashboard">
        <div class="refund-status-card" :class="order.refund_status || 'none'">
          <span>退款状态</span>
          <strong>{{ refundLabel(order.refund_status) }}</strong>
          <p>{{ refundStatusDescription }}</p>
        </div>
        <div class="refund-metric">
          <span>已退金额</span>
          <strong>{{ formatMoney(order.refunded_amount) }}</strong>
        </div>
        <div class="refund-metric">
          <span>可退金额</span>
          <strong>{{ formatMoney(refundableAmount) }}</strong>
        </div>
        <div class="refund-metric highlight">
          <span>确认后净额</span>
          <strong>{{ formatMoney(confirmedNetAmount) }}</strong>
          <small>{{ netAmountHint }}</small>
        </div>
      </div>

      <div class="refund-advice">
        <strong>{{ refundAdvice.title }}</strong>
        <p>{{ refundAdvice.text }}</p>
        <el-button type="warning" plain :disabled="!canRefund(order)" @click="openRefund">{{ refundAdvice.button }}</el-button>
      </div>

      <div v-if="refundLogs.length || Number(order.refunded_amount || 0) > 0" class="refund-records">
        <div class="refund-record-head">
          <span>退款记录</span>
          <strong>{{ refundLogs.length || 1 }} 条</strong>
        </div>
        <article v-for="(log, index) in refundLogs" :key="index">
          <div>
            <strong>{{ log.text || '退款记录' }}</strong>
            <small>{{ actionLabel(log.action) }}</small>
          </div>
          <span>{{ formatTime(log.time) }}</span>
        </article>
        <p v-if="!refundLogs.length">系统已有退款金额记录，但暂无更详细的操作日志。建议补充退款原因，方便后续财务核对。</p>
      </div>
      <el-empty v-else description="暂无退款记录" />
    </DataPanel>

    <DataPanel eyebrow="LOGS" title="内部处理记录" description="接单、完成、关闭、退款等关键动作都会沉淀为后续复盘依据。">
      <el-timeline class="order-timeline">
        <el-timeline-item v-for="(log, index) in order.operation_logs || []" :key="index" type="primary" :timestamp="formatTime(log.time)">
          <strong>{{ log.text || actionLabel(log.action) }}</strong>
          <p>{{ actionLabel(log.action) }}</p>
        </el-timeline-item>
      </el-timeline>
      <el-empty v-if="!(order.operation_logs || []).length" description="暂无处理记录" />
    </DataPanel>

    <el-dialog v-model="noteDialogVisible" title="订单内部备注" width="520px">
      <el-input
        v-model="noteForm.merchant_note"
        type="textarea"
        :rows="4"
        maxlength="500"
        show-word-limit
        placeholder="可记录顾客偏好、异常情况、履约提醒等，仅商家端可见。"
      />
      <template #footer>
        <el-button @click="noteDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveNote">保存备注</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="refundDialogVisible" title="售后退款记录" width="560px">
      <el-alert
        class="refund-alert"
        type="warning"
        show-icon
        :closable="false"
        title="当前为财务退款记录。真实退款仍需在支付宝/微信等支付渠道执行，后续可继续接入正式退款接口。"
      />
      <el-form label-width="100px">
        <el-form-item label="可退金额">
          <el-input :model-value="formatMoney(refundableAmount)" disabled />
        </el-form-item>
        <el-form-item label="退款方式">
          <el-radio-group v-model="refundForm.full">
            <el-radio-button :label="true">全额退款</el-radio-button>
            <el-radio-button :label="false">自定义金额</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="!refundForm.full" label="退款金额">
          <el-input-number v-model="refundForm.amount_yuan" :min="0.01" :max="refundableAmount / 100" :precision="2" />
        </el-form-item>
        <el-form-item label="退款原因">
          <el-input v-model="refundForm.reason" type="textarea" :rows="3" maxlength="120" show-word-limit />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="refundDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="refundSaving" @click="submitRefund">确认记录退款</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  acceptMerchantOrder,
  closeMerchantOrder,
  confirmMerchantOrderPayment,
  completeMerchantOrder,
  fetchMerchantOrderDetail,
  refundMerchantOrder,
  updateMerchantOrderNote
} from '../../api/modules'
import DataPanel from '../../components/design/DataPanel.vue'
import PageHero from '../../components/design/PageHero.vue'
import { printStoreOrderReceipt } from '../../utils/orderReceipt'

const route = useRoute()
const router = useRouter()
const order = ref({})
const noteDialogVisible = ref(false)
const refundDialogVisible = ref(false)
const refundSaving = ref(false)
const noteForm = reactive({ merchant_note: '' })
const refundForm = reactive({ full: true, amount_yuan: 0, reason: '' })

const paidStatuses = ['received', 'accepted', 'completed', 'closed']
const isConfirmedPaid = computed(() => paidStatuses.includes(order.value.status) || Boolean(order.value.paid_at))
const receivableAmount = computed(() => Number(order.value.total_amount || order.value.amount || 0))
const paidAmount = computed(() => Number(order.value.paid_amount ?? order.value.total_amount ?? order.value.amount ?? 0))
const refundableAmount = computed(() => {
  if (!isConfirmedPaid.value) return 0
  return Math.max(paidAmount.value - Number(order.value.refunded_amount || 0), 0)
})
const confirmedNetAmount = computed(() => {
  if (!isConfirmedPaid.value) return 0
  return Math.max(paidAmount.value - Number(order.value.refunded_amount || 0), 0)
})
const netAmountHint = computed(() => {
  if (!isConfirmedPaid.value) return '未确认收款，不计入实收'
  if (Number(order.value.refunded_amount || 0) > 0) return '已扣除退款'
  return '已确认收款'
})
const refundLogs = computed(() => (Array.isArray(order.value.operation_logs) ? order.value.operation_logs : []).filter((item) => item.action === 'refund'))
const stockAlertLogs = computed(() => (Array.isArray(order.value.operation_logs) ? order.value.operation_logs : []).filter((item) => item.action === 'stock_deducted'))
const appendLogs = computed(() => (Array.isArray(order.value.operation_logs) ? order.value.operation_logs : []).filter((item) => item.action === 'items_appended'))
const hasStockDanger = computed(() => stockAlertLogs.value.some(isStockDanger))
const totalQuantity = computed(() => (Array.isArray(order.value.items) ? order.value.items : []).reduce((sum, item) => sum + Number(item.quantity || 0), 0))
const paymentMarkedAt = computed(() => logTime('customer_paid') || logTime('submitted') || '')
const pickupCode = computed(() => {
  const text = String(order.value.order_no || order.value.id || '')
  return text ? text.slice(-4).toUpperCase() : '-'
})
const waitMinutes = computed(() => {
  const baseTime = order.value.paid_at || order.value.created_at
  const timestamp = baseTime ? new Date(baseTime).getTime() : 0
  if (!timestamp) return 0
  return Math.max(Math.floor((Date.now() - timestamp) / 60000), 0)
})
const waitText = computed(() => {
  if (waitMinutes.value < 60) return `${waitMinutes.value} 分钟`
  const hours = Math.floor(waitMinutes.value / 60)
  const minutes = waitMinutes.value % 60
  return minutes ? `${hours} 小时 ${minutes} 分钟` : `${hours} 小时`
})
const stockAlertTitle = computed(() => {
  if (hasStockDanger.value) return '库存异常，接单前先核对商品'
  if (stockAlertLogs.value.length) return '库存已自动扣减'
  return ''
})
const stockAlertText = computed(() => {
  if (hasStockDanger.value) return '系统已确认付款，但部分商品库存不足或并发变动。建议先联系顾客确认替换、退款或等待补货，再继续处理。'
  return '支付成功后已按订单商品扣减库存，不限库存商品不会扣减。'
})
const paymentState = computed(() => {
  if (order.value.status === 'pending') {
    return { title: '等待顾客支付', hint: '未付款订单不计入实收，也不建议提前处理。' }
  }
  if (order.value.status === 'failed') {
    return { title: '支付未完成', hint: '顾客可在订单状态页重新支付，商家先不要接单处理。' }
  }
  if (order.value.status === 'payment_confirming') {
    return { title: '待确认收款', hint: '顾客已标记付款，请核对商家收款账户到账后再确认。' }
  }
  if (['submitted', 'preparing'].includes(order.value.status)) {
    return { title: '稍后结算', hint: '该订单已提交给商家，当前不按已付款处理。' }
  }
  if (paidStatuses.includes(order.value.status)) {
    return { title: '付款已确认', hint: '付款成功后订单直接进入商家处理，不需要商家审核付款。' }
  }
  return { title: '待核对', hint: '请核对订单状态和支付记录。' }
})
const refundRiskTitle = computed(() => {
  if (order.value.refund_status === 'full') return '已全额退款'
  if (order.value.refund_status === 'partial') return '部分退款'
  if (Number(order.value.refunded_amount || 0) > 0) return '有退款记录'
  return '暂无退款'
})
const settlementState = computed(() => {
  if (order.value.settlement_id) return { title: '已进结算单', hint: '该订单已被纳入结算记录，退款后需同步核对结算。' }
  if (paidStatuses.includes(order.value.status)) return { title: '待结算核对', hint: '可在财务对账中按周期汇总，退款后以净实收为准。' }
  return { title: '未进入结算', hint: '未付款或异常订单不会进入实收结算。' }
})
const briefTone = computed(() => {
  if (order.value.status === 'received') return 'primary'
  if (order.value.status === 'submitted') return 'primary'
  if (order.value.status === 'payment_confirming') return 'warning'
  if (order.value.status === 'preparing') return 'success'
  if (order.value.status === 'accepted') return 'success'
  if (['pending', 'failed'].includes(order.value.status)) return 'warning'
  if (Number(order.value.refunded_amount || 0) > 0) return 'refund'
  return 'neutral'
})
const nextStep = computed(() => {
  if (order.value.status === 'pending') {
    return {
      label: '支付待完成',
      title: '等待顾客完成支付',
      text: '这笔订单还没有形成实收，顾客可在订单状态页继续支付；商家只需核对，不建议提前处理。'
    }
  }
  if (order.value.status === 'failed') {
    return {
      label: '支付异常',
      title: '支付未完成，先不要履约',
      text: '可引导顾客回到订单状态页重新支付。支付成功后系统会自动进入商家处理。'
    }
  }
  if (order.value.status === 'submitted') {
    return {
      label: '建议动作',
      title: '确认订单并处理',
      text: '订单已提交，建议确认后打印小票或进入处理流程。'
    }
  }
  if (order.value.status === 'payment_confirming') {
    return {
      label: '建议动作',
      title: '核对到账并确认收款',
      text: '顾客已点击“我已付款”。请先核对支付宝/微信实际到账，再确认收款；确认后订单才会计入实收。'
    }
  }
  if (order.value.status === 'received') {
    return {
      label: '建议动作',
      title: '接单并打印小票',
      text: '顾客已付款，建议接单后立即打印订单小票，减少漏单和备注遗漏。'
    }
  }
  if (order.value.status === 'accepted' || order.value.status === 'preparing') {
    return {
      label: '建议动作',
      title: '完成履约后标记完成',
      text: '订单正在处理或履约，完成后及时标记，方便财务对账和后续复购分析。'
    }
  }
  if (order.value.status === 'completed') {
    return {
      label: '复盘动作',
      title: '订单已完成，关注评价和复购',
      text: '可以引导顾客评价、分享领券，并把热销商品用于下一轮活动。'
    }
  }
  if (order.value.status === 'closed') {
    return {
      label: '核对动作',
      title: '订单已关闭，核对退款和备注',
      text: '如发生退款或取消，请确认退款原因、支付渠道和内部备注都已记录。'
    }
  }
  return {
    label: '核对动作',
    title: '请先核对订单状态',
    text: '当前状态不明确，建议返回订单列表刷新后再处理。'
  }
})
const refundStatusDescription = computed(() => {
  if (!isConfirmedPaid.value) return '订单尚未确认收款，暂不产生可退金额。'
  if (order.value.refund_status === 'full') return '该订单已记录全额退款，确认后净额为 0。'
  if (order.value.refund_status === 'partial') return '该订单已记录部分退款，仍有剩余确认后净额。'
  if (Number(order.value.refunded_amount || 0) > 0) return '已有退款金额记录，请核对退款状态是否同步。'
  return '该订单暂未记录退款。'
})
const refundAdvice = computed(() => {
  if (order.value.refund_status === 'full') {
    return {
      title: '售后已完成',
      text: '当前订单已全额退款，建议核对支付渠道到账状态，并在内部备注中记录顾客沟通结果。',
      button: '已全额退款'
    }
  }
  if (order.value.refund_status === 'partial') {
    return {
      title: '仍可继续退款',
      text: `订单已部分退款，剩余可退 ${formatMoney(refundableAmount.value)}。如后续继续售后，可继续追加退款记录。`,
      button: '追加退款'
    }
  }
  if (canRefund(order.value)) {
    return {
      title: '可发起售后退款',
      text: '当前订单已确认收款且仍有可退金额。如顾客投诉、缺货或取消履约，可先记录退款原因，后续再接真实支付渠道退款。',
      button: '记录退款'
    }
  }
  return {
    title: '暂不可退款',
    text: '订单未进入可退款状态，或已无可退金额。请先确认支付和订单状态。',
    button: '不可退款'
  }
})
const fulfillmentChecklist = computed(() => [
  {
    label: hasStockDanger.value ? '存在库存异常：接单前先联系顾客或调整商品' : '库存已核对，未发现阻塞接单的库存异常',
    done: !hasStockDanger.value
  },
  {
    label: paymentChecklistText.value,
    done: paymentChecklistDone.value
  },
  {
    label: order.value.customer_note ? `已查看顾客备注：${order.value.customer_note}` : '顾客无特殊备注',
    done: true
  },
  {
    label: !['submitted', 'payment_confirming', 'received'].includes(order.value.status) ? '已处理接单动作' : '待处理：建议确认后打印小票',
    done: !['submitted', 'payment_confirming', 'received'].includes(order.value.status)
  },
  {
    label: ['completed', 'closed'].includes(order.value.status) ? '订单已结束，后续只需核对售后/退款' : '履约完成后及时标记完成',
    done: ['completed', 'closed'].includes(order.value.status)
  }
])

const paymentChecklistText = computed(() => {
  if (order.value.status === 'payment_confirming') return '顾客已标记付款：等待商家确认到账'
  if (paidStatuses.includes(order.value.status)) return '付款已确认，无需商家二次审核付款'
  if (['submitted', 'preparing'].includes(order.value.status)) return '先提交后结算订单：可先处理，后续再完成结算'
  return '等待顾客完成支付后再履约'
})
const paymentChecklistDone = computed(() => paidStatuses.includes(order.value.status) || ['submitted', 'preparing'].includes(order.value.status))
const primaryActionFallback = computed(() => {
  if (['pending', 'failed'].includes(order.value.status)) return '等待顾客完成支付，商家暂不处理履约'
  if (order.value.status === 'completed') return '订单已完成，可继续核对售后或打印小票'
  if (order.value.status === 'closed') return '订单已关闭，只需核对退款和备注'
  return '暂无推荐主动作'
})

const timeline = computed(() => [
  { status: 'pending', title: '订单创建', description: '顾客已提交订单，等待支付。', time: formatTime(order.value.created_at) },
  { status: 'received', title: ['submitted', 'preparing', 'payment_confirming'].includes(order.value.status) ? '订单提交' : '支付完成', description: order.value.status === 'payment_confirming' ? '顾客已标记付款，等待商家确认收款。' : ['submitted', 'preparing'].includes(order.value.status) ? '订单已提交给商家，后续再结算。' : '支付成功后直接进入商家处理，无需商家审核付款。', time: formatTime(order.value.paid_at || logTime('submitted') || logTime('customer_paid') || logTime('paid') || order.value.updated_at) },
  { status: 'accepted', title: order.value.status === 'payment_confirming' ? '确认收款' : order.value.status === 'preparing' ? '商家处理中' : '商家接单', description: order.value.status === 'payment_confirming' ? '请核对到账后点击确认收款。' : order.value.status === 'preparing' ? '门店已开启自动接单，订单进入处理流程。' : '商家接单后开始处理或履约。', time: logTime('accepted') || logTime('auto_accepted') || logTime('merchant_payment_confirmed') || (['accepted', 'preparing', 'completed'].includes(order.value.status) ? formatTime(order.value.updated_at) : '-') },
  { status: 'completed', title: order.value.status === 'closed' ? '订单关闭' : '订单完成', description: order.value.status === 'closed' ? '订单已关闭或取消。' : '服务或商品已完成交付。', time: logTime(order.value.status === 'closed' ? 'closed' : 'completed') || (order.value.status === 'completed' ? formatTime(order.value.updated_at) : '-') }
])

const load = async () => {
  const res = await fetchMerchantOrderDetail(route.params.id)
  order.value = res.data.order || {}
}

const confirmStockBeforeAccept = async () => {
  if (!hasStockDanger.value || order.value.status !== 'received') return true
  await ElMessageBox.confirm(
    '该订单存在库存异常。请确认已联系顾客或已有替换/退款处理方案，再继续接单。',
    '库存异常提醒',
    { type: 'warning', confirmButtonText: '已确认，继续接单', cancelButtonText: '先不接单' }
  )
  return true
}

const accept = async () => {
  await confirmStockBeforeAccept()
  await acceptMerchantOrder(order.value.id)
  ElMessage.success('订单已接单')
  load()
}

const confirmPayment = async () => {
  await ElMessageBox.confirm(confirmPaymentHtml(), '确认收款核对', {
    type: 'warning',
    dangerouslyUseHTMLString: true,
    confirmButtonText: '已核对到账，确认收款',
    cancelButtonText: '先不确认'
  })
  await confirmMerchantOrderPayment(order.value.id)
  ElMessage.success('已确认收款')
  load()
}

const acceptAndPrint = async () => {
  if (printStoreOrderReceipt(order.value)) {
    await accept()
  } else {
    ElMessage.warning('浏览器阻止了打印窗口，请允许弹窗后重试')
  }
}

const complete = async () => {
  await completeMerchantOrder(order.value.id)
  ElMessage.success('订单已完成')
  load()
}

const close = async () => {
  await ElMessageBox.confirm('确认关闭这笔订单吗？', '关闭订单', { type: 'warning' })
  await closeMerchantOrder(order.value.id)
  ElMessage.success('订单已关闭')
  load()
}

const openNote = () => {
  noteForm.merchant_note = order.value.merchant_note || ''
  noteDialogVisible.value = true
}

const saveNote = async () => {
  await updateMerchantOrderNote(order.value.id, { merchant_note: noteForm.merchant_note })
  ElMessage.success('备注已保存')
  noteDialogVisible.value = false
  load()
}

const openRefund = () => {
  refundForm.full = true
  refundForm.amount_yuan = refundableAmount.value / 100
  refundForm.reason = ''
  refundDialogVisible.value = true
}

const submitRefund = async () => {
  const amount = refundForm.full ? 0 : Math.round(Number(refundForm.amount_yuan || 0) * 100)
  await ElMessageBox.confirm('确认记录这笔退款吗？请确保真实支付渠道已同步处理。', '确认退款', { type: 'warning' })
  refundSaving.value = true
  try {
    await refundMerchantOrder(order.value.id, {
      full: refundForm.full,
      amount,
      reason: refundForm.reason
    })
    ElMessage.success('退款记录已创建')
    refundDialogVisible.value = false
    await load()
  } finally {
    refundSaving.value = false
  }
}

const printOrder = () => {
  if (!printStoreOrderReceipt(order.value)) {
    ElMessage.warning('浏览器阻止了打印窗口，请允许弹窗后重试')
  }
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
const copyOrderNo = () => copyText(order.value.order_no, '订单号已复制')
const callCustomer = () => {
  if (!order.value.customer_phone) return
  window.location.href = `tel:${order.value.customer_phone}`
}
const canClose = (row) => ['submitted', 'payment_confirming', 'received', 'preparing', 'accepted'].includes(row.status)
const canRefund = (row) => row?.order_type === 'store_order' && refundableAmount.value > 0 && ['received', 'accepted', 'completed', 'closed'].includes(row.status)
const itemLineAmount = (row) => Number(row.line_amount ?? row.amount ?? (Number(row.price || 0) * Number(row.quantity || 0)))
const formatMoney = (value) => `¥${(Number(value || 0) / 100).toFixed(2)}`
const formatTime = (value) => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '-'
const escapeHtml = (value) => String(value ?? '').replace(/[&<>"']/g, (char) => ({
  '&': '&amp;',
  '<': '&lt;',
  '>': '&gt;',
  '"': '&quot;',
  "'": '&#39;'
}[char]))
const confirmPaymentHtml = () => `
  <div class="confirm-payment-card">
    <p>请先打开商家的支付宝/微信收款账户，确认该笔款项已经到账，再点击确认。</p>
    <div><span>应收金额</span><strong>${escapeHtml(formatMoney(order.value.total_amount || order.value.amount))}</strong></div>
    <div><span>订单号</span><strong>${escapeHtml(order.value.order_no || '-')}</strong></div>
    <div><span>核对码</span><strong>${escapeHtml(pickupCode.value)}</strong></div>
    <div><span>顾客手机号</span><strong>${escapeHtml(order.value.customer_phone || '未留手机号')}</strong></div>
    <div><span>商品数量</span><strong>${escapeHtml(totalQuantity.value)} 件</strong></div>
    <div><span>标记付款</span><strong>${escapeHtml(paymentMarkedAt.value || '-')}</strong></div>
    <div><span>顾客备注</span><strong>${escapeHtml(order.value.customer_note || '无')}</strong></div>
  </div>
`
const statusLabel = (status) => ({
  pending: '待支付',
  submitted: '已提交',
  payment_confirming: '待确认收款',
  preparing: '处理中',
  received: '待接单',
  accepted: '已接单',
  completed: '已完成',
  closed: '已关闭',
  failed: '支付失败'
}[status] || status || '-')
const statusType = (status) => ({
  pending: 'warning',
  submitted: 'primary',
  payment_confirming: 'warning',
  preparing: 'success',
  received: 'primary',
  accepted: 'success',
  completed: 'success',
  closed: 'info',
  failed: 'danger'
}[status] || 'info')
const refundLabel = (status) => ({
  none: '未退款',
  partial: '部分退款',
  full: '全额退款'
}[status] || '未退款')
const refundType = (status) => ({
  partial: 'warning',
  full: 'success'
}[status] || 'info')
const actionLabel = (action) => ({
  paid: '支付成功',
  stock_deducted: '库存扣减',
  accepted: '商家已接单',
  submitted: '订单已提交',
  customer_paid: '顾客标记已付款',
  items_appended: '顾客追加商品',
  merchant_payment_confirmed: '商家确认收款',
  auto_accepted: '自动接单',
  completed: '商家已完成订单',
  closed: '订单已关闭',
  refund: '已记录退款',
  note: '更新内部备注'
}[action] || action || '处理记录')
function isStockDanger(log) {
  return String(log?.text || '').includes('库存异常')
}
const logTime = (action) => {
  const logs = Array.isArray(order.value.operation_logs) ? order.value.operation_logs : []
  const log = logs.find((item) => item.action === action)
  return log?.time ? formatTime(log.time) : ''
}

const timelineType = (item) => {
  const normalizedStatus = order.value.status === 'closed' ? 'completed' : order.value.status === 'payment_confirming' ? 'accepted' : order.value.status
  const orderIndex = ['pending', 'received', 'accepted', 'completed'].indexOf(normalizedStatus)
  const itemIndex = ['pending', 'received', 'accepted', 'completed'].indexOf(item.status)
  return itemIndex >= 0 && itemIndex <= orderIndex ? 'success' : 'info'
}

onMounted(load)
</script>

<style scoped>
.order-detail-page {
  display: grid;
  gap: 16px;
}

.transaction-brief {
  display: grid;
  grid-template-columns: minmax(260px, 1fr) minmax(0, 1.6fr) auto;
  gap: 14px;
  align-items: stretch;
  padding: 18px 20px;
  border: 1px solid #dbeafe;
  border-radius: 22px;
  background:
    radial-gradient(circle at 100% 0%, rgba(37, 99, 235, 0.1), transparent 32%),
    #ffffff;
  box-shadow: 0 14px 36px rgba(15, 39, 71, 0.07);
}

.transaction-brief.primary {
  border-color: #93c5fd;
  background:
    radial-gradient(circle at 100% 0%, rgba(37, 99, 235, 0.16), transparent 32%),
    #eff6ff;
}

.transaction-brief.success {
  border-color: #bbf7d0;
  background:
    radial-gradient(circle at 100% 0%, rgba(22, 163, 74, 0.12), transparent 32%),
    #ecfdf5;
}

.transaction-brief.warning,
.transaction-brief.refund {
  border-color: #fed7aa;
  background:
    radial-gradient(circle at 100% 0%, rgba(249, 115, 22, 0.14), transparent 32%),
    #fff7ed;
}

.brief-main,
.brief-metrics div {
  min-height: 104px;
  padding: 14px;
  border: 1px solid rgba(203, 213, 225, 0.82);
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.78);
}

.brief-main span,
.brief-main strong,
.brief-main p,
.brief-metrics span,
.brief-metrics strong,
.brief-metrics small {
  display: block;
}

.brief-main span,
.brief-metrics span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.08em;
}

.brief-main strong {
  margin-top: 6px;
  color: #0f2747;
  font-size: 22px;
  line-height: 1.25;
}

.brief-main p {
  margin: 8px 0 0;
  color: #64748b;
  line-height: 1.6;
}

.brief-metrics {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
}

.brief-metrics strong {
  margin-top: 7px;
  color: #0f2747;
  font-size: 18px;
  line-height: 1.25;
}

.brief-metrics small {
  margin-top: 6px;
  color: #64748b;
  line-height: 1.45;
}

.brief-actions {
  display: grid;
  align-content: center;
  gap: 10px;
  min-width: 136px;
}

.fulfillment-strip {
  display: grid;
  grid-template-columns: 1.1fr repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.pickup-focus,
.fulfillment-card {
  min-height: 118px;
  padding: 16px;
  border: 1px solid #dbeafe;
  border-radius: 18px;
  background: #ffffff;
  box-shadow: 0 10px 26px rgba(15, 39, 71, 0.06);
}

.pickup-focus {
  border-color: #93c5fd;
  background:
    radial-gradient(circle at 100% 0%, rgba(37, 99, 235, 0.14), transparent 36%),
    #eff6ff;
}

.fulfillment-card.highlight {
  border-color: #bbf7d0;
  background: #ecfdf5;
}

.pickup-focus span,
.pickup-focus strong,
.pickup-focus small,
.fulfillment-card span,
.fulfillment-card strong,
.fulfillment-card small {
  display: block;
}

.pickup-focus span,
.fulfillment-card span {
  color: #64748b;
  font-size: 12px;
  font-weight: 800;
}

.pickup-focus strong {
  margin-top: 6px;
  color: #0f2747;
  font-size: 42px;
  line-height: 1;
  letter-spacing: 0;
}

.fulfillment-card strong {
  margin-top: 8px;
  color: #0f2747;
  font-size: 18px;
  line-height: 1.3;
  word-break: break-all;
}

.pickup-focus small,
.fulfillment-card small {
  margin-top: 8px;
  color: #64748b;
}

.inline-actions {
  display: flex;
  gap: 8px;
  margin-top: 6px;
}

.payment-check-board {
  display: grid;
  grid-template-columns: minmax(260px, 0.95fr) minmax(0, 1.45fr) minmax(220px, 0.7fr);
  gap: 14px;
  align-items: stretch;
  padding: 18px 20px;
  border: 1px solid #fbbf24;
  border-radius: 22px;
  background:
    radial-gradient(circle at 100% 0%, rgba(245, 158, 11, 0.14), transparent 34%),
    linear-gradient(135deg, #fffbeb, #ffffff);
  box-shadow: 0 16px 38px rgba(146, 64, 14, 0.1);
}

.payment-check-main span,
.payment-check-main strong,
.payment-check-main p,
.payment-check-grid span,
.payment-check-grid strong,
.payment-check-grid small,
.payment-check-note span,
.payment-check-note strong {
  display: block;
}

.payment-check-main span,
.payment-check-grid span,
.payment-check-note span {
  color: #d97706;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.08em;
}

.payment-check-main strong {
  margin-top: 6px;
  color: #78350f;
  font-size: 22px;
  line-height: 1.25;
}

.payment-check-main p {
  margin: 8px 0 0;
  color: #92400e;
  line-height: 1.65;
}

.payment-check-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.payment-check-grid div,
.payment-check-note {
  padding: 13px;
  border: 1px solid #fde68a;
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.82);
}

.payment-check-grid div.amount {
  border-color: #f59e0b;
  background: #fff7ed;
}

.payment-check-grid strong,
.payment-check-note strong {
  margin-top: 6px;
  color: #0f2747;
  font-size: 18px;
  line-height: 1.3;
  word-break: break-all;
}

.payment-check-grid div.amount strong {
  color: #b45309;
  font-size: 24px;
}

.payment-check-grid small {
  margin-top: 6px;
  color: #92400e;
  line-height: 1.45;
}

.payment-check-note {
  min-height: 100%;
}

.payment-check-actions {
  display: grid;
  align-content: center;
  gap: 10px;
}

.stock-alert-board {
  display: grid;
  grid-template-columns: minmax(260px, 0.9fr) minmax(0, 1.6fr);
  gap: 14px;
  padding: 16px 18px;
  border: 1px solid #fed7aa;
  border-radius: 20px;
  background: linear-gradient(135deg, #fff7ed, #ffffff);
  box-shadow: 0 14px 34px rgba(154, 52, 18, 0.08);
}

.stock-alert-board > div:first-child span,
.stock-alert-board > div:first-child strong,
.stock-alert-board > div:first-child p {
  display: block;
}

.stock-alert-board > div:first-child span {
  color: #ea580c;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.1em;
}

.stock-alert-board > div:first-child strong {
  margin-top: 6px;
  color: #7c2d12;
  font-size: 22px;
}

.stock-alert-board > div:first-child p {
  margin: 8px 0 0;
  color: #9a3412;
  line-height: 1.6;
}

.stock-alert-list {
  display: grid;
  gap: 10px;
}

.stock-alert-list article {
  padding: 12px;
  border: 1px solid #fed7aa;
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.84);
}

.stock-alert-list article.danger {
  border-color: #fecaca;
  background: #fff1f2;
}

.stock-alert-list strong,
.stock-alert-list p,
.stock-alert-list small {
  display: block;
}

.stock-alert-list strong {
  color: #9a3412;
}

.stock-alert-list article.danger strong {
  color: #b91c1c;
}

.stock-alert-list p {
  margin: 6px 0 0;
  color: #475569;
  line-height: 1.55;
}

.stock-alert-list small {
  margin-top: 6px;
  color: #94a3b8;
}

.receipt-grid,
.content-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.7fr) minmax(320px, 0.9fr);
  gap: 14px;
  align-items: start;
}

.status-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 14px;
}

.kitchen-check {
  display: grid;
  grid-template-columns: 130px 120px 1fr;
  gap: 10px;
  margin-bottom: 12px;
}

.kitchen-check div,
.handler-card,
.items-summary {
  padding: 13px;
  border: 1px solid #bfdbfe;
  border-radius: 16px;
  background:
    radial-gradient(circle at 100% 0%, rgba(37, 99, 235, 0.1), transparent 34%),
    #eff6ff;
}

.kitchen-check span,
.handler-card span,
.items-summary span {
  display: block;
  color: #2563eb;
  font-size: 12px;
  font-weight: 800;
}

.kitchen-check strong {
  display: block;
  margin-top: 6px;
  color: #0f2747;
  font-size: 18px;
  line-height: 1.35;
  word-break: break-all;
}

.summary-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}

.summary-item {
  min-height: 82px;
  padding: 13px;
  border: 1px solid #e5edf9;
  border-radius: 16px;
  background: #f8fbff;
}

.summary-item.highlight {
  border-color: #93c5fd;
  background: linear-gradient(135deg, #eff6ff, #dbeafe);
}

.summary-item span,
.note-grid span {
  display: block;
  color: #64748b;
  font-size: 12px;
}

.summary-item strong {
  display: block;
  margin-top: 7px;
  color: #0f2747;
  font-size: 17px;
  line-height: 1.35;
}

.action-stack {
  display: grid;
  gap: 10px;
}

.primary-action-group,
.secondary-action-group {
  display: grid;
  gap: 10px;
  padding: 12px;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  background: #f8fbff;
}

.primary-action-group span,
.secondary-action-group span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
}

.primary-action-group strong {
  color: #0f2747;
  line-height: 1.5;
}

.secondary-action-group {
  border-color: #e2e8f0;
  background: #ffffff;
}

.secondary-action-group div {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.handler-card {
  margin-bottom: 12px;
}

.fulfillment-checklist {
  display: grid;
  gap: 8px;
  margin-bottom: 12px;
}

.fulfillment-checklist div {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 10px 12px;
  border: 1px solid #e5edf9;
  border-radius: 12px;
  background: #f8fbff;
}

.fulfillment-checklist div.done {
  border-color: #bbf7d0;
  background: #f0fdf4;
}

.fulfillment-checklist span {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: 999px;
  color: #2563eb;
  background: #dbeafe;
  font-weight: 900;
  flex: 0 0 auto;
}

.fulfillment-checklist div.done span {
  color: #166534;
  background: #bbf7d0;
}

.fulfillment-checklist p {
  margin: 0;
  color: #475569;
  line-height: 1.45;
}

.handler-card strong {
  display: block;
  margin-top: 6px;
  color: #0f2747;
  line-height: 1.55;
}

.handler-card small {
  display: block;
  margin-top: 6px;
  color: #64748b;
}

.process-tip {
  margin-top: 14px;
  padding: 14px;
  border: 1px solid #bbf7d0;
  border-radius: 16px;
  background: #ecfdf5;
  color: #0f766e;
}

.process-tip p,
.note-grid p,
.order-timeline p,
.product-cell p {
  margin: 6px 0 0;
  color: #64748b;
  line-height: 1.65;
}

.note-grid {
  display: grid;
  gap: 12px;
}

.note-grid div {
  padding: 14px;
  border: 1px solid #e5edf9;
  border-radius: 16px;
  background: #f8fbff;
}

.append-board {
  display: grid;
  gap: 12px;
  padding: 16px 18px;
  border: 1px solid #bfdbfe;
  border-radius: 20px;
  background:
    radial-gradient(circle at 100% 0%, rgba(37, 99, 235, 0.1), transparent 34%),
    #ffffff;
  box-shadow: 0 14px 34px rgba(15, 39, 71, 0.06);
}

.append-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
}

.append-head span,
.append-head strong,
.append-head p {
  display: block;
}

.append-head span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.08em;
}

.append-head strong {
  margin-top: 6px;
  color: #0f2747;
  font-size: 20px;
}

.append-head p {
  margin: 7px 0 0;
  color: #64748b;
  line-height: 1.6;
}

.append-list {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 10px;
}

.append-list article {
  padding: 12px;
  border: 1px solid #dbeafe;
  border-radius: 16px;
  background: #f8fbff;
}

.append-list b,
.append-list p,
.append-list small {
  display: block;
}

.append-list b {
  color: #1d4ed8;
}

.append-list p {
  margin: 6px 0 0;
  color: #334155;
  line-height: 1.55;
}

.append-list small {
  margin-top: 7px;
  color: #94a3b8;
}

.items-table,
.order-timeline {
  margin-top: 4px;
}

.items-summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.items-summary strong {
  color: #0f2747;
  font-size: 18px;
  white-space: nowrap;
}

.product-cell {
  display: flex;
  gap: 12px;
  align-items: center;
}

.product-cell img {
  width: 54px;
  height: 54px;
  border-radius: 14px;
  object-fit: cover;
  background: #eef4fb;
}

.refund-dashboard {
  display: grid;
  grid-template-columns: 1.4fr repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.refund-status-card,
.refund-metric {
  min-height: 112px;
  padding: 15px;
  border: 1px solid #e5edf9;
  border-radius: 18px;
  background: #f8fbff;
}

.refund-status-card {
  background:
    radial-gradient(circle at 100% 0%, rgba(37, 99, 235, 0.12), transparent 34%),
    linear-gradient(135deg, #eff6ff, #ffffff);
}

.refund-status-card.partial {
  border-color: #fed7aa;
  background:
    radial-gradient(circle at 100% 0%, rgba(249, 115, 22, 0.13), transparent 34%),
    #fff7ed;
}

.refund-status-card.full {
  border-color: #bbf7d0;
  background:
    radial-gradient(circle at 100% 0%, rgba(22, 163, 74, 0.12), transparent 34%),
    #ecfdf5;
}

.refund-status-card span,
.refund-metric span {
  display: block;
  color: #64748b;
  font-size: 12px;
}

.refund-status-card strong,
.refund-metric strong {
  display: block;
  margin-top: 7px;
  color: #0f2747;
  font-size: 22px;
  line-height: 1.2;
}

.refund-status-card p {
  margin: 8px 0 0;
  color: #64748b;
  line-height: 1.55;
}

.refund-metric.highlight {
  border-color: #93c5fd;
  background: linear-gradient(135deg, #eff6ff, #e0f2fe);
}

.refund-advice {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-top: 12px;
  padding: 14px;
  border: 1px solid #bfdbfe;
  border-radius: 18px;
  background: #f8fbff;
}

.refund-advice strong {
  display: block;
  color: #0f2747;
}

.refund-advice p {
  margin: 6px 0 0;
  color: #64748b;
  line-height: 1.6;
}

.refund-alert {
  margin-bottom: 16px;
}

.refund-records {
  display: grid;
  gap: 8px;
  margin-bottom: 14px;
  padding: 12px;
  border: 1px solid #fed7aa;
  border-radius: 16px;
  background: #fff7ed;
}

.refund-record-head,
.refund-records article {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.refund-records article small {
  display: block;
  margin-top: 4px;
  color: #c2410c;
}

.refund-record-head span,
.refund-records article span,
.refund-records p {
  color: #9a3412;
}

.refund-records p {
  margin: 0;
}

:global(.confirm-payment-card) {
  display: grid;
  gap: 8px;
  color: #1f2937;
}

:global(.confirm-payment-card p) {
  margin: 0 0 4px;
  color: #92400e;
  line-height: 1.6;
}

:global(.confirm-payment-card div) {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 10px;
  border: 1px solid #fde68a;
  border-radius: 10px;
  background: #fffbeb;
}

:global(.confirm-payment-card span) {
  color: #6b7280;
}

:global(.confirm-payment-card strong) {
  text-align: right;
  color: #111827;
}

@media (max-width: 1180px) {
  .transaction-brief,
  .payment-check-board {
    grid-template-columns: 1fr;
  }

  .brief-actions,
  .payment-check-actions {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .receipt-grid,
  .fulfillment-strip,
  .content-grid {
    grid-template-columns: 1fr;
  }

  .refund-dashboard {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .kitchen-check {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .summary-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 720px) {
  .brief-metrics,
  .brief-actions,
  .payment-check-grid,
  .payment-check-actions,
  .summary-grid,
  .kitchen-check,
  .refund-dashboard {
    grid-template-columns: 1fr;
  }

  .items-summary {
    align-items: flex-start;
    flex-direction: column;
  }

  .refund-advice {
    align-items: flex-start;
    flex-direction: column;
  }

  .append-head {
    flex-direction: column;
  }
}

@media print {
  :global(.merchant-sidebar),
  :global(.el-button),
  :global(.el-dialog),
  .action-stack,
  .process-tip {
    display: none !important;
  }

  .order-detail-page {
    display: block;
    background: #fff;
  }

  .receipt-grid,
  .content-grid {
    display: block;
  }
}
</style>
