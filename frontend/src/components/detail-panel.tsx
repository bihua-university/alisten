import type { ReactNode } from 'react'
import { useEffect, useRef, useState } from 'react'
import { useStore } from '@/hooks/use-store'
import { performanceStore } from '@/stores/performance-store'
import { supportsViewTransition } from '@/utils/view-transition'

export type DetailPanelView = 'chat' | 'queue' | 'history'

export function DetailPanel({ panel, children }: {
  panel: DetailPanelView | null
  children: (panel: DetailPanelView) => ReactNode
}) {
  const [renderedPanel, setRenderedPanel] = useState(panel)
  const element = useRef<HTMLElement>(null)
  const previous = useRef<DetailPanelView | null>(null)
  const performance = useStore(performanceStore, s => s)
  const closing = panel === null

  // Keep the outgoing content until its exit animation finishes.
  if (panel && panel !== renderedPanel)
    setRenderedPanel(panel)

  // A swap between two open views animates; the first open is covered by the panel's own animation.
  const swapped = !closing && previous.current !== null && previous.current !== renderedPanel
  previous.current = closing ? null : renderedPanel
  // The view transition pushes both views itself, so the CSS fallback only runs where it is unavailable.
  const sliding = swapped && !supportsViewTransition()

  useEffect(() => {
    if (!closing || !renderedPanel || !element.current)
      return
    const duration = Number.parseFloat(getComputedStyle(element.current).animationDuration) * 1000
    // Also release content if the animation is cancelled or hidden by a breakpoint.
    const timer = window.setTimeout(() => setRenderedPanel(null), duration ? duration + 50 : 0)
    return () => window.clearTimeout(timer)
  }, [closing, renderedPanel, performance])

  if (!renderedPanel)
    return null

  return (
    <aside
      ref={element}
      className={`detail-panel${closing ? ' detail-panel-closing' : ''}`}
      aria-hidden={closing || undefined}
      {...(closing ? { inert: '' } : {})}
      onAnimationEnd={(event) => {
        if (event.target === event.currentTarget && closing)
          setRenderedPanel(null)
      }}
    >
      <div
        className={`detail-panel-content${sliding ? ' detail-panel-swap' : ''}`}
        key={renderedPanel}
      >
        {children(renderedPanel)}
      </div>
    </aside>
  )
}
