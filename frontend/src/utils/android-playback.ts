import { host, platform } from 'nsh'
import { playerStore } from '@/stores/player-store'
import { websocketStore } from '@/stores/websocket-store'
import { normalizeArtworkUrl } from './artwork-url'

/** Publish session changes and reuse the audio engine's existing progress events. */
export function bindAndroidPlayback() {
  if (platform.os !== 'android' || !host.available())
    return () => {}
  let disposed = false
  let queued = false
  let active = false
  let previous = ''
  let focusAvailable = false
  let resuming = false
  playerStore.setAudioInterrupted(true)
  const resume = () => {
    if (disposed || !active || !focusAvailable || resuming || !playerStore.isAudioInterrupted()
      || websocketStore.state.connectionStatus !== 'connected') {
      return
    }
    resuming = true
    void playerStore.resumeAfterInterruption().catch(error => console.warn('恢复播放同步失败:', error)).finally(() => {
      resuming = false
    })
  }
  const publish = () => {
    queued = false
    if (disposed)
      return
    const status = websocketStore.state.connectionStatus
    active = status === 'connected' || (active && (status === 'reconnecting' || status === 'connecting'))
    if (!active && focusAvailable) {
      focusAvailable = false
      playerStore.setAudioInterrupted(true)
    }
    const song = playerStore.state.currentSong
    const state = {
      active,
      playing: playerStore.state.isPlaying,
      wantsAudio: !!playerStore.state.audioUrl,
      title: song?.title || '等待下一首歌曲',
      artist: song?.artist || '同频',
      album: song?.album || '',
      artwork: normalizeArtworkUrl(song?.cover) || '',
      durationMs: song?.duration || 0,
      positionMs: Math.floor(playerStore.getCurrentTime() * 1000),
      startedAtMs: playerStore.state.startedAtMs,
    }
    // MediaSession extrapolates position itself; refresh its anchor at most
    // once per ten seconds, or immediately for a track/playback change.
    const next = JSON.stringify({ ...state, positionMs: Math.floor(state.positionMs / 10_000) })
    resume()
    if (next === previous)
      return
    previous = next
    void host.call('app.backgroundPlayback', state).catch(error => console.warn('更新 Android 后台播放失败:', error))
  }
  const schedule = () => {
    if (queued || disposed)
      return
    queued = true
    queueMicrotask(publish)
  }
  const next = host.on('app.media.next', () => void playerStore.skipSong())
  const focus = host.on('app.audioFocus', (payload) => {
    focusAvailable = (payload as { available?: boolean } | null)?.available === true
    playerStore.setAudioInterrupted(true)
    if (focusAvailable)
      resume()
  })
  const tick = host.on('app.connection.tick', resume)
  const stop = host.on('app.media.stop', () => {
    websocketStore.disconnect()
    playerStore.resetRoom()
  })
  playerStore.addEventListener('change', schedule)
  playerStore.registerProgressCallback(schedule)
  websocketStore.addEventListener('change', schedule)
  schedule()
  return () => {
    disposed = true
    playerStore.removeEventListener('change', schedule)
    playerStore.unregisterProgressCallback(schedule)
    websocketStore.removeEventListener('change', schedule)
    next()
    focus()
    tick()
    stop()
    void host.call('app.backgroundPlayback', { active: false }).catch(() => {})
  }
}
