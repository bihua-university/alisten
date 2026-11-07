import type { MusicSearchSelection } from '@/types'
import { cleanup, createAnimatedValue, Easing, effect, Scroll, signal, untracked, View } from 'nsh'
import { motionDuration } from '@/theme'
import { ChatPanel } from './chat-panel'
import ListenPage from './listen-page'
import QueuePage from './queue-page'
import SearchPage from './search-page'

export type MobileTabView = 'listen' | 'search' | 'queue' | 'chat'

const mobileTabViews: MobileTabView[] = ['listen', 'search', 'queue', 'chat']

export function MobileTabPages(props: {
  active: MobileTabView
  onSearch: () => void
  initialQuery?: string
  initialSearch?: MusicSearchSelection
  onSubmitSearch: (selection: Omit<MusicSearchSelection, 'id'>) => void
}) {
  const initial = mobileTabViews.indexOf(props.active)
  const position = createAnimatedValue(initial)
  const width = signal(0)
  const pages = signal([initial])
  let transition = 0
  let first = true

  effect(() => {
    const target = mobileTabViews.indexOf(props.active)
    const duration = motionDuration(300)
    const revision = ++transition
    if (first || !duration) {
      first = false
      position.set(target)
      pages.set([target])
      return
    }

    // Keep every crossed page present until the native animation settles.
    // Retarget the same value so rapid taps can reverse without restarting.
    const retained = untracked(pages)
    const start = Math.min(target, ...retained)
    const end = Math.max(target, ...retained)
    pages.set(mobileTabViews.map((_, index) => index).slice(start, end + 1))
    void position.timing(target, { duration, easing: Easing.out(Easing.cubic) }).then(() => {
      if (revision === transition)
        pages.set([target])
    })
  })
  cleanup(() => {
    transition++
    position.stop()
  })

  return (
    <View onLayout={({ bounds }) => width.set(bounds.size.width)} style={{ flex: 1, minHeight: 0, minWidth: 0, overflow: 'hidden' }}>
      <Each of={pages()} key={index => index}>
        {index => (
          <View
            accessibilityElementsHidden={mobileTabViews[index()] !== props.active}
            style={{ position: 'absolute', top: 0, right: 0, bottom: 0, left: 0, minHeight: 0, minWidth: 0, transform: [{ translateX: position.sub(index()).mul(-width()) }], pointerEvents: mobileTabViews[index()] === props.active ? 'auto' : 'none' }}
          >
            <If when={index() === 0}><ListenPage /></If>
            <If when={index() === 1}>
              <Scroll keyboardShouldPersistTaps="handled" style={{ flex: 1, minHeight: 0 }} contentStyle={{ flexGrow: 1 }}>
                <SearchPage initialQuery={props.initialQuery} initialSearch={props.initialSearch} onSearch={props.onSubmitSearch} />
              </Scroll>
            </If>
            <If when={index() === 2}><QueuePage onSearch={props.onSearch} /></If>
            <If when={index() === 3}><ChatPanel /></If>
          </View>
        )}
      </Each>
    </View>
  )
}
