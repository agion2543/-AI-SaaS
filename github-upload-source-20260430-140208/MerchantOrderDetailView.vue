<template>
  <div class="detail-stack">
    <section class="page-card printable">
      <div class="detail-header">
        <div>
          <el-button class="no-print" text @click="router.push('/merchant/orders')">返回订单列表</el-button>
          <h2 class="page-title">订单详情 / 小票</h2>
          <p class="detail-subtitle">{{ order.order_no || '-' }}</p>
        </div>
        <div class="header-actions no-print">
          <el-tag size="large" :type="statusType(order.status)">{{ statusLabel(order.status) }}</el-tag>
          <el-tag size="large" :type="refundType(order.refund_status)">{{ refundLabel(order.refund_status) }}</el-tag>
          <el-button @click="printOrder">打印小票</el-button>
          <el-button type="warning" plain :disabled="!canRefund(order)" @click="openRefund">售后退款</el-button>
        </div>
      </div>

      <div class="summary-grid">
        <div class="summary-item"><span>门店</span><strong>{{ order.store?.name || '-' }}</strong></div>
        <div class="summary-item"><span>顾客手机号</span><strong>{{ order.customer_phone || '-' }}</strong></div>
        <div class="summary-item"><span>订单原价</span><strong>{{ formatMoney(order.amount) }}</strong></div>
        <div class="summary-item"><span>优惠金额</span><strong>-{{ formatMoney(order.discount_amount) }}</strong></div>
        <div class="summary-item"><span>实付金额</span><strong>{{ formatMoney(order.total_amount) }}</strong></div>
        <div class="summary-item"><span>已退款</span><strong>{{ formatMoney(order.refunded_amount) }}</strong></div>
        <div class="summary-item"><span>可退金额</span><strong>{{ formatMoney(refundableAmount) }}</strong></div>
        <div class="summary-item"><span>支付时间</span><strong>{{ formatTime(order.paid_at) }}</strong></div>
      </div>
    </section>

    <section class="page-card no-print">
      <div class="section-head">
        <h2 class="page-title">订单进度</h2>
        <div class="action-row">
          <el-button v-if="order.status === 'received'" type="primary" @click="accept">接单</el-button>
          <el-button v-if="order.status === 'accepted'" type="success" @click="complete">完成订单</el-button>
          <el-button v-if="canClose(order)" type="danger" plain @click="close">关闭订单</el-button>
          <el-button @click="openNote">编辑内部备注</el-button>
        </div>
      </div>

      <el-timeline class="order-timeline">
        <el-timeline-item v-for="item in timeline" :key="item.status" :type="timelineType(item)" :timestamp="item.time">
          <strong>{{ item.title }}</strong>
          <p>{{ item.description }}</p>
        </el-timeline-item>
      </el-timeline>
    </section>

    <section class="page-card printable">
      <h2 class="page-title">商品明细</h2>
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
        <el-table-column label="单价" width="120"><template #default="{ row }">{{ formatMoney(row.price) }}</template></el-table-column>
        <el-table-column prop="quantity" label="数量" width="100" />
        <el-table-column label="小计" width="120"><template #default="{ row }">{{ formatMoney(row.line_amount) }}</template></el-table-column>
      </el-table>
    </section>

    <section class="grid-2 no-print">
      <div class="page-card">
        <h2 class="page-title">顾客备注</h2>
        <p class="note-copy">{{ order.customer_note || '顾客暂无备注。' }}</p>
      </div>
      <div class="page-card">
        <h2 class="page-title">商家内部备注</h2>
        <p class="note-copy">{{ order.merchant_note || '暂无内部备注。可记录顾客偏好、异常情况或履约提醒。' }}</p>
      </div>
    </section>

    <section class="page-card no-print">
      <h2 class="page-title">内部处理记录</h2>
      <el-timeline class="order-timeline">
        <el-timeline-item v-for="(log, index) in order.operation_logs || []" :key="index" type="primary" :timestamp="formatTime(log.time)">
          <strong>{{ log.text }}</strong>
          <p>{{ log.action }}</p>
        </el-timeline-item>
        <el-empty v-if="!(order.operation_logs || []).length" description="暂无处理记录" />
      </el-timeline>
    </section>

    <el-dialog v-model="noteDialogVisible" title="订单内部备注" width="520px">
      <el-input v-model="noteForm.merchant_note" type="textarea" :rows="4" maxlength="500" show-word-limit />
      <template #footer>
        <el-button @click="noteDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveNote">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="refundDialogVisible" title="售后退款" width="520px">
      <el-alert
        class="refund-alert"
        type="warning"
        show-icon
        :closable="false"
        title="当前为财务退款记录。真实退款需由商家收款账户或支付渠道执行。"
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
        <el-button type="primary" :loading="refundSaving" @click="submitRefund">确认退款</el-button>
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
  completeMerchantOrder,
  fetchMerchantOrderDetail,
  refundMerchantOrder,
  updateMerchantOrderNote
} from '../../api/modules'

