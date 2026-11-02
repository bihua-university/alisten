import type { Song } from '@/types'
import { Store } from './store-base'

interface PlayHistoryItem {
  song: Song
  playedAt: number
}

interface SearchHistoryItem {
  query: string
  platform: string
  searchMode: string
  timestamp: number
}

function getStorageItem<T>(key: string, defaultValue: T): T {
  try {
    const item = localStorage.getItem(key)
    return item ? JSON.parse(item) : defaultValue
  } catch {
    return defaultValue
  }
}

function setStorageItem(key: string, value: any) {
  localStorage.setItem(key, JSON.stringify(value))
}

function searchHistoryKey(query: string) {
  return query.trim().toLowerCase()
}

function loadSearchHistory(): SearchHistoryItem[] {
  const saved = getStorageItem<unknown>('alisten_search_history', [])
  if (!Array.isArray(saved))
    return []
  const latest = new Map<string, SearchHistoryItem>()
  for (const item of saved) {
    if (!item || typeof item.query !== 'string' || !item.query.trim() || item.query.trimStart().startsWith('*')
      || typeof item.platform !== 'string' || typeof item.searchMode !== 'string' || typeof item.timestamp !== 'number') {
      continue
    }
    const key = searchHistoryKey(item.query)
    const previous = latest.get(key)
    if (!previous || item.timestamp > previous.timestamp)
      latest.set(key, { ...item, query: item.query.trim() })
  }
  const history = [...latest.values()].sort((a, b) => b.timestamp - a.timestamp)
  if (JSON.stringify(history) !== JSON.stringify(saved)) {
    try {
      setStorageItem('alisten_search_history', history)
    } catch (error) {
      console.warn('整理搜索历史失败:', error)
    }
  }
  return history
}

const MAX_PLAY_HISTORY_SIZE = 500

interface HistoryState {
  playHistory: PlayHistoryItem[]
  enableSearchHistory: boolean
  maxSearchHistoryCount: number
  searchHistory: SearchHistoryItem[]
  isRecording: boolean
}

class HistoryStore extends Store<HistoryState> {
  constructor() {
    super({
      playHistory: getStorageItem<PlayHistoryItem[]>('music-play-history', []),
      enableSearchHistory: getStorageItem('alisten_enable_search_history', true),
      maxSearchHistoryCount: getStorageItem('alisten_max_search_history_count', 10),
      searchHistory: loadSearchHistory(),
      isRecording: true,
    })
  }

  addToPlayHistory(song: Song) {
    if (!this.state.isRecording)
      return
    const now = Date.now()
    const existingIndex = this.state.playHistory.findIndex(item => item.song.id === song.id)
    let newHistory = [...this.state.playHistory]
    if (existingIndex !== -1) {
      newHistory.splice(existingIndex, 1)
    }
    newHistory.unshift({ song: { ...song }, playedAt: now })
    if (newHistory.length > MAX_PLAY_HISTORY_SIZE) {
      newHistory = newHistory.slice(0, MAX_PLAY_HISTORY_SIZE)
    }
    this.setState({ playHistory: newHistory })
    setStorageItem('music-play-history', newHistory)
  }

  get playHistoryList(): PlayHistoryItem[] {
    return this.state.playHistory
  }

  get playHistoryByDate() {
    const groups: { [key: string]: PlayHistoryItem[] } = {}
    this.state.playHistory.forEach((item) => {
      const dateKey = new Date(item.playedAt).toLocaleDateString('zh-CN', { year: 'numeric', month: 'long', day: 'numeric' })
      if (!groups[dateKey])
        groups[dateKey] = []
      groups[dateKey].push(item)
    })
    return Object.entries(groups).map(([date, items]) => ({ date, items }))
  }

  get playStats() {
    const totalPlays = this.state.playHistory.length
    const totalDuration = this.state.playHistory.reduce((sum, item) => sum + item.song.duration / 1000, 0)
    return { totalPlays, totalDuration }
  }

