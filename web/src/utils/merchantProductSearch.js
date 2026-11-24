export function createDebouncedAction(action, delay = 300, timers = globalThis) {
  let timer = null

  function cancel() {
    if (timer !== null) {
      timers.clearTimeout(timer)
      timer = null
    }
  }

  return {
    schedule(...args) {
      cancel()
      timer = timers.setTimeout(() => {
        timer = null
        action(...args)
      }, delay)
    },
    cancel,
    flush(...args) {
      if (timer === null) return
      cancel()
      return action(...args)
    }
  }
}

export function createRequestVersionGuard() {
  let version = 0

  return {
    current: () => version,
    next: () => ++version,
    isCurrent: candidate => candidate === version
  }
}
