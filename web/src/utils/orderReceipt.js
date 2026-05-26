const escapeHtml = (value) => String(value ?? '')
  .replaceAll('&', '&amp;')
  .replaceAll('<', '&lt;')
  .replaceAll('>', '&gt;')
  .replaceAll('"', '&quot;')
  .replaceAll("'", '&#39;')

const formatMoney = (value) => `¥${(Number(value || 0) / 100).toFixed(2)}`

const formatTime = (value) => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '-'

const itemLineAmount = (row) => Number(row.line_amount ?? row.amount ?? (Number(row.price || 0) * Number(row.quantity || 0)))

const orderAmount = (row) => Number(row.paid_amount ?? row.total_amount ?? row.amount ?? 0)

const itemQuantity = (row) => (Array.isArray(row.items) ? row.items : []).reduce((sum, item) => sum + Number(item.quantity || 0), 0)

const pickupCode = (row) => {
  const text = String(row.order_no || row.id || '')
  return text ? text.slice(-4).toUpperCase() : '-'
}

const statusLabel = (status) => ({
  pending: '待支付',
  submitted: '已提交',
  payment_confirming: '待确认收款',
  preparing: '处理中',
  received: '待接单',
  accepted: '已接单',
  completed: '已完成',
  closed: '已关闭',
  failed: '支付失败'
}[status] || status || '-')

const refundLabel = (status) => ({
  none: '未退款',
  partial: '部分退款',
  full: '全额退款'
}[status] || '未退款')

