import type { RoomInfo } from './index'

export const LIVEHOUSE_PROTOCOL_VERSION = 2

export interface ChatData {
  content: string
  sentAtMs: number
  user: User
}

export interface ChatRequest {
  content: string
}

export type Empty = Record<string, never>

export interface Error {
  code: ErrorCode
  message: string
}

export type ErrorCode = 'INVALID_MESSAGE' | 'UNSUPPORTED_VERSION' | 'UNKNOWN_TYPE' | 'INVALID_ARGUMENT' | 'NOT_FOUND' | 'UNAUTHORIZED' | 'RATE_LIMITED' | 'CONFLICT' | 'PROVIDER_ERROR' | 'UNAVAILABLE' | 'LEASE_LOST' | 'INTERNAL_ERROR'

export interface HeartbeatRequest {
  clientTimeMs: number
}

export interface HeartbeatResult {
  clientTimeMs: number
  serverTimeMs: number
}

export interface LikeResult {
  id: string
  name: string
  likes: number
  source: Source
}

export interface MembersData {
  onlineCount: number
  users: User[]
}

export interface MusicSearchResult {
  items: Song[]
  totalSize: number
}

export interface PickRequestFields {
  id?: string
  name?: string
  source: Source
}
export type PickRequest = PickRequestFields & ({ id: string } | { name: string })

export interface PickResult {
  artist: string
  id: string
  name: string
  source: Source
}

export type PlayMode = 'sequential' | 'random'
export type PlaybackStatus = 'stopped' | 'playing'

export interface PlayableSong {
  /** 播放地址对应的 ReplayGain，单位 dB；缺失时读取音频标签。 */
  gain?: number
  album: string
  artist: string
  durationMs: number
  id: string
  lyric: string
  name: string
  pictureUrl: string
  source: Source
  url: string
  webUrl: string
}

export interface PlaybackData {
  song: PlayableSong | null
  status: PlaybackStatus
  startedAtMs: number
  positionMs: number
  serverTimeMs: number
  user: User | null
}

export interface Playlist {
  creator: string
  creatorId: string
  description: string
  id: string
  name: string
  pictureUrl: string
  playCount: number
  songCount: number
  source: Source
}

export interface PlaylistSearchResult {
  items: Playlist[]
  totalSize: number
}

export interface QueueData {
  current: QueueItem | null
  items: QueueItem[]
  onlineCount: number
}

export interface QueueItem {
  likes: number
  song: Song
  user: User
}

export interface SearchRequest {
  keyword: string
  pageIndex: number
  pageSize: number
  source: Source
}

export interface SettingsData {
  playMode: PlayMode
}

export interface SettingsPatch {
  playMode?: PlayMode
}

export interface Song {
  album: string
  artist: string
  durationMs: number
  id: string
  name: string
  pictureUrl: string
  source: Source
  webUrl: string
}

export interface SongRef {
  id: string
  source: Source
}

export type Source = 'netease' | 'qq' | 'kuwo'

export interface User {
  emailHash: string
  name: string
}

export interface UserUpdateRequest {
  clientTimeMs: number
  email: string
  name: string
}

export interface UserUpdateResult {
  clientTimeMs: number
  serverTimeMs: number
  user: User
}

export interface VoteResult {
  currentVotes: number
  requiredVotes: number
  skipped: boolean
}

export interface RoomSnapshot {
  revision: number
  room: RoomInfo
  queue: QueueData
  playback: PlaybackData
  members: MembersData
  settings: SettingsData
  chat: ChatData[]
  serverTimeMs: number
}

export interface RequestDataMap {
  'clock.ping': HeartbeatRequest
  'room.sync': Empty
  'chat.send': ChatRequest
  'member.update': UserUpdateRequest
  'queue.remove': SongRef
  'queue.like': SongRef
  'queue.get': Empty
  'playback.skipVote': Empty
  'playback.get': Empty
  'settings.get': Empty
  'settings.set': SettingsPatch
  'members.get': Empty
}

export interface ResponseDataMap {
  'clock.ping': HeartbeatResult
  'room.sync': RoomSnapshot
  'chat.send': Empty
  'member.update': UserUpdateResult
  'queue.remove': SongRef & { name: string }
  'queue.like': LikeResult
  'queue.get': QueueData
  'playback.skipVote': VoteResult
  'playback.get': PlaybackData
  'settings.get': SettingsData
  'settings.set': SettingsData
  'members.get': MembersData
}

export interface StoreEventDataMap {
  'chat.message': ChatData
  'members.updated': MembersData
  'queue.updated': QueueData
  'playback.updated': PlaybackData
  'settings.updated': SettingsData
}

export type RequestType = keyof RequestDataMap
export type StoreEventType = keyof StoreEventDataMap
export type RequestMessage<T extends RequestType = RequestType> = {
  [K in T]: { version: 2, kind: 'request', type: K, requestId: string, data: RequestDataMap[K] }
}[T]
export type ResponseMessage<T extends RequestType = RequestType> = {
  [K in T]: { version: 2, kind: 'response', type: K, requestId: string } & (
    { data: ResponseDataMap[K], error?: never } | { data: Empty, error: Error }
  )
}[T]
