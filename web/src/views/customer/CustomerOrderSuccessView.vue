<template>
  <main class="order-status-page">
    <section class="phone-shell">
      <header class="status-hero" :class="heroClass">
        <div class="hero-top">
          <span>订单状态</span>
          <el-button text class="refresh-btn" :loading="loading" @click="load">刷新</el-button>
        </div>
        <div class="status-icon">{{ statusMeta.icon }}</div>
        <h1>{{ statusMeta.title }}</h1>
        <p>{{ statusMeta.tip }}</p>
        <div class="hero-actions">
          <el-button v-if="canRetry" type="primary" plain :loading="retrying" @click="retryPay">
            {{ status === 'failed' ? '重新支付' : '继续支付' }}
          </el-button>
          <el-button v-else-if="status === 'payment_confirming'" plain :loading="loading" @click="load">
            刷新状态
          </el-button>
          <el-button v-else-if="canAppendItems" plain @click="continueOrder">
            继续选购
          </el-button>
          <el-button v-else-if="['received', 'accepted'].includes(status)" plain :disabled="!contactPhone" @click="callStore">
            联系门店
          </el-button>
          <el-button v-else plain @click="backStore">再逛逛</el-button>
        </div>
      </header>

      <section v-if="loading && !order.order_no" class="panel">
        <el-skeleton :rows="8" animated />
      </section>

      <template v-else>
        <section class="panel order-card">
          <div class="order-no">
            <div>
              <span>订单号</span>
              <strong>{{ order.order_no || '-' }}</strong>
            </div>
            <el-button text type="primary" @click="copyOrderNo">复制</el-button>
          </div>
          <div class="next-action">
            <span>{{ nextAction.label }}</span>
            <strong>{{ nextAction.title }}</strong>
            <p>{{ nextAction.tip }}</p>
          </div>
          <div v-if="showPaidNotice" class="paid-notice">
            <span>PAYMENT DONE</span>
            <strong>付款已确认，不需要商家审核付款</strong>
            <p>你只需要等待门店接单、处理或履约；到店核对时出示本页订单号和订单核对码即可。</p>
          </div>
          <div v-if="payReturnNotice" class="pay-return-notice">
            <span>PAYMENT CHECK</span>
            <strong>{{ payReturnNotice.title }}</strong>
            <p>{{ payReturnNotice.tip }}</p>
          </div>
          <div v-if="status === 'payment_confirming'" class="confirming-notice">
            <span>PAYMENT SUBMITTED</span>
            <strong>已提交付款确认，请不要重复付款</strong>
            <p>商家会核对到账金额。确认后，本页会自动更新为已付款处理状态；如果等待较久，可以刷新或联系门店。</p>
          </div>
          <div class="pickup-strip">
            <div>
              <span>订单核对码</span>
              <strong>{{ pickupCode }}</strong>
            </div>
            <div>
              <span>商品数量</span>
              <strong>{{ totalQuantity }} 件</strong>
            </div>
            <div>
              <span>门店电话</span>
              <strong>{{ contactPhone || '-' }}</strong>
            </div>
          </div>
          <el-alert
            v-if="isRefunded"
            class="refund-tip"
            type="warning"
            show-icon
            :closable="false"
            :title="`订单已记录退款 ${yuan(order.refunded_amount)}，确认后净额 ${yuan(confirmedNetAmount)}，具体到账以支付渠道为准。`"
          />
          <div class="summary-grid">
            <div>
              <span>门店</span>
              <strong>{{ order.store?.name || '-' }}</strong>
            </div>
            <div>
              <span>应收金额</span>
              <strong>{{ yuan(receivableAmount) }}</strong>
            </div>
            <div>
              <span>收款状态</span>
              <strong>{{ paymentSummary.title }}</strong>
            </div>
            <div>
              <span>优惠抵扣</span>
              <strong>-{{ yuan(order.discount_amount) }}</strong>
            </div>
            <div>
              <span>退款金额</span>
              <strong>{{ yuan(order.refunded_amount) }}</strong>
            </div>
            <div class="net-paid">
              <span>确认后净额</span>
              <strong>{{ yuan(confirmedNetAmount) }}</strong>
              <small>{{ paymentSummary.hint }}</small>
            </div>
          </div>
          <div class="refresh-line">
            <span>最近刷新：{{ lastLoadedAt || '-' }}</span>
            <span v-if="canRetry">页面会自动刷新支付状态</span>
          </div>

          <div class="action-focus-card" :class="actionFocus.tone">
            <div>
              <span>{{ actionFocus.label }}</span>
              <strong>{{ actionFocus.title }}</strong>
              <p>{{ actionFocus.text }}</p>
            </div>
            <div class="action-focus-buttons">
              <el-button
                v-if="actionFocus.primary === 'retry'"
                type="primary"
                :loading="retrying"
                @click="retryPay"
              >
                {{ status === 'failed' ? '重新支付' : '继续支付' }}
              </el-button>
              <el-button
                v-else-if="actionFocus.primary === 'mark_paid'"
                type="primary"
                :loading="markingPaid"
                @click="markPaid"
              >
                我已付款
              </el-button>
              <el-button
                v-else-if="actionFocus.primary === 'refresh'"
                type="primary"
                :loading="loading"
                @click="load"
              >
                刷新状态
              </el-button>
              <el-button
                v-else-if="actionFocus.primary === 'append'"
                type="primary"
                @click="continueOrder"
              >
                继续选购
              </el-button>
              <el-button v-else type="primary" @click="backStore">返回门店</el-button>
              <el-button plain :disabled="!contactPhone" @click="callStore">联系门店</el-button>
            </div>
          </div>

          <div v-if="canRetry" class="pay-rescue-card">
            <div>
              <span>{{ status === 'failed' ? '支付失败/中断' : '待支付订单' }}</span>
              <strong>订单已保留，不需要重新点单</strong>
              <p>继续支付会沿用当前订单和优惠；如果刚才关闭了支付页，点下面按钮重新发起即可。</p>
            </div>
            <el-button type="primary" :loading="retrying" @click="retryPay">
              {{ status === 'failed' ? '重新支付' : '继续支付' }}
            </el-button>
          </div>

          <div v-if="canRetry" class="retry-box">
            <strong>{{ status === 'failed' ? '支付未完成，可重新发起' : '订单待支付，请继续完成付款' }}</strong>
            <p>如果刚刚中断或关闭了支付宝页面，可以在这里重新支付，不需要重新点单。</p>
            <el-radio-group v-model="payMode" class="pay-mode">
              <el-radio-button label="page">跳转支付宝</el-radio-button>
              <el-radio-button label="qr">扫码支付</el-radio-button>
            </el-radio-group>
            <el-button type="primary" size="large" :loading="retrying" @click="retryPay">
              {{ status === 'failed' ? '重新支付' : '继续支付' }}
            </el-button>
          </div>

          <div v-if="canUseMerchantQr" class="merchant-pay-card">
            <div class="merchant-pay-head">
              <div>
                <span>MERCHANT QR</span>
                <strong>扫码向商家付款</strong>
                <p>付款会直接进入商家账户。付款完成后回到本页点击“我已付款”，商家核对到账后订单会继续处理。</p>
              </div>
              <b>{{ yuan(order.total_amount || order.amount) }}</b>
            </div>
            <div class="pay-steps">
              <div>
                <span>1</span>
                <strong>选择付款方式</strong>
                <p>使用支付宝或微信扫描下方商家收款码。</p>
              </div>
              <div>
                <span>2</span>
                <strong>确认付款金额</strong>
                <p>付款金额请与本页应付金额保持一致。</p>
              </div>
              <div>
                <span>3</span>
                <strong>返回本页确认</strong>
                <p>付款后点击“我已付款”，等待商家核对到账。</p>
              </div>
            </div>
            <div class="merchant-qr-grid">
              <article v-for="item in merchantPayOptions" :key="item.key">
                <strong>{{ item.label }}</strong>
                <img :src="qrDisplayUrl(item.value, item.key)" :alt="item.label" @error="markMerchantQrBroken(item.key)" />
                <small>{{ paymentConfig.account_name || order.store?.name || '商家收款账户' }}</small>
              </article>
            </div>
            <div class="merchant-pay-meta">
              <span>订单号：{{ order.order_no || '-' }}</span>
              <span>核对码：{{ pickupCode }}</span>
              <span>应付：{{ yuan(order.total_amount || order.amount) }}</span>
            </div>
            <el-button
              type="primary"
              size="large"
              class="paid-mark-btn"
              :loading="markingPaid"
              @click="markPaid"
            >
              我已付款，通知商家确认
            </el-button>
          </div>
          <div v-else-if="showMerchantQrMissing" class="merchant-pay-missing">
            <strong>门店暂未启用线上收款码</strong>
            <p>当前订单已提交。请按门店现场指引付款，或联系门店确认付款方式。</p>
            <el-button plain :disabled="!contactPhone" @click="callStore">联系门店</el-button>
          </div>
        </section>

        <section class="panel">
          <div class="section-title">
            <div>
              <span>PROCESS</span>
              <h2>订单进度</h2>
            </div>
            <el-tag :type="statusTagType">{{ statusLabel }}</el-tag>
          </div>
          <div class="timeline">
            <article
              v-for="step in timeline"
              :key="step.key"
              :class="['step', { active: step.active, done: step.done }]"
            >
              <div class="dot"></div>
              <div>
                <strong>{{ step.title }}</strong>
                <p>{{ step.text }}</p>
                <small>{{ step.time }}</small>
              </div>
            </article>
          </div>
        </section>

        <section class="panel">
          <div class="section-title">
            <div>
              <span>ITEMS</span>
              <h2>商品明细</h2>
            </div>
            <strong>{{ items.length }} 项</strong>
          </div>
          <div v-if="items.length" class="items">
            <article v-for="item in items" :key="`${item.product_id || item.name}-${item.quantity}`" class="item-row">
              <div>
                <strong>{{ item.name }}</strong>
                <span>{{ yuan(item.price) }} x {{ item.quantity }}</span>
              </div>
              <b>{{ yuan(item.line_amount || item.price * item.quantity) }}</b>
            </article>
          </div>
          <el-empty v-else description="暂无商品明细" />
        </section>

        <section class="panel info-panel">
          <div>
            <span>顾客备注</span>
            <p>{{ order.customer_note || '暂无备注' }}</p>
          </div>
          <div>
            <span>门店联系</span>
            <p>{{ contactPhone || '暂无联系方式' }}</p>
          </div>
          <div>
            <span>下单时间</span>
            <p>{{ formatTime(order.created_at) }}</p>
          </div>
        </section>

        <section v-if="canSharePoster" class="panel poster-panel">
          <div class="section-title">
            <div>
              <span>SHARE</span>
              <h2>分享领券海报</h2>
            </div>
          </div>
          <div class="poster-card">
            <div class="poster-glow"></div>
            <div class="poster-head">
              <div>
                <span class="poster-badge">好友扫码领券</span>
                <h3>{{ order.store?.name || '本地好店' }}</h3>
              </div>
              <div class="reward-stamp">
                <strong>{{ couponRewardText }}</strong>
                <span>复购奖励</span>
              </div>
            </div>
            <div class="poster-product">
              <span>今日推荐</span>
              <strong>{{ recommendedProduct }}</strong>
              <p>{{ posterCopy }}</p>
            </div>
            <div class="poster-coupon-row">
              <div class="coupon-ticket">
                <span>好友专享券</span>
                <strong>{{ friendCouponText }}</strong>
                <small>扫码进店可领取</small>
              </div>
              <div class="poster-qr-box">
                <img :src="posterQrUrl" alt="门店二维码" />
                <small>长按识别二维码</small>
              </div>
            </div>
            <div class="poster-footer">
              <span>分享给好友，下次消费更省钱</span>
              <b>{{ order.order_no || '' }}</b>
            </div>
          </div>
          <div class="poster-actions">
            <el-button type="primary" :loading="sharing" @click="copyShareText">复制分享文案</el-button>
            <el-button @click="backStore">进入门店</el-button>
          </div>
        </section>

        <section v-if="canReview" class="panel review-panel">
          <div class="section-title">
            <div>
              <span>REVIEW</span>
              <h2>{{ order.reviewed_at ? '我的评价' : '给商家一个评价' }}</h2>
            </div>
          </div>
          <div v-if="order.reviewed_at" class="review-result">
            <el-rate :model-value="order.customer_rating" disabled />
            <p>{{ order.customer_review || '顾客只留下了星级评分。' }}</p>
          </div>
          <div v-else class="review-form">
            <el-rate v-model="reviewForm.rating" />
            <el-input
              v-model="reviewForm.review"
              type="textarea"
              :rows="3"
              maxlength="200"
              show-word-limit
              placeholder="写下你的体验，帮助商家持续改进。"
            />
            <el-button type="primary" :loading="reviewing" @click="submitReview">提交评价</el-button>
          </div>
        </section>

        <div class="bottom-actions">
          <el-button v-if="canRetry" type="primary" size="large" :loading="retrying" @click="retryPay">继续支付</el-button>
          <el-button v-else-if="status === 'payment_confirming'" type="primary" size="large" :loading="loading" @click="load">刷新状态</el-button>
          <el-button v-else-if="canAppendItems" type="primary" size="large" @click="continueOrder">继续选购</el-button>
          <el-button v-else type="primary" size="large" @click="backStore">返回门店</el-button>
          <el-button size="large" :disabled="!contactPhone" @click="callStore">联系门店</el-button>
        </div>
      </template>
    </section>

    <el-dialog v-model="qrDialogVisible" title="支付宝扫码支付" width="92%" class="qr-dialog">
      <div class="qr-wrap">
        <img :src="qrImageUrl" alt="支付宝扫码支付" />
        <p>请使用支付宝扫码完成支付。支付成功后，本页面会自动刷新订单状态。</p>
      </div>
    </el-dialog>
  </main>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  createCustomerOrderShare,
  fetchCustomerOrder,
  markCustomerOrderPaid,
  retryCustomerOrderPayment,
  reviewCustomerOrder
} from '../../api/modules'