export const printStoreOrderReceipt = (order) => {
  const current = order || {}
  const items = Array.isArray(current.items) ? current.items : []
  const logs = Array.isArray(current.operation_logs) ? current.operation_logs : []
  const appendLogs = logs.filter((log) => log.action === 'items_appended')
  const receiptNo = current.order_no || `ORDER-${current.id || ''}`
  const netPaidAmount = Math.max(orderAmount(current) - Number(current.refunded_amount || 0), 0)
  const paidAt = current.paid_at || current.updated_at || current.created_at
  const printWindow = window.open('', '_blank', 'width=420,height=720')

  if (!printWindow) return false

  const itemsHtml = items.length
    ? items.map((item) => `
      <tr>
        <td>
          <strong>${escapeHtml(item.name || '未命名商品')}</strong>
          ${item.description ? `<small>${escapeHtml(item.description)}</small>` : ''}
          <small>单价 ${formatMoney(item.price)}</small>
        </td>
        <td class="num">x${Number(item.quantity || 0)}</td>
        <td class="money">${formatMoney(itemLineAmount(item))}</td>
      </tr>
    `).join('')
    : '<tr><td colspan="3" class="empty">暂无商品明细</td></tr>'
  const appendHtml = appendLogs.length
    ? `
      <div class="divider"></div>
      <div class="append">
        <strong>追加记录：${appendLogs.length} 轮</strong>
        ${appendLogs.map((log, index) => `
          <div class="append-row">
            <b>第 ${index + 1} 轮</b>
            <span>${escapeHtml(log.text || '顾客追加商品')}</span>
            <small>${escapeHtml(formatTime(log.time))}</small>
          </div>
        `).join('')}
      </div>
    `
    : ''

  printWindow.document.open()
  printWindow.document.write(`
    <!doctype html>
    <html>
      <head>
        <meta charset="utf-8" />
        <title>订单小票 ${escapeHtml(receiptNo)}</title>
        <style>
          * { box-sizing: border-box; }
          body {
            margin: 0;
            padding: 10px;
            background: #fff;
            color: #111827;
            font-family: "Microsoft YaHei", "SimSun", Arial, sans-serif;
            font-size: 12px;
            line-height: 1.45;
          }
          .receipt { width: 76mm; max-width: 100%; margin: 0 auto; }
          .center { text-align: center; }
          h1 { margin: 0 0 6px; font-size: 18px; font-weight: 800; }
          .subtitle { margin-bottom: 8px; color: #374151; font-size: 12px; }
          .pickup {
            margin: 8px auto 6px;
            padding: 8px 10px;
            border: 2px solid #111827;
            border-radius: 10px;
            font-size: 24px;
            font-weight: 900;
            letter-spacing: 0.12em;
          }
          .divider { border-top: 1px dashed #9ca3af; margin: 8px 0; }
          .row { display: flex; justify-content: space-between; gap: 8px; margin: 3px 0; }
          .row span:first-child { color: #6b7280; white-space: nowrap; }
          .row strong,
          .row span:last-child { text-align: right; word-break: break-all; }
          table { width: 100%; border-collapse: collapse; margin-top: 6px; }
          th { color: #6b7280; font-weight: 600; border-bottom: 1px dashed #d1d5db; padding: 4px 0; text-align: left; }
          td { border-bottom: 1px dashed #e5e7eb; padding: 6px 0; vertical-align: top; }
          td small { display: block; margin-top: 2px; color: #6b7280; }
          .num { width: 36px; text-align: center; white-space: nowrap; }
          .money { width: 62px; text-align: right; white-space: nowrap; }
          .total { font-size: 16px; font-weight: 800; }
          .note {
            margin-top: 6px;
            padding: 7px;
            border: 1px dashed #cbd5e1;
            border-radius: 8px;
            background: #f8fafc;
            word-break: break-all;
          }
          .note strong { display: block; margin-bottom: 3px; }
          .note.important { border-color: #111827; background: #fff7ed; font-size: 13px; }
          .append {
            padding: 7px;
            border: 1px dashed #111827;
            border-radius: 8px;
            background: #eff6ff;
          }
          .append > strong { display: block; margin-bottom: 5px; }
          .append-row { padding: 5px 0; border-top: 1px dashed #cbd5e1; }
          .append-row:first-of-type { border-top: 0; }
          .append-row b,
          .append-row span,
          .append-row small { display: block; }
          .append-row span { margin-top: 2px; }
          .append-row small { margin-top: 2px; color: #6b7280; }
          .empty { color: #9ca3af; text-align: center; }
          .footer { margin-top: 10px; color: #6b7280; text-align: center; font-size: 11px; }
          @page { size: 80mm auto; margin: 4mm; }
          @media print { body { padding: 0; } .receipt { width: 72mm; } }
        </style>
      </head>
      <body>
        <main class="receipt">
          <div class="center">
            <h1>${escapeHtml(current.store?.name || '门店小票')}</h1>
            <div class="subtitle">出餐小票 / 取餐核对</div>
            <div class="pickup">${escapeHtml(pickupCode(current))}</div>
          </div>

          <div class="divider"></div>
          <div class="row"><span>订单号</span><strong>${escapeHtml(receiptNo)}</strong></div>
          <div class="row"><span>订单状态</span><strong>${escapeHtml(statusLabel(current.status))}</strong></div>
          <div class="row"><span>售后状态</span><strong>${escapeHtml(refundLabel(current.refund_status))}</strong></div>
          <div class="row"><span>商品总数</span><strong>${itemQuantity(current)} 件</strong></div>
          <div class="row"><span>顾客电话</span><strong>${escapeHtml(current.customer_phone || '-')}</strong></div>
          <div class="row"><span>下单时间</span><strong>${escapeHtml(formatTime(current.created_at))}</strong></div>
          <div class="row"><span>支付时间</span><strong>${escapeHtml(formatTime(paidAt))}</strong></div>
          <div class="row"><span>打印时间</span><strong>${escapeHtml(formatTime(new Date()))}</strong></div>

          <div class="divider"></div>
          <table>
            <thead>
              <tr>
                <th>商品</th>
                <th class="num">数量</th>
                <th class="money">小计</th>
              </tr>
            </thead>
            <tbody>${itemsHtml}</tbody>
          </table>
          ${appendHtml}

          <div class="divider"></div>
          <div class="row"><span>商品原价</span><strong>${formatMoney(current.amount)}</strong></div>
          <div class="row"><span>优惠抵扣</span><strong>-${formatMoney(current.discount_amount)}</strong></div>
          <div class="row total"><span>实付金额</span><strong>${formatMoney(orderAmount(current))}</strong></div>
          <div class="row"><span>已退款</span><strong>${formatMoney(current.refunded_amount)}</strong></div>
          <div class="row"><span>退款后实付</span><strong>${formatMoney(netPaidAmount)}</strong></div>

          <div class="divider"></div>
          <div class="note important">
            <strong>顾客备注</strong>
            ${escapeHtml(current.customer_note || '无')}
          </div>
          <div class="note">
            <strong>商家内部备注</strong>
            ${escapeHtml(current.merchant_note || '无')}
          </div>

          <div class="footer">请按小票内容核对出餐</div>
        </main>
        <script>
          window.onload = function () {
            window.focus();
            window.print();
          };
        <\/script>
      </body>
    </html>
  `)
  printWindow.document.close()

  return true
}
