import { CirclePlus, Download, History, Search, Trash2 } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useStore } from '@/hooks/use-store'
import { historyStore } from '@/stores/history-store'
import { notificationStore } from '@/stores/notification-store'
import { playerStore } from '@/stores/player-store'
import { websocketStore } from '@/stores/websocket-store'
import { SongArtwork } from './song-artwork'
import { AsyncIconButton, EmptyState, IconButton, Modal } from './ui'

export default function HistoryPage({
  compact = false,
}: {
  compact?: boolean
}) {
  const history = useStore(historyStore, s => s.playHistory)
  const recording = useStore(historyStore, s => s.isRecording)
  const connected = useStore(
    websocketStore,
    s => s.connectionStatus === 'connected',
  )
  const [query, setQuery] = useState('')
  const [limit, setLimit] = useState(80)
  const [confirm, setConfirm] = useState(false)
  const filtered = useMemo(
    () =>
      history.filter(item =>
        `${item.song.title} ${item.song.artist} ${item.song.album}`
          .toLowerCase()
          .includes(query.toLowerCase()),
      ),
    [history, query],
  )
  let lastDate = ''
  return (
    <div className={`page-content history-page ${compact ? 'history-compact' : ''}`}>
      <div className="page-title">
        <h1 className="text-ui-xl">播放历史</h1>
        <p className="text-ui-base muted">重温那些一起听过的旋律。</p>
      </div>
      <div className="history-toolbar">
        <label className="search-field">
          <Search size={18} />
          <input
            value={query}
            onChange={(e) => {
              setQuery(e.target.value)
              setLimit(80)
            }}
            placeholder="搜索播放历史"
            aria-label="搜索播放历史"
          />
        </label>
        <IconButton
          icon={Download}
          label="导出播放历史"
          disabled={!history.length}
          onClick={() => historyStore.exportPlayHistory()}
        />
        <IconButton
          icon={Trash2}
          label="清空播放历史"
          disabled={!history.length}
          onClick={() => setConfirm(true)}
        />
      </div>
      <div className="section-heading">
        <span className="text-ui-sm muted">
          {history.length}
          {' '}
          首歌曲 · 保存在此设备
        </span>
        <label className="checkbox-label text-ui-sm">
          <input
            type="checkbox"
            checked={recording}
            onChange={() => historyStore.togglePlayHistoryRecording()}
          />
          记录播放历史
        </label>
      </div>
      {filtered.length
        ? (
            <div className="history-list">
              {filtered.slice(0, limit).map((item) => {
                const date = new Date(item.playedAt).toLocaleDateString('zh-CN', {
                  month: 'long',
                  day: 'numeric',
                  year: 'numeric',
                })
                const heading = date !== lastDate
                lastDate = date
                return (
                  <div key={`${item.song.source}:${item.song.id}:${item.playedAt}`}>
                    {heading && (
                      <h2 className="history-date text-ui-sm muted">{date}</h2>
                    )}
                    <div className="history-row">
                      <SongArtwork song={item.song} />
                      <div className="history-song song-copy">
                        <strong className="text-ui-base truncate">
                          {item.song.title}
                        </strong>
                        <span className="text-ui-sm muted truncate">
                          {item.song.artist}
                          {' '}
                          ·
                          {item.song.album}
                        </span>
                      </div>
                      <time
                        className="text-ui-sm muted"
                        dateTime={new Date(item.playedAt).toISOString()}
                      >
                        {new Date(item.playedAt).toLocaleTimeString('zh-CN', {
                          hour: '2-digit',
                          minute: '2-digit',
                        })}
                      </time>
                      <AsyncIconButton
                        icon={CirclePlus}
                        className="song-action"
                        label={`点播 ${item.song.title}`}
                        disabled={!connected}
                        onAction={() => playerStore.pickMusic(item.song)}
                      />
                      <IconButton
                        icon={Trash2}
                        className="song-action"
                        label={`删除 ${item.song.title} 的记录`}
                        onClick={() => {
                          historyStore.removeFromPlayHistory(history.indexOf(item))
                          notificationStore.success('已删除播放记录')
                        }}
                      />
                    </div>
                  </div>
                )
              })}
              {filtered.length > limit && (
                <button
                  type="button"
                  className="button-secondary load-more"
                  onClick={() => setLimit(n => n + 80)}
                >
                  显示更多
                </button>
              )}
            </div>
          )
        : (
            <EmptyState
              icon={History}
              title={query ? '没有找到相关记录' : '你的音乐足迹，从这里开始'}
            >
              {query
                ? '换个关键词试试。'
                : '房间播放过的歌曲会保存在此设备，方便再次点播。'}
            </EmptyState>
          )}
      {confirm && (
        <Modal title="清空播放历史" onClose={() => setConfirm(false)}>
          <p className="text-ui-base muted">将清除此设备上的全部播放记录。</p>
          <div className="dialog-actions">
            <button
              type="button"
              className="button-secondary"
              onClick={() => setConfirm(false)}
            >
              取消
            </button>
            <button
              type="button"
              className="button-destructive"
              onClick={() => {
                historyStore.clearPlayHistory()
                setConfirm(false)
              }}
            >
              清空记录
            </button>
          </div>
        </Modal>
      )}
    </div>
  )
}
