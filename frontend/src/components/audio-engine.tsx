import { useEffect, useRef } from 'react'
import { chatStore } from '@/stores/chat-store'
import { historyStore } from '@/stores/history-store'
import { mediaSessionStore } from '@/stores/media-session-store'
import { notificationStore } from '@/stores/notification-store'
import { performanceStore } from '@/stores/performance-store'
import { playerStore } from '@/stores/player-store'
import { pwaStore } from '@/stores/pwa-store'
import { userSettingsStore } from '@/stores/user-settings-store'
import { websocketStore } from '@/stores/websocket-store'
import { getAppConfig, validateConfig } from '@/utils/config'
import { setViewportHeight } from '@/utils/mobile'

/** Audio/lifecycle subscriptions stay outside the React rendering path. */
export function AudioEngine() {
  const ref = useRef<HTMLAudioElement>(null)
  useEffect(() => {
    const config = getAppConfig()
    const errors = validateConfig(config)
    if (errors.length)
      console.warn('配置错误:', errors)
    const audio = ref.current!
    playerStore.setAudioPlayer(audio)
    let lastSong = playerStore.state.currentSong
    let lastStartedAtMs = playerStore.state.startedAtMs
    let lastHistoryKey = ''
    const onPlayer = () => {
      const song = playerStore.state.currentSong
      if (song === lastSong)
        return
      const changedUrl = song?.url !== lastSong?.url
      const changedStart = playerStore.state.startedAtMs !== lastStartedAtMs
      lastSong = song
      lastStartedAtMs = playerStore.state.startedAtMs
      if (song?.url) {
        if (changedUrl)
          playerStore.handleNewSong()
        else if (changedStart)
          void playerStore.onCanPlay()
        const key = `${song.source}:${song.id}:${playerStore.state.startedAtMs}`
        if (key !== lastHistoryKey) {
          historyStore.addToPlayHistory(song)
          lastHistoryKey = key
        }
      } else {
        lastHistoryKey = ''
      }
      document.title = song
        ? `${song.title} - ${song.artist} | ${config.app.name}`
        : config.app.name
    }
    let lastStatus = websocketStore.state.connectionStatus
    let outageNotified = false
    const onConnection = () => {
      const status = websocketStore.state.connectionStatus
      if (status === lastStatus)
        return
      lastStatus = status
      if (status === 'connected') {
        outageNotified = false
        notificationStore.connectionSuccess()
        void userSettingsStore.syncUserSettings()
        void userSettingsStore.pullSetting()
        void playerStore.requestMusicSync()
        void playerStore.refreshQueue()
        void chatStore.refreshOnlineUsers()
      } else if (
        (status === 'error' || status === 'reconnecting')
        && !outageNotified
      ) {
        outageNotified = true
        notificationStore.connectionError('连接中断，正在自动重连')
      }
    }
    const onResize = () => {
      setViewportHeight()
      performanceStore.applySettings()
    }
    onResize()
    window.addEventListener('resize', onResize, { passive: true })
    playerStore.addEventListener('change', onPlayer)
    websocketStore.addEventListener('change', onConnection)
    mediaSessionStore.setupActionHandlers({
      onPlay: () => {
        void playerStore.requestMusicSync()
      },
      onPause: () => audio.pause(),
      onNextTrack: () => {
        void playerStore.skipSong()
      },
      onSeekBackward: null,
      onSeekForward: null,
      onStop: null,
      onPreviousTrack: null,
    })
    return () => {
      window.removeEventListener('resize', onResize)
      playerStore.removeEventListener('change', onPlayer)
      websocketStore.removeEventListener('change', onConnection)
      audio.pause()
      playerStore.setAudioPlayer(null)
      websocketStore.disconnect()
      mediaSessionStore.clearSession()
      pwaStore.stopPeriodicUpdateCheck()
    }
  }, [])
  return (
    <audio
      ref={ref}
      preload="auto"
      onTimeUpdate={e => playerStore.onAudioTimeUpdate(e.nativeEvent)}
      onError={e => playerStore.onAudioError(e.nativeEvent)}
      onPlay={() => playerStore.startProgressUpdate()}
      onPause={() => playerStore.stopProgressUpdate()}
      onEnded={() => playerStore.stopProgressUpdate()}
    />
  )
}
