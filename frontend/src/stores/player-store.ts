import type { SearchResult, Song } from '@/types'
import type { PlaybackData, QueueData, QueueItem, Source } from '@/types/livehouse-protocol'
import { pickMusic as pickMusicAPI, recommendMusic, resolveMusic } from '@/utils/api'
import { gainToCoefficient, readReplayGain } from '@/utils/replay-gain'
import { adaptProtocolUser, getDefaultAvatar } from '@/utils/user'
import { lyricsStore } from './lyrics-store'
import { mediaSessionStore } from './media-session-store'
import { notificationStore } from './notification-store'
import { Store } from './store-base'
import { websocketStore } from './websocket-store'

interface PlayerState {
  currentTime: number
  currentSong: Song | null
  /** 当前歌曲开始播放的服务器时间戳（毫秒），0 表示无 */
  startedAtMs: number
  /** 等待队列（不含当前歌曲） */
  playlist: Song[]
  volume: number
  isMuted: boolean
  needManualStartPlay: boolean
  recommendations: SearchResult[]
}

const DEFAULT_GAIN_COEFFICIENT = 0.6
let audioPlayer: HTMLAudioElement | null = null
let rafId: number | null = null
let currentTimeValue = 0
let lastProgressEmitTime = 0
const progressCallbacks = new Set<(time: number) => void>()
let delayedStartTimer: ReturnType<typeof setTimeout> | null = null
let volumeStorage = 75
let muteStorage = false

try {
  const stored = localStorage.getItem('VOLUME')
  if (stored !== null) {
    const v = Number.parseInt(stored, 10)
    volumeStorage = Number.isNaN(v) ? 75 : Math.max(0, Math.min(100, v))
  }
  muteStorage = localStorage.getItem('MUTE') === 'true'
} catch (error) {
  console.warn('无法读取本地存储的音量设置:', error)
}

function saveVolume(volume: number) {
  localStorage.setItem('VOLUME', volume.toString())
}

function saveMute(isMuted: boolean) {
  localStorage.setItem('MUTE', isMuted.toString())
}

/** 修正历史遗留的酷我 CDN 链接，并统一升级为 https */
function normalizeAudioUrl(rawUrl: string): string {
  let url = rawUrl
  if (url.includes('kuwo.cn') && !url.includes('-')) {
    const urls = url.split('.sycdn.')
    const headUrls = urls[0].replace('http://', '').split('.')
    const lastHeadUrl = headUrls[headUrls.length - 1]
    url = `https://${lastHeadUrl}-sycdn.${urls[1]}`
  }
  return url.replace('http://', 'https://')
}

function mapQueueItem(item: QueueItem): Song {
  return {
    id: item.song.id,
    source: item.song.source,
    title: item.song.name,
    artist: item.song.artist || '未知艺术家',
    album: item.song.album || '未知专辑',
    duration: item.song.durationMs,
    cover: item.song.pictureUrl || getDefaultAvatar(item.song.id),
    webUrl: item.song.webUrl || undefined,
    requestedBy: adaptProtocolUser(item.user),
  }
}

class PlayerStore extends Store<PlayerState> {
  private lastLyricContent = ''
  private playbackData: PlaybackData | null = null
  private playbackResolution = 0
  private resolvedPlayback: { key: string, url: string, gain?: number } | null = null
  private lastAudioErrorKey = ''
  private replayGainUrl: string | null = null
  private replayGainDb: number | undefined
  private replayGainCoefficient: number | null = null
  private replayGainRequest: AbortController | null = null
  private gainCoefficient = DEFAULT_GAIN_COEFFICIENT
  private handleCanPlay = () => {
    void this.onCanPlay()
  }

  constructor() {
    super({
      currentTime: 0,
      currentSong: null,
      startedAtMs: 0,
      playlist: [],
      volume: volumeStorage,
      isMuted: muteStorage,
      needManualStartPlay: false,
      recommendations: [],
    })

    websocketStore.registerEventHandler('playback.updated', (data) => {
      void this.applyPlayback(data)
    })

    websocketStore.registerEventHandler('queue.updated', (data) => {
      this.applyQueue(data)
    })
  }