const route = useRoute()
const router = useRouter()
const order = ref({})
const noteDialogVisible = ref(false)
const refundDialogVisible = ref(false)
const refundSaving = ref(false)
const noteForm = reactive({ merchant_note: '' })
const refundForm = reactive({ full: true, amount_yuan: 0, reason: '' })

const refundableAmount = computed(() => Math.max(Number(order.value.total_amount || 0) - Number(order.value.refunded_amount || 0), 0))
const formatMoney = (value) => `¥${(Number(value || 0) / 100).toFixed(2)}`
const formatTime = (value) => value ? String(value).replace('T', ' ').slice(0, 19) : '-'
const statusLabel = (status) => ({
  pending: '待支付',
  received: '待接单',
  accepted: '已接单',
  completed: '已完成',
  closed: '已关闭',
  failed: '支付失败'
}[status] || status || '-')
const statusType = (status) => ({
  pending: 'warning',
  received: 'primary',
  accepted: 'success',
  completed: 'success',
  closed: 'info',
  failed: 'danger'
}[status] || 'info')
const refundLabel = (status) => ({ none: '未退款', partial: '部分退款', full: '全额退款' }[status] || '未退款')
const refundType = (status) => ({ partial: 'warning', full: 'success' }[status] || 'info')
const canClose = (row) => ['received', 'accepted'].includes(row.status)
const canRefund = (row) => row?.order_type === 'store_order' && refundableAmount.value > 0 && ['received', 'accepted', 'completed', 'closed'].includes(row.status)

const timeline = computed(() => [
  { status: 'pending', title: '订单创建', description: '顾客已提交订单，等待支付。', time: formatTime(order.value.created_at) },
  { status: 'received', title: '支付完成', description: '支付成功，订单进入待接单。', time: formatTime(order.value.paid_at || order.value.updated_at) },
  { status: 'accepted', title: '商家接单', description: '商家确认接单，开始履约。', time: ['accepted', 'completed'].includes(order.value.status) ? formatTime(order.value.updated_at) : '-' },
  { status: 'completed', title: '订单完成', description: '服务或商品已完成交付。', time: order.value.status === 'completed' ? formatTime(order.value.updated_at) : '-' }
])

const timelineType = (item) => {
  const orderIndex = ['pending', 'received', 'accepted', 'completed'].indexOf(order.value.status)
  const itemIndex = ['pending', 'received', 'accepted', 'completed'].indexOf(item.status)
  return itemIndex >= 0 && itemIndex <= orderIndex ? 'success' : 'info'
}

const load = async () => {
  const res = await fetchMerchantOrderDetail(route.params.id)
  order.value = res.data.order || {}
}

const accept = async () => {
  await acceptMerchantOrder(order.value.id)
  ElMessage.success('订单已接单')
  load()
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
  await ElMessageBox.confirm('确认记录该笔退款吗？请确保真实支付渠道已同步处理。', '确认退款', { type: 'warning' })
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

const printOrder = () => window.print()

onMounted(load)
</script>

<style scoped>
.detail-stack { display: grid; gap: 20px; }
.detail-header, .section-head, .header-actions, .action-row { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.header-actions, .action-row { flex-wrap: wrap; align-items: center; justify-content: flex-start; }
.detail-subtitle, .note-copy, .order-timeline p { margin: 6px 0 0; color: var(--muted); line-height: 1.7; }
.summary-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: 14px; margin-top: 18px; }
.summary-item { padding: 16px; border: 1px solid #e5edf9; border-radius: 14px; background: #f8fbff; }
.summary-item span { display: block; color: var(--muted); font-size: 13px; }
.summary-item strong { display: block; margin-top: 8px; }
.items-table, .order-timeline { margin-top: 16px; }
.product-cell { display: flex; gap: 12px; align-items: center; }
.product-cell img { width: 54px; height: 54px; border-radius: 12px; object-fit: cover; background: #eef4fb; }
.product-cell p { margin: 4px 0 0; color: var(--muted); }
.grid-2 { display: grid; grid-template-columns: repeat(auto-fit, minmax(260px, 1fr)); gap: 20px; }
.refund-alert { margin-bottom: 16px; }
@media print {
  .no-print { display: none !important; }
  .detail-stack { display: block; }
  .page-card { box-shadow: none !important; border: none !important; page-break-inside: avoid; }
}
</style>
