import test from 'node:test'
import assert from 'node:assert/strict'
import { buildMerchantStorePayload, isValidMerchantStorePhone } from './merchantStoreContract.js'

test('builds create and update payloads with the backend store DTO field names', () => {
  const payload = buildMerchantStorePayload({
    name: ' 合成门店 ',
    address: ' 合成地址 ',
    contact_phone: ' 13800138000 ',
    is_open: false,
    business_hours: ' 周一至周日 09:00-21:00 ',
    phone: 'legacy-phone',
    is_active: true,
    description: '旧版不支持的简介',
    notice: '旧版不支持的公告',
    image: '旧版不支持的图片'
  })

  assert.deepEqual(payload, {
    name: '合成门店',
    address: '合成地址',
    contact_phone: '13800138000',
    is_open: false,
    business_hours: '周一至周日 09:00-21:00'
  })
  assert.equal('phone' in payload, false)
  assert.equal('is_active' in payload, false)
  assert.equal('description' in payload, false)
  assert.equal('notice' in payload, false)
  assert.equal('image' in payload, false)
})

test('builds a complete required payload when toggling a store open or closed', () => {
  const store = { name: '合成门店', address: '合成地址', contact_phone: '13800138000', is_open: true, business_hours: '09:00-21:00' }
  assert.deepEqual(buildMerchantStorePayload(store, { is_open: false }), {
    name: '合成门店',
    address: '合成地址',
    contact_phone: '13800138000',
    is_open: false,
    business_hours: '09:00-21:00'
  })
})

test('defaults new stores to open when the DTO optional field is omitted from the form', () => {
  assert.equal(buildMerchantStorePayload({ name: '合成门店', contact_phone: '13800138000' }).is_open, true)
})

test('accepts only an 11-digit mainland mobile number for the required contact_phone', () => {
  for (const phone of ['13800138000', ' 13800138000 ']) {
    assert.equal(isValidMerchantStorePhone(phone), true)
  }
  for (const phone of ['', '1380013800', '23800138000', '1380013800a']) {
    assert.equal(isValidMerchantStorePhone(phone), false)
  }
})
