import type { SearchResult, Song } from '@/types'
import type { PlaybackData, QueueData, QueueItem, Source } from '@/types/livehouse-protocol'
import { pickMusic as pickMusicAPI, recommendMusic, resolveMusic } from '@/utils/api'
import { gainToCoefficient, readReplayGain } from '@/utils/replay-gain'
import { localStorage } from '@/utils/storage'
import { adaptProtocolUser, getDefaultAvatar } from '@/utils/user'
import { lyricsStore } from './lyrics-store'
import { mediaSessionStore } from './media-session-store'
import { notificationStore } from './notification-store'
import { Store } from './store-base'
import { websocketStore } from './websocket-store'

export interface AudioPlayer {
  seek: (seconds: number) => void
  play?: (source: string) => Promise<{ status: 'playing' | 'blocked' | 'aborted' | 'error', message?: string }>
}

interface PlayerState {
  currentTime: number
  currentSong: Song | null
  startedAtMs: number
  playlist: Song[]
  volume: number
  outputVolume: number
  isMuted: boolean
  isPlaying: boolean
  isBuffering: boolean
  positionRevision: number
  audioUrl: string
  needManualStartPlay: boolean
  recommendations: SearchResult[]
}

const DEFAULT_GAIN_COEFFICIENT = 0.6
let audioPlayer: AudioPlayer | null = null
let currentTimeValue = 0
const progressCallbacks = new Set<(time: number) => void>()
let delayedStartTimer: ReturnType<typeof setTimeout> | null = null
let volumeStorage = 75
let muteStorage = false