const route = useRoute()
const router = useRouter()
const loading = ref(false)
const retrying = ref(false)
const markingPaid = ref(false)
const sharing = ref(false)
const reviewing = ref(false)
const order = ref({})
const paymentConfig = ref({})
const brokenMerchantQrs = ref({})
const lastLoadedAt = ref('')
const previousStatus = ref('')
const payMode = ref('page')
const qrDialogVisible = ref(false)
const qrImageUrl = ref('')
const shareCampaign = ref({})
const reviewForm = ref({ rating: 5, review: '' })
let pollTimer = null
let pollAttempts = 0

const orderNo = computed(() => route.params.orderNo || route.query.orderNo || '')
const isPayReturn = computed(() => route.query.from === 'pay_return')
const status = computed(() => order.value.status || 'pending')
const items = computed(() => Array.isArray(order.value.items) ? order.value.items : [])
const storeId = computed(() => order.value.store_id || order.value.store?.id)
const contactPhone = computed(() => order.value.store?.contact_phone || order.value.merchant?.contact_phone || '')
const isRefunded = computed(() => Number(order.value.refunded_amount || 0) > 0)
const confirmedStatuses = ['received', 'accepted', 'completed', 'closed']
const receivableAmount = computed(() => Number(order.value.total_amount || order.value.amount || 0))
const paidAmount = computed(() => Number(order.value.paid_amount ?? order.value.total_amount ?? order.value.amount ?? 0))
const isConfirmedPaid = computed(() => confirmedStatuses.includes(status.value) || Boolean(order.value.paid_at))
const confirmedNetAmount = computed(() => {
  if (!isConfirmedPaid.value) return 0
  return Math.max(paidAmount.value - Number(order.value.refunded_amount || 0), 0)
})
const totalQuantity = computed(() => items.value.reduce((sum, item) => sum + Number(item.quantity || 0), 0))
const pickupCode = computed(() => {
  const text = String(order.value.order_no || '')
  return text ? text.slice(-4).toUpperCase() : '-'
})
const canRetry = computed(() => ['pending', 'failed'].includes(status.value))
const canAppendItems = computed(() => ['submitted', 'preparing'].includes(status.value) && !order.value.paid_at && Boolean(storeId.value))
const canRequestShare = computed(() => ['received', 'accepted', 'completed'].includes(status.value) && Boolean(order.value.paid_at))
const canSharePoster = computed(() => canRequestShare.value && Boolean(shareCampaign.value.share_code))
const canReview = computed(() => ['accepted', 'completed'].includes(status.value) && Boolean(order.value.paid_at))
const showPaidNotice = computed(() => ['received', 'accepted', 'completed'].includes(status.value) && Boolean(order.value.paid_at) && !isRefunded.value)
const paymentSummary = computed(() => {
  if (['pending', 'failed'].includes(status.value)) {
    return { title: '待支付', hint: '未付款，不计入已收款' }
  }
  if (status.value === 'payment_confirming') {
    return { title: '待商家确认收款', hint: '请勿重复付款' }
  }
  if (['submitted', 'preparing'].includes(status.value) && !order.value.paid_at) {
    return { title: '已提交，稍后结算', hint: '未确认收款' }
  }
  if (isConfirmedPaid.value) {
    return { title: '已确认收款', hint: Number(order.value.refunded_amount || 0) > 0 ? '已扣除退款' : '已确认' }
  }
  return { title: '待核对', hint: '请联系门店核对' }
})
const merchantPayOptions = computed(() => {
  const options = []
  if (paymentConfig.value.alipay_qr_code) options.push({ key: 'alipay', label: '支付宝收款码', value: paymentConfig.value.alipay_qr_code })
  if (paymentConfig.value.wechat_qr_code) options.push({ key: 'wechat', label: '微信收款码', value: paymentConfig.value.wechat_qr_code })
  return options
})
const canUseMerchantQr = computed(() => ['submitted', 'preparing'].includes(status.value) && !order.value.paid_at && merchantPayOptions.value.length > 0)
const showMerchantQrMissing = computed(() => ['submitted', 'preparing'].includes(status.value) && !order.value.paid_at && merchantPayOptions.value.length === 0)
const payReturnNotice = computed(() => {
  if (!isPayReturn.value || !canRetry.value) return null
  if (status.value === 'failed') {
    return {
      title: '支付未完成，订单还在',
      tip: '可能是支付页被关闭、支付失败或支付渠道返回关闭状态。可以直接继续支付，不需要重新点单。'
    }
  }
  return {
    title: '正在同步付款结果',
    tip: '如果你已经完成付款，请稍等几秒，页面会自动进入商家处理；不需要商家审核付款。'
  }
})
const actionFocus = computed(() => {
  if (canRetry.value) {
    return {
      label: status.value === 'failed' ? 'PAYMENT RETRY' : 'PAYMENT',
      title: status.value === 'failed' ? '支付未完成，可重新发起' : '订单已保留，请继续完成付款',
      text: '继续支付会沿用当前订单和优惠，不需要重新点单。',
      primary: 'retry',
      tone: 'pay'
    }
  }
  if (canUseMerchantQr.value) {
    return {
      label: 'MERCHANT QR',
      title: '付款后记得点击“我已付款”',
      text: '商家核对到账后订单会继续处理；没有到账前不要重复提交。',
      primary: 'mark_paid',
      tone: 'pay'
    }
  }
  if (status.value === 'payment_confirming') {
    return {
      label: 'WAITING CONFIRM',
      title: '已提交付款确认，请等待商家核对',
      text: '不用重复扫码付款，可刷新状态或联系门店确认。',
      primary: 'refresh',
      tone: 'warning'
    }
  }
  if (canAppendItems.value) {
    return {
      label: 'CONTINUE ORDER',
      title: '还可以继续选购',
      text: '继续选择的商品会追加到当前进行中的订单。',
      primary: 'append',
      tone: 'primary'
    }
  }
  if (['received', 'accepted'].includes(status.value)) {
    return {
      label: 'ORDER PROCESSING',
      title: '订单正在由门店处理',
      text: '到店或核对时出示订单号和核对码即可。',
      primary: 'store',
      tone: 'success'
    }
  }
  return {
    label: 'NEXT',
    title: status.value === 'completed' ? '订单已完成，欢迎再来' : '可返回门店继续浏览',
    text: status.value === 'completed' ? '可以评价、分享领券或再次下单。' : '如订单异常，请保留订单号联系门店。',
    primary: 'store',
    tone: status.value === 'completed' ? 'success' : 'primary'
  }
})

