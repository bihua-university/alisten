import type { LyricLine } from '@/types'
import { reduceMotion } from 'nsh'
import { isValidLrc, parseLyrics } from '@/utils/lrcParser'
import { performanceStore } from './performance-store'
import { Store } from './store-base'

interface LyricsState {
  currentLyrics: LyricLine[]
  currentLyricIndex: number
}

export interface LyricContainer {
  reset: () => void
  scrollToCenter: (index: number, smooth: boolean) => void
}

const registeredContainers = new Set<LyricContainer>()
let scrollRafId: number | null = null

class LyricsStore extends Store<LyricsState> {
  constructor() {
    super({ currentLyrics: [], currentLyricIndex: -1 })
  }

  private resetScroll() {
    if (scrollRafId !== null) {
      cancelAnimationFrame(scrollRafId)
      scrollRafId = null
    }
    registeredContainers.forEach(container => container.reset())
  }

  syncScrollAllContainers(smooth = !reduceMotion() && !performanceStore.state.reducedMotion && !['low', 'off'].includes(performanceStore.state.level)) {
    registeredContainers.forEach((container) => {
      container.scrollToCenter(this.state.currentLyricIndex, smooth)
    })
  }

  setCurrentLyrics(lyrics: LyricLine[]) {
    this.resetScroll()
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
    this.resetScroll()
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
    if (result !== this.state.currentLyricIndex)
      this.setCurrentLyricIndex(result)
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

  registerContainer(container: LyricContainer) {
    registeredContainers.add(container)
  }

  unregisterContainer(container: LyricContainer) {
    registeredContainers.delete(container)
  }
}

export const lyricsStore = new LyricsStore()
