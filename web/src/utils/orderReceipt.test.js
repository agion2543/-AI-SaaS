import assert from 'node:assert/strict'
import test from 'node:test'
import { printStoreOrderReceipt } from './orderReceipt.js'

const renderReceipt = (order) => {
  let html = ''
  const printWindow = {
    document: {
      open() {},
      write(value) { html = value },
      close() {}
    }
  }
  assert.equal(printStoreOrderReceipt(order, printWindow), true)
  return html
}

test('table order receipt prefers and HTML-escapes the immutable table label', () => {
  const html = renderReceipt({
    order_no: 'SYNTHETIC-TABLE-ORDER',
    table_no: 8,
    table_label: '<包间 & A>',
    store: { name: '合成门店' },
    items: []
  })
  assert.match(html, /桌号 &lt;包间 &amp; A&gt;/)
  assert.doesNotMatch(html, /桌号 8号/)
  assert.match(html, /class="table-number"/)
})
test('table order receipt falls back to the numbered table label for legacy orders', () => {
  const html = renderReceipt({ order_no: 'SYNTHETIC-OLD-TABLE', table_no: 8, items: [] })
  assert.match(html, /桌号 8号桌/)
})

test('non-table order receipt does not show a table number', () => {
  const html = renderReceipt({
    order_no: 'SYNTHETIC-NON-TABLE-ORDER',
    table_no: 0,
    items: []
  })
  assert.doesNotMatch(html, /桌号/)
})