  clearPlayHistory() {
    this.setState({ playHistory: [] })
    setStorageItem('music-play-history', [])
  }

  removeFromPlayHistory(index: number) {
    if (index >= 0 && index < this.state.playHistory.length) {
      const newHistory = [...this.state.playHistory]
      newHistory.splice(index, 1)
      this.setState({ playHistory: newHistory })
      setStorageItem('music-play-history', newHistory)
    }
  }

  searchPlayHistory(query: string): PlayHistoryItem[] {
    if (!query.trim())
      return this.state.playHistory
    const lower = query.toLowerCase()
    return this.state.playHistory.filter(item =>
      item.song.title.toLowerCase().includes(lower)
      || item.song.artist.toLowerCase().includes(lower)
      || (item.song.album && item.song.album.toLowerCase().includes(lower)),
    )
  }

  togglePlayHistoryRecording() {
    this.setState({ isRecording: !this.state.isRecording })
  }

  exportPlayHistory() {
    const data = { exportedAt: new Date().toISOString(), history: this.state.playHistory }
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `music-history-${new Date().toISOString().split('T')[0]}.json`
    link.click()
    URL.revokeObjectURL(url)
  }

  private getValidSearchHistory(): SearchHistoryItem[] {
    if (!this.state.enableSearchHistory)
      return []
    try {
      return this.state.searchHistory
        .filter((item: any) => item && typeof item.query === 'string' && typeof item.platform === 'string'
          && typeof item.searchMode === 'string' && typeof item.timestamp === 'number')
        .slice(0, this.state.maxSearchHistoryCount)
    } catch (error) {
      console.warn('搜索记录验证失败:', error)
      return []
    }
  }

  addToSearchHistory(query: string, platform: string, searchMode: string) {
    if (!this.state.enableSearchHistory || !query.trim() || query.trimStart().startsWith('*'))
      return
    const newItem: SearchHistoryItem = { query: query.trim(), platform, searchMode, timestamp: Date.now() }
    let history = this.getValidSearchHistory().filter(item => searchHistoryKey(item.query) !== searchHistoryKey(newItem.query))
    history.unshift(newItem)
    if (history.length > this.state.maxSearchHistoryCount) {
      history = history.slice(0, this.state.maxSearchHistoryCount)
    }
    this.setState({ searchHistory: history })
    setStorageItem('alisten_search_history', history)
  }

  getSearchHistory(platform?: string, searchMode?: string): SearchHistoryItem[] {
    if (!this.state.enableSearchHistory)
      return []
    let history = this.getValidSearchHistory()
    if (platform || searchMode) {
      history = history.filter((item) => {
        if (platform && item.platform !== platform)
          return false
        if (searchMode && item.searchMode !== searchMode)
          return false
        return true
      })
    }
    return history
  }

  removeFromSearchHistory(query: string, platform: string, searchMode: string) {
    if (!this.state.enableSearchHistory)
      return
    const history = this.getValidSearchHistory().filter(item =>
      !(item.query === query && item.platform === platform && item.searchMode === searchMode),
    )
    this.setState({ searchHistory: history })
    setStorageItem('alisten_search_history', history)
  }

  clearSearchHistory() {
    this.setState({ searchHistory: [] })
    setStorageItem('alisten_search_history', [])
  }

  updateSearchHistorySettings(enabled: boolean, maxCount?: number) {
    const newMax = maxCount !== undefined && maxCount > 0 && maxCount <= 50 ? maxCount : this.state.maxSearchHistoryCount
    this.setState({ enableSearchHistory: enabled, maxSearchHistoryCount: newMax })
    setStorageItem('alisten_enable_search_history', enabled)
    setStorageItem('alisten_max_search_history_count', newMax)
  }

  get currentSearchHistory(): SearchHistoryItem[] {
    return this.getValidSearchHistory()
  }

  get isSearchHistoryEnabled(): boolean {
    return this.state.enableSearchHistory
  }
}

export const historyStore = new HistoryStore()