const statusMap = {
  pending: { title: '等待支付', tip: '订单已经创建，请在有效时间内完成支付。', icon: '¥', tag: 'warning' },
  failed: { title: '支付未完成', tip: '支付失败或已中断，可以重新发起支付。', icon: '!', tag: 'danger' },
  submitted: { title: '订单已提交', tip: '订单已发送给商家，可在本页查看处理进度。', icon: '单', tag: 'primary' },
  preparing: { title: '商家处理中', tip: '订单已进入处理流程，请留意页面状态。', icon: '处', tag: 'success' },
  payment_confirming: { title: '等待商家确认收款', tip: '已提交付款确认，请不要重复付款。商家核对到账后订单会继续处理。', icon: '款', tag: 'warning' },
  received: { title: '付款成功，商家处理中', tip: '订单已发送给商家，门店会尽快处理。', icon: '✓', tag: 'primary' },
  accepted: { title: '商家已接单', tip: '订单正在处理或履约，请留意门店通知。', icon: '单', tag: 'success' },
  completed: { title: '订单已完成', tip: '感谢下单，欢迎再次光临。', icon: '好', tag: 'success' },
  closed: { title: '订单已关闭', tip: '订单已关闭，如有疑问请联系门店。', icon: '×', tag: 'info' }
}

