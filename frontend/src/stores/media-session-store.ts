import type { Song } from '@/types'
import { host, platform } from 'nsh'
import { normalizeArtworkUrl } from '@/utils/artwork-url'
import { onHostEvent } from '@/utils/runtime'

type MediaAction = 'play' | 'pause' | 'previoustrack' | 'nexttrack' | 'seekbackward' | 'seekforward' | 'stop'

const actionHandlers = new Map<MediaAction, (() => void) | null>()
let removeActionListener: (() => void) | undefined

function isSupported() {
  return platform.os === 'web' && host.available()
}

function updateMetadata(song: Song | null) {
  if (!isSupported())
    return
  const metadata = song
    ? {
        title: song.title,
        artist: song.artist,
        album: song.album || '未知专辑',
        artwork: [{ src: normalizeArtworkUrl(song.cover) || '' }],
      }
    : null
  void host.call('media.update', { song: metadata }).catch(error => console.warn('更新媒体会话失败:', error))
}

function setupActionHandlers(callbacks: Partial<Record<MediaAction, (() => void) | null>>) {
  if (!isSupported())
    return
  removeActionListener?.()
  actionHandlers.clear()
  for (const [action, handler] of Object.entries(callbacks) as [MediaAction, (() => void) | null][])
    actionHandlers.set(action, handler)
  const actions = [...actionHandlers].filter(([, handler]) => handler).map(([action]) => action)
  void host.call('media.actions', { actions }).catch(error => console.warn('设置媒体会话操作失败:', error))
  removeActionListener = onHostEvent('media.action', ({ action }: { action: MediaAction }) => {
    actionHandlers.get(action)?.()
  })
}

function clearSession() {
  updateMetadata(null)
  removeActionListener?.()
  removeActionListener = undefined
  actionHandlers.clear()
  if (isSupported())
    void host.call('media.actions', { actions: [] })
}

export const mediaSessionStore = {
  isSupported,
  updateMetadata,
  setupActionHandlers,
  clearSession,
}
