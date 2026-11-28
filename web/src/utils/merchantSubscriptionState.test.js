import test from 'node:test'
import assert from 'node:assert/strict'
import {
  isMerchantSubscriptionActive,
  isMerchantRouteAllowedBySubscription,
  merchantSubscriptionAccessState,
  merchantSubscriptionDisplayState
} from './merchantSubscriptionState.js'
import {
  hasValidMerchantSubscriptionSnapshot,
  isLegacyMerchantSubscriptionOrderWithoutSnapshot,
  merchantSubscriptionSnapshotLabel,
  merchantSubscriptionSnapshotPrice
} from './merchantSubscriptionOrderSnapshot.js'
import {
  providerPaidSubscriptionOrders,
  providerPaidSubscriptionRevenueCents
} from './subscriptionRevenue.js'

const now = Date.parse('2030-01-01T00:00:00.000Z')
const future = '2030-01-31T00:00:00.000Z'

test('paused merchant subscription is not presented as active despite future expiry', () => {
  const merchant = {
    subscription_status: 'paused',
    subscription_expire_at: future,
    subscription_pause_reason: '合成测试暂停'
  }
  assert.equal(isMerchantSubscriptionActive(merchant, now), false)
  assert.deepEqual(merchantSubscriptionDisplayState(merchant, now), {
    title: '已暂停',
    hint: '合成测试暂停'
  })
})

test('only active subscriptions with a future expiry are presented as available', () => {
  assert.equal(isMerchantSubscriptionActive({ subscription_status: 'active', subscription_expire_at: future }, now), true)
  assert.equal(isMerchantSubscriptionActive({ subscription_status: 'active', subscription_expire_at: '2029-12-31T23:59:59.000Z' }, now), false)
  assert.equal(isMerchantSubscriptionActive({ subscription_status: 'active' }, now), false)
  assert.equal(isMerchantSubscriptionActive({ subscription_status: 'inactive', subscription_expire_at: future }, now), false)
})

test('subscription access distinguishes ordinary expiry from explicit pause and unknown states', () => {
  assert.equal(merchantSubscriptionAccessState({ subscription_status: 'active', subscription_expire_at: future }, now), 'active')
  assert.equal(merchantSubscriptionAccessState({ subscription_status: 'active', subscription_expire_at: '2029-12-31T23:59:59.000Z' }, now), 'expired')
  assert.equal(merchantSubscriptionAccessState({ subscription_status: 'paused', subscription_expire_at: future }, now), 'paused')
  assert.equal(merchantSubscriptionAccessState({ subscription_status: 'inactive', subscription_expire_at: '2029-12-31T23:59:59.000Z' }, now), 'unavailable')
  assert.equal(merchantSubscriptionAccessState({ subscription_status: 'active' }, now), 'unavailable')
})

test('platform subscription view uses snapshot name, duration and price, never mutable relation fallback', () => {
  const order = {
    merchant_plan_name_snapshot: '下单时月付',
    merchant_plan_duration_days_snapshot: 30,
    merchant_plan_price_cents_snapshot: 19900,
    amount: 19900,
    total_amount: 19900,
    merchant_plan: { name: '后来改名年付', duration_days: 365, price_cents: 99900 }
  }
  assert.equal(hasValidMerchantSubscriptionSnapshot(order), true)
  assert.equal(merchantSubscriptionSnapshotLabel(order), '下单时月付 · 30 天')
  assert.equal(merchantSubscriptionSnapshotPrice(order), 19900)
})

test('legacy pending subscription without snapshots is flagged for manual review', () => {
  const order = {
    amount: 19900,
    total_amount: 19900,
    merchant_plan: { name: '当前可变套餐', duration_days: 90, price_cents: 19900 }
  }
  assert.equal(hasValidMerchantSubscriptionSnapshot(order), false)
  assert.equal(merchantSubscriptionSnapshotLabel(order), '订单快照缺失或无效（需人工复核）')
  assert.equal(merchantSubscriptionSnapshotPrice(order), null)
})

test('manual-review draft is offered only when every immutable snapshot field is absent', () => {
  assert.equal(isLegacyMerchantSubscriptionOrderWithoutSnapshot({
    merchant_plan_name_snapshot: null,
    merchant_plan_price_cents_snapshot: null,
    merchant_plan_duration_days_snapshot: null
  }), true)
  assert.equal(isLegacyMerchantSubscriptionOrderWithoutSnapshot({}), true)
  assert.equal(isLegacyMerchantSubscriptionOrderWithoutSnapshot({
    merchant_plan_name_snapshot: null,
    merchant_plan_price_cents_snapshot: 19900,
    merchant_plan_duration_days_snapshot: null
  }), false)
  assert.equal(isLegacyMerchantSubscriptionOrderWithoutSnapshot({
    merchant_plan_name_snapshot: '月付',
    merchant_plan_price_cents_snapshot: 19900,
    merchant_plan_duration_days_snapshot: 30
  }), false)
})

test('a manually verified subscription remains outside provider-paid count and revenue', () => {
  const onlyManualRecord = [{
    order_type: 'merchant_subscription',
    status: 'manually_verified',
    total_amount: 19900,
    manual_review: { amount_cents: 19900 },
    manual_review_audit: { reviewed_at: '2026-11-13T08:00:00Z' }
  }]
  assert.deepEqual(providerPaidSubscriptionOrders(onlyManualRecord), [])
  assert.equal(providerPaidSubscriptionRevenueCents(onlyManualRecord), 0)

  const existingPaidOrder = { order_type: 'merchant_subscription', status: 'paid', total_amount: 34900 }
  assert.deepEqual(providerPaidSubscriptionOrders([...onlyManualRecord, existingPaidOrder]), [existingPaidOrder])
  assert.equal(providerPaidSubscriptionRevenueCents([...onlyManualRecord, existingPaidOrder]), 34900)
})

test('tickets and announcements stay reachable without a subscription (P3 fix)', () => {
  const noSubscription = { allowNoSubscription: true }
  const readOnly = { allowExpiredReadOnly: true }
  const plain = {}

  // Never subscribed (PC 'unavailable', mobile 'unknown') and paused both
  // reach tickets/announcements, but not ordinary pages like orders.
  assert.equal(isMerchantRouteAllowedBySubscription('unavailable', noSubscription), true)
  assert.equal(isMerchantRouteAllowedBySubscription('unknown', noSubscription), true)
  assert.equal(isMerchantRouteAllowedBySubscription('paused', noSubscription), true)
  assert.equal(isMerchantRouteAllowedBySubscription('unavailable', plain), false)
  assert.equal(isMerchantRouteAllowedBySubscription('unknown', plain), false)
  assert.equal(isMerchantRouteAllowedBySubscription('paused', plain), false)

  // Existing behaviour is unchanged: active always passes, expired only
  // with allowExpiredReadOnly.
  assert.equal(isMerchantRouteAllowedBySubscription('active', plain), true)
  assert.equal(isMerchantRouteAllowedBySubscription('expired', readOnly), true)
  assert.equal(isMerchantRouteAllowedBySubscription('expired', plain), false)
})
