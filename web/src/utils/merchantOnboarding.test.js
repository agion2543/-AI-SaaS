import test from 'node:test'
import assert from 'node:assert/strict'
import {
  isMerchantOnboardingRejected,
  isMerchantPending
} from './merchantOnboarding.js'

test('pending is the onboarding state; active and suspended merchants are not pending', () => {
  assert.equal(isMerchantPending({ status: 'pending' }), true)
  assert.equal(isMerchantPending({ status: 'active' }), false)
  assert.equal(isMerchantPending({ status: 'suspended' }), false)
  assert.equal(isMerchantPending({}), false)
})

test('rejected means pending plus a reason plus a review time', () => {
  assert.equal(isMerchantOnboardingRejected({ status: 'pending', review_reason: '资料不完整', reviewed_at: '2026-11-28T08:00:00.000Z' }), true)
  assert.equal(isMerchantOnboardingRejected({ status: 'pending', review_reason: '', reviewed_at: '2026-11-28T08:00:00.000Z' }), false)
  assert.equal(isMerchantOnboardingRejected({ status: 'pending', review_reason: '资料不完整', reviewed_at: null }), false)
  assert.equal(isMerchantOnboardingRejected({ status: 'pending' }), false)
  assert.equal(isMerchantOnboardingRejected({ status: 'active', review_reason: '资料不完整', reviewed_at: '2026-11-28T08:00:00.000Z' }), false)
})
