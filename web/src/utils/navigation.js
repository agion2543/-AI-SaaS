const normalizePath = (value = '/') => {
  const pathname = String(value || '/').split(/[?#]/, 1)[0]
  const withLeadingSlash = pathname.startsWith('/') ? pathname : `/${pathname}`
  return withLeadingSlash.replace(/\/+$/, '') || '/'
}

const isWithinRoute = (pathname, basePath) => (
  pathname === basePath || pathname.startsWith(`${basePath}/`)
)

export const getMerchantPcActivePath = (path) => {
  const pathname = normalizePath(path)

  if (isWithinRoute(pathname, '/merchant/orders') && pathname !== '/merchant/orders') {
    return '/merchant/orders'
  }

  if (/^\/merchant\/stores\/[^/]+\/products(?:\/.*)?$/.test(pathname)) {
    return '/merchant/stores'
  }

  return pathname
}

export const getMerchantMobileActiveTab = (path) => {
  const pathname = normalizePath(path)

  if (isWithinRoute(pathname, '/m/merchant/orders')) return 1
  if (
    isWithinRoute(pathname, '/m/merchant/products') ||
    isWithinRoute(pathname, '/m/merchant/stores') ||
    isWithinRoute(pathname, '/m/merchant/coupons') ||
    isWithinRoute(pathname, '/m/merchant/promotions') ||
    isWithinRoute(pathname, '/m/merchant/group-buy') ||
    isWithinRoute(pathname, '/m/merchant/groupbuy')
  ) return 2
  if (isWithinRoute(pathname, '/m/merchant/finance')) return 3

  // Secondary pages (AI, settings, and unclassified merchant pages) return to the Home tab.
  return 0
}
