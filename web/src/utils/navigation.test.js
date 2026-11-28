import test from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import {
  getMerchantMobileActiveTab,
  getMerchantPcActivePath
} from './navigation.js'

const mobileRoutes = await readFile(new URL('../router/mobile.js', import.meta.url), 'utf8')
const pcRoutes = await readFile(new URL('../router/index.js', import.meta.url), 'utf8')
const mobileLayout = await readFile(new URL('../mobile/merchant/MerchantLayout.vue', import.meta.url), 'utf8')
const pcMerchantLayout = await readFile(new URL('../layout/MerchantLayout.vue', import.meta.url), 'utf8')
const merchantHome = await readFile(new URL('../mobile/merchant/MerchantHome.vue', import.meta.url), 'utf8')
const merchantAI = await readFile(new URL('../mobile/merchant/MerchantAI.vue', import.meta.url), 'utf8')
const merchantSettings = await readFile(new URL('../mobile/merchant/MerchantSettings.vue', import.meta.url), 'utf8')
const merchantProducts = await readFile(new URL('../mobile/merchant/MerchantProducts.vue', import.meta.url), 'utf8')

test('H5 uses the branded AI assistant name consistently and keeps the home entry prominent', () => {
  assert.match(mobileLayout, /<van-tabbar-item to="\/m\/merchant\/products" icon="shop-o">\s*商品\s*<\/van-tabbar-item>/)
  assert.doesNotMatch(mobileLayout, /to="\/m\/merchant\/ai"/)
  assert.ok(merchantHome.indexOf('class="ai-entry"') < merchantHome.indexOf('class="activity-intro"'))
  assert.ok(merchantHome.includes('商擎 AI 运营助手'))
  assert.ok(merchantHome.includes('AI 服务尚未接入'))
  assert.ok(merchantHome.includes("router.push('/m/merchant/ai')"))
  assert.ok(merchantHome.includes(`<van-grid-item icon="shop-o" text="商品" @click="$router.push('/m/merchant/products')" />`))
  assert.ok(merchantAI.includes('title="商擎 AI 运营助手（未接入）"'))
  assert.ok(merchantAI.includes('AI 服务尚未接入'))
  assert.ok(merchantSettings.includes('<li>商擎 AI 运营助手</li>'))
})

test('H5 home order summary and to-dos use the merchant-wide order scope', () => {
  assert.ok(merchantHome.includes('按商家全部门店当前读取的最多 20 笔订单记录汇总'))
  assert.match(merchantHome, /fetchMerchantOrders\(\{\s*status: '',\s*page: 1,\s*page_size: 20\s*\}\)/)
  assert.ok(merchantHome.includes("if (['pending', 'failed', 'payment_confirming', 'submitted', 'received'].includes(order.status))"))
})

test('H5 product swipe actions are discoverable only when business writes are available', () => {
  assert.match(merchantProducts, /<p v-if="canWrite && products\.length > 0" class="product-swipe-hint">/)
  assert.ok(merchantProducts.includes('左滑商品可编辑、上架/下架或删除'))
})

test('PC order and store-product details activate their parent menu', () => {
  assert.equal(getMerchantPcActivePath('/merchant/orders/ord-42'), '/merchant/orders')
  assert.equal(getMerchantPcActivePath('/merchant/orders/ord-42?from=dashboard'), '/merchant/orders')
  assert.equal(getMerchantPcActivePath('/merchant/stores/12/products'), '/merchant/stores')
  assert.equal(getMerchantPcActivePath('/merchant/stores/12/products?category=food'), '/merchant/stores')
  assert.equal(getMerchantPcActivePath('/merchant/group-buy'), '/merchant/group-buy')
})

