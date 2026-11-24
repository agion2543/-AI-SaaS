export function createMerchantOrderRequestCoordinator() {
  let generation = 0
  let activeRequest = null

  function isCurrent(request) {
    return request?.generation === generation && activeRequest === request
  }

  function begin(page) {
    if (activeRequest?.generation === generation) {
      return { request: activeRequest, isNew: false }
    }

    const request = { generation, page, promise: null }
    activeRequest = request
    return { request, isNew: true }
  }

  function invalidate() {
    generation += 1
    activeRequest = null
  }

  function finish(request) {
    if (!isCurrent(request)) return false
    activeRequest = null
    return true
  }

  return { begin, invalidate, isCurrent, finish }
}
