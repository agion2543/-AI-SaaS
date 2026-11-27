import test from 'node:test'
import assert from 'node:assert/strict'
import {
  ANNOUNCEMENT_STATUSES,
  ANNOUNCEMENT_TYPES,
  MAINTENANCE_STATUSES,
  announcementStatusLabel,
  announcementTypeLabel,
  announcementVisibleAt,
  maintenanceBlocksModule,
  maintenanceBlockText,
  maintenanceModulesLabel,
  maintenanceStatusLabel,
} from './maintenance.js'

test('announcement type and status labels cover the backend enums', () => {
  assert.deepEqual(
    ANNOUNCEMENT_TYPES.map((item) => item.value),
    ['system_update', 'promo', 'price_change'],
  )
  assert.equal(announcementTypeLabel('system_update'), '系统更新')
  assert.equal(announcementTypeLabel('promo'), '活动促销')
  assert.equal(announcementTypeLabel('price_change'), '价格变动')
  assert.deepEqual(
    ANNOUNCEMENT_STATUSES.map((item) => item.value),
    ['draft', 'published', 'archived'],
  )
  assert.equal(announcementStatusLabel('published'), '已发布')
  assert.deepEqual(
    MAINTENANCE_STATUSES.map((item) => item.value),
    ['draft', 'scheduled', 'active', 'completed', 'cancelled'],
  )
  assert.equal(maintenanceStatusLabel('active'), '进行中')
  assert.equal(maintenanceStatusLabel('unknown'), 'unknown')
})

test('announcement visibility matches the backend rule', () => {
  const now = new Date('2026-11-28T10:00:00+08:00')
  const base = { status: 'published', published_at: '2026-11-28T09:00:00+08:00' }
  assert.equal(announcementVisibleAt(base, now), true)
  assert.equal(announcementVisibleAt({ ...base, status: 'draft' }, now), false)
  assert.equal(announcementVisibleAt({ ...base, status: 'archived' }, now), false)
  assert.equal(announcementVisibleAt({ ...base, publish_at: '2026-11-28T11:00:00+08:00' }, now), false)
  assert.equal(announcementVisibleAt({ ...base, publish_at: '2026-11-28T09:30:00+08:00' }, now), true)
  assert.equal(announcementVisibleAt({ ...base, effective_from: '2026-11-28T11:00:00+08:00' }, now), false)
  assert.equal(announcementVisibleAt({ ...base, effective_to: '2026-11-28T09:00:00+08:00' }, now), false)
  assert.equal(announcementVisibleAt({ ...base, effective_to: '2026-11-29T09:00:00+08:00' }, now), true)
})

test('maintenance blocking matches the backend rule', () => {
  const now = new Date('2026-11-28T10:00:00+08:00')
  const active = {
    status: 'active',
    window_end: '2026-11-28T12:00:00+08:00',
    affected_modules: 'all',
  }
  assert.equal(maintenanceBlocksModule(active, 'ordering', now), true)
  assert.equal(maintenanceBlocksModule(active, 'payment', now), true)
  assert.equal(maintenanceBlocksModule({ ...active, status: 'scheduled' }, 'ordering', now), false)
  assert.equal(maintenanceBlocksModule({ ...active, status: 'completed' }, 'ordering', now), false)
  assert.equal(maintenanceBlocksModule({ ...active, window_end: '2026-11-28T09:00:00+08:00' }, 'ordering', now), false)
  assert.equal(maintenanceBlocksModule({ ...active, affected_modules: 'payment' }, 'ordering', now), false)
  assert.equal(maintenanceBlocksModule({ ...active, affected_modules: 'payment' }, 'payment', now), true)
  assert.equal(maintenanceBlocksModule({ ...active, affected_modules: 'ordering,payment' }, 'ordering', now), true)
  assert.equal(maintenanceBlocksModule(null, 'ordering', now), false)
})

test('maintenance module labels and block text stay honest', () => {
  assert.equal(maintenanceModulesLabel('all'), '全部模块')
  assert.equal(maintenanceModulesLabel('ordering,payment'), 'ordering、payment')
  assert.equal(maintenanceModulesLabel(''), '未注明')
  const text = maintenanceBlockText({
    window_end: '2026-11-28T12:00:00+08:00',
  })
  assert.match(text, /预计 .* 前恢复/)
  assert.match(text, /已下单的订单不受影响/)
})
