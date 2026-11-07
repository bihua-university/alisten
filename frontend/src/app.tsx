import type { ViewRef } from 'nsh'
import type { IconName } from './components/icon'
import type { MobileTabView } from './components/mobile-tab-pages'
import type { MusicSearchSelection, RoomInfo } from '@/types'
import { cleanup, createMotion, ErrorBoundary, LayoutAnimation, Overlay, Scroll, signal, Text, View, windowSize } from 'nsh'
import { AudioEngine } from './components/audio-engine'
import { ChatPanel } from './components/chat-panel'
import { HeaderSearch } from './components/header-search'
import HistoryPage from './components/history-page'
import { Icon } from './components/icon'
import ListenPage from './components/listen-page'
import { MobileTabPages } from './components/mobile-tab-pages'
import { Notifications, SystemPrompts } from './components/notifications'
import { PlayerBar } from './components/player-bar'
import { PlaylistPanel } from './components/playlist-panel'
import QueuePage from './components/queue-page'
import RoomBrowser from './components/room-browser'
import SearchPage from './components/search-page'
import SettingsPage from './components/settings-page'
import { EmptyState, IconButton, Modal, TextButton } from './components/ui'
import { useStore } from './hooks/use-store'
import { chatStore } from './stores/chat-store'
import { notificationStore } from './stores/notification-store'
import { playerStore } from './stores/player-store'
import { roomStore } from './stores/room-store'
import { searchStore } from './stores/search-store'
import { userSettingsStore } from './stores/user-settings-store'
import { websocketStore } from './stores/websocket-store'
import { brand, compact, font, foreground, motionDuration, muted, success, surface } from './theme'
import { applyEnvironment } from './utils/environment'
import { getLocation, onHostEvent, reloadApp, shareRoom } from './utils/runtime'
import { saveLastJoinedRoom, saveRoomPassword } from './utils/user'

type AppView = 'listen' | 'search' | 'queue' | 'chat' | 'history' | 'rooms' | 'settings'
type DetailPanelView = 'queue' | 'chat' | 'history'

const navigation: { view: AppView, label: string, icon: IconName }[] = [
  { view: 'listen', label: '正在聆听', icon: 'Headphones' },
  { view: 'queue', label: '播放列表', icon: 'ListMusic' },
  { view: 'chat', label: '房间聊天', icon: 'MessageCircle' },
  { view: 'history', label: '播放历史', icon: 'History' },
  { view: 'rooms', label: '发现房间', icon: 'House' },
]

const mobileNavigation: { view: MobileTabView, label: string, icon: IconName }[] = [
  { view: 'listen', label: '聆听', icon: 'House' },
  { view: 'search', label: '搜索', icon: 'Search' },
  { view: 'queue', label: '待播', icon: 'ListMusic' },
  { view: 'chat', label: '聊天', icon: 'MessageCircle' },
]