  /** 应用播放状态快照，播放前通过 livehouse-api 获取最新音频地址。 */
  private async applyPlayback(data: PlaybackData) {
    const resolution = ++this.playbackResolution
    this.playbackData = data
    if (!data.song) {
      this.resolvedPlayback = null
      this.lastAudioErrorKey = ''
      this.playbackData = null
      this.updateReplayGain(null)
      this.clearDelayedStart()
      if (audioPlayer) {
        audioPlayer.pause()
        audioPlayer.removeAttribute('src')
        audioPlayer.load()
      }
      this.setState({ currentSong: null, startedAtMs: 0, currentTime: 0 })
      this.lastLyricContent = ''
      lyricsStore.loadLrcLyrics('')
      return
    }

    const song = data.song
    const key = `${song.source}:${song.id}:${data.startedAtMs}`
    if (this.lastAudioErrorKey !== key)
      this.lastAudioErrorKey = ''
    if (this.resolvedPlayback?.key === key) {
      this.applyResolvedPlayback(data, this.resolvedPlayback.url, this.resolvedPlayback.gain)
      return
    }

    if (this.state.currentSong?.id !== song.id || this.state.currentSong.source !== song.source)
      audioPlayer?.pause()
    this.applyResolvedPlayback(data, '')
    try {
      const resolved = await resolveMusic(song.source, song.id)
      if (resolution !== this.playbackResolution)
        return
      this.resolvedPlayback = { key, url: resolved.url, gain: resolved.gain }
      this.applyResolvedPlayback(data, resolved.url, resolved.gain)
    } catch (error) {
      if (resolution === this.playbackResolution)
        notificationStore.error(error instanceof Error ? error.message : '获取播放地址失败')
    }
  }

  private applyResolvedPlayback(data: PlaybackData, url: string, gain?: number) {
    const song = data.song!
    const music: Song = {
      url: normalizeAudioUrl(url),
      gain,
      id: song.id,
      source: song.source,
      title: song.name,
      artist: song.artist || '未知艺术家',
      album: song.album || '未知专辑',
      duration: song.durationMs,
      cover: song.pictureUrl || getDefaultAvatar(song.id),
      webUrl: song.webUrl || undefined,
    }
    this.updateReplayGain(music.url || null, music.gain)
    this.applyGain(this.replayGainCoefficient ?? DEFAULT_GAIN_COEFFICIENT)

    this.setState({ currentSong: music, startedAtMs: data.startedAtMs })
    if (audioPlayer) {
      audioPlayer.volume = this.getEffectiveVolume()
    }
    const lyricContent = song.lyric || ''
    if (lyricContent !== this.lastLyricContent) {
      this.lastLyricContent = lyricContent
      lyricsStore.loadLrcLyrics(lyricContent)
      lyricsStore.syncLyrics(this.getCurrentTime())
    }
    mediaSessionStore.updateMetadata(music)
  }

  /**
   * 应用队列快照，只替换等待队列；「正在播放」一律取自播放快照，
   * 避免旧快照把刚切到的歌曲覆盖回上一首。
   */
  private applyQueue(data: QueueData) {
    this.setState({ playlist: data.items.map(mapQueueItem) })
  }

  private updateReplayGain(url: string | null, gainDb?: number) {
    if (url === this.replayGainUrl && gainDb === this.replayGainDb)
      return
    this.replayGainRequest?.abort()
    this.replayGainRequest = null
    this.replayGainUrl = url
    this.replayGainDb = gainDb
    this.replayGainCoefficient = null
    if (!url) {
      this.resetGain()
      return
    }
    const coefficient = gainToCoefficient(gainDb)
    if (coefficient !== null) {
      this.replayGainCoefficient = coefficient
      this.applyGain(coefficient)
      return
    }
    const request = new AbortController()
    this.replayGainRequest = request
    void readReplayGain(url, request.signal).then((coefficient) => {
      if (request.signal.aborted)
        return
      this.replayGainCoefficient = coefficient
      this.applyGain(coefficient ?? DEFAULT_GAIN_COEFFICIENT)
    })
  }

