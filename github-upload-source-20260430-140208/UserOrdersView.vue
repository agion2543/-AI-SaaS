<template>
  <div class="page-card">
    <div class="toolbar">
      <h2 class="page-title">我的订单</h2>
      <el-button @click="load">刷新</el-button>
    </div>
    <el-table :data="orders">
      <el-table-column prop="order_no" label="订单号" min-width="210" />
      <el-table-column prop="amount" label="金额(分)" width="120" />
      <el-table-column prop="payment_channel" label="支付渠道" width="120" />
      <el-table-column prop="status" label="状态" width="120" />
      <el-table-column label="操作" width="220">
        <template #default="{ row }">
          <el-button size="small" @click="view(row)">查看支付</el-button>
          <el-button v-if="row.status === 'pending'" size="small" type="danger" @click="cancel(row)">取消订单</el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { cancelUserOrder, fetchUserOrders } from '../../api/modules'

const router = useRouter()
const orders = ref([])
const PAYMENT_CACHE_KEY = 'user_payment_cache'

const load = async () => {
  const res = await fetchUserOrders()
  orders.value = res.data
}

const view = (row) => {
  const cache = JSON.parse(window.sessionStorage.getItem(PAYMENT_CACHE_KEY) || '{}')
  const payment = cache[row.order_no] || {}
  router.push({
    path: '/portal/pay',
    query: {
      orderNo: row.order_no,
      mode: payment.mode || (row.payment_channel === 'alipay' ? 'page' : ''),
      paymentUrl: payment.payment_url || '',
      qrCode: payment.qr_code || ''
    }
  })
}

const cancel = async (row) => {
  await cancelUserOrder(row.order_no)
  ElMessage.success('订单已取消')
  load()
}

onMounted(load)
</script>