export function App() {
  const viewHistory = signal<{ views: AppView[], index: number }>({ views: ['rooms'], index: 0 })
  const view = () => viewHistory().views[viewHistory().index]
  const joined = signal(false)
  const roomName = useStore(roomStore, state => state.name)
  const panel = signal<DetailPanelView | null>(null)
  const helpOpen = signal(false)
  const searchOpen = signal(false)
  const searchRequest = signal<MusicSearchSelection>()
  const searchQuery = signal('')
  const pageMotion = createMotion()
  const panelMotion = createMotion()
  const panelContentMotion = createMotion()
  let requestedPanel: DetailPanelView | null = null
  let panelTransition = 0
  cleanup(() => {
    panelTransition++
    panelContentMotion.stop()
  })
  const changePanel = (next: (current: DetailPanelView | null) => DetailPanelView | null) => {
    const target = next(requestedPanel)
    if (target === requestedPanel)
      return
    requestedPanel = target
    const transition = ++panelTransition
    const current = panel()
    panelContentMotion.reset()
    if (target === current)
      return

    // Content switches keep the panel shell and the lyrics layout still.
    // Queue/history share their own synchronized indicator/page transition.
    if (current && target) {
      const duration = motionDuration(220)
      if ((current === 'chat') === (target === 'chat') || !duration) {
        panel.set(target)
        return
      }
      void (async () => {
        await panelContentMotion.play({
          duration: duration * 0.3,
          easing: 'easeIn',
          keyframes: { opacity: [1, 0] },
        })
        if (transition !== panelTransition)
          return
        panel.set(target)
        await panelContentMotion.play({
          duration: duration * 0.7,
          easing: 'easeOut',
          keyframes: { opacity: [0, 1] },
        })
      })()
      return
    }

    LayoutAnimation.configureNext({
      duration: motionDuration(240),
      create: { type: 'easeOut', property: 'opacity' },
      update: { type: 'easeInEaseOut' },
      delete: { type: 'easeIn', property: 'opacity' },
    })
    panel.set(target)
    if (target) {
      void panelMotion.play({
        duration: motionDuration(240),
        keyframes: [
          { opacity: 0, translateX: 16 },
          { opacity: 1, translateX: 0 },
        ],
      })
    }
  }

  const isMobileTab = (value: AppView): value is MobileTabView => mobileNavigation.some(item => item.view === value)
  const animatePageChange = (previous: AppView, next: AppView) => {
    pageMotion.reset()
    if (compact() && isMobileTab(previous) && isMobileTab(next))
      return
    void pageMotion.play({ duration: motionDuration(180), keyframes: { opacity: [0, 1] } })
  }
  const setView = (nextView: AppView) => {
    searchOpen.set(false)
    const previous = viewHistory()
    if (previous.views[previous.index] === nextView)
      return
    const views = [...previous.views.slice(0, previous.index + 1), nextView]
    viewHistory.set({ views, index: views.length - 1 })
    animatePageChange(previous.views[previous.index], nextView)
  }
  const stepView = (direction: -1 | 1) => {
    searchOpen.set(false)
    const previous = view()
    viewHistory.update(previous => ({
      ...previous,
      index: Math.max(0, Math.min(previous.views.length - 1, previous.index + direction)),
    }))
    if (previous !== view())
      animatePageChange(previous, view())
  }
  const submitSearch = (selection: Omit<MusicSearchSelection, 'id'>) => {
    if (!selection.label)
      searchQuery.set(selection.keyword)
    searchRequest.update(previous => ({ ...selection, id: (previous?.id || 0) + 1 }))
    searchStore.clearSearchResults()
    searchOpen.set(false)
    setView('search')
  }
  const onQueue = () => {
    if (compact()) {
      setView('queue')
    } else if (view() === 'queue') {
      setView('listen')
      changePanel(current => current === 'queue' ? null : current)
    } else {
      changePanel(current => current === 'queue' ? null : 'queue')
    }
  }
  const onChat = () => {
    if (compact()) {
      setView('chat')
    } else if (view() === 'chat') {
      setView('listen')
      changePanel(current => current === 'chat' ? null : current)
    } else {
      changePanel(current => current === 'chat' ? null : 'chat')
    }
  }
  const join = async (room: RoomInfo, password?: string) => {
    websocketStore.disconnect()
    playerStore.resetRoom()
    chatStore.reset()
    searchStore.clearSearchResults()
    roomStore.updateRoomInfo(room)
    roomStore.setCurrentPassword(password)
    try {
      await websocketStore.connectAndWait()
    } catch (error) {
      websocketStore.disconnect()
      throw error
    }
    if (password)
      saveRoomPassword(room.id, password)
    saveLastJoinedRoom(room.id)
  }
  const enterRoom = () => {
    joined.set(true)
    setView('listen')
    changePanel(() => null)
  }
  const share = async () => {
    const url = applyEnvironment(getLocation())
    url.searchParams.set('houseId', roomStore.state.id)
    url.searchParams.set('housePwd', '')
    try {
      const result = await shareRoom(`加入我的音乐房间 - ${roomStore.state.name}`, url.href)
      if (result === 'copied')
        notificationStore.success('房间链接已复制')
    } catch (error) {
      if (!(error instanceof DOMException && error.name === 'AbortError'))
        notificationStore.error('无法分享房间链接')
    }
  }
  const mobile = compact
  cleanup(onHostEvent('app.media.stop', () => {
    joined.set(false)
    changePanel(() => null)
    setView('listen')
  }))

  return (
    <ErrorBoundary
      fallback={(error) => {
        console.error('界面加载失败:', error)
        return (
          <EmptyState title="界面加载失败" action={<TextButton title="重新加载" onPress={reloadApp} />}>
            {error instanceof Error ? error.message : String(error)}
          </EmptyState>
        )
      }}
    >
      <AudioEngine />
      <View style={{ flex: 1, paddingTop: 'env(safe-area-inset-top)', paddingBottom: mobile() ? 0 : 'env(safe-area-inset-bottom)', backgroundColor: '#000000' }}>
        <View style={{ flex: 1, minHeight: 0 }}>
          <View style={{ height: mobile() ? 56 : 76, flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: mobile() ? 8 : 20, paddingHorizontal: mobile() ? 12 : 24, backgroundColor: '#000000', zIndex: 30 }}>
            <View style={{ flexDirection: 'row', alignItems: 'center', gap: mobile() ? 0 : 4, width: mobile() ? 88 : undefined, flexShrink: 0 }}>
              <HeaderMenu view={view()} joined={joined()} onNavigate={setView} onShare={share} onHelp={() => helpOpen.set(true)} />
              <IconButton icon="ChevronLeft" label="后退" disabled={viewHistory().index === 0} onPress={() => stepView(-1)} />
              <If when={!mobile()}>
                <IconButton icon="ChevronRight" label="前进" disabled={viewHistory().index === viewHistory().views.length - 1} onPress={() => stepView(1)} />
              </If>
            </View>
            <If when={!mobile()}>
              <View style={{ flex: 1, maxWidth: 680, minWidth: 0, marginHorizontal: 'auto', flexDirection: 'row', alignItems: 'center', gap: 12 }}>
                <IconButton
                  icon="House"
                  label={joined() ? '返回聆听' : '发现房间'}
                  selected={view() === (joined() ? 'listen' : 'rooms')}
                  color={view() === (joined() ? 'listen' : 'rooms') ? brand : muted}
                  onPress={() => setView(joined() ? 'listen' : 'rooms')}
                  style={{ width: 44, height: 44, borderRadius: 999, backgroundColor: surface }}
                />
                <If when={!mobile()}>
                  <HeaderSearch
                    query={searchQuery()}
                    onQueryChange={value => searchQuery.set(value)}
                    initialSearch={searchRequest()}
                    open={searchOpen()}
                    onOpenChange={value => searchOpen.set(value)}
                    onSearch={submitSearch}
                  />
                </If>
              </View>
            </If>
            <If when={mobile()}>
              <View style={{ flex: 1, minWidth: 0, gap: 2, alignItems: 'center' }}>
                <Text style={[font(), { maxWidth: '100%', color: foreground, fontWeight: '600', textAlign: 'center', numberOfLines: 1 }]}>{joined() ? roomName() || '同频' : '同频'}</Text>
                <ConnectionStatus joined={joined()} />
              </View>
            </If>
            <View style={{ width: mobile() ? 88 : undefined, flexShrink: 0, flexDirection: 'row', alignItems: 'center', justifyContent: 'flex-end', gap: 8 }}>
              <If when={!mobile()}>
                <ConnectionStatus joined={joined()} />
              </If>
              <UserProfile onPress={() => setView('settings')} />
            </View>
          </View>
          <View style={{ flex: 1, minHeight: 0, flexDirection: 'row', gap: mobile() ? 0 : 8, padding: mobile() ? 0 : 8, backgroundColor: '#000000' }}>
            <View style={[{ flex: 1, minWidth: 0, minHeight: 0, borderRadius: mobile() ? 0 : 12, overflow: 'hidden', backgroundColor: '#121212' }, pageMotion.style]}>
              <If when={view() === 'rooms'}>
                <PageScroll><RoomBrowser onJoin={join} joined={joined()} onJoined={enterRoom} /></PageScroll>
              </If>
              <If when={view() !== 'rooms' && view() !== 'settings' && !joined()}>
                <EmptyState icon="Headphones" title="先找一个听歌房" action={<TextButton title="发现房间" onPress={() => setView('rooms')} />}>加入一个房间，开始同步听歌、点歌与聊天。</EmptyState>
              </If>
              <If when={mobile() && joined() && isMobileTab(view())}>
                <MobileTabPages active={view() as MobileTabView} onSearch={() => setView('search')} initialQuery={searchQuery()} initialSearch={searchRequest()} onSubmitSearch={submitSearch} />
              </If>
              <If when={!mobile() && joined() && view() === 'listen'}>
                <ListenPage />
              </If>
              <If when={!mobile() && joined() && view() === 'search'}>
                <PageScroll><SearchPage initialQuery={searchQuery()} initialSearch={searchRequest()} onSearch={submitSearch} /></PageScroll>
              </If>
              <If when={!mobile() && joined() && view() === 'queue'}>
                <QueuePage onSearch={() => setView('search')} />
              </If>
              <If when={!mobile() && joined() && view() === 'chat'}>
                <ChatPanel />
              </If>
              <If when={joined() && view() === 'history'}>
                <HistoryPage />
              </If>
              <If when={view() === 'settings'}>
                <SettingsPage />
              </If>
            </View>
            <If when={joined() && panel() && view() !== panel() && !mobile()}>
              <View style={[{ width: windowSize().width <= 1100 ? 420 : 480, minHeight: 0, borderRadius: 12, overflow: 'hidden', backgroundColor: '#121212' }, panelMotion.style]}>
                <View style={[{ flex: 1, minHeight: 0, minWidth: 0 }, panelContentMotion.style]}>
                  <If
                    when={panel() === 'chat'}
                    else={(
                      <PlaylistPanel
                        active={panel() === 'history' ? 'history' : 'queue'}
                        onSelect={next => changePanel(() => next)}
                        onClose={() => changePanel(() => null)}
                        onSearch={() => setView('search')}
                      />
                    )}
                  >
                    <ChatPanel onClose={() => changePanel(() => null)} />
                  </If>
                </View>
              </View>
            </If>
          </View>
        </View>
        <PlayerBar
          onLyrics={() => setView('listen')}
          onQueue={onQueue}
          onChat={onChat}
          canOpenPanels={joined()}
          chatOpen={joined() && (view() === 'chat' || panel() === 'chat')}
          queueOpen={joined() && (view() === 'queue' || panel() === 'queue')}
        />
        <If when={mobile()}>
          <View role="tablist" accessibilityLabel="底部导航" style={{ display: 'grid', gridTemplateColumns: 'repeat(4, minmax(0, 1fr))', paddingHorizontal: 8, paddingTop: 4, paddingBottom: 'calc(env(safe-area-inset-bottom) + 6px)', backgroundColor: '#000000' }}>
            <Each of={mobileNavigation} key={item => item.view}>
              {item => (
                <View role="tab" accessibilityState={{ selected: view() === item().view || (item().view === 'listen' && view() === 'rooms') }} class="cursor-pointer" accessibilityLabel={item().label} onPress={() => setView(item().view === 'listen' && !joined() ? 'rooms' : item().view)} style={{ minHeight: 54, alignItems: 'center', justifyContent: 'center', gap: 4, borderRadius: 8 }}>
                  <Icon name={item().icon} size={24} color={view() === item().view || (item().view === 'listen' && view() === 'rooms') ? foreground : muted} />
                  <Text style={[font(-3), { fontWeight: view() === item().view ? '600' : '400', color: view() === item().view || (item().view === 'listen' && view() === 'rooms') ? foreground : muted }]}>{item().label}</Text>
                </View>
              )}
            </Each>
          </View>
        </If>
      </View>
      <If when={helpOpen()}>
        <Modal title="同频" onClose={() => helpOpen.set(false)}>
          <View style={{ gap: 16 }}>
            <Text style={[font(4), { color: foreground, fontWeight: '600' }]}>和朋友一起听同一首歌</Text>
            <Text style={[font(), { color: muted, lineHeight: 26 }]}>创建或加入房间后，可以同步播放、搜索点歌、投票切歌，并在房间里聊天。</Text>
            <Text style={[font(-1), { color: muted }]}>NSH 0.4.0 · Web / Windows / Android</Text>
          </View>
        </Modal>
      </If>
      <SystemPrompts enabled={joined() && view() !== 'rooms'} />
      <Notifications />
    </ErrorBoundary>
  )
}

