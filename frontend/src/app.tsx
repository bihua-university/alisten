import type { LucideIcon } from 'lucide-react'
import type { ErrorInfo, ReactNode } from 'react'
import type { DetailPanelView } from './components/detail-panel'
import type { MusicSearchSelection, RoomInfo } from './types'
import {
  ChevronLeft,
  ChevronRight,
  Headphones,
  HelpCircle,
  History,
  House,
  ListMusic,
  MessageCircle,
  MoreHorizontal,
  Search,
  Settings,
  Share2,
  Wifi,
  WifiOff,
  X,
} from 'lucide-react'
import { Component, lazy, Suspense, useCallback, useState } from 'react'
import { AudioEngine } from './components/audio-engine'
import { DetailPanel } from './components/detail-panel'
import { HeaderSearch } from './components/header-search'
import { Notifications, SystemPrompts } from './components/notifications'
import { PlayerBar } from './components/player-bar'
import { EmptyState, IconButton, Skeleton } from './components/ui'
import { useStore } from './hooks/use-store'
import { chatStore } from './stores/chat-store'
import { notificationStore } from './stores/notification-store'
import { playerStore } from './stores/player-store'
import { roomStore } from './stores/room-store'
import { searchStore } from './stores/search-store'
import { userSettingsStore } from './stores/user-settings-store'
import { websocketStore } from './stores/websocket-store'
import { saveLastJoinedRoom, saveRoomPassword } from './utils/user'
import { withViewTransition } from './utils/view-transition'

const Rooms = lazy(() => import('./components/room-browser'))
const Listen = lazy(() => import('./components/listen-page'))
const Queue = lazy(() => import('./components/queue-page'))
const SearchPage = lazy(() => import('./components/search-page'))
const HistoryPage = lazy(() => import('./components/history-page'))
const SettingsPage = lazy(() => import('./components/settings-page'))
const HelpModal = lazy(() => import('./components/help-modal'))
const Chat = lazy(() =>
  import('./components/chat-panel').then(module => ({
    default: module.ChatPanel,
  })),
)

type View = 'listen' | 'search' | 'queue' | 'chat' | 'history' | 'rooms' | 'settings'
const navigation: { view: View, label: string, icon: LucideIcon }[] = [
  { view: 'listen', label: '正在聆听', icon: Headphones },
  { view: 'queue', label: '播放列表', icon: ListMusic },
  { view: 'chat', label: '房间聊天', icon: MessageCircle },
  { view: 'history', label: '播放历史', icon: History },
  { view: 'rooms', label: '发现房间', icon: House },
]
const mobileNavigation = navigation.filter(item => item.view !== 'history' && item.view !== 'rooms')