const statusMeta = computed(() => {
  if (isRefunded.value && order.value.refund_status === 'full') {
    return { title: '订单已退款', tip: '退款已记录，请以实际支付渠道到账为准。', icon: '退', tag: 'warning' }
  }
  return statusMap[status.value] || statusMap.received
})
const statusLabel = computed(() => statusMeta.value.title)
const statusTagType = computed(() => statusMeta.value.tag)
const heroClass = computed(() => ({
  waiting: status.value === 'pending',
  failed: status.value === 'failed' || status.value === 'closed',
  done: status.value === 'completed',
  refunded: isRefunded.value
}))

const nextAction = computed(() => {
  if (canRetry.value) {
    return {
      label: '下一步',
      title: isPayReturn.value ? '付款结果同步后直接进入商家处理' : '完成付款后订单会自动进入商家处理',
      tip: isPayReturn.value
        ? '支付渠道回到本页后会自动同步状态；这不是商家审核，只是确认支付渠道结果。'
        : '如果刚才支付页面被关闭，可以点击继续支付；支付成功后不需要商家再审核付款。'
    }
  }
  if (status.value === 'submitted') {
    return {
      label: '当前进度',
      title: '订单已提交给商家',
      tip: '商家会根据门店设置处理订单；后续可在本页继续查看状态。'
    }
  }
  if (status.value === 'preparing') {
    return {
      label: '当前进度',
      title: '商家正在处理订单',
      tip: '订单已自动进入处理流程；如需沟通可联系门店。'
    }
  }
  if (status.value === 'payment_confirming') {
    return {
      label: '当前进度',
      title: '等待商家确认收款',
      tip: '你已经提交“我已付款”，不用再次扫码。页面会自动刷新，商家确认到账后订单会继续处理。'
    }
  }
  if (status.value === 'received') {
    return {
      label: '当前进度',
      title: '订单已到达商家',
      tip: '付款已完成，门店可直接处理订单；如需加急或修改备注，可联系门店。'
    }
  }
  if (status.value === 'accepted') {
    return {
      label: '当前进度',
      title: '商家正在处理订单',
      tip: '请留意门店通知，需要核对时可出示本页面订单信息。'
    }
  }
  if (status.value === 'completed') {
    return {
      label: '复购建议',
      title: '订单已完成，可以评价或分享领券',
      tip: '留下评价可帮助商家改进，分享给好友还能形成复购和裂变。'
    }
  }
  return {
    label: '提示',
    title: '如有疑问请联系门店',
    tip: '订单异常或已关闭时，建议保留订单号并联系门店核对。'
  }
})

const storeUrl = computed(() => {
  if (!storeId.value) return window.location.href
  const code = shareCampaign.value.share_code
  return `${window.location.origin}/customer/store/${storeId.value}${code ? `?share_code=${encodeURIComponent(code)}` : ''}`
})
const posterQrUrl = computed(() => buildQrImage(storeUrl.value))
const posterCopy = computed(() => {
  if (shareCampaign.value.poster_copy) return shareCampaign.value.poster_copy
  const storeName = order.value.store?.name || '这家本地好店'
  const firstItem = items.value[0]?.name
  if (firstItem) return `我刚在${storeName}下单了${firstItem}，体验不错，扫码看看有没有你的专属优惠。`
  return `我刚在${storeName}下单，扫码看看有没有你的专属优惠。`
})
const recommendedProduct = computed(() => items.value[0]?.name || '门店招牌好物')
const friendCouponText = computed(() => {
  const discount = Number(order.value.discount_amount || 0)
  if (discount > 0) return `立减 ${yuan(Math.max(500, Math.min(discount, 2000)))}`
  return '到店立减券'
})
const couponRewardText = computed(() => {
  if (shareCampaign.value.reward_text) return shareCampaign.value.reward_text
  return '返券'
})

