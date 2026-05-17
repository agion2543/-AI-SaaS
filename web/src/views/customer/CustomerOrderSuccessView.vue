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
          <div class="summary-grid">
            <div>
              <span>门店</span>
              <strong>{{ order.store?.name || '-' }}</strong>
            </div>
            <div>
              <span>实付金额</span>
              <strong>{{ yuan(order.total_amount || order.amount) }}</strong>
            </div>
            <div>
              <span>优惠抵扣</span>
              <strong>-{{ yuan(order.discount_amount) }}</strong>
            </div>
            <div>
              <span>退款金额</span>
              <strong>{{ yuan(order.refunded_amount) }}</strong>
            </div>
          </div>
          <div class="refresh-line">
            <span>最近刷新：{{ lastLoadedAt || '-' }}</span>
            <span v-if="canRetry">页面会自动刷新支付状态</span>
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
          <el-button type="primary" size="large" @click="backStore">返回门店</el-button>
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
  retryCustomerOrderPayment,
  reviewCustomerOrder
} from '../../api/modules'

const route = useRoute()
const router = useRouter()
const loading = ref(false)
const retrying = ref(false)
const sharing = ref(false)
const reviewing = ref(false)
const order = ref({})
const lastLoadedAt = ref('')
const previousStatus = ref('')
const payMode = ref('page')
const qrDialogVisible = ref(false)
const qrImageUrl = ref('')
const shareCampaign = ref({})
const reviewForm = ref({ rating: 5, review: '' })
let pollTimer = null

const orderNo = computed(() => route.params.orderNo || route.query.orderNo || '')
const status = computed(() => order.value.status || 'pending')
const items = computed(() => Array.isArray(order.value.items) ? order.value.items : [])
const storeId = computed(() => order.value.store_id || order.value.store?.id)
const contactPhone = computed(() => order.value.store?.contact_phone || order.value.merchant?.contact_phone || '')
const isRefunded = computed(() => Number(order.value.refunded_amount || 0) > 0)
const canRetry = computed(() => ['pending', 'failed'].includes(status.value))
const canRequestShare = computed(() => ['received', 'accepted', 'completed'].includes(status.value))
const canSharePoster = computed(() => canRequestShare.value && Boolean(shareCampaign.value.share_code))
const canReview = computed(() => ['accepted', 'completed'].includes(status.value))

const statusMap = {
  pending: { title: '等待支付', tip: '订单已经创建，请在有效时间内完成支付。', icon: '¥', tag: 'warning' },
  failed: { title: '支付未完成', tip: '支付失败或已中断，可以重新发起支付。', icon: '!', tag: 'danger' },
  received: { title: '下单成功，等待接单', tip: '商家已收到订单，会尽快确认接单。', icon: '✓', tag: 'primary' },
  accepted: { title: '商家已接单', tip: '订单正在处理，请留意门店通知。', icon: '单', tag: 'success' },
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
      title: '完成支付后订单才会发送给商家',
      tip: '如果刚才支付页面被关闭，可以点击继续支付；支付成功后本页会自动进入待接单状态。'
    }
  }
  if (status.value === 'received') {
    return {
      label: '下一步',
      title: '等待商家确认接单',
      tip: '商家接单后会开始制作或履约，你也可以通过本页联系门店确认。'
    }
  }
  if (status.value === 'accepted') {
    return {
      label: '当前进度',
      title: '商家正在处理订单',
      tip: '请留意门店通知，到店或取餐时可出示本页面核对订单。'
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
  const rank = { pending: 1, failed: 1, received: 2, accepted: 3, completed: 4, closed: 4 }
  const current = rank[status.value] || 1
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
      title: '支付完成',
      text: '支付成功后订单进入待接单。',
      time: order.value.paid_at ? formatTime(order.value.paid_at) : '-',
      done: current >= 2,
      active: current === 2
    },
    {
      key: 'accepted',
      title: '商家接单',
      text: '商家确认接单并开始处理。',
      time: logTime('accepted'),
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
    lastLoadedAt.value = new Date().toLocaleTimeString('zh-CN', { hour12: false })
    if (!canRetry.value) {
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
      startPolling()
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

const startPolling = () => {
  stopPolling()
  pollTimer = window.setInterval(load, 3000)
}
const stopPolling = () => {
  if (pollTimer) window.clearInterval(pollTimer)
  pollTimer = null
}
const backStore = () => {
  if (storeId.value) router.push(`/customer/store/${storeId.value}`)
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
const yuan = (value) => `¥${(Number(value || 0) / 100).toFixed(2)}`
const formatTime = (value) => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '-'

onMounted(() => {
  load()
  if (orderNo.value) startPolling()
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

.next-action span {
  color: #2563eb;
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

.next-action p {
  margin: 6px 0 0;
  color: #64748b;
  line-height: 1.55;
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

.summary-grid strong {
  display: block;
  margin-top: 6px;
  font-size: 18px;
}

.refresh-line {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  margin-top: 10px;
  color: #94a3b8;
  font-size: 12px;
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
  .poster-coupon-row {
    grid-template-columns: 1fr;
  }
}
</style>