export function App() {
  const [viewHistory, setViewHistory] = useState<{ views: View[], index: number }>({ views: ['rooms'], index: 0 })
  const view = viewHistory.views[viewHistory.index]
  const [joined, setJoined] = useState(false)
  const [panel, setPanel] = useState<DetailPanelView | null>(null)
  const [helpOpen, setHelpOpen] = useState(false)
  const [searchOpen, setSearchOpen] = useState(false)
  const [searchRequest, setSearchRequest] = useState<MusicSearchSelection>()
  // Text owned by the search fields. Opening a playlist searches an internal `*id` lookup and carries
  // its title as a label, which must not overwrite the keyword the user typed.
  const [searchQuery, setSearchQuery] = useState('')
  const setView = useCallback((nextView: View) => {
    setSearchOpen(false)
    setViewHistory((previous) => {
      if (previous.views[previous.index] === nextView)
        return previous
      const views = [...previous.views.slice(0, previous.index + 1), nextView]
      return { views, index: views.length - 1 }
    })
  }, [])
  const stepView = useCallback((direction: -1 | 1) => {
    setSearchOpen(false)
    setViewHistory(previous => ({
      ...previous,
      index: Math.max(0, Math.min(previous.views.length - 1, previous.index + direction)),
    }))
  }, [])
  const changePanel = useCallback((next: (current: DetailPanelView | null) => DetailPanelView | null) => {
    const target = next(panel)
    if (target === panel)
      return
    // Only a swap between two open views pushes sideways; opening and closing keep the panel's own animation.
    if (panel && target)
      withViewTransition(() => setPanel(target))
    else
      setPanel(target)
  }, [panel])
  const roomName = useStore(roomStore, s => s.name)
  const onSearch = useCallback(() => setSearchOpen(true), [])
  const submitSearch = useCallback(
    (selection: Omit<MusicSearchSelection, 'id'>) => {
      if (!selection.label)
        setSearchQuery(selection.keyword)
      setSearchRequest(prev => ({ ...selection, id: (prev?.id || 0) + 1 }))
      searchStore.clearSearchResults()
      setSearchOpen(false)
      setView('search')
    },
    [setView],
  )
  const onLyrics = useCallback(() => setView('listen'), [setView])
  const onQueue = useCallback(() => {
    if (window.matchMedia('(max-width: 767px)').matches) {
      setView('queue')
    } else if (view === 'queue') {
      setView('listen')
      changePanel(p => (p === 'queue' ? null : p))
    } else {
      changePanel(p => (p === 'queue' ? null : 'queue'))
    }
  }, [setView, changePanel, view])
  const onChat = useCallback(() => {
    if (window.matchMedia('(max-width: 767px)').matches) {
      setView('chat')
    } else if (view === 'chat') {
      setView('listen')
      changePanel(p => (p === 'chat' ? null : p))
    } else {
      changePanel(p => (p === 'chat' ? null : 'chat'))
    }
  }, [setView, changePanel, view])
  const closeHelp = useCallback(() => setHelpOpen(false), [])
  const join = useCallback((room: RoomInfo, password?: string) => {
    websocketStore.disconnect()
    playerStore.resetRoom()
    chatStore.reset()
    searchStore.clearSearchResults()
    roomStore.updateRoomInfo(room)
    roomStore.setCurrentPassword(password)
    if (password)
      saveRoomPassword(room.id, password)
    saveLastJoinedRoom(room.id)
    setJoined(true)
    setView('listen')
    setPanel(null)
    websocketStore.connect()
  }, [setView])
  const share = async () => {
    const url = new URL(window.location.pathname, window.location.origin)
    url.searchParams.set('houseId', roomStore.state.id)
    url.searchParams.set('housePwd', '')
    try {
      if (navigator.share) {
        await navigator.share({
          title: `加入我的音乐房间 - ${roomStore.state.name}`,
          text: '来和我一起听歌吧！',
          url: url.href,
        })
      } else if (navigator.clipboard && window.isSecureContext) {
        await navigator.clipboard.writeText(url.href)
        notificationStore.success('房间链接已复制')
      } else {
        window.prompt('复制房间链接', url.href)
      }
    } catch (error) {
      if (!(error instanceof DOMException && error.name === 'AbortError'))
        window.prompt('复制房间链接', url.href)
    }
  }
  return (
    <AppErrorBoundary>
      <AudioEngine />
      <div className="app-shell">
        <div className="workspace">
          <header className="workspace-header">
            <div className="header-leading">
              <details
                className="header-menu"
                onBlur={(e) => {
                  if (!e.currentTarget.contains(e.relatedTarget as Node | null))
                    e.currentTarget.removeAttribute('open')
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Escape')
                    e.currentTarget.removeAttribute('open')
                }}
              >
                <summary aria-label="更多功能" title="更多功能">
                  <MoreHorizontal size={20} />
                </summary>
                <nav className="main-nav" aria-label="主导航">
                  {navigation.map(({ view: target, label, icon: Icon }) => (
                    <button
                      type="button"
                      key={target}
                      className={`nav-item text-ui-base ${view === target ? 'selected' : ''}`}
                      aria-current={view === target ? 'page' : undefined}
                      onClick={(e) => {
                        setView(target)
                        e.currentTarget
                          .closest('details')
                          ?.removeAttribute('open')
                      }}
                    >
                      <Icon size={20} strokeWidth={1.7} />
                      {label}
                    </button>
                  ))}
                  <button
                    type="button"
                    className={`nav-item text-ui-base ${view === 'settings' ? 'selected' : ''}`}
                    aria-current={view === 'settings' ? 'page' : undefined}
                    onClick={(e) => {
                      setView('settings')
                      e.currentTarget.closest('details')?.removeAttribute('open')
                    }}
                  >
                    <Settings size={20} />
                    设置
                  </button>
                  <button
                    type="button"
                    className="nav-item text-ui-base"
                    disabled={!joined}
                    onClick={(e) => {
                      void share()
                      e.currentTarget
                        .closest('details')
                        ?.removeAttribute('open')
                    }}
                  >
                    <Share2 size={20} />
                    分享房间
                  </button>
                  <button
                    type="button"
                    className="nav-item text-ui-base"
                    onClick={(e) => {
                      setHelpOpen(true)
                      e.currentTarget
                        .closest('details')
                        ?.removeAttribute('open')
                    }}
                  >
                    <HelpCircle size={20} />
                    帮助
                  </button>
                </nav>
              </details>
              <IconButton
                icon={ChevronLeft}
                label="后退"
                disabled={viewHistory.index === 0}
                onClick={() => stepView(-1)}
              />
              <IconButton
                icon={ChevronRight}
                label="前进"
                className="header-forward"
                disabled={viewHistory.index === viewHistory.views.length - 1}
                onClick={() => stepView(1)}
              />
            </div>
            <div className="header-search-area">
              <IconButton
                icon={House}
                label={joined ? '返回聆听' : '发现房间'}
                title={joined ? roomName : '发现房间'}
                className="home-widget"
                aria-current={view === (joined ? 'listen' : 'rooms') ? 'page' : undefined}
                onClick={() => setView(joined ? 'listen' : 'rooms')}
              />
              <HeaderSearch
                key={searchRequest?.id}
                initialQuery={searchQuery}
                initialSearch={searchRequest}
                open={searchOpen}
                onOpenChange={setSearchOpen}
                onSearch={submitSearch}
              />
              <IconButton
                icon={Search}
                label="搜索音乐"
                className="mobile-search-button"
                aria-current={view === 'search' ? 'page' : undefined}
                onClick={() => setView('search')}
              />
            </div>
            <div className="header-actions">
              <ConnectionStatus joined={joined} />
              <UserProfile onClick={() => setView('settings')} />
            </div>
          </header>
          <div className="workspace-body">
            <main
              className={`main-content ${view === 'chat' ? 'chat-page' : view === 'settings' ? 'settings-view' : ''}`}
              key={view}
            >
              <Suspense
                fallback={(
                  <div className="page-content">
                    <Skeleton rows={6} />
                  </div>
                )}
              >
                {view === 'settings'
                  ? (
                      <SettingsPage />
                    )
                  : view === 'rooms'
                    ? (
                        <Rooms onJoin={join} joined={joined} />
                      )
                    : view === 'history'
                      ? (
                          <HistoryPage />
                        )
                      : !joined
                          ? (
                              <EmptyState
                                icon={Headphones}
                                title="先找一个听歌房"
                                action={(
                                  <button
                                    type="button"
                                    className="button-primary"
                                    onClick={() => setView('rooms')}
                                  >
                                    发现房间
                                  </button>
                                )}
                              >
                                加入一个房间，开始同步听歌、点歌与聊天。
                              </EmptyState>
                            )
                          : view === 'listen'
                            ? (
                                <Listen />
                              )
                            : view === 'search'
                              ? (
                                  <SearchPage
                                    key={searchRequest?.id}
                                    initialQuery={searchQuery}
                                    initialSearch={searchRequest}
                                    onSearch={submitSearch}
                                  />
                                )
                              : view === 'queue'
                                ? (
                                    <Queue onSearch={onSearch} />
                                  )
                                : (
                                    <Chat />
                                  )}
              </Suspense>
            </main>
            <DetailPanel panel={joined && view !== panel ? panel : null}>
              {visiblePanel => (
                <>
                  {visiblePanel !== 'chat' && (
                    <div className="detail-tabs" aria-label="侧栏视图">
                      <button
                        type="button"
                        className="detail-tab text-ui-base"
                        aria-pressed={visiblePanel === 'queue'}
                        onClick={() => changePanel(() => 'queue')}
                      >
                        队列
                      </button>
                      <button
                        type="button"
                        className="detail-tab text-ui-base"
                        aria-pressed={visiblePanel === 'history'}
                        onClick={() => changePanel(() => 'history')}
                      >
                        最近播放
                      </button>
                      <IconButton
                        icon={X}
                        label="关闭侧栏"
                        onClick={() => changePanel(() => null)}
                      />
                    </div>
                  )}
                  <Suspense fallback={<Skeleton />}>
                    {visiblePanel === 'chat'
                      ? (
                          <Chat onClose={() => changePanel(() => null)} />
                        )
                      : visiblePanel === 'history'
                        ? (
                            <HistoryPage compact />
                          )
                        : (
                            <Queue compact onSearch={onSearch} />
                          )}
                  </Suspense>
                </>
              )}
            </DetailPanel>
          </div>
        </div>
        <PlayerBar
          onLyrics={onLyrics}
          onQueue={onQueue}
          onChat={onChat}
          canOpenPanels={joined}
          chatOpen={joined && (view === 'chat' || panel === 'chat')}
          queueOpen={joined && (view === 'queue' || panel === 'queue')}
        />
        <nav className="mobile-nav" aria-label="移动端导航">
          {mobileNavigation.map(({ view: target, icon: Icon }) => (
            <button
              type="button"
              className={`text-ui-xs ${view === target ? 'selected' : ''}`}
              key={target}
              aria-current={view === target ? 'page' : undefined}
              onClick={() => setView(target)}
            >
              <Icon size={24} strokeWidth={1.7} />
              <span>
                {target === 'listen'
                  ? '聆听'
                  : target === 'queue'
                    ? '待播'
                    : '聊天'}
              </span>
            </button>
          ))}
        </nav>
      </div>
      <Suspense fallback={null}>
        {helpOpen && <HelpModal onClose={closeHelp} />}
      </Suspense>
      <SystemPrompts />
      <Notifications />
    </AppErrorBoundary>
  )
}