const timeline = computed(() => {
  const rank = { pending: 1, failed: 1, submitted: 2, received: 2, preparing: 3, payment_confirming: 3, accepted: 3, completed: 4, closed: 4 }
  const current = rank[status.value] || 1
  const isSubmitLater = ['submitted', 'preparing', 'payment_confirming'].includes(status.value) && !order.value.paid_at
  return [
    {
      key: 'created',
      title: '订单创建',
      text: '顾客已提交订单。',
      time: formatTime(order.value.created_at),
      done: current >= 1,
      active: current === 1
    },
    {
      key: 'paid',
      title: isSubmitLater ? '订单提交' : '支付完成',
      text: isSubmitLater ? '订单已发送给商家，后续可继续查看处理进度。' : '支付成功后订单自动进入商家处理。',
      time: order.value.paid_at ? formatTime(order.value.paid_at) : '-',
      done: current >= 2,
      active: current === 2
    },
    {
      key: 'accepted',
      title: status.value === 'payment_confirming' ? '确认收款' : status.value === 'preparing' ? '商家处理中' : '商家接单',
      text: status.value === 'payment_confirming' ? '已提交付款确认，不需要重复付款，等待商家核对到账。' : status.value === 'preparing' ? '门店已开启自动接单，订单进入处理流程。' : '门店接单后继续处理或履约；这不是付款审核。',
      time: logTime('accepted') || logTime('auto_accepted'),
      done: current >= 3,
      active: current === 3
    },
    {
      key: 'completed',
      title: status.value === 'closed' ? '订单关闭' : '订单完成',
      text: status.value === 'closed' ? '订单已关闭或取消。' : '服务或商品已完成交付。',
      time: logTime(status.value === 'closed' ? 'closed' : 'completed'),
      done: current >= 4,
      active: current === 4
    }
  ]
})

const load = async () => {
  if (!orderNo.value) return
  loading.value = true
  try {
    const res = await fetchCustomerOrder(orderNo.value)
    const nextOrder = res.data.order || {}
    if (previousStatus.value && nextOrder.status && previousStatus.value !== nextOrder.status) {
      ElMessage.success(`订单状态已更新：${statusMap[nextOrder.status]?.title || nextOrder.status}`)
    }
    previousStatus.value = nextOrder.status || previousStatus.value
    order.value = nextOrder
    paymentConfig.value = res.data.payment_config || paymentConfig.value || {}
    lastLoadedAt.value = new Date().toLocaleTimeString('zh-CN', { hour12: false })
    if (status.value === 'failed') {
      stopPolling()
    } else if (!canRetry.value && status.value !== 'payment_confirming') {
      qrDialogVisible.value = false
      stopPolling()
      ensureShareCampaign()
    }
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '订单加载失败')
  } finally {
    loading.value = false
  }
}

const markPaid = async () => {
  if (!orderNo.value) return
  markingPaid.value = true
  try {
    const res = await markCustomerOrderPaid(orderNo.value)
    order.value = res.data.order || order.value
    paymentConfig.value = res.data.payment_config || paymentConfig.value
    ElMessage.success('已提交付款确认，请等待商家核对到账')
    startPolling('payment_confirming')
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '通知商家失败')
  } finally {
    markingPaid.value = false
  }
}

const retryPay = async () => {
  if (!orderNo.value) return
  retrying.value = true
  try {
    const res = await retryCustomerOrderPayment(orderNo.value, { pay_mode: payMode.value })
    order.value = res.data.order || order.value
    const payment = res.data.payment
    if (payment?.mode === 'qr' && payment.qr_code) {
      qrImageUrl.value = buildQrImage(payment.qr_code)
      qrDialogVisible.value = true
      startPolling('qr')
      return
    }
    const paymentUrl = payment?.payment_url || payment?.payment_form || payment?.form
    if (paymentUrl) submitAlipayPage(paymentUrl)
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '重新支付失败')
  } finally {
    retrying.value = false
  }
}

const ensureShareCampaign = async () => {
  if (!canRequestShare.value || !orderNo.value || shareCampaign.value.share_code || sharing.value) return
  sharing.value = true
  try {
    const res = await createCustomerOrderShare(orderNo.value)
    shareCampaign.value = res.data.campaign || {}
  } catch {
    shareCampaign.value = {}
    // 分享海报属于增强能力，失败不影响订单状态展示。
  } finally {
    sharing.value = false
  }
}

const submitReview = async () => {
  if (!orderNo.value) return
  reviewing.value = true
  try {
    const res = await reviewCustomerOrder(orderNo.value, {
      rating: reviewForm.value.rating,
      review: reviewForm.value.review.trim()
    })
    order.value = res.data.order || order.value
    ElMessage.success('评价已提交，感谢反馈')
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '评价提交失败')
  } finally {
    reviewing.value = false
  }
}

const submitAlipayPage = (paymentUrl) => {
  if (paymentUrl.trim().startsWith('<form')) {
    document.open()
    document.write(paymentUrl)
    document.close()
    document.querySelector('form')?.submit()
    return
  }
  window.location.href = paymentUrl
}

const startPolling = (reason = 'status') => {
  stopPolling()
  pollAttempts = 0
  pollTimer = window.setInterval(() => {
    pollAttempts += 1
    load()
    if (pollAttempts >= 40 && reason !== 'qr') {
      stopPolling()
    }
  }, 3000)
}
const stopPolling = () => {
  if (pollTimer) window.clearInterval(pollTimer)
  pollTimer = null
  pollAttempts = 0
}
const backStore = () => {
  if (storeId.value) router.push(`/customer/store/${storeId.value}`)
}
const continueOrder = () => {
  if (storeId.value && order.value.order_no) {
    router.push(`/customer/store/${storeId.value}?active_order=${encodeURIComponent(order.value.order_no)}`)
  }
}
const callStore = () => {
  if (contactPhone.value) window.location.href = `tel:${contactPhone.value}`
}
const copyShareText = async () => {
  try {
    await navigator.clipboard.writeText(`${posterCopy.value}\n${storeUrl.value}`)
    ElMessage.success('分享文案已复制')
  } catch {
    ElMessage.warning('复制失败，请手动复制页面链接')
  }
}
const copyOrderNo = async () => {
  if (!order.value.order_no) return
  try {
    await navigator.clipboard.writeText(order.value.order_no)
    ElMessage.success('订单号已复制')
  } catch {
    ElMessage.warning('复制失败，请手动长按订单号复制')
  }
}
const logTime = (action) => {
  const logs = Array.isArray(order.value.operation_logs) ? order.value.operation_logs : []
  const log = logs.find((item) => item.action === action)
  return log?.time ? formatTime(log.time) : '-'
}
const buildQrImage = (value) => `https://api.qrserver.com/v1/create-qr-code/?size=260x260&data=${encodeURIComponent(value)}`
const qrDisplayUrl = (value, key = '') => {
  const text = String(value || '').trim()
  if (!text) return ''
  if (key && brokenMerchantQrs.value[key]) return buildQrImage(text)
  if (text.startsWith('data:image/')) return text
  if (/\.(png|jpe?g|webp|gif|svg)(\?.*)?$/i.test(text)) return text
  return buildQrImage(text)
}
const markMerchantQrBroken = (key) => {
  brokenMerchantQrs.value = { ...brokenMerchantQrs.value, [key]: true }
}
const yuan = (value) => `¥${(Number(value || 0) / 100).toFixed(2)}`
const formatTime = (value) => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '-'

