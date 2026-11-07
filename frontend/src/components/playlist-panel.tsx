import { createAnimatedValue, Easing, effect, signal, Text, View } from 'nsh'
import { Button } from 'nsh/base'
import { brand, focusRing, font, foreground, motionDuration, muted } from '@/theme'
import HistoryPage from './history-page'
import QueuePage from './queue-page'
import { IconButton } from './ui'

export function PlaylistPanel(props: {
  active: 'queue' | 'history'
  onSelect: (view: 'queue' | 'history') => void
  onClose: () => void
  onSearch: () => void
}) {
  const width = signal(0)
  const queueWidth = signal(0)
  const queueTextWidth = signal(0)
  const historyTextWidth = signal(0)
  const position = createAnimatedValue(props.active === 'history' ? 1 : 0)
  let initial = true
  effect(() => {
    const target = props.active === 'history' ? 1 : 0
    const duration = motionDuration(280)
    if (initial || !duration) {
      position.set(target)
      initial = false
    } else {
      // One native value keeps the indicator and both pages in sync, even
      // when the user reverses direction before the transition finishes.
      void position.timing(target, { duration, easing: Easing.out(Easing.cubic) })
    }
    return () => position.stop()
  })

  return (
    <View style={{ flex: 1, minHeight: 0, minWidth: 0 }}>
      <View style={{ flexDirection: 'row', alignItems: 'center', paddingHorizontal: 16, paddingVertical: 12 }}>
        <View role="tablist" accessibilityLabel="播放列表" style={{ flexDirection: 'row', gap: 20 }}>
          <Button
            role="tab"
            accessibilityLabel="队列"
            selected={props.active === 'queue'}
            onPress={() => props.onSelect('queue')}
            onLayout={({ bounds }) => queueWidth.set(bounds.size.width)}
            focusStyle={focusRing}
            style={{ minHeight: 40, paddingHorizontal: 14, paddingVertical: 10, borderRadius: 8, justifyContent: 'center', cursor: 'pointer' }}
          >
            <Text onLayout={({ bounds }) => queueTextWidth.set(bounds.size.width)} style={[font(), { color: props.active === 'queue' ? foreground : muted, fontWeight: '600' }]}>队列</Text>
          </Button>
          <Button
            role="tab"
            accessibilityLabel="最近播放"
            selected={props.active === 'history'}
            onPress={() => props.onSelect('history')}
            focusStyle={focusRing}
            style={{ minHeight: 40, paddingHorizontal: 14, paddingVertical: 10, borderRadius: 8, justifyContent: 'center', cursor: 'pointer' }}
          >
            <Text onLayout={({ bounds }) => historyTextWidth.set(bounds.size.width)} style={[font(), { color: props.active === 'history' ? foreground : muted, fontWeight: '600' }]}>最近播放</Text>
          </Button>
          <View style={{
            position: 'absolute',
            left: 0,
            bottom: 0,
            width: queueTextWidth() || 1,
            height: 3,
            borderRadius: 2,
            backgroundColor: brand,
            opacity: queueTextWidth() && historyTextWidth() ? 1 : 0,
            transformOrigin: 'left center',
            transform: [
              { translateX: position.mul(queueWidth() + 20).add(14) },
              { scaleX: position.mul(historyTextWidth() / (queueTextWidth() || 1) - 1).add(1) },
            ],
            pointerEvents: 'none',
          }}
          />
        </View>
        <View style={{ flex: 1 }} />
        <IconButton icon="X" label="关闭侧栏" onPress={props.onClose} />
      </View>
      <View
        onLayout={({ bounds }) => width.set(bounds.size.width)}
        style={{ flex: 1, minHeight: 0, overflow: 'hidden' }}
      >
        <View
          accessibilityElementsHidden={props.active !== 'queue'}
          style={{ position: 'absolute', top: 0, right: 0, bottom: 0, left: 0, minHeight: 0, transform: [{ translateX: position.mul(-width()) }], pointerEvents: props.active === 'queue' ? 'auto' : 'none' }}
        >
          <QueuePage compact onSearch={props.onSearch} />
        </View>
        <View
          accessibilityElementsHidden={props.active !== 'history'}
          style={{ position: 'absolute', top: 0, right: 0, bottom: 0, left: 0, minHeight: 0, transform: [{ translateX: position.sub(1).mul(-width()) }], pointerEvents: props.active === 'history' ? 'auto' : 'none' }}
        >
          <HistoryPage compact />
        </View>
      </View>
    </View>
  )
}
