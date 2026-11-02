import {
  ListMusic,
  MessageCircle,
  SkipForward,
  Volume2,
  VolumeX,
} from 'lucide-react'
import { memo, useEffect, useRef, useState } from 'react'
import { useStore } from '@/hooks/use-store'
import { playerStore } from '@/stores/player-store'
import { websocketStore } from '@/stores/websocket-store'
import { formatTime } from '@/utils/time'
import { Artwork, IconButton } from './ui'

const PlaybackProgress = memo(() => {
  const song = useStore(playerStore, s => s.currentSong)
  const bar = useRef<HTMLDivElement>(null)
  const timeLabel = useRef<HTMLSpanElement>(null)
  const meter = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const duration = (song?.duration || 0) / 1000
    let lastSecond = -1
    const update = (time: number) => {
      const clamped = Math.max(0, Math.min(duration, time))
      if (bar.current)
        bar.current.style.transform = `scaleX(${duration ? clamped / duration : 0})`
      const second = Math.floor(clamped)
      if (second !== lastSecond) {
        lastSecond = second
        if (timeLabel.current)
          timeLabel.current.textContent = `${formatTime(clamped)} / ${formatTime(duration)}`
        meter.current?.setAttribute('aria-valuenow', String(second))
      }
    }
    update(playerStore.getCurrentTime())
    playerStore.registerProgressCallback(update)
    return () => playerStore.unregisterProgressCallback(update)
  }, [song])
  return (
    <>
      <span ref={timeLabel} className="player-time text-ui-xs">
        0:00 / 0:00
      </span>
      <div className="playback-progress">
        <div
          ref={meter}
          className="progress-track"
          role="progressbar"
          aria-label="播放进度"
          aria-valuemin={0}
          aria-valuemax={Math.floor((song?.duration || 0) / 1000)}
          aria-valuenow={0}
        >
          <div ref={bar} className="progress-fill" />
        </div>
      </div>
    </>
  )
})

export const PlayerBar = memo(
  ({
    onLyrics,
    onQueue,
    onChat,
    canOpenPanels,
    chatOpen,
    queueOpen,
  }: {
    onLyrics: () => void
    onQueue: () => void
    onChat: () => void
    canOpenPanels: boolean
    chatOpen: boolean
    queueOpen: boolean
  }) => {
    const song = useStore(playerStore, s => s.currentSong)
    const volume = useStore(playerStore, s => s.volume)
    const muted = useStore(playerStore, s => s.isMuted)
    const connected = useStore(
      websocketStore,
      s => s.connectionStatus === 'connected',
    )
    const [skipping, setSkipping] = useState(false)
    return (
      <section className="player-bar glass-surface" aria-label="播放器">
        <button
          type="button"
          className="player-artwork"
          onClick={onLyrics}
          aria-label="查看专辑封面与歌词"
        >
          <Artwork src={song?.cover} title={song?.title || '暂无歌曲'} />
        </button>
        <div className="player-track">
          <div className="player-song">
            <span className="song-copy">
              <strong className="text-ui-lg truncate">
                {song?.title || '暂无歌曲'}
              </strong>
              <span className="text-ui-caption muted truncate">
                {song?.artist || '未知艺术家'}
              </span>
            </span>
          </div>
          <PlaybackProgress />
        </div>
        <div className="player-right">
          <div className="player-center transport">
            <IconButton
              icon={SkipForward}
              label="投票切歌"
              disabled={!song || !connected || skipping}
              onClick={async () => {
                setSkipping(true)
                await playerStore.skipSong()
                setSkipping(false)
              }}
            />
          </div>
          <div className="player-tools">
            <IconButton
              icon={muted ? VolumeX : Volume2}
              label={muted ? '取消静音' : '静音'}
              onClick={() => playerStore.toggleMute()}
            />
            <input
              className="volume-slider"
              type="range"
              min="0"
              max="100"
              value={muted ? 0 : volume}
              aria-label="音量"
              onChange={(e) => {
                playerStore.setVolume(Number(e.target.value))
                if (muted && Number(e.target.value) > 0)
                  playerStore.toggleMute()
              }}
            />
            <IconButton
              icon={MessageCircle}
              label="房间聊天"
              className="player-panel-toggle"
              disabled={!canOpenPanels}
              aria-pressed={chatOpen}
              onClick={onChat}
            />
            <IconButton
              icon={ListMusic}
              label="待播队列"
              className="player-panel-toggle"
              disabled={!canOpenPanels}
              aria-pressed={queueOpen}
              onClick={onQueue}
            />
          </div>
        </div>
      </section>
    )
  },
)
