import test from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'

const merchantOrdersPage = await readFile(new URL('../mobile/merchant/MerchantOrders.vue', import.meta.url), 'utf8')

test('H5 order list distinguishes request failures from empty results and supports retry', () => {
  assert.ok(merchantOrdersPage.includes('v-model:error="orderLoadError"'))
  assert.ok(merchantOrdersPage.includes('error-text="加载失败，点击重试"'))
  assert.ok(merchantOrdersPage.includes('orderLoadError.value = true'))
  assert.ok(merchantOrdersPage.includes('orderLoadError.value = false'))
  assert.ok(merchantOrdersPage.includes("if (orderLoadError.value) return '订单加载失败'"))
})

test('H5 order empty state names the selected status filter', () => {
  assert.ok(merchantOrdersPage.includes('暂无${selectedTab}订单'))
  assert.ok(merchantOrdersPage.includes("return selectedTab ? `暂无${selectedTab}订单` : '暂无订单'"))
  assert.ok(merchantOrdersPage.includes('visibleOrders.length === 0 && !loading && (finished || orderLoadError)'))
  assert.ok(merchantOrdersPage.includes('<van-empty :description="emptyOrdersDescription" />'))
})

test('H5 order list uses a request page snapshot and blocks stale responses', () => {
  assert.ok(merchantOrdersPage.includes('orderRequestCoordinator.begin(page.value)'))
  assert.ok(merchantOrdersPage.includes('page: request.page'))
  assert.ok(merchantOrdersPage.includes('if (request.page === 1)'))
  assert.ok(merchantOrdersPage.includes('if (!orderRequestCoordinator.isCurrent(request)) return'))
  assert.ok(merchantOrdersPage.includes('orderRequestCoordinator.invalidate()'))
  assert.ok(merchantOrdersPage.includes('if (orderRequestCoordinator.finish(request))'))
})
