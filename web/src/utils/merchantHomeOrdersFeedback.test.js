import test from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'

const merchantHome = await readFile(new URL('../mobile/merchant/MerchantHome.vue', import.meta.url), 'utf8')

test('H5 home distinguishes loading from successful zero-valued order summaries', () => {
  assert.ok(merchantHome.includes("const orderSummaryStatus = ref('loading')"))
  assert.ok(merchantHome.includes('正在加载订单摘要…'))
  assert.match(merchantHome, /<van-grid v-else :column="2" :border="false">/)
  assert.ok(merchantHome.includes("orderSummaryStatus.value = 'ready'"))
  assert.match(merchantHome, /<van-list v-if="orderSummaryStatus === 'ready'" finished>/)
  assert.match(merchantHome, /<div v-if="pendingOrders.length === 0" class="empty-pending">/)
})

test('H5 home reports request failure instead of showing an empty result and exposes retry', () => {
  assert.ok(merchantHome.includes("orderSummaryStatus.value = 'error'"))
  assert.ok(merchantHome.includes('订单摘要加载失败，暂时无法确认统计数据。'))
  assert.ok(merchantHome.includes('订单加载失败，无法确认是否有待处理订单。'))
  assert.match(merchantHome, /@click="loadOrders">重试<\/van-button>/)
  assert.ok(merchantHome.includes("if (!orderList) throw new Error('订单接口未返回有效列表')"))
})