test('H5 AI page stays under home while product management selects the third tab', () => {
  assert.equal(getMerchantMobileActiveTab('/m/merchant/home'), 0)
  assert.equal(getMerchantMobileActiveTab('/m/merchant/stores'), 0)
  assert.equal(getMerchantMobileActiveTab('/m/merchant/group-buy'), 0)
  assert.equal(getMerchantMobileActiveTab('/m/merchant/coupons'), 0)
  assert.equal(getMerchantMobileActiveTab('/m/merchant/products'), 2)
  assert.equal(getMerchantMobileActiveTab('/m/merchant/products?group=featured'), 2)
  assert.equal(getMerchantMobileActiveTab('/m/merchant/orders'), 1)
  assert.equal(getMerchantMobileActiveTab('/m/merchant/orders/42'), 1)
  assert.equal(getMerchantMobileActiveTab('/m/merchant/finance'), 1)
  assert.equal(getMerchantMobileActiveTab('/m/merchant/ai'), 0)
  assert.equal(getMerchantMobileActiveTab('/m/merchant/mine'), 3)
  assert.equal(getMerchantMobileActiveTab('/m/merchant/settings'), 3)
  assert.equal(getMerchantMobileActiveTab('/m/merchant/subscription'), 3)
})

test('affected PC and H5 routes remain declared', () => {
  for (const route of [
    "path: 'orders/:id'",
    "path: 'stores/:storeId/products'",
    "path: 'group-buy'"
  ]) {
    assert.ok(pcRoutes.includes(route), `Missing PC route declaration: ${route}`)
  }

  for (const route of [
    "path: 'orders/:id'",
    "path: 'products'",
    "path: 'mine'",
    "path: 'stores'",
    "path: 'finance'",
    "path: 'group-buy'",
    "path: 'coupons'",
    "path: 'ai'",
    "path: 'settings'"
  ]) {
    assert.ok(mobileRoutes.includes(route), `Missing H5 route declaration: ${route}`)
  }

  assert.ok(mobileRoutes.includes("path: 'mine', component: MerchantMine, name: 'MerchantMine', meta: { allowExpiredReadOnly: true }"))
  assert.ok(mobileRoutes.includes("path: 'subscription', component: MerchantSubscription, name: 'MerchantSubscription', meta: { allowSubscriptionLanding: true }"))
  // P3 fix: tickets (and PC announcements) stay reachable without a
  // subscription, so merchants who never subscribed or are paused can
  // still reach support and read announcements.
  assert.ok(mobileRoutes.includes("path: 'tickets', component: MerchantTickets, name: 'MerchantTickets', meta: { allowExpiredReadOnly: true, allowNoSubscription: true }"))
  assert.ok(pcRoutes.includes("path: 'announcements', component: MerchantAnnouncementsView, meta: { requiresMerchantAuth: true, allowExpiredReadOnly: true, allowNoSubscription: true }"))
  assert.ok(pcRoutes.includes("path: 'tickets', component: MerchantTicketsView, meta: { requiresMerchantAuth: true, allowExpiredReadOnly: true, allowNoSubscription: true }"))
  assert.ok(pcRoutes.includes('isMerchantRouteAllowedBySubscription(accessState, to.meta)'))
  assert.ok(mobileRoutes.includes('isMerchantRouteAllowedBySubscription(state, to.meta)'))
  // P3 fix follow-up: the PC merchant layout also blocked paused merchants
  // with a full-page pause lock instead of the route view, and disabled the
  // tickets/announcements menu entries. Both must exempt allowNoSubscription
  // routes, or the router guard fix alone would not make the pages usable.
  assert.ok(pcMerchantLayout.includes("subscriptionPaused && route.path !== '/merchant/subscription' && !route.meta.allowNoSubscription"))
  assert.ok(!pcMerchantLayout.includes('<el-sub-menu index="merchant-more" :disabled="subscriptionPaused">'))
  assert.ok(pcMerchantLayout.includes('<el-menu-item index="/merchant/announcements">{{ text.announcements }}</el-menu-item>'))
  assert.ok(pcMerchantLayout.includes('<el-menu-item index="/merchant/tickets">{{ text.tickets }}</el-menu-item>'))
})
