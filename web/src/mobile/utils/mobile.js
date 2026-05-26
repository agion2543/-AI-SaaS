/**
 * Mobile Utils - 移动端工具函数
 */

// ============ 本地存储 ============

/**
 * 存储购物车
 * @param {string} storeId - 门店 ID
 * @param {object} cart - 购物车对象
 */
export function saveCart(storeId, cart) {
  localStorage.setItem(`cart_${storeId}`, JSON.stringify(cart))
}

/**
 * 获取购物车
 * @param {string} storeId - 门店 ID
 * @returns {object} 购物车对象
 */
export function getCart(storeId) {
  try {
    const saved = localStorage.getItem(`cart_${storeId}`)
    return saved ? JSON.parse(saved) : {}
  } catch (e) {
    return {}
  }
}

/**
 * 清除购物车
 * @param {string} storeId - 门店 ID
 */
export function clearCart(storeId) {
  localStorage.removeItem(`cart_${storeId}`)
}

// ============ 支付相关 ============

/**
 * 判断是否为开发环境
 */
export const isDev = import.meta.env.DEV || true

/**
 * 模拟支付（开发阶段使用）
 * @param {string} orderNo - 订单号
 * @param {number} amount - 金额
 * @returns {Promise}
 */
export function mockPayment(orderNo, amount) {
  return new Promise((resolve, reject) => {
    setTimeout(() => {
      // 90% 成功率
      if (Math.random() > 0.1) {
        resolve({ success: true, orderNo, message: '支付成功' })
      } else {
        reject({ success: false, orderNo, message: '支付失败，请重试' })
      }
    }, 1500)
  })
}

/**
 * 微信 H5 支付
 * @param {object} orderInfo - 订单信息
 * @returns {Promise}
 */
export async function requestWechatH5Pay(orderInfo) {
  const { data } = await axios.post('/v1/payment/h5-pay', {
    order_no: orderInfo.orderNo,
    amount: orderInfo.amount,
    description: orderInfo.description,
    notify_url: `${window.location.origin}/api/payment/notify`
  })

  if (data.h5_url) {
    window.location.href = data.h5_url
  }

  return data
}

/**
 * 支付结果轮询
 * @param {string} orderNo - 订单号
 * @param {function} onSuccess - 成功回调
 * @param {function} onFail - 失败回调
 * @param {number} maxTime - 最大轮询时间（毫秒）
 */
export function pollPaymentStatus(orderNo, onSuccess, onFail, maxTime = 5 * 60 * 1000) {
  let elapsed = 0
  const interval = 2000

  const timer = setInterval(async () => {
    try {
      const { data } = await axios.get(`/v1/order/${orderNo}/status`)
      if (data.status === 'paid' || data.status === 'completed') {
        clearInterval(timer)
        onSuccess && onSuccess(data)
      } else if (data.status === 'failed' || data.status === 'cancelled') {
        clearInterval(timer)
        onFail && onFail(data)
      }
    } catch (e) {
      // 继续轮询
    }

    elapsed += interval
    if (elapsed >= maxTime) {
      clearInterval(timer)
      onFail && onFail({ message: '支付超时' })
    }
  }, interval)

  return () => clearInterval(timer)
}

// ============ 微信 SDK ============

/**
 * 获取微信 JS-SDK 配置
 * @returns {Promise}
 */
export async function getWechatConfig() {
  const { data } = await axios.get('/api/wechat/jsconfig', {
    params: { url: window.location.href.split('#')[0] }
  })
  return data
}

/**
 * 初始化微信 JS-SDK
 */
export async function initWechatJSSDK() {
  if (!isWechatBrowser()) {
    console.log('Not in WeChat browser, skip JS-SDK initialization')
    return
  }

  try {
    const config = await getWechatConfig()

    wx.config({
      debug: isDev,
      appId: config.appId,
      timestamp: config.timestamp,
      nonceStr: config.nonceStr,
      signature: config.signature,
      jsApiList: [
        'updateAppMessageShareData',
        'updateTimelineShareData',
        'scanQRCode',
        'chooseWXPay'
      ]
    })

    wx.ready(() => {
      console.log('WeChat JS-SDK initialized')
    })

    wx.error((res) => {
      console.error('WeChat JS-SDK error:', res)
    })
  } catch (e) {
    console.error('Failed to initialize WeChat JS-SDK:', e)
  }
}

