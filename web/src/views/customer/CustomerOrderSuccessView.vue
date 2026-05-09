<template>
  <main class="success-page">
    <section class="success-card">
      <div class="success-icon">✓</div>
      <span class="eyebrow">ORDER SUCCESS</span>
      <h1>{{ t.title }}</h1>
      <p>{{ t.subtitle }}</p>

      <el-skeleton v-if="loading" :rows="6" animated />

      <template v-else>
        <div v-if="order.order_no" class="summary-grid">
          <div class="summary-item full">
            <span>{{ t.orderNo }}</span>
            <strong>{{ order.order_no }}</strong>
          </div>
          <div class="summary-item">
            <span>{{ t.store }}</span>
            <strong>{{ order.store?.name || '-' }}</strong>
          </div>
          <div class="summary-item">
            <span>{{ t.status }}</span>
            <strong>{{ statusLabel(order.status) }}</strong>
          </div>
          <div class="summary-item">
            <span>{{ t.payAmount }}</span>
            <strong>{{ yuan(order.total_amount || order.amount) }}</strong>
          </div>
          <div v-if="Number(order.discount_amount || 0) > 0" class="summary-item">
            <span>{{ t.discount }}</span>
            <strong>-{{ yuan(order.discount_amount) }}</strong>
          </div>
        </div>

        <div v-if="items.length" class="items-card">
          <div class="card-title">{{ t.items }}</div>
          <div v-for="item in items" :key="item.product_id || item.name" class="item-row">
            <span>{{ item.name }} x {{ item.quantity }}</span>
            <strong>{{ yuan(item.line_amount || item.price * item.quantity) }}</strong>
          </div>
        </div>

        <div v-if="order.customer_note" class="note-card">
          <span>{{ t.note }}</span>
          <p>{{ order.customer_note }}</p>
        </div>

        <div class="action-row">
          <el-button type="primary" size="large" @click="goStore">{{ t.backStore }}</el-button>
          <el-button v-if="phoneLink" size="large" @click="callStore">{{ t.callStore }}</el-button>
        </div>
      </template>
    </section>
  </main>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { fetchCustomerOrder } from '../../api/modules'

const t = {
  title: '\u4e0b\u5355\u6210\u529f',
  subtitle: '\u5546\u5bb6\u6536\u5230\u8ba2\u5355\u540e\u4f1a\u5c3d\u5feb\u63a5\u5355\u5904\u7406\uff0c\u8bf7\u7559\u610f\u95e8\u5e97\u8054\u7cfb\u3002',
  orderNo: '\u8ba2\u5355\u53f7',
  store: '\u95e8\u5e97',
  status: '\u72b6\u6001',
  payAmount: '\u5b9e\u4ed8\u91d1\u989d',
  discount: '\u4f18\u60e0',
  items: '\u5546\u54c1\u660e\u7ec6',
  note: '\u987e\u5ba2\u5907\u6ce8',
  backStore: '\u8fd4\u56de\u95e8\u5e97',
  callStore: '\u8054\u7cfb\u95e8\u5e97',
  loadFailed: '\u8ba2\u5355\u52a0\u8f7d\u5931\u8d25'
}

const route = useRoute()
const router = useRouter()
const loading = ref(true)
const order = ref({})

const phoneLink = computed(() => order.value.store?.contact_phone || order.value.merchant?.contact_phone || '')
const items = computed(() => {
  const raw = order.value.items
  if (Array.isArray(raw)) return raw
  if (typeof raw === 'string') {
    try {
      return JSON.parse(raw)
    } catch {
      return []
    }
  }
  return []
})

const formatYuan = (value) => (Number(value || 0) / 100).toFixed(2)
const yuan = (value) => `\u00a5${formatYuan(value)}`
const statusLabel = (status) => ({
  pending: '\u5f85\u652f\u4ed8',
  paid: '\u5df2\u652f\u4ed8',
  received: '\u5f85\u5546\u5bb6\u63a5\u5355',
  accepted: '\u5546\u5bb6\u5df2\u63a5\u5355',
  completed: '\u5df2\u5b8c\u6210',
  closed: '\u5df2\u5173\u95ed',
  canceled: '\u5df2\u53d6\u6d88'
}[status] || status || '-')

const load = async () => {
  const orderNo = route.query.orderNo
  if (!orderNo) {
    loading.value = false
    return
  }
  try {
    const res = await fetchCustomerOrder(orderNo)
    order.value = res.data.order || {}
  } catch (err) {
    ElMessage.error(err.response?.data?.message || t.loadFailed)
  } finally {
    loading.value = false
  }
}

const goStore = () => {
  const storeId = order.value.store_id || order.value.store?.id
  if (storeId) router.push(`/customer/store/${storeId}`)
}

const callStore = () => {
  if (phoneLink.value) window.location.href = `tel:${phoneLink.value}`
}

onMounted(load)
</script>

<style scoped>
.success-page {
  min-height: 100vh;
  padding: 18px 0;
  display: grid;
  place-items: start center;
  color: #172033;
  background:
    radial-gradient(circle at 88% 0%, rgba(34, 197, 94, 0.26), transparent 32%),
    linear-gradient(180deg, #f7fff8 0%, #eef7ff 100%);
}

.success-card {
  width: min(430px, calc(100vw - 24px));
  padding: 24px;
  border-radius: 30px;
  background: rgba(255, 255, 255, 0.95);
  box-shadow: 0 24px 70px rgba(15, 23, 42, 0.14);
}

.success-icon {
  width: 58px;
  height: 58px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  color: #fff;
  background: linear-gradient(135deg, #16a34a, #22c55e);
  font-size: 30px;
  font-weight: 900;
}

.eyebrow {
  display: block;
  margin-top: 16px;
  color: #16a34a;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.12em;
}

h1 {
  margin: 8px 0;
  font-size: 34px;
}

p {
  margin: 0;
  color: #64748b;
  line-height: 1.7;
}

.summary-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  margin-top: 22px;
}

.summary-item,
.items-card,
.note-card {
  padding: 14px;
  border-radius: 18px;
  border: 1px solid #bbf7d0;
  background: #f0fdf4;
}

.summary-item.full {
  grid-column: 1 / -1;
}

.summary-item span,
.note-card span {
  display: block;
  color: #15803d;
  font-size: 12px;
}

.summary-item strong {
  display: block;
  margin-top: 7px;
  word-break: break-all;
}

.items-card,
.note-card {
  margin-top: 14px;
}

.card-title {
  font-weight: 900;
  margin-bottom: 8px;
}

.item-row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding-top: 8px;
}

.action-row {
  margin-top: 18px;
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}

@media (max-width: 420px) {
  .summary-grid,
  .action-row {
    grid-template-columns: 1fr;
  }
}
</style>