function UserProfile({ onClick }: { onClick: () => void }) {
  const name = useStore(userSettingsStore, s => s.userName)
  return (
    <button
      type="button"
      className="user-profile"
      aria-label="设置"
      title="个人设置"
      onClick={onClick}
    >
      <span className="user-initial text-ui-lg">
        {(name || '听').slice(0, 1)}
      </span>
      <span className="song-copy">
        <strong className="text-ui-base truncate">{name || '匿名用户'}</strong>
        <span className="text-ui-xs muted">在这里，和音乐相遇</span>
      </span>
    </button>
  )
}

function ConnectionStatus({ joined }: { joined: boolean }) {
  const status = useStore(websocketStore, s => s.connectionStatus)
  const connected = status === 'connected'
  return (
    <button
      type="button"
      className={`connection-status text-ui-xs ${connected ? 'connected' : ''}`}
      title={joined && !connected ? '点击重试连接' : '房间连接状态'}
      disabled={!joined || connected}
      onClick={() => websocketStore.reconnect()}
    >
      {connected ? <Wifi size={12} /> : <WifiOff size={12} />}
      <span>
        {!joined
          ? '未加入'
          : connected
            ? '同步在线'
            : status === 'connecting'
              ? '连接中'
              : '重连中'}
      </span>
    </button>
  )
}

class AppErrorBoundary extends Component<
  { children: ReactNode },
  { error: boolean }
> {
  state = { error: false }
  static getDerivedStateFromError() {
    return { error: true }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('界面加载失败:', error, info.componentStack)
  }

  render() {
    if (this.state.error) {
      return (
        <EmptyState
          title="界面加载失败"
          action={(
            <button
              type="button"
              className="button-primary"
              onClick={() => window.location.reload()}
            >
              重新加载
            </button>
          )}
        >
          请刷新页面后重试。
        </EmptyState>
      )
    }
    return this.props.children
  }
}