/**
 * 判断是否在微信浏览器中
 */
export function isWechatBrowser() {
  const ua = navigator.userAgent.toLowerCase()
  return ua.indexOf('micromessenger') !== -1
}

/**
 * 分享到微信
 * @param {object} shareInfo - 分享信息
 */
export function shareToWechat(shareInfo) {
  if (!isWechatBrowser()) return

  wx.updateAppMessageShareData({
    title: shareInfo.title,
    desc: shareInfo.desc,
    link: shareInfo.link,
    imgUrl: shareInfo.imgUrl,
    success: () => {
      console.log('Share success')
    }
  })

  wx.updateTimelineShareData({
    title: shareInfo.title,
    link: shareInfo.link,
    imgUrl: shareInfo.imgUrl
  })
}

// ============ 工具函数 ============

/**
 * 格式化金额
 * @param {number|string} amount - 金额
 * @returns {string}
 */
export function formatPrice(amount) {
  return parseFloat(amount || 0).toFixed(2)
}

/**
 * 格式化时间
 * @param {string|Date} time - 时间
 * @param {string} format - 格式
 * @returns {string}
 */
export function formatTime(time, format = 'YYYY-MM-DD HH:mm') {
  if (!time) return '-'

  const date = new Date(time)

  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hours = String(date.getHours()).padStart(2, '0')
  const minutes = String(date.getMinutes()).padStart(2, '0')
  const seconds = String(date.getSeconds()).padStart(2, '0')

  return format
    .replace('YYYY', year)
    .replace('MM', month)
    .replace('DD', day)
    .replace('HH', hours)
    .replace('mm', minutes)
    .replace('ss', seconds)
}

/**
 * 防抖函数
 * @param {function} func - 要防抖的函数
 * @param {number} wait - 等待时间
 */
export function debounce(func, wait = 300) {
  let timeout
  return function executedFunction(...args) {
    const later = () => {
      clearTimeout(timeout)
      func(...args)
    }
    clearTimeout(timeout)
    timeout = setTimeout(later, wait)
  }
}

/**
 * 节流函数
 * @param {function} func - 要节流的函数
 * @param {number} limit - 限制时间
 */
export function throttle(func, limit = 300) {
  let inThrottle
  return function executedFunction(...args) {
    if (!inThrottle) {
      func(...args)
      inThrottle = true
      setTimeout(() => (inThrottle = false), limit)
    }
  }
}

/**
 * 复制文本到剪贴板
 * @param {string} text - 要复制的文本
 * @returns {Promise}
 */
export function copyToClipboard(text) {
  if (navigator.clipboard) {
    return navigator.clipboard.writeText(text)
  }

  // 降级方案
  const textarea = document.createElement('textarea')
  textarea.value = text
  textarea.style.position = 'fixed'
  textarea.style.opacity = '0'
  document.body.appendChild(textarea)
  textarea.select()
  document.execCommand('copy')
  document.body.removeChild(textarea)

  return Promise.resolve()
}

/**
 * 获取 URL 参数
 * @param {string} name - 参数名
 * @returns {string|null}
 */
export function getUrlParam(name) {
  const reg = new RegExp('(^|&)' + name + '=([^&]*)(&|$)')
  const r = window.location.search.substr(1).match(reg)
  if (r != null) {
    return decodeURIComponent(r[2])
  }
  return null
}

/**
 * 拨打电话
 * @param {string} phone - 电话号码
 */
export function makePhoneCall(phone) {
  window.location.href = `tel:${phone}`
}

/**
 * 复制订单号
 * @param {string} orderNo - 订单号
 */
export async function copyOrderNo(orderNo) {
  try {
    await copyToClipboard(orderNo)
    return true
  } catch (e) {
    return false
  }
}
