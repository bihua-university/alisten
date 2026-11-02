import type { Song } from '@/types'

const actionHandlers: Map<MediaSessionAction, (() => void) | null> = new Map()

function isSupported() {
  return 'mediaSession' in navigator && 'setActionHandler' in navigator.mediaSession
}

function updateMetadata(song: Song | null) {
  if (!isSupported())
    return
  if (!song) {
    navigator.mediaSession.metadata = null
    navigator.mediaSession.playbackState = 'none'
    return
  }
  try {
    navigator.mediaSession.metadata = new MediaMetadata({
      title: song.title,
      artist: song.artist,
      album: song.album || '未知专辑',
      artwork: [{ src: song.cover }],
    })
  } catch (error) {
    console.warn('更新媒体会话元数据失败:', error)
  }
}

function setupActionHandlers(callbacks: {
  onPlay?: (() => void) | null
  onPause?: (() => void) | null
  onPreviousTrack?: (() => void) | null
  onNextTrack?: (() => void) | null
  onSeekBackward?: (() => void) | null
  onSeekForward?: (() => void) | null
  onStop?: (() => void) | null
}) {
  if (!isSupported())
    return
  try {
    if (callbacks.onPlay !== undefined) {
      actionHandlers.set('play', callbacks.onPlay)
      navigator.mediaSession.setActionHandler('play', callbacks.onPlay)
    }
    if (callbacks.onPause !== undefined) {
      actionHandlers.set('pause', callbacks.onPause)
      navigator.mediaSession.setActionHandler('pause', callbacks.onPause)
    }
    if (callbacks.onPreviousTrack !== undefined) {
      actionHandlers.set('previoustrack', callbacks.onPreviousTrack)
      navigator.mediaSession.setActionHandler('previoustrack', callbacks.onPreviousTrack)
    }
    if (callbacks.onNextTrack !== undefined) {
      actionHandlers.set('nexttrack', callbacks.onNextTrack)
      navigator.mediaSession.setActionHandler('nexttrack', callbacks.onNextTrack)
    }
    if (callbacks.onSeekBackward !== undefined) {
      actionHandlers.set('seekbackward', callbacks.onSeekBackward)
      navigator.mediaSession.setActionHandler('seekbackward', callbacks.onSeekBackward)
    }
    if (callbacks.onSeekForward !== undefined) {
      actionHandlers.set('seekforward', callbacks.onSeekForward)
      navigator.mediaSession.setActionHandler('seekforward', callbacks.onSeekForward)
    }
    if (callbacks.onStop !== undefined) {
      actionHandlers.set('stop', callbacks.onStop)
      navigator.mediaSession.setActionHandler('stop', callbacks.onStop)
    }
  } catch (error) {
    console.warn('设置媒体会话操作处理器失败:', error)
  }
}

function clearSession() {
  if (!isSupported())
    return
  try {
    navigator.mediaSession.metadata = null
    navigator.mediaSession.playbackState = 'none'
    const actions: MediaSessionAction[] = ['play', 'pause', 'previoustrack', 'nexttrack', 'seekbackward', 'seekforward', 'stop']
    actions.forEach((action) => {
      try {
        navigator.mediaSession.setActionHandler(action, null)
        actionHandlers.delete(action)
      } catch { /* ignore */ }
    })
  } catch (error) {
    console.warn('清除媒体会话失败:', error)
  }
}

export const mediaSessionStore = {
  isSupported,
  updateMetadata,
  setupActionHandlers,
  clearSession,
}