  private resetGain() {
    this.applyGain(DEFAULT_GAIN_COEFFICIENT)
  }

  private applyGain(coefficient: number) {
    this.gainCoefficient = coefficient
    if (audioPlayer)
      audioPlayer.volume = this.getEffectiveVolume()
  }

  // 用户音量与歌曲增益立即生效。
  // HTMLAudioElement 的音量上限为 100%。
  private getEffectiveVolume(): number {
    return Math.min(1, (this.state.volume / 100) * this.gainCoefficient)
  }

  setAudioPlayer(el: HTMLAudioElement | null) {
    audioPlayer?.removeEventListener('canplay', this.handleCanPlay)
    this.resetGain()
    this.clearDelayedStart()
    this.stopProgressUpdate()
    audioPlayer = el
    if (el) {
      this.updateReplayGain(this.state.currentSong?.url || null, this.state.currentSong?.gain)
      this.applyGain(this.replayGainCoefficient ?? DEFAULT_GAIN_COEFFICIENT)
      el.volume = this.getEffectiveVolume()
      el.muted = this.state.isMuted
    } else {
      this.updateReplayGain(null)
    }
  }

  getAudioPlayer(): HTMLAudioElement | null {
    return audioPlayer
  }

  private runProgressLoop() {
    if (audioPlayer && !audioPlayer.paused && !audioPlayer.ended) {
      const now = performance.now()
      // Follow the media clock: wall time keeps advancing during buffering and
      // can disagree with the sound after a seek or playback-rate change.
      const newTime = this.getCurrentTime()
      if (newTime >= (this.state.currentSong?.duration || 0) / 1000)
        audioPlayer.pause()
      currentTimeValue = newTime
      // Only changing lines notify subscribers; keep lyric boundaries precise
      // without raising the frequency of progress-bar DOM updates.
      lyricsStore.syncLyrics(newTime)
      // 每 100ms 触发一次进度回调，降低 DOM 更新频率（约 10fps）
      if (now - lastProgressEmitTime >= 100) {
        lastProgressEmitTime = now
        progressCallbacks.forEach(cb => cb(currentTimeValue))
      }
      rafId = requestAnimationFrame(() => this.runProgressLoop())
      return
    }
    rafId = null
  }

  startProgressUpdate() {
    if (rafId === null) {
      rafId = requestAnimationFrame(() => this.runProgressLoop())
    }
  }

  stopProgressUpdate() {
    if (rafId !== null) {
      cancelAnimationFrame(rafId)
      rafId = null
    }
  }

  registerProgressCallback(cb: (time: number) => void) {
    progressCallbacks.add(cb)
  }

  unregisterProgressCallback(cb: (time: number) => void) {
    progressCallbacks.delete(cb)
  }

  getCurrentTime(): number {
    if (audioPlayer)
      return Math.max(0, Math.min(audioPlayer.currentTime, (this.state.currentSong?.duration || 0) / 1000))
    return currentTimeValue
  }

  async playAudio(): Promise<{ audioPlaySuccess: boolean | undefined }> {
    if (audioPlayer) {
      audioPlayer.volume = this.getEffectiveVolume()
      audioPlayer.muted = this.state.isMuted
      try {
        await audioPlayer.play()
      } catch (e) {
        console.warn('播放失败', e)
        return { audioPlaySuccess: false }
      }
      this.startProgressUpdate()
      return { audioPlaySuccess: true }
    }
    return { audioPlaySuccess: undefined }
  }

  setAudioCurrentTime(time: number) {
    if (audioPlayer) {
      try {
        audioPlayer.currentTime = Math.max(0, time)
        currentTimeValue = audioPlayer.currentTime
      } catch (e) {
        console.warn('设置播放时间失败:', e)
      }
    }
  }

