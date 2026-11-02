interface ModalEntry {
  close: () => void
}

const stack: ModalEntry[] = []
let awaitingBack = false
let bound = false

function pushMarker() {
  window.history.pushState({ ...window.history.state, alistenModal: true }, '', window.location.href)
}

function unbindIfIdle() {
  if (!stack.length && !awaitingBack && bound) {
    window.removeEventListener('popstate', onPop)
    bound = false
  }
}

function onPop() {
  if (awaitingBack) {
    awaitingBack = false
    if (stack.length)
      pushMarker()
    unbindIfIdle()
    return
  }
  const top = stack.pop()
  top?.close()
  if (stack.length)
    pushMarker()
  unbindIfIdle()
}

/** One history marker for the whole dialog stack; Back closes only the top. */
export function registerModalHistory(close: () => void) {
  if (!bound) {
    window.addEventListener('popstate', onPop)
    bound = true
  }
  if (!stack.length && !awaitingBack)
    pushMarker()
  const entry = { close }
  stack.push(entry)
  return () => {
    const index = stack.indexOf(entry)
    if (index < 0)
      return
    stack.splice(index, 1)
    if (!stack.length && !awaitingBack && window.history.state?.alistenModal) {
      awaitingBack = true
      window.history.back()
    }
    unbindIfIdle()
  }
}
