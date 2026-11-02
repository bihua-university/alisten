import { flushSync } from 'react-dom'

type ViewTransitionDocument = Document & {
  startViewTransition?: (callback: () => void) => { finished: Promise<void> }
}

function motionAllowed() {
  const level = document.body.classList
  if (level.contains('performance-off') || level.contains('performance-low') || level.contains('no-animations'))
    return false
  return !window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

/** True when the browser can push a view transition right now: supported, and motion is allowed. */
export function supportsViewTransition() {
  return typeof (document as ViewTransitionDocument).startViewTransition === 'function' && motionAllowed()
}

/** Applies a view or panel update inside a browser view transition, so the outgoing view can animate out. */
export function withViewTransition(update: () => void) {
  const doc = document as ViewTransitionDocument
  if (!doc.startViewTransition || !supportsViewTransition()) {
    update()
    return
  }
  doc.startViewTransition(() => {
    // The browser captures the new state when this callback returns, so React must commit synchronously.
    // eslint-disable-next-line react-dom/no-flush-sync -- required by the view-transition update callback
    flushSync(update)
  })
}