  private clearDelayedStart() {
    if (delayedStartTimer) {
      clearTimeout(delayedStartTimer)
      delayedStartTimer = null
    }
  }

  syncAudioCurrentTime() {
    this.clearDelayedStart()
    const startedAtMs = this.state.startedAtMs
    if (!startedAtMs)
      return
    const elapsedMs = Date.now() + websocketStore.getClockOffsetMs() - startedAtMs
    // 播放开始时间可能领先约 200ms，属正常预推送而非时钟错误：等到开始时间再播放
    if (elapsedMs < 0) {
      this.setAudioCurrentTime(0)
      currentTimeValue = 0
      this.setState({ currentTime: 0 })
      lyricsStore.syncLyrics(0)
      delayedStartTimer = setTimeout(() => {
        delayedStartTimer = null
        void this.onCanPlay()
      }, -elapsedMs)
      return
    }
    const durationMs = this.state.currentSong?.duration ?? 0
    const newTimeSeconds = Math.min(elapsedMs, durationMs) / 1000
    if (audioPlayer) {
      this.setAudioCurrentTime(newTimeSeconds)
      currentTimeValue = newTimeSeconds
      this.setState({ currentTime: newTimeSeconds })
      lyricsStore.syncLyrics(newTimeSeconds)
      console.log('同步新时间:', `${newTimeSeconds}s`)
    }
  }

  onAudioTimeUpdate(event: Event) {
    const audio = event.target as HTMLAudioElement
    if (audio) {
      currentTimeValue = audio.currentTime
      lastProgressEmitTime = performance.now()
      progressCallbacks.forEach(cb => cb(currentTimeValue))
      lyricsStore.syncLyrics(audio.currentTime)
    }
  }

  onAudioError(event: Event) {
    const audio = event.target as HTMLAudioElement
    console.error('音频播放错误:', audio.error)
    const data = this.playbackData
    if (!data?.song)
      return
    const key = `${data.song.source}:${data.song.id}:${data.startedAtMs}`
    if (this.lastAudioErrorKey === key)
      return
    this.lastAudioErrorKey = key
    this.resolvedPlayback = null
    void this.applyPlayback(data)
  }

  async onCanPlay() {
    this.syncAudioCurrentTime()
    const elapsedMs = Date.now() + websocketStore.getClockOffsetMs() - this.state.startedAtMs
    if (!this.state.startedAtMs || elapsedMs < 0 || elapsedMs >= (this.state.currentSong?.duration || 0)) {
      audioPlayer?.pause()
      return
    }
    const res = await this.playAudio()
    if (res.audioPlaySuccess === false)
      this.setState({ needManualStartPlay: true })
  }

  handleNewSong() {
    const song = this.state.currentSong
    if (song && audioPlayer && song.url) {
      console.log('加载新歌曲:', song.title)
      // 直接设置 src，确保 load() 前音频源已经就绪
      // Seeking can fire canplay again. Consume it once per source load,
      // before syncing currentTime, to avoid an endless seek/play loop.
      audioPlayer.removeEventListener('canplay', this.handleCanPlay)
      audioPlayer.addEventListener('canplay', this.handleCanPlay, { once: true })
      audioPlayer.src = song.url
      audioPlayer.load()
    }
  }

  setVolumeFromClick(event: MouseEvent) {
    const target = event.currentTarget as HTMLElement
    const rect = target.getBoundingClientRect()
    const clickX = event.clientX - rect.left
    const percentage = (clickX / rect.width) * 100
    const newVolume = Math.max(0, Math.min(100, percentage))
    this.setState({ volume: newVolume })
    saveVolume(newVolume)
    if (newVolume > 0 && this.state.isMuted) {
      this.setState({ isMuted: false })
      saveMute(false)
    }
    if (audioPlayer)
      audioPlayer.volume = this.getEffectiveVolume()
  }