onMounted(() => {
  load()
  if (orderNo.value) startPolling(isPayReturn.value ? 'pay_return' : 'status')
})
onBeforeUnmount(stopPolling)
</script>

<style scoped>
.order-status-page {
  min-height: 100vh;
  padding: 14px 0 30px;
  color: #142033;
  background:
    radial-gradient(circle at 10% 0%, rgba(37, 99, 235, 0.18), transparent 30%),
    radial-gradient(circle at 88% 8%, rgba(14, 165, 233, 0.2), transparent 34%),
    linear-gradient(180deg, #f8fbff 0%, #edf6ff 100%);
}

.phone-shell {
  width: min(430px, calc(100vw - 24px));
  margin: 0 auto;
  display: grid;
  gap: 12px;
}

.status-hero,
.panel {
  border: 1px solid rgba(203, 213, 225, 0.74);
  border-radius: 26px;
  background: rgba(255, 255, 255, 0.94);
  box-shadow: 0 18px 52px rgba(15, 23, 42, 0.11);
}

.status-hero {
  padding: 22px;
  color: #fff;
  background:
    radial-gradient(circle at 88% 12%, rgba(255, 255, 255, 0.24), transparent 30%),
    linear-gradient(135deg, #0f2747, #2563eb 58%, #06b6d4);
}

.status-hero.waiting {
  background: linear-gradient(135deg, #0f2747, #f59e0b);
}

.status-hero.failed {
  background: linear-gradient(135deg, #450a0a, #ef4444);
}

.status-hero.done {
  background: linear-gradient(135deg, #052e16, #16a34a);
}

.hero-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  opacity: 0.92;
}

.refresh-btn {
  color: #fff;
}

.status-icon {
  width: 58px;
  height: 58px;
  margin: 20px 0 12px;
  display: grid;
  place-items: center;
  border-radius: 20px;
  color: #0f2747;
  background: #fff;
  font-size: 28px;
  font-weight: 900;
}

.status-hero h1 {
  margin: 0;
  font-size: 26px;
  line-height: 1.25;
}

.status-hero p {
  margin: 8px 0 0;
  color: rgba(255, 255, 255, 0.82);
  line-height: 1.65;
}

.hero-actions {
  display: flex;
  gap: 10px;
  margin-top: 16px;
}

.hero-actions :deep(.el-button) {
  border-color: rgba(255, 255, 255, 0.72);
  color: #fff;
  background: rgba(255, 255, 255, 0.12);
}

.panel {
  padding: 16px;
}

.order-no {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: center;
  padding: 13px;
  border-radius: 18px;
  background: #f1f7ff;
}

.order-no span,
.summary-grid span,
.info-panel span {
  display: block;
  color: #64748b;
  font-size: 12px;
}

.order-no strong {
  display: block;
  margin-top: 4px;
  word-break: break-all;
}

.next-action {
  margin-top: 10px;
  padding: 14px;
  border: 1px solid #bfdbfe;
  border-radius: 20px;
  background: linear-gradient(135deg, #eff6ff, #f8fbff);
}

.pay-return-notice {
  margin-top: 10px;
  padding: 14px;
  border: 1px solid #fed7aa;
  border-radius: 20px;
  background: linear-gradient(135deg, #fff7ed, #ffffff);
}

.paid-notice {
  margin-top: 10px;
  padding: 14px;
  border: 1px solid #bbf7d0;
  border-radius: 20px;
  background: linear-gradient(135deg, #ecfdf5, #ffffff);
}

.confirming-notice {
  margin-top: 10px;
  padding: 14px;
  border: 1px solid #fde68a;
  border-radius: 20px;
  background: linear-gradient(135deg, #fffbeb, #ffffff);
}

.next-action span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.08em;
}

.pay-return-notice span {
  color: #ea580c;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.08em;
}

.paid-notice span {
  color: #059669;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.08em;
}

.confirming-notice span {
  color: #d97706;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.08em;
}

.next-action strong {
  display: block;
  margin-top: 5px;
  color: #0f2747;
  font-size: 16px;
}

.pay-return-notice strong {
  display: block;
  margin-top: 5px;
  color: #7c2d12;
  font-size: 16px;
}

.paid-notice strong {
  display: block;
  margin-top: 5px;
  color: #065f46;
  font-size: 16px;
}

.confirming-notice strong {
  display: block;
  margin-top: 5px;
  color: #92400e;
  font-size: 16px;
}

.next-action p {
  margin: 6px 0 0;
  color: #64748b;
  line-height: 1.55;
}

.pay-return-notice p {
  margin: 6px 0 0;
  color: #9a3412;
  line-height: 1.55;
}

.paid-notice p {
  margin: 6px 0 0;
  color: #047857;
  line-height: 1.55;
}

.confirming-notice p {
  margin: 6px 0 0;
  color: #92400e;
  line-height: 1.55;
}

.pickup-strip {
  display: grid;
  grid-template-columns: 1fr 1fr 1.2fr;
  gap: 8px;
  margin-top: 10px;
}

.pickup-strip div {
  min-height: 72px;
  padding: 12px;
  border: 1px solid #bfdbfe;
  border-radius: 18px;
  background:
    radial-gradient(circle at 100% 0%, rgba(37, 99, 235, 0.12), transparent 38%),
    #eff6ff;
}

.pickup-strip span {
  display: block;
  color: #2563eb;
  font-size: 11px;
  font-weight: 900;
}

.pickup-strip strong {
  display: block;
  margin-top: 7px;
  color: #0f2747;
  font-size: 17px;
  line-height: 1.2;
  word-break: break-all;
}

.refund-tip {
  margin-top: 10px;
  border-radius: 16px;
}

.summary-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  margin-top: 10px;
}

.summary-grid div,
.info-panel div {
  padding: 13px;
  border-radius: 18px;
  background: #f8fbff;
  border: 1px solid #e2eaf7;
}

.summary-grid .net-paid {
  border-color: #bfdbfe;
  background: linear-gradient(135deg, #eff6ff, #e0f2fe);
}

.summary-grid strong {
  display: block;
  margin-top: 6px;
  font-size: 18px;
}

.summary-grid small {
  display: block;
  margin-top: 3px;
  color: #64748b;
  font-size: 12px;
  line-height: 1.4;
}

.refresh-line {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  margin-top: 10px;
  color: #94a3b8;
  font-size: 12px;
}

.action-focus-card {
  display: grid;
  gap: 12px;
  margin-top: 12px;
  padding: 14px;
  border: 1px solid #bfdbfe;
  border-radius: 20px;
  background: linear-gradient(135deg, #eff6ff, #ffffff);
}

.action-focus-card.pay {
  border-color: #fed7aa;
  background: linear-gradient(135deg, #fff7ed, #ffffff);
}

.action-focus-card.warning {
  border-color: #fde68a;
  background: linear-gradient(135deg, #fffbeb, #ffffff);
}

.action-focus-card.success {
  border-color: #bbf7d0;
  background: linear-gradient(135deg, #ecfdf5, #ffffff);
}

.action-focus-card span,
.action-focus-card strong,
.action-focus-card p {
  display: block;
}

.action-focus-card span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.08em;
}

.action-focus-card.pay span {
  color: #ea580c;
}

.action-focus-card.warning span {
  color: #d97706;
}

.action-focus-card.success span {
  color: #059669;
}

.action-focus-card strong {
  margin-top: 5px;
  color: #0f2747;
  font-size: 17px;
}

.action-focus-card p {
  margin: 6px 0 0;
  color: #64748b;
  line-height: 1.55;
}

.action-focus-buttons {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 8px;
}

.action-focus-buttons :deep(.el-button) {
  min-height: 42px;
  border-radius: 14px;
}

.pay-rescue-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: 12px;
  padding: 14px;
  border-radius: 20px;
  color: #fff;
  background: linear-gradient(135deg, #0f2747, #2563eb 60%, #06b6d4);
  box-shadow: 0 16px 34px rgba(37, 99, 235, 0.2);
}

.pay-rescue-card span,
.pay-rescue-card strong,
.pay-rescue-card p {
  display: block;
}

.pay-rescue-card span {
  color: rgba(255, 255, 255, 0.72);
  font-size: 12px;
  font-weight: 900;
}

.pay-rescue-card strong {
  margin-top: 3px;
  font-size: 16px;
}

.pay-rescue-card p {
  margin: 5px 0 0;
  color: rgba(255, 255, 255, 0.76);
  font-size: 12px;
  line-height: 1.5;
}

.pay-rescue-card :deep(.el-button) {
  flex: 0 0 auto;
  border: 0;
  color: #0f2747;
  background: #fff;
}

.retry-box {
  margin-top: 12px;
  padding: 14px;
  border-radius: 20px;
  background: #fff7ed;
  border: 1px solid #fed7aa;
}

.retry-box p {
  margin: 6px 0 12px;
  color: #9a3412;
  line-height: 1.55;
}

.pay-mode {
  width: 100%;
  margin-bottom: 12px;
}

.retry-box :deep(.el-button--large) {
  width: 100%;
}

.merchant-pay-card {
  margin-top: 12px;
  display: grid;
  gap: 12px;
  padding: 14px;
  border: 1px solid #bfdbfe;
  border-radius: 20px;
  background: linear-gradient(135deg, #eff6ff, #ffffff);
}

.merchant-pay-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: flex-start;
}

.merchant-pay-head span {
  display: block;
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.08em;
}

.merchant-pay-head strong,
.merchant-pay-head p {
  display: block;
  margin: 5px 0 0;
}

.merchant-pay-head p {
  color: #64748b;
  line-height: 1.6;
}

.merchant-pay-head b {
  white-space: nowrap;
  color: #0f2747;
  font-size: 20px;
}

.pay-steps {
  display: grid;
  gap: 8px;
}

.pay-steps div {
  display: grid;
  grid-template-columns: 28px 1fr;
  column-gap: 8px;
  padding: 10px;
  border: 1px solid #dbeafe;
  border-radius: 16px;
  background: #ffffff;
}

.pay-steps span {
  width: 24px;
  height: 24px;
  display: grid;
  place-items: center;
  grid-row: span 2;
  border-radius: 999px;
  color: #ffffff;
  background: #2563eb;
  font-size: 12px;
  font-weight: 900;
}

.pay-steps strong {
  color: #0f2747;
}

.pay-steps p {
  grid-column: 2;
  margin: 3px 0 0;
  color: #64748b;
  font-size: 12px;
  line-height: 1.45;
}

.merchant-qr-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(148px, 1fr));
  gap: 10px;
}

.merchant-qr-grid article {
  display: grid;
  gap: 8px;
  justify-items: center;
  padding: 12px;
  border: 1px solid #dbeafe;
  border-radius: 18px;
  background: #ffffff;
}

.merchant-qr-grid img {
  width: 128px;
  height: 128px;
  object-fit: contain;
}

.merchant-qr-grid small {
  color: #64748b;
  text-align: center;
}

.merchant-pay-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.merchant-pay-meta span {
  padding: 6px 9px;
  border: 1px solid #bfdbfe;
  border-radius: 999px;
  color: #1d4ed8;
  background: #eff6ff;
  font-size: 12px;
  font-weight: 800;
}

.paid-mark-btn {
  width: 100%;
}

.merchant-pay-missing {
  display: grid;
  gap: 8px;
  margin-top: 14px;
  padding: 14px;
  border: 1px solid #fed7aa;
  border-radius: 18px;
  background: #fff7ed;
}

.merchant-pay-missing strong {
  color: #9a3412;
  font-size: 16px;
}

.merchant-pay-missing p {
  margin: 0;
  color: #9a3412;
  line-height: 1.55;
}

.section-title {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: flex-start;
  margin-bottom: 12px;
}

.section-title span {
  color: #2563eb;
  font-size: 11px;
  font-weight: 900;
  letter-spacing: 0.12em;
}

.section-title h2 {
  margin: 3px 0 0;
  font-size: 19px;
}

.timeline {
  display: grid;
  gap: 12px;
}

.step {
  display: grid;
  grid-template-columns: 18px 1fr;
  gap: 10px;
  position: relative;
}

.step:not(:last-child)::before {
  content: '';
  position: absolute;
  left: 7px;
  top: 20px;
  bottom: -13px;
  width: 2px;
  background: #dbeafe;
}

.dot {
  width: 16px;
  height: 16px;
  margin-top: 2px;
  border-radius: 50%;
  background: #dbeafe;
  border: 4px solid #eff6ff;
  z-index: 1;
}

.step.done .dot {
  background: #2563eb;
}

.step.active .dot {
  background: #22c55e;
  box-shadow: 0 0 0 6px rgba(34, 197, 94, 0.13);
}

.step strong {
  display: block;
}

.step p {
  margin: 4px 0;
  color: #64748b;
  line-height: 1.45;
}

.step small {
  color: #94a3b8;
}

.items {
  display: grid;
  gap: 8px;
}

.item-row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 12px;
  border-radius: 18px;
  background: #f8fbff;
  border: 1px solid #e2eaf7;
}

.item-row span {
  display: block;
  margin-top: 4px;
  color: #64748b;
  font-size: 12px;
}

.item-row b {
  white-space: nowrap;
}

.info-panel {
  display: grid;
  gap: 10px;
}

.info-panel p {
  margin: 6px 0 0;
  line-height: 1.6;
}

.poster-card {
  position: relative;
  overflow: hidden;
  padding: 18px;
  border-radius: 28px;
  color: #fff;
  background:
    radial-gradient(circle at 82% 10%, rgba(125, 211, 252, 0.44), transparent 27%),
    radial-gradient(circle at 12% 90%, rgba(34, 197, 94, 0.38), transparent 28%),
    linear-gradient(145deg, #061a35 0%, #0f3a8a 48%, #0891b2 100%);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.24), 0 20px 50px rgba(15, 39, 71, 0.22);
}

.poster-glow {
  position: absolute;
  inset: 12px;
  border: 1px solid rgba(255, 255, 255, 0.22);
  border-radius: 23px;
  pointer-events: none;
}

.poster-head,
.poster-coupon-row,
.poster-footer {
  position: relative;
  z-index: 1;
}

.poster-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: flex-start;
}

.poster-badge {
  display: inline-flex;
  padding: 5px 10px;
  border-radius: 999px;
  color: #075985;
  background: #e0f2fe;
  font-size: 12px;
  font-weight: 900;
}

.poster-card h3 {
  margin: 12px 0 0;
  font-size: 24px;
  line-height: 1.18;
}

.reward-stamp {
  width: 70px;
  height: 70px;
  display: grid;
  place-items: center;
  flex-shrink: 0;
  border: 2px dashed rgba(255, 255, 255, 0.72);
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.13);
  text-align: center;
  transform: rotate(8deg);
}

.reward-stamp strong {
  display: block;
  font-size: 19px;
  line-height: 1;
}

.reward-stamp span {
  margin-top: 2px;
  display: block;
  font-size: 10px;
  color: rgba(255, 255, 255, 0.78);
}

.poster-product {
  position: relative;
  z-index: 1;
  margin-top: 18px;
  padding: 14px;
  border-radius: 22px;
  background: rgba(255, 255, 255, 0.12);
  border: 1px solid rgba(255, 255, 255, 0.2);
}

.poster-product span {
  color: #bae6fd;
  font-size: 11px;
  font-weight: 900;
  letter-spacing: 0.12em;
}

.poster-product strong {
  display: block;
  margin-top: 4px;
  font-size: 21px;
}

.poster-product p {
  margin: 7px 0 0;
  color: rgba(255, 255, 255, 0.82);
  line-height: 1.6;
}

.poster-coupon-row {
  display: grid;
  grid-template-columns: 1fr 124px;
  gap: 12px;
  align-items: stretch;
  margin-top: 12px;
}

.coupon-ticket {
  position: relative;
  min-height: 128px;
  padding: 15px;
  border-radius: 22px;
  color: #0f2747;
  background: linear-gradient(135deg, #fff, #e0f2fe);
}

.coupon-ticket::before,
.coupon-ticket::after {
  content: '';
  position: absolute;
  right: -8px;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: #087aa4;
}

.coupon-ticket::before {
  top: 28px;
}

.coupon-ticket::after {
  bottom: 28px;
}

.coupon-ticket span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
}

.coupon-ticket strong {
  display: block;
  margin-top: 12px;
  font-size: 26px;
  line-height: 1.05;
}

.coupon-ticket small {
  display: block;
  margin-top: 10px;
  color: #64748b;
}

.poster-qr-box {
  padding: 10px;
  border-radius: 22px;
  background: rgba(255, 255, 255, 0.92);
  text-align: center;
}

.poster-qr-box img {
  width: 100px;
  height: 100px;
  display: block;
  margin: 0 auto;
  border-radius: 14px;
}

.poster-qr-box small {
  display: block;
  margin-top: 6px;
  color: #0f2747;
  font-size: 11px;
}

.poster-footer {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  margin-top: 12px;
  color: rgba(255, 255, 255, 0.74);
  font-size: 11px;
}

.poster-footer b {
  max-width: 120px;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  font-weight: 500;
}

.poster-actions,
.bottom-actions {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  margin-top: 12px;
}

.review-form {
  display: grid;
  gap: 12px;
}

.review-result {
  padding: 14px;
  border-radius: 18px;
  background: #f8fbff;
}

.review-result p {
  margin: 8px 0 0;
  color: #64748b;
}

.bottom-actions {
  position: sticky;
  bottom: 12px;
  z-index: 5;
  padding: 10px;
  border: 1px solid rgba(203, 213, 225, 0.82);
  border-radius: 22px;
  background: rgba(255, 255, 255, 0.9);
  box-shadow: 0 16px 44px rgba(15, 23, 42, 0.16);
  backdrop-filter: blur(14px);
}

.qr-wrap {
  text-align: center;
}

.qr-wrap img {
  width: 220px;
  height: 220px;
  border-radius: 18px;
}

.qr-wrap p {
  color: #64748b;
}

@media (max-width: 360px) {
  .summary-grid,
  .poster-actions,
  .bottom-actions,
  .pay-rescue-card,
  .poster-coupon-row,
  .pickup-strip {
    grid-template-columns: 1fr;
  }

  .pay-rescue-card {
    display: grid;
  }

  .pay-rescue-card :deep(.el-button) {
    width: 100%;
  }
}
</style>