function HeaderMenu(props: {
  view: AppView
  joined: boolean
  onNavigate: (view: AppView) => void
  onShare: () => void
  onHelp: () => void
}) {
  const open = signal(false)
  const popup = createMotion()
  let anchor: ViewRef | undefined
  const toggle = () => {
    if (!open()) {
      void popup.play({
        duration: motionDuration(200),
        keyframes: [
          { opacity: 0, translateY: -8, scale: 0.96 },
          { opacity: 1, translateY: 0, scale: 1 },
        ],
      })
    }
    open.update(value => !value)
  }
  return (
    <View ref={anchor}>
      <IconButton icon="MoreHorizontal" label="更多功能" selected={open()} onPress={toggle} />
      <If when={open()}>
        <Overlay
          anchor={anchor}
          placement="bottom"
          align="start"
          offset={12}
          onPressOutside={() => open.set(false)}
          onCancel={() => open.set(false)}
          style={[{ width: 180, padding: 6, gap: 4, borderWidth: 1, borderColor: '#333337', borderRadius: 8, backgroundColor: '#242427', boxShadow: '0 8px 24px rgba(0,0,0,0.24)', transformOrigin: 'top left' }, popup.style]}
        >
          <Each of={navigation} key={item => item.view}>
            {item => (
              <MenuItem
                icon={item().icon}
                label={item().label}
                selected={props.view === item().view}
                onPress={() => {
                  props.onNavigate(item().view)
                  open.set(false)
                }}
              />
            )}
          </Each>
          <MenuItem
            icon="Settings"
            label="设置"
            selected={props.view === 'settings'}
            onPress={() => {
              props.onNavigate('settings')
              open.set(false)
            }}
          />
          <MenuItem
            icon="Share2"
            label="分享房间"
            disabled={!props.joined}
            onPress={() => {
              props.onShare()
              open.set(false)
            }}
          />
          <MenuItem
            icon="HelpCircle"
            label="帮助"
            onPress={() => {
              props.onHelp()
              open.set(false)
            }}
          />
        </Overlay>
      </If>
    </View>
  )
}