  setVolume(volume: number) {
    volume = Math.max(0, Math.min(100, volume))
    this.setState({ volume })
    saveVolume(volume)
    if (audioPlayer)
      audioPlayer.volume = this.getEffectiveVolume()
  }

  toggleMute() {
    const newMuted = !this.state.isMuted
    this.setState({ isMuted: newMuted })
    saveMute(newMuted)
    if (audioPlayer)
      audioPlayer.muted = newMuted
  }

  get progressPercentage(): number {
    if (this.state.currentSong?.duration) {
      return (this.state.currentTime / (this.state.currentSong.duration / 1000)) * 100
    }
    return 0
  }

  async pickMusic(result: { id?: string, title: string, source?: string }) {
    const source = (result.source || 'netease') as Source
    try {
      const res = await pickMusicAPI({ source, id: result.id, name: result.title })
      notificationStore.success(`已发送点歌请求: ${res.name}`)
    } catch (error) {
      notificationStore.error(error instanceof Error ? error.message : '点歌失败')
    }
  }

  async skipSong() {
    try {
      const res = await websocketStore.request('playback.skipVote', {})
      if (res.skipped) {
        notificationStore.success('已切歌', { icon: 'fa-solid fa-forward', duration: 2000 })
      } else {
        notificationStore.info(`切歌投票 ${res.currentVotes}/${res.requiredVotes}`, { icon: 'fa-solid fa-forward', duration: 2000 })
      }
    } catch (error) {
      notificationStore.error(error instanceof Error ? error.message : '切歌失败')
    }
  }

  async requestMusicSync() {
    console.log('请求重新同步音乐')
    try {
      const previousUrl = this.state.currentSong?.url
      const previousStartedAtMs = this.state.startedAtMs
      const data = await websocketStore.request('playback.get', {})
      await this.applyPlayback(data)
      // Explicit resume/sync still seeks once even for an identical snapshot.
      if (this.state.currentSong?.url && this.state.currentSong.url === previousUrl
        && this.state.startedAtMs === previousStartedAtMs) {
        await this.onCanPlay()
      }
    } catch (error) {
      console.warn('同步播放状态失败:', error instanceof Error ? error.message : error)
    }
  }

  async refreshQueue() {
    try {
      this.applyQueue(await websocketStore.request('queue.get', {}))
    } catch (error) {
      console.warn('获取队列失败:', error instanceof Error ? error.message : error)
    }
  }

  resetRoom() {
    this.playbackResolution++
    this.playbackData = null
    this.resolvedPlayback = null
    this.lastAudioErrorKey = ''
    this.updateReplayGain(null)
    this.clearDelayedStart()
    audioPlayer?.pause()
    audioPlayer?.removeAttribute('src')
    audioPlayer?.load()
    this.stopProgressUpdate()
    currentTimeValue = 0
    this.setState({ currentSong: null, startedAtMs: 0, currentTime: 0, playlist: [], recommendations: [], needManualStartPlay: false })
    lyricsStore.clearLyrics()
    this.lastLyricContent = ''
    mediaSessionStore.updateMetadata(null)
  }

  async pullRecommendations() {
    try {
      const data = await recommendMusic(this.state.currentSong?.source === 'netease' ? this.state.currentSong.id : undefined)
      const results: SearchResult[] = data.items
        .filter(item => item && item.id && item.name)
        .map(item => ({
          id: item.id,
          originalId: item.id,
          source: item.source,
          title: item.name,
          artist: item.artist || '未知艺术家',
          album: item.album || '未知专辑',
          cover: item.pictureUrl || getDefaultAvatar(item.id),
          duration: item.durationMs,
          webUrl: item.webUrl || undefined,
        }))
      this.setState({ recommendations: results })
    } catch (error) {
      notificationStore.error(error instanceof Error ? error.message : '获取推荐失败')
    }
  }

  clearManualStartPlay() {
    this.setState({ needManualStartPlay: false })
  }
}

export const playerStore = new PlayerStore()
