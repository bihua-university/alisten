import { ListMusic, Search } from 'lucide-react'
import { useStore } from '@/hooks/use-store'
import { playerStore } from '@/stores/player-store'
import { websocketStore } from '@/stores/websocket-store'
import { SongArtwork } from './song-artwork'
import { SongList } from './song-list'
import { EmptyState } from './ui'

export default function QueuePage({
  onSearch,
  compact = false,
}: {
  onSearch: () => void
  compact?: boolean
}) {
  const songs = useStore(playerStore, s => s.playlist)
  const current = useStore(playerStore, s => s.queueCurrent)
  const connected = useStore(
    websocketStore,
    s => s.connectionStatus === 'connected',
  )
  return (
    <section
      className={compact ? 'queue-panel' : 'page-content queue-page'}
      aria-label="待播队列"
    >
      <div className="section-heading">
        <h1 className={compact ? 'text-ui-base' : 'text-ui-xl'}>
          播放列表
          <span className="text-ui-sm muted">{songs.length}</span>
        </h1>
        <button
          type="button"
          className="button-ghost accent"
          disabled={!connected}
          onClick={onSearch}
        >
          <Search size={16} />
          搜索
        </button>
      </div>
      {current && (
        <div className="queue-current">
          <SongArtwork song={current} />
          <div className="song-copy">
            <span className="text-ui-xs accent">当前歌曲</span>
            <strong className="text-ui-base truncate">{current.title}</strong>
            <span className="text-ui-sm muted truncate">{current.artist}</span>
          </div>
        </div>
      )}
      {songs.length
        ? (
            <SongList songs={songs} queue disabled={!connected} />
          )
        : (
            <EmptyState
              icon={ListMusic}
              title="队列还是空的"
              action={(
                <button
                  type="button"
                  className="button-secondary"
                  disabled={!connected}
                  onClick={onSearch}
                >
                  搜索音乐
                </button>
              )}
            >
              每个人都可以点歌，一起决定下一首。
            </EmptyState>
          )}
    </section>
  )
}
