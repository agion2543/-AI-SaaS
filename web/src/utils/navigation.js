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
  if (isWithinRoute(pathname, '/merchant/stores') && pathname !== '/merchant/stores') {
    return '/merchant/stores'
  }

  return pathname
}

export const getMerchantMobileActiveTab = (path) => {
  const pathname = normalizePath(path)

  if (
    isWithinRoute(pathname, '/m/merchant/orders') ||
    isWithinRoute(pathname, '/m/merchant/finance')
  ) return 1

  if (isWithinRoute(pathname, '/m/merchant/products')) return 2

  if (
    isWithinRoute(pathname, '/m/merchant/home') ||
    isWithinRoute(pathname, '/m/merchant/stores') ||
    isWithinRoute(pathname, '/m/merchant/group-buy') ||
    isWithinRoute(pathname, '/m/merchant/coupons') ||
    isWithinRoute(pathname, '/m/merchant/ai')
  ) return 0

  if (
    isWithinRoute(pathname, '/m/merchant/mine') ||
    isWithinRoute(pathname, '/m/merchant/settings') ||
    isWithinRoute(pathname, '/m/merchant/subscription')
  ) return 3

  // Keep a visible selection for future secondary merchant pages.
  return 0
}