try {
  const stored = localStorage.getItem('VOLUME')
  if (stored !== null) {
    const value = Number.parseInt(stored, 10)
    volumeStorage = Number.isNaN(value) ? 75 : Math.max(0, Math.min(100, value))
  }
  muteStorage = localStorage.getItem('MUTE') === 'true'
} catch (error) {
  console.warn('无法读取本地存储的音量设置:', error)
}

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
  private audioInterrupted = false
  private interruptionRevision = 0
  private playAttempt = 0

  setAudioInterrupted(interrupted: boolean) {
    this.audioInterrupted = interrupted
    this.interruptionRevision++
    if (interrupted)
      this.setState({ isPlaying: false })
  }

  isAudioInterrupted() {
    return this.audioInterrupted
  }

  async resumeAfterInterruption() {
    const revision = this.interruptionRevision
    const playbackRevision = this.playbackResolution
    const ws = websocketStore.state.ws
    const data = await websocketStore.request('playback.get', {}, 10_000)
    if (revision !== this.interruptionRevision || ws !== websocketStore.state.ws || playbackRevision !== this.playbackResolution)
      return
    this.audioInterrupted = false
    await this.applyPlayback(data)
  }

  private lastLyricContent = ''
  private lastLyricTrack = ''
  private playbackData: PlaybackData | null = null
  private loadedAudioUrl = ''
  private playbackResolution = 0
  private resolvedPlayback: { key: string, url: string, gain?: number } | null = null
  private lastAudioErrorKey = ''
  private replayGainUrl: string | null = null
  private replayGainDb: number | undefined
  private replayGainCoefficient: number | null = null
  private replayGainRequest: AbortController | null = null
  private gainCoefficient = DEFAULT_GAIN_COEFFICIENT

  constructor() {
    super({
      currentTime: 0,
      currentSong: null,
      startedAtMs: 0,
      playlist: [],
      volume: volumeStorage,
      outputVolume: (volumeStorage / 100) * DEFAULT_GAIN_COEFFICIENT,
      isMuted: muteStorage,
      isPlaying: false,
      isBuffering: false,
      positionRevision: 0,
      audioUrl: '',
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

  private async applyPlayback(data: PlaybackData) {
    const resolution = ++this.playbackResolution
    this.playbackData = data
    if (!data.song) {
      this.loadedAudioUrl = ''
      this.resolvedPlayback = null
      this.lastAudioErrorKey = ''
      this.playbackData = null
      this.updateReplayGain(null)
      this.clearDelayedStart()
      this.setState({ currentSong: null, startedAtMs: 0, currentTime: 0, audioUrl: '', isPlaying: false, isBuffering: false, needManualStartPlay: false })
      this.lastLyricContent = ''
      this.lastLyricTrack = ''
      lyricsStore.loadLrcLyrics('')
      return
    }

    const song = data.song
    const key = `${song.source}:${song.id}:${data.startedAtMs}`
    if (this.lastAudioErrorKey !== key)
      this.lastAudioErrorKey = ''
    if (this.resolvedPlayback?.key === key) {
      this.applyResolvedPlayback(data, this.resolvedPlayback.url, this.resolvedPlayback.gain)
      // Reconnect snapshots may describe the same track. Reuse the loaded
      // source and seek once to the room's current position instead of waiting
      // for an onLoad event that will never repeat for an unchanged URL.
      await this.onCanPlay()
      return
    }
    if (this.state.currentSong?.id !== song.id || this.state.currentSong.source !== song.source)
      this.setState({ isPlaying: false })
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
    const lyricTrack = `${song.source}:${song.id}:${data.startedAtMs}`
    const trackChanged = lyricTrack !== this.lastLyricTrack
    if (trackChanged) {
      this.lastLyricTrack = lyricTrack
      currentTimeValue = 0
    }
    const music: Song = {
      url: normalizeAudioUrl(url),
      gain,
      id: song.id,
      source: song.source,
      title: song.name,
      artist: song.artist || '未知艺术家',
      album: song.album || '未知专辑',
      duration: song.durationMs || (this.state.currentSong?.id === song.id && this.state.currentSong.source === song.source ? this.state.currentSong.duration : 0),
      cover: song.pictureUrl || getDefaultAvatar(song.id),
      webUrl: song.webUrl || undefined,
    }
    this.updateReplayGain(music.url || null, music.gain)
    this.applyGain(this.replayGainCoefficient ?? DEFAULT_GAIN_COEFFICIENT)
    this.setState({ currentSong: music, startedAtMs: data.startedAtMs })
    const lyricContent = song.lyric || ''
    if (trackChanged || lyricContent !== this.lastLyricContent) {
      this.lastLyricContent = lyricContent
      lyricsStore.loadLrcLyrics(lyricContent)
      lyricsStore.syncLyrics(this.getCurrentTime())
    }
    mediaSessionStore.updateMetadata(music)
  }

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
    this.setState({ outputVolume: this.getEffectiveVolume() })
  }

  private getEffectiveVolume(): number {
    return Math.min(1, (this.state.volume / 100) * this.gainCoefficient)
  }

  setAudioPlayer(player: AudioPlayer | null) {
    this.loadedAudioUrl = ''
    this.resetGain()
    this.clearDelayedStart()
    this.stopProgressUpdate()
    audioPlayer = player
    if (player)
      this.updateReplayGain(this.state.currentSong?.url || null, this.state.currentSong?.gain)
    else
      this.updateReplayGain(null)
  }

  startProgressUpdate() {
    this.onAudioTimeUpdate(currentTimeValue)
  }

  stopProgressUpdate() {}

  registerProgressCallback(callback: (time: number) => void) {
    progressCallbacks.add(callback)
  }

  unregisterProgressCallback(callback: (time: number) => void) {
    progressCallbacks.delete(callback)
  }

  getCurrentTime(): number {
    return Math.max(0, Math.min(currentTimeValue, (this.state.currentSong?.duration || 0) / 1000))
  }

  async playAudio(): Promise<{ audioPlaySuccess: boolean | undefined }> {
    const source = this.state.audioUrl
    const player = audioPlayer
    if (!source || !player || this.audioInterrupted)
      return { audioPlaySuccess: undefined }
    const attempt = ++this.playAttempt
    const startedAtMs = this.state.startedAtMs
    const interruption = this.interruptionRevision
    const isCurrent = () => attempt === this.playAttempt && player === audioPlayer
      && source === this.state.audioUrl && startedAtMs === this.state.startedAtMs && interruption === this.interruptionRevision
    if (player.play) {
      try {
        const result = await player.play(source)
        if (!isCurrent())
          return { audioPlaySuccess: undefined }
        if (result.status === 'blocked') {
          this.setState({ isPlaying: false, isBuffering: false, needManualStartPlay: true })
          return { audioPlaySuccess: false }
        }
        if (result.status !== 'playing') {
          if (result.status === 'error')
            this.onAudioError({ message: result.message || '无法启动音频播放' })
          return { audioPlaySuccess: undefined }
        }
      } catch (error) {
        if (isCurrent())
          this.onAudioError({ message: error instanceof Error ? error.message : String(error) })
        return { audioPlaySuccess: undefined }
      }
    }
    this.setState({ isPlaying: true, needManualStartPlay: false })
    this.startProgressUpdate()
    return { audioPlaySuccess: true }
  }

  setAudioCurrentTime(time: number) {
    const next = Math.max(0, time)
    try {
      audioPlayer?.seek(next)
      currentTimeValue = next
      this.setState({ positionRevision: this.state.positionRevision + 1 })
    } catch (error) {
      console.warn('设置播放时间失败:', error)
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
    if (elapsedMs < 0) {
      this.setAudioCurrentTime(0)
      currentTimeValue = 0
      this.setState({ currentTime: 0, isPlaying: false })
      lyricsStore.syncLyrics(0)
      delayedStartTimer = setTimeout(() => {
        delayedStartTimer = null
        void this.onCanPlay()
      }, -elapsedMs)
      return
    }
    const durationMs = this.state.currentSong?.duration ?? 0
    const newTimeSeconds = Math.min(elapsedMs, durationMs) / 1000
    this.setAudioCurrentTime(newTimeSeconds)
    currentTimeValue = newTimeSeconds
    this.setState({ currentTime: newTimeSeconds })
    lyricsStore.syncLyrics(newTimeSeconds)
  }

  onAudioTimeUpdate(time: number) {
    currentTimeValue = time
    progressCallbacks.forEach(callback => callback(currentTimeValue))
    lyricsStore.syncLyrics(time)
  }

  onAudioError(error?: { message: string }) {
    console.error('音频播放错误:', error?.message || '未知错误')
    const data = this.playbackData
    if (!data?.song)
      return
    this.loadedAudioUrl = ''
    this.setState({ isPlaying: false, isBuffering: false, audioUrl: '', needManualStartPlay: false })
    const key = `${data.song.source}:${data.song.id}:${data.startedAtMs}`
    if (this.lastAudioErrorKey === key)
      return
    this.lastAudioErrorKey = key
    this.resolvedPlayback = null
    void this.applyPlayback(data)
  }

  async onAudioLoad(source: string, durationSeconds: number) {
    if (source !== this.state.audioUrl)
      return
    this.loadedAudioUrl = source
    this.setState({ isBuffering: false })
    const song = this.state.currentSong
    if (song && song.duration <= 0 && Number.isFinite(durationSeconds) && durationSeconds > 0)
      this.setState({ currentSong: { ...song, duration: Math.round(durationSeconds * 1000) } })
    await this.onCanPlay()
  }

  async onCanPlay() {
    // Native seeks before preparation are ignored. A new source must finish
    // loading before either the initial join or a reconnect can start it.
    if (!audioPlayer || !this.state.audioUrl || this.loadedAudioUrl !== this.state.audioUrl)
      return
    if (this.playbackData?.status !== 'playing') {
      this.setState({ isPlaying: false, needManualStartPlay: false })
      return
    }
    this.syncAudioCurrentTime()
    const elapsedMs = Date.now() + websocketStore.getClockOffsetMs() - this.state.startedAtMs
    if (!this.state.startedAtMs || elapsedMs < 0 || elapsedMs >= (this.state.currentSong?.duration || 0)) {
      this.setState({ isPlaying: false, needManualStartPlay: false })
      return
    }
    await this.playAudio()
  }

  handleNewSong() {
    const song = this.state.currentSong
    if (song?.url) {
      if (song.url === this.state.audioUrl) {
        if (this.loadedAudioUrl === song.url)
          void this.onCanPlay()
        return
      }
      this.loadedAudioUrl = ''
      this.setState({ audioUrl: song.url, isPlaying: false, isBuffering: true, currentTime: 0, needManualStartPlay: false })
    }
  }

  setVolume(volume: number) {
    volume = Math.max(0, Math.min(100, volume))
    this.setState({ volume, outputVolume: Math.min(1, (volume / 100) * this.gainCoefficient) })
    localStorage.setItem('VOLUME', volume.toString())
  }

  toggleMute() {
    const isMuted = !this.state.isMuted
    this.setState({ isMuted })
    localStorage.setItem('MUTE', isMuted.toString())
  }

  get progressPercentage(): number {
    if (this.state.currentSong?.duration)
      return (this.state.currentTime / (this.state.currentSong.duration / 1000)) * 100
    return 0
  }

  async pickMusic(result: { id?: string, title: string, source?: string }) {
    const source = (result.source || 'netease') as Source
    try {
      const response = await pickMusicAPI({ source, id: result.id, name: result.title })
      notificationStore.success(`已发送点歌请求: ${response.name}`)
    } catch (error) {
      notificationStore.error(error instanceof Error ? error.message : '点歌失败')
    }
  }

  async skipSong() {
    try {
      const response = await websocketStore.request('playback.skipVote', {})
      if (response.skipped)
        notificationStore.success('已切歌', { duration: 2000 })
      else
        notificationStore.info(`切歌投票 ${response.currentVotes}/${response.requiredVotes}`, { duration: 2000 })
    } catch (error) {
      notificationStore.error(error instanceof Error ? error.message : '切歌失败')
    }
  }

  async requestMusicSync() {
    try {
      const data = await websocketStore.request('playback.get', {})
      await this.applyPlayback(data)
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
    this.interruptionRevision++
    this.loadedAudioUrl = ''
    this.playbackResolution++
    this.playbackData = null
    this.resolvedPlayback = null
    this.lastAudioErrorKey = ''
    this.updateReplayGain(null)
    this.clearDelayedStart()
    this.stopProgressUpdate()
    currentTimeValue = 0
    this.setState({
      currentSong: null,
      startedAtMs: 0,
      currentTime: 0,
      playlist: [],
      recommendations: [],
      needManualStartPlay: false,
      audioUrl: '',
      isPlaying: false,
      isBuffering: false,
    })
    lyricsStore.clearLyrics()
    this.lastLyricContent = ''
    mediaSessionStore.updateMetadata(null)
    this.lastLyricTrack = ''
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
}

export const playerStore = new PlayerStore()
