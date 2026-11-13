import test from 'node:test'
import assert from 'node:assert/strict'
import {
  merchantFromSubscriptionPayload,
  mobileMerchantSubscriptionState,
  mobileSubscriptionActions
} from './mobileMerchantSubscription.js'

const now = Date.parse('2030-01-01T00:00:00.000Z')
const active = {
  merchant: {
    status: 'active',
    subscription_status: 'active',
    subscription_expire_at: '2030-02-01T00:00:00.000Z'
  },
  plans: [{ id: 1, name: '月付', price_cents: 19900, duration_days: 30 }],
  active: true,
  platform_payment: { platform_subscription_note: '合成配置' }
}

test('parses the actual subscription endpoint envelope, not a legacy end_date field', () => {
  assert.equal(merchantFromSubscriptionPayload(active), active.merchant)
  assert.equal(mobileMerchantSubscriptionState(active, now), 'active')
  assert.equal(mobileMerchantSubscriptionState({ merchant: { end_date: '2030-02-01T00:00:00Z' } }, now), 'unknown')
})

test('recognizes ordinary expiry from the actual primary or legacy expiry field', () => {
  assert.equal(mobileMerchantSubscriptionState({
    merchant: { status: 'active', subscription_status: 'active', subscription_expire_at: '2029-12-31T23:59:59.000Z' }
  }, now), 'expired')
  assert.equal(mobileMerchantSubscriptionState({
    merchant: { status: 'active', subscription_status: 'active', subscription_expired_at: '2029-12-31T23:59:59.000Z' }
  }, now), 'expired')
})

test('safety pause takes precedence over a future expiry and exposes only recovery actions', () => {
  const paused = {
    merchant: {
      status: 'active',
      subscription_status: 'paused',
      subscription_expire_at: '2030-02-01T00:00:00.000Z',
      subscription_pause_reason: '合成暂停原因'
    }
  }
  assert.equal(mobileMerchantSubscriptionState(paused, now), 'paused')
  assert.deepEqual(mobileSubscriptionActions('paused'), {
    canReadBusiness: false,
    canWriteBusiness: false,
    canRenew: false,
    canRecheck: true,
    showSupportPath: true
  })
  assert.equal(mobileMerchantSubscriptionState({
    merchant: { status: 'suspended', subscription_status: 'active', subscription_expire_at: '2030-02-01T00:00:00Z' }
  }, now), 'paused')
})

test('unknown or malformed responses fail closed and offer a recheck, not business actions', () => {
  for (const payload of [null, {}, { merchant: null }, { merchant: { subscription_status: 'active' } }, { merchant: { subscription_status: 'inactive' } }]) {
    assert.equal(mobileMerchantSubscriptionState(payload, now), 'unknown')
  }
  assert.deepEqual(mobileSubscriptionActions('unknown'), {
    canReadBusiness: false,
    canWriteBusiness: false,
    canRenew: false,
    canRecheck: true,
    showSupportPath: false
  })
})

test('active and expired states keep read access but only active state exposes business writes', () => {
  assert.deepEqual(mobileSubscriptionActions('active'), {
    canReadBusiness: true,
    canWriteBusiness: true,
    canRenew: true,
    canRecheck: false,
    showSupportPath: false
  })
  assert.deepEqual(mobileSubscriptionActions('expired'), {
    canReadBusiness: true,
    canWriteBusiness: false,
    canRenew: true,
    canRecheck: false,
    showSupportPath: false
  })
})
