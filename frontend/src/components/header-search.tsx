import type { MusicSearchSelection } from '@/types'
import { Clock, Search, X } from 'lucide-react'
import { useEffect, useId, useRef, useState } from 'react'
import { useStore } from '@/hooks/use-store'
import { historyStore } from '@/stores/history-store'
import { websocketStore } from '@/stores/websocket-store'
import { IconButton } from './ui'

export function HeaderSearch({
  initialQuery,
  initialSearch,
  open,
  onOpenChange,
  onSearch,
}: {
  initialQuery: string
  initialSearch?: MusicSearchSelection
  open: boolean
  onOpenChange: (open: boolean) => void
  onSearch: (selection: Omit<MusicSearchSelection, 'id'>) => void
}) {
  const [query, setQuery] = useState(initialQuery)
  const [source] = useState<'netease' | 'qq'>(() => {
    if (initialSearch)
      return initialSearch.source
    try {
      return JSON.parse(
        localStorage.getItem('music-modal-platform') || '"netease"',
      ) === 'qq'
        ? 'qq'
        : 'netease'
    } catch {
      return 'netease'
    }
  })
  const mode = initialSearch?.mode ?? 'all'
  const ref = useRef<HTMLDivElement>(null)
  const input = useRef<HTMLInputElement>(null)
  const id = useId()
  const connected = useStore(
    websocketStore,
    s => s.connectionStatus === 'connected',
  )
  const history = useStore(historyStore, s => s.searchHistory)
  const enabled = useStore(historyStore, s => s.enableSearchHistory)
  const showHistory = open && enabled && history.length > 0
  useEffect(() => {
    if (!open)
      return
    input.current?.focus()
    const dismiss = (e: PointerEvent) => {
      if (!ref.current?.contains(e.target as Node))
        onOpenChange(false)
    }
    document.addEventListener('pointerdown', dismiss)
    return () => document.removeEventListener('pointerdown', dismiss)
  }, [open, onOpenChange])
  const submit = () => {
    if (connected && query.trim())
      onSearch({ keyword: query.trim(), source, mode })
  }
  return (
    <div
      className="header-search"
      ref={ref}
      onKeyDown={(e) => {
        if (e.key === 'Escape') {
          e.stopPropagation()
          onOpenChange(false)
        }
      }}
    >
      <form
        className={`header-search-field ${open ? 'search-open' : ''}`}
        role="search"
        onSubmit={(e) => {
          e.preventDefault()
          submit()
        }}
      >
        <Search size={22} strokeWidth={1.7} aria-hidden="true" />
        <input
          ref={input}
          aria-label="搜索音乐"
          aria-haspopup="dialog"
          aria-expanded={showHistory}
          aria-controls={showHistory ? id : undefined}
          placeholder="想播放什么？"
          value={query}
          onFocus={() => onOpenChange(true)}
          onClick={() => onOpenChange(true)}
          onChange={e => setQuery(e.target.value)}
        />
        {query && (
          <IconButton
            icon={X}
            label="清空顶部搜索"
            onClick={() => onSearch({ keyword: '', source, mode })}
          />
        )}
      </form>
      {showHistory && (
        <div
          className="search-popover"
          role="dialog"
          aria-label="搜索选项"
          id={id}
        >
          <div className="header-search-history">
            <span className="text-ui-sm muted">最近搜索</span>
            {history.slice(0, 4).map(item => (
              <button
                type="button"
                className="search-recent text-ui-caption"
                key={[item.query, item.platform, item.searchMode].join(':')}
                disabled={!connected}
                onClick={() => {
                  setQuery(item.query)
                  onSearch({
                    keyword: item.query,
                    source: item.platform === 'qq' ? 'qq' : 'netease',
                    mode:
                        item.searchMode === 'all' ? 'all' : item.searchMode === 'playlist' ? 'playlist' : 'song',
                  })
                }}
              >
                <Clock size={16} />
                <span className="truncate">{item.query}</span>
              </button>
            ))}
          </div>

        </div>
      )}
    </div>
  )
}
