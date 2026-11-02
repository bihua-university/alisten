export interface Song {
  id?: string
  title: string
  artist: string
  album: string
  cover: string
  url?: string
  webUrl?: string
  duration: number
  requestedBy?: User
  source?: string
}

export interface User {
  name: string
  email?: string
  emailHash?: string
  avatar: string
}

export interface ChatMessage {
  content: string
  timestamp: number
  user: User
}

export interface RoomInfo {
  id: string
  name: string
  description: string
  population: number
  needPwd: boolean
  ultimate: boolean
}

export interface LyricLine {
  text: string
  time: number
}

export interface MusicSource {
  id: string
  name: string
  icon: string
  color: string
  description: string
}

export interface SearchResult extends Song {
  originalId: string
  previewUrl?: string
}

export interface WebSocketConfig {
  url: string
  reconnectAttempts: number
  reconnectInterval: number
  heartbeatInterval: number
}

export type ConnectionStatus = 'connecting' | 'connected' | 'disconnected' | 'error' | 'reconnecting'

export type PlayMode = 'sequential' | 'random'

export interface MusicSearchSelection {
  id: number
  keyword: string
  /** User-facing name for queries that are not plain text, such as an opened playlist. */
  label?: string
  mode: 'all' | 'song' | 'playlist'
  source: 'netease' | 'qq'
}
