import type { LyricLine } from '@/types'
import { isValidLrc, parseLyrics } from '@/utils/lrcParser'
import { performanceStore } from './performance-store'
import { Store } from './store-base'

interface LyricsState {
  currentLyrics: LyricLine[]
  currentLyricIndex: number
}

const registeredContainers = new Set<HTMLElement>()
let scrollRafId: number | null = null

class LyricsStore extends Store<LyricsState> {
  constructor() {
    super({ currentLyrics: [], currentLyricIndex: -1 })
  }

  private scrollToCenter(container: HTMLElement, index: number, smooth = true) {
    let before: HTMLElement | undefined
    let after: HTMLElement | undefined
    for (const line of container.querySelectorAll<HTMLElement>('[data-lyric-index]')) {
      const lineIndex = Number(line.dataset.lyricIndex)
      if (lineIndex <= index)
        before = line
      if (lineIndex >= index) {
        after = line
        break
      }
    }
    before ??= after
    after ??= before
    if (!before || !after)
      return
    // During a silent cue, center the boundary between the surrounding words
    // without inserting an empty row in the layout.
    const center = (before.offsetTop + before.clientHeight / 2 + after.offsetTop + after.clientHeight / 2) / 2
    container.scrollTo({ top: Math.max(0, center - container.clientHeight / 2), behavior: smooth ? 'smooth' : 'instant' })
  }

  syncScrollAllContainers(smooth = !performanceStore.state.reducedMotion && !['low', 'off'].includes(performanceStore.state.level) && !window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
    registeredContainers.forEach((container) => {
      this.scrollToCenter(container, this.state.currentLyricIndex, smooth)
    })
  }

  setCurrentLyrics(lyrics: LyricLine[]) {
    // Keep silent cues in the timeline only; they never create lyric DOM rows.
    const compactLyrics: LyricLine[] = []
    for (const line of lyrics) {
      const text = line.text.trim()
      if (!text && compactLyrics.length && !compactLyrics[compactLyrics.length - 1].text)
        continue
      compactLyrics.push({ ...line, text })
    }
    this.setState({ currentLyrics: compactLyrics, currentLyricIndex: -1 })
  }

  setCurrentLyricIndex(index: number) {
    if (index !== this.state.currentLyricIndex) {
      this.setState({ currentLyricIndex: index })
      if (scrollRafId === null) {
        scrollRafId = requestAnimationFrame(() => {
          this.syncScrollAllContainers()
          scrollRafId = null
        })
      }
    }
  }

  clearLyrics() {
    this.setState({ currentLyrics: [], currentLyricIndex: -1 })
  }

  get lyricLines(): Array<LyricLine & { isActive: boolean, isPassed: boolean, isComing: boolean }> {
    return this.state.currentLyrics.map((lyric, index) => ({
      ...lyric,
      isActive: !!lyric.text && index === this.state.currentLyricIndex,
      isPassed: index < this.state.currentLyricIndex,
      isComing: index > this.state.currentLyricIndex,
    }))
  }

  syncLyrics(currentTime: number) {
    const lyrics = this.state.currentLyrics
    if (lyrics.length === 0)
      return
    let left = 0
    let right = lyrics.length - 1
    let result = -1
    while (left <= right) {
      const mid = Math.floor((left + right) / 2)
      if (lyrics[mid].time <= currentTime) {
        result = mid
        left = mid + 1
      } else {
        right = mid - 1
      }
    }
    if (result !== this.state.currentLyricIndex) {
      this.setCurrentLyricIndex(result)
    }
  }

  loadLrcLyrics(lrcContent: string) {
    try {
      if (!lrcContent.trim() || !isValidLrc(lrcContent)) {
        this.clearLyrics()
        return false
      }
      const parsed = parseLyrics(lrcContent)
      this.setCurrentLyrics(parsed.lyrics)
      return true
    } catch (error) {
      console.error('解析LRC歌词失败:', error)
      return false
    }
  }

  seekToLyric(index: number): number {
    if (index >= 0 && index < this.state.currentLyrics.length) {
      this.setCurrentLyricIndex(index)
      return this.state.currentLyrics[index].time
    }
    return 0
  }

  getCurrentLyricInfo() {
    const current = this.state.currentLyrics[this.state.currentLyricIndex]
    const next = this.state.currentLyrics[this.state.currentLyricIndex + 1]
    const prev = this.state.currentLyrics[this.state.currentLyricIndex - 1]
    return {
      current: current || null,
      next: next || null,
      prev: prev || null,
      progress: Math.max(0, this.state.currentLyricIndex) / Math.max(1, this.state.currentLyrics.length - 1),
      totalLines: this.state.currentLyrics.length,
    }
  }

  registerContainer(el: HTMLElement) {
    registeredContainers.add(el)
  }

  unregisterContainer(el: HTMLElement) {
    registeredContainers.delete(el)
  }
}

export const lyricsStore = new LyricsStore()
