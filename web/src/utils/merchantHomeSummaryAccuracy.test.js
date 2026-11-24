import test from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'

const merchantHome = await readFile(new URL('../mobile/merchant/MerchantHome.vue', import.meta.url), 'utf8')

test('H5 home formats API cent amounts as yuan for revenue and order cards', () => {
  assert.ok(merchantHome.includes("import { formatCents } from '@/utils/productPricing'"))
  assert.ok(merchantHome.includes('stats.value.todayRevenue = formatCents(todayRevenue)'))
  assert.ok(merchantHome.includes('stats.value.monthRevenue = formatCents(monthRevenue)'))
  assert.ok(merchantHome.includes('formatCents(order.total_amount ?? order.amount ?? 0)'))
  assert.ok(merchantHome.includes('收款按付款确认时间统计且未扣退款'))
})

test('H5 monthly order summary does not show zero values while loading or after failure', () => {
  assert.match(merchantHome, /本月订单与收款[\s\S]*?<div v-if="orderSummaryStatus === 'loading'"[\s\S]*?<div v-else-if="orderSummaryStatus === 'error'"[\s\S]*?<van-grid v-else :column="2" :gutter="10">/)
})

test('H5 group-buy metrics disclose the 100-store coverage limit and malformed store data is treated as unavailable', () => {
  assert.ok(merchantHome.includes('最多 100 家门店'))
  assert.ok(merchantHome.includes('门店超过 100 家时，首页不含其余门店'))
  assert.ok(merchantHome.includes('已读取门店累计跳转点击'))
  assert.ok(merchantHome.includes('storeListLoadFailed.value = true'))
})
