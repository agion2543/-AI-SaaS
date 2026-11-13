const storageKey = (orderNo) => `customer_order_access_${String(orderNo || '')}`

export const saveCustomerOrderAccessToken = (orderNo, token) => {
  if (!orderNo || !token) return ''
  localStorage.setItem(storageKey(orderNo), String(token))
  return String(token)
}

export const getCustomerOrderAccessToken = (orderNo) => {
  if (!orderNo) return ''
  return localStorage.getItem(storageKey(orderNo)) || ''
}

export const stripCustomerOrderAccessTokenFromUrl = () => {
  if (typeof window === 'undefined') return false
  const url = new URL(window.location.href)
  let changed = false

  if (url.searchParams.has('access_token')) {
    url.searchParams.delete('access_token')
    changed = true
  }

  const fragment = url.hash.replace(/^#/, '')
  if (fragment && !fragment.startsWith('/')) {
    const fragmentParams = new URLSearchParams(fragment)
    if (fragmentParams.has('access_token')) {
      fragmentParams.delete('access_token')
      url.hash = fragmentParams.toString()
      changed = true
    }
  }

  if (changed) {
    window.history.replaceState(window.history.state, '', `${url.pathname}${url.search}${url.hash}`)
  }
  return changed
}

export const captureCustomerOrderAccessToken = (orderNo, route) => {
  const queryToken = Array.isArray(route?.query?.access_token)
    ? route.query.access_token[0]
    : route?.query?.access_token
  const hashToken = new URLSearchParams(String(route?.hash || '').replace(/^#/, '')).get('access_token')
  const token = String(queryToken || hashToken || '')
  return token ? saveCustomerOrderAccessToken(orderNo, token) : getCustomerOrderAccessToken(orderNo)
}

export const customerOrderAccessPath = (orderNo, token) => {
  const path = `/customer/orders/${encodeURIComponent(orderNo)}`
  return token ? `${path}#access_token=${encodeURIComponent(token)}` : path
}

export const mobileCustomerOrderAccessPath = (orderNo, token) => {
  const path = `/m/order/${encodeURIComponent(orderNo)}`
  return token ? `${path}?access_token=${encodeURIComponent(token)}` : path
}

export const customerOrderAccessLink = (orderNo, token, baseUrl = window.location.origin) => {
  return `${baseUrl}${customerOrderAccessPath(orderNo, token)}`
}
