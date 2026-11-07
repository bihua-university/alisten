import type { VideoRef } from 'nsh'
import type { AudioPlayer } from '@/stores/player-store'
import { cleanup, effect, host, platform, Video } from 'nsh'
import { useStore } from '@/hooks/use-store'
import { historyStore } from '@/stores/history-store'
import { mediaSessionStore } from '@/stores/media-session-store'
import { notificationStore } from '@/stores/notification-store'
import { playerStore } from '@/stores/player-store'
import { userSettingsStore } from '@/stores/user-settings-store'
import { websocketStore } from '@/stores/websocket-store'
import { bindAndroidPlayback } from '@/utils/android-playback'
import { getAppConfig, validateConfig } from '@/utils/config'
import { getEnvironment } from '@/utils/environment'
import { setDocumentTitle, supportsBrowserPlayback } from '@/utils/runtime'

export function AudioEngine() {
  const audioUrl = useStore(playerStore, state => state.audioUrl)
  const isPlaying = useStore(playerStore, state => state.isPlaying)
  const muted = useStore(playerStore, state => state.isMuted)
  const volume = useStore(playerStore, state => state.outputVolume)
  let video: VideoRef | undefined

  effect(() => {
    const releaseAndroidPlayback = bindAndroidPlayback()
    const config = getAppConfig()
    const errors = validateConfig(config)
    if (errors.length)
      console.warn('配置错误:', errors)
    playerStore.setAudioPlayer({
      seek: seconds => video?.seek(seconds),
      play: supportsBrowserPlayback()
        ? source => host.call('media.play', { source }) as ReturnType<NonNullable<AudioPlayer['play']>>
        : undefined,
    })
    let lastSong: typeof playerStore.state.currentSong | undefined
    let lastStartedAtMs = 0
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
      setDocumentTitle(song ? `${song.title} - ${song.artist} | ${config.app.name}` : config.app.name)
      if (getEnvironment() === 'canary')
        setDocumentTitle(`${song ? `${song.title} - ${song.artist} | ` : ''}${config.app.name} · Canary`)
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
      } else if ((status === 'error' || status === 'reconnecting') && !outageNotified) {
        outageNotified = true
        notificationStore.connectionError('连接中断，正在自动重连')
      }
    }
    playerStore.addEventListener('change', onPlayer)
    onPlayer()
    websocketStore.addEventListener('change', onConnection)
    mediaSessionStore.setupActionHandlers({
      play: () => {
        void playerStore.requestMusicSync()
      },
      pause: () => playerStore.setState({ isPlaying: false }),
      nexttrack: () => {
        void playerStore.skipSong()
      },
      seekbackward: null,
      seekforward: null,
      stop: null,
      previoustrack: null,
    })
    cleanup(() => {
      releaseAndroidPlayback()
      playerStore.removeEventListener('change', onPlayer)
      websocketStore.removeEventListener('change', onConnection)
      playerStore.setState({ isPlaying: false })
      playerStore.setAudioPlayer(null)
      websocketStore.disconnect()
      mediaSessionStore.clearSession()
    })
  })

  return (
    <Each of={audioUrl() ? [audioUrl()] : []} key={source => source}>
      {source => (
        <Video
          testID="alisten-audio"
          ref={video}
          src={source()}
          paused={!isPlaying()}
          muted={muted()}
          volume={volume()}
          audioSession={platform.os === 'android' ? 'mixWithOthers' : 'playback'}
          progressUpdateInterval={100}
          style={{ width: 1, height: 1, opacity: 0, pointerEvents: 'none' }}
          onLoad={({ duration }) => {
            void playerStore.onAudioLoad(source(), duration)
          }}
          onProgress={({ currentTime }) => {
            if (source() === playerStore.state.audioUrl && playerStore.state.isPlaying)
              playerStore.onAudioTimeUpdate(currentTime)
          }}
          onBuffer={({ isBuffering }) => playerStore.setState({ isBuffering })}
          onError={error => playerStore.onAudioError(error)}
          onEnd={() => {
            playerStore.setState({ isPlaying: false, isBuffering: false })
            playerStore.stopProgressUpdate()
          }}
        />
      )}
    </Each>
  )
}