function MenuItem(props: { icon: IconName, label: string, selected?: boolean, disabled?: boolean, onPress: () => void }) {
  return (
    <View
      role="button"
      class="cursor-pointer"
      accessibilityLabel={props.label}
      accessibilityState={{ selected: props.selected, disabled: props.disabled }}
      onPress={props.disabled ? undefined : props.onPress}
      style={{ minHeight: 44, flexDirection: 'row', alignItems: 'center', gap: 12, paddingHorizontal: 12, paddingVertical: 10, borderRadius: 8, backgroundColor: props.selected ? '#3b3b3e' : 'transparent', opacity: props.disabled ? 0.45 : 1 }}
    >
      <Icon name={props.icon} color={brand} />
      <Text style={[font(), { color: foreground }]}>{props.label}</Text>
    </View>
  )
}

function UserProfile(props: { onPress: () => void }) {
  const name = useStore(userSettingsStore, state => state.userName)
  return (
    <View role="button" class="cursor-pointer" accessibilityLabel="设置" hitSlop={6} onPress={props.onPress} style={{ width: 34, height: 34, borderRadius: 999, alignItems: 'center', justifyContent: 'center', backgroundColor: '#203320' }}>
      <Text style={[font(4), { color: brand }]}>{(name() || '听').slice(0, 1)}</Text>
    </View>
  )
}

function ConnectionStatus(props: { joined: boolean }) {
  const status = useStore(websocketStore, state => state.connectionStatus)
  const connected = () => status() === 'connected'
  return (
    <View
      role="button"
      class="cursor-pointer"
      accessibilityLabel="房间连接状态"
      onPress={props.joined && !connected() ? () => websocketStore.reconnect() : undefined}
      style={{ flexDirection: 'row', alignItems: 'center', gap: 4, paddingHorizontal: 6, paddingVertical: 4, borderRadius: 4 }}
    >
      <Icon name={connected() ? 'Wifi' : 'WifiOff'} size={12} color={connected() ? success : muted} />
      <Text style={[font(-4), { color: connected() ? success : muted }]}>
        {!props.joined ? '未加入' : connected() ? '同步在线' : status() === 'connecting' ? '连接中' : '重连中'}
      </Text>
    </View>
  )
}

function PageScroll(props: { children: any }) {
  return (
    <Scroll keyboardShouldPersistTaps="handled" style={{ flex: 1 }} contentStyle={{ flexGrow: 1 }}>
      {props.children}
    </Scroll>
  )
}
