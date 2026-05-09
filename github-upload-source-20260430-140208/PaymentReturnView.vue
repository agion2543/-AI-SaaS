<template>
  <div class="page-card return-card">
    <div class="eyebrow">{{ text.eyebrow }}</div>
    <h1>{{ text.title }}</h1>
    <div class="state" :class="statusClass">{{ statusText }}</div>
    <p>{{ text.orderNo }}{{ orderNo }}</p>
    <div class="actions">
      <el-button type="primary" @click="check">{{ text.refresh }}</el-button>
      <el-button @click="router.push('/portal/orders')">{{ text.viewOrders }}</el-button>
      <el-button @click="router.push('/portal/profile')">{{ text.viewMembership }}</el-button>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { fetchUserOrderDetail } from '../../api/modules'

const text = {
  eyebrow: '\u652f\u4ed8\u7ed3\u679c',
  title: '\u8ba2\u5355\u652f\u4ed8\u72b6\u6001',
  orderNo: '\u8ba2\u5355\u53f7\uff1a',
  refresh: '\u5237\u65b0\u72b6\u6001',
  viewOrders: '\u67e5\u770b\u8ba2\u5355',
  viewMembership: '\u67e5\u770b\u4f1a\u5458',
  paid: '\u652f\u4ed8\u6210\u529f\uff0c\u4f1a\u5458\u6743\u76ca\u5df2\u751f\u6548',
  closed: '\u8ba2\u5355\u5df2\u53d6\u6d88',
  failed: '\u652f\u4ed8\u5931\u8d25',
  pending: '\u8ba2\u5355\u4ecd\u5728\u7b49\u5f85\u652f\u4ed8'
}

const route = useRoute()
const router = useRouter()
const status = ref('pending')
const orderNo = computed(() => route.query.orderNo || '')

const statusText = computed(() => {
  if (status.value === 'paid') return text.paid
  if (status.value === 'closed') return text.closed
  if (status.value === 'failed') return text.failed
  return text.pending
})

const statusClass = computed(() => ({
  paid: status.value === 'paid',
  failed: status.value === 'failed' || status.value === 'closed'
}))

const check = async () => {
  if (!orderNo.value) return
  const res = await fetchUserOrderDetail(orderNo.value)
  status.value = res.data.status
}

onMounted(check)
</script>

<style scoped>
.return-card {
  max-width: 720px;
  margin: 0 auto;
  text-align: center;
}

.state {
  font-size: 28px;
  font-weight: 800;
  margin: 18px 0;
}

.state.paid {
  color: var(--success);
}

.state.failed {
  color: #dc2626;
}

.actions {
  display: flex;
  justify-content: center;
  gap: 12px;
  margin-top: 20px;
  flex-wrap: wrap;
}

.eyebrow {
  color: var(--brand);
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0.15em;
  margin-bottom: 8px;
}
</style>
