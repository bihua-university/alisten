import type { ViewRef } from 'nsh'
import type { MusicSearchSelection } from '@/types'
import { createMotion, effect, Overlay, signal, Text, View } from 'nsh'
import { Input } from 'nsh/base'
import { useStore } from '@/hooks/use-store'
import { historyStore } from '@/stores/history-store'
import { websocketStore } from '@/stores/websocket-store'
import { brand, font, foreground, inputBorder, muted, subtle } from '@/theme'
import { Icon } from './icon'
import { IconButton } from './ui'

export function HeaderSearch(props: {
  query: string
  onQueryChange: (query: string) => void
  initialSearch?: MusicSearchSelection
  open: boolean
  onOpenChange: (open: boolean) => void
  onSearch: (selection: Omit<MusicSearchSelection, 'id'>) => void
}) {
  let anchor: ViewRef | undefined
  const connected = useStore(websocketStore, state => state.connectionStatus === 'connected')
  const history = useStore(historyStore, state => state.searchHistory)
  const enabled = useStore(historyStore, state => state.enableSearchHistory)
  const popup = createMotion()
  const anchorWidth = signal(0)
  effect(() => {
    if (props.open) {
      void popup.play({
        duration: 200,
        keyframes: [
          { opacity: 0, translateY: -8, scale: 0.96 },
          { opacity: 1, translateY: 0, scale: 1 },
        ],
      })
    }
  })
  const source = () => props.initialSearch?.source ?? 'netease'
  const mode = () => props.initialSearch?.mode ?? 'all'
  const submit = () => {
    if (connected() && props.query.trim())
      props.onSearch({ keyword: props.query.trim(), source: source(), mode: mode() })
  }
  return (
    <View ref={anchor} onLayout={({ bounds }) => anchorWidth.set(bounds.size.width)} style={{ flex: 1, minWidth: 0 }}>
      <View style={{ height: 48, flexDirection: 'row', alignItems: 'center', gap: 12, paddingHorizontal: 16, borderWidth: 1, borderColor: props.open ? brand : inputBorder, borderRadius: 24, backgroundColor: '#1f1f1f' }}>
        <Icon name="Search" size={22} />
        <Input
          accessibilityLabel="搜索音乐"
          value={props.query}
          onChange={({ value }) => props.onQueryChange(value)}
          onFocus={() => props.onOpenChange(true)}
          onSubmit={submit}
          returnKeyType="search"
          placeholder="想播放什么？"
          style={{ flex: 1, color: foreground, placeholderColor: subtle, fontSize: font().fontSize }}
        />
        <If when={props.query}>
          <IconButton icon="X" label="清空顶部搜索" onPress={() => props.onSearch({ keyword: '', source: source(), mode: mode() })} />
        </If>
      </View>
      <If when={props.open && enabled() && history().length > 0}>
        <Overlay
          anchor={anchor}
          placement="bottom"
          align="start"
          offset={12}
          onPressOutside={() => props.onOpenChange(false)}
          onCancel={() => props.onOpenChange(false)}
          style={[{ width: anchorWidth(), padding: 16, gap: 6, borderWidth: 1, borderColor: '#333337', borderRadius: 12, backgroundColor: '#242427', boxShadow: '0 12px 32px rgba(0,0,0,0.24)', transformOrigin: 'top center' }, popup.style]}
        >
          <Text style={[font(-2), { color: muted }]}>最近搜索</Text>
          <Each of={history().slice(0, 4)} key={item => `${item.query}:${item.platform}:${item.searchMode}`}>
            {item => (
              <View
                role="button"
                class="cursor-pointer"
                accessibilityLabel={item().query}
                onPress={() => {
                  props.onQueryChange(item().query)
                  props.onSearch({
                    keyword: item().query,
                    source: item().platform === 'qq' ? 'qq' : 'netease',
                    mode: item().searchMode === 'all' ? 'all' : item().searchMode === 'playlist' ? 'playlist' : 'song',
                  })
                }}
                style={{ flexDirection: 'row', alignItems: 'center', gap: 10, padding: 8, borderRadius: 8 }}
              >
                <Icon name="Clock3" size={16} />
                <Text style={[font(-1), { color: foreground, flex: 1, numberOfLines: 1 }]}>{item().query}</Text>
              </View>
            )}
          </Each>
        </Overlay>
      </If>
    </View>
  )
}
